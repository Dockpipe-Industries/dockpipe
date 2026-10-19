#include "TerminalHostLaunch.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QFile>
#include <QProcess>
#include <QProcessEnvironment>
#include <QRegularExpression>
#include <QScopeGuard>
#include <QTemporaryDir>
#include <QTest>

#ifdef Q_OS_LINUX
#include <cerrno>
#include <cstdio>
#include <csignal>
#include <fcntl.h>
#include <poll.h>
#include <sys/ioctl.h>
#include <sys/prctl.h>
#include <sys/socket.h>
#include <sys/wait.h>
#include <termios.h>
#include <unistd.h>

namespace {
volatile sig_atomic_t bridgeControl = -1;

void forwardBridgeSignal(int signal)
{
    const char message = signal == SIGHUP ? 'H' : 'T';
    if (bridgeControl >= 0) {
        const auto written = ::write(bridgeControl, &message, 1);
        (void)written;
    }
}

int bridgeFixture(const QStringList &arguments)
{
    const int separator = arguments.indexOf("--");
    if (separator < 0 || !arguments.contains("--watch-bus"))
        return 120;
    QString directory;
    for (const auto &argument : arguments) {
        if (argument.startsWith("--directory="))
            directory = argument.mid(12);
        if (argument.startsWith("--env=")) {
            const auto assignment = argument.mid(6).toUtf8();
            const int equals = assignment.indexOf('=');
            if (equals <= 0)
                return 121;
            ::setenv(assignment.left(equals).constData(), assignment.mid(equals + 1).constData(), 1);
        }
    }
    if (::chdir(directory.toLocal8Bit().constData()) != 0)
        return 122;
    QList<QByteArray> encoded;
    for (const auto &argument : arguments.mid(separator + 1))
        encoded.append(argument.toLocal8Bit());
    QVector<char *> command;
    for (auto &argument : encoded)
        command.append(argument.data());
    command.append(nullptr);
    int control[2];
    if (::socketpair(AF_UNIX, SOCK_STREAM | SOCK_CLOEXEC, 0, control) != 0)
        return 123;
    const pid_t portal = ::fork();
    if (portal < 0)
        return 124;
    if (portal == 0) {
        ::close(control[0]);
        ::setsid();
        ::prctl(PR_SET_CHILD_SUBREAPER, 1);
        const pid_t host = ::fork();
        if (host == 0) {
            ::close(control[1]);
            // Faithfully model Flatpak HostCommand's setup: an independent host
            // session claims the forwarded stdio PTY, not the sandbox bridge.
            if (::setsid() < 0 || ::ioctl(STDIN_FILENO, TIOCSCTTY, 0) < 0)
                ::_exit(126);
            ::execvp(command.first(), command.data());
            ::_exit(127);
        }
        if (host < 0)
            ::_exit(124);
        bool busConnected = true;
        QElapsedTimer disconnected;
        int status = 0;
        while (::waitpid(host, &status, WNOHANG) == 0) {
            if (busConnected) {
                pollfd descriptor{control[1], POLLIN | POLLHUP, 0};
                if (::poll(&descriptor, 1, 20) > 0) {
                    char message = 0;
                    const auto count = ::read(control[1], &message, 1);
                    if (count == 0) {
                        // Flatpak's watch-bus handler sends SIGINT to the host
                        // command group when the bridge's bus owner vanishes.
                        ::kill(-host, SIGINT);
                        busConnected = false;
                        disconnected.start();
                    } else if (count == 1) {
                        ::kill(host, message == 'H' ? SIGHUP : SIGTERM);
                    }
                }
            } else {
                ::usleep(20000);
                if (disconnected.elapsed() > 5000)
                    ::kill(-host, SIGKILL);
            }
        }
        if (busConnected) {
            const auto written = ::write(control[1], &status, sizeof(status));
            (void)written;
        }
        ::close(control[1]);
        // Test-owned subreaper prevents killed fixture jobs accumulating under
        // the machine's PID1. No unrelated process can become this child.
        QElapsedTimer reap;
        reap.start();
        while (reap.elapsed() < 2000) {
            int childStatus = 0;
            const pid_t child = ::waitpid(-1, &childStatus, WNOHANG);
            if (child < 0 && errno == ECHILD)
                break;
            ::usleep(10000);
        }
        ::_exit(0);
    }
    ::close(control[1]);
    bridgeControl = control[0];
    ::signal(SIGHUP, forwardBridgeSignal);
    ::signal(SIGTERM, forwardBridgeSignal);
    int status = 0;
    size_t received = 0;
    while (received < sizeof(status)) {
        const auto count = ::read(control[0], reinterpret_cast<char *>(&status) + received, sizeof(status) - received);
        if (count > 0)
            received += size_t(count);
        else if (count < 0 && errno == EINTR)
            continue;
        else
            break;
    }
    ::close(control[0]);
    bridgeControl = -1;
    ::waitpid(portal, nullptr, 0);
    return received == sizeof(status) && WIFEXITED(status) ? WEXITSTATUS(status) : 128;
}

class HostPty final {
public:
    QProcess process;
    QByteArray output;
    int master = -1;
    int slave = -1;

    ~HostPty()
    {
        if (process.state() != QProcess::NotRunning) {
            process.terminate();
            if (!process.waitForFinished(5000)) {
                process.kill();
                process.waitForFinished(1000);
            }
        }
        if (master >= 0)
            ::close(master);
        if (slave >= 0)
            ::close(slave);
    }

    bool start(const TerminalHostLaunch::Plan &plan, const QProcessEnvironment &environment = QProcessEnvironment::systemEnvironment())
    {
        master = ::posix_openpt(O_RDWR | O_NOCTTY | O_NONBLOCK | O_CLOEXEC);
        if (master < 0 || ::grantpt(master) || ::unlockpt(master))
            return false;
        slave = ::open(::ptsname(master), O_RDWR | O_NOCTTY | O_CLOEXEC);
        if (slave < 0)
            return false;
        process.setWorkingDirectory(plan.workingDirectory);
        process.setProcessEnvironment(environment);
        process.setChildProcessModifier([this] {
            // The bridge is local, but must not own the host's future ctty.
            if (::setsid() < 0)
                ::_exit(126);
            for (int descriptor = 0; descriptor < 3; ++descriptor) {
                if (::dup2(slave, descriptor) < 0)
                    ::_exit(126);
            }
            ::close(master);
            ::close(slave);
        });
        process.start(plan.program, plan.arguments);
        const bool started = process.waitForStarted(3000);
        ::close(slave);
        slave = -1;
        return started;
    }

    qint64 foregroundGroup(const QString &bridge, qint64 job) const
    {
        if (bridge == QCoreApplication::applicationFilePath())
            return ::tcgetpgrp(master);
        // Host IDs cannot be compared to tcgetpgrp in the sandbox PID namespace.
        // Query only the known fixture job on the host, through the real portal.
        QProcess query;
        query.start(bridge, {"--host", "--watch-bus", "--directory=/", "--", "ps",
            "-o", "tpgid=", "-p", QString::number(job)});
        if (!query.waitForFinished(1000)) {
            query.kill();
            query.waitForFinished(1000);
            return -1;
        }
        if (query.exitCode() != 0)
            return -1;
        return query.readAllStandardOutput().trimmed().toLongLong();
    }

    bool send(const QByteArray &bytes)
    {
        return ::write(master, bytes.constData(), size_t(bytes.size())) == bytes.size();
    }

    void drain()
    {
        char buffer[4096];
        for (int reads = 0; reads < 16; ++reads) {
            const auto count = ::read(master, buffer, sizeof(buffer));
            if (count <= 0)
                break;
            if (output.size() < 256 * 1024)
                output.append(buffer, int(count));
        }
    }

    bool contains(const QByteArray &marker)
    {
        drain();
        return output.contains(marker);
    }
};

bool processRunning(qint64 process, const QString &bridge)
{
    if (bridge != QCoreApplication::applicationFilePath()) {
        QProcess query;
        query.start(bridge, {"--host", "--watch-bus", "--directory=/", "--", "ps",
            "-o", "stat=", "-p", QString::number(process)});
        if (!query.waitForFinished(1000)) {
            query.kill();
            query.waitForFinished(1000);
            return true;
        }
        if (query.exitStatus() != QProcess::NormalExit)
            return true;
        if (query.exitCode() == 1)
            return false;
        const auto state = query.readAllStandardOutput().trimmed();
        return query.exitCode() != 0 || state.isEmpty() || !state.startsWith('Z');
    }
    QFile status(QString("/proc/%1/stat").arg(process));
    if (!status.open(QIODevice::ReadOnly))
        return false;
    const auto fields = status.read(4096).split(')');
    return fields.size() < 2 || !fields.last().trimmed().startsWith('Z');
}
}
#endif

class TerminalHostLaunchTest final : public QObject {
    Q_OBJECT
private slots:
    void keepsArgumentsSeparate();
    void rejectsInvalidLaunch();
    void hostPtyJobControlAndResize();
    void nativeFlatpakHostPty();
    void cleanupAcrossJobGroups_data();
    void cleanupAcrossJobGroups();
    void nativeFlatpakHostCleanup_data();
    void nativeFlatpakHostCleanup();
    void reportsUnsupportedHostTools_data();
    void reportsUnsupportedHostTools();
    void ignoresUnrelatedInventoryMembers();
    void cleansProcessWithNewlineName();
private:
    void exerciseHostPty(const QString &bridge);
    void exerciseHostCleanup(bool abrupt, const QString &bridge);
};

void TerminalHostLaunchTest::keepsArgumentsSeparate()
{
    const QString executable = QCoreApplication::applicationFilePath();
    const auto plan = TerminalHostLaunch::forHost("/bin/bash", {"-i", "literal; $(not-a-command)"}, "/tmp/a b", executable);
    QVERIFY2(plan.error.isEmpty(), qPrintable(plan.error));
    QVERIFY(plan.host);
    QCOMPARE(plan.program, executable);
    QVERIFY(plan.arguments.contains("--directory=/tmp/a b"));
    QCOMPARE(plan.arguments.last(), QString("literal; $(not-a-command)"));
    QVERIFY(!plan.arguments.at(8).contains("/tmp/a b"));
}

void TerminalHostLaunchTest::rejectsInvalidLaunch()
{
    const QString bridge = QCoreApplication::applicationFilePath();
    QVERIFY(!TerminalHostLaunch::forHost("bash", {}, "/tmp", bridge).error.isEmpty());
    QVERIFY(!TerminalHostLaunch::forHost("/bin/sh", {}, "relative", bridge).error.isEmpty());
    QVERIFY(!TerminalHostLaunch::forHost("/bin/sh", {QString(1, QChar::Null)}, "/tmp", bridge).error.isEmpty());
    QVERIFY(!TerminalHostLaunch::forHost("/bin/sh", {QString(65536, 'x')}, "/tmp", bridge).error.isEmpty());
    QVERIFY(!TerminalHostLaunch::forHost("/bin/sh", {}, "/tmp", "/not-a-bridge").error.isEmpty());
}

void TerminalHostLaunchTest::hostPtyJobControlAndResize()
{
    exerciseHostPty(QCoreApplication::applicationFilePath());
}

void TerminalHostLaunchTest::nativeFlatpakHostPty()
{
    if (!TerminalHostLaunch::required())
        QSKIP("Actual Flatpak portal proof requires execution inside a Flatpak sandbox.");
    exerciseHostPty("/usr/bin/flatpak-spawn");
}

void TerminalHostLaunchTest::exerciseHostPty(const QString &bridge)
{
#ifndef Q_OS_LINUX
    Q_UNUSED(bridge);
    QSKIP("The Flatpak host PTY fixture requires Linux.");
#else
    HostPty terminal;
    const QMap<QString, QString> environment{{"PS1", "DOCKPIPE_HOST_PROMPT> "}, {"HISTFILE", "/dev/null"}};
    const auto plan = TerminalHostLaunch::forHost("/bin/bash", {"--noprofile", "--norc", "-i"}, "/tmp", bridge, environment);
    QVERIFY2(terminal.start(plan), qPrintable(terminal.process.errorString()));
    QTRY_VERIFY2_WITH_TIMEOUT(terminal.contains("DOCKPIPE_HOST_PROMPT> "), terminal.output.constData(), 3000);
    QVERIFY(terminal.send("trap 'printf \"__WINCH__\\n\"' WINCH; printf '__HOST_READY__\\n'\n"));
    QTRY_VERIFY2_WITH_TIMEOUT(terminal.contains("__HOST_READY__\r\n"), terminal.output.constData(), 3000);
    QVERIFY2(!terminal.output.contains("no job control"), terminal.output.constData());
    terminal.output.clear();
    winsize dimensions{};
    dimensions.ws_col = 113;
    dimensions.ws_row = 37;
    QCOMPARE(::ioctl(terminal.master, TIOCSWINSZ, &dimensions), 0);
    QTRY_VERIFY_WITH_TIMEOUT(terminal.contains("__WINCH__"), 3000);
    QVERIFY(terminal.send("stty size; sh -c 'printf \"__JOB__%s\\n\" $$; exec sleep 30'\n"));
    QRegularExpression jobIdentity("__JOB__(\\d+)\\r?\\n");
    QTRY_VERIFY_WITH_TIMEOUT((terminal.drain(), terminal.output.contains("37 113\r\n")
        && jobIdentity.match(QString::fromUtf8(terminal.output)).hasMatch()), 3000);
    const qint64 job = jobIdentity.match(QString::fromUtf8(terminal.output)).captured(1).toLongLong();
    QTRY_COMPARE_WITH_TIMEOUT(terminal.foregroundGroup(bridge, job), job, 3000);
    QVERIFY(terminal.send(QByteArray(1, char(26))));
    QTRY_VERIFY_WITH_TIMEOUT(terminal.contains("Stopped"), 3000);
    QVERIFY(terminal.send("bg\nfg\n"));
    QTRY_COMPARE_WITH_TIMEOUT(terminal.foregroundGroup(bridge, job), job, 3000);
    terminal.output.clear();
    QVERIFY(terminal.send(QByteArray(1, char(3))));
    QTRY_VERIFY2_WITH_TIMEOUT(terminal.contains("DOCKPIPE_HOST_PROMPT> "), terminal.output.constData(), 3000);
    QVERIFY(terminal.send("printf '__AFTER_INTERRUPT__\\n'; exit 17\n"));
    QTRY_VERIFY2_WITH_TIMEOUT(terminal.contains("__AFTER_INTERRUPT__\r\n"), terminal.output.constData(), 3000);
    QTRY_COMPARE_WITH_TIMEOUT(terminal.process.state(), QProcess::NotRunning, 5000);
    QCOMPARE(terminal.process.exitCode(), 17);
#endif
}

void TerminalHostLaunchTest::cleanupAcrossJobGroups_data()
{
    QTest::addColumn<bool>("abrupt");
    QTest::newRow("forwarded-termination") << false;
    QTest::newRow("bridge-death-watch-bus") << true;
}

void TerminalHostLaunchTest::cleanupAcrossJobGroups()
{
    QFETCH(bool, abrupt);
    exerciseHostCleanup(abrupt, QCoreApplication::applicationFilePath());
}

void TerminalHostLaunchTest::nativeFlatpakHostCleanup_data()
{
    cleanupAcrossJobGroups_data();
}

void TerminalHostLaunchTest::nativeFlatpakHostCleanup()
{
    if (!TerminalHostLaunch::required())
        QSKIP("Actual Flatpak cleanup proof requires execution inside a Flatpak sandbox.");
    QFETCH(bool, abrupt);
    exerciseHostCleanup(abrupt, "/usr/bin/flatpak-spawn");
}

void TerminalHostLaunchTest::exerciseHostCleanup(bool abrupt, const QString &bridge)
{
#ifndef Q_OS_LINUX
    Q_UNUSED(abrupt);
    Q_UNUSED(bridge);
    QSKIP("The Flatpak host PTY fixture requires Linux.");
#else
    HostPty terminal;
    const auto plan = TerminalHostLaunch::forHost("/bin/bash", {"--noprofile", "--norc", "-i"}, "/tmp", bridge,
        {{"PS1", "DOCKPIPE_HOST_PROMPT> "}, {"HISTFILE", "/dev/null"}});
    QVERIFY(terminal.start(plan));
    QTRY_VERIFY2_WITH_TIMEOUT(terminal.contains("DOCKPIPE_HOST_PROMPT> "), terminal.output.constData(), 3000);
    QVERIFY(terminal.send("sh -c 'trap \"\" HUP TERM; printf \"__BACKGROUND__%s\\n\" $$; exec sleep 30' &\n"));
    QRegularExpression identity("__BACKGROUND__(\\d+)\\r?\\n");
    QTRY_VERIFY2_WITH_TIMEOUT((terminal.drain(), identity.match(QString::fromUtf8(terminal.output)).hasMatch()), terminal.output.constData(), 3000);
    const qint64 background = identity.match(QString::fromUtf8(terminal.output)).captured(1).toLongLong();
    QVERIFY(background > 1 && processRunning(background, bridge));
    QVERIFY(terminal.send("sh -c 'printf \"__FOREGROUND__%s\\n\" $$; exec sleep 30'\n"));
    QRegularExpression foregroundIdentity("__FOREGROUND__(\\d+)\\r?\\n");
    QTRY_VERIFY_WITH_TIMEOUT((terminal.drain(), foregroundIdentity.match(QString::fromUtf8(terminal.output)).hasMatch()), 3000);
    const qint64 foreground = foregroundIdentity.match(QString::fromUtf8(terminal.output)).captured(1).toLongLong();
    QVERIFY(foreground > 1 && processRunning(foreground, bridge));
    if (abrupt)
        terminal.process.kill();
    else
        terminal.process.terminate();
    QTRY_COMPARE_WITH_TIMEOUT(terminal.process.state(), QProcess::NotRunning, 5000);
    QTRY_VERIFY_WITH_TIMEOUT(!processRunning(background, bridge) && !processRunning(foreground, bridge), 5000);
#endif
}

void TerminalHostLaunchTest::reportsUnsupportedHostTools_data()
{
    QTest::addColumn<QByteArray>("producer");
    QTest::addColumn<QByteArray>("diagnostic");
    QTest::addColumn<bool>("completeTools");
    const QByteArray failedInventory("could not verify complete session cleanup");
    QTest::newRow("unsupported-ps") << QByteArray("#!/bin/sh\nexit 42\n") << failedInventory << true;
    QTest::newRow("empty-success-inventory") << QByteArray("#!/bin/sh\nexit 0\n") << failedInventory << true;
    QTest::newRow("timed-out-producer") << QByteArray("#!/bin/sh\nexec sleep 5\n") << failedInventory << true;
    QTest::newRow("too-many-members") << QByteArray(
        "#!/bin/sh\n"
        "index=0\n"
        "while [ \"$index\" -lt 1025 ]; do\n"
        "    printf '2\\n'\n"
        "    index=$((index + 1))\n"
        "done\n") << QByteArray("exceeded 1024 session members") << true;
    QTest::newRow("missing-utility") << QByteArray("#!/bin/sh\nexit 0\n")
        << QByteArray("host terminal requires head") << false;
}

void TerminalHostLaunchTest::reportsUnsupportedHostTools()
{
#ifndef Q_OS_LINUX
    QSKIP("The Flatpak host PTY fixture requires Linux.");
#else
    QFETCH(QByteArray, producer);
    QFETCH(QByteArray, diagnostic);
    QFETCH(bool, completeTools);
    QTemporaryDir tools;
    QVERIFY(tools.isValid());
    QFile ps(tools.filePath("ps"));
    QVERIFY(ps.open(QIODevice::WriteOnly));
    QCOMPARE(ps.write(producer), qint64(producer.size()));
    ps.close();
    QVERIFY(ps.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert("PATH", completeTools ? tools.path() + ":/usr/bin:/bin" : tools.path());
    HostPty terminal;
    const auto plan = TerminalHostLaunch::forHost("/bin/sh", {"-c", "exit 0"}, "/tmp", QCoreApplication::applicationFilePath());
    QVERIFY(terminal.start(plan, environment));
    QTRY_COMPARE_WITH_TIMEOUT(terminal.process.state(), QProcess::NotRunning, 5000);
    QCOMPARE(terminal.process.exitCode(), 125);
    QVERIFY2(terminal.contains(diagnostic), terminal.output.constData());
#endif
}

void TerminalHostLaunchTest::ignoresUnrelatedInventoryMembers()
{
#ifndef Q_OS_LINUX
    QSKIP("The Flatpak host session inventory fixture requires Linux.");
#else
    QProcess unrelated;
    unrelated.setChildProcessModifier([] {
        if (::setsid() < 0)
            ::_exit(126);
    });
    unrelated.start("/bin/sleep", {"30"});
    QVERIFY(unrelated.waitForStarted(3000));
    const auto cleanup = qScopeGuard([&] {
        if (unrelated.state() != QProcess::NotRunning) {
            unrelated.kill();
            unrelated.waitForFinished(1000);
        }
    });
    QTemporaryDir tools;
    QVERIFY(tools.isValid());
    QFile ps(tools.filePath("ps"));
    QVERIFY(ps.open(QIODevice::WriteOnly));
    const QByteArray producer = "#!/bin/sh\nprintf '%s\\n' \"$2\" \"$DOCKPIPE_TEST_UNRELATED\"\n";
    QCOMPARE(ps.write(producer), qint64(producer.size()));
    ps.close();
    QVERIFY(ps.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert("PATH", tools.path() + ":/usr/bin:/bin");
    environment.insert("DOCKPIPE_TEST_UNRELATED", QString::number(unrelated.processId()));
    HostPty terminal;
    const auto plan = TerminalHostLaunch::forHost("/bin/sh", {"-c", "exit 0"}, "/tmp", QCoreApplication::applicationFilePath());
    QVERIFY(terminal.start(plan, environment));
    QTRY_COMPARE_WITH_TIMEOUT(terminal.process.state(), QProcess::NotRunning, 5000);
    QCOMPARE(terminal.process.exitCode(), 0);
    QCOMPARE(unrelated.state(), QProcess::Running);
#endif
}

void TerminalHostLaunchTest::cleansProcessWithNewlineName()
{
#ifndef Q_OS_LINUX
    QSKIP("The process-name cleanup regression requires Linux.");
#else
    HostPty terminal;
    const QString executable = QCoreApplication::applicationFilePath();
    const auto plan = TerminalHostLaunch::forHost(executable, {"--named-job"}, "/tmp", executable);
    QVERIFY(terminal.start(plan));
    QRegularExpression identity("__NAMED_JOB__(\\d+)\\r?\\n");
    QTRY_VERIFY2_WITH_TIMEOUT((terminal.drain(), identity.match(QString::fromUtf8(terminal.output)).hasMatch()),
        terminal.output.constData(), 3000);
    const qint64 job = identity.match(QString::fromUtf8(terminal.output)).captured(1).toLongLong();
    QVERIFY(job > 1);
    QFile initialStatus(QString("/proc/%1/stat").arg(job));
    QVERIFY(initialStatus.open(QIODevice::ReadOnly));
    const auto initialRecord = initialStatus.read(4096);
    const auto initialStart = initialRecord.mid(initialRecord.lastIndexOf(") ") + 2).split(' ').value(19);
    QVERIFY(!initialStart.isEmpty());
    const auto cleanup = qScopeGuard([&] {
        QFile status(QString("/proc/%1/stat").arg(job));
        if (!status.open(QIODevice::ReadOnly))
            return;
        const auto record = status.read(4096);
        const auto start = record.mid(record.lastIndexOf(") ") + 2).split(' ').value(19);
        if (record.contains("(dock\njob)") && start == initialStart)
            ::kill(pid_t(job), SIGKILL);
    });
    terminal.process.terminate();
    QTRY_COMPARE_WITH_TIMEOUT(terminal.process.state(), QProcess::NotRunning, 5000);
    QTRY_VERIFY_WITH_TIMEOUT(!processRunning(job, executable), 5000);
#endif
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
#ifdef Q_OS_LINUX
    if (application.arguments().value(1) == "--named-job") {
        ::prctl(PR_SET_NAME, "dock\njob", 0, 0, 0);
        ::signal(SIGHUP, SIG_IGN);
        ::signal(SIGTERM, SIG_IGN);
        ::alarm(10);
        std::printf("__NAMED_JOB__%ld\n", long(::getpid()));
        std::fflush(stdout);
        for (;;)
            ::pause();
    }
    if (application.arguments().value(1) == "--host")
        return bridgeFixture(application.arguments().mid(1));
#endif
    TerminalHostLaunchTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "TerminalHostLaunchTest.moc"
