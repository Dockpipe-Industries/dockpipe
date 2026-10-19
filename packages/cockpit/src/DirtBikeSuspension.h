#pragma once
#include <QtGlobal>
#include <cmath>

// Mass-normalized suspension forces (cm/s²). Both axles share a bounded
// progressive spring with distinct compression and rebound damping.
namespace DirtBikeSuspension {
inline constexpr float gravity = 981;
inline constexpr float halfWheelbase = 74;

inline float support(float compression, float relativeSpeed)
{
    if (!std::isfinite(compression) || !std::isfinite(relativeSpeed) || compression < -24)
        return 0;
    const float stroke = qBound(0.f, compression / 24, 1.f);
    const float stiffness = 110 * (1 + .55f * stroke * stroke);
    const float damper = relativeSpeed < 0 ? 12.f : 19.f;
    return qBound(0.f, (gravity + stiffness * compression - damper * relativeSpeed) * .5f, 7000.f);
}

inline float pitchLever(float pitch)
{
    // The derivative of axle height sin(pitch) * halfWheelbase. Use this
    // same lever for velocity and torque so damping cannot inject energy
    // when a rotating bike passes through vertical or inverted attitudes.
    return std::isfinite(pitch) ? std::cos(pitch) * halfWheelbase : 0;
}
}
