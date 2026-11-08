#pragma once

#include <QByteArray>
#include <QJsonObject>
#include <QObject>
#include <QVector>
#include <functional>
#include <memory>

class QProcess;
class QTemporaryFile;
class QTimer;

// Ephemeral typed terminal relay. Input is bounded memory, never diagnostics or
// workflow logs. One acknowledged input sequence is in flight at a time.
class RemoteTerminalSession final : public QObject {
    Q_OBJECT
public:
    explicit RemoteTerminalSession(QObject *parent = nullptr);
    ~RemoteTerminalSession() override;
    bool start(const QJsonObject &catalog, const QString &root);
    bool send(const QByteArray &data);
    bool sendAcknowledged(const QByteArray &data, std::function<void(bool)> completion);
    void resize(int columns, int rows);
    void stop();
    bool running() const { return m_active && !m_closing && !m_nativeClosed; }
    bool active() const { return m_active; }
    bool ready() const { return m_started && running(); }
    QString error() const { return m_error; }

signals:
    void output(const QByteArray &data, quint64 cursor, bool truncated);
    void stateChanged();
    void finished(int exitCode);

private:
    void drain();
    void consume(const QByteArray &line);
    void acknowledge(quint64 sequence);
    void flushInput();
    void completeInput(bool success, quint64 through);
    bool sendFrame(const QJsonObject &frame);
    bool matchesReference(const QJsonObject &reference) const;
    void fail(const QString &error);
    void finish(int code);
    void releaseProcess();
    QProcess *m_process = nullptr;
    QTimer *m_openDeadline;
    QTimer *m_closeDeadline;
    std::unique_ptr<QTemporaryFile> m_request;
    QJsonObject m_intent;
    QJsonObject m_reference;
    QByteArray m_lines;
    QByteArray m_pending;
    QByteArray m_inFlight;
    quint64 m_sequence = 0;
    quint64 m_acceptedBytes = 0;
    quint64 m_inFlightEnd = 0;
    QVector<QPair<quint64, std::function<void(bool)>>> m_inputCompletions;
    quint64 m_acknowledged = 0;
    quint64 m_cursor = 0;
    QString m_error;
    int m_columns = 80;
    int m_rows = 24;
    bool m_active = false;
    bool m_started = false;
    bool m_nativeClosed = false;
    bool m_closing = false;
};
