#pragma once

#include "SshMachineConnection.h"

class SshMachineConnectionStore final {
public:
    explicit SshMachineConnectionStore(const QString &path);

    QString path() const;
    // A missing store returns false and a diagnostic; it never invents a host.
    bool load(SshMachineConnection *connection, QString *error = nullptr) const;
    bool save(const SshMachineConnection &connection, QString *error = nullptr) const;

private:
    QString m_path;
};
