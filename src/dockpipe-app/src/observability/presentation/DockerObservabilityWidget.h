#pragma once

#include <QFutureWatcher>
#include <atomic>
#include <memory>
#include <QWidget>
#include <QJsonArray>
#include "ResourceCanvas.h"
#include "RemoteCommand.h"
#include "RemoteFeatureClient.h"

class QLabel;
class QComboBox;
class QResizeEvent;
class QStackedWidget;
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
    ~DockerObservabilityWidget() override;
    void setNodes(const QStringList &nodes);
    bool controlPending() const { return m_containerActionBusy; }
    bool setFeatureTarget(const QJsonObject &target, bool canControl, QString *error = nullptr);
    bool setFeatureCatalog(const QJsonObject &catalog, QString *error = nullptr);

signals:
    void remoteExecRequested(const QJsonObject &catalog, const QString &container, const QString &shell);

public slots:
    void refresh();
    void setActive(bool active);

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;

private slots:
    void onContainersSelectionChanged();
    void onNetworksSelectionChanged();
    void onVolumesSelectionChanged();
    void onContainerSearchChanged(const QString &text);
    void onContainersContextMenuRequested(const QPoint &pos);
    void onStartSelectedContainer();
    void onStopSelectedContainer();

private:
    friend class DockerRemoteFeatureTest;
    bool isRemoteTarget() const;
    QString targetNode() const;
    bool canControlTarget() const;
    void cancelFeatureRead();
    void refreshFeatureSnapshot();
    void refreshFeatureDetails(const QString &kind, const QString &object);
    void cancelFeatureDetails();
    void runRemoteContainerAction(const QString &action, const QString &id, const QString &name);
    void changeMachine();
    void refreshRemote(bool allowAuthentication = false);
    void applyRemoteSnapshot(const QJsonObject &snapshot);
    MonitoringMachineSelector *m_machine = nullptr;
    QLabel *m_machineLabel = nullptr;
    bool m_featureMode = false;
    bool m_featureCanControl = false;
    QJsonObject m_featureTarget;
    QJsonObject m_featureCatalog;
    RemoteFeatureClient::Cancel m_cancelFeatureRead;
    RemoteFeatureClient::Cancel m_cancelFeatureDetail;
    quint64 m_featureDetailGeneration = 0;
    QString m_featureDetailKey;
    std::shared_ptr<std::atomic_bool> m_localReadCancelled = std::make_shared<std::atomic_bool>(false);
    bool m_featureReadBusy = false;
    quint64 m_featureReadGeneration = 0;
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
    QWidget *buildListDashboard();
    void updateDashboardLayout();
    void updateListResults();
    void setDashboardView(bool dashboard);
    void selectResource(const QString &key);
    void updateInspector();
    void openExec(const QString &shell);
    void runContainerAction(const QString &action);
    DockerResourceInspector *m_inspector = nullptr;
    QDialog *m_inventory = nullptr;
    QStackedWidget *m_views = nullptr;
    QWidget *m_dashboard = nullptr;
    QWidget *m_dashboardScroll = nullptr;
    QSplitter *m_dashboardSplitter = nullptr;
    QWidget *m_canvasInspectorHost = nullptr;
    QWidget *m_listMonitoringHost = nullptr;
    QWidget *m_listInventoryHost = nullptr;
    bool m_dashboardView = false;
    QString m_selectedResource;
    DockerSnapshot m_snapshot;
    bool m_monitorPaused = false;
    bool m_containerActionBusy = false;
    void updateGraph(const DockerSnapshot &snapshot);
    ResourceCanvas *m_canvas = nullptr;
    QWidget *buildContainersPage();
    QWidget *buildNetworksPage();
    QWidget *buildVolumesPage();

    static CommandResult runContainerRequest(const QString &binary, const QJsonObject &request,
        const std::shared_ptr<std::atomic_bool> &cancelled = {});
    static DockerSnapshot collectDockerSnapshot(const QString &binary,
        const std::shared_ptr<std::atomic_bool> &cancelled);
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
    QLabel *m_runningCount = nullptr;
    QLabel *m_listResults = nullptr;
    QLabel *m_listEmptyState = nullptr;
    QComboBox *m_stateFilter = nullptr;
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
