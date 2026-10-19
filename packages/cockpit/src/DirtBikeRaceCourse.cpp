#include "DirtBikeRaceCourse.h"
#include "RedBudCourse.h"
#include <cmath>
#include <limits>
#include <QtGlobal>

DirtBikeRaceCourse::Sample DirtBikeRaceCourse::sample(float distance, float laneOffset, Layout layout)
{
    if (layout == Layout::RedBud)
        return RedBudCourse::sample(distance, laneOffset);
    if (!std::isfinite(distance) || !std::isfinite(laneOffset))
        return {};
    distance = std::fmod(distance, length);
    if (distance < 0)
        distance += length;
    Sample result;
    if (distance < straight) {
        result.position = {-radius, 0, -8600 + distance};
    } else if (distance < straight + turn) {
        const float angle = (distance - straight) / radius;
        result.position = {-radius * std::cos(angle), 0, 8600 + radius * std::sin(angle)};
        result.heading = angle;
        result.curvature = 1 / radius;
    } else if (distance < straight * 2 + turn) {
        result.position = {radius, 0, 8600 - (distance - straight - turn)};
        result.heading = 3.14159265358979323846f;
    } else {
        const float angle = (distance - straight * 2 - turn) / radius;
        result.position = {radius * std::cos(angle), 0, -8600 - radius * std::sin(angle)};
        result.heading = 3.14159265358979323846f + angle;
        result.curvature = 1 / radius;
    }
    const QVector3D right(std::cos(result.heading), 0, -std::sin(result.heading));
    result.position += right * qBound(-250.f, laneOffset, 250.f);
    return result;
}

float DirtBikeRaceCourse::progress(const QVector3D &point, Layout layout)
{
    if (layout == Layout::RedBud)
        return RedBudCourse::progress(point);
    if (!std::isfinite(point.x()) || !std::isfinite(point.z()))
        return 0;
    // 128 segments put the maximum arc chord error below 8 cm, much less than
    // the 600 cm half-lane. Fixed finite authoring data bounds all search work.
    constexpr int segments = 128;
    float nearest = std::numeric_limits<float>::max();
    float distance = 0;
    const QVector3D flat(point.x(), 0, point.z());
    for (int index = 0; index < segments; ++index) {
        const float begin = length * index / segments;
        const float end = length * (index + 1) / segments;
        const auto a = sample(begin).position;
        const auto span = sample(end).position - a;
        const float t = qBound(0.f, QVector3D::dotProduct(flat - a, span) / span.lengthSquared(), 1.f);
        const float error = (flat - a - span * t).lengthSquared();
        if (error < nearest) {
            nearest = error;
            distance = begin + (end - begin) * t;
        }
    }
    return distance;
}

float DirtBikeRaceCourse::courseLength(Layout layout)
{
    return layout == Layout::RedBud ? RedBudCourse::length() : length;
}
