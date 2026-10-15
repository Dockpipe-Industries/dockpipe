#include "LauncherSettings.h"
#include "ContextStore.h"

#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QSaveFile>

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
    cockpit.voiceCommands = cockpitObject.value("voiceCommands").toBool(false);
    cockpit.hardwareAcceleration = cockpitObject.value("hardwareAcceleration").toBool(true);
    sidebarVisible = o.value(QStringLiteral("sidebarVisible")).toBool(true);
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
    QJsonObject o;
    o.insert(QStringLiteral("version"), 1);
    o.insert("cockpit", QJsonObject{{"enabled", cockpit.enabled}, {"appControl", cockpit.appControl},
        {"orbitalMotion", cockpit.orbitalMotion}, {"controller", cockpit.controller},
        {"showShip", cockpit.showShip}, {"voiceCommands", cockpit.voiceCommands},
        {"hardwareAcceleration", cockpit.hardwareAcceleration}});
    o.insert(QStringLiteral("sidebarVisible"), sidebarVisible);
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

    QSaveFile f(filePath());
    if (!f.open(QIODevice::WriteOnly))
        return false;
    const auto bytes = QJsonDocument(o).toJson(QJsonDocument::Indented);
    return f.write(bytes) == bytes.size() && f.commit();
}

LauncherSettings LauncherSettings::current()
{
    LauncherSettings s;
    s.load();
    return s;
}
