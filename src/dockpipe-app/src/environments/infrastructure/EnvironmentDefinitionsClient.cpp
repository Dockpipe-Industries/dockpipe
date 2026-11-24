#include "EnvironmentDefinitionsClient.h"

#include "DockpipeChoices.h"

#include <QDir>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QProcess>
#include <QRegularExpression>
#include <QSet>
#include <QTimer>
#include <algorithm>
#include <utility>

namespace {

// Core admits up to 2,048 personal entries and up to 2,048 explicit paths.
constexpr qsizetype maxDefinitionEntries = 4096;
constexpr qsizetype maxResponseBytes = 32 * 1024 * 1024;
constexpr int requestTimeoutMs = 15000;
const QRegularExpression instanceIdPattern(QStringLiteral("\\A[0-9a-f]{32}\\z"));
const QRegularExpression runIdPattern(QStringLiteral("\\A[0-9a-f]{8}\\z"));
const QRegularExpression containerIdPattern(QStringLiteral("\\A[0-9a-f]{64}\\z"));

bool parseDefinitions(const QByteArray &bytes, QVector<EnvironmentDefinition> &definitions)
{
    const auto document = QJsonDocument::fromJson(bytes);
    const auto result = document.object();
    const auto entries = result.value("definitions").toArray();
    if (result.value("schema") != "dockpipe.environment.definitions/v1"
        || !result.value("definitions").isArray() || entries.size() > maxDefinitionEntries) {
        return false;
    }

    QSet<QString> paths;
    definitions.reserve(entries.size());
    for (const auto &entry : entries) {
        const auto object = entry.toObject();
        EnvironmentDefinition definition;
        definition.scope = object.value("scope").toString();
        definition.path = object.value("path").toString();
        if ((definition.scope != "personal" && definition.scope != "explicit")
            || !QDir::isAbsolutePath(definition.path) || definition.path.startsWith(QLatin1Char(':'))
            || definition.path.contains(QChar::Null) || paths.contains(definition.path)) {
            return false;
        }

        const auto fields = object.value("definition").toObject();
        if (object.contains("error")) {
            definition.error = object.value("error").toString();
            if (object.contains("definition") || (definition.error != "missing"
                && definition.error != "invalid" && definition.error != "name-mismatch")) {
                return false;
            }
            if (definition.error == "invalid" && object.value("reason") == "unsupported-kind") {
                paths.insert(definition.path);
                continue;
            }
        } else {
            definition.name = fields.value("name").toString();
            definition.kind = fields.value("kind").toString();
            const auto provision = fields.value("provision").toObject();
            definition.workflow = provision.value("workflow").toString();
            definition.package = provision.value("package").toString();
            definition.container = fields.value("container").toObject();
            definition.vm = fields.value("vm").toObject();
            const bool directContainer = fields.contains("container");
            const bool directVm = fields.contains("vm");
            const auto storage = fields.value("storage").toObject();
            definition.storageProfile = storage.value("profile").toString();
            if (fields.value("schema") != "dockpipe.environment/v1" || definition.name.isEmpty()
                || (definition.kind != "vm" && definition.kind != "container")
                || (directContainer && (definition.kind != "container" || fields.contains("provision")
                    || definition.container.value("image").toString().isEmpty()))
                || (directVm && (definition.kind != "vm" || fields.contains("provision") || directContainer
                    || definition.vm.value("os").toString().isEmpty()
                    || definition.vm.value("cpus").toInt() <= 0 || definition.vm.value("memory_gib").toInt() <= 0
                    || definition.vm.value("disk_gib").toInt() <= 0))
                || (!directContainer && !directVm && definition.workflow.isEmpty())
                || (provision.contains("package") && !provision.value("package").isString())
                || (fields.contains("storage") && (definition.kind != "vm" || definition.storageProfile.isEmpty()))) {
                return false;
            }
            const auto instanceId = object.value("instance_id").toString();
            if (instanceIdPattern.match(instanceId).hasMatch())
                definition.instanceId = instanceId;

            if (object.contains("runtime")) {
                const auto runtime = object.value("runtime").toObject();
                const auto state = runtime.value("state").toString();
                if (runtime.value("schema") != "dockpipe.environment.runtime/v1"
                    || definition.instanceId.isEmpty() || runtime.value("instance_id") != definition.instanceId
                    || (state != "running" && state != "stopped" && state != "unverified")) {
                    return false;
                }
                definition.runtimeState = state;
                const auto runId = runtime.value("run_id").toString();
                const auto containerId = runtime.value("container_id").toString();
                if (definition.kind == "container" && state == "running" && containerIdPattern.match(containerId).hasMatch())
                    definition.runtimeContainerId = containerId;
                if (definition.kind == "vm" && runIdPattern.match(runId).hasMatch())
                    definition.runtimeRunId = runId;
                const auto runtimeWorkdir = runtime.value("workdir").toString();
                if (!definition.runtimeRunId.isEmpty() && QDir::isAbsolutePath(runtimeWorkdir)
                    && !runtimeWorkdir.startsWith(QLatin1Char(':')) && !runtimeWorkdir.contains(QChar::Null)) {
                    definition.runtimeWorkdir = runtimeWorkdir;
                }
            }
        }
        paths.insert(definition.path);
        definitions.append(std::move(definition));
    }
    return true;
}

}

EnvironmentDefinitionsClient::~EnvironmentDefinitionsClient()
{
    cancel();
}

void EnvironmentDefinitionsClient::load(const QString &workdir, const QStringList &files)
{
    cancel();
    QString error;
    const auto binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty()) {
        emit failed(error);
        return;
    }

    auto *process = new QProcess(this);
    m_process = process;
    process->setProgram(binary);
    process->setWorkingDirectory(workdir.isEmpty() ? QDir::homePath() : workdir);
    process->setStandardErrorFile(QProcess::nullDevice());
    process->setStandardInputFile(QProcess::nullDevice());
    QStringList arguments{QStringLiteral("environment"), QStringLiteral("definitions"), QStringLiteral("--observe")};
    for (const auto &file : files)
        arguments << QStringLiteral("--file") << file;
    process->setArguments(arguments);

    connect(process, &QProcess::readyReadStandardOutput, this, [this, process]() {
        drain(process);
    });
    connect(process, &QProcess::errorOccurred, this, [this, process](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            fail(process, tr("Could not start Dockpipe to read environment definitions."));
    });
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this, process](int code, QProcess::ExitStatus status) {
            if (!drain(process))
                return;
            if (status != QProcess::NormalExit || code != 0) {
                fail(process, tr("Dockpipe could not list environment definitions."));
                return;
            }

            QVector<EnvironmentDefinition> definitions;
            if (!parseDefinitions(m_output, definitions)) {
                fail(process, tr("Dockpipe returned an invalid environment definition list."));
                return;
            }

            cancel();
            emit loaded(definitions);
        });
    QTimer::singleShot(requestTimeoutMs, process, [this, process]() {
        fail(process, tr("Reading environment definitions timed out."));
    });
    process->start();
}

bool EnvironmentDefinitionsClient::drain(QProcess *process)
{
    if (m_process != process)
        return false;

    const auto available = std::min(process->bytesAvailable(), qint64(maxResponseBytes - m_output.size() + 1));
    m_output += process->read(available);
    if (m_output.size() > maxResponseBytes) {
        fail(process, tr("The environment definition list exceeds its size limit."));
        return false;
    }
    return true;
}

void EnvironmentDefinitionsClient::fail(QProcess *process, const QString &error)
{
    if (m_process != process)
        return;

    cancel();
    emit failed(error);
}

void EnvironmentDefinitionsClient::cancel()
{
    auto *process = std::exchange(m_process, nullptr);
    m_output.clear();
    if (!process)
        return;

    disconnect(process, nullptr, this, nullptr);
    if (process->state() == QProcess::NotRunning) {
        process->deleteLater();
    } else {
        connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished),
                process, &QProcess::deleteLater);
        connect(process, &QProcess::started, process, [process]() {
            process->kill();
        });
        connect(process, &QProcess::errorOccurred, process, [process](QProcess::ProcessError error) {
            if (error == QProcess::FailedToStart)
                process->deleteLater();
        });
        process->kill();
    }
}
