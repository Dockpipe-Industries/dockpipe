#include "WorkspaceGraphLayout.h"

#include <QDebug>

#include <algorithm>
#include <cmath>
#include <cstdint>

namespace {
constexpr int maximumNodes = 500;
constexpr int relaxationSteps = 56;
constexpr double edgeLength = 135.0;

// Double precision keeps force calculations finite even when the caller supplies
// a finite QVector3D near its float coordinate limit.
struct Point
{
    double x = 0.0;
    double y = 0.0;
    double z = 0.0;

    Point operator+(const Point &other) const
    {
        return {x + other.x, y + other.y, z + other.z};
    }

    Point operator-(const Point &other) const
    {
        return {x - other.x, y - other.y, z - other.z};
    }

    Point operator*(double scale) const
    {
        return {x * scale, y * scale, z * scale};
    }

    double length() const
    {
        return std::sqrt(x * x + y * y + z * z);
    }
};

Point unitVector(const Point &point)
{
    const double length = point.length();
    if (length < 0.000001)
        return {};
    return point * (1.0 / length);
}

std::uint64_t stableHash(const QString &id)
{
    std::uint64_t hash = UINT64_C(14695981039346656037);
    for (const QChar character : id) {
        hash ^= character.unicode();
        hash *= UINT64_C(1099511628211);
    }
    return hash;
}

std::uint64_t mix(std::uint64_t value)
{
    value ^= value >> 30;
    value *= UINT64_C(0xbf58476d1ce4e5b9);
    value ^= value >> 27;
    value *= UINT64_C(0x94d049bb133111eb);
    return value ^ (value >> 31);
}

Point seedDirection(std::uint64_t seed)
{
    const std::uint64_t first = mix(seed);
    const std::uint64_t second = mix(seed + UINT64_C(0x9e3779b97f4a7c15));
    constexpr double inverseRange = 1.0 / 4294967296.0;
    constexpr double fullCircle = 6.28318530717958647692;
    const double height = 2.0 * (first >> 32) * inverseRange - 1.0;
    const double angle = fullCircle * (second >> 32) * inverseRange;
    const double radius = std::sqrt(std::max(0.0, 1.0 - height * height));
    return {radius * std::cos(angle), height, radius * std::sin(angle)};
}

bool finite(const QVector3D &position)
{
    return std::isfinite(position.x()) && std::isfinite(position.y())
        && std::isfinite(position.z());
}
}

QVector<QVector3D> WorkspaceGraphLayout::arrange(const QVector<WorkspaceGraphLayoutNode> &nodes)
{
    if (nodes.size() > maximumNodes) {
        qWarning("Workspace graph layout exceeds the 500 visible-node limit");
        return {};
    }

    for (qsizetype index = 0; index < nodes.size(); ++index) {
        const auto &node = nodes.at(index);
        const bool validParent = index == 0 ? node.parent == -1
                                            : node.parent >= 0 && node.parent < index;
        if (!validParent || !finite(node.position)) {
            qWarning("Workspace graph layout requires finite coordinates and a parent-before-child tree");
            return {};
        }
    }

    QVector<Point> positions(nodes.size());
    QVector<Point> anchors(nodes.size());
    QVector<std::uint64_t> hashes;
    hashes.reserve(nodes.size());
    for (const auto &node : nodes)
        hashes.append(stableHash(node.id));

    for (qsizetype index = 1; index < nodes.size(); ++index) {
        const auto &node = nodes.at(index);
        if (node.positioned) {
            positions[index] = {node.position.x(), node.position.y(), node.position.z()};
            anchors[index] = positions.at(index);
        } else {
            const Point parentPosition = positions.at(node.parent);
            const Point direction = unitVector(seedDirection(hashes.at(index))
                                               + unitVector(parentPosition) * 0.65);
            positions[index] = parentPosition + direction * edgeLength;
        }
    }

    QVector<Point> forces(nodes.size());
    for (int step = 0; step < relaxationSteps; ++step) {
        std::fill(forces.begin(), forces.end(), Point{});

        for (qsizetype first = 0; first < nodes.size(); ++first) {
            for (qsizetype second = first + 1; second < nodes.size(); ++second) {
                Point difference = positions.at(first) - positions.at(second);
                double distance = difference.length();
                if (distance < 0.001) {
                    // Separate identical seeds/positions without Qt's randomized
                    // hashing, a random generator, or persistent simulation state.
                    difference = seedDirection(hashes.at(first) ^ mix(hashes.at(second)
                                               + static_cast<std::uint64_t>(second)));
                    distance = 1.0;
                }

                const double magnitude = std::min(35.0, 28000.0 / (distance * distance + 64.0));
                const Point repulsion = difference * (magnitude / distance);
                forces[first] = forces.at(first) + repulsion;
                forces[second] = forces.at(second) - repulsion;
            }
        }

        for (qsizetype index = 1; index < nodes.size(); ++index) {
            const auto &node = nodes.at(index);
            const Point toParent = positions.at(node.parent) - positions.at(index);
            const double distance = toParent.length();
            if (distance > 0.001) {
                const Point spring = toParent * (0.10 * (distance - edgeLength) / distance);
                forces[index] = forces.at(index) + spring;
                forces[node.parent] = forces.at(node.parent) - spring;
            }
        }

        const double cooling = 1.0 - 0.8 * step / (relaxationSteps - 1);
        for (qsizetype index = 1; index < nodes.size(); ++index) {
            Point force = forces.at(index) - positions.at(index) * 0.004;
            if (nodes.at(index).positioned)
                force = force + (anchors.at(index) - positions.at(index)) * 0.18;

            Point displacement = force * 0.45;
            const double maximumStep = (nodes.at(index).positioned ? 4.0 : 16.0) * cooling;
            const double length = displacement.length();
            if (length > maximumStep)
                displacement = displacement * (maximumStep / length);
            positions[index] = positions.at(index) + displacement;
        }
    }

    QVector<QVector3D> result;
    result.reserve(nodes.size());
    for (const auto &position : positions)
        result.append(QVector3D(static_cast<float>(position.x), static_cast<float>(position.y),
                                static_cast<float>(position.z)));
    return result;
}
