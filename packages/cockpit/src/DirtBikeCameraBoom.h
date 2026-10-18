#pragma once

#include <QVector3D>
#include <cmath>

// Chase-camera clearance is independent of bike physics. The obstruction
// predicate includes the scene's camera radius; 25 cm sample spacing is smaller
// than a thin wall's 50 cm expanded footprint. Work is capped at two sweeps.
namespace DirtBikeCameraBoom {
constexpr int maximumSteps = 32;
constexpr float sampleSpacing = 25;

struct Result {
    QVector3D position;
    bool pending = false;
};

template<typename Obstructed>
Result resolve(const QVector3D &anchor, const QVector3D &desired,
    const QVector3D &previous, float seconds, bool snap, const Obstructed &obstructed)
{
    const auto finite = [](const QVector3D &point) {
        return std::isfinite(point.x()) && std::isfinite(point.y()) && std::isfinite(point.z());
    };
    if (!finite(anchor))
        return {};
    const auto sweep = [&](const QVector3D &target) {
        if (!finite(target) || obstructed(anchor))
            return anchor;
        auto offset = target - anchor;
        const float length = offset.length();
        if (!std::isfinite(length))
            return anchor;
        constexpr float maximumReach = maximumSteps * sampleSpacing;
        if (length > maximumReach)
            offset *= maximumReach / length;
        const int steps = qBound(1, int(std::ceil(qMin(length, maximumReach) / sampleSpacing)), maximumSteps);
        auto clear = anchor;
        for (int step = 1; step <= steps; ++step) {
            const auto point = anchor + offset * (float(step) / steps);
            if (obstructed(point))
                return clear;
            clear = point;
        }
        return clear;
    };
    const auto clearTarget = sweep(desired);
    if (snap || !finite(previous))
        return {clearTarget, false};
    const float elapsed = std::isfinite(seconds) ? qBound(0.f, seconds, .05f) : 0;
    const auto smoothed = previous + (clearTarget - previous) * (1 - std::exp(-9 * elapsed));
    // Pull inward immediately when the old/smoothed position is occluded.
    // Returning outward remains damped instead of snapping to the full boom.
    const auto position = sweep(smoothed);
    return {position, (position - clearTarget).lengthSquared() > 1};
}
}
