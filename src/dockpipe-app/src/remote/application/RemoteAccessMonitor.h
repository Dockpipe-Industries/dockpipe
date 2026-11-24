#pragma once

#include "LauncherUiExport.h"
#include "RemoteCommand.h"

#include <QObject>
#include <QString>
#include <QTimer>

// MainWindow owns the monitor. Background checks never request authentication;
// only an explicit unlock starts the CLI's native operating-system ceremony.
class DOCKPIPE_LAUNCHER_UI_EXPORT RemoteAccessMonitor : public QObject {
    Q_OBJECT
public:
    enum class State { Checking, Unlocked, Locked, Unavailable };
    Q_ENUM(State)

    explicit RemoteAccessMonitor(QObject *parent = nullptr);
    ~RemoteAccessMonitor() override;

    State state() const;
    QString detail() const;
    bool busy() const;

public slots:
    void refresh();
    void unlock();
    void cancelUnlock();

signals:
    void changed();
    void unlockFinished(bool succeeded);

private:
    void refreshStatus(bool finishingUnlock = false, bool unlockSucceeded = false);
    void applyStatus(bool succeeded, const QByteArray &output);
    void setUnavailable(const QString &detail);
    void appendDiagnostics(const QString &text);

    State m_state = State::Checking;
    QString m_detail;
    QString m_unlockError;
    QByteArray m_diagnostics;
    bool m_busy = false;
    bool m_unlocking = false;
    QTimer m_pollTimer;
    // Destroy the command before other members: its destructor clears callbacks
    // and performs bounded child-process cleanup without touching monitor state.
    RemoteCommand m_command;
};
