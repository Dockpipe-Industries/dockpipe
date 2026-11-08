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
class QSplitter;
class QResizeEvent;
class MonitoringMachineSelector;
class SystemResourceOverview;
class SystemProcessInspector;
class WorkflowRunInspector;

// A view over the CLI snapshot contract. It never owns execution or termination.
class ExecutionExplorer : public QWidget {
    Q_OBJECT
public:
    enum class Purpose { WorkflowRuns, System };
    explicit ExecutionExplorer(QWidget *parent = nullptr, Purpose purpose = Purpose::WorkflowRuns);
    void setHistory(bool history);
    void setListView(bool list);
    void setWorkdir(const QString &workdir);
    void setActive(bool active);
    bool applySnapshot(const QJsonObject &snapshot);
    void refresh();
    void setNodes(const QStringList &nodes);

protected:
    void resizeEvent(QResizeEvent *event) override;

private:
    QWidget *buildSystemPage();
    QWidget *buildRunSummary();
    void updateRunSummary();
    void updateRunLayout();
    void updatePresentation();
    void clearSystemProcesses();
    void updateSystemLayout();
    int visibleRecordCeiling() const;
    void refreshWithAuthentication(bool allowAuthentication);
    void render();
    void inspect(const QString &identity);
    void inspectSystem();
    void changeMachine();
    MonitoringMachineSelector *m_machine;
    SystemResourceOverview *m_overview;
    QString processIdentity(const QJsonObject &process) const;
    QString metricText(const QJsonValue &value, const QString &unit) const;
    QJsonValue runDuration(const QJsonObject &record) const;
    ResourceCanvas *m_canvas;
    QWidget *m_inspector;
    WorkflowRunInspector *m_runInspector;
    QWidget *m_runSummary;
    QWidget *m_runSummaryCards[4];
    QLabel *m_runSummaryTitles[4];
    QLabel *m_runSummaryValues[4];
    QLabel *m_runSummaryCaptions[4];
    QLabel *m_runScope;
    QLabel *m_runTableTitle;
    QLabel *m_runResults;
    QWidget *m_runSearchHost;
    QComboBox *m_runStateFilter;
    QWidget *m_canvasInspectorHost;
    QWidget *m_runListPage;
    QSplitter *m_runListSplitter;
    QTableWidget *m_runs;
    QLabel *m_emptyRuns;
    QLabel *m_pageTitle;
    QLabel *m_pageDescription;
    QPushButton *m_canvasViewButton;
    QPushButton *m_listViewButton;
    bool m_history = false;
    bool m_listView = false;
    QStackedWidget *m_views;
    QTableWidget *m_system;
    SystemProcessInspector *m_systemInspector;
    QSplitter *m_systemSplitter;
    QWidget *m_systemSearchHost;
    QWidget *m_workflowSearchHost;
    QWidget *m_workflowControls;
    QWidget *m_machineBar;
    QLabel *m_monitoringCadence;
    QLabel *m_processCount;
    QLabel *m_emptyProcesses;
    QComboBox *m_processFilter;
    QComboBox *m_mode;
    QCheckBox *m_resources;
    QCheckBox *m_global;
    QLineEdit *m_search;
    QLabel *m_status;
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
    QMap<QString, QJsonObject> m_records;
    int m_visibleLimit = 32;
    quint64 m_generation = 0;
    bool m_active = false;
};
