#include "SpatialInteriorGeometry.h"
#include "SpatialFoliagePlacement.h"
#include "DirtBikeDynamics.h"
#include <cmath>

namespace {
QVector3D localVector(const QVector3D &point, float yaw)
{
    const float angle = yaw * 3.14159265358979323846f / 180;
    return {point.x() * std::cos(angle) + point.z() * std::sin(angle), point.y(),
        -point.x() * std::sin(angle) + point.z() * std::cos(angle)};
}
}

QVector<QVector3D> SpatialInteriorGeometry::surface(const SpatialInterior &room, const SpatialDisplay &display)
{
    if (display.freestanding) {
        const float radians = display.yaw * 3.14159265358979323846 / 180;
        const QVector3D right(std::cos(radians), 0, std::sin(radians));
        const QVector3D horizontal = right * display.size.width() / 2;
        const QVector3D vertical(0, display.size.height() / 2, 0);
        return {display.center - horizontal + vertical, display.center + horizontal + vertical,
            display.center + horizontal - vertical, display.center - horizontal - vertical};
    }
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
    // Furniture occludes interaction as well as pixels. A near/far slab test
    // bounds each ray against the cached axis-aligned procedural solids.
    for (const auto &solid : room.solids) {
        const auto rayOrigin = localVector(origin - solid.center, solid.yaw);
        const auto rayDirection = localVector(direction, solid.yaw);
        const auto low = -solid.size / 2;
        const auto high = solid.size / 2;
        qreal near = 0;
        qreal far = nearest;
        bool intersects = true;
        for (int axis = 0; axis < 3; ++axis) {
            if (std::abs(rayDirection[axis]) < 0.0001) {
                if (rayOrigin[axis] < low[axis] || rayOrigin[axis] > high[axis]) {
                    intersects = false;
                    break;
                }
                continue;
            }
            qreal entry = (low[axis] - rayOrigin[axis]) / rayDirection[axis];
            qreal exit = (high[axis] - rayOrigin[axis]) / rayDirection[axis];
            if (entry > exit)
                std::swap(entry, exit);
            near = qMax(near, entry);
            far = qMin(far, exit);
            if (near > far) {
                intersects = false;
                break;
            }
        }
        if (intersects && near > 0)
            nearest = qMin(nearest, near);
    }
    const auto occludeMesh = [&](const SpatialMesh &mesh) {
        for (const auto &face : mesh.faces) {
            if (face.size() < 3 || face.size() > 4)
                continue;
            bool valid = true;
            for (int index : face)
                valid = valid && index >= 0 && index < mesh.vertices.size();
            if (!valid)
                continue;
            // Triangulate convex quads for picking; geometry remains decorative
            // for movement until mesh collision is implemented separately.
            for (int corner = 1; corner + 1 < face.size(); ++corner) {
                const auto first = mesh.vertices[face[0]];
                const auto edgeOne = mesh.vertices[face[corner]] - first;
                const auto edgeTwo = mesh.vertices[face[corner + 1]] - first;
                const auto cross = QVector3D::crossProduct(direction, edgeTwo);
                const float determinant = QVector3D::dotProduct(edgeOne, cross);
                if (std::abs(determinant) < 0.0001)
                    continue;
                const auto offset = origin - first;
                const float u = QVector3D::dotProduct(offset, cross) / determinant;
                const auto secondCross = QVector3D::crossProduct(offset, edgeOne);
                const float v = QVector3D::dotProduct(direction, secondCross) / determinant;
                const float distance = QVector3D::dotProduct(edgeTwo, secondCross) / determinant;
                if (u >= -0.0001 && v >= -0.0001 && u + v <= 1.0001 && distance > 0 && distance < nearest)
                    nearest = distance;
            }
        }
    };
    int faceBudget = SpatialInterior::maximumMeshFaces;
    int vertexBudget = SpatialInterior::maximumMeshVertices;
    for (int meshIndex = 0; meshIndex < qMin(room.meshes.size(), qsizetype(SpatialInterior::maximumMeshes)); ++meshIndex) {
        const auto &mesh = room.meshes[meshIndex];
        if (mesh.vertices.size() > vertexBudget || mesh.faces.size() > faceBudget)
            break;
        vertexBudget -= mesh.vertices.size();
        faceBudget -= mesh.faces.size();
        occludeMesh(mesh);
    }
    for (int index = 0; index < qMin(int(room.rideSurfaces.size()), SpatialRideSurface::maximumSurfaces); ++index)
        occludeMesh(room.rideSurfaces[index].mesh());
    for (const auto &action : room.actions) {
        const auto offset = action.center - origin;
        const float along = QVector3D::dotProduct(offset, direction);
        const float radius = qMin(85.f, action.size.length() / 3);
        const float distance = along;
        if (offset.length() <= 340 && along > 0 && distance < nearest
            && (offset - direction * along).length() <= radius) {
            nearest = distance;
            result = "action:" + action.id;
        }
    }
    for (const auto &vehicle : room.vehicles) {
        const float distance = SpatialVehicleModel::rayDistance(vehicle, origin, direction);
        if (distance >= 0 && distance < nearest) {
            nearest = distance;
            result = "vehicle:" + vehicle.id;
        }
    }
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

QString SpatialInteriorGeometry::approachedPortal(const SpatialInterior &room, const QVector3D &previous,
    const QVector3D &position, const QVector3D &lookDirection)
{
    for (const auto &display : room.displays) {
        if (!display.portal || !display.freestanding || display.portalApproachDistance <= 0)
            continue;
        const float radians = display.yaw * 3.14159265358979323846 / 180;
        const QVector3D outward(std::sin(radians), 0, -std::cos(radians));
        const QVector3D right(std::cos(radians), 0, std::sin(radians));
        const auto offset = position - display.portalCenter;
        const qreal distance = QVector3D::dotProduct(offset, outward);
        const qreal previousDistance = QVector3D::dotProduct(previous - display.portalCenter, outward);
        // Enter only while approaching and facing the stall, never when walking
        // past it along the aisle, backing away or turning in place.
        if (distance >= -25 && distance < display.portalApproachDistance
            && previousDistance > distance + 0.01
            && std::abs(QVector3D::dotProduct(offset, right)) < display.size.width() / 2 - 15
            && std::abs(offset.y()) < qMax(100.0, display.size.height() / 2)
            && QVector3D::dotProduct(lookDirection, -outward) > 0.6)
            return display.id;
    }
    return {};
}

float SpatialInteriorGeometry::walkingHeight(const SpatialInterior &room, const QVector3D &position)
{
    float floor = DirtBikeDynamics::groundHeight(room, position);
    for (const auto &surface : room.walkSurfaces) {
        if (surface.bounds.contains(QPointF(position.x(), position.z()))
            && surface.height <= position.y() - 120 && surface.height > floor)
            floor = surface.height;
    }
    return floor + 160;
}

bool SpatialInteriorGeometry::blocked(const SpatialInterior &room, const QVector3D &position)
{
    for (int index = 0; index < qMin(room.foliage.size(), qsizetype(SpatialInterior::maximumFoliage)); ++index) {
        const auto trunk = SpatialFoliagePlacement::trunk(room.foliage[index]);
        const auto offset = position - trunk.center;
        if (position.y() - (room.locomotion == "fly" ? 60 : 160) < trunk.center.y() + trunk.size.y() / 2
            && position.y() + 25 > trunk.center.y() - trunk.size.y() / 2
            && std::abs(offset.x()) < trunk.size.x() / 2 + 25
            && std::abs(offset.z()) < trunk.size.z() / 2 + 25)
            return true;
    }
    for (const auto &solid : room.solids) {
        // Feet clear low trim; the walking body ends 25 units above eye level.
        if (!solid.collision || solid.center.y() + solid.size.y() / 2 < 30
            || solid.center.y() + solid.size.y() / 2 <= position.y() - (room.locomotion == "fly" ? 60 : 160) + 1
            || solid.center.y() - solid.size.y() / 2 >= position.y() + 25)
            continue;
        const auto offset = localVector(position - solid.center, solid.yaw);
        if (std::abs(offset.x()) < solid.size.x() / 2 + 25
            && std::abs(offset.z()) < solid.size.z() / 2 + 25)
            return true;
    }
    for (const auto &vehicle : room.vehicles) {
        if (SpatialVehicleModel::contains(vehicle, position, 25))
            return true;
    }
    // A 25-unit camera radius keeps walking inside the room; movement slides by axis.
    return std::abs(position.x()) > room.size.width() / 2 - 25
        || std::abs(position.z()) > room.size.height() / 2 - 25;
}
