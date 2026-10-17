#include "SpatialInteriorGeometry.h"
#include <cmath>

QVector<QVector3D> SpatialInteriorGeometry::surface(const SpatialInterior &room, const SpatialDisplay &display)
{
    const float left = display.position.x() - display.size.width() / 2;
    const float right = display.position.x() + display.size.width() / 2;
    const float bottom = display.position.y() - display.size.height() / 2;
    const float top = display.position.y() + display.size.height() / 2;
    const float x = room.size.width() / 2 - 2;
    const float z = room.size.height() / 2 - 2;
    if (display.wall == "north")
        return {{left, top, z}, {right, top, z}, {right, bottom, z}, {left, bottom, z}};
    if (display.wall == "south")
        return {{-left, top, -z}, {-right, top, -z}, {-right, bottom, -z}, {-left, bottom, -z}};
    if (display.wall == "east")
        return {{x, top, -left}, {x, top, -right}, {x, bottom, -right}, {x, bottom, -left}};
    return {{-x, top, left}, {-x, top, right}, {-x, bottom, right}, {-x, bottom, left}};
}

QString SpatialInteriorGeometry::target(const SpatialInterior &room, const QVector3D &origin, const QVector3D &direction)
{
    qreal nearest = 10000;
    QString result;
    for (const auto &display : room.displays) {
        const auto corners = surface(room, display);
        const auto horizontal = corners[1] - corners[0];
        const auto vertical = corners[3] - corners[0];
        const auto normal = QVector3D::crossProduct(horizontal, vertical).normalized();
        const qreal denominator = QVector3D::dotProduct(normal, direction);
        if (std::abs(denominator) < 0.0001)
            continue;
        const qreal distance = QVector3D::dotProduct(normal, corners[0] - origin) / denominator;
        if (distance <= 0 || distance >= nearest)
            continue;
        const auto offset = origin + direction * distance - corners[0];
        const qreal u = QVector3D::dotProduct(offset, horizontal) / horizontal.lengthSquared();
        const qreal v = QVector3D::dotProduct(offset, vertical) / vertical.lengthSquared();
        if (u >= 0 && u <= 1 && v >= 0 && v <= 1) {
            nearest = distance;
            result = display.id;
        }
    }
    return result;
}

bool SpatialInteriorGeometry::blocked(const SpatialInterior &room, const QVector3D &position)
{
    // A 25-unit camera radius keeps walking inside the room; movement slides by axis.
    return std::abs(position.x()) > room.size.width() / 2 - 25
        || std::abs(position.z()) > room.size.height() / 2 - 25;
}
