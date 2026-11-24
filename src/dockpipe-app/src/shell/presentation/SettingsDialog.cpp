#include "SettingsDialog.h"
#include "WorkspacePackageManager.h"
#include "RemoteSecurityWidget.h"
#include "Theme.h"
#include "LauncherTranslations.h"
#include <QApplication>
#include <QComboBox>
#include <QMessageBox>
#ifdef Q_OS_MAC
#include "DesktopPermissionsWidget.h"
#endif
#include <QProgressBar>
#include <QSignalBlocker>

#include <QDialogButtonBox>
#include <QFileDialog>
#include <QFormLayout>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QVBoxLayout>
#include <QCheckBox>
#include <QFrame>
#include <QScrollArea>
#include <QTabWidget>

namespace {

QLabel *settingsLabel(const QString &text, const char *role)
{
    auto *label = new QLabel(text);
    label->setProperty("settingsRole", QString::fromLatin1(role));
    label->setWordWrap(true);
    return label;
}

QScrollArea *settingsPage(QVBoxLayout *&layout, const QString &title, const QString &description)
{
    auto *scroll = new QScrollArea;
    scroll->setObjectName("settingsScrollArea");
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    scroll->viewport()->setObjectName("settingsViewport");
    auto *content = new QWidget;
    content->setObjectName("settingsPage");
    scroll->setWidget(content);
    layout = new QVBoxLayout(content);
    layout->setContentsMargins(22, 22, 22, 22);
    layout->setSpacing(16);
    layout->addWidget(settingsLabel(title, "title"));
    layout->addWidget(settingsLabel(description, "description"));
    return scroll;
}

QFrame *settingsCard(QVBoxLayout *&layout, const QString &title)
{
    auto *card = new QFrame;
    card->setProperty("settingsCard", true);
    layout = new QVBoxLayout(card);
    layout->setContentsMargins(18, 16, 18, 16);
    layout->setSpacing(12);
    layout->addWidget(settingsLabel(title, "sectionTitle"));
    return card;
}

QWidget *pathRow(QLineEdit *edit, const QString &buttonText, QObject *receiver, const char *slot)
{
    auto *row = new QWidget;
    auto *layout = new QHBoxLayout(row);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(8);
    auto *button = new QPushButton(buttonText);
    QObject::connect(button, SIGNAL(clicked()), receiver, slot);
    layout->addWidget(edit, 1);
    layout->addWidget(button, 0);
    return row;
}

} // namespace

SettingsDialog::SettingsDialog(const LauncherSettings &settings, QWidget *parent)
    : QDialog(parent), m_settings(settings)
{
    setWindowTitle(tr("Settings"));
    setObjectName("settingsDialog");
    resize(800, 700);
    setMinimumSize(640, 520);

    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(20, 20, 20, 16);
    outer->setSpacing(16);
    QVBoxLayout *generalLayout = nullptr;
    auto *generalPage = settingsPage(generalLayout, tr("Appearance & workspace"),
        tr("Choose how your workspace looks and what appears in the sidebar."));
    QVBoxLayout *appearanceLayout = nullptr;
    auto *appearance = settingsCard(appearanceLayout, tr("Appearance"));
    auto *themeRow = new QFormLayout;
    themeRow->setContentsMargins(0, 0, 0, 0);
    themeRow->setHorizontalSpacing(24);
    m_themeMode = new QComboBox;
    m_themeMode->setObjectName("settingsThemeMode");
    m_themeMode->addItem(tr("System"), int(LauncherThemeMode::System));
    m_themeMode->addItem(tr("Light"), int(LauncherThemeMode::Light));
    m_themeMode->addItem(tr("Dark"), int(LauncherThemeMode::Dark));
    m_themeMode->setCurrentIndex(m_themeMode->findData(int(dockpipeLauncherThemeMode())));
    themeRow->addRow(tr("Theme"), m_themeMode);
    m_language = new QComboBox;
    m_language->setObjectName("settingsLanguage");
    m_language->addItem(tr("System language"), "system");
    for (const auto &language : LauncherTranslations::languages())
        m_language->addItem(language.nativeName, language.code);
    m_language->setCurrentIndex(m_language->findData(LauncherTranslations::savedLanguage()));
    themeRow->addRow(tr("Language"), m_language);
    appearanceLayout->addLayout(themeRow);
    appearanceLayout->addWidget(settingsLabel(
        tr("Changes preview immediately. Save keeps your choice; Cancel restores it. System follows your device."), "description"));
    auto *languageNotice = settingsLabel(tr("Restart Dockpipe to apply a language change."), "description");
    languageNotice->setObjectName("settingsNotice");
    appearanceLayout->addWidget(languageNotice);
    generalLayout->addWidget(appearance);
    connect(m_themeMode, qOverload<int>(&QComboBox::currentIndexChanged), this, [this]() {
        m_themePreviewed = true;
        previewDockpipeLauncherTheme(*qApp, static_cast<LauncherThemeMode>(m_themeMode->currentData().toInt()));
    });

    QVBoxLayout *sidebarLayout = nullptr;
    auto *sidebar = settingsCard(sidebarLayout, tr("Sidebar"));
    m_defaultDashboardVisible = new QCheckBox(tr("Show Dashboard above Apps"));
    m_defaultDashboardVisible->setObjectName("settingsDefaultDashboardVisible");
    m_defaultDashboardVisible->setChecked(settings.defaultDashboardVisible);
    m_defaultDashboardVisible->setToolTip(tr("Hiding the built-in Dashboard keeps its widgets and layout."));
    sidebarLayout->addWidget(m_defaultDashboardVisible);
    sidebarLayout->addWidget(settingsLabel(tr("Your widgets and layout are kept when Dashboard is hidden."), "description"));
    generalLayout->addWidget(sidebar);

#ifdef Q_OS_MAC
    generalLayout->addWidget(new DesktopPermissionsWidget);
#endif

    generalLayout->addStretch();

    QVBoxLayout *sourcesLayout = nullptr;
    auto *sourcesPage = settingsPage(sourcesLayout, tr("Packages & paths"),
        tr("Manage package catalogs and discovery defaults for this device."));
    QVBoxLayout *pathsLayout = nullptr;
    auto *paths = settingsCard(pathsLayout, tr("Discovery paths"));
    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);
    form->setRowWrapPolicy(QFormLayout::WrapLongRows);
    form->setHorizontalSpacing(20);
    form->setVerticalSpacing(14);

    m_repoRootOverride = new QLineEdit(m_settings.repoRootOverride);
    m_repoRootOverride->setPlaceholderText(tr("Optional fallback repo root"));
    form->addRow(tr("Repository fallback"), pathRow(m_repoRootOverride, tr("Browse…"), this, SLOT(browseRepoRoot())));

    m_globalRootOverride = new QLineEdit(m_settings.globalRootOverride);
    m_globalRootOverride->setPlaceholderText(tr("Optional global Dockpipe data root"));
    form->addRow(tr("Global data root"), pathRow(m_globalRootOverride, tr("Browse…"), this, SLOT(browseGlobalRoot())));

    m_extraWorkflowRoots = new QPlainTextEdit(joinLines(m_settings.extraWorkflowRoots));
    m_extraWorkflowRoots->setPlaceholderText(tr("One workflow root per line"));
    m_extraWorkflowRoots->setTabChangesFocus(true);
    m_extraWorkflowRoots->setMaximumHeight(80);
    form->addRow(tr("Extra workflow roots"), m_extraWorkflowRoots);

    pathsLayout->addLayout(form);
    pathsLayout->addWidget(settingsLabel(
        tr("Environment variables take priority, followed by the opened repository, then these defaults."), "description"));
    sourcesLayout->addWidget(paths);

    QVBoxLayout *catalogLayout = nullptr;
    auto *catalog = settingsCard(catalogLayout, tr("Package catalogs"));
    m_packageRemotes = new QPlainTextEdit(joinLines(m_settings.packageRemotes));
    m_packageRemotes->setPlaceholderText(tr("One HTTPS catalog URL per line; empty disables Marketplace"));
    m_packageRemotes->setTabChangesFocus(true);
    m_packageRemotes->setMaximumHeight(80);
    catalogLayout->addWidget(m_packageRemotes);
    catalogLayout->addWidget(settingsLabel(tr("One HTTPS catalog URL per line. Leave empty to disable Marketplace."), "description"));
    auto *remotePresets = new QHBoxLayout;
    auto *production = new QPushButton(tr("Use production"));
    auto *staging = new QPushButton(tr("Use staging"));
    remotePresets->addWidget(production);
    remotePresets->addWidget(staging);
    remotePresets->addStretch();
    connect(production, &QPushButton::clicked, this, [this]() {
        m_packageRemotes->setPlainText(QStringLiteral("https://packages.dockpipe.com"));
    });
    connect(staging, &QPushButton::clicked, this, [this]() {
        m_packageRemotes->setPlainText(QStringLiteral("https://packages.staging.dockpipe.com"));
    });
    catalogLayout->addLayout(remotePresets);
    sourcesLayout->addWidget(catalog);
    sourcesLayout->addStretch();

    QVBoxLayout *advancedLayout = nullptr;
    auto *advancedPage = settingsPage(advancedLayout, tr("Advanced"),
        tr("Experimental workspace modes. These features are optional and off by default."));
    QVBoxLayout *tilingLayout = nullptr;
    auto *tiling = settingsCard(tilingLayout, tr("Tiling workspace"));
    tiling->setObjectName("tilingSettingsGroup");
    m_tilingEnabled = new QCheckBox(tr("Enable tiling mode"));
    m_tilingEnabled->setObjectName("settingsTilingEnabled");
    m_tilingEnabled->setChecked(m_settings.tilingEnabled);
    tilingLayout->addWidget(m_tilingEnabled);
    tilingLayout->addWidget(settingsLabel(
        tr("Arrange panels in splits and tabs. Turning this off returns to Canvas and releases desktop apps; saved layouts are kept."), "description"));
    advancedLayout->addWidget(tiling);

    QVBoxLayout *cockpitLayout = nullptr;
    auto *cockpit = settingsCard(cockpitLayout, tr("Cockpit workspace"));
    cockpit->setObjectName("cockpitSettingsGroup");
    m_cockpitEnabled = new QCheckBox(tr("Enable Cockpit mode"));
    m_cockpitEnabled->setObjectName("settingsCockpitEnabled");
    m_cockpitEnabled->setChecked(m_settings.cockpit.enabled);
    cockpitLayout->addWidget(m_cockpitEnabled);
    cockpitLayout->addWidget(settingsLabel(
        tr("Explore rooms and the spatial web. Enabling downloads the Cockpit package. Turning this off stops flight, app capture, controller input and voice control."), "description"));
    auto *options = new QWidget;
    options->setObjectName("cockpitSettingsOptions");
    auto *optionsLayout = new QVBoxLayout(options);
    optionsLayout->setContentsMargins(0, 4, 0, 4);
    optionsLayout->setSpacing(12);
    const auto option = [&](const QString &label, const QString &name, bool checked, const QString &hint) {
        auto *checkbox = new QCheckBox(label);
        checkbox->setObjectName(name);
        checkbox->setChecked(checked);
        checkbox->setToolTip(hint);
        optionsLayout->addWidget(checkbox);
        return checkbox;
    };
    m_appControl = option(tr("App previews and native control"), "settingsCockpitAppControl", m_settings.cockpit.appControl,
        tr("Allow local app previews and native control on supported X11 desktops. Entering a room can preview up to three uniquely matched assigned apps."));
    m_orbitalMotion = option(tr("Orbital motion"), "settingsCockpitOrbitalMotion", m_settings.cockpit.orbitalMotion,
        tr("Revolve planets around the sun while the orbital view is visible."));
    m_controller = option(tr("Enable controller"), "settingsCockpitController", m_settings.cockpit.controller,
        tr("Use an SDL-compatible controller while the cockpit is visible."));
    m_showShip = option(tr("Show spaceship"), "settingsCockpitShowShip", m_settings.cockpit.showShip,
        tr("Show the cockpit canopy and instrument deck."));
    m_hardwareAcceleration = option(tr("Hardware acceleration"), "settingsCockpitHardware", m_settings.cockpit.hardwareAcceleration,
        tr("Use the GPU when supported. Fall back to software rendering if unavailable."));
    auto *experimental = new QWidget;
    experimental->setObjectName("cockpitExperimentalSettingsGroup");
    auto *experimentalLayout = new QVBoxLayout(experimental);
    experimentalLayout->setContentsMargins(0, 4, 0, 0);
    experimentalLayout->setSpacing(6);
    m_voiceCommands = new QCheckBox(tr("Enable local voice commands"));
    m_voiceCommands->setObjectName("settingsCockpitVoice");
    m_voiceCommands->setChecked(m_settings.cockpit.voiceCommands);
    m_voiceCommands->setToolTip(tr("Experimental and off by default. Opt in to hold-to-talk; disabling this hides voice controls and stops any active microphone session."));
    experimentalLayout->addWidget(m_voiceCommands);
    experimentalLayout->addWidget(settingsLabel(
        tr("Hold to talk. Turning this off stops the microphone and hides voice controls."), "description"));
    optionsLayout->addWidget(experimental);
    options->setEnabled(m_cockpitEnabled->isChecked());
    auto *packageStatus = new QLabel;
    packageStatus->setObjectName("cockpitPackageStatus");
    packageStatus->setWordWrap(true);
    auto *packageProgress = new QProgressBar;
    packageProgress->setRange(0, 0);
    packageProgress->hide();
    auto *retry = new QPushButton(tr("Retry download"));
    retry->setObjectName("cockpitPackageRetry");
    retry->hide();
    cockpitLayout->addWidget(packageStatus);
    cockpitLayout->addWidget(packageProgress);
    cockpitLayout->addWidget(retry);
    cockpitLayout->addWidget(options);
    advancedLayout->addWidget(cockpit);
    advancedLayout->addStretch();

    m_tabs = new QTabWidget;
    m_tabs->setObjectName(QStringLiteral("settingsTabs"));
    m_tabs->addTab(generalPage, tr("General"));
    m_tabs->addTab(sourcesPage, tr("Packages && paths"));
    m_remoteSecurity = new RemoteSecurityWidget;
    m_tabs->addTab(m_remoteSecurity, tr("Security"));
    m_tabs->addTab(advancedPage, tr("Advanced"));
    connect(m_tabs, &QTabWidget::currentChanged, this, [this](int index) {
        if (m_tabs->widget(index) == m_remoteSecurity)
            m_remoteSecurity->refresh();
    });
    outer->addWidget(m_tabs, 1);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setText(tr("Save"));
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    outer->addWidget(buttons);
    auto &packages = WorkspacePackageManager::instance();
    const auto finishInstall = [this, options, buttons, packageProgress](bool enabled) {
        m_installPending = false;
        const QSignalBlocker blocker(m_cockpitEnabled);
        m_cockpitEnabled->setChecked(enabled);
        m_cockpitEnabled->setEnabled(true);
        options->setEnabled(enabled);
        buttons->button(QDialogButtonBox::Ok)->setEnabled(true);
        packageProgress->hide();
    };
    connect(&packages, &WorkspacePackageManager::prepared, this, [this, finishInstall, packageStatus, retry]() {
        if (!m_installPending)
            return;
        finishInstall(true);
        packageStatus->setText(tr("Cockpit is installed and ready."));
        retry->hide();
    });
    connect(&packages, &WorkspacePackageManager::failed, this, [this, finishInstall, packageStatus, retry](const QString &message) {
        if (!m_installPending)
            return;
        finishInstall(false);
        packageStatus->setText(message);
        retry->show();
    });
    connect(&packages, &WorkspacePackageManager::progress, this, [this, packageStatus](const QString &message) {
        if (m_installPending)
            packageStatus->setText(message);
    });
    connect(m_cockpitEnabled, &QCheckBox::toggled, this,
        [this, options, buttons, packageProgress, packageStatus, retry](bool enabled) {
            options->setEnabled(false);
            retry->hide();
            if (!enabled) {
                packageStatus->clear();
                return;
            }
            m_installPending = true;
            m_cockpitEnabled->setEnabled(false);
            buttons->button(QDialogButtonBox::Ok)->setEnabled(false);
            packageProgress->show();
            auto requested = updatedSettings();
            WorkspacePackageManager::instance().prepare(requested);
        });
    connect(retry, &QPushButton::clicked, this, [this]() { m_cockpitEnabled->setChecked(true); });
    connect(this, &QDialog::finished, this, [this]() {
        if (m_installPending) {
            m_installPending = false;
            WorkspacePackageManager::instance().cancel();
        }
    });
}

void SettingsDialog::showSecurityPage()
{
    m_tabs->setCurrentWidget(m_remoteSecurity);
}

SettingsDialog::~SettingsDialog()
{
    if (m_themePreviewed)
        resetDockpipeLauncherThemePreview(*qApp);
    if (m_installPending)
        WorkspacePackageManager::instance().cancel();
}

void SettingsDialog::accept()
{
    const auto mode = static_cast<LauncherThemeMode>(m_themeMode->currentData().toInt());
    if (!LauncherTranslations::saveAppearance(mode, m_language->currentData().toString())) {
        QMessageBox::warning(this, tr("Could not save appearance"), tr("The appearance preferences could not be saved. Check that your application settings are writable, then try again."));
        return;
    }
    m_themePreviewed = false;
    resetDockpipeLauncherThemePreview(*qApp);
    QDialog::accept();
}

void SettingsDialog::reject()
{
    if (m_themePreviewed) {
        m_themePreviewed = false;
        resetDockpipeLauncherThemePreview(*qApp);
    }
    QDialog::reject();
}

LauncherSettings SettingsDialog::updatedSettings() const
{
    LauncherSettings out = m_settings;
    out.tilingEnabled = m_tilingEnabled->isChecked();
    out.defaultDashboardVisible = m_defaultDashboardVisible->isChecked();
    out.repoRootOverride = m_repoRootOverride->text().trimmed();
    out.globalRootOverride = m_globalRootOverride->text().trimmed();
    out.extraWorkflowRoots = splitLines(m_extraWorkflowRoots->toPlainText());
    out.packageRemotes = splitLines(m_packageRemotes->toPlainText());
    out.cockpit = {m_cockpitEnabled->isChecked() && !m_installPending, m_appControl->isChecked(), m_orbitalMotion->isChecked(),
        m_controller->isChecked(), m_showShip->isChecked(), m_voiceCommands->isChecked(),
        m_hardwareAcceleration->isChecked()};
    return out;
}

void SettingsDialog::browseRepoRoot()
{
    const QString dir = QFileDialog::getExistingDirectory(this, tr("Choose repo root"), m_repoRootOverride->text());
    if (!dir.isEmpty())
        m_repoRootOverride->setText(dir);
}

void SettingsDialog::browseGlobalRoot()
{
    const QString dir = QFileDialog::getExistingDirectory(this, tr("Choose global root"), m_globalRootOverride->text());
    if (!dir.isEmpty())
        m_globalRootOverride->setText(dir);
}

QStringList SettingsDialog::splitLines(const QString &text)
{
    QStringList out;
    for (const QString &line : text.split(QLatin1Char('\n'))) {
        const QString trimmed = line.trimmed();
        if (!trimmed.isEmpty())
            out.append(trimmed);
    }
    return out;
}

QString SettingsDialog::joinLines(const QStringList &values)
{
    return values.join(QLatin1Char('\n'));
}
