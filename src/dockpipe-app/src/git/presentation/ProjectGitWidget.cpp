#include "ProjectGitWidget.h"
#include <QPointer>
#include <QTimer>
#include <QCoreApplication>
#include "CanvasIcons.h"
#include "FileExplorerIcons.h"
#include "GitDiffWidget.h"
#include "GitHistoryWidget.h"
#include "GitRepositoryWidget.h"
#include "GitConflictWidget.h"
#include "ProjectFilesWidget.h"
#include "RemoteGitContext.h"
#include <QTabWidget>
#include <QShowEvent>
#include <QHideEvent>
#include <QStackedWidget>
#include <QMessageBox>

#include <QApplication>
#include <QButtonGroup>
#include <QSettings>
#include <QSet>
#include <QHash>
#include <QTreeWidget>
#include <QTreeWidgetItemIterator>
#include <QToolButton>
#include <QStyle>
#include <QFontDatabase>
#include <QFileInfo>
#include <QPainter>
#include <QStyledItemDelegate>
#include <QTimer>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSplitter>
#include <QVBoxLayout>

namespace {
class ChangedFileDelegate final : public QStyledItemDelegate {
public:
    using QStyledItemDelegate::QStyledItemDelegate;
    QSize sizeHint(const QStyleOptionViewItem &option, const QModelIndex &index) const override
    {
        if (index.data(Qt::UserRole + 3).toBool())
            return QSize(260, 26);
        const bool folders = option.widget && option.widget->property("gitFolders").toBool();
        return QSize(260, folders ? 32 : 58);
    }
    void paint(QPainter *painter, const QStyleOptionViewItem &option, const QModelIndex &index) const override
    {
        if (index.data(Qt::UserRole + 3).toBool()) {
            QStyledItemDelegate::paint(painter, option, index);
            return;
        }
        const bool folders = option.widget && option.widget->property("gitFolders").toBool();
        QStyleOptionViewItem base(option);
        initStyleOption(&base, index);
        base.text.clear();
        base.icon = QIcon();
        QApplication::style()->drawControl(QStyle::CE_ItemViewItem, &base, painter, option.widget);
        painter->save();
        const QRect row = folders ? option.rect.adjusted(8, 3, -8, -3) : option.rect.adjusted(12, 8, -12, -8);
        const QString path = index.data(Qt::UserRole).toString();
        const QFileInfo info(path);
        FileExplorerIcons::instance().icon(info).paint(painter, QRect(row.left(), row.top() + 4, 18, 18));
        const int left = row.left() + 28;
        const int width = qMax(0, row.width() - 112);
        painter->setPen(option.palette.color(QPalette::Text));
        painter->drawText(QRect(left, row.top(), width, 22), Qt::AlignVCenter,
            option.fontMetrics.elidedText(info.fileName(), Qt::ElideMiddle, width));
        painter->setPen(option.palette.color(QPalette::PlaceholderText));
        auto small = option.font;
        small.setPointSizeF(qMax(8.0, small.pointSizeF() - 1));
        painter->setFont(small);
        if (!folders) {
            painter->drawText(QRect(left, row.top() + 23, width, 18), Qt::AlignVCenter,
                QFontMetrics(small).elidedText(info.path() == "." ? QCoreApplication::translate("ProjectGitWidget", "Project root") : info.path(), Qt::ElideMiddle, width));
        }
        const QString status = index.data(Qt::UserRole + 2).toString().trimmed();
        const bool dark = option.palette.color(QPalette::Base).lightness() < 128;
        const bool added = status == "??" || status.contains('A');
        QColor color = added ? QColor(dark ? "#77cfb2" : "#21795f") : QColor(dark ? "#e5ba74" : "#92600f");
        if (status.contains('D'))
            color = QColor(dark ? "#ee98a3" : "#b84154");
        const QRect badge(row.right() - 72, row.top() + 5, 72, 24);
        QColor fill = color;
        fill.setAlpha(24);
        painter->setPen(Qt::NoPen);
        painter->setBrush(fill);
        painter->drawRoundedRect(badge, 5, 5);
        painter->setPen(color);
        QString label = QCoreApplication::translate("ProjectGitWidget", "Modified");
        if (status == "??")
            label = QCoreApplication::translate("ProjectGitWidget", "New");
        else if (status.contains('U') || status == "AA" || status == "DD")
            label = QCoreApplication::translate("ProjectGitWidget", "Conflict");
        else if (status.contains('D'))
            label = QCoreApplication::translate("ProjectGitWidget", "Deleted");
        else if (status.contains('R'))
            label = QCoreApplication::translate("ProjectGitWidget", "Renamed");
        else if (added)
            label = QCoreApplication::translate("ProjectGitWidget", "Added");
        painter->drawText(badge, Qt::AlignCenter, label);
        painter->restore();
    }
};
}

ProjectGitWidget::ProjectGitWidget(QWidget *parent) : QWidget(parent), m_command(this)
{
    setObjectName("projectGit");
    setAttribute(Qt::WA_StyledBackground);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(24, 20, 24, 16);
    layout->setSpacing(18);
    auto *header = new QHBoxLayout;
    auto *identity = new QVBoxLayout;
    identity->setSpacing(4);
    auto *title = new QLabel(tr("Git"));
    title->setObjectName("gitHeading");
    identity->addWidget(title);
    auto *subtitle = new QLabel(tr("Review changes, commit your work, and keep branches in sync."));
    subtitle->setObjectName("gitSubtitle");
    subtitle->setWordWrap(true);
    identity->addWidget(subtitle);
    header->addLayout(identity, 1);
    m_heading = new QLabel(tr("No project"));
    m_heading->setObjectName("gitBranch");
    m_heading->setTextFormat(Qt::PlainText);
    m_heading->setMaximumWidth(280);
    header->addWidget(m_heading);
    m_controls = new QWidget;
    auto *controls = new QHBoxLayout(m_controls);
    controls->setContentsMargins(0, 0, 0, 0);
    const QList<CanvasIcons::Symbol> icons{CanvasIcons::Symbol::Refresh, CanvasIcons::Symbol::Download, CanvasIcons::Symbol::Upload};
    int iconIndex = 0;
    for (const auto &entry : QList<QPair<QString, QString>>{{tr("Refresh"), "status"}, {tr("Pull"), "pull"}, {tr("Push"), "push"}}) {
        auto *button = new QPushButton(CanvasIcons::icon(icons[iconIndex++]), entry.first);
        button->setToolTip(entry.second == "pull" ? tr("Pull fast-forward changes into a clean checkout")
            : entry.second == "push" ? tr("Push this branch to origin") : tr("Refresh saved project changes"));
        controls->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, action = entry.second]() { runAction(action); });
    }
    header->addWidget(m_controls);
    layout->addLayout(header);
    auto *summary = new QWidget;
    summary->setObjectName("gitSummary");
    auto *summaryLayout = new QHBoxLayout(summary);
    summaryLayout->setContentsMargins(18, 12, 18, 12);
    summaryLayout->setSpacing(32);
    for (const auto &entry : QList<QPair<QString, QString>>{
             {tr("Changed files"), "gitChangedCount"}, {tr("Staged files"), "gitStagedCount"},
             {tr("Conflicts"), "gitConflictCount"}}) {
        auto *metric = new QHBoxLayout;
        metric->setSpacing(10);
        auto *value = new QLabel(QString::fromUtf8("—"));
        value->setObjectName(entry.second);
        value->setProperty("gitMetric", true);
        metric->addWidget(value);
        auto *caption = new QLabel(entry.first);
        caption->setObjectName("gitMetricCaption");
        metric->addWidget(caption);
        summaryLayout->addLayout(metric);
    }
    summaryLayout->addStretch();
    layout->addWidget(summary);
    m_tabs = new QTabWidget;
    m_tabs->setObjectName("gitTabs");
    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    splitter->setObjectName("gitChangesSplitter");
    splitter->setHandleWidth(12);
    auto *changes = new QWidget;
    changes->setObjectName("gitChangesPanel");
    changes->setAttribute(Qt::WA_StyledBackground);
    auto *changesLayout = new QVBoxLayout(changes);
    changesLayout->setContentsMargins(14, 14, 14, 14);
    changesLayout->setSpacing(12);
    m_count = new QLabel(tr("Changed files"));
    m_count->setObjectName("gitPanelTitle");
    auto *changesHeader = new QHBoxLayout;
    changesHeader->addWidget(m_count, 1);
    auto *viewToggle = new QWidget;
    viewToggle->setObjectName("gitChangesViewToggle");
    auto *viewLayout = new QHBoxLayout(viewToggle);
    viewLayout->setContentsMargins(3, 3, 3, 3);
    viewLayout->setSpacing(2);
    auto *viewButtons = new QButtonGroup(this);
    for (bool folders : {false, true}) {
        auto *button = new QToolButton;
        button->setObjectName(folders ? "gitFoldersView" : "gitFlatView");
        button->setIcon(CanvasIcons::icon(folders ? CanvasIcons::Symbol::Folder : CanvasIcons::Symbol::List));
        button->setText(folders ? tr("Folders") : tr("List"));
        button->setToolTip(folders ? tr("Group changes by folder") : tr("Show a flat list of changed files"));
        button->setAccessibleName(button->toolTip());
        button->setCheckable(true);
        viewButtons->addButton(button);
        viewLayout->addWidget(button);
        connect(button, &QToolButton::clicked, this, [this, folders] { setFolderView(folders); });
    }
    changesHeader->addWidget(viewToggle);
    changesLayout->addLayout(changesHeader);
    auto *filter = new QLineEdit;
    filter->setObjectName("gitFileFilter");
    filter->setPlaceholderText(tr("Filter changed files…"));
    filter->setClearButtonEnabled(true);
    changesLayout->addWidget(filter);
    auto *groups = new QSplitter(Qt::Vertical);
    groups->setChildrenCollapsible(false);
    groups->setHandleWidth(10);
    for (bool staged : {false, true}) {
        auto *group = new QWidget;
        auto *groupLayout = new QVBoxLayout(group);
        groupLayout->setContentsMargins(0, 0, 0, 0);
        groupLayout->setSpacing(6);
        auto *heading = new QHBoxLayout;
        auto *label = new QLabel(staged ? tr("Staged · 0") : tr("Unstaged · 0"));
        label->setObjectName(staged ? "gitStagedHeading" : "gitUnstagedHeading");
        label->setProperty("gitGroupTitle", true);
        heading->addWidget(label, 1);
        auto *action = new QPushButton(staged ? tr("Unstage all") : tr("Stage all"));
        action->setObjectName(staged ? "gitUnstageAll" : "gitStageAll");
        connect(action, &QPushButton::clicked, this, [this, staged] { stageGroup(!staged); });
        heading->addWidget(action);
        groupLayout->addLayout(heading);
        auto *files = new QListWidget;
        files->setObjectName(staged ? "gitStagedFiles" : "gitChangedFiles");
        files->setMinimumWidth(290);
        files->setMinimumHeight(66);
        files->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Ignored);
        files->setItemDelegate(new ChangedFileDelegate(files));
        files->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
        files->setSpacing(2);
        auto *views = new QStackedWidget;
        views->addWidget(files);
        auto *tree = new QTreeWidget;
        tree->setObjectName(staged ? "gitStagedTree" : "gitUnstagedTree");
        tree->setProperty("gitFolders", true);
        tree->setHeaderHidden(true);
        tree->setIndentation(16);
        tree->setMinimumHeight(66);
        tree->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Ignored);
        tree->setItemDelegate(new ChangedFileDelegate(tree));
        tree->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
        tree->setContextMenuPolicy(Qt::NoContextMenu);
        views->addWidget(tree);
        groupLayout->addWidget(views, 1);
        if (staged) {
            m_stagedTree = tree;
            m_stagedViews = views;
        } else {
            m_unstagedTree = tree;
            m_unstagedViews = views;
        }
        auto selectTreeFile = [this, files](QTreeWidgetItem *item) {
            if (!item || item->data(0, Qt::UserRole + 3).toBool() || busy())
                return;
            const QString path = item->data(0, Qt::UserRole).toString();
            for (int row = 0; row < files->count(); ++row) {
                if (files->item(row)->data(Qt::UserRole).toString() == path) {
                    const QSignalBlocker blocker(files);
                    files->setCurrentRow(row);
                    selectFile(files);
                    return;
                }
            }
        };
        connect(tree, &QTreeWidget::currentItemChanged, this, [selectTreeFile](QTreeWidgetItem *item) { selectTreeFile(item); });
        connect(tree, &QTreeWidget::itemClicked, this, [selectTreeFile](QTreeWidgetItem *item) { selectTreeFile(item); });
        auto *empty = new QLabel(staged ? tr("Stage changes to include them in your commit.") : tr("No unstaged changes."));
        empty->setObjectName(staged ? "gitStagedEmpty" : "gitUnstagedEmpty");
        empty->setWordWrap(true);
        empty->setProperty("gitGroupHint", true);
        groupLayout->addWidget(empty);
        groups->addWidget(group);
        if (staged)
            m_staged = files;
        else
            m_files = files;
        connect(files, &QListWidget::currentItemChanged, this, [this, files](QListWidgetItem *item) {
            if (item)
                selectFile(files);
        });
        connect(files, &QListWidget::itemClicked, this, [this, files] { selectFile(files); });
    }
    m_activeFiles = m_files;
    changesLayout->addWidget(groups, 1);
    connect(filter, &QLineEdit::textChanged, this, &ProjectGitWidget::filterFiles);
    m_commitPanel = new QWidget;
    m_commitPanel->setObjectName("gitCommitPanel");
    auto *commitLayout = new QVBoxLayout(m_commitPanel);
    commitLayout->setContentsMargins(0, 14, 0, 0);
    commitLayout->setSpacing(12);
    auto *commitHeading = new QLabel(tr("Commit changes"));
    commitHeading->setObjectName("gitPanelTitle");
    commitLayout->addWidget(commitHeading);
    m_selection = new QLabel(tr("Select files to commit"));
    m_selection->setObjectName("editorMuted");
    commitLayout->addWidget(m_selection);
    m_message = new QLineEdit;
    m_message->setObjectName("gitCommitMessage");
    m_message->setPlaceholderText(tr("Describe your changes…"));
    m_message->setMaxLength(4096);
    commitLayout->addWidget(m_message);
    auto *commit = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Check), tr("Commit staged"));
    commit->setObjectName("gitCommitStaged");
    commitLayout->addWidget(commit);
    connect(commit, &QPushButton::clicked, this, [this] {
        runRequest(QJsonObject{{"action", "commit-index"}, {"message", m_message->text()}, {"confirm", true}});
    });
    auto *hint = new QLabel(tr("Only staged changes are committed. Unstaged edits stay in your working tree."));
    hint->setObjectName("editorMuted");
    hint->setWordWrap(true);
    commitLayout->addWidget(hint);
    changesLayout->addWidget(m_commitPanel);
    splitter->addWidget(changes);
    auto *review = new QWidget;
    review->setObjectName("gitReviewPanel");
    review->setAttribute(Qt::WA_StyledBackground);
    auto *reviewLayout = new QVBoxLayout(review);
    reviewLayout->setContentsMargins(0, 0, 0, 0);
    reviewLayout->setSpacing(0);
    m_diffTitle = new QLabel(tr("Select a file to review"));
    m_diffTitle->setObjectName("gitDiffTitle");
    m_diffTitle->setTextFormat(Qt::PlainText);
    m_diffTitle->setMargin(16);
    m_diffTitle->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    reviewLayout->addWidget(m_diffTitle);
    m_fileActions = new QWidget;
    m_fileActions->setObjectName("gitReviewToolbar");
    auto *fileActions = new QHBoxLayout(m_fileActions);
    fileActions->setContentsMargins(14, 10, 14, 10);
    fileActions->setSpacing(8);
    auto *comparisonLabel = new QLabel(tr("Unstaged changes"));
    comparisonLabel->setObjectName("gitComparisonLabel");
    fileActions->addWidget(comparisonLabel, 1);
    for (const auto &entry : QList<QPair<QString, QString>>{{tr("Stage"), "stage"}, {tr("Unstage"), "unstage"}, {tr("Discard"), "discard"}}) {
        auto *button = new QPushButton(entry.first);
        button->setObjectName("gitFile-" + entry.second);
        fileActions->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, action = entry.second] {
            if (!m_activeFiles->currentItem())
                return;
            const QString path = m_activeFiles->currentItem()->data(Qt::UserRole).toString();
            const auto reviewedSnapshot = m_snapshot.value("snapshot");
            if (action == "discard") {
                QMessageBox dialog(QMessageBox::Question, tr("Discard saved changes"),
                    tr("Discard the unstaged saved changes in %1? This cannot be undone.").arg(path),
                    QMessageBox::Yes | QMessageBox::Cancel, this);
                dialog.setTextFormat(Qt::PlainText);
                dialog.setDefaultButton(QMessageBox::Cancel);
                if (dialog.exec() != QMessageBox::Yes)
                    return;
            }
            runRequest(QJsonObject{{"action", action}, {"paths", QJsonArray{path}}, {"confirm", true}, {"snapshot", reviewedSnapshot}});
        });
    }
    fileActions->addStretch();
    reviewLayout->addWidget(m_fileActions);
    m_diff = new GitDiffWidget;
    m_diff->setObjectName("projectGitDiff");
    auto *reviewStack = new QStackedWidget;
    reviewStack->setObjectName("gitReviewStack");
    reviewStack->addWidget(m_diff);
    auto *empty = new QWidget;
    auto *emptyLayout = new QVBoxLayout(empty);
    emptyLayout->setContentsMargins(32, 32, 32, 32);
    emptyLayout->addStretch();
    auto *emptyTitle = new QLabel(tr("Your changes, in focus"));
    emptyTitle->setObjectName("gitEmptyTitle");
    emptyTitle->setAlignment(Qt::AlignCenter);
    emptyTitle->setWordWrap(true);
    emptyLayout->addWidget(emptyTitle);
    auto *emptyHint = new QLabel(tr("Refresh the repository, then select a file to review its changes."));
    emptyHint->setObjectName("gitEmptyHint");
    emptyHint->setAlignment(Qt::AlignCenter);
    emptyHint->setWordWrap(true);
    emptyLayout->addWidget(emptyHint);
    emptyLayout->addStretch();
    reviewStack->addWidget(empty);
    reviewStack->setCurrentIndex(1);
    reviewLayout->addWidget(reviewStack, 1);
    splitter->addWidget(review);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({350, 800});
    m_tabs->addTab(splitter, tr("Changes"));
    m_history = new GitHistoryWidget;
    m_repository = new GitRepositoryWidget;
    m_conflicts = new GitConflictWidget;
    m_tabs->addTab(m_history, tr("History"));
    m_tabs->addTab(m_repository, tr("Branches && stashes"));
    m_tabs->addTab(m_conflicts, tr("Conflicts"));
    layout->addWidget(m_tabs, 1);
    connect(m_tabs, &QTabWidget::currentChanged, this, [this] {
        if (m_tabs->currentWidget() != m_history)
            return;
        QTimer::singleShot(0, this, [this] {
            auto *query = m_history->findChild<QLineEdit *>("gitHistoryQuery");
            auto *commits = m_history->findChild<QListWidget *>("gitHistoryCommits");
            if (!m_snapshot.isEmpty() && query->text().isEmpty() && commits->count() == 0)
                m_history->findChild<QPushButton *>("gitHistorySearch")->click();
        });
    });
    connect(m_history, &GitHistoryWidget::actionRequested, this, &ProjectGitWidget::runRequest);
    connect(m_repository, &GitRepositoryWidget::actionRequested, this, &ProjectGitWidget::runRequest);
    connect(m_conflicts, &GitConflictWidget::actionRequested, this, &ProjectGitWidget::runRequest);
    connect(m_conflicts, &GitConflictWidget::refreshRequested, this, &ProjectGitWidget::refresh);
    connect(m_message, &QLineEdit::textChanged, this, &ProjectGitWidget::updateCommitState);
    updateCommitState();
    auto *footer = new QHBoxLayout;
    m_status = new QLabel;
    m_status->setObjectName("gitStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    footer->addWidget(m_status, 1);
    auto *cancel = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Close), tr("Cancel operation"));
    cancel->hide();
    footer->addWidget(cancel);
    layout->addLayout(footer);
    connect(cancel, &QPushButton::clicked, &m_command, &ProjectGitClient::cancel);
    connect(&m_command, &ProjectGitClient::busyChanged, this, [this, cancel](bool busy) {
        m_controls->setEnabled(!busy);
        m_commitPanel->setEnabled(!busy);
        m_files->setEnabled(!busy);
        m_staged->setEnabled(!busy);
        m_unstagedTree->setEnabled(!busy);
        m_stagedTree->setEnabled(!busy);
        updateCommitState();
        m_fileActions->setEnabled(!busy && m_activeFiles->currentItem());
        m_history->setBusy(busy);
        m_repository->setBusy(busy);
        m_conflicts->setBusy(busy);
        cancel->setVisible(busy);
    });

    auto *passive = new ProjectGitClient(this);
    passive->setObjectName(QStringLiteral("projectGitPassiveReader"));
    auto *poll = new QTimer(this);
    poll->setObjectName(QStringLiteral("projectGitStatusPoll"));
    poll->setInterval(5000);
    connect(poll, &QTimer::timeout, this, &ProjectGitWidget::refreshVisible);
    setFolderView(QSettings().value("git/changesView", "list").toString() == "folders");
}

void ProjectGitWidget::setWorkdir(const QString &workdir)
{
    if (busy() || m_workdir == workdir || !confirmLeave())
        return;
    m_workdir = workdir;
    m_snapshot = {};
    m_history->setSnapshot(workdir, {});
    m_repository->setSnapshot(workdir, {});
    m_conflicts->setSnapshot(workdir, {});
    m_files->clear();
    m_staged->clear();
    m_unstagedTree->clear();
    m_stagedTree->clear();
    m_activeFiles = m_files;
    m_diff->clear();
    m_message->clear();
    findChild<QLineEdit *>("gitFileFilter")->clear();
    for (const auto *name : {"gitChangedCount", "gitStagedCount", "gitConflictCount"})
        findChild<QLabel *>(name)->setText(QString::fromUtf8("—"));
    findChild<QStackedWidget *>("gitReviewStack")->setCurrentIndex(1);
    findChild<QLabel *>("gitEmptyTitle")->setText(tr("Your changes, in focus"));
    findChild<QLabel *>("gitEmptyHint")->setText(tr("Refresh the repository, then select a file to review its changes."));
    m_count->setText(tr("Changed files"));
    m_diffTitle->setText(tr("Select a file to review"));
    m_fileActions->setEnabled(false);
    updateCommitState();
    m_heading->setText(QFileInfo(workdir).fileName());
    m_heading->setToolTip(workdir);
    m_status->setText(tr("Refresh to review this project's changes."));
    if (isVisible())
        QTimer::singleShot(0, this, &ProjectGitWidget::refreshVisible);
}

void ProjectGitWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    findChild<QTimer *>(QStringLiteral("projectGitStatusPoll"))->start();
    QTimer::singleShot(0, this, &ProjectGitWidget::refreshVisible);
}

void ProjectGitWidget::hideEvent(QHideEvent *event)
{
    findChild<QTimer *>(QStringLiteral("projectGitStatusPoll"))->stop();
    if (auto *passive = findChild<ProjectGitClient *>(QStringLiteral("projectGitPassiveReader")); passive && passive->busy())
        passive->cancel();
    // Hiding a view stops discovery only. An admitted mutation keeps its
    // original owner, target and completion; it must not be interrupted here.
    QWidget::hideEvent(event);
}

void ProjectGitWidget::refreshVisible()
{
    if (!isVisible() || m_workdir.isEmpty() || m_conflicts->hasUnsavedChanges()
        || m_message->hasFocus() || !m_message->text().isEmpty())
        return;
    for (const auto *client : findChildren<ProjectGitClient *>()) {
        if (client->busy())
            return;
    }
    if (auto *editor = m_conflicts->findChild<ProjectFilesWidget *>(); editor && editor->filesOperationPending())
        return;
    auto *reader = findChild<ProjectGitClient *>(QStringLiteral("projectGitPassiveReader"));
    const QString root = m_workdir;
    const QJsonObject before = m_snapshot;
    const QJsonObject target = RemoteGitContext::target(this);
    const QString grant = RemoteGitContext::root(this);
    reader->run(root, {{"action", "status"}},
        [this, root, before, target, grant](bool ok, const QJsonObject &snapshot, const QString &) {
            if (!ok || !isVisible() || m_workdir != root || m_snapshot != before || busy()
                || RemoteGitContext::target(this) != target || RemoteGitContext::root(this) != grant
                || m_conflicts->hasUnsavedChanges() || m_message->hasFocus() || !m_message->text().isEmpty())
                return;
            if (auto *editor = m_conflicts->findChild<ProjectFilesWidget *>(); editor && editor->filesOperationPending())
                return;
            // This passive status never drives the mutation client's busy UI.
            // A changed listing offers an explicit new comparison; unchanged
            // listings retain the already reviewed diff and viewport untouched.
            setProperty("projectGitPassiveApply", true);
            render(snapshot);
            setProperty("projectGitPassiveApply", false);
        });
}

void ProjectGitWidget::refresh() { runAction("status"); }

void ProjectGitWidget::runAction(const QString &action)
{
    if (busy())
        return;
    if (m_workdir.isEmpty()) {
        m_status->setText(tr("Open a project folder first."));
        return;
    }
    QJsonObject request{{"workdir", m_workdir}, {"action", action}, {"snapshot", m_snapshot.value("snapshot")}};
    if (action == "diff" || action == "compare") {
        if (!m_activeFiles->currentItem())
            return;
        const QString reviewedPath = m_activeFiles->currentItem()->data(Qt::UserRole).toString();
        request["paths"] = QJsonArray{reviewedPath};
        // Keep the completed review and its title visible while the next
        // comparison is in flight. Mutations remain disabled until completion.
        findChild<QStackedWidget *>("gitReviewStack")->setCurrentIndex(0);
        if (action == "compare")
            request["comparison"] = m_activeFiles == m_staged ? "staged" : "unstaged";
    }
    runRequest(request);
}

bool ProjectGitWidget::confirmLeave()
{
    return (!m_conflicts || m_conflicts->confirmLeave()) && !busy();
}

void ProjectGitWidget::confirmLeaveAsync(std::function<void(bool)> completion)
{
    if (busy()) {
        completion(false);
        return;
    }
    const QPointer<ProjectGitWidget> guard(this);
    m_conflicts->confirmLeaveAsync([guard, completion = std::move(completion)](bool allowed) {
        if (!guard)
            return;
        if (!allowed || !guard->busy()) {
            completion(allowed);
            return;
        }
        // Saving a conflict draft refreshes its review token. Await that read
        // before continuing a close or context change; never retry a mutation.
        auto *waiter = new QObject(guard);
        connect(&guard->m_command, &ProjectGitClient::busyChanged, waiter,
            [guard, waiter, completion](bool busy) {
                if (busy || !guard)
                    return;
                QObject::disconnect(&guard->m_command, nullptr, waiter, nullptr);
                waiter->deleteLater();
                QTimer::singleShot(0, guard, [guard, completion] {
                    if (guard)
                        completion(guard->confirmLeave());
                });
            });
    });
}

void ProjectGitWidget::setWorkdirAsync(const QString &workdir, std::function<void(bool)> completion)
{
    confirmLeaveAsync([this, workdir, completion = std::move(completion)](bool allowed) {
        if (allowed)
            setWorkdir(workdir);
        if (completion)
            completion(allowed && m_workdir == workdir);
    });
}

void ProjectGitWidget::runRequest(QJsonObject request)
{
    if (busy() || m_workdir.isEmpty())
        return;
    if (auto *passive = findChild<ProjectGitClient *>(QStringLiteral("projectGitPassiveReader")); passive && passive->busy())
        passive->cancel();
    const QString action = request.value("action").toString();
    if (!request.contains("snapshot"))
        request["snapshot"] = m_snapshot.value("snapshot");
    const bool readOnly = action == "status" || action == "diff" || action == "compare";
    if (!readOnly && !confirmLeave())
        return;
    // Saving a conflict draft can start a refresh. Require another explicit action afterwards.
    if (busy())
        return;
    if (!readOnly && m_snapshot.isEmpty()) {
        m_status->setText(tr("Refresh the repository before changing it."));
        return;
    }
    const QString reviewedPath = request.value("paths").toArray().isEmpty()
        ? QString() : request.value("paths").toArray().first().toString();
    m_status->setText(tr("%1…").arg(action));
    const bool clearCherryPickQueue = action == "cherry-pick" && m_history->property("gitPickingQueuedCommits").toBool();
    m_command.run(m_workdir, request,
        [this, action, reviewedPath, clearCherryPickQueue](bool ok, const QJsonObject &snapshot, const QString &error) {
            if (!ok) {
                m_snapshot = {};
                for (const auto *name : {"gitChangedCount", "gitStagedCount", "gitConflictCount"})
                    findChild<QLabel *>(name)->setText(QString::fromUtf8("—"));
                findChild<QLabel *>("gitEmptyTitle")->setText(tr("Your changes, in focus"));
                findChild<QLabel *>("gitEmptyHint")->setText(tr("Refresh the repository, then select a file to review its changes."));
                updateCommitState();
                m_status->setText(error);
                return;
            }
            if (action == "diff" || action == "compare") {
                auto review = snapshot.value("review").toObject();
                if (action == "diff") {
                    review.insert("path", reviewedPath);
                    review.insert("patch", snapshot.value("diff").toString());
                }
                m_diff->setReview(review);
                m_diffTitle->setText(reviewedPath);
                m_diffTitle->setToolTip(reviewedPath);
                const QString notice = review.value("notice").toString();
                if (!notice.isEmpty())
                    m_status->setText(notice);
                else
                    m_status->setText(snapshot.value("notice").toString(tr("Reviewing saved changes in %1").arg(reviewedPath)));
                if (action == "compare")
                    m_diffTitle->setText(tr("%1 · %2 → %3").arg(reviewedPath,
                        review.value("before_label").toString(), review.value("after_label").toString()));
                return;
            }
            if (clearCherryPickQueue)
                m_history->clearQueue();
            if (action == "commit" || action == "commit-index")
                m_message->clear();
            render(snapshot);
        });
}

void ProjectGitWidget::render(const QJsonObject &snapshot)
{
    if (!m_snapshot.isEmpty() && snapshot == m_snapshot) {
        // The core has just revalidated this exact reviewed state, including
        // file revisions. Preserve selection, scroll and the completed diff
        // instead of clearing it and issuing another identical comparison.
        if (!property("projectGitPassiveApply").toBool())
            m_repository->setSnapshot(m_workdir, snapshot);
        updateCommitState();
        m_status->setText(snapshot.value("notice").toString(snapshot.value("changes").toArray().isEmpty()
            ? tr("Working tree is clean.") : tr("Stage changes to prepare your next commit.")));
        return;
    }
    const bool hadOperation = !m_snapshot.value("operation").toObject().value("kind").toString().isEmpty();
    m_snapshot = snapshot;
    m_history->setSnapshot(m_workdir, snapshot);
    m_repository->setSnapshot(m_workdir, snapshot);
    m_conflicts->setSnapshot(m_workdir, snapshot);
    const bool hasOperation = !snapshot.value("operation").toObject().value("kind").toString().isEmpty();
    if (hasOperation && !hadOperation)
        m_tabs->setCurrentWidget(m_conflicts);
    const QString branch = snapshot.value("branch").toString();
    m_heading->setText(branch.isEmpty() ? tr("Detached HEAD") : branch);
    m_heading->setToolTip(snapshot.value("root").toString());
    const QString previous = m_activeFiles->currentItem() ? m_activeFiles->currentItem()->data(Qt::UserRole).toString() : QString();
    const bool reviewedStaged = m_activeFiles == m_staged;
    const QSignalBlocker unstagedBlocker(m_files);
    const QSignalBlocker stagedBlocker(m_staged);
    m_files->clear();
    m_staged->clear();
    int conflicts = 0;
    const auto changes = snapshot.value("changes").toArray();
    for (const auto &value : changes) {
        const auto change = value.toObject();
        const QString path = change.value("path").toString();
        const QString status = change.value("status").toString();
        const bool conflict = status.contains('U') || status == "AA" || status == "DD";
        if (conflict)
            ++conflicts;
        const bool staged = !conflict && status.size() >= 2 && status.front() != ' ' && status.front() != '?';
        const bool unstaged = conflict || status == "??" || (status.size() >= 2 && status.at(1) != ' ');
        for (auto *group : {m_files, m_staged}) {
            if ((group == m_staged && !staged) || (group == m_files && !unstaged))
                continue;
            auto *item = new QListWidgetItem(path, group);
            item->setData(Qt::UserRole, path);
            item->setData(Qt::UserRole + 1, status);
            QString groupStatus = status;
            if (!conflict && status != "??" && status.size() >= 2)
                groupStatus = QString(group == m_staged ? status.front() : status.at(1));
            item->setData(Qt::UserRole + 2, groupStatus);
            item->setToolTip(path + " · " + status);
        }
    }
    m_count->setText(tr("Working changes"));
    findChild<QLabel *>("gitChangedCount")->setText(QString::number(changes.size()));
    findChild<QLabel *>("gitStagedCount")->setText(QString::number(m_staged->count()));
    findChild<QLabel *>("gitConflictCount")->setText(QString::number(conflicts));
    findChild<QLabel *>("gitUnstagedHeading")->setText(tr("Unstaged · %1").arg(m_files->count()));
    findChild<QLabel *>("gitStagedHeading")->setText(tr("Staged · %1").arg(m_staged->count()));
    findChild<QLabel *>("gitUnstagedEmpty")->setVisible(m_files->count() == 0);
    findChild<QLabel *>("gitStagedEmpty")->setVisible(m_staged->count() == 0);
    rebuildFolderViews();
    filterFiles(findChild<QLineEdit *>("gitFileFilter")->text());
    updateCommitState();
    m_diff->clear();
    findChild<QStackedWidget *>("gitReviewStack")->setCurrentIndex(1);
    findChild<QLabel *>("gitEmptyTitle")->setText(changes.isEmpty() ? tr("All caught up") : tr("Select a file to review"));
    findChild<QLabel *>("gitEmptyHint")->setText(changes.isEmpty()
        ? tr("Your working tree is clean. New saved changes will appear here after a refresh.")
        : tr("Review unstaged edits, then stage the changes you want to commit."));
    m_diffTitle->setText(tr("Select a file to review"));
    m_fileActions->setEnabled(false);
    m_status->setText(snapshot.value("notice").toString(changes.isEmpty() ? tr("Working tree is clean.")
        : tr("Stage changes to prepare your next commit.")));
    QListWidget *preferred = reviewedStaged ? m_staged : m_files;
    QListWidgetItem *selected = nullptr;
    for (auto *group : {preferred, preferred == m_files ? m_staged : m_files}) {
        for (int row = 0; row < group->count(); ++row) {
            auto *item = group->item(row);
            if (!item->isHidden() && item->data(Qt::UserRole).toString() == previous) {
                selected = item;
                m_activeFiles = group;
                break;
            }
        }
        if (selected)
            break;
    }
    if (!selected) {
        for (auto *group : {preferred, preferred == m_files ? m_staged : m_files}) {
            for (int row = 0; row < group->count(); ++row) {
                if (!group->item(row)->isHidden()) {
                    selected = group->item(row);
                    m_activeFiles = group;
                    break;
                }
            }
            if (selected)
                break;
        }
    }
    if (selected) {
        m_activeFiles->setCurrentItem(selected);
        if (!property("projectGitPassiveApply").toBool())
            selectFile(m_activeFiles);
    }
}

void ProjectGitWidget::updateCommitState()
{
    const bool available = !busy() && !m_snapshot.isEmpty()
        && m_snapshot.value("operation").toObject().value("kind").toString().isEmpty();
    if (m_staged->count() == 0)
        m_selection->setText(tr("No staged files"));
    else if (m_staged->count() == 1)
        m_selection->setText(tr("1 staged file ready to commit"));
    else
        m_selection->setText(tr("%1 staged files ready to commit").arg(m_staged->count()));
    findChild<QPushButton *>("gitCommitStaged")->setEnabled(available && m_staged->count() > 0 && !m_message->text().trimmed().isEmpty());
    for (auto *group : {m_files, m_staged}) {
        bool hasVisibleFiles = false;
        for (int row = 0; row < group->count(); ++row)
            hasVisibleFiles |= !group->item(row)->isHidden();
        findChild<QPushButton *>(group == m_files ? "gitStageAll" : "gitUnstageAll")->setEnabled(available && hasVisibleFiles);
    }
}

void ProjectGitWidget::selectFile(QListWidget *group)
{
    if (busy() || !group->currentItem())
        return;
    m_activeFiles = group;
    const bool staged = group == m_staged;
    QListWidget *other = staged ? m_files : m_staged;
    other->clearSelection();
    other->setCurrentRow(-1);
    syncFolderSelection();
    findChild<QLabel *>("gitComparisonLabel")->setText(staged ? tr("Staged changes") : tr("Unstaged changes"));
    findChild<QPushButton *>("gitFile-stage")->setVisible(!staged);
    findChild<QPushButton *>("gitFile-unstage")->setVisible(staged);
    findChild<QPushButton *>("gitFile-discard")->setVisible(!staged);
    m_fileActions->setEnabled(true);
    runAction("compare");
}

void ProjectGitWidget::stageGroup(bool stage)
{
    if (busy())
        return;
    QListWidget *group = stage ? m_files : m_staged;
    QJsonArray paths;
    for (int row = 0; row < group->count(); ++row) {
        if (!group->item(row)->isHidden())
            paths.append(group->item(row)->data(Qt::UserRole).toString());
    }
    if (paths.isEmpty())
        return;
    runRequest(QJsonObject{{"action", stage ? "stage" : "unstage"}, {"paths", paths}, {"confirm", true}});
}

void ProjectGitWidget::filterFiles(const QString &query)
{
    for (auto *group : {m_files, m_staged}) {
        for (int row = 0; row < group->count(); ++row) {
            auto *item = group->item(row);
            item->setHidden(!item->data(Qt::UserRole).toString().contains(query, Qt::CaseInsensitive));
        }
        int visible = 0;
        for (int row = 0; row < group->count(); ++row)
            visible += !group->item(row)->isHidden();
        auto *hint = findChild<QLabel *>(group == m_staged ? "gitStagedEmpty" : "gitUnstagedEmpty");
        if (group->count() > 0)
            hint->setText(tr("No files match your filter."));
        else if (group == m_staged)
            hint->setText(tr("Stage changes to include them in your commit."));
        else
            hint->setText(tr("No unstaged changes."));
        hint->setVisible(visible == 0);
    }
    for (auto *tree : {m_unstagedTree, m_stagedTree}) {
        QList<QTreeWidgetItem *> items;
        for (QTreeWidgetItemIterator iterator(tree); *iterator; ++iterator)
            items.append(*iterator);
        for (auto iterator = items.crbegin(); iterator != items.crend(); ++iterator) {
            auto *item = *iterator;
            if (!item->data(0, Qt::UserRole + 3).toBool()) {
                item->setHidden(!item->data(0, Qt::UserRole).toString().contains(query, Qt::CaseInsensitive));
                continue;
            }
            bool visible = false;
            for (int child = 0; child < item->childCount(); ++child)
                visible |= !item->child(child)->isHidden();
            item->setHidden(!visible);
            if (visible && !query.isEmpty())
                item->setExpanded(true);
        }
    }
    findChild<QPushButton *>("gitStageAll")->setText(query.isEmpty() ? tr("Stage all") : tr("Stage shown"));
    findChild<QPushButton *>("gitUnstageAll")->setText(query.isEmpty() ? tr("Unstage all") : tr("Unstage shown"));
    updateCommitState();
}

void ProjectGitWidget::setFolderView(bool folders)
{
    m_unstagedViews->setCurrentIndex(folders ? 1 : 0);
    m_stagedViews->setCurrentIndex(folders ? 1 : 0);
    findChild<QToolButton *>("gitFoldersView")->setChecked(folders);
    findChild<QToolButton *>("gitFlatView")->setChecked(!folders);
    QSettings().setValue("git/changesView", folders ? "folders" : "list");
    syncFolderSelection();
}

void ProjectGitWidget::rebuildFolderViews()
{
    for (auto *files : {m_files, m_staged}) {
        auto *tree = files == m_files ? m_unstagedTree : m_stagedTree;
        const QSignalBlocker blocker(tree);
        QSet<QString> collapsed;
        for (QTreeWidgetItemIterator iterator(tree); *iterator; ++iterator) {
            auto *item = *iterator;
            if (item->data(0, Qt::UserRole + 3).toBool() && !item->isExpanded())
                collapsed.insert(item->data(0, Qt::UserRole).toString());
        }
        tree->clear();
        QHash<QString, QTreeWidgetItem *> folders;
        // Build directories first so folders precede files at every level.
        for (int row = 0; row < files->count(); ++row) {
            const QString path = files->item(row)->data(Qt::UserRole).toString();
            const auto parts = path.split('/');
            QString prefix;
            auto *parent = tree->invisibleRootItem();
            for (int depth = 0; depth + 1 < parts.size(); ++depth) {
                if (!prefix.isEmpty())
                    prefix += '/';
                prefix += parts.at(depth);
                auto *folder = folders.value(prefix);
                if (!folder) {
                    folder = new QTreeWidgetItem(parent, {parts.at(depth)});
                    folder->setData(0, Qt::UserRole, prefix);
                    folder->setData(0, Qt::UserRole + 3, true);
                    folder->setIcon(0, CanvasIcons::icon(CanvasIcons::Symbol::Folder));
                    folder->setToolTip(0, prefix);
                    folder->setExpanded(!collapsed.contains(prefix));
                    folders.insert(prefix, folder);
                }
                parent = folder;
            }
        }
        for (int row = 0; row < files->count(); ++row) {
            const auto *file = files->item(row);
            const QString path = file->data(Qt::UserRole).toString();
            const int separator = path.lastIndexOf('/');
            auto *parent = separator < 0 ? tree->invisibleRootItem() : folders.value(path.left(separator));
            auto *item = new QTreeWidgetItem(parent, {QFileInfo(path).fileName()});
            for (int role = Qt::UserRole; role <= Qt::UserRole + 2; ++role)
                item->setData(0, role, file->data(role));
            item->setToolTip(0, file->toolTip());
        }
    }
}

void ProjectGitWidget::syncFolderSelection()
{
    for (auto *files : {m_files, m_staged}) {
        auto *tree = files == m_files ? m_unstagedTree : m_stagedTree;
        const QSignalBlocker blocker(tree);
        tree->clearSelection();
        tree->setCurrentItem(nullptr);
        if (files != m_activeFiles || !files->currentItem())
            continue;
        const QString path = files->currentItem()->data(Qt::UserRole).toString();
        for (QTreeWidgetItemIterator iterator(tree); *iterator; ++iterator) {
            auto *item = *iterator;
            if (!item->data(0, Qt::UserRole + 3).toBool() && item->data(0, Qt::UserRole).toString() == path) {
                tree->setCurrentItem(item);
                for (auto *parent = item->parent(); parent; parent = parent->parent())
                    parent->setExpanded(true);
                tree->scrollToItem(item);
                QTimer::singleShot(0, tree, [tree, path] {
                    auto *current = tree->currentItem();
                    if (current && current->data(0, Qt::UserRole).toString() == path)
                        tree->scrollToItem(current, QAbstractItemView::PositionAtCenter);
                });
                break;
            }
        }
    }
}
