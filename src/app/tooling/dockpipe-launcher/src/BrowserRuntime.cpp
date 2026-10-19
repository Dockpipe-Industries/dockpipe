#include "BrowserRuntime.h"

#include <QCoreApplication>
#include <QFileInfo>
#include <QLibraryInfo>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineSettings>
#include <QWebEngineView>
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#include <QWebEngineExtensionManager>
#endif
#endif

namespace {
class BrowserPage final : public QWebEnginePage {
public:
    BrowserPage(QWebEngineProfile *profile, QObject *parent)
        : QWebEnginePage(profile, parent) {}

protected:
    bool acceptNavigationRequest(const QUrl &url, NavigationType, bool) override
    {
        return BrowserRuntime::allowsNavigation(url, profile()) || url == QUrl("about:blank");
    }
};
}

bool BrowserRuntime::allowsUrl(const QUrl &url)
{
    return url.isValid() && (url.scheme() == "https" || url.scheme() == "http")
        && !url.host().isEmpty() && url.userInfo().isEmpty() && url.toEncoded().size() <= 4096;
}

bool BrowserRuntime::allowsNavigation(const QUrl &url, QWebEngineProfile *profile)
{
    if (allowsUrl(url))
        return true;
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
    if (profile && !profile->isOffTheRecord() && url.isValid()
        && url.scheme() == "chrome-extension" && url.userInfo().isEmpty()
        && url.toEncoded().size() <= 4096) {
        for (const auto &extension : profile->extensionManager()->extensions()) {
            if (extension.isEnabled() && extension.isLoaded() && extension.id() == url.host())
                return true;
        }
    }
#endif
#endif
    Q_UNUSED(profile);
    return false;
}

QString BrowserRuntime::rendererError()
{
#if defined(Q_OS_LINUX)
    QString renderer = qEnvironmentVariable("QTWEBENGINEPROCESS_PATH");
    if (renderer.isEmpty()) {
        renderer = QLibraryInfo::path(QLibraryInfo::LibraryExecutablesPath) + "/QtWebEngineProcess";
        if (!QFileInfo(renderer).isExecutable()) {
            const QString bundled = QCoreApplication::applicationDirPath() + "/QtWebEngineProcess";
            if (QFileInfo(bundled).isExecutable()) {
                renderer = bundled;
                qputenv("QTWEBENGINEPROCESS_PATH", renderer.toUtf8());
            }
        }
    }
    if (!QFileInfo(renderer).isExecutable())
        return QCoreApplication::translate("BrowserRuntime",
            "The Qt WebEngine renderer is missing. Reinstall the desktop browser runtime to browse here.");
#endif
    return {};
}

QWebEngineView *BrowserRuntime::createView(QWebEngineProfile *profile, QWidget *parent)
{
    auto *view = new QWebEngineView(parent);
    view->setPage(new BrowserPage(profile, view));
    view->settings()->setAttribute(QWebEngineSettings::JavascriptCanOpenWindows, false);
    view->settings()->setAttribute(QWebEngineSettings::LocalContentCanAccessFileUrls, false);
    view->settings()->setAttribute(QWebEngineSettings::FullScreenSupportEnabled, false);
    view->settings()->setAttribute(QWebEngineSettings::PlaybackRequiresUserGesture, true);
    return view;
}
