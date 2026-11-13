#pragma once

#include <QJsonObject>
#include <QList>
#include <QSslCertificate>
#include <optional>

// Invalid or incomplete snapshots never become empty trusted-root sets.
std::optional<QList<QSslCertificate>> parseBrowserNetworkTrust(
    const QJsonObject &snapshot, const QJsonObject &expectedTarget);
