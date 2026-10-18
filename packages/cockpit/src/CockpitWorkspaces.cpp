#include "CockpitWidget.h"
#include "FurnishingController.h"
#include "CockpitHudPanel.h"
#include "SpatialWorkspaceEditor.h"
#include <QLabel>
#include <QPushButton>
#include <QCheckBox>
#include <QComboBox>
#include <QInputDialog>
#include <QMessageBox>

bool CockpitWidget::saveSpatialWorkspace(const SpatialWorkspace &workspace, QString &error)
{
    if (!m_spatialStore.save(workspace, error))
        return false;
    m_spatialWorkspace = workspace;
    m_spatialError.clear();
    refreshWorkspaceDestinations();
    return true;
}

void CockpitWidget::editWorkspaces()
{
    auto *editor = new SpatialWorkspaceEditor(m_spatialWorkspace);
    editor->save = [this](const SpatialWorkspace &workspace, QString &error) {
        return saveSpatialWorkspace(workspace, error);
    };
    editor->reload = [this](SpatialWorkspace &workspace, QString &error) {
        if (!m_spatialStore.load(workspace, error))
            return false;
        m_spatialWorkspace = workspace;
        m_spatialError.clear();
        refreshWorkspaceDestinations();
        return true;
    };
    connect(editor, &SpatialWorkspaceEditor::furnishRequested, this, [this](const QString &path) {
        visit(path);
        if (m_flight->furnishing() && !m_flight->furnishingActive())
            m_flight->furnishing()->toggle();
    });
    connect(editor, &SpatialWorkspaceEditor::travelRequested, this,
        [this](const QString &path) { visit(path); });
    connect(editor, &SpatialWorkspaceEditor::approachRequested, this, [this](const QString &path) {
        if (path.startsWith("planet:")) {
            returnToOrbit();
        } else {
            for (const auto &planet : m_spatialWorkspace.planets) {
                for (const auto &building : planet.buildings) {
                    if (path == "building:" + building.id)
                        visit("planet:" + planet.id);
                }
            }
        }
        m_flight->approach(path);
    });
    openPanel("ephemeral", editor, false);
    m_panel->setTitle(tr("CUSTOM WORKSPACES / EDIT LAYOUT"));
    if (!m_spatialError.isEmpty())
        editor->showStatus(m_spatialError);
}

bool CockpitWidget::visitWorkspace(const QString &path)
{
    if (path.startsWith("building:")) {
        for (const auto &planet : m_spatialWorkspace.planets) {
            for (const auto &building : planet.buildings) {
                if (path != "building:" + building.id)
                    continue;
                closePanel();
                m_appWindowTimer.stop();
                m_pendingDisplay.clear();
                m_destination = path;
                m_back->setEnabled(true);
                m_flight->setInterior(building.scene(), building.name, false, false, "workspace:" + building.id);
                if ((m_preferences.enabled && m_preferences.appControl))
                    previewInteriorApps();
                m_navigation->setTitle(building.name);
                m_status->setText(tr("%1 · Interior").arg(building.name));
                m_flight->setFocus();
                return true;
            }
        }
        returnToOrbit();
        return true;
    }
    if (!path.startsWith("planet:"))
        return false;
    for (const auto &planet : m_spatialWorkspace.planets) {
        if (path != "planet:" + planet.id)
            continue;
        closePanel();
        m_appWindowTimer.stop();
        m_pendingDisplay.clear();
        m_destination = path;
        m_back->setEnabled(true);
        QVector<WorkspaceFlightBuilding> buildings;
        for (const auto &building : planet.buildings) {
            qreal height = 100;
            if (building.style == "tower")
                height = 170;
            else if (building.style == "pavilion")
                height = 80;
            else if (building.style == "plot")
                height = 8;
            WorkspaceFlightBuilding structure{building.name, height, true, false, "building:" + building.id};
            structure.position = building.position;
            structure.positioned = true;
            structure.theme = planet.theme;
            buildings.append(structure);
        }
        m_flight->setCockpitWorld(buildings, false);
        m_flight->setSurfaceSpawn(planet.spawn);
        m_navigation->setTitle(tr("NAVIGATION / %1").arg(planet.name));
        m_status->setText(tr("%1 · Custom workspace · Use Custom workspaces to edit this planet.").arg(planet.name));
        m_flight->setFocus();
        return true;
    }
    returnToOrbit();
    return true;
}

void CockpitWidget::activateDisplay(const QString &id, WorkspaceFlightView *view, bool control)
{
    if (!view)
        view = m_flight;
    if (!view->walking())
        return;
    const auto displays = view->interiorDisplays();
    const SpatialDisplay *selected = view->furnishing()->baseDisplay(id);
    for (const auto &display : displays) {
        if (display.id == id)
            selected = &display;
    }
    if (!selected)
        return;
    if (!(m_preferences.enabled && m_preferences.appControl)) {
        view->setFlightStatus(tr("Enable App control under Settings → Cockpit mode, then select the display to connect."));
        if (!control)
            return;
        m_controls->show();
        m_controls->raise();
        placeHud();
        return;
    }
    QString error;
    const auto windows = appWindowProvider(error);
    if (!error.isEmpty()) {
        view->setFlightStatus(error);
        return;
    }
    QVector<DesktopAppChoice> matches;
    for (const auto &window : windows) {
        if (window.applicationClass.compare(selected->application.windowClass, Qt::CaseInsensitive) == 0)
            matches.append(window);
    }
    if (!control && matches.size() != 1) {
        view->setFlightStatus(tr("Select the screen to launch the app or choose its window."));
        return;
    }
    if (matches.isEmpty()) {
        for (const auto &app : DesktopAppCatalog::applications()) {
            if (app.desktopId != selected->application.desktopId || app.desktopId.isEmpty())
                continue;
            if (QMessageBox::question(this, tr("Launch application?"),
                tr("%1 is not running. Launch it now?").arg(app.name)) == QMessageBox::Yes) {
                m_pendingDisplay = id;
                m_pendingDisplayView = view;
                launchApp(app, false);
            }
            return;
        }
        view->setFlightStatus(tr("%1 is unavailable. Install/open it or edit this display's application reference.").arg(selected->application.name));
        return;
    }
    int index = 0;
    if (matches.size() > 1) {
        QStringList labels;
        for (int number = 0; number < matches.size(); ++number)
            labels.append(QString::number(number + 1) + " · " + matches[number].title);
        bool accepted = false;
        const QString choice = QInputDialog::getItem(this, tr("Choose active application window"),
            tr("Several windows match. Choose the window to control for this room session."), labels, 0, false, &accepted);
        if (!accepted)
            return;
        index = labels.indexOf(choice);
    }
    if (control) {
        m_controls->hide();
        m_navigation->hide();
    }
    view->connectInteriorDisplay(id, matches[index], control);
}

void CockpitWidget::previewInteriorApps()
{
    if (!m_flight->walking() || !(m_preferences.enabled && m_preferences.appControl))
        return;
    QString error;
    const auto windows = appWindowProvider(error);
    if (!error.isEmpty()) {
        m_flight->setFlightStatus(error);
        return;
    }
    for (const auto &display : m_flight->interiorDisplays()) {
        if (display.application.windowClass.isEmpty())
            continue;
        QVector<DesktopAppChoice> matches;
        for (const auto &window : windows) {
            if (window.applicationClass.compare(display.application.windowClass, Qt::CaseInsensitive) == 0)
                matches.append(window);
        }
        // Never launch an app or guess between matching windows merely
        // because the user entered a room. Selection resolves ambiguity.
        if (matches.size() == 1)
            m_flight->connectInteriorDisplay(display.id, matches.first(), false);
    }
}

void CockpitWidget::refreshWorkspaceDestinations()
{
    m_workspaceDestinations->clear();
    m_workspaceDestinations->addItem(tr("Travel to workspace…"));
    for (const auto &planet : m_spatialWorkspace.planets) {
        m_workspaceDestinations->addItem(planet.name, "planet:" + planet.id);
        for (const auto &building : planet.buildings)
            m_workspaceDestinations->addItem(planet.name + " / " + building.name, "building:" + building.id);
    }
}
