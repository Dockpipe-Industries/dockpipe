#include "BrowserNetworkTrust.h"

#include <QCryptographicHash>
#include <QJsonArray>
#include <QRegularExpression>
#include <QtEndian>

std::optional<QList<QSslCertificate>> parseBrowserNetworkTrust(
    const QJsonObject &snapshot, const QJsonObject &expectedTarget)
{
    static const QRegularExpression digestPattern(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    static const QRegularExpression nodePattern(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    if (expectedTarget.size() != 3
        || !nodePattern.match(expectedTarget.value("node").toString()).hasMatch()
        || !digestPattern.match(expectedTarget.value("fingerprint").toString()).hasMatch()
        || !digestPattern.match(expectedTarget.value("generation").toString()).hasMatch()
        || snapshot.size() != 4
        || snapshot.value("schema") != "dockpipe.remote.network-trust/v1"
        || snapshot.value("target").toObject() != expectedTarget
        || !digestPattern.match(snapshot.value("digest").toString()).hasMatch()
        || !snapshot.value("certificates").isArray())
        return std::nullopt;

    const auto entries = snapshot.value("certificates").toArray();
    if (entries.size() > 512)
        return std::nullopt;
    QCryptographicHash material(QCryptographicHash::Sha256);
    material.addData(QByteArrayLiteral("dockpipe.remote.network-trust/v1\n"));
    QByteArray previous;
    qsizetype totalBytes = 0;
    QList<QSslCertificate> certificates;
    for (const auto &entry : entries) {
        if (!entry.isString() || entry.toString().size() > 21848)
            return std::nullopt;
        const auto encoded = entry.toString().toLatin1();
        const auto decoded = QByteArray::fromBase64Encoding(encoded, QByteArray::AbortOnBase64DecodingErrors);
        if (!decoded || decoded.decoded.isEmpty() || decoded.decoded.size() > 16 * 1024
            || decoded.decoded.toBase64() != encoded)
            return std::nullopt;
        totalBytes += decoded.decoded.size();
        if (totalBytes > 2 * 1024 * 1024)
            return std::nullopt;
        const auto fingerprint = QCryptographicHash::hash(decoded.decoded, QCryptographicHash::Sha256);
        if (!previous.isEmpty() && fingerprint <= previous)
            return std::nullopt;
        previous = fingerprint;
        const auto parsed = QSslCertificate::fromData(decoded.decoded, QSsl::Der);
        if (parsed.size() != 1 || parsed.first().isNull() || parsed.first().toDer() != decoded.decoded)
            return std::nullopt;
        const auto length = qToBigEndian(quint32(decoded.decoded.size()));
        material.addData(QByteArrayView(reinterpret_cast<const char *>(&length), sizeof(length)));
        material.addData(decoded.decoded);
        certificates.append(parsed.first());
    }
    if (material.result().toHex() != snapshot.value("digest").toString().toLatin1())
        return std::nullopt;
    return certificates;
}
