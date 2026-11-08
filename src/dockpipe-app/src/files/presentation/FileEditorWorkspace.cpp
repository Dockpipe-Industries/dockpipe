#include "FileEditorWorkspace.h"
#include "SourceEditor.h"
#include "FileExplorerIcons.h"
#include <QEvent>
#include <QFileInfo>
#include <QHash>
#include <QHBoxLayout>
#include <QMenu>
#include <QLabel>
#include <QPlainTextDocumentLayout>
#include <QScrollBar>
#include <QSignalBlocker>
#include <QSplitter>
#include <QStackedWidget>
#include <QTabBar>
#include <QTextCursor>
#include <QTextDocument>
#include <QToolButton>
#include <QVBoxLayout>

struct FileEditorWorkspace::Pane {
    struct ViewState {
        QTextCursor cursor;
        int verticalScroll = 0;
        int horizontalScroll = 0;
    };
    QWidget *widget = nullptr;
    QTabBar *tabs = nullptr;
    QStackedWidget *stack = nullptr;
    SourceEditor *normal = nullptr;
    SourceEditor *workflow = nullptr;
    QWidget *empty = nullptr;
    QPointer<QTextDocument> normalEmptyDocument;
    QPointer<QTextDocument> workflowEmptyDocument;
    QString displayedPath;
    QHash<QString, ViewState> views;

    QList<QWidget *> surfaces() const
    {
        return {widget, tabs, stack, empty, normal, normal->viewport(),
            workflow, workflow->viewport()};
    }
};

FileEditorWorkspace::FileEditorWorkspace(QStackedWidget *primaryStack,
    SourceEditor *primaryNormal, SourceEditor *primaryWorkflow, QWidget *primaryEmpty,
    QWidget *parent)
    : QWidget(parent), m_splitter(new QSplitter(Qt::Horizontal, this))
{
    setObjectName("fileEditorWorkspace");
    m_splitter->setObjectName("filesPaneSplitter");
    m_splitter->setChildrenCollapsible(false);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->addWidget(m_splitter);
    m_primary.reset(createPane(primaryStack, primaryNormal, primaryWorkflow,
        primaryEmpty, "fileTabsPrimary"));
    auto *secondaryStack = new QStackedWidget;
    auto *secondaryNormal = new SourceEditor;
    secondaryNormal->setObjectName("fileSecondaryTextEditor");
    auto *secondaryWorkflow = new SourceEditor;
    secondaryWorkflow->setObjectName("workflowSecondaryFileTextEditor");
    auto *secondaryEmpty = new QLabel(tr("Open a file in this pane"));
    secondaryEmpty->setAlignment(Qt::AlignCenter);
    secondaryEmpty->setWordWrap(true);
    secondaryEmpty->setObjectName("fileSecondaryEmpty");
    secondaryStack->addWidget(secondaryNormal);
    secondaryStack->addWidget(secondaryWorkflow);
    secondaryStack->addWidget(secondaryEmpty);
    m_secondary.reset(createPane(secondaryStack, secondaryNormal, secondaryWorkflow,
        secondaryEmpty, "fileTabsSecondary"));
    m_secondary->widget->hide();
    m_active = m_primary.get();
}

FileEditorWorkspace::~FileEditorWorkspace()
{
    // QObject deletes child documents after C++ members have been destroyed.
    // Retire their callbacks while the record map is still available.
    for (const auto &entry : m_documents) {
        if (entry.second.document)
            disconnect(entry.second.document, nullptr, this, nullptr);
    }
    for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
        for (QWidget *surface : pane->surfaces())
            surface->removeEventFilter(this);
    }
}

FileEditorWorkspace::Pane *FileEditorWorkspace::createPane(QStackedWidget *stack,
    SourceEditor *normal, SourceEditor *workflow, QWidget *empty, const QString &tabName)
{
    auto *pane = new Pane;
    pane->widget = new QWidget(m_splitter);
    pane->widget->setMinimumWidth(220);
    pane->tabs = new QTabBar(pane->widget);
    pane->tabs->setObjectName(tabName);
    pane->tabs->setTabsClosable(true);
    pane->tabs->setMovable(true);
    pane->tabs->setExpanding(false);
    pane->tabs->setUsesScrollButtons(true);
    pane->tabs->setElideMode(Qt::ElideMiddle);
    pane->tabs->setContextMenuPolicy(Qt::CustomContextMenu);
    pane->stack = stack;
    pane->normal = normal;
    pane->workflow = workflow;
    pane->empty = empty;
    auto *layout = new QVBoxLayout(pane->widget);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    layout->addWidget(pane->tabs);
    layout->addWidget(stack);
    stack->setCurrentWidget(empty);
    for (QWidget *surface : pane->surfaces())
        surface->installEventFilter(this);
    connect(pane->tabs, &QTabBar::currentChanged, this, [this, pane] {
        m_active = pane;
        showCurrent(pane);
        emit currentChanged();
    });
    connect(pane->tabs, &QTabBar::tabCloseRequested, this,
        [this, pane](int index) { closeTab(pane, index); });
    connect(pane->tabs, &QTabBar::customContextMenuRequested, this,
        [this, pane](const QPoint &point) {
            const int index = pane->tabs->tabAt(point);
            if (index < 0)
                return;
            const QString path = pane->tabs->tabData(index).toString();
            QMenu menu(this);
            auto *openOther = menu.addAction(tr("Open in other pane"));
            auto *moveOther = menu.addAction(tr("Move to other pane"));
            menu.addSeparator();
            auto *close = menu.addAction(tr("Close"));
            auto *closeOthers = menu.addAction(tr("Close others"));
            QAction *chosen = menu.exec(pane->tabs->mapToGlobal(point));
            if (!chosen)
                return;
            m_active = pane;
            if (chosen == openOther) {
                openInOtherPane(path);
            } else if (chosen == moveOther) {
                moveToOtherPane(path);
            } else if (chosen == close) {
                closeTab(pane, index);
            } else if (chosen == closeOthers) {
                QStringList others;
                for (int tab = 0; tab < pane->tabs->count(); ++tab) {
                    const QString candidate = pane->tabs->tabData(tab).toString();
                    if (candidate != path)
                        others.append(candidate);
                }
                for (const QString &candidate : others) {
                    for (int tab = 0; tab < pane->tabs->count(); ++tab) {
                        if (pane->tabs->tabData(tab).toString() == candidate) {
                            if (!closeTab(pane, tab))
                                return;
                            break;
                        }
                    }
                }
            }
        });
    return pane;
}

FileEditorDocument *FileEditorWorkspace::document(const QString &path)
{
    const auto found = m_documents.find(path);
    return found == m_documents.end() ? nullptr : &found->second;
}

const FileEditorDocument *FileEditorWorkspace::document(const QString &path) const
{
    const auto found = m_documents.find(path);
    return found == m_documents.end() ? nullptr : &found->second;
}

FileEditorDocument *FileEditorWorkspace::activeDocument() { return document(activePath()); }

QStringList FileEditorWorkspace::paths() const
{
    QStringList result;
    for (const auto &entry : m_documents)
        result.append(entry.first);
    return result;
}

bool FileEditorWorkspace::contains(const QString &path) const { return document(path) != nullptr; }
QString FileEditorWorkspace::error() const { return m_error; }
QString FileEditorWorkspace::activePath() const { return m_active->displayedPath; }
QStackedWidget *FileEditorWorkspace::activeStack() const { return m_active->stack; }
SourceEditor *FileEditorWorkspace::activeNormalEditor() const { return m_active->normal; }
SourceEditor *FileEditorWorkspace::activeWorkflowEditor() const { return m_active->workflow; }

SourceEditor *FileEditorWorkspace::activeEditor() const
{
    const auto *record = document(activePath());
    if (!record)
        return nullptr;
    return record->workflow ? m_active->workflow : m_active->normal;
}

bool FileEditorWorkspace::hasUnsavedChanges() const
{
    for (const auto &entry : m_documents) {
        const auto &record = entry.second;
        if (!record.workflow && record.document && record.document->isModified())
            return true;
    }
    return false;
}

void FileEditorWorkspace::saveView(Pane *pane)
{
    const auto *record = document(pane->displayedPath);
    if (!record || !record->document)
        return;
    SourceEditor *editor = record->workflow ? pane->workflow : pane->normal;
    if (editor->document() != record->document)
        return;
    pane->views.insert(pane->displayedPath, {editor->textCursor(),
        editor->verticalScrollBar()->value(), editor->horizontalScrollBar()->value()});
}

void FileEditorWorkspace::detachDocument(Pane *pane, QTextDocument *textDocument)
{
    for (SourceEditor *editor : {pane->normal, pane->workflow}) {
        if (editor->document() != textDocument)
            continue;
        auto &emptyDocument = editor == pane->normal
            ? pane->normalEmptyDocument : pane->workflowEmptyDocument;
        if (!emptyDocument) {
            emptyDocument = new QTextDocument(this);
            emptyDocument->setDocumentLayout(new QPlainTextDocumentLayout(emptyDocument));
        }
        editor->setFileDocument(emptyDocument, {});
    }
}

void FileEditorWorkspace::showCurrent(Pane *pane)
{
    saveView(pane);
    const QString path = pane->tabs->tabData(pane->tabs->currentIndex()).toString();
    const auto *record = document(path);
    pane->displayedPath = record && record->document ? path : QString();
    if (pane->displayedPath.isEmpty()) {
        pane->stack->setCurrentWidget(pane->empty);
        return;
    }
    SourceEditor *editor = record->workflow ? pane->workflow : pane->normal;
    editor->setReadOnly(false);
    editor->setFileDocument(record->document, path);
    pane->stack->setCurrentWidget(editor);
    const auto state = pane->views.constFind(path);
    if (state != pane->views.cend() && state->cursor.document() == record->document) {
        editor->setTextCursor(state->cursor);
        editor->verticalScrollBar()->setValue(state->verticalScroll);
        editor->horizontalScrollBar()->setValue(state->horizontalScroll);
    }
}

void FileEditorWorkspace::select(Pane *pane, const QString &path)
{
    m_active = pane;
    for (int index = 0; index < pane->tabs->count(); ++index) {
        if (pane->tabs->tabData(index).toString() != path)
            continue;
        if (pane->tabs->currentIndex() == index) {
            showCurrent(pane);
            emit currentChanged();
        } else {
            pane->tabs->setCurrentIndex(index);
        }
        if (auto *editor = activeEditor())
            editor->setFocus();
        return;
    }
}

void FileEditorWorkspace::addTab(Pane *pane, const QString &path)
{
    {
        const QSignalBlocker blocker(pane->tabs);
        const int index = pane->tabs->addTab(QFileInfo(path).fileName());
        pane->tabs->setTabData(index, path);
        pane->tabs->setTabToolTip(index, path);
        auto *closeSlot = new QWidget(pane->tabs);
        closeSlot->setObjectName("fileTabCloseSlot");
        auto *closeLayout = new QHBoxLayout(closeSlot);
        closeLayout->setContentsMargins(0, 0, 12, 0);
        closeLayout->setSpacing(0);
        auto *close = new QToolButton(closeSlot);
        close->setObjectName("fileTabClose");
        close->setFixedSize(20, 20);
        close->setAutoRaise(true);
        close->setToolTip(tr("Close %1").arg(QFileInfo(path).fileName()));
        close->setAccessibleName(close->toolTip());
        closeLayout->addWidget(close);
        closeSlot->setFixedSize(32, 24);
        pane->tabs->setTabButton(index, QTabBar::RightSide, closeSlot);
        connect(close, &QToolButton::clicked, this, [this, pane, closeSlot] {
            for (int tab = 0; tab < pane->tabs->count(); ++tab) {
                if (pane->tabs->tabButton(tab, QTabBar::RightSide) == closeSlot) {
                    closeTab(pane, tab);
                    return;
                }
            }
        });
    }
    refreshTabs();
    select(pane, path);
}

bool FileEditorWorkspace::activate(const QString &path)
{
    if (!contains(path))
        return false;
    // Prefer the active pane when a document is deliberately visible twice.
    Pane *other = m_active == m_primary.get() ? m_secondary.get() : m_primary.get();
    for (Pane *pane : {m_active, other}) {
        for (int index = 0; index < pane->tabs->count(); ++index) {
            if (pane->tabs->tabData(index).toString() == path) {
                select(pane, path);
                return true;
            }
        }
    }
    addTab(m_active, path);
    return true;
}

bool FileEditorWorkspace::addDocument(const QString &path, const QByteArray &originalBytes,
    const QString &text, bool crlf, bool bom)
{
    m_error.clear();
    if (contains(path))
        return activate(path);
    // 32 unique documents, each read at at most 1 MiB by the owning file adapter.
    if (m_documents.size() >= 32) {
        m_error = tr("32 files are already open. Close unused tabs before opening another file.");
        return false;
    }
    auto *textDocument = new QTextDocument(this);
    textDocument->setDocumentLayout(new QPlainTextDocumentLayout(textDocument));
    textDocument->setDocumentMargin(14);
    textDocument->setPlainText(text);
    textDocument->setModified(false);
    m_documents.emplace(path, FileEditorDocument{path, originalBytes, crlf, bom,
        false, false, textDocument});
    watchDocument(path, textDocument);
    addTab(m_active, path);
    emit documentsChanged();
    return true;
}

void FileEditorWorkspace::watchDocument(const QString &path, QTextDocument *textDocument)
{
    connect(textDocument, &QTextDocument::modificationChanged, this, [this] {
        refreshTabs();
        emit documentsChanged();
    });
    connect(textDocument, &QTextDocument::contentsChanged, this, [this] {
        emit documentsChanged();
    });
    connect(textDocument, &QObject::destroyed, this, [this, path] {
        // An external workflow owner may release its document before this widget.
        const auto *record = document(path);
        if (record && !record->document)
            removeDocument(path);
    });
}

bool FileEditorWorkspace::setWorkflowDocument(const QString &path, QTextDocument *textDocument)
{
    m_error.clear();
    auto *existing = document(path);
    if (existing && !existing->workflow && existing->document
        && existing->document->isModified()) {
        m_error = tr("Save or close the file's draft before binding the workflow editor.");
        return false;
    }
    if (textDocument && !existing && m_documents.size() >= 32
        && !contains(m_workflowPath)) {
        m_error = tr("32 files are already open. Close unused tabs before opening another file.");
        return false;
    }
    if (!m_workflowPath.isEmpty() && m_workflowPath != path)
        removeDocument(m_workflowPath);
    if (!textDocument || path.isEmpty()) {
        if (!m_workflowPath.isEmpty())
            removeDocument(m_workflowPath);
        m_workflowPath.clear();
        m_workflowDocument.clear();
        return true;
    }
    if (existing && existing->document == textDocument && existing->workflow) {
        m_workflowPath = path;
        refreshTabs();
        return true;
    }
    if (existing) {
        const QPointer<QTextDocument> previous = existing->document;
        const bool previouslyOwned = !existing->workflow;
        if (previous)
            disconnect(previous, nullptr, this, nullptr);
        *existing = FileEditorDocument{path, {}, false, false, true, false, textDocument};
        for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
            pane->views.remove(path);
            if (pane->displayedPath == path)
                showCurrent(pane);
            // A hidden editor may still hold the replaced document.
            if (previous && previous != textDocument)
                detachDocument(pane, previous);
        }
        if (previouslyOwned && previous && previous != textDocument)
            delete previous;
    } else {
        m_documents.emplace(path, FileEditorDocument{path, {}, false, false,
            true, false, textDocument});
    }
    m_workflowPath = path;
    m_workflowDocument = textDocument;
    watchDocument(path, textDocument);
    refreshTabs();
    emit documentsChanged();
    return true;
}

bool FileEditorWorkspace::openWorkflow(const QString &path)
{
    if (!contains(path) && path == m_workflowPath && m_workflowDocument) {
        if (!setWorkflowDocument(path, m_workflowDocument))
            return false;
    }
    const auto *record = document(path);
    return record && record->workflow && record->document && activate(path);
}

void FileEditorWorkspace::refreshTabs()
{
    QHash<QString, int> basenameCounts;
    for (const auto &entry : m_documents)
        ++basenameCounts[QFileInfo(entry.first).fileName()];
    for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
        if (!pane)
            continue;
        for (int index = 0; index < pane->tabs->count(); ++index) {
            const QString path = pane->tabs->tabData(index).toString();
            const auto *record = document(path);
            const QFileInfo info(path);
            QString title = info.fileName();
            if (basenameCounts.value(title) > 1)
                title += QString::fromUtf8(" — ") + info.path();
            if (record && record->document && record->document->isModified())
                title += QString::fromUtf8(" •");
            pane->tabs->setTabText(index, title);
            pane->tabs->setTabIcon(index, FileExplorerIcons::instance().icon(info));
            pane->tabs->setTabToolTip(index, path);
            if (auto *slot = pane->tabs->tabButton(index, QTabBar::RightSide)) {
                if (auto *close = slot->findChild<QToolButton *>("fileTabClose")) {
                    close->setToolTip(tr("Close %1").arg(info.fileName()));
                    close->setAccessibleName(close->toolTip());
                }
            }
        }
    }
}

int FileEditorWorkspace::viewCount(const QString &path) const
{
    int count = 0;
    for (const Pane *pane : {m_primary.get(), m_secondary.get()}) {
        for (int index = 0; index < pane->tabs->count(); ++index) {
            if (pane->tabs->tabData(index).toString() == path)
                ++count;
        }
    }
    return count;
}

void FileEditorWorkspace::removeTab(Pane *pane, int index)
{
    saveView(pane);
    {
        const QSignalBlocker blocker(pane->tabs);
        pane->tabs->removeTab(index);
    }
    showCurrent(pane);
}

bool FileEditorWorkspace::closeTab(Pane *pane, int index)
{
    if (property("fileWritePending").toBool())
        return false;
    if (index < 0 || index >= pane->tabs->count())
        return false;
    const QString path = pane->tabs->tabData(index).toString();
    const auto *record = document(path);
    if (record && !record->workflow && record->document
        && record->document->isModified() && viewCount(path) == 1) {
        if (!confirmClose || !confirmClose(path))
            return false;
    }
    // The confirmation callback can activate another tab or save the draft.
    for (int tab = 0; tab < pane->tabs->count(); ++tab) {
        if (pane->tabs->tabData(tab).toString() == path) {
            removeTab(pane, tab);
            break;
        }
    }
    record = document(path);
    if (record && !record->workflow && viewCount(path) == 0)
        removeDocument(path);
    emit currentChanged();
    return true;
}

bool FileEditorWorkspace::closeActiveTab()
{
    return closeTab(m_active, m_active->tabs->currentIndex());
}

bool FileEditorWorkspace::renameDocument(const QString &oldPath, const QString &newPath)
{
    if (property("fileWritePending").toBool())
        return false;
    m_error.clear();
    auto *record = document(oldPath);
    if (!record) {
        m_error = tr("The file is no longer open.");
        return false;
    }
    if (record->workflow) {
        m_error = tr("Rename workflows through the workflow editor.");
        return false;
    }
    if (newPath.isEmpty() || (newPath != oldPath && contains(newPath))) {
        m_error = tr("Another open file already uses the destination path.");
        return false;
    }
    if (newPath == oldPath)
        return true;

    const QPointer<QTextDocument> textDocument = record->document;
    for (Pane *pane : {m_primary.get(), m_secondary.get()})
        saveView(pane);
    if (textDocument)
        disconnect(textDocument, nullptr, this, nullptr);
    auto node = m_documents.extract(oldPath);
    node.key() = newPath;
    node.mapped().path = newPath;
    m_documents.insert(std::move(node));
    for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
        if (pane->views.contains(oldPath))
            pane->views.insert(newPath, pane->views.take(oldPath));
        for (int index = 0; index < pane->tabs->count(); ++index) {
            if (pane->tabs->tabData(index).toString() == oldPath)
                pane->tabs->setTabData(index, newPath);
        }
        if (pane->displayedPath == oldPath) {
            pane->displayedPath = newPath;
            pane->normal->setFileDocument(textDocument, newPath);
        }
    }
    if (textDocument)
        watchDocument(newPath, textDocument);
    refreshTabs();
    emit documentsChanged();
    emit currentChanged();
    return true;
}

void FileEditorWorkspace::removeDocument(const QString &path)
{
    auto found = m_documents.find(path);
    if (found == m_documents.end())
        return;
    if (property("fileWritePending").toBool() && found->second.document)
        return;
    const QPointer<QTextDocument> textDocument = found->second.document;
    const bool owned = !found->second.workflow;
    for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
        for (int index = pane->tabs->count() - 1; index >= 0; --index) {
            if (pane->tabs->tabData(index).toString() == path)
                removeTab(pane, index);
        }
        pane->views.remove(path);
    }
    // Detach both visible and hidden editors before releasing an owned document
    // or its external binding. Empty pane views must not keep stale documents.
    if (textDocument) {
        for (Pane *pane : {m_primary.get(), m_secondary.get()})
            detachDocument(pane, textDocument);
    }
    m_documents.erase(found);
    if (textDocument) {
        disconnect(textDocument, nullptr, this, nullptr);
        if (owned)
            delete textDocument;
    }
    emit documentsChanged();
    emit currentChanged();
}

void FileEditorWorkspace::clearDocuments()
{
    const QStringList openPaths = paths();
    for (const QString &path : openPaths)
        removeDocument(path);
}

void FileEditorWorkspace::split(Qt::Orientation orientation)
{
    const bool layoutChanged = m_splitter->orientation() != orientation || m_secondary->widget->isHidden();
    m_splitter->setOrientation(orientation);
    m_secondary->widget->show();
    if (layoutChanged) {
        const int extent = orientation == Qt::Horizontal ? m_splitter->width() : m_splitter->height();
        const int paneExtent = qMax(1, (extent - m_splitter->handleWidth()) / 2);
        m_splitter->setSizes({paneExtent, paneExtent});
    }
}

bool FileEditorWorkspace::openInOtherPane(const QString &path)
{
    if (!contains(path))
        return false;
    Pane *other = m_active == m_primary.get() ? m_secondary.get() : m_primary.get();
    split(m_splitter->orientation());
    for (int index = 0; index < other->tabs->count(); ++index) {
        if (other->tabs->tabData(index).toString() == path) {
            select(other, path);
            return true;
        }
    }
    addTab(other, path);
    return true;
}

bool FileEditorWorkspace::moveToOtherPane(const QString &path)
{
    Pane *source = m_active;
    saveView(source);
    Pane *target = source == m_primary.get() ? m_secondary.get() : m_primary.get();
    if (!target->views.contains(path) && source->views.contains(path))
        target->views.insert(path, source->views.value(path));
    if (!openInOtherPane(path))
        return false;
    for (int index = 0; index < source->tabs->count(); ++index) {
        if (source->tabs->tabData(index).toString() == path) {
            removeTab(source, index);
            break;
        }
    }
    emit currentChanged();
    return true;
}

void FileEditorWorkspace::singlePane()
{
    saveView(m_secondary.get());
    const QString selectedPath = activePath();
    const bool restoreSecondaryView = m_active == m_secondary.get()
        && m_secondary->views.contains(selectedPath);
    const Pane::ViewState secondaryView = m_secondary->views.value(selectedPath);
    QStringList secondaryPaths;
    for (int index = 0; index < m_secondary->tabs->count(); ++index)
        secondaryPaths.append(m_secondary->tabs->tabData(index).toString());
    for (const QString &path : secondaryPaths) {
        bool alreadyOpen = false;
        for (int index = 0; index < m_primary->tabs->count(); ++index)
            alreadyOpen |= m_primary->tabs->tabData(index).toString() == path;
        if (!alreadyOpen) {
            m_primary->views.insert(path, m_secondary->views.value(path));
            addTab(m_primary.get(), path);
        }
    }
    while (m_secondary->tabs->count() > 0)
        removeTab(m_secondary.get(), 0);
    m_secondary->widget->hide();
    m_active = m_primary.get();
    if (!selectedPath.isEmpty())
        select(m_primary.get(), selectedPath);
    if (restoreSecondaryView) {
        SourceEditor *editor = activeEditor();
        if (editor && secondaryView.cursor.document() == editor->document()) {
            editor->setTextCursor(secondaryView.cursor);
            editor->verticalScrollBar()->setValue(secondaryView.verticalScroll);
            editor->horizontalScrollBar()->setValue(secondaryView.horizontalScroll);
            saveView(m_primary.get());
        }
    }
    emit currentChanged();
}

bool FileEditorWorkspace::eventFilter(QObject *object, QEvent *event)
{
    if (event->type() == QEvent::FocusIn || event->type() == QEvent::MouseButtonPress) {
        for (Pane *pane : {m_primary.get(), m_secondary.get()}) {
            if (pane && pane->surfaces().contains(qobject_cast<QWidget *>(object))) {
                m_active = pane;
                emit currentChanged();
                break;
            }
        }
    }
    return QWidget::eventFilter(object, event);
}
