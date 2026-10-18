#include "DirtBikeRace.h"
#include "SpatialWorkspace.h"
#include <cmath>

SpatialVehicle DirtBikeRace::playerGrid(DirtBikeRaceCourse::Layout layout)
{
    SpatialVehicle vehicle;
    vehicle.id = "mesa-bike";
    vehicle.name = layout == DirtBikeRaceCourse::Layout::RedBud ? "RedBud 250 · Dirt bike" : "Mesa 250 · Dirt bike";
    vehicle.body = "dirt-bike";
    vehicle.paint = QColor("#ed6837");
    vehicle.position = DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start, 0, layout).position;
    vehicle.heading = DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start, 0, layout).heading;
    return vehicle;
}

void DirtBikeRace::start(std::uint32_t seed, DirtBikeRaceCourse::Layout layout)
{
    *this = {};
    m_layout = layout;
    m_phase = Phase::Countdown;
    m_player.reset(playerGrid(layout).position, layout);
    for (int index = 0; index < rivalCount; ++index) {
        auto &rival = m_rivals[index];
        rival.vehicle = playerGrid(layout);
        rival.vehicle.id = QString("mesa-rival-%1").arg(index);
        rival.vehicle.name = index == 0 ? "Riley · Blue" : "Alex · Green";
        rival.vehicle.paint = index == 0 ? QColor("#337ec4") : QColor("#70a33b");
        rival.vehicle.position = DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start - 250,
            index == 0 ? -200.f : 200.f, layout).position;
        rival.bike.reset(rival.vehicle);
        rival.driver.reset(index, seed ^ (0x9e3779b9u * (index + 1)), layout);
        rival.progress.reset(rival.vehicle.position, layout);
    }
}

void DirtBikeRace::cancel()
{
    m_phase = Phase::Idle;
}

const DirtBikeRace::Rival *DirtBikeRace::rival(const QString &id) const
{
    for (const auto &rival : m_rivals) {
        if (!rival.vehicle.id.isEmpty() && rival.vehicle.id == id)
            return &rival;
    }
    return nullptr;
}

bool DirtBikeRace::active() const
{
    if (m_phase == Phase::Countdown || m_phase == Phase::Racing)
        return true;
    if (m_phase == Phase::Finished) {
        for (const auto &rival : m_rivals) {
            if (!rival.progress.finished() || rival.bike.unsettled())
                return true;
        }
    }
    return false;
}

void DirtBikeRace::advance(float seconds, const DirtBikeDynamics::State &player, const SpatialInterior &room)
{
    if (!active() || !std::isfinite(seconds) || seconds <= 0)
        return;
    seconds = qMin(seconds, .05f);
    if (m_phase == Phase::Countdown) {
        m_countdown = qMax(0.f, m_countdown - seconds);
        if (m_countdown > 0)
            return;
        m_phase = Phase::Racing;
    }
    m_elapsed += seconds;
    m_player.advance(player.position, m_elapsed);
    for (auto &rival : m_rivals)
        advanceRival(rival, seconds, room);
    if (m_player.finished())
        m_phase = Phase::Finished;
}

void DirtBikeRace::advanceRival(Rival &rival, float seconds, const SpatialInterior &room)
{
    const auto &state = rival.bike.state();
    const float along = DirtBikeRaceCourse::progress(state.position, m_layout);
    const auto line = DirtBikeRaceCourse::sample(along, rival.driver.lane(), m_layout);
    auto input = rival.driver.controls(state, seconds, room, rival.vehicle.id);
    if (rival.progress.finished()) {
        input.throttle = 0;
        input.serviceBrake = 0;
        input.rearBrake = true; // Park without engaging the player's hold-brake reverse assist.
        input.steering = 0;
    }
    rival.bike.advance(input, seconds, room);
    rival.vehicle.position = rival.bike.state().position;
    rival.vehicle.heading = rival.bike.state().heading;
    rival.progress.advance(rival.vehicle.position, m_elapsed);
    const auto deviation = rival.vehicle.position - line.position;
    if (!rival.progress.finished() && (std::abs(rival.bike.state().speed) < 80
        || std::hypot(deviation.x(), deviation.z()) > 900 || rival.bike.state().crashed))
        rival.stalled += seconds;
    else
        rival.stalled = 0;
    if (rival.stalled > 4) {
        auto recovered = rival.vehicle;
        recovered.heading = rival.progress.recoveryHeading();
        const QVector3D right(std::cos(recovered.heading), 0, -std::sin(recovered.heading));
        const float side = rival.vehicle.id.endsWith('0') ? -1.f : 1.f;
        recovered.position = rival.progress.recoveryPosition() + right * (side * 220);
        recovered.position.setY(DirtBikeDynamics::groundHeight(room, recovered.position));
        if (SpatialVehicleModel::blocked(room, recovered))
            return; // Wait for this rider's checkpoint to clear; never spawn inside another actor.
        rival.vehicle = recovered;
        rival.bike.reset(rival.vehicle);
        rival.progress.rejoin(rival.vehicle.position);
        rival.stalled = 0;
        ++rival.recoveries;
        rival.driver.recovered();
    }
}

SpatialVehicle DirtBikeRace::recoverPlayer(const SpatialInterior &room)
{
    auto vehicle = playerGrid(m_layout);
    vehicle.position = m_player.recoveryPosition();
    vehicle.position.setY(DirtBikeDynamics::groundHeight(room, vehicle.position));
    vehicle.heading = m_player.recoveryHeading();
    m_player.rejoin(vehicle.position);
    return vehicle;
}

int DirtBikeRace::lap() const
{
    return qMin(DirtBikeRaceProgress::raceLaps, m_player.laps() + 1);
}

int DirtBikeRace::place(const QVector3D &playerPosition) const
{
    int place = 1;
    for (const auto &rival : m_rivals) {
        if (rival.progress.finished()) {
            if (!m_player.finished() || rival.progress.finishTime() < m_player.finishTime())
                ++place;
        } else if (!m_player.finished() && rival.progress.distance(rival.vehicle.position) > m_player.distance(playerPosition)) {
            ++place;
        }
    }
    return place;
}
