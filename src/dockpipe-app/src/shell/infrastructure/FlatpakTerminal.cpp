#include "FlatpakTerminal.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QObject>
#include <QProcess>
#include <QSaveFile>

namespace {

const QByteArray legacyWrapper =
    "#!/bin/sh\n"
    "# Dockpipe Flatpak terminal command. Remove this file to uninstall the shortcut.\n"
    "exec flatpak run --command=dockpipe com.dockpipe.Dockpipe \"$@\"\n";

const QByteArray wrapper =
    "#!/bin/sh\n"
    "# Dockpipe Flatpak terminal command. Remove this file to uninstall the shortcut.\n"
    "if ! workdir=$(pwd -P) || [ ! -d \"$workdir\" ]; then\n"
    "    echo 'Dockpipe cannot access the current folder. Change to an existing project folder and retry.' >&2\n"
    "    exit 1\n"
    "fi\n"
    "exec flatpak run --command=dockpipe --cwd=\"$workdir\" --filesystem=\"$workdir\" com.dockpipe.Dockpipe \"$@\"\n";

} // namespace

QString FlatpakTerminal::command()
{
    return QStringLiteral("flatpak run --command=dockpipe com.dockpipe.Dockpipe");
}

QString FlatpakTerminal::wrapperPath(const QString &home)
{
    return QDir(home).filePath(QStringLiteral(".local/bin/dockpipe"));
}

FlatpakTerminal::Result FlatpakTerminal::install(const QString &home)
{
    if (!QDir::isAbsolutePath(home))
        return {false, QObject::tr("Cannot determine your home directory.")};

    const QString path = wrapperPath(home);
    const QFileInfo existing(path);
    if (existing.isSymLink() || (existing.exists() && !existing.isFile()))
        return {false, QObject::tr("%1 already exists. It was left unchanged.").arg(path)};

    QFile file(path);
    if (existing.exists()) {
        if (!file.open(QIODevice::ReadOnly))
            return {false, QObject::tr("Cannot read %1: %2").arg(path, file.errorString())};
        const QByteArray contents = file.readAll();
        file.close();
        if (contents != wrapper && contents != legacyWrapper)
            return {false, QObject::tr("%1 already contains another command. It was left unchanged.").arg(path)};
        if (!existing.isExecutable())
            return {false, QObject::tr("%1 exists but is not executable. Check its permissions.").arg(path)};
        if (contents == legacyWrapper) {
            QSaveFile updated(path);
            if (!updated.open(QIODevice::WriteOnly)
                || updated.write(wrapper) != wrapper.size()
                || !updated.setPermissions(existing.permissions())
                || !updated.commit())
                return {false, QObject::tr("Could not update the terminal shortcut: %1").arg(updated.errorString())};
            return {true, QObject::tr("Updated the terminal shortcut at %1 to preserve the current folder.").arg(path)};
        }
        return {true, QObject::tr("The terminal shortcut is already installed at %1.").arg(path)};
    }

    if (!QDir().mkpath(existing.absolutePath()))
        return {false, QObject::tr("Cannot create %1.").arg(existing.absolutePath())};

    // NewOnly refuses existing files and symlinks even if they appeared after
    // the checks above. Never replace a user's native CLI or another shortcut.
    if (!file.open(QIODevice::WriteOnly | QIODevice::NewOnly))
        return {false, QObject::tr("Cannot create %1: %2").arg(path, file.errorString())};

    const bool written = file.write(wrapper) == wrapper.size() && file.flush();
    const bool executable = written && file.setPermissions(
        QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner
        | QFile::ReadGroup | QFile::ExeGroup | QFile::ReadOther | QFile::ExeOther);
    if (!executable) {
        const QString error = file.errorString();
        file.close();
        file.remove();
        return {false, QObject::tr("Could not install the terminal shortcut: %1").arg(error)};
    }
    file.close();
    return {true, QObject::tr("Installed %1. Open a new terminal and run dockpipe --version.").arg(path)};
}

FlatpakTerminal::Result FlatpakTerminal::completeSetup(const HostEnvironment &environment)
{
    // The lookup runs on the host, not against the sandbox's /app/bin/dockpipe.
    // Keep an existing native command as the user's selected terminal CLI.
    if (!environment.command.isEmpty()
        && QDir::cleanPath(environment.command) != wrapperPath(environment.home))
        return {true, QObject::tr("Using the existing terminal command: %1").arg(environment.command)};

    const auto installed = install(environment.home);
    if (!installed.success)
        return installed;
    const QString bin = QDir(environment.home).filePath(QStringLiteral(".local/bin"));
    if (environment.path.split(QLatin1Char(':')).contains(bin))
        return installed;

    QString startupPath;
    QByteArray configuration;
    const QString shell = QFileInfo(environment.shell).fileName();
    if (shell == QStringLiteral("bash") || shell == QStringLiteral("zsh")) {
        const QString directory = shell == QStringLiteral("zsh") ? environment.zshHome : environment.home;
        if (!QDir::isAbsolutePath(directory))
            return {false, QObject::tr("The CLI shortcut is installed, but the shell configuration directory is invalid.")};
        startupPath = QDir(directory).filePath(shell == QStringLiteral("bash")
            ? QStringLiteral(".bashrc") : QStringLiteral(".zshrc"));
        configuration = "\n# Dockpipe Flatpak terminal PATH\n"
            "case \":$PATH:\" in\n"
            "    *\":$HOME/.local/bin:\"*) ;;\n"
            "    *) export PATH=\"$PATH:$HOME/.local/bin\" ;;\n"
            "esac\n";
    } else if (shell == QStringLiteral("fish")) {
        if (!QDir::isAbsolutePath(environment.configHome))
            return {false, QObject::tr("The CLI shortcut is installed, but the Fish configuration directory is invalid.")};
        startupPath = QDir(environment.configHome).filePath(QStringLiteral("fish/conf.d/dockpipe.fish"));
        configuration = "# Dockpipe Flatpak terminal PATH\n"
            "if not contains -- \"$HOME/.local/bin\" $PATH\n"
            "    set -gx PATH $PATH \"$HOME/.local/bin\"\n"
            "end\n";
    } else {
        return {false, QObject::tr("The CLI shortcut is installed at %1. Add ~/.local/bin to PATH for your shell (%2).")
            .arg(wrapperPath(environment.home), shell)};
    }

    const QFileInfo existing(startupPath);
    if (existing.isSymLink() || (existing.exists() && !existing.isFile()))
        return {false, QObject::tr("The CLI shortcut is installed, but %1 was left unchanged because it is not a regular file.").arg(startupPath)};
    QFile startup(startupPath);
    if (existing.exists()) {
        if (!startup.open(QIODevice::ReadOnly))
            return {false, QObject::tr("Cannot read shell configuration %1.").arg(startupPath)};
        const QByteArray contents = startup.readAll();
        startup.close();
        if (contents.contains(configuration))
            return installed;
        if (shell == QStringLiteral("fish") || contents.contains("# Dockpipe Flatpak terminal PATH"))
            return {false, QObject::tr("The CLI shortcut is installed, but existing configuration in %1 was left unchanged.").arg(startupPath)};
    }
    if (!QDir().mkpath(existing.absolutePath())
        || !startup.open(QIODevice::WriteOnly | (existing.exists() ? QIODevice::Append : QIODevice::NewOnly))
        || startup.write(configuration) != configuration.size() || !startup.flush())
        return {false, QObject::tr("The CLI shortcut is installed, but PATH setup failed for %1: %2").arg(startupPath, startup.errorString())};
    return installed;
}

FlatpakTerminal::Result FlatpakTerminal::setupForCurrentUser()
{
    if (qEnvironmentVariableIsEmpty("FLATPAK_ID"))
        return {true, QString()};

    QProcess host;
    // flatpak-spawn inherits the host service environment unless --clear-env is
    // requested. Read only the fields needed for terminal integration, using NUL
    // separators so spaces and shell metacharacters in paths remain data.
    host.start(QStringLiteral("flatpak-spawn"), {
        QStringLiteral("--host"), QStringLiteral("/bin/sh"), QStringLiteral("-c"),
        QStringLiteral("printf '%s\\0' \"$HOME\" \"$(command -v dockpipe || :)\" \"$PATH\" "
                       "\"$SHELL\" \"${XDG_CONFIG_HOME:-$HOME/.config}\" \"${ZDOTDIR:-$HOME}\"")});
    if (!host.waitForStarted(2000) || !host.waitForFinished(3000)
        || host.exitStatus() != QProcess::NormalExit || host.exitCode() != 0) {
        host.kill();
        host.waitForFinished(1000);
        return {false, QObject::tr("Automatic terminal setup could not reach the host. Dockpipe remains available.")};
    }
    const auto fields = host.readAllStandardOutput().split('\0');
    if (fields.size() != 7 || !fields.last().isEmpty())
        return {false, QObject::tr("Automatic terminal setup received an invalid host environment.")};
    return completeSetup({QString::fromUtf8(fields[0]), QString::fromUtf8(fields[1]),
                          QString::fromUtf8(fields[2]), QString::fromUtf8(fields[3]),
                          QString::fromUtf8(fields[4]), QString::fromUtf8(fields[5])});
}
