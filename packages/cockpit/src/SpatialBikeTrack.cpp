#include "SpatialBikeTrack.h"
#include "DirtBikeTrackArt.h"
#include "RedBudCourse.h"
#include "RedBudTerrain.h"
#include "DirtBikeRace.h"
#include <cmath>

SpatialInterior SpatialBikeTrack::create()
{
    SpatialInterior room;
    room.environmentKey = "dockpipe:mesa-moto:v1";
    // Keep the riding loop compact, with explorable woodland around it rather
    // than putting the visible world boundary directly behind each berm.
    room.size = {32000, 38000};
    room.height = 6000;
    room.outdoors = true;
    room.naturalGround = true;
    room.fogDistance = 22000;
    room.floorColor = QColor("#626f60");
    room.groundMaterial = SpatialSurfaceMaterial::Grass;
    room.skyColor = QColor("#888f9c");
    room.spawn = {-2700, -8670};
    room.exit = {0, -11950};
    const auto box = [&](QVector3D center, QVector3D size, QColor color, const QString &label = {}, bool collision = true) {
        room.solids.append({center, size, color, collision, SpatialSolidShape::Box, label});
    };
    // Physics uses the same prepared straight footprints as the visible dirt
    // course. Zero-height lanes preserve terrain height and existing jumps.
    for (float x : {-2700.f, 2700.f}) {
        SpatialRideSurface lane;
        lane.center = {x, 0, 0};
        lane.width = 1200;
        lane.color = QColor("#756c63");
        lane.profile = {{-8600, 0}, {8600, 0}};
        room.rideSurfaces.append(lane);
    }
    const auto ramp = [&](float x, float z, float heading, QVector<QPointF> profile) {
        SpatialRideSurface surface;
        surface.center = {x, 0, z};
        surface.heading = heading;
        surface.width = 1200;
        surface.color = QColor("#8a7d73");
        surface.sculpted = true;
        surface.profile = profile;
        room.rideSurfaces.append(surface);
    };
    ramp(-2700, -4200, 0, {{-900,0},{-650,35},{-400,100},{-200,160},{200,160},{450,125},{800,40},{1100,0}});
    ramp(-2700, 1500, 0, {{-1000,0},{-750,30},{-450,140},{-200,280},{400,280},{700,200},{1200,60},{1600,0}});
    ramp(2700, 3000, 3.14159265f, {{-1000,0},{-750,30},{-450,150},{-200,320},{700,320},{1000,220},{1700,60},{2100,0}});
    for (int index = 0; index < 3; ++index)
        ramp(2700, -4000 - index * 1100, 3.14159265f, {{-400,0},{-200,60},{0,95},{200,60},{400,0}});
    for (int turn = 0; turn < 2; ++turn) {
        SpatialRideSurface berm;
        berm.center = turn ? QVector3D(2700, 0, -8600) : QVector3D(-2700, 0, 8600);
        berm.heading = turn ? 3.14159265f : 0;
        berm.width = 1200;
        berm.curveRadius = 2700;
        berm.bank = .85f;
        berm.sculpted = true;
        berm.color = QColor("#8a7d73");
        const float length = berm.curveRadius * 3.14159265f;
        berm.profile = {{0, 0}, {length * .12f, 65}, {length * .28f, 120},
            {length * .5f, 120}, {length * .72f, 120}, {length * .88f, 65}, {length, 0}};
        room.rideSurfaces.append(berm);
    }
    // Markers remain outside the riding line. No invisible barriers on ramps.
    for (float side : {-1.f, 1.f}) {
        for (int index = 0; index < 9; ++index) {
            const float z = -8000 + index * 2000;
            box({side * 3350, 45, z}, {100,90,220}, index % 2 ? QColor("#eee4cd") : QColor("#ce5635"));
        }
    }
    box({-2700, 550, -7700}, {1600,130,90}, QColor("#1d333b"), "MESA MOTO · START → JUMPS");
    for (float x : {-3500.f, -1900.f})
        box({x, 260, -7700}, {55,520,55}, QColor("#e5d4af"));
    // Readable track-facing boards, outside the lane and the hill footprints.
    // The former tall slabs squeezed labels onto an 80 cm-wide front edge.
    const auto trackSign = [&](float x, float z, const QString &title, float yaw) {
        room.solids.append({{x, 230, z}, {360, 160, 24}, QColor("#254750"),
            true, SpatialSolidShape::Box, title, yaw});
        for (float offset : {-145.f, 145.f})
            box({x + offset, 85, z}, {16, 170, 16}, QColor("#b6b3a9"));
    };
    trackSign(-3700, -4500, "01 · WARM UP", 0);
    trackSign(-3700, 1500, "02 · TABLETOP", 0);
    trackSign(3700, 3000, "03 · BIG AIR", 180);
    // Pit pavilion and spectator tiers provide a sense of scale and a landmark.
    box({-5300, 434, -8200}, {1900,12,1300}, QColor("#d0c8b8"));
    for (float x : {-6100.f, -4500.f})
        for (float z : {-8700.f, -7700.f})
            box({x,220,z}, {16,440,16}, QColor("#344248"));
    // Service bench stays against the outer side, clear of the bike approach
    // and the headquarters transfer sign. Separate legs leave the space legible.
    box({-5970, 105, -8200}, {170,18,520}, QColor("#8f9695"));
    for (float z : {-8410.f, -7990.f})
        box({-5970,48,z}, {120,96,18}, QColor("#344248"));
    box({-5970,135,-8360}, {95,42,95}, QColor("#ae4530"), "TOOLS", false);
    for (float x : {-6100.f, -4500.f})
        box({x,410,-8200}, {18,18,1000}, QColor("#344248"));
    for (int row = 0; row < 3; ++row)
        box({-5200.f - row * 220, 45.f + row * 65, -6100}, {200,90.f + row * 130,1800}, QColor("#cbb99b"));
    DirtBikeTrackArt::decorate(room);
    SpatialVehicle bike;
    bike.id = "mesa-bike";
    bike.name = "Mesa 250 · Dirt bike";
    bike.body = "dirt-bike";
    bike.paint = QColor("#ed6837");
    bike.position = {-2700,0,-8400};
    room.vehicles.append(bike);
    SpatialInteriorAction home;
    home.id = "pit-home";
    home.title = "Return to headquarters · E / A";
    home.center = {-5300,160,-8200};
    home.size = {500,300,100};
    home.link = "hq";
    room.actions.append(home);
    box(home.center, home.size, QColor("#263c47"), "DOCKPIPE HQ · E / A");
    DirtBikeTrackArt::plant(room);
    return room;
}

SpatialInterior SpatialBikeTrack::createRedBud()
{
    SpatialInterior room;
    room.environmentKey = "dockpipe:redbud-moto:v1";
    room.size = {53760, 38400};
    room.height = 6500;
    room.outdoors = true;
    room.naturalGround = true;
    room.fogDistance = 23000;
    room.groundMaterial = SpatialSurfaceMaterial::Grass;
    room.floorColor = QColor("#71814d");
    room.skyColor = QColor("#a2b4be");
    room.terrain = RedBudTerrain::create();
    if (!room.terrain)
        return {};
    auto bike = DirtBikeRace::playerGrid(DirtBikeRaceCourse::Layout::RedBud);
    bike.name = "RedBud · 250 dirt bike";
    bike.position.setY(room.terrain->sample(bike.position).height);
    room.vehicles.append(bike);
    const QVector3D forward(std::sin(bike.heading), 0, std::cos(bike.heading));
    const QVector3D right(std::cos(bike.heading), 0, -std::sin(bike.heading));
    const auto spawn = bike.position - forward * 260;
    room.spawn = {spawn.x(), spawn.z()};
    room.spawnHeight = room.terrain->sample(spawn).height + 160;
    room.spawnYaw = bike.heading;
    const auto exit = spawn - forward * 900 - right * 1400;
    room.exit = {exit.x(), exit.z()};
    const auto sign = [&](QVector3D position, const QString &label, QColor color) {
        position.setY(room.terrain->sample(position).height + 250);
        room.solids.append({position, {700,160,25}, color, true, SpatialSolidShape::Box, label, -bike.heading * 180 / 3.14159265f});
    };
    sign(bike.position + forward * 4000 - right * 1600, "REDBUD · 2024 MAP STUDY", QColor("#a3302f"));
    sign(bike.position - right * 1800, "3 LAPS · G RACE · T RAIN", QColor("#263d30"));
    // Course-side flag posts follow the authored line, grounded on the same
    // terrain as the tyres. No collision walls around the lane.
    for (int index = 0; index < 36; ++index) {
        const auto point = RedBudCourse::sample(RedBudCourse::length() * index / 36, 780);
        auto base = point.position;
        base.setY(room.terrain->sample(base).height);
        room.solids.append({base + QVector3D(0,90,0), {12,180,12}, QColor("#ddd4bd"), false});
        room.solids.append({base + QVector3D(35,160,0), {70,45,8}, index % 2 ? QColor("#f3eee3") : QColor("#bb3533"), false});
    }
    for (int index = 0; index < 96; ++index) {
        const float angle = index * 2.39996323f;
        const float radius = 4500 + std::sqrt(float(index) / 96) * 19000;
        QVector3D point(std::cos(angle) * radius, 0, std::sin(angle) * radius * .7f);
        const float along = RedBudCourse::progress(point);
        const auto nearest = RedBudCourse::sample(along).position;
        if (std::hypot(point.x() - nearest.x(), point.z() - nearest.z()) < 1500)
            continue;
        point.setY(room.terrain->sample(point).height);
        room.foliage.append({point, 1100.f + (index % 4) * 180, 750.f + (index % 3) * 120});
    }
    return room;
}
