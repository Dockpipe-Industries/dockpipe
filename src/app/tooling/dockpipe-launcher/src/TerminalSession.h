#pragma once

#include <QByteArray>
#include <QObject>
#include <QString>
#include <memory>

// Session-only local interactive shell. DockPipe does not persist terminal bytes;
// the selected shell retains its ordinary startup and history behavior.
class TerminalSession final : public QObject {
    Q_OBJECT
public:
    explicit TerminalSession(QObject *parent = nullptr);
    ~TerminalSession() override;
    bool start(QString workdir);
    bool send(QByteArray data);
    void resize(int columns, int rows);
    void stop();
    bool running() const;
    QString error() const;
signals:
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
