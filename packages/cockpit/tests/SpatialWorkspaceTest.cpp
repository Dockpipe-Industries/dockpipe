#include "SpatialWorkspaceStore.h"
#include <QCoreApplication>
#include <QFile>
#include <QDir>
#include <QJsonDocument>
#include <QJsonObject>
#include <QTemporaryDir>
#include <cstdio>
#include <limits>

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir temporary;
    const QString path = temporary.filePath("spatial.json");
    SpatialWorkspaceStore store(path);
    SpatialWorkspace workspace;
    QString error;
    auto require = [](bool condition, const char *message) {
        if (!condition) {
            std::fprintf(stderr, "%s\n", message);
            std::exit(1);
        }
    };
    require(store.load(workspace, error), "missing config must create an empty workspace");
    SpatialPlanet planet;
    planet.id = SpatialWorkspace::newId();
    planet.name = "Development";
    SpatialBuilding building;
    building.id = SpatialWorkspace::newId();
    building.name = "Workshop";
    SpatialDisplay display;
    display.id = SpatialWorkspace::newId();
    display.application = {"fixture.desktop", "fixture", "Fixture"};
    building.interior.displays.append(display);
    planet.buildings.append(building);
    workspace.planets.append(planet);
    require(store.save(workspace, error), qPrintable(error));
    SpatialWorkspace restored;
    SpatialWorkspaceStore reader(path);
    require(reader.load(restored, error), qPrintable(error));
    require(restored.planets[0].buildings[0].interior.displays[0].application.desktopId == "fixture.desktop", "reference round trip");
    require(restored.planets[0].id == planet.id, "stable identity round trip");
    auto plot = SpatialBuilding::emptyPlot("Builder plot");
    plot.position = {640, 640};
    workspace.planets[0].buildings.append(plot);
    require(store.save(workspace, error), qPrintable(error));
    require(reader.load(restored, error), qPrintable(error));
    const auto &savedPlot = restored.planets[0].buildings.last();
    require(savedPlot.id == plot.id && savedPlot.style == "plot", "plot identity round trip");
    require(savedPlot.scene().outdoors && savedPlot.scene().solids.isEmpty(), "plot has no shell or furniture");
    require(savedPlot.interior.size == QSizeF(2400, 2400), "plot construction bounds persist");
    workspace.planets[0].name = "Renamed";
    require(store.save(workspace, error), "rename save");
    require(!reader.save(restored, error), "stale writer must not overwrite");
    workspace.planets[0].buildings[0].position.setX(std::numeric_limits<double>::infinity());
    require(!store.save(workspace, error), "reject nonfinite coordinates");
    workspace.planets[0].buildings[0].position = {};
    workspace.planets[0].buildings[0].interior.displays[0].position.setY(0);
    require(!store.save(workspace, error), "reject display outside wall");
    workspace.planets[0].buildings[0].interior.displays[0].position.setY(180);
    workspace.planets.append(workspace.planets[0]);
    require(!store.save(workspace, error), "reject duplicate IDs");
    workspace.planets.removeLast();
    QFile file(path);
    require(file.open(QIODevice::ReadOnly), "read saved bytes");
    QByteArray bytes = file.readAll();
    file.close();
    require(!bytes.contains("windowId") && !bytes.contains("pixels") && !bytes.contains("command"), "metadata only");
    auto root = QJsonDocument::fromJson(bytes).object();
    root.insert("command", "must-never-run");
    require(file.open(QIODevice::WriteOnly), "prepare malicious scene fixture");
    file.write(QJsonDocument(root).toJson());
    file.close();
    require(!reader.load(restored, error), "reject executable scene fields");
    require(restored.planets[0].name == "Development", "failed load preserves live model");
    require(!reader.save(restored, error), "failed load cannot overwrite original");
    root.remove("command");
    root.insert("version", 99);
    require(file.open(QIODevice::WriteOnly | QIODevice::Truncate), "future version fixture");
    file.write(QJsonDocument(root).toJson());
    file.close();
    require(!reader.load(restored, error), "future versions preserved");
    require(file.open(QIODevice::WriteOnly | QIODevice::Truncate), "oversize fixture");
    file.write(QByteArray(SpatialWorkspaceStore::maximumBytes + 1, ' '));
    file.close();
    require(!reader.load(restored, error), "reject oversized configuration before parsing");
    SpatialWorkspace capacity;
    for (int planetIndex = 0; planetIndex < SpatialWorkspace::maximumPlanets; ++planetIndex) {
        SpatialPlanet loaded;
        loaded.id = SpatialWorkspace::newId();
        loaded.name = "Capacity fixture";
        if (planetIndex < 4) {
            for (int buildingIndex = 0; buildingIndex < 64; ++buildingIndex) {
                SpatialBuilding structure;
                structure.id = SpatialWorkspace::newId();
                structure.name = "Building";
                structure.position = {qreal(buildingIndex % 8 * 320), qreal(buildingIndex / 8 * 320)};
                for (int displayIndex = 0; displayIndex < 8; ++displayIndex) {
                    SpatialDisplay surface = display;
                    surface.id = SpatialWorkspace::newId();
                    structure.interior.displays.append(surface);
                }
                loaded.buildings.append(structure);
            }
        }
        capacity.planets.append(loaded);
    }
    SpatialWorkspaceStore capacityStore(temporary.filePath("capacity.json"));
    SpatialWorkspace empty;
    require(capacityStore.load(empty, error), "capacity load");
    require(capacityStore.save(capacity, error), qPrintable(error));
    require(capacityStore.load(restored, error), "capacity round trip");
    require(restored.planets.size() == 24, "retain inactive configurations");
    auto extra = display;
    extra.id = SpatialWorkspace::newId();
    capacity.planets[0].buildings[0].interior.displays.append(extra);
    require(!capacityStore.save(capacity, error), "aggregate display exhaustion");
    SpatialWorkspaceStore retryStore(temporary.filePath("absent/retry.json"));
    require(retryStore.load(empty, error), "load missing optional config");
    require(!retryStore.save(empty, error), "failed save reports absent directory");
    QDir().mkpath(temporary.filePath("absent"));
    require(retryStore.save(empty, error), "failed save can retry without losing revision");
    std::puts("Spatial persistence, conflicts, capacity, retry and permission schema passed.");
}
