#pragma once

#include "WorkflowCatalogData.h"

class WorkflowCatalog {
public:
    /// Launcher-facing catalog provided by `dockpipe catalog list --format json`.
    static WorkflowCatalogData discoverCatalog(const QString &hintWorkdir = QString());

    /// All workflows with metadata (any category).
    static QVector<WorkflowMeta> discoverAll(const QString &repoRoot, const QString &hintWorkdir = QString());

    /// Only workflows with `category: app` (case-insensitive).
    static QVector<WorkflowMeta> discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir = QString());
};
