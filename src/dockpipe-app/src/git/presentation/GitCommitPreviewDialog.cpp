#include "GitCommitPreviewDialog.h"
#include "FileExplorerIcons.h"
#include "GitDiffWidget.h"

#include <QDialogButtonBox>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QScreen>
#include <QPushButton>
#include <QSplitter>
#include <QTimer>
#include <QVBoxLayout>

GitCommitPreviewDialog::GitCommitPreviewDialog(const QString &workdir, const QString &commit,
    const QJsonObject &cachedReview, QWidget *parent)
    : QDialog(parent), m_reader(this)
{
    setObjectName("gitCommitPreview");
    setWindowTitle(tr("Commit preview · %1").arg(commit.left(12)));
    setAttribute(Qt::WA_StyledBackground);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(20, 20, 20, 16);
    layout->setSpacing(16);

    auto *splitter = new QSplitter;
    splitter->setChildrenCollapsible(false);
    splitter->setHandleWidth(12);
    auto *filesPanel = new QWidget;
    filesPanel->setProperty("gitCard", true);
    filesPanel->setAttribute(Qt::WA_StyledBackground);
    filesPanel->setMinimumWidth(200);
    auto *filesLayout = new QVBoxLayout(filesPanel);
    filesLayout->setContentsMargins(14, 14, 14, 14);
    filesLayout->setSpacing(12);
    m_fileHeading = new QLabel(tr("Changed files"));
    m_fileHeading->setObjectName("gitPanelTitle");
    filesLayout->addWidget(m_fileHeading);
    auto *filter = new QLineEdit;
    filter->setObjectName("gitCommitPreviewFilter");
    filter->setPlaceholderText(tr("Filter changed files…"));
    filter->setClearButtonEnabled(true);
    filesLayout->addWidget(filter);
    m_files = new QListWidget;
    m_files->setObjectName("gitCommitPreviewFiles");
    m_files->setAccessibleName(tr("Files changed in this commit"));
    m_files->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_files->setTextElideMode(Qt::ElideMiddle);
    filesLayout->addWidget(m_files, 1);
    splitter->addWidget(filesPanel);
    m_diff = new GitDiffWidget;
    m_diff->setObjectName("gitCommitPreviewDiff");
    m_diff->setFileSelectorVisible(false);
    splitter->addWidget(m_diff);
    splitter->setStretchFactor(1, 1);
    splitter->setSizes({290, 950});
    layout->addWidget(splitter, 1);
    connect(m_files, &QListWidget::currentRowChanged, this, [this](int row) {
        m_diff->selectFile(row);
        if (row >= 0)
            m_status->setText(m_files->item(row)->data(Qt::UserRole).toString());
    });
    connect(filter, &QLineEdit::textChanged, this, [this](const QString &query) {
        for (int row = 0; row < m_files->count(); ++row) {
            auto *item = m_files->item(row);
            item->setHidden(!item->data(Qt::UserRole).toString().contains(query, Qt::CaseInsensitive));
        }
    });
    auto *footer = new QHBoxLayout;
    m_status = new QLabel(tr("Loading commit changes…"));
    m_status->setObjectName("gitCommitPreviewStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    footer->addWidget(m_status, 1);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close);
    buttons->button(QDialogButtonBox::Close)->setAutoDefault(false);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    connect(this, &QDialog::finished, &m_reader, &ProjectGitClient::cancel);
    footer->addWidget(buttons);
    layout->addLayout(footer);
    const QSize available = screen()->availableGeometry().size() - QSize(64, 64);
    resize(qMin(1320, available.width()), qMin(860, available.height()));
    if (!cachedReview.isEmpty()) {
        showReview(cachedReview);
    } else {
        QTimer::singleShot(0, this, [this, workdir, commit] {
            m_reader.run(workdir, {{"action", "commit-review"}, {"commit", commit}},
                [this](bool ok, const QJsonObject &reply, const QString &error) {
                    if (!ok) {
                        m_status->setText(error);
                        return;
                    }
                    showReview(reply.value("commit_review").toObject());
                });
        });
    }
}

void GitCommitPreviewDialog::showReview(const QJsonObject &review)
{
    m_diff->setCommitReview(review);
    const auto paths = m_diff->changedFiles();
    m_files->clear();
    for (const auto &path : paths) {
        const QFileInfo info(path);
        auto *item = new QListWidgetItem(FileExplorerIcons::instance().icon(info),
            info.fileName() + '\n' + (info.path() == "." ? tr("Project root") : info.path()), m_files);
        item->setData(Qt::UserRole, path);
        item->setToolTip(path);
    }
    m_fileHeading->setText(tr("Changed files (%1)").arg(paths.size()));
    if (!paths.isEmpty())
        m_files->setCurrentRow(0);
    const QString notice = review.value("notice").toString();
    if (!notice.isEmpty())
        m_status->setText(notice);
    else if (paths.isEmpty())
        m_status->setText(tr("No file changes available in this preview."));
}
