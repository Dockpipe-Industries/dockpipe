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
    void refreshRemote(bool allowAuthentication = false);
    void setWorkspaceFilter(bool enabled);
    void showRemoteJobs();
    void showRemoteJob(const QString &jobId);
signals:
    void browseWorkflowsRequested();
private:
    void run(const QStringList &args, RemoteCommand::Completion completion, bool allowAuthentication = false);
    void loadRemoteJobs(bool allowAuthentication = false);
    QString selectedJob() const;
    QString selectedRunningJob() const;
    void refreshRemoteLog();
    void clearRemoteLog();
    void updateRunControls();
    SessionManager &m_sessions;
    ContextStore &m_store;
    RemoteCommand m_command;
    RemoteCommand m_logCommand;
    QString m_workdir;
    QTableWidget *m_local;
    QTableWidget *m_jobs;
    QComboBox *m_location;
    QStackedWidget *m_localPages;
    QStackedWidget *m_remotePages;
    QLabel *m_count;
    QPlainTextEdit *m_output;
    QPlainTextEdit *m_remoteLog;
    QLabel *m_status;
    QPushButton *m_cancelJob;
    QPushButton *m_downloadResult;
    QPushButton *m_openLogs;
    QPushButton *m_stopRun;
    QString m_requestedJob;
    QString m_displayedLogJob;
    bool m_remoteLoaded = false;
    bool m_workspaceOnly = false;
};
