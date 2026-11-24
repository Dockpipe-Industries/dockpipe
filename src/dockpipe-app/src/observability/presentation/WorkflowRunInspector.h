#pragma once

#include <QJsonObject>
#include <QWidget>

class QLabel;
class QPlainTextEdit;
class QTabWidget;
class ExecutionMetricChart;

class WorkflowRunInspector : public QWidget {
    Q_OBJECT
public:
    explicit WorkflowRunInspector(QWidget *parent = nullptr);
    void clear();
    void setRun(const QString &identity, const QJsonObject &record, const QString &duration,
                const QString &memory, const QString &cpu, const QString &events,
                const QString &sample, bool showResources, bool history);

private:
    QString m_identity;
    QString m_sample;
    QLabel *m_title;
    QLabel *m_subtitle;
    QLabel *m_state;
    QLabel *m_duration;
    QLabel *m_runtime;
    QLabel *m_memory;
    QLabel *m_cpu;
    QLabel *m_metadata[7];
    QLabel *m_empty;
    QWidget *m_content;
    QWidget *m_resources;
    QTabWidget *m_tabs;
    QPlainTextEdit *m_events;
    ExecutionMetricChart *m_chart;
};
