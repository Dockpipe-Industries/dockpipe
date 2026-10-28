#pragma once

#include <QFutureWatcher>
#include <QWidget>
#include <QJsonArray>
#include "ResourceCanvas.h"
#include "RemoteCommand.h"

class QLabel;
class DockerResourceInspector;
class QDialog;
class QLineEdit;
class QPlainTextEdit;
class QSplitter;
class QTabWidget;
class QTableWidget;
class QTimer;
class QWidget;
class MonitoringMachineSelector;

class DockerObservabilityWidget : public QWidget {
    Q_OBJECT
public:
    explicit DockerObservabilityWidget(QWidget *parent = nullptr);
    void setNodes(const QStringList &nodes);

public slots:
    void refresh();
    void setActive(bool active);

protected:
    void showEvent(QShowEvent *event) override;

private slots:
    void onContainersSelectionChanged();
    void onNetworksSelectionChanged();
    void onVolumesSelectionChanged();
    void onContainerSearchChanged(const QString &text);
    void onContainersContextMenuRequested(const QPoint &pos);
    void onStartSelectedContainer();
    void onStopSelectedContainer();

private:
    void changeMachine();
    void refreshRemote(bool allowAuthentication = false);
    void applyRemoteSnapshot(const QJsonObject &snapshot);
    MonitoringMachineSelector *m_machine = nullptr;
    RemoteCommand m_remoteCommand;
    quint64 m_generation = 0;
    quint64 m_localRefreshGeneration = 0;
    struct ContainerRowData {
        QString id;
        QString name;
        QString state;
        QString statusText;
        QString image;
        QString ports;
        QString created;
    };

    struct NetworkRowData {
        QString id;
        QString name;
        QString driver;
        QString scope;
    };

    struct VolumeRowData {
        QString name;
        QString driver;
        QString mountpoint;
    };

    struct CommandResult {
        bool ok = false;
        QString stdoutText;
        QString stderrText;
    };

    struct DockerSnapshot {
        QJsonArray topology;
        QString topologyNotice;
        bool containersOk = false;
        QString containersError;
        QVector<ContainerRowData> containers;

        bool networksOk = false;
        QString networksError;
        QVector<NetworkRowData> networks;

        bool volumesOk = false;
        QString volumesError;
        QVector<VolumeRowData> volumes;
    };

    struct ContainerDetailSnapshot {
        QString containerId;
        QString inspectText;
        QString logsText;
    };

    struct ObjectDetailSnapshot {
        QString objectId;
        QString detailText;
    };

    void buildUi();
    void selectResource(const QString &key);
    void updateInspector();
    void openExec(const QString &shell);
    void runContainerAction(const QString &action);
    DockerResourceInspector *m_inspector = nullptr;
    QDialog *m_inventory = nullptr;
    QString m_selectedResource;
    DockerSnapshot m_snapshot;
    bool m_monitorPaused = false;
    bool m_containerActionBusy = false;
    void updateGraph(const DockerSnapshot &snapshot);
    ResourceCanvas *m_canvas = nullptr;
    QWidget *buildContainersPage();
    QWidget *buildNetworksPage();
    QWidget *buildVolumesPage();

    static CommandResult runDocker(const QStringList &args, bool mergeOutput = false);
    static DockerSnapshot collectDockerSnapshot();
    void setStatus(const QString &text);
    void refreshQuietly();
    void updateSummary();
    void applyContainerFilter();
    void updateContainerActionState();
    bool hasAnyObjects() const;
    void setLoadingState(bool loading, const QString &text = QString());
    QString selectedContainerId() const;
    QString selectedContainerName() const;
    QString selectedContainerState() const;
    QString selectedNetworkId() const;
    QString selectedVolumeName() const;

    void applySnapshot(const DockerSnapshot &snapshot);
    void refreshContainerDetailsAsync(const QString &containerId);
    void refreshNetworkDetailsAsync(const QString &networkId);
    void refreshVolumeDetailsAsync(const QString &volumeName);

    void renderContainerDetails(const QString &containerId);
    void renderNetworkDetails(const QString &networkId);
    void renderVolumeDetails(const QString &volumeName);

    QLabel *m_status = nullptr;
    QLabel *m_containerCount = nullptr;
    QLabel *m_networkCount = nullptr;
    QLabel *m_volumeCount = nullptr;
    QLineEdit *m_search = nullptr;
    QTabWidget *m_tabs = nullptr;

    QTableWidget *m_containers = nullptr;
    QPlainTextEdit *m_containerDetails = nullptr;
    QPlainTextEdit *m_containerLogs = nullptr;

    QTableWidget *m_networks = nullptr;
    QPlainTextEdit *m_networkDetails = nullptr;

    QTableWidget *m_volumes = nullptr;
    QPlainTextEdit *m_volumeDetails = nullptr;
    QFutureWatcher<DockerSnapshot> *m_refreshWatcher = nullptr;
    QFutureWatcher<ContainerDetailSnapshot> *m_containerDetailWatcher = nullptr;
    QFutureWatcher<ObjectDetailSnapshot> *m_networkDetailWatcher = nullptr;
    QFutureWatcher<ObjectDetailSnapshot> *m_volumeDetailWatcher = nullptr;
    QTimer *m_autoRefreshTimer = nullptr;
    bool m_hasLoadedOnce = false;
    bool m_refreshPending = false;
    bool m_refreshPendingQuiet = false;
    bool m_currentRefreshQuiet = false;
    bool m_loading = false;
    bool m_active = false;
    QString m_pendingContainerDetailId;
    QString m_displayedContainerDetailId;
    QString m_pendingNetworkDetailId;
    QString m_pendingVolumeDetailId;
};
