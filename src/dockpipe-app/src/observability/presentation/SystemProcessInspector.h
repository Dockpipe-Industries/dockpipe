#pragma once

#include <QJsonObject>
#include <QWidget>

class QLabel;
class ExecutionMetricChart;

// Presents the selected observation; it does not acquire metrics or control processes.
class SystemProcessInspector : public QWidget {
    Q_OBJECT
public:
    explicit SystemProcessInspector(QWidget *parent = nullptr);
    static QString stateCode(const QString &state);
    static QString stateLabel(const QString &state);
    void clear();
    void setProcess(const QString &identity, const QJsonObject &process,
                    const QString &memory, const QString &cpu, const QString &sample);

private:
    QString m_identity;
    QString m_sample;
    QLabel *m_title;
    QLabel *m_subtitle;
    QLabel *m_state;
    QLabel *m_cpu;
    QLabel *m_memory;
    QLabel *m_parent;
    QLabel *m_startIdentity;
    QWidget *m_content;
    QLabel *m_empty;
    ExecutionMetricChart *m_chart;
};
