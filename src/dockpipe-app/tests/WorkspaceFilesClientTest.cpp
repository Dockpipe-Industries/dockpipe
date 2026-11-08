#include "WorkspaceFilesClient.h"
#include "WorkspaceFileModel.h"

#include <QAbstractItemModelTester>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QSignalSpy>
#include <QTest>
#include <QTimer>
#include <cstdio>

namespace {
const QString revision(64, QLatin1Char('a'));

int fixture(QCoreApplication &application)
{
    QFile input;
    if (!input.open(stdin, QIODevice::ReadOnly))
        return 2;
    const auto envelope = QJsonDocument::fromJson(input.readAll()).object();
    const auto request = envelope.value("request").toObject();
    const QString path = request.value("path").toString();
    if (path == "slow") {
        QTimer::singleShot(5000, &application, &QCoreApplication::quit);
        return application.exec();
    }
    QByteArray output;
    if (path == "overflow") {
        output = QByteArray(2 * 1024 * 1024 + 1, 'x');
    } else if (path == "invalid") {
        output = "not json";
    } else if (request.value("action") == "list") {
        QJsonArray entries;
        bool more = false;
        if (path == "." && request.value("after").toString().isEmpty()) {
            entries = QJsonArray{QJsonObject{{"name", "docs"}, {"kind", "directory"}},
                QJsonObject{{"name", "hardware-key-notes.txt"}, {"kind", "file"}, {"size_bytes", "9007199254740993"}}};
            more = true;
        } else if (path == ".") {
            if (request.value("revision") != revision)
                return 4;
            entries = QJsonArray{QJsonObject{{"name", "last.txt"}, {"kind", "file"}, {"size_bytes", "4"}}};
        } else if (path == "docs") {
            entries = QJsonArray{QJsonObject{{"name", "guide.md"}, {"kind", "file"}, {"size_bytes", "12"}}};
        }
        output = QJsonDocument(QJsonObject{{"schema", "dockpipe.files/v1"}, {"path", path},
            {"revision", revision}, {"entries", entries}, {"more", more},
            {"after", more ? QStringLiteral("hardware-key-notes.txt") : QString()}}).toJson(QJsonDocument::Compact);
    } else {
        output = QJsonDocument(QJsonObject{{"schema", "dockpipe.files/v1"}, {"path", path},
            {"revision", revision}, {"content", QString::fromLatin1(QByteArray("\xEF\xBB\xBFhello\r\n").toBase64())}})
            .toJson(QJsonDocument::Compact);
    }
    QFile standardOutput;
    if (!standardOutput.open(stdout, QIODevice::WriteOnly))
        return 2;
    standardOutput.write(output);
    standardOutput.flush();
    return 0;
}

QJsonObject readRequest(const QString &path)
{
    return {{"schema", "dockpipe.files/v1"}, {"action", "read"}, {"path", path}};
}
}

class WorkspaceFilesClientTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    }

    void preservesProtocolNamesAndExactBytes()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.localRoot = QDir::tempPath();
        context.generation = 8;
        client.setContext(context);
        bool done = false;
        client.run(readRequest("hardware-key-notes.txt"), this, [&](const auto &result) {
            QVERIFY(result.success());
            QCOMPARE(result.context.generation, quint64(8));
            QCOMPARE(QByteArray::fromBase64(result.value.value("content").toString().toLatin1()),
                QByteArray("\xEF\xBB\xBFhello\r\n"));
            done = true;
        });
        QVERIFY(!done);
        QTRY_VERIFY(done);
    }

    void cancellationBusyTimeoutAndBounds()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.localRoot = QDir::tempPath();
        client.setContext(context);
        bool cancelled = false;
        bool busy = false;
        client.run(readRequest("slow"), this, [&](const auto &result) {
            cancelled = result.outcome == WorkspaceFilesClient::Outcome::Cancelled;
        });
        client.run(readRequest("ignored"), this, [&](const auto &result) {
            busy = result.outcome == WorkspaceFilesClient::Outcome::Busy;
        });
        client.cancel();
        QTRY_VERIFY(cancelled && busy);
        for (const QString &path : {QStringLiteral("slow"), QStringLiteral("overflow"), QStringLiteral("invalid")}) {
            bool failed = false;
            client.run(readRequest(path), this, [&](const auto &result) {
                failed = result.outcome == WorkspaceFilesClient::Outcome::Failure && !result.error.isEmpty();
            }, path == "slow" ? 20 : 30000);
            QTRY_VERIFY(failed);
        }
    }

    void ownerAndContextReplacement()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.localRoot = QDir::tempPath();
        context.generation = 1;
        client.setContext(context);
        bool called = false;
        auto *owner = new QObject;
        client.run(readRequest("slow"), owner, [&](const auto &) { called = true; });
        delete owner;
        QTRY_VERIFY(!client.busy());
        QVERIFY(!called);
        bool cancelled = false;
        client.run(readRequest("slow"), this, [&](const auto &result) {
            cancelled = result.outcome == WorkspaceFilesClient::Outcome::Cancelled;
            QCOMPARE(result.context.generation, quint64(1));
        });
        context.generation = 2;
        client.setContext(context);
        QTRY_VERIFY(cancelled);
    }

    void remoteNeverFallsBackAndOwnerDestructionDoesNotWait()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.localRoot = QDir::tempPath();
        client.setContext(context);
        bool failed = false;
        client.run(readRequest("readme.txt"), this, [&](const auto &result) {
            failed = result.outcome == WorkspaceFilesClient::Outcome::Failure;
        });
        QTRY_VERIFY(failed);
        context.kind = WorkspaceFilesClient::Context::Kind::Local;
        auto *pending = new WorkspaceFilesClient;
        pending->setContext(context);
        pending->run(readRequest("slow"), this, [](const auto &) {});
        QElapsedTimer elapsed;
        elapsed.start();
        delete pending;
        QVERIFY(elapsed.elapsed() < 100);
    }

    void lazyDirectoryPages()
    {
        WorkspaceFileModel model;
        QAbstractItemModelTester tester(&model, QAbstractItemModelTester::FailureReportingMode::QtTest);
        WorkspaceFilesClient::Context context;
        context.localRoot = QDir::tempPath();
        QSignalSpy loaded(&model, &WorkspaceFileModel::directoryLoaded);
        model.setContext(context);
        QTRY_VERIFY(loaded.count() >= 1);
        QTRY_VERIFY(model.rowCount() >= 2);
        const auto file = model.indexForPath("hardware-key-notes.txt");
        QVERIFY(file.isValid());
        QCOMPARE(file.data(WorkspaceFileModel::SizeBytesRole).toString(), QStringLiteral("9007199254740993"));
        if (model.canFetchMore({}))
            model.fetchMore({});
        QTRY_COMPARE(model.rowCount(), 3);
        const auto directory = model.indexForPath("docs");
        QVERIFY(model.isDirectory(directory));
        model.fetchMore(directory);
        QTRY_COMPARE(model.rowCount(directory), 1);
        QCOMPARE(model.relativePath(model.index(0, 0, directory)), QStringLiteral("docs/guide.md"));
        QCOMPARE(model.parent(model.index(0, 0, directory)), directory);
        model.setDirectory("docs");
        QTRY_COMPARE(model.rowCount(), 1);
        QCOMPARE(model.relativePath(model.index(0, 0)), QStringLiteral("docs/guide.md"));
        model.refresh();
        QTRY_COMPARE(model.rowCount(), 1);
        QVERIFY(model.indexForPath("docs/guide.md").isValid());
    }

    void remoteContextBindingAndLateResponses()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.nodeId = "worker";
        context.fingerprint = QString(64, 'b');
        context.workerGeneration = "worker-generation";
        context.rootId = "opaque-root";
        context.generation = 1;
        client.setContext(context);
        WorkspaceFilesClient::Completion transportCompletion;
        bool transportCancelled = false;
        bool boundContext = false;
        client.setRemoteDispatch([&](const auto &captured, const auto &request, auto completion) {
            boundContext = captured.nodeId == context.nodeId && captured.workerGeneration == "worker-generation"
                && captured.rootId == "opaque-root" && request.value("path") == QStringLiteral("notes.txt");
            transportCompletion = std::move(completion);
            return WorkspaceFilesClient::Cancel([&]() { transportCancelled = true; });
        });
        bool cancelled = false;
        client.run(readRequest("notes.txt"), this, [&](const auto &result) {
            cancelled = result.outcome == WorkspaceFilesClient::Outcome::Cancelled;
        });
        QVERIFY(boundContext);
        context.generation = 2;
        client.setContext(context);
        QVERIFY(transportCancelled);
        QTRY_VERIFY(cancelled);
        // A late reply from the revoked worker cannot finish a new attempt.
        transportCompletion({WorkspaceFilesClient::Outcome::Success, {{"schema", "dockpipe.files/v1"}}, {}, {}});
        QVERIFY(!client.busy());
    }

    void cancellationFromBusySignalDoesNotStartAHelper()
    {
        WorkspaceFilesClient client;
        WorkspaceFilesClient::Context context;
        context.localRoot = QDir::tempPath();
        client.setContext(context);
        connect(&client, &WorkspaceFilesClient::busyChanged, &client, [&](bool busy) {
            if (busy)
                client.cancel();
        });
        bool cancelled = false;
        client.run(readRequest("slow"), this, [&](const auto &result) {
            cancelled = result.outcome == WorkspaceFilesClient::Outcome::Cancelled;
        });
        QTRY_VERIFY(cancelled);
        QVERIFY(!client.busy());
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == "files")
        return fixture(application);
    WorkspaceFilesClientTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "WorkspaceFilesClientTest.moc"
