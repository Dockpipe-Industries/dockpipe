#pragma once

#include "ProjectGitClient.h"
#include <QJsonObject>
#include <QWidget>

class GitHistoryWidget;
class GitRepositoryWidget;
class GitConflictWidget;
class QTabWidget;
class QComboBox;
class QLabel;
class QListWidget;
class QPlainTextEdit;
class QLineEdit;
class QTemporaryFile;
class QPushButton;

class ProjectGitWidget : public QWidget {
    Q_OBJECT
public:
    explicit ProjectGitWidget(QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    bool busy() const { return m_command.busy(); }
    void refresh();
    bool confirmLeave();

private:
    void runAction(const QString &action);
    void runRequest(QJsonObject request);
    void render(const QJsonObject &snapshot);
    void updateCommitState();
    QTabWidget *m_tabs;
    GitHistoryWidget *m_history;
    GitRepositoryWidget *m_repository;
    GitConflictWidget *m_conflicts;
    QComboBox *m_comparison;
    QWidget *m_fileActions;
    QString m_workdir;
    QJsonObject m_snapshot;
    ProjectGitClient m_command;
    QLabel *m_heading;
    QLabel *m_status;
    QListWidget *m_files;
    QPlainTextEdit *m_diff;
    QLineEdit *m_message;
    QWidget *m_controls;
    QWidget *m_commitPanel;
    QLabel *m_count;
    QLabel *m_selection;
    QLabel *m_diffTitle;
};
