#include "ExecutionMetricChart.h"
#include <QPainter>
#include <QPainterPath>
#include <algorithm>

ExecutionMetricChart::ExecutionMetricChart(QWidget *parent) : QWidget(parent)
{
    setMinimumHeight(90);
    setMaximumHeight(120);
    setAccessibleName(tr("Selected process CPU utilization, last 60 samples"));
    m_caption = tr("CPU · one core = 100% · gaps are unavailable");
}

void ExecutionMetricChart::setCaption(const QString &caption)
{
    m_caption = caption;
    setAccessibleName(caption);
    update();
}

void ExecutionMetricChart::clear()
{
    m_samples.clear();
    update();
}

void ExecutionMetricChart::append(std::optional<double> cpuPercent)
{
    m_samples.append(cpuPercent);
    if (m_samples.size() > 60)
        m_samples.removeFirst();
    update();
}

int ExecutionMetricChart::sampleCount() const
{
    return m_samples.size();
}

void ExecutionMetricChart::paintEvent(QPaintEvent *)
{
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing);
    const auto text = palette().color(QPalette::Text);
    painter.setPen(text);
    painter.drawText(QRect(8, 0, width() - 16, 22), m_caption);
    const QRectF plot(8, 28, width() - 16, height() - 38);
    painter.setPen(palette().color(QPalette::Mid));
    painter.drawLine(plot.bottomLeft(), plot.bottomRight());
    double ceiling = 100;
    bool measured = false;
    for (const auto &sample : m_samples) {
        if (sample) {
            measured = true;
            ceiling = std::max(ceiling, *sample);
        }
    }
    if (!measured) {
        painter.setPen(text);
        painter.drawText(plot, Qt::AlignCenter, tr("Waiting for two measurements"));
        return;
    }
    QPainterPath line;
    bool connected = false;
    for (int index = 0; index < m_samples.size(); ++index) {
        if (!m_samples[index]) {
            connected = false;
            continue;
        }
        const QPointF point(plot.left() + index * plot.width() / 59,
            plot.bottom() - *m_samples[index] * plot.height() / ceiling);
        if (connected)
            line.lineTo(point);
        else
            line.moveTo(point);
        connected = true;
    }
    painter.setPen(QPen(palette().color(QPalette::Highlight), 2));
    painter.drawPath(line);
}
