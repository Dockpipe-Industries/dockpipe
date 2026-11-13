#include "BrowserNetworkContext.h"
#include "BrowserNetworkTrust.h"
#include "DockpipeChoices.h"

#include <QCoreApplication>
#include <QDeadlineTimer>
#include <QGuiApplication>
#include <QJsonDocument>
#include <QLabel>
#include <QProcess>
#include <QPointer>
#include <QRegularExpression>
#include <QTemporaryFile>
#include <QTimer>
#include <QUrl>
#include <QVBoxLayout>
#include <QWindow>
#if defined(Q_OS_LINUX)
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#endif

namespace {
bool writePrivateRequest(QTemporaryFile &file, const QJsonObject &value)
{
    const auto bytes = QJsonDocument(value).toJson(QJsonDocument::Compact);
    return file.open() && file.setPermissions(QFile::ReadOwner | QFile::WriteOwner)
        && file.write(bytes) == bytes.size() && file.flush();
}

bool ownsWindow(quint64 id, qint64 pid)
{
#if defined(Q_OS_LINUX)
    Display *display = XOpenDisplay(nullptr);
    if (!display)
        return false;
    const Atom property = XInternAtom(display, "_NET_WM_PID", True);
    Atom actualType = 0;
    int format = 0;
    unsigned long count = 0;
    unsigned long remaining = 0;
    unsigned char *data = nullptr;
    bool valid = false;
    if (property && XGetWindowProperty(display, Window(id), property, 0, 1, False,
            XA_CARDINAL, &actualType, &format, &count, &remaining, &data) == Success) {
        valid = actualType == XA_CARDINAL && format == 32 && count == 1
            && data && *reinterpret_cast<unsigned long *>(data) == static_cast<unsigned long>(pid);
    }
    if (data)
        XFree(data);
    XCloseDisplay(display);
    return valid;
#else
    Q_UNUSED(id);
    Q_UNUSED(pid);
    return false;
#endif
}
}

BrowserNetworkContext::BrowserNetworkContext(const QJsonObject &target, const QString &storagePath,
                                           QWidget *parent)
    : QWidget(parent), m_target(target), m_storagePath(storagePath),
      m_status(new QLabel(tr("Remote browser is disconnected."), this)), m_deadline(new QTimer(this))
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->addWidget(m_status);
    m_status->setWordWrap(true);
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this] {
        stop(tr("Remote browser startup timed out."));
    });
}

BrowserNetworkContext::~BrowserNetworkContext()
{
    releaseProcesses();
}

void BrowserNetworkContext::setTarget(const QJsonObject &target)
{
    if (target == m_target)
        return;
    stop(tr("Remote browser permissions changed. Reconnecting…"));
    m_target = target;
}

void BrowserNetworkContext::start()
{
    if (m_relay)
        return;
#if !defined(Q_OS_LINUX)
    stop(tr("Remote browser embedding is currently supported on Linux X11 only."));
    return;
#endif
    if (QGuiApplication::platformName() != QStringLiteral("xcb")) {
        stop(tr("Remote browser embedding requires an X11 desktop."));
        return;
    }
    QString error;
    const auto binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty()) {
        stop(error);
        return;
    }
    m_request = std::make_unique<QTemporaryFile>();
    if (!writePrivateRequest(*m_request, {{"schema", "dockpipe.remote.network/v1"}, {"target", m_target}})) {
        stop(tr("Could not create the private remote browser request."));
        return;
    }
    m_stopping = false;
    m_relayBuffer.clear();
    m_hostBuffer.clear();
    m_relay = new QProcess(this);
    m_relay->setProcessChannelMode(QProcess::SeparateChannels);
    connect(m_relay, &QProcess::readyReadStandardError, this, [this] {
        m_relay->readAllStandardError(); // Never surface credential-bearing child diagnostics.
    });
    connect(m_relay, &QProcess::readyReadStandardOutput, this, &BrowserNetworkContext::relayOutput);
    connect(m_relay, &QProcess::errorOccurred, this, [this] { stop(tr("Remote browser relay failed.")); });
    connect(m_relay, &QProcess::finished, this, [this] { stop(tr("Remote browser connection closed.")); });
    m_status->setText(tr("Connecting remote browser…"));
    m_deadline->start(30000);
    m_relay->start(binary, {"remote", "network-proxy", "--stdio", "--request", m_request->fileName()});
}

void BrowserNetworkContext::relayOutput()
{
    const auto bytes = m_relay->readAllStandardOutput();
    if (m_host || bytes.size() + m_relayBuffer.size() > 4 * 1024 * 1024) {
        stop(tr("Invalid remote browser relay response."));
        return;
    }
    m_relayBuffer += bytes;
    const auto newline = m_relayBuffer.indexOf('\n');
    if (newline < 0)
        return;
    const auto ready = QJsonDocument::fromJson(m_relayBuffer.left(newline)).object();
    m_relayBuffer.clear();
    const QUrl endpoint(ready.value("endpoint").toString());
    if (ready.value("type") != "ready" || ready.value("target").toObject() != m_target
        || endpoint.scheme() != "http" || endpoint.host() != "127.0.0.1"
        || endpoint.port() < 1 || endpoint.port() > 65535 || !endpoint.userInfo().isEmpty()
        || !endpoint.path().isEmpty() || endpoint.hasQuery() || endpoint.hasFragment()
        || ready.value("username").toString() != "dockpipe"
        || !QRegularExpression("\\A[0-9a-f]{64}\\z").match(ready.value("password").toString()).hasMatch()) {
        stop(tr("Invalid remote browser connection descriptor."));
        return;
    }
    if (ready.contains("tls_trust")
        && (!ready.value("tls_trust").isObject()
            || !parseBrowserNetworkTrust(ready.value("tls_trust").toObject(), m_target))) {
        stop(tr("The remote TLS trust snapshot could not be verified."));
        return;
    }
    launchHost(ready);
}

void BrowserNetworkContext::launchHost(const QJsonObject &ready)
{
    QJsonObject config{{"schema", "dockpipe.browser.host/v1"},
        {"endpoint", ready.value("endpoint")}, {"username", ready.value("username")},
        {"password", ready.value("password")}, {"storage", m_storagePath}};
    if (ready.contains("tls_trust")) {
        config.insert("tls_trust", ready.value("tls_trust"));
        config.insert("target", m_target);
    }
    m_hostRequest = std::make_unique<QTemporaryFile>();
    if (!writePrivateRequest(*m_hostRequest, config)) {
        stop(tr("Could not create the private browser host request."));
        return;
    }
    m_host = new QProcess(this);
    connect(m_host, &QProcess::readyReadStandardError, this, [this] { m_host->readAllStandardError(); });
    connect(m_host, &QProcess::readyReadStandardOutput, this, &BrowserNetworkContext::hostOutput);
    connect(m_host, &QProcess::errorOccurred, this, [this] { stop(tr("Remote browser host failed.")); });
    connect(m_host, &QProcess::finished, this, [this] { stop(tr("Remote browser host closed.")); });
    m_host->start(QCoreApplication::applicationFilePath(),
        {"--browser-network-host", "--request", m_hostRequest->fileName()});
}

void BrowserNetworkContext::hostOutput()
{
    m_hostBuffer += m_host->readAllStandardOutput();
    if (m_hostBuffer.size() > 1024 || m_ready) {
        stop(tr("Invalid browser host response."));
        return;
    }
    const auto newline = m_hostBuffer.indexOf('\n');
    if (newline < 0)
        return;
    const auto ready = QJsonDocument::fromJson(m_hostBuffer.left(newline)).object();
    bool validId = false;
    const auto id = ready.value("window").toString().toULongLong(&validId);
    if (ready.value("type") != "ready" || !validId || !id
        || ready.value("pid").toString() != QString::number(m_host->processId())
        || !ownsWindow(id, m_host->processId())) {
        stop(tr("Browser host window ownership could not be verified."));
        return;
    }
    auto *window = QWindow::fromWinId(WId(id));
    if (!window) {
        stop(tr("Browser host window could not be embedded."));
        return;
    }
    m_container = QWidget::createWindowContainer(window, this);
    m_container->setObjectName(QStringLiteral("remoteBrowserContainer"));
    m_container->setFocusPolicy(Qt::StrongFocus);
    // Select the native-child coordinate mode before QWindowContainer's first
    // Show event. Creating its WId after showing leaves Qt using top-level offsets.
    m_container->setAttribute(Qt::WA_NativeWindow);
    const QPointer<QWidget> container(m_container);
    const QPointer<QWindow> embeddedWindow(window);
    const QPointer<QProcess> owner(m_host);
    const auto admitEmbedding = [this, container, embeddedWindow, owner] {
        if (!container || !embeddedWindow || !owner || container != m_container
            || owner != m_host || m_ready || !embeddedWindow->isVisible())
            return;
        // QWindowContainer must perform its own first-show reparenting. Calling
        // setParent on its foreign window earlier removes that window from the
        // container, silently disabling all later geometry/visibility updates.
        QGuiApplication::sync();
        if (!container || !embeddedWindow || !owner || container != m_container
            || owner != m_host || m_ready)
            return;
        if (embeddedWindow->parent() != container->windowHandle()) {
            stop(tr("Browser host window did not attach to its native container."));
            return;
        }
        const auto parentId = container->winId();
        m_ready = true;
        m_deadline->stop();
        writeCommand({{"type", "embedded"}, {"parent", QString::number(quint64(parentId))},
            {"pid", QString::number(QCoreApplication::applicationPid())}});
        writeCommand(m_presentation);
        if (!m_pendingUrl.isEmpty())
            navigate(m_pendingUrl);
    };
    connect(window, &QWindow::visibleChanged, m_container, [container, admitEmbedding](bool visible) {
        if (visible)
            QTimer::singleShot(0, container, admitEmbedding);
    });
    layout()->addWidget(m_container);
    m_status->hide();
    m_hostRequest.reset();
    m_container->show();
    if (window->isVisible())
        QTimer::singleShot(0, m_container, admitEmbedding);
}

void BrowserNetworkContext::navigate(const QUrl &url)
{
    if (m_ready) {
        writeCommand({{"type", "navigate"}, {"url", url.toString()}});
        m_pendingUrl = QUrl();
    } else {
        m_pendingUrl = url;
    }
}

void BrowserNetworkContext::setPresentation(bool chrome, bool canvas, bool viewport, bool suspended)
{
    m_presentation = {{"type", "presentation"}, {"chrome", chrome}, {"canvas", canvas},
        {"viewport", viewport}, {"suspended", suspended}};
    if (m_ready)
        writeCommand(m_presentation);
}

void BrowserNetworkContext::writeCommand(const QJsonObject &command)
{
    if (m_host && m_host->state() == QProcess::Running)
        m_host->write(QJsonDocument(command).toJson(QJsonDocument::Compact) + '\n');
}

void BrowserNetworkContext::stop(const QString &reason)
{
    if (m_stopping)
        return;
    m_stopping = true;
    m_deadline->stop();
    m_ready = false;
    releaseProcesses();
    delete m_container;
    m_container = nullptr;
    m_status->setText(reason);
    m_status->show();
}

void BrowserNetworkContext::releaseProcesses()
{
    // Closing the helper's stdin lets it persist tabs before the proxy disappears.
    const QList<QProcess *> processes{m_host, m_relay};
    m_host = nullptr;
    m_relay = nullptr;
    QDeadlineTimer gracefulDeadline(1000);
    for (auto *process : processes) {
        if (!process)
            continue;
        process->disconnect(this);
        process->closeWriteChannel();
        if (process->state() != QProcess::NotRunning)
            process->waitForFinished(int(gracefulDeadline.remainingTime()));
    }
    for (auto *process : processes) {
        if (process && process->state() != QProcess::NotRunning)
            process->kill();
    }
    QDeadlineTimer forcedDeadline(1000);
    for (auto *process : processes) {
        if (!process)
            continue;
        if (process->state() != QProcess::NotRunning)
            process->waitForFinished(int(forcedDeadline.remainingTime()));
        delete process;
    }
    m_request.reset();
    m_hostRequest.reset();
}
