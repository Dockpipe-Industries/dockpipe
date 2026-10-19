#include "BrowserLinkRouter.h"
#include "BrowserPreferences.h"

#include <QApplication>
#include <QDesktopServices>
#include <QStandardPaths>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <memory>

#define CHECK_LINK(expression) \
    do { \
        if (!(expression)) { \
            std::fprintf(stderr, "Browser link check at %d: %s\n", __LINE__, #expression); \
            return 1; \
        } \
    } while (false)

class FileHandler final : public QObject {
    Q_OBJECT
public:
    QUrl opened;
public slots:
    void open(const QUrl &url) { opened = url; }
};

int main(int argc, char **argv)
{
    QTemporaryDir directory;
    CHECK_LINK(directory.isValid());
    qputenv("XDG_CONFIG_HOME", directory.path().toUtf8());
    qputenv("APPDATA", directory.path().toUtf8());
    QApplication app(argc, argv);
    QCoreApplication::setOrganizationName("DockPipeRouterTest");
    QCoreApplication::setApplicationName("IsolatedLinks");
    // Never change the user's configuration on platforms with a different
    // config-location mechanism; this fixture requires temporary isolation.
    if (!QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation).startsWith(directory.path() + '/'))
        return 77;
    auto *preferences = BrowserPreferences::instance();
    BrowserSettings settings = preferences->settings();
    settings.webLinkDestination = "system";
    settings.localLinkDestination = "dockpipe";
    CHECK_LINK(preferences->save(settings));

    QList<QUrl> internal;
    QList<QUrl> external;
    int failures = 0;
    bool externalSuccess = true;
    FileHandler files;
    QDesktopServices::setUrlHandler("file", &files, "open");
    {
        BrowserLinkRouter router(nullptr, [&](const QUrl &url) {
            external.append(url);
            return externalSuccess;
        });
        QObject::connect(&router, &BrowserLinkRouter::openInDockPipe, &app,
            [&](const QUrl &url) { internal.append(url); });
        QObject::connect(&router, &BrowserLinkRouter::failed, &app,
            [&](const QString &) { ++failures; });
        const QUrl web("https://example.org/docs");
        const QUrl local("http://localhost:8123/app");
        CHECK_LINK(QDesktopServices::openUrl(web));
        CHECK_LINK(external == QList<QUrl>{web} && internal.isEmpty());
        CHECK_LINK(QDesktopServices::openUrl(local));
        CHECK_LINK(internal == QList<QUrl>{local} && external.size() == 1);

        settings.webLinkDestination = "dockpipe";
        settings.localLinkDestination = "system";
        CHECK_LINK(preferences->save(settings));
        CHECK_LINK(QDesktopServices::openUrl(web));
        CHECK_LINK(internal.last() == web && internal.size() == 2);
        CHECK_LINK(QDesktopServices::openUrl(local));
        CHECK_LINK(external.last() == local && external.size() == 2);
        CHECK_LINK(BrowserLinkRouter::openInSystemBrowser(web));
        CHECK_LINK(external.last() == web && external.size() == 3 && internal.size() == 2);
        externalSuccess = false;
        CHECK_LINK(!BrowserLinkRouter::openInSystemBrowser(web));
        CHECK_LINK(failures == 1 && internal.size() == 2);
        externalSuccess = true;
        CHECK_LINK(QDesktopServices::openUrl(web));
        CHECK_LINK(internal.size() == 3); // The explicit external bypass was scoped.

        const int externalBeforeInvalid = external.size();
        CHECK_LINK(!BrowserLinkRouter::openInSystemBrowser(QUrl("https://user:secret@example.org")));
        CHECK_LINK(!BrowserLinkRouter::openInSystemBrowser(QUrl("file:///tmp/test")));
        QDesktopServices::openUrl(QUrl("https://user:secret@example.org"));
        CHECK_LINK(failures == 2 && external.size() == externalBeforeInvalid && internal.size() == 3);

        const QUrl file = QUrl::fromLocalFile(directory.filePath("document.txt"));
        CHECK_LINK(QDesktopServices::openUrl(file));
        CHECK_LINK(files.opened == file && external.size() == externalBeforeInvalid);
        std::unique_ptr<QThread> worker(QThread::create([web]() { QDesktopServices::openUrl(web); }));
        worker->start();
        CHECK_LINK(worker->wait(5000));
        CHECK_LINK(internal.size() == 3);
        QApplication::processEvents();
        CHECK_LINK(internal.size() == 4 && internal.last() == web);
    }
    const QUrl after = QUrl::fromLocalFile(directory.filePath("after.txt"));
    CHECK_LINK(QDesktopServices::openUrl(after) && files.opened == after);
    QDesktopServices::unsetUrlHandler("file");
    std::puts("Link preferences, local classification, explicit external bypass, failure, thread delivery and untouched file handlers passed.");
    return 0;
}

#include "BrowserLinkRouterTest.moc"
