#include "DashboardWidgetFactory.h"
#include "DashboardFilesWidget.h"
#include <QDesktopServices>
#include <QMessageBox>
#include <QUrl>

#include "BrowserWidget.h"
#include "SystemResourceOverview.h"

#include <QComboBox>
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
#include <QSignalBlocker>
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
                setStyleSheet(canvas ? QStringLiteral(
                    "QWidget#canvasDashboardView { background: transparent; }"
                    "QWidget#canvasDashboardView QFrame#systemMetricCard { background: palette(base); border: none; border-radius: 14px; }"
                    "QWidget#canvasDashboardView QLabel#systemMetricTitle { color: palette(highlight); font-size: 11px; }"
                    "QWidget#canvasDashboardView QProgressBar { border: none; border-radius: 3px; background: palette(alternate-base); max-height: 5px; }"
                    "QWidget#canvasDashboardView QProgressBar::chunk { border-radius: 3px; background: palette(highlight); }"
                    "QWidget#canvasDashboardView QListWidget, QWidget#canvasDashboardView QTableWidget { background: transparent; border: none; border-radius: 10px; }"
                    "QWidget#canvasDashboardView QListWidget::item { padding: 5px 7px; border-radius: 7px; }"
                    "QWidget#canvasDashboardView QListWidget::item:selected { background: palette(highlight); color: palette(highlighted-text); }"
                    "QWidget#canvasDashboardView QLineEdit { border: none; border-radius: 14px; padding: 7px 10px; background: palette(base); }"
                    "QWidget#canvasDashboardView QPushButton { border: none; border-radius: 14px; padding: 7px 12px; background: palette(alternate-base); }"
                    "QWidget#canvasDashboardView QPushButton:hover { background: palette(highlight); color: palette(highlighted-text); }"
                    "QWidget#canvasDashboardView QLabel#dashboardWidgetStatus { font-size: 11px; padding: 3px; }") : QString());
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
            emit launchWorkflow(list->currentItem()->data(Qt::UserRole).toString());
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
        [table, status](const QJsonArray &containers, const QString &error) {
            table->setRowCount(0);
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            for (int index = 0; index < qMin(20, int(containers.size())); ++index) {
                const auto entry = containers[index].toObject();
                addRow(table, {entry.value("Names").toString(), entry.value("State").toString(), entry.value("Image").toString()});
            }
            status->setText(containers.isEmpty() ? tr("No local Docker containers.")
                : tr("%1 of %2 containers · includes stopped containers").arg(table->rowCount()).arg(containers.size()));
        });
    connect(table, &QTableWidget::cellDoubleClicked, this, [this]() { emit openFullView("containers"); });
    return widget;
}

QWidget *DashboardWidgetFactory::createFiles(QWidget *parent, const QString &scope)
{
    auto *widget = new DashboardFilesWidget(parent);
    widget->setContext(scope, m_workdir);
    connect(this, &DashboardWidgetFactory::workdirChanged, widget, [widget](const QString &project) {
        widget->setContext(widget->scope(), project);
    });
    connect(widget, &DashboardFilesWidget::openFileRequested, widget, [this, widget](const QString &path) {
        if (widget->scope() == "project") {
            emit openFile(path);
        } else if (!QDesktopServices::openUrl(QUrl::fromLocalFile(path))) {
            QMessageBox::warning(widget, tr("Open file"), tr("The system could not open this file."));
        }
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
        [branch, list, status](const QJsonObject &snapshot, const QString &error) {
            list->clear();
            branch->clear();
            if (!error.isEmpty()) {
                status->setText(error);
                return;
            }
            branch->setText(snapshot.value("branch").toString().isEmpty() ? tr("Detached HEAD") : snapshot.value("branch").toString());
            const auto changes = snapshot.value("changes").toArray();
            for (int index = 0; index < qMin(32, int(changes.size())); ++index) {
                const auto entry = changes[index].toObject();
                list->addItem(entry.value("status").toString() + "  " + entry.value("path").toString());
            }
            status->setText(changes.isEmpty() ? tr("Working tree is clean.")
                : tr("%1 of %2 changes · review and commit in Git").arg(list->count()).arg(changes.size()));
        });
    connect(list, &QListWidget::itemDoubleClicked, this, [this]() { emit openFullView("git"); });
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

QWidget *DashboardWidgetFactory::createBrowser(QWidget *parent)
{
    return new BrowserWidget(parent);
}
