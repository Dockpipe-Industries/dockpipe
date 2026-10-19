#include "DirtBikeRaceProgress.h"
#include <cmath>
#include <QtGlobal>

void DirtBikeRaceProgress::reset(const QVector3D &position, DirtBikeRaceCourse::Layout layout)
{
    *this = {};
    m_layout = layout;
    m_previous = position;
}

void DirtBikeRaceProgress::rejoin(const QVector3D &position)
{
    m_previous = position;
}

void DirtBikeRaceProgress::advance(const QVector3D &position, float elapsed)
{
    const float courseLength = DirtBikeRaceCourse::courseLength(m_layout);
    const float gateSpacing = courseLength / DirtBikeRaceCourse::gates;
    if (finished() || !std::isfinite(position.x()) || !std::isfinite(position.y())
        || !std::isfinite(position.z()) || !std::isfinite(elapsed) || elapsed < 0)
        return;
    const auto movement = position - m_previous;
    // A reset/teleport must explicitly rejoin, not sweep through checkpoints.
    if (movement.lengthSquared() > std::pow(gateSpacing * .5f, 2)) {
        m_previous = position;
        return;
    }
    const auto gate = DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start
        + m_nextGate * gateSpacing, 0, m_layout);
    const QVector3D forward(std::sin(gate.heading), 0, std::cos(gate.heading));
    const QVector3D right(std::cos(gate.heading), 0, -std::sin(gate.heading));
    const float before = QVector3D::dotProduct(m_previous - gate.position, forward);
    const float after = QVector3D::dotProduct(position - gate.position, forward);
    if (before < 0 && after >= 0) {
        const auto crossing = m_previous + movement * (-before / (after - before));
        if (std::abs(QVector3D::dotProduct(crossing - gate.position, right)) <= 600) {
            if (m_nextGate == 0) {
                ++m_laps;
                if (finished())
                    m_finishTime = elapsed;
            }
            m_nextGate = (m_nextGate + 1) % DirtBikeRaceCourse::gates;
        }
    }
    m_previous = position;
}

float DirtBikeRaceProgress::distance(const QVector3D &position) const
{
    const float courseLength = DirtBikeRaceCourse::courseLength(m_layout);
    const float gateSpacing = courseLength / DirtBikeRaceCourse::gates;
    if (finished())
        return raceLaps * courseLength;
    const int passed = m_nextGate == 0 ? DirtBikeRaceCourse::gates - 1 : m_nextGate - 1;
    float along = DirtBikeRaceCourse::progress(position, m_layout) - DirtBikeRaceCourse::start;
    if (along < 0)
        along += courseLength;
    if (passed == 0 && along > courseLength - gateSpacing)
        along = 0; // Grid riders behind the start have not passed the first gate.
    const float last = passed * gateSpacing;
    return m_laps * courseLength + last
        + qBound(0.f, along - last, gateSpacing);
}

QVector3D DirtBikeRaceProgress::recoveryPosition() const
{
    const float courseLength = DirtBikeRaceCourse::courseLength(m_layout);
    const float gateSpacing = courseLength / DirtBikeRaceCourse::gates;
    const int passed = m_nextGate == 0 ? DirtBikeRaceCourse::gates - 1 : m_nextGate - 1;
    return DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start
        + passed * gateSpacing + 80, 0, m_layout).position;
}

float DirtBikeRaceProgress::recoveryHeading() const
{
    const float courseLength = DirtBikeRaceCourse::courseLength(m_layout);
    const float gateSpacing = courseLength / DirtBikeRaceCourse::gates;
    const int passed = m_nextGate == 0 ? DirtBikeRaceCourse::gates - 1 : m_nextGate - 1;
    return DirtBikeRaceCourse::sample(DirtBikeRaceCourse::start
        + passed * gateSpacing + 80, 0, m_layout).heading;
}
