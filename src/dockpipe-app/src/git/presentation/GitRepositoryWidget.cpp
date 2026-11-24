#include "GitRepositoryWidget.h"
#include "CanvasIcons.h"
#include "RemoteGitContext.h"

#include <QCheckBox>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QSplitter>
#include <QShowEvent>
#include <QTimer>
#include <QVBoxLayout>

GitRepositoryWidget::GitRepositoryWidget(QWidget *parent)
    : QWidget(parent), m_reader(this)
{
    setObjectName("gitRepository");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    m_controls = new QWidget;
    auto *content = new QVBoxLayout(m_controls);
    content->setContentsMargins(0, 0, 0, 0);
    content->setSpacing(14);
    auto *toolbar = new QHBoxLayout;
    m_remotes = new QLabel;
    m_remotes->setTextFormat(Qt::PlainText);
    m_remotes->setWordWrap(true);
    toolbar->addWidget(m_remotes, 1);
    auto *refreshButton = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Refresh), tr("Refresh"));
    refreshButton->setObjectName("gitRepositoryRefresh");
    toolbar->addWidget(refreshButton);
    connect(refreshButton, &QPushButton::clicked, this, [this] {
        // An explicit refresh re-reads repository metadata. Automatic status
        // presentation can reuse the catalog already supplied by that status.
        setProperty("gitRepositorySnapshotCatalog", false);
        refresh();
    });
    auto *fetch = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Download), tr("Fetch"));
    fetch->setObjectName("gitRepositoryFetch");
    toolbar->addWidget(fetch);
    connect(fetch, &QPushButton::clicked, this, [this] {
        emit actionRequested(QJsonObject{{"action", "fetch"}, {"snapshot", m_snapshotToken}});
    });
    content->addLayout(toolbar);

    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(12);
    auto *branches = new QWidget;
    branches->setProperty("gitCard", true);
    branches->setAttribute(Qt::WA_StyledBackground);
    auto *branchLayout = new QVBoxLayout(branches);
    branchLayout->setContentsMargins(16, 16, 16, 16);
    branchLayout->setSpacing(12);
    auto *branchHeading = new QLabel(tr("Branches"));
    branchHeading->setObjectName("gitPanelTitle");
    branchLayout->addWidget(branchHeading);
    m_branches = new QListWidget;
    m_branches->setObjectName("gitRepositoryBranches");
    branchLayout->addWidget(m_branches, 1);
    m_branchName = new QLineEdit;
    m_branchName->setObjectName("gitRepositoryBranchName");
    m_branchName->setPlaceholderText(tr("New branch name"));
    m_branchName->setMaxLength(1024);
    branchLayout->addWidget(m_branchName);
    auto *create = new QPushButton(tr("Create branch"));
    create->setObjectName("gitRepositoryBranchCreate");
    branchLayout->addWidget(create);
    connect(create, &QPushButton::clicked, this, [this] {
        const QString name = m_branchName->text().trimmed();
        if (!name.isEmpty())
            emit actionRequested(QJsonObject{{"action", "branch-create"}, {"name", name}, {"snapshot", m_snapshotToken}});
    });
    auto *branchActions = new QHBoxLayout;
    for (const auto &entry : QList<QPair<QString, QString>>{
             {tr("Switch"), "branch-switch"}, {tr("Merge into current"), "merge"}, {tr("Delete"), "branch-delete"}}) {
        auto *button = new QPushButton(entry.first);
        button->setObjectName("gitRepository-" + entry.second);
        branchActions->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, action = entry.second] { requestBranchAction(action); });
    }
    branchLayout->addLayout(branchActions);
    splitter->addWidget(branches);

    auto *stashes = new QWidget;
    stashes->setProperty("gitCard", true);
    stashes->setAttribute(Qt::WA_StyledBackground);
    auto *stashLayout = new QVBoxLayout(stashes);
    stashLayout->setContentsMargins(16, 16, 16, 16);
    stashLayout->setSpacing(12);
    auto *stashHeading = new QLabel(tr("Stashes"));
    stashHeading->setObjectName("gitPanelTitle");
    stashLayout->addWidget(stashHeading);
    m_stashes = new QListWidget;
    m_stashes->setObjectName("gitRepositoryStashes");
    stashLayout->addWidget(m_stashes, 1);
    m_stashMessage = new QLineEdit;
    m_stashMessage->setObjectName("gitRepositoryStashMessage");
    m_stashMessage->setPlaceholderText(tr("Stash description (optional)"));
    m_stashMessage->setMaxLength(4096);
    stashLayout->addWidget(m_stashMessage);
    m_includeUntracked = new QCheckBox(tr("Include untracked files"));
    m_includeUntracked->setObjectName("gitRepositoryStashUntracked");
    stashLayout->addWidget(m_includeUntracked);
    auto *stashActions = new QHBoxLayout;
    for (const auto &entry : QList<QPair<QString, QString>>{
             {tr("Save"), "stash-save"}, {tr("Apply"), "stash-apply"}, {tr("Pop"), "stash-pop"}, {tr("Drop"), "stash-drop"}}) {
        auto *button = new QPushButton(entry.first);
        button->setObjectName("gitRepository-" + entry.second);
        stashActions->addWidget(button);
        connect(button, &QPushButton::clicked, this, [this, action = entry.second] { requestStashAction(action); });
    }
    stashLayout->addLayout(stashActions);
    splitter->addWidget(stashes);
    content->addWidget(splitter, 1);
    layout->addWidget(m_controls, 1);
    m_status = new QLabel;
    m_status->setObjectName("gitRepositoryStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    connect(&m_reader, &ProjectGitClient::busyChanged, this, [this](bool busy) {
        updateControls();
        if (!busy && m_refreshPending && isVisible())
            QTimer::singleShot(0, this, &GitRepositoryWidget::refresh);
    });
    updateControls();
}

void GitRepositoryWidget::setSnapshot(const QString &workdir, const QJsonObject &snapshot)
{
    if (m_workdir != workdir) {
        ++m_generation;
        m_reader.cancel();
        m_workdir = workdir;
        m_branches->clear();
        m_stashes->clear();
        setProperty("gitRepositoryRenderedCatalog", QVariant());
        setProperty("gitRepositoryRenderedBranch", QVariant());
        m_branchName->clear();
        m_stashMessage->clear();
        m_status->clear();
    }
    const QString token = snapshot.value("snapshot").toString();
    if (!m_snapshotToken.isEmpty() && token != m_snapshotToken) {
        ++m_generation;
        m_reader.cancel();
    }
    m_snapshotToken = token;
    m_currentBranch = snapshot.value("branch").toString();
    const auto operation = snapshot.value("operation").toObject();
    m_operationActive = !operation.value("kind").toString().isEmpty();
    const bool catalogAvailable = snapshot.value("catalog").isObject();
    setProperty("gitRepositorySnapshotCatalog", catalogAvailable);
    if (catalogAvailable)
        renderBranches(snapshot.value("catalog").toObject());
    m_refreshPending = true;
    updateControls();
    // Hidden branch/stash panels must not add remote round trips to opening
    // Changes or comparing files. The current snapshot token remains fresh.
    if (isVisible())
        refresh();
}

void GitRepositoryWidget::setBusy(bool busy)
{
    m_busy = busy;
    updateControls();
    if (!busy && m_refreshPending && isVisible())
        refresh();
}

void GitRepositoryWidget::refresh()
{
    if (!isVisible()) {
        m_refreshPending = true;
        return;
    }
    if (m_workdir.isEmpty() || m_busy || m_reader.busy())
        return;
    m_refreshPending = false;
    const auto generation = m_generation;
    const auto workdir = m_workdir;
    m_status->setText(tr("Loading branches and stashes…"));
    auto loadStashes = [this, generation, workdir] {
        if (!isVisible()) {
            setProperty("gitRepositorySnapshotCatalog", true);
            m_refreshPending = true;
            return;
        }
        m_reader.run(workdir, QJsonObject{{"action", "stash-list"}},
            [this, generation](bool ok, const QJsonObject &reply, const QString &error) {
                if (generation != m_generation)
                    return;
                if (ok) {
                    renderStashes(reply);
                    m_status->setText(m_catalogNotice);
                } else {
                    m_status->setText(error);
                }
                updateControls();
            });
    };
    const bool catalogAvailable = property("gitRepositorySnapshotCatalog").toBool();
    setProperty("gitRepositorySnapshotCatalog", false);
    if (catalogAvailable) {
        loadStashes();
        return;
    }
    m_reader.run(workdir, QJsonObject{{"action", "catalog"}},
        [this, generation, loadStashes](bool ok, const QJsonObject &reply, const QString &error) {
            if (generation != m_generation)
                return;
            if (!ok) {
                m_status->setText(error);
                return;
            }
            renderBranches(reply.value("catalog").toObject());
            loadStashes();
        });
}

void GitRepositoryWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_refreshPending)
        QTimer::singleShot(0, this, &GitRepositoryWidget::refresh);
}

void GitRepositoryWidget::renderBranches(const QJsonObject &catalog)
{
    if (property("gitRepositoryRenderedCatalog").isValid()
        && property("gitRepositoryRenderedCatalog").toJsonObject() == catalog
        && property("gitRepositoryRenderedBranch").toString() == m_currentBranch)
        return;
    setProperty("gitRepositoryRenderedCatalog", catalog);
    setProperty("gitRepositoryRenderedBranch", m_currentBranch);
    QString selected;
    if (m_branches->currentItem())
        selected = m_branches->currentItem()->data(Qt::UserRole).toString();
    m_catalogNotice = catalog.value("notice").toString();
    m_branches->clear();
    for (const auto &value : catalog.value("branches").toArray()) {
        const auto branch = value.toObject();
        const QString name = branch.value("name").toString();
        const QString label = name == m_currentBranch ? tr("%1 (current)").arg(name) : name;
        auto *item = new QListWidgetItem(label, m_branches);
        item->setData(Qt::UserRole, name);
        item->setData(Qt::UserRole + 1, branch.value("id").toString());
        item->setToolTip(tr("%1\n%2\nUpstream: %3\n%4")
            .arg(name, branch.value("id").toString(), branch.value("upstream").toString(), branch.value("divergence").toString()));
        if (name == selected)
            m_branches->setCurrentItem(item);
    }
    QStringList remotes;
    for (const auto &remote : catalog.value("remotes").toArray())
        remotes.append(remote.toString());
    m_remotes->setText(remotes.isEmpty() ? tr("No remotes configured") : tr("Remotes: %1").arg(remotes.join(", ")));
}

void GitRepositoryWidget::renderStashes(const QJsonObject &reply)
{
    QString selected;
    if (m_stashes->currentItem())
        selected = m_stashes->currentItem()->data(Qt::UserRole).toString();
    m_stashes->clear();
    for (const auto &value : reply.value("stashes").toArray()) {
        const auto stash = value.toObject();
        const QString id = stash.value("id").toString();
        auto *item = new QListWidgetItem(tr("%1  %2").arg(stash.value("selector").toString(), stash.value("subject").toString()), m_stashes);
        item->setData(Qt::UserRole, id);
        item->setData(Qt::UserRole + 1, stash.value("selector").toString());
        item->setToolTip(id);
        if (id == selected)
            m_stashes->setCurrentItem(item);
    }
}

void GitRepositoryWidget::updateControls()
{
    m_controls->setEnabled(!m_busy && !m_reader.busy() && !m_workdir.isEmpty() && !m_operationActive);
    const bool writable = RemoteGitContext::canWrite(this);
    for (const auto *name : {"gitRepositoryFetch", "gitRepositoryBranchCreate", "gitRepository-branch-switch",
                            "gitRepository-merge", "gitRepository-branch-delete", "gitRepository-stash-save",
                            "gitRepository-stash-apply", "gitRepository-stash-pop", "gitRepository-stash-drop"}) {
        findChild<QPushButton *>(name)->setEnabled(writable);
    }
}

bool GitRepositoryWidget::confirm(const QString &title, const QString &text)
{
    QMessageBox dialog(QMessageBox::Question, title, text, QMessageBox::Yes | QMessageBox::Cancel, this);
    dialog.setTextFormat(Qt::PlainText);
    dialog.setDefaultButton(QMessageBox::Cancel);
    return dialog.exec() == QMessageBox::Yes;
}

void GitRepositoryWidget::requestBranchAction(const QString &action)
{
    auto *item = m_branches->currentItem();
    if (!item)
        return;
    const QString name = item->data(Qt::UserRole).toString();
    QJsonObject request{{"action", action}, {"snapshot", m_snapshotToken}};
    if (action == "merge") {
        request.insert("commit", item->data(Qt::UserRole + 1).toString());
        if (!confirm(tr("Merge branch"), tr("Merge %1 into the current branch? Conflicts will open in the conflict guide.").arg(name)))
            return;
        request.insert("ref", name);
        request.insert("confirm", true);
    } else {
        request.insert("name", name);
        if (action == "branch-delete") {
            if (!confirm(tr("Delete branch"), tr("Delete local branch %1? Git will reject deletion if it contains unmerged work.").arg(name)))
                return;
            request.insert("confirm", true);
        }
    }
    emit actionRequested(request);
}

void GitRepositoryWidget::requestStashAction(const QString &action)
{
    QJsonObject request{{"action", action}, {"snapshot", m_snapshotToken}};
    if (action == "stash-save") {
        request.insert("message", m_stashMessage->text());
        request.insert("include_untracked", m_includeUntracked->isChecked());
    } else {
        auto *item = m_stashes->currentItem();
        if (!item)
            return;
        const QString id = item->data(Qt::UserRole).toString();
        request.insert("stash", id);
        request.insert("stash_selector", item->data(Qt::UserRole + 1).toString());
        if (action == "stash-drop" || action == "stash-pop") {
            const QString text = action == "stash-drop"
                ? tr("Permanently drop selected stash %1?").arg(id)
                : tr("Apply selected stash %1 and remove it if applying succeeds?").arg(id);
            if (!confirm(action == "stash-drop" ? tr("Drop stash") : tr("Pop stash"), text))
                return;
            request.insert("confirm", true);
        }
    }
    emit actionRequested(request);
}
