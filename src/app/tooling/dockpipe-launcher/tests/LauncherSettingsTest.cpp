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
    CHECK(defaults.defaultDashboardVisible);
    CHECK(!defaults.tilingEnabled);
    CHECK(!defaults.cockpit.enabled && !defaults.cockpit.appControl && !defaults.cockpit.controller);
    CHECK(!defaults.cockpit.voiceCommands && defaults.cockpit.orbitalMotion);
    CHECK(defaults.save());
    QFile file(LauncherSettings::filePath());
    CHECK(file.open(QIODevice::WriteOnly));
    CHECK(file.write("{\"uiMode\":\"advanced\",\"cockpit\":{\"enabled\":\"true\",\"voiceCommands\":true}}") > 0);
    file.close();
    CHECK(defaults.load() && !defaults.cockpit.enabled);
    CHECK(!defaults.cockpit.voiceCommands);
    CHECK(!defaults.tilingEnabled);
    CHECK(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    CHECK(file.write("{\"uiMode\":\"advanced\",\"experimental\":{\"tiling\":\"true\",\"future\":42}}") > 0);
    file.close();
    CHECK(defaults.load() && !defaults.tilingEnabled);

    SettingsDialog dialog(defaults);
    auto *showDashboard = dialog.findChild<QCheckBox *>("settingsDefaultDashboardVisible");
    CHECK(showDashboard && showDashboard->isChecked());
    showDashboard->setChecked(false);
    CHECK(!dialog.updatedSettings().defaultDashboardVisible);
    CHECK(LauncherSettings::current().defaultDashboardVisible);
    auto *tiling = dialog.findChild<QCheckBox *>("settingsTilingEnabled");
    CHECK(tiling && !tiling->isChecked() && tiling->text().contains("Experimental"));
    tiling->setChecked(true);
    CHECK(dialog.updatedSettings().tilingEnabled);
    CHECK(!LauncherSettings::current().tilingEnabled);
    auto *group = dialog.findChild<QGroupBox *>("cockpitSettingsGroup");
    auto *options = dialog.findChild<QWidget *>("cockpitSettingsOptions");
    auto *enabled = dialog.findChild<QCheckBox *>("settingsCockpitEnabled");
    CHECK(group && options && enabled && group->isAncestorOf(options));
    CHECK(group->title().contains("Experimental") && enabled->text().contains("Experimental"));
    auto *experimental = dialog.findChild<QGroupBox *>("cockpitExperimentalSettingsGroup");
    auto *voice = dialog.findChild<QCheckBox *>("settingsCockpitVoice");
    CHECK(experimental && voice && options->isAncestorOf(experimental));
    CHECK(experimental->isAncestorOf(voice) && !voice->isChecked());
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
    CHECK(!loaded.defaultDashboardVisible);
    CHECK(loaded.tilingEnabled);
    CHECK(loaded.cockpit.enabled && loaded.cockpit.appControl && loaded.cockpit.controller);
    CHECK(loaded.cockpit.voiceCommands && !loaded.cockpit.orbitalMotion);
    CHECK(loaded.uiMode == "advanced");
    enabled->setChecked(false);
    CHECK(!options->isEnabled());
    updated = dialog.updatedSettings();
    CHECK(!updated.cockpit.enabled && updated.cockpit.appControl && updated.cockpit.controller);
    tiling->setChecked(false);
    updated = dialog.updatedSettings();
    dialog.reject();
    CHECK(LauncherSettings::current().tilingEnabled);
    CHECK(LauncherSettings::current().cockpit.enabled);
    CHECK(updated.save());
    CHECK(file.open(QIODevice::ReadOnly));
    const auto json = QJsonDocument::fromJson(file.readAll()).object();
    CHECK(!json.value("cockpit").toObject().value("enabled").toBool(true));
    CHECK(!json.value("cockpit").toObject().contains("voiceCommands"));
    CHECK(json.value("cockpit").toObject().value("experimental").toObject().value("voiceCommands").toBool());
    CHECK(!json.contains("appControl"));
    CHECK(!json.value("experimental").toObject().value("tiling").toBool(true));
    CHECK(json.value("experimental").toObject().value("future").toInt() == 42);
    file.close();
    bool completed = true;
    bool suppressed = true;
    CHECK(LauncherSettings::loadDashboardTourState(completed, suppressed));
    CHECK(!completed && !suppressed);
    CHECK(LauncherSettings::saveDashboardTourState(false, true));
    CHECK(updated.save());
    CHECK(LauncherSettings::loadDashboardTourState(completed, suppressed));
    CHECK(!completed && suppressed);
    CHECK(LauncherSettings::saveDashboardTourState(true, false));
    CHECK(LauncherSettings::saveDashboardTourState(false, false));
    CHECK(LauncherSettings::loadDashboardTourState(completed, suppressed));
    CHECK(completed && !suppressed);
    CHECK(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    const QByteArray invalid = "{\"dashboardTour\":{\"completed\":\"true\"},\"unrelated\":42}";
    CHECK(file.write(invalid) == invalid.size());
    file.close();
    CHECK(!LauncherSettings::loadDashboardTourState(completed, suppressed));
    CHECK(!LauncherSettings::saveDashboardTourState(true, true));
    CHECK(file.open(QIODevice::ReadOnly));
    CHECK(file.readAll() == invalid);
    file.close();
    CHECK(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    CHECK(file.write("{\"dashboardTour\":{\"future\":42},\"unrelated\":17}") > 0);
    file.close();
    CHECK(LauncherSettings::saveDashboardTourState(true, true));
    CHECK(file.open(QIODevice::ReadOnly));
    const auto preserved = QJsonDocument::fromJson(file.readAll()).object();
    CHECK(preserved.value("unrelated").toInt() == 17);
    CHECK(preserved.value("dashboardTour").toObject().value("future").toInt() == 42);
    std::puts("Dashboard tour persistence, merge and malformed-setting protection passed.");
    std::puts("Cockpit defaults, nested settings, persistence, cancel and disabled preferences passed.");
    return 0;
}
