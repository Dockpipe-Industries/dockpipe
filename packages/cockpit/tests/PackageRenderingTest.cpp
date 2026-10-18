#include "WorkspacePackageManager.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QGraphicsView>
#include <QKeyEvent>
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
public:
    QVector<double> intervals;
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
    CHECK(flight->viewport()->inherits("QOpenGLWidget"));
    FrameRecorder recorder;
    flight->viewport()->installEventFilter(&recorder);
    QKeyEvent forward(QEvent::KeyPress, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &forward);
    idle(1500);
    QKeyEvent stop(QEvent::KeyRelease, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &stop);
    flight->viewport()->removeEventFilter(&recorder);
    CHECK(!recorder.intervals.isEmpty());
    std::sort(recorder.intervals.begin(), recorder.intervals.end());
    const QString capture = qEnvironmentVariable("DOCKPIPE_PACKAGE_RENDER_CAPTURE");
    if (!capture.isEmpty())
        CHECK(host.screen()->grabWindow(host.winId()).save(capture));
    std::printf("package_load_ms=%lld cockpit_construction_ms=%lld movement_frames=%lld frame_p95_ms=%.2f frame_max_ms=%.2f compositor_failures=%d\n",
        loadMilliseconds, constructionMilliseconds, qint64(recorder.intervals.size()),
        recorder.intervals[recorder.intervals.size() * 95 / 100], recorder.intervals.last(), compositorFailures);
    CHECK(compositorFailures == 0);
    CHECK(recorder.intervals.size() >= 40);
    surface->hide();
    surface->show();
    idle(100);
    CHECK(compositorFailures == 0);
    qInstallMessageHandler(previousHandler);
    return 0;
}
