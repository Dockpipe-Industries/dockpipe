#include "SessionManager.h"

#include "DockpipeChoices.h"

#include <QDateTime>
#include <QDir>
#include <QFile>
#include <QProcess>
#include <QProcessEnvironment>
#include <QTimer>

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
                emit sessionFailed(ctx.id, tr("Workflow output exceeded the consumer's line capacity."));
                stop(ctx.id);
                return false;
            }
            const QString line = ctx.maximumOutputLineBytes > 0
                ? QString::fromUtf8(rawLine) : QString::fromLocal8Bit(rawLine);
            if (ctx.maximumOutputLineBytes > 0 && line.toUtf8() != rawLine) {
                emit sessionFailed(ctx.id, tr("Workflow returned invalid UTF-8 output."));
                stop(ctx.id);
                return false;
            }
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
            emit sessionFailed(ctx.id, tr("Workflow output exceeded the consumer's line capacity."));
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
        // Capacity-bound consumers use the UTF-8 package event protocol even
        // when Windows' console locale uses a different legacy encoding.
        const QString text = ctx.maximumOutputLineBytes > 0
            ? QString::fromUtf8(visible) : QString::fromLocal8Bit(visible);
        SessionInfo &session = m_info[ctx.id];
        if (!session.ready && text.contains(readinessMarker())) {
            session.ready = true;
            emit sessionReady(ctx.id);
        }
        emit sessionOutput(ctx.id, text);
    };

    connect(proc, &QProcess::readyRead, this, [flushOutput]() { flushOutput(false); });

    auto *startupDeadline = new QTimer(proc);
    startupDeadline->setSingleShot(true);
    startupDeadline->setInterval(15000);

    // Each callback owns this attempt only. Signals may synchronously retry the
    // same context after teardown, so never remove a replacement process.
    auto complete = [this, ctx, proc, logFile, flushOutput, startupDeadline]
            (int exitCode, QProcess::ExitStatus status, const QString &startupError) {
        if (proc->property("sessionCompleted").toBool())
            return;
        proc->setProperty("sessionCompleted", true);
        startupDeadline->stop();
        flushOutput(true);
        logFile->close();
        if (m_processes.value(ctx.id) != proc) {
            proc->deleteLater();
            return;
        }
        m_processes.remove(ctx.id);
        m_pendingOutput.remove(ctx.id);
        SessionInfo &session = m_info[ctx.id];
        const bool cancelled = session.stopRequested;
        const bool startupFailed = !startupError.isEmpty() && !cancelled;
        session.status = cancelled || (!startupFailed && exitCode == 0 && status == QProcess::NormalExit)
            ? SessionStatus::Stopped : SessionStatus::Failed;
        session.pid = 0;
        session.ready = false;
        const QString runtimeError = cancelled ? QString() : proc->property("sessionRuntimeError").toString();
        const QString failure = startupFailed ? startupError : runtimeError;
        if (!failure.isEmpty())
            session.errorString = failure;
        proc->deleteLater();
        if (startupFailed)
            emit sessionFailed(ctx.id, startupError);
        else {
            if (!runtimeError.isEmpty())
                emit sessionFailed(ctx.id, runtimeError);
            // A failure handler may already have admitted a replacement. The
            // old attempt must not publish a stop against that new session.
            if (!m_processes.contains(ctx.id))
                emit sessionStopped(ctx.id, exitCode, status);
        }
    };

    connect(proc, &QProcess::started, this, [this, ctx, proc, startupDeadline]() {
        startupDeadline->stop();
        if (m_processes.value(ctx.id) != proc)
            return;
        m_info[ctx.id].pid = proc->processId();
        if (!proc->property("sessionStartupError").toString().isEmpty()) {
            proc->kill();
            return;
        }
        if (m_info.value(ctx.id).stopRequested) {
            // terminate() during QProcess::Starting may not signal the child.
            // Reapply cancellation once it has a process ID.
            proc->setProperty("sessionStopSent", false);
            stop(ctx.id);
            return;
        }
        m_info[ctx.id].status = SessionStatus::Running;
        emit sessionStarted(ctx.id);
    });
    connect(proc, QOverload<int, QProcess::ExitStatus>::of(&QProcess::finished), this,
            [proc, complete](int exitCode, QProcess::ExitStatus status) {
        complete(exitCode, status, proc->property("sessionStartupError").toString());
    });
    connect(proc, &QProcess::errorOccurred, this, [proc, complete](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart) {
            complete(-1, QProcess::CrashExit, proc->errorString());
        } else if (proc->state() == QProcess::NotRunning) {
            // finished() owns teardown and publication so failure observers can
            // safely retry without the old attempt removing their process.
            proc->setProperty("sessionRuntimeError", proc->errorString());
        }
    });
    connect(startupDeadline, &QTimer::timeout, this, [proc]() {
        proc->setProperty("sessionStartupError", tr("Workflow process did not start within 15 seconds."));
        proc->kill();
    });

    m_processes[ctx.id] = proc;
    startupDeadline->start();
    proc->start();
    return true;
}

void SessionManager::stop(const QString &contextId)
{
    QPointer<QProcess> proc = m_processes.value(contextId);
    if (!proc || proc->state() == QProcess::NotRunning || proc->property("sessionStopSent").toBool())
        return;
    m_info[contextId].stopRequested = true;
    proc->setProperty("sessionStopSent", true);
    const qint64 pid = proc->processId();
#ifdef Q_OS_WIN
    proc->terminate();
    QTimer::singleShot(8000, proc, [proc, pid]() {
        if (!proc || proc->state() == QProcess::NotRunning)
            return;
        wchar_t systemDirectory[MAX_PATH];
        const UINT length = GetSystemDirectoryW(systemDirectory, MAX_PATH);
        if (length > 0 && length < MAX_PATH && pid > 0) {
            auto *treeKiller = new QProcess(proc);
            treeKiller->setProgram(QDir::toNativeSeparators(
                QString::fromWCharArray(systemDirectory, int(length)) + QStringLiteral("/taskkill.exe")));
            treeKiller->setArguments({QStringLiteral("/PID"), QString::number(pid),
                                     QStringLiteral("/T"), QStringLiteral("/F")});
            connect(treeKiller, &QProcess::finished, treeKiller, &QObject::deleteLater);
            treeKiller->start();
            QTimer::singleShot(4000, treeKiller, [treeKiller]() { treeKiller->kill(); });
        }
        QTimer::singleShot(5000, proc, [proc]() {
            if (proc && proc->state() != QProcess::NotRunning)
                proc->kill();
        });
    });
#else
    if (pid > 0) {
        // Give workflow scripts their existing 12-second graceful teardown,
        // without holding the GUI event loop while they clean up containers.
        if (::kill(static_cast<pid_t>(-pid), SIGTERM) != 0)
            proc->terminate();
    } else {
        proc->terminate();
    }
    QTimer::singleShot(12000, proc, [proc, pid]() {
        if (!proc || proc->state() == QProcess::NotRunning)
            return;
        if (pid > 0)
            ::kill(static_cast<pid_t>(-pid), SIGKILL);
        proc->kill();
    });
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
    // QProcess drains its write buffer asynchronously; prompt submission must
    // not wait for a child that has stopped reading stdin.
}
