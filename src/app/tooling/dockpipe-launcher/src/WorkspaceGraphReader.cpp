#include "WorkspaceGraphReader.h"
#include "WorkspaceGraphPage.h"
#include <QCoreApplication>
#include <QJsonDocument>
#include <QProcess>

namespace {
// A cancelled helper must exit before any view starts another read. This also
// bounds rapid open/close churn when an OS filesystem syscall is slow to cancel.
QPointer<QProcess> activeHelper;
}

WorkspaceGraphReader::WorkspaceGraphReader(QObject *parent, int deadlineMilliseconds)
    : QObject(parent), m_deadlineMilliseconds(qBound(1, deadlineMilliseconds, 25000))
{
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        m_failure = tr("Directory read timed out. Retry or choose a smaller folder.");
        m_timedOut = true;
        m_abandoned = true;
        const quint64 token = m_pending.isEmpty() ? m_token : m_pendingToken;
        m_pending = {};
        emit completed(token, {{"error", m_failure}});
        if (m_process)
            m_process->kill();
    });
}

WorkspaceGraphReader::~WorkspaceGraphReader()
{
    if (!m_process)
        return;
    auto *process = m_process.data();
    disconnect(process, nullptr, this, nullptr);
    process->kill();
    // Never wait for filesystem I/O in a widget destructor. The transient process
    // owns its last cleanup callback and no longer references the closed graph.
    if (process->state() == QProcess::NotRunning) {
        process->deleteLater();
    } else {
        connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), process, &QObject::deleteLater);
    }
}

bool WorkspaceGraphReader::busy() const { return m_process || !m_pending.isEmpty(); }
bool WorkspaceGraphReader::helperActive() { return !activeHelper.isNull(); }

void WorkspaceGraphReader::request(quint64 token, const QJsonObject &request)
{
    if (m_process && m_timedOut) {
        emit completed(token, {{"error", tr("The timed-out filesystem read is still stopping. No additional reader will be started.")}});
        return;
    }
    m_pending = request;
    m_pendingToken = token;
    if (m_process) {
        m_abandoned = true;
        m_process->kill();
        m_timeout.start(m_deadlineMilliseconds);
    } else {
        startPending();
    }
}

void WorkspaceGraphReader::cancel()
{
    m_pending = {};
    m_abandoned = true;
    m_timeout.stop();
    if (m_process)
        m_process->kill();
}

void WorkspaceGraphReader::startPending()
{
    if (m_pending.isEmpty())
        return;
    const auto request = m_pending;
    m_token = m_pendingToken;
    m_pending = {};
    if (activeHelper) {
        emit completed(m_token, {{"error", tr("The previous directory read is still stopping. Retry in a moment.")}});
        return;
    }
    m_process = new QProcess(QCoreApplication::instance());
    activeHelper = m_process;
    m_abandoned = false;
    m_timedOut = false;
    m_output.clear();
    m_failure.clear();
    connect(m_process, &QProcess::readyReadStandardOutput, this, &WorkspaceGraphReader::drain);
    connect(m_process, &QProcess::readyReadStandardError, this, [this]() {
        // The helper emits structured errors. Drain diagnostics without retaining them.
        m_process->readAllStandardError();
    });
    connect(m_process, &QProcess::started, this, [this, request]() {
        m_process->write(QJsonDocument(request).toJson(QJsonDocument::Compact));
        m_process->closeWriteChannel();
    });
    connect(m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int exitCode, QProcess::ExitStatus status) { finish(exitCode == 0 && status == QProcess::NormalExit); });
    connect(m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_failure = tr("Cannot start the directory reader: %1").arg(m_process->errorString());
            finish(false);
        }
    });
    m_timeout.start(m_deadlineMilliseconds);
    m_process->start(QCoreApplication::applicationFilePath(), {"--workspace-graph-page"});
}

void WorkspaceGraphReader::drain()
{
    if (!m_process)
        return;
    const QByteArray bytes = m_process->readAllStandardOutput();
    if (bytes.size() > WorkspaceGraphPage::maximumReplyBytes - m_output.size()) {
        m_failure = tr("Directory response exceeds the 2 MiB limit. Reduce the page size.");
        m_process->kill();
        return;
    }
    m_output += bytes;
}

void WorkspaceGraphReader::finish(bool success)
{
    if (!m_process)
        return;
    drain();
    m_timeout.stop();
    auto *finished = m_process.data();
    disconnect(finished, nullptr, this, nullptr);
    m_process = nullptr;
    activeHelper = nullptr;
    finished->deleteLater();
    if (!m_abandoned) {
        QJsonParseError error;
        const auto document = QJsonDocument::fromJson(m_output, &error);
        if (!success || !m_failure.isEmpty() || error.error != QJsonParseError::NoError || !document.isObject()) {
            emit completed(m_token, {{"error", m_failure.isEmpty() ? tr("Directory read failed. Refresh to retry.") : m_failure}});
        } else {
            emit completed(m_token, document.object());
        }
    }
    startPending();
}
