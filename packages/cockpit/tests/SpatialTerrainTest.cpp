#include "SpatialTerrain.h"
#include "SpatialBikeTrack.h"
#include "RedBudCourse.h"
#include "DirtBikeDynamics.h"
#include "DirtBikeRace.h"
#include "SpatialGpuRenderer.h"
#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include <cstdio>
#include <cmath>
#include <limits>

#define CHECK_TERRAIN(value) do { if (!(value)) { std::fprintf(stderr, "Terrain assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

int spatialTerrainTest()
{
    SpatialTerrain::Configuration configuration;
    configuration.columns = 17;
    configuration.rows = 17;
    configuration.rainRate = .06f;
    configuration.wettingRate = .035f;
    const auto basin = [](float x, float z) {
        return SpatialTerrain::Seed{100 + (x * x + z * z) / 12000, 1};
    };
    auto dry = SpatialTerrain::create(configuration, basin);
    CHECK_TERRAIN(dry);
    auto invalid = configuration;
    invalid.columns = std::numeric_limits<int>::max();
    CHECK_TERRAIN(!SpatialTerrain::create(invalid, basin));
    invalid = configuration;
    invalid.soilSubdivisions = 100;
    CHECK_TERRAIN(!SpatialTerrain::create(invalid, basin));
    CHECK_TERRAIN(!SpatialTerrain::create(configuration, [](float, float) {
        return SpatialTerrain::Seed{std::numeric_limits<float>::quiet_NaN(), 1};
    }));
    const QVector3D forward(0,0,1);
    const auto fresh = dry->sample({0,0,0});
    dry->wheelContact({}, forward, 0, 1, 0);
    dry->wheelContact({}, forward, 10, 0, 0);
    CHECK_TERRAIN(dry->sample({}).rut == 0);
    for (int pass = 0; pass < 10; ++pass) {
        for (int distance = -200; distance <= 200; distance += 10)
            dry->wheelContact({0,0,float(distance)}, forward, 10, .5f, 0);
    }
    const auto rut = dry->sample({});
    CHECK_TERRAIN(rut.rut > 1 && rut.rut < 16 && rut.height < fresh.height - 1);
    CHECK_TERRAIN(dry->sample({120,0,0}).rut == 0);
    auto wet = SpatialTerrain::create(configuration, basin);
    wet->setRaining(true);
    for (int tick = 0; tick < 2400; ++tick)
        wet->advance(.05f);
    const auto pooled = wet->sample({});
    const auto high = wet->sample({1200,0,1200});
    std::printf("terrain_water low=%.3f high=%.3f wetness=%.3f grip=%.3f total=%.3f\n",
        pooled.water, high.water, pooled.wetness, pooled.grip, wet->totalWater());
    CHECK_TERRAIN(pooled.water > high.water + 1 && pooled.wetness > .9f && pooled.grip < fresh.grip);
    for (int pass = 0; pass < 10; ++pass) {
        for (int distance = -200; distance <= 200; distance += 10)
            wet->wheelContact({0,0,float(distance)}, forward, 10, .5f, 0);
    }
    CHECK_TERRAIN(wet->sample({}).rut > rut.rut * 1.5f);
    const float remaining = wet->totalWater();
    wet->setRaining(false);
    wet->advance(std::numeric_limits<float>::infinity());
    CHECK_TERRAIN(wet->totalWater() == remaining);
    for (int tick = 0; tick < 1200; ++tick)
        wet->advance(.05f);
    CHECK_TERRAIN(wet->totalWater() < remaining && wet->sample({}).water > 0);
    const float persistentRut = wet->sample({}).rut;
    for (int tick = 0; tick < 120; ++tick)
        wet->advance(.05f);
    CHECK_TERRAIN(wet->sample({}).rut == persistentRut);
    CHECK_TERRAIN(!wet->meshes({})[2].faces.isEmpty());
    CHECK_TERRAIN(wet->meshes({std::numeric_limits<float>::quiet_NaN(),0,0}).isEmpty());
    const auto meshes = dry->meshes({});
    bool contactVertex = false;
    for (const auto &mesh : meshes) {
        for (const auto &vertex : mesh.vertices) {
            if (mesh.material != SpatialSurfaceMaterial::Plain
                && std::hypot(vertex.x(), vertex.z()) < 300) {
                CHECK_TERRAIN(std::abs(vertex.y() - dry->sample(vertex).height) < .001f);
                contactVertex = true;
            }
        }
    }
    CHECK_TERRAIN(contactVertex);
    // Verify interior triangle contact, not just matching mesh vertices.
    for (const auto &mesh : meshes) {
        for (const auto &face : mesh.faces) {
            const auto a = mesh.vertices[face[0]];
            const auto b = mesh.vertices[face[1]];
            const auto c = mesh.vertices[face[2]];
            const auto point = (a + b + c) / 3;
            if (std::hypot(point.x(), point.z()) < 300)
                CHECK_TERRAIN(std::abs(point.y() - dry->sample(point).height) < .01f);
        }
    }
    SpatialTerrain::Configuration flatConfiguration = configuration;
    auto flatSeed = [](float, float) { return SpatialTerrain::Seed{100,1}; };
    auto dryFlat = SpatialTerrain::create(flatConfiguration, flatSeed);
    auto wetFlat = SpatialTerrain::create(flatConfiguration, flatSeed);
    wetFlat->setRaining(true);
    for (int tick = 0; tick < 600; ++tick)
        wetFlat->advance(.05f);
    SpatialInterior dryRoom;
    dryRoom.size = {10000,10000};
    dryRoom.terrain = dryFlat;
    SpatialInterior wetRoom = dryRoom;
    wetRoom.terrain = wetFlat;
    SpatialVehicle flatVehicle;
    flatVehicle.id = "traction-proof";
    flatVehicle.body = "dirt-bike";
    flatVehicle.position = {0,100,0};
    DirtBikeDynamics dryBike;
    DirtBikeDynamics wetBike;
    dryBike.reset(flatVehicle);
    wetBike.reset(flatVehicle);
    for (int tick = 0; tick < 60; ++tick) {
        dryBike.advance({1,0,0,false}, 1.f/60, dryRoom);
        wetBike.advance({1,0,0,false}, 1.f/60, wetRoom);
    }
    std::printf("terrain_physical_grip dry_speed=%.2f wet_speed=%.2f\n", dryBike.state().speed, wetBike.state().speed);
    CHECK_TERRAIN(!dryBike.state().crashed && !wetBike.state().crashed);
    CHECK_TERRAIN(wetBike.state().speed < dryBike.state().speed * .95f);
    SpatialInterior soilRoom;
    soilRoom.size = {3840,3840};
    soilRoom.terrain = dry;
    SpatialVehicle vehicle;
    vehicle.id = "generic-wheel-test";
    vehicle.body = "dirt-bike";
    vehicle.position = {500,dry->sample({500,0,0}).height,0};
    DirtBikeDynamics bike;
    bike.reset(vehicle);
    const int before = dry->wornCells();
    for (int tick = 0; tick < 60; ++tick)
        bike.advance({1,0,0,false}, 1.f / 60, soilRoom);
    CHECK_TERRAIN(dry->wornCells() > before && !bike.state().crashed);

    auto room = SpatialBikeTrack::createRedBud();
    CHECK_TERRAIN(room.terrain && room.vehicles.size() == 1 && room.rideSurfaces.isEmpty());
    WorkspaceFlightView scene({}, false);
    scene.setInterior(room, "RedBud admission");
    const auto camera = scene.cameraPosition();
    const auto bikeCenter = room.vehicles[0].position + QVector3D(0,67.5f,0);
    std::printf("redbud_admission vehicles=%d distance=%.1f target=%s\n", int(scene.vehicles().size()),
        (camera - bikeCenter).length(), qPrintable(SpatialInteriorGeometry::target(room, camera, (bikeCenter-camera).normalized())));
    CHECK_TERRAIN(scene.terrain() != room.terrain.get());
    CHECK_TERRAIN(scene.enterVehicle("mesa-bike"));
    CHECK_TERRAIN(scene.startBikeRace());
    const float length = RedBudCourse::length();
    float low = 10000;
    float highPoint = 0;
    for (int index = 0; index < 128; ++index) {
        const float distance = length * index / 128;
        const auto point = RedBudCourse::sample(distance);
        CHECK_TERRAIN(std::abs(std::remainder(RedBudCourse::progress(point.position) - distance, length)) < 40);
        const float height = room.terrain->sample(point.position).height;
        low = qMin(low, height);
        highPoint = qMax(highPoint, height);
    }
    CHECK_TERRAIN(length > 150000 && length < 300000 && highPoint - low > 600);
    int packedVertices = 0;
    for (const auto &mesh : room.terrain->meshes(room.vehicles[0].position))
        packedVertices += mesh.faces.size() * 6;
    // Reserve the existing three 4,086-face riders, scenery and water headroom.
    CHECK_TERRAIN(packedVertices + 3 * 4086 * 6 + 12000 < SpatialGpuRenderer::maximumVertices);
    std::printf("redbud_course length_cm=%.1f elevation_range_cm=%.1f terrain_packed_vertices=%d\n",
        length, highPoint - low, packedVertices);

    DirtBikeRace race;
    race.start(19, DirtBikeRaceCourse::Layout::RedBud);
    room.vehicles[0].position += QVector3D(1500,0,0); // Keep the stationary player off the racing line.
    for (const auto &rival : race.rivals())
        room.vehicles.append(rival.vehicle);
    DirtBikeDynamics player;
    player.reset(room.vehicles[0]);
    for (int tick = 0; tick < 18000; ++tick) {
        race.advance(.05f, player.state(), room);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
        if (race.rivals()[0].progress.finished() && race.rivals()[1].progress.finished())
            break;
    }
    for (const auto &rival : race.rivals()) {
        std::printf("redbud_rival laps=%d gate=%d finish=%.2f recoveries=%d speed=%.1f\n",
            rival.progress.laps(), rival.progress.nextGate(), rival.progress.finishTime(), rival.recoveries, rival.bike.state().speed);
        CHECK_TERRAIN(rival.progress.finished() && rival.recoveries < 12);
    }
    std::printf("terrain: reusable seed, tyre wear, wet grip, runoff, drainage, matching contact mesh and RedBud laps passed\n");
    return 0;
}
