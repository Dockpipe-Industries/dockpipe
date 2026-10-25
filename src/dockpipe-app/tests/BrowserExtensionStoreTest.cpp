#include "BrowserExtensionStore.h"

#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLockFile>
#include <QTemporaryDir>
#include <QThread>
#include <QWebEngineProfile>
#include <functional>
#include <memory>
#include <cstdio>

#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#define DOCKPIPE_EXTENSION_STORE_TEST_AVAILABLE
#include <QWebEngineExtensionManager>
#include <QWebEngineProfileBuilder>
#endif
#endif

static bool writeFile(const QString &path, const QByteArray &contents)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(contents) == contents.size();
}

#ifdef DOCKPIPE_EXTENSION_STORE_TEST_AVAILABLE
static QWebEngineExtensionInfo fixture(QWebEngineExtensionManager *manager)
{
    for (const auto &extension : manager->extensions()) {
        if (extension.name() == "Durable fixture")
            return extension;
    }
    return QWebEngineExtensionInfo();
}
#endif

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (elapsed.elapsed() < 15000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

#define CHECK_STORE(expression) \
    do { \
        if (!(expression)) { \
            std::fprintf(stderr, "Store check failed at line %d: %s\n", __LINE__, #expression); \
            return 1; \
        } \
    } while (false)

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    QTemporaryDir directory;
    CHECK_STORE(directory.isValid());
    const QString root = directory.filePath("managed");
    QString error;
    {
        QWebEngineProfile privateProfile;
        BrowserExtensionStore store(&privateProfile, nullptr, root);
        CHECK_STORE(BrowserExtensionStore::forProfile(&privateProfile) == &store);
        CHECK_STORE(!store.owns("../outside"));
        // A syntactically valid unknown ID must not request private extension services.
        CHECK_STORE(!store.owns(QString(32, QLatin1Char('a'))));
        CHECK_STORE(!store.enabled(QString(32, QLatin1Char('a'))));
        CHECK_STORE(!store.setEnabled("../outside", true, &error));
        CHECK_STORE(!store.remove("../outside", &error));
    }
#ifdef DOCKPIPE_EXTENSION_STORE_TEST_AVAILABLE
    const QString source = directory.filePath("source");
    CHECK_STORE(QDir().mkpath(source));
    CHECK_STORE(writeFile(source + "/manifest.json",
        R"({"manifest_version":3,"name":"Durable fixture","version":"1.0"})"));
    CHECK_STORE(writeFile(source + "/fixture.txt", "copied content"));
    QString stableId;
    auto createProfile = [&](const QString &name) {
        QWebEngineProfileBuilder builder;
        builder.setPersistentStoragePath(directory.filePath(name));
        builder.setCachePath(directory.filePath(name + "-cache"));
        builder.setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
        return std::unique_ptr<QWebEngineProfile>(builder.createProfile(name));
    };
    {
        auto profile = createProfile("first");
        CHECK_STORE(profile);
        BrowserExtensionStore store(profile.get(), nullptr, root);
        QApplication::processEvents();
        auto *manager = profile->extensionManager();
        manager->loadExtension(source);
        CHECK_STORE(waitUntil([&]() { return fixture(manager).isLoaded(); }));
        const auto extension = fixture(manager);
        CHECK_STORE(extension.isLoaded());
        stableId = extension.id();
        // Validation runs on the copied content, rather than trusting a previous load.
        CHECK_STORE(writeFile(source + "/manifest.json", R"({"manifest_version":2})"));
        CHECK_STORE(!store.persist(extension, &error));
        CHECK_STORE(writeFile(source + "/manifest.json",
            R"({"manifest_version":3,"name":"Durable fixture","version":"1.0"})"));
        {
            QFile oversized(source + "/too-large.bin");
            CHECK_STORE(oversized.open(QIODevice::WriteOnly));
            CHECK_STORE(oversized.resize(64 * 1024 * 1024 + 1));
        }
        CHECK_STORE(!store.persist(extension, &error));
        CHECK_STORE(QFile::remove(source + "/too-large.bin"));
#ifndef Q_OS_WIN
        CHECK_STORE(QFile::link(source + "/fixture.txt", source + "/linked.txt"));
        CHECK_STORE(!store.persist(extension, &error));
        CHECK_STORE(QFile::remove(source + "/linked.txt"));
#endif
        CHECK_STORE(QDir(root).entryList({".install-*"}, QDir::Dirs | QDir::Hidden).isEmpty());
        const bool persisted = store.persist(extension, &error);
        if (!persisted)
            std::fprintf(stderr, "Persist failed: %s; path=%s id=%s\n", qPrintable(error), qPrintable(extension.path()), qPrintable(extension.id()));
        CHECK_STORE(persisted);
        CHECK_STORE(!store.enabled(stableId));
        CHECK_STORE(QFileInfo::exists(root + "/" + stableId + "/fixture.txt"));
        CHECK_STORE(store.setEnabled(stableId, true, &error));
        CHECK_STORE(store.activate(stableId, &error));
        CHECK_STORE(waitUntil([&]() {
            for (const auto &loaded : manager->extensions()) {
                if (QFileInfo(loaded.path()).canonicalFilePath()
                    == QFileInfo(root + "/" + stableId).canonicalFilePath())
                    return loaded.isEnabled();
            }
            return false;
        }));
    }
    CHECK_STORE(QDir(source).removeRecursively());
    {
        auto profile = createProfile("second");
        CHECK_STORE(profile);
        BrowserExtensionStore store(profile.get(), nullptr, root);
        auto *manager = profile->extensionManager();
        CHECK_STORE(waitUntil([&]() {
            return fixture(manager).isLoaded() && fixture(manager).isEnabled();
        }));
        const auto extension = fixture(manager);
        CHECK_STORE(extension.isLoaded() && store.owns(extension.id()));
        CHECK_STORE(store.enabled(extension.id()));
        CHECK_STORE(store.setEnabled(extension.id(), false, &error));
    }
    {
        auto profile = createProfile("third");
        CHECK_STORE(profile);
        BrowserExtensionStore store(profile.get(), nullptr, root);
        auto *manager = profile->extensionManager();
        CHECK_STORE(waitUntil([&]() { return fixture(manager).isLoaded(); }));
        const auto extension = fixture(manager);
        CHECK_STORE(!extension.isEnabled());
        CHECK_STORE(store.remove(extension.id(), &error));
        CHECK_STORE(!store.owns(extension.id()));
        CHECK_STORE(!QFileInfo::exists(root + "/" + stableId));
    }
    // Independently created browser profiles merge writes into one locked catalog.
    {
        const QString sharedRoot = directory.filePath("shared-root");
        const QString firstSource = directory.filePath("first-source");
        const QString secondSource = directory.filePath("second-source");
        CHECK_STORE(QDir().mkpath(firstSource) && QDir().mkpath(secondSource));
        const QByteArray manifest = R"({"manifest_version":3,"name":"Durable fixture","version":"1.0"})";
        CHECK_STORE(writeFile(firstSource + "/manifest.json", manifest));
        CHECK_STORE(writeFile(secondSource + "/manifest.json", manifest));
        auto firstProfile = createProfile("shared-first");
        auto secondProfile = createProfile("shared-second");
        CHECK_STORE(firstProfile && secondProfile);
        BrowserExtensionStore firstStore(firstProfile.get(), nullptr, sharedRoot);
        BrowserExtensionStore secondStore(secondProfile.get(), nullptr, sharedRoot);
        QApplication::processEvents();
        firstProfile->extensionManager()->loadExtension(firstSource);
        secondProfile->extensionManager()->loadExtension(secondSource);
        CHECK_STORE(waitUntil([&]() {
            return fixture(firstProfile->extensionManager()).isLoaded()
                && fixture(secondProfile->extensionManager()).isLoaded();
        }));
        const auto first = fixture(firstProfile->extensionManager());
        const auto second = fixture(secondProfile->extensionManager());
        CHECK_STORE(first.id() != second.id());
        CHECK_STORE(firstStore.persist(first, &error));
        CHECK_STORE(secondStore.persist(second, &error));
        CHECK_STORE(firstStore.setEnabled(first.id(), true, &error));
        CHECK_STORE(secondStore.setEnabled(second.id(), true, &error));
        CHECK_STORE(firstStore.remove(first.id(), &error));
        {
            QLockFile competingOperation(sharedRoot + "/registry.lock");
            CHECK_STORE(competingOperation.tryLock(0));
            CHECK_STORE(!secondStore.setEnabled(second.id(), false, &error));
            CHECK_STORE(error.contains("busy"));
        }
        CHECK_STORE(secondStore.setEnabled(second.id(), false, &error));
        QFile registry(sharedRoot + "/registry.json");
        CHECK_STORE(registry.open(QIODevice::ReadOnly));
        const QJsonObject entries = QJsonDocument::fromJson(registry.readAll()).object()
            .value("extensions").toObject();
        CHECK_STORE(entries.size() == 1 && entries.contains(second.id()));
        CHECK_STORE(!entries.value(second.id()).toObject().value("enabled").toBool());
        CHECK_STORE(!firstStore.setEnabled(first.id(), true, &error));
    }
#ifndef Q_OS_WIN
    // Managed storage itself and its registry cannot redirect through links.
    {
        const QString linkTarget = directory.filePath("link-target");
        const QString linkedRoot = directory.filePath("linked-root");
        CHECK_STORE(QDir().mkpath(linkTarget));
        CHECK_STORE(QFile::link(linkTarget, linkedRoot));
        auto profile = createProfile("linked-root-profile");
        BrowserExtensionStore store(profile.get(), nullptr, linkedRoot);
        CHECK_STORE(!store.lastError().isEmpty());
        CHECK_STORE(!store.setEnabled(stableId, true, &error));
    }
    {
        const QString linkedRegistryRoot = directory.filePath("linked-registry");
        const QString target = directory.filePath("registry-target.json");
        CHECK_STORE(QDir().mkpath(linkedRegistryRoot));
        CHECK_STORE(writeFile(target, R"({"version":1,"extensions":{}})"));
        CHECK_STORE(QFile::link(target, linkedRegistryRoot + "/registry.json"));
        auto profile = createProfile("linked-registry-profile");
        BrowserExtensionStore store(profile.get(), nullptr, linkedRegistryRoot);
        CHECK_STORE(!store.lastError().isEmpty());
        CHECK_STORE(!store.remove(stableId, &error));
    }
#endif
    // Malformed registries cannot silently replace previous user choices.
    CHECK_STORE(writeFile(root + "/registry.json", "not json"));
    {
        auto profile = createProfile("invalid-registry");
        CHECK_STORE(profile);
        BrowserExtensionStore store(profile.get(), nullptr, root);
        CHECK_STORE(!store.lastError().isEmpty());
    }
    // A store destroyed before its deferred startup callback must not run restore.
    {
        auto profile = createProfile("destroyed-store");
        auto *store = new BrowserExtensionStore(profile.get(), nullptr, directory.filePath("destroyed"));
        delete store;
        QApplication::processEvents();
        CHECK_STORE(!fixture(profile->extensionManager()).isLoaded());
    }
    std::puts("Extension copy, activation, restart, shared registry locking, symlink rejection and lifecycle checks passed.");
#else
    std::puts("Extension store unsupported-Qt guard passed.");
#endif
    return 0;
}
