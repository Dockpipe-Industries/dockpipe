#include "GitConflictWidget.h"
#include "ProjectFilesWidget.h"

#include <QDir>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QSplitter>
#include <QVBoxLayout>

GitConflictWidget::GitConflictWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("gitConflictGuide");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    m_heading = new QLabel(tr("No operation in progress"));
    m_heading->setObjectName("gitConflictHeading");
    m_heading->setTextFormat(Qt::PlainText);
    m_heading->setWordWrap(true);
    layout->addWidget(m_heading);
    auto *guide = new QLabel(tr("1. Open each conflicted file, edit the conflict markers, and save.\n"
                               "2. Stage each saved resolution, or choose a version below.\n"
                               "3. Continue once all conflicts are resolved."));
    guide->setObjectName("editorMuted");
    guide->setWordWrap(true);
    layout->addWidget(guide);

    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    auto *sidebar = new QWidget;
    auto *sidebarLayout = new QVBoxLayout(sidebar);
    sidebarLayout->setContentsMargins(0, 0, 8, 0);
    sidebarLayout->addWidget(new QLabel(tr("Unresolved files")));
    m_conflicts = new QListWidget;
    m_conflicts->setObjectName("gitConflictFiles");
    sidebarLayout->addWidget(m_conflicts, 1);
    m_open = new QPushButton(tr("Open selected file"));
    m_open->setObjectName("gitConflictOpen");
    sidebarLayout->addWidget(m_open);
    connect(m_open, &QPushButton::clicked, this, &GitConflictWidget::openSelected);
    connect(m_conflicts, &QListWidget::itemDoubleClicked, this, [this] { openSelected(); });

    m_resolutionActions = new QWidget;
    auto *actions = new QVBoxLayout(m_resolutionActions);
    actions->setContentsMargins(0, 0, 0, 0);
    for (const auto &entry : QList<QPair<QString, QString>>{
             {tr("Stage saved resolution"), "resolve-stage"},
             {tr("Use ours"), "resolve-ours"}, {tr("Use theirs"), "resolve-theirs"},
             {tr("Resolve by deleting"), "resolve-delete"}}) {
        auto *button = new QPushButton(entry.first);
        button->setObjectName("gitConflict-" + entry.second);
        actions->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, action = entry.second] { resolveSelected(action); });
    }
    sidebarLayout->addWidget(m_resolutionActions);
    auto *versions = new QLabel(tr("Ours and theirs refer to Git's index versions for this operation. "
                                  "Choosing a version replaces the working file and stages it. "
                                  "For binary files, choose a version or resolve by deleting."));
    versions->setObjectName("editorMuted");
    versions->setWordWrap(true);
    sidebarLayout->addWidget(versions);
    splitter->addWidget(sidebar);
    m_editor = new ProjectFilesWidget;
    m_editor->setObjectName("gitConflictEditor");
    // This editor shares its width with the conflict list. Its preferred file
    // explorer width must not impose a minimum on every launcher page.
    m_editor->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    splitter->addWidget(m_editor);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({280, 800});
    layout->addWidget(splitter, 1);

    auto *operationActions = new QHBoxLayout;
    m_status = new QLabel;
    m_status->setObjectName("gitConflictStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    operationActions->addWidget(m_status, 1);
    m_continue = new QPushButton(tr("Continue"));
    m_continue->setObjectName("gitConflictContinue");
    operationActions->addWidget(m_continue);
    connect(m_continue, &QPushButton::clicked, this, [this] {
        const QString reviewedSnapshot = m_snapshotToken;
        if (confirmLeave())
            emit actionRequested(QJsonObject{{"action", "operation-continue"}, {"snapshot", reviewedSnapshot}});
    });
    m_skip = new QPushButton(tr("Skip current commit"));
    m_skip->setObjectName("gitConflictSkip");
    operationActions->addWidget(m_skip);
    connect(m_skip, &QPushButton::clicked, this, [this] {
        const QString reviewedSnapshot = m_snapshotToken;
        if (confirmLeave() && confirm(tr("Skip commit"), tr("Discard the current commit's conflict resolution and skip this commit?")))
            emit actionRequested(QJsonObject{{"action", "operation-skip"}, {"confirm", true}, {"snapshot", reviewedSnapshot}});
    });
    m_abort = new QPushButton(tr("Abort operation"));
    m_abort->setObjectName("gitConflictAbort");
    operationActions->addWidget(m_abort);
    connect(m_abort, &QPushButton::clicked, this, [this] {
        const QString reviewedSnapshot = m_snapshotToken;
        if (confirmLeave() && confirm(tr("Abort operation"), tr("Abort the active Git operation and restore its starting state? Saved conflict-resolution edits may be discarded.")))
            emit actionRequested(QJsonObject{{"action", "operation-abort"}, {"confirm", true}, {"snapshot", reviewedSnapshot}});
    });
    layout->addLayout(operationActions);
    connect(m_conflicts, &QListWidget::currentItemChanged, this, [this] { updateControls(); });
    connect(m_editor, &ProjectFilesWidget::dirtyChanged, this, [this] { updateControls(); });
    connect(m_editor, &ProjectFilesWidget::fileSaved, this, [this] { emit refreshRequested(); });
    updateControls();
}

void GitConflictWidget::setSnapshot(const QString &workdir, const QJsonObject &snapshot)
{
    if (m_workdir != workdir) {
        if (!m_editor->setRoot(workdir)) {
            m_status->setText(tr("Project change cancelled; the conflict editor still contains the previous project."));
            return;
        }
        m_workdir = workdir;
        m_reloadAfterResolution = false;
    }
    m_snapshotToken = snapshot.value("snapshot").toString();
    m_operation = snapshot.value("operation").toObject();
    const QString kind = m_operation.value("kind").toString();
    const QString current = m_operation.value("current").toString();
    const auto remaining = m_operation.value("remaining").toArray();
    QStringList remainingIds;
    for (const auto &id : remaining)
        remainingIds.append(id.toString());
    if (kind.isEmpty()) {
        m_heading->setText(tr("No operation in progress"));
        m_heading->setToolTip({});
    } else if (kind == "conflicts") {
        m_heading->setText(tr("Resolve the unmerged index, then commit the staged result"));
        m_heading->setToolTip({});
    } else {
        m_heading->setText(tr("%1 · current %2 · %3 remaining").arg(kind, current.left(12)).arg(remaining.size()));
        m_heading->setToolTip(tr("Current: %1\nRemaining:\n%2").arg(current, remainingIds.join('\n')));
    }
    QString selected;
    if (m_conflicts->currentItem())
        selected = m_conflicts->currentItem()->data(Qt::UserRole).toString();
    m_conflicts->clear();
    for (const auto &value : m_operation.value("conflicts").toArray()) {
        const QString path = value.toString();
        auto *item = new QListWidgetItem(path, m_conflicts);
        item->setData(Qt::UserRole, path);
        if (path == selected)
            m_conflicts->setCurrentItem(item);
    }
    if (!m_conflicts->currentItem() && m_conflicts->count() > 0)
        m_conflicts->setCurrentRow(0);
    if (m_reloadAfterResolution && !m_editor->hasUnsavedChanges()) {
        m_reloadAfterResolution = false;
        m_editor->reloadFile();
    }
    // The existing editor is deliberately not reloaded: saved or unsaved review
    // state must survive status refreshes and completion of individual stages.
    if (m_conflicts->count() > 0)
        m_status->setText(tr("%1 unresolved file(s)").arg(m_conflicts->count()));
    else if (kind.isEmpty())
        m_status->setText(tr("No conflicts to resolve."));
    else
        m_status->setText(tr("No unresolved paths. Continue when ready."));
    updateControls();
}

void GitConflictWidget::setBusy(bool busy)
{
    m_busy = busy;
    updateControls();
}

bool GitConflictWidget::confirmLeave()
{
    return m_editor->confirmLeave();
}

bool GitConflictWidget::hasUnsavedChanges() const
{
    return m_editor->hasUnsavedChanges();
}

bool GitConflictWidget::confirm(const QString &title, const QString &text)
{
    QMessageBox dialog(QMessageBox::Question, title, text, QMessageBox::Yes | QMessageBox::Cancel, this);
    dialog.setTextFormat(Qt::PlainText);
    dialog.setDefaultButton(QMessageBox::Cancel);
    return dialog.exec() == QMessageBox::Yes;
}

void GitConflictWidget::openSelected()
{
    auto *item = m_conflicts->currentItem();
    if (!item || m_busy)
        return;
    const QString path = item->data(Qt::UserRole).toString();
    if (!m_editor->openFile(QDir(m_workdir).filePath(path)))
        m_status->setText(tr("Could not open %1. Check the editor message; binary or missing files can be resolved by choosing a version or deleting.").arg(path));
    else
        m_status->setText(tr("Edit and save %1, then stage its resolution.").arg(path));
}

void GitConflictWidget::resolveSelected(const QString &action)
{
    auto *item = m_conflicts->currentItem();
    if (!item || m_busy)
        return;
    const QString path = item->data(Qt::UserRole).toString();
    const QString reviewedSnapshot = m_snapshotToken;
    if (!confirmLeave())
        return;
    QString question;
    if (action == "resolve-stage")
        question = tr("Stage the saved working-file resolution for %1? Verify that conflict markers have been removed.").arg(path);
    else if (action == "resolve-delete")
        question = tr("Delete %1 and stage the deletion as its conflict resolution?").arg(path);
    else
        question = tr("Replace %1 with Git's %2 version and stage that resolution?")
            .arg(path, action == "resolve-ours" ? tr("ours") : tr("theirs"));
    if (!confirm(tr("Resolve selected conflict"), question))
        return;
    if (action != "resolve-stage" && m_editor->currentPath() == QDir(m_workdir).filePath(path))
        m_reloadAfterResolution = true;
    emit actionRequested(QJsonObject{{"action", action}, {"paths", QJsonArray{path}}, {"confirm", true}, {"snapshot", reviewedSnapshot}});
}

void GitConflictWidget::updateControls()
{
    const bool active = !m_operation.value("kind").toString().isEmpty();
    const bool selected = m_conflicts->currentItem() != nullptr;
    m_conflicts->setEnabled(!m_busy);
    m_open->setEnabled(!m_busy && selected);
    m_editor->setEnabled(!m_busy);
    m_resolutionActions->setEnabled(!m_busy && selected && active);
    m_continue->setEnabled(!m_busy && active && m_operation.value("can_continue").toBool());
    m_skip->setEnabled(!m_busy && active && m_operation.value("can_skip").toBool());
    m_abort->setEnabled(!m_busy && active && m_operation.value("can_abort").toBool());
}
