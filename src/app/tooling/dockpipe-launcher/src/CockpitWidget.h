#pragma once
#include "WorkspaceFlightView.h"
#include "DesktopAppCatalog.h"
#include "SpatialWorkspaceStore.h"
#include "CockpitSettings.h"
#include <QElapsedTimer>
#include <QProcess>
#include <QMap>
#include <QPointer>
#include <QTimer>
#include <QWidget>

class CockpitController;
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
class CockpitWidget final : public QWidget {
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
    void applySettings(const CockpitSettings &settings);
    CockpitSettings settings() const { return m_preferences; }
    std::function<QWidget *(const QString &)> pageProvider;
    std::function<QVector<WorkspaceFlightBuilding>(const QString &)> worldProvider;
    std::function<bool(const QString &)> workflowOpener;
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
    void refreshWorkspaceDestinations();
    QComboBox *m_workspaceDestinations = nullptr;
    void activateDisplay(const QString &id);
    void previewInteriorApps();
    QString m_pendingDisplay;
    bool visitWorkspace(const QString &path);
    SpatialWorkspace m_spatialWorkspace;
    SpatialWorkspaceStore m_spatialStore;
    QString m_spatialError;
    void openPanel(const QString &key, QWidget *content, bool fullscreen);
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
    WorkspaceFlightView *m_flight;
    CockpitCommandDeck *m_commandDeck = nullptr;
    CockpitHudPanel *m_navigation;
    CockpitHudPanel *m_controls;
    CockpitHudPanel *m_panel = nullptr;
    ProjectFilesWidget *m_files;
    CockpitVoice *m_voice;
    CockpitController *m_controller;
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
