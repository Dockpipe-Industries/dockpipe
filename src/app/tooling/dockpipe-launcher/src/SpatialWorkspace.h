#pragma once
#include <QPointF>
#include <QSizeF>
#include <QString>
#include <QVector>

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
};

struct SpatialInterior {
    QString layout = "rectangular";
    QSizeF size{1200, 1200};
    qreal height = 400;
    QPointF spawn{0, -350};
    QPointF exit{0, -580};
    QVector<SpatialDisplay> displays;
};

struct SpatialBuilding {
    QString id;
    QString name;
    QPointF position;
    QString style = "hangar";
    SpatialInterior interior;
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
