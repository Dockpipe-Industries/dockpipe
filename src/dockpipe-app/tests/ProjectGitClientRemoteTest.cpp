#include "ProjectGitClient.h"
#include "RemoteGitContext.h"

#include <QCoreApplication>
#include <QFile>
#include <QJsonDocument>
#include <QSignalSpy>
#include <QTest>
#include <QThread>
#include <cstdio>

namespace {
QJsonObject target()
{
    return {{"node", "fixture"}, {"fingerprint", QString(64, 'a')}, {"generation", QString(64, 'b')}};
}

int fixture(const QStringList &arguments)
{
    const auto mode = qgetenv("DOCKPIPE_GIT_REMOTE_FIXTURE");
    if (mode == "pause")
        QThread::msleep(1000);
    const int requestIndex = arguments.indexOf(QStringLiteral("--request"));
    QFile input(arguments.value(requestIndex + 1));
    if (requestIndex < 0 || !input.open(QIODevice::ReadOnly))
        return 2;
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    QJsonObject response;
    if (arguments.value(1) == QStringLiteral("git")) {
        if (mode != "local" || request.value("workdir").toString() != QStringLiteral("/fixture/local"))
            return 3;
        response = {{"root", "/fixture/local"}, {"branch", "local-fixture"}};
    } else {
        const int authIndex = arguments.indexOf(QStringLiteral("--auth"));
        if (arguments.value(2) != QStringLiteral("feature-call") || authIndex < 0
            || arguments.value(authIndex + 1) != QStringLiteral("never")
            || request.value("payload").toObject().contains("workdir")
            || request.value("target").toObject() != target()
            || request.value("root").toString() != QStringLiteral("checkout"))
            return 4;
        request.insert(QStringLiteral("id"), QString(64, 'c'));
        if (mode == "wrong-target")
            request.insert(QStringLiteral("target"), QJsonObject{{"node", "other"}});
        if (mode == "wrong-root")
            request.insert(QStringLiteral("root"), QStringLiteral("other"));
        if (mode == "wrong-action")
            request.insert(QStringLiteral("action"), QStringLiteral("git.push"));
        if (mode == "wrong-nonce")
            request.insert(QStringLiteral("nonce"), QString(64, 'd'));
        if (mode == "wrong-payload")
            request.insert(QStringLiteral("payload"), QJsonObject{{"action", "push"}});
        if (mode == "wrong-deadline")
            request.insert(QStringLiteral("deadline"), QStringLiteral("2000-01-01T00:00:00Z"));
        if (mode == "wrong-schema")
            request.insert(QStringLiteral("schema"), QStringLiteral("different/v1"));
        if (mode == "invalid-id")
            request.insert(QStringLiteral("id"), QString(64, 'z'));
        const QString status = mode == "unknown" ? QStringLiteral("unknown") : QStringLiteral("success");
        const QJsonObject snapshot{{"root", "/worker/private/checkout"}, {"branch", "hardware-key-fixture"}};
        QJsonObject result{{"status", status}, {"payload", snapshot}};
        if (mode == "wrong-result")
            result.insert(QStringLiteral("status"), QStringLiteral("failure"));
        if (mode == "non-object")
            result.insert(QStringLiteral("payload"), QStringLiteral("invalid snapshot"));
        response = {{"request", request}, {"status", status}, {"result", result},
            {"receipt_only", mode == "receipt-only"}};
    }
    if (mode == "garbage") {
        std::fputs("unframed output\n", stdout);
        return 0;
    }
    const auto output = QJsonDocument(response).toJson(QJsonDocument::Compact) + '\n';
    std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
    return mode == "process-failure" ? 1 : 0;
}
}

class ProjectGitClientRemoteTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_GIT_REMOTE_FIXTURE");
    }

    void remoteIntentAndVirtualRoot()
    {
        ProjectGitClient client;
        RemoteGitContext::bind(&client, true, target(), QStringLiteral("checkout"));
        int completions = 0;
        bool succeeded = false;
        QJsonObject snapshot;
        client.run(QStringLiteral("/"), {{"action", "status"}, {"workdir", "/"}},
            [&](bool ok, const QJsonObject &value, const QString &) {
                succeeded = ok;
                snapshot = value;
                ++completions;
            });
        QVERIFY(client.busy());
        QTRY_COMPARE(completions, 1);
        QVERIFY(succeeded);
        QVERIFY(!client.busy());
        QCOMPARE(snapshot.value("root").toString(), QStringLiteral("/"));
        QCOMPARE(snapshot.value("branch").toString(), QStringLiteral("hardware-key-fixture"));
    }

    void rejectsChangedOrUncertainResult_data()
    {
        QTest::addColumn<QByteArray>("mode");
        for (const auto &mode : {"wrong-target", "wrong-root", "wrong-action", "wrong-nonce", "wrong-payload",
                 "wrong-deadline", "wrong-schema", "invalid-id", "wrong-result", "receipt-only", "unknown",
                 "non-object", "garbage", "process-failure"})
            QTest::newRow(mode) << QByteArray(mode);
    }

    void rejectsChangedOrUncertainResult()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_GIT_REMOTE_FIXTURE", mode);
        ProjectGitClient client;
        RemoteGitContext::bind(&client, true, target(), QStringLiteral("checkout"));
        int completions = 0;
        bool succeeded = true;
        QString message;
        client.run(QStringLiteral("/"), {{"action", "status"}},
            [&](bool ok, const QJsonObject &value, const QString &error) {
                succeeded = ok;
                QVERIFY(value.isEmpty());
                message = error;
                ++completions;
            });
        QTRY_COMPARE(completions, 1);
        QVERIFY(!succeeded);
        QVERIFY(message.contains(QStringLiteral("Refresh")));
        QVERIFY(!message.contains(QStringLiteral("/worker/private")));
    }

    void missingRemoteContextNeverFallsBackToLocal()
    {
        QObject owner;
        ProjectGitClient client(&owner);
        RemoteGitContext::bind(&owner, true, {}, {});
        QSignalSpy busy(&client, &ProjectGitClient::busyChanged);
        bool succeeded = true;
        int completions = 0;
        client.run(QStringLiteral("/fixture/local"), {{"action", "commit"}},
            [&](bool ok, const QJsonObject &, const QString &) { succeeded = ok; ++completions; });
        QCOMPARE(completions, 1);
        QVERIFY(!succeeded);
        QVERIFY(RemoteGitContext::isRemote(&client));
        QVERIFY(busy.isEmpty());
    }

    void rejectsCallerSelectedHostDirectory()
    {
        ProjectGitClient client;
        RemoteGitContext::bind(&client, true, target(), QStringLiteral("checkout"));
        QSignalSpy busy(&client, &ProjectGitClient::busyChanged);
        bool succeeded = true;
        client.run(QStringLiteral("/"), {{"action", "commit"}, {"workdir", "/local/private"}},
            [&](bool ok, const QJsonObject &, const QString &) { succeeded = ok; });
        QVERIFY(!succeeded);
        QVERIFY(busy.isEmpty());
    }

    void rejectsOversizedRequestBeforeStartingTransport()
    {
        ProjectGitClient client;
        RemoteGitContext::bind(&client, true, target(), QStringLiteral("checkout"));
        QSignalSpy busy(&client, &ProjectGitClient::busyChanged);
        int completions = 0;
        client.run(QStringLiteral("/"), {{"action", "commit"}, {"message", QString(1024 * 1024, 'x')}},
            [&](bool ok, const QJsonObject &, const QString &) { QVERIFY(!ok); ++completions; });
        QCOMPARE(completions, 1);
        QVERIFY(!client.busy());
        QVERIFY(busy.isEmpty());
    }

    void busyAndCancellationRetainExistingTransportSemantics()
    {
        qputenv("DOCKPIPE_GIT_REMOTE_FIXTURE", "pause");
        ProjectGitClient client;
        RemoteGitContext::bind(&client, true, target(), QStringLiteral("checkout"));
        int completions = 0;
        client.run(QStringLiteral("/"), {{"action", "status"}},
            [&](bool ok, const QJsonObject &, const QString &) { QVERIFY(!ok); ++completions; });
        QVERIFY(client.busy());
        client.run(QStringLiteral("/"), {{"action", "push"}},
            [&](bool ok, const QJsonObject &, const QString &) { QVERIFY(!ok); ++completions; });
        QCOMPARE(completions, 1);
        client.cancel();
        QTRY_COMPARE(completions, 2);
        QVERIFY(!client.busy());
        client.cancel();
        QCOMPARE(completions, 2);
    }

    void localAdmissionUsesTheTrustedOperatorBinary()
    {
        qputenv("DOCKPIPE_GIT_REMOTE_FIXTURE", "local");
        ProjectGitClient client;
        int completions = 0;
        bool succeeded = false;
        client.run(QStringLiteral("/fixture/local"), {{"action", "status"}},
            [&](bool ok, const QJsonObject &value, const QString &) {
                succeeded = ok;
                QCOMPARE(value.value("branch").toString(), QStringLiteral("local-fixture"));
                ++completions;
            });
        QTRY_COMPARE(completions, 1);
        QVERIFY(succeeded);
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == QStringLiteral("remote")
        || application.arguments().value(1) == QStringLiteral("git"))
        return fixture(application.arguments());
    ProjectGitClientRemoteTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "ProjectGitClientRemoteTest.moc"
