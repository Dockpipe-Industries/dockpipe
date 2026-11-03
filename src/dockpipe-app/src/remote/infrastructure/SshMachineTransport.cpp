#include "SshMachineTransport.h"
#include "SshMachinePrivateFiles.h"

#include <QCryptographicHash>
#include <QJsonArray>
#include <QJsonDocument>
#include <QSaveFile>

namespace {
bool validLocalReference(const QString &path)
{
    if (path.size() > 4096 || path.toUtf8().size() > 4096)
        return false;
    for (const QChar character : path) {
        if (character.category() == QChar::Other_Control)
            return false;
    }
    return !path.contains('%') && !path.contains('$');
}

QString quoteRemote(const QString &value)
{
    QString escaped = value;
    escaped.replace('\'', QStringLiteral("'\\''"));
    return '\'' + escaped + '\'';
}

QString trustedClient(const QString &requested)
{
    const QStringList candidates = requested.isEmpty()
        ? QStringList{QStringLiteral("/usr/bin/ssh"), QStringLiteral("/bin/ssh")}
        : QStringList{requested};
    for (const auto &candidate : candidates) {
        if (!validLocalReference(candidate))
            continue;
        const QFileInfo information(candidate);
        if (!information.isAbsolute() || !information.isFile() || !information.isExecutable())
            continue;
#ifdef Q_OS_UNIX
        const QFileInfo canonical(information.canonicalFilePath());
        // Some desktop sandboxes map the immutable system image owner to an
        // unmapped uid. Fixed system SSH paths remain admissible when this
        // process cannot write them; arbitrary overrides still need root ownership.
        const QStringList systemClients{QStringLiteral("/usr/bin/ssh"), QStringLiteral("/bin/ssh"),
            QStringLiteral("/usr/local/bin/ssh")};
        const bool immutableSystemClient = systemClients.contains(canonical.absoluteFilePath())
            && !canonical.isWritable();
        if ((canonical.ownerId() != 0 && !immutableSystemClient)
            || (canonical.permissions() & (QFileDevice::WriteGroup | QFileDevice::WriteOther)))
            continue;
#endif
        return information.canonicalFilePath();
    }
    return {};
}

bool prepareKnownHosts(const QString &path)
{
    const QFileInfo existing(path);
    if (existing.exists() || existing.isSymLink())
        return SshMachinePrivateFiles::ownedRegularPath(existing, false)
            && SshMachinePrivateFiles::privatePermissions(existing);
    QSaveFile file(path);
    file.setDirectWriteFallback(false);
    return file.open(QIODevice::WriteOnly)
        && file.setPermissions(SshMachinePrivateFiles::FilePermissions) && file.commit();
}

QString optionPath(const QString &path)
{
    QString escaped = path;
    escaped.replace('\\', QStringLiteral("\\\\"));
    escaped.replace('"', QStringLiteral("\\\""));
    return '"' + escaped + '"';
}
}

SshMachineTransport::Plan SshMachineTransport::interactive(const SshMachineConnection &connection,
    const QString &stateDirectory, const QString &sshProgram)
{
    return prepare(connection, stateDirectory, sshProgram, true);
}

SshMachineTransport::Plan SshMachineTransport::snapshot(const SshMachineConnection &connection,
    const QString &stateDirectory, const QString &sshProgram)
{
    return prepare(connection, stateDirectory, sshProgram, false);
}

SshMachineTransport::Plan SshMachineTransport::prepare(const SshMachineConnection &connection,
    const QString &stateDirectory, const QString &sshProgram, bool interactive)
{
    Plan plan;
    plan.error = connection.validationError();
    if (!plan.error.isEmpty())
        return plan;
#if !defined(Q_OS_LINUX) && !defined(Q_OS_MACOS)
    Q_UNUSED(stateDirectory);
    Q_UNUSED(sshProgram);
    Q_UNUSED(interactive);
    plan.error = QStringLiteral("Shared authenticated SSH machine sessions are supported on Linux and macOS only.");
    return plan;
#else
    const QString program = trustedClient(sshProgram);
    if (program.isEmpty()) {
        plan.error = QStringLiteral("An absolute trusted system SSH executable is required.");
        return plan;
    }
    // OpenSSH expands percent tokens in identity/control/known-hosts paths.
    // Reject them in these local references rather than resolving a different file.
    if (!validLocalReference(stateDirectory) || !validLocalReference(connection.identityFile)
        || !SshMachinePrivateFiles::prepareDirectory(stateDirectory)) {
        plan.error = QStringLiteral("SSH state and identity references require private absolute paths without expansion tokens.");
        return plan;
    }
    if (!connection.identityFile.isEmpty()) {
        const QFileInfo identity(connection.identityFile);
        if (!SshMachinePrivateFiles::ownedRegularPath(identity, false)
            || !SshMachinePrivateFiles::privatePermissions(identity)) {
            plan.error = QStringLiteral("The SSH identity reference is unavailable or its permissions are not private.");
            return plan;
        }
    }
    const QJsonArray identity{connection.id, connection.host, connection.user, connection.port,
        connection.nativeCliPath, connection.identityFile};
    const auto digest = QCryptographicHash::hash(QJsonDocument(identity).toJson(QJsonDocument::Compact),
        QCryptographicHash::Sha256).toHex().left(20);
    const QString directory = QDir(stateDirectory).filePath(QString::fromLatin1(digest));
    const QString controlPath = QDir(directory).filePath(QStringLiteral("master"));
    // Darwin sockaddr_un has a 104-byte pathname. Reserve terminator and margin.
    if (controlPath.toUtf8().size() > 100 || !SshMachinePrivateFiles::prepareDirectory(directory)) {
        plan.error = QStringLiteral("The private SSH control socket directory is unavailable or its path exceeds 100 bytes.");
        return plan;
    }
    const QString knownHosts = QDir(directory).filePath(QStringLiteral("known_hosts"));
    if (!prepareKnownHosts(knownHosts)) {
        plan.error = QStringLiteral("The connection's private SSH host-key store is unavailable.");
        return plan;
    }
    plan.program = program;
    plan.workingDirectory = QDir::homePath();
    // Ignore ambient SSH configuration. All host identity, forwarding and
    // multiplexing choices below belong to this explicit machine connection.
    plan.arguments = {"-F", "/dev/null", "-o", "ConnectTimeout=5", "-o", "ConnectionAttempts=1",
        "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=2",
        "-o", "ForwardAgent=no", "-o", "ForwardX11=no", "-o", "ClearAllForwardings=yes",
        "-o", "GlobalKnownHostsFile=/dev/null", "-o", "UserKnownHostsFile=" + optionPath(knownHosts),
        "-o", "ControlMaster=auto", "-o", "ControlPersist=300", "-S", controlPath,
        "-o", interactive ? "StrictHostKeyChecking=ask" : "StrictHostKeyChecking=yes",
        "-o", interactive ? "BatchMode=no" : "BatchMode=yes"};
    if (!connection.identityFile.isEmpty())
        plan.arguments.append({"-o", "IdentitiesOnly=yes", "-i", connection.identityFile});
    plan.arguments.append(interactive ? "-tt" : "-T");
    if (!interactive)
        plan.arguments.append("-n");
    plan.arguments.append({"-l", connection.user, "-p", QString::number(connection.port), "--", connection.host});
    if (!interactive)
        plan.arguments.append(quoteRemote(connection.nativeCliPath) + QStringLiteral(" machine snapshot --json"));
    // No remote command means SSH starts the operator's actual login shell.
    // remoteDirectory is metadata for later explicit actions, never container cwd.
    return plan;
#endif
}
