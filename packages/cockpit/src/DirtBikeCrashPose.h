#pragma once

#include "DirtBikeDynamics.h"
#include <QQuaternion>

// Presentation-only rigid fall. Part bounds provide conservative terrain
// clearance; this is not a ragdoll or an additional physics simulation.
class DirtBikeCrashPose {
public:
    DirtBikeCrashPose(const DirtBikeDynamics::State &bike,
        const QVector<SpatialVehicleFace> &model, const SpatialInterior &room);
    QVector3D point(const QVector3D &rest) const;
    const QQuaternion &rotation() const { return m_rotation; }

private:
    QQuaternion m_rotation;
    QVector3D m_offset;
};
