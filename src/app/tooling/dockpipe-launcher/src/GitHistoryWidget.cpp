#include "GitHistoryWidget.h"

#include "CanvasIcons.h"
#include "GitCommitQueueModel.h"
#include "SourceEditor.h"

#include <QComboBox>
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
    cancel->setEnabled(false);
    connect(cancel, &QPushButton::clicked, &m_reader, &ProjectGitClient::cancel);
    connect(&m_reader, &ProjectGitClient::busyChanged, cancel, &QPushButton::setEnabled);
    layout->addLayout(searchBar);

    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    auto *history = new QWidget;
    auto *historyLayout = new QVBoxLayout(history);
    historyLayout->setContentsMargins(0, 0, 0, 0);
    m_commits = new QListWidget;
    m_commits->setObjectName("gitHistoryCommits");
    m_commits->setSelectionMode(QAbstractItemView::ExtendedSelection);
    m_commits->setMinimumWidth(250);
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

    m_detail = new SourceEditor;
    m_detail->setObjectName("gitHistoryDetail");
    m_detail->setReadOnly(true);
    m_detail->setFileName("commit.diff");
    m_detail->setPlaceholderText(tr("Select a commit to inspect its message and patch."));
    splitter->addWidget(m_detail);

    auto *queuePanel = new QWidget;
    queuePanel->setObjectName("gitCherryPickPanel");
    auto *queueLayout = new QVBoxLayout(queuePanel);
    queueLayout->setContentsMargins(8, 0, 0, 0);
    auto *heading = new QLabel(tr("CHERRY-PICK QUEUE"));
    heading->setObjectName("editorSectionLabel");
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
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({320, 600, 300});
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
    connect(m_pick, &QPushButton::clicked, this, &GitHistoryWidget::requestCherryPick);
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
    if (workdir != m_workdir) {
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
        const auto commits = history.value("commits").toArray();
        if (commits.size() > pageSize) {
            m_status->setText(tr("History reply exceeded the 50-commit page contract."));
            return;
        }
        for (const auto &value : commits) {
            const auto commit = value.toObject();
            const QString id = commit.value("id").toString();
            const QString subject = commit.value("subject").toString();
            auto *item = new QListWidgetItem(id.left(12) + "  " + subject, m_commits);
            item->setData(Qt::UserRole, id);
            item->setData(Qt::UserRole + 1, subject);
            item->setToolTip(Qt::convertFromPlainText(id + "\n" + commit.value("author").toString() + "\n" + subject));
        }
        m_offset = history.value("offset").toInt();
        m_hasMore = history.value("has_more").toBool();
        m_pageQuery = request.value("query").toString();
        m_pageMatch = request.value("match").toString();
        m_pageRef = request.value("ref").toString();
        m_page->setText(commits.isEmpty() ? tr("No matching commits")
            : tr("%1–%2").arg(m_offset + 1).arg(m_offset + commits.size()));
        m_status->setText(tr("%1 commits loaded. Select commits and add them to the queue in the order you want.").arg(commits.size()));
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
            const QString notice = review.value("notice").toString();
            QString detail = review.value("body").toString() + "\n\n" + review.value("patch").toString();
            if (review.value("limited").toBool()) {
                const QString limitation = notice.isEmpty() ? tr("The commit exceeds the preview limits. The complete patch is unavailable here.") : notice;
                detail.prepend(limitation + "\n\n");
                m_status->setText(limitation);
            } else {
                m_status->setText(tr("Reviewing the selected commit. The working tree has not changed."));
            }
            m_detail->setPlainText(detail);
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
    const QStringList ids = m_queue->commitIds();
    if (m_parentBusy || m_reader.busy() || ids.isEmpty() || m_workdir.isEmpty())
        return;
    const QString reviewedSnapshot = m_snapshotToken;
    const QString reviewedWorkdir = m_workdir;
    QMessageBox confirmation(QMessageBox::Question, tr("Cherry-pick queued commits"),
        tr("Apply %1 commits, in this order, onto %2?").arg(ids.size()).arg(m_branch.isEmpty() ? tr("detached HEAD") : m_branch),
        QMessageBox::Yes | QMessageBox::No, this);
    confirmation.setTextFormat(Qt::PlainText);
    confirmation.setDetailedText(ids.join('\n'));
    confirmation.setDefaultButton(QMessageBox::No);
    if (confirmation.exec() != QMessageBox::Yes)
        return;
    if (m_parentBusy || reviewedSnapshot != m_snapshotToken || reviewedWorkdir != m_workdir || ids != m_queue->commitIds()) {
        m_status->setText(tr("The repository or queue changed during confirmation. Review it before trying again."));
        return;
    }
    emit actionRequested({{"action", "cherry-pick"}, {"commits", QJsonArray::fromStringList(ids)},
        {"snapshot", reviewedSnapshot}, {"confirm", true}});
}

void GitHistoryWidget::requestRevert()
{
    const auto selected = m_commits->selectedItems();
    if (m_parentBusy || m_reader.busy() || selected.size() != 1 || m_workdir.isEmpty())
        return;
    const QString id = selected.first()->data(Qt::UserRole).toString();
    const QString reviewedSnapshot = m_snapshotToken;
    const QString reviewedWorkdir = m_workdir;
    QMessageBox confirmation(QMessageBox::Question, tr("Revert commit"),
        tr("Create a commit reversing %1 on %2?").arg(id, m_branch.isEmpty() ? tr("detached HEAD") : m_branch),
        QMessageBox::Yes | QMessageBox::No, this);
    confirmation.setTextFormat(Qt::PlainText);
    confirmation.setDefaultButton(QMessageBox::No);
    if (confirmation.exec() != QMessageBox::Yes)
        return;
    if (m_parentBusy || reviewedSnapshot != m_snapshotToken || reviewedWorkdir != m_workdir) {
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
