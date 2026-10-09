#pragma once

#include "RemoteCommand.h"
#include "SessionManager.h"
#include "ContextStore.h"
#include <QWidget>

class QLabel;
class QPlainTextEdit;
class QTableWidget;

class ActivityWidget : public QWidget {
    Q_OBJECT
public:
    ActivityWidget(SessionManager &sessions, ContextStore &store, QWidget *parent = nullptr);
    void setWorkdir(const QString &workdir);
    void refreshLocal();
    void refreshRemote();
private:
    void run(const QStringList &args, RemoteCommand::Completion completion);
    QString selectedJob() const;
    SessionManager &m_sessions;
    ContextStore &m_store;
    RemoteCommand m_command;
    QString m_workdir;
    QTableWidget *m_local;
    QTableWidget *m_jobs;
    QPlainTextEdit *m_output;
    QLabel *m_status;
    bool m_remoteLoaded = false;
};
