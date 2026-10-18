#include "BrowserExtensionsDialog.h"
#include "BrowserRuntime.h"
#include <QWebEngineView>
#include <memory>

#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QListWidget>
#include <QPushButton>
#include <QTemporaryDir>
#include <QThread>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <cstdio>
#include <functional>

#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#define DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
#include <QWebEngineExtensionManager>
#include <QWebEngineProfileBuilder>
#endif
#endif

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

static bool writeFile(const QString &path, const QByteArray &contents)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(contents) == contents.size();
}

#define CHECK_EXTENSION(expression) \
    do { \
        if (!(expression)) { \
            std::fprintf(stderr, "Extension check failed at line %d: %s\n", __LINE__, #expression); \
            return 1; \
        } \
    } while (false)

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication app(argc, argv);
    QWebEngineProfile privateProfile;
    BrowserExtensionsDialog privateDialog(&privateProfile);
    CHECK_EXTENSION(!privateDialog.findChild<QPushButton *>("browserExtensionLoad")->isEnabled());

#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    CHECK_EXTENSION(privateDialog.findChild<QLabel *>("browserExtensionsStatus")->text().contains("private"));
    QTemporaryDir directory;
    CHECK_EXTENSION(directory.isValid());
    const QString extensionDirectory = directory.filePath("extension");
    CHECK_EXTENSION(QDir().mkpath(extensionDirectory));
    CHECK_EXTENSION(writeFile(extensionDirectory + "/manifest.json", R"({
        "manifest_version": 3,
        "name": "DockPipe extension fixture",
        "version": "1.0",
        "action": {"default_popup": "popup.html"}
    })"));
    CHECK_EXTENSION(writeFile(extensionDirectory + "/popup.html",
        "<!doctype html><title>Extension popup ready</title><p>Session extension works.</p>"));
    // Set storage before constructing Chromium's profile services. Changing it
    // afterward replaces the preference service retained by extension services.
    QWebEngineProfileBuilder profileBuilder;
    profileBuilder.setPersistentStoragePath(directory.filePath("profile"));
    profileBuilder.setCachePath(directory.filePath("cache"));
    profileBuilder.setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
    std::unique_ptr<QWebEngineProfile> profileOwner(profileBuilder.createProfile("dockpipe-extension-test"));
    CHECK_EXTENSION(profileOwner);
    QWebEngineProfile &profile = *profileOwner;
    BrowserExtensionsDialog dialog(&profile);
    auto *list = dialog.findChild<QListWidget *>("browserExtensionsList");
    auto *toggle = dialog.findChild<QPushButton *>("browserExtensionToggle");
    auto *open = dialog.findChild<QPushButton *>("browserExtensionOpen");
    auto *unload = dialog.findChild<QPushButton *>("browserExtensionUnload");
    auto *status = dialog.findChild<QLabel *>("browserExtensionsStatus");
    CHECK_EXTENSION(list && toggle && open && unload && status);
    auto *manager = profile.extensionManager();
    CHECK_EXTENSION(manager);
    manager->loadExtension(extensionDirectory);
    QString fixtureId;
    CHECK_EXTENSION(waitUntil([&]() {
        for (int row = 0; row < list->count(); ++row) {
            if (list->item(row)->text().contains("DockPipe extension fixture")) {
                list->setCurrentRow(row);
                fixtureId = list->item(row)->data(Qt::UserRole).toString();
                return true;
            }
        }
        return false;
    }));
    CHECK_EXTENSION(toggle->text() == "Enable" && !open->isEnabled());
    toggle->click();
    CHECK_EXTENSION(toggle->text() == "Disable" && open->isEnabled());
    QUrl popupUrl;
    QObject::connect(&dialog, &BrowserExtensionsDialog::openUrl, &dialog,
        [&](const QUrl &url) { popupUrl = url; });
    open->click();
    CHECK_EXTENSION(popupUrl.scheme() == "chrome-extension" && popupUrl.host() == fixtureId);
    {
        CHECK_EXTENSION(BrowserRuntime::allowsNavigation(popupUrl, &profile));
        CHECK_EXTENSION(!BrowserRuntime::allowsNavigation(QUrl("chrome-extension://unknown/popup.html"), &profile));
        std::unique_ptr<QWebEngineView> popup(BrowserRuntime::createView(&profile));
        popup->load(popupUrl);
        CHECK_EXTENSION(waitUntil([&]() { return popup->title() == "Extension popup ready"; }));
    }
    toggle->click();
    CHECK_EXTENSION(toggle->text() == "Enable" && !open->isEnabled());
    CHECK_EXTENSION(!BrowserRuntime::allowsNavigation(popupUrl, &profile));
    unload->click();
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.id() == fixtureId)
                return false;
        }
        return true;
    }));
    manager->loadExtension(directory.filePath("missing-extension"));
    CHECK_EXTENSION(waitUntil([&]() { return status->text().contains("Could not load"); }));
    std::puts("Extension load, enable, popup rendering, disable, unload and failure handling passed.");
#else
    CHECK_EXTENSION(privateDialog.findChild<QLabel *>("browserExtensionsStatus")->text().contains("6.10"));
    std::puts("Older Qt extension availability explanation passed.");
#endif
    return 0;
}
