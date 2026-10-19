#include "BrowserExtensionsDialog.h"
#include "BrowserExtensionPackage.h"
#include "BrowserExtensionStore.h"

#include <QDialogButtonBox>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QInputDialog>
#include <QLineEdit>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QFile>
#include <QMessageBox>
#include <QTimer>
#include <QSaveFile>
#include <QListWidget>
#include <QPushButton>
#include <QVBoxLayout>
#include <QWebEngineProfile>

#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#if QT_CONFIG(webengine_extensions)
#define DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
#include <QWebEngineExtensionInfo>
#include <QWebEngineExtensionManager>
#endif
#endif

BrowserExtensionsDialog::BrowserExtensionsDialog(QWebEngineProfile *profile, QWidget *parent)
    : QDialog(parent), m_profile(profile)
{
    setWindowTitle(tr("Browser extensions"));
    setObjectName("browserExtensionsDialog");
    resize(840, 600);
    auto *layout = new QVBoxLayout(this);
    auto *explanation = new QLabel(tr("Install extensions from the Chrome Web Store or a ZIP/CRX package. "
        "Review their permissions before enabling them. Extensions depend on the browser APIs supported by this build."));
    explanation->setWordWrap(true);
    layout->addWidget(explanation);
    m_extensions = new QListWidget;
    m_extensions->setObjectName("browserExtensionsList");
    layout->addWidget(m_extensions, 1);
    m_details = new QLabel;
    m_details->setTextFormat(Qt::PlainText);
    m_details->setWordWrap(true);
    m_details->setTextInteractionFlags(Qt::TextSelectableByMouse);
    layout->addWidget(m_details);
    auto *actions = new QHBoxLayout;
    m_webStore = new QPushButton(tr("Add from Chrome Web Store…"));
    m_webStore->setObjectName("browserExtensionWebStore");
    m_package = new QPushButton(tr("Install package…"));
    m_package->setObjectName("browserExtensionPackage");
    m_load = new QPushButton(tr("Load unpacked…"));
    m_load->setObjectName("browserExtensionLoad");
    m_toggle = new QPushButton(tr("Enable"));
    m_toggle->setObjectName("browserExtensionToggle");
    m_unload = new QPushButton(tr("Remove from session"));
    m_unload->setObjectName("browserExtensionUnload");
    m_open = new QPushButton(tr("Open extension"));
    m_open->setObjectName("browserExtensionOpen");
    auto *installActions = new QHBoxLayout;
    installActions->addWidget(m_webStore);
    installActions->addWidget(m_package);
    installActions->addWidget(m_load);
    installActions->addStretch();
    layout->addLayout(installActions);
    actions->addStretch();
    actions->addWidget(m_toggle);
    actions->addWidget(m_open);
    actions->addWidget(m_unload);
    layout->addLayout(actions);
    m_status = new QLabel;
    m_status->setObjectName("browserExtensionsStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Close);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
    connect(m_extensions, &QListWidget::currentRowChanged, this, &BrowserExtensionsDialog::updateSelection);
    connect(m_webStore, &QPushButton::clicked, this, &BrowserExtensionsDialog::chooseWebStoreExtension);
    connect(m_package, &QPushButton::clicked, this, &BrowserExtensionsDialog::choosePackage);
    connect(m_load, &QPushButton::clicked, this, &BrowserExtensionsDialog::loadUnpacked);
    connect(m_toggle, &QPushButton::clicked, this, &BrowserExtensionsDialog::toggleSelected);
    connect(m_unload, &QPushButton::clicked, this, &BrowserExtensionsDialog::unloadSelected);
    connect(m_open, &QPushButton::clicked, this, &BrowserExtensionsDialog::openSelected);
    if (profile) {
        connect(profile, &QObject::destroyed, this, [this]() {
            m_available = false;
            m_extensions->clear();
            m_status->setText(tr("The browser session has closed."));
            setBusy(false);
        });
    }

#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!profile || profile->isOffTheRecord()) {
        m_status->setText(tr("Extensions require a named browser profile and are unavailable in private profiles."));
    } else if (auto *manager = profile->extensionManager()) {
        m_available = true;
        m_store = BrowserExtensionStore::forProfile(profile);
        m_download = new BrowserExtensionPackage(this);
        connect(m_download, &BrowserExtensionPackage::ready, this, [this](const QString &path) {
            setBusy(false);
            installPackage(path, m_download->publicKey());
        });
        connect(m_download, &BrowserExtensionPackage::failed, this, [this](const QString &error) {
            setBusy(false);
            m_status->setText(error);
        });
        connect(manager, &QWebEngineExtensionManager::loadFinished, this,
            [this](const QWebEngineExtensionInfo &extension) {
                if ((!m_busy && m_pendingPath.isEmpty())
                    || (!m_pendingPath.isEmpty() && QFileInfo(extension.path()).absoluteFilePath() == m_pendingPath)) {
                    m_pendingPath.clear();
                    setBusy(false);
                    if (!extension.error().isEmpty())
                        m_status->setText(tr("Could not load extension: %1").arg(extension.error()));
                    else if (extension.isLoaded()) {
                        if (m_store && m_store->owns(extension.id()) && m_store->enabled(extension.id()))
                            m_status->setText(tr("Installed and enabled %1.").arg(extension.name()));
                        else
                            m_status->setText(tr("Loaded %1. Select Enable to allow it to run.").arg(extension.name()));
                    }
                }
                refreshExtensions();
            });
        auto removalFinished = [this](const QWebEngineExtensionInfo &extension) {
            if (!extension.error().isEmpty())
                m_status->setText(tr("Could not remove extension: %1").arg(extension.error()));
            refreshExtensions();
        };
        connect(manager, &QWebEngineExtensionManager::uninstallFinished, this, removalFinished);
        connect(manager, &QWebEngineExtensionManager::unloadFinished, this,
            [this](const QWebEngineExtensionInfo &extension) {
                if (!extension.error().isEmpty())
                    m_status->setText(tr("Could not remove extension: %1").arg(extension.error()));
                else
                    m_status->setText(tr("Removed %1 from this session.").arg(extension.name()));
                refreshExtensions();
            });
        m_status->setText(tr("Loaded extensions start disabled. Extension support depends on the APIs each extension uses."));
    } else {
        m_status->setText(tr("Extension support is unavailable in this browser runtime."));
    }
#else
    m_status->setText(tr("Extensions require a desktop build with Qt WebEngine 6.10 or newer and extension support enabled."));
#endif
    setBusy(false);
    refreshExtensions();
}

void BrowserExtensionsDialog::setBusy(bool busy)
{
    m_busy = busy;
    m_load->setEnabled(m_available && !busy);
    m_webStore->setEnabled(m_available && !busy);
    m_package->setEnabled(m_available && !busy);
    updateSelection();
}

void BrowserExtensionsDialog::refreshExtensions()
{
    const auto *selected = m_extensions->currentItem();
    const QString selectedId = selected ? selected->data(Qt::UserRole).toString() : QString();
    m_extensions->clear();
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (m_available && m_profile) {
        for (const auto &extension : m_profile->extensionManager()->extensions()) {
            const QString state = extension.isEnabled() ? tr("Enabled") : tr("Disabled");
            auto *item = new QListWidgetItem(tr("%1 — %2").arg(extension.name(), state), m_extensions);
            item->setData(Qt::UserRole, extension.id());
            if (extension.id() == selectedId)
                m_extensions->setCurrentItem(item);
        }
    }
#endif
    if (!m_extensions->currentItem() && m_extensions->count() > 0)
        m_extensions->setCurrentRow(0);
    updateSelection();
}

void BrowserExtensionsDialog::updateSelection()
{
    m_toggle->setEnabled(false);
    m_unload->setEnabled(false);
    m_open->setEnabled(false);
    m_details->clear();
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || !m_extensions->currentItem())
        return;
    const QString selectedId = m_extensions->currentItem()->data(Qt::UserRole).toString();
    for (const auto &extension : m_profile->extensionManager()->extensions()) {
        if (extension.id() != selectedId)
            continue;
        m_details->setText(tr("%1\nID: %2\n%3\n%4").arg(extension.name(), extension.id(), extension.description(), extension.path()));
        m_unload->setText(m_store && m_store->owns(selectedId) ? tr("Uninstall") : tr("Remove from session"));
        m_toggle->setText(extension.isEnabled() ? tr("Disable") : tr("Enable"));
        m_toggle->setEnabled(!m_busy);
        m_unload->setEnabled(!m_busy);
        const QUrl popup = extension.actionPopupUrl();
        m_open->setEnabled(!m_busy && extension.isEnabled()
            && popup.scheme() == "chrome-extension" && popup.host() == selectedId);
        return;
    }
#endif
}

void BrowserExtensionsDialog::loadUnpacked()
{
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || m_busy)
        return;
    QPointer<BrowserExtensionsDialog> owner(this);
    const QString directory = QFileDialog::getExistingDirectory(this, tr("Select an unpacked extension folder"));
    if (!owner || !m_profile || directory.isEmpty())
        return;
    if (!QFileInfo(QDir(directory).filePath("manifest.json")).isReadable()) {
        m_status->setText(tr("Choose a folder containing a readable manifest.json file."));
        return;
    }
    m_pendingPath = QFileInfo(directory).canonicalFilePath();
    setBusy(true);
    m_status->setText(tr("Loading extension… You can close this window while loading finishes."));
    m_profile->extensionManager()->loadExtension(m_pendingPath);
#endif
}

void BrowserExtensionsDialog::toggleSelected()
{
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || m_busy || !m_extensions->currentItem())
        return;
    const QString selectedId = m_extensions->currentItem()->data(Qt::UserRole).toString();
    auto *manager = m_profile->extensionManager();
    for (const auto &extension : manager->extensions()) {
        if (extension.id() != selectedId)
            continue;
        const bool enabled = !extension.isEnabled();
        QString error;
        if (m_store && m_store->owns(selectedId) && !m_store->setEnabled(selectedId, enabled, &error)) {
            m_status->setText(tr("Could not save extension setting: %1").arg(error));
            return;
        }
        manager->setExtensionEnabled(extension, enabled);
        if (extension.isEnabled() != enabled)
            m_status->setText(tr("The extension did not change state. Try reopening the extension manager."));
        else
            m_status->setText(enabled ? tr("Extension enabled.") : tr("Extension disabled."));
        refreshExtensions();
        return;
    }
#endif
}

void BrowserExtensionsDialog::unloadSelected()
{
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || m_busy || !m_extensions->currentItem())
        return;
    const QString selectedId = m_extensions->currentItem()->data(Qt::UserRole).toString();
    auto *manager = m_profile->extensionManager();
    for (const auto &extension : manager->extensions()) {
        if (extension.id() == selectedId) {
            if (m_store && m_store->owns(selectedId)) {
                QString error;
                if (!m_store->remove(selectedId, &error)) {
                    m_status->setText(tr("Could not uninstall extension: %1").arg(error));
                    return;
                }
                if (extension.isInstalled())
                    manager->uninstallExtension(extension);
                else
                    manager->unloadExtension(extension);
            } else {
                manager->unloadExtension(extension);
            }
            return;
        }
    }
#endif
}

void BrowserExtensionsDialog::openSelected()
{
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || m_busy || !m_extensions->currentItem())
        return;
    const QString selectedId = m_extensions->currentItem()->data(Qt::UserRole).toString();
    for (const auto &extension : m_profile->extensionManager()->extensions()) {
        const QUrl popup = extension.actionPopupUrl();
        if (extension.id() == selectedId && extension.isEnabled()
            && popup.scheme() == "chrome-extension" && popup.host() == selectedId) {
            QPointer<BrowserExtensionsDialog> owner(this);
            emit openUrl(popup);
            if (owner)
                accept();
            return;
        }
    }
#endif
}

void BrowserExtensionsDialog::chooseWebStoreExtension()
{
    bool accepted = false;
    const QString source = QInputDialog::getText(this, tr("Add from Chrome Web Store"),
        tr("Paste the extension’s Chrome Web Store URL or ID:"), QLineEdit::Normal, {}, &accepted);
    if (accepted)
        addWebStoreExtension(source);
}

void BrowserExtensionsDialog::choosePackage()
{
    const QString file = QFileDialog::getOpenFileName(this, tr("Install extension package"), {},
        tr("Extension packages (*.zip *.crx)"));
    if (!file.isEmpty())
        installPackage(file);
}

void BrowserExtensionsDialog::addWebStoreExtension(const QString &urlOrId)
{
    if (!m_available || !m_profile || !m_download || m_busy)
        return;
    const QString id = BrowserExtensionPackage::extensionId(urlOrId);
    if (id.isEmpty()) {
        m_status->setText(tr("Enter a valid Chrome Web Store extension URL or 32-character extension ID."));
        return;
    }
    setBusy(true);
    m_status->setText(tr("Downloading extension package… Close this window to cancel."));
    m_download->fetch(id);
}

void BrowserExtensionsDialog::installPackage(const QString &file, const QString &sourceKey)
{
#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
    if (!m_available || !m_profile || m_busy)
        return;
    auto directory = std::make_shared<QTemporaryDir>();
    QString error;
    const QString zip = directory->filePath("extension.zip");
    QString packageKey;
    if (!directory->isValid() || !BrowserExtensionPackage::prepareFile(file, zip, &error, &packageKey)) {
        m_status->setText(tr("Could not open extension package: %1").arg(error));
        return;
    }
    m_pendingKey = sourceKey.isEmpty() ? packageKey : sourceKey;
    m_packageDirectory = directory;
    m_pendingInstall = std::make_shared<PendingInstall>();
    const auto pending = m_pendingInstall;
    pending->waiting = true;
    auto *manager = m_profile->extensionManager();
    m_existingIds.clear();
    for (const auto &extension : manager->extensions())
        m_existingIds.append(extension.id());
    const QStringList existingIds = m_existingIds;
    QPointer<BrowserExtensionsDialog> owner(this);
    auto connection = std::make_shared<QMetaObject::Connection>();
    *connection = connect(manager, &QWebEngineExtensionManager::installFinished, manager,
        [owner, manager, pending, directory, connection, existingIds](const QWebEngineExtensionInfo &extension) {
            QObject::disconnect(*connection);
            pending->waiting = false;
            if (!owner || pending->cancelled) {
                if (extension.isInstalled() && !existingIds.contains(extension.id())) {
                    manager->setExtensionEnabled(extension, false);
                    manager->uninstallExtension(extension);
                }
                if (owner) {
                    owner->setBusy(false);
                    owner->refreshExtensions();
                }
                return;
            }
            owner->finishInstall(extension);
        });
    QTimer::singleShot(60000, this, [this, pending]() {
        if (!pending->waiting)
            return;
        pending->cancelled = true;
        m_status->setText(tr("Extension installation timed out. Close this window and try again later. "
            "Any late installation will be removed."));
    });
    setBusy(true);
    m_status->setText(tr("Preparing extension for permission review…"));
    manager->installExtension(zip);
#else
    Q_UNUSED(file);
    Q_UNUSED(sourceKey);
#endif
}

#ifdef DOCKPIPE_BROWSER_EXTENSIONS_AVAILABLE
static bool saveExtensionKey(const QWebEngineExtensionInfo &extension, const QString &key, QString *error)
{
    if (key.isEmpty())
        return true;
    const QString path = QDir(extension.path()).filePath("manifest.json");
    QFile manifest(path);
    if (!manifest.open(QIODevice::ReadOnly) || manifest.size() > 1024 * 1024) {
        *error = QObject::tr("The installed extension manifest could not be read.");
        return false;
    }
    const QJsonDocument document = QJsonDocument::fromJson(manifest.readAll());
    manifest.close();
    if (!document.isObject()) {
        *error = QObject::tr("The installed extension manifest is invalid.");
        return false;
    }
    QJsonObject object = document.object();
    object.insert("key", key);
    const QByteArray contents = QJsonDocument(object).toJson();
    QSaveFile output(path);
    if (!output.open(QIODevice::WriteOnly) || output.write(contents) != contents.size() || !output.commit()) {
        *error = QObject::tr("Could not preserve the extension’s signed identity.");
        return false;
    }
    return true;
}

static QString extensionPermissions(const QWebEngineExtensionInfo &extension, QString *error)
{
    QFile manifest(QDir(extension.path()).filePath("manifest.json"));
    if (!manifest.open(QIODevice::ReadOnly) || manifest.size() > 1024 * 1024) {
        *error = QObject::tr("The installed manifest could not be read for permission review.");
        return {};
    }
    QJsonParseError parseError;
    const QJsonDocument document = QJsonDocument::fromJson(manifest.readAll(), &parseError);
    if (parseError.error != QJsonParseError::NoError || !document.isObject()) {
        *error = QObject::tr("The extension manifest is invalid.");
        return {};
    }
    const QJsonObject object = document.object();
    QStringList permissions;
    auto appendArray = [&permissions](const QJsonArray &values, const QString &label) {
        for (const auto &value : values) {
            if (value.isString())
                permissions.append(label + value.toString());
        }
    };
    appendArray(object.value("permissions").toArray(), QObject::tr("API: "));
    appendArray(object.value("host_permissions").toArray(), QObject::tr("Website: "));
    for (const auto &script : object.value("content_scripts").toArray())
        appendArray(script.toObject().value("matches").toArray(), QObject::tr("Page content: "));
    appendArray(object.value("optional_permissions").toArray(), QObject::tr("Optional API: "));
    appendArray(object.value("optional_host_permissions").toArray(), QObject::tr("Optional website: "));
    permissions.removeDuplicates();
    if (permissions.size() > 256 || permissions.join('\n').size() > 65536) {
        *error = QObject::tr("The requested permission list exceeds the review limit.");
        return {};
    }
    return permissions.isEmpty() ? QObject::tr("No explicit permissions requested.") : permissions.join('\n');
}

void BrowserExtensionsDialog::finishInstall(const QWebEngineExtensionInfo &extension)
{
    QPointer<QWebEngineExtensionManager> manager = m_profile ? m_profile->extensionManager() : nullptr;
    if (!manager)
        return;
    if (!extension.error().isEmpty() || !extension.isInstalled()) {
        setBusy(false);
        m_status->setText(tr("Could not install extension: %1").arg(extension.error()));
        return;
    }
    if (m_existingIds.contains(extension.id())) {
        setBusy(false);
        m_status->setText(tr("This extension is already installed. Remove it before installing another package with the same ID."));
        refreshExtensions();
        return;
    }
    manager->setExtensionEnabled(extension, false);
    QString error;
    if (!saveExtensionKey(extension, m_pendingKey, &error)) {
        setBusy(false);
        manager->uninstallExtension(extension);
        m_status->setText(error);
        return;
    }
    const QString permissions = extensionPermissions(extension, &error);
    if (!error.isEmpty()) {
        setBusy(false);
        manager->uninstallExtension(extension);
        m_status->setText(error);
        return;
    }
    auto *confirmation = new QMessageBox(QMessageBox::Question, tr("Install and enable extension?"), {},
        QMessageBox::Yes | QMessageBox::Cancel, this);
    confirmation->setObjectName("browserExtensionPermissions");
    confirmation->setTextFormat(Qt::PlainText);
    confirmation->setText(tr("%1\nID: %2\n\n%3\n\nRequested permissions:\n%4")
        .arg(extension.name(), extension.id(), extension.description(), permissions));
    confirmation->setInformativeText(tr("This extension will be available after restarting Dockpipe. "
        "Allow it to access the websites and browser features listed above?"));
    confirmation->setDefaultButton(QMessageBox::Cancel);
    confirmation->button(QMessageBox::Yes)->setText(tr("Install and enable"));
    QPointer<BrowserExtensionsDialog> owner(this);
    const int answer = confirmation->exec();
    if (!owner) {
        if (manager)
            manager->uninstallExtension(extension);
        return;
    }
    confirmation->deleteLater();
    if (!manager)
        return;
    if (answer != QMessageBox::Yes || (m_pendingInstall && m_pendingInstall->cancelled)) {
        manager->uninstallExtension(extension);
        m_status->setText(tr("Installation cancelled. The extension was removed."));
    } else if (!m_store || !m_store->persist(extension, &error)
        || !m_store->setEnabled(extension.id(), true, &error)) {
        if (m_store && m_store->owns(extension.id())) {
            QString cleanupError;
            m_store->remove(extension.id(), &cleanupError);
        }
        manager->uninstallExtension(extension);
        m_status->setText(tr("Could not save extension: %1").arg(error));
    } else {
        manager->uninstallExtension(extension);
        if (m_store->activate(extension.id(), &error))
            m_status->setText(tr("Installed and enabled %1.").arg(extension.name()));
        else
            m_status->setText(tr("Extension saved but could not be loaded: %1").arg(error));
    }
    m_packageDirectory.reset();
    setBusy(false);
    refreshExtensions();
}
#endif

void BrowserExtensionsDialog::reject()
{
    if (m_pendingInstall)
        m_pendingInstall->cancelled = true;
    if (m_download)
        m_download->cancel();
    QDialog::reject();
}
