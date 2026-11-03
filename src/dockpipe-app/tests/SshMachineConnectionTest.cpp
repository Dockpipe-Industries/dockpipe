#include "SshMachineConnection.h"
#include "SshMachineConnectionStore.h"
#include "SshMachineTransport.h"

#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QProcess>
#include <QTemporaryDir>
#include <QTest>

namespace {
SshMachineConnection exampleConnection()
{
    SshMachineConnection connection;
    connection.id = "office";
    connection.label = "Office host";
    connection.host = "192.0.2.7";
    connection.user = "operator";
    connection.nativeCliPath = "/opt/dockpipe/bin/dockpipe";
    return connection;
}

bool writePrivate(const QString &path, const QByteArray &contents)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner)
        && file.write(contents) == contents.size();
}
}

class SshMachineConnectionTest : public QObject {
    Q_OBJECT

private slots:
    void rejectsInvalidConnectionFields();
    void storeRoundTripAndPermissions();
    void storeRejectsUnknownFieldsAndPreservesDestination();
    void failedSavePreservesPreviousProfile();
    void storeRejectsSymlink();
    void plansUseExplicitConnectionAndAuthentication();
    void snapshotQuotesRemoteExecutable();
    void invalidTransportNeverFallsBack();
};

void SshMachineConnectionTest::rejectsInvalidConnectionFields()
{
    const auto valid = exampleConnection();
    QVERIFY(valid.validationError().isEmpty());
    auto loopback = valid;
    loopback.host = "::1";
    QVERIFY(loopback.validationError().isEmpty());
    auto invalid = valid;
    invalid.host = "-oProxyCommand=bad";
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.host += '\n';
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.user = "operator@other";
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.port = 65536;
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.nativeCliPath = "dockpipe";
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.nativeCliPath.append(QChar::Null);
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.identityFile = "relative-key";
    QVERIFY(!invalid.validationError().isEmpty());
    invalid = valid;
    invalid.label = QString(161, 'x');
    QVERIFY(!invalid.validationError().isEmpty());
}

void SshMachineConnectionTest::storeRoundTripAndPermissions()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString storePath = directory.filePath("private/connection.json");
    SshMachineConnectionStore store(storePath);
    QString error;
    const auto expected = exampleConnection();
    QVERIFY2(store.save(expected, &error), qPrintable(error));
    SshMachineConnection actual;
    QVERIFY2(store.load(&actual, &error), qPrintable(error));
    QCOMPARE(actual.id, expected.id);
    QCOMPARE(actual.host, expected.host);
    QCOMPARE(actual.nativeCliPath, expected.nativeCliPath);
#ifdef Q_OS_UNIX
    const auto publicBits = QFileDevice::ReadGroup | QFileDevice::WriteGroup | QFileDevice::ExeGroup
        | QFileDevice::ReadOther | QFileDevice::WriteOther | QFileDevice::ExeOther;
    QVERIFY(!(QFileInfo(storePath).permissions() & publicBits));
    QVERIFY(!(QFileInfo(QFileInfo(storePath).absolutePath()).permissions() & publicBits));
    QVERIFY(QFile::setPermissions(storePath, QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ReadGroup));
    QVERIFY(!store.load(&actual, &error));
#endif
}

void SshMachineConnectionTest::storeRejectsUnknownFieldsAndPreservesDestination()
{
    QTemporaryDir directory;
    SshMachineConnectionStore store(directory.filePath("connection.json"));
    QVERIFY(store.save(exampleConnection()));
    QFile file(store.path());
    QVERIFY(file.open(QIODevice::ReadOnly));
    auto document = QJsonDocument::fromJson(file.readAll());
    file.close();
    auto object = document.object();
    object.insert("password", "must-never-be-accepted");
    QVERIFY(writePrivate(store.path(), QJsonDocument(object).toJson()));
    auto destination = exampleConnection();
    destination.id = "unchanged";
    QString error;
    QVERIFY(!store.load(&destination, &error));
    QCOMPARE(destination.id, QString("unchanged"));
    QVERIFY(!error.isEmpty());
    QVERIFY(writePrivate(store.path(), QByteArray(32769, ' ')));
    QVERIFY(!store.load(&destination, &error));
    QCOMPARE(destination.id, QString("unchanged"));
}

void SshMachineConnectionTest::failedSavePreservesPreviousProfile()
{
    QTemporaryDir directory;
    SshMachineConnectionStore store(directory.filePath("connection.json"));
    const auto expected = exampleConnection();
    QVERIFY(store.save(expected));
    auto invalid = expected;
    invalid.host = "host; other-command";
    QVERIFY(!store.save(invalid));
    SshMachineConnection actual;
    QVERIFY(store.load(&actual));
    QCOMPARE(actual.host, expected.host);
    QVERIFY(QDir().mkdir(directory.filePath("destination")));
    SshMachineConnectionStore unwritable(directory.filePath("destination"));
    QVERIFY(!unwritable.save(expected));
    QVERIFY(QFileInfo(directory.filePath("destination")).isDir());
}

void SshMachineConnectionTest::storeRejectsSymlink()
{
#ifdef Q_OS_UNIX
    QTemporaryDir directory;
    SshMachineConnectionStore original(directory.filePath("original.json"));
    QVERIFY(original.save(exampleConnection()));
    const QString link = directory.filePath("link.json");
    QVERIFY(QFile::link(original.path(), link));
    SshMachineConnectionStore linked(link);
    SshMachineConnection actual;
    QVERIFY(!linked.load(&actual));
    QVERIFY(!linked.save(exampleConnection()));
    QVERIFY(original.load(&actual));
#else
    QSKIP("Unix symlink ownership test");
#endif
}

void SshMachineConnectionTest::plansUseExplicitConnectionAndAuthentication()
{
#if defined(Q_OS_LINUX) || defined(Q_OS_MACOS)
    QTemporaryDir directory("/tmp/dp-ssh-XXXXXX");
    auto connection = exampleConnection();
    connection.remoteDirectory = "/remote/operator/home";
    const auto interactive = SshMachineTransport::interactive(connection, directory.path());
    QVERIFY2(interactive.valid(), qPrintable(interactive.error));
    QVERIFY(interactive.arguments.contains("StrictHostKeyChecking=ask"));
    QVERIFY(interactive.arguments.contains("BatchMode=no"));
    QVERIFY(interactive.arguments.contains("-tt"));
    QCOMPARE(interactive.arguments.last(), connection.host);
    QCOMPARE(interactive.workingDirectory, QDir::homePath());
    QVERIFY(!interactive.arguments.contains(connection.remoteDirectory));
    const auto snapshot = SshMachineTransport::snapshot(connection, directory.path());
    QVERIFY2(snapshot.valid(), qPrintable(snapshot.error));
    QVERIFY(snapshot.arguments.contains("StrictHostKeyChecking=yes"));
    QVERIFY(snapshot.arguments.contains("BatchMode=yes"));
    QVERIFY(snapshot.arguments.contains("-n"));
    QCOMPARE(snapshot.arguments.last(), QString("'/opt/dockpipe/bin/dockpipe' machine snapshot --json"));
    const int separator = snapshot.arguments.indexOf("--");
    QVERIFY(separator > 0);
    QCOMPARE(snapshot.arguments.at(separator + 1), connection.host);
    const int control = snapshot.arguments.indexOf("-S");
    QCOMPARE(snapshot.arguments.at(control + 1), interactive.arguments.at(interactive.arguments.indexOf("-S") + 1));
    connection.user = "another";
    const auto other = SshMachineTransport::snapshot(connection, directory.path());
    QVERIFY(other.valid());
    QVERIFY(other.arguments.at(other.arguments.indexOf("-S") + 1) != snapshot.arguments.at(control + 1));
#else
    QTemporaryDir directory;
    QVERIFY(!SshMachineTransport::interactive(exampleConnection(), directory.path()).valid());
#endif
}

void SshMachineConnectionTest::snapshotQuotesRemoteExecutable()
{
#if defined(Q_OS_LINUX) || defined(Q_OS_MACOS)
    QTemporaryDir directory("/tmp/dp-ssh-XXXXXX");
    auto connection = exampleConnection();
    connection.nativeCliPath = directory.filePath("cli ' $(touch injected) ; file");
    QVERIFY(writePrivate(connection.nativeCliPath, "#!/bin/sh\nprintf '%s\\n' \"$@\"\n"));
    QVERIFY(QFile::setPermissions(connection.nativeCliPath,
        QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner));
    const auto plan = SshMachineTransport::snapshot(connection, directory.path());
    QVERIFY2(plan.valid(), qPrintable(plan.error));
    QProcess shell;
    shell.setWorkingDirectory(directory.path());
    shell.start("/bin/sh", {"-c", plan.arguments.last()});
    QVERIFY(shell.waitForFinished(3000));
    QCOMPARE(shell.exitCode(), 0);
    QCOMPARE(shell.readAllStandardOutput(), QByteArray("machine\nsnapshot\n--json\n"));
    QVERIFY(!QFileInfo::exists(directory.filePath("injected")));
#else
    QSKIP("Unix remote shell quoting test");
#endif
}

void SshMachineConnectionTest::invalidTransportNeverFallsBack()
{
    QTemporaryDir directory;
    auto connection = exampleConnection();
    connection.host = "-bad";
    auto plan = SshMachineTransport::interactive(connection, directory.path());
    QVERIFY(!plan.valid());
    QVERIFY(plan.program.isEmpty());
    QVERIFY(plan.arguments.isEmpty());
    connection = exampleConnection();
    plan = SshMachineTransport::snapshot(connection, directory.path(), "ssh");
    QVERIFY(!plan.valid());
    QVERIFY(plan.program.isEmpty());
    connection.identityFile = directory.filePath("missing-key");
    plan = SshMachineTransport::interactive(connection, directory.path());
    QVERIFY(!plan.valid());
    QVERIFY(plan.program.isEmpty());
    connection = exampleConnection();
    plan = SshMachineTransport::snapshot(connection, directory.filePath("%C"));
    QVERIFY(!plan.valid());
    QVERIFY(plan.program.isEmpty());
}

QTEST_GUILESS_MAIN(SshMachineConnectionTest)
#include "SshMachineConnectionTest.moc"
