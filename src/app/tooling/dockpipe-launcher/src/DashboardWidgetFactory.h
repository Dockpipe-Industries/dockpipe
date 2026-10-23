#pragma once

#include "RemoteCommand.h"
#include "ProjectGitClient.h"
#include "WorkflowCatalog.h"

#include <QJsonArray>
#include <QJsonObject>
#include <QObject>
#include <QPointer>
#include <QVector>

class ContextStore;
class SessionManager;
class TerminalHub;
class QTimer;
class QWidget;

struct DashboardWidgetDescriptor {
    QString id;
    QString title;
    QString description;
};

// Compact views reuse launcher services. This factory does not execute workflows.
class DashboardWidgetFactory final : public QObject {
    Q_OBJECT
public:
    DashboardWidgetFactory(SessionManager &sessions, ContextStore &store, QObject *parent = nullptr);
    static QVector<DashboardWidgetDescriptor> catalog();
    QWidget *create(const QString &kind, QWidget *parent = nullptr, const QString &fileScope = "project");
    void setWorkdir(const QString &workdir);
    QString workdir() const { return m_workdir; }
    void setWorkflows(const QVector<WorkflowMeta> &workflows);
    QVector<WorkflowMeta> workflows() const { return m_workflows; }
    void refresh();
    void setPackageDisplays(const QString &source, const QVector<PackageDisplayMeta> &displays);
    QVector<PackageDisplayMeta> packageDisplays() const;
    void requestPackageDisplayInventory() { emit packageDisplayInventoryRequested(); }
    TerminalHub &terminals() const { return *m_terminals; }

signals:
    void packageDisplaysChanged();
    void packageDisplayInventoryRequested();
    void openFullView(const QString &pageKey);
    void launchWorkflow(const QString &workflowId);
    void cloneWorkflow(const QString &workflowId);
    void openFile(const QString &path);
    void workdirChanged(const QString &workdir);
    void workflowsChanged();
    void snapshotChanged(const QJsonObject &snapshot, const QString &error);
    void containersChanged(const QJsonArray &containers, const QString &error);
    void gitChanged(const QJsonObject &snapshot, const QString &error);
    void machinesChanged(const QJsonArray &nodes, const QString &error);

protected:
    bool eventFilter(QObject *watched, QEvent *event) override;

private:
    bool hasVisibleKind(const QStringList &kinds) const;
    void refreshSnapshot();
    void refreshContainers();
    void refreshGit();
    void refreshMachines();
    QWidget *createSystem(QWidget *parent);
    QWidget *createExecutions(QWidget *parent);
    QWidget *createWorkflows(QWidget *parent);
    QWidget *createContainers(QWidget *parent);
    QWidget *createBrowser(QWidget *parent);
    QWidget *createFiles(QWidget *parent, const QString &scope);
    QWidget *createGit(QWidget *parent);
    QWidget *createMachines(QWidget *parent);
    QWidget *wrap(const QString &kind, const QString &pageKey, QWidget *content, QWidget *parent);

    SessionManager &m_sessions;
    ContextStore &m_store;
    TerminalHub *m_terminals;
    RemoteCommand m_snapshotCommand;
    RemoteCommand m_containerCommand;
    RemoteCommand m_machineCommand;
    ProjectGitClient m_gitCommand;
    QTimer *m_timer;
    QVector<QPointer<QWidget>> m_widgets;
    QVector<WorkflowMeta> m_workflows;
    QMap<QString, QVector<PackageDisplayMeta>> m_packageDisplays;
    QString m_workdir;
    quint64 m_generation = 0;
};
