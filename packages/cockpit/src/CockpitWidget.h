#pragma once
#include "WorkspaceFlightView.h"
#include "DesktopAppCatalog.h"
#include "SpatialWorkspaceStore.h"
#include "LauncherWorkspacePackage.h"
#include <QElapsedTimer>
#include <QProcess>
#include <QMap>
#include <QPointer>
#include <QTimer>
#include <QWidget>

class CockpitController;
class CockpitInputMenu;
class SpatialWebBrowser;
class GitReviewWidget;
class CockpitCommandDeck;
class CockpitHudPanel;
class CockpitVoice;
class ProjectFilesWidget;
class QLabel;
class QPushButton;
class QCheckBox;
class QComboBox;
class QStackedWidget;

// Standalone navigation surface over the launcher's existing operational pages.
// Owns ephemeral worlds, HUD layout, input lifetimes and one borrowed page.
class CockpitWidget final : public LauncherWorkspaceSurface {
    Q_OBJECT
public:
    explicit CockpitWidget(QWidget *parent = nullptr);
    ~CockpitWidget() override;
    void setWorkspace(const QString &root);
    void visit(const QString &destination, bool fullscreen = false);
    void returnToOrbit();
    const SpatialWorkspace &spatialWorkspace() const { return m_spatialWorkspace; }
    bool saveSpatialWorkspace(const SpatialWorkspace &workspace, QString &error);
    void editWorkspaces();
    void goBack();
    void closePanel();
    bool confirmLeave();
    WorkspaceFlightView *flight() const { return m_flight; }
    QString destination() const { return m_destination; }
    QString activePanel() const { return m_panelKey; }
    QString resourceSection() const override { return isVisible() ? m_resourceSection : QString(); }
    void applySettings(const CockpitSettings &settings);
    CockpitSettings settings() const { return m_preferences; }
    std::function<QVector<WorkspaceFlightBuilding>(const QString &)> worldProvider;
    std::function<QVector<DesktopAppChoice>(QString &)> appWindowProvider = DesktopAppWindow::availableWindows;
signals:
    void exitRequested();
    void settingsRequested();
    void panelActivityChanged(const QString &destination, bool active);
protected:
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    bool eventFilter(QObject *object, QEvent *event) override;
private:
    void openInputMenu(const QString &section = QString());
    void closeInputMenu(bool restoreFocus = true);
    void triggerInputAction(const QString &action);
    void useControllerHints(bool controller);
    CockpitInputMenu *m_inputMenu = nullptr;
    QString m_inputSection;
    bool m_controllerHints = false;
    bool m_controllerNeedsNeutral = false;
    float m_inputRoll = 0;
    bool m_inputBrake = false;
    float m_inputThrottle = 0;
    float m_inputServiceBrake = 0;
    void refreshWorkspaceDestinations();
    void setResourceSection(const QString &section);
    void refreshResources(bool force = false);
    bool activateOperation(const QString &path);
    void showMachine(bool inside);
    void customizeAndroid();
    QString m_resourceSection;
    QString m_machineId;
    bool m_insideMachine = false;
    int m_resourcePage = 0;
    QJsonObject m_resourceSnapshot;
    QTimer m_resourceTimer;
    QComboBox *m_workspaceDestinations = nullptr;
    void activateDisplay(const QString &id, WorkspaceFlightView *view = nullptr, bool control = true);
    void previewInteriorApps();
    QString m_pendingDisplay;
    QPointer<WorkspaceFlightView> m_pendingDisplayView;
    bool visitWorkspace(const QString &path);
    SpatialWorkspace m_spatialWorkspace;
    SpatialWorkspaceStore m_spatialStore;
    QString m_spatialError;
    void openPanel(const QString &key, QWidget *content, bool fullscreen);
    void openWeb();
    void openGit();
    WorkspaceFlightView *controlledFlight() const;
    bool pilotFlightActive() const;
    void openApps();
    void previewOpenApps();
    void openInstalledApps();
    void launchApp(const DesktopApplication &app, bool fullscreen);
    void awaitAppWindow();
    void activate(const QString &path, bool fullscreen);
    void previewPanel(const QString &path, const QString &key, QWidget *page);
    void arrange(bool enabled);
    void resetHud();
    void saveHud();
    void placeHud();
    void handleVoice(const QString &command);
    QStackedWidget *m_views = nullptr;
    WorkspaceFlightView *m_flight;
    CockpitCommandDeck *m_commandDeck = nullptr;
    CockpitHudPanel *m_navigation;
    CockpitHudPanel *m_controls;
    CockpitHudPanel *m_panel = nullptr;
    ProjectFilesWidget *m_files;
    QPointer<SpatialWebBrowser> m_web;
    QPointer<GitReviewWidget> m_git;
    CockpitVoice *m_voice;
    CockpitController *m_controller;
    int m_controllerVoiceButton = -1;
    QLabel *m_status;
    CockpitSettings m_preferences;
    QPushButton *m_arrange;
    QPushButton *m_back;
    QPointer<QWidget> m_content;
    QPointer<QWidget> m_returnParent;
    QPointer<QWidget> m_placeholder;
    int m_returnIndex = -1;
    QString m_panelKey;
    QString m_destination = "orbit";
    QString m_root;
    QMap<QString, DesktopAppChoice> m_apps;
    QVector<DesktopAppChoice> m_appPreviewChoices;
    QMap<QString, DesktopApplication> m_installedApps;
    QProcess m_appLaunch;
    QTimer m_appWindowTimer;
    QElapsedTimer m_appWindowDeadline;
    DesktopApplication m_pendingApp;
    bool m_pendingAppFullscreen = false;
    QRectF m_navigationPlacement{0.06, 0.12, 0.18, 0.46};
    QRectF m_controlsPlacement{0.72, 0.61, 0.23, 0.27};
    QRectF m_panelPlacement{0.12, 0.12, 0.76, 0.72};
    QTimer m_focusTimer;
    bool m_arranging = false;
    bool m_panelFullscreen = false;
};
