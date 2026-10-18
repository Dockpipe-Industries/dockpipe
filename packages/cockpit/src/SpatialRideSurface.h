#pragma once
#include "SpatialMesh.h"
#include <QPointF>

// Centimetres, local +Z along the ramp. The same linear profile supplies wheel
// contacts, walking height and rendered triangles; no hidden collision ramp.
struct SpatialRideSurface {
    QVector3D center;
    float heading = 0;
    float width = 600;
    QVector<QPointF> profile; // Increasing local Z and height; 2..8 knots.
    bool sculpted = false; // Four samples per knot span, tapered dirt shoulders.
    float curveRadius = 0; // Signed radius in cm; zero is a straight ramp.
    float bank = 0; // 0..1, raises the outside of a curved riding lane.
    QColor color{"#a56f43"};
    static constexpr int maximumSurfaces = 16;
    bool valid() const;
    float heightAt(const QVector3D &point) const; // -1 outside footprint.
    SpatialMesh mesh() const;
};
