#include "SshMachineConnection.h"

#include <QDir>
#include <QRegularExpression>

namespace {
bool boundedText(const QString &text, int maximum)
{
    if (text.size() > maximum || text.toUtf8().size() > maximum)
        return false;
    for (const QChar character : text) {
        if (character.category() == QChar::Other_Control
            || character.category() == QChar::Other_Surrogate)
            return false;
    }
    return true;
}

bool remotePath(const QString &path)
{
    return path.startsWith('/') && boundedText(path, 4096);
}
}

QString SshMachineConnection::validationError() const
{
    static const QRegularExpression identifier(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    static const QRegularExpression hostname(QStringLiteral("\\A[A-Za-z0-9:][A-Za-z0-9.:-]{0,252}\\z"));
    static const QRegularExpression account(QStringLiteral("\\A[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}\\z"));
    if (!identifier.match(id).hasMatch())
        return QStringLiteral("The machine connection needs a valid identifier of at most 64 characters.");
    if (label.trimmed().isEmpty() || !boundedText(label, 160))
        return QStringLiteral("The machine connection needs a label of at most 160 UTF-8 bytes.");
    if (!hostname.match(host).hasMatch() || !account.match(user).hasMatch() || port < 1 || port > 65535)
        return QStringLiteral("The SSH host, account or port is invalid.");
    if (!remotePath(nativeCliPath))
        return QStringLiteral("The native CLI requires an absolute remote path of at most 4096 UTF-8 bytes.");
    if (!identityFile.isEmpty() && (!QDir::isAbsolutePath(identityFile) || !boundedText(identityFile, 4096)))
        return QStringLiteral("The SSH identity reference requires an absolute local path.");
    if (!remoteDirectory.isEmpty() && !remotePath(remoteDirectory))
        return QStringLiteral("The remote directory requires an absolute remote path.");
    return {};
}
