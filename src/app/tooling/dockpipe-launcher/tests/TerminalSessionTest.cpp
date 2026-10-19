#include "TerminalSession.h"

#include <QElapsedTimer>
#include <QFile>
#include <QPointer>
#include <QProcess>
#include <QRegularExpression>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>

#ifdef Q_OS_UNIX
#include <cerrno>
#include <signal.h>
#include <unistd.h>
#endif

class TerminalSessionTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void terminalInputResizeAndExit();
    void rejectsInvalidInputAndDirectory();
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
    QSKIP("Unix PTY fixture; Windows has an explicit unsupported backend.");
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

void TerminalSessionTest::outputFloodDoesNotBlockEventLoop()
{
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
    QVERIFY(terminal.start({}));
    QVERIFY(terminal.send("stty -echo; yes terminal-output-fixture\n"));
    QTRY_VERIFY_WITH_TIMEOUT(total >= 256 * 1024, 3000);
    ticks = 0;
    QTest::qWait(350);
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
    QTRY_VERIFY_WITH_TIMEOUT(restarted && nextSessionOutput.contains("RESTART_ready"), 3000);
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
    connect(terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) {
        total += bytes.size();
        if (total >= 256 * 1024)
            delete terminal.data();
    });
    QVERIFY(terminal->start({}));
    QVERIFY(terminal->send("stty -echo; yes destroyed-session-output-fixture\n"));
    QTRY_VERIFY_WITH_TIMEOUT(!terminal, 3000);
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
