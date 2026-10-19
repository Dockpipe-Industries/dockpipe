#include "BrowserExtensionCrx.h"

#include <QByteArrayView>
#include <QCryptographicHash>
#include <QtEndian>

namespace {
// CRX headers contain small identity/proof records. Limit parser work and bytes
// hashed to 1 MiB, independently of the much larger extension ZIP payload.
constexpr quint32 maximumHeaderBytes = 1024 * 1024;

class ProtoReader final {
public:
    explicit ProtoReader(QByteArrayView bytes) : m_bytes(bytes) {}

    bool next(quint32 *number, QByteArrayView *bytes)
    {
        quint64 tag = 0;
        if (!varint(&tag) || (tag >> 3) == 0 || (tag >> 3) > 0x1fffffff)
            return false;
        *number = quint32(tag >> 3);
        *bytes = {};
        const quint32 wire = tag & 7;
        quint64 value = 0;
        if (wire == 0)
            return varint(&value);
        if (wire == 1)
            return advance(8);
        if (wire == 5)
            return advance(4);
        if (wire != 2 || !varint(&value) || value > quint64(m_bytes.size() - m_offset))
            return false;
        *bytes = m_bytes.sliced(m_offset, qsizetype(value));
        m_offset += qsizetype(value);
        return true;
    }

    bool atEnd() const { return m_offset == m_bytes.size(); }

private:
    bool advance(qsizetype length)
    {
        if (length > m_bytes.size() - m_offset)
            return false;
        m_offset += length;
        return true;
    }

    bool varint(quint64 *value)
    {
        *value = 0;
        for (int index = 0; index < 10; ++index) {
            if (m_offset == m_bytes.size())
                return false;
            const quint8 byte = quint8(m_bytes[m_offset++]);
            if (index == 9 && byte > 1)
                return false;
            *value |= quint64(byte & 0x7f) << (index * 7);
            if (!(byte & 0x80))
                return true;
        }
        return false;
    }

    QByteArrayView m_bytes;
    qsizetype m_offset = 0;
};

QByteArray fail(QString *error, const QString &message)
{
    if (error)
        *error = message;
    return {};
}

bool readIdentity(QByteArrayView signedHeader, QByteArrayView *identity)
{
    ProtoReader reader(signedHeader);
    while (!reader.atEnd()) {
        quint32 number = 0;
        QByteArrayView bytes;
        if (!reader.next(&number, &bytes))
            return false;
        if (number == 1) {
            if (!identity->isEmpty() || bytes.size() != 16)
                return false;
            *identity = bytes;
        }
    }
    return identity->size() == 16;
}

bool readProof(QByteArrayView proof, QByteArrayView *key)
{
    ProtoReader reader(proof);
    bool hasSignature = false;
    while (!reader.atEnd()) {
        quint32 number = 0;
        QByteArrayView bytes;
        if (!reader.next(&number, &bytes))
            return false;
        if (number == 1) {
            if (!key->isEmpty() || bytes.isEmpty())
                return false;
            *key = bytes;
        } else if (number == 2) {
            if (hasSignature || bytes.isEmpty())
                return false;
            hasSignature = true;
        }
    }
    return !key->isEmpty() && hasSignature;
}

QString extensionId(QByteArrayView identity)
{
    QString result;
    result.reserve(32);
    for (char value : identity) {
        const quint8 byte = quint8(value);
        result.append(QChar('a' + (byte >> 4)));
        result.append(QChar('a' + (byte & 15)));
    }
    return result;
}
}

QByteArray BrowserExtensionCrx::extractPublicKey(const QByteArray &crx, const QString &expectedId,
                                               QString *error)
{
    if (error)
        error->clear();
    if (!expectedId.isEmpty()) {
        if (expectedId.size() != 32)
            return fail(error, QStringLiteral("The expected Chrome extension ID is invalid."));
        for (QChar character : expectedId) {
            if (character < QLatin1Char('a') || character > QLatin1Char('p'))
                return fail(error, QStringLiteral("The expected Chrome extension ID is invalid."));
        }
    }
    if (crx.size() < 12 || crx.first(4) != "Cr24"
        || qFromLittleEndian<quint32>(crx.constData() + 4) != 3)
        return fail(error, QStringLiteral("The extension must use the CRX3 package format."));
    const quint32 headerSize = qFromLittleEndian<quint32>(crx.constData() + 8);
    if (headerSize == 0 || headerSize > maximumHeaderBytes
        || headerSize > quint64(crx.size() - 12))
        return fail(error, QStringLiteral("The CRX3 identity header is missing, truncated or exceeds 1 MiB."));

    const QByteArrayView header(crx.constData() + 12, headerSize);
    QByteArrayView signedHeader;
    ProtoReader reader(header);
    while (!reader.atEnd()) {
        quint32 number = 0;
        QByteArrayView bytes;
        if (!reader.next(&number, &bytes))
            return fail(error, QStringLiteral("The CRX3 identity header is malformed."));
        if (number == 10000) {
            if (!signedHeader.isEmpty() || bytes.isEmpty())
                return fail(error, QStringLiteral("The CRX3 signed identity is duplicated or missing."));
            signedHeader = bytes;
        }
    }
    QByteArrayView identity;
    if (!readIdentity(signedHeader, &identity))
        return fail(error, QStringLiteral("The CRX3 package has no valid signed extension ID."));
    if (!expectedId.isEmpty() && extensionId(identity) != expectedId)
        return fail(error, QStringLiteral("The downloaded package does not match the requested extension ID."));

    QByteArray matchingKey;
    ProtoReader proofs(header);
    while (!proofs.atEnd()) {
        quint32 number = 0;
        QByteArrayView bytes;
        if (!proofs.next(&number, &bytes))
            return fail(error, QStringLiteral("The CRX3 identity header is malformed."));
        if (number != 2 && number != 3)
            continue;
        QByteArrayView key;
        if (!readProof(bytes, &key))
            return fail(error, QStringLiteral("The CRX3 package contains a malformed identity proof."));
        const QByteArray digest = QCryptographicHash::hash(key, QCryptographicHash::Sha256);
        if (QByteArrayView(digest).first(16) == identity)
            matchingKey = key.toByteArray();
    }
    if (matchingKey.isEmpty())
        return fail(error, QStringLiteral("No CRX3 public key matches the signed extension ID."));
    return matchingKey;
}
