#include "TerminalSession.h"

#include <QCoreApplication>
#include <QDir>
#include <QDirIterator>
#include <QElapsedTimer>
#include <QVector>
#include <QFileInfo>
#include <QPointer>
#include <QProcess>
#include <QProcessEnvironment>
#include <QSocketNotifier>
#include <QSet>
#include <QTimer>

#ifdef Q_OS_UNIX
#include <cerrno>
#include <cstdlib>
#include <fcntl.h>
#include <signal.h>
#include <sys/ioctl.h>
#include <termios.h>
#include <unistd.h>
#ifdef Q_OS_MACOS
#include <libproc.h>
#endif
#endif

namespace {
constexpr qsizetype InputLimit = 64 * 1024;
constexpr qsizetype ReadLimit = 64 * 1024;
#ifdef Q_OS_UNIX
struct TerminalGroupMember {
    pid_t process;
    pid_t group;
};

QVector<TerminalGroupMember> terminalGroups(pid_t session, bool &complete)
{
    QVector<TerminalGroupMember> members;
    complete = true;
    if (session <= 1)
        return members;
    // Cleanup is bounded even on a host with many unrelated processes. Only
    // identifiers/session membership are inspected, never arguments or content.
    constexpr int maximumHostProcesses = 16384;
    constexpr int maximumSessionMembers = 1024;
    QElapsedTimer elapsed;
    elapsed.start();
    const auto add = [&](pid_t process) {
        if (process > 1 && ::getsid(process) == session) {
            const pid_t group = ::getpgid(process);
            if (group > 1 && group != ::getpgrp())
                members.append({process, group});
        }
    };
#ifdef Q_OS_LINUX
    QDirIterator processes(QStringLiteral("/proc"), QDir::Dirs | QDir::NoDotAndDotDot);
    int examined = 0;
    while (processes.hasNext()) {
        if (++examined > maximumHostProcesses || members.size() >= maximumSessionMembers || elapsed.elapsed() >= 50) {
            complete = false;
            break;
        }
        processes.next();
        bool valid = false;
        const auto process = processes.fileName().toLongLong(&valid);
        if (valid)
            add(pid_t(process));
    }
#elif defined(Q_OS_MACOS)
    QVector<pid_t> processes(maximumHostProcesses);
    const int bytes = ::proc_listpids(PROC_ALL_PIDS, 0, processes.data(), int(processes.size() * sizeof(pid_t)));
    if (bytes <= 0 || bytes >= int(processes.size() * sizeof(pid_t)))
        complete = false;
    const int count = qBound(0, bytes / int(sizeof(pid_t)), maximumHostProcesses);
    for (int index = 0; index < count; ++index) {
        if (members.size() >= maximumSessionMembers || elapsed.elapsed() >= 50) {
            complete = false;
            break;
        }
        add(processes[index]);
    }
#else
    complete = false;
#endif
    return members;
}

void signalTerminalGroups(const QVector<TerminalGroupMember> &members, pid_t session, int signal)
{
    QSet<pid_t> signalled;
    for (const auto &member : members) {
        // Keep an actual member identity, including groups whose leader exited.
        // A process from an unrelated/reused session must never be signalled.
        if (!signalled.contains(member.group) && member.group != ::getpgrp()
            && ::getsid(member.process) == session && ::getpgid(member.process) == member.group) {
            ::kill(-member.group, signal);
            signalled.insert(member.group);
        }
    }
}

void failTerminalChild(QProcess *process, const char *message, int error)
{
#if QT_VERSION >= QT_VERSION_CHECK(6, 7, 0)
    process->failChildProcessModifier(message, error);
#else
    Q_UNUSED(process);
    Q_UNUSED(message);
    Q_UNUSED(error);
    ::_exit(126);
#endif
}
#endif
}

struct TerminalSession::State {
    QProcess *process = nullptr;
    QSocketNotifier *reader = nullptr;
    QSocketNotifier *writer = nullptr;
    QByteArray pendingInput;
    QString error;
    int master = -1;
    int slave = -1;
    int columns = 80;
    int rows = 24;
    quint64 generation = 0;
    qint64 shellSession = -1;
};

TerminalSession::TerminalSession(QObject *parent)
    : QObject(parent), m_state(std::make_unique<State>())
{
    connect(QCoreApplication::instance(), &QCoreApplication::aboutToQuit, this, [this] { stopSession(true); });
}

TerminalSession::~TerminalSession()
{
    disconnect(this, nullptr, nullptr, nullptr);
    stopSession(true);
}

bool TerminalSession::running() const
{
    return m_state->process && m_state->process->state() != QProcess::NotRunning;
}

QString TerminalSession::error() const
{
    return m_state->error;
}

void TerminalSession::setError(const QString &message)
{
    m_state->error = message;
    emit stateChanged();
}

bool TerminalSession::start(QString workdir)
{
    if (running()) {
        setError(tr("This terminal is already running."));
        return false;
    }
    stop();
    m_state->error.clear();
#ifndef Q_OS_UNIX
    Q_UNUSED(workdir);
    setError(tr("Interactive terminals are unavailable in this build on Windows. A ConPTY backend is required."));
    return false;
#else
    if (workdir.isEmpty())
        workdir = QDir::homePath();
    if (!QFileInfo(workdir).isDir()) {
        setError(tr("The terminal working directory does not exist."));
        return false;
    }
    QString shell = qEnvironmentVariable("SHELL");
    if (!QFileInfo(shell).isAbsolute() || !QFileInfo(shell).isFile() || !QFileInfo(shell).isExecutable())
        shell = QStringLiteral("/bin/sh");
#ifdef Q_OS_MACOS
    m_state->master = ::posix_openpt(O_RDWR | O_NOCTTY);
    if (m_state->master >= 0 && (::fcntl(m_state->master, F_SETFD, FD_CLOEXEC) < 0
        || ::fcntl(m_state->master, F_SETFL, O_NONBLOCK) < 0)) {
        closeDescriptors();
        setError(tr("Could not configure the interactive terminal device."));
        return false;
    }
#else
    m_state->master = ::posix_openpt(O_RDWR | O_NOCTTY | O_CLOEXEC | O_NONBLOCK);
#endif
    if (m_state->master < 0 || ::grantpt(m_state->master) != 0 || ::unlockpt(m_state->master) != 0) {
        closeDescriptors();
        setError(tr("Could not allocate an interactive terminal."));
        return false;
    }
    // ptsname has process-local storage; all session creation is on the owning
    // Qt thread, and the slave is opened before any asynchronous work begins.
    const char *slaveName = ::ptsname(m_state->master);
    if (slaveName)
        m_state->slave = ::open(slaveName, O_RDWR | O_NOCTTY | O_CLOEXEC);
    if (m_state->slave < 0) {
        closeDescriptors();
        setError(tr("Could not open the interactive terminal device."));
        return false;
    }
    // Reserve our descriptors away from stdio and QProcess's later private
    // pipes, including apps launched with one or more standard descriptors shut.
    for (int *descriptor : {&m_state->master, &m_state->slave}) {
        const int reserved = ::fcntl(*descriptor, F_DUPFD_CLOEXEC, 64);
        if (reserved < 0) {
            closeDescriptors();
            setError(tr("Could not reserve the interactive terminal descriptors."));
            return false;
        }
        ::close(*descriptor);
        *descriptor = reserved;
    }
    resize(m_state->columns, m_state->rows);
    const quint64 generation = ++m_state->generation;
    auto *process = new QProcess(this);
    process->setObjectName("terminalShell");
    m_state->process = process;
    process->setWorkingDirectory(workdir);
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert("TERM", "xterm-256color");
    environment.insert("COLORTERM", "truecolor");
    process->setProcessEnvironment(environment);
    const int master = m_state->master;
    const int slave = m_state->slave;
    process->setChildProcessModifier([process, master, slave] {
        if (::setsid() < 0 || ::ioctl(slave, TIOCSCTTY, 0) < 0)
            failTerminalChild(process, "Cannot create terminal session", errno);
        for (int descriptor = 0; descriptor <= 2; ++descriptor) {
            if (::dup2(slave, descriptor) < 0)
                failTerminalChild(process, "Cannot connect terminal device", errno);
        }
        if (slave > STDERR_FILENO)
            ::close(slave);
        if (master > STDERR_FILENO)
            ::close(master);
    });
    m_state->reader = new QSocketNotifier(master, QSocketNotifier::Read, this);
    m_state->reader->setEnabled(false);
    m_state->writer = new QSocketNotifier(master, QSocketNotifier::Write, this);
    m_state->writer->setEnabled(false);
    connect(m_state->reader, &QSocketNotifier::activated, this, &TerminalSession::readOutput);
    connect(m_state->writer, &QSocketNotifier::activated, this, &TerminalSession::writeInput);
    connect(process, &QProcess::started, this, [this, generation] {
        if (generation != m_state->generation)
            return;
        if (m_state->slave >= 0)
            ::close(m_state->slave);
        m_state->slave = -1;
        m_state->shellSession = m_state->process->processId();
        m_state->reader->setEnabled(true);
        emit stateChanged();
    });
    connect(process, &QProcess::errorOccurred, this, [this, process, generation](QProcess::ProcessError failure) {
        if (generation != m_state->generation || failure != QProcess::FailedToStart)
            return;
        m_state->error = tr("Could not start the terminal shell: %1").arg(process->errorString());
        QPointer<TerminalSession> guarded = this;
        stop();
        if (guarded)
            emit finished(-1);
    });
    connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this, generation](int exitCode, QProcess::ExitStatus) {
            if (generation != m_state->generation)
                return;
            // Drain the final bounded PTY tail before relinquishing its master.
            QPointer<TerminalSession> guarded = this;
            readOutput();
            if (!guarded || generation != m_state->generation)
                return;
            stop();
            if (guarded && m_state->generation == generation + 1)
                emit finished(exitCode);
        });
    process->start(shell, {"-i"});
    emit stateChanged();
    return true;
#endif
}

bool TerminalSession::send(QByteArray data)
{
    if (!running() || m_state->master < 0) {
        setError(tr("The terminal is not running."));
        return false;
    }
    if (data.size() > InputLimit || data.size() > InputLimit - m_state->pendingInput.size()) {
        setError(tr("Terminal input exceeds the 64 KiB pending-input limit. Wait before sending more."));
        return false;
    }
    m_state->pendingInput.append(data);
    const quint64 generation = m_state->generation;
    QPointer<TerminalSession> guarded = this;
    writeInput();
    return guarded && generation == m_state->generation && running();
}

void TerminalSession::writeInput()
{
#ifdef Q_OS_UNIX
    if (m_state->master < 0 || !m_state->writer)
        return;
    qsizetype budget = InputLimit;
    int interrupted = 0;
    while (!m_state->pendingInput.isEmpty() && budget > 0) {
        const auto count = ::write(m_state->master, m_state->pendingInput.constData(),
            size_t(qMin(budget, m_state->pendingInput.size())));
        if (count > 0) {
            m_state->pendingInput.remove(0, count);
            budget -= count;
        } else if (count < 0 && errno == EINTR && ++interrupted < 3) {
            continue;
        } else if (count < 0 && (errno == EAGAIN || errno == EWOULDBLOCK || errno == EINTR)) {
            break;
        } else {
            QPointer<TerminalSession> guarded = this;
            stop();
            if (guarded)
                setError(tr("The terminal stopped accepting input."));
            return;
        }
    }
    m_state->writer->setEnabled(!m_state->pendingInput.isEmpty());
#endif
}

void TerminalSession::readOutput()
{
#ifdef Q_OS_UNIX
    if (m_state->master < 0 || !m_state->reader)
        return;
    QByteArray bytes;
    bytes.reserve(ReadLimit);
    char buffer[8192];
    int interrupted = 0;
    bool ended = false;
    while (bytes.size() < ReadLimit) {
        const auto count = ::read(m_state->master, buffer,
            size_t(qMin<qsizetype>(sizeof(buffer), ReadLimit - bytes.size())));
        if (count > 0) {
            bytes.append(buffer, count);
        } else if (count < 0 && errno == EINTR && ++interrupted < 3) {
            continue;
        } else {
            ended = count == 0 || (count < 0 && errno != EAGAIN && errno != EWOULDBLOCK && errno != EINTR);
            break;
        }
    }
    if (ended)
        m_state->reader->setEnabled(false);
    if (!bytes.isEmpty())
        emit output(bytes);
#endif
}

void TerminalSession::resize(int columns, int rows)
{
    m_state->columns = qBound(1, columns, 1000);
    m_state->rows = qBound(1, rows, 1000);
#ifdef Q_OS_UNIX
    if (m_state->master >= 0) {
        winsize size{};
        size.ws_col = static_cast<unsigned short>(m_state->columns);
        size.ws_row = static_cast<unsigned short>(m_state->rows);
        if (::ioctl(m_state->master, TIOCSWINSZ, &size) != 0)
            setError(tr("Could not resize the terminal."));
    }
#endif
}

void TerminalSession::closeDescriptors()
{
    if (m_state->reader) {
        m_state->reader->setEnabled(false);
        m_state->reader->deleteLater();
        m_state->reader = nullptr;
    }
    if (m_state->writer) {
        m_state->writer->setEnabled(false);
        m_state->writer->deleteLater();
        m_state->writer = nullptr;
    }
#ifdef Q_OS_UNIX
    if (m_state->master >= 0)
        ::close(m_state->master);
    if (m_state->slave >= 0)
        ::close(m_state->slave);
#endif
    m_state->master = -1;
    m_state->slave = -1;
    m_state->pendingInput.clear();
}

void TerminalSession::stop()
{
    stopSession(false);
}

void TerminalSession::stopSession(bool immediate)
{
    ++m_state->generation;
    if (!m_state->process && m_state->master < 0 && m_state->slave < 0)
        return;
    auto *process = m_state->process;
    m_state->process = nullptr;
#ifdef Q_OS_UNIX
    const pid_t session = m_state->shellSession > 1 ? pid_t(m_state->shellSession)
        : (process ? pid_t(process->processId()) : -1);
    const pid_t foreground = m_state->master >= 0 ? ::tcgetpgrp(m_state->master) : -1;
    bool complete = true;
    auto members = terminalGroups(session, complete);
    if (session > 1)
        members.append({session, session});
    if (foreground > 1)
        members.append({foreground, foreground});
    signalTerminalGroups(members, session, SIGHUP);
    if (!complete)
        m_state->error = tr("Terminal cleanup reached its process-inspection limit. Some detached jobs may remain.");
#endif
    m_state->shellSession = -1;
    closeDescriptors();
    if (process)
        process->disconnect(this);
    // A separate drain guard survives shell exit long enough to kill remaining
    // foreground/background groups. Application shutdown also forces this drain.
    auto *drain = new QObject(QCoreApplication::instance());
    QPointer<QProcess> guarded = process;
    const auto killRemaining = [guarded
#ifdef Q_OS_UNIX
        , session, members
#endif
    ] {
#ifdef Q_OS_UNIX
        signalTerminalGroups(members, session, SIGKILL);
#endif
        if (guarded && guarded->state() != QProcess::NotRunning)
            guarded->kill();
    };
    if (process) {
        if (process->state() == QProcess::NotRunning) {
            process->deleteLater();
        } else {
            process->setParent(QCoreApplication::instance());
            connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), process, &QObject::deleteLater);
            connect(process, &QProcess::started, process, [process] { process->kill(); });
            connect(process, &QProcess::errorOccurred, process, [process](QProcess::ProcessError failure) {
                if (failure == QProcess::FailedToStart)
                    process->deleteLater();
            });
        }
    }
    if (immediate) {
        killRemaining();
        drain->deleteLater();
    } else {
        connect(QCoreApplication::instance(), &QCoreApplication::aboutToQuit, drain, killRemaining);
        QTimer::singleShot(500, drain, [drain, killRemaining] {
            killRemaining();
            drain->deleteLater();
        });
    }
    emit stateChanged();
}
