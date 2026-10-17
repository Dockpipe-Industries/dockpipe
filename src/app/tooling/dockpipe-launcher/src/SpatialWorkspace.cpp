#include "SpatialWorkspace.h"
#include <QSet>
#include <QUuid>
#include <cmath>

QString SpatialWorkspace::newId()
{
    return QUuid::createUuid().toString(QUuid::WithoutBraces);
}

QString SpatialWorkspace::validate() const
{
    QSet<QString> identities;
    auto validId = [&identities](const QString &id) {
        if (QUuid(id).isNull() || identities.contains(id))
            return false;
        identities.insert(id);
        return true;
    };
    auto text = [](const QString &value) {
        return !value.trimmed().isEmpty() && value.size() <= 180 && !value.contains(QChar::Null);
    };
    auto number = [](qreal value, qreal minimum, qreal maximum) {
        return std::isfinite(value) && value >= minimum && value <= maximum;
    };
    auto location = [&number](const QPointF &point, qreal limit) {
        return number(point.x(), -limit, limit) && number(point.y(), -limit, limit);
    };
    if (planets.size() > maximumPlanets)
        return "A universe supports at most 24 custom planets.";
    int buildings = 0;
    int displays = 0;
    for (const auto &planet : planets) {
        if (!validId(planet.id) || !text(planet.name)
            || !QStringList{"ocean", "desert", "forest"}.contains(planet.theme)
            || !location(planet.spawn, 8000) || planet.spawn.y() < -1100
            || planet.buildings.size() > maximumBuildings)
            return "Invalid planet identity, name, theme, spawn or building count.";
        buildings += planet.buildings.size();
        for (const auto &building : planet.buildings) {
            if (!validId(building.id) || !text(building.name) || !location(building.position, 8000)
                || building.position.y() < -800
                || !QStringList{"hangar", "tower", "pavilion"}.contains(building.style))
                return "Invalid building identity, name, position or style.";
            if (std::abs(building.position.x() - planet.spawn.x()) < 110
                && std::abs(building.position.y() - planet.spawn.y()) < 90)
                return "A building must leave the planet arrival point clear.";
            for (const auto &other : planet.buildings) {
                if (other.id != building.id && std::abs(other.position.x() - building.position.x()) < 200
                    && std::abs(other.position.y() - building.position.y()) < 160)
                    return "Buildings need clear space around their entrances.";
            }
            const auto &room = building.interior;
            const qreal halfWidth = room.size.width() / 2;
            const qreal halfDepth = room.size.height() / 2;
            if (room.layout != "rectangular" || !number(room.size.width(), 600, 2400)
                || !number(room.size.height(), 600, 2400) || !number(room.height, 320, 800)
                || !number(room.spawn.x(), -halfWidth + 30, halfWidth - 30)
                || !number(room.spawn.y(), -halfDepth + 30, halfDepth - 30)
                || !number(room.exit.x(), -halfWidth + 80, halfWidth - 80)
                || room.exit.y() != -halfDepth + 20 || room.displays.size() > maximumDisplays)
                return "Invalid rectangular room bounds, spawn, exit or display count.";
            displays += room.displays.size();
            for (const auto &display : room.displays) {
                const auto &app = display.application;
                const bool side = display.wall == "east" || display.wall == "west";
                const qreal wallWidth = side ? room.size.height() : room.size.width();
                if (!validId(display.id) || !text(app.name) || !text(app.windowClass)
                    || app.desktopId.size() > 180 || app.desktopId.contains('/') || app.desktopId.contains('\\')
                    || !QStringList{"north", "south", "east", "west"}.contains(display.wall)
                    || !number(display.size.width(), 80, wallWidth - 40)
                    || !number(display.size.height(), 60, room.height - 40)
                    || !number(display.position.x(), -wallWidth / 2 + display.size.width() / 2 + 20,
                               wallWidth / 2 - display.size.width() / 2 - 20)
                    || !number(display.position.y(), display.size.height() / 2 + 20,
                               room.height - display.size.height() / 2 - 20))
                    return "Invalid application reference or display outside its wall.";
                if (display.wall == "south" && display.position.y() - display.size.height() / 2 < 240
                    && std::abs(-display.position.x() - room.exit.x()) < display.size.width() / 2 + 80)
                    return "Keep the exit door clear of application displays.";
            }
        }
    }
    if (buildings > maximumTotalBuildings || displays > maximumTotalDisplays)
        return "Universe capacity exceeded (256 buildings or 2048 displays).";
    return {};
}
