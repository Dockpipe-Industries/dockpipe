#include "BrowserNetworkHost.h"
#include "BrowserNetworkTrust.h"
#include "BrowserWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QAuthenticator>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QNetworkProxy>
#include <QProcess>
#include <QRegularExpression>
#include <QSocketNotifier>
#include <QTextStream>
#include <QWebEnginePage>
#include <QWindow>
#include <QEvent>
#include <QScopedValueRollback>
#include <cstring>
#if defined(Q_OS_UNIX)
#include <fcntl.h>
#include <sys/stat.h>
#include <unistd.h>
#endif
#if defined(Q_OS_LINUX)
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#undef FocusIn
#endif

namespace {
#if defined(DOCKPIPE_BROWSER_SDK_HAS_ISOLATED_SESSIONS)
// A QWidget hosting WebEngine may recreate its native surface when its first
// render widget enables RHI composition. Export a stable QWindow instead; keep
// every replacement QWidget surface parented below that window in this process.
class EmbeddedBrowserSurface final : public QObject {
public:
    EmbeddedBrowserSurface(QWindow &surface, BrowserWidget &browser)
        : m_surface(surface), m_browser(browser)
    {
        // Native events can land on the plain exported QWindow while mouse
        // events reach a child QWidget. Neither implicitly activates the other
        // focus hierarchy across the foreign-window boundary.
        qApp->installEventFilter(this);
        // Qt clears QWidget activation after delivering native FocusIn when
        // it cannot map the plain exported root back to a top-level widget.
        // This signal follows that lookup, so restore only our own focus tree.
        connect(qGuiApp, &QGuiApplication::focusWindowChanged, this, [this](QWindow *window) {
            if (window == &m_surface || ownsBrowserFocus(window))
                activateBrowser(false);
        });
    }

    void synchronize()
    {
        if (m_synchronizing || !m_browser.internalWinId() || !m_browser.windowHandle())
            return;
        QScopedValueRollback<bool> guard(m_synchronizing, true);
        m_browser.windowHandle()->setParent(&m_surface);
        m_browser.setGeometry(QRect(QPoint(), m_surface.size()));
    }

protected:
    bool eventFilter(QObject *object, QEvent *event) override
    {
        if ((object == &m_browser && event->type() == QEvent::WinIdChange)
            || (object == &m_surface && event->type() == QEvent::Resize))
            synchronize();
        if (object == &m_surface && event->type() == QEvent::FocusIn)
            activateBrowser(true);
        if (event->type() == QEvent::MouseButtonPress) {
            const auto *widget = qobject_cast<QWidget *>(object);
            // Native dialogs (including DevTools) own their own activation.
            if (widget && widget->window() == &m_browser)
                activateBrowser(true);
        }
        return false;
    }

private:
    bool ownsBrowserFocus(QWindow *window) const
    {
        for (; window; window = window->parent()) {
            if (window == m_browser.windowHandle())
                return true;
        }
        return false;
    }

    void activateBrowser(bool requestNativeFocus)
    {
        if (m_activating || !m_browser.isVisible() || !m_browser.windowHandle())
            return;
        QScopedValueRollback<bool> guard(m_activating, true);
        // Qt 6's active-widget lookup climbs to the plain stable QWindow and
        // cannot map it to this nested QWidget. Native activation alone leaves
        // widget shortcuts and caret focus inactive across this boundary.
        QT_WARNING_PUSH
        QT_WARNING_DISABLE_DEPRECATED
        QApplication::setActiveWindow(&m_browser);
        QT_WARNING_POP
        if (requestNativeFocus) {
            m_browser.windowHandle()->requestActivate();
#if defined(Q_OS_LINUX)
            // Qt XCB intentionally does not activate embedded windows and its
            // XEmbed focus request stops at our plain stable QWindow. Complete
            // this explicit user-focus handoff only to our mapped QWidget root.
            Display *display = XOpenDisplay(nullptr);
            if (display) {
                const Window id = Window(m_browser.windowHandle()->winId());
                XWindowAttributes attributes{};
                if (XGetWindowAttributes(display, id, &attributes) && attributes.map_state == IsViewable) {
                    XSetInputFocus(display, id, RevertToParent, CurrentTime);
                    XFlush(display);
                }
                XCloseDisplay(display);
            }
#endif
        }
    }

    QWindow &m_surface;
    BrowserWidget &m_browser;
    bool m_synchronizing = false;
    bool m_activating = false;
};

QSize nativeWindowSize(quint64 id)
{
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return {};
    XWindowAttributes attributes{};
    const auto valid = XGetWindowAttributes(display, Window(id), &attributes);
    XCloseDisplay(display);
    return valid ? QSize(attributes.width, attributes.height) : QSize();
}

bool isOwnedNativeParent(quint64 browserId, quint64 parentId, qint64 parentPid)
{
    if (!parentId || parentPid != ::getppid())
        return false;
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return false;
    Window root = 0;
    Window actualParent = 0;
    Window *children = nullptr;
    unsigned int count = 0;
    const bool reparented = XQueryTree(display, Window(browserId), &root, &actualParent,
        &children, &count) && actualParent == Window(parentId) && actualParent != root;
    if (children)
        XFree(children);
    const Atom property = XInternAtom(display, "_NET_WM_PID", True);
    Atom type = 0;
    int format = 0;
    unsigned long length = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    bool owned = false;
    if (reparented && property && XGetWindowProperty(display, Window(parentId), property,
            0, 1, False, XA_CARDINAL, &type, &format, &length, &remaining, &data) == Success) {
        owned = type == XA_CARDINAL && format == 32 && length == 1 && data
            && *reinterpret_cast<unsigned long *>(data) == static_cast<unsigned long>(parentPid);
    }
    if (data)
        XFree(data);
    XCloseDisplay(display);
    return owned;
}
#endif

QJsonObject readPrivateConfig(const QString &path)
{
#if defined(Q_OS_UNIX)
    const auto name = QFile::encodeName(path);
    const int descriptor = ::open(name.constData(), O_RDONLY | O_NOFOLLOW | O_CLOEXEC);
    if (descriptor < 0)
        return {};
    struct stat status{};
    if (::fstat(descriptor, &status) || !S_ISREG(status.st_mode) || status.st_uid != ::getuid()
        || (status.st_mode & 0077) || status.st_size < 1 || status.st_size > 4 * 1024 * 1024) {
        ::close(descriptor);
        return {};
    }
    QFile file;
    if (!file.open(descriptor, QIODevice::ReadOnly, QFileDevice::AutoCloseHandle)) {
        ::close(descriptor);
        return {};
    }
    return QJsonDocument::fromJson(file.read(4 * 1024 * 1024 + 1)).object();
#else
    Q_UNUSED(path);
    return {};
#endif
}
}

bool browserNetworkHostSupportsIsolatedSessions()
{
#if defined(DOCKPIPE_BROWSER_SDK_HAS_ISOLATED_SESSIONS)
    return true;
#else
    return false;
#endif
}

std::optional<int> dispatchBrowserNetworkHost(int argc, char **argv)
{
    bool requested = false;
    for (int index = 1; index < argc; ++index)
        requested |= std::strcmp(argv[index], "--browser-network-host") == 0;
    if (!requested)
        return std::nullopt;
    if (argc != 4 || std::strcmp(argv[1], "--browser-network-host")
        || std::strcmp(argv[2], "--request"))
        return 2;
    const auto config = readPrivateConfig(QString::fromLocal8Bit(argv[3]));
    const QUrl endpoint(config.value("endpoint").toString());
    const QString password = config.value("password").toString();
    const QString storage = config.value("storage").toString();
    const bool hasTrust = config.contains("tls_trust");
    if (config.size() != (hasTrust ? 7 : 5) || config.value("schema") != "dockpipe.browser.host/v1"
        || endpoint.scheme() != "http" || endpoint.host() != "127.0.0.1"
        || endpoint.port() < 1 || endpoint.port() > 65535 || !endpoint.userInfo().isEmpty()
        || !endpoint.path().isEmpty() || endpoint.hasQuery() || endpoint.hasFragment()
        || config.value("username") != "dockpipe"
        || !QRegularExpression("\\A[0-9a-f]{64}\\z").match(password).hasMatch()
        || !QDir::isAbsolutePath(storage) || storage.size() > 4096)
        return 2;
#if !defined(DOCKPIPE_BROWSER_SDK_HAS_ISOLATED_SESSIONS)
    return 3;
#else
    // Replace inherited Chromium options: operators cannot accidentally enable
    // QUIC, loopback bypass, or a second proxy inside this network context.
    QByteArray chromiumFlags = QByteArray("--disable-quic --disable-features=DnsOverHttps,AsyncDns --dns-prefetch-disable --force-webrtc-ip-handling-policy=disable_non_proxied_udp --proxy-bypass-list=<-loopback> --proxy-server=")
        + endpoint.toEncoded() + " --host-resolver-rules=\"MAP * ~NOTFOUND, EXCLUDE 127.0.0.1\"";
    const auto inheritedFlags = QProcess::splitCommand(qEnvironmentVariable("QTWEBENGINE_CHROMIUM_FLAGS"));
    if (inheritedFlags.contains(QStringLiteral("--disable-gpu")))
        chromiumFlags += " --disable-gpu";
    qputenv("QTWEBENGINE_CHROMIUM_FLAGS", chromiumFlags);
    qunsetenv("QTWEBENGINE_REMOTE_DEBUGGING");
    // Qt's application-proxy adapter inserts a simple-hostname bypass. Its
    // system-config path respects Chromium's explicit fixed proxy and bypass
    // preferences above, including localhost and remote DNS.
    QNetworkProxyFactory::setUseSystemConfiguration(true);
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication app(argc, argv);
    if (QGuiApplication::platformName() != "xcb")
        return 3;
    QCoreApplication::setApplicationName(QStringLiteral("dockpipe-launcher"));
    QCoreApplication::setOrganizationName(QStringLiteral("dockpipe"));
    applyDockpipeLauncherTheme(app);
    connectDockpipeLauncherThemeUpdates(app);
    BrowserSessionOptions options;
    if (hasTrust) {
        if (!config.value("tls_trust").isObject() || !config.value("target").isObject())
            return 2;
        const auto trust = parseBrowserNetworkTrust(config.value("tls_trust").toObject(),
            config.value("target").toObject());
        if (!trust)
            return 2;
        options.additionalTrustedCertificates = *trust;
    }
    options.contextId = QFileInfo(storage).fileName();
    options.persistentStoragePath = storage;
    options.allowExtensionInstallation = false;
    options.configurePage = [password](QWebEnginePage *page) {
        QObject::connect(page, &QWebEnginePage::proxyAuthenticationRequired, page,
            [password](const QUrl &, QAuthenticator *authenticator, const QString &proxyHost) {
                if (proxyHost == QStringLiteral("127.0.0.1")) {
                    authenticator->setUser(QStringLiteral("dockpipe"));
                    authenticator->setPassword(password);
                }
            });
    };
    std::unique_ptr<QWindow> embeddingParent;
    QWindow surface;
    surface.setFlags(Qt::FramelessWindowHint | Qt::BypassWindowManagerHint);
    surface.resize(1100, 760);
    BrowserWidget browser(options);
    EmbeddedBrowserSurface embeddedSurface(surface, browser);
    if (!browser.activeView())
        return 4;
    // The parent adopts this native window before it becomes visible. Never
    // register a normal top-level client that the window manager could decorate
    // or retain as an empty frame after cross-process reparenting.
    browser.setWindowFlags(Qt::FramelessWindowHint | Qt::BypassWindowManagerHint);
    browser.resize(1100, 760);
    browser.winId();
    embeddedSurface.synchronize();
    QTextStream(stdout) << QJsonDocument(QJsonObject{{"type", "ready"},
        {"window", QString::number(quint64(surface.winId()))},
        {"pid", QString::number(QCoreApplication::applicationPid())}}).toJson(QJsonDocument::Compact) << Qt::endl;
    QByteArray input;
    bool embedded = false;
    QSocketNotifier notifier(STDIN_FILENO, QSocketNotifier::Read);
    QObject::connect(&notifier, &QSocketNotifier::activated, &app, [&] {
        char buffer[4096];
        const auto count = ::read(STDIN_FILENO, buffer, sizeof(buffer));
        if (count <= 0) {
            notifier.setEnabled(false);
            browser.saveSession();
            browser.hide();
            surface.hide();
            app.exit(0);
            return;
        }
        input.append(buffer, count);
        // Commands carry one URL. 64 KiB accommodates long valid URLs without
        // permitting an unlimited producer on the private parent pipe.
        if (input.size() > 65536) {
            app.exit(2);
            return;
        }
        qsizetype newline = -1;
        while ((newline = input.indexOf('\n')) >= 0) {
            const auto command = QJsonDocument::fromJson(input.left(newline)).object();
            input.remove(0, newline + 1);
            if (command.value("type") == "embedded") {
                bool validParent = false;
                const auto parentId = command.value("parent").toString().toULongLong(&validParent);
                bool validPid = false;
                const auto parentPid = command.value("pid").toString().toLongLong(&validPid);
                if (command.size() != 3 || embedded || !validParent || !validPid
                    || !isOwnedNativeParent(quint64(surface.winId()), parentId, parentPid)) {
                    app.exit(2);
                    return;
                }
                embeddingParent.reset(QWindow::fromWinId(WId(parentId)));
                if (!embeddingParent) {
                    app.exit(2);
                    return;
                }
                // Both Qt connections adopt the same native parent. The stable
                // root survives lazy WebEngine QWidget surface replacement.
                QGuiApplication::sync();
                surface.setParent(embeddingParent.get());
                const auto parentSize = nativeWindowSize(parentId);
                if (parentSize.isEmpty()) {
                    app.exit(2);
                    return;
                }
                surface.setGeometry(QRect(QPoint(), parentSize));
                embeddedSurface.synchronize();
                embedded = true;
                surface.show();
                browser.show();
                embeddedSurface.synchronize();
                continue;
            }
            if (command.value("type") == "presentation") {
                if (command.size() != 5 || !command.value("chrome").isBool()
                    || !command.value("canvas").isBool() || !command.value("viewport").isBool()
                    || !command.value("suspended").isBool()) {
                    app.exit(2);
                    return;
                }
                browser.setChromeVisible(command.value("chrome").toBool());
                browser.setCanvasPresentationEnabled(command.value("canvas").toBool());
                browser.setCanvasViewportMode(command.value("viewport").toBool());
                browser.setBackgroundSuspended(command.value("suspended").toBool());
                continue;
            }
            if (command.size() != 2 || command.value("type") != "navigate") {
                app.exit(2);
                return;
            }
            const QUrl url(command.value("url").toString());
            if (url.scheme() != "http" && url.scheme() != "https" && url != QUrl("about:blank")) {
                app.exit(2);
                return;
            }
            browser.navigate(url);
        }
    });
    QObject::connect(&app, &QCoreApplication::aboutToQuit, &browser, [&] { browser.saveSession(); });
    return app.exec();
#endif
}
