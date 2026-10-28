#pragma once

#include "RemoteCommand.h"
#include <QJsonArray>
#include <QJsonObject>
#include <QMap>
#include <QWidget>

class ResourceCanvas;
class ExecutionMetricChart;
class QCheckBox;
class QComboBox;
class QLabel;
class QLineEdit;
class QPlainTextEdit;
class QPushButton;
class QStackedWidget;
class QTableWidget;
class QTimer;
class MonitoringMachineSelector;
class SystemResourceOverview;

// A view over the CLI snapshot contract. It never owns execution or termination.
class ExecutionExplorer : public QWidget {
    Q_OBJECT
public:
    explicit ExecutionExplorer(QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void setActive(bool active);
    bool applySnapshot(const QJsonObject &snapshot);
    void refresh();
    void setNodes(const QStringList &nodes);

private:
    void refreshWithAuthentication(bool allowAuthentication);
    void render();
    void inspect(const QString &identity);
    void inspectSystem();
    void changeMachine();
    MonitoringMachineSelector *m_machine;
    SystemResourceOverview *m_overview;
    QString processIdentity(const QJsonObject &process) const;
    QString metricText(const QJsonValue &value, const QString &unit) const;
    ExecutionMetricChart *m_chart;
    ResourceCanvas *m_canvas;
    QStackedWidget *m_views;
    QTableWidget *m_system;
    QComboBox *m_mode;
    QCheckBox *m_resources;
    QCheckBox *m_global;
    QLineEdit *m_search;
    QLabel *m_status;
    QLabel *m_title;
    QLabel *m_details;
    QPlainTextEdit *m_events;
    QPushButton *m_more;
    QPushButton *m_pause;
    QPushButton *m_refresh = nullptr;
    QTimer *m_timer;
    RemoteCommand m_command;
    QString m_workdir;
    QString m_selected;
    QJsonObject m_snapshot;
    QMap<QString, double> m_cpuCounters;
    qint64 m_sampleTime = 0;
    QString m_chartSample;
    QMap<QString, QJsonObject> m_records;
    int m_visibleLimit = 32;
    quint64 m_generation = 0;
    bool m_active = false;
};
