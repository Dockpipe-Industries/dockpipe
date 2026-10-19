#pragma once

#include <QObject>
#include <QJsonObject>
#include <QStringList>

struct TerminalSettings {
    QString theme = QStringLiteral("dockpipe");
    QString fontFamily = QStringLiteral("monospace");
    int fontSize = 13;
    QString cursorStyle = QStringLiteral("block");
    bool cursorBlink = false;
    int scrollback = 2000;
    int opacity = 100;
    QString backgroundTarget = QStringLiteral("canvas");
    bool scanlines = false;
    bool glow = false;
    int glowStrength = 50;
    int glowSpread = 2;
};

class TerminalPreferences final : public QObject {
    Q_OBJECT
public:
    static TerminalPreferences *instance();
    TerminalSettings settings() const;
    bool save(const TerminalSettings &settings);
    static QStringList themeIds();
    static QJsonObject theme(const QString &id);
    static int renderingOpacity(const TerminalSettings &settings);

signals:
    void changed();

private:
    explicit TerminalPreferences(QObject *parent);
};
