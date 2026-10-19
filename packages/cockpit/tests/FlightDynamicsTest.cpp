#include "ArcadeFlightDynamics.h"
#include "WorkspaceFlightView.h"
#include "FlightProjection.h"
#include <QFocusEvent>
#include <QApplication>
#include <QElapsedTimer>
#include <QKeyEvent>
#include <QMouseEvent>
#include <QPushButton>
#include <QThread>
#include <cmath>
#include <cstdio>
#include <limits>

#define VERIFY_FLIGHT(value) do { if (!(value)) { std::fprintf(stderr, "Flight assertion failed at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void waitFrames(int milliseconds)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}

QVector3D simulate(ArcadeFlightDynamics &craft, const ArcadeFlightDynamics::Input &input, int frames, double seconds,
    bool orbit = false)
{
    QVector3D distance;
    for (int frame = 0; frame < frames; ++frame)
        distance += craft.advance(input, seconds, orbit);
    return distance;
}
}

int flightDynamicsTest()
{
    using Mode = ArcadeFlightDynamics::Mode;
    ArcadeFlightDynamics slowFrames;
    ArcadeFlightDynamics fastFrames;
    ArcadeFlightDynamics::Input thrust;
    thrust.thrust = {0, 0, 1};
    const auto slowDistance = simulate(slowFrames, thrust, 60, 1.0 / 30);
    const auto fastDistance = simulate(fastFrames, thrust, 288, 1.0 / 144);
    VERIFY_FLIGHT((slowDistance - fastDistance).length() < 0.1);
    VERIFY_FLIGHT((slowFrames.velocity() - fastFrames.velocity()).length() < 0.1);
    VERIFY_FLIGHT(slowDistance.z() > 450 && slowDistance.z() < 520);
    const float explorerSpeed = slowFrames.velocity().length();
    simulate(slowFrames, {}, 120, 1.0 / 60);
    VERIFY_FLIGHT(slowFrames.velocity().isNull() && !slowFrames.unsettled());

    // Space has no horizon: releasing controls must preserve pitch and bank.
    for (const Mode mode : {Mode::Explorer, Mode::Pilot}) {
        ArcadeFlightDynamics space;
        space.setMode(mode);
        space.reset(ArcadeFlightDynamics::attitude(0.4f, -0.7f, 0.3f));
        const auto initial = space.orientation();
        simulate(space, {}, 240, 1.0 / 60, true);
        VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(initial, space.orientation())) > 0.99999f);
        VERIFY_FLIGHT(!space.unsettled());

        space.steer(0.4f, 0.2f);
        simulate(space, {}, 120, 1.0 / 60, true);
        const auto steered = space.orientation();
        VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(initial, steered)) < 0.99f);
        simulate(space, thrust, 60, 1.0 / 60, true);
        simulate(space, {}, 900, 1.0 / 60, true);
        VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(steered, space.orientation())) > 0.99999f);
        VERIFY_FLIGHT(!space.unsettled());

        ArcadeFlightDynamics::Input spaceTurn;
        space.reset({});
        spaceTurn.turn = {1, 0, 0};
        simulate(space, spaceTurn, 120, 1.0 / 60, true);
        VERIFY_FLIGHT(space.orientation().rotatedVector({0, 1, 0}).y() < 0);
        simulate(space, {}, 120, 1.0 / 60, true);
        const auto turned = space.orientation();
        simulate(space, {}, 240, 1.0 / 60, true);
        VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(turned, space.orientation())) > 0.99999f);
        VERIFY_FLIGHT(!space.unsettled());
    }

    ArcadeFlightDynamics pilot;
    pilot.setMode(Mode::Pilot);
    simulate(pilot, thrust, 120, 1.0 / 60);
    VERIFY_FLIGHT(pilot.velocity().length() > explorerSpeed * 1.9);
    const auto coasting = simulate(pilot, {}, 30, 1.0 / 60);
    VERIFY_FLIGHT(coasting.z() > 200 && pilot.velocity().z() > 300);
    ArcadeFlightDynamics::Input brake;
    brake.brake = true;
    simulate(pilot, brake, 45, 1.0 / 60);
    VERIFY_FLIGHT(pilot.velocity().isNull());
    thrust.boost = true;
    simulate(pilot, thrust, 120, 1.0 / 60);
    VERIFY_FLIGHT(pilot.velocity().length() > 1300 && pilot.velocity().length() <= 1351);
    const auto capped = pilot.advance(thrust, 100, true);
    VERIFY_FLIGHT(capped.length() < 203);
    VERIFY_FLIGHT(pilot.advance(thrust, std::numeric_limits<double>::quiet_NaN(), true).isNull());

    pilot.reset({});
    ArcadeFlightDynamics::Input turn;
    turn.turn = {1, 0, 0};
    simulate(pilot, turn, 110, 1.0 / 60);
    VERIFY_FLIGHT(pilot.orientation().rotatedVector({0, 1, 0}).y() < -0.9);
    turn.turn = {0, 0, 1};
    simulate(pilot, turn, 210, 1.0 / 60);
    VERIFY_FLIGHT(std::abs(pilot.orientation().length() - 1) < 0.0001);
    pilot.setMode(Mode::Explorer);
    simulate(pilot, {}, 240, 1.0 / 60);
    VERIFY_FLIGHT(pilot.orientation().rotatedVector({0, 1, 0}).y() > 0.999);
    VERIFY_FLIGHT(!pilot.unsettled());
    turn.turn = {0, 1, 0};
    simulate(pilot, turn, 60, 1.0 / 60);
    VERIFY_FLIGHT(pilot.orientation().rotatedVector({1, 0, 0}).y() < -0.15);
    pilot.halt();
    VERIFY_FLIGHT(!pilot.unsettled() && pilot.velocity().isNull());

    WorkspaceFlightView flight({}, false);
    flight.resize(1440, 900);
    flight.show();
    SpatialInterior room;
    room.size = {10000, 10000};
    room.height = 1200;
    room.locomotion = "fly";
    room.exit = {0, -4900};
    room.solids.append({{0, 300, 0}, {4000, 600, 2}, QColor("#526777")});
    flight.setInterior(room, "Collision fixture");
    auto *mode = flight.findChild<QPushButton *>("cockpitFlightMode");
    VERIFY_FLIGHT(mode && mode->isVisible());
    mode->click();
    VERIFY_FLIGHT(flight.flightMode() == Mode::Pilot);
    QKeyEvent switchMode(QEvent::KeyPress, Qt::Key_F, Qt::NoModifier);
    QApplication::sendEvent(&flight, &switchMode);
    VERIFY_FLIGHT(flight.flightMode() == Mode::Explorer);
    QApplication::sendEvent(&flight, &switchMode);
    VERIFY_FLIGHT(flight.flightMode() == Mode::Pilot);
    flight.setPilotInput({0, 0, 1}, {}, true);
    waitFrames(1300);
    VERIFY_FLIGHT(flight.cameraPosition().z() < -25 && flight.cameraPosition().z() > -50);
    VERIFY_FLIGHT(flight.flightVelocity().z() == 0);
    flight.setPilotInput({1, 0, 0}, {}, true);
    waitFrames(150);
    flight.setPilotInput({}, {}, false);
    VERIFY_FLIGHT(flight.moving());
    const auto beforeCoast = flight.cameraPosition();
    waitFrames(100);
    VERIFY_FLIGHT(flight.cameraPosition().x() > beforeCoast.x());
    flight.setPilotFlightControls(0, true);
    waitFrames(700);
    VERIFY_FLIGHT(flight.flightVelocity().isNull());
    flight.setPilotFlightControls(0, false);
    waitFrames(50);
    VERIFY_FLIGHT(!flight.moving());
    const auto beforeKeyboardRoll = flight.craftOrientation();
    QKeyEvent rollKey(QEvent::KeyPress, Qt::Key_C, Qt::NoModifier);
    QApplication::sendEvent(&flight, &rollKey);
    waitFrames(300);
    VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(beforeKeyboardRoll, flight.craftOrientation())) < 0.99f);
    QKeyEvent releaseRollKey(QEvent::KeyRelease, Qt::Key_C, Qt::NoModifier);
    QApplication::sendEvent(&flight, &releaseRollKey);
    flight.hide();
    const auto parked = flight.cameraPosition();
    const auto attitude = flight.craftOrientation();
    waitFrames(150);
    VERIFY_FLIGHT(!flight.moving() && flight.flightVelocity().isNull());
    VERIFY_FLIGHT(flight.cameraPosition() == parked && flight.craftOrientation() == attitude);
    flight.show();
    flight.setInteriorLocomotion("walk");
    VERIFY_FLIGHT(!mode->isVisible());
    flight.setInteriorLocomotion("fly");
    VERIFY_FLIGHT(mode->isVisible());

    room.solids.clear();
    SpatialDisplay target;
    target.id = "rolled-target";
    target.freestanding = true;
    target.center = {200, 240, 800};
    target.size = {120, 90};
    room.displays.append(target);
    flight.setInterior(room, "Rolled interaction fixture");
    flight.setPilotFlightControls(1, false);
    waitFrames(600);
    flight.suspendPilot();
    const auto relative = flight.craftOrientation().conjugated().rotatedVector(target.center - flight.cameraPosition());
    const qreal focal = FlightProjection::focalLength(flight.viewport()->size());
    const QPointF projected(flight.viewport()->width() / 2.0 + relative.x() * focal / relative.z(),
        flight.viewport()->height() / 2.0 - relative.y() * focal / relative.z());
    QString selected;
    flight.destinationActivated = [&](const QString &id, bool) { selected = id; };
    QMouseEvent select(QEvent::MouseButtonPress, projected, projected, Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &select);
    VERIFY_FLIGHT(selected == "display:rolled-target");
    flight.setPilotInput({0, 0, 1}, {}, true);
    waitFrames(100);
    QFocusEvent unfocused(QEvent::FocusOut);
    QApplication::sendEvent(&flight, &unfocused);
    VERIFY_FLIGHT(!flight.moving() && flight.flightVelocity().isNull());

    QVector<WorkspaceFlightBuilding> worlds;
    for (const QString &name : {"Files", "Apps", "Workflows", "Containers", "Machines", "Executions", "Web"})
        worlds.append({name, 80, true, false, name.toLower()});
    flight.setCockpitWorld(worlds, true);
    flight.setOrbitMotion(false);
    flight.setFlightMode(Mode::Explorer);
    flight.suspendPilot();
    waitFrames(100);
    const auto beforeSteering = flight.craftOrientation();
    QMouseEvent steerPress(QEvent::MouseButtonPress, QPointF(500, 400), QPointF(500, 400),
        Qt::RightButton, Qt::RightButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &steerPress);
    QMouseEvent steerMove(QEvent::MouseMove, QPointF(550, 300), QPointF(550, 300),
        Qt::NoButton, Qt::RightButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &steerMove);
    QMouseEvent steerRelease(QEvent::MouseButtonRelease, QPointF(550, 300), QPointF(550, 300),
        Qt::RightButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &steerRelease);
    waitFrames(900);
    const auto afterSteering = flight.craftOrientation();
    VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(beforeSteering, afterSteering)) < 0.99f);
    VERIFY_FLIGHT(afterSteering.rotatedVector({0, 0, 1}).y() < -0.15f);
    waitFrames(900);
    VERIFY_FLIGHT(std::abs(QQuaternion::dotProduct(afterSteering, flight.craftOrientation())) > 0.99999f);
    VERIFY_FLIGHT(!flight.moving());
    VERIFY_FLIGHT(flight.grab().save("/tmp/cockpit-arcade-orbit.png"));
    // Head look moves the view independently without turning or thrusting the hull.
    const auto hull = flight.craftOrientation();
    const auto origin = flight.cameraPosition();
    QMouseEvent press(QEvent::MouseButtonPress, QPointF(500, 400), QPointF(500, 400),
        Qt::MiddleButton, Qt::MiddleButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &press);
    QMouseEvent look(QEvent::MouseMove, QPointF(600, 440), QPointF(600, 440),
        Qt::NoButton, Qt::MiddleButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &look);
    waitFrames(50);
    VERIFY_FLIGHT(flight.craftOrientation() == hull && flight.cameraPosition() == origin);
    VERIFY_FLIGHT(flight.grab().save("/tmp/cockpit-arcade-headlook.png"));
    QMouseEvent release(QEvent::MouseButtonRelease, QPointF(600, 440), QPointF(600, 440),
        Qt::MiddleButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(flight.viewport(), &release);
    flight.resize(900, 650);
    waitFrames(50);
    VERIFY_FLIGHT(flight.grab().save("/tmp/cockpit-arcade-compact.png"));
    const auto frames = flight.renderedFrames();
    waitFrames(100);
    VERIFY_FLIGHT(flight.renderedFrames() == frames);
    std::puts("Arcade flight: frame-rate independence, hover, coast, brake, boost, loops, planetary leveling, space attitude retention, thin-wall collision, lifecycle, head look and idle rendering passed.");
    return 0;
}
