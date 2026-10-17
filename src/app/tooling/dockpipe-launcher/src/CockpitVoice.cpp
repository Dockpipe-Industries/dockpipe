#include "CockpitVoice.h"
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QStandardPaths>

CockpitVoice::CockpitVoice(QObject *parent) : QObject(parent)
{
    m_deadline.setSingleShot(true);
    m_deadline.setInterval(15000);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        cancel();
        emit statusChanged(tr("Voice timed out. Microphone released."));
    });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, [this]() {
        const QByteArray chunk = m_process.readAllStandardOutput();
        m_outputBytes += chunk.size();
        if (m_outputBytes > 16384) {
            cancel();
            emit statusChanged(tr("Voice helper exceeded its response limit."));
            return;
        }
        m_output += chunk;
        while (m_output.contains('\n')) {
            const int end = m_output.indexOf('\n');
            const auto reply = QJsonDocument::fromJson(m_output.left(end)).object();
            m_output.remove(0, end + 1);
            if (m_cancelled)
                continue;
            if (reply.value("status") == "listening")
                emit statusChanged(tr("Listening locally… release to send · 8 second maximum"));
            if (reply.contains("error"))
                emit statusChanged(reply.value("error").toString().left(400));
            if (reply.contains("command"))
                m_command = reply.value("command").toString().left(80);
        }
    });
    connect(&m_process, &QProcess::readyReadStandardError, this, [this]() {
        m_outputBytes += m_process.readAllStandardError().size();
        if (m_outputBytes > 16384)
            cancel();
    });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            m_deadline.stop();
            emit statusChanged(tr("Local speech helper could not start."));
        }
    });
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int code, QProcess::ExitStatus status) {
            m_deadline.stop();
            if (m_cancelled || code != 0 || status != QProcess::NormalExit)
                return;
            if (m_command.isEmpty())
                emit statusChanged(tr("No command recognized. Hold to try again."));
            else {
                emit statusChanged(tr("Heard: %1").arg(m_command));
                emit commandRecognized(m_command);
            }
        });
}

CockpitVoice::~CockpitVoice()
{
    cancel();
    if (active())
        m_process.waitForFinished(1000);
}

void CockpitVoice::start()
{
    if (active())
        return;
#if !defined(Q_OS_LINUX)
    emit statusChanged(tr("Local speech currently requires native Linux and PocketSphinx."));
    return;
#endif
    if (QFile::exists("/.flatpak-info")) {
        emit statusChanged(tr("Local cockpit speech is not available in this Flatpak build."));
        return;
    }
    const QString python = QStandardPaths::findExecutable("python3");
    const QString script = m_directory.filePath("local_voice.py");
    if (python.isEmpty() || !m_directory.isValid()
        || (!QFile::exists(script) && !QFile::copy(":/cockpit/local_voice.py", script))) {
        emit statusChanged(tr("Local speech needs Python 3, libpocketsphinx3, pocketsphinx-en-us and libpulse0."));
        return;
    }
    m_cancelled = false;
    m_output.clear();
    m_command.clear();
    m_outputBytes = 0;
    m_process.start(python, {"-I", script});
    m_deadline.start();
    emit statusChanged(tr("Preparing local microphone…"));
}

void CockpitVoice::finish()
{
    if (active())
        m_process.write("stop\n");
}

void CockpitVoice::cancel()
{
    m_cancelled = true;
    m_deadline.stop();
    if (active())
        m_process.kill();
}
