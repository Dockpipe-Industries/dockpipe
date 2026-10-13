#include "FlatpakTerminal.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QProcess>
#include <QProcessEnvironment>
#include <QTemporaryDir>
#include <QElapsedTimer>
#include <QThread>
#include <cstdio>

static QByteArray readFile(const QString &path)
{
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.readAll();
}

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir temporary;
    if (!temporary.isValid())
        return 1;
    const QString home = temporary.filePath(QStringLiteral("home with spaces"));
    if (QFile::exists(FlatpakTerminal::wrapperPath(home)) || !FlatpakTerminal::install(home).success)
        return 2;
    const QString wrapper = FlatpakTerminal::wrapperPath(home);
    const QByteArray original = readFile(wrapper);
    if (!FlatpakTerminal::install(home).success || readFile(wrapper) != original)
        return 4;

    const QString fixtureBin = temporary.filePath(QStringLiteral("fixture-bin"));
    QDir().mkpath(fixtureBin);
    QFile flatpak(QDir(fixtureBin).filePath(QStringLiteral("flatpak")));
    if (!flatpak.open(QIODevice::WriteOnly))
        return 5;
    flatpak.write("#!/bin/sh\nprintf '%s\\n' \"$@\"\npwd\nexit 23\n");
    flatpak.close();
    flatpak.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    QProcess process;
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert(QStringLiteral("PATH"), fixtureBin + QStringLiteral(":/usr/bin:/bin"));
    process.setProcessEnvironment(environment);
    const QString project = temporary.filePath(QStringLiteral("Recordings/Final Cuts $(must-not-run)"));
    if (!QDir().mkpath(project))
        return 23;
    process.setWorkingDirectory(project);
    process.start(wrapper, {QStringLiteral("--version"), QStringLiteral("two words"),
                           QStringLiteral("$(must-not-run)"), QStringLiteral(""), QStringLiteral("*.yml")});
    if (!process.waitForFinished(5000) || process.exitCode() != 23)
        return 6;
    const QByteArray expected = QByteArray("run\n--command=dockpipe\n--cwd=")
        + project.toUtf8() + "\n--filesystem=" + project.toUtf8()
        + "\ncom.dockpipe.Dockpipe\n--version\ntwo words\n$(must-not-run)\n\n*.yml\n"
        + project.toUtf8() + '\n';
    if (process.readAllStandardOutput() != expected)
        return 7;

    // Resolve a shell's logical symlink path to the real host directory before
    // exposing it to Flatpak. The sandbox must receive that exact same cwd.
    const QString linkedProject = temporary.filePath(QStringLiteral("project-link"));
    if (!QFile::link(project, linkedProject))
        return 24;
    environment.insert(QStringLiteral("PWD"), linkedProject);
    process.setProcessEnvironment(environment);
    process.setWorkingDirectory(linkedProject);
    process.start(wrapper, {QStringLiteral("init")});
    if (!process.waitForFinished(5000) || process.exitCode() != 23
        || !process.readAllStandardOutput().contains("--cwd=" + project.toUtf8() + '\n'))
        return 25;

    // A disappeared working directory must fail before Flatpak can fall back.
    const QString deletedProject = temporary.filePath(QStringLiteral("removed-project"));
    QDir().mkpath(deletedProject);
    process.setWorkingDirectory(temporary.path());
    process.start(QStringLiteral("/bin/sh"), {QStringLiteral("-c"),
        QStringLiteral("cd \"$1\" && rmdir \"$1\" && exec \"$2\" init"),
        QStringLiteral("removed-directory-test"), deletedProject, wrapper});
    if (!process.waitForFinished(5000) || process.exitCode() == 0
        || process.exitCode() == 23 || !process.readAllStandardOutput().isEmpty())
        return 26;

    // Existing users already have the old shortcut on PATH. Upgrade only its
    // exact known contents; a native command or modified shortcut remains owned
    // by the user.
    QFile legacy(wrapper);
    if (!legacy.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return 27;
    legacy.write("#!/bin/sh\n"
        "# Dockpipe Flatpak terminal command. Remove this file to uninstall the shortcut.\n"
        "exec flatpak run --command=dockpipe com.dockpipe.Dockpipe \"$@\"\n");
    legacy.close();
    const auto permissions = QFile::permissions(wrapper);
    FlatpakTerminal::HostEnvironment existingHost{home, wrapper, QDir(home).filePath(QStringLiteral(".local/bin")),
        QStringLiteral("/bin/bash"), QString(), QString()};
    if (!FlatpakTerminal::completeSetup(existingHost).success || readFile(wrapper) != original
        || QFile::permissions(wrapper) != permissions)
        return 28;

    QFile native(wrapper);
    if (!native.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return 8;
    native.write("native-cli-placeholder\n");
    native.close();
    if (FlatpakTerminal::install(home).success || FlatpakTerminal::completeSetup(existingHost).success
        || readFile(wrapper) != "native-cli-placeholder\n")
        return 9;
    QFile::remove(wrapper);
    if (!QFile::link(temporary.filePath(QStringLiteral("missing-target")), wrapper)
        || FlatpakTerminal::install(home).success)
        return 10;
    QFile::remove(wrapper);
    QDir().mkdir(wrapper);
    if (FlatpakTerminal::install(home).success || FlatpakTerminal::install(QStringLiteral("relative")).success)
        return 11;
    QDir().rmdir(wrapper);

    for (const QString &shell : {QStringLiteral("bash"), QStringLiteral("zsh"), QStringLiteral("fish")}) {
        const QString shellHome = temporary.filePath(shell);
        const QString configHome = QDir(shellHome).filePath(QStringLiteral(".config"));
        FlatpakTerminal::HostEnvironment host{shellHome, QString(), QStringLiteral("/usr/bin:/bin"),
            QStringLiteral("/bin/") + shell, configHome, shellHome};
        if (!FlatpakTerminal::completeSetup(host).success)
            return 12;
        const QString startup = shell == QStringLiteral("fish")
            ? QDir(configHome).filePath(QStringLiteral("fish/conf.d/dockpipe.fish"))
            : QDir(shellHome).filePath(QStringLiteral(".") + shell + QStringLiteral("rc"));
        const QByteArray contents = readFile(startup);
        if (!contents.contains("# Dockpipe Flatpak terminal PATH")
            || !FlatpakTerminal::completeSetup(host).success || readFile(startup) != contents)
            return 13;
    }
    const QString preserveHome = temporary.filePath(QStringLiteral("preserve"));
    QDir().mkpath(preserveHome);
    QFile bashrc(QDir(preserveHome).filePath(QStringLiteral(".bashrc")));
    if (!bashrc.open(QIODevice::WriteOnly))
        return 14;
    bashrc.write("# Existing user settings\nexport USER_SETTING=kept\n");
    bashrc.close();
    FlatpakTerminal::HostEnvironment preserve{preserveHome, QString(), QStringLiteral("/usr/bin:/bin"),
        QStringLiteral("/bin/bash"), QString(), QString()};
    if (!FlatpakTerminal::completeSetup(preserve).success
        || !readFile(bashrc.fileName()).startsWith("# Existing user settings\nexport USER_SETTING=kept\n"))
        return 15;
    QProcess shell;
    auto shellEnvironment = QProcessEnvironment::systemEnvironment();
    shellEnvironment.insert(QStringLiteral("HOME"), preserveHome);
    // Keep an installed host CLI from shadowing the fixture's new shortcut.
    shellEnvironment.insert(QStringLiteral("PATH"), fixtureBin);
    shell.setProcessEnvironment(shellEnvironment);
    shell.start(QStringLiteral("/bin/bash"), {QStringLiteral("--rcfile"), bashrc.fileName(),
        QStringLiteral("-ic"), QStringLiteral("command -v dockpipe; printf '%s' \"$USER_SETTING\"")});
    if (!shell.waitForFinished(5000) || shell.exitCode() != 0
        || shell.readAllStandardOutput() != FlatpakTerminal::wrapperPath(preserveHome).toUtf8() + "\nkept")
        return 16;

    const QString nativeHome = temporary.filePath(QStringLiteral("native"));
    FlatpakTerminal::HostEnvironment nativeHost{nativeHome, QStringLiteral("/usr/bin/dockpipe"),
        QStringLiteral("/usr/bin:/bin"), QStringLiteral("/bin/bash"), QString(), QString()};
    if (!FlatpakTerminal::completeSetup(nativeHost).success || QFile::exists(nativeHome))
        return 17;

    // Exercise the actual launcher entry point, with a temporary host fixture.
    // No user action or Settings dialog participates in terminal setup.
    const QString autoHome = temporary.filePath(QStringLiteral("automatic"));
    QDir().mkpath(autoHome);
    QFile bridge(QDir(fixtureBin).filePath(QStringLiteral("flatpak-spawn")));
    if (!bridge.open(QIODevice::WriteOnly))
        return 18;
    bridge.write("#!/bin/sh\nprintf '%s\\0' \"$DOCKPIPE_TERMINAL_TEST_HOME\" '' '/usr/bin:/bin' '/bin/bash' \"$DOCKPIPE_TERMINAL_TEST_HOME/.config\" \"$DOCKPIPE_TERMINAL_TEST_HOME\"\n");
    bridge.close();
    bridge.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    QFile cli(QDir(fixtureBin).filePath(QStringLiteral("test-cli")));
    if (!cli.open(QIODevice::WriteOnly))
        return 19;
    cli.write("#!/bin/sh\necho '{}'\n");
    cli.close();
    cli.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    if (argc != 2)
        return 20;
    auto startupEnvironment = QProcessEnvironment::systemEnvironment();
    startupEnvironment.insert(QStringLiteral("PATH"), fixtureBin + QStringLiteral(":/usr/bin:/bin"));
    startupEnvironment.insert(QStringLiteral("FLATPAK_ID"), QStringLiteral("com.dockpipe.Dockpipe"));
    startupEnvironment.insert(QStringLiteral("DOCKPIPE_TERMINAL_TEST_HOME"), autoHome);
    startupEnvironment.insert(QStringLiteral("DOCKPIPE_BIN"), cli.fileName());
    startupEnvironment.insert(QStringLiteral("DOCKPIPE_GLOBAL_ROOT"), QDir(autoHome).filePath(QStringLiteral("state")));
    for (const QString &key : {QStringLiteral("XDG_CONFIG_HOME"), QStringLiteral("XDG_CACHE_HOME"), QStringLiteral("XDG_DATA_HOME")})
        startupEnvironment.insert(key, QDir(autoHome).filePath(key));
    QProcess launcher;
    launcher.setProcessEnvironment(startupEnvironment);
    launcher.setWorkingDirectory(autoHome);
    launcher.start(QString::fromLocal8Bit(argv[1]), {QStringLiteral("--allow-second-instance"), QStringLiteral("--start-home")});
    if (!launcher.waitForStarted(5000))
        return 21;
    QElapsedTimer timeout;
    timeout.start();
    while (timeout.elapsed() < 8000 && !readFile(QDir(autoHome).filePath(QStringLiteral(".bashrc"))).contains("esac\n")) {
        QCoreApplication::processEvents();
        QThread::msleep(20);
    }
    const bool automatic = QFile::exists(FlatpakTerminal::wrapperPath(autoHome))
        && readFile(QDir(autoHome).filePath(QStringLiteral(".bashrc"))).contains("esac\n");
    launcher.terminate();
    if (!launcher.waitForFinished(3000)) {
        launcher.kill();
        launcher.waitForFinished(1000);
    }
    if (!automatic) {
        std::fprintf(stderr, "%s", launcher.readAllStandardError().constData());
        return 22;
    }
    std::puts("PASS: automatic launcher setup, shell PATH integration, argument/exit forwarding, and native CLI preservation");
    return 0;
}
