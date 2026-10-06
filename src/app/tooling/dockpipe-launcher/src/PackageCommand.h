#pragma once

#include <QJsonObject>
#include <QObject>
#include <QProcess>
#include <QTimer>

// Asynchronous adapter for the CLI package contract. It never downloads or installs itself.
class PackageCommand : public QObject {
    Q_OBJECT
public:
    explicit PackageCommand(QObject *parent = nullptr);
    ~PackageCommand() override;
    bool busy() const;
    void start(const QString &program, const QStringList &arguments, const QString &workdir);
    void cancel();

signals:
    void completed(const QJsonObject &result);
    void failed(const QString &message);

private:
    void readOutput();
    void finish(int exitCode, QProcess::ExitStatus status);
    QProcess m_process;
    QTimer m_timeout;
    QByteArray m_output;
    QByteArray m_errors;
    QString m_failure;
    bool m_active = false;
};
