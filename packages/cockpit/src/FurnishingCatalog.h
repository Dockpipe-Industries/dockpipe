#pragma once
#include "SpatialWorkspace.h"

// Catalogue geometry is reusable, package-owned and independent of placement UI.
enum class FurnishingMount { Floor, Surface, Wall, Ceiling };
struct FurnishingDefinition {
    QString id;
    QString name;
    QString category;
    QString family;
    QVector3D size;
    QColor color;
    FurnishingMount mount = FurnishingMount::Floor;
    bool screen = false;
    QString asset;
};
struct ScreenBinding {
    QString provider = "text";
    QString target;
    QString text = "Choose an input in Furnishing mode";
    SpatialApplicationReference application;
};
struct FurnishingObject {
    QString id;
    QString type;
    QVector3D position;
    float yaw = 0;
    ScreenBinding binding;
    QString baseDisplay; // Adapter identity for an existing generated/legacy screen.
    QSizeF screenSize;
    bool removed = false;
    ScreenBinding marqueeBinding{"text", {}, "ARCADE"};
};
namespace FurnishingCatalog {
const QVector<FurnishingDefinition> &items();
const FurnishingDefinition *find(const QString &id);
QVector<SpatialSolid> parts(const FurnishingObject &object);
SpatialMesh mesh(const FurnishingObject &object);
int faceCount(const FurnishingObject &object);
bool modular(const FurnishingObject &object);
SpatialDisplay display(const FurnishingObject &object);
SpatialDisplay marquee(const FurnishingObject &object);
int screenCount(const FurnishingObject &object);
QVector<SpatialInteriorAction> actions(const FurnishingObject &object);
QVector3D size(const FurnishingObject &object);
QVector3D rotate(const QVector3D &point, float yaw);
}
