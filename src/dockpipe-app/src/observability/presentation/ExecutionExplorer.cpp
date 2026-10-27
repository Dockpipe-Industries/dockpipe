#include "ExecutionExplorer.h"
#include "MonitoringMachineSelector.h"
#include "SystemResourceOverview.h"
#include "ExecutionMetricChart.h"
#include <cmath>
#include <algorithm>
#include "CanvasIcons.h"
#include "DockpipeChoices.h"
#include "ResourceCanvas.h"

#include <QCheckBox>
#include <QComboBox>
#include <QDateTime>
#include <QHeaderView>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QStackedWidget>
#include <QTableWidget>
#include <QTimer>
#include <QVBoxLayout>
#include <QSet>

ExecutionExplorer::ExecutionExplorer(QWidget *parent) : QWidget(parent), m_command(this)
{
    setObjectName("executionExplorerPage");
    setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *toolbar = new QHBoxLayout;
    toolbar->setContentsMargins(20, 12, 20, 4);
    m_mode = new QComboBox;
    m_mode->setObjectName("executionMode");
    m_mode->addItems({tr("Dockpipe Managed"), tr("System")});
    toolbar->addWidget(m_mode);
    m_machine = new MonitoringMachineSelector;
    toolbar->addWidget(m_machine);
    m_global = new QCheckBox(tr("Standalone"));
    m_global->setToolTip(tr("Show operations recorded without a project."));
    toolbar->addWidget(m_global);
    m_search = new QLineEdit;
    m_search->setObjectName("executionSearch");
    m_search->setPlaceholderText(tr("Find a workflow, task or process…"));
    toolbar->addWidget(m_search, 1);
    m_resources = new QCheckBox(tr("Resource metrics"));
    m_resources->setChecked(true);
    toolbar->addWidget(m_resources);
    m_pause = new QPushButton(tr("Pause updates"));
    m_pause->setObjectName("executionPause");
    m_pause->setCheckable(true);
    toolbar->addWidget(m_pause);
    m_more = new QPushButton(tr("Show more"));
    toolbar->addWidget(m_more);
    layout->addLayout(toolbar);
    m_overview = new SystemResourceOverview;
    m_overview->hide();
    layout->addWidget(m_overview);
    connect(m_machine, qOverload<int>(&QComboBox::currentIndexChanged), this, &ExecutionExplorer::changeMachine);
    connect(m_mode, qOverload<int>(&QComboBox::currentIndexChanged), this, [this](int mode) {
        m_overview->setVisible(mode == 1);
    });

    auto *inspector = new QWidget;
    auto *inspectorLayout = new QVBoxLayout(inspector);
    m_title = new QLabel(tr("Select an execution"));
    m_title->setObjectName("executionTitle");
    m_title->setTextFormat(Qt::PlainText);
    m_title->setWordWrap(true);
    QFont titleFont = m_title->font();
    titleFont.setPointSize(16);
    titleFont.setBold(true);
    m_title->setFont(titleFont);
    inspectorLayout->addWidget(m_title);
    m_details = new QLabel(tr("Managed records come from Dockpipe's execution boundaries. System processes are discovered and are never treated as owned."));
    m_details->setObjectName("executionDetails");
    m_details->setTextFormat(Qt::PlainText);
    m_details->setWordWrap(true);
    m_details->setTextInteractionFlags(Qt::TextSelectableByMouse);
    inspectorLayout->addWidget(m_details);
    m_chart = new ExecutionMetricChart;
    inspectorLayout->addWidget(m_chart);
    inspectorLayout->addWidget(new QLabel(tr("Lifecycle & operation log")));
    m_events = new QPlainTextEdit;
    m_events->setObjectName("executionEvents");
    m_events->setReadOnly(true);
    m_events->setMaximumBlockCount(200);
    m_events->setMinimumHeight(170);
    m_events->setPlaceholderText(tr("Select an execution to see its lifecycle. Raw command output stays in the existing run logs."));
    inspectorLayout->addWidget(m_events, 1);
    m_canvas = new ResourceCanvas("executionExplorer", tr("Execution Explorer"),
        tr("Workflow → task → process · authoritative managed associations"), inspector, this, false);
    m_canvas->setObjectName("executionCanvas");
    m_canvas->setDetailsLabel(tr("Execution details & lifecycle"));
    m_canvas->addInactiveFilter(tr("Finished"));
    m_views = new QStackedWidget;
    m_views->addWidget(m_canvas);
    auto *systemPage = new QWidget;
    auto *systemLayout = new QVBoxLayout(systemPage);
    auto *systemNotice = new QLabel(tr("Discovered processes · best effort · no ownership inferred. Select a row to inspect its identity and metrics."));
    systemNotice->setWordWrap(true);
    systemLayout->addWidget(systemNotice);
    m_system = new QTableWidget(0, 6);
    m_system->setObjectName("systemProcesses");
    m_system->setHorizontalHeaderLabels({tr("Process"), tr("PID"), tr("Parent PID"), tr("State"), tr("Memory"), tr("CPU")});
    m_system->horizontalHeader()->setSectionResizeMode(QHeaderView::Stretch);
    m_system->horizontalHeader()->setSectionsClickable(true);
    m_system->horizontalHeader()->setSortIndicatorShown(true);
    m_system->horizontalHeader()->setSortIndicator(0, Qt::AscendingOrder);
    connect(m_system->horizontalHeader(), &QHeaderView::sortIndicatorChanged, this, [this]() {
        if (m_mode->currentIndex() == 1)
            render();
    });
    m_system->verticalHeader()->hide();
    m_system->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_system->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_system->setSelectionMode(QAbstractItemView::SingleSelection);
    systemLayout->addWidget(m_system);
    m_views->addWidget(systemPage);
    layout->addWidget(m_views, 1);
    m_status = new QLabel(tr("Choose a project to inspect its recent executions."));
    m_status->setObjectName("executionStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_status->setMargin(12);
    layout->addWidget(m_status);
    m_timer = new QTimer(this);
    m_timer->setInterval(4000);
    connect(m_timer, &QTimer::timeout, this, &ExecutionExplorer::refresh);
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, &ExecutionExplorer::inspect);
    connect(m_search, &QLineEdit::textChanged, this, [this]() { render(); });
    connect(m_resources, &QCheckBox::toggled, this, [this]() { ++m_generation; refresh(); });
    connect(m_global, &QCheckBox::toggled, this, [this]() {
        ++m_generation;
        m_cpuCounters.clear();
        m_snapshot = {};
        m_selected.clear();
        render();
        refresh();
    });
    connect(m_pause, &QPushButton::toggled, this, [this](bool paused) {
        ++m_generation;
        m_cpuCounters.clear();
        m_pause->setText(paused ? tr("Resume updates") : tr("Pause updates"));
        if (paused) {
            m_timer->stop();
            m_command.cancel();
            m_status->setText(tr("Monitoring paused. Displayed values are a snapshot."));
        } else if (m_active) {
            m_timer->start();
            refresh();
        }
    });
    connect(m_mode, qOverload<int>(&QComboBox::currentIndexChanged), this, [this](int mode) {
        ++m_generation;
        m_cpuCounters.clear();
        m_chart->clear();
        m_chartSample.clear();
        m_visibleLimit = mode == 0 ? 32 : 250;
        m_selected.clear();
        m_records.clear();
        m_snapshot = {};
        m_views->setCurrentIndex(mode);
        render();
        refresh();
    });
    connect(m_more, &QPushButton::clicked, this, [this]() {
        const int ceiling = m_mode->currentIndex() == 0 ? 64 : 4096;
        m_visibleLimit = qMin(ceiling, m_visibleLimit * 2);
        render();
    });
    connect(m_system, &QTableWidget::itemSelectionChanged, this, &ExecutionExplorer::inspectSystem);
}

void ExecutionExplorer::inspectSystem()
{
    if (m_system->currentRow() < 0)
        return;
    const QString key = m_system->item(m_system->currentRow(), 0)->data(Qt::UserRole).toString();
    m_selected = key;
    const auto record = m_records.value(key);
    m_status->setText(tr("Discovered · %1 · PID %2 · parent %3 · start identity %4 · memory %5 · CPU %6")
        .arg(record.value("name").toString()).arg(record.value("pid").toInt())
        .arg(record.value("parent_pid").toInt()).arg(record.value("start_token").toString(tr("Unavailable")))
        .arg(metricText(record.value("memory_bytes"), "bytes")).arg(metricText(record.value("cpu_percent"), "%")));
}

void ExecutionExplorer::setWorkdir(const QString &workdir)
{
    if (m_workdir == workdir)
        return;
    m_cpuCounters.clear();
    m_chart->clear();
    m_chartSample.clear();
    m_workdir = workdir;
    ++m_generation;
    m_snapshot = {};
    m_selected.clear();
    render();
    refresh();
}

void ExecutionExplorer::setActive(bool active)
{
    m_active = active;
    if (active && !m_pause->isChecked()) {
        m_timer->start();
        refresh();
    } else {
        ++m_generation;
        m_timer->stop();
        m_command.cancel();
    }
}

void ExecutionExplorer::refresh()
{
    if (!m_active || m_pause->isChecked() || m_command.busy())
        return;
    QStringList args{"runs", "observe", "--json"};
    if (m_global->isChecked() || m_workdir.isEmpty())
        args << "--global";
    else
        args << "--workdir" << m_workdir;
    if (m_mode->currentIndex() == 1) {
        args << "--system";
    } else {
        const QString execution = m_records.value(m_selected).value("id").toString();
        if (!execution.isEmpty())
            args << "--execution" << execution;
    }
    if (m_resources->isChecked()) {
        args << "--resources";
        const QString container = m_records.value(m_selected).value("container").toString();
        if (!container.isEmpty())
            args << "--container" << container;
    }
    const QString node = m_machine->currentData().toString();
    QString program;
    if (!node.isEmpty()) {
        args = {"remote", "observe", "--node", node};
        QString error;
        program = DockpipeChoices::operatorDockpipeBinary(&error);
        if (program.isEmpty()) {
            m_overview->clear();
            m_system->setRowCount(0);
            m_cpuCounters.clear();
            m_sampleTime = 0;
            m_snapshot = {};
            m_status->setText(error);
            return;
        }
    } else {
        program = DockpipeChoices::preferredDockpipeBinary(m_workdir);
    }
    const quint64 generation = m_generation;
    m_command.run(program, args,
        [this, generation, node](bool ok, const QByteArray &output) {
            if (generation != m_generation)
                return;
            const auto document = QJsonDocument::fromJson(output);
            auto snapshot = document.object();
            if (!node.isEmpty()) {
                if (!ok || snapshot.value("schema").toString() != "dockpipe.machine_snapshot.v1"
                    || snapshot.value("node").toString() != node || snapshot.value("state").toString() != "live"
                    || !snapshot.value("execution").isObject()) {
                    m_overview->clear();
                    m_system->setRowCount(0);
                    m_cpuCounters.clear();
                    m_sampleTime = 0;
                    m_snapshot = {};
                    m_status->setText(tr("%1 · System monitoring unavailable or stale. On the paired worker, enable it with: dockpipe remote monitoring --system --docker. The worker and broker must support monitoring.").arg(node));
                    return;
                }
                snapshot = snapshot.value("execution").toObject();
                if (snapshot.value("captured_at") == m_snapshot.value("captured_at"))
                    return;
            }
            if (!ok || !document.isObject() || !applySnapshot(snapshot)) {
                m_overview->clear();
                m_status->setText(tr("Monitoring unavailable. Displayed data may be stale. Check the CLI version and machine permissions."));
            }
        }, 5000);
}

void ExecutionExplorer::setNodes(const QStringList &nodes)
{
    m_machine->setNodes(nodes);
}

void ExecutionExplorer::changeMachine()
{
    ++m_generation;
    m_command.cancel();
    m_selected.clear();
    m_snapshot = {};
    m_records.clear();
    m_cpuCounters.clear();
    m_sampleTime = 0;
    m_chart->clear();
    m_overview->clear();
    m_system->setRowCount(0);
    const bool remote = !m_machine->currentData().toString().isEmpty();
    m_mode->setEnabled(!remote);
    m_global->setEnabled(!remote);
    m_resources->setEnabled(!remote);
    if (remote)
        m_mode->setCurrentIndex(1);
    m_status->setText(tr("Waiting for the selected machine…"));
    QTimer::singleShot(0, this, &ExecutionExplorer::refresh);
}

bool ExecutionExplorer::applySnapshot(const QJsonObject &snapshot)
{
    if (snapshot.value("schema").toString() != "dockpipe.execution_snapshot.v1"
        || !snapshot.value("executions").isArray()
        || !snapshot.value("processes").isArray()
        || snapshot.value("executions").toArray().size() > 256
        || snapshot.value("processes").toArray().size() > 4096)
        return false;
    QJsonObject measured = snapshot;
    m_overview->applySnapshot(snapshot);
    const qint64 sampleTime = QDateTime::fromString(snapshot.value("captured_at").toString(), Qt::ISODateWithMs).toMSecsSinceEpoch();
    QMap<QString, double> nextCounters;
    const auto measure = [&](QJsonObject process) {
        const QString identity = processIdentity(process);
        const auto counter = process.value("cpu_seconds");
        if (counter.isDouble() && !process.value("start_token").toString().isEmpty()) {
            const double current = counter.toDouble();
            if (m_cpuCounters.contains(identity) && sampleTime > m_sampleTime) {
                const double delta = current - m_cpuCounters.value(identity);
                const double percent = 100000.0 * delta / (sampleTime - m_sampleTime);
                if (delta >= 0 && std::isfinite(percent))
                    process.insert("cpu_percent", percent);
            }
            nextCounters.insert(identity, current);
        }
        return process;
    };
    for (const QString &collection : {QStringLiteral("processes"), QStringLiteral("executions")}) {
        QJsonArray rows;
        for (const auto &value : snapshot.value(collection).toArray()) {
            auto record = value.toObject();
            if (collection == "processes") {
                record = measure(record);
            } else {
                auto process = record.value("metrics").toObject();
                if (!process.isEmpty())
                    record.insert("metrics", measure(process));
                auto resource = record.value("resource").toObject();
                if (!resource.isEmpty()) {
                    process = resource.value("metrics").toObject();
                    if (!process.isEmpty())
                        resource.insert("metrics", measure(process));
                    record.insert("resource", resource);
                }
            }
            rows.append(record);
        }
        measured.insert(collection, rows);
    }
    m_cpuCounters = nextCounters;
    m_sampleTime = sampleTime;
    m_snapshot = measured;
    render();
    return true;
}

QString ExecutionExplorer::processIdentity(const QJsonObject &process) const
{
    return QString::number(process.value("pid").toInt()) + ":" + process.value("start_token").toString();
}

QString ExecutionExplorer::metricText(const QJsonValue &value, const QString &unit) const
{
    if (!value.isDouble())
        return tr("Unavailable");
    const double number = value.toDouble();
    if (unit == "bytes") {
        if (number >= 1024 * 1024 * 1024)
            return QString::number(number / (1024 * 1024 * 1024), 'f', 1) + " GiB";
        return QString::number(number / (1024 * 1024), 'f', 1) + " MiB";
    }
    if (unit == "ms")
        return QString::number(number / 1000, 'f', 1) + tr(" s");
    return QString::number(number, 'g', 6) + (unit.isEmpty() ? QString() : " " + unit);
}

void ExecutionExplorer::render()
{
    m_records.clear();
    const bool system = m_mode->currentIndex() == 1;
    QVector<QJsonObject> records;
    for (const auto &value : m_snapshot.value(system ? "processes" : "executions").toArray())
        records.append(value.toObject());
    if (system) {
        const QStringList keys{"name", "pid", "parent_pid", "state", "memory_bytes", "cpu_percent"};
        const int column = m_system->horizontalHeader()->sortIndicatorSection();
        const QString key = keys.value(column, "name");
        const bool numeric = column == 1 || column == 2 || column == 4 || column == 5;
        const bool ascending = m_system->horizontalHeader()->sortIndicatorOrder() == Qt::AscendingOrder;
        // Sort the full retained snapshot before applying the visible-row limit,
        // so the busiest process can appear even if it arrived after row 250.
        std::stable_sort(records.begin(), records.end(), [key, numeric, ascending](const QJsonObject &left, const QJsonObject &right) {
            const auto leftValue = left.value(key);
            const auto rightValue = right.value(key);
            int comparison = 0;
            if (numeric) {
                const bool leftAvailable = leftValue.isDouble() && std::isfinite(leftValue.toDouble());
                const bool rightAvailable = rightValue.isDouble() && std::isfinite(rightValue.toDouble());
                if (leftAvailable != rightAvailable)
                    return leftAvailable; // Unavailable metrics stay last in either direction.
                if (leftAvailable && leftValue.toDouble() != rightValue.toDouble())
                    return ascending ? leftValue.toDouble() < rightValue.toDouble() : leftValue.toDouble() > rightValue.toDouble();
            } else {
                comparison = QString::compare(leftValue.toString(), rightValue.toString(), Qt::CaseInsensitive);
            }
            if (comparison != 0)
                return ascending ? comparison < 0 : comparison > 0;
            if (left.value("pid").toInt() != right.value("pid").toInt())
                return left.value("pid").toInt() < right.value("pid").toInt();
            return left.value("start_token").toString() < right.value("start_token").toString();
        });
    }
    const QString query = m_search->text().trimmed();
    QVector<ResourceNode> nodes;
    QVector<ResourceEdge> edges;
    QSet<QString> groups;
    int matching = 0;
    m_system->blockSignals(true);
    m_system->setRowCount(0);
    for (const auto &record : records) {
        const QString title = system ? record.value("name").toString() : record.value("workflow").toString(tr("Standalone execution"));
        const QString searchable = title + " " + record.value("task").toString() + " " + record.value("id").toString();
        if (!searchable.contains(query, Qt::CaseInsensitive))
            continue;
        if (++matching > m_visibleLimit)
            continue;
        const QString key = system ? processIdentity(record) : record.value("id").toString();
        m_records.insert(key, record);
        if (system) {
            const int row = m_system->rowCount();
            m_system->insertRow(row);
            QStringList cells{title, QString::number(record.value("pid").toInt()), QString::number(record.value("parent_pid").toInt()),
                record.value("state").toString(), metricText(record.value("memory_bytes"), "bytes"), metricText(record.value("cpu_percent"), "%")};
            for (int column = 0; column < cells.size(); ++column)
                m_system->setItem(row, column, new QTableWidgetItem(cells[column]));
            m_system->item(row, 0)->setData(Qt::UserRole, key);
            if (key == m_selected)
                m_system->selectRow(row);
            continue;
        }
        const QString workflow = "workflow:" + record.value("workflow_run").toString(key);
        const QString task = workflow + ":task:" + record.value("task").toString("standalone");
        if (!groups.contains(workflow)) {
            nodes.append({workflow, "workflow", title, tr("This computer"), tr("Managed"), 0});
            groups.insert(workflow);
        }
        if (!groups.contains(task)) {
            nodes.append({task, "task", record.value("task").toString(tr("Standalone operation")), record.value("runtime").toString(), tr("Managed"), 1});
            edges.append({workflow, task, tr("task"), false});
            groups.insert(task);
        }
        const QString state = record.value("state").toString();
        ResourceStatus status = ResourceStatus::Inactive;
        if (state == "running")
            status = ResourceStatus::Running;
        else if (state == "failed")
            status = ResourceStatus::Error;
        else if (state == "unverified" || state == "cancelled")
            status = ResourceStatus::Warning;
        const int pid = record.value("process").toObject().value("pid").toInt();
        nodes.append({key, "process", tr("PID %1 · %2").arg(pid).arg(record.value("runtime").toString()),
            key, state, 2, status, state != "running", state == "running"});
        edges.append({task, key, tr("managed"), false});
        const auto resource = record.value("resource").toObject();
        if (!resource.isEmpty()) {
            const QString resourceKey = "resource:" + key;
            const QString resourceState = resource.value("state").toString("unverified");
            nodes.append({resourceKey, "vm", resource.value("id").toString(), resource.value("runtime").toString() + tr(" · host process only"),
                resourceState, 3, resourceState == "running" ? ResourceStatus::Running : ResourceStatus::Warning,
                resourceState != "running", resourceState == "running"});
            edges.append({key, resourceKey, tr("runtime association"), false});
            auto resourceRecord = record;
            resourceRecord.insert("process", resource.value("process"));
            resourceRecord.insert("metrics", resource.value("metrics"));
            resourceRecord.insert("runtime", resource.value("runtime"));
            resourceRecord.insert("state", resourceState);
            m_records.insert(resourceKey, resourceRecord);
        }
        const QString container = record.value("container").toString();
        if (!container.isEmpty()) {
            const QString containerKey = "container:" + key;
            nodes.append({containerKey, "container", container.left(24), tr("Docker association"), tr("Associated"), 3, ResourceStatus::Neutral, false, false});
            edges.append({key, containerKey, tr("runtime association"), false});
            auto containerRecord = record;
            containerRecord.insert("metrics", record.value("container_metrics").toObject());
            containerRecord.insert("runtime", "docker container");
            containerRecord.insert("process", QJsonObject{});
            containerRecord.insert("parent_pid", QJsonValue());
            m_records.insert(containerKey, containerRecord);
        }
    }
    m_system->blockSignals(false);
    if (!system) {
        for (auto iterator = m_records.cbegin(); iterator != m_records.cend(); ++iterator) {
            const QString parent = iterator.value().value("parent_id").toString();
            if (!iterator.key().startsWith("container:") && !iterator.key().startsWith("resource:") && m_records.contains(parent))
                edges.append({parent, iterator.key(), tr("managed child"), false});
        }
        m_canvas->setGraph(nodes, edges, nodes.isEmpty() ? tr("No matching managed executions. Run a workflow with this CLI to populate recent activity.") : QString());
        inspect(m_selected);
    }
    m_more->setEnabled(matching > m_visibleLimit && m_visibleLimit < (system ? 4096 : 64));
    QStringList notices;
    if (m_snapshot.value("truncated").toBool())
        notices << tr("Snapshot limit reached; additional resources may exist");
    for (const auto &notice : m_snapshot.value("notices").toArray())
        notices << notice.toString();
    m_status->setText(tr("%1 of %2 matching %3 · updated %4%5")
        .arg(qMin(matching, m_visibleLimit)).arg(matching).arg(system ? tr("processes") : tr("executions"))
        .arg(m_snapshot.value("captured_at").toString(tr("not yet")))
        .arg(notices.isEmpty() ? QString() : " · " + notices.join(" · ")));
    if (system)
        inspectSystem();
}

void ExecutionExplorer::inspect(const QString &identity)
{
    if (m_selected != identity) {
        m_chart->clear();
        m_chartSample.clear();
    }
    const bool selectionChanged = m_selected != identity;
    m_selected = identity;
    if (selectionChanged)
        QTimer::singleShot(0, this, &ExecutionExplorer::refresh);
    if (!m_records.contains(identity)) {
        m_chart->clear();
        m_chartSample.clear();
        m_title->setText(tr("Select an execution"));
        m_details->setText(tr("Select a process or associated container to inspect its lifecycle. Missing metrics are shown as unavailable."));
        m_events->clear();
        return;
    }
    const auto record = m_records.value(identity);
    const auto process = record.value("process").toObject();
    const auto metrics = record.value("metrics").toObject();
    QJsonValue duration = record.value("duration_ms");
    if (record.value("state").toString() == "running") {
        const auto started = QDateTime::fromString(record.value("started_at").toString(), Qt::ISODateWithMs);
        const auto captured = QDateTime::fromString(m_snapshot.value("captured_at").toString(), Qt::ISODateWithMs);
        if (started.isValid() && captured.isValid())
            duration = double(qMax<qint64>(0, started.msecsTo(captured)));
    }
    m_title->setText(record.value("workflow").toString(tr("Standalone execution")) + " · " + record.value("state").toString());
    m_details->setText(tr("Execution %1\nTask %2 · runtime %3 · machine %4\nPID %5 · parent PID %6\nMemory %7 · CPU %8\nDuration %9\nContainer %10\nOperation log %11")
        .arg(record.value("id").toString()).arg(record.value("task").toString(tr("None")))
        .arg(record.value("runtime").toString()).arg(record.value("machine").toString())
        .arg(process.value("pid").toInt() > 0 ? QString::number(process.value("pid").toInt()) : tr("Unavailable"))
        .arg(record.value("parent_pid").toInt() > 0 ? QString::number(record.value("parent_pid").toInt()) : tr("Unavailable"))
        .arg(metricText(metrics.value("memory_bytes"), "bytes")).arg(metricText(metrics.value("cpu_percent"), "%"))
        .arg(metricText(duration, "ms"))
        .arg(record.value("container").toString(tr("None"))).arg(record.value("log").toString(tr("Unavailable; Dockpipe logs remain in Activity"))));
    const QString captured = m_snapshot.value("captured_at").toString();
    if (m_chartSample != captured) {
        const auto percent = metrics.value("cpu_percent");
        m_chart->append(percent.isDouble() ? std::optional<double>(percent.toDouble()) : std::nullopt);
        m_chartSample = captured;
    }
    QStringList events{record.value("started_at").toString() + tr("  Process registered")};
    if (record.contains("finished_at"))
        events << record.value("finished_at").toString() + "  " + record.value("state").toString()
            + tr(" · exit %1").arg(metricText(record.value("exit_code"), QString()));
    if (!record.value("reason").toString().isEmpty())
        events << record.value("reason").toString();
    if (!record.value("log_notice").toString().isEmpty())
        events << "" << record.value("log_notice").toString();
    for (const auto &value : record.value("operation_events").toArray()) {
        const auto event = value.toObject();
        events << event.value("ts").toString() + "  " + event.value("unit").toString()
            + " · " + event.value("status").toString();
    }
    m_events->setPlainText(events.join('\n'));
}
