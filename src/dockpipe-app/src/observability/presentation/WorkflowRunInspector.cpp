#include "WorkflowRunInspector.h"
#include "WorkflowRunStateDelegate.h"
#include "ExecutionMetricChart.h"

#include <QDateTime>
#include <QFontDatabase>
#include <QFrame>
#include <QGridLayout>
#include <QLabel>
#include <QPlainTextEdit>
#include <QScrollArea>
#include <QStyle>
#include <QTabWidget>
#include <QVBoxLayout>
#include <cmath>

namespace {
QString recordedTime(const QJsonValue &value)
{
    auto time = QDateTime::fromString(value.toString(), Qt::ISODateWithMs);
    if (!time.isValid())
        time = QDateTime::fromString(value.toString(), Qt::ISODate);
    return time.isValid() ? time.toLocalTime().toString("MMM d, yyyy · HH:mm:ss")
                          : WorkflowRunInspector::tr("Not recorded");
}
}

WorkflowRunInspector::WorkflowRunInspector(QWidget *parent) : QWidget(parent)
{
    setObjectName("workflowRunInspector");
    setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(20, 20, 20, 16);
    layout->setSpacing(12);
    auto *eyebrow = new QLabel(tr("RUN INSPECTOR"));
    eyebrow->setObjectName("workflowInspectorEyebrow");
    layout->addWidget(eyebrow);
    m_title = new QLabel;
    m_title->setObjectName("executionTitle");
    m_title->setTextFormat(Qt::PlainText);
    m_title->setWordWrap(true);
    layout->addWidget(m_title);
    m_subtitle = new QLabel;
    m_subtitle->setObjectName("executionDetails");
    m_subtitle->setTextFormat(Qt::PlainText);
    m_subtitle->setWordWrap(true);
    layout->addWidget(m_subtitle);
    m_state = new QLabel;
    m_state->setObjectName("workflowRunState");
    m_state->setTextFormat(Qt::PlainText);
    layout->addWidget(m_state, 0, Qt::AlignLeft);
    m_content = new QWidget;
    auto *content = new QVBoxLayout(m_content);
    content->setContentsMargins(0, 0, 0, 0);
    content->setSpacing(14);
    const auto metrics = [](QLabel *&first, QLabel *&second, const QString &firstTitle,
                            const QString &secondTitle, const QString &firstName, const QString &secondName) {
        auto *row = new QWidget;
        auto *grid = new QGridLayout(row);
        grid->setContentsMargins(0, 0, 0, 0);
        grid->setHorizontalSpacing(12);
        const QStringList titles{firstTitle, secondTitle};
        const QStringList names{firstName, secondName};
        QList<QLabel *> values;
        for (int index = 0; index < 2; ++index) {
            auto *card = new QFrame;
            card->setObjectName("workflowInspectorMetric");
            auto *cardLayout = new QVBoxLayout(card);
            cardLayout->setContentsMargins(12, 10, 12, 10);
            auto *title = new QLabel(titles[index]);
            title->setObjectName("workflowInspectorLabel");
            cardLayout->addWidget(title);
            auto *value = new QLabel;
            value->setObjectName(names[index]);
            value->setProperty("workflowInspectorValue", true);
            value->setTextFormat(Qt::PlainText);
            value->setWordWrap(true);
            cardLayout->addWidget(value);
            grid->addWidget(card, 0, index);
            grid->setColumnStretch(index, 1);
            values.append(value);
        }
        first = values[0];
        second = values[1];
        return row;
    };
    content->addWidget(metrics(m_duration, m_runtime, tr("Duration"), tr("Runtime"), "workflowRunDuration", "workflowRunRuntime"));
    m_resources = metrics(m_memory, m_cpu, tr("Memory"), tr("CPU usage"), "workflowRunMemory", "workflowRunCpu");
    content->addWidget(m_resources);
    m_tabs = new QTabWidget;
    m_tabs->setObjectName("workflowInspectorTabs");
    m_events = new QPlainTextEdit;
    m_events->setObjectName("executionEvents");
    m_events->setReadOnly(true);
    m_events->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));
    m_events->setMaximumBlockCount(200);
    m_events->setMinimumHeight(150);
    m_events->setPlaceholderText(tr("No lifecycle events recorded."));
    m_tabs->addTab(m_events, tr("Lifecycle"));
    auto *details = new QWidget;
    auto *grid = new QGridLayout(details);
    grid->setContentsMargins(12, 16, 12, 16);
    grid->setHorizontalSpacing(16);
    grid->setVerticalSpacing(14);
    const QStringList names{tr("Execution ID"), tr("Started"), tr("Finished"), tr("PID"),
                            tr("Parent PID"), tr("Container"), tr("Operation log")};
    for (int index = 0; index < names.size(); ++index) {
        auto *label = new QLabel(names[index]);
        label->setObjectName("workflowInspectorLabel");
        grid->addWidget(label, index, 0, Qt::AlignTop);
        m_metadata[index] = new QLabel;
        m_metadata[index]->setTextFormat(Qt::PlainText);
        m_metadata[index]->setTextInteractionFlags(Qt::TextSelectableByMouse);
        m_metadata[index]->setWordWrap(true);
        grid->addWidget(m_metadata[index], index, 1);
    }
    grid->setColumnStretch(1, 1);
    grid->setRowStretch(names.size(), 1);
    auto *scroll = new QScrollArea;
    scroll->setFrameShape(QFrame::NoFrame);
    scroll->setWidgetResizable(true);
    scroll->setWidget(details);
    m_tabs->addTab(scroll, tr("Details"));
    content->addWidget(m_tabs, 1);
    m_chart = new ExecutionMetricChart;
    m_chart->setCaption(tr("Process CPU · one core = 100%"));
    content->addWidget(m_chart);
    layout->addWidget(m_content, 1);
    m_empty = new QLabel(tr("Choose a task to see its outcome, timing, and recorded lifecycle."));
    m_empty->setObjectName("workflowInspectorEmpty");
    m_empty->setWordWrap(true);
    m_empty->setAlignment(Qt::AlignCenter);
    layout->addWidget(m_empty, 1);
    auto *notice = new QLabel(tr("Run output and remote results are in Logs & remote jobs."));
    notice->setObjectName("workflowInspectorNotice");
    notice->setWordWrap(true);
    layout->addWidget(notice);
    clear();
}

void WorkflowRunInspector::clear()
{
    m_identity.clear();
    m_sample.clear();
    m_chart->clear();
    m_events->clear();
    m_title->setText(tr("Select a workflow task"));
    m_subtitle->hide();
    m_state->hide();
    m_content->hide();
    m_empty->show();
    setMinimumHeight(220);
}

void WorkflowRunInspector::setRun(const QString &identity, const QJsonObject &record, const QString &duration,
                                  const QString &memory, const QString &cpu, const QString &events,
                                  const QString &sample, bool showResources, bool history)
{
    if (identity != m_identity) {
        m_identity = identity;
        m_sample.clear();
        m_chart->clear();
        m_tabs->setCurrentIndex(0);
    }
    const QString workflow = record.value("workflow").toString();
    m_title->setText(workflow.isEmpty() ? tr("Standalone task") : workflow);
    const QString machine = record.value("machine").toString();
    QString location = machine;
    if (machine == "local")
        location = tr("This computer");
    else if (machine.isEmpty())
        location = tr("Machine unavailable");
    const QString task = record.value("task").toString();
    m_subtitle->setText(tr("Task %1 · %2").arg(task.isEmpty() ? tr("Standalone") : task, location));
    m_subtitle->show();
    const QString state = record.value("state").toString();
    m_state->setText(WorkflowRunStateDelegate::label(state));
    const QString tone = WorkflowRunStateDelegate::tone(state);
    if (m_state->property("tone").toString() != tone) {
        m_state->setProperty("tone", tone);
        m_state->style()->unpolish(m_state);
        m_state->style()->polish(m_state);
    }
    m_state->show();
    m_duration->setText(duration);
    m_runtime->setText(record.value("runtime").toString(tr("Unavailable")));
    m_memory->setText(memory);
    m_cpu->setText(cpu);
    m_resources->setVisible(showResources);
    if (m_events->toPlainText() != events)
        m_events->setPlainText(events);
    const auto process = record.value("process").toObject();
    m_metadata[0]->setText(record.value("id").toString());
    m_metadata[1]->setText(recordedTime(record.value("started_at")));
    m_metadata[2]->setText(state == "running" ? tr("In progress") : recordedTime(record.value("finished_at")));
    m_metadata[3]->setText(process.value("pid").isDouble() ? QString::number(process.value("pid").toInt()) : tr("Unavailable"));
    m_metadata[4]->setText(record.value("parent_pid").isDouble() ? QString::number(record.value("parent_pid").toInt()) : tr("Unavailable"));
    m_metadata[5]->setText(record.value("container").toString(tr("None recorded")));
    m_metadata[6]->setText(record.value("log").toString(tr("Not recorded")));
    m_empty->hide();
    m_content->show();
    const auto metrics = record.value("metrics").toObject();
    const bool hasCpuMetrics = metrics.value("cpu_seconds").isDouble() || metrics.value("cpu_percent").isDouble();
    m_chart->setVisible(showResources && hasCpuMetrics && !history && state == "running");
    if (sample != m_sample) {
        const auto percent = record.value("metrics").toObject().value("cpu_percent");
        const double value = percent.toDouble(-1);
        m_chart->append(percent.isDouble() && std::isfinite(value) && value >= 0
            ? std::optional<double>(value) : std::nullopt);
        m_sample = sample;
    }
    setMinimumHeight(540);
}
