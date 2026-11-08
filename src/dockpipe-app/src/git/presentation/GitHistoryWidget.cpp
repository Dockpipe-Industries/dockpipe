#include "GitHistoryWidget.h"

#include "CanvasIcons.h"
#include "GitCommitQueueModel.h"
#include "GitCommitGraphDelegate.h"
#include "GitDiffWidget.h"
#include "GitCommitPreviewDialog.h"

#include <QComboBox>
#include <QApplication>
#include <QClipboard>
#include <QMenu>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QListView>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QSignalBlocker>
#include <QSplitter>
#include <QVBoxLayout>
#include <QTextDocument>

namespace {
QPushButton *namedButton(const QString &text, const char *name, QBoxLayout *layout)
{
    auto *button = new QPushButton(text);
    button->setObjectName(QLatin1String(name));
    layout->addWidget(button);
    return button;
}
}

GitHistoryWidget::GitHistoryWidget(QWidget *parent) : QWidget(parent), m_reader(this)
{
    setObjectName("gitHistory");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(14);
    auto *searchBar = new QHBoxLayout;
    m_query = new QLineEdit;
    m_query->setObjectName("gitHistoryQuery");
    m_query->setPlaceholderText(tr("Search commits…"));
    m_query->setMaxLength(256);
    searchBar->addWidget(m_query, 1);
    m_match = new QComboBox;
    m_match->setObjectName("gitHistoryMatch");
    m_match->addItem(tr("Keywords"), "keyword");
    m_match->addItem(tr("SHA / subject starts with"), "prefix");
    m_match->setToolTip(tr("Keywords search commit subjects and bodies. Prefix matches a SHA or the start of a subject."));
    searchBar->addWidget(m_match);
    m_ref = new QComboBox;
    m_ref->setObjectName("gitHistoryRef");
    m_ref->addItem(tr("All branches"), "all");
    m_ref->addItem(tr("Current HEAD"), QString());
    searchBar->addWidget(m_ref);
    m_search = namedButton(tr("Search"), "gitHistorySearch", searchBar);
    m_search->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Search));
    auto *cancel = namedButton(tr("Cancel read"), "gitHistoryCancel", searchBar);
    cancel->setVisible(false);
    connect(cancel, &QPushButton::clicked, &m_reader, &ProjectGitClient::cancel);
    connect(&m_reader, &ProjectGitClient::busyChanged, cancel, &QPushButton::setVisible);
    auto *queueToggle = namedButton(tr("Cherry-pick queue"), "gitQueueToggle", searchBar);
    queueToggle->setCheckable(true);
    layout->addLayout(searchBar);

    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(12);
    auto *history = new QWidget;
    history->setProperty("gitCard", true);
    history->setAttribute(Qt::WA_StyledBackground);
    auto *historyLayout = new QVBoxLayout(history);
    historyLayout->setContentsMargins(14, 14, 14, 14);
    historyLayout->setSpacing(12);
    auto *historyHeading = new QLabel(tr("Commit graph"));
    historyHeading->setObjectName("gitPanelTitle");
    historyLayout->addWidget(historyHeading);
    m_commits = new QListWidget;
    m_commits->setObjectName("gitHistoryCommits");
    m_commits->setItemDelegate(new GitCommitGraphDelegate(m_commits));
    m_commits->setSpacing(0);
    m_commits->setSelectionMode(QAbstractItemView::ExtendedSelection);
    m_commits->setContextMenuPolicy(Qt::CustomContextMenu);
    connect(m_commits, &QListWidget::customContextMenuRequested, this, &GitHistoryWidget::showCommitMenu);
    m_commits->setMinimumWidth(350);
    historyLayout->addWidget(m_commits, 1);
    auto *paging = new QHBoxLayout;
    m_previous = namedButton(tr("Previous"), "gitHistoryPrevious", paging);
    m_page = new QLabel(tr("Search to load history"));
    m_page->setObjectName("gitHistoryPage");
    m_page->setTextFormat(Qt::PlainText);
    paging->addWidget(m_page, 1);
    m_next = namedButton(tr("Next"), "gitHistoryNext", paging);
    historyLayout->addLayout(paging);
    auto *selection = new QHBoxLayout;
    m_add = namedButton(tr("Add selected to queue"), "gitHistoryAdd", selection);
    m_revert = namedButton(tr("Revert selected…"), "gitHistoryRevert", selection);
    historyLayout->addLayout(selection);
    splitter->addWidget(history);

    m_detail = new GitDiffWidget;
    m_detail->setObjectName("gitHistoryDetail");
    splitter->addWidget(m_detail);

    auto *queuePanel = new QWidget;
    queuePanel->setObjectName("gitCherryPickPanel");
    queuePanel->setProperty("gitCard", true);
    queuePanel->setAttribute(Qt::WA_StyledBackground);
    auto *queueLayout = new QVBoxLayout(queuePanel);
    queueLayout->setContentsMargins(14, 14, 14, 14);
    queueLayout->setSpacing(12);
    auto *heading = new QLabel(tr("Cherry-pick queue"));
    heading->setObjectName("gitPanelTitle");
    queueLayout->addWidget(heading);
    m_target = new QLabel;
    m_target->setObjectName("gitQueueTarget");
    m_target->setTextFormat(Qt::PlainText);
    m_target->setWordWrap(true);
    queueLayout->addWidget(m_target);
    m_queue = new GitCommitQueueModel(this);
    m_queueView = new QListView;
    m_queueView->setObjectName("gitCherryPickQueue");
    m_queueView->setModel(m_queue);
    m_queueView->setMinimumWidth(260);
    queueLayout->addWidget(m_queueView, 1);
    auto *ordering = new QHBoxLayout;
    m_up = namedButton(tr("Up"), "gitQueueUp", ordering);
    m_down = namedButton(tr("Down"), "gitQueueDown", ordering);
    m_reverse = namedButton(tr("Reverse"), "gitQueueReverse", ordering);
    queueLayout->addLayout(ordering);
    auto *removing = new QHBoxLayout;
    m_remove = namedButton(tr("Remove"), "gitQueueRemove", removing);
    m_clear = namedButton(tr("Clear"), "gitQueueClear", removing);
    queueLayout->addLayout(removing);
    auto *hint = new QLabel(tr("Applied from top to bottom. Resolve any conflicts before continuing the sequence."));
    hint->setWordWrap(true);
    hint->setObjectName("editorMuted");
    queueLayout->addWidget(hint);
    auto *picking = new QHBoxLayout;
    m_pick = namedButton(tr("Cherry-pick queued…"), "gitQueueStart", picking);
    queueLayout->addLayout(picking);
    splitter->addWidget(queuePanel);
    queuePanel->hide();
    connect(queueToggle, &QPushButton::toggled, queuePanel, &QWidget::setVisible);
    connect(m_queue, &QAbstractItemModel::rowsInserted, this, [queueToggle] {
        queueToggle->setChecked(true);
    });
    connect(m_queue, &QAbstractItemModel::rowsInserted, this, [this, queueToggle] {
        queueToggle->setText(tr("Cherry-pick queue · %1").arg(m_queue->rowCount()));
    });
    auto updateQueueLabel = [this, queueToggle] {
        queueToggle->setText(tr("Cherry-pick queue · %1").arg(m_queue->rowCount()));
    };
    connect(m_queue, &QAbstractItemModel::rowsRemoved, this, updateQueueLabel);
    connect(m_queue, &QAbstractItemModel::modelReset, this, updateQueueLabel);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({440, 600, 300});
    layout->addWidget(splitter, 1);
    m_status = new QLabel;
    m_status->setObjectName("gitHistoryStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);

    connect(m_search, &QPushButton::clicked, this, [this] { search(0); });
    connect(m_query, &QLineEdit::returnPressed, this, [this] { search(0); });
    connect(m_query, &QLineEdit::textChanged, this, &GitHistoryWidget::updateControls);
    connect(m_match, &QComboBox::currentIndexChanged, this, &GitHistoryWidget::updateControls);
    connect(m_ref, &QComboBox::currentIndexChanged, this, &GitHistoryWidget::updateControls);
    connect(m_previous, &QPushButton::clicked, this, [this] { search(qMax(0, m_offset - pageSize)); });
    connect(m_next, &QPushButton::clicked, this, [this] { search(m_offset + pageSize); });
    connect(m_commits, &QListWidget::currentItemChanged, this, [this] { loadCommit(); });
    connect(m_commits, &QListWidget::itemSelectionChanged, this, &GitHistoryWidget::updateControls);
    connect(m_add, &QPushButton::clicked, this, &GitHistoryWidget::addSelected);
    connect(m_revert, &QPushButton::clicked, this, &GitHistoryWidget::requestRevert);
    connect(m_pick, &QPushButton::clicked, this, [this] { requestCherryPick(); });
    connect(m_up, &QPushButton::clicked, this, [this] { moveSelected(-1); });
    connect(m_down, &QPushButton::clicked, this, [this] { moveSelected(1); });
    connect(m_remove, &QPushButton::clicked, this, [this] { m_queue->removeCommit(m_queueView->currentIndex().row()); });
    connect(m_clear, &QPushButton::clicked, this, &GitHistoryWidget::clearQueue);
    connect(m_reverse, &QPushButton::clicked, this, [this] {
        const int count = m_queue->rowCount();
        for (int row = 1; row < count; ++row)
            m_queue->moveCommit(row, 0);
    });
    connect(m_queueView->selectionModel(), &QItemSelectionModel::currentChanged, this, &GitHistoryWidget::updateControls);
    connect(m_queue, &QAbstractItemModel::rowsInserted, this, &GitHistoryWidget::updateControls);
    connect(m_queue, &QAbstractItemModel::rowsRemoved, this, &GitHistoryWidget::updateControls);
    connect(m_queue, &QAbstractItemModel::rowsMoved, this, &GitHistoryWidget::updateControls);
    connect(m_queue, &QAbstractItemModel::modelReset, this, &GitHistoryWidget::updateControls);
    connect(&m_reader, &ProjectGitClient::busyChanged, this, &GitHistoryWidget::updateControls);
    updateControls();
}

void GitHistoryWidget::setSnapshot(const QString &workdir, const QJsonObject &snapshot)
{
    QJsonObject references;
    for (const auto &value : snapshot.value("catalog").toObject().value("branches").toArray()) {
        const auto branch = value.toObject();
        const QString id = branch.value("id").toString();
        auto labels = references.value(id).toArray();
        labels.append(branch.value("name").toString());
        references.insert(id, labels);
    }
    const QString head = snapshot.value("head").toString();
    if (!head.isEmpty()) {
        auto labels = references.value(head).toArray();
        labels.prepend(QStringLiteral("HEAD"));
        references.insert(head, labels);
    }
    setProperty("gitGraphReferences", references);
    if (workdir != m_workdir) {
        setProperty("gitLoadedCommit", QString());
        setProperty("gitLoadedReview", QJsonObject());
        ++m_generation;
        m_reader.cancel();
        m_workdir = workdir;
        m_queue->clear();
        m_commits->clear();
        m_detail->clear();
        m_query->clear();
        m_offset = 0;
        m_hasMore = false;
        m_page->setText(tr("Search to load history"));
        m_status->clear();
    }
    m_operationActive = !snapshot.value("operation").toObject().value("kind").toString().isEmpty();
    m_branch = snapshot.value("branch").toString();
    m_snapshotToken = snapshot.value("snapshot").toString();
    const QString previousRef = m_ref->currentData().toString();
    const QSignalBlocker blocker(m_ref);
    m_ref->clear();
    m_ref->addItem(tr("All branches"), "all");
    m_ref->addItem(tr("Current HEAD"), QString());
    for (const auto &value : snapshot.value("catalog").toObject().value("branches").toArray()) {
        const QString name = value.toObject().value("name").toString();
        if (!name.isEmpty() && m_ref->findData(name) < 0)
            m_ref->addItem(name, name);
    }
    const int selected = m_ref->findData(previousRef);
    m_ref->setCurrentIndex(selected < 0 ? 0 : selected);
    updateControls();
}

void GitHistoryWidget::setBusy(bool busy)
{
    m_parentBusy = busy;
    updateControls();
}

void GitHistoryWidget::clearQueue()
{
    m_queue->clear();
}

void GitHistoryWidget::search(int offset)
{
    if (m_reader.busy() || m_workdir.isEmpty() || m_parentBusy)
        return;
    if (m_query->text().toUtf8().size() > 256 || m_query->text().contains(QChar::Null)) {
        m_status->setText(tr("Use a search query of at most 256 UTF-8 bytes without null characters."));
        return;
    }
    const quint64 generation = ++m_generation;
    const QJsonObject request{{"action", "history"}, {"query", m_query->text()},
        {"match", m_match->currentData().toString()}, {"ref", m_ref->currentData().toString()},
        {"offset", offset}, {"limit", pageSize}};
    m_status->setText(tr("Searching commit history…"));
    m_reader.run(m_workdir, request, [this, generation, request](bool ok, const QJsonObject &reply, const QString &error) {
        if (generation != m_generation)
            return;
        if (!ok) {
            m_status->setText(error);
            return;
        }
        const auto history = reply.value("history").toObject();
        const QSignalBlocker blocker(m_commits);
        m_commits->clear();
        m_detail->clear();
        auto commits = history.value("commits").toArray();
        if (commits.size() > pageSize) {
            m_status->setText(tr("History reply exceeded the 50-commit page contract."));
            return;
        }
        const auto references = property("gitGraphReferences").toJsonObject();
        for (int row = 0; row < commits.size(); ++row) {
            auto commit = commits.at(row).toObject();
            const QString id = commit.value("id").toString();
            auto labels = commit.value("refs").toArray();
            for (const auto &label : references.value(id).toArray()) {
                if (!labels.contains(label))
                    labels.append(label);
            }
            commit.insert("refs", labels);
            commits[row] = commit;
            const QString subject = commit.value("subject").toString();
            auto *item = new QListWidgetItem(id.left(12) + "  " + subject, m_commits);
            item->setData(Qt::UserRole, id);
            item->setData(Qt::UserRole + 1, subject);
            item->setToolTip(Qt::convertFromPlainText(id + "\n" + commit.value("author").toString() + "\n" + subject));
        }
        if (!GitCommitGraphDelegate::prepare(m_commits, commits)) {
            m_commits->clear();
            m_status->setText(tr("History reply could not be displayed as a commit graph."));
            return;
        }
        m_offset = history.value("offset").toInt();
        m_hasMore = history.value("has_more").toBool();
        m_pageQuery = request.value("query").toString();
        m_pageMatch = request.value("match").toString();
        m_pageRef = request.value("ref").toString();
        m_page->setText(commits.isEmpty() ? tr("No matching commits")
            : tr("%1–%2").arg(m_offset + 1).arg(m_offset + commits.size()));
        m_status->setText(tr("%1 commits · Select a commit to review. Dashed links continue beyond these results.").arg(commits.size()));
        updateControls();
    });
}

void GitHistoryWidget::loadCommit()
{
    if (m_reader.busy() || !m_commits->currentItem())
        return;
    const QString id = m_commits->currentItem()->data(Qt::UserRole).toString();
    const quint64 generation = ++m_generation;
    m_detail->clear();
    m_status->setText(tr("Loading commit %1…").arg(id.left(12)));
    m_reader.run(m_workdir, {{"action", "commit-review"}, {"commit", id}},
        [this, generation](bool ok, const QJsonObject &reply, const QString &error) {
            if (generation != m_generation)
                return;
            if (!ok) {
                m_status->setText(error);
                return;
            }
            const auto review = reply.value("commit_review").toObject();
            setProperty("gitLoadedReview", review);
            setProperty("gitLoadedCommit", review.value("commit").toObject().value("id").toString());
            const QString notice = review.value("notice").toString();
            if (review.value("limited").toBool()) {
                const QString limitation = notice.isEmpty() ? tr("The commit exceeds the preview limits. The complete patch is unavailable here.") : notice;
                m_status->setText(limitation);
            } else {
                m_status->setText(tr("Reviewing the selected commit. The working tree has not changed."));
            }
            m_detail->setCommitReview(review);
        });
}

void GitHistoryWidget::addSelected()
{
    int added = 0;
    int rejected = 0;
    // QListWidget selection order is unspecified; visible history order is stable.
    for (int row = 0; row < m_commits->count(); ++row) {
        const auto *item = m_commits->item(row);
        if (!item->isSelected())
            continue;
        if (m_queue->addCommit(item->data(Qt::UserRole).toString(), item->data(Qt::UserRole + 1).toString()))
            ++added;
        else
            ++rejected;
    }
    m_status->setText(tr("Added %1 commits. %2 duplicates, invalid entries or entries beyond the 128-commit limit were not added.").arg(added).arg(rejected));
    updateControls();
}

void GitHistoryWidget::moveSelected(int direction)
{
    const int row = m_queueView->currentIndex().row();
    if (m_queue->moveCommit(row, row + direction))
        m_queueView->setCurrentIndex(m_queue->index(row + direction));
    updateControls();
}

void GitHistoryWidget::requestCherryPick()
{
    requestCherryPick(m_queue->commitIds(), true);
}

void GitHistoryWidget::requestCherryPick(const QStringList &ids, bool queued)
{
    if (m_parentBusy || m_reader.busy() || ids.isEmpty() || m_workdir.isEmpty()
        || m_snapshotToken.isEmpty() || m_operationActive)
        return;
    const QString reviewedSnapshot = m_snapshotToken;
    const QString reviewedWorkdir = m_workdir;
    const QString target = m_branch.isEmpty() ? tr("detached HEAD") : m_branch;
    const QString description = queued ? tr("Apply %1 commits, in this order, onto %2?").arg(ids.size()).arg(target)
        : tr("Apply commit %1 onto %2?").arg(ids.first().left(12), target);
    QMessageBox confirmation(QMessageBox::Question, queued ? tr("Cherry-pick queued commits") : tr("Cherry-pick commit"),
        description,
        QMessageBox::Yes | QMessageBox::No, this);
    confirmation.setTextFormat(Qt::PlainText);
    confirmation.setDetailedText(ids.join('\n'));
    confirmation.setDefaultButton(QMessageBox::No);
    if (confirmation.exec() != QMessageBox::Yes)
        return;
    if (m_parentBusy || m_reader.busy() || m_operationActive || reviewedSnapshot != m_snapshotToken
        || reviewedWorkdir != m_workdir || (queued && ids != m_queue->commitIds())) {
        m_status->setText(tr("The repository or queue changed during confirmation. Review it before trying again."));
        return;
    }
    setProperty("gitPickingQueuedCommits", queued);
    emit actionRequested({{"action", "cherry-pick"}, {"commits", QJsonArray::fromStringList(ids)},
        {"snapshot", reviewedSnapshot}, {"confirm", true}});
}

void GitHistoryWidget::requestRevert()
{
    const auto selected = m_commits->selectedItems();
    if (m_parentBusy || m_reader.busy() || selected.size() != 1 || m_workdir.isEmpty())
        return;
    const QString id = selected.first()->data(Qt::UserRole).toString();
    requestRevertCommit(id);
}

void GitHistoryWidget::requestRevertCommit(const QString &id)
{
    if (m_parentBusy || m_reader.busy() || m_workdir.isEmpty() || m_snapshotToken.isEmpty() || m_operationActive)
        return;
    const QString reviewedSnapshot = m_snapshotToken;
    const QString reviewedWorkdir = m_workdir;
    QMessageBox confirmation(QMessageBox::Question, tr("Revert commit"),
        tr("Create a commit reversing %1 on %2?").arg(id, m_branch.isEmpty() ? tr("detached HEAD") : m_branch),
        QMessageBox::Yes | QMessageBox::No, this);
    confirmation.setTextFormat(Qt::PlainText);
    confirmation.setDefaultButton(QMessageBox::No);
    if (confirmation.exec() != QMessageBox::Yes)
        return;
    if (m_parentBusy || m_reader.busy() || m_operationActive
        || reviewedSnapshot != m_snapshotToken || reviewedWorkdir != m_workdir) {
        m_status->setText(tr("The repository changed during confirmation. Review it before trying again."));
        return;
    }
    emit actionRequested({{"action", "revert"}, {"commit", id}, {"snapshot", reviewedSnapshot}, {"confirm", true}});
}

void GitHistoryWidget::updateControls()
{
    const bool ready = !m_parentBusy && !m_reader.busy() && !m_workdir.isEmpty();
    const bool writable = ready && !m_snapshotToken.isEmpty() && !m_operationActive;
    m_search->setEnabled(ready);
    m_query->setEnabled(ready);
    m_match->setEnabled(ready);
    m_ref->setEnabled(ready);
    m_commits->setEnabled(ready);
    const bool sameSearch = m_query->text() == m_pageQuery && m_match->currentData().toString() == m_pageMatch
        && m_ref->currentData().toString() == m_pageRef;
    m_previous->setEnabled(ready && sameSearch && m_offset > 0);
    m_next->setEnabled(ready && sameSearch && m_hasMore && m_offset + pageSize <= 100000);
    m_add->setEnabled(ready && !m_commits->selectedItems().isEmpty() && m_queue->rowCount() < GitCommitQueueModel::maximumCommits);
    m_revert->setEnabled(writable && m_commits->selectedItems().size() == 1);
    m_pick->setEnabled(writable && m_queue->rowCount() > 0);
    m_queueView->setEnabled(!m_parentBusy);
    const int row = m_queueView->currentIndex().row();
    m_up->setEnabled(!m_parentBusy && row > 0);
    m_down->setEnabled(!m_parentBusy && row >= 0 && row + 1 < m_queue->rowCount());
    m_remove->setEnabled(!m_parentBusy && row >= 0);
    m_reverse->setEnabled(!m_parentBusy && m_queue->rowCount() > 1);
    m_clear->setEnabled(!m_parentBusy && m_queue->rowCount() > 0);
    m_target->setText(tr("%1 / %2 commits · Target: %3").arg(m_queue->rowCount()).arg(GitCommitQueueModel::maximumCommits)
        .arg(m_branch.isEmpty() ? tr("detached HEAD") : m_branch));
}

void GitHistoryWidget::showCommitMenu(const QPoint &point)
{
    auto *item = m_commits->itemAt(point);
    if (!item || m_parentBusy || m_reader.busy())
        return;
    const QString id = item->data(Qt::UserRole).toString();
    const QString subject = item->data(Qt::UserRole + 1).toString();
    const QString workdir = m_workdir;
    const QString snapshot = m_snapshotToken;
    {
        const QSignalBlocker blocker(m_commits);
        if (!item->isSelected())
            m_commits->clearSelection();
        m_commits->setCurrentItem(item, QItemSelectionModel::NoUpdate);
        item->setSelected(true);
    }
    updateControls();
    QMenu menu(this);
    menu.setObjectName("gitCommitContextMenu");
    auto *preview = menu.addAction(tr("Open commit preview"));
    preview->setObjectName("gitCommitPreviewAction");
    menu.addSeparator();
    auto *cherryPick = menu.addAction(tr("Cherry-pick this commit…"));
    cherryPick->setObjectName("gitCommitCherryPickAction");
    cherryPick->setEnabled(!m_snapshotToken.isEmpty() && !m_operationActive);
    auto *queue = menu.addAction(tr("Add commit to cherry-pick queue"));
    queue->setObjectName("gitCommitQueueAction");
    queue->setEnabled(m_queue->rowCount() < GitCommitQueueModel::maximumCommits);
    auto *revert = menu.addAction(tr("Revert this commit…"));
    revert->setObjectName("gitCommitRevertAction");
    revert->setEnabled(cherryPick->isEnabled());
    menu.addSeparator();
    auto *copy = menu.addAction(tr("Copy commit SHA"));
    copy->setObjectName("gitCommitCopyAction");
    QAction *chosen = menu.exec(m_commits->viewport()->mapToGlobal(point));
    if (chosen == copy) {
        QApplication::clipboard()->setText(id);
    } else if (chosen) {
        if (m_parentBusy || m_workdir != workdir || m_snapshotToken != snapshot) {
            m_status->setText(tr("The repository changed while the menu was open. Review it before trying again."));
            return;
        }
        if (chosen == preview) {
            const bool cached = property("gitLoadedCommit").toString() == id;
            const auto review = cached ? property("gitLoadedReview").toJsonObject() : QJsonObject();
            GitCommitPreviewDialog dialog(workdir, id, review, this);
            dialog.exec();
        } else if (chosen == cherryPick) {
            requestCherryPick({id}, false);
        } else if (chosen == queue) {
            const bool added = m_queue->addCommit(id, subject);
            m_status->setText(added ? tr("Commit added to the cherry-pick queue.") : tr("This commit is already queued or cannot be added."));
            updateControls();
        } else if (chosen == revert) {
            requestRevertCommit(id);
        }
    }
    if (!m_parentBusy && m_workdir == workdir)
        loadCommit();
}
