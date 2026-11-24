#include "ManagedTerminalSession.h"

#include "DockpipeChoices.h"
#include <QCoreApplication>
#include <QJsonDocument>
#include <QPointer>
#include <QProcess>
#include <QTimer>
#include <cmath>
#include <limits>

namespace {
constexpr auto Schema = "dockpipe.local-terminal/v1";
constexpr qsizetype InputLimit = 64 * 1024;
constexpr qsizetype LineLimit = 96 * 1024;
constexpr quint64 MaxId = 9007199254740991ULL;

bool integer(const QJsonValue &value, quint64 &result)
{
    const double number = value.toDouble(-1);
    if (number < 0 || number > double(MaxId) || std::floor(number) != number)
        return false;
    result = quint64(number);
    return true;
}
}

ManagedTerminalSession::ManagedTerminalSession(QObject *parent)
    : QObject(parent), m_startDeadline(new QTimer(this)), m_closeDeadline(new QTimer(this))
{
    m_startDeadline->setSingleShot(true);
    m_closeDeadline->setSingleShot(true);
    connect(m_startDeadline, &QTimer::timeout, this, [this] {
        fail(tr("The terminal core connection was lost. Pending input was not replayed."));
    });
    connect(m_closeDeadline, &QTimer::timeout, this, [this] {
        m_error = tr("Terminal session cleanup could not be verified.");
        if (m_process && m_process->state() != QProcess::NotRunning)
            m_process->kill();
    });
}

ManagedTerminalSession::~ManagedTerminalSession()
{
    releaseProcess();
}

bool ManagedTerminalSession::start(const NativeTerminalLaunch::Snapshot &profile)
{
    if (m_process || m_active)
        return false;
    if (!NativeTerminalLaunch::valid(profile)) {
        m_error = profile.error;
        return false;
    }
    QString error;
    const QString binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty()) {
        m_error = error;
        return false;
    }
    const auto frame = QJsonDocument(QJsonObject{{"schema", Schema}, {"type", "start"}, {"id", 1},
        {"profile", profile.toJson()}, {"columns", m_columns}, {"rows", m_rows}}).toJson(QJsonDocument::Compact) + '\n';
    if (frame.size() > 2 * 1024 * 1024) {
        m_error = tr("The terminal core returned an invalid response.");
        return false;
    }
    m_active = true;
    auto *process = new QProcess(this);
    m_process = process;
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, process, [process] {
            if (process->state() != QProcess::NotRunning)
                process->kill();
        });
    }
    connect(process, &QProcess::started, this, [this, process, frame] {
        if (m_closing) {
            process->terminate();
            return;
        }
        if (process->write(frame) != frame.size())
            fail(tr("The terminal core connection was lost. Pending input was not replayed."));
    });
    connect(process, &QProcess::readyReadStandardOutput, this, &ManagedTerminalSession::drain);
    connect(process, &QProcess::readyReadStandardError, this, [process] { process->readAllStandardError(); });
    connect(process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_cleanupVerified = true; // No core process or native provider exists.
            m_error = tr("Could not start the trusted terminal core client.");
            finish(1);
        }
    });
    connect(process, &QProcess::finished, this, [this](int code, QProcess::ExitStatus) {
        const QPointer<ManagedTerminalSession> guard(this);
        drain();
        if (!guard)
            return;
        if (!m_cleanupVerified && m_error.isEmpty())
            m_error = tr("Terminal session cleanup could not be verified.");
        finish(m_exited ? m_exitCode : code);
    });
    m_startDeadline->start(30000);
    process->start(binary, {"terminal", "--stdio"});
    return true;
}

bool ManagedTerminalSession::writeFrame(QJsonObject frame)
{
    if (!m_process || m_process->state() != QProcess::Running || m_process->bytesToWrite() > 2 * InputLimit)
        return false;
    frame.insert("schema", Schema);
    const auto line = QJsonDocument(frame).toJson(QJsonDocument::Compact) + '\n';
    return line.size() <= LineLimit && m_process->write(line) == line.size();
}

void ManagedTerminalSession::drain()
{
    if (!m_process)
        return;
    const auto bytes = m_process->readAllStandardOutput();
    qsizetype offset = 0;
    while (offset < bytes.size()) {
        const auto newline = bytes.indexOf('\n', offset);
        const auto end = newline < 0 ? bytes.size() : newline;
        if (end - offset > LineLimit - m_lines.size()) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        m_lines.append(bytes.constData() + offset, end - offset);
        offset = end + (newline < 0 ? 0 : 1);
        if (newline < 0)
            break;
        const auto line = std::move(m_lines);
        m_lines.clear();
        const QPointer<ManagedTerminalSession> guard(this);
        consume(line);
        if (!guard || !m_active)
            return;
    }
}

void ManagedTerminalSession::consume(const QByteArray &line)
{
    QJsonParseError parse;
    const auto document = QJsonDocument::fromJson(line, &parse);
    const auto frame = document.object();
    quint64 id = 0;
    if (parse.error != QJsonParseError::NoError || !document.isObject() || frame.value("schema") != Schema
        || !integer(frame.value("id"), id)) {
        fail(tr("The terminal core returned an invalid response."));
        return;
    }
    const QString type = frame.value("type").toString();
    const QPointer<ManagedTerminalSession> guard(this);
    if (type == "ready") {
        const QString session = frame.value("session_id").toString();
        if (id != 1 || m_ready || session.isEmpty() || session.size() > 128 || session.contains(QChar::Null)) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        m_session = session;
        m_ready = true;
        m_startDeadline->stop();
        if (!m_closing) {
            flushResize();
            if (!guard)
                return;
            flushInput();
        }
        if (guard)
            emit stateChanged();
        return;
    }
    if (type == "ack") {
        if (id == m_resizeId && id != 0) {
            m_resizeId = 0;
            if (!m_closing)
                flushResize();
            return;
        }
        quint64 written = 0;
        if (id == 0 || id != m_inputId || !integer(frame.value("bytes"), written)
            || written != quint64(m_inFlight.size())) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        m_inputId = 0;
        m_inFlight.fill('\0');
        m_inFlight.clear();
        completeInput(true, m_inFlightEnd);
        if (guard && !m_closing)
            flushInput();
        return;
    }
    if (type == "output") {
        quint64 cursor = 0;
        quint64 oldest = 0;
        // The core omits zero-valued oldest cursors from its JSON events.
        const auto oldestValue = frame.value("oldest");
        const bool validOldest = oldestValue.isUndefined() || integer(oldestValue, oldest);
        const auto data = QByteArray::fromBase64Encoding(frame.value("data").toString().toLatin1(),
            QByteArray::AbortOnBase64DecodingErrors);
        if (id != 0 || !m_ready || !integer(frame.value("cursor"), cursor) || !validOldest
            || !data || data.decoded.size() > 32 * 1024 || cursor < m_cursor || oldest > cursor
            || cursor < quint64(data.decoded.size())) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        const auto begin = cursor - quint64(data.decoded.size());
        const bool truncated = frame.value("truncated").toBool();
        if (begin != m_cursor && !(truncated && begin >= m_cursor)) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        m_cursor = cursor;
        if (!data.decoded.isEmpty())
            emit output(data.decoded, cursor, truncated);
        return;
    }
    if (type == "closed") {
        const QString cleanup = frame.value("cleanup").toString();
        const auto exitValue = frame.value("exit_code");
        const double exitCode = exitValue.toDouble();
        if (id != 0 || m_exited || (cleanup != "complete" && cleanup != "unknown")
            || (!exitValue.isUndefined() && (!exitValue.isDouble() || std::floor(exitCode) != exitCode
                || exitCode < std::numeric_limits<int>::min() || exitCode > std::numeric_limits<int>::max()))) {
            fail(tr("The terminal core returned an invalid response."));
            return;
        }
        m_exited = true;
        m_exitCode = int(exitCode);
        m_cleanupVerified = cleanup == "complete";
        if (!m_cleanupVerified)
            m_error = tr("Terminal session cleanup could not be verified.");
        m_startDeadline->stop();
        completeInput(false, 0);
        if (!guard)
            return;
        if (m_process)
            m_process->closeWriteChannel();
        m_closeDeadline->start(7000);
        emit stateChanged();
        return;
    }
    if (type == "error") {
        fail(tr("The terminal core connection was lost. Pending input was not replayed."));
        return;
    }
    fail(tr("The terminal core returned an invalid response."));
}

bool ManagedTerminalSession::send(const QByteArray &data)
{
    return sendAcknowledged(data, {});
}

bool ManagedTerminalSession::sendAcknowledged(const QByteArray &data, std::function<void(bool)> completion)
{
    if (!running() || data.isEmpty() || data.size() > InputLimit - m_pending.size() - m_inFlight.size()
        || (completion && m_completions.size() >= 128))
        return false;
    m_pending.append(data);
    m_acceptedBytes += quint64(data.size());
    if (completion)
        m_completions.append({m_acceptedBytes, std::move(completion)});
    flushInput();
    return true;
}

void ManagedTerminalSession::flushInput()
{
    if (!m_ready || !running() || m_inputId || m_pending.isEmpty())
        return;
    if (m_nextId == MaxId) {
        fail(tr("The terminal core returned an invalid response."));
        return;
    }
    m_inputId = ++m_nextId;
    m_inFlight = std::move(m_pending);
    m_pending.clear();
    m_inFlightEnd = m_acceptedBytes;
    if (!writeFrame({{"type", "input"}, {"id", double(m_inputId)}, {"data", QString::fromLatin1(m_inFlight.toBase64())}}))
        fail(tr("The terminal core connection was lost. Pending input was not replayed."));
}

void ManagedTerminalSession::resize(int columns, int rows)
{
    m_columns = qBound(1, columns, 1000);
    m_rows = qBound(1, rows, 1000);
    flushResize();
}

void ManagedTerminalSession::flushResize()
{
    if (!m_ready || !running() || m_resizeId || (m_columns == m_sentColumns && m_rows == m_sentRows))
        return;
    if (m_nextId == MaxId) {
        fail(tr("The terminal core returned an invalid response."));
        return;
    }
    m_resizeId = ++m_nextId;
    m_sentColumns = m_columns;
    m_sentRows = m_rows;
    if (!writeFrame({{"type", "resize"}, {"id", double(m_resizeId)}, {"columns", m_columns}, {"rows", m_rows}}))
        fail(tr("The terminal core connection was lost. Pending input was not replayed."));
}

void ManagedTerminalSession::completeInput(bool success, quint64 through)
{
    QVector<std::function<void(bool)>> completions;
    while (!m_completions.isEmpty() && (!success || m_completions.first().first <= through))
        completions.append(m_completions.takeFirst().second);
    const QPointer<ManagedTerminalSession> guard(this);
    for (const auto &completion : completions) {
        completion(success);
        if (!guard)
            return;
    }
}

void ManagedTerminalSession::stop()
{
    if (!m_active || m_closing)
        return;
    m_closing = true;
    m_startDeadline->stop();
    const QPointer<ManagedTerminalSession> guard(this);
    completeInput(false, 0);
    if (!guard)
        return;
    m_pending.fill('\0');
    m_pending.clear();
    if (m_process) {
        if (m_nextId < MaxId)
            writeFrame({{"type", "close"}, {"id", double(++m_nextId)}});
        m_process->closeWriteChannel();
    }
    m_closeDeadline->start(7000);
    emit stateChanged();
}

void ManagedTerminalSession::fail(const QString &error)
{
    m_error = error;
    stop();
}

void ManagedTerminalSession::finish(int code)
{
    if (!m_active)
        return;
    m_active = false;
    m_startDeadline->stop();
    m_closeDeadline->stop();
    const QPointer<ManagedTerminalSession> guard(this);
    completeInput(false, 0);
    if (!guard)
        return;
    m_pending.fill('\0');
    m_pending.clear();
    m_inFlight.fill('\0');
    m_inFlight.clear();
    emit stateChanged();
    if (guard)
        emit finished(code);
}

void ManagedTerminalSession::releaseProcess()
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
    QTimer::singleShot(7000, process, [process] {
        if (process->state() != QProcess::NotRunning)
            process->kill();
    });
}
