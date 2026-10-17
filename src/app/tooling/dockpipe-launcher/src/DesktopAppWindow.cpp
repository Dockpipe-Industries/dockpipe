#include "DesktopAppWindow.h"
#include <QCoreApplication>
#include <QFile>
#include <QGuiApplication>
#include <QWidget>
#include <QWindow>
#include <QPointer>
#include <QPainter>

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

QByteArray property(Display *display, Window window, const char *name, Atom type, long words)
{
    Atom actual = 0;
    int format = 0;
    unsigned long count = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    const int result = XGetWindowProperty(display, window, XInternAtom(display, name, False),
        0, words, False, type, &actual, &format, &count, &remaining, &data);
    QByteArray value;
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
}
#endif

struct DesktopAppWindow::Native {
#if defined(DOCKPIPE_X11_BILLBOARDS) && QT_CONFIG(xcb)
    Display *display = nullptr;
    bool redirected = false;
    unsigned long pid = 0;
    QRect originalGeometry;
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
        const auto clients = property(display, DefaultRootWindow(display), "_NET_CLIENT_LIST", XA_WINDOW, 256);
        if (clients.isEmpty())
            error = QObject::tr("No selectable windows, or the desktop window list exceeds 256 entries.");
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
            const auto title = property(display, window, "_NET_WM_NAME", XInternAtom(display, "UTF8_STRING", False), 256);
            if (!title.isEmpty()) {
                const auto windowClass = property(display, window, "WM_CLASS", XA_STRING, 256).split('\0');
                const QString applicationClass = windowClass.size() > 1 ? QString::fromUtf8(windowClass[1]) : QString();
                choices.append({window, QString::fromUtf8(title).simplified(), applicationClass});
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
        }
    }
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
    // Detach before destroying the container, including cancellation while the
    // window manager is still processing withdrawal.
    if (m_native->foreign)
        m_native->foreign->setParent(nullptr);
    auto *native = qGuiApp->nativeInterface<QNativeInterface::QX11Application>();
    auto *connection = native->connection();
    const auto geometry = m_native->originalGeometry;
    auto *failure = xcb_request_check(connection, xcb_reparent_window_checked(connection,
        m_choice.id, DefaultRootWindow(m_native->display), geometry.x(), geometry.y()));
    free(failure); // BadWindow is expected when the application has closed.
    delete m_native->container.data();
    m_native->container = nullptr;
    // Some Qt versions retain wrapper ownership after native detachment.
    // QPointer handles either ownership outcome without deleting it twice.
    delete m_native->foreign.data();
    m_native->foreign = nullptr;
    {
        XErrorScope errors(m_native->display);
        const auto geometry = m_native->originalGeometry;
        XMoveResizeWindow(m_native->display, m_choice.id, geometry.x(), geometry.y(), geometry.width(), geometry.height());
        XMapWindow(m_native->display, m_choice.id);
    }
    failure = xcb_request_check(connection, xcb_change_save_set_checked(connection, XCB_SET_MODE_DELETE, m_choice.id));
    free(failure); // BadWindow is expected if the user closed the embedded app.
    m_native->saved = false;
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
