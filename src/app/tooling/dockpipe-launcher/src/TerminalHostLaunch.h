#pragma once

#include "TerminalLaunchContext.h"

#include <QMap>
#include <QString>
#include <QStringList>

// Flatpak forwards the PTY descriptors; its host service owns the controlling
// session. The fixed supervisor retains that session while verifying and
// terminating its jobs.
class TerminalHostLaunch final {
public:
    struct Plan {
        QString program;
        QStringList arguments;
        QString workingDirectory;
        QString error;
        bool host = false;
    };

    static bool required(TerminalLaunchContext context = TerminalLaunchContext::Automatic);
    static Plan prepare(const QString &shell, const QStringList &arguments, const QString &workingDirectory,
        const QMap<QString, QString> &environment = {},
        TerminalLaunchContext context = TerminalLaunchContext::Automatic);
    // Explicit bridge injection keeps faithful process-boundary fixtures out of
    // production environment variables or shell command interpolation.
    static Plan forHost(const QString &shell, const QStringList &arguments,
        const QString &workingDirectory, const QString &bridge = QStringLiteral("/usr/bin/flatpak-spawn"),
        const QMap<QString, QString> &environment = {});
};
