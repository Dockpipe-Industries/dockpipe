#pragma once

#include "SpatialVehicle.h"

// A section in a common orthonormal cross-section frame. Centres can curve
// along the sweep; radii must stay positive so no collapsed rings are emitted.
struct SpatialLoftSection {
    QVector3D center;
    float width = 0;
    float depth = 0;
};

struct SpatialLoftResult {
    QVector<SpatialVehicleFace> faces;
    QString error;
    bool succeeded() const { return error.isEmpty(); }
};

namespace SpatialMeshLoft {
// The caller supplies its remaining aggregate polygon budget. Admission occurs
// before output allocation. Failure returns no partial mesh. Style (including
// articulation group) is copied; positions/normals/UVs are generated afresh.
SpatialLoftResult build(const QVector<SpatialLoftSection> &sections,
    QVector3D across, QVector3D depth, int sides, int faceBudget,
    const SpatialVehicleFace &style, bool capped = true);
}
