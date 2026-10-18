#pragma once
#include "SpatialVehicle.h"
#include "DirtBikeTyreContact.h"
#include <QQuaternion>

namespace DirtBikeModel {
enum Part {
    Frame, FrontWheel, RearWheel, Fork, Rider, RiderArm, ForkLower, Swingarm, Helmet,
    LeftUpperArm, LeftForearm, LeftHand, RightUpperArm, RightForearm, RightHand,
    LeftThigh, LeftShin, LeftBoot, RightThigh, RightShin, RightBoot, Handlebars
};
inline bool riderPart(int part)
{
    return part == Rider || part == RiderArm || part == Helmet
        || (part >= LeftUpperArm && part <= RightBoot);
}
constexpr int maximumFaces = 4096;
inline QVector3D chassisPoint(const QVector3D &point, const QQuaternion &attitude)
{
    // The solver pitches the axle pair about its midpoint. Keeping that same
    // pivot here avoids lifting both visible wheels as the chassis tilts.
    const QVector3D axleMidpoint(0, DirtBikeTyreContact::radius, 0);
    return axleMidpoint + attitude.rotatedVector(point - axleMidpoint);
}
QVector3D steerPoint(const QVector3D &point, float angle);
QVector<SpatialVehicleFace> build(const QColor &paint, bool rider);
}
