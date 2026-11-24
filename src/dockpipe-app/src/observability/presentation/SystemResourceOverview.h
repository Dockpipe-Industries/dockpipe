#pragma once

#include <QCoreApplication>

#include <QJsonObject>
#include <QWidget>

class QLabel;
class QProgressBar;
class QGridLayout;
class QResizeEvent;
class ExecutionMetricChart;

class SystemResourceOverview : public QWidget {
    Q_DECLARE_TR_FUNCTIONS(SystemResourceOverview)
public:
    explicit SystemResourceOverview(QWidget *parent = nullptr, bool compact = false);
    void applySnapshot(const QJsonObject &snapshot);
    void clear();

protected:
    void resizeEvent(QResizeEvent *event) override;

private:
    QGridLayout *m_grid;
    QWidget *m_cards[6];
    QLabel *m_captions[6];
    QWidget *m_chartCard;
    int m_columns = 3;
    bool m_compact = false;
    QLabel *m_values[6];
    QProgressBar *m_bars[4];
    ExecutionMetricChart *m_chart;
    QJsonObject m_previous;
    QString m_identity;
    qint64 m_sampleTime = 0;
};
