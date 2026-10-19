#include "RedBudCourse.h"
#include <array>
#include <cmath>
#include <limits>
#include <QPointF>
#include <QVector>

namespace {
// Centreline traced from the official 2024 Version 2 map, clockwise through
// numbered features 1..36. 30 cm / map pixel sets an approximate riding scale.
const std::array<QPointF, 54> anchors{{
    {770,1230},{770,1100},{798,978},{820,906},{855,891},{894,918},
    {897,1050},{897,1260},{925,1308},{972,1305},{987,1256},{985,1080},
    {1008,1035},{1138,989},{1260,949},{1332,907},{1351,858},{1322,802},
    {1275,763},{1207,739},{1203,710},{1270,704},{1450,710},{1615,703},
    {1688,698},{1729,667},{1716,635},{1662,616},{1500,634},{1300,648},
    {1297,599},{1327,551},{1363,514},{1370,474},{1341,449},{1306,464},
    {1246,540},{1197,589},{1150,558},{1040,474},{937,403},{899,408},
    {824,502},{760,588},{684,622},{638,613},{607,574},{615,528},
    {680,464},{709,420},{700,379},{645,352},{423,284},{239,246}
}};
const std::array<QPointF, 17> western{{
    {120,240},{62,258},{55,316},{82,438},{122,592},{166,720},
    {195,737},{219,715},{218,678},{185,563},{159,432},{146,339},
    {165,308},{193,326},{221,447},{277,654},{322,768}
}};
struct Route {
    QVector<QVector3D> points;
    QVector<float> distances;
    float length = 0;
    Route()
    {
        QVector<QPointF> knots;
        for (auto point : anchors)
            knots.append(point);
        for (auto point : western)
            knots.append(point);
        knots.append({405,780});
        knots.append({453,815});
        knots.append({492,891});
        knots.append({524,1040});
        knots.append({554,1140});
        knots.append({597,1180});
        knots.append({674,1186});
        // Four samples per authored span preserve tight switchbacks while
        // bounding projection/search to 312 segments for this finite map.
        for (int index = 0; index < knots.size(); ++index) {
            const auto a = knots[(index + knots.size() - 1) % knots.size()];
            const auto b = knots[index];
            const auto c = knots[(index + 1) % knots.size()];
            const auto d = knots[(index + 2) % knots.size()];
            for (int step = 0; step < 4; ++step) {
                const float t = step / 4.f;
                const auto point = (b * 2 + (c - a) * t
                    + (a * 2 - b * 5 + c * 4 - d) * t * t
                    + (-a + b * 3 - c * 3 + d) * t * t * t) * .5;
                const float x = (point.x() - 890) * 30;
                const float z = (point.y() - 770) * 30;
                points.append({x, RedBudCourse::landscape(x, z), z});
            }
        }
        distances.append(0);
        for (int index = 0; index < points.size(); ++index) {
            const auto span = points[(index + 1) % points.size()] - points[index];
            length += std::hypot(span.x(), span.z());
            distances.append(length);
        }
    }
};
const Route &route()
{
    static const Route value;
    return value;
}
}

float RedBudCourse::landscape(float x, float z)
{
    const float ridge = std::exp(-std::pow((x + 12000) / 13000, 2) - std::pow((z + 7000) / 15000, 2));
    const float hollow = std::exp(-std::pow((x - 11000) / 3800, 2) - std::pow((z + 4000) / 4000, 2));
    return 850 + 450 * std::sin(x / 16000) * std::cos(z / 12000)
        + 1700 * ridge - 300 * hollow + 90 * std::sin((x + z) / 4500);
}

float RedBudCourse::length()
{
    return route().length;
}

DirtBikeRaceCourse::Sample RedBudCourse::sample(float distance, float lane)
{
    if (!std::isfinite(distance) || !std::isfinite(lane))
        return {};
    const auto &data = route();
    distance = std::fmod(distance, data.length);
    if (distance < 0)
        distance += data.length;
    int index = 0;
    while (index + 1 < data.points.size() && data.distances[index + 1] < distance)
        ++index;
    const int next = (index + 1) % data.points.size();
    const auto span = data.points[next] - data.points[index];
    const float segment = data.distances[index + 1] - data.distances[index];
    DirtBikeRaceCourse::Sample result;
    result.position = data.points[index] + span * ((distance - data.distances[index]) / segment);
    result.heading = std::atan2(span.x(), span.z());
    const auto before = data.points[index] - data.points[(index + data.points.size() - 1) % data.points.size()];
    const auto after = data.points[(next + 1) % data.points.size()] - data.points[next];
    const float change = std::remainder(std::atan2(after.x(), after.z()) - std::atan2(before.x(), before.z()), 6.283185307f);
    result.curvature = change / (segment * 2);
    result.position += QVector3D(std::cos(result.heading), 0, -std::sin(result.heading)) * qBound(-250.f, lane, 250.f);
    return result;
}

float RedBudCourse::progress(const QVector3D &point)
{
    const auto &data = route();
    float nearest = std::numeric_limits<float>::max();
    float distance = 0;
    for (int index = 0; index < data.points.size(); ++index) {
        auto offset = point - data.points[index];
        auto span = data.points[(index + 1) % data.points.size()] - data.points[index];
        offset.setY(0);
        span.setY(0);
        const float t = qBound(0.f, QVector3D::dotProduct(offset, span) / span.lengthSquared(), 1.f);
        const float error = (offset - span * t).lengthSquared();
        if (error < nearest) {
            nearest = error;
            distance = data.distances[index] + (data.distances[index + 1] - data.distances[index]) * t;
        }
    }
    return distance;
}
