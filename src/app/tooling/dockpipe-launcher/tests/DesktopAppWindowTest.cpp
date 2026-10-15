#include "DesktopAppWindow.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QLineEdit>
#include <QLabel>
#include <QProcess>
#include <QPushButton>
#include <QThread>
#include <QTimer>
#include <QVBoxLayout>
#include <cstdio>
#include <functional>
#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xlib.h>
#include <X11/keysym.h>
#endif

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "App window assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

namespace {
bool waitFor(const std::function<bool()> &condition)
{
    QElapsedTimer timer;
    timer.start();
    do {
        QApplication::processEvents();
        if (condition())
            return true;
        QThread::msleep(10);
    } while (timer.elapsed() < 5000);
    return false;
}
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    if (!DesktopAppWindow::unavailableReason().isEmpty())
        return 77;
#ifdef DOCKPIPE_X11_BILLBOARDS
    if (app.arguments().contains("--fixture")) {
        QWidget window;
        window.setWindowTitle("Dockpipe isolated billboard fixture");
        window.setStyleSheet("QWidget { background: #195a89; color: white; }");
        window.resize(640, 480);
        auto *layout = new QVBoxLayout(&window);
        auto *heading = new QLabel("BILLBOARD TEST APP");
        heading->setStyleSheet("font-size: 24px; font-weight: bold; padding: 20px;");
        layout->addWidget(heading);
        auto *input = new QLineEdit;
        input->setPlaceholderText("Type inside the live app");
        input->setStyleSheet("background: white; color: black; border: 2px solid #51ddbc; padding: 12px;");
        input->setObjectName("testAppInput");
        layout->addWidget(input);
        auto *button = new QPushButton("Click inside the live app");
        layout->addWidget(button);
        layout->addStretch();
        QObject::connect(input, &QLineEdit::textChanged, &window, [](const QString &value) {
            std::printf("TEXT:%s\n", value.toUtf8().constData());
            std::fflush(stdout);
        });
        window.show();
        input->setFocus();
        std::printf("WINDOW:%llu\n", static_cast<unsigned long long>(window.winId()));
        std::fflush(stdout);
        QTimer::singleShot(30000, &app, &QApplication::quit);
        return app.exec();
    }
    if (app.arguments().contains("--embed-fixture")) {
        const quint64 id = app.arguments().last().toULongLong();
        QWidget window;
        window.resize(800, 600);
        auto *layout = new QVBoxLayout(&window);
        DesktopAppWindow selected({id, "Fixture"});
        QString error;
        CHECK(selected.prepareControl(error));
        CHECK(waitFor([&]() { return selected.controlReady(); }));
        auto *container = selected.takeControl(&window, error);
        CHECK(container);
        layout->addWidget(container);
        window.show();
        std::printf("EMBEDDED\n");
        std::fflush(stdout);
        QTimer::singleShot(15000, &app, &QApplication::quit);
        const int result = app.exec();
        selected.releaseControl();
        return result;
    }

    QProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(qEnvironmentVariable("DOCKPIPE_NATIVE_TEST_FIXTURE", app.applicationFilePath()), {"--fixture"});
    CHECK(fixture.waitForStarted());
    QByteArray output;
    CHECK(waitFor([&]() { output += fixture.readAllStandardOutput(); return output.contains("WINDOW:") && output.contains('\n'); }));
    const quint64 id = output.split('\n').first().mid(7).toULongLong();
    CHECK(id);
    DesktopAppWindow selected({id, "Fixture"});
    CHECK(selected.alive());
    QString error;
    QImage preview;
    CHECK(waitFor([&]() { preview = selected.capture(error); return !preview.isNull() && preview.pixelColor(preview.width() / 2, preview.height() - 5).blue() > preview.pixelColor(preview.width() / 2, preview.height() - 5).red(); }));
    CHECK(preview.size() == QSize(1280, 720));
    CHECK(preview.pixelColor(preview.width() / 2, preview.height() - 5).blue() > preview.pixelColor(preview.width() / 2, preview.height() - 5).red());

    Display *display = XOpenDisplay(nullptr);
    CHECK(display);
    const auto parentOf = [&](Window window) {
        Window root = 0;
        Window parent = 0;
        Window *children = nullptr;
        unsigned int count = 0;
        XQueryTree(display, window, &root, &parent, &children, &count);
        if (children)
            XFree(children);
        return parent;
    };
    QWidget host;
    host.resize(1000, 700);
    auto *layout = new QVBoxLayout(&host);
    CHECK(selected.prepareControl(error));
    CHECK(waitFor([&]() { return selected.controlReady(); }));
    QWidget *container = selected.takeControl(&host, error);
    CHECK(container);
    layout->addWidget(container);
    host.show();
    CHECK(waitFor([&]() { return parentOf(id) != DefaultRootWindow(display); }));
    const auto embeddedParent = parentOf(id);
    Window ancestor = embeddedParent;
    for (int depth = 0; depth < 10 && ancestor && ancestor != host.winId() && ancestor != DefaultRootWindow(display); ++depth)
        ancestor = parentOf(ancestor);
    CHECK(ancestor == host.winId());
    XWindowAttributes embeddedAttributes{};
    CHECK(waitFor([&]() { return XGetWindowAttributes(display, id, &embeddedAttributes)
        && embeddedAttributes.map_state == IsViewable; }));
    CHECK(waitFor([&]() { return selected.controlAttached(); }));
    // Some apps restore desktop dimensions after attachment. Correct both the
    // origin and size without moving the app back to the desktop.
    XMoveResizeWindow(display, id, 1500, 900, 1800, 1000);
    XSync(display, False);
    CHECK(selected.syncControlGeometry());
    CHECK(XGetWindowAttributes(display, id, &embeddedAttributes));
    const qreal scale = host.devicePixelRatioF();
    CHECK(embeddedAttributes.width == qRound(container->width() * scale));
    CHECK(embeddedAttributes.height == qRound(container->height() * scale));
    int nativeX = 0;
    int nativeY = 0;
    Window child = 0;
    CHECK(XTranslateCoordinates(display, id, host.winId(), 0, 0, &nativeX, &nativeY, &child));
    const QPoint offset = container->mapTo(&host, QPoint());
    CHECK(nativeX == qRound(offset.x() * scale) && nativeY == qRound(offset.y() * scale));
    host.activateWindow();
    container->setFocus();
    CHECK(waitFor([&]() { return selected.focusControl(); }));
    Window focused = 0;
    int revert = 0;
    XGetInputFocus(display, &focused, &revert);
    CHECK(focused == id);
    // Target only our disposable fixture. This is not a production input API.
    XKeyEvent key{};
    key.display = display;
    key.window = id;
    key.root = DefaultRootWindow(display);
    key.type = KeyPress;
    key.keycode = XKeysymToKeycode(display, XK_a);
    key.same_screen = True;
    XSendEvent(display, id, False, KeyPressMask, reinterpret_cast<XEvent *>(&key));
    XFlush(display);
    CHECK(waitFor([&]() { output += fixture.readAllStandardOutput(); return output.contains("TEXT:a"); }));
    // Simulate an app/window manager reclaiming its desktop window after a
    // successful handoff. A live process alone must not count as attachment.
    XReparentWindow(display, id, DefaultRootWindow(display), 100, 100);
    XMapWindow(display, id);
    XFlush(display);
    CHECK(waitFor([&]() { return !selected.controlAttached(); }));
    CHECK(selected.alive());
    selected.releaseControl();
    CHECK(waitFor([&]() { return parentOf(id) != embeddedParent; }));
    CHECK(selected.alive());
    CHECK(waitFor([&]() { return !selected.capture(error).isNull(); }));

    // If Dockpipe dies, the X save-set must preserve the other application's
    // window, even when Qt's native container is destroyed by the X server.
    QProcess crashedHost;
    crashedHost.start(app.applicationFilePath(), {"--embed-fixture", QString::number(id)});
    CHECK(crashedHost.waitForStarted());
    QByteArray crashOutput;
    CHECK(waitFor([&]() { crashOutput += crashedHost.readAllStandardOutput(); return crashOutput.contains("EMBEDDED"); }));
    crashedHost.kill();
    CHECK(crashedHost.waitForFinished());
    CHECK(selected.alive());
    CHECK(waitFor([&]() { return !selected.capture(error).isNull(); }));
    CHECK(fixture.state() == QProcess::Running);

    // Closing a selected app is an expected failure, not a launcher crash.
    CHECK(selected.prepareControl(error));
    CHECK(waitFor([&]() { return selected.controlReady(); }));
    container = selected.takeControl(&host, error);
    CHECK(container);
    layout->addWidget(container);
    fixture.terminate();
    CHECK(fixture.waitForFinished());
    CHECK(!selected.alive());
    CHECK(selected.capture(error).isNull() && !error.isEmpty());
    selected.releaseControl();
    XCloseDisplay(display);
    std::puts("Native app preview, embedded input, detach, crash recovery and closed-window checks passed.");
#endif
    return 0;
}
