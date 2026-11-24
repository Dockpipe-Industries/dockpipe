#include "ExecutionExplorer.h"
#include "MonitoringMachineSelector.h"
#include "SystemResourceOverview.h"
#include "SystemProcessInspector.h"
#include "WorkflowRunInspector.h"
#include "WorkflowRunStateDelegate.h"
#include "ExecutionMetricChart.h"
#include <cmath>
#include <algorithm>
#include "CanvasIcons.h"
#include "DockpipeChoices.h"
#include "ResourceCanvas.h"
#include "NativeHostConnection.h"
#include "NativeHostSnapshot.h"

#include <QCheckBox>
#include <QButtonGroup>
#include <QFrame>
#include <QMenu>
#include <QAction>
#include <QSettings>
#include <QSplitter>
#include <QResizeEvent>
#include <QSignalBlocker>
#include <QScrollArea>
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

ExecutionExplorer::ExecutionExplorer(QWidget *parent, Purpose purpose) : QWidget(parent), m_command(this)
{
    setObjectName("executionExplorerPage");
    setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *heading = new QVBoxLayout;
    heading->setContentsMargins(0, 0, 0, 0);
    heading->setSpacing(4);
    m_pageTitle = new QLabel(tr("Running workflows"));
    m_pageTitle->setObjectName("observabilityPageTitle");
    heading->addWidget(m_pageTitle);
    m_pageDescription = new QLabel;
    m_pageDescription->setObjectName("observabilityPageDescription");
    m_pageDescription->setWordWrap(true);
    heading->addWidget(m_pageDescription);
    auto *header = new QHBoxLayout;
    header->setContentsMargins(24, 16, 24, 8);
    header->addLayout(heading, 1);
    auto *monitoringActions = new QHBoxLayout;
    monitoringActions->setSpacing(8);
    header->addLayout(monitoringActions);
    layout->addLayout(header);
    m_workflowControls = new QWidget;
    auto *toolbar = new QHBoxLayout(m_workflowControls);
    toolbar->setContentsMargins(24, 12, 24, 4);
    m_mode = new QComboBox(this);
    m_mode->setObjectName("executionMode");
    m_mode->addItems({tr("Dockpipe Managed"), tr("System")});
    m_mode->hide();
    m_machineBar = new QWidget;
    m_machineBar->setObjectName("systemMachineBar");
    auto *machineLayout = new QHBoxLayout(m_machineBar);
    machineLayout->setContentsMargins(24, 12, 24, 4);
    machineLayout->addWidget(new QLabel(tr("Machine")));
    m_machine = new MonitoringMachineSelector;
    machineLayout->addWidget(m_machine);
    machineLayout->addStretch();
    m_monitoringCadence = new QLabel(tr("Automatic updates · every 4 seconds"));
    m_monitoringCadence->setObjectName("systemMonitoringCadence");
    machineLayout->addWidget(m_monitoringCadence);
    layout->insertWidget(0, m_machineBar);
    m_global = new QCheckBox(tr("Standalone runs"), this);
    m_global->setObjectName("executionStandalone");
    m_global->setToolTip(tr("Show operations recorded without a project."));
    m_global->hide();
    m_runScope = new QLabel;
    m_runScope->setObjectName("workflowRunScope");
    m_runScope->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Preferred);
    toolbar->addWidget(m_runScope);
    m_search = new QLineEdit;
    m_search->setObjectName("executionSearch");
    m_search->setPlaceholderText(tr("Search workflows or tasks…"));
    m_search->setClearButtonEnabled(true);
    m_search->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Search), QLineEdit::LeadingPosition);
    m_workflowSearchHost = new QWidget;
    auto *searchLayout = new QHBoxLayout(m_workflowSearchHost);
    searchLayout->setContentsMargins(0, 0, 0, 0);
    searchLayout->addWidget(m_search);
    toolbar->addWidget(m_workflowSearchHost, 1);
    m_resources = new QCheckBox(tr("Resource metrics"), this);
    m_resources->setObjectName("executionResources");
    m_resources->setChecked(true);
    m_resources->hide();
    auto *options = new QPushButton(tr("Options"));
    options->setObjectName("workflowRunOptions");
    options->setSizePolicy(QSizePolicy::Maximum, QSizePolicy::Fixed);
    options->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Settings));
    auto *menu = new QMenu(options);
    const auto addOption = [menu](const QString &title, QCheckBox *value) {
        auto *action = menu->addAction(title);
        action->setCheckable(true);
        action->setChecked(value->isChecked());
        action->setToolTip(value->toolTip());
        QObject::connect(action, &QAction::toggled, value, &QCheckBox::setChecked);
        QObject::connect(value, &QCheckBox::toggled, action, &QAction::setChecked);
    };
    addOption(tr("Standalone runs"), m_global);
    addOption(tr("Resource metrics"), m_resources);
    options->setMenu(menu);
    toolbar->addWidget(options);
    m_refresh = new QPushButton(tr("Refresh"));
    m_refresh->setObjectName("executionRefresh");
    m_refresh->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Refresh));
    monitoringActions->addWidget(m_refresh);
    m_pause = new QPushButton(tr("Pause updates"));
    m_pause->setObjectName("executionPause");
    m_pause->setCheckable(true);
    monitoringActions->addWidget(m_pause);
    m_more = new QPushButton(tr("Show more"));
    m_more->setObjectName("executionMore");
    monitoringActions->addWidget(m_more);
    auto *viewSwitcher = new QFrame;
    viewSwitcher->setObjectName("observabilityViewSwitcher");
    viewSwitcher->setSizePolicy(QSizePolicy::Maximum, QSizePolicy::Fixed);
    auto *viewLayout = new QHBoxLayout(viewSwitcher);
    viewLayout->setContentsMargins(3, 3, 3, 3);
    viewLayout->setSpacing(2);
    auto *viewButtons = new QButtonGroup(this);
    m_canvasViewButton = new QPushButton(tr("Canvas"));
    m_canvasViewButton->setObjectName("executionCanvasView");
    m_canvasViewButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Branch));
    m_listViewButton = new QPushButton(tr("List"));
    m_listViewButton->setObjectName("executionListView");
    m_listViewButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::List));
    for (auto *button : {m_canvasViewButton, m_listViewButton}) {
        button->setCheckable(true);
        viewButtons->addButton(button);
        viewLayout->addWidget(button);
    }
    toolbar->addWidget(viewSwitcher);
    connect(m_canvasViewButton, &QPushButton::clicked, this, [this]() { setListView(false); });
    connect(m_listViewButton, &QPushButton::clicked, this, [this]() { setListView(true); });
    layout->addWidget(m_workflowControls);
    m_overview = new SystemResourceOverview;
    m_overview->hide();
    connect(m_machine, qOverload<int>(&QComboBox::currentIndexChanged), this, &ExecutionExplorer::changeMachine);
    connect(m_mode, qOverload<int>(&QComboBox::currentIndexChanged), this, [this](int mode) {
        m_overview->setVisible(mode == 1);
    });

    m_runInspector = new WorkflowRunInspector;
    m_inspector = m_runInspector;
    auto *inspector = m_inspector;
    m_canvasInspectorHost = new QWidget;
    auto *canvasInspectorLayout = new QVBoxLayout(m_canvasInspectorHost);
    canvasInspectorLayout->setContentsMargins(0, 0, 0, 0);
    canvasInspectorLayout->addWidget(inspector);
    m_canvas = new ResourceCanvas("executionExplorer", tr("Workflow runs"),
        tr("Follow each workflow through its tasks and processes."), m_canvasInspectorHost, this, true);
    m_canvas->setObjectName("executionCanvas");
    m_canvas->setDetailsLabel(tr("Execution details & lifecycle"));

    m_views = new QStackedWidget;
    m_views->addWidget(m_canvas);
    m_views->addWidget(buildSystemPage());
    auto *runListContents = new QWidget;
    runListContents->setObjectName("workflowRunListContents");
    runListContents->setAttribute(Qt::WA_StyledBackground, true);
    auto *runListScroll = new QScrollArea;
    runListScroll->setFrameShape(QFrame::NoFrame);
    runListScroll->setWidgetResizable(true);
    runListScroll->setWidget(runListContents);
    m_runListPage = runListScroll;
    auto *runListLayout = new QVBoxLayout(runListContents);
    runListLayout->setContentsMargins(24, 12, 24, 16);
    m_runListSplitter = new QSplitter(Qt::Horizontal);
    m_runListSplitter->setObjectName("workflowRunListSplitter");
    m_runListSplitter->setChildrenCollapsible(false);
    m_runListSplitter->setHandleWidth(12);
    auto *runTableCard = new QFrame;
    runTableCard->setObjectName("workflowRunTableCard");
    auto *runTableLayout = new QVBoxLayout(runTableCard);
    runTableLayout->setContentsMargins(16, 16, 16, 12);
    runTableLayout->setSpacing(14);
    runTableCard->setMinimumHeight(360);
    auto *runHeader = new QHBoxLayout;
    m_runTableTitle = new QLabel;
    m_runTableTitle->setObjectName("workflowRunTableTitle");
    runHeader->addWidget(m_runTableTitle);
    runHeader->addStretch();
    m_runResults = new QLabel;
    m_runResults->setObjectName("workflowRunResults");
    runHeader->addWidget(m_runResults);
    runTableLayout->addLayout(runHeader);
    auto *runToolbar = new QHBoxLayout;
    runToolbar->setSpacing(12);
    m_runSearchHost = new QWidget;
    auto *runSearchLayout = new QHBoxLayout(m_runSearchHost);
    runSearchLayout->setContentsMargins(0, 0, 0, 0);
    runToolbar->addWidget(m_runSearchHost, 1);
    m_runStateFilter = new QComboBox;
    m_runStateFilter->setObjectName("workflowRunStateFilter");
    m_runStateFilter->setAccessibleName(tr("Filter run history by outcome"));
    m_runStateFilter->addItem(tr("All states"), "all");
    for (const QString &state : {QStringLiteral("succeeded"), QStringLiteral("failed"), QStringLiteral("cancelled"),
                                 QStringLiteral("terminated"), QStringLiteral("unverified")})
        m_runStateFilter->addItem(WorkflowRunStateDelegate::label(state), state);
    runToolbar->addWidget(m_runStateFilter);
    runTableLayout->addLayout(runToolbar);
    m_runs = new QTableWidget(0, 6);
    m_runs->setObjectName("workflowRunsTable");
    m_runs->setItemDelegateForColumn(2, new WorkflowRunStateDelegate(m_runs));
    m_runs->setHorizontalHeaderLabels({tr("Workflow"), tr("Task"), tr("State"), tr("Runtime"), tr("Started"), tr("Duration")});
    m_runs->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_runs->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);
    m_runs->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_runs->horizontalHeader()->setSortIndicatorShown(true);
    m_runs->horizontalHeader()->setSortIndicator(4, Qt::DescendingOrder);
    m_runs->horizontalHeader()->setSectionsClickable(true);
    m_runs->verticalHeader()->hide();
    m_runs->verticalHeader()->setDefaultSectionSize(48);
    m_runs->setShowGrid(false);
    m_runs->setAlternatingRowColors(true);
    m_runs->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_runs->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_runs->setSelectionMode(QAbstractItemView::SingleSelection);
    m_runs->setMinimumHeight(200);
    runTableLayout->addWidget(m_runs, 1);
    m_emptyRuns = new QLabel;
    m_emptyRuns->setObjectName("workflowRunsEmpty");
    m_emptyRuns->setAlignment(Qt::AlignCenter);
    m_emptyRuns->setWordWrap(true);
    runTableLayout->addWidget(m_emptyRuns, 1);
    m_runListSplitter->addWidget(runTableCard);
    runListLayout->addWidget(m_runListSplitter);
    m_views->addWidget(m_runListPage);
    m_runSummary = buildRunSummary();
    layout->addWidget(m_runSummary);
    connect(m_runStateFilter, qOverload<int>(&QComboBox::currentIndexChanged), this, [this]() { render(); });
    connect(m_runs->horizontalHeader(), &QHeaderView::sortIndicatorChanged, this, [this]() {
        if (m_mode->currentIndex() == 0)
            render();
    });
    connect(m_runs, &QTableWidget::itemSelectionChanged, this, [this]() {
        if (auto *item = m_runs->item(m_runs->currentRow(), 0))
            inspect(item->data(Qt::UserRole).toString());
    });
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
    connect(m_refresh, &QPushButton::clicked, this, [this]() { refreshWithAuthentication(true); });
    connect(&m_command, &RemoteCommand::busyChanged, m_refresh, [this](bool busy) {
        m_refresh->setEnabled(!busy);
    });
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, &ExecutionExplorer::inspect);
    connect(m_search, &QLineEdit::textChanged, this, [this]() { render(); });
    connect(m_resources, &QCheckBox::toggled, this, [this]() {
        ++m_generation;
        render();
        refresh();
    });
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
        m_monitoringCadence->setText(paused ? tr("Updates paused") : tr("Automatic updates · every 4 seconds"));
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
        m_runInspector->clear();
        m_visibleLimit = mode == 0 ? 32 : 250;
        m_selected.clear();
        m_records.clear();
        m_snapshot = {};
        updatePresentation();
        render();
        refresh();
    });
    connect(m_more, &QPushButton::clicked, this, [this]() {
        m_visibleLimit = qMin(visibleRecordCeiling(), m_visibleLimit * 2);
        render();
    });
    connect(m_system, &QTableWidget::itemSelectionChanged, this, &ExecutionExplorer::inspectSystem);
    if (purpose == Purpose::System)
        m_mode->setCurrentIndex(1);
    const bool list = QSettings().value("workflowRuns/viewMode", "list").toString() == "list";
    setListView(list);
}

void ExecutionExplorer::setHistory(bool history)
{
    if (m_history == history)
        return;
    m_history = history;
    m_selected.clear();
    m_visibleLimit = 32;
    const bool runningList = QSettings().value("workflowRuns/viewMode", "list").toString() == "list";
    setListView(history || runningList);
}

void ExecutionExplorer::setListView(bool list)
{
    m_listView = m_history || list;
    m_canvasViewButton->setChecked(!m_listView);
    m_listViewButton->setChecked(m_listView);
    m_visibleLimit = qMin(m_visibleLimit, visibleRecordCeiling());
    updatePresentation();
    render();
    if (!m_history)
        QSettings().setValue("workflowRuns/viewMode", list ? "list" : "canvas");
}

QWidget *ExecutionExplorer::buildRunSummary()
{
    auto *summary = new QWidget;
    auto *layout = new QHBoxLayout(summary);
    layout->setContentsMargins(24, 12, 24, 8);
    layout->setSpacing(12);
    for (int index = 0; index < 4; ++index) {
        auto *card = new QFrame;
        card->setObjectName("workflowSummaryCard");
        m_runSummaryCards[index] = card;
        auto *cardLayout = new QVBoxLayout(card);
        cardLayout->setContentsMargins(18, 14, 18, 14);
        cardLayout->setSpacing(4);
        m_runSummaryTitles[index] = new QLabel;
        m_runSummaryTitles[index]->setObjectName("workflowSummaryTitle");
        m_runSummaryValues[index] = new QLabel;
        m_runSummaryValues[index]->setObjectName("workflowSummaryValue" + QString::number(index));
        m_runSummaryValues[index]->setProperty("workflowSummaryValue", true);
        m_runSummaryCaptions[index] = new QLabel;
        m_runSummaryCaptions[index]->setObjectName("workflowSummaryCaption");
        m_runSummaryCaptions[index]->setWordWrap(true);
        cardLayout->addWidget(m_runSummaryTitles[index]);
        cardLayout->addWidget(m_runSummaryValues[index]);
        cardLayout->addWidget(m_runSummaryCaptions[index]);
        layout->addWidget(card, 1);
    }
    return summary;
}

void ExecutionExplorer::updateRunSummary()
{
    int count = 0;
    int succeeded = 0;
    int failed = 0;
    QSet<QString> workflows;
    QSet<QString> runtimes;
    for (const auto &value : m_snapshot.value("executions").toArray()) {
        const auto record = value.toObject();
        const QString state = record.value("state").toString();
        if ((state == "running") == m_history)
            continue;
        ++count;
        if (state == "succeeded")
            ++succeeded;
        if (state == "failed")
            ++failed;
        const QString workflow = record.value("workflow").toString();
        if (!workflow.isEmpty())
            workflows.insert(workflow);
        const QString runtime = record.value("runtime").toString();
        if (!runtime.isEmpty())
            runtimes.insert(runtime);
    }
    QStringList titles{tr("Active tasks"), tr("Workflows"), tr("Runtimes")};
    QStringList captions{tr("In the latest snapshot"), tr("Distinct recorded names"), tr("Execution backends in use")};
    QList<int> values{count, int(workflows.size()), int(runtimes.size())};
    if (m_history) {
        titles = {tr("Recorded tasks"), tr("Succeeded"), tr("Failed"), tr("Other states")};
        captions = {tr("In retained history"), tr("Reported success"), tr("Reported failure"), tr("Includes cancelled and unverified")};
        values = {count, succeeded, failed, count - succeeded - failed};
    }
    for (int index = 0; index < 4; ++index) {
        m_runSummaryCards[index]->setVisible(index < titles.size());
        if (index >= titles.size())
            continue;
        m_runSummaryTitles[index]->setText(titles[index]);
        m_runSummaryValues[index]->setText(m_snapshot.value("executions").isArray()
            ? QString::number(values[index]) : QStringLiteral("—"));
        m_runSummaryCaptions[index]->setText(captions[index]);
    }
    m_runScope->setText(m_global->isChecked() || m_workdir.isEmpty() ? tr("Standalone runs") : tr("Workspace runs"));
}

void ExecutionExplorer::updateRunLayout()
{
    const Qt::Orientation orientation = width() < 1180 ? Qt::Vertical : Qt::Horizontal;
    const bool changed = m_runListSplitter->orientation() != orientation;
    if (changed)
        m_runListSplitter->setOrientation(orientation);
    const QWidget *table = m_runListSplitter->widget(0);
    const QSize tableMinimum = table->minimumSizeHint().expandedTo(table->minimumSize());
    const QSize inspectorMinimum = m_inspector->minimumSizeHint().expandedTo(m_inspector->minimumSize());
    m_runListSplitter->setMinimumHeight(orientation == Qt::Vertical
        ? tableMinimum.height() + inspectorMinimum.height() + m_runListSplitter->handleWidth()
        : qMax(tableMinimum.height(), inspectorMinimum.height()));
    if (changed) {
        const QList<int> sizes = orientation == Qt::Horizontal ? QList<int>{800, 420}
            : QList<int>{tableMinimum.height(), inspectorMinimum.height()};
        m_runListSplitter->setSizes(sizes);
    }
}

QWidget *ExecutionExplorer::buildSystemPage()
{
    auto *contents = new QWidget;
    contents->setObjectName("systemPageContents");
    contents->setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(contents);
    layout->setContentsMargins(24, 12, 24, 24);
    layout->setSpacing(20);
    layout->addWidget(m_overview);
    m_systemSplitter = new QSplitter(Qt::Horizontal);
    m_systemSplitter->setObjectName("systemProcessSplitter");
    m_systemSplitter->setChildrenCollapsible(false);
    m_systemSplitter->setHandleWidth(12);
    auto *card = new QFrame;
    card->setObjectName("systemProcessCard");
    card->setMinimumHeight(360);
    auto *cardLayout = new QVBoxLayout(card);
    cardLayout->setContentsMargins(16, 16, 16, 12);
    cardLayout->setSpacing(14);
    auto *header = new QHBoxLayout;
    auto *title = new QLabel(tr("Processes"));
    title->setObjectName("systemProcessSectionTitle");
    header->addWidget(title);
    header->addStretch();
    m_processCount = new QLabel(tr("No processes loaded"));
    m_processCount->setObjectName("systemProcessCount");
    header->addWidget(m_processCount);
    cardLayout->addLayout(header);
    m_systemSearchHost = new QWidget;
    auto *searchLayout = new QHBoxLayout(m_systemSearchHost);
    searchLayout->setContentsMargins(0, 0, 0, 0);
    auto *toolbar = new QHBoxLayout;
    toolbar->setSpacing(12);
    toolbar->addWidget(m_systemSearchHost, 1);
    m_processFilter = new QComboBox;
    m_processFilter->setObjectName("systemProcessFilter");
    m_processFilter->setAccessibleName(tr("Filter processes by state"));
    m_processFilter->addItem(tr("All states"), "all");
    m_processFilter->addItem(tr("Running"), "R");
    m_processFilter->addItem(tr("Sleeping"), "S");
    m_processFilter->addItem(tr("Stopped"), "T");
    toolbar->addWidget(m_processFilter);
    cardLayout->addLayout(toolbar);
    m_system = new QTableWidget(0, 6);
    m_system->setObjectName("systemProcesses");
    m_system->setHorizontalHeaderLabels({tr("Process"), tr("PID"), tr("Parent PID"), tr("State"), tr("Memory"), tr("CPU")});
    m_system->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_system->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);
    m_system->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_system->horizontalHeader()->setSectionsClickable(true);
    m_system->horizontalHeader()->setSortIndicatorShown(true);
    m_system->horizontalHeader()->setSortIndicator(0, Qt::AscendingOrder);
    m_system->horizontalHeader()->setMinimumSectionSize(72);
    m_system->verticalHeader()->hide();
    m_system->verticalHeader()->setDefaultSectionSize(48);
    m_system->setShowGrid(false);
    m_system->setAlternatingRowColors(true);
    m_system->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_system->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_system->setSelectionMode(QAbstractItemView::SingleSelection);
    m_system->setMinimumHeight(200);
    for (int column : {1, 2, 4, 5})
        m_system->horizontalHeaderItem(column)->setTextAlignment(Qt::AlignRight | Qt::AlignVCenter);
    cardLayout->addWidget(m_system, 1);
    m_emptyProcesses = new QLabel;
    m_emptyProcesses->setObjectName("systemProcessesEmpty");
    m_emptyProcesses->setAlignment(Qt::AlignCenter);
    m_emptyProcesses->setWordWrap(true);
    cardLayout->addWidget(m_emptyProcesses, 1);
    m_systemSplitter->addWidget(card);
    m_systemInspector = new SystemProcessInspector;
    m_systemSplitter->addWidget(m_systemInspector);
    m_systemSplitter->setSizes({800, 420});
    m_systemSplitter->setStretchFactor(0, 3);
    layout->addWidget(m_systemSplitter, 1);
    connect(m_system->horizontalHeader(), &QHeaderView::sortIndicatorChanged, this, [this]() {
        if (m_mode->currentIndex() == 1)
            render();
    });
    connect(m_processFilter, qOverload<int>(&QComboBox::currentIndexChanged), this, [this]() { render(); });
    auto *scroll = new QScrollArea;
    scroll->setObjectName("systemObservabilityScroll");
    scroll->setFrameShape(QFrame::NoFrame);
    scroll->setWidgetResizable(true);
    scroll->setWidget(contents);
    return scroll;
}

void ExecutionExplorer::updateSystemLayout()
{
    const Qt::Orientation orientation = width() < 1180 ? Qt::Vertical : Qt::Horizontal;
    const bool changed = m_systemSplitter->orientation() != orientation;
    if (changed)
        m_systemSplitter->setOrientation(orientation);
    const QWidget *table = m_systemSplitter->widget(0);
    const QSize tableMinimum = table->minimumSizeHint().expandedTo(table->minimumSize());
    const QSize inspectorMinimum = m_systemInspector->minimumSizeHint().expandedTo(m_systemInspector->minimumSize());
    // QScrollArea must retain enough content height for both panes when stacked.
    const int height = orientation == Qt::Vertical
        ? tableMinimum.height() + inspectorMinimum.height() + m_systemSplitter->handleWidth()
        : qMax(tableMinimum.height(), inspectorMinimum.height());
    m_systemSplitter->setMinimumHeight(height);
    if (changed) {
        const QList<int> sizes = orientation == Qt::Horizontal
            ? QList<int>{800, 420} : QList<int>{tableMinimum.height(), inspectorMinimum.height()};
        m_systemSplitter->setSizes(sizes);
    }
}

void ExecutionExplorer::clearSystemProcesses()
{
    m_system->setRowCount(0);
    m_system->hide();
    m_emptyProcesses->setText(tr("No process data to display\nRefresh to load the selected machine's process list."));
    m_emptyProcesses->show();
    m_processCount->setText(tr("No processes loaded"));
    m_systemInspector->clear();
    updateSystemLayout();
    if (m_mode->currentIndex() == 1) {
        m_selected.clear();
        m_records.clear();
    }
}

int ExecutionExplorer::visibleRecordCeiling() const
{
    if (m_mode->currentIndex() == 1)
        return 4096; // The process snapshot contract.
    if (m_listView)
        return 256; // The retained execution snapshot contract.
    return 64; // Preserve the existing interactive graph budget.
}

void ExecutionExplorer::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    if (m_mode->currentIndex() == 1) {
        updateSystemLayout();
    } else if (m_listView) {
        updateRunLayout();
    }
}

void ExecutionExplorer::updatePresentation()
{
    const bool system = m_mode->currentIndex() == 1;
    if (system) {
        m_pageTitle->setText(tr("System"));
        m_pageDescription->setText(tr("Live resource usage and processes on the selected machine."));
    } else if (m_history) {
        m_pageTitle->setText(tr("Run history"));
        m_pageDescription->setText(tr("Recent recorded workflow runs, their outcomes, and lifecycle details."));
    } else {
        m_pageTitle->setText(tr("Running workflows"));
        m_pageDescription->setText(tr("Follow active workflow tasks and their progress."));
    }
    m_search->setPlaceholderText(system ? tr("Search by process name or PID…") : tr("Search workflows or tasks…"));
    m_machineBar->setVisible(system);
    m_workflowControls->setVisible(!system);
    QWidget *searchHost = m_workflowSearchHost;
    if (system)
        searchHost = m_systemSearchHost;
    else if (m_listView)
        searchHost = m_runSearchHost;
    searchHost->layout()->addWidget(m_search);
    m_search->show();
    m_workflowSearchHost->setVisible(!system && !m_listView);
    m_global->hide();
    m_resources->hide();
    m_runSummary->setVisible(!system && m_listView);
    m_runStateFilter->setVisible(m_history);
    m_runTableTitle->setText(m_history ? tr("Recorded tasks") : tr("Active tasks"));
    m_canvasViewButton->parentWidget()->setVisible(!system && !m_history);
    m_overview->setVisible(system);
    m_more->setText(system ? tr("Show more processes") : tr("Show more runs"));
    if (!system && m_listView) {
        m_runListSplitter->addWidget(m_inspector);
        m_inspector->setMinimumWidth(360);
        updateRunLayout();
        m_inspector->show();
        m_views->setCurrentWidget(m_runListPage);
    } else {
        m_canvasInspectorHost->layout()->addWidget(m_inspector);
        m_inspector->setMinimumWidth(0);
        m_inspector->setMinimumHeight(0);
        m_inspector->show();
        m_views->setCurrentIndex(system ? 1 : 0);
    }
    if (system)
        updateSystemLayout();
}

void ExecutionExplorer::inspectSystem()
{
    if (m_system->currentRow() < 0) {
        m_systemInspector->clear();
        m_selected.clear();
        updateSystemLayout();
        return;
    }
    const QString key = m_system->item(m_system->currentRow(), 0)->data(Qt::UserRole).toString();
    m_selected = key;
    const auto record = m_records.value(key);
    m_systemInspector->setProcess(key, record, metricText(record.value("memory_bytes"), "bytes"),
        metricText(record.value("cpu_percent"), "%"), m_snapshot.value("captured_at").toString());
    updateSystemLayout();
}

void ExecutionExplorer::setWorkdir(const QString &workdir)
{
    if (m_workdir == workdir)
        return;
    m_cpuCounters.clear();
    m_runInspector->clear();
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
    refreshWithAuthentication(false);
}

void ExecutionExplorer::refreshWithAuthentication(bool allowAuthentication)
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
    if (node == NativeHostConnection::selectionKey()) {
        if (!m_nativeHost || !m_nativeHost->available()) {
            clearMonitoringSnapshot(tr("Native host unavailable. Open this application through its authenticated host session."));
            return;
        }
        const auto plan = m_nativeHost->plan(NativeHostConnection::Operation::Snapshot);
        if (!plan.valid()) {
            clearMonitoringSnapshot(plan.error);
            return;
        }
        const quint64 generation = m_generation;
        const quint64 revision = m_nativeHost->revision();
        const QString expectedNode = m_nativeHost->status().node;
        m_command.runBounded(plan.program, plan.arguments,
            [this, generation, revision, expectedNode](bool ok, const QByteArray &output) {
                if (generation != m_generation || !m_active || m_pause->isChecked()
                    || !m_nativeHost || !m_nativeHost->available() || revision != m_nativeHost->revision())
                    return;
                QJsonObject snapshot;
                if (!ok || !NativeHostSnapshot::decode(output, expectedNode, "execution", &snapshot)
                    || !applySnapshot(snapshot)) {
                    clearMonitoringSnapshot(tr("Native host system monitoring unavailable. Refresh the authenticated host session."));
                    m_nativeHost->refresh();
                    return;
                }
                m_status->setText(m_nativeHost->label() + QStringLiteral(" · ") + m_status->text());
            }, 10000, 1024 * 1024);
        return;
    }
    QString program;
    if (!node.isEmpty()) {
        args = {"remote", "observe", "--node", node, "--auth",
                allowAuthentication ? "auto" : "never"};
        QString error;
        program = DockpipeChoices::operatorDockpipeBinary(&error);
        if (program.isEmpty()) {
            m_overview->clear();
            clearSystemProcesses();
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
                    clearSystemProcesses();
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
                if (m_mode->currentIndex() == 1)
                    clearSystemProcesses();
                m_status->setText(tr("Monitoring unavailable. Displayed data may be stale. Check the CLI version and machine permissions."));
            }
        }, !node.isEmpty() && allowAuthentication ? 150000 : 5000);
}

void ExecutionExplorer::setNodes(const QStringList &nodes)
{
    m_machine->setNodes(nodes);
}

void ExecutionExplorer::setNativeHostConnection(NativeHostConnection *connection)
{
    if (m_nativeHost)
        disconnect(m_nativeHost, nullptr, this, nullptr);
    m_nativeHost = connection;
    if (connection) {
        connect(connection, &NativeHostConnection::statusChanged, this, &ExecutionExplorer::nativeHostChanged);
        connect(connection, &QObject::destroyed, this, &ExecutionExplorer::nativeHostChanged);
    }
    m_machine->setNativeHost(connection ? connection->label() : tr("Native host — unavailable"), true);
    changeMachine();
}

void ExecutionExplorer::nativeHostChanged()
{
    m_machine->setNativeHost(m_nativeHost ? m_nativeHost->label() : tr("Native host — unavailable"), false);
    if (m_machine->currentData().toString() == NativeHostConnection::selectionKey())
        changeMachine();
}

void ExecutionExplorer::clearMonitoringSnapshot(const QString &status)
{
    m_selected.clear();
    m_snapshot = {};
    m_records.clear();
    m_cpuCounters.clear();
    m_sampleTime = 0;
    m_runInspector->clear();
    m_overview->clear();
    clearSystemProcesses();
    m_canvas->setGraph({}, {}, status);
    m_status->setText(status);
}

void ExecutionExplorer::changeMachine()
{
    ++m_generation;
    m_command.cancel();
    clearMonitoringSnapshot(tr("Waiting for the selected machine…"));
    const bool remote = !m_machine->currentData().toString().isEmpty();
    m_mode->setEnabled(!remote);
    m_global->setEnabled(!remote);
    m_resources->setEnabled(!remote);
    if (remote)
        m_mode->setCurrentIndex(1);
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
    if (!std::isfinite(number) || (!unit.isEmpty() && number < 0))
        return tr("Unavailable");
    if (unit == "bytes") {
        if (number >= 1024 * 1024 * 1024)
            return QString::number(number / (1024 * 1024 * 1024), 'f', 1) + " GiB";
        return QString::number(number / (1024 * 1024), 'f', 1) + " MiB";
    }
    if (unit == "ms")
        return QString::number(number / 1000, 'f', 1) + tr(" s");
    return QString::number(number, 'g', 6) + (unit.isEmpty() ? QString() : " " + unit);
}

QJsonValue ExecutionExplorer::runDuration(const QJsonObject &record) const
{
    if (record.value("state").toString() == "running") {
        const auto started = QDateTime::fromString(record.value("started_at").toString(), Qt::ISODateWithMs);
        const auto captured = QDateTime::fromString(m_snapshot.value("captured_at").toString(), Qt::ISODateWithMs);
        if (started.isValid() && captured.isValid())
            return double(qMax<qint64>(0, started.msecsTo(captured)));
    }
    return record.value("duration_ms");
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
    if (!system) {
        const QStringList keys{"workflow", "task", "state", "runtime", "started_at", "duration_ms"};
        const int column = m_runs->horizontalHeader()->sortIndicatorSection();
        const QString key = keys.value(column, "started_at");
        const bool ascending = m_runs->horizontalHeader()->sortIndicatorOrder() == Qt::AscendingOrder;
        std::stable_sort(records.begin(), records.end(), [this, key, column, ascending](const QJsonObject &left, const QJsonObject &right) {
            if (column == 5) {
                const auto leftDuration = runDuration(left);
                const auto rightDuration = runDuration(right);
                const bool leftKnown = leftDuration.isDouble() && std::isfinite(leftDuration.toDouble()) && leftDuration.toDouble() >= 0;
                const bool rightKnown = rightDuration.isDouble() && std::isfinite(rightDuration.toDouble()) && rightDuration.toDouble() >= 0;
                if (leftKnown != rightKnown)
                    return leftKnown;
                if (leftKnown && leftDuration.toDouble() != rightDuration.toDouble())
                    return ascending ? leftDuration.toDouble() < rightDuration.toDouble()
                                     : leftDuration.toDouble() > rightDuration.toDouble();
            } else {
                const int comparison = QString::compare(left.value(key).toString(), right.value(key).toString(), Qt::CaseInsensitive);
                if (comparison != 0)
                    return ascending ? comparison < 0 : comparison > 0;
            }
            return left.value("id").toString() < right.value("id").toString();
        });
        updateRunSummary();
    }
    const QString query = m_search->text().trimmed();
    QVector<ResourceNode> nodes;
    QVector<ResourceEdge> edges;
    QSet<QString> groups;
    int matching = 0;
    const QSignalBlocker runSelectionBlocker(m_runs);
    m_runs->setRowCount(0);
    m_system->blockSignals(true);
    m_system->setRowCount(0);
    for (const auto &record : records) {
        if (!system) {
            const bool running = record.value("state").toString() == "running";
            if (running == m_history)
                continue;
            const QString outcome = m_runStateFilter->currentData().toString();
            if (m_history && outcome != "all" && record.value("state").toString() != outcome)
                continue;
        }
        const QString title = system ? record.value("name").toString() : record.value("workflow").toString(tr("Standalone execution"));
        QString searchable = title + " " + record.value("task").toString() + " " + record.value("id").toString();
        if (system) {
            const QString state = SystemProcessInspector::stateCode(record.value("state").toString());
            const QString filter = m_processFilter->currentData().toString();
            const bool matches = filter == "all" || state == filter || (filter == "T" && state == "t");
            if (!matches)
                continue;
            searchable += " " + QString::number(record.value("pid").toInt())
                + " " + QString::number(record.value("parent_pid").toInt())
                + " " + SystemProcessInspector::stateLabel(state);
        }
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
                SystemProcessInspector::stateLabel(record.value("state").toString()),
                metricText(record.value("memory_bytes"), "bytes"), metricText(record.value("cpu_percent"), "%")};
            for (int column = 0; column < cells.size(); ++column)
                m_system->setItem(row, column, new QTableWidgetItem(cells[column]));
            m_system->item(row, 0)->setData(Qt::UserRole, key);
            m_system->item(row, 0)->setToolTip(title);
            m_system->item(row, 3)->setToolTip(record.value("state").toString());
            for (int column : {1, 2, 4, 5})
                m_system->item(row, column)->setTextAlignment(Qt::AlignRight | Qt::AlignVCenter);
            if (key == m_selected)
                m_system->selectRow(row);
            continue;
        }
        const int row = m_runs->rowCount();
        m_runs->insertRow(row);
        const auto started = QDateTime::fromString(record.value("started_at").toString(), Qt::ISODateWithMs);
        const QJsonValue duration = runDuration(record);
        const QString state = record.value("state").toString();
        const QString stateLabel = WorkflowRunStateDelegate::label(state);
        const QStringList cells{title, record.value("task").toString(tr("Standalone")),
            stateLabel, record.value("runtime").toString(),
            started.isValid() ? started.toLocalTime().toString("MMM d, HH:mm:ss") : tr("Unavailable"), metricText(duration, "ms")};
        for (int column = 0; column < cells.size(); ++column)
            m_runs->setItem(row, column, new QTableWidgetItem(cells[column]));
        m_runs->item(row, 0)->setData(Qt::UserRole, key);
        m_runs->item(row, 0)->setToolTip(title + "\n" + key);
        m_runs->item(row, 2)->setData(Qt::UserRole + 1, state);
        m_runs->item(row, 2)->setToolTip(record.value("reason").toString(state));
        m_runs->item(row, 5)->setTextAlignment(Qt::AlignRight | Qt::AlignVCenter);
        if (key == m_selected)
            m_runs->selectRow(row);
        // List mode can show the full retained snapshot without expanding the hidden graph.
        if (matching > 64)
            continue;
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
    if (system) {
        const int count = m_system->rowCount();
        m_processCount->setText(tr("%1 of %2 processes").arg(count).arg(records.size()));
        m_system->setVisible(count > 0);
        m_emptyProcesses->setVisible(count == 0);
        m_emptyProcesses->setText(records.isEmpty()
            ? tr("No process data to display\nRefresh to load the selected machine's process list.")
            : tr("No matching processes\nTry another search or state filter."));
    }
    m_runResults->setText(tr("%1 of %2 matching tasks").arg(m_runs->rowCount()).arg(matching));
    m_runs->setVisible(m_runs->rowCount() > 0);
    m_emptyRuns->setVisible(m_runs->rowCount() == 0);
    if (!query.isEmpty() || (m_history && m_runStateFilter->currentData().toString() != "all"))
        m_emptyRuns->setText(tr("No matching workflow tasks\nTry another search or outcome filter."));
    else if (m_history)
        m_emptyRuns->setText(tr("No recorded runs yet\nCompleted workflow runs will appear here."));
    else
        m_emptyRuns->setText(tr("No workflows running\nStart a workflow from the editor to follow it here."));
    if (!system) {
        for (auto iterator = m_records.cbegin(); iterator != m_records.cend(); ++iterator) {
            const QString parent = iterator.value().value("parent_id").toString();
            if (!iterator.key().startsWith("container:") && !iterator.key().startsWith("resource:") && m_records.contains(parent))
                edges.append({parent, iterator.key(), tr("managed child"), false});
        }
        m_canvas->setGraph(nodes, edges, nodes.isEmpty() ? tr("No matching workflow runs in this view.") : QString());
        inspect(m_selected);
    }
    m_more->setVisible(matching > m_visibleLimit);
    m_more->setEnabled(matching > m_visibleLimit && m_visibleLimit < visibleRecordCeiling());
    QStringList notices;
    if (m_snapshot.value("truncated").toBool())
        notices << tr("Snapshot limit reached; additional resources may exist");
    for (const auto &notice : m_snapshot.value("notices").toArray())
        notices << notice.toString();
    m_status->setText(tr("%1 of %2 matching %3 · updated %4%5")
        .arg(qMin(matching, m_visibleLimit)).arg(matching).arg(system ? tr("processes") : tr("workflow tasks"))
        .arg(m_snapshot.value("captured_at").toString(tr("not yet")))
        .arg(notices.isEmpty() ? QString() : " · " + notices.join(" · ")));
    if (system)
        inspectSystem();
}

void ExecutionExplorer::inspect(const QString &identity)
{
    const bool changed = m_selected != identity;
    m_selected = identity;
    if (changed)
        QTimer::singleShot(0, this, &ExecutionExplorer::refresh);
    if (!m_records.contains(identity)) {
        m_runInspector->clear();
        if (m_listView)
            updateRunLayout();
        return;
    }
    const auto record = m_records.value(identity);
    const auto metrics = record.value("metrics").toObject();
    const QJsonValue duration = runDuration(record);
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
    m_runInspector->setRun(identity, record, metricText(duration, "ms"), metricText(metrics.value("memory_bytes"), "bytes"),
        metricText(metrics.value("cpu_percent"), "%"), events.join('\n'), m_snapshot.value("captured_at").toString(),
        m_resources->isChecked(), m_history);
    if (m_listView)
        updateRunLayout();
}
