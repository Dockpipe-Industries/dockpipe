#pragma once

#include "SshMachineConnection.h"

#include <QStringList>

// Builds commands only. The terminal/command owner controls execution,
// cancellation and bounded output. No connection failure selects localhost.
class SshMachineTransport final {
public:
    struct Plan {
        QString program;
        QStringList arguments;
        QString workingDirectory;
        QString error;

        bool valid() const { return error.isEmpty() && !program.isEmpty(); }
    };

    static Plan interactive(const SshMachineConnection &connection, const QString &stateDirectory,
        const QString &sshProgram = {});
    static Plan snapshot(const SshMachineConnection &connection, const QString &stateDirectory,
        const QString &sshProgram = {});

private:
    static Plan prepare(const SshMachineConnection &connection, const QString &stateDirectory,
        const QString &sshProgram, bool interactive);
};
