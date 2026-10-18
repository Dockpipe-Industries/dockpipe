#include "CockpitWidget.h"
#include "CockpitInputMenu.h"
#include "FurnishingController.h"
#include "FurnishingCatalog.h"
#include "GitReviewWidget.h"
#include "SpatialWebBrowser.h"
#include <QComboBox>

void CockpitWidget::useControllerHints(bool controller)
{
    m_controllerHints = controller;
    m_flight->setControllerHints(controller);
    if (m_web) {
        m_web->scene()->setControllerHints(controller);
        m_web->scene()->setPreviewAutoScroll(m_flight->previewAutoScroll());
    }
    if (m_git) {
        m_git->scene()->setControllerHints(controller);
        m_git->scene()->setPreviewAutoScroll(m_flight->previewAutoScroll());
    }
    if (m_inputMenu)
        m_inputMenu->setControllerHints(controller);
}

void CockpitWidget::openInputMenu(const QString &section)
{
    auto *flight = controlledFlight();
    flight->suspendPilot();
    m_inputSection = section;
    const bool canBuild = flight->isVisible() && flight->walking() && flight->furnishing()
        && flight->furnishing()->available()
        && !flight->editorActive() && flight->activeVehicleId().isEmpty();
    QVector<CockpitMenuEntry> entries;
    QString title = tr("COCKPIT / COMMAND MENU");
    if (section == "navigation") {
        title = tr("GO SOMEWHERE");
        entries.append({"visit:orbit", tr("Orbit"), tr("Return to the solar system.")});
        for (const QString &destination : {QString("files"), QString("apps"), QString("workflows"), QString("containers"),
                 QString("machines"), QString("executions"), QString("web"), QString("git")})
            entries.append({"visit:" + destination, destination.toUpper(), tr("Enter this destination.")});
        for (int index = 0; index < m_workspaceDestinations->count(); ++index) {
            const QString path = m_workspaceDestinations->itemData(index).toString();
            if (!path.isEmpty())
                entries.append({"visit:" + path, m_workspaceDestinations->itemText(index), tr("Enter your saved spatial workspace.")});
        }
    } else if (section == "build") {
        title = tr("BUILD / SPAWN AN ITEM");
        for (const auto &item : FurnishingCatalog::items())
            entries.append({"spawn:" + item.id, item.name,
                item.category + tr(" · Spawn a placement preview. Aim, then A / Enter to place; B / Esc cancels."),
                canBuild, item.id, item.category});
    } else {
        entries.append({"menu:navigation", tr("Navigation"), tr("Planets, websites, Git review and your saved spaces.")});
        entries.append({"menu:build", tr("Build / spawn an item"), canBuild
            ? tr("Choose furniture, screens, wall art and building parts, then place them using the crosshair.")
            : tr("Enter a room and leave the vehicle to build here."), canBuild});
        entries.append({"furnish", flight->furnishingActive() ? tr("Finish furnishing") : tr("Edit room / furnishing"),
            tr("Select, move, rotate, duplicate and configure objects already in this room."), canBuild});
        if (flight->furnishingActive() && !flight->furnishing()->selectedId().isEmpty()) {
            entries.append({"object:move", tr("Move selected object"), tr("Pick it up and aim at a new placement.")});
            entries.append({"object:duplicate", tr("Duplicate selected object"), tr("Place a new copy with the same settings.")});
            entries.append({"object:configure", tr("Configure selected screen / artwork"), tr("Choose this object's content source.")});
            entries.append({"object:delete", tr("Remove selected object"), tr("Remove this object from the room.")});
        }
        if (flight->ridingBike()) {
            if (flight->bikeRaceAvailable())
                entries.append({"bike-race", tr("Start / restart three-lap race"),
                    tr("Race Riley and Alex around the course. Keyboard: G. Three-second countdown.")});
            entries.append({"bike-reset", tr("Recover bike"), tr("Return the bike and rider to the starting position.")});
            if (flight->terrainWeatherAvailable())
                entries.append({"terrain-rain", flight->terrainRaining() ? tr("Stop rain") : tr("Start rain"),
                    tr("Rain softens soil; runoff pools in low spots. Ground stays wet after rain stops. T.")});
            entries.append({"bike-camera", tr("Switch chase / helmet camera"), tr("See your rider or ride from the helmet. V / Y.")});
            entries.append({"bike-sound", flight->bikeSoundEnabled() ? tr("Mute bike sounds") : tr("Enable bike sounds"), tr("Engine, tyres, wind and landing effects. M.")});
        }
        if (flight->hasTexturedTerrain())
            entries.append({"terrain-detail", flight->terrainSharp() ? tr("Resolution: Native → Reduced") : tr("Resolution: Reduced → Native"),
                tr("Native is the default and uses your full display resolution. Reduced renders at up to 1280 × 800 and looks softer.")});
        entries.append({"preview", flight->previewAutoScroll() ? tr("Pause preview scrolling") : tr("Resume preview scrolling"),
            tr("Pause or resume text at its current position. Keyboard: P. Controller: View outside furnishing.")});
        entries.append({"mode", tr("Switch Explorer / Pilot"), tr("Explorer: precision movement. Pilot: momentum and full rotation. F / Y."),
            flight->isVisible() && (!flight->walking() || flight->interiorLocomotion() == "fly")});
        entries.append({"fullscreen", tr("Toggle fullscreen"), tr("Fill the spatial view or restore the window. Keyboard: F11."), flight->isVisible()});
        if (m_web && m_web->isVisible())
            entries.append({"web-toggle", m_web->spatial() ? tr("Use live webpage") : tr("Return to spatial world"), tr("Switch views without rebuilding the world.")});
        if (m_git && m_git->isVisible() && !m_git->spatial())
            entries.append({"git-resume", tr("Enter Git review rooms"), tr("Resume your current file in the spatial review."), !m_git->busy() && !m_git->model().files.isEmpty()});
        entries.append({"back", tr("Back / leave this space"), tr("Return one level from the current activity.")});
    }
    QWidget *surface = flight;
    if (m_web && m_web->isVisible() && !m_web->spatial())
        surface = m_web;
    else if (m_git && m_git->isVisible() && !m_git->spatial())
        surface = m_git;
    m_inputMenu->present(surface, title, entries, m_controllerHints);
}

void CockpitWidget::closeInputMenu(bool restoreFocus)
{
    if (!m_inputMenu || m_inputMenu->isHidden())
        return;
    m_inputMenu->hide();
    m_inputMenu->setParent(m_flight);
    m_controllerNeedsNeutral = true;
    auto *flight = controlledFlight();
    flight->suspendPilot();
    if (restoreFocus && flight->isVisible() && !flight->editorActive())
        flight->setFocus();
}

void CockpitWidget::triggerInputAction(const QString &action)
{
    if (action.startsWith("menu:")) {
        openInputMenu(action.mid(5));
        return;
    }
    auto *flight = controlledFlight();
    closeInputMenu();
    if (action.startsWith("visit:")) {
        const QString destination = action.mid(6);
        if (destination == "orbit")
            returnToOrbit();
        else
            visit(destination);
    } else if (action.startsWith("spawn:")) {
        if (auto *furnishing = flight->furnishing()) {
            if (!furnishing->active())
                furnishing->toggle();
            if (furnishing->active())
                furnishing->selectType(action.mid(6));
        }
    } else if (action == "furnish") {
        if (flight->furnishing())
            flight->furnishing()->toggle();
    } else if (action.startsWith("object:") && flight->furnishingActive()) {
        auto *furnishing = flight->furnishing();
        if (action == "object:move")
            furnishing->moveSelected();
        else if (action == "object:duplicate")
            furnishing->duplicateSelected();
        else if (action == "object:configure")
            furnishing->configure();
        else if (action == "object:delete")
            furnishing->removeSelected();
    } else if (action == "terrain-rain") {
        flight->toggleTerrainRain();
    } else if (action == "bike-camera") {
        flight->toggleBikeCamera();
    } else if (action == "bike-sound") {
        flight->toggleBikeSound();
    } else if (action == "bike-race") {
        flight->startBikeRace();
    } else if (action == "bike-reset") {
        flight->resetBike();
    } else if (action == "terrain-detail") {
        flight->setTerrainSharp(!flight->terrainSharp());
    } else if (action == "preview") {
        const bool enabled = !flight->previewAutoScroll();
        m_flight->setPreviewAutoScroll(enabled);
        if (m_git)
            m_git->scene()->setPreviewAutoScroll(enabled);
        if (m_web)
            m_web->scene()->setPreviewAutoScroll(enabled);
    } else if (action == "mode") {
        if (!flight->editorActive() && (!flight->walking() || flight->interiorLocomotion() == "fly"))
            flight->setFlightMode(flight->flightMode() == ArcadeFlightDynamics::Mode::Explorer
                ? ArcadeFlightDynamics::Mode::Pilot : ArcadeFlightDynamics::Mode::Explorer);
    } else if (action == "fullscreen") {
        flight->setFlightFullscreen(!flight->flightFullscreen());
    } else if (action == "git-resume") {
        if (m_git && m_git->isVisible())
            m_git->beginReview();
    } else if (action == "web-toggle") {
        if (m_web && m_web->isVisible()) {
            if (m_web->spatial())
                m_web->showPage();
            else
                m_web->showSpace();
        }
    } else if (action == "back") {
        if (flight->appControlling() || flight->seated()) {
            flight->goBack();
        } else if (m_git && m_git->isVisible()) {
            m_git->goBack();
        } else if (m_web && m_web->isVisible()) {
            if (!m_web->scene()->activeVehicleId().isEmpty())
                m_web->scene()->exitVehicle();
            else if (m_web->spatial())
                m_web->showPage();
            else
                returnToOrbit();
        } else if (m_content || m_flight->editorActive()) {
            closePanel();
            m_flight->stopAppBillboards();
        } else {
            m_flight->goBack();
        }
    }
    useControllerHints(m_controllerHints);
}

void WorkspaceFlightView::setControllerHints(bool controller)
{
    if (m_controllerHints == controller)
        return;
    m_controllerHints = controller;
    if (m_furnishing)
        m_furnishing->refreshInputHints();
    viewport()->update();
}

QString WorkspaceFlightView::inputHints() const
{
    if (ridingBike())
        return m_controllerHints ? tr("RT: throttle · LT: brake · X: rear brake · Left stick back + RT: wheelie · Left stick X: steer · Right stick: look (2s return) · Y: camera · Start: menu")
                                 : tr("W/S: throttle/brake · A/D: steer · ↓: pull back / wheelie · ↑: lean forward · Right-drag: look (2s return) · Space/X: rear brake · V: camera · M: sound · R: recover · G: race");
    if (m_seatReturn)
        return m_controllerHints ? tr("Right stick: look · A / B: stand · Start: menu")
                                 : tr("Right-drag/arrows: look · E / Esc: stand · F1: menu");
    if (m_droneReturn)
        return m_controllerHints ? tr("Sticks: fly/look · Climb to orbit · A / B: return to pad · Start: menu")
                                 : tr("WASD: fly · Q/E: altitude · Climb to orbit · Esc: return to pad · F: pilot mode");
    if (m_controllerHints) {
        if (furnishingActive())
            return tr("Start: menu · A: place/select · B: cancel · D-pad: browse/rotate · LB/RB: category · Y: configure");
        if (!activeVehicleId().isEmpty())
            return tr("Start: menu · Left stick: drive · X: brake · B: exit vehicle");
        if (walking() && interiorLocomotion() != "fly")
            return tr("Start: menu · Left stick: walk · Right stick: look · A: interact · B: back · View: preview scroll");
        if (flightMode() == ArcadeFlightDynamics::Mode::Pilot)
            return tr("Start: menu · Sticks: fly/look · LB/RB: roll · L3: boost · X: brake · Y: mode · A: interact");
        return tr("Start: menu · Sticks: fly/look · Triggers: altitude · LB: boost · Y: mode · A: interact · View: preview scroll");
    }
    if (furnishingActive())
        return tr("F1: menu · Enter: place/select · Esc: cancel · ↑/↓: browse · ←/→: rotate · I: configure");
    if (!activeVehicleId().isEmpty())
        return tr("F1: menu · W/S: throttle · A/D: steer · Space: brake · E: exit vehicle");
    if (walking() && interiorLocomotion() != "fly")
        return tr("F1: menu · WASD: walk · Right-drag: look · E: interact · Esc: back · P: preview scroll");
    return tr("F1: menu · WASD: fly · Q/E: altitude · Arrows/right-drag: steer · Shift: boost · Z/C: roll · F: mode · P: preview scroll");
}
