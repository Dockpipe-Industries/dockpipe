#include "DashboardWorkflowView.h"

#include "DockpipeChoices.h"
#include "ExecutionExplorer.h"
#include "WorkflowCatalog.h"
#include "WorkflowEditorWidget.h"
#include <QDialog>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QFutureWatcher>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPushButton>
#include <QShortcut>
#include <QShowEvent>
#include <QStackedWidget>
#include <QTabBar>
#include <QVBoxLayout>
#include <QtConcurrent/QtConcurrentRun>

DashboardWorkflowView::DashboardWorkflowView(QWidget *parent)
    : QWidget(parent),
      m_editor(new WorkflowEditorWidget({}, {}, this)),
      m_runs(new ExecutionExplorer(this, ExecutionExplorer::Purpose::WorkflowRuns)),
      m_sections(new QTabBar(this)),
      m_views(new QStackedWidget(this)),
      m_picker(new QDialog(this)),
      m_search(new QLineEdit(m_picker)),
      m_workflows(new QListWidget(m_picker)),
      m_catalogStatus(new QLabel(m_picker)),
      m_open(new QPushButton(tr("Edit on canvas"), m_picker)),
      m_refresh(new QPushButton(tr("Refresh"), m_picker)),
      m_catalogWatcher(new QFutureWatcher<WorkflowCatalogData>(this))
{
    setObjectName("dashboardWorkflowView");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    auto *header = new QHBoxLayout;
    header->setContentsMargins(8, 6, 8, 6);
    m_sections->setObjectName("dashboardWorkflowSections");
    m_sections->setExpanding(false);
    m_sections->setDrawBase(false);
    m_sections->addTab(tr("Editor"));
    m_sections->addTab(tr("Running"));
    m_sections->addTab(tr("History"));
    header->addWidget(m_sections);
    header->addStretch();
    auto *browse = new QPushButton(tr("Open workflow…"), this);
    browse->setObjectName("dashboardBrowseWorkflow");
    header->addWidget(browse);
    layout->addLayout(header);
    m_editor->setMinimumSize(0, 0);
    m_runs->setMinimumSize(0, 0);
    m_views->addWidget(m_editor);
    m_views->addWidget(m_runs);
    layout->addWidget(m_views, 1);
    m_editor->setEnabled(false);

    connect(m_sections, &QTabBar::currentChanged, this, [this](int section) {
        m_runs->setActive(false);
        m_runs->setHistory(section == 2);
        m_views->setCurrentIndex(section == 0 ? 0 : 1);
        updateRunActivity();
    });
    connect(browse, &QPushButton::clicked, this, &DashboardWorkflowView::openWorkflowPicker);
    connect(m_editor, &WorkflowEditorWidget::browseRequested,
        this, &DashboardWorkflowView::openWorkflowPicker);
    connect(m_editor, &WorkflowEditorWidget::runRequested, this, [this](const QString &path) {
        if (!m_workdir.isEmpty() && !path.isEmpty())
            emit workflowRunRequested(m_workdir, path);
    });
    connect(m_editor, &WorkflowEditorWidget::workflowSaved, this, [this]() {
        if (m_picker->isVisible())
            discoverWorkflows();
    });
    auto *pickerShortcut = new QShortcut(QKeySequence(Qt::CTRL | Qt::Key_P), this);
    pickerShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(pickerShortcut, &QShortcut::activated, this, &DashboardWorkflowView::openWorkflowPicker);

    m_picker->setObjectName("dashboardWorkflowPicker");
    m_picker->setWindowTitle(tr("Open workflow"));
    m_picker->resize(720, 520);
    auto *pickerLayout = new QVBoxLayout(m_picker);
    m_search->setPlaceholderText(tr("Search workflows by name, description or path…"));
    m_search->setClearButtonEnabled(true);
    m_catalogStatus->setTextFormat(Qt::PlainText);
    m_catalogStatus->setWordWrap(true);
    m_workflows->setObjectName("dashboardWorkflowPickerList");
    m_workflows->setUniformItemSizes(true);
    pickerLayout->addWidget(m_search);
    pickerLayout->addWidget(m_catalogStatus);
    pickerLayout->addWidget(m_workflows, 1);
    auto *buttons = new QHBoxLayout;
    auto *file = new QPushButton(tr("Open YAML file…"), m_picker);
    auto *cancel = new QPushButton(tr("Cancel"), m_picker);
    for (auto *button : {file, m_refresh, cancel, m_open})
        button->setAutoDefault(false);
    buttons->addWidget(file);
    buttons->addWidget(m_refresh);
    buttons->addStretch();
    buttons->addWidget(cancel);
    buttons->addWidget(m_open);
    pickerLayout->addLayout(buttons);
    m_open->setEnabled(false);
    connect(m_search, &QLineEdit::textChanged, this, &DashboardWorkflowView::filterWorkflows);
    connect(m_search, &QLineEdit::returnPressed, m_workflows, qOverload<>(&QWidget::setFocus));
    connect(m_workflows, &QListWidget::currentItemChanged, this, [this]() {
        const auto *item = m_workflows->currentItem();
        m_open->setEnabled(item && !item->isHidden());
    });
    connect(m_open, &QPushButton::clicked, this, &DashboardWorkflowView::openSelectedWorkflow);
    connect(m_workflows, &QListWidget::itemDoubleClicked, this, [this]() { openSelectedWorkflow(); });
    connect(cancel, &QPushButton::clicked, m_picker, &QDialog::reject);
    connect(m_refresh, &QPushButton::clicked, this, &DashboardWorkflowView::discoverWorkflows);
    connect(file, &QPushButton::clicked, this, [this]() {
        const QString path = QFileDialog::getOpenFileName(m_picker, tr("Open workflow"),
            m_workdir, tr("Workflow YAML (*.yml *.yaml)"));
        if (!path.isEmpty()) {
            openWorkflowAsync(path, [this](bool opened) {
                if (opened)
                    m_picker->accept();
            });
        }
    });
    connect(m_catalogWatcher, &QFutureWatcher<WorkflowCatalogData>::finished, this, [this]() {
        m_refresh->setEnabled(true);
        if (m_catalogRoot != m_workdir) {
            if (m_picker->isVisible())
                discoverWorkflows();
            return;
        }
        const auto catalog = m_catalogWatcher->result();
        m_workflows->clear();
        for (const auto &workflow : catalog.workflows) {
            if (workflow.configPath.isEmpty() || workflow.source == "vscode")
                continue;
            const QString path = QDir(m_workdir).absoluteFilePath(workflow.configPath);
            auto *item = new QListWidgetItem(workflow.catalogTitle() + QStringLiteral("\n")
                + QDir(m_workdir).relativeFilePath(path), m_workflows);
            item->setData(Qt::UserRole, path);
            item->setData(Qt::UserRole + 1, workflow.catalogDescription());
            item->setToolTip(workflow.catalogDescription() + QStringLiteral("\n") + path);
        }
        m_catalogStatus->setText(m_workflows->count() == 0
            ? tr("No editable workflow sources were returned. Refresh the catalog or open a YAML file.")
            : tr("%1 workflows in %2").arg(m_workflows->count()).arg(m_workdir));
        filterWorkflows();
    });
}

bool DashboardWorkflowView::setWorkdir(const QString &workdir)
{
    const QFileInfo directory(workdir);
    if (workdir.trimmed().isEmpty() || !directory.isDir())
        return false;
    const QString root = directory.absoluteFilePath();
    if (root == m_workdir)
        return true;
    if (!m_editor->setWorkspace(root, DockpipeChoices::preferredDockpipeBinary(root)))
        return false;
    m_workdir = root;
    m_editor->setEnabled(true);
    m_runs->setActive(false);
    m_runs->setWorkdir(root);
    m_workflows->clear();
    m_open->setEnabled(false);
    if (m_picker->isVisible())
        discoverWorkflows();
    updateRunActivity();
    return true;
}

bool DashboardWorkflowView::openWorkflow(const QString &path)
{
    if (m_workdir.isEmpty())
        return false;
    if (!m_editor->openWorkflow(QDir(m_workdir).absoluteFilePath(path)))
        return false;
    m_sections->setCurrentIndex(0);
    return true;
}

bool DashboardWorkflowView::confirmLeave()
{
    return m_editor->confirmDiscardChanges();
}

void DashboardWorkflowView::openWorkflowPicker()
{
    if (m_workdir.isEmpty())
        return;
    m_picker->show();
    m_picker->raise();
    m_picker->activateWindow();
    m_search->setFocus();
    discoverWorkflows();
}

void DashboardWorkflowView::discoverWorkflows()
{
    if (m_workdir.isEmpty() || m_catalogWatcher->isRunning())
        return;
    m_catalogRoot = m_workdir;
    m_refresh->setEnabled(false);
    m_workflows->clear();
    m_open->setEnabled(false);
    m_catalogStatus->setText(tr("Discovering workflows in %1…").arg(m_workdir));
    const QString root = m_workdir;
    m_catalogWatcher->setFuture(QtConcurrent::run([root]() {
        return WorkflowCatalog::discoverCatalog(root);
    }));
}

void DashboardWorkflowView::filterWorkflows()
{
    const QString search = m_search->text().trimmed();
    for (int index = 0; index < m_workflows->count(); ++index) {
        auto *item = m_workflows->item(index);
        const QString content = item->text() + QLatin1Char('\n') + item->data(Qt::UserRole + 1).toString();
        item->setHidden(!content.contains(search, Qt::CaseInsensitive));
    }
    const auto *item = m_workflows->currentItem();
    m_open->setEnabled(item && !item->isHidden());
}

void DashboardWorkflowView::openSelectedWorkflow()
{
    const auto *item = m_workflows->currentItem();
    if (item && !item->isHidden()) {
        openWorkflowAsync(item->data(Qt::UserRole).toString(), [this](bool opened) {
            if (opened)
                m_picker->accept();
        });
    }
}

void DashboardWorkflowView::updateRunActivity()
{
    m_runs->setActive(isVisible() && m_sections->currentIndex() > 0 && !m_workdir.isEmpty());
}

void DashboardWorkflowView::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    updateRunActivity();
}

void DashboardWorkflowView::hideEvent(QHideEvent *event)
{
    m_runs->setActive(false);
    m_picker->hide();
    QWidget::hideEvent(event);
}

void DashboardWorkflowView::confirmLeaveAsync(std::function<void(bool)> completion)
{
    m_editor->confirmDiscardChangesAsync(std::move(completion));
}

void DashboardWorkflowView::setWorkdirAsync(const QString &workdir, std::function<void(bool)> completion)
{
    const QFileInfo directory(workdir);
    if (workdir.trimmed().isEmpty() || !directory.isDir()) {
        if (completion)
            completion(false);
        return;
    }
    const QString root = directory.absoluteFilePath();
    m_editor->setWorkspaceAsync(root, DockpipeChoices::preferredDockpipeBinary(root),
        [this, root, completion = std::move(completion)](bool accepted) {
            const bool applied = accepted && setWorkdir(root);
            if (completion)
                completion(applied);
        });
}

void DashboardWorkflowView::openWorkflowAsync(const QString &path, std::function<void(bool)> completion)
{
    if (m_workdir.isEmpty()) {
        if (completion)
            completion(false);
        return;
    }
    m_editor->openWorkflowAsync(QDir(m_workdir).absoluteFilePath(path),
        [this, completion = std::move(completion)](bool opened) {
            if (opened)
                m_sections->setCurrentIndex(0);
            if (completion)
                completion(opened);
        });
}
