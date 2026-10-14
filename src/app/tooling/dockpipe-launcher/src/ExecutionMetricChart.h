#pragma once

#include <QVector>
#include <QWidget>
#include <optional>

// One selected resource, at most four minutes of four-second samples.
class ExecutionMetricChart : public QWidget {
public:
    void setCaption(const QString &caption);
    explicit ExecutionMetricChart(QWidget *parent = nullptr);
    void clear();
    void append(std::optional<double> cpuPercent);
    int sampleCount() const;
protected:
    void paintEvent(QPaintEvent *event) override;
private:
    QString m_caption;
    QVector<std::optional<double>> m_samples;
};
