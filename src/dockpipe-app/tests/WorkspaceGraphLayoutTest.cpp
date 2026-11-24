#include "WorkspaceGraphLayout.h"

#include <QCoreApplication>
#include <QElapsedTimer>

#include <algorithm>
#include <cmath>
#include <cstdio>
#include <limits>

namespace {
bool check(bool condition, const char *message)
{
    if (!condition)
        std::fprintf(stderr, "%s\n", message);
    return condition;
}

QVector<WorkspaceGraphLayoutNode> tree(int count)
{
    QVector<WorkspaceGraphLayoutNode> nodes;
    nodes.append({QStringLiteral("root"), -1, {}, false});
    for (int index = 1; index < count; ++index) {
        const int parent = index < 5 ? 0 : 1 + (index - 5) % 4;
        nodes.append({QStringLiteral("node-%1").arg(index), parent, {}, false});
    }
    return nodes;
}

bool finitePositions(const QVector<QVector3D> &positions)
{
    for (const auto &position : positions) {
        if (!std::isfinite(position.x()) || !std::isfinite(position.y())
            || !std::isfinite(position.z()))
            return false;
    }
    return true;
}

float minimumSeparation(const QVector<QVector3D> &positions)
{
    float minimum = std::numeric_limits<float>::max();
    for (qsizetype first = 0; first < positions.size(); ++first) {
        for (qsizetype second = first + 1; second < positions.size(); ++second)
            minimum = std::min(minimum, (positions.at(first) - positions.at(second)).length());
    }
    return minimum;
}

bool branchesRemainClustered(const QVector<WorkspaceGraphLayoutNode> &nodes,
                            const QVector<QVector3D> &positions)
{
    double withinTotal = 0.0;
    double betweenTotal = 0.0;
    int withinCount = 0;
    int betweenCount = 0;
    for (qsizetype first = 5; first < nodes.size(); ++first) {
        for (qsizetype second = first + 1; second < nodes.size(); ++second) {
            const double distance = (positions.at(first) - positions.at(second)).length();
            if (nodes.at(first).parent == nodes.at(second).parent) {
                withinTotal += distance;
                ++withinCount;
            } else {
                betweenTotal += distance;
                ++betweenCount;
            }
        }
    }
    return withinTotal / withinCount < betweenTotal / betweenCount * 0.8;
}
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const auto nodes = tree(150);
    const auto positions = WorkspaceGraphLayout::arrange(nodes);
    if (!check(positions.size() == nodes.size() && finitePositions(positions),
               "layout did not return one finite position per visible node")
        || !check(positions == WorkspaceGraphLayout::arrange(nodes), "layout is not deterministic")
        || !check(positions.front().isNull(), "root moved from the origin")
        || !check(minimumSeparation(positions) > 10.0f, "constellation nodes overlap")
        || !check(branchesRemainClustered(nodes, positions), "branches lost their local clusters"))
        return 1;

    // A nonzero scalar triple product establishes genuinely spatial geometry,
    // independent of the camera's orientation or a particular coordinate axis.
    const float volume = std::abs(QVector3D::dotProduct(positions.at(1),
                                 QVector3D::crossProduct(positions.at(2), positions.at(3))));
    if (!check(volume > 10000.0f, "layout collapsed into a plane"))
        return 1;

    auto expanded = nodes;
    for (qsizetype index = 0; index < expanded.size(); ++index) {
        expanded[index].position = positions.at(index);
        expanded[index].positioned = true;
    }
    for (int index = 150; index < 170; ++index)
        expanded.append({QStringLiteral("expanded-%1").arg(index), 1, {}, false});
    const auto expandedPositions = WorkspaceGraphLayout::arrange(expanded);
    double totalMovement = 0.0;
    for (qsizetype index = 0; index < nodes.size(); ++index)
        totalMovement += (expandedPositions.at(index) - positions.at(index)).length();
    if (!check(finitePositions(expandedPositions) && expandedPositions.front().isNull(),
               "expansion lost finite positions or the root anchor")
        || !check(totalMovement / nodes.size() < 30.0, "expansion discarded existing spatial positions"))
        return 1;

    auto coincident = tree(70);
    for (auto &node : coincident) {
        node.id = QStringLiteral("identical-id");
        node.positioned = true;
    }
    const auto separated = WorkspaceGraphLayout::arrange(coincident);
    if (!check(finitePositions(separated) && minimumSeparation(separated) > 1.0f,
               "identical ids and initial positions were not separated"))
        return 1;

    QElapsedTimer timer;
    timer.start();
    const auto maximumPositions = WorkspaceGraphLayout::arrange(tree(500));
    const qint64 elapsedMilliseconds = timer.elapsed();
    std::fprintf(stdout, "500-node layout: %lld ms; minimum separation: %.1f units\n",
                 static_cast<long long>(elapsedMilliseconds), minimumSeparation(maximumPositions));
    if (!check(maximumPositions.size() == 500 && finitePositions(maximumPositions),
               "500-node budget boundary did not produce complete finite output")
        || !check(WorkspaceGraphLayout::arrange(tree(501)).isEmpty(), "over-budget graph was accepted")
        || !check(WorkspaceGraphLayout::arrange({}).isEmpty(), "empty graph changed")
        || !check(WorkspaceGraphLayout::arrange(tree(1)) == QVector<QVector3D>{QVector3D{}},
                  "single root was not pinned"))
        return 1;

    auto invalid = tree(2);
    invalid[1].parent = 1;
    if (!check(WorkspaceGraphLayout::arrange(invalid).isEmpty(), "self parent was accepted"))
        return 1;
    invalid[1].parent = 2;
    if (!check(WorkspaceGraphLayout::arrange(invalid).isEmpty(), "future parent was accepted"))
        return 1;
    invalid[1].parent = -1;
    if (!check(WorkspaceGraphLayout::arrange(invalid).isEmpty(), "second root was accepted"))
        return 1;
    invalid = tree(2);
    invalid[0].parent = 0;
    if (!check(WorkspaceGraphLayout::arrange(invalid).isEmpty(), "invalid root parent was accepted"))
        return 1;

    for (const float invalidCoordinate : {std::numeric_limits<float>::quiet_NaN(),
                                         std::numeric_limits<float>::infinity()}) {
        invalid = tree(2);
        invalid[1].position.setZ(invalidCoordinate);
        if (!check(WorkspaceGraphLayout::arrange(invalid).isEmpty(), "non-finite coordinate was accepted"))
            return 1;
    }

    // Finite coordinates must not overflow during squared-distance arithmetic.
    auto extreme = tree(3);
    extreme[1].position = QVector3D(std::numeric_limits<float>::max(), 0.0f, 0.0f);
    extreme[1].positioned = true;
    extreme[2].position = -extreme.at(1).position;
    extreme[2].positioned = true;
    if (!check(finitePositions(WorkspaceGraphLayout::arrange(extreme)),
               "finite extreme coordinates overflowed during layout"))
        return 1;

    return 0;
}
