#include "LauncherTranslations.h"
#include "Theme.h"

#include <QApplication>
#include <QLocale>
#include <QObject>
#include <QPointer>
#include <QSettings>
#include <QTranslator>

namespace {
bool supported(const QString &code)
{
    for (const auto &language : LauncherTranslations::languages()) {
        if (language.code == code)
            return true;
    }
    return false;
}

QString matchLanguage(QString code)
{
    code.replace('-', '_');
    if (supported(code))
        return code;
    const QLocale locale(code);
    if (locale.language() == QLocale::Chinese) {
        if (locale.script() == QLocale::TraditionalHanScript)
            return "zh_TW";
        return "zh_CN";
    }
    const QString language = code.section('_', 0, 0).toLower();
    if (supported(language))
        return language;
    return {};
}
}

QVector<LauncherTranslations::Language> LauncherTranslations::languages()
{
    // Native names remain recognizable when the current interface uses another language.
    return {{"en", QStringLiteral("English")}, {"es", QStringLiteral("Español")},
        {"fr", QStringLiteral("Français")}, {"de", QStringLiteral("Deutsch")},
        {"pt", QStringLiteral("Português")}, {"it", QStringLiteral("Italiano")},
        {"nl", QStringLiteral("Nederlands")}, {"pl", QStringLiteral("Polski")},
        {"ru", QStringLiteral("Русский")}, {"uk", QStringLiteral("Українська")},
        {"tr", QStringLiteral("Türkçe")}, {"ar", QStringLiteral("العربية")},
        {"hi", QStringLiteral("हिन्दी")}, {"ja", QStringLiteral("日本語")},
        {"ko", QStringLiteral("한국어")}, {"zh_CN", QStringLiteral("简体中文")},
        {"zh_TW", QStringLiteral("繁體中文")}};
}

QString LauncherTranslations::savedLanguage()
{
    const QString selection = QSettings().value("appearance/language", "system").toString();
    return selection == "system" || supported(selection) ? selection : QStringLiteral("system");
}

QString LauncherTranslations::resolveLanguage(const QString &selection, const QStringList &systemLanguages)
{
    if (selection != "system") {
        const QString match = matchLanguage(selection);
        return match.isEmpty() ? QStringLiteral("en") : match;
    }
    for (const auto &preference : systemLanguages) {
        const QString match = matchLanguage(preference);
        if (!match.isEmpty())
            return match;
    }
    return QStringLiteral("en");
}

bool LauncherTranslations::saveAppearance(LauncherThemeMode theme, const QString &language)
{
    if (language != "system" && !supported(language))
        return false;
    QString themeName;
    switch (theme) {
    case LauncherThemeMode::System:
        themeName = "system";
        break;
    case LauncherThemeMode::Light:
        themeName = "light";
        break;
    case LauncherThemeMode::Dark:
        themeName = "dark";
        break;
    default:
        return false;
    }
    QSettings settings;
    settings.setValue("appearance/theme", themeName);
    settings.setValue("appearance/language", language);
    settings.sync();
    return settings.status() == QSettings::NoError;
}

void LauncherTranslations::initialize(QApplication &application)
{
    if (application.property("dockpipeTranslationLanguage").isValid())
        return;
    QString language = resolveLanguage(savedLanguage(), QLocale::system().uiLanguages());
    QPointer<QTranslator> framework = new QTranslator(&application);
    if (framework->load(":/i18n/qtbase_" + language + ".qm"))
        application.installTranslator(framework);
    else
        delete framework;
    auto *catalog = new QTranslator(&application);
    if (catalog->load(":/i18n/dockpipe_" + language + ".qm")) {
        application.installTranslator(catalog);
    } else {
        delete catalog;
        if (framework) {
            application.removeTranslator(framework);
            delete framework;
        }
        language = "en";
        auto *fallback = new QTranslator(&application);
        if (fallback->load(":/i18n/dockpipe_en.qm"))
            application.installTranslator(fallback);
        else
            delete fallback;
    }
    // Explicit English overrides an RTL system. System mode keeps its regional formatting.
    const QLocale locale = savedLanguage() == "system"
        ? QLocale::system() : QLocale(language);
    QLocale::setDefault(locale);
    application.setLayoutDirection(QLocale(language).textDirection());
    application.setProperty("dockpipeTranslationLanguage", language);
}

QString LauncherTranslations::activeLanguage()
{
    return qApp ? qApp->property("dockpipeTranslationLanguage").toString() : QString();
}
