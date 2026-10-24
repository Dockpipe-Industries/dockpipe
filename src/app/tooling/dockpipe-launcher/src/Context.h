#pragma once

#include <QString>
#include <QMap>
#include <QJsonObject>
#include <QUuid>

struct Context {
    QString id;
    QString label;
    QString workdir;
    /// Bundled workflow name (e.g. vscode); empty if using workflowFile only.
    QString workflow;
    /// Path to workflow config.yml; empty if using workflow name only.
    QString workflowFile;
    QString resolver;
    QString strategy;
    QString runtime;
    QString dockpipeBinary;
    QString envFile;
    /// Each entry is one dockpipe `--env` argument, typically `KEY=value` (e.g. OPENAI_API_KEY=…).
    QStringList extraDockpipeEnv;
    /// Per-launch sensitive inputs. Never serialized or included in command previews.
    QMap<QString, QString> transientLaunchEnv;
    /// Optional consumer capacity for one streamed output line; never serialized.
    int maximumOutputLineBytes = 0;

    static Context createNew();
    static Context fromJson(const QJsonObject &o);
    QJsonObject toJson() const;
};
