#include "SpatialMeshLoft.h"

#include <algorithm>
#include <cmath>
#include <utility>

namespace {
constexpr float pi = 3.14159265358979323846f;

bool finite(const QVector3D &point)
{
    return std::isfinite(point.x()) && std::isfinite(point.y()) && std::isfinite(point.z());
}
}

SpatialLoftResult SpatialMeshLoft::build(const QVector<SpatialLoftSection> &sections,
    QVector3D across, QVector3D depth, int sides, int faceBudget,
    const SpatialVehicleFace &style, bool capped)
{
    // Bound rings and sides individually as well as their product. The model
    // consumer admits 4096 polygons; the loft cannot expand beyond that contract.
    constexpr int maximumFaces = SpatialVehicleModel::maximumFaces;
    if (faceBudget < 0 || faceBudget > maximumFaces || sections.size() < 2
        || sections.size() > maximumFaces || sides < 3 || sides > maximumFaces)
        return {{}, "Invalid loft dimensions or face budget"};
    const qint64 count = (sections.size() - 1 + (capped ? 2 : 0)) * qint64(sides);
    if (count > faceBudget)
        return {{}, "Loft exceeds remaining model face budget"};
    if (!finite(across) || !finite(depth) || across.lengthSquared() < .001f
        || depth.lengthSquared() < .001f)
        return {{}, "Invalid loft cross-section frame"};
    across.normalize();
    depth.normalize();
    if (std::abs(QVector3D::dotProduct(across, depth)) > .001f)
        return {{}, "Loft cross-section axes must be perpendicular"};
    const auto axis = QVector3D::crossProduct(across, depth).normalized();
    for (int ring = 0; ring < sections.size(); ++ring) {
        const auto &section = sections[ring];
        if (!finite(section.center) || !std::isfinite(section.width) || !std::isfinite(section.depth)
            || section.width <= .001f || section.depth <= .001f)
            return {{}, "Invalid loft section"};
        if (ring > 0 && QVector3D::dotProduct(section.center - sections[ring - 1].center, axis) <= .001f)
            return {{}, "Loft sections must advance along the cross-section axis"};
    }

    const auto point = [&](int ring, int side) {
        const float angle = (side % sides) * 2 * pi / sides;
        const auto &section = sections[ring];
        return section.center + across * (section.width * std::cos(angle))
            + depth * (section.depth * std::sin(angle));
    };
    const auto normal = [&](int ring, int side) {
        const float angle = (side % sides) * 2 * pi / sides;
        const auto &section = sections[ring];
        const auto around = across * (-section.width * std::sin(angle))
            + depth * (section.depth * std::cos(angle));
        const auto along = point(qMin(ring + 1, int(sections.size()) - 1), side)
            - point(qMax(0, ring - 1), side);
        return QVector3D::crossProduct(around, along).normalized();
    };
    SpatialLoftResult result;
    result.faces.reserve(count);
    const auto append = [&](QVector<QVector3D> points, QVector<QVector3D> normals) {
        auto face = style;
        face.vertices = std::move(points);
        face.normals = std::move(normals);
        face.uvs.clear();
        const auto geometric = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
            face.vertices[2] - face.vertices[0]);
        if (QVector3D::dotProduct(geometric, face.normals.first()) < 0) {
            std::reverse(face.vertices.begin(), face.vertices.end());
            std::reverse(face.normals.begin(), face.normals.end());
        }
        result.faces.append(std::move(face));
    };
    for (int ring = 0; ring + 1 < sections.size(); ++ring) {
        for (int side = 0; side < sides; ++side) {
            append({point(ring, side), point(ring, side + 1), point(ring + 1, side + 1), point(ring + 1, side)},
                {normal(ring, side), normal(ring, side + 1), normal(ring + 1, side + 1), normal(ring + 1, side)});
        }
    }
    if (capped) {
        for (int ring : {0, int(sections.size()) - 1}) {
            const auto capNormal = ring == 0 ? -axis : axis;
            for (int side = 0; side < sides; ++side)
                append({sections[ring].center, point(ring, side), point(ring, side + 1)},
                    {capNormal, capNormal, capNormal});
        }
    }
    // Finite inputs can still overflow during float geometry arithmetic, or
    // lose distinct vertices at extreme coordinate magnitudes. Never publish
    // those surfaces as a successful mesh.
    for (const auto &face : result.faces) {
        const auto area = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
            face.vertices[2] - face.vertices[0]);
        if (!finite(area) || !std::isfinite(area.lengthSquared()) || area.lengthSquared() <= 1e-8f)
            return {{}, "Degenerate or overflowing loft surface"};
        for (int vertex = 0; vertex < face.vertices.size(); ++vertex) {
            if (!finite(face.vertices[vertex]) || !finite(face.normals[vertex])
                || face.normals[vertex].lengthSquared() < .99f)
                return {{}, "Invalid loft vertex or normal"};
        }
    }
    return result;
}
