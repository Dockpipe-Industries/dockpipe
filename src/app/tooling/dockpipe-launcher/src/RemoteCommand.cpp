#include "RemoteCommand.h"

RemoteCommand::RemoteCommand(QObject *parent) : QObject(parent)
{
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        emit output(tr("Operation timed out. Existing remote state is preserved.\n"));
        cancel();
    });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &RemoteCommand::drain);
    connect(&m_process, &QProcess::readyReadStandardError, this, [this]() {
        emit output(QString::fromUtf8(m_process.readAllStandardError()));
    });
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
            [this](int code, QProcess::ExitStatus status) {
        drain();
        finish(!m_cancelled && code == 0 && status == QProcess::NormalExit);
    });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        emit output(m_process.errorString() + QLatin1Char('\n'));
        if (error == QProcess::FailedToStart)
            finish(false);
    });
}

RemoteCommand::~RemoteCommand()
{
    m_completion = {};
    m_process.disconnect(this);
    if (m_process.state() != QProcess::NotRunning) {
        m_process.kill();
        m_process.waitForFinished(1000);
    }
}

bool RemoteCommand::busy() const
{
    return bool(m_completion);
}

void RemoteCommand::run(const QString &program, const QStringList &arguments,
                        Completion completion, int timeoutMs)
{
    if (busy())
        return;
    m_stdout.clear();
    m_cancelled = false;
    m_completion = std::move(completion);
    emit busyChanged(true);
    m_process.start(program, arguments);
    m_timeout.start(timeoutMs);
}

void RemoteCommand::drain()
{
    const QByteArray chunk = m_process.readAllStandardOutput();
    if (m_stdout.size() + chunk.size() > 20 * 1024 * 1024) {
        emit output(tr("Output exceeded the display limit.\n"));
        cancel();
        return;
    }
    m_stdout += chunk;
    emit standardOutput(chunk);
    emit output(QString::fromUtf8(chunk));
}

void RemoteCommand::cancel()
{
    if (!busy())
        return;
    m_cancelled = true;
    m_process.terminate();
    QTimer::singleShot(2000, &m_process, [this]() {
        if (m_cancelled && m_process.state() != QProcess::NotRunning)
            m_process.kill();
    });
}

void RemoteCommand::finish(bool success)
{
    m_timeout.stop();
    auto completion = std::move(m_completion);
    m_completion = {};
    const QByteArray output = m_stdout;
    emit busyChanged(false);
    if (completion)
        completion(success, output);
}
