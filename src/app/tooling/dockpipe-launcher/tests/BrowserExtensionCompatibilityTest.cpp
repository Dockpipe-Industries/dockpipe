#include "BrowserExtensionPackage.h"
#include "BrowserExtensionStore.h"
#include "BrowserExtensionsDialog.h"
#include "BrowserRuntime.h"

#include <QApplication>
#include <QElapsedTimer>
#include <QJsonDocument>
#include <QJsonArray>
#include <QJsonObject>
#include <QMessageBox>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <QVariant>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineScript>
#include <QWebEngineScriptCollection>
#include <QWebEngineView>
#include <cstdio>
#include <functional>
#include <memory>

#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#define DOCKPIPE_EXTENSION_COMPATIBILITY_AVAILABLE
#include <QWebEngineExtensionManager>
#include <QWebEngineProfileBuilder>
#endif
#endif

namespace {
bool waitUntil(const std::function<bool()> &predicate, int milliseconds)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

void report(const QJsonObject &object)
{
    const QByteArray json = QJsonDocument(object).toJson(QJsonDocument::Compact);
    std::fprintf(stdout, "%s\n", json.constData());
    std::fflush(stdout);
}
}

int main(int argc, char **argv)
{
    const QString requestedId = qEnvironmentVariable("DOCKPIPE_EXTENSION_COMPAT_ID");
    if (requestedId.isEmpty()) {
        std::puts("Set DOCKPIPE_EXTENSION_COMPAT_ID to explicitly run the live extension compatibility check.");
        return 77;
    }
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
#ifdef DOCKPIPE_EXTENSION_COMPATIBILITY_AVAILABLE
    const QString id = BrowserExtensionPackage::extensionId(requestedId);
    if (id.isEmpty()) {
        report({{"stage", "input"}, {"error", "Invalid extension ID."}});
        return 1;
    }
    BrowserExtensionPackage package;
    QString zip;
    QString error;
    QObject::connect(&package, &BrowserExtensionPackage::ready, &application,
        [&](const QString &path) { zip = path; });
    QObject::connect(&package, &BrowserExtensionPackage::failed, &application,
        [&](const QString &message) { error = message; });
    package.fetch(id);
    if (!waitUntil([&]() { return !zip.isEmpty() || !error.isEmpty(); }, 60000) || zip.isEmpty()) {
        package.cancel();
        report({{"stage", "download"}, {"error", error.isEmpty() ? "Timed out." : error.left(4096)}});
        return 1;
    }

    QTemporaryDir directory;
    if (!directory.isValid())
        return 1;
    QWebEngineProfileBuilder builder;
    builder.setPersistentStoragePath(directory.filePath("profile"));
    builder.setCachePath(directory.filePath("cache"));
    builder.setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
    std::unique_ptr<QWebEngineProfile> profile(builder.createProfile("dockpipe-extension-compatibility"));
    if (!profile)
        return 1;
    BrowserExtensionStore store(profile.get(), profile.get(), directory.filePath("extensions"));
    BrowserExtensionsDialog dialog(profile.get());
    bool reviewed = false;
    QTimer confirmation;
    QObject::connect(&confirmation, &QTimer::timeout, &dialog, [&]() {
        auto *box = dialog.findChild<QMessageBox *>("browserExtensionPermissions");
        if (box) {
            reviewed = true;
            box->done(QMessageBox::Yes);
        }
    });
    confirmation.start(10);
    dialog.installPackage(zip, package.publicKey());
    QUrl popupUrl;
    QString installedName;
    const bool installed = waitUntil([&]() {
        for (const auto &extension : profile->extensionManager()->extensions()) {
            if (extension.id() == id && extension.isEnabled() && store.owns(id)) {
                popupUrl = extension.actionPopupUrl();
                installedName = extension.name();
                return true;
            }
        }
        return false;
    }, 15000);
    confirmation.stop();
    if (!installed || !reviewed) {
        report({{"stage", "installation"}, {"id", id}, {"permissions_reviewed", reviewed},
            {"error", "The approved extension did not become enabled with the expected identity."}});
        return 1;
    }
    report({{"stage", "installed"}, {"id", id}, {"name", installedName},
        {"popup", popupUrl.toString()}, {"permissions_reviewed", reviewed}});
    if (!BrowserRuntime::allowsNavigation(popupUrl, profile.get())) {
        report({{"stage", "popup"}, {"error", "No supported extension action popup."}});
        return 1;
    }

    std::unique_ptr<QWebEngineView> view(BrowserRuntime::createView(profile.get()));
    view->resize(900, 720);
    view->show();
    QWebEngineScript diagnostics;
    diagnostics.setName("Dockpipe compatibility error capture");
    diagnostics.setInjectionPoint(QWebEngineScript::DocumentCreation);
    diagnostics.setWorldId(QWebEngineScript::MainWorld);
    diagnostics.setSourceCode(QStringLiteral(R"JS(
        (() => {
            window.__dockpipeCompatibilityErrors = [];
            const record = message => {
                if (window.__dockpipeCompatibilityErrors.length < 32)
                    window.__dockpipeCompatibilityErrors.push(String(message).slice(0, 2048));
            };
            window.addEventListener('error', event => record(event.message));
            window.addEventListener('unhandledrejection', event => record(event.reason));
            const originalError = console.error;
            console.error = (...messages) => {
                record(messages.map(String).join(' '));
                originalError.apply(console, messages);
            };
        })();
    )JS"));
    view->page()->scripts().insert(diagnostics);
    bool loadFinished = false;
    bool loaded = false;
    QObject::connect(view.get(), &QWebEngineView::loadFinished, &application, [&](bool success) {
        loaded = success;
        loadFinished = true;
    });
    view->load(popupUrl);
    if (!waitUntil([&]() { return loadFinished; }, 15000) || !loaded) {
        report({{"stage", "popup"}, {"error", "The extension popup did not load."}});
        return 1;
    }
    QElapsedTimer settling;
    settling.start();
    struct PopupInspection {
        bool reading = false;
        bool inspected = false;
        QJsonObject contents;
    };
    const auto inspection = std::make_shared<PopupInspection>();
    waitUntil([&]() {
        if (inspection->inspected && !inspection->reading && settling.elapsed() >= 5000)
            return true;
        if (!inspection->reading) {
            inspection->reading = true;
            view->page()->runJavaScript(QStringLiteral(R"JS(
                JSON.stringify({
                    text: (document.body ? document.body.innerText : '').slice(0, 12000),
                    title: document.title.slice(0, 1000),
                    errors: window.__dockpipeCompatibilityErrors || []
                })
            )JS"), [inspection](const QVariant &result) {
                const QJsonDocument document = QJsonDocument::fromJson(result.toString().toUtf8());
                if (document.isObject()) {
                    inspection->contents = document.object();
                    inspection->inspected = true;
                }
                inspection->reading = false;
            });
        }
        return inspection->inspected && !inspection->reading && settling.elapsed() >= 5000;
    }, 15000);
    QJsonObject contents = inspection->contents;
    contents.insert("stage", "popup-rendered");
    contents.insert("id", id);
    contents.insert("loaded", loaded);
    contents.insert("account_login_tested", false);
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_EXTENSION_COMPAT_SCREENSHOT");
    if (!screenshot.isEmpty())
        contents.insert("screenshot_saved", view->grab().save(screenshot));
    report(contents);
    const QString text = contents.value("text").toString();
    const bool hasRuntimeErrors = !contents.value("errors").toArray().isEmpty();
    const bool visibleFailure = text.contains("something went wrong", Qt::CaseInsensitive)
        || text.contains("could not start", Qt::CaseInsensitive)
        || text.contains("cannot read properties", Qt::CaseInsensitive)
        || text.contains("unsupported browser", Qt::CaseInsensitive)
        || text.contains("browser is not supported", Qt::CaseInsensitive);
    if (!inspection->inspected || text.trimmed().isEmpty() || hasRuntimeErrors || visibleFailure) {
        report({{"stage", "compatibility-failure"},
            {"error", "Installation succeeded, but popup content or runtime errors need inspection."}});
        return 1;
    }
    std::puts("Installation and visible popup passed. Account login, vault access and native desktop integration were not tested.");
    return 0;
#else
    std::puts("This compatibility check requires Qt WebEngine 6.10 extension support.");
    return 77;
#endif
}
