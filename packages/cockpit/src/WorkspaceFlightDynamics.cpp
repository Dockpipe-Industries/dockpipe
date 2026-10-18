#include "WorkspaceFlightView.h"
#include <QPushButton>
#include <cmath>

bool WorkspaceFlightView::craftFlying() const
{
    return !m_interior || m_interior->locomotion == "fly";
}

QQuaternion WorkspaceFlightView::craftOrientation() const
{
    if (craftFlying() && m_attitudeActive && !travelling())
        return m_dynamics.orientation();
    if (ridingBike() && !bikeChaseCamera()) {
        const auto &bike = m_bike.state();
        return ArcadeFlightDynamics::attitude(m_yaw, m_pitch + bike.pitch * .55f)
            * QQuaternion::fromAxisAndAngle(0, 0, 1, bike.lean * 28.64789f);
    }
    return ArcadeFlightDynamics::attitude(m_yaw, m_pitch);
}

QQuaternion WorkspaceFlightView::viewOrientation() const
{
    return craftOrientation() * ArcadeFlightDynamics::attitude(m_headYaw, m_headPitch);
}

void WorkspaceFlightView::prepareCraft()
{
    if (m_attitudeActive)
        return;
    m_dynamics.reset(ArcadeFlightDynamics::attitude(m_yaw, m_pitch));
    m_attitudeActive = true;
}

void WorkspaceFlightView::setFlightMode(ArcadeFlightDynamics::Mode mode)
{
    if (m_dynamics.mode() == mode)
        return;
    m_dynamics.setMode(mode);
    updateFlightModeButton();
    if (craftFlying() && isVisible() && !m_editorActive && !travelling() && !m_tick.isActive()) {
        m_elapsed.start();
        m_tick.start();
    }
    viewport()->update();
}

void WorkspaceFlightView::updateFlightModeButton()
{
    if (!m_flightModeButton)
        return;
    const bool explorer = flightMode() == ArcadeFlightDynamics::Mode::Explorer;
    m_flightModeButton->setText(explorer ? tr("Explorer · F") : tr("Pilot · F"));
    m_flightModeButton->setToolTip(tr("Switch flight mode (F)\nExplorer: precise hover; assisted leveling on planets only.\nSpace flight holds your chosen attitude.\nPilot: speed, momentum and full rotation.\nWASD thrust · Q/E vertical · Arrows or right-drag steer\nShift boost · X brake · Z/C roll in space or Pilot · Middle-drag head look\nPilot controller: LB/RB roll · Left-stick click boost"));
    m_flightModeButton->setVisible(craftFlying() && !m_editorActive);
    m_flightModeButton->adjustSize();
    m_flightModeButton->move(qMax(8, width() - m_flightModeButton->width() - 16), 54);
    m_flightModeButton->raise();
}

void WorkspaceFlightView::setPilotFlightControls(float roll, bool brake)
{
    if (bikeCrashed())
        return;
    if ((!craftFlying() && !ridingBike()) || m_editorActive || !isVisible() || travelling())
        return;
    m_pilotRoll = std::isfinite(roll) ? qBound(-1.0f, roll, 1.0f) : 0;
    m_pilotBrake = brake;
    if ((m_pilotRoll != 0 || brake) && !m_tick.isActive()) {
        m_elapsed.start();
        m_tick.start();
    }
}

void WorkspaceFlightView::suspendPilot()
{
    stop();
    viewport()->update();
}
