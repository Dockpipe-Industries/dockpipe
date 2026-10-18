#pragma once
#include "DirtBikeDynamics.h"

// Small presentation effect; it consumes contact state without changing it.
class DirtBikeDust {
public:
    struct Particle {
        QVector3D position;
        float life;
    };
    static constexpr int maximumParticles = 24;
    static constexpr float lifetime = 1.1f;
    DirtBikeDust();
    void advance(const DirtBikeDynamics::State &bike, float seconds, const SpatialInterior &room);
    void clear();
    bool isEmpty() const { return m_particles.isEmpty(); }
    const QVector<Particle> &particles() const { return m_particles; }
private:
    QVector<Particle> m_particles;
    float m_clock = 0;
};
