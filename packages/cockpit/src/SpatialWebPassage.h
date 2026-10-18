#pragma once

#include "SpatialWorkspace.h"
#include <optional>

// One bounded connection between two resident areas. Coordinates are rebased on
// arrival, so a long browsing session cannot accumulate an ever-growing world.
class SpatialWebPassage {
public:
    static constexpr int segmentLength = 400;
    static constexpr int maximumSegments = 4;
    static constexpr int maximumLength = segmentLength * maximumSegments;
    static constexpr int solidBudget = 224;
    bool begin(const SpatialInterior &source, const SpatialDisplay &entrance);
    bool advance(const QVector3D &camera);
    bool stage(const SpatialInterior &destination);
    SpatialInterior room() const;
    float distance(const QVector3D &camera) const;
    bool arrived(const QVector3D &camera) const;
    QVector3D destinationPosition(const QVector3D &camera) const;
    float yaw() const { return m_yaw; }
    bool ready() const { return m_destination.has_value(); }
    int segments() const { return m_segments; }
    void fail() { m_failed = true; }
    bool failed() const { return m_failed; }
private:
    QVector3D point(const QVector3D &local) const;
    SpatialInterior m_source;
    std::optional<SpatialInterior> m_destination;
    QVector3D m_origin;
    QVector3D m_destinationOffset;
    float m_yaw = 0;
    int m_segments = 2;
    bool m_failed = false;
};
