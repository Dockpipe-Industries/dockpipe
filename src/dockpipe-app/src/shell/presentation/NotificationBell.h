#pragma once

#include <QPointer>
#include <QDateTime>
#include <QVector>
#include <QToolButton>

class RemoteAccessMonitor;
class QDialog;
class QAction;
class QLabel;
class QPushButton;

class NotificationBell final : public QToolButton {
    Q_OBJECT
public:
    explicit NotificationBell(RemoteAccessMonitor &monitor, QWidget *parent = nullptr);
    enum class WorkflowOutcome { Completed, Failed, Stopped };
    void notifyWorkflow(const QString &title, const QString &logPath, WorkflowOutcome outcome);
    void notifyRemoteWorkflow(const QString &title, const QString &jobId, WorkflowOutcome outcome);
    int unreadCount() const;
    QWidget *createMachinesNotice(QWidget *parent);
    void machinesEntered();
    void machinesLeft();

signals:
    void securitySettingsRequested();
    void workflowLogsRequested(const QString &title, const QString &logPath);
    void workflowHistoryRequested();
    void remoteJobRequested(const QString &jobId);

private:
    void updateState();
    void showUnlockPrompt();

    struct WorkflowNotice {
        quint64 id;
        QString title;
        QString logPath;
        WorkflowOutcome outcome;
        QDateTime time;
        bool unread = true;
        QString remoteJob;
    };
    QVector<WorkflowNotice> m_workflows;
    quint64 m_nextId = 0;
    RemoteAccessMonitor &m_monitor;
    QPointer<QLabel> m_notificationDetail;
    QPointer<QAction> m_unlockAction;
    QPointer<QDialog> m_prompt;
    QPointer<QLabel> m_promptDetail;
    QPointer<QPushButton> m_promptUnlock;
    QPointer<QWidget> m_notice;
    QPointer<QLabel> m_noticeText;
    QPointer<QPushButton> m_noticeAction;
    bool m_machinesActive = false;
    bool m_visitPending = false;
    bool m_unlockRequested = false;
};
