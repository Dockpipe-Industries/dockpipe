#pragma once

#include <QString>
#include <QJsonObject>
#include <QStringList>
#include <QMap>
#include <QVector>

struct PackageNativeDisplayMeta {
    int interfaceVersion = 0;
    QString packageRoot;
    QMap<QString, QString> libraries;
};

struct PackageDisplayMeta {
    QString id;
    QString title;
    qint64 processId = 0;
    QString processStartToken;
    PackageNativeDisplayMeta nativePackage;
    QJsonObject options;
    QString leaseId;
};

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
    QString source;
    QString sourceKind;
    QString sourcePath;
    QString sourceName;
    bool cloneable = false;
    bool supported = true;
    QString unsupportedReason;
    QStringList warnings;

    QString sourceLabel() const;
    QString catalogTitle() const;
    QString catalogDescription() const;
    QVector<WorkflowInputMeta> inputs;
    WorkflowViewMeta view;
    QMap<QString, QString> vars;
};

struct PackageSurfaceEnvironmentBinding {
    QString asset;
    QString inherit;
    bool operator==(const PackageSurfaceEnvironmentBinding &other) const
    {
        return asset == other.asset && inherit == other.inherit;
    }
};

struct PackageSurfaceActionMeta {
    QString id;
    QString title;
    QString scope;
    QString workflow;
    QString workflowId;
    QMap<QString, QMap<QString, PackageSurfaceEnvironmentBinding>> launchEnvironment;
};

struct PackageSurfaceMeta {
    QString packageRoot;
    QString id;
    QString title;
    QString snapshot;
    QVector<PackageSurfaceActionMeta> actions;
    PackageNativeDisplayMeta nativeDisplay;
};

struct ResolverMeta {
    QString name;
    QString title;
    QString version;
    QString description;
    QString capability;
    QString remoteSetup;
    PackageSurfaceMeta surface;
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
