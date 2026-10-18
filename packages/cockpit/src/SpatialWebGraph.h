#pragma once

#include "SpatialWebDocument.h"

// Meaning only: containment is independent of geometry and traversal layout.
struct SpatialWebNode {
    enum class Role { Hub, Collection, Destination };
    enum class Action { None, Navigate, PlayMedia };
    Role role = Role::Hub;
    Action action = Action::None;
    QString id;
    QString title;
    QUrl url;
    int parent = -1;
    int itemIndex = -1;
    float importance = 1;
};

struct SpatialWebGraph {
    // Root + at most one collection and one destination per extracted item.
    static constexpr int maximumNodes = 1 + 2 * SpatialWebDocument::maximumItems;
    QVector<SpatialWebNode> nodes;
    static SpatialWebGraph fromDocument(const SpatialWebDocument &document);
    bool validate(QString &error) const;
};
