#include "DirtBikeRace.h"
#include "SpatialBikeTrack.h"
#include "SpatialWorkspace.h"
#include <cmath>
#include <cstdio>
#include <limits>

#define CHECK_RACE(value) do { if (!(value)) { std::fprintf(stderr, "Race assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

int dirtBikeRaceTest()
{
    SpatialInterior flat;
    flat.size = {100000, 100000};
    flat.groundMaterial = SpatialSurfaceMaterial::Dirt;
    auto vehicle = DirtBikeRace::playerGrid();
    vehicle.position = {};
    DirtBikeDynamics wheelie;
    DirtBikeDynamics neutral;
    wheelie.reset(vehicle);
    neutral.reset(vehicle);
    for (int frame = 0; frame < 60; ++frame) {
        wheelie.advance({1, 0, 0, false}, 1.f / 60, flat);
        neutral.advance({1, 0, 0, false}, 1.f / 60, flat);
    }
    bool lifted = false;
    for (int frame = 0; frame < 90; ++frame) {
        wheelie.advance({1, 0, 1, false}, 1.f / 60, flat);
        neutral.advance({1, 0, 0, false}, 1.f / 60, flat);
        if (wheelie.state().pitch > .35f && wheelie.state().frontSupport < .03f
            && wheelie.state().rearSupport > .2f) {
            lifted = true;
            break;
        }
    }
    std::printf("bike_wheelie lifted=%d pitch=%.3f front=%.3f rear=%.3f neutral_pitch=%.3f\n",
        lifted, wheelie.state().pitch, wheelie.state().frontSupport,
        wheelie.state().rearSupport, neutral.state().pitch);
    CHECK_RACE(lifted && !wheelie.state().crashed && !neutral.state().crashed);
    CHECK_RACE(neutral.state().frontSupport > .1f && neutral.state().pitch < .2f);
    const float raised = wheelie.state().pitch;
    DirtBikeDynamics::Input lower;
    lower.rearBrake = true;
    for (int frame = 0; frame < 40; ++frame)
        wheelie.advance(lower, 1.f / 60, flat);
    std::printf("bike_wheelie_lower pitch=%.3f front=%.3f crashed=%d\n",
        wheelie.state().pitch, wheelie.state().frontSupport, wheelie.state().crashed);
    CHECK_RACE(!wheelie.state().crashed && wheelie.state().pitch < raised - .1f);
    CHECK_RACE(wheelie.state().frontSupport > .1f);
    DirtBikeDynamics stopped;
    stopped.reset(vehicle);
    for (int frame = 0; frame < 120; ++frame)
        stopped.advance({0, 0, 1, false}, 1.f / 60, flat);
    CHECK_RACE(!stopped.state().crashed && stopped.state().pitch < .25f);

    using namespace DirtBikeRaceCourse;
    for (int index = 0; index < 128; ++index) {
        const float distance = length * index / 128;
        const auto point = sample(distance);
        CHECK_RACE(std::abs(std::remainder(progress(point.position) - distance, length)) < 20);
    }
    DirtBikeRaceProgress laps;
    laps.reset(sample(start - 10).position);
    for (float distance = 0; distance <= length * 3 + 100; distance += 100)
        laps.advance(sample(start + distance).position, distance / 1000);
    CHECK_RACE(laps.finished() && laps.laps() == 3 && laps.finishTime() > 0);
    DirtBikeRaceProgress backwards;
    backwards.reset(sample(start).position);
    for (float distance = 0; distance <= length * 3; distance += 100)
        backwards.advance(sample(start - distance).position, distance / 1000);
    CHECK_RACE(backwards.laps() == 0 && !backwards.finished());
    DirtBikeRaceProgress shortcut;
    shortcut.reset(sample(start - 20).position);
    for (int index = 0; index < 100; ++index) {
        shortcut.advance(sample(start + 20).position, index);
        shortcut.advance(sample(start - 20).position, index);
    }
    CHECK_RACE(shortcut.laps() == 0 && shortcut.nextGate() == 1);
    shortcut.advance(sample(start + gateSpacing * 4).position, 101);
    CHECK_RACE(shortcut.laps() == 0 && shortcut.nextGate() == 1);

    // Tick and recover a second driver heavily between calls to the first.
    // Its decisions must not consume the first rider's state or random stream.
    DirtBikeRivalDriver independent;
    DirtBikeRivalDriver reference;
    DirtBikeRivalDriver other;
    independent.reset(0, 1234);
    reference.reset(0, 1234);
    other.reset(1, 9876);
    DirtBikeDynamics::State sensed;
    sensed.speed = 1800;
    bool distinct = false;
    for (int frame = 0; frame < 120; ++frame) {
        const auto location = sample(frame % 2 ? straight - 2000 : 1000);
        sensed.position = location.position;
        sensed.heading = location.heading;
        const auto expected = reference.controls(sensed, 1.f / 60, flat, "first");
        for (int tick = 0; tick < 7; ++tick)
            other.controls(sensed, 1.f / 60, flat, "second");
        if (frame % 9 == 0)
            other.recovered();
        const auto actual = independent.controls(sensed, 1.f / 60, flat, "first");
        CHECK_RACE(actual.steering == expected.steering && actual.throttle == expected.throttle
            && actual.serviceBrake == expected.serviceBrake && actual.pitch == expected.pitch);
        distinct |= independent.commitment() != other.commitment();
    }
    CHECK_RACE(distinct && independent.decisions() > 0);

    DirtBikeRace race;
    race.start();
    auto room = SpatialBikeTrack::create();
    room.vehicles.clear();
    room.vehicles.append(DirtBikeRace::playerGrid());
    for (const auto &rival : race.rivals())
        room.vehicles.append(rival.vehicle);
    DirtBikeDynamics player;
    player.reset(DirtBikeRace::playerGrid());
    const auto grid = race.rivals()[0].vehicle.position;
    race.advance(std::numeric_limits<float>::infinity(), player.state(), room);
    CHECK_RACE(race.countdown() == 3);
    for (int frame = 0; frame < 150; ++frame)
        race.advance(1.f / 60, player.state(), room);
    CHECK_RACE(race.phase() == DirtBikeRace::Phase::Countdown);
    CHECK_RACE(race.rivals()[0].vehicle.position == grid);
    CHECK_RACE(race.place(player.state().position) == 1);
    for (int frame = 0; frame < 36000; ++frame) {
        race.advance(1.f / 60, player.state(), room);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
        if (race.rivals()[0].progress.finished() && race.rivals()[1].progress.finished())
            break;
    }
    for (const auto &rival : race.rivals()) {
        std::printf("bike_rival %s laps=%d next=%d finish=%.2f recoveries=%d x=%.1f z=%.1f speed=%.1f\n",
            qPrintable(rival.vehicle.name), rival.progress.laps(), rival.progress.nextGate(),
            rival.progress.finishTime(), rival.recoveries, rival.vehicle.position.x(),
            rival.vehicle.position.z(), rival.bike.state().speed);
        CHECK_RACE(rival.progress.finished() && rival.recoveries <= 3
            && rival.progress.finishTime() < 145);
    }
    for (int frame = 0; frame < 180; ++frame) {
        race.advance(1.f / 60, player.state(), room);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
    }
    for (const auto &rival : race.rivals())
        CHECK_RACE(!rival.bike.state().reversing && std::abs(rival.bike.state().speed) < 1);
    CHECK_RACE(race.place(player.state().position) == 3);
    const auto recovery = race.recoverPlayer(room);
    CHECK_RACE(std::isfinite(recovery.position.y()) && race.lap() == 1);
    race.cancel();
    const auto paused = race.rivals()[0].vehicle.position;
    race.advance(1, player.state(), room);
    CHECK_RACE(race.rivals()[0].vehicle.position == paused);
    race.start();
    CHECK_RACE(race.phase() == DirtBikeRace::Phase::Countdown && race.elapsed() == 0);
    CHECK_RACE(race.rivals()[0].progress.laps() == 0 && race.rivals()[0].recoveries == 0);
    for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
        room.vehicles[index + 1] = race.rivals()[index].vehicle;
    auto playerPath = player.state();
    for (int frame = 0; frame < 61; ++frame)
        race.advance(.05f, playerPath, room);
    // Script only the external player positions to qualify event bookkeeping;
    // the rival bikes continue to use physical simulation throughout.
    for (float distance = 0; distance <= length * 3 + 100; distance += 100) {
        playerPath.position = sample(start + distance).position;
        race.advance(.05f, playerPath, room);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
    }
    CHECK_RACE(race.phase() == DirtBikeRace::Phase::Finished && race.lap() == 3);
    CHECK_RACE(race.place(playerPath.position) == 1);
    const float finish = race.elapsed();
    for (int frame = 0; frame < 36000 && race.active(); ++frame) {
        race.advance(1.f / 60, playerPath, room);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
    }
    CHECK_RACE(!race.active() && race.elapsed() == finish);
    for (const auto &rival : race.rivals())
        CHECK_RACE(rival.progress.finished() && !rival.bike.state().reversing);
    // Fixed seeds exercise separate line/brake choices against real jumps,
    // berms, collisions and the parked player. No scripted crashes or boosts.
    int crashes = 0;
    int cleanFinishes = 0;
    for (std::uint32_t seed : {7u, 19u, 41u, 83u}) {
        race.start(seed);
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index)
            room.vehicles[index + 1] = race.rivals()[index].vehicle;
        std::array<bool, DirtBikeRace::rivalCount> wasCrashed{};
        std::array<float, DirtBikeRace::rivalCount> peakSpeed{};
        for (int frame = 0; frame < 12000; ++frame) {
            race.advance(1.f / 60, player.state(), room);
            for (int index = 0; index < DirtBikeRace::rivalCount; ++index) {
                const auto &rival = race.rivals()[index];
                const auto &state = rival.bike.state();
                if (state.crashed && !wasCrashed[index]) {
                    CHECK_RACE(state.crashReason != DirtBikeDynamics::CrashReason::None);
                    ++crashes;
                    std::printf("bike_rival_crash seed=%u rider=%d reason=%d commitment=%.3f\n",
                        seed, index, int(state.crashReason), rival.driver.commitment());
                }
                wasCrashed[index] = state.crashed;
                peakSpeed[index] = qMax(peakSpeed[index], state.speed);
                room.vehicles[index + 1] = rival.vehicle;
            }
            if (race.rivals()[0].progress.finished() && race.rivals()[1].progress.finished())
                break;
        }
        for (int index = 0; index < DirtBikeRace::rivalCount; ++index) {
            const auto &rival = race.rivals()[index];
            std::printf("bike_rival_seed seed=%u rider=%d finish=%.2f recoveries=%d peak=%.1f decisions=%d\n",
                seed, index, rival.progress.finishTime(), rival.recoveries, peakSpeed[index], rival.driver.decisions());
            CHECK_RACE(rival.progress.finished() && rival.recoveries <= 3);
            CHECK_RACE(rival.progress.finishTime() < 145 && peakSpeed[index] > 1800);
            CHECK_RACE(rival.driver.decisions() >= 6);
            if (rival.recoveries == 0)
                ++cleanFinishes;
        }
    }
    CHECK_RACE(cleanFinishes > 0 && crashes > 0);
    std::printf("bike_rival_independence: isolated decisions, clean finishes=%d physical crashes=%d\n",
        cleanFinishes, crashes);
    std::printf("bike_race: wheelie, ordered progress and physical rival completion passed\n");
    return 0;
}
