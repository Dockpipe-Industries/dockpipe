#pragma once
#include <QtGlobal>
#include <QVector2D>
#include <cmath>

// Small transient contact-patch model in mass-normalized cm/s². Elastic demand
// builds over rolling distance; the axle's combined-slip envelope allocates
// drive, braking and lateral grip together. Coefficients are assisted defaults,
// not measurements from a commercial tyre or a calibrated off-road model.
namespace DirtBikeTyre {
// Planar vectors use x = longitudinal and y = lateral. Slip and the friction
// ellipse belong to the wheel frame; chassis force and yaw use its inverse.
class SteeringFrame {
public:
    explicit SteeringFrame(float angle) : m_cosine(std::cos(angle)), m_sine(std::sin(angle)) {}

    QVector2D toWheel(float longitudinal, float lateral) const
    {
        return {longitudinal * m_cosine + lateral * m_sine,
            lateral * m_cosine - longitudinal * m_sine};
    }

    QVector2D toChassis(float longitudinal, float lateral) const
    {
        return {longitudinal * m_cosine - lateral * m_sine,
            lateral * m_cosine + longitudinal * m_sine};
    }

private:
    float m_cosine;
    float m_sine;
};

inline float brakingDemand(float velocity, float capacity, float seconds, float contactResponse)
{
    if (!std::isfinite(velocity) || !std::isfinite(capacity) || !std::isfinite(seconds)
        || !std::isfinite(contactResponse) || capacity <= 0 || seconds <= 0 || contactResponse < 1)
        return 0;
    // A contact impulse opposes motion, including its yaw contribution. Do
    // not spend a full brake force after the wheel has already stopped.
    const float stoppingForce = std::abs(velocity) / (seconds * contactResponse);
    return -std::copysign(qMin(capacity, stoppingForce), velocity);
}

inline float lateralDemand(float slipVelocity, float speed, float grip, float seconds,
    float &relaxedSlipAngle)
{
    if (!std::isfinite(slipVelocity) || !std::isfinite(speed) || !std::isfinite(grip)
        || !std::isfinite(seconds) || seconds <= 0 || grip <= 0) {
        relaxedSlipAngle = 0;
        return 0;
    }
    constexpr float relaxationLength = 25; // cm of rolling contact to build force.
    if (!std::isfinite(relaxedSlipAngle))
        relaxedSlipAngle = 0;
    const float forwardSpeed = qMax(300.f, std::abs(speed));
    const float timeConstant = qBound(.008f, relaxationLength / forwardSpeed, .04f);
    const float targetSlip = std::atan2(slipVelocity, forwardSpeed);
    const float response = -std::expm1(-qMin(seconds, .05f) / timeConstant);
    relaxedSlipAngle += (targetSlip - relaxedSlipAngle) * response;
    // Cornering stiffness scales with supported load. A finite low-speed
    // denominator and bounded relaxation time avoid singular parking behaviour.
    constexpr float corneringStiffness = 72;
    const float stiffness = qMin(grip * corneringStiffness, forwardSpeed * 20);
    return -std::tan(qBound(-1.4f, relaxedSlipAngle, 1.4f)) * stiffness;
}
}
