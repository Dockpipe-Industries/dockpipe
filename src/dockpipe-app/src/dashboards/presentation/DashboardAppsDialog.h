#pragma once

#include "DesktopApplicationCatalog.h"
#include "WorkflowCatalogData.h"
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
        bool allowDesktopApps = true, const QVector<PackageDisplayMeta> &displays = {});
    void setPackageDisplays(const QVector<PackageDisplayMeta> &displays);
signals:
    void packageDisplayRequested(const QString &id);
    void workflowRequested(const QString &id);
    void cloneRequested(const QString &id);
    void desktopApplicationRequested(const DesktopApplicationEntry &application);
private:
    void rebuildList();
    void launchSelected();
    QVector<WorkflowMeta> m_workflows;
    QVector<PackageDisplayMeta> m_displays;
    QVector<DesktopApplicationEntry> m_applications;
    QHash<int, QIcon> m_applicationIcons;
    QLineEdit *m_search;
    QListWidget *m_list;
    QLabel *m_status;
    QPushButton *m_launch;
    QPushButton *m_clone;
    bool m_launching = false;
};
