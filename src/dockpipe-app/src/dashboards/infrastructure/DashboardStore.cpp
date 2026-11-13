#include "DashboardStore.h"
#include <QCoreApplication>
#include <QDir>

#include <algorithm>
#include "ContextStore.h"

#include <QCryptographicHash>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLockFile>
#include <QRegularExpression>
#include <QSaveFile>
#include <QSet>
#include <QUuid>

namespace {
// Layout metadata for at most 64 panels fits well below this capacity. Do not
// allow a configuration file to become an unbounded browser/session artifact.
constexpr qint64 maximumStoreBytes = 1024 * 1024;

bool readBytes(QByteArray &bytes, QString &error)
{
    QFile file(DashboardStore::filePath());
    if (!file.exists()) {
        bytes.clear();
        return true;
    }
    if (!file.open(QIODevice::ReadOnly) || file.size() > maximumStoreBytes) {
        error = QCoreApplication::translate("DashboardStore", "Could not read dashboards, or the file exceeds 1 MiB. Existing data was kept.");
        return false;
    }
    bytes = file.read(maximumStoreBytes + 1);
    if (file.error() != QFile::NoError || bytes.size() > maximumStoreBytes) {
        error = QCoreApplication::translate("DashboardStore", "Could not read dashboard settings. Existing data was kept.");
        return false;
    }
    return true;
}

QByteArray revision(const QByteArray &bytes)
{
    return QCryptographicHash::hash(bytes, QCryptographicHash::Sha256);
}

bool validMachineContext(const QJsonValue &value)
{
    if (!value.isObject())
        return false;
    const auto context = value.toObject();
    const QString mode = context.value("mode").toString();
    if (mode != "follow" && mode != "local" && mode != "remote")
        return false;
    for (auto item = context.begin(); item != context.end(); ++item) {
        if (item.key() != "mode" && item.key() != "target" && item.key() != "rootId")
            return false;
    }
    static const QRegularExpression identifier(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    if (context.contains("rootId")) {
        if (!context.value("rootId").isString())
            return false;
        const QString root = context.value("rootId").toString();
        if (!root.isEmpty() && !identifier.match(root).hasMatch())
            return false;
    }
    if (!context.contains("target"))
        return mode != "remote";
    if (mode == "local" || !context.value("target").isObject())
        return false;
    const auto target = context.value("target").toObject();
    if (target.size() != 3 || !target.contains("node") || !target.contains("fingerprint") || !target.contains("generation"))
        return false;
    return identifier.match(target.value("node").toString()).hasMatch()
        && digest.match(target.value("fingerprint").toString()).hasMatch()
        && digest.match(target.value("generation").toString()).hasMatch();
}

bool validId(const QString &id)
{
    const QUuid uuid(id);
    return !uuid.isNull() && uuid.toString(QUuid::WithoutBraces) == id;
}

QJsonObject optionsJson(const DashboardOptions &options)
{
    return {{"compactTiling", options.compactTiling}, {"autoHideDock", options.autoHideDock}, {"showPanelHeaders", options.showPanelHeaders},
            {"autoAcquireApps", options.autoAcquireApps}, {"canvasBackground", options.canvasBackground},
            {"canvasImagePath", options.canvasImagePath}, {"canvasFileScope", options.canvasFileScope},
            {"tilingFileScope", options.tilingFileScope}};
}

bool readOptions(const QJsonObject &object, DashboardOptions &options)
{
    for (const QString &name : {QString("compactTiling"), QString("autoHideDock"), QString("showPanelHeaders"), QString("autoAcquireApps")}) {
        if (object.contains(name) && !object.value(name).isBool())
            return false;
    }
    for (const QString &name : {QString("canvasBackground"), QString("canvasImagePath"), QString("canvasFileScope"), QString("tilingFileScope")}) {
        if (object.contains(name) && !object.value(name).isString())
            return false;
    }
    options.autoHideDock = object.value("autoHideDock").toBool(true);
    options.compactTiling = object.value("compactTiling").toBool(true);
    options.showPanelHeaders = object.value("showPanelHeaders").toBool(true);
    options.autoAcquireApps = object.value("autoAcquireApps").toBool(false);
    options.canvasBackground = object.value("canvasBackground").toString("grid");
    options.canvasImagePath = object.value("canvasImagePath").toString();
    options.canvasFileScope = object.value("canvasFileScope").toString("project");
    options.tilingFileScope = object.value("tilingFileScope").toString("system");
    return true;
}

QJsonObject retainTree(const QJsonObject &node, const QSet<QString> &retained, int depth, int &nodes)
{
    // Legacy layouts contain at most 16 leaves and 31 binary tree nodes.
    if (depth > 16 || ++nodes > 31)
        return {};
    if (node.value("type") == "tabs") {
        QJsonArray ids;
        for (const auto &id : node.value("ids").toArray()) {
            if (retained.contains(id.toString()))
                ids.append(id);
        }
        if (ids.isEmpty())
            return {};
        QJsonValue active = node.value("active");
        if (!ids.contains(active))
            active = ids.first();
        return {{"type", "tabs"}, {"ids", ids}, {"active", active}};
    }
    if (node.value("type") != "split")
        return {};
    const auto originalChildren = node.value("children").toArray();
    const auto originalSizes = node.value("sizes").toArray();
    if (originalChildren.size() != 2 || originalSizes.size() != 2)
        return {};
    QJsonArray children;
    QJsonArray sizes;
    for (int index = 0; index < 2; ++index) {
        const auto child = retainTree(originalChildren.at(index).toObject(), retained, depth + 1, nodes);
        if (!child.isEmpty()) {
            children.append(child);
            sizes.append(originalSizes.at(index));
        }
    }
    if (children.isEmpty())
        return {};
    if (children.size() == 1)
        return children.first().toObject();
    return {{"type", "split"}, {"orientation", node.value("orientation")}, {"children", children}, {"sizes", sizes}};
}
}

QJsonObject DashboardStore::layoutForEnvironment(const DashboardDefinition &dashboard, const QString &environment)
{
    QJsonObject layout = dashboard.layout;
    if (layout.value("version") == 2)
        layout = layout.value(environment).toObject();
    if (layout.isEmpty())
        return {};
    QSet<QString> retained;
    for (const auto &panel : dashboard.panels) {
        if (panel.environment == environment)
            retained.insert(panel.id);
    }
    layout["mode"] = environment;
    for (const QString &key : {QString("canvas"), QString("floatingGeometry")}) {
        auto geometry = layout.value(key).toObject();
        for (const auto &id : geometry.keys()) {
            if (!retained.contains(id))
                geometry.remove(id);
        }
        layout[key] = geometry;
    }
    for (const QString &key : {QString("canvasOrder"), QString("floating")}) {
        QJsonArray ids;
        for (const auto &id : layout.value(key).toArray()) {
            if (retained.contains(id.toString()))
                ids.append(id);
        }
        layout[key] = ids;
    }
    int nodes = 0;
    layout["tree"] = retainTree(layout.value("tree").toObject(), retained, 1, nodes);
    return layout;
}

QString DashboardStore::filePath()
{
    return ContextStore::configDir() + QStringLiteral("/dashboards.json");
}

QString DashboardStore::newId()
{
    return QUuid::createUuid().toString(QUuid::WithoutBraces);
}

QString DashboardStore::defaultDashboardId()
{
    return QStringLiteral("c68d1e00-2ac2-4710-9200-000000000001");
}

bool DashboardStore::ensureDefaultDashboard(QVector<DashboardDefinition> &dashboards)
{
    for (auto &dashboard : dashboards) {
        if (dashboard.id != defaultDashboardId())
            continue;
        if (dashboard.name == QStringLiteral("Dashboard"))
            return false;
        dashboard.name = QStringLiteral("Dashboard");
        return true;
    }
    dashboards.prepend({defaultDashboardId(), QStringLiteral("Dashboard"), {}, {}});
    return true;
}

bool DashboardStore::validate(const QVector<DashboardDefinition> &dashboards, QString &error)
{
    const bool hasDefault = std::any_of(dashboards.cbegin(), dashboards.cend(), [](const DashboardDefinition &dashboard) {
        return dashboard.id == defaultDashboardId();
    });
    if (dashboards.size() > maximumDashboards || (!hasDefault && dashboards.size() == maximumDashboards)) {
        error = QCoreApplication::translate("DashboardStore", "Up to %1 dashboards are supported.").arg(maximumDashboards);
        return false;
    }
    QSet<QString> dashboardIds;
    QSet<QString> panelIds;
    const QRegularExpression kindPattern(QStringLiteral("^[a-z][a-z0-9_.-]{0,79}$"));
    int totalPanels = 0;
    for (const auto &dashboard : dashboards) {
        if (!validId(dashboard.id) || dashboardIds.contains(dashboard.id)
            || dashboard.name.trimmed().isEmpty() || dashboard.name.size() > 80
            || dashboard.name.contains(QChar::Null)
            || dashboard.panels.size() > maximumPanelsPerDashboard) {
            error = QCoreApplication::translate("DashboardStore", "Invalid dashboard identity, name or widget count.");
            return false;
        }
        dashboardIds.insert(dashboard.id);
        const auto &options = dashboard.options;
        if (!QStringList{"grid", "plain", "dusk", "ocean", "image", "desktop"}.contains(options.canvasBackground)
            || options.canvasImagePath.size() > 4096 || options.canvasImagePath.contains(QChar::Null)
            || (options.canvasBackground == "image" && options.canvasImagePath.isEmpty())) {
            error = QCoreApplication::translate("DashboardStore", "Invalid dashboard background settings.");
            return false;
        }
        if (!QStringList{"project", "system"}.contains(options.canvasFileScope)
            || !QStringList{"project", "system"}.contains(options.tilingFileScope)) {
            error = QCoreApplication::translate("DashboardStore", "Invalid dashboard file-manager scope.");
            return false;
        }
        if (dashboard.layout.contains("panelContexts") && !dashboard.layout.value("panelContexts").isObject()) {
            error = QCoreApplication::translate("DashboardStore", "Invalid dashboard repository contexts.");
            return false;
        }
        const auto contexts = dashboard.layout.value("panelContexts").toObject();
        if (contexts.size() > dashboard.panels.size()) {
            error = QCoreApplication::translate("DashboardStore", "Too many dashboard repository contexts.");
            return false;
        }
        for (auto entry = contexts.begin(); entry != contexts.end(); ++entry) {
            const auto panel = std::find_if(dashboard.panels.cbegin(), dashboard.panels.cend(), [&](const auto &value) {
                return value.id == entry.key();
            });
            if (panel == dashboard.panels.cend() || !entry.value().isObject()
                || !QStringList{"files-full", "git-full", "workflows-full", "file-preview", "git-preview",
                    "files", "git", "containers", "terminal", "send-input", "browser"}.contains(panel->kind)) {
                error = QCoreApplication::translate("DashboardStore", "Invalid dashboard repository context identity.");
                return false;
            }
            const auto context = entry.value().toObject();
            for (auto value = context.begin(); value != context.end(); ++value) {
                if (value.key() == "machine") {
                    if (panel->kind == "browser" && value.value().toObject().value("mode") == "follow") {
                        error = QCoreApplication::translate("DashboardStore", "Browser widgets require an independent machine selection.");
                        return false;
                    }
                    if (validMachineContext(value.value()))
                        continue;
                } else if (value.key() == "browserContextId" && panel->kind == "browser") {
                    if (value.value().isString() && validId(value.value().toString()))
                        continue;
                } else if (value.key() == "staged") {
                    if (value.value().isBool())
                        continue;
                } else if ((value.key() == "root" || value.key() == "localRoot" || value.key() == "path") && value.value().isString()) {
                    const QString path = value.value().toString();
                    if (path.size() <= 4096 && !path.contains(QChar::Null)
                        && (value.key() == "path" || path.isEmpty() || QDir::isAbsolutePath(path)))
                        continue;
                }
                error = QCoreApplication::translate("DashboardStore", "Invalid dashboard repository path or preview setting.");
                return false;
            }
        }
        totalPanels += dashboard.panels.size();
        for (const auto &panel : dashboard.panels) {
            if (!validId(panel.id) || panelIds.contains(panel.id) || !kindPattern.match(panel.kind).hasMatch()
                || (panel.environment != "canvas" && panel.environment != "tiling")) {
                error = QCoreApplication::translate("DashboardStore", "Invalid or duplicate dashboard widget identity.");
                return false;
            }
            panelIds.insert(panel.id);
        }
    }
    if (totalPanels > maximumTotalPanels) {
        error = QCoreApplication::translate("DashboardStore", "Up to %1 widgets across all dashboards are supported.").arg(maximumTotalPanels);
        return false;
    }
    return true;
}

bool DashboardStore::load(QVector<DashboardDefinition> &dashboards, QString &error)
{
    error.clear();
    m_loaded = false;
    QByteArray bytes;
    if (!readBytes(bytes, error))
        return false;
    QVector<DashboardDefinition> loaded;
    if (!bytes.isEmpty()) {
        QJsonParseError parseError;
        const auto document = QJsonDocument::fromJson(bytes, &parseError);
        const auto object = document.object();
        if (parseError.error != QJsonParseError::NoError || !document.isObject()
            || (object.value("version") != 1 && object.value("version") != 2) || !object.value("dashboards").isArray()) {
            error = QCoreApplication::translate("DashboardStore", "Dashboard settings are invalid or use an unsupported version. Existing data was kept.");
            return false;
        }
        const auto items = object.value("dashboards").toArray();
        if (items.size() > maximumDashboards) {
            error = QCoreApplication::translate("DashboardStore", "Too many saved dashboards.");
            return false;
        }
        for (const auto &value : items) {
            const auto item = value.toObject();
            if (!value.isObject() || !item.value("panels").isArray() || !item.value("layout").isObject()) {
                error = QCoreApplication::translate("DashboardStore", "Invalid dashboard metadata. Existing data was kept.");
                return false;
            }
            DashboardDefinition definition{item.value("id").toString(), item.value("name").toString(), {}, item.value("layout").toObject()};
            if ((item.contains("options") && !item.value("options").isObject())
                || !readOptions(item.value("options").toObject(), definition.options)) {
                error = QCoreApplication::translate("DashboardStore", "Invalid dashboard options. Existing data was kept.");
                return false;
            }
            const auto panels = item.value("panels").toArray();
            if (panels.size() > maximumPanelsPerDashboard) {
                error = QCoreApplication::translate("DashboardStore", "Too many saved dashboard widgets.");
                return false;
            }
            for (const auto &panelValue : panels) {
                const auto panel = panelValue.toObject();
                QString environment = panel.value("environment").toString();
                if (object.value("version") == 1) {
                    environment = definition.layout.value("mode").toString("canvas");
                    if (panel.value("kind") == "desktop-window")
                        environment = "tiling";
                }
                definition.panels.append({panel.value("id").toString(), panel.value("kind").toString(), environment});
            }
            loaded.append(definition);
        }
    }
    if (!validate(loaded, error))
        return false;
    dashboards = loaded;
    m_revision = revision(bytes);
    m_loaded = true;
    return true;
}

bool DashboardStore::save(const QVector<DashboardDefinition> &dashboards, QString &error)
{
    error.clear();
    if (!m_loaded || !validate(dashboards, error)) {
        if (error.isEmpty())
            error = QCoreApplication::translate("DashboardStore", "Load dashboard settings successfully before changing them.");
        return false;
    }
    QJsonArray items;
    for (const auto &definition : dashboards) {
        QJsonArray panels;
        for (const auto &panel : definition.panels)
            panels.append(QJsonObject{{"id", panel.id}, {"kind", panel.kind}, {"environment", panel.environment}});
        items.append(QJsonObject{{"id", definition.id}, {"name", definition.name}, {"panels", panels},
                                {"layout", definition.layout}, {"options", optionsJson(definition.options)}});
    }
    const auto bytes = QJsonDocument(QJsonObject{{"version", 2}, {"dashboards", items}}).toJson();
    if (bytes.size() > maximumStoreBytes) {
        error = QCoreApplication::translate("DashboardStore", "Dashboard settings exceed the 1 MiB metadata limit.");
        return false;
    }
    QLockFile lock(filePath() + ".lock");
    if (!lock.tryLock()) {
        error = QCoreApplication::translate("DashboardStore", "Dashboard settings are being saved elsewhere. Try again.");
        return false;
    }
    QByteArray current;
    if (!readBytes(current, error))
        return false;
    if (revision(current) != m_revision) {
        error = QCoreApplication::translate("DashboardStore", "Dashboard settings changed outside this window. Restart DockPipe to reload them; no data was overwritten.");
        return false;
    }
    QSaveFile file(filePath());
    if (!file.open(QIODevice::WriteOnly) || file.write(bytes) != bytes.size() || !file.commit()) {
        error = QCoreApplication::translate("DashboardStore", "Could not save dashboards. Your changes remain open; try saving again.");
        return false;
    }
    m_revision = revision(bytes);
    return true;
}
