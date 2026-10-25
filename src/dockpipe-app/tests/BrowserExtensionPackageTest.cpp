#include "BrowserExtensionPackage.h"
#include <QFile>
#include <QFileInfo>
#include <QCryptographicHash>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QUrlQuery>
#include <QtEndian>

namespace {
QByteArray fixture()
{
    return QByteArray::fromBase64("UEsDBBQAAAAAAOyEU13IDHjWNwAAADcAAAANAAAAbWFuaWZlc3QuanNvbnsibWFuaWZlc3RfdmVyc2lvbiI6MywibmFtZSI6IkZpeHR1cmUiLCJ2ZXJzaW9uIjoiMS4wIn1QSwECFAMUAAAAAADshFNdyAx41jcAAAA3AAAADQAAAAAAAAAAAAAAgAEAAAAAbWFuaWZlc3QuanNvblBLBQYAAAAAAQABADsAAABiAAAAAAA=");
}
bool save(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(bytes) == bytes.size();
}
}
class BrowserExtensionPackageTest : public QObject {
    Q_OBJECT
private slots:
    void storeAddresses()
    {
        const QString id(32, 'a');
        QCOMPARE(BrowserExtensionPackage::extensionId(id), id);
        QCOMPARE(BrowserExtensionPackage::extensionId("https://chromewebstore.google.com/detail/example/" + id + "?hl=en"), id);
        QCOMPARE(BrowserExtensionPackage::extensionId("https://chrome.google.com/webstore/detail/example/" + id), id);
        for (const QString &input : QStringList{"https://example.com", "https://chromewebstore.google.com.evil/detail/" + id,
                 "https://user@chromewebstore.google.com/detail/" + id, "http://chromewebstore.google.com/detail/" + id,
                 "https://chromewebstore.google.com/detail/" + id + "#fragment", QString(32, 'z')})
            QVERIFY2(BrowserExtensionPackage::extensionId(input).isEmpty(), qPrintable(input));
        const QUrl url = BrowserExtensionPackage::downloadUrl(id);
        QCOMPARE(url.host(), "clients2.google.com");
        QCOMPARE(url.scheme(), "https");
        QCOMPARE(QUrlQuery(url).queryItemValue("acceptformat"), "crx3");
        QCOMPARE(QUrlQuery(url).queryItemValue("x", QUrl::FullyDecoded), "id=" + id + "&installsource=ondemand&uc");
        QVERIFY(BrowserExtensionPackage::downloadUrl("bad").isEmpty());
    }
    void archiveAndCrx()
    {
        QTemporaryDir directory;
        const QString input = directory.filePath("package.crx");
        const QString output = directory.filePath("package.zip");
        QString error;
        QVERIFY(save(input, fixture()));
        QVERIFY2(BrowserExtensionPackage::prepareFile(input, output, &error), qPrintable(error));
        QFile file(output);
        QVERIFY(file.open(QIODevice::ReadOnly));
        QCOMPARE(file.readAll(), fixture());
        file.close();
        const QByteArray key("fixture-public-key");
        const QByteArray identity = QCryptographicHash::hash(key, QCryptographicHash::Sha256).left(16);
        const QByteArray proof = QByteArray::fromHex("0a") + char(key.size()) + key + QByteArray::fromHex("120173");
        const QByteArray signedData = QByteArray::fromHex("0a10") + identity;
        const QByteArray header = QByteArray::fromHex("12") + char(proof.size()) + proof
            + QByteArray::fromHex("82f104") + char(signedData.size()) + signedData;
        QByteArray crx("Cr24");
        crx.append(QByteArray::fromHex("03000000"));
        QByteArray size(4, '\0');
        qToLittleEndian<quint32>(header.size(), size.data());
        crx.append(size);
        crx.append(header);
        crx.append(fixture());
        QVERIFY(save(input, crx));
        QVERIFY2(BrowserExtensionPackage::prepareFile(input, output, &error), qPrintable(error));
        crx[4] = 2;
        QVERIFY(save(input, crx));
        QVERIFY(!BrowserExtensionPackage::prepareFile(input, output, &error));
        QVERIFY(!error.isEmpty());
    }
    void rejectUnsafeArchives()
    {
        QTemporaryDir directory;
        const QString input = directory.filePath("bad.zip");
        const QString output = directory.filePath("prepared.zip");
        const QByteArray original = fixture();
        const int central = original.indexOf(QByteArray::fromHex("504b0102"));
        QList<QByteArray> unsafe;
        unsafe << QByteArray("not an archive") << original.left(original.size() - 1);
        QByteArray oversized = original;
        qToLittleEndian<quint32>(65 * 1024 * 1024, oversized.data() + central + 24);
        unsafe << oversized;
        QByteArray encrypted = original;
        encrypted[central + 8] = 1;
        unsafe << encrypted;
        QByteArray symlink = original;
        qToLittleEndian<quint32>(0120777u << 16, symlink.data() + central + 38);
        unsafe << symlink;
        QByteArray traversal = original;
        traversal.replace("manifest.json", "../evilxx.json");
        unsafe << traversal;
        QByteArray wrongLocal = original;
        wrongLocal[30] = 'x';
        unsafe << wrongLocal;
        for (const auto &bytes : unsafe) {
            QVERIFY(save(input, bytes));
            QString error;
            QVERIFY(!BrowserExtensionPackage::prepareFile(input, output, &error));
            QVERIFY(!error.isEmpty());
            QVERIFY(!QFile::exists(output));
        }
        QFile sparse(input);
        QVERIFY(sparse.open(QIODevice::WriteOnly));
        QVERIFY(sparse.resize(33 * 1024 * 1024));
        sparse.close();
        QString error;
        QVERIFY(!BrowserExtensionPackage::prepareFile(input, output, &error));
    }
    void liveStoreDownload()
    {
        const QString id = qEnvironmentVariable("DOCKPIPE_EXTENSION_LIVE_ID");
        if (id.isEmpty())
            QSKIP("Set DOCKPIPE_EXTENSION_LIVE_ID for an explicit live download check.");
        BrowserExtensionPackage package;
        QSignalSpy failure(&package, &BrowserExtensionPackage::failed);
        QSignalSpy ready(&package, &BrowserExtensionPackage::ready);
        package.fetch(id);
        QTRY_VERIFY_WITH_TIMEOUT(!failure.isEmpty() || !ready.isEmpty(), 65000);
        QVERIFY2(failure.isEmpty(), failure.isEmpty() ? "" : qPrintable(failure.first().first().toString()));
        QCOMPARE(ready.count(), 1);
        QVERIFY(QFileInfo(ready.first().first().toString()).size() > 22);
        QVERIFY(!package.publicKey().isEmpty());
    }
    void invalidFetchAndCancel()
    {
        BrowserExtensionPackage package;
        QSignalSpy failure(&package, &BrowserExtensionPackage::failed);
        QSignalSpy ready(&package, &BrowserExtensionPackage::ready);
        package.fetch("invalid");
        QCOMPARE(failure.count(), 1);
        package.cancel();
        QCOMPARE(ready.count(), 0);
    }
};
QTEST_GUILESS_MAIN(BrowserExtensionPackageTest)
#include "BrowserExtensionPackageTest.moc"
