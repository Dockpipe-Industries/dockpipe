#include "RemoteWorkspaceFilesDispatch.h"
#include "RemoteFeatureClient.h"

#include <QCoreApplication>
#include <QFile>
#include <QJsonDocument>
#include <QSettings>
#include <QTemporaryDir>
#include <QTest>
#include <cstdio>

namespace {
int fixture(const QString &command)
{
    QFile input;
    if (!input.open(stdin, QIODevice::ReadOnly))
        return 2;
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    QJsonObject response;
    if (command == "feature-prepare") {
        for (const auto &key : {"schema", "target", "root", "action", "deadline"})
            response.insert(key, request.value(key));
        response.insert("id", QString(64, 'c'));
    } else if (command == "feature-call") {
        const auto payload = request.value("payload").toObject();
        const auto path = qEnvironmentVariableIsSet("DOCKPIPE_FILES_WRONG_RECEIPT")
            ? QJsonValue("another-file.txt") : payload.value("destination");
        const QJsonObject result{{"schema", "dockpipe.files/v1"}, {"path", path},
            {"revision", payload.value("revision")}};
        response = {{"request", request}, {"status", "success"},
            {"result", QJsonObject{{"status", "success"}, {"payload", result}}}};
    } else {
        return 3;
    }
    const auto bytes = QJsonDocument(response).toJson(QJsonDocument::Compact);
    std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
    return 0;
}
}

class RemoteWorkspaceFilesDispatchTest final : public QObject {
    Q_OBJECT
private slots:
    void renameKeepsDestinationReceiptAndRejectsDifferentFile()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        QObject owner;
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.nodeId = "worker";
        context.fingerprint = QString(64, 'a');
        context.workerGeneration = QString(64, 'b');
        context.rootId = "project";
        client.setContext(context);
        client.setRemoteDispatch(RemoteWorkspaceFilesDispatch::create(&owner));
        const QJsonObject request{{"schema", "dockpipe.files/v1"}, {"action", "rename"},
            {"path", "old.txt"}, {"destination", "new.txt"}, {"revision", QString(64, 'a')}};
        WorkspaceFilesClient::Result result;
        int completions = 0;
        client.run(request, &owner, [&](const auto &response) { result = response; ++completions; });
        QTRY_COMPARE(completions, 1);
        QVERIFY(result.success());
        QCOMPARE(result.value.value("path").toString(), QString("new.txt"));
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());

        qputenv("DOCKPIPE_FILES_WRONG_RECEIPT", "1");
        client.run(request, &owner, [&](const auto &response) { result = response; ++completions; });
        QTRY_COMPARE(completions, 2);
        QCOMPARE(result.outcome, WorkspaceFilesClient::Outcome::Failure);
        QCOMPARE(RemoteFeatureClient::pendingRecoveries().size(), 1);
        const auto reference = RemoteFeatureClient::pendingRecoveries().first().toObject();
        QCOMPARE(reference.value("target").toObject().value("node").toString(), QString("worker"));
        QCOMPARE(reference.value("action").toString(), QString("files.rename"));
        QVERIFY(!QJsonDocument(reference).toJson().contains("old.txt"));
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == "remote")
        return fixture(application.arguments().value(2));
    application.setOrganizationName("DockpipeFilesFixture");
    application.setApplicationName("RemoteWorkspaceFilesDispatchTest");
    QTemporaryDir settings;
    if (!settings.isValid())
        return 8;
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    RemoteWorkspaceFilesDispatchTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteWorkspaceFilesDispatchTest.moc"
