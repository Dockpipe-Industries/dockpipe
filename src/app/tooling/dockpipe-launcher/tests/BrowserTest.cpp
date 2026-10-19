#include "BrowserRuntime.h"
#include "BrowserWidget.h"
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
    CHECK_BROWSER(BrowserRuntime::allowsUrl(QUrl("https://example.com")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("file:///tmp/private")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("https://user:password@example.com")));
    CHECK_BROWSER(!BrowserRuntime::allowsUrl(QUrl("javascript:alert(1)")));
    CHECK_BROWSER(!BrowserWidget::addressUrl(" ").isValid());
    CHECK_BROWSER(BrowserWidget::addressUrl("example.com").host() == "example.com");
    CHECK_BROWSER(BrowserWidget::addressUrl("weather tomorrow").host() == "duckduckgo.com");

    QTcpServer server;
    CHECK_BROWSER(server.listen(QHostAddress::LocalHost));
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
        QPointer<QWebEngineDownloadRequest> cancelledDownload;
        bool downloadRequested = false;
        QObject::connect(profile, &QWebEngineProfile::downloadRequested, &browser,
            [&](QWebEngineDownloadRequest *download) {
                downloadRequested = true;
                cancelledDownload = download;
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
        QTemporaryDir downloads;
        CHECK_BROWSER(downloads.isValid());
        const QString savedFile = downloads.filePath("saved.html");
        downloadRequested = false;
        QTimer acceptSaveDialog;
        acceptSaveDialog.setInterval(20);
        QObject::connect(&acceptSaveDialog, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QFileDialog *>()) {
                dialog->selectFile(savedFile);
                QMetaObject::invokeMethod(dialog, "accept", Qt::DirectConnection);
            }
        });
        acceptSaveDialog.start();
        view->page()->download(first, "browser-test.html");
        CHECK_BROWSER(waitUntil([&]() { return downloadRequested && cancelledDownload.isNull(); }));
        acceptSaveDialog.stop();
        QFile downloaded(savedFile);
        CHECK_BROWSER(downloaded.open(QIODevice::ReadOnly));
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
        const QString pdfPath = downloads.filePath("page.pdf");
        QTimer acceptPrint;
        acceptPrint.setInterval(20);
        QObject::connect(&acceptPrint, &QTimer::timeout, &browser, [&]() {
            if (auto *dialog = browser.findChild<QPrintDialog *>()) {
                dialog->printer()->setOutputFormat(QPrinter::PdfFormat);
                dialog->printer()->setOutputFileName(pdfPath);
                dialog->QDialog::done(QDialog::Accepted);
            }
        });
        auto *print = actionNamed("Print…");
        CHECK_BROWSER(print);
        acceptPrint.start();
        print->trigger();
        acceptPrint.stop();
        CHECK_BROWSER(waitUntil([&]() { return status->text().contains("sent to printer"); }));
        QFile pdf(pdfPath);
        CHECK_BROWSER(pdf.open(QIODevice::ReadOnly) && pdf.read(5) == "%PDF-");
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
