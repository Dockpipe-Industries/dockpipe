#include "WorkspaceGraphWidget.h"
#include "WorkspaceGraphNode.h"
#include "WorkspaceFlightView.h"
#include <QKeyEvent>
#include "WorkspaceGraphPage.h"
#include "WorkspaceGraphReader.h"
#include "ProjectFilesWidget.h"
#include "CanvasView.h"
#include "Theme.h"
#include <QAction>
#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QJsonArray>
#include <QGraphicsItem>
#include <QJsonDocument>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPointer>
#include <QOpenGLWidget>
#include <QOpenGLFunctions>
#include <QOpenGLContext>
#include <QGraphicsRectItem>
#include <QScreen>
#include <QPushButton>
#include <QProcess>
#include <QMessageBox>
#include <QShortcut>
#include <QSplitter>
#include <QStackedWidget>
#include <QStyle>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <QToolButton>
#include <QVBoxLayout>
#include <cstdio>
#include <cstring>
#include <ctime>
#include <functional>
#include <limits>
#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xlib.h>
// Xlib event macros otherwise replace Qt's scoped event names below.
#undef KeyPress
#undef KeyRelease
#undef FocusIn
#undef FocusOut
#endif

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "Graph assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

namespace {
bool waitFor(const std::function<bool()> &condition, int timeout = 5000)
{
    QElapsedTimer timer;
    timer.start();
    do {
        QApplication::processEvents();
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        if (condition())
            return true;
        QThread::msleep(2);
    } while (timer.elapsed() < timeout);
    return false;
}
void idle(int milliseconds)
{
    QEventLoop loop;
    QTimer::singleShot(milliseconds, &loop, &QEventLoop::quit);
    loop.exec();
}
bool write(const QString &path, const QByteArray &content = "example\n")
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(content) == content.size();
}
qint64 residentKiB(const QByteArray &field = "VmRSS:")
{
#ifdef Q_OS_LINUX
    QFile status("/proc/self/status");
    if (status.open(QIODevice::ReadOnly)) {
        for (const auto &line : status.readAll().split('\n')) {
            if (line.startsWith(field))
                return line.simplified().split(' ').value(1).toLongLong();
        }
    }
#endif
    return 0;
}
QJsonObject page(const QString &root, const QString &directory, int limit, const QString &after = {}, const QString &stamp = {})
{
    return WorkspaceGraphPage::read({{"root", root}, {"directory", directory}, {"limit", limit}, {"after", after}, {"stamp", stamp}, {"hidden", true}});
}
}

static int folderFlightTest()
{
    QTemporaryDir world;
    QDir directory(world.path());
    CHECK(directory.mkpath("portal/empty"));
    WorkspaceFlightView flightWorld({}, QGuiApplication::platformName() != "offscreen");
    flightWorld.resize(900, 660);
    flightWorld.show();
    flightWorld.setWorkspace(world.path(), world.path(), true);
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    const auto doubleClickFolder = [&]() {
        const QPointF center = flightWorld.viewport()->rect().center();
        QMouseEvent press(QEvent::MouseButtonPress, center, center,
            Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(flightWorld.viewport(), &press);
        QMouseEvent release(QEvent::MouseButtonRelease, center, center,
            Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
        QApplication::sendEvent(flightWorld.viewport(), &release);
        QMouseEvent doubleClick(QEvent::MouseButtonDblClick, center, center,
            Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(flightWorld.viewport(), &doubleClick);
        QApplication::sendEvent(flightWorld.viewport(), &release);
    };
    doubleClickFolder();
    CHECK(flightWorld.travelling() && !flightWorld.loading());
    const QVector3D start = flightWorld.cameraPosition();
    idle(100);
    CHECK(flightWorld.cameraPosition().y() > start.y());
    QKeyEvent cancel(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
    QApplication::sendEvent(&flightWorld, &cancel);
    CHECK(!flightWorld.travelling() && flightWorld.directory() == world.path());
    flightWorld.enterDirectory(world.path());
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    doubleClickFolder();
    CHECK(flightWorld.travelling());
    CHECK(waitFor([&]() { return flightWorld.directory() == directory.filePath("portal"); }));
    CHECK(!flightWorld.travelling() && flightWorld.buildingCount() == 1);
    doubleClickFolder();
    CHECK(flightWorld.travelling());
    QKeyEvent skip(QEvent::KeyPress, Qt::Key_Space, Qt::NoModifier);
    QApplication::sendEvent(&flightWorld, &skip);
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    CHECK(flightWorld.directory() == directory.filePath("portal/empty"));
    flightWorld.enterDirectory(world.path());
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    doubleClickFolder();
    CHECK(directory.rmdir("portal/empty") && directory.rmdir("portal"));
    flightWorld.skipTravel();
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    CHECK(flightWorld.directory() == world.path() && flightWorld.worldStatus().contains("Cannot enter"));
    CHECK(!flightWorld.travelling());
    CHECK(directory.mkdir("portal"));
    flightWorld.enterDirectory(world.path());
    CHECK(waitFor([&]() { return !flightWorld.loading(); }));
    doubleClickFolder();
    flightWorld.hide();
    CHECK(!flightWorld.travelling() && !flightWorld.loading());
    CHECK(flightWorld.directory() == world.path());
    std::puts("Folder double-click travel, cancellation, arrival, missing-folder failure and hide cleanup passed.");
    return 0;
}

int main(int argc, char **argv)
{
    if (argc == 2 && std::strcmp(argv[1], "--workspace-graph-page") == 0) {
        // Test-only slow filesystem fixture. Production has no delay or override.
        QThread::msleep(qBound(0, qEnvironmentVariableIntValue("DOCKPIPE_GRAPH_TEST_DELAY_MS"), 2000));
        return WorkspaceGraphPage::run(argc, argv);
    }
    QApplication app(argc, argv);
    if (app.arguments().contains("--folder-flight"))
        return folderFlightTest();
    applyDockpipeLauncherTheme(app);
    if (qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light") {
        QFile shared(":/theme/pipeon.qss"), light(":/theme/pipeon-light.qss");
        CHECK(shared.open(QIODevice::ReadOnly) && light.open(QIODevice::ReadOnly));
        app.setStyleSheet(QString());
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
    }
    QTemporaryDir root;
    CHECK(root.isValid());
    const QString base = root.path();
    if (qEnvironmentVariableIsSet("DOCKPIPE_BILLBOARD_FIXTURE")) {
        QProcess fixture;
        fixture.start(qEnvironmentVariable("DOCKPIPE_BILLBOARD_FIXTURE"), {"--fixture"});
        CHECK(fixture.waitForStarted());
        QByteArray reply;
        CHECK(waitFor([&]() { reply += fixture.readAllStandardOutput(); return reply.contains("WINDOW:") && reply.contains('\n'); }));
        const quint64 id = reply.split('\n').first().mid(7).toULongLong();
        CHECK(id);
        const bool hardware = !qEnvironmentVariableIsSet("DOCKPIPE_BILLBOARD_SOFTWARE");
        QWidget host;
        auto *hostLayout = new QVBoxLayout(&host);
        auto *chrome = new QLabel("Workspace chrome");
        hostLayout->addWidget(chrome);
        auto *alreadyHidden = new QLabel("Must stay hidden");
        hostLayout->addWidget(alreadyHidden);
        alreadyHidden->hide();
        WorkspaceFlightView flight({}, hardware);
        hostLayout->addWidget(&flight);
        host.resize(1280, 800);
        host.show();
        if (hardware)
            CHECK(waitFor([&]() { return flight.hardwareAccelerated(); }));
        flight.setAppBillboards({{id, "Dockpipe isolated app fixture"}});
        idle(1200);
        CHECK(!flight.editorActive());
        flight.findChild<QPushButton *>("flightFullscreen")->click();
        CHECK(waitFor([&]() { return host.isFullScreen(); }));
        CHECK(flight.isVisible() && chrome->isHidden() && alreadyHidden->isHidden());
        QKeyEvent escapeFullscreen(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
        QApplication::sendEvent(&flight, &escapeFullscreen);
        CHECK(waitFor([&]() { return !host.isFullScreen(); }));
        CHECK(chrome->isVisible() && alreadyHidden->isHidden());
        idle(600); // Let the compositor finish restoring and repainting the window.
        const QString capture = qEnvironmentVariable("DOCKPIPE_GRAPH_SCREENSHOT");
        if (!capture.isEmpty())
            flight.screen()->grabWindow(flight.winId()).save(capture + ".billboard.png");
        flight.fire();
        CHECK(!flight.editorActive() && flight.previewPath() == "billboard:0");
        idle(80);
        if (!capture.isEmpty())
            CHECK(flight.screen()->grabWindow(flight.winId()).save(capture + ".hologram.png"));
        flight.fire();
        CHECK(flight.editorActive());
        CHECK(!flight.hardwareAccelerationRequested());
        auto *panel = flight.findChild<QWidget *>("billboardControlPanel");
        CHECK(panel && panel->isVisible());
        CHECK(waitFor([&]() { return flight.findChild<QWidget *>("billboardNativeApp") != nullptr; }));
        idle(1500);
        const QPixmap controlFrame = flight.screen()->grabWindow(flight.winId());
        auto *nativeApp = flight.findChild<QWidget *>("billboardNativeApp");
        const QPoint appCenter = nativeApp->mapTo(&flight, nativeApp->rect().center());
        const QColor appPixel = controlFrame.toImage().pixelColor(appCenter * controlFrame.devicePixelRatio());
        CHECK(appPixel.blue() > 100 && appPixel.red() < 60);
        if (!capture.isEmpty())
            controlFrame.save(capture + ".control.png");
        auto *fullscreen = flight.findChild<QPushButton *>("billboardFullscreen");
        CHECK(fullscreen);
        fullscreen->click();
        CHECK(waitFor([&]() { return host.isFullScreen() && panel->size() == flight.size(); }));
        idle(500);
        CHECK(flight.editorActive() && nativeApp->isVisible() && fixture.state() == QProcess::Running);
        const QPixmap fullscreenFrame = flight.screen()->grabWindow(flight.winId());
        const QPoint fullscreenCenter = nativeApp->mapTo(&flight, nativeApp->rect().center());
        const QColor fullscreenPixel = fullscreenFrame.toImage().pixelColor(fullscreenCenter * fullscreenFrame.devicePixelRatio());
        CHECK(fullscreenPixel.blue() > 100 && fullscreenPixel.red() < 60);
        if (!capture.isEmpty())
            fullscreenFrame.save(capture + ".fullscreen.png");
        fullscreen->click();
        CHECK(waitFor([&]() { return !host.isFullScreen(); }));
        CHECK(flight.editorActive() && nativeApp->isVisible() && chrome->isVisible());
        host.resize(900, 620);
        QApplication::processEvents();
        CHECK(flight.rect().contains(panel->geometry()));
        flight.findChild<QPushButton *>("billboardReleaseControl")->click();
        CHECK(!flight.editorActive());
        if (hardware)
            CHECK(waitFor([&]() { return flight.hardwareAccelerated(); }));
        flight.fire();
        if (!flight.appControlling())
            flight.fire();
        CHECK(flight.editorActive());
        flight.hide();
        CHECK(!flight.editorActive());
        CHECK(!flight.findChild<QWidget *>("billboardControlPanel"));
        CHECK(fixture.state() == QProcess::Running);
        flight.show();
        flight.setCockpitWorld({{"Fixture app", 95, true, false, "app:fixture"}}, false);
        flight.destinationActivated = [&](const QString &path, bool) {
            if (path == "app:fixture")
                flight.controlApp({id, "Dockpipe isolated app fixture"}, false);
        };
        flight.previewApp("app:fixture", {id, "Dockpipe isolated app fixture"});
        idle(700);
        CHECK(flight.previewPath() == "app:fixture" && !flight.editorActive());
        if (!capture.isEmpty())
            CHECK(flight.screen()->grabWindow(flight.winId()).save(capture + ".app-city-hologram.png"));
        flight.fire();
        CHECK(waitFor([&]() { return flight.findChild<QWidget *>("billboardNativeApp") != nullptr; }));
        idle(200);
#ifdef DOCKPIPE_X11_BILLBOARDS
        Display *display = XOpenDisplay(nullptr);
        CHECK(display);
        XReparentWindow(display, id, DefaultRootWindow(display), 100, 100);
        XMapWindow(display, id);
        XFlush(display);
        CHECK(waitFor([&]() { return !flight.editorActive() && !flight.appControlling(); }));
        CHECK(!flight.findChild<QWidget *>("billboardControlPanel"));
        CHECK(flight.previewPath() == "app:fixture");
        CHECK(flight.worldStatus().contains("restored"));
        CHECK(fixture.state() == QProcess::Running);
        if (hardware)
            CHECK(waitFor([&]() { return flight.hardwareAccelerated(); }));
        idle(600);
        if (!capture.isEmpty())
            CHECK(flight.screen()->grabWindow(flight.winId()).save(capture + ".recovered-control.png"));
        XCloseDisplay(display);
#else
        flight.findChild<QPushButton *>("billboardReleaseControl")->click();
#endif
        flight.hide();
        DesktopAppWindow restored({id, "Fixture"});
        QString restoreError;
        CHECK(waitFor([&]() { return !restored.capture(restoreError).isNull(); }));
        fixture.terminate();
        CHECK(fixture.waitForFinished());
        std::puts("Native billboard rendering, shooting, embedded panel, fullscreen entry/exit, chrome restoration, renderer restoration, resize and hide cleanup passed.");
        return 0;
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_GPU_SMOKE")) {
        CHECK(write(root.filePath("README.md")));
        CHECK(QDir().mkdir(root.filePath("source")));
        WorkspaceGraphWidget preview(base, base);
        preview.resize(1180, 780);
        preview.show();
        CHECK(waitFor([&]() { return !preview.busy(); }));
        preview.setSpatial(true);
        auto *view = static_cast<WorkspaceGraphView *>(preview.canvas());
        CHECK(waitFor([&]() { return view->rendererDescription().startsWith("GPU"); }));
        auto *gl = qobject_cast<QOpenGLWidget *>(view->viewport());
        CHECK(gl && gl->isValid() && view->hardwareAccelerated());
        gl->makeCurrent();
        const auto *renderer = gl->context()->functions()->glGetString(GL_RENDERER);
        CHECK(renderer);
        std::printf("Actual viewport renderer: %s\n", renderer);
        gl->doneCurrent();
        preview.selectPath(root.filePath("README.md"));
        preview.findChild<QPushButton *>("graphRotate")->click();
        idle(100);
        // QGraphicsView owns painting through its viewport event filter;
        // QOpenGLWidget::grabFramebuffer would invoke its empty paintGL again.
        const QImage framebuffer = preview.screen()->grabWindow(preview.winId()).toImage();
        CHECK(!framebuffer.isNull());
        CHECK(framebuffer.pixelColor(20, 20) != framebuffer.pixelColor(framebuffer.width() / 2, framebuffer.height() / 2));
        const QString capture = qEnvironmentVariable("DOCKPIPE_GRAPH_SCREENSHOT");
        if (!capture.isEmpty())
            CHECK(framebuffer.save(capture));
        view->setHardwareAcceleration(false);
        CHECK(!view->hardwareAccelerated() && !qobject_cast<QOpenGLWidget *>(view->viewport()));
        view->setHardwareAcceleration(true);
        CHECK(waitFor([&]() { return view->rendererDescription().startsWith("GPU"); }));
        preview.setSpatial(false);
        CHECK(!view->hardwareAccelerated() && !qobject_cast<QOpenGLWidget *>(view->viewport()));
        preview.applyCockpitSettings(CockpitSettings{true});
        preview.openFlight();
        auto *flight = preview.flightView();
        CHECK(flight && waitFor([&]() { return flight->rendererDescription().startsWith("GPU"); }));
        CHECK(waitFor([&]() { return !flight->loading(); }));
        idle(100);
        if (!capture.isEmpty())
            CHECK(preview.screen()->grabWindow(preview.winId()).save(capture + ".flight.png"));
        preview.closeFlight();
        QApplication::processEvents();
        preview.hide();
        QTemporaryDir hudRoot;
        CHECK(write(hudRoot.filePath("telemetry.py"), QByteArray("# Drone telemetry\naltitude = 180\n").repeated(250)));
        ProjectFilesWidget hudFiles;
        hudFiles.applyCockpitSettings(CockpitSettings{true});
        hudFiles.resize(1180, 780);
        hudFiles.show();
        hudFiles.activateWindow();
        CHECK(hudFiles.setRoot(hudRoot.path()));
        hudFiles.openGraph();
        auto *hudGraph = hudFiles.findChild<WorkspaceGraphWidget *>();
        CHECK(waitFor([&]() { return !hudGraph->busy(); }));
        hudGraph->openFlight();
        auto *hudFlight = hudGraph->flightView();
        CHECK(waitFor([&]() { return !hudFlight->loading() && hudFlight->hardwareAccelerated(); }));
        hudFlight->setFocus();
        idle(100);
        hudFlight->fire();
        CHECK(!hudFlight->previewPath().isEmpty() && !hudFiles.flightEditorOpen());
        hudFlight->fire();
        CHECK(waitFor([&]() { return hudFiles.flightEditorOpen(); }));
        CHECK(hudFlight->hardwareAccelerated() && hudFlight->editorActive());
        hudFlight->setHardwareAcceleration(false);
        CHECK(hudFiles.flightEditorOpen() && !hudFlight->hardwareAccelerated());
        hudFlight->setHardwareAcceleration(true);
        CHECK(waitFor([&]() { return hudFlight->rendererDescription().startsWith("GPU"); }));
        auto *hudPanel = hudFiles.findChild<QWidget *>("flightEditorHud");
        CHECK(hudPanel->isAncestorOf(hudFlight->childAt(hudFlight->rect().center())));
        idle(100);
        if (!capture.isEmpty())
            CHECK(hudFiles.screen()->grabWindow(hudFiles.winId()).save(capture + ".hud.png"));
        auto *hudEditor = hudFiles.findChild<QPlainTextEdit *>("fileTextEditor");
        hudEditor->moveCursor(QTextCursor::End);
        hudEditor->insertPlainText("# Shared editor draft\n");
        QKeyEvent saveKey(QEvent::KeyPress, Qt::Key_S, Qt::ControlModifier);
        QApplication::sendEvent(hudEditor, &saveKey);
        CHECK(!hudFiles.hasUnsavedChanges());
        hudFiles.resize(900, 660);
        idle(80);
        if (!capture.isEmpty())
            CHECK(hudFiles.screen()->grabWindow(hudFiles.winId()).save(capture + ".hud-compact.png"));
        QKeyEvent escape(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
        QApplication::sendEvent(hudEditor, &escape);
        CHECK(!hudFiles.flightEditorOpen() && !hudFlight->editorActive());
        hudFiles.closeGraph();
        CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
        std::puts("Hardware viewport, flight HUD composition, Ctrl+S and Escape passed.");
        return 0;
    }
    CHECK(QDir().mkpath(root.filePath("source/nested/deep")));
    CHECK(write(root.filePath("source/nested/deep/hidden-from-root.cpp")));
    CHECK(write(root.filePath("README.md"), "# Workspace\n"));
    CHECK(write(root.filePath("config.yml"), "name: example\nsteps: []\n"));
    CHECK(write(root.filePath(".settings")));
    CHECK(QDir().mkdir(root.filePath(".git")));
    auto first = page(base, base, 2);
    CHECK(!first.contains("error") && first.value("entries").toArray().size() == 2 && first.value("more").toBool());
    CHECK(first.value("entries").toArray().first().toObject().value("name").toString() == ".settings");
    auto second = page(base, base, 2, first.value("after").toString(), first.value("stamp").toString());
    CHECK(!second.contains("error") && second.value("entries").toArray().size() == 2 && !second.value("more").toBool());
    CHECK(!QJsonDocument(first).toJson().contains("hidden-from-root"));
    auto filtered = WorkspaceGraphPage::read({{"root", base}, {"directory", base}, {"limit", 150}, {"hidden", false}});
    CHECK(filtered.value("entries").toArray().size() == 3);
    CHECK(write(root.filePath("new-file.txt")));
    CHECK(page(base, base, 2, first.value("after").toString(), first.value("stamp").toString()).contains("error"));
    CHECK(QFile::remove(root.filePath("new-file.txt")));
    CHECK(page(base, QDir(base).filePath(".."), 2).contains("error"));
    CHECK(page(base, root.filePath(".git"), 2).contains("error"));
    CHECK(page(base, base, 151).contains("error"));
    CHECK(page(base, root.filePath("deleted"), 2).contains("error"));
    CHECK(QDir().mkdir(root.filePath("empty")));
    CHECK(page(base, root.filePath("empty"), 5).value("entries").toArray().isEmpty());
#ifdef Q_OS_UNIX
    CHECK(QFile::link(base, root.filePath("cycle")));
    CHECK(QFile::link(root.filePath("missing-target"), root.filePath("broken")));
    CHECK(page(base, root.filePath("cycle"), 5).contains("error"));
    bool sawCycle = false, sawBroken = false;
    for (const auto &entry : page(base, base, 150).value("entries").toArray()) {
        const auto object = entry.toObject();
        sawCycle |= object.value("name") == "cycle" && object.value("kind") == "link";
        sawBroken |= object.value("name") == "broken" && object.value("kind") == "link";
        if (object.value("kind") != "file")
            CHECK(!object.contains("size_bytes"));
    }
    CHECK(sawCycle && sawBroken);
    CHECK(QDir().mkdir(root.filePath("denied")));
    CHECK(QFile::setPermissions(root.filePath("denied"), {}));
    const auto denied = page(base, root.filePath("denied"), 5);
    CHECK(QFile::setPermissions(root.filePath("denied"), QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    CHECK(denied.contains("error"));
#endif
    QString deep = root.filePath("deep");
    for (int depth = 0; depth < 80; ++depth)
        deep += "/d";
    CHECK(QDir().mkpath(deep));
    CHECK(page(base, deep, 5).value("entries").toArray().isEmpty());
    const QString unicodeName = QString::fromUtf8("設計 — résumé.md");
    CHECK(write(root.filePath(unicodeName)));
    const QString longName(180, 'x');
    CHECK(write(root.filePath(longName + ".txt")));

    const QMap<QString, qint64> fileSizes{{"empty.bin", 0}, {"small.bin", 1024},
        {"medium.bin", 1024 * 1024}, {"large.bin", 32 * 1024 * 1024}};
    for (auto it = fileSizes.cbegin(); it != fileSizes.cend(); ++it) {
        QFile sizedFile(root.filePath(it.key()));
        CHECK(sizedFile.open(QIODevice::WriteOnly) && sizedFile.resize(it.value()));
    }
    for (const auto &entry : page(base, base, 150).value("entries").toArray()) {
        const auto object = entry.toObject();
        const QString name = object.value("name").toString();
        if (fileSizes.contains(name))
            CHECK(object.value("size_bytes").toString().toLongLong() == fileSizes.value(name));
    }

    // No graph or helper exists while the normal editor is shown.
    ProjectFilesWidget files;
    files.resize(1180, 780);
    CHECK(files.setRoot(base));
    files.show();
    idle(100);
    CHECK(!files.findChild<WorkspaceGraphWidget *>() && !WorkspaceGraphReader::helperActive());
    const qint64 baseline = residentKiB();
    QElapsedTimer cold;
    cold.start();
    auto *graphButton = files.findChild<QToolButton *>("openWorkspaceGraph");
    CHECK(graphButton);
    graphButton->click();
    QPointer<WorkspaceGraphWidget> graph = files.findChild<WorkspaceGraphWidget *>();
    CHECK(graph && waitFor([&]() { return !graph->busy(); }));
    QMap<QString, qreal> fileHeights;
    for (auto *item : graph->canvas()->scene()->items()) {
        auto *chip = dynamic_cast<WorkspaceGraphNode *>(item);
        if (chip)
            fileHeights.insert(QFileInfo(chip->data(0).toString()).fileName(), chip->chipHeight());
    }
    CHECK(fileHeights.value("empty.bin") > 0);
    CHECK(fileHeights.value("empty.bin") < fileHeights.value("small.bin"));
    CHECK(fileHeights.value("small.bin") < fileHeights.value("medium.bin"));
    CHECK(fileHeights.value("medium.bin") < fileHeights.value("large.bin"));
    WorkspaceGraphNode maximumFile("maximum.bin", "file", std::numeric_limits<qint64>::max());
    CHECK(maximumFile.chipHeight() == fileHeights.value("large.bin"));
    const qint64 coldMs = cold.elapsed();
    CHECK(graph->hasPath(root.filePath("source")) && !graph->hasPath(root.filePath("source/nested")));
    QGraphicsItem *rootItem = nullptr;
    for (auto *item : graph->canvas()->scene()->items()) {
        if (item->data(0).toString() == base)
            rootItem = item;
    }
    CHECK(rootItem);
    const int loadedBeforeToggle = graph->nodeCount();
    const QPointF customPosition(35, 57);
    rootItem->setPos(customPosition);
    graph->findChild<QPushButton *>("graph3D")->click();
    CHECK(graph->spatial() && graph->nodeCount() == loadedBeforeToggle);
    if (QGuiApplication::platformName() == "offscreen")
        CHECK(!static_cast<WorkspaceGraphView *>(graph->canvas())->hardwareAccelerated());
    CHECK(!graph->busy() && !WorkspaceGraphReader::helperActive());
    CHECK(!(rootItem->flags() & QGraphicsItem::ItemIsMovable));
    const QPointF projected = rootItem->pos();
    graph->findChild<QPushButton *>("graphRotate")->click();
    CHECK(rootItem->pos() != projected);
    const QPointF beforeDrag = rootItem->pos();
    QMouseEvent press(QEvent::MouseButtonPress, QPointF(120, 120), Qt::LeftButton, Qt::LeftButton, Qt::ShiftModifier);
    QMouseEvent move(QEvent::MouseMove, QPointF(180, 145), Qt::NoButton, Qt::LeftButton, Qt::ShiftModifier);
    QMouseEvent release(QEvent::MouseButtonRelease, QPointF(180, 145), Qt::LeftButton, Qt::NoButton, Qt::ShiftModifier);
    QApplication::sendEvent(graph->canvas()->viewport(), &press);
    QApplication::sendEvent(graph->canvas()->viewport(), &move);
    QApplication::sendEvent(graph->canvas()->viewport(), &release);
    CHECK(rootItem->pos() != beforeDrag && !graph->busy());
    const QPointF beforeMiddleDrag = rootItem->pos();
    QMouseEvent middlePress(QEvent::MouseButtonPress, QPointF(120, 120), Qt::MiddleButton, Qt::MiddleButton, Qt::NoModifier);
    QMouseEvent middleMove(QEvent::MouseMove, QPointF(190, 150), Qt::NoButton, Qt::MiddleButton, Qt::NoModifier);
    QMouseEvent middleRelease(QEvent::MouseButtonRelease, QPointF(190, 150), Qt::MiddleButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(graph->canvas()->viewport(), &middlePress);
    QApplication::sendEvent(graph->canvas()->viewport(), &middleMove);
    QApplication::sendEvent(graph->canvas()->viewport(), &middleRelease);
    CHECK(rootItem->pos() != beforeMiddleDrag && !graph->busy());
    const QPointF afterMiddleDrag = rootItem->pos();
    QMouseEvent hover(QEvent::MouseMove, QPointF(230, 170), Qt::NoButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(graph->canvas()->viewport(), &hover);
    CHECK(rootItem->pos() == afterMiddleDrag);
    // A higher-depth object must not obscure the selected label. Deselecting
    // restores ordinary depth ordering, without raising the whole chip.
    auto *rootChip = dynamic_cast<WorkspaceGraphNode *>(rootItem);
    CHECK(rootChip);
    graph->selectPath(base);
    graph->canvas()->resetTransform();
    graph->canvas()->centerOn(rootItem);
    const QRectF labelArea = rootChip->mapRectToScene(rootChip->cityLabelRect());
    auto *occluder = graph->canvas()->scene()->addRect(labelArea.adjusted(-4, -4, 4, 4), QPen(Qt::red), QBrush(Qt::red));
    occluder->setZValue(10000000);
    const QPoint labelPixel = graph->canvas()->mapFromScene(labelArea.topLeft()) + QPoint(4, 22);
    const bool darkLabel = graph->canvas()->palette().color(QPalette::Base).lightness() < 128;
    QImage selectedImage = graph->canvas()->viewport()->grab().toImage();
    const qreal pixelRatio = selectedImage.devicePixelRatio();
    const QPoint physicalPixel(qRound(labelPixel.x() * pixelRatio), qRound(labelPixel.y() * pixelRatio));
    CHECK(selectedImage.pixelColor(physicalPixel) == QColor(darkLabel ? "#14202b" : "#ffffff"));
    graph->canvas()->scene()->clearSelection();
    CHECK(graph->canvas()->viewport()->grab().toImage().pixelColor(physicalPixel) == QColor(Qt::red));
    delete occluder;
    graph->selectPath(base);
    graph->findChild<QPushButton *>("graph2D")->click();
    CHECK(!graph->spatial() && rootItem->pos() == customPosition);
    CHECK(rootItem->flags() & QGraphicsItem::ItemIsMovable);
    QApplication::sendEvent(graph->canvas()->viewport(), &middlePress);
    QApplication::sendEvent(graph->canvas()->viewport(), &middleMove);
    QApplication::sendEvent(graph->canvas()->viewport(), &middleRelease);
    CHECK(rootItem->pos() == customPosition);
    const int nodesBeforeFlight = graph->nodeCount();
    graph->openFlight();
    CHECK(!graph->flightView());
    CHECK(!graph->findChild<QAction *>("openGraphFlight")->isVisible());
    graph->applyCockpitSettings(CockpitSettings{true});
    CHECK(graph->findChild<QAction *>("openGraphFlight")->isVisible());
    graph->openFlight();
    auto *flight = graph->flightView();
    CHECK(flight && waitFor([&]() { return !flight->loading(); }));
    CHECK(!flight->moving() && !WorkspaceGraphReader::helperActive());
    const Qt::WindowStates previousState = graph->window()->windowState();
    QKeyEvent enterFullscreen(QEvent::KeyPress, Qt::Key_F11, Qt::NoModifier);
    QApplication::sendEvent(flight, &enterFullscreen);
    CHECK(flight->flightFullscreen() && graph->window()->isFullScreen());
    QKeyEvent leaveFullscreen(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
    QApplication::sendEvent(flight, &leaveFullscreen);
    CHECK(graph->flightView() == flight && !flight->flightFullscreen());
    CHECK(graph->window()->windowState() == previousState);
    const QVector3D flightStart = flight->cameraPosition();
    QKeyEvent flyForward(QEvent::KeyPress, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &flyForward);
    idle(120);
    CHECK(flight->moving() && flight->cameraPosition().z() > flightStart.z());
    QKeyEvent releaseForward(QEvent::KeyRelease, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(flight, &releaseForward);
    CHECK(!flight->moving());
    const auto stoppedPosition = flight->cameraPosition();
    idle(70);
    CHECK(flight->cameraPosition() == stoppedPosition);
    QApplication::sendEvent(flight, &flyForward);
    flight->hide();
    CHECK(!flight->moving());
    flight->show();
    QKeyEvent exitFlight(QEvent::KeyPress, Qt::Key_Escape, Qt::NoModifier);
    QApplication::sendEvent(flight, &exitFlight);
    CHECK(!graph->flightView() && graph->nodeCount() == nodesBeforeFlight);
    graph->setSpatial(false);
    CHECK(rootItem->pos() == customPosition);
    const QPointF rootPosition = rootItem->pos();
    cold.restart();
    graph->expand(root.filePath("source"));
    CHECK(waitFor([&]() { return !graph->busy(); }));
    const qint64 expandMs = cold.elapsed();
    CHECK(rootItem->pos() == rootPosition);
    CHECK(graph->hasPath(root.filePath("source/nested")) && !graph->hasPath(root.filePath("source/nested/deep")));
    graph->collapse(root.filePath("source"));
    CHECK(!graph->hasPath(root.filePath("source/nested")));
    CHECK(graph->canvas()->scene()->items().size() == graph->nodeCount() * 2 - 1);
    emit graph->openFileRequested(root.filePath("README.md"));
    CHECK(waitFor([&]() { return graph.isNull(); }));
    CHECK(files.currentPath() == root.filePath("README.md"));
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    editor->insertPlainText("Unsaved draft\n");
    files.openGraph();
    files.closeGraph();
    CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
    CHECK(files.hasUnsavedChanges() && editor->toPlainText().contains("Unsaved draft"));
    editor->document()->setModified(false);

    {
        WorkspaceFlightView collision({{"tower", 180, true, false}}, false);
        collision.resize(900, 660);
        collision.show();
        idle(30);
        QKeyEvent boost(QEvent::KeyPress, Qt::Key_Shift, Qt::ShiftModifier);
        QApplication::sendEvent(&collision, &boost);
        QApplication::sendEvent(&collision, &flyForward);
        idle(1250);
        CHECK(collision.cameraPosition().z() < -70 && collision.cameraPosition().z() > -140);
        QFocusEvent lostFocus(QEvent::FocusOut);
        QApplication::sendEvent(&collision, &lostFocus);
        CHECK(!collision.moving());
    }

    // Directory worlds replace their page, and entering a hangar never reads descendants eagerly.
    CHECK(folderFlightTest() == 0);
    {
        QTemporaryDir world;
        QDir directory(world.path());
        CHECK(directory.mkpath("portal/empty"));
        WorkspaceFlightView flightWorld({}, false);
        flightWorld.resize(900, 660);
        flightWorld.show();
        flightWorld.setWorkspace(world.path(), world.path(), true);
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.buildingCount() == 1 && flightWorld.directory() == world.path());
        QKeyEvent boost(QEvent::KeyPress, Qt::Key_Shift, Qt::ShiftModifier);
        QApplication::sendEvent(&flightWorld, &boost);
        QApplication::sendEvent(&flightWorld, &flyForward);
        CHECK(waitFor([&]() { return flightWorld.directory() == directory.filePath("portal"); }));
        CHECK(flightWorld.buildingCount() == 1 && !flightWorld.moving());
        flightWorld.enterDirectory(directory.filePath("portal/empty"));
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.buildingCount() == 0 && flightWorld.worldStatus().contains("Empty"));
        flightWorld.goUp();
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.directory() == directory.filePath("portal"));
        flightWorld.enterDirectory(directory.filePath("missing"));
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.worldStatus().contains("Cannot enter") && flightWorld.buildingCount() == 1);
        flightWorld.enterDirectory(QDir::tempPath());
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.worldStatus().contains("Cannot enter") && flightWorld.directory() == directory.filePath("portal"));
        for (int file = 0; file < 155; ++file)
            CHECK(write(directory.filePath(QString("page-%1.txt").arg(file, 3, 10, QChar('0')))));
        flightWorld.enterDirectory(world.path());
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(flightWorld.hasNextPage() && flightWorld.buildingCount() == 150);
        flightWorld.nextPage();
        CHECK(waitFor([&]() { return !flightWorld.loading(); }));
        CHECK(!flightWorld.hasNextPage() && flightWorld.buildingCount() == 6);
        flightWorld.enterDirectory(world.path());
        flightWorld.hide();
        CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
        CHECK(!flightWorld.moving());
    }
    {
        WorkspaceGraphWidget navigation(base, base);
        navigation.resize(900, 660);
        navigation.show();
        CHECK(waitFor([&]() { return !navigation.busy(); }));
        navigation.setSpatial(true);
        QString opened;
        QObject::connect(&navigation, &WorkspaceGraphWidget::openFileRequested, &navigation,
            [&](const QString &path) { opened = path; });
        navigation.activatePath(root.filePath("README.md"));
        CHECK(opened.isEmpty());
        CHECK(waitFor([&]() { return !opened.isEmpty(); }));
        CHECK(opened == root.filePath("README.md"));
        navigation.activatePath(root.filePath("source"));
        CHECK(waitFor([&]() { return !navigation.hasPath(base) && !navigation.busy(); }));
        CHECK(navigation.hasPath(root.filePath("source")));
        navigation.findChild<QPushButton *>("graphParent")->click();
        CHECK(waitFor([&]() { return navigation.hasPath(base) && !navigation.busy(); }));
        opened.clear();
        navigation.activatePath(root.filePath("README.md"));
        navigation.hide();
        idle(700);
        CHECK(opened.isEmpty()); // Hiding cancels the pending open, not just the paint loop.
    }

    // Reticle hits respect occluding folders and links; shots never modify files.
    {
        QString acquired;
        WorkspaceFlightView target({{"file", 120, false, false, root.filePath("README.md")}}, false);
        target.resize(900, 660);
        target.show();
        target.fileTargeted = [&](const QString &path) { acquired = path; };
        target.fire();
        CHECK(acquired.isEmpty() && target.previewPath() == root.filePath("README.md"));
        target.fire();
        CHECK(waitFor([&]() { return !acquired.isEmpty(); }));
        CHECK(acquired == root.filePath("README.md"));
        acquired.clear();
        target.fire();
        target.hide();
        idle(170);
        CHECK(acquired.isEmpty());
        WorkspaceFlightView blocker({{"side", 100, false, false}, {"folder", 85, true, false, base},
            {"side", 100, false, false}, {"side", 100, false, false},
            {"behind", 180, false, false, root.filePath("README.md")}}, false);
        blocker.fileTargeted = [&](const QString &path) { acquired = path; };
        blocker.fire();
        idle(170);
        CHECK(acquired.isEmpty() && blocker.worldStatus().contains("Folder"));
        WorkspaceFlightView linked({{"link", 120, false, true, root.filePath("README.md")}}, false);
        linked.fileTargeted = [&](const QString &path) { acquired = path; };
        linked.fire();
        idle(170);
        CHECK(acquired.isEmpty() && linked.worldStatus().contains("read-only"));
        WorkspaceFlightView miss({}, false);
        miss.fire();
        CHECK(miss.worldStatus().contains("No file"));
    }
    {
        QTemporaryDir hudRoot;
        const QString scriptPath = hudRoot.filePath("script.py");
        CHECK(write(scriptPath, QByteArray("# Flight editor fixture\nvalue = 42\n").repeated(250)));
        ProjectFilesWidget hudFiles;
        hudFiles.applyCockpitSettings(CockpitSettings{true});
        hudFiles.resize(1180, 780);
        hudFiles.show();
        hudFiles.activateWindow();
        CHECK(hudFiles.setRoot(hudRoot.path()));
        auto *originalEditor = hudFiles.findChild<QPlainTextEdit *>("fileTextEditor");
        hudFiles.openGraph();
        auto *hudGraph = hudFiles.findChild<WorkspaceGraphWidget *>();
        CHECK(hudGraph && waitFor([&]() { return !hudGraph->busy(); }));
        hudGraph->openFlight();
        auto *hudFlight = hudGraph->flightView();
        CHECK(hudFlight && waitFor([&]() { return !hudFlight->loading(); }));
        const QVector3D camera = hudFlight->cameraPosition();
        const QPointF target = hudFlight->viewport()->rect().center();
        QMouseEvent shot(QEvent::MouseButtonPress, target, target,
            Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(hudFlight->viewport(), &shot);
        CHECK(!hudFiles.flightEditorOpen() && hudFlight->previewPath() == scriptPath);
        QApplication::sendEvent(hudFlight->viewport(), &shot);
        CHECK(waitFor([&]() { return hudFiles.flightEditorOpen(); }));
        auto *hud = hudFiles.findChild<QWidget *>("flightEditorHud");
        CHECK(hud && hud->isAncestorOf(originalEditor) && hudFiles.currentPath() == scriptPath);
        CHECK(hudFlight->editorActive() && !hudFlight->moving());
        QApplication::sendEvent(hudFlight, &flyForward);
        idle(80);
        CHECK(hudFlight->cameraPosition() == camera && !hudFlight->moving());
        originalEditor->moveCursor(QTextCursor::End);
        originalEditor->insertPlainText("# Saved from the cockpit\n");
        const QString screenshot = qEnvironmentVariable("DOCKPIPE_GRAPH_SCREENSHOT");
        if (!screenshot.isEmpty())
            CHECK(hudFiles.grab().save(screenshot + ".hud.png"));
        hudFiles.findChild<QPushButton *>("editorSave")->click();
        QFile saved(scriptPath);
        CHECK(saved.open(QIODevice::ReadOnly) && saved.readAll().endsWith("# Saved from the cockpit\n"));
        saved.close();
        originalEditor->insertPlainText("# Keep this draft\n");
        CHECK(write(scriptPath, "# External disk edit\n"));
        CHECK(!hudFiles.saveFile() && hudFiles.hasUnsavedChanges());
        hudFiles.findChild<QPushButton *>("closeFlightEditor")->click();
        CHECK(!hudFiles.flightEditorOpen() && !hudFlight->editorActive());
        CHECK(hudFiles.hasUnsavedChanges() && originalEditor->toPlainText().contains("Keep this draft"));
        CHECK(originalEditor->document()->isUndoAvailable());
        CHECK(write(hudRoot.filePath("other.py"), "# another file\n"));
        QTimer::singleShot(0, []() {
            for (auto *widget : QApplication::topLevelWidgets()) {
                if (auto *dialog = qobject_cast<QMessageBox *>(widget))
                    dialog->done(QMessageBox::Cancel);
            }
        });
        emit hudGraph->flightFileRequested(hudRoot.filePath("other.py"));
        CHECK(!hudFiles.flightEditorOpen() && hudFiles.currentPath() == scriptPath && hudFiles.hasUnsavedChanges());
        emit hudGraph->flightFileRequested(scriptPath);
        CHECK(hudFiles.flightEditorOpen());
        hudFiles.applyCockpitSettings(CockpitSettings{});
        CHECK(!hudGraph->flightView());
        CHECK(!hudFiles.flightEditorOpen() && hudFiles.hasUnsavedChanges());
        hudFiles.applyCockpitSettings(CockpitSettings{true});
        hudFiles.closeGraph();
        CHECK(waitFor([&]() { return !hudFiles.findChild<WorkspaceGraphWidget *>(); }));
        CHECK(hudFiles.findChild<QPlainTextEdit *>("fileTextEditor") == originalEditor && originalEditor->isVisible());
        CHECK(originalEditor->toPlainText().contains("Keep this draft"));
        originalEditor->document()->setModified(false);
        CHECK(hudFiles.reloadFile());
        CHECK(originalEditor->toPlainText() == "# External disk edit\n");

        // The same mount also preserves the workflow's shared YAML document.
        QTextDocument workflow;
        workflow.setDocumentLayout(new QPlainTextDocumentLayout(&workflow));
        workflow.setPlainText("name: shared-workflow\n");
        hudFiles.setWorkflowDocument(scriptPath, &workflow);
        hudFiles.openGraph();
        hudGraph = hudFiles.findChild<WorkspaceGraphWidget *>();
        CHECK(waitFor([&]() { return !hudGraph->busy(); }));
        hudGraph->openFlight();
        CHECK(waitFor([&]() { return !hudGraph->flightView()->loading(); }));
        CHECK(write(hudRoot.filePath("binary.dat"), QByteArray(8192, '\0')));
        emit hudGraph->flightFileRequested(hudRoot.filePath("binary.dat"));
        CHECK(!hudFiles.flightEditorOpen() && !hudGraph->flightView()->editorActive());
        CHECK(write(hudRoot.filePath("oversize.txt"), QByteArray(1024 * 1024 + 1, 'x')));
        emit hudGraph->flightFileRequested(hudRoot.filePath("oversize.txt"));
        CHECK(!hudFiles.flightEditorOpen() && !hudGraph->flightView()->editorActive());
        // Switch away first so openFile selects the shared workflow document.
        CHECK(hudFiles.openFile(hudRoot.filePath("other.py")));
        emit hudGraph->flightFileRequested(scriptPath);
        auto *sharedEditor = hudFiles.findChild<QPlainTextEdit *>("workflowFileTextEditor");
        CHECK(hudFiles.flightEditorOpen() && sharedEditor->document() == &workflow && sharedEditor->isVisible());
        bool workflowSave = false;
        QObject::connect(&hudFiles, &ProjectFilesWidget::workflowSaveRequested, &hudFiles, [&]() { workflowSave = true; });
        CHECK(hudFiles.saveFile() && workflowSave);
        hudFiles.hide();
        CHECK(!hudFiles.flightEditorOpen() && sharedEditor->document() == &workflow);
        CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
    }

    // Small limits make pagination and narrowing visible and deterministic.
    {
        WorkspaceGraphWidget limited(base, base, nullptr, {2, 5});
        limited.resize(1180, 780);
        limited.show();
        CHECK(waitFor([&]() { return !limited.busy(); }));
        CHECK(limited.nodeCount() == 3 && limited.hasMore(base));
        limited.loadMore(base);
        CHECK(waitFor([&]() { return !limited.busy(); }));
        CHECK(limited.nodeCount() == 5 && limited.hasMore(base));
        limited.loadMore(base);
        CHECK(limited.statusText().contains("limit", Qt::CaseInsensitive));
        limited.collapse(base);
        CHECK(limited.nodeCount() == 1 && limited.canvas()->scene()->items().size() == 1);
        limited.expand(base);
        CHECK(waitFor([&]() { return !limited.busy(); }));
        CHECK(limited.nodeCount() == 3);
    }
    CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
    // Deterministic cancellation, rapid navigation and directory deletion while a request is pending.
    qputenv("DOCKPIPE_GRAPH_TEST_DELAY_MS", "1200");
    {
        WorkspaceGraphWidget slow(base, base);
        slow.navigate(root.filePath("empty"));
        slow.navigate(root.filePath("source"));
        CHECK(waitFor([&]() { return !slow.busy(); }));
        CHECK(slow.hasPath(root.filePath("source/nested")) && !slow.hasPath(root.filePath("README.md")));
        slow.navigate(root.filePath("empty"));
        CHECK(QDir().rmdir(root.filePath("empty")));
        CHECK(waitFor([&]() { return !slow.busy(); }));
        CHECK(slow.statusText().contains("directory", Qt::CaseInsensitive) || slow.statusText().contains("workspace"));
        slow.navigate(base);
        slow.findChild<QPushButton *>("graphCancelRead")->click();
        CHECK(waitFor([&]() { return !slow.busy(); }));
        CHECK(slow.nodeCount() == 1 && slow.statusText().contains("cancelled"));
    }
    CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
    {
        WorkspaceGraphReader deadline(nullptr, 30);
        bool timedOut = false;
        QObject::connect(&deadline, &WorkspaceGraphReader::completed, &app, [&](quint64 token, const QJsonObject &result) {
            timedOut = token == 41 && result.value("error").toString().contains("timed out");
        });
        deadline.request(41, {{"root", base}, {"directory", base}, {"limit", 2}});
        CHECK(waitFor([&]() { return timedOut; }, 1000));
        CHECK(waitFor([&]() { return !deadline.busy(); }, 1000));
    }
    qunsetenv("DOCKPIPE_GRAPH_TEST_DELAY_MS");
    CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));

    CHECK(QDir().mkdir(root.filePath("large")));
    for (int entry = 0; entry < 5000; ++entry)
        CHECK(write(root.filePath(QString("large/file-%1.txt").arg(entry, 5, 10, QChar('0')))));
    qint64 peak = baseline;
    qint64 layoutUs = 0;
    qint64 largeMs = 0;
    qint64 spatialOrbitUs = 0;
    qint64 spatialPaintUs = 0;
    qint64 flightPaintUs = 0;
    double idleCpu = 0;
    {
        cold.restart();
        WorkspaceGraphWidget large(base, root.filePath("large"));
        large.resize(1180, 780);
        large.show();
        CHECK(waitFor([&]() { return !large.busy(); }));
        largeMs = cold.elapsed();
        CHECK(large.nodeCount() == 151 && large.hasMore(root.filePath("large")));
        layoutUs = large.lastLayoutMicroseconds();
        for (int pageNumber = 0; pageNumber < 3; ++pageNumber) {
            large.loadMore(root.filePath("large"));
            CHECK(waitFor([&]() { return !large.busy(); }));
            layoutUs = qMax(layoutUs, large.lastLayoutMicroseconds());
        }
        CHECK(large.nodeCount() == 500 && large.hasMore(root.filePath("large")));
        large.setSpatial(true);
        QElapsedTimer orbitTimer;
        orbitTimer.start();
        for (int turn = 0; turn < 12; ++turn)
            large.findChild<QPushButton *>("graphRotate")->click();
        spatialOrbitUs = orbitTimer.nsecsElapsed() / 12000;
        CHECK(large.nodeCount() == 500 && !WorkspaceGraphReader::helperActive());
        // Include a complete, zoomed-out city render in the local receipt.
        large.canvas()->fitInView(large.canvas()->scene()->itemsBoundingRect(), Qt::KeepAspectRatio);
        orbitTimer.restart();
        CHECK(!large.canvas()->viewport()->grab().isNull());
        spatialPaintUs = orbitTimer.nsecsElapsed() / 1000;
        peak = qMax(peak, residentKiB("VmHWM:"));
        idle(150);
        const auto cpu = std::clock();
        idle(1000);
        idleCpu = double(std::clock() - cpu) / CLOCKS_PER_SEC;
        CHECK(idleCpu < 0.25); // No perpetual simulation; allows instrumented/slow test hosts.
        large.applyCockpitSettings(CockpitSettings{true});
        large.openFlight();
        CHECK(large.flightView() && waitFor([&]() { return !large.flightView()->loading(); }));
        CHECK(!large.flightView()->moving());
        orbitTimer.restart();
        CHECK(!large.flightView()->viewport()->grab().isNull());
        flightPaintUs = orbitTimer.nsecsElapsed() / 1000;
        CHECK(!WorkspaceGraphReader::helperActive());
        large.closeFlight();
        large.collapse(root.filePath("large"));
        CHECK(large.nodeCount() == 1);
    }
    CHECK(waitFor([]() { return !WorkspaceGraphReader::helperActive(); }));
    const qint64 retainedBeforeChurn = residentKiB();
    for (int iteration = 0; iteration < 12; ++iteration) {
        files.openGraph();
        QPointer<WorkspaceGraphWidget> opened = files.findChild<WorkspaceGraphWidget *>();
        CHECK(opened);
        CHECK(waitFor([&]() { return !opened->busy(); }));
        files.closeGraph();
        CHECK(waitFor([&]() { return opened.isNull() && !WorkspaceGraphReader::helperActive(); }));
    }
    const qint64 retained = residentKiB();
#if !defined(__SANITIZE_ADDRESS__)
    CHECK(retained <= retainedBeforeChurn + 8192); // Normal allocator caches may remain; graph objects must not accumulate.
#else
    // ASan deliberately quarantines freed allocations. Object/helper destruction
    // is checked above; measure retained RSS with the ordinary executable.
    std::puts("ASan allocation quarantine: RSS stability is checked in the normal build.");
#endif
    qputenv("DOCKPIPE_GRAPH_TEST_DELAY_MS", "1200");
    files.openGraph();
    QPointer<WorkspaceGraphWidget> hiddenGraph = files.findChild<WorkspaceGraphWidget *>();
    files.hide();
    CHECK(waitFor([&]() { return hiddenGraph.isNull() && !WorkspaceGraphReader::helperActive(); }));
    qunsetenv("DOCKPIPE_GRAPH_TEST_DELAY_MS");
    files.show();
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_GRAPH_SCREENSHOT");
    if (!screenshot.isEmpty()) {
        files.openGraph();
        graph = files.findChild<WorkspaceGraphWidget *>();
        CHECK(waitFor([&]() { return !graph->busy(); }));
        graph->expand(root.filePath("source"));
        CHECK(waitFor([&]() { return !graph->busy(); }));
        graph->selectPath(root.filePath("source"));
        QApplication::processEvents();
        files.grab().save(screenshot);
        graph->setSpatial(true);
        QApplication::processEvents();
        files.grab().save(screenshot + ".3d.png");
        files.resize(900, 660);
        QApplication::processEvents();
        files.grab().save(screenshot + ".compact.png");
        graph->openFlight();
        CHECK(waitFor([&]() { return !graph->flightView()->loading(); }));
        files.grab().save(screenshot + ".flight.png");
        graph->closeFlight();
        files.closeGraph();
        CHECK(waitFor([&]() { return graph.isNull() && !WorkspaceGraphReader::helperActive(); }));
    }
    QJsonObject metrics{{"cold_open_ms", coldMs}, {"expansion_ms", expandMs}, {"large_5000_first_page_ms", largeMs},
        {"spatial_500_node_orbit_us", spatialOrbitUs},
        {"spatial_500_node_paint_us", spatialPaintUs},
        {"flight_150_node_page_paint_us", flightPaintUs},
        {"max_page_layout_us", layoutUs}, {"baseline_rss_kib", baseline}, {"peak_parent_rss_kib", peak},
        {"retained_rss_kib", retained}, {"retained_before_12_cycles_kib", retainedBeforeChurn}, {"idle_cpu_seconds_over_1s", idleCpu}};
    std::puts(QJsonDocument(metrics).toJson(QJsonDocument::Compact).constData());
    std::puts("Workspace graph lazy pages, limits, cancellation, scope, links, failures, editor routing and cleanup passed.");
    return 0;
}
