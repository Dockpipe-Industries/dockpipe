#include "CockpitWidget.h"
#include "CockpitHudPanel.h"
#include "FurnishingController.h"
#include "FurnishingLayout.h"
#include <QApplication>
#include <QCheckBox>
#include <QElapsedTimer>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMainWindow>
#include <QOpenGLWidget>
#include <QPainter>
#include <QProcess>
#include <QPushButton>
#include <QScreen>
#include <QStackedWidget>
#include <QThread>
#include <QTimer>
#include <QSocketNotifier>
#include <cstdio>
#ifdef Q_OS_UNIX
#include <unistd.h>
#endif

#define VERIFY(value) do { if (!(value)) { std::fprintf(stderr, "Handoff assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static void settle(int milliseconds)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}

class GpuAppFixture final : public QOpenGLWidget {
protected:
    void paintGL() override
    {
        QPainter painter(this);
        painter.fillRect(rect(), QColor("#195a89"));
        painter.setPen(Qt::white);
        painter.drawText(rect(), Qt::AlignCenter, "GPU APP FIXTURE");
    }
};

int nativeHandoffFixture()
{
    qApp->setQuitOnLastWindowClosed(false);
    QVector<QWidget *> windows;
    const auto create = [&]() {
        auto *window = new QWidget;
        windows.append(window);
        window->setWindowTitle("Dockpipe isolated GPU application");
        auto *layout = new QVBoxLayout(window);
        layout->addWidget(new GpuAppFixture, 1);
        auto *input = new QLineEdit;
        input->setPlaceholderText("Disposable input fixture");
        layout->addWidget(input);
        window->resize(640, 480);
        window->show();
        std::printf("WINDOW:%llu\n", static_cast<unsigned long long>(window->winId()));
        std::fflush(stdout);
    };
    create();
#ifdef Q_OS_UNIX
    QSocketNotifier commands(STDIN_FILENO, QSocketNotifier::Read);
    QObject::connect(&commands, &QSocketNotifier::activated, qApp, [&]() {
        char bytes[64];
        const auto count = ::read(STDIN_FILENO, bytes, sizeof(bytes));
        if (count <= 0) {
            commands.setEnabled(false);
            return;
        }
        const QByteArray command(bytes, int(count));
        if (command == "sibling\n") {
            if (windows.size() == 1)
                create();
            return;
        }
        if (command == "close-first\n") {
            if (!windows.isEmpty())
                delete windows.takeFirst();
            return;
        }
        if (command != "replace\n" && command != "ambiguous\n" && command != "missing\n")
            return;
        qDeleteAll(windows);
        windows.clear();
        if (command != "missing\n") {
            QTimer::singleShot(250, qApp, [&, command]() {
                create();
                if (command == "ambiguous\n")
                    create();
            });
        }
    });
#endif
    QTimer::singleShot(90000, qApp, &QApplication::quit);
    const int result = qApp->exec();
    qDeleteAll(windows);
    return result;
}

int nativeHandoffTest()
{
    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
    const QString fixturePath = qEnvironmentVariable("DOCKPIPE_SPATIAL_FIXTURE");
    const int generalHandoffCycles = qEnvironmentVariableIsSet("DOCKPIPE_NATIVE_ARCADE_ONLY") ? 0 : 3;
    QProcess fixture;
    if (fixturePath.isEmpty())
        fixture.start(QCoreApplication::applicationFilePath(), {"--gpu-app-fixture"});
    else
        fixture.start(fixturePath, {"--fixture"});
    VERIFY(fixture.waitForStarted());
    QByteArray output;
    for (int attempt = 0; attempt < 100 && !output.contains('\n'); ++attempt) {
        settle(20);
        output += fixture.readAllStandardOutput();
    }
    const quint64 id = output.split('\n').first().mid(7).toULongLong();
    VERIFY(id);
    DesktopAppChoice choice{id, "Disposable app", "Fixture"};
    QMainWindow host;
    host.setWindowTitle("Dockpipe isolated handoff regression");
    host.setWindowFlag(Qt::WindowStaysOnTopHint);
    auto *central = new QWidget;
    auto *layout = new QHBoxLayout(central);
    auto *sidebar = new QLabel("Fixture sidebar");
    sidebar->setFixedWidth(160);
    layout->addWidget(sidebar);
    auto *pages = new QStackedWidget;
    layout->addWidget(pages);
    pages->addWidget(new QWidget);
    auto *cockpit = new CockpitWidget;
    cockpit->appWindowProvider = [choice](QString &error) {
        error.clear();
        return QVector<DesktopAppChoice>{choice};
    };
    pages->addWidget(cockpit);
    pages->setCurrentWidget(cockpit);
    host.setCentralWidget(central);
    host.resize(1200, 800);
    host.show();
    cockpit->visit("apps");
    auto *flight = cockpit->flight();
    settle(600);
    const QString appPath = "app:" + QString::number(id);
    VERIFY(flight->captureCount() == 0);
    {
        auto preferences = cockpit->settings();
        preferences.appControl = true;
        cockpit->applySettings(preferences);
    }
    for (int attempt = 0; attempt < 100 && !flight->hasAppPreview(appPath); ++attempt)
        settle(20);
    VERIFY(flight->hasAppPreview(appPath));
    VERIFY(!flight->appControlling());
    QWidget *originalViewport = flight->viewport();
    DesktopAppWindow hostCapture({quint64(host.winId()), "Isolated test host", "DockpipeCockpitTest"});
    const auto hostPixels = [&]() {
        QString captureError;
        const auto image = hostCapture.capture(captureError);
        if (!captureError.isEmpty())
            std::fprintf(stderr, "Host capture: %s\n", qPrintable(captureError));
        return image;
    };
    const auto captureRectangle = [&](const QRect &logical) {
        const auto fitted = QSizeF(host.size()).scaled(1280, 720, Qt::KeepAspectRatio);
        const qreal scale = fitted.width() / host.width();
        const QPointF padding((1280 - fitted.width()) / 2, (720 - fitted.height()) / 2);
        return QRectF(padding + QPointF(logical.topLeft()) * scale, QSizeF(logical.size()) * scale).toAlignedRect();
    };
    const auto visibleScene = [&](const QString &suffix) {
        host.raise();
        host.activateWindow();
        flight->viewport()->update();
        settle(300);
        const QPoint origin = flight->viewport()->mapTo(&host, QPoint());
        const QImage image = hostPixels().copy(captureRectangle(QRect(origin, flight->viewport()->size())));
        const QString prefix = qEnvironmentVariable("DOCKPIPE_SPATIAL_SCREENSHOT");
        if (!prefix.isEmpty())
            image.save(prefix + suffix + ".png");
        int colored = 0;
        int samples = 0;
        for (int y = image.height() / 4; y < image.height() * 3 / 4; y += 8) {
            for (int x = image.width() / 4; x < image.width() * 3 / 4; x += 8) {
                ++samples;
                const QColor pixel = image.pixelColor(x, y);
                if (pixel.blue() > 15 || pixel.green() > 15 || pixel.red() > 15)
                    ++colored;
            }
        }
        std::printf("handoff%s colored=%d/%d renderer=%s native_flight=%d native_parent=%d\n",
            qPrintable(suffix), colored, samples, qPrintable(flight->rendererDescription()),
            flight->testAttribute(Qt::WA_NativeWindow), cockpit->testAttribute(Qt::WA_NativeWindow));
        return samples > 0 && colored > samples / 2;
    };
    VERIFY(visibleScene("-before"));
    for (int cycle = 0; cycle < generalHandoffCycles; ++cycle) {
        flight->controlApp(choice, false);
        for (int attempt = 0; attempt < 150 && !flight->window()->findChild<QWidget *>("billboardNativeApp"); ++attempt)
            settle(20);
        VERIFY(flight->window()->findChild<QWidget *>("billboardNativeApp"));
        settle(300);
        VERIFY(flight->appControlling());
        VERIFY(flight->viewport() == originalViewport);
        VERIFY(!flight->testAttribute(Qt::WA_NativeWindow));
        VERIFY(!cockpit->testAttribute(Qt::WA_NativeWindow));
        flight->window()->findChild<QPushButton *>("billboardReleaseControl")->click();
        settle(500);
        VERIFY(!flight->appControlling());
        VERIFY(flight->viewport() == originalViewport);
        VERIFY(visibleScene(QString("-return-%1").arg(cycle)));
        VERIFY(fixture.state() == QProcess::Running);
    }
    {
        auto preferences = cockpit->settings();
        preferences.appControl = false;
        cockpit->applySettings(preferences);
    }
    VERIFY(!flight->hasAppPreview(appPath));
    SpatialDisplay display;
    display.id = SpatialWorkspace::newId();
    display.application = {"fixture.desktop", choice.applicationClass, choice.title};
    display.size = QSizeF(700, 300);
    SpatialBuilding building;
    building.id = SpatialWorkspace::newId();
    building.name = "Handoff room";
    building.interior.displays = {display};
    SpatialPlanet planet;
    planet.id = SpatialWorkspace::newId();
    planet.name = "Fixture planet";
    planet.buildings = {building};
    SpatialWorkspace workspace;
    workspace.planets = {planet};
    QString error;
    VERIFY(cockpit->saveSpatialWorkspace(workspace, error));
    cockpit->visit("building:" + building.id);
    const auto stoppedCaptures = flight->captureCount();
    settle(600);
    VERIFY(flight->captureCount() == stoppedCaptures && flight->interiorStreamCount() == 0);
    cockpit->appWindowProvider = [choice](QString &error) {
        error.clear();
        auto other = choice;
        ++other.id;
        return QVector<DesktopAppChoice>{choice, other};
    };
    {
        auto preferences = cockpit->settings();
        preferences.appControl = true;
        cockpit->applySettings(preferences);
    }
    settle(600);
    VERIFY(flight->interiorStreamCount() == 0 && !flight->appControlling());
    {
        auto preferences = cockpit->settings();
        preferences.appControl = false;
        cockpit->applySettings(preferences);
    }
    cockpit->appWindowProvider = [choice](QString &error) {
        error.clear();
        return QVector<DesktopAppChoice>{choice};
    };
    {
        auto preferences = cockpit->settings();
        preferences.appControl = true;
        cockpit->applySettings(preferences);
    }
    for (int attempt = 0; attempt < 100 && !flight->hasInteriorPreview(display.id); ++attempt)
        settle(20);
    VERIFY(flight->hasInteriorPreview(display.id) && !flight->appControlling());
    VERIFY(visibleScene("-room-preview"));
    for (int cycle = 0; cycle < generalHandoffCycles; ++cycle) {
        flight->destinationActivated("display:" + display.id, false);
        for (int attempt = 0; attempt < 150 && !host.findChild<QWidget *>("billboardNativeApp"); ++attempt)
            settle(20);
        VERIFY(host.findChild<QWidget *>("billboardNativeApp"));
        settle(300);
        VERIFY(flight->appControlling());
        auto *panel = host.findChild<CockpitHudPanel *>("billboardControlPanel");
        VERIFY(panel && panel->isVisible());
        VERIFY(QRect(flight->mapTo(&host, QPoint()), flight->size()).contains(panel->geometry()));
        if (cycle == 1) {
            panel->fullscreenButton()->click();
            settle(400);
            VERIFY(flight->flightFullscreen() && panel->isVisible() && flight->appControlling());
            panel->fullscreenButton()->click();
            settle(400);
            VERIFY(!flight->flightFullscreen() && panel->isVisible() && flight->appControlling());
        }
        if (cycle == 2) {
            host.hide();
            settle(150);
            VERIFY(!flight->appControlling());
            host.show();
        } else {
            panel->closeButton()->click();
        }
        settle(600);
        VERIFY(flight->viewport() == originalViewport);
        VERIFY(!flight->testAttribute(Qt::WA_NativeWindow));
        VERIFY(!cockpit->testAttribute(Qt::WA_NativeWindow));
        for (int attempt = 0; attempt < 100 && !flight->hasInteriorPreview(display.id); ++attempt)
            settle(20);
        VERIFY(!flight->appControlling() && flight->hasInteriorPreview(display.id));
        VERIFY(visibleScene(QString("-room-return-%1").arg(cycle)));
    }
    // Native input is fitted to the cabinet's projected screen while its shell
    // and independent marquee remain in the first-person scene.
    FurnishingLayout arcade;
    VERIFY(arcade.load("fixture:native-arcade", error));
    FurnishingObject cabinet;
    cabinet.id = "arcade";
    cabinet.type = "arcade-cabinet";
    cabinet.position = {0, 111, 100};
    cabinet.binding = {"application", {}, {}, display.application};
    cabinet.marqueeBinding = {"text", {}, "DOCKPIPE ARCADE"};
    arcade.objects = {cabinet};
    VERIFY(arcade.save(error));
    SpatialInterior arcadeRoom;
    arcadeRoom.size = {2000, 2000};
    arcadeRoom.environmentKey = arcade.environment;
    flight->setInterior(arcadeRoom, "Native arcade fixture");
    flight->setSurfaceSpawn({0, -200});
    const auto arcadeStanding = flight->cameraPosition();
    for (int cycle = 0; cycle < 4; ++cycle) {
        if (cycle == 2) {
            flight->setFlightFullscreen(true);
            settle(400);
        }
        if (cycle == 0)
            flight->fire();
        else
            VERIFY(flight->connectInteriorDisplay("furnishing:arcade", choice, true));
        for (int attempt = 0; attempt < 150 && !host.findChild<QWidget *>("billboardNativeApp"); ++attempt)
            settle(20);
        VERIFY(host.findChild<QWidget *>("billboardNativeApp"));
        VERIFY(flight->appControlling());
        auto *panel = host.findChild<CockpitHudPanel *>("billboardControlPanel");
        VERIFY(panel && !panel->closeButton()->isVisible());
        VERIFY(panel->geometry() == flight->cabinetControlRect().translated(flight->viewport()->mapTo(&host, QPoint())));
        VERIFY(host.findChild<QPushButton *>("arcadeLeaveControl")->isVisible());
        if (fixturePath.isEmpty() && (cycle == 1 || cycle == 2)) {
            const auto playingPosition = flight->cameraPosition();
            fixture.readAllStandardOutput();
            fixture.write("replace\n");
            QByteArray replacementOutput;
            for (int attempt = 0; attempt < 150 && !replacementOutput.contains('\n'); ++attempt) {
                settle(20);
                replacementOutput += fixture.readAllStandardOutput();
            }
            const quint64 replacementId = replacementOutput.split('\n').first().mid(7).toULongLong();
            VERIFY(replacementId && replacementId != choice.id);
            choice.id = replacementId;
            for (int attempt = 0; attempt < 200; ++attempt) {
                settle(20);
                if (panel->findChild<QWidget *>("billboardNativeApp")
                    && !panel->findChild<QLabel *>("billboardConnecting"))
                    break;
            }
            VERIFY(flight->appControlling());
            VERIFY(flight->cameraPosition() == playingPosition);
            VERIFY(panel->findChild<QWidget *>("billboardNativeApp"));
            VERIFY(!panel->findChild<QLabel *>("billboardConnecting"));
        }
        // Resize while connected, then let QGraphicsView finish resizing its
        // viewport before checking the actual app pixels against the 3D screen.
        if (cycle == 3)
            flight->setFlightFullscreen(true);
        else if (!flight->flightFullscreen())
            host.resize(host.width() - 80, host.height() - 30);
        settle(250);
        VERIFY(visibleScene(QString("-arcade-playing-%1").arg(cycle)));
        const QRect expected = flight->cabinetControlRect().translated(flight->viewport()->mapTo(&host, QPoint()));
        VERIFY(panel->geometry() == expected);
        const auto appPixels = hostPixels();
        const auto captured = captureRectangle(expected);
        const QPoint sample = captured.topLeft() + QPoint(captured.width() / 4, captured.height() / 4);
        const auto appColor = appPixels.pixelColor(sample);
        std::printf("Arcade screen pixel: %s at %d,%d\n", qPrintable(appColor.name()), sample.x(), sample.y());
        if (fixturePath.isEmpty())
            VERIFY(appColor == QColor("#195a89"));
        if (cycle == 0)
            host.findChild<QPushButton *>("arcadeLeaveControl")->click();
        else
            flight->goBack();
        settle(400);
        VERIFY(!flight->appControlling() && flight->cameraPosition() == arcadeStanding);
        VERIFY(flight->cabinetControlRect().isEmpty());
        VERIFY(!host.findChild<QPushButton *>("arcadeLeaveControl"));
        VERIFY(flight->viewport() == originalViewport && fixture.state() == QProcess::Running);
        flight->setFlightFullscreen(false);
        settle(300);
        VERIFY(visibleScene(QString("-arcade-return-%1").arg(cycle)));
    }
    if (fixturePath.isEmpty()) {
        // An unrelated process with the same window class must never be adopted.
        QProcess unrelated;
        unrelated.start(QCoreApplication::applicationFilePath(), {"--gpu-app-fixture"});
        VERIFY(unrelated.waitForStarted());
        QByteArray unrelatedOutput;
        for (int attempt = 0; attempt < 100 && !unrelatedOutput.contains('\n'); ++attempt) {
            settle(20);
            unrelatedOutput += unrelated.readAllStandardOutput();
        }
        VERIFY(unrelatedOutput.contains("WINDOW:"));
        const quint64 unrelatedId = unrelatedOutput.split('\n').first().mid(7).toULongLong();
        unrelated.write("sibling\n");
        QByteArray siblingOutput;
        for (int attempt = 0; attempt < 100 && !siblingOutput.contains('\n'); ++attempt) {
            settle(20);
            siblingOutput += unrelated.readAllStandardOutput();
        }
        VERIFY(siblingOutput.contains("WINDOW:"));
        settle(200);
        DesktopAppWindow withExistingSibling({unrelatedId, "Existing sibling fixture", {}});
        unrelated.write("close-first\n");
        for (int attempt = 0; attempt < 100 && withExistingSibling.alive(); ++attempt)
            settle(20);
        VERIFY(!withExistingSibling.alive());
        QString siblingError;
        VERIFY(!withExistingSibling.replacement(siblingError) && siblingError.isEmpty());
        for (const QByteArray command : {QByteArray("missing\n"), QByteArray("ambiguous\n")}) {
            VERIFY(flight->connectInteriorDisplay("furnishing:arcade", choice, true));
            settle(500);
            VERIFY(flight->appControlling());
            DesktopAppWindow original(choice);
            fixture.readAllStandardOutput();
            fixture.write(command);
            for (int attempt = 0; attempt < 350 && flight->appControlling(); ++attempt)
                settle(20);
            VERIFY(!flight->appControlling());
            VERIFY(flight->cameraPosition() == arcadeStanding);
            VERIFY(!host.findChild<QWidget *>("billboardControlPanel"));
            QString replacementError;
            VERIFY(!original.replacement(replacementError));
            if (command == "missing\n") {
                VERIFY(replacementError.isEmpty());
                VERIFY(flight->worldStatus().contains("5 seconds"));
            } else {
                VERIFY(replacementError.contains("Several"));
                VERIFY(flight->worldStatus().contains("Several"));
            }
            fixture.readAllStandardOutput();
            fixture.write("replace\n");
            QByteArray freshOutput;
            for (int attempt = 0; attempt < 150 && !freshOutput.contains('\n'); ++attempt) {
                settle(20);
                freshOutput += fixture.readAllStandardOutput();
            }
            choice.id = freshOutput.split('\n').first().mid(7).toULongLong();
            VERIFY(choice.id);
            // Manual selection is only possible after the WM publishes the
            // new client. Do not inject an ID printed before that handshake.
            bool selectable = false;
            for (int attempt = 0; attempt < 100 && !selectable; ++attempt) {
                settle(20);
                QString discoveryError;
                for (const auto &window : DesktopAppWindow::availableWindows(discoveryError))
                    selectable |= window.id == choice.id;
            }
            VERIFY(selectable);
        }
        unrelated.terminate();
        VERIFY(unrelated.waitForFinished(3000));
        VERIFY(flight->connectInteriorDisplay("furnishing:arcade", choice, false));
        VERIFY(flight->interiorStreamCount() == 1);
        settle(600);
        const auto captures = flight->captureCount();
        fixture.readAllStandardOutput();
        fixture.write("replace\n");
        QByteArray previewOutput;
        for (int attempt = 0; attempt < 150 && !previewOutput.contains('\n'); ++attempt) {
            settle(20);
            previewOutput += fixture.readAllStandardOutput();
        }
        choice.id = previewOutput.split('\n').first().mid(7).toULongLong();
        VERIFY(choice.id);
        for (int attempt = 0; attempt < 200 && flight->captureCount() <= captures; ++attempt)
            settle(20);
        VERIFY(flight->captureCount() > captures);
        VERIFY(flight->hasInteriorPreview("furnishing:arcade") && !flight->appControlling());
    }
    cockpit->appWindowProvider = [&choice](QString &error) {
        error.clear();
        return QVector<DesktopAppChoice>{choice};
    };
    cockpit->visit("building:" + building.id);
    flight->destinationActivated("display:" + display.id, false);
    settle(500);
    VERIFY(flight->appControlling());
    auto preferences = cockpit->settings();
    preferences.enabled = false;
    cockpit->applySettings(preferences);
    VERIFY(!flight->appControlling() && flight->interiorStreamCount() == 0);
    VERIFY(!flight->moving() && !flight->orbitAnimating());
    const auto disabledCaptures = flight->captureCount();
    settle(600);
    VERIFY(flight->captureCount() == disabledCaptures && fixture.state() == QProcess::Running);
    preferences.enabled = true;
    cockpit->applySettings(preferences);
    for (int attempt = 0; attempt < 100 && !flight->hasInteriorPreview(display.id); ++attempt)
        settle(20);
    VERIFY(flight->hasInteriorPreview(display.id));
    flight->destinationActivated("display:" + display.id, false);
    settle(500);
    VERIFY(flight->appControlling());
    fixture.terminate();
    VERIFY(fixture.waitForFinished(3000));
    settle(800);
    VERIFY(!flight->appControlling() && !flight->worldStatus().isEmpty());
    VERIFY(visibleScene("-closed-app"));
    std::puts("Apps and room previews, repeated return pixels, fullscreen, hide, master disable and closed-app recovery passed.");
    return 0;
}

