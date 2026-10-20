#include "DashboardPage.h"
#include "BrowserWidget.h"
#include "SendInputWidget.h"
#include <QWebEngineView>
#include <QTcpServer>
#include <QTcpSocket>
#include "DashboardDock.h"
#include <QCursor>
#include <QScreen>
#include "DashboardFilesWidget.h"
#include "FileExplorerView.h"
#include <QSignalSpy>
#include <QShortcut>
#include <QStyle>
#include "DashboardTourDialog.h"
#include "LauncherSettings.h"
#include <QRadioButton>
#include <QScopeGuard>
#include "DashboardStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardWorkspace.h"
#include "CanvasPanel.h"
#include "ContextStore.h"
#include "SessionManager.h"
#include "Theme.h"
#include "DesktopWindowWidget.h"
#include "DesktopApplicationCatalog.h"
#include "DesktopAppWindow.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDebug>
#include <QDialog>
#include <QDialogButtonBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QImage>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLibrary>
#include <QLineEdit>
#include <QMenu>
#include <QPointer>
#include <QProcess>
#include <QPushButton>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <QToolButton>
#include <QTabBar>
#include <QTabWidget>
#include <algorithm>
#include <memory>
#include <vector>

#ifdef DOCKPIPE_X11_BILLBOARDS
#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <X11/Xutil.h>
#include <X11/keysym.h>
#endif

namespace {
DashboardDefinition emptyDashboard()
{
    DashboardDefinition definition;
    definition.id = DashboardStore::newId();
    definition.name = QStringLiteral("Development workspace");
    return definition;
}

void drainEvents()
{
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
}

bool captureIfRequested(QWidget &widget, const QString &name)
{
    const QString directory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    if (directory.isEmpty())
        return true;
    if (!QDir().mkpath(directory))
        return false;
    drainEvents();
    return widget.grab().save(QDir(directory).filePath(name + ".png"));
}

QColor pixelAt(const QImage &image, const QPoint &position)
{
    return image.pixelColor(position * image.devicePixelRatio());
}

bool isolatedXvfbDisplay()
{
#ifdef Q_OS_LINUX
    const QByteArray display = qgetenv("DISPLAY").split('.').first();
    if (!display.startsWith(':') || display.size() < 2)
        return false;
    bool numeric = false;
    display.mid(1).toInt(&numeric);
    if (!numeric)
        return false;
    // An opt-in alone must not allow this fixture to acquire real desktop apps.
    // Match the selected local X display to an actual Xvfb server process.
    for (const QString &entry : QDir("/proc").entryList(QDir::Dirs | QDir::NoDotAndDotDot)) {
        entry.toUInt(&numeric);
        if (!numeric)
            continue;
        QFile command("/proc/" + entry + "/cmdline");
        if (!command.open(QIODevice::ReadOnly))
            continue;
        const auto arguments = command.read(8192).split('\0');
        if (!arguments.isEmpty() && QFileInfo(QString::fromLocal8Bit(arguments.first())).fileName() == "Xvfb"
            && arguments.contains(display))
            return true;
    }
#endif
    return false;
}

class FixtureProcess final : public QProcess {
public:
    ~FixtureProcess() override
    {
        if (state() == NotRunning)
            return;
        terminate();
        if (!waitForFinished(2000)) {
            kill();
            waitForFinished(1000);
        }
    }
};

#ifdef Q_OS_LINUX
class IsolatedDesktopLaunch final {
public:
    IsolatedDesktopLaunch()
    {
        for (const QByteArray &key : {QByteArray("PATH"), QByteArray("XDG_DATA_HOME"),
                 QByteArray("XDG_DATA_DIRS"), QByteArray("DOCKPIPE_TEST_LAUNCH_MARKER")}) {
            m_environment.append({key, qgetenv(key)});
        }
    }

    ~IsolatedDesktopLaunch()
    {
        for (const auto &entry : m_environment) {
            if (entry.second.isNull())
                qunsetenv(entry.first.constData());
            else
                qputenv(entry.first.constData(), entry.second);
        }
    }

    bool initialize()
    {
        if (!m_directory.isValid() || !QDir().mkpath(m_directory.filePath("applications")))
            return false;
        QFile desktop(m_directory.filePath("applications/dashboard-launch-fixture.desktop"));
        if (!desktop.open(QIODevice::WriteOnly))
            return false;
        desktop.write("[Desktop Entry]\nType=Application\nName=Dashboard launch fixture\n"
            "Exec=/bin/true\nStartupWMClass=DockpipeLaunchFixture\n");
        desktop.close();
        if (!configureLauncher())
            return false;
        qputenv("PATH", m_directory.path().toUtf8());
        qputenv("XDG_DATA_HOME", m_directory.path().toUtf8());
        qputenv("XDG_DATA_DIRS", m_directory.filePath("empty").toUtf8());
        qputenv("DOCKPIPE_TEST_LAUNCH_MARKER", markerPath().toUtf8());
        QString error;
        const auto entries = DesktopApplicationCatalog::enumerate(&error);
        if (!error.isEmpty() || entries.size() != 1)
            return false;
        application = entries.first();
        return true;
    }

    bool configureLauncher(int exitCode = 0, int delayMilliseconds = 0)
    {
        QFile launcher(m_directory.filePath("gio"));
        if (!launcher.open(QIODevice::WriteOnly | QIODevice::Truncate))
            return false;
        // Dispatch metadata is the only effect. Native fixture processes are
        // owned separately; this launcher cannot start a real desktop app.
        QByteArray script = "#!/bin/sh\nprintf '%s\\n' \"$2\" >> \"$DOCKPIPE_TEST_LAUNCH_MARKER\"\n";
        if (delayMilliseconds > 0)
            script += "/bin/sleep " + QByteArray::number(delayMilliseconds / 1000.0, 'f', 3) + '\n';
        script += "exit " + QByteArray::number(exitCode) + '\n';
        if (launcher.write(script) != script.size())
            return false;
        launcher.close();
        return launcher.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    }

    int dispatchCount() const
    {
        QFile marker(markerPath());
        if (!marker.open(QIODevice::ReadOnly))
            return 0;
        return marker.read(4096).count('\n');
    }

    DesktopApplicationEntry application;

private:
    QString markerPath() const { return m_directory.filePath("dispatched"); }
    QTemporaryDir m_directory;
    QVector<QPair<QByteArray, QByteArray>> m_environment;
};
#endif

#ifdef DOCKPIPE_X11_BILLBOARDS
class NativeHandleEventRecorder final : public QObject {
public:
    QStringList events;

protected:
    bool eventFilter(QObject *object, QEvent *event) override
    {
        if (events.size() < 64) {
            if (event->type() == QEvent::MouseButtonPress)
                events.append("MouseButtonPress");
            else if (event->type() == QEvent::MouseButtonDblClick)
                events.append("MouseButtonDblClick");
            else if (event->type() == QEvent::MouseButtonRelease)
                events.append("MouseButtonRelease");
        }
        return QObject::eventFilter(object, event);
    }
};

class IsolatedNativePointer final {
public:
    IsolatedNativePointer() : m_library("Xtst", 6) {}

    bool initialize()
    {
        if (!m_library.load())
            return false;
        m_motion = reinterpret_cast<Motion>(m_library.resolve("XTestFakeMotionEvent"));
        m_button = reinterpret_cast<Button>(m_library.resolve("XTestFakeButtonEvent"));
        m_key = reinterpret_cast<Key>(m_library.resolve("XTestFakeKeyEvent"));
        auto query = reinterpret_cast<Query>(m_library.resolve("XTestQueryExtension"));
        m_display = XOpenDisplay(nullptr);
        int event = 0;
        int error = 0;
        int major = 0;
        int minor = 0;
        return m_display && m_motion && m_button && m_key && query
            && query(m_display, &event, &error, &major, &minor);
    }

    bool move(const QPoint &point)
    {
        const bool sent = m_motion(m_display, -1, point.x(), point.y(), 0);
        XSync(m_display, False);
        return sent;
    }

    bool button(bool pressed)
    {
        const bool sent = m_button(m_display, 1, pressed, 0);
        if (sent)
            m_pressed = pressed;
        XSync(m_display, False);
        return sent;
    }

    bool escape()
    {
        const KeyCode code = XKeysymToKeycode(m_display, XK_Escape);
        const bool sent = code && m_key(m_display, code, True, 0) && m_key(m_display, code, False, 0);
        XSync(m_display, False);
        return sent;
    }

    void focus(quint64 id)
    {
        XSetInputFocus(m_display, static_cast<Window>(id), RevertToParent, CurrentTime);
        XSync(m_display, False);
    }

    quint64 focusedWindow() const
    {
        Window window = None;
        int revert = 0;
        XGetInputFocus(m_display, &window, &revert);
        return window;
    }

    ~IsolatedNativePointer()
    {
        if (!m_display)
            return;
        if (m_pressed && m_button) {
            m_button(m_display, 1, False, 0);
            XSync(m_display, False);
        }
        XCloseDisplay(m_display);
    }

private:
    using Motion = Bool (*)(Display *, int, int, int, unsigned long);
    using Button = Bool (*)(Display *, unsigned int, Bool, unsigned long);
    using Key = Bool (*)(Display *, unsigned int, Bool, unsigned long);
    using Query = Bool (*)(Display *, int *, int *, int *, int *);
    QLibrary m_library;
    Display *m_display = nullptr;
    Motion m_motion = nullptr;
    Button m_button = nullptr;
    Key m_key = nullptr;
    bool m_pressed = false;
};

class IsolatedClientList final {
public:
    bool install(quint64 nativeId)
    {
        m_display = XOpenDisplay(nullptr);
        if (!m_display)
            return false;
        m_root = DefaultRootWindow(m_display);
        m_property = XInternAtom(m_display, "_NET_CLIENT_LIST", False);
        Atom type = None;
        int format = 0;
        unsigned long count = 0;
        unsigned long remaining = 0;
        unsigned char *data = nullptr;
        const int result = XGetWindowProperty(m_display, m_root, m_property, 0, 128, False,
            AnyPropertyType, &type, &format, &count, &remaining, &data);
        const bool valid = result == Success && remaining == 0
            && (type == None || (type == XA_WINDOW && format == 32));
        if (valid && type == XA_WINDOW) {
            m_hadProperty = true;
            const auto *windows = reinterpret_cast<unsigned long *>(data);
            for (unsigned long index = 0; index < count; ++index)
                m_previous.append(windows[index]);
        }
        if (data)
            XFree(data);
        if (!valid)
            return false;
        const unsigned long window = static_cast<unsigned long>(nativeId);
        XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(&window), 1);
        XSync(m_display, False);
        m_installed = true;
        return true;
    }

    void replace(const QVector<quint64> &ids)
    {
        QVector<unsigned long> windows;
        for (quint64 id : ids)
            windows.append(static_cast<unsigned long>(id));
        XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(windows.constData()), int(windows.size()));
        XSync(m_display, False);
    }

    void removeProperty()
    {
        XDeleteProperty(m_display, m_root, m_property);
        XSync(m_display, False);
    }

    bool setWindowClass(quint64 id, const QByteArray &name)
    {
        const QByteArray value = name + '\0' + name + '\0';
        XChangeProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "WM_CLASS", False),
            XA_STRING, 8, PropModeReplace, reinterpret_cast<const unsigned char *>(value.constData()), value.size());
        XSync(m_display, False);
        return true;
    }

    void removeProcessId(quint64 id)
    {
        XDeleteProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "_NET_WM_PID", False));
        XSync(m_display, False);
    }

    void setShadowMargins(quint64 id, bool invalid)
    {
        const unsigned long margins[] = {invalid ? 0xffffffffUL : 16, 16, 10, 32};
        XChangeProperty(m_display, static_cast<Window>(id), XInternAtom(m_display, "_GTK_FRAME_EXTENTS", False),
            XA_CARDINAL, 32, PropModeReplace, reinterpret_cast<const unsigned char *>(margins), 4);
        XSync(m_display, False);
    }

    QRect geometry(quint64 id) const
    {
        XWindowAttributes attributes{};
        if (!XGetWindowAttributes(m_display, static_cast<Window>(id), &attributes))
            return {};
        return QRect(attributes.x, attributes.y, attributes.width, attributes.height);
    }

    bool pointerTargets(quint64 id) const
    {
        Window current = m_root;
        // Walk only the hit-tested ancestor chain. Match the native control,
        // rather than QWidget visibility, to detect occlusion by the app.
        for (int depth = 0; depth < 32; ++depth) {
            Window root = 0;
            Window child = 0;
            int rootX = 0;
            int rootY = 0;
            int localX = 0;
            int localY = 0;
            unsigned int mask = 0;
            if (!XQueryPointer(m_display, current, &root, &child,
                    &rootX, &rootY, &localX, &localY, &mask) || !child)
                return false;
            if (child == static_cast<Window>(id))
                return true;
            if (child == current)
                return false;
            current = child;
        }
        return false;
    }

    QStringList pointerHierarchy() const
    {
        QStringList result;
        Window current = m_root;
        for (int depth = 0; depth < 32; ++depth) {
            Window root = 0;
            Window child = 0;
            int rootX = 0;
            int rootY = 0;
            int localX = 0;
            int localY = 0;
            unsigned int mask = 0;
            if (!XQueryPointer(m_display, current, &root, &child,
                    &rootX, &rootY, &localX, &localY, &mask) || !child)
                break;
            XWindowAttributes attributes{};
            XGetWindowAttributes(m_display, child, &attributes);
            int globalX = 0;
            int globalY = 0;
            Window translatedChild = 0;
            XTranslateCoordinates(m_display, child, m_root, 0, 0, &globalX, &globalY, &translatedChild);
            QWidget *widget = QWidget::find(WId(child));
            result.append(QString("id=%1 global=%2,%3 size=%4x%5 mapped=%6 widget=%7 class=%8")
                .arg(qulonglong(child)).arg(globalX).arg(globalY).arg(attributes.width).arg(attributes.height)
                .arg(attributes.map_state).arg(widget ? widget->objectName() : QString("foreign"))
                .arg(widget ? widget->metaObject()->className() : "foreign"));
            if (child == current)
                break;
            current = child;
        }
        return result;
    }

    quint64 parentWindow(quint64 id) const
    {
        Window root = 0;
        Window parent = 0;
        Window *children = nullptr;
        unsigned int count = 0;
        const bool found = XQueryTree(m_display, static_cast<Window>(id), &root, &parent, &children, &count);
        if (children)
            XFree(children);
        return found ? parent : 0;
    }

    bool viewable(quint64 id) const
    {
        XWindowAttributes attributes{};
        return XGetWindowAttributes(m_display, static_cast<Window>(id), &attributes)
            && attributes.map_state == IsViewable;
    }

    bool descendantOf(quint64 id, quint64 ancestor) const
    {
        for (int depth = 0; depth < 32 && id; ++depth) {
            if (id == ancestor)
                return true;
            const quint64 parent = parentWindow(id);
            if (parent == id)
                break;
            id = parent;
        }
        return false;
    }

    void setUnmappedState(quint64 id, unsigned long state, int format = 32,
        int count = 2, Atom propertyType = None, bool unmap = true)
    {
        const Atom stateAtom = XInternAtom(m_display, "WM_STATE", False);
        const unsigned long values[] = {state, None, None};
        XChangeProperty(m_display, static_cast<Window>(id), stateAtom,
            propertyType == None ? stateAtom : propertyType, format, PropModeReplace,
            reinterpret_cast<const unsigned char *>(values), count);
        if (unmap)
            XUnmapWindow(m_display, static_cast<Window>(id));
        XSync(m_display, False);
    }

    long managedState(quint64 id) const
    {
        const Atom stateAtom = XInternAtom(m_display, "WM_STATE", False);
        Atom type = None;
        int format = 0;
        unsigned long count = 0;
        unsigned long remaining = 0;
        unsigned char *data = nullptr;
        const int result = XGetWindowProperty(m_display, static_cast<Window>(id), stateAtom,
            0, 2, False, stateAtom, &type, &format, &count, &remaining, &data);
        long state = -1;
        if (result == Success && type == stateAtom && format == 32 && count == 2 && !remaining && data)
            state = reinterpret_cast<unsigned long *>(data)[0];
        if (data)
            XFree(data);
        return state;
    }

    void setDockWindow(quint64 id)
    {
        const Atom type = XInternAtom(m_display, "_NET_WM_WINDOW_TYPE_DOCK", False);
        XChangeProperty(m_display, static_cast<Window>(id),
            XInternAtom(m_display, "_NET_WM_WINDOW_TYPE", False), XA_ATOM, 32, PropModeReplace,
            reinterpret_cast<const unsigned char *>(&type), 1);
        XSync(m_display, False);
    }

    void beginWindowManagerReplies()
    {
        // This connection only observes notifications on the verified Xvfb
        // root. It never redirects input or manages the user's real desktop.
        XSelectInput(m_display, m_root, SubstructureNotifyMask);
        XSync(m_display, False);
    }

    void serviceWindowManagerReplies(const QVector<quint64> &ids)
    {
        const Atom changeState = XInternAtom(m_display, "WM_CHANGE_STATE", False);
        for (int index = 0; index < 64 && XPending(m_display); ++index) {
            XEvent event{};
            XNextEvent(m_display, &event);
            if (event.type == UnmapNotify && event.xunmap.send_event
                && ids.contains(quint64(event.xunmap.window))) {
                // XWithdrawWindow already unmapped the client. The WM acknowledgment
                // updates its state without a second, potentially late unmap.
                setUnmappedState(quint64(event.xunmap.window), WithdrawnState, 32, 2, None, false);
            } else if (event.type == ClientMessage && ids.contains(quint64(event.xclient.window))
                && event.xclient.message_type == changeState && event.xclient.format == 32
                && event.xclient.data.l[0] == IconicState) {
                ++m_iconifyRequests;
                setUnmappedState(quint64(event.xclient.window), IconicState);
            }
        }
    }

    int iconifyRequests() const { return m_iconifyRequests; }

    ~IsolatedClientList()
    {
        if (!m_display)
            return;
        if (m_installed) {
            if (m_hadProperty) {
                XChangeProperty(m_display, m_root, m_property, XA_WINDOW, 32, PropModeReplace,
                    reinterpret_cast<const unsigned char *>(m_previous.constData()), int(m_previous.size()));
            } else {
                XDeleteProperty(m_display, m_root, m_property);
            }
            XSync(m_display, False);
        }
        XCloseDisplay(m_display);
    }

private:
    Display *m_display = nullptr;
    Window m_root = 0;
    Atom m_property = 0;
    QVector<unsigned long> m_previous;
    bool m_hadProperty = false;
    bool m_installed = false;
    int m_iconifyRequests = 0;
};
#endif
}

class DashboardPageTest final : public QObject {
    Q_OBJECT
private slots:
    void paintsOwnSurroundAfterWindowResize();
    void independentEnvironmentsAndRoundTrip();
    void optionsAndSettingsControls();
    void repeatedSwitchesRetainContentAndObjects();
    void autoAcquireLifecycle_data();
    void autoAcquireLifecycle();
    void retainsAndReleasesInactiveForeignWindow();
    void discoveryIncludesMinimizedAndMoreThanThreeWindows();
    void nativePointerArrangesForeignWindows();
    void dockPreferenceChangesReservedSpace();
    void desktopLaunchAdmission();
    void desktopLaunchAcquisition();
    void staleDesktopPanelsAreNotRestored();
    void tourLifecycle();
    void canvasGadgetsAndBrowserTearOut();
    void fileScopesAndCanvasProjectPicker();
    void desktopBackgroundLifecycle();
};

void DashboardPageTest::paintsOwnSurroundAfterWindowResize()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    QPalette palette = page.palette();
    palette.setColor(QPalette::Window, QColor(34, 45, 56, 80));
    page.setPalette(palette);
    page.resize(900, 650);
    page.show();
    drainEvents();
    QColor surround = page.palette().color(QPalette::Window);
    surround.setAlpha(255);
    const auto ownMargin = [&page]() {
        QImage image(page.size(), QImage::Format_ARGB32_Premultiplied);
        image.fill(Qt::transparent);
        // Deliberately omit DrawWindowBackground: the page must paint its
        // surround without an ancestor or QWidget::render supplying it.
        page.render(&image, QPoint(), QRegion(), QWidget::DrawChildren);
        return image.pixelColor(4, 4);
    };
    QCOMPARE(ownMargin(), surround);
    page.showMaximized();
    drainEvents();
    QCOMPARE(ownMargin(), surround);
    page.showNormal();
    page.resize(1050, 720);
    drainEvents();
    QCOMPARE(ownMargin(), surround);
    for (const auto &theme : {QString("dark"), QString("light")}) {
        const QString path = QFINDTESTDATA("../resources/theme/pipeon-" + theme + ".qss");
        QVERIFY(!path.isEmpty());
        QFile stylesheet(path);
        QVERIFY(stylesheet.open(QIODevice::ReadOnly));
        page.setStyleSheet(QString::fromUtf8(stylesheet.readAll()));
        drainEvents();
        const QColor expected = theme == "dark" ? QColor("#191d24") : QColor("#f7f8fb");
        QCOMPARE(ownMargin(), expected);
    }
}

void DashboardPageTest::independentEnvironmentsAndRoundTrip()
{
    QTemporaryDir project;
    QVERIFY(project.isValid());
    QFile file(project.filePath("README.txt"));
    QVERIFY(file.open(QIODevice::WriteOnly));
    QVERIFY(file.write("Dashboard fixture project\n") > 0);
    file.close();
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(project.path());
    WorkflowMeta workflow;
    workflow.workflowId = "build-fixture";
    workflow.displayName = "Build project";
    factory.setWorkflows({workflow});
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    page.show();
    drainEvents();
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(canvas && tiling);
    QCOMPARE(page.environment(), QString("canvas"));
    QVERIFY(!page.addWidgetKind("desktop-window"));
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(page.addWidgetKind("files"));
    QVERIFY(page.addWidgetKind("send-input"));
    drainEvents();
    const auto canvasDefinition = page.snapshot();
    QCOMPARE(canvasDefinition.panels.size(), 2);
    for (const auto &panel : canvasDefinition.panels)
        QCOMPARE(panel.environment, QString("canvas"));
    QPointer<QWidget> files = canvas->findChild<QWidget *>("dashboardWidget_files");
    QPointer<QWidget> canvasInput = canvas->findChild<QWidget *>("dashboardWidget_send-input");
    QVERIFY(files && canvasInput);
    const auto canvasLayout = canvas->saveLayout();
    QVERIFY(captureIfRequested(page, "canvas"));
    page.setFullscreen(true);
    QVERIFY(captureIfRequested(page, "fullscreen-top-bar"));
    page.setFullscreen(false);
    drainEvents();

    page.setEnvironment("tiling");
    drainEvents();
    QCOMPARE(page.environment(), QString("tiling"));
    QVERIFY(tiling->findChildren<CanvasPanel *>().isEmpty());
    QVERIFY(!files->isVisible() && !canvasInput->isVisible());
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    const auto panels = page.snapshot().panels;
    const QString workflowId = panels.at(2).id;
    const QString inputId = panels.at(3).id;
    QVERIFY(tiling->dockPanel(inputId, workflowId, "right"));
    drainEvents();
    QPointer<QWidget> workflows = tiling->findChild<QWidget *>("dashboardWidget_workflows");
    QPointer<QWidget> tilingInput = tiling->findChild<QWidget *>("dashboardWidget_send-input");
    QVERIFY(workflows && tilingInput && tilingInput != canvasInput);
    auto *bar = page.findChild<QWidget *>("dashboardTopBar");
    auto *environments = page.findChild<QWidget *>("dashboardEnvironments");
    QCOMPARE(environments->geometry().top(), bar->geometry().bottom() + 1);
    QVERIFY(captureIfRequested(page, "tiling"));
    page.setEnvironment("canvas");
    drainEvents();
    QCOMPARE(canvas->findChild<QWidget *>("dashboardWidget_files"), files.data());
    QCOMPARE(canvas->findChild<QWidget *>("dashboardWidget_send-input"), canvasInput.data());
    QCOMPARE(canvas->saveLayout(), canvasLayout);
    QVERIFY(files->isVisible() && canvasInput->isVisible());
    QVERIFY(!workflows->isVisible() && !tilingInput->isVisible());
    page.setEnvironment("invalid");
    QCOMPARE(page.environment(), QString("canvas"));
    page.setEnvironment("tiling");
    drainEvents();
    const auto saved = page.snapshot();
    QCOMPARE(saved.layout.value("version").toInt(), 2);
    QCOMPARE(saved.layout.value("mode").toString(), QString("tiling"));
    QCOMPARE(saved.layout.value("canvas").toObject(), canvasLayout);
    QCOMPARE(saved.layout.value("tiling").toObject().value("tree").toObject().value("type").toString(), QString("split"));

    DashboardStore store;
    QVector<DashboardDefinition> existing;
    QString error;
    QVERIFY2(store.load(existing, error), qPrintable(error));
    QVERIFY2(store.save({saved}, error), qPrintable(error));
    DashboardStore restoredStore;
    QVector<DashboardDefinition> readback;
    QVERIFY2(restoredStore.load(readback, error), qPrintable(error));
    QCOMPARE(readback.size(), 1);
    QCOMPARE(readback.first().layout, saved.layout);
    QCOMPARE(readback.first().panels.size(), saved.panels.size());
    for (int index = 0; index < saved.panels.size(); ++index) {
        QCOMPARE(readback.first().panels[index].id, saved.panels[index].id);
        QCOMPARE(readback.first().panels[index].kind, saved.panels[index].kind);
        QCOMPARE(readback.first().panels[index].environment, saved.panels[index].environment);
    }
    DashboardPage restored(readback.first(), &factory);
    restored.resize(page.size());
    restored.show();
    drainEvents();
    QCOMPARE(restored.environment(), QString("tiling"));
    auto *restoredCanvas = restored.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *restoredTiling = restored.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QCOMPARE(restoredCanvas->findChildren<CanvasPanel *>().size(), 2);
    QCOMPARE(restoredTiling->findChildren<CanvasPanel *>().size(), 2);
    QVERIFY(restoredCanvas->findChild<QWidget *>("dashboardWidget_files"));
    QVERIFY(!restoredTiling->findChild<QWidget *>("dashboardWidget_files"));
    QVERIFY(restoredTiling->findChild<QWidget *>("dashboardWidget_workflows"));
    QVERIFY(!restoredCanvas->findChild<QWidget *>("dashboardWidget_workflows"));
    // A restored inactive Canvas must keep its saved positions while Tiling is
    // shown, then apply them when its real viewport becomes available.
    QCOMPARE(restoredCanvas->saveLayout().value("canvas"), canvasLayout.value("canvas"));
    restored.setEnvironment("canvas");
    drainEvents();
    QCOMPARE(restoredCanvas->saveLayout().value("canvas"), canvasLayout.value("canvas"));
    QCOMPARE(restoredCanvas->saveLayout().value("canvasOrder"), canvasLayout.value("canvasOrder"));
}

void DashboardPageTest::optionsAndSettingsControls()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 760);
    page.show();
    drainEvents();
    auto options = page.snapshot().options;
    QVERIFY(options.compactTiling && options.showPanelHeaders);
    QVERIFY(!options.autoAcquireApps);
    QCOMPARE(options.canvasBackground, QString("grid"));
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    const QImage grid = canvas->grab().toImage();
    QVERIFY(pixelAt(grid, QPoint(32, 48)) != pixelAt(grid, QPoint(33, 49)));
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    page.setEnvironment("tiling");
    const QImage tilingGrid = tiling->grab().toImage();
    QCOMPARE(pixelAt(tilingGrid, QPoint(32, 48)), pixelAt(grid, QPoint(32, 48)));
    QCOMPARE(pixelAt(tilingGrid, QPoint(33, 49)), pixelAt(grid, QPoint(33, 49)));
    page.setEnvironment("canvas");
    options.canvasBackground = "plain";
    QVERIFY(page.applyOptions(options));
    const QImage plain = canvas->grab().toImage();
    QCOMPARE(pixelAt(plain, QPoint(32, 48)), pixelAt(plain, QPoint(33, 49)));
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(32, 48)), pixelAt(plain, QPoint(32, 48)));
    QVERIFY(pixelAt(grid, QPoint(32, 48)) != pixelAt(plain, QPoint(32, 48)));
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    options.canvasBackground = "image";
    options.canvasImagePath = directory.filePath("missing.png");
    QVERIFY(!page.applyOptions(options));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    QVERIFY(page.snapshot().options.canvasImagePath.isEmpty());
    options.canvasBackground = "unknown";
    QVERIFY(!page.applyOptions(options));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    QVERIFY(!page.findChild<QLabel *>("dashboardSaveError")->text().isEmpty());
    page.showSaveError({});
    auto *settings = page.findChild<QPushButton *>("dashboardSettingsButton");
    QVERIFY(settings && !settings->icon().isNull());
    QVERIFY(settings->text().isEmpty());
    QCOMPARE(settings->accessibleName(), QString("Dashboard settings"));

    bool inspected = false;
    bool captured = false;
    QTimer::singleShot(0, &page, [&]() {
        auto *dialog = page.findChild<QDialog *>("dashboardSettingsDialog");
        if (!dialog)
            return;
        auto *compact = dialog->findChild<QCheckBox *>("dashboardSettingsCompact");
        auto *headers = dialog->findChild<QCheckBox *>("dashboardSettingsHeaders");
        auto *acquire = dialog->findChild<QCheckBox *>("dashboardSettingsAutoAcquire");
        auto *background = dialog->findChild<QComboBox *>("dashboardSettingsBackground");
        auto *image = dialog->findChild<QLineEdit *>("dashboardSettingsImage");
        auto *canvasFiles = dialog->findChild<QComboBox *>("dashboardSettingsCanvasFiles");
        auto *tilingFiles = dialog->findChild<QComboBox *>("dashboardSettingsTilingFiles");
        inspected = canvasFiles && tilingFiles
            && canvasFiles->currentData() == "project" && tilingFiles->currentData() == "system"
            && compact && headers && acquire && background && image
            && compact->isChecked() && headers->isChecked() && !acquire->isChecked()
            && background->currentData() == "plain" && !image->isEnabled();
        if (inspected) {
            compact->setChecked(false);
            headers->setChecked(false);
            background->setCurrentIndex(background->findData("dusk"));
            captured = captureIfRequested(*dialog, "settings");
        }
        dialog->reject();
    });
    settings->click();
    QVERIFY(inspected && captured);
    QVERIFY(page.snapshot().options.compactTiling);
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));

    bool saved = false;
    QTimer::singleShot(0, &page, [&]() {
        auto *dialog = page.findChild<QDialog *>("dashboardSettingsDialog");
        if (!dialog)
            return;
        dialog->findChild<QCheckBox *>("dashboardSettingsCompact")->setChecked(false);
        dialog->findChild<QCheckBox *>("dashboardSettingsAutoHideDock")->setChecked(false);
        dialog->findChild<QCheckBox *>("dashboardSettingsHeaders")->setChecked(false);
        auto *background = dialog->findChild<QComboBox *>("dashboardSettingsBackground");
        background->setCurrentIndex(background->findData("ocean"));
        auto *canvasFiles = dialog->findChild<QComboBox *>("dashboardSettingsCanvasFiles");
        auto *tilingFiles = dialog->findChild<QComboBox *>("dashboardSettingsTilingFiles");
        canvasFiles->setCurrentIndex(canvasFiles->findData("system"));
        tilingFiles->setCurrentIndex(tilingFiles->findData("project"));
        saved = true;
        dialog->findChild<QDialogButtonBox *>()->button(QDialogButtonBox::Save)->click();
    });
    settings->click();
    QVERIFY(saved);
    const auto selected = page.snapshot().options;
    QVERIFY(!selected.compactTiling && !selected.showPanelHeaders && !selected.autoAcquireApps);
    QVERIFY(!selected.autoHideDock);
    QCOMPARE(selected.canvasBackground, QString("ocean"));
    QCOMPARE(selected.canvasFileScope, QString("system"));
    QCOMPARE(selected.tilingFileScope, QString("project"));
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    QVERIFY2(store.save({page.snapshot()}, error), qPrintable(error));
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    const auto restored = definitions.first().options;
    QCOMPARE(restored.compactTiling, selected.compactTiling);
    QCOMPARE(restored.showPanelHeaders, selected.showPanelHeaders);
    QCOMPARE(restored.autoAcquireApps, selected.autoAcquireApps);
    QCOMPARE(restored.canvasBackground, selected.canvasBackground);
    QCOMPARE(restored.canvasImagePath, selected.canvasImagePath);
    QCOMPARE(restored.canvasFileScope, selected.canvasFileScope);
    QCOMPARE(restored.tilingFileScope, selected.tilingFileScope);
}

void DashboardPageTest::repeatedSwitchesRetainContentAndObjects()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    page.setEnvironment("tiling");
    QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(page.addWidgetKind("send-input"));
    page.show();
    QList<QPointer<QWidget>> content;
    for (const QString &name : {QString("dashboardCanvasWorkspace"), QString("dashboardTilingWorkspace")}) {
        auto *workspace = page.findChild<DashboardWorkspace *>(name);
        content.append(workspace->findChild<QWidget *>("dashboardWidget_workflows"));
        content.append(workspace->findChild<QWidget *>("dashboardWidget_send-input"));
    }
    for (int warmup = 0; warmup < 4; ++warmup) {
        page.setEnvironment("canvas");
        drainEvents();
        page.setEnvironment("tiling");
        drainEvents();
    }
    const int tilingObjects = page.findChildren<QObject *>().size();
    page.setEnvironment("canvas");
    drainEvents();
    const int canvasObjects = page.findChildren<QObject *>().size();
    QList<int> contentCounts;
    for (const auto &widget : content) {
        QVERIFY(widget);
        contentCounts.append(widget->findChildren<QObject *>().size());
    }
    QElapsedTimer elapsed;
    elapsed.start();
    for (int pair = 0; pair < 40; ++pair) {
        page.setEnvironment("tiling");
        drainEvents();
        QCOMPARE(page.findChildren<QObject *>().size(), tilingObjects);
        page.setEnvironment("canvas");
        drainEvents();
        QCOMPARE(page.findChildren<QObject *>().size(), canvasObjects);
    }
    const qint64 duration = elapsed.elapsed();
    qInfo("Dashboard Page: 80 environment switches in %lld ms; canvas objects=%d tiling objects=%d",
        static_cast<long long>(duration), canvasObjects, tilingObjects);
    QVERIFY2(duration < 5000, "80 lightweight Page environment switches exceeded the five-second regression budget");
    for (int index = 0; index < content.size(); ++index) {
        QVERIFY(content[index]);
        QCOMPARE(content[index]->findChildren<QObject *>().size(), contentCounts[index]);
        QVERIFY(page.isAncestorOf(content[index]));
    }
    QCOMPARE(page.panelCount(), 4);
}

void DashboardPageTest::dockPreferenceChangesReservedSpace()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    drainEvents();
    auto *dock = page.findChild<DashboardDock *>();
    auto *environments = page.findChild<QWidget *>("dashboardEnvironments");
    QVERIFY(dock && environments);
    QVERIFY(dock->isHidden());
    QCOMPARE(dock->parentWidget(), environments);
    const int fullHeight = environments->height();
    auto options = page.snapshot().options;
    options.autoHideDock = false;
    QVERIFY(page.applyOptions(options));
    drainEvents();
    QVERIFY(dock->isVisible());
    QCOMPARE(dock->parentWidget(), &page);
    QVERIFY(environments->height() < fullHeight);
    options.autoHideDock = true;
    QVERIFY(page.applyOptions(options));
    drainEvents();
    QCOMPARE(environments->height(), fullHeight);
    dock->reveal(true);
    QTRY_VERIFY(dock->isVisible());
    QVERIFY(captureIfRequested(page, "dock-overlay"));
}

void DashboardPageTest::nativePointerArrangesForeignWindows()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native input: the selected display must be a verified local Xvfb server");
    IsolatedNativePointer pointer;
    QVERIFY2(pointer.initialize(), "The disposable display requires the XTEST extension and libXtst");
    std::vector<std::unique_ptr<FixtureProcess>> fixtures;
    QVector<quint64> ids;
    for (int index = 0; index < 2; ++index) {
        auto fixture = std::make_unique<FixtureProcess>();
        fixture->setProcessChannelMode(QProcess::ForwardedErrorChannel);
        fixture->start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
        QVERIFY(fixture->waitForStarted(3000));
        QByteArray output;
        QTRY_VERIFY_WITH_TIMEOUT((output += fixture->readAllStandardOutput()).startsWith("WINDOW:")
            && output.contains('\n'), 5000);
        const quint64 id = output.split('\n').first().mid(7).toULongLong();
        QVERIFY(id);
        ids.append(id);
        fixtures.push_back(std::move(fixture));
    }
    IsolatedClientList clients;
    QVERIFY(clients.install(ids.first()));
    clients.replace(ids);
    const quint64 firstParent = clients.parentWindow(ids[0]);
    const quint64 secondParent = clients.parentWindow(ids[1]);
    DashboardWorkspace workspace("native-pointer-arrangement");
    workspace.resize(1200, 800);
    QPointer<DesktopWindowWidget> first = new DesktopWindowWidget;
    QPointer<DesktopWindowWidget> second = new DesktopWindowWidget;
    QVERIFY(workspace.addPanel("first", "First native fixture", first));
    QVERIFY(workspace.addPanel("second", "Second native fixture", second));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("first", "second", "left"));
    workspace.show();
    workspace.activateWindow();
    QString error;
    QVERIFY2(first->acquireWindow(ids[0], error), qPrintable(error));
    QVERIFY2(second->acquireWindow(ids[1], error), qPrintable(error));
    QTRY_VERIFY_WITH_TIMEOUT(first->hasInteractiveWindow() && second->hasInteractiveWindow(), 5000);
    QPointer<CanvasPanel> firstFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_first");
    QPointer<CanvasPanel> secondFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_second");
    QVERIFY(firstFrame && secondFrame);
    const auto actions = [&]() {
        return firstFrame->findChild<QToolButton *>("dashboardTileActions");
    };
    const auto hoverHandle = [&]() {
        return pointer.move(firstFrame->mapToGlobal(QPoint(firstFrame->width() / 2, 12)));
    };
    const auto previewVisible = [&]() {
        const auto edges = workspace.findChildren<QWidget *>("dashboardDropPreviewEdge");
        return edges.size() == 4 && std::all_of(edges.cbegin(), edges.cend(), [](QWidget *edge) { return edge->isVisible(); });
    };
    const auto previewHidden = [&]() {
        const auto edges = workspace.findChildren<QWidget *>("dashboardDropPreviewEdge");
        return std::none_of(edges.cbegin(), edges.cend(), [](QWidget *edge) { return edge->isVisible(); });
    };
    const auto groupFor = [&](QWidget *content) -> QTabWidget * {
        for (QWidget *parent = content->parentWidget(); parent && parent != &workspace; parent = parent->parentWidget()) {
            if (auto *group = qobject_cast<QTabWidget *>(parent))
                return group;
        }
        return nullptr;
    };
    const auto splitWithFirstOnRight = [&]() {
        const auto tree = workspace.saveLayout().value("tree").toObject();
        const auto children = tree.value("children").toArray();
        return tree.value("type") == "split" && tree.value("orientation") == "horizontal"
            && children.size() == 2
            && children.first().toObject().value("ids").toArray() == QJsonArray{"second"}
            && children.last().toObject().value("ids").toArray() == QJsonArray{"first"};
    };

    NativeHandleEventRecorder handleEvents;
    QVERIFY(actions());
    actions()->installEventFilter(&handleEvents);

    // Wait for native layout propagation before starting the captured gesture.
    // QWidget visibility alone does not establish server-side hit-test readiness.
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(ids[1]), 3000);
    const auto beforePreviewHierarchy = clients.pointerHierarchy();

    // Real server input crosses the other process's client surface, not Qt's
    // synthetic drag/drop path. The source handle must keep the pointer grab.
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions() && actions()->isVisible()
        && actions()->internalWinId() && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QTRY_VERIFY2_WITH_TIMEOUT(QWidget::mouseGrabber() == actions(),
        qPrintable(handleEvents.events.join(", ")), 3000);
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    if (!clients.pointerTargets(ids[1])) {
        qInfo() << "Before native preview:" << beforePreviewHierarchy;
        qInfo() << "After native preview:" << clients.pointerHierarchy();
        qInfo() << "Native client parents" << clients.parentWindow(ids[0]) << clients.parentWindow(ids[1]);
        const auto widgets = workspace.findChildren<QWidget *>();
        int nativeCount = 0;
        for (QWidget *widget : widgets) {
            if (!widget->internalWinId())
                continue;
            qInfo() << "Native widget" << widget->internalWinId() << widget->metaObject()->className()
                    << widget->objectName() << "parent" << (widget->parentWidget() ? widget->parentWidget()->internalWinId() : 0)
                    << "geometry" << widget->geometry() << "visible" << widget->isVisible();
            if (++nativeCount == 128)
                break;
        }
    }
    QVERIFY(clients.pointerTargets(ids[1]));
    QVERIFY(pointer.move(secondFrame->mapToGlobal(QPoint(secondFrame->width() - 12, secondFrame->height() / 2))));
    QVERIFY(pointer.button(false));
    QTRY_VERIFY_WITH_TIMEOUT(splitWithFirstOnRight(), 3000);
    QVERIFY(previewHidden());
    QCOMPARE(first->acquiredWindowId(), ids[0]);
    QCOMPARE(second->acquiredWindowId(), ids[1]);

    const auto centerGestureDiagnostics = [&]() {
        QString result;
        QDebug output(&result);
        auto *cancel = workspace.findChild<QShortcut *>("dashboardCancelDrag");
        output << "cursor" << QCursor::pos() << "grabber" << QWidget::mouseGrabber()
               << "grabbed source" << (QWidget::mouseGrabber() == actions())
               << "cancel enabled" << (cancel && cancel->isEnabled())
               << "popup" << QApplication::activePopupWidget();
        for (CanvasPanel *frame : {firstFrame.data(), secondFrame.data()}) {
            output << "frame" << frame->objectName()
                   << "workspace rect" << QRect(frame->mapTo(&workspace, QPoint()), frame->size())
                   << "global rect" << QRect(frame->mapToGlobal(QPoint()), frame->size());
        }
        for (QWidget *edge : workspace.findChildren<QWidget *>("dashboardDropPreviewEdge"))
            output << "preview" << edge->internalWinId() << edge->geometry() << edge->isVisible();
        output << "native path" << clients.pointerHierarchy();
        return result;
    };

    // The same native handle can turn two independent clients into retained tabs.
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions()->isVisible() && actions()->internalWinId()
        && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QTRY_VERIFY2_WITH_TIMEOUT(QWidget::mouseGrabber() == actions(),
        qPrintable(handleEvents.events.join(", ")), 3000);
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    const auto beforeCenterRelease = centerGestureDiagnostics();
    QVERIFY(pointer.button(false));
    const auto tabbedTogether = [&]() {
        const auto tree = workspace.saveLayout().value("tree").toObject();
        auto *group = groupFor(first);
        return tree.value("type") == "tabs" && tree.value("active") == "first"
            && tree.value("ids").toArray() == QJsonArray{"second", "first"}
            && group && group->count() == 2 && group->currentWidget()->isAncestorOf(first)
            && first->isVisible() && !second->isVisible();
    };
    QElapsedTimer tabWait;
    tabWait.start();
    while (!tabbedTogether() && tabWait.elapsed() < 3000)
        QTest::qWait(10);
    if (!tabbedTogether()) {
        qInfo() << "Before center release" << beforeCenterRelease;
        qInfo() << "After center release" << centerGestureDiagnostics();
        qInfo() << "Center-drop layout" << workspace.saveLayout();
        qInfo() << "Center-drop cursor" << QCursor::pos()
                << "first global" << QRect(firstFrame->mapToGlobal(QPoint()), firstFrame->size())
                << "second global" << QRect(secondFrame->mapToGlobal(QPoint()), secondFrame->size());
        qInfo() << "Center-drop native path" << clients.pointerHierarchy();
    }
    QVERIFY(tabbedTogether());
    QVERIFY(first->isVisible());
    QCOMPARE(first->acquiredWindowId(), ids[0]);
    QCOMPARE(second->acquiredWindowId(), ids[1]);

    // Leaving a visible tab label over its own pane edge splits out that tab.
    QPointer<QTabBar> bar = groupFor(first)->tabBar();
    const int selected = groupFor(first)->currentIndex();
    QTRY_VERIFY_WITH_TIMEOUT(bar->isVisible() && bar->tabRect(selected).isValid()
        && bar->rect().contains(bar->tabRect(selected).center()), 3000);
    // The tab's left side contains its action controls. Drag the bare label,
    // rather than accidentally pressing one of those independent buttons.
    const QPoint labelPoint(bar->tabRect(selected).right() - 12, bar->tabRect(selected).center().y());
    QCOMPARE(bar->tabAt(labelPoint), selected);
    QVERIFY(!bar->childAt(labelPoint));
    QVERIFY(pointer.move(bar->mapToGlobal(labelPoint)));
    QElapsedTimer labelWait;
    labelWait.start();
    while ((!bar->internalWinId() || !clients.pointerTargets(bar->internalWinId())) && labelWait.elapsed() < 3000)
        QTest::qWait(10);
    if (!bar->internalWinId() || !clients.pointerTargets(bar->internalWinId())) {
        qInfo() << "Native tab label" << bar->internalWinId() << "geometry" << bar->geometry()
                << "selected rect" << bar->tabRect(selected) << "label point" << labelPoint
                << "child at point" << bar->childAt(labelPoint) << "cursor" << QCursor::pos()
                << "global rect" << QRect(bar->mapToGlobal(QPoint()), bar->size());
        qInfo() << "Native tab label pointer path" << clients.pointerHierarchy();
    }
    QVERIFY(bar->internalWinId() && clients.pointerTargets(bar->internalWinId()));
    QVERIFY(pointer.button(true));
    QTRY_COMPARE_WITH_TIMEOUT(QWidget::mouseGrabber(), bar.data(), 3000);
    QVERIFY(pointer.move(firstFrame->mapToGlobal(QPoint(firstFrame->width() - 12, firstFrame->height() / 2))));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    QVERIFY(pointer.button(false));
    QTRY_VERIFY_WITH_TIMEOUT(splitWithFirstOnRight(), 3000);
    QTRY_VERIFY_WITH_TIMEOUT(first->hasInteractiveWindow() && second->hasInteractiveWindow(), 5000);
    QVERIFY(previewHidden());

    // An app had keyboard focus before the gesture. Escape must still cancel
    // the captured drag, leave both clients nested, and suppress its click menu.
    pointer.focus(ids[1]);
    QTRY_COMPARE_WITH_TIMEOUT(pointer.focusedWindow(), ids[1], 3000);
    const auto beforeCancel = workspace.saveLayout().value("tree");
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions()->isVisible() && actions()->internalWinId()
        && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QTRY_VERIFY2_WITH_TIMEOUT(QWidget::mouseGrabber() == actions(),
        qPrintable(handleEvents.events.join(", ")), 3000);
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    QVERIFY(pointer.escape());
    QTRY_VERIFY_WITH_TIMEOUT(QWidget::mouseGrabber() == nullptr && previewHidden(), 3000);
    QVERIFY(pointer.button(false));
    QCOMPARE(workspace.saveLayout().value("tree"), beforeCancel);
    QVERIFY(!QApplication::activePopupWidget());
    QVERIFY(first->hasInteractiveWindow() && second->hasInteractiveWindow());

    // A click remains the existing tile menu, while hiding the workspace or
    // deleting a dragged panel must always release native input and previews.
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions()->isVisible() && actions()->internalWinId()
        && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QVERIFY(pointer.button(false));
    QTRY_VERIFY_WITH_TIMEOUT(QApplication::activePopupWidget(), 3000);
    QVERIFY(pointer.escape());
    QTRY_VERIFY_WITH_TIMEOUT(!QApplication::activePopupWidget(), 3000);
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions()->isVisible() && actions()->internalWinId()
        && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QTRY_VERIFY2_WITH_TIMEOUT(QWidget::mouseGrabber() == actions(),
        qPrintable(handleEvents.events.join(", ")), 3000);
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    workspace.hide();
    QTRY_VERIFY_WITH_TIMEOUT(QWidget::mouseGrabber() == nullptr && previewHidden(), 3000);
    QVERIFY(pointer.button(false));
    workspace.show();
    QTRY_VERIFY_WITH_TIMEOUT(first->hasInteractiveWindow() && second->hasInteractiveWindow(), 5000);
    QVERIFY(hoverHandle());
    QTRY_VERIFY2_WITH_TIMEOUT(actions()->isVisible() && actions()->internalWinId()
        && clients.pointerTargets(actions()->internalWinId()),
        qPrintable(clients.pointerHierarchy().join("; ")), 3000);
    QVERIFY(pointer.button(true));
    QTRY_VERIFY2_WITH_TIMEOUT(QWidget::mouseGrabber() == actions(),
        qPrintable(handleEvents.events.join(", ")), 3000);
    QVERIFY(pointer.move(secondFrame->mapToGlobal(secondFrame->rect().center())));
    QTRY_VERIFY_WITH_TIMEOUT(previewVisible(), 3000);
    workspace.removePanel("first");
    QTRY_VERIFY_WITH_TIMEOUT(first.isNull() && QWidget::mouseGrabber() == nullptr && previewHidden(), 3000);
    QVERIFY(pointer.button(false));
    QTRY_VERIFY_WITH_TIMEOUT(clients.parentWindow(ids[0]) == firstParent && clients.viewable(ids[0]), 5000);
    QVERIFY(second->hasInteractiveWindow());
    QCOMPARE(second->acquiredWindowId(), ids[1]);
    second->releaseWindow();
    QTRY_VERIFY_WITH_TIMEOUT(clients.parentWindow(ids[1]) == secondParent && clients.viewable(ids[1]), 5000);
    for (const auto &fixture : fixtures)
        QCOMPARE(fixture->state(), QProcess::Running);
    qInfo() << "Native XTEST handle split/tab, own-pane tab split, Escape/menu and hide/remove cleanup passed";
#endif
}

void DashboardPageTest::discoveryIncludesMinimizedAndMoreThanThreeWindows()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition: the selected display must be a verified local Xvfb server");
    std::vector<std::unique_ptr<FixtureProcess>> fixtures;
    QVector<quint64> ids;
    for (int index = 0; index < 7; ++index) {
        auto fixture = std::make_unique<FixtureProcess>();
        fixture->setProcessChannelMode(QProcess::ForwardedErrorChannel);
        fixture->start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
        QVERIFY(fixture->waitForStarted(3000));
        QByteArray output;
        QTRY_VERIFY_WITH_TIMEOUT((output += fixture->readAllStandardOutput()).startsWith("WINDOW:")
            && output.contains('\n'), 5000);
        const quint64 id = output.split('\n').first().mid(7).toULongLong();
        QVERIFY(id);
        ids.append(id);
        fixtures.push_back(std::move(fixture));
    }
    IsolatedClientList clients;
    QVERIFY(clients.install(ids.first()));
    clients.replace(ids);
    QString error;
    QTRY_COMPARE_WITH_TIMEOUT(DesktopWindowWidget::availableWindows(error).size(), 7, 5000);
    QVERIFY2(error.isEmpty(), qPrintable(error));

    const quint64 minimized = ids[4];
    const quint64 withdrawn = ids[5];
    const quint64 dock = ids[6];
    const quint64 originalParent = clients.parentWindow(minimized);
    QVERIFY(originalParent);
    clients.setUnmappedState(minimized, IconicState);
    clients.setUnmappedState(withdrawn, WithdrawnState);
    clients.setDockWindow(dock);
    QVERIFY(!clients.viewable(minimized));
    const auto discoveredIds = [&]() {
        QSet<quint64> result;
        for (const auto &choice : DesktopWindowWidget::availableWindows(error))
            result.insert(choice.id);
        return result;
    };
    QSet<quint64> expected;
    for (int index = 0; index < 5; ++index)
        expected.insert(ids[index]);
    QCOMPARE(discoveredIds(), expected);
    QVERIFY2(error.isEmpty(), qPrintable(error));
    const auto selected = DesktopAppWindow::availableWindows(error, minimized);
    QCOMPARE(selected.size(), 1);
    QCOMPARE(selected.first().id, minimized);
    const auto visible = DesktopAppWindow::availableWindows(error, ids.first());
    QCOMPARE(visible.size(), 1);
    QCOMPARE(visible.first().id, ids.first());
    QVERIFY(DesktopAppWindow::availableWindows(error, withdrawn).isEmpty());
    QVERIFY(DesktopAppWindow::availableWindows(error, dock).isEmpty());
    QVERIFY(DesktopAppWindow::availableWindows(error, 0x7fffffffULL).isEmpty());
    QVERIFY2(error.isEmpty(), qPrintable(error));

    // Unmapped metadata must be the complete, correctly typed ICCCM pair.
    clients.setUnmappedState(withdrawn, IconicState, 32, 2, XA_CARDINAL);
    QCOMPARE(discoveredIds(), expected);
    QVERIFY(DesktopAppWindow::availableWindows(error, withdrawn).isEmpty());
    clients.setUnmappedState(withdrawn, IconicState, 32, 1);
    QCOMPARE(discoveredIds(), expected);
    QVERIFY(DesktopAppWindow::availableWindows(error, withdrawn).isEmpty());
    clients.setUnmappedState(withdrawn, IconicState, 32, 3);
    QCOMPARE(discoveredIds(), expected);
    QVERIFY(DesktopAppWindow::availableWindows(error, withdrawn).isEmpty());
    clients.setUnmappedState(withdrawn, IconicState, 8, 2);
    QCOMPARE(discoveredIds(), expected);
    QVERIFY(DesktopAppWindow::availableWindows(error, withdrawn).isEmpty());
    clients.setUnmappedState(withdrawn, WithdrawnState);

    clients.beginWindowManagerReplies();
    QTimer manager;
    manager.setInterval(5);
    connect(&manager, &QTimer::timeout, &manager, [&] { clients.serviceWindowManagerReplies(ids); });
    manager.start();
    DashboardWorkspace workspace("minimized-native-inventory");
    workspace.resize(1000, 700);
    auto *native = new DesktopWindowWidget;
    QVERIFY(workspace.addPanel("native", "Minimized fixture", native));
    workspace.show();
    QVERIFY2(native->acquireWindow(minimized, error), qPrintable(error));
    QTRY_VERIFY_WITH_TIMEOUT(native->hasInteractiveWindow()
        && clients.viewable(minimized) && clients.descendantOf(minimized, workspace.winId()), 5000);
    QCOMPARE(native->acquiredWindowId(), minimized);
    native->releaseWindow();
    QTRY_COMPARE_WITH_TIMEOUT(clients.iconifyRequests(), 1, 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.parentWindow(minimized) == originalParent
        && !clients.viewable(minimized) && clients.managedState(minimized) == IconicState, 5000);
    QCOMPARE(discoveredIds(), expected);
    for (const auto &fixture : fixtures)
        QCOMPARE(fixture->state(), QProcess::Running);
    workspace.hide();

    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    auto page = std::make_unique<DashboardPage>(emptyDashboard(), &factory);
    page->resize(1400, 1000);
    page->show();
    page->setEnvironment("tiling");
    auto options = page->snapshot().options;
    options.autoAcquireApps = true;
    QVERIFY(page->applyOptions(options));
    QTRY_COMPARE_WITH_TIMEOUT(page->panelCount(), 5, 5000);
    const auto acquiredWindows = page->findChildren<DesktopWindowWidget *>();
    QCOMPARE(acquiredWindows.size(), 5);
    QSet<quint64> acquiredIds;
    auto *managedWorkspace = page->findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(managedWorkspace);
    for (auto *window : acquiredWindows) {
        acquiredIds.insert(window->acquiredWindowId());
        const QString panelId = window->parentWidget()->objectName().mid(QString("dashboardPanel_").size());
        QVERIFY(managedWorkspace->activatePanel(panelId));
        QTRY_VERIFY_WITH_TIMEOUT(window->hasInteractiveWindow(), 5000);
        auto *status = window->findChild<QLabel *>("desktopWindowStatus");
        QVERIFY(status && status->isHidden());
        QVERIFY(clients.descendantOf(window->acquiredWindowId(), managedWorkspace->winId()));
    }
    QCOMPARE(acquiredIds, expected);
    // Later clients may share tabs, but every eligible client remains acquired.
    // Closing the owner while some tabs are hidden must restore every client.
    page->hide();
    page.reset();
    QTRY_COMPARE_WITH_TIMEOUT(clients.iconifyRequests(), 2, 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.managedState(minimized) == IconicState
        && !clients.viewable(minimized), 5000);
    for (int index = 0; index < 5; ++index) {
        qInfo() << "Released fixture" << index << ids[index] << "state" << clients.managedState(ids[index])
                << "viewable" << clients.viewable(ids[index]);
        QTRY_COMPARE_WITH_TIMEOUT(clients.parentWindow(ids[index]), originalParent, 5000);
        if (ids[index] != minimized)
            QTRY_VERIFY_WITH_TIMEOUT(clients.viewable(ids[index]), 5000);
    }
    QCOMPARE(discoveredIds(), expected);
    for (const auto &fixture : fixtures)
        QCOMPARE(fixture->state(), QProcess::Running);
    qInfo() << "Seven discovered; five acquired and individually interactive; all released alive with Iconic restoration";
#endif
}

void DashboardPageTest::retainsAndReleasesInactiveForeignWindow()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition: the selected display must be a verified local Xvfb server");
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:")
        && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    const quint64 originalParent = clients.parentWindow(nativeId);
    QVERIFY(originalParent);
    QTRY_VERIFY_WITH_TIMEOUT(clients.viewable(nativeId), 5000);

    auto ownedWorkspace = std::make_unique<DashboardWorkspace>("deferred-native-regression");
    QPointer<DashboardWorkspace> workspace = ownedWorkspace.get();
    workspace->resize(1100, 760);
    QPointer<DesktopWindowWidget> native = new DesktopWindowWidget;
    QVERIFY(workspace->addPanel("native", "Native fixture", native));
    QVERIFY(workspace->addPanel("other", "Ordinary panel", new QLabel("Retained ordinary panel")));
    workspace->show();
    QString error;
    QVERIFY2(native->acquireWindow(nativeId, error), qPrintable(error));
    QTRY_VERIFY_WITH_TIMEOUT(native->hasInteractiveWindow(), 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.descendantOf(nativeId, workspace->winId()), 5000);
    workspace->setMode("tiling");
    QVERIFY(workspace->dockPanel("native", "other", "tab"));
    QVERIFY(workspace->activatePanel("other"));
    workspace->setMode("canvas");
    workspace->setMode("tiling");
    QVERIFY(workspace->findChild<QWidget *>("dashboardPanelSlot"));
    QVERIFY(!native->isVisible());
    QCOMPARE(native->acquiredWindowId(), nativeId);
    QTRY_VERIFY_WITH_TIMEOUT(!clients.viewable(nativeId), 5000);
    auto *monitor = native->findChild<QTimer *>("desktopNativeMonitor");
    QVERIFY(monitor);
    QTRY_VERIFY_WITH_TIMEOUT(!monitor->isActive(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);

    auto *group = workspace->findChild<QTabWidget *>("dashboardTabGroup");
    QVERIFY(group && group->count() == 2);
    int nativeIndex = -1;
    for (int index = 0; index < group->count(); ++index) {
        if (group->tabText(index) == "Native fixture")
            nativeIndex = index;
    }
    QVERIFY(nativeIndex >= 0 && nativeIndex != group->currentIndex());
    QElapsedTimer activation;
    activation.start();
    QTest::mouseClick(group->tabBar(), Qt::LeftButton, {}, group->tabBar()->tabRect(nativeIndex).center());
    QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
        && clients.viewable(nativeId) && clients.descendantOf(nativeId, workspace->winId()), 5000);
    QVERIFY(group->currentWidget()->isAncestorOf(native));
    qInfo() << "Deferred native first activation ms" << activation.elapsed();
    QCOMPARE(native->acquiredWindowId(), nativeId);
    QCOMPARE(fixture.state(), QProcess::Running);

    for (int cycle = 0; cycle < 2; ++cycle) {
        for (const QString &mode : {QStringLiteral("canvas"), QStringLiteral("tiling")}) {
            workspace->setMode(mode);
            QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
                && clients.viewable(nativeId) && clients.descendantOf(nativeId, workspace->winId()), 5000);
            QCOMPARE(native->acquiredWindowId(), nativeId);
            QCOMPARE(fixture.state(), QProcess::Running);
        }
        QVERIFY(workspace->minimizePanel("native"));
        QTRY_VERIFY_WITH_TIMEOUT(!native->isVisible() && !clients.viewable(nativeId), 5000);
        QCOMPARE(native->acquiredWindowId(), nativeId);
        QCOMPARE(fixture.state(), QProcess::Running);
        QVERIFY(workspace->activatePanel("native"));
        QTRY_VERIFY_WITH_TIMEOUT(native->isVisible() && native->hasInteractiveWindow()
            && clients.viewable(nativeId), 5000);
        QCOMPARE(native->acquiredWindowId(), nativeId);
    }
    QVERIFY(workspace->activatePanel("other"));
    QTRY_VERIFY_WITH_TIMEOUT(!native->isVisible() && !clients.viewable(nativeId), 5000);
    QCOMPARE(native->acquiredWindowId(), nativeId);
    QCOMPARE(fixture.state(), QProcess::Running);
    workspace->setAttribute(Qt::WA_DeleteOnClose);
    QVERIFY(workspace->close());
    ownedWorkspace.release();
    QTRY_VERIFY_WITH_TIMEOUT(workspace.isNull() && native.isNull(), 5000);
    QTRY_VERIFY_WITH_TIMEOUT(clients.parentWindow(nativeId) == originalParent
        && clients.viewable(nativeId), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    qInfo() << "Native client survived deferred tab activation, modes, minimize/restore and workspace close";
#endif
}

void DashboardPageTest::autoAcquireLifecycle_data()
{
    QTest::addColumn<bool>("omitProcessId");
    QTest::addColumn<int>("shadowMode");
    QTest::newRow("published-process-id") << false << 0;
#ifdef DOCKPIPE_X11_RESOURCE_IDENTITIES
    QTest::newRow("server-process-id") << true << 0;
#endif
    QTest::newRow("client-shadow-margins") << false << 1;
    QTest::newRow("invalid-shadow-margins") << false << 2;
}

void DashboardPageTest::autoAcquireLifecycle()
{
    QFETCH(bool, omitProcessId);
    QFETCH(int, shadowMode);
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop acquisition requires explicit opt-in on a disposable Xvfb display.");
#ifndef DOCKPIPE_X11_BILLBOARDS
    QSKIP("This build has no X11 native-window backend.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition: the selected display must be a verified local Xvfb server");
    const QString unavailable = DesktopWindowWidget::automaticAcquisitionUnavailableReason();
    QVERIFY2(unavailable.isEmpty(), qPrintable(unavailable));
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture", "--reject-first-close"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:") && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    if (omitProcessId)
        clients.removeProcessId(nativeId);
    if (shadowMode)
        clients.setShadowMargins(nativeId, shadowMode == 2);
    const auto available = [nativeId]() {
        QString error;
        const auto windows = DesktopWindowWidget::availableWindows(error);
        return error.isEmpty() && windows.size() == 1 && windows.first().id == nativeId;
    };
    // Never opt into auto acquisition while another external window is present.
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    page.setEnvironment("tiling");
    drainEvents();
    QVERIFY(!page.snapshot().options.autoAcquireApps);
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(available());
    auto options = page.snapshot().options;
    options.autoAcquireApps = true;
    QVERIFY(page.applyOptions(options));
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 1, 5000);
    QPointer<DesktopWindowWidget> acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    auto *dock = page.findChild<DashboardDock *>();
    auto *edge = page.findChild<QWidget *>("dashboardDockRevealEdge");
    QVERIFY(dock && edge);
    QCursor::setPos(edge->mapToGlobal(edge->rect().center()));
    QTRY_VERIFY_WITH_TIMEOUT(dock->isVisible(), 1500);
    QTRY_VERIFY(dock->geometry().bottom() < dock->parentWidget()->height());
    const QString screenshotDirectory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    if (!screenshotDirectory.isEmpty()) {
        QDir().mkpath(screenshotDirectory);
        QTest::qWait(200);
        QVERIFY(page.screen()->grabWindow(0).save(screenshotDirectory + "/native-dock-overlay.png"));
    }
    QCursor::setPos(page.mapToGlobal(QPoint(20, 20)));
    QTRY_VERIFY_WITH_TIMEOUT(dock->isHidden(), 1500);
    auto *nativeContainer = acquired->findChild<QWidget *>("billboardNativeApp");
    QVERIFY(nativeContainer);
    QTRY_COMPARE(nativeContainer->size(), acquired->size());
    QCOMPARE(nativeContainer->mapTo(acquired, QPoint()), QPoint());
    const auto expectedGeometry = [&]() {
        const qreal scale = nativeContainer->devicePixelRatioF();
        const QSize size(qRound(nativeContainer->width() * scale), qRound(nativeContainer->height() * scale));
        return shadowMode == 1 ? QRect(QPoint(-16, -10), size + QSize(32, 42)) : QRect(QPoint(), size);
    };
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    QVERIFY(edge->isHidden());
    const QPoint bottomPixel = acquired->mapToGlobal(QPoint(acquired->width() / 2, acquired->height() - 2));
    const QColor bottomColor = page.screen()->grabWindow(0, bottomPixel.x(), bottomPixel.y(), 1, 1).toImage().pixelColor(0, 0);
    QVERIFY2(bottomColor.blue() > bottomColor.red(), "The dock hotspot must not paint a gray strip over the native app");
    auto *pane = qobject_cast<QWidget *>(acquired->parent());
    QVERIFY(pane);
    QCOMPARE(acquired->width(), pane->width());
    const QString panelId = pane->objectName().mid(QString("dashboardPanel_").size());
    auto *workspace = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(page.addWidgetKind("workflows"));
    const QString neighborId = page.snapshot().panels.last().id;
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    const QSize splitSize = acquired->size();
    auto *tileActions = pane->findChild<QToolButton *>("dashboardTileActions");
    QVERIFY(tileActions);
    const auto openTileMenu = [&]() {
        QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
        QTest::mouseClick(tileActions, Qt::LeftButton);
        return qobject_cast<QMenu *>(QApplication::activePopupWidget());
    };
    const auto findAction = [](QMenu *menu, const QString &text) -> QAction * {
        if (!menu)
            return nullptr;
        for (QAction *action : menu->actions()) {
            if (action->text() == text)
                return action;
        }
        return nullptr;
    };
    QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isVisible(), 1500);
    QVERIFY(tileActions->testAttribute(Qt::WA_NativeWindow));
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(tileActions->winId()), 1500);
    QMenu *tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *expand = findAction(tileMenu, "Expand panel");
    QVERIFY(expand);
    tileMenu->close();
    expand->trigger();
    QTRY_VERIFY(acquired->width() > splitSize.width());
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *restore = findAction(tileMenu, "Restore panel");
    QVERIFY(restore);
    tileMenu->close();
    restore->trigger();
    QTRY_COMPARE(acquired->size(), splitSize);
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    tileActions->clearFocus();
    QCursor::setPos(acquired->mapToGlobal(acquired->rect().center()));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isHidden(), 1500);
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(nativeId), 1500);
    workspace->panelCloseRequested(neighborId);
    QTRY_COMPARE(clients.geometry(nativeId), expectedGeometry());
    QVERIFY(workspace->minimizePanel(panelId));
    QVERIFY(!acquired->isVisible());
    QVERIFY(!available());
    QVERIFY(workspace->activatePanel(panelId));
    QTRY_VERIFY(acquired->hasInteractiveWindow());
    QVERIFY(!acquired->acquiredWindowTitle().isEmpty());
    QVERIFY(!acquired->applicationIcon().isNull());
    QVERIFY(page.snapshot().panels.isEmpty());
    QVERIFY(page.snapshot().layout.value("tiling").toObject().value("tree").toObject().isEmpty());

    auto *monitor = acquired->findChild<QTimer *>("desktopNativeMonitor");
    QVERIFY(monitor);
    page.setEnvironment("canvas");
    QVERIFY(acquired && !acquired->isVisible());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    QTRY_VERIFY(!monitor->isActive());
    QVERIFY(!available());
    page.hide();
    page.show();
    drainEvents();
    QCOMPARE(page.environment(), QString("canvas"));
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(!available());
    page.setEnvironment("tiling");
    QCOMPARE(page.findChild<DesktopWindowWidget *>(), acquired.data());
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    // Navigating to another page retains it. Closing the launcher explicitly
    // releases all desktop panels before hiding or destroying the host.
    page.hide();
    QTRY_VERIFY(!monitor->isActive());
    QVERIFY(acquired && !available());
    page.show();
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCursor::setPos(pane->mapToGlobal(QPoint(pane->width() / 2, tileActions->height() / 2)));
    QTRY_VERIFY_WITH_TIMEOUT(tileActions->isVisible(), 1500);
    QTRY_VERIFY_WITH_TIMEOUT(clients.pointerTargets(tileActions->winId()), 1500);
    tileMenu = openTileMenu();
    QVERIFY(tileMenu);
    auto *release = findAction(tileMenu, "Release to desktop");
    QVERIFY(release);
    tileMenu->close();
    release->trigger();
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QVERIFY(page.addWidgetKind("workflows"));
    const QString retainedPanelId = page.snapshot().panels.last().id;
    page.releaseDesktopWindows();
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(page.snapshot().panels.first().id, retainedPanelId);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    // Shutdown can reach closeEvent, aboutToQuit, and the destructor. Releasing
    // more than once must keep the other app and ordinary dashboard panels.
    page.releaseDesktopWindows();
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(available());
    page.hide();
    page.show();
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 2, 5000);
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    workspace->panelCloseRequested(retainedPanelId);
    options.autoAcquireApps = false;
    QVERIFY(page.applyOptions(options));
    QTRY_VERIFY(acquired.isNull());
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    drainEvents();
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(available());
    auto *closingPage = new DashboardPage(emptyDashboard(), &factory);
    closingPage->show();
    closingPage->setEnvironment("tiling");
    options.autoAcquireApps = true;
    QVERIFY(closingPage->applyOptions(options));
    QPointer<DesktopWindowWidget> closingWindow = closingPage->findChild<DesktopWindowWidget *>();
    QVERIFY(closingWindow);
    // Switching immediately, before native attachment finishes, must also keep
    // the app owned without consuming its visible attachment timeout.
    closingPage->setEnvironment("canvas");
    QTest::qWait(3200);
    QVERIFY(closingWindow);
    QCOMPARE(closingWindow->acquiredWindowId(), nativeId);
    QVERIFY(!closingWindow->findChild<QTimer *>("desktopNativeMonitor")->isActive());
    QVERIFY(!available());
    closingPage->setEnvironment("tiling");
    QTRY_VERIFY_WITH_TIMEOUT(closingWindow->hasInteractiveWindow(), 5000);
    closingPage->setEnvironment("canvas");
    QVERIFY(closingWindow && !closingWindow->isVisible());
    delete closingPage;
    QVERIFY(closingWindow.isNull());
    QTRY_VERIFY_WITH_TIMEOUT(available(), 5000);
    QCOMPARE(fixture.state(), QProcess::Running);
    // Only an explicit user close in managed Tiling sends the OS close request.
    // The first request is rejected by the fixture, like cancelling a save prompt.
    options.autoAcquireApps = true;
    QVERIFY(page.applyOptions(options));
    acquired = page.findChild<DesktopWindowWidget *>();
    QVERIFY(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    auto *managedWorkspace = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    const QString managedId = acquired->parentWidget()->objectName().mid(QString("dashboardPanel_").size());
    managedWorkspace->panelCloseRequested(managedId);
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).contains("CLOSE:cancelled"), 3000);
    QVERIFY(acquired && acquired->hasInteractiveWindow());
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(fixture.state(), QProcess::Running);
    managedWorkspace->panelCloseRequested(managedId);
    QTRY_COMPARE_WITH_TIMEOUT(fixture.state(), QProcess::NotRunning, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(acquired.isNull(), 3000);
    QCOMPARE(page.panelCount(), 0);
    QVERIFY(page.findChild<QLabel *>("dashboardSaveError")->isHidden());
#endif
}

void DashboardPageTest::desktopLaunchAdmission()
{
#ifndef Q_OS_LINUX
    QSKIP("Isolated gio dispatch is Linux-specific.");
#else
    IsolatedDesktopLaunch launch;
    QVERIFY(launch.initialize());
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.show();
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 0);
    page.setEnvironment("tiling");
    page.setCanAddPanel([]() { return false; });
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 0);
    page.setCanAddPanel({});
    for (int index = 0; index < DashboardStore::maximumPanelsPerDashboard; ++index)
        QVERIFY(page.addWidgetKind("workflows"));
    QVERIFY(!page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), DashboardStore::maximumPanelsPerDashboard);
    QTest::qWait(250);
    QCOMPARE(launch.dispatchCount(), 0);
    QVERIFY(page.findChildren<DesktopWindowWidget *>().isEmpty());
#endif
}

void DashboardPageTest::desktopLaunchAcquisition()
{
    if (qEnvironmentVariable("DOCKPIPE_ISOLATED_DESKTOP_TEST") != "1")
        QSKIP("Desktop launch acquisition requires opt-in on a disposable Xvfb display.");
#if !defined(DOCKPIPE_X11_BILLBOARDS) || !defined(Q_OS_LINUX)
    QSKIP("This fixture requires Linux X11 native window support.");
#else
    QVERIFY2(QGuiApplication::platformName() == "xcb" && isolatedXvfbDisplay(),
        "Refusing native acquisition outside a verified local Xvfb server");
    IsolatedDesktopLaunch launch;
    QVERIFY(launch.initialize());
    FixtureProcess fixture;
    fixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    fixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(fixture.waitForStarted(3000));
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += fixture.readAllStandardOutput()).startsWith("WINDOW:") && output.contains('\n'), 5000);
    const quint64 nativeId = output.split('\n').first().mid(7).toULongLong();
    QVERIFY(nativeId);
    IsolatedClientList clients;
    QVERIFY(clients.install(nativeId));
    QVERIFY(clients.setWindowClass(nativeId, "dockpipelaunchfixture"));
    clients.replace({});
    const auto windowIsSelectable = [nativeId]() {
        QString error;
        const auto choices = DesktopWindowWidget::availableWindows(error);
        return error.isEmpty() && choices.size() == 1 && choices.first().id == nativeId;
    };
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1000, 740);
    page.show();
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 1);
    auto windows = page.findChildren<DesktopWindowWidget *>();
    QCOMPARE(windows.size(), 1);
    QPointer<DesktopWindowWidget> acquired = windows.first();
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 1, 3000);
    QVERIFY(!acquired->findChild<QDialog *>("desktopWindowPicker"));
    // Make the owned window discoverable only after dispatch, as a newly mapped
    // application would appear. The class intentionally differs only in case.
    clients.replace({nativeId});
    QTRY_VERIFY_WITH_TIMEOUT(acquired->hasInteractiveWindow(), 5000);
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    QVERIFY(page.snapshot().panels.isEmpty());
    QVERIFY(page.snapshot().layout.value("tiling").toObject().value("tree").toObject().isEmpty());
    QVERIFY(!acquired->findChild<QDialog *>("desktopWindowPicker"));
    auto *discovery = acquired->findChild<QTimer *>("desktopApplicationDiscovery");
    QVERIFY(discovery && !discovery->isActive());
    page.setCanAddPanel([]() { return false; });
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(page.panelCount(), 1);
    QCOMPARE(launch.dispatchCount(), 1);
    page.setCanAddPanel({});
    auto closeDesktopPanel = [&page](DesktopWindowWidget *window) {
        auto *frame = window->parentWidget();
        const QString id = frame->objectName().mid(QString("dashboardPanel_").size());
        page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace")->panelCloseRequested(id);
    };
    page.setEnvironment("canvas");
    QVERIFY(acquired && !acquired->isVisible());
    QCOMPARE(acquired->acquiredWindowId(), nativeId);
    closeDesktopPanel(acquired);
    QTRY_VERIFY_WITH_TIMEOUT(acquired.isNull(), 3000);
    QCOMPARE(page.panelCount(), 0);
    QTRY_VERIFY_WITH_TIMEOUT(windowIsSelectable(), 3000);
    QCOMPARE(fixture.state(), QProcess::Running);

    // Releasing a pending launch prevents a later matching window from being
    // acquired, including after returning to Tiling with automatic capture off.
    clients.replace({});
    QVERIFY(launch.configureLauncher(0, 500));
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 2, 3000);
    bool responsive = false;
    QTimer::singleShot(0, &page, [&responsive]() { responsive = true; });
    QTRY_VERIFY_WITH_TIMEOUT(responsive, 200);
    page.setEnvironment("canvas");
    closeDesktopPanel(page.findChild<DesktopWindowWidget *>());
    clients.replace({nativeId});
    QTest::qWait(700);
    for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
        auto *discovery = window->findChild<QTimer *>("desktopApplicationDiscovery");
        QVERIFY(discovery && !discovery->isActive());
        QVERIFY(!window->hasInteractiveWindow());
        QVERIFY(!window->findChild<QDialog *>("desktopWindowPicker"));
    }
    page.setEnvironment("tiling");
    QTest::qWait(700);
    QVERIFY(windowIsSelectable());
    for (auto *window : page.findChildren<DesktopWindowWidget *>())
        QVERIFY(!window->hasInteractiveWindow());

    QVERIFY(launch.configureLauncher());
    // Dock activation brings the sole existing exact-class window into Tiling
    // without dispatching another process or racing a delayed new window.
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(launch.dispatchCount(), 2);
    const auto attachedWindow = [&page]() -> DesktopWindowWidget * {
        for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
            if (window->hasInteractiveWindow())
                return window;
        }
        return nullptr;
    };
    QTRY_VERIFY_WITH_TIMEOUT(attachedWindow(), 5000);
    QCOMPARE(attachedWindow()->acquiredWindowId(), nativeId);
    QVERIFY(!attachedWindow()->findChild<QDialog *>("desktopWindowPicker"));
    page.setEnvironment("canvas");
    closeDesktopPanel(page.findChild<DesktopWindowWidget *>());
    QTRY_VERIFY_WITH_TIMEOUT(windowIsSelectable(), 3000);

    FixtureProcess secondFixture;
    secondFixture.setProcessChannelMode(QProcess::ForwardedErrorChannel);
    secondFixture.start(QCoreApplication::applicationDirPath() + "/desktop-app-window-test", {"--fixture"});
    QVERIFY(secondFixture.waitForStarted(3000));
    QByteArray secondOutput;
    QTRY_VERIFY_WITH_TIMEOUT((secondOutput += secondFixture.readAllStandardOutput()).startsWith("WINDOW:")
        && secondOutput.contains('\n'), 5000);
    const quint64 secondId = secondOutput.split('\n').first().mid(7).toULongLong();
    QVERIFY(secondId && secondId != nativeId);
    QVERIFY(clients.setWindowClass(secondId, "DockpipeLaunchFixture"));
    clients.replace({nativeId, secondId});
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QCOMPARE(launch.dispatchCount(), 2);
    QVERIFY(page.findChild<QDialog *>("desktopWindowPicker"));
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    // Cancelling the chooser removes its pending tile without acquiring either
    // eligible window. The widgets belong to this session, never saved metadata.
    page.findChild<QDialog *>("desktopWindowPicker")->reject();
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    page.setEnvironment("canvas");
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    QCOMPARE(fixture.state(), QProcess::Running);
    QCOMPARE(secondFixture.state(), QProcess::Running);
    // Missing desktop metadata is a refusal, not a reason to dispatch blindly.
    clients.removeProperty();
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTest::qWait(100);
    QCOMPARE(launch.dispatchCount(), 2);
    QVERIFY(!attachedWindow());
    auto *dashboardError = page.findChild<QLabel *>("dashboardSaveError");
    QVERIFY(dashboardError);
    QTRY_VERIFY_WITH_TIMEOUT(dashboardError->text().contains("No app was launched"), 3000);
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    page.setEnvironment("canvas");

    clients.replace({});
    QVERIFY(launch.configureLauncher(9));
    page.setEnvironment("tiling");
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_COMPARE_WITH_TIMEOUT(launch.dispatchCount(), 3, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(dashboardError->text().contains("exit 9"), 3000);
    QTRY_COMPARE_WITH_TIMEOUT(page.panelCount(), 0, 3000);
    QTRY_VERIFY_WITH_TIMEOUT(page.findChildren<DesktopWindowWidget *>().isEmpty(), 3000);
    clients.replace({nativeId});
    QTest::qWait(200);
    QVERIFY(!attachedWindow());
    for (auto *window : page.findChildren<DesktopWindowWidget *>()) {
        auto *timer = window->findChild<QTimer *>("desktopApplicationDiscovery");
        QVERIFY(timer && !timer->isActive());
    }
    // Closing an owned native fixture removes its panel and collapses the split
    // while preserving the neighboring ordinary widget and its content instance.
    QVERIFY(launch.configureLauncher());
    QVERIFY(page.addWidgetKind("workflows"));
    const QString ordinaryId = page.snapshot().panels.first().id;
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QPointer<QWidget> ordinary = tiling->findChild<QWidget *>("dashboardWidget_workflows");
    QVERIFY(ordinary);
    QVERIFY(page.launchDesktopApplication(launch.application));
    QTRY_VERIFY_WITH_TIMEOUT(attachedWindow(), 5000);
    QPointer<DesktopWindowWidget> closing = attachedWindow();
    QPointer<QTimer> closingDiscovery = closing->findChild<QTimer *>("desktopApplicationDiscovery");
    QVERIFY(closingDiscovery && !closingDiscovery->isActive());
    QCOMPARE(page.panelCount(), 2);
    QCOMPARE(page.snapshot().panels.size(), 1);
    clients.replace({});
    fixture.terminate();
    QVERIFY(fixture.waitForFinished(3000));
    QTRY_VERIFY_WITH_TIMEOUT(closing.isNull(), 5000);
    QVERIFY(closingDiscovery.isNull());
    QCOMPARE(page.panelCount(), 1);
    QVERIFY(ordinary && ordinary->isVisible());
    QCOMPARE(tiling->findChild<QWidget *>("dashboardWidget_workflows"), ordinary.data());
    const auto remainingTree = page.snapshot().layout.value("tiling").toObject().value("tree").toObject();
    QCOMPARE(remainingTree.value("type").toString(), QString("tabs"));
    QCOMPARE(remainingTree.value("ids").toArray(), QJsonArray{ordinaryId});
    page.setEnvironment("canvas");
#endif
}

void DashboardPageTest::staleDesktopPanelsAreNotRestored()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage original(emptyDashboard(), &factory);
    original.resize(1000, 740);
    original.show();
    QVERIFY(original.addWidgetKind("workflows"));
    original.setEnvironment("tiling");
    QVERIFY(original.addWidgetKind("workflows"));
    QVERIFY(original.addWidgetKind("send-input"));
    const auto ordinaryPanels = original.snapshot().panels;
    auto *originalTiling = original.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(originalTiling->dockPanel(ordinaryPanels.at(2).id, ordinaryPanels.at(1).id, "tab"));
    drainEvents();
    auto saved = original.snapshot();
    const auto expectedCanvas = saved.layout.value("canvas").toObject();
    const auto expectedTiling = saved.layout.value("tiling").toObject();
    const auto expectedTree = expectedTiling.value("tree").toObject();
    const QString staleSplit = "stale-desktop-split";
    const QString staleFloating = "stale-desktop-floating";
    const QString staleCanvas = "stale-desktop-canvas";
    saved.panels.append({staleSplit, "desktop-window", "tiling"});
    saved.panels.append({staleFloating, "desktop-window", "tiling"});
    saved.panels.append({staleCanvas, "desktop-window", "canvas"});
    auto tiling = expectedTiling;
    const QJsonObject staleTabs{{"type", "tabs"}, {"ids", QJsonArray{staleSplit}}, {"active", staleSplit}};
    tiling["tree"] = QJsonObject{{"type", "split"}, {"orientation", "horizontal"},
        {"children", QJsonArray{expectedTree, staleTabs}}, {"sizes", QJsonArray{3, 1}}};
    tiling["floating"] = QJsonArray{staleFloating};
    tiling["floatingGeometry"] = QJsonObject{{staleFloating, QJsonArray{20, 20, 400, 300}}};
    saved.layout["tiling"] = tiling;
    auto canvas = expectedCanvas;
    auto order = canvas.value("canvasOrder").toArray();
    order.append(staleCanvas);
    canvas["canvasOrder"] = order;
    auto geometry = canvas.value("canvas").toObject();
    geometry[staleCanvas] = QJsonArray{30, 30, 400, 300};
    canvas["canvas"] = geometry;
    saved.layout["canvas"] = canvas;
    DashboardPage restored(saved, &factory);
    restored.resize(original.size());
    restored.show();
    drainEvents();
    QCOMPARE(restored.panelCount(), 3);
    QVERIFY(restored.findChildren<DesktopWindowWidget *>().isEmpty());
    QVERIFY(restored.findChild<QLabel *>("dashboardSaveError")->text().isEmpty());
    const auto clean = restored.snapshot();
    QCOMPARE(clean.panels.size(), 3);
    QCOMPARE(clean.layout.value("tiling").toObject().value("tree").toObject(), expectedTree);
    QVERIFY(clean.layout.value("tiling").toObject().value("floating").toArray().isEmpty());
    const QByteArray serialized = QJsonDocument(clean.layout).toJson(QJsonDocument::Compact);
    QVERIFY(!serialized.contains("stale-desktop"));
    restored.setEnvironment("canvas");
    restored.setEnvironment("tiling");
    QCOMPARE(restored.panelCount(), 3);
    QVERIFY(restored.findChildren<DesktopWindowWidget *>().isEmpty());
}

void DashboardPageTest::tourLifecycle()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.show();
    auto respond = [&page](bool finish, bool suppress) {
        QTimer::singleShot(0, &page, [finish, suppress]() {
            auto *dialog = qobject_cast<DashboardTourDialog *>(QApplication::activeModalWidget());
            QVERIFY(dialog);
            dialog->findChild<QComboBox *>("dashboardTourBackground")->setCurrentIndex(
                dialog->findChild<QComboBox *>("dashboardTourBackground")->findData("ocean"));
            dialog->findChild<QRadioButton *>("dashboardTourTiling")->setChecked(true);
            dialog->findChild<QCheckBox *>("dashboardTourDoNotShowAgain")->setChecked(suppress);
            if (finish)
                dialog->accept();
            else
                dialog->reject();
        });
    };
    bool completed = true;
    bool suppressed = true;
    respond(false, false);
    page.showTour(true);
    QCOMPARE(page.environment(), QString("canvas"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("grid"));
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(!completed && !suppressed);
    respond(false, true);
    page.showTour(true);
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(!completed && suppressed);
    // Suppressed automatic invocation must return without opening a dialog.
    page.showTour(true);
    respond(true, false);
    page.showTour();
    QCOMPARE(page.environment(), QString("tiling"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("ocean"));
    QVERIFY(!page.snapshot().options.autoAcquireApps);
    QVERIFY(LauncherSettings::loadDashboardTourState(completed, suppressed));
    QVERIFY(completed && !suppressed);
    page.showTour(true);
    // Manual replay remains available after completion, with cancellation inert.
    respond(false, false);
    page.showTour();
    QCOMPARE(page.environment(), QString("tiling"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("ocean"));
    auto options = page.snapshot().options;
    options.canvasBackground = "desktop";
    QVERIFY(page.applyOptions(options));
    QString error;
    QVERIFY2(DashboardStore::validate({page.snapshot()}, error), qPrintable(error));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("desktop"));
}

void DashboardPageTest::desktopBackgroundLifecycle()
{
#ifndef Q_OS_LINUX
    QSKIP("Mock gsettings lifecycle is Linux-specific.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QByteArray oldPath = qgetenv("PATH");
    const QByteArray oldDesktop = qgetenv("XDG_CURRENT_DESKTOP");
    const QByteArray oldImage = qgetenv("DOCKPIPE_TEST_WALLPAPER");
    const auto restoreEnvironment = qScopeGuard([&]() {
        qputenv("PATH", oldPath);
        qputenv("XDG_CURRENT_DESKTOP", oldDesktop);
        if (oldImage.isNull())
            qunsetenv("DOCKPIPE_TEST_WALLPAPER");
        else
            qputenv("DOCKPIPE_TEST_WALLPAPER", oldImage);
    });
    const QString imagePath = directory.filePath("wallpaper.png");
    QImage image(80, 80, QImage::Format_RGB32);
    image.fill(QColor("#38c862"));
    QVERIFY(image.save(imagePath));
    QFile program(directory.filePath("gsettings"));
    QVERIFY(program.open(QIODevice::WriteOnly));
    const QByteArray script = "#!/bin/sh\n/bin/sleep 0.15\n"
        "if [ \"$1\" = get ]; then exit 1; fi\n"
        "printf \"org.gnome.desktop.background picture-uri '%s'\\n\" \"$DOCKPIPE_TEST_WALLPAPER\"\n";
    QCOMPARE(program.write(script), script.size());
    program.close();
    QVERIFY(program.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    qputenv("PATH", directory.path().toUtf8() + ":" + oldPath);
    qputenv("XDG_CURRENT_DESKTOP", "GNOME");
    qputenv("DOCKPIPE_TEST_WALLPAPER", imagePath.toUtf8());
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(900, 700);
    page.show();
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    auto *status = page.findChild<QLabel *>("dashboardBackgroundStatus");
    QVERIFY(canvas && tiling && status);
    page.setEnvironment("tiling");
    auto options = page.snapshot().options;
    options.canvasBackground = "desktop";
    QElapsedTimer response;
    response.start();
    QVERIFY(page.applyOptions(options));
    QVERIFY2(response.elapsed() < 100, "Wallpaper lookup blocked the UI thread.");
    options.canvasBackground = "plain";
    QVERIFY(page.applyOptions(options));
    QTest::qWait(500);
    QVERIFY(pixelAt(canvas->grab().toImage(), QPoint(20, 20)) != QColor("#38c862"));
    QCOMPARE(page.snapshot().options.canvasBackground, QString("plain"));
    options.canvasBackground = "desktop";
    QVERIFY(page.applyOptions(options));
    QTRY_COMPARE_WITH_TIMEOUT(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#38c862"), 3000);
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#38c862"));
    QVERIFY(status->text().isEmpty());
    // A wallpaper changed in place refreshes on returning to Canvas.
    image.fill(QColor("#d63fd9"));
    QVERIFY(image.save(imagePath));
    page.setEnvironment("canvas");
    page.setEnvironment("tiling");
    QTRY_COMPARE_WITH_TIMEOUT(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"), 3000);
    QCOMPARE(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    qputenv("DOCKPIPE_TEST_WALLPAPER", directory.filePath("missing.png").toUtf8());
    page.setEnvironment("tiling");
    page.setEnvironment("canvas");
    QTRY_VERIFY_WITH_TIMEOUT(!status->text().isEmpty(), 3000);
    QCOMPARE(pixelAt(canvas->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    QCOMPARE(pixelAt(tiling->grab().toImage(), QPoint(20, 20)), QColor("#d63fd9"));
    qInfo("Wallpaper lookup stayed asynchronous; stale results discarded, refresh and failure fallback passed.");
#endif
}

void DashboardPageTest::canvasGadgetsAndBrowserTearOut()
{
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1400, 1000);
    page.show();
    QVERIFY(page.addWidgetKind("send-input"));
    QVERIFY(page.addWidgetKind("system"));
    QVERIFY(page.addWidgetKind("browser"));
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    QVERIFY(canvas);
    drainEvents();
    auto *composer = canvas->findChild<SendInputWidget *>();
    QVERIFY(composer && composer->canvasPresentationEnabled());
    const auto definition = page.snapshot();
    const QString composerId = definition.panels.at(0).id;
    auto *composerFrame = canvas->findChild<CanvasPanel *>("dashboardPanel_" + composerId);
    QVERIFY(composerFrame && composerFrame->property("dashboardCapsule").toBool());
    QVERIFY(composerFrame->height() < 180);
    QVERIFY(composerFrame->findChild<QWidget *>("dashboardPanelClose")->isHidden());
    QVERIFY(composerFrame->findChild<QWidget *>("dashboardWidgetFooter")->isHidden());
    auto *source = canvas->findChild<BrowserWidget *>();
    QVERIFY(source && source->canvasViewportMode());
    auto *sourceFrame = canvas->findChild<CanvasPanel *>("dashboardPanel_" + definition.panels.at(2).id);
    QVERIFY(sourceFrame->findChild<QWidget *>("dashboardPanelHeader")->isHidden());
    QVERIFY(sourceFrame->findChild<QWidget *>("dashboardCanvasBrowserGrip")->isVisible());
    QVERIFY(sourceFrame->findChild<QWidget *>("dashboardPanelMenuButton")->isVisible());
    auto *view = source->activeView();
    QVERIFY(view);
    auto *profile = view->page()->profile();
    QTcpServer server;
    QVERIFY(server.listen(QHostAddress::LocalHost));
    connect(&server, &QTcpServer::newConnection, &server, [&server] {
        while (auto *socket = server.nextPendingConnection()) {
            QObject::connect(socket, &QTcpSocket::readyRead, socket, [socket] {
                socket->readAll();
                const QByteArray body("<title>Live canvas tab</title><p>Same live page after tear-out</p>");
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nConnection: close\r\nContent-Length: "
                    + QByteArray::number(body.size()) + "\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
            QObject::connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        }
    });
    source->navigate(QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort())));
    QTRY_COMPARE(view->title(), QString("Live canvas tab"));
    emit source->tabTearOutRequested(view, canvas->mapToGlobal(QPoint(620, 400)));
    QCOMPARE(page.panelCount(), 4);
    const QString detachedId = page.snapshot().panels.last().id;
    auto *detached = canvas->findChild<CanvasPanel *>("dashboardPanel_" + detachedId);
    auto *destination = detached ? detached->findChild<BrowserWidget *>() : nullptr;
    QVERIFY(destination && destination != source);
    QCOMPARE(destination->activeView(), view);
    QCOMPARE(view->page()->profile(), profile);
    QCOMPARE(view->title(), QString("Live canvas tab"));
    QVERIFY(captureIfRequested(page, "canvas-spatial-widgets"));
    page.setEnvironment("tiling");
    QVERIFY(page.addWidgetKind("send-input"));
    QVERIFY(page.addWidgetKind("browser"));
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    QVERIFY(!tiling->findChild<SendInputWidget *>()->canvasPresentationEnabled());
    QVERIFY(!tiling->findChild<BrowserWidget *>()->canvasViewportMode());
    QVERIFY(captureIfRequested(page, "canvas-spatial-normal-tiling"));
    page.setEnvironment("canvas");
    const int before = page.panelCount();
    page.setCanAddPanel([] { return false; });
    emit source->tabTearOutRequested(source->activeView(), canvas->mapToGlobal(QPoint(400, 300)));
    QCOMPARE(page.panelCount(), before);
    QCOMPARE(destination->activeView(), view);
}

void DashboardPageTest::fileScopesAndCanvasProjectPicker()
{
    QTemporaryDir firstProject;
    QTemporaryDir secondProject;
    QVERIFY(firstProject.isValid() && secondProject.isValid());
    QVERIFY(QDir().mkpath(secondProject.filePath("Documents")));
    QFile readme(secondProject.filePath("README.md"));
    QVERIFY(readme.open(QIODevice::WriteOnly));
    readme.write("# Dashboard fixture\n");
    readme.close();
    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(firstProject.path());
    DashboardPage page(emptyDashboard(), &factory);
    page.resize(1100, 780);
    page.show();
    QVERIFY(page.addWidgetKind("files"));
    auto *canvas = page.findChild<DashboardWorkspace *>("dashboardCanvasWorkspace");
    auto *tiling = page.findChild<DashboardWorkspace *>("dashboardTilingWorkspace");
    auto *canvasFiles = canvas->findChild<DashboardFilesWidget *>();
    QVERIFY(canvasFiles);
    QCOMPARE(canvasFiles->scope(), QString("project"));
    auto *canvasExplorer = canvasFiles->findChild<FileExplorerView *>();
    QVERIFY(canvasExplorer);
    canvasExplorer->setMode(FileExplorerView::Mode::Folders);
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(firstProject.path()).canonicalFilePath());
    auto *picker = page.findChild<QPushButton *>("dashboardCanvasProject");
    QVERIFY(picker && picker->isVisible());
    QSignalSpy projectRequested(&page, &DashboardPage::projectChangeRequested);
    picker->click();
    QCOMPARE(projectRequested.size(), 1);
    page.setEnvironment("tiling");
    QVERIFY(picker->isHidden());
    QVERIFY(page.addWidgetKind("files"));
    auto *systemFiles = tiling->findChild<DashboardFilesWidget *>();
    QVERIFY(systemFiles);
    QCOMPARE(systemFiles->scope(), QString("system"));
    auto *tilingExplorer = systemFiles->findChild<FileExplorerView *>();
    QVERIFY(tilingExplorer);
    tilingExplorer->setMode(FileExplorerView::Mode::Folders);
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    QVERIFY(systemFiles->navigateTo(firstProject.path()));
    factory.setWorkdir(secondProject.path());
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(secondProject.path()).canonicalFilePath());
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(firstProject.path()).canonicalFilePath());
    QVERIFY(picker->toolTip().contains(QDir::toNativeSeparators(secondProject.path())));
    auto options = page.snapshot().options;
    options.canvasFileScope = "system";
    options.tilingFileScope = "project";
    QVERIFY(page.applyOptions(options));
    QCOMPARE(systemFiles->scope(), QString("project"));
    QCOMPARE(systemFiles->currentDirectory(), QFileInfo(secondProject.path()).canonicalFilePath());
    QCOMPARE(canvasFiles->scope(), QString("system"));
    QCOMPARE(canvasFiles->currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    QVERIFY(captureIfRequested(page, "file-scopes-tiling"));
    // Capture synthetic fixture names rather than the operator's home contents.
    QVERIFY(canvasFiles->navigateTo(secondProject.path()));
    page.setEnvironment("canvas");
    QVERIFY(picker->isVisible());
    QCOMPARE(canvas->findChild<DashboardFilesWidget *>(), canvasFiles);
    QCOMPARE(tiling->findChild<DashboardFilesWidget *>(), systemFiles);
    QCOMPARE(canvasExplorer->mode(), FileExplorerView::Mode::Folders);
    QCOMPARE(tilingExplorer->mode(), FileExplorerView::Mode::Folders);
    QVERIFY(captureIfRequested(page, "file-scopes"));
}

int main(int argc, char **argv)
{
    QTemporaryDir config;
    if (!config.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("XDG_CACHE_HOME", config.path().toUtf8());
    qputenv("APPDATA", config.path().toUtf8());
    QApplication application(argc, argv);
    application.setOrganizationName("DockpipeDashboardPageTest");
    application.setApplicationName("DashboardPageFixture");
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    if (application.styleSheet().isEmpty())
        return 1;
    if (qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light") {
        QFile shared(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!shared.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 1;
        application.setStyleSheet({});
        application.setPalette(application.style()->standardPalette());
        application.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
    }
    DashboardPageTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardPageTest.moc"
