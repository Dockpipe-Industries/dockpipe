#include "DesktopAppWindow.h"
#include <QApplication>
#include <QCloseEvent>
#include <QElapsedTimer>
#include <QLineEdit>
#include <QLabel>
#include <QProcess>
#include <QPixmap>
#include <QPushButton>
#include <QThread>
#include <QTimer>
#include <QVBoxLayout>
#include <cstdio>
#include <functional>
#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#endif

#define CHECK(condition) do { if (!(condition)) { std::fprintf(stderr, "App window assertion failed at line %d: %s\n", __LINE__, #condition); return __LINE__; } } while (false)

namespace {
class CloseAwareFixture final : public QWidget {
public:
    explicit CloseAwareFixture(bool rejectFirstClose) : m_rejectFirstClose(rejectFirstClose) {}
protected:
    void closeEvent(QCloseEvent *event) override
    {
        if (m_rejectFirstClose) {
            m_rejectFirstClose = false;
            std::printf("CLOSE:cancelled\n");
            std::fflush(stdout);
            event->ignore();
            return;
        }
        QWidget::closeEvent(event);
    }
private:
    bool m_rejectFirstClose;
};

class FixtureProcess final : public QProcess {
public:
    ~FixtureProcess() override
    {
        if (state() == NotRunning)
            return;
        terminate();
        if (!waitForFinished(6000)) {
            kill();
            waitForFinished(1000);
        }
    }
};

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
        CloseAwareFixture window(app.arguments().contains("--reject-first-close"));
        // Browsers draw their own title bars. A restored client must not gain
        // an extra desktop frame after Qt has temporarily embedded it.
        window.setWindowFlag(Qt::FramelessWindowHint);
        window.setWindowTitle("Dockpipe isolated billboard fixture");
        QPixmap icon(32, 32);
        icon.fill(QColor("#35b77c"));
        window.setWindowIcon(QIcon(icon));
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

    FixtureProcess fixture;
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
    const bool externalFixture = !qEnvironmentVariableIsEmpty("DOCKPIPE_NATIVE_TEST_FIXTURE");
    CHECK(waitFor([&]() {
        preview = selected.capture(error);
        if (preview.isNull())
            return false;
        const QColor pixel = preview.pixelColor(preview.width() / 2, preview.height() - 5);
        return externalFixture || pixel.blue() > pixel.red();
    }));
    CHECK(preview.size() == QSize(1280, 720));

    Display *display = XOpenDisplay(nullptr);
    CHECK(display);
    if (!externalFixture) {
        CHECK(!selected.applicationIcon().isNull());
        const Atom iconProperty = XInternAtom(display, "_NET_WM_ICON", False);
        const unsigned long invalidIcon[] = {0xffffffffUL, 0xffffffffUL};
        XChangeProperty(display, id, iconProperty, XA_CARDINAL, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(invalidIcon), 2);
        XSync(display, False);
        CHECK(selected.applicationIcon().isNull());
        const unsigned long validIcon[] = {2, 2, 0xff35b77cUL, 0xff35b77cUL, 0xff35b77cUL, 0xff35b77cUL};
        XChangeProperty(display, id, iconProperty, XA_CARDINAL, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(validIcon), 6);
        XSync(display, False);
        CHECK(!selected.applicationIcon().isNull());
        const Atom modernTitle = XInternAtom(display, "_NET_WM_NAME", False);
        const Atom utf8 = XInternAtom(display, "UTF8_STRING", False);
        const auto discoveredTitle = [&]() {
            QString discoveryError;
            for (const auto &choice : DesktopAppWindow::availableWindows(discoveryError)) {
                if (choice.id == id)
                    return choice.title;
            }
            return QString();
        };
        CHECK(waitFor([&]() { return !discoveredTitle().isEmpty(); }));
        XStoreName(display, id, "Legacy RetroArch fixture");
        XDeleteProperty(display, id, modernTitle);
        XSync(display, False);
        CHECK(discoveredTitle() == "Legacy RetroArch fixture");

        const QByteArray preferredTitle("Modern fixture");
        XChangeProperty(display, id, modernTitle, utf8, 8, PropModeReplace,
            reinterpret_cast<const unsigned char *>(preferredTitle.constData()), preferredTitle.size());
        XSync(display, False);
        CHECK(discoveredTitle() == "Modern fixture");

        // Missing or oversized legacy metadata must not allocate an unbounded title.
        XDeleteProperty(display, id, modernTitle);
        XDeleteProperty(display, id, XA_WM_NAME);
        XSync(display, False);
        CHECK(discoveredTitle().isEmpty());
        const QByteArray oversizedTitle(1025, 'x');
        XStoreName(display, id, oversizedTitle.constData());
        XSync(display, False);
        CHECK(discoveredTitle().isEmpty());
        XStoreName(display, id, "Legacy RetroArch fixture");
        XSync(display, False);
        CHECK(discoveredTitle() == "Legacy RetroArch fixture");
    }
    const auto windowProperty = [&](const char *name, Atom type, long maximum) {
        Atom actual = None;
        int format = 0;
        unsigned long count = 0;
        unsigned long remaining = 0;
        unsigned char *data = nullptr;
        const int status = XGetWindowProperty(display, id, XInternAtom(display, name, False),
            0, maximum, False, type, &actual, &format, &count, &remaining, &data);
        QByteArray bytes;
        if (status == Success && actual == type && format == 32 && !remaining && data)
            bytes = QByteArray(reinterpret_cast<char *>(data), count * sizeof(long));
        if (data)
            XFree(data);
        return bytes;
    };
    const Atom motifType = XInternAtom(display, "_MOTIF_WM_HINTS", False);
    const auto originalDecorations = windowProperty("_MOTIF_WM_HINTS", motifType, 5);
    CHECK(!originalDecorations.isEmpty());
    const auto originalSizeHints = windowProperty("WM_NORMAL_HINTS", XA_WM_SIZE_HINTS, 18);
    const auto sizeConstraints = [&]() {
        XSizeHints hints{};
        long supplied = 0;
        if (!XGetWMNormalHints(display, id, &hints, &supplied))
            return QVector<long>{};
        QVector<long> values{hints.flags & (PMinSize | PMaxSize | PResizeInc | PAspect | PBaseSize)};
        if (hints.flags & PMinSize)
            values += {hints.min_width, hints.min_height};
        if (hints.flags & PMaxSize)
            values += {hints.max_width, hints.max_height};
        if (hints.flags & PResizeInc)
            values += {hints.width_inc, hints.height_inc};
        if (hints.flags & PAspect)
            values += {hints.min_aspect.x, hints.min_aspect.y, hints.max_aspect.x, hints.max_aspect.y};
        if (hints.flags & PBaseSize)
            values += {hints.base_width, hints.base_height};
        return values;
    };
    const auto originalConstraints = sizeConstraints();
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
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_NATIVE_TEST_FIXTURE"))
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
    const auto restoredDecorations = windowProperty("_MOTIF_WM_HINTS", motifType, 5);
    if (restoredDecorations != originalDecorations)
        std::fprintf(stderr, "Decoration hints changed: before=%s after=%s\n",
            originalDecorations.toHex().constData(), restoredDecorations.toHex().constData());
    CHECK(restoredDecorations == originalDecorations);
    CHECK(sizeConstraints() == originalConstraints);
    if (!externalFixture)
        CHECK(windowProperty("WM_NORMAL_HINTS", XA_WM_SIZE_HINTS, 18) == originalSizeHints);

    for (int cycle = 0; cycle < 3; ++cycle) {
        CHECK(selected.prepareControl(error));
        CHECK(waitFor([&]() { return selected.controlReady(); }));
        container = selected.takeControl(&host, error);
        CHECK(container);
        layout->addWidget(container);
        container->show();
        CHECK(waitFor([&]() { return selected.controlAttached(); }));
        if (cycle == 1) {
            host.showFullScreen();
            CHECK(waitFor([&]() { return host.isFullScreen(); }));
            CHECK(selected.syncControlGeometry());
        }
        selected.releaseControl();
        CHECK(selected.alive() && !selected.controlAttached());
        CHECK(waitFor([&]() { return !selected.capture(error).isNull(); }));
        CHECK(windowProperty("_MOTIF_WM_HINTS", motifType, 5) == originalDecorations);
        // Chromium updates preferred desktop coordinates asynchronously after
        // a resize. Its sizing constraints must survive; those coordinates may
        // legitimately differ from the initial capture on later cycles.
        CHECK(sizeConstraints() == originalConstraints);
        if (!externalFixture)
            CHECK(windowProperty("WM_NORMAL_HINTS", XA_WM_SIZE_HINTS, 18) == originalSizeHints);
        host.showNormal();
    }

    // If Dockpipe dies, the X save-set must preserve the other application's
    // window, even when Qt's native container is destroyed by the X server.
    FixtureProcess crashedHost;
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
    CHECK(waitFor([&]() { return !selected.alive(); }));
    CHECK(selected.capture(error).isNull() && !error.isEmpty());
    selected.releaseControl();
    XCloseDisplay(display);
    std::puts("Native app preview, embedded input, detach, crash recovery and closed-window checks passed.");
#endif
    return 0;
}
