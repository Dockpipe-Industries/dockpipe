#pragma once

#include <QJsonObject>
#include <QString>

struct EnvironmentDefinition {
    QString path;
    QString scope;
    QString name;
    QString kind;
    QString workflow;
    QString package;
    QString storageProfile;
    QString instanceId;
    QString runtimeState;
    QString runtimeRunId;
    QString runtimeWorkdir;
    QString runtimeContainerId;
    QString error;
    QJsonObject container;
    QJsonObject vm;
};
