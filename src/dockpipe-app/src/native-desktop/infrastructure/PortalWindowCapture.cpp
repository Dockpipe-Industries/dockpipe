#include "PortalWindowCapture.h"
#include "DesktopCaptureFrameBuffer.h"

#include <QDBusArgument>
#include <QCoreApplication>
#include <QDBusConnection>
#include <QDBusMessage>
#include <QDBusObjectPath>
#include <QDBusPendingCallWatcher>
#include <QDBusPendingReply>
#include <QDBusUnixFileDescriptor>
#include <QProcess>
#include <QStandardPaths>
#include <QTimer>
#include <QUuid>
#include <fcntl.h>
#include <cerrno>
#include <signal.h>
#include <unistd.h>

namespace {
const QString Service = QStringLiteral("org.freedesktop.portal.Desktop");
const QString DesktopPath = QStringLiteral("/org/freedesktop/portal/desktop");
const QString ScreenCast = QStringLiteral("org.freedesktop.portal.ScreenCast");
const QString Request = QStringLiteral("org.freedesktop.portal.Request");
const QString Session = QStringLiteral("org.freedesktop.portal.Session");

QString token()
{
    return QStringLiteral("dockpipe_") + QUuid::createUuid().toString(QUuid::Id128);
}
}

struct PortalWindowCapture::State {
    std::unique_ptr<QDBusConnection> bus;
    QString connectionName;
    QString requestPath;
    QString sessionPath;
    QString method;
    QString executable;
    QString target;
    QDBusUnixFileDescriptor remote;
    int childDescriptor = -1;
    QProcess *helper = nullptr;
    QTimer *deadline = nullptr;
    DesktopCaptureFrameBuffer frames;
    QByteArray diagnostics;
    quint64 generation = 0;
    bool active = false;
    bool receivedFrame = false;
    bool pauseRequested = false;
    bool helperPaused = false;
    int frameDeadlineRemaining = -1;
};

PortalWindowCapture::PortalWindowCapture(QObject *parent)
    : QObject(parent), m_state(std::make_unique<State>())
{
    m_state->deadline = new QTimer(this);
    m_state->deadline->setObjectName("portalCaptureDeadline");
    m_state->deadline->setSingleShot(true);
    connect(m_state->deadline, &QTimer::timeout, this, [this] {
        fail(tr("Window sharing timed out. Choose a window to try again."));
    });
}

PortalWindowCapture::~PortalWindowCapture()
{
    stop();
}

void PortalWindowCapture::start()
{
    stop();
    m_state->executable = QStandardPaths::findExecutable("gst-launch-1.0");
    if (m_state->executable.isEmpty()) {
        fail(tr("Wayland window sharing needs GStreamer (gst-launch-1.0), its PipeWire and JPEG plugins, and your desktop sharing portal."));
        return;
    }
    m_state->connectionName = token();
    m_state->bus = std::make_unique<QDBusConnection>(
        QDBusConnection::connectToBus(QDBusConnection::SessionBus, m_state->connectionName));
    if (!m_state->bus->isConnected()) {
        fail(tr("Cannot connect to the desktop sharing portal."));
        return;
    }
    m_state->active = true;
    m_state->deadline->start(120000);
    emit statusChanged(tr("Choose a window in your desktop's sharing dialog. Sharing is view-only."));
    request("CreateSession", {}, {{"session_handle_token", token()}});
}

void PortalWindowCapture::request(const QString &method, const QVariantList &arguments, QVariantMap options)
{
    if (!m_state->active || !m_state->bus)
        return;
    QString sender = m_state->bus->baseService().mid(1);
    sender.replace('.', '_');
    const QString requestToken = token();
    options.insert("handle_token", requestToken);
    m_state->requestPath = "/org/freedesktop/portal/desktop/request/" + sender + '/' + requestToken;
    m_state->method = method;
    if (!m_state->bus->connect(Service, m_state->requestPath, Request, "Response", this,
            SLOT(response(uint,QVariantMap,QDBusMessage)))) {
        fail(tr("Cannot listen for the desktop sharing decision."));
        return;
    }
    QDBusMessage message = QDBusMessage::createMethodCall(Service, DesktopPath, ScreenCast, method);
    QVariantList values = arguments;
    values.append(options);
    message.setArguments(values);
    const auto generation = m_state->generation;
    const QString expectedPath = m_state->requestPath;
    auto *watcher = new QDBusPendingCallWatcher(m_state->bus->asyncCall(message, 10000), this);
    connect(watcher, &QDBusPendingCallWatcher::finished, this,
        [this, generation, expectedPath](QDBusPendingCallWatcher *completed) {
            QDBusPendingReply<QDBusObjectPath> reply = *completed;
            completed->deleteLater();
            if (!m_state->active || generation != m_state->generation)
                return;
            if (reply.isError()) {
                fail(tr("Desktop sharing request failed: %1").arg(reply.error().message()));
                return;
            }
            if (reply.value().path() != expectedPath)
                fail(tr("The desktop portal returned an unsupported request identity."));
        });
}

void PortalWindowCapture::response(uint result, const QVariantMap &values, const QDBusMessage &message)
{
    if (!m_state->active || !m_state->bus || message.path() != m_state->requestPath)
        return;
    m_state->bus->disconnect(Service, m_state->requestPath, Request, "Response", this,
        SLOT(response(uint,QVariantMap,QDBusMessage)));
    m_state->requestPath.clear();
    if (result != 0) {
        if (result == 1) {
            stop();
            emit cancelled();
        } else {
            fail(tr("The desktop declined window sharing."));
        }
        return;
    }
    if (m_state->method == "CreateSession") {
        m_state->sessionPath = values.value("session_handle").toString();
        if (!m_state->sessionPath.startsWith("/org/freedesktop/portal/desktop/session/")) {
            fail(tr("The desktop returned an invalid sharing session."));
            return;
        }
        m_state->bus->connect(Service, m_state->sessionPath, Session, "Closed", this, SLOT(sessionClosed(QVariantMap)));
        request("SelectSources", {QVariant::fromValue(QDBusObjectPath(m_state->sessionPath))},
            {{"types", uint(2)}, {"multiple", false}, {"persist_mode", uint(0)}});
    } else if (m_state->method == "SelectSources") {
        request("Start", {QVariant::fromValue(QDBusObjectPath(m_state->sessionPath)), QString()}, {});
    } else if (m_state->method == "Start") {
        openRemote(values);
    }
}

void PortalWindowCapture::openRemote(const QVariantMap &values)
{
    if (!values.value("streams").canConvert<QDBusArgument>()) {
        fail(tr("The desktop returned invalid window stream metadata."));
        return;
    }
    const QDBusArgument streams = values.value("streams").value<QDBusArgument>();
    if (streams.currentSignature() != "a(ua{sv})") {
        fail(tr("The desktop returned an invalid window stream signature."));
        return;
    }
    streams.beginArray();
    uint node = 0;
    QVariantMap properties;
    if (!streams.atEnd()) {
        streams.beginStructure();
        streams >> node >> properties;
        streams.endStructure();
    }
    const bool extra = !streams.atEnd();
    streams.endArray();
    if (!node || extra || (properties.contains("source_type") && properties.value("source_type").toUInt() != 2)) {
        fail(tr("The desktop did not return one approved window stream."));
        return;
    }
    // Modern portals provide the immutable PipeWire object serial. Older
    // portals expose only the scoped remote's node id, retained for this session.
    const qulonglong serial = properties.value("pipewire-serial").toULongLong();
    m_state->target = serial ? QString("target-object=%1").arg(serial) : QString("path=%1").arg(node);
    QDBusMessage message = QDBusMessage::createMethodCall(Service, DesktopPath, ScreenCast, "OpenPipeWireRemote");
    message.setArguments({QVariant::fromValue(QDBusObjectPath(m_state->sessionPath)), QVariantMap()});
    const auto generation = m_state->generation;
    auto *watcher = new QDBusPendingCallWatcher(m_state->bus->asyncCall(message, 10000), this);
    connect(watcher, &QDBusPendingCallWatcher::finished, this,
        [this, generation](QDBusPendingCallWatcher *completed) {
            QDBusPendingReply<QDBusUnixFileDescriptor> reply = *completed;
            completed->deleteLater();
            if (!m_state->active || generation != m_state->generation)
                return;
            if (reply.isError() || !reply.value().isValid()) {
                fail(tr("The desktop could not open the approved PipeWire stream."));
                return;
            }
            m_state->remote = reply.value();
            startHelper();
        });
}

void PortalWindowCapture::startHelper()
{
    auto *helper = new QProcess(this);
    helper->setObjectName("portalWindowCaptureHelper");
    m_state->helper = helper;
    m_state->childDescriptor = fcntl(m_state->remote.fileDescriptor(), F_DUPFD_CLOEXEC, 64);
    if (m_state->childDescriptor < 0) {
        fail(tr("Cannot prepare the approved PipeWire connection for capture."));
        return;
    }
    const int childDescriptor = m_state->childDescriptor;
    helper->setChildProcessModifier([childDescriptor] {
        if (fcntl(childDescriptor, F_SETFD, 0) < 0)
            _exit(126);
    });
    connect(helper, &QProcess::started, this, [this] {
        close(m_state->childDescriptor);
        m_state->childDescriptor = -1;
        m_state->remote = QDBusUnixFileDescriptor();
        m_state->deadline->start(15000);
        setPaused(m_state->pauseRequested);
    });
    connect(helper, &QProcess::readyReadStandardOutput, this, &PortalWindowCapture::readFrames);
    connect(helper, &QProcess::readyReadStandardError, this, [this, helper] {
        constexpr qsizetype DiagnosticBytes = 4096;
        const qsizetype remaining = DiagnosticBytes - m_state->diagnostics.size();
        helper->setReadChannel(QProcess::StandardError);
        const bool exceeded = helper->bytesAvailable() > remaining;
        if (remaining > 0)
            m_state->diagnostics.append(helper->read(remaining));
        helper->setReadChannel(QProcess::StandardOutput);
        if (exceeded)
            fail(tr("The capture helper exceeded its diagnostic limit: %1 [truncated]")
                .arg(QString::fromUtf8(m_state->diagnostics)));
    });
    connect(helper, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (m_state->active && error == QProcess::FailedToStart)
            fail(tr("Could not start the local GStreamer capture helper."));
    });
    connect(helper, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int, QProcess::ExitStatus) {
            if (m_state->active)
                fail(tr("Window sharing ended. Check GStreamer's PipeWire and JPEG plugins. %1")
                    .arg(QString::fromUtf8(m_state->diagnostics).trimmed()));
        });
    // One pending pipeline buffer, bounded output dimensions, and no recording
    // or audio. Drop source frames rather than accumulating work behind the UI.
    helper->start(m_state->executable, {"-q", "pipewiresrc", QString("fd=%1").arg(childDescriptor), m_state->target,
        "!", "queue", "max-size-buffers=1", "leaky=downstream", "!", "videorate", "drop-only=true",
        "!", "video/x-raw,framerate=15/1", "!", "videoconvert", "!", "videoscale", "add-borders=true",
        "!", "video/x-raw,width=1280,height=720,pixel-aspect-ratio=1/1", "!", "jpegenc", "quality=75",
        "!", "fdsink", "fd=1", "sync=false"});
}

void PortalWindowCapture::readFrames()
{
    auto *helper = m_state->helper;
    if (!helper || !m_state->active)
        return;
    if (helper->bytesAvailable() > DesktopCaptureFrameBuffer::MaximumFrameBytes) {
        fail(tr("The capture helper exceeded the 4 MiB pending-output limit."));
        return;
    }
    if (m_state->helperPaused)
        return;
    QImage latest;
    QString error;
    // Bound each dispatch to the transport budget; read only 64 KiB at a time.
    qsizetype budget = DesktopCaptureFrameBuffer::MaximumFrameBytes;
    while (budget > 0 && helper->bytesAvailable() > 0) {
        const QByteArray bytes = helper->read(qMin<qsizetype>(budget, 64 * 1024));
        budget -= bytes.size();
        if (bytes.isEmpty())
            break;
        if (!m_state->frames.append(bytes, latest, error)) {
            fail(error);
            return;
        }
    }
    if (!latest.isNull()) {
        m_state->deadline->stop();
        if (!m_state->receivedFrame) {
            m_state->receivedFrame = true;
            emit statusChanged(tr("Live window share · View only. Use the original window for input."));
        }
        emit frameReady(latest);
    }
}

void PortalWindowCapture::setPaused(bool paused)
{
    m_state->pauseRequested = paused;
    auto *helper = m_state->helper;
    if (!helper || helper->state() != QProcess::Running || m_state->helperPaused == paused)
        return;
    const auto processId = helper->processId();
    if (processId <= 0)
        return;
    if (::kill(pid_t(processId), paused ? SIGSTOP : SIGCONT) != 0) {
        // A helper that exited concurrently is reconciled by QProcess::finished.
        if (errno != ESRCH)
            fail(tr("Could not pause or resume the window capture helper."));
        return;
    }
    m_state->helperPaused = paused;
    if (paused) {
        m_state->frameDeadlineRemaining = m_state->deadline->remainingTime();
        m_state->deadline->stop();
    } else {
        if (m_state->frameDeadlineRemaining >= 0)
            m_state->deadline->start(qMax(1, m_state->frameDeadlineRemaining));
        m_state->frameDeadlineRemaining = -1;
        // Keep partial JPEG framing intact across the stopped process. At most
        // the existing bounded pipe buffer remains queued while hidden.
        readFrames();
    }
}

void PortalWindowCapture::sessionClosed(const QVariantMap &details)
{
    Q_UNUSED(details);
    if (m_state->active)
        fail(tr("The desktop ended this window share."));
}

void PortalWindowCapture::stop()
{
    ++m_state->generation;
    m_state->active = false;
    m_state->deadline->stop();
    const auto pending = findChildren<QDBusPendingCallWatcher *>(QString(), Qt::FindDirectChildrenOnly);
    for (auto *watcher : pending) {
        watcher->disconnect(this);
        watcher->deleteLater();
    }
    if (m_state->bus) {
        if (!m_state->requestPath.isEmpty())
            m_state->bus->call(QDBusMessage::createMethodCall(Service, m_state->requestPath, Request, "Close"), QDBus::NoBlock);
        if (!m_state->sessionPath.isEmpty())
            m_state->bus->call(QDBusMessage::createMethodCall(Service, m_state->sessionPath, Session, "Close"), QDBus::NoBlock);
        // A private connection also closes sessions whose CreateSession reply
        // races cancellation. It cannot disconnect another DockPipe client.
        QDBusConnection::disconnectFromBus(m_state->connectionName);
        m_state->bus.reset();
    }
    if (m_state->helper) {
        auto *helper = m_state->helper;
        helper->disconnect(this);
        if (m_state->helperPaused && helper->state() == QProcess::Running && helper->processId() > 0)
            ::kill(pid_t(helper->processId()), SIGCONT);
        helper->closeReadChannel(QProcess::StandardOutput);
        helper->closeReadChannel(QProcess::StandardError);
        if (helper->state() != QProcess::NotRunning) {
            // The application owns the short drain after this widget closes.
            // Never wait on a child from the GUI thread. One grace period and
            // one kill escalation replace blocking per-panel shutdown waits.
            helper->setParent(QCoreApplication::instance());
            connect(helper, qOverload<int, QProcess::ExitStatus>(&QProcess::finished),
                helper, &QObject::deleteLater);
            connect(QCoreApplication::instance(), &QCoreApplication::aboutToQuit,
                helper, [helper] { helper->kill(); });
            helper->terminate();
            QTimer::singleShot(500, helper, [helper] {
                if (helper->state() != QProcess::NotRunning)
                    helper->kill();
            });
        } else {
            helper->deleteLater();
        }
        m_state->helper = nullptr;
    }
    m_state->remote = QDBusUnixFileDescriptor();
    if (m_state->childDescriptor >= 0) {
        close(m_state->childDescriptor);
        m_state->childDescriptor = -1;
    }
    m_state->requestPath.clear();
    m_state->sessionPath.clear();
    m_state->frames.clear();
    m_state->diagnostics.clear();
    m_state->receivedFrame = false;
    m_state->pauseRequested = false;
    m_state->helperPaused = false;
    m_state->frameDeadlineRemaining = -1;
}

void PortalWindowCapture::fail(const QString &message)
{
    stop();
    emit failed(message);
}
