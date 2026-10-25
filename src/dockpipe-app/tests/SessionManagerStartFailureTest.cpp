#include "SessionManager.h"
#include "DockpipeChoices.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QEvent>
#include <QEventLoop>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>

// This fixture always supplies its own binary. Discovery must not launch the CLI.
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
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().contains(QStringLiteral("session-regression-child"))) {
        std::printf("[dockpipe-ready]\n");
        return 0;
    }
    QTemporaryDir directory;
    if (!directory.isValid())
        return 1;
    Context context = Context::createNew();
    context.workdir = directory.path();
    context.dockpipeBinary = directory.filePath(QStringLiteral("missing-executable"));
    context.workflow = QStringLiteral("session-regression-child");
    SessionManager manager;
    int failures = 0;
    int starts = 0;
    int stops = 0;
    bool consistentFailureState = true;
    bool retryFromFailure = false;
    bool retryStarted = false;
    QObject::connect(&manager, &SessionManager::sessionFailed, &application,
                     [&](const QString &id, const QString &error) {
        ++failures;
        const SessionInfo info = manager.info(id);
        consistentFailureState = consistentFailureState && id == context.id && !error.isEmpty()
            && info.status == SessionStatus::Failed && info.errorString == error && info.pid == 0
            && !info.ready && !manager.isRunning(id);
        if (retryFromFailure) {
            retryFromFailure = false;
            Context retry = context;
            retry.dockpipeBinary = QCoreApplication::applicationFilePath();
            retryStarted = manager.launch(retry, directory.path());
        }
    });
    QObject::connect(&manager, &SessionManager::sessionStarted, &application, [&](const QString &) { ++starts; });
    QObject::connect(&manager, &SessionManager::sessionStopped, &application,
                     [&](const QString &, int, QProcess::ExitStatus) { ++stops; });

    for (int attempt = 1; attempt <= 2; ++attempt) {
        if (!check(!manager.launch(context, directory.path()), "missing executable unexpectedly launched"))
            return 1;
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        if (!check(failures == attempt && starts == 0 && stops == 0 && consistentFailureState,
                   "failed start must publish one failure with final state and no start/stop events"))
            return 1;
    }

    retryFromFailure = true;
    if (!check(!manager.launch(context, directory.path()) && retryStarted,
               "failure handler could not retry the same context"))
        return 1;
    QElapsedTimer elapsed;
    elapsed.start();
    while (stops == 0 && elapsed.elapsed() < 5000) {
        QCoreApplication::processEvents(QEventLoop::AllEvents, 20);
        QThread::msleep(5);
    }
    if (!check(failures == 3 && starts == 1 && stops == 1 && consistentFailureState,
               "outer failed-start cleanup interfered with the replacement session"))
        return 1;
    if (!check(manager.info(context.id).status == SessionStatus::Stopped && !manager.isRunning(context.id),
               "replacement session did not stop cleanly"))
        return 1;
    return 0;
}
