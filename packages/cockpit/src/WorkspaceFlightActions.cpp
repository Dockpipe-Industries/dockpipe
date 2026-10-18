#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include <cmath>

bool WorkspaceFlightView::activateInteriorAction(const QString &id)
{
    if (!m_interior || m_interior->streaming || m_interior->locomotion != "walk"
        || m_seatReturn || m_droneReturn)
        return false;
    for (const auto &action : m_interior->actions) {
        if (action.id != id || (action.center - m_camera).length() > 340)
            continue;
        const auto direction = (action.center - m_camera).normalized();
        if (SpatialInteriorGeometry::target(*m_interior, m_camera, direction) != "action:" + id)
            return false;
        if (action.kind == SpatialActionKind::Destination) {
            if (destinationActivated)
                destinationActivated("civic:" + action.link, false);
            return true;
        }
        if (action.kind != SpatialActionKind::Seat
            && SpatialInteriorGeometry::blocked(*m_interior, action.destination))
            return false;
        stop();
        if (action.kind == SpatialActionKind::Seat)
            m_seatReturn = m_camera;
        if (action.kind == SpatialActionKind::Drone) {
            const auto boardingPoint = m_camera;
            setInteriorLocomotion("fly");
            m_droneReturn = boardingPoint;
        }
        m_camera = action.destination;
        m_yaw = action.yaw;
        m_pitch = 0;
        if (action.kind == SpatialActionKind::Seat) {
            for (const auto &display : m_interior->displays) {
                if (display.id != "cinema-screen")
                    continue;
                const auto direction = (display.center - m_camera).normalized();
                m_yaw = std::atan2(direction.x(), direction.z());
                m_pitch = std::asin(direction.y());
                break;
            }
        }
        m_attitudeActive = false;
        setFlightStatus(action.kind == SpatialActionKind::Seat
            ? tr("Seated · Look freely · E / A or Back to stand") : action.title);
        viewport()->update();
        return true;
    }
    return false;
}

bool WorkspaceFlightView::leaveInteriorAction()
{
    if (!m_interior || (!m_seatReturn && !m_droneReturn))
        return false;
    const auto position = m_seatReturn ? *m_seatReturn : *m_droneReturn;
    // Return only to the clear boarding/standing point, never through a nearby wall.
    if (SpatialInteriorGeometry::blocked(*m_interior, position)) {
        setFlightStatus(tr("The return position is blocked. Clear it before leaving."));
        return true;
    }
    stop();
    m_seatReturn.reset();
    m_droneReturn.reset();
    setInteriorLocomotion("walk");
    m_camera = position;
    m_pitch = 0;
    setFlightStatus(tr("Walk · Aim at a seat, lift, entrance or drone and press E / A"));
    viewport()->update();
    return true;
}
