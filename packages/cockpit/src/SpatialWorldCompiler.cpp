#include "SpatialWebWorld.h"
#include "SpatialInteriorGeometry.h"
#include "DirtBikeDynamics.h"
#include <QSet>
#include <cmath>

namespace {
constexpr float pi = 3.14159265358979323846f;
bool finite(const QVector3D &point)
{
    return std::isfinite(point.x()) && std::isfinite(point.y()) && std::isfinite(point.z())
        && std::abs(point.x()) <= 10000 && std::abs(point.y()) <= 10000 && std::abs(point.z()) <= 10000;
}
QVector3D rotate(const QVector3D &point, float yaw)
{
    const float radians = yaw * pi / 180;
    return {point.x() * std::cos(radians) - point.z() * std::sin(radians), point.y(),
        point.x() * std::sin(radians) + point.z() * std::cos(radians)};
}
bool routeClear(const SpatialInterior &room, const QVector3D &from, const QVector3D &to)
{
    // At most 800 samples per edge, no wider than the 25-unit camera radius.
    const int steps = qMax(1, int(std::ceil((to - from).length() / 25)));
    if (steps > 800)
        return false;
    for (int step = 0; step <= steps; ++step) {
        if (SpatialInteriorGeometry::blocked(room, from + (to - from) * (float(step) / steps)))
            return false;
    }
    return true;
}
}

SpatialWorldResult SpatialWebWorld::compile(const SpatialWorldPlan &plan)
{
    SpatialWorldResult result;
    const auto fail = [&](const QString &message) {
        result.diagnostic = message;
        return result;
    };
    QString graphError;
    if (!plan.graph.validate(graphError))
        return fail(graphError);
    if (plan.transforms.isEmpty() || plan.transforms.size() > SpatialWorldPlan::maximumTransforms
        || plan.parts.size() > SpatialInterior::maximumSolids
        || plan.anchors.size() != plan.graph.nodes.size()
        || plan.connections.size() > SpatialWebGraph::maximumNodes
        || plan.displays.size() > SpatialWebDocument::maximumItems)
        return fail("Construction exceeds its node, part, route or display budget.");
    if (!std::isfinite(plan.size.width()) || !std::isfinite(plan.size.height())
        || plan.size.width() < 1000 || plan.size.height() < 1000 || plan.size.width() > 16000 || plan.size.height() > 16000
        || !std::isfinite(plan.height) || plan.height < 400 || plan.height > 4000 || !finite(plan.spawn)
        || plan.spawn.y() < 60 || plan.spawn.y() > plan.height - 40
        || !QStringList{"walk", "fly", "drive"}.contains(plan.options.locomotion))
        return fail("Invalid world bounds or traversal mode.");
    if (plan.options.topology != SpatialTopology::Branches && plan.options.topology != SpatialTopology::Loop
        && plan.options.topology != SpatialTopology::Spine && plan.options.topology != SpatialTopology::Layers)
        return fail("Unknown topology algorithm.");
    auto &room = result.interior;
    room.size = plan.size;
    room.height = plan.height;
    room.spawn = {plan.spawn.x(), plan.spawn.z()};
    room.spawnHeight = plan.spawn.y();
    room.exit = {0, -plan.size.height() / 2 + 30};
    room.locomotion = plan.options.locomotion;
    room.outdoors = plan.options.style.outdoors;
    room.naturalGround = room.outdoors && !plan.options.features.isEmpty();
    room.floorColor = plan.options.style.ground;
    room.skyColor = plan.options.style.sky;
    room.wallColor = plan.options.style.structure.darker(190);
    if (!room.floorColor.isValid() || !room.skyColor.isValid() || !room.wallColor.isValid())
        return fail("Invalid material palette.");
    QVector<SpatialWorldTransform> transforms;
    for (int index = 0; index < plan.transforms.size(); ++index) {
        auto transform = plan.transforms[index];
        if (transform.parent < -1 || transform.parent >= index || !finite(transform.translation)
            || !std::isfinite(transform.yaw) || std::abs(transform.yaw) > 360
            || !std::isfinite(transform.scale) || transform.scale < 0.25f || transform.scale > 4)
            return fail("Invalid or cyclic construction transform.");
        if (transform.parent >= 0) {
            const auto &parent = transforms[transform.parent];
            transform.translation = parent.translation + rotate(transform.translation * parent.scale, parent.yaw);
            transform.yaw += parent.yaw;
            transform.scale *= parent.scale;
        }
        if (!finite(transform.translation) || transform.scale < 0.25f || transform.scale > 4)
            return fail("Accumulated construction transform exceeds bounds.");
        transforms.append(transform);
    }
    for (const auto &part : plan.parts) {
        if (part.transform < 0 || part.transform >= transforms.size() || !finite(part.center) || !finite(part.size)
            || part.size.x() <= 0 || part.size.y() <= 0 || part.size.z() <= 0 || !part.color.isValid()
            || part.label.size() > 180 || (part.shape != SpatialSolidShape::Box && part.shape != SpatialSolidShape::Pyramid
                && part.shape != SpatialSolidShape::Octahedron))
            return fail("Invalid construction part.");
        const auto &transform = transforms[part.transform];
        SpatialSolid solid;
        solid.center = transform.translation + rotate(part.center * transform.scale, transform.yaw);
        solid.size = part.size * transform.scale;
        solid.color = part.color;
        solid.collision = part.collision;
        solid.shape = part.shape;
        solid.label = part.label;
        solid.yaw = transform.yaw;
        if (!finite(solid.center) || !finite(solid.size) || solid.size.length() > 2400
            || solid.center.y() - solid.size.y() / 2 < -0.01f || solid.center.y() + solid.size.y() / 2 > room.height
            || std::abs(solid.center.x()) + solid.size.length() / 2 > room.size.width() / 2
            || std::abs(solid.center.z()) + solid.size.length() / 2 > room.size.height() / 2)
            return fail("Constructed part extends beyond world bounds.");
        room.solids.append(solid);
    }
    if (plan.rideSurfaces.size() > SpatialRideSurface::maximumSurfaces)
        return fail("Construction exceeds the ride-surface budget.");
    for (const auto &surface : plan.rideSurfaces) {
        if (!surface.valid())
            return fail("Invalid ride-surface profile.");
        for (int index = 1; index < surface.profile.size(); ++index) {
            const auto span = surface.profile[index] - surface.profile[index - 1];
            if (std::abs(span.y() / span.x()) > .75)
                return fail("Ride surface is too steep for shared walking routes.");
        }
        for (const auto &knot : surface.profile) {
            if (2 * knot.y() * surface.bank / (surface.width * .64f) > .75f)
                return fail("Ride surface banking is too steep for shared walking routes.");
        }
        for (const auto &vertex : surface.mesh().vertices) {
            if (!finite(vertex) || std::abs(vertex.x()) > room.size.width() / 2 - 40
                || std::abs(vertex.z()) > room.size.height() / 2 - 40 || vertex.y() > room.height - 200)
                return fail("Ride surface extends beyond world bounds.");
        }
        room.rideSurfaces.append(surface);
    }
    if (DirtBikeDynamics::groundHeight(room, plan.spawn) > plan.spawn.y() - 120)
        return fail("Ride surface obstructs the arrival point.");
    if (plan.vehicles.size() > SpatialVehicleModel::maximumVehicles)
        return fail("Construction exceeds the vehicle budget.");
    QSet<QString> vehicleIds;
    for (const auto &vehicle : plan.vehicles) {
        const bool bound = std::any_of(plan.graph.nodes.cbegin(), plan.graph.nodes.cend(), [&](const auto &node) {
            return node.role == SpatialWebNode::Role::Destination && node.id == vehicle.id;
        });
        if (!SpatialVehicleModel::valid(vehicle) || !bound || vehicleIds.contains(vehicle.id)
            || SpatialVehicleModel::blocked(room, vehicle)
            || DirtBikeDynamics::groundHeight(room, vehicle.position) > 1)
            return fail("Invalid, overlapping or unbound vehicle.");
        vehicleIds.insert(vehicle.id);
        room.vehicles.append(vehicle);
    }
    QSet<int> reached{0};
    for (int index = 0; index < plan.anchors.size(); ++index) {
        const auto &anchor = plan.anchors[index];
        if (anchor.node != index || !finite(anchor.position) || anchor.position.y() < 60 || anchor.position.y() > room.height - 40
            || (room.locomotion != "fly" && std::abs(anchor.position.y() - plan.spawn.y()) > 1))
            return fail("Invalid interaction anchor or unsupported vertical walking route.");
    }
    if (!routeClear(room, plan.spawn, plan.anchors[0].position))
        return fail("The world entrance is obstructed.");
    SpatialMesh paths;
    paths.color = room.floorColor.lighter(135);
    if (room.locomotion == "fly") {
        paths.color = plan.options.style.accent;
        paths.color.setAlpha(130);
    }
    for (const auto &edge : plan.connections) {
        if (edge.from < 0 || edge.to < 0 || edge.from >= plan.anchors.size() || edge.to >= plan.anchors.size())
            return fail("Traversal connection has an invalid endpoint.");
        auto from = plan.anchors[edge.from].position;
        auto to = plan.anchors[edge.to].position;
        if (!routeClear(room, from, to))
            return fail("A construction part obstructs a reserved route.");
        if ((to - from).lengthSquared() < 1)
            continue;
        const auto direction = (to - from).normalized();
        float routeWidth = room.locomotion == "fly" ? 8 : 85;
        if (plan.options.features.size() == 8 && plan.options.features[6] > 0.55f)
            routeWidth = 200;
        const auto side = QVector3D(direction.z(), 0, -direction.x()) * routeWidth;
        const float offset = room.locomotion == "fly" ? 120 : plan.spawn.y() - 3;
        from.setY(from.y() - offset);
        to.setY(to.y() - offset);
        const int first = paths.vertices.size();
        paths.vertices.append(from - side);
        paths.vertices.append(from + side);
        paths.vertices.append(to + side);
        paths.vertices.append(to - side);
        paths.faces.append({first, first + 1, first + 2, first + 3});
    }
    // Bounded flood over explicit undirected routes. Containment alone cannot
    // establish reachability; a disconnected decorative island is rejected.
    for (int pass = 0; pass < plan.anchors.size(); ++pass) {
        for (const auto &edge : plan.connections) {
            if (reached.contains(edge.from))
                reached.insert(edge.to);
            if (reached.contains(edge.to))
                reached.insert(edge.from);
        }
    }
    if (reached.size() != plan.anchors.size())
        return fail("Construction contains unreachable content.");
    if (plan.meshes.size() + 1 > SpatialInterior::maximumMeshes)
        return fail("Construction exceeds the mesh budget.");
    int vertexCount = paths.vertices.size();
    int faceCount = paths.faces.size();
    for (const auto &mesh : plan.meshes) {
        vertexCount += mesh.vertices.size();
        faceCount += mesh.faces.size();
        if (vertexCount > SpatialInterior::maximumMeshVertices || faceCount > SpatialInterior::maximumMeshFaces
            || !mesh.color.isValid())
            return fail("Construction exceeds the polygon budget or has an invalid material.");
        for (const auto &vertex : mesh.vertices) {
            if (!finite(vertex) || std::abs(vertex.x()) > room.size.width() / 2
                || std::abs(vertex.z()) > room.size.height() / 2 || vertex.y() < 0 || vertex.y() > room.height)
                return fail("Invalid mesh vertex.");
        }
        for (const auto &face : mesh.faces) {
            if (face.size() < 3 || face.size() > 4)
                return fail("Mesh faces must be triangles or quads.");
            QSet<int> corners;
            for (int vertex : face) {
                if (vertex < 0 || vertex >= mesh.vertices.size() || corners.contains(vertex))
                    return fail("Invalid mesh face index.");
                corners.insert(vertex);
            }
            if (QVector3D::crossProduct(mesh.vertices[face[1]] - mesh.vertices[face[0]],
                mesh.vertices[face[2]] - mesh.vertices[face[0]]).lengthSquared() < 0.01)
                return fail("Degenerate mesh face.");
        }
        room.meshes.append(mesh);
    }
    QSet<QString> bindings;
    for (const auto &display : plan.displays) {
        const auto node = std::find_if(plan.graph.nodes.cbegin(), plan.graph.nodes.cend(), [&](const SpatialWebNode &candidate) {
            return candidate.role == SpatialWebNode::Role::Destination && candidate.id == display.id;
        });
        if (node == plan.graph.nodes.cend() || bindings.contains(display.id) || !finite(display.center)
            || !std::isfinite(display.yaw) || std::abs(display.yaw) > 360 || !finite(display.portalCenter)
            || !std::isfinite(display.size.width()) || !std::isfinite(display.size.height())
            || display.size.width() < 100 || display.size.width() > 1000 || display.size.height() < 60 || display.size.height() > 640
            || !display.freestanding || !std::isfinite(display.portalApproachDistance)
            || display.portalApproachDistance < 110 || display.portalApproachDistance > 240
            || (display.portal && node->action != SpatialWebNode::Action::Navigate))
            return fail("Invalid content-to-surface binding.");
        const int nodeIndex = int(node - plan.graph.nodes.cbegin());
        const auto anchor = plan.anchors[nodeIndex].position;
        const auto direction = (display.center - anchor).normalized();
        SpatialInterior probe = room;
        probe.displays = {display};
        if ((display.portalCenter - anchor).length() > 1
            || SpatialInteriorGeometry::target(probe, anchor, direction) != display.id)
            return fail("Content " + display.id + " is occluded from its interaction point.");
        bindings.insert(display.id);
    }
    const int destinations = std::count_if(plan.graph.nodes.cbegin(), plan.graph.nodes.cend(), [](const SpatialWebNode &node) {
        return node.role == SpatialWebNode::Role::Destination;
    });
    if (bindings.size() != destinations)
        return fail("Construction dropped a content destination.");
    room.displays = plan.displays;
    room.meshes.append(paths);
    result.success = true;
    return result;
}

SpatialWorldResult SpatialWebWorld::construct(const SpatialWebDocument &document, const SpatialWorldOptions &options)
{
    const auto graph = SpatialWebGraph::fromDocument(document);
    auto result = compile(plan(document, graph, options));
    if (result.success)
        return result;
    const QString diagnostic = result.diagnostic;
    SpatialWorldOptions fallback;
    fallback.topology = SpatialTopology::Spine;
    fallback.style.repetitions = 0;
    result = compile(plan(document, graph, fallback));
    result.fallback = result.success;
    if (result.success)
        result.diagnostic = "Simplified world: " + diagnostic;
    return result;
}
