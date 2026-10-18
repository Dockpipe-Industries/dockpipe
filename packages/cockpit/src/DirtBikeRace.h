#pragma once
#include "DirtBikeDynamics.h"
#include "DirtBikeRaceProgress.h"
#include "DirtBikeRivalDriver.h"
#include <array>

struct SpatialInterior;

// One local three-rider event. Rivals use the same bounded solver as the player;
// only their control inputs are automatic. No unbounded agent or path search.
class DirtBikeRace {
public:
    enum class Phase { Idle, Countdown, Racing, Finished };
    static constexpr int rivalCount = 2;
    struct Rival {
        SpatialVehicle vehicle;
        DirtBikeDynamics bike;
        DirtBikeRivalDriver driver;
        DirtBikeRaceProgress progress;
        float stalled = 0;
        int recoveries = 0;
    };
    static SpatialVehicle playerGrid(DirtBikeRaceCourse::Layout layout = DirtBikeRaceCourse::Layout::Mesa);
    void start(std::uint32_t seed = 0x4d455341u, DirtBikeRaceCourse::Layout layout = DirtBikeRaceCourse::Layout::Mesa);
    void cancel();
    void advance(float seconds, const DirtBikeDynamics::State &player, const SpatialInterior &room);
    SpatialVehicle recoverPlayer(const SpatialInterior &room);
    const Rival *rival(const QString &id) const;
    const std::array<Rival, rivalCount> &rivals() const { return m_rivals; }
    Phase phase() const { return m_phase; }
    bool active() const;
    float countdown() const { return m_countdown; }
    float elapsed() const { return m_player.finished() ? m_player.finishTime() : m_elapsed; }
    int lap() const;
    int place(const QVector3D &playerPosition) const;
private:
    DirtBikeRaceCourse::Layout m_layout = DirtBikeRaceCourse::Layout::Mesa;
    void advanceRival(Rival &rival, float seconds, const SpatialInterior &room);
    std::array<Rival, rivalCount> m_rivals;
    DirtBikeRaceProgress m_player;
    Phase m_phase = Phase::Idle;
    float m_countdown = 3;
    float m_elapsed = 0;
};
