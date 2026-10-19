#include "SpatialWebWorld.h"
#include "SpatialWorldArchitecture.h"
#include "SpatialAnimalModel.h"
#include <QCryptographicHash>
#include <QRegularExpression>
#include <QSet>
#include <cmath>

namespace {
constexpr float pi = 3.14159265358979323846f;

}

SpatialWorldOptions SpatialWebWorld::optionsFor(const SpatialWebDocument &document)
{
    SpatialWorldOptions options;
    QByteArray identity = document.url.adjusted(QUrl::RemoveQuery | QUrl::RemoveFragment).toEncoded();
    identity += document.title.toUtf8();
    QSet<QString> categories;
    float averageAspect = 0;
    int illustrated = 0;
    for (const auto &item : document.items) {
        identity += '\n' + item.title.toUtf8() + ':' + item.category.toUtf8() + ':' + item.kind.toUtf8();
        if (!item.category.isEmpty())
            categories.insert(item.category);
        if (item.hasImage) {
            averageAspect += item.visualAspect;
            ++illustrated;
        }
    }
    const auto digest = QCryptographicHash::hash(identity, QCryptographicHash::Sha256);
    options.seed = static_cast<unsigned char>(digest[0]) * 256 + static_cast<unsigned char>(digest[1]);
    // Departments branch, dense catalogues form aisles, and visual collections
    // form a promenade. The page's structure drives composition before styling.
    if (categories.size() >= 3)
        options.topology = SpatialTopology::Branches;
    else if (document.items.size() >= 8 && illustrated < 6)
        options.topology = SpatialTopology::Spine;
    else
        options.topology = SpatialTopology::Loop;
    if (illustrated)
        averageAspect /= illustrated;
    options.spacing = qBound(0.85f, 0.9f + averageAspect * 0.13f, 1.3f);
    const QString evidence = document.evidence.toLower();
    // Policy produces independent composition parameters. No renderer or parser
    // knows these theme names, and all combinations use the same construction rules.
    if (evidence.contains(QRegularExpression("medieval|castle|dragon|wizard|sorcery|fantasy world"))) {
        options.style = {QColor("#797a69"), QColor("#607e96"), QColor("#96959b"), QColor("#705887"),
            SpatialSolidShape::Pyramid, true, 490, 3};
    } else if (document.venue == "park" || evidence.contains(QRegularExpression("woodland|rain[ -]?forest|forest|national park|hiking trail|campground|wildlife|habitat|animal sanctuary"))) {
        options.style = {QColor("#426c42"), QColor("#8eb7bb"), QColor("#71533b"), QColor("#347e4a"),
            SpatialSolidShape::Pyramid, true, 430, 0};
        options.features = {0.85f, 0, 0, 0, 0, 0, 0, 0};
    } else if (evidence.contains(QRegularExpression("spaceflight|astronomy|spacecraft|starship|solar system|interstellar|galaxy"))) {
        options.style = {QColor("#080c22"), QColor("#080c22"), QColor("#536c99"), QColor("#7e78cc"),
            SpatialSolidShape::Octahedron, true, 400, 2};
        options.locomotion = "fly";
        options.topology = SpatialTopology::Layers;
    } else if (evidence.contains(QRegularExpression("amusement park|theme park|roller coaster|fairground|carnival"))) {
        options.style = {QColor("#67925a"), QColor("#8cc5e4"), QColor("#edce84"), QColor("#cf658b"),
            SpatialSolidShape::Pyramid, true, 350, 2};
    }
    if (document.venue == "showroom") {
        options.locomotion = "walk";
        options.spacing = 1.4f;
        options.style.outdoors = true;
        options.style.ground = QColor("#43515a");
        options.style.sky = QColor("#88b5cc");
    }
    if (document.venue == "store") {
        options.style.ground = QColor("#a5aaa7");
        options.spacing = 0.85f;
    }
    if (document.accent.isValid() && document.accent.saturation() >= 35)
        options.style.accent = document.accent;
    SpatialWorldArchitecture::interpret(options, document);
    if (document.stableLayout) {
        const auto stable = QCryptographicHash::hash(document.layoutIdentity.toUtf8(), QCryptographicHash::Sha256);
        options.seed = static_cast<unsigned char>(stable[0]) * 256 + static_cast<unsigned char>(stable[1]);
        options.topology = SpatialTopology::Spine;
        options.spacing = 1;
        options.locomotion = "walk";
        if (document.siteOverview)
            options.style.outdoors = true;
    }
    return options;
}

SpatialWorldPlan SpatialWebWorld::plan(const SpatialWebDocument &document, const SpatialWebGraph &graph,
    const SpatialWorldOptions &options)
{
    SpatialWorldPlan result;
    result.graph = graph;
    result.options = options;
    QString error;
    if (!graph.validate(error) || !std::isfinite(options.spacing) || options.spacing < 0.75f || options.spacing > 2
        || options.style.repetitions < 0 || options.style.repetitions > 3
        || !std::isfinite(options.style.roofHeight) || options.style.roofHeight < 280 || options.style.roofHeight > 600)
        return result;
    const bool layered = options.topology == SpatialTopology::Layers;
    result.height = layered ? 2000 : 900 * options.spacing;
    if (layered)
        result.options.locomotion = "fly";
    const float eye = result.options.locomotion == "drive" ? 110 : 160;
    result.spawn = {0, layered ? 450.0f : eye, -350};
    result.anchors.resize(graph.nodes.size());
    result.transforms.append({-1, {}, 0, options.spacing});
    QVector<int> destinations;
    QVector<int> groups;
    for (int index = 0; index < graph.nodes.size(); ++index) {
        result.anchors[index] = {index, {0, result.spawn.y(), 0}};
        if (graph.nodes[index].role == SpatialWebNode::Role::Destination)
            destinations.append(index);
        else if (graph.nodes[index].role == SpatialWebNode::Role::Collection)
            groups.append(index);
    }
    const float radius = qMax(1100.0f, destinations.size() * 190.0f);
    // Keep siblings adjacent in the abstract ordering without changing source IDs.
    std::stable_sort(destinations.begin(), destinations.end(), [&](int left, int right) {
        return graph.nodes[left].parent < graph.nodes[right].parent;
    });
    for (int ordinal = 0; ordinal < destinations.size(); ++ordinal) {
        const int node = destinations[ordinal];
        QVector3D location;
        const int slot = document.items[graph.nodes[node].itemIndex].spatialSlot;
        if (document.stableLayout && slot >= 0 && slot < 12) {
            location = {(slot % 2 == 0 ? -800.0f : 800.0f), eye, 1000.0f + float(slot / 2) * 680};
        } else if (options.topology == SpatialTopology::Spine) {
            location = {(ordinal % 2 == 0 ? -800.0f : 800.0f), eye,
                (ordinal / 2 - (destinations.size() - 1) / 4.0f) * 680};
        } else {
            const float angle = 2 * pi * ordinal / qMax(1, int(destinations.size())) + pi / 6;
            float distance = radius;
            if (options.topology == SpatialTopology::Branches)
                distance += groups.indexOf(graph.nodes[node].parent) * 240;
            // A small stable radial variation avoids identical circular halls.
            distance += ((options.seed + ordinal * 37) % 5) * 55;
            location = {distance * std::sin(angle), eye, distance * std::cos(angle)};
            if (layered)
                location.setY(450 + (ordinal % 3) * 380);
        }
        location.setX(location.x() * options.spacing);
        location.setZ(location.z() * options.spacing);
        result.anchors[node].position = location;
    }
    for (int group : groups) {
        QVector3D center;
        int count = 0;
        for (int node : destinations) {
            if (graph.nodes[node].parent == group) {
                center += result.anchors[node].position;
                ++count;
            }
        }
        if (count)
            center /= count;
        if (options.topology == SpatialTopology::Spine)
            center.setX(0);
        else {
            center.setX(center.x() * 0.45f);
            center.setZ(center.z() * 0.45f);
        }
        result.anchors[group].position = center;
        result.connections.append({0, group});
    }
    for (int ordinal = 0; ordinal < destinations.size(); ++ordinal) {
        const int nodeIndex = destinations[ordinal];
        const auto &node = graph.nodes[nodeIndex];
        if (node.itemIndex < 0 || node.itemIndex >= document.items.size()) {
            result.anchors.clear();
            return result;
        }
        const auto &item = document.items[node.itemIndex];
        const auto anchor = result.anchors[nodeIndex].position;
        auto outward = result.anchors[node.parent].position - anchor;
        if (options.topology == SpatialTopology::Spine)
            outward = {-anchor.x(), 0, 0};
        outward.setY(0);
        if (outward.lengthSquared() < 1)
            outward = {0, 0, -1};
        outward.normalize();
        const float yaw = std::atan2(outward.x(), -outward.z()) * 180 / pi;
        const auto origin = anchor - outward * 200;
        const int transform = result.transforms.size();
        // Collection/node transforms can be nested without exposing hierarchy to
        // the renderer. Here a shared root owns uniformly scaled local assemblies.
        result.transforms.append({0, origin / options.spacing, yaw, 1});
        const auto part = [&](QVector3D center, QVector3D size, QColor color,
                              SpatialSolidShape shape = SpatialSolidShape::Box, QString label = {}) {
            result.parts.append({transform, center, size, color, shape, true, label});
        };
        const float ground = layered ? -160 : -anchor.y() / options.spacing;
        const bool product = item.kind == "product";
        const bool animal = item.kind == "animal";
        const bool sectionEntrance = !layered && !product && !animal && item.vehicleBody.isEmpty()
            && node.action == SpatialWebNode::Action::Navigate;
        const float visualWidth = item.hasImage ? qBound(240.0f, item.visualAspect * 190, 440.0f) : 330;
        const bool vehicle = !item.vehicleBody.isEmpty() && !layered;
        // Stable local variation helps distinguish neighbouring destinations.
        const int hue = (qMax(0, options.style.accent.hsvHue()) + options.seed % 25 + ordinal * 13) % 360;
        const QColor accent = QColor::fromHsv(hue, options.style.accent.hsvSaturation(), options.style.accent.value());
        if (sectionEntrance) {
            // A usable doorway and a recessed display window are separate parts.
            // Eight parts per building keep a full twelve-destination area bounded.
            part({-350, ground + 230, 220}, {40, 460, 440}, options.style.structure);
            part({350, ground + 230, 220}, {40, 460, 440}, options.style.structure);
            part({0, ground + 230, 430}, {740, 460, 30}, options.style.structure.darker(115));
            part({0, ground + 490, 220}, {780, 100, 490}, accent, options.style.crown);
            part({0, ground + 150, 15}, {240, 300, 20}, options.style.structure.darker(150),
                SpatialSolidShape::Box, "Enter");
            part({0, ground + 365, 15}, {680, 70, 35}, options.style.structure,
                SpatialSolidShape::Box, item.title.left(140));
            const bool nature = options.features.size() == 8 && options.features[0] > 0.45f;
            if (nature) {
                part({-245, ground + 135, 105}, {35, 270, 35}, QColor("#765339"));
                part({-245, ground + 310, 105}, {170, 190, 170}, QColor("#3b794c"), SpatialSolidShape::Pyramid);
            }
        } else if (!vehicle) {
            part({0, ground + 35, 100}, {visualWidth + 30, 70, 260}, options.style.structure,
                SpatialSolidShape::Box, product ? (item.title + " " + item.detail).left(180) : QString());
            // Repetition, support, cap and content are separate rules. The same rules
            // can produce shelves, a canopy, clustered spires or an orbital assembly.
            if (!animal && !options.features.isEmpty())
                SpatialWorldArchitecture::assemble(result, item, transform, ground, visualWidth, ordinal);
            for (int repeat = 0; !animal && repeat < options.style.repetitions; ++repeat) {
                const float x = (repeat - (options.style.repetitions - 1) / 2.0f) * 190;
                part({x, ground + 170, 245}, {32, 340, 32}, options.style.structure);
                part({x, ground + options.style.roofHeight, 245}, {200, 160, 220},
                    accent, options.style.crown);
            }
            if (!product && !animal)
                part({0, 0, 20}, {visualWidth, 205, 20}, options.style.structure);
        }
        SpatialDisplay display;
        display.id = node.id;
        display.application.name = node.title;
        display.freestanding = true;
        display.center = origin + QVector3D(0, 20, 0);
        const float displayWidth = (visualWidth - 30) * node.importance;
        // The captured poster is 512x320; resizing its fixture must not stretch
        // website artwork or native signage.
        display.size = {displayWidth, displayWidth * 320 / 512};
        if (product)
            display.center.setY(origin.y() + (ground + 70) * options.spacing + display.size.height() / 2);
        if (sectionEntrance) {
            const QVector3D right(std::cos(yaw * pi / 180), 0, std::sin(yaw * pi / 180));
            display.center = origin - outward * (90 * options.spacing) + right * (235 * options.spacing);
            display.center.setY(205 * options.spacing);
            const float windowWidth = qBound(140.0f, visualWidth * 0.44f, 195.0f) * options.spacing;
            display.size = {windowWidth, windowWidth / 1.6f};
        }
        display.yaw = yaw;
        display.sprite = product;
        display.portal = node.action == SpatialWebNode::Action::Navigate && !product && !vehicle && !animal;
        display.portalCenter = anchor;
        display.portalApproachDistance = sectionEntrance ? 240 : 110;
        if (animal) {
            const QVector3D right(std::cos(yaw * pi / 180), 0, std::sin(yaw * pi / 180));
            display.center = origin - right * (135 * options.spacing);
            display.center.setY(120 * options.spacing);
            display.size = {180 * options.spacing, 112.5 * options.spacing};
        }
        if (vehicle) {
            SpatialVehicle car;
            car.id = item.id;
            car.name = item.title;
            car.body = item.vehicleBody;
            car.year = item.vehicleYear;
            car.paint = QColor::fromHsv((options.seed + ordinal * 47) % 360, 150, 190);
            car.position = anchor - outward * 440;
            car.position.setY(0);
            car.heading = std::atan2(outward.x(), outward.z()) + pi / 2;
            result.vehicles.append(car);
            display.center = anchor - outward * 70;
            display.center.setY(205);
            display.size = {180, 112};
        }
        result.displays.append(display);
        if (options.topology == SpatialTopology::Loop && ordinal > 0)
            result.connections.append({destinations[ordinal - 1], nodeIndex});
        else
            result.connections.append({node.parent, nodeIndex});
    }
    if (options.topology == SpatialTopology::Loop && destinations.size() > 2)
        result.connections.append({destinations.last(), destinations.first()});
    // Keep the existing renderer's global budgets. Reserve paths and scenery
    // first; at most four illustrative models complement the source exhibits.
    int modelVertices = 40 + result.connections.size() * 4;
    int modelFaces = 32 + result.connections.size() * 2;
    int models = 0;
    for (int ordinal = 0; ordinal < destinations.size() && models < 4; ++ordinal) {
        const auto &node = graph.nodes[destinations[ordinal]];
        const auto &item = document.items[node.itemIndex];
        auto mesh = SpatialAnimalModel::assemble(item.animalForm);
        if (mesh.vertices.isEmpty() || modelVertices + mesh.vertices.size() > SpatialInterior::maximumMeshVertices
            || modelFaces + mesh.faces.size() > SpatialInterior::maximumMeshFaces)
            continue;
        const auto &transform = result.transforms[ordinal + 1];
        const float angle = transform.yaw * pi / 180;
        for (auto &vertex : mesh.vertices) {
            vertex += QVector3D(60, 106, 120);
            vertex = QVector3D(vertex.x() * std::cos(angle) - vertex.z() * std::sin(angle), vertex.y(),
                vertex.x() * std::sin(angle) + vertex.z() * std::cos(angle));
            vertex = transform.translation * options.spacing + vertex * options.spacing;
            vertex.setY(vertex.y() - result.anchors[destinations[ordinal]].position.y());
        }
        modelVertices += mesh.vertices.size();
        modelFaces += mesh.faces.size();
        result.meshes.append(mesh);
        const float ground = -result.anchors[destinations[ordinal]].position.y() / options.spacing;
        result.parts.append({ordinal + 1, {60, ground + 80, 120}, {230, 22, 190}, options.style.structure,
            SpatialSolidShape::Box, true, "Illustrative model · not to scale"});
        ++models;
    }
    float extentX = 900;
    float extentZ = 900;
    for (const auto &anchor : result.anchors) {
        extentX = qMax(extentX, std::abs(anchor.position.x()) + 1600);
        extentZ = qMax(extentZ, std::abs(anchor.position.z()) + 1600);
    }
    result.size = {extentX * 2, extentZ * 2};
    if (options.topology == SpatialTopology::Spine && !document.stableLayout)
        result.spawn.setZ(-extentZ + 350);
    if (options.features.size() == 8 && (options.features[0] > 0.3f || options.features[1] > 0.4f)) {
        // Low polygon scenery occupies the outer margin, beyond every reserved
        // destination/route. Eight hills use only 40 vertices and 32 faces.
        for (int index = 0; index < 8; ++index) {
            const float angle = index * pi / 4;
            const float x = std::sin(angle) * (extentX - 400);
            const float z = std::cos(angle) * (extentZ - 400);
            // Reserve the entrance approach as well as destination routes.
            // A 280-unit hill footprint needs another 200 units of clearance.
            if (std::abs(x - result.spawn.x()) < 480 && std::abs(z - result.spawn.z()) < 480)
                continue;
            const float height = 250 + ((options.seed + index * 53) % 6) * 55;
            SpatialMesh hill;
            hill.color = options.style.ground.darker(110 + index * 4);
            hill.vertices = {{x - 280, 0, z - 280}, {x + 280, 0, z - 280},
                {x + 280, 0, z + 280}, {x - 280, 0, z + 280}, {x, height, z}};
            hill.faces = {{0, 4, 1}, {1, 4, 2}, {2, 4, 3}, {3, 4, 0}};
            result.meshes.append(hill);
        }
    }
    return result;
}
