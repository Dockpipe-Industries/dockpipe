#pragma once
#include <QtGlobal>
#include <array>
#include <cmath>

namespace DirtBikeTyreContact {
inline constexpr float radius = 35;

// Five terrain samples define a short rolling profile. Solve the supporting
// circle against each linear segment, instead of treating the tyre as a point.
// This is a rigid longitudinal envelope, not a deformable tyre or a full mesh
// collision query. Features narrower than half the radius can be missed.
template <typename HeightAtOffset>
float supportHeight(HeightAtOffset heightAtOffset)
{
    std::array<float, 5> heights;
    for (int index = 0; index < int(heights.size()); ++index) {
        heights[index] = heightAtOffset((index - 2) * radius * .5f);
        if (!std::isfinite(heights[index]))
            return 0;
    }
    if (heights[0] == heights[2] && heights[1] == heights[2]
        && heights[3] == heights[2] && heights[4] == heights[2])
        return heights[2];
    float support = heights[2];
    for (int index = 0; index < 4; ++index) {
        const float start = (index - 2) * radius * .5f;
        const float end = start + radius * .5f;
        const float slope = (heights[index + 1] - heights[index]) / (end - start);
        const float tangent = qBound(start, radius * slope / std::hypot(1.f, slope), end);
        const float ground = heights[index] + (tangent - start) * slope;
        const float clearance = std::sqrt(qMax(0.f, radius * radius - tangent * tangent));
        support = qMax(support, ground + clearance - radius);
    }
    return support;
}
}
