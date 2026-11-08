#include "LauncherTranslations.h"
#include "SourceEditor.h"
#include "Theme.h"

#include <QApplication>
#include <QLocale>
#include <QSettings>
#include <QTemporaryDir>
#include <QTest>
#include <QTranslator>
#include <QTextDocument>
#include <QTextOption>

class LauncherTranslationsTest final : public QObject {
    Q_OBJECT
private slots:
    void regionalAndScriptFallbacks();
    void catalogsAndRightToLeftStartup();
};

void LauncherTranslationsTest::regionalAndScriptFallbacks()
{
    using LauncherTranslations::resolveLanguage;
    QCOMPARE(resolveLanguage("system", {"fr-CA", "en-US"}), QString("fr"));
    QCOMPARE(resolveLanguage("system", {"sv-SE", "ja-JP"}), QString("ja"));
    QCOMPARE(resolveLanguage("system", {"zh-Hant-HK"}), QString("zh_TW"));
    QCOMPARE(resolveLanguage("system", {"zh-Hans-SG"}), QString("zh_CN"));
    QCOMPARE(resolveLanguage("system", {"pt-BR"}), QString("pt"));
    QCOMPARE(resolveLanguage("en", {"ar-SA"}), QString("en"));
    QCOMPARE(resolveLanguage("system", {"sv-SE"}), QString("en"));
    QCOMPARE(resolveLanguage("invalid", {"ar-SA"}), QString("en"));
}

void LauncherTranslationsTest::catalogsAndRightToLeftStartup()
{
    QTemporaryDir settingsDirectory;
    QVERIFY(settingsDirectory.isValid());
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settingsDirectory.path());
    QCoreApplication::setOrganizationName("DockpipeTranslationsTest");
    QCoreApplication::setApplicationName("IsolatedTranslations");
    QCOMPARE(LauncherTranslations::savedLanguage(), QString("system"));
    const auto languages = LauncherTranslations::languages();
    QCOMPARE(languages.size(), 17);
    for (const auto &language : languages) {
        QTranslator translator;
        QVERIFY2(translator.load(":/i18n/dockpipe_" + language.code + ".qm"), qPrintable(language.code));
        const QString settings = translator.translate("SettingsDialog", "Settings");
        QVERIFY2(!settings.isEmpty(), qPrintable(language.code));
        if (language.code != "en")
            QVERIFY2(settings != "Settings", qPrintable(language.code));
    }
    QVERIFY(LauncherTranslations::saveAppearance(LauncherThemeMode::Dark, "ar"));
    QCOMPARE(dockpipeLauncherThemeMode(), LauncherThemeMode::Dark);
    QVERIFY(!LauncherTranslations::saveAppearance(LauncherThemeMode::Light, "invalid"));
    QCOMPARE(LauncherTranslations::savedLanguage(), QString("ar"));
    QCOMPARE(dockpipeLauncherThemeMode(), LauncherThemeMode::Dark);
    LauncherTranslations::initialize(*qApp);
    QCOMPARE(LauncherTranslations::activeLanguage(), QString("ar"));
    QCOMPARE(qApp->layoutDirection(), Qt::RightToLeft);
    QCOMPARE(QLocale().language(), QLocale::Arabic);
    QVERIFY(QCoreApplication::translate("SettingsDialog", "Settings") != "Settings");
    SourceEditor source;
    QCOMPARE(source.layoutDirection(), Qt::LeftToRight);
    QCOMPARE(source.document()->defaultTextOption().textDirection(), Qt::LeftToRight);
}

QTEST_MAIN(LauncherTranslationsTest)
#include "LauncherTranslationsTest.moc"
