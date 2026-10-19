#pragma once

#include "DesktopApplicationCatalog.h"
#include "WorkflowCatalog.h"
#include <QDialog>
#include <QHash>

class QLabel;
class QLineEdit;
class QListWidget;
class QPushButton;

class DashboardAppsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit DashboardAppsDialog(const QVector<WorkflowMeta> &workflows, QWidget *parent = nullptr,
        bool allowDesktopApps = true);
signals:
    void workflowRequested(const QString &id);
    void cloneRequested(const QString &id);
    void desktopApplicationRequested(const DesktopApplicationEntry &application);
private:
    void rebuildList();
    void launchSelected();
    QVector<WorkflowMeta> m_workflows;
    QVector<DesktopApplicationEntry> m_applications;
    QHash<int, QIcon> m_applicationIcons;
    QLineEdit *m_search;
    QListWidget *m_list;
    QLabel *m_status;
    QPushButton *m_launch;
    QPushButton *m_clone;
    bool m_launching = false;
};
