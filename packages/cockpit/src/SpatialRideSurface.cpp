#include "SpatialRideSurface.h"
#include "SpatialVehicle.h"
#include <algorithm>
#include <cmath>

namespace {
constexpr int subdivisions = 4;
constexpr float pi = 3.14159265358979323846f;
float tangent(const QVector<QPointF> &profile, int index)
{
    if (index == 0 || index + 1 == profile.size())
        return 0;
    const auto before = profile[index] - profile[index - 1];
    const auto after = profile[index + 1] - profile[index];
    const float a = before.y() / before.x();
    const float b = after.y() / after.x();
    if (a * b <= 0)
        return 0;
    return 2 * a * b / (a + b);
}

QPointF sample(const SpatialRideSurface &surface, int index)
{
    const int span = qMin(index / subdivisions, int(surface.profile.size()) - 2);
    const float t = float(index - span * subdivisions) / subdivisions;
    const auto a = surface.profile[span];
    const auto b = surface.profile[span + 1];
    const float length = b.x() - a.x();
    const float height = (2*t*t*t - 3*t*t + 1)*a.y()
        + (t*t*t - 2*t*t + t)*length*tangent(surface.profile, span)
        + (-2*t*t*t + 3*t*t)*b.y()
        + (t*t*t - t*t)*length*tangent(surface.profile, span + 1);
    return {a.x() + t * length, height};
}

float shoulderHeight(const SpatialRideSurface &surface, float x, float t, float a, float b)
{
    const float shoulder = surface.width * .18f;
    const float inside = (surface.width / 2 - std::abs(x)) / shoulder;
    if (inside >= 1)
        return a + (b - a) * t;
    // Same diagonal as the two visible shoulder triangles, not a bilinear
    // approximation floating above/below the rendered contact surface.
    const float across = qBound(0.f, inside, 1.f);
    if (t >= across)
        return b * across;
    return a * across + (b - a) * t;
}

QVector3D curvedPoint(const SpatialRideSurface &surface, const QPointF &knot, float fraction)
{
    const float radius = std::abs(surface.curveRadius);
    const float direction = std::copysign(1.f, surface.curveRadius);
    const float angle = knot.x() / radius;
    const float offset = fraction * surface.width;
    const float height = std::abs(fraction) == .5f ? 0
        : knot.y() * (1 - fraction / .32f * surface.bank);
    return {direction * (radius - (radius - offset) * std::cos(angle)), height,
        (radius - offset) * std::sin(angle)};
}

float triangleHeight(const QVector3D &point, const QVector3D &a,
    const QVector3D &b, const QVector3D &c)
{
    const float denominator = (b.z() - c.z()) * (a.x() - c.x())
        + (c.x() - b.x()) * (a.z() - c.z());
    if (std::abs(denominator) < .001f)
        return -1;
    const float first = ((b.z() - c.z()) * (point.x() - c.x())
        + (c.x() - b.x()) * (point.z() - c.z())) / denominator;
    const float second = ((c.z() - a.z()) * (point.x() - c.x())
        + (a.x() - c.x()) * (point.z() - c.z())) / denominator;
    const float third = 1 - first - second;
    if (first < -.00001f || second < -.00001f || third < -.00001f)
        return -1;
    return qMax(0.f, first * a.y() + second * b.y() + third * c.y());
}

float curvedHeight(const SpatialRideSurface &surface, const QVector3D &local)
{
    const float radius = std::abs(surface.curveRadius);
    const float direction = std::copysign(1.f, surface.curveRadius);
    const float fromCenter = radius - direction * local.x();
    const float distance = std::hypot(fromCenter, local.z());
    const float z = std::abs(local.z()) < .01f ? 0 : local.z();
    const float along = std::atan2(z, fromCenter) * radius;
    // A four-strip semicircle is the coarsest admitted shape. Its chord can
    // sit inside the analytic radius by up to 1-cos(pi/8), hence this margin.
    const float chordMargin = (radius + surface.width / 2) * .08f;
    if (std::abs(distance - radius) > surface.width / 2 + chordMargin
        || along < -.01f || along > surface.profile.last().x() + .01f)
        return -1;
    const int count = (surface.profile.size() - 1) * subdivisions;
    // The angular range picks one strip. Contact then uses its actual visible
    // triangles, not an analytic arc that could float above the coarse mesh.
    constexpr float fractions[]{-.5f, -.32f, .32f, .5f};
    for (int index = 0; index < count; ++index) {
        const auto a = sample(surface, index);
        const auto b = sample(surface, index + 1);
        if (along < a.x() - .01f || along > b.x() + .01f)
            continue;
        for (int lane = 0; lane < 3; ++lane) {
            const auto first = curvedPoint(surface, a, fractions[lane]);
            const auto second = curvedPoint(surface, a, fractions[lane + 1]);
            const auto third = curvedPoint(surface, b, fractions[lane + 1]);
            const auto fourth = curvedPoint(surface, b, fractions[lane]);
            const float firstHeight = triangleHeight(local, first, second, third);
            if (firstHeight >= 0)
                return firstHeight;
            const float secondHeight = triangleHeight(local, first, third, fourth);
            if (secondHeight >= 0)
                return secondHeight;
        }
    }
    return -1;
}
}

bool SpatialRideSurface::valid() const
{
    if (!std::isfinite(center.x()) || !std::isfinite(center.z()) || center.y() != 0
        || !std::isfinite(heading) || std::abs(heading) > 6.284f
        || !std::isfinite(width) || width < 100 || width > 2000
        || profile.size() < 2 || profile.size() > 8 || !color.isValid())
        return false;
    if (!std::isfinite(curveRadius) || !std::isfinite(bank) || bank < 0 || bank > 1)
        return false;
    if (curveRadius == 0) {
        if (bank != 0)
            return false;
    } else if (!sculpted || std::abs(curveRadius) < qMax(1000.f, width)
        || std::abs(curveRadius) > 8000 || profile.first().x() != 0
        || profile.last().x() > std::abs(curveRadius) * pi) {
        return false;
    }
    float previous = -10001;
    for (const auto &knot : profile) {
        if (!std::isfinite(knot.x()) || !std::isfinite(knot.y()) || knot.x() <= previous
            || std::abs(knot.x()) > 10000 || knot.y() < 0 || knot.y() * (1 + bank) > 800)
            return false;
        previous = knot.x();
    }
    return profile.first().y() == 0 && profile.last().y() == 0;
}

float SpatialRideSurface::heightAt(const QVector3D &point) const
{
    if (!valid())
        return -1;
    const auto local = SpatialVehicleModel::rotate(point - center, -heading);
    if (curveRadius != 0)
        return curvedHeight(*this, local);
    if (std::abs(local.x()) > width / 2 + .01f || local.z() < profile.first().x() - .01f || local.z() > profile.last().x() + .01f)
        return -1;
    if (sculpted) {
        const int count = (profile.size() - 1) * subdivisions;
        for (int index = 0; index < count; ++index) {
            const auto a = sample(*this, index);
            const auto b = sample(*this, index + 1);
            if (local.z() <= b.x()) {
                const float t = qBound(0.f, float((local.z() - a.x()) / (b.x() - a.x())), 1.f);
                return shoulderHeight(*this, local.x(), t, a.y(), b.y());
            }
        }
        return 0;
    }
    for (int index = 1; index < profile.size(); ++index) {
        const auto &a = profile[index - 1];
        const auto &b = profile[index];
        if (local.z() <= b.x())
            return a.y() + (b.y() - a.y()) * (local.z() - a.x()) / (b.x() - a.x());
    }
    return 0;
}

SpatialMesh SpatialRideSurface::mesh() const
{
    SpatialMesh result;
    result.color = color;
    if (!valid())
        return result;
    if (sculpted) {
        result.material = SpatialSurfaceMaterial::Dirt;
        const int count = (profile.size() - 1) * subdivisions;
        for (int index = 0; index <= count; ++index) {
            const auto knot = sample(*this, index);
            for (float fraction : {-0.5f, -0.32f, 0.32f, 0.5f}) {
                const float height = std::abs(fraction) == .5f ? 0 : knot.y();
                const auto local = curveRadius == 0
                    ? QVector3D(fraction * width, height, float(knot.x()))
                    : curvedPoint(*this, knot, fraction);
                result.vertices.append(center + SpatialVehicleModel::rotate(
                    local, heading));
            }
        }
        for (int index = 0; index < count; ++index) {
            const int a = index * 4;
            const int b = a + 4;
            if (curveRadius != 0) {
                for (int lane = 0; lane < 3; ++lane) {
                    QVector<int> first{a + lane, a + lane + 1, b + lane + 1};
                    QVector<int> second{a + lane, b + lane + 1, b + lane};
                    if (curveRadius < 0) {
                        std::reverse(first.begin(), first.end());
                        std::reverse(second.begin(), second.end());
                    }
                    result.faces.append(first);
                    result.faces.append(second);
                }
                continue;
            }
            // Mirrored diagonals keep the two shoulders symmetric for contact.
            result.faces.append({a, b + 1, b});
            result.faces.append({a, a + 1, b + 1});
            result.faces.append({a + 1, a + 2, b + 2, b + 1});
            result.faces.append({a + 3, b + 2, a + 2});
            result.faces.append({a + 3, b + 3, b + 2});
        }
        for (const auto &face : result.faces) {
            const auto normal = QVector3D::crossProduct(result.vertices[face[1]] - result.vertices[face[0]],
                result.vertices[face[2]] - result.vertices[face[0]]).normalized();
            const float light = .72f + .28f * std::abs(QVector3D::dotProduct(normal, QVector3D(-.4f, .85f, -.3f).normalized()));
            result.faceColors.append(QColor::fromRgbF(color.redF()*light, color.greenF()*light, color.blueF()*light));
        }
        return result;
    }
    for (const auto &knot : profile) {
        for (float side : {-1.f, 1.f}) {
            result.vertices.append(center + SpatialVehicleModel::rotate({side * width / 2, float(knot.y()), float(knot.x())}, heading));
            result.vertices.append(center + SpatialVehicleModel::rotate({side * width / 2, 0, float(knot.x())}, heading));
        }
    }
    for (int index = 0; index + 1 < profile.size(); ++index) {
        const int a = index * 4;
        const int b = a + 4;
        result.faces.append({a, a + 2, b + 2, b});
        result.faces.append({a, b, b + 1, a + 1});
        result.faces.append({a + 2, a + 3, b + 3, b + 2});
    }
    return result;
}
