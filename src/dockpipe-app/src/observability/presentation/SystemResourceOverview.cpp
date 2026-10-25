#include "SystemResourceOverview.h"
#include "ExecutionMetricChart.h"
#include <QDateTime>
#include <QFrame>
#include <QGridLayout>
#include <QLabel>
#include <QProgressBar>
#include <QVBoxLayout>
#include <cmath>

namespace {
QString bytes(double value)
{
    const QStringList units{"B", "KiB", "MiB", "GiB", "TiB"};
    int unit = 0;
    while (value >= 1024 && unit < units.size() - 1) {
        value /= 1024;
        ++unit;
    }
    return QString::number(value, 'f', unit == 0 ? 0 : 1) + " " + units[unit];
}

bool measured(const QJsonObject &object, const QString &key)
{
    const auto value = object.value(key);
    return value.isDouble() && std::isfinite(value.toDouble()) && value.toDouble() >= 0;
}
}

SystemResourceOverview::SystemResourceOverview(QWidget *parent, bool compact) : QWidget(parent), m_compact(compact)
{
    setObjectName("systemResourceOverview");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(compact ? 0 : 20, 0, compact ? 0 : 20, 0);
    auto *grid = new QGridLayout;
    const QStringList titles{tr("CPU"), tr("Memory"), tr("Swap"), tr("Disk"), tr("Network"), tr("Uptime & load")};
    for (int index = 0; index < titles.size(); ++index) {
        auto *card = new QFrame;
        card->setObjectName("systemMetricCard");
        card->setFrameShape(QFrame::StyledPanel);
        auto *cardLayout = new QVBoxLayout(card);
        auto *title = new QLabel(titles[index]);
        title->setObjectName("systemMetricTitle");
        auto font = title->font();
        font.setBold(true);
        title->setFont(font);
        cardLayout->addWidget(title);
        m_values[index] = new QLabel;
        m_values[index]->setObjectName("systemMetric" + QString::number(index));
        m_values[index]->setTextFormat(Qt::PlainText);
        m_values[index]->setWordWrap(true);
        auto valueFont = m_values[index]->font();
        valueFont.setPointSize(compact ? 11 : 13);
        m_values[index]->setFont(valueFont);
        cardLayout->addWidget(m_values[index]);
        if (index < 4) {
            m_bars[index] = new QProgressBar;
            m_bars[index]->setRange(0, 100);
            m_bars[index]->setTextVisible(false);
            m_bars[index]->setMaximumHeight(8);
            cardLayout->addWidget(m_bars[index]);
        }
        const int columns = compact ? 2 : 3;
        grid->addWidget(card, index / columns, index % columns);
    }
    m_values[3]->setToolTip(tr("Capacity of the collector's root filesystem on Linux or system volume on Windows. Includes reserved space in used capacity."));
    m_values[4]->setToolTip(tr("Sum of non-loopback interfaces. Bridges, VPNs and virtual interfaces may count the same traffic more than once."));
    layout->addLayout(grid);
    m_chart = new ExecutionMetricChart;
    m_chart->setCaption(tr("System CPU · all cores = 100% · last 60 samples"));
    layout->addWidget(m_chart);
    m_chart->setVisible(!compact);
    clear();
}

void SystemResourceOverview::clear()
{
    m_previous = {};
    m_identity.clear();
    m_sampleTime = 0;
    m_chart->clear();
    for (auto *label : m_values)
        label->setText(tr("Unavailable"));
    for (auto *bar : m_bars) {
        bar->setValue(0);
        bar->setEnabled(false);
    }
}

void SystemResourceOverview::applySnapshot(const QJsonObject &snapshot)
{
    const auto resources = snapshot.value("system").toObject();
    const QString boot = resources.value("boot_id").toString();
    const QString identity = snapshot.value("machine").toString() + ":" + boot;
    const qint64 sampleTime = QDateTime::fromString(snapshot.value("captured_at").toString(), Qt::ISODateWithMs).toMSecsSinceEpoch();
    if (identity != m_identity || sampleTime < m_sampleTime)
        clear();
    // The broker can return the same sample more than once. Preserve rates and
    // history until a new worker measurement arrives.
    if (sampleTime == m_sampleTime && !m_previous.isEmpty())
        return;
    const bool comparable = !boot.isEmpty() && identity == m_identity && sampleTime > m_sampleTime && m_sampleTime > 0;
    std::optional<double> cpu;
    if (comparable && measured(resources, "cpu_total") && measured(resources, "cpu_idle")
        && measured(m_previous, "cpu_total") && measured(m_previous, "cpu_idle")) {
        const double total = resources.value("cpu_total").toDouble() - m_previous.value("cpu_total").toDouble();
        const double idle = resources.value("cpu_idle").toDouble() - m_previous.value("cpu_idle").toDouble();
        if (total > 0 && idle >= 0 && idle <= total)
            cpu = 100 * (total - idle) / total;
    }
    QString cpuText = tr("Unavailable");
    if (cpu)
        cpuText = tr("%1% · %2 logical CPUs").arg(*cpu, 0, 'f', 1).arg(resources.value("logical_cpus").toInt());
    else if (measured(resources, "cpu_total"))
        cpuText = tr("Waiting for measurements");
    m_values[0]->setText(cpuText);
    m_chart->setVisible(!m_compact && measured(resources, "cpu_total"));
    m_bars[0]->setEnabled(cpu.has_value());
    m_bars[0]->setValue(cpu ? qRound(*cpu) : 0);
    m_chart->append(cpu);
    const QStringList totals{"memory_total", "swap_total", "disk_total"};
    const QStringList available{"memory_available", "swap_free", "disk_available"};
    for (int index = 0; index < 3; ++index) {
        const double total = resources.value(totals[index]).toDouble();
        const double free = resources.value(available[index]).toDouble();
        const bool valid = measured(resources, totals[index]) && measured(resources, available[index]) && free <= total;
        m_values[index + 1]->setText(valid ? tr("%1 / %2 used").arg(bytes(total - free), bytes(total)) : tr("Unavailable"));
        m_bars[index + 1]->setEnabled(valid);
        m_bars[index + 1]->setValue(valid && total > 0 ? qRound(100 * (total - free) / total) : 0);
    }
    QStringList rates;
    for (const auto &key : {QStringLiteral("network_received"), QStringLiteral("network_sent")}) {
        if (comparable && measured(resources, key) && measured(m_previous, key)
            && resources.value(key).toDouble() >= m_previous.value(key).toDouble()) {
            const double rate = 1000 * (resources.value(key).toDouble() - m_previous.value(key).toDouble()) / (sampleTime - m_sampleTime);
            rates << bytes(rate) + "/s";
        } else {
            rates << tr("Unavailable");
        }
    }
    m_values[4]->setText(tr("↓ %1   ↑ %2").arg(rates[0], rates[1]));
    const QString uptime = measured(resources, "uptime_seconds")
        ? tr("%1 h").arg(resources.value("uptime_seconds").toDouble() / 3600, 0, 'f', 1) : tr("Unavailable");
    if (measured(resources, "load_1")) {
        const QString load = QString::number(resources.value("load_1").toDouble(), 'f', 2);
        m_values[5]->setText(tr("%1 · load %2 (1 min)").arg(uptime, load));
    } else {
        m_values[5]->setText(uptime);
    }
    m_previous = resources;
    m_identity = identity;
    m_sampleTime = sampleTime;
}
