#include "RemoteCommand.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <cstdio>
#include <csignal>
#ifdef Q_OS_WIN
#include <fcntl.h>
#include <io.h>
#endif

namespace {
constexpr qsizetype MaximumOutputBytes = 20 * 1024 * 1024;

struct Outcome {
    bool completed = false;
    bool success = false;
    int completions = 0;
    QByteArray result;
    QByteArray streamed;
    QString diagnostics;
};

bool check(bool condition, const char *description)
{
    if (!condition)
        std::fprintf(stderr, "RemoteCommand budget test failed: %s\n", description);
    return condition;
}

int fixture(const QStringList &arguments)
{
#ifdef Q_OS_WIN
    if (_setmode(_fileno(stdout), _O_BINARY) == -1
        || _setmode(_fileno(stderr), _O_BINARY) == -1)
        return 10;
#endif
    const QString mode = arguments.value(2);
    if (mode == "machine-json") {
        std::fputs("{\"processes\":[{\"name\":\"hardware-key-worker\"}]}\n", stdout);
        std::fflush(stdout);
        return 0;
    }
    if (mode == "marker") {
        QFile marker(arguments.value(3));
        return marker.open(QIODevice::WriteOnly) ? 0 : 11;
    }
    if (mode == "bytes") {
        const QByteArray output(arguments.value(3).toInt(), 'o');
        const QByteArray error(arguments.value(4).toInt(), 'e');
        if (std::fwrite(output.constData(), 1, size_t(output.size()), stdout) != size_t(output.size()))
            return 12;
        std::fflush(stdout);
        if (std::fwrite(error.constData(), 1, size_t(error.size()), stderr) != size_t(error.size()))
            return 13;
        std::fflush(stderr);
        return 0;
    }
    if (mode == "cancel-lines") {
        std::fputs("first\nsecond\n", stdout);
        std::fflush(stdout);
        QThread::msleep(300);
        return 0;
    }
#ifdef Q_OS_UNIX
    if (mode == "ignore-termination") {
        std::signal(SIGTERM, SIG_IGN);
        std::fputs("ready\n", stdout);
        std::fflush(stdout);
        QThread::msleep(arguments.value(3).toULong());
        return 0;
    }
#endif
    if (mode == "sleep") {
        QThread::msleep(1000);
        return 0;
    }
    return 14;
}

Outcome invoke(RemoteCommand &command, const QStringList &arguments, qsizetype maximum,
               bool cancelOnOutput = false, int timeoutMs = 3000)
{
    Outcome outcome;
    QEventLoop loop;
    const auto standardOutput = QObject::connect(&command, &RemoteCommand::standardOutput, &loop,
        [&](const QByteArray &bytes) {
            outcome.streamed += bytes;
            if (cancelOnOutput)
                command.cancel();
        });
    const auto diagnostics = QObject::connect(&command, &RemoteCommand::output, &loop,
        [&](const QString &text) { outcome.diagnostics += text; });
    command.runBounded(QCoreApplication::applicationFilePath(), arguments,
        [&](bool success, const QByteArray &data) {
            outcome.completed = true;
            outcome.success = success;
            ++outcome.completions;
            outcome.result = data;
            loop.quit();
        }, timeoutMs, maximum);
    QTimer::singleShot(6000, &loop, &QEventLoop::quit);
    if (!outcome.completed)
        loop.exec();
    QObject::disconnect(standardOutput);
    QObject::disconnect(diagnostics);
    return outcome;
}

bool aggregateBudget()
{
    RemoteCommand command;
    const auto exact = invoke(command, {"--fixture", "bytes", "16", "16"}, 32);
    if (!check(exact.completed && exact.success && exact.completions == 1,
               "exact aggregate stdout/stderr boundary should succeed")
        || !check(exact.result == QByteArray(16, 'o'), "unterminated stdout should be returned"))
        return false;
    const auto overflow = invoke(command, {"--fixture", "bytes", "16", "17"}, 32);
    if (!check(overflow.completed && !overflow.success && overflow.completions == 1,
               "individually valid streams exceeding aggregate boundary should fail")
        || !check(overflow.diagnostics.contains("command response limit"), "aggregate rejection should be visible"))
        return false;
    const auto stderrOnly = invoke(command, {"--fixture", "bytes", "0", "33"}, 32);
    if (!check(stderrOnly.completed && !stderrOnly.success, "stderr-only overflow should fail"))
        return false;
    const auto stdoutOnly = invoke(command, {"--fixture", "bytes", "33", "0"}, 32);
    return check(stdoutOnly.completed && !stdoutOnly.success, "stdout-only overflow should fail")
        && check(!command.busy(), "finished command should not remain busy");
}

bool ordinaryMachineJsonIsNotAnInteractionFrame()
{
    RemoteCommand command;
    const auto outcome = invoke(command, {"--fixture", "machine-json"}, 256);
    return check(outcome.completed && outcome.success && outcome.completions == 1,
                 "ordinary machine JSON must not become a reserved interaction frame")
        && check(outcome.result.contains("hardware-key-worker"), "process names must remain intact");
}

bool cancelledOutputStopsImmediately()
{
    RemoteCommand command;
    const auto cancelled = invoke(command, {"--fixture", "cancel-lines"}, 64, true);
    return check(cancelled.completed && !cancelled.success && cancelled.completions == 1,
                 "cancel should fail exactly once")
        && check(cancelled.streamed == "first\n", "cancelled buffered lines should not be emitted")
        && check(!cancelled.diagnostics.contains("second"), "cancelled lines should not enter diagnostics");
}

bool invocationStateResets()
{
    RemoteCommand command;
    const auto overflow = invoke(command, {"--fixture", "bytes", "16", "17"}, 32);
    const auto next = invoke(command, {"--fixture", "bytes", "8", "8"}, 16);
    const auto larger = invoke(command, {"--fixture", "bytes", "32", "32"}, 64);
    return check(overflow.completed && !overflow.success, "setup invocation should exceed its budget")
        && check(next.completed && next.success && next.result == QByteArray(8, 'o'),
                 "next invocation should reset cancellation, stdout and aggregate counters")
        && check(larger.completed && larger.success && larger.result == QByteArray(32, 'o'),
                 "per-invocation limit should be replaced");
}

bool invalidLimitDoesNotSpawn()
{
    QTemporaryDir directory;
    if (!check(directory.isValid(), "marker fixture directory should exist"))
        return false;
    RemoteCommand command;
    int busyTransitions = 0;
    QObject::connect(&command, &RemoteCommand::busyChanged, &command,
                     [&](bool) { ++busyTransitions; });
    for (const qsizetype maximum : {qsizetype(-1), qsizetype(0), MaximumOutputBytes + 1}) {
        const QString marker = directory.filePath(QString::number(maximum));
        const auto outcome = invoke(command, {"--fixture", "marker", marker}, maximum);
        if (!check(outcome.completed && !outcome.success && outcome.completions == 1,
                   "invalid limit should fail synchronously once")
            || !check(!QFile::exists(marker), "invalid limit must not spawn the marker process"))
            return false;
    }
    QEventLoop settle;
    QTimer::singleShot(100, &settle, &QEventLoop::quit);
    settle.exec();
    return check(busyTransitions == 0 && !command.busy(), "invalid limits must not enter busy state")
        && check(QDir(directory.path()).entryList(QDir::Files).isEmpty(), "no delayed invalid invocation should spawn");
}

bool timeoutFails()
{
    RemoteCommand command;
    const auto outcome = invoke(command, {"--fixture", "sleep"}, 32, false, 30);
    return check(outcome.completed && !outcome.success && outcome.completions == 1,
                 "timeout should fail exactly once")
        && check(outcome.diagnostics.contains("timed out"), "timeout failure should be visible");
}

bool previousCancellationCannotKillLaterInvocation()
{
#ifdef Q_OS_UNIX
    RemoteCommand command;
    const auto first = invoke(command, {"--fixture", "ignore-termination", "150"}, 64, true);
    if (!check(first.completed && !first.success, "first cancelled child should finish during its grace period"))
        return false;
    QEventLoop loop;
    QElapsedTimer cancellationAge;
    bool completed = false;
    bool success = true;
    const auto ready = QObject::connect(&command, &RemoteCommand::standardOutput, &loop,
        [&](const QByteArray &) {
            QTimer::singleShot(1100, &command, [&]() {
                cancellationAge.start();
                command.cancel();
            });
        });
    command.runBounded(QCoreApplication::applicationFilePath(),
        {"--fixture", "ignore-termination", "5000"}, [&](bool ok, const QByteArray &) {
            completed = true;
            success = ok;
            loop.quit();
        }, 6000, 64);
    QTimer::singleShot(6000, &loop, &QEventLoop::quit);
    loop.exec();
    QObject::disconnect(ready);
    return check(completed && !success && cancellationAge.isValid(), "second cancelled invocation should finish")
        && check(cancellationAge.elapsed() >= 1700,
                 "a previous invocation's delayed kill must not shorten the new cancellation grace period");
#else
    return true;
#endif
}
}

int main(int argc, char **argv)
{
    // The byte fixture must not include Qt's locale/startup diagnostics in its
    // measured stderr. Its output is deliberately smaller than those messages.
    if (argc > 1 && QByteArray(argv[1]) == "--fixture") {
        QStringList arguments;
        for (int index = 0; index < argc; ++index)
            arguments.append(QString::fromUtf8(argv[index]));
        return fixture(arguments);
    }
    QCoreApplication application(argc, argv);
    return aggregateBudget()
        && ordinaryMachineJsonIsNotAnInteractionFrame()
        && cancelledOutputStopsImmediately()
        && invocationStateResets()
        && invalidLimitDoesNotSpawn()
        && timeoutFails()
        && previousCancellationCannotKillLaterInvocation() ? 0 : 1;
}
