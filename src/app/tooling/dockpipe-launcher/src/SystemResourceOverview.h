#pragma once

#include <QJsonObject>
#include <QWidget>

class QLabel;
class QProgressBar;
class ExecutionMetricChart;

class SystemResourceOverview : public QWidget {
public:
    explicit SystemResourceOverview(QWidget *parent = nullptr);
    void applySnapshot(const QJsonObject &snapshot);
    void clear();

private:
    QLabel *m_values[6];
    QProgressBar *m_bars[4];
    ExecutionMetricChart *m_chart;
    QJsonObject m_previous;
    QString m_identity;
    qint64 m_sampleTime = 0;
};
