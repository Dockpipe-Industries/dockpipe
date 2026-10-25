#include "BrowserPreferences.h"
#include "LauncherSettings.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLockFile>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <cstdio>

#define CHECK(value) \
    do { \
        if (!(value)) { \
            std::fprintf(stderr, "Browser preferences check at %d: %s\n", __LINE__, #value); \
            return 1; \
        } \
    } while (false)

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    QTemporaryDir config;
    CHECK(config.isValid());
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QCoreApplication::setOrganizationName("DockpipeBrowserPreferencesTest");
    QCoreApplication::setApplicationName("BrowserPreferencesFixture");
    auto *preferences = BrowserPreferences::instance();
    QSignalSpy changed(preferences, &BrowserPreferences::changed);
    BrowserSettings settings = preferences->settings();
    CHECK(settings.askWhereToSave && settings.javascript && settings.images);
    CHECK(!settings.showFullUrl);
    CHECK(settings.webLinkDestination == "system" && settings.localLinkDestination == "dockpipe");
    CHECK(QDir::isAbsolutePath(settings.downloadDirectory));
    CHECK(BrowserPreferences::permission(settings, QUrl("https://example.com/page"), "camera") == "ask");
    CHECK(BrowserPreferences::permission(settings, QUrl("file:///tmp/page"), "camera") == "block");
    CHECK(BrowserPreferences::permission(settings, QUrl("https://example.com"), "unsupported") == "block");
    CHECK(BrowserPreferences::originKey(QUrl("https://EXAMPLE.com:443/page?q=1")) == "https://example.com");
    CHECK(BrowserPreferences::originKey(QUrl("http://localhost:8080/page")) == "http://localhost:8080");
    CHECK(BrowserPreferences::originKey(QUrl("https://user:password@example.com")).isEmpty());
    CHECK(BrowserPreferences::isLocalUrl(QUrl("http://localhost:8080")));
    CHECK(BrowserPreferences::isLocalUrl(QUrl("http://demo.localhost")));
    CHECK(BrowserPreferences::isLocalUrl(QUrl("http://127.0.0.2")));
    CHECK(BrowserPreferences::isLocalUrl(QUrl("http://[::1]")));
    CHECK(!BrowserPreferences::isLocalUrl(QUrl("file:///tmp/page")));
    CHECK(!BrowserPreferences::isLocalUrl(QUrl("http://192.168.1.2")));
    CHECK(!BrowserPreferences::isLocalUrl(QUrl("https://localhost.example.com")));

    QFile file(LauncherSettings::filePath());
    CHECK(file.open(QIODevice::WriteOnly));
    const QByteArray existing = R"({"uiMode":"advanced","futureField":{"keep":true}})";
    CHECK(file.write(existing) == existing.size());
    file.close();
    settings.showFullUrl = true;
    settings.askWhereToSave = false;
    settings.downloadDirectory = config.filePath("downloads");
    settings.permissionDefaults["camera"] = "block";
    settings.sitePermissions["https://example.com"]["camera"] = "allow";
    CHECK(preferences->save(settings));
    CHECK(changed.count() == 1);
    CHECK(preferences->settings().downloadDirectory == settings.downloadDirectory);
    CHECK(BrowserPreferences::permission(preferences->settings(), QUrl("https://example.com/page"), "camera") == "allow");
    CHECK(BrowserPreferences::permission(preferences->settings(), QUrl("https://other.example"), "camera") == "block");
    CHECK(file.open(QIODevice::ReadOnly));
    QJsonObject root = QJsonDocument::fromJson(file.readAll()).object();
    file.close();
    CHECK(root.value("uiMode") == "advanced");
    CHECK(root.value("futureField").toObject().value("keep").toBool());
    auto launcher = LauncherSettings::current();
    launcher.sidebarVisible = false;
    CHECK(launcher.save());
    CHECK(preferences->settings().showFullUrl);
    CHECK(preferences->settings().sitePermissions == settings.sitePermissions);
    CHECK(file.open(QIODevice::ReadOnly));
    root = QJsonDocument::fromJson(file.readAll()).object();
    file.close();
    CHECK(root.value("futureField").toObject().value("keep").toBool());

    BrowserSettings invalid = settings;
    invalid.downloadDirectory = "relative/downloads";
    CHECK(!preferences->save(invalid));
    invalid = settings;
    invalid.permissionDefaults["camera"] = "unknown";
    CHECK(!preferences->save(invalid));
    invalid = settings;
    invalid.sitePermissions["https://example.com/path"]["camera"] = "allow";
    CHECK(!preferences->save(invalid));
    invalid = settings;
    invalid.sitePermissions.clear();
    for (int index = 0; index < 200; ++index)
        invalid.sitePermissions[QString("https://site%1.example").arg(index)]["camera"] = "ask";
    CHECK(preferences->save(invalid));
    invalid.sitePermissions["https://overflow.example"]["camera"] = "ask";
    CHECK(!preferences->save(invalid));
    CHECK(preferences->save(settings));
    {
        QLockFile lock(LauncherSettings::filePath() + ".lock");
        CHECK(lock.tryLock());
        CHECK(!preferences->save(settings));
        CHECK(!launcher.save());
    }
    CHECK(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    const QByteArray corrupt = "{broken user preferences";
    CHECK(file.write(corrupt) == corrupt.size());
    file.close();
    CHECK(!preferences->save(settings));
    CHECK(!launcher.save());
    CHECK(file.open(QIODevice::ReadOnly));
    CHECK(file.readAll() == corrupt);
    file.close();
    CHECK(preferences->settings().permissionDefaults.value("camera") == "ask");
    std::puts("Browser preferences defaults, policies, merge preservation, bounds, lock and corruption checks passed.");
    return 0;
}
