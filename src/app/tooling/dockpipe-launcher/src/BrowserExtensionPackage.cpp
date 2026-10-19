#include "BrowserExtensionPackage.h"
#include "BrowserExtensionCrx.h"

#include <QFile>
#include <QFileInfo>
#include <QNetworkAccessManager>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QRegularExpression>
#include <QSaveFile>
#include <QSet>
#include <QTimer>
#include <QUrlQuery>
#include <QtEndian>
#include <QtWebEngineCore/qtwebenginecoreglobal.h>

namespace {
constexpr qsizetype maximumPackageBytes = 32 * 1024 * 1024;
constexpr quint64 maximumExpandedBytes = 64 * 1024 * 1024;
quint16 shortAt(const QByteArray &bytes, qsizetype offset)
{
    return qFromLittleEndian<quint16>(bytes.constData() + offset);
}
quint32 wordAt(const QByteArray &bytes, qsizetype offset)
{
    return qFromLittleEndian<quint32>(bytes.constData() + offset);
}
bool validateZip(const QByteArray &bytes, QString *error)
{
    auto reject = [error] {
        if (error)
            *error = QObject::tr("Invalid or unsupported extension archive. Use a ZIP with manifest.json at its root, at most 4096 files and 64 MiB expanded content.");
        return false;
    };
    if (bytes.size() < 22)
        return reject();
    qsizetype end = bytes.size() - 22;
    const qsizetype first = qMax<qsizetype>(0, end - 65535);
    for (; end >= first; --end) {
        if (wordAt(bytes, end) == 0x06054b50 && end + 22 + shortAt(bytes, end + 20) == bytes.size())
            break;
    }
    if (end < first || shortAt(bytes, end + 4) || shortAt(bytes, end + 6))
        return reject();
    const int entries = shortAt(bytes, end + 10);
    const quint64 centralStart = wordAt(bytes, end + 16);
    const quint64 centralSize = wordAt(bytes, end + 12);
    if (entries < 1 || entries > 4096 || shortAt(bytes, end + 8) != entries || centralStart + centralSize != quint64(end))
        return reject();
    quint64 expanded = 0;
    qsizetype offset = centralStart;
    QSet<QByteArray> paths;
    bool manifest = false;
    for (int index = 0; index < entries; ++index) {
        if (offset < 0 || offset + 46 > end || wordAt(bytes, offset) != 0x02014b50)
            return reject();
        const int nameSize = shortAt(bytes, offset + 28);
        const int extraSize = shortAt(bytes, offset + 30);
        const int commentSize = shortAt(bytes, offset + 32);
        const quint32 compressed = wordAt(bytes, offset + 20);
        const quint32 size = wordAt(bytes, offset + 24);
        const quint32 local = wordAt(bytes, offset + 42);
        const int method = shortAt(bytes, offset + 10);
        if (nameSize < 1 || offset + 46 + nameSize + extraSize + commentSize > end
            || (shortAt(bytes, offset + 8) & 1) || (method != 0 && method != 8)
            || shortAt(bytes, offset + 34) || ((wordAt(bytes, offset + 38) >> 16) & 0170000) == 0120000)
            return reject();
        const QByteArray name = bytes.mid(offset + 46, nameSize);
        if (name.startsWith('/') || name.contains('\\') || name.contains(':') || name.contains('\0') || paths.contains(name))
            return reject();
        for (const auto &part : name.split('/')) {
            if (part == ".." || part == ".")
                return reject();
        }
        paths.insert(name);
        manifest |= name == "manifest.json";
        expanded += size;
        if (expanded > maximumExpandedBytes || quint64(local) + 30 > centralStart || wordAt(bytes, local) != 0x04034b50)
            return reject();
        const quint64 payload = quint64(local) + 30 + shortAt(bytes, local + 26) + shortAt(bytes, local + 28);
        if (payload + compressed > centralStart || shortAt(bytes, local + 26) != nameSize
            || bytes.mid(local + 30, nameSize) != name || shortAt(bytes, local + 8) != method
            || (shortAt(bytes, local + 6) & 1))
            return reject();
        offset += 46 + nameSize + extraSize + commentSize;
    }
    return manifest && offset == end ? true : reject();
}
bool trustedDownload(const QUrl &url)
{
    const QString host = url.host().toLower();
    return url.scheme() == "https" && url.userInfo().isEmpty() && (url.port() == -1 || url.port() == 443)
        && (host == "clients2.google.com" || host == "clients2.googleusercontent.com"
            || host.endsWith(".googleusercontent.com") || host == "dl.google.com");
}
}

BrowserExtensionPackage::BrowserExtensionPackage(QObject *parent) : QObject(parent),
    m_network(new QNetworkAccessManager(this)), m_timeout(new QTimer(this))
{
    m_timeout->setSingleShot(true);
    connect(m_timeout, &QTimer::timeout, this, [this] { fail(tr("Extension download timed out. Try again.")); });
}
BrowserExtensionPackage::~BrowserExtensionPackage() { cancel(); }

QString BrowserExtensionPackage::extensionId(const QString &input)
{
    static const QRegularExpression valid("^[a-p]{32}$");
    const QString text = input.trimmed();
    if (valid.match(text).hasMatch())
        return text;
    const QUrl url(text, QUrl::StrictMode);
    if (!url.isValid() || url.scheme() != "https" || !url.userInfo().isEmpty() || url.hasFragment()
        || (url.port() != -1 && url.port() != 443) || text.size() > 4096)
        return {};
    const bool modern = url.host() == "chromewebstore.google.com" && url.path().startsWith("/detail/");
    const bool legacy = url.host() == "chrome.google.com" && url.path().startsWith("/webstore/detail/");
    const QStringList segments = url.path().split('/', Qt::SkipEmptyParts);
    if (segments.isEmpty())
        return {};
    const QString id = segments.last();
    return (modern || legacy) && valid.match(id).hasMatch() ? id : QString();
}
QUrl BrowserExtensionPackage::downloadUrl(const QString &id)
{
    if (extensionId(id) != id)
        return {};
    QUrl url("https://clients2.google.com/service/update2/crx");
    QUrlQuery query;
    query.addQueryItem("response", "redirect");
    query.addQueryItem("prodversion", QString::fromLatin1(qWebEngineChromiumVersion()));
    query.addQueryItem("acceptformat", "crx3");
    query.addQueryItem("x", "id=" + id + "&installsource=ondemand&uc");
    url.setQuery(query);
    return url;
}
bool BrowserExtensionPackage::prepareFile(const QString &input, const QString &outputZip, QString *error, QString *publicKey)
{
    if (publicKey)
        publicKey->clear();
    QFile file(input);
    if (!QFileInfo(input).isFile() || QFileInfo(input).isSymLink() || !file.open(QIODevice::ReadOnly)
        || file.size() > maximumPackageBytes) {
        if (error)
            *error = tr("Choose a readable ZIP or CRX3 extension package smaller than 32 MiB.");
        return false;
    }
    QByteArray bytes = file.read(maximumPackageBytes + 1);
    if (bytes.size() > maximumPackageBytes || file.error() != QFile::NoError) {
        if (error)
            *error = tr("Could not read the extension package within the 32 MiB limit.");
        return false;
    }
    if (bytes.startsWith("Cr24")) {
        if (bytes.size() < 12 || wordAt(bytes, 4) != 3 || wordAt(bytes, 8) > 1024 * 1024
            || quint64(wordAt(bytes, 8)) + 12 >= quint64(bytes.size())) {
            if (error)
                *error = tr("Invalid CRX3 package header.");
            return false;
        }
        const QByteArray key = BrowserExtensionCrx::extractPublicKey(bytes, {}, error);
        if (key.isEmpty())
            return false;
        if (publicKey)
            *publicKey = QString::fromLatin1(key.toBase64());
        bytes.remove(0, 12 + wordAt(bytes, 8));
    }
    if (!validateZip(bytes, error))
        return false;
    QSaveFile output(outputZip);
    if (!output.open(QIODevice::WriteOnly) || output.write(bytes) != bytes.size() || !output.commit()) {
        if (error)
            *error = tr("Could not prepare extension package: %1").arg(output.errorString());
        return false;
    }
    return true;
}
void BrowserExtensionPackage::fetch(const QString &id)
{
    cancel();
    m_bytes.clear();
    m_redirects = 0;
    m_publicKey.clear();
    m_expectedId = id;
    const QUrl url = downloadUrl(id);
    if (url.isEmpty() || !m_directory.isValid()) {
        emit failed(tr("Enter a Chrome Web Store extension URL or its 32-letter ID."));
        return;
    }
    m_timeout->start(60000);
    request(url);
}
void BrowserExtensionPackage::request(const QUrl &url)
{
    if (!trustedDownload(url) || ++m_redirects > 5) {
        fail(tr("The extension server returned an unsupported download redirect."));
        return;
    }
    QNetworkRequest networkRequest(url);
    networkRequest.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::ManualRedirectPolicy);
    networkRequest.setRawHeader("Accept", "application/x-chrome-extension, application/zip");
    auto *reply = m_network->get(networkRequest);
    reply->setReadBufferSize(maximumPackageBytes + 1);
    m_reply = reply;
    connect(reply, &QNetworkReply::readyRead, this, [this, reply] {
        if (reply != m_reply)
            return;
        m_bytes += reply->readAll();
        if (m_bytes.size() > maximumPackageBytes)
            fail(tr("Extension package exceeds the 32 MiB download limit."));
    });
    connect(reply, &QNetworkReply::finished, this, [this, reply] {
        if (reply != m_reply)
            return;
        m_reply.clear();
        reply->deleteLater();
        const auto redirect = reply->attribute(QNetworkRequest::RedirectionTargetAttribute);
        if (redirect.isValid()) {
            m_bytes.clear();
            request(reply->url().resolved(redirect.toUrl()));
            return;
        }
        m_timeout->stop();
        if (reply->error() != QNetworkReply::NoError || reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt() != 200) {
            emit failed(tr("Could not download this extension from the Chrome Web Store: %1").arg(reply->errorString()));
            return;
        }
        m_bytes += reply->readAll();
        QString identityError;
        const QByteArray publicKey = BrowserExtensionCrx::extractPublicKey(m_bytes, m_expectedId, &identityError);
        if (publicKey.isEmpty()) {
            m_bytes.clear();
            emit failed(identityError);
            return;
        }
        m_publicKey = QString::fromLatin1(publicKey.toBase64());
        QFile raw(m_directory.filePath("download.crx"));
        if (m_bytes.size() > maximumPackageBytes || !raw.open(QIODevice::WriteOnly | QIODevice::Truncate)
            || raw.write(m_bytes) != m_bytes.size()) {
            emit failed(tr("Could not save the downloaded extension package."));
            return;
        }
        raw.close();
        m_bytes.clear();
        const QString zip = m_directory.filePath("extension.zip");
        QString error;
        if (prepareFile(raw.fileName(), zip, &error))
            emit ready(zip);
        else
            emit failed(error);
    });
}
void BrowserExtensionPackage::cancel()
{
    m_timeout->stop();
    if (m_reply) {
        auto *reply = m_reply.data();
        m_reply.clear();
        reply->abort();
        reply->deleteLater();
    }
    m_bytes.clear();
}
void BrowserExtensionPackage::fail(const QString &error)
{
    cancel();
    emit failed(error);
}
