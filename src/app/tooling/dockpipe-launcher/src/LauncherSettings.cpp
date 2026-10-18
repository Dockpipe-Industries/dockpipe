#include "LauncherSettings.h"
#include "ContextStore.h"

#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonParseError>
#include <QLockFile>
#include <QSaveFile>

namespace {
// Launcher preferences are small structured configuration, not an artifact
// store. Allow ample room for bounded browser permissions and existing roots.
constexpr qint64 maximumSettingsBytes = 4 * 1024 * 1024;

bool readSettingsObject(QJsonObject &settings)
{
    QFile file(LauncherSettings::filePath());
    if (!file.exists())
        return true;
    if (!file.open(QIODevice::ReadOnly) || file.size() > maximumSettingsBytes)
        return false;
    QJsonParseError error;
    const QJsonDocument document = QJsonDocument::fromJson(file.read(maximumSettingsBytes + 1), &error);
    if (error.error != QJsonParseError::NoError || !document.isObject())
        return false;
    settings = document.object();
    return true;
}

bool writeSettingsObject(const QJsonObject &settings)
{
    const QByteArray bytes = QJsonDocument(settings).toJson(QJsonDocument::Indented);
    if (bytes.size() > maximumSettingsBytes)
        return false;
    QSaveFile file(LauncherSettings::filePath());
    if (!file.open(QIODevice::WriteOnly))
        return false;
    return file.write(bytes) == bytes.size() && file.commit();
}
}

QString LauncherSettings::filePath()
{
    return ContextStore::configDir() + QStringLiteral("/launcher.json");
}

bool LauncherSettings::load()
{
    uiMode = QStringLiteral("basic");
    basicView = QStringLiteral("icons");
    projectFolder.clear();
    repoRootOverride.clear();
    globalRootOverride.clear();
    extraWorkflowRoots.clear();
    packageRemotes = {QStringLiteral("https://packages.dockpipe.com")};
    thirdPartyDisclaimerDismissed = false;
    sidebarVisible = true;
    defaultDashboardVisible = true;
    cockpit = {};
    recentProjectFolders.clear();

    QFile f(filePath());
    if (!f.exists() || !f.open(QIODevice::ReadOnly))
        return true;
    const QJsonDocument doc = QJsonDocument::fromJson(f.readAll());
    f.close();
    if (!doc.isObject())
        return true;
    const QJsonObject o = doc.object();
    const auto cockpitObject = o.value("cockpit").toObject();
    cockpit.enabled = cockpitObject.value("enabled").toBool(false);
    cockpit.appControl = cockpitObject.value("appControl").toBool(false);
    cockpit.orbitalMotion = cockpitObject.value("orbitalMotion").toBool(true);
    cockpit.controller = cockpitObject.value("controller").toBool(false);
    cockpit.showShip = cockpitObject.value("showShip").toBool(true);
    // Voice requires a fresh opt-in after moving behind the experimental gate.
    // The former top-level preference must not silently enable the experiment.
    cockpit.voiceCommands = cockpitObject.value("experimental").toObject().value("voiceCommands").toBool(false);
    cockpit.hardwareAcceleration = cockpitObject.value("hardwareAcceleration").toBool(true);
    sidebarVisible = o.value(QStringLiteral("sidebarVisible")).toBool(true);
    defaultDashboardVisible = o.value(QStringLiteral("defaultDashboardVisible")).toBool(true);
    const QString um = o.value(QStringLiteral("uiMode")).toString();
    if (um == QStringLiteral("basic") || um == QStringLiteral("advanced"))
        uiMode = um;
    const QString bv = o.value(QStringLiteral("basicView")).toString();
    if (bv == QStringLiteral("icons") || bv == QStringLiteral("list"))
        basicView = bv;
    projectFolder = o.value(QStringLiteral("projectFolder")).toString();
    repoRootOverride = o.value(QStringLiteral("repoRootOverride")).toString();
    globalRootOverride = o.value(QStringLiteral("globalRootOverride")).toString();
    if (o.contains(QStringLiteral("thirdPartyDisclaimerDismissed")))
        thirdPartyDisclaimerDismissed = o.value(QStringLiteral("thirdPartyDisclaimerDismissed")).toBool();
    if (o.contains(QStringLiteral("extraWorkflowRoots")) && o.value(QStringLiteral("extraWorkflowRoots")).isArray()) {
        const QJsonArray a = o.value(QStringLiteral("extraWorkflowRoots")).toArray();
        for (const QJsonValue &v : a) {
            const QString p = v.toString().trimmed();
            if (!p.isEmpty())
                extraWorkflowRoots.append(p);
        }
    }
    if (o.contains(QStringLiteral("packageRemotes")) && o.value(QStringLiteral("packageRemotes")).isArray()) {
        packageRemotes.clear();
        const QJsonArray a = o.value(QStringLiteral("packageRemotes")).toArray();
        for (const QJsonValue &v : a) {
            const QString p = v.toString().trimmed();
            if (!p.isEmpty())
                packageRemotes.append(p);
        }
    }
    if (o.contains(QStringLiteral("recentProjectFolders")) && o.value(QStringLiteral("recentProjectFolders")).isArray()) {
        const QJsonArray a = o.value(QStringLiteral("recentProjectFolders")).toArray();
        for (const QJsonValue &v : a) {
            const QString p = v.toString();
            if (!p.isEmpty())
                recentProjectFolders.append(p);
        }
        while (recentProjectFolders.size() > kMaxRecentProjects)
            recentProjectFolders.removeLast();
    }
    if (!projectFolder.isEmpty() && recentProjectFolders.isEmpty()) {
        recentProjectFolders.prepend(QDir::cleanPath(projectFolder));
    }
    return true;
}

void LauncherSettings::addRecentProject(const QString &path)
{
    if (path.isEmpty())
        return;
    const QString c = QDir::cleanPath(path);
    recentProjectFolders.removeAll(c);
    recentProjectFolders.prepend(c);
    while (recentProjectFolders.size() > kMaxRecentProjects)
        recentProjectFolders.removeLast();
}

bool LauncherSettings::save() const
{
    QLockFile lock(filePath() + QStringLiteral(".lock"));
    if (!lock.tryLock())
        return false;
    QJsonObject o;
    if (!readSettingsObject(o))
        return false;
    o.insert(QStringLiteral("version"), 1);
    o.insert("cockpit", QJsonObject{{"enabled", cockpit.enabled}, {"appControl", cockpit.appControl},
        {"orbitalMotion", cockpit.orbitalMotion}, {"controller", cockpit.controller},
        {"showShip", cockpit.showShip}, {"experimental", QJsonObject{{"voiceCommands", cockpit.voiceCommands}}},
        {"hardwareAcceleration", cockpit.hardwareAcceleration}});
    o.insert(QStringLiteral("sidebarVisible"), sidebarVisible);
    o.insert(QStringLiteral("defaultDashboardVisible"), defaultDashboardVisible);
    o.insert(QStringLiteral("uiMode"), uiMode);
    o.insert(QStringLiteral("basicView"), basicView);
    o.insert(QStringLiteral("projectFolder"), projectFolder);
    o.insert(QStringLiteral("repoRootOverride"), repoRootOverride);
    o.insert(QStringLiteral("globalRootOverride"), globalRootOverride);
    o.insert(QStringLiteral("thirdPartyDisclaimerDismissed"), thirdPartyDisclaimerDismissed);
    {
        QJsonArray a;
        for (const QString &p : extraWorkflowRoots)
            a.append(p);
        o.insert(QStringLiteral("extraWorkflowRoots"), a);
    }
    {
        QJsonArray a;
        for (const QString &p : packageRemotes)
            a.append(p);
        o.insert(QStringLiteral("packageRemotes"), a);
    }
    {
        QJsonArray a;
        for (const QString &p : recentProjectFolders)
            a.append(p);
        o.insert(QStringLiteral("recentProjectFolders"), a);
    }

    return writeSettingsObject(o);
}

bool LauncherSettings::loadBrowserSettings(QJsonObject &settings)
{
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue browser = root.value(QStringLiteral("browser"));
    if (!browser.isUndefined() && !browser.isObject())
        return false;
    settings = browser.toObject();
    return true;
}

bool LauncherSettings::saveBrowserSettings(const QJsonObject &settings)
{
    QLockFile lock(filePath() + QStringLiteral(".lock"));
    if (!lock.tryLock())
        return false;
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue previous = root.value(QStringLiteral("browser"));
    if (!previous.isUndefined() && !previous.isObject())
        return false;
    root.insert(QStringLiteral("browser"), settings);
    return writeSettingsObject(root);
}

bool LauncherSettings::loadTerminalSettings(QJsonObject &settings)
{
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue terminal = root.value(QStringLiteral("terminal"));
    if (!terminal.isUndefined() && !terminal.isObject())
        return false;
    settings = terminal.toObject();
    return true;
}

bool LauncherSettings::saveTerminalSettings(const QJsonObject &settings)
{
    QLockFile lock(filePath() + QStringLiteral(".lock"));
    if (!lock.tryLock())
        return false;
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue previous = root.value(QStringLiteral("terminal"));
    if (!previous.isUndefined() && !previous.isObject())
        return false;
    root.insert(QStringLiteral("terminal"), settings);
    return writeSettingsObject(root);
}

LauncherSettings LauncherSettings::current()
{
    LauncherSettings s;
    s.load();
    return s;
}

bool LauncherSettings::loadDashboardTourState(bool &completed, bool &doNotShowAgain)
{
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue value = root.value("dashboardTour");
    if (!value.isUndefined() && !value.isObject())
        return false;
    const QJsonObject state = value.toObject();
    for (const QString &key : {QString("completed"), QString("doNotShowAgain")}) {
        if (state.contains(key) && !state.value(key).isBool())
            return false;
    }
    completed = state.value("completed").toBool();
    doNotShowAgain = state.value("doNotShowAgain").toBool();
    return true;
}

bool LauncherSettings::saveDashboardTourState(bool completed, bool doNotShowAgain)
{
    QLockFile lock(filePath() + QStringLiteral(".lock"));
    if (!lock.tryLock())
        return false;
    QJsonObject root;
    if (!readSettingsObject(root))
        return false;
    const QJsonValue previous = root.value("dashboardTour");
    if (!previous.isUndefined() && !previous.isObject())
        return false;
    QJsonObject state = previous.toObject();
    for (const QString &key : {QString("completed"), QString("doNotShowAgain")}) {
        if (state.contains(key) && !state.value(key).isBool())
            return false;
    }
    state.insert("completed", completed || state.value("completed").toBool());
    state.insert("doNotShowAgain", doNotShowAgain);
    root.insert("dashboardTour", state);
    return writeSettingsObject(root);
}
