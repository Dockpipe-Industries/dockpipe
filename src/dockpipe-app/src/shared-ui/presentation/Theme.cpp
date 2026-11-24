#include "Theme.h"
#include "WindowChrome.h"

#include <QApplication>
#include <QColor>
#include <QDir>
#include <QFile>
#include <QFont>
#include <QFontDatabase>
#include <QGuiApplication>
#include <QIODevice>
#include <QPalette>
#include <QProcess>
#include <QSettings>
#include <QVariant>
#include <QString>
#include <QStyle>
#include <QTextStream>
#include <QtGlobal>
#include <cstdlib>
#include <optional>

#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
#include <QStyleHints>
#endif

namespace {

QString loadResource(const QString &path)
{
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return {};
    return QString::fromUtf8(f.readAll());
}

/// Fusion-friendly dark palette so palette(base/window/…) match pipeon-dark.qss.
QPalette fusionDarkPalette()
{
    QPalette p;
    const QColor win(48, 48, 48);
    const QColor base(40, 40, 40);
    const QColor alt(56, 56, 56);
    const QColor text(235, 235, 235);
    const QColor mid(120, 120, 120);
    const QColor highlight(42, 130, 218);

    p.setColor(QPalette::Window, win);
    p.setColor(QPalette::WindowText, text);
    p.setColor(QPalette::Base, base);
    p.setColor(QPalette::AlternateBase, alt);
    p.setColor(QPalette::Text, text);
    p.setColor(QPalette::Button, win);
    p.setColor(QPalette::ButtonText, text);
    p.setColor(QPalette::BrightText, QColor(255, 100, 100));
    p.setColor(QPalette::Link, highlight);
    p.setColor(QPalette::Highlight, highlight);
    p.setColor(QPalette::HighlightedText, Qt::white);
    p.setColor(QPalette::PlaceholderText, QColor(160, 160, 160));
    p.setColor(QPalette::ToolTipBase, alt);
    p.setColor(QPalette::ToolTipText, text);
    p.setColor(QPalette::Light, alt.lighter(120));
    p.setColor(QPalette::Midlight, QColor(80, 80, 80));
    p.setColor(QPalette::Mid, mid);
    p.setColor(QPalette::Dark, QColor(30, 30, 30));
    p.setColor(QPalette::Shadow, QColor(20, 20, 20));
    return p;
}

QPalette fusionLightPalette()
{
    QPalette palette;
    palette.setColor(QPalette::Window, QColor("#f3f6fa"));
    palette.setColor(QPalette::WindowText, QColor("#25344a"));
    palette.setColor(QPalette::Base, Qt::white);
    palette.setColor(QPalette::AlternateBase, QColor("#edf2f8"));
    palette.setColor(QPalette::Text, QColor("#25344a"));
    palette.setColor(QPalette::Button, QColor("#f7f9fc"));
    palette.setColor(QPalette::ButtonText, QColor("#25344a"));
    palette.setColor(QPalette::BrightText, QColor("#ab3544"));
    palette.setColor(QPalette::Link, QColor("#0d6fd1"));
    palette.setColor(QPalette::LinkVisited, QColor("#6c4c9c"));
    palette.setColor(QPalette::Highlight, QColor("#0d6fd1"));
    palette.setColor(QPalette::HighlightedText, Qt::white);
    palette.setColor(QPalette::PlaceholderText, QColor("#64758b"));
    palette.setColor(QPalette::ToolTipBase, Qt::white);
    palette.setColor(QPalette::ToolTipText, QColor("#25344a"));
    palette.setColor(QPalette::Light, Qt::white);
    palette.setColor(QPalette::Midlight, QColor("#e4eaf2"));
    palette.setColor(QPalette::Mid, QColor("#a1afbf"));
    palette.setColor(QPalette::Dark, QColor("#64758b"));
    palette.setColor(QPalette::Shadow, QColor("#34465d"));
    return palette;
}

std::optional<bool> tryGnomeColorScheme()
{
    QProcess proc;
    proc.start(QStringLiteral("gsettings"),
               {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"), QStringLiteral("color-scheme")});
    if (!proc.waitForFinished(400) || proc.exitCode() != 0)
        return std::nullopt;
    QString s = QString::fromUtf8(proc.readAllStandardOutput()).trimmed();
    s.remove(QLatin1Char('\''));
    if (s == QStringLiteral("prefer-dark"))
        return true;
    if (s == QStringLiteral("prefer-light"))
        return false;
    return std::nullopt;
}

std::optional<bool> tryGnomeGtkThemeDark()
{
    QProcess proc;
    proc.start(QStringLiteral("gsettings"),
               {QStringLiteral("get"), QStringLiteral("org.gnome.desktop.interface"), QStringLiteral("gtk-theme")});
    if (!proc.waitForFinished(400) || proc.exitCode() != 0)
        return std::nullopt;
    const QString t = QString::fromUtf8(proc.readAllStandardOutput()).trimmed().remove(QLatin1Char('\'')).toLower();
    if (t.contains(QStringLiteral("dark")) || t.endsWith(QStringLiteral("-dark")))
        return true;
    return std::nullopt;
}

bool gtkSettingsIniPreferDark()
{
    const QString path = QDir::homePath() + QStringLiteral("/.config/gtk-3.0/settings.ini");
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;
    QTextStream in(&f);
    bool inSettings = false;
    while (!in.atEnd()) {
        QString line = in.readLine().trimmed();
        if (line == QStringLiteral("[Settings]"))
            inSettings = true;
        else if (line.startsWith(QLatin1Char('[')))
            inSettings = false;
        if (inSettings && line.startsWith(QStringLiteral("gtk-application-prefer-dark-theme=")))
            return line.section(QLatin1Char('='), 1).trimmed() == QStringLiteral("1");
    }
    return false;
}

bool kdeGlobalsPreferDark()
{
    const QString path = QDir::homePath() + QStringLiteral("/.config/kdeglobals");
    QFile f(path);
    if (!f.open(QIODevice::ReadOnly | QIODevice::Text))
        return false;
    QTextStream in(&f);
    bool inGeneral = false;
    while (!in.atEnd()) {
        QString line = in.readLine().trimmed();
        if (line == QStringLiteral("[General]"))
            inGeneral = true;
        else if (line.startsWith(QLatin1Char('[')))
            inGeneral = false;
        if (inGeneral && line.startsWith(QStringLiteral("ColorScheme="))) {
            const QString scheme = line.section(QLatin1Char('='), 1).trimmed().toLower();
            return scheme.contains(QStringLiteral("dark"));
        }
    }
    return false;
}

bool gtkThemeEnvDark()
{
    const char *gtk = std::getenv("GTK_THEME");
    if (!gtk)
        return false;
    const QString t = QString::fromLocal8Bit(gtk).toLower();
    return t.contains(QStringLiteral("dark"));
}

bool isDarkUi(const QPalette &systemPalette)
{
#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
    switch (QGuiApplication::styleHints()->colorScheme()) {
    case Qt::ColorScheme::Dark:
        return true;
    case Qt::ColorScheme::Light:
        return false;
    default:
        break;
    }
#endif

#ifdef Q_OS_LINUX
    if (const auto g = tryGnomeColorScheme(); g.has_value())
        return *g;
    if (gtkSettingsIniPreferDark())
        return true;
    if (kdeGlobalsPreferDark())
        return true;
    if (const auto g = tryGnomeGtkThemeDark(); g.has_value() && *g)
        return true;
    if (gtkThemeEnvDark())
        return true;
#endif

    // Last resort (e.g. qt6ct / custom platform theme with a dark palette). Fusion alone often stays light at startup on Linux.
    const QColor bg = systemPalette.color(QPalette::Window);
    return bg.lightnessF() < 0.45;
}

void applyLauncherFont(QApplication &app)
{
    const QStringList preferredFamilies{
        QStringLiteral("Aptos"),
        QStringLiteral("Segoe UI Variable Text"),
        QStringLiteral("SF Pro Text"),
        QStringLiteral("Noto Sans"),
        QStringLiteral("Segoe UI"),
        QStringLiteral("Ubuntu"),
    };

    const QStringList families = QFontDatabase::families();
    QFont font = app.font();
    for (const QString &family : preferredFamilies) {
        if (!families.contains(family))
            continue;
        font.setFamily(family);
        break;
    }
    font.setPointSizeF(qMax(10.0, font.pointSizeF()));
    if (app.font() != font)
        app.setFont(font);
}

} // namespace

LauncherThemeMode dockpipeLauncherThemeMode()
{
    const QString mode = QSettings().value("appearance/theme", "system").toString();
    if (mode == "dark")
        return LauncherThemeMode::Dark;
    if (mode == "light")
        return LauncherThemeMode::Light;
    return LauncherThemeMode::System;
}

bool saveDockpipeLauncherThemeMode(LauncherThemeMode mode)
{
    QString value;
    switch (mode) {
    case LauncherThemeMode::System:
        value = "system";
        break;
    case LauncherThemeMode::Light:
        value = "light";
        break;
    case LauncherThemeMode::Dark:
        value = "dark";
        break;
    default:
        return false;
    }
    QSettings settings;
    settings.setValue("appearance/theme", value);
    settings.sync();
    return settings.status() == QSettings::NoError;
}

void previewDockpipeLauncherTheme(QApplication &app, LauncherThemeMode mode)
{
    app.setProperty("dockpipeThemePreview", int(mode));
    applyDockpipeLauncherTheme(app);
}

void resetDockpipeLauncherThemePreview(QApplication &app)
{
    app.setProperty("dockpipeThemePreview", QVariant());
    applyDockpipeLauncherTheme(app);
}

void applyDockpipeLauncherTheme(QApplication &app)
{
    installLauncherWindowChrome(app);
    if (!app.property("dockpipeSystemPalette").isValid())
        app.setProperty("dockpipeSystemPalette", QVariant::fromValue(app.palette()));
    LauncherThemeMode mode = dockpipeLauncherThemeMode();
    if (app.property("dockpipeThemePreview").isValid())
        mode = static_cast<LauncherThemeMode>(app.property("dockpipeThemePreview").toInt());
    const bool dark = mode == LauncherThemeMode::Dark
        || (mode == LauncherThemeMode::System && isDarkUi(qvariant_cast<QPalette>(app.property("dockpipeSystemPalette"))));

    applyLauncherFont(app);

    QString sheet = loadResource(QStringLiteral(":/theme/pipeon.qss"));
    sheet += loadResource(dark ? QStringLiteral(":/theme/pipeon-dark.qss")
                               : QStringLiteral(":/theme/pipeon-light.qss"));
    const bool stylesheetChanged = app.styleSheet() != sheet;
    // Unpolish the old stylesheet before changing the palette. Otherwise its
    // saved widget palettes can restore dark roles during a switch to Light.
    if (stylesheetChanged)
        app.setStyleSheet(QString());
    app.setPalette(dark ? fusionDarkPalette() : fusionLightPalette());
    if (stylesheetChanged)
        app.setStyleSheet(sheet);
}

void connectDockpipeLauncherThemeUpdates(QApplication &app)
{
#if QT_VERSION >= QT_VERSION_CHECK(6, 5, 0)
    QObject::connect(QGuiApplication::styleHints(), &QStyleHints::colorSchemeChanged, &app,
                       [&app]() { applyDockpipeLauncherTheme(app); });
#else
    Q_UNUSED(app);
#endif
}
