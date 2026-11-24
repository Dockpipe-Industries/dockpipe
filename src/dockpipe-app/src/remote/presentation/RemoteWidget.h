#pragma once

#include "RemoteCommand.h"
#include "ResourceCanvas.h"
#include "WorkflowCatalogData.h"
#include <QWidget>
#include <QPointer>
#include <QJsonArray>

class ResourceCanvas;
class HardwareKeyDialog;
class QJsonObject;
class QLabel;
class QComboBox;
class QLineEdit;
class QCheckBox;
class QPlainTextEdit;
class QPushButton;
class QTableWidget;
class QStackedWidget;
class QTimer;
class QShowEvent;
class QDialog;
class QVBoxLayout;

class RemoteWidget : public QWidget {
    Q_OBJECT
public:
    explicit RemoteWidget(QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void refresh(bool allowAuthentication = false);
    void setResolvers(const QVector<ResolverMeta> &resolvers);
    void selectResolver(const QString &name);
    void openMachineSettings(const QString &node);
    void beginEnvironmentPairing();
    void beginEnvironmentConnectionSetup();
protected:
    void showEvent(QShowEvent *event) override;
signals:
    void nodesChanged(const QStringList &nodes);
    void pairedMachinesChanged(const QJsonArray &machines);
    void machineInventoryChanged(const QVector<ResourceNode> &nodes, const QVector<ResourceEdge> &edges);
    void environmentFlowFinished();
    void installProviderRequested();
private:
    QWidget *buildOverview();
    QWidget *buildPairing();
    QWidget *buildConnection();
    void setPairingStep(int step);
    void updateManagementPresentation();
    void beginEnvironmentFlow(int page);
    void leaveManagementPage();
    void finishEnvironmentFlow();
    QPushButton *m_pairingBack = nullptr;
    QPushButton *m_setupBack = nullptr;
    QPushButton *m_connectionBack = nullptr;
    QPushButton *m_connectionDone = nullptr;
    QLabel *m_pairingTitle = nullptr;
    QStackedWidget *m_pairingSteps = nullptr;
    QLabel *m_pairingProgress = nullptr;
    QLabel *m_pairingSuccess = nullptr;
    QLabel *m_pairingWaiting = nullptr;
    QLabel *m_pairingKeyError = nullptr;
    QPushButton *m_startPairing = nullptr;
    QPushButton *m_reviewPairing = nullptr;
    QString m_pairingEndpoint;
    QWidget *buildSetup();
    void showPage(int index);
    void updateActions();
    void refreshPairings(bool allowAuthentication = false);
    void refreshNodes(bool allowAuthentication = false);
    void refreshSecurityStatus();
    void startWorkerService(bool pairingCompleted = false, bool terminalSetupFailed = false);
    void enableHomeTerminal(bool pairingCompleted);
    void replaceService(const QString &role);
    void updateServiceNotices(const QJsonObject &info);
    void updateSetup();
    void run(const QStringList &arguments, RemoteCommand::Completion completion = {}, int timeoutMs = 150000,
             bool allowAuthentication = true);
    void decidePairing(bool approve);
    ResourceCanvas *m_canvas = nullptr;
    QString m_graphRemoteEndpoint;
    void updateMachineGraph();
    void setListView(bool list);
    void updateMachineList();
    void removeMachine(const QString &node);
    void updateMachineSettings();
    QWidget *m_machineSettingsPanel = nullptr;
    QDialog *m_machineSettingsDialog = nullptr;
    QDialog *m_managementDialog = nullptr;
    bool m_environmentFlow = false;
    bool m_interactiveCommand = false;
    QPushButton *m_canvasAdd = nullptr;
    QPushButton *m_localSettings = nullptr;
    QLabel *m_machineSettingsError = nullptr;
    QString m_settingsNode;
    QWidget *m_management = nullptr;
    QWidget *m_canvasManagementHost = nullptr;
    QWidget *m_listManagementHost = nullptr;
    QStackedWidget *m_views = nullptr;
    QPushButton *m_canvasView = nullptr;
    QPushButton *m_listView = nullptr;
    QLineEdit *m_machineSearch = nullptr;
    QComboBox *m_accessFilter = nullptr;
    QPushButton *m_connectionSettings = nullptr;
    QWidget *m_connectionCard = nullptr;
    QVBoxLayout *m_overviewLayout = nullptr;
    QVBoxLayout *m_setupConnectionLayout = nullptr;
    QPushButton *m_openProviderSetup = nullptr;
    QLabel *m_listResults = nullptr;
    QLabel *m_selectedMachine = nullptr;
    bool m_machineInventoryLoaded = false;
    bool m_brokerServiceNeedsUpdate = false;
    QPushButton *m_detailsToggle = nullptr;
    QString m_workdir;
    QVector<ResolverMeta> m_resolvers;
    QComboBox *m_resolver = nullptr;
    QLineEdit *m_hostname = nullptr;
    QLabel *m_hostnameLabel = nullptr;
    QLabel *m_setupHint = nullptr;
    QLabel *m_existingConnection = nullptr;
    QPushButton *m_installProvider = nullptr;
    QLabel *m_connectionSummary = nullptr;
    QLabel *m_brokerServiceHint = nullptr;
    QLabel *m_workerServiceHint = nullptr;
    QPushButton *m_updateBrokerService = nullptr;
    QPushButton *m_updateWorkerService = nullptr;
    QLabel *m_pairingAddress = nullptr;
    QPushButton *m_setup = nullptr;
    RemoteCommand m_command;
    RemoteCommand m_securityStatusCommand;
    QPointer<HardwareKeyDialog> m_hardwareKeyDialog;
    QLabel *m_status = nullptr;
    QLabel *m_pairingCode = nullptr;
    QLabel *m_overviewHint = nullptr;
    QLabel *m_securityStatus = nullptr;
    QLabel *m_pairingHint = nullptr;
    QLabel *m_windowHint = nullptr;
    QString m_pendingOutput;
    QString m_requestOutcome;
    QTableWidget *m_nodes = nullptr;
    QTableWidget *m_pairings = nullptr;
    QLineEdit *m_endpoint = nullptr;
    QLabel *m_endpointError = nullptr;
    QLineEdit *m_name = nullptr;
    QCheckBox *m_allow = nullptr;
    QCheckBox *m_allowTerminal = nullptr;
    QPlainTextEdit *m_output = nullptr;
    QStackedWidget *m_pages = nullptr;
    QTimer *m_pairingTimer = nullptr;
    QTimer *m_securityStatusTimer = nullptr;
    QPushButton *m_approve = nullptr;
    QPushButton *m_deny = nullptr;
    QPushButton *m_revoke = nullptr;
    QPushButton *m_removeMachine = nullptr;
    QPushButton *m_request = nullptr;
    QPushButton *m_review = nullptr;
    QPushButton *m_cancel = nullptr;
    QList<QPushButton *> m_actions;
    bool m_brokerAvailable = false;
    QString m_preferredResolver;
    QString m_savedResolver;
};
