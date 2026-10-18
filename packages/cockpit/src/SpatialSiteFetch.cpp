#include "SpatialSiteFetch.h"
#include "SpatialSitePolicy.h"
#include <QHostInfo>
#include <QNetworkReply>
#include <QNetworkRequest>
#include <QNetworkProxy>
#include <QSslConfiguration>

SpatialSiteFetch::SpatialSiteFetch(QObject *parent) : QObject(parent), m_network(this)
{
    m_network.setProxy(QNetworkProxy::NoProxy);
    m_deadline.setSingleShot(true);
    m_deadline.setInterval(8000);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        auto response = m_response;
        response.error = "Public fetch timed out";
        cancel();
        emit completed(response);
    });
}
SpatialSiteFetch::~SpatialSiteFetch() { cancel(); }
void SpatialSiteFetch::cancel()
{
    ++m_generation;
    m_busy = false;
    m_deadline.stop();
    if (m_lookup >= 0)
        QHostInfo::abortHostLookup(m_lookup);
    m_lookup = -1;
    if (m_reply) {
        m_reply->disconnect(this);
        m_reply->abort();
        m_reply->deleteLater();
        m_reply = nullptr;
    }
    m_response = {};
    m_network.clearAccessCache();
}
void SpatialSiteFetch::finish(const SpatialSiteResponse &response)
{
    const auto copy = response;
    cancel();
    emit completed(copy);
}
void SpatialSiteFetch::get(const QUrl &url, const QUrl &origin, int maximumBytes)
{
    cancel();
    m_maximumBytes = qBound(1, maximumBytes, 512 * 1024);
    m_response.url = url;
    m_busy = true;
    const QString restriction = SpatialSitePolicy::restriction(url, origin);
    if (!restriction.isEmpty()) {
        m_response.error = restriction;
        const auto generation = m_generation;
        QTimer::singleShot(0, this, [this, generation]() {
            if (generation == m_generation)
                finish(m_response);
        });
        return;
    }
    m_deadline.start();
    const auto generation = m_generation;
    m_lookup = QHostInfo::lookupHost(url.host(), this, [this, url, generation](const QHostInfo &info) {
        if (generation != m_generation)
            return;
        m_lookup = -1;
        if (info.error() != QHostInfo::NoError || info.addresses().isEmpty() || info.addresses().size() > 32) {
            m_response.error = "Site DNS lookup failed or exceeded the address budget";
            finish(m_response);
            return;
        }
        for (const auto &address : info.addresses()) {
            if (!SpatialSitePolicy::publicAddress(address)) {
                m_response.error = "DNS resolved to a restricted network address";
                finish(m_response);
                return;
            }
        }
        // The connection uses the validated address, not another hostname lookup.
        // TLS verification/SNI and HTTP Host retain the original public hostname.
        QUrl pinned = url;
        pinned.setHost(info.addresses().first().toString());
        QNetworkRequest request(pinned);
        request.setPeerVerifyName(url.host());
        request.setRawHeader("Host", url.authority(QUrl::FullyEncoded).toUtf8());
        request.setRawHeader("User-Agent", "DockpipeSpatial/1.0");
        request.setRawHeader("Accept", "text/html,application/xhtml+xml,application/xml,text/xml,text/plain,image/*;q=0.5");
        request.setRawHeader("Accept-Encoding", "identity");
        request.setAttribute(QNetworkRequest::RedirectPolicyAttribute, QNetworkRequest::ManualRedirectPolicy);
        request.setAttribute(QNetworkRequest::CookieLoadControlAttribute, QNetworkRequest::Manual);
        request.setAttribute(QNetworkRequest::CookieSaveControlAttribute, QNetworkRequest::Manual);
        request.setAttribute(QNetworkRequest::AuthenticationReuseAttribute, QNetworkRequest::Manual);
        request.setAttribute(QNetworkRequest::Http2AllowedAttribute, false);
        request.setTransferTimeout(8000);
        m_reply = m_network.get(request);
        m_reply->setReadBufferSize(64 * 1024);
        connect(m_reply, &QNetworkReply::metaDataChanged, this, [this]() {
            if (!m_reply)
                return;
            if (m_reply->header(QNetworkRequest::ContentLengthHeader).toLongLong() > m_maximumBytes
                || (!m_reply->rawHeader("Content-Encoding").isEmpty() && m_reply->rawHeader("Content-Encoding") != "identity")) {
                m_response.error = "Compressed or oversized response rejected";
                finish(m_response);
            }
        });
        connect(m_reply, &QIODevice::readyRead, this, [this]() {
            if (!m_reply)
                return;
            m_response.body += m_reply->read(m_maximumBytes + 1 - m_response.body.size());
            if (m_response.body.size() > m_maximumBytes) {
                m_response.error = "Response byte budget exceeded";
                finish(m_response);
            }
        });
        connect(m_reply, &QNetworkReply::finished, this, [this]() {
            if (!m_reply)
                return;
            m_response.body += m_reply->read(m_maximumBytes + 1 - m_response.body.size());
            m_response.status = m_reply->attribute(QNetworkRequest::HttpStatusCodeAttribute).toInt();
            m_response.contentType = m_reply->rawHeader("Content-Type").toLower();
            m_response.modified = QString::fromLatin1(m_reply->rawHeader("Last-Modified")).left(64);
            m_response.robots = m_reply->rawHeader("X-Robots-Tag").toLower();
            const auto target = m_reply->attribute(QNetworkRequest::RedirectionTargetAttribute).toUrl();
            if (!target.isEmpty())
                m_response.redirect = m_response.url.resolved(target);
            if (m_response.body.size() > m_maximumBytes)
                m_response.error = "Response byte budget exceeded";
            else if (m_reply->error() != QNetworkReply::NoError && m_response.status == 0)
                m_response.error = m_reply->errorString();
            finish(m_response);
        });
    });
}
