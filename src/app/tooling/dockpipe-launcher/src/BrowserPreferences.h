#pragma once

#include "LauncherUiExport.h"

#include <QMap>
#include <QObject>
#include <QString>
#include <QUrl>

struct BrowserSettings {
    bool showFullUrl = false;
    QString webLinkDestination = QStringLiteral("system");
    QString localLinkDestination = QStringLiteral("dockpipe");
    QString downloadDirectory;
    bool askWhereToSave = true;
    bool javascript = true;
    bool images = true;
    QMap<QString, QString> permissionDefaults;
    QMap<QString, QMap<QString, QString>> sitePermissions;
};

class DOCKPIPE_LAUNCHER_UI_EXPORT BrowserPreferences final : public QObject {
    Q_OBJECT
public:
    static BrowserPreferences *instance();
    BrowserSettings settings() const;
    bool save(const BrowserSettings &settings);

    static QString originKey(const QUrl &url);
    static bool isLocalUrl(const QUrl &url);
    static QString permission(const BrowserSettings &settings, const QUrl &url, const QString &feature);

signals:
    void changed();

private:
    explicit BrowserPreferences(QObject *parent);
};
