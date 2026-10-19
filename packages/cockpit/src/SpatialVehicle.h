#pragma once

#include <QColor>
#include "SpatialSurfaceMaterial.h"
#include <QString>
#include <QVector>
#include <QVector3D>
#include <QVector2D>

// Content identifies the vehicle; procedural geometry is deliberately approximate.
// Position is the ground centre, heading is radians, forward is local +Z.
struct SpatialVehicle {
    QString id;
    QString name;
    QString body = "sedan";
    int year = 0;
    QColor paint{"#287bb5"};
    QVector3D position;
    float heading = 0;
};

struct SpatialVehicleFace {
    QVector<QVector3D> vertices;
    QColor color;
    int part = 0; // Optional articulated model group; zero is rigid geometry.
    SpatialSurfaceMaterial material = SpatialSurfaceMaterial::Plain;
    bool singleSided = false;
    QColor albedo; // Unlit source colour for native lighting; painter keeps color.
    QVector<QVector3D> normals; // Optional smooth normals, one per vertex.
    SpatialSurfaceFinish finish = SpatialSurfaceFinish::Standard;
    QVector<QVector2D> uvs; // Optional rest-model coordinates, stable under articulation.
};

struct SpatialInterior;
namespace SpatialVehicleModel {
constexpr int maximumVehicles = 12;
constexpr int maximumFaces = 4096; // Detailed bike plus rider; cars remain below 224.
bool valid(const SpatialVehicle &vehicle);
QVector3D size(const SpatialVehicle &vehicle);
QVector3D rotate(const QVector3D &local, float heading);
QVector<SpatialVehicleFace> build(const SpatialVehicle &vehicle);
bool contains(const SpatialVehicle &vehicle, const QVector3D &point, float padding = 0);
bool blocked(const SpatialInterior &room, const SpatialVehicle &vehicle);
float rayDistance(const SpatialVehicle &vehicle, const QVector3D &origin, const QVector3D &direction);
}
