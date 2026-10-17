#pragma once

#include <QString>

struct WorkflowScriptPath {
    QString relativePath;
    QString error;

    static WorkflowScriptPath resolve(const QString &workspace, const QString &script);
};
