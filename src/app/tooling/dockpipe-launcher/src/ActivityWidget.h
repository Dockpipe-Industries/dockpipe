#pragma once

#include "RemoteCommand.h"
#include "SessionManager.h"
#include "ContextStore.h"
#include <QWidget>

class QLabel;
class QPlainTextEdit;
class QPushButton;
class QTableWidget;
class QComboBox;
class QStackedWidget;

class ActivityWidget : public QWidget {
    Q_OBJECT
public:
    ActivityWidget(SessionManager &sessions, ContextStore &store, QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void refreshLocal();
    void refreshRemote();
    void setWorkspaceFilter(bool enabled);
    void showRemoteJobs();
signals:
    void browseWorkflowsRequested();
private:
    void run(const QStringList &args, RemoteCommand::Completion completion);
    void loadRemoteJobs();
    QString selectedJob() const;
    void updateRunControls();
    SessionManager &m_sessions;
    ContextStore &m_store;
    RemoteCommand m_command;
    QString m_workdir;
    QTableWidget *m_local;
    QTableWidget *m_jobs;
    QComboBox *m_location;
    QStackedWidget *m_localPages;
    QStackedWidget *m_remotePages;
    QLabel *m_count;
    QPlainTextEdit *m_output;
    QLabel *m_status;
    QPushButton *m_cancelJob;
    QPushButton *m_downloadResult;
    QPushButton *m_openLogs;
    QPushButton *m_stopRun;
    bool m_remoteLoaded = false;
    bool m_workspaceOnly = false;
};
