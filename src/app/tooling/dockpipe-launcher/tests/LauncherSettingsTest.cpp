#include "LauncherSettings.h"
#include "SettingsDialog.h"
#include <QApplication>
#include <QCheckBox>
#include <QFile>
#include <QGroupBox>
#include <QJsonDocument>
#include <QJsonObject>
#include <QScrollArea>
#include <QTemporaryDir>
#include <cstdio>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Settings assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QCoreApplication::setOrganizationName("DockpipeSettingsTest");
    QCoreApplication::setApplicationName("SettingsFixture");
    LauncherSettings defaults;
    CHECK(defaults.load());
    CHECK(!defaults.cockpit.enabled && !defaults.cockpit.appControl && !defaults.cockpit.controller);
    CHECK(!defaults.cockpit.voiceCommands && defaults.cockpit.orbitalMotion);
    CHECK(defaults.save());
    QFile file(LauncherSettings::filePath());
    CHECK(file.open(QIODevice::WriteOnly));
    CHECK(file.write("{\"uiMode\":\"advanced\",\"cockpit\":{\"enabled\":\"true\"}}") > 0);
    file.close();
    CHECK(defaults.load() && !defaults.cockpit.enabled);

    SettingsDialog dialog(defaults);
    auto *group = dialog.findChild<QGroupBox *>("cockpitSettingsGroup");
    auto *options = dialog.findChild<QWidget *>("cockpitSettingsOptions");
    auto *enabled = dialog.findChild<QCheckBox *>("settingsCockpitEnabled");
    CHECK(group && options && enabled && group->isAncestorOf(options));
    CHECK(!enabled->isChecked() && !options->isEnabled());
    CHECK(options->findChildren<QCheckBox *>().size() == 6);
    dialog.show();
    QApplication::processEvents();
    auto *scroll = dialog.findChild<QScrollArea *>();
    scroll->ensureWidgetVisible(group);
    QApplication::processEvents();
    const QString capture = qEnvironmentVariable("DOCKPIPE_SETTINGS_SCREENSHOT");
    if (!capture.isEmpty())
        CHECK(dialog.grab().save(capture + "-disabled.png"));
    enabled->setChecked(true);
    CHECK(options->isEnabled());
    for (auto *option : options->findChildren<QCheckBox *>())
        option->setChecked(true);
    dialog.findChild<QCheckBox *>("settingsCockpitOrbitalMotion")->setChecked(false);
    auto updated = dialog.updatedSettings();
    CHECK(updated.cockpit.enabled && updated.cockpit.appControl && updated.cockpit.controller);
    CHECK(updated.cockpit.voiceCommands && !updated.cockpit.orbitalMotion);
    CHECK(!LauncherSettings::current().cockpit.enabled);
    if (!capture.isEmpty())
        CHECK(dialog.grab().save(capture + "-enabled.png"));
    CHECK(updated.save());
    auto loaded = LauncherSettings::current();
    CHECK(loaded.cockpit.enabled && loaded.cockpit.appControl && loaded.cockpit.controller);
    CHECK(loaded.cockpit.voiceCommands && !loaded.cockpit.orbitalMotion);
    CHECK(loaded.uiMode == "advanced");
    enabled->setChecked(false);
    CHECK(!options->isEnabled());
    updated = dialog.updatedSettings();
    CHECK(!updated.cockpit.enabled && updated.cockpit.appControl && updated.cockpit.controller);
    dialog.reject();
    CHECK(LauncherSettings::current().cockpit.enabled);
    CHECK(updated.save());
    CHECK(file.open(QIODevice::ReadOnly));
    const auto json = QJsonDocument::fromJson(file.readAll()).object();
    CHECK(!json.value("cockpit").toObject().value("enabled").toBool(true));
    CHECK(!json.contains("appControl"));
    std::puts("Cockpit defaults, nested settings, persistence, cancel and disabled preferences passed.");
    return 0;
}
