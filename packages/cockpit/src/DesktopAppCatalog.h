#pragma once
#include <QString>
#include "SpatialWorkspace.h"
#include "DesktopAppWindow.h"
#include <QVector>

struct DesktopApplication {
    QString name;
    QString desktopFile;
    QString windowClass;
    QString desktopId;
};

class DesktopAppCatalog final {
public:
    // Bounded local desktop metadata only. GIO owns Exec-field interpretation.
    static QVector<DesktopApplication> applications();
    static SpatialApplicationReference reference(const DesktopAppChoice &window, const QVector<DesktopApplication> &catalog);
};
