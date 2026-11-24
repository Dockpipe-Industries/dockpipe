#include "FileEditorWorkspace.h"
#include "PipeLangLanguageClient.h"
#include "SourceEditor.h"

#include <QApplication>
#include <QDialog>
#include <QDialogButtonBox>
#include <QFile>
#include <QFileInfo>
#include <QHelpEvent>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QSignalSpy>
#include <QStackedWidget>
#include <QTemporaryDir>
#include <QTest>
#include <QTextBlock>
#include <QTextDocument>
#include <QToolButton>
#include <QToolTip>

class PipeLangEditorTest : public QObject {
    Q_OBJECT
private:
    QString node() const { return qEnvironmentVariable("DOCKPIPE_TEST_NODE"); }
    QString compiler() const { return qEnvironmentVariable("PIPELANG_MODELC"); }
    QString server() const { return qEnvironmentVariable("DOCKPIPE_TEST_PIPELANG_SERVER"); }
    PipeLangLanguageClient::Configuration fixture(const QString &root) const
    {
        return {node(), QStringLiteral(DOCKPIPE_LSP_FIXTURE), compiler(), root};
    }

private slots:
    void initTestCase()
    {
        qInfo() << "Selected tools" << node() << compiler() << server();
        qInfo() << "Node admission" << QFileInfo(node()).exists() << QFileInfo(node()).isExecutable();
    }

    void explicitSelection()
    {
        PipeLangLanguageClient client;
        QSignalSpy status(&client, &PipeLangLanguageClient::statusChanged);
        QVERIFY(!client.start({"node", "language-server.js", "pipelang-modelc", {}}));
        QVERIFY(!client.isReady());
        QCOMPARE(status.size(), 1);
    }

    void transportVersionsCancellationAndRestart()
    {
        if (node().isEmpty() || compiler().isEmpty())
            QSKIP("Select DOCKPIPE_TEST_NODE and PIPELANG_MODELC for process tests.");
        QTemporaryDir root;
        QVERIFY(root.isValid());
        const QString path = root.filePath("Sample.pipe");
        PipeLangLanguageClient client;
        QSignalSpy diagnostics(&client, &PipeLangLanguageClient::diagnostics);
        QSignalSpy hover(&client, &PipeLangLanguageClient::hoverResult);
        QSignalSpy versions(&client, &PipeLangLanguageClient::documentVersionChanged);
        QSignalSpy statuses(&client, &PipeLangLanguageClient::statusChanged);
        qputenv("DOCKPIPE_LSP_FIXTURE_MODE", "normal");
        client.setDocument(path, "first");
        QVERIFY2(client.start(fixture(root.path())), qPrintable(statuses.isEmpty() ? QString() : statuses.last()[0].toString()));
        QTRY_VERIFY_WITH_TIMEOUT(client.isReady(), 5000);
        QTRY_VERIFY_WITH_TIMEOUT(!diagnostics.isEmpty() && !diagnostics.last()[2].value<QJsonArray>().isEmpty(), 5000);
        for (const auto &notification : diagnostics) {
            for (const auto &item : notification[2].value<QJsonArray>())
                QCOMPARE(item.toObject().value("message").toString(), QStringLiteral("current"));
        }
        client.hover(path, 0, 1);
        client.setDocument(path, "second");
        QTest::qWait(350);
        QVERIFY(hover.isEmpty());
        client.hover(path, 0, 1);
        QTRY_COMPARE_WITH_TIMEOUT(hover.size(), 1, 5000);
        const int previousVersion = versions.last()[1].toInt();
        client.closeDocument(path);
        client.setDocument(path, "third");
        QVERIFY(versions.last()[1].toInt() > previousVersion);
        client.stop();
        QTRY_VERIFY_WITH_TIMEOUT(!statuses.isEmpty() && statuses.last()[0].toString().contains("stopped"), 5000);
        QVERIFY(!client.isReady());
        QVERIFY2(client.start(fixture(root.path())), qPrintable(statuses.isEmpty() ? QString() : statuses.last()[0].toString()));
        QTRY_VERIFY_WITH_TIMEOUT(client.isReady(), 5000);
        client.stop();
        QTRY_VERIFY_WITH_TIMEOUT(statuses.last()[0].toString().contains("stopped"), 5000);
        qunsetenv("DOCKPIPE_LSP_FIXTURE_MODE");
    }

    void malformedAndDeadServer()
    {
        if (node().isEmpty() || compiler().isEmpty())
            QSKIP("Select DOCKPIPE_TEST_NODE and PIPELANG_MODELC for process tests.");
        QTemporaryDir root;
        {
            PipeLangLanguageClient client;
            QSignalSpy statuses(&client, &PipeLangLanguageClient::statusChanged);
            qputenv("DOCKPIPE_LSP_FIXTURE_MODE", "malformed");
            QVERIFY2(client.start(fixture(root.path())), qPrintable(statuses.isEmpty() ? QString() : statuses.last()[0].toString()));
            QTRY_VERIFY_WITH_TIMEOUT(statuses.size() >= 2, 5000);
            QVERIFY(!client.isReady());
            bool rejected = false;
            for (const auto &status : statuses)
                rejected |= status[0].toString().contains("Content-Length");
            QVERIFY(rejected);
        }
        {
            PipeLangLanguageClient client;
            QSignalSpy statuses(&client, &PipeLangLanguageClient::statusChanged);
            QSignalSpy diagnostics(&client, &PipeLangLanguageClient::diagnostics);
            qputenv("DOCKPIPE_LSP_FIXTURE_MODE", "death");
            const QString path = root.filePath("Sample.pipe");
            client.setDocument(path, "content");
            QVERIFY2(client.start(fixture(root.path())), qPrintable(statuses.isEmpty() ? QString() : statuses.last()[0].toString()));
            QTRY_VERIFY_WITH_TIMEOUT(client.isReady(), 5000);
            client.hover(path, 0, 0);
            QTRY_VERIFY_WITH_TIMEOUT(!client.isReady(), 5000);
            QVERIFY(statuses.last()[0].toString().contains("exited"));
            QVERIFY(diagnostics.last()[2].value<QJsonArray>().isEmpty());
        }
        qunsetenv("DOCKPIPE_LSP_FIXTURE_MODE");
    }

    void realCompilerEditor()
    {
        if (node().isEmpty() || compiler().isEmpty() || server().isEmpty())
            QSKIP("Select installed Node, standalone server and compiler for the real Qt smoke.");
        QTemporaryDir root;
        QVERIFY(root.isValid());
        const QString path = root.filePath("App.pipe");
        QFile disk(path);
        QVERIFY(disk.open(QIODevice::WriteOnly));
        disk.write("module Example; public Class DiskOnly {}\n");
        disk.close();
        QString source = "module Example;\npublic model Class App {\n    public int Run() { var count = 42; return count; }\n}\n";
        auto *stack = new QStackedWidget;
        auto *normal = new SourceEditor;
        auto *workflow = new SourceEditor;
        auto *empty = new QWidget;
        stack->addWidget(normal);
        stack->addWidget(workflow);
        stack->addWidget(empty);
        FileEditorWorkspace workspace(stack, normal, workflow, empty);
        workspace.resize(1000, 600);
        workspace.show();
        QVERIFY(workspace.addDocument(path, {}, source, false, false));
        auto *client = workspace.findChild<PipeLangLanguageClient *>();
        QVERIFY(client);
        QSignalSpy diagnostics(client, &PipeLangLanguageClient::diagnostics);
        QSignalSpy hover(client, &PipeLangLanguageClient::hoverResult);
        connect(client, &PipeLangLanguageClient::statusChanged, &workspace, [](const QString &message) { qInfo() << message; });
        bool configured = false;
        QTimer::singleShot(0, &workspace, [&] {
            auto *dialog = qobject_cast<QDialog *>(QApplication::activeModalWidget());
            if (!dialog)
                return;
            dialog->findChild<QLineEdit *>("pipeLangNodePath")->setText(node());
            dialog->findChild<QLineEdit *>("pipeLangServerPath")->setText(server());
            dialog->findChild<QLineEdit *>("pipeLangCompilerPath")->setText(compiler());
            dialog->findChild<QLineEdit *>("pipeLangSourceRoot")->setText(root.path());
            configured = true;
            dialog->accept();
        });
        workspace.findChild<QToolButton *>("configurePipeLangServer")->click();
        QVERIFY(configured);
        QTRY_VERIFY_WITH_TIMEOUT(client->isReady(), 15000);
        QTRY_VERIFY_WITH_TIMEOUT(diagnostics.size() >= 2, 15000);
        QVERIFY(diagnostics.last()[2].value<QJsonArray>().isEmpty());
        auto cursor = normal->document()->find(QStringLiteral("count"));
        cursor.setPosition(cursor.selectionStart() + 1);
        normal->setTextCursor(cursor);
        const QPoint point = normal->cursorRect(cursor).center();
        QHelpEvent help(QEvent::ToolTip, point, normal->viewport()->mapToGlobal(point));
        QApplication::sendEvent(normal->viewport(), &help);
        QTRY_COMPARE_WITH_TIMEOUT(hover.size(), 1, 15000);
        QVERIFY(QToolTip::text().contains(QStringLiteral("int")));
        QVERIFY(workspace.openInOtherPane(path));
        normal->setPlainText(source.replace(QStringLiteral("42"), QStringLiteral("missingValue")));
        QTRY_VERIFY_WITH_TIMEOUT(!normal->document()->property("pipeLangDiagnostics").toList().isEmpty(), 15000);
        QVERIFY(normal->extraSelections().size() > 1);
        QVERIFY(workspace.activeNormalEditor()->extraSelections().size() > 1);
        normal->setPlainText("module Example;\npublic model Class App { public int Run() { return 7; } }\n");
        QVERIFY(normal->document()->property("pipeLangDiagnostics").toList().isEmpty());
        QTRY_VERIFY_WITH_TIMEOUT(diagnostics.last()[2].value<QJsonArray>().isEmpty(), 15000);
        QVERIFY(workspace.renameDocument(path, root.filePath("Renamed.pipe")));
        QVERIFY(workspace.addDocument(root.filePath("Other.txt"), {}, "ordinary text", false, false));
        QVERIFY(workspace.activeNormalEditor()->document()->property("pipeLangDiagnostics").toList().isEmpty());
        client->stop();
        auto *status = workspace.findChild<QLabel *>("pipeLangServerStatus");
        QTRY_VERIFY_WITH_TIMEOUT(status->text().contains("stopped"), 5000);
    }
};

QTEST_MAIN(PipeLangEditorTest)
#include "PipeLangEditorTest.moc"
