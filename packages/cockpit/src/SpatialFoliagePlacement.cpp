#include "SpatialFoliagePlacement.h"
#include "SpatialInteriorGeometry.h"
#include <cmath>

namespace {
QRectF footprint(const SpatialSolid &solid)
{
    const float angle = solid.yaw * 3.14159265358979323846f / 180;
    const float width = std::abs(std::cos(angle)) * solid.size.x()
        + std::abs(std::sin(angle)) * solid.size.z();
    const float depth = std::abs(std::sin(angle)) * solid.size.x()
        + std::abs(std::cos(angle)) * solid.size.z();
    return {solid.center.x() - width / 2, solid.center.z() - depth / 2, width, depth};
}

QRectF projectedBounds(const QVector<QVector3D> &points)
{
    if (points.isEmpty())
        return {};
    float left = points.first().x(), right = left;
    float top = points.first().z(), bottom = top;
    for (const auto &point : points) {
        left = qMin(left, point.x());
        right = qMax(right, point.x());
        top = qMin(top, point.z());
        bottom = qMax(bottom, point.z());
    }
    // Nonzero footprint also reserves vertical signs and thin mesh walls.
    return {left, top, qMax(1.f, right - left), qMax(1.f, bottom - top)};
}
}

SpatialFoliagePlacement::SpatialFoliagePlacement(const SpatialInterior &room)
    : m_bounds(-room.size.width() / 2, -room.size.height() / 2,
        room.size.width(), room.size.height()), m_height(room.height)
{
    for (int index = 0; index < qMin(room.solids.size(), qsizetype(SpatialInterior::maximumSolids)); ++index) {
        const auto &solid = room.solids[index];
        auto bounds = footprint(solid);
        // Leave five metres around signage/landmarks for approach and reading,
        // not merely enough clearance to avoid touching the sign's plane.
        if (!solid.label.isEmpty())
            bounds.adjust(-500, -500, 500, 500);
        m_obstacles.append(bounds);
    }

    // Per-face bounds preserve the empty centre of a track ribbon. Projecting
    // slopes down to the ground also prevents planting underneath a hillside,
    // even when its coarse movement collider does not cover the visible slope.
    int vertices = 0;
    int faces = 0;
    for (int index = 0; index < qMin(room.meshes.size(), qsizetype(SpatialInterior::maximumMeshes)); ++index) {
        const auto &mesh = room.meshes[index];
        if (vertices + mesh.vertices.size() > SpatialInterior::maximumMeshVertices
            || faces + mesh.faces.size() > SpatialInterior::maximumMeshFaces)
            break;
        vertices += mesh.vertices.size();
        faces += mesh.faces.size();
        for (const auto &face : mesh.faces) {
            if (face.size() < 3 || face.size() > 4)
                continue;
            QVector<QVector3D> points;
            for (int vertex : face) {
                if (vertex < 0 || vertex >= mesh.vertices.size()) {
                    points.clear();
                    break;
                }
                points.append(mesh.vertices[vertex]);
            }
            if (!points.isEmpty())
                m_obstacles.append(projectedBounds(points));
        }
    }
    for (int index = 0; index < qMin(room.rideSurfaces.size(), qsizetype(SpatialRideSurface::maximumSurfaces)); ++index) {
        const auto &surface = room.rideSurfaces[index];
        if (!surface.valid())
            continue;
        // Curved berms do not fit the old straight endpoint rectangle. Reserve
        // the actual bounded footprint so foliage cannot grow through a turn.
        m_obstacles.append(projectedBounds(surface.mesh().vertices));
    }
    for (int index = 0; index < qMin(room.displays.size(), qsizetype(SpatialWorkspace::maximumDisplays)); ++index)
        m_obstacles.append(projectedBounds(SpatialInteriorGeometry::surface(room, room.displays[index])));
    for (int index = 0; index < qMin(room.actions.size(), qsizetype(SpatialInterior::maximumActions)); ++index) {
        const auto &action = room.actions[index];
        m_obstacles.append(footprint({action.center, action.size, {}, true, SpatialSolidShape::Box, {}, action.yaw}));
    }
    for (int index = 0; index < qMin(room.vehicles.size(), qsizetype(SpatialVehicleModel::maximumVehicles)); ++index) {
        const auto &vehicle = room.vehicles[index];
        m_obstacles.append(footprint({vehicle.position, SpatialVehicleModel::size(vehicle), {}, true,
            SpatialSolidShape::Box, {}, -vehicle.heading * 57.29578f}));
    }
    for (const auto &point : {room.spawn, room.exit})
        m_obstacles.append({point.x() - 150, point.y() - 150, 300, 300});
}

QString SpatialFoliagePlacement::error(const SpatialFoliage &plant) const
{
    if (!std::isfinite(plant.base.x()) || !std::isfinite(plant.base.y()) || !std::isfinite(plant.base.z())
        || !std::isfinite(plant.height) || !std::isfinite(plant.width)
        || plant.height < 50 || plant.height > 2400 || plant.width < 50 || plant.width > 2400)
        return "Invalid foliage dimensions";
    // Sloped planting needs terrain-aligned art and shadows. Until supported,
    // only clear flat ground is admissible, never a buried or floating tree.
    if (plant.base.y() != 0)
        return "Foliage requires clear flat ground";
    if (m_admitted >= SpatialInterior::maximumFoliage)
        return "Foliage budget reached";
    const auto bounds = footprint(envelope(plant));
    if (!m_bounds.contains(bounds) || plant.height > m_height)
        return "Keep the entire tree inside the environment";
    for (const auto &obstacle : m_obstacles) {
        if (bounds.intersects(obstacle))
            return "Tree overlaps scenery, terrain or a reserved route";
    }
    return {};
}

bool SpatialFoliagePlacement::admit(const SpatialFoliage &plant)
{
    if (!error(plant).isEmpty())
        return false;
    m_obstacles.append(footprint(envelope(plant)));
    ++m_admitted;
    return true;
}

SpatialSolid SpatialFoliagePlacement::envelope(const SpatialFoliage &plant)
{
    // The quad turns with the camera. Its full swept width plus 50 cm of
    // clearance must stay free at every view angle, not only at generation.
    return {plant.base + QVector3D(0, plant.height / 2, 0),
        {plant.width + 100, plant.height, plant.width + 100}, {}, true};
}

SpatialSolid SpatialFoliagePlacement::trunk(const SpatialFoliage &plant)
{
    // Physical trunk only; never submitted as a visible brown box.
    return {plant.base + QVector3D(0, 100, 0), {45, 200, 45}, {}, true};
}
