#pragma once

#include "Context.h"
#include "ContextStore.h"
#include "LauncherSettings.h"
#include "SessionManager.h"
#include "WorkflowCatalog.h"
#include "DashboardStore.h"

#include <QFutureWatcher>
#include <QMainWindow>
#include <QPointer>
#include <QSystemTrayIcon>

class QCloseEvent;
class QListWidget;
class QListWidgetItem;
class QLabel;
class QLineEdit;
class QPlainTextEdit;
class QFrame;
class QStackedWidget;
class QTabWidget;
class QAction;
class QActionGroup;
class BasicModeWidget;
class BrowserWidget;
class TerminalWidget;
class BrowserLinkRouter;
class DashboardPage;
class DashboardWidgetFactory;
class QPushButton;
class DockerObservabilityWidget;
class QTimer;
class QComboBox;
class RemoteWidget;
class ActivityWidget;
class ExecutionExplorer;
class WorkflowEditorWidget;
class ProjectFilesWidget;
class ProjectGitWidget;
class QDialog;
class QSplitter;
class LauncherWorkspaceSurface;
class PackageSurfacePage;

struct AsyncWorkflowCatalogResult {
    QString workdir;
    QString repoRoot;
    WorkflowCatalogData catalog;
};

class MainWindow : public QMainWindow {
#ifdef DOCKPIPE_ENABLE_QUALIFICATION
    friend class PackageDisplayQualification;
#endif
    Q_OBJECT
public:
    explicit MainWindow(QWidget *parent = nullptr);
    ~MainWindow() override;

protected:
    void paintEvent(QPaintEvent *event) override;
    void closeEvent(QCloseEvent *event) override;
    void changeEvent(QEvent *event) override;

private slots:
    void onLaunch();
    void onRelaunch();
    void onStop();
    void onStopAllForRepo();
    void onOpenLogs();
    void onOpenFolder();
    void onTrayActivate(QSystemTrayIcon::ActivationReason reason);
    void onSessionChanged();
    void rebuildUi();

    void onFileOpenProject();
    void onRefreshAppList();
    void onViewBasic();
    void onViewAdvanced();
    void onViewIconGrid();
    void onViewCompactList();
    void onOpenSettings();
    void onManagePackages();
    void onBasicLaunch(const QString &workflowId);
    void onBasicConfigure(const QString &workflowId);
    void cloneWorkflow(const QString &workflowId);
    void onBasicBackHome();
    void onBasicOpenRecent(const QString &absPath);
    void onBasicContinueLast();
    void activateHome();
    void onAbout();
    void onDismissThirdPartyDisclaimer();
    void onRestoreThirdPartyDisclaimer();
    void onAdvancedSearchChanged(const QString &text);
    void onSessionOutput(const QString &contextId, const QString &text);
    void onSessionPrompt(const QString &contextId, const QString &payload);

private:
    void setupMenuBar();
    void selectPage(int page);
    int selectedPage() const;
    void setupDashboards();
    void syncPackageSurfaces(const QVector<ResolverMeta> &providers);
    void removePackageSurface(const QString &id);
    bool launchPackageAwareContext(const Context &context);
    void launchSurfaceWorkflow(const QString &workflowId, const QString &surfaceId,
                               const QString &actionId, const QString &resourceId);
    QMap<QString, QPointer<PackageSurfacePage>> m_surfacePages;
    void applyDefaultDashboardVisibility();
    void applyTilingAvailability();
    void createDashboard();
    void addDashboardNavigation(const DashboardDefinition &definition);
    void ensureDashboard(int page);
    void renameDashboard(const QString &id);
    void removeDashboard(const QString &id);
    QVector<DashboardDefinition> dashboardSnapshots() const;
    bool persistDashboards();
    bool confirmDashboardClose();
    void releaseDashboardWindows();
    void toggleDashboardFullscreen();
    void updateDashboardFullscreen();
    void updateStatusBarVisibility();
    void ensureBrowser();
    void ensureTerminal();
    void ensureCockpit();
    void applyCockpitAvailability();
    bool saveSettings(const LauncherSettings &settings);
    void setupAdvancedPage(QWidget *page);
    void setupWorkflowPicker();
    void openWorkflowPicker();
    void openPickedWorkflow();
    void runPickedWorkflow();
    void updateWorkflowPreview();
    bool launchWorkflow(const Context &display);
    void rememberWorkflowRun(const Context &context);
    void setWorkflowConsoleVisible(bool visible);
    bool setWorkflowWorkspace(const QString &workdir);
    void quitLauncher();
    Context *activeWorkflowContext();
    void applyUiMode();
    void clearContextList();
    void rebuildAdvancedContextList();
    void applyAdvancedContextFilter();
    void updateBasicPage();
    void refreshSessionUi();
    bool hasContext(const Context &c) const;
    Context *findContext(const QString &workdir, const QString &workflow, const QString &workflowFile);
    Context *ensureBasicWorkflowContext(const QString &workflowId);
    WorkflowMeta findWorkflowMeta(const QString &workdir, const QString &workflowId, const QString &workflowFile) const;
    bool configureContextForWorkflow(Context &ctx, const WorkflowMeta &meta, bool forceDialog = false);
    bool openWorkflowConfig(const WorkflowMeta &meta);
    Context *findStoredContextForDisplay(const Context &display);
    Context *ensureStoredContextForDisplay(const Context &display);
    Context *currentAdvancedDisplayContext();
    Context *currentContext();
    QListWidgetItem *currentItem();
    void setupTray();
    void setupDisclaimerBar();
    void applyContextMenu(QListWidgetItem *item, const QPoint &globalPos);
    void refreshInlineConsole();
    void appendInlineConsole(const QString &text);
    QString currentContextCommandLine() const;
    void startAdvancedContextDiscovery();
    void startBasicAppDiscovery();
    void startWorkflowCatalogDiscovery();
    void applyWorkflowCatalogResult(const AsyncWorkflowCatalogResult &result);

    ContextStore m_store;
    SessionManager m_sessions;
    LauncherSettings m_settings;

    QListWidget *m_navigation = nullptr;
    QFrame *m_navigationRail = nullptr;
    QComboBox *m_runTarget = nullptr;
    RemoteWidget *m_remoteWidget = nullptr;
    ActivityWidget *m_activityWidget = nullptr;
    ExecutionExplorer *m_executionExplorer = nullptr;
    QTabWidget *m_executionTabs = nullptr;
    QStackedWidget *m_stack = nullptr;
    BasicModeWidget *m_basicWidget = nullptr;
    BrowserWidget *m_browserWidget = nullptr;
    TerminalWidget *m_terminalWidget = nullptr;
    BrowserLinkRouter *m_browserLinks = nullptr;
    DashboardWidgetFactory *m_dashboardFactory = nullptr;
    DashboardStore m_dashboardStore;
    QVector<DashboardDefinition> m_dashboards;
    QMap<QString, QPointer<DashboardPage>> m_dashboardPages;
    QPushButton *m_addDashboard = nullptr;
    QAction *m_addDashboardAction = nullptr;
    QActionGroup *m_pageGroup = nullptr;
    bool m_dashboardStoreReady = false;
    bool m_dashboardDirty = false;
    bool m_fullscreenWasMaximized = false;
    QAction *m_fullscreenAction = nullptr;
    QWidget *m_advancedPage = nullptr;
    QPushButton *m_workspaceButton = nullptr;
    QLabel *m_workspacePath = nullptr;
    DockerObservabilityWidget *m_dockerWidget = nullptr;

    WorkflowEditorWidget *m_workflowEditor = nullptr;
    ProjectFilesWidget *m_projectFiles = nullptr;
    ProjectGitWidget *m_projectGit = nullptr;
    LauncherWorkspaceSurface *m_cockpit = nullptr;
    QDialog *m_workflowPicker = nullptr;
    QWidget *m_consolePanel = nullptr;
    QPushButton *m_consoleToggle = nullptr;
    QWidget *m_workspaceBar = nullptr;
    Context m_canvasContext;
    Context m_outputContext;
    QWidget *m_workflowActivityPanel = nullptr;
    ActivityWidget *m_workflowActivity = nullptr;
    QLabel *m_workflowPreview = nullptr;
    QPushButton *m_pickerRun = nullptr;
    QPushButton *m_pickerEdit = nullptr;
    QPushButton *m_pickerClone = nullptr;
    bool m_workflowCloneBusy = false;
    QComboBox *m_pickerTarget = nullptr;
    QStringList m_recentWorkflowKeys;
    QVector<WorkflowMeta> m_workspaceWorkflows;
    QListWidget *m_list = nullptr;
    QLabel *m_hint = nullptr;
    QLineEdit *m_search = nullptr;
    QTimer *m_advancedSearchTimer = nullptr;
    QTimer *m_basicLaunchingTimer = nullptr;
    QLabel *m_consoleTitle = nullptr;
    QLabel *m_consoleMeta = nullptr;
    QPlainTextEdit *m_console = nullptr;
    QFrame *m_emptyState = nullptr;
    QLabel *m_emptyTitle = nullptr;
    QLabel *m_emptyBody = nullptr;
    QSystemTrayIcon *m_tray = nullptr;

    QVector<QAction *> m_pageActions;
    QAction *m_actSidebar = nullptr;
    QAction *m_actIcons = nullptr;
    QAction *m_actList = nullptr;

    QWidget *m_disclaimerContainer = nullptr;
    QString m_consoleContextId;
    QVector<WorkflowMeta> m_basicApps;
    QVector<Context> m_advancedSourceContexts;
    QVector<Context> m_advancedContexts;
    QString m_basicLaunchingContextId;
    QString m_basicLaunchingWorkflowId;
    QFutureWatcher<AsyncWorkflowCatalogResult> *m_workflowCatalogWatcher = nullptr;
    QString m_advancedDiscoveryRequestedWorkdir;
    QString m_advancedDiscoveryRunningWorkdir;
    QString m_basicAppsRequestedWorkdir;
    QString m_basicAppsRunningWorkdir;
    QString m_workflowCatalogRequestedWorkdir;
    QString m_workflowCatalogRunningWorkdir;
    bool m_advancedDiscoveryLoading = false;
    bool m_basicAppsLoading = false;
    bool m_workflowCatalogRefreshPending = false;
};
