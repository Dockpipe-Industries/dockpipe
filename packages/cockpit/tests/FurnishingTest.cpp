#include "FurnishingController.h"
#include "FurnishingLayout.h"
#include "ScreenProviders.h"
#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialAssetLibrary.h"
#include <QApplication>
#include <QComboBox>
#include <QElapsedTimer>
#include <QFile>
#include <QDir>
#include <QLineEdit>
#include <QListWidget>
#include <QPushButton>
#include <QKeyEvent>
#include <QMouseEvent>
#include <QScreen>
#include "DesktopAppWindow.h"
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <algorithm>
#include <limits>

#define VERIFY(value) do { if (!(value)) { std::fprintf(stderr, "Furnishing assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)
namespace {
void idle(int milliseconds)
{
    QElapsedTimer time;
    time.start();
    while (time.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}
}
int furnishingTest()
{
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("XDG_DATA_HOME", (config.path() + "/data").toUtf8());
    qputenv("XDG_DATA_DIRS", (config.path() + "/system-data").toUtf8());
    VERIFY(QDir().mkpath(config.path() + "/data/applications"));
    for (const QString &name : {QString("Alpha"), QString("Beta")}) {
        QFile application(config.path() + "/data/applications/" + name + ".desktop");
        VERIFY(application.open(QIODevice::WriteOnly));
        application.write(("[Desktop Entry]\nType=Application\nName=Fixture " + name
            + "\nStartupWMClass=Fixture" + name + "\nExec=false\n").toUtf8());
    }
    VERIFY(FurnishingCatalog::items().size() >= 190);
    QSet<QString> ids;
    for (const auto &item : FurnishingCatalog::items()) {
        VERIFY(!ids.contains(item.id));
        ids.insert(item.id);
        FurnishingObject object;
        object.id = "test";
        object.type = item.id;
        object.position = item.size / 2;
        VERIFY(FurnishingLayout::validObject(object));
        VERIFY(FurnishingCatalog::parts(object).size() >= 1 && FurnishingCatalog::parts(object).size() <= 8);
        if (!item.asset.isEmpty()) {
            const auto mesh = FurnishingCatalog::mesh(object);
            VERIFY(!mesh.faces.isEmpty() && mesh.faces.size() <= 2048);
            VERIFY(mesh.faceColors.size() == mesh.faces.size());
            VERIFY(FurnishingCatalog::faceCount(object) == mesh.faces.size());
        }
    }
    SpatialInterior room;
    room.environmentKey = "fixture:room-one";
    room.size = {2200, 2200};
    room.height = 700;
    room.exit = {0, -1000};
    SpatialDisplay legacy;
    legacy.id = "existing-provider";
    legacy.application.name = "Existing content";
    room.displays.append(legacy);
    const auto floor = FurnishingLayout::target(room, {0, 160, -300}, QVector3D(0, -1, 1).normalized());
    VERIFY(floor.hit && floor.normal.y() > 0.9f && qAbs(floor.point.y()) < 0.1);
    const auto wall = FurnishingLayout::target(room, {0, 160, 0}, {0, 0, 1});
    VERIFY(wall.hit && wall.normal.z() < -0.9f);
    const auto ceiling = FurnishingLayout::target(room, {0, 160, 0}, {0, 1, 0});
    VERIFY(ceiling.hit && ceiling.normal.y() < -0.9f);
    VERIFY(SpatialAssetLibrary::model("../outside").faces.isEmpty());
    VERIFY(SpatialAssetLibrary::model("kenney-missing").faces.isEmpty());
    auto plot = SpatialBuilding::emptyPlot("Construction fixture").scene();
    FurnishingObject door;
    door.id = "door";
    door.type = "habitat-door";
    door.position = {0, 151, 0};
    VERIFY(FurnishingLayout::placementError(door, plot, {}, floor).isEmpty());
    plot.solids = FurnishingCatalog::parts(door);
    VERIFY(!SpatialInteriorGeometry::blocked(plot, {0, 160, 0}));
    VERIFY(SpatialInteriorGeometry::blocked(plot, {85, 160, 0}));
    door.yaw = 90;
    plot.solids = FurnishingCatalog::parts(door);
    VERIFY(!SpatialInteriorGeometry::blocked(plot, {0, 160, 0}));
    VERIFY(SpatialInteriorGeometry::blocked(plot, {0, 160, 85}));
    plot.solids.clear();
    FurnishingObject tile;
    tile.id = "floor";
    tile.type = "habitat-floor";
    tile.position = {0, 5, 0};
    auto neighbor = tile;
    neighbor.id = "neighbor";
    neighbor.position.setX(200);
    VERIFY(FurnishingLayout::placementError(tile, plot, {neighbor}, floor).isEmpty());
    neighbor.position.setX(100);
    VERIFY(!FurnishingLayout::placementError(tile, plot, {neighbor}, floor).isEmpty());
    QVector<FurnishingObject> detailedModels;
    for (int index = 0; index < 8; ++index) {
        FurnishingObject model;
        model.id = QString::number(index);
        model.type = "kenney-bathtub";
        model.position = {float(5000 + index * 500), 100, 0};
        detailedModels.append(model);
    }
    VERIFY(FurnishingLayout::placementError(tile, plot, detailedModels, floor).contains("Scene budget"));
    FurnishingObject chair;
    chair.id = "chair-one";
    chair.type = "chair";
    chair.position = {0, 53.5f, 100};
    VERIFY(FurnishingLayout::placementError(chair, room, {}, floor).isEmpty());
    room.foliage.append({{250, 0, 100}, 800, 760});
    VERIFY(FurnishingLayout::placementError(chair, room, {}, floor).contains("canopy"));
    room.foliage.clear();
    VERIFY(!FurnishingLayout::placementError(chair, room, {}, wall).isEmpty());
    auto overlap = chair;
    overlap.id = "other";
    VERIFY(!FurnishingLayout::placementError(chair, room, {overlap}, floor).isEmpty());
    auto fullActions = room;
    fullActions.actions.resize(SpatialInterior::maximumActions);
    VERIFY(!FurnishingLayout::placementError(chair, fullActions, {}, floor).isEmpty());
    chair.yaw = 45;
    VERIFY(FurnishingLayout::rayObject(chair, {0, 60, -100}, {0, 0, 1}) < 2500);

    FurnishingLayout store;
    QString error;
    VERIFY(store.load("fixture:store", error));
    store.objects.append(chair);
    VERIFY(store.save(error));
    FurnishingLayout reloaded;
    VERIFY(reloaded.load("fixture:store", error));
    VERIFY(reloaded.objects.size() == 1 && reloaded.objects[0].yaw == 45);
    store.objects[0].yaw = 90;
    VERIFY(store.save(error));
    VERIFY(!reloaded.save(error) && error.contains("another instance"));
    store.objects[0].position.setX(std::numeric_limits<float>::quiet_NaN());
    VERIFY(!store.save(error));
    VERIFY(reloaded.load("fixture:store", error) && reloaded.objects[0].yaw == 90);
    VERIFY(reloaded.load("fixture:other-room", error) && reloaded.objects.isEmpty());
    VERIFY(!ScreenProviders::capabilities({"web", "https://example.invalid", {}}, {}).available);
    VERIFY(ScreenProviders::capabilities({"surface", legacy.id, {}}, {legacy.id}).interactive);
    VERIFY(!ScreenProviders::capabilities({"surface", "missing", {}}, {legacy.id}).available);
    VERIFY(!ScreenProviders::preview({"file", "/dev/zero", {}}, ScreenProviders::capabilities({"file", {}, {}}, {})).isNull());

    const bool hardware = QGuiApplication::platformName() != "offscreen";
    WorkspaceFlightView view({}, hardware);
    view.resize(1440, 960);
    if (hardware)
        view.setWindowFlag(Qt::WindowStaysOnTopHint);
    view.setInterior(room, "Furnishing workshop");
    view.show();
    idle(100);
    const auto sceneCapture = [&]() {
        if (hardware) {
            DesktopAppWindow capture({quint64(view.winId()), "Furnishing fixture", "DockpipeCockpitTest"});
            QString captureError;
            return QPixmap::fromImage(capture.capture(captureError));
        }
        return view.grab();
    };
    auto *furnishing = view.furnishing();
    VERIFY(furnishing && !furnishing->active());
    QImage source(512, 320, QImage::Format_ARGB32);
    source.fill(QColor("#a0d5d1"));
    VERIFY(view.setInteriorImage(legacy.id, source));
    QKeyEvent open(QEvent::KeyPress, Qt::Key_F2, Qt::NoModifier);
    QApplication::sendEvent(&view, &open);
    VERIFY(furnishing->active());
    auto *grid = view.findChild<QListWidget *>("furnishingGrid");
    auto *search = view.findChild<QLineEdit *>("furnishingSearch");
    auto *categories = view.findChild<QComboBox *>("furnishingCategory");
    VERIFY(grid && search && categories);
    categories->setCurrentIndex(0);
    search->setText("kitchen");
    VERIFY(grid->count() >= 10 && !grid->item(0)->icon().isNull());
    search->setText("nonexistent-model-fixture");
    VERIFY(grid->count() == 0);
    furnishing->button(12, true);
    VERIFY(!furnishing->ghost());
    search->clear();
    categories->setCurrentText("Seating");
    const QString catalogCapture = qEnvironmentVariable("DOCKPIPE_FURNISHING_CAPTURE");
    if (!catalogCapture.isEmpty()) {
        idle(100);
        VERIFY(view.grab().save(catalogCapture + "-catalog.png"));
    }
    furnishing->selectType("chair");
    furnishing->updatePreview();
    if (!furnishing->validity().isEmpty())
        std::fprintf(stderr, "Placement: %s\n", qPrintable(furnishing->validity()));
    VERIFY(furnishing->ghost() && furnishing->validity().isEmpty());
    const auto placed = *furnishing->ghost();
    const QPointF cursor = view.viewport()->rect().center();
    QMouseEvent place(QEvent::MouseButtonPress, cursor, view.viewport()->mapToGlobal(cursor.toPoint()),
        Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(view.viewport(), &place);
    VERIFY(!furnishing->ghost() && furnishing->objects().size() == 2);
    QKeyEvent move(QEvent::KeyPress, Qt::Key_M, Qt::NoModifier);
    QApplication::sendEvent(&view, &move);
    VERIFY(furnishing->ghost());
    furnishing->rotate(15);
    furnishing->cancel();
    VERIFY(!furnishing->ghost() && furnishing->objects().last().yaw == placed.yaw);
    furnishing->duplicateSelected();
    VERIFY(furnishing->ghost() && furnishing->ghost()->id != placed.id);
    furnishing->cancel();
    furnishing->selectObject(placed.id);
    QKeyEvent remove(QEvent::KeyPress, Qt::Key_Delete, Qt::NoModifier);
    QApplication::sendEvent(&view, &remove);
    VERIFY(furnishing->objects().size() == 1);

    furnishing->selectType("monitor");
    furnishing->confirm();
    VERIFY(furnishing->objects().size() == 2);
    const QString screen = furnishing->selectedId();
    VERIFY(furnishing->bindSelected({"surface", legacy.id, {}}));
    VERIFY(furnishing->sourceId("furnishing:" + screen) == legacy.id);
    QString activated;
    view.destinationActivated = [&](const QString &id, bool) { activated = id; };
    furnishing->toggle();
    VERIFY(furnishing->activateScreen("furnishing:" + screen) && activated == "display:" + legacy.id);
    furnishing->toggle();
    furnishing->selectObject(screen);
    VERIFY(furnishing->bindSelected({"text", {}, "DOCKPIPE\nYour room. Your tools.\nUniversal screen fixture"}));
    VERIFY(furnishing->sourceId("furnishing:" + screen) == "furnishing-content:" + screen);
    furnishing->selectObject("base:" + legacy.id);
    VERIFY(furnishing->bindSelected({"text", {}, "Replaced legacy content"}));
    VERIFY(furnishing->sourceId(legacy.id) == "furnishing-content:base:" + legacy.id);
    furnishing->removeSelected();
    VERIFY(furnishing->suppress(legacy.id));
    view.setInterior(room, "Regenerated workshop");
    VERIFY(furnishing->suppress(legacy.id));
    VERIFY(std::any_of(furnishing->objects().cbegin(), furnishing->objects().cend(), [&](const auto &item) { return item.id == screen; }));
    furnishing->button(4, true);
    if (!furnishing->active())
        furnishing->button(4, true);
    const int category = view.findChild<QComboBox *>("furnishingCategory")->currentIndex();
    furnishing->button(10, true);
    VERIFY(view.findChild<QComboBox *>("furnishingCategory")->currentIndex() != category);
    furnishing->button(12, true);
    VERIFY(furnishing->ghost());
    furnishing->button(1, true);
    VERIFY(!furnishing->ghost());
    furnishing->selectType("desk");
    view.setSurfaceSpawn({350, -350});
    furnishing->updatePreview();
    furnishing->confirm();
    VERIFY(!furnishing->ghost());
    furnishing->selectType("large-plant");
    view.setSurfaceSpawn({-350, -350});
    furnishing->updatePreview();
    furnishing->confirm();
    VERIFY(!furnishing->ghost());
    furnishing->selectType("chair");
    view.setSurfaceSpawn({350, -600});
    furnishing->updatePreview();
    const QString capture = qEnvironmentVariable("DOCKPIPE_FURNISHING_CAPTURE");
    idle(100);
    if (!capture.isEmpty()) {
        const auto picture = sceneCapture();
        VERIFY(picture.save(capture + "-placement.png"));
    }
    furnishing->cancel();
    furnishing->selectObject(screen);
    furnishing->configure();
    view.resize(900, 650);
    idle(100);
    if (!capture.isEmpty()) {
        const auto picture = sceneCapture();
        VERIFY(picture.save(capture + "-input.png"));
    }
    furnishing->cancel();
    furnishing->toggle();
    view.hide();
    const auto frames = view.renderedFrames();
    idle(150);
    VERIFY(!furnishing->active() && view.renderedFrames() == frames);
    view.setInterior(room, "Re-entered workshop");
    VERIFY(furnishing->objects().size() == 4);
    room.environmentKey = "fixture:room-two";
    view.setInterior(room, "Different room");
    VERIFY(furnishing->objects().size() == 1 && !furnishing->suppress(legacy.id));

    // A room with just one generated surface must still offer independent apps.
    FurnishingLayout gallery;
    VERIFY(gallery.load("fixture:source-gallery", error));
    for (const auto &id : {QString("first"), QString("second")}) {
        FurnishingObject monitor;
        monitor.id = id;
        monitor.type = "monitor";
        monitor.position = {id == "first" ? -350.0f : 350.0f, 105, 200};
        gallery.objects.append(monitor);
    }
    for (const auto &type : {QString("poster"), QString("art")}) {
        FurnishingObject art;
        art.id = type;
        art.type = type;
        art.position = {type == "poster" ? -200.0f : 200.0f, 240, 1090};
        gallery.objects.append(art);
        VERIFY(FurnishingCatalog::find(type)->screen);
    }
    VERIFY(gallery.save(error));
    room.environmentKey = gallery.environment;
    QStringList requests;
    view.applicationDisplayRequested = [&](const QString &id, bool control) {
        requests.append(id + (control ? ":control" : ":preview"));
    };
    view.setInterior(room, "Source gallery");
    view.show();
    furnishing->toggle();
    auto *provider = view.findChild<QComboBox *>("furnishingProvider");
    auto *applications = view.findChild<QComboBox *>("furnishingApplication");
    auto *filePath = view.findChild<QLineEdit *>("furnishingFile");
    VERIFY(provider && applications && filePath);
    for (int index = 0; index < 2; ++index) {
        const QString id = index == 0 ? "first" : "second";
        const QString name = index == 0 ? "Alpha" : "Beta";
        furnishing->selectObject(id);
        furnishing->configure();
        provider->setCurrentIndex(provider->findData("application"));
#if defined(Q_OS_LINUX)
        VERIFY(applications->isVisible() && applications->count() >= 2);
        const int selected = applications->findText("Installed · Fixture " + name);
        VERIFY(selected >= 0);
        applications->setCurrentIndex(selected);
        furnishing->confirm();
#else
        VERIFY(furnishing->bindSelected({"application", {}, {}, {name + ".desktop", "Fixture" + name, "Fixture " + name}}));
#endif
        VERIFY(!furnishing->configuring());
        VERIFY(furnishing->sourceId("furnishing:" + id) == "furnishing:" + id);
        VERIFY(requests.contains("furnishing:" + id + ":preview"));
    }
    const auto displays = view.interiorDisplays();
    for (const auto &display : displays) {
        if (display.id == "furnishing:first")
            VERIFY(display.application.windowClass == "FixtureAlpha");
        if (display.id == "furnishing:second")
            VERIFY(display.application.windowClass == "FixtureBeta");
    }
    VERIFY(furnishing->activateScreen("furnishing:first"));
    VERIFY(requests.last() == "furnishing:first:control");

    for (const QString &type : {QString("poster"), QString("art")}) {
        const QString path = config.path() + "/" + type + ".png";
        QImage image(120, 180, QImage::Format_RGB32);
        image.fill(type == "poster" ? QColor("#e62c79") : QColor("#34d5ac"));
        VERIFY(image.save(path));
        furnishing->selectObject(type);
        furnishing->configure();
        provider->setCurrentIndex(provider->findData("file"));
        VERIFY(view.findChild<QPushButton *>("furnishingBrowseFile")->isVisible());
        filePath->setText(path);
        furnishing->confirm();
        VERIFY(!furnishing->configuring());
        VERIFY(furnishing->sourceId("furnishing:" + type) == "furnishing-content:" + type);
        const auto preview = ScreenProviders::preview({"file", path, {}}, ScreenProviders::capabilities({"file", path, {}}, {}));
        VERIFY(preview.pixelColor(preview.width() / 2, preview.height() / 2) == image.pixelColor(0, 0));
    }
    furnishing->toggle();
    view.setSurfaceSpawn({0, 500});
    idle(100);
    const auto galleryImage = (sceneCapture()).toImage();
    if (!capture.isEmpty())
        VERIFY(galleryImage.save(capture + "-gallery.png"));
    int posterPixels = 0;
    int artPixels = 0;
    for (int y = 0; y < galleryImage.height(); ++y) {
        for (int x = 0; x < galleryImage.width(); ++x) {
            const auto color = galleryImage.pixelColor(x, y);
            posterPixels += color == QColor("#e62c79");
            artPixels += color == QColor("#34d5ac");
        }
    }
    VERIFY(posterPixels > 100 && artPixels > 100);
    VERIFY(gallery.load(room.environmentKey, error));
    VERIFY(gallery.objects[0].binding.application.windowClass == "FixtureAlpha");
    VERIFY(gallery.objects[1].binding.application.windowClass == "FixtureBeta");
    VERIFY(gallery.objects[2].binding.target.endsWith("/poster.png"));
    VERIFY(gallery.objects[3].binding.target.endsWith("/art.png"));
    view.setInterior(room, "Restored gallery");
    VERIFY(furnishing->sourceId("furnishing:poster") == "furnishing-content:poster");
    VERIFY(furnishing->sourceId("furnishing:art") == "furnishing-content:art");
    VERIFY(!ScreenProviders::capabilities({"application", {}, {}}, {}).available);
    FurnishingLayout arcadeLayout;
    VERIFY(arcadeLayout.load("fixture:arcade", error));
    FurnishingObject cabinet;
    cabinet.id = "cabinet";
    cabinet.type = "arcade-cabinet";
    cabinet.position = {0, 111, 100};
    const auto marquee = ScreenProviders::marquee(cabinet.marqueeBinding);
    VERIFY(marquee.size() == QSize(512, 138));
    cabinet.binding = {"application", {}, {}, {"Alpha.desktop", "FixtureAlpha", "Fixture Alpha"}};
    arcadeLayout.objects.append(cabinet);
    VERIFY(arcadeLayout.save(error));
    room.environmentKey = arcadeLayout.environment;
    room.displays.clear();
    view.setInterior(room, "Arcade workshop");
    view.setSurfaceSpawn({0, -200});
    VERIFY(view.interiorDisplays().size() == 2);
    VERIFY(FurnishingCatalog::screenCount(cabinet) == 2);
    auto fullRoom = room;
    fullRoom.displays.resize(SpatialWorkspace::maximumDisplays - 1);
    VERIFY(!FurnishingLayout::placementError(cabinet, fullRoom, {}, floor).isEmpty());
    furnishing->toggle();
    furnishing->selectObject(cabinet.id);
    furnishing->configure();
    auto *surface = view.findChild<QComboBox *>("furnishingCabinetSurface");
    VERIFY(surface && surface->isVisible());
    furnishing->button(10, true);
    VERIFY(surface->currentIndex() == 1);
    VERIFY(provider->findData("application") == -1);
    VERIFY(!furnishing->bindSelected(cabinet.binding));
    VERIFY(furnishing->bindSelected({"file", config.path() + "/poster.png", {}}));
    VERIFY(arcadeLayout.load(room.environmentKey, error));
    VERIFY(arcadeLayout.objects.first().binding.application.windowClass == "FixtureAlpha");
    VERIFY(arcadeLayout.objects.first().marqueeBinding.target.endsWith("/poster.png"));
    VERIFY(furnishing->sourceId("furnishing:cabinet:marquee") == "furnishing-content:cabinet:marquee");
    furnishing->duplicateSelected();
    VERIFY(furnishing->ghost() && furnishing->ghost()->binding.application.windowClass == "FixtureAlpha");
    VERIFY(furnishing->ghost()->marqueeBinding.target.endsWith("/poster.png"));
    furnishing->cancel();
    furnishing->toggle();
    const auto standing = view.cameraPosition();
    VERIFY(view.beginCabinetControl("furnishing:cabinet"));
    VERIFY(!view.cabinetControlRect().isEmpty());
    VERIFY(view.viewport()->rect().contains(view.cabinetControlRect()));
    VERIFY(view.cameraPosition() != standing);
    idle(100);
    if (!capture.isEmpty()) {
        const auto picture = sceneCapture();
        VERIFY(picture.save(capture + "-arcade.png"));
    }
    view.endCabinetControl();
    VERIFY(view.cameraPosition() == standing && view.cabinetControlRect().isEmpty());
    view.setSurfaceSpawn({900, -900});
    VERIFY(!view.beginCabinetControl("furnishing:cabinet"));
    view.setInterior(room, "Restored arcade");
    VERIFY(furnishing->sourceId("furnishing:cabinet:marquee") == "furnishing-content:cabinet:marquee");

    // Each seat family and rotated instances share action-button sit/stand.
    for (const auto &definition : FurnishingCatalog::items()) {
        if (definition.category != "Seating")
            continue;
        for (float yaw : {0.0f, 90.0f}) {
            FurnishingLayout seating;
            VERIFY(seating.load("fixture:seat:" + definition.id + QString::number(yaw), error));
            FurnishingObject seat;
            seat.id = "seat";
            seat.type = definition.id;
            seat.yaw = yaw;
            seat.position = {0, definition.size.y() / 2 + 1, 0};
            seating.objects = {seat};
            VERIFY(seating.save(error));
            room.environmentKey = seating.environment;
            view.setInterior(room, "Sittable furniture");
            const auto approach = FurnishingCatalog::rotate({0, 160, -220}, yaw);
            view.setSurfaceSpawn({approach.x(), approach.z()});
            const auto before = view.cameraPosition();
            const auto actions = FurnishingCatalog::actions(seat);
            VERIFY(actions.size() == 1);
            VERIFY(view.activateInteriorAction(actions.first().id));
            VERIFY(view.seated());
            const auto sitting = view.cameraPosition();
            view.setPilotInput({0, 0, 1}, {}, false);
            idle(50);
            view.setPilotInput({}, {}, false);
            VERIFY(view.cameraPosition() == sitting);
            view.fire();
            VERIFY(!view.seated() && view.cameraPosition() == before);
            furnishing->toggle();
            furnishing->selectObject(seat.id);
            furnishing->removeSelected();
            furnishing->toggle();
            VERIFY(!view.activateInteriorAction(actions.first().id));
        }
    }
    // A real furnished prefab room proves mesh rendering shares the existing
    // scene path, not just the catalog thumbnail path.
    FurnishingLayout baseLayout;
    VERIFY(baseLayout.load("fixture:modular-base", error));
    const auto addObject = [&](const QString &type, float x, float z, float yaw = 0, float ground = 0) {
        FurnishingObject object;
        object.id = SpatialWorkspace::newId();
        object.type = type;
        object.position = {x, FurnishingCatalog::find(type)->size.y() / 2 + ground + 1, z};
        object.yaw = yaw;
        baseLayout.objects.append(object);
    };
    for (int x = -2; x <= 2; ++x) {
        for (int z = 0; z <= 2; ++z)
            addObject("habitat-floor", x * 200, z * 200);
        addObject("habitat-window", x * 200, 500, 0, 9);
        addObject("habitat-roof", x * 200, 400, 0, 310);
    }
    for (int z = 0; z <= 2; ++z) {
        addObject("habitat-wall", -500, z * 200, 90, 9);
        addObject("habitat-wall", 500, z * 200, 90, 9);
    }
    addObject("kenney-loungeDesignSofa", -300, 300, 0, 9);
    addObject("kenney-tableCoffee", -300, 150, 0, 9);
    addObject("kenney-pottedPlant", -420, 420, 0, 9);
    addObject("kenney-kitchenFridge", 400, 410, 0, 9);
    addObject("kenney-kitchenCabinet", 270, 410, 0, 9);
    addObject("kenney-lampRoundFloor", 100, 410, 0, 9);
    VERIFY(baseLayout.save(error));
    plot.environmentKey = baseLayout.environment;
    view.resize(1440, 960);
    view.setInterior(plot, "Modular base · imported furniture");
    view.setSurfaceSpawn({0, -650});
    view.show();
    idle(200);
    VERIFY(furnishing->objects().size() == baseLayout.objects.size());
    if (!capture.isEmpty())
        VERIFY(sceneCapture().save(capture + "-base.png"));
    view.setSurfaceSpawn({0, -1050});
    furnishing->toggle();
    furnishing->selectType("sandstone-floor");
    VERIFY(furnishing->ghost() && furnishing->validity().isEmpty());
    furnishing->rotate(15);
    VERIFY(furnishing->ghost()->yaw == 90);
    const QString firstTile = furnishing->ghost()->id;
    furnishing->confirm();
    VERIFY(furnishing->ghost() && furnishing->ghost()->id != firstTile);
    furnishing->cancel();
    furnishing->selectType("habitat-roof");
    VERIFY(furnishing->ghost() && furnishing->validity().isEmpty());
    VERIFY(qRound(furnishing->ghost()->position.y()) == 317);
    furnishing->button(7, true);
    VERIFY(qRound(furnishing->ghost()->position.y()) == 617);
    QKeyEvent heightKey(QEvent::KeyPress, Qt::Key_H, Qt::NoModifier);
    QApplication::sendEvent(&view, &heightKey);
    VERIFY(qRound(furnishing->ghost()->position.y()) < 100);
    furnishing->cancel();
    furnishing->toggle();
    std::puts("Furnishing: asset grid, imported meshes, modular construction, plot rendering, arcade surfaces, sittable furniture, placement budgets, controller commands and persistence passed.");
    return 0;
}
