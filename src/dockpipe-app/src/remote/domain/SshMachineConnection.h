#pragma once

#include <QString>

// Public connection metadata only. Authentication material remains in SSH's
// interactive channel or in the referenced operator-owned identity file.
struct SshMachineConnection {
    QString id;
    QString label;
    QString host;
    QString user;
    int port = 22;
    QString nativeCliPath;
    QString identityFile;
    QString remoteDirectory;

    QString validationError() const;
};
