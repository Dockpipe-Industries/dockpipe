#pragma once

#include "SpatialWebGraph.h"

// These are composition algorithms, not environment names. Appearance and
// locomotion are independent inputs; vertical routes require flight today.
enum class SpatialTopology { Branches, Loop, Spine, Layers };

struct SpatialWorldStyle {
    QColor ground = QColor("#607580");
    QColor sky = QColor("#273e52");
    QColor structure = QColor("#8199a5");
    QColor accent = QColor("#66c5ba");
    SpatialSolidShape crown = SpatialSolidShape::Box;
    bool outdoors = false;
    float roofHeight = 320;
    int repetitions = 2;
};

struct SpatialWorldOptions {
    SpatialTopology topology = SpatialTopology::Branches;
    SpatialWorldStyle style;
    QString locomotion = "walk";
    float spacing = 1;
    quint32 seed = 0;
    QVector<float> features;
};

struct SpatialWorldTransform {
    int parent = -1;
    QVector3D translation;
    float yaw = 0;
    float scale = 1;
};

struct SpatialWorldPart {
    int transform = -1;
    QVector3D center;
    QVector3D size;
    QColor color;
    SpatialSolidShape shape = SpatialSolidShape::Box;
    bool collision = true;
    QString label;
};

struct SpatialWorldAnchor {
    int node = -1;
    QVector3D position;
};

struct SpatialWorldConnection {
    int from = -1;
    int to = -1;
};

// A bounded construction proposal. Only a validated, compiled plan is published.
// Local/model-authored proposals can share this boundary without executing code.
struct SpatialWorldPlan {
    static constexpr int maximumTransforms = SpatialWebGraph::maximumNodes;
    SpatialWebGraph graph;
    SpatialWorldOptions options;
    QVector<SpatialWorldTransform> transforms;
    QVector<SpatialWorldPart> parts;
    QVector<SpatialMesh> meshes;
    QVector<SpatialVehicle> vehicles;
    QVector<SpatialRideSurface> rideSurfaces;
    QVector<SpatialWorldAnchor> anchors;
    QVector<SpatialWorldConnection> connections;
    QVector<SpatialDisplay> displays;
    QSizeF size;
    float height = 2000;
    QVector3D spawn = {0, 160, -350};
};

struct SpatialWorldResult {
    bool success = false;
    bool fallback = false;
    QString diagnostic;
    SpatialInterior interior;
};
