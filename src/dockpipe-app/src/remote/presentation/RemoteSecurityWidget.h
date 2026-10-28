#pragma once

#include "RemoteCommand.h"

#include <QWidget>

class QLabel;
class QCheckBox;
class QPushButton;
class QSpinBox;
class QJsonObject;

class RemoteSecurityWidget final : public QWidget {
    Q_OBJECT
public:
    explicit RemoteSecurityWidget(QWidget *parent = nullptr);

    void refresh();

private:
    void run(const QStringList &arguments, int timeoutMs,
             bool allowAuthentication,
             RemoteCommand::Completion completion);
    void applyStatus(const QJsonObject &status);
    void setUnavailable(const QString &detail, bool locked = false);
    void showDetail(const QString &detail);
    void clearOperationError();
    void setBusy(bool busy);
    void updateActions();

private slots:
    void savePolicy();

private:
    RemoteCommand m_command;
    QLabel *m_status = nullptr;
    QLabel *m_capabilities = nullptr;
    QLabel *m_detail = nullptr;
    QSpinBox *m_idleSeconds = nullptr;
    QCheckBox *m_lockOnScreenLock = nullptr;
    QCheckBox *m_lockOnSleep = nullptr;
    QPushButton *m_refresh = nullptr;
    QPushButton *m_unlock = nullptr;
    QPushButton *m_lock = nullptr;
    QPushButton *m_savePolicy = nullptr;
    bool m_supported = false;
    bool m_authorityAvailable = false;
    bool m_locked = false;
    bool m_idleSupported = false;
    bool m_screenLockSupported = false;
    bool m_sleepSupported = false;
    bool m_hasStatus = false;
    bool m_busy = false;
    bool m_policySavePending = false;
    QString m_operationError;
    QByteArray m_commandDiagnostics;
};
