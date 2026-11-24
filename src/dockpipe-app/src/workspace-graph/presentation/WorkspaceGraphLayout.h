#pragma once

#include <QString>
#include <QVector>
#include <QVector3D>

struct WorkspaceGraphLayoutNode
{
    QString id;
    int parent = -1;
    QVector3D position;
    bool positioned = false;
};

class WorkspaceGraphLayout
{
public:
    // The visible graph is one parent-before-child tree with its root at index 0.
    // At most 500 nodes are accepted, matching the graph page's visible-node budget.
    // Coordinates must be finite, including unused initial coordinates. The root
    // is pinned at the origin; positioned nodes retain a soft position anchor.
    // Empty input returns empty. Invalid input returns empty and emits a warning.
    // Work is bounded to 56 relaxation steps, O(node count squared), with O(n) storage.
    static QVector<QVector3D> arrange(const QVector<WorkspaceGraphLayoutNode> &nodes);
};
