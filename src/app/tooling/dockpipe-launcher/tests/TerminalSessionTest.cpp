#include "TerminalSession.h"
#include "TerminalHostLaunch.h"

#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QPointer>
#include <QProcess>
#include <QRegularExpression>
#include <QSignalSpy>
#include <QScopeGuard>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <functional>

#ifdef Q_OS_UNIX
#include <cerrno>
#include <signal.h>
#include <unistd.h>
#endif

// macOS treats manually polled processEvents differently from a running
// application event loop. Exercise the real timer/notifier dispatcher here.
static bool waitForTerminalEvents(const std::function<bool()> &predicate, int timeoutMs)
{
    if (predicate())
        return true;
    QEventLoop loop;
    QElapsedTimer elapsed;
    elapsed.start();
    bool matched = false;
    QTimer observer;
    observer.setTimerType(Qt::PreciseTimer);
    observer.setInterval(1);
    QObject::connect(&observer, &QTimer::timeout, &loop, [&] {
        if (elapsed.elapsed() < timeoutMs && predicate()) {
            matched = true;
            loop.quit();
        }
    });
    QTimer::singleShot(timeoutMs, Qt::PreciseTimer, &loop, &QEventLoop::quit);
    observer.start();
    loop.exec();
    return matched;
}

class TerminalSessionTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void terminalInputResizeAndExit();
    void explicitProgramAndArguments();
    void nativeFlatpakHostIntegration();
    void rejectsInvalidInputAndDirectory();
    void outputFloodDoesNotBlockEventLoop_data();
    void outputFloodDoesNotBlockEventLoop();
    void outputCallbackCanRestartSession();
    void outputCallbackCanDestroySession();
    void stopCleansForegroundJobWithoutBlocking_data();
    void stopCleansForegroundJobWithoutBlocking();
    void immediateDestructionReapsStartingShell();
    void cleanupTestCase();
private:
    QByteArray m_originalShell;
};

void TerminalSessionTest::initTestCase()
{
#ifndef Q_OS_UNIX
    QSKIP("Unix PTY fixture; Windows ConPTY has a dedicated native fixture.");
#else
    m_originalShell = qgetenv("SHELL");
    qputenv("SHELL", "/bin/sh");
#endif
}

void TerminalSessionTest::terminalInputResizeAndExit()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalSession terminal;
    QByteArray received;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { received += bytes; });
    QSignalSpy exited(&terminal, &TerminalSession::finished);
    terminal.resize(117, 41);
    QVERIFY(terminal.start(directory.path()));
    QVERIFY(terminal.send("stty -echo\n"));
    // A following command cannot be mistaken for terminal echo after stty.
    QVERIFY(terminal.send("test -t 0 && test -t 1 && test -t 2 && printf '\\nPTY_READY\\n'; printf 'SIZE='; stty size; pwd\n"));
    QTRY_VERIFY_WITH_TIMEOUT(received.contains("SIZE=41 117"), 3000);
    QVERIFY(received.contains("PTY_READY\r\n"));
    QVERIFY(received.contains(directory.path().toUtf8()));
    terminal.resize(92, 29);
    QVERIFY(terminal.send("printf 'RESIZED='; stty size\n"));
    QTRY_VERIFY_WITH_TIMEOUT(received.contains("RESIZED=29 92"), 3000);
    QVERIFY(terminal.send("printf '\\nFINAL_TAIL\\n'; exit 7\n"));
    QTRY_COMPARE_WITH_TIMEOUT(exited.size(), 1, 3000);
    QCOMPARE(exited.front().front().toInt(), 7);
    QVERIFY(received.contains("FINAL_TAIL\r\n"));
    QVERIFY(!terminal.running());
}

void TerminalSessionTest::explicitProgramAndArguments()
{
    TerminalSession terminal;
    QByteArray output;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { output += bytes; });
    QSignalSpy finished(&terminal, &TerminalSession::finished);
    QVERIFY(terminal.start({}, QStringLiteral("/bin/sh"),
        {QStringLiteral("-c"), QStringLiteral("printf 'EXPLICIT:%s\n' \"$1\"; exit 9"),
            QStringLiteral("terminal-fixture"), QStringLiteral("space and 'quote' and \"double\"")}));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 3000);
    QCOMPARE(finished.front().front().toInt(), 9);
    QVERIFY2(output.contains("EXPLICIT:space and 'quote' and \"double\""), output.constData());
    QVERIFY(!terminal.start({}, QStringLiteral("relative-shell"), {}));
    finished.clear();
    output.clear();
    QVERIFY(terminal.start({}, QStringLiteral("/bin/sh"),
        {QStringLiteral("-c"), QStringLiteral("printf 'ENVIRONMENT:%s' \"$DOCKPIPE_TERMINAL_FIXTURE\"")},
        {{QStringLiteral("DOCKPIPE_TERMINAL_FIXTURE"), QStringLiteral("space and quote' and $literal")}}));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 3000);
    QVERIFY2(output.contains("ENVIRONMENT:space and quote' and $literal"), output.constData());
}

void TerminalSessionTest::nativeFlatpakHostIntegration()
{
    if (!TerminalHostLaunch::required())
        QSKIP("The public host-terminal integration requires an actual Flatpak runtime.");
    TerminalSession terminal;
    QByteArray output;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { output += bytes; });
    QSignalSpy finished(&terminal, &TerminalSession::finished);
    terminal.resize(117, 41);
    const QMap<QString, QString> environment = {
        {QStringLiteral("PS1"), QStringLiteral("DOCKPIPE_HOST_READY> ")},
        {QStringLiteral("HISTFILE"), QStringLiteral("/dev/null")},
    };
    QVERIFY(terminal.start(QStringLiteral("/tmp"), QStringLiteral("/bin/bash"),
        {QStringLiteral("--noprofile"), QStringLiteral("--norc"), QStringLiteral("-i")}, environment));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("DOCKPIPE_HOST_READY>"), 5000);
    QVERIFY(terminal.send("printf 'HOST_SIZE='; stty size\r"));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("HOST_SIZE=41 117"), 3000);
    QVERIFY(terminal.send("printf 'HOST_CWD='; pwd\r"));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("HOST_CWD=/tmp"), 3000);
    terminal.resize(92, 29);
    QVERIFY(terminal.send("printf 'HOST_RESIZED='; stty size\r"));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("HOST_RESIZED=29 92"), 3000);
    // The expected marker is absent from echoed input and is assembled by the
    // real host shell. The host supervisor must also preserve its exit status.
    QVERIFY(terminal.send("printf 'HOST_%s\\n' 'FINAL_TAIL'; exit 7\r"));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 5000);
    QCOMPARE(finished.front().front().toInt(), 7);
    QVERIFY2(output.contains("HOST_FINAL_TAIL"), output.constData());
    QVERIFY(!terminal.running());
}

void TerminalSessionTest::rejectsInvalidInputAndDirectory()
{
    TerminalSession terminal;
    QVERIFY(!terminal.start("/a/dockpipe/fixture/that/does/not/exist"));
    QVERIFY(!terminal.error().isEmpty());
    QVERIFY(!terminal.running());
    QVERIFY(!terminal.send("no shell"));
    QTemporaryDir directory;
    QVERIFY(terminal.start(directory.path()));
    QVERIFY(!terminal.send(QByteArray(64 * 1024 + 1, 'x')));
    QVERIFY(terminal.error().contains("64 KiB"));
    QVERIFY(terminal.running());
    terminal.stop();
    QVERIFY(!terminal.running());
}

void TerminalSessionTest::outputFloodDoesNotBlockEventLoop_data()
{
    QTest::addColumn<QByteArray>("command");
    QTest::newRow("yes") << QByteArray("stty -echo; yes terminal-output-fixture\n");
    QTest::newRow("small-writes") << QByteArray(
        "stty -echo; while :; do printf 'terminal-output-fixture\\n'; done\n");
}

void TerminalSessionTest::outputFloodDoesNotBlockEventLoop()
{
    QFETCH(QByteArray, command);
    TerminalSession terminal;
    qint64 total = 0;
    qsizetype largest = 0;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) {
        total += bytes.size();
        largest = qMax(largest, bytes.size());
    });
    int ticks = 0;
    QTimer timer;
    timer.setInterval(10);
    connect(&timer, &QTimer::timeout, this, [&] { ++ticks; });
    timer.start();
    QElapsedTimer elapsed;
    elapsed.start();
    const auto diagnostics = qScopeGuard([&] {
        qInfo() << "Terminal flood diagnostics bytes" << total << "largest batch" << largest
                << "elapsed ms" << elapsed.elapsed() << "heartbeat ticks" << ticks;
    });
    QVERIFY(terminal.start({}));
    QVERIFY(terminal.send(command));
    QVERIFY(waitForTerminalEvents([&] { return total >= 256 * 1024; }, 3000));
    ticks = 0;
    QEventLoop heartbeatWindow;
    QTimer::singleShot(350, Qt::PreciseTimer, &heartbeatWindow, &QEventLoop::quit);
    heartbeatWindow.exec();
    qInfo() << "Terminal flood heartbeat ticks" << ticks << "bytes" << total;
    QVERIFY(ticks >= 10);
    QVERIFY(largest <= 64 * 1024);
    QElapsedTimer stopTime;
    stopTime.start();
    terminal.stop();
    QVERIFY2(stopTime.elapsed() < 200, "Terminal output flood blocked shutdown");
    QVERIFY(!terminal.running());
}

void TerminalSessionTest::outputCallbackCanRestartSession()
{
    TerminalSession terminal;
    qint64 total = 0;
    bool restarted = false;
    QByteArray nextSessionOutput;
    QElapsedTimer elapsed;
    elapsed.start();
    const auto diagnostics = qScopeGuard([&] {
        qInfo() << "Terminal restart diagnostics bytes" << total << "elapsed ms" << elapsed.elapsed()
                << "restarted" << restarted << "next output bytes" << nextSessionOutput.size();
    });
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) {
        if (restarted) {
            nextSessionOutput += bytes;
            return;
        }
        total += bytes.size();
        if (total < 256 * 1024)
            return;
        terminal.stop();
        restarted = true;
        QVERIFY(terminal.start({}));
        QVERIFY(terminal.send("stty -echo; printf '\\nRESTART_%s\\n' ready\n"));
    });
    QVERIFY(terminal.start({}));
    QVERIFY(terminal.send("stty -echo; yes old-session-output-fixture\n"));
    QVERIFY(waitForTerminalEvents([&] {
        return restarted && nextSessionOutput.contains("RESTART_ready");
    }, 3000));
    QVERIFY(!nextSessionOutput.contains("old-session-output-fixture"));
    QVERIFY(terminal.running());
    terminal.stop();
    const auto finalOutput = nextSessionOutput;
    QTest::qWait(30);
    QCOMPARE(nextSessionOutput, finalOutput);
}

void TerminalSessionTest::outputCallbackCanDestroySession()
{
    QPointer<TerminalSession> terminal = new TerminalSession;
    qint64 total = 0;
    QElapsedTimer elapsed;
    elapsed.start();
    const auto diagnostics = qScopeGuard([&] {
        qInfo() << "Terminal destruction diagnostics bytes" << total << "elapsed ms" << elapsed.elapsed()
                << "destroyed" << terminal.isNull();
    });
    connect(terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) {
        total += bytes.size();
        if (total >= 256 * 1024)
            delete terminal.data();
    });
    QVERIFY(terminal->start({}));
    QVERIFY(terminal->send("stty -echo; yes destroyed-session-output-fixture\n"));
    QVERIFY(waitForTerminalEvents([&] { return !terminal; }, 3000));
    QTest::qWait(30);
}

void TerminalSessionTest::stopCleansForegroundJobWithoutBlocking_data()
{
    QTest::addColumn<bool>("background");
    QTest::newRow("foreground") << false;
    QTest::newRow("background") << true;
}

void TerminalSessionTest::stopCleansForegroundJobWithoutBlocking()
{
#ifdef Q_OS_UNIX
    QFETCH(bool, background);
    TerminalSession terminal;
    QByteArray received;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { received += bytes; });
    QVERIFY(terminal.start({}));
    // The foreground group leader survives HUP so the delayed kill is exercised.
    QByteArray command("stty -echo; /bin/sh -c 'trap \"\" HUP TERM; echo CHILD_PID=$$; while :; do sleep 1; done'");
    if (background)
        command.append(" &");
    command.append('\n');
    QVERIFY(terminal.send(command));
    const QRegularExpression expression("CHILD_PID=([0-9]+)");
    QRegularExpressionMatch match;
    QTRY_VERIFY_WITH_TIMEOUT((match = expression.match(QString::fromUtf8(received))).hasMatch(), 3000);
    const pid_t child = pid_t(match.captured(1).toLongLong());
    QVERIFY(child > 1);
    QVERIFY(::kill(child, 0) == 0);
    QPointer<QProcess> shell = terminal.findChild<QProcess *>("terminalShell");
    QVERIFY(shell);
    QElapsedTimer stopTime;
    stopTime.start();
    terminal.stop();
    QVERIFY(stopTime.elapsed() < 200);
    QTRY_VERIFY_WITH_TIMEOUT(!shell, 2000);
    const auto childExited = [child] {
        if (::kill(child, 0) != 0 && errno == ESRCH)
            return true;
#ifdef Q_OS_LINUX
        QFile status(QString("/proc/%1/status").arg(child));
        if (!status.open(QIODevice::ReadOnly))
            return true;
        // An orphan zombie awaits the host subreaper; it no longer runs.
        return status.readAll().contains("State:\tZ");
#else
        return false;
#endif
    };
    QTRY_VERIFY_WITH_TIMEOUT(childExited(), 2000);
#endif
}

void TerminalSessionTest::immediateDestructionReapsStartingShell()
{
    QPointer<QProcess> shell;
    {
        TerminalSession terminal;
        QVERIFY(terminal.start({}));
        shell = terminal.findChild<QProcess *>("terminalShell");
        QVERIFY(shell);
    }
    QTRY_VERIFY_WITH_TIMEOUT(!shell, 2000);
}

void TerminalSessionTest::cleanupTestCase()
{
    qputenv("SHELL", m_originalShell);
}

QTEST_GUILESS_MAIN(TerminalSessionTest)
#include "TerminalSessionTest.moc"
