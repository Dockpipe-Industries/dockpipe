#pragma once

#include <QDir>
#include <QFile>
#include <QFileInfo>

#ifdef Q_OS_UNIX
#include <unistd.h>
#endif

namespace SshMachinePrivateFiles {
constexpr auto DirectoryPermissions = QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner;
constexpr auto FilePermissions = QFileDevice::ReadOwner | QFileDevice::WriteOwner;

inline bool ownedRegularPath(const QFileInfo &information, bool directory)
{
    if (information.isSymLink() || (directory ? !information.isDir() : !information.isFile()))
        return false;
#ifdef Q_OS_UNIX
    if (information.ownerId() != static_cast<uint>(::geteuid()))
        return false;
#endif
    return true;
}

inline bool privatePermissions(const QFileInfo &information)
{
    const auto publicBits = QFileDevice::ReadGroup | QFileDevice::WriteGroup | QFileDevice::ExeGroup
        | QFileDevice::ReadOther | QFileDevice::WriteOther | QFileDevice::ExeOther;
    return !(information.permissions() & publicBits);
}

inline bool prepareDirectory(const QString &path)
{
    if (!QDir::isAbsolutePath(path))
        return false;
    const QFileInfo previous(path);
    if (previous.isSymLink() || (previous.exists() && !ownedRegularPath(previous, true)))
        return false;
    return QDir().mkpath(path) && ownedRegularPath(QFileInfo(path), true)
        && QFile::setPermissions(path, DirectoryPermissions);
}
}
