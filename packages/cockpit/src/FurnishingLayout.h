#pragma once
#include "FurnishingCatalog.h"
#include <QByteArray>

struct FurnishingHit {
    bool hit = false;
    QVector3D point;
    QVector3D normal;
    float distance = 2500;
};
// Separate user overlay; generated room data never becomes the saved layout.
class FurnishingLayout {
public:
    static constexpr int maximumObjects = 192;
    static constexpr int maximumParts = 768;
    static constexpr int maximumBytes = 512 * 1024;
    QVector<FurnishingObject> objects;
    QString environment;
    bool load(const QString &location, QString &error);
    bool save(QString &error);
    static bool validObject(const FurnishingObject &object);
    static FurnishingHit target(const SpatialInterior &room, const QVector3D &origin, const QVector3D &direction);
    static float rayObject(const FurnishingObject &object, const QVector3D &origin, const QVector3D &direction);
    static QString placementError(const FurnishingObject &object, const SpatialInterior &base,
        const QVector<FurnishingObject> &others, const FurnishingHit &hit);
private:
    QString m_path;
    QByteArray m_revision;
    bool m_loaded = false;
};
