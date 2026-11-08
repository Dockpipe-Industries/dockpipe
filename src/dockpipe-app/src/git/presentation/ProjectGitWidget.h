#pragma once

#include "ProjectGitClient.h"
#include <QJsonObject>
#include <QWidget>

class GitHistoryWidget;
class GitRepositoryWidget;
class GitConflictWidget;
class GitDiffWidget;
class QTabWidget;
class QLabel;
class QListWidget;
class QPlainTextEdit;
class QLineEdit;
class QTemporaryFile;
class QPushButton;
class QTreeWidget;
class QStackedWidget;

class ProjectGitWidget : public QWidget {
    Q_OBJECT
public:
    explicit ProjectGitWidget(QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    QString workdir() const { return m_workdir; }
    bool busy() const { return m_command.busy(); }
    void refresh();
    bool confirmLeave();
    void confirmLeaveAsync(std::function<void(bool)> completion);
    void setWorkdirAsync(const QString &workdir, std::function<void(bool)> completion = {});

private:
    void runAction(const QString &action);
    void runRequest(QJsonObject request);
    void render(const QJsonObject &snapshot);
    void updateCommitState();
    void selectFile(QListWidget *group);
    void stageGroup(bool stage);
    void filterFiles(const QString &query);
    void setFolderView(bool folders);
    void rebuildFolderViews();
    void syncFolderSelection();
    QTabWidget *m_tabs;
    GitHistoryWidget *m_history;
    GitRepositoryWidget *m_repository;
    GitConflictWidget *m_conflicts;
    QWidget *m_fileActions;
    QString m_workdir;
    QJsonObject m_snapshot;
    ProjectGitClient m_command;
    QLabel *m_heading;
    QLabel *m_status;
    QListWidget *m_files;
    QListWidget *m_staged;
    QListWidget *m_activeFiles;
    QTreeWidget *m_unstagedTree;
    QTreeWidget *m_stagedTree;
    QStackedWidget *m_unstagedViews;
    QStackedWidget *m_stagedViews;
    GitDiffWidget *m_diff;
    QLineEdit *m_message;
    QWidget *m_controls;
    QWidget *m_commitPanel;
    QLabel *m_count;
    QLabel *m_selection;
    QLabel *m_diffTitle;
};
