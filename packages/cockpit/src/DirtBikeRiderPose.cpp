#include "DirtBikeRiderPose.h"
#include "DirtBikeModel.h"
#include "DirtBikeSteeringRig.h"
#include <cmath>

namespace {
float bounded(float value, float minimum, float maximum)
{
    return std::isfinite(value) ? qBound(minimum, value, maximum) : 0;
}

int boneIndex(int part)
{
    return part >= DirtBikeModel::LeftUpperArm && part <= DirtBikeModel::RightBoot
        ? part - DirtBikeModel::LeftUpperArm : -1;
}
}

QVector3D DirtBikeRiderPose::shoulder(bool right)
{
    return {right ? 17.f : -17.f, 146, -2};
}

QVector3D DirtBikeRiderPose::elbow(bool right)
{
    return {right ? 30.f : -30.f, 129, 11};
}

QVector3D DirtBikeRiderPose::grip(bool right)
{
    return DirtBikeSteeringRig::grip(right);
}

QVector3D DirtBikeRiderPose::hip(bool right)
{
    return {right ? 15.f : -15.f, 115, -30};
}

QVector3D DirtBikeRiderPose::knee(bool right)
{
    return {right ? 27.f : -27.f, 88, 7};
}

QVector3D DirtBikeRiderPose::ankle(bool right)
{
    return {right ? 24.f : -24.f, 60, -18};
}

DirtBikeRiderPose::DirtBikeRiderPose(float steering, float lean, float weight, float compression)
{
    steering = bounded(steering, -DirtBikeSteeringRig::maximumAngle, DirtBikeSteeringRig::maximumAngle);
    lean = bounded(lean, -1.5f, 1.5f);
    weight = bounded(weight, -1.f, 1.f);
    compression = bounded(compression, -8.f, 12.f);
    m_bodyOffset = {-std::sin(lean) * 12, -compression, -weight * 18};
    const auto steeringRotation = DirtBikeSteeringRig::steeringRotation(steering);
    std::array<QVector3D, 2> hands;
    for (int side = 0; side < 2; ++side)
        hands[side] = DirtBikeSteeringRig::steerPoint(grip(side), steering);

    const float upperLength = (elbow(false) - shoulder(false)).length();
    const float lowerLength = (grip(false) - elbow(false)).length();
    const float reach = upperLength + lowerLength - .5f;
    constexpr float shoulderWeight = .86f;
    const float legReach = (knee(false) - hip(false)).length()
        + (ankle(false) - knee(false)).length() - .5f;
    constexpr float hipWeight = .55f;
    // Alternating projections keep one shared torso pose inside all four limb
    // reach spheres. Sixteen bounded passes cover the admitted input range.
    for (int pass = 0; pass < 16; ++pass) {
        for (int side = 0; side < 2; ++side) {
            const auto offset = bodyPoint(shoulder(side)) - hands[side];
            const float distance = offset.length();
            if (distance > reach)
                m_bodyOffset -= offset * ((distance - reach) / (distance * shoulderWeight));
            const auto hipOffset = bodyPoint(hip(side)) - ankle(side);
            const float hipDistance = hipOffset.length();
            if (hipDistance > legReach)
                m_bodyOffset -= hipOffset * ((hipDistance - legReach) / (hipDistance * hipWeight));
        }
    }

    for (int side = 0; side < 2; ++side) {
        limb(side * 3, shoulder(side), elbow(side), grip(side),
            bodyPoint(shoulder(side)), hands[side], steeringRotation);
        limb(6 + side * 3, hip(side), knee(side), ankle(side),
            bodyPoint(hip(side)), ankle(side), {});
    }
}

void DirtBikeRiderPose::limb(int index, const QVector3D &restRoot, const QVector3D &restJoint,
    const QVector3D &restEnd, const QVector3D &posedRoot, const QVector3D &posedEnd,
    const QQuaternion &endRotation)
{
    const float upperLength = (restJoint - restRoot).length();
    const float lowerLength = (restEnd - restJoint).length();
    const auto span = posedEnd - posedRoot;
    const float distance = qMax(.001f, span.length());
    const auto direction = span / distance;
    const float along = (upperLength * upperLength - lowerLength * lowerLength
        + distance * distance) / (2 * distance);
    const float outward = std::sqrt(qMax(0.f, upperLength * upperLength - along * along));
    auto bend = restJoint - restRoot;
    bend -= direction * QVector3D::dotProduct(bend, direction);
    if (bend.lengthSquared() < .001f)
        bend = QVector3D::crossProduct(direction, {0, 1, 0});
    bend.normalize();
    const auto posedJoint = posedRoot + direction * along + bend * outward;
    m_bones[index] = {restRoot, posedRoot,
        QQuaternion::rotationTo(restJoint - restRoot, posedJoint - posedRoot)};
    m_bones[index + 1] = {restJoint, posedJoint,
        QQuaternion::rotationTo(restEnd - restJoint, posedEnd - posedJoint)};
    m_bones[index + 2] = {restEnd, posedEnd, endRotation};
}

QVector3D DirtBikeRiderPose::bodyPoint(const QVector3D &rest) const
{
    return rest + m_bodyOffset * qBound(0.f, (rest.y() - 60) / 100, 1.f);
}

QVector3D DirtBikeRiderPose::bodyNormal(const QVector3D &rest, const QVector3D &normal) const
{
    if (rest.y() <= 60 || rest.y() >= 160)
        return normal;
    // Inverse-transpose of the height-weighted torso shear. Using rest normals
    // after the shoulders move gives highlights that slide through the jersey.
    auto posed = normal;
    posed.setY(posed.y() - QVector3D::dotProduct(m_bodyOffset, normal) / (100 + m_bodyOffset.y()));
    return posed.normalized();
}

QVector3D DirtBikeRiderPose::point(int part, const QVector3D &rest) const
{
    const int index = boneIndex(part);
    if (index < 0)
        return bodyPoint(rest);
    const auto &bone = m_bones[index];
    return bone.destination + bone.rotation.rotatedVector(rest - bone.origin);
}

QQuaternion DirtBikeRiderPose::rotation(int part) const
{
    const int index = boneIndex(part);
    return index < 0 ? QQuaternion() : m_bones[index].rotation;
}
