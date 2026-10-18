#pragma once
#include "SpatialWorkspace.h"
#include <QVector3D>

namespace SpatialInteriorGeometry {
QVector<QVector3D> surface(const SpatialInterior &room, const SpatialDisplay &display);
QString target(const SpatialInterior &room, const QVector3D &origin, const QVector3D &direction);
float walkingHeight(const SpatialInterior &room, const QVector3D &position);
bool blocked(const SpatialInterior &room, const QVector3D &position);
QString approachedPortal(const SpatialInterior &room, const QVector3D &previous,
    const QVector3D &position, const QVector3D &lookDirection);
}
