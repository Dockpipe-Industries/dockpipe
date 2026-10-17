#pragma once
#include <QObject>
#include <QJsonObject>
#include <QPointer>
#include <QTimer>

class QProcess;

// One cancellable helper process, one replaceable pending request. No worker threads
// are left blocked in a network filesystem inside the GUI process.
class WorkspaceGraphReader final : public QObject {
    Q_OBJECT
public:
    explicit WorkspaceGraphReader(QObject *parent = nullptr, int deadlineMilliseconds = 25000);
    ~WorkspaceGraphReader() override;
    void request(quint64 token, const QJsonObject &request);
    void cancel();
    bool busy() const;
    static bool helperActive();
signals:
    void completed(quint64 token, const QJsonObject &page);
private:
    void startPending();
    void drain();
    void finish(bool success);
    QPointer<QProcess> m_process;
    QTimer m_timeout;
    int m_deadlineMilliseconds;
    QJsonObject m_pending;
    QByteArray m_output;
    QString m_failure;
    quint64 m_token = 0;
    quint64 m_pendingToken = 0;
    bool m_abandoned = false;
    bool m_timedOut = false;
};
