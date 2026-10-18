#include "WorkspaceFlightView.h"
#include "SpatialBikeTrack.h"
#include "SpatialReturnPortal.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialTerrain.h"
#include <limits>
#include <cmath>
#include <QApplication>
#include <QElapsedTimer>
#include <QThread>
#include <cstdio>

#define CHECK_PORTAL(value) do { if (!(value)) { std::fprintf(stderr, "Portal assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

int returnPortalTest()
{
    SpatialInterior room;
    room.environmentKey = "fixture:return-portal-regression";
    room.size = {2400,2400};
    room.spawn = {0,0};
    room.exit = room.spawn;
    WorkspaceFlightView scene({}, false);
    scene.setAttribute(Qt::WA_DontShowOnScreen);
    scene.resize(320,200);
    int returns = 0;
    scene.orbitRequested = [&] { ++returns; };
    scene.setInterior(room, "Arrival portal regression");
    scene.show();
    scene.setPilotInput({0,0,1}, {}, false);
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 200) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
    scene.suspendPilot();
    std::printf("portal_arrival forward_returns=%d\n", returns);
    CHECK_PORTAL(returns == 0);

    // This used to exit anywhere behind the portal plane, even 10 m away.
    room.spawn = {0,-1000};
    room.exit = {0,0};
    room.size = {4000,4000};
    scene.setInterior(room, "Bounded portal regression");
    scene.setPilotInput({0,0,1}, {}, false);
    timer.restart();
    while (timer.elapsed() < 200) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
    CHECK_PORTAL(returns == 0);
    scene.setPilotInput({0,0,1}, {}, true);
    timer.restart();
    while (timer.elapsed() < 2500 && returns == 0) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
    scene.suspendPilot();
    scene.hide();
    CHECK_PORTAL(returns == 1); // Deliberately walking into the opening still exits.

    using namespace SpatialReturnPortal;
    const QVector3D opening(0,160,0);
    CHECK_PORTAL(!entered(opening, {0,160,10}, opening, interiorRadius));
    CHECK_PORTAL(!entered({0,160,-1500}, {0,160,-1400}, opening, interiorRadius));
    CHECK_PORTAL(!entered({0,800,-100}, {0,800,100}, opening, interiorRadius));
    CHECK_PORTAL(!entered({200,160,-100}, {200,160,100}, opening, interiorRadius));
    CHECK_PORTAL(!entered({0,160,-100}, {0,160,-100}, opening, interiorRadius));
    CHECK_PORTAL(entered({0,160,-100}, {0,160,-50}, opening, interiorRadius));
    CHECK_PORTAL(entered({0,160,-300}, {0,160,300}, opening, interiorRadius));
    CHECK_PORTAL(!entered({0,160,-100}, {0,160,0}, opening, std::numeric_limits<float>::quiet_NaN()));

    room.spawn = {};
    room.exit = {};
    const QVector3D arrival(0,160,0);
    room.solids.append({{0,120,-500},{250,240,200},Qt::gray,true});
    place(room, arrival, 0);
    CHECK_PORTAL(room.returnPortalWalkEntry);
    CHECK_PORTAL(std::hypot(room.exit.x(), room.exit.y()) >= arrivalClearance);
    CHECK_PORTAL(!SpatialInteriorGeometry::blocked(room, center(room)));
    const auto previousExit = room.exit;
    place(room, arrival, 0);
    CHECK_PORTAL(room.exit == previousExit); // Safe authored positions are stable.
    const auto streamedArrival = center(room);
    place(room, streamedArrival, 1.2f);
    CHECK_PORTAL(std::hypot(room.exit.x() - streamedArrival.x(), room.exit.y() - streamedArrival.z()) >= arrivalClearance);

    SpatialInterior tiny;
    tiny.size = {300,300};
    tiny.spawn = {};
    tiny.exit = {};
    place(tiny, arrival, 0);
    CHECK_PORTAL(!tiny.returnPortalWalkEntry);

    SpatialTerrain::Configuration configuration;
    configuration.columns = 9;
    configuration.rows = 9;
    room.height = 2000;
    room.terrain = SpatialTerrain::create(configuration, [](float,float) {
        return SpatialTerrain::Seed{800,0};
    });
    CHECK_PORTAL(room.terrain && std::abs(center(room).y() - 960) < .01f);
    auto redbud = SpatialBikeTrack::createRedBud();
    CHECK_PORTAL(std::hypot(redbud.exit.x() - redbud.spawn.x(), redbud.exit.y() - redbud.spawn.y()) >= arrivalClearance);
    const QVector3D redbudArrival(redbud.spawn.x(), redbud.spawnHeight, redbud.spawn.y());
    place(redbud, redbudArrival, redbud.spawnYaw);
    CHECK_PORTAL(redbud.returnPortalWalkEntry);
    CHECK_PORTAL(std::abs(center(redbud).y() - redbud.terrain->sample(center(redbud)).height - 160) < .01f);
    returns = 0;
    scene.setInterior(redbud, "RedBud arrival regression");
    scene.setUpdatesEnabled(false); // Exercise real movement without the software terrain painter.
    scene.show();
    const auto redbudStart = scene.cameraPosition();
    scene.setPilotInput({0,0,1}, {}, false);
    timer.restart();
    while (timer.elapsed() < 200) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
    scene.suspendPilot();
    scene.hide();
    const auto movement = scene.cameraPosition() - redbudStart;
    CHECK_PORTAL(returns == 0 && std::hypot(movement.x(), movement.z()) > 5);
    std::printf("redbud_arrival forward_returns=%d travel_cm=%.1f\n", returns, std::hypot(movement.x(), movement.z()));
    std::printf("return_portals: safe arrivals, bounded entry, deliberate exit, streaming, obstacles and terrain height passed\n");
    return 0;
}
