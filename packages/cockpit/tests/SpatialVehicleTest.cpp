#include "SpatialWebWorld.h"
#include "SpatialInteriorGeometry.h"
#include "WorkspaceFlightView.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QThread>
#include <QKeyEvent>
#include <limits>
#include <cstdio>

#define VERIFY_VEHICLE(value) do { if (!(value)) { std::fprintf(stderr, "Vehicle assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
bool waitFor(const std::function<bool()> &condition, int milliseconds = 2000)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (!condition() && elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    return condition();
}
}

int spatialVehicleTest()
{
    SpatialVehicle vehicle;
    vehicle.id = "0";
    vehicle.name = "2026 Example pickup";
    vehicle.body = "pickup";
    vehicle.year = 2026;
    for (const QString &body : {"pickup", "suv", "van", "sedan", "coupe"}) {
        auto sample = vehicle;
        sample.body = body;
        const auto faces = SpatialVehicleModel::build(sample);
        VERIFY_VEHICLE(!faces.isEmpty() && faces.size() <= SpatialVehicleModel::maximumFaces);
        std::printf("vehicle_body=%s faces=%lld\n", qPrintable(body), static_cast<long long>(faces.size()));
        for (const auto &face : faces) {
            VERIFY_VEHICLE(face.vertices.size() >= 3 && face.vertices.size() <= 4);
            for (const auto &point : face.vertices)
                VERIFY_VEHICLE(point.y() >= 0 && point.y() <= SpatialVehicleModel::size(sample).y());
        }
    }
    auto invalid = vehicle;
    invalid.heading = std::numeric_limits<float>::quiet_NaN();
    VERIFY_VEHICLE(!SpatialVehicleModel::valid(invalid) && SpatialVehicleModel::build(invalid).isEmpty());
    invalid = vehicle;
    invalid.year = 9000;
    VERIFY_VEHICLE(!SpatialVehicleModel::valid(invalid));

    SpatialWebDocument document;
    document.title = "Vehicle inventory";
    document.url = QUrl("https://example.org/vehicles");
    document.venue = "showroom";
    for (int index = 0; index < 12; ++index) {
        SpatialWebItem item;
        item.id = QString::number(index);
        item.title = "2026 Pickup " + item.id;
        item.kind = "vehicle";
        item.vehicleBody = "pickup";
        item.vehicleYear = 2026;
        item.url = QUrl("https://example.org/trucks/" + item.id);
        document.items.append(item);
    }
    auto options = SpatialWebWorld::optionsFor(document);
    for (auto topology : {SpatialTopology::Branches, SpatialTopology::Loop, SpatialTopology::Spine}) {
        options.topology = topology;
        const auto result = SpatialWebWorld::construct(document, options);
        std::printf("vehicle_topology=%d success=%d fallback=%d diagnostic=%s\n", int(topology), result.success, result.fallback, qPrintable(result.diagnostic));
        VERIFY_VEHICLE(result.success && !result.fallback);
        VERIFY_VEHICLE(result.interior.vehicles.size() == 12 && result.interior.displays.size() == 12);
        VERIFY_VEHICLE(result.interior.locomotion == "walk");
        for (const auto &car : result.interior.vehicles)
            VERIFY_VEHICLE(!SpatialVehicleModel::blocked(result.interior, car));
    }
    auto plan = SpatialWebWorld::plan(document, SpatialWebGraph::fromDocument(document), options);
    plan.vehicles.append(plan.vehicles.first());
    VERIFY_VEHICLE(!SpatialWebWorld::compile(plan).success);
    plan.vehicles.removeLast();
    plan.vehicles[1].position = plan.vehicles[0].position;
    VERIFY_VEHICLE(!SpatialWebWorld::compile(plan).success);

    SpatialInterior room;
    room.size = {6000, 6000};
    room.vehicles = {vehicle};
    room.spawn = {0, -1000};
    // A thin post beside the driver's centre still hits the vehicle's footprint.
    room.solids = {{{95, 80, 0}, {5, 160, 5}, Qt::gray}};
    VERIFY_VEHICLE(SpatialVehicleModel::blocked(room, vehicle));
    room.solids.clear();
    VERIFY_VEHICLE(!SpatialVehicleModel::blocked(room, vehicle));
    VERIFY_VEHICLE(SpatialInteriorGeometry::blocked(room, {0, 160, 0}));
    VERIFY_VEHICLE(SpatialInteriorGeometry::target(room, {0, 100, -500}, {0, 0, 1}) == "vehicle:0");
    room.solids = {{{0, 100, -350}, {400, 200, 20}, Qt::gray}};
    VERIFY_VEHICLE(SpatialInteriorGeometry::target(room, {0, 100, -500}, {0, 0, 1}).isEmpty());
    room.solids.clear();

    WorkspaceFlightView scene({}, false);
    scene.resize(900, 600);
    scene.show();
    scene.setInterior(room, "Vehicle test");
    VERIFY_VEHICLE(!scene.enterVehicle("0"));
    scene.setSurfaceSpawn({-200, 0});
    VERIFY_VEHICLE(scene.enterVehicle("0"));
    VERIFY_VEHICLE(scene.activeVehicleId() == "0" && scene.interiorLocomotion() == "drive");
    const auto start = scene.vehicles()[0].position;
    scene.setPilotInput({0, 0, 1}, {}, false);
    VERIFY_VEHICLE(waitFor([&]() { return scene.vehicles()[0].position.z() > start.z() + 60; }));
    scene.setPilotInput({1, 0, 1}, {}, false);
    VERIFY_VEHICLE(waitFor([&]() { return scene.vehicles()[0].position.x() > start.x() + 10; }));
    // Real keyboard braking, and a mouse/stick look does not turn the wheels.
    const auto heading = scene.vehicles()[0].heading;
    scene.setPilotInput({}, {1, 0}, false);
    QKeyEvent brake(QEvent::KeyPress, Qt::Key_Space, Qt::NoModifier);
    QApplication::sendEvent(&scene, &brake);
    QElapsedTimer braking;
    braking.start();
    auto lastBrakePosition = scene.vehicles()[0].position;
    int stationary = 0;
    VERIFY_VEHICLE(waitFor([&]() {
        const auto position = scene.vehicles()[0].position;
        stationary = position == lastBrakePosition ? stationary + 1 : 0;
        lastBrakePosition = position;
        return braking.elapsed() > 300 && stationary > 8;
    }, 1000));
    QKeyEvent release(QEvent::KeyRelease, Qt::Key_Space, Qt::NoModifier);
    QApplication::sendEvent(&scene, &release);
    scene.setPilotInput({}, {}, false);
    VERIFY_VEHICLE(waitFor([&]() { return !scene.moving(); }, 4000));
    VERIFY_VEHICLE(scene.vehicles()[0].heading == heading);
    const auto parked = scene.vehicles()[0].position;
    VERIFY_VEHICLE(scene.exitVehicle());
    VERIFY_VEHICLE(scene.activeVehicleId().isEmpty() && scene.interiorLocomotion() == "walk");
    VERIFY_VEHICLE(scene.vehicles()[0].position == parked);
    VERIFY_VEHICLE(!SpatialVehicleModel::contains(scene.vehicles()[0], scene.cameraPosition(), 25));
    VERIFY_VEHICLE(scene.enterVehicle("0"));
    scene.goBack();
    VERIFY_VEHICLE(scene.activeVehicleId().isEmpty() && scene.walking());

    // At full acceleration a thin wall stops the bonnet before it crosses.
    room.solids = {{{0, 100, 650}, {3000, 200, 5}, Qt::gray}};
    scene.setInterior(room, "Collision");
    scene.setSurfaceSpawn({-200, 0});
    VERIFY_VEHICLE(scene.enterVehicle("0"));
    scene.setPilotInput({0, 0, 1}, {}, false);
    VERIFY_VEHICLE(waitFor([&]() { return scene.vehicles()[0].position.z() > 370; }, 3000));
    QElapsedTimer push;
    push.start();
    while (push.elapsed() < 400) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_VEHICLE(scene.vehicles()[0].position.z() < 394);
    scene.setPilotInput({}, {}, false);
    VERIFY_VEHICLE(scene.exitVehicle());
    // Both doors and the front/rear are blocked: stay seated, never teleport out.
    room.solids = {{{-200, 100, 0}, {40, 200, 800}, Qt::gray},
        {{200, 100, 0}, {40, 200, 800}, Qt::gray}, {{0, 100, -320}, {500, 200, 40}, Qt::gray},
        {{0, 100, 320}, {500, 200, 40}, Qt::gray}};
    scene.setInterior(room, "Blocked doors");
    scene.setSurfaceSpawn({-150, 0});
    VERIFY_VEHICLE(scene.enterVehicle("0"));
    VERIFY_VEHICLE(!scene.exitVehicle() && scene.activeVehicleId() == "0");
    VERIFY_VEHICLE(scene.worldStatus().contains("No clear place"));
    scene.setInterior({}, "Replacement world");
    VERIFY_VEHICLE(scene.vehicles().isEmpty() && scene.activeVehicleId().isEmpty() && !scene.moving());
    std::puts("Vehicle geometry, composition, occlusion, entry, steering, parking, collision and reset passed.");
    return 0;
}
