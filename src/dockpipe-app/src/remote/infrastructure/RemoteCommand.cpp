#include "RemoteCommand.h"
#include <QJsonArray>
#include <QRegularExpression>

#include <QJsonDocument>
#include <QJsonObject>

RemoteCommand::RemoteCommand(QObject *parent) : QObject(parent)
{
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        emit output(tr("Operation timed out. Existing remote state is preserved.\n"));
        cancel();
    });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &RemoteCommand::drain);
    connect(&m_process, &QProcess::readyReadStandardError, this, [this]() {
        emit output(QString::fromUtf8(m_process.readAllStandardError()));
    });
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this](int code, QProcess::ExitStatus status) {
        drain();
        if (!m_pendingLine.isEmpty()) {
            consumeLine(m_pendingLine);
            m_pendingLine.clear();
        }
        finish(!m_cancelled && code == 0 && status == QProcess::NormalExit);
    });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        emit output(m_process.errorString() + QLatin1Char('\n'));
        if (error == QProcess::FailedToStart)
            finish(false);
    });
}

RemoteCommand::~RemoteCommand()
{
    m_completion = {};
    m_process.disconnect(this);
    if (m_process.state() != QProcess::NotRunning) {
        m_process.terminate();
        if (!m_process.waitForFinished(1000)) {
            m_process.kill();
            m_process.waitForFinished(1000);
        }
    }
}

bool RemoteCommand::busy() const
{
    return bool(m_completion);
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
    m_cancelled = false;
    m_completion = std::move(completion);
    emit busyChanged(true);
    m_process.start(program, arguments);
    m_timeout.start(timeoutMs);
}

void RemoteCommand::drain()
{
    const QByteArray chunk = m_process.readAllStandardOutput();
    // Keep the existing 20 MiB command-output budget while framing control lines.
    // Check before retaining a chunk so a missing newline cannot grow the buffer.
    if (m_stdout.size() + m_pendingLine.size() + chunk.size() > 20 * 1024 * 1024) {
        emit output(tr("Output exceeded the display limit.\n"));
        m_pendingLine.clear();
        cancel();
        return;
    }
    m_pendingLine += chunk;
    while (m_pendingLine.contains('\n')) {
        const qsizetype end = m_pendingLine.indexOf('\n');
        const QByteArray line = m_pendingLine.left(end + 1);
        m_pendingLine.remove(0, end + 1);
        consumeLine(line);
    }
}

void RemoteCommand::consumeLine(const QByteArray &line)
{
    const auto event = QJsonDocument::fromJson(line).object();
    const QString eventName = event.value("event").toString();
    // Reserved protocol frames never reach diagnostics, including invalid frames.
    if (eventName.startsWith("hardware-key-") || line.contains("hardware-key-")) {
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
        } else if (valid && eventName == "hardware-key-status") {
            const QString message = event.value("message").toString();
            if (!message.isEmpty() && message.size() <= 256) {
                emit hardwareKeyStatus(message);
                return;
            }
        }
        emit output(tr("Rejected an invalid security-key interaction request.\n"));
        cancel();
        return;
    }
    if (m_stdout.size() + line.size() > 20 * 1024 * 1024) {
        emit output(tr("Output exceeded the display limit.\n"));
        cancel();
        return;
    }
    m_stdout += line;
    emit standardOutput(line);
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
        emit output(tr("Security-key response exceeded the protocol limit.\n"));
        cancel();
        return;
    }
    m_promptId.clear();
    m_promptDevices.clear();
    // QProcess retains bytes only until writing them to the child's private pipe.
    const bool queued = m_process.write(response) == response.size();
    response.fill('\0');
    if (!queued)
        cancel();
}

void RemoteCommand::cancel()
{
    if (!busy())
        return;
    m_cancelled = true;
    m_promptId.clear();
    m_promptDevices.clear();
    m_process.terminate();
    QTimer::singleShot(2000, &m_process, [this]() {
        if (m_cancelled && m_process.state() != QProcess::NotRunning)
            m_process.kill();
    });
}

void RemoteCommand::finish(bool success)
{
    m_timeout.stop();
    auto completion = std::move(m_completion);
    m_completion = {};
    const QByteArray output = m_stdout;
    emit busyChanged(false);
    if (completion)
        completion(success, output);
}
