#include "NativeTerminalLaunch.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QProcess>
#include <QProcessEnvironment>
#include <QTemporaryDir>
#include <QTest>

#ifndef DOCKPIPE_TERMINAL_HOST_TEST_BINARY
#define DOCKPIPE_TERMINAL_HOST_TEST_BINARY ""
#endif

namespace {
const QString schema = QStringLiteral("dockpipe.local-terminal/v1");

QJsonObject frame(const QString &type, int id)
{
    return {{QStringLiteral("schema"), schema}, {QStringLiteral("type"), type}, {QStringLiteral("id"), id}};
}

void writeFrame(QProcess &provider, const QJsonObject &object)
{
    provider.write(QJsonDocument(object).toJson(QJsonDocument::Compact) + '\n');
}

class Frames final {
public:
    QList<QJsonObject> read(QProcess &provider)
    {
        bytes += provider.readAllStandardOutput();
        QList<QJsonObject> result;
        while (true) {
            const qsizetype end = bytes.indexOf('\n');
            if (end < 0)
                return result;
            const QJsonDocument document = QJsonDocument::fromJson(bytes.left(end));
            bytes.remove(0, end + 1);
            result.append(document.object());
        }
    }
    QByteArray bytes;
};
}

class NativeTerminalProviderTest final : public QObject {
    Q_OBJECT
private slots:
    void frozenEnvironmentAndInvocationPath();
    void rejectsIncompleteAndUnknownProfiles();
    void actualWriteAndEofCleanup();
    void partialNativeWriteIsUncertain();
    void frozenEnvironmentIsConsumedExactly();
    void rejectsDuplicateAndUnknownFields();
};

void NativeTerminalProviderTest::frozenEnvironmentAndInvocationPath()
{
#ifdef Q_OS_UNIX
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString invokedPath = directory.filePath(QStringLiteral("invoked-shell"));
    QVERIFY(QFile::link(QStringLiteral("/bin/sh"), invokedPath));
    const auto snapshot = NativeTerminalLaunch::resolve(directory.path(), invokedPath,
        {QStringLiteral("-c"), QStringLiteral("exit 0")},
        {{QStringLiteral("DOCKPIPE_NATIVE_TEST"), QStringLiteral("frozen")}}, TerminalLaunchContext::Local);
    QVERIFY2(snapshot.error.isEmpty(), qPrintable(snapshot.error));
    QCOMPARE(snapshot.executable, invokedPath);
    QVERIFY(snapshot.environment.contains(QStringLiteral("TERM=xterm-256color")));
    QVERIFY(snapshot.environment.contains(QStringLiteral("COLORTERM=truecolor")));
    QVERIFY(snapshot.environment.contains(QStringLiteral("DOCKPIPE_NATIVE_TEST=frozen")));
    auto roundtrip = NativeTerminalLaunch::Snapshot{};
    QVERIFY(NativeTerminalLaunch::fromJson(snapshot.toJson(), roundtrip));
    QCOMPARE(roundtrip.environment, snapshot.environment);
    QCOMPARE(roundtrip.executable, invokedPath);
#else
    QSKIP("Unix executable symlink fixture.");
#endif
}

void NativeTerminalProviderTest::rejectsIncompleteAndUnknownProfiles()
{
    NativeTerminalLaunch::Snapshot result;
    QJsonObject profile = {{QStringLiteral("id"), QStringLiteral("fixture")},
        {QStringLiteral("executable"), QStringLiteral("/bin/sh")}, {QStringLiteral("dir"), QStringLiteral("/tmp")},
        {QStringLiteral("args"), QJsonArray{}}, {QStringLiteral("env"), QJsonArray{}},
        {QStringLiteral("route"), QStringLiteral("native")}, {QStringLiteral("environment_mode"), QStringLiteral("complete")}};
#ifdef Q_OS_UNIX
    QVERIFY(NativeTerminalLaunch::fromJson(profile, result));
#endif
    profile.insert(QStringLiteral("route"), QStringLiteral("host"));
    QVERIFY(!NativeTerminalLaunch::fromJson(profile, result));
    profile.insert(QStringLiteral("environment_mode"), QStringLiteral("inherit-host"));
    QVERIFY(!NativeTerminalLaunch::fromJson(profile, result));
    profile.insert(QStringLiteral("env"), QJsonArray{QStringLiteral("TERM=xterm-256color"), QStringLiteral("COLORTERM=truecolor")});
#ifdef Q_OS_UNIX
    QVERIFY(NativeTerminalLaunch::fromJson(profile, result));
#endif
    profile.insert(QStringLiteral("provider"), QStringLiteral("/untrusted/provider"));
    QVERIFY(!NativeTerminalLaunch::fromJson(profile, result));
}

void NativeTerminalProviderTest::actualWriteAndEofCleanup()
{
#ifndef Q_OS_UNIX
    QSKIP("Native PTY provider proof requires Unix; Windows has a separate ConPTY lane.");
#else
    const QString binary = QString::fromUtf8(DOCKPIPE_TERMINAL_HOST_TEST_BINARY);
    QVERIFY2(!binary.isEmpty(), "Set DOCKPIPE_TERMINAL_HOST_TEST_BINARY to the built QtCore helper.");
    QTemporaryDir directory;
    QProcess provider;
    provider.start(binary, {QStringLiteral("--stdio")});
    QVERIFY(provider.waitForStarted(3000));
    const auto launch = NativeTerminalLaunch::resolve(directory.path(), QStringLiteral("/bin/sh"),
        {QStringLiteral("-c"), QStringLiteral("stty -echo; printf READY; read value; printf 'RECEIVED:%s' \"$value\"; sleep 30")},
        {}, TerminalLaunchContext::Local);
    auto start = frame(QStringLiteral("start"), 1);
    start.insert(QStringLiteral("profile"), launch.toJson());
    start.insert(QStringLiteral("columns"), 80);
    start.insert(QStringLiteral("rows"), 24);
    writeFrame(provider, start);
    Frames decoder;
    QList<QJsonObject> received;
    QByteArray terminalOutput;
    const auto collect = [&] {
        const auto batch = decoder.read(provider);
        for (const auto &message : batch) {
            if (message.value(QStringLiteral("type")) == QStringLiteral("output"))
                terminalOutput += QByteArray::fromBase64(message.value(QStringLiteral("data")).toString().toLatin1());
            received.append(message);
        }
    };
    QTRY_VERIFY_WITH_TIMEOUT((collect(), terminalOutput.contains("READY")), 3000);
    auto input = frame(QStringLiteral("input"), 2);
    const QByteArray bytes = "write-receipt-fixture\n";
    input.insert(QStringLiteral("data"), QString::fromLatin1(bytes.toBase64()));
    writeFrame(provider, input);
    QTRY_VERIFY_WITH_TIMEOUT((collect(), terminalOutput.contains("RECEIVED:write-receipt-fixture")), 3000);
    bool acknowledged = false;
    for (const auto &message : received) {
        if (message.value(QStringLiteral("type")) == QStringLiteral("ack") && message.value(QStringLiteral("id")) == 2) {
            QCOMPARE(message.value(QStringLiteral("bytes")).toInt(), bytes.size());
            acknowledged = true;
        }
    }
    QVERIFY(acknowledged);
    provider.closeWriteChannel();
    QTRY_COMPARE_WITH_TIMEOUT(provider.state(), QProcess::NotRunning, 5000);
    collect();
    QVERIFY(!received.isEmpty());
    QCOMPARE(received.last().value(QStringLiteral("type")).toString(), QStringLiteral("closed"));
    QCOMPARE(received.last().value(QStringLiteral("cleanup")).toString(), QStringLiteral("complete"));
    QVERIFY2(provider.readAllStandardError().isEmpty(), "The helper must not emit raw diagnostics.");
#endif
}

void NativeTerminalProviderTest::partialNativeWriteIsUncertain()
{
#ifndef Q_OS_UNIX
    QSKIP("Unix PTY saturation fixture.");
#else
    QProcess provider;
    provider.start(QString::fromUtf8(DOCKPIPE_TERMINAL_HOST_TEST_BINARY), {QStringLiteral("--stdio")});
    QVERIFY(provider.waitForStarted(3000));
    const auto launch = NativeTerminalLaunch::resolve(QDir::tempPath(), QStringLiteral("/bin/sh"),
        {QStringLiteral("-c"), QStringLiteral("stty raw -echo; printf READY; sleep 30")},
        {}, TerminalLaunchContext::Local);
    auto start = frame(QStringLiteral("start"), 1);
    start.insert(QStringLiteral("profile"), launch.toJson());
    start.insert(QStringLiteral("columns"), 80);
    start.insert(QStringLiteral("rows"), 24);
    writeFrame(provider, start);
    Frames decoder;
    QList<QJsonObject> received;
    QByteArray output;
    const auto collect = [&] {
        for (const auto &message : decoder.read(provider)) {
            received.append(message);
            if (message.value(QStringLiteral("type")) == QStringLiteral("output"))
                output += QByteArray::fromBase64(message.value(QStringLiteral("data")).toString().toLatin1());
        }
    };
    QTRY_VERIFY_WITH_TIMEOUT((collect(), output.contains("READY")), 3000);
    auto input = frame(QStringLiteral("input"), 2);
    input.insert(QStringLiteral("data"), QString::fromLatin1(QByteArray(64 * 1024, 'x').toBase64()));
    writeFrame(provider, input);
    QTest::qWait(150);
    collect();
    for (const auto &message : received)
        QVERIFY(message.value(QStringLiteral("type")) != QStringLiteral("ack"));
    provider.closeWriteChannel();
    QTRY_COMPARE_WITH_TIMEOUT(provider.state(), QProcess::NotRunning, 5000);
    collect();
    QCOMPARE(received.last().value(QStringLiteral("type")).toString(), QStringLiteral("closed"));
    QCOMPARE(received.last().value(QStringLiteral("cleanup")).toString(), QStringLiteral("unknown"));
    bool uncertainInput = false;
    for (const auto &message : received) {
        QVERIFY(message.value(QStringLiteral("type")) != QStringLiteral("ack"));
        uncertainInput = uncertainInput || message.value(QStringLiteral("code")) == QStringLiteral("input_failed");
    }
    QVERIFY(uncertainInput);
#endif
}

void NativeTerminalProviderTest::frozenEnvironmentIsConsumedExactly()
{
#ifndef Q_OS_UNIX
    QSKIP("Unix exact-environment fixture.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString invokedPath = directory.filePath(QStringLiteral("frozen-shell"));
    QVERIFY(QFile::link(QStringLiteral("/bin/sh"), invokedPath));
    auto launch = NativeTerminalLaunch::resolve(directory.path(), invokedPath,
        {QStringLiteral("-c"), QStringLiteral("printf 'FROZEN:%s;LEAK:%s;ARGV0:%s' \"$FROZEN\" \"${AMBIENT_LEAK-unset}\" \"$0\"")},
        {}, TerminalLaunchContext::Local);
    launch.environment = {QStringLiteral("FROZEN=yes"), QStringLiteral("TERM=xterm-256color"), QStringLiteral("COLORTERM=truecolor")};
    QProcess provider;
    auto ambient = QProcessEnvironment::systemEnvironment();
    ambient.insert(QStringLiteral("FROZEN"), QStringLiteral("changed-after-binding"));
    ambient.insert(QStringLiteral("AMBIENT_LEAK"), QStringLiteral("must-not-inherit"));
    provider.setProcessEnvironment(ambient);
    provider.start(QString::fromUtf8(DOCKPIPE_TERMINAL_HOST_TEST_BINARY), {QStringLiteral("--stdio")});
    QVERIFY(provider.waitForStarted(3000));
    auto start = frame(QStringLiteral("start"), 1);
    start.insert(QStringLiteral("profile"), launch.toJson());
    start.insert(QStringLiteral("columns"), 80);
    start.insert(QStringLiteral("rows"), 24);
    writeFrame(provider, start);
    QTRY_COMPARE_WITH_TIMEOUT(provider.state(), QProcess::NotRunning, 5000);
    Frames decoder;
    QByteArray output;
    for (const auto &message : decoder.read(provider)) {
        if (message.value(QStringLiteral("type")) == QStringLiteral("output"))
            output += QByteArray::fromBase64(message.value(QStringLiteral("data")).toString().toLatin1());
    }
    QCOMPARE(output, QByteArray("FROZEN:yes;LEAK:unset;ARGV0:") + invokedPath.toUtf8());
#endif
}

void NativeTerminalProviderTest::rejectsDuplicateAndUnknownFields()
{
    const QString binary = QString::fromUtf8(DOCKPIPE_TERMINAL_HOST_TEST_BINARY);
    QVERIFY(!binary.isEmpty());
    const QList<QByteArray> invalid = {
        "{\"schema\":\"dockpipe.local-terminal/v1\",\"type\":\"start\",\"id\":1,\"id\":1}\n",
        "{\"schema\":\"dockpipe.local-terminal/v1\",\"type\":\"start\",\"id\":1,\"Provider\":\"/tmp/untrusted\"}\n",
        "{\"schema\":\"dockpipe.local-terminal/v2\",\"type\":\"start\",\"id\":1}\n"
    };
    for (const auto &request : invalid) {
        QProcess provider;
        provider.start(binary, {QStringLiteral("--stdio")});
        QVERIFY(provider.waitForStarted(3000));
        provider.write(request);
        provider.closeWriteChannel();
        QTRY_COMPARE_WITH_TIMEOUT(provider.state(), QProcess::NotRunning, 3000);
        Frames decoder;
        const auto messages = decoder.read(provider);
        QVERIFY(messages.size() >= 2);
        QCOMPARE(messages.first().value(QStringLiteral("type")).toString(), QStringLiteral("error"));
        QCOMPARE(messages.last().value(QStringLiteral("type")).toString(), QStringLiteral("closed"));
        QCOMPARE(messages.last().value(QStringLiteral("cleanup")).toString(), QStringLiteral("complete"));
    }
}

QTEST_GUILESS_MAIN(NativeTerminalProviderTest)
#include "NativeTerminalProviderTest.moc"
