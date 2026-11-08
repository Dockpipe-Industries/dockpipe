#include "WorkspaceFilesClient.h"
#include <QCoreApplication>
#include "DockpipeChoices.h"

#include <QDir>
#include <QJsonDocument>
#include <QJsonParseError>
#include <QProcess>
#include <QTimer>

namespace {
constexpr qsizetype MaxProtocolBytes = 2 * 1024 * 1024;
constexpr qsizetype MaxDiagnosticBytes = 16 * 1024;

// Do not destroy a live QProcess on the GUI thread: its destructor waits.
void reap(QProcess *process)
{
    process->setParent(nullptr);
    if (process->state() == QProcess::NotRunning) {
        process->deleteLater();
        return;
    }
    QObject::connect(process, &QProcess::finished, process, &QObject::deleteLater);
    QObject::connect(process, &QProcess::errorOccurred, process, [process](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            process->deleteLater();
    });
    process->kill();
}
}

bool WorkspaceFilesClient::Context::operator==(const Context &other) const
{
    return kind == other.kind && localRoot == other.localRoot && nodeId == other.nodeId
        && fingerprint == other.fingerprint && workerGeneration == other.workerGeneration
        && rootId == other.rootId && generation == other.generation;
}

WorkspaceFilesClient::WorkspaceFilesClient(QObject *parent)
    : QObject(parent), m_deadline(new QTimer(this))
{
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this]() {
        stop(Outcome::Failure, tr("Workspace file operation timed out; its effect may need refresh."));
    });
}

WorkspaceFilesClient::~WorkspaceFilesClient()
{
    m_active = false;
    m_completion = {};
    if (m_remoteCancel)
        m_remoteCancel();
    releaseProcess();
}

void WorkspaceFilesClient::setContext(const Context &context)
{
    if (m_context == context)
        return;
    m_context = context;
    cancel();
}

void WorkspaceFilesClient::setRemoteDispatch(RemoteDispatch dispatch)
{
    cancel();
    m_remoteDispatch = std::move(dispatch);
}

void WorkspaceFilesClient::run(const QJsonObject &request, QObject *owner,
                              Completion completion, int timeoutMs)
{
    if (!owner || !completion)
        return;
    auto reject = [self = QPointer<WorkspaceFilesClient>(this), owner = QPointer<QObject>(owner), completion, context = m_context]
        (Outcome outcome, const QString &error) {
        QTimer::singleShot(0, owner, [self, owner, completion, context, outcome, error]() {
            if (self && owner)
                completion(Result{outcome, {}, error, context});
        });
    };
    if (busy()) {
        reject(Outcome::Busy, tr("A workspace file operation is already running."));
        return;
    }
    const QByteArray envelope = QJsonDocument(QJsonObject{
        {"root", m_context.localRoot}, {"request", request}}).toJson(QJsonDocument::Compact);
    if (envelope.size() > MaxProtocolBytes || timeoutMs < 1 || timeoutMs > 30000) {
        reject(Outcome::Failure, tr("Workspace file request exceeds its transport contract."));
        return;
    }
    if (m_context.kind == Context::Kind::Remote) {
        if (!m_remoteDispatch || m_context.nodeId.isEmpty() || m_context.fingerprint.isEmpty()
            || m_context.workerGeneration.isEmpty() || m_context.rootId.isEmpty()) {
            reject(Outcome::Failure, tr("The selected remote Files context is unavailable."));
            return;
        }
    } else if (!QDir::isAbsolutePath(m_context.localRoot)) {
        reject(Outcome::Failure, tr("Workspace Files requires an absolute selected root."));
        return;
    }

    m_attemptContext = m_context;
    m_owner = owner;
    m_completion = std::move(completion);
    m_active = true;
    m_stopped = false;
    m_stdout.clear();
    m_stderr.clear();
    const quint64 attempt = ++m_attempt;
    m_ownerDestroyed = connect(owner, &QObject::destroyed, this, &WorkspaceFilesClient::cancel);
    m_deadline->start(timeoutMs);
    const QPointer<WorkspaceFilesClient> lifetime(this);
    emit busyChanged(true);
    if (!lifetime || !m_active || m_attempt != attempt)
        return;

    if (m_attemptContext.kind == Context::Kind::Remote) {
        const QPointer<WorkspaceFilesClient> self(this);
        Cancel cancellation = m_remoteDispatch(m_attemptContext, request, [self, attempt](const Result &result) {
            if (self && self->m_active && self->m_attempt == attempt)
                self->finish(result);
        });
        if (m_active && m_attempt == attempt)
            m_remoteCancel = std::move(cancellation);
        return;
    }

    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        finish(Result{Outcome::Failure, {}, error, m_attemptContext});
        return;
    }
    auto *process = new QProcess(this);
    m_process = process;
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, process, [process] {
            if (process->state() != QProcess::NotRunning)
                process->kill();
        });
    }
    connect(process, &QProcess::readyReadStandardOutput, this, &WorkspaceFilesClient::drain);
    connect(process, &QProcess::readyReadStandardError, this, &WorkspaceFilesClient::drain);
    connect(process, &QProcess::started, this, [this, process, envelope]() {
        if (m_stopped) {
            process->kill();
            return;
        }
        if (process->write(envelope) != envelope.size())
            stop(Outcome::Failure, tr("Could not send the workspace file request."));
        process->closeWriteChannel();
    });
    connect(process, &QProcess::errorOccurred, this, [this, process](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            finish(Result{Outcome::Failure, {}, process->errorString(), m_attemptContext});
    });
    connect(process, &QProcess::finished, this, [this](int exitCode, QProcess::ExitStatus status) {
        drain();
        if (m_stopped) {
            finish(Result{m_stopOutcome, {}, m_stopError, m_attemptContext});
            return;
        }
        if (exitCode != 0 || status != QProcess::NormalExit) {
            QString error = QString::fromUtf8(m_stderr).trimmed();
            if (error.isEmpty())
                error = tr("Workspace file helper failed (exit %1). Check the installed Dockpipe CLI.").arg(exitCode);
            finish(Result{Outcome::Failure, {}, error, m_attemptContext});
            return;
        }
        QJsonParseError parseError;
        const auto document = QJsonDocument::fromJson(m_stdout, &parseError);
        if (parseError.error != QJsonParseError::NoError || !document.isObject()
            || document.object().value("schema") != QStringLiteral("dockpipe.files/v1")) {
            finish(Result{Outcome::Failure, {}, tr("Invalid workspace file response."), m_attemptContext});
            return;
        }
        finish(Result{Outcome::Success, document.object(), {}, m_attemptContext});
    });
    process->start(program, {QStringLiteral("files"), QStringLiteral("--request"), QStringLiteral("-")});
}

void WorkspaceFilesClient::drain()
{
    if (!m_process)
        return;
    // Read fixed chunks so an untrusted child cannot make us allocate its entire output.
    for (const auto channel : {QProcess::StandardOutput, QProcess::StandardError}) {
        m_process->setReadChannel(channel);
        QByteArray &retained = channel == QProcess::StandardOutput ? m_stdout : m_stderr;
        const qsizetype limit = channel == QProcess::StandardOutput ? MaxProtocolBytes : MaxDiagnosticBytes;
        while (m_process->bytesAvailable() > 0) {
            const QByteArray chunk = m_process->read(8192);
            if (retained.size() + chunk.size() > limit) {
                stop(Outcome::Failure, tr("Workspace file response exceeded its transport limit."));
                return;
            }
            retained += chunk;
        }
    }
}

void WorkspaceFilesClient::cancel()
{
    stop(Outcome::Cancelled, tr("Workspace file operation was cancelled; refresh before retrying a write."));
}

void WorkspaceFilesClient::stop(Outcome outcome, const QString &error)
{
    if (!m_active || m_stopped)
        return;
    m_stopped = true;
    m_stopOutcome = outcome;
    m_stopError = error;
    if (m_remoteCancel) {
        auto cancelRemote = std::move(m_remoteCancel);
        cancelRemote();
    }
    if (m_process) {
        m_process->kill();
        return;
    }
    finish(Result{outcome, {}, error, m_attemptContext});
}

void WorkspaceFilesClient::releaseProcess()
{
    if (!m_process)
        return;
    auto *process = m_process;
    m_process = nullptr;
    process->disconnect(this);
    reap(process);
}

void WorkspaceFilesClient::finish(Result result)
{
    if (!m_active)
        return;
    result.context = m_attemptContext;
    if (result.success() && (result.value.value("schema") != QStringLiteral("dockpipe.files/v1")
        || !result.value.value("path").isString()
        || QJsonDocument(result.value).toJson(QJsonDocument::Compact).size() > MaxProtocolBytes)) {
        result.outcome = Outcome::Failure;
        result.value = {};
        result.error = tr("Invalid workspace file response.");
    }
    if (m_stopped) {
        result.outcome = m_stopOutcome;
        result.error = m_stopError;
        result.value = {};
    }
    auto completion = std::move(m_completion);
    const QPointer<QObject> owner = m_owner;
    disconnect(m_ownerDestroyed);
    m_deadline->stop();
    m_remoteCancel = {};
    m_active = false;
    releaseProcess();
    // Defer user code so even a failed launch has the same asynchronous contract.
    if (owner) {
        const QPointer<WorkspaceFilesClient> self(this);
        QTimer::singleShot(0, owner, [self, owner, completion, result]() mutable {
            if (!self || !owner || !completion)
                return;
            if (!(result.context == self->m_context)) {
                result.outcome = Outcome::Cancelled;
                result.value = {};
                result.error = tr("The workspace file context changed before completion.");
            }
            completion(result);
        });
    }
    emit busyChanged(false);
}
