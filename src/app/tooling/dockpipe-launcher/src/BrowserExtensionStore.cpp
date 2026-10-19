#include "BrowserExtensionStore.h"

#include <QDir>
#include <QDirIterator>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QLockFile>
#include <QRegularExpression>
#include <QSaveFile>
#include <QStandardPaths>
#include <QTimer>
#include <QUuid>
#include <QWebEngineProfile>

#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#define DOCKPIPE_EXTENSION_STORE_AVAILABLE
#include <QWebEngineExtensionInfo>
#include <QWebEngineExtensionManager>
#endif
#endif

namespace {
// Installation is interactive; bound copied code to 64 MiB and 4096 filesystem entries.
constexpr qint64 maximumBytes = 64 * 1024 * 1024;
constexpr int maximumEntries = 4096;
constexpr qint64 maximumRegistryBytes = 1024 * 1024;

bool fail(QString *error, const QString &message)
{
    if (error)
        *error = message;
    return false;
}

bool validId(const QString &id)
{
    static const QRegularExpression pattern(QStringLiteral("^[a-p]{32}$"));
    return pattern.match(id).hasMatch();
}

bool validateManifest(const QString &root, QString *error)
{
    QFile manifest(QDir(root).filePath("manifest.json"));
    if (!manifest.open(QIODevice::ReadOnly) || manifest.size() > maximumRegistryBytes)
        return fail(error, QObject::tr("The extension manifest is missing or exceeds 1 MiB."));
    const QByteArray contents = manifest.read(maximumRegistryBytes + 1);
    if (contents.size() > maximumRegistryBytes)
        return fail(error, QObject::tr("The extension manifest exceeds 1 MiB."));
    const QJsonDocument document = QJsonDocument::fromJson(contents);
    if (!document.isObject() || document.object().value("manifest_version").toInt() != 3)
        return fail(error, QObject::tr("Only extensions with a valid Manifest V3 manifest can be saved."));
    return true;
}

bool copyExtension(const QString &source, const QString &destination, QString *error)
{
    if (QFileInfo(source).isSymLink() || !QFileInfo(source).isDir())
        return fail(error, QObject::tr("The extension source must be a real directory."));
    if (!validateManifest(source, error))
        return false;
    if (!QDir().mkpath(destination))
        return fail(error, QObject::tr("Could not create the managed extension directory."));

    QDirIterator iterator(source, QDir::AllEntries | QDir::NoDotAndDotDot | QDir::Hidden | QDir::System,
        QDirIterator::Subdirectories);
    qint64 totalBytes = 0;
    int entries = 0;
    const QDir sourceDirectory(source);
    while (iterator.hasNext()) {
        iterator.next();
        const QFileInfo information = iterator.fileInfo();
        const QString relative = sourceDirectory.relativeFilePath(information.absoluteFilePath());
        if (++entries > maximumEntries || information.isSymLink() || relative == ".."
            || relative.startsWith("../") || QDir::isAbsolutePath(relative))
            return fail(error, QObject::tr("Extension content contains a link or exceeds 4096 entries."));
        const QString target = QDir(destination).filePath(relative);
        if (information.isDir()) {
            if (!QDir().mkpath(target))
                return fail(error, QObject::tr("Could not create an extension subdirectory."));
        } else if (information.isFile()) {
            const qint64 size = information.size();
            if (size < 0 || size > maximumBytes - totalBytes)
                return fail(error, QObject::tr("The extension exceeds the 64 MiB installation limit."));
            totalBytes += size;
            QFile input(information.absoluteFilePath());
            QSaveFile output(target);
            if (!input.open(QIODevice::ReadOnly) || !output.open(QIODevice::WriteOnly))
                return fail(error, QObject::tr("Could not copy an extension file."));
            qint64 copied = 0;
            while (!input.atEnd()) {
                const QByteArray chunk = input.read(65536);
                copied += chunk.size();
                if (chunk.isEmpty() || copied > size || output.write(chunk) != chunk.size())
                    return fail(error, QObject::tr("An extension file changed or could not be copied."));
            }
            if (copied != size || !output.commit())
                return fail(error, QObject::tr("Could not finish copying an extension file."));
        } else {
            return fail(error, QObject::tr("Extension content contains an unsupported filesystem entry."));
        }
    }
    return true;
}
}

BrowserExtensionStore::BrowserExtensionStore(QWebEngineProfile *profile, QObject *parent, const QString &root)
    : QObject(parent ? parent : profile), m_profile(profile),
      m_root(root.isEmpty() ? QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)
          + "/BrowserExtensions" : QDir(root).absolutePath())
{
    loadRegistry(&m_lastError);
    QTimer::singleShot(0, this, &BrowserExtensionStore::restore);
}

BrowserExtensionStore *BrowserExtensionStore::forProfile(QWebEngineProfile *profile)
{
    return profile ? profile->findChild<BrowserExtensionStore *>(QString(), Qt::FindDirectChildrenOnly) : nullptr;
}

QString BrowserExtensionStore::storedId(const QString &id) const
{
    if (m_entries.contains(id))
        return id;
#ifdef DOCKPIPE_EXTENSION_STORE_AVAILABLE
    if (m_profile) {
        for (const auto &extension : m_profile->extensionManager()->extensions()) {
            if (extension.id() != id)
                continue;
            const QString path = QFileInfo(extension.path()).canonicalFilePath();
            if (path.isEmpty())
                continue;
            for (auto entry = m_entries.begin(); entry != m_entries.end(); ++entry) {
                if (validId(entry.key()) && path == QFileInfo(QDir(m_root).filePath(entry.key())).canonicalFilePath())
                    return entry.key();
            }
        }
    }
#endif
    return QString();
}

bool BrowserExtensionStore::owns(const QString &id) const
{
    return !storedId(id).isEmpty();
}

bool BrowserExtensionStore::enabled(const QString &id) const
{
    return m_entries.value(storedId(id)).toObject().value("enabled").toBool();
}

bool BrowserExtensionStore::loadRegistry(QString *error)
{
    const QFileInfo rootInformation(m_root);
    const QString registryPath = QDir(m_root).filePath("registry.json");
    const QFileInfo registryInformation(registryPath);
    if (rootInformation.isSymLink() || (rootInformation.exists() && !rootInformation.isDir())
        || registryInformation.isSymLink()
        || (registryInformation.exists() && !registryInformation.isFile()))
        return fail(error, tr("Extension storage and its registry must not be symbolic links or special files."));

    QFile registry(registryPath);
    if (!registry.exists()) {
        m_entries = QJsonObject();
        return true;
    }
    if (!registry.open(QIODevice::ReadOnly) || registry.size() > maximumRegistryBytes)
        return fail(error, tr("Could not read the saved extension registry."));
    const QByteArray contents = registry.read(maximumRegistryBytes + 1);
    if (contents.size() > maximumRegistryBytes)
        return fail(error, tr("The saved extension registry exceeds 1 MiB."));
    const QJsonDocument document = QJsonDocument::fromJson(contents);
    const QJsonObject object = document.object();
    if (!document.isObject() || object.value("version").toInt() != 1
        || !object.value("extensions").isObject())
        return fail(error, tr("The saved extension registry is invalid."));
    const QJsonObject entries = object.value("extensions").toObject();
    for (auto entry = entries.begin(); entry != entries.end(); ++entry) {
        if (!validId(entry.key()) || !entry.value().isObject()
            || !entry.value().toObject().value("enabled").isBool()
            || QFileInfo(QDir(m_root).filePath(entry.key())).isSymLink())
            return fail(error, tr("The saved extension registry contains an invalid entry or linked directory."));
    }
    m_entries = entries;
    return true;
}

bool BrowserExtensionStore::beginMutation(std::unique_ptr<QLockFile> &lock, QString *error)
{
    // All profiles share this catalog. Lock, then reread before merging a single
    // change, so a stale profile never overwrites another profile's decisions.
    if (!loadRegistry(error) || !QDir().mkpath(m_root))
        return false;
    const QString lockPath = QDir(m_root).filePath("registry.lock");
    if (QFileInfo(lockPath).isSymLink())
        return fail(error, tr("The extension registry lock must not be a symbolic link."));
    lock = std::make_unique<QLockFile>(lockPath);
    if (!lock->tryLock(0))
        return fail(error, tr("Extension storage is busy. Retry after the other operation finishes."));
    if (!loadRegistry(error))
        return false;
    m_lastError.clear();
    return true;
}

bool BrowserExtensionStore::saveRegistry(const QJsonObject &entries, QString *error)
{
    const QByteArray contents = QJsonDocument(QJsonObject{{"version", 1}, {"extensions", entries}}).toJson();
    if (contents.size() > maximumRegistryBytes || !QDir().mkpath(m_root))
        return fail(error, tr("The extension registry is too large or its directory is unavailable."));
    QSaveFile registry(QDir(m_root).filePath("registry.json"));
    if (!registry.open(QIODevice::WriteOnly) || registry.write(contents) != contents.size() || !registry.commit())
        return fail(error, tr("Could not save the extension registry."));
    m_entries = entries;
    return true;
}

bool BrowserExtensionStore::persist(const QWebEngineExtensionInfo &extension, QString *error)
{
#ifdef DOCKPIPE_EXTENSION_STORE_AVAILABLE
    if (!extension.isLoaded() || !validId(extension.id()))
        return fail(error, tr("Only successfully loaded extensions can be saved."));
    std::unique_ptr<QLockFile> lock;
    if (!beginMutation(lock, error))
        return false;
    if (owns(extension.id()))
        return true;
    const QString destination = QDir(m_root).filePath(extension.id());
    if (QFileInfo::exists(destination))
        return fail(error, tr("An unmanaged extension directory already occupies this ID."));
    const QString staging = QDir(m_root).filePath(".install-" + QUuid::createUuid().toString(QUuid::WithoutBraces));
    if (!copyExtension(extension.path(), staging, error)) {
        QDir(staging).removeRecursively();
        return false;
    }
    if (!QDir().rename(staging, destination)) {
        QDir(staging).removeRecursively();
        return fail(error, tr("Could not publish the managed extension directory."));
    }
    QJsonObject entries = m_entries;
    entries.insert(extension.id(), QJsonObject{{"enabled", false}});
    if (!saveRegistry(entries, error)) {
        QDir(destination).removeRecursively();
        return false;
    }
    return true;
#else
    Q_UNUSED(extension);
    return fail(error, tr("Extension storage requires Qt WebEngine 6.10 extension support."));
#endif
}

bool BrowserExtensionStore::activate(const QString &id, QString *error)
{
#ifdef DOCKPIPE_EXTENSION_STORE_AVAILABLE
    if (!m_profile || m_profile->isOffTheRecord())
        return fail(error, tr("A regular browser profile is required to activate saved extensions."));
    if (!loadRegistry(error))
        return false;
    const QString key = storedId(id);
    if (key.isEmpty())
        return fail(error, tr("This extension is not saved in Dockpipe."));
    const QString path = QDir(m_root).filePath(key);
    if (QFileInfo(path).isSymLink() || !validateManifest(path, error))
        return false;
    m_profile->extensionManager()->loadExtension(path);
    return true;
#else
    Q_UNUSED(id);
    return fail(error, tr("Extension activation requires Qt WebEngine 6.10 extension support."));
#endif
}

bool BrowserExtensionStore::setEnabled(const QString &id, bool enabled, QString *error)
{
    std::unique_ptr<QLockFile> lock;
    if (!beginMutation(lock, error))
        return false;
    const QString key = storedId(id);
    if (key.isEmpty())
        return fail(error, tr("This extension is not saved in Dockpipe."));
    QJsonObject entries = m_entries;
    entries.insert(key, QJsonObject{{"enabled", enabled}});
    return saveRegistry(entries, error);
}

bool BrowserExtensionStore::remove(const QString &id, QString *error)
{
    std::unique_ptr<QLockFile> lock;
    if (!beginMutation(lock, error))
        return false;
    const QString key = storedId(id);
    if (key.isEmpty() || !validId(key))
        return fail(error, tr("This extension is not saved in Dockpipe."));
    QJsonObject entries = m_entries;
    entries.remove(key);
    if (!saveRegistry(entries, error))
        return false;
    if (!QDir(QDir(m_root).filePath(key)).removeRecursively())
        return fail(error, tr("The extension was forgotten, but its files could not be removed."));
    return true;
}

void BrowserExtensionStore::restore()
{
    if (!m_lastError.isEmpty()) {
        emit restoreFailed(m_lastError);
        return;
    }
#ifdef DOCKPIPE_EXTENSION_STORE_AVAILABLE
    if (!m_profile || m_profile->isOffTheRecord())
        return;
    auto *manager = m_profile->extensionManager();
    connect(manager, &QWebEngineExtensionManager::loadFinished, this,
        [this, manager](const QWebEngineExtensionInfo &extension) {
            QString error;
            if (!loadRegistry(&error)) {
                emit restoreFailed(error);
                return;
            }
            const QString key = storedId(extension.id());
            const QString loadedPath = QFileInfo(extension.path()).canonicalFilePath();
            const QString managedPath = QFileInfo(QDir(m_root).filePath(key)).canonicalFilePath();
            // A matching manifest ID alone never grants a newly loaded path trust.
            if (!key.isEmpty() && !loadedPath.isEmpty() && loadedPath == managedPath
                && extension.isLoaded() && enabled(extension.id()))
                manager->setExtensionEnabled(extension, true);
        });
    const QJsonObject restoringEntries = m_entries;
    for (auto entry = restoringEntries.begin(); entry != restoringEntries.end(); ++entry) {
        if (!validId(entry.key()))
            continue;
        const QString path = QDir(m_root).filePath(entry.key());
        QString error;
        if (QFileInfo(path).isSymLink() || !validateManifest(path, &error)) {
            emit restoreFailed(tr("Could not restore extension %1: %2").arg(entry.key(), error));
            continue;
        }
        manager->loadExtension(path);
    }
#endif
}
