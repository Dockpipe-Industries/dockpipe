#pragma once

#include "LauncherUiExport.h"

#include <QObject>
#include <QProcess>
#include <QTimer>
#include <QJsonObject>
#include <QSet>
#include <functional>

// All remote UI actions use the CLI and its private credential store.
class DOCKPIPE_LAUNCHER_UI_EXPORT RemoteCommand : public QObject {
    Q_OBJECT
public:
    using Completion = std::function<void(bool, const QByteArray &)>;
    explicit RemoteCommand(QObject *parent = nullptr);
    ~RemoteCommand() override;
    bool busy() const;
    void run(const QString &program, const QStringList &arguments, Completion completion,
             int timeoutMs = 30000);
    void cancel();
    void submitHardwareKeyResponse(const QString &id, const QString &device, const QString &pin);

signals:
    void output(const QString &text);
    void standardOutput(const QByteArray &data);
    void busyChanged(bool busy);
    void hardwareKeyPrompt(const QJsonObject &prompt);
    void hardwareKeyStatus(const QString &message);

private:
    void finish(bool success);
    void drain();
    void consumeLine(const QByteArray &line);
    QProcess m_process;
    QTimer m_timeout;
    QByteArray m_stdout;
    QByteArray m_pendingLine;
    bool m_guiInteraction = false;
    QString m_promptId;
    QStringList m_promptDevices;
    QSet<QString> m_seenPromptIds;
    Completion m_completion;
    bool m_cancelled = false;
};
