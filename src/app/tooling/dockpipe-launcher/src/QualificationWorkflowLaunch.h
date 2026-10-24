#pragma once

#include "WorkflowCatalog.h"

#include <QJsonObject>
#include <QSet>

class WorkflowLaunchDialog;

// Used only by the opt-in qualifier and its native widget tests.
class QualificationWorkflowLaunch final {
public:
    enum class Outcome { Pending, Accepted, Failure };

    struct Resource {
        QString id;
        QString launchId;
        bool hasDisplay = false;
    };

    static bool validate(const WorkflowMeta &workflow, const QJsonObject &inputs, QString &error);
    static Outcome submit(WorkflowLaunchDialog &dialog, const WorkflowMeta &workflow,
                       const QJsonObject &inputs, QString &error);
    static QString resolve(const QString &launchId, const QSet<QString> &previousResources,
                           const QVector<Resource> &resources, QString &error);
};
