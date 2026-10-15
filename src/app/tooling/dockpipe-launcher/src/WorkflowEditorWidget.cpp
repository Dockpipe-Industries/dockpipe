#include "WorkflowEditorWidget.h"
#include "WorkflowScriptPath.h"
#include "ProjectFilesWidget.h"
#include "SourceEditor.h"
#include "WorkflowCanvas.h"
#include "CanvasIcons.h"
#include "CanvasPanel.h"
#include <QGraphicsDropShadowEffect>

#include <QComboBox>
#include <QMenu>
#include <QResizeEvent>
#include <QTabBar>
#include <QWidgetAction>
#include <QCryptographicHash>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QFontDatabase>
#include <QFormLayout>
#include <QHBoxLayout>
#include <QJsonDocument>
#include <QLabel>
#include <QKeySequence>
#include <QLineEdit>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QProcess>
#include <QProcessEnvironment>
#include <QPushButton>
#include <QRegularExpression>
#include <QSaveFile>
#include <QSet>
#include <QScrollArea>
#include <QSettings>
#include <QShortcut>
#include <QSignalBlocker>
#include <QSplitter>
#include <QTabWidget>
#include <QTemporaryFile>
#include <QTimer>
#include <QVBoxLayout>

namespace {
constexpr qsizetype maxSourceBytes = 1024 * 1024;
constexpr qsizetype maxResponseBytes = 8 * 1024 * 1024;
constexpr int maxCanvasCards = 256;

QJsonObject edit(const QString &operation, const QJsonArray &path, const QJsonValue &value = {})
{
    return {{"operation", operation}, {"path", path}, {"value", value}};
}

QString shellQuote(QString value)
{
    value.replace("'", "'\\''");
    return "'" + value + "'";
}
}

WorkflowEditorWidget::WorkflowEditorWidget(const QString &workdir, const QString &binary, QWidget *parent)
    : QWidget(parent), m_workdir(workdir), m_binary(binary)
{
    using Symbol = CanvasIcons::Symbol;
    setObjectName("workflowStudio");
    setWindowTitle(tr("Workflow studio"));
    setMinimumSize(640, 320);
    // The document owns the full viewport. Controls are independent overlays.
    m_tabs = new QTabWidget(this);
    m_tabs->setObjectName("canvasDocument");
    m_tabs->setDocumentMode(true);
    m_tabs->tabBar()->hide();
    m_canvas = new WorkflowCanvas;
    m_canvas->setObjectName("workflowCanvas");
    m_yaml = new SourceEditor;
    static_cast<SourceEditor *>(m_yaml)->setFileName("config.yml");
    m_yaml->setObjectName("workflowYaml");
    m_yaml->setFont(QFontDatabase::systemFont(QFontDatabase::FixedFont));
    m_tabs->addTab(m_canvas, tr("Canvas"));
    m_tabs->addTab(m_yaml, tr("YAML"));
    m_files = new ProjectFilesWidget;
    m_tabs->addTab(m_files, tr("Files"));
    connect(m_files, &ProjectFilesWidget::workflowSaveRequested, this, [this]() { saveWorkflow(false); });
    connect(m_files, &ProjectFilesWidget::workflowOpenRequested, this, [this](const QString &path) {
        if (path == m_path) {
            m_files->closeGraph();
            m_tabs->setCurrentIndex(0);
        } else {
            openWorkflow(path);
        }
    });

    auto floatingPanel = [this](const QString &name) {
        auto *panel = new QFrame(this);
        panel->setObjectName(name);
        panel->setProperty("canvasOverlay", true);
        auto *shadow = new QGraphicsDropShadowEffect(panel);
        shadow->setBlurRadius(16);
        shadow->setOffset(0, 3);
        shadow->setColor(QColor(0, 0, 0, 45));
        panel->setGraphicsEffect(shadow);
        return panel;
    };
    m_actions = floatingPanel("canvasDocumentControls");
    auto *actions = new QHBoxLayout(m_actions);
    actions->setContentsMargins(5, 5, 5, 5);
    actions->setSpacing(4);
    m_workspacePicker = new QPushButton(tr("Workspace"));
    m_workspacePicker->setObjectName("canvasWorkspacePicker");
    m_workspacePicker->setMaximumWidth(160);
    m_workspacePicker->setIcon(CanvasIcons::icon(Symbol::Folder));
    actions->addWidget(m_workspacePicker);
    auto *browse = new QPushButton(tr("Workflows"));
    browse->setObjectName("workflowPickerButton");
    browse->setIcon(CanvasIcons::icon(Symbol::Workflow));
    browse->setToolTip(tr("Open a workflow (Ctrl+P / ⌘P)"));
    actions->addWidget(browse);
    m_filesToggle = new QPushButton(CanvasIcons::icon(Symbol::Folder), tr("Files"));
    m_filesToggle->setObjectName("workflowFilesToggle");
    m_filesToggle->setCheckable(true);
    actions->addWidget(m_filesToggle);
    connect(m_filesToggle, &QPushButton::clicked, this, [this]() {
        m_tabs->setCurrentIndex(m_tabs->currentIndex() == 2 ? 0 : 2);
    });
    connect(browse, &QPushButton::clicked, this, &WorkflowEditorWidget::browseRequested);
    m_filename = new QLabel(tr("Untitled"));
    m_filename->setObjectName("studioFilename");
    m_filename->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    actions->addWidget(m_filename, 1);
    auto *more = new QPushButton;
    more->setIcon(CanvasIcons::icon(Symbol::More));
    more->setObjectName("canvasMoreButton");
    more->setAccessibleName(tr("Workflow actions"));
    more->setFixedWidth(32);
    auto *menu = new QMenu(more);
    menu->addAction(CanvasIcons::icon(Symbol::Add), tr("New workflow"), this, [this]() {
        if (confirmDiscardChanges())
            newWorkflow();
    });
    menu->addAction(CanvasIcons::icon(Symbol::Check), tr("Validate workflow"), this, [this]() {
        if (!requireAppliedInspector())
            return;
        const QString directory = m_path.isEmpty() ? m_workdir : QFileInfo(m_path).absolutePath();
        validateSource(directory, [this]() { showStatus(tr("Workflow is valid. Tool availability is checked when you run it.")); });
    });
    menu->addSeparator();
    menu->addAction(CanvasIcons::icon(Symbol::Fit), tr("Reset panel layout"), this, [this]() {
        static_cast<CanvasPanel *>(m_inspector)->resetPlacement();
        if (m_consolePanel)
            static_cast<CanvasPanel *>(m_consolePanel)->resetPlacement();
        if (m_activityPanel)
            static_cast<CanvasPanel *>(m_activityPanel)->resetPlacement();
        arrangeOverlays();
    });
    more->setMenu(menu);
    actions->addWidget(more);

    m_palette = floatingPanel("canvasRunControls");
    auto *palette = new QHBoxLayout(m_palette);
    palette->setContentsMargins(5, 5, 5, 5);
    palette->setSpacing(4);
    auto *add = new QPushButton(tr("Add"));
    add->setObjectName("canvasAddButton");
    add->setIcon(CanvasIcons::icon(Symbol::Add));
    auto *addMenu = new QMenu(add);
    for (const QString &type : {QStringLiteral("Command"), QStringLiteral("Script"), QStringLiteral("Workflow"), QStringLiteral("Parallel")}) {
        Symbol icon = Symbol::Terminal;
        if (type == "Script")
            icon = Symbol::Script;
        else if (type == "Workflow")
            icon = Symbol::Workflow;
        else if (type == "Parallel")
            icon = Symbol::Parallel;
        auto *item = addMenu->addAction(CanvasIcons::icon(icon), type);
        item->setObjectName("add" + type);
        connect(item, &QAction::triggered, this, [this, type]() { addStep(type); });
    }
    add->setMenu(addMenu);
    palette->addWidget(add);
    auto *save = new QPushButton(tr("Save"));
    save->setObjectName("canvasSaveButton");
    save->setIcon(CanvasIcons::icon(Symbol::Save));
    auto *saveShortcut = new QShortcut(QKeySequence::Save, m_tabs);
    saveShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(m_tabs, &QTabWidget::currentChanged, saveShortcut, [saveShortcut](int index) { saveShortcut->setEnabled(index != 2); });
    connect(saveShortcut, &QShortcut::activated, this, [this]() {
        if (m_tabs->currentIndex() != 2)
            saveWorkflow(false);
    });
    save->setToolTip(tr("Save workflow (%1)").arg(QKeySequence(QKeySequence::Save).toString(QKeySequence::NativeText)));
    connect(save, &QPushButton::clicked, this, [this]() {
        if (m_tabs->currentIndex() == 2)
            m_files->saveFile();
        else
            saveWorkflow(false);
    });
    palette->addWidget(save);
    auto *run = new QPushButton(tr("Run"));
    run->setObjectName("primaryButton");
    run->setIcon(CanvasIcons::icon(Symbol::Play));
    run->setToolTip(tr("Validate, save, and run this workflow"));
    connect(run, &QPushButton::clicked, this, [this]() { saveWorkflow(true); });
    palette->addWidget(run);

    m_viewControls = floatingPanel("canvasViewControls");
    auto *viewControls = new QVBoxLayout(m_viewControls);
    viewControls->setContentsMargins(4, 4, 4, 4);
    viewControls->setSpacing(2);
    auto viewButton = [viewControls](Symbol symbol, const QString &name, const QString &tip) {
        auto *button = new QPushButton;
        button->setIcon(CanvasIcons::icon(symbol));
        button->setIconSize(QSize(18, 18));
        button->setObjectName(name);
        button->setToolTip(tip);
        button->setAccessibleName(tip);
        button->setFixedSize(36, 36);
        viewControls->addWidget(button);
        return button;
    };
    connect(viewButton(Symbol::Add, "canvasZoomIn", tr("Zoom in")), &QPushButton::clicked, this, [this]() { m_canvas->zoomBy(1.2); });
    auto *zoomLevel = new QPushButton("100%");
    zoomLevel->setObjectName("canvasZoomLevel");
    zoomLevel->setFixedSize(36, 26);
    zoomLevel->setToolTip(tr("Reset zoom to 100%"));
    viewControls->addWidget(zoomLevel);
    connect(zoomLevel, &QPushButton::clicked, this, [this]() {
        m_canvas->zoomBy(1.0 / m_canvas->transform().m11());
    });
    connect(m_canvas, &WorkflowCanvas::zoomChanged, zoomLevel, [zoomLevel](int percent) {
        zoomLevel->setText(QString::number(percent) + "%");
    });
    connect(viewButton(Symbol::Minus, "canvasZoomOut", tr("Zoom out")), &QPushButton::clicked, this, [this]() { m_canvas->zoomBy(1.0 / 1.2); });
    connect(viewButton(Symbol::Fit, "canvasFit", tr("Fit canvas")), &QPushButton::clicked, this, [this]() { m_canvas->fitDocument(); });
    connect(viewButton(Symbol::Undo, "canvasUndo", tr("Undo canvas edit")), &QPushButton::clicked, this, [this]() {
        if (!requireAppliedInspector() || m_history.isEmpty())
            return;
        m_yaml->setPlainText(m_history.takeLast());
        refreshCanvas();
    });
    auto *divider = new QFrame;
    divider->setObjectName("canvasControlDivider");
    divider->setFixedHeight(1);
    viewControls->addWidget(divider);
    m_yamlToggle = viewButton(Symbol::Code, "canvasYamlToggle", tr("Toggle YAML source"));
    m_yamlToggle->setCheckable(true);
    connect(m_yamlToggle, &QPushButton::clicked, this, [this]() {
        m_tabs->setCurrentIndex(m_tabs->currentIndex() == 0 ? 1 : 0);
    });
    m_properties = viewButton(Symbol::Properties, "workflowPropertiesButton", tr("Properties"));
    m_properties->setCheckable(true);
    connect(m_properties, &QPushButton::toggled, this, [this](bool visible) {
        m_inspector->setVisible(visible);
        if (visible)
            m_inspector->raise();
        arrangeOverlays();
    });

    auto *scroll = new QScrollArea;
    scroll->setObjectName("studioInspectorScroll");
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    m_inspector = new CanvasPanel("properties", this);
    m_inspector->setObjectName("studioSidebar");
    m_inspector->setMinimumWidth(280);

    auto *sidebarLayout = new QVBoxLayout(m_inspector);
    sidebarLayout->setContentsMargins(12, 10, 12, 12);
    auto *inspectorTitleBar = new QWidget;
    inspectorTitleBar->setObjectName("canvasPropertiesTitleBar");
    auto *inspectorHeader = new QHBoxLayout(inspectorTitleBar);
    inspectorHeader->setContentsMargins(0, 0, 0, 0);
    static_cast<CanvasPanel *>(m_inspector)->setDragHandle(inspectorTitleBar);
    inspectorHeader->addWidget(new QLabel(tr("Properties")), 1);
    auto *closeInspector = new QPushButton;
    closeInspector->setIcon(CanvasIcons::icon(Symbol::Close));
    closeInspector->setObjectName("canvasCloseProperties");
    closeInspector->setFixedSize(28, 28);
    closeInspector->setAccessibleName(tr("Close properties"));
    connect(closeInspector, &QPushButton::clicked, this, [this]() { m_properties->setChecked(false); });
    inspectorHeader->addWidget(closeInspector);
    sidebarLayout->addWidget(inspectorTitleBar);
    auto *settingsWidget = new QWidget;
    settingsWidget->setObjectName("studioInspector");
    auto *inspector = new QVBoxLayout(settingsWidget);
    inspector->setContentsMargins(16, 12, 4, 12);
    auto *workflowTitle = new QLabel(tr("WORKFLOW"));
    workflowTitle->setObjectName("sectionTitle");
    inspector->addWidget(workflowTitle);
    auto *form = new QFormLayout;
    form->setRowWrapPolicy(QFormLayout::WrapAllRows);
    auto line = [form](const QString &label, const QString &name) {
        auto *field = new QLineEdit;
        field->setObjectName(name);
        form->addRow(label, field);
        return field;
    };
    m_name = line(tr("Name"), "workflowName");
    m_description = line(tr("Description"), "workflowDescription");
    inspector->addLayout(form);
    m_selectionTitle = new QLabel(tr("Select a step"));
    m_selectionTitle->setObjectName("sectionTitle");
    inspector->addWidget(m_selectionTitle);
    auto *stepForm = new QFormLayout;
    m_stepForm = stepForm;
    stepForm->setRowWrapPolicy(QFormLayout::WrapAllRows);
    // Build the step fields separately so labels remain above controls at narrow widths.
    auto stepLine = [stepForm](const QString &label, const QString &name) {
        auto *field = new QLineEdit;
        field->setObjectName(name);
        stepForm->addRow(label, field);
        return field;
    };
    m_id = stepLine(tr("Step name"), "stepId");
    m_kind = new QComboBox;
    m_kind->addItem(tr("This host"), "host");
    m_kind->addItem(tr("Container"), "container");
    stepForm->addRow(tr("Run in"), m_kind);
    connect(m_kind, qOverload<int>(&QComboBox::activated), this, [this](int) {
        const bool container = m_kind->currentData().toString() == "container";
        m_runtime->setEnabled(container);
        m_resolver->setEnabled(container);
        updateStepVisibility();
    });
    m_command = new QPlainTextEdit;
    m_command->setObjectName("stepCommand");
    m_command->setMaximumHeight(95);
    m_command->setPlaceholderText(tr("Command or application to run"));
    stepForm->addRow(tr("Command"), m_command);
    m_cwd = stepLine(tr("Working directory"), "stepCwd");
    m_cwd->setPlaceholderText(tr("repo or artifacts (default: repo)"));
    m_cwd->setToolTip(tr("repo uses the selected workspace; artifacts uses this workflow's output directory. "
                        "For a subdirectory, use cd in the command. File paths are not accepted here."));
    m_runtime = stepLine(tr("Runtime profile (container)"), "stepRuntime");
    m_resolver = stepLine(tr("Tool integration (container)"), "stepResolver");
    m_package = stepLine(tr("Package namespace"), "stepPackage");
    m_workflow = stepLine(tr("Packaged workflow"), "stepWorkflow");
    m_outputs = stepLine(tr("Output environment file"), "stepOutputs");
    m_vars = new QPlainTextEdit;
    m_vars->setObjectName("stepVars");
    m_vars->setMaximumHeight(100);
    m_vars->setPlaceholderText(tr("NAME=value\nUse references for secrets."));
    stepForm->addRow(tr("Environment"), m_vars);
    inspector->addLayout(stepForm);
    scroll->setWidget(settingsWidget);
    sidebarLayout->addWidget(scroll, 1);
    auto *apply = new QPushButton(tr("Apply changes"));
    apply->setObjectName("applyStep");
    sidebarLayout->addWidget(apply);
    connect(apply, &QPushButton::clicked, this, &WorkflowEditorWidget::applyInspector);
    auto *order = new QHBoxLayout;
    for (int direction : {-1, 1}) {
        auto *button = new QPushButton(direction < 0 ? tr("Move earlier") : tr("Move later"));
        order->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, direction]() { moveStep(direction); });
    }
    sidebarLayout->addLayout(order);
    auto *remove = new QPushButton(tr("Remove step"));
    sidebarLayout->addWidget(remove);
    connect(remove, &QPushButton::clicked, this, [this]() {
        if (requireAppliedInspector() && !m_selection.isEmpty())
            refreshCanvas({edit("remove", m_selection)});
    });
    auto *hint = new QLabel(tr("Connections show execution order. Drag cards to arrange the canvas; use Move earlier/later to reorder. Select a parallel group to add tasks to it."));
    hint->setObjectName("hintText");
    hint->setWordWrap(true);
    inspector->addWidget(hint);
    inspector->addStretch();
    m_inspector->hide();
    m_notice = floatingPanel("canvasNotice");
    auto *notice = new QHBoxLayout(m_notice);
    notice->setContentsMargins(12, 8, 12, 8);
    m_status = new QLabel;
    m_status->setWordWrap(true);
    m_status->setTextInteractionFlags(Qt::TextSelectableByMouse);
    notice->addWidget(m_status, 1);
    m_cancel = new QPushButton(tr("Cancel operation"));
    m_cancel->hide();
    notice->addWidget(m_cancel);
    m_noticeTimer = new QTimer(this);
    m_noticeTimer->setSingleShot(true);
    connect(m_noticeTimer, &QTimer::timeout, m_notice, &QWidget::hide);
    m_notice->hide();
    m_process = new QProcess(this);
    m_deadline = new QTimer(this);
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this]() {
        m_processFailure = tr("Operation timed out. Your draft is unchanged.");
        m_process->kill();
    });
    connect(m_cancel, &QPushButton::clicked, this, [this]() {
        m_processFailure = tr("Operation cancelled. Your draft is unchanged.");
        m_process->kill();
    });
    auto readOutput = [this]() {
        m_output += m_process->readAllStandardOutput();
        m_errors += m_process->readAllStandardError();
        if (m_output.size() + m_errors.size() > maxResponseBytes) {
            m_processFailure = tr("CLI output exceeded the editor's 8 MiB response limit.");
            m_output.clear();
            m_errors.clear();
            m_process->kill();
        }
    };
    connect(m_process, &QProcess::readyReadStandardOutput, this, readOutput);
    connect(m_process, &QProcess::readyReadStandardError, this, readOutput);
    connect(m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this, readOutput](int, QProcess::ExitStatus) { readOutput(); finishCli(); });
    connect(m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_processFailure = tr("Cannot start Dockpipe: %1").arg(m_process->errorString());
            finishCli();
        }
    });
    connect(m_canvas, &WorkflowCanvas::nodeSelected, this, &WorkflowEditorWidget::selectNode);
    connect(m_tabs, &QTabWidget::currentChanged, this, [this](int index) {
        if (!m_refreshing && !requireAppliedInspector()) {
            QSignalBlocker blocker(m_tabs);
            m_tabs->setCurrentIndex(0);
            m_yamlToggle->setChecked(false);
            m_filesToggle->setChecked(false);
            return;
        }
        if (index == 0 && !m_refreshing)
            refreshCanvas();
        m_yamlToggle->setChecked(index == 1);
        m_filesToggle->setChecked(index == 2);
        m_viewControls->setVisible(index != 2);
        m_palette->findChild<QPushButton *>("canvasAddButton")->setVisible(index != 2);
        m_palette->findChild<QPushButton *>("canvasSaveButton")->setVisible(index != 2);
        if (index == 2)
            m_properties->setChecked(false);
        arrangeOverlays();
        m_palette->findChild<QPushButton *>("canvasAddButton")->setEnabled(index == 0 && !m_busy && m_canvasReady);
        m_inspector->setEnabled(index == 0 && !m_busy && m_canvasReady);
    });
    connect(m_yaml, &QPlainTextEdit::textChanged, this, &WorkflowEditorWidget::updateDocumentTitle);
    // Enter in a property field must never activate New or Save and run.
    for (auto *button : findChildren<QPushButton *>())
        button->setAutoDefault(false);
    newWorkflow();
}

WorkflowEditorWidget::~WorkflowEditorWidget()
{
    // Scene teardown emits selection changes while QWidget deletes children.
    // Disconnect before our document/inspector state members are destroyed.
    for (auto *child : findChildren<QObject *>())
        QObject::disconnect(child, nullptr, this, nullptr);
    if (m_process->state() != QProcess::NotRunning) {
        m_process->disconnect(this);
        m_process->kill();
        m_process->waitForFinished(1000);
    }
    delete m_validationFile;
}

void WorkflowEditorWidget::updateDocumentTitle()
{
    QString title = tr("Untitled");
    if (!m_path.isEmpty()) {
        title = workflowName();
        if (title.isEmpty())
            title = QFileInfo(m_path).completeBaseName();
    }
    if (m_yaml->toPlainText() != m_savedSource)
        title += " •";
    m_filename->setText(title);
    m_filename->setToolTip(m_path);
}

void WorkflowEditorWidget::showStatus(const QString &message, bool failure)
{
    m_status->setText(message.size() > 1800 ? message.left(1800) + tr("\n[Diagnostics truncated]") : message);
    m_status->setStyleSheet(failure ? "color: #e79595" : "");
    m_status->setToolTip(m_status->text());
    m_noticeTimer->stop();
    m_notice->show();
    if (!failure && !m_busy)
        m_noticeTimer->start(4500);
    arrangeOverlays();
}

void WorkflowEditorWidget::setBusy(bool busy)
{
    m_busy = busy;
    m_actions->setEnabled(!busy);
    m_tabs->setEnabled(!busy);
    m_palette->setEnabled(!busy);
    m_viewControls->setEnabled(!busy);
    m_palette->findChild<QPushButton *>("canvasAddButton")->setEnabled(!busy && m_tabs->currentIndex() == 0 && m_canvasReady);
    m_inspector->setEnabled(!busy && m_tabs->currentIndex() == 0 && m_canvasReady);
    m_cancel->setVisible(busy);
    m_viewControls->findChild<QPushButton *>("canvasUndo")->setEnabled(!busy && !m_history.isEmpty());
}

void WorkflowEditorWidget::startCli(const QStringList &arguments, const QByteArray &input,
                                   const std::function<void(const QByteArray &)> &success)
{
    if (m_busy)
        return;
    m_success = success;
    m_output.clear();
    m_errors.clear();
    m_processFailure.clear();
    setBusy(true);
    showStatus(tr("Checking workflow…"));
    QProcessEnvironment environment = QProcessEnvironment::systemEnvironment();
    environment.insert("DOCKPIPE_WORKDIR", QDir::cleanPath(m_workdir));
    m_process->setProcessEnvironment(environment);
    m_process->setWorkingDirectory(m_workdir.isEmpty() ? QDir::homePath() : m_workdir);
    m_process->start(m_binary, arguments);
    m_process->write(input);
    m_process->closeWriteChannel();
    m_deadline->start(30000);
}

void WorkflowEditorWidget::finishCli()
{
    if (!m_busy)
        return;
    m_deadline->stop();
    setBusy(false);
    const auto success = m_success;
    m_success = {};
    if (m_validationFile) {
        if (!m_validationFile->remove())
            m_processFailure = tr("Cannot remove the temporary validation file: %1").arg(m_validationFile->fileName());
        delete m_validationFile;
        m_validationFile = nullptr;
    }
    const bool failed = !m_processFailure.isEmpty() || m_process->exitStatus() != QProcess::NormalExit || m_process->exitCode() != 0;
    if (failed && m_process->arguments() == QStringList{"workflow", "document"}) {
        m_canvasReady = m_editPending;
        if (!m_editPending) {
            m_inspectorBaseline.clear();
            m_refreshing = true;
            m_tabs->setCurrentIndex(1);
            m_refreshing = false;
        }
        setBusy(false);
    }
    if (!m_processFailure.isEmpty()) {
        showStatus(m_processFailure, true);
    } else if (m_process->exitStatus() != QProcess::NormalExit || m_process->exitCode() != 0) {
        showStatus(tr("%1\nIf this Dockpipe does not support the editor, update the CLI and launcher together.")
                   .arg(QString::fromUtf8(m_errors).trimmed()), true);
    } else if (success) {
        success(m_output);
    }
}

void WorkflowEditorWidget::newWorkflow()
{
    m_inspectorBaseline.clear();
    m_path.clear();
    m_document = {};
    m_diskBytes.clear();
    m_history.clear();
    m_canvas->setDocument({});
    m_filename->setText(tr("Untitled"));
    m_properties->setChecked(false);
    m_savedSource = "name: my-workflow\ndescription: \"\"\ndocker_preflight: false\nsteps: []\n";
    m_yaml->setPlainText(m_savedSource);
    m_files->setRoot({});
    m_files->setWorkflowDocument({}, m_yaml->document());
    m_refreshing = true;
    m_tabs->setCurrentIndex(0);
    m_refreshing = false;
    emit documentChanged(QString());
    // Let construction/open finish before the first asynchronous CLI operation.
    QTimer::singleShot(0, this, [this]() {
        if (!m_busy)
            refreshCanvas();
    });
}

bool WorkflowEditorWidget::openWorkflow(const QString &path)
{
    if (m_busy)
        return false;
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly) || file.size() > maxSourceBytes) {
        showStatus(tr("Cannot read workflow, or it exceeds the 1 MiB editor limit."), true);
        return false;
    }
    const QByteArray bytes = file.read(maxSourceBytes + 1);
    const QString source = QString::fromUtf8(bytes);
    if (source.toUtf8() != bytes) {
        showStatus(tr("Workflow must be UTF-8. The file was not changed."), true);
        return false;
    }
    if (!confirmDiscardChanges())
        return false;
    m_inspectorBaseline.clear();
    m_properties->setChecked(false);
    m_refreshing = true;
    m_tabs->setCurrentIndex(0);
    m_refreshing = false;
    m_restoreLayout = true;
    m_path = QFileInfo(path).absoluteFilePath();
    m_document = {};
    m_canvas->setDocument({});
    m_diskBytes = bytes;
    m_savedSource = source;
    m_history.clear();
    m_yaml->setPlainText(source);
    m_files->setRoot(QFileInfo(m_path).absolutePath());
    m_files->setWorkflowDocument(m_path, m_yaml->document());
    updateDocumentTitle();
    emit documentChanged(m_path);
    refreshCanvas();
    return true;
}

void WorkflowEditorWidget::rememberSource(const QString &source)
{
    m_history.append(source);
    qsizetype bytes = 0;
    for (const auto &entry : m_history)
        bytes += entry.size() * 2;
    while (m_history.size() > 1 && (bytes > 8 * maxSourceBytes || m_history.size() > 50)) {
        bytes -= m_history.first().size() * 2;
        m_history.removeFirst();
    }
}

void WorkflowEditorWidget::refreshCanvas(const QJsonArray &edits)
{
    const QString source = m_yaml->toPlainText();
    if (source.toUtf8().size() > maxSourceBytes) {
        showStatus(tr("Workflow exceeds the 1 MiB editor limit."), true);
        return;
    }
    m_editPending = !edits.isEmpty();
    m_canvasReady = false;
    const QJsonObject request{{"source", source}, {"edits", edits}};
    startCli({"workflow", "document"}, QJsonDocument(request).toJson(QJsonDocument::Compact),
             [this, source, edits](const QByteArray &output) {
        QJsonParseError error;
        const QJsonObject response = QJsonDocument::fromJson(output, &error).object();
        if (error.error != QJsonParseError::NoError || !response.value("document").isObject() || !response.value("source").isString()) {
            showStatus(tr("Dockpipe returned an invalid document response."), true);
            return;
        }
        const QJsonObject document = response.value("document").toObject();
        const QJsonArray steps = document.value("steps").toArray();
        int cards = steps.size();
        bool supportedShape = document.value("steps").isArray();
        for (const auto &step : steps) {
            supportedShape = supportedShape && step.isObject();
            const QJsonObject item = step.toObject();
            if (!item.contains("group"))
                continue;
            const QJsonObject group = item.value("group").toObject();
            const QJsonArray tasks = group.value("tasks").toArray();
            supportedShape = supportedShape && group.value("mode").toString() == "async" && group.value("tasks").isArray();
            for (const auto &task : tasks)
                supportedShape = supportedShape && task.isObject() && !task.toObject().contains("group");
            cards += tasks.size();
        }
        if (!supportedShape || cards > maxCanvasCards) {
            m_inspectorBaseline.clear();
            m_document = {};
            m_canvas->setDocument({});
            m_refreshing = true;
            m_tabs->setCurrentIndex(1);
            m_refreshing = false;
            showStatus(tr("This document uses a shape outside the first canvas editor, or exceeds 256 cards. Edit and validate it in the YAML tab; its contents are preserved."));
            return;
        }
        if (!edits.isEmpty())
            rememberSource(source);
        m_inspectorBaseline.clear();
        m_canvasReady = true;
        setBusy(false);
        m_document = document;
        updateDocumentTitle();
        m_yaml->setPlainText(response.value("source").toString());
        m_canvas->setDocument(document);
        QSettings settings;
        if (m_restoreLayout) {
            const QJsonObject layout = QJsonDocument::fromJson(settings.value(layoutKey()).toByteArray()).object();
            const QString digest = QString::fromLatin1(QCryptographicHash::hash(m_yaml->toPlainText().toUtf8(), QCryptographicHash::Sha256).toHex());
            if (layout.value("sourceHash").toString() == digest)
                m_canvas->restorePositions(layout.value("positions").toObject());
            m_restoreLayout = false;
        }
        m_name->setText(document.value("name").toString());
        m_description->setText(document.value("description").toString());
        m_refreshing = true;
        selectNode({});
        m_refreshing = false;
        showStatus(tr("%1 steps · Scroll to zoom · Drag to pan").arg(steps.size()));
        m_notice->hide();
    });
}

QJsonValue WorkflowEditorWidget::selectedValue() const
{
    QJsonValue value(m_document);
    for (const auto &part : m_selection) {
        if (value.isArray())
            value = value.toArray().at(part.toString().toInt());
        else
            value = value.toObject().value(part.toString());
    }
    return value;
}

void WorkflowEditorWidget::selectNode(const QJsonArray &path)
{
    if (!m_refreshing && !requireAppliedInspector()) {
        m_canvas->selectPath(m_selection);
        return;
    }
    m_selection = path;
    if (!path.isEmpty())
        m_properties->setChecked(true);
    const QJsonObject step = path.isEmpty() ? QJsonObject() : selectedValue().toObject();
    const bool ordinary = !path.isEmpty() && !step.contains("group");
    const bool packaged = step.contains("workflow");
    QString selectionTitle = tr("STEP SETTINGS");
    if (path.isEmpty())
        selectionTitle = tr("Select a step");
    else if (step.contains("group"))
        selectionTitle = tr("Parallel group");
    m_selectionTitle->setText(selectionTitle);
    const QList<QWidget *> fields{m_id, m_kind, m_command, m_cwd, m_runtime, m_resolver, m_package, m_workflow, m_outputs, m_vars};
    for (auto *field : fields)
        field->setEnabled(ordinary);
    m_kind->setEnabled(ordinary && !packaged);
    m_command->setEnabled(ordinary && !packaged);
    m_runtime->setEnabled(ordinary && !packaged && m_kind->currentData().toString() != "host");
    m_resolver->setEnabled(ordinary && !packaged && m_kind->currentData().toString() != "host");
    m_package->setEnabled(ordinary && packaged);
    m_workflow->setEnabled(ordinary && packaged);
    m_id->setText(step.value("id").toString());
    m_kind->setCurrentIndex(step.value("kind").toString() == "host" ? 0 : 1);
    m_runtime->setEnabled(ordinary && !packaged && m_kind->currentIndex() == 1);
    m_resolver->setEnabled(ordinary && !packaged && m_kind->currentIndex() == 1);
    m_command->setPlainText(step.value("cmd").toString(step.value("command").toString()));
    m_cwd->setText(step.value("cwd").toString());
    m_runtime->setText(step.value("runtime").toString());
    m_resolver->setText(step.value("resolver").toString());
    m_package->setText(step.value("package").toString());
    m_workflow->setText(step.value("workflow").toString());
    m_outputs->setText(step.value("outputs").toString());
    QStringList variables;
    const QJsonObject vars = step.value("vars").toObject();
    bool simpleVariables = true;
    for (auto it = vars.begin(); it != vars.end(); ++it) {
        simpleVariables = simpleVariables && it.value().isString() && !it.value().toString().contains('\n');
        variables.append(it.key() + "=" + it.value().toString());
    }
    m_vars->setEnabled(ordinary && simpleVariables);
    if (!simpleVariables)
        variables = {tr("Edit structured or multiline environment values in YAML.")};
    m_vars->setPlainText(variables.join('\n'));
    updateStepVisibility();
    m_inspectorBaseline = inspectorState();
}

void WorkflowEditorWidget::addStep(const QString &type)
{
    if (!requireAppliedInspector() || !m_document.value("steps").isArray())
        return;
    QJsonArray path{QStringLiteral("steps")};
    if (!m_selection.isEmpty() && selectedValue().toObject().contains("group")) {
        if (type == "Parallel") {
            showStatus(tr("Parallel groups cannot be nested."), true);
            return;
        }
        path = m_selection;
        path.append("group");
        path.append("tasks");
    }
    QSet<QString> identifiers;
    for (const auto &entry : m_document.value("steps").toArray()) {
        const QJsonObject item = entry.toObject();
        identifiers.insert(item.value("id").toString());
        for (const auto &task : item.value("group").toObject().value("tasks").toArray())
            identifiers.insert(task.toObject().value("id").toString());
    }
    int number = 1;
    while (identifiers.contains("step-" + QString::number(number)))
        ++number;
    const QString identifier = "step-" + QString::number(number);
    QJsonObject step{{"id", identifier}, {"kind", "host"}, {"cmd", "echo ready"}, {"cwd", "repo"}};
    if (type == "Script") {
        // Browse the granted workspace directly in Flatpak, so an older portal
        // cannot turn every selected script into an unrelated document mount.
        QFileDialog::Options options;
        if (qEnvironmentVariableIsSet("FLATPAK_ID") || QFile::exists("/.flatpak-info"))
            options |= QFileDialog::DontUseNativeDialog;
        const QString script = QFileDialog::getOpenFileName(this, tr("Choose a script"), m_workdir, {}, nullptr, options);
        if (script.isEmpty())
            return;
        const auto resolved = WorkflowScriptPath::resolve(m_workdir, script);
        if (!resolved.error.isEmpty()) {
            showStatus(resolved.error, true);
            return;
        }
        const QString relative = resolved.relativePath;
        const QString suffix = QFileInfo(script).suffix().toLower();
        QString interpreter = "sh";
        if (suffix == "py")
            interpreter = "python3";
        else if (suffix == "js")
            interpreter = "node";
        else if (suffix == "ps1")
            interpreter = "pwsh -File";
        step["cmd"] = interpreter + " " + shellQuote(relative);
        step["id"] = QFileInfo(script).completeBaseName();
    } else if (type == "Workflow") {
        step = {{"id", identifier}, {"package", "my-package"}, {"workflow", "my-workflow"}};
    } else if (type == "Parallel") {
        step = {{"group", QJsonObject{{"mode", "async"}, {"tasks", QJsonArray{step}}}}};
    }
    refreshCanvas({edit("append", path, step)});
}

void WorkflowEditorWidget::applyInspector()
{
    QJsonArray edits;
    auto change = [&edits](const QJsonArray &base, const QJsonObject &object, const QString &key, const QString &value) {
        if (object.value(key).toString() == value)
            return;
        QJsonArray path = base;
        path.append(key);
        edits.append(edit(value.isEmpty() ? "remove" : "set", path, value));
    };
    change({}, m_document, "name", m_name->text());
    change({}, m_document, "description", m_description->text());
    if (!m_selection.isEmpty()) {
        const QJsonObject step = selectedValue().toObject();
        if (!step.contains("group")) {
            change(m_selection, step, "id", m_id->text());
            change(m_selection, step, "cwd", m_cwd->text());
            change(m_selection, step, "outputs", m_outputs->text());
            if (step.contains("workflow")) {
                change(m_selection, step, "package", m_package->text());
                change(m_selection, step, "workflow", m_workflow->text());
            } else {
                change(m_selection, step, "kind", m_kind->currentData().toString());
                change(m_selection, step, step.contains("command") ? "command" : "cmd", m_command->toPlainText());
                change(m_selection, step, "runtime", m_kind->currentIndex() == 0 ? QString() : m_runtime->text());
                change(m_selection, step, "resolver", m_kind->currentIndex() == 0 ? QString() : m_resolver->text());
            }
            QJsonObject variables;
            for (const auto &line : (m_vars->isEnabled() ? m_vars->toPlainText() : QString()).split('\n')) {
                if (line.trimmed().isEmpty())
                    continue;
                const int separator = line.indexOf('=');
                const QString key = line.left(separator);
                if (separator <= 0 || !QRegularExpression("^[A-Za-z_][A-Za-z0-9_]*$").match(key).hasMatch() || variables.contains(key)) {
                    showStatus(tr("Environment entries must be unique NAME=value lines."), true);
                    return;
                }
                variables.insert(key, line.mid(separator+1));
            }
            if (m_vars->isEnabled() && variables != step.value("vars").toObject()) {
                QJsonArray path = m_selection;
                path.append("vars");
                const QJsonObject oldVariables = step.value("vars").toObject();
                if (!step.contains("vars")) {
                    edits.append(edit("set", path, variables));
                } else {
                    for (auto it = oldVariables.begin(); it != oldVariables.end(); ++it) {
                        if (!variables.contains(it.key())) {
                            QJsonArray keyPath = path;
                            keyPath.append(it.key());
                            edits.append(edit("remove", keyPath));
                        }
                    }
                    for (auto it = variables.begin(); it != variables.end(); ++it) {
                        if (oldVariables.value(it.key()) != it.value()) {
                            QJsonArray keyPath = path;
                            keyPath.append(it.key());
                            edits.append(edit("set", keyPath, it.value()));
                        }
                    }
                }
            }
        }
    }
    if (!edits.isEmpty())
        refreshCanvas(edits);
}

void WorkflowEditorWidget::moveStep(int direction)
{
    if (!requireAppliedInspector() || m_selection.isEmpty())
        return;
    const int index = m_selection.last().toString().toInt();
    QJsonValue sequence(m_document);
    for (int i = 0; i < m_selection.size()-1; ++i) {
        const QString part = m_selection[i].toString();
        sequence = sequence.isArray() ? sequence.toArray().at(part.toInt()) : sequence.toObject().value(part);
    }
    const int destination = index + direction;
    if (destination < 0 || destination >= sequence.toArray().size())
        return;
    QJsonObject operation = edit("move", m_selection);
    operation["destination"] = destination;
    refreshCanvas({operation});
}

QString WorkflowEditorWidget::layoutKey() const
{
    const QByteArray identity = m_path.toUtf8();
    return "workflowStudio/layout/" + QString::fromLatin1(QCryptographicHash::hash(identity, QCryptographicHash::Sha256).toHex());
}

void WorkflowEditorWidget::validateSource(const QString &directory, const std::function<void()> &success)
{
    const QByteArray source = m_yaml->toPlainText().toUtf8();
    if (source.size() > maxSourceBytes) {
        showStatus(tr("Workflow exceeds the 1 MiB editor limit."), true);
        return;
    }
    delete m_validationFile;
    m_validationFile = new QTemporaryFile(QDir(directory).filePath(".dockpipe-editor-XXXXXX.yml"));
    if (!m_validationFile->open() || m_validationFile->write(source) != source.size() || !m_validationFile->flush()) {
        showStatus(tr("Cannot create a temporary validation file alongside this workflow."), true);
        return;
    }
    m_validationFile->close();
    startCli({"workflow", "validate", m_validationFile->fileName()}, {}, [success](const QByteArray &) { success(); });
}

void WorkflowEditorWidget::saveWorkflow(bool run)
{
    if (run && m_files->hasUnsavedChanges() && !m_files->saveFile())
        return;
    if (!requireAppliedInspector())
        return;
    QString destination = m_path;
    if (destination.isEmpty()) {
        destination = QFileDialog::getSaveFileName(this, tr("Save workflow"), QDir(m_workdir).filePath("my-workflow.yml"), tr("Workflow YAML (*.yml *.yaml)"));
        if (destination.isEmpty())
            return;
    }
    if (QFileInfo(destination).isSymLink()) {
        showStatus(tr("Open the real workflow file instead of overwriting a symbolic link."), true);
        return;
    }
    QFile current(destination);
    QByteArray expected;
    const bool existed = current.exists();
    if (existed) {
        if (!current.open(QIODevice::ReadOnly) || current.size() > maxSourceBytes) {
            showStatus(tr("Cannot safely read the existing workflow."), true);
            return;
        }
        expected = current.read(maxSourceBytes + 1);
        if (expected.size() > maxSourceBytes) {
            showStatus(tr("The existing workflow grew beyond the editor limit. Your draft has been kept."), true);
            return;
        }
        if (destination == m_path && expected != m_diskBytes) {
            showStatus(tr("This workflow changed on disk. Open it again before saving; your draft has been kept."), true);
            return;
        }
    }
    const QByteArray source = m_yaml->toPlainText().toUtf8();
    validateSource(QFileInfo(destination).absolutePath(), [this, destination, source, expected, existed, run]() {
        QFile check(destination);
        if (QFileInfo(destination).isSymLink() || check.exists() != existed || (existed && (!check.open(QIODevice::ReadOnly) || check.read(maxSourceBytes+1) != expected))) {
            showStatus(tr("The file changed during validation. Your draft has been kept."), true);
            return;
        }
        QSaveFile file(destination);
        if (!file.open(QIODevice::WriteOnly) || file.write(source) != source.size() || !file.commit()) {
            showStatus(tr("Cannot save workflow: %1").arg(file.errorString()), true);
            return;
        }
        m_path = destination;
        m_files->setRoot(QFileInfo(destination).absolutePath());
        m_files->setWorkflowDocument(destination, m_yaml->document());
        m_diskBytes = source;
        m_savedSource = QString::fromUtf8(source);
        QSettings settings;
        const QString digest = QString::fromLatin1(QCryptographicHash::hash(source, QCryptographicHash::Sha256).toHex());
        settings.setValue(layoutKey(), QJsonDocument(QJsonObject{{"sourceHash", digest}, {"positions", m_canvas->positions()}}).toJson(QJsonDocument::Compact));
        updateDocumentTitle();
        setWindowTitle(tr("Workflow studio"));
        showStatus(tr("Workflow saved."));
        emit documentChanged(destination);
        emit workflowSaved(destination);
        if (run)
            emit runRequested(destination);
    });
}

bool WorkflowEditorWidget::discardChanges()
{
    if (m_yaml->toPlainText() == m_savedSource && (m_inspectorBaseline.isEmpty() || inspectorState() == m_inspectorBaseline))
        return true;
    return QMessageBox::question(this, tr("Unsaved workflow"), tr("Discard the unsaved changes to this workflow?"),
                                 QMessageBox::Discard | QMessageBox::Cancel, QMessageBox::Cancel) == QMessageBox::Discard;
}

bool WorkflowEditorWidget::confirmDiscardChanges()
{
    if (m_busy) {
        showStatus(tr("Cancel the current operation before switching workflows or closing."));
        return false;
    }
    return m_files->confirmLeave() && discardChanges();
}

bool WorkflowEditorWidget::setWorkspace(const QString &workdir, const QString &binary)
{
    if (m_workdir == workdir && m_binary == binary)
        return true;
    if (!confirmDiscardChanges())
        return false;
    m_workdir = workdir;
    m_binary = binary;
    newWorkflow();
    return true;
}

QString WorkflowEditorWidget::inspectorState() const
{
    const QJsonArray fields{m_name->text(), m_description->text(), m_id->text(), m_kind->currentData().toString(),
        m_command->toPlainText(), m_cwd->text(), m_runtime->text(), m_resolver->text(), m_package->text(),
        m_workflow->text(), m_outputs->text(), m_vars->toPlainText()};
    return QString::fromUtf8(QJsonDocument(fields).toJson(QJsonDocument::Compact));
}

bool WorkflowEditorWidget::requireAppliedInspector()
{
    if (m_inspectorBaseline.isEmpty() || inspectorState() == m_inspectorBaseline)
        return true;
    showStatus(tr("Apply your inspector changes before changing selection, switching views, or saving."), true);
    return false;
}

void WorkflowEditorWidget::updateStepVisibility()
{
    const QJsonObject step = m_selection.isEmpty() ? QJsonObject() : selectedValue().toObject();
    const bool ordinary = !m_selection.isEmpty() && !step.contains("group");
    const bool packaged = step.contains("workflow");
    const bool container = !packaged && m_kind->currentIndex() == 1;
    auto visible = [this](QWidget *field, bool show) {
#if QT_VERSION >= QT_VERSION_CHECK(6, 4, 0)
        m_stepForm->setRowVisible(field, show);
#else
        field->setVisible(show);
        if (QWidget *label = m_stepForm->labelForField(field))
            label->setVisible(show);
#endif
    };
    for (QWidget *field : QList<QWidget *>{m_id, m_cwd, m_outputs, m_vars})
        visible(field, ordinary);
    for (QWidget *field : QList<QWidget *>{m_kind, m_command})
        visible(field, ordinary && !packaged);
    for (QWidget *field : QList<QWidget *>{m_runtime, m_resolver})
        visible(field, ordinary && container);
    for (QWidget *field : QList<QWidget *>{m_package, m_workflow})
        visible(field, ordinary && packaged);
}

void WorkflowEditorWidget::setWorkspaceMenu(const QString &name, QMenu *menu)
{
    m_workspacePicker->setText(name);
    m_workspacePicker->setMenu(menu);
    arrangeOverlays();
}

void WorkflowEditorWidget::setConsoleWidgets(QWidget *panel, QWidget *controls)
{
    m_consolePanel = panel;
    m_consoleControls = controls;
    panel->setParent(this);
    controls->setParent(this);
    panel->setProperty("canvasOverlay", true);
    controls->setProperty("canvasOverlay", true);
    panel->hide();
    controls->show();
    arrangeOverlays();
}

void WorkflowEditorWidget::setConsoleVisible(bool visible)
{
    if (m_consolePanel)
        m_consolePanel->setVisible(visible);
    if (visible && m_consolePanel)
        m_consolePanel->raise();
    arrangeOverlays();
}

void WorkflowEditorWidget::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    arrangeOverlays();
}

void WorkflowEditorWidget::arrangeOverlays()
{
    if (!m_notice)
        return;
    const int margin = 16;
    m_tabs->setGeometry(m_tabs->currentIndex() == 2 ? rect().adjusted(0, 64, 0, -56) : rect());
    if (m_activityPanel)
        static_cast<CanvasPanel *>(m_activityPanel)->placeDefault(QRect(76, 90, qMin(850, width() - 108), height() - 180));
    m_palette->adjustSize();
    m_palette->move(width() - margin - m_palette->width(), margin);
    const int titleWidth = qMin(460, width() - m_palette->width() - 3 * margin);
    m_actions->setGeometry(margin, margin, titleWidth, m_actions->sizeHint().height());
    m_viewControls->adjustSize();
    m_viewControls->move(margin, height() - margin - m_viewControls->height());
    const int consoleHeight = qMin(260, qMax(200, height() / 3));
    const bool stackedPanels = width() < 800 && m_consolePanel && !m_consolePanel->isHidden();
    const int inspectorHeight = stackedPanels ? height() - consoleHeight - 172 : height() - 152;
    static_cast<CanvasPanel *>(m_inspector)->placeDefault(QRect(width() - 352, 80, 336, qMax(220, inspectorHeight)));
    if (m_consoleControls) {
        m_consoleControls->adjustSize();
        m_consoleControls->move(width() - margin - m_consoleControls->width(), height() - margin - m_consoleControls->height());
    }
    if (m_consolePanel) {
        const int inspectorSpace = m_inspector->isHidden() || stackedPanels ? 0 : 336;
        static_cast<CanvasPanel *>(m_consolePanel)->placeDefault(QRect(76, height() - consoleHeight - 76,
            qMax(260, width() - 108 - inspectorSpace), consoleHeight));
    }
    const int noticeWidth = qMin(560, width() - 160);
    const int noticeHeight = qMin(160, qMax(44, m_notice->heightForWidth(noticeWidth)));
    int noticeY = height() - noticeHeight - 76;
    if (m_consolePanel && !m_consolePanel->isHidden())
        noticeY -= consoleHeight + 12;
    m_notice->setGeometry((width() - noticeWidth) / 2, qMax(76, noticeY), noticeWidth, noticeHeight);
    for (auto *overlay : {m_actions, m_palette, m_viewControls, m_consoleControls, m_notice}) {
        if (overlay)
            overlay->raise();
    }
}

void WorkflowEditorWidget::setActivityPanel(QWidget *panel)
{
    m_activityPanel = panel;
    panel->setParent(this);
    panel->show();
    arrangeOverlays();
    panel->raise();
}
