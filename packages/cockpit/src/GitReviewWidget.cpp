#include "GitReviewWidget.h"
#include "WorkspaceFlightView.h"
#include "SourceEditor.h"
#include "ProjectFilesWidget.h"
#include <QApplication>
#include <QComboBox>
#include <QDir>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScrollBar>
#include <QSignalBlocker>
#include <QSplitter>
#include <QStackedWidget>
#include <QTabWidget>
#include <QTextBlock>
#include <QTreeWidget>
#include <QVBoxLayout>
#include <QKeyEvent>
#include <QPainter>

namespace {
QLabel *plainLabel(const QString &text = {})
{
    auto *label = new QLabel(text);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    return label;
}
QPushButton *button(QBoxLayout *layout, const QString &text, const QString &name = {})
{
    auto *result = new QPushButton(text);
    result->setObjectName(name);
    layout->addWidget(result);
    return result;
}
}

GitReviewWidget::GitReviewWidget(bool hardware, QWidget *parent) : QWidget(parent), m_client(this)
{
    setObjectName("gitReviewUniverse");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(6, 6, 6, 6);
    auto *navigation = new QHBoxLayout;
    connect(button(navigation, tr("Orbit")), &QPushButton::clicked, this, [this]() {
        if (confirmLeave())
            emit exitRequested();
    });
    connect(button(navigation, tr("Git hub")), &QPushButton::clicked, this, &GitReviewWidget::showHub);
    connect(button(navigation, tr("Review all changes"), "gitBeginReview"), &QPushButton::clicked, this, &GitReviewWidget::beginReview);
    connect(button(navigation, tr("Return to space"), "gitReturnSpace"), &QPushButton::clicked, this, &GitReviewWidget::returnToSpace);
    connect(button(navigation, tr("Decision chamber")), &QPushButton::clicked, this, &GitReviewWidget::showSummary);
    m_refresh = button(navigation, tr("Refresh"), "gitReviewRefresh");
    connect(m_refresh, &QPushButton::clicked, this, &GitReviewWidget::refresh);
    auto *cancel = button(navigation, tr("Cancel"));
    connect(cancel, &QPushButton::clicked, &m_client, &ProjectGitClient::cancel);
    navigation->addStretch();
    layout->addLayout(navigation);
    m_location = plainLabel(tr("Git Planet · select a project"));
    layout->addWidget(m_location);
    auto *body = new QSplitter;
    auto *sidebar = new QWidget;
    auto *filesLayout = new QVBoxLayout(sidebar);
    filesLayout->setContentsMargins(0, 0, 4, 0);
    auto *search = new QLineEdit;
    search->setPlaceholderText(tr("Jump to file…"));
    search->setObjectName("gitReviewSearch");
    filesLayout->addWidget(search);
    m_files = new QListWidget;
    m_files->setObjectName("gitReviewFiles");
    filesLayout->addWidget(m_files);
    connect(search, &QLineEdit::textChanged, this, [this](const QString &text) {
        for (int row = 0; row < m_files->count(); ++row)
            m_files->item(row)->setHidden(!m_model.files[row].path.contains(text, Qt::CaseInsensitive));
    });
    connect(m_files, &QListWidget::itemActivated, this, [this](QListWidgetItem *item) { jumpToFile(m_files->row(item)); });
    connect(m_files, &QListWidget::itemClicked, this, [this](QListWidgetItem *item) { jumpToFile(m_files->row(item)); });
    sidebar->setMinimumWidth(180);
    body->addWidget(sidebar);
    m_pages = new QStackedWidget;
    body->addWidget(m_pages);
    body->setStretchFactor(1, 1);
    body->setSizes({230, 1100});
    layout->addWidget(body, 1);

    m_hub = new QWidget;
    auto *hubLayout = new QVBoxLayout(m_hub);
    m_briefing = plainLabel();
    m_briefing->setObjectName("gitReviewBriefing");
    hubLayout->addWidget(m_briefing);
    connect(button(hubLayout, tr("Working Changes → Enter review gauntlet")), &QPushButton::clicked, this, &GitReviewWidget::beginReview);
    auto *catalog = new QTabWidget;
    m_branches = new QTreeWidget;
    m_branches->setHeaderLabels({tr("Branch"), tr("Commit"), tr("Upstream / divergence")});
    catalog->addTab(m_branches, tr("Branches"));
    m_history = new QTreeWidget;
    m_history->setHeaderLabels({tr("Commit"), tr("Subject"), tr("Author / parents")});
    catalog->addTab(m_history, tr("History"));
    catalog->addTab(plainLabel(tr("Pull request district\n\nThe current project Git service has no connected PR provider. Local review works without a remote or account. No remote comments, approvals or merges are available.")), tr("Pull requests"));
    hubLayout->addWidget(catalog, 1);
    m_pages->addWidget(m_hub);

    m_scene = new WorkspaceFlightView({}, hardware);
    m_scene->setObjectName("gitReviewFlight");
    m_scene->setShipVisible(false);
    m_scene->destinationActivated = [this](const QString &path, bool) {
        if (path == "display:next") {
            if (m_first + GitReviewWorld::filesPerSection < m_model.files.size())
                jumpToFile(m_first + GitReviewWorld::filesPerSection);
            else
                showSummary();
            return;
        }
        if (path.startsWith("display:before-") || path.startsWith("display:after-")) {
            bool valid = false;
            const int index = path.mid(path.lastIndexOf('-') + 1).toInt(&valid);
            if (valid && index >= 0 && index < m_model.files.size() && !busy()) {
                if (index != m_current) {
                    m_current = index;
                    m_hunk = 0;
                    loadCurrent(true);
                } else if (m_diff.path == m_model.files[index].path) {
                    inspectHunk(m_hunk);
                } else {
                    loadCurrent(true);
                }
            }
        }
    };
    m_scene->orbitRequested = [this]() { goBack(); };
    m_scene->exitRequested = [this]() { showHub(); };
    m_scene->installEventFilter(this);
    m_pages->addWidget(m_scene);

    m_focus = new QWidget;
    auto *focusLayout = new QVBoxLayout(m_focus);
    auto *hunks = new QHBoxLayout;
    connect(button(hunks, tr("Previous file")), &QPushButton::clicked, this, [this]() { jumpToFile(m_current - 1); });
    connect(button(hunks, tr("Previous hunk")), &QPushButton::clicked, this, [this]() { inspectHunk(m_hunk - 1); });
    connect(button(hunks, tr("Next hunk")), &QPushButton::clicked, this, [this]() { inspectHunk(m_hunk + 1); });
    connect(button(hunks, tr("Next file")), &QPushButton::clicked, this, [this]() { jumpToFile(m_current + 1); });
    m_comparison = new QComboBox;
    m_comparison->setObjectName("gitReviewComparison");
    m_comparison->addItem(tr("Index → Working tree"), int(GitComparison::Unstaged));
    m_comparison->addItem(tr("HEAD → Index"), int(GitComparison::Staged));
    m_comparison->addItem(tr("HEAD → Working tree"), int(GitComparison::Combined));
    hunks->addWidget(m_comparison);
    connect(m_comparison, &QComboBox::activated, this, [this]() { m_hunk = 0; loadCurrent(true); });
    auto *context = new QComboBox;
    context->addItem(tr("3 context lines"), 3);
    context->addItem(tr("20 context lines"), 20);
    context->addItem(tr("1000 context lines"), 1000);
    hunks->addWidget(context);
    connect(context, &QComboBox::activated, this, [this, context]() { m_context = context->currentData().toInt(); loadCurrent(true); });
    focusLayout->addLayout(hunks);
    auto *comparisonPanes = new QSplitter;
    auto pane = [&](SourceEditor *&editor, QLabel *&label, const QString &name) {
        auto *container = new QWidget;
        auto *paneLayout = new QVBoxLayout(container);
        label = plainLabel();
        paneLayout->addWidget(label);
        editor = new SourceEditor;
        editor->setObjectName(name);
        editor->setReadOnly(true);
        paneLayout->addWidget(editor);
        comparisonPanes->addWidget(container);
    };
    pane(m_before, m_beforeLabel, "gitReviewBefore");
    pane(m_after, m_afterLabel, "gitReviewAfter");
    focusLayout->addWidget(comparisonPanes, 1);
    connect(m_before->verticalScrollBar(), &QScrollBar::valueChanged, m_after->verticalScrollBar(), &QScrollBar::setValue);
    connect(m_after->verticalScrollBar(), &QScrollBar::valueChanged, m_before->verticalScrollBar(), &QScrollBar::setValue);
    connect(m_before->horizontalScrollBar(), &QScrollBar::valueChanged, m_after->horizontalScrollBar(), &QScrollBar::setValue);
    connect(m_after->horizontalScrollBar(), &QScrollBar::valueChanged, m_before->horizontalScrollBar(), &QScrollBar::setValue);
    auto *actions = new QHBoxLayout;
    m_reviewed = button(actions, tr("Mark comparison reviewed"), "gitMarkReviewed");
    connect(m_reviewed, &QPushButton::clicked, this, [this]() {
        if (m_current < 0 || m_diff.path != m_model.files[m_current].path || busy() || m_diff.limited || m_diff.conflict)
            return;
        if (!m_progress.mark(m_model.files[m_current], comparison(), m_hunk)) {
            m_status->setText(tr("The local review history is full (2048 records). This comparison was not marked."));
            return;
        }
        updateFileList();
        // Preserve the room and position; a completion stripe will update when
        // returning to the spatial view, outside any active flight frame.
        m_status->setText(tr("Comparison marked reviewed for this content revision."));
    });
    m_stage = button(actions, tr("Stage file"), "gitStageFile");
    m_unstage = button(actions, tr("Unstage file"), "gitUnstageFile");
    m_discard = button(actions, tr("Discard working changes…"), "gitDiscardFile");
    connect(m_stage, &QPushButton::clicked, this, [this]() { mutate("stage"); });
    connect(m_unstage, &QPushButton::clicked, this, [this]() { mutate("unstage"); });
    connect(m_discard, &QPushButton::clicked, this, [this]() { mutate("discard"); });
    connect(button(actions, tr("Open editor")), &QPushButton::clicked, this, &GitReviewWidget::openEditor);
    focusLayout->addLayout(actions);
    m_notes = new QPlainTextEdit;
    m_notes->setPlaceholderText(tr("Local review notes (not published). Maximum 4096 characters."));
    m_notes->setMaximumHeight(90);
    connect(m_notes, &QPlainTextEdit::textChanged, this, [this]() {
        if (m_notes->toPlainText().size() > 4096) {
            const int position = m_notes->textCursor().position();
            m_notes->setPlainText(m_notes->toPlainText().left(4096));
            QTextCursor cursor = m_notes->textCursor();
            cursor.setPosition(qMin(position, 4096));
            m_notes->setTextCursor(cursor);
            m_notes->document()->setModified(true);
        }
    });
    m_notes->setObjectName("gitReviewNotes");
    focusLayout->addWidget(m_notes);
    connect(button(focusLayout, tr("Save local note")), &QPushButton::clicked, this, &GitReviewWidget::saveNote);
    m_pages->addWidget(m_focus);

    m_summary = new QWidget;
    auto *summaryLayout = new QVBoxLayout(m_summary);
    m_summaryText = plainLabel();
    summaryLayout->addWidget(m_summaryText);
    m_message = new QLineEdit;
    m_message->setObjectName("gitReviewCommitMessage");
    m_message->setMaxLength(4096);
    m_message->setPlaceholderText(tr("Commit message for the current staged index…"));
    summaryLayout->addWidget(m_message);
    m_commit = button(summaryLayout, tr("Commit staged changes…"), "gitCommitIndex");
    connect(m_commit, &QPushButton::clicked, this, [this]() { mutate("commit-index"); });
    connect(m_message, &QLineEdit::textChanged, this, &GitReviewWidget::updateButtons);
    connect(button(summaryLayout, tr("Return to review")), &QPushButton::clicked, this, &GitReviewWidget::returnToSpace);
    summaryLayout->addStretch();
    m_pages->addWidget(m_summary);
    m_status = plainLabel();
    m_status->setObjectName("gitReviewStatus");
    layout->addWidget(m_status);
    connect(&m_client, &ProjectGitClient::busyChanged, this, [this, cancel](bool busy) {
        cancel->setVisible(busy);
        updateButtons();
    });
    cancel->hide();
    m_positionTimer.setInterval(100);
    connect(&m_positionTimer, &QTimer::timeout, this, &GitReviewWidget::trackPosition);
    updateButtons();
}

bool GitReviewWidget::spatial() const { return m_pages->currentWidget() == m_scene; }
GitComparison GitReviewWidget::comparison() const { return GitComparison(m_comparison->currentData().toInt()); }
void GitReviewWidget::setHardwareAcceleration(bool hardware) { m_scene->setFlightHardwareAcceleration(hardware); }

void GitReviewWidget::setWorkdir(const QString &root)
{
    if (root == m_workdir)
        return;
    ++m_generation;
    m_client.cancel();
    m_workdir = root;
    m_model = {};
    m_diff = {};
    m_current = -1;
    m_first = 0;
    m_files->clear();
    m_pages->setCurrentWidget(m_hub);
    m_briefing->setText(tr("Loading the active local repository…"));
    if (isVisible())
        refresh();
}

void GitReviewWidget::run(QJsonObject request, ProjectGitClient::Completion completion)
{
    const quint64 generation = m_generation;
    m_client.run(m_workdir, request, [this, generation, completion = std::move(completion)](bool ok, const QJsonObject &reply, const QString &error) {
        if (generation != m_generation) {
            if (isVisible())
                QTimer::singleShot(0, this, &GitReviewWidget::refresh);
            return;
        }
        if (!ok)
            m_status->setText(error);
        completion(ok, reply, error);
        updateButtons();
    });
}

void GitReviewWidget::refresh()
{
    if (busy() || m_workdir.isEmpty())
        return;
    m_status->setText(tr("Reading saved working changes and content identities…"));
    saveNote();
    const QString selected = m_current >= 0 && m_current < m_model.files.size() ? m_model.files[m_current].path : QString();
    run({{"action", "review-status"}}, [this, selected](bool ok, const QJsonObject &reply, const QString &) {
        if (!ok) {
            m_model = {};
            m_diff = {};
            m_current = -1;
            m_files->clear();
            m_briefing->setText(tr("Select a local Git repository. No remote, account or initial commit is required."));
            m_pages->setCurrentWidget(m_hub);
            return;
        }
        QString error;
        GitReviewModel updated;
        if (!GitReviewModel::parse(reply, updated, error)) {
            m_status->setText(error);
            return;
        }
        const bool same = updated.snapshot == m_model.snapshot;
        m_model = updated;
        m_progress.load(m_model.root);
        const QString restore = selected.isEmpty() ? m_progress.lastPath() : selected;
        m_current = -1;
        for (int index = 0; index < m_model.files.size(); ++index) {
            if (m_model.files[index].path == restore)
                m_current = index;
        }
        if (m_current < 0 && !m_model.files.isEmpty())
            m_current = 0;
        if (!same)
            m_diff = {};
        updateFileList();
        m_briefing->setText(tr("WORKING CHANGES\n%1\nBranch: %2\n%3 changed files · %4 reviewed\n\nWalk through staged and unstaged versions independently. HEAD → Working tree is also available; it never replaces a hidden staged version.\nSaved files only. Conflicts and binary changes are explicitly labeled.")
            .arg(m_model.root, m_model.branch.isEmpty() ? tr("Detached HEAD") : m_model.branch).arg(m_model.files.size()).arg(reviewedCount()));
        m_status->setText(reply.value("notice").toString(tr("Ready. Content changes invalidate prior review marks.")));
        if (spatial() && m_current >= 0) {
            m_first = m_current / GitReviewWorld::filesPerSection * GitReviewWorld::filesPerSection;
            publishSection(same);
            loadCurrent(false);
        } else if (m_pages->currentWidget() == m_focus && m_current >= 0) {
            loadCurrent(true);
        } else if (m_pages->currentWidget() == m_summary) {
            showSummary();
        } else {
            run({{"action", "catalog"}, {"snapshot", m_model.snapshot}}, [this](bool success, const QJsonObject &catalogReply, const QString &) {
                if (!success)
                    return;
                const auto catalog = catalogReply.value("catalog").toObject();
                m_history->clear();
                m_branches->clear();
                for (const auto &value : catalog.value("branches").toArray()) {
                    const auto branch = value.toObject();
                    new QTreeWidgetItem(m_branches, {branch.value("name").toString(), branch.value("id").toString().left(12),
                        branch.value("upstream").toString() + " " + branch.value("divergence").toString()});
                }
                for (const auto &value : catalog.value("commits").toArray()) {
                    const auto commit = value.toObject();
                    QStringList parents;
                    for (const auto &parent : commit.value("parents").toArray())
                        parents.append(parent.toString().left(12));
                    auto *item = new QTreeWidgetItem(m_history, {commit.value("id").toString().left(12), commit.value("subject").toString(), commit.value("author").toString()});
                    new QTreeWidgetItem(item, {tr("Parents"), parents.join(", ")});
                    item->setToolTip(0, commit.value("id").toString());
                }
                QStringList remotes;
                for (const auto &remote : catalog.value("remotes").toArray())
                    remotes.append(remote.toString());
                m_status->setText(tr("Remotes: %1. %2").arg(remotes.isEmpty() ? tr("none — local review available") : remotes.join(", "), catalog.value("notice").toString()));
            });
        }
    });
}

int GitReviewWidget::reviewedCount() const
{
    int count = 0;
    for (const auto &file : m_model.files)
        count += m_progress.reviewed(file);
    return count;
}

void GitReviewWidget::updateFileList()
{
    const QSignalBlocker block(m_files);
    m_files->clear();
    for (const auto &file : m_model.files) {
        QString state = tr("Pending");
        if (file.conflict())
            state = tr("CONFLICT");
        else if (m_progress.stale(file))
            state = tr("STALE");
        else if (m_progress.reviewed(file))
            state = tr("Reviewed");
        auto *item = new QListWidgetItem(state + " · " + file.status + "\n" + file.path, m_files);
        item->setHidden(!file.path.contains(findChild<QLineEdit *>("gitReviewSearch")->text(), Qt::CaseInsensitive));
        item->setToolTip(file.path + (file.originalPath.isEmpty() ? QString() : tr("\nRenamed from %1").arg(file.originalPath)));
    }
    if (m_current >= 0)
        m_files->setCurrentRow(m_current);
    if (m_pages->currentWidget() != m_focus)
        m_location->setText(tr("Git Planet · %1 · %2/%3 files reviewed").arg(m_model.branch).arg(reviewedCount()).arg(m_model.files.size()));
}

void GitReviewWidget::beginReview()
{
    if (!busy() && !m_model.files.isEmpty())
        jumpToFile(qMax(0, m_current));
}

void GitReviewWidget::publishSection(bool preservePosition, bool continueWalking)
{
    m_world = GitReviewWorld::build(m_model, m_progress, m_first);
    if (!m_world.world.success) {
        m_status->setText(m_world.world.diagnostic);
        m_pages->setCurrentWidget(m_hub);
        return;
    }
    auto room = m_world.world.interior;
    if (m_current >= m_first && m_current - m_first < m_world.filePositions.size()) {
        const auto anchor = m_world.filePositions[m_current - m_first];
        room.spawn = {anchor.x(), anchor.z() - 650};
    }
    room.environmentKey = "git:" + m_model.root + ":" + QString::number(m_first);
    if (continueWalking) {
        m_scene->streamInterior(room, tr("Working Changes review"),
            {float(room.spawn.x()), 160, float(room.spawn.y())});
    } else {
        m_scene->setInterior(room, tr("Working Changes review"), preservePosition, preservePosition, room.environmentKey);
    }
    m_scene->setShipVisible(false);
    for (int slot = 0; slot < m_world.filePositions.size(); ++slot) {
        const int index = m_first + slot;
        const auto &file = m_model.files[index];
        const auto *diff = m_diff.path == file.path && m_diff.revision == file.revision ? &m_diff : nullptr;
        if (diff && index == m_current)
            continue;
        m_scene->setInteriorImage("before-" + QString::number(index), GitReviewWorld::codeSurface(file, diff, m_hunk, true));
        m_scene->setInteriorImage("after-" + QString::number(index), GitReviewWorld::codeSurface(file, diff, m_hunk, false));
    }
    GitReviewFile gate;
    gate.path = m_first + GitReviewWorld::filesPerSection < m_model.files.size() ? tr("NEXT REVIEW SECTION →") : tr("DECISION CHAMBER →");
    GitReviewDiff gateText;
    gateText.beforeLabel = tr("WALK FORWARD");
    gateText.notice = tr("%1 of %2 files reviewed.\nContinue through the changes or jump to any file in the list.").arg(reviewedCount()).arg(m_model.files.size());
    m_scene->setInteriorImage("next", GitReviewWorld::codeSurface(gate, &gateText, 0, true));
    prepareCodePreviews();
}

void GitReviewWidget::prepareCodePreviews()
{
    m_beforePreview = {};
    m_afterPreview = {};
    m_previewFile = -1;
    if (m_current < 0 || m_current >= m_model.files.size() || m_diff.path != m_model.files[m_current].path)
        return;
    const auto &file = m_model.files[m_current];
    m_beforePreview = GitReviewWorld::codePreview(file, &m_diff, m_hunk, true);
    m_afterPreview = GitReviewWorld::codePreview(file, &m_diff, m_hunk, false);
    m_previewFile = m_current;
    m_beforeOffset = -1;
    m_afterOffset = -1;
    m_previewClock.start();
    m_previewMilliseconds = 0;
    advanceCodePreviews();
}

void GitReviewWidget::advanceCodePreviews()
{
    if (!spatial() || m_previewFile != m_current || m_previewFile < 0)
        return;
    const auto delta = m_previewClock.restart();
    if (m_scene->previewAutoScroll())
        m_previewMilliseconds += delta;
    const auto elapsed = m_previewMilliseconds;
    const auto update = [&](const ScrollingPreview &preview, int &previousOffset, const QString &side) {
        const int offset = preview.offsetAt(elapsed);
        if (offset == previousOffset)
            return;
        previousOffset = offset;
        m_scene->setInteriorImage(side + QString::number(m_current), preview.imageAt(elapsed));
    };
    update(m_beforePreview, m_beforeOffset, "before-");
    update(m_afterPreview, m_afterOffset, "after-");
}

void GitReviewWidget::jumpToFile(int index, bool continueWalking)
{
    if (busy() || index < 0 || index >= m_model.files.size())
        return;
    if (m_editor && !m_editor->confirmLeave())
        return;
    saveNote();
    m_current = index;
    m_first = index / GitReviewWorld::filesPerSection * GitReviewWorld::filesPerSection;
    m_hunk = m_model.files[index].path == m_progress.lastPath() ? m_progress.lastHunk() : 0;
    m_diff = {};
    const bool resume = m_model.files[index].path == m_progress.lastPath();
    m_comparison->setCurrentIndex(resume ? int(m_progress.lastComparison()) : (m_model.files[index].unstaged() ? 0 : 1));
    m_pages->setCurrentWidget(m_scene);
    publishSection(false, continueWalking);
    if (!m_world.world.success)
        return;
    m_scene->setFocus();
    loadCurrent(false);
}

void GitReviewWidget::loadCurrent(bool focus)
{
    if (busy() || m_current < 0 || m_current >= m_model.files.size())
        return;
    const auto file = m_model.files[m_current];
    m_focusAfterLoad = focus;
    m_status->setText(tr("Loading %1 · %2…").arg(file.path, GitReviewModel::comparisonName(comparison())));
    run({{"action", "compare"}, {"snapshot", m_model.snapshot}, {"paths", QJsonArray{file.path}},
        {"comparison", GitReviewModel::comparisonName(comparison())}, {"context", m_context}},
        [this, file](bool ok, const QJsonObject &reply, const QString &) {
            if (!ok) {
                m_diff = {};
                updateButtons();
                return;
            }
            QString error;
            if (!GitReviewDiff::parse(reply.value("review").toObject(), m_diff, error) || m_diff.revision != file.revision) {
                m_status->setText(error.isEmpty() ? tr("The comparison revision changed. Refresh before reviewing.") : error);
                m_diff = {};
                return;
            }
            m_hunk = qBound(0, m_hunk, qMax(0, int(m_diff.hunks.size()) - 1));
            prepareCodePreviews();
            m_status->setText(m_diff.notice.isEmpty() ? tr("%1 → %2 · %3 hunks · E / A or click a wall to inspect").arg(m_diff.beforeLabel, m_diff.afterLabel).arg(m_diff.hunks.size()) : m_diff.notice);
            m_progress.visit(file, m_hunk, comparison());
            // Loading code changes selection, not review status. Rebuilding up
            // to 2048 list rows and rehashing review marks here stalls walking.
            m_files->setCurrentRow(m_current);
            if (m_focusAfterLoad)
                inspectHunk(m_hunk);
        });
}

void GitReviewWidget::inspectHunk(int index)
{
    if (busy() || m_current < 0 || m_diff.path != m_model.files[m_current].path)
        return;
    saveNote();
    m_hunk = qBound(0, index, qMax(0, int(m_diff.hunks.size()) - 1));
    m_scene->suspendPilot();
    m_pages->setCurrentWidget(m_focus);
    m_progress.visit(m_model.files[m_current], m_hunk, comparison());
    updateCode();
    updateButtons();
}

void GitReviewWidget::updateCode()
{
    m_beforeLabel->setText(m_diff.beforeLabel);
    m_afterLabel->setText(m_diff.afterLabel);
    const GitReviewHunk hunk = m_diff.hunks.isEmpty() ? GitReviewHunk{} : m_diff.hunks[m_hunk];
    auto fill = [&](SourceEditor *editor, bool before) {
        QStringList lines;
        QVector<int> mapping;
        for (const auto &row : hunk.rows) {
            lines.append(before ? row.before : row.after);
            mapping.append(before ? row.beforeLine : row.afterLine);
        }
        editor->setFileName(m_diff.path);
        editor->setPlainText(lines.join('\n'));
        editor->setLineMapping(mapping);
        for (int index = 0; index < hunk.rows.size(); ++index) {
            const auto &row = hunk.rows[index];
            if (!(before ? row.removed : row.added))
                continue;
            QTextCursor cursor(editor->document()->findBlockByNumber(index));
            QTextBlockFormat format;
            format.setBackground(before ? QColor(125, 45, 59, 85) : QColor(36, 124, 84, 85));
            cursor.setBlockFormat(format);
        }
    };
    fill(m_before, true);
    fill(m_after, false);
    m_notes->setPlainText(m_progress.note(m_model.files[m_current]));
    m_notes->document()->setModified(false);
    m_location->setText(tr("%1 · file %2/%3 · hunk %4/%5 · +%6 −%7\n%8")
        .arg(m_diff.path).arg(m_current + 1).arg(m_model.files.size()).arg(m_diff.hunks.isEmpty() ? 0 : m_hunk + 1)
        .arg(m_diff.hunks.size()).arg(hunk.additions).arg(hunk.deletions).arg(hunk.heading));
}

void GitReviewWidget::returnToSpace()
{
    if (busy() || m_current < 0)
        return;
    if (m_editor && !m_editor->confirmLeave())
        return;
    saveNote();
    m_pages->setCurrentWidget(m_scene);
    publishSection(true);
    m_scene->setFocus();
}

void GitReviewWidget::showHub()
{
    if (m_mutating || (m_editor && !m_editor->confirmLeave()))
        return;
    saveNote();
    m_focusAfterLoad = false;
    m_scene->suspendPilot();
    m_pages->setCurrentWidget(m_hub);
    if (!busy())
        refresh();
}

void GitReviewWidget::showSummary()
{
    if (m_mutating || (m_editor && !m_editor->confirmLeave()))
        return;
    saveNote();
    m_focusAfterLoad = false;
    m_scene->suspendPilot();
    m_pages->setCurrentWidget(m_summary);
    int staged = 0, conflicts = 0, stale = 0;
    for (const auto &file : m_model.files) {
        staged += file.staged();
        conflicts += file.conflict();
        stale += m_progress.stale(file);
    }
    m_summaryText->setText(tr("DECISION CHAMBER\n\n%1 / %2 files reviewed\n%3 remaining · %4 stale · %5 conflicts\n%6 staged files\n\nCommit uses exactly the current index. Working-tree edits are not silently staged. A changed snapshot will block the operation until refreshed.\n\nLocal notes stay on this machine. CI and remote discussions are unavailable without a provider.")
        .arg(reviewedCount()).arg(m_model.files.size()).arg(m_model.files.size() - reviewedCount()).arg(stale).arg(conflicts).arg(staged));
    updateButtons();
}

void GitReviewWidget::mutate(const QString &action)
{
    if (busy() || m_model.snapshot.isEmpty() || (m_editor && !m_editor->confirmLeave()))
        return;
    QJsonObject request{{"action", action}, {"snapshot", m_model.snapshot}};
    if (action == "commit-index") {
        if (m_message->text().trimmed().isEmpty())
            return;
        if (QMessageBox::question(this, tr("Commit staged changes"), tr("Commit the current staged index with this message? Unstaged edits remain unchanged.\n\n%1").arg(m_message->text()), QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
            return;
        request["message"] = m_message->text();
        request["confirm"] = true;
    } else {
        if (m_current < 0)
            return;
        request["paths"] = QJsonArray{m_model.files[m_current].path};
        if (action == "discard") {
            if (QMessageBox::question(this, tr("Discard working changes"), tr("Replace saved working-tree changes in %1 with its index version? This cannot be undone by DockPipe. Staged changes are preserved.").arg(m_model.files[m_current].path), QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
                return;
            request["confirm"] = true;
        }
    }
    saveNote();
    m_mutating = true;
    run(request, [this, action](bool ok, const QJsonObject &, const QString &) {
        m_mutating = false;
        if (ok) {
            if (action == "commit-index")
                m_message->clear();
            refresh();
        }
    });
}

void GitReviewWidget::saveNote()
{
    if (m_current >= 0 && m_current < m_model.files.size()
        && m_diff.path == m_model.files[m_current].path && m_pages->currentWidget() == m_focus
        && m_notes->document()->isModified()) {
        if (!m_progress.setNote(m_model.files[m_current], m_notes->toPlainText())) {
            m_status->setText(tr("The local review history is full (2048 records). This note could not be saved."));
            return;
        }
        m_notes->document()->setModified(false);
    }
}

void GitReviewWidget::openEditor()
{
    if (busy() || m_current < 0)
        return;
    saveNote();
    if (!m_editor) {
        m_editor = new ProjectFilesWidget;
        m_pages->addWidget(m_editor);
    }
    if (m_editor->setRoot(m_model.root) && m_editor->openFile(QDir(m_model.root).filePath(m_model.files[m_current].path)))
        m_pages->setCurrentWidget(m_editor);
}

void GitReviewWidget::updateButtons()
{
    const bool selected = m_current >= 0 && m_current < m_model.files.size();
    const bool ready = selected && !busy() && !m_model.files[m_current].conflict();
    m_reviewed->setEnabled(ready && !m_diff.limited && m_diff.path == m_model.files[m_current].path);
    m_stage->setEnabled(ready && m_model.files[m_current].unstaged());
    m_unstage->setEnabled(ready && m_model.files[m_current].staged());
    m_discard->setEnabled(ready && m_model.files[m_current].unstaged() && m_model.files[m_current].status != "??"
        && m_model.files[m_current].originalPath.isEmpty());
    bool staged = false, conflicts = false;
    for (const auto &file : m_model.files) {
        staged = staged || file.staged();
        conflicts = conflicts || file.conflict();
    }
    m_commit->setEnabled(!busy() && staged && !conflicts && !m_message->text().trimmed().isEmpty());
    m_refresh->setEnabled(!busy());
    m_comparison->setEnabled(!busy());
}

void GitReviewWidget::trackPosition()
{
    if (isVisible())
        advanceCodePreviews();
    if (!isVisible() || !spatial() || busy() || m_scene->furnishingActive() || !m_world.world.success || m_world.filePositions.isEmpty())
        return;
    const auto position = m_scene->cameraPosition();
    if (position.z() > m_world.filePositions.last().z() + 540) {
        if (m_first + GitReviewWorld::filesPerSection < m_model.files.size())
            jumpToFile(m_first + GitReviewWorld::filesPerSection, true);
        else
            showSummary();
        return;
    }
    for (int slot = 0; slot < m_world.filePositions.size(); ++slot) {
        if ((position - m_world.filePositions[slot]).length() < 420 && m_current != m_first + slot) {
            m_current = m_first + slot;
            m_hunk = 0;
            m_comparison->setCurrentIndex(m_model.files[m_current].unstaged() ? 0 : 1);
            loadCurrent(false);
            return;
        }
    }
}

void GitReviewWidget::goBack()
{
    if (!spatial()) {
        returnToSpace();
    } else if (m_first > 0) {
        jumpToFile(m_first - 1);
    } else {
        showHub();
    }
}
bool GitReviewWidget::confirmLeave()
{
    if (m_mutating) {
        m_status->setText(tr("Wait for the Git operation to finish, or cancel it before leaving."));
        return false;
    }
    saveNote();
    return !m_editor || m_editor->confirmLeave();
}
void GitReviewWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_previewClock.isValid())
        m_previewClock.restart();
    m_positionTimer.start();
    if (m_model.root.isEmpty())
        refresh();
}
void GitReviewWidget::hideEvent(QHideEvent *event)
{
    m_positionTimer.stop();
    saveNote();
    QWidget::hideEvent(event);
}
bool GitReviewWidget::eventFilter(QObject *object, QEvent *event)
{
    if (object == m_scene && !m_scene->furnishingActive() && event->type() == QEvent::KeyPress) {
        const auto *key = static_cast<QKeyEvent *>(event);
        if (key->key() == Qt::Key_PageDown || key->key() == Qt::Key_PageUp) {
            jumpToFile(m_current + (key->key() == Qt::Key_PageDown ? 1 : -1));
            return true;
        }
    }
    return QWidget::eventFilter(object, event);
}
