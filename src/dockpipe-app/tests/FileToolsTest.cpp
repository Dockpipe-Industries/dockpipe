#include "FileToolDialog.h"
#include "FileToolsController.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceFilesTestSupport.h"
#include "SessionManager.h"
#include "Theme.h"
#include "WorkflowCatalog.h"

#include <QApplication>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QMenu>
#include <QPlainTextEdit>
#include <QPointer>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QTextBrowser>
#include <QTimer>
#include <QToolButton>
#include <cstdio>
#ifdef Q_OS_UNIX
#include <csignal>
#endif

namespace {
int mockWorkflow(const QStringList &arguments)
{
    QString mode;
    for (const auto &argument : arguments) {
        if (argument.startsWith("TEST_MODE="))
            mode = argument.mid(10);
    }
    if (mode == "wait") {
#ifdef Q_OS_UNIX
        std::signal(SIGTERM, SIG_IGN);
#endif
        std::printf("fixture-ready\n");
        std::fflush(stdout);
        QCoreApplication::instance()->exec();
        return 0;
    }
    QByteArray line = "::dockpipe-file::" + QJsonDocument(QJsonObject{
        {"schema", 1}, {"kind", "markdown"}, {"text", QString::fromUtf8("# Inspection\n\nCafé: valid document.")}
    }).toJson(QJsonDocument::Compact) + '\n';
    if (mode == "malformed")
        line = "::dockpipe-file::{broken}\n";
    else if (mode == "duplicate")
        line += line;
    else if (mode == "missing")
        line = "Diagnostic output without a result.\n";
    else if (mode == "oversized")
        line = QByteArray(1024 * 1024 + 1, 'x');
    else if (mode == "aggregate")
        line = (QByteArray(4095, 'x') + '\n').repeated(513);
    else if (mode == "invalid-utf8")
        line = "::dockpipe-file::{\"schema\":1,\"kind\":\"markdown\",\"text\":\"\xff\"}\n";
    std::fwrite(line.constData(), 1, size_t(line.size()), stdout);
    std::fflush(stdout);
    return mode == "failure" ? 7 : 0;
}

Context invocation(const QString &root, const QString &mode)
{
    Context context = Context::createNew();
    context.workdir = root;
    context.workflowFile = root + "/action.yml";
    context.dockpipeBinary = QCoreApplication::applicationFilePath();
    context.maximumOutputLineBytes = 1024 * 1024;
    context.extraDockpipeEnv = {"TEST_MODE=" + mode};
    return context;
}
}

class FileToolsTest final : public QObject {
    Q_OBJECT
private slots:
    void results_data();
    void results();
    void closingCancelsSession();
    void discoversMatchingActionsWithoutExecuting();
    void installedPackage();
};

void FileToolsTest::results_data()
{
    QTest::addColumn<QString>("mode");
    QTest::addColumn<bool>("success");
    QTest::newRow("valid-unicode") << QString("good") << true;
    QTest::newRow("malformed") << QString("malformed") << false;
    QTest::newRow("duplicate") << QString("duplicate") << false;
    QTest::newRow("missing") << QString("missing") << false;
    QTest::newRow("nonzero-after-result") << QString("failure") << false;
    QTest::newRow("oversized-unterminated-line") << QString("oversized") << false;
    QTest::newRow("aggregate-output-limit") << QString("aggregate") << false;
    QTest::newRow("invalid-utf8") << QString("invalid-utf8") << false;
}

void FileToolsTest::results()
{
    QFETCH(QString, mode);
    QFETCH(bool, success);
    QTemporaryDir root;
    auto *dialog = new FileToolDialog(invocation(root.path(), mode), "Inspect document", "Example tools", "example.json");
    dialog->show();
    auto *status = dialog->findChild<QLabel *>("fileToolStatus");
    QTRY_VERIFY_WITH_TIMEOUT(status->property("failed").toBool() || status->text().startsWith("Report complete"), 5000);
    const bool completed = !status->property("failed").toBool();
    QVERIFY2(completed == success, qPrintable(status->text()));
    auto *preview = dialog->findChild<QTextBrowser *>();
    if (success)
        QVERIFY(preview->toPlainText().contains(QString::fromUtf8("Café")));
    else
        QVERIFY(preview->toPlainText().isEmpty());
    if (success && !qEnvironmentVariableIsEmpty("DOCKPIPE_FILE_TOOL_CAPTURE"))
        dialog->grab().save(qEnvironmentVariable("DOCKPIPE_FILE_TOOL_CAPTURE"));
    dialog->close();
}

void FileToolsTest::closingCancelsSession()
{
    QTemporaryDir root;
    auto *dialog = new FileToolDialog(invocation(root.path(), "wait"), "Inspect document", "Example tools", "example.json");
    auto *sessions = dialog->findChild<SessionManager *>();
    QVERIFY(sessions);
    QSignalSpy started(sessions, &SessionManager::sessionStarted);
    QSignalSpy stopped(sessions, &SessionManager::sessionStopped);
    const QPointer<FileToolDialog> lifetime(dialog);
    dialog->show();
    QTRY_COMPARE_WITH_TIMEOUT(started.count(), 1, 5000);
    QTRY_VERIFY_WITH_TIMEOUT(dialog->findChild<QPlainTextEdit *>("fileToolDetails")->toPlainText().contains("fixture-ready"), 5000);
    dialog->close();
    QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), 1, 15000);
    QTRY_VERIFY(lifetime.isNull());
}

void FileToolsTest::discoversMatchingActionsWithoutExecuting()
{
    QTemporaryDir root;
    QFile document(root.filePath("sample.json"));
    QVERIFY(document.open(QIODevice::WriteOnly));
    document.write("{\"value\":true}\n");
    document.close();
    ProjectFilesWidget files;
    files.resize(1000, 700);
    QVERIFY(FilesTest::setRoot(files, root.path()));
    QVERIFY(FilesTest::open(files, document.fileName()));
    ResolverMeta provider;
    provider.name = "example-tools";
    provider.surface.id = "example.tools";
    provider.surface.title = "Example tools";
    PackageSurfaceActionMeta action;
    action.id = "inspect";
    action.title = "Inspect document";
    action.scope = "file";
    action.workflow = root.filePath("not-installed.yml");
    action.fileExtensions = {"json"};
    provider.surface.actions = {action};
    FileToolsController::attach(&files, {provider});
    files.show();
    auto *button = files.findChild<QToolButton *>("filePackageTools");
    QVERIFY(button && !button->isHidden());
    auto *menu = button->menu();
    QVERIFY(QMetaObject::invokeMethod(menu, "aboutToShow"));
    QCOMPARE(menu->actions().size(), 2);
    QCOMPARE(menu->actions().last()->data().toString(), QString("example.tools:inspect"));
    QVERIFY(!files.findChild<FileToolDialog *>());
    QVERIFY(!files.findChild<SessionManager *>());
    QVERIFY(FilesTest::create(files, "notes.txt"));
    QVERIFY(QMetaObject::invokeMethod(menu, "aboutToShow"));
    QCOMPARE(menu->actions().size(), 1);
    QVERIFY(!menu->actions().first()->isEnabled());
    FileToolsController::attach(&files, {});
    QVERIFY(button->isHidden());
}

void FileToolsTest::installedPackage()
{
    const QString consumer = qEnvironmentVariable("DOCKPIPE_FILE_TOOL_CONSUMER");
    if (consumer.isEmpty())
        QSKIP("Set DOCKPIPE_FILE_TOOL_CONSUMER to qualify an installed package candidate.");
    const auto catalog = WorkflowCatalog::discoverCatalog(consumer);
    QVector<ResolverMeta> providers;
    for (const auto &provider : catalog.resolverDetails) {
        for (const auto &action : provider.surface.actions) {
            if (action.scope == "file" && action.fileExtensions.contains("json")) {
                providers.append(provider);
                break;
            }
        }
    }
    QVERIFY(!providers.isEmpty());
    QTemporaryDir root;
    QFile document(root.filePath(QString::fromUtf8("document café.json")));
    QVERIFY(document.open(QIODevice::WriteOnly));
    document.write("{\"name\":\"Café\",\"value\":9007199254740993}\n");
    document.close();
    ProjectFilesWidget files;
    files.resize(1180, 760);
    QVERIFY(FilesTest::setRoot(files, root.path()));
    QVERIFY(FilesTest::open(files, document.fileName()));
    FileToolsController::attach(&files, providers);
    files.show();
    auto *button = files.findChild<QToolButton *>("filePackageTools");
    QVERIFY(QMetaObject::invokeMethod(button->menu(), "aboutToShow"));
    auto *action = button->menu()->actions().last();
    QVERIFY(action->isEnabled() && !action->data().toString().isEmpty());
    action->trigger();
    auto *dialog = files.findChild<FileToolDialog *>();
    QVERIFY(dialog);
    auto *status = dialog->findChild<QLabel *>("fileToolStatus");
    QTRY_VERIFY_WITH_TIMEOUT(status->property("failed").toBool() || status->text().startsWith("Report complete"), 15000);
    QVERIFY2(!status->property("failed").toBool(), qPrintable(status->text()
        + "\n" + dialog->findChild<QPlainTextEdit *>("fileToolDetails")->toPlainText()));
    QVERIFY(dialog->findChild<QTextBrowser *>()->toPlainText().contains("9007199254740993"));
    const QString capture = qEnvironmentVariable("DOCKPIPE_FILE_TOOL_CAPTURE");
    if (!capture.isEmpty()) {
        QApplication::processEvents();
        dialog->grab().save(capture + ".installed.png");
        files.grab().save(capture + ".files.png");
    }
    dialog->close();
}

int main(int argc, char **argv)
{
    QStringList arguments;
    for (int index = 1; index < argc; ++index)
        arguments.append(QString::fromLocal8Bit(argv[index]));
    if (arguments.contains("--workflow-file")) {
        QCoreApplication child(argc, argv);
        return mockWorkflow(arguments);
    }
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QApplication app(argc, argv);
    app.setOrganizationName("dockpipe-test");
    app.setApplicationName("file-tools-test");
    previewDockpipeLauncherTheme(app, qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
        ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    FileToolsTest tests;
    return QTest::qExec(&tests, argc, argv);
}
#include "FileToolsTest.moc"
