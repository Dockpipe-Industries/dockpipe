#include "CockpitWidget.h"
#include "CockpitController.h"
#include "SpatialWorkspaceEditor.h"
#include <QApplication>
#include <QDialog>
#include <QLineEdit>
#include <QPushButton>
#include <QTemporaryDir>
#include <QTimer>
#include <QElapsedTimer>
#include <QThread>
#include <QKeyEvent>
#include <QComboBox>
#include <QCheckBox>
#include <QDir>
#include <QFile>
#include <QProcess>
#include <QTreeWidget>
#include <QDoubleSpinBox>
#include <QScreen>
#include <QProcessEnvironment>
#include <algorithm>
#include <ctime>
#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xlib.h>
#include <X11/keysym.h>
constexpr int nativeKeyPress = KeyPress;
#undef KeyPress
#undef KeyRelease
#endif
#include <cstdio>

#define VERIFY(value) do { if (!(value)) { std::fprintf(stderr, "Spatial assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static void settle(int milliseconds)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}

int spatialRestartTest()
{
    CockpitWidget restored;
    VERIFY(restored.spatialWorkspace().planets.size() == 1);
    const auto &building = restored.spatialWorkspace().planets[0].buildings[0];
    VERIFY(building.interior.displays.size() == 2);
    VERIFY(building.interior.displays[0].size.width() == 400);
    restored.visit("building:" + building.id);
    VERIFY(restored.flight()->interiorStreamCount() == 0);
    VERIFY(!restored.flight()->appControlling());
    return 0;
}

int spatialCockpitTest()
{
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("XDG_DATA_HOME", config.filePath("data").toUtf8());
    qputenv("XDG_DATA_DIRS", config.filePath("empty").toUtf8());
    QProcess fixture;
    DesktopAppChoice window;
    const QString fixturePath = qEnvironmentVariable("DOCKPIPE_SPATIAL_FIXTURE");
    if (!fixturePath.isEmpty()) {
        fixture.start(fixturePath, {"--fixture"});
        VERIFY(fixture.waitForStarted());
        QByteArray output;
        for (int attempt = 0; attempt < 100 && !output.contains('\n'); ++attempt) {
            settle(20);
            output += fixture.readAllStandardOutput();
        }
        const quint64 id = output.split('\n').first().mid(7).toULongLong();
        VERIFY(id);
        QString error;
        for (int attempt = 0; attempt < 100 && !window.id; ++attempt) {
            for (const auto &choice : DesktopAppWindow::availableWindows(error)) {
                if (choice.id == id)
                    window = choice;
            }
            settle(20);
        }
        VERIFY(window.id && !window.applicationClass.isEmpty());
        int matches = 0;
        for (const auto &choice : DesktopAppWindow::availableWindows(error))
            matches += choice.applicationClass == window.applicationClass;
        VERIFY(matches == 1); // Never select or move an unrelated desktop window.
    }
    QDir().mkpath(config.filePath("data/applications"));
    QFile desktop(config.filePath("data/applications/spatial-fixture.desktop"));
    VERIFY(desktop.open(QIODevice::WriteOnly));
    const QString windowClass = window.id ? window.applicationClass : "SpatialFixture";
    desktop.write(("[Desktop Entry]\nType=Application\nName=Spatial Test App\nExec=/bin/false\nStartupWMClass=" + windowClass + "\n").toUtf8());
    desktop.close();
    CockpitWidget cockpit;
    cockpit.appWindowProvider = [window](QString &error) {
        error.clear();
        if (window.id) {
            DesktopAppWindow selected(window);
            if (selected.alive())
                return QVector<DesktopAppChoice>{window};
        }
        return QVector<DesktopAppChoice>{};
    };
    cockpit.resize(1280, 900);
    cockpit.show();
    cockpit.editWorkspaces();
    auto *editor = cockpit.findChild<SpatialWorkspaceEditor *>();
    VERIFY(editor);
    QTimer::singleShot(0, []() {
        auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
        if (dialog) {
            dialog->findChild<QLineEdit *>("spatialName")->setText("Development");
            dialog->accept();
        }
    });
    editor->findChild<QPushButton *>("spatialAddPlanet")->click();
    QTimer::singleShot(0, []() {
        auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
        if (dialog) {
            dialog->findChild<QLineEdit *>("spatialName")->setText("Workshop");
            dialog->accept();
        }
    });
    editor->findChild<QPushButton *>("spatialAddBuilding")->click();
    QTimer::singleShot(0, []() {
        auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
        if (!dialog)
            return;
        auto *apps = dialog->findChild<QComboBox *>("spatialApplication");
        const int index = apps->findText("Installed · Spatial Test App");
        if (index >= 0) {
            apps->setCurrentIndex(index);
            dialog->accept();
        } else {
            dialog->reject();
        }
    });
    editor->findChild<QPushButton *>("spatialAddDisplay")->click();
    QTimer::singleShot(0, []() {
        auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
        if (!dialog)
            return;
        dialog->findChild<QDoubleSpinBox *>("spatialWidth")->setValue(400);
        dialog->findChild<QDoubleSpinBox *>("spatialOffset")->setValue(-160);
        dialog->accept();
    });
    editor->findChild<QPushButton *>("spatialEdit")->click();
    QTimer::singleShot(0, []() {
        auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
        if (!dialog)
            return;
        auto *apps = dialog->findChild<QComboBox *>("spatialApplication");
        apps->setCurrentIndex(apps->findText("Installed · Spatial Test App"));
        dialog->findChild<QComboBox *>("spatialWall")->setCurrentText("east");
        dialog->accept();
    });
    editor->findChild<QPushButton *>("spatialAddDisplay")->click();
    editor->findChild<QPushButton *>("spatialSave")->click();
    VERIFY(cockpit.spatialWorkspace().planets.size() == 1);
    const QString editorScreenshot = qEnvironmentVariable("DOCKPIPE_SPATIAL_SCREENSHOT");
    if (!editorScreenshot.isEmpty()) {
        settle(60);
        VERIFY(editor->grab().save(editorScreenshot + ".editor.png"));
    }
    const QString id = cockpit.spatialWorkspace().planets[0].id;
    VERIFY(cockpit.spatialWorkspace().planets[0].buildings[0].interior.displays.size() == 2);
    const QString buildingId = cockpit.spatialWorkspace().planets[0].buildings[0].id;
    const QString displayId = cockpit.spatialWorkspace().planets[0].buildings[0].interior.displays[0].id;
    cockpit.returnToOrbit();
    VERIFY(cockpit.flight()->buildingCount() == 8);
    cockpit.visit("planet:" + id);
    VERIFY(cockpit.destination() == "planet:" + id && !cockpit.flight()->orbit());
    VERIFY(cockpit.flight()->buildingCount() == 1);
    cockpit.flight()->fire();
    VERIFY(cockpit.destination().startsWith("building:"));
    VERIFY(cockpit.flight()->walking());
    VERIFY(cockpit.flight()->interiorStreamCount() == 0);
    QKeyEvent interact(QEvent::KeyPress, Qt::Key_E, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &interact);
    VERIFY(!cockpit.flight()->appControlling());
    VERIFY(cockpit.flight()->worldStatus().contains("Enable App control"));
    // Hide permission explanation before movement assertions.
    cockpit.findChild<QPushButton *>("cockpitControlsToggle")->click();
    cockpit.activateWindow();
    cockpit.raise();
    cockpit.flight()->setFocus();
    settle(200);
    const auto before = cockpit.flight()->cameraPosition();
    cockpit.findChild<CockpitController *>()->pilotInput({0, 1, 1}, {0, 0.2}, false);
    settle(250);
    cockpit.flight()->setPilotInput({}, {}, false);
    VERIFY(cockpit.flight()->cameraPosition().z() > before.z());
    VERIFY(cockpit.flight()->cameraPosition().y() == 160);
    VERIFY(!cockpit.flight()->moving());
    QKeyEvent walk(QEvent::KeyPress, Qt::Key_D, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &walk);
    settle(80);
    QKeyEvent release(QEvent::KeyRelease, Qt::Key_D, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &release);
    VERIFY(cockpit.flight()->cameraPosition().x() > before.x());
    cockpit.flight()->setPilotInput({1, 0, 0}, {}, true);
    settle(1200);
    cockpit.flight()->setPilotInput({}, {}, false);
    VERIFY(cockpit.flight()->cameraPosition().x() <= 575);
    settle(60);
    const quint64 idleFrames = cockpit.flight()->renderedFrames();
    settle(100);
    VERIFY(cockpit.flight()->renderedFrames() == idleFrames);
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_SPATIAL_SCREENSHOT");
    const auto capture = [&cockpit](const QString &path) {
        if (QGuiApplication::platformName() == "offscreen")
            return cockpit.grab().save(path);
        return cockpit.screen()->grabWindow(cockpit.winId()).save(path);
    };
    if (!screenshot.isEmpty())
        VERIFY(capture(screenshot));
    if (window.id) {
        cockpit.visit("building:" + buildingId);
        auto preferences = cockpit.settings();
        preferences.appControl = true;
        cockpit.applySettings(preferences);
        cockpit.flight()->destinationActivated("display:" + displayId, false);
        for (int attempt = 0; attempt < 150 && !cockpit.window()->findChild<QWidget *>("billboardNativeApp"); ++attempt)
            settle(20);
        VERIFY(cockpit.window()->findChild<QWidget *>("billboardNativeApp"));
        settle(300);
        VERIFY(cockpit.flight()->appControlling());
#ifdef DOCKPIPE_X11_BILLBOARDS
        Display *connection = XOpenDisplay(nullptr);
        VERIFY(connection);
        XKeyEvent key{};
        key.display = connection;
        key.window = window.id;
        key.root = DefaultRootWindow(connection);
        key.type = nativeKeyPress;
        key.keycode = XKeysymToKeycode(connection, XK_a);
        key.same_screen = True;
        XSendEvent(connection, window.id, False, KeyPressMask, reinterpret_cast<XEvent *>(&key));
        XFlush(connection);
        QByteArray input;
        for (int attempt = 0; attempt < 100 && !input.contains("TEXT:a"); ++attempt) {
            settle(20);
            input += fixture.readAllStandardOutput();
        }
        XCloseDisplay(connection);
        VERIFY(input.contains("TEXT:a"));
#endif
        cockpit.window()->findChild<QPushButton *>("billboardReleaseControl")->click();
        settle(1200);
        VERIFY(cockpit.flight()->captureCount() > 0);
        VERIFY(!cockpit.flight()->appControlling());
        VERIFY(cockpit.flight()->interiorStreamCount() == 1);
        if (!screenshot.isEmpty())
            VERIFY(capture(screenshot + ".display.png"));
        std::printf("capture_count=%llu last_capture_ms=%.3f\n",
            static_cast<unsigned long long>(cockpit.flight()->captureCount()), cockpit.flight()->lastCaptureNanoseconds() / 1e6);
        // Two references to one explicitly selected app reuse the same stream.
        const QString secondDisplay = cockpit.spatialWorkspace().planets[0].buildings[0].interior.displays[1].id;
        VERIFY(cockpit.flight()->connectInteriorDisplay(secondDisplay, window, false));
        VERIFY(cockpit.flight()->interiorStreamCount() == 1);
        cockpit.activateWindow();
        cockpit.raise();
        cockpit.flight()->setFocus();
        settle(200);
        auto *navigationToggle = cockpit.findChild<QPushButton *>("cockpitNavigationToggle");
        VERIFY(cockpit.flight()->childAt(navigationToggle->geometry().center()) == navigationToggle);
        cockpit.flight()->setPilotInput({}, {1, 0}, false);
        settle(3300);
        cockpit.flight()->setPilotInput({}, {}, false);
        settle(100);
        const auto awayCaptures = cockpit.flight()->captureCount();
        settle(600);
        VERIFY(cockpit.flight()->captureCount() == awayCaptures);
        VERIFY(cockpit.flight()->connectInteriorDisplay(displayId, window, true));
        settle(300);
        VERIFY(cockpit.flight()->appControlling());
        cockpit.goBack();
        VERIFY(fixture.state() == QProcess::Running);
        VERIFY(!cockpit.flight()->appControlling() && cockpit.flight()->interiorStreamCount() == 0);
        cockpit.visit("building:" + buildingId);
        QProcess gpu;
        gpu.start("nvidia-smi", {"--query-compute-apps=pid,used_gpu_memory", "--format=csv,noheader,nounits"});
        QString gpuMemory = "unavailable: per-process graphics allocation not exposed";
        if (gpu.waitForStarted(1000) && gpu.waitForFinished(2000)) {
            const auto rows = QString::fromUtf8(gpu.readAllStandardOutput()).split('\n');
            for (const auto &row : rows) {
                const auto columns = row.split(',');
                if (columns.size() == 2 && columns[0].trimmed().toLongLong() == QCoreApplication::applicationPid())
                    gpuMemory = columns[1].trimmed() + " MiB";
            }
        } else {
            gpu.kill();
            gpu.waitForFinished(1000);
        }
        std::printf("gpu_memory=%s\n", qPrintable(gpuMemory));
    }
    const std::clock_t idleStart = std::clock();
    cockpit.hide();
    settle(50);
    const auto hiddenFrames = cockpit.flight()->renderedFrames();
    const auto hiddenCaptures = cockpit.flight()->captureCount();
    settle(1000);
    VERIFY(cockpit.flight()->renderedFrames() == hiddenFrames);
    VERIFY(cockpit.flight()->captureCount() == hiddenCaptures);
    std::printf("hidden_cpu_ms=%.3f hidden_paints=0 hidden_captures=0 last_draw_ms=%.3f\n",
        (std::clock() - idleStart) * 1000.0 / CLOCKS_PER_SEC, cockpit.flight()->lastDrawNanoseconds() / 1e6);
    cockpit.show();
    cockpit.goBack();
    VERIFY(cockpit.destination() == "planet:" + id);
    cockpit.goBack();
    VERIFY(cockpit.flight()->orbit());
    CockpitWidget restarted;
    VERIFY(restarted.spatialWorkspace().planets[0].name == "Development");
    VERIFY(restarted.flight()->buildingCount() == 8);
    VERIFY(restarted.spatialWorkspace().planets[0].buildings[0].interior.displays.size() == 2);
    restarted.visit("building:" + buildingId);
    VERIFY(restarted.flight()->interiorStreamCount() == 0 && !restarted.flight()->appControlling());
    QProcess restart;
    auto restartEnvironment = QProcessEnvironment::systemEnvironment();
    restartEnvironment.insert("QT_QPA_PLATFORM", "offscreen");
    restart.setProcessEnvironment(restartEnvironment);
    restart.start(QCoreApplication::applicationFilePath(), {"--spatial-readback"});
    VERIFY(restart.waitForStarted(2000) && restart.waitForFinished(5000));
    VERIFY(restart.exitStatus() == QProcess::NormalExit && restart.exitCode() == 0);
    cockpit.returnToOrbit();
    cockpit.flight()->approach("planet:" + id);
    VERIFY(cockpit.flight()->travelling());
    cockpit.flight()->skipTravel();
    VERIFY(cockpit.destination() == "planet:" + id && !cockpit.flight()->travelling());
    cockpit.flight()->approach("building:" + buildingId);
    VERIFY(cockpit.flight()->travelling());
    cockpit.flight()->skipTravel();
    VERIFY(!cockpit.flight()->travelling() && !cockpit.flight()->walking());
    cockpit.visit("building:" + buildingId);
    QKeyEvent escape(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &escape);
    VERIFY(cockpit.destination() == "planet:" + id);
    cockpit.visit("apps");
    VERIFY(cockpit.destination() == "apps" && cockpit.flight()->buildingCount() >= 1);
    if (window.id) {
        VERIFY(fixture.state() == QProcess::Running);
        DesktopAppWindow alive(window);
        VERIFY(alive.alive());
        fixture.terminate();
        VERIFY(fixture.waitForFinished(3000));
        auto missing = cockpit.spatialWorkspace();
        missing.planets[0].buildings[0].interior.displays[0].application = {"missing.desktop", "NeverInstalledFixture", "Missing fixture"};
        QString error;
        VERIFY(cockpit.saveSpatialWorkspace(missing, error));
        cockpit.visit("building:" + buildingId);
        cockpit.flight()->destinationActivated("display:" + displayId, false);
        VERIFY(cockpit.flight()->worldStatus().contains("unavailable"));
        VERIFY(cockpit.flight()->interiorStreamCount() == 0);
        // Removing a display is presentation-only and does not invoke a launcher.
        missing.planets[0].buildings[0].interior.displays.clear();
        VERIFY(cockpit.saveSpatialWorkspace(missing, error));
    }
    cockpit.visit("building:" + buildingId);
    cockpit.activateWindow();
    cockpit.flight()->setFocus();
    settle(300);
    const auto benchmark = [&cockpit](const char *scene) {
        settle(100);
        QVector<double> frameTimes;
        QVector<double> drawTimes;
        for (int frame = 0; frame < 50; ++frame) {
            QElapsedTimer timer;
            timer.start();
            cockpit.flight()->viewport()->repaint();
            QApplication::processEvents();
            if (frame >= 10) {
                frameTimes.append(timer.nsecsElapsed() / 1e6);
                drawTimes.append(cockpit.flight()->lastDrawNanoseconds() / 1e6);
            }
        }
        std::sort(frameTimes.begin(), frameTimes.end());
        std::sort(drawTimes.begin(), drawTimes.end());
        std::printf("scene=%s renderer=%s frame_median_ms=%.3f frame_p95_ms=%.3f draw_p95_ms=%.3f\n",
            scene, qPrintable(cockpit.flight()->rendererDescription()), frameTimes[20], frameTimes[38], drawTimes[38]);
    };
    SpatialInterior capacityRoom;
    for (int index = 0; index < 24; ++index) {
        SpatialDisplay surface;
        surface.id = SpatialWorkspace::newId();
        surface.application = {"fixture.desktop", "fixture", "Application reference"};
        surface.wall = QStringList{"north", "east", "west"}[index / 8];
        surface.position = {-360.0 + (index % 4) * 240, 120.0 + ((index % 8) / 4) * 160};
        surface.size = {200, 120};
        capacityRoom.displays.append(surface);
    }
    cockpit.flight()->setInterior(capacityRoom, "Capacity fixture");
    benchmark("room-24-displays");
    QVector<WorkspaceFlightBuilding> buildings;
    for (int index = 0; index < 64; ++index)
        buildings.append({QString("Building %1").arg(index), 100, true, false, QString("fixture:%1").arg(index)});
    cockpit.flight()->setCockpitWorld(buildings, false);
    benchmark("planet-64-buildings");
    buildings.resize(31);
    cockpit.flight()->setCockpitWorld(buildings, true);
    benchmark("orbit-31-planets");
    std::puts("Spatial UI, placement, permissions, walking, collision, lifecycle, restart and Applications checks passed.");
    return 0;
}
