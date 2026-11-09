#include "ProjectFilesWidget.h"
#include "CanvasIcons.h"
#include "SourceEditor.h"
#include "WorkspaceGraphWidget.h"
#include "MarkdownPreview.h"
#include "FileExplorerView.h"
#include "FileEditorWorkspace.h"
#include "WorkspaceFileModel.h"
#include "WorkspaceFilesController.h"
#include <QHideEvent>
#include <QShowEvent>

#include <QDir>
#include <QApplication>
#include <QFileInfo>
#include <QMenu>
#include <QToolButton>
#include <QPainter>
#include <QFontDatabase>
#include <QHBoxLayout>
#include <QInputDialog>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPointer>
#include <QPushButton>
#include <QSortFilterProxyModel>
#include <QCryptographicHash>
#include <QSignalBlocker>
#include <QStyle>
#include <QTabBar>
#include <QShortcut>
#include <QSplitter>
#include <QStackedWidget>
#include <QTextDocument>
#include <QTreeView>
#include <QTimer>
#include <QDesktopServices>
#include <QVBoxLayout>
#include <memory>

namespace {
constexpr qint64 maxFileBytes = 1024 * 1024; // Same editable-document bound as workflow YAML.
}

ProjectFilesWidget::ProjectFilesWidget(QWidget *parent) : QWidget(parent)
{
    auto *files = new WorkspaceFilesController(this);
    setObjectName("projectFiles");
    setAttribute(Qt::WA_StyledBackground);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(20, 16, 20, 12);
    layout->setSpacing(12);
    auto *header = new QWidget;
    header->setObjectName("filesPageHeader");
    auto *pageHeader = new QHBoxLayout(header);
    pageHeader->setContentsMargins(0, 0, 0, 0);
    pageHeader->setSpacing(16);
    auto *pageTitles = new QVBoxLayout;
    pageTitles->setSpacing(4);
    auto *pageTitle = new QLabel(tr("Files"));
    pageTitle->setObjectName("filesPageTitle");
    pageTitles->addWidget(pageTitle);
    auto *rootLabel = new QLabel(tr("Browse and edit your workspace."));
    rootLabel->setObjectName("filesPageSubtitle");
    rootLabel->setTextFormat(Qt::PlainText);
    rootLabel->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    pageTitles->addWidget(rootLabel);
    pageHeader->addLayout(pageTitles, 1);
    layout->addWidget(header);
    auto *splitter = new QSplitter;
    m_splitter = splitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(12);
    auto *explorer = new QWidget;
    explorer->setObjectName("filesExplorerPanel");
    explorer->setAttribute(Qt::WA_StyledBackground);
    auto *explorerLayout = new QVBoxLayout(explorer);
    explorerLayout->setContentsMargins(12, 12, 12, 12);
    explorerLayout->setSpacing(10);
    m_controls = new QWidget;
    m_controls->setEnabled(false);
    auto *toolbar = new QHBoxLayout(m_controls);
    toolbar->setContentsMargins(0, 0, 0, 0);
    toolbar->setSizeConstraint(QLayout::SetMinimumSize);
    auto *heading = new QLabel(tr("Explorer"));
    heading->setObjectName("filesPanelTitle");
    toolbar->addWidget(heading, 1);
    auto newFile = [this]() {
        bool ok = false;
        const QString name = QInputDialog::getText(this, tr("New file"), tr("Path relative to this folder"), QLineEdit::Normal, {}, &ok);
        if (ok && !name.isEmpty())
            createFile(QDir(m_root).relativeFilePath(QDir(m_explorerDirectory).absoluteFilePath(name)));
    };
    auto addTool = [this](QHBoxLayout *row, const QString &label, CanvasIcons::Symbol symbol, const auto &action) {
        auto *button = new QToolButton;
        button->setIcon(CanvasIcons::icon(symbol));
        button->setToolTip(label);
        button->setAccessibleName(label);
        button->setObjectName("editorIconButton");
        button->setFixedSize(32, 32);
        row->addWidget(button);
        connect(button, &QToolButton::clicked, this, action);
        return button;
    };
    m_graphButton = new QPushButton(tr("Graph"));
    m_graphButton->setObjectName("openWorkspaceGraph");
    m_graphButton->installEventFilter(this);
    m_graphButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Branch));
    m_graphButton->setIconSize(QSize(16, 16));
    m_graphButton->setSizePolicy(QSizePolicy::Fixed, QSizePolicy::Fixed);
    m_graphButton->setAccessibleName(tr("Workspace graph"));
    m_graphButton->setToolTip(tr("Open the workspace graph for the selected folder"));
    m_graphButton->setEnabled(false);
    pageHeader->addWidget(m_graphButton);
    connect(m_graphButton, &QPushButton::clicked, this, &ProjectFilesWidget::openGraph);
    addTool(toolbar, tr("New file"), CanvasIcons::Symbol::Add, newFile);
    addTool(toolbar, tr("New folder"), CanvasIcons::Symbol::Folder, [this]() {
        bool ok = false;
        const QString name = QInputDialog::getText(this, tr("New folder"), tr("Folder name"), QLineEdit::Normal, {}, &ok);
        const QString path = QDir(m_explorerDirectory).filePath(name);
        if (!ok || name.isEmpty() || !contained(path))
            return;
        auto *files = WorkspaceFilesController::get(this);
        if (filesOperationPending())
            return;
        files->beginWrite();
        files->writer->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "mkdir"},
            {"path", QDir(m_root).relativeFilePath(path)}}, this, [this, files](const auto &result) {
            files->endWrite();
            if (!result.success())
                showMessage(result.error);
            else
                m_model->refresh();
        });
    });
    addTool(toolbar, tr("Refresh files"), CanvasIcons::Symbol::Refresh, [this]() {
        m_model->refresh();
    });
    explorerLayout->addWidget(m_controls);
    m_model = new WorkspaceFileModel(this);
    auto *refreshTimer = new QTimer(this);
    refreshTimer->setObjectName("filesAutomaticRefresh");
    refreshTimer->setInterval(5000);
    connect(refreshTimer, &QTimer::timeout, this, [this]() {
        if (isVisible() && isEnabled() && !filesOperationPending())
            m_model->revalidate();
    });
    files->filter->setSourceModel(m_model);
    m_model->setShowHidden(true);
    m_explorer = new FileExplorerView;
    m_explorer->setModel(files->filter);
    m_tree = m_explorer->treeView();
    m_tree->setObjectName("fileTree");
    m_tree->setSortingEnabled(true);
    m_tree->sortByColumn(0, Qt::AscendingOrder);
    m_explorer->setMinimumWidth(180);
    m_explorer->setAvailable(false);
    auto *navigation = m_explorer->toolbarLayout();
    m_explorerUp = new QToolButton;
    m_explorerUp->setObjectName("fileExplorerUp");
    m_explorerUp->setProperty("fileNavigation", true);
    m_explorerUp->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Up));
    m_explorerUp->setToolTip(tr("Up one folder"));
    m_explorerUp->setAccessibleName(tr("Up one folder"));
    m_explorerUp->setFixedSize(32, 32);
    auto *rootButton = new QToolButton;
    rootButton->setProperty("fileNavigation", true);
    rootButton->setObjectName("fileExplorerRoot");
    rootButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Folder));
    rootButton->setToolTip(tr("Project root"));
    rootButton->setAccessibleName(tr("Project root"));
    rootButton->setFixedSize(32, 32);
    m_explorerLocation = new QLabel;
    m_explorerLocation->setObjectName("editorSectionLabel");
    m_explorerLocation->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    navigation->insertWidget(0, m_explorerUp);
    navigation->insertWidget(1, rootButton);
    navigation->insertWidget(2, m_explorerLocation, 1);
    connect(m_explorerUp, &QToolButton::clicked, this, [this]() {
        navigateExplorer(QFileInfo(m_explorerDirectory).absolutePath());
    });
    connect(rootButton, &QToolButton::clicked, this, [this]() { navigateExplorer(m_root); });
    connect(m_model, &WorkspaceFileModel::directoryLoaded, this, [this](const QString &directory) {
        const QString listingError = property("filesListingError").toString();
        if (!listingError.isEmpty() && property("filesListingErrorPath").toString() == directory
            && m_status->text() == listingError) {
            setProperty("filesListingError", QString());
            showMessage({});
        }
        if (!m_root.isEmpty() && directory == QDir(m_root).relativeFilePath(m_explorerDirectory)) {
            auto *files = WorkspaceFilesController::get(this);
            m_explorer->setRootIndex(files->filter->mapFromSource(m_model->indexForPath(directory)));
            m_explorer->setAvailable(true);
        }
    });
    connect(m_model, &WorkspaceFileModel::listingFailed, this, [this](const QString &directory, const QString &error) {
        setProperty("filesListingError", error);
        setProperty("filesListingErrorPath", directory);
        showMessage(error);
    });
    explorerLayout->addWidget(m_explorer, 1);
    auto *filter = new QLineEdit;
    filter->setObjectName("filesFilter");
    filter->setClearButtonEnabled(true);
    filter->setPlaceholderText(tr("Filter filenames…"));
    filter->setAccessibleName(tr("Filter filenames"));
    explorerLayout->insertWidget(1, filter);
    connect(filter, &QLineEdit::textChanged, this, [this](const QString &text) {
        WorkspaceFilesController::get(this)->filter->setFilterFixedString(text);
    });
    splitter->addWidget(explorer);

    auto *workspace = new QWidget;
    m_workspace = workspace;
    workspace->setObjectName("editorWorkspace");
    auto *workspaceLayout = new QVBoxLayout(workspace);
    workspaceLayout->setContentsMargins(0, 0, 0, 0);
    workspaceLayout->setSpacing(0);
    auto *fileBar = new QHBoxLayout;
    fileBar->setObjectName("filesDocumentHeader");
    fileBar->setContentsMargins(14, 8, 8, 8);
    fileBar->setSpacing(10);
    auto *fileIcon = new QLabel;
    fileIcon->setPixmap(CanvasIcons::icon(CanvasIcons::Symbol::Code).pixmap(20, 20));
    auto *documentIdentity = new QWidget;
    auto *identityLayout = new QHBoxLayout(documentIdentity);
    identityLayout->setContentsMargins(0, 0, 0, 0);
    identityLayout->setSpacing(10);
    identityLayout->addWidget(fileIcon);
    auto *fileTitles = new QVBoxLayout;
    fileTitles->setSpacing(3);
    m_title = new QLabel(tr("Files"));
    m_title->setObjectName("filesDocumentTitle");
    m_title->setTextFormat(Qt::PlainText);
    m_title->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    fileTitles->addWidget(m_title);
    auto *fileLocation = new QLabel;
    fileLocation->setObjectName("filesDocumentLocation");
    fileLocation->setTextFormat(Qt::PlainText);
    fileLocation->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    fileTitles->addWidget(fileLocation);
    identityLayout->addLayout(fileTitles, 1);
    fileBar->addWidget(documentIdentity, 1);
    m_fileActions = new QWidget;
    m_fileActions->setObjectName("fileDocumentActions");
    auto *actions = new QHBoxLayout(m_fileActions);
    actions->setContentsMargins(0, 0, 0, 0);
    actions->setSpacing(6);
    auto *save = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Save), tr("Save"));
    save->setObjectName("editorSave");
    save->setToolTip(tr("Save file (Ctrl+S)"));
    actions->addWidget(save);
    connect(save, &QPushButton::clicked, this, [this]() { saveFileAsync(); });
    m_openWorkflow = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Play), tr("Open as workflow"));
    m_openWorkflow->setObjectName("fileOpenWorkflow");
    m_openWorkflow->setToolTip(tr("Review this YAML in Workflow Studio, then use Run to validate and launch it with Dockpipe."));
    actions->addWidget(m_openWorkflow);
    connect(m_openWorkflow, &QPushButton::clicked, this, [this]() {
        if (WorkspaceFilesController::get(this)->reader->context().kind == WorkspaceFilesClient::Context::Kind::Remote)
            return;
        const QString suffix = QFileInfo(m_path).suffix().toLower();
        if ((suffix != "yml" && suffix != "yaml") || !contained(m_path))
            return;
        const QString path = m_path;
        confirmPathAsync(path, [this, path](bool allowed) {
            if (allowed)
                emit workflowOpenRequested(path);
        });
    });
    auto *find = new QLineEdit;
    find->setObjectName("fileFind");
    find->setPlaceholderText(tr("Find in file · Enter for next, Shift+Enter for previous"));
    m_findBar = new QWidget;
    m_findBar->hide();
    auto *findLayout = new QHBoxLayout(m_findBar);
    findLayout->setContentsMargins(12, 4, 12, 8);
    findLayout->addWidget(find, 1);
    auto showFind = [this, find]() {
        if (m_path.isEmpty())
            return;
        m_findBar->show();
        find->setFocus();
        find->selectAll();
    };
    m_previewButton = addTool(actions, tr("Preview Markdown"), CanvasIcons::Symbol::Preview, []() {});
    m_previewButton->setObjectName("fileMarkdownPreviewButton");
    m_previewButton->setCheckable(true);
    m_previewButton->setAccessibleName(tr("Preview Markdown"));
    connect(m_previewButton, &QToolButton::toggled, this, [this](bool checked) {
        if (auto *record = editorWorkspace()->activeDocument())
            record->preview = checked;
        updateMarkdownPreview();
    });
    addTool(actions, tr("Find in file (Ctrl+F)"), CanvasIcons::Symbol::Search, showFind);
    auto *split = addTool(actions, tr("Split editor"), CanvasIcons::Symbol::Sidebar, []() {});
    split->setObjectName("fileSplitEditor");
    auto *splitMenu = new QMenu(split);
    const auto splitCurrent = [this](Qt::Orientation orientation) {
        const QString path = m_path;
        editorWorkspace()->split(orientation);
        if (!path.isEmpty())
            editorWorkspace()->openInOtherPane(path);
    };
    splitMenu->addAction(tr("Split right"), this, [splitCurrent]() { splitCurrent(Qt::Horizontal); });
    splitMenu->addAction(tr("Split below"), this, [splitCurrent]() { splitCurrent(Qt::Vertical); });
    splitMenu->addSeparator();
    splitMenu->addAction(tr("Single pane"), this, [this]() { editorWorkspace()->singlePane(); });
    split->setMenu(splitMenu);
    split->setPopupMode(QToolButton::InstantPopup);
    addTool(findLayout, tr("Close search"), CanvasIcons::Symbol::Close, [this]() { m_findBar->hide(); });
    auto *more = addTool(actions, tr("File actions"), CanvasIcons::Symbol::More, []() {});
    auto *menu = new QMenu(more);
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Rename), tr("Rename…"), this, [this]() {
        bool ok = false;
        const QString name = QInputDialog::getText(this, tr("Rename file"), tr("Path relative to this folder"),
            QLineEdit::Normal, QDir(m_root).relativeFilePath(m_path), &ok);
        if (ok && !name.isEmpty())
            renameFile(QDir(m_root).filePath(name));
    });
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Refresh), tr("Reload from disk"), this, [this]() { reloadFileAsync(); });
    menu->addAction(tr("Save all files"), this, [this]() { saveAllFilesAsync(); });
    menu->addSeparator();
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Trash), tr("Delete file…"), this, [this]() {
        if (!m_path.isEmpty() && QMessageBox::question(this, tr("Delete file"),
            tr("Permanently delete %1?").arg(QDir(m_root).relativeFilePath(m_path)),
            QMessageBox::Yes | QMessageBox::Cancel, QMessageBox::Cancel) == QMessageBox::Yes)
            deleteFile();
    });
    more->setMenu(menu);
    more->setPopupMode(QToolButton::InstantPopup);
    addTool(actions, tr("Close file"), CanvasIcons::Symbol::Close, [this]() {
        editorWorkspace()->closeActiveTab();
    });
    auto *headerActions = new QWidget;
    headerActions->setObjectName("filesToolbarActions");
    auto *headerActionsLayout = new QHBoxLayout(headerActions);
    headerActionsLayout->setContentsMargins(0, 0, 0, 0);
    headerActionsLayout->setSpacing(6);
    actions->removeWidget(split);
    headerActionsLayout->addWidget(m_fileActions);
    headerActionsLayout->addWidget(split);
    fileBar->addWidget(headerActions, 0, Qt::AlignRight);
    workspaceLayout->addLayout(fileBar);
    workspace->installEventFilter(this);
    workspaceLayout->addWidget(m_findBar);
    m_documents = new QStackedWidget;
    m_editor = new SourceEditor;
    m_editor->setObjectName("fileTextEditor");
    m_workflowEditor = new SourceEditor;
    m_workflowEditor->setObjectName("workflowFileTextEditor");
    for (auto *editor : {m_editor, m_workflowEditor})
        m_documents->addWidget(editor);
    m_editor->setReadOnly(true);
    m_empty = new QWidget;
    m_empty->setObjectName("fileEditorEmpty");
    auto *emptyLayout = new QVBoxLayout(m_empty);
    emptyLayout->setSpacing(16);
    emptyLayout->addStretch();
    auto *emptyIcon = new QLabel;
    emptyIcon->setPixmap(CanvasIcons::icon(CanvasIcons::Symbol::Code).pixmap(48, 48));
    emptyLayout->addWidget(emptyIcon, 0, Qt::AlignHCenter);
    auto *emptyTitle = new QLabel(tr("Everything in its place"));
    emptyTitle->setObjectName("editorEmptyTitle");
    emptyLayout->addWidget(emptyTitle, 0, Qt::AlignHCenter);
    auto *emptyHint = new QLabel(tr("Select a file to edit, or create something new."));
    emptyHint->setObjectName("editorMuted");
    emptyHint->setWordWrap(true);
    emptyLayout->addWidget(emptyHint, 0, Qt::AlignHCenter);
    auto *create = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Add), tr("Create a file"));
    connect(create, &QPushButton::clicked, this, newFile);
    emptyLayout->addWidget(create, 0, Qt::AlignHCenter);
    emptyLayout->addStretch();
    m_documents->addWidget(m_empty);
    m_documents->setCurrentWidget(m_empty);
    auto *editorSplit = new QSplitter(Qt::Horizontal);
    editorSplit->setChildrenCollapsible(false);
    auto *tabWorkspace = new FileEditorWorkspace(m_documents, static_cast<SourceEditor *>(m_editor),
        static_cast<SourceEditor *>(m_workflowEditor), m_empty);
    editorSplit->addWidget(tabWorkspace);
    m_markdownPreview = new MarkdownPreview;
    m_markdownPreview->hide();
    editorSplit->addWidget(m_markdownPreview);
    editorSplit->setStretchFactor(0, 1);
    editorSplit->setStretchFactor(1, 1);
    workspaceLayout->addWidget(editorSplit, 1);
    m_previewTimer = new QTimer(this);
    m_previewTimer->setSingleShot(true);
    m_previewTimer->setInterval(150);
    connect(m_previewTimer, &QTimer::timeout, this, &ProjectFilesWidget::renderMarkdownPreview);
    connect(m_markdownPreview, &QTextBrowser::anchorClicked, this, [this](const QUrl &url) {
        if (url.isRelative() && url.path().isEmpty()) {
            m_markdownPreview->scrollToAnchor(url.fragment());
        } else if (url.scheme() == "https" || url.scheme() == "http") {
            QDesktopServices::openUrl(url);
        } else if (url.isRelative() && url.authority().isEmpty()) {
            openFile(QDir(QFileInfo(m_path).absolutePath()).absoluteFilePath(url.path()));
        }
    });
    m_editorStatus = new QLabel;
    m_editorStatus->setObjectName("editorStatus");
    m_editorStatus->setMargin(10);
    workspaceLayout->addWidget(m_editorStatus);
    splitter->addWidget(workspace);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({300, 800});
    layout->addWidget(splitter, 1);
    m_status = new QLabel;
    m_status->setObjectName("editorMuted");
    m_status->setWordWrap(true);
    m_status->setTextFormat(Qt::PlainText);
    layout->addWidget(m_status);
    connect(find, &QLineEdit::returnPressed, this, [this, find]() {
        auto *editor = qobject_cast<QPlainTextEdit *>(m_documents->currentWidget());
        if (!editor || find->text().isEmpty())
            return;
        const bool backwards = QApplication::keyboardModifiers().testFlag(Qt::ShiftModifier);
        const auto flags = backwards ? QTextDocument::FindBackward : QTextDocument::FindFlags{};
        if (!editor->find(find->text(), flags)) {
            editor->moveCursor(backwards ? QTextCursor::End : QTextCursor::Start);
            if (!editor->find(find->text(), flags))
                showMessage(tr("No matches for “%1”.").arg(find->text()));
        }
    });
    auto *saveShortcut = new QShortcut(QKeySequence::Save, this);
    saveShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(saveShortcut, &QShortcut::activated, this, [this]() { saveFileAsync(); });
    auto *search = new QShortcut(QKeySequence::Find, this);
    search->setContext(Qt::WidgetWithChildrenShortcut);
    connect(search, &QShortcut::activated, this, showFind);
    auto *closeTab = new QShortcut(QKeySequence::Close, this);
    closeTab->setContext(Qt::WidgetWithChildrenShortcut);
    connect(closeTab, &QShortcut::activated, this, [this]() { editorWorkspace()->closeActiveTab(); });
    connect(m_explorer, &FileExplorerView::clicked, this, [this](const QModelIndex &index) {
        if (!index.data(WorkspaceFileModel::DirectoryRole).toBool())
            openFileAsync(explorerPath(index));
    });
    connect(m_explorer, &FileExplorerView::activated, this, [this](const QModelIndex &index) {
        if (index.data(WorkspaceFileModel::DirectoryRole).toBool()) {
            if (m_explorer->mode() == FileExplorerView::Mode::Folders)
                navigateExplorer(explorerPath(index));
        } else {
            openFileAsync(explorerPath(index));
        }
    });
    // Reparenting the editors can rehighlight them and emit textChanged.
    // Connect only once every pane used by these callbacks is constructed.
    tabWorkspace->confirmClose = [this](const QString &path) {
        if (filesOperationPending())
            return false;
        auto *record = editorWorkspace()->document(path);
        if (!record || record->workflow || !record->document || !record->document->isModified())
            return true;
        struct CloseDecision {
            bool synchronous = true;
            bool complete = false;
            bool allowed = false;
        };
        auto decision = std::make_shared<CloseDecision>();
        confirmPathAsync(path, [this, path, decision](bool allowed) {
            decision->complete = true;
            decision->allowed = allowed;
            if (decision->synchronous || !allowed || !editorWorkspace()->activate(path))
                return;
            const auto *record = editorWorkspace()->document(path);
            if (!record || !record->document || record->document->isModified())
                return;
            editorWorkspace()->closeActiveTab();
        });
        decision->synchronous = false;
        return decision->complete && decision->allowed;
    };
    connect(tabWorkspace, &FileEditorWorkspace::currentChanged, this, &ProjectFilesWidget::syncActiveDocument);
    connect(tabWorkspace, &FileEditorWorkspace::documentsChanged, this, [this]() {
        setProperty("fileLeaveConfirmed", false);
        syncActiveDocument();
        if (m_previewButton->isChecked())
            m_previewTimer->start();
    });
    syncActiveDocument();
}

ProjectFilesWidget::~ProjectFilesWidget() = default;

void ProjectFilesWidget::setFocusedPreview(bool focused)
{
    findChild<QWidget *>("filesPageHeader")->setVisible(!focused);
    m_splitter->widget(0)->setVisible(!focused);
    layout()->setContentsMargins(focused ? 0 : 20, focused ? 0 : 16,
        focused ? 0 : 20, focused ? 0 : 12);
}

bool ProjectFilesWidget::eventFilter(QObject *object, QEvent *event)
{
    if ((event->type() == QEvent::Resize || event->type() == QEvent::LayoutRequest)
        && object->objectName() == "editorWorkspace") {
        auto *documentHeader = object->findChild<QHBoxLayout *>("filesDocumentHeader");
        const auto *actions = m_workspace->findChild<QWidget *>("filesToolbarActions");
        const bool compact = m_workspace->width() < actions->sizeHint().width() + 220;
        const auto direction = compact ? QBoxLayout::TopToBottom : QBoxLayout::LeftToRight;
        if (documentHeader->direction() != direction) {
            documentHeader->setDirection(direction);
            documentHeader->setStretch(0, compact ? 0 : 1);
        }
    }
    if (object == m_graphButton && (event->type() == QEvent::FontChange
        || event->type() == QEvent::StyleChange || event->type() == QEvent::Polish)) {
        // Stylesheet font changes can leave a nested toolbar's cached height
        // unchanged. Use the polished button hint as its content minimum.
        QTimer::singleShot(0, m_graphButton, [button = m_graphButton] {
            button->setMinimumSize(button->sizeHint());
        });
    }
    return QWidget::eventFilter(object, event);
}

bool ProjectFilesWidget::contained(const QString &path) const
{
    if (m_root.isEmpty() || !QDir::isAbsolutePath(path))
        return false;
    const QString relative = QDir(m_root).relativeFilePath(QDir::cleanPath(path));
    return relative != ".." && !relative.startsWith("../") && !QDir::isAbsolutePath(relative);
}

QString ProjectFilesWidget::explorerPath(const QModelIndex &index) const
{
    return QDir(m_root).filePath(index.data(WorkspaceFileModel::RelativePathRole).toString());
}

bool ProjectFilesWidget::filesOperationPending() const
{
    const auto *files = WorkspaceFilesController::get(this);
    return files->writer->busy() || files->pendingWrites > 0 || files->savingAll;
}

void ProjectFilesWidget::showMessage(const QString &message)
{
    m_status->setText(message);
    m_status->setVisible(!message.isEmpty());
}

FileEditorWorkspace *ProjectFilesWidget::editorWorkspace() const
{
    return findChild<FileEditorWorkspace *>("fileEditorWorkspace");
}

void ProjectFilesWidget::syncActiveDocument()
{
    auto *workspace = editorWorkspace();
    const QString previousPath = m_path;
    m_editor = workspace->activeNormalEditor();
    m_workflowEditor = workspace->activeWorkflowEditor();
    m_documents = workspace->activeStack();
    for (auto *tabs : workspace->findChildren<QTabBar *>()) {
        const bool active = tabs->parentWidget() == m_documents->parentWidget();
        if (tabs->property("filePaneActive").toBool() != active) {
            tabs->setProperty("filePaneActive", active);
            tabs->style()->unpolish(tabs);
            tabs->style()->polish(tabs);
            tabs->update();
        }
    }
    const auto *record = workspace->activeDocument();
    m_path = record ? record->path : QString();
    if (previousPath != m_path)
        showMessage({});
    m_original = record ? record->original : QByteArray();
    m_crlf = record && record->crlf;
    m_bom = record && record->bom;
    {
        const QSignalBlocker blocker(m_previewButton);
        m_previewButton->setChecked(record && record->preview);
    }
    if (auto *editor = workspace->activeEditor())
        connect(editor, &QPlainTextEdit::cursorPositionChanged, this, &ProjectFilesWidget::updateEditorStatus, Qt::UniqueConnection);
    updateTitle();
}

bool ProjectFilesWidget::hasUnsavedChanges() const
{
    return editorWorkspace()->hasUnsavedChanges();
}

QTextDocument *ProjectFilesWidget::currentDocument() const
{
    const auto *record = editorWorkspace()->activeDocument();
    return record ? record->document.data() : nullptr;
}

bool ProjectFilesWidget::confirmCurrentFile()
{
    const auto *record = editorWorkspace()->activeDocument();
    return !filesOperationPending() && (!record || record->workflow || !record->document
        || !record->document->isModified());
}

void ProjectFilesWidget::confirmPathAsync(const QString &path, Completion completion)
{
    if (filesOperationPending()) {
        showMessage(tr("A file operation is still running. Your open files are preserved."));
        if (completion)
            completion(false);
        return;
    }
    const auto *record = editorWorkspace()->document(path);
    if (!record || record->workflow || !record->document || !record->document->isModified()) {
        if (completion)
            completion(true);
        return;
    }
    const auto answer = QMessageBox::question(this, tr("Unsaved file"),
        tr("Save changes to %1?").arg(QFileInfo(path).fileName()),
        QMessageBox::Save | QMessageBox::Discard | QMessageBox::Cancel, QMessageBox::Save);
    if (answer == QMessageBox::Save) {
        savePathAsync(path, [this, path, completion](bool saved) {
            const auto *record = editorWorkspace()->document(path);
            const bool clean = record && record->document && !record->document->isModified();
            if (completion)
                completion(saved && clean);
        });
    } else if (completion) {
        completion(answer == QMessageBox::Discard);
    }
}

bool ProjectFilesWidget::confirmLeave()
{
    return !filesOperationPending() && (!hasUnsavedChanges() || property("fileLeaveConfirmed").toBool());
}

void ProjectFilesWidget::confirmLeaveAsync(Completion completion)
{
    if (filesOperationPending()) {
        showMessage(tr("A file operation is still running. Your open files are preserved."));
        if (completion)
            completion(false);
        return;
    }
    if (!hasUnsavedChanges() || property("fileLeaveConfirmed").toBool()) {
        if (completion)
            completion(true);
        return;
    }
    int dirtyFiles = 0;
    for (const auto &path : editorWorkspace()->paths()) {
        const auto *record = editorWorkspace()->document(path);
        if (!record->workflow && record->document && record->document->isModified())
            ++dirtyFiles;
    }
    const auto answer = QMessageBox::question(this, tr("Unsaved files"),
        tr("Save changes to %n open file(s) before leaving?", nullptr, dirtyFiles),
        QMessageBox::Save | QMessageBox::Discard | QMessageBox::Cancel, QMessageBox::Save);
    auto finish = [this, completion](bool allowed) {
        if (allowed) {
            setProperty("fileLeaveConfirmed", true);
            QTimer::singleShot(0, this, [this]() { setProperty("fileLeaveConfirmed", false); });
        }
        if (completion)
            completion(allowed);
    };
    if (answer == QMessageBox::Save) {
        saveAllFilesAsync([this, finish](bool saved) { finish(saved && !hasUnsavedChanges()); });
    } else {
        finish(answer == QMessageBox::Discard);
    }
}

void ProjectFilesWidget::clearFile()
{
    const QString closingPath = m_path;
    if (!closingPath.isEmpty())
        editorWorkspace()->removeDocument(closingPath);
}

bool ProjectFilesWidget::setRoot(const QString &root)
{
    if (!root.isEmpty() && !QDir::isAbsolutePath(root)) {
        showMessage(tr("This folder is unavailable. Your open files are still here."));
        return false;
    }
    const QString directory = root.isEmpty() ? QString() : QDir::cleanPath(root);
    if (m_root == directory && WorkspaceFilesController::get(this)->reader->context().kind
        == WorkspaceFilesClient::Context::Kind::Local)
        return true;
    if (!confirmLeave())
        return false;
    closeGraph();
    editorWorkspace()->clearDocuments();
    editorWorkspace()->setWorkflowDocument({}, nullptr);
    m_workflowPath.clear();
    m_root = directory;
    auto *files = WorkspaceFilesController::get(this);
    WorkspaceFilesClient::Context context;
    context.localRoot = directory;
    context.generation = files->reader->context().generation + 1;
    files->setContext(context);
    m_controls->setEnabled(!directory.isEmpty());
    m_explorer->setAvailable(!directory.isEmpty());
    m_explorer->setRootIndex({});
    m_explorerDirectory.clear();
    m_explorerLocation->clear();
    m_explorerUp->setEnabled(false);
    m_model->setContext(context);
    if (!directory.isEmpty())
        navigateExplorer(directory);
    auto *rootLabel = findChild<QLabel *>("filesPageSubtitle");
    rootLabel->setText(directory.isEmpty() ? tr("Open a project to browse and edit its files.")
                                         : QDir::toNativeSeparators(directory));
    rootLabel->setToolTip(QDir::toNativeSeparators(directory));
    m_graphButton->setEnabled(!directory.isEmpty());
    showMessage({});
    return true;
}

void ProjectFilesWidget::setRootAsync(const QString &root, Completion completion)
{
    confirmLeaveAsync([this, root, completion](bool allowed) {
        const bool changed = allowed && setRoot(root);
        if (completion)
            completion(changed);
    });
}

bool ProjectFilesWidget::navigateExplorer(const QString &directory)
{
    if (!contained(directory))
        return false;
    const QString relative = QDir(m_root).relativeFilePath(QDir::cleanPath(directory));
    m_explorerDirectory = QDir::cleanPath(directory);
    m_model->setDirectory(relative);
    m_explorer->setRootIndex({});
    m_explorer->setAvailable(true);
    m_model->fetchMore({});
    m_explorerLocation->setText(relative == "." ? tr("Root") : QFileInfo(directory).fileName());
    m_explorerLocation->setToolTip(QDir::toNativeSeparators(directory));
    m_explorerUp->setEnabled(relative != ".");
    return true;
}

void ProjectFilesWidget::setWorkflowDocument(const QString &path, QTextDocument *document)
{
    if (WorkspaceFilesController::get(this)->reader->context().kind == WorkspaceFilesClient::Context::Kind::Remote)
        return;
    const QString workflowPath = path.isEmpty() ? QString() : QDir::cleanPath(QFileInfo(path).absoluteFilePath());
    if (!editorWorkspace()->setWorkflowDocument(workflowPath, document)) {
        showMessage(editorWorkspace()->error());
        return;
    }
    m_workflowPath = workflowPath;
    syncActiveDocument();
}

bool ProjectFilesWidget::openFile(const QString &path)
{
    const QString key = QDir::cleanPath(path);
    if (contained(key) && editorWorkspace()->contains(key)) {
        auto *files = WorkspaceFilesController::get(this);
        ++files->openGeneration;
        files->reader->cancel();
        return editorWorkspace()->activate(key);
    }
    if (contained(key) && !m_workflowPath.isEmpty() && key == m_workflowPath) {
        ++WorkspaceFilesController::get(this)->openGeneration;
        return editorWorkspace()->openWorkflow(key);
    }
    openFileAsync(path);
    return false;
}

void ProjectFilesWidget::openFileAsync(const QString &path, Completion completion)
{
    if (!contained(path)) {
        showMessage(tr("Choose a regular file inside this folder. Linked files and Git metadata are not edited here."));
        if (completion)
            completion(false);
        return;
    }
    const QString key = QDir::cleanPath(path);
    if (editorWorkspace()->contains(key)) {
        auto *files = WorkspaceFilesController::get(this);
        ++files->openGeneration;
        files->reader->cancel();
        const bool activated = editorWorkspace()->activate(key);
        if (completion)
            completion(activated);
        return;
    }
    if (!m_workflowPath.isEmpty() && key == m_workflowPath) {
        ++WorkspaceFilesController::get(this)->openGeneration;
        const bool opened = editorWorkspace()->openWorkflow(key);
        if (!opened)
            showMessage(editorWorkspace()->error());
        else
            showMessage(tr("Workflow YAML · shared with the canvas and YAML view"));
        if (completion)
            completion(opened);
        return;
    }
    loadFileAsync(key, false, std::move(completion));
}

bool ProjectFilesWidget::loadFile(const QString &path, bool reload)
{
    loadFileAsync(path, reload, {});
    return false;
}

void ProjectFilesWidget::loadFileAsync(const QString &path, bool reload, Completion completion)
{
    auto *files = WorkspaceFilesController::get(this);
    const quint64 generation = ++files->openGeneration;
    const auto context = files->reader->context();
    const QString relative = QDir(m_root).relativeFilePath(path);
    const QPointer<QTextDocument> reloading = reload && editorWorkspace()->document(path)
        ? editorWorkspace()->document(path)->document : nullptr;
    const int documentRevision = reloading ? reloading->revision() : -1;
    auto execute = [this, files, generation, context, path, relative, reload, reloading, documentRevision, completion]() {
        if (generation != files->openGeneration) {
            if (completion)
                completion(false);
            return;
        }
        files->reader->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "read"}, {"path", relative}},
            this, [this, files, generation, context, path, relative, reload, reloading, documentRevision, completion](const auto &result) {
            if (generation != files->openGeneration || !(context == files->reader->context())) {
                if (completion)
                    completion(false);
                return;
            }
            if (!result.success()) {
                showMessage(result.error);
                if (completion)
                    completion(false);
                return;
            }
            const auto decoded = QByteArray::fromBase64Encoding(result.value.value("content").toString().toLatin1(),
                QByteArray::AbortOnBase64DecodingErrors);
            const QByteArray bytes = decoded.decoded;
            const bool bom = bytes.startsWith("\xEF\xBB\xBF");
            const QByteArray textBytes = bom ? bytes.mid(3) : bytes;
            const QString text = QString::fromUtf8(textBytes);
            const QString revision = result.value.value("revision").toString();
            const QString digest = QString::fromLatin1(QCryptographicHash::hash(bytes, QCryptographicHash::Sha256).toHex());
            if (!decoded || bytes.size() > maxFileBytes || bytes.contains('\0') || text.toUtf8() != textBytes
                || revision != digest || result.value.value("path").toString() != relative) {
                showMessage(tr("This file is binary, is not UTF-8, or exceeds the 1 MiB editor limit."));
                if (completion)
                    completion(false);
                return;
            }
            const bool crlf = bytes.contains("\r\n");
            bool opened = false;
            if (reload) {
                auto *record = editorWorkspace()->document(path);
                if (record && record->document == reloading && reloading && !record->workflow
                    && reloading->revision() == documentRevision) {
                    // Update the captured document, even if another tab became active.
                    reloading->setPlainText(text);
                    reloading->setModified(false);
                    record->original = bytes;
                    record->crlf = crlf;
                    record->bom = bom;
                    editorWorkspace()->refreshTabs();
                    opened = true;
                }
            } else {
                opened = editorWorkspace()->addDocument(path, bytes, text, crlf, bom);
            }
            if (opened) {
                files->revisions.insert(path, revision);
                syncActiveDocument();
                showMessage({});
            }
            if (completion)
                completion(opened);
        });
    };
    if (files->reader->busy())
        files->reader->cancel();
    if (files->reader->busy()) {
        auto connection = std::make_shared<QMetaObject::Connection>();
        *connection = connect(files->reader, &WorkspaceFilesClient::busyChanged, this,
            [execute, connection](bool busy) {
            if (!busy) {
                QObject::disconnect(*connection);
                execute();
            }
        });
    } else {
        execute();
    }
}

bool ProjectFilesWidget::reloadFile()
{
    reloadFileAsync();
    return false;
}

void ProjectFilesWidget::reloadFileAsync(Completion completion)
{
    const QString path = m_path;
    const auto *record = editorWorkspace()->document(path);
    if (!record || record->workflow) {
        showMessage(tr("Reopen the workflow from Workflows to reload its YAML and canvas together."));
        if (completion)
            completion(false);
        return;
    }
    confirmPathAsync(path, [this, path, completion](bool allowed) {
        if (allowed)
            loadFileAsync(path, true, completion);
        else if (completion)
            completion(false);
    });
}

bool ProjectFilesWidget::saveFile()
{
    saveFileAsync();
    return false;
}

void ProjectFilesWidget::saveFileAsync(Completion completion)
{
    savePathAsync(m_path, std::move(completion));
}

void ProjectFilesWidget::savePathAsync(const QString &path, Completion completion)
{
    auto *files = WorkspaceFilesController::get(this);
    auto *record = editorWorkspace()->document(path);
    if (!record || !record->document || record->workflow || files->writer->busy()) {
        if (record && record->workflow)
            emit workflowSaveRequested();
        if (completion)
            completion(false);
        return;
    }
    const QPointer<QTextDocument> document = record->document;
    const QString text = document->toPlainText();
    QByteArray bytes = text.toUtf8();
    if (record->crlf)
        bytes.replace("\n", "\r\n");
    if (record->bom)
        bytes.prepend("\xEF\xBB\xBF");
    if (bytes.size() > maxFileBytes || !contained(path) || !files->revisions.contains(path)) {
        showMessage(tr("The file exceeds 1 MiB or its location changed. Your edits are still here."));
        if (completion)
            completion(false);
        return;
    }
    const auto context = files->writer->context();
    const QString relative = QDir(m_root).relativeFilePath(path);
    files->beginWrite();
    files->writer->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "write"},
        {"path", relative}, {"revision", files->revisions.value(path)},
        {"content", QString::fromLatin1(bytes.toBase64())}}, this,
        [this, files, context, path, relative, document, bytes, text, completion](const auto &result) {
        files->endWrite();
        bool saved = result.success() && context == files->writer->context();
        auto *record = editorWorkspace()->document(path);
        const QString expected = QString::fromLatin1(QCryptographicHash::hash(bytes, QCryptographicHash::Sha256).toHex());
        saved = saved && result.value.value("revision").toString() == expected
            && result.value.value("path").toString() == relative;
        if (saved && record && document && record->document == document) {
            record->original = bytes;
            files->revisions.insert(path, expected);
            document->setModified(document->toPlainText() != text);
            editorWorkspace()->refreshTabs();
            syncActiveDocument();
            showMessage(tr("Saved %1").arg(QDir(m_root).relativeFilePath(path)));
            emit fileSaved(path);
        } else if (!saved) {
            showMessage(result.error.isEmpty() ? tr("Cannot save: %1").arg(tr("Invalid workspace file response.")) : result.error);
        }
        if (completion)
            completion(saved);
    });
}

bool ProjectFilesWidget::saveAllFiles()
{
    if (!hasUnsavedChanges() && !filesOperationPending())
        return true;
    saveAllFilesAsync();
    return false;
}

void ProjectFilesWidget::saveAllFilesAsync(Completion completion)
{
    auto *files = WorkspaceFilesController::get(this);
    if (filesOperationPending()) {
        if (completion)
            completion(false);
        return;
    }
    files->setSavingAll(true);
    const QStringList paths = editorWorkspace()->paths();
    auto next = std::make_shared<std::function<void(int)>>();
    const std::weak_ptr<std::function<void(int)>> weakNext(next);
    *next = [this, files, paths, completion, weakNext](int position) {
        int current = position;
        while (current < paths.size()) {
            const auto *record = editorWorkspace()->document(paths[current]);
            if (record && !record->workflow && record->document && record->document->isModified())
                break;
            ++current;
        }
        if (current == paths.size()) {
            files->setSavingAll(false);
            if (completion)
                completion(true);
            return;
        }
        auto next = weakNext.lock();
        savePathAsync(paths[current], [this, files, current, next, completion](bool saved) {
            if (!saved) {
                files->setSavingAll(false);
                if (completion)
                    completion(false);
                return;
            }
            (*next)(current + 1);
        });
    };
    (*next)(0);
}

bool ProjectFilesWidget::createFile(const QString &relativePath)
{
    createFileAsync(relativePath);
    return false;
}

void ProjectFilesWidget::createFileAsync(const QString &relativePath, Completion completion)
{
    auto *files = WorkspaceFilesController::get(this);
    if (filesOperationPending()) {
        if (completion)
            completion(false);
        return;
    }
    const QString path = QDir(m_root).filePath(relativePath);
    files->beginWrite();
    files->writer->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "create"}, {"path", relativePath}},
        this, [this, files, path, completion](const auto &result) {
        files->endWrite();
        if (!result.success()) {
            showMessage(result.error);
            if (completion)
                completion(false);
            return;
        }
        m_model->refresh();
        openFileAsync(path, completion);
    });
}

bool ProjectFilesWidget::renameFile(const QString &destination)
{
    renameFileAsync(destination);
    return false;
}

void ProjectFilesWidget::renameFileAsync(const QString &destination, Completion completion)
{
    auto *files = WorkspaceFilesController::get(this);
    const QString previous = m_path;
    const QString target = QDir::cleanPath(destination);
    if (filesOperationPending() || previous.isEmpty() || previous == m_workflowPath || !contained(target)
        || editorWorkspace()->contains(target) || !files->revisions.contains(previous)) {
        if (completion)
            completion(false);
        return;
    }
    files->beginWrite();
    files->writer->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "rename"},
        {"path", QDir(m_root).relativeFilePath(previous)}, {"destination", QDir(m_root).relativeFilePath(target)},
        {"revision", files->revisions.value(previous)}}, this, [this, files, previous, target, completion](const auto &result) {
        files->endWrite();
        bool renamed = result.success();
        if (renamed) {
            renamed = editorWorkspace()->renameDocument(previous, target);
            if (renamed)
                files->revisions.insert(target, files->revisions.take(previous));
            m_model->refresh();
            syncActiveDocument();
        } else {
            showMessage(result.error);
        }
        if (completion)
            completion(renamed);
    });
}

bool ProjectFilesWidget::deleteFile()
{
    deleteFileAsync();
    return false;
}

void ProjectFilesWidget::deleteFileAsync(Completion completion)
{
    const QString path = m_path;
    auto *files = WorkspaceFilesController::get(this);
    if (path.isEmpty() || path == m_workflowPath || filesOperationPending() || !files->revisions.contains(path)) {
        if (completion)
            completion(false);
        return;
    }
    confirmPathAsync(path, [this, files, path, completion](bool allowed) {
        if (!allowed) {
            if (completion)
                completion(false);
            return;
        }
        const auto *record = editorWorkspace()->document(path);
        const QPointer<QTextDocument> document = record ? record->document : nullptr;
        const int revision = document ? document->revision() : -1;
        files->beginWrite();
        files->writer->run(QJsonObject{{"schema", "dockpipe.files/v1"}, {"action", "delete"},
            {"path", QDir(m_root).relativeFilePath(path)}, {"revision", files->revisions.value(path)}}, this,
            [this, files, path, document, revision, completion](const auto &result) {
            files->endWrite();
            if (result.success()) {
                files->revisions.remove(path);
                // A user can keep typing while deletion runs. Retain that draft.
                if (document && document->revision() != revision)
                    document->setModified(true);
                else
                    editorWorkspace()->removeDocument(path);
                m_model->refresh();
            } else {
                showMessage(result.error);
            }
            if (completion)
                completion(result.success());
        });
    });
}

void ProjectFilesWidget::updateTitle()
{
    const auto *document = currentDocument();
    const bool modified = document && document->isModified();
    QString title = m_path.isEmpty() ? tr("No file open") : QFileInfo(m_path).fileName();
    if (!m_path.isEmpty() && modified)
        title += tr("  • Unsaved");
    m_title->setText(title);
    m_title->setToolTip(m_path);
    const QString relativeDirectory = QDir(m_root).relativeFilePath(QFileInfo(m_path).absolutePath());
    auto *fileLocation = findChild<QLabel *>("filesDocumentLocation");
    fileLocation->setText(m_path.isEmpty() ? tr("Workspace editor")
        : (relativeDirectory == "." ? tr("Workspace root") : relativeDirectory));
    fileLocation->setToolTip(m_path.isEmpty() ? QString{} : QDir::toNativeSeparators(m_path));
    m_fileActions->setEnabled(!m_path.isEmpty());
    findChild<QPushButton *>("editorSave")->setEnabled(!m_path.isEmpty() && modified);
    const QString suffix = QFileInfo(m_path).suffix().toLower();
    const bool local = WorkspaceFilesController::get(this)->reader->context().kind == WorkspaceFilesClient::Context::Kind::Local;
    m_openWorkflow->setVisible(local && (suffix == "yml" || suffix == "yaml"));
    updateMarkdownPreview();
    updateEditorStatus();
    emit dirtyChanged(hasUnsavedChanges());
}

void ProjectFilesWidget::updateMarkdownPreview()
{
    const QString suffix = QFileInfo(m_path).suffix().toLower();
    const bool markdown = m_documents->currentWidget() == m_editor
        && (suffix == "md" || suffix == "markdown" || suffix == "mdown" || suffix == "mkd");
    m_previewButton->setVisible(markdown);
    if (!markdown && m_previewButton->isChecked())
        m_previewButton->setChecked(false);
    m_markdownPreview->setVisible(markdown && m_previewButton->isChecked());
    m_previewButton->setToolTip(m_previewButton->isChecked()
        ? tr("Hide Markdown preview") : tr("Preview Markdown"));
    if (markdown && m_previewButton->isChecked())
        m_previewTimer->start();
    else
        m_previewTimer->stop();
}

void ProjectFilesWidget::renderMarkdownPreview()
{
    // Preview shares the editor's 1 MiB UTF-8 contract, including unsaved drafts.
    const QString text = m_editor->toPlainText();
    if (text.toUtf8().size() > maxFileBytes) {
        m_markdownPreview->setPlainText(tr("This draft exceeds the 1 MiB Markdown preview limit."));
        return;
    }
    m_markdownPreview->render(text);
}

void ProjectFilesWidget::updateEditorStatus()
{
    if (!m_editorStatus)
        return;
    if (m_path.isEmpty()) {
        m_editorStatus->setText(tr("Built-in editor   ·   Ctrl+F to find   ·   Ctrl+S to save"));
        return;
    }
    if (m_documents->currentWidget() == m_empty)
        return;
    auto *editor = static_cast<SourceEditor *>(m_documents->currentWidget());
    const auto cursor = editor->textCursor();
    m_editorStatus->setText(tr("%1   ·   UTF-8   ·   %2                         Ln %3, Col %4")
        .arg(editor->languageName(), m_crlf && editor == m_editor ? "CRLF" : "LF")
        .arg(cursor.blockNumber() + 1).arg(cursor.positionInBlock() + 1));
}

void ProjectFilesWidget::applyCockpitSettings(const CockpitSettings &settings)
{
    Q_UNUSED(settings);
    // Files are host-owned and remain available when the optional universe is disabled.
}

void ProjectFilesWidget::openGraph()
{
    if (m_graph || m_root.isEmpty() || WorkspaceFilesController::get(this)->reader->context().kind
        == WorkspaceFilesClient::Context::Kind::Remote)
        return;
    QString directory = m_explorerDirectory.isEmpty() ? m_root : m_explorerDirectory;
    const auto selected = m_tree->currentIndex();
    if (selected.isValid()) {
        const QString path = explorerPath(selected);
        directory = selected.data(WorkspaceFileModel::DirectoryRole).toBool() ? path : QFileInfo(path).absolutePath();
    }
    m_graph = new WorkspaceGraphWidget(m_root, directory, this);
    m_splitter->hide();
    m_status->hide();
    static_cast<QVBoxLayout *>(layout())->insertWidget(1, m_graph, 1);
    m_graphButton->setEnabled(false);
    connect(m_graph, &WorkspaceGraphWidget::closeRequested, this, &ProjectFilesWidget::closeGraph);
    connect(m_graph, &WorkspaceGraphWidget::openFileRequested, this, [this](const QString &path) {
        openFileAsync(path, [this](bool opened) {
            if (opened)
                closeGraph();
            else
                m_status->show();
        });
    });
    connect(m_graph, &WorkspaceGraphWidget::revealRequested, this, [this](const QString &path) {
        const auto source = m_model->indexForPath(QDir(m_root).relativeFilePath(path));
        const QModelIndex index = WorkspaceFilesController::get(this)->filter->mapFromSource(source);
        if (index.isValid()) {
            m_tree->setCurrentIndex(index);
            m_tree->scrollTo(index);
            if (index.data(WorkspaceFileModel::DirectoryRole).toBool())
                m_tree->expand(index);
        }
        closeGraph();
    });
    m_graph->show();
}

void ProjectFilesWidget::closeGraph()
{
    m_graphButton->setEnabled(!m_root.isEmpty() && WorkspaceFilesController::get(this)->reader->context().kind
        == WorkspaceFilesClient::Context::Kind::Local);
    if (!m_graph)
        return;
    auto *graph = m_graph;
    m_graph = nullptr;
    layout()->removeWidget(graph);
    graph->hide();
    // Deferred destruction is needed when the request originates in a graph item.
    graph->deleteLater();
    m_splitter->show();
    m_status->setVisible(!m_status->text().isEmpty());
}

void ProjectFilesWidget::hideEvent(QHideEvent *event)
{
    QWidget::hideEvent(event);
    findChild<QTimer *>("filesAutomaticRefresh")->stop();
    m_model->setActive(false);
    // Adding an OpenGL viewport to an already visible Files page can recreate
    // its native window with a synchronous hide/show. Let that transaction
    // finish before deciding whether the user actually left this page.
    const QPointer<WorkspaceGraphWidget> hiddenGraph(m_graph);
    QTimer::singleShot(0, this, [this, hiddenGraph]() {
        if (hiddenGraph && m_graph == hiddenGraph && !isVisible())
            closeGraph();
    });
}

void ProjectFilesWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    m_model->setActive(isEnabled());
    findChild<QTimer *>("filesAutomaticRefresh")->start();
}
