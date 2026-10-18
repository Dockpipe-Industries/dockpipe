#include "TerminalPreferences.h"
#include "TerminalSettingsDialog.h"
#include "LauncherSettings.h"

#include <QCheckBox>
#include <QComboBox>
#include <QColor>
#include <QDialogButtonBox>
#include <QFile>
#include <QJsonDocument>
#include <QLabel>
#include <QLockFile>
#include <QPalette>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSignalSpy>
#include <QSpinBox>
#include <QSlider>
#include <QTemporaryDir>
#include <QTest>

class TerminalPreferencesTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void init();
    void defaultsAndSharedPersistence();
    void rejectsInvalidValues();
    void olderSettingsKeepGlowDefaults();
    void preservesLockedAndMalformedSettings();
    void dialogPreviewCancelAndSave();
    void dialogReportsSaveFailure();

private:
    QTemporaryDir m_config;
};

void TerminalPreferencesTest::initTestCase()
{
    QVERIFY(m_config.isValid());
    qputenv("XDG_CONFIG_HOME", m_config.path().toUtf8());
    QCoreApplication::setOrganizationName("DockpipeTerminalPreferencesTest");
    QCoreApplication::setApplicationName("TerminalPreferencesFixture");
}

void TerminalPreferencesTest::init()
{
    QFile file(LauncherSettings::filePath());
    QVERIFY(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    const QByteArray initial = R"({"browser":{"javascript":false},"futureField":{"keep":true}})";
    QCOMPARE(file.write(initial), initial.size());
}

void TerminalPreferencesTest::defaultsAndSharedPersistence()
{
    auto *preferences = TerminalPreferences::instance();
    auto settings = preferences->settings();
    QCOMPARE(settings.theme, QString("dockpipe"));
    QCOMPARE(settings.fontFamily, QString("monospace"));
    QCOMPARE(settings.fontSize, 13);
    QCOMPARE(settings.scrollback, 2000);
    QCOMPARE(settings.opacity, 100);
    QCOMPARE(settings.backgroundTarget, QString("canvas"));
    QCOMPARE(settings.glowStrength, 50);
    QCOMPARE(settings.glowSpread, 2);
    QVERIFY(!settings.cursorBlink && !settings.glow && !settings.scanlines);
    QSignalSpy changed(preferences, &TerminalPreferences::changed);
    settings.theme = "green";
    settings.fontSize = 28;
    settings.scrollback = 10000;
    settings.opacity = 30;
    settings.backgroundTarget = "canvas";
    settings.cursorStyle = "bar";
    settings.scanlines = true;
    settings.glowStrength = 100;
    settings.glowSpread = 8;
    QVERIFY(preferences->save(settings));
    QCOMPARE(changed.size(), 1);
    auto launcher = LauncherSettings::current();
    launcher.sidebarVisible = false;
    QVERIFY(launcher.save());
    const auto restored = preferences->settings();
    QCOMPARE(restored.theme, settings.theme);
    QCOMPARE(restored.fontSize, settings.fontSize);
    QCOMPARE(restored.scrollback, settings.scrollback);
    QCOMPARE(restored.opacity, settings.opacity);
    QCOMPARE(restored.backgroundTarget, settings.backgroundTarget);
    QCOMPARE(restored.cursorStyle, settings.cursorStyle);
    QCOMPARE(restored.glowStrength, settings.glowStrength);
    QCOMPARE(restored.glowSpread, settings.glowSpread);
    QVERIFY(restored.scanlines);
    QFile file(LauncherSettings::filePath());
    QVERIFY(file.open(QIODevice::ReadOnly));
    const auto root = QJsonDocument::fromJson(file.readAll()).object();
    QVERIFY(root.value("futureField").toObject().value("keep").toBool());
    QCOMPARE(root.value("browser").toObject().value("javascript").toBool(), false);
    for (const QString &id : TerminalPreferences::themeIds()) {
        const auto theme = TerminalPreferences::theme(id);
        QVERIFY(QColor(theme.value("background").toString()).isValid());
        QVERIFY(QColor(theme.value("foreground").toString()).isValid());
        QVERIFY(QColor(theme.value("cursor").toString()).isValid());
        QVERIFY(QColor(theme.value("selectionBackground").toString()).isValid());
    }
}

void TerminalPreferencesTest::rejectsInvalidValues()
{
    auto *preferences = TerminalPreferences::instance();
    QSignalSpy changed(preferences, &TerminalPreferences::changed);
    const auto rejects = [preferences](TerminalSettings settings) { return !preferences->save(settings); };
    auto settings = TerminalSettings();
    settings.fontSize = 8;
    QVERIFY(rejects(settings));
    settings.fontSize = 29;
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.scrollback = 99;
    QVERIFY(rejects(settings));
    settings.scrollback = 10001;
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.opacity = 29;
    QVERIFY(rejects(settings));
    settings.opacity = 101;
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.backgroundTarget = "unknown";
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.fontFamily = QString(129, 'a');
    QVERIFY(rejects(settings));
    settings.fontFamily = QString("mono") + QChar::Null;
    QVERIFY(rejects(settings));
    settings.fontFamily = "   ";
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.theme = "unknown";
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.cursorStyle = "unknown";
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.glowStrength = -1;
    QVERIFY(rejects(settings));
    settings.glowStrength = 101;
    QVERIFY(rejects(settings));
    settings = TerminalSettings();
    settings.glowSpread = -1;
    QVERIFY(rejects(settings));
    settings.glowSpread = 9;
    QVERIFY(rejects(settings));
    QCOMPARE(changed.size(), 0);
    QVERIFY(LauncherSettings::saveTerminalSettings({{"opacity", 40.5}}));
    QCOMPARE(preferences->settings().opacity, 100);
    QVERIFY(LauncherSettings::saveTerminalSettings({{"cursorBlink", "true"}}));
    QVERIFY(!preferences->settings().cursorBlink);
    QVERIFY(LauncherSettings::saveTerminalSettings({{"glowStrength", 45.5}}));
    QCOMPARE(preferences->settings().glowStrength, 50);
    QVERIFY(LauncherSettings::saveTerminalSettings({{"glowSpread", "3"}}));
    QCOMPARE(preferences->settings().glowSpread, 2);
    QVERIFY(LauncherSettings::saveTerminalSettings({{"backgroundTarget", true}}));
    QCOMPARE(preferences->settings().backgroundTarget, QString("canvas"));
    settings = TerminalSettings();
    settings.glowStrength = 0;
    settings.glowSpread = 0;
    QVERIFY(preferences->save(settings));
    QCOMPARE(preferences->settings().glowStrength, 0);
    QCOMPARE(preferences->settings().glowSpread, 0);
}

void TerminalPreferencesTest::olderSettingsKeepGlowDefaults()
{
    // Existing configurations predate the two sliders; no write is needed to read them.
    QVERIFY(LauncherSettings::saveTerminalSettings({{"theme", "amber"}, {"glow", true}, {"opacity", 80}}));
    QFile file(LauncherSettings::filePath());
    QVERIFY(file.open(QIODevice::ReadOnly));
    const QByteArray before = file.readAll();
    file.close();
    auto *preferences = TerminalPreferences::instance();
    QSignalSpy changed(preferences, &TerminalPreferences::changed);
    const auto settings = preferences->settings();
    QCOMPARE(settings.theme, QString("amber"));
    QCOMPARE(settings.opacity, 80);
    QCOMPARE(settings.backgroundTarget, QString("canvas"));
    QVERIFY(settings.glow);
    QCOMPARE(settings.glowStrength, 50);
    QCOMPARE(settings.glowSpread, 2);
    QCOMPARE(changed.size(), 0);
    QVERIFY(file.open(QIODevice::ReadOnly));
    QCOMPARE(file.readAll(), before);
    file.close();
    QVERIFY(preferences->save(settings));
    QVERIFY(file.open(QIODevice::ReadOnly));
    const auto terminal = QJsonDocument::fromJson(file.readAll()).object().value("terminal").toObject();
    QCOMPARE(terminal.value("glowStrength").toInt(), 50);
    QCOMPARE(terminal.value("glowSpread").toInt(), 2);
    QCOMPARE(terminal.value("backgroundTarget").toString(), QString("canvas"));
}

void TerminalPreferencesTest::preservesLockedAndMalformedSettings()
{
    auto *preferences = TerminalPreferences::instance();
    QSignalSpy changed(preferences, &TerminalPreferences::changed);
    {
        QLockFile lock(LauncherSettings::filePath() + ".lock");
        QVERIFY(lock.tryLock());
        QVERIFY(!preferences->save(TerminalSettings()));
    }
    QFile file(LauncherSettings::filePath());
    QVERIFY(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    const QByteArray corrupt = "{broken preferences";
    QCOMPARE(file.write(corrupt), corrupt.size());
    file.close();
    QVERIFY(!preferences->save(TerminalSettings()));
    QCOMPARE(changed.size(), 0);
    QCOMPARE(preferences->settings().opacity, 100);
    QVERIFY(file.open(QIODevice::ReadOnly));
    QCOMPARE(file.readAll(), corrupt);
}

void TerminalPreferencesTest::dialogPreviewCancelAndSave()
{
    auto *preferences = TerminalPreferences::instance();
    QSignalSpy changed(preferences, &TerminalPreferences::changed);
    {
        TerminalSettingsDialog dialog;
        auto *theme = dialog.findChild<QComboBox *>("terminalSettingsTheme");
        auto *opacity = dialog.findChild<QSpinBox *>("terminalSettingsOpacity");
        auto *preview = dialog.findChild<QPlainTextEdit *>("terminalSettingsPreview");
        auto *background = dialog.findChild<QComboBox *>("terminalSettingsBackgroundTarget");
        auto *glow = dialog.findChild<QCheckBox *>("terminalSettingsGlow");
        auto *strength = dialog.findChild<QSlider *>("terminalSettingsGlowStrength");
        auto *spread = dialog.findChild<QSlider *>("terminalSettingsGlowSpread");
        QVERIFY(theme && opacity && preview && background && glow && strength && spread);
        QCOMPARE(background->currentData().toString(), QString("canvas"));
        QCOMPARE(background->count(), 3);
        background->setCurrentIndex(background->findData("desktop"));
        QCOMPARE(opacity->value(), 100);
        QVERIFY(!strength->isEnabled() && !spread->isEnabled());
        QCOMPARE(strength->value(), 50);
        QCOMPARE(spread->value(), 2);
        glow->setChecked(true);
        QVERIFY(strength->isEnabled() && spread->isEnabled());
        strength->setValue(75);
        spread->setValue(6);
        QCOMPARE(dialog.findChild<QLabel *>("terminalSettingsGlowStrengthValue")->text(), QString("75%"));
        QCOMPARE(dialog.findChild<QLabel *>("terminalSettingsGlowSpreadValue")->text(), QString("6 px"));
        glow->setChecked(false);
        QVERIFY(!strength->isEnabled() && !spread->isEnabled());
        QCOMPARE(opacity->value(), 100);
        theme->setCurrentIndex(theme->findData("paper"));
        QCOMPARE(opacity->value(), 100);
        QCOMPARE(preview->palette().color(QPalette::Base),
            QColor(TerminalPreferences::theme("paper").value("background").toString()));
        opacity->setValue(60);
        dialog.findChild<QDialogButtonBox *>("terminalSettingsButtons")->button(QDialogButtonBox::Cancel)->click();
        QCOMPARE(dialog.result(), int(QDialog::Rejected));
    }
    QCOMPARE(changed.size(), 0);
    QCOMPARE(preferences->settings().opacity, 100);
    QCOMPARE(preferences->settings().backgroundTarget, QString("canvas"));
    QCOMPARE(preferences->settings().glowStrength, 50);
    QCOMPARE(preferences->settings().glowSpread, 2);
    TerminalSettingsDialog dialog;
    dialog.findChild<QSpinBox *>("terminalSettingsOpacity")->setValue(70);
    auto *background = dialog.findChild<QComboBox *>("terminalSettingsBackgroundTarget");
    background->setCurrentIndex(background->findData("desktop"));
    dialog.findChild<QCheckBox *>("terminalSettingsGlow")->setChecked(true);
    dialog.findChild<QSlider *>("terminalSettingsGlowStrength")->setValue(80);
    dialog.findChild<QSlider *>("terminalSettingsGlowSpread")->setValue(5);
    dialog.findChild<QDialogButtonBox *>("terminalSettingsButtons")->button(QDialogButtonBox::Save)->click();
    QCOMPARE(dialog.result(), int(QDialog::Accepted));
    QCOMPARE(changed.size(), 1);
    QCOMPARE(preferences->settings().opacity, 70);
    QCOMPARE(preferences->settings().backgroundTarget, QString("desktop"));
    QVERIFY(preferences->settings().glow);
    QCOMPARE(preferences->settings().glowStrength, 80);
    QCOMPARE(preferences->settings().glowSpread, 5);
}

void TerminalPreferencesTest::dialogReportsSaveFailure()
{
    TerminalSettingsDialog dialog;
    dialog.show();
    QLockFile lock(LauncherSettings::filePath() + ".lock");
    QVERIFY(lock.tryLock());
    dialog.findChild<QDialogButtonBox *>("terminalSettingsButtons")->button(QDialogButtonBox::Save)->click();
    QCOMPARE(dialog.result(), int(QDialog::Rejected));
    QVERIFY(dialog.isVisible());
    auto *error = dialog.findChild<QLabel *>("terminalSettingsError");
    QVERIFY(error && error->isVisible() && !error->text().isEmpty());
}

QTEST_MAIN(TerminalPreferencesTest)
#include "TerminalPreferencesTest.moc"
