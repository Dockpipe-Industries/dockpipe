#include "SpatialWebPassage.h"
#include "SpatialInteriorGeometry.h"
#include <algorithm>
#include <cmath>

namespace {
QVector3D rotate(const QVector3D &point, float degrees)
{
    const float radians = degrees * 3.14159265358979323846f / 180;
    return {point.x() * std::cos(radians) - point.z() * std::sin(radians), point.y(),
        point.x() * std::sin(radians) + point.z() * std::cos(radians)};
}
}

bool SpatialWebPassage::begin(const SpatialInterior &source, const SpatialDisplay &entrance)
{
    if (source.locomotion != "walk" || source.streaming || !entrance.portal || !entrance.freestanding
        || source.solids.size() > SpatialInterior::maximumSolids || source.displays.size() > 12
        || !std::isfinite(entrance.yaw) || !std::isfinite(entrance.portalCenter.x())
        || !std::isfinite(entrance.portalCenter.z()))
        return false;
    m_source = source;
    m_destination.reset();
    m_failed = false;
    m_segments = 2;
    m_yaw = entrance.yaw;
    m_origin = entrance.portalCenter;
    m_origin.setY(0);
    // The selected facade becomes an open doorway. Remove its backing/counter,
    // never arbitrary geometry elsewhere in the source area.
    m_source.solids.erase(std::remove_if(m_source.solids.begin(), m_source.solids.end(), [this](const SpatialSolid &solid) {
        const auto local = rotate(solid.center - m_origin, -m_yaw);
        return std::abs(local.x()) < 320 && local.z() > -20 && local.z() < segmentLength * 3;
    }), m_source.solids.end());
    for (auto &display : m_source.displays) {
        display.portal = false;
        if (display.id == entrance.id) {
            display.center = point({-310, 190, -20});
            display.size = {210, 132};
            display.yaw = m_yaw;
        }
    }
    return true;
}

QVector3D SpatialWebPassage::point(const QVector3D &local) const
{
    return m_origin + rotate(local, m_yaw);
}

float SpatialWebPassage::distance(const QVector3D &camera) const
{
    return rotate(camera - m_origin, -m_yaw).z();
}

bool SpatialWebPassage::advance(const QVector3D &camera)
{
    if (m_destination || m_failed)
        return false;
    const int needed = qBound(2, int(std::floor(qMax(0.0f, distance(camera)) / segmentLength)) + 2, maximumSegments);
    if (needed <= m_segments)
        return false;
    m_segments = needed;
    return true;
}

bool SpatialWebPassage::stage(const SpatialInterior &destination)
{
    if (destination.solids.size() > SpatialInterior::maximumSolids || destination.displays.size() > 12
        || m_source.rideSurfaces.size() + destination.rideSurfaces.size() > SpatialRideSurface::maximumSurfaces)
        return false;
    if (m_failed)
        return false;
    m_destination = destination;
    const QVector3D spawn(destination.spawn.x(), 0, destination.spawn.y());
    m_destinationOffset = point({0, 0, float(m_segments * segmentLength + 160)}) - rotate(spawn, m_yaw);
    m_failed = false;
    return true;
}

bool SpatialWebPassage::arrived(const QVector3D &camera) const
{
    return ready() && distance(camera) >= m_segments * segmentLength + 130;
}

QVector3D SpatialWebPassage::destinationPosition(const QVector3D &camera) const
{
    return rotate(camera - m_destinationOffset, -m_yaw);
}

SpatialInterior SpatialWebPassage::room() const
{
    SpatialInterior result = m_source;
    result.streaming = true;
    result.actions.clear();
    result.walkSurfaces.clear();
    result.environmentKey.clear();
    result.outdoors = true;
    result.height = qMax(500.0, result.height);
    const float length = m_segments * segmentLength;
    const auto part = [&](QVector3D center, QVector3D size, QColor color, QString label = {}) {
        SpatialSolid solid{point(center), size, color, true, SpatialSolidShape::Box, label};
        solid.yaw = m_yaw;
        result.solids.append(solid);
    };
    // Four short sections maximum; each extension happens at least one section
    // ahead. No timer or inference creates geometry once that cap is reached.
    for (int segment = 0; segment < m_segments; ++segment) {
        const float center = segment * segmentLength + segmentLength / 2;
        part({-190, 190, center}, {30, 380, segmentLength}, result.wallColor);
        part({190, 190, center}, {30, 380, segmentLength}, result.wallColor);
        part({0, 390, center}, {410, 20, segmentLength}, result.wallColor.lighter(125));
        part({0, 372, center - 160}, {340, 6, 12}, QColor("#83d9ce"));
    }
    part({-205, 190, length}, {60, 380, 40}, result.wallColor.lighter(130));
    part({205, 190, length}, {60, 380, 40}, result.wallColor.lighter(130));
    if (!ready()) {
        // A visible waist-high gate keeps the waiting door and its message in
        // view, rather than letting the camera press against a blank surface.
        if (m_segments == maximumSegments || m_failed)
            part({0, 85, length - 500}, {350, 35, 20}, QColor("#83d9ce"));
        part({0, 150, length}, {350, 300, 30}, QColor("#385d69"), m_failed
            ? QStringLiteral("Could not load · Back to return") : QStringLiteral("Loading — please wait"));
    } else {
        // Sliding panels stay parked beside the opening after the page is ready.
        part({-270, 150, length}, {170, 300, 30}, QColor("#385d69"));
        part({270, 150, length}, {170, 300, 30}, QColor("#385d69"));
        const auto &next = *m_destination;
        for (auto solid : next.solids) {
            solid.center = m_destinationOffset + rotate(solid.center, m_yaw);
            solid.yaw += m_yaw;
            result.solids.append(solid);
        }
        for (auto display : next.displays) {
            // Streaming uses namespaced textures; source pages commonly reuse
            // numeric DOM item IDs. Restore their normal IDs on arrival.
            display.id.prepend("arrival:");
            const auto corners = SpatialInteriorGeometry::surface(next, display);
            display.center = m_destinationOffset + rotate((corners[0] + corners[2]) / 2, m_yaw);
            if (!display.freestanding) {
                if (display.wall == "south")
                    display.yaw = 180;
                else if (display.wall == "east")
                    display.yaw = -90;
                else if (display.wall == "west")
                    display.yaw = 90;
            }
            display.freestanding = true;
            display.yaw += m_yaw;
            display.portal = false;
            result.displays.append(display);
        }
        for (auto surface : next.rideSurfaces) {
            surface.center = m_destinationOffset + rotate(surface.center, m_yaw);
            surface.heading = std::remainder(surface.heading - m_yaw * 3.14159265358979323846f / 180, 6.283185307f);
            result.rideSurfaces.append(surface);
        }
        int vertices = 0;
        int faces = 0;
        for (const auto &mesh : result.meshes) {
            vertices += mesh.vertices.size();
            faces += mesh.faces.size();
        }
        for (auto mesh : next.meshes) {
            if (result.meshes.size() >= SpatialInterior::maximumMeshes
                || vertices + mesh.vertices.size() > SpatialInterior::maximumMeshVertices
                || faces + mesh.faces.size() > SpatialInterior::maximumMeshFaces)
                break;
            vertices += mesh.vertices.size();
            faces += mesh.faces.size();
            for (auto &vertex : mesh.vertices)
                vertex = m_destinationOffset + rotate(vertex, m_yaw);
            if (mesh.hasSmoothNormals()) {
                for (auto &normal : mesh.normals)
                    normal = rotate(normal, m_yaw);
            } else {
                mesh.normals.clear();
            }
            result.meshes.append(mesh);
        }
        result.height = qMax(result.height, next.height);
    }
    // Bound the existing renderer around both areas and the finite passage.
    float extentX = result.size.width() / 2;
    float extentZ = result.size.height() / 2;
    for (const auto &solid : result.solids) {
        extentX = qMax(extentX, std::abs(solid.center.x()) + solid.size.length() / 2 + 200);
        extentZ = qMax(extentZ, std::abs(solid.center.z()) + solid.size.length() / 2 + 200);
    }
    for (const auto &display : result.displays) {
        extentX = qMax(extentX, std::abs(display.center.x()) + 500);
        extentZ = qMax(extentZ, std::abs(display.center.z()) + 500);
    }
    result.size = {extentX * 2, extentZ * 2};
    result.exit = {extentX + 100, -extentZ - 100};
    return result;
}
