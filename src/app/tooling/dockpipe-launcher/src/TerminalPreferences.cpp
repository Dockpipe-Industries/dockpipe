#include "TerminalPreferences.h"
#include "LauncherSettings.h"

#include <QCoreApplication>
#include <QJsonObject>
#include <QPointer>

namespace {
bool validSettings(const TerminalSettings &settings)
{
    return TerminalPreferences::themeIds().contains(settings.theme)
        && !settings.fontFamily.trimmed().isEmpty() && settings.fontFamily.size() <= 128
        && !settings.fontFamily.contains(QChar::Null)
        && settings.fontSize >= 9 && settings.fontSize <= 28
        && (settings.cursorStyle == "block" || settings.cursorStyle == "underline" || settings.cursorStyle == "bar")
        && settings.scrollback >= 100 && settings.scrollback <= 10000
        && settings.opacity >= 30 && settings.opacity <= 100
        && (settings.backgroundTarget == "panel" || settings.backgroundTarget == "canvas"
            || settings.backgroundTarget == "desktop")
        && settings.glowStrength >= 0 && settings.glowStrength <= 100
        && settings.glowSpread >= 0 && settings.glowSpread <= 8;
}
}

TerminalPreferences::TerminalPreferences(QObject *parent) : QObject(parent) {}

TerminalPreferences *TerminalPreferences::instance()
{
    static QPointer<TerminalPreferences> preferences;
    if (!preferences)
        preferences = new TerminalPreferences(QCoreApplication::instance());
    return preferences;
}

QStringList TerminalPreferences::themeIds()
{
    return {QStringLiteral("dockpipe"), QStringLiteral("amber"), QStringLiteral("green"),
            QStringLiteral("paper"), QStringLiteral("midnight")};
}

QJsonObject TerminalPreferences::theme(const QString &id)
{
    QString background = QStringLiteral("#171c24");
    QString foreground = QStringLiteral("#e4e9ef");
    QString cursor = QStringLiteral("#a4e3d2");
    QString selection = QStringLiteral("#405c73");
    if (id == "amber") {
        background = QStringLiteral("#20180b");
        foreground = QStringLiteral("#ffbf69");
        cursor = QStringLiteral("#ffd49a");
        selection = QStringLiteral("#66421e");
    } else if (id == "green") {
        background = QStringLiteral("#0b1a12");
        foreground = QStringLiteral("#8ee6a1");
        cursor = QStringLiteral("#c0ffd0");
        selection = QStringLiteral("#275a3a");
    } else if (id == "paper") {
        background = QStringLiteral("#f4efe3");
        foreground = QStringLiteral("#292b2d");
        cursor = QStringLiteral("#225870");
        selection = QStringLiteral("#b7cccf");
    } else if (id == "midnight") {
        background = QStringLiteral("#111327");
        foreground = QStringLiteral("#cdd6f4");
        cursor = QStringLiteral("#b8a9ff");
        selection = QStringLiteral("#414166");
    }
    return {{"background", background}, {"foreground", foreground},
            {"cursor", cursor}, {"selectionBackground", selection}};
}

TerminalSettings TerminalPreferences::settings() const
{
    TerminalSettings settings;
    QJsonObject object;
    if (!LauncherSettings::loadTerminalSettings(object))
        return settings;
    const auto string = [&object](const QString &name, QString &destination) {
        const QJsonValue value = object.value(name);
        if (value.isUndefined())
            return true;
        if (!value.isString())
            return false;
        destination = value.toString();
        return true;
    };
    const auto integer = [&object](const QString &name, int &destination) {
        const QJsonValue value = object.value(name);
        if (value.isUndefined())
            return true;
        if (!value.isDouble() || value.toDouble() != value.toInt(-1))
            return false;
        destination = value.toInt(-1);
        return true;
    };
    const auto boolean = [&object](const QString &name, bool &destination) {
        const QJsonValue value = object.value(name);
        if (value.isUndefined())
            return true;
        if (!value.isBool())
            return false;
        destination = value.toBool();
        return true;
    };
    if (!string("theme", settings.theme) || !string("fontFamily", settings.fontFamily)
        || !string("cursorStyle", settings.cursorStyle) || !integer("fontSize", settings.fontSize)
        || !integer("scrollback", settings.scrollback) || !integer("opacity", settings.opacity)
        || !string("backgroundTarget", settings.backgroundTarget)
        || !boolean("cursorBlink", settings.cursorBlink) || !boolean("scanlines", settings.scanlines)
        || !boolean("glow", settings.glow) || !integer("glowStrength", settings.glowStrength)
        || !integer("glowSpread", settings.glowSpread) || !validSettings(settings))
        return TerminalSettings();
    return settings;
}

int TerminalPreferences::renderingOpacity(const TerminalSettings &settings)
{
    // Changing Qt's scene graph backend requires a new application session.
    // Keep a GPU session opaque rather than drawing terminals over other panels.
    return qEnvironmentVariable("QT_QUICK_BACKEND") == "software" ? settings.opacity : 100;
}

bool TerminalPreferences::save(const TerminalSettings &settings)
{
    if (!validSettings(settings))
        return false;
    const QJsonObject object = {
        {"theme", settings.theme}, {"fontFamily", settings.fontFamily}, {"fontSize", settings.fontSize},
        {"cursorStyle", settings.cursorStyle}, {"cursorBlink", settings.cursorBlink},
        {"scrollback", settings.scrollback}, {"opacity", settings.opacity},
        {"backgroundTarget", settings.backgroundTarget},
        {"scanlines", settings.scanlines}, {"glow", settings.glow},
        {"glowStrength", settings.glowStrength}, {"glowSpread", settings.glowSpread}
    };
    if (!LauncherSettings::saveTerminalSettings(object))
        return false;
    emit changed();
    return true;
}
