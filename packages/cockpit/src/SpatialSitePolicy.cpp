#include "SpatialSitePolicy.h"
#include <QCryptographicHash>
#include <QRegularExpression>
#include <QUrlQuery>
#include <QXmlStreamReader>
#include <algorithm>

namespace {
bool robotsMatches(QString pattern, const QString &path)
{
    const bool anchored = pattern.endsWith('$');
    if (anchored)
        pattern.chop(1);
    else
        pattern += '*';
    int cursor = 0;
    int position = 0;
    int wildcard = -1;
    int retry = 0;
    while (position < path.size()) {
        if (cursor < pattern.size() && pattern[cursor] != '*' && pattern[cursor] == path[position]) {
            ++cursor;
            ++position;
        } else if (cursor < pattern.size() && pattern[cursor] == '*') {
            wildcard = cursor++;
            retry = position;
        } else if (wildcard >= 0) {
            cursor = wildcard + 1;
            position = ++retry;
        } else {
            return false;
        }
    }
    while (cursor < pattern.size() && pattern[cursor] == '*')
        ++cursor;
    return cursor == pattern.size();
}
QString robotsOctets(const QString &text)
{
    // Encode spans between percent signs: newer Qt always escapes '%' even when it
    // appears in the exclusion set. Existing percent triplets must stay triplets.
    QByteArray encoded;
    const auto spans = text.split('%');
    for (int index = 0; index < spans.size(); ++index) {
        if (index > 0)
            encoded.append('%');
        encoded.append(QUrl::toPercentEncoding(spans[index], "/?*&=$:;,+@!()'[]"));
    }
    QByteArray normalized;
    for (int index = 0; index < encoded.size(); ++index) {
        if (encoded[index] == '%' && index + 2 < encoded.size()) {
            bool valid = false;
            const int value = encoded.mid(index + 1, 2).toInt(&valid, 16);
            if (valid) {
                const bool unreserved = (value >= 'a' && value <= 'z') || (value >= 'A' && value <= 'Z')
                    || (value >= '0' && value <= '9') || QByteArray("-._~").contains(char(value));
                if (unreserved)
                    normalized.append(char(value));
                else
                    normalized.append(encoded.mid(index, 3).toUpper());
                index += 2;
                continue;
            }
        }
        normalized.append(encoded[index]);
    }
    return QString::fromLatin1(normalized);
}
}

QString SpatialSitePolicy::identity(const QString &text)
{
    return QString::fromLatin1(QCryptographicHash::hash(text.toUtf8(), QCryptographicHash::Sha256).toHex().left(24));
}
QUrl SpatialSitePolicy::normalize(const QUrl &input)
{
    QUrl url = assetUrl(input);
    if (url.isEmpty())
        return {};
    // Keep only known content identifiers. Unknown/session/tracking parameters never enter the crawler.
    QList<QPair<QString, QString>> retained;
    for (const auto &pair : QUrlQuery(url).queryItems()) {
        if (QStringList{"id", "v", "article", "lang", "language"}.contains(pair.first.toLower()) && pair.second.size() <= 160)
            retained.append(pair);
    }
    std::sort(retained.begin(), retained.end());
    QUrlQuery query;
    for (const auto &pair : retained)
        query.addQueryItem(pair.first, pair.second);
    url.setQuery(query);
    return url;
}
QUrl SpatialSitePolicy::assetUrl(const QUrl &input)
{
    if (!input.isValid() || !input.userInfo().isEmpty() || input.toEncoded().size() > 4096
        || (input.scheme() != "http" && input.scheme() != "https") || input.host().isEmpty())
        return {};
    QUrl url = input.adjusted(QUrl::NormalizePathSegments | QUrl::RemoveFragment);
    url.setHost(url.host().toLower());
    if (url.port() == (url.scheme() == "https" ? 443 : 80))
        url.setPort(-1);
    if (url.path().isEmpty())
        url.setPath("/");
    // Image CDNs use query parameters for format, dimensions and signatures.
    // Keep those opaque; page deduplication rules would break the resource URL.
    return url;
}
bool SpatialSitePolicy::sameOrigin(const QUrl &a, const QUrl &b)
{
    return a.scheme() == b.scheme() && a.host().compare(b.host(), Qt::CaseInsensitive) == 0
        && a.port(a.scheme() == "https" ? 443 : 80) == b.port(b.scheme() == "https" ? 443 : 80);
}
bool SpatialSitePolicy::publicAddress(const QHostAddress &address)
{
    bool ipv4 = false;
    const quint32 value = address.toIPv4Address(&ipv4);
    if (ipv4) {
        // Include special-use, documentation, multicast, link-local and carrier-grade ranges.
        for (const auto &subnet : QStringList{"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "127.0.0.0/8",
            "169.254.0.0/16", "172.16.0.0/12", "192.0.0.0/24", "192.0.2.0/24", "192.88.99.0/24", "192.168.0.0/16",
            "198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/3"}) {
            if (QHostAddress(value).isInSubnet(QHostAddress::parseSubnet(subnet)))
                return false;
        }
        return true;
    }
    // Public global unicast only; exclude transition/documentation special ranges.
    if (!address.isInSubnet(QHostAddress::parseSubnet("2000::/3")))
        return false;
    for (const auto &subnet : QStringList{"2001::/23", "2001:db8::/32", "2002::/16", "3fff::/20"}) {
        if (address.isInSubnet(QHostAddress::parseSubnet(subnet)))
            return false;
    }
    return true;
}
QString SpatialSitePolicy::restriction(const QUrl &url, const QUrl &origin)
{
    if (normalize(url).isEmpty() || !sameOrigin(url, origin))
        return "Outside the HTTP(S) site origin or invalid URL";
    if (url.port(-1) != -1 && url.port() != 80 && url.port() != 443)
        return "Nonstandard network port";
    const QString host = url.host().toLower();
    if (!host.contains('.') || host.endsWith('.') || host.endsWith(".local") || host.endsWith(".localhost")
        || host.endsWith(".localdomain") || host.endsWith(".lan") || host.endsWith(".corp")
        || host.endsWith(".internal") || host.endsWith(".home") || host.endsWith(".test") || host.endsWith(".invalid"))
        return "Local or reserved host";
    const QHostAddress address(host);
    if (!address.isNull() && !publicAddress(address))
        return "Private or restricted network address";
    return {};
}
QString SpatialSitePolicy::utilityReason(const QUrl &url)
{
    const QString path = url.path().toLower();
    if (path.contains(QRegularExpression("(^|/)(login|logout|signin|signup|sign-in|register|registration|password|reset-password|cart|checkout|account|oauth|auth)(/|$|\\.)")))
        return "Authentication, account or transaction endpoint";
    if (path.contains(QRegularExpression("(^|/)(api|graphql|wp-json|tracking|analytics|redirect|ads|search|tag|tags|feed|downloads?|genindex|py-modindex)(/|$|\\.)")))
        return "Utility, API, search or repetitive listing";
    if (path.contains(QRegularExpression("/page/[0-9]+/?$")) || QUrlQuery(url).hasQueryItem("page")
        || QUrlQuery(url).hasQueryItem("offset") || QUrlQuery(url).hasQueryItem("q") || QUrlQuery(url).hasQueryItem("s"))
        return "Pagination or temporary search variant";
    if (path.contains(QRegularExpression("\\.(json|xml|js|css|png|jpe?g|gif|svg|webp|zip|exe|pdf|mp4|mp3|woff2?)/?$")))
        return "Non-HTML resource";
    return {};
}

SpatialRobotsPolicy SpatialRobotsPolicy::parse(const QByteArray &bytes, const QUrl &origin)
{
    struct Group { QStringList agents; QVector<Rule> rules; int delay = 750; };
    QVector<Group> groups;
    Group group;
    bool directives = false;
    int lines = 0;
    SpatialRobotsPolicy result;
    for (const auto &raw : QString::fromUtf8(bytes.left(512 * 1024)).split('\n')) {
        if (++lines > 8192) {
            result.rules = {{"/", false}};
            return result;
        }
        const QString line = raw.section('#', 0, 0).trimmed();
        const int colon = line.indexOf(':');
        if (colon < 0)
            continue;
        const QString key = line.left(colon).trimmed().toLower();
        const QString value = line.mid(colon + 1).trimmed();
        if (key == "sitemap") {
            const auto url = SpatialSitePolicy::normalize(origin.resolved(QUrl(value)));
            if (!url.isEmpty() && SpatialSitePolicy::sameOrigin(url, origin) && result.sitemaps.size() < 16)
                result.sitemaps.append(url);
        } else if (key == "user-agent") {
            if (directives) {
                groups.append(group);
                group = {};
                directives = false;
            }
            group.agents.append(value.toLower());
        } else if (!group.agents.isEmpty()) {
            directives = true;
            if ((key == "allow" || key == "disallow") && !value.isEmpty() && value.size() <= 2048)
                group.rules.append({value, key == "allow"});
            if (group.rules.size() > 512 || groups.size() > 512 || value.count('*') > 16 || value.size() > 2048) {
                result.rules = {{"/", false}};
                return result;
            }
            if (key == "crawl-delay") {
                bool valid = false;
                const double delay = value.toDouble(&valid);
                if (valid && delay >= 0)
                    group.delay = qBound(750, int(qMin(delay, 600.0) * 1000), 600000);
            }
        }
    }
    groups.append(group);
    bool specific = false;
    for (const auto &entry : groups)
        specific = specific || entry.agents.contains("dockpipespatial");
    for (const auto &entry : groups) {
        if (entry.agents.contains(specific ? "dockpipespatial" : "*")) {
            result.rules += entry.rules;
            if (result.rules.size() > 512) {
                result.rules = {{"/", false}};
                return result;
            }
            result.delayMs = qMax(result.delayMs, entry.delay);
        }
    }
    return result;
}
bool SpatialRobotsPolicy::allows(const QUrl &url) const
{
    QString path = url.path(QUrl::FullyEncoded);
    if (url.hasQuery())
        path += "?" + url.query(QUrl::FullyEncoded);
    path = robotsOctets(path);
    int longest = -1;
    bool allowed = true;
    for (const auto &rule : rules) {
        const QString normalized = robotsOctets(rule.path);
        const int specificity = normalized.size();
        if (robotsMatches(normalized, path) && (specificity > longest || (specificity == longest && rule.allow))) {
            longest = specificity;
            allowed = rule.allow;
        }
    }
    return allowed;
}
QVector<SpatialSitePolicy::SitemapEntry> SpatialSitePolicy::sitemap(const QByteArray &bytes, const QUrl &origin, QString &error)
{
    error.clear();
    QVector<SitemapEntry> result;
    if (bytes.size() > 512 * 1024) {
        error = "Sitemap byte budget exceeded";
        return result;
    }
    QXmlStreamReader xml(bytes);
    bool index = false;
    bool rootSeen = false;
    int depth = 0;
    SitemapEntry entry;
    while (!xml.atEnd()) {
        xml.readNext();
        if (xml.isDTD() || xml.isEntityReference()) {
            error = "Sitemap entity declarations are not supported";
            return {};
        }
        if (xml.isStartElement()) {
            ++depth;
            const QString name = xml.name().toString();
            if (!rootSeen) {
                rootSeen = true;
                if (name != u"urlset" && name != u"sitemapindex") {
                    error = "Not a sitemap document";
                    return {};
                }
                index = name == u"sitemapindex";
            }
            if (depth > 5) {
                error = "Sitemap nesting budget exceeded";
                return {};
            }
            if (name == u"loc" || name == u"lastmod") {
                const auto text = xml.readElementText(QXmlStreamReader::ErrorOnUnexpectedElement).trimmed();
                --depth;
                if (name == u"loc")
                    entry.url = normalize(QUrl(text));
                else
                    entry.modified = text.left(64);
            }
        } else if (xml.isEndElement()) {
            --depth;
            if (xml.name() == u"url" || xml.name() == u"sitemap") {
                entry.index = index;
                if (!entry.url.isEmpty() && sameOrigin(entry.url, origin))
                    result.append(entry);
                entry = {};
                if (result.size() >= 256)
                    break;
            }
        }
    }
    if (xml.hasError()) {
        error = "Malformed sitemap: " + xml.errorString();
        return {};
    }
    return result;
}
