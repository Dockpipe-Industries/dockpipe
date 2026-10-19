#include <memory>
#pragma once
#include <QPointF>
#include <QRectF>
#include <QSizeF>
#include <QString>
#include <QVector>
#include <QVector3D>
#include <QColor>
#include "SpatialMesh.h"
#include "SpatialVehicle.h"
#include "SpatialRideSurface.h"

// Durable references contain catalogue identity, never native window IDs or pixels.
struct SpatialApplicationReference {
    QString desktopId;
    QString windowClass;
    QString name;
};

struct SpatialDisplay {
    QString id;
    SpatialApplicationReference application;
    QString wall = "north";
    QPointF position{0, 180}; // Horizontal offset and centre height, in scene units.
    QSizeF size{300, 180};
    // Transient world placements are not part of the saved wall-display format.
    bool freestanding = false;
    QVector3D center;
    qreal yaw = 0;
    bool portal = false;
    QVector3D portalCenter;
    qreal portalApproachDistance = 0;
    bool sprite = false;
    QString prompt; // Transient interaction hint; saved app screens use the default.
};

enum class SpatialSolidShape { Box, Pyramid, Octahedron };
// Procedural room furniture, owned by a transient scene rather than a website.
struct SpatialSolid {
    QVector3D center;
    QVector3D size;
    QColor color;
    bool collision = true;
    SpatialSolidShape shape = SpatialSolidShape::Box;
    QString label;
    float yaw = 0;
};

struct SpatialConnection {
    QVector3D from;
    QVector3D to;
    QColor color;
    bool active = false;
    QString sourceId;
    QString targetId;
    QString label;
};

enum class SpatialActionKind { Seat, Transfer, Drone, Destination };
struct SpatialInteriorAction {
    QString id;
    QString title;
    QVector3D center;
    QVector3D size;
    SpatialActionKind kind = SpatialActionKind::Destination;
    QVector3D destination;
    float yaw = 0;
    QString link;
};

struct SpatialWalkSurface {
    QRectF bounds; // Horizontal x/z footprint, independent of rendering geometry.
    float height = 0;
};

// Built-in, non-interactive scenery; it never becomes an app stream or link.
struct SpatialFoliage {
    QVector3D base;
    float height = 800;
    float width = 800;
};

class SpatialTerrain;

struct SpatialInterior {
    std::shared_ptr<SpatialTerrain> terrain; // Session-owned shared contacts, cloned on scene admission.
    QString environmentKey; // Stable identity for the separate furnishing overlay.
    QString layout = "rectangular";
    QSizeF size{1200, 1200};
    qreal height = 400;
    QPointF spawn{0, -350};
    QPointF exit{0, -580};
    bool returnPortalWalkEntry = true; // Transient admission policy; cramped arrivals use explicit activation.
    QVector<SpatialDisplay> displays;
    static constexpr int maximumActions = 32;
    QVector<SpatialInteriorAction> actions;
    QVector<SpatialWalkSurface> walkSurfaces;
    QVector<SpatialRideSurface> rideSurfaces; // Bounded contact geometry for generated tracks.
    // Six faces per solid; 96 accommodates twelve exhibits plus room furniture.
    static constexpr int maximumSolids = 96;
    bool streaming = false; // Transient two-area passage; never persisted.
    QVector<SpatialSolid> solids;
    static constexpr int maximumConnections = 64;
    QVector<SpatialConnection> connections; // Transient authoritative relationships.
    // Transient presentation and movement policy; saved custom rooms retain
    // their existing indoor walking defaults.
    bool outdoors = false;
    SpatialSurfaceMaterial groundMaterial = SpatialSurfaceMaterial::Plain;
    bool naturalGround = false; // Transient procedural presentation, not saved room metadata.
    float fogDistance = 6500; // Outdoor haze scale in scene centimetres.
    QColor floorColor{"#172a34"};
    QColor skyColor{"#111d27"};
    QColor wallColor{"#233b4b"};
    QString locomotion = "walk";
    qreal spawnHeight = 160;
    float spawnYaw = 0;
    static constexpr int maximumMeshVertices = 512;
    static constexpr int maximumMeshFaces = 256;
    static constexpr int maximumMeshes = 16;
    QVector<SpatialMesh> meshes;
    static constexpr int maximumFoliage = 96;
    QVector<SpatialFoliage> foliage; // Shared alpha artwork; bounded per scene.
    QVector<SpatialVehicle> vehicles; // Transient, at most twelve enterable vehicles.
};

struct SpatialBuilding {
    QString id;
    QString name;
    QPointF position;
    QString style = "hangar";
    SpatialInterior interior;
    SpatialInterior scene() const;
    static SpatialBuilding emptyPlot(const QString &name);
};

struct SpatialPlanet {
    QString id;
    QString name;
    QString theme = "ocean";
    QPointF spawn{0, -700};
    QVector<SpatialBuilding> buildings;
};

struct SpatialWorkspace {
    static constexpr int version = 1;
    // Product capacity: a small universe, bounded per-scene geometry and 2 MiB metadata.
    static constexpr int maximumPlanets = 24;
    static constexpr int maximumBuildings = 64;
    static constexpr int maximumDisplays = 24;
    static constexpr int maximumTotalBuildings = 256;
    static constexpr int maximumTotalDisplays = 2048;
    QVector<SpatialPlanet> planets;
    static QString newId();
    QString validate() const;
};
