#pragma once

#include "LauncherUiExport.h"
#include "RemoteCommand.h"
#include <QJsonObject>
#include <QTemporaryFile>
#include <memory>

// Shared asynchronous transport for conventional and optional project UIs.
// All Git semantics and mutations stay in the runtime's project Git service.
class DOCKPIPE_LAUNCHER_UI_EXPORT ProjectGitClient final : public QObject {
    Q_OBJECT
public:
    using Completion = std::function<void(bool, const QJsonObject &, const QString &)>;
    explicit ProjectGitClient(QObject *parent = nullptr);
    bool busy() const { return m_command.busy(); }
    void cancel() { m_command.cancel(); }
    void run(const QString &workdir, QJsonObject request, Completion completion);
signals:
    void busyChanged(bool busy);
private:
    RemoteCommand m_command;
    std::unique_ptr<QTemporaryFile> m_request;
    QString m_diagnostics;
};
