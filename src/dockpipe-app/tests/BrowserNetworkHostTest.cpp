#include "BrowserNetworkHost.h"
#include "BrowserNetworkContext.h"
#include "BrowserNetworkTrust.h"
#include "BrowserWidget.h"
#include "RemoteBrowserWidget.h"
#include "WorkspaceMachineMenu.h"

#include <QApplication>
#include <QDeadlineTimer>
#include <QElapsedTimer>
#include <QFileInfo>
#include <QLineEdit>
#include <QSet>
#include <QStandardPaths>
#include <QSaveFile>
#include <QStackedWidget>
#include <QRegularExpression>
#include <QJsonDocument>
#include <QMenu>
#include <QProcess>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTimer>
#include <QWindow>
#include <QVBoxLayout>
#include <QSettings>
#include <QScopeGuard>
#include <QTemporaryDir>
#include <QTemporaryFile>
#include <QtTest>
#if defined(Q_OS_LINUX)
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#endif

namespace {
#if defined(Q_OS_LINUX)
struct NativeWindowState {
    bool valid = false;
    bool bypassesWindowManager = false;
    bool mapped = false;
    bool parentIsRoot = false;
    bool managedClient = false;
    quint64 parentId = 0;
    QRect rootGeometry;
};

NativeWindowState nativeWindowState(quint64 id)
{
    NativeWindowState state;
    Display *display = XOpenDisplay(nullptr);
    if (!display) {
        qWarning("Cannot open X display for native state query");
        return state;
    }
    XWindowAttributes attributes{};
    Window root = 0;
    Window parent = 0;
    Window *children = nullptr;
    unsigned int count = 0;
    const auto attributesResult = XGetWindowAttributes(display, Window(id), &attributes);
    const auto treeResult = XQueryTree(display, Window(id), &root, &parent, &children, &count);
    if (!attributesResult || !treeResult)
        qWarning("Native state query failed for %llu: attrs=%d tree=%d", id, attributesResult, treeResult);
    if (attributesResult && treeResult) {
        state.valid = true;
        state.bypassesWindowManager = attributes.override_redirect;
        state.mapped = attributes.map_state == IsViewable;
        state.parentIsRoot = parent == root;
        state.parentId = quint64(parent);
        int rootX = 0;
        int rootY = 0;
        Window ignored = 0;
        XTranslateCoordinates(display, Window(id), root, 0, 0, &rootX, &rootY, &ignored);
        state.rootGeometry = QRect(rootX, rootY, attributes.width, attributes.height);
    }
    if (children)
        XFree(children);
    const Atom clients = XInternAtom(display, "_NET_CLIENT_LIST", True);
    Atom type = 0;
    int format = 0;
    unsigned long length = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    if (clients && XGetWindowProperty(display, root, clients, 0, 4096, False,
            XA_WINDOW, &type, &format, &length, &remaining, &data) == Success
        && type == XA_WINDOW && format == 32 && data) {
        const auto *windows = reinterpret_cast<unsigned long *>(data);
        for (unsigned long index = 0; index < length; ++index)
            state.managedClient |= windows[index] == id;
    }
    if (data)
        XFree(data);
    XCloseDisplay(display);
    return state;
}
QList<quint64> nativeChildren(quint64 id)
{
    QList<quint64> result;
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return result;
    Window root = 0;
    Window parent = 0;
    Window *children = nullptr;
    unsigned int count = 0;
    if (XQueryTree(display, Window(id), &root, &parent, &children, &count)) {
        for (unsigned int index = 0; index < count; ++index)
            result.append(quint64(children[index]));
    }
    if (children)
        XFree(children);
    XCloseDisplay(display);
    return result;
}

quint64 largestMappedNativeChild(quint64 id)
{
    quint64 result = 0;
    qint64 largestArea = 0;
    for (const auto child : nativeChildren(id)) {
        const auto state = nativeWindowState(child);
        const auto area = qint64(state.rootGeometry.width()) * state.rootGeometry.height();
        if (state.valid && state.mapped && area > largestArea) {
            result = child;
            largestArea = area;
        }
    }
    return result;
}

qint64 nativeProcessId(quint64 id)
{
    qint64 result = 0;
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return result;
    const Atom property = XInternAtom(display, "_NET_WM_PID", True);
    Atom type = 0;
    int format = 0;
    unsigned long length = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    if (property && XGetWindowProperty(display, Window(id), property, 0, 1, False,
            XA_CARDINAL, &type, &format, &length, &remaining, &data) == Success
        && type == XA_CARDINAL && format == 32 && length == 1 && data)
        result = qint64(*reinterpret_cast<unsigned long *>(data));
    if (data)
        XFree(data);
    XCloseDisplay(display);
    return result;
}

quint64 nativeInputFocus()
{
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return 0;
    Window focused = 0;
    int revert = 0;
    XGetInputFocus(display, &focused, &revert);
    XCloseDisplay(display);
    return quint64(focused);
}

QColor nativeContentPixel(quint64 id)
{
    QColor result;
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return result;
    XWindowAttributes attributes{};
    if (XGetWindowAttributes(display, Window(id), &attributes) && attributes.map_state == IsViewable) {
        XImage *image = XGetImage(display, Window(id), attributes.width - 30, attributes.height - 30,
            1, 1, AllPlanes, ZPixmap);
        if (image) {
            XColor color{};
            color.pixel = XGetPixel(image, 0, 0);
            if (XQueryColor(display, attributes.colormap, &color))
                result = QColor(color.red / 256, color.green / 256, color.blue / 256);
            XDestroyImage(image);
        }
    }
    XCloseDisplay(display);
    return result;
}

#endif
}

class BrowserNetworkHostTest final : public QObject {
    Q_OBJECT
private slots:
    void rejectsMalformedPrivateRequests();
    void verifiesIdentityBoundTrustSnapshots();
    void retainsOfflineMachineWithoutCreatingLocalBrowser();
    void nodeOnlyMenuExposesNetworkMachines();
    void helperUsesAuthenticatedLoopbackProxyAndStopsOnParentEof();
    void contextReleasesOwnedChildren();
    void productionContextResizesRenderedChild();
    void productionContextAcceptsNativeKeyboardFocus();
    void liveContextClosesOnAuthorityLoss();

private:
    void checkProductionContext(bool focus);
};

void BrowserNetworkHostTest::verifiesIdentityBoundTrustSnapshots()
{
    QFile fixture(QFINDTESTDATA("fixtures/network-trust-ca.json"));
    QVERIFY(fixture.open(QIODevice::ReadOnly));
    const auto snapshot = QJsonDocument::fromJson(fixture.readAll()).object();
    const auto target = snapshot.value("target").toObject();
    const auto trusted = parseBrowserNetworkTrust(snapshot, target);
    QVERIFY(trusted);
    QCOMPARE(trusted->size(), 1);
    QCOMPARE(trusted->first().subjectInfo(QSslCertificate::CommonName).first(),
        QStringLiteral("Dockpipe Network Trust Test CA"));

    auto changed = snapshot;
    changed["digest"] = QString(64, '0');
    QVERIFY(!parseBrowserNetworkTrust(changed, target));
    auto stale = target;
    stale["generation"] = QString(64, 'c');
    QVERIFY(!parseBrowserNetworkTrust(snapshot, stale));
    changed = snapshot;
    changed["extra"] = true;
    QVERIFY(!parseBrowserNetworkTrust(changed, target));
    changed = snapshot;
    const auto certificate = snapshot.value("certificates").toArray().first();
    changed["certificates"] = QJsonArray{certificate, certificate};
    QVERIFY(!parseBrowserNetworkTrust(changed, target));
    changed["certificates"] = QJsonArray{QString(21849, 'A')};
    QVERIFY(!parseBrowserNetworkTrust(changed, target));
    changed["certificates"] = QJsonArray{QStringLiteral("not base64")};
    QVERIFY(!parseBrowserNetworkTrust(changed, target));

    changed = snapshot;
    changed["certificates"] = QJsonArray{};
    changed["digest"] = QStringLiteral("f20fe3cd74c9677b18bea61835aaf3f325ada3fd4025b66daeb3c78b512c85a2");
    const auto empty = parseBrowserNetworkTrust(changed, target);
    QVERIFY(empty && empty->isEmpty());
}

void BrowserNetworkHostTest::rejectsMalformedPrivateRequests()
{
    QTemporaryFile request;
    QVERIFY(request.open());
    request.write("{\"schema\":\"dockpipe.browser.host/v1\"}");
    request.flush();
    QByteArray executable("browser-test");
    QByteArray mode("--browser-network-host");
    QByteArray option("--request");
    QByteArray path = QFile::encodeName(request.fileName());
    char *arguments[] = {executable.data(), mode.data(), option.data(), path.data()};
    QCOMPARE(dispatchBrowserNetworkHost(4, arguments).value_or(-1), 2);
    QCOMPARE(dispatchBrowserNetworkHost(2, arguments).value_or(-1), 2);
    QTemporaryDir storage;
    const auto valid = QJsonDocument(QJsonObject{{"schema", "dockpipe.browser.host/v1"},
        {"endpoint", "http://127.0.0.1:12345"}, {"username", "dockpipe"},
        {"password", QString(64, 'a')}, {"storage", storage.path() + "/profile"}}).toJson(QJsonDocument::Compact);
    QVERIFY(request.resize(0));
    QCOMPARE(request.write(valid), valid.size());
    QVERIFY(request.flush());
    QVERIFY(request.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ReadGroup));
    QCOMPARE(dispatchBrowserNetworkHost(4, arguments).value_or(-1), 2);
    QVERIFY(request.setPermissions(QFile::ReadOwner | QFile::WriteOwner));
    const auto symlink = storage.path() + "/linked-request";
    QVERIFY(QFile::link(request.fileName(), symlink));
    path = QFile::encodeName(symlink);
    arguments[3] = path.data();
    QCOMPARE(dispatchBrowserNetworkHost(4, arguments).value_or(-1), 2);
    char *normal[] = {executable.data()};
    QVERIFY(!dispatchBrowserNetworkHost(1, normal).has_value());
}

void BrowserNetworkHostTest::retainsOfflineMachineWithoutCreatingLocalBrowser()
{
    const QString contextId = QStringLiteral("test-offline-browser");
    const QJsonObject target{{"node", "offline"}, {"fingerprint", QString(64, 'a')}};
    QSettings settings;
    settings.setValue("browserMachines/" + contextId,
        QJsonDocument(QJsonObject{{"target", target}, {"root", ""}}).toJson(QJsonDocument::Compact));
    {
        RemoteBrowserWidget widget(contextId);
        QCOMPARE(widget.machineTarget(), target);
        QVERIFY(!widget.localBrowser());
        QVERIFY(!widget.activeView());
        widget.setMachineCatalog({}, false);
        widget.navigate(QUrl("https://example.invalid"));
        QCOMPARE(widget.machineTarget(), target);
        QVERIFY(!widget.localBrowser());
        for (auto *process : widget.findChildren<QProcess *>())
            QCOMPARE(process->state(), QProcess::NotRunning);
    }
    settings.remove("browserMachines/" + contextId);
}

void BrowserNetworkHostTest::nodeOnlyMenuExposesNetworkMachines()
{
    WorkspaceMachineMenu menu;
    menu.setNodeOnlyMode(true);
    menu.setAccessUnlocked(true);
    const QJsonObject target{{"node", "remote-browser"}, {"fingerprint", QString(64, 'b')}};
    menu.setCatalog({QJsonObject{{"target", target}, {"enabled", true}}});
    QSignalSpy requests(&menu, &WorkspaceMachineMenu::contextRequested);
    QVERIFY(QMetaObject::invokeMethod(menu.menu(), "aboutToShow", Qt::DirectConnection));
    QAction *machine = nullptr;
    for (auto *action : menu.menu()->actions()) {
        if (action->text() == "remote-browser")
            machine = action;
    }
    QVERIFY(machine);
    machine->trigger();
    QCOMPARE(requests.size(), 1);
    QCOMPARE(requests.first().first().toJsonObject(), target);
}

void BrowserNetworkHostTest::helperUsesAuthenticatedLoopbackProxyAndStopsOnParentEof()
{
    if (QGuiApplication::platformName() != "xcb")
        QSKIP("Native helper embedding requires X11");
    QTcpServer proxy;
    QVERIFY(proxy.listen(QHostAddress::LocalHost));
    const QByteArray password(64, 'a');
    const QByteArray expected = "Proxy-Authorization: Basic " + (QByteArray("dockpipe:") + password).toBase64();
    bool authenticated = false;
    bool routedLocalhost = false;
    bool routedRemoteDns = false;
    connect(&proxy, &QTcpServer::newConnection, this, [&] {
        while (auto *socket = proxy.nextPendingConnection()) {
            auto pending = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [&, socket, pending] {
                *pending += socket->readAll();
                if (!pending->contains("\r\n\r\n"))
                    return;
                qInfo("Proxy received request (authenticated=%d, localhost=%d)", pending->contains(expected), pending->startsWith("GET http://localhost:43210/dockpipe-proxy-test "));
                routedRemoteDns |= pending->startsWith("GET http://remote-dns.invalid/dockpipe-proxy-test ");
                authenticated |= pending->contains(expected);
                routedLocalhost |= pending->startsWith("GET http://localhost:43210/dockpipe-proxy-test ");
                if (!pending->contains(expected)) {
                    socket->write("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic realm=\"dockpipe\"\r\nContent-Length: 0\r\nConnection: close\r\n\r\n");
                } else {
                    const QByteArray body("<html><title>Remote test</title><style>html,body{margin:0;min-height:100vh;background:#aa0011}</style><body>Proxy reached<script>setTimeout(()=>document.body.style.background='#1ac940',100)</script></body></html>");
                    socket->write("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: "
                        + QByteArray::number(body.size()) + "\r\nConnection: close\r\n\r\n" + body);
                }
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        }
    });
    QTemporaryDir storage;
    QTemporaryFile request;
    QVERIFY(request.open());
    const auto config = QJsonDocument(QJsonObject{{"schema", "dockpipe.browser.host/v1"},
        {"endpoint", QString("http://127.0.0.1:%1").arg(proxy.serverPort())},
        {"username", "dockpipe"}, {"password", QString::fromLatin1(password)},
        {"storage", storage.path() + "/profile"}}).toJson(QJsonDocument::Compact);
    QCOMPARE(request.write(config), config.size());
    QVERIFY(request.flush());
    QProcess child;
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert("QTWEBENGINE_CHROMIUM_FLAGS", "--disable-gpu");
    child.setProcessEnvironment(environment);
    child.start(QCoreApplication::applicationFilePath(), {"--browser-network-host", "--request", request.fileName()});
    QVERIFY(child.waitForStarted(5000));
    qInfo("Browser helper process started");
    QByteArray output;
    QTRY_VERIFY_WITH_TIMEOUT((output += child.readAllStandardOutput()).contains('\n')
        || child.state() == QProcess::NotRunning, 15000);
    const auto ready = QJsonDocument::fromJson(output.trimmed()).object();
    QVERIFY2(ready.value("type") == "ready", child.readAllStandardError().constData());
    QCOMPARE(ready.value("pid").toString(), QString::number(child.processId()));
    const auto nativeId = ready.value("window").toString().toULongLong();
#if defined(Q_OS_LINUX)
    const auto beforeEmbedding = nativeWindowState(nativeId);
    QVERIFY(beforeEmbedding.valid);
    QVERIFY(beforeEmbedding.bypassesWindowManager);
    QVERIFY(!beforeEmbedding.mapped);
    QVERIFY(!beforeEmbedding.managedClient);
#endif
    QWidget embedded;
    embedded.resize(800, 600);
    QVBoxLayout layout(&embedded);
    auto *foreign = QWindow::fromWinId(WId(nativeId));
    QVERIFY(foreign);
    auto *pages = new QStackedWidget(&embedded);
    layout.addWidget(pages);
    auto *browserPage = new QWidget(pages);
    auto *browserLayout = new QVBoxLayout(browserPage);
    auto *container = QWidget::createWindowContainer(foreign, browserPage);
    container->setAttribute(Qt::WA_NativeWindow);
    browserLayout->addWidget(container);
    pages->addWidget(browserPage);
    pages->addWidget(new QWidget(pages));
    embedded.show();
    QVERIFY(QTest::qWaitForWindowExposed(&embedded));
    const auto parentId = container->winId();
    foreign->setParent(container->windowHandle());
    foreign->setGeometry(container->rect());
    QGuiApplication::sync();
    child.write(QJsonDocument(QJsonObject{{"type", "embedded"},
        {"parent", QString::number(quint64(parentId))},
        {"pid", QString::number(QCoreApplication::applicationPid())}}).toJson(QJsonDocument::Compact) + '\n');
#if defined(Q_OS_LINUX)
    QTRY_VERIFY_WITH_TIMEOUT(nativeWindowState(nativeId).mapped, 5000);
    QVERIFY(!nativeWindowState(nativeId).parentIsRoot);
    QVERIFY(!nativeWindowState(nativeId).managedClient);
#endif
    child.write("{\"type\":\"navigate\",\"url\":\"http://localhost:43210/dockpipe-proxy-test\"}\n");
    QTRY_VERIFY_WITH_TIMEOUT(authenticated && routedLocalhost, 15000);
    child.write("{\"type\":\"navigate\",\"url\":\"http://remote-dns.invalid/dockpipe-proxy-test\"}\n");
    QTRY_VERIFY_WITH_TIMEOUT(routedRemoteDns, 15000);
    qInfo("Authenticated loopback and remote DNS requests reached the proxy");
#if defined(Q_OS_LINUX)
    QTest::qWait(500);
    const auto settledState = nativeWindowState(nativeId);
    if (!settledState.valid)
        qInfo().noquote() << child.readAllStandardError();
    QVERIFY(settledState.valid);
    QCOMPARE(settledState.parentId, quint64(parentId));
    QCOMPARE(settledState.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
    embedded.move(embedded.pos() + QPoint(135, 75));
    embedded.resize(930, 680);
    QTest::qWait(500);
    const auto movedState = nativeWindowState(nativeId);
    QVERIFY(movedState.valid);
    QCOMPARE(movedState.parentId, quint64(parentId));
    QCOMPARE(movedState.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
    QVERIFY(!nativeWindowState(nativeId).parentIsRoot);
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(nativeId), QColor(26, 201, 64), 5000);
    pages->setCurrentIndex(1);
    QTest::qWait(500);
    QVERIFY(!nativeWindowState(nativeId).mapped);
    pages->setCurrentIndex(0);
    QTest::qWait(500);
    QVERIFY(nativeWindowState(nativeId).mapped);
    const auto restoredState = nativeWindowState(nativeId);
    QVERIFY(restoredState.valid);
    QCOMPARE(restoredState.parentId, quint64(parentId));
    QCOMPARE(restoredState.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(nativeId), QColor(26, 201, 64), 5000);
    embedded.hide();
    QTest::qWait(500);
    QVERIFY(!nativeWindowState(nativeId).mapped);
    embedded.show();
    QTest::qWait(500);
    QVERIFY(nativeWindowState(nativeId).mapped);
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(nativeId), QColor(26, 201, 64), 5000);
#endif
    child.closeWriteChannel();
    QVERIFY(child.waitForFinished(5000));
    QCOMPARE(child.exitStatus(), QProcess::NormalExit);
    QCOMPARE(child.exitCode(), 0);
}

void BrowserNetworkHostTest::contextReleasesOwnedChildren()
{
    if (QGuiApplication::platformName() != "xcb")
        QSKIP("Native helper embedding requires X11");
    QTemporaryDir directory;
    const QJsonObject target{{"node", "fixture"}, {"fingerprint", QString(64, 'b')},
        {"generation", QString(64, 'c')}};
    const auto ready = QJsonDocument(QJsonObject{{"type", "ready"},
        {"endpoint", "http://127.0.0.1:9"}, {"username", "dockpipe"},
        {"password", QString(64, 'a')}, {"target", target}}).toJson(QJsonDocument::Compact);
    QFile relay(directory.path() + "/relay.py");
    QVERIFY(relay.open(QIODevice::WriteOnly));
    const auto script = QByteArray("#!/usr/bin/python3\nimport base64,sys\nsys.stdout.buffer.write(base64.b64decode(b'")
        + ready.toBase64() + "') + b'\\n')\nsys.stdout.buffer.flush()\nsys.stdin.buffer.read()\n";
    QCOMPARE(relay.write(script), script.size());
    QVERIFY(relay.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    relay.close();
    const auto previousBinary = qgetenv("DOCKPIPE_BIN");
    const auto previousFlags = qgetenv("QTWEBENGINE_CHROMIUM_FLAGS");
    const auto restore = qScopeGuard([&] {
        if (previousBinary.isNull())
            qunsetenv("DOCKPIPE_BIN");
        else
            qputenv("DOCKPIPE_BIN", previousBinary);
        if (previousFlags.isNull())
            qunsetenv("QTWEBENGINE_CHROMIUM_FLAGS");
        else
            qputenv("QTWEBENGINE_CHROMIUM_FLAGS", previousFlags);
    });
    qputenv("DOCKPIPE_BIN", QFile::encodeName(relay.fileName()));
    qputenv("QTWEBENGINE_CHROMIUM_FLAGS", "--disable-gpu");
    BrowserNetworkContext context(target, directory.path() + "/profile");
    context.resize(800, 600);
    context.show();
    context.navigate(QUrl("about:blank"));
    context.start();
    QTRY_VERIFY_WITH_TIMEOUT(context.findChild<QWidget *>("remoteBrowserContainer"), 15000);
    QCOMPARE(context.findChildren<QProcess *>().size(), 2);
    context.stop("Closed by test");
    QVERIFY(!context.findChild<QWidget *>("remoteBrowserContainer"));
    QVERIFY(context.findChildren<QProcess *>().isEmpty());
}

void BrowserNetworkHostTest::productionContextResizesRenderedChild()
{
    checkProductionContext(false);
}

void BrowserNetworkHostTest::productionContextAcceptsNativeKeyboardFocus()
{
    checkProductionContext(true);
}

void BrowserNetworkHostTest::checkProductionContext(bool focus)
{
    if (QGuiApplication::platformName() != "xcb")
        QSKIP("Native browser resize requires X11");
#if defined(Q_OS_LINUX)
    QTcpServer proxy;
    QVERIFY(proxy.listen(QHostAddress::LocalHost));
    QSet<QByteArray> nativeRequests;
    connect(&proxy, &QTcpServer::newConnection, this, [&] {
        while (auto *socket = proxy.nextPendingConnection()) {
            auto request = std::make_shared<QByteArray>();
            connect(socket, &QTcpSocket::readyRead, socket, [socket, request, &nativeRequests] {
                *request += socket->readAll();
                if (!request->contains("\r\n\r\n"))
                    return;
                nativeRequests.insert(request->left(request->indexOf("\r\n")));
                const QByteArray body("<html><body style='margin:0;min-height:100vh;background:#1ac940'>Resize proof"
                    "<script>setInterval(()=>history.replaceState({},'',location.pathname+'#tick'+Date.now()),80)</script></body></html>");
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nConnection: close\r\nContent-Length: "
                    + QByteArray::number(body.size()) + "\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
            connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        }
    });
    QTemporaryDir directory;
    const QJsonObject target{{"node", "fixture"}, {"fingerprint", QString(64, 'b')},
        {"generation", QString(64, 'c')}};
    const auto ready = QJsonDocument(QJsonObject{{"type", "ready"},
        {"endpoint", QString("http://127.0.0.1:%1").arg(proxy.serverPort())}, {"username", "dockpipe"},
        {"password", QString(64, 'a')}, {"target", target}}).toJson(QJsonDocument::Compact);
    QFile relay(directory.path() + "/relay.py");
    QVERIFY(relay.open(QIODevice::WriteOnly));
    const auto script = QByteArray("#!/usr/bin/python3\nimport base64,sys\nsys.stdout.buffer.write(base64.b64decode(b'")
        + ready.toBase64() + "') + b'\\n')\nsys.stdout.buffer.flush()\nsys.stdin.buffer.read()\n";
    QCOMPARE(relay.write(script), script.size());
    QVERIFY(relay.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    relay.close();
    const auto previousBinary = qgetenv("DOCKPIPE_BIN");
    const auto previousFlags = qgetenv("QTWEBENGINE_CHROMIUM_FLAGS");
    const auto restore = qScopeGuard([&] {
        if (previousBinary.isNull())
            qunsetenv("DOCKPIPE_BIN");
        else
            qputenv("DOCKPIPE_BIN", previousBinary);
        if (previousFlags.isNull())
            qunsetenv("QTWEBENGINE_CHROMIUM_FLAGS");
        else
            qputenv("QTWEBENGINE_CHROMIUM_FLAGS", previousFlags);
    });
    qputenv("DOCKPIPE_BIN", QFile::encodeName(relay.fileName()));
    qputenv("QTWEBENGINE_CHROMIUM_FLAGS", "--disable-gpu");
    QWidget host;
    QVBoxLayout layout(&host);
    auto *outside = new QLineEdit(&host);
    outside->setObjectName("parentFocusInput");
    layout.addWidget(outside);
    BrowserNetworkContext context(target, directory.path() + "/profile", &host);
    layout.addWidget(&context);
    host.resize(1000, 700);
    context.navigate(QUrl("http://resize-fixture.invalid/"));
    context.start();
    QTRY_VERIFY_WITH_TIMEOUT(context.findChild<QWidget *>("remoteBrowserContainer"), 15000);
    auto *container = context.findChild<QWidget *>("remoteBrowserContainer");
    QVERIFY(!container->isVisible());
    host.showNormal();
    const auto containerId = quint64(container->winId());
    qint64 helperPid = 0;
    for (auto *process : context.findChildren<QProcess *>()) {
        if (process->program() == QCoreApplication::applicationFilePath())
            helperPid = process->processId();
    }
    QVERIFY(helperPid);
    quint64 surfaceId = 0;
    for (const auto id : nativeChildren(containerId)) {
        if (nativeProcessId(id) == helperPid)
            surfaceId = id;
    }
    QVERIFY2(surfaceId, "Production context must embed its owned helper root");
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(surfaceId), QColor(26, 201, 64), 5000);
    if (focus) {
        // Qt synthetic key events delivered to a child object bypass the native
        // foreign-window focus route. XTest input through xdotool targets the actual
        // X server focus, without manually focusing the helper's native windows.
        const auto inputTool = QStandardPaths::findExecutable("xdotool");
        QVERIFY2(!inputTool.isEmpty(), "Native focus regression requires xdotool/XTest");
        const auto nativeInput = [&](const QStringList &arguments) {
            QProcess input;
            input.start(inputTool, arguments);
            return input.waitForFinished(5000) && input.exitStatus() == QProcess::NormalExit && input.exitCode() == 0;
        };
        const auto clickAt = [&](const QPoint &point) {
            return nativeInput({"mousemove", "--sync", QString::number(point.x()), QString::number(point.y()), "click", "1"});
        };
        const auto typeText = [&](const QString &text) {
            return nativeInput({"type", "--clearmodifiers", "--delay", "1", "--", text});
        };
        const auto key = [&](const QString &combination) {
            return nativeInput({"key", "--clearmodifiers", combination});
        };
        QCOMPARE(container->focusPolicy(), Qt::StrongFocus);
        const auto addressPoint = [&] { return container->mapToGlobal(QPoint(container->width() / 2, 64)); };
        QVERIFY(clickAt(addressPoint()));
        QVERIFY(key("ctrl+a"));
        QVERIFY(typeText("http://focus-fixture.invalid/typed-one"));
        QVERIFY(key("Return"));
        QTRY_VERIFY_WITH_TIMEOUT(nativeRequests.contains("GET http://focus-fixture.invalid/typed-one HTTP/1.1"), 8000);
        QVERIFY(key("ctrl+l"));
        QVERIFY(typeText("http://focus-fixture.invalid/typed-two"));
        QVERIFY(key("Return"));
        QTRY_VERIFY_WITH_TIMEOUT(nativeRequests.contains("GET http://focus-fixture.invalid/typed-two HTTP/1.1"), 8000);
        // Xvfb has no window manager to activate the ordinary parent on click.
        // Simulate only that parent activation, never the helper's focus route.
        host.activateWindow();
        QTest::qWait(100);
        QVERIFY(clickAt(outside->mapToGlobal(outside->rect().center())));
        QTRY_VERIFY(outside->hasFocus());
        QVERIFY(typeText("parent"));
        QTRY_COMPARE(outside->text(), QString("parent"));
        // Frequent renderer URL changes and a native geometry update must not
        // reactivate the embedded browser after the user focuses another input.
        host.resize(host.size() + QSize(20, 20));
        QTest::qWait(500);
        QVERIFY(typeText("-retained"));
        QTRY_COMPARE(outside->text(), QString("parent-retained"));
        QVERIFY(clickAt(addressPoint()));
        // xdotool waits block this test's GUI thread. Let the parent finish its
        // native focus notifications before sending the next physical keys.
        QTest::qWait(100);
        const auto focusedId = nativeInputFocus();
        const auto browserId = largestMappedNativeChild(surfaceId);
        const auto browserGeometry = nativeWindowState(browserId);
        QVERIFY(focusedId > 1);
        QCOMPARE(nativeProcessId(focusedId), helperPid);
        QVERIFY(browserGeometry.mapped);
        QCOMPARE(browserGeometry.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
        QVERIFY(browserGeometry.rootGeometry.contains(addressPoint()));
        QCOMPARE(outside->text(), QString("parent-retained"));
        QVERIFY(key("ctrl+a"));
        QVERIFY(typeText("http://focus-fixture.invalid/typed-three"));
        QVERIFY(key("Return"));
        QTRY_VERIFY_WITH_TIMEOUT(nativeRequests.contains("GET http://focus-fixture.invalid/typed-three HTTP/1.1"), 8000);
        context.stop("Native focus proof completed");
        QVERIFY(context.findChildren<QProcess *>().isEmpty());
        return;
    }
    const QList<QSize> sizes{{1000, 700}, {1350, 850}, {850, 650}, {1550, 950}};
    for (const auto &size : sizes) {
        host.showNormal();
        host.resize(size);
        QTRY_COMPARE_WITH_TIMEOUT(host.size(), size, 5000);
        QTest::qWait(150);
        const auto surface = nativeWindowState(surfaceId);
        QVERIFY(surface.valid);
        QCOMPARE(surface.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
        const auto browserId = largestMappedNativeChild(surfaceId);
        QVERIFY(browserId);
        QTRY_COMPARE_WITH_TIMEOUT(nativeWindowState(browserId).rootGeometry, surface.rootGeometry, 5000);
        QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(surfaceId), QColor(26, 201, 64), 5000);
    }
    host.showMaximized();
    QTest::qWait(500);
    QVERIFY(host.isMaximized());
    QVERIFY(host.width() > sizes.last().width() || host.height() > sizes.last().height());
    const auto maximized = nativeWindowState(surfaceId);
    QCOMPARE(maximized.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
    QCOMPARE(nativeWindowState(largestMappedNativeChild(surfaceId)).rootGeometry, maximized.rootGeometry);
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(surfaceId), QColor(26, 201, 64), 5000);
    host.showNormal();
    host.resize(1200, 780);
    QTRY_COMPARE_WITH_TIMEOUT(host.size(), QSize(1200, 780), 5000);
    QTest::qWait(150);
    const auto restored = nativeWindowState(surfaceId);
    QCOMPARE(restored.rootGeometry, QRect(container->mapToGlobal(QPoint()), container->size()));
    QCOMPARE(nativeWindowState(largestMappedNativeChild(surfaceId)).rootGeometry, restored.rootGeometry);
    QTRY_COMPARE_WITH_TIMEOUT(nativeContentPixel(surfaceId), QColor(26, 201, 64), 5000);
    context.stop("Resize proof completed");
    QVERIFY(context.findChildren<QProcess *>().isEmpty());
#endif
}

void BrowserNetworkHostTest::liveContextClosesOnAuthorityLoss()
{
    const auto requestPath = qEnvironmentVariable("DOCKPIPE_BROWSER_LIVE_REQUEST");
    if (requestPath.isEmpty())
        QSKIP("Live authority-loss validation requires an explicit private request");
    QVERIFY2(QGuiApplication::platformName() == "xcb", "Live browser context requires X11");
    const auto receiptPath = qEnvironmentVariable("DOCKPIPE_BROWSER_LIVE_RECEIPT");
    QVERIFY(QFileInfo(receiptPath).isAbsolute());
    QVERIFY(receiptPath != requestPath);
    const QFileInfo requestInfo(requestPath);
    QVERIFY(requestInfo.isAbsolute());
    QVERIFY(requestInfo.isFile());
    QVERIFY(!requestInfo.isSymLink());
    QVERIFY(!(requestInfo.permissions() & (QFile::ReadGroup | QFile::WriteGroup | QFile::ExeGroup
        | QFile::ReadOther | QFile::WriteOther | QFile::ExeOther)));
    QVERIFY(requestInfo.size() > 0 && requestInfo.size() <= 8192);
    QFile request(requestPath);
    QVERIFY(request.open(QIODevice::ReadOnly));
    const auto descriptor = QJsonDocument::fromJson(request.read(8193)).object();
    QCOMPARE(descriptor.size(), 2);
    const auto target = descriptor.value("target").toObject();
    QCOMPARE(target.size(), 3);
    const QRegularExpression digest("\\A[0-9a-f]{64}\\z");
    QVERIFY(QRegularExpression("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z")
        .match(target.value("node").toString()).hasMatch());
    QVERIFY(digest.match(target.value("fingerprint").toString()).hasMatch());
    QVERIFY(digest.match(target.value("generation").toString()).hasMatch());
    const QUrl url(descriptor.value("url").toString());
    QVERIFY(url.isValid());
    QVERIFY(url.scheme() == QStringLiteral("http") || url.scheme() == QStringLiteral("https"));
    QVERIFY(url.host() == "localhost" || url.host() == "127.0.0.1");
    QVERIFY(url.port() > 0 && url.port() <= 65535);
    QVERIFY(url.userInfo().isEmpty());
    QVERIFY(!qEnvironmentVariableIsEmpty("DOCKPIPE_BIN"));

    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    BrowserNetworkContext context(target, directory.path() + "/profile");
    context.resize(800, 600);
    context.show();
    context.navigate(url);
    context.start();
    QDeadlineTimer startupDeadline(35000);
    while (!context.findChild<QWidget *>("remoteBrowserContainer") && !startupDeadline.hasExpired())
        QTest::qWait(50);
    QVERIFY2(context.findChild<QWidget *>("remoteBrowserContainer"), "Production context did not embed its owned helper");
    QCOMPARE(context.findChildren<QProcess *>().size(), 2);
    QVERIFY(context.findChildren<BrowserWidget *>().isEmpty());

    QElapsedTimer revocationTime;
    const auto writeReceipt = [&](const QString &phase) {
        QJsonArray children;
        for (auto *process : context.findChildren<QProcess *>())
            children.append(QString::number(process->processId()));
        const QJsonObject receipt{{"phase", phase}, {"children", children},
            {"embedded", context.findChild<QWidget *>("remoteBrowserContainer") != nullptr},
            {"local_browser", !context.findChildren<BrowserWidget *>().isEmpty()},
            {"elapsed_ms", revocationTime.isValid() ? revocationTime.elapsed() : 0}};
        QSaveFile output(receiptPath);
        if (!output.open(QIODevice::WriteOnly)
            || !output.setPermissions(QFile::ReadOwner | QFile::WriteOwner))
            return false;
        const auto bytes = QJsonDocument(receipt).toJson(QJsonDocument::Compact);
        return output.write(bytes) == bytes.size() && output.commit();
    };
    revocationTime.start();
    QVERIFY(writeReceipt("ready"));
    QDeadlineTimer revocationDeadline(60000);
    const auto closed = [&] {
        return context.findChildren<QProcess *>().isEmpty()
            && !context.findChild<QWidget *>("remoteBrowserContainer");
    };
    while (!closed() && !revocationDeadline.hasExpired())
        QTest::qWait(50);
    const bool observedAuthorityLoss = closed();
    QVERIFY(writeReceipt(observedAuthorityLoss ? "closed" : "timeout"));
    QVERIFY2(observedAuthorityLoss, "Production browser context did not close within 60 seconds of readiness");
    QVERIFY(context.findChildren<BrowserWidget *>().isEmpty());
}

int main(int argc, char **argv)
{
    if (const auto helper = dispatchBrowserNetworkHost(argc, argv))
        return *helper;
    QApplication app(argc, argv);
    QTemporaryDir settings;
    QCoreApplication::setOrganizationName("dockpipe-browser-test");
    QCoreApplication::setApplicationName("network-host");
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    BrowserNetworkHostTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "BrowserNetworkHostTest.moc"
