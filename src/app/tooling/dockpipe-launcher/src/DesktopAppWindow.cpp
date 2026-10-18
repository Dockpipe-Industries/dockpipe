#include "DesktopAppWindow.h"
#include <QCoreApplication>
#include <QFile>
#include <QGuiApplication>
#include <QWidget>
#include <QWindow>
#include <QPointer>
#include <QPainter>
#include <QPixmap>
#include <QSet>
#include <cstring>

#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
#include <QtGui/qguiapplication_platform.h>
#include <xcb/xcb.h>
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <X11/extensions/Xcomposite.h>
#include <X11/extensions/Xrender.h>

namespace {
// Xlib calls in this backend run only on the GUI thread, on our own connection.
// Catch expected BadWindow/BadPixmap races without changing Qt's XCB handling.
class XErrorScope {
public:
    explicit XErrorScope(Display *display) : m_display(display)
    {
        XSync(display, False);
        s_display = display;
        s_error = false;
        m_previous = XSetErrorHandler(handle);
        s_previous = m_previous;
    }
    bool failed()
    {
        XSync(m_display, False);
        return s_error;
    }
    ~XErrorScope()
    {
        XSync(m_display, False);
        XSetErrorHandler(m_previous);
        s_display = nullptr;
    }
private:
    static int handle(Display *display, XErrorEvent *event)
    {
        if (display == s_display) {
            s_error = true;
            return 0;
        }
        return s_previous ? s_previous(display, event) : 0;
    }
    Display *m_display;
    XErrorHandler m_previous;
    static inline Display *s_display = nullptr;
    static inline bool s_error = false;
    static inline XErrorHandler s_previous = nullptr;
};

QByteArray property(Display *display, Window window, const char *name, Atom type, long words, bool *valid = nullptr)
{
    Atom actual = 0;
    int format = 0;
    unsigned long count = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    const int result = XGetWindowProperty(display, window, XInternAtom(display, name, False),
        0, words, False, type, &actual, &format, &count, &remaining, &data);
    QByteArray value;
    if (valid)
        *valid = result == Success && actual == type && format == 32 && !remaining;
    if (result == Success && actual == type && !remaining && data) {
        const int itemBytes = format == 32 ? sizeof(long) : format / 8;
        value = QByteArray(reinterpret_cast<char *>(data), int(count) * itemBytes);
    }
    if (data)
        XFree(data);
    return value;
}

unsigned long processId(Display *display, Window window)
{
    const auto bytes = property(display, window, "_NET_WM_PID", XA_CARDINAL, 1);
    unsigned long pid = 0;
    if (bytes.size() == sizeof(pid))
        memcpy(&pid, bytes.constData(), sizeof(pid));
    return pid;
}

QByteArray processStart(unsigned long pid)
{
    QFile stat(QStringLiteral("/proc/%1/stat").arg(pid));
    if (!pid || !stat.open(QIODevice::ReadOnly))
        return {};
    const auto data = stat.read(4096);
    if (data.size() == 4096)
        return {};
    const int commandEnd = data.lastIndexOf(')');
    if (commandEnd < 0)
        return {};
    const auto fields = data.mid(commandEnd + 2).simplified().split(' ');
    // Fields after the parenthesized command start at field 3; starttime is 22.
    return fields.value(19);
}

struct DesktopProperty {
    Atom name = None;
    Atom type = None;
    QByteArray data;
};

bool desktopProperties(Display *display, Window window, QVector<DesktopProperty> &properties)
{
    // Only the ICCCM/EWMH hints affected by embedding. Fixed protocol sizes
    // bound each read; lists allow at most 32 atoms, never arbitrary app data.
    const struct { const char *name; long words; } hints[] = {
        {"_MOTIF_WM_HINTS", 5}, {"WM_NORMAL_HINTS", 18},
        {"WM_TRANSIENT_FOR", 1}, {"_NET_WM_WINDOW_TYPE", 32},
        {"_NET_WM_STATE", 32}, {"_NET_WM_DESKTOP", 1}
    };
    properties.clear();
    for (const auto &hint : hints) {
        DesktopProperty saved;
        saved.name = XInternAtom(display, hint.name, False);
        int format = 0;
        unsigned long count = 0;
        unsigned long remaining = 0;
        unsigned char *data = nullptr;
        const int status = XGetWindowProperty(display, window, saved.name, 0,
            hint.words, False, AnyPropertyType, &saved.type, &format, &count, &remaining, &data);
        const bool valid = status == Success && !remaining && (saved.type == None || format == 32);
        if (valid && data && saved.type != None)
            saved.data = QByteArray(reinterpret_cast<char *>(data), count * sizeof(long));
        if (data)
            XFree(data);
        if (!valid) {
            properties.clear();
            return false;
        }
        properties.append(saved);
    }
    return true;
}
}
#endif

struct DesktopAppWindow::Native {
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    Display *display = nullptr;
    bool redirected = false;
    unsigned long pid = 0;
    QByteArray processStart;
    QString applicationClass;
    QSet<quint64> existingWindows;
    QRect originalGeometry;
    QVector<DesktopProperty> originalProperties;
    QPointer<QWindow> foreign;
    QPointer<QWidget> container;
    bool saved = false;
#endif
};

QString DesktopAppWindow::unavailableReason()
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (QGuiApplication::platformName() != "xcb")
        return QObject::tr("App billboards currently require a Linux X11 session. Wayland capture and control are not supported yet.");
    if (QFile::exists("/.flatpak-info"))
        return QObject::tr("App billboards are unavailable in the Flatpak sandbox.");
    auto *native = qGuiApp->nativeInterface<QNativeInterface::QX11Application>();
    if (!native)
        return QObject::tr("This Qt build cannot embed X11 app windows.");
    int event = 0;
    int error = 0;
    if (!XCompositeQueryExtension(native->display(), &event, &error)
        || !XRenderQueryExtension(native->display(), &event, &error))
        return QObject::tr("The desktop does not provide Composite and Render support.");
    return {};
#else
    return QObject::tr("App billboards are unavailable in this build. The experiment requires Linux X11 support.");
#endif
}

QVector<DesktopAppChoice> DesktopAppWindow::availableWindows(QString &error)
{
    error = unavailableReason();
    QVector<DesktopAppChoice> choices;
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!error.isEmpty())
        return choices;
    Display *display = XOpenDisplay(nullptr);
    if (!display) {
        error = QObject::tr("Cannot connect to the desktop.");
        return choices;
    }
    {
        XErrorScope errors(display);
        // The picker is intentionally bounded; never recursively inspect windows.
        bool validClients = false;
        const auto clients = property(display, DefaultRootWindow(display), "_NET_CLIENT_LIST", XA_WINDOW, 256, &validClients);
        if (!validClients || errors.failed())
            error = QObject::tr("Cannot read the desktop window list, or it exceeds 256 entries.");
        for (int offset = 0; offset + int(sizeof(Window)) <= clients.size(); offset += sizeof(Window)) {
            Window window = 0;
            memcpy(&window, clients.constData() + offset, sizeof(window));
            XWindowAttributes attributes{};
            if (!XGetWindowAttributes(display, window, &attributes) || attributes.map_state != IsViewable
                || attributes.override_redirect || attributes.c_class != InputOutput)
                continue;
            const auto pid = processId(display, window);
            if (!pid || pid == static_cast<unsigned long>(QCoreApplication::applicationPid()))
                continue;
            const auto types = property(display, window, "_NET_WM_WINDOW_TYPE", XA_ATOM, 16);
            const Atom normal = XInternAtom(display, "_NET_WM_WINDOW_TYPE_NORMAL", False);
            bool normalWindow = types.isEmpty();
            for (int index = 0; index + int(sizeof(Atom)) <= types.size(); index += sizeof(Atom)) {
                Atom type = 0;
                memcpy(&type, types.constData() + index, sizeof(type));
                normalWindow |= type == normal;
            }
            if (!normalWindow)
                continue;
            QString title = QString::fromUtf8(property(display, window, "_NET_WM_NAME",
                XInternAtom(display, "UTF8_STRING", False), 256)).simplified();
            // Older X11 clients (including RetroArch) only publish ICCCM WM_NAME.
            // Keep the same 1 KiB property limit and prefer the modern UTF-8 title.
            if (title.isEmpty())
                title = QString::fromLatin1(property(display, window, "WM_NAME", XA_STRING, 256)).simplified();
            if (!title.isEmpty()) {
                const auto windowClass = property(display, window, "WM_CLASS", XA_STRING, 256).split('\0');
                const QString applicationClass = windowClass.size() > 1 ? QString::fromUtf8(windowClass[1]) : QString();
                choices.append({window, title, applicationClass});
            }
        }
    }
    XCloseDisplay(display);
#endif
    return choices;
}

DesktopAppWindow::DesktopAppWindow(const DesktopAppChoice &choice)
    : m_native(std::make_unique<Native>()), m_choice(choice)
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (unavailableReason().isEmpty()) {
        m_native->display = XOpenDisplay(nullptr);
        if (m_native->display) {
            XErrorScope errors(m_native->display);
            m_native->pid = processId(m_native->display, choice.id);
            m_native->processStart = processStart(m_native->pid);
            const auto classes = property(m_native->display, choice.id, "WM_CLASS", XA_STRING, 256).split('\0');
            m_native->applicationClass = QString::fromUtf8(classes.value(1));
            const auto clients = property(m_native->display, DefaultRootWindow(m_native->display),
                "_NET_CLIENT_LIST", XA_WINDOW, 256);
            for (int offset = 0; offset + int(sizeof(Window)) <= clients.size(); offset += sizeof(Window)) {
                Window window = 0;
                memcpy(&window, clients.constData() + offset, sizeof(window));
                m_native->existingWindows.insert(window);
            }
        }
    }
#endif
}

std::optional<DesktopAppChoice> DesktopAppWindow::replacement(QString &error) const
{
    error.clear();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->display || m_native->processStart.isEmpty()
        || processStart(m_native->pid) != m_native->processStart) {
        error = QObject::tr("The selected application has exited.");
        return {};
    }
    if (alive())
        return {};
    if (m_native->applicationClass.isEmpty()) {
        error = QObject::tr("The application has no window identity. Select its new window manually.");
        return {};
    }
    const auto windows = availableWindows(error);
    if (!error.isEmpty())
        return {};
    std::optional<DesktopAppChoice> match;
    for (const auto &candidate : windows) {
        if (candidate.id == m_choice.id || m_native->existingWindows.contains(candidate.id)
            || candidate.applicationClass.compare(m_native->applicationClass, Qt::CaseInsensitive) != 0)
            continue;
        XErrorScope errors(m_native->display);
        if (processId(m_native->display, candidate.id) != m_native->pid || errors.failed())
            continue;
        if (match) {
            error = QObject::tr("Several replacement windows match. Select the application's window manually.");
            return {};
        }
        match = candidate;
    }
    return match;
#else
    error = unavailableReason();
    return {};
#endif
}

DesktopAppWindow::~DesktopAppWindow()
{
    releaseControl();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (m_native->display) {
        {
            XErrorScope errors(m_native->display);
            if (m_native->redirected)
                XCompositeUnredirectWindow(m_native->display, m_choice.id, CompositeRedirectAutomatic);
        }
        XCloseDisplay(m_native->display);
    }
#endif
}

bool DesktopAppWindow::alive() const
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->display || !m_native->pid)
        return false;
    XErrorScope errors(m_native->display);
    return processId(m_native->display, m_choice.id) == m_native->pid && !errors.failed();
#else
    return false;
#endif
}

QImage DesktopAppWindow::capture(QString &error)
{
    error.clear();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!alive()) {
        error = QObject::tr("Window closed. Choose it again before the next flight.");
        return {};
    }
    Display *display = m_native->display;
    XErrorScope errors(display);
    XWindowAttributes attributes{};
    if (!XGetWindowAttributes(display, m_choice.id, &attributes) || attributes.map_state != IsViewable) {
        error = QObject::tr("Window is minimized or unavailable.");
        return {};
    }
    // Source remains server-side. Read back only a 1280 x 720 texture (~3.6 MiB).
    // Reject oversized surfaces before naming a pixmap: at most an 8K desktop.
    if (attributes.width <= 0 || attributes.height <= 0 || attributes.width > 7680 || attributes.height > 4320) {
        error = QObject::tr("Window exceeds the 7680 × 4320 capture limit.");
        return {};
    }
    if (!m_native->redirected) {
        XCompositeRedirectWindow(display, m_choice.id, CompositeRedirectAutomatic);
        if (errors.failed()) {
            error = QObject::tr("The desktop refused window capture.");
            return {};
        }
        m_native->redirected = true;
    }
    const Pixmap source = XCompositeNameWindowPixmap(display, m_choice.id);
    if (errors.failed()) {
        error = QObject::tr("This window cannot be streamed.");
        return {};
    }
    const QSize size = QSize(attributes.width, attributes.height).scaled(1280, 720, Qt::KeepAspectRatio);
    auto *format = XRenderFindVisualFormat(display, attributes.visual);
    auto *outputFormat = XRenderFindStandardFormat(display, PictStandardRGB24);
    if (!format || !outputFormat) {
        XFreePixmap(display, source);
        error = QObject::tr("Unsupported window pixel format.");
        return {};
    }
    const Pixmap output = XCreatePixmap(display, DefaultRootWindow(display), size.width(), size.height(), 24);
    const Picture inputPicture = XRenderCreatePicture(display, source, format, 0, nullptr);
    const Picture outputPicture = XRenderCreatePicture(display, output, outputFormat, 0, nullptr);
    XTransform transform{};
    transform.matrix[0][0] = XDoubleToFixed(double(attributes.width) / size.width());
    transform.matrix[1][1] = XDoubleToFixed(double(attributes.height) / size.height());
    transform.matrix[2][2] = XDoubleToFixed(1);
    XRenderSetPictureTransform(display, inputPicture, &transform);
    XRenderSetPictureFilter(display, inputPicture, FilterBilinear, nullptr, 0);
    XRenderComposite(display, PictOpSrc, inputPicture, 0, outputPicture, 0, 0, 0, 0, 0, 0, size.width(), size.height());
    XImage *pixels = nullptr;
    if (!errors.failed())
        pixels = XGetImage(display, output, 0, 0, size.width(), size.height(), AllPlanes, ZPixmap);
    QImage image;
    if (pixels && !errors.failed()) {
        image = QImage(size, QImage::Format_RGB32);
        for (int y = 0; y < size.height(); ++y) {
            auto *row = reinterpret_cast<QRgb *>(image.scanLine(y));
            for (int x = 0; x < size.width(); ++x) {
                const unsigned long pixel = XGetPixel(pixels, x, y);
                row[x] = qRgb((pixel >> 16) & 255, (pixel >> 8) & 255, pixel & 255);
            }
        }
    }
    if (pixels)
        XDestroyImage(pixels);
    XRenderFreePicture(display, inputPicture);
    XRenderFreePicture(display, outputPicture);
    XFreePixmap(display, output);
    XFreePixmap(display, source);
    if (image.isNull())
        error = QObject::tr("Window capture failed. Stop sharing and select the window again.");
    if (image.isNull())
        return {};
    QImage preview(1280, 720, QImage::Format_RGB32);
    preview.fill(QColor("#101d28"));
    QPainter painter(&preview);
    painter.drawImage(QPoint((preview.width() - image.width()) / 2, (preview.height() - image.height()) / 2), image);
    return preview;
#else
    error = unavailableReason();
    return {};
#endif
}

bool DesktopAppWindow::prepareControl(QString &error)
{
    error.clear();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (m_native->saved)
        return true;
    if (!alive()) {
        error = QObject::tr("The selected window has closed.");
        return false;
    }
    if (m_native->redirected) {
        XErrorScope errors(m_native->display);
        XCompositeUnredirectWindow(m_native->display, m_choice.id, CompositeRedirectAutomatic);
        if (errors.failed()) {
            error = QObject::tr("Cannot release the preview surface for app control.");
            return false;
        }
        m_native->redirected = false;
    }
    auto *native = qGuiApp->nativeInterface<QNativeInterface::QX11Application>();
    XWindowAttributes attributes{};
    int x = 0;
    int y = 0;
    Window child = 0;
    {
        XErrorScope errors(m_native->display);
        if (!XGetWindowAttributes(m_native->display, m_choice.id, &attributes)
            || !XTranslateCoordinates(m_native->display, m_choice.id, DefaultRootWindow(m_native->display), 0, 0, &x, &y, &child)
            || !desktopProperties(m_native->display, m_choice.id, m_native->originalProperties)
            || errors.failed()) {
            error = QObject::tr("Cannot read the selected window.");
            return false;
        }
    }
    m_native->originalGeometry = QRect(x, y, attributes.width, attributes.height);
    // Use Qt's connection: if Dockpipe crashes, the server reparents the foreign
    // window before destroying Qt's native container. A separate connection's
    // save-set would not protect it from this client's teardown.
    auto *connection = native->connection();
    auto *failure = xcb_request_check(connection, xcb_change_save_set_checked(connection, XCB_SET_MODE_INSERT, m_choice.id));
    if (failure) {
        free(failure);
        error = QObject::tr("Cannot protect the app window during embedding.");
        return false;
    }
    m_native->saved = true;
    // ICCCM withdrawal lets the compositor retire its desktop surface before
    // the client is attached to a different hierarchy. The caller polls for
    // acknowledgment with a bounded timer; never block the GUI waiting for a WM.
    {
        XErrorScope errors(m_native->display);
        XWithdrawWindow(m_native->display, m_choice.id, DefaultScreen(m_native->display));
        if (errors.failed()) {
            error = QObject::tr("The desktop refused to release this window.");
            return false;
        }
    }
    return true;
#else
    error = unavailableReason();
    return false;
#endif
}

bool DesktopAppWindow::controlReady() const
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->saved || !alive())
        return false;
    XErrorScope errors(m_native->display);
    const Atom stateAtom = XInternAtom(m_native->display, "WM_STATE", False);
    const auto bytes = property(m_native->display, m_choice.id, "WM_STATE", stateAtom, 2);
    unsigned long state = WithdrawnState;
    if (bytes.size() >= int(sizeof(state)))
        memcpy(&state, bytes.constData(), sizeof(state));
    Window root = 0;
    Window parent = 0;
    Window *children = nullptr;
    unsigned int count = 0;
    const bool known = XQueryTree(m_native->display, m_choice.id, &root, &parent, &children, &count);
    if (children)
        XFree(children);
    return known && state == WithdrawnState && parent == root && !errors.failed();
#else
    return false;
#endif
}

QWidget *DesktopAppWindow::takeControl(QWidget *parent, QString &error)
{
    error.clear();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (m_native->container)
        return m_native->container;
    if (!controlReady()) {
        error = QObject::tr("The desktop has not released this window yet.");
        return nullptr;
    }
    m_native->foreign = QWindow::fromWinId(m_choice.id);
    if (!m_native->foreign) {
        releaseControl();
        error = QObject::tr("Qt could not embed this app window.");
        return nullptr;
    }
    m_native->container = QWidget::createWindowContainer(m_native->foreign, parent);
    m_native->container->setFocusPolicy(Qt::StrongFocus);
    m_native->container->setObjectName("billboardNativeApp");
    return m_native->container;
#else
    Q_UNUSED(parent);
    error = unavailableReason();
    return nullptr;
#endif
}

bool DesktopAppWindow::controlAttached() const
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->saved || !m_native->container || !m_native->foreign || !alive())
        return false;
    XErrorScope errors(m_native->display);
    XWindowAttributes attributes{};
    if (!XGetWindowAttributes(m_native->display, m_choice.id, &attributes)
        || attributes.map_state != IsViewable)
        return false;
    // Qt may attach the foreign QWindow directly to the host's native window
    // while the QWidget container remains non-native. Do not force winId() on
    // that container: creating another native window changes the hierarchy.
    const Window host = m_native->container->window()->winId();
    Window ancestor = m_choice.id;
    // Inspect only the parent chain, never the application's children. Thirty
    // two native ancestors covers the HUD hierarchy and bounds malformed trees.
    for (int depth = 0; depth < 32; ++depth) {
        Window root = 0;
        Window parent = 0;
        Window *children = nullptr;
        unsigned int count = 0;
        const bool known = XQueryTree(m_native->display, ancestor, &root, &parent, &children, &count);
        if (children)
            XFree(children);
        if (!known || errors.failed())
            return false;
        if (parent == host)
            return true;
        if (!parent || parent == root || parent == ancestor)
            return false;
        ancestor = parent;
    }
#endif
    return false;
}

bool DesktopAppWindow::syncControlGeometry()
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!controlAttached())
        return false;
    QWidget *container = m_native->container;
    QWidget *host = container->window();
    const QPoint offset = container->mapTo(host, QPoint());
    const qreal scale = host->devicePixelRatioF();
    const int width = qMax(1, qRound(container->width() * scale));
    const int height = qMax(1, qRound(container->height() * scale));
    XErrorScope errors(m_native->display);
    Window root = 0;
    Window parent = 0;
    Window *children = nullptr;
    unsigned int count = 0;
    const bool known = XQueryTree(m_native->display, m_choice.id, &root, &parent, &children, &count);
    if (children)
        XFree(children);
    if (!known || !parent || errors.failed())
        return false;
    int x = 0;
    int y = 0;
    Window child = 0;
    if (!XTranslateCoordinates(m_native->display, host->winId(), parent,
            qRound(offset.x() * scale), qRound(offset.y() * scale), &x, &y, &child))
        return false;
    XWindowAttributes attributes{};
    if (!XGetWindowAttributes(m_native->display, m_choice.id, &attributes))
        return false;
    // Foreign apps can restore their own desktop geometry after reparenting.
    // Keep the native surface in the HUD without creating extra native widgets.
    if (attributes.x != x || attributes.y != y || attributes.width != width || attributes.height != height)
        XMoveResizeWindow(m_native->display, m_choice.id, x, y, width, height);
    return !errors.failed();
#else
    return false;
#endif
}

void DesktopAppWindow::releaseControl()
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->saved)
        return;
    auto *native = qGuiApp->nativeInterface<QNativeInterface::QX11Application>();
    auto *connection = native->connection();
    // Keep the client unmapped until its desktop hints are restored. Reparenting
    // a mapped client can let the WM frame it with Qt's temporary embed hints.
    auto *failure = xcb_request_check(connection, xcb_unmap_window_checked(connection, m_choice.id));
    free(failure);
    if (m_native->foreign)
        m_native->foreign->setParent(nullptr);
    delete m_native->container.data();
    m_native->container = nullptr;
    // Some Qt versions retain wrapper ownership after native detachment.
    delete m_native->foreign.data();
    m_native->foreign = nullptr;

    // The checked request also orders Qt's teardown before our separate Xlib
    // connection restores properties. An absent original property must remain
    // absent, not inherit a default written by the foreign QWindow wrapper.
    const auto geometry = m_native->originalGeometry;
    failure = xcb_request_check(connection, xcb_reparent_window_checked(connection,
        m_choice.id, DefaultRootWindow(m_native->display), geometry.x(), geometry.y()));
    free(failure); // BadWindow is expected when the application has closed.
    {
        XErrorScope errors(m_native->display);
        for (const auto &saved : m_native->originalProperties) {
            if (saved.type == None) {
                XDeleteProperty(m_native->display, m_choice.id, saved.name);
            } else {
                XChangeProperty(m_native->display, m_choice.id, saved.name, saved.type, 32,
                    PropModeReplace, reinterpret_cast<const unsigned char *>(saved.data.constData()),
                    saved.data.size() / sizeof(long));
            }
        }
        XMoveResizeWindow(m_native->display, m_choice.id, geometry.x(), geometry.y(), geometry.width(), geometry.height());
        XMapWindow(m_native->display, m_choice.id);
    }
    failure = xcb_request_check(connection, xcb_change_save_set_checked(connection, XCB_SET_MODE_DELETE, m_choice.id));
    free(failure); // BadWindow is expected if the user closed the embedded app.
    m_native->saved = false;
    m_native->originalProperties.clear();
#endif
}

bool DesktopAppWindow::requestClose(QString &error)
{
    error.clear();
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!alive()) {
        error = QObject::tr("The desktop window is no longer available.");
        return false;
    }
    XErrorScope errors(m_native->display);
    const Atom deleteWindow = XInternAtom(m_native->display, "WM_DELETE_WINDOW", False);
    // Query only the bounded protocol property; never terminate the process if
    // the app does not support a cooperative window-close request.
    bool validProtocols = false;
    const QByteArray protocols = property(m_native->display, m_choice.id, "WM_PROTOCOLS", XA_ATOM, 256, &validProtocols);
    bool supportsClose = false;
    for (qsizetype offset = 0; offset + qsizetype(sizeof(Atom)) <= protocols.size(); offset += sizeof(Atom)) {
        Atom protocol = 0;
        std::memcpy(&protocol, protocols.constData() + offset, sizeof(Atom));
        supportsClose |= protocol == deleteWindow;
    }
    if (!validProtocols || !supportsClose) {
        error = QObject::tr("This app does not support a normal window-close request. Close it using its own controls.");
        return false;
    }
    XEvent event{};
    event.xclient.type = ClientMessage;
    event.xclient.window = m_choice.id;
    event.xclient.message_type = XInternAtom(m_native->display, "WM_PROTOCOLS", False);
    event.xclient.format = 32;
    event.xclient.data.l[0] = deleteWindow;
    event.xclient.data.l[1] = CurrentTime;
    const bool sent = XSendEvent(m_native->display, m_choice.id, False, NoEventMask, &event);
    if (!sent || errors.failed()) {
        error = QObject::tr("Could not send a close request to this desktop window.");
        return false;
    }
    return true;
#else
    error = unavailableReason();
    return false;
#endif
}

bool DesktopAppWindow::focusControl()
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!m_native->container || !m_native->container->isVisible() || !alive())
        return false;
    auto *native = qGuiApp->nativeInterface<QNativeInterface::QX11Application>();
    auto *connection = native->connection();
    auto *failure = xcb_request_check(connection, xcb_set_input_focus_checked(connection,
        XCB_INPUT_FOCUS_PARENT, m_choice.id, XCB_CURRENT_TIME));
    const bool succeeded = !failure;
    free(failure);
    return succeeded;
#else
    return false;
#endif
}

QIcon DesktopAppWindow::applicationIcon() const
{
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    if (!alive())
        return {};
    XErrorScope errors(m_native->display);
    // Limit untrusted icon properties to 1 MiB of wire-format CARDINAL data.
    const auto bytes = property(m_native->display, m_choice.id, "_NET_WM_ICON", XA_CARDINAL, 262144);
    if (!errors.failed() && bytes.size() % sizeof(unsigned long) == 0) {
        const qsizetype count = bytes.size() / sizeof(unsigned long);
        const auto word = [&](qsizetype index) {
            unsigned long value = 0;
            memcpy(&value, bytes.constData() + index * sizeof(value), sizeof(value));
            return value;
        };
        qsizetype offset = 0;
        QImage best;
        while (offset + 2 <= count) {
            const unsigned long width = word(offset++);
            const unsigned long height = word(offset++);
            if (!width || !height || width > 512 || height > 512
                || width * height > static_cast<unsigned long>(count - offset))
                break;
            if (best.isNull() || (width >= 32 && width < static_cast<unsigned long>(best.width()))
                || (best.width() < 32 && width > static_cast<unsigned long>(best.width()))) {
                QImage candidate(int(width), int(height), QImage::Format_ARGB32);
                if (candidate.isNull())
                    break;
                for (unsigned long y = 0; y < height; ++y) {
                    auto *line = reinterpret_cast<QRgb *>(candidate.scanLine(int(y)));
                    for (unsigned long x = 0; x < width; ++x)
                        line[x] = QRgb(word(offset + qsizetype(y * width + x)));
                }
                best = candidate;
            }
            offset += qsizetype(width * height);
        }
        if (!best.isNull())
            return QIcon(QPixmap::fromImage(best));
    }
#endif
    QIcon icon = QIcon::fromTheme(m_choice.applicationClass);
    if (icon.isNull())
        icon = QIcon::fromTheme(m_choice.applicationClass.toLower());
    return icon;
}
