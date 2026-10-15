#pragma once
#include <QSize>

namespace FlightProjection {
constexpr qreal nearPlane = 8;

inline qreal focalLength(const QSize &viewport)
{
    // Bound both fields of view so tall windows retain their enclosing canopy.
    return qMax(viewport.width() * 0.75, viewport.height() * 0.65);
}
}
