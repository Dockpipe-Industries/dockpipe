#include "RemoteDesktopWidget.h"

#include "DockpipeChoices.h"
#include "PackageDisplayWidget.h"
#include "RemoteAccessMonitor.h"
#include "RemoteDesktopRecoveryDialog.h"
#include "RemoteDesktopSettingsDialog.h"

#include <QComboBox>
#include <QCheckBox>
#include <QDir>
#include <QDialog>
#include <QFileInfo>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QMenu>
#include <QPushButton>
#include <QRegularExpression>
#include <QShowEvent>
#include <QSettings>
#include <QSignalBlocker>
#include <QStandardItemModel>
#include <QTimer>
#include <QUuid>
#include <QVBoxLayout>

namespace {
bool digest(const QJsonValue &value)
{
    static const QRegularExpression expression(QStringLiteral("^[0-9a-f]{64}$"));
    return value.isString() && expression.match(value.toString()).hasMatch();
}

bool validCatalog(const QJsonObject &catalog)
{
    const auto target = catalog.value("target").toObject();
    const auto provider = catalog.value("provider").toObject();
    static const QRegularExpression node(QStringLiteral("^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$"));
    return catalog.value("schema") == QJsonValue("dockpipe.remote.desktop/v1")
        && node.match(target.value("node").toString()).hasMatch()
        && digest(target.value("fingerprint")) && digest(target.value("generation"))
        && digest(catalog.value("epoch")) && digest(provider.value("artifact"))
        && !provider.value("id").toString().isEmpty() && provider.value("id").toString().size() <= 128
        && !provider.value("version").toString().isEmpty() && provider.value("version").toString().size() <= 32
        && catalog.value("view").isBool() && catalog.value("input").isBool()
        && (!catalog.value("input").toBool() || catalog.value("view").toBool());
}

QJsonObject identityOf(const QJsonObject &catalog)
{
    const auto target = catalog.value("target").toObject();
    return {{"node", target.value("node")}, {"fingerprint", target.value("fingerprint")}};
}

QString displayPreferenceKey(const QJsonObject &catalog)
{
    return QStringLiteral("remote/physicalDesktop/%1").arg(identityOf(catalog).value("fingerprint").toString());
}

struct ProviderRestrictions {
    bool clipboardUnavailable = false;
    bool clipboardImagesUnavailable = false;
    bool clipboardFilesUnavailable = false;
    bool virtualDesktopUnavailable = false;
    bool showLocalCursor = false;
    QString dropPasteModifier;

    void apply(QJsonObject &capture, QJsonObject &presentation) const
    {
        if (clipboardUnavailable)
            capture.remove("clipboard_text");
        if (clipboardImagesUnavailable)
            capture.remove("clipboard_images");
        if (clipboardFilesUnavailable)
            capture.remove("clipboard_files");
        if (virtualDesktopUnavailable && capture.value("display_source") == QJsonValue("virtual"))
            capture.remove("display_source");
        if (showLocalCursor)
            presentation.insert("hide_cursor", false);
        if (!dropPasteModifier.isEmpty() && capture.value("clipboard_files").toBool())
            presentation.insert("drop_paste_modifier", dropPasteModifier);
    }
};

ProviderRestrictions restrictionsFor(const QJsonObject &catalog)
{
    const auto artifact = catalog.value("provider").toObject().value("artifact");
    if (!digest(artifact))
        return {};

    // Preview capabilities are local and bound to the exact sender image.
    QSettings preferences;
    preferences.beginGroup(QStringLiteral("remote/desktopProviderRestrictions/") + artifact.toString());
    const bool clipboardUnavailable = preferences.value(QStringLiteral("clipboardUnavailable")).toBool();
    const bool richUnavailable = clipboardUnavailable
        || preferences.value(QStringLiteral("richClipboardUnavailable")).toBool();
    const auto pasteModifier = preferences.value(QStringLiteral("dropPasteModifier")).toString();
    return {
        clipboardUnavailable,
        richUnavailable || preferences.value(QStringLiteral("clipboardImagesUnavailable")).toBool(),
        richUnavailable || preferences.value(QStringLiteral("clipboardFilesUnavailable")).toBool(),
        preferences.value(QStringLiteral("virtualDesktopUnavailable")).toBool(),
        preferences.value(QStringLiteral("showLocalCursor")).toBool(),
        pasteModifier == "control" || pasteModifier == "command" ? pasteModifier : QString{}
    };
}
}

RemoteDesktopWidget::RemoteDesktopWidget(QWidget *parent)
    : QWidget(parent), m_command(this)
{
    setObjectName(QStringLiteral("remoteDesktopPage"));
    const QSettings preferences;
    QString quality = preferences.value(QStringLiteral("remote/desktopQuality"), QStringLiteral("high")).toString();
    if (quality != "low" && quality != "balanced" && quality != "high" && quality != "ultra")
        quality = QStringLiteral("high");
    int frameRate = preferences.value(QStringLiteral("remote/desktopFrameRate"), 30).toInt();
    if (frameRate != 10 && frameRate != 15 && frameRate != 24 && frameRate != 30 && frameRate != 60)
        frameRate = 30;
    const QString scale = preferences.value(QStringLiteral("remote/desktopScale"), QStringLiteral("fit")).toString();
    m_captureOptions = {{"quality", quality}, {"frame_rate", frameRate}};
    if (preferences.value(QStringLiteral("remote/desktopShareClipboard"), true).toBool())
        m_captureOptions.insert("clipboard_text", true);
    if (preferences.value(QStringLiteral("remote/desktopShareImages"), false).toBool())
        m_captureOptions.insert("clipboard_images", true);
    if (preferences.value(QStringLiteral("remote/desktopShareFiles"), false).toBool())
        m_captureOptions.insert("clipboard_files", true);
    if (preferences.value(QStringLiteral("remote/desktopDisplaySource")).toString() == "virtual")
        m_captureOptions.insert("display_source", "virtual");
    m_presentationOptions = {{"scale", scale == "actual" ? "actual" : "fit"},
        {"hide_cursor", preferences.value(QStringLiteral("remote/desktopHideCursor"), true).toBool()}};
    if (preferences.value(QStringLiteral("remote/desktopSwapControlCommand"), false).toBool())
        m_presentationOptions.insert("swap_control_command", true);
    m_decoderMode = preferences.value(QStringLiteral("remote/desktopDecoder"), QStringLiteral("software")).toString();
    if (m_decoderMode != "auto" && m_decoderMode != "hardware" && m_decoderMode != "software")
        m_decoderMode = QStringLiteral("software");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(24, 20, 24, 20);
    auto *title = new QLabel(tr("Remote Desktop"));
    title->setObjectName(QStringLiteral("pageTitle"));
    layout->addWidget(title);
    auto *controls = new QHBoxLayout;
    m_machine = new QComboBox;
    m_machine->setObjectName(QStringLiteral("desktopMachine"));
    m_machine->setMinimumContentsLength(18);
    m_machine->setMaximumWidth(320);
    m_machine->setPlaceholderText(tr("Choose a paired computer"));
    m_mode = new QComboBox;
    m_mode->setObjectName(QStringLiteral("desktopMode"));
    m_mode->addItem(tr("View"), QStringLiteral("view"));
    m_mode->addItem(tr("Control"), QStringLiteral("control"));
    m_mode->setToolTip(tr("View or control an existing desktop through its authorized native provider."));
    m_connect = new QPushButton(tr("Connect"));
    m_connect->setObjectName(QStringLiteral("desktopConnect"));
    m_disconnect = new QPushButton(tr("Disconnect"));
    m_disconnect->setObjectName(QStringLiteral("desktopDisconnect"));
    m_refresh = new QPushButton(tr("Refresh"));
    m_refresh->setObjectName(QStringLiteral("desktopRefresh"));
    m_unlock = new QPushButton(tr("Unlock remote access"));
    m_unlock->setObjectName(QStringLiteral("desktopUnlock"));
    m_packages = new QPushButton(tr("Set up provider…"));
    m_packages->setObjectName(QStringLiteral("desktopSetup"));
    controls->addWidget(m_machine);
    controls->addWidget(m_mode);
    controls->addWidget(m_connect);
    controls->addWidget(m_disconnect);
    m_keepConnected = new QCheckBox(tr("Keep connected across tabs"));
    m_keepConnected->setObjectName(QStringLiteral("desktopKeepConnected"));
    m_keepConnected->setChecked(QSettings().value(QStringLiteral("remote/keepDesktopConnectedAcrossTabs"), false).toBool());
    controls->addStretch();
    m_settings = new QPushButton(tr("Settings…"));
    m_settings->setObjectName(QStringLiteral("desktopSettings"));
    m_fullscreen = new QPushButton(tr("Fullscreen"));
    m_fullscreen->setObjectName(QStringLiteral("desktopFullscreen"));
    controls->addWidget(m_refresh);
    controls->addWidget(m_unlock);
    m_retained = RemoteDesktopRecoveryDialog::createButton(this);
    layout->addLayout(controls);
    auto *preferenceRow = new QWidget;
    preferenceRow->setObjectName(QStringLiteral("desktopCompactControls"));
    auto *preferenceControls = new QHBoxLayout(preferenceRow);
    preferenceControls->setContentsMargins(0, 0, 0, 0);
    preferenceControls->addWidget(m_keepConnected);
    preferenceControls->addStretch();
    preferenceControls->addWidget(m_settings);
    preferenceControls->addWidget(m_fullscreen);
    layout->addWidget(preferenceRow);
    auto *providerControls = new QHBoxLayout;
    providerControls->addWidget(m_packages);
    providerControls->addWidget(m_retained);
    providerControls->addStretch();
    layout->addLayout(providerControls);
    setTabOrder(m_unlock, m_keepConnected);
    setTabOrder(m_fullscreen, m_packages);
    setTabOrder(m_packages, m_retained);
    m_status = new QLabel(tr("Choose a paired computer and refresh its desktop readiness."));
    m_status->setObjectName(QStringLiteral("desktopStatus"));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_status->setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Maximum);
    layout->addWidget(m_status);
    m_preferencesNotice = new QLabel;
    m_preferencesNotice->setObjectName(QStringLiteral("desktopPreferencesNotice"));
    m_preferencesNotice->setTextFormat(Qt::PlainText);
    m_preferencesNotice->setWordWrap(true);
    m_preferencesNotice->hide();
    layout->addWidget(m_preferencesNotice);
    m_decoderStatus = new QLabel;
    m_decoderStatus->setObjectName(QStringLiteral("desktopDecoderStatus"));
    m_decoderStatus->setTextFormat(Qt::PlainText);
    m_decoderStatus->setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Maximum);
    m_decoderStatus->hide();
    layout->addWidget(m_decoderStatus);
    m_display = new PackageDisplayWidget;
    m_display->setObjectName(QStringLiteral("remoteDesktopDisplay"));
    m_display->setCloseOnHide(true);
    m_display->setKeepSessionOnHide(m_keepConnected->isChecked());
    m_display->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Expanding);
    layout->addWidget(m_display, 1);
    m_timer = new QTimer(this);
    m_timer->setInterval(15000);
    m_decoderTimer = new QTimer(this);
    m_decoderTimer->setInterval(500);
    connect(m_decoderTimer, &QTimer::timeout, this, &RemoteDesktopWidget::refreshDecoderStatus);
    connect(m_keepConnected, &QCheckBox::toggled, this, [this](bool checked) {
        QSettings().setValue(QStringLiteral("remote/keepDesktopConnectedAcrossTabs"), checked);
        m_display->setKeepSessionOnHide(checked || !m_fullscreenWindow.isNull());
        if (!checked && !isVisible())
            disconnectDesktop();
    });
    connect(m_timer, &QTimer::timeout, this, &RemoteDesktopWidget::refresh);
    connect(m_settings, &QPushButton::clicked, this, &RemoteDesktopWidget::showDesktopSettings);
    connect(m_fullscreen, &QPushButton::clicked, this, &RemoteDesktopWidget::enterFullscreen);
    connect(&m_command, &RemoteCommand::busyChanged, this, [this](bool busy) {
        updateActions();
        if (!busy && !m_busy && isVisible())
            QTimer::singleShot(0, this, &RemoteDesktopWidget::refresh);
    });
    connect(m_machine, &QComboBox::currentIndexChanged, this, [this] {
        m_automaticMode = true;
        applySelection();
    });
    connect(m_mode, &QComboBox::activated, this, [this] { m_automaticMode = false; });
    connect(m_mode, &QComboBox::currentIndexChanged, this, &RemoteDesktopWidget::updateActions);
    connect(m_refresh, &QPushButton::clicked, this, &RemoteDesktopWidget::refresh);
    connect(m_connect, &QPushButton::clicked, this, [this] { connectDesktop(selectedIdentity(), m_mode->currentData().toString()); });
    connect(m_disconnect, &QPushButton::clicked, this, &RemoteDesktopWidget::disconnectDesktop);
    connect(m_packages, &QPushButton::clicked, this, &RemoteDesktopWidget::showProviderSetup);
    connect(m_unlock, &QPushButton::clicked, this, [this] { if (m_access) m_access->unlock(); });
    connect(m_display, &PackageDisplayWidget::displayFailed, this, [this](const QString &error) {
        if (m_closing)
            return;
        m_connected = false;
        const auto decoderFailure = m_display->surfaceMetadata(QByteArrayLiteral("decoderFailure"));
        const auto code = decoderFailure.value("code").toString();
        if (code == "hardware-unavailable" || code == "hardware-failed")
            m_status->setText(tr("Hardware decoding failed. Choose Automatic or Software and reconnect."));
        else
            m_status->setText(error.left(4096));
        updateActions();
        QTimer::singleShot(0, this, [this] { if (!m_connected && !m_closing) m_display->clearDisplay(); });
    });
    connect(m_display, &PackageDisplayWidget::displayCloseCompleted, this, [this](const QJsonObject &result) {
        if (!m_closing)
            return;
        const auto reference = result.value("reference").toObject();
        const bool closed = result.size() == 3 && result.value("schema") == QJsonValue("dockpipe.desktop.close/v1")
            && result.value("outcome") == QJsonValue("closed") && reference.size() == 4
            && reference.value("schema") == QJsonValue("dockpipe.remote.desktop/v1") && digest(reference.value("id"))
            && reference.value("target") == m_displayCatalog.value("target")
            && reference.value("epoch") == m_displayCatalog.value("epoch");
        m_closing = false;
        m_displayCatalog = {};
        m_status->setText(closed ? tr("Disconnected. Remote cleanup was verified.")
                                : tr("Disconnected locally. The native provider reports any unconfirmed remote cleanup."));
        updateActions();
    });
    updateActions();
}

RemoteDesktopWidget::~RemoteDesktopWidget()
{
    m_connected = false;
    m_closing = true;
    m_timer->stop();
    m_decoderTimer->stop();
    disconnect(&m_command, nullptr, this, nullptr);
    cancelRefresh();
    disconnect(m_display, nullptr, this, nullptr);
    exitFullscreen();
}

void RemoteDesktopWidget::setCompactPresentation(bool enabled)
{
    if (m_compactPresentation == enabled)
        return;
    m_compactPresentation = enabled;
    m_keepConnected->setText(enabled ? tr("Keep connected") : tr("Keep connected across tabs"));
    m_keepConnected->setToolTip(tr("Keep connected across tabs"));
    m_machine->setMinimumContentsLength(enabled ? 8 : 18);
    m_machine->setMaximumWidth(enabled ? 180 : 320);
    findChild<QLabel *>(QStringLiteral("pageTitle"))->setVisible(!enabled);
    const int margin = enabled ? 8 : 24;
    layout()->setContentsMargins(margin, enabled ? 8 : 20, margin, enabled ? 8 : 20);
    updateActions();
}

void RemoteDesktopWidget::showDesktopSettings()
{
    const auto catalog = m_connected ? m_displayCatalog : selectedCatalog();
    const auto displays = catalog.value("displays").toArray();
    const auto restrictions = restrictionsFor(catalog);
    QSettings preferences;
    const auto physicalDisplayKey = displayPreferenceKey(catalog);
    const auto previousDisplay = preferences.value(physicalDisplayKey).toString();
    RemoteDesktopSettingsDialog dialog(m_captureOptions, m_presentationOptions, m_decoderMode, this);
    dialog.setProviderRestrictions(restrictions.clipboardUnavailable, restrictions.virtualDesktopUnavailable,
        restrictions.showLocalCursor, restrictions.clipboardImagesUnavailable, restrictions.clipboardFilesUnavailable);
    dialog.setPhysicalDisplays(displays, previousDisplay);
    if (dialog.exec() != QDialog::Accepted)
        return;
    auto capture = dialog.captureOptions();
    auto presentation = dialog.presentationOptions();
    if (restrictions.clipboardUnavailable && m_captureOptions.contains("clipboard_text"))
        capture.insert("clipboard_text", m_captureOptions.value("clipboard_text"));
    if (restrictions.clipboardImagesUnavailable && m_captureOptions.contains("clipboard_images"))
        capture.insert("clipboard_images", m_captureOptions.value("clipboard_images"));
    if (restrictions.clipboardFilesUnavailable && m_captureOptions.contains("clipboard_files"))
        capture.insert("clipboard_files", m_captureOptions.value("clipboard_files"));
    bool physicalDisplayChanged = false;
    if (!displays.isEmpty()) {
        const auto physicalDisplay = capture.take("display_source").toString();
        physicalDisplayChanged = physicalDisplay != previousDisplay;
        if (physicalDisplay.isEmpty())
            preferences.remove(physicalDisplayKey);
        else
            preferences.setValue(physicalDisplayKey, physicalDisplay);
    }
    if ((!displays.isEmpty() || restrictions.virtualDesktopUnavailable) && m_captureOptions.contains("display_source"))
        capture.insert("display_source", m_captureOptions.value("display_source"));
    if (restrictions.showLocalCursor)
        presentation.insert("hide_cursor", m_presentationOptions.value("hide_cursor").toBool(true));
    const bool captureChanged = capture != m_captureOptions;
    const bool presentationChanged = presentation != m_presentationOptions;
    const bool decoderChanged = dialog.decoderMode() != m_decoderMode;
    m_captureOptions = capture;
    m_presentationOptions = presentation;
    m_decoderMode = dialog.decoderMode();
    preferences.setValue(QStringLiteral("remote/desktopQuality"), capture.value("quality").toString());
    preferences.setValue(QStringLiteral("remote/desktopFrameRate"), capture.value("frame_rate").toInt());
    preferences.setValue(QStringLiteral("remote/desktopShareClipboard"), capture.value("clipboard_text").toBool());
    preferences.setValue(QStringLiteral("remote/desktopShareImages"), capture.value("clipboard_images").toBool());
    preferences.setValue(QStringLiteral("remote/desktopShareFiles"), capture.value("clipboard_files").toBool());
    preferences.setValue(QStringLiteral("remote/desktopDisplaySource"),
        capture.value("display_source").toString(QStringLiteral("existing")));
    preferences.setValue(QStringLiteral("remote/desktopScale"), presentation.value("scale").toString());
    preferences.setValue(QStringLiteral("remote/desktopHideCursor"), presentation.value("hide_cursor").toBool());
    preferences.setValue(QStringLiteral("remote/desktopSwapControlCommand"), presentation.value("swap_control_command").toBool());
    preferences.setValue(QStringLiteral("remote/desktopDecoder"), m_decoderMode);

    restrictions.apply(capture, presentation);
    const bool presentationDeferred = m_connected && presentationChanged
        && !m_display->updatePresentationOptions(presentation);
    if (m_connected && (captureChanged || decoderChanged || presentationDeferred || physicalDisplayChanged)) {
        m_preferencesNotice->setText(tr("Some settings will apply on the next connection."));
        m_preferencesNotice->show();
    }
}

void RemoteDesktopWidget::enterFullscreen()
{
    if (!m_connected || m_closing || m_fullscreenWindow || !m_display->hasPackageSurface())
        return;
    auto *dialog = new QDialog(this, Qt::Window);
    dialog->setObjectName(QStringLiteral("desktopFullscreenWindow"));
    dialog->setWindowTitle(tr("Remote Desktop"));
    auto *layout = new QVBoxLayout(dialog);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *exit = new QPushButton(tr("Exit fullscreen"));
    exit->setObjectName(QStringLiteral("desktopExitFullscreen"));
    exit->setAutoDefault(false);
    layout->addWidget(exit, 0, Qt::AlignRight);
    m_fullscreenWindow = dialog;
    m_display->setKeepSessionOnHide(true);
    this->layout()->removeWidget(m_display);
    m_display->setParent(dialog);
    layout->addWidget(m_display, 1);
    connect(exit, &QPushButton::clicked, this, &RemoteDesktopWidget::exitFullscreen);
    connect(dialog, &QDialog::finished, this, [this, dialog] {
        if (m_fullscreenWindow == dialog)
            exitFullscreen();
    });
    dialog->showFullScreen();
    m_display->show();
}

void RemoteDesktopWidget::exitFullscreen()
{
    auto *dialog = m_fullscreenWindow.data();
    if (!dialog)
        return;
    m_fullscreenWindow = nullptr;
    disconnect(dialog, &QDialog::finished, this, nullptr);
    dialog->layout()->removeWidget(m_display);
    m_display->setParent(this);
    qobject_cast<QVBoxLayout *>(layout())->addWidget(m_display, 1);
    m_display->show();
    m_display->setKeepSessionOnHide(m_keepConnected->isChecked());
    dialog->close();
    dialog->deleteLater();
}

void RemoteDesktopWidget::refreshDecoderStatus()
{
    const auto metadata = m_display->surfaceMetadata(QByteArrayLiteral("decoderStatus"));
    const auto backend = metadata.value("backend").toString();
    QString description;
    if (backend == "cuda") {
        description = tr("Hardware (CUDA/NVDEC)");
    } else if (backend == "vaapi") {
        description = tr("Hardware (VAAPI)");
    } else if (backend == "software") {
        const bool fallback = metadata.value("requested") == QJsonValue("auto")
            && !metadata.value("fallback_reason").toString().isEmpty();
        description = fallback ? tr("Software (automatic fallback)") : tr("Software");
    }
    m_decoderStatus->setVisible(m_connected && !description.isEmpty());
    if (!description.isEmpty())
        m_decoderStatus->setText(tr("Decoder: %1").arg(description));
}

void RemoteDesktopWidget::setResolvers(const QVector<ResolverMeta> &resolvers)
{
    m_resolvers = resolvers;
    updateActions();
}

void RemoteDesktopWidget::setAccessMonitor(RemoteAccessMonitor *monitor)
{
    if (m_access)
        disconnect(m_access, nullptr, this, nullptr);
    m_access = monitor;
    if (monitor) {
        connect(monitor, &RemoteAccessMonitor::changed, this, [this] {
            if (!accessUnlocked()) {
                cancelRefresh();
                disconnectDesktop();
                m_status->setText(m_access->detail());
            }
            updateActions();
        });
        connect(monitor, &RemoteAccessMonitor::unlockFinished, this, [this](bool success) { if (success && isVisible()) refresh(); });
    }
    if (!accessUnlocked()) {
        cancelRefresh();
        disconnectDesktop();
    }
    updateActions();
}

bool RemoteDesktopWidget::accessUnlocked() const
{
    return m_access && m_access->state() == RemoteAccessMonitor::State::Unlocked;
}

void RemoteDesktopWidget::setPairedMachines(const QJsonArray &machines)
{
    const auto selected = selectedIdentity();
    m_paired = machines;
    const QSignalBlocker blocker(m_machine);
    m_machine->clear();
    for (const auto &value : machines) {
        const auto machine = value.toObject();
        if (machine.value("status") != QJsonValue("paired") || machine.value("node").toString().isEmpty())
            continue;
        const QJsonObject identity{{"node", machine.value("node")}, {"fingerprint", machine.value("fingerprint")}};
        m_machine->addItem(digest(machine.value("fingerprint")) ? machine.value("node").toString()
            : tr("%1 (pairing update required)").arg(machine.value("node").toString()), identity);
    }
    int selectedIndex = -1;
    for (int index = 0; index < m_machine->count(); ++index) {
        if (m_machine->itemData(index).toJsonObject() == selected)
            selectedIndex = index;
    }
    m_machine->setCurrentIndex(selectedIndex);
    if (m_connected && selectedIndex < 0)
        disconnectDesktop();
    applySelection();
}

QJsonObject RemoteDesktopWidget::selectedIdentity() const
{
    return m_machine->currentData().toJsonObject();
}

QJsonObject RemoteDesktopWidget::selectedCatalog() const
{
    const auto selected = selectedIdentity();
    if (selected.isEmpty())
        return {};
    QJsonObject matched;
    for (const auto &value : m_catalogs) {
        const auto catalog = value.toObject();
        if (identityOf(catalog) == selected) {
            if (!matched.isEmpty())
                return {};
            matched = catalog;
        }
    }
    return matched;
}

const ResolverMeta *RemoteDesktopWidget::providerFor(const QJsonObject &catalog) const
{
    const auto provider = catalog.value("provider").toObject();
    const ResolverMeta *matched = nullptr;
    for (const auto &resolver : m_resolvers) {
        if (resolver.capability == "remote.desktop" && resolver.name == provider.value("id").toString()
            && resolver.version == provider.value("version").toString() && resolver.surface.nativeDisplay.interfaceVersion == 1
            && !resolver.surface.nativeDisplay.libraries.value(PackageDisplayWidget::platformName()).isEmpty()) {
            if (matched)
                return nullptr;
            matched = &resolver;
        }
    }
    return matched;
}

void RemoteDesktopWidget::applySelection()
{
    if (m_connected || m_closing) {
        updateActions();
        return;
    }

    const auto catalog = selectedCatalog();
    const bool control = catalog.value("input").toBool();
    if (auto *model = qobject_cast<QStandardItemModel *>(m_mode->model()))
        model->item(1)->setEnabled(control);
    if (m_automaticMode)
        m_mode->setCurrentIndex(catalog.isEmpty() || control ? 1 : 0);
    else if (!control && m_mode->currentData() == QVariant(QStringLiteral("control")))
        m_mode->setCurrentIndex(0);
    if (!m_busy && accessUnlocked()) {
        if (selectedIdentity().isEmpty())
            m_status->setText(m_machine->count() == 0 ? tr("No paired computers are available. Add a computer in Environments.") : tr("Choose a paired computer."));
        else if (!digest(selectedIdentity().value("fingerprint")))
            m_status->setText(tr("Pair this computer again to enable verified desktop access."));
        else if (catalog.isEmpty() || !catalog.value("view").toBool())
            m_status->setText(tr("This computer is offline or its native desktop provider is not ready. Enable desktop access on that computer, then refresh."));
        else if (!providerFor(catalog))
            m_status->setText(tr("Install a compatible native desktop provider for this viewer in Packages."));
        else
            m_status->setText(tr("Ready. Connect opens the selected computer and access mode."));
    }
    updateActions();
}

void RemoteDesktopWidget::updateActions()
{
    if ((!m_connected || m_closing) && m_fullscreenWindow)
        exitFullscreen();
    const auto catalog = selectedCatalog();
    const bool ready = !catalog.isEmpty() && catalog.value("view").toBool() && providerFor(catalog);
    m_connect->setEnabled(accessUnlocked() && ready && !m_busy && !m_command.busy() && !m_connected && !m_closing);
    m_disconnect->setEnabled(m_connected && !m_closing);
    m_connect->setVisible(!m_connected && !m_closing);
    m_disconnect->setVisible(m_connected || m_closing);
    m_status->setVisible(!m_connected);
    m_refresh->setEnabled(accessUnlocked() && !m_busy && !m_command.busy() && !m_closing);
    m_machine->setEnabled(!m_busy && !m_connected && !m_closing);
    m_mode->setEnabled(ready && !m_busy && !m_connected && !m_closing);
    m_unlock->setVisible(!accessUnlocked());
    m_unlock->setEnabled(m_access && !m_access->busy());
    m_packages->setEnabled(!m_busy && !m_connected && !m_closing);
    m_packages->setVisible(!m_connected && !m_closing && !m_compactPresentation);
    m_retained->setVisible(!m_connected && !m_closing && !m_compactPresentation);
    m_refresh->setVisible(!m_connected && !m_closing);
    m_settings->setEnabled(!m_closing);
    m_fullscreen->setEnabled(m_connected && !m_closing);
    m_fullscreen->setVisible(m_connected && !m_closing);
    if (m_connected && !m_closing && isVisible()) {
        m_decoderTimer->start();
    } else {
        m_decoderTimer->stop();
        m_decoderStatus->hide();
    }
}

void RemoteDesktopWidget::query(const QString &command, quint64 generation, std::function<void(const QJsonDocument &)> completion)
{
    m_command.run(m_cli, {"remote", command, "--auth", "never"}, [this, generation, completion](bool success, const QByteArray &data) {
        if (generation != m_generation || !isVisible())
            return;
        const auto failed = [this] {
            m_busy = false;
            m_catalogs = {};
            m_status->setText(tr("Desktop discovery failed. Check remote access and refresh."));
            updateActions();
        };
        if (!success || data.size() > 1024 * 1024) {
            failed();
            return;
        }
        QJsonParseError error;
        const auto document = QJsonDocument::fromJson(data, &error);
        if (error.error != QJsonParseError::NoError) {
            failed();
            return;
        }
        completion(document);
    }, 10000);
}

void RemoteDesktopWidget::refresh()
{
    refreshMetadata(false);
}

void RemoteDesktopWidget::refreshMetadata(bool connecting)
{
    if (!isVisible() || m_busy || m_closing || m_command.busy() || !accessUnlocked())
        return;
    QString error;
    m_cli = DockpipeChoices::operatorDockpipeBinary(&error);
    if (m_cli.isEmpty()) {
        m_status->setText(error);
        return;
    }
    const auto identity = selectedIdentity();
    const QString mode = m_mode->currentData().toString();
    m_busy = true;
    const auto generation = ++m_generation;
    updateActions();
    query("info", generation, [this, generation, identity, mode, connecting](const QJsonDocument &info) {
        m_stateDirectory = info.object().value("state_directory").toString();
        if (m_stateDirectory.isEmpty() || !QFileInfo(m_stateDirectory).isAbsolute()) {
            m_busy = false;
            m_status->setText(tr("The operator CLI does not provide a valid remote state directory. Update the CLI and refresh."));
            updateActions();
            return;
        }
        query("nodes", generation, [this, generation, identity, mode, connecting](const QJsonDocument &nodes) {
            if (!nodes.isArray() || nodes.array().size() > 64) {
                m_busy = false;
                m_status->setText(tr("Invalid paired computer catalog."));
                updateActions();
                return;
            }
            setPairedMachines(nodes.array());
            query("desktop-catalog", generation, [this, identity, mode, connecting](const QJsonDocument &result) {
                m_busy = false;
                m_catalogs = {};
                if (!result.isArray() || result.array().size() > 64) {
                    m_status->setText(tr("Invalid desktop readiness catalog."));
                    updateActions();
                    return;
                }
                for (const auto &value : result.array()) {
                    if (!value.isObject() || !validCatalog(value.toObject())) {
                        m_status->setText(tr("Invalid desktop readiness catalog."));
                        updateActions();
                        return;
                    }
                }
                m_catalogs = result.array();
                applySelection();
                if (connecting)
                    openDisplay(identity, mode);
            });
        });
    });
}

void RemoteDesktopWidget::connectDesktop(const QJsonObject &identity, const QString &mode)
{
    if (identity.isEmpty() || mode.isEmpty() || m_connected || m_closing)
        return;
    refreshMetadata(true);
}

void RemoteDesktopWidget::openDisplay(const QJsonObject &identity, const QString &mode)
{
    const auto catalog = selectedCatalog();
    const auto *provider = providerFor(catalog);
    const bool administrator = mode == QStringLiteral("control") && catalog.value("administrator_input").toBool();
    if (!isVisible() || !accessUnlocked() || identity != selectedIdentity() || catalog.isEmpty() || !provider
        || !catalog.value("view").toBool() || (mode != "view" && mode != "control")
        || (mode == "control" && !catalog.value("input").toBool())) {
        m_status->setText(tr("The selected desktop identity or permission changed. Refresh and connect again."));
        updateActions();
        return;
    }
    PackageDisplayMeta display;
    display.id = QStringLiteral("remote-desktop");
    display.title = tr("Remote Desktop");
    display.nativePackage = provider->surface.nativeDisplay;
    display.leaseId = QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto capture = m_captureOptions;
    auto presentation = m_presentationOptions;
    restrictionsFor(catalog).apply(capture, presentation);
    const auto displays = catalog.value("displays").toArray();
    if (!displays.isEmpty()) {
        capture.remove("display_source");
        const auto source = QSettings().value(displayPreferenceKey(catalog)).toString();
        bool found = source.isEmpty();
        for (const auto &value : displays) {
            if (!source.isEmpty() && value.toObject().value("source") == QJsonValue(source)) {
                capture.insert("display_source", source);
                found = true;
                break;
            }
        }
        if (!found) {
            m_status->setText(tr("The saved display is no longer available. Choose a display in Settings."));
            updateActions();
            return;
        }
    }
    if (mode != "control" || administrator) {
        capture.remove("clipboard_text");
        capture.remove("clipboard_images");
        capture.remove("clipboard_files");
        presentation.remove("drop_paste_modifier");
    }
    display.options = {{"schema", "dockpipe.remote.desktop-display/v1"}, {"cli", m_cli}, {"state_directory", m_stateDirectory},
        {"catalog", catalog}, {"mode", mode}, {"capture", capture}, {"display", presentation},
        {"decoder", m_decoderMode}};
    if (administrator)
        display.options.insert("schema", QStringLiteral("dockpipe.remote.desktop-administrator-display/v1"));
    m_preferencesNotice->hide();
    m_displayCatalog = catalog;
    m_connected = true;
    m_status->setText(administrator
        ? tr("Opening the selected desktop with administrator control…")
        : tr("Opening the selected desktop…"));
    updateActions();
    m_display->setDisplay(display);
}

void RemoteDesktopWidget::disconnectDesktop()
{
    if (!m_connected)
        return;
    m_connected = false;
    m_closing = true;
    m_status->setText(tr("Closing the selected desktop…"));
    updateActions();
    m_display->requestClose();
}

void RemoteDesktopWidget::cancelRefresh()
{
    ++m_generation;
    m_busy = false;
    m_command.cancel();
}

void RemoteDesktopWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    m_timer->start();
    if (m_connected)
        m_decoderTimer->start();
    refresh();
}

void RemoteDesktopWidget::hideEvent(QHideEvent *event)
{
    exitFullscreen();
    m_timer->stop();
    m_decoderTimer->stop();
    cancelRefresh();
    if (!m_keepConnected->isChecked())
        disconnectDesktop();
    QWidget::hideEvent(event);
}

void RemoteDesktopWidget::showProviderSetup()
{
    QMenu menu(this);
    for (const auto &resolver : m_resolvers) {
        if (resolver.capability != "remote.desktop")
            continue;
        for (const auto &action : resolver.surface.actions) {
            if (action.id != "setup" || action.scope != "surface" || action.workflowId.isEmpty())
                continue;
            auto *entry = menu.addAction(resolver.title.isEmpty() ? resolver.name : resolver.title);
            connect(entry, &QAction::triggered, this, [this, workflow = action.workflowId, surface = resolver.surface.id, id = action.id] {
                emit providerSetupRequested(workflow, surface, id);
            });
        }
    }
    if (menu.isEmpty()) {
        emit installProviderRequested();
        return;
    }
    menu.addSeparator();
    menu.addAction(tr("Install provider…"), this, &RemoteDesktopWidget::installProviderRequested);
    menu.exec(m_packages->mapToGlobal(QPoint(0, m_packages->height())));
}
