#pragma once

#include <QString>
#include <QStringList>
#include <QMap>
#include <QVector>

struct WorkflowInputMeta {
    QString fieldName;
    QString envName;
    QString type;
    QString elementType;
    QString description;
    QString defaultValue;
    QMap<QString, QString> attributes;
    QVector<WorkflowInputMeta> children;
};

struct WorkflowViewSectionMeta {
    QString id;
    QString title;
    QString description;
    QStringList fields;
};

struct WorkflowViewPageMeta {
    QString id;
    QString title;
    QString description;
    QVector<WorkflowViewSectionMeta> sections;
};

struct WorkflowViewEntryOptionMeta {
    QString value;
    QString label;
    QString next;
    QStringList pages;
};

struct WorkflowViewEntryMeta {
    QString type;
    QString field;
    QString title;
    QString description;
    QVector<WorkflowViewEntryOptionMeta> options;
};

struct WorkflowViewMeta {
    WorkflowViewEntryMeta entry;
    QVector<WorkflowViewPageMeta> pages;
};

/// One workflow entry returned by Dockpipe's launcher/tooling catalog contract.
struct WorkflowMeta {
    /// Folder / workflow name passed to `dockpipe --workflow`.
    QString workflowId;
    QString displayName;
    QString description;
    QString category;
    QString iconPath;
    QString configPath;
    QVector<WorkflowInputMeta> inputs;
    WorkflowViewMeta view;
    QMap<QString, QString> vars;
};

struct ResolverMeta {
    QString name;
    QString title;
    QString version;
    QString description;
    QString capability;
    QString remoteSetup;
};

struct WorkflowCatalogData {
    QVector<ResolverMeta> resolverDetails;
    QVector<WorkflowMeta> workflows;
    QStringList resolvers;
    QStringList strategies;
    QStringList runtimes;
};

class WorkflowCatalog {
public:
    /// Launcher-facing catalog provided by `dockpipe catalog list --format json`.
    static WorkflowCatalogData discoverCatalog(const QString &hintWorkdir = QString());

    /// All workflows with metadata (any category).
    static QVector<WorkflowMeta> discoverAll(const QString &repoRoot, const QString &hintWorkdir = QString());

    /// Only workflows with `category: app` (case-insensitive).
    static QVector<WorkflowMeta> discoverAppWorkflows(const QString &repoRoot, const QString &hintWorkdir = QString());
};
