#include "SpatialCivicWorld.h"
#include "FurnishingCatalog.h"
#include <algorithm>
#include <cmath>

namespace {
class Builder {
public:
    SpatialInterior room;

    Builder()
    {
        room.size = {10000, 9200};
        room.height = 4200;
        room.outdoors = true;
        room.naturalGround = true;
        room.floorColor = QColor("#252b38");
        room.skyColor = QColor("#54677e");
        room.exit = {0, -4500};
    }

    void box(QVector3D position, QVector3D size, const char *color, QString label = {}, bool collision = true)
    {
        room.solids.append({position, size, QColor(color), collision, SpatialSolidShape::Box, label});
    }

    void action(QString id, QString title, QVector3D center, SpatialActionKind kind,
        QVector3D destination = {}, QString link = {})
    {
        room.actions.append({id, title, center, {120, 150, 120}, kind, destination, 0, link});
    }

    void seat(QString id, float x, float z, float floor = 0)
    {
        box({x, floor + 45, z}, {140, 90, 140}, "#693048");
        box({x, floor + 110, z - 60}, {140, 110, 30}, "#943f59");
        action(id, "Sit · E / A", {x, floor + 190, z}, SpatialActionKind::Seat, {x, floor + 190, z});
    }

    // Architectural detail shares one mesh per material. It costs no collision
    // objects, textures, light passes or work after the room is constructed.
    void panel(QVector<QVector3D> corners, const char *color)
    {
        const QColor material(color);
        auto mesh = std::find_if(room.meshes.begin(), room.meshes.end(),
            [&](const SpatialMesh &value) { return value.color == material; });
        if (mesh == room.meshes.end()) {
            room.meshes.append({{}, {}, material});
            mesh = room.meshes.end() - 1;
        }
        const int first = mesh->vertices.size();
        mesh->vertices += corners;
        mesh->faces.append({first, first + 1, first + 2, first + 3});
    }

    void frontage(float x, float y, float z, float width, float height, const char *color)
    {
        panel({{x - width / 2, y, z}, {x + width / 2, y, z},
            {x + width / 2, y + height, z}, {x - width / 2, y + height, z}}, color);
    }

    void gateway(QString id, QString title, float x, float z, QString link)
    {
        // A street-sized building with an open central entrance, layered cornice,
        // illuminated marquee and tall glazing, rather than a freestanding gate.
        box({x - 480, 550, z + 220}, {500, 1100, 620}, "#8b7b70");
        box({x + 480, 550, z + 220}, {500, 1100, 620}, "#8b7b70");
        box({x, 950, z + 220}, {1460, 300, 620}, "#4c5363");
        box({x, 590, z - 160}, {1500, 140, 330}, "#b89559", title);
        frontage(x, 1060, z - 95, 1500, 45, "#d1b78b");
        frontage(x, 520, z - 330, 1460, 22, "#ffe5af");
        frontage(x, 645, z - 330, 1460, 16, "#ffe5af");
        for (float side : {-1.f, 1.f}) {
            frontage(x + side * 480, 60, z - 95, 360, 400, "#d1b78b");
            frontage(x + side * 480, 90, z - 98, 300, 330, "#31465a");
            frontage(x + side * 675, 680, z - 95, 30, 350, "#ffe5af");
        }
        windows(x, z - 95, 1000, 740, 1);
        panel({{x - 680, 11, z - 800}, {x + 680, 11, z - 800},
            {x + 500, 11, z - 250}, {x - 500, 11, z - 250}}, "#675347");
        frontage(x, 0, z + 500, 460, 800, "#31465a");
        frontage(x, 80, z + 495, 360, 580, "#d1b78b");
        action(id, title + " · Enter", {x, 170, z - 100}, SpatialActionKind::Destination, {}, link);
    }

    void windows(float x, float z, float width, float firstHeight, int rows = 2)
    {
        for (int row = 0; row < rows; ++row) {
            for (int column = 0; column < 3; ++column)
                frontage(x + (column - 1) * width / 3, firstHeight + row * 260,
                    z, 120, 160, "#d1b78b");
        }
    }

    void furnishing(const QString &type, QVector3D position, float yaw = 0)
    {
        FurnishingObject object;
        object.type = type;
        object.position = position;
        object.yaw = yaw;
        room.solids += FurnishingCatalog::parts(object);
    }

    void street()
    {
        // The boulevard crosses the lobby doors. Pavements and markings are
        // shallow non-blocking geometry; distant buildings are six reusable boxes.
        box({0, 4, -2000}, {9800, 8, 1400}, "#171d29", {}, false);
        box({0, 10, -1200}, {9800, 20, 200}, "#89909b", {}, false);
        box({0, 10, -2800}, {9800, 20, 200}, "#89909b", {}, false);
        for (int index = 0; index < 6; ++index) {
            const float x = -4000 + index * 1600;
            const float height = 1200 + (index % 3) * 460;
            box({x, height / 2, -3650}, {1300, height, 900}, index % 2 ? "#354359" : "#4c4049");
            windows(x, -3190, 1000, 600);
            frontage(x, height - 60, -3190, 1320, 55, "#d1b78b");
            frontage(x, 60, -3190, 950, 300, "#31465a");
            box({x, 12, -2000}, {600, 5, 18}, "#d9b874", {}, false);
        }
        for (const float x : {-1900.f, 1900.f}) {
            box({x, 240, -1220}, {24, 480, 24}, "#111b29");
            box({x, 480, -1220}, {100, 35, 100}, "#ffe6ae", {}, false);
        }
        box({1800, 24, -2000}, {420, 48, 420}, "#263b50", "DRONE PAD");
        box({1800, 95, -2000}, {190, 90, 160}, "#84c4ce");
        action("drone", "Board drone · E / A", {1800, 190, -2000}, SpatialActionKind::Drone, {1800, 320, -2000});
    }

    void shell(const QString &title, bool company = false)
    {
        // Auditorium and lobby share a wide, uninterrupted centre aisle.
        const float depth = company ? 4100 : 5400;
        const float center = -1100 + depth / 2;
        const char *wall = company ? "#22292f" : "#352532";
        box({-1500, 800, center}, {60, 1600, depth}, wall);
        box({1500, 800, center}, {60, 1600, depth}, wall);
        box({0, 800, -1100 + depth}, {3000, 1600, 60}, company ? "#111519" : "#241c2a");
        box({0, 1620, center}, {3060, 40, depth}, "#17202e");
        for (float side : {-1.f, 1.f}) {
            box({side * 950, 330, -1100}, {1100, 660, 70}, "#354459");
            box({side * 950, 300, 0}, {1100, 600, 60}, wall);
            box({side * 1100, 60, -640}, {450, 120, 180}, "#425568");
            box({side * 1100, 150, -720}, {450, 180, 40}, "#617083");
        }
        box({0, 730, -1200}, {3060, 140, 400}, "#b89559", title);
        frontage(0, 650, -1405, 3060, 24, "#ffe5af");
        frontage(0, 790, -1405, 3060, 18, "#ffe5af");
        frontage(0, 1510, -1135, 3060, 65, "#d1b78b");
        for (float side : {-1.f, 1.f}) {
            frontage(side * 950, 90, -1138, 950, 480, "#31465a");
            frontage(side * 950, 950, -1135, 900, 360, "#d1b78b");
            frontage(side * 1450, 70, -1140, 35, 1400, "#ffe5af");
            // Gold-edged wall bays and luminous coves frame the long interior.
            panel({{side * 1460, 1370, 100}, {side * 1460, 1370, 3900},
                {side * 1460, 1400, 3900}, {side * 1460, 1400, 100}}, "#ffe5af");
            for (float z : {1300.f, 2700.f}) {
                panel({{side * 1460, 450, z}, {side * 1460, 450, z + 650},
                    {side * 1460, 1250, z + 650}, {side * 1460, 1250, z}}, "#675347");
            }
        }
        panel({{-350, 13, -1400}, {350, 13, -1400}, {350, 13, 3900}, {-350, 13, 3900}}, "#675347");
        box({0, 4, -550}, {2950, 8, 1050}, "#737b87", {}, false);
        box({0, 12, 1500}, {340, 6, 3000}, company ? "#213a31" : "#783a47", {}, false);
        box({0, 1060, -550}, {520, 60, 380}, company ? "#a4eccd" : "#ffdf9c", {}, false);
    }
};
}

SpatialInterior SpatialCivicWorld::cinema(const SpatialWebDocument &document)
{
    Builder builder;
    builder.room.spawn = {0, -650};
    builder.street();
    builder.shell("GRAND CINEMA · LOBBY");
    builder.box({0, 45, 4050}, {2800, 90, 350}, "#4d2b40");
    for (float side : {-1.f, 1.f}) {
        builder.box({side * 1350, 800, 4160}, {220, 1500, 160}, "#8e3450");
        builder.box({side * 1420, 350, 2200}, {20, 32, 3600}, "#edbd77", {}, false);
        for (float z : {1500.f, 3000.f}) {
            builder.box({side * 1460, 750, z}, {30, 260, 160}, "#b99a68");
            builder.box({side * 1435, 750, z}, {20, 180, 80}, "#ffe3a3", {}, false);
        }
    }
    for (float z : {1500.f, 2900.f}) {
        builder.panel({{-700, 1560, z - 200}, {700, 1560, z - 200},
            {700, 1560, z + 200}, {-700, 1560, z + 200}}, "#d1b78b");
        builder.panel({{-600, 1558, z - 130}, {600, 1558, z - 130},
            {600, 1558, z + 130}, {-600, 1558, z + 130}}, "#ffe5af");
    }
    // Two Art Deco ceiling ribs: fixed twelve-segment arches, cached with the
    // other architectural meshes and still within the 512-vertex world budget.
    for (float z : {900.f, 3200.f}) {
        for (int segment = 0; segment < 12; ++segment) {
            const float a = segment * 3.14159265f / 12;
            const float b = (segment + 1) * 3.14159265f / 12;
            const QVector3D left(1440 * std::cos(a), 940 + 570 * std::sin(a), z);
            const QVector3D right(1440 * std::cos(b), 940 + 570 * std::sin(b), z);
            builder.panel({left, right, right + QVector3D(0, 35, 0), left + QVector3D(0, 35, 0)}, "#d1b78b");
        }
    }
    builder.box({0, 1510, 4160}, {2800, 60, 160}, "#d1aa68");
    SpatialDisplay screen;
    screen.id = "cinema-screen";
    screen.application.name = "Grand screen · Select a film from the wall";
    screen.freestanding = true;
    screen.center = {0, 810, 4000};
    screen.size = {2400, 1350};
    builder.room.displays.append(screen);
    for (int row = 0; row < 3; ++row) {
        for (int column = 0; column < 4; ++column) {
            const float x = (column - 1.5f) * 340 + (column < 2 ? -180 : 180);
            builder.seat(QString("stalls-%1-%2").arg(row).arg(column), x, 1100 + row * 560);
        }
    }
    // A real raised floor, supported by the walking collider. Lifts avoid steep
    // stairs and give keyboard/controller users the same accessible route.
    builder.box({0, 420, 480}, {2900, 40, 850}, "#394454");
    builder.room.walkSurfaces.append({QRectF(-1450, 55, 2900, 850), 440});
    builder.box({0, 500, 900}, {2900, 120, 30}, "#b89965");
    for (int column = 0; column < 4; ++column)
        builder.seat(QString("balcony-%1").arg(column), (column - 1.5f) * 420, 580, 440);
    builder.box({-1260, 180, -110}, {160, 360, 200}, "#697581", "BALCONY LIFT");
    builder.action("balcony-up", "Balcony lift · E / A", {-1260, 170, -260}, SpatialActionKind::Transfer, {-1100, 600, 300});
    builder.box({-1260, 760, 90}, {220, 80, 40}, "#b89965", "LOBBY LIFT", false);
    builder.action("balcony-down", "Lobby lift · E / A", {-1260, 610, 220}, SpatialActionKind::Transfer, {-1100, 160, -250});
    for (int index = 0; index < qMin(11, int(document.items.size())); ++index) {
        const float side = index % 2 == 0 ? -1 : 1;
        SpatialDisplay poster;
        poster.id = document.items[index].id;
        poster.application.name = document.items[index].title;
        poster.freestanding = true;
        poster.center = {side * 1460, 240, float(-760 + index / 2 * 560)};
        poster.size = {340, 212};
        poster.yaw = -side * 90;
        builder.room.displays.append(poster);
    }
    builder.gateway("hq", "DOCKPIPE HQ", -2800, -1100, "hq");
    builder.gateway("search", "CITY DIRECTORY", 2800, -1100, "search");
    return builder.room;
}

SpatialInterior SpatialCivicWorld::headquarters()
{
    Builder builder;
    builder.room.environmentKey = "dockpipe:headquarters:v1";
    builder.room.spawn = {0, -650};
    builder.street();
    builder.shell("DOCKPIPE INDUSTRIES INC. · HEADQUARTERS", true);
    builder.room.floorColor = QColor("#191e23");
    builder.box({0, 90, 1600}, {900, 180, 220}, "#b79b6d", "RECEPTION · EXPLORE THE SPATIAL WEB");
    builder.box({0, 700, 2900}, {2200, 1100, 60}, "#111519", "DOCKPIPE INDUSTRIES INC.");
    builder.furnishing("sculpture", {850, 85, 2300});
    SpatialDisplay brand;
    brand.id = "hq-brand";
    brand.application.name = "Dockpipe Industries Inc.";
    brand.freestanding = true;
    brand.center = {0, 650, 2850};
    brand.size = {1100, 1100};
    brand.prompt = "Welcome home";
    builder.room.displays.append(brand);
    for (float side : {-1.f, 1.f}) {
        builder.furnishing("tree", {side * 1100, 150, 2100});
        builder.furnishing("couch", {side * 750, 55, 900});
        builder.furnishing("coffee-table", {side * 750, 22.5f, 530});
        builder.furnishing("floor-lamp", {side * 1100, 100, 1000});
    }
    builder.gateway("track", "REDBUD · DIRT BIKE TRACK", -4400, -3200, "track");
    builder.gateway("cinema", "GRAND CINEMA", -2800, -1100, "https://www.youtube.com");
    builder.gateway("search", "CITY DIRECTORY · SEARCH", 2800, -1100, "search");
    builder.action("reception", "Search the web · E / A", {0, 220, 1450}, SpatialActionKind::Destination, {}, "search");
    return builder.room;
}

SpatialInterior SpatialCivicWorld::city(const SpatialWebDocument &document)
{
    Builder builder;
    // Arrive facing the first result, rather than the empty gap between shops.
    builder.room.spawn = {-4000, -2700};
    builder.street();
    // Stable slots keep the search ranking legible. Every result has its source
    // title and poster at street level, plus a real doorway into that website.
    for (int index = 0; index < qMin(12, int(document.items.size())); ++index) {
        const auto &item = document.items[index];
        const float x = -4000 + (index % 6) * 1600;
        const float z = index < 6 ? -1000 : 1500;
        builder.box({x - 430, 320, z}, {360, 640, 460}, "#4f5266");
        builder.box({x + 430, 320, z}, {360, 640, 460}, "#4f5266");
        builder.box({x, 600, z}, {1220, 100, 460}, "#bb9b69", item.detail.isEmpty() ? item.url.host() : item.detail);
        const float height = 700 + (index % 3) * 260;
        builder.box({x, 650 + height / 2, z + 200}, {1300, height, 900},
            index % 2 ? "#66514c" : "#394858");
        builder.windows(x, z - 260, 1100, 720, 1);
        builder.frontage(x, 520, z - 245, 1220, 22, "#ffe5af");
        builder.frontage(x, 650 + height - 40, z - 260, 1320, 45, "#d1b78b");
        builder.frontage(x - 430, 60, z - 245, 300, 400, "#31465a");
        SpatialDisplay display;
        display.id = item.id;
        display.application.name = item.title;
        display.freestanding = true;
        display.center = {x + 430, 240, z - 245};
        display.size = {320, 200};
        display.portal = !item.url.isEmpty();
        display.portalCenter = {x, 160, z - 260};
        display.portalApproachDistance = 160;
        builder.room.displays.append(display);
    }
    builder.action("hq", "Return to Dockpipe HQ · E / A", {0, 160, -2900}, SpatialActionKind::Destination, {}, "hq");
    return builder.room;
}

QImage SpatialCivicWorld::headquartersSign()
{
    // Exact company-site artwork, bundled once. Runtime uses the same bounded
    // texture path as website displays; HQ needs no network request or browser.
    return QImage(":/cockpit/branding/dockpipe-industries.png")
        .scaled(320, 320, Qt::KeepAspectRatio, Qt::SmoothTransformation);
}
