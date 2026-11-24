#pragma once

#include <QSet>
#include <QString>

/// Per-invocation execution metadata; never stored in a saved Context or its environment.
struct SessionLaunchOptions {
    /// Declared nonsecret workflow inputs use authoritative CLI overrides.
    QSet<QString> workflowInputOverrideNames;
    QString environmentDefinitionFile;
    QString environmentDefinitionName;
    /// Passive observations report outcomes inline rather than in the activity bell.
    bool notifyWorkflowOutcome = true;
};
