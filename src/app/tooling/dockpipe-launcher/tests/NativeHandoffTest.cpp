#include "CockpitWidget.h"
#include "CockpitHudPanel.h"
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
#include <cstdio>

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
    QWidget window;
    window.setWindowTitle("Dockpipe isolated GPU application");
    auto *layout = new QVBoxLayout(&window);
    layout->addWidget(new GpuAppFixture, 1);
    auto *input = new QLineEdit;
    input->setPlaceholderText("Disposable input fixture");
    layout->addWidget(input);
    window.resize(640, 480);
    window.show();
    std::printf("WINDOW:%llu\n", static_cast<unsigned long long>(window.winId()));
    std::fflush(stdout);
    QTimer::singleShot(60000, qApp, &QApplication::quit);
    return qApp->exec();
}

int nativeHandoffTest()
{
    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
    const QString fixturePath = qEnvironmentVariable("DOCKPIPE_SPATIAL_FIXTURE");
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
    const auto visibleScene = [&](const QString &suffix) {
        host.raise();
        host.activateWindow();
        flight->viewport()->update();
        settle(300);
        const QPoint origin = flight->viewport()->mapTo(&host, QPoint());
        const QImage image = host.screen()->grabWindow(host.winId(), origin.x(), origin.y(),
            flight->viewport()->width(), flight->viewport()->height()).toImage();
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
    for (int cycle = 0; cycle < 3; ++cycle) {
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
    for (int cycle = 0; cycle < 3; ++cycle) {
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
        VERIFY(!flight->appControlling() && flight->hasInteriorPreview(display.id));
        VERIFY(visibleScene(QString("-room-return-%1").arg(cycle)));
    }
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

