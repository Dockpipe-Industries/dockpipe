#pragma once

#include "LauncherUiExport.h"
#include "RemoteCommand.h"

#include <QHash>
#include <QObject>
#include <QTimer>

// Owned by MainWindow; tracks submissions from this app session only.
class DOCKPIPE_LAUNCHER_UI_EXPORT RemoteWorkflowMonitor : public QObject {
    Q_OBJECT
public:
    explicit RemoteWorkflowMonitor(QObject *parent = nullptr);
    ~RemoteWorkflowMonitor() override;

    // At most 128 outstanding submissions, retained until terminal state or exit.
    bool track(const QString &jobId, const QString &title);

public slots:
    void setAccessUnlocked(bool unlocked);
    void refresh();

signals:
    void finished(const QString &jobId, const QString &title, const QString &outcome);

private:
    void updatePolling();

    QHash<QString, QString> m_pending;
    QTimer m_pollTimer;
    bool m_accessUnlocked = false;
    // Revoke an in-flight response after access locks, including a rapid unlock.
    quint64 m_accessGeneration = 0;
    RemoteCommand m_command;
};
