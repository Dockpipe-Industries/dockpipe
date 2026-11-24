#include "SessionManager.h"
#include "DockpipeChoices.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QTemporaryDir>
#include <QTimer>
#include <cstdio>
#include <cstdlib>
#include <functional>
#ifndef Q_OS_WIN
#include <csignal>
#endif

// Every fixture selects its own executable; discovery must not invoke a CLI.
QString DockpipeChoices::preferredDockpipeBinary(const QString &)
{
    return {};
}

namespace {
bool check(bool condition, const char *message)
{
    if (!condition)
        std::fprintf(stderr, "%s\n", message);
    return condition;
}

bool waitFor(const std::function<bool()> &done)
{
    if (done())
        return true;
    QEventLoop loop;
    QTimer poll;
    poll.setInterval(5);
    QObject::connect(&poll, &QTimer::timeout, &loop, [&]() {
        if (done())
            loop.quit();
    });
    QTimer::singleShot(5000, &loop, &QEventLoop::quit);
    poll.start();
    loop.exec();
    return done();
}
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QString startupMarker = QStringLiteral("session-startup-marker");
    if (application.arguments().contains(startupMarker)) {
        QFile marker(startupMarker);
        return marker.open(QIODevice::WriteOnly) ? 0 : 1;
    }

    if (application.arguments().contains(QStringLiteral("session-lifecycle-child"))) {
#ifndef Q_OS_WIN
        // Simulate a workflow which needs time to finish its graceful teardown.
        std::signal(SIGTERM, SIG_IGN);
#endif
        QTimer::singleShot(50, &application, []() {
            std::printf("[dockpipe-ready]\n");
            std::fflush(stdout);
        });
        QTimer::singleShot(650, &application, &QCoreApplication::quit);
        return application.exec();
    }

    if (application.arguments().contains(QStringLiteral("session-crash-child"))) {
        QTimer::singleShot(10, &application, []() { std::abort(); });
        return application.exec();
    }

    QTemporaryDir directory;
    if (!directory.isValid())
        return 1;
    Context context = Context::createNew();
    context.workdir = directory.path();
    context.dockpipeBinary = QCoreApplication::applicationFilePath();
    context.workflow = QStringLiteral("session-lifecycle-child");
    SessionManager manager;
    int starts = 0;
    int stops = 0;
    int failures = 0;
    int heartbeats = 0;
    bool cancellationReturned = false;
    bool inputReturned = false;
    bool cancelOnOutput = true;
    QTimer heartbeat;
    heartbeat.setInterval(10);
    QObject::connect(&heartbeat, &QTimer::timeout, &application, [&]() { ++heartbeats; });
    QObject::connect(&manager, &SessionManager::sessionStarted, &application, [&](const QString &) { ++starts; });
    QObject::connect(&manager, &SessionManager::sessionStopped, &application,
                     [&](const QString &, int, QProcess::ExitStatus) { ++stops; });
    QObject::connect(&manager, &SessionManager::sessionFailed, &application,
                     [&](const QString &, const QString &) { ++failures; });
    QObject::connect(&manager, &SessionManager::sessionOutput, &application,
                     [&](const QString &id, const QString &) {
        if (!cancelOnOutput)
            return;
        cancelOnOutput = false;
        QElapsedTimer elapsed;
        elapsed.start();
        // This child never reads stdin; writing must remain asynchronous.
        manager.sendInput(id, QString(1024 * 1024, QLatin1Char('x')));
        inputReturned = elapsed.elapsed() < 100;
        elapsed.restart();
        manager.stop(id);
        manager.stop(id);
        cancellationReturned = elapsed.elapsed() < 100;
    });
    heartbeat.start();
    if (!check(manager.launch(context, directory.path()), "launch request was not admitted"))
        return 1;
    if (!check(manager.info(context.id).status == SessionStatus::Starting
               && manager.isRunning(context.id) && starts == 0,
               "launch synchronously waited for process startup"))
        return 1;
    if (!check(!manager.launch(context, directory.path()), "duplicate launch was admitted"))
        return 1;
    if (!check(waitFor([&]() { return stops == 1; }), "cancelled session did not finish"))
        return 1;
    if (!check(cancellationReturned && inputReturned && heartbeats > 15 && failures == 0
               && starts == 1 && manager.info(context.id).status == SessionStatus::Stopped,
               "interactive cancellation/input blocked, reentered teardown, or lost terminal state"))
        return 1;

    // Cancellation before the queued start must complete without spawning a child.
    context.workflow = startupMarker;
    if (!check(manager.launch(context, directory.path()), "retry after cancellation was rejected"))
        return 1;
    manager.stop(context.id);
    manager.stop(context.id);
    if (!check(stops == 1 && waitFor([&]() { return stops == 2; })
               && starts == 1 && failures == 0 && !manager.isRunning(context.id)
               && manager.info(context.id).status == SessionStatus::Stopped
               && !QFile::exists(directory.filePath(startupMarker)),
               "cancellation before startup spawned a child or lost completion"))
        return 1;

    context.workflow = QStringLiteral("session-crash-child");
    if (!check(manager.launch(context, directory.path()), "crash fixture was not admitted"))
        return 1;
    if (!check(waitFor([&]() { return stops == 3; }) && failures == 1
               && manager.info(context.id).status == SessionStatus::Failed
               && !manager.info(context.id).errorString.isEmpty(),
               "runtime crash lost its failure/stop signals or final diagnostic"))
        return 1;
    return 0;
}
