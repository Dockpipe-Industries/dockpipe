// Host ABI and late-loading qualification transferred from
// SpatialPipe/packages/cockpit/tests/PackageRenderingTest.cpp.
// The package implementation and its feature tests remain owned by SpatialPipe.
#include "WorkspacePackageManager.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QGraphicsView>
#include <QKeyEvent>
#include <QLibrary>
#include <QOffscreenSurface>
#include <QOpenGLContext>
#include <QOpenGLFunctions>
#include <QPushButton>
#include <QScreen>
#include <QTemporaryDir>
#include <QThread>
#include <QVBoxLayout>
#include <algorithm>
#include <cstdio>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Rendering assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
int compositorFailures = 0;
QtMessageHandler previousHandler = nullptr;

void recordMessage(QtMsgType type, const QMessageLogContext &context, const QString &message)
{
    if (message.contains("no opengl support set"))
        ++compositorFailures;
    if (previousHandler)
        previousHandler(type, context, message);
}

void idle(int milliseconds)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}

class FrameRecorder final : public QObject {
    Q_OBJECT
public:
    QVector<double> intervals;
    int completedFrames = 0;
public slots:
    void recordCompletedFrame()
    {
        ++completedFrames;
    }
protected:
    bool eventFilter(QObject *, QEvent *event) override
    {
        if (event->type() == QEvent::Paint) {
            if (m_timer.isValid())
                intervals.append(m_timer.nsecsElapsed() / 1e6);
            m_timer.start();
        }
        return false;
    }
private:
    QElapsedTimer m_timer;
};

void recordMovement(QGraphicsView *flight, FrameRecorder &recorder)
{
    flight->viewport()->installEventFilter(&recorder);
    QKeyEvent forward(QEvent::KeyPress, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &forward);
    idle(1500);
    QKeyEvent stop(QEvent::KeyRelease, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &stop);
    flight->viewport()->removeEventFilter(&recorder);
    std::sort(recorder.intervals.begin(), recorder.intervals.end());
}

void reportFrames(const char *phase, const FrameRecorder &recorder)
{
    const double percentile95 = recorder.intervals.isEmpty() ? 0
        : recorder.intervals[recorder.intervals.size() * 95 / 100];
    const double maximum = recorder.intervals.isEmpty() ? 0 : recorder.intervals.last();
    std::printf("phase=%s movement_frames=%lld frame_p95_ms=%.2f frame_max_ms=%.2f compositor_failures=%d\n",
        phase, qint64(recorder.intervals.size()), percentile95, maximum, compositorFailures);
}
}

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication app(argc, argv);
    if (app.platformName() == "offscreen" || app.platformName() == "minimal")
        return 77;
    QTemporaryDir directory;
    CHECK(directory.isValid());
    qputenv("XDG_CONFIG_HOME", directory.path().toUtf8());
    qputenv("XDG_DATA_HOME", directory.path().toUtf8());
    previousHandler = qInstallMessageHandler(recordMessage);

    // Link only the host SDK, never Cockpit. The host's backing store must
    // already exist before the optional package is loaded. This catches missing
    // startup compositor support that tests linked to Cockpit cannot reproduce.
    QWidget host;
    host.resize(1440, 900);
    auto *layout = new QVBoxLayout(&host);
    auto *open = new QPushButton("Open Cockpit");
    layout->addWidget(open);
    host.show();
    idle(200);

    WorkspacePackageManager manager;
    LauncherSettings settings;
    settings.projectFolder = directory.path();
    QElapsedTimer entry;
    entry.start();
    manager.prepare(settings, false);
    CHECK(manager.ready());
    const qint64 loadMilliseconds = entry.elapsed();
    auto *surface = manager.createWorkspace(&host);
    CHECK(surface);
    surface->setWorkspace(directory.path());
    CockpitSettings preferences;
    preferences.enabled = true;
    preferences.orbitalMotion = false;
    surface->applySettings(preferences);
    layout->addWidget(surface);
    open->hide();
    const qint64 constructionMilliseconds = entry.elapsed() - loadMilliseconds;
    idle(500);
    auto *flight = surface->findChild<QGraphicsView *>("workspaceDroneFlight");
    CHECK(flight);
    QOpenGLContext probe;
    CHECK(probe.create());
    QOffscreenSurface probeSurface;
    probeSurface.setFormat(probe.format());
    probeSurface.create();
    CHECK(probeSurface.isValid() && probe.makeCurrent(&probeSurface));
    const auto *rendererBytes = probe.functions()->glGetString(GL_RENDERER);
    CHECK(rendererBytes);
    const QString renderer = QString::fromLatin1(reinterpret_cast<const char *>(rendererBytes));
    probe.doneCurrent();
    const QString normalizedRenderer = renderer.toLower();
    const bool softwareOpenGL = normalizedRenderer.contains("llvmpipe") || normalizedRenderer.contains("softpipe")
        || normalizedRenderer.contains("software") || normalizedRenderer.contains("basic render");
    if (softwareOpenGL)
        CHECK(!flight->viewport()->inherits("QOpenGLWidget"));
    else
        CHECK(flight->viewport()->inherits("QOpenGLWidget"));
    std::printf("opengl_renderer=%s product_viewport=%s package_load_ms=%lld cockpit_construction_ms=%lld\n",
        qPrintable(renderer), softwareOpenGL ? "raster" : "hardware-OpenGL",
        loadMilliseconds, constructionMilliseconds);

    // Keep the performance contract on the viewport selected by the product.
    // Software Mesa uses raster rendering; the synthetic GL phase below is only
    // a late-compositor integration check, not a supported rendering mode.
    FrameRecorder productRecorder;
    recordMovement(flight, productRecorder);
    reportFrames("product", productRecorder);
    CHECK(compositorFailures == 0);
    CHECK(productRecorder.intervals.size() >= 40);
    surface->hide();
    surface->show();
    idle(100);
    CHECK(compositorFailures == 0);

    if (softwareOpenGL) {
        // Keep OpenGLWidgets out of this host's link dependencies. Loading the
        // private factory now preserves the original late-import condition.
        QLibrary factory(QStringLiteral(DOCKPIPE_TEST_GL_FACTORY_PATH));
        factory.setLoadHints(QLibrary::PreventUnloadHint);
        CHECK(factory.load());
        using CreateViewport = QWidget *(*)(const QSurfaceFormat *);
        using ValidViewport = bool (*)(const QWidget *);
        auto createViewport = reinterpret_cast<CreateViewport>(factory.resolve("dockpipe_test_create_gl_viewport"));
        auto validViewport = reinterpret_cast<ValidViewport>(factory.resolve("dockpipe_test_gl_viewport_valid"));
        CHECK(createViewport && validViewport);
        const auto format = probe.format();
        auto *viewport = createViewport(&format);
        CHECK(viewport);
        flight->setViewportUpdateMode(QGraphicsView::FullViewportUpdate);
        flight->setViewport(viewport);
        FrameRecorder compositorRecorder;
        // Observe completed composition without linking OpenGLWidgets into this
        // host, which would defeat the late-loading regression condition.
        CHECK(QObject::connect(viewport, SIGNAL(frameSwapped()),
            &compositorRecorder, SLOT(recordCompletedFrame())));
        idle(500);
        CHECK(validViewport(viewport));
        compositorRecorder.completedFrames = 0;
        recordMovement(flight, compositorRecorder);
        reportFrames("synthetic-software-GL", compositorRecorder);
        std::printf("phase=synthetic-software-GL completed_frames=%d\n", compositorRecorder.completedFrames);
        CHECK(validViewport(viewport));
        CHECK(compositorRecorder.intervals.size() >= 2);
        CHECK(compositorRecorder.completedFrames >= 2);
        CHECK(compositorFailures == 0);

        const int completedBeforeShow = compositorRecorder.completedFrames;
        surface->hide();
        surface->show();
        viewport->update();
        idle(500);
        std::printf("phase=synthetic-software-GL recovery_completed_frames=%d\n",
            compositorRecorder.completedFrames - completedBeforeShow);
        CHECK(validViewport(viewport));
        CHECK(compositorRecorder.completedFrames > completedBeforeShow);
        CHECK(compositorFailures == 0);
    }
    const QString capture = qEnvironmentVariable("DOCKPIPE_PACKAGE_RENDER_CAPTURE");
    if (!capture.isEmpty())
        CHECK(host.screen()->grabWindow(host.winId()).save(capture));
    qInstallMessageHandler(previousHandler);
    return 0;
}

#include "PackageRenderingTest.moc"
