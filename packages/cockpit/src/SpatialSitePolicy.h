#pragma once

#include <QHostAddress>
#include <QStringList>
#include <QUrl>
#include <QVector>

struct SpatialRobotsPolicy {
    struct Rule { QString path; bool allow = false; };
    QVector<Rule> rules;
    QVector<QUrl> sitemaps;
    int delayMs = 750;
    bool allows(const QUrl &url) const;
    static SpatialRobotsPolicy parse(const QByteArray &bytes, const QUrl &origin);
};

namespace SpatialSitePolicy {
QUrl normalize(const QUrl &url);
QUrl assetUrl(const QUrl &url);
bool sameOrigin(const QUrl &first, const QUrl &second);
bool publicAddress(const QHostAddress &address);
QString restriction(const QUrl &url, const QUrl &origin);
QString utilityReason(const QUrl &url);
QString identity(const QString &text);
struct SitemapEntry { QUrl url; QString modified; bool index = false; };
QVector<SitemapEntry> sitemap(const QByteArray &bytes, const QUrl &origin, QString &error);
}
