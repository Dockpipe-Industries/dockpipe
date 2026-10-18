#include "DirtBikeRivalDriver.h"
#include "DirtBikeRaceCourse.h"
#include "SpatialWorkspace.h"
#include "SpatialTerrain.h"
#include <cmath>

void DirtBikeRivalDriver::reset(int rider, std::uint32_t seed, DirtBikeRaceCourse::Layout layout)
{
    *this = {};
    m_layout = layout;
    m_random = seed ? seed : 1;
    m_preferredLane = rider == 0 ? -130.f : 130.f;
    m_lane = m_preferredLane;
    m_side = rider == 0 ? -1.f : 1.f;
}

float DirtBikeRivalDriver::randomUnit()
{
    // Local xorshift32: another actor's ticks, recovery or decisions cannot
    // consume this actor's stream. Randomness changes intent, never the solver.
    m_random ^= m_random << 13;
    m_random ^= m_random >> 17;
    m_random ^= m_random << 5;
    return float(m_random >> 8) / 16777216.f;
}

void DirtBikeRivalDriver::recovered()
{
    m_caution = .12f;
    m_approachingCorner = false;
}

DirtBikeDynamics::Input DirtBikeRivalDriver::controls(const DirtBikeDynamics::State &state, float seconds, const SpatialInterior &room, const QString &id)
{
    const float along = DirtBikeRaceCourse::progress(state.position, m_layout);
    const auto line = DirtBikeRaceCourse::sample(along, m_lane, m_layout);
    // Twelve 250 cm probes cover the braking distance from straight-line pace
    // to a berm. This finite course needs no graph search or shared planner.
    constexpr int probes = 12;
    constexpr float spacing = 250;
    const bool approaching = DirtBikeRaceCourse::sample(along + probes * spacing, 0, m_layout).curvature != 0
        || line.curvature != 0;
    m_distanceSinceDecision += std::abs(state.speed) * seconds;
    const bool nextSection = m_layout == DirtBikeRaceCourse::Layout::RedBud && m_distanceSinceDecision > 5000;
    if (approaching && (!m_approachingCorner || nextSection)) {
        m_distanceSinceDecision = 0;
        const float confidence = randomUnit();
        m_commitment = .94f + confidence * .12f - m_caution;
        m_braking = 700 + randomUnit() * 250;
        m_preferredLane = m_side * (100 + randomUnit() * 120);
        ++m_decisions;
    }
    m_approachingCorner = approaching;
    m_caution = qMax(0.f, m_caution - seconds * .002f);
    float desiredLane = m_preferredLane;
    const QVector3D forward(std::sin(state.heading), 0, std::cos(state.heading));
    const QVector3D right(std::cos(state.heading), 0, -std::sin(state.heading));
    for (const auto &other : room.vehicles) {
        if (other.id == id)
            continue;
        const auto separation = other.position - state.position;
        const float ahead = QVector3D::dotProduct(separation, forward);
        const float lateral = QVector3D::dotProduct(separation, right);
        if (ahead > 0 && ahead < qMax(1000.f, state.speed * .8f) && std::abs(lateral) < 250)
            desiredLane = m_side * 250;
    }
    m_lane += (desiredLane - m_lane) * qMin(1.f, seconds * 2);

    m_targetSpeed = 2600;
    for (int probe = 0; probe <= probes; ++probe) {
        const auto ahead = DirtBikeRaceCourse::sample(along + probe * spacing, m_lane, m_layout);
        if (std::abs(ahead.curvature) < .00001f)
            continue;
        // The same speed-dependent steering envelope and raked front wheel as
        // the player constrain corner pace. Confidence can overestimate it.
        const float radius = 1 / std::abs(ahead.curvature) - m_lane * std::copysign(1.f, ahead.curvature);
        const float cornerSpeed = std::sqrt(.9f * 981 * qMax(400.f, radius) * std::cos(.4537856f)) * m_commitment;
        const float reachable = std::sqrt(cornerSpeed * cornerSpeed + 2 * m_braking * probe * spacing);
        m_targetSpeed = qMin(m_targetSpeed, reachable);
    }
    const float lookAhead = qBound(300.f, std::abs(state.speed) * .4f, 650.f);
    const auto aim = DirtBikeRaceCourse::sample(along + lookAhead, m_lane, m_layout);
    const auto offset = aim.position - state.position;
    const float error = std::remainder(std::atan2(offset.x(), offset.z()) - state.heading, 6.283185307f);
    const float limit = qMin(.48f, std::atan2(.9f * 981 * 148, qMax(1.f, state.speed * state.speed)));
    DirtBikeDynamics::Input input;
    input.steering = qBound(-1.f, (error * .85f + std::atan(148 * aim.curvature) / std::cos(.4537856f)) / limit, 1.f);
    if (room.terrain) {
        const auto contact = room.terrain->sample(state.position);
        m_targetSpeed *= std::sqrt(contact.grip);
    }
    input.throttle = qBound(0.f, (m_targetSpeed - state.speed) / 180, 1.f);
    input.serviceBrake = qBound(0.f, (state.speed - m_targetSpeed) / 300, 1.f);
    // Keep the rider centred under power. In flight they can correct pitch
    // using the same weight controls, rather than changing chassis rotation.
    if (!state.grounded)
        input.pitch = qBound(-.5f, -state.pitch * .7f - state.pitchRate * .3f, .5f);
    return input;
}
