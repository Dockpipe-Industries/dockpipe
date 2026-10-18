#pragma once

#include "SpatialWorkspace.h"
#include <QJsonObject>
#include <QMap>
#include <QImage>

struct SpatialOperationsScene {
    SpatialInterior interior;
    QMap<QString, QImage> signs;
    QString notice;
    int page = 0;
    int pages = 1;
};

// Consumes bounded, read-only launcher snapshots. No discovery or commands here.
class SpatialOperationsWorld {
public:
    static constexpr int pageSize = 12;
    static constexpr int maximumContextNodes = 8;
    static SpatialOperationsScene campus(const QString &section, const QJsonObject &snapshot, int page);
    static SpatialOperationsScene laboratory(const QJsonObject &machine, bool inside,
        const QString &nickname, const QColor &color, int body);
    static QVector<QJsonObject> machines(const QJsonObject &snapshot);
};
