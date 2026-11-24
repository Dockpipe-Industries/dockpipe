#pragma once

#include "WorkflowCatalogData.h"
#include <QWidget>
#include <functional>

class ExecutionExplorer;
class WorkflowEditorWidget;
class QDialog;
class QLabel;
class QLineEdit;
class QListWidget;
class QPushButton;
class QStackedWidget;
class QTabBar;
class QShowEvent;
class QHideEvent;
template <typename T> class QFutureWatcher;

// A locally scoped composition of the existing workflow editor and run explorer.
// Launch configuration and execution remain owned by the hosting shell.
class DashboardWorkflowView : public QWidget {
    Q_OBJECT
public:
    explicit DashboardWorkflowView(QWidget *parent = nullptr);
    bool setWorkdir(const QString &workdir);
    QString workdir() const { return m_workdir; }
    bool openWorkflow(const QString &path);
    bool confirmLeave();
    void confirmLeaveAsync(std::function<void(bool)> completion);
    void setWorkdirAsync(const QString &workdir, std::function<void(bool)> completion = {});
    void openWorkflowAsync(const QString &path, std::function<void(bool)> completion = {});

signals:
    void workflowRunRequested(const QString &workdir, const QString &workflowFile);

protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    void openWorkflowPicker();
    void discoverWorkflows();
    void filterWorkflows();
    void openSelectedWorkflow();
    void updateRunActivity();

    QString m_workdir;
    QString m_catalogRoot;
    WorkflowEditorWidget *m_editor;
    ExecutionExplorer *m_runs;
    QTabBar *m_sections;
    QStackedWidget *m_views;
    QDialog *m_picker;
    QLineEdit *m_search;
    QListWidget *m_workflows;
    QLabel *m_catalogStatus;
    QPushButton *m_open;
    QPushButton *m_refresh;
    QFutureWatcher<WorkflowCatalogData> *m_catalogWatcher;
};
