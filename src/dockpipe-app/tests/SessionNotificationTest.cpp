#include "Context.h"
#include "LauncherSettings.h"
#include "MainWindow.h"
#include "NotificationBell.h"
#include "PackageSurfacePage.h"
#include "SessionManager.h"

#include <QApplication>
#include <QCryptographicHash>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QMessageBox>
#include <QProcessEnvironment>
#include <QRegularExpression>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <cstdio>

#ifdef Q_OS_LINUX
#include <signal.h>
#include <unistd.h>
#endif

class SessionNotificationTest final : public QObject {
    Q_OBJECT
private slots:
    void declaredInputsUseAuthoritativeOverridesWithoutChangingExtraEnvironment()
    {
        Context context = Context::createNew();
        context.extraDockpipeEnv = {"FIXTURE_MODE =selected", "FIXTURE_EMPTY=", "EXTRA=value",
                                   "DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC=  exact value  "};
        SessionLaunchOptions options;
        options.workflowInputOverrideNames = {"FIXTURE_MODE", "FIXTURE_EMPTY", "DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC"};
        context.transientLaunchEnv.insert("FIXTURE_SECRET", "private fixture");
        const QStringList arguments = SessionManager::dockpipeArguments(context, options);
        QCOMPARE(arguments.at(arguments.indexOf("FIXTURE_MODE =selected") - 1), QString("--var"));
        QCOMPARE(arguments.at(arguments.indexOf("FIXTURE_EMPTY=") - 1), QString("--var"));
        QCOMPARE(arguments.at(arguments.indexOf("EXTRA=value") - 1), QString("--env"));
        QCOMPARE(arguments.at(arguments.indexOf("DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC=  exact value  ") - 1), QString("--env"));
        QVERIFY(!arguments.join(' ').contains("private fixture"));
        QVERIFY(!context.toJson().contains("workflowInputOverrideNames"));
        QCOMPARE(SessionManager::dockpipeArguments(Context::fromJson(context.toJson()), options), arguments);

        const QStringList legacyArguments = SessionManager::dockpipeArguments(context);
        QCOMPARE(legacyArguments.at(legacyArguments.indexOf("FIXTURE_MODE =selected") - 1), QString("--env"));
        QCOMPARE(legacyArguments.at(legacyArguments.indexOf("FIXTURE_EMPTY=") - 1), QString("--env"));
        QVERIFY(!legacyArguments.contains("--var"));
    }

    void launchOptionsBelongToOneAttemptAndLegacyLaunchKeepsItsDefaults()
    {
        QTemporaryDir fixture;
        QVERIFY(fixture.isValid());
        Context context = Context::createNew();
        context.workdir = fixture.path();
        context.workflow = "foreground-success";
        context.dockpipeBinary = QCoreApplication::applicationFilePath();
        context.extraDockpipeEnv = {"FIXTURE_MODE=selected", "FIXTURE_EMPTY="};
        SessionLaunchOptions options;
        options.workflowInputOverrideNames = {"FIXTURE_MODE", "FIXTURE_EMPTY"};
        options.notifyWorkflowOutcome = false;
        SessionManager sessions;
        QSignalSpy stopped(&sessions, &SessionManager::sessionStopped);

        QVERIFY(sessions.launch(context, fixture.path(), options));
        QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), 1, 5000);
        const SessionInfo explicitAttempt = sessions.info(context.id);
        QCOMPARE(explicitAttempt.arguments, SessionManager::dockpipeArguments(context, options));
        QVERIFY(explicitAttempt.arguments.contains("--var"));
        QVERIFY(!explicitAttempt.notifyWorkflowOutcome);

        QVERIFY(sessions.launch(context, fixture.path()));
        QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), 2, 5000);
        const SessionInfo legacyAttempt = sessions.info(context.id);
        QCOMPARE(legacyAttempt.arguments, SessionManager::dockpipeArguments(context));
        QVERIFY(!legacyAttempt.arguments.contains("--var"));
        QVERIFY(legacyAttempt.notifyWorkflowOutcome);
    }

    void declaredInputsReachPinnedCoreWithExactEffectiveValues()
    {
#ifndef Q_OS_LINUX
        QSKIP("The actual core transport fixture currently requires Linux and /usr/bin/python3.");
#else
        const QString cli = qEnvironmentVariable("DOCKPIPE_TEST_CORE_CLI");
        const QByteArray expectedSha256 = qgetenv("DOCKPIPE_TEST_CORE_CLI_SHA256");
        if (cli.isEmpty() && expectedSha256.isEmpty())
            QSKIP("Set DOCKPIPE_TEST_CORE_CLI and its exact DOCKPIPE_TEST_CORE_CLI_SHA256 for actual CLI transport proof.");
        QVERIFY2(QFileInfo(cli).isAbsolute() && QFileInfo(cli).isFile() && !QFileInfo(cli).isSymLink()
                     && QFileInfo(cli).isExecutable() && QFileInfo(cli).size() <= 256 * 1024 * 1024,
                 "Select an absolute bounded regular executable for actual core transport proof.");
        QVERIFY(QRegularExpression("^[0-9a-f]{64}$").match(QString::fromLatin1(expectedSha256)).hasMatch());
        QFile selectedCli(cli);
        QVERIFY(selectedCli.open(QIODevice::ReadOnly));
        QCryptographicHash hash(QCryptographicHash::Sha256);
        QVERIFY(hash.addData(&selectedCli));
        QCOMPARE(hash.result().toHex(), expectedSha256);
        selectedCli.close();
        if (!QFileInfo::exists("/usr/bin/python3"))
            QSKIP("The actual core transport fixture requires /usr/bin/python3.");

        QTemporaryDir fixture;
        QVERIFY(fixture.isValid());
        QDir directory(fixture.path());
        QVERIFY(directory.mkpath("project/workflows/transport/models"));
        QVERIFY(directory.mkpath("home"));
        QVERIFY(directory.mkpath("tmp"));
        const QString project = directory.filePath("project");
        const QString workflow = directory.filePath("project/workflows/transport/config.yml");
        const auto write = [](const QString &path, const QByteArray &contents) {
            QFile file(path);
            return file.open(QIODevice::WriteOnly | QIODevice::NewOnly)
                && file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner)
                && file.write(contents) == contents.size();
        };
        QVERIFY(write(directory.filePath("project/dockpipe.config.json"), "{\"schema\":1}\n"));
        const QByteArray workflowSource = R"WORKFLOW(name: gui-input-transport
vault: none
docker_preflight: false
types: [models/FixtureConfig.pipe]
inputs:
  Mode: ephemeral
  Clear: declared-default
vars:
  FIXTURE_MODE: ephemeral
  FIXTURE_CLEAR: declared-default
  DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC: imported-default
steps:
  - id: capture-generated-inputs
    kind: host
    cwd: repo
    cmd: /usr/bin/python3 "$DOCKPIPE_WORKFLOW_DIR/capture.py"
)WORKFLOW";
        const QByteArray modelSource = R"MODEL(public Class FixtureConfig
{
    [EnvName = "FIXTURE_MODE"]
    public string Mode = "ephemeral";
    [EnvName = "FIXTURE_CLEAR"]
    public string Clear = "declared-default";
}
)MODEL";
        const QByteArray captureSource = R"PYTHON(import json
import os
from pathlib import Path

result = {
    "mode": os.environ.get("FIXTURE_MODE"),
    "clear": os.environ.get("FIXTURE_CLEAR"),
    "clear_present": "FIXTURE_CLEAR" in os.environ,
    "imported": os.environ.get("DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC"),
}
(Path(os.environ["DOCKPIPE_WORKDIR"]) / "effective-inputs.json").write_text(json.dumps(result))
)PYTHON";
        QVERIFY(write(workflow, workflowSource));
        QVERIFY(write(directory.filePath("project/workflows/transport/models/FixtureConfig.pipe"), modelSource));
        QVERIFY(write(directory.filePath("project/workflows/transport/capture.py"), captureSource));

        Context context = Context::createNew();
        context.workdir = project;
        context.workflowFile = workflow;
        context.extraDockpipeEnv = {"FIXTURE_MODE =selected", "FIXTURE_CLEAR=",
                                   "DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC=  exact value  "};
        SessionLaunchOptions options;
        options.workflowInputOverrideNames = {"FIXTURE_MODE", "FIXTURE_CLEAR", "DOCKPIPE_VSCODE_INPUT_CD42404D52AD55CC"};
        const QStringList arguments = SessionManager::dockpipeArguments(context, options)
            + QStringList{"--no-vault", "--no-op-inject", "--no-compile-deps"};
        QProcessEnvironment environment;
        environment.insert("PATH", "/usr/bin:/bin");
        environment.insert("HOME", directory.filePath("home"));
        environment.insert("TMPDIR", directory.filePath("tmp"));
        environment.insert("LANG", "C.UTF-8");
        environment.insert("DOCKPIPE_VAULT_INJECT", "0");
        environment.insert("DOCKPIPE_INTERACTIVE", "0");
        for (const QString &name : {QString("CONFIG"), QString("CACHE"), QString("DATA"), QString("STATE")}) {
            const QString relative = "home/" + name.toLower();
            QVERIFY(directory.mkpath(relative));
            environment.insert("XDG_" + name + "_HOME", directory.filePath(relative));
        }
        const QString stdoutPath = directory.filePath("core.stdout");
        const QString stderrPath = directory.filePath("core.stderr");
        QProcess process;
        process.setProcessEnvironment(environment);
        process.setWorkingDirectory(project);
        process.setStandardOutputFile(stdoutPath);
        process.setStandardErrorFile(stderrPath);
        process.setChildProcessModifier([]() {
            if (::setpgid(0, 0) != 0)
                _exit(127);
        });
        process.start(cli, arguments);
        QVERIFY2(process.waitForStarted(5000), qPrintable(process.errorString()));
        const qint64 pid = process.processId();
        QElapsedTimer deadline;
        deadline.start();
        bool bounded = true;
        while (process.state() != QProcess::NotRunning) {
            if (deadline.elapsed() >= 20000 || QFileInfo(stdoutPath).size() > 1024 * 1024
                || QFileInfo(stderrPath).size() > 1024 * 1024) {
                bounded = false;
                ::kill(-pid, SIGTERM);
                if (!process.waitForFinished(3000)) {
                    ::kill(-pid, SIGKILL);
                    process.waitForFinished(2000);
                }
                break;
            }
            process.waitForFinished(50);
        }
        QVERIFY2(bounded, "Actual core transport exceeded its 20 second or 1MiB diagnostic bound.");
        QVERIFY(QFileInfo(stdoutPath).size() <= 1024 * 1024);
        QVERIFY(QFileInfo(stderrPath).size() <= 1024 * 1024);
        QVERIFY(selectedCli.open(QIODevice::ReadOnly));
        QCryptographicHash finalHash(QCryptographicHash::Sha256);
        QVERIFY(finalHash.addData(&selectedCli));
        QCOMPARE(finalHash.result().toHex(), expectedSha256);
        selectedCli.close();
        QFile diagnostics(stderrPath);
        QVERIFY(diagnostics.open(QIODevice::ReadOnly));
        const QByteArray diagnostic = diagnostics.read(4096);
        QCOMPARE(process.exitStatus(), QProcess::NormalExit);
        QVERIFY2(process.exitCode() == 0, diagnostic.constData());
        QFile effective(directory.filePath("project/effective-inputs.json"));
        QVERIFY(effective.open(QIODevice::ReadOnly));
        QVERIFY(effective.size() <= 4096);
        const QJsonObject values = QJsonDocument::fromJson(effective.readAll()).object();
        QCOMPARE(values.value("mode").toString(), QString("selected"));
        QVERIFY(values.value("clear_present").toBool());
        QCOMPARE(values.value("clear").toString(), QString());
        QCOMPARE(values.value("imported").toString(), QString("  exact value  "));
#endif
    }

    void passiveSnapshotsStayInlineWhileUserWorkflowsNotify()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        QFile project(workspace.filePath("dockpipe.config.json"));
        QVERIFY(project.open(QIODevice::WriteOnly));
        QCOMPARE(project.write("{\"schema\":1}\n"), qint64(13));
        project.close();
        LauncherSettings settings;
        settings.projectFolder = workspace.path();
        settings.uiMode = "advanced";
        settings.thirdPartyDisclaimerDismissed = true;
        settings.defaultDashboardVisible = false;
        settings.save();
        QVERIFY(LauncherSettings::saveDashboardTourState(true, true));

        SessionLaunchOptions defaults;
        QVERIFY(defaults.notifyWorkflowOutcome);
        defaults.notifyWorkflowOutcome = false;
        QVERIFY(!defaults.notifyWorkflowOutcome);
        QVERIFY(!Context::createNew().toJson().contains("notifyWorkflowOutcome"));
        QVERIFY(SessionLaunchOptions{}.notifyWorkflowOutcome);

        MainWindow window;
        qInfo("Main window notification wiring initialized");
        auto *sessions = window.findChild<SessionManager *>();
        auto *notifications = window.findChild<NotificationBell *>();
        QVERIFY(sessions);
        QVERIFY(notifications);
        QCOMPARE(notifications->unreadCount(), 0);
        QSignalSpy stopped(sessions, &SessionManager::sessionStopped);
        QSignalSpy failed(sessions, &SessionManager::sessionFailed);

        PackageSurfacePage page(*sessions);
        ResolverMeta provider;
        provider.name = "fixture-provider";
        provider.surface.id = "fixture.resources";
        provider.surface.title = "Fixture resources";
        provider.surface.snapshot = "snapshot-success";
        page.setWorkspace(workspace.path(), QCoreApplication::applicationFilePath());
        page.setProviders({provider});
        page.show();
        page.setActive(true);
        qInfo("Initial passive snapshot requested");
        QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), 1, 5000);
        auto *poll = page.findChild<QTimer *>("surfacePoll");
        QVERIFY(poll);
        poll->stop();
        for (int iteration = 0; iteration < 4; ++iteration) {
            const int expected = stopped.count() + 1;
            page.refresh();
            QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), expected, 5000);
            const QString id = stopped.last().at(0).toString();
            QVERIFY(!sessions->info(id).notifyWorkflowOutcome);
            QCOMPARE(notifications->unreadCount(), 0);
        }

        auto *status = page.findChild<QLabel *>("surfaceStatus");
        QVERIFY(status);
        QCOMPARE(status->text(), QString("State refreshed."));
        qInfo("Repeated passive snapshots completed without workflow notices");
        provider.surface.snapshot = "snapshot-failure";
        const int expected = stopped.count() + 1;
        page.setProviders({provider});
        QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), expected, 5000);
        QVERIFY(status->text().contains("failed"));
        QVERIFY(status->text().contains("stale"));
        QCOMPARE(notifications->unreadCount(), 0);
        page.setActive(false);
        qInfo("Failed passive snapshot retained its inline error");

        // These direct session fixtures are outside MainWindow's surface registry,
        // so its normal startup-error warning still needs a real dismissal.
        int warningCount = 0;
        QTimer dismissWarnings;
        connect(&dismissWarnings, &QTimer::timeout, this, [&warningCount]() {
            if (auto *warning = qobject_cast<QMessageBox *>(QApplication::activeModalWidget())) {
                ++warningCount;
                warning->accept();
            }
        });
        dismissWarnings.start(10);

        Context observation = Context::createNew();
        observation.workdir = workspace.path();
        observation.workflow = "foreground-success";
        observation.dockpipeBinary = QCoreApplication::applicationFilePath();
        SessionLaunchOptions observationOptions;
        observationOptions.notifyWorkflowOutcome = false;
        const int failuresBefore = failed.count();
        QVERIFY(!sessions->launch(observation, workspace.filePath("missing-logs-directory"), observationOptions));
        QCOMPARE(failed.count(), failuresBefore + 1);
        QCOMPARE(sessions->info(observation.id).status, SessionStatus::Failed);
        QVERIFY(!sessions->info(observation.id).notifyWorkflowOutcome);
        QCOMPARE(notifications->unreadCount(), 0);
        qInfo("Passive log-open failure retained notification metadata");

        for (const QString &workflow : {QString("foreground-success"), QString("foreground-failure")}) {
            Context context = Context::createNew();
            context.workdir = workspace.path();
            context.workflow = workflow;
            context.dockpipeBinary = QCoreApplication::applicationFilePath();
            const int expectedStops = stopped.count() + 1;
            const int expectedNotices = notifications->unreadCount() + 1;
            QVERIFY(sessions->launch(context, workspace.path()));
            QTRY_COMPARE_WITH_TIMEOUT(stopped.count(), expectedStops, 5000);
            QVERIFY(sessions->info(context.id).notifyWorkflowOutcome);
            QCOMPARE(notifications->unreadCount(), expectedNotices);
        }
        Context missing = Context::createNew();
        missing.workdir = workspace.path();
        missing.workflow = "foreground-success";
        missing.dockpipeBinary = workspace.filePath("missing-cli");
        const int expectedNotices = notifications->unreadCount() + 1;
        QVERIFY(sessions->launch(missing, workspace.path()));
        QTRY_COMPARE_WITH_TIMEOUT(notifications->unreadCount(), expectedNotices, 5000);
        QCOMPARE(warningCount, 2);
        qInfo("Foreground success, failure and startup failure all notified");
    }
};

int main(int argc, char **argv)
{
    for (int index = 1; index + 1 < argc; ++index) {
        if (QByteArray(argv[index]) != "--workflow-file" && QByteArray(argv[index]) != "--workflow")
            continue;
        const QByteArray workflow(argv[index + 1]);
        if (workflow == "snapshot-success")
            std::fputs("::dockpipe-surface::{\"schema\":1,\"kind\":\"snapshot\",\"resources\":[]}\n", stdout);
        return workflow.endsWith("failure") ? 7 : 0;
    }
    if (argc > 1 && (QByteArray(argv[1]) == "catalog" || QByteArray(argv[1]) == "remote")) {
        std::fputs("{}\n", stdout);
        return 0;
    }
    QTemporaryDir profile;
    if (!profile.isValid())
        return 2;
    qputenv("XDG_CONFIG_HOME", profile.path().toUtf8());
    qputenv("XDG_STATE_HOME", profile.filePath("state").toUtf8());
    qputenv("XDG_CACHE_HOME", profile.filePath("cache").toUtf8());
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    application.setQuitOnLastWindowClosed(false);
    qputenv("DOCKPIPE_BIN", QFile::encodeName(QCoreApplication::applicationFilePath()));
    QCoreApplication::setApplicationName("session-notification-test");
    QCoreApplication::setOrganizationName("dockpipe-test");
    SessionNotificationTest tests;
    // Match the ordinary GUI event loop while exercising nested error dialogs.
    QTimer::singleShot(0, &application, [&]() {
        application.exit(QTest::qExec(&tests, argc, argv));
    });
    return application.exec();
}

#include "SessionNotificationTest.moc"
