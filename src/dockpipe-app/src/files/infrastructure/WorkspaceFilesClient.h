#pragma once

#include "LauncherUiExport.h"

#include <QJsonObject>
#include <QObject>
#include <QPointer>
#include <functional>

class QProcess;
class QTimer;

// Private Files transport. The core owns path, revision and filesystem policy.
class DOCKPIPE_LAUNCHER_UI_EXPORT WorkspaceFilesClient final : public QObject {
    Q_OBJECT
public:
    struct Context {
        enum class Kind { Local, Remote };
        Kind kind = Kind::Local;
        QString localRoot;
        QString nodeId;
        QString fingerprint;
        QString workerGeneration;
        QString rootId;
        quint64 generation = 0;
        bool writeGranted = false;
        bool canWrite() const { return kind == Kind::Local || writeGranted; }
        bool operator==(const Context &other) const;
    };
    enum class Outcome { Success, Cancelled, Failure, Busy };
    struct Result {
        Outcome outcome = Outcome::Failure;
        QJsonObject value;
        QString error;
        Context context;
        bool success() const { return outcome == Outcome::Success; }
    };
    using Completion = std::function<void(const Result &)>;
    // Remote composition must bind this exact context to an authenticated worker.
    // The returned function cancels the transport; an absent dispatcher fails closed.
    using Cancel = std::function<void()>;
    using RemoteDispatch = std::function<Cancel(const Context &, const QJsonObject &, Completion)>;

    explicit WorkspaceFilesClient(QObject *parent = nullptr);
    ~WorkspaceFilesClient() override;
    void setContext(const Context &context);
    Context context() const { return m_context; }
    void setRemoteDispatch(RemoteDispatch dispatch);
    bool busy() const { return m_active; }
    void run(const QJsonObject &request, QObject *owner, Completion completion, int timeoutMs = 30000);
    void cancel();

signals:
    void busyChanged(bool busy);

private:
    void drain();
    void stop(Outcome outcome, const QString &error);
    void finish(Result result);
    void releaseProcess();
    Context m_context;
    Context m_attemptContext;
    QProcess *m_process = nullptr;
    QTimer *m_deadline;
    QPointer<QObject> m_owner;
    QMetaObject::Connection m_ownerDestroyed;
    Completion m_completion;
    RemoteDispatch m_remoteDispatch;
    Cancel m_remoteCancel;
    QByteArray m_stdout;
    QByteArray m_stderr;
    QString m_stopError;
    Outcome m_stopOutcome = Outcome::Failure;
    quint64 m_attempt = 0;
    bool m_active = false;
    bool m_stopped = false;
};
