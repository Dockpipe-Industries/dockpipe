#include "RemoteFeatureClient.h"

#include "DockpipeChoices.h"
#include <QDateTime>
#include <QJsonArray>
#include <QJsonDocument>
#include <QProcess>
#include <QTimer>
#include <QUuid>

namespace {
constexpr qsizetype MaxRequestBytes = 2 << 20;
constexpr qsizetype MaxResponseBytes = 20 << 20;
constexpr qsizetype MaxDiagnosticsBytes = 64 << 10;

void reapProcess(QProcess *process)
{
    if (!process)
        return;
    process->disconnect();
    process->setParent(nullptr);
    if (process->state() == QProcess::NotRunning) {
        process->deleteLater();
        return;
    }
    QObject::connect(process, &QProcess::finished, process, &QObject::deleteLater);
    if (auto *application = QCoreApplication::instance()) {
        QObject::connect(application, &QCoreApplication::aboutToQuit, process, [process] {
            if (process->state() != QProcess::NotRunning)
                process->kill();
        });
    }
    process->terminate();
    QTimer::singleShot(2500, process, [process] {
        if (process->state() != QProcess::NotRunning)
            process->kill();
    });
}
}

QJsonObject RemoteFeatureClient::prepareIntent(const QJsonObject &target, const QString &root,
                                               const QString &action, const QJsonObject &payload)
{
    // The caller retains this object before dispatch. Core computes its canonical
    // digest; retrying this exact intent cannot silently authorize a second write.
    QString nonce = QUuid::createUuid().toString(QUuid::Id128);
    nonce += QUuid::createUuid().toString(QUuid::Id128);
    QJsonObject intent{{"schema", QStringLiteral("dockpipe.remote.features/v1")},
        {"target", target}, {"action", action}, {"payload", payload}, {"nonce", nonce},
        {"deadline", QDateTime::currentDateTimeUtc().addSecs(25).toString(Qt::ISODateWithMs)}};
    if (!root.isEmpty())
        intent.insert(QStringLiteral("root"), root);
    return intent;
}

RemoteFeatureClient::RemoteFeatureClient(QObject *owner, QJsonObject intent, bool catalog,
                                         Completion completion)
    : QObject(owner), m_owner(owner), m_intent(std::move(intent)), m_completion(std::move(completion)),
      m_deadline(new QTimer(this)), m_catalog(catalog)
{
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this] {
        stop(m_catalog ? Outcome::Failure : Outcome::Unknown,
             tr("Remote operation timed out. Refresh state before retrying a write."));
    });
}

RemoteFeatureClient::~RemoteFeatureClient()
{
    m_completion = {};
    releaseProcess();
}

RemoteFeatureClient::Cancel RemoteFeatureClient::catalog(QObject *owner, Completion completion, int timeoutMs)
{
    return start(owner, {}, true, std::move(completion), timeoutMs);
}

RemoteFeatureClient::Cancel RemoteFeatureClient::invoke(QObject *owner, const QJsonObject &intent,
                                                       Completion completion, int timeoutMs)
{
    return start(owner, intent, false, std::move(completion), timeoutMs);
}

RemoteFeatureClient::Cancel RemoteFeatureClient::start(QObject *owner, const QJsonObject &intent,
    bool catalog, Completion completion, int timeoutMs)
{
    if (!owner)
        return {};
    auto *call = new RemoteFeatureClient(owner, intent, catalog, std::move(completion));
    const QPointer<RemoteFeatureClient> guard(call);
    QTimer::singleShot(0, call, [call, timeoutMs] { call->launch(timeoutMs); });
    return [guard] {
        if (guard)
            guard->cancel();
    };
}

void RemoteFeatureClient::launch(int timeoutMs)
{
    if (m_finished || m_stopping)
        return;
    const auto input = m_catalog ? QByteArray() : QJsonDocument(m_intent).toJson(QJsonDocument::Compact);
    if (timeoutMs < 1 || timeoutMs > 30000 || input.size() > MaxRequestBytes || (!m_catalog && m_intent.isEmpty())) {
        finish({Outcome::Failure, {}, tr("Invalid remote feature request bounds.")});
        return;
    }
    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        finish({Outcome::Failure, {}, error});
        return;
    }
    m_deadline->start(timeoutMs);
    auto *process = new QProcess(this);
    m_process = process;
    if (auto *application = QCoreApplication::instance()) {
        connect(application, &QCoreApplication::aboutToQuit, process, [process] {
            if (process->state() != QProcess::NotRunning)
                process->kill();
        });
    }
    connect(process, &QProcess::readyReadStandardOutput, this, &RemoteFeatureClient::drain);
    connect(process, &QProcess::readyReadStandardError, this, &RemoteFeatureClient::drain);
    connect(process, &QProcess::started, this, [this, process, input] {
        if (m_stopping) {
            process->terminate();
            return;
        }
        if (!input.isEmpty() && process->write(input) != input.size()) {
            stop(Outcome::Unknown, tr("Remote request delivery is uncertain. Refresh state before retrying a write."));
            return;
        }
        process->closeWriteChannel();
    });
    connect(process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            finish({Outcome::Failure, {}, tr("Could not start the trusted Dockpipe remote client.")});
    });
    connect(process, &QProcess::finished, this, [this](int exitCode, QProcess::ExitStatus status) {
        completed(exitCode, status == QProcess::NormalExit);
    });
    QStringList arguments{QStringLiteral("remote"), m_catalog ? QStringLiteral("features") : QStringLiteral("feature-call"),
                          QStringLiteral("--auth"), QStringLiteral("never")};
    if (!m_catalog)
        arguments << QStringLiteral("--request") << QStringLiteral("-");
    process->start(program, arguments);
}

void RemoteFeatureClient::drain()
{
    if (!m_process || m_finished)
        return;
    const auto output = m_process->readAllStandardOutput();
    const auto diagnostics = m_process->readAllStandardError();
    if (output.size() > MaxResponseBytes - m_stdout.size()
        || diagnostics.size() > MaxDiagnosticsBytes - m_stderr.size()) {
        stop(m_catalog ? Outcome::Failure : Outcome::Unknown,
             tr("Remote response exceeded its transport limit."));
        return;
    }
    m_stdout += output;
    m_stderr += diagnostics;
}

void RemoteFeatureClient::completed(int exitCode, bool normalExit)
{
    if (m_finished)
        return;
    const QPointer<RemoteFeatureClient> guard(this);
    drain();
    if (!guard || m_finished)
        return;
    if (m_stopping) {
        finish({m_stopOutcome, {}, m_stopError});
        return;
    }
    QJsonParseError parseError;
    const auto document = QJsonDocument::fromJson(m_stdout, &parseError);
    if (parseError.error != QJsonParseError::NoError) {
        finish({m_catalog ? Outcome::Failure : Outcome::Unknown, {},
                tr("Remote result is unavailable. Refresh state before retrying a write.")});
        return;
    }
    if (m_catalog) {
        if (!normalExit || exitCode != 0 || !document.isArray() || document.array().size() > 64) {
            finish({Outcome::Failure, {}, tr("Remote machine catalog is unavailable.")});
            return;
        }
        finish({Outcome::Success, document.array(), {}});
        return;
    }
    const auto operation = document.object();
    const auto request = operation.value(QStringLiteral("request")).toObject();
    if (request.value(QStringLiteral("target")) != m_intent.value(QStringLiteral("target"))
        || request.value(QStringLiteral("root")) != m_intent.value(QStringLiteral("root"))
        || request.value(QStringLiteral("action")) != m_intent.value(QStringLiteral("action"))
        || request.value(QStringLiteral("id")).toString().size() != 64) {
        finish({Outcome::Unknown, {}, tr("Remote result identity changed. Refresh the selected machine.")});
        return;
    }
    const auto result = operation.value(QStringLiteral("result")).toObject();
    const QString status = operation.value(QStringLiteral("status")).toString();
    if (normalExit && exitCode == 0 && status == QStringLiteral("success")
        && !operation.value(QStringLiteral("receipt_only")).toBool()
        && result.value(QStringLiteral("status")) == QStringLiteral("success")) {
        if (request.value(QStringLiteral("nonce")) != m_intent.value(QStringLiteral("nonce"))
            || request.value(QStringLiteral("payload")) != m_intent.value(QStringLiteral("payload"))
            || QDateTime::fromString(request.value(QStringLiteral("deadline")).toString(), Qt::ISODateWithMs)
                != QDateTime::fromString(m_intent.value(QStringLiteral("deadline")).toString(), Qt::ISODateWithMs)) {
            finish({Outcome::Unknown, {}, tr("Remote result does not match the captured operation.")});
            return;
        }
        finish({Outcome::Success, result.value(QStringLiteral("payload")), {}});
    } else {
        const auto outcome = status == QStringLiteral("cancelled") ? Outcome::Cancelled
            : status == QStringLiteral("failure") ? Outcome::Failure : Outcome::Unknown;
        finish({outcome, operation, result.value(QStringLiteral("error")).toString().left(1024)});
    }
}

void RemoteFeatureClient::cancel()
{
    stop(Outcome::Cancelled, tr("Cancellation requested for the original remote operation. Refresh state before retrying a write."));
}

void RemoteFeatureClient::stop(Outcome outcome, const QString &error)
{
    if (m_finished || m_stopping)
        return;
    m_stopping = true;
    m_stopOutcome = outcome;
    m_stopError = error;
    m_deadline->stop();
    // Core handles SIGTERM by cancelling the captured operation. Keep a grace
    // period for that request, then reap the child without blocking the GUI.
    releaseProcess();
    finish({outcome, {}, error});
}

void RemoteFeatureClient::releaseProcess()
{
    auto *process = m_process;
    m_process = nullptr;
    reapProcess(process);
}

void RemoteFeatureClient::finish(Result result)
{
    if (m_finished)
        return;
    m_finished = true;
    m_deadline->stop();
    releaseProcess();
    auto completion = std::move(m_completion);
    const QPointer<RemoteFeatureClient> guard(this);
    if (m_owner && completion)
        completion(result);
    if (guard)
        deleteLater();
}
