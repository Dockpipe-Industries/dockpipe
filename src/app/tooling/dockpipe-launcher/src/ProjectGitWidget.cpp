#include "ProjectGitWidget.h"
#include "DockpipeChoices.h"
#include "CanvasIcons.h"
#include "SourceEditor.h"

#include <QApplication>
#include <QStyle>
#include <QFontDatabase>
#include <QFileInfo>
#include <QPainter>
#include <QStyledItemDelegate>
#include <QTimer>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSplitter>
#include <QTemporaryFile>
#include <QVBoxLayout>

namespace {
class ChangedFileDelegate final : public QStyledItemDelegate {
public:
    using QStyledItemDelegate::QStyledItemDelegate;
    QSize sizeHint(const QStyleOptionViewItem &, const QModelIndex &) const override { return QSize(260, 58); }
    void paint(QPainter *painter, const QStyleOptionViewItem &option, const QModelIndex &index) const override
    {
        QStyleOptionViewItem base(option);
        initStyleOption(&base, index);
        base.text.clear();
        base.icon = QIcon();
        QApplication::style()->drawControl(QStyle::CE_ItemViewItem, &base, painter, option.widget);
        painter->save();
        const QRect row = option.rect.adjusted(32, 6, -12, -6);
        CanvasIcons::icon(CanvasIcons::Symbol::Code).paint(painter, QRect(row.left(), row.top() + 5, 18, 18));
        const QString path = index.data(Qt::UserRole).toString();
        const QFileInfo info(path);
        const int left = row.left() + 28;
        const int width = qMax(0, row.width() - 64);
        painter->setPen(option.palette.color(QPalette::Text));
        painter->drawText(QRect(left, row.top(), width, 22), Qt::AlignVCenter,
            option.fontMetrics.elidedText(info.fileName(), Qt::ElideMiddle, width));
        painter->setPen(option.palette.color(QPalette::PlaceholderText));
        auto small = option.font;
        small.setPointSizeF(qMax(8.0, small.pointSizeF() - 1));
        painter->setFont(small);
        painter->drawText(QRect(left, row.top() + 23, width, 18), Qt::AlignVCenter,
            QFontMetrics(small).elidedText(info.path() == "." ? QObject::tr("Project root") : info.path(), Qt::ElideMiddle, width));
        const QString status = index.data(Qt::UserRole + 1).toString().trimmed();
        const bool added = status == "??" || status.contains('A');
        QColor color = added ? QColor("#46b994") : QColor("#d5a454");
        if (status.contains('D'))
            color = QColor("#e17e89");
        const QRect badge(row.right() - 22, row.top() + 4, 24, 24);
        QColor fill = color;
        fill.setAlpha(24);
        painter->setPen(Qt::NoPen);
        painter->setBrush(fill);
        painter->drawRoundedRect(badge, 5, 5);
        painter->setPen(color);
        painter->drawText(badge, Qt::AlignCenter, added ? "+" : status.left(1));
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
    auto *title = new QLabel(tr("Source control"));
    title->setObjectName("gitHeading");
    header->addWidget(title);
    m_heading = new QLabel(tr("No project"));
    m_heading->setObjectName("gitBranch");
    m_heading->setTextFormat(Qt::PlainText);
    header->addWidget(m_heading);
    header->addStretch();
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
    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(16);
    auto *changes = new QWidget;
    auto *changesLayout = new QVBoxLayout(changes);
    changesLayout->setContentsMargins(0, 0, 0, 0);
    m_count = new QLabel(tr("CHANGES"));
    m_count->setObjectName("editorSectionLabel");
    changesLayout->addWidget(m_count);
    m_files = new QListWidget;
    m_files->setObjectName("gitChangedFiles");
    m_files->setMinimumWidth(260);
    m_files->setItemDelegate(new ChangedFileDelegate(m_files));
    m_files->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_files->setSpacing(3);
    changesLayout->addWidget(m_files, 1);
    m_commitPanel = new QWidget;
    m_commitPanel->setObjectName("gitCommitPanel");
    auto *commitLayout = new QVBoxLayout(m_commitPanel);
    commitLayout->setContentsMargins(14, 14, 14, 14);
    commitLayout->setSpacing(12);
    m_selection = new QLabel(tr("Select files to commit"));
    m_selection->setObjectName("editorMuted");
    commitLayout->addWidget(m_selection);
    m_message = new QLineEdit;
    m_message->setObjectName("gitCommitMessage");
    m_message->setPlaceholderText(tr("Describe your changes…"));
    m_message->setMaxLength(4096);
    commitLayout->addWidget(m_message);
    auto *commit = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Check), tr("Commit selected"));
    commit->setObjectName("gitCommitSelected");
    commitLayout->addWidget(commit);
    auto *hint = new QLabel(tr("Commits include saved files. Push when ready."));
    hint->setObjectName("editorMuted");
    hint->setWordWrap(true);
    commitLayout->addWidget(hint);
    connect(commit, &QPushButton::clicked, this, [this]() { runAction("commit"); });
    changesLayout->addWidget(m_commitPanel);
    splitter->addWidget(changes);
    auto *review = new QWidget;
    review->setObjectName("editorWorkspace");
    auto *reviewLayout = new QVBoxLayout(review);
    reviewLayout->setContentsMargins(0, 0, 0, 0);
    m_diffTitle = new QLabel(tr("Select a file to review"));
    m_diffTitle->setTextFormat(Qt::PlainText);
    m_diffTitle->setMargin(16);
    m_diffTitle->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    reviewLayout->addWidget(m_diffTitle);
    m_diff = new SourceEditor;
    m_diff->setObjectName("projectGitDiff");
    m_diff->setReadOnly(true);
    m_diff->setPlaceholderText(tr("Select a changed file to review its saved changes."));
    reviewLayout->addWidget(m_diff, 1);
    splitter->addWidget(review);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({320, 800});
    layout->addWidget(splitter, 1);
    connect(m_files, &QListWidget::currentItemChanged, this, [this](QListWidgetItem *item) {
        if (item)
            runAction("diff");
    });
    connect(m_files, &QListWidget::itemChanged, this, &ProjectGitWidget::updateCommitState);
    connect(m_message, &QLineEdit::textChanged, this, &ProjectGitWidget::updateCommitState);
    updateCommitState();
    auto *footer = new QHBoxLayout;
    m_status = new QLabel;
    m_status->setObjectName("editorMuted");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    footer->addWidget(m_status, 1);
    auto *cancel = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Close), tr("Cancel operation"));
    cancel->hide();
    footer->addWidget(cancel);
    layout->addLayout(footer);
    connect(cancel, &QPushButton::clicked, &m_command, &RemoteCommand::cancel);
    connect(&m_command, &RemoteCommand::busyChanged, this, [this, cancel](bool busy) {
        m_controls->setEnabled(!busy);
        m_commitPanel->setEnabled(!busy);
        m_files->setEnabled(!busy);
        cancel->setVisible(busy);
    });
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        if (m_diagnostics.size() < 8192)
            m_diagnostics += text.toUtf8().left(8192 - m_diagnostics.size());
    });
}

void ProjectGitWidget::setWorkdir(const QString &workdir)
{
    if (busy() || m_workdir == workdir)
        return;
    m_workdir = workdir;
    m_snapshot = {};
    m_files->clear();
    m_diff->clear();
    m_message->clear();
    m_heading->setText(QFileInfo(workdir).fileName());
    m_heading->setToolTip(workdir);
    m_status->setText(tr("Refresh to review this project's changes."));
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
    QString reviewedPath;
    if (action == "diff") {
        if (!m_files->currentItem())
            return;
        reviewedPath = m_files->currentItem()->data(Qt::UserRole).toString();
        request["paths"] = QJsonArray{reviewedPath};
        m_diffTitle->setText(reviewedPath);
        m_diff->clear();
    }
    if (action == "commit") {
        QJsonArray paths;
        for (int row = 0; row < m_files->count(); ++row) {
            const auto *item = m_files->item(row);
            if (item->checkState() == Qt::Checked)
                paths.append(item->data(Qt::UserRole).toString());
        }
        if (paths.isEmpty() || m_message->text().trimmed().isEmpty()) {
            m_status->setText(tr("Select files and enter a commit message first."));
            return;
        }
        request["paths"] = paths;
        request["message"] = m_message->text();
    }
    m_request = new QTemporaryFile(this);
    const QByteArray bytes = QJsonDocument(request).toJson(QJsonDocument::Compact);
    if (!m_request->open() || m_request->write(bytes) != bytes.size() || !m_request->flush()) {
        m_status->setText(tr("Cannot prepare the Git action."));
        delete m_request;
        m_request = nullptr;
        return;
    }
    m_request->close();
    m_diagnostics.clear();
    m_status->setText(tr("%1…").arg(action));
    m_command.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), {"git", "--request", m_request->fileName()},
        [this, action, reviewedPath](bool ok, const QByteArray &output) {
            delete m_request;
            m_request = nullptr;
            QJsonParseError error;
            const auto document = QJsonDocument::fromJson(output, &error);
            if (!ok || error.error != QJsonParseError::NoError || !document.isObject()) {
                m_snapshot = {};
                updateCommitState();
                m_status->setText(tr("Git action did not finish successfully. Refresh before retrying.\n%1").arg(QString::fromUtf8(m_diagnostics)));
                return;
            }
            if (action == "diff") {
                const auto snapshot = document.object();
                static_cast<SourceEditor *>(m_diff)->setFileName(m_files->currentItem()->data(Qt::UserRole + 1).toString() == "??"
                    ? reviewedPath : "review.diff");
                m_diff->setPlainText(snapshot.value("diff").toString());
                m_status->setText(snapshot.value("notice").toString(tr("Reviewing saved changes in %1").arg(reviewedPath)));
                return;
            }
            if (action == "commit")
                m_message->clear();
            render(document.object());
        }, 125000);
}

void ProjectGitWidget::render(const QJsonObject &snapshot)
{
    m_snapshot = snapshot;
    const QString branch = snapshot.value("branch").toString();
    m_heading->setText(branch.isEmpty() ? tr("Detached HEAD") : branch);
    m_heading->setToolTip(snapshot.value("root").toString());
    const QString previous = m_files->currentItem() ? m_files->currentItem()->data(Qt::UserRole).toString() : QString();
    QStringList checked;
    for (int row = 0; row < m_files->count(); ++row) {
        if (m_files->item(row)->checkState() == Qt::Checked)
            checked.append(m_files->item(row)->data(Qt::UserRole).toString());
    }
    const QSignalBlocker block(m_files);
    m_files->clear();
    int selectedRow = 0;
    for (const auto &value : snapshot.value("changes").toArray()) {
        const auto change = value.toObject();
        const QString path = change.value("path").toString();
        auto *item = new QListWidgetItem(path, m_files);
        item->setData(Qt::UserRole, path);
        item->setData(Qt::UserRole + 1, change.value("status").toString());
        item->setToolTip(path + " · " + change.value("status").toString());
        item->setCheckState(checked.contains(path) ? Qt::Checked : Qt::Unchecked);
        if (path == previous)
            selectedRow = m_files->count() - 1;
    }
    m_count->setText(tr("CHANGES   %1").arg(m_files->count()));
    updateCommitState();
    m_diff->clear();
    m_diffTitle->setText(m_files->count() == 0 ? tr("All changes committed") : tr("Select a file to review"));
    m_status->setText(snapshot.value("notice").toString(m_files->count() == 0 ? tr("Working tree is clean.")
        : tr("Select files to include in your next commit.")));
    if (m_files->count() > 0) {
        m_files->setCurrentRow(selectedRow);
        runAction("diff");
    }
}

void ProjectGitWidget::updateCommitState()
{
    int selected = 0;
    for (int row = 0; row < m_files->count(); ++row)
        selected += m_files->item(row)->checkState() == Qt::Checked;
    m_selection->setText(tr("%1 of %2 files selected").arg(selected).arg(m_files->count()));
    auto *commit = findChild<QPushButton *>("gitCommitSelected");
    commit->setEnabled(selected > 0 && !m_message->text().trimmed().isEmpty() && !m_snapshot.isEmpty());
}
