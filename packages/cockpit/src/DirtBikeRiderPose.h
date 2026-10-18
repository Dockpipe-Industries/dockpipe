#pragma once

#include <QQuaternion>
#include <QVector3D>
#include <array>

// Bounded two-bone limbs, evaluated once per bike update. Hands stay on the
// bars and boots on the pegs; the torso gives way before a limb overextends.
class DirtBikeRiderPose {
public:
    DirtBikeRiderPose(float steering, float lean, float weight, float compression);
    QVector3D point(int part, const QVector3D &rest) const;
    QQuaternion rotation(int part) const;
    QVector3D bodyPoint(const QVector3D &rest) const;
    QVector3D bodyNormal(const QVector3D &rest, const QVector3D &normal) const;

    static QVector3D shoulder(bool right);
    static QVector3D elbow(bool right);
    static QVector3D grip(bool right);
    static QVector3D hip(bool right);
    static QVector3D knee(bool right);
    static QVector3D ankle(bool right);

private:
    struct Bone {
        QVector3D origin;
        QVector3D destination;
        QQuaternion rotation;
    };
    void limb(int index, const QVector3D &restRoot, const QVector3D &restJoint,
        const QVector3D &restEnd, const QVector3D &posedRoot, const QVector3D &posedEnd,
        const QQuaternion &endRotation);
    std::array<Bone, 12> m_bones;
    QVector3D m_bodyOffset;
};
