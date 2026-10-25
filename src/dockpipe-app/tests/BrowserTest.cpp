#include "BrowserRuntime.h"
#include "BrowserWidget.h"
#include "BrowserTabBar.h"
#include "BrowserPreferences.h"
#if QT_VERSION >= QT_VERSION_CHECK(6, 8, 0)
#include <QWebEnginePermission>
#endif
#include "Theme.h"

#include <QAction>
#include <QApplication>
#include <QCheckBox>
#include <QElapsedTimer>
#include <QFileDialog>
#include <QFile>
#include <QLabel>
#include <QMessageBox>
#include <QPrintDialog>
#include <QPrinter>
#include <QDir>
#include <QMenu>
#include <QTabBar>
#include <QSignalSpy>
#include <QTreeWidget>
#include <QListWidget>
#include <QLineEdit>
#include <QPointer>
#include <QPushButton>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <QTimer>
#include <QWebEngineDownloadRequest>
#include <QWebEngineHistory>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineView>
#include <QWebEngineSettings>
#include <functional>
#include <cstdio>

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (elapsed.elapsed() < 10000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

#define CHECK_BROWSER(expression) \
    do { \
        if (!(expression)) { \
            std::fprintf(stderr, "Browser check failed at line %d: %s\n", __LINE__, #expression); \
            return 1; \
        } \
    } while (false)

static bool startBrowserServer(QTcpServer &server)
{
    if (!server.listen(QHostAddress::LocalHost))
        return false;
    QObject::connect(&server, &QTcpServer::newConnection, &server, [&]() {
        while (server.hasPendingConnections()) {
            auto *socket = server.nextPendingConnection();
            QObject::connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            QObject::connect(socket, &QTcpSocket::readyRead, socket, [socket]() {
                QByteArray request = socket->property("request").toByteArray() + socket->readAll();
                if (!request.contains("\r\n\r\n")) {
                    socket->setProperty("request", request);
                    return;
                }
                if (request.startsWith("GET /slow "))
                    return;
                const bool second = request.startsWith("GET /second ");
                const QByteArray title = second ? "Second page" : "Browser test page";
                const QByteArray body = "<!doctype html><html><head><title>" + title
                    + "</title></head><body style='font-family:sans-serif;padding:48px;background:#eef3fa'>"
                      "<h1>" + title + "</h1><p>DockPipe Browser is rendering this local test website.</p>"
                      "<a href='/second'>Open the second page</a><script>document.body.dataset.ready='yes'</script>"
                      "</body></html>";
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nConnection: close\r\nContent-Length: "
                    + QByteArray::number(body.size()) + "\r\n\r\n" + body);
                socket->disconnectFromHost();
            });
        }
    });
    return true;
}

static int runBrowserChecks()
{
    CHECK_BROWSER(BrowserRuntime::allowsUrl(QUrl("https://example.com")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("file:///tmp/private")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("https://user:password@example.com")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("javascript:alert(1)")));
    CHECK_BROWSER(!BrowserWidget::addressUrl(" ").isValid());
    CHECK_BROWSER(BrowserWidget::addressUrl("example.com").host() == "example.com");
    CHECK_BROWSER(BrowserWidget::addressUrl("weather tomorrow").host() == "duckduckgo.com");

    QTcpServer server;
    CHECK_BROWSER(startBrowserServer(server));
    const QUrl first(QString("http://127.0.0.1:%1/").arg(server.serverPort()));
    QString sessionPath;
    QPointer<QWebEngineProfile> profile;
    QPointer<QWebEnginePage> page;
    {
        BrowserWidget browser;
        browser.resize(1100, 720);
        browser.show();
        auto *view = browser.findChild<QWebEngineView *>("browserWebView");
        CHECK_BROWSER(view);
        profile = view->page()->profile();
        page = view->page();
        CHECK_BROWSER(!profile->isOffTheRecord());
        sessionPath = profile->persistentStoragePath();
        CHECK_BROWSER(sessionPath.startsWith(QDir::tempPath()));
        {
            BrowserWidget separateBrowser;
            CHECK_BROWSER(separateBrowser.activeView());
            CHECK_BROWSER(separateBrowser.activeView()->page()->profile() != profile);
            CHECK_BROWSER(separateBrowser.activeView()->page()->profile()->persistentStoragePath() != sessionPath);
        }
        auto *address = browser.findChild<QLineEdit *>("browserAddress");
        auto *status = browser.findChild<QLabel *>("browserStatus");
        auto *stop = browser.findChild<QPushButton *>("browserStop");
        CHECK_BROWSER(address && status && stop);
        auto *annotateButton = browser.findChild<QPushButton *>("browserAnnotate");
        auto *extensionsButton = browser.findChild<QPushButton *>("browserExtensions");
        auto *downloadsButton = browser.findChild<QPushButton *>("browserDownloads");
        auto *siteButton = browser.findChild<QPushButton *>("browserSiteInfo");
        auto *moreButton = browser.findChild<QPushButton *>("browserMore");
        CHECK_BROWSER(annotateButton && extensionsButton && downloadsButton && siteButton && moreButton);
        CHECK_BROWSER(annotateButton->isVisible() && extensionsButton->isVisible() && downloadsButton->isVisible());
        browser.resize(440, 720);
        const bool compactToolbarReady = waitUntil([&]() {
            return !annotateButton->isVisible() && !extensionsButton->isVisible()
                && !downloadsButton->isVisible() && address->width() >= 160;
        });
        if (!compactToolbarReady || browser.width() != 440) {
            std::fprintf(stderr, "Compact toolbar: browser=%d minimum=%d hint=%d address=%d annotate=%d extensions=%d downloads=%d\n",
                browser.width(), browser.minimumWidth(), browser.minimumSizeHint().width(), address->width(),
                annotateButton->isVisible(), extensionsButton->isVisible(), downloadsButton->isVisible());
        }
        CHECK_BROWSER(compactToolbarReady);
        CHECK_BROWSER(siteButton->isVisible() && moreButton->isVisible());
        CHECK_BROWSER(browser.width() == 440);
        CHECK_BROWSER(browser.findChild<QAction *>("browserAnnotateAction"));
        CHECK_BROWSER(browser.findChild<QPushButton *>("browserBack")->isVisible());
        CHECK_BROWSER(browser.findChild<QPushButton *>("browserForward")->isVisible());
        CHECK_BROWSER(browser.findChild<QPushButton *>("browserReload")->isVisible());
        browser.resize(1100, 720);
        CHECK_BROWSER(waitUntil([&]() {
            return annotateButton->isVisible() && extensionsButton->isVisible() && downloadsButton->isVisible();
        }));
        address->setText(first.toString());
        browser.findChild<QPushButton *>("browserGo")->click();
        CHECK_BROWSER(waitUntil([&]() { return view->title() == "Browser test page" && !stop->isEnabled(); }));
        bool scriptFinished = false;
        view->page()->runJavaScript("document.body.dataset.ready", [&](const QVariant &result) {
            scriptFinished = result.toString() == "yes";
        });
        CHECK_BROWSER(waitUntil([&]() { return scriptFinished; }));
        auto preferences = BrowserPreferences::instance()->settings();
        preferences.showFullUrl = true;
        preferences.images = false;
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        if (address->text() != first.toString())
            std::fprintf(stderr, "Address preference: shown=%s expected=%s actual=%s full=%d focused=%d modified=%d\n",
                qPrintable(address->text()), qPrintable(first.toString()), qPrintable(view->url().toString()),
                BrowserPreferences::instance()->settings().showFullUrl, address->hasFocus(), address->isModified());
        CHECK_BROWSER(address->text() == first.toString());
        CHECK_BROWSER(!view->settings()->testAttribute(QWebEngineSettings::AutoLoadImages));
        preferences.showFullUrl = false;
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        browser.activateWindow();
        address->setFocus();
        CHECK_BROWSER(waitUntil([&]() { return address->hasFocus() && address->text() == first.toString(); }));
        view->setFocus();
        CHECK_BROWSER(waitUntil([&]() { return !address->hasFocus() && address->text() == first.authority(); }));
        {
            BrowserWidget otherBrowser;
            CHECK_BROWSER(!otherBrowser.activeView()->settings()->testAttribute(QWebEngineSettings::AutoLoadImages));
            preferences.images = true;
            CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
            CHECK_BROWSER(otherBrowser.activeView()->settings()->testAttribute(QWebEngineSettings::AutoLoadImages));
        }
        // Exercise the page permission signal without depending on OS camera or
        // location hardware. The origin and decision use the real browser page.
        view->page()->featurePermissionRequested(first, QWebEnginePage::Notifications);
        auto *permissionPrompt = browser.findChild<QMessageBox *>("browserPermissionPrompt");
        CHECK_BROWSER(permissionPrompt);
        permissionPrompt->checkBox()->setChecked(true);
        permissionPrompt->button(QMessageBox::No)->click();
        CHECK_BROWSER(BrowserPreferences::permission(BrowserPreferences::instance()->settings(), first, "notifications") == "block");
        QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        CHECK_BROWSER(!browser.findChild<QMessageBox *>("browserPermissionPrompt"));
        view->page()->featurePermissionRequested(first, QWebEnginePage::Notifications);
        CHECK_BROWSER(!browser.findChild<QMessageBox *>("browserPermissionPrompt"));
        for (int transition = 0; transition < 3; ++transition) {
            view->page()->featurePermissionRequested(first, QWebEnginePage::Geolocation);
            QPointer<QMessageBox> pendingPrompt = browser.findChild<QMessageBox *>("browserPermissionPrompt");
            CHECK_BROWSER(pendingPrompt);
            pendingPrompt->checkBox()->setChecked(true);
            if (transition == 0)
                browser.setChromeVisible(false);
            else if (transition == 1)
                browser.setBackgroundSuspended(true);
            else
                browser.hide();
            CHECK_BROWSER(!pendingPrompt || !pendingPrompt->isVisible());
            CHECK_BROWSER(BrowserPreferences::permission(BrowserPreferences::instance()->settings(), first, "location") == "ask");
            browser.show();
            browser.setChromeVisible(true);
            browser.setBackgroundSuspended(false);
            QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
        }
#if QT_VERSION >= QT_VERSION_CHECK(6, 8, 0)
        const QUrl frameOrigin("https://embedded.example.test");
        preferences = BrowserPreferences::instance()->settings();
        preferences.sitePermissions[BrowserPreferences::originKey(frameOrigin)]["notifications"] = "allow";
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        view->page()->featurePermissionRequested(frameOrigin, QWebEnginePage::Notifications);
        CHECK_BROWSER(profile->queryPermission(frameOrigin, QWebEnginePermission::PermissionType::Notifications).state()
            == QWebEnginePermission::State::Granted);
        preferences.sitePermissions[BrowserPreferences::originKey(frameOrigin)]["notifications"] = "block";
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        CHECK_BROWSER(profile->queryPermission(frameOrigin, QWebEnginePermission::PermissionType::Notifications).state()
            == QWebEnginePermission::State::Denied);
#endif
        browser.navigate(QUrl("file:///tmp/private"));
        CHECK_BROWSER(view->url() == first && status->text().contains("HTTP"));
        if (QApplication::platformName() == "offscreen") {
            // The offscreen compositor cannot deliver native WebEngine input.
            // Still verify real document loads and history through the address bar.
            address->setText(first.resolved(QUrl("/second")).toString());
            browser.findChild<QPushButton *>("browserGo")->click();
        } else {
            // Chromium intentionally skips script-only redirects in Back history;
            // verify a user-activated link with native input instead of JS click().
            QPoint linkPosition;
            bool linkLocated = false;
            view->page()->runJavaScript(
                "(() => { const r = document.querySelector('a').getBoundingClientRect(); return [r.x + 8, r.y + 8]; })()",
                [&](const QVariant &result) {
                    const auto coordinates = result.toList();
                    if (coordinates.size() == 2) {
                        linkPosition = QPoint(coordinates[0].toInt(), coordinates[1].toInt());
                        linkLocated = true;
                    }
                });
            CHECK_BROWSER(waitUntil([&]() { return linkLocated; }));
            CHECK_BROWSER(view->focusProxy());
            QTest::mouseClick(view->focusProxy(), Qt::LeftButton, Qt::NoModifier, linkPosition);
        }
        CHECK_BROWSER(waitUntil([&]() { return view->title() == "Second page" && !stop->isEnabled(); }));
        CHECK_BROWSER(view->history()->canGoBack());
        CHECK_BROWSER(browser.findChild<QPushButton *>("browserBack")->isEnabled());
        browser.findChild<QPushButton *>("browserBack")->click();
        const bool returned = waitUntil([&]() { return view->url() == first && !stop->isEnabled(); });
        if (!returned)
            std::fprintf(stderr, "Back navigation: url=%s back=%d history=%d stop=%d status=%s\n",
                qPrintable(view->url().toString()), browser.findChild<QPushButton *>("browserBack")->isEnabled(),
                view->history()->canGoBack(), stop->isEnabled(), qPrintable(status->text()));
        CHECK_BROWSER(returned);
        CHECK_BROWSER(browser.findChild<QPushButton *>("browserForward")->isEnabled());
        QTemporaryDir downloads;
        CHECK_BROWSER(downloads.isValid());
        preferences = BrowserPreferences::instance()->settings();
        preferences.downloadDirectory = downloads.path();
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        QPointer<QWebEngineDownloadRequest> cancelledDownload;
        bool downloadRequested = false;
        QObject::connect(profile, &QWebEngineProfile::downloadRequested, &browser,
            [&](QWebEngineDownloadRequest *download) {
                downloadRequested = true;
                cancelledDownload = download;
                const auto recordDownload = [&browser, download]() {
                    browser.setProperty("testDownloadDetails", QStringLiteral("state=%1 reason=%2 directory=%3 file=%4")
                        .arg(static_cast<int>(download->state()))
                        .arg(download->interruptReasonString(), download->downloadDirectory(), download->downloadFileName()));
                };
                recordDownload();
                QObject::connect(download, &QWebEngineDownloadRequest::stateChanged, &browser, recordDownload);
            });
        QTimer dismissSaveDialog;
        dismissSaveDialog.setInterval(20);
        QObject::connect(&dismissSaveDialog, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QFileDialog *>())
                dialog->reject();
        });
        dismissSaveDialog.start();
        view->page()->download(first, "browser-test.html");
        CHECK_BROWSER(waitUntil([&]() { return downloadRequested && cancelledDownload.isNull(); }));
        dismissSaveDialog.stop();
        CHECK_BROWSER(status->text().contains("cancelled"));
        const QString savedFile = downloads.filePath("saved.html");
        downloadRequested = false;
        bool savePathSelected = false;
        QTimer acceptSaveDialog;
        acceptSaveDialog.setInterval(20);
        QObject::connect(&acceptSaveDialog, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QFileDialog *>()) {
                if (!dialog->isVisible())
                    return;
                auto *fileNameEditor = dialog->findChild<QLineEdit *>("fileNameEdit");
                if (!fileNameEditor) {
                    dialog->reject();
                    return;
                }
                // selectFile() need not replace the filename in an already
                // visible dialog. Edit the same field used for user input.
                fileNameEditor->setText(savedFile);
                savePathSelected = dialog->selectedFiles() == QStringList{savedFile};
                if (!savePathSelected) {
                    dialog->reject();
                    return;
                }
                QMetaObject::invokeMethod(dialog, "accept", Qt::DirectConnection);
            }
        });
        acceptSaveDialog.start();
        view->page()->download(first, "browser-test.html");
        CHECK_BROWSER(waitUntil([&]() { return downloadRequested && cancelledDownload.isNull(); }));
        acceptSaveDialog.stop();
        CHECK_BROWSER(savePathSelected);
        QFile downloaded(savedFile);
        const bool downloadOpened = downloaded.open(QIODevice::ReadOnly);
        if (!downloadOpened)
            std::fprintf(stderr, "Accepted download unreadable: expected=%s error=%s status=%s request=%s files=%s\n",
                qPrintable(savedFile), qPrintable(downloaded.errorString()), qPrintable(status->text()),
                qPrintable(browser.property("testDownloadDetails").toString()),
                qPrintable(QDir(downloads.path()).entryList(QDir::Files).mid(0, 20).join(", ")));
        CHECK_BROWSER(downloadOpened);
        CHECK_BROWSER(downloaded.readAll().contains("Browser test page"));
        CHECK_BROWSER(status->text().contains("Download saved"));
        preferences = BrowserPreferences::instance()->settings();
        preferences.askWhereToSave = false;
        preferences.downloadDirectory = downloads.path();
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        QFile existing(downloads.filePath("duplicate.html"));
        CHECK_BROWSER(existing.open(QIODevice::WriteOnly));
        CHECK_BROWSER(existing.write("keep original") == 13);
        existing.close();
        view->page()->download(first, "duplicate.html");
        view->page()->download(first, "duplicate.html");
        CHECK_BROWSER(waitUntil([&]() {
            return QDir(downloads.path()).entryList({"*.html"}, QDir::Files).size() == 4;
        }));
        CHECK_BROWSER(!browser.findChild<QFileDialog *>());
        CHECK_BROWSER(existing.open(QIODevice::ReadOnly) && existing.readAll() == "keep original");
        existing.close();
        preferences.askWhereToSave = true;
        CHECK_BROWSER(BrowserPreferences::instance()->save(preferences));
        browser.findChild<QPushButton *>("browserDownloads")->click();
        auto *downloadList = browser.findChild<QTreeWidget *>("browserDownloadsList");
        CHECK_BROWSER(downloadList && downloadList->topLevelItemCount() >= 1);
        browser.findChild<QPushButton *>("browserDownloadsClear")->click();
        CHECK_BROWSER(downloadList->topLevelItemCount() == 0 && QFile::exists(savedFile));
        if (auto *panel = downloadList->window())
            panel->close();
        auto *tabs = browser.findChild<QTabBar *>("browserTabs");
        CHECK_BROWSER(tabs && tabs->count() == 1);
        bool lifecycleOrdered = true;
        int activeChanges = 0;
        const auto beforeChange = QObject::connect(&browser, &BrowserWidget::activeViewAboutToChange, &browser,
            [&](QWebEngineView *previous) {
                lifecycleOrdered = lifecycleOrdered && previous
                    && previous == browser.activeView() && previous->page();
            });
        const auto afterChange = QObject::connect(&browser, &BrowserWidget::activeViewChanged, &browser,
            [&](QWebEngineView *current) {
                lifecycleOrdered = lifecycleOrdered && current == browser.activeView();
                ++activeChanges;
            });
        browser.findChild<QPushButton *>("browserNewTab")->click();
        CHECK_BROWSER(tabs->count() == 2);
        tabs->setCurrentIndex(0);
        CHECK_BROWSER(view->url() == first);
        QMetaObject::invokeMethod(tabs, "tabCloseRequested", Q_ARG(int, 1));
        CHECK_BROWSER(tabs->count() == 1);
        // Closing a background tab must not detach the selected page or its
        // spatial media controller.
        CHECK_BROWSER(lifecycleOrdered && activeChanges == 2);
        auto *menu = browser.findChild<QMenu *>("browserMenu");
        CHECK_BROWSER(menu);
        const auto actionNamed = [menu](const QString &text) -> QAction * {
            for (auto *action : menu->findChildren<QAction *>()) {
                if (action->text() == text)
                    return action;
            }
            return nullptr;
        };
        for (auto *action : menu->actions()) {
            if (action->text() == "Find in page")
                action->trigger();
        }
        auto *find = browser.findChild<QLineEdit *>("browserFindText");
        CHECK_BROWSER(find && find->isVisible());
        find->setText("DockPipe");
        CHECK_BROWSER(waitUntil([&]() {
            for (auto *label : browser.findChildren<QLabel *>()) {
                if (label->text() == "1 of 1")
                    return true;
            }
            return false;
        }));
        browser.findChild<QPushButton *>("browserFindClose")->click();
        const QString capture = qEnvironmentVariable("DOCKPIPE_BROWSER_CAPTURE");
        if (!capture.isEmpty()) {
            QApplication::processEvents();
            CHECK_BROWSER(browser.grab().save(capture));
            menu->popup(browser.mapToGlobal(QPoint(browser.width() - menu->sizeHint().width(), 100)));
            QApplication::processEvents();
            CHECK_BROWSER(menu->grab().save(capture + ".menu.png"));
            menu->close();
        }
        auto *phone = actionNamed("Phone · 390 px");
        auto *fullWidth = actionNamed("Full width");
        CHECK_BROWSER(phone && fullWidth);
        phone->trigger();
        CHECK_BROWSER(waitUntil([&]() { return view->width() == 390; }));
        fullWidth->trigger();
        CHECK_BROWSER(waitUntil([&]() { return view->width() == browser.width(); }));
        auto *history = actionNamed("History…");
        CHECK_BROWSER(history);
        history->trigger();
        auto *historyList = browser.findChild<QListWidget *>("browserHistoryList");
        CHECK_BROWSER(historyList && historyList->count() >= 2);
        historyList->window()->close();
        const QString screenshotPath = downloads.filePath("page.png");
        QTimer saveScreenshot;
        saveScreenshot.setInterval(20);
        QObject::connect(&saveScreenshot, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QFileDialog *>()) {
                dialog->selectFile(screenshotPath);
                QMetaObject::invokeMethod(dialog, "accept", Qt::DirectConnection);
            }
        });
        auto *screenshot = actionNamed("Take a screenshot…");
        CHECK_BROWSER(screenshot);
        saveScreenshot.start();
        screenshot->trigger();
        saveScreenshot.stop();
        CHECK_BROWSER(!QImage(screenshotPath).isNull());
        CHECK_BROWSER(actionNamed("Print…"));
        QTimer confirmClear;
        confirmClear.setInterval(20);
        QObject::connect(&confirmClear, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QMessageBox *>())
                dialog->button(QMessageBox::Yes)->click();
        });
        auto *clear = actionNamed("Clear browsing data…");
        CHECK_BROWSER(clear);
        confirmClear.start();
        clear->trigger();
        confirmClear.stop();
        CHECK_BROWSER(page.isNull() && profile.isNull() && tabs->count() == 1);
        CHECK_BROWSER(waitUntil([&]() { return !QDir(sessionPath).exists(); }));
        view = browser.findChild<QWebEngineView *>("browserWebView");
        CHECK_BROWSER(view);
        page = view->page();
        profile = page->profile();
        CHECK_BROWSER(profile->persistentStoragePath() != sessionPath);
        sessionPath = profile->persistentStoragePath();
        browser.navigate(first.resolved(QUrl("/slow")));
        CHECK_BROWSER(waitUntil([&]() { return stop->isEnabled(); }));
        stop->click();
        CHECK_BROWSER(waitUntil([&]() { return !stop->isEnabled(); }));
        // A failed request must stop the spinner and leave navigation usable.
        server.close();
        browser.navigate(first.resolved(QUrl("/unavailable")));
        CHECK_BROWSER(waitUntil([&]() { return status->text().contains("did not finish"); }));
        CHECK_BROWSER(!stop->isEnabled());
        CHECK_BROWSER(lifecycleOrdered);
        QObject::disconnect(beforeChange);
        QObject::disconnect(afterChange);
    }
    CHECK_BROWSER(page.isNull() && profile.isNull());
    CHECK_BROWSER(waitUntil([&]() { return !QDir(sessionPath).exists(); }));
#if defined(Q_OS_LINUX)
    const QByteArray originalRenderer = qgetenv("QTWEBENGINEPROCESS_PATH");
    qputenv("QTWEBENGINEPROCESS_PATH", "/nonexistent/dockpipe-browser-test/QtWebEngineProcess");
    {
        BrowserWidget missingRuntime;
        CHECK_BROWSER(!missingRuntime.findChild<QWebEngineView *>());
        CHECK_BROWSER(missingRuntime.findChild<QLabel *>("browserStatus")->text().contains("missing"));
    }
    if (originalRenderer.isNull())
        qunsetenv("QTWEBENGINEPROCESS_PATH");
    else
        qputenv("QTWEBENGINEPROCESS_PATH", originalRenderer);
#endif
    std::puts("Browser navigation, JavaScript, history, cancellation, failure and cleanup passed.");
    return 0;
}

class BrowserTest : public QObject
{
    Q_OBJECT

private slots:
    void canvasViewportIsContextScoped()
    {
        BrowserWidget browser;
        browser.resize(820, 600);
        browser.show();
        auto *address = browser.findChild<QLineEdit *>("browserAddress");
        auto *toggle = browser.findChild<QPushButton *>("browserCanvasViewport");
        QVERIFY(address && toggle);
        QVERIFY(address->isVisible());
        QVERIFY(!toggle->isVisible());
        browser.setProperty("dashboardCanvasPresentation", true);
        QVERIFY(browser.canvasViewportMode());
        QVERIFY(!address->isVisible());
        QVERIFY(toggle->isVisible());
        toggle->click();
        QVERIFY(!browser.canvasViewportMode());
        QVERIFY(address->isVisible());
        toggle->click();
        QVERIFY(browser.canvasViewportMode());
        browser.setProperty("dashboardCanvasPresentation", false);
        QVERIFY(address->isVisible());
        QVERIFY(!toggle->isVisible());
        browser.setChromeVisible(false);
        browser.setCanvasPresentationEnabled(true);
        QVERIFY(!toggle->isVisible());
        browser.setChromeVisible(true);
        QVERIFY(toggle->isVisible());
    }

    void tearOutRetainsLivePageAndSessionAfterSourceCloses()
    {
        QTcpServer server;
        QVERIFY(startBrowserServer(server));
        auto source = std::make_unique<BrowserWidget>();
        auto destination = std::make_unique<BrowserWidget>();
        source->setCanvasPresentationEnabled(true);
        destination->setCanvasPresentationEnabled(true);
        source->show();
        destination->show();
        auto *view = source->activeView();
        QPointer<QWebEnginePage> page(view->page());
        QPointer<QWebEngineProfile> profile(page->profile());
        source->navigate(QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort())));
        QVERIFY(waitUntil([&]() { return view->title() == "Browser test page"; }));
        bool seeded = false;
        page->runJavaScript("window.tearOutState = 'retained'; sessionStorage.setItem('tearOut', 'yes'); true",
            [&](const QVariant &) { seeded = true; });
        QVERIFY(waitUntil([&]() { return seeded; }));
        QVERIFY(source->moveTabTo(view, destination.get()));
        QCOMPARE(destination->activeView(), view);
        QCOMPARE(view->page(), page.data());
        QCOMPARE(view->page()->profile(), profile.data());
        QCOMPARE(destination->findChild<QTabBar *>("browserTabs")->count(), 1);
        QVERIFY(source->activeView() != view);
        source.reset();
        QVERIFY(page && profile);
        bool retained = false;
        page->runJavaScript("window.tearOutState === 'retained' && sessionStorage.getItem('tearOut') === 'yes'",
            [&](const QVariant &value) { retained = value.toBool(); });
        QVERIFY(waitUntil([&]() { return retained; }));
        QVERIFY(!destination->moveTabTo(view, destination.get()));
        BrowserWidget ordinary;
        QVERIFY(!destination->moveTabTo(view, &ordinary));
        destination->setCanvasViewportMode(false);
        destination->findChild<QPushButton *>("browserDownloads")->click();
        QVERIFY(destination->findChild<QTreeWidget *>("browserDownloadsList"));
        destination.reset();
        QVERIFY(page.isNull());
        QVERIFY(profile.isNull());
    }

    void tabTearOutGestureIsCanvasOnly()
    {
        BrowserTabBar tabs;
        tabs.resize(360, 40);
        tabs.addTab("One");
        tabs.show();
        QSignalSpy requests(&tabs, &BrowserTabBar::tearOutRequested);
        const QPoint start = tabs.tabRect(0).center();
        const QPoint outside(start.x(), 100);
        QTest::mousePress(&tabs, Qt::LeftButton, Qt::NoModifier, start);
        QTest::mouseRelease(&tabs, Qt::LeftButton, Qt::NoModifier, outside);
        QCOMPARE(requests.count(), 0);
        tabs.setTearOutEnabled(true);
        QTest::mousePress(&tabs, Qt::LeftButton, Qt::NoModifier, start);
        QTest::mouseMove(&tabs, outside);
        QTest::keyClick(&tabs, Qt::Key_Escape);
        QTest::mouseRelease(&tabs, Qt::LeftButton, Qt::NoModifier, outside);
        QCOMPARE(requests.count(), 0);
        QTest::mousePress(&tabs, Qt::LeftButton, Qt::NoModifier, start);
        tabs.hide();
        tabs.show();
        QTest::mouseRelease(&tabs, Qt::LeftButton, Qt::NoModifier, outside);
        QCOMPARE(requests.count(), 0);
        QTest::mousePress(&tabs, Qt::LeftButton, Qt::NoModifier, start);
        QTest::mouseMove(&tabs, outside);
        QTest::mouseRelease(&tabs, Qt::LeftButton, Qt::NoModifier, outside);
        QCOMPARE(requests.count(), 1);
        QCOMPARE(requests.first().at(0).toInt(), 0);
    }

    void navigationAndLifecycle()
    {
        QCOMPARE(runBrowserChecks(), 0);
    }

    void printDialogToPdf()
    {
#if defined(Q_OS_MACOS) || defined(Q_OS_WIN)
        // QPrintDialog uses an OS modal panel on these platforms, even with
        // AA_DontUseNativeDialogs. Qt documents that done() cannot close it:
        // https://doc.qt.io/qt-6/qprintdialog.html#done
        // The widget timer below cannot automate that native boundary.
        QSKIP("Native OS print panels require separate interactive verification; "
              "QDialog::done cannot accept them.");
#else
        QTcpServer server;
        QVERIFY(startBrowserServer(server));
        BrowserWidget browser;
        browser.resize(1100, 720);
        browser.show();
        auto *view = browser.findChild<QWebEngineView *>("browserWebView");
        auto *status = browser.findChild<QLabel *>("browserStatus");
        auto *menu = browser.findChild<QMenu *>("browserMenu");
        QVERIFY(view && status && menu);
        bool loaded = false;
        connect(view, &QWebEngineView::loadFinished, &browser, [&](bool success) {
            loaded = success;
        });
        browser.navigate(QUrl(QString("http://127.0.0.1:%1/").arg(server.serverPort())));
        QVERIFY(waitUntil([&]() { return loaded; }));

        QAction *print = nullptr;
        for (auto *action : menu->findChildren<QAction *>()) {
            if (action->text() == "Print…") {
                print = action;
                break;
            }
        }
        QVERIFY(print);
        QTemporaryDir downloads;
        QVERIFY(downloads.isValid());
        const QString pdfPath = downloads.filePath("page.pdf");
        QTimer acceptPrint;
        acceptPrint.setInterval(20);
        connect(&acceptPrint, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QPrintDialog *>()) {
                dialog->printer()->setOutputFormat(QPrinter::PdfFormat);
                dialog->printer()->setOutputFileName(pdfPath);
                dialog->QDialog::done(QDialog::Accepted);
            }
        });
        acceptPrint.start();
        print->trigger();
        acceptPrint.stop();
        QVERIFY(waitUntil([&]() { return status->text().contains("sent to printer"); }));
        QFile pdf(pdfPath);
        QVERIFY(pdf.open(QIODevice::ReadOnly));
        QCOMPARE(pdf.read(5), QByteArray("%PDF-"));
#endif
    }
};

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QCoreApplication::setAttribute(Qt::AA_DontUseNativeDialogs);
    QTemporaryDir testStorage;
    qputenv("XDG_DATA_HOME", testStorage.path().toUtf8());
    qputenv("XDG_CACHE_HOME", testStorage.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", testStorage.path().toUtf8());
    QApplication app(argc, argv);
    app.setStyle("Fusion");
    applyDockpipeLauncherTheme(app);
    BrowserTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "BrowserTest.moc"
