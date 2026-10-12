#include "PackageCommand.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QJsonArray>
#include <QTemporaryDir>
#include <QTimer>
#include <cstdio>

static bool check(bool condition, const char *message)
{
    if (!condition)
        std::fprintf(stderr, "%s\n", message);
    return condition;
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    QTemporaryDir directory;
    if (!directory.isValid())
        return 1;
    const QString script = directory.filePath(QStringLiteral("cli.sh"));
    QFile file(script);
    if (!file.open(QIODevice::WriteOnly))
        return 1;
    file.write("#!/bin/sh\ncase \"$1\" in\n"
               "catalog) printf '%s' '{\"packages\":[{\"name\":\"sample\"}]}' ;;\n"
               "failure) echo 'checksum mismatch' >&2; exit 1 ;;\n"
               "malformed) echo 'not json' ;;\n"
               "slow) exec sleep 10 ;;\n"
               "stubborn) trap '' TERM; exec sleep 10 ;;\nesac\n");
    file.close();
    PackageCommand command;
    bool success = false;
    QString error;
    QJsonObject result;
    QEventLoop loop;
    QObject::connect(&command, &PackageCommand::completed, &loop, [&](const QJsonObject &value) {
        success = true;
        result = value;
        loop.quit();
    });
    QObject::connect(&command, &PackageCommand::failed, &loop, [&](const QString &value) {
        error = value;
        loop.quit();
    });
    auto run = [&](const QString &mode) {
        success = false;
        error.clear();
        command.start(QStringLiteral("/bin/sh"), {script, mode}, directory.path());
        QTimer watchdog;
        watchdog.setSingleShot(true);
        QObject::connect(&watchdog, &QTimer::timeout, &loop, &QEventLoop::quit);
        watchdog.start(5000);
        loop.exec();
    };
    run(QStringLiteral("catalog"));
    if (!check(success && result.value(QStringLiteral("packages")).toArray().size() == 1 && !command.busy(), "catalog response failed"))
        return 1;
    run(QStringLiteral("failure"));
    if (!check(!success && error.contains(QStringLiteral("checksum mismatch")), "CLI failure lost"))
        return 1;
    run(QStringLiteral("malformed"));
    if (!check(!success && error.contains(QStringLiteral("invalid package response")), "malformed output accepted"))
        return 1;
    QElapsedTimer elapsed;
    elapsed.start();
    QTimer::singleShot(20, &command, &PackageCommand::cancel);
    run(QStringLiteral("slow"));
    if (!check(!success && error.contains(QStringLiteral("cancelled")) && elapsed.elapsed() < 3000, "cancellation blocked or lost"))
        return 1;
    elapsed.restart();
    QTimer::singleShot(50, &command, &PackageCommand::cancel);
    run(QStringLiteral("stubborn"));
    if (!check(!success && error.contains(QStringLiteral("cancelled")) && elapsed.elapsed() < 3500 && !command.busy(), "forced cancellation failed"))
        return 1;
    run(QStringLiteral("catalog"));
    if (!check(success, "cannot refresh after cancellation"))
        return 1;
    return 0;
}
