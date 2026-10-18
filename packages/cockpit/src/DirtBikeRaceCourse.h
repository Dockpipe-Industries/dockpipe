#pragma once
#include <QVector3D>

// The centreline of the authored Mesa oval, in centimetres. Direction agrees
// with the left straight and both curved ride surfaces. No per-frame allocation.
namespace DirtBikeRaceCourse {
enum class Layout { Mesa, RedBud };
float courseLength(Layout layout);
constexpr float straight = 17200;
constexpr float radius = 2700;
constexpr float turn = radius * 3.14159265358979323846f;
constexpr float length = straight * 2 + turn * 2;
constexpr float start = 400;
constexpr int gates = 8;
constexpr float gateSpacing = length / gates;
struct Sample {
    QVector3D position;
    float heading = 0;
    float curvature = 0;
};
Sample sample(float distance, float laneOffset = 0, Layout layout = Layout::Mesa);
float progress(const QVector3D &point, Layout layout = Layout::Mesa);
}
