#pragma once
#include "DirtBikeRaceCourse.h"

// Ordered forward-crossing gates prevent start-line oscillation, reverse riding
// or a shortcut across the infield from manufacturing a completed lap.
class DirtBikeRaceProgress {
public:
    static constexpr int raceLaps = 3;
    void reset(const QVector3D &position, DirtBikeRaceCourse::Layout layout = DirtBikeRaceCourse::Layout::Mesa);
    void rejoin(const QVector3D &position);
    void advance(const QVector3D &position, float elapsed);
    float distance(const QVector3D &position) const;
    QVector3D recoveryPosition() const;
    float recoveryHeading() const;
    int laps() const { return m_laps; }
    int nextGate() const { return m_nextGate; }
    bool finished() const { return m_laps == raceLaps; }
    float finishTime() const { return m_finishTime; }
private:
    DirtBikeRaceCourse::Layout m_layout = DirtBikeRaceCourse::Layout::Mesa;
    QVector3D m_previous;
    int m_laps = 0;
    int m_nextGate = 1;
    float m_finishTime = 0;
};
