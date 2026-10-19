#include "TerminalSession.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QJsonArray>
#include <QJsonDocument>
#include <QPointer>
#include <QProcess>
#include <QRegularExpression>
#include <QSignalSpy>
#include <QScopeGuard>
#include <QTemporaryDir>
#include <QFile>
#include <QTest>
#include <QTimer>

#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>

namespace {
HANDLE testDiagnosticHandle = nullptr;

// QtTest's stdout logger is not visible in every Windows console-subsystem
// configuration. Write bounded test milestones through the original captured
// stderr handle so a CTest timeout still identifies its active case and phase.
void writeDiagnostic(const QString &message)
{
    const QByteArray bytes = (QStringLiteral("ConPTY fixture: ") + message.left(2048) + QLatin1Char('\n')).toUtf8();
    DWORD written = 0;
    if (testDiagnosticHandle && testDiagnosticHandle != INVALID_HANDLE_VALUE)
        WriteFile(testDiagnosticHandle, bytes.constData(), DWORD(bytes.size()), &written, nullptr);
}

void reportSession(const TerminalSession &terminal, qsizetype outputBytes, int finishedCount)
{
    writeDiagnostic(QStringLiteral("state running=%1 output-bytes=%2 finished-count=%3 error=%4")
        .arg(terminal.running()).arg(outputBytes).arg(finishedCount).arg(terminal.error().left(512)));
}

bool writeChild(const QByteArray &bytes)
{
    DWORD count = 0;
    return WriteFile(GetStdHandle(STD_OUTPUT_HANDLE), bytes.constData(), DWORD(bytes.size()), &count, nullptr)
        && count == DWORD(bytes.size());
}

int terminalChild(const QStringList &arguments)
{
    if (arguments.contains(QStringLiteral("--linger"))) {
        Sleep(INFINITE);
        return 0;
    }
    DWORD mode = 0;
    if (!GetConsoleMode(GetStdHandle(STD_INPUT_HANDLE), &mode)
        || !GetConsoleMode(GetStdHandle(STD_OUTPUT_HANDLE), &mode)
        || !GetConsoleMode(GetStdHandle(STD_ERROR_HANDLE), &mode))
        return 101; // All three handles must belong to the actual ConPTY.
    SetConsoleCP(CP_UTF8);
    SetConsoleOutputCP(CP_UTF8);
    SetConsoleMode(GetStdHandle(STD_INPUT_HANDLE), ENABLE_PROCESSED_INPUT);
    QJsonArray encodedArguments;
    for (const QString &argument : arguments.mid(2))
        encodedArguments.append(argument);
    writeChild("ARGS:" + QJsonDocument(encodedArguments).toJson(QJsonDocument::Compact) + "\r\n");
    writeChild("DIRECTORY:" + QDir::currentPath().toUtf8() + "\r\n");
    writeChild("ENVIRONMENT:" + qgetenv("DOCKPIPE_TERMINAL_FIXTURE") + "\r\n");
    writeChild("READY\r\n");
    const auto size = [] {
        CONSOLE_SCREEN_BUFFER_INFO dimensions{};
        if (GetConsoleScreenBufferInfo(GetStdHandle(STD_OUTPUT_HANDLE), &dimensions))
            writeChild("SIZE:" + QByteArray::number(dimensions.dwSize.X) + ":" + QByteArray::number(dimensions.dwSize.Y) + "\r\n");
    };
    size();
    QByteArray command;
    char bytes[256];
    while (true) {
        DWORD count = 0;
        if (!ReadFile(GetStdHandle(STD_INPUT_HANDLE), bytes, DWORD(sizeof(bytes)), &count, nullptr) || !count)
            return 0;
        command.append(bytes, qsizetype(count));
        if (command.size() > 1024)
            return 102;
        if (!command.contains('\r') && !command.contains('\n'))
            continue;
        command = command.trimmed();
        if (command == "size") {
            size();
        } else if (command == "exit") {
            writeChild("FINAL_TAIL\r\n");
            return 7;
        } else if (command == "flood") {
            QByteArray flood(8192, ' ');
            quint32 pattern = 1;
            while (true) {
                // ConPTY serializes screen changes, not every write verbatim.
                // Changing printable content prevents an identical-screen
                // optimization from turning the flood into a no-output test.
                for (char &character : flood) {
                    pattern = pattern * 1664525u + 1013904223u;
                    character = char('!' + ((pattern >> 24) % 90));
                }
                if (!writeChild(flood))
                    return 0;
            }
        } else if (command == "descendant") {
            STARTUPINFOW startup{};
            startup.cb = sizeof(startup);
            PROCESS_INFORMATION child{};
            const QString executable = QDir::toNativeSeparators(QCoreApplication::applicationFilePath());
            std::wstring line = (QStringLiteral("\"") + QDir::toNativeSeparators(executable)
                + QStringLiteral("\" --terminal-child --linger")).toStdWString();
            if (!CreateProcessW(reinterpret_cast<LPCWSTR>(executable.utf16()), line.data(), nullptr, nullptr,
                    FALSE, 0, nullptr, nullptr, &startup, &child))
                return 103;
            writeChild("DESCENDANT:" + QByteArray::number(child.dwProcessId) + ";END\r\n");
            CloseHandle(child.hThread);
            CloseHandle(child.hProcess);
        }
        command.clear();
    }
}
}

class TerminalSessionWinTest final : public QObject {
    Q_OBJECT
private slots:
    void init();
    void cleanup();
    void inputResizeArgumentsAndExit();
    void commandProcessorShell();
    void invalidStartAndInputBounds();
    void asynchronousLaunchFailure();
    void redirectedParentHandlesStayOutOfPseudoConsole();
    void floodAndDescendantCleanup();
    void callbackRestartAndDestruction();

private:
    QElapsedTimer m_caseTimer;
};

void TerminalSessionWinTest::init()
{
    m_caseTimer.start();
    writeDiagnostic(QStringLiteral("BEGIN %1").arg(QString::fromLatin1(QTest::currentTestFunction())));
}

void TerminalSessionWinTest::cleanup()
{
    writeDiagnostic(QStringLiteral("END %1 failed=%2 elapsed-ms=%3")
        .arg(QString::fromLatin1(QTest::currentTestFunction()))
        .arg(QTest::currentTestFailed()).arg(m_caseTimer.elapsed()));
}

void TerminalSessionWinTest::inputResizeArgumentsAndExit()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalSession terminal;
    QByteArray output;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { output += bytes; });
    QSignalSpy finished(&terminal, &TerminalSession::finished);
    const auto sessionDiagnostics = qScopeGuard([&] {
        reportSession(terminal, output.size(), finished.size());
        if (!finished.isEmpty())
            writeDiagnostic(QStringLiteral("child exit code=%1").arg(finished.front().front().toInt()));
    });
    terminal.resize(117, 41);
    const QStringList special = {QStringLiteral(""), QStringLiteral("two words"), QStringLiteral("quote\"value"),
        QStringLiteral("trailing\\"), QStringLiteral("before\\\"quote"), QString::fromUtf8("Unicode-\xc3\xa9")};
    QStringList arguments{QStringLiteral("--terminal-child")};
    arguments.append(special);
    const QString environmentValue = QString::fromUtf8("space and Unicode-\xc3\xa9");
    writeDiagnostic(QStringLiteral("starting structured argv child"));
    QVERIFY(terminal.start(directory.path(), QCoreApplication::applicationFilePath(), arguments,
        {{QStringLiteral("DOCKPIPE_TERMINAL_FIXTURE"), environmentValue}}));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("READY"), 5000);
    writeDiagnostic(QStringLiteral("child READY received"));
    QJsonArray expected;
    for (const QString &argument : special)
        expected.append(argument);
    QVERIFY2(output.contains("ARGS:" + QJsonDocument(expected).toJson(QJsonDocument::Compact)), output.constData());
    QVERIFY(output.contains(QDir::fromNativeSeparators(directory.path()).toUtf8()));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("SIZE:117:41"), 5000);
    QVERIFY2(output.contains("ENVIRONMENT:" + environmentValue.toUtf8()), output.constData());
    writeDiagnostic(QStringLiteral("requesting resize"));
    terminal.resize(92, 29);
    // The worker serializes resize and close. Wait for the new dimensions to
    // arrive through the actual child console API, rather than inspecting state.
    QTimer sizeProbe;
    sizeProbe.setInterval(20);
    connect(&sizeProbe, &QTimer::timeout, this, [&] { terminal.send("size\r"); });
    sizeProbe.start();
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("SIZE:92:29"), 3000);
    sizeProbe.stop();
    writeDiagnostic(QStringLiteral("resize observed; requesting child exit"));
    QVERIFY(terminal.send("exit\r"));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 5000);
    QCOMPARE(finished.front().front().toInt(), 7);
    QVERIFY2(output.contains("FINAL_TAIL"), output.constData());
    QVERIFY(!terminal.running());
}

void TerminalSessionWinTest::commandProcessorShell()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalSession terminal;
    QByteArray output;
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) { output += bytes; });
    QSignalSpy finished(&terminal, &TerminalSession::finished);
    const auto sessionDiagnostics = qScopeGuard([&] { reportSession(terminal, output.size(), finished.size()); });
    writeDiagnostic(QStringLiteral("starting command processor"));
    QVERIFY(terminal.start(directory.path())); // Actual cmd.exe fallback.
    // The echoed input contains %PREFIX%, not CMD_READY. Only a running shell
    // can expand this marker, so terminal input echo cannot satisfy the oracle.
    QVERIFY(terminal.send("set PREFIX=CMD_\r"));
    QVERIFY(terminal.send("echo %PREFIX%READY\r"));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains("CMD_READY"), 5000);
    writeDiagnostic(QStringLiteral("command processor READY received"));
    QVERIFY(terminal.send("echo %CD%\r"));
    QTRY_VERIFY_WITH_TIMEOUT(output.contains(QDir::toNativeSeparators(directory.path()).toUtf8()), 5000);
    writeDiagnostic(QStringLiteral("command processor working directory observed; requesting exit"));
    QVERIFY(terminal.send("echo %PREFIX%TAIL\r"));
    QVERIFY(terminal.send("exit 11\r"));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 5000);
    QCOMPARE(finished.front().front().toInt(), 11);
    QVERIFY2(output.contains("CMD_TAIL"), output.constData());
}

void TerminalSessionWinTest::invalidStartAndInputBounds()
{
    TerminalSession terminal;
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    QVERIFY(!terminal.start(directory.path() + QStringLiteral("/missing")));
    QVERIFY(!terminal.start({}, QStringLiteral("relative-shell.exe")));
    QVERIFY(!terminal.send("no shell"));
    const QString executable = QCoreApplication::applicationFilePath();
    QVERIFY(!terminal.start({}, executable, {QString(32767, QLatin1Char('x'))}));
    QVERIFY(!terminal.start({}, executable, {QString(QChar::Null)}));
    QVERIFY(!terminal.start({}, executable, {}, {{QStringLiteral("bad=name"), QStringLiteral("value")}}));
    QVERIFY(!terminal.start({}, executable, {}, {{QStringLiteral("name"), QString(QChar::Null)}}));
    QVERIFY(terminal.start({}, executable, {QStringLiteral("--terminal-child")}));
    QVERIFY(!terminal.send(QByteArray(64 * 1024 + 1, 'x')));
    QVERIFY(terminal.error().contains(QStringLiteral("64 KiB")));
    terminal.stop();
    QVERIFY(!terminal.running());
}

void TerminalSessionWinTest::asynchronousLaunchFailure()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    QFile invalidProgram(directory.filePath(QStringLiteral("not-a-program.exe")));
    QVERIFY(invalidProgram.open(QIODevice::WriteOnly));
    QCOMPARE(invalidProgram.write("not a Windows executable"), qint64(24));
    invalidProgram.close();
    TerminalSession terminal;
    QSignalSpy finished(&terminal, &TerminalSession::finished);
    const auto sessionDiagnostics = qScopeGuard([&] { reportSession(terminal, 0, finished.size()); });
    writeDiagnostic(QStringLiteral("starting invalid executable asynchronously"));
    QVERIFY(terminal.start(directory.path(), invalidProgram.fileName(), {}));
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 5000);
    QVERIFY(!terminal.running());
    QVERIFY(!terminal.error().isEmpty());
    QCOMPARE(finished.front().front().toInt(), -1);
}

void TerminalSessionWinTest::redirectedParentHandlesStayOutOfPseudoConsole()
{
    // QProcess deliberately supplies pipe-backed stdio, even when this fixture
    // itself was started from an interactive console. The selected child cases
    // require real ConPTY input/output/error handles and launch-failure cleanup.
    QProcess redirected;
    redirected.setProcessChannelMode(QProcess::MergedChannels);
    QByteArray diagnostics;
    connect(&redirected, &QProcess::readyRead, this, [&] {
        const QByteArray bytes = redirected.readAll();
        if (diagnostics.size() + bytes.size() > 64 * 1024) {
            redirected.kill();
            return;
        }
        diagnostics.append(bytes);
    });
    const auto stopFixture = qScopeGuard([&] {
        if (redirected.state() != QProcess::NotRunning) {
            redirected.kill();
            redirected.waitForFinished(1000);
        }
    });
    QSignalSpy finished(&redirected, qOverload<int, QProcess::ExitStatus>(&QProcess::finished));
    writeDiagnostic(QStringLiteral("starting nested fixture with redirected standard handles"));
    redirected.start(QCoreApplication::applicationFilePath(),
        {QStringLiteral("inputResizeArgumentsAndExit"), QStringLiteral("asynchronousLaunchFailure")});
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 1, 15000);
    diagnostics.append(redirected.readAll().left(64 * 1024 - diagnostics.size()));
    QCOMPARE(redirected.exitStatus(), QProcess::NormalExit);
    QVERIFY2(redirected.exitCode() == 0, diagnostics.constData());
}

void TerminalSessionWinTest::floodAndDescendantCleanup()
{
    TerminalSession terminal;
    QByteArray prefix;
    qint64 total = 0;
    qsizetype largest = 0;
    int ticks = 0;
    QElapsedTimer elapsed;
    elapsed.start();
    const auto diagnostics = qScopeGuard([&] {
        writeDiagnostic(QStringLiteral("flood output-bytes=%1 largest-batch=%2 elapsed-ms=%3 heartbeat-ticks=%4 error=%5")
            .arg(total).arg(largest).arg(elapsed.elapsed()).arg(ticks).arg(terminal.error().left(512)));
    });
    connect(&terminal, &TerminalSession::output, this, [&](const QByteArray &bytes) {
        total += bytes.size();
        largest = qMax(largest, bytes.size());
        if (prefix.size() < 8192)
            prefix.append(bytes.left(8192 - prefix.size()));
    });
    terminal.resize(160, 60);
    QVERIFY(terminal.start({}, QCoreApplication::applicationFilePath(), {QStringLiteral("--terminal-child")}));
    QTRY_VERIFY_WITH_TIMEOUT(prefix.contains("READY"), 5000);
    writeDiagnostic(QStringLiteral("flood child READY received; requesting descendant"));
    QVERIFY(terminal.send("descendant\r"));
    const QRegularExpression descendantPattern(QStringLiteral("DESCENDANT:(\\d+);END"));
    QTRY_VERIFY_WITH_TIMEOUT(descendantPattern.match(QString::fromUtf8(prefix)).hasMatch(), 5000);
    const auto match = descendantPattern.match(QString::fromUtf8(prefix));
    QVERIFY(match.hasMatch());
    const HANDLE descendant = OpenProcess(SYNCHRONIZE, FALSE, match.captured(1).toULong());
    QVERIFY(descendant != nullptr);
    const auto closeDescendant = qScopeGuard([&] { CloseHandle(descendant); });
    writeDiagnostic(QStringLiteral("descendant handle opened; requesting flood"));
    QVERIFY(terminal.send("flood\r"));
    QTRY_VERIFY_WITH_TIMEOUT(total >= 256 * 1024, 3000);
    writeDiagnostic(QStringLiteral("flood threshold reached"));
    QTimer heartbeat;
    heartbeat.setInterval(10);
    connect(&heartbeat, &QTimer::timeout, this, [&] { ++ticks; });
    heartbeat.start();
    QEventLoop loop;
    QTimer::singleShot(350, &loop, &QEventLoop::quit);
    loop.exec();
    QVERIFY(ticks >= 10);
    QVERIFY(largest <= 64 * 1024);
    QElapsedTimer stopTime;
    stopTime.start();
    writeDiagnostic(QStringLiteral("stopping flood session"));
    terminal.stop();
    QVERIFY(stopTime.elapsed() < 200);
    QVERIFY(!terminal.running());
    QTRY_COMPARE_WITH_TIMEOUT(WaitForSingleObject(descendant, 0), DWORD(WAIT_OBJECT_0), 5000);
}

void TerminalSessionWinTest::callbackRestartAndDestruction()
{
    auto *terminal = new TerminalSession(this);
    QPointer<TerminalSession> guarded = terminal;
    bool restarted = false;
    bool destroyed = false;
    QByteArray readyOutput;
    connect(terminal, &TerminalSession::output, this, [&, terminal](const QByteArray &bytes) {
        readyOutput.append(bytes.left(4096 - readyOutput.size()));
        if (!readyOutput.contains("READY"))
            return;
        readyOutput.clear();
        if (!restarted) {
            writeDiagnostic(QStringLiteral("callback stopping and restarting child"));
            restarted = true;
            terminal->stop();
            QVERIFY(terminal->start({}, QCoreApplication::applicationFilePath(), {QStringLiteral("--terminal-child")}));
        } else {
            writeDiagnostic(QStringLiteral("callback deleting restarted terminal"));
            delete terminal;
            destroyed = true;
        }
    });
    QVERIFY(terminal->start({}, QCoreApplication::applicationFilePath(), {QStringLiteral("--terminal-child")}));
    QTRY_VERIFY_WITH_TIMEOUT(destroyed, 10000);
    QVERIFY(restarted);
    QVERIFY(guarded.isNull());
    // Destruction while startup is in flight must also remain nonblocking.
    QElapsedTimer elapsed;
    elapsed.start();
    auto *starting = new TerminalSession;
    QVERIFY(starting->start({}, QCoreApplication::applicationFilePath(), {QStringLiteral("--terminal-child")}));
    delete starting;
    QVERIFY(elapsed.elapsed() < 200);
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == QStringLiteral("--terminal-child"))
        return terminalChild(application.arguments());
    const HANDLE currentProcess = GetCurrentProcess();
    const bool ownsDiagnosticHandle = DuplicateHandle(currentProcess, GetStdHandle(STD_ERROR_HANDLE), currentProcess,
        &testDiagnosticHandle, 0, FALSE, DUPLICATE_SAME_ACCESS);
    if (!ownsDiagnosticHandle)
        testDiagnosticHandle = GetStdHandle(STD_ERROR_HANDLE);
    const auto closeDiagnostics = qScopeGuard([ownsDiagnosticHandle] {
        if (ownsDiagnosticHandle)
            CloseHandle(testDiagnosticHandle);
    });
    writeDiagnostic(QStringLiteral("test process started"));
    TerminalSessionWinTest tests;
    const int result = QTest::qExec(&tests, argc, argv);
    writeDiagnostic(QStringLiteral("test process finished result=%1").arg(result));
    return result;
}

#include "TerminalSessionWinTest.moc"
