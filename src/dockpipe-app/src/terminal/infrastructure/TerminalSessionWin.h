#pragma once

#include <QByteArray>
#include <QMap>
#include <QString>
#include <QStringList>
#include <memory>
#include <functional>

// Private ConPTY transport. Native creation, synchronous pipe I/O and teardown
// never execute on the GUI thread. No terminal bytes are persisted.
class TerminalSessionWin final {
public:
    struct Update {
        QByteArray output;
        QString error;
        bool started = false;
        quint64 written = 0;
        bool completed = false;
        int exitCode = -1;
    };

    TerminalSessionWin();
    ~TerminalSessionWin();
    bool start(const QString &directory, const QString &program, const QStringList &arguments,
        int columns, int rows, const QMap<QString, QString> &environment, QString &error);
    bool startPrepared(const QString &directory, const QString &program, const QStringList &arguments,
        int columns, int rows, const QStringList &environment, QString &error);
    // 0 means pending, 1 verified cleanup, -1 completed with uncertain cleanup.
    std::function<int()> cleanupProbe() const;
    bool send(const QByteArray &bytes);
    void resize(int columns, int rows);
    void stop();
    Update takeUpdate();
    bool running() const;

private:
    struct State;
    std::shared_ptr<State> m_state;
};
