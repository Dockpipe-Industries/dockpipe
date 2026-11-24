#pragma once

#include <QCoreApplication>

#include "ProjectGitClient.h"
#include <QDialog>

class GitDiffWidget;
class QLabel;
class QListWidget;

class GitCommitPreviewDialog final : public QDialog {
    Q_DECLARE_TR_FUNCTIONS(GitCommitPreviewDialog)
public:
    GitCommitPreviewDialog(const QString &workdir, const QString &commit,
        const QJsonObject &cachedReview, QWidget *parent = nullptr);

private:
    void showReview(const QJsonObject &review);

    ProjectGitClient m_reader;
    GitDiffWidget *m_diff;
    QListWidget *m_files;
    QLabel *m_fileHeading;
    QLabel *m_status;
};
