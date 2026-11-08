#include "LauncherEnvironment.h"

#include <QCoreApplication>
#include <QProcess>
#include <QStringList>
#include <QTextStream>

int main(int argc, char *argv[])
{
    QCoreApplication app(argc, argv);
    if (app.arguments().contains(QStringLiteral("--child"))) {
        QTextStream(stdout) << qEnvironmentVariable("PATH") << '\n'
                            << qEnvironmentVariable("DOCKER_CONTEXT") << '\n'
                            << qEnvironmentVariable("DOCKER_HOST") << '\n'
                            << qEnvironmentVariable("DOCKER_CONFIG") << '\n';
        return 0;
    }

    QProcessEnvironment startup;
    startup.insert("DOCKER_CONTEXT", "operator-profile");
    startup.insert("DESKTOP_STARTUP_ID", "consumed-startup-id");
    startup.insert("XDG_ACTIVATION_TOKEN", "consumed-activation-token");
    qputenv("DOCKPIPE_GLOBAL_ROOT", "/saved/preference");
    qputenv("QT_QUICK_BACKEND", "software");
    const auto restart = launcherRestartEnvironment(startup);
    if (restart.contains("DOCKPIPE_GLOBAL_ROOT") || restart.contains("QT_QUICK_BACKEND")
        || restart.value("DOCKER_CONTEXT") != "operator-profile") {
        QTextStream(stderr) << "Restart promoted application defaults into operator overrides\n";
        return 1;
    }
#ifdef Q_OS_LINUX
    if (restart.contains("DESKTOP_STARTUP_ID") || restart.contains("XDG_ACTIVATION_TOKEN"))
        return 1;
#endif
    startup.insert("DOCKPIPE_GLOBAL_ROOT", "/operator/root");
    startup.insert("QT_QUICK_BACKEND", "operator-backend");
    const auto explicitRestart = launcherRestartEnvironment(startup);
    if (explicitRestart.value("DOCKPIPE_GLOBAL_ROOT") != "/operator/root"
        || explicitRestart.value("QT_QUICK_BACKEND") != "operator-backend")
        return 1;

    const QStringList originalPaths = {
        QStringLiteral("/usr/bin:/bin:/usr/sbin:/sbin"),
        QStringLiteral("/custom tools/bin:/usr/local/bin:/opt/homebrew/bin:/usr/bin"),
        QStringLiteral("/custom tools/bin::/usr/bin:"),
        QString(),
    };
    for (const QString &original : originalPaths) {
        qputenv("PATH", original.toLocal8Bit());
        qputenv("DOCKER_CONTEXT", "selected-profile");
        qputenv("DOCKER_HOST", "unix:///custom/engine.sock");
        qputenv("DOCKER_CONFIG", "/custom docker config");
        extendMacOSExecutablePath();
        const QString path = qEnvironmentVariable("PATH");
        const QStringList entries = path.split(QLatin1Char(':'));
        if ((!original.isEmpty() && !path.startsWith(original))
            || entries.count(QStringLiteral("/opt/homebrew/bin")) != 1
            || entries.count(QStringLiteral("/usr/local/bin")) != 1
            || !entries.contains(QStringLiteral("/bin"))) {
            QTextStream(stderr) << "Invalid executable search path: " << path << '\n';
            return 1;
        }
        extendMacOSExecutablePath();
        if (qEnvironmentVariable("PATH") != path) {
            QTextStream(stderr) << "Executable search path changed on repeated setup\n";
            return 1;
        }

        QProcess child;
        child.start(QCoreApplication::applicationFilePath(), {QStringLiteral("--child")});
        if (!child.waitForStarted() || !child.waitForFinished(10000)
            || child.exitStatus() != QProcess::NormalExit || child.exitCode() != 0) {
            QTextStream(stderr) << "Cannot check child environment: " << child.errorString() << '\n';
            return 1;
        }
        const QByteArray expected = path.toLocal8Bit()
            + "\nselected-profile\nunix:///custom/engine.sock\n/custom docker config\n";
        QByteArray output = child.readAllStandardOutput();
        output.replace("\r\n", "\n");
        if (output != expected) {
            QTextStream(stderr) << "Child did not inherit PATH and Docker endpoint settings\n";
            return 1;
        }
    }
    return 0;
}
