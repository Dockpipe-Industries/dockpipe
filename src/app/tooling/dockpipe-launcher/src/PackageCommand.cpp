#include "PackageCommand.h"

#include <QJsonDocument>
#include <QJsonParseError>

PackageCommand::PackageCommand(QObject *parent) : QObject(parent)
{
    m_timeout.setSingleShot(true);
    m_timeout.setInterval(10 * 60 * 1000);
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &PackageCommand::readOutput);
    connect(&m_process, &QProcess::readyReadStandardError, this, &PackageCommand::readOutput);
    connect(&m_process, &QProcess::finished, this, &PackageCommand::finish);
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_active = false;
            m_timeout.stop();
            emit failed(tr("Could not start DockPipe: %1").arg(m_process.errorString()));
        }
    });
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        if (m_failure.isEmpty())
            m_failure = tr("Package operation timed out. Try refreshing the catalog.");
        m_process.kill();
    });
}

PackageCommand::~PackageCommand()
{
    if (m_process.state() != QProcess::NotRunning) {
        m_process.terminate();
        if (!m_process.waitForFinished(1500)) {
            m_process.kill();
            m_process.waitForFinished(1500);
        }
    }
}

bool PackageCommand::busy() const
{
    return m_active;
}

void PackageCommand::start(const QString &program, const QStringList &arguments, const QString &workdir)
{
    if (busy())
        return;
    m_output.clear();
    m_errors.clear();
    m_failure.clear();
    m_active = true;
    m_process.setWorkingDirectory(workdir);
    m_process.start(program, arguments);
    m_timeout.start(10 * 60 * 1000);
}

void PackageCommand::cancel()
{
    if (!busy())
        return;
    m_failure = tr("Package operation cancelled. Refresh to check the installed state.");
    m_process.terminate();
    // Bound shutdown if a child fails to respond; start() resets the timeout.
    m_timeout.start(2000);
}

void PackageCommand::readOutput()
{
    m_output += m_process.readAllStandardOutput();
    m_errors += m_process.readAllStandardError();
    if (m_output.size() > 8 * 1024 * 1024 || m_errors.size() > 1024 * 1024) {
        m_failure = tr("DockPipe returned too much package output.");
        m_process.kill();
    }
}

void PackageCommand::finish(int exitCode, QProcess::ExitStatus status)
{
    readOutput();
    m_active = false;
    m_timeout.stop();
    if (!m_failure.isEmpty()) {
        emit failed(m_failure);
        return;
    }
    if (status != QProcess::NormalExit || exitCode != 0) {
        emit failed(m_errors.isEmpty() ? tr("DockPipe package operation failed.") : QString::fromUtf8(m_errors).left(4000));
        return;
    }
    QJsonParseError error;
    const QJsonDocument document = QJsonDocument::fromJson(m_output, &error);
    if (error.error != QJsonParseError::NoError || !document.isObject()) {
        emit failed(tr("DockPipe returned an invalid package response. Update the CLI and launcher together."));
        return;
    }
    emit completed(document.object());
}
