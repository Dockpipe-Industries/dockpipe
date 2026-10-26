#pragma once

#include "RemoteCommand.h"
#include "WorkflowCatalogData.h"
#include <QWidget>

class ResourceCanvas;
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

class RemoteWidget : public QWidget {
    Q_OBJECT
public:
    explicit RemoteWidget(QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void refresh();
    void setResolvers(const QVector<ResolverMeta> &resolvers);
    void selectResolver(const QString &name);
signals:
    void nodesChanged(const QStringList &nodes);
    void installProviderRequested();
private:
    QWidget *buildOverview();
    QWidget *buildPairing();
    QWidget *buildConnection();
    QWidget *buildSetup();
    void showPage(int index);
    void updateActions();
    void refreshPairings();
    void refreshNodes();
    void startWorkerService();
    void replaceService(const QString &role);
    void updateServiceNotices(const QJsonObject &info);
    void updateSetup();
    void run(const QStringList &arguments, RemoteCommand::Completion completion = {}, int timeoutMs = 30000);
    void decidePairing(bool approve);
    ResourceCanvas *m_canvas = nullptr;
    QString m_graphRemoteEndpoint;
    void updateMachineGraph();
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
    QLabel *m_status = nullptr;
    QLabel *m_pairingCode = nullptr;
    QLabel *m_overviewHint = nullptr;
    QLabel *m_pairingHint = nullptr;
    QLabel *m_windowHint = nullptr;
    QString m_pendingOutput;
    QString m_requestOutcome;
    QTableWidget *m_nodes = nullptr;
    QTableWidget *m_pairings = nullptr;
    QLineEdit *m_endpoint = nullptr;
    QLineEdit *m_name = nullptr;
    QCheckBox *m_allow = nullptr;
    QPlainTextEdit *m_output = nullptr;
    QStackedWidget *m_pages = nullptr;
    QTimer *m_pairingTimer = nullptr;
    QPushButton *m_approve = nullptr;
    QPushButton *m_deny = nullptr;
    QPushButton *m_revoke = nullptr;
    QPushButton *m_request = nullptr;
    QPushButton *m_review = nullptr;
    QPushButton *m_cancel = nullptr;
    QList<QPushButton *> m_actions;
    bool m_brokerAvailable = false;
    QString m_preferredResolver;
    QString m_savedResolver;
};
