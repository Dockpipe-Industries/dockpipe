#include "SessionManager.h"

#include "DockpipeChoices.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QProcess>
#include <QProcessEnvironment>

#include <functional>

#ifdef Q_OS_WIN
#include <qt_windows.h>
#else
#include <errno.h>
#include <signal.h>
#include <sys/types.h>
#include <unistd.h>
#endif

SessionManager::SessionManager(QObject *parent) : QObject(parent) {}

namespace {
QString promptEventPrefix()
{
    return QStringLiteral("::dockpipe-prompt::");
}
}

SessionInfo SessionManager::info(const QString &contextId) const
{
    return m_info.value(contextId);
}

bool SessionManager::isReady(const QString &contextId) const
{
    return m_info.value(contextId).ready;
}

bool SessionManager::isRunning(const QString &contextId) const
{
    const QPointer<QProcess> p = m_processes.value(contextId);
    return p && p->state() != QProcess::NotRunning;
}

QString SessionManager::readinessMarker()
{
    return QStringLiteral("[dockpipe-ready]");
}

QStringList SessionManager::dockpipeArguments(const Context &ctx)
{
    QStringList args;
    if (!ctx.workflowFile.isEmpty()) {
        args << QStringLiteral("--workflow-file") << ctx.workflowFile;
    } else if (!ctx.workflow.isEmpty()) {
        args << QStringLiteral("--workflow") << ctx.workflow;
    } else {
        args << QStringLiteral("--workflow") << QStringLiteral("vscode");
    }
    args << QStringLiteral("--workdir") << QDir::toNativeSeparators(ctx.workdir);
    if (!ctx.resolver.isEmpty())
        args << QStringLiteral("--resolver") << ctx.resolver;
    if (!ctx.runtime.isEmpty())
        args << QStringLiteral("--runtime") << ctx.runtime;
    if (!ctx.strategy.isEmpty())
        args << QStringLiteral("--strategy") << ctx.strategy;
    if (!ctx.envFile.isEmpty())
        args << QStringLiteral("--env-file") << QDir::toNativeSeparators(ctx.envFile);
    for (const QString &line : ctx.extraDockpipeEnv) {
        const QString t = line.trimmed();
        if (t.isEmpty())
            continue;
        const QString key = t.section(QLatin1Char('='), 0, 0);
        args << QStringLiteral("--env") << (key.startsWith("DOCKPIPE_VSCODE_INPUT_") ? line : t);
    }
    return args;
}

bool SessionManager::launch(const Context &ctx, const QString &logsDir)
{
    if (ctx.workdir.isEmpty())
        return false;
    if (m_processes.contains(ctx.id) && m_processes[ctx.id] && m_processes[ctx.id]->state() != QProcess::NotRunning)
        return false;

    QString program = ctx.dockpipeBinary.trimmed();
    if (program.isEmpty())
        program = DockpipeChoices::preferredDockpipeBinary(ctx.workdir);

    const QString ts = QDateTime::currentDateTime().toString(QStringLiteral("yyyyMMdd-HHmmss"));
    const QString logPath = logsDir + QLatin1Char('/') + ctx.id + QLatin1Char('-') + ts + QStringLiteral(".log");

    auto *proc = new QProcess(this);
    proc->setProgram(program);
    proc->setArguments(dockpipeArguments(ctx));
    proc->setProcessChannelMode(QProcess::MergedChannels);
#ifndef Q_OS_WIN
    // stop() sends SIGTERM to -pid, so give each dockpipe launch its own process group.
    proc->setChildProcessModifier([]() {
        if (::setpgid(0, 0) != 0 && errno != EACCES) {
            _exit(127);
        }
    });
#endif

    // Force project workdir for dockpipe and host scripts. Inherited DOCKPIPE_* from the desktop/shell
    // can be wrong, so set DOCKPIPE_WORKDIR explicitly to match the chosen context.
    const QString wd = QDir::cleanPath(ctx.workdir);
    QProcessEnvironment env = QProcessEnvironment::systemEnvironment();
    env.insert(QStringLiteral("DOCKPIPE_WORKDIR"), wd);
    env.insert(QStringLiteral("DOCKPIPE_SDK_PROMPT_MODE"), QStringLiteral("json"));
    for (auto input = ctx.transientLaunchEnv.cbegin(); input != ctx.transientLaunchEnv.cend(); ++input)
        env.insert(input.key(), input.value());
    proc->setProcessEnvironment(env);
    proc->setWorkingDirectory(wd);

    auto *logFile = new QFile(logPath, proc);
    if (!logFile->open(QIODevice::WriteOnly | QIODevice::Append)) {
        delete proc;
        emit sessionFailed(ctx.id, QStringLiteral("Cannot open log file: ") + logPath);
        return false;
    }

    SessionInfo si;
    si.contextId = ctx.id;
    si.status = SessionStatus::Starting;
    si.logPath = logPath;
    si.program = program;
    si.arguments = proc->arguments();
    si.ready = false;
    m_info[ctx.id] = si;

    std::function<void(bool)> flushOutput = [this, ctx, proc, logFile](bool drainRemainder) {
        // Output signals can synchronously cancel and finish this process.
        // Keep parsing storage independent of hash entries removed by teardown.
        QByteArray pending = m_pendingOutput.take(ctx.id);
        pending += proc->readAll();
        if (m_info.value(ctx.id).stopRequested)
            return;

        QByteArray visible;
        auto processLine = [this, ctx, &visible](const QByteArray &rawLine) {
            if (ctx.maximumOutputLineBytes > 0 && rawLine.size() > ctx.maximumOutputLineBytes) {
                emit sessionFailed(ctx.id, QStringLiteral("Workflow output exceeded the consumer's line capacity."));
                stop(ctx.id);
                return false;
            }
            const QString line = QString::fromLocal8Bit(rawLine);
            if (line.startsWith(promptEventPrefix())) {
                QString payload = line.mid(promptEventPrefix().size());
                while (payload.endsWith(QLatin1Char('\n')) || payload.endsWith(QLatin1Char('\r')))
                    payload.chop(1);
                emit sessionPrompt(ctx.id, payload);
                return !m_info.value(ctx.id).stopRequested;
            }
            visible += rawLine;
            return true;
        };

        while (true) {
            const int newline = pending.indexOf('\n');
            if (newline < 0)
                break;
            const QByteArray line = pending.left(newline + 1);
            pending.remove(0, newline + 1);
            if (!processLine(line))
                return;
        }

        if (ctx.maximumOutputLineBytes > 0 && pending.size() > ctx.maximumOutputLineBytes) {
            pending.clear();
            emit sessionFailed(ctx.id, QStringLiteral("Workflow output exceeded the consumer's line capacity."));
            stop(ctx.id);
            return;
        }

        if (drainRemainder && !pending.isEmpty()) {
            if (!processLine(pending))
                return;
            pending.clear();
        }
        if (!pending.isEmpty())
            m_pendingOutput.insert(ctx.id, pending);

        if (visible.isEmpty())
            return;
        logFile->write(visible);
        logFile->flush();
        const QString text = QString::fromLocal8Bit(visible);
        SessionInfo &session = m_info[ctx.id];
        if (!session.ready && text.contains(readinessMarker())) {
            session.ready = true;
            emit sessionReady(ctx.id);
        }
        emit sessionOutput(ctx.id, text);
    };

    connect(proc, &QProcess::readyRead, this, [flushOutput]() { flushOutput(false); });

    connect(proc, QOverload<int, QProcess::ExitStatus>::of(&QProcess::finished), this,
            [this, ctx, proc, logFile, flushOutput](int exitCode, QProcess::ExitStatus st) {
                flushOutput(true);
                logFile->close();
                logFile->deleteLater();
                m_processes.remove(ctx.id);
                m_pendingOutput.remove(ctx.id);
                SessionInfo &s = m_info[ctx.id];
                if (s.stopRequested || (exitCode == 0 && st == QProcess::NormalExit))
                    s.status = SessionStatus::Stopped;
                else
                    s.status = SessionStatus::Failed;
                s.pid = 0;
                s.ready = false;
                emit sessionStopped(ctx.id, exitCode, st);
                proc->deleteLater();
            });

    connect(proc, &QProcess::errorOccurred, this, [this, ctx, proc, logFile, flushOutput](QProcess::ProcessError error) {
        // launch() owns startup failure reporting after waitForStarted returns.
        // Handling it here too would report and tear down the same attempt twice.
        if (error == QProcess::FailedToStart)
            return;
        flushOutput(true);
        if (proc->state() == QProcess::NotRunning) {
            logFile->close();
            logFile->deleteLater();
            m_processes.remove(ctx.id);
            m_pendingOutput.remove(ctx.id);
            SessionInfo &s = m_info[ctx.id];
            s.status = SessionStatus::Failed;
            s.errorString = proc->errorString();
            s.ready = false;
            if (!s.stopRequested)
                emit sessionFailed(ctx.id, proc->errorString());
        }
    });

    m_processes[ctx.id] = proc;
    proc->start();
    if (!proc->waitForStarted(15000)) {
        const QString error = proc->errorString();
        logFile->close();
        logFile->deleteLater();
        proc->deleteLater();
        m_processes.remove(ctx.id);
        m_pendingOutput.remove(ctx.id);
        SessionInfo &session = m_info[ctx.id];
        session.status = SessionStatus::Failed;
        session.errorString = error;
        session.pid = 0;
        session.ready = false;
        // Publish only after teardown so a failure handler can retry this context.
        emit sessionFailed(ctx.id, error);
        return false;
    }

    m_info[ctx.id].status = SessionStatus::Running;
    m_info[ctx.id].pid = proc->processId();
    m_info[ctx.id].ready = false;
    m_processes[ctx.id] = proc;
    emit sessionStarted(ctx.id);
    return true;
}

void SessionManager::stop(const QString &contextId)
{
    QPointer<QProcess> proc = m_processes.value(contextId);
    if (!proc || proc->state() == QProcess::NotRunning || m_info.value(contextId).stopRequested)
        return;
    // waitForFinished dispatches output/error/finished signals. A consumer can
    // call stop again from those signals; only the first call owns teardown.
    m_info[contextId].stopRequested = true;
    const qint64 pid = proc->processId();
#ifdef Q_OS_WIN
    proc->terminate();
    if (!proc->waitForFinished(8000)) {
        wchar_t systemDirectory[MAX_PATH];
        const UINT length = GetSystemDirectoryW(systemDirectory, MAX_PATH);
        if (length > 0 && length < MAX_PATH) {
            QProcess treeKiller;
            treeKiller.setProgram(QDir::toNativeSeparators(
                QString::fromWCharArray(systemDirectory, int(length)) + QStringLiteral("/taskkill.exe")));
            treeKiller.setArguments({QStringLiteral("/PID"), QString::number(pid),
                                    QStringLiteral("/T"), QStringLiteral("/F")});
            treeKiller.start();
            if (!treeKiller.waitForStarted(1000) || !treeKiller.waitForFinished(3000)) {
                treeKiller.kill();
                treeKiller.waitForFinished(1000);
            }
        }
        // Reap the QProcess we own even if tree termination fails or its exit
        // notification is still queued. taskkill's exit is not the child's exit.
        if (proc && proc->state() != QProcess::NotRunning) {
            proc->kill();
            proc->waitForFinished(3000);
        }
    }
#else
    if (pid > 0) {
        // Prefer process-group signal so bash child scripts can tear down docker.
        if (::kill(static_cast<pid_t>(-pid), SIGTERM) != 0)
            proc->terminate();
    } else {
        proc->terminate();
    }
    if (!proc->waitForFinished(12000)) {
        proc->kill();
        proc->waitForFinished(3000);
    }
#endif
}

void SessionManager::sendInput(const QString &contextId, const QString &text)
{
    QPointer<QProcess> proc = m_processes.value(contextId);
    if (!proc || proc->state() == QProcess::NotRunning)
        return;
    QByteArray payload = text.toUtf8();
    payload.append('\n');
    proc->write(payload);
    proc->waitForBytesWritten(3000);
}
