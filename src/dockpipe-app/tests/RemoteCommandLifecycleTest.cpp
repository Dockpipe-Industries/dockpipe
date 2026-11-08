#include "RemoteCommand.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QTemporaryDir>
#include <QTimer>
#include <cstdio>
#ifndef Q_OS_WIN
#include <csignal>
#endif

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QStringList arguments = application.arguments();
    if (arguments.contains("--json-substring")) {
        std::printf("{\"path\":\"hardware-key-not-a-protocol-frame.txt\"}\n");
        return 0;
    }
    if (arguments.contains("--child")) {
#ifndef Q_OS_WIN
        if (arguments.contains("--graceful"))
            std::signal(SIGTERM, SIG_IGN);
#endif
        std::printf("ready\n");
        std::fflush(stdout);
        const QString marker = arguments.value(arguments.indexOf("--marker") + 1);
        const bool writeMarker = arguments.contains("--marker");
        QTimer::singleShot(1400, &application, [marker, writeMarker]() {
            if (writeMarker) {
                QFile file(marker);
                if (file.open(QIODevice::WriteOnly))
                    file.write("survived shutdown");
            }
            std::printf("done\n");
            std::fflush(stdout);
            QCoreApplication::quit();
        });
        return application.exec();
    }
    if (arguments.contains("--shutdown-parent")) {
        RemoteCommand shutdownCommand;
        QObject::connect(&shutdownCommand, &RemoteCommand::standardOutput, &application,
            [&](const QByteArray &bytes) {
                if (bytes.contains("ready"))
                    application.quit();
            });
        shutdownCommand.run(QCoreApplication::applicationFilePath(),
            {"--child", "--graceful", "--marker", arguments.value(arguments.indexOf("--marker") + 1)},
            [](bool, const QByteArray &) {});
        return application.exec();
    }
    {
        RemoteCommand jsonCommand;
        QEventLoop loop;
        bool passed = false;
        QTimer timeout;
        timeout.setSingleShot(true);
        QObject::connect(&timeout, &QTimer::timeout, &loop, &QEventLoop::quit);
        jsonCommand.run(QCoreApplication::applicationFilePath(), {"--json-substring"},
            [&](bool success, const QByteArray &output) {
                passed = success && output.contains("hardware-key-not-a-protocol-frame.txt");
                loop.quit();
            });
        timeout.start(2000);
        loop.exec();
        if (!passed) {
            std::fprintf(stderr, "Ordinary JSON content was mistaken for a security-key frame.\n");
            return 1;
        }
    }
#ifdef Q_OS_WIN
    // Windows console cancellation uses the kill deadline rather than SIGTERM.
    std::printf("This SIGTERM cancellation regression requires a POSIX child.\n");
    return 77;
#else
    {
        RemoteCommand retry;
        QEventLoop loop;
        QTimer timeout;
        timeout.setSingleShot(true);
        QObject::connect(&timeout, &QTimer::timeout, &loop, &QEventLoop::quit);
        bool replacementSucceeded = false;
        retry.run(QString(), {}, [&](bool success, const QByteArray &) {
            if (success) {
                loop.quit();
                return;
            }
            retry.run(QCoreApplication::applicationFilePath(), {"--child"},
                [&](bool replacementSuccess, const QByteArray &output) {
                    replacementSucceeded = replacementSuccess && output.contains("done");
                    loop.quit();
                }, 3000);
        }, 20);
        timeout.start(4000);
        loop.exec();
        if (!replacementSucceeded) {
            std::fprintf(stderr, "A failed start replaced the new command's deadline.\n");
            return 1;
        }
    }
    {
        QTemporaryDir directory;
        const QString marker = directory.filePath("surviving-child");
        QProcess shutdownParent;
        shutdownParent.start(QCoreApplication::applicationFilePath(), {"--shutdown-parent", "--marker", marker});
        if (!shutdownParent.waitForFinished(3000) || shutdownParent.exitCode() != 0)
            return 1;
        QEventLoop settle;
        QTimer::singleShot(1600, &settle, &QEventLoop::quit);
        settle.exec();
        if (QFile::exists(marker)) {
            std::fprintf(stderr, "A command survived its owner's application shutdown.\n");
            return 1;
        }
    }
    bool destructionCallback = false;
    {
        auto *dying = new RemoteCommand;
        QEventLoop ready;
        QTimer timeout;
        timeout.setSingleShot(true);
        bool started = false;
        QObject::connect(&timeout, &QTimer::timeout, &ready, &QEventLoop::quit);
        QObject::connect(dying, &RemoteCommand::standardOutput, &ready, [&](const QByteArray &bytes) {
            if (bytes.contains("ready")) {
                started = true;
                ready.quit();
            }
        });
        dying->run(QCoreApplication::applicationFilePath(), {"--child", "--graceful"},
            [&](bool, const QByteArray &) { destructionCallback = true; });
        timeout.start(2000);
        ready.exec();
        if (!started) {
            delete dying;
            return 1;
        }
        dying->cancel();
        QElapsedTimer destruction;
        destruction.start();
        delete dying;
        if (destruction.elapsed() >= 100) {
            std::fprintf(stderr, "Command destruction blocked the event loop.\n");
            return 1;
        }
    }
    RemoteCommand command;
    QElapsedTimer elapsed;
    elapsed.start();
    bool secondAttempt = false;
    bool passed = false;
    QObject::connect(&command, &RemoteCommand::standardOutput, &application, [&](const QByteArray &bytes) {
        if (bytes.contains("ready"))
            command.cancel();
    });
    command.run(QCoreApplication::applicationFilePath(), {"--child"},
        [&](bool success, const QByteArray &) {
            if (success)
                return;
            QTimer::singleShot(1200, &application, [&]() {
                secondAttempt = true;
                command.run(QCoreApplication::applicationFilePath(), {"--child", "--graceful"},
                    [&](bool secondSuccess, const QByteArray &output) {
                        passed = !secondSuccess && output.contains("done");
                        application.quit();
                    });
            });
        });
    QTimer::singleShot(6000, &application, &QCoreApplication::quit);
    application.exec();
    if (!secondAttempt || !passed || destructionCallback) {
        std::fprintf(stderr, "An old cancellation deadline interrupted the replacement command (%lld ms).\n",
            static_cast<long long>(elapsed.elapsed()));
        return 1;
    }
    return 0;
#endif
}
