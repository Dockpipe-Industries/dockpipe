#include "SystemProcessInspector.h"
#include "ExecutionMetricChart.h"

#include <QFrame>
#include <QGridLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QStyle>
#include <QVBoxLayout>
#include <cmath>

SystemProcessInspector::SystemProcessInspector(QWidget *parent) : QWidget(parent)
{
    setObjectName("systemProcessInspector");
    setAttribute(Qt::WA_StyledBackground, true);
    setMinimumWidth(340);
    setMinimumHeight(420);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(20, 20, 20, 16);
    layout->setSpacing(12);
    auto *eyebrow = new QLabel(tr("PROCESS INSPECTOR"));
    eyebrow->setObjectName("systemInspectorEyebrow");
    layout->addWidget(eyebrow);
    m_title = new QLabel;
    m_title->setObjectName("systemProcessTitle");
    m_title->setTextFormat(Qt::PlainText);
    m_title->setWordWrap(true);
    layout->addWidget(m_title);
    m_subtitle = new QLabel;
    m_subtitle->setObjectName("systemProcessSubtitle");
    m_subtitle->setTextFormat(Qt::PlainText);
    layout->addWidget(m_subtitle);
    m_state = new QLabel;
    m_state->setObjectName("systemProcessState");
    m_state->setTextFormat(Qt::PlainText);
    layout->addWidget(m_state, 0, Qt::AlignLeft);

    m_content = new QWidget;
    auto *content = new QVBoxLayout(m_content);
    content->setContentsMargins(0, 0, 0, 0);
    content->setSpacing(18);
    auto *metrics = new QHBoxLayout;
    const auto metric = [metrics](const QString &title, const QString &name) {
        auto *card = new QFrame;
        card->setObjectName("systemProcessMetricCard");
        auto *cardLayout = new QVBoxLayout(card);
        cardLayout->setContentsMargins(12, 10, 12, 10);
        auto *label = new QLabel(title);
        label->setObjectName("systemInspectorLabel");
        cardLayout->addWidget(label);
        auto *value = new QLabel;
        value->setObjectName(name);
        value->setTextFormat(Qt::PlainText);
        cardLayout->addWidget(value);
        metrics->addWidget(card, 1);
        return value;
    };
    m_cpu = metric(tr("CPU usage"), "systemProcessCpu");
    m_memory = metric(tr("Memory"), "systemProcessMemory");
    content->addLayout(metrics);
    auto *identity = new QGridLayout;
    identity->setHorizontalSpacing(12);
    identity->setVerticalSpacing(10);
    const auto detail = [identity](int row, const QString &title, const QString &name) {
        auto *label = new QLabel(title);
        label->setObjectName("systemInspectorLabel");
        identity->addWidget(label, row, 0, Qt::AlignTop);
        auto *value = new QLabel;
        value->setObjectName(name);
        value->setTextFormat(Qt::PlainText);
        value->setTextInteractionFlags(Qt::TextSelectableByMouse);
        value->setWordWrap(true);
        identity->addWidget(value, row, 1);
        return value;
    };
    m_parent = detail(0, tr("Parent PID"), "systemProcessParent");
    m_startIdentity = detail(1, tr("Start identity"), "systemProcessIdentity");
    identity->setColumnStretch(1, 1);
    content->addLayout(identity);
    m_chart = new ExecutionMetricChart;
    m_chart->setObjectName("systemProcessCpuHistory");
    m_chart->setCaption(tr("Process CPU · one core = 100%"));
    content->addWidget(m_chart);
    content->addStretch();
    layout->addWidget(m_content, 1);
    m_empty = new QLabel(tr("Choose a row to inspect a process and its resource usage."));
    m_empty->setObjectName("systemProcessEmpty");
    m_empty->setWordWrap(true);
    m_empty->setAlignment(Qt::AlignCenter);
    layout->addWidget(m_empty, 1);
    auto *notice = new QLabel(tr("Read-only process monitoring"));
    notice->setObjectName("systemInspectorNotice");
    layout->addWidget(notice);
    clear();
}

QString SystemProcessInspector::stateCode(const QString &state)
{
    if (state.compare("running", Qt::CaseInsensitive) == 0)
        return "R";
    if (state.compare("sleeping", Qt::CaseInsensitive) == 0)
        return "S";
    if (state.compare("stopped", Qt::CaseInsensitive) == 0)
        return "T";
    return state;
}

QString SystemProcessInspector::stateLabel(const QString &reportedState)
{
    const QString state = stateCode(reportedState);
    if (state == "R")
        return tr("Running");
    if (state == "S")
        return tr("Sleeping");
    if (state == "D")
        return tr("Waiting for I/O");
    if (state == "T")
        return tr("Stopped");
    if (state == "t")
        return tr("Tracing stop");
    if (state == "Z")
        return tr("Zombie");
    if (state == "I")
        return tr("Idle");
    if (state == "X" || state == "x")
        return tr("Dead");
    if (state.isEmpty())
        return tr("Unavailable");
    return state;
}

void SystemProcessInspector::clear()
{
    m_identity.clear();
    m_sample.clear();
    m_chart->clear();
    m_title->setText(tr("Select a process"));
    m_subtitle->hide();
    m_state->hide();
    m_content->hide();
    m_empty->show();
}

void SystemProcessInspector::setProcess(const QString &identity, const QJsonObject &process,
                                       const QString &memory, const QString &cpu, const QString &sample)
{
    if (m_identity != identity) {
        m_chart->clear();
        m_sample.clear();
        m_identity = identity;
    }
    const QString name = process.value("name").toString();
    m_title->setText(name.isEmpty() ? tr("Unnamed process") : name);
    const auto pid = process.value("pid");
    m_subtitle->setText(pid.isDouble() ? tr("PID %1").arg(pid.toInt()) : tr("PID unavailable"));
    m_subtitle->show();
    const QString state = process.value("state").toString();
    m_state->setText(stateLabel(state));
    m_state->setToolTip(state);
    QString tone = "neutral";
    const QString code = stateCode(state);
    if (code == "R")
        tone = "active";
    else if (code == "Z" || code == "X" || code == "x")
        tone = "warning";
    if (m_state->property("tone").toString() != tone) {
        m_state->setProperty("tone", tone);
        m_state->style()->unpolish(m_state);
        m_state->style()->polish(m_state);
    }
    m_state->show();
    m_cpu->setText(cpu);
    m_memory->setText(memory);
    const auto parent = process.value("parent_pid");
    m_parent->setText(parent.isDouble() ? QString::number(parent.toInt()) : tr("Unavailable"));
    const QString startIdentity = process.value("start_token").toString();
    m_startIdentity->setText(startIdentity.isEmpty() ? tr("Unavailable") : startIdentity);
    m_empty->hide();
    m_content->show();
    m_chart->setVisible(process.value("cpu_seconds").isDouble() || process.value("cpu_percent").isDouble());
    if (m_sample != sample) {
        const auto percent = process.value("cpu_percent");
        const double value = percent.toDouble(-1);
        m_chart->append(percent.isDouble() && std::isfinite(value) && value >= 0
            ? std::optional<double>(value) : std::nullopt);
        m_sample = sample;
    }
}
