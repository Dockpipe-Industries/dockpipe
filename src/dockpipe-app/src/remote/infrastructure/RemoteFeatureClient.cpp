#include "RemoteFeatureClient.h"

#include "DockpipeChoices.h"
#include <QDateTime>
#include <QJsonArray>
#include <QJsonDocument>
#include <QProcess>
#include <QTimer>
#include <QUuid>
#include <QSettings>

namespace {
constexpr qsizetype MaxRequestBytes = 2 << 20;
constexpr qsizetype MaxResponseBytes = 20 << 20;
constexpr qsizetype MaxDiagnosticsBytes = 64 << 10;

constexpr auto RecoverySettingsKey = "remote/uncertainFeatureOperations";
constexpr int MaximumRecoveries = 128;

class CallState final : public QObject {
public:
    explicit CallState(QObject *parent) : QObject(parent)
    {
        setObjectName(QStringLiteral("remoteFeatureCallState"));
    }
    QString program;
    QString command;
    QString recoveryResource;
    QJsonObject reference;
    bool dispatched = false;
    bool mutation = false;
    bool recovery = false;
};

CallState *state(RemoteFeatureClient *client)
{
    return static_cast<CallState *>(client->findChild<QObject *>(
        QStringLiteral("remoteFeatureCallState"), Qt::FindDirectChildrenOnly));
}

QJsonObject referenceOnly(const QJsonObject &value)
{
    QJsonObject reference;
    for (const auto &key : {"schema", "id", "target", "root", "action", "deadline"}) {
        if (value.contains(key))
            reference.insert(key, value.value(key));
    }
    if (reference.contains("target")) {
        const auto original = reference.value("target").toObject();
        reference.insert("target", QJsonObject{{"node", original.value("node")},
            {"fingerprint", original.value("fingerprint")}, {"generation", original.value("generation")}});
    }
    return reference;
}

bool digest(const QString &value)
{
    if (value.size() != 64)
        return false;
    for (const QChar character : value) {
        if (!(character >= '0' && character <= '9') && !(character >= 'a' && character <= 'f'))
            return false;
    }
    return true;
}

bool validReference(const QJsonObject &reference)
{
    const auto target = reference.value("target").toObject();
    return reference.value("schema") == QStringLiteral("dockpipe.remote.features/v1")
        && digest(reference.value("id").toString())
        && !target.value("node").toString().isEmpty()
        && digest(target.value("fingerprint").toString())
        && digest(target.value("generation").toString())
        && !reference.value("action").toString().isEmpty()
        && QDateTime::fromString(reference.value("deadline").toString(), Qt::ISODateWithMs).isValid()
        && QJsonDocument(reference).toJson(QJsonDocument::Compact).size() <= 4096;
}

bool sameMetadata(const QJsonObject &left, const QJsonObject &right, bool requireId)
{
    for (const auto &key : {"schema", "target", "root", "action"}) {
        if (left.value(key) != right.value(key))
            return false;
    }
    if (requireId && left.value("id") != right.value("id"))
        return false;
    return QDateTime::fromString(left.value("deadline").toString(), Qt::ISODateWithMs)
        == QDateTime::fromString(right.value("deadline").toString(), Qt::ISODateWithMs);
}

bool mutationAction(const QString &action)
{
    // Storage classification mirrors the core contract; core preparation and
    // worker grants remain authoritative for admission.
    const QStringList mutations{"files.write", "files.create", "files.rename", "files.delete", "files.mkdir",
        "git.commit", "git.pull", "git.stage", "git.unstage", "git.discard", "git.commit-index",
        "git.branch-create", "git.branch-switch", "git.branch-delete", "git.fetch", "git.merge",
        "git.stash-save", "git.stash-apply", "git.stash-pop", "git.stash-drop", "git.revert",
        "git.cherry-pick", "git.operation-continue", "git.operation-skip", "git.operation-abort",
        "git.resolve-stage", "git.resolve-ours", "git.resolve-theirs", "git.resolve-delete",
        "git.push", "git.tag-publish", "containers.start", "containers.stop", "containers.restart", "containers.terminal-recover", "terminal.host-recover"};
    return mutations.contains(action);
}

QJsonArray loadRecoveries(bool *valid = nullptr)
{
    if (valid)
        *valid = true;
    const QByteArray bytes = QSettings().value(RecoverySettingsKey).toByteArray();
    if (bytes.isEmpty())
        return {};
    QJsonParseError parseError{};
    const auto document = bytes.size() <= MaximumRecoveries * 8192
        ? QJsonDocument::fromJson(bytes, &parseError) : QJsonDocument();
    if (parseError.error != QJsonParseError::NoError || !document.isArray()
        || document.array().size() > MaximumRecoveries) {
        if (valid)
            *valid = false;
        return {};
    }
    const auto entries = document.array();
    QJsonArray retained;
    for (const auto &entry : entries) {
        const auto stored = entry.toObject();
        const auto reference = referenceOnly(stored);
        if (retained.size() < MaximumRecoveries && validReference(reference)
            && mutationAction(reference.value("action").toString())) {
            auto safe = reference;
            if (stored.value("reconciled").toBool())
                safe.insert("reconciled", true);
            if (digest(stored.value("recovery_resource").toString()))
                safe.insert("recovery_resource", stored.value("recovery_resource"));
            retained.append(safe);
        } else if (valid) {
            *valid = false;
        }
    }
    return retained;
}

bool saveRecovery(const QJsonObject &reference, bool remove, bool reconciled = false)
{
    if (!validReference(reference) || !mutationAction(reference.value("action").toString()))
        return true;
    bool valid = false;
    const auto retained = loadRecoveries(&valid);
    if (!valid)
        return false;
    QJsonArray entries;
    bool found = false;
    for (const auto &entry : retained) {
        const auto existing = entry.toObject();
        if (existing.value("id") == reference.value("id") && existing.value("target") == reference.value("target")) {
            found = true;
            if (!remove) {
                auto safe = referenceOnly(reference);
                if (reconciled || existing.value("reconciled").toBool())
                    safe.insert("reconciled", true);
                const auto resource = reference.value("recovery_resource").toString(existing.value("recovery_resource").toString());
                if (digest(resource))
                    safe.insert("recovery_resource", resource);
                entries.append(safe);
            }
        } else {
            entries.append(existing);
        }
    }
    if (!remove && !found) {
        if (entries.size() >= MaximumRecoveries)
            return false;
        auto safe = referenceOnly(reference);
        if (reconciled)
            safe.insert("reconciled", true);
        if (digest(reference.value("recovery_resource").toString()))
            safe.insert("recovery_resource", reference.value("recovery_resource"));
        entries.append(safe);
    }
    QSettings settings;
    settings.setValue(RecoverySettingsKey, QJsonDocument(entries).toJson(QJsonDocument::Compact));
    settings.sync();
    return settings.status() == QSettings::NoError;
}

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
    new CallState(this);
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this] {
        stop(state(this)->dispatched || state(this)->recovery ? Outcome::Unknown : Outcome::Failure,
             tr("Remote operation timed out. Inspect its original receipt before another write."));
    });
}

void RemoteFeatureClient::retainUncertain(const QJsonObject &reference)
{
    saveRecovery(referenceOnly(reference), false);
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

RemoteFeatureClient::Cancel RemoteFeatureClient::inspect(QObject *owner, const QJsonObject &reference,
                                                        Completion completion, int timeoutMs)
{
    return recover(owner, reference, QStringLiteral("feature-status"), std::move(completion), timeoutMs);
}

RemoteFeatureClient::Cancel RemoteFeatureClient::cancelOperation(QObject *owner, const QJsonObject &reference,
                                                                Completion completion, int timeoutMs)
{
    return recover(owner, reference, QStringLiteral("feature-cancel"), std::move(completion), timeoutMs);
}

RemoteFeatureClient::Cancel RemoteFeatureClient::reconcile(QObject *owner, const QJsonObject &reference,
                                                          Completion completion, int timeoutMs)
{
    return recover(owner, reference, QStringLiteral("feature-reconcile"), std::move(completion), timeoutMs);
}

RemoteFeatureClient::Cancel RemoteFeatureClient::retireContainer(QObject *owner, const QJsonObject &reference,
                                                               Completion completion, int timeoutMs)
{
    return recover(owner, reference, QStringLiteral("feature-retire-container"), std::move(completion), timeoutMs);
}

QJsonArray RemoteFeatureClient::pendingRecoveries()
{
    return loadRecoveries();
}

RemoteFeatureClient::Cancel RemoteFeatureClient::recover(QObject *owner, const QJsonObject &reference,
    const QString &command, Completion completion, int timeoutMs)
{
    if (!owner)
        return {};
    auto *call = new RemoteFeatureClient(owner, referenceOnly(reference), false, std::move(completion));
    auto *current = state(call);
    current->command = command;
    current->reference = referenceOnly(reference);
    current->recoveryResource = reference.value("recovery_resource").toString();
    current->recovery = true;
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
    auto *current = state(this);
    const auto input = m_catalog ? QByteArray() : QJsonDocument(m_intent).toJson(QJsonDocument::Compact);
    if (timeoutMs < 1 || timeoutMs > 30000 || input.size() > MaxRequestBytes || (!m_catalog && m_intent.isEmpty())
        || (current->recovery && !validReference(current->reference))) {
        finish({Outcome::Failure, {}, tr("Invalid remote feature request bounds.")});
        return;
    }
    if ((current->command == QStringLiteral("feature-reconcile") || current->command == QStringLiteral("feature-retire-container"))
        && (!mutationAction(current->reference.value("action").toString())
            || (current->command == QStringLiteral("feature-retire-container")
                && !current->reference.value("action").toString().startsWith("containers.")))) {
        finish({Outcome::Failure, {}, tr("The recovery strategy does not match the captured mutation.")});
        return;
    }
    QString error;
    current->program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (current->program.isEmpty()) {
        finish({Outcome::Failure, {}, error});
        return;
    }
    // Preparation and execution share this one deadline; preparation cannot
    // renew the caller's total operation budget.
    m_deadline->start(timeoutMs);
    QString command = QStringLiteral("feature-prepare");
    if (m_catalog)
        command = QStringLiteral("features");
    else if (current->recovery)
        command = current->command;
    launchProcess(command, m_intent);
}

void RemoteFeatureClient::launchProcess(const QString &command, const QJsonObject &request)
{
    auto *current = state(this);
    current->command = command;
    m_stdout.clear();
    m_stderr.clear();
    const auto input = m_catalog ? QByteArray() : QJsonDocument(request).toJson(QJsonDocument::Compact);
    if (input.size() > MaxRequestBytes) {
        finish({Outcome::Failure, {}, tr("Prepared remote request exceeds its transport limit.")});
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
    connect(process, &QProcess::readyReadStandardOutput, this, &RemoteFeatureClient::drain);
    connect(process, &QProcess::readyReadStandardError, this, &RemoteFeatureClient::drain);
    connect(process, &QProcess::started, this, [this, process, input, command] {
        auto *current = state(this);
        if (m_stopping) {
            process->terminate();
            return;
        }
        if (command == QStringLiteral("feature-call")) {
            // Once a call can receive stdin its effect may be uncertain. Save
            // only identity metadata before transmitting any effectful request.
            if (current->mutation && !saveRecovery(current->reference, false)) {
                stop(Outcome::Failure, tr("Cannot retain remote mutation recovery metadata. Resolve saved operations or repair settings before another write."));
                return;
            }
            current->dispatched = true;
        }
        if (!input.isEmpty() && process->write(input) != input.size()) {
            stop(current->dispatched ? Outcome::Unknown : Outcome::Failure,
                 tr("Remote request delivery is uncertain. Inspect the original operation."));
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
    QStringList arguments{QStringLiteral("remote"), command == QStringLiteral("feature-retire-container") ? QStringLiteral("feature-reconcile") : command};
    if (command == QStringLiteral("feature-retire-container"))
        arguments << QStringLiteral("--recovery") << QStringLiteral("retire-container")
                  << QStringLiteral("--expected-container") << current->recoveryResource;
    if (command != QStringLiteral("feature-prepare"))
        arguments << QStringLiteral("--auth") << QStringLiteral("never");
    if (!m_catalog)
        arguments << QStringLiteral("--request") << QStringLiteral("-");
    process->start(current->program, arguments);
}

void RemoteFeatureClient::drain()
{
    if (!m_process || m_finished)
        return;
    for (const auto channel : {QProcess::StandardOutput, QProcess::StandardError}) {
        m_process->setReadChannel(channel);
        QByteArray &retained = channel == QProcess::StandardOutput ? m_stdout : m_stderr;
        const qsizetype limit = channel == QProcess::StandardOutput ? MaxResponseBytes : MaxDiagnosticsBytes;
        while (m_process->bytesAvailable() > 0) {
            const QByteArray chunk = m_process->read(8192);
            if (chunk.size() > limit - retained.size()) {
                stop(state(this)->dispatched || state(this)->recovery ? Outcome::Unknown : Outcome::Failure,
                    tr("Remote response exceeded its transport limit."));
                return;
            }
            retained += chunk;
        }
    }
}

void RemoteFeatureClient::completed(int exitCode, bool normalExit)
{
    if (m_finished)
        return;
    const QPointer<RemoteFeatureClient> guard(this);
    drain();
    if (!guard || m_finished)
        return;
    auto *current = state(this);
    if (m_stopping) {
        finish({m_stopOutcome, {}, m_stopError});
        return;
    }
    QJsonParseError parseError;
    const auto document = QJsonDocument::fromJson(m_stdout, &parseError);
    if (parseError.error != QJsonParseError::NoError) {
        finish({current->dispatched || current->recovery ? Outcome::Unknown : Outcome::Failure, {},
                tr("Remote result is unavailable. Inspect the original operation before another write.")});
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
    if (current->command == QStringLiteral("feature-prepare")) {
        const auto reference = document.object();
        if (!normalExit || exitCode != 0 || !validReference(reference)
            || reference != referenceOnly(reference) || !sameMetadata(reference, m_intent, false)
            || (m_intent.contains("id") && m_intent.value("id") != reference.value("id"))) {
            finish({Outcome::Failure, {}, tr("Core could not prepare the exact captured remote operation. No operation was dispatched.")});
            return;
        }
        current->reference = reference;
        current->mutation = mutationAction(reference.value("action").toString());
        m_intent.insert("id", reference.value("id"));
        releaseProcess();
        if (!m_deadline->isActive()) {
            finish({Outcome::Failure, {}, tr("Remote operation preparation exhausted its deadline.")});
            return;
        }
        launchProcess(QStringLiteral("feature-call"), m_intent);
        return;
    }
    const auto operation = document.object();
    const auto request = operation.value("request").toObject();
    if (!document.isObject() || !sameMetadata(request, current->reference, true)) {
        finish({Outcome::Unknown, {}, tr("Remote result identity changed. Inspect the original captured operation.")});
        return;
    }
    const auto result = operation.value("result").toObject();
    const QString status = operation.value("status").toString();
    if (current->recovery) {
        if (digest(operation.value("recovery_resource").toString())) {
            current->reference.insert("recovery_resource", operation.value("recovery_resource"));
            saveRecovery(current->reference, false);
        }
        const bool knownStatus = status == "queued" || status == "running" || status == "unknown"
            || status == "success" || status == "failure" || status == "cancelled";
        const bool reconciled = operation.value("reconciled").toBool();
        if (!knownStatus || (result.contains("status") && result.value("status") != status)
            || (reconciled && (status != "unknown" || !operation.value("receipt_only").toBool()
                || !operation.value("recovery_requested").toBool() || !operation.value("cancel_requested").toBool()))) {
            finish({Outcome::Unknown, {}, tr("Invalid recovery receipt. The original operation remains uncertain.")});
            return;
        }
        if (normalExit && (status == "success" || status == "failure") && result.value("status") == status)
            saveRecovery(current->reference, true);
        if (normalExit && exitCode == 0 && status == "unknown" && operation.value("receipt_only").toBool()
            && operation.value("recovery_requested").toBool() && operation.value("cancel_requested").toBool()
            && operation.value("reconciled").toBool())
            saveRecovery(current->reference, false, true);
        Outcome outcome = Outcome::Unknown;
        if (normalExit && exitCode == 0 && status == "success")
            outcome = Outcome::Success;
        else if (normalExit && status == "failure")
            outcome = Outcome::Failure;
        finish({outcome, operation, result.value("error").toString().left(1024)});
        return;
    }
    if (normalExit && exitCode == 0 && status == QStringLiteral("success")
        && !operation.value(QStringLiteral("receipt_only")).toBool()
        && result.value(QStringLiteral("status")) == QStringLiteral("success")) {
        if (request.value("nonce") != m_intent.value("nonce")
            || request.value("payload") != m_intent.value("payload")) {
            finish({Outcome::Unknown, {}, tr("Remote result does not match the captured operation.")});
            return;
        }
        saveRecovery(current->reference, true);
        finish({Outcome::Success, result.value("payload"), {}});
    } else {
        if (status == "failure" && result.value("status") == status)
            saveRecovery(current->reference, true);
        Outcome outcome = Outcome::Unknown;
        if (status == QStringLiteral("cancelled"))
            outcome = Outcome::Cancelled;
        else if (status == QStringLiteral("failure"))
            outcome = Outcome::Failure;
        finish({outcome, operation, result.value("error").toString().left(1024)});
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
    result.reference = state(this)->reference;
    m_deadline->stop();
    releaseProcess();
    auto completion = std::move(m_completion);
    const QPointer<RemoteFeatureClient> guard(this);
    if (m_owner && completion)
        completion(result);
    if (guard)
        deleteLater();
}
