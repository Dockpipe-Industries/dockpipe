#include "FurnishingCatalog.h"
#include "SpatialAssetLibrary.h"
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <cmath>
#include <QtMath>

const QVector<FurnishingDefinition> &FurnishingCatalog::items()
{
    using Mount = FurnishingMount;
    static const QVector<FurnishingDefinition> catalog = [] {
        QVector<FurnishingDefinition> result{
        {"chair", "Simple chair", "Seating", "chair", {65, 105, 65}, QColor("#b58c64")},
        {"office-chair", "Office chair", "Seating", "chair", {75, 130, 75}, QColor("#485e79")},
        {"stool", "Stool", "Seating", "stool", {55, 70, 55}, QColor("#bb8c57")},
        {"bench", "Bench", "Seating", "chair", {200, 95, 70}, QColor("#ae875b")},
        {"couch", "Couch", "Seating", "couch", {230, 110, 95}, QColor("#637ba1")},
        {"lounge", "Lounge chair", "Seating", "couch", {110, 95, 110}, QColor("#bc8878")},
        {"table", "Small table", "Tables", "table", {120, 85, 100}, QColor("#b69670")},
        {"desk", "Desk", "Tables", "desk", {190, 90, 95}, QColor("#8399aa")},
        {"workbench", "Long workbench", "Tables", "desk", {280, 100, 100}, QColor("#bf9857")},
        {"coffee-table", "Coffee table", "Tables", "table", {150, 45, 80}, QColor("#9f7657")},
        {"side-table", "Side table", "Tables", "table", {65, 65, 65}, QColor("#ac9477")},
        {"meeting-table", "Meeting table", "Tables", "table", {300, 85, 140}, QColor("#78929d")},
        {"shelf", "Small shelf", "Storage", "shelf", {100, 130, 45}, QColor("#a48c72")},
        {"large-shelf", "Large shelf", "Storage", "shelf", {170, 230, 60}, QColor("#a48c72")},
        {"cabinet", "Cabinet", "Storage", "cabinet", {110, 130, 65}, QColor("#748d96")},
        {"locker", "Locker", "Storage", "cabinet", {65, 200, 65}, QColor("#678b9a")},
        {"crate", "Crate", "Storage", "crate", {80, 80, 80}, QColor("#ba945e")},
        {"chest", "Storage chest", "Storage", "crate", {130, 65, 70}, QColor("#94653e")},
        {"tool-cart", "Tool cart", "Storage", "shelf", {110, 95, 65}, QColor("#b75f59")},
        {"pedestal", "Pedestal stand", "Storage", "pedestal", {75, 100, 75}, QColor("#b4bdc5")},
        {"floor-lamp", "Floor lamp", "Lighting", "lamp", {70, 200, 70}, QColor("#efd599")},
        {"desk-lamp", "Desk lamp", "Lighting", "lamp", {35, 60, 35}, QColor("#f1d5a1"), Mount::Surface},
        {"wall-light", "Wall light", "Lighting", "panel", {45, 85, 14}, QColor("#f4dba5"), Mount::Wall},
        {"ceiling-light", "Ceiling light", "Lighting", "ceiling", {120, 15, 80}, QColor("#f0e5b5"), Mount::Ceiling},
        {"neon-strip", "Neon light strip", "Lighting", "panel", {200, 12, 8}, QColor("#55e0cc"), Mount::Wall},
        {"beacon", "Accent light beacon", "Lighting", "beacon", {45, 100, 45}, QColor("#6fdbd6")},
        {"plant", "Small plant", "Nature", "plant", {55, 85, 55}, QColor("#62aa7c"), Mount::Surface},
        {"large-plant", "Large plant", "Nature", "plant", {95, 180, 95}, QColor("#5caa78")},
        {"hanging-plant", "Hanging plant", "Nature", "plant", {75, 110, 75}, QColor("#78b48b"), Mount::Ceiling},
        {"tree", "Decorative tree planter", "Nature", "tree", {180, 300, 180}, QColor("#5a9c79")},
        {"rug", "Small rug", "Rugs", "rug", {180, 3, 140}, QColor("#b17681")},
        {"large-rug", "Large rug", "Rugs", "rug", {350, 3, 260}, QColor("#648eb6")},
        {"round-rug", "Round rug", "Rugs", "round", {220, 3, 220}, QColor("#ad9469")},
        {"runner", "Runner rug", "Rugs", "rug", {100, 3, 380}, QColor("#8c839e")},
        {"poster-frame", "Poster frame", "Wall art", "art", {95, 140, 6}, QColor("#b9a184"), Mount::Wall, true},
        {"poster", "Large poster", "Wall art", "art", {180, 250, 6}, QColor("#b56f91"), Mount::Wall, true},
        {"art", "Wall art panel", "Wall art", "art", {240, 150, 8}, QColor("#719dbc"), Mount::Wall, true},
        {"banner", "Banner", "Wall art", "banner", {85, 250, 6}, QColor("#9f698b"), Mount::Wall, true},
        {"sign", "Sign plaque", "Wall art", "art", {160, 60, 8}, QColor("#baa574"), Mount::Wall, true},
        {"neon-sign", "Neon sign", "Wall art", "art", {170, 80, 8}, QColor("#66dcbd"), Mount::Wall, true},
        {"wall-screen", "Wall screen", "Screens", "screen", {300, 180, 8}, QColor("#344e65"), Mount::Wall, true},
        {"monitor", "Freestanding monitor", "Screens", "monitor", {180, 210, 60}, QColor("#425c74"), Mount::Floor, true},
        {"desk-monitor", "Desk monitor", "Screens", "monitor", {100, 90, 35}, QColor("#425c74"), Mount::Surface, true},
        {"display-board", "Large display board", "Screens", "monitor", {360, 280, 80}, QColor("#425c74"), Mount::Floor, true},
        {"cinema-screen", "Cinema screen", "Screens", "monitor", {650, 390, 110}, QColor("#425c74"), Mount::Floor, true},
        {"hologram", "Holographic display stand", "Screens", "monitor", {200, 230, 80}, QColor("#65bfb4"), Mount::Floor, true},
        {"bin", "Trash bin", "Misc", "bin", {55, 85, 55}, QColor("#657f8a")},
        {"speaker", "Speaker", "Tech", "speaker", {60, 130, 55}, QColor("#344458")},
        {"clock", "Clock", "Wall art", "clock", {70, 70, 8}, QColor("#e0cba7"), Mount::Wall},
        {"trophy", "Trophy / statue", "Misc", "trophy", {65, 110, 65}, QColor("#d3ab5c"), Mount::Surface},
        {"sculpture", "Potted sculpture", "Misc", "trophy", {100, 170, 100}, QColor("#9ca6ca")},
        {"console", "Side console", "Tech", "desk", {140, 110, 70}, QColor("#52727e")},
        {"terminal", "Terminal pedestal", "Screens", "monitor", {110, 155, 75}, QColor("#52727e"), Mount::Floor, true},
        {"arcade-cabinet", "Arcade cabinet", "Screens", "arcade", {105, 220, 95}, QColor("#34364f"), Mount::Floor, true},
        {"server-small", "Small server rack", "Tech", "server", {80, 120, 80}, QColor("#425265")},
        {"server-large", "Large server rack", "Tech", "server", {100, 230, 100}, QColor("#425265")}
        };
        for (const auto &theme : {QString("Habitat"), QString("Sandstone"), QString("Timber")}) {
            QColor color("#688a9a");
            if (theme == "Sandstone")
                color = QColor("#c7a77c");
            else if (theme == "Timber")
                color = QColor("#a7764c");
            const QString prefix = theme.toLower() + "-";
            result += {
                {prefix + "floor", theme + " floor tile", "Build · " + theme, "foundation", {200, 8, 200}, color, Mount::Surface},
                {prefix + "wall", theme + " wall", "Build · " + theme, "wall", {200, 300, 16}, color, Mount::Surface},
                {prefix + "door", theme + " doorway", "Build · " + theme, "door", {200, 300, 16}, color, Mount::Surface},
                {prefix + "window", theme + " window wall", "Build · " + theme, "window", {200, 300, 16}, color, Mount::Surface},
                {prefix + "roof", theme + " roof tile", "Build · " + theme, "roof", {200, 12, 200}, color.darker(130), Mount::Surface},
                {prefix + "pillar", theme + " pillar", "Build · " + theme, "pillar", {20, 300, 20}, color.lighter(120), Mount::Surface},
                {prefix + "railing", theme + " railing", "Build · " + theme, "railing", {200, 100, 12}, color, Mount::Surface}
            };
        }
        QFile file(":/cockpit/models/catalog.json");
        if (file.open(QIODevice::ReadOnly) && file.size() <= 128 * 1024) {
            const auto entries = QJsonDocument::fromJson(file.readAll()).array();
            for (int index = 0; index < qMin(entries.size(), qsizetype(256)); ++index) {
                const auto data = entries[index].toObject();
                const auto size = data.value("size").toArray();
                if (size.size() != 3)
                    continue;
                FurnishingDefinition definition;
                definition.id = data.value("id").toString();
                definition.asset = definition.id;
                definition.name = data.value("name").toString();
                definition.category = data.value("category").toString();
                definition.family = data.value("family").toString();
                definition.size = {float(size[0].toDouble()), float(size[1].toDouble()), float(size[2].toDouble())};
                definition.color = QColor("#b9c4cf");
                const auto mount = data.value("mount").toString();
                if (mount == "wall")
                    definition.mount = Mount::Wall;
                else if (mount == "surface")
                    definition.mount = Mount::Surface;
                else if (mount == "ceiling")
                    definition.mount = Mount::Ceiling;
                result.append(definition);
            }
        }
        return result;
    }();
    return catalog;
}
const FurnishingDefinition *FurnishingCatalog::find(const QString &id)
{
    for (const auto &item : items()) {
        if (item.id == id)
            return &item;
    }
    return nullptr;
}
QVector3D FurnishingCatalog::rotate(const QVector3D &point, float yaw)
{
    const float angle = qDegreesToRadians(yaw);
    return {point.x() * std::cos(angle) - point.z() * std::sin(angle), point.y(),
        point.x() * std::sin(angle) + point.z() * std::cos(angle)};
}
QVector3D FurnishingCatalog::size(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    if (item && item->screen && object.screenSize.isValid())
        return {float(object.screenSize.width()), float(object.screenSize.height()), 8};
    return item ? item->size : QVector3D{};
}
QVector<SpatialSolid> FurnishingCatalog::parts(const FurnishingObject &object)
{
    QVector<SpatialSolid> result;
    const auto *item = find(object.type);
    if (!item || object.removed)
        return result;
    const auto dimensions = size(object);
    const float w = dimensions.x(), h = dimensions.y(), d = dimensions.z();
    auto part = [&](QVector3D center, QVector3D extent, QColor color, SpatialSolidShape shape = SpatialSolidShape::Box) {
        result.append({object.position + rotate(center - QVector3D(0, h / 2, 0), object.yaw), extent, color,
            item->family != "rug" && item->family != "round", shape, {}, object.yaw});
    };
    const auto color = item->color;
    const auto dark = color.darker(175);
    const QString family = object.baseDisplay.isEmpty() ? item->family : "screen";
    if (!item->asset.isEmpty()) {
        // Meshes keep their original silhouette; a single box owns collision.
        part({0, h / 2, 0}, {w, h, d}, color);
    } else if (family == "door" || family == "window") {
        const float sideWidth = family == "door" ? 35 : 20;
        for (int side : {-1, 1})
            part({side * (w - sideWidth) / 2, h / 2, 0}, {sideWidth, h, d}, color);
        part({0, h - 20, 0}, {w - 2 * sideWidth, 40, d}, color);
        if (family == "window")
            part({0, 50, 0}, {w - 2 * sideWidth, 100, d}, color);
    } else if (family == "railing") {
        part({0, h - 5, 0}, {w, 10, d}, color);
        for (int side : {-1, 1})
            part({side * (w / 2 - 6), h / 2, 0}, {12, h, d}, dark);
    } else if (family == "wall") {
        part({0, h / 2, 0}, {w, h, d}, color);
        part({0, h - 12, -d / 2 - 2}, {w, 12, 4}, color.lighter(135));
        part({0, 12, -d / 2 - 2}, {w, 12, 4}, dark);
    } else if (family == "chair" || family == "stool" || family == "table" || family == "desk") {
        const float top = family == "chair" ? h * 0.5f : h - 8;
        part({0, top, 0}, {w, 12, d}, color);
        for (int side : {-1, 1}) {
            part({side * (w / 2 - 8), top / 2, -d / 2 + 8}, {10, top, 10}, dark);
            part({side * (w / 2 - 8), top / 2, d / 2 - 8}, {10, top, 10}, dark);
        }
        if (family == "chair")
            part({0, h * 0.8f, d / 2 - 6}, {w, h * 0.4f, 12}, color);
        if (family == "desk")
            part({-w * 0.3f, h * 0.5f, 0}, {w * 0.25f, h * 0.7f, d * 0.85f}, dark);
    } else if (family == "couch") {
        part({0, h * 0.3f, 0}, {w, h * 0.6f, d}, dark);
        part({0, h * 0.58f, -d * 0.1f}, {w * 0.8f, h * 0.18f, d * 0.7f}, color);
        part({0, h * 0.8f, d * 0.4f}, {w, h * 0.4f, d * 0.2f}, color);
        for (int side : {-1, 1})
            part({side * w * 0.45f, h * 0.55f, 0}, {w * 0.1f, h * 0.7f, d}, color);
    } else if (family == "shelf" || family == "server") {
        for (int side : {-1, 1})
            part({side * (w / 2 - 5), h / 2, 0}, {10, h, d}, dark);
        for (int level = 0; level < 4; ++level)
            part({0, 6 + level * (h - 12) / 3, 0}, {w, 10, d}, color);
        if (family == "server")
            part({0, h / 2, -d / 2 + 6}, {w * 0.7f, h * 0.8f, 8}, QColor("#75ceba"));
    } else if (family == "plant" || family == "tree" || family == "trophy") {
        part({0, h * 0.15f, 0}, {w * 0.6f, h * 0.3f, d * 0.6f}, QColor("#a58d76"));
        part({0, h * 0.5f, 0}, {w * 0.12f, h * 0.6f, d * 0.12f}, dark);
        part({0, h * 0.7f, 0}, {w, h * 0.6f, d}, color, SpatialSolidShape::Octahedron);
    } else if (family == "lamp" || family == "beacon" || family == "pedestal") {
        part({0, 5, 0}, {w, 10, d}, dark);
        part({0, h * 0.45f, 0}, {w * 0.15f, h * 0.85f, d * 0.15f}, dark);
        part({0, h * 0.9f, 0}, {w, h * 0.2f, d}, color);
    } else if (family == "arcade") {
        part({0, 48, 6}, {w, 96, 78}, dark);
        for (int side : {-1, 1})
            part({side * 49.0f, 150, 8}, {7, 140, 78}, color);
        part({0, 151, 31}, {92, 110, 12}, dark);
        part({0, 203, -3}, {w, 34, 85}, color);
        part({0, 101, -13}, {w, 13, 69}, QColor("#59516e"));
        part({-27, 115, -24}, {9, 20, 9}, QColor("#f07b97"));
        part({23, 110, -25}, {29, 5, 11}, QColor("#58d7c5"));
    } else if (family == "monitor") {
        part({0, 6, 0}, {w * 0.55f, 12, d}, dark);
        part({0, h * 0.3f, 0}, {14, h * 0.6f, 14}, dark);
        part({0, h * 0.7f, 0}, {w, h * 0.6f, 10}, color);
    } else if (family == "art" || family == "clock" || family == "banner" || family == "speaker") {
        part({0, h / 2, 0}, {w, h, d}, dark);
        if (!item->screen) {
            part({0, h / 2, -d / 2 - 1}, {w * 0.85f, h * 0.85f, 2}, color);
            part({0, h * 0.52f, -d / 2 - 3}, {w * 0.5f, h * 0.08f, 2}, color.lighter(145));
        }
    } else if (family == "round") {
        part({0, h / 2, 0}, {w * 0.71f, h, d * 0.71f}, color);
        auto corner = object;
        corner.yaw += 45;
        result.append({object.position, {w * 0.71f, h, d * 0.71f}, color, false, SpatialSolidShape::Box, {}, corner.yaw});
    } else {
        part({0, h / 2, 0}, {w, h, d}, color);
        if (family == "cabinet" || family == "crate" || family == "bin")
            part({0, h * 0.8f, -d / 2 - 1}, {w * 0.6f, 6, 2}, dark);
    }
    return result;
}

SpatialMesh FurnishingCatalog::mesh(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    if (!item || item->asset.isEmpty() || object.removed)
        return {};
    auto result = SpatialAssetLibrary::model(item->asset);
    for (auto &vertex : result.vertices)
        vertex = object.position + rotate(vertex, object.yaw);
    return result;
}

int FurnishingCatalog::faceCount(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    if (item && !item->asset.isEmpty())
        return SpatialAssetLibrary::model(item->asset).faces.size();
    int count = 0;
    for (const auto &part : parts(object))
        count += part.shape == SpatialSolidShape::Octahedron ? 8 : 6;
    return count;
}

bool FurnishingCatalog::modular(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    return item && item->category.startsWith("Build · ");
}
SpatialDisplay FurnishingCatalog::display(const FurnishingObject &object)
{
    SpatialDisplay result;
    result.id = object.baseDisplay.isEmpty() ? "furnishing:" + object.id : object.baseDisplay;
    result.freestanding = true;
    result.center = object.position;
    result.yaw = object.yaw;
    const auto dimensions = size(object);
    const auto *item = find(object.type);
    if (item && item->family == "arcade") {
        result.center += rotate({0, 37, -32}, object.yaw);
        result.size = {86, 76};
    } else if (object.baseDisplay.isEmpty() && item && item->family == "monitor") {
        result.center += rotate({0, dimensions.y() * 0.2f, -7}, object.yaw);
        result.size = {dimensions.x() - 12, dimensions.y() * 0.6f - 12};
    } else {
        result.center += rotate({0, 0, -6}, object.yaw);
        result.size = {dimensions.x(), dimensions.y()};
    }
    if (item && item->category == "Wall art") {
        result.center = object.position + rotate({0, 0, -dimensions.z() / 2 - 5}, object.yaw);
        result.size = {dimensions.x() * 0.85, dimensions.y() * 0.85};
    }
    if (object.binding.provider == "application")
        result.application = object.binding.application;
    result.prompt = "Select to connect or configure with Furnish / F2";
    return result;
}

SpatialDisplay FurnishingCatalog::marquee(const FurnishingObject &object)
{
    SpatialDisplay result;
    result.id = "furnishing:" + object.id + ":marquee";
    result.freestanding = true;
    result.center = object.position + rotate({0, 93, -48}, object.yaw);
    result.yaw = object.yaw;
    result.size = {89, 24};
    result.prompt = "Configure marquee artwork with Furnish / F2";
    return result;
}

int FurnishingCatalog::screenCount(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    if (!item || !item->screen)
        return 0;
    return item->family == "arcade" ? 2 : 1;
}

QVector<SpatialInteriorAction> FurnishingCatalog::actions(const FurnishingObject &object)
{
    const auto *item = find(object.type);
    if (!item || object.removed || item->category != "Seating")
        return {};
    const auto dimensions = size(object);
    const float seatHeight = item->family == "stool" ? dimensions.y() - 8 : dimensions.y() * 0.55f;
    const auto seat = object.position + rotate({0, seatHeight - dimensions.y() / 2, -8}, object.yaw);
    SpatialInteriorAction action;
    action.id = "furnishing-seat:" + object.id;
    action.title = "Sit · " + item->name;
    action.kind = SpatialActionKind::Seat;
    action.center = seat + rotate({0, 25, -dimensions.z() / 2}, object.yaw);
    action.size = {dimensions.x(), 45, 40};
    action.destination = seat + QVector3D(0, 75, 0);
    action.yaw = qDegreesToRadians(180 - object.yaw);
    return {action};
}
