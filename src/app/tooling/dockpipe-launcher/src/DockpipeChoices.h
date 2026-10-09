#pragma once

#include <QString>
#include <QStringList>

/// Launcher-facing Dockpipe choice lists sourced from the Dockpipe CLI contract.
/// Falls back to static lists when no usable Dockpipe catalog is available.
class DockpipeChoices {
public:
    /// Walk upward from workdir to find a dockpipe project root.
    static QString findRepoRoot(const QString &hintWorkdir);

    /// Prefer DOCKPIPE_BIN, then the hinted project's compiled repo-local binary, then plain `dockpipe`.
    static QString preferredDockpipeBinary(const QString &hintWorkdir);

    void scan(const QString &repoRoot, const QString &hintWorkdir = QString());

    QStringList workflowNames;
    QStringList workflowConfigPaths;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};
