#include "DesktopAppCatalog.h"
#include <QDirIterator>
#include <QFileInfo>
#include <QSet>
#include <QSettings>
#include <QStandardPaths>
#include <algorithm>

QVector<DesktopApplication> DesktopAppCatalog::applications()
{
    QVector<DesktopApplication> result;
#if defined(Q_OS_LINUX)
    QSet<QString> seen;
    int examined = 0;
    qint64 metadataBytes = 0;
    const auto roots = QStandardPaths::standardLocations(QStandardPaths::ApplicationsLocation);
    for (const QString &root : roots) {
        QDirIterator entries(root, {"*.desktop"}, QDir::Files, QDirIterator::Subdirectories);
        while (entries.hasNext()) {
            // Bound synchronous inventory to one UI-sized catalog. No file contents,
            // thumbnails, executable parsing or recursive symlink traversal.
            if (examined++ >= 1000 || metadataBytes >= 8 * 1024 * 1024 || result.size() >= 500)
                break;
            const QString path = entries.next();
            const QFileInfo info(path);
            const QString id = QDir(root).relativeFilePath(path).replace('/', '-');
            if (seen.contains(id))
                continue;
            seen.insert(id);
            if (info.size() > 128 * 1024 || info.size() < 0)
                continue;
            metadataBytes += info.size();
            QSettings desktop(path, QSettings::IniFormat);
            desktop.beginGroup("Desktop Entry");
            if (desktop.value("Type").toString() != "Application" || desktop.value("Hidden", false).toBool()
                || desktop.value("NoDisplay", false).toBool())
                continue;
            const QString name = desktop.value("Name").toString().left(180);
            if (name.isEmpty())
                continue;
            result.append({name, path, desktop.value("StartupWMClass", info.completeBaseName()).toString().left(180), id});
        }
    }
    std::sort(result.begin(), result.end(), [](const auto &left, const auto &right) {
        return QString::localeAwareCompare(left.name, right.name) < 0;
    });
#endif
    return result;
}

SpatialApplicationReference DesktopAppCatalog::reference(const DesktopAppChoice &window, const QVector<DesktopApplication> &catalog)
{
    const DesktopApplication *match = nullptr;
    for (const auto &app : catalog) {
        if (app.windowClass.compare(window.applicationClass, Qt::CaseInsensitive) != 0)
            continue;
        if (match)
            return {{}, window.applicationClass, window.applicationClass};
        match = &app;
    }
    if (match)
        return {match->desktopId, match->windowClass, match->name};
    return {{}, window.applicationClass, window.applicationClass};
}
