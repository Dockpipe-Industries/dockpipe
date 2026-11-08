#include "RemoteWorkflowMonitor.h"

#include <QApplication>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <cstring>
#include <functional>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Remote workflow monitor assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 5000) {
        QApplication::processEvents();
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
    if (arguments != QStringList{"remote", "jobs", "--auth", "never"})
        return 2;
    if (QFile::exists(directory.filePath(QStringLiteral("slow"))))
        QThread::msleep(300);
    if (QFile::exists(directory.filePath(QStringLiteral("fail"))))
        return 1;
    QFile output;
    if (!output.open(stdout, QIODevice::WriteOnly))
        return 2;
    const QByteArray jobs = readFile(directory.filePath(QStringLiteral("jobs")));
    return output.write(jobs) == jobs.size() && output.flush() ? 0 : 2;
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
    const QString jobsPath = directory.filePath(QStringLiteral("jobs"));
    const QString callsPath = directory.filePath(QStringLiteral("calls"));
    CHECK(writeFile(jobsPath, R"([{"id":"tracked","status":"queued"},{"id":"untracked","status":"success"}])"));

    RemoteWorkflowMonitor monitor;
    auto *command = monitor.findChild<RemoteCommand *>();
    CHECK(command);
    QStringList outcomes;
    QObject::connect(&monitor, &RemoteWorkflowMonitor::finished, &application,
                     [&](const QString &id, const QString &title, const QString &outcome) {
        outcomes.append(id + QLatin1Char(':') + title + QLatin1Char(':') + outcome);
    });
    CHECK(monitor.track(QStringLiteral("tracked"), QStringLiteral("Build")));
    monitor.refresh();
    CHECK(!QFile::exists(callsPath));
    monitor.setAccessUnlocked(true);
    monitor.refresh();
    CHECK(waitUntil([&]() { return !command->busy(); }));
    CHECK(outcomes.isEmpty());
    CHECK(readFile(callsPath) == "remote jobs --auth never\n");

    const QByteArray unfinishedJobs[]{
        R"([{"id":"tracked","status":"running"}])",
        R"([{"id":"tracked","status":"unknown"}])",
        "[]", "malformed JSON"
    };
    for (const QByteArray &jobs : unfinishedJobs) {
        CHECK(writeFile(jobsPath, jobs));
        monitor.refresh();
        CHECK(waitUntil([&]() { return !command->busy(); }));
        CHECK(outcomes.isEmpty());
    }

    CHECK(writeFile(jobsPath, R"([{"id":"tracked","status":"success"}])"));
    CHECK(writeFile(directory.filePath(QStringLiteral("fail")), {}));
    monitor.refresh();
    CHECK(waitUntil([&]() { return !command->busy(); }));
    CHECK(outcomes.isEmpty());
    CHECK(QFile::remove(directory.filePath(QStringLiteral("fail"))));
    monitor.refresh();
    CHECK(waitUntil([&]() { return outcomes.size() == 1; }));
    CHECK(outcomes.first() == QStringLiteral("tracked:Build:completed"));
    monitor.refresh();
    CHECK(!command->busy());
    CHECK(outcomes.size() == 1);

    monitor.setAccessUnlocked(false);
    CHECK(monitor.track(QStringLiteral("failure"), QStringLiteral("Test")));
    CHECK(monitor.track(QStringLiteral("cancelled"), QStringLiteral("Deploy")));
    CHECK(writeFile(jobsPath, R"([{"id":"failure","status":"failure"},{"id":"cancelled","status":"cancelled"}])"));
    monitor.setAccessUnlocked(true);
    CHECK(waitUntil([&]() { return outcomes.size() == 3; }));
    CHECK(outcomes.contains(QStringLiteral("failure:Test:failed")));
    CHECK(outcomes.contains(QStringLiteral("cancelled:Deploy:cancelled")));

    CHECK(writeFile(directory.filePath(QStringLiteral("slow")), {}));
    CHECK(writeFile(jobsPath, R"([{"id":"revoked","status":"success"}])"));
    CHECK(monitor.track(QStringLiteral("revoked"), QStringLiteral("Revoked")));
    CHECK(command->busy());
    monitor.setAccessUnlocked(false);
    CHECK(waitUntil([&]() { return !command->busy(); }));
    CHECK(outcomes.size() == 3);
    CHECK(QFile::remove(directory.filePath(QStringLiteral("slow"))));
    monitor.setAccessUnlocked(true);
    CHECK(waitUntil([&]() { return outcomes.size() == 4; }));
    CHECK(outcomes.last() == QStringLiteral("revoked:Revoked:completed"));

    RemoteWorkflowMonitor boundedMonitor;
    CHECK(!boundedMonitor.track({}, QStringLiteral("Empty")));
    for (int index = 0; index < 128; ++index)
        CHECK(boundedMonitor.track(QString::number(index), QStringLiteral("Pending")));
    CHECK(boundedMonitor.track(QStringLiteral("0"), QStringLiteral("Duplicate")));
    CHECK(!boundedMonitor.track(QStringLiteral("overflow"), QStringLiteral("Overflow")));

    std::puts("Remote workflow completion, failure, cancellation, access gating and tracking bounds passed.");
    return 0;
}
