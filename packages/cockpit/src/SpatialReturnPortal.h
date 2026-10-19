#pragma once
#include <QPointF>
#include <QVector3D>

struct SpatialInterior;

namespace SpatialReturnPortal {
constexpr float interiorRadius = 65;
constexpr float arrivalClearance = 400; // Four-metre clear arrival apron.
void place(SpatialInterior &room, const QVector3D &arrival, float heading);
QVector3D center(const SpatialInterior &room);
bool entered(const QVector3D &previous, const QVector3D &position,
    const QVector3D &portal, float radius);
}
