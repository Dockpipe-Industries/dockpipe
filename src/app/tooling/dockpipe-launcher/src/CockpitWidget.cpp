#include "CockpitWidget.h"
#include "CanvasIcons.h"
#include "CockpitController.h"
#include "CockpitCommandDeck.h"
#include "CockpitHudPanel.h"
#include "CockpitVoice.h"
#include "ProjectFilesWidget.h"
#include "SpatialWorkspaceEditor.h"
#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDir>
#include <QDirIterator>
#include <QFileDialog>
#include <QFileInfo>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QProcess>
#include <QPainter>
#include <QPushButton>
#include <QSettings>
#include <QSignalBlocker>
#include <QStackedWidget>
#include <QStandardPaths>
#include <QVBoxLayout>

namespace {
const QStringList destinations{"files", "apps", "workflows", "containers", "settings", "machines", "executions"};
QString titleFor(const QString &key)
{
    if (key == "files")
        return QObject::tr("Filesystem");
    QString title = key;
    if (!title.isEmpty())
        title[0] = title[0].toUpper();
    return title;
}
}

CockpitWidget::CockpitWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("cockpitWorkspace");
    // The host creates this view only after enabling the optional feature.
    m_preferences.enabled = true;
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    m_flight = new WorkspaceFlightView({}, !qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_SOFTWARE"), this);
    layout->addWidget(m_flight);
    m_commandDeck = new CockpitCommandDeck(m_flight);
    connect(m_commandDeck, &CockpitCommandDeck::destinationRequested, this, [this](const QString &destination) {
        m_commandDeck->setDestination(m_destination);
        if (m_flight->orbit())
            m_flight->approach(destination);
        else
            visit(destination);
        m_flight->setFocus();
    });
    m_flight->installEventFilter(this);
    m_files = new ProjectFilesWidget(this);
    m_files->hide();
    m_root = QDir::homePath();
    m_files->setRoot(m_root);
    m_appWindowTimer.setInterval(300);
    connect(&m_appWindowTimer, &QTimer::timeout, this, &CockpitWidget::awaitAppWindow);
    connect(&m_appLaunch, &QProcess::errorOccurred, this, [this](QProcess::ProcessError) {
        m_appWindowTimer.stop();
        m_status->setText(tr("The desktop could not launch that application."));
    });
    m_appLaunch.setStandardErrorFile(QProcess::nullDevice());
    m_appLaunch.setStandardOutputFile(QProcess::nullDevice());
    connect(&m_appLaunch, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int code, QProcess::ExitStatus status) {
            if (code != 0 || status != QProcess::NormalExit) {
                m_appWindowTimer.stop();
                m_status->setText(tr("The desktop could not launch that application."));
            }
        });
    m_voice = new CockpitVoice(this);
    m_controller = new CockpitController(this);
    m_navigation = new CockpitHudPanel(tr("NAVIGATION / SOL SYSTEM"), m_flight);
    m_navigation->setObjectName("cockpitNavigation");
    m_navigation->setClosable(false);
    const QMap<QString, CanvasIcons::Symbol> symbols{
        {"files", CanvasIcons::Symbol::Folder}, {"apps", CanvasIcons::Symbol::Monitor},
        {"workflows", CanvasIcons::Symbol::Workflow}, {"containers", CanvasIcons::Symbol::Parallel},
        {"settings", CanvasIcons::Symbol::Properties}, {"machines", CanvasIcons::Symbol::Terminal},
        {"executions", CanvasIcons::Symbol::Play}};
    for (const QString &key : destinations) {
        auto *button = new QPushButton(CanvasIcons::icon(symbols.value(key)), titleFor(key));
        button->setObjectName("cockpitDestination_" + key);
        m_navigation->body()->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, key]() { visit(key); });
    }
    m_workspaceDestinations = new QComboBox;
    m_workspaceDestinations->setObjectName("cockpitWorkspaceDestinations");
    m_workspaceDestinations->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    m_navigation->body()->addWidget(m_workspaceDestinations);
    connect(m_workspaceDestinations, &QComboBox::activated, this, [this](int index) {
        const QString path = m_workspaceDestinations->itemData(index).toString();
        if (!path.isEmpty())
            visit(path);
    });
    auto *workspaces = new QPushButton(tr("Custom workspaces…"));
    workspaces->setObjectName("cockpitWorkspaces");
    m_navigation->body()->addWidget(workspaces);
    connect(workspaces, &QPushButton::clicked, this, &CockpitWidget::editWorkspaces);
    auto *currentPanel = new QPushButton(tr("Open destination panel"));
    m_navigation->body()->addWidget(currentPanel);
    connect(currentPanel, &QPushButton::clicked, this, [this]() {
        if (m_destination == "files")
            openPanel("files", m_files, false);
        else if (m_destination == "apps")
            openInstalledApps();
        else if (m_destination.startsWith("planet:") || m_destination.startsWith("building:"))
            editWorkspaces();
        else if (pageProvider)
            openPanel(m_destination, pageProvider(m_destination), false);
    });
    auto *root = new QPushButton(tr("Choose filesystem root…"));
    m_navigation->body()->addWidget(root);
    connect(root, &QPushButton::clicked, this, [this]() {
        const QString selected = QFileDialog::getExistingDirectory(this, tr("Filesystem planet root"), m_root);
        if (!selected.isEmpty() && m_files->setRoot(selected)) {
            m_root = selected;
            visit("files");
        }
    });
    auto *orbit = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Undo), tr("Return to orbit"));
    m_navigation->body()->addWidget(orbit);
    connect(orbit, &QPushButton::clicked, this, &CockpitWidget::returnToOrbit);
    auto *leave = new QPushButton(tr("Leave cockpit"));
    m_navigation->body()->addWidget(leave);
    connect(leave, &QPushButton::clicked, this, &CockpitWidget::exitRequested);
    m_controls = new CockpitHudPanel(tr("SHIP / CONTROLS"), m_flight);
    m_controls->setObjectName("cockpitControls");
    m_controls->setClosable(false);
    m_arrange = new QPushButton(tr("Arrange HUD"));
    m_arrange->setCheckable(true);
    m_controls->body()->addWidget(m_arrange);
    connect(m_arrange, &QPushButton::toggled, this, &CockpitWidget::arrange);
    auto *reset = new QPushButton(tr("Reset HUD layout"));
    m_controls->body()->addWidget(reset);
    connect(reset, &QPushButton::clicked, this, &CockpitWidget::resetHud);
    auto *talk = new QPushButton(tr("Hold to talk · Local"));
    talk->setObjectName("cockpitPushToTalk");
    talk->setToolTip(tr("Open files/apps/workflows/containers/settings/machines/executions; return to orbit; close panel; full screen; exit full screen; reset HUD."));
    m_controls->body()->addWidget(talk);
    connect(talk, &QPushButton::pressed, m_voice, &CockpitVoice::start);
    connect(talk, &QPushButton::released, m_voice, &CockpitVoice::finish);
    auto *preferences = new QPushButton(tr("Cockpit settings…"));
    preferences->setObjectName("cockpitOpenSettings");
    m_controls->body()->addWidget(preferences);
    connect(preferences, &QPushButton::clicked, this, &CockpitWidget::settingsRequested);
    talk->setEnabled(false);
    m_status = new QLabel(tr("WASD · Q/E altitude · Right-drag steer · Middle-drag look around · Shift boost\nClick a file or open app to preview. Back / Backspace or the return portal takes you one level back."));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_controls->body()->addWidget(m_status);
    connect(m_voice, &CockpitVoice::statusChanged, m_status, &QLabel::setText);
    connect(m_voice, &CockpitVoice::commandRecognized, this, &CockpitWidget::handleVoice);
    connect(m_controller, &CockpitController::statusChanged, m_status, &QLabel::setText);
    connect(m_controller, &CockpitController::pilotInput, m_flight, &WorkspaceFlightView::setPilotInput);
    connect(m_controller, &CockpitController::buttonChanged, this, [this](int button, bool pressed) {
        if (button == 10 && m_preferences.voiceCommands) {
            if (pressed)
                m_voice->start();
            else
                m_voice->finish();
        }
        if (!pressed)
            return;
        if (button == 0 && !m_flight->editorActive())
            m_flight->fire();
        else if (button == 1) {
            if (m_content || m_flight->editorActive()) {
                closePanel();
                m_flight->stopAppBillboards();
            } else {
                m_flight->goBack();
            }
        } else if (button == 6) {
            m_flight->setFlightFullscreen(!m_flight->flightFullscreen());
        }
    });
    m_focusTimer.setInterval(100);
    connect(&m_focusTimer, &QTimer::timeout, this, [this]() {
        const bool active = isVisible() && window()->isActiveWindow() && !QApplication::activeModalWidget();
        m_controller->setInputActive(active);
        if (!active)
            m_voice->cancel();
    });
    m_panel = new CockpitHudPanel(tr("HUD"), m_flight);
    m_panel->setObjectName("cockpitContentPanel");
    m_panel->hide();
    connect(m_panel, &CockpitHudPanel::closeRequested, this, &CockpitWidget::closePanel);
    connect(m_panel, &CockpitHudPanel::fullscreenRequested, this, [this]() {
        m_panelFullscreen = !m_panelFullscreen;
        m_flight->setFlightFullscreen(m_panelFullscreen);
        placeHud();
    });
    for (auto *panel : {m_navigation, m_controls, m_panel}) {
        connect(panel, &CockpitHudPanel::placementChanged, this, [this, panel]() {
            if (panel == m_navigation)
                m_navigationPlacement = panel->placement();
            else if (panel == m_controls)
                m_controlsPlacement = panel->placement();
            else
                m_panelPlacement = panel->placement();
            saveHud();
        });
    }
    QSettings settings;
    m_navigationPlacement = settings.value("cockpit/navigation", m_navigationPlacement).toRectF();
    m_controlsPlacement = settings.value("cockpit/controls", m_controlsPlacement).toRectF();
    m_panelPlacement = settings.value("cockpit/panel", m_panelPlacement).toRectF();
    // Keep the windshield clear for piloting. Configuration remains one click
    // away, and can be arranged on the windshield when the user needs it.
    m_navigation->hide();
    m_controls->hide();
    auto *navigationToggle = new QPushButton(tr("Navigation"), m_flight);
    navigationToggle->setObjectName("cockpitNavigationToggle");
    navigationToggle->setGeometry(16, 12, 126, 38);
    connect(navigationToggle, &QPushButton::clicked, this, [this]() {
        m_navigation->setVisible(m_navigation->isHidden());
        m_navigation->raise();
        placeHud();
    });
    auto *controlsToggle = new QPushButton(tr("Ship controls"), m_flight);
    controlsToggle->setObjectName("cockpitControlsToggle");
    controlsToggle->setGeometry(150, 12, 132, 38);
    connect(controlsToggle, &QPushButton::clicked, this, [this]() {
        m_controls->setVisible(m_controls->isHidden());
        m_controls->raise();
        placeHud();
    });
    m_back = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Undo), tr("Back · ⌫"), m_flight);
    m_back->setObjectName("cockpitBack");
    m_back->setGeometry(290, 12, 116, 38);
    for (auto *button : {navigationToggle, controlsToggle, m_back}) {
        button->setStyleSheet("QPushButton { background: rgba(8, 17, 25, 225); color: #b4cbc9; border: 1px solid #3b525b; border-radius: 5px; padding: 7px; font-size: 11px; }"
            "QPushButton:hover { background: #20343e; border-color: #88b7b4; }"
            "QPushButton:disabled { color: #5b6a74; border-color: #28343d; }");
    }
    m_back->setToolTip(tr("Return one level: parent folder, or orbit from a planet. You can also fly through the return portal."));
    connect(m_back, &QPushButton::clicked, m_flight, &WorkspaceFlightView::goBack);
    m_flight->destinationActivated = [this](const QString &path, bool fullscreen) { activate(path, fullscreen); };
    m_flight->orbitRequested = [this]() { goBack(); };
    m_flight->exitRequested = [this]() { emit exitRequested(); };
    m_flight->fileTargeted = [this](const QString &path) {
        if (m_files->openFile(path))
            openPanel("files", m_files, false);
    };
    connect(m_files, &ProjectFilesWidget::workflowOpenRequested, this, [this](const QString &path) {
        if (workflowOpener && workflowOpener(path) && pageProvider)
            openPanel("workflows", pageProvider("workflows"), false);
    });
    m_flight->rendererChanged = [this]() { placeHud(); };
    m_spatialStore.load(m_spatialWorkspace, m_spatialError);
    refreshWorkspaceDestinations();
    returnToOrbit();
}

CockpitWidget::~CockpitWidget()
{
    m_voice->cancel();
    m_controller->disable();
    m_appLaunch.kill();
    m_appLaunch.waitForFinished(1000);
    closePanel();
    m_flight->setFlightFullscreen(false);
}

void CockpitWidget::setWorkspace(const QString &root)
{
    if (!root.isEmpty() && root != m_root && m_files->setRoot(root))
        m_root = root;
}

void CockpitWidget::applySettings(const CockpitSettings &settings)
{
    const bool wasCapturing = m_preferences.enabled && m_preferences.appControl;
    const bool wasController = m_preferences.enabled && m_preferences.controller;
    m_preferences = settings;
    m_files->applyCockpitSettings(settings);
    const bool captureEnabled = settings.enabled && settings.appControl;
    const bool controllerEnabled = settings.enabled && settings.controller;
    m_flight->setShipVisible(settings.showShip);
    m_flight->setOrbitMotion(settings.enabled && settings.orbitalMotion);
    if (settings.hardwareAcceleration != m_flight->hardwareAccelerationRequested())
        m_flight->setFlightHardwareAcceleration(settings.hardwareAcceleration);
    findChild<QPushButton *>("cockpitPushToTalk")->setEnabled(settings.enabled && settings.voiceCommands);
    if (!settings.enabled || !settings.voiceCommands)
        m_voice->cancel();
    if (!captureEnabled) {
        m_appWindowTimer.stop();
        m_pendingDisplay.clear();
        m_flight->stopAppBillboards();
    } else if (!wasCapturing && isVisible()) {
        if (m_flight->walking())
            previewInteriorApps();
        else if (m_destination == "apps")
            previewOpenApps();
    }
    if (!controllerEnabled)
        m_controller->disable();
    else if (!wasController && isVisible())
        m_controller->enable();
    if (!settings.enabled) {
        m_flight->setFlightFullscreen(false);
        m_flight->setEditorActive(true);
    } else if (!m_content && !m_arranging && !m_flight->appControlling()) {
        m_flight->setEditorActive(false);
    }
}

bool CockpitWidget::confirmLeave() { return m_files->confirmLeave(); }

void CockpitWidget::returnToOrbit()
{
    closePanel();
    m_appWindowTimer.stop();
    m_pendingDisplay.clear();
    m_destination = "orbit";
    m_back->setEnabled(false);
    QVector<WorkspaceFlightBuilding> planets;
    for (const QString &key : destinations)
        planets.append({titleFor(key), 100, true, false, key});
    for (const auto &planet : m_spatialWorkspace.planets) {
        WorkspaceFlightBuilding destination{planet.name, 100, true, false, "planet:" + planet.id};
        destination.theme = planet.theme;
        planets.append(destination);
    }
    m_flight->setCockpitWorld(planets, true);
    m_commandDeck->setDestination(m_destination);
    placeHud();
    m_navigation->setTitle(tr("NAVIGATION / SOL SYSTEM"));
    m_status->setText(tr("Select a planet or a deck destination to engage autopilot. WASD flies, right-drag steers, and middle-drag looks around the cabin. Orbital motion pauses while a destination is approached."));
    m_flight->setFocus();
}

void CockpitWidget::goBack()
{
    closePanel();
    m_flight->stopAppBillboards();
    if (m_destination.startsWith("building:")) {
        for (const auto &planet : m_spatialWorkspace.planets) {
            for (const auto &building : planet.buildings) {
                if (m_destination == "building:" + building.id) {
                    visit("planet:" + planet.id);
                    return;
                }
            }
        }
    }
    if (m_destination == "files" && !m_flight->directory().isEmpty()
        && m_flight->directory() != QDir::cleanPath(m_root)) {
        m_flight->goUp();
    } else {
        returnToOrbit();
    }
    m_flight->setFocus();
}

void CockpitWidget::visit(const QString &destination, bool fullscreen)
{
    if (visitWorkspace(destination)) {
        m_commandDeck->setDestination(m_destination);
        placeHud();
        return;
    }
    if (!destinations.contains(destination))
        return;
    closePanel();
    m_appWindowTimer.stop();
    if (m_appLaunch.state() != QProcess::NotRunning)
        m_appLaunch.kill();
    m_pendingDisplay.clear();
    m_destination = destination;
    m_back->setEnabled(true);
    m_navigation->setTitle(tr("NAVIGATION / %1").arg(titleFor(destination).toUpper()));
    if (destination == "apps") {
        openApps();
    } else if (destination == "files") {
        m_flight->setCockpitWorld({}, false);
        m_flight->setWorkspace(m_root, m_root, true);
        m_status->setText(tr("Filesystem: %1 · Click a file for a hologram; select again to edit. Fly into folder hangars. Back or the return portal takes you one level up.").arg(m_root));
    } else if (destination == "settings") {
        m_flight->setCockpitWorld({{tr("Settings control"), 100, true, false, "panel:settings"}}, false);
    } else {
        QVector<WorkspaceFlightBuilding> buildings;
        if (worldProvider)
            buildings = worldProvider(destination);
        if (buildings.isEmpty())
            buildings.append({titleFor(destination) + tr(" control"), 100, true, false, "panel:" + destination});
        m_flight->setCockpitWorld(buildings, false);
    }
    m_flight->setFocus();
    m_commandDeck->setDestination(m_destination);
    placeHud();
}

void CockpitWidget::activate(const QString &path, bool fullscreen)
{
    if (path.startsWith("display:")) {
        activateDisplay(path.mid(8));
    } else if (path.startsWith("planet:") || path.startsWith("building:")) {
        visit(path, fullscreen);
    } else if (destinations.contains(path)) {
        visit(path, fullscreen);
    } else if (m_apps.contains(path)) {
        if (!(m_preferences.enabled && m_preferences.appControl)) {
            m_status->setText(tr("Enable App control under Settings → Cockpit mode before connecting a window."));
            m_controls->show();
            m_controls->raise();
            placeHud();
            return;
        }
        const QString unavailable = DesktopAppWindow::unavailableReason();
        if (!unavailable.isEmpty()) {
            m_status->setText(unavailable);
            return;
        }
        closePanel();
        if (!fullscreen && m_flight->previewPath() != path)
            m_flight->previewApp(path, m_apps.value(path));
        else
            m_flight->controlApp(m_apps.value(path), fullscreen);
    } else if (m_installedApps.contains(path)) {
        const auto app = m_installedApps.value(path);
        if (!fullscreen && m_flight->previewPath() != path)
            m_flight->previewCard(path, app.name, tr("%1\n\nInstalled application\n\nSelect again to launch and connect its window.").arg(app.name));
        else
            launchApp(app, fullscreen);
    } else if (path == "installed-apps") {
        if (!fullscreen && m_flight->previewPath() != path)
            m_flight->previewCard(path, tr("Installed apps"), tr("Browse installed desktop applications.\n\nSearch the catalog and choose an app to launch.\nNo app starts until you select Launch."));
        else
            openInstalledApps();
    } else if (path.startsWith("workflow:")) {
        if (!fullscreen && m_flight->previewPath() != path)
            m_flight->previewDocument(path, path.mid(9));
        else if (workflowOpener && workflowOpener(path.mid(9)) && pageProvider)
            openPanel("workflows", pageProvider("workflows"), fullscreen);
    } else if (path.startsWith("panel:") && pageProvider) {
        const QString key = path.mid(6);
        QWidget *page = pageProvider(key);
        if (!fullscreen && m_flight->previewPath() != path)
            previewPanel(path, key, page);
        else
            openPanel(key, page, fullscreen);
    }
}

void CockpitWidget::previewPanel(const QString &path, const QString &key, QWidget *page)
{
    if (!page) {
        m_flight->previewCard(path, titleFor(key), tr("This control surface is unavailable."));
        return;
    }
    // One bounded snapshot of the existing widget. Rendering never reparents
    // or shows it, activates its monitors, or changes the selected workflow.
    page->ensurePolished();
    if (page->layout())
        page->layout()->activate();
    const QSize source = page->size().expandedTo(QSize(1, 1));
    QImage snapshot(768, 432, QImage::Format_ARGB32_Premultiplied);
    snapshot.fill(page->palette().color(QPalette::Window));
    QPainter painter(&snapshot);
    const qreal scale = qMin(768.0 / source.width(), 432.0 / source.height());
    painter.translate((768 - source.width() * scale) / 2, (432 - source.height() * scale) / 2);
    painter.scale(scale, scale);
    page->render(&painter, {}, QRegion(page->rect()));
    painter.end();
    m_flight->previewSurface(path, titleFor(key), snapshot);
}

void CockpitWidget::openApps()
{
    QString error;
    const auto windows = appWindowProvider(error);
    m_apps.clear();
    m_appPreviewChoices.clear();
    QVector<WorkspaceFlightBuilding> buildings;
    buildings.append({tr("Launch an installed app"), 110, true, false, "installed-apps"});
    for (const auto &window : windows) {
        if (buildings.size() == 150)
            break;
        const QString key = "app:" + QString::number(window.id);
        m_apps.insert(key, window);
        if (m_appPreviewChoices.size() < 3)
            m_appPreviewChoices.append(window);
        buildings.append({window.title, 95, true, false, key});
    }
    m_installedApps.clear();
    for (const auto &app : DesktopAppCatalog::applications()) {
        if (buildings.size() >= 150)
            break;
        const QString key = "launch:" + app.desktopFile;
        m_installedApps.insert(key, app);
        buildings.append({app.name, 100, true, false, key});
    }
    // Keep the running apps together in front of the installed-app catalog.
    // A large catalog must not push their first row far outside the windshield.
    for (int index = 0; index < buildings.size(); ++index) {
        auto &building = buildings[index];
        building.positioned = true;
        if (index == 0) {
            building.position = QPointF(-640, 0);
        } else if (index <= m_apps.size()) {
            const int slot = index - 1;
            const int columns = qMin(3, int(m_apps.size()));
            building.position = QPointF((slot % columns - (columns - 1) / 2.0) * 320, (slot / columns) * 320);
        } else {
            const int slot = index - m_apps.size() - 1;
            const int openRows = (m_apps.size() + 2) / 3;
            building.position = QPointF((slot % 6 - 2.5) * 320, (openRows + 1 + slot / 6) * 320);
        }
    }
    m_flight->setCockpitWorld(buildings, false);
    if (m_preferences.enabled && m_preferences.appControl)
        previewOpenApps();
    m_status->setText(error.isEmpty()
        ? tr("%1 open apps · Enable App control for live wall previews. Click for a hologram, select again for HUD control. Revisit Apps to refresh.").arg(m_apps.size())
        : error);
}

void CockpitWidget::previewOpenApps()
{
    m_flight->previewAppWindows(m_appPreviewChoices);
}

void CockpitWidget::openInstalledApps()
{
    auto *view = new QWidget;
    auto *layout = new QVBoxLayout(view);
    auto *search = new QLineEdit;
    search->setPlaceholderText(tr("Find an installed application…"));
    layout->addWidget(search);
    auto *list = new QListWidget;
    layout->addWidget(list, 1);
    auto *hint = new QLabel(tr("Launch an installed app, then revisit Apps to connect its window. Dockpipe app workflows remain available in the Workflows planet."));
    hint->setWordWrap(true);
    layout->addWidget(hint);
    auto *launch = new QPushButton(tr("Launch selected app"));
    launch->setEnabled(false);
    layout->addWidget(launch);
    auto *place = new QPushButton(tr("Place in a custom workspace…"));
    place->setEnabled(false);
    layout->addWidget(place);
    const auto applications = DesktopAppCatalog::applications();
    for (int index = 0; index < applications.size(); ++index) {
        auto *item = new QListWidgetItem(CanvasIcons::icon(CanvasIcons::Symbol::Monitor), applications[index].name, list);
        item->setData(Qt::UserRole, index);
    }
    list->sortItems();
    connect(list, &QListWidget::currentRowChanged, view, [place](int row) { place->setEnabled(row >= 0); });
    connect(place, &QPushButton::clicked, view, [this, list, applications]() {
        if (!list->currentItem())
            return;
        const auto app = applications[list->currentItem()->data(Qt::UserRole).toInt()];
        editWorkspaces();
        if (auto *editor = qobject_cast<SpatialWorkspaceEditor *>(m_content.data()))
            editor->preferApplication({app.desktopId, app.windowClass, app.name});
    });
    connect(search, &QLineEdit::textChanged, view, [list](const QString &query) {
        for (int index = 0; index < list->count(); ++index)
            list->item(index)->setHidden(!list->item(index)->text().contains(query, Qt::CaseInsensitive));
    });
    connect(list, &QListWidget::currentRowChanged, view, [launch](int row) { launch->setEnabled(row >= 0); });
    connect(launch, &QPushButton::clicked, view, [this, list, applications]() {
        if (list->currentItem())
            launchApp(applications[list->currentItem()->data(Qt::UserRole).toInt()], false);
    });
    openPanel("ephemeral", view, false);
}

void CockpitWidget::launchApp(const DesktopApplication &app, bool fullscreen)
{
    const QString gio = QStandardPaths::findExecutable("gio");
    if (gio.isEmpty() || m_appLaunch.state() != QProcess::NotRunning) {
        m_status->setText(tr("Application launch needs Linux GIO; wait for any pending launch to finish."));
        return;
    }
    closePanel();
    m_pendingApp = app;
    m_pendingAppFullscreen = fullscreen;
    m_appLaunch.start(gio, {"launch", app.desktopFile});
    m_appWindowDeadline.start();
    m_appWindowTimer.start();
    m_status->setText(tr("Launching %1…").arg(app.name));
}

void CockpitWidget::awaitAppWindow()
{
    if (m_appWindowDeadline.elapsed() > 10000) {
        m_appWindowTimer.stop();
        if (m_appLaunch.state() != QProcess::NotRunning)
            m_appLaunch.kill();
        m_status->setText(tr("Launch requested. Revisit Apps to select the window if it could not be matched automatically."));
        return;
    }
    if (!(m_preferences.enabled && m_preferences.appControl))
        return;
    QString error;
    QVector<DesktopAppChoice> matches;
    for (const auto &window : DesktopAppWindow::availableWindows(error)) {
        if (!m_pendingApp.windowClass.isEmpty()
            && window.applicationClass.compare(m_pendingApp.windowClass, Qt::CaseInsensitive) == 0)
            matches.append(window);
    }
    // Never guess between several windows or attach an unrelated new window.
    if (matches.size() == 1) {
        m_appWindowTimer.stop();
        if (!m_pendingDisplay.isEmpty()) {
            m_flight->connectInteriorDisplay(m_pendingDisplay, matches.first(), true);
            m_pendingDisplay.clear();
        } else {
            m_flight->controlApp(matches.first(), m_pendingAppFullscreen);
        }
        m_status->setText(tr("Connected to %1").arg(m_pendingApp.name));
    }
}

void CockpitWidget::openPanel(const QString &key, QWidget *content, bool fullscreen)
{
    if (!content)
        return;
    closePanel();
    m_flight->stopAppBillboards();
    m_returnParent = content->parentWidget();
    auto *stack = qobject_cast<QStackedWidget *>(m_returnParent.data());
    if (stack) {
        const QSignalBlocker blocker(stack);
        m_returnIndex = stack->indexOf(content);
        m_placeholder = new QWidget;
        stack->insertWidget(m_returnIndex, m_placeholder);
        stack->removeWidget(content);
    }
    m_content = content;
    m_panelKey = key;
    content->setParent(m_panel);
    m_panel->body()->addWidget(content);
    content->show();
    m_panel->setTitle(titleFor(m_destination) + tr(" / HUD"));
    m_flight->setEditorActive(true);
    m_panelFullscreen = fullscreen;
    if (fullscreen)
        m_flight->setFlightFullscreen(true);
    placeHud();
    m_panel->show();
    m_panel->raise();
    emit panelActivityChanged(key, true);
}

void CockpitWidget::closePanel()
{
    if (!m_content)
        return;
    QWidget *content = m_content;
    const bool spatialEditor = qobject_cast<SpatialWorkspaceEditor *>(content) != nullptr;
    const QString key = m_panelKey;
    m_panel->body()->removeWidget(content);
    content->hide();
    if (auto *stack = qobject_cast<QStackedWidget *>(m_returnParent.data())) {
        const QSignalBlocker blocker(stack);
        const bool returningToPage = stack->currentWidget() == m_placeholder;
        stack->insertWidget(m_returnIndex, content);
        if (returningToPage)
            stack->setCurrentWidget(content);
        if (m_placeholder) {
            stack->removeWidget(m_placeholder);
            delete m_placeholder;
        }
    } else if (key == "ephemeral") {
        content->deleteLater();
    } else if (m_returnParent) {
        content->setParent(m_returnParent);
    }
    m_placeholder.clear();
    m_content.clear();
    m_returnParent.clear();
    m_returnIndex = -1;
    m_panelKey.clear();
    m_panelFullscreen = false;
    m_panel->hide();
    m_flight->setEditorActive(false);
    emit panelActivityChanged(key, false);
    if (spatialEditor) {
        if (m_destination == "orbit")
            returnToOrbit();
        else
            visit(m_destination);
    }
    placeHud();
    m_flight->setFocus();
}

void CockpitWidget::arrange(bool enabled)
{
    m_arranging = enabled;
    if (enabled) {
        m_navigation->show();
        m_controls->show();
    }
    for (auto *panel : {m_navigation, m_controls, m_panel})
        panel->setArrangementEnabled(enabled);
    m_arrange->setText(enabled ? tr("Lock HUD layout") : tr("Arrange HUD"));
    m_flight->setEditorActive(enabled || bool(m_content) || m_flight->appControlling());
    placeHud();
    if (!enabled)
        saveHud();
}

void CockpitWidget::saveHud()
{
    QSettings settings;
    settings.setValue("cockpit/navigation", m_navigationPlacement);
    settings.setValue("cockpit/controls", m_controlsPlacement);
    settings.setValue("cockpit/panel", m_panelPlacement);
}

void CockpitWidget::resetHud()
{
    m_navigationPlacement = QRectF(0.06, 0.12, 0.18, 0.46);
    m_controlsPlacement = QRectF(0.72, 0.61, 0.23, 0.27);
    m_panelPlacement = QRectF(0.12, 0.12, 0.76, 0.72);
    placeHud();
    saveHud();
}

void CockpitWidget::placeHud()
{
    m_navigation->place(m_navigationPlacement);
    m_controls->place(m_controlsPlacement);
    m_status->setMinimumHeight(qMax(48, m_status->heightForWidth(qMax(100, m_controls->width() - 24))));
    m_controls->fitToViewport();
    m_panel->place(m_panelFullscreen ? QRectF(0, 0, 1, 1) : m_panelPlacement);
    const int deckWidth = qMin(1120, qMax(1, m_flight->width() - 48));
    m_commandDeck->setGeometry((m_flight->width() - deckWidth) / 2, qMax(0, m_flight->height() - 64), deckWidth, 52);
    m_commandDeck->setVisible(!m_flight->editorActive() && !m_flight->walking());
    m_commandDeck->raise();
    const bool compact = m_flight->width() < 700;
    if (auto *navigation = m_flight->findChild<QPushButton *>("cockpitNavigationToggle")) {
        navigation->setText(compact ? tr("Nav") : tr("Navigation"));
        navigation->setGeometry(16, 12, compact ? 74 : 126, 38);
    }
    if (auto *controls = m_flight->findChild<QPushButton *>("cockpitControlsToggle")) {
        controls->setText(compact ? tr("Controls") : tr("Ship controls"));
        controls->setGeometry(compact ? 98 : 150, 12, compact ? 84 : 132, 38);
    }
    m_back->setText(compact ? tr("Back") : tr("Back · ⌫"));
    m_back->setGeometry(compact ? 190 : 290, 12, compact ? 68 : 116, 38);
    // A restored OpenGL viewport is a new child; keep navigation above it.
    for (const QString &name : {QString("cockpitNavigationToggle"), QString("cockpitControlsToggle"), QString("cockpitBack")}) {
        if (auto *button = m_flight->findChild<QPushButton *>(name))
            button->raise();
    }
    for (auto *panel : {m_navigation, m_controls, m_panel}) {
        if (panel->isVisible())
            panel->raise();
    }
}

void CockpitWidget::handleVoice(const QString &command)
{
    if (!isVisible())
        return;
    if (command == "open workspace") {
        editWorkspaces();
        return;
    }
    if (command == "next workspace" && !m_spatialWorkspace.planets.isEmpty()) {
        int next = 0;
        for (int index = 0; index < m_spatialWorkspace.planets.size(); ++index) {
            if (m_destination == "planet:" + m_spatialWorkspace.planets[index].id)
                next = (index + 1) % m_spatialWorkspace.planets.size();
        }
        visit("planet:" + m_spatialWorkspace.planets[next].id);
        return;
    }
    if (command.startsWith("open ") && destinations.contains(command.mid(5)))
        visit(command.mid(5));
    else if (command == "return to orbit")
        returnToOrbit();
    else if (command == "close panel") {
        closePanel();
        m_flight->stopAppBillboards();
    } else if (command == "full screen")
        m_flight->setFlightFullscreen(true);
    else if (command == "exit full screen")
        m_flight->setFlightFullscreen(false);
    else if (command == "reset hud")
        resetHud();
}

void CockpitWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_preferences.enabled && m_preferences.appControl) {
        if (m_flight->walking())
            previewInteriorApps();
        else if (m_destination == "apps")
            previewOpenApps();
    }
    placeHud();
    m_focusTimer.start();
    if (m_preferences.enabled && m_preferences.controller)
        m_controller->enable();
}

void CockpitWidget::hideEvent(QHideEvent *event)
{
    m_voice->cancel();
    m_controller->disable();
    m_focusTimer.stop();
    m_appWindowTimer.stop();
    if (m_appLaunch.state() != QProcess::NotRunning)
        m_appLaunch.kill();
    // QStackedLayout is still switching to the placeholder during Hide.
    // Restore the borrowed page after that transition, never delete its current
    // target underneath Qt's setCurrentIndex call.
    m_panel->hide();
    QTimer::singleShot(0, this, [this]() {
        if (!isVisible())
            closePanel();
    });
    m_flight->stopAppBillboards();
    m_flight->setFlightFullscreen(false);
    QWidget::hideEvent(event);
}

bool CockpitWidget::eventFilter(QObject *object, QEvent *event)
{
    if (m_panel && object == m_flight && event->type() == QEvent::Resize)
        placeHud();
    return QWidget::eventFilter(object, event);
}
