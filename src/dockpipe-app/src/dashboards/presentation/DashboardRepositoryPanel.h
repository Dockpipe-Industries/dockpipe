#pragma once

#include <QJsonObject>
#include <QWidget>
#include <functional>

class DashboardWorkflowView;
class ProjectFilesWidget;
class ProjectGitWidget;
class QLabel;

// A full tool view owns its repository independently of the shell workspace.
class DashboardRepositoryPanel final : public QWidget {
    Q_OBJECT
public:
    DashboardRepositoryPanel(const QString &kind, const QString &repository, QWidget *parent = nullptr);
    DashboardRepositoryPanel(const QString &kind, const QString &repository, const QJsonObject &catalog,
                             const QString &rootId, QWidget *parent = nullptr);
    bool canChangeMachineContext(QString *error = nullptr) const;
    bool setMachineContext(const QJsonObject &catalog, const QString &rootId, QString *error = nullptr);
    QString repository() const { return m_repository; }
    bool setRepository(const QString &repository);
    QJsonObject context() const;
    bool restoreContext(const QJsonObject &context);
    bool confirmLeave();
    void confirmLeaveAsync(std::function<void(bool)> completion);
    void setRepositoryAsync(const QString &repository, std::function<void(bool)> completion = {});
    bool openWorkflow(const QString &path);

signals:
    void contextChanged();
    void duplicateRequested();
    void workflowRunRequested(const QString &repository, const QString &path);
    void workflowOpenRequested(const QString &repository, const QString &path);

private:
    bool confirmCommitDraft();
    QString m_kind;
    QString m_repository;
    QLabel *m_location;
    QLabel *m_error;
    DashboardWorkflowView *m_workflows = nullptr;
    ProjectFilesWidget *m_files = nullptr;
    ProjectGitWidget *m_git = nullptr;
};
