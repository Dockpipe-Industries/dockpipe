#pragma once

#include <QCoreApplication>
#include <QJsonObject>
#include <QJsonArray>
#include <QJsonValue>
#include <QObject>
#include <QPointer>
#include <functional>

class QProcess;
class QTimer;

// Private typed CLI transport. Credentials and OS-session authority stay in core.
// Each call owns its captured intent; cancellation never follows a later selection.
class RemoteFeatureClient final : public QObject {
    Q_DECLARE_TR_FUNCTIONS(RemoteFeatureClient)
public:
    enum class Outcome { Success, Failure, Cancelled, Unknown };
    struct Result {
        Outcome outcome = Outcome::Failure;
        QJsonValue value;
        QString error;
        QJsonObject reference;
    };
    using Completion = std::function<void(const Result &)>;
    using Cancel = std::function<void()>;

    static QJsonObject prepareIntent(const QJsonObject &target, const QString &root,
                                     const QString &action, const QJsonObject &payload);
    static Cancel catalog(QObject *owner, Completion completion, int timeoutMs = 30000);
    static Cancel invoke(QObject *owner, const QJsonObject &intent, Completion completion,
                         int timeoutMs = 30000);
    static Cancel inspect(QObject *owner, const QJsonObject &reference, Completion completion,
                          int timeoutMs = 30000);
    static Cancel cancelOperation(QObject *owner, const QJsonObject &reference, Completion completion,
                                  int timeoutMs = 30000);
    static Cancel reconcile(QObject *owner, const QJsonObject &reference, Completion completion,
                            int timeoutMs = 30000);
    static Cancel retireContainer(QObject *owner, const QJsonObject &reference, Completion completion,
                                  int timeoutMs = 30000);
    // Only bounded mutation identity/status metadata is persisted, never intent content.
    static QJsonArray pendingRecoveries();
    // Domain adapters restore the receipt if a transport success has an invalid payload.
    static void retainUncertain(const QJsonObject &reference);
    ~RemoteFeatureClient() override;

private:
    RemoteFeatureClient(QObject *owner, QJsonObject intent, bool catalog, Completion completion);
    static Cancel start(QObject *owner, const QJsonObject &intent, bool catalog,
                        Completion completion, int timeoutMs);
    static Cancel recover(QObject *owner, const QJsonObject &reference, const QString &command,
                          Completion completion, int timeoutMs);
    void launch(int timeoutMs);
    void launchProcess(const QString &command, const QJsonObject &input);
    void drain();
    void cancel();
    void stop(Outcome outcome, const QString &error);
    void completed(int exitCode, bool normalExit);
    void finish(Result result);
    void releaseProcess();
    QPointer<QObject> m_owner;
    QJsonObject m_intent;
    Completion m_completion;
    QProcess *m_process = nullptr;
    QTimer *m_deadline;
    QByteArray m_stdout;
    QByteArray m_stderr;
    QString m_stopError;
    Outcome m_stopOutcome = Outcome::Failure;
    bool m_catalog = false;
    bool m_stopping = false;
    bool m_finished = false;
};
