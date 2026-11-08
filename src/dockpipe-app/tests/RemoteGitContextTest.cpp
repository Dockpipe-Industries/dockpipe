#include "GitConflictWidget.h"
#include "ProjectFilesWidget.h"
#include "ProjectGitClient.h"
#include "ProjectGitWidget.h"
#include "RemoteGitContext.h"
#include "WorkspaceFilesController.h"

#include <QApplication>
#include <QCryptographicHash>
#include <QJsonArray>
#include <QPushButton>
#include <QTemporaryDir>
#include <QTest>
#include <QTextDocument>
#include <QThread>

namespace {
QJsonObject target(const QString &node)
{
    return {{"node", node}, {"fingerprint", QString(64, 'a')}, {"generation", QString(64, 'b')}};
}
}

class RemoteGitContextTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
        // Configuration exercises no installed CLI or real machine services.
        qputenv("DOCKPIPE_BIN", "/unavailable/remote-git-fixture");
    }

    void bindsAllClientsAndKeepsMissingSelectionsRemote()
    {
        ProjectGitWidget widget;
        const auto first = target(QStringLiteral("first"));
        QVERIFY(RemoteGitContext::configure(&widget, first, QStringLiteral("checkout")));
        const auto clients = widget.findChildren<ProjectGitClient *>();
        QVERIFY(clients.size() >= 3);
        for (const auto *client : clients) {
            QVERIFY(RemoteGitContext::isRemote(client));
            QCOMPARE(RemoteGitContext::target(client), first);
            QCOMPARE(RemoteGitContext::root(client), QStringLiteral("checkout"));
        }
        auto *editor = widget.findChild<ProjectFilesWidget *>(QStringLiteral("gitConflictEditor"));
        QVERIFY(editor);
        QCOMPARE(editor->rootPath(), QStringLiteral("/"));
        QCOMPARE(WorkspaceFilesController::get(editor)->reader->context().kind,
            WorkspaceFilesClient::Context::Kind::Remote);
        QVERIFY(RemoteGitContext::configure(&widget, {}, {}));
        QVERIFY(RemoteGitContext::isRemote(&widget));
        for (auto *client : clients) {
            QVERIFY(RemoteGitContext::isRemote(client));
            QVERIFY(RemoteGitContext::target(client).isEmpty());
            bool completed = false;
            client->run(QStringLiteral("/"), {{"action", "commit"}},
                [&](bool ok, const QJsonObject &, const QString &) { QVERIFY(!ok); completed = true; });
            QVERIFY(completed);
            QVERIFY(!client->busy());
        }
        QTemporaryDir local;
        QVERIFY(RemoteGitContext::configureLocal(&widget, local.path()));
        QVERIFY(!RemoteGitContext::isRemote(&widget));
        for (const auto *client : clients)
            QVERIFY(!RemoteGitContext::isRemote(client));
        QCOMPARE(editor->rootPath(), local.path());
    }

    void refusesBusyChildClientWithoutChangingTarget()
    {
        ProjectGitWidget widget;
        const auto first = target(QStringLiteral("first"));
        QVERIFY(RemoteGitContext::configure(&widget, first, QStringLiteral("checkout")));
        QTest::qWait(20);
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        auto *client = widget.findChild<ProjectGitClient *>();
        QVERIFY(client);
        bool completed = false;
        client->run(QStringLiteral("/"), {{"action", "status"}},
            [&](bool, const QJsonObject &, const QString &) { completed = true; });
        QVERIFY(client->busy());
        QString error;
        QVERIFY(!RemoteGitContext::configure(&widget, target(QStringLiteral("second")), QStringLiteral("other"), &error));
        QVERIFY(!error.isEmpty());
        QCOMPARE(RemoteGitContext::target(&widget), first);
        for (const auto *child : widget.findChildren<ProjectGitClient *>())
            QCOMPARE(RemoteGitContext::target(child), first);
        client->cancel();
        QTRY_VERIFY(completed);
        QVERIFY(!client->busy());
    }

    void conflictReadWriteUsesCapturedFilesContextAndPreservesDrafts()
    {
        ProjectGitWidget widget;
        const auto first = target(QStringLiteral("first"));
        QVERIFY(RemoteGitContext::configure(&widget, first, QStringLiteral("checkout")));
        QTest::qWait(20);
        auto *conflicts = widget.findChild<GitConflictWidget *>();
        auto *editor = widget.findChild<ProjectFilesWidget *>(QStringLiteral("gitConflictEditor"));
        QVERIFY(conflicts && editor);
        auto context = WorkspaceFilesController::get(editor)->reader->context();
        QByteArray contents("<<<<<<< ours\n=======\n>>>>>>> theirs\n");
        int reads = 0;
        int writes = 0;
        bool correctContext = true;
        auto dispatch = [&](const WorkspaceFilesClient::Context &captured, const QJsonObject &request,
                            WorkspaceFilesClient::Completion completion) -> WorkspaceFilesClient::Cancel {
            correctContext = correctContext && captured == context;
            QJsonObject response{{"schema", "dockpipe.files/v1"}, {"path", request.value("path")}};
            const auto action = request.value("action").toString();
            if (action == QStringLiteral("read")) {
                ++reads;
                response.insert(QStringLiteral("content"), QString::fromLatin1(contents.toBase64()));
            } else if (action == QStringLiteral("write")) {
                ++writes;
                const auto expectedRevision = QCryptographicHash::hash(contents, QCryptographicHash::Sha256).toHex();
                correctContext = correctContext && request.value("revision").toString().toLatin1() == expectedRevision;
                contents = QByteArray::fromBase64(request.value("content").toString().toLatin1());
            } else {
                response.insert(QStringLiteral("entries"), QJsonArray{});
            }
            response.insert(QStringLiteral("revision"), QString::fromLatin1(
                QCryptographicHash::hash(contents, QCryptographicHash::Sha256).toHex()));
            completion({WorkspaceFilesClient::Outcome::Success, response, {}, captured});
            return {};
        };
        QVERIFY(WorkspaceFilesController::configure(editor, context, dispatch));
        conflicts->setSnapshot(QStringLiteral("/"), {{"snapshot", "reviewed"},
            {"operation", QJsonObject{{"kind", "merge"}, {"conflicts", QJsonArray{"conflict.txt"}}}}});
        conflicts->findChild<QPushButton *>(QStringLiteral("gitConflictOpen"))->click();
        QTRY_VERIFY(editor->currentDocument());
        QCOMPARE(reads, 1);
        QCOMPARE(editor->currentPath(), QStringLiteral("/conflict.txt"));
        editor->currentDocument()->setPlainText(QStringLiteral("resolved remotely\n"));
        editor->currentDocument()->setModified(true);
        QString error;
        QVERIFY(!RemoteGitContext::configure(&widget, target(QStringLiteral("second")), QStringLiteral("other"), &error));
        QVERIFY(!error.isEmpty());
        QVERIFY(editor->hasUnsavedChanges());
        QCOMPARE(RemoteGitContext::target(&widget), first);
        QVERIFY(WorkspaceFilesController::get(editor)->reader->context() == context);
        bool saved = false;
        editor->saveFileAsync([&](bool ok) { saved = ok; });
        QTRY_VERIFY(saved);
        QCOMPARE(writes, 1);
        QVERIFY(correctContext);
        QCOMPARE(contents, QByteArray("resolved remotely\n"));
        QVERIFY(!editor->hasUnsavedChanges());
        QVERIFY(RemoteGitContext::configure(&widget, target(QStringLiteral("second")), QStringLiteral("other")));
        QCOMPARE(WorkspaceFilesController::get(editor)->reader->context().nodeId, QStringLiteral("second"));
        QVERIFY(!editor->currentDocument());
    }
};

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    if (application.arguments().value(1) == QStringLiteral("remote")) {
        QThread::sleep(5);
        return 1;
    }
    RemoteGitContextTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteGitContextTest.moc"
