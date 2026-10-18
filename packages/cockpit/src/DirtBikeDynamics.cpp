#include "DirtBikeDynamics.h"
#include "DirtBikeTyre.h"
#include "SpatialTerrain.h"
#include "DirtBikeSteeringRig.h"
#include "DirtBikeTyreContact.h"
#include "DirtBikeSuspension.h"
#include "SpatialWorkspace.h"
#include <cmath>

namespace {
constexpr float stepSeconds = 1.f / 120;
constexpr float gravity = DirtBikeSuspension::gravity;
constexpr float halfWheelbase = DirtBikeSuspension::halfWheelbase;
constexpr float yawInertia = halfWheelbase * halfWheelbase * 1.3f;
constexpr float fullTurn = 6.283185307f;
constexpr float wheelRadius = DirtBikeTyreContact::radius;
constexpr float gearSpeed[] = {620, 980, 1400, 1900, 2400, 3000};

float tyreHeight(const SpatialInterior &room, const QVector3D &point, const QVector3D &forward)
{
    return DirtBikeTyreContact::supportHeight([&](float offset) {
        return DirtBikeDynamics::groundHeight(room, point + forward * offset);
    });
}

float angleError(float angle, float reference)
{
    return std::remainder(angle - reference, fullTurn);
}

// Allocate longitudinal and lateral force together. Giving engine force first
// priority used to consume all rear grip and make a powered turn unstable.
float combinedGripScale(float longitudinal, float lateral, float grip, float lateralGrip)
{
    if (grip <= 0 || lateralGrip <= 0)
        return 0;
    const float demand = std::hypot(longitudinal / grip, lateral / lateralGrip);
    // Contact relaxation smooths force onset. Allocate the unsaturated elastic
    // demand here; clipping lateral force first starves rear cornering grip.
    return 1 / qMax(1.f, demand);
}

float surfaceGrip(SpatialSurfaceMaterial material)
{
    // Assisted dry-surface tuning, not measured tyre coefficients. Prepared
    // dirt keeps the established handling; grass is only modestly less grippy.
    if (material == SpatialSurfaceMaterial::Grass)
        return .88f;
    if (material == SpatialSurfaceMaterial::Rock)
        return .96f;
    return 1;
}

float axis(float value)
{
    return std::isfinite(value) ? qBound(-1.f, value, 1.f) : 0;
}
}

void DirtBikeDynamics::reset(const SpatialVehicle &vehicle)
{
    m_state = {};
    m_state.position = vehicle.position;
    m_state.heading = vehicle.heading;
    m_id = vehicle.id;
    m_remainder = 0;
    m_airTime = 0;
    m_reverseHold = 0;
    m_gripRiskTime = 0;
    m_fallSide = 1;
}

float DirtBikeDynamics::groundHeight(const SpatialInterior &room, const QVector3D &point)
{
    if (room.terrain)
        return room.terrain->sample(point).height;
    float height = 0;
    const int count = qMin(int(room.rideSurfaces.size()), SpatialRideSurface::maximumSurfaces);
    for (int index = 0; index < count; ++index)
        height = qMax(height, room.rideSurfaces[index].heightAt(point));
    return height;
}

SpatialSurfaceMaterial DirtBikeDynamics::groundMaterial(const SpatialInterior &room, const QVector3D &point)
{
    if (room.terrain)
        return room.terrain->sample(point).soil > .25f ? SpatialSurfaceMaterial::Dirt : SpatialSurfaceMaterial::Grass;
    // Ride surfaces are authored dirt lanes/ramps. Even a zero-height lane
    // overrides the surrounding ground material inside its admitted footprint.
    const int count = qMin(int(room.rideSurfaces.size()), SpatialRideSurface::maximumSurfaces);
    for (int index = 0; index < count; ++index) {
        if (room.rideSurfaces[index].heightAt(point) >= 0)
            return SpatialSurfaceMaterial::Dirt;
    }
    return room.groundMaterial;
}

bool DirtBikeDynamics::unsettled() const
{
    if (m_state.crashed)
        return m_state.crashTime < crashDuration || !m_state.grounded;
    return !m_state.grounded || m_state.velocity.lengthSquared() > 1
        || std::abs(m_state.pitchRate) > .01f || std::abs(m_state.yawRate) > .01f
        || std::abs(m_state.leanRate) > .01f || std::abs(m_state.lean) > .01f
        || std::abs(m_state.steeringAngle) > .001f;
}

void DirtBikeDynamics::advance(const Input &input, float seconds, const SpatialInterior &room)
{
    if (!std::isfinite(seconds) || seconds <= 0)
        return;
    m_state.landingImpact = 0;
    // At most six steps per frame. A stalled/hidden UI cannot accumulate debt.
    m_remainder += qMin(seconds, .05f);
    for (int count = 0; m_remainder + 1e-9 >= stepSeconds && count < 6; ++count) {
        if (m_state.crashed)
            advanceCrash(room);
        else
            step(input, room);
        m_remainder -= stepSeconds;
    }
    m_remainder = qBound(0.0, m_remainder, double(stepSeconds));
}

void DirtBikeDynamics::step(const Input &input, const SpatialInterior &room)
{
    auto &state = m_state;
    const auto forward = SpatialVehicleModel::rotate({0, 0, 1}, state.heading);
    const auto right = SpatialVehicleModel::rotate({1, 0, 0}, state.heading);
    state.speed = QVector3D::dotProduct(state.velocity, forward);
    // Steering changes the front wheel's rolling footprint as well as its
    // lateral force. Update it before sampling contact, with a bounded forecast
    // for the footprint's ground-relative damping over the next solver step.
    const float speedSquared = state.speed * state.speed;
    const float steeringLimit = qMin(.48f, std::atan2(.9f * gravity * halfWheelbase * 2,
        qMax(1.f, speedSquared)));
    const float targetSteering = axis(input.steering) * steeringLimit;
    state.steeringAngle += (targetSteering - state.steeringAngle) * 12 * stepSeconds;
    const float nextSteering = state.steeringAngle + (targetSteering - state.steeringAngle) * 12 * stepSeconds;
    const auto frontDirection = SpatialVehicleModel::rotate({0, 0, 1}, state.heading + DirtBikeSteeringRig::rollingYaw(state.steeringAngle));
    const float pitchLever = DirtBikeSuspension::pitchLever(state.pitch);
    const auto frontPoint = state.position + forward * pitchLever;
    const auto rearPoint = state.position - forward * pitchLever;
    const float frontHeight = tyreHeight(room, frontPoint, frontDirection);
    const float rearHeight = tyreHeight(room, rearPoint, forward);
    const float pitchOffset = std::sin(state.pitch) * halfWheelbase;
    state.frontCompression = frontHeight - state.position.y() - pitchOffset;
    state.rearCompression = rearHeight - state.position.y() + pitchOffset;
    // Ground-relative damping includes axle motion from pitch and yaw, not
    // just chassis translation. Projected contacts shorten during nose lift.
    const QVector3D horizontalStep(state.velocity.x() * stepSeconds, 0, state.velocity.z() * stepSeconds);
    const auto axleMotion = (right * (state.yawRate * pitchLever)
        - forward * (std::sin(state.pitch) * halfWheelbase * state.pitchRate)) * stepSeconds;
    const auto rollingDirection = SpatialVehicleModel::rotate({0, 0, 1}, state.heading + state.yawRate * stepSeconds);
    const auto frontRollingDirection = SpatialVehicleModel::rotate({0, 0, 1},
        state.heading + state.yawRate * stepSeconds + DirtBikeSteeringRig::rollingYaw(nextSteering));
    const float frontGroundRate = (tyreHeight(room, frontPoint + horizontalStep + axleMotion, frontRollingDirection) - frontHeight) / stepSeconds;
    const float rearGroundRate = (tyreHeight(room, rearPoint + horizontalStep - axleMotion, rollingDirection) - rearHeight) / stepSeconds;
    const float frontForce = DirtBikeSuspension::support(state.frontCompression,
        state.velocity.y() + state.pitchRate * pitchLever - frontGroundRate);
    const float rearForce = DirtBikeSuspension::support(state.rearCompression,
        state.velocity.y() - state.pitchRate * pitchLever - rearGroundRate);
    state.frontSupport = frontForce / gravity;
    state.rearSupport = rearForce / gravity;
    state.grounded = frontForce > 0 || rearForce > 0;
    // Keep the terrain reference within +/-90 degrees even while the axles
    // exchange places during a flip; upside-down wheels are not upright ground.
    const float groundPitch = std::abs(pitchLever) > 1
        ? std::atan((frontHeight - rearHeight) / (pitchLever * 2)) : 0;
    const float leftHeight = groundHeight(room, state.position - right * 35);
    const float rightHeight = groundHeight(room, state.position + right * 35);
    const float groundRoll = std::atan2(rightHeight - leftHeight, 70.f);
    if (!state.grounded) {
        m_airTime += stepSeconds;
    } else if (m_airTime > .12f) {
        const float impactSpeed = qMax(0.f, (frontGroundRate + rearGroundRate) * .5f - state.velocity.y());
        state.landingImpact = qMax(state.landingImpact, qMin(1.f, impactSpeed / 1800));
        m_airTime = 0;
        const auto landing = landingCrashReason(state.pitch, state.lean, groundPitch, impactSpeed, groundRoll);
        if (landing != CrashReason::None) {
            crash(landing);
            return;
        }
    } else {
        m_airTime = 0;
    }
    const float rawThrottle = axis(input.throttle);
    const float brakeDemand = input.brake ? 1.f
        : qMax(qMax(0.f, -rawThrottle), qMax(0.f, axis(input.serviceBrake)));
    const bool reverseHeld = brakeDemand >= .2f && rawThrottle <= 0
        && !input.brake && !input.rearBrake && state.grounded;
    const bool wasReversing = state.reversing;
    if (!reverseHeld || state.speed > 15) {
        m_reverseHold = 0;
        state.reversing = false;
    } else if (!state.reversing) {
        const float horizontalSpeed = std::hypot(state.velocity.x(), state.velocity.z());
        m_reverseHold = horizontalSpeed < 8 ? m_reverseHold + stepSeconds : 0;
        state.reversing = m_reverseHold >= .75f;
    }
    // Brake and reverse share LT / S. Rear brake never engages reverse, and
    // overlapping throttle/brake remains a stop request. This is a walking-
    // pace push assist, not a fictitious reverse gear on a motocross engine.
    const float requestedThrottle = state.reversing ? -brakeDemand : qMax(0.f, rawThrottle);
    if (wasReversing && !state.reversing)
        state.throttle = 0;
    state.throttle += (requestedThrottle - state.throttle) * 16 * stepSeconds;
    if (std::abs(state.throttle) < .0001f)
        state.throttle = 0;
    const float throttle = state.throttle;
    const float previousWeight = state.riderWeight;
    state.riderWeight += (axis(input.pitch) - state.riderWeight) * 8 * stepSeconds;
    const float pullRate = qMax(0.f, (state.riderWeight - previousWeight) / stepSeconds);
    // Automatic clutch and six gears: hysteresis prevents hunting, and the
    // short torque interruption is shared with the HUD and exhaust RPM.
    state.shiftTime = qMax(0.f, state.shiftTime - stepSeconds);
    float coupledRpm = 1600 + std::abs(state.speed) * 7200 / gearSpeed[state.gear - 1];
    if (state.grounded && state.shiftTime == 0) {
        if (coupledRpm > 8700 && state.gear < 6) {
            ++state.gear;
            state.shiftTime = .12f;
        } else if (coupledRpm < 4600 && state.gear > 1) {
            --state.gear;
            state.shiftTime = .12f;
        }
    }
    coupledRpm = 1600 + std::abs(state.speed) * 7200 / gearSpeed[state.gear - 1];
    const float freeRpm = 1600 + qMax(0.f, throttle) * 8200;
    const float targetRpm = state.grounded ? qMax(coupledRpm, 1600 + qMax(0.f, throttle) * 2200) : freeRpm;
    state.engineRpm += qBound(-20000.f * stepSeconds, targetRpm - state.engineRpm, 14000.f * stepSeconds);
    state.engineRpm = qBound(1600.f, state.engineRpm, 10500.f);
    float acceleration = 0;
    if (state.grounded) {
        // Suspension forces carry both weight transfer and terrain unloading.
        // A nearly airborne wheel cannot retain its full static grip. Keep the
        // displayed load split smooth, but use actual support for tyre forces.
        const float targetLoad = frontForce / (frontForce + rearForce);
        state.frontLoad += (targetLoad - state.frontLoad) * 10 * stepSeconds;
        const float bankLoad = 1 / qMax(.6f, std::cos(groundRoll));
        // Bound transient bottom-out capacity to twice the whole static weight
        // per axle; one stiff impact cannot create an arbitrarily strong tyre.
        const float frontGrip = qMin(frontForce, gravity * 2) * bankLoad * 1.45f
            * surfaceGrip(groundMaterial(room, frontPoint))
            * (room.terrain ? room.terrain->sample(frontPoint).grip : 1.f);
        const float rearGrip = qMin(rearForce, gravity * 2) * bankLoad * 1.35f
            * surfaceGrip(groundMaterial(room, rearPoint))
            * (room.terrain ? room.terrain->sample(rearPoint).grip : 1.f);
        const float braking = state.reversing ? 0.f : brakeDemand;
        const float frontBrake = qMin(frontGrip, braking * 1450 * .72f);
        const float rearBrake = qMin(rearGrip, braking * 1450 * .28f + (input.rearBrake ? 700.f : 0.f));
        const float rpmOffset = (state.engineRpm - 6800) / 6800;
        const float torque = (1 - .24f * rpmOffset * rpmOffset) * (1 - .035f * (state.gear - 1));
        const float shift = state.shiftTime > 0 ? .35f : 1.f;
        float drive = qBound(-rearGrip, throttle * (throttle >= 0 ? 840 * torque * shift : 320), rearGrip);
        if (state.reversing)
            drive = -qMin(rearGrip, brakeDemand * qBound(0.f, (state.speed + 120) * 4, 240.f));
        if (braking > 0 || (input.rearBrake && requestedThrottle <= 0))
            drive = 0;
        const float mudDrag = room.terrain ? room.terrain->sample(state.position).water * .035f : 0;
        const float rollingDrag = state.speed * (.13f + mudDrag) + state.speed * std::abs(state.speed) * .00009f;
        const float engineBrake = throttle >= 0 && requestedThrottle <= .01f
            ? qMin(75.f, std::abs(state.speed) * .12f) : 0;
        const float lateral = QVector3D::dotProduct(state.velocity, right);
        // Separate contact velocities make the front tyre pull the bike into
        // the corner and the rear follow. Turning is a torque response, not a
        // heading change followed by a sideways velocity correction.
        const float rollingYaw = DirtBikeSteeringRig::rollingYaw(state.steeringAngle);
        const DirtBikeTyre::SteeringFrame frontFrame(rollingYaw);
        const auto frontVelocity = frontFrame.toWheel(state.speed, lateral + state.yawRate * pitchLever);
        const float brakeYawLever = pitchLever * std::sin(rollingYaw);
        const float frontContactResponse = 1 + brakeYawLever * brakeYawLever / yawInertia;
        const float frontLongitudinal = DirtBikeTyre::brakingDemand(
            frontVelocity.x(), frontBrake, stepSeconds, frontContactResponse);
        // Include this step's drive in the rear brake impulse. Otherwise a
        // held rear brake releases at zero speed and reapplies one step later,
        // making an opposing throttle/brake request creep and oscillate.
        const float rearLongitudinal = drive + DirtBikeTyre::brakingDemand(
            state.speed + drive * stepSeconds, rearBrake, stepSeconds, 1);
        const float rearSlip = lateral - state.yawRate * pitchLever;
        const float rearLateralGrip = rearGrip * (input.rearBrake ? .24f : 1.f);
        const float frontDemand = DirtBikeTyre::lateralDemand(frontVelocity.y(), frontVelocity.x(), frontGrip,
            stepSeconds, state.frontSlipAngle);
        const float rearDemand = DirtBikeTyre::lateralDemand(rearSlip, state.speed, rearLateralGrip,
            stepSeconds, state.rearSlipAngle);
        const float frontScale = combinedGripScale(frontLongitudinal, frontDemand, frontGrip, frontGrip);
        const float rearScale = combinedGripScale(rearLongitudinal, rearDemand, rearGrip, rearLateralGrip);
        // Braking and cornering act along the steered tyre, not along the
        // chassis axes. Rotating both components also preserves contact power:
        // a lateral front force cannot add free forward energy while turning.
        const auto frontChassisForce = frontFrame.toChassis(frontLongitudinal * frontScale, frontDemand * frontScale);
        const float frontCornering = frontChassisForce.y();
        const float rearCornering = rearDemand * rearScale;
        acceleration = frontChassisForce.x() + rearLongitudinal * rearScale
            - rollingDrag - std::copysign(engineBrake, state.speed);
        if (braking > 0 || input.rearBrake) {
            // Brake force cannot reverse a wheel within a solver step.
            const float stoppingAcceleration = std::abs(state.speed) / stepSeconds;
            acceleration = qBound(-stoppingAcceleration, acceleration, stoppingAcceleration);
        }
        float lateralAcceleration = frontCornering + rearCornering
            - gravity * std::sin(groundRoll) * std::cos(groundRoll);
        // Static friction holds a stopped bike on an ordinary bank. Once
        // moving, gravity and the two tyre patches determine its lateral motion.
        if (std::abs(state.speed) < 20 && std::abs(lateral) < 20 && std::abs(throttle) < .01f
            && std::abs(groundRoll) < .6f)
            lateralAcceleration = -lateral / stepSeconds;
        state.velocity += right * lateralAcceleration * stepSeconds;
        // Mass-normalized yaw inertia for a compact bike/rider combination.
        const float yawAcceleration = (frontCornering - rearCornering) * pitchLever / yawInertia;
        state.yawRate = qBound(-1.7f, state.yawRate + yawAcceleration * stepSeconds, 1.7f);
        if (std::abs(state.speed) < 10)
            state.yawRate *= 1 - 12 * stepSeconds;
        state.heading = std::remainder(state.heading + state.yawRate * stepSeconds, 6.283185307f);
        const float moving = qBound(0.f, std::abs(state.speed) / 400, 1.f);
        const float turnLean = -std::atan(state.speed * state.yawRate / gravity) * .85f;
        const float targetLean = qBound(groundRoll - .95f,
            groundRoll * (1 - moving) + turnLean * moving, groundRoll + .95f);
        state.leanRate += ((targetLean - state.lean) * 64 - state.leanRate * 16) * stepSeconds;
        state.lean += state.leanRate * stepSeconds;
        const float slipAngle = std::atan2(std::abs(lateral), std::abs(state.speed) + 100);
        state.skid = qBound(0.f, slipAngle / .6f
            + (input.rearBrake ? std::abs(state.speed) / 1800 : 0), 1.f);
        // A request outside the theoretical grip budget is not itself a fall.
        // Require sustained, substantial actual sideslip before a low-side.
        const bool losingGrip = std::hypot(state.velocity.x(), state.velocity.z()) > 650
            && slipAngle > .65f;
        m_gripRiskTime = losingGrip ? m_gripRiskTime + stepSeconds : qMax(0.f, m_gripRiskTime - stepSeconds * 2);
        if (m_gripRiskTime > .65f) {
            crash(CrashReason::LostGrip);
            return;
        }
        acceleration -= gravity * std::sin(groundPitch);
    } else {
        state.skid = 0;
        state.frontSlipAngle = 0;
        state.rearSlipAngle = 0;
        state.yawRate *= 1 - stepSeconds;
        // Tyre torque ends at takeoff, but existing angular motion does not.
        // Carry the damped yaw momentum through flight until contact resumes.
        state.heading = std::remainder(state.heading + state.yawRate * stepSeconds, fullTurn);
        // Rider can adjust roll in the air, but steering cannot create tyre
        // yaw traction without contact. Rotation carries through the landing.
        state.leanRate += (-axis(input.steering) * 1.2f - state.leanRate * 2) * stepSeconds;
        state.lean = qBound(-1.3f, state.lean + state.leanRate * stepSeconds, 1.3f);
        m_gripRiskTime = qMax(0.f, m_gripRiskTime - stepSeconds);
    }
    state.acceleration = acceleration;
    const float wheelPitchAcceleration = advanceWheels(input, frontForce > 0, rearForce > 0);
    state.velocity += forward * acceleration * stepSeconds;
    state.velocity.setY(state.velocity.y() + (frontForce + rearForce - gravity) * stepSeconds);
    // Pulling rearward raises/shifts the assisted combined bike/rider COM.
    // Rear tyre acceleration can then unload the front suspension naturally;
    // no wheelie angle or front-wheel position is prescribed. Rear braking
    // reverses this torque, giving the rider a way to catch the balance point.
    const float pullBack = qMax(0.f, state.riderWeight);
    const float weightTransfer = state.grounded
        ? acceleration * (.0055f + pullBack * .01f)
            + gravity * (35 * pullBack) / (halfWheelbase * halfWheelbase)
            + (throttle > .1f && state.speed > 80 ? qMin(8.f, pullRate * 1.1f) : 0)
        : 0;
    const float pitchAcceleration = (frontForce - rearForce) * pitchLever / (halfWheelbase * halfWheelbase)
        + weightTransfer
        + wheelPitchAcceleration
        + axis(input.pitch) * (state.grounded ? 1.2f : 3.5f) - state.pitchRate * (state.grounded ? 3.f : .65f);
    state.pitchRate = qBound(-3.f, state.pitchRate + pitchAcceleration * stepSeconds, 3.f);
    state.pitch = std::remainder(state.pitch + state.pitchRate * stepSeconds, fullTurn);
    if (room.terrain && !state.crashed) {
        const float travel = std::abs(state.speed) * stepSeconds;
        room.terrain->wheelContact(frontPoint, frontRollingDirection, travel, state.frontSupport, std::abs(state.frontSlipAngle));
        room.terrain->wheelContact(rearPoint, rollingDirection, travel, state.rearSupport, std::abs(state.rearSlipAngle));
    }
    const auto previous = state.position;
    state.position += state.velocity * stepSeconds;
    SpatialVehicle probe;
    probe.id = m_id;
    probe.body = "dirt-bike";
    probe.position = state.position;
    probe.heading = state.heading;
    if (SpatialVehicleModel::blocked(room, probe)) {
        state.position = previous;
        if (QVector3D(state.velocity.x(), 0, state.velocity.z()).length() > 650) {
            crash(CrashReason::Collision);
            return;
        }
        state.velocity.setX(0);
        state.velocity.setZ(0);
    }
    // Suspension has a finite stroke. Resolve a bottomed-out landing as a
    // contact impulse rather than letting a fast descent tunnel into the dirt
    // while the spring takes several steps to arrest it.
    const auto nextForward = SpatialVehicleModel::rotate({0, 0, 1}, state.heading);
    const float nextPitchOffset = std::sin(state.pitch) * halfWheelbase;
    const float nextPitchLever = DirtBikeSuspension::pitchLever(state.pitch);
    const auto nextFrontDirection = SpatialVehicleModel::rotate({0, 0, 1}, state.heading + DirtBikeSteeringRig::rollingYaw(state.steeringAngle));
    const float supportHeight = qMax(
        tyreHeight(room, state.position + nextForward * nextPitchLever, nextFrontDirection) - nextPitchOffset,
        tyreHeight(room, state.position - nextForward * nextPitchLever, nextForward) + nextPitchOffset);
    if (state.position.y() < supportHeight - 24 && state.velocity.y() < 0) {
        state.position.setY(supportHeight - 24);
        state.velocity.setY(-state.velocity.y() * .04f);
        state.grounded = true;
    }
    const float floor = groundHeight(room, state.position);
    // Recoverable arcade crash: stop instead of tunnelling into terrain or
    // feeding unstable suspension impulses back into the camera.
    if (state.position.y() < floor - 55 || (state.grounded
        && landingCrashReason(state.pitch, state.lean, groundPitch, 0, groundRoll) != CrashReason::None)) {
        state.position.setY(qMax(state.position.y(), floor));
        crash(CrashReason::BadLanding);
        return;
    }
    state.speed = QVector3D::dotProduct(state.velocity, forward);
}

float DirtBikeDynamics::advanceWheels(const Input &input, bool frontContact, bool rearContact)
{
    auto &state = m_state;
    const float frontBefore = state.frontWheelSpeed;
    const float rearBefore = state.rearWheelSpeed;
    const float brake = input.brake || input.throttle < 0
        ? 1.f : qMax(0.f, axis(input.serviceBrake));
    const auto slow = [](float speed, float deceleration) {
        return std::copysign(qMax(0.f, std::abs(speed) - deceleration * stepSeconds), speed);
    };
    if (frontContact) {
        state.frontWheelSpeed = state.speed / wheelRadius;
    } else {
        state.frontWheelSpeed = slow(frontBefore, 240 * brake + .15f);
    }
    if (rearContact) {
        state.rearWheelSpeed = input.rearBrake ? 0 : state.speed / wheelRadius;
    } else {
        // The current gear bounds free-wheel RPM; holding the throttle cannot
        // add angular momentum forever. Automatic clutch remains an assist.
        const float targetSpeed = qMax(0.f, state.throttle) * 8200
            * gearSpeed[state.gear - 1] / (7200 * wheelRadius);
        if (state.throttle > .01f && brake == 0 && !input.rearBrake) {
            state.rearWheelSpeed += qBound(-60 * stepSeconds,
                targetSpeed - state.rearWheelSpeed, 60 * stepSeconds);
        } else {
            state.rearWheelSpeed = slow(rearBefore, 100 * brake
                + (input.rearBrake ? 180.f : 0.f) + .7f);
        }
    }
    state.frontWheelAngle = std::remainder(state.frontWheelAngle + state.frontWheelSpeed * stepSeconds, fullTurn);
    state.rearWheelAngle = std::remainder(state.rearWheelAngle + state.rearWheelSpeed * stepSeconds, fullTurn);
    if (frontContact || rearContact)
        return 0; // Supported chassis torque already comes from tyre forces.
    // Equal/opposite wheel momentum pitches the airborne chassis. SI inertias
    // are approximate bike/rider defaults (kg m²), not a measured model: a rear
    // brake tap lowers the nose; accelerating the rear wheel raises it.
    constexpr float frontInertia = .45f;
    constexpr float rearInertia = .65f;
    constexpr float chassisPitchInertia = 90;
    return ((state.frontWheelSpeed - frontBefore) * frontInertia
        + (state.rearWheelSpeed - rearBefore) * rearInertia) / (chassisPitchInertia * stepSeconds);
}

DirtBikeDynamics::CrashReason DirtBikeDynamics::landingCrashReason(float pitch, float lean, float groundPitch, float impactSpeed, float groundRoll)
{
    if (!std::isfinite(pitch) || !std::isfinite(lean) || !std::isfinite(groundPitch)
        || !std::isfinite(impactSpeed) || !std::isfinite(groundRoll))
        return CrashReason::BadLanding;
    if (impactSpeed > 1800)
        return CrashReason::HardLanding;
    const float pitchError = std::abs(angleError(pitch, groundPitch));
    const float rollError = std::abs(angleError(lean, groundRoll));
    if (pitchError > 1.25f || rollError > 1.25f
        || (impactSpeed > 450 && (pitchError > .78f || rollError > 1.f)))
        return CrashReason::BadLanding;
    return CrashReason::None;
}

QString DirtBikeDynamics::crashDescription(CrashReason reason)
{
    switch (reason) {
    case CrashReason::Collision: return QStringLiteral("Hit an obstacle");
    case CrashReason::HardLanding: return QStringLiteral("Landing impact too hard");
    case CrashReason::BadLanding: return QStringLiteral("Landed off balance");
    case CrashReason::LostGrip: return QStringLiteral("Tyres lost grip");
    case CrashReason::None: return {};
    }
    return {};
}

void DirtBikeDynamics::crash(CrashReason reason)
{
    m_state.crashed = true;
    m_state.reversing = false;
    m_reverseHold = 0;
    m_state.crashReason = reason;
    m_state.crashTime = 0;
    m_state.pitchRate = 0;
    m_state.yawRate = 0;
    m_state.acceleration = 0;
    m_state.landingImpact = 1;
    m_fallSide = m_state.lean < 0 ? -1 : 1;
    // The wreck can slide, but collision energy cannot propel it across a room.
    m_state.velocity *= .35f;
    m_state.velocity.setY(qMin(0.f, m_state.velocity.y()));
}

void DirtBikeDynamics::advanceCrash(const SpatialInterior &room)
{
    if (m_state.crashTime >= crashDuration && m_state.grounded)
        return;
    auto &state = m_state;
    state.crashTime = qMin(crashDuration, state.crashTime + stepSeconds);
    state.velocity.setY(state.velocity.y() - gravity * stepSeconds);
    const auto previous = state.position;
    state.position += state.velocity * stepSeconds;
    SpatialVehicle probe;
    probe.id = m_id;
    probe.body = "dirt-bike";
    probe.position = state.position;
    probe.heading = state.heading;
    if (SpatialVehicleModel::blocked(room, probe)) {
        state.position.setX(previous.x());
        state.position.setZ(previous.z());
        state.velocity.setX(0);
        state.velocity.setZ(0);
    }
    const float floor = groundHeight(room, state.position);
    state.grounded = state.position.y() <= floor;
    if (state.grounded) {
        state.position.setY(floor);
        state.velocity.setY(0);
    }
    state.velocity.setX(state.velocity.x() * (1 - 3.5f * stepSeconds));
    state.velocity.setZ(state.velocity.z() * (1 - 3.5f * stepSeconds));
    state.lean += (m_fallSide * 1.45f - state.lean) * 5 * stepSeconds;
    state.pitch += (.08f - state.pitch) * 5 * stepSeconds;
    state.speed = QVector3D(state.velocity.x(), 0, state.velocity.z()).length();
    state.skid = qMin(1.f, state.speed / 500);
    // Finishing the pose cannot suspend a wreck in mid-air. Stop its motion
    // only after contact; the fixed-step budget remains unchanged.
    if (state.crashTime >= crashDuration && state.grounded) {
        state.velocity = {};
        state.speed = 0;
    }
}
