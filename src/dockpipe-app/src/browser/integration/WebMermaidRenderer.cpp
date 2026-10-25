#include "WebMermaidRenderer.h"

#include <QBuffer>
#include <QImageReader>
#include <QJsonArray>
#include <QJsonDocument>
#include <QVariantMap>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineSettings>
#include <QWebEngineUrlRequestInfo>
#include <QWebEngineUrlRequestInterceptor>

namespace {
const QUrl rendererUrl(QStringLiteral("qrc:/mermaid/index.html"));
constexpr qsizetype maximumSourceCharacters = 50000;
constexpr qsizetype maximumEncodedImageBytes = 22 * 1024 * 1024;
constexpr qsizetype maximumDecodedImageBytes = 16 * 1024 * 1024;

class OfflineRequests final : public QWebEngineUrlRequestInterceptor {
public:
    using QWebEngineUrlRequestInterceptor::QWebEngineUrlRequestInterceptor;
    void interceptRequest(QWebEngineUrlRequestInfo &request) override
    {
        const QUrl url = request.requestUrl();
        const bool permitted = url == rendererUrl
            || url == QUrl(QStringLiteral("qrc:/mermaid/runtime.js"))
            || url == QUrl(QStringLiteral("qrc:/mermaid/mermaid.min.js"));
        request.block(!permitted);
    }
};

class OfflinePage final : public QWebEnginePage {
public:
    using QWebEnginePage::QWebEnginePage;
protected:
    bool acceptNavigationRequest(const QUrl &url, NavigationType, bool mainFrame) override
    {
        return mainFrame && url == rendererUrl;
    }
    QWebEnginePage *createWindow(WebWindowType) override { return nullptr; }
};
}

WebMermaidRenderer::WebMermaidRenderer(QObject *parent)
    : MermaidRenderer(parent), m_profile(new QWebEngineProfile(this))
{
    m_profile->setHttpCacheType(QWebEngineProfile::MemoryHttpCache);
    m_profile->setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
    m_profile->setUrlRequestInterceptor(new OfflineRequests(m_profile));
    m_timeout.setSingleShot(true);
    m_timeout.setInterval(10000);
    connect(&m_timeout, &QTimer::timeout, this, [this] {
        complete({}, tr("Mermaid rendering timed out."));
    });
}

WebMermaidRenderer::~WebMermaidRenderer()
{
    cancel();
    // Pages must be destroyed before their profile, including deferred pages.
    const auto pages = m_profile->findChildren<QWebEnginePage *>();
    qDeleteAll(pages);
}

void WebMermaidRenderer::cancel()
{
    ++m_generation;
    m_timeout.stop();
    // Only the page whose callback may currently be running needs deferred
    // destruction. Repeated replacements cannot accumulate retired pages.
    if (m_retiredPage)
        delete m_retiredPage.data();
    if (m_page) {
        m_page->triggerAction(QWebEnginePage::Stop);
        m_retiredPage = m_page;
        m_page->deleteLater();
        m_page = nullptr;
    }
}

void WebMermaidRenderer::complete(const QImage &image, const QString &error)
{
    cancel();
    const quint64 generation = m_generation;
    // Leave the browser callback before exposing completion: consumers may
    // immediately replace the render, cancel it, or delete this renderer.
    QTimer::singleShot(0, this, [this, generation, image, error] {
        if (generation == m_generation)
            emit finished(image, error);
    });
}

void WebMermaidRenderer::render(const QString &source, bool dark)
{
    cancel();
    if (source.size() > maximumSourceCharacters) {
        emit finished({}, tr("Mermaid diagrams are limited to 50,000 characters."));
        return;
    }
    const quint64 generation = m_generation;
    m_page = new OfflinePage(m_profile, m_profile);
    auto *settings = m_page->settings();
    settings->setAttribute(QWebEngineSettings::JavascriptCanOpenWindows, false);
    settings->setAttribute(QWebEngineSettings::JavascriptCanAccessClipboard, false);
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessRemoteUrls, false);
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessFileUrls, false);
    settings->setAttribute(QWebEngineSettings::LocalStorageEnabled, false);
    settings->setAttribute(QWebEngineSettings::PluginsEnabled, false);
    settings->setAttribute(QWebEngineSettings::FullScreenSupportEnabled, false);
    connect(m_page, &QWebEnginePage::loadFinished, this, [this, generation, source, dark](bool success) {
        if (generation != m_generation || !m_page)
            return;
        if (!success) {
            complete({}, tr("The bundled Mermaid renderer could not be loaded."));
            return;
        }
        const QByteArray arguments = QJsonDocument(QJsonArray{source, dark}).toJson(QJsonDocument::Compact);
        m_page->runJavaScript(QStringLiteral("window.dockpipeRender(...%1);").arg(QString::fromUtf8(arguments)));
        poll(generation);
    });
    connect(m_page, &QWebEnginePage::renderProcessTerminated, this,
            [this, generation](QWebEnginePage::RenderProcessTerminationStatus, int) {
        if (generation == m_generation && m_page)
            complete({}, tr("The Mermaid renderer stopped unexpectedly."));
    });
    m_timeout.start();
    m_page->load(rendererUrl);
}

void WebMermaidRenderer::poll(quint64 generation)
{
    if (generation != m_generation || !m_page)
        return;
    const QPointer<WebMermaidRenderer> guard(this);
    m_page->runJavaScript(QStringLiteral("window.dockpipeResult"), [this, guard, generation](const QVariant &value) {
        if (!guard)
            return;
        if (generation != m_generation || !m_page)
            return;
        const QVariantMap result = value.toMap();
        if (result.isEmpty()) {
            QTimer::singleShot(25, this, [this, generation] { poll(generation); });
            return;
        }
        if (result.contains(QStringLiteral("error"))) {
            complete({}, result.value(QStringLiteral("error")).toString().left(1000));
            return;
        }
        const QString encoded = result.value(QStringLiteral("png")).toString();
        const QString prefix = QStringLiteral("data:image/png;base64,");
        if (!encoded.startsWith(prefix) || encoded.size() > maximumEncodedImageBytes) {
            complete({}, tr("Mermaid returned an invalid or oversized image."));
            return;
        }
        QByteArray bytes = QByteArray::fromBase64(encoded.mid(prefix.size()).toLatin1(),
                                                QByteArray::AbortOnBase64DecodingErrors);
        if (bytes.isEmpty() || bytes.size() > maximumDecodedImageBytes) {
            complete({}, tr("Mermaid returned an invalid or oversized image."));
            return;
        }
        QBuffer buffer(&bytes);
        buffer.open(QIODevice::ReadOnly);
        QImageReader reader(&buffer, "PNG");
        const QSize size = reader.size();
        if (!size.isValid() || size.width() > 4096 || size.height() > 4096
            || qint64(size.width()) * size.height() > 4000000) {
            complete({}, tr("Mermaid returned an oversized image."));
            return;
        }
        const QImage image = reader.read();
        if (image.isNull() || image.sizeInBytes() > maximumDecodedImageBytes) {
            complete({}, tr("The Mermaid image could not be decoded."));
            return;
        }
        complete(image, {});
    });
}

void installWebMermaidRenderer()
{
    Q_INIT_RESOURCE(mermaid);
    MermaidRenderer::setFactory([](QObject *parent) { return new WebMermaidRenderer(parent); });
}
