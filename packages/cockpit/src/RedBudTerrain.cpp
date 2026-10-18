#include "RedBudTerrain.h"
#include "RedBudCourse.h"
#include <cmath>

namespace {
float smooth(float value)
{
    value = qBound(0.f, value, 1.f);
    return value * value * (3 - 2 * value);
}
float mound(float distance, float center, float width, float height)
{
    const float t = std::abs(distance - center) / width;
    return t >= 1 ? 0 : height * .5f * (1 + std::cos(t * 3.14159265358979323846f));
}
}

std::shared_ptr<SpatialTerrain> RedBudTerrain::create()
{
    const auto mapPoint = [](float x, float y) { return QVector3D((x - 890) * 30, 0, (y - 770) * 30); };
    const float leap = RedBudCourse::progress(mapPoint(235, 500));
    const float tabletop = RedBudCourse::progress(mapPoint(1490, 710));
    const float rollers = RedBudCourse::progress(mapPoint(1150, 990));
    SpatialTerrain::Configuration configuration;
    // Accelerated wetting/accumulation for a short playable weather trial.
    configuration.soilColor = QColor("#887261");
    configuration.firmColor = QColor("#67774b");
    configuration.rainRate = .02f;
    configuration.wettingRate = .035f;
    return SpatialTerrain::create(configuration, [=](float x, float z) {
        const QVector3D point(x, 0, z);
        const float along = RedBudCourse::progress(point);
        const auto line = RedBudCourse::sample(along);
        const float distance = std::hypot(x - line.position.x(), z - line.position.z());
        const float soil = 1 - smooth((distance - 450) / 220);
        float jumps = mound(along, leap - 1500, 700, 340)
            + mound(along, leap, 650, 220) + mound(along, leap + 1500, 1000, 390)
            + mound(along, tabletop, 1100, 240);
        for (int roller = 0; roller < 4; ++roller)
            jumps += mound(along, rollers + roller * 750, 320, 65);
        const float bank = qMin(90.f, std::abs(line.curvature) * 100000) * smooth(distance / 550);
        return SpatialTerrain::Seed{RedBudCourse::landscape(x, z) + soil * (jumps + bank), soil};
    });
}
