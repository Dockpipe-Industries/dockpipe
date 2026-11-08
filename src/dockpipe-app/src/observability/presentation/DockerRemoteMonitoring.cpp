#include "DockerObservabilityWidget.h"
#include "DockpipeChoices.h"
#include "MonitoringMachineSelector.h"
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
    return m_featureMode ? !m_featureTarget.isEmpty() : !m_machine->currentData().toString().isEmpty();
}

QString DockerObservabilityWidget::targetNode() const
{
    return m_featureMode ? m_featureTarget.value("node").toString() : m_machine->currentData().toString();
}

bool DockerObservabilityWidget::canControlTarget() const
{
    return !isRemoteTarget() || (m_featureMode && m_featureCanControl);
}

bool DockerObservabilityWidget::setFeatureTarget(const QJsonObject &target, bool canControl, QString *error)
{
    if (error)
        error->clear();
    const bool changed = !m_featureMode || target != m_featureTarget;
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
    m_featureMode = true;
    m_featureTarget = target;
    m_featureCanControl = canControl;
    m_machine->hide();
    m_machineLabel->hide();
    if (changed)
        changeMachine();
    else
        updateInspector();
    return true;
}

void DockerObservabilityWidget::cancelFeatureRead()
{
    ++m_featureReadGeneration;
    auto cancel = std::move(m_cancelFeatureRead);
    m_cancelFeatureRead = {};
    m_featureReadBusy = false;
    if (cancel)
        cancel();
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

void DockerObservabilityWidget::changeMachine()
{
    ++m_generation;
    cancelFeatureRead();
    m_remoteCommand.cancel();
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
    setStatus(tr("Waiting for the selected machine…"));
    if (m_active && !m_monitorPaused)
        QTimer::singleShot(0, this, &DockerObservabilityWidget::refresh);
}

void DockerObservabilityWidget::refreshRemote(bool allowAuthentication)
{
    if (m_featureMode) {
        refreshFeatureSnapshot();
        return;
    }
    if (!m_active || m_monitorPaused || m_remoteCommand.busy())
        return;
    const QString node = m_machine->currentData().toString();
    const quint64 generation = m_generation;
    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        m_snapshot = {};
        m_selectedResource.clear();
        m_containers->setRowCount(0);
        m_networks->setRowCount(0);
        m_volumes->setRowCount(0);
        updateGraph(m_snapshot);
        updateInspector();
        updateSummary();
        setStatus(error);
        return;
    }
    m_remoteCommand.run(program,
        {"remote", "observe", "--node", node, "--auth",
         allowAuthentication ? "auto" : "never"}, [this, node, generation](bool ok, const QByteArray &output) {
            if (generation != m_generation || !m_active || m_monitorPaused)
                return;
            const auto envelope = QJsonDocument::fromJson(output).object();
            if (!ok || envelope.value("schema").toString() != "dockpipe.machine_snapshot.v1"
                || envelope.value("node").toString() != node || envelope.value("state").toString() != "live"
                || !envelope.value("docker").isObject()) {
                m_snapshot = {};
                m_selectedResource.clear();
                m_containers->setRowCount(0);
                m_networks->setRowCount(0);
                m_volumes->setRowCount(0);
                updateGraph(m_snapshot);
                updateInspector();
                updateSummary();
                setStatus(tr("%1 · Docker monitoring unavailable or stale. On the paired worker, enable: dockpipe remote monitoring --system --docker. The worker and broker must support monitoring.").arg(node));
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
    setStatus(targetNode() + (canControlTarget() ? tr(" · Remote inventory") : tr(" · Remote inventory · read-only"))
        + (converted.topologyNotice.isEmpty() ? QString() : " · " + converted.topologyNotice));
}
