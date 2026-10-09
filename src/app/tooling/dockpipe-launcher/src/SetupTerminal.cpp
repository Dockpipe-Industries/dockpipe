#include "SetupTerminal.h"

#include <QFileInfo>
#include <QProcess>
#include <QSettings>

namespace {
QString shellQuote(QString value)
{
    value.replace(QLatin1Char('\''), QStringLiteral("'\"'\"'"));
    return QLatin1Char('\'') + value + QLatin1Char('\'');
}
}

bool openSetupTerminal(const QString &program, const QStringList &arguments, QString *error)
{
#ifdef Q_OS_MACOS
    QStringList command{shellQuote(program)};
    for (const auto &argument : arguments)
        command.append(shellQuote(argument));
    QString scriptCommand = command.join(QLatin1Char(' '));
    scriptCommand.replace(QStringLiteral("\\"), QStringLiteral("\\\\"));
    scriptCommand.replace(QStringLiteral("\""), QStringLiteral("\\\""));
    const QString script = QStringLiteral("tell application \"Terminal\"\nactivate\ndo script \"%1\"\nend tell").arg(scriptCommand);
    if (QProcess::startDetached(QStringLiteral("/usr/bin/osascript"), {QStringLiteral("-e"), script}))
        return true;
#elif defined(Q_OS_LINUX)
    // Keep the terminal alive after setup so failures and the completion message
    // remain visible. The CLI and all user input are positional shell arguments.
    const QString terminalScript = QStringLiteral(
        "if command -v xdg-terminal-exec >/dev/null 2>&1; then exec xdg-terminal-exec \"$@\"; "
        "elif command -v gnome-terminal >/dev/null 2>&1; then exec gnome-terminal -- \"$@\"; "
        "elif command -v konsole >/dev/null 2>&1; then exec konsole -e \"$@\"; "
        "elif command -v x-terminal-emulator >/dev/null 2>&1; then exec x-terminal-emulator -e \"$@\"; "
        "elif command -v xterm >/dev/null 2>&1; then exec xterm -e \"$@\"; "
        "else echo 'No supported terminal found' >&2; exit 1; fi");
    QStringList command{program};
    command.append(arguments);
    const bool flatpak = QFileInfo::exists(QStringLiteral("/.flatpak-info"));
    if (flatpak) {
        QSettings info(QStringLiteral("/.flatpak-info"), QSettings::IniFormat);
        const QString application = info.value(QStringLiteral("Application/name")).toString();
        if (application.isEmpty()) {
            *error = QStringLiteral("Flatpak application identity is unavailable.");
            return false;
        }
        command = {QStringLiteral("flatpak"), QStringLiteral("run"), QStringLiteral("--command=dockpipe"), application};
        command.append(arguments);
    }
    QStringList terminalArgs{QStringLiteral("-c"), terminalScript, QStringLiteral("dockpipe-terminal"),
                            QStringLiteral("sh"), QStringLiteral("-c"),
                            QStringLiteral("\"$@\"; result=$?; printf '\\nSetup exited with status %s. Press Enter to close.\\n' \"$result\"; read reply; exit \"$result\""),
                            QStringLiteral("dockpipe-setup")};
    terminalArgs.append(command);
    if (flatpak) {
        terminalArgs.prepend(QStringLiteral("sh"));
        terminalArgs.prepend(QStringLiteral("--host"));
        if (QProcess::startDetached(QStringLiteral("flatpak-spawn"), terminalArgs))
            return true;
    } else if (QProcess::startDetached(QStringLiteral("sh"), terminalArgs)) {
        return true;
    }
#else
    Q_UNUSED(program);
    Q_UNUSED(arguments);
#endif
    *error = QStringLiteral("Could not open a terminal for remote setup. Remote services currently support Linux and macOS.");
    return false;
}
