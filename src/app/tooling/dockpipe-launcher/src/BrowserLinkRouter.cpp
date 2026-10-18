#include "BrowserLinkRouter.h"
#include "BrowserPreferences.h"

#include <QDesktopServices>
#include <QScopedValueRollback>
#include <QThread>
#include <utility>

namespace {
struct ExternalRequest {
    bool handled = false;
    bool success = false;
};

thread_local ExternalRequest *explicitExternalRequest = nullptr;

bool validWebUrl(const QUrl &url)
{
    // Match the address-entry contract before routing a desktop-originated URL.
    return url.isValid() && (url.scheme() == "http" || url.scheme() == "https")
        && !url.host().isEmpty() && url.userInfo().isEmpty() && url.toEncoded().size() <= 4096;
}
}

BrowserLinkRouter::BrowserLinkRouter(QObject *parent, ExternalOpener externalOpener)
    : QObject(parent), m_externalOpener(std::move(externalOpener))
{
    if (!m_externalOpener)
        m_externalOpener = [](const QUrl &url) { return QDesktopServices::openUrl(url); };
    QDesktopServices::setUrlHandler("http", this, "openUrl");
    QDesktopServices::setUrlHandler("https", this, "openUrl");
}

BrowserLinkRouter::~BrowserLinkRouter()
{
    // The launcher has one MainWindow/router. Unregister before QObject teardown,
    // restoring Qt's default desktop handlers and preventing stale callbacks.
    QDesktopServices::unsetUrlHandler("http");
    QDesktopServices::unsetUrlHandler("https");
}

bool BrowserLinkRouter::openInSystemBrowser(const QUrl &url)
{
    if (!validWebUrl(url))
        return false;
    ExternalRequest request;
    QScopedValueRollback<ExternalRequest *> bypass(explicitExternalRequest, &request);
    const bool dispatched = QDesktopServices::openUrl(url);
    return request.handled ? request.success : dispatched;
}

void BrowserLinkRouter::openUrl(const QUrl &url)
{
    if (!validWebUrl(url)) {
        emit failed(tr("The web link is invalid or contains embedded credentials."));
        return;
    }
    if (explicitExternalRequest) {
        explicitExternalRequest->handled = true;
        explicitExternalRequest->success = openExternal(url);
        return;
    }
    // QDesktopServices invokes handlers on the caller's thread. Preferences and
    // UI delivery belong to our desktop thread; re-enter dispatch there later.
    if (QThread::currentThread() != thread()) {
        QMetaObject::invokeMethod(this, [url]() { QDesktopServices::openUrl(url); }, Qt::QueuedConnection);
        return;
    }
    const BrowserSettings settings = BrowserPreferences::instance()->settings();
    const QString destination = BrowserPreferences::isLocalUrl(url)
        ? settings.localLinkDestination : settings.webLinkDestination;
    if (destination == "dockpipe")
        emit openInDockPipe(url);
    else
        openExternal(url);
}

bool BrowserLinkRouter::openExternal(const QUrl &url)
{
    // Qt explicitly supports calling openUrl again inside its scheme handler:
    // the nested call uses the OS desktop mechanism instead of recursing here.
    // https://doc.qt.io/qt-6/qdesktopservices.html#setUrlHandler
    const bool opened = m_externalOpener(url);
    if (!opened)
        emit failed(tr("The system browser could not open this web link."));
    return opened;
}
