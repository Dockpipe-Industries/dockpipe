#include "SpatialWorkspaceStore.h"
#include "ContextStore.h"
#include <QCryptographicHash>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLockFile>
#include <QSaveFile>
#include <limits>

namespace {
QJsonArray point(const QPointF &value) { return {value.x(), value.y()}; }
QPointF readPoint(const QJsonValue &value)
{
    const auto array = value.toArray();
    const double invalid = std::numeric_limits<double>::quiet_NaN();
    if (array.size() != 2)
        return {invalid, invalid};
    return {array[0].toDouble(invalid), array[1].toDouble(invalid)};
}
bool fields(const QJsonObject &object, const QStringList &keys)
{
    if (object.size() != keys.size())
        return false;
    for (const auto &key : keys) {
        if (!object.contains(key))
            return false;
    }
    return true;
}
QByteArray encode(const SpatialWorkspace &workspace)
{
    QJsonArray planets;
    for (const auto &planet : workspace.planets) {
        QJsonArray buildings;
        for (const auto &building : planet.buildings) {
            QJsonArray displays;
            for (const auto &display : building.interior.displays) {
                const auto &app = display.application;
                displays.append(QJsonObject{{"id", display.id}, {"wall", display.wall},
                    {"position", point(display.position)}, {"size", point({display.size.width(), display.size.height()})},
                    {"application", QJsonObject{{"desktopId", app.desktopId}, {"windowClass", app.windowClass}, {"name", app.name}}}});
            }
            const auto &room = building.interior;
            buildings.append(QJsonObject{{"id", building.id}, {"name", building.name}, {"style", building.style},
                {"position", point(building.position)}, {"interior", QJsonObject{{"layout", room.layout},
                    {"size", point({room.size.width(), room.size.height()})}, {"height", room.height},
                    {"spawn", point(room.spawn)}, {"exit", point(room.exit)}, {"displays", displays}}}});
        }
        planets.append(QJsonObject{{"id", planet.id}, {"name", planet.name}, {"theme", planet.theme},
            {"spawn", point(planet.spawn)}, {"buildings", buildings}});
    }
    return QJsonDocument(QJsonObject{{"version", SpatialWorkspace::version}, {"planets", planets}}).toJson();
}
bool decode(const QByteArray &bytes, SpatialWorkspace &workspace)
{
    const auto root = QJsonDocument::fromJson(bytes).object();
    if (!fields(root, {"version", "planets"}) || root.value("version").toDouble() != SpatialWorkspace::version
        || !root.value("planets").isArray() || root.value("planets").toArray().size() > SpatialWorkspace::maximumPlanets)
        return false;
    for (const auto &value : root.value("planets").toArray()) {
        const auto object = value.toObject();
        if (!fields(object, {"id", "name", "theme", "spawn", "buildings"}) || !object.value("buildings").isArray()
            || object.value("buildings").toArray().size() > SpatialWorkspace::maximumBuildings)
            return false;
        SpatialPlanet planet;
        planet.id = object.value("id").toString();
        planet.name = object.value("name").toString();
        planet.theme = object.value("theme").toString();
        planet.spawn = readPoint(object.value("spawn"));
        for (const auto &buildingValue : object.value("buildings").toArray()) {
            const auto item = buildingValue.toObject();
            if (!fields(item, {"id", "name", "style", "position", "interior"}))
                return false;
            SpatialBuilding building;
            building.id = item.value("id").toString();
            building.name = item.value("name").toString();
            building.style = item.value("style").toString();
            building.position = readPoint(item.value("position"));
            const auto room = item.value("interior").toObject();
            if (!fields(room, {"layout", "size", "height", "spawn", "exit", "displays"})
                || !room.value("displays").isArray() || room.value("displays").toArray().size() > SpatialWorkspace::maximumDisplays)
                return false;
            auto &interior = building.interior;
            interior.layout = room.value("layout").toString();
            const auto size = readPoint(room.value("size"));
            interior.size = {size.x(), size.y()};
            interior.height = room.value("height").toDouble(-1);
            interior.spawn = readPoint(room.value("spawn"));
            interior.exit = readPoint(room.value("exit"));
            for (const auto &displayValue : room.value("displays").toArray()) {
                const auto surface = displayValue.toObject();
                const auto app = surface.value("application").toObject();
                if (!fields(surface, {"id", "wall", "position", "size", "application"})
                    || !fields(app, {"desktopId", "windowClass", "name"}) || !app.value("desktopId").isString())
                    return false;
                SpatialDisplay display;
                display.id = surface.value("id").toString();
                display.wall = surface.value("wall").toString();
                display.position = readPoint(surface.value("position"));
                const auto dimensions = readPoint(surface.value("size"));
                display.size = {dimensions.x(), dimensions.y()};
                display.application = {app.value("desktopId").toString(), app.value("windowClass").toString(), app.value("name").toString()};
                interior.displays.append(display);
            }
            planet.buildings.append(building);
        }
        workspace.planets.append(planet);
    }
    return workspace.validate().isEmpty();
}
bool read(const QString &path, QByteArray &bytes, QString &error)
{
    QFile file(path);
    if (!file.exists())
        return true;
    if (!file.open(QIODevice::ReadOnly)) {
        error = file.errorString();
        return false;
    }
    if (file.size() > SpatialWorkspaceStore::maximumBytes) {
        error = "Spatial workspace exceeds the 2 MiB metadata limit.";
        return false;
    }
    bytes = file.read(SpatialWorkspaceStore::maximumBytes + 1);
    if (file.error() != QFile::NoError || bytes.size() > SpatialWorkspaceStore::maximumBytes || bytes.isEmpty()) {
        error = "Could not read complete spatial workspace metadata.";
        return false;
    }
    return true;
}
QByteArray revision(const QByteArray &bytes)
{
    return QCryptographicHash::hash(bytes, QCryptographicHash::Sha256);
}
}

SpatialWorkspaceStore::SpatialWorkspaceStore(const QString &path)
    : m_path(path.isEmpty() ? ContextStore::configDir() + "/spatial-workspaces.json" : path)
{
}

bool SpatialWorkspaceStore::load(SpatialWorkspace &workspace, QString &error)
{
    error.clear();
    m_loaded = false;
    QByteArray bytes;
    if (!read(m_path, bytes, error))
        return false;
    SpatialWorkspace candidate;
    if (!bytes.isEmpty() && !decode(bytes, candidate)) {
        error = "Invalid or unsupported spatial workspace configuration. The file has been preserved.";
        return false;
    }
    workspace = candidate;
    m_revision = revision(bytes);
    m_loaded = true;
    return true;
}

bool SpatialWorkspaceStore::save(const SpatialWorkspace &workspace, QString &error)
{
    error = workspace.validate();
    if (!error.isEmpty())
        return false;
    if (!m_loaded) {
        error = "Load a valid workspace before saving.";
        return false;
    }
    const QByteArray bytes = encode(workspace);
    if (bytes.size() > maximumBytes) {
        error = "Spatial workspace exceeds the 2 MiB metadata limit.";
        return false;
    }
    QLockFile lock(m_path + ".lock");
    if (!lock.tryLock(0)) {
        error = "Another Dockpipe instance is saving this workspace. Try again.";
        return false;
    }
    QByteArray existing;
    if (!read(m_path, existing, error))
        return false;
    if (revision(existing) != m_revision) {
        error = "Workspace changed in another instance. Reload before editing.";
        return false;
    }
    QSaveFile file(m_path);
    if (!file.open(QIODevice::WriteOnly) || file.write(bytes) != bytes.size() || !file.commit()) {
        error = file.errorString();
        return false;
    }
    m_revision = revision(bytes);
    return true;
}
