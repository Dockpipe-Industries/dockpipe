#include "BrowserPreferences.h"
#include "LauncherSettings.h"

#include <QCoreApplication>
#include <QDir>
#include <QHostAddress>
#include <QJsonObject>
#include <QPointer>
#include <QStandardPaths>

namespace {
const QStringList permissionFeatures = {
    QStringLiteral("camera"), QStringLiteral("microphone"),
    QStringLiteral("location"), QStringLiteral("notifications")
};
constexpr qsizetype maximumExplicitOrigins = 200;
constexpr qsizetype maximumOriginLength = 2048;

BrowserSettings defaults()
{
    BrowserSettings settings;
    settings.downloadDirectory = QStandardPaths::writableLocation(QStandardPaths::DownloadLocation);
    if (settings.downloadDirectory.isEmpty())
        settings.downloadDirectory = QDir::homePath();
    for (const QString &feature : permissionFeatures)
        settings.permissionDefaults.insert(feature, QStringLiteral("ask"));
    return settings;
}

bool validPolicies(const QMap<QString, QString> &policies)
{
    for (auto policy = policies.cbegin(); policy != policies.cend(); ++policy) {
        if (!permissionFeatures.contains(policy.key()))
            return false;
        if (policy.value() != "ask" && policy.value() != "allow" && policy.value() != "block")
            return false;
    }
    return true;
}

bool validSettings(const BrowserSettings &settings)
{
    if ((settings.webLinkDestination != "system" && settings.webLinkDestination != "dockpipe")
        || (settings.localLinkDestination != "system" && settings.localLinkDestination != "dockpipe")
        || settings.downloadDirectory.size() > 4096
        || !QDir::isAbsolutePath(settings.downloadDirectory)
        || settings.downloadDirectory.contains(QChar::Null)
        || !validPolicies(settings.permissionDefaults)
        || settings.sitePermissions.size() > maximumExplicitOrigins)
        return false;
    for (auto origin = settings.sitePermissions.cbegin(); origin != settings.sitePermissions.cend(); ++origin) {
        if (origin.key().size() > maximumOriginLength || origin.key().isEmpty()
            || BrowserPreferences::originKey(QUrl(origin.key())) != origin.key()
            || !validPolicies(origin.value()))
            return false;
    }
    return true;
}

QJsonObject policiesToJson(const QMap<QString, QString> &policies)
{
    QJsonObject object;
    for (auto policy = policies.cbegin(); policy != policies.cend(); ++policy)
        object.insert(policy.key(), policy.value());
    return object;
}

bool readPolicies(const QJsonValue &value, QMap<QString, QString> &policies)
{
    if (value.isUndefined())
        return true;
    if (!value.isObject())
        return false;
    const QJsonObject object = value.toObject();
    for (auto policy = object.begin(); policy != object.end(); ++policy) {
        if (!policy.value().isString())
            return false;
        policies.insert(policy.key(), policy.value().toString());
    }
    return validPolicies(policies);
}
}

BrowserPreferences::BrowserPreferences(QObject *parent) : QObject(parent)
{
}

BrowserPreferences *BrowserPreferences::instance()
{
    static QPointer<BrowserPreferences> preferences;
    if (!preferences)
        preferences = new BrowserPreferences(QCoreApplication::instance());
    return preferences;
}

BrowserSettings BrowserPreferences::settings() const
{
    BrowserSettings settings = defaults();
    QJsonObject object;
    if (!LauncherSettings::loadBrowserSettings(object))
        return settings;
    const auto boolean = [&object](const QString &name, bool &destination) {
        const QJsonValue value = object.value(name);
        if (!value.isUndefined() && !value.isBool())
            return false;
        if (value.isBool())
            destination = value.toBool();
        return true;
    };
    const auto string = [&object](const QString &name, QString &destination) {
        const QJsonValue value = object.value(name);
        if (!value.isUndefined() && !value.isString())
            return false;
        if (value.isString())
            destination = value.toString();
        return true;
    };
    if (!boolean("showFullUrl", settings.showFullUrl)
        || !boolean("askWhereToSave", settings.askWhereToSave)
        || !boolean("javascript", settings.javascript)
        || !boolean("images", settings.images)
        || !string("webLinkDestination", settings.webLinkDestination)
        || !string("localLinkDestination", settings.localLinkDestination)
        || !string("downloadDirectory", settings.downloadDirectory)
        || !readPolicies(object.value("permissionDefaults"), settings.permissionDefaults))
        return defaults();
    const QJsonValue origins = object.value("sitePermissions");
    if (!origins.isUndefined() && !origins.isObject())
        return defaults();
    const QJsonObject sites = origins.toObject();
    if (sites.size() > maximumExplicitOrigins)
        return defaults();
    for (auto site = sites.begin(); site != sites.end(); ++site) {
        if (!readPolicies(site.value(), settings.sitePermissions[site.key()]))
            return defaults();
    }
    return validSettings(settings) ? settings : defaults();
}

bool BrowserPreferences::save(const BrowserSettings &settings)
{
    if (!validSettings(settings))
        return false;
    QJsonObject sites;
    for (auto site = settings.sitePermissions.cbegin(); site != settings.sitePermissions.cend(); ++site)
        sites.insert(site.key(), policiesToJson(site.value()));
    const QJsonObject object = {
        {"showFullUrl", settings.showFullUrl}, {"webLinkDestination", settings.webLinkDestination},
        {"localLinkDestination", settings.localLinkDestination},
        {"downloadDirectory", QDir::cleanPath(settings.downloadDirectory)},
        {"askWhereToSave", settings.askWhereToSave}, {"javascript", settings.javascript},
        {"images", settings.images}, {"permissionDefaults", policiesToJson(settings.permissionDefaults)},
        {"sitePermissions", sites}
    };
    if (!LauncherSettings::saveBrowserSettings(object))
        return false;
    emit changed();
    return true;
}

QString BrowserPreferences::originKey(const QUrl &url)
{
    if (!url.isValid() || (url.scheme() != "http" && url.scheme() != "https")
        || url.host().isEmpty() || !url.userInfo().isEmpty())
        return {};
    QUrl origin;
    origin.setScheme(url.scheme().toLower());
    origin.setHost(url.host().toLower());
    const int defaultPort = origin.scheme() == "https" ? 443 : 80;
    if (url.port(defaultPort) != defaultPort)
        origin.setPort(url.port());
    const QString key = origin.toString(QUrl::FullyEncoded);
    return key.size() <= maximumOriginLength ? key : QString();
}

bool BrowserPreferences::isLocalUrl(const QUrl &url)
{
    if (originKey(url).isEmpty())
        return false;
    QString host = url.host().toLower();
    if (host.endsWith('.'))
        host.chop(1);
    if (host == "localhost" || host.endsWith(".localhost"))
        return true;
    const QHostAddress address(host);
    return !address.isNull() && address.isLoopback();
}

QString BrowserPreferences::permission(const BrowserSettings &settings, const QUrl &url, const QString &feature)
{
    if (!permissionFeatures.contains(feature))
        return QStringLiteral("block");
    const QString origin = originKey(url);
    if (origin.isEmpty())
        return QStringLiteral("block");
    const QString policy = settings.sitePermissions.value(origin).value(feature,
        settings.permissionDefaults.value(feature, QStringLiteral("ask")));
    if (policy != "ask" && policy != "allow" && policy != "block")
        return QStringLiteral("block");
    return policy;
}
