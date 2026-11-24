#include "RemoteTerminalSession.h"

#include "DockpipeChoices.h"
#include <QCoreApplication>
#include <QDateTime>
#include <QJsonArray>
#include <QJsonDocument>
#include <QPointer>
#include <QProcess>
#include <QRegularExpression>
#include <QTemporaryFile>
#include <QTimer>
#include <QUuid>
#include <cmath>

namespace {
constexpr qsizetype InputLimit = 64 * 1024;
constexpr qsizetype OutputChunkLimit = 32 * 1024;
constexpr qsizetype LineLimit = 96 * 1024;
constexpr quint64 MaxExactInteger = 9007199254740991ULL;

bool exactInteger(const QJsonValue &value, quint64 &result)
{
    const double number = value.toDouble(-1);
    if (number < 0 || number > double(MaxExactInteger) || std::floor(number) != number)
        return false;
    result = quint64(number);
    return true;
}
}

RemoteTerminalSession::RemoteTerminalSession(QObject *parent)
    : QObject(parent), m_openDeadline(new QTimer(this)), m_closeDeadline(new QTimer(this))
{
    m_openDeadline->setSingleShot(true);
    m_closeDeadline->setSingleShot(true);
    connect(m_openDeadline, &QTimer::timeout, this, [this] {
        fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
    });
    connect(m_closeDeadline, &QTimer::timeout, this, [this] {
        if (m_process && m_process->state() != QProcess::NotRunning)
            m_process->kill();
    });
}

RemoteTerminalSession::~RemoteTerminalSession()
{
    releaseProcess();
}

bool RemoteTerminalSession::start(const QJsonObject &catalog, const QString &root)
{
    return startSelected(catalog, root, {}, {});
}

bool RemoteTerminalSession::startContainer(const QJsonObject &catalog, const QString &container, const QString &shell)
{
    return startSelected(catalog, {}, container, shell);
}

bool RemoteTerminalSession::startSelected(const QJsonObject &catalog, const QString &root,
                                         const QString &container, const QString &shell)
{
    if (m_active || m_process)
        return false;
    static const QRegularExpression identifier(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    const auto target = catalog.value("target").toObject();
    bool granted = false;
    for (const auto &value : catalog.value("roots").toArray()) {
        const auto permission = value.toObject();
        if (permission.value("id").toString() == root && permission.value("terminal").toBool())
            granted = true;
    }
    if (!container.isEmpty()) {
        granted = catalog.value("docker_exec").toBool() && catalog.value("docker_read").toBool()
            && root.isEmpty() && digest.match(container).hasMatch()
            && (shell == "/bin/sh" || shell == "/bin/bash");
    } else if (!identifier.match(root).hasMatch() || !shell.isEmpty()) {
        granted = false;
    }
    if (!granted
        || !identifier.match(target.value("node").toString()).hasMatch()
        || !digest.match(target.value("fingerprint").toString()).hasMatch()
        || !digest.match(target.value("generation").toString()).hasMatch()
        || !digest.match(catalog.value("terminal_epoch").toString()).hasMatch()) {
        m_error = tr("Remote terminal access is unavailable for this machine or root.");
        return false;
    }
    QString binaryError;
    const QString binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
    if (binary.isEmpty()) {
        m_error = binaryError;
        return false;
    }
    m_intent = {{"schema", "dockpipe.remote.terminal/v1"}, {"target", target},
        {"epoch", catalog.value("terminal_epoch")}, {"root", root},
        {"nonce", QUuid::createUuid().toString(QUuid::Id128) + QUuid::createUuid().toString(QUuid::Id128)},
        {"deadline", QDateTime::currentDateTimeUtc().addSecs(25).toString(Qt::ISODateWithMs)},
        {"columns", m_columns}, {"rows", m_rows}};
    if (!container.isEmpty()) {
        m_intent["container"] = container;
        m_intent["shell"] = shell;
    }
    m_request = std::make_unique<QTemporaryFile>();
    const auto intent = QJsonDocument(m_intent).toJson(QJsonDocument::Compact);
    if (!m_request->open() || m_request->write(intent) != intent.size() || !m_request->flush()) {
        m_error = tr("Could not start the trusted remote terminal client.");
        m_request.reset();
        return false;
    }
    m_request->close();
    m_error.clear();
    m_active = true;
    m_closing = false;
    auto *process = new QProcess(this);
    m_process = process;
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, process, [process] {
            if (process->state() != QProcess::NotRunning)
                process->kill();
        });
    }
    connect(process, &QProcess::started, this, [this, process] {
        if (m_closing)
            process->terminate();
    });
    connect(process, &QProcess::readyReadStandardOutput, this, &RemoteTerminalSession::drain);
    connect(process, &QProcess::readyReadStandardError, this, [process] {
        // Diagnostics cannot contain terminal output or input. Discard rather
        // than rendering arbitrary CLI stderr inside a privileged terminal.
        process->readAllStandardError();
    });
    connect(process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_error = tr("Could not start the trusted remote terminal client.");
            finish(1);
        }
    });
    connect(process, &QProcess::finished, this, [this](int code, QProcess::ExitStatus status) {
        const QPointer<RemoteTerminalSession> guard(this);
        drain();
        if (!guard)
            return;
        if (!m_closing && (!m_started || status != QProcess::NormalExit || code != 0) && m_error.isEmpty())
            m_error = tr("The remote terminal connection was lost. Pending input was not replayed.");
        finish(code);
    });
    m_openDeadline->start(30000);
    process->start(binary, {"remote", "terminal", "--prepare", "--request", m_request->fileName(), "--auth", "never"});
    return true;
}

bool RemoteTerminalSession::matchesReference(const QJsonObject &reference) const
{
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    return reference.value("schema") == m_intent.value("schema")
        && reference.value("target") == m_intent.value("target")
        && reference.value("epoch") == m_intent.value("epoch")
        && digest.match(reference.value("id").toString()).hasMatch()
        && (m_reference.isEmpty() || reference == m_reference);
}

void RemoteTerminalSession::drain()
{
    if (!m_process)
        return;
    const auto chunk = m_process->readAllStandardOutput();
    // A chunk may contain several valid bounded frames. Retain only the current
    // line so a fast output stream cannot accumulate a second output ring here.
    qsizetype offset = 0;
    while (offset < chunk.size()) {
        const auto newline = chunk.indexOf('\n', offset);
        const auto end = newline < 0 ? chunk.size() : newline;
        if (end - offset > LineLimit - m_lines.size()) {
            fail(tr("The remote terminal returned an invalid response."));
            return;
        }
        m_lines.append(chunk.constData() + offset, end - offset);
        offset = end + (newline < 0 ? 0 : 1);
        if (newline < 0)
            break;
        const auto line = std::move(m_lines);
        m_lines.clear();
        const QPointer<RemoteTerminalSession> guard(this);
        consume(line);
        if (!guard || !m_active || m_closing)
            return;
    }
}

void RemoteTerminalSession::consume(const QByteArray &line)
{
    QJsonParseError parse;
    const auto document = QJsonDocument::fromJson(line, &parse);
    const auto event = document.object();
    if (parse.error != QJsonParseError::NoError || !document.isObject()) {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    const QString type = event.value("type").toString();
    if (type == "prepared") {
        const auto reference = event.value("reference").toObject();
        if (m_prepared || !matchesReference(reference)) {
            fail(tr("The remote terminal returned an invalid response."));
            return;
        }
        m_reference = reference;
        m_prepared = true;
        if (!sendFrame({{"type", "prepared-ack"}, {"reference", reference}}))
            fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
        return;
    }
    if (type == "session") {
        const auto reference = event.value("reference").toObject();
        if (!m_prepared || !matchesReference(reference)) {
            fail(tr("The remote terminal returned an invalid response."));
            return;
        }
        m_reference = reference;
        return;
    }
    if (type == "ack") {
        quint64 sequence = 0;
        if (m_reference.isEmpty() || !exactInteger(event.value("sequence"), sequence)) {
            fail(tr("The remote terminal returned an invalid response."));
            return;
        }
        acknowledge(sequence);
        return;
    }
    if (type == "error") {
        fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
        return;
    }
    if (type != "output" && type != "closed") {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    const auto view = event.value("view").toObject();
    quint64 cursor = 0;
    quint64 oldest = 0;
    quint64 end = 0;
    quint64 ack = 0;
    const auto encoded = view.value("data").toString().toLatin1();
    const auto decoded = QByteArray::fromBase64Encoding(encoded, QByteArray::AbortOnBase64DecodingErrors);
    if (m_reference.isEmpty() || !matchesReference(view.value("reference").toObject())
        || !exactInteger(view.value("cursor"), cursor) || !exactInteger(view.value("oldest"), oldest)
        || !exactInteger(view.value("end"), end) || end < cursor
        || !exactInteger(view.value("ack_sequence"), ack) || !decoded
        || decoded.decoded.size() > OutputChunkLimit || oldest > cursor
        || cursor < quint64(decoded.decoded.size()) || cursor < m_cursor) {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    const quint64 start = cursor - quint64(decoded.decoded.size());
    const bool truncated = view.value("truncated").toBool();
    if (start != m_cursor && !(truncated && start >= m_cursor)) {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    const QString state = view.value("state").toString();
    const bool closingState = state == "closed" || state == "unknown";
    if ((type == "closed" && (!closingState || cursor != end || !decoded.decoded.isEmpty()))
        || (ack > m_acknowledged
            && (m_inFlight.isEmpty() || ack != m_sequence || ack != m_acknowledged + 1))) {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    const bool firstClosed = closingState && !m_nativeClosed;
    if (closingState) {
        m_nativeClosed = true;
        m_cleanupVerified = state == "closed";
    }
    m_cursor = cursor;
    const QPointer<RemoteTerminalSession> guard(this);
    if (!decoded.decoded.isEmpty())
        emit output(decoded.decoded, cursor, truncated);
    if (!guard)
        return;
    acknowledge(ack);
    if (!guard || !m_active || m_closing)
        return;
    if (closingState) {
        completeInput(false, 0);
        if (!guard)
            return;
        if (state == "unknown" || view.value("reason") == "authority-lost")
            m_error = tr("The remote terminal connection was lost. Pending input was not replayed.");
        if (type == "closed") {
            stop();
        } else if (firstClosed) {
            emit stateChanged();
        }
        // Closed-state output is still a data frame. The CLI drains its final
        // retained chunks before the metadata-only closed event.
        return;
    }
    if (state == "running" && type == "output" && !m_nativeClosed) {
        const bool firstRunning = !m_started;
        m_started = true;
        m_openDeadline->stop();
        if (firstRunning && !sendFrame({{"type", "resize"}, {"columns", m_columns}, {"rows", m_rows}})) {
            fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
            return;
        }
        flushInput();
        if (guard && firstRunning)
            emit stateChanged();
    } else if ((state != "queued" && state != "opening") || m_nativeClosed) {
        fail(tr("The remote terminal returned an invalid response."));
    }
}

void RemoteTerminalSession::acknowledge(quint64 sequence)
{
    if (sequence <= m_acknowledged)
        return;
    if (m_inFlight.isEmpty() || sequence != m_sequence || sequence != m_acknowledged + 1) {
        fail(tr("The remote terminal returned an invalid response."));
        return;
    }
    m_acknowledged = sequence;
    m_inFlight.fill('\0');
    m_inFlight.clear();
    const QPointer<RemoteTerminalSession> guard(this);
    completeInput(true, m_inFlightEnd);
    if (guard)
        flushInput();
}

void RemoteTerminalSession::completeInput(bool success, quint64 through)
{
    QVector<std::function<void(bool)>> completions;
    while (!m_inputCompletions.isEmpty() && (!success || m_inputCompletions.first().first <= through))
        completions.append(m_inputCompletions.takeFirst().second);
    const QPointer<RemoteTerminalSession> guard(this);
    for (const auto &completion : completions) {
        completion(success);
        if (!guard)
            return;
    }
}

bool RemoteTerminalSession::send(const QByteArray &data)
{
    return sendAcknowledged(data, {});
}

bool RemoteTerminalSession::sendAcknowledged(const QByteArray &data, std::function<void(bool)> completion)
{
    if ((completion && m_inputCompletions.size() >= 128) || !running() || data.isEmpty() || data.size() > InputLimit - m_pending.size() - m_inFlight.size())
        return false;
    m_pending += data;
    m_acceptedBytes += quint64(data.size());
    if (completion)
        m_inputCompletions.append({m_acceptedBytes, std::move(completion)});
    flushInput();
    return true;
}

bool RemoteTerminalSession::sendFrame(const QJsonObject &frame)
{
    if (!m_process || m_process->state() != QProcess::Running || m_process->bytesToWrite() > 2 * InputLimit)
        return false;
    const auto line = QJsonDocument(frame).toJson(QJsonDocument::Compact) + '\n';
    return m_process->write(line) == line.size();
}

void RemoteTerminalSession::flushInput()
{
    if (!m_started || !running() || !m_inFlight.isEmpty() || m_pending.isEmpty())
        return;
    if (m_sequence == MaxExactInteger) {
        fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
        return;
    }
    m_inFlight = std::move(m_pending);
    m_pending.clear();
    ++m_sequence;
    m_inFlightEnd = m_acceptedBytes;
    if (!sendFrame({{"type", "input"}, {"sequence", double(m_sequence)},
                    {"data", QString::fromLatin1(m_inFlight.toBase64())}}))
        fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
}

void RemoteTerminalSession::resize(int columns, int rows)
{
    m_columns = qBound(1, columns, 1000);
    m_rows = qBound(1, rows, 1000);
    if (m_started && running() && !sendFrame({{"type", "resize"}, {"columns", m_columns}, {"rows", m_rows}}))
        fail(tr("The remote terminal connection was lost. Pending input was not replayed."));
}

void RemoteTerminalSession::stop()
{
    if (!m_active || m_closing)
        return;
    m_closing = true;
    const QPointer<RemoteTerminalSession> guard(this);
    completeInput(false, 0);
    if (!guard)
        return;
    m_openDeadline->stop();
    m_pending.fill('\0');
    m_pending.clear();
    m_inFlight.fill('\0');
    m_inFlight.clear();
    if (m_process) {
        sendFrame({{"type", "close"}});
        m_process->closeWriteChannel();
    }
    m_closeDeadline->start(2500);
    emit stateChanged();
}

void RemoteTerminalSession::fail(const QString &error)
{
    m_error = error;
    stop();
}

void RemoteTerminalSession::finish(int code)
{
    if (!m_active)
        return;
    m_active = false;
    const QPointer<RemoteTerminalSession> guard(this);
    completeInput(false, 0);
    if (!guard)
        return;
    m_openDeadline->stop();
    m_closeDeadline->stop();
    m_pending.fill('\0');
    m_pending.clear();
    m_inFlight.fill('\0');
    m_inFlight.clear();
    m_request.reset();
    emit stateChanged();
    if (guard)
        emit finished(code);
}

void RemoteTerminalSession::releaseProcess()
{
    auto *process = m_process;
    m_process = nullptr;
    if (!process)
        return;
    process->disconnect(this);
    process->setParent(nullptr);
    if (process->state() == QProcess::NotRunning) {
        process->deleteLater();
        return;
    }
    connect(process, &QProcess::finished, process, &QObject::deleteLater);
    connect(process, &QProcess::started, process, [process] { process->terminate(); });
    process->terminate();
    QTimer::singleShot(2500, process, [process] {
        if (process->state() != QProcess::NotRunning)
            process->kill();
    });
}

bool RemoteTerminalSession::confirmRecovery(const QJsonObject &reference)
{
    if (m_reference.isEmpty() || reference != m_reference)
        return false;
    m_cleanupVerified = true;
    m_nativeClosed = true;
    stop();
    return true;
}

bool RemoteTerminalSession::confirmContainerRecovery(const QJsonObject &reference)
{
    return containerSession() && confirmRecovery(reference);
}
