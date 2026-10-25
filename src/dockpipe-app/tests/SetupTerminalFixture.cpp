#include "SetupTerminal.h"

#include <QDir>
#include <QFile>

namespace {
bool recordTerminalCommand(const QString &program, const QStringList &arguments, QString *error)
{
    const QString directory = qEnvironmentVariable("DOCKPIPE_TEST_ROOT");
    if (directory.isEmpty()) {
        *error = QStringLiteral("Terminal fixture directory is missing.");
        return false;
    }

    QFile output(QDir(directory).filePath(QStringLiteral("exec-terminal-args")));
    if (!output.open(QIODevice::WriteOnly | QIODevice::Truncate)) {
        *error = output.errorString();
        return false;
    }

    QStringList command{program};
    command.append(arguments);
    const QByteArray content = (command.join(QLatin1Char('\n')) + QLatin1Char('\n')).toUtf8();
    if (output.write(content) != content.size()) {
        *error = output.errorString();
        return false;
    }
    return true;
}
}

bool openSetupTerminal(const QString &program, const QStringList &arguments, QString *error)
{
    return recordTerminalCommand(program, arguments, error);
}

bool openCommandTerminal(const QString &program, const QStringList &arguments, QString *error)
{
    return recordTerminalCommand(program, arguments, error);
}
