#include "NativeHostConnection.h"
#include "NativeHostSnapshot.h"
#include "NativeHostDockerAction.h"
#include "NativeHostDockerQuery.h"
#include "MonitoringMachineSelector.h"
#include "TerminalHub.h"

#include <QApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QSaveFile>
#include <QScopeGuard>
#include <QSet>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>

namespace {
QJsonObject connectedStatus()
{
    return {{"schema", "dockpipe.native_host_status.v1"}, {"available", true}, {"connected", true},
        {"node", "host.example"}, {"username", "operator"}, {"uid", 1026}, {"gid", 100},
        {"groups", QJsonArray{100, 101}}, {"generation", "generation-one"}, {"home", "/host/operator"}};
}

bool writeFile(const QString &path, const QByteArray &data)
{
    QSaveFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(data) == data.size() && file.commit();
}

int hostFixture(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QStringList arguments = application.arguments();
    QFile output;
    if (!output.open(stdout, QIODevice::WriteOnly))
        return 90;
    if (arguments.value(2) == "status") {
        QFile status(qEnvironmentVariable("DOCKPIPE_TEST_NATIVE_STATUS"));
        if (!status.open(QIODevice::ReadOnly))
            return 91;
        const QByteArray contents = QJsonDocument::fromJson(status.readAll()).toJson(QJsonDocument::Compact) + '\n';
        if (output.write(contents) != contents.size() || !output.flush())
            return 92;
        return QJsonDocument::fromJson(contents).object().value("available").toBool() ? 0 : 1;
    }
    if (arguments.value(2) == "terminal") {
        const QJsonObject invocation{{"arguments", QJsonArray::fromStringList(arguments.mid(1))},
            {"cwd", QDir::currentPath()}};
        if (!writeFile(qEnvironmentVariable("DOCKPIPE_TEST_NATIVE_TERMINAL"), QJsonDocument(invocation).toJson()))
            return 93;
        output.write("native-terminal-fixture\n");
        output.flush();
        QTimer::singleShot(15000, &application, &QCoreApplication::quit);
        return application.exec();
    }
    return 94;
}
}

class NativeHostConnectionTest : public QObject {
    Q_OBJECT
private slots:
    void statusRequiresActualConnectedIdentity();
    void snapshotsRequireCurrentHostAndBoundedRows();
    void referencesAreOpaqueAndNeverBecomeCommands();
    void configuredUnavailableHostNeverFallsBack();
    void nativeTerminalUsesWorkerCwdAndClearsOnRevocation();
    void selectorRetainsNativeChoiceAcrossPairedRefresh();
    void dockerActionsRejectCommandsAndMismatchedReceipts();
    void dockerQueriesRequireTypedBoundedMatchingResults();
};

void NativeHostConnectionTest::statusRequiresActualConnectedIdentity()
{
    NativeHostStatus status;
    QVERIFY(NativeHostStatus::decode(QJsonDocument(connectedStatus()).toJson(), &status));
    QVERIFY(status.available);
    QCOMPARE(status.username, QString("operator"));
    QCOMPARE(status.uid, quint32(1026));
    auto withoutGroups = connectedStatus();
    withoutGroups.remove("groups");
    QVERIFY(NativeHostStatus::decode(QJsonDocument(withoutGroups).toJson(), &status));
    for (const QString &field : {QStringLiteral("schema"), QStringLiteral("node"), QStringLiteral("username")}) {
        auto invalid = connectedStatus();
        invalid.remove(field);
        QVERIFY(!NativeHostStatus::decode(QJsonDocument(invalid).toJson(), &status));
    }
    auto invalid = connectedStatus();
    invalid.insert("uid", 0);
    QVERIFY(!NativeHostStatus::decode(QJsonDocument(invalid).toJson(), &status));
    invalid.insert("uid", 1026.5);
    QVERIFY(!NativeHostStatus::decode(QJsonDocument(invalid).toJson(), &status));
    invalid = connectedStatus();
    invalid.insert("connected", false);
    QVERIFY(!NativeHostStatus::decode(QJsonDocument(invalid).toJson(), &status));
    invalid = connectedStatus();
    invalid.insert("groups", QJsonArray{-1});
    QVERIFY(!NativeHostStatus::decode(QJsonDocument(invalid).toJson(), &status));
    QVERIFY(!NativeHostStatus::decode(QByteArray(1024 * 1024 + 1, ' '), &status));
    const QJsonObject unavailable{{"schema", "dockpipe.native_host_status.v1"}, {"available", false}, {"connected", false}};
    QVERIFY(NativeHostStatus::decode(QJsonDocument(unavailable).toJson(), &status));
    QVERIFY(!status.available);
    QVERIFY(status.username.isEmpty());
}

void NativeHostConnectionTest::snapshotsRequireCurrentHostAndBoundedRows()
{
    QJsonObject execution{{"schema", "dockpipe.execution_snapshot.v1"}, {"processes", QJsonArray{}}, {"executions", QJsonArray{}}};
    QJsonObject envelope{{"schema", "dockpipe.machine_snapshot.v1"}, {"node", "host.example"},
        {"state", "live"}, {"execution", execution}};
    QJsonObject result;
    QVERIFY(NativeHostSnapshot::decode(QJsonDocument(envelope).toJson(), "host.example", "execution", &result));
    QVERIFY(!NativeHostSnapshot::decode(QJsonDocument(envelope).toJson(), "another-host", "execution", &result));
    QJsonArray processes;
    for (int index = 0; index < 4097; ++index)
        processes.append(QJsonObject{{"pid", index + 1}});
    execution.insert("processes", processes);
    envelope.insert("execution", execution);
    QVERIFY(!NativeHostSnapshot::decode(QJsonDocument(envelope).toJson(), "host.example", "execution", &result));
    processes.removeLast();
    execution.insert("processes", processes);
    envelope.insert("execution", execution);
    QVERIFY(NativeHostSnapshot::decode(QJsonDocument(envelope).toJson(), "host.example", "execution", &result));
    envelope.insert("state", "stale");
    QVERIFY(!NativeHostSnapshot::decode(QJsonDocument(envelope).toJson(), "host.example", "execution", &result));
}

void NativeHostConnectionTest::referencesAreOpaqueAndNeverBecomeCommands()
{
    QTemporaryDir temporary;
    const QString reference = temporary.filePath("reference ' ; literal.json");
    QVERIFY(writeFile(reference, "not GUI-readable connection metadata"));
    const bool hadBinary = qEnvironmentVariableIsSet("DOCKPIPE_BIN");
    const QByteArray previousBinary = qgetenv("DOCKPIPE_BIN");
    const auto restore = qScopeGuard([hadBinary, previousBinary] {
        if (hadBinary)
            qputenv("DOCKPIPE_BIN", previousBinary);
        else
            qunsetenv("DOCKPIPE_BIN");
    });
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    NativeHostConnection connection(reference);
    const auto terminal = connection.plan(NativeHostConnection::Operation::Terminal);
    QVERIFY2(terminal.valid(), qPrintable(terminal.error));
    QCOMPARE(terminal.program, QFileInfo(QCoreApplication::applicationFilePath()).canonicalFilePath());
    QCOMPARE(terminal.arguments, QStringList({"host", "terminal", "--connection", reference}));
    QCOMPARE(terminal.workingDirectory, QDir::homePath());
    QVERIFY(!terminal.arguments.contains("--cwd"));
    const auto snapshot = connection.plan(NativeHostConnection::Operation::Snapshot);
    QCOMPARE(snapshot.arguments, QStringList({"host", "snapshot", "--connection", reference, "--json"}));
    const QString container(64, 'a');
    const auto stop = connection.dockerPlan("stop", container);
    QVERIFY(stop.valid());
    QCOMPARE(stop.arguments, QStringList({"host", "docker", "stop", "--connection", reference, "--id", container, "--json"}));
    const auto query = connection.dockerQueryPlan("container", container);
    QVERIFY(query.valid());
    QCOMPARE(query.arguments, QStringList({"host", "docker-query", "--connection", reference,
        "--kind", "container", "--id", container, "--json"}));
    QVERIFY(connection.dockerQueryPlan("volume", "app_data-1").valid());
    QVERIFY(!connection.dockerQueryPlan("exec", container).valid());
    QVERIFY(!connection.dockerQueryPlan("volume", "name; command").valid());
    QVERIFY(!connection.dockerPlan("exec", container).valid());
    QVERIFY(!connection.dockerPlan("stop", "container; command").valid());
    NativeHostConnection relative("relative.json");
    QVERIFY(!relative.plan(NativeHostConnection::Operation::Status).valid());
}

void NativeHostConnectionTest::configuredUnavailableHostNeverFallsBack()
{
    QTemporaryDir temporary;
    NativeHostConnection connection(temporary.filePath("missing.json"));
    TerminalHub hub(TerminalLaunchContext::Local);
    hub.setNativeHostConnection(&connection);
    QVERIFY(hub.nativeHostConfigured());
    QCOMPARE(hub.shellProfiles().first().id, NativeHostConnection::selectionKey());
    QVERIFY(hub.createSession(QDir::homePath()).isEmpty());
    QVERIFY(!hub.creationError().isEmpty());
    QVERIFY(hub.sessionIds().isEmpty());
    const auto local = hub.shellCatalog().profiles();
    QVERIFY(!local.isEmpty());
    bool foundLocal = false;
    for (const auto &profile : hub.shellProfiles()) {
        if (profile.id == local.first().id)
            foundLocal = profile.label.startsWith("Application container");
    }
    QVERIFY(foundLocal);
}

void NativeHostConnectionTest::nativeTerminalUsesWorkerCwdAndClearsOnRevocation()
{
#ifdef Q_OS_UNIX
    QTemporaryDir temporary;
    const QString reference = temporary.filePath("connection.json");
    const QString statusFile = temporary.filePath("status.json");
    const QString invocation = temporary.filePath("terminal.json");
    QVERIFY(writeFile(reference, "opaque"));
    auto response = connectedStatus();
    response.insert("username", "hardware-key-operator");
    QVERIFY(writeFile(statusFile, QJsonDocument(response).toJson()));
    const QStringList names{"DOCKPIPE_BIN", "DOCKPIPE_TEST_NATIVE_STATUS", "DOCKPIPE_TEST_NATIVE_TERMINAL"};
    QMap<QString, QByteArray> previous;
    QSet<QString> present;
    for (const auto &name : names) {
        previous.insert(name, qgetenv(name.toUtf8().constData()));
        if (qEnvironmentVariableIsSet(name.toUtf8().constData()))
            present.insert(name);
    }
    const auto restore = qScopeGuard([names, previous, present] {
        for (const auto &name : names) {
            if (present.contains(name))
                qputenv(name.toUtf8().constData(), previous.value(name));
            else
                qunsetenv(name.toUtf8().constData());
        }
    });
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    qputenv("DOCKPIPE_TEST_NATIVE_STATUS", statusFile.toUtf8());
    qputenv("DOCKPIPE_TEST_NATIVE_TERMINAL", invocation.toUtf8());
    NativeHostConnection connection(reference);
    TerminalHub hub(TerminalLaunchContext::Local);
    hub.setNativeHostConnection(&connection);
    connection.refresh();
    QTRY_VERIFY_WITH_TIMEOUT(connection.available(), 3000);
    const QString session = hub.createSession("/container-only/nonexistent/workdir");
    QVERIFY2(!session.isEmpty(), qPrintable(hub.creationError()));
    QTRY_VERIFY_WITH_TIMEOUT(QFileInfo::exists(invocation), 3000);
    QFile file(invocation);
    QVERIFY(file.open(QIODevice::ReadOnly));
    const auto launch = QJsonDocument::fromJson(file.readAll()).object();
    QCOMPARE(launch.value("cwd").toString(), QDir::homePath());
    QCOMPARE(launch.value("arguments").toArray(), QJsonArray({"host", "terminal", "--connection", reference}));
    QVERIFY(hub.title(session).contains("host.example"));
    QVERIFY(hub.title(session).contains("operator"));
    quint64 cursor = 0;
    bool truncated = false;
    QByteArray received;
    const auto hasOutput = [&] {
        received += hub.readOutput(session, cursor, truncated);
        return received.contains("native-terminal-fixture");
    };
    QTRY_VERIFY_WITH_TIMEOUT(hasOutput(), 3000);
    const QJsonObject unavailable{{"schema", "dockpipe.native_host_status.v1"}, {"available", false}, {"connected", false}};
    QVERIFY(writeFile(statusFile, QJsonDocument(unavailable).toJson()));
    connection.refresh();
    QTRY_VERIFY_WITH_TIMEOUT(!connection.available(), 3000);
    QVERIFY(!hub.sessionIds().contains(session));
    QVERIFY(hub.readOutput(session, cursor, truncated).isEmpty());
    QVERIFY(hub.createSession(QDir::homePath()).isEmpty());
#else
    QSKIP("Native PTY bridge fixture is qualified on Unix");
#endif
}

void NativeHostConnectionTest::selectorRetainsNativeChoiceAcrossPairedRefresh()
{
    MonitoringMachineSelector selector;
    selector.setNativeHost("Native host — unavailable", true);
    selector.setNodes({"paired-one"});
    QCOMPARE(selector.currentData().toString(), NativeHostConnection::selectionKey());
    QCOMPARE(selector.itemText(0), QString("Application container"));
    selector.setNativeHost("host.example · operator", false);
    QCOMPARE(selector.currentData().toString(), NativeHostConnection::selectionKey());
    selector.setCurrentIndex(selector.findData("paired-one"));
    selector.setNativeHost("Native host — unavailable", false);
    QCOMPARE(selector.currentData().toString(), QString("paired-one"));
}

void NativeHostConnectionTest::dockerActionsRejectCommandsAndMismatchedReceipts()
{
    const QString id(64, 'a');
    QVERIFY(NativeHostDockerAction::validRequest("start", id));
    QVERIFY(NativeHostDockerAction::validRequest("stop", id.left(12)));
    QVERIFY(!NativeHostDockerAction::validRequest("exec", id));
    QVERIFY(!NativeHostDockerAction::validRequest("stop", "container-name"));
    QVERIFY(!NativeHostDockerAction::validRequest("stop", id + '\n'));
    QJsonObject result{{"schema", "dockpipe.native_host_docker_action.v1"},
        {"outcome", "Success"}, {"action", "stop"}, {"id", id}};
    QVERIFY(NativeHostDockerAction::succeeded(QJsonDocument(result).toJson(), "stop", id));
    QVERIFY(!NativeHostDockerAction::succeeded(QJsonDocument(result).toJson(), "start", id));
    QVERIFY(!NativeHostDockerAction::succeeded(QJsonDocument(result).toJson(), "stop", QString(64, 'b')));
    result.insert("outcome", "Cancelled");
    QVERIFY(!NativeHostDockerAction::succeeded(QJsonDocument(result).toJson(), "stop", id));
    result.insert("outcome", "Failure");
    QVERIFY(!NativeHostDockerAction::succeeded(QJsonDocument(result).toJson(), "stop", id));
}

void NativeHostConnectionTest::dockerQueriesRequireTypedBoundedMatchingResults()
{
    const QString id(64, 'a');
    QVERIFY(NativeHostDockerQuery::validRequest("container", id));
    QVERIFY(NativeHostDockerQuery::validRequest("network", id.toUpper()));
    QVERIFY(NativeHostDockerQuery::validRequest("volume", QString(255, 'v')));
    QVERIFY(!NativeHostDockerQuery::validRequest("volume", QString(256, 'v')));
    QVERIFY(!NativeHostDockerQuery::validRequest("volume", "../data"));
    QVERIFY(!NativeHostDockerQuery::validRequest("container", id + '\n'));
    QJsonObject result{{"schema", "dockpipe.native_host_docker_query.v1"}, {"outcome", "Success"},
        {"kind", "container"}, {"id", id}, {"inspect", QJsonObject{{"Name", "example"}}},
        {"logs", "last line"}, {"logs_truncated", true}};
    NativeHostDockerQuery query;
    const auto decode = [&]() {
        return NativeHostDockerQuery::decode(QJsonDocument(result).toJson(), "container", id, &query);
    };
    QVERIFY(decode());
    QCOMPARE(query.logs, QString("last line"));
    QVERIFY(query.logsTruncated);
    QVERIFY(!NativeHostDockerQuery::decode(QJsonDocument(result).toJson(), "network", id, &query));
    QVERIFY(!NativeHostDockerQuery::decode(QJsonDocument(result).toJson(), "container", QString(64, 'b'), &query));
    for (const QString &outcome : {QStringLiteral("Failure"), QStringLiteral("Cancelled")}) {
        result.insert("outcome", outcome);
        QVERIFY(!decode());
    }
    result.insert("outcome", "Success");
    result.insert("inspect", QJsonArray{});
    QVERIFY(!decode());
    result.insert("inspect", QJsonObject{});
    result.insert("logs_error", "docker_logs_unavailable");
    QVERIFY(decode());
    QCOMPARE(query.logsError, QString("docker_logs_unavailable"));
    result.insert("logs_error", "unexpected error");
    QVERIFY(!decode());
    result.remove("logs_error");
    result.insert("logs", QString(64 * 1024, 'x'));
    QVERIFY(decode());
    result.insert("logs", QString(64 * 1024 + 1, 'x'));
    QVERIFY(!decode());
    result.insert("logs", "");
    result.insert("inspect", QJsonObject{{"value", QString(256 * 1024, 'x')}});
    QVERIFY(!decode());
    result.insert("inspect", QJsonObject{});
    result.insert("logs", QString(1024 * 1024, 'x'));
    QVERIFY(!decode());
}

int main(int argc, char **argv)
{
    if (argc > 1 && QString::fromLocal8Bit(argv[1]) == "host")
        return hostFixture(argc, argv);
    QApplication application(argc, argv);
    NativeHostConnectionTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "NativeHostConnectionTest.moc"
