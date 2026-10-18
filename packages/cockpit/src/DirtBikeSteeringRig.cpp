#include "DirtBikeSteeringRig.h"
#include "DirtBikeModel.h"
#include <cmath>

namespace {
constexpr float degrees = 57.2957795f;
constexpr float rake = 26.f / degrees;

float finiteBound(float value, float minimum, float maximum)
{
    return std::isfinite(value) ? qBound(minimum, value, maximum) : 0;
}
}

QVector3D DirtBikeSteeringRig::stem()
{
    return {0, 98, 40};
}
QVector3D DirtBikeSteeringRig::axis()
{
    return {0, std::cos(rake), -std::sin(rake)};
}
QVector3D DirtBikeSteeringRig::frontAxle()
{
    return {0, 35, 74};
}

QVector3D DirtBikeSteeringRig::forkPoint(bool right, float height)
{
    return {right ? 13.f : -13.f, height, 74 + (35 - height) * std::tan(rake)};
}

QVector3D DirtBikeSteeringRig::grip(bool right)
{
    return {right ? 35.f : -35.f, 127, 40};
}

QQuaternion DirtBikeSteeringRig::steeringRotation(float angle)
{
    return QQuaternion::fromAxisAndAngle(axis(), finiteBound(angle, -maximumAngle, maximumAngle) * degrees);
}

QVector3D DirtBikeSteeringRig::steerPoint(const QVector3D &point, float angle)
{
    return stem() + steeringRotation(angle).rotatedVector(point - stem());
}

float DirtBikeSteeringRig::rollingYaw(float angle)
{
    const auto forward = steeringRotation(angle).rotatedVector({0, 0, 1});
    return std::atan2(forward.x(), forward.z());
}

bool DirtBikeSteeringRig::steeredPart(int part)
{
    return part == DirtBikeModel::FrontWheel || part == DirtBikeModel::Fork
        || part == DirtBikeModel::ForkLower || part == DirtBikeModel::Handlebars;
}

bool DirtBikeSteeringRig::visibleFromRider(int part)
{
    return !DirtBikeModel::riderPart(part)
        || (part >= DirtBikeModel::LeftUpperArm && part <= DirtBikeModel::RightHand);
}

DirtBikeSteeringRig::DirtBikeSteeringRig(float angle, float compression, float wheelSpin)
    : m_steering(steeringRotation(angle)),
      m_wheelSpin(QQuaternion::fromAxisAndAngle(1, 0, 0,
          std::isfinite(wheelSpin) ? std::remainder(wheelSpin, 6.283185307f) * degrees : 0)),
      m_compression(finiteBound(compression, -24, 24))
{
}

QVector3D DirtBikeSteeringRig::point(int part, const QVector3D &rest) const
{
    if (!steeredPart(part))
        return rest;
    auto local = rest;
    if (part == DirtBikeModel::FrontWheel)
        local = frontAxle() + m_wheelSpin.rotatedVector(local - frontAxle());
    // Preserve the solver's vertical axle travel. A full telescopic/contact
    // geometry revision is separate from the shared steering joint.
    if (part == DirtBikeModel::FrontWheel || part == DirtBikeModel::ForkLower)
        local.setY(local.y() + m_compression);
    return stem() + m_steering.rotatedVector(local - stem());
}

QQuaternion DirtBikeSteeringRig::rotation(int part) const
{
    if (part == DirtBikeModel::FrontWheel)
        return m_steering * m_wheelSpin;
    return steeredPart(part) ? m_steering : QQuaternion();
}
