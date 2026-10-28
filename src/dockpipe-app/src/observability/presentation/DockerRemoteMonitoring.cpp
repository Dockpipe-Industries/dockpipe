#include "DockerObservabilityWidget.h"
#include "DockpipeChoices.h"
#include "MonitoringMachineSelector.h"
#include <QJsonDocument>
#include <QJsonObject>
#include <QPlainTextEdit>
#include <QTableWidget>
#include <QTimer>

void DockerObservabilityWidget::setNodes(const QStringList &nodes)
{
    m_machine->setNodes(nodes);
}

void DockerObservabilityWidget::changeMachine()
{
    ++m_generation;
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
            setStatus(tr("Remote Docker inventory exceeds the supported bound."));
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
    setStatus(m_machine->currentData().toString() + tr(" · Remote inventory · read-only")
        + (converted.topologyNotice.isEmpty() ? QString() : " · " + converted.topologyNotice));
}
