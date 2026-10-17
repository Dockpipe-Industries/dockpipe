#pragma once
#include <QObject>
#include <QProcess>
#include <QTemporaryDir>
#include <QTimer>

class CockpitVoice final : public QObject {
    Q_OBJECT
public:
    explicit CockpitVoice(QObject *parent = nullptr);
    ~CockpitVoice() override;
    void start();
    void finish();
    void cancel();
    bool active() const { return m_process.state() != QProcess::NotRunning; }
signals:
    void statusChanged(const QString &status);
    void commandRecognized(const QString &command);
private:
    QTemporaryDir m_directory;
    QProcess m_process;
    QTimer m_deadline;
    QByteArray m_output;
    QString m_command;
    bool m_cancelled = false;
    int m_outputBytes = 0;
};
