#pragma once

#include "Context.h"
#include "LauncherUiExport.h"

#include <QHash>
#include <QObject>
#include <QPointer>
#include <QProcess>
#include <QByteArray>
#include <QStringList>

enum class SessionStatus { Stopped, Starting, Running, Failed };

struct SessionInfo {
    QString contextId;
    SessionStatus status = SessionStatus::Stopped;
    QString logPath;
    QString program;
    QStringList arguments;
    qint64 pid = 0;
    QString errorString;
    bool ready = false;
    bool stopRequested = false;
};

class DOCKPIPE_LAUNCHER_UI_EXPORT SessionManager : public QObject {
    Q_OBJECT
public:
    explicit SessionManager(QObject *parent = nullptr);

    SessionInfo info(const QString &contextId) const;
    bool isRunning(const QString &contextId) const;
    bool isReady(const QString &contextId) const;

    /// Build dockpipe argv (program + args) for a context.
    static QStringList dockpipeArguments(const Context &ctx);
    static QString readinessMarker();

    // True admits an asynchronous start. Observe sessionStarted/sessionFailed
    // for the result; false rejects the request before a process is admitted.
    bool launch(const Context &ctx, const QString &logsDir);
    void stop(const QString &contextId);
    void sendInput(const QString &contextId, const QString &text);

signals:
    void sessionStarted(const QString &contextId);
    void sessionReady(const QString &contextId);
    void sessionStopped(const QString &contextId, int exitCode, QProcess::ExitStatus);
    void sessionFailed(const QString &contextId, const QString &error);
    void sessionOutput(const QString &contextId, const QString &text);
    void sessionPrompt(const QString &contextId, const QString &payload);

private:
    QHash<QString, QPointer<QProcess>> m_processes;
    QHash<QString, SessionInfo> m_info;
    QHash<QString, QByteArray> m_pendingOutput;
};
