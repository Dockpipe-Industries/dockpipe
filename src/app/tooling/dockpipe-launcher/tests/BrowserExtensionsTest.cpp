#include "BrowserExtensionsDialog.h"
#include "BrowserRuntime.h"
#include "BrowserExtensionStore.h"
#include <QWebEngineView>
#include <memory>

#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QListWidget>
#include <QPushButton>
#include <QMessageBox>
#include <QTimer>
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
    BrowserExtensionStore store(&profile, &profile, directory.filePath("managed-extensions"));
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
    const QByteArray package = QByteArray::fromBase64(
        "UEsDBBQAAAAAAMqEU12TlRj6IAEAACABAAANAAAAbWFuaWZlc3QuanNvbnsibWFuaWZlc3RfdmVyc2lvbiI6IDMsICJuYW1l"
        "IjogIkRvY2tQaXBlIGluc3RhbGxlZCBmaXh0dXJlIiwgInZlcnNpb24iOiAiMS4wIiwgInBlcm1pc3Npb25zIjogWyJzdG9y"
        "YWdlIl0sICJob3N0X3Blcm1pc3Npb25zIjogWyJodHRwczovL2V4YW1wbGUuaW52YWxpZC8qIl0sICJjb250ZW50X3Njcmlw"
        "dHMiOiBbeyJtYXRjaGVzIjogWyJodHRwczovL2V4YW1wbGUuaW52YWxpZC8qIl0sICJqcyI6IFsiY29udGVudC5qcyJdfV0s"
        "ICJhY3Rpb24iOiB7ImRlZmF1bHRfcG9wdXAiOiAicG9wdXAuaHRtbCJ9fVBLAwQUAAAAAADKhFNdpMa5jA0AAAANAAAACgAA"
        "AGNvbnRlbnQuanMvKiBmaXh0dXJlICovUEsDBBQAAAAAAMqEU10qSgk/MwAAADMAAAAKAAAAcG9wdXAuaHRtbDwhZG9jdHlw"
        "ZSBodG1sPjx0aXRsZT5JbnN0YWxsZWQgcG9wdXAgcmVhZHk8L3RpdGxlPlBLAQIUAxQAAAAAAMqEU12TlRj6IAEAACABAAAN"
        "AAAAAAAAAAAAAACAAQAAAABtYW5pZmVzdC5qc29uUEsBAhQDFAAAAAAAyoRTXaTGuYwNAAAADQAAAAoAAAAAAAAAAAAAAIAB"
        "SwEAAGNvbnRlbnQuanNQSwECFAMUAAAAAADKhFNdKkoJPzMAAAAzAAAACgAAAAAAAAAAAAAAgAGAAQAAcG9wdXAuaHRtbFBL"
        "BQYAAAAAAwADAKsAAADbAQAAAAA=");
    const QString packagePath = directory.filePath("fixture.zip");
    CHECK_EXTENSION(writeFile(packagePath, package));
    dialog.addWebStoreExtension("https://example.invalid/not-a-store");
    CHECK_EXTENSION(status->text().contains("valid Chrome Web Store"));
    dialog.installPackage(directory.filePath("missing.zip"));
    CHECK_EXTENSION(status->text().contains("Could not open"));
    bool reviewedPermissions = false;
    QTimer reviewTimer;
    QObject::connect(&reviewTimer, &QTimer::timeout, &dialog, [&]() {
        auto *review = dialog.findChild<QMessageBox *>("browserExtensionPermissions");
        if (!review)
            return;
        reviewedPermissions = review->textFormat() == Qt::PlainText
            && review->defaultButton() == review->button(QMessageBox::Cancel)
            && review->text().contains("API: storage")
            && review->text().contains("Website: https://example.invalid/*")
            && review->text().contains("Page content: https://example.invalid/*");
        review->done(QMessageBox::Yes);
    });
    reviewTimer.start(10);
    dialog.installPackage(packagePath);
    CHECK_EXTENSION(waitUntil([&]() { return status->text().contains("Installed and enabled"); }));
    reviewTimer.stop();
    CHECK_EXTENSION(reviewedPermissions);
    QString installedId;
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.name() == "DockPipe installed fixture" && extension.isEnabled()
                && store.owns(extension.id())) {
                installedId = extension.id();
                return true;
            }
        }
        return false;
    }));
    CHECK_EXTENSION(!installedId.isEmpty() && store.owns(installedId) && store.enabled(installedId));
    for (int row = 0; row < list->count(); ++row) {
        if (list->item(row)->data(Qt::UserRole).toString() == installedId)
            list->setCurrentRow(row);
    }
    toggle->click();
    CHECK_EXTENSION(!store.enabled(installedId));
    unload->click();
    CHECK_EXTENSION(!store.owns(installedId));
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.id() == installedId)
                return false;
        }
        return true;
    }));
    bool cancelledReview = false;
    QObject::disconnect(&reviewTimer, nullptr, &dialog, nullptr);
    QObject::connect(&reviewTimer, &QTimer::timeout, &dialog, [&]() {
        auto *review = dialog.findChild<QMessageBox *>("browserExtensionPermissions");
        if (review) {
            cancelledReview = true;
            review->done(QMessageBox::Cancel);
        }
    });
    reviewTimer.start(10);
    dialog.installPackage(packagePath);
    CHECK_EXTENSION(waitUntil([&]() { return cancelledReview && status->text().contains("Installation cancelled"); }));
    reviewTimer.stop();
    CHECK_EXTENSION(!store.owns(installedId));
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.name() == "DockPipe installed fixture")
                return false;
        }
        return true;
    }));
    // Closing the owner during the permission dialog must also roll back installation.
    QPointer<BrowserExtensionsDialog> interruptedDialog = new BrowserExtensionsDialog(&profile);
    bool deletedDuringReview = false;
    QObject::disconnect(&reviewTimer, nullptr, &dialog, nullptr);
    QObject::connect(&reviewTimer, &QTimer::timeout, &dialog, [&]() {
        if (interruptedDialog
            && interruptedDialog->findChild<QMessageBox *>("browserExtensionPermissions")) {
            delete interruptedDialog.data();
            deletedDuringReview = true;
        }
    });
    reviewTimer.start(10);
    interruptedDialog->installPackage(packagePath);
    CHECK_EXTENSION(waitUntil([&]() { return deletedDuringReview && !interruptedDialog; }));
    reviewTimer.stop();
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.name() == "DockPipe installed fixture")
                return false;
        }
        return true;
    }));
    // A blocked registry must not leave an enabled, unsaved extension behind.
    const QString registry = directory.filePath("managed-extensions/registry.json");
    CHECK_EXTENSION(QFile::remove(registry));
    CHECK_EXTENSION(QDir().mkpath(registry));
    QObject::disconnect(&reviewTimer, nullptr, &dialog, nullptr);
    QObject::connect(&reviewTimer, &QTimer::timeout, &dialog, [&]() {
        auto *review = dialog.findChild<QMessageBox *>("browserExtensionPermissions");
        if (review)
            review->done(QMessageBox::Yes);
    });
    reviewTimer.start(10);
    dialog.installPackage(packagePath);
    CHECK_EXTENSION(waitUntil([&]() { return status->text().contains("Could not save extension"); }));
    reviewTimer.stop();
    CHECK_EXTENSION(waitUntil([&]() {
        for (const auto &extension : manager->extensions()) {
            if (extension.name() == "DockPipe installed fixture")
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
