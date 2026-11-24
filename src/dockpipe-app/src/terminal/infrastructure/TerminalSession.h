#pragma once

#include "TerminalLaunchContext.h"
#include "NativeTerminalLaunch.h"

#include <QByteArray>
#include <QObject>
#include <QMap>
#include <QString>
#include <QStringList>
#include <memory>

// Session-only local PTY process (shell or host bridge). Dockpipe does not persist terminal bytes;
// the selected shell retains its ordinary startup and history behavior.
class TerminalSession final : public QObject {
    Q_OBJECT
public:
    explicit TerminalSession(QObject *parent = nullptr);
    explicit TerminalSession(TerminalLaunchContext context, QObject *parent = nullptr);
    ~TerminalSession() override;
    bool start(QString workdir, const QString &program = {}, const QStringList &arguments = {},
        const QMap<QString, QString> &environment = {});
    bool startPrepared(const NativeTerminalLaunch::Snapshot &launch);
    bool send(QByteArray data);
    quint64 inputWrittenCursor() const;
    void stopAndWait();
    void resize(int columns, int rows);
    void stop();
    bool running() const;
    QString error() const;
signals:
    void started();
    void inputWritten(quint64 cursor);
    void cleanupFinished(bool verified);
    void output(QByteArray data);
    void stateChanged();
    void finished(int exitCode);
private:
    struct State;
    std::unique_ptr<State> m_state;
    void readOutput();
    void writeInput();
    void closeDescriptors();
    void stopSession(bool immediate);
    void setError(const QString &message);
};
