#pragma once

#include "RemoteCommand.h"
#include <QJsonObject>
#include <QWidget>

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

private:
    void runAction(const QString &action);
    void render(const QJsonObject &snapshot);
    void updateCommitState();
    QString m_workdir;
    QJsonObject m_snapshot;
    QByteArray m_diagnostics;
    RemoteCommand m_command;
    QTemporaryFile *m_request = nullptr;
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
