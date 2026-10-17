#pragma once
#include "SpatialWorkspace.h"
#include <QVector3D>

namespace SpatialInteriorGeometry {
QVector<QVector3D> surface(const SpatialInterior &room, const SpatialDisplay &display);
QString target(const SpatialInterior &room, const QVector3D &origin, const QVector3D &direction);
bool blocked(const SpatialInterior &room, const QVector3D &position);
}
