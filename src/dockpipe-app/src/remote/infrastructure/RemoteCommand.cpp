#include "RemoteCommand.h"
#include <QCoreApplication>
#include <QJsonArray>
#include <QRegularExpression>
#include <QPointer>

#include <QJsonDocument>
#include <QJsonObject>

namespace {
class ExternalOperation final : public QObject {
public:
    ExternalOperation(QObject *parent, std::function<void()> cancellation)
        : QObject(parent), cancel(std::move(cancellation))
    {
        setObjectName(QStringLiteral("remoteExternalOperation"));
    }
    std::function<void()> cancel;
    QMetaObject::Connection ownerDestroyed;
};

ExternalOperation *externalOperation(const RemoteCommand *command)
{
    return static_cast<ExternalOperation *>(command->findChild<QObject *>(
        QStringLiteral("remoteExternalOperation"), Qt::FindDirectChildrenOnly));
}
}

RemoteCommand::RemoteCommand(QObject *parent) : QObject(parent)
{
    auto *child = new QProcess(this);
    child->setObjectName(QStringLiteral("remoteCommandProcess"));
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, child, [child] {
            if (child->state() != QProcess::NotRunning)
                child->kill();
        });
    }
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        const QPointer<RemoteCommand> guard(this);
        emit output(tr("Operation timed out. Existing remote state is preserved.\n"));
        if (guard)
            cancel();
    });
    connect(&process(), &QProcess::readyReadStandardOutput, this, &RemoteCommand::drain);
    connect(&process(), &QProcess::started, this, [this]() {
        if (m_cancelled)
            process().terminate();
    });
    connect(&process(), &QProcess::readyReadStandardError, this, [this]() {
        emit output(QString::fromUtf8(process().readAllStandardError()));
    });
    connect(&process(), qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this](int code, QProcess::ExitStatus status) {
        const QPointer<RemoteCommand> guard(this);
        drain();
        if (!guard)
            return;
        if (!m_pendingLine.isEmpty()) {
            consumeLine(m_pendingLine);
            if (!guard)
                return;
            m_pendingLine.clear();
        }
        finish(!m_cancelled && code == 0 && status == QProcess::NormalExit);
    });
    connect(&process(), &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        const QPointer<RemoteCommand> guard(this);
        emit output(process().errorString() + QLatin1Char('\n'));
        if (guard && error == QProcess::FailedToStart)
            finish(false);
    });
}

QProcess &RemoteCommand::process() const
{
    // Child state preserves the installed SDK object's physical layout. The
    // historical embedded QProcess remains idle for binary compatibility.
    return *findChild<QProcess *>(QStringLiteral("remoteCommandProcess"), Qt::FindDirectChildrenOnly);
}

RemoteCommand::~RemoteCommand()
{
    m_completion = {};
    auto *child = &process();
    child->disconnect(this);
    child->setParent(nullptr);
    if (child->state() == QProcess::NotRunning) {
        child->deleteLater();
        return;
    }
    connect(child, &QProcess::finished, child, &QObject::deleteLater);
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, child, [child] {
            if (child->state() != QProcess::NotRunning)
                child->kill();
        });
    }
    child->terminate();
    QTimer::singleShot(2500, child, [child] {
        if (child->state() != QProcess::NotRunning)
            child->kill();
    });
}

bool RemoteCommand::busy() const
{
    return bool(m_completion);
}

bool RemoteCommand::trackExternalOperation(QObject *owner, std::function<void()> cancel)
{
    if (!owner || !cancel || busy())
        return false;
    auto *operation = new ExternalOperation(this, std::move(cancel));
    operation->ownerDestroyed = connect(owner, &QObject::destroyed, this, &RemoteCommand::completeExternalOperation);
    // Existing compiled clients inline through busy(), which must still reflect
    // this transport. The sentinel is never an external operation completion.
    m_completion = [](bool, const QByteArray &) {};
    emit busyChanged(true);
    return true;
}

void RemoteCommand::completeExternalOperation()
{
    auto *operation = externalOperation(this);
    if (!operation)
        return;
    disconnect(operation->ownerDestroyed);
    delete operation;
    m_completion = {};
    emit busyChanged(false);
}

void RemoteCommand::run(const QString &program, const QStringList &arguments,
                        Completion completion, int timeoutMs)
{
    if (busy())
        return;
    m_stdout.clear();
    m_pendingLine.clear();
    const int interactionOption = arguments.indexOf("--interaction");
    m_guiInteraction = interactionOption >= 0 && arguments.value(interactionOption + 1) == "gui";
    m_promptId.clear();
    m_promptDevices.clear();
    m_seenPromptIds.clear();
    process().setProperty("remoteCommandAttempt",
        process().property("remoteCommandAttempt").toULongLong() + 1);
    m_cancelled = false;
    m_completion = std::move(completion);
    const QPointer<RemoteCommand> guard(this);
    emit busyChanged(true);
    if (!guard)
        return;
    m_timeout.start(timeoutMs);
    process().start(program, arguments);
}

void RemoteCommand::drain()
{
    const QByteArray chunk = process().readAllStandardOutput();
    // Keep the existing 20 MiB command-output budget while framing control lines.
    // Check before retaining a chunk so a missing newline cannot grow the buffer.
    if (m_stdout.size() + m_pendingLine.size() + chunk.size() > 20 * 1024 * 1024) {
        const QPointer<RemoteCommand> guard(this);
        emit output(tr("Output exceeded the display limit.\n"));
        if (guard) {
            m_pendingLine.clear();
            cancel();
        }
        return;
    }
    m_pendingLine += chunk;
    while (m_pendingLine.contains('\n')) {
        const qsizetype end = m_pendingLine.indexOf('\n');
        const QByteArray line = m_pendingLine.left(end + 1);
        m_pendingLine.remove(0, end + 1);
        const QPointer<RemoteCommand> guard(this);
        consumeLine(line);
        if (!guard)
            return;
    }
}

void RemoteCommand::consumeLine(const QByteArray &line)
{
    const auto event = QJsonDocument::fromJson(line).object();
    const QString eventName = event.value("event").toString();
    // Reserved frames remain private. Ordinary JSON from non-interactive Git or
    // file calls may legitimately contain this text in a path or diff.
    if (eventName.startsWith("hardware-key-") || (m_guiInteraction && line.contains("hardware-key-"))) {
        bool valid = !m_cancelled && m_guiInteraction && line.size() <= 8192;
        if (valid && eventName == "hardware-key-prompt") {
            static const QRegularExpression identifier("\\A[0-9a-f]{32}\\z");
            const QString id = event.value("id").toString();
            const QString title = event.value("title").toString();
            const QString details = event.value("details").toString();
            const auto devices = event.value("devices").toArray();
            valid = m_promptId.isEmpty() && !m_seenPromptIds.contains(id)
                && identifier.match(id).hasMatch()
                && !title.isEmpty() && title.size() <= 160
                && !details.isEmpty() && details.size() <= 4096
                && event.value("pin_required").isBool()
                && !devices.isEmpty() && devices.size() <= 8;
            QStringList deviceIds;
            for (const auto &value : devices) {
                const auto device = value.toObject();
                const QString deviceId = device.value("id").toString();
                const QString name = device.value("name").toString();
                if (deviceId.isEmpty() || deviceId.size() > 32 || deviceIds.contains(deviceId)
                    || name.isEmpty() || name.size() > 160) {
                    valid = false;
                    break;
                }
                deviceIds.append(deviceId);
            }
            if (valid) {
                m_promptId = id;
                m_promptDevices = deviceIds;
                m_seenPromptIds.insert(id);
                emit hardwareKeyPrompt(event);
                return;
            }
        } else if (valid && eventName == "hardware-key-unavailable" && event.size() == 1
                   && m_promptId.isEmpty()) {
            emit hardwareKeyUnavailable();
            return;
        } else if (valid && eventName == "hardware-key-status") {
            const QString message = event.value("message").toString();
            if (!message.isEmpty() && message.size() <= 256) {
                emit hardwareKeyStatus(message);
                return;
            }
        }
        const QPointer<RemoteCommand> guard(this);
        emit output(tr("Rejected an invalid security-key interaction request.\n"));
        if (guard)
            cancel();
        return;
    }
    if (m_stdout.size() + line.size() > 20 * 1024 * 1024) {
        const QPointer<RemoteCommand> guard(this);
        emit output(tr("Output exceeded the display limit.\n"));
        if (guard)
            cancel();
        return;
    }
    m_stdout += line;
    const QPointer<RemoteCommand> guard(this);
    emit standardOutput(line);
    if (guard)
        emit output(QString::fromUtf8(line));
}

void RemoteCommand::submitHardwareKeyResponse(const QString &id, const QString &device,
                                             const QString &pin)
{
    if (!busy() || m_cancelled || !m_guiInteraction || id.isEmpty()
        || id != m_promptId || !m_promptDevices.contains(device))
        return;
    QByteArray response = QJsonDocument(QJsonObject{{"id", id}, {"device", device},
        {"pin", pin}, {"cancel", false}}).toJson(QJsonDocument::Compact) + '\n';
    if (response.size() > 8192) {
        response.fill('\0');
        const QPointer<RemoteCommand> guard(this);
        emit output(tr("Security-key response exceeded the protocol limit.\n"));
        if (guard)
            cancel();
        return;
    }
    m_promptId.clear();
    m_promptDevices.clear();
    // QProcess retains bytes only until writing them to the child's private pipe.
    const bool queued = process().write(response) == response.size();
    response.fill('\0');
    if (!queued)
        cancel();
}

void RemoteCommand::cancel()
{
    if (auto *operation = externalOperation(this)) {
        const auto cancellation = operation->cancel;
        cancellation();
        return;
    }
    if (!busy() || m_cancelled)
        return;
    m_cancelled = true;
    m_promptId.clear();
    m_promptDevices.clear();
    process().terminate();
    const quint64 attempt = process().property("remoteCommandAttempt").toULongLong();
    QTimer::singleShot(2000, this, [this, attempt]() {
        if (attempt == process().property("remoteCommandAttempt").toULongLong()
            && m_cancelled && process().state() != QProcess::NotRunning)
            process().kill();
    });
}

void RemoteCommand::finish(bool success)
{
    m_timeout.stop();
    auto completion = std::move(m_completion);
    m_completion = {};
    const QByteArray output = m_stdout;
    const QPointer<RemoteCommand> guard(this);
    emit busyChanged(false);
    if (guard && completion)
        completion(success, output);
}
