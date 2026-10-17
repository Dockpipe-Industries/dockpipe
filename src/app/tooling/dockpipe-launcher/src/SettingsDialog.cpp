#include "SettingsDialog.h"

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
#include <QGroupBox>
#include <QScrollArea>

namespace {

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
    resize(760, 720);

    auto *outer = new QVBoxLayout(this);
    auto *scroll = new QScrollArea;
    scroll->setWidgetResizable(true);
    scroll->setFrameShape(QFrame::NoFrame);
    auto *content = new QWidget;
    scroll->setWidget(content);
    outer->addWidget(scroll, 1);
    auto *layout = new QVBoxLayout(content);
    layout->setContentsMargins(14, 14, 14, 14);
    layout->setSpacing(10);

    auto *intro = new QLabel(
        tr("Configure launcher defaults for package sources and repo discovery. Environment variables still win, and opening a folder inside a repo continues to override the repo-root fallback automatically."));
    intro->setWordWrap(true);
    layout->addWidget(intro);

    auto *form = new QFormLayout;
    form->setFieldGrowthPolicy(QFormLayout::ExpandingFieldsGrow);

    m_repoRootOverride = new QLineEdit(m_settings.repoRootOverride);
    m_repoRootOverride->setPlaceholderText(tr("Optional fallback repo root"));
    form->addRow(tr("Repo Root Fallback"), pathRow(m_repoRootOverride, tr("Browse…"), this, SLOT(browseRepoRoot())));

    m_globalRootOverride = new QLineEdit(m_settings.globalRootOverride);
    m_globalRootOverride->setPlaceholderText(tr("Optional global Dockpipe data root"));
    form->addRow(tr("Global Root Override"), pathRow(m_globalRootOverride, tr("Browse…"), this, SLOT(browseGlobalRoot())));

    m_extraWorkflowRoots = new QPlainTextEdit(joinLines(m_settings.extraWorkflowRoots));
    m_extraWorkflowRoots->setPlaceholderText(tr("One workflow root per line"));
    m_extraWorkflowRoots->setTabChangesFocus(true);
    m_extraWorkflowRoots->setMaximumHeight(80);
    form->addRow(tr("Extra Workflow Roots"), m_extraWorkflowRoots);

    m_packageRemotes = new QPlainTextEdit(joinLines(m_settings.packageRemotes));
    m_packageRemotes->setPlaceholderText(tr("One HTTPS catalog URL per line; empty disables Marketplace"));
    m_packageRemotes->setTabChangesFocus(true);
    m_packageRemotes->setMaximumHeight(80);
    form->addRow(tr("Package Remotes"), m_packageRemotes);
    auto *remotePresets = new QHBoxLayout;
    auto *production = new QPushButton(tr("Use production"));
    auto *staging = new QPushButton(tr("Use staging"));
    remotePresets->addWidget(production);
    remotePresets->addWidget(staging);
    connect(production, &QPushButton::clicked, this, [this]() {
        m_packageRemotes->setPlainText(QStringLiteral("https://packages.dockpipe.com"));
    });
    connect(staging, &QPushButton::clicked, this, [this]() {
        m_packageRemotes->setPlainText(QStringLiteral("https://packages.staging.dockpipe.com"));
    });
    form->addRow(tr("Official catalog"), remotePresets);

    layout->addLayout(form, 1);

    auto *notes = new QLabel(
        tr("Precedence: environment variables first, then the repo discovered from the opened folder, then these saved defaults."));
    notes->setWordWrap(true);
    layout->addWidget(notes);

    auto *cockpit = new QGroupBox(tr("Cockpit mode"));
    cockpit->setObjectName("cockpitSettingsGroup");
    auto *cockpitLayout = new QVBoxLayout(cockpit);
    m_cockpitEnabled = new QCheckBox(tr("Enable Cockpit mode"));
    m_cockpitEnabled->setObjectName("settingsCockpitEnabled");
    m_cockpitEnabled->setChecked(m_settings.cockpit.enabled);
    cockpitLayout->addWidget(m_cockpitEnabled);
    auto *description = new QLabel(tr("An optional bonus workspace for flying and exploring rooms. Off by default. Disabling it stops flight, app capture, controller input and voice control."));
    description->setWordWrap(true);
    cockpitLayout->addWidget(description);
    auto *options = new QWidget;
    options->setObjectName("cockpitSettingsOptions");
    auto *optionsLayout = new QVBoxLayout(options);
    optionsLayout->setContentsMargins(24, 4, 0, 4);
    const auto option = [&](const QString &label, const QString &name, bool checked, const QString &hint) {
        auto *checkbox = new QCheckBox(label);
        checkbox->setObjectName(name);
        checkbox->setChecked(checked);
        checkbox->setToolTip(hint);
        optionsLayout->addWidget(checkbox);
        return checkbox;
    };
    m_appControl = option(tr("App control · Experimental"), "settingsCockpitAppControl", m_settings.cockpit.appControl,
        tr("Allow local app previews and native control on supported X11 desktops. Entering a room can preview up to three uniquely matched assigned apps."));
    m_orbitalMotion = option(tr("Orbital motion"), "settingsCockpitOrbitalMotion", m_settings.cockpit.orbitalMotion,
        tr("Revolve planets around the sun while the orbital view is visible."));
    m_controller = option(tr("Enable controller"), "settingsCockpitController", m_settings.cockpit.controller,
        tr("Use an SDL-compatible controller while the cockpit is visible."));
    m_voiceCommands = option(tr("Enable local voice commands"), "settingsCockpitVoice", m_settings.cockpit.voiceCommands,
        tr("Enable hold-to-talk in the cockpit. The microphone starts only while you hold the button."));
    m_showShip = option(tr("Show spaceship"), "settingsCockpitShowShip", m_settings.cockpit.showShip,
        tr("Show the cockpit canopy and instrument deck."));
    m_hardwareAcceleration = option(tr("Hardware acceleration"), "settingsCockpitHardware", m_settings.cockpit.hardwareAcceleration,
        tr("Use the GPU when supported. Fall back to software rendering if unavailable."));
    options->setEnabled(m_cockpitEnabled->isChecked());
    connect(m_cockpitEnabled, &QCheckBox::toggled, options, &QWidget::setEnabled);
    cockpitLayout->addWidget(options);
    layout->addWidget(cockpit);

    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    connect(buttons, &QDialogButtonBox::accepted, this, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    outer->addWidget(buttons);
}

LauncherSettings SettingsDialog::updatedSettings() const
{
    LauncherSettings out = m_settings;
    out.repoRootOverride = m_repoRootOverride->text().trimmed();
    out.globalRootOverride = m_globalRootOverride->text().trimmed();
    out.extraWorkflowRoots = splitLines(m_extraWorkflowRoots->toPlainText());
    out.packageRemotes = splitLines(m_packageRemotes->toPlainText());
    out.cockpit = {m_cockpitEnabled->isChecked(), m_appControl->isChecked(), m_orbitalMotion->isChecked(),
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
