#pragma once

#include "WorkflowCatalog.h"
#include <QJsonObject>
#include <QMap>
#include <QProcess>
#include <QWidget>

struct Context;
class SessionManager;
class PackageDisplayWidget;
class QComboBox;
class QLabel;
class QPlainTextEdit;
class QPushButton;
class QTableWidget;
class QTimer;
class QMenu;
class QAction;

// Generic installed-package resource page. All observation and actions execute
// declared workflows through the same session/prompt/cancellation boundary.
class PackageSurfacePage final : public QWidget {
    Q_OBJECT
public:
    explicit PackageSurfacePage(SessionManager &sessions, QWidget *parent = nullptr);
    ~PackageSurfacePage() override;
    void setWorkspace(const QString &workdir, const QString &binary);
    void setProviders(const QVector<ResolverMeta> &providers);
    void setActive(bool active);
    void refresh();
    void refreshInventory();
    QJsonObject inventoryDiagnostic() const;
    bool supportsWorkflow(const QString &workflowId) const;
    bool prepareLaunchContext(Context &invocation, QString &error) const;
    void expectLaunch(const QString &workflowId, const QString &launchId);
    void cancelLaunch(const QString &launchId);
    bool ownsSession(const QString &id) const { return id == m_snapshotId || id == m_actionId; }
    bool applyEvent(const QJsonObject &event, QString &error);
    static constexpr int maximumResources = 256;
    static constexpr int maximumEventBytes = 1024 * 1024;

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

signals:
    void launchResolved(const QString &launchId);
    void displaysChanged(const QVector<PackageDisplayMeta> &displays);
    void publicWorkflowRequested(const QString &workflowId, const QString &surfaceId,
                                 const QString &actionId, const QString &resourceId);

private:
    struct Resource {
        QString id;
        QString title;
        QString state;
        QMap<QString, QString> details;
        QStringList actions;
        qint64 displayProcessId = 0;
        QString displayStartToken;
        QString launchId;
        bool packageDisplay = false;
        QJsonObject displayOptions;
    };
    void selectProvider();
    void recordInventoryTransition(const QString &transition);
    void drainInventoryRequest();
    void updateActions();
    void showResource();
    void runAction(const PackageSurfaceActionMeta &action);
    void runWorkflow(const QString &file, const QString &action, const QString &resource, bool snapshot);
    void receiveOutput(const QString &contextId, const QString &text);
    void finishOperation(const QString &contextId, bool success);
    void failOperation(const QString &contextId, const QString &message);
    void cancelOperations();
    QString selectedResource() const;
    const ResolverMeta *provider() const;
    SessionManager &m_sessions;
    QVector<ResolverMeta> m_providers;
    QVector<Resource> m_resources;
    QString m_workdir;
    QString m_binary;
    QString m_snapshotId;
    QString m_actionId;
    QComboBox *m_providerSelector;
    QLabel *m_status;
    QTableWidget *m_table;
    QPlainTextEdit *m_details;
    QPlainTextEdit *m_output;
    QMenu *m_actionsMenu;
    QComboBox *m_resourceSelector;
    QPushButton *m_cancel;
    PackageDisplayWidget *m_display;
    QTimer *m_poll;
    QTimer *m_snapshotDeadline;
    QTimer *m_actionDeadline;
    QTimer *m_launchDeadline;
    QString m_expectedLaunchId;
    bool m_selectInitialResource = false;
    QMap<QString, QByteArray> m_pending;
    QMap<QString, int> m_received;
    QMap<QString, QString> m_failures;
    QMap<QAction *, PackageSurfaceActionMeta> m_actions;
    QStringList m_inventoryTransitions;
    bool m_inventoryRefreshPending = false;
    bool m_active = false;
    bool m_snapshotBusy = false;
    bool m_actionBusy = false;
    bool m_snapshotReceived = false;
    bool m_stateFresh = false;
};
