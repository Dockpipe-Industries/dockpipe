#pragma once

#include <QJsonObject>
#include <QString>
#include <QVector>

struct DashboardPanelDefinition {
    QString id;
    QString kind;
    QString environment = QStringLiteral("canvas");
};

struct DashboardOptions {
    bool compactTiling = true;
    bool autoHideDock = true;
    bool showPanelHeaders = true;
    bool autoAcquireApps = false;
    QString canvasBackground = QStringLiteral("grid");
    QString canvasImagePath;
    QString canvasFileScope = QStringLiteral("project");
    QString tilingFileScope = QStringLiteral("system");
};

struct DashboardDefinition {
    QString id;
    QString name;
    QVector<DashboardPanelDefinition> panels;
    QJsonObject layout;
    DashboardOptions options;
};

// User-authored layout metadata only. Native handles, browser data, execution
// credentials and running processes remain owned by their existing services.
class DashboardStore final {
public:
    // Twelve custom dashboards plus the permanent built-in dashboard.
    static constexpr int maximumDashboards = 13;
    static QString defaultDashboardId();
    static bool ensureDefaultDashboard(QVector<DashboardDefinition> &dashboards);
    static constexpr int maximumPanelsPerDashboard = 16;
    static constexpr int maximumTotalPanels = 64;
    bool load(QVector<DashboardDefinition> &dashboards, QString &error);
    bool save(const QVector<DashboardDefinition> &dashboards, QString &error);
    static QString filePath();
    static QString newId();
    static bool validate(const QVector<DashboardDefinition> &dashboards, QString &error);
    static QJsonObject layoutForEnvironment(const DashboardDefinition &dashboard, const QString &environment);

private:
    QByteArray m_revision;
    bool m_loaded = false;
};
