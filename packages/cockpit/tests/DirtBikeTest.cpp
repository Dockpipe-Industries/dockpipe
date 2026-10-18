#include "DirtBikeDynamics.h"
#include "DirtBikeTyre.h"
#include "DirtBikeTyreContact.h"
#include "DirtBikeSuspension.h"
#include "DirtBikeModel.h"
#include "SpatialMeshLoft.h"
#include "SpatialSunShadow.h"
#include "DirtBikeRiderPose.h"
#include "DirtBikeSteeringRig.h"
#include "DirtBikeCrashPose.h"
#include "DirtBikeCameraBoom.h"
#include "FurnishingController.h"
#include "DirtBikeSound.h"
#include "FlightProjection.h"
#include "CockpitController.h"
#include <array>
#include "SpatialBikeTrack.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialFoliagePlacement.h"
#include "SpatialWebWorld.h"
#include "SpatialWebPassage.h"
#include "SpatialWebBrowser.h"
#include <QPushButton>
#include <QOpenGLWidget>
#include <QOpenGLFramebufferObject>
#include <QOpenGLPaintDevice>
#include <QOpenGLFunctions>
#include <QPainter>
#include "WorkspaceFlightView.h"
#include <QApplication>
#include <QElapsedTimer>
#include <QThread>
#include <QKeyEvent>
#include <limits>
#include <cmath>
#include <cstdio>

#define CHECK_BIKE(value) do { if (!(value)) { std::fprintf(stderr, "Bike assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void idle(int milliseconds)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
}
void ride(DirtBikeDynamics &bike, const DirtBikeDynamics::Input &input, const SpatialInterior &room, int frames, float dt = 1.f / 60)
{
    for (int frame = 0; frame < frames; ++frame)
        bike.advance(input, dt, room);
}
}

int dirtBikeRaceTest();

int dirtBikeTest()
{
    if (dirtBikeRaceTest() != 0)
        return 1;
    // Dust follows the rear contact patch, not a chassis-height emitter. It
    // remains bounded through stalls, wheelies, flight and room changes.
    {
        SpatialInterior dustRoom;
        dustRoom.size = {100000, 100000};
        SpatialRideSurface platform;
        platform.width = 1200;
        platform.profile = {{-2000, 0}, {-1000, 200}, {1000, 200}, {2000, 0}};
        CHECK_BIKE(platform.valid());
        dustRoom.rideSurfaces.append(platform);
        DirtBikeDust dust;
        DirtBikeDynamics::State contact;
        contact.position = {0, 225, 0};
        contact.speed = 1000;
        contact.pitch = .5f;
        contact.heading = .8f;
        contact.rearSupport = 0;
        for (int frame = 0; frame < 10; ++frame)
            dust.advance(contact, .02f, dustRoom);
        CHECK_BIKE(dust.isEmpty());
        contact.rearSupport = .5f;
        for (int frame = 0; frame < 3; ++frame)
            dust.advance(contact, .02f, dustRoom);
        CHECK_BIKE(dust.particles().size() == 1);
        auto expected = contact.position - SpatialVehicleModel::rotate({0, 0, 1}, contact.heading)
            * (74 * std::cos(contact.pitch));
        expected.setY(210);
        CHECK_BIKE((dust.particles().first().position - expected).length() < .001f);
        const float life = dust.particles().first().life;
        dust.advance(contact, std::numeric_limits<float>::quiet_NaN(), dustRoom);
        CHECK_BIKE(dust.particles().first().life == life);
        contact.grounded = false;
        dust.advance(contact, 2, dustRoom);
        CHECK_BIKE(dust.isEmpty());
        contact.grounded = true;
        contact.crashed = true;
        contact.rearSupport = 0;
        dust.advance(contact, .03f, dustRoom);
        dust.advance(contact, .03f, dustRoom);
        CHECK_BIKE(dust.particles().size() == 1);
        CHECK_BIKE(dust.particles().first().position == QVector3D(0, 210, 0));
        for (int frame = 0; frame < 2000; ++frame) {
            dust.advance(contact, .01f, dustRoom);
            CHECK_BIKE(dust.particles().size() <= DirtBikeDust::maximumParticles);
        }
        dust.advance(contact, 60, dustRoom);
        CHECK_BIKE(dust.particles().size() <= 1); // No missed-emission replay.
        contact.speed = 0;
        dust.advance(contact, 2, dustRoom);
        CHECK_BIKE(dust.isEmpty());
        contact.speed = 1000;
        dust.advance(contact, .04f, dustRoom);
        dust.clear();
        dust.advance(contact, .02f, dustRoom);
        CHECK_BIKE(dust.isEmpty()); // Clear also resets the emission clock.
    }

    // Uncompressed visual axles follow the solver's projected wheelbase at
    // every attitude. Roll must not move the axle midpoint off its centreline.
    // This isolates chassis geometry from steering and suspension articulation.
    for (float pitch : {-3.14159265f, -1.57f, -.7f, 0.f, .7f, 1.57f, 3.14159265f}) {
        for (float lean : {-1.2f, -.5f, 0.f, .5f, 1.2f}) {
            const auto attitude = ArcadeFlightDynamics::attitude(0, pitch, lean);
            for (float axleZ : {-74.f, 74.f}) {
                const auto axle = DirtBikeModel::chassisPoint({0, 35, axleZ}, attitude);
                const QVector3D expected(0, 35 + axleZ * std::sin(pitch), axleZ * std::cos(pitch));
                CHECK_BIKE((axle - expected).length() < .001f);
            }
            const auto midpoint = DirtBikeModel::chassisPoint({0, 35, 0}, attitude);
            CHECK_BIKE((midpoint - QVector3D(0, 35, 0)).length() < .001f);
            const QVector3D bodyPoint(12, 98, -20);
            const auto tilted = DirtBikeModel::chassisPoint(bodyPoint, attitude);
            CHECK_BIKE(std::abs((tilted - midpoint).length()
                - (bodyPoint - QVector3D(0, 35, 0)).length()) < .001f);
        }
    }

    // Circle clearance above a plane has an analytic answer for every slope.
    // The fixed footprint must preserve flat ground and be symmetric in travel.
    for (float slope : {-2.f, -.7f, -.1f, 0.f, .1f, .7f, 2.f}) {
        int samples = 0;
        const float support = DirtBikeTyreContact::supportHeight([&](float offset) {
            ++samples;
            return 100 + slope * offset;
        });
        const float expected = 100 + 35 * (std::hypot(1.f, slope) - 1);
        CHECK_BIKE(std::abs(support - expected) < .001f);
        CHECK_BIKE(samples == 5);
    }
    CHECK_BIKE(DirtBikeTyreContact::supportHeight([](float) {
        return std::numeric_limits<float>::quiet_NaN();
    }) == 0);
    // Roll toward a crest with a slope break exactly represented by the taps.
    // An independent dense circle scan bounds the piecewise analytic solution.
    const auto crest = [](float offset) { return 100 - std::abs(offset - 17.5f) * .7f; };
    float scannedSupport = 0;
    for (int sample = 0; sample <= 7000; ++sample) {
        const float offset = -35 + sample * .01f;
        scannedSupport = qMax(scannedSupport,
            crest(offset) + std::sqrt(qMax(0.f, 35 * 35 - offset * offset)) - 35);
    }
    CHECK_BIKE(std::abs(DirtBikeTyreContact::supportHeight(crest) - scannedSupport) < .001f);

    // A ramp foot starts lifting the rolling tyre before its centre reaches
    // the ramp. Sample interpolation is conservative at this convex seam;
    // bound its error and continuity instead of claiming exact mesh contact.
    float previousSupport = 0;
    for (int step = 0; step <= 560; ++step) {
        const float position = -70 + step * .25f;
        const float support = DirtBikeTyreContact::supportHeight([&](float offset) {
            return qMax(0.f, .7f * (position + offset));
        });
        const float exact = qMax(0.f, .7f * position + 35 * (std::hypot(1.f, .7f) - 1));
        CHECK_BIKE(support >= exact - .001f && support <= exact + 3.1f);
        CHECK_BIKE(support >= previousSupport - .001f && support - previousSupport < .176f);
        if (position == -5)
            CHECK_BIKE(support > 4);
        previousSupport = support;
    }

    // A steered front tyre crosses a side slope along its wheel direction.
    // The rear still rolls along the chassis heading. Circle support on this
    // planar fixture has an analytic answer independent of the implementation.
    {
        SpatialInterior slopeRoom;
        slopeRoom.size = {10000, 10000};
        SpatialRideSurface slope;
        slope.width = 1200;
        slope.profile = {{-1000, 0}, {-200, 0}, {200, 600}, {1000, 0}};
        slopeRoom.rideSurfaces.append(slope);
        CHECK_BIKE(slope.valid());
        for (float heading : {-1.5707963f, 1.5707963f}) {
            for (float steering : {-1.f, 1.f}) {
                SpatialVehicle crossSlope;
                crossSlope.id = "steered-contact";
                crossSlope.body = "dirt-bike";
                crossSlope.position = {0, 300, 0};
                crossSlope.heading = heading;
                DirtBikeDynamics tyre;
                tyre.reset(crossSlope);
                tyre.advance({0, steering, 0, false}, 1.f / 120, slopeRoom);
                // Rodrigues projection for a 26-degree raked head axis,
                // independent of the rig helper used by the solver.
                const float angle = tyre.state().steeringAngle;
                const float rake = 26.f / 57.2957795f;
                const float rollingYaw = std::atan2(std::cos(rake) * std::sin(angle),
                    std::cos(angle) + std::sin(rake) * std::sin(rake) * (1 - std::cos(angle)));
                const float rollingSlope = 1.5f * std::cos(heading + rollingYaw);
                const float expected = 35 * (std::hypot(1.f, rollingSlope) - 1);
                CHECK_BIKE(expected > .07f);
                CHECK_BIKE(std::abs(tyre.state().frontCompression - expected) < .002f);
                CHECK_BIKE(std::abs(tyre.state().rearCompression) < .002f);
                CHECK_BIKE(!tyre.state().crashed);
            }
        }
    }

    // All four limbs preserve their lengths and joint continuity through the
    // input envelope. Hands follow the grips and boots remain on the footpegs.
    for (float steering : {-.6f, 0.f, .6f}) {
        for (float lean : {-1.2f, 0.f, 1.2f}) {
            for (float weight : {-1.f, 0.f, 1.f}) {
                for (float compression : {-8.f, 0.f, 12.f}) {
                    const DirtBikeRiderPose pose(steering, lean, weight, compression);
                    for (int side = 0; side < 2; ++side) {
                        const auto shoulder = DirtBikeRiderPose::shoulder(side);
                        const auto elbow = DirtBikeRiderPose::elbow(side);
                        const auto grip = DirtBikeRiderPose::grip(side);
                        const int upperArm = side ? DirtBikeModel::RightUpperArm : DirtBikeModel::LeftUpperArm;
                        const int forearm = side ? DirtBikeModel::RightForearm : DirtBikeModel::LeftForearm;
                        const int hand = side ? DirtBikeModel::RightHand : DirtBikeModel::LeftHand;
                        const auto shoulderPoint = pose.point(upperArm, shoulder);
                        const auto elbowPoint = pose.point(upperArm, elbow);
                        const auto handPoint = pose.point(hand, grip);
                        CHECK_BIKE((shoulderPoint - pose.bodyPoint(shoulder)).length() < .001f);
                        CHECK_BIKE((elbowPoint - pose.point(forearm, elbow)).length() < .01f);
                        CHECK_BIKE((handPoint - pose.point(forearm, grip)).length() < .01f);
                        CHECK_BIKE((handPoint - DirtBikeModel::steerPoint(grip, steering)).length() < .001f);
                        CHECK_BIKE(std::abs((elbowPoint - shoulderPoint).length() - (elbow - shoulder).length()) < .01f);
                        CHECK_BIKE(std::abs((handPoint - elbowPoint).length() - (grip - elbow).length()) < .01f);
                        CHECK_BIKE((pose.bodyPoint(shoulder) - shoulder).length() < 40);
                        CHECK_BIKE(std::abs(pose.rotation(forearm).length() - 1) < .001f);
                        const auto hip = DirtBikeRiderPose::hip(side);
                        const auto knee = DirtBikeRiderPose::knee(side);
                        const auto ankle = DirtBikeRiderPose::ankle(side);
                        const int thigh = side ? DirtBikeModel::RightThigh : DirtBikeModel::LeftThigh;
                        const int shin = side ? DirtBikeModel::RightShin : DirtBikeModel::LeftShin;
                        const int boot = side ? DirtBikeModel::RightBoot : DirtBikeModel::LeftBoot;
                        const auto hipPoint = pose.point(thigh, hip);
                        const auto kneePoint = pose.point(thigh, knee);
                        const auto anklePoint = pose.point(shin, ankle);
                        CHECK_BIKE((hipPoint - pose.bodyPoint(hip)).length() < .001f);
                        CHECK_BIKE((kneePoint - pose.point(shin, knee)).length() < .01f);
                        CHECK_BIKE((anklePoint - ankle).length() < .01f);
                        CHECK_BIKE((pose.point(boot, ankle) - ankle).length() < .001f);
                        CHECK_BIKE(std::abs((kneePoint - hipPoint).length() - (knee - hip).length()) < .01f);
                        CHECK_BIKE(std::abs((anklePoint - kneePoint).length() - (ankle - knee).length()) < .01f);
                        CHECK_BIKE(std::abs(pose.rotation(shin).length() - 1) < .001f);
                        CHECK_BIKE(kneePoint.z() > hipPoint.z()); // Knees bend forward, not backward.
                    }
                    const QVector3D chest(0, 135, -25);
                    const QVector3D tangent(0, 1, 0);
                    const auto transformedTangent = pose.bodyPoint(chest + tangent) - pose.bodyPoint(chest);
                    const auto transformedNormal = pose.bodyNormal(chest, {0, 0, -1});
                    CHECK_BIKE(std::abs(QVector3D::dotProduct(transformedTangent, transformedNormal)) < .001f);
                }
            }
        }
    }
    const DirtBikeRiderPose invalidPose(std::numeric_limits<float>::infinity(),
        std::numeric_limits<float>::quiet_NaN(), 0, 0);
    CHECK_BIKE(std::isfinite(invalidPose.point(DirtBikeModel::LeftHand, {}).x()));
    CHECK_BIKE(std::isfinite(invalidPose.point(DirtBikeModel::RightShin, {}).y()));
    for (float pitch : {-3.14159265f, -2.f, -1.5707963f, -.6f, 0.f, .6f, 1.5707963f, 2.f, 3.14159265f, 6.2831853f}) {
        const float lever = DirtBikeSuspension::pitchLever(pitch);
        const double epsilon = .0001;
        const double heightDerivative = (std::sin(double(pitch) + epsilon) - std::sin(double(pitch) - epsilon))
            * DirtBikeSuspension::halfWheelbase / (2 * epsilon);
        CHECK_BIKE(std::abs(lever - heightDerivative) < .01);
        for (float rate : {-3.f, -.5f, .5f, 3.f}) {
            const float frontVelocity = rate * lever;
            const float rearVelocity = -frontVelocity;
            const float frontForce = DirtBikeSuspension::support(0, frontVelocity);
            const float rearForce = DirtBikeSuspension::support(0, rearVelocity);
            const float staticForce = DirtBikeSuspension::gravity * .5f;
            const float dampingPower = (frontForce - staticForce) * frontVelocity
                + (rearForce - staticForce) * rearVelocity;
            const float suspensionTorque = (frontForce - rearForce) * lever;
            CHECK_BIKE(dampingPower <= .001f);
            CHECK_BIKE(suspensionTorque * rate <= .001f);
            CHECK_BIKE(std::abs(dampingPower - suspensionTorque * rate) < .05f);
        }
    }
    CHECK_BIKE(DirtBikeSuspension::support(-25, -10000) == 0);
    CHECK_BIKE(DirtBikeSuspension::support(1000, -10000) == 7000);
    CHECK_BIKE(DirtBikeSuspension::support(std::numeric_limits<float>::quiet_NaN(), 0) == 0);
    CHECK_BIKE(DirtBikeSuspension::support(0, std::numeric_limits<float>::infinity()) == 0);
    for (float steering : {-.48f, 0.f, .48f}) {
        const DirtBikeTyre::SteeringFrame frame(steering);
        const auto velocity = frame.toWheel(1000, 120);
        const auto force = frame.toChassis(-400, -250);
        const float wheelPower = QVector2D::dotProduct(velocity, {-400, -250});
        const float chassisPower = QVector2D::dotProduct({1000, 120}, force);
        CHECK_BIKE(std::abs(wheelPower - chassisPower) < .1f);
        const auto rolling = frame.toWheel(1000 * std::cos(steering), 1000 * std::sin(steering));
        CHECK_BIKE(std::abs(rolling.x() - 1000) < .001f && std::abs(rolling.y()) < .001f);
        const auto brakeForce = frame.toChassis(-400, 0);
        CHECK_BIKE(brakeForce.x() < 0 && brakeForce.y() * steering <= 0);
        float slip = 0;
        const auto straightVelocity = frame.toWheel(1000, 0);
        float demand = 0;
        for (int step = 0; step < 60; ++step)
            demand = DirtBikeTyre::lateralDemand(straightVelocity.y(), straightVelocity.x(), 500, 1.f / 120, slip);
        const auto cornerForce = frame.toChassis(0, demand);
        CHECK_BIKE(cornerForce.x() <= 0 && cornerForce.y() * steering >= 0);
    }
    for (float velocity : {-1500.f, -1.f, 0.f, 1.f, 1500.f}) {
        for (float response : {1.f, 1.2f}) {
            constexpr float dt = 1.f / 120;
            const float demand = DirtBikeTyre::brakingDemand(velocity, 700, dt, response);
            const float after = velocity + demand * dt * response;
            CHECK_BIKE(demand * velocity <= 0 && std::abs(demand) <= 700);
            CHECK_BIKE(std::abs(after) <= std::abs(velocity) + .0001f);
            CHECK_BIKE(after * velocity >= -.0001f);
        }
    }
    CHECK_BIKE(DirtBikeTyre::brakingDemand(0, 700, 1.f / 120, 1) == 0);
    CHECK_BIKE(DirtBikeTyre::brakingDemand(100, 700, 0, 1) == 0);
    CHECK_BIKE(DirtBikeTyre::brakingDemand(100, 700, 1.f / 120, 0) == 0);
    CHECK_BIKE(DirtBikeTyre::brakingDemand(std::numeric_limits<float>::quiet_NaN(), 700, 1.f / 120, 1) == 0);
    float relaxedSlip = 0;
    const float initialDemand = DirtBikeTyre::lateralDemand(100, 1500, 500, 1.f / 120, relaxedSlip);
    float settledDemand = 0;
    for (int step = 0; step < 60; ++step)
        settledDemand = DirtBikeTyre::lateralDemand(100, 1500, 500, 1.f / 120, relaxedSlip);
    CHECK_BIKE(initialDemand < 0 && settledDemand < initialDemand && std::isfinite(settledDemand));
    float mirroredSlip = 0;
    float mirroredForce = 0;
    for (int step = 0; step < 61; ++step)
        mirroredForce = DirtBikeTyre::lateralDemand(-100, 1500, 500, 1.f / 120, mirroredSlip);
    CHECK_BIKE(std::abs(mirroredForce + settledDemand) < .001f);
    CHECK_BIKE(DirtBikeTyre::lateralDemand(100, 0, 0, 1.f / 120, relaxedSlip) == 0 && relaxedSlip == 0);
    for (int step = 0; step < 60; ++step) {
        const float force = DirtBikeTyre::lateralDemand(5000, 0, 500, 1.f / 120, relaxedSlip);
        CHECK_BIKE(std::isfinite(force) && std::abs(force) < 40000);
    }
    const QSize frustumSize(3840, 2160);
    CHECK_BIKE(FlightProjection::outsidePlanes({0, 0, 100}, frustumSize) == 0);
    CHECK_BIKE(FlightProjection::outsidePlanes({500, 0, 100}, frustumSize) & 4);
    CHECK_BIKE(FlightProjection::outsidePlanes({0, 500, 100}, frustumSize) & 16);
    CHECK_BIKE(FlightProjection::outsidePlanes({0, 0, -10}, frustumSize) & 1);
    // Large/crossing polygons must survive even with every corner off-screen.
    CHECK_BIKE((FlightProjection::outsidePlanes({-500, 0, 100}, frustumSize)
        & FlightProjection::outsidePlanes({500, 0, 100}, frustumSize)) == 0);
    CHECK_BIKE((FlightProjection::outsidePlanes({0, 0, -10}, frustumSize)
        & FlightProjection::outsidePlanes({0, 0, 100}, frustumSize)) == 0);
    // Camera hold/return is deterministic and independent of riding physics.
    // An obstructed previous camera must pull in immediately; validating only
    // the target still lets exponential smoothing leave it behind a wall.
    {
        int queries = 0;
        const auto wall = [&](const QVector3D &point) {
            ++queries;
            return point.z() <= -250 && point.z() >= -300 && std::abs(point.x()) < 100;
        };
        const auto pulledIn = DirtBikeCameraBoom::resolve({}, {0, 0, -600},
            {0, 0, -600}, 1.f / 60, false, wall).position;
        CHECK_BIKE(pulledIn.z() > -250 && pulledIn.z() < -190);
        CHECK_BIKE(queries <= 2 * (DirtBikeCameraBoom::maximumSteps + 1));
        const auto aroundCorner = DirtBikeCameraBoom::resolve({}, {500, 0, 0},
            {0, 0, -600}, 1.f / 60, false, wall).position;
        CHECK_BIKE(aroundCorner.z() > -250);
        const auto clear = [](const QVector3D &) { return false; };
        const QVector3D target(0, 0, -600);
        const auto releaseResult = DirtBikeCameraBoom::resolve({}, target, pulledIn, 1.f / 60, false, clear);
        const auto released = releaseResult.position;
        CHECK_BIKE(releaseResult.pending);
        const auto expected = pulledIn + (target - pulledIn) * (1 - std::exp(-9.f / 60));
        CHECK_BIKE((released - expected).length() < .001f);
        CHECK_BIKE(released.z() > target.z() && released.z() < pulledIn.z());
        CHECK_BIKE(DirtBikeCameraBoom::resolve({}, target, {}, 0, true, clear).position == target);
        const auto ground = [](const QVector3D &point) { return point.y() < 55; };
        const auto aboveGround = DirtBikeCameraBoom::resolve({0, 190, 0}, {0, -100, -600},
            {0, -200, -600}, .02f, false, ground).position;
        CHECK_BIKE(aboveGround.y() >= 55);
        queries = 0;
        const auto count = [&](const QVector3D &) { ++queries; return false; };
        const auto bounded = DirtBikeCameraBoom::resolve({}, {0, 0, -100000},
            {0, 0, -100000}, 60, false, count).position;
        CHECK_BIKE(bounded.length() <= 800.001f);
        CHECK_BIKE(queries <= 2 * (DirtBikeCameraBoom::maximumSteps + 1));
        auto settling = DirtBikeCameraBoom::resolve({}, target, target, .02f, false, wall);
        for (int frame = 0; frame < 180 && settling.pending; ++frame)
            settling = DirtBikeCameraBoom::resolve({}, target, settling.position, 1.f / 60, false, wall);
        CHECK_BIKE(!settling.pending && settling.position.z() > -250);
        const float invalid = std::numeric_limits<float>::quiet_NaN();
        CHECK_BIKE(DirtBikeCameraBoom::resolve({}, target, {invalid, 0, 0}, .02f, false, clear).position == target);
    }
    DirtBikeCameraLook cameraLook;
    for (int frame = 0; frame < 30; ++frame)
        cameraLook.advance({1, .5}, 1.0 / 60);
    const auto heldLook = cameraLook.offset();
    CHECK_BIKE(heldLook.x() > .8 && heldLook.y() > .2);
    for (int frame = 0; frame < 114; ++frame)
        cameraLook.advance({}, 1.0 / 60);
    CHECK_BIKE(cameraLook.offset() == heldLook && cameraLook.pending());
    for (int frame = 0; frame < 18; ++frame)
        cameraLook.advance({}, 1.0 / 60);
    CHECK_BIKE(cameraLook.offset().x() > 0 && cameraLook.offset().x() < heldLook.x());
    cameraLook.advance({-1, 0}, 1.0 / 60);
    const auto interruptedReturn = cameraLook.offset();
    for (int frame = 0; frame < 100; ++frame)
        cameraLook.advance({.001, -.001}, 1.0 / 60);
    CHECK_BIKE(cameraLook.offset() == interruptedReturn);
    for (int frame = 0; frame < 240; ++frame)
        cameraLook.advance({}, 1.0 / 60);
    CHECK_BIKE(cameraLook.offset().isNull() && !cameraLook.pending());
    cameraLook.nudge({100, -100});
    CHECK_BIKE(cameraLook.offset().x() <= 3 && cameraLook.offset().y() >= -.65);
    cameraLook.reset();
    cameraLook.advance({1, 1}, std::numeric_limits<double>::quiet_NaN());
    cameraLook.nudge({std::numeric_limits<double>::infinity(), 0});
    CHECK_BIKE(!cameraLook.pending());
    DirtBikeCameraLook slowCamera;
    DirtBikeCameraLook fastCamera;
    slowCamera.nudge({1, .4});
    fastCamera.nudge({1, .4});
    for (int frame = 0; frame < 75; ++frame)
        slowCamera.advance({}, 1.0 / 30);
    for (int frame = 0; frame < 300; ++frame)
        fastCamera.advance({}, 1.0 / 120);
    CHECK_BIKE(std::abs(slowCamera.offset().x() - fastCamera.offset().x()) < .0001);

    CHECK_BIKE(CockpitController::normalizedTrigger(-32768) == 0);
    CHECK_BIKE(CockpitController::normalizedTrigger(0) == 0);
    CHECK_BIKE(CockpitController::normalizedTrigger(32767) == 1);
    CHECK_BIKE(CockpitController::normalizedTrigger(16384) > .45f);
    DirtBikeSynth synth;
    std::array<float, 4800> sound;
    synth.render(sound.data(), int(sound.size()), {1, 1400, .7f, 1, false});
    double energy = 0;
    for (float sample : sound) {
        CHECK_BIKE(std::isfinite(sample) && std::abs(sample) <= .7f);
        energy += sample*sample;
    }
    CHECK_BIKE(energy > 1);
    DirtBikeSynth silentEngine;
    DirtBikeSynth::Mix engineOff;
    engineOff.engineRunning = false;
    silentEngine.render(sound.data(), int(sound.size()), engineOff);
    for (float sample : sound)
        CHECK_BIKE(sample == 0);
    // A stale slip value cannot produce gravel noise in flight. Engine and
    // wind are disabled here so the contact-specific layer is isolated.
    DirtBikeSynth gravel;
    engineOff.skid = 1;
    engineOff.airborne = true;
    gravel.render(sound.data(), int(sound.size()), engineOff);
    for (float sample : sound)
        CHECK_BIKE(sample == 0);
    engineOff.airborne = false;
    gravel.render(sound.data(), int(sound.size()), engineOff);
    double gravelEnergy = 0;
    for (float sample : sound)
        gravelEnergy += sample * sample;
    CHECK_BIKE(gravelEnergy > 1);
    if (qEnvironmentVariable("SDL_AUDIODRIVER") == "dummy") {
        qputenv("SDL_AUDIODRIVER", "dockpipe-missing-driver-fixture");
        {
            DirtBikeSound unavailable;
            CHECK_BIKE(!unavailable.start() && !unavailable.playing());
            CHECK_BIKE(!unavailable.start());
            unavailable.stop();
        }
        qputenv("SDL_AUDIODRIVER", "dummy");
        DirtBikeSound audio;
        CHECK_BIKE(audio.start() && audio.playing());
        audio.update({.5f, 500, 0, 0, false});
        idle(80);
        audio.stop();
        CHECK_BIKE(!audio.playing());
        CHECK_BIKE(audio.start());
        audio.stop();
    }
    CHECK_BIKE(!QImage(":/forgepipe/materials/dirt.jpg").isNull());
    CHECK_BIKE(!QImage(":/forgepipe/materials/rock.jpg").isNull());
    CHECK_BIKE(!QImage(":/forgepipe/materials/grass.jpg").isNull());
    const QImage juniper(":/forgepipe/materials/juniper.png");
    CHECK_BIKE(!juniper.isNull() && juniper.hasAlphaChannel());
    CHECK_BIKE(juniper.pixelColor(0, 0).alpha() == 0);
    const auto dirt = SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Dirt, QColor("#98704c"), 0);
    CHECK_BIKE(dirt.size() == QSize(512, 512));
    CHECK_BIKE(dirt == SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Dirt, QColor("#98704c"), 0));
    CHECK_BIKE(SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Dirt, Qt::gray, 4).size() == QSize(32, 32));
    CHECK_BIKE(SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Plain, Qt::gray, 0).isNull());
    CHECK_BIKE(SpatialSurfaceTextures::tile(static_cast<SpatialSurfaceMaterial>(99), Qt::gray, 0).isNull());
    // Shared loft admission: exact polygon budget, outward smooth normals,
    // no partial output on malformed or excessive input, preserved part/style.
    {
        SpatialVehicleFace style;
        style.part = DirtBikeModel::LeftForearm;
        style.material = SpatialSurfaceMaterial::Fabric;
        style.albedo = QColor("#abcdee");
        const QVector<SpatialLoftSection> rings{{{0, 0, 0}, 3, 2}, {{0, 0, 10}, 2, 1}};
        const auto mesh = SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 1, 0}, 8, 24, style);
        CHECK_BIKE(mesh.succeeded() && mesh.faces.size() == 24);
        const auto openMesh = SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 1, 0}, 8, 8, style, false);
        CHECK_BIKE(openMesh.succeeded() && openMesh.faces.size() == 8);
        for (const auto &face : mesh.faces) {
            CHECK_BIKE(face.part == style.part && face.material == style.material && face.albedo == style.albedo);
            CHECK_BIKE(face.normals.size() == face.vertices.size());
            const auto normal = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
                face.vertices[2] - face.vertices[0]).normalized();
            CHECK_BIKE(QVector3D::dotProduct(normal, face.normals.first()) > .5f);
        }
        const auto rejected = SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 1, 0}, 8, 23, style);
        CHECK_BIKE(!rejected.succeeded() && rejected.faces.isEmpty());
        CHECK_BIKE(!SpatialMeshLoft::build(rings, {1, 0, 0}, {1, 0, 0}, 8, 24, style).succeeded());
        CHECK_BIKE(!SpatialMeshLoft::build(rings, {}, {0, 1, 0}, 8, 24, style).succeeded());
        CHECK_BIKE(!SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 1, 0}, 2, 24, style).succeeded());
        CHECK_BIKE(!SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 1, 0}, 8, -1, style).succeeded());
        auto invalid = rings;
        invalid[1].width = 0;
        CHECK_BIKE(!SpatialMeshLoft::build(invalid, {1, 0, 0}, {0, 1, 0}, 8, 24, style).succeeded());
        invalid = rings;
        invalid[1].center.setZ(std::numeric_limits<float>::quiet_NaN());
        CHECK_BIKE(!SpatialMeshLoft::build(invalid, {1, 0, 0}, {0, 1, 0}, 8, 24, style).succeeded());
        invalid = rings;
        invalid[0].width = std::numeric_limits<float>::max();
        invalid[1].width = std::numeric_limits<float>::max();
        CHECK_BIKE(!SpatialMeshLoft::build(invalid, {1, 0, 0}, {0, 1, 0}, 8, 24, style).succeeded());
        invalid = rings;
        invalid[1].center = invalid[0].center;
        CHECK_BIKE(!SpatialMeshLoft::build(invalid, {1, 0, 0}, {0, 1, 0}, 8, 24, style).succeeded());
    }
    const auto riderModel = DirtBikeModel::build(QColor("#ed6837"), true);
    std::printf("bike_model_faces=%lld\n", static_cast<long long>(riderModel.size()));
    int metalFaces = 0;
    int paintedFaces = 0;
    int fabricFaces = 0;
    for (const auto &face : riderModel) {
        metalFaces += face.finish == SpatialSurfaceFinish::Metal;
        paintedFaces += face.finish == SpatialSurfaceFinish::Painted;
        fabricFaces += face.material == SpatialSurfaceMaterial::Fabric;
        if (face.material == SpatialSurfaceMaterial::Plain) {
            CHECK_BIKE(face.uvs.isEmpty());
        } else {
            CHECK_BIKE(face.uvs.size() == face.vertices.size());
            for (const auto &uv : face.uvs)
                CHECK_BIKE(std::isfinite(uv.x()) && std::isfinite(uv.y())
                    && std::abs(uv.x()) < 10 && std::abs(uv.y()) < 10);
            // A textured triangle must retain a nonzero UV area; otherwise
            // the authored surface collapses to a stretched line in the shader.
            const auto a = face.uvs[1] - face.uvs[0];
            const auto b = face.uvs[2] - face.uvs[0];
            CHECK_BIKE(std::abs(a.x() * b.y() - a.y() * b.x()) > .000001f);
        }

        if (face.part == DirtBikeModel::FrontWheel || face.part == DirtBikeModel::RearWheel) {
            if (face.albedo.lightnessF() < .22f)
                CHECK_BIKE(face.material == SpatialSurfaceMaterial::Rubber);
            else
                CHECK_BIKE(face.finish == SpatialSurfaceFinish::Metal
                    && face.material == SpatialSurfaceMaterial::Plain);
        }
        if (face.part == DirtBikeModel::Rider)
            CHECK_BIKE(face.material == SpatialSurfaceMaterial::Fabric);
    }
    CHECK_BIKE(metalFaces > 300 && paintedFaces > 300 && fabricFaces > 300);

    CHECK_BIKE(riderModel.size() <= DirtBikeModel::maximumFaces);
    CHECK_BIKE(riderModel.size() > 2200); // Detailed bike and articulated rider remain bounded.
    for (const auto &face : riderModel) {
        CHECK_BIKE(face.vertices.size() >= 3);
        const float area = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
            face.vertices[2] - face.vertices[0]).length();
        if (area <= .001f) {
            std::fprintf(stderr, "Degenerate bike polygon part=%d vertices=%lld\n", face.part,
                static_cast<long long>(face.vertices.size()));
            for (const auto &point : face.vertices)
                std::fprintf(stderr, "  %.5f %.5f %.5f\n", point.x(), point.y(), point.z());
        }
        CHECK_BIKE(area > .001f);
        CHECK_BIKE(face.albedo.isValid());
        CHECK_BIKE(face.normals.isEmpty() || face.normals.size() == face.vertices.size());
        for (const auto &normal : face.normals)
            CHECK_BIKE(std::abs(normal.length() - 1) < .001f);
    }
    CHECK_BIKE(riderModel.size() > DirtBikeModel::build(QColor("#ed6837"), false).size());
    // A settled rider rests on terrain in either fall direction. Validate the
    // actual authored vertices, independently of the part-bound contact pass.
    {
        SpatialInterior ground;
        ground.size = {100000, 100000};
        for (float slope : {0.f, .35f, -.35f}) {
            ground.rideSurfaces.clear();
            SpatialRideSurface ramp;
            ramp.width = 2000;
            ramp.profile = {{-2000, 0}, {-700, 350 - slope * 700},
                {700, 350 + slope * 700}, {2000, 0}};
            CHECK_BIKE(ramp.valid());
            ground.rideSurfaces.append(ramp);
            for (float side : {-1.f, 1.f}) {
                for (float heading : {0.f, .8f, 2.6f}) {
                    DirtBikeDynamics::State fallen;
                    fallen.position = {0, 350, 0};
                    fallen.lean = side * 1.45f;
                    fallen.heading = heading;
                    fallen.crashed = true;
                    fallen.crashTime = DirtBikeDynamics::crashDuration;
                    const DirtBikeCrashPose pose(fallen, riderModel, ground);
                    float minimumClearance = 10000;
                    for (const auto &face : riderModel) {
                        if (!DirtBikeModel::riderPart(face.part))
                            continue;
                        for (const auto &point : face.vertices) {
                            const auto world = fallen.position
                                + SpatialVehicleModel::rotate(pose.point(point), heading);
                            const float clearance = world.y() - DirtBikeDynamics::groundHeight(ground, world);
                            CHECK_BIKE(std::isfinite(clearance) && clearance >= 1.99f);
                            minimumClearance = qMin(minimumClearance, clearance);
                        }
                    }
                    if (slope == 0)
                        CHECK_BIKE(std::abs(minimumClearance - 2) < .01f);
                    else
                        CHECK_BIKE(minimumClearance < 30); // Conservative, not exact surface fitting.
                    const QVector3D shoulder(20, 150, 0), boot(20, 50, 0);
                    CHECK_BIKE(std::abs((pose.point(shoulder) - pose.point(boot)).length() - 100) < .001f);
                    CHECK_BIKE(std::abs(pose.rotation().length() - 1) < .001f);
                    fallen.crashTime += 10;
                    const DirtBikeCrashPose later(fallen, riderModel, ground);
                    CHECK_BIKE((later.point(shoulder) - pose.point(shoulder)).length() < .001f);
                }
            }
        }
        ground.rideSurfaces.clear();
        DirtBikeDynamics::State airborneWreck;
        airborneWreck.position = {0, 3000, 0};
        airborneWreck.grounded = false;
        airborneWreck.crashed = true;
        airborneWreck.crashTime = 2;
        const DirtBikeCrashPose airbornePose(airborneWreck, riderModel, ground);
        CHECK_BIKE(airborneWreck.position.y() + airbornePose.point({0, 112, -20}).y() > 2900);
        for (int frame = 0; frame <= 90; ++frame) {
            auto falling = airborneWreck;
            falling.position = {};
            falling.grounded = true;
            falling.crashTime = frame * .01f;
            const DirtBikeCrashPose pose(falling, riderModel, ground);
            for (const auto &face : riderModel) {
                if (DirtBikeModel::riderPart(face.part)) {
                    for (const auto &point : face.vertices)
                        CHECK_BIKE(pose.point(point).y() >= 1.99f);
                }
            }
        }
    }
    const QVector3D axle(0, 35, 74);
    const auto steered = DirtBikeModel::steerPoint(axle, .4f);
    const auto mirrored = DirtBikeModel::steerPoint(axle, -.4f);
    CHECK_BIKE(steered.x() > 0 && mirrored.x() < 0);
    CHECK_BIKE(std::abs(steered.x() + mirrored.x()) < .001f);
    CHECK_BIKE(DirtBikeModel::steerPoint(DirtBikeSteeringRig::stem(), .4f) == DirtBikeSteeringRig::stem());
    // Every generated front part uses one rigid head joint. Wheel spin and
    // lower-leg travel occur before steering; fixed frame points do not turn.
    for (float angle : {-.6f, 0.f, .6f}) {
        const DirtBikeSteeringRig rig(angle, 12, .7f);
        const auto stem = DirtBikeSteeringRig::stem();
        const auto axis = DirtBikeSteeringRig::axis();
        const auto onAxis = stem + axis * 20;
        CHECK_BIKE((DirtBikeSteeringRig::steerPoint(onAxis, angle) - onAxis).length() < .001f);
        for (int part : {DirtBikeModel::Fork, DirtBikeModel::Handlebars}) {
            const auto point = DirtBikeSteeringRig::grip(true);
            CHECK_BIKE((rig.point(part, point) - DirtBikeSteeringRig::steerPoint(point, angle)).length() < .001f);
            CHECK_BIKE(std::abs((rig.point(part, point) - stem).length() - (point - stem).length()) < .001f);
        }
        const auto axle = DirtBikeSteeringRig::frontAxle();
        const auto wheelAxle = rig.point(DirtBikeModel::FrontWheel, axle);
        CHECK_BIKE((wheelAxle - rig.point(DirtBikeModel::ForkLower, axle)).length() < .001f);
        CHECK_BIKE(rig.point(DirtBikeModel::Frame, stem) == stem);
        const auto wheelForward = rig.rotation(DirtBikeModel::Fork).rotatedVector({0, 0, 1});
        CHECK_BIKE(std::abs(std::atan2(wheelForward.x(), wheelForward.z())
            - DirtBikeSteeringRig::rollingYaw(angle)) < .0001f);
        for (bool right : {false, true}) {
            const auto upper = DirtBikeSteeringRig::forkPoint(right, 116);
            const auto lower = DirtBikeSteeringRig::forkPoint(right, 35);
            CHECK_BIKE(QVector3D::crossProduct((upper - lower).normalized(), axis).length() < .001f);
        }
    }
    CHECK_BIKE(DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::Handlebars));
    CHECK_BIKE(DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::RightHand));
    CHECK_BIKE(!DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::Helmet));
    CHECK_BIKE(!DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::Rider));
    CHECK_BIKE(DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::LeftUpperArm));
    CHECK_BIKE(DirtBikeSteeringRig::visibleFromRider(DirtBikeModel::LeftForearm));
    // The controls lie below the original narrow lens. Painter culling must
    // use the rider lens too, or native and fallback views disagree.
    CHECK_BIKE(FlightProjection::outsidePlanes({0, -60, 100}, {3840, 2160}) != 0);
    CHECK_BIKE(FlightProjection::outsidePlanes({0, -60, 100}, {3840, 2160}, 2160 * .55) == 0);
    CHECK_BIKE(DirtBikeSteeringRig::rollingYaw(std::numeric_limits<float>::quiet_NaN()) == 0);
    CHECK_BIKE(std::abs(DirtBikeSteeringRig::rollingYaw(100)
        - DirtBikeSteeringRig::rollingYaw(DirtBikeSteeringRig::maximumAngle)) < .0001f);
    auto track = SpatialBikeTrack::create();
    int meshVertices = 0, meshFaces = 0;
    for (const auto &mesh : track.meshes) {
        meshVertices += mesh.vertices.size();
        meshFaces += mesh.faces.size();
        CHECK_BIKE(mesh.faceColors.isEmpty() || mesh.faceColors.size() == mesh.faces.size());
        if (!mesh.normals.isEmpty()) {
            CHECK_BIKE(mesh.hasSmoothNormals());
            for (const auto &normal : mesh.normals)
                CHECK_BIKE(std::abs(normal.lengthSquared() - 1) < .001f && normal.y() > 0);
            auto malformed = mesh;
            malformed.normals.first() = {};
            CHECK_BIKE(!malformed.hasSmoothNormals());
            malformed.normals.first() = {0, std::numeric_limits<float>::infinity(), 0};
            CHECK_BIKE(!malformed.hasSmoothNormals());
            malformed.normals.removeLast();
            CHECK_BIKE(!malformed.hasSmoothNormals());
        }
        for (const auto &face : mesh.faces) {
            CHECK_BIKE(face.size() >= 3 && face.size() <= 4);
            for (int vertex : face)
                CHECK_BIKE(vertex >= 0 && vertex < mesh.vertices.size());
            const auto normal = QVector3D::crossProduct(mesh.vertices[face[1]] - mesh.vertices[face[0]],
                mesh.vertices[face[2]] - mesh.vertices[face[0]]);
            CHECK_BIKE(normal.lengthSquared() > .001f);
            // The raised fabric canopy's outward faces must point up for
            // daylight and shadowing; inward winding makes the roof look dark.
            if (mesh.material == SpatialSurfaceMaterial::Fabric && mesh.faces.size() == 4)
                CHECK_BIKE(normal.y() > 0);
            if (!mesh.normals.isEmpty())
                CHECK_BIKE(normal.y() > 0);
        }
    }
    CHECK_BIKE(meshVertices <= SpatialInterior::maximumMeshVertices && meshFaces <= SpatialInterior::maximumMeshFaces);
    CHECK_BIKE(!SpatialInteriorGeometry::blocked(track, {-5300,160,-8600}));
    CHECK_BIKE(SpatialInteriorGeometry::blocked(track, {-5970,105,-8200}));
    std::printf("bike_track_mesh_vertices=%d faces=%d\n", meshVertices, meshFaces);
    CHECK_BIKE(track.solids.size() <= SpatialInterior::maximumSolids);
    int trackSigns = 0;
    for (const auto &solid : track.solids) {
        if (!solid.label.startsWith("01 ·") && !solid.label.startsWith("02 ·")
            && !solid.label.startsWith("03 ·"))
            continue;
        ++trackSigns;
        CHECK_BIKE(solid.size.x() >= solid.size.y() * 2 && solid.size.z() <= 30);
        CHECK_BIKE(solid.center.y() - solid.size.y() / 2 >= 140);
        CHECK_BIKE(std::abs(solid.center.x()) - solid.size.x() / 2 > 3300);
        CHECK_BIKE(solid.yaw == (solid.center.x() > 0 ? 180 : 0));
    }
    CHECK_BIKE(trackSigns == 3);
    CHECK_BIKE(track.foliage.size() >= 48 && track.foliage.size() <= 72);
    std::printf("bike_foliage=%lld\n", static_cast<long long>(track.foliage.size()));
    SpatialFoliagePlacement trackPlacement(track);
    const auto repeatTrack = SpatialBikeTrack::create();
    CHECK_BIKE(track.foliage.size() == repeatTrack.foliage.size());
    for (int index = 0; index < track.foliage.size(); ++index) {
        CHECK_BIKE(trackPlacement.admit(track.foliage[index]));
        CHECK_BIKE(track.foliage[index].base == repeatTrack.foliage[index].base);
    }
    // The exact reported failure classes: signage and buried hillside art.
    SpatialFoliagePlacement originalLocations(track);
    CHECK_BIKE(!originalLocations.error({{-4300, 0, -4500}, 800, 760}).isEmpty());
    CHECK_BIKE(!originalLocations.error({{-4800, 0, 1200}, 800, 760}).isEmpty());
    SpatialInterior plantingRoom;
    plantingRoom.size = {10000, 10000};
    plantingRoom.height = 2000;
    const SpatialFoliage tree{{1000, 0, 1000}, 800, 760};
    SpatialFoliagePlacement clearPlacement(plantingRoom);
    CHECK_BIKE(clearPlacement.error(tree).isEmpty());
    CHECK_BIKE(clearPlacement.admit(tree));
    CHECK_BIKE(!clearPlacement.admit(tree));
    CHECK_BIKE(!clearPlacement.error({{4900, 0, 1000}, 800, 760}).isEmpty());
    CHECK_BIKE(!clearPlacement.error({{2000, 80, 2000}, 800, 760}).isEmpty());
    CHECK_BIKE(!clearPlacement.error({{2000, 0, 2000}, 2100, 760}).isEmpty());
    CHECK_BIKE(!clearPlacement.error({{2000, 0, 2000}, 800, std::numeric_limits<float>::infinity()}).isEmpty());
    // A thin, rotated non-collidable sign still occupies visible space.
    plantingRoom.solids.append({{1000, 300, 1000}, {1000, 600, 10}, Qt::gray,
        false, SpatialSolidShape::Box, {}, 45});
    CHECK_BIKE(!SpatialFoliagePlacement(plantingRoom).error(tree).isEmpty());
    plantingRoom.solids = {{{1000, 300, 1000}, {80, 600, 800}, Qt::gray,
        false, SpatialSolidShape::Box, "Readable sign"}};
    CHECK_BIKE(!SpatialFoliagePlacement(plantingRoom).error({{1800, 0, 1000}, 800, 760}).isEmpty());
    plantingRoom.solids.clear();
    SpatialMesh slope;
    slope.vertices = {{600, 0, 600}, {1400, 0, 600}, {1000, 1500, 1800}};
    slope.faces = {{0, 1, 2}};
    plantingRoom.meshes.append(slope);
    CHECK_BIKE(!SpatialFoliagePlacement(plantingRoom).error(tree).isEmpty());
    plantingRoom.meshes.clear();
    plantingRoom.foliage.append(tree);
    CHECK_BIKE(SpatialInteriorGeometry::blocked(plantingRoom, tree.base + QVector3D(0, 160, 0)));
    CHECK_BIKE(!SpatialInteriorGeometry::blocked(plantingRoom, tree.base + QVector3D(150, 160, 0)));
    SpatialVehicle treeProbe;
    treeProbe.id = "tree-probe";
    treeProbe.body = "dirt-bike";
    treeProbe.position = tree.base;
    CHECK_BIKE(SpatialVehicleModel::blocked(plantingRoom, treeProbe));
    treeProbe.position.setX(1500);
    CHECK_BIKE(!SpatialVehicleModel::blocked(plantingRoom, treeProbe));
    SpatialInterior capacityRoom;
    capacityRoom.size = {20000, 20000};
    SpatialFoliagePlacement capacityPlacement(capacityRoom);
    for (int index = 0; index < SpatialInterior::maximumFoliage; ++index) {
        CHECK_BIKE(capacityPlacement.admit({{-8000.f + (index % 8) * 500,
            0, 2000.f + (index / 8) * 500}, 100, 100}));
    }
    CHECK_BIKE(capacityPlacement.error({{5000, 0, 5000}, 100, 100}).contains("budget"));
    CHECK_BIKE(track.rideSurfaces.size() <= SpatialRideSurface::maximumSurfaces);
    CHECK_BIKE(DirtBikeDynamics::groundMaterial(track, {-2700, 0, -8400}) == SpatialSurfaceMaterial::Dirt);
    CHECK_BIKE(DirtBikeDynamics::groundMaterial(track, {0, 0, 11300}) == SpatialSurfaceMaterial::Dirt);
    CHECK_BIKE(DirtBikeDynamics::groundMaterial(track, {0, 0, 0}) == SpatialSurfaceMaterial::Grass);
    {
        SpatialInterior materials;
        materials.groundMaterial = SpatialSurfaceMaterial::Grass;
        SpatialRideSurface lane;
        lane.width = 1200;
        lane.profile = {{-2000, 0}, {2000, 0}};
        materials.rideSurfaces.append(lane);
        CHECK_BIKE(DirtBikeDynamics::groundMaterial(materials, {599, 0, 0}) == SpatialSurfaceMaterial::Dirt);
        CHECK_BIKE(DirtBikeDynamics::groundMaterial(materials, {601, 0, 0}) == SpatialSurfaceMaterial::Grass);
        CHECK_BIKE(DirtBikeDynamics::groundMaterial(materials, {0, 0, 2001}) == SpatialSurfaceMaterial::Grass);
        materials.rideSurfaces[0].width = -1;
        CHECK_BIKE(DirtBikeDynamics::groundMaterial(materials, {}) == SpatialSurfaceMaterial::Grass);
        materials.rideSurfaces.fill(materials.rideSurfaces.first(), SpatialRideSurface::maximumSurfaces);
        materials.rideSurfaces.append(lane);
        CHECK_BIKE(DirtBikeDynamics::groundMaterial(materials, {}) == SpatialSurfaceMaterial::Grass);
    }
    auto vehicle = track.vehicles.first();
    CHECK_BIKE(SpatialVehicleModel::valid(vehicle));
    CHECK_BIKE(SpatialVehicleModel::build(vehicle).size() <= SpatialVehicleModel::maximumFaces);
    for (const auto &surface : track.rideSurfaces) {
        CHECK_BIKE(surface.valid());
        const auto mesh = surface.mesh();
        for (const auto &face : mesh.faces) {
            QVector3D center;
            for (int vertex : face)
                center += mesh.vertices[vertex];
            center /= face.size();
            CHECK_BIKE(std::abs(surface.heightAt(center) - center.y()) < .02f);
        }
        for (const auto &point : mesh.vertices) {
            if (point.y() > 0)
                CHECK_BIKE(std::abs(surface.heightAt(point) - point.y()) < .02f);
        }
    }
    // Curved berm contacts use the same triangles, including mirrored turns,
    // translated/rotated areas, and the coarsest admitted four-strip arc.
    for (float radius : {-2700.f, 2700.f}) {
        auto berm = track.rideSurfaces.last();
        berm.curveRadius = radius;
        berm.center = {700, 0, -900};
        berm.heading = .73f;
        for (bool coarse : {false, true}) {
            if (coarse)
                berm.profile = {{0, 0}, {8482.3, 0}};
            CHECK_BIKE(berm.valid());
            const auto mesh = berm.mesh();
            for (const auto &face : mesh.faces) {
                const auto a = mesh.vertices[face[0]];
                const auto b = mesh.vertices[face[1]];
                const auto c = mesh.vertices[face[2]];
                for (const auto &weights : {QVector3D(.2f, .3f, .5f), QVector3D(.05f, .9f, .05f)}) {
                    const auto point = a * weights.x() + b * weights.y() + c * weights.z();
                    CHECK_BIKE(std::abs(berm.heightAt(point) - point.y()) < .03f);
                }
            }
            CHECK_BIKE(berm.heightAt(berm.center + QVector3D(10000, 0, 10000)) < 0);
        }
    }
    auto malformedBerm = track.rideSurfaces.last();
    malformedBerm.curveRadius = 1;
    CHECK_BIKE(!malformedBerm.valid());
    malformedBerm = track.rideSurfaces.last();
    malformedBerm.bank = std::numeric_limits<float>::quiet_NaN();
    CHECK_BIKE(!malformedBerm.valid());
    malformedBerm = track.rideSurfaces.last();
    malformedBerm.curveRadius = 0;
    CHECK_BIKE(!malformedBerm.valid());

    auto bankedBike = vehicle;
    bankedBike.position = {0, DirtBikeDynamics::groundHeight(track, {0, 0, 11300}), 11300};
    bankedBike.heading = 1.5707963f;
    DirtBikeDynamics bankedRest;
    bankedRest.reset(bankedBike);
    ride(bankedRest, {}, track, 180);
    CHECK_BIKE(!bankedRest.state().crashed && bankedRest.state().grounded);
    CHECK_BIKE(std::abs(bankedRest.state().lean) > .1f && std::abs(bankedRest.state().lean) < .5f);
    CHECK_BIKE((bankedRest.state().position - bankedBike.position).length() < 15);
    bankedBike.position = {-2700, 0, 8600};
    bankedBike.heading = 0;
    DirtBikeDynamics bankedTurn;
    bankedTurn.reset(bankedBike);
    float turnProgress = 0;
    float maximumRadiusError = 0;
    for (int frame = 0; frame < 1200 && turnProgress < 2.9f; ++frame) {
        const auto &state = bankedTurn.state();
        const float radialZ = state.position.z() - 8600;
        const float radius = std::hypot(state.position.x(), radialZ);
        turnProgress = std::atan2(radialZ, -state.position.x());
        maximumRadiusError = qMax(maximumRadiusError, std::abs(radius - 2700));
        const float desiredHeading = turnProgress + std::atan2(radius - 2700, 500.f);
        const float headingError = std::remainder(desiredHeading - state.heading, 6.2831853f);
        const float steeringLimit = qMin(.48f, std::atan2(.9f * 981 * 148,
            qMax(1.f, state.speed * state.speed)));
        DirtBikeDynamics::Input followBerm;
        followBerm.throttle = .25f;
        followBerm.steering = qBound(-1.f, (std::atan(148.f / 2700) + headingError * .35f) / steeringLimit, 1.f);
        bankedTurn.advance(followBerm, 1.f / 60, track);
        CHECK_BIKE(!bankedTurn.state().crashed);
    }
    std::printf("bike_berm progress=%.3f radius_error_cm=%.2f lean=%.3f\n",
        turnProgress, maximumRadiusError, bankedTurn.state().lean);
    CHECK_BIKE(turnProgress > 2.9f && maximumRadiusError < 400);
    CHECK_BIKE(DirtBikeDynamics::landingCrashReason(0, 1.4f, 0, 500, .5f) == DirtBikeDynamics::CrashReason::None);
    CHECK_BIKE(DirtBikeDynamics::landingCrashReason(0, 1.4f, 0, 500) == DirtBikeDynamics::CrashReason::BadLanding);
    SpatialInterior hiddenBike;
    hiddenBike.size = {4000, 4000};
    SpatialRideSurface barrier;
    barrier.width = 400;
    barrier.profile = {{-300, 0}, {0, 250}, {300, 0}};
    hiddenBike.rideSurfaces.append(barrier);
    auto behind = vehicle;
    behind.position = {0, 0, 500};
    hiddenBike.vehicles.append(behind);
    CHECK_BIKE(SpatialInteriorGeometry::target(hiddenBike, {0, 100, -500}, {0, 0, 1}).isEmpty());
    auto invalid = track.rideSurfaces.first();
    invalid.profile[1].setX(invalid.profile[0].x());
    CHECK_BIKE(!invalid.valid() && invalid.mesh().vertices.isEmpty());
    invalid = track.rideSurfaces.first();
    invalid.width = std::numeric_limits<float>::quiet_NaN();
    CHECK_BIKE(!invalid.valid());

    SpatialInterior flat;
    flat.size = {100000, 100000};
    vehicle.position = {};
    // Surface tuning changes available tyre force, while dry grass must
    // remain controllable at ordinary steering inputs. Start braking from
    // identical state so stopping differences cannot be explained by speed.
    {
        SpatialInterior dirt = flat;
        dirt.groundMaterial = SpatialSurfaceMaterial::Dirt;
        SpatialInterior grass = flat;
        grass.groundMaterial = SpatialSurfaceMaterial::Grass;
        DirtBikeDynamics dirtDrive, grassDrive;
        dirtDrive.reset(vehicle);
        grassDrive.reset(vehicle);
        ride(dirtDrive, {1, 0, 0, false}, dirt, 180);
        ride(grassDrive, {1, 0, 0, false}, grass, 180);
        CHECK_BIKE(grassDrive.state().speed < dirtDrive.state().speed);
        CHECK_BIKE(grassDrive.state().speed > dirtDrive.state().speed * .75f);
        auto dirtStop = dirtDrive;
        auto grassStop = dirtDrive;
        DirtBikeDynamics::Input brake;
        brake.serviceBrake = 1;
        ride(dirtStop, brake, dirt, 30);
        ride(grassStop, brake, grass, 30);
        CHECK_BIKE(grassStop.state().speed > dirtStop.state().speed);
        CHECK_BIKE(!grassStop.state().crashed && !dirtStop.state().crashed);
        ride(grassDrive, {.6f, .5f, 0, false}, grass, 240);
        CHECK_BIKE(!grassDrive.state().crashed && grassDrive.state().speed > 500);
        CHECK_BIKE(std::isfinite(grassDrive.state().heading));
        std::printf("bike_surface dirt_stop=%.1f grass_stop=%.1f grass_turn_speed=%.1f\n",
            dirtStop.state().speed, grassStop.state().speed, grassDrive.state().speed);
    }
    DirtBikeDynamics bike;
    bike.reset(vehicle);
    ride(bike, {}, flat, 600);
    CHECK_BIKE(bike.state().position.length() < .01 && !bike.unsettled());
    CHECK_BIKE(std::abs(bike.state().frontSupport - .5f) < .001f);
    CHECK_BIKE(std::abs(bike.state().rearSupport - .5f) < .001f);
    // Two identical bikes ask for drive while one suspension is near full
    // extension. Its lightly loaded rear contact must not deliver static grip.
    DirtBikeDynamics loadedDrive, unloadedDrive;
    loadedDrive.reset(vehicle);
    auto extendedBike = vehicle;
    extendedBike.position.setY(8);
    unloadedDrive.reset(extendedBike);
    loadedDrive.advance({1, 0, 0, false}, 1.f / 120, flat);
    unloadedDrive.advance({1, 0, 0, false}, 1.f / 120, flat);
    CHECK_BIKE(unloadedDrive.state().grounded && !unloadedDrive.state().crashed);
    CHECK_BIKE(unloadedDrive.state().rearSupport < loadedDrive.state().rearSupport * .2f);
    CHECK_BIKE(unloadedDrive.state().speed < loadedDrive.state().speed * .85f);
    std::printf("bike_supported_drive loaded=%.3f unloaded=%.3f load=%.3f\n",
        loadedDrive.state().speed, unloadedDrive.state().speed, unloadedDrive.state().rearSupport);
    ride(bike, {0, 1, 0, false}, flat, 60);
    CHECK_BIKE(bike.state().steeringAngle > .4f && bike.state().heading == 0);
    CHECK_BIKE(bike.state().position.length() < .01f);
    ride(bike, {}, flat, 120);
    CHECK_BIKE(!bike.unsettled());
    // Holding a service brake with the bars turned must not create propulsion
    // or yaw at rest, including overlapping trigger input on a controller.
    for (float steering : {-1.f, 1.f}) {
        for (float throttle : {0.f, 1.f}) {
            DirtBikeDynamics stopped;
            stopped.reset(vehicle);
            DirtBikeDynamics::Input heldBrake;
            heldBrake.steering = steering;
            heldBrake.throttle = throttle;
            heldBrake.serviceBrake = 1;
            ride(stopped, heldBrake, flat, throttle > 0 ? 300 : 30);
            std::printf("bike_held_brake steer=%.1f throttle=%.1f distance=%.4f heading=%.4f speed=%.4f\n",
                steering, throttle, stopped.state().position.length(), stopped.state().heading,
                stopped.state().velocity.length());
            CHECK_BIKE(stopped.state().position.length() < .01f);
            CHECK_BIKE(std::abs(stopped.state().heading) < .001f);
            CHECK_BIKE(stopped.state().velocity.length() < .01f);
        }
    }
    // Light-space quantization holds a stationary world's shadow coordinates
    // through sub-texel camera translation, including the depth comparison.
    const auto shadowOrigin = SpatialSunShadow::projection({0, 160, 0});
    const auto shadowMoved = SpatialSunShadow::projection({.01f, 180, .01f});
    for (const auto &point : {QVector3D(300, 20, 400), QVector3D(-500, 80, -200)}) {
        CHECK_BIKE((shadowOrigin.map(point) - shadowMoved.map(point)).length() < .000001f);
    }
    // Both keyboard S and controller LT stop first, then push backwards.
    for (bool controller : {false, true}) {
        DirtBikeDynamics reversing;
        reversing.reset(vehicle);
        DirtBikeDynamics::Input held;
        held.throttle = controller ? 0 : -1;
        held.serviceBrake = controller ? 1 : 0;
        ride(reversing, held, flat, 30);
        CHECK_BIKE(!reversing.state().reversing && std::abs(reversing.state().speed) < .01f);
        ride(reversing, {}, flat, 1);
        ride(reversing, held, flat, 30);
        CHECK_BIKE(!reversing.state().reversing); // Releasing restarts the hold delay.
        ride(reversing, held, flat, 150);
        CHECK_BIKE(reversing.state().reversing && reversing.state().position.z() < -80);
        CHECK_BIKE(reversing.state().speed < -60 && reversing.state().speed > -125);
        CHECK_BIKE(!reversing.state().crashed);
        ride(reversing, {}, flat, 1);
        CHECK_BIKE(!reversing.state().reversing);
        ride(reversing, {1, 0, 0, false}, flat, 90);
        CHECK_BIKE(reversing.state().speed > 100);
        reversing.reset(vehicle);
        ride(reversing, {0, 0, 0, false, 0, true}, flat, 180);
        CHECK_BIKE(!reversing.state().reversing && reversing.state().position.length() < .01f);
    }
    DirtBikeDynamics leftTurn, rightTurn;
    for (auto *turn : {&leftTurn, &rightTurn}) {
        turn->reset(vehicle);
        ride(*turn, {.5f, 0, 0, false}, flat, 120);
    }
    ride(leftTurn, {.4f, -.5f, 0, false}, flat, 45);
    ride(rightTurn, {.4f, .5f, 0, false}, flat, 45);
    CHECK_BIKE(!leftTurn.state().crashed && !rightTurn.state().crashed);
    CHECK_BIKE(leftTurn.state().heading < 0 && rightTurn.state().heading > 0);
    CHECK_BIKE(leftTurn.state().lean > 0 && rightTurn.state().lean < 0);
    CHECK_BIKE(std::abs(leftTurn.state().position.x() + rightTurn.state().position.x()) < .1f);
    CHECK_BIKE(rightTurn.state().position.x() > 10);
    const auto bankedUp = ArcadeFlightDynamics::attitude(0, 0, rightTurn.state().lean).rotatedVector({0, 1, 0});
    CHECK_BIKE(bankedUp.x() > 0); // Right turn leans toward the inside, not away.
    const float enteringTurnSpeed = rightTurn.state().speed;
    DirtBikeDynamics::Input turnBrake;
    turnBrake.serviceBrake = .5f;
    turnBrake.steering = .5f;
    ride(rightTurn, turnBrake, flat, 30);
    turnBrake.steering = -.5f;
    ride(leftTurn, turnBrake, flat, 30);
    CHECK_BIKE(!leftTurn.state().crashed && !rightTurn.state().crashed);
    CHECK_BIKE(rightTurn.state().speed >= 0 && rightTurn.state().speed < enteringTurnSpeed);
    CHECK_BIKE(std::abs(leftTurn.state().position.x() + rightTurn.state().position.x()) < .1f);
    CHECK_BIKE(std::abs(leftTurn.state().heading + rightTurn.state().heading) < .001f);
    ride(bike, {1, 0, 0, false}, flat, 180);
    CHECK_BIKE(bike.state().speed > 1200 && bike.state().position.z() > 2000);
    ride(bike, {1, 1, 0, false}, flat, 30);
    std::printf("bike_turn heading=%.3f lean=%.3f speed=%.1f slip_front=%.3f slip_rear=%.3f\n",
        bike.state().heading, bike.state().lean, bike.state().speed, bike.state().frontSlipAngle, bike.state().rearSlipAngle);
    CHECK_BIKE(bike.state().heading > .2 && bike.state().lean < -.1);
    ride(bike, {0, 0, 0, true}, flat, 180);
    CHECK_BIKE(std::abs(bike.state().speed) < 1);

    DirtBikeDynamics gentleBrake, fullBrake, rearBrake;
    for (auto *candidate : {&gentleBrake, &fullBrake, &rearBrake}) {
        candidate->reset(vehicle);
        ride(*candidate, {1, 0, 0, false}, flat, 180);
    }
    DirtBikeDynamics::Input braking;
    braking.serviceBrake = .25f;
    ride(gentleBrake, braking, flat, 30);
    braking.serviceBrake = 1;
    ride(fullBrake, braking, flat, 30);
    braking.serviceBrake = 0;
    braking.rearBrake = true;
    ride(rearBrake, braking, flat, 30);
    std::printf("brake_speeds full=%.1f rear=%.1f gentle=%.1f\n", fullBrake.state().speed, rearBrake.state().speed, gentleBrake.state().speed);
    CHECK_BIKE(fullBrake.state().speed < rearBrake.state().speed);
    CHECK_BIKE(rearBrake.state().speed < gentleBrake.state().speed);
    CHECK_BIKE(rearBrake.state().skid > .2f && fullBrake.state().skid == 0);
    CHECK_BIKE(fullBrake.state().frontLoad > .5f);
    CHECK_BIKE(fullBrake.state().frontCompression > fullBrake.state().rearCompression);
    braking.serviceBrake = 1;
    ride(rearBrake, braking, flat, 180);
    CHECK_BIKE(std::abs(rearBrake.state().speed) < .01f);

    DirtBikeDynamics powertrain;
    powertrain.reset(vehicle);
    int changes = 0;
    int gear = 1;
    for (int frame = 0; frame < 480; ++frame) {
        powertrain.advance({1, 0, 0, false}, 1.f/60, flat);
        const auto &state = powertrain.state();
        CHECK_BIKE(state.gear >= 1 && state.gear <= 6 && std::isfinite(state.engineRpm));
        CHECK_BIKE(state.engineRpm >= 1600 && state.engineRpm <= 10500);
        if (state.gear != gear) {
            ++changes;
            gear = state.gear;
        }
    }
    CHECK_BIKE(gear >= 3 && changes < 10 && !powertrain.state().crashed);
    const float poweredSpeed = powertrain.state().speed;
    ride(powertrain, {}, flat, 60);
    CHECK_BIKE(powertrain.state().speed < poweredSpeed - 100);
    ride(powertrain, {0, 0, .5f, false}, flat, 12);
    CHECK_BIKE(powertrain.state().riderWeight > .3f);

    DirtBikeDynamics sixty, thirty;
    sixty.reset(vehicle);
    thirty.reset(vehicle);
    ride(sixty, {1, .3, 0, false}, flat, 240);
    ride(thirty, {1, .3, 0, false}, flat, 120, 1.f / 30);
    CHECK_BIKE((sixty.state().position - thirty.state().position).length() < .1);
    auto before = sixty.state().position;
    sixty.advance({}, std::numeric_limits<float>::quiet_NaN(), flat);
    CHECK_BIKE(sixty.state().position == before);
    sixty.advance({}, 60, flat);
    CHECK_BIKE((sixty.state().position - before).length() < 150);

    // Actual authored straight: both jumps must produce airtime and controllable
    // landings without a forced launch velocity or a hidden collision shape.
    vehicle = track.vehicles.first();
    bike.reset(vehicle);
    bool airborne = false;
    bool landed = false;
    float peakClearance = 0;
    for (int frame = 0; frame < 600 && bike.state().position.z() < 6000; ++frame) {
        bike.advance({1, 0, 0, false}, 1.f/60, track);
        const auto &state = bike.state();
        airborne |= !state.grounded && state.position.y() > 80;
        landed |= airborne && state.grounded;
        peakClearance = qMax(peakClearance, state.position.y() - DirtBikeDynamics::groundHeight(track, state.position));
        if (state.crashed) {
            std::fprintf(stderr, "track crash z=%.1f y=%.1f pitch=%.3f speed=%.1f\n", state.position.z(), state.position.y(), state.pitch, state.speed);
            break;
        }
    }
    std::printf("bike_jumps airborne=%d landed=%d clearance_cm=%.1f z=%.1f\n", airborne, landed, peakClearance, bike.state().position.z());
    CHECK_BIKE(airborne && landed && peakClearance > 50 && !bike.state().crashed);
    CHECK_BIKE(bike.state().position.z() > 5500);
    // The authored tabletop permits a complete flip. Braking rotation too
    // early must still reject the resulting off-balance touchdown.
    for (float brakeAngle : {4.f, 5.5f}) {
        DirtBikeDynamics flip;
        flip.reset(track.vehicles.first());
        float rotation = 0;
        float previousPitch = 0;
        float peakRotation = 0;
        bool started = false;
        bool landedFlip = false;
        for (int frame = 0; frame < 700 && !flip.state().crashed; ++frame) {
            const auto &state = flip.state();
            float pitchInput = 0;
            if (!state.grounded && state.position.z() > 1000 && !landedFlip) {
                started = true;
                pitchInput = rotation < brakeAngle ? 1.f : -1.f;
            }
            flip.advance({1, 0, pitchInput, false}, 1.f / 60, track);
            rotation += std::remainder(flip.state().pitch - previousPitch, 6.283185307f);
            previousPitch = flip.state().pitch;
            peakRotation = qMax(peakRotation, rotation);
            if (started && flip.state().grounded) {
                landedFlip = true;
                break;
            }
        }
        CHECK_BIKE(landedFlip);
        if (brakeAngle > 5) {
            CHECK_BIKE(peakRotation > 6.28f && !flip.state().crashed);
        } else {
            CHECK_BIKE(flip.state().crashReason == DirtBikeDynamics::CrashReason::BadLanding);
        }
    }

    vehicle.position = {2700, 0, 8000};
    vehicle.heading = 3.14159265f;
    bike.reset(vehicle);
    const bool traceContact = qEnvironmentVariableIsSet("DOCKPIPE_BIKE_CONTACT_TRACE");
    bool previousContact = bike.state().grounded;
    for (int frame = 0; frame < 720 && bike.state().position.z() > -6500 && !bike.state().crashed; ++frame) {
        bike.advance({1, 0, 0, false}, 1.f/60, track);
        const auto &state = bike.state();
        if (traceContact && (frame % 10 == 0 || previousContact != state.grounded || state.crashed))
            std::printf("contact frame=%d z=%.1f y=%.1f pitch=%.3f rate=%.3f ground=%d support=%.2f/%.2f\n",
                frame, state.position.z(), state.position.y(), state.pitch, state.pitchRate,
                state.grounded, state.frontSupport, state.rearSupport);
        previousContact = state.grounded;
    }
    std::printf("bike_return_straight crashed=%d reason=%d z=%.1f y=%.1f pitch=%.2f velocity_y=%.1f\n",
        bike.state().crashed, int(bike.state().crashReason), bike.state().position.z(), bike.state().position.y(),
        bike.state().pitch, bike.state().velocity.y());
    CHECK_BIKE(!bike.state().crashed && bike.state().position.z() < -6000);
    vehicle.heading = 0;

    // Identical takeoff states isolate wheel angular momentum from rider input.
    // Throttle raises the nose, a rear-brake tap lowers it, and neither creates
    // forward thrust in mid-air. A locked wheel cannot keep adding pitch torque.
    DirtBikeDynamics takeoff;
    takeoff.reset(track.vehicles.first());
    for (int frame = 0; frame < 600; ++frame) {
        const auto before = takeoff.state();
        const auto attitude = ArcadeFlightDynamics::attitude(before.heading, before.pitch);
        const auto offset = attitude.rotatedVector({0, 0, DirtBikeSuspension::halfWheelbase});
        const auto rollingDirection = ArcadeFlightDynamics::attitude(before.heading, 0).rotatedVector({0, 0, 1});
        const auto contactHeight = [&](const QVector3D &point) {
            return DirtBikeTyreContact::supportHeight([&](float distance) {
                return DirtBikeDynamics::groundHeight(track, point + rollingDirection * distance);
            });
        };
        const float frontHeight = contactHeight(before.position + offset);
        const float rearHeight = contactHeight(before.position - offset);
        takeoff.advance({1, 0, 0, false}, 1.f / 120, track);
        CHECK_BIKE(std::abs(takeoff.state().frontCompression - (frontHeight - before.position.y() - offset.y())) < .02f);
        CHECK_BIKE(std::abs(takeoff.state().rearCompression - (rearHeight - before.position.y() + offset.y())) < .02f);
        if (!takeoff.state().grounded && takeoff.state().position.y() > 100)
            break;
    }
    CHECK_BIKE(!takeoff.state().grounded && takeoff.state().rearWheelSpeed > 10);
    CHECK_BIKE(takeoff.state().frontSupport == 0 && takeoff.state().rearSupport == 0);
    auto coastAir = takeoff;
    auto throttleAir = takeoff;
    auto brakeAir = takeoff;
    DirtBikeDynamics::Input rearTap;
    rearTap.rearBrake = true;
    for (int frame = 0; frame < 40; ++frame) {
        coastAir.advance({}, 1.f / 120, track);
        throttleAir.advance({1, 0, 0, false}, 1.f / 120, track);
        brakeAir.advance(rearTap, 1.f / 120, track);
    }
    std::printf("bike_air_control coast=%.3f throttle=%.3f brake=%.3f wheel=%.2f\n",
        coastAir.state().pitch, throttleAir.state().pitch, brakeAir.state().pitch, brakeAir.state().rearWheelSpeed);
    CHECK_BIKE(!coastAir.state().grounded && !throttleAir.state().grounded && !brakeAir.state().grounded);
    CHECK_BIKE(throttleAir.state().pitch > coastAir.state().pitch + .003f);
    CHECK_BIKE(brakeAir.state().pitch < coastAir.state().pitch - .01f);
    CHECK_BIKE(brakeAir.state().rearWheelSpeed == 0);
    CHECK_BIKE(brakeAir.state().frontWheelSpeed > 10);
    CHECK_BIKE(std::abs(throttleAir.state().position.z() - coastAir.state().position.z()) < .01f);

    // Turn on an elevated straight, then leave contact with nonzero yaw.
    // Existing momentum rotates the airborne bike; fresh steering must not
    // invent a tyre yaw force. Compare against the fixed-step decay integral.
    {
        SpatialInterior elevated;
        elevated.size = {100000, 100000};
        SpatialRideSurface platform;
        platform.width = 2000;
        platform.profile = {{-10000, 0}, {-9000, 600}, {9000, 600}, {10000, 0}};
        CHECK_BIKE(platform.valid());
        elevated.rideSurfaces.append(platform);
        auto launch = vehicle;
        launch.position = {0, 600, 0};
        DirtBikeDynamics carryingYaw;
        carryingYaw.reset(launch);
        ride(carryingYaw, {.6f, 0, 0, false}, elevated, 120);
        ride(carryingYaw, {.3f, .4f, 0, false}, elevated, 25);
        CHECK_BIKE(carryingYaw.state().grounded && !carryingYaw.state().crashed);
        CHECK_BIKE(carryingYaw.state().yawRate > .05f);
        auto opposedInput = carryingYaw;
        const auto takeoff = carryingYaw.state();
        float expectedHeading = takeoff.heading;
        float expectedRate = takeoff.yawRate;
        for (int step = 0; step < 30; ++step) {
            expectedRate *= 1 - 1.f / 120;
            expectedHeading += expectedRate / 120;
        }
        SpatialInterior airborneRoom;
        airborneRoom.size = elevated.size;
        ride(carryingYaw, {}, airborneRoom, 15);
        ride(opposedInput, {0, -1, 0, false}, airborneRoom, 15);
        CHECK_BIKE(!carryingYaw.state().grounded && !carryingYaw.state().crashed);
        CHECK_BIKE(std::abs(carryingYaw.state().heading - expectedHeading) < .001f);
        CHECK_BIKE(std::abs(carryingYaw.state().yawRate - expectedRate) < .001f);
        CHECK_BIKE(std::abs(carryingYaw.state().heading - opposedInput.state().heading) < .001f);
        CHECK_BIKE(carryingYaw.state().heading > takeoff.heading + .01f);
    }
    // No steering traction in mid-air; rider pitch still responds.
    vehicle.position = {0, 600, 0};
    bike.reset(vehicle);
    ride(bike, {1, 1, 1, false}, flat, 15);
    CHECK_BIKE(!bike.state().grounded && bike.state().heading == 0 && bike.state().pitch > .05);
    ride(bike, {}, flat, 180);
    CHECK_BIKE(std::isfinite(bike.state().position.y()));
    vehicle.position = {0, 1000, 0};
    bike.reset(vehicle);
    ride(bike, {}, flat, 240);
    CHECK_BIKE(!bike.state().crashed && std::abs(bike.state().position.y()) < 1);
    vehicle.position = {};
    bike.reset(vehicle);
    flat.solids.append({{0, 100, 600}, {1000, 200, 5}, Qt::gray});
    ride(bike, {1, 0, 0, false}, flat, 240);
    CHECK_BIKE(bike.state().position.z() < 500);

    // Fast impacts cause a bounded wipeout; low-speed bumps remain recoverable.
    SpatialInterior crashRoom;
    crashRoom.size = {100000, 100000};
    crashRoom.solids.append({{0, 350, 1800}, {4000, 700, 20}, Qt::gray});
    vehicle.position = {};
    bike.reset(vehicle);
    ride(bike, {1, 0, 0, false}, crashRoom, 360);
    CHECK_BIKE(bike.state().crashed && bike.state().crashReason == DirtBikeDynamics::CrashReason::Collision);
    CHECK_BIKE(!bike.unsettled() && bike.state().velocity.isNull());
    const auto wreckPosition = bike.state().position;
    ride(bike, {1, 1, 1, false}, crashRoom, 120);
    CHECK_BIKE(bike.state().position == wreckPosition);
    CHECK_BIKE(std::abs(bike.state().lean) > 1.3f);
    crashRoom.solids.first().center.setZ(250);
    bike.reset(vehicle);
    ride(bike, {1, 0, 0, false}, crashRoom, 180);
    CHECK_BIKE(!bike.state().crashed);
    const auto stuckPosition = bike.state().position;
    DirtBikeDynamics::Input backAway;
    backAway.serviceBrake = 1;
    ride(bike, backAway, crashRoom, 180);
    CHECK_BIKE(bike.state().reversing && !bike.state().crashed);
    CHECK_BIKE(bike.state().position.z() < stuckPosition.z() - 150);

    SpatialInterior landingRoom;
    landingRoom.size = {100000, 100000};
    // A wreck can leave an elevated contact surface while its fall animation
    // is still active. Grounded must be recomputed instead of latched forever.
    {
        auto elevated = landingRoom;
        SpatialRideSurface platform;
        platform.width = 1200;
        platform.profile = {{-2000, 0}, {-1000, 200}, {1000, 200}, {2000, 0}};
        elevated.rideSurfaces.append(platform);
        auto drop = vehicle;
        drop.position = {0, 3500, 0};
        DirtBikeDynamics wreck;
        wreck.reset(drop);
        for (int frame = 0; frame < 300 && !wreck.state().crashed; ++frame)
            wreck.advance({}, 1.f / 60, elevated);
        CHECK_BIKE(wreck.state().crashed);
        CHECK_BIKE(wreck.state().position.y() >= 199);
        wreck.advance({}, 1.f / 60, elevated);
        CHECK_BIKE(wreck.state().grounded);
        // Leave the support just before the animation expires. The pose may
        // finish while falling; gravity and ticking must continue to contact.
        while (wreck.state().crashTime < DirtBikeDynamics::crashDuration - .1f)
            wreck.advance({}, 1.f / 60, elevated);
        wreck.advance({}, 1.f / 60, landingRoom);
        CHECK_BIKE(!wreck.state().grounded && wreck.state().velocity.y() < 0);
        ride(wreck, {}, landingRoom, 10);
        CHECK_BIKE(wreck.state().crashTime == DirtBikeDynamics::crashDuration);
        CHECK_BIKE(!wreck.state().grounded && wreck.unsettled());
        ride(wreck, {}, landingRoom, 120);
        CHECK_BIKE(wreck.state().grounded && wreck.state().position.y() == 0);
        CHECK_BIKE(!wreck.unsettled() && wreck.state().velocity.isNull());
    }
    for (float dropHeight : {50.f, 200.f, 600.f}) {
        auto dropVehicle = vehicle;
        dropVehicle.position = {0, dropHeight, 0};
        DirtBikeDynamics suspensionDrop;
        suspensionDrop.reset(dropVehicle);
        float peakSupport = 0;
        float peakCompression = 0;
        for (int frame = 0; frame < 600; ++frame) {
            suspensionDrop.advance({}, 1.f / 60, landingRoom);
            const auto &state = suspensionDrop.state();
            CHECK_BIKE(!state.crashed && std::isfinite(state.position.y()));
            CHECK_BIKE(state.frontSupport >= 0 && state.rearSupport >= 0);
            peakSupport = qMax(peakSupport, state.frontSupport + state.rearSupport);
            peakCompression = qMax(peakCompression, state.frontCompression);
        }
        // Landing transfers transient force through the springs, then settles
        // back to static support instead of creating perpetual bounce/energy.
        CHECK_BIKE(peakSupport > 1.2f && peakCompression > 0);
        CHECK_BIKE(std::abs(suspensionDrop.state().position.y()) < .1f);
        CHECK_BIKE(std::abs(suspensionDrop.state().velocity.y()) < .1f);
        CHECK_BIKE(std::abs(suspensionDrop.state().frontSupport - .5f) < .01f);
        CHECK_BIKE(std::abs(suspensionDrop.state().rearSupport - .5f) < .01f);
    }
    vehicle.position = {0, 3000, 0};
    bike.reset(vehicle);
    ride(bike, {}, landingRoom, 360);
    CHECK_BIKE(bike.state().crashReason == DirtBikeDynamics::CrashReason::HardLanding);
    vehicle.position = {0, 600, 0};
    bike.reset(vehicle);
    ride(bike, {0, 0, 1, false}, landingRoom, 240);
    CHECK_BIKE(bike.state().crashReason == DirtBikeDynamics::CrashReason::BadLanding);
    vehicle.position = {};
    bike.reset(vehicle);
    ride(bike, {1, 0, 0, false}, landingRoom, 480);
    float peakSideways = 0;
    for (int frame = 0; frame < 120 && !bike.state().crashed; ++frame) {
        bike.advance({1, .5f, 0, false}, 1.f/60, landingRoom);
        const auto right = SpatialVehicleModel::rotate({1, 0, 0}, bike.state().heading);
        peakSideways = qMax(peakSideways, std::abs(QVector3D::dotProduct(bike.state().velocity, right)));
    }
    std::printf("bike_half_turn sideways_cm_s=%.2f crashed=%d speed=%.2f\n",
        peakSideways, bike.state().crashed, bike.state().speed);
    CHECK_BIKE(!bike.state().crashed && peakSideways < 200 && bike.state().speed > 1000);
    ride(bike, {1, 1, 0, false}, landingRoom, 240);
    std::printf("bike_full_turn crash=%d speed=%.1f lean=%.3f\n", int(bike.state().crashReason), bike.state().speed, bike.state().lean);
    CHECK_BIKE(!bike.state().crashed);
    // Slow skids can scrub off their speed and recover; a faster sustained
    // locked-rear-wheel slide still loses grip. Ordinary steering above must
    // not be mistaken for either deliberate slide.
    DirtBikeDynamics::Input slide{1, 1, 0, false};
    slide.rearBrake = true;
    for (int runUp : {180, 360}) {
        DirtBikeDynamics probe;
        probe.reset(vehicle);
        ride(probe, {1, 0, 0, false}, landingRoom, runUp);
        ride(probe, {1, 1, 0, false}, landingRoom, 36);
        ride(probe, slide, landingRoom, 240);
        std::printf("wheel_frame_slide runup=%d crash=%d speed=%.1f\n",
            runUp, int(probe.state().crashReason), probe.state().speed);
        if (runUp == 180) {
            CHECK_BIKE(!probe.state().crashed && probe.state().velocity.length() < 1);
        } else {
            CHECK_BIKE(probe.state().crashReason == DirtBikeDynamics::CrashReason::LostGrip);
        }
    }
    auto grassSlide = landingRoom;
    grassSlide.groundMaterial = SpatialSurfaceMaterial::Grass;
    bike.reset(vehicle);
    ride(bike, {1, 0, 0, false}, grassSlide, 180);
    ride(bike, {1, 1, 0, false}, grassSlide, 36);
    ride(bike, slide, grassSlide, 240);
    // A shorter run-up lets this slide slow safely before the sustained
    // high-speed loss-of-grip criterion. Do not force a crash just for skidding.
    CHECK_BIKE(!bike.state().crashed && bike.state().velocity.length() < 1);
    bike.reset(vehicle);
    ride(bike, {1, 0, 0, false}, grassSlide, 360);
    ride(bike, {1, 1, 0, false}, grassSlide, 36);
    float grassPeakSlip = 0;
    float grassPeakSlideSpeed = 0;
    for (int frame = 0; frame < 240 && !bike.state().crashed; ++frame) {
        bike.advance(slide, 1.f / 60, grassSlide);
        const auto right = SpatialVehicleModel::rotate({1, 0, 0}, bike.state().heading);
        const float sideways = QVector3D::dotProduct(bike.state().velocity, right);
        grassPeakSlip = qMax(grassPeakSlip, std::atan2(std::abs(sideways), std::abs(bike.state().speed) + 100));
        grassPeakSlideSpeed = qMax(grassPeakSlideSpeed, bike.state().velocity.length());
    }
    std::printf("grass_slide reason=%d speed=%.1f peak_slip=%.3f peak_speed=%.1f\n",
        int(bike.state().crashReason), bike.state().speed, grassPeakSlip, grassPeakSlideSpeed);
    CHECK_BIKE(bike.state().crashReason == DirtBikeDynamics::CrashReason::LostGrip);
    bike.reset(vehicle);
    CHECK_BIKE(!bike.state().crashed && bike.state().crashReason == DirtBikeDynamics::CrashReason::None);
    ride(bike, {.3f, .2f, 0, false}, landingRoom, 240);
    CHECK_BIKE(!bike.state().crashed && bike.state().position.length() > 100);
    for (int turns : {-2, -1, 0, 1, 2}) {
        const float pitch = turns * 6.283185307f;
        CHECK_BIKE(DirtBikeDynamics::landingCrashReason(pitch, 0, 0, 900)
            == DirtBikeDynamics::CrashReason::None);
        CHECK_BIKE(DirtBikeDynamics::landingCrashReason(pitch + .3f, 0, .3f, 900)
            == DirtBikeDynamics::CrashReason::None);
        CHECK_BIKE(DirtBikeDynamics::landingCrashReason(pitch + 1.f, 0, 0, 900)
            == DirtBikeDynamics::CrashReason::BadLanding);
        CHECK_BIKE(DirtBikeDynamics::landingCrashReason(pitch, 0, 0, 2300)
            == DirtBikeDynamics::CrashReason::HardLanding);
    }
    CHECK_BIKE(DirtBikeDynamics::landingCrashReason(3.14159265f, 0, 0, 100)
        == DirtBikeDynamics::CrashReason::BadLanding);
    CHECK_BIKE(DirtBikeDynamics::landingCrashReason(0, 1.4f, 0, 100)
        == DirtBikeDynamics::CrashReason::BadLanding);
    CHECK_BIKE(DirtBikeDynamics::landingCrashReason(std::numeric_limits<float>::quiet_NaN(), 0, 0, 100)
        == DirtBikeDynamics::CrashReason::BadLanding);

    // The existing web planner and compiler accept the same bike model.
    SpatialWebDocument document;
    document.url = QUrl("https://example.org/motorcycles");
    document.title = "Motocross inventory";
    document.venue = "showroom";
    SpatialWebItem item;
    item.id = "bike";
    item.title = "250 motocross bike";
    item.kind = "vehicle";
    item.vehicleBody = "dirt-bike";
    item.url = QUrl("https://example.org/motorcycles/motocross-250");
    document.items.append(item);
    const auto world = SpatialWebWorld::construct(document, SpatialWebWorld::optionsFor(document));
    CHECK_BIKE(world.success && !world.fallback && world.interior.vehicles.size() == 1);
    CHECK_BIKE(world.interior.vehicles.first().body == "dirt-bike");

    auto plan = SpatialWebWorld::plan(document, SpatialWebGraph::fromDocument(document), SpatialWebWorld::optionsFor(document));
    SpatialRideSurface proposal;
    proposal.center = {0, 0, 0};
    proposal.width = 200;
    proposal.profile = {{-100, 0}, {0, 30}, {100, 0}};
    plan.rideSurfaces.append(proposal);
    CHECK_BIKE(SpatialWebWorld::compile(plan).success);
    plan.rideSurfaces[0].profile[1].setY(800);
    CHECK_BIKE(!SpatialWebWorld::compile(plan).success);
    plan.rideSurfaces.fill(proposal, SpatialRideSurface::maximumSurfaces + 1);
    CHECK_BIKE(!SpatialWebWorld::compile(plan).success);
    plan.rideSurfaces = {proposal};
    plan.rideSurfaces[0].center.setX(20000);
    CHECK_BIKE(!SpatialWebWorld::compile(plan).success);

    // Streamed destinations retain the same visible/contact ramp after rotation.
    SpatialInterior origin;
    SpatialDisplay entrance;
    entrance.id = "test-door";
    entrance.portal = true;
    entrance.freestanding = true;
    entrance.yaw = 90;
    entrance.portalCenter = {0, 0, 100};
    origin.displays.append(entrance);
    SpatialWebPassage passage;
    CHECK_BIKE(passage.begin(origin, entrance));
    SpatialInterior destination;
    destination.rideSurfaces.append(proposal);
    SpatialMesh arrivalMesh;
    arrivalMesh.vertices = {{0, 0, 0}, {0, 100, 0}, {100, 0, 0}};
    arrivalMesh.faces = {{0, 1, 2}};
    arrivalMesh.normals.fill({0, 0, -1}, 3);
    destination.meshes.append(arrivalMesh);
    CHECK_BIKE(passage.stage(destination));
    const auto streamed = passage.room();
    CHECK_BIKE(streamed.rideSurfaces.size() == 1);
    const auto &transformed = streamed.rideSurfaces.first();
    CHECK_BIKE(std::abs(transformed.heightAt(transformed.center) - 30) < .01);
    CHECK_BIKE(streamed.meshes.size() == 1 && streamed.meshes.first().hasSmoothNormals());
    const auto &arrival = streamed.meshes.first();
    const auto arrivalNormal = QVector3D::crossProduct(arrival.vertices[1] - arrival.vertices[0],
        arrival.vertices[2] - arrival.vertices[0]).normalized();
    for (const auto &normal : arrival.normals)
        CHECK_BIKE(QVector3D::dotProduct(normal, arrivalNormal) > .999f);

    // Sanitizer runs can qualify the deterministic solver/model contracts
    // without turning the wall-clock interactive lane into a timing lottery.
    // The normal test still executes that lane with its original deadlines.
    if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_PHYSICS_ONLY")) {
        std::printf("bike_physics_models: deterministic contracts passed\n");
        return 0;
    }
    const bool hardware = qEnvironmentVariableIsSet("DOCKPIPE_BIKE_HARDWARE");
    QElapsedTimer sceneEntry;
    sceneEntry.start();
    WorkspaceFlightView scene({}, hardware);
    const auto constructedAt = sceneEntry.elapsed();
    if (hardware) {
        // Optional host-GPU qualification creates no visible/focused windows.
        scene.setAttribute(Qt::WA_DontShowOnScreen);
        scene.setAttribute(Qt::WA_ShowWithoutActivating);
    }
    scene.resize(hardware ? QSize(3840, 2160) : QSize(1280, 800));
    CHECK_BIKE(scene.terrainSharp());
    scene.show();
    const auto shownAt = sceneEntry.elapsed();
    scene.setInterior(track, "Mesa Moto · Dirt-bike proving ground");
    const auto preparedAt = sceneEntry.elapsed();
    idle(120);
    const auto settledAt = sceneEntry.elapsed();
    const QString capture = qEnvironmentVariable("DOCKPIPE_BIKE_CAPTURE");
    if (hardware)
        CHECK_BIKE(!capture.isEmpty());
    QVector<double> gpuFrameTimes;
    const auto captureScene = [&](const QString &suffix, bool saveImage = true, QImage *capturedImage = nullptr) {
        if (auto *gl = qobject_cast<QOpenGLWidget *>(scene.viewport())) {
            // A deliberately unmapped widget has no composited window image.
            // Render the same view into an explicit GPU FBO for visual proof.
            gl->makeCurrent();
            QImage image;
            {
                QOpenGLFramebufferObject target(scene.viewport()->size());
                if (target.isValid() && target.bind()) {
                    QOpenGLPaintDevice device(scene.viewport()->size());
                    QPainter painter(&device);
                    QElapsedTimer frameTime;
                    frameTime.start();
                    scene.render(&painter);
                    if (scene.terrainSharp() && !scene.nativeSceneActive()) {
                        std::fprintf(stderr, "Native pass fell back: %s\n", qPrintable(scene.nativeSceneError()));
                        return false;
                    }
                    painter.end();
                    gl->context()->functions()->glFinish();
                    gpuFrameTimes.append(frameTime.nsecsElapsed() / 1e6);
                    if (saveImage)
                        image = target.toImage();
                    target.release();
                }
            }
            gl->doneCurrent();
            if (!saveImage)
                return true;
            if (image.isNull() || image.pixelColor(image.width()/2, image.height()/3)
                == image.pixelColor(image.width()/2, image.height()*3/4))
                return false;
            if (capturedImage)
                *capturedImage = image;
            return image.save(capture + suffix);
        }
        return scene.grab().save(capture + suffix);
    };
    QImage smoothScene;
    if (!capture.isEmpty())
        CHECK_BIKE(captureScene("-pit.png", true, hardware ? &smoothScene : nullptr));
    if (hardware) {
        std::printf("bike_gpu_initial_scene_ms=%lld first_render_ms=%.2f\n",
            static_cast<long long>(sceneEntry.elapsed()), gpuFrameTimes.first());
        std::printf("bike_gpu_entry constructor_ms=%lld show_ms=%lld prepare_ms=%lld events_ms=%lld capture_ms=%lld\n",
            static_cast<long long>(constructedAt), static_cast<long long>(shownAt - constructedAt),
            static_cast<long long>(preparedAt - shownAt), static_cast<long long>(settledAt - preparedAt),
            static_cast<long long>(sceneEntry.elapsed() - settledAt));
        // Verify the full scene admission path preserves authored normals.
        // Discarding them used to make the same hills identically flat shaded.
        auto flatTrack = track;
        for (auto &mesh : flatTrack.meshes)
            mesh.normals.clear();
        scene.setInterior(flatTrack, "Mesa Moto · Dirt-bike proving ground");
        QImage flatScene;
        CHECK_BIKE(captureScene("-flat-slopes.png", true, &flatScene));
        CHECK_BIKE(flatScene.size() == smoothScene.size());
        int changedSlopePixels = 0;
        for (int y = flatScene.height() / 4; y < flatScene.height() * 3 / 4; y += 4) {
            for (int x = 20; x < flatScene.width() / 3; x += 4) {
                const auto smooth = smoothScene.pixelColor(x, y);
                const auto flat = flatScene.pixelColor(x, y);
                if (std::abs(smooth.red() - flat.red()) + std::abs(smooth.green() - flat.green())
                    + std::abs(smooth.blue() - flat.blue()) > 12)
                    ++changedSlopePixels;
            }
        }
        std::printf("bike_smooth_slope_pixels=%d\n", changedSlopePixels);
        CHECK_BIKE(changedSlopePixels > 1000);
        smoothScene = {};
        flatScene = {};
        scene.setInterior(track, "Mesa Moto · Dirt-bike proving ground");
        // Adding a chair or previewing a prop must not demote the whole track
        // to the painter backend. No layout is saved by this preview-only check.
        auto *furnishing = scene.furnishing();
        CHECK_BIKE(furnishing && furnishing->available());
        furnishing->toggle();
        furnishing->selectType("chair");
        CHECK_BIKE(furnishing->ghost().has_value());
        CHECK_BIKE(captureScene("-furnishing.png"));
        furnishing->cancel();
        furnishing->cancel();
        CHECK_BIKE(!furnishing->active());
    }
    if (hardware) {
        // This lane verifies actual GPU pixels in an unmapped framebuffer.
        // Wall-clock interactive timing belongs to the mapped native/raster
        // lane below, not a widget deliberately excluded from window exposure.
        CHECK_BIKE(scene.hardwareAccelerated());
        CHECK_BIKE(scene.terrainSharp());
        std::printf("bike_native_scene=%d error=%s\n", scene.nativeSceneActive(), qPrintable(scene.nativeSceneError()));
        CHECK_BIKE(scene.nativeSceneActive());
        if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_REDBUD_CAPTURE")) {
            auto redbud = SpatialBikeTrack::createRedBud();
            CHECK_BIKE(redbud.terrain);
            scene.setInterior(redbud, "RedBud · 2024 map study");
            CHECK_BIKE(scene.enterVehicle("mesa-bike"));
            CHECK_BIKE(scene.startBikeRace());
            CHECK_BIKE(captureScene("-redbud-dry.png"));
            scene.setPilotVehicleControls(.6f, 0);
            idle(4500);
            CHECK_BIKE(scene.terrain()->wornCells() > 0 && !scene.bikeCrashed());
            CHECK_BIKE(captureScene("-redbud-race.png"));
            gpuFrameTimes.clear();
            QVector<double> updates;
            for (int frame = 0; frame < 36; ++frame) {
                idle(20);
                updates.append((scene.lastBikeUpdateNanoseconds() + scene.lastTerrainUpdateNanoseconds()) / 1e6);
                CHECK_BIKE(captureScene({}, false));
            }
            std::sort(gpuFrameTimes.begin(), gpuFrameTimes.end());
            std::sort(updates.begin(), updates.end());
            std::printf("redbud_native frames=36 median_ms=%.2f p95_ms=%.2f terrain_and_bike_p95_ms=%.2f\n",
                gpuFrameTimes[18], gpuFrameTimes[34], updates[34]);
            scene.suspendPilot();
            redbud.terrain = std::make_shared<SpatialTerrain>(*scene.terrain());
            redbud.terrain->setRaining(true);
            for (int tick = 0; tick < 1800; ++tick)
                redbud.terrain->advance(.05f);
            for (int pass = 0; pass < 24; ++pass) {
                for (int distance = 100; distance < 1800; distance += 10) {
                    const auto point = DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start + distance, -150,
                        DirtBikeRaceCourse::Layout::RedBud);
                    const QVector3D forward(std::sin(point.heading),0,std::cos(point.heading));
                    redbud.terrain->wheelContact(point.position, forward, 10, .6f, .3f);
                }
            }
            scene.setInterior(redbud, "RedBud · Wet soil inspection");
            CHECK_BIKE(scene.enterVehicle("mesa-bike"));
            CHECK_BIKE(scene.startBikeRace());
            CHECK_BIKE(scene.terrainRaining() && scene.terrain()->totalWater() > 0);
            CHECK_BIKE(captureScene("-redbud-wet.png"));
            const float wetness = scene.terrain()->wetness();
            scene.toggleTerrainRain();
            CHECK_BIKE(!scene.terrainRaining() && scene.terrain()->wetness() == wetness);
            scene.suspendPilot();
            idle(200);
            CHECK_BIKE(scene.terrain()->wetness() == wetness);
            CHECK_BIKE(captureScene("-redbud-ruts.png"));
            std::printf("redbud_native: dry/wet terrain, physical tyre wear, rain toggle, pause, three riders and native admission passed\n");
            return 0;
        }
        if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_STUNTS_CAPTURE")) {
            auto stuntRoom = track;
            stuntRoom.vehicles.first().position = {0, 0, -7000};
            stuntRoom.spawn = {0, -7250};
            scene.setInterior(stuntRoom, "Mesa Moto · Wheelie inspection");
            CHECK_BIKE(scene.enterVehicle("mesa-bike"));
            scene.setPilotVehicleControls(1, 0);
            scene.setPilotInput({}, {.8, 0}, false);
            idle(1000);
            scene.setPilotInput({0, 0, -1}, {}, false);
            QElapsedTimer lift;
            lift.start();
            while (!scene.bikeCrashed() && lift.elapsed() < 2200
                && !(scene.bikeState().pitch > .35f && scene.bikeState().frontSupport < .03f))
                idle(20);
            CHECK_BIKE(!scene.bikeCrashed() && scene.bikeState().pitch > .35f);
            CHECK_BIKE(scene.bikeState().frontSupport < .03f && scene.bikeState().rearSupport > .2f);
            CHECK_BIKE(captureScene("-wheelie.png"));
            scene.setPilotVehicleControls(0, 0);
            scene.setPilotInput({}, {}, false);
            scene.setPilotFlightControls(0, true);
            idle(800);
            scene.suspendPilot();
            CHECK_BIKE(!scene.bikeCrashed() && scene.bikeState().frontSupport > .1f);
            CHECK_BIKE(captureScene("-wheelie-lowered.png"));
            scene.setInterior(track, "Mesa Moto · Race inspection");
            CHECK_BIKE(scene.enterVehicle("mesa-bike") && scene.bikeRaceAvailable());
            CHECK_BIKE(scene.startBikeRace());
            CHECK_BIKE(scene.vehicles().size() == 3);
            CHECK_BIKE(captureScene("-race-grid.png"));
            const auto grid = scene.bikeRace().rivals()[0].vehicle.position;
            const auto playerGrid = scene.bikeState().position;
            idle(2000);
            CHECK_BIKE((scene.bikeState().position - playerGrid).length() < .5f);
            CHECK_BIKE(!scene.bikeState().reversing);
            idle(1300);
            CHECK_BIKE(scene.bikeRace().phase() == DirtBikeRace::Phase::Racing);
            scene.setPilotVehicleControls(.65f, 0);
            idle(1800);
            CHECK_BIKE((scene.bikeRace().rivals()[0].vehicle.position - grid).length() > 300);
            CHECK_BIKE(!scene.bikeCrashed());
            CHECK_BIKE(captureScene("-race.png"));
            gpuFrameTimes.clear();
            QVector<double> updates;
            for (int frame = 0; frame < 36; ++frame) {
                idle(20);
                updates.append(scene.lastBikeUpdateNanoseconds() / 1e6);
                CHECK_BIKE(captureScene({}, false));
            }
            std::sort(gpuFrameTimes.begin(), gpuFrameTimes.end());
            std::sort(updates.begin(), updates.end());
            std::printf("bike_race_native frames=36 median_ms=%.2f p95_ms=%.2f update_p95_ms=%.2f\n",
                gpuFrameTimes[18], gpuFrameTimes[34], updates[34]);
            if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_RIVAL_CORNERS")) {
                DirtBikeRivalDriver cameraRider;
                cameraRider.reset(0, 19);
                QElapsedTimer cornerWait;
                cornerWait.start();
                bool cornered = false;
                while (cornerWait.elapsed() < 20000 && !scene.bikeCrashed()) {
                    auto sensedRoom = track;
                    sensedRoom.vehicles = scene.vehicles();
                    const auto controls = cameraRider.controls(scene.bikeState(), .02f, sensedRoom, "mesa-bike");
                    scene.setPilotVehicleControls(controls.throttle, controls.serviceBrake);
                    scene.setPilotInput({controls.steering, 0, -controls.pitch}, {}, false);
                    idle(20);
                    for (const auto &rival : scene.bikeRace().rivals()) {
                        const auto &state = rival.bike.state();
                        const float along = DirtBikeRaceCourse::progress(state.position);
                        if (along > DirtBikeRaceCourse::straight + 1000
                            && along < DirtBikeRaceCourse::straight + DirtBikeRaceCourse::turn - 1000
                            && std::abs(state.lean) > .15f && !state.crashed) {
                            std::printf("bike_rival_native_corner id=%s speed=%.1f lean=%.3f steering=%.3f\n",
                                qPrintable(rival.vehicle.id), state.speed, state.lean, state.steeringAngle);
                            cornered = true;
                            break;
                        }
                    }
                    if (cornered)
                        break;
                }
                CHECK_BIKE(cornered && !scene.bikeCrashed());
                CHECK_BIKE(captureScene("-rival-corner.png"));
            }
            scene.suspendPilot();
            const float paused = scene.bikeRace().elapsed();
            idle(200);
            CHECK_BIKE(scene.bikeRace().elapsed() == paused);
            CHECK_BIKE(scene.startBikeRace() && scene.vehicles().size() == 3);
            scene.suspendPilot();
            std::printf("bike_stunts_native: wheelie/brake and two-rival race passed\n");
            return 0;
        }
        if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_STEERING_INSPECTION")) {
            auto steeringRoom = track;
            steeringRoom.vehicles.first().position = {0, 0, -7000};
            steeringRoom.spawn = {0, -7250};
            scene.setInterior(steeringRoom, "Mesa Moto · Steering assembly inspection");
            CHECK_BIKE(scene.enterVehicle("mesa-bike"));
            for (bool chase : {true, false}) {
                if (scene.bikeChaseCamera() != chase)
                    scene.toggleBikeCamera();
                QImage neutral;
                for (int input : {0, -1, 1}) {
                    scene.resetBike();
                    scene.setPilotInput({float(input), 0, 0}, {}, false);
                    idle(500);
                    scene.suspendPilot();
                    const auto position = scene.vehicles().first().position;
                    CHECK_BIKE((position - steeringRoom.vehicles.first().position).length() < .5f);
                    QImage image;
                    const QString view = chase ? "-steering-chase-" : "-steering-rider-";
                    QString direction = "neutral";
                    if (input < 0)
                        direction = "left";
                    else if (input > 0)
                        direction = "right";
                    CHECK_BIKE(captureScene(view + direction + ".png", true, &image));
                    if (input == 0) {
                        neutral = image;
                    } else {
                        int changed = 0;
                        for (int y = image.height() / 2; y < image.height(); y += 2) {
                            for (int x = image.width() / 5; x < image.width() * 4 / 5; x += 2) {
                                const auto a = image.pixelColor(x, y);
                                const auto b = neutral.pixelColor(x, y);
                                if (std::abs(a.red() - b.red()) + std::abs(a.green() - b.green())
                                    + std::abs(a.blue() - b.blue()) > 20)
                                    ++changed;
                            }
                        }
                        std::printf("bike_steering_pixels chase=%d input=%d changed=%d\n", chase, input, changed);
                        CHECK_BIKE(changed > 1000);
                    }
                }
                gpuFrameTimes.clear();
                for (int frame = 0; frame < 36; ++frame)
                    CHECK_BIKE(captureScene({}, false));
                std::sort(gpuFrameTimes.begin(), gpuFrameTimes.end());
                std::printf("bike_steering_render chase=%d frames=36 median_ms=%.2f p95_ms=%.2f\n",
                    chase, gpuFrameTimes[18], gpuFrameTimes[34]);
            }
            // Inspect the wheel/forks and grips together from in front of
            // the parked bike, without adding an alternate pose/render path.
            if (!scene.bikeChaseCamera())
                scene.toggleBikeCamera();
            for (int input : {0, -1, 1}) {
                scene.resetBike();
                scene.setPilotInput({}, {1, 0}, false);
                idle(1250);
                scene.suspendPilot();
                scene.setPilotInput({float(input), 0, 0}, {}, false);
                idle(500);
                scene.suspendPilot();
                QString direction = "neutral";
                if (input < 0)
                    direction = "left";
                else if (input > 0)
                    direction = "right";
                CHECK_BIKE(captureScene("-steering-front-" + direction + ".png"));
            }
            std::printf("bike_steering_native: left/neutral/right assembly captures passed\n");
            return 0;
        }
        auto cornerRoom = track;
        cornerRoom.vehicles.first().position = {0, 0, -7000};
        cornerRoom.spawn = {0, -7250};
        scene.setInterior(cornerRoom, "Mesa Moto · Cornering inspection");
        CHECK_BIKE(scene.enterVehicle("mesa-bike"));
        CHECK_BIKE(captureScene("-rider.png"));
        // View-only orbit shows the same articulated rider from the side.
        // No throttle or steering is applied during this inspection.
        for (int frame = 0; frame < 42; ++frame) {
            scene.setPilotInput({}, {1, .12}, false);
            idle(20);
        }
        scene.suspendPilot();
        CHECK_BIKE(captureScene("-rider-detail.png"));
        scene.setInterior(cornerRoom, "Mesa Moto · Bodywork inspection");
        scene.streamInterior(cornerRoom, "Mesa Moto · Bodywork inspection", {-300, 125, -7230}, .92f);
        CHECK_BIKE(captureScene("-bike-detail.png"));
        scene.setInterior(cornerRoom, "Mesa Moto · Cornering inspection");
        CHECK_BIKE(scene.enterVehicle("mesa-bike"));
        const auto turnStart = scene.vehicles().first();
        for (int frame = 0; frame < 100; ++frame) {
            scene.setPilotVehicleControls(.6f, 0);
            scene.setPilotInput({.6f, 0, 0}, {.3, 0}, false);
            idle(20);
        }
        CHECK_BIKE(!scene.bikeCrashed());
        CHECK_BIKE(std::abs(scene.vehicles().first().heading - turnStart.heading) > .02f);
        CHECK_BIKE(captureScene("-corner.png"));
        scene.suspendPilot();
        if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_BENCHMARK")) {
            gpuFrameTimes.clear();
            for (int frame = 0; frame < 36; ++frame) {
                scene.setPilotVehicleControls(.3f, 0);
                scene.setPilotInput({.15f, 0, 0}, {.8, 0}, false);
                idle(20);
                CHECK_BIKE(captureScene({}, false));
            }
            scene.suspendPilot();
            std::sort(gpuFrameTimes.begin(), gpuFrameTimes.end());
            std::printf("bike_gpu_turn_frames=%lld median_ms=%.2f p95_ms=%.2f\n",
                static_cast<long long>(gpuFrameTimes.size()), gpuFrameTimes[gpuFrameTimes.size()/2],
                gpuFrameTimes[gpuFrameTimes.size()*95/100]);
        }
        if (qEnvironmentVariableIsSet("DOCKPIPE_BIKE_SPIN_BENCHMARK")) {
            auto spinRoom = track;
            spinRoom.vehicles.first().position = {0, 0, -7000};
            spinRoom.spawn = {0, -7250};
            scene.setInterior(spinRoom, "Spin-out regression");
            CHECK_BIKE(scene.enterVehicle("mesa-bike"));
            for (int frame = 0; frame < 280; ++frame) {
                scene.setPilotVehicleControls(1, 0);
                scene.setPilotInput({}, {}, false);
                idle(20);
            }
            CHECK_BIKE(!scene.bikeCrashed());
            gpuFrameTimes.clear();
            QVector<double> updateTimes;
            // Surface grip changes how long a deliberate slide takes to
            // fall. Keep at least 90 measured frames and cap the attempt at
            // 180, then still require an actual crash and settled wreck.
            for (int frame = 0; frame < 180 && (frame < 90 || !scene.bikeCrashed()); ++frame) {
                scene.setPilotVehicleControls(1, 0);
                scene.setPilotInput({1, 0, 0}, {}, false);
                scene.setPilotFlightControls(0, frame >= 30);
                idle(20);
                updateTimes.append(scene.lastBikeUpdateNanoseconds() / 1e6);
                CHECK_BIKE(captureScene({}, false));
            }
            std::sort(gpuFrameTimes.begin(), gpuFrameTimes.end());
            std::sort(updateTimes.begin(), updateTimes.end());
            const int measuredFrames = gpuFrameTimes.size();
            std::printf("bike_gpu_spin crashed=%d frames=%d render_median_ms=%.2f render_p95_ms=%.2f render_max_ms=%.2f update_p95_ms=%.2f\n",
                scene.bikeCrashed(), measuredFrames, gpuFrameTimes[measuredFrames / 2],
                gpuFrameTimes[measuredFrames * 95 / 100], gpuFrameTimes.last(),
                updateTimes[measuredFrames * 95 / 100]);
            CHECK_BIKE(scene.bikeCrashed());
            scene.suspendPilot();
            CHECK_BIKE(captureScene("-spin.png"));
            scene.setPilotInput({}, {}, false);
            idle(2800);
            CHECK_BIKE(scene.bikeCrashed() && !scene.moving());
            CHECK_BIKE(captureScene("-settled.png"));
        }
        // streamInterior preserves look; reset it so earlier camera input
        // cannot accidentally point a benchmark/capture away from the track.
        scene.setInterior(track, "Mesa Moto · Banked turn inspection");
        scene.streamInterior(track, "Mesa Moto · Banked turn inspection", {-3600, 700, 9500}, .9f);
        CHECK_BIKE(captureScene("-berm.png"));
        scene.setInterior(track, "Mesa Moto · Placement inspection");
        scene.streamInterior(track, "Mesa Moto · Placement inspection", {-3000, 160, -4500}, -1.5707963f);
        CHECK_BIKE(captureScene("-placement.png"));
        scene.setInterior(track, "Mesa Moto · Pit shelter inspection");
        scene.streamInterior(track, "Mesa Moto · Pit shelter inspection", {-5300, 240, -9950}, 0);
        CHECK_BIKE(captureScene("-shelter.png"));
        scene.setInterior(track, "Mesa Moto · GPU overview");
        scene.streamInterior(track, "Mesa Moto · GPU overview", {0, 1800, -11500}, 0);
        for (int frame = 0; frame < 8; ++frame)
            CHECK_BIKE(captureScene("-overview.png"));
        std::printf("bike_gpu_visual_renderer=%s draw_submission_ms=%.2f\n",
            qPrintable(scene.rendererDescription()), scene.lastDrawNanoseconds()/1e6);
        scene.setTerrainSharp(false);
        CHECK_BIKE(!scene.terrainSharp());
        for (int frame = 0; frame < 8; ++frame)
            CHECK_BIKE(captureScene("-smooth.png"));
        std::printf("bike_gpu_smooth_draw_ms=%.2f\n", scene.lastDrawNanoseconds()/1e6);
        return 0;
    }
    // A turn's approximately 48 MiB of material variants must retain image
    // identity (and therefore reusable GPU uploads), while larger working
    // sets still evict old entries rather than growing without a bound.
    const QColor cacheTint(32, 32, 32);
    const auto resident = SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Grass, cacheTint, 0);
    const auto fillMaterials = [](int begin, int end) {
        for (int index = begin; index < end; ++index) {
            const QColor tint(64 + (index % 8) * 16, 64 + (index / 8) * 16, 128);
            SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Grass, tint, 0);
        }
    };
    fillMaterials(0, 48);
    CHECK_BIKE(SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Grass, cacheTint, 0).cacheKey()
        == resident.cacheKey());
    fillMaterials(0, 80);
    CHECK_BIKE(SpatialSurfaceTextures::tile(SpatialSurfaceMaterial::Grass, cacheTint, 0).cacheKey()
        != resident.cacheKey());
    CHECK_BIKE(scene.enterVehicle("mesa-bike"));
    CHECK_BIKE(scene.ridingBike() && scene.bikeChaseCamera());
    CHECK_BIKE(scene.cameraPosition().z() < scene.vehicles().first().position.z() - 300);
    const auto parkedBike = scene.vehicles().first();
    scene.setPilotInput({}, {1, .4}, false);
    idle(450);
    scene.setPilotInput({}, {}, false);
    idle(500);
    CHECK_BIKE(std::abs(scene.cameraPosition().x() - parkedBike.position.x()) > 100);
    CHECK_BIKE(scene.vehicles().first().heading == parkedBike.heading);
    CHECK_BIKE((scene.vehicles().first().position - parkedBike.position).length() < 20);
    CHECK_BIKE(scene.moving()); // Return delay must tick even on a parked bike.
    scene.suspendPilot();
    const auto pausedCamera = scene.cameraPosition();
    idle(100);
    CHECK_BIKE(!scene.moving() && scene.cameraPosition() == pausedCamera);
    scene.setPilotInput({}, {}, false);
    idle(4000);
    CHECK_BIKE(std::abs(scene.cameraPosition().x() - parkedBike.position.x()) < 5);
    CHECK_BIKE(!scene.moving());
    scene.toggleBikeCamera();
    CHECK_BIKE(!scene.bikeChaseCamera());
    scene.toggleBikeCamera();
    scene.toggleBikeSound();
    CHECK_BIKE(!scene.bikeSoundEnabled());
    scene.toggleBikeSound();
    scene.setPilotVehicleControls(1, 0);
    scene.setPilotInput({}, {}, false);
    idle(1100);
    std::printf("bike_interactive_z=%.1f last_draw_ms=%.2f renderer=%s\n", scene.vehicles().first().position.z(),
        scene.lastDrawNanoseconds()/1e6, qPrintable(scene.rendererDescription()));
    CHECK_BIKE(scene.vehicles().first().position.z() > -8200);
    CHECK_BIKE(!scene.exitVehicle());
    scene.setPilotVehicleControls(0, 1);
    scene.setPilotInput({}, {}, false);
    idle(1000);
    CHECK_BIKE(scene.exitVehicle());
    CHECK_BIKE(!scene.ridingBike());
    CHECK_BIKE(scene.enterVehicle("mesa-bike"));
    QKeyEvent recover(QEvent::KeyPress, Qt::Key_R, Qt::NoModifier);
    QApplication::sendEvent(&scene, &recover);
    CHECK_BIKE(!scene.moving());
    CHECK_BIKE(scene.vehicles().first().position == track.vehicles.first().position);
    // Replacing the scene clears the bike session and its recovery point.
    scene.setInterior(track, "Mesa Moto · Dirt-bike proving ground");
    CHECK_BIKE(!scene.ridingBike());
    CHECK_BIKE(scene.enterVehicle("mesa-bike"));
    idle(100);
    if (!capture.isEmpty())
        CHECK_BIKE(captureScene("-rider.png"));
    if (!capture.isEmpty()) {
        scene.streamInterior(track, "Mesa Moto · Bike inspection", {-3100, 160, -8650}, .95f);
        idle(100);
        CHECK_BIKE(captureScene("-bike.png"));
        scene.streamInterior(track, "Mesa Moto · Track overview", {0, 1800, -11500}, -.95f);
        idle(100);
        CHECK_BIKE(captureScene("-overview.png"));
        std::printf("bike_render_last_ms=%.2f\n", scene.lastDrawNanoseconds() / 1e6);
    }
    auto collisionTrack = track;
    collisionTrack.solids.append({{-2700, 350, -7000}, {1600, 700, 20}, Qt::gray});
    scene.setInterior(collisionTrack, "Crash recovery fixture");
    CHECK_BIKE(scene.enterVehicle("mesa-bike"));
    if (scene.bikeChaseCamera())
        scene.toggleBikeCamera();
    scene.setPilotVehicleControls(1, 0);
    QElapsedTimer crashWait;
    crashWait.start();
    while (!scene.bikeCrashed() && crashWait.elapsed() < 4000)
        idle(20);
    CHECK_BIKE(scene.bikeCrashed());
    scene.suspendPilot();
    const auto pausedWreck = scene.vehicles().first().position;
    idle(100);
    CHECK_BIKE(!scene.moving() && scene.vehicles().first().position == pausedWreck);
    scene.setPilotInput({}, {}, false);
    idle(2800);
    CHECK_BIKE(scene.bikeCrashed() && scene.bikeChaseCamera() && !scene.moving());
    if (!capture.isEmpty())
        CHECK_BIKE(captureScene("-crash.png"));
    const auto settled = scene.vehicles().first().position;
    scene.setPilotVehicleControls(1, 0);
    scene.setPilotInput({1, 0, 1}, {}, true);
    idle(100);
    CHECK_BIKE(!scene.moving() && scene.vehicles().first().position == settled);
    scene.resetBike();
    CHECK_BIKE(!scene.bikeCrashed() && !scene.bikeChaseCamera());
    CHECK_BIKE(scene.vehicles().first().position == track.vehicles.first().position);
    scene.setInterior({}, "Replacement world");
    CHECK_BIKE(!scene.ridingBike() && !scene.moving());
    if (QApplication::platformName() != "offscreen") {
        SpatialWebBrowser browser(false);
        browser.resize(1280, 800);
        browser.show();
        CHECK_BIKE(!browser.findChild<QPushButton *>("spatialBikeTrack"));
        browser.showHeadquarters();
        CHECK_BIKE(!browser.scene()->ridingBike());
        browser.scene()->destinationActivated("civic:track", false);
        CHECK_BIKE(!browser.scene()->enterVehicle("mesa-bike"));
    }
    return 0;
}
