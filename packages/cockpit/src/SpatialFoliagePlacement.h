#pragma once
#include "SpatialWorkspace.h"

// Flat-ground scenery admission. Reserve the entire camera-facing crown sweep,
// not just the trunk. Built once per placement batch, never during rendering.
class SpatialFoliagePlacement {
public:
    explicit SpatialFoliagePlacement(const SpatialInterior &room);
    QString error(const SpatialFoliage &plant) const;
    bool admit(const SpatialFoliage &plant);
    static SpatialSolid envelope(const SpatialFoliage &plant);
    static SpatialSolid trunk(const SpatialFoliage &plant);

private:
    QRectF m_bounds;
    qreal m_height = 0;
    QVector<QRectF> m_obstacles;
    int m_admitted = 0;
};
