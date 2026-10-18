#include "SpatialOperationsWorld.h"
#include <QJsonArray>
#include <QPainter>
#include <QSet>
#include <algorithm>

namespace {
QColor statusColor(const QJsonObject &node)
{
    switch (node.value("status").toInt()) {
    case 1: return QColor("#65e2b2");
    case 3: return QColor("#f3c468");
    case 4: return QColor("#f37f84");
    default: return QColor("#7e96ab");
    }
}

void solid(SpatialInterior &room, const QVector3D &center, const QVector3D &size,
    const QColor &color, SpatialSolidShape shape = SpatialSolidShape::Box)
{
    room.solids.append({center, size, color, true, shape});
}

void sign(SpatialOperationsScene &scene, const QString &id, const QString &title,
    const QString &subtitle, const QVector3D &center, const QColor &color, bool portal = false)
{
    SpatialDisplay display;
    display.id = id;
    display.application.name = title.left(120);
    display.prompt = subtitle.left(200);
    display.freestanding = true;
    display.center = center;
    display.size = {320, 150};
    display.portal = portal;
    display.portalCenter = center;
    display.portalApproachDistance = portal ? 125 : 0;
    scene.interior.displays.append(display);
    if (scene.signs.size() >= 12)
        return;
    QImage image(512, 240, QImage::Format_ARGB32_Premultiplied);
    image.fill(QColor("#101d2a"));
    QPainter painter(&image);
    painter.fillRect(0, 0, 512, 8, color);
    QFont font = painter.font();
    font.setPixelSize(25);
    font.setBold(true);
    painter.setFont(font);
    painter.setPen(Qt::white);
    painter.drawText(QRect(20, 24, 472, 82), Qt::AlignLeft | Qt::TextWordWrap, title.left(100));
    font.setPixelSize(18);
    font.setBold(false);
    painter.setFont(font);
    painter.setPen(color);
    painter.drawText(QRect(20, 115, 472, 110), Qt::AlignLeft | Qt::TextWordWrap, subtitle.left(200));
    scene.signs.insert(id, image);
}

bool admittedEdge(const QString &section, const QJsonObject &edge, const QMap<QString, QJsonObject> &nodes)
{
    const QString from = edge.value("from").toString();
    const QString to = edge.value("to").toString();
    if (!nodes.contains(from) || !nodes.contains(to))
        return false;
    if (section != "containers")
        return true;
    const QString sourceKind = nodes.value(from).value("kind").toString();
    const QString targetKind = nodes.value(to).value("kind").toString();
    return !edge.value("dependency").toBool()
        && ((sourceKind == "network" && targetKind == "container")
            || (sourceKind == "container" && targetKind == "volume"));
}
}

SpatialOperationsScene SpatialOperationsWorld::campus(const QString &section, const QJsonObject &snapshot, int page)
{
    SpatialOperationsScene scene;
    auto &room = scene.interior;
    room.outdoors = true;
    room.size = {4200, 15000};
    room.height = 1100;
    room.spawn = {0, -7000};
    room.exit = {0, -7300};
    room.floorColor = QColor("#182b32");
    room.skyColor = QColor(section == "containers" ? "#182d40" : "#241f3d");
    QMap<QString, QJsonObject> nodes;
    const auto inputNodes = snapshot.value("nodes").toArray();
    for (int index = 0; index < qMin(inputNodes.size(), qsizetype(1024)); ++index) {
        const auto node = inputNodes[index].toObject();
        const QString kind = node.value("kind").toString();
        const QString id = node.value("id").toString();
        if (id.isEmpty() || (section == "containers" && kind != "container" && kind != "network" && kind != "volume"))
            continue;
        nodes.insert(id, node);
    }
    const auto ids = nodes.keys();
    scene.pages = qMax(1, (int(ids.size()) + pageSize - 1) / pageSize);
    scene.page = qBound(0, page, scene.pages - 1);
    QStringList selected;
    for (int index = scene.page * pageSize; index < qMin(int(ids.size()), (scene.page + 1) * pageSize); ++index)
        selected.append(ids[index]);
    const QSet<QString> primary(selected.begin(), selected.end());
    QVector<QJsonObject> edges;
    const auto inputEdges = snapshot.value("edges").toArray();
    for (int index = 0; index < qMin(inputEdges.size(), qsizetype(4096)); ++index) {
        const auto edge = inputEdges[index].toObject();
        if (!admittedEdge(section, edge, nodes))
            continue;
        edges.append(edge);
        const QString from = edge.value("from").toString();
        const QString to = edge.value("to").toString();
        if (selected.size() < pageSize + maximumContextNodes && primary.contains(from) && !selected.contains(to))
            selected.append(to);
        if (selected.size() < pageSize + maximumContextNodes && primary.contains(to) && !selected.contains(from))
            selected.append(from);
    }
    QMap<QString, QVector3D> positions;
    int rows[3] = {0, 0, 0};
    for (const auto &id : selected) {
        const auto node = nodes.value(id);
        const QString kind = node.value("kind").toString();
        int lane = 1;
        if (kind == "network" || kind == "workflow")
            lane = 0;
        else if (kind == "volume" || kind == "process" || kind == "vm")
            lane = 2;
        const float x = (lane - 1) * 850;
        const float z = -5100 + rows[lane]++ * 600;
        const QColor accent = statusColor(node);
        positions.insert(id, {x, 320, z});
        if (kind == "container") {
            solid(room, {x, 100, z}, {450, 200, 330}, QColor("#456072"));
            solid(room, {x, 235, z}, {470, 70, 350}, accent.darker(180), SpatialSolidShape::Pyramid);
            solid(room, {x + 160, 290, z + 80}, {65, 230, 65}, QColor("#758391"));
            solid(room, {x - 110, 100, z - 169}, {125, 160, 8}, accent.darker(190));
        } else if (kind == "network") {
            solid(room, {x, 35, z}, {490, 70, 380}, QColor("#264d64"));
            solid(room, {x, 160, z + 60}, {100, 250, 100}, QColor("#51bcdf"));
            solid(room, {x, 310, z + 60}, {260, 65, 120}, QColor("#8de8f4"));
        } else if (kind == "volume") {
            solid(room, {x, 25, z}, {490, 50, 380}, QColor("#5b4a39"));
            solid(room, {x - 110, 145, z}, {170, 240, 250}, QColor("#ca9b63"));
            solid(room, {x + 110, 145, z}, {170, 240, 250}, QColor("#ba8853"));
        } else {
            solid(room, {x, 40, z}, {450, 80, 350}, QColor("#414258"));
            solid(room, {x, 160, z + 55}, {300, 240, 200}, accent.darker(170),
                kind == "workflow" ? SpatialSolidShape::Pyramid : SpatialSolidShape::Box);
            solid(room, {x, 300, z + 55}, {90, 90, 90}, accent, SpatialSolidShape::Octahedron);
        }
        sign(scene, "resource:" + id, node.value("title").toString(),
            kind + " · " + node.value("state").toString() + "\nSelect for controls", {x, 170, z - 190}, accent);
    }
    for (const auto &edge : edges) {
        const QString from = edge.value("from").toString();
        const QString to = edge.value("to").toString();
        if (!positions.contains(from) || !positions.contains(to) || room.connections.size() >= SpatialInterior::maximumConnections)
            continue;
        const bool mount = nodes.value(to).value("kind").toString() == "volume";
        const auto source = nodes.value(from);
        const auto target = nodes.value(to);
        // Network activity follows the attached container; storage follows its owner.
        const bool active = mount ? source.value("active").toBool() : target.value("active").toBool();
        room.connections.append({positions.value(from), positions.value(to),
            QColor(mount ? "#e7b46f" : "#66dcf2"), active, from, to, edge.value("label").toString()});
    }
    sign(scene, "operations:controls", "Operations console", "Open existing controls and full topology", {-500, 170, -6500}, QColor("#9adfd8"));
    if (scene.page > 0)
        sign(scene, "operations:previous", "Previous sector", "Select to travel", {-1100, 170, -6500}, QColor("#91a8c9"));
    if (scene.page + 1 < scene.pages)
        sign(scene, "operations:next", "Next sector", "Select to travel", {1100, 170, -6500}, QColor("#91a8c9"));
    scene.notice = QString("Sector %1/%2 · %3/%4 resources · %5/%6 connections shown. ")
        .arg(scene.page + 1).arg(scene.pages).arg(selected.size()).arg(nodes.size()).arg(room.connections.size()).arg(edges.size());
    scene.notice += section == "containers"
        ? "Cyan: network attachments. Amber: named-volume mounts. Pulses mean container running, not measured traffic. Dependencies and bind mounts remain in controls. "
        : "Links show managed workflow/task/process relationships. Pulses mean running, not measured traffic. ";
    if (nodes.isEmpty())
        scene.notice += "No resources in the current snapshot. Open Operations console for discovery status. ";
    if (inputNodes.size() > 1024 || inputEdges.size() > 4096)
        scene.notice += "Source limited to 1,024 resources and 4,096 connections. ";
    scene.notice += snapshot.value("notice").toString().left(2048);
    return scene;
}

QVector<QJsonObject> SpatialOperationsWorld::machines(const QJsonObject &snapshot)
{
    QVector<QJsonObject> result;
    const auto nodes = snapshot.value("nodes").toArray();
    for (int index = 0; index < qMin(nodes.size(), qsizetype(1024)); ++index) {
        const auto node = nodes[index].toObject();
        const auto kind = node.value("kind").toString();
        if (kind == "machine" || kind == "computer")
            result.append(node);
    }
    std::sort(result.begin(), result.end(), [](const auto &first, const auto &second) {
        return first.value("id").toString() < second.value("id").toString();
    });
    return result;
}

SpatialOperationsScene SpatialOperationsWorld::laboratory(const QJsonObject &machine, bool inside,
    const QString &nickname, const QColor &color, int body)
{
    SpatialOperationsScene scene;
    auto &room = scene.interior;
    room.size = {2400, 2600};
    room.height = 650;
    room.spawn = {0, -900};
    room.exit = {0, -1200};
    room.outdoors = !inside;
    room.naturalGround = !inside;
    room.floorColor = QColor(inside ? "#1e2e3e" : "#26423c");
    room.skyColor = QColor(inside ? "#111e32" : "#1b344c");
    const QString title = machine.value("title").toString();
    if (!inside) {
        solid(room, {-450, 240, 150}, {400, 480, 650}, QColor("#49677b"));
        solid(room, {450, 240, 150}, {400, 480, 650}, QColor("#49677b"));
        solid(room, {0, 510, 150}, {1400, 90, 750}, color);
        solid(room, {0, 240, 475}, {1400, 480, 60}, QColor("#263f58"));
        sign(scene, "machine:enter", title + " · Laboratory", "Walk into the entrance or select to enter", {0, 175, -240}, color, true);
        scene.notice = "Approach the laboratory entrance to meet this machine’s android. " + machine.value("state").toString();
        return scene;
    }
    solid(room, {0, 35, 140}, {470, 70, 420}, QColor("#47697b"));
    const float width = body == 1 ? 175 : 120;
    const float torsoHeight = body == 2 ? 170 : 140;
    solid(room, {0, 245, 140}, {width, torsoHeight, 85}, color);
    solid(room, {0, 365, 140}, {100, 90, 85}, color.lighter(130),
        body == 2 ? SpatialSolidShape::Octahedron : SpatialSolidShape::Box);
    solid(room, {0, 370, 95}, {80, 20, 8}, QColor("#b5f9f6"));
    for (int side : {-1, 1}) {
        solid(room, {float(side * 42), 120, 140}, {44, 130, 60}, color.darker(150));
        solid(room, {side * (width / 2 + 32), 235, 140}, {42, 145, 55}, color.darker(120));
    }
    sign(scene, "machine:customize", nickname.isEmpty() ? title : nickname,
        "Customize android\nAppearance is stored locally", {-310, 180, 20}, color);
    sign(scene, "operations:machine-controls", title,
        machine.value("state").toString() + "\nOpen machine controls", {310, 180, 20}, QColor("#8ee2e2"));
    scene.notice = title + " · " + machine.value("state").toString()
        + " · Pairing/access status is not an online-health check. Select the left console to customize your android.";
    return scene;
}
