#include "RemoteAccessMonitor.h"
#include "NotificationBell.h"
#include "Theme.h"
#include <QDialog>
#include <QMenu>
#include <QAction>
#include <QMainWindow>
#include <QMenuBar>
#include <QPushButton>
#include <QVBoxLayout>

#include <QApplication>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPointer>
#include <QProcess>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <cstring>
#include <functional>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Remote access monitor assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 5000) {
        QApplication::processEvents();
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

static QByteArray readFile(const QString &path)
{
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.readAll();
}

static bool writeFile(const QString &path, const QByteArray &contents)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(contents) == contents.size();
}

static int runFixture()
{
    const QDir directory(qEnvironmentVariable("DOCKPIPE_TEST_ROOT"));
    const QStringList arguments = QCoreApplication::arguments().mid(1);
    QFile calls(directory.filePath(QStringLiteral("calls")));
    if (!calls.open(QIODevice::WriteOnly | QIODevice::Append))
        return 2;
    calls.write(arguments.join(QLatin1Char(' ')).toUtf8() + '\n');
    calls.close();

    const QString command = arguments.value(1);
    if (command == QStringLiteral("unlock")) {
        if (!writeFile(directory.filePath(QStringLiteral("unlock-started")), "yes"))
            return 2;
        if (QFile::exists(directory.filePath(QStringLiteral("hold-unlock"))))
            QThread::sleep(30);
        if (QFile::exists(directory.filePath(QStringLiteral("fail-unlock")))) {
            std::fputs("Fixture authentication was cancelled.\n", stderr);
            return 1;
        }
        return writeFile(directory.filePath(QStringLiteral("status")),
                         R"({"supported":true,"authority_available":true,"locked":false})") ? 0 : 2;
    }

    if (command != QStringLiteral("security-status"))
        return 2;
    if (QFile::exists(directory.filePath(QStringLiteral("slow-status"))))
        QThread::msleep(150);
    if (QFile::exists(directory.filePath(QStringLiteral("fail-status")))) {
        std::fputs("Fixture authority cannot be reached.\n", stderr);
        return 1;
    }
    QFile output;
    if (!output.open(stdout, QIODevice::WriteOnly))
        return 2;
    const QByteArray status = readFile(directory.filePath(QStringLiteral("status")));
    return output.write(status) == status.size() && output.flush() ? 0 : 2;
}

int main(int argc, char **argv)
{
    if (argc > 1 && std::strcmp(argv[1], "remote") == 0) {
        QCoreApplication application(argc, argv);
        return runFixture();
    }

    QApplication application(argc, argv);
    QTemporaryDir directory;
    CHECK(directory.isValid());
    qputenv("DOCKPIPE_TEST_ROOT", directory.path().toUtf8());
#ifdef Q_OS_WIN
    const QString cliName = QStringLiteral("dockpipe.exe");
#else
    const QString cliName = QStringLiteral("dockpipe");
#endif
    const QString cliPath = directory.filePath(cliName);
    CHECK(QFile::copy(QCoreApplication::applicationFilePath(), cliPath));
    CHECK(QFile::setPermissions(cliPath, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    qputenv("DOCKPIPE_BIN", cliPath.toUtf8());
    const QString statusPath = directory.filePath(QStringLiteral("status"));
    const QByteArray lockedStatus = R"({"supported":true,"authority_available":true,"locked":true,"detail":"Fixture access is locked."})";
    CHECK(writeFile(statusPath, lockedStatus));
    CHECK(writeFile(directory.filePath(QStringLiteral("slow-status")), {}));

    RemoteAccessMonitor monitor;
    CHECK(monitor.state() == RemoteAccessMonitor::State::Checking);
    int unlockCompletions = 0;
    bool unlockSucceeded = false;
    QObject::connect(&monitor, &RemoteAccessMonitor::unlockFinished, &application, [&](bool succeeded) {
        ++unlockCompletions;
        unlockSucceeded = succeeded;
    });
    monitor.refresh();
    CHECK(monitor.busy());
    monitor.refresh();
    monitor.unlock();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(monitor.state() == RemoteAccessMonitor::State::Locked);
    CHECK(monitor.detail().startsWith(QStringLiteral("Remote access is locked. Authenticate to continue.")));
    CHECK(monitor.detail().contains(QStringLiteral("Fixture access is locked.")));
    const QString callsPath = directory.filePath(QStringLiteral("calls"));
    CHECK(readFile(callsPath) == "remote security-status --auth never\n");
    CHECK(unlockCompletions == 0);
    CHECK(QFile::remove(directory.filePath(QStringLiteral("slow-status"))));

    monitor.unlock();
    CHECK(waitUntil([&]() { return unlockCompletions == 1; }));
    CHECK(unlockSucceeded && !monitor.busy());
    CHECK(monitor.state() == RemoteAccessMonitor::State::Unlocked);
    CHECK(monitor.detail() == QStringLiteral("Remote access is unlocked."));
    CHECK(readFile(callsPath).endsWith("remote unlock --auth auto\nremote security-status --auth never\n"));

    CHECK(writeFile(statusPath, lockedStatus));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(monitor.state() == RemoteAccessMonitor::State::Locked);
    CHECK(writeFile(directory.filePath(QStringLiteral("fail-unlock")), {}));
    monitor.unlock();
    CHECK(waitUntil([&]() { return unlockCompletions == 2; }));
    CHECK(!unlockSucceeded && monitor.state() == RemoteAccessMonitor::State::Locked);
    CHECK(monitor.detail().contains(QStringLiteral("Fixture authentication was cancelled.")));
    const int failedUnlockCount = readFile(callsPath).count("remote unlock");
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(readFile(callsPath).count("remote unlock") == failedUnlockCount);
    CHECK(QFile::remove(directory.filePath(QStringLiteral("fail-unlock"))));

    CHECK(writeFile(directory.filePath(QStringLiteral("hold-unlock")), {}));
    CHECK(QFile::remove(directory.filePath(QStringLiteral("unlock-started"))));
    monitor.unlock();
    CHECK(waitUntil([&]() { return QFile::exists(directory.filePath(QStringLiteral("unlock-started"))); }));
    monitor.cancelUnlock();
    CHECK(waitUntil([&]() { return unlockCompletions == 3; }));
    CHECK(!unlockSucceeded && monitor.state() == RemoteAccessMonitor::State::Locked);
    CHECK(QFile::remove(directory.filePath(QStringLiteral("hold-unlock"))));

    CHECK(writeFile(statusPath, R"({"supported":false,"authority_available":true,"locked":true})"));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(monitor.state() == RemoteAccessMonitor::State::Locked);
    CHECK(monitor.detail().contains(QStringLiteral("native lock capabilities are unavailable")));

    const QByteArray unavailableStatuses[]{
        R"({"supported":false,"authority_available":false,"locked":true,"detail":"Fixture authority unavailable."})",
        R"({"supported":true,"authority_available":true})",
        R"({"supported":true,"authority_available":true,"locked":"false"})",
        "not JSON"
    };
    for (const QByteArray &status : unavailableStatuses) {
        CHECK(writeFile(statusPath, status));
        monitor.refresh();
        CHECK(waitUntil([&]() { return !monitor.busy(); }));
        CHECK(monitor.state() == RemoteAccessMonitor::State::Unavailable);
        CHECK(!monitor.detail().isEmpty());
    }

    CHECK(writeFile(directory.filePath(QStringLiteral("fail-status")), {}));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(monitor.state() == RemoteAccessMonitor::State::Unavailable);
    CHECK(monitor.detail().contains(QStringLiteral("Fixture authority cannot be reached.")));
    CHECK(QFile::remove(directory.filePath(QStringLiteral("fail-status"))));

    // Destruction owns and terminates an in-flight CLI without invoking callbacks.
    CHECK(writeFile(directory.filePath(QStringLiteral("hold-unlock")), {}));
    CHECK(QFile::remove(directory.filePath(QStringLiteral("unlock-started"))));
    auto *transientMonitor = new RemoteAccessMonitor;
    transientMonitor->unlock();
    CHECK(waitUntil([&]() { return QFile::exists(directory.filePath(QStringLiteral("unlock-started"))); }));
    QPointer<QProcess> unlockProcess = transientMonitor->findChild<QProcess *>(QStringLiteral("remoteCommandProcess"));
    CHECK(unlockProcess && unlockProcess->state() == QProcess::Running);

    QElapsedTimer shutdownTimer;
    shutdownTimer.start();
    delete transientMonitor;
    CHECK(shutdownTimer.elapsed() < 3000);
    // Destruction returns before the detached process finishes. Keep the test's
    // event loop servicing its cleanup and prove it released the fixture binary.
    CHECK(waitUntil([&]() { return unlockProcess.isNull(); }));

    qputenv("DOCKPIPE_BIN", directory.filePath(QStringLiteral("missing-cli")).toUtf8());
    monitor.refresh();
    CHECK(!monitor.busy() && monitor.state() == RemoteAccessMonitor::State::Unavailable);
    monitor.unlock();
    CHECK(unlockCompletions == 4 && !unlockSucceeded);

    // Visiting Machines prompts once. Dismissing it never starts authentication;
    // successful explicit authentication clears both the notice and the bell.
    CHECK(QFile::remove(directory.filePath(QStringLiteral("hold-unlock"))));
    qputenv("DOCKPIPE_BIN", cliPath.toUtf8());
    CHECK(writeFile(statusPath, lockedStatus));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    applyDockpipeLauncherTheme(application);
    previewDockpipeLauncherTheme(application, qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
        ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    QMainWindow host;
    host.resize(880, 440);
    host.menuBar()->setNativeMenuBar(false);
    host.menuBar()->addMenu("Settings");
    auto *indicator = new NotificationBell(monitor, host.menuBar());
    host.menuBar()->setCornerWidget(indicator, Qt::TopRightCorner);
    auto *machines = new QWidget;
    auto *layout = new QVBoxLayout(machines);
    auto *notice = indicator->createMachinesNotice(machines);
    layout->addWidget(notice);
    layout->addStretch();
    host.setCentralWidget(machines);
    host.show();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(indicator->isVisible());
    CHECK(host.menuBar()->cornerWidget(Qt::TopRightCorner)->isAncestorOf(indicator)
        || host.menuBar()->cornerWidget(Qt::TopRightCorner) == indicator);
    CHECK(indicator->property("needsAttention").toBool());
    CHECK(notice->isVisible());
    const QString images = qEnvironmentVariable("DOCKPIPE_REMOTE_ACCESS_IMAGES");
    if (!images.isEmpty())
        CHECK(host.grab().save(images + "-locked.png"));
    const int unlocksBeforeVisit = readFile(callsPath).count("remote unlock");
    indicator->machinesEntered();
    CHECK(waitUntil([&]() { return host.findChild<QDialog *>("remoteUnlockDialog"); }));
    QPointer<QDialog> prompt = host.findChild<QDialog *>("remoteUnlockDialog");
    CHECK(prompt && prompt->isVisible());
    if (!images.isEmpty())
        CHECK(prompt->grab().save(images + "-prompt.png"));
    prompt->reject();
    CHECK(waitUntil([&]() { return prompt.isNull(); }));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(!host.findChild<QDialog *>("remoteUnlockDialog"));
    CHECK(readFile(callsPath).count("remote unlock") == unlocksBeforeVisit);
    indicator->machinesEntered();
    CHECK(waitUntil([&]() { return host.findChild<QDialog *>("remoteUnlockDialog"); }));
    auto *confirm = host.findChild<QPushButton *>("remoteUnlockConfirm");
    CHECK(confirm && confirm->isEnabled());
    confirm->click();
    CHECK(waitUntil([&]() { return !monitor.busy() && monitor.state() == RemoteAccessMonitor::State::Unlocked; }));
    CHECK(waitUntil([&]() { return !host.findChild<QDialog *>("remoteUnlockDialog"); }));
    CHECK(!indicator->property("needsAttention").toBool());
    CHECK(!notice->isVisible());
    indicator->machinesEntered();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(!host.findChild<QDialog *>("remoteUnlockDialog"));
    CHECK(readFile(callsPath).count("remote unlock") == unlocksBeforeVisit + 1);
    CHECK(writeFile(statusPath, R"({"supported":false,"authority_available":false,"locked":true})"));
    indicator->machinesEntered();
    CHECK(waitUntil([&]() { return !monitor.busy(); }));
    CHECK(notice->isVisible());
    CHECK(!host.findChild<QDialog *>("remoteUnlockDialog"));
    CHECK(indicator->text() == "Remote unavailable");
    indicator->machinesLeft();
    int openedLogs = 0;
    QObject::connect(indicator, &NotificationBell::workflowLogsRequested, &host,
        [&](const QString &title, const QString &path) {
            if (title == "Release build" && path == "/tmp/release-build.log")
                ++openedLogs;
        });
    indicator->notifyWorkflow("Release build", "/tmp/release-build.log", NotificationBell::WorkflowOutcome::Completed);
    CHECK(indicator->unreadCount() == 1);
    CHECK(indicator->text().endsWith(" · 1"));
    CHECK(QMetaObject::invokeMethod(indicator->menu(), "aboutToShow", Qt::DirectConnection));
    auto *completion = indicator->menu()->findChild<QAction *>("workflowNotification");
    CHECK(completion && completion->text().contains("Completed"));
    completion->trigger();
    CHECK(openedLogs == 1 && indicator->unreadCount() == 0);
    for (int index = 0; index < 55; ++index)
        indicator->notifyWorkflow(QString("Build %1").arg(index), QString(), NotificationBell::WorkflowOutcome::Failed);
    CHECK(indicator->unreadCount() == 50);
    CHECK(QMetaObject::invokeMethod(indicator->menu(), "aboutToShow", Qt::DirectConnection));
    CHECK(indicator->menu()->findChildren<QAction *>("workflowNotification").size() == 8);
    bool markedRead = false;
    for (auto *action : indicator->menu()->actions()) {
        if (action->text() == "Mark all as read") {
            action->trigger();
            markedRead = true;
        }
    }
    CHECK(markedRead && indicator->unreadCount() == 0);
    QString selectedRemoteJob;
    QObject::connect(indicator, &NotificationBell::remoteJobRequested, &host,
        [&](const QString &job) { selectedRemoteJob = job; });
    indicator->notifyRemoteWorkflow("Remote build", "job-fixture", NotificationBell::WorkflowOutcome::Failed);
    CHECK(QMetaObject::invokeMethod(indicator->menu(), "aboutToShow", Qt::DirectConnection));
    indicator->menu()->findChild<QAction *>("workflowNotification")->trigger();
    CHECK(selectedRemoteJob == "job-fixture" && indicator->unreadCount() == 0);
    if (!images.isEmpty()) {
        indicator->menu()->popup(host.mapToGlobal(QPoint(420, 40)));
        QApplication::processEvents();
        CHECK(indicator->menu()->grab().save(images + "-inbox.png"));
        indicator->menu()->hide();
    }


    std::puts("Remote access status, noninteractive checks, unlock reconciliation and cancellation passed.");
    return 0;
}
