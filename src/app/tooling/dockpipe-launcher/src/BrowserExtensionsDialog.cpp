#include "BrowserExtensionsDialog.h"

#include <QDialogButtonBox>
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
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
    resize(680, 480);
    auto *layout = new QVBoxLayout(this);
    auto *explanation = new QLabel(tr("Load unpacked Manifest V3 extensions for this app session. "
        "Enable only extensions you trust: they can read or change websites you visit."));
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
    m_load = new QPushButton(tr("Load unpacked…"));
    m_load->setObjectName("browserExtensionLoad");
    m_toggle = new QPushButton(tr("Enable"));
    m_toggle->setObjectName("browserExtensionToggle");
    m_unload = new QPushButton(tr("Remove from session"));
    m_unload->setObjectName("browserExtensionUnload");
    m_open = new QPushButton(tr("Open extension"));
    m_open->setObjectName("browserExtensionOpen");
    actions->addWidget(m_load);
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
        connect(manager, &QWebEngineExtensionManager::loadFinished, this,
            [this](const QWebEngineExtensionInfo &extension) {
                if (m_pendingPath.isEmpty() || QFileInfo(extension.path()).absoluteFilePath() == m_pendingPath) {
                    m_pendingPath.clear();
                    setBusy(false);
                    if (!extension.error().isEmpty())
                        m_status->setText(tr("Could not load extension: %1").arg(extension.error()));
                    else if (extension.isLoaded())
                        m_status->setText(tr("Loaded %1. Select Enable to allow it to run.").arg(extension.name()));
                }
                refreshExtensions();
            });
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
        m_details->setText(tr("%1\nID: %2\n%3").arg(extension.description(), extension.id(), extension.path()));
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
            manager->unloadExtension(extension);
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
