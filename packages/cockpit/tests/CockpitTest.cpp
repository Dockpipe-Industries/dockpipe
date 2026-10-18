#include "CockpitWidget.h"
#include "CockpitVoice.h"
#include "WorkspaceGraphPage.h"
#include "CockpitHudPanel.h"
#include "CockpitController.h"
#include "CockpitCommandDeck.h"
#include "FlightSolarSystem.h"
#include <QToolButton>
#include <QCheckBox>
#include "ProjectFilesWidget.h"
#include "Theme.h"
#include <QApplication>
#include <QFile>
#include <QDir>
#include <QDialog>
#include <QKeyEvent>
#include <QLabel>
#include <QLibrary>
#include <QMouseEvent>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScreen>
#include <QSettings>
#include <QStackedWidget>
#include <QTemporaryDir>
#include <QThread>
#include <QElapsedTimer>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <algorithm>

int inputControlsTest();
int dirtBikeTest();
int spatialTerrainTest();
int returnPortalTest();

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Cockpit assertion failed at %d: %s\n", __LINE__, #value); return 1; } } while(false)
static void idle(int milliseconds)
{
    QElapsedTimer clock;
    clock.start();
    while (clock.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}

int spatialSiteTest();
int furnishingTest();
int gitReviewTest();
int flightDynamicsTest();
int spatialOperationsTest();
int spatialCockpitTest();
int spatialRestartTest();
int spatialWebTest();
int spatialIntelligenceTest();
int spatialWebModelTest();
int spatialCivicTest();
int spatialVehicleTest();
int nativeHandoffTest();
int nativeHandoffFixture();

class FlightFrameRecorder final : public QObject {
public:
    QVector<double> intervals;
protected:
    bool eventFilter(QObject *, QEvent *event) override
    {
        if (event->type() == QEvent::Paint) {
            if (m_clock.isValid())
                intervals.append(m_clock.nsecsElapsed() / 1e6);
            m_clock.start();
        }
        return false;
    }
private:
    QElapsedTimer m_clock;
};

int main(int argc, char **argv)
{
    if (argc == 2 && std::strcmp(argv[1], "--workspace-graph-page") == 0)
        return WorkspaceGraphPage::run(argc, argv);
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    QCoreApplication::setOrganizationName("DockpipeCockpitTest");
    QCoreApplication::setApplicationName("CockpitFixture");
    QTemporaryDir settings;
    if (!application.arguments().contains("--spatial-readback"))
        qputenv("XDG_CONFIG_HOME", settings.path().toUtf8());
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    applyDockpipeLauncherTheme(application);
    if (application.arguments().contains("--gpu-app-fixture"))
        return nativeHandoffFixture();
    if (application.arguments().contains("--site-world"))
        return spatialSiteTest();
    if (application.arguments().contains("--furnishing"))
        return furnishingTest();
    if (application.arguments().contains("--input-controls"))
        return inputControlsTest();
    if (application.arguments().contains("--spatial-readback"))
        return spatialRestartTest();
    if (application.arguments().contains("--spatial-web"))
        return spatialWebTest();
    if (application.arguments().contains("--return-portals"))
        return returnPortalTest();
    if (application.arguments().contains("--spatial-terrain"))
        return spatialTerrainTest();
    if (application.arguments().contains("--dirt-bike"))
        return dirtBikeTest();
    if (application.arguments().contains("--spatial-vehicles"))
        return spatialVehicleTest();
    if (application.arguments().contains("--spatial-intelligence"))
        return spatialIntelligenceTest();
    if (application.arguments().contains("--spatial-civic"))
        return spatialCivicTest();
    if (application.arguments().contains("--spatial-web-model"))
        return spatialWebModelTest();
    if (application.arguments().contains("--git-review"))
        return gitReviewTest();
    if (application.arguments().contains("--flight-dynamics"))
        return flightDynamicsTest();
    if (application.arguments().contains("--operations"))
        return spatialOperationsTest();
    if (application.arguments().contains("--spatial"))
        return spatialCockpitTest();
    if (application.arguments().contains("--native-handoff"))
        return nativeHandoffTest();
    if (application.arguments().contains("--bridge-preview")) {
        QTemporaryDir root;
        QDir directory(root.path());
        for (const QString &folder : {QString("Projects"), QString("Research"), QString("Workspaces")})
            CHECK(directory.mkdir(folder));
        CockpitWidget bridge;
        bridge.setWorkspace(root.path());
        bridge.resize(1440, 1000);
        bridge.show();
        auto preferences = bridge.settings();
        preferences.orbitalMotion = false;
        bridge.applySettings(preferences);
        idle(400);
        const QString capture = qEnvironmentVariable("DOCKPIPE_COCKPIT_SCREENSHOT");
        if (!capture.isEmpty()) {
            CHECK(bridge.grab().save(capture + ".orbit.png"));
            bridge.visit("files");
            idle(300);
            CHECK(bridge.grab().save(capture + ".surface.png"));
            bridge.returnToOrbit();
            bridge.resize(720, 560);
            idle(100);
            CHECK(bridge.grab().save(capture + ".compact.png"));
            bridge.resize(1440, 1000);
            bridge.findChild<QPushButton *>("cockpitControlsToggle")->click();
            idle(100);
            CHECK(bridge.grab().save(capture + ".controls.png"));
        }
        return 0;
    }
    if (application.arguments().contains("--flight-performance")) {
        const bool hardware = !qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_SOFTWARE");
        WorkspaceFlightView flight({}, hardware);
        flight.resize(1440, 900);
        flight.show();
        if (qEnvironmentVariableIsSet("DOCKPIPE_PERF_FULLSCREEN"))
            flight.showFullScreen();
        idle(400);
        for (int count : {150, 500, 7}) {
            QVector<WorkspaceFlightBuilding> buildings;
            for (int index = 0; index < count; ++index)
                buildings.append({QString("structure-%1.cpp").arg(index), qreal(50 + index % 80),
                    index % 3 == 0, false, QString("fixture:%1").arg(index)});
            flight.setCockpitWorld(buildings, count == 7);
            idle(150);
            QVector<double> frames;
            QVector<double> draws;
            const quint64 frameStart = flight.renderedFrames();
            QMouseEvent press(QEvent::MouseButtonPress, QPointF(500, 400), QPointF(500, 400),
                Qt::RightButton, Qt::RightButton, Qt::NoModifier);
            QApplication::sendEvent(flight.viewport(), &press);
            for (int frame = 0; frame < 100; ++frame) {
                QMouseEvent look(QEvent::MouseMove, QPointF(500 + frame * 2, 400), QPointF(500 + frame * 2, 400),
                    Qt::NoButton, Qt::RightButton, Qt::NoModifier);
                QElapsedTimer timer;
                timer.start();
                QApplication::sendEvent(flight.viewport(), &look);
                QApplication::processEvents();
                flight.viewport()->repaint();
                QApplication::processEvents();
                if (frame >= 10) {
                    frames.append(timer.nsecsElapsed() / 1000000.0);
                    draws.append(flight.lastDrawNanoseconds() / 1000000.0);
                }
            }
            std::sort(frames.begin(), frames.end());
            std::sort(draws.begin(), draws.end());
            std::printf("scene=%s structures=%d viewport=%dx%d renderer=%s median_ms=%.2f p95_ms=%.2f max_ms=%.2f draw_median_ms=%.2f draw_p95_ms=%.2f paints=%llu\n",
                count == 7 ? "orbit" : "city", count, flight.viewport()->width(), flight.viewport()->height(),
                qPrintable(flight.rendererDescription()), frames[frames.size() / 2], frames[int(frames.size() * 0.95)], frames.last(),
                draws[draws.size() / 2], draws[int(draws.size() * 0.95)], static_cast<unsigned long long>(flight.renderedFrames() - frameStart));
            std::fflush(stdout);
            if (count == 150) {
                flight.setCockpitWorld(buildings, false);
                flight.setSurfaceSpawn({160, -700});
                idle(150);
                FlightFrameRecorder recorder;
                flight.viewport()->installEventFilter(&recorder);
                flight.setPilotInput({0, 0, 1}, {}, false);
                const quint64 movingStart = flight.renderedFrames();
                QElapsedTimer movement;
                movement.start();
                idle(6000);
                const qreal movingFps = (flight.renderedFrames() - movingStart) * 1000.0 / movement.elapsed();
                flight.setPilotInput({}, {}, false);
                flight.viewport()->removeEventFilter(&recorder);
                CHECK(!recorder.intervals.isEmpty());
                std::sort(recorder.intervals.begin(), recorder.intervals.end());
                const auto &intervals = recorder.intervals;
                const int missedFrames = std::count_if(intervals.begin(), intervals.end(),
                    [](double milliseconds) { return milliseconds > 25; });
                for (int attempt = 0; attempt < 150 && flight.moving(); ++attempt)
                    idle(20);
                const quint64 idleStart = flight.renderedFrames();
                idle(300);
                std::printf("movement_fps=%.1f interval_p95_ms=%.2f interval_max_ms=%.2f gaps_over_25ms=%d idle_paints=%llu\n", movingFps,
                    intervals[int(intervals.size() * 0.95)], intervals.last(), missedFrames,
                    static_cast<unsigned long long>(flight.renderedFrames() - idleStart));
                std::fflush(stdout);
            }
        }
        return 0;
    }
    QTemporaryDir root;
    QFile file(root.filePath("README.md"));
    CHECK(file.open(QIODevice::WriteOnly));
    file.write(QByteArray("# Cockpit fixture\n").repeated(250));
    file.close();
    const QJsonObject excerpt = WorkspaceGraphPage::read({{"operation", "preview"},
        {"root", root.path()}, {"path", file.fileName()}});
    CHECK(excerpt.value("truncated").toBool());
    CHECK(excerpt.value("text").toString().split('\n').size() == 128);
    CHECK(WorkspaceGraphPage::read({{"operation", "preview"}, {"root", root.path()},
        {"path", root.filePath("../outside.txt")}}).contains("error"));
    CHECK(WorkspaceGraphPage::read({{"operation", "preview"}, {"root", root.path()},
        {"path", root.path()}}).contains("error"));
    QFile binary(root.filePath("binary.dat"));
    CHECK(binary.open(QIODevice::WriteOnly));
    binary.write(QByteArray(32, '\0'));
    binary.close();
    CHECK(WorkspaceGraphPage::read({{"operation", "preview"}, {"root", root.path()},
        {"path", binary.fileName()}}).value("text").toString().contains("Binary"));
    CHECK(binary.remove());
    QStackedWidget pages;
    auto *original = new QLabel("Original containers surface");
    pages.addWidget(original);
    auto *cockpit = new CockpitWidget;
    cockpit->setWorkspace(root.path());
    cockpit->pageProvider = [original](const QString &) { return original; };
    pages.addWidget(cockpit);
    pages.resize(1440, 1000);
    pages.setCurrentWidget(cockpit);
    pages.show();
    idle(300);
    CHECK(cockpit->flight()->orbit());
    CHECK(cockpit->flight()->buildingCount() == 8);
    for (int index = 0; index < 31; ++index) {
        const QVector3D origin = FlightSolarSystem::position(index, 0);
        CHECK(std::abs((origin - FlightSolarSystem::sun()).length() - FlightSolarSystem::radius(index)) < 0.01);
        CHECK((origin - FlightSolarSystem::position(index, FlightSolarSystem::period(index))).length() < 0.01);
        CHECK(FlightSolarSystem::period(index + 1) > FlightSolarSystem::period(index));
    }
    CHECK(cockpit->flight()->orbitAnimating());
    auto motion = cockpit->settings();
    auto *talk = cockpit->findChild<QPushButton *>("cockpitPushToTalk");
    auto *voice = cockpit->findChild<CockpitVoice *>();
    CHECK(talk && voice && talk->isHidden() && !talk->isEnabled());
    motion.voiceCommands = true;
    cockpit->applySettings(motion);
    CHECK(!talk->isHidden() && talk->isEnabled());
    motion.voiceCommands = false;
    cockpit->applySettings(motion);
    CHECK(talk->isHidden() && !talk->isEnabled());
    // Recognition queued before a setting change must not navigate afterward.
    CHECK(QMetaObject::invokeMethod(voice, "commandRecognized", Qt::DirectConnection, Q_ARG(QString, QString("open files"))));
    CHECK(cockpit->flight()->orbit());
    CHECK(motion.orbitalMotion);
    motion.orbitalMotion = false;
    cockpit->applySettings(motion);
    CHECK(!cockpit->flight()->orbitAnimating());
    idle(80);
    const auto pausedFrames = cockpit->flight()->renderedFrames();
    idle(120);
    CHECK(cockpit->flight()->renderedFrames() == pausedFrames);
    motion.orbitalMotion = true;
    cockpit->applySettings(motion);
    CHECK(cockpit->flight()->orbitAnimating());
    cockpit->flight()->setEditorActive(true);
    CHECK(!cockpit->flight()->orbitAnimating());
    cockpit->flight()->setEditorActive(false);
    CHECK(cockpit->flight()->orbitAnimating());
    cockpit->hide();
    CHECK(!cockpit->flight()->orbitAnimating());
    const auto hiddenOrbitFrames = cockpit->flight()->renderedFrames();
    idle(120);
    CHECK(cockpit->flight()->renderedFrames() == hiddenOrbitFrames);
    cockpit->show();
    CHECK(cockpit->flight()->orbitAnimating());
    cockpit->findChild<QToolButton *>("cockpitRoute_files")->click();
    CHECK(cockpit->flight()->travelling() && !cockpit->flight()->orbitAnimating());
    cockpit->flight()->skipTravel();
    CHECK(cockpit->destination() == "files");
    CHECK(cockpit->findChild<QToolButton *>("cockpitRoute_files")->isChecked());
    cockpit->returnToOrbit();
    for (int attempt = 0; attempt < 50 && cockpit->flight()->loading(); ++attempt)
        idle(10);
    CHECK(!cockpit->flight()->loading());
    const QString capture = qEnvironmentVariable("DOCKPIPE_COCKPIT_SCREENSHOT");
    if (!capture.isEmpty()) {
        idle(400);
        const QPixmap frame = QGuiApplication::platformName() == "offscreen" ? pages.grab()
            : pages.screen()->grabWindow(pages.winId());
        CHECK(frame.save(capture + ".orbit.png"));
    }
    cockpit->visit("workflows");
    CHECK(!cockpit->flight()->orbit() && cockpit->flight()->buildingCount() == 1);
    cockpit->flight()->fire();
    CHECK(!cockpit->flight()->editorActive());
    CHECK(cockpit->flight()->previewPath() == "panel:workflows");
    CHECK(!cockpit->flight()->previewImage().isNull());
    CHECK(pages.widget(0) == original && !original->isVisible());
    if (!capture.isEmpty()) {
        idle(100);
        const QPixmap frame = QGuiApplication::platformName() == "offscreen" ? pages.grab()
            : pages.screen()->grabWindow(pages.winId());
        CHECK(frame.save(capture + ".native-hologram.png"));
    }
    cockpit->flight()->fire();
    CHECK(cockpit->flight()->editorActive());
    CHECK(!cockpit->findChild<CockpitCommandDeck *>()->isVisible());
    CHECK(original->isVisible() && pages.count() == 2);
    CHECK(cockpit->findChild<CockpitHudPanel *>("cockpitContentPanel")->isAncestorOf(original));
    cockpit->closePanel();
    CHECK(pages.widget(0) == original && !cockpit->flight()->editorActive());
    CHECK(cockpit->findChild<CockpitCommandDeck *>()->isVisible());
    cockpit->flight()->fire();
    pages.setCurrentIndex(0);
    idle(50);
    CHECK(pages.currentWidget() == original && original->isVisible());
    pages.setCurrentWidget(cockpit);
    int activations = 0;
    QObject::connect(cockpit, &CockpitWidget::panelActivityChanged, cockpit,
        [&activations](const QString &, bool active) { activations += active ? 1 : 0; });
    for (const QString &destination : {QString("workflows")}) {
        const int before = activations;
        cockpit->visit(destination);
        cockpit->flight()->fire();
        CHECK(cockpit->flight()->previewPath() == "panel:" + destination);
        CHECK(!cockpit->flight()->previewImage().isNull());
        CHECK(!cockpit->flight()->editorActive() && activations == before);
        CHECK(pages.widget(0) == original && !original->isVisible());
        cockpit->flight()->fire();
        CHECK(cockpit->flight()->editorActive() && activations == before + 1);
        cockpit->closePanel();
    }
    cockpit->returnToOrbit();
    cockpit->visit("settings");
    CHECK(cockpit->destination() == "orbit");
    CHECK(!cockpit->findChild<QToolButton *>("cockpitRoute_settings"));
    QTemporaryDir workflowRoot;
    QFile workflow(workflowRoot.filePath("config.yml"));
    CHECK(workflow.open(QIODevice::WriteOnly));
    workflow.write("name: Preview fixture\nsteps: []\n");
    workflow.close();
    int workflowOpens = 0;
    cockpit->workflowOpener = [&workflowOpens](const QString &) { ++workflowOpens; return true; };
    cockpit->worldProvider = [&workflow](const QString &) {
        return QVector<WorkspaceFlightBuilding>{{"Preview fixture", 95, true, false, "workflow:" + workflow.fileName()}};
    };
    cockpit->visit("workflows");
    cockpit->flight()->fire();
    CHECK(cockpit->flight()->previewPath() == "workflow:" + workflow.fileName());
    for (int attempt = 0; attempt < 100 && cockpit->flight()->previewLoading(); ++attempt)
        idle(20);
    CHECK(!cockpit->flight()->previewImage().isNull());
    CHECK(!cockpit->flight()->editorActive() && workflowOpens == 0);
    cockpit->flight()->fire();
    CHECK(cockpit->flight()->editorActive() && workflowOpens == 1);
    cockpit->closePanel();
    cockpit->worldProvider = {};
    cockpit->visit("files");
    cockpit->closePanel();
    for (int attempt = 0; attempt < 100 && cockpit->flight()->loading(); ++attempt)
        idle(20);
    CHECK(!cockpit->flight()->loading() && cockpit->flight()->buildingCount() == 1);
    cockpit->flight()->fire();
    CHECK(cockpit->flight()->previewPath() == root.filePath("README.md"));
    CHECK(!cockpit->flight()->editorActive());
    for (int attempt = 0; attempt < 100 && cockpit->flight()->previewLoading(); ++attempt)
        idle(20);
    CHECK(!cockpit->flight()->previewImage().isNull());
    if (!capture.isEmpty()) {
        idle(100);
        const QPixmap frame = QGuiApplication::platformName() == "offscreen" ? pages.grab()
            : pages.screen()->grabWindow(pages.winId());
        CHECK(frame.save(capture + ".hologram.png"));
    }
    cockpit->flight()->fire();
    idle(180);
    CHECK(cockpit->flight()->editorActive());
    cockpit->closePanel();
    auto *files = cockpit->findChild<ProjectFilesWidget *>();
    CHECK(files->openFile(root.filePath("README.md")));
    auto *editor = files->findChild<QPlainTextEdit *>("fileTextEditor");
    editor->insertPlainText("draft");
    cockpit->returnToOrbit();
    CHECK(cockpit->flight()->previewPath().isEmpty());
    CHECK(files->hasUnsavedChanges());
    CHECK(!cockpit->flight()->moving());
    // Every nested world has a one-level return, including empty folders.
    // Exercise the actual visible button, gate click, and keyboard entry points.
    CHECK(QDir(root.path()).mkpath("parent/child"));
    cockpit->visit("files");
    for (int attempt = 0; attempt < 100 && cockpit->flight()->loading(); ++attempt)
        idle(20);
    cockpit->flight()->enterDirectory(root.filePath("parent/child"));
    for (int attempt = 0; attempt < 100 && cockpit->flight()->loading(); ++attempt)
        idle(20);
    CHECK(cockpit->flight()->directory() == root.filePath("parent/child"));
    CHECK(cockpit->flight()->hasReturnPortal());
    auto *back = cockpit->findChild<QPushButton *>("cockpitBack");
    CHECK(back && back->isVisible() && back->isEnabled());
    back->click();
    for (int attempt = 0; attempt < 100 && cockpit->flight()->loading(); ++attempt)
        idle(20);
    CHECK(cockpit->flight()->directory() == root.filePath("parent"));
    const auto projectInitial = [](WorkspaceFlightView *view, const QVector3D &world) {
        const QVector3D relative = world - view->cameraPosition();
        const qreal pitch = -0.08;
        const qreal y = relative.y() * std::cos(pitch) - relative.z() * std::sin(pitch);
        const qreal z = relative.y() * std::sin(pitch) + relative.z() * std::cos(pitch);
        const qreal focal = qMax(view->viewport()->width() * 0.75, view->viewport()->height() * 0.65);
        return QPointF(view->viewport()->width() / 2.0 + relative.x() * focal / z,
            view->viewport()->height() / 2.0 - y * focal / z);
    };
    const auto click = [](WorkspaceFlightView *view, const QPointF &point) {
        QMouseEvent press(QEvent::MouseButtonPress, point, Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(view->viewport(), &press);
        QMouseEvent release(QEvent::MouseButtonRelease, point, Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
        QApplication::sendEvent(view->viewport(), &release);
    };
    idle(30);
    if (!capture.isEmpty()) {
        const QPixmap frame = QGuiApplication::platformName() == "offscreen" ? pages.grab()
            : pages.screen()->grabWindow(pages.winId());
        CHECK(frame.save(capture + ".return-hangar.png"));
    }
    click(cockpit->flight(), projectInitial(cockpit->flight(), cockpit->flight()->returnPortalPosition()));
    for (int attempt = 0; attempt < 100 && cockpit->flight()->loading(); ++attempt)
        idle(20);
    CHECK(cockpit->flight()->directory() == root.path());
    CHECK(cockpit->flight()->returnPortalPosition().y() > 500);
    idle(30);
    if (!capture.isEmpty()) {
        const QPixmap frame = QGuiApplication::platformName() == "offscreen" ? pages.grab()
            : pages.screen()->grabWindow(pages.winId());
        CHECK(frame.save(capture + ".sky-portal.png"));
    }
    // The label extends beyond the distant opening; its edge is clickable too.
    click(cockpit->flight(), projectInitial(cockpit->flight(), cockpit->flight()->returnPortalPosition()) + QPointF(130, 0));
    CHECK(cockpit->flight()->orbit() && !back->isEnabled());
    CHECK(!cockpit->flight()->hasReturnPortal());
    CHECK(files->hasUnsavedChanges());
    cockpit->visit("containers");
    QKeyEvent returnKey(QEvent::KeyPress, Qt::Key_Backspace, Qt::NoModifier);
    QApplication::sendEvent(cockpit->flight(), &returnKey);
    CHECK(cockpit->flight()->orbit());

    // Off-center clicks must select the actual building, never the crosshair.
    WorkspaceFlightView selection({{"Left", 80, false, false, "left.txt"},
        {"Right", 80, false, false, "right.txt"}}, false);
    selection.resize(1200, 800);
    selection.show();
    selection.activateWindow();
    selection.setFocus();
    idle(150);
    const QPointF left = projectInitial(&selection, {-160, 120, 0});
    click(&selection, left);
    CHECK(selection.previewPath() == "left.txt");
    CHECK(!selection.editorActive());
    click(&selection, projectInitial(&selection, {160, 120, 0}));
    CHECK(selection.previewPath() == "right.txt");
    int openedFiles = 0;
    selection.fileTargeted = [&](const QString &path) {
        if (path == "right.txt")
            ++openedFiles;
    };
    click(&selection, projectInitial(&selection, {160, 120, 0}));
    for (int attempt = 0; attempt < 100 && openedFiles == 0; ++attempt)
        idle(20);
    CHECK(openedFiles == 1);
    selection.hide();

    // Fly through the sky gate without an idle timer or
    // repeated activation. This uses ordinary pilot input, not camera teleport.
    WorkspaceFlightView portal({}, false);
    portal.resize(1200, 800);
    portal.setCockpitWorld({}, false);
    portal.show();
    portal.activateWindow();
    portal.setFocus();
    idle(150);
    int returns = 0;
    portal.orbitRequested = [&]() {
        ++returns;
        portal.setCockpitWorld({}, true);
    };
    const QVector3D route = portal.returnPortalPosition() - portal.cameraPosition();
    portal.setPilotInput(portal.craftOrientation().conjugated().rotatedVector(route.normalized()), {}, true);
    for (int attempt = 0; attempt < 250 && returns == 0; ++attempt)
        idle(20);
    CHECK(returns == 1 && portal.orbit() && !portal.moving());
    idle(100);
    CHECK(returns == 1);
    portal.hide();
    pages.activateWindow();
    cockpit->flight()->setFocus();
    idle(50);
    auto *navigation = cockpit->findChild<CockpitHudPanel *>("cockpitNavigation");
    navigation->show();
    navigation->place(QRectF(4, -2, 0.3, 0.4));
    CHECK(cockpit->flight()->rect().contains(navigation->geometry()));
    navigation->setArrangementEnabled(true);
    navigation->place(QRectF(0.1, 0.1, 0.25, 0.6));
    auto *grip = navigation->findChild<QWidget *>("cockpitResizeGrip");
    CHECK(grip);
    const QSize windowSize = pages.size();
    const QSize panelSize = navigation->size();
    const QPoint gripPosition = grip->mapToGlobal(grip->rect().center());
    QMouseEvent resizePress(QEvent::MouseButtonPress, QPointF(grip->rect().center()), QPointF(gripPosition),
        Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(grip, &resizePress);
    QMouseEvent resizeMove(QEvent::MouseMove, QPointF(grip->rect().center()), QPointF(gripPosition + QPoint(80, 40)),
        Qt::NoButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(grip, &resizeMove);
    QMouseEvent resizeRelease(QEvent::MouseButtonRelease, QPointF(grip->rect().center()), QPointF(gripPosition + QPoint(80, 40)),
        Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(grip, &resizeRelease);
    CHECK(navigation->width() > panelSize.width() && pages.size() == windowSize);
    int destructorInputs = 0;
    auto *retiredController = new CockpitController;
    QObject::connect(retiredController, &CockpitController::pilotInput, &application,
        [&](const QVector3D &, const QPointF &, bool) { ++destructorInputs; });
    delete retiredController;
    CHECK(destructorInputs == 0);
    CHECK(CockpitController::normalizedAxis(1000) == 0);
    CHECK(CockpitController::normalizedAxis(-32768) == -1);
    CHECK(CockpitController::normalizedAxis(32767) == 1);
#if defined(Q_OS_LINUX)
    if (qEnvironmentVariableIsSet("DOCKPIPE_CONTROLLER_FIXTURE")) {
        // SDL's virtual joystick has zero USB identifiers. Never select a real
        // gamepad connected to the host when exercising this fixture.
        qputenv("SDL_GAMECONTROLLER_IGNORE_DEVICES_EXCEPT", "0x0000/0x0000");
        QLibrary sdl("SDL2-2.0", 0);
        CHECK(sdl.load());
        auto init = reinterpret_cast<int (*)(quint32)>(sdl.resolve("SDL_InitSubSystem"));
        auto quit = reinterpret_cast<void (*)(quint32)>(sdl.resolve("SDL_QuitSubSystem"));
        auto attach = reinterpret_cast<int (*)(int, int, int, int)>(sdl.resolve("SDL_JoystickAttachVirtual"));
        auto detach = reinterpret_cast<int (*)(int)>(sdl.resolve("SDL_JoystickDetachVirtual"));
        auto open = reinterpret_cast<void *(*)(int)>(sdl.resolve("SDL_JoystickOpen"));
        auto close = reinterpret_cast<void (*)(void *)>(sdl.resolve("SDL_JoystickClose"));
        auto setAxis = reinterpret_cast<int (*)(void *, int, qint16)>(sdl.resolve("SDL_JoystickSetVirtualAxis"));
        auto setButton = reinterpret_cast<int (*)(void *, int, quint8)>(sdl.resolve("SDL_JoystickSetVirtualButton"));
        CHECK(init && quit && attach && detach && open && close && setAxis && setButton);
        CHECK(init(0x2000) == 0);
        const int device = attach(1, 6, 15, 0);
        CHECK(device >= 0);
        void *joystick = open(device);
        CHECK(joystick);
        CockpitController controller;
        QVector3D received;
        bool fired = false;
        bool boost = false;
        float roll = 0;
        bool pilotBindings = false;
        controller.pilotFlightActive = [&]() { return pilotBindings; };
        QObject::connect(&controller, &CockpitController::pilotInput, [&](const QVector3D &movement, const QPointF &, bool boosting) {
            received = movement;
            boost = boosting;
        });
        QObject::connect(&controller, &CockpitController::flightControls, [&](float inputRoll, bool) {
            roll = inputRoll;
        });
        QObject::connect(&controller, &CockpitController::buttonChanged, [&](int button, bool pressed) {
            if (button == 0 && pressed)
                fired = true;
        });
        CHECK(controller.enable());
        controller.setInputActive(true);
        setAxis(joystick, 1, -32768);
        setButton(joystick, 0, 1);
        idle(120);
        CHECK(received.z() > 0.9 && fired);
        CHECK(setButton(joystick, 9, 1) == 0);
        idle(120);
        CHECK(boost && roll == 0);
        pilotBindings = true;
        idle(120);
        CHECK(!boost && roll == -1);
        CHECK(setButton(joystick, 9, 0) == 0);
        CHECK(setButton(joystick, 10, 1) == 0);
        idle(120);
        CHECK(!boost && roll == 1);
        CHECK(setButton(joystick, 9, 1) == 0);
        idle(120);
        CHECK(!boost && roll == 0);
        CHECK(setButton(joystick, 7, 1) == 0);
        idle(120);
        CHECK(boost && roll == 0);
        CHECK(setButton(joystick, 9, 0) == 0);
        CHECK(setButton(joystick, 10, 0) == 0);
        CHECK(setButton(joystick, 7, 0) == 0);
        idle(120);
        CHECK(!boost && roll == 0);
        pilotBindings = false;
        CHECK(setButton(joystick, 13, 1) == 0);
        idle(120);
        CHECK(!boost && roll == -1);
        CHECK(setButton(joystick, 13, 0) == 0);
        controller.setInputActive(false);
        CHECK(received.isNull());
        controller.setInputActive(true);
        CHECK(detach(device) == 0);
        idle(120);
        CHECK(received.isNull());
        controller.disable();
        close(joystick);
        quit(0x2000);
        std::puts("Virtual controller mapped axes, Pilot bumper roll, boost isolation, mode changes, button release, focus loss and disconnect passed.");
    }
#endif
    const auto initial = cockpit->flight()->cameraPosition();
    cockpit->flight()->setPilotInput({0, 0, 1}, {0, 0}, false);
    idle(150);
    CHECK(cockpit->flight()->cameraPosition().z() > initial.z());
    cockpit->flight()->setPilotInput({}, {}, false);
    for (int attempt = 0; attempt < 150 && cockpit->flight()->moving(); ++attempt)
        idle(20);
    CHECK(!cockpit->flight()->moving());
    cockpit->hide();
    CHECK(!cockpit->flight()->moving());
    CHECK(!cockpit->flight()->orbitAnimating());
    WorkspaceFlightView journey({}, false);
    journey.resize(1000, 700);
    journey.show();
    int arrivals = 0;
    journey.destinationActivated = [&](const QString &destination, bool) {
        if (destination == "files")
            ++arrivals;
        journey.setCockpitWorld({{"Arrival", 90, true, false, "panel:files"}}, false);
    };
    journey.setCockpitWorld({{"Filesystem", 100, true, false, "files"}}, true);
    journey.setOrbitMotion(false);
    const auto clickPlanet = [&]() {
        const auto relative = FlightSolarSystem::position(0, 0) - journey.cameraPosition();
        const qreal focal = qMax(journey.viewport()->width() * 0.75, journey.viewport()->height() * 0.65);
        const qreal forward = relative.y() * std::sin(-0.70) + relative.z() * std::cos(-0.70);
        const qreal vertical = relative.y() * std::cos(-0.70) - relative.z() * std::sin(-0.70);
        const QPointF point(journey.viewport()->width() / 2.0 + relative.x() * focal / forward,
            journey.viewport()->height() / 2.0 - vertical * focal / forward);
        QMouseEvent click(QEvent::MouseButtonPress, point, point, Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(journey.viewport(), &click);
    };
    clickPlanet();
    CHECK(journey.travelling() && arrivals == 0);
    for (int attempt = 0; attempt < 180 && journey.travelling(); ++attempt)
        idle(20);
    CHECK(!journey.travelling() && !journey.orbit() && arrivals == 1);
    CHECK(std::abs(journey.cameraPosition().y() - 180) < 1);
    journey.setCockpitWorld({{"Filesystem", 100, true, false, "files"}}, true);
    clickPlanet();
    CHECK(journey.travelling());
    journey.hide();
    idle(100);
    CHECK(!journey.travelling() && arrivals == 1);
    QVector<WorkspaceFlightBuilding> solarDestinations;
    for (int index = 0; index < 7; ++index)
        solarDestinations.append({QString("Destination %1").arg(index), 100, true, false, QString("case:%1").arg(index)});
    journey.show();
    journey.setCockpitWorld(solarDestinations, true);
    journey.setSurfaceSpawn({0, -1800});
    journey.approach("case:3");
    CHECK(journey.travelling());
    for (int attempt = 0; attempt < 180 && journey.travelling(); ++attempt) {
        CHECK((journey.cameraPosition() - FlightSolarSystem::sun()).length() >= 400);
        idle(20);
    }
    CHECK(!journey.travelling());
    std::puts("Cockpit destinations, original page ownership, dirty drafts, HUD bounds and controller deadzone passed.");
    return 0;
}
