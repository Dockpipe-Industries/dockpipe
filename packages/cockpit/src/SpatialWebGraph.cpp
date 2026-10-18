#include "SpatialWebGraph.h"
#include <QHash>
#include <QSet>
#include <cmath>

SpatialWebGraph SpatialWebGraph::fromDocument(const SpatialWebDocument &document)
{
    SpatialWebGraph graph;
    if (document.items.size() > SpatialWebDocument::maximumItems)
        return graph;
    graph.nodes.append({SpatialWebNode::Role::Hub, SpatialWebNode::Action::None,
        "root", document.title, document.url});
    QHash<QString, int> collections;
    for (int index = 0; index < document.items.size(); ++index) {
        const auto &item = document.items[index];
        const QString category = item.category.isEmpty() ? document.title : item.category;
        if (!collections.contains(category)) {
            collections.insert(category, graph.nodes.size());
            graph.nodes.append({SpatialWebNode::Role::Collection, SpatialWebNode::Action::None,
                "collection-" + QString::number(index), category, {}, 0});
        }
        auto action = SpatialWebNode::Action::None;
        if (item.kind == "video")
            action = SpatialWebNode::Action::PlayMedia;
        else if (!item.url.isEmpty() && item.kind != "excerpt"
            && (document.stableLayout || !SpatialWebDocument::samePage(item.url, document.url)))
            action = SpatialWebNode::Action::Navigate;
        graph.nodes.append({SpatialWebNode::Role::Destination, action, item.id, item.title,
            item.url, collections.value(category), index, item.hasImage ? 1.2f : 1.0f});
    }
    return graph;
}

bool SpatialWebGraph::validate(QString &error) const
{
    error.clear();
    if (nodes.isEmpty() || nodes.size() > maximumNodes || nodes[0].role != SpatialWebNode::Role::Hub
        || nodes[0].parent != -1) {
        error = "Invalid content graph root or node budget.";
        return false;
    }
    QSet<QString> identities;
    QSet<int> items;
    for (int index = 0; index < nodes.size(); ++index) {
        const auto &node = nodes[index];
        // Parent-before-child ordering prevents cycles and bounds expansion depth.
        if (node.id.isEmpty() || identities.contains(node.id) || node.id.size() > 64
            || node.title.size() > 180 || (!node.url.isEmpty() && !SpatialWebDocument::allowedUrl(node.url))
            || !std::isfinite(node.importance) || node.importance < 0.5f || node.importance > 2
            || (index > 0 && (node.parent < 0 || node.parent >= index))) {
            error = "Invalid content identity, relationship or destination.";
            return false;
        }
        if ((node.role != SpatialWebNode::Role::Hub && node.role != SpatialWebNode::Role::Collection
                && node.role != SpatialWebNode::Role::Destination)
            || (node.action != SpatialWebNode::Action::None && node.action != SpatialWebNode::Action::Navigate
                && node.action != SpatialWebNode::Action::PlayMedia)
            || (node.action == SpatialWebNode::Action::Navigate && node.url.isEmpty())) {
            error = "Invalid content role or action.";
            return false;
        }
        if (index > 0) {
            const auto parentRole = nodes[node.parent].role;
            if ((node.role == SpatialWebNode::Role::Collection && parentRole != SpatialWebNode::Role::Hub)
                || (node.role == SpatialWebNode::Role::Destination && parentRole != SpatialWebNode::Role::Collection)
                || node.role == SpatialWebNode::Role::Hub) {
                error = "Unsupported content containment relationship.";
                return false;
            }
        }
        if (node.role == SpatialWebNode::Role::Destination) {
            if (node.itemIndex < 0 || node.itemIndex >= SpatialWebDocument::maximumItems || items.contains(node.itemIndex)) {
                error = "Invalid content binding.";
                return false;
            }
            items.insert(node.itemIndex);
        }
        identities.insert(node.id);
    }
    return true;
}
