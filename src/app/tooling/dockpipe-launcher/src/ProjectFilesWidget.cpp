#include "ProjectFilesWidget.h"
#include "CanvasIcons.h"
#include "SourceEditor.h"
#include "WorkspaceFiles.h"
#include "WorkspacePackageManager.h"
#include <QHideEvent>

#include <QDir>
#include <QApplication>
#include <QFile>
#include <QFileInfo>
#include <QFileSystemModel>
#include <QFileIconProvider>
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
#include <QPushButton>
#include <QSaveFile>
#include <QShortcut>
#include <QSplitter>
#include <QStackedWidget>
#include <QTextDocument>
#include <QTreeView>
#include <QVBoxLayout>

namespace {
constexpr qint64 maxFileBytes = 1024 * 1024; // Same editable-document bound as workflow YAML.
}

namespace {
class ProjectFileIcons final : public QFileIconProvider {
public:
    QIcon icon(const QFileInfo &file) const override
    {
        return CanvasIcons::icon(file.isDir() ? CanvasIcons::Symbol::Folder : CanvasIcons::Symbol::Code);
    }

    QString type(const QFileInfo &file) const override
    {
        const QString displayType = QFileIconProvider::type(file);
        if (!displayType.isEmpty())
            return displayType;

        // Qt 6.2 needs a nonempty type to insert newly discovered file rows.
        if (file.isDir())
            return QCoreApplication::translate("ProjectFilesWidget", "Folder");
        return QCoreApplication::translate("ProjectFilesWidget", "File");
    }

    QIcon icon(IconType type) const override
    {
        return CanvasIcons::icon(type == Folder ? CanvasIcons::Symbol::Folder : CanvasIcons::Symbol::Code);
    }
};

class ProjectFileTree final : public QTreeView {
public:
    using QTreeView::QTreeView;
protected:
    void drawBranches(QPainter *painter, const QRect &rect, const QModelIndex &index) const override
    {
        if (!model()->hasChildren(index))
            return;
        const auto symbol = isExpanded(index) ? CanvasIcons::Symbol::ChevronDown : CanvasIcons::Symbol::ChevronRight;
        CanvasIcons::icon(symbol).paint(painter, QRect(rect.right() - 18, rect.center().y() - 7, 14, 14));
    }
};
}

ProjectFilesWidget::ProjectFilesWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("projectFiles");
    setAttribute(Qt::WA_StyledBackground);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(20, 16, 20, 12);
    layout->setSpacing(12);
    auto *splitter = new QSplitter;
    m_splitter = splitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(12);
    auto *explorer = new QWidget;
    auto *explorerLayout = new QVBoxLayout(explorer);
    explorerLayout->setContentsMargins(0, 0, 0, 0);
    m_controls = new QWidget;
    m_controls->setEnabled(false);
    auto *toolbar = new QHBoxLayout(m_controls);
    toolbar->setContentsMargins(8, 0, 0, 8);
    auto *heading = new QLabel(tr("EXPLORER"));
    heading->setObjectName("editorSectionLabel");
    toolbar->addWidget(heading, 1);
    auto newFile = [this]() {
        bool ok = false;
        const QString name = QInputDialog::getText(this, tr("New file"), tr("Path relative to this folder"), QLineEdit::Normal, {}, &ok);
        if (ok && !name.isEmpty())
            createFile(name);
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
    auto *graph = addTool(toolbar, tr("Workspace graph"), CanvasIcons::Symbol::Branch, [this]() { openGraph(); });
    graph->setObjectName("openWorkspaceGraph");
    graph->setText(tr("Graph"));
    graph->setToolButtonStyle(Qt::ToolButtonTextBesideIcon);
    graph->setFixedSize(72, 32);
    addTool(toolbar, tr("New file"), CanvasIcons::Symbol::Add, newFile);
    addTool(toolbar, tr("New folder"), CanvasIcons::Symbol::Folder, [this]() {
        bool ok = false;
        const QString name = QInputDialog::getText(this, tr("New folder"), tr("Folder name"), QLineEdit::Normal, {}, &ok);
        const QString path = QDir(m_root).filePath(name);
        if (ok && !name.isEmpty() && (!contained(path) || !QDir().mkdir(path)))
            showMessage(tr("Cannot create that folder. Choose a new name inside this folder."));
    });
    explorerLayout->addWidget(m_controls);
    m_model = new QFileSystemModel(this);
    // QFileSystemModel does not own a custom provider. Its lifetime exceeds the model.
    static ProjectFileIcons fileIcons;
    m_model->setIconProvider(&fileIcons);
    m_model->setFilter(QDir::AllEntries | QDir::NoDotAndDotDot | QDir::Hidden | QDir::NoSymLinks);
    m_model->setReadOnly(true);
    m_tree = new ProjectFileTree;
    m_tree->setObjectName("fileTree");
    m_tree->setModel(m_model);
    m_tree->setHeaderHidden(true);
    m_tree->setUniformRowHeights(true);
    m_tree->setIndentation(18);
    m_tree->setIconSize(QSize(18, 18));
    m_tree->setAnimated(false);
    for (int column = 1; column < 4; ++column)
        m_tree->hideColumn(column);
    m_tree->setMinimumWidth(180);
    m_tree->setEnabled(false);
    m_tree->hide();
    connect(m_model, &QFileSystemModel::directoryLoaded, this, [this]() {
        if (m_root.isEmpty())
            return;
        const QModelIndex index = m_model->index(m_root);
        if (index.isValid()) {
            m_tree->setRootIndex(index);
            m_tree->show();
        }
    });
    m_tree->setSortingEnabled(true);
    m_tree->sortByColumn(0, Qt::AscendingOrder);
    explorerLayout->addWidget(m_tree, 1);
    splitter->addWidget(explorer);

    auto *workspace = new QWidget;
    m_workspace = workspace;
    workspace->setObjectName("editorWorkspace");
    auto *workspaceLayout = new QVBoxLayout(workspace);
    workspaceLayout->setContentsMargins(0, 0, 0, 0);
    workspaceLayout->setSpacing(0);
    auto *fileBar = new QHBoxLayout;
    fileBar->setContentsMargins(14, 8, 8, 8);
    fileBar->setSpacing(10);
    auto *fileIcon = new QLabel;
    fileIcon->setPixmap(CanvasIcons::icon(CanvasIcons::Symbol::Code).pixmap(20, 20));
    fileBar->addWidget(fileIcon);
    m_title = new QLabel(tr("Files"));
    m_title->setTextFormat(Qt::PlainText);
    m_title->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    fileBar->addWidget(m_title, 1);
    m_fileActions = new QWidget;
    auto *actions = new QHBoxLayout(m_fileActions);
    actions->setContentsMargins(0, 0, 0, 0);
    actions->setSpacing(6);
    auto *save = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Save), tr("Save file"));
    save->setObjectName("editorSave");
    save->setToolTip(tr("Save file (Ctrl+S)"));
    actions->addWidget(save);
    connect(save, &QPushButton::clicked, this, [this]() { saveFile(); });
    m_openWorkflow = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Play), tr("Open as workflow"));
    m_openWorkflow->setObjectName("fileOpenWorkflow");
    m_openWorkflow->setToolTip(tr("Review this YAML in Workflow Studio, then use Run to validate and launch it with Dockpipe."));
    actions->addWidget(m_openWorkflow);
    connect(m_openWorkflow, &QPushButton::clicked, this, [this]() {
        const QString suffix = QFileInfo(m_path).suffix().toLower();
        if ((suffix != "yml" && suffix != "yaml") || !contained(m_path) || !confirmLeave())
            return;
        const QString path = m_path;
        closeFlightEditor();
        emit workflowOpenRequested(path);
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
    addTool(actions, tr("Find in file (Ctrl+F)"), CanvasIcons::Symbol::Search, showFind);
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
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Refresh), tr("Reload from disk"), this, [this]() { reloadFile(); });
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
        if (confirmLeave())
            clearFile();
    });
    fileBar->addWidget(m_fileActions);
    workspaceLayout->addLayout(fileBar);
    workspaceLayout->addWidget(m_findBar);
    m_documents = new QStackedWidget;
    m_editor = new SourceEditor;
    m_editor->setObjectName("fileTextEditor");
    m_workflowEditor = new SourceEditor;
    m_workflowEditor->setObjectName("workflowFileTextEditor");
    for (auto *editor : {m_editor, m_workflowEditor}) {
        m_documents->addWidget(editor);
        connect(editor, &QPlainTextEdit::cursorPositionChanged, this, &ProjectFilesWidget::updateEditorStatus);
        connect(editor, &QPlainTextEdit::textChanged, this, &ProjectFilesWidget::updateTitle);
    }
    m_editor->setReadOnly(true);
    m_empty = new QWidget;
    m_empty->setObjectName("fileEditorEmpty");
    auto *emptyLayout = new QVBoxLayout(m_empty);
    emptyLayout->setSpacing(16);
    emptyLayout->addStretch();
    auto *emptyIcon = new QLabel;
    emptyIcon->setPixmap(CanvasIcons::icon(CanvasIcons::Symbol::Code).pixmap(48, 48));
    emptyLayout->addWidget(emptyIcon, 0, Qt::AlignHCenter);
    auto *emptyTitle = new QLabel(tr("Your files. Your workspace."));
    emptyTitle->setObjectName("editorEmptyTitle");
    emptyLayout->addWidget(emptyTitle, 0, Qt::AlignHCenter);
    auto *emptyHint = new QLabel(tr("Open a file from the explorer to start editing."));
    emptyHint->setObjectName("editorMuted");
    emptyHint->setWordWrap(true);
    emptyLayout->addWidget(emptyHint, 0, Qt::AlignHCenter);
    auto *create = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Add), tr("Create a file"));
    connect(create, &QPushButton::clicked, this, newFile);
    emptyLayout->addWidget(create, 0, Qt::AlignHCenter);
    emptyLayout->addStretch();
    m_documents->addWidget(m_empty);
    m_documents->setCurrentWidget(m_empty);
    workspaceLayout->addWidget(m_documents, 1);
    m_editorStatus = new QLabel;
    m_editorStatus->setObjectName("editorStatus");
    m_editorStatus->setMargin(10);
    workspaceLayout->addWidget(m_editorStatus);
    splitter->addWidget(workspace);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({260, 800});
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
    connect(saveShortcut, &QShortcut::activated, this, [this]() { saveFile(); });
    auto *search = new QShortcut(QKeySequence::Find, this);
    search->setContext(Qt::WidgetWithChildrenShortcut);
    connect(search, &QShortcut::activated, this, showFind);
    connect(m_tree, &QTreeView::clicked, this, [this](const QModelIndex &index) {
        if (!m_model->isDir(index))
            openFile(m_model->filePath(index));
    });
    connect(m_editor->document(), &QTextDocument::modificationChanged, this, &ProjectFilesWidget::updateTitle);
    updateTitle();
}

ProjectFilesWidget::~ProjectFilesWidget()
{
    // The pane is owned by this file surface, even while mounted above the flight view.
    closeFlightEditor();
}

void ProjectFilesWidget::openFlightEditor(const QString &path)
{
    auto *flight = m_graph ? m_graph->flightWidget() : nullptr;
    if (!flight || m_flightHud)
        return;
    m_graph->pauseFlight(true);
    if (!openFile(path)) {
        m_status->show();
        m_graph->pauseFlight(false);
        flight->setFocus();
        return;
    }
    m_splitterSizes = m_splitter->sizes();
    m_flightHud = new QWidget(flight);
    m_flightHud->setObjectName("flightEditorHud");
    m_flightHud->setAttribute(Qt::WA_StyledBackground);
    const bool dark = palette().color(QPalette::Base).lightness() < 128;
    const QString background = dark ? "rgba(14, 27, 38, 234)" : "rgba(246, 251, 253, 234)";
    const QString text = dark ? "#e1f8f4" : "#173e49";
    const QString editorBackground = dark ? "rgba(14, 27, 38, 0)" : "rgba(246, 251, 253, 0)";
    m_flightHud->setStyleSheet(QStringLiteral(
        "QWidget#flightEditorHud { background: rgba(5, 12, 22, 30); }"
        "QWidget#flightEditorPanel { background: %1; border: 1px solid #409e9b; border-radius: 12px; }"
        "QWidget#flightEditorPanel QWidget#editorWorkspace { background: transparent; border: none; }"
        "QWidget#flightEditorPanel QPlainTextEdit { background: %3; color: %2; border: none; }"
        "QWidget#flightEditorPanel QLabel { background: transparent; }"
        "QLabel#flightEditorHeading { color: %2; font-weight: 600; }").arg(background, text, editorBackground));
    auto *overlay = new QVBoxLayout(m_flightHud);
    auto *panel = new QWidget;
    panel->setObjectName("flightEditorPanel");
    panel->setAttribute(Qt::WA_StyledBackground);
    auto *panelLayout = new QVBoxLayout(panel);
    panelLayout->setContentsMargins(12, 12, 12, 8);
    panelLayout->setSpacing(4);
    auto *header = new QHBoxLayout;
    auto *heading = new QLabel(tr("FILE LINK  /  FLIGHT PAUSED"));
    heading->setObjectName("flightEditorHeading");
    header->addWidget(heading, 1);
    auto *resume = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Close), tr("Return to flight · Esc"));
    resume->setObjectName("closeFlightEditor");
    connect(resume, &QPushButton::clicked, this, &ProjectFilesWidget::closeFlightEditor);
    header->addWidget(resume);
    panelLayout->addLayout(header);
    // Move the original pane, not a copy of its document or save logic. The tree
    // remains hidden and no new filesystem model or editor buffer is created.
    panelLayout->addWidget(m_workspace, 1);
    panelLayout->addWidget(m_status);
    overlay->addWidget(panel);
    auto *escape = new QShortcut(QKeySequence(Qt::Key_Escape), m_flightHud);
    escape->setContext(Qt::WidgetWithChildrenShortcut);
    connect(escape, &QShortcut::activated, this, &ProjectFilesWidget::closeFlightEditor);
    flight->installEventFilter(this);
    positionFlightEditor();
    m_workspace->show();
    m_status->show();
    m_flightHud->show();
    m_flightHud->raise();
    m_documents->currentWidget()->setFocus();
}

void ProjectFilesWidget::positionFlightEditor()
{
    if (!m_flightHud || !m_graph || !m_graph->flightWidget())
        return;
    auto *flight = m_graph->flightWidget();
    m_flightHud->setGeometry(flight->rect());
    const int horizontal = qMax(12, (flight->width() - 1100) / 2);
    const int top = qBound(12, flight->height() / 14, 48);
    const int bottom = qBound(12, flight->height() / 8, 100);
    m_flightHud->layout()->setContentsMargins(horizontal, top, horizontal, bottom);
}

bool ProjectFilesWidget::eventFilter(QObject *object, QEvent *event)
{
    if (m_flightHud && m_graph && object == m_graph->flightWidget() && event->type() == QEvent::Resize)
        positionFlightEditor();
    return QWidget::eventFilter(object, event);
}

void ProjectFilesWidget::closeFlightEditor()
{
    if (!m_flightHud)
        return;
    auto *hud = m_flightHud;
    m_flightHud = nullptr;
    auto *flight = m_graph ? m_graph->flightWidget() : nullptr;
    if (flight)
        flight->removeEventFilter(this);
    m_workspace->hide();
    m_workspace->setParent(m_splitter);
    m_splitter->insertWidget(1, m_workspace);
    m_splitter->setSizes(m_splitterSizes);
    m_workspace->show();
    static_cast<QVBoxLayout *>(layout())->addWidget(m_status);
    m_status->setVisible(!m_graph);
    hud->hide();
    hud->deleteLater();
    if (flight) {
        m_graph->pauseFlight(false);
        flight->setFocus();
    }
    // Unsaved text and undo history remain in the same editor when flight resumes.
}

bool ProjectFilesWidget::contained(const QString &path) const
{
    return WorkspaceFiles::contains(m_root, path);
}

void ProjectFilesWidget::showMessage(const QString &message) { m_status->setText(message); }

bool ProjectFilesWidget::hasUnsavedChanges() const
{
    return m_editor->document()->isModified();
}

bool ProjectFilesWidget::confirmLeave()
{
    if (!hasUnsavedChanges())
        return true;
    const auto answer = QMessageBox::question(this, tr("Unsaved file"), tr("Save changes to %1?").arg(m_path),
        QMessageBox::Save | QMessageBox::Discard | QMessageBox::Cancel, QMessageBox::Save);
    if (answer == QMessageBox::Save)
        return saveFile();
    if (answer == QMessageBox::Discard) {
        m_editor->document()->setModified(false);
        return true;
    }
    return false;
}

void ProjectFilesWidget::clearFile()
{
    m_path.clear();
    m_original.clear();
    m_editor->clear();
    m_editor->document()->setModified(false);
    m_editor->setReadOnly(true);
    m_documents->setCurrentWidget(m_empty);
    m_findBar->hide();
    updateTitle();
}

bool ProjectFilesWidget::setRoot(const QString &root)
{
    if (m_root == root)
        return true;
    if (!confirmLeave())
        return false;
    closeGraph();
    m_root = root;
    clearFile();
    m_tree->setEnabled(!root.isEmpty());
    m_controls->setEnabled(!root.isEmpty());
    m_tree->hide();
    if (!root.isEmpty()) {
        const QModelIndex index = m_model->setRootPath(root);
        if (index.isValid()) {
            m_tree->setRootIndex(index);
            m_tree->show();
        }
    }
    showMessage(root.isEmpty() ? tr("Open a project or save a workflow to browse its files.") : root);
    return true;
}

void ProjectFilesWidget::setWorkflowDocument(const QString &path, QTextDocument *document)
{
    if (m_path == m_workflowPath && path != m_workflowPath)
        clearFile();
    m_workflowPath = path;
    m_workflowEditor->setDocument(document);
    static_cast<SourceEditor *>(m_workflowEditor)->setFileName(path);
    connect(document, &QTextDocument::modificationChanged, this, &ProjectFilesWidget::updateTitle, Qt::UniqueConnection);
}

bool ProjectFilesWidget::openFile(const QString &path)
{
    if (path == m_path)
        return true;
    if (!contained(path)) {
        showMessage(tr("Choose a regular file inside this folder. Linked files and Git metadata are not edited here."));
        return false;
    }
    if (!confirmLeave())
        return false;
    if (!m_workflowPath.isEmpty() && QFileInfo(path).absoluteFilePath() == QFileInfo(m_workflowPath).absoluteFilePath()) {
        m_path = path;
        m_documents->setCurrentWidget(m_workflowEditor);
        showMessage(tr("Workflow YAML · shared with the canvas and YAML view"));
        updateTitle();
        return true;
    }
    QFile file(path);
    if (!QFileInfo(path).isFile() || !file.open(QIODevice::ReadOnly) || file.size() > maxFileBytes) {
        showMessage(tr("Cannot open this file. The built-in editor supports text files up to 1 MiB."));
        return false;
    }
    QByteArray bytes = file.read(maxFileBytes + 1);
    const bool bom = bytes.startsWith("\xEF\xBB\xBF");
    const QByteArray textBytes = bom ? bytes.mid(3) : bytes;
    const QString text = QString::fromUtf8(textBytes);
    if (bytes.size() > maxFileBytes || bytes.contains('\0') || text.toUtf8() != textBytes) {
        showMessage(tr("This file is binary, is not UTF-8, or exceeds the 1 MiB editor limit."));
        return false;
    }
    m_path = path;
    m_original = bytes;
    m_crlf = bytes.contains("\r\n");
    m_bom = bytes.startsWith("\xEF\xBB\xBF");
    if (m_bom)
        bytes.remove(0, 3);
    static_cast<SourceEditor *>(m_editor)->setFileName(path);
    m_editor->setPlainText(QString::fromUtf8(bytes));
    m_editor->document()->setModified(false);
    m_editor->setReadOnly(false);
    m_documents->setCurrentWidget(m_editor);
    showMessage(tr("UTF-8 · %1").arg(m_crlf ? "CRLF" : "LF"));
    updateTitle();
    return true;
}

bool ProjectFilesWidget::reloadFile()
{
    if (m_path.isEmpty())
        return false;
    if (m_documents->currentWidget() == m_workflowEditor) {
        showMessage(tr("Reopen the workflow from Workflows to reload its YAML and canvas together."));
        return false;
    }
    if (!confirmLeave())
        return false;
    const QString path = m_path;
    m_path.clear();
    if (openFile(path))
        return true;
    m_path = path;
    return false;
}

bool ProjectFilesWidget::saveFile()
{
    if (m_documents->currentWidget() == m_workflowEditor) {
        emit workflowSaveRequested();
        return true;
    }
    if (m_path.isEmpty() || m_editor->isReadOnly())
        return false;
    QByteArray bytes = m_editor->toPlainText().toUtf8();
    if (m_crlf)
        bytes.replace("\n", "\r\n");
    if (m_bom)
        bytes.prepend("\xEF\xBB\xBF");
    if (bytes.size() > maxFileBytes || !contained(m_path)) {
        showMessage(tr("The file exceeds 1 MiB or its location changed. Your edits are still here."));
        return false;
    }
    QFile current(m_path);
    if (!QFileInfo(m_path).isFile() || !current.open(QIODevice::ReadOnly) || current.read(maxFileBytes + 1) != m_original) {
        showMessage(tr("This file changed on disk. Your edits are preserved; keep a copy, then Reload to read the disk version."));
        return false;
    }
    QSaveFile file(m_path);
    if (!file.open(QIODevice::WriteOnly) || file.write(bytes) != bytes.size() || !file.commit()) {
        showMessage(tr("Cannot save: %1").arg(file.errorString()));
        return false;
    }
    m_original = bytes;
    m_editor->document()->setModified(false);
    updateTitle();
    showMessage(tr("Saved %1").arg(QDir(m_root).relativeFilePath(m_path)));
    emit fileSaved(m_path);
    return true;
}

bool ProjectFilesWidget::createFile(const QString &relativePath)
{
    const QString path = QDir(m_root).filePath(relativePath);
    if (!contained(path) || !confirmLeave())
        return false;
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly | QIODevice::NewOnly)) {
        showMessage(tr("Cannot create file: %1").arg(file.errorString()));
        return false;
    }
    file.close();
    return openFile(path);
}

bool ProjectFilesWidget::renameFile(const QString &destination)
{
    if (m_path.isEmpty() || m_path == m_workflowPath || !contained(m_path) || !contained(destination) || !confirmLeave()) {
        showMessage(tr("Choose a supporting file and a destination inside this folder. Rename the open workflow from the project Files view."));
        return false;
    }
    if (!QFile::rename(m_path, destination)) {
        showMessage(tr("Cannot rename the file. The destination may already exist."));
        return false;
    }
    clearFile();
    return openFile(destination);
}

bool ProjectFilesWidget::deleteFile()
{
    if (m_path.isEmpty() || m_path == m_workflowPath || !contained(m_path) || !confirmLeave())
        return false;
    if (!QFile::remove(m_path)) {
        showMessage(tr("Cannot delete the file."));
        return false;
    }
    clearFile();
    return true;
}

void ProjectFilesWidget::updateTitle()
{
    m_title->setText(m_path.isEmpty() ? tr("Select a file") : QDir(m_root).relativeFilePath(m_path)
        + ((m_documents->currentWidget() == m_workflowEditor ? m_workflowEditor->document()->isModified() : hasUnsavedChanges())
            ? tr("  • Unsaved") : ""));
    m_title->setToolTip(m_path);
    m_fileActions->setEnabled(!m_path.isEmpty());
    const QString suffix = QFileInfo(m_path).suffix().toLower();
    m_openWorkflow->setVisible(suffix == "yml" || suffix == "yaml");
    updateEditorStatus();
    emit dirtyChanged(hasUnsavedChanges());
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
    m_cockpitSettings = settings;
    if (!settings.enabled)
        closeGraph();
    if (m_graph)
        m_graph->applyCockpitSettings(settings);
}

void ProjectFilesWidget::openGraph()
{
    if (m_graph || m_root.isEmpty())
        return;
    if (!m_cockpitSettings.enabled) {
        m_status->setText(tr("Enable Cockpit mode in Settings to use the spatial workspace."));
        return;
    }
    QString directory = m_root;
    const auto selected = m_tree->currentIndex();
    if (selected.isValid()) {
        const QString path = m_model->filePath(selected);
        directory = m_model->isDir(selected) ? path : QFileInfo(path).absolutePath();
    }
    m_graph = WorkspacePackageManager::instance().createFiles(m_root, directory, this);
    if (!m_graph) {
        m_status->setText(tr("Enable Cockpit mode in Settings to install the spatial workspace package."));
        return;
    }
    m_graph->applyCockpitSettings(m_cockpitSettings);
    m_splitter->hide();
    m_status->hide();
    static_cast<QVBoxLayout *>(layout())->insertWidget(0, m_graph, 1);
    m_graph->requestClose = [this]() { closeGraph(); };
    m_graph->requestFlightFile = [this](const QString &path) { openFlightEditor(path); };
    m_graph->flightWillClose = [this]() { closeFlightEditor(); };
    m_graph->rendererChanged = [this]() {
        if (m_flightHud)
            m_flightHud->raise();
    };
    m_graph->requestOpenFile = [this](const QString &path) {
        if (openFile(path))
            closeGraph();
        else
            m_status->show();
    };
    m_graph->requestReveal = [this](const QString &path) {
        const QModelIndex index = m_model->index(path);
        if (index.isValid()) {
            m_tree->setCurrentIndex(index);
            m_tree->scrollTo(index);
            if (m_model->isDir(index))
                m_tree->expand(index);
        }
        closeGraph();
    };
    m_graph->show();
}

void ProjectFilesWidget::closeGraph()
{
    closeFlightEditor();
    if (!m_graph)
        return;
    auto *graph = m_graph;
    m_graph = nullptr;
    layout()->removeWidget(graph);
    graph->hide();
    // Deferred destruction is needed when the request originates in a graph item.
    graph->deleteLater();
    m_splitter->show();
    m_status->show();
}

void ProjectFilesWidget::hideEvent(QHideEvent *event)
{
    closeGraph();
    QWidget::hideEvent(event);
}
