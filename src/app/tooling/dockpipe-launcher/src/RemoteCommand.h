#pragma once

#include <QObject>
#include <QProcess>
#include <QTimer>
#include <functional>

// All remote UI actions use the CLI and its private credential store.
class RemoteCommand : public QObject {
    Q_OBJECT
public:
    using Completion = std::function<void(bool, const QByteArray &)>;
    explicit RemoteCommand(QObject *parent = nullptr);
    ~RemoteCommand() override;
    bool busy() const;
    void run(const QString &program, const QStringList &arguments, Completion completion,
             int timeoutMs = 30000);
    void cancel();

signals:
    void output(const QString &text);
    void standardOutput(const QByteArray &data);
    void busyChanged(bool busy);

private:
    void finish(bool success);
    void drain();
    QProcess m_process;
    QTimer m_timeout;
    QByteArray m_stdout;
    Completion m_completion;
    bool m_cancelled = false;
};
