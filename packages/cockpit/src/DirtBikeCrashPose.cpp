#include "DirtBikeCrashPose.h"
#include "DirtBikeModel.h"
#include <array>
#include <cmath>
#include <limits>

namespace {
const QVector3D hip(0, 112, -20);
struct PartBounds {
    QVector3D minimum;
    QVector3D maximum;
    bool populated = false;

    void include(const QVector3D &point)
    {
        if (!populated) {
            minimum = maximum = point;
            populated = true;
            return;
        }
        for (int axis = 0; axis < 3; ++axis) {
            minimum[axis] = qMin(minimum[axis], point[axis]);
            maximum[axis] = qMax(maximum[axis], point[axis]);
        }
    }
};
}

DirtBikeCrashPose::DirtBikeCrashPose(const DirtBikeDynamics::State &bike,
    const QVector<SpatialVehicleFace> &model, const SpatialInterior &room)
{
    const float time = std::isfinite(bike.crashTime) ? bike.crashTime : 0;
    const float fall = qBound(0.f, time / .9f, 1.f);
    const float side = bike.lean < 0 ? -1.f : 1.f;
    m_rotation = QQuaternion::fromEulerAngles(20 * fall, 0, 80 * fall * side);
    m_offset = {140 * fall * side, -70 * fall + 100 * std::sin(fall * 3.14159265f), -40 * fall};

    // Scan the already bounded model once, then sample five terrain points per
    // occupied body part. No per-vertex terrain queries or dynamic allocations.
    std::array<PartBounds, DirtBikeModel::RightBoot + 1> bounds;
    const int faceCount = qMin(int(model.size()), DirtBikeModel::maximumFaces);
    for (int faceIndex = 0; faceIndex < faceCount; ++faceIndex) {
        const auto &face = model[faceIndex];
        if (!DirtBikeModel::riderPart(face.part) || face.part >= int(bounds.size()))
            continue;
        for (const auto &rest : face.vertices) {
            const auto world = bike.position + SpatialVehicleModel::rotate(point(rest), bike.heading);
            bounds[face.part].include(world);
        }
    }
    float clearance = -std::numeric_limits<float>::infinity();
    for (const auto &part : bounds) {
        if (!part.populated)
            continue;
        const auto center = (part.minimum + part.maximum) * .5f;
        float floor = DirtBikeDynamics::groundHeight(room, center);
        for (float x : {part.minimum.x(), part.maximum.x()}) {
            for (float z : {part.minimum.z(), part.maximum.z()})
                floor = qMax(floor, DirtBikeDynamics::groundHeight(room, {x, 0, z}));
        }
        clearance = qMax(clearance, floor + 2 - part.minimum.y());
    }
    if (!std::isfinite(clearance))
        return;
    // Lift out of terrain immediately. Lower smoothly only once the wreck is
    // grounded, so an airborne crash does not teleport its rider to the floor.
    const float settle = fall * fall * (3 - 2 * fall);
    if (clearance > 0)
        m_offset.setY(m_offset.y() + clearance);
    else if (bike.grounded)
        m_offset.setY(m_offset.y() + clearance * settle);
}

QVector3D DirtBikeCrashPose::point(const QVector3D &rest) const
{
    return hip + m_rotation.rotatedVector(rest - hip) + m_offset;
}
