#include "DirtBikeDust.h"
#include "DirtBikeSuspension.h"
#include <algorithm>
#include <cmath>

DirtBikeDust::DirtBikeDust()
{
    m_particles.reserve(maximumParticles);
}

void DirtBikeDust::clear()
{
    m_particles.clear();
    m_clock = 0;
}

void DirtBikeDust::advance(const DirtBikeDynamics::State &bike, float seconds, const SpatialInterior &room)
{
    if (!std::isfinite(seconds) || seconds <= 0)
        return;
    // Age existing particles through a stall, but never replay missed emissions.
    for (auto &particle : m_particles)
        particle.life -= seconds;
    m_particles.erase(std::remove_if(m_particles.begin(), m_particles.end(),
        [](const Particle &particle) { return particle.life <= 0; }), m_particles.end());
    const bool contact = bike.grounded && (bike.crashed || bike.rearSupport > .03f);
    if (!contact || !std::isfinite(bike.speed) || std::abs(bike.speed) <= 250
        || !std::isfinite(bike.heading) || !std::isfinite(bike.pitch)
        || !std::isfinite(bike.position.x()) || !std::isfinite(bike.position.z())) {
        m_clock = 0;
        return;
    }
    constexpr float interval = .055f;
    m_clock = qMin(interval, m_clock + qMin(seconds, .05f));
    if (m_clock < interval)
        return;
    m_clock -= interval;
    if (m_particles.size() >= maximumParticles)
        return;
    auto origin = bike.position;
    if (!bike.crashed) {
        const auto forward = SpatialVehicleModel::rotate({0, 0, 1}, bike.heading);
        origin -= forward * DirtBikeSuspension::pitchLever(bike.pitch);
    }
    origin.setY(DirtBikeDynamics::groundHeight(room, origin) + 10);
    m_particles.append({origin, lifetime});
}
