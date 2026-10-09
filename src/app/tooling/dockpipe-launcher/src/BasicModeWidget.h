#pragma once

#include "WorkflowCatalog.h"

#include <QHash>
#include <QWidget>

class QLabel;
class QListWidget;
class QPushButton;
class QStackedWidget;
class QTimer;
class QFrame;

/// IDE-style flow: home (recent projects) → workspace (apps for the selected folder).
class BasicModeWidget : public QWidget {
    Q_OBJECT
public:
    explicit BasicModeWidget(QWidget *parent = nullptr);

    void showHomePage();
    void showWorkspacePage();

    void setRecentProjects(const QStringList &paths);
    void setContinueLastVisible(bool visible);

    void setApps(const QVector<WorkflowMeta> &apps);
    void setAppDiscoveryLoading(bool loading);
    void setRunningByWorkflow(const QHash<QString, bool> &running);
    void setViewIconMode(bool icons);
    void setLaunchingWorkflow(const QString &workflowId, const QString &displayName);
    void clearLaunchingWorkflow();

signals:
    void openProjectRequested();
    void refreshAppsRequested();
    void packagesRequested();
    void launchRequested(const QString &workflowId);
    void configureRequested(const QString &workflowId);

    void backToHomeRequested();
    void recentProjectSelected(const QString &absPath);
    void continueLastRequested();
private slots:
    void onRefresh();

private:
    void applyViewMode();
    void rebuildItemTexts();
    void rebuildRecentList();
    void updateResponsiveMetrics();
    void updateEmptyState();
    void updateLoadingBanner();
    void updateLaunchOverlayGeometry();

protected:
    void resizeEvent(QResizeEvent *event) override;

    QStackedWidget *m_stack = nullptr;
    QWidget *m_homePage = nullptr;
    QWidget *m_workspacePage = nullptr;

    QListWidget *m_recentList = nullptr;
    QLabel *m_homeEmptyHint = nullptr;
    QPushButton *m_openProjectHome = nullptr;
    QPushButton *m_continueLast = nullptr;

    QWidget *m_emptyApps = nullptr;
    QLabel *m_emptyAppsTitle = nullptr;
    QLabel *m_emptyAppsBody = nullptr;
    QPushButton *m_emptyPackages = nullptr;
    QLabel *m_loadingBanner = nullptr;
    QWidget *m_appsPage = nullptr;
    QWidget *m_launchOverlay = nullptr;
    QFrame *m_launchOverlayCard = nullptr;
    QLabel *m_launchOverlayGlyph = nullptr;
    QLabel *m_launchOverlayTitle = nullptr;
    QLabel *m_launchOverlayBody = nullptr;
    QPushButton *m_refresh = nullptr;
    QListWidget *m_list = nullptr;
    QTimer *m_loadingTimer = nullptr;

    QStringList m_recentPaths;
    QVector<WorkflowMeta> m_apps;
    QHash<QString, bool> m_running;
    bool m_iconMode = true;
    QString m_launchingWorkflowId;
    QString m_launchingWorkflowName;
    int m_loadingFrame = 0;
    bool m_appDiscoveryLoading = false;
};
