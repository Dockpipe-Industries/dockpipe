#include "DashboardWidgetFactory.h"
#include "DashboardFilesWidget.h"
#include <QDesktopServices>
#include <QMessageBox>
#include <QUrl>

#include "BrowserWidget.h"
#include "RemoteBrowserWidget.h"
#include "RemoteAccessMonitor.h"
#include <QUuid>
#include "SystemResourceOverview.h"

#include <QComboBox>
#include <QCoreApplication>
#include <QDynamicPropertyChangeEvent>
#include <QAbstractItemView>
#include <QDir>
#include <QFileInfo>
#include <QFileSystemModel>
#include <QHeaderView>
#include <QFrame>
#include <QLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPushButton>
#include <QProgressBar>
#include <QSignalBlocker>
#include <QStyle>
#include <QTableWidget>
#include <QTreeView>
#include <QVBoxLayout>
#include <algorithm>

namespace {
// Presentation belongs to the canvas host; the same widget keeps its normal
// page and tiling controls when it leaves that host.
class CanvasDashboardView final : public QWidget {
public:
    explicit CanvasDashboardView(QWidget *parent) : QWidget(parent)
    {
        setObjectName("canvasDashboardView");
    }

protected:
    bool event(QEvent *event) override
    {
        if (event->type() == QEvent::DynamicPropertyChange) {
            const auto *change = static_cast<QDynamicPropertyChangeEvent *>(event);
            if (change->propertyName() == "dashboardCanvasPresentation") {
                const bool canvas = property("dashboardCanvasPresentation").toBool();
                for (auto *card : findChildren<QFrame *>("systemMetricCard")) {
                    auto *cardLayout = card->layout();
                    if (!cardLayout)
                        continue;
                    if (canvas) {
                        cardLayout->setProperty("canvasOriginalMargins", QVariant::fromValue(cardLayout->contentsMargins()));
                        cardLayout->setProperty("canvasOriginalSpacing", cardLayout->spacing());
                        cardLayout->setContentsMargins(8, 6, 8, 6);
                        cardLayout->setSpacing(4);
                    } else if (cardLayout->property("canvasOriginalMargins").isValid()) {
                        cardLayout->setContentsMargins(cardLayout->property("canvasOriginalMargins").value<QMargins>());
                        cardLayout->setSpacing(cardLayout->property("canvasOriginalSpacing").toInt());
                    }
                }
                const auto progressBars = findChildren<QProgressBar *>();
                if (canvas) {
                    for (auto *progressBar : progressBars) {
                        if (!progressBar->property("canvasOriginalMaximumHeight").isValid())
                            progressBar->setProperty("canvasOriginalMaximumHeight", progressBar->maximumHeight());
                    }
                }
                // The common theme owns canvas rules. Refresh each affected
                // widget once after its root presentation selector changes.
                // Keep this synchronous for detached and inactive tab contents.
                auto presentationWidgets = findChildren<QWidget *>();
                presentationWidgets.prepend(this);
                for (auto *presentationWidget : presentationWidgets) {
                    presentationWidget->style()->unpolish(presentationWidget);
                    presentationWidget->style()->polish(presentationWidget);
                    QEvent styleChange(QEvent::StyleChange);
                    QCoreApplication::sendEvent(presentationWidget, &styleChange);
                }
                // Qt clears stylesheet height constraints when the rule leaves.
                // Restore the programmatic bounds captured before canvas entry.
                if (!canvas) {
                    for (auto *progressBar : progressBars) {
                        const QVariant originalMaximum = progressBar->property("canvasOriginalMaximumHeight");
                        if (!originalMaximum.isValid())
                            continue;
                        progressBar->setMaximumHeight(originalMaximum.toInt());
                        progressBar->setProperty("canvasOriginalMaximumHeight", QVariant());
                    }
                }
                updateGeometry();
            }
        }
        return QWidget::event(event);
    }
};
QLabel *notice(QWidget *parent, const QString &text = {})
{
    auto *label = new QLabel(text, parent);
    label->setObjectName("dashboardWidgetStatus");
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    return label;
}

QTableWidget *compactTable(const QStringList &headers, QWidget *parent)
{
    auto *table = new QTableWidget(0, headers.size(), parent);
    table->setHorizontalHeaderLabels(headers);
    table->horizontalHeader()->setSectionResizeMode(QHeaderView::Stretch);
    table->verticalHeader()->hide();
    table->verticalHeader()->setDefaultSectionSize(30);
    table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    table->setSelectionBehavior(QAbstractItemView::SelectRows);
    table->setSelectionMode(QAbstractItemView::SingleSelection);
    table->setShowGrid(false);
    table->setAlternatingRowColors(true);
    return table;
}

void addRow(QTableWidget *table, const QStringList &cells)
{
    const int row = table->rowCount();
    table->insertRow(row);
    for (int column = 0; column < cells.size(); ++column) {
        auto *item = new QTableWidgetItem(cells[column]);
        item->setToolTip(cells[column]);
        table->setItem(row, column, item);
    }
}
}

QWidget *DashboardWidgetFactory::createSystem(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *overview = new SystemResourceOverview(widget, true);
    layout->addWidget(overview, 1);
    auto *status = notice(widget, tr("Waiting for measurements…"));
    layout->addWidget(status);
    connect(this, &DashboardWidgetFactory::snapshotChanged, widget,
        [overview, status](const QJsonObject &snapshot, const QString &error) {
            if (!error.isEmpty()) {
                overview->clear();
                status->setText(error);
                return;
            }
            overview->applySnapshot(snapshot);
            status->setText(tr("This computer · updates every 4 seconds"));
        });
    return widget;
}

QWidget *DashboardWidgetFactory::createExecutions(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *table = compactTable({tr("Workflow / task"), tr("State")}, widget);
    table->setObjectName("dashboardExecutions");
    layout->addWidget(table, 1);
    auto *status = notice(widget, tr("Waiting for recent operations…"));
    layout->addWidget(status);
    connect(this, &DashboardWidgetFactory::snapshotChanged, widget,
        [table, status](const QJsonObject &snapshot, const QString &error) {
            table->setRowCount(0);
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            const auto executions = snapshot.value("executions").toArray();
            QVector<QJsonObject> sorted;
            for (const auto &entry : executions)
                sorted.append(entry.toObject());
            std::stable_sort(sorted.begin(), sorted.end(), [](const QJsonObject &left, const QJsonObject &right) {
                return left.value("started_at").toString() > right.value("started_at").toString();
            });
            for (int index = 0; index < qMin(12, int(sorted.size())); ++index) {
                const auto &entry = sorted[index];
                QString title = entry.value("workflow").toString();
                if (title.isEmpty())
                    title = tr("Standalone operation");
                const QString task = entry.value("task").toString();
                if (!task.isEmpty())
                    title += " / " + task;
                addRow(table, {title, entry.value("state").toString()});
            }
            status->setText(executions.isEmpty() ? tr("No recorded operations in this project.")
                : tr("Latest %1 of %2 managed operations").arg(table->rowCount()).arg(executions.size()));
        });
    connect(table, &QTableWidget::cellDoubleClicked, this, [this]() { emit openFullView("executions"); });
    return widget;
}

QWidget *DashboardWidgetFactory::createWorkflows(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *search = new QLineEdit;
    search->setObjectName("dashboardWorkflowSearch");
    search->setPlaceholderText(tr("Find a workflow…"));
    layout->addWidget(search);
    auto *list = new QListWidget;
    list->setObjectName("dashboardWorkflowList");
    layout->addWidget(list, 1);
    auto *status = notice(widget);
    layout->addWidget(status);
    auto *launch = new QPushButton(tr("Launch workflow"));
    launch->setObjectName("dashboardLaunchWorkflow");
    layout->addWidget(launch);
    auto *clone = new QPushButton(tr("Clone to workflows"));
    clone->setObjectName("dashboardCloneWorkflow");
    layout->addWidget(clone);
    const auto selectionChanged = [list, launch, clone]() {
        const auto *item = list->currentItem();
        launch->setEnabled(item && item->data(Qt::UserRole + 1).toBool());
        clone->setVisible(item && item->data(Qt::UserRole + 2).toBool());
    };
    const auto render = [this, search, list, status, launch, selectionChanged]() {
        const QString selected = list->currentItem() ? list->currentItem()->data(Qt::UserRole).toString() : QString();
        const QSignalBlocker blocker(list);
        list->clear();
        int matching = 0;
        for (const auto &workflow : m_workflows) {
            const QString title = workflow.catalogTitle();
            if (!(title + " " + workflow.workflowId).contains(search->text().trimmed(), Qt::CaseInsensitive))
                continue;
            ++matching;
            // A compact launcher shows a short searchable window, not the full
            // catalog page. Searching still matches the complete shared catalog.
            if (list->count() >= 40)
                continue;
            auto *item = new QListWidgetItem(title, list);
            item->setData(Qt::UserRole, workflow.workflowId);
            item->setToolTip(workflow.catalogDescription());
            item->setData(Qt::UserRole + 1, workflow.supported);
            item->setData(Qt::UserRole + 2, workflow.cloneable);
            if (workflow.workflowId == selected)
                list->setCurrentItem(item);
        }
        if (!list->currentItem() && list->count() > 0)
            list->setCurrentRow(0);
        selectionChanged();
        status->setText(matching == 0 ? tr("No matching workflows. Open Workflows to choose a project or refresh its catalog.")
            : tr("%1 of %2 workflows · configure before launch").arg(list->count()).arg(matching));
    };
    connect(this, &DashboardWidgetFactory::workflowsChanged, widget, render);
    connect(search, &QLineEdit::textChanged, widget, render);
    connect(list, &QListWidget::currentRowChanged, widget, selectionChanged);
    connect(clone, &QPushButton::clicked, this, [this, list]() {
        if (const auto *item = list->currentItem())
            emit cloneWorkflow(item->data(Qt::UserRole).toString());
    });
    const auto launchSelected = [this, list]() {
        if (list->currentItem() && list->currentItem()->data(Qt::UserRole + 1).toBool())
            emit launchWorkflowOnMachine(list->currentItem()->data(Qt::UserRole).toString(), machineTarget(), m_sharedRoot);
    };
    connect(launch, &QPushButton::clicked, this, launchSelected);
    connect(list, &QListWidget::itemDoubleClicked, this, launchSelected);
    render();
    return widget;
}

QWidget *DashboardWidgetFactory::createContainers(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *table = compactTable({tr("Container"), tr("State"), tr("Image")}, widget);
    table->setObjectName("dashboardContainers");
    layout->addWidget(table, 1);
    auto *status = notice(widget, tr("Waiting for Docker…"));
    layout->addWidget(status);
    connect(this, &DashboardWidgetFactory::containersChanged, widget,
        [this, table, status](const QJsonArray &containers, const QString &error) {
            table->setRowCount(0);
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            for (int index = 0; index < qMin(20, int(containers.size())); ++index) {
                const auto entry = containers[index].toObject();
                addRow(table, {entry.value("Names").toString(), entry.value("State").toString(), entry.value("Image").toString()});
            }
            status->setText(containers.isEmpty() && m_sharedMachine.isEmpty() ? tr("No local Docker containers.")
                : tr("%1 of %2 containers · includes stopped containers").arg(table->rowCount()).arg(containers.size()));
        });
    connect(table, &QTableWidget::cellDoubleClicked, this, [this]() { emit openFullView("containers"); });
    return widget;
}

QWidget *DashboardWidgetFactory::createFiles(QWidget *parent, const QString &scope)
{
    auto *widget = new DashboardFilesWidget(parent);
    widget->setMachineContext(machineTarget(), m_sharedRoot, m_workdir);
    widget->setContext(scope, m_workdir);
    connect(this, &DashboardWidgetFactory::workdirChanged, widget, [widget](const QString &project) {
        widget->setContext(widget->scope(), project);
    });
    connect(widget, &DashboardFilesWidget::openFileRequested, widget, [this, widget](const QString &path) {
        const QString root = !widget->machineTarget().isEmpty() ? QStringLiteral("/")
            : (widget->scope() == "project" ? m_workdir : widget->currentDirectory());
        emit filePreviewRequested(widget, root, path);
    });
    return widget;
}

QWidget *DashboardWidgetFactory::createGit(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *branch = notice(widget);
    branch->setObjectName("dashboardGitBranch");
    layout->addWidget(branch);
    auto *list = new QListWidget;
    list->setObjectName("dashboardGitChanges");
    layout->addWidget(list, 1);
    auto *status = notice(widget, tr("Waiting for Git status…"));
    layout->addWidget(status);
    connect(this, &DashboardWidgetFactory::gitChanged, widget,
        [this, branch, list, status](const QJsonObject &snapshot, const QString &error) {
            list->clear();
            branch->clear();
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            branch->setText(snapshot.value("branch").toString().isEmpty() ? tr("Detached HEAD") : snapshot.value("branch").toString());
            const auto changes = snapshot.value("changes").toArray();
            for (const auto &value : changes) {
                if (list->count() >= 32)
                    break;
                const auto entry = value.toObject();
                const QString code = entry.value("status").toString();
                const bool conflict = code.contains('U') || code == "AA" || code == "DD";
                const bool staged = !conflict && code.size() >= 2 && code.front() != ' ' && code.front() != '?';
                const bool unstaged = conflict || code == "??" || code.size() < 2 || code.at(1) != ' ';
                for (bool stagedArea : {false, true}) {
                    if ((stagedArea && !staged) || (!stagedArea && !unstaged) || list->count() >= 32)
                        continue;
                    const QString caption = stagedArea ? tr("Staged") : tr("Working tree");
                    auto *item = new QListWidgetItem(caption + " · " + entry.value("path").toString(), list);
                    item->setData(Qt::UserRole, entry.value("path").toString());
                    item->setData(Qt::UserRole + 1, m_sharedMachine.isEmpty() ? m_workdir : QStringLiteral("/"));
                    item->setData(Qt::UserRole + 2, stagedArea);
                }
            }
            status->setText(changes.isEmpty() ? tr("Working tree is clean.")
                : tr("%1 changed files · %2 review entries shown").arg(changes.size()).arg(list->count()));
        });
    connect(list, &QListWidget::itemActivated, widget, [this, widget](QListWidgetItem *item) {
        emit gitPreviewRequested(widget, item->data(Qt::UserRole + 1).toString(),
            item->data(Qt::UserRole).toString(), item->data(Qt::UserRole + 2).toBool());
    });
    return widget;
}

QWidget *DashboardWidgetFactory::createMachines(QWidget *parent)
{
    auto *widget = new CanvasDashboardView(parent);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *table = compactTable({tr("Machine"), tr("Access")}, widget);
    table->setObjectName("dashboardMachines");
    layout->addWidget(table, 1);
    auto *status = notice(widget, tr("Waiting for managed machines…"));
    layout->addWidget(status);
    connect(this, &DashboardWidgetFactory::machinesChanged, widget,
        [table, status](const QJsonArray &nodes, const QString &error) {
            table->setRowCount(0);
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            for (int index = 0; index < qMin(20, int(nodes.size())); ++index) {
                const auto entry = nodes[index].toObject();
                addRow(table, {entry.value("node").toString(), entry.value("status").toString()});
            }
            status->setText(nodes.isEmpty() ? tr("No managed machines. Add one in Machines.")
                : tr("%1 of %2 machines · access status does not imply online").arg(table->rowCount()).arg(nodes.size()));
        });
    connect(table, &QTableWidget::cellDoubleClicked, this, [this]() { emit openFullView("machines"); });
    return widget;
}

QWidget *DashboardWidgetFactory::createBrowser(QWidget *parent, const QString &contextId)
{
    const QString identity = contextId.isEmpty() ? QUuid::createUuid().toString(QUuid::WithoutBraces) : contextId;
    auto *browser = new RemoteBrowserWidget(identity, parent);
    const bool unlocked = m_browserAccess && m_browserAccess->state() == RemoteAccessMonitor::State::Unlocked;
    browser->setMachineCatalog(m_machineCatalog, unlocked);
    connect(browser, &RemoteBrowserWidget::refreshRequested, this, &DashboardWidgetFactory::machineCatalogRefreshRequested);
    connect(browser, &RemoteBrowserWidget::unlockRequested, this, [this] {
        if (m_browserAccess)
            m_browserAccess->unlock();
    });
    return browser;
}
