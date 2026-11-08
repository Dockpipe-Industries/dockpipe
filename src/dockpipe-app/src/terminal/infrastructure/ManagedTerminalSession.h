#pragma once

#include "NativeTerminalLaunch.h"
#include <QByteArray>
#include <QHash>
#include <QObject>
#include <QVector>
#include <functional>

class QProcess;
class QTimer;

// Local UI transport only. Core owns admission and the trusted native provider;
// this class never starts a shell, returns permits, or journals terminal bytes.
class ManagedTerminalSession final : public QObject {
    Q_OBJECT
public:
    explicit ManagedTerminalSession(QObject *parent = nullptr);
    ~ManagedTerminalSession() override;
    bool start(const NativeTerminalLaunch::Snapshot &profile);
    bool send(const QByteArray &data);
    bool sendAcknowledged(const QByteArray &data, std::function<void(bool)> completion);
    void resize(int columns, int rows);
    void stop();
    bool running() const { return m_active && !m_closing && !m_exited; }
    bool active() const { return m_active; }
    bool ready() const { return m_ready && running(); }
    bool cleanupVerified() const { return m_cleanupVerified; }
    QString error() const { return m_error; }

signals:
    void output(const QByteArray &data, quint64 cursor, bool truncated);
    void stateChanged();
    void finished(int exitCode);

private:
    bool writeFrame(QJsonObject frame);
    void drain();
    void consume(const QByteArray &line);
    void flushInput();
    void flushResize();
    void completeInput(bool success, quint64 through);
    void fail(const QString &error);
    void finish(int code);
    void releaseProcess();
    QProcess *m_process = nullptr;
    QTimer *m_startDeadline;
    QTimer *m_closeDeadline;
    QByteArray m_lines;
    QByteArray m_pending;
    QByteArray m_inFlight;
    QVector<QPair<quint64, std::function<void(bool)>>> m_completions;
    QString m_error;
    QString m_session;
    quint64 m_nextId = 1;
    quint64 m_inputId = 0;
    quint64 m_resizeId = 0;
    quint64 m_acceptedBytes = 0;
    quint64 m_inFlightEnd = 0;
    quint64 m_cursor = 0;
    int m_columns = 80;
    int m_rows = 24;
    int m_sentColumns = 0;
    int m_sentRows = 0;
    int m_exitCode = 0;
    bool m_active = false;
    bool m_ready = false;
    bool m_closing = false;
    bool m_exited = false;
    bool m_cleanupVerified = false;
};
