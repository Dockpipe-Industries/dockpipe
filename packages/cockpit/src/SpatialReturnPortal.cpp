#include "SpatialReturnPortal.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialWorkspace.h"
#include <cmath>

namespace {
bool finite(const QVector3D &point)
{
    return std::isfinite(point.x()) && std::isfinite(point.y()) && std::isfinite(point.z());
}
}

QVector3D SpatialReturnPortal::center(const SpatialInterior &room)
{
    QVector3D position(room.exit.x(), room.spawnHeight, room.exit.y());
    position.setY(SpatialInteriorGeometry::walkingHeight(room, position));
    return position;
}

void SpatialReturnPortal::place(SpatialInterior &room, const QVector3D &arrival, float heading)
{
    // Portal radius plus the existing 25 cm walking-body clearance keeps the
    // opening inside room bounds. Search is finite: four rings, eight bearings.
    constexpr float margin = interiorRadius + 25;
    const float halfWidth = room.size.width() / 2 - margin;
    const float halfDepth = room.size.height() / 2 - margin;
    if (!finite(arrival) || !std::isfinite(heading) || !std::isfinite(halfWidth)
        || !std::isfinite(halfDepth) || halfWidth <= 0 || halfDepth <= 0) {
        room.returnPortalWalkEntry = false;
        return;
    }

    QPointF best = room.exit;
    float bestDistance = -1;
    const auto consider = [&](QPointF point) {
        if (!std::isfinite(point.x()) || !std::isfinite(point.y()))
            return false;
        point.setX(qBound(-halfWidth, float(point.x()), halfWidth));
        point.setY(qBound(-halfDepth, float(point.y()), halfDepth));
        QVector3D position(point.x(), arrival.y(), point.y());
        position.setY(SpatialInteriorGeometry::walkingHeight(room, position));
        if (SpatialInteriorGeometry::blocked(room, position))
            return false;
        const float distance = std::hypot(position.x() - arrival.x(), position.z() - arrival.z());
        if (distance > bestDistance) {
            best = point;
            bestDistance = distance;
        }
        return distance >= arrivalClearance;
    };
    if (consider(room.exit)) {
        room.exit = best;
        room.returnPortalWalkEntry = true;
        return;
    }

    constexpr float pi = 3.14159265358979323846f;
    constexpr float bearings[]{pi, pi * .75f, -pi * .75f, pi * .5f, -pi * .5f, pi * .25f, -pi * .25f, 0};
    for (float distance : {500.f, 750.f, 1000.f, arrivalClearance}) {
        for (float bearing : bearings) {
            const float angle = heading + bearing;
            const QPointF point(arrival.x() + std::sin(angle) * distance,
                arrival.z() + std::cos(angle) * distance);
            if (consider(point)) {
                room.exit = best;
                room.returnPortalWalkEntry = true;
                return;
            }
        }
    }
    // Small or crowded rooms retain the farthest clear candidate and require
    // explicit navigation instead of allowing an accidental walk-through exit.
    room.exit = best;
    room.returnPortalWalkEntry = false;
}

bool SpatialReturnPortal::entered(const QVector3D &previous, const QVector3D &position,
    const QVector3D &portal, float radius)
{
    if (!finite(previous) || !finite(position) || !finite(portal) || !std::isfinite(radius) || radius <= 0)
        return false;
    const float radiusSquared = radius * radius;
    if (!std::isfinite(radiusSquared) || (previous - portal).lengthSquared() <= radiusSquared)
        return false;
    const auto movement = position - previous;
    const float travelSquared = movement.lengthSquared();
    if (!std::isfinite(travelSquared) || travelSquared <= .000001f)
        return false;
    const float fraction = qBound(0.f, QVector3D::dotProduct(portal - previous, movement) / travelSquared, 1.f);
    const auto nearest = previous + movement * fraction;
    return (nearest - portal).lengthSquared() <= radiusSquared;
}
