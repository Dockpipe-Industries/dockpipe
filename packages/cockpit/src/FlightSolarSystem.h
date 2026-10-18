#pragma once
#include <QVector3D>
#include <cmath>

// A deliberately compressed orbital scale for navigable workspace destinations.
// The finite world page owns the bodies; no filesystem or background simulation.
namespace FlightSolarSystem {
inline QVector3D sun() { return {0, 450, 5000}; }
inline double radius(int index) { return 1050.0 + index * 390.0; }
inline double period(int index)
{
    // Kepler's radius^(3/2) relationship, scaled to an observable UI timescale.
    return 65.0 * std::pow(radius(index) / radius(0), 1.5);
}
inline QVector3D position(int index, double seconds)
{
    const double angle = index * 2.399963229728653 + 0.45 + seconds * 6.283185307179586 / period(index);
    return sun() + QVector3D(std::cos(angle) * radius(index), 0, std::sin(angle) * radius(index));
}
}
