#pragma once
#include "SpatialVehicle.h"
#include "SpatialRideSurface.h"

// Package-owned arcade bike. Scene units are cm, seconds and radians. A fixed
// 120 Hz two-contact suspension keeps the simulation independent of painting.
class DirtBikeDynamics {
public:
    enum class CrashReason { None, Collision, HardLanding, BadLanding, LostGrip };
    static constexpr float crashDuration = 2.4f;
    struct Input {
        float throttle = 0; // Negative input brakes, then reverses slowly.
        float steering = 0;
        float pitch = 0; // Positive pulls back: rider weight, wheelie lift and airborne nose-up.
        bool brake = false;
        float serviceBrake = 0;
        bool rearBrake = false;
    };
    struct State {
        QVector3D position;
        QVector3D velocity;
        float heading = 0;
        float yawRate = 0; // Angular momentum from the two tyre contact patches.
        float pitch = 0;
        float pitchRate = 0;
        float lean = 0;
        float leanRate = 0;
        float riderWeight = 0;
        float engineRpm = 1600;
        float throttle = 0;
        float shiftTime = 0;
        int gear = 1;
        float frontCompression = 0;
        float rearCompression = 0;
        float speed = 0;
        float skid = 0;
        float steeringAngle = 0;
        float frontLoad = .5f; // Fraction of supported weight on the front tyre.
        float frontSupport = .5f; // Actual suspension force / total static weight.
        float rearSupport = .5f;
        float frontSlipAngle = 0; // Relaxed contact-patch states, radians.
        float rearSlipAngle = 0;
        float frontWheelSpeed = 0; // rad/s, independent while airborne or locked.
        float rearWheelSpeed = 0;
        float frontWheelAngle = 0; // Wrapped radians, advanced by the fixed solver.
        float rearWheelAngle = 0;
        float acceleration = 0;
        float landingImpact = 0;
        float crashTime = 0;
        CrashReason crashReason = CrashReason::None;
        bool reversing = false; // Low-speed push assist after holding the service brake at rest.
        bool grounded = true;
        bool crashed = false;
    };
    void reset(const SpatialVehicle &vehicle);
    void advance(const Input &input, float seconds, const SpatialInterior &room);
    const State &state() const { return m_state; }
    bool unsettled() const;
    static float groundHeight(const SpatialInterior &room, const QVector3D &point);
    static SpatialSurfaceMaterial groundMaterial(const SpatialInterior &room, const QVector3D &point);
    static QString crashDescription(CrashReason reason);
    static CrashReason landingCrashReason(float pitch, float lean, float groundPitch, float impactSpeed, float groundRoll = 0);
private:
    void step(const Input &input, const SpatialInterior &room);
    void crash(CrashReason reason);
    void advanceCrash(const SpatialInterior &room);
    float advanceWheels(const Input &input, bool frontContact, bool rearContact);
    State m_state;
    QString m_id;
    double m_remainder = 0;
    float m_airTime = 0;
    float m_reverseHold = 0;
    float m_gripRiskTime = 0;
    float m_fallSide = 1;
};
