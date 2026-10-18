#include "MainWindow.h"

#include "DockpipeChoices.h"
#include "WorkflowEditorWidget.h"
#include "ProjectFilesWidget.h"
#include "ProjectGitWidget.h"
#include "CanvasIcons.h"
#include "CanvasPanel.h"
#include "ActivityWidget.h"

#include <QApplication>
#include <QComboBox>
#include <QDialog>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QFontDatabase>
#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QShortcut>
#include <QSettings>
#include <QScrollArea>
#include <QSignalBlocker>
#include <QSplitter>
#include <QStatusBar>
#include <QTimer>
#include <QVBoxLayout>

void MainWindow::setupAdvancedPage(QWidget *page)
{
    auto *root = new QVBoxLayout(page);
    root->setContentsMargins(0, 0, 0, 0);
    root->setSpacing(0);
    m_workflowEditor = new WorkflowEditorWidget(m_settings.projectFolder,
        DockpipeChoices::preferredDockpipeBinary(m_settings.projectFolder), page);
    root->addWidget(m_workflowEditor);
    connect(m_workflowEditor, &WorkflowEditorWidget::browseRequested, this, &MainWindow::openWorkflowPicker);
    connect(m_workflowEditor, &WorkflowEditorWidget::documentChanged, this, [this](const QString &path) {
        m_canvasContext = Context{};
        if (!path.isEmpty()) {
            m_canvasContext = Context::createNew();
            m_canvasContext.workdir = m_workflowEditor->workdir();
            if (m_canvasContext.workdir.isEmpty())
                m_canvasContext.workdir = QFileInfo(path).absolutePath();
            m_canvasContext.workflowFile = path;
            m_canvasContext.label = m_workflowEditor->workflowName();
            if (m_canvasContext.label.isEmpty())
                m_canvasContext.label = QFileInfo(path).completeBaseName();
            m_canvasContext.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(m_canvasContext.workdir);
        }
        if (path.isEmpty())
            setWorkflowConsoleVisible(false);
        refreshInlineConsole();
    });
    connect(m_workflowEditor, &WorkflowEditorWidget::workflowSaved, this, [this](const QString &) {
        startWorkflowCatalogDiscovery();
    });
    connect(m_workflowEditor, &WorkflowEditorWidget::runRequested, this, [this](const QString &) {
        onLaunch();
    });

    m_consolePanel = new CanvasPanel("output");
    m_consolePanel->setObjectName("workflowTerminalPanel");
    auto *consoleLayout = new QVBoxLayout(m_consolePanel);
    consoleLayout->setContentsMargins(16, 8, 16, 8);
    consoleLayout->setSpacing(6);
    auto *consoleTitleBar = new QWidget;
    consoleTitleBar->setObjectName("canvasOutputTitleBar");
    auto *consoleHeader = new QHBoxLayout(consoleTitleBar);
    consoleHeader->setContentsMargins(0, 0, 0, 0);
    static_cast<CanvasPanel *>(m_consolePanel)->setDragHandle(consoleTitleBar);
    m_consoleTitle = new QLabel(tr("Run output"));
    m_consoleTitle->setObjectName("consoleTitle");
    consoleHeader->addWidget(m_consoleTitle, 1);
    auto *stop = new QPushButton(tr("Stop"));
    auto *logs = new QPushButton(tr("Full logs"));
    auto *hide = new QPushButton(tr("Hide"));
    hide->setToolTip(tr("Hide output (Ctrl+` / ⌘`)"));
    for (auto *control : {stop, logs, hide})
        consoleHeader->addWidget(control);
    connect(stop, &QPushButton::clicked, this, &MainWindow::onStop);
    connect(logs, &QPushButton::clicked, this, &MainWindow::onOpenLogs);
    connect(hide, &QPushButton::clicked, this, [this]() { setWorkflowConsoleVisible(false); });
    consoleLayout->addWidget(consoleTitleBar);
    m_consoleMeta = new QLabel;
    m_consoleMeta->setObjectName("consoleMeta");
    m_consoleMeta->setTextFormat(Qt::PlainText);
    m_consoleMeta->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    consoleLayout->addWidget(m_consoleMeta);
    m_console = new QPlainTextEdit;
    m_console->setObjectName("inlineConsole");
    m_console->setReadOnly(true);
    m_console->setLineWrapMode(QPlainTextEdit::NoWrap);
    m_console->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));
    m_console->setMaximumBlockCount(10000);
    m_console->setMinimumHeight(70);
    consoleLayout->addWidget(m_console, 1);
    auto *input = new QLineEdit;
    input->setObjectName("workflowRunInput");
    input->setPlaceholderText(tr("Send input to the running workflow…"));
    input->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));
    consoleLayout->addWidget(input);
    connect(input, &QLineEdit::returnPressed, this, [this, input]() {
        Context *context = currentContext();
        if (context && m_sessions.isRunning(context->id)) {
            m_sessions.sendInput(context->id, input->text());
            input->clear();
        }
    });
    auto *bottomBar = new QFrame;
    bottomBar->setObjectName("workflowCanvasBar");
    auto *bottom = new QHBoxLayout(bottomBar);
    bottom->setContentsMargins(5, 5, 5, 5);
    m_consoleToggle = new QPushButton(tr("Output"));
    m_consoleToggle->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Terminal));
    m_consoleToggle->setObjectName("workflowConsoleToggle");
    m_consoleToggle->setCheckable(true);
    m_consoleToggle->setToolTip(tr("Toggle run output (Ctrl+` / ⌘`)"));
    connect(m_consoleToggle, &QPushButton::toggled, this, &MainWindow::setWorkflowConsoleVisible);
    bottom->addWidget(m_consoleToggle);
    auto *activity = new QPushButton(tr("Runs && logs"));
    activity->setObjectName("workflowActivityButton");
    activity->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Workflow));
    bottom->addWidget(activity);
    connect(activity, &QPushButton::clicked, this, [this]() {
        if (!m_workflowActivityPanel) {
            auto *panel = new CanvasPanel("workflowActivity", m_workflowEditor);
            m_workflowActivityPanel = panel;
            panel->setObjectName("workflowActivityPanel");
            auto *layout = new QVBoxLayout(panel);
            auto *header = new QWidget;
            auto *row = new QHBoxLayout(header);
            row->addWidget(new QLabel(tr("Workspace runs · Remote jobs includes all broker jobs")), 1);
            auto *close = new QPushButton;
            close->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Close));
            close->setAccessibleName(tr("Close runs and logs"));
            row->addWidget(close);
            connect(close, &QPushButton::clicked, panel, &QWidget::hide);
            panel->setDragHandle(header);
            layout->addWidget(header);
            m_workflowActivity = new ActivityWidget(m_sessions, m_store, panel);
            m_workflowActivity->setWorkspaceFilter(true);
            layout->addWidget(m_workflowActivity, 1);
        }
        m_workflowActivity->setWorkdir(m_settings.projectFolder);
        m_workflowActivity->refreshLocal();
        m_workflowEditor->setActivityPanel(m_workflowActivityPanel);
    });
    auto *shortcuts = new QPushButton;
    shortcuts->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Keyboard));
    shortcuts->setObjectName("workflowShortcuts");
    shortcuts->setAccessibleName(tr("Keyboard shortcuts"));
    shortcuts->setToolTip(tr("Keyboard shortcuts"));
    bottom->addWidget(shortcuts);
    connect(shortcuts, &QPushButton::clicked, this, [this]() {
        QMessageBox::information(this, tr("Canvas shortcuts"),
            tr("Ctrl / ⌘ + P     Switch workflow\nCtrl / ⌘ + S     Save workflow\nCtrl / ⌘ + `     Toggle output\n\nScroll to zoom · Drag the background to pan\nSelect a card to edit its properties"));
    });
    m_runTarget = new QComboBox;
    m_runTarget->setObjectName("runTarget");
    m_runTarget->setAccessibleName(tr("Run on"));
    m_runTarget->setToolTip(tr("Choose the computer to run this workflow"));
    m_runTarget->addItem(CanvasIcons::icon(CanvasIcons::Symbol::Monitor), tr("This computer"), QString());
    m_runTarget->setMinimumWidth(160);
    const QString lastTarget = QSettings().value("workflowStudio/lastRunTarget").toString();
    if (!lastTarget.isEmpty()) {
        m_runTarget->addItem(lastTarget + tr(" — unavailable"), lastTarget);
        m_runTarget->setItemData(1, true, Qt::UserRole + 1);
        m_runTarget->setCurrentIndex(1);
    }
    connect(m_runTarget, &QComboBox::currentIndexChanged, this, [this](int index) {
        QSettings().setValue("workflowStudio/lastRunTarget", m_runTarget->currentData());
        if (m_pickerTarget) {
            const QSignalBlocker blocker(m_pickerTarget);
            m_pickerTarget->setCurrentIndex(index);
        }
        updateWorkflowPreview();
    });
    bottom->addWidget(m_runTarget);
    m_workflowEditor->setConsoleWidgets(m_consolePanel, bottomBar);

    auto *toggleOutput = new QShortcut(QKeySequence(Qt::CTRL | Qt::Key_QuoteLeft), page);
    toggleOutput->setContext(Qt::WidgetWithChildrenShortcut);
    connect(toggleOutput, &QShortcut::activated, this, [this]() {
        setWorkflowConsoleVisible(m_consolePanel->isHidden());
    });
    auto *picker = new QShortcut(QKeySequence(Qt::CTRL | Qt::Key_P), page);
    picker->setContext(Qt::WidgetWithChildrenShortcut);
    connect(picker, &QShortcut::activated, this, &MainWindow::openWorkflowPicker);
    setupWorkflowPicker();
    m_basicLaunchingTimer = new QTimer(this);
    m_basicLaunchingTimer->setInterval(1000);
    connect(m_basicLaunchingTimer, &QTimer::timeout, this, [this]() {
        if (m_basicLaunchingContextId.isEmpty()) {
            m_basicLaunchingTimer->stop();
            return;
        }
        const bool stillRunning = m_sessions.isRunning(m_basicLaunchingContextId);
        const SessionInfo si = m_sessions.info(m_basicLaunchingContextId);
        if (!stillRunning || si.ready || si.status == SessionStatus::Failed || si.status == SessionStatus::Stopped) {
            m_basicLaunchingContextId.clear();
            m_basicLaunchingWorkflowId.clear();
            m_basicLaunchingTimer->stop();
            refreshSessionUi();
        }
    });

}

void MainWindow::setupWorkflowPicker()
{
    m_workflowPicker = new QDialog(this);
    m_workflowPicker->setObjectName("workflowPicker");
    m_workflowPicker->setWindowTitle(tr("Workflows"));
    m_workflowPicker->resize(960, 650);
    m_recentWorkflowKeys = QSettings().value("workflowStudio/recentWorkflows").toStringList().mid(0, 12);
    auto *pickerLayout = new QVBoxLayout(m_workflowPicker);
    pickerLayout->setContentsMargins(20, 20, 20, 20);
    pickerLayout->setSpacing(12);
    m_hint = new QLabel(tr("Run a saved workflow, or open it on the canvas to make changes."));
    m_hint->setObjectName(QStringLiteral("hintText"));
    m_hint->setWordWrap(true);
    pickerLayout->addWidget(m_hint);

    m_search = new QLineEdit(m_workflowPicker);
    m_search->setObjectName(QStringLiteral("surfaceSearch"));
    m_search->setClearButtonEnabled(true);
    m_search->setPlaceholderText(tr("Search workflows by label, folder, workflow, resolver…"));
    connect(m_search, &QLineEdit::textChanged, this, &MainWindow::onAdvancedSearchChanged);
    pickerLayout->addWidget(m_search);

    m_advancedSearchTimer = new QTimer(this);
    m_advancedSearchTimer->setSingleShot(true);
    m_advancedSearchTimer->setInterval(120);
    connect(m_advancedSearchTimer, &QTimer::timeout, this, &MainWindow::applyAdvancedContextFilter);

    auto *listPanel = new QFrame;
    listPanel->setObjectName(QStringLiteral("listPanel"));
    auto *listOuter = new QVBoxLayout(listPanel);
    listOuter->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(m_workflowPicker);
    m_list->setObjectName("workflowPickerList");
    m_list->setFrameShape(QFrame::NoFrame);
    m_list->setSpacing(2);
    m_list->setUniformItemSizes(true);

    listOuter->addWidget(m_list, 1);

    m_emptyState = new QFrame;
    m_emptyState->setObjectName(QStringLiteral("emptyState"));
    auto *emptyLay = new QVBoxLayout(m_emptyState);
    emptyLay->setContentsMargins(28, 36, 28, 36);
    emptyLay->setSpacing(8);
    m_emptyTitle = new QLabel(tr("No workflows yet"));
    m_emptyTitle->setObjectName(QStringLiteral("emptyTitle"));
    m_emptyTitle->setAlignment(Qt::AlignCenter);
    m_emptyBody = new QLabel(
        tr("Open a project folder with File → Open project folder."));
    m_emptyBody->setObjectName(QStringLiteral("emptyBody"));
    m_emptyBody->setWordWrap(true);
    m_emptyBody->setAlignment(Qt::AlignCenter);
    emptyLay->addWidget(m_emptyTitle);
    emptyLay->addWidget(m_emptyBody);

    listOuter->addWidget(m_emptyState, 1);
    listOuter->addWidget(m_list, 1);
    m_emptyState->hide();
    m_list->hide();

    auto *browser = new QSplitter;
    browser->setChildrenCollapsible(false);
    listPanel->setMinimumWidth(260);
    browser->addWidget(listPanel);
    auto *previewPanel = new QFrame;
    previewPanel->setObjectName("workflowPreviewPanel");
    previewPanel->setMinimumWidth(280);
    auto *previewLayout = new QVBoxLayout(previewPanel);
    previewLayout->setContentsMargins(20, 20, 20, 20);
    previewLayout->setSpacing(16);
    auto *previewScroll = new QScrollArea;
    previewScroll->setObjectName("workflowPreviewScroll");
    previewScroll->setFrameShape(QFrame::NoFrame);
    previewScroll->setWidgetResizable(true);
    m_workflowPreview = new QLabel;
    m_workflowPreview->setObjectName("workflowPreview");
    m_workflowPreview->setWordWrap(true);
    m_workflowPreview->setTextFormat(Qt::RichText);
    m_workflowPreview->setAlignment(Qt::AlignTop | Qt::AlignLeft);
    m_workflowPreview->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_workflowPreview->setMargin(0);
    previewScroll->setWidget(m_workflowPreview);
    previewLayout->addWidget(previewScroll, 1);
    previewLayout->addWidget(new QLabel(tr("Run on")));
    m_pickerTarget = new QComboBox;
    m_pickerTarget->setObjectName("workflowPickerTarget");
    m_pickerTarget->setAccessibleName(tr("Run on"));
    m_pickerTarget->setModel(m_runTarget->model());
    m_pickerTarget->setCurrentIndex(m_runTarget->currentIndex());
    connect(m_pickerTarget, &QComboBox::activated, this, [this](int index) {
        m_runTarget->setCurrentIndex(index);
    });
    previewLayout->addWidget(m_pickerTarget);
    browser->addWidget(previewPanel);
    browser->setStretchFactor(0, 1);
    browser->setStretchFactor(1, 1);
    pickerLayout->addWidget(browser, 1);
    auto *buttons = new QHBoxLayout;
    auto *file = new QPushButton(tr("Open YAML file…"));
    file->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Folder));
    auto *refresh = new QPushButton(tr("Refresh"));
    auto *cancel = new QPushButton(tr("Cancel"));
    auto *open = new QPushButton(tr("Edit on canvas"));
    m_pickerEdit = open;
    open->setObjectName("openWorkflowOnCanvas");
    m_pickerRun = new QPushButton(tr("Run"));
    m_pickerRun->setObjectName("runPickedWorkflow");
    m_pickerRun->setProperty("dialogPrimary", true);
    m_pickerRun->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Play));
    // Search Enter must never execute a workflow accidentally.
    for (auto *control : {file, refresh, cancel, open, m_pickerRun})
        control->setAutoDefault(false);
    buttons->addWidget(file);
    buttons->addWidget(refresh);
    buttons->addStretch();
    buttons->addWidget(cancel);
    buttons->addWidget(open);
    buttons->addWidget(m_pickerRun);
    pickerLayout->addLayout(buttons);
    connect(file, &QPushButton::clicked, this, [this]() {
        const QString path = QFileDialog::getOpenFileName(m_workflowPicker, tr("Open workflow"),
            m_settings.projectFolder, tr("Workflow YAML (*.yml *.yaml)"));
        if (!path.isEmpty() && m_workflowEditor->openWorkflow(path))
            m_workflowPicker->accept();
    });
    connect(refresh, &QPushButton::clicked, this, &MainWindow::startWorkflowCatalogDiscovery);
    connect(cancel, &QPushButton::clicked, m_workflowPicker, &QDialog::reject);
    connect(open, &QPushButton::clicked, this, &MainWindow::openPickedWorkflow);
    connect(m_pickerRun, &QPushButton::clicked, this, &MainWindow::runPickedWorkflow);
    connect(m_list, &QListWidget::currentItemChanged, this, [this]() { updateWorkflowPreview(); });
    connect(m_search, &QLineEdit::returnPressed, m_list, qOverload<>(&QWidget::setFocus));
    updateWorkflowPreview();

}

void MainWindow::openWorkflowPicker()
{
    startWorkflowCatalogDiscovery();
    updateWorkflowPreview();
    m_search->setFocus();
    m_search->selectAll();
    m_workflowPicker->exec();
    refreshInlineConsole();
}

void MainWindow::openPickedWorkflow()
{
    const Context *selected = currentAdvancedDisplayContext();
    if (!selected)
        return;
    const Context context = *selected;
    const WorkflowMeta meta = findWorkflowMeta(context.workdir, context.workflow, context.workflowFile);
    const QString path = context.workflowFile.isEmpty() ? meta.configPath : context.workflowFile;
    if (path.isEmpty()) {
        QMessageBox::information(m_workflowPicker, tr("Workflow source unavailable"),
            tr("This workflow does not expose its YAML source. Refresh the catalog or open a YAML file."));
        return;
    }
    if (m_workflowEditor->openWorkflow(path))
        m_workflowPicker->accept();
}

void MainWindow::setWorkflowConsoleVisible(bool visible)
{
    m_workflowEditor->setConsoleVisible(visible);
    const QSignalBlocker blocker(m_consoleToggle);
    m_consoleToggle->setChecked(visible);

}

Context *MainWindow::activeWorkflowContext()
{
    return m_canvasContext.workflowFile.isEmpty() ? nullptr : &m_canvasContext;
}

bool MainWindow::setWorkflowWorkspace(const QString &workdir)
{
    if (m_projectGit && m_projectGit->busy()) {
        statusBar()->showMessage(tr("Wait for the Git action to finish, or cancel it in Git before changing projects."), 8000);
        return false;
    }
    if (m_projectFiles && !m_projectFiles->confirmLeave())
        return false;
    if (m_workflowEditor && !m_workflowEditor->setWorkspace(workdir, DockpipeChoices::preferredDockpipeBinary(workdir)))
        return false;
    if (m_projectFiles)
        m_projectFiles->setRoot(workdir);
    if (m_projectGit)
        m_projectGit->setWorkdir(workdir);
    return true;
}

void MainWindow::quitLauncher()
{
    if (m_projectGit && m_projectGit->busy()) {
        statusBar()->showMessage(tr("Wait for the Git action to finish, or cancel it in Git before quitting."), 8000);
        return;
    }
    if ((!m_projectFiles || m_projectFiles->confirmLeave())
        && (!m_workflowEditor || m_workflowEditor->confirmDiscardChanges())
        && confirmDashboardClose())
        qApp->quit();
}

void MainWindow::updateWorkflowPreview()
{
    if (!m_workflowPreview || !m_pickerRun || !m_pickerEdit)
        return;
    const Context *selected = currentAdvancedDisplayContext();
    const bool unavailable = m_runTarget->currentData(Qt::UserRole + 1).toBool();
    m_pickerRun->setEnabled(selected && !unavailable && !m_advancedDiscoveryLoading);
    m_pickerEdit->setEnabled(selected != nullptr);
    if (!selected) {
        m_workflowPreview->setText(tr("<h3>Select a workflow</h3><p>Review its purpose and inputs here, then choose Run or Edit on canvas.</p>"));
        return;
    }
    const WorkflowMeta meta = findWorkflowMeta(selected->workdir, selected->workflow, selected->workflowFile);
    const QString title = meta.displayName.isEmpty() ? selected->label : meta.displayName;
    const QString description = meta.description.isEmpty() ? tr("No description provided.") : meta.description;
    QString text = "<h2>" + title.toHtmlEscaped() + "</h2><p>" + description.toHtmlEscaped() + "</p>";
    text += "<p><b>" + tr("Inputs") + "</b></p>";
    if (meta.inputs.isEmpty()) {
        text += "<p>" + tr("No declared inputs. The workflow may prompt while running.") + "</p>";
    } else {
        text += "<ul>";
        for (const auto &input : meta.inputs) {
            text += "<li><b>" + input.fieldName.toHtmlEscaped() + "</b>";
            if (!input.description.isEmpty())
                text += " — " + input.description.toHtmlEscaped();
            text += "</li>";
        }
        text += "</ul><p>" + tr("Required values are requested before launch. Saved values are reused.") + "</p>";
    }
    text += "<p><b>" + tr("Workspace") + "</b><br>" + selected->workdir.toHtmlEscaped() + "</p>";
    text += "<p>" + tr("The selected computer needs the tools and platform required by this workflow.") + "</p>";
    if (unavailable)
        text += "<p><b>" + tr("This run target is unavailable. Choose another computer or reconnect it in Machines.") + "</b></p>";
    m_workflowPreview->setText(text);
}

void MainWindow::runPickedWorkflow()
{
    const Context *selected = currentAdvancedDisplayContext();
    if (!selected)
        return;
    const Context context = *selected;
    m_pickerRun->setEnabled(false);
    if (launchWorkflow(context))
        m_workflowPicker->accept();
    updateWorkflowPreview();
}
