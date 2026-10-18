#pragma once

#include <QQuaternion>
#include <QVector3D>

// Package-owned attachment contract for the generated front assembly. All
// coordinates are centimetres in chassis rest space; angles are radians.
class DirtBikeSteeringRig {
public:
    static constexpr float maximumAngle = .6f;
    static QVector3D stem();
    static QVector3D axis();
    static QVector3D frontAxle();
    static QVector3D forkPoint(bool right, float height);
    static QVector3D grip(bool right);
    static QQuaternion steeringRotation(float angle);
    static QVector3D steerPoint(const QVector3D &point, float angle);
    static float rollingYaw(float angle);
    static bool steeredPart(int part);
    static bool visibleFromRider(int part);

    DirtBikeSteeringRig(float angle, float compression, float wheelSpin);
    QVector3D point(int part, const QVector3D &rest) const;
    QQuaternion rotation(int part) const;

private:
    QQuaternion m_steering;
    QQuaternion m_wheelSpin;
    float m_compression;
};
