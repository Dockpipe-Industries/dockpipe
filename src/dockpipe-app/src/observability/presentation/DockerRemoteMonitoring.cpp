#include "DockerObservabilityWidget.h"
#include "DockpipeChoices.h"
#include "MonitoringMachineSelector.h"
#include "NativeHostSnapshot.h"
#include "NativeHostDockerAction.h"
#include "NativeHostDockerQuery.h"
#include "DockerResourceInspector.h"
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <utility>
#include <QPlainTextEdit>
#include <QTableWidget>
#include <QTimer>
#include <QRegularExpression>

DockerObservabilityWidget::~DockerObservabilityWidget()
{
    ++m_generation;
    cancelFeatureRead();
}

bool DockerObservabilityWidget::isRemoteTarget() const
{
    return !m_featureTarget.isEmpty() || !m_machine->currentData().toString().isEmpty();
}

QString DockerObservabilityWidget::targetNode() const
{
    return m_featureTarget.isEmpty() ? m_machine->currentData().toString() : m_featureTarget.value("node").toString();
}

bool DockerObservabilityWidget::isNativeHostTarget() const
{
    return m_featureTarget.isEmpty()
        && m_machine->currentData().toString() == NativeHostConnection::selectionKey();
}

bool DockerObservabilityWidget::canControlTarget() const
{
    if (isNativeHostTarget())
        return nativeContainerControlsAvailable();
    return !isRemoteTarget() || (!m_featureTarget.isEmpty() && m_featureCanControl);
}

bool DockerObservabilityWidget::setFeatureTarget(const QJsonObject &target, bool canControl, QString *error)
{
    if (error)
        error->clear();
    const bool changed = target != m_featureTarget;
    if (changed && m_containerActionBusy) {
        if (error)
            *error = tr("Wait for the current container control operation before changing machines.");
        return false;
    }
    if (!target.isEmpty()) {
        static const QRegularExpression identity(QStringLiteral("^[0-9a-f]{64}$"));
        if (target.value("node").toString().isEmpty()
            || !identity.match(target.value("fingerprint").toString()).hasMatch()
            || !identity.match(target.value("generation").toString()).hasMatch()) {
            if (error)
                *error = tr("Select a machine with a verified fingerprint and worker generation.");
            return false;
        }
    }
    m_featureTarget = target;
    if (m_featureCatalog.value("target").toObject() != target)
        m_featureCatalog = {};
    m_featureCanControl = canControl;
    m_machine->setVisible(target.isEmpty());
    m_machineLabel->setVisible(target.isEmpty());
    if (changed)
        changeMachine();
    else
        updateInspector();
    return true;
}

bool DockerObservabilityWidget::setFeatureCatalog(const QJsonObject &catalog, QString *error)
{
    if (!setFeatureTarget(catalog.value("target").toObject(), catalog.value("docker_control").toBool(), error))
        return false;
    m_featureCatalog = catalog;
    updateInspector();
    return true;
}

void DockerObservabilityWidget::cancelFeatureRead()
{
    m_localReadCancelled->store(true);
    m_localReadCancelled = std::make_shared<std::atomic_bool>(false);
    cancelFeatureDetails();
    ++m_featureReadGeneration;
    auto cancel = std::move(m_cancelFeatureRead);
    m_cancelFeatureRead = {};
    m_featureReadBusy = false;
    if (cancel)
        cancel();
}

void DockerObservabilityWidget::cancelFeatureDetails()
{
    ++m_featureDetailGeneration;
    m_featureDetailKey.clear();
    auto cancel = std::move(m_cancelFeatureDetail);
    m_cancelFeatureDetail = {};
    if (cancel)
        cancel();
}

void DockerObservabilityWidget::refreshFeatureDetails(const QString &kind, const QString &object)
{
    if (!m_active || m_monitorPaused || m_containerActionBusy || object.isEmpty())
        return;
    const auto key = kind + ":" + object;
    if (m_featureDetailKey == key)
        return;
    cancelFeatureDetails();
    m_featureDetailKey = key;
    const auto target = m_featureTarget;
    const auto generation = m_generation;
    const auto detailGeneration = m_featureDetailGeneration;
    auto *details = m_containerDetails;
    if (kind == "network")
        details = m_networkDetails;
    else if (kind == "volume")
        details = m_volumeDetails;
    details->setPlainText(tr("Loading resource details…"));
    if (kind == "container")
        m_containerLogs->setPlainText(tr("Loading container logs…"));
    const auto intent = RemoteFeatureClient::prepareIntent(target, {}, "containers.inspect", {{"kind", kind}, {"object", object}});
    m_cancelFeatureDetail = RemoteFeatureClient::invoke(this, intent,
        [this, target, generation, detailGeneration, key, kind, object, details](const auto &result) {
            if (generation != m_generation || detailGeneration != m_featureDetailGeneration
                || target != m_featureTarget || m_selectedResource != key || !m_active || m_monitorPaused)
                return;
            m_cancelFeatureDetail = {};
            const auto receipt = result.value.toObject();
            const bool valid = result.outcome == RemoteFeatureClient::Outcome::Success
                && receipt.value("schema").toString() == "dockpipe.containers/v1"
                && receipt.value("action").toString() == "inspect"
                && receipt.value("kind").toString() == kind && receipt.value("object").toString() == object
                && receipt.value("inspect").isArray() && receipt.value("inspect").toArray().size() == 1;
            if (valid) {
                details->setPlainText(QString::fromUtf8(QJsonDocument(receipt.value("inspect").toArray()).toJson(QJsonDocument::Indented)));
            } else {
                details->setPlainText(tr("Could not inspect resource: %1").arg(result.error.isEmpty() ? tr("Invalid resource receipt.") : result.error));
            }
            if (kind != "container") {
                m_featureDetailKey.clear();
                return;
            }
            const auto logsIntent = RemoteFeatureClient::prepareIntent(target, {}, "containers.logs", {{"kind", kind}, {"object", object}});
            m_cancelFeatureDetail = RemoteFeatureClient::invoke(this, logsIntent,
                [this, target, generation, detailGeneration, key, object](const auto &logs) {
                    if (generation != m_generation || detailGeneration != m_featureDetailGeneration
                        || target != m_featureTarget || m_selectedResource != key || !m_active || m_monitorPaused)
                        return;
                    m_cancelFeatureDetail = {};
                    m_featureDetailKey.clear();
                    const auto receipt = logs.value.toObject();
                    const bool valid = logs.outcome == RemoteFeatureClient::Outcome::Success
                        && receipt.value("schema").toString() == "dockpipe.containers/v1"
                        && receipt.value("action").toString() == "logs"
                        && receipt.value("kind").toString() == "container" && receipt.value("object").toString() == object
                        && (!receipt.contains("logs") || receipt.value("logs").isString());
                    m_containerLogs->setPlainText(valid ? receipt.value("logs").toString()
                        : tr("Could not load container logs: %1").arg(logs.error.isEmpty() ? tr("Invalid log receipt.") : logs.error));
                });
        });
}

void DockerObservabilityWidget::refreshFeatureSnapshot()
{
    if (!m_active || m_monitorPaused || m_featureReadBusy || m_containerActionBusy)
        return;
    const auto target = m_featureTarget;
    const auto generation = m_generation;
    const auto readGeneration = m_featureReadGeneration;
    const auto intent = RemoteFeatureClient::prepareIntent(target, {}, "containers.snapshot", {});
    m_featureReadBusy = true;
    m_cancelFeatureRead = RemoteFeatureClient::invoke(this, intent, [this, target, generation, readGeneration](const auto &result) {
        if (readGeneration != m_featureReadGeneration || generation != m_generation || target != m_featureTarget || !m_active || m_monitorPaused)
            return;
        m_featureReadBusy = false;
        m_cancelFeatureRead = {};
        if (result.outcome != RemoteFeatureClient::Outcome::Success || !result.value.isObject()) {
            applyRemoteSnapshot({{"available", false}});
            setStatus(tr("%1 · Remote Docker inventory unavailable: %2").arg(targetNode(), result.error));
            return;
        }
        applyRemoteSnapshot(result.value.toObject());
        m_hasLoadedOnce = true;
    });
}

void DockerObservabilityWidget::runRemoteContainerAction(const QString &action, const QString &id, const QString &name)
{
    const auto target = m_featureTarget;
    const auto generation = m_generation;
    const auto intent = RemoteFeatureClient::prepareIntent(target, {}, "containers." + action, {{"container", id}});
    m_containerActionBusy = true;
    cancelFeatureRead();
    updateInspector();
    setStatus(tr("%1 · %2: %3…").arg(targetNode(), name, action));
    RemoteFeatureClient::invoke(this, intent, [this, target, generation, action, id, name](const auto &result) {
        m_containerActionBusy = false;
        updateInspector();
        if (generation != m_generation || target != m_featureTarget)
            return;
        const auto receipt = result.value.toObject();
        if (result.outcome != RemoteFeatureClient::Outcome::Success
            || receipt.value("schema").toString() != "dockpipe.containers/v1"
            || receipt.value("action").toString() != action || receipt.value("container").toString() != id) {
            if (result.outcome == RemoteFeatureClient::Outcome::Success)
                RemoteFeatureClient::retainUncertain(result.reference);
            const QString reason = result.error.isEmpty() ? tr("Invalid container control receipt.") : result.error;
            setStatus(tr("%1 · %2 failed for %3: %4").arg(targetNode(), action, name, reason));
            return;
        }
        refresh();
    });
}

void DockerObservabilityWidget::setNodes(const QStringList &nodes)
{
    m_machine->setNodes(nodes);
}

void DockerObservabilityWidget::setNativeHostConnection(NativeHostConnection *connection)
{
    if (m_nativeHost)
        disconnect(m_nativeHost, nullptr, this, nullptr);
    m_nativeHost = connection;
    if (connection) {
        connect(connection, &NativeHostConnection::statusChanged, this, &DockerObservabilityWidget::nativeHostChanged);
        connect(connection, &QObject::destroyed, this, &DockerObservabilityWidget::nativeHostChanged);
    }
    m_machine->setNativeHost(connection ? connection->label() : tr("Native host — unavailable"), true);
    changeMachine();
}

void DockerObservabilityWidget::nativeHostChanged()
{
    m_machine->setNativeHost(m_nativeHost ? m_nativeHost->label() : tr("Native host — unavailable"), false);
    if (isNativeHostTarget())
        changeMachine();
}

void DockerObservabilityWidget::changeMachine()
{
    ++m_generation;
    cancelFeatureRead();
    m_remoteCommand.cancel();
    m_nativeActionCommand.cancel();
    cancelNativeDetails();
    m_nativeActionNotice.clear();
    clearRemoteSnapshot(tr("Waiting for the selected machine…"));
    if (m_active && !m_monitorPaused)
        QTimer::singleShot(0, this, &DockerObservabilityWidget::refresh);
}

bool DockerObservabilityWidget::nativeContainerControlsAvailable() const
{
    return isNativeHostTarget()
        && m_nativeHost && m_nativeHost->available() && m_snapshot.containersOk
        && m_selectedResource == QStringLiteral("container:") + selectedContainerId()
        && NativeHostDockerAction::validRequest("start", selectedContainerId());
}

QString DockerObservabilityWidget::remoteInspectionNotice() const
{
    if (isNativeHostTarget())
        return tr("Native host inspection and recent logs are available here. Use the host terminal for container shells.");
    return tr("Remote monitoring shares inventory and topology. Application logs and interactive controls remain local.");
}

void DockerObservabilityWidget::runNativeContainerAction(const QString &action, const QString &id, const QString &name)
{
    if (!nativeContainerControlsAvailable() || m_containerActionBusy || m_nativeActionCommand.busy())
        return;
    const auto plan = m_nativeHost->dockerPlan(action, id);
    if (!plan.valid()) {
        setStatus(plan.error);
        return;
    }
    m_containerActionBusy = true;
    m_nativeActionNotice.clear();
    updateInspector();
    setStatus(tr("%1: %2…").arg(name, action));
    const quint64 generation = m_generation;
    const quint64 revision = m_nativeHost->revision();
    m_nativeActionCommand.runBounded(plan.program, plan.arguments,
        [this, generation, revision, action, id, name](bool ok, const QByteArray &output) {
            m_containerActionBusy = false;
            updateInspector();
            if (generation != m_generation || !m_nativeHost || !m_nativeHost->available()
                || revision != m_nativeHost->revision())
                return;
            if (ok && NativeHostDockerAction::succeeded(output, action, id))
                m_nativeActionNotice = tr("%1 completed for %2.").arg(action, name);
            else
                m_nativeActionNotice = tr("%1 was not confirmed for %2. Refreshing current state.").arg(action, name);
            setStatus(m_nativeActionNotice);
            refresh();
        }, 10000, 64 * 1024);
}

void DockerObservabilityWidget::cancelNativeDetails()
{
    ++m_nativeQuerySerial;
    m_nativeQueryKey.clear();
    m_nativeQueryCommand.cancel();
}

void DockerObservabilityWidget::refreshNativeDetails(const QString &kind, const QString &id)
{
    const QString key = kind + ":" + id;
    if (!m_active || m_monitorPaused || !m_nativeHost || !m_nativeHost->available()
        || !isNativeHostTarget()
        || key != m_selectedResource)
        return;
    if (m_nativeQueryCommand.busy()) {
        if (key != m_nativeQueryKey)
            cancelNativeDetails();
        return;
    }
    const auto plan = m_nativeHost->dockerQueryPlan(kind, id);
    if (!plan.valid()) {
        m_inspector->setStatus(plan.error);
        return;
    }
    if (key != m_nativeInspectKey) {
        m_nativeInspectKey.clear();
        m_nativeInspect = {};
        m_containerDetails->clear();
        m_containerLogs->clear();
        m_networkDetails->clear();
        m_volumeDetails->clear();
        updateInspector();
    }
    m_nativeQueryKey = key;
    m_inspector->setStatus(tr("Loading native host inspection…"));
    const quint64 serial = ++m_nativeQuerySerial;
    const quint64 generation = m_generation;
    const quint64 revision = m_nativeHost->revision();
    m_nativeQueryCommand.runBounded(plan.program, plan.arguments,
        [this, serial, generation, revision, kind, id, key](bool ok, const QByteArray &output) {
            if (serial != m_nativeQuerySerial) {
                // A different selection may have arrived while cancellation drained.
                // Start it only after the previous child has actually finished.
                QTimer::singleShot(0, this, [this]() {
                    refreshNativeDetails(m_selectedResource.section(':', 0, 0), m_selectedResource.section(':', 1));
                });
                return;
            }
            m_nativeQueryKey.clear();
            if (generation != m_generation || !m_active || m_monitorPaused || key != m_selectedResource
                || !m_nativeHost || !m_nativeHost->available() || revision != m_nativeHost->revision()
                || !isNativeHostTarget())
                return;
            QString selectedId = selectedContainerId();
            if (kind == "network")
                selectedId = selectedNetworkId();
            else if (kind == "volume")
                selectedId = selectedVolumeName();
            if (selectedId != id)
                return;
            NativeHostDockerQuery query;
            if (!ok || !NativeHostDockerQuery::decode(output, kind, id, &query)) {
                m_nativeInspectKey.clear();
                m_nativeInspect = {};
                const QString message = tr("Native host inspection unavailable or cancelled. Refresh to try again.");
                QPlainTextEdit *details = m_containerDetails;
                if (kind == "network")
                    details = m_networkDetails;
                else if (kind == "volume")
                    details = m_volumeDetails;
                details->setPlainText(message);
                if (kind == "container")
                    m_containerLogs->setPlainText(message);
                updateInspector();
                m_inspector->setStatus(message);
                return;
            }
            m_nativeInspectKey = key;
            m_nativeInspect = query.inspect;
            QPlainTextEdit *details = m_containerDetails;
            if (kind == "network")
                details = m_networkDetails;
            else if (kind == "volume")
                details = m_volumeDetails;
            // Compact JSON keeps presentation bounded even for deeply nested
            // raw objects whose pretty-print whitespace could expand greatly.
            details->setPlainText(QString::fromUtf8(QJsonDocument(query.inspect).toJson(QJsonDocument::Compact)));
            QString notice = tr("Native host inspection refreshed.");
            if (kind == "container") {
                QString logs = query.logs;
                if (!query.logsError.isEmpty()) {
                    logs = tr("Recent logs are unavailable for this container. Its logging driver may not support reading logs.");
                    notice = tr("Inspection refreshed; recent logs unavailable.");
                } else if (query.logsTruncated) {
                    logs += tr("\n[Recent logs limited to 64 KiB.]");
                    notice = tr("Inspection refreshed; recent logs limited to 64 KiB.");
                }
                m_containerLogs->setPlainText(logs);
            }
            updateInspector();
            m_inspector->setStatus(notice);
        }, 10000, 1024 * 1024);
}

void DockerObservabilityWidget::clearRemoteSnapshot(const QString &status)
{
    cancelNativeDetails();
    m_nativeInspectKey.clear();
    m_nativeDisplayedKey.clear();
    m_nativeInspect = {};
    m_selectedResource.clear();
    m_pendingContainerDetailId.clear();
    m_displayedContainerDetailId.clear();
    m_pendingNetworkDetailId.clear();
    m_pendingVolumeDetailId.clear();
    m_containerDetails->clear();
    m_containerLogs->clear();
    m_networkDetails->clear();
    m_volumeDetails->clear();
    m_containers->setRowCount(0);
    m_networks->setRowCount(0);
    m_volumes->setRowCount(0);
    m_snapshot = {};
    updateGraph(m_snapshot);
    updateInspector();
    updateSummary();
    m_hasLoadedOnce = false;
    setStatus(status);
}

void DockerObservabilityWidget::refreshRemote(bool allowAuthentication)
{
    if (!m_featureTarget.isEmpty()) {
        refreshFeatureSnapshot();
        return;
    }
    if (!m_active || m_monitorPaused || m_remoteCommand.busy())
        return;
    const QString node = m_machine->currentData().toString();
    const quint64 generation = m_generation;
    if (node == NativeHostConnection::selectionKey()) {
        if (!m_nativeHost || !m_nativeHost->available()) {
            clearRemoteSnapshot(tr("Native host unavailable. Open this application through its authenticated host session."));
            return;
        }
        const auto plan = m_nativeHost->plan(NativeHostConnection::Operation::Snapshot);
        if (!plan.valid()) {
            clearRemoteSnapshot(plan.error);
            return;
        }
        const quint64 revision = m_nativeHost->revision();
        const QString expectedNode = m_nativeHost->status().node;
        m_remoteCommand.runBounded(plan.program, plan.arguments,
            [this, generation, revision, expectedNode](bool ok, const QByteArray &output) {
                if (generation != m_generation || !m_active || m_monitorPaused
                    || !m_nativeHost || !m_nativeHost->available() || revision != m_nativeHost->revision())
                    return;
                QJsonObject snapshot;
                if (!ok || !NativeHostSnapshot::decode(output, expectedNode, "docker", &snapshot)) {
                    clearRemoteSnapshot(tr("Native host Docker monitoring unavailable. Refresh the authenticated host session."));
                    m_nativeHost->refresh();
                    return;
                }
                if (!snapshot.value("available").toBool()) {
                    clearRemoteSnapshot(tr("Native host Docker inventory is unavailable for this account."));
                    return;
                }
                applyRemoteSnapshot(snapshot);
            }, 10000, 1024 * 1024);
        return;
    }
    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        clearRemoteSnapshot(error);
        return;
    }
    m_remoteCommand.run(program,
        {"remote", "observe", "--node", node, "--auth",
         allowAuthentication ? "auto" : "never"}, [this, node, generation](bool ok, const QByteArray &output) {
            if (generation != m_generation || node != m_machine->currentData().toString()
                || !m_active || m_monitorPaused)
                return;
            const auto envelope = QJsonDocument::fromJson(output).object();
            if (!ok || envelope.value("schema").toString() != "dockpipe.machine_snapshot.v1"
                || envelope.value("node").toString() != node || envelope.value("state").toString() != "live"
                || !envelope.value("docker").isObject()) {
                clearRemoteSnapshot(tr("%1 · Docker monitoring unavailable or stale. On the paired worker, enable: dockpipe remote monitoring --system --docker. The worker and broker must support monitoring.").arg(node));
                return;
            }
            applyRemoteSnapshot(envelope.value("docker").toObject());
        }, allowAuthentication ? 150000 : 5000);
}

void DockerObservabilityWidget::applyRemoteSnapshot(const QJsonObject &snapshot)
{
    DockerSnapshot converted;
    converted.containersOk = snapshot.value("available").toBool();
    converted.networksOk = converted.containersOk;
    converted.volumesOk = converted.containersOk;
    const QStringList collections{"containers", "networks", "volumes", "topology"};
    for (const auto &collection : collections) {
        if (snapshot.value(collection).toArray().size() > 256) {
            applyRemoteSnapshot({{"available", false}});
            setStatus(tr("%1 · Remote Docker inventory exceeds the supported bound.").arg(targetNode()));
            return;
        }
    }
    QStringList notices;
    for (const auto &notice : snapshot.value("notices").toArray())
        notices << notice.toString();
    converted.topologyNotice = notices.join(" · ");
    if (!converted.containersOk) {
        m_containers->setRowCount(0);
        m_networks->setRowCount(0);
        m_volumes->setRowCount(0);
        converted.containersError = tr("Remote Docker inventory unavailable. ") + converted.topologyNotice;
        converted.networksError = converted.containersError;
        converted.volumesError = converted.containersError;
    }
    for (const auto &value : snapshot.value("containers").toArray()) {
        const auto row = value.toObject();
        converted.containers.append({row.value("ID").toString(), row.value("Names").toString(),
            row.value("State").toString(), row.value("Status").toString(), row.value("Image").toString(),
            row.value("Ports").toString(), row.value("RunningFor").toString()});
    }
    for (const auto &value : snapshot.value("networks").toArray()) {
        const auto row = value.toObject();
        converted.networks.append({row.value("ID").toString(), row.value("Name").toString(),
            row.value("Driver").toString(), row.value("Scope").toString()});
    }
    for (const auto &value : snapshot.value("volumes").toArray()) {
        const auto row = value.toObject();
        converted.volumes.append({row.value("Name").toString(), row.value("Driver").toString(), tr("Not shared")});
    }
    converted.topology = snapshot.value("topology").toArray();
    if (snapshot.value("truncated").toBool())
        converted.topologyNotice += tr(" Inventory limited to 256 objects of each kind.");
    applySnapshot(converted);
    QString status = targetNode();
    if (isNativeHostTarget())
        status = m_machine->currentText() + tr(" · Native host inventory");
    else
        status += canControlTarget() ? tr(" · Remote inventory") : tr(" · Remote inventory · read-only");
    if (!converted.topologyNotice.isEmpty())
        status += " · " + converted.topologyNotice;
    if (!m_nativeActionNotice.isEmpty())
        status += " · " + m_nativeActionNotice;
    setStatus(status);
}
