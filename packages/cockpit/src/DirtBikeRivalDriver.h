#pragma once
#include "DirtBikeDynamics.h"
#include "DirtBikeRaceCourse.h"
#include <cstdint>

// Each rider owns its decisions and random stream. The driver can only request
// normal controls; it has no authority to move a bike or change its physics.
class DirtBikeRivalDriver {
public:
    void reset(int rider, std::uint32_t seed, DirtBikeRaceCourse::Layout layout = DirtBikeRaceCourse::Layout::Mesa);
    DirtBikeDynamics::Input controls(const DirtBikeDynamics::State &state, float seconds, const SpatialInterior &room, const QString &id);
    void recovered();
    float targetSpeed() const { return m_targetSpeed; }
    float commitment() const { return m_commitment; }
    float lane() const { return m_lane; }
    int decisions() const { return m_decisions; }
private:
    DirtBikeRaceCourse::Layout m_layout = DirtBikeRaceCourse::Layout::Mesa;
    float randomUnit();
    std::uint32_t m_random = 1;
    float m_preferredLane = 0;
    float m_side = 1;
    float m_lane = 0;
    float m_commitment = 1;
    float m_braking = 850;
    float m_caution = 0;
    float m_targetSpeed = 0;
    float m_distanceSinceDecision = 0;
    bool m_approachingCorner = false;
    int m_decisions = 0;
};
