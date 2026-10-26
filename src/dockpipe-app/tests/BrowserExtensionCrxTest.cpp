#include "BrowserExtensionCrx.h"

#include <QCryptographicHash>
#include <QtEndian>
#include <QtTest>

namespace {
QByteArray varint(quint64 value)
{
    QByteArray result;
    do {
        const quint8 byte = value & 0x7f;
        value >>= 7;
        result.append(char(value ? byte | 0x80 : byte));
    } while (value);
    return result;
}

QByteArray field(quint32 number, const QByteArray &bytes)
{
    return varint((quint64(number) << 3) | 2) + varint(bytes.size()) + bytes;
}

QByteArray package(const QByteArray &header)
{
    QByteArray result("Cr24", 4);
    char integer[4];
    qToLittleEndian<quint32>(3, integer);
    result.append(integer, 4);
    qToLittleEndian<quint32>(quint32(header.size()), integer);
    result.append(integer, 4);
    return result + header + "PK\x03\x04";
}

QByteArray identity(const QByteArray &key)
{
    return QCryptographicHash::hash(key, QCryptographicHash::Sha256).first(16);
}

QByteArray proof(const QByteArray &key)
{
    return field(1, key) + field(2, "signature fixture is not verified");
}

QString id(const QByteArray &bytes)
{
    QString result;
    for (char value : bytes) {
        const quint8 byte = quint8(value);
        result += QChar('a' + (byte >> 4));
        result += QChar('a' + (byte & 15));
    }
    return result;
}
}

class BrowserExtensionCrxTest final : public QObject {
    Q_OBJECT
private slots:
    void selectsMatchingKeyAcrossProofs();
    void rejectsInvalidIdentityAndProofs();
    void rejectsTruncatedAndOversizedHeaders();
    void rejectsMalformedProtobuf();
};

void BrowserExtensionCrxTest::selectsMatchingKeyAcrossProofs()
{
    const QByteArray key = QByteArray::fromHex("3006300406022a03");
    const QByteArray otherKey = QByteArray::fromHex("3006300406022a04");
    const QByteArray header = field(2, proof(otherKey)) + field(10000, field(1, identity(key)))
        + field(3, proof(key)) + field(123, "future metadata");
    QString error = "stale error";
    QCOMPARE(BrowserExtensionCrx::extractPublicKey(package(header), id(identity(key)), &error), key);
    QVERIFY(error.isEmpty());
    QCOMPARE(BrowserExtensionCrx::extractPublicKey(package(header), {}, nullptr), key);
}

void BrowserExtensionCrxTest::rejectsInvalidIdentityAndProofs()
{
    const QByteArray key("public key fixture");
    const QByteArray signedHeader = field(10000, field(1, identity(key)));
    const QByteArray valid = signedHeader + field(2, proof(key));
    QString error;
    QVERIFY(BrowserExtensionCrx::extractPublicKey(package(valid), QString(32, 'z'), &error).isEmpty());
    QVERIFY(!error.isEmpty());
    QVERIFY(BrowserExtensionCrx::extractPublicKey(package(valid), QString(32, 'a'), &error).isEmpty());
    for (const QByteArray &header : QList<QByteArray>{
             field(2, proof(key)),
             signedHeader + signedHeader + field(2, proof(key)),
             field(10000, field(1, QByteArray(15, 'a'))) + field(2, proof(key)),
             field(10000, field(1, identity(key)) + field(1, identity(key))) + field(2, proof(key)),
             signedHeader + field(2, proof("different key")),
             signedHeader + field(2, field(1, key)),
             signedHeader + field(2, field(1, key) + field(1, key) + field(2, "signature")),
             signedHeader + field(2, field(1, key) + field(2, "signature") + field(2, "duplicate"))}) {
        QVERIFY(BrowserExtensionCrx::extractPublicKey(package(header), {}, &error).isEmpty());
        QVERIFY(!error.isEmpty());
    }
}

void BrowserExtensionCrxTest::rejectsTruncatedAndOversizedHeaders()
{
    QString error;
    const QByteArray key("key");
    const QByteArray valid = package(field(10000, field(1, identity(key))) + field(2, proof(key)));
    for (qsizetype length = 0; length < valid.size() - 4; ++length)
        QVERIFY(BrowserExtensionCrx::extractPublicKey(valid.first(length), {}, &error).isEmpty());
    QByteArray wrongVersion = valid;
    wrongVersion[4] = 2;
    QVERIFY(BrowserExtensionCrx::extractPublicKey(wrongVersion, {}, &error).isEmpty());
    QByteArray oversized = package(QByteArray(1024 * 1024 + 1, 'x'));
    QVERIFY(BrowserExtensionCrx::extractPublicKey(oversized, {}, &error).isEmpty());
    QVERIFY(error.contains("1 MiB"));
}

void BrowserExtensionCrxTest::rejectsMalformedProtobuf()
{
    QString error;
    for (const QByteArray &malformed : QList<QByteArray>{
             QByteArray::fromHex("00"), QByteArray::fromHex("0b"),
             QByteArray::fromHex("0a08aabb"), QByteArray(11, char(0x80)),
             QByteArray::fromHex("ffffffffffffffffff02"),
             QByteArray::fromHex("0a80808080808080808002"),
             QByteArray::fromHex("090000"), QByteArray::fromHex("0d0000")}) {
        QVERIFY(BrowserExtensionCrx::extractPublicKey(package(malformed), {}, &error).isEmpty());
        QVERIFY(!error.isEmpty());
    }
}

QTEST_GUILESS_MAIN(BrowserExtensionCrxTest)
#include "BrowserExtensionCrxTest.moc"
