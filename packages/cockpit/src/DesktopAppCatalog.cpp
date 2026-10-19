#include "DesktopAppCatalog.h"
#include "DesktopApplicationCatalog.h"

QVector<DesktopApplication> DesktopAppCatalog::applications()
{
    QVector<DesktopApplication> result;
#ifdef Q_OS_LINUX
    for (const auto &app : DesktopApplicationCatalog::enumerate())
        result.append({app.name, app.launchTarget, app.windowClass, app.id});
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
