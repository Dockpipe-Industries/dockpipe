#pragma once

#include "DashboardStore.h"
#include <QWidget>
#include <QJsonArray>
#include <QSet>
#include <functional>

class BrowserWidget;
class QWebEngineView;
class DashboardWidgetFactory;
class DashboardWorkspace;
class DashboardDock;
class QFrame;
class QLabel;
class QPushButton;
class QTimer;
class QStackedWidget;
struct DesktopApplicationEntry;
struct PackageDisplayMeta;

class DashboardPage final : public QWidget {
    Q_OBJECT
public:
    DashboardPage(const DashboardDefinition &definition, DashboardWidgetFactory *factory, QWidget *parent = nullptr);
    ~DashboardPage() override;
    DashboardDefinition snapshot() const;
    void setName(const QString &name);
    void showSaveError(const QString &error);
    void setCanAddPanel(std::function<bool()> check);
    bool addWidgetKind(const QString &kind);
    bool hasPendingChanges() const;
    bool confirmLeave();
    void confirmLeaveAsync(std::function<void(bool)> completion);
    bool canChangeSharedMachineContext(QString *error = nullptr) const;
    bool setSharedMachineContext(const QJsonObject &catalog, const QString &rootId, QString *error = nullptr);
    void setMachineCatalog(const QJsonArray &catalog);
    void setFullscreen(bool fullscreen);
    bool launchDesktopApplication(const DesktopApplicationEntry &application);
    void addWindowForApplication(const QString &name);
    bool attachPackageDisplay(const QString &title, qint64 processId, const QString &startToken, QString &error);
    bool attachPackageDisplay(const PackageDisplayMeta &display, QString &error);
    void releaseDesktopWindows();
    int panelCount() const { return m_definition.panels.size(); }
    QString environment() const;
    void setEnvironment(const QString &environment);
    void setTilingEnabled(bool enabled);
    bool applyOptions(const DashboardOptions &options);
    void showTour(bool automatic = false);

signals:
    void definitionChanged();
    void renameRequested();
    void removeRequested();
    void fullscreenRequested();
    void sidebarRequested();
    void appWorkflowRequested(const QString &id);
    void cloneWorkflowRequested(const QString &id);
    void projectChangeRequested();
    void workflowRunRequested(const QString &repository, const QString &path);
    void workflowRunOnMachineRequested(const QString &repository, const QString &path,
                                       const QJsonObject &target, const QString &rootId);

protected:
    void paintEvent(QPaintEvent *event) override;
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    QWidget *createContent(const QString &kind, const QString &environment, const QJsonObject &context = {});
    void tearOutBrowserTab(BrowserWidget *source, QWebEngineView *view, const QPoint &globalPosition);
    QString panelTitle(const QString &kind) const;
    void chooseWidget();
    void scheduleSave();
    void refreshDock();
    void chooseApp();
    void updateClock();
    void showSettings();
    void refreshDesktopBackground();
    void updateFileContexts();
    QJsonObject resolveMachineCatalog(const QJsonObject &machine) const;
    QString resolveMachineRoot(const QJsonObject &machine) const;
    bool applyPanelMachineContext(QWidget *content, const QJsonObject &machine, QString *error = nullptr);
    void installMachineMenu(QWidget *content, const QString &kind, const QJsonObject &machine);
    bool addPanel(const QString &kind, bool chooseWindow, const QJsonObject &context = {});
    void bindRepositoryPanel(const QString &id, QWidget *content);
    void openCanvasTool(const QString &kind, const QJsonObject &context);
    bool confirmPanelLeave(const QString &id, DashboardWorkspace *workspace);
    void confirmPanelLeaveAsync(const QString &id, DashboardWorkspace *workspace, std::function<void(bool)> completion);
    void closePanel(const QString &id, DashboardWorkspace *workspace);
    void removePanel(const QString &id, DashboardWorkspace *workspace);
    void bindWindow(const QString &id, QWidget *content);
    void enterTiling();
    DashboardWorkspace *workspaceFor(const QString &environment) const;
    DashboardDefinition m_definition;
    DashboardWidgetFactory *m_factory;
    DashboardWorkspace *m_workspace;
    DashboardWorkspace *m_canvasWorkspace;
    DashboardWorkspace *m_tilingWorkspace;
    QStackedWidget *m_environments;
    QSet<QString> m_automaticPanels;
    DashboardDock *m_dock;
    QFrame *m_toolbar;
    QLabel *m_hint;
    QPushButton *m_projectButton;
    QLabel *m_clock;
    QPushButton *m_fullscreenButton;
    QPushButton *m_sidebarButton;
    QTimer *m_clockTimer;
    QLabel *m_title;
    QWidget *m_errorRow;
    QLabel *m_error;
    QPushButton *m_retry;
    QPushButton *m_canvas;
    QPushButton *m_tiling;
    QTimer *m_saveTimer;
    std::function<bool()> m_canAddPanel;
    QLabel *m_backgroundStatus;
    QString m_desktopBackgroundPath;
    qint64 m_desktopBackgroundModified = -1;
    qint64 m_desktopBackgroundSize = -1;
    bool m_backgroundLookupPending = false;
    bool m_tourOpen = false;
    quint64 m_backgroundGeneration = 0;
    bool m_tilingEnabled = false;
    bool m_loading = true;
    bool m_fullscreen = false;
};
