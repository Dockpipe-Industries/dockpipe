#include "SpatialSitePage.h"
#include "SpatialSitePolicy.h"
#include <QJsonDocument>
#include <QJsonObject>
#include <QJsonArray>
#include <QRegularExpression>
#include <QTextDocumentFragment>
#include <QCryptographicHash>

namespace {
void addImage(SpatialSitePage &page, const QString &source, const QUrl &base)
{
    if (source.trimmed().isEmpty() || page.assets.size() >= 4)
        return;
    const auto url = SpatialSitePolicy::assetUrl(base.resolved(QUrl(source.trimmed())));
    if (!url.isEmpty() && !page.assets.contains(url))
        page.assets.append(url);
}
QString attribute(const QString &tag, const QString &key)
{
    const QRegularExpression pattern("(?:^|\\s)" + key + "\\s*=\\s*(?:\"([^\"]*)\"|'([^']*)'|([^\\s>]+))",
        QRegularExpression::CaseInsensitiveOption);
    const auto match = pattern.match(tag);
    for (int index = 1; index <= 3; ++index) {
        if (!match.captured(index).isEmpty())
            return QTextDocumentFragment::fromHtml(match.captured(index).left(4096)).toPlainText();
    }
    return {};
}
QStringList elements(const QString &html, const QString &name, int maximum, int length)
{
    const QRegularExpression pattern("<(?:" + name + ")\\b[^>]*>([\\s\\S]*?)</(?:" + name + ")\\s*>",
        QRegularExpression::CaseInsensitiveOption);
    auto matches = pattern.globalMatch(html);
    QStringList result;
    while (matches.hasNext() && result.size() < maximum)
        result.append(SpatialSiteContent::plain(matches.next().captured(1), length));
    return result;
}
QString articleText(const QString &html)
{
    // This bounded source is already capped at 512 KiB. Walk tags once rather
    // than using nested-element regex replacements: a gallery often contains
    // several divs inside the copyright/caption container.
    const QRegularExpression tags("<(/?)([a-z][a-z0-9:-]*)\\b([^<>]*)>",
        QRegularExpression::CaseInsensitiveOption);
    const QRegularExpression ancillaryClass("(?:^|[\\s_-])(copyright|caption|credit|credits)(?:$|[\\s_-])",
        QRegularExpression::CaseInsensitiveOption);
    const QRegularExpression hidden("(?:^|\\s)hidden(?:\\s|=|$)", QRegularExpression::CaseInsensitiveOption);
    const QSet<QString> ancillaryTags{"aside", "figure", "figcaption", "button", "svg", "canvas", "template"};
    const QSet<QString> voidTags{"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr"};
    QString filtered;
    QString suppressedTag;
    int suppressedDepth = 0;
    int cursor = 0;
    auto matches = tags.globalMatch(html);
    while (matches.hasNext()) {
        const auto match = matches.next();
        const QString tag = match.captured(2).toLower();
        const QString attrs = match.captured(3);
        const bool closing = !match.captured(1).isEmpty();
        const bool leaf = voidTags.contains(tag) || attrs.trimmed().endsWith('/');
        if (suppressedDepth == 0)
            filtered += html.mid(cursor, match.capturedStart() - cursor);
        if (suppressedDepth > 0) {
            if (tag == suppressedTag && closing)
                --suppressedDepth;
            else if (tag == suppressedTag && !leaf)
                ++suppressedDepth;
        } else if (!closing && (ancillaryTags.contains(tag) || hidden.match(attrs).hasMatch()
            || attribute(attrs, "aria-hidden").compare("true", Qt::CaseInsensitive) == 0
            || attribute(attrs, "aria-roledescription").compare("carousel", Qt::CaseInsensitive) == 0
            || ancillaryClass.match(attribute(attrs, "class")).hasMatch())) {
            if (!leaf) {
                suppressedTag = tag;
                suppressedDepth = 1;
            }
            filtered += ' ';
        } else {
            filtered += match.captured(0);
        }
        cursor = match.capturedEnd();
    }
    if (suppressedDepth == 0)
        filtered += html.mid(cursor);

    // Prefer prose to flattened headings, slide counters and navigation. Do
    // not blacklist words such as copyright: they can be the article's subject.
    QStringList paragraphs;
    QSet<QString> seen;
    for (const auto &paragraph : elements(filtered, "p", 256, 24000)) {
        if (paragraph.size() < 40 || seen.contains(paragraph))
            continue;
        seen.insert(paragraph);
        paragraphs.append(paragraph);
    }
    if (!paragraphs.isEmpty())
        return paragraphs.join(' ').left(24000);
    return SpatialSiteContent::plain(filtered, 24000);
}
void structured(SpatialSitePage &page, const QString &html)
{
    const QRegularExpression scripts("<script\\b([^>]*)>([\\s\\S]*?)</script\\s*>", QRegularExpression::CaseInsensitiveOption);
    auto matches = scripts.globalMatch(html);
    QVector<QJsonValue> pending;
    int bytes = 0;
    while (matches.hasNext()) {
        const auto match = matches.next();
        if (attribute(match.captured(1), "type") != "application/ld+json")
            continue;
        const auto json = match.captured(2).toUtf8();
        bytes += json.size();
        if (bytes > 65536)
            break;
        const auto document = QJsonDocument::fromJson(json);
        if (document.isObject())
            pending.append(document.object());
        else if (document.isArray())
            pending.append(document.array());
    }
    for (int cursor = 0; cursor < pending.size() && cursor < 128; ++cursor) {
        const auto value = pending[cursor];
        if (value.isArray()) {
            for (const auto &entry : value.toArray()) {
                if (pending.size() < 128)
                    pending.append(entry);
            }
            continue;
        }
        const auto object = value.toObject();
        const QString type = object.value("@type").toString();
        if (type == "ImageObject") {
            addImage(page, object.value("contentUrl").toString(), page.node.url);
            addImage(page, object.value("url").toString(), page.node.url);
        }
        addImage(page, object.value("image").toString(), page.node.url);
        addImage(page, object.value("thumbnailUrl").toString(), page.node.url);
        if (QStringList{"Product", "Course", "Article", "NewsArticle", "TechArticle", "Museum", "VisualArtwork", "VideoObject"}.contains(type))
            page.kind = type.toLower();
        if (object.value("articleSection").isString())
            page.section = object.value("articleSection").toString().simplified().left(80);
        if (page.summary.isEmpty())
            page.summary = object.value("description").toString().simplified().left(1200);
        if (type == "BreadcrumbList") {
            const auto list = object.value("itemListElement").toArray();
            for (int index = 0; index < qMin(8, int(list.size())); ++index) {
                const auto item = list[index].toObject();
                QString name = item.value("name").toString();
                if (name.isEmpty())
                    name = item.value("item").toObject().value("name").toString();
                if (!name.isEmpty())
                    page.breadcrumbs.append(name.left(80));
            }
        }
        for (auto it = object.begin(); it != object.end() && pending.size() < 128; ++it) {
            if (it.value().isObject() || it.value().isArray())
                pending.append(it.value());
        }
    }
}
}
QString SpatialSiteContent::plain(const QString &html, int limit)
{
    return QTextDocumentFragment::fromHtml(html).toPlainText().simplified().left(limit);
}
SpatialSitePage SpatialSiteContent::parse(const QByteArray &bytes, const QUrl &url)
{
    SpatialSitePage page;
    page.node.id = SpatialSitePolicy::identity(url.toString());
    page.node.url = url;
    page.node.role = SpatialWebNode::Role::Destination;
    page.node.action = SpatialWebNode::Action::Navigate;
    page.canonical = url;
    page.fetched = QDateTime::currentDateTimeUtc();
    QString html = QString::fromUtf8(bytes.left(512 * 1024));
    structured(page, html);
    html.remove(QRegularExpression("<!--[\\s\\S]*?-->"));
    html.remove(QRegularExpression("<(script|style)\\b[^>]*>[\\s\\S]*?</\\1\\s*>", QRegularExpression::CaseInsensitiveOption));
    page.node.title = elements(html, "title", 1, 180).value(0);
    page.headings = elements(html, "h[12]", 8, 120);
    QUrl base = url;
    const auto baseTag = QRegularExpression("<base\\b([^>]*)>", QRegularExpression::CaseInsensitiveOption).match(html);
    if (baseTag.hasMatch()) {
        const auto target = url.resolved(QUrl(attribute(baseTag.captured(1), "href")));
        if (!SpatialSitePolicy::normalize(target).isEmpty())
            base = target;
    }
    if (page.breadcrumbs.isEmpty()) {
        const auto crumb = QRegularExpression("<(nav|ol|div)\\b([^>]*)>([\\s\\S]*?)</\\1\\s*>", QRegularExpression::CaseInsensitiveOption);
        auto matches = crumb.globalMatch(html);
        while (matches.hasNext()) {
            const auto match = matches.next();
            if ((attribute(match.captured(2), "aria-label") + attribute(match.captured(2), "class")).contains("breadcrumb", Qt::CaseInsensitive)) {
                page.breadcrumbs = elements(match.captured(3), "a|span", 8, 80);
                break;
            }
        }
    }
    const QRegularExpression tags("<(meta|link|a|img)\\b([^<>]{0,8192})>", QRegularExpression::CaseInsensitiveOption);
    auto matches = tags.globalMatch(html);
    int examined = 0;
    while (matches.hasNext() && ++examined <= 2048) {
        const auto match = matches.next();
        const QString name = match.captured(1).toLower();
        const QString attrs = match.captured(2);
        const QString rel = attribute(attrs, "rel").toLower();
        if (name == "meta") {
            const QString key = (attribute(attrs, "name") + " " + attribute(attrs, "property")).toLower();
            const QString value = attribute(attrs, "content");
            if (attribute(attrs, "property").compare("og:image", Qt::CaseInsensitive) == 0
                || attribute(attrs, "name").compare("twitter:image", Qt::CaseInsensitive) == 0)
                addImage(page, value, base);
            if (key.contains("robots") || key.contains("dockpipespatial")) {
                page.restricted = page.restricted || value.contains(QRegularExpression("\\b(noindex|none|noarchive)\\b", QRegularExpression::CaseInsensitiveOption));
                page.noFollow = page.noFollow || value.contains(QRegularExpression("\\b(nofollow|none)\\b", QRegularExpression::CaseInsensitiveOption));
            }
            if (key.contains("description") && page.summary.isEmpty())
                page.summary = value.left(1200);
            if (key.contains("article:section"))
                page.section = value.left(80);
            if (key.contains("published_time"))
                page.modified = value.left(64);
        } else if (name == "link" && rel.contains("canonical")) {
            const QUrl canonical = SpatialSitePolicy::normalize(base.resolved(QUrl(attribute(attrs, "href"))));
            if (!canonical.isEmpty())
                page.canonical = canonical;
        } else if (name == "a" && page.links.size() < 64 && !rel.contains("nofollow")) {
            const QUrl target = base.resolved(QUrl(attribute(attrs, "href")));
            const int end = html.indexOf(QRegularExpression("</a\\s*>", QRegularExpression::CaseInsensitiveOption), match.capturedEnd());
            const QString title = end >= 0 ? plain(html.mid(match.capturedEnd(), qMin(2048, end - match.capturedEnd())), 180) : QString();
            const QString before = html.mid(qMax(0, int(match.capturedStart()) - 8192), qMin(8192, int(match.capturedStart()))).toLower();
            const bool nav = before.lastIndexOf("<nav") > before.lastIndexOf("</nav")
                || before.lastIndexOf("<header") > before.lastIndexOf("</header");
            if (!title.isEmpty() && !SpatialSitePolicy::normalize(target).isEmpty())
                page.links.append({target, title, rel.left(80), nav});
        } else if (name == "img" && page.assets.size() < 4) {
            const int before = page.assets.size();
            for (const QString &key : {"data-src", "data-lazy-src", "data-original", "src"}) {
                addImage(page, attribute(attrs, key), base);
                if (page.assets.size() > before)
                    break;
            }
            if (page.assets.size() == before) {
                // Choose the first declared responsive candidate without fetching
                // every resolution. The normal image byte/pixel limits still apply.
                QString sourceSet = attribute(attrs, "data-srcset");
                if (sourceSet.isEmpty())
                    sourceSet = attribute(attrs, "srcset");
                const QString candidate = sourceSet.section(',', 0, 0).simplified().section(' ', 0, 0);
                addImage(page, candidate, base);
            }
        }
    }
    html.remove(QRegularExpression("<!--[\\s\\S]*?-->", QRegularExpression::DotMatchesEverythingOption));
    html.remove(QRegularExpression("<(script|style|nav|header|footer|form|noscript)\\b[^>]*>[\\s\\S]*?</\\1\\s*>", QRegularExpression::CaseInsensitiveOption));
    const auto main = QRegularExpression("<(main|article)\\b[^>]*>([\\s\\S]*?)</\\1\\s*>", QRegularExpression::CaseInsensitiveOption).match(html);
    const QString content = articleText(main.hasMatch() ? main.captured(2) : html);
    page.characters = content.size();
    if (page.summary.isEmpty())
        page.summary = content.left(1200);
    if (page.node.title.isEmpty())
        page.node.title = page.headings.value(0, url.host());
    if (page.section.isEmpty() && page.breadcrumbs.size() >= 2)
        page.section = page.breadcrumbs[page.breadcrumbs.size() - 2];
    const QString normalized = content.toCaseFolded().simplified();
    page.contentHash = SpatialSitePolicy::identity(normalized);
    const auto words = normalized.split(QRegularExpression("\\W+"), Qt::SkipEmptyParts);
    for (int index = 0; index + 3 < words.size() && page.fingerprints.size() < 256; index += 3) {
        const auto hash = QCryptographicHash::hash((words[index] + " " + words[index + 1] + " " + words[index + 2]).toUtf8(), QCryptographicHash::Sha256);
        quint32 value = 0;
        for (int byte = 0; byte < 4; ++byte)
            value = (value << 8) | static_cast<unsigned char>(hash[byte]);
        page.fingerprints.insert(value);
    }
    page.status = SpatialSitePage::Status::Ready;
    return page;
}
void SpatialSiteContent::classify(SpatialSitePage &page, int threshold, const QString &subject)
{
    if (page.restricted) {
        // Retain a URL and policy decision for inspection, never an archived excerpt.
        page.node.title = "Restricted source";
        page.summary.clear();
        page.section.clear();
        page.kind.clear();
        page.headings.clear();
        page.breadcrumbs.clear();
        page.links.clear();
        page.assets.clear();
        page.fingerprints.clear();
        page.contentHash.clear();
        page.characters = 0;
        page.score = 0;
        page.status = SpatialSitePage::Status::Excluded;
        page.reason = "Source requests no indexing/archiving or access is restricted";
        return;
    }
    const QString utility = SpatialSitePolicy::utilityReason(page.node.url);
    const QString evidence = (page.node.title + " " + page.headings.join(' ') + " " + page.summary).toLower();
    if (page.kind.isEmpty()) {
        if (evidence.contains(QRegularExpression("\\b(product|price|shop|buy)\\b")))
            page.kind = "product";
        else if (evidence.contains(QRegularExpression("\\b(course|lecture|lesson|tutorial)\\b")))
            page.kind = "course";
        else
            page.kind = "article";
    }
    if (page.section.isEmpty() || page.section.compare("Home", Qt::CaseInsensitive) == 0) {
        const QList<QPair<QString, QString>> subjects{
            {"Wildlife & habitats", "animal|animals|wildlife|habitat|habitats|rainforest|rain forest|woodland|ecology"},
            {"Software & systems", "api|software|configuration|programming|install|reference|developer|documentation"},
            {"Science & discovery", "science|physics|biology|astronomy|chemistry|research"},
            {"History & collections", "museum|history|historical|artifact|archive|ancient|collection"},
            {"Arts & culture", "art|painting|music|culture|sculpture|literature"},
            {"Learning", "course|lesson|tutorial|lecture|education|curriculum"},
            {"Products & services", "product|price|store|shop|service|catalog"},
            {"People & organization", "about|organization|department|faculty|team|contact"}};
        for (const auto &entry : subjects) {
            if (evidence.contains(QRegularExpression("\\b(" + entry.second + ")\\b"))) {
                page.section = entry.first;
                break;
            }
        }
        if (page.section.isEmpty())
            page.section = "Explore";
    }
    page.score = 20 + qMin(40, page.characters / 60) + (page.headings.isEmpty() ? 0 : 10)
        + (page.prominent ? 15 : 0) + (page.assets.isEmpty() ? 0 : 5);
    int overlap = 0;
    for (const auto &word : subject.toLower().split(QRegularExpression("\\W+"), Qt::SkipEmptyParts)) {
        if (word.size() >= 5 && evidence.contains(word) && ++overlap >= 2)
            break;
    }
    page.score += overlap * 5;
    if (evidence.contains(QRegularExpression("\\b(cookie policy|privacy policy|terms of use)\\b"))
        && !subject.contains(QRegularExpression("law|legal|policy|research", QRegularExpression::CaseInsensitiveOption)))
        page.score -= 35;
    if (page.characters < 100)
        page.score -= 30;
    if (!utility.isEmpty())
        page.score -= 60;
    page.score = qBound(0, page.score, 100);
    if (page.override == SpatialSitePage::Override::Exclude) {
        page.status = SpatialSitePage::Status::Excluded;
        page.reason = "Excluded by user";
    } else if (page.override == SpatialSitePage::Override::Include) {
        page.status = SpatialSitePage::Status::Ready;
        page.reason = "Included by user; source access rules still apply";
    } else {
        page.status = page.score >= threshold ? SpatialSitePage::Status::Ready : SpatialSitePage::Status::Excluded;
        page.reason = !utility.isEmpty() ? utility : QString("Relevance %1/100: %2 characters, %3 headings, %4; threshold %5")
            .arg(page.score).arg(page.characters).arg(page.headings.size()).arg(page.prominent ? "primary navigation" : "content link").arg(threshold);
    }
}
bool SpatialSiteContent::duplicate(const SpatialSitePage &a, const SpatialSitePage &b)
{
    if (a.characters < 100 || b.characters < 100)
        return false;
    if (a.contentHash == b.contentHash || a.canonical == b.canonical)
        return true;
    if (a.fingerprints.size() < 8 || b.fingerprints.size() < 8)
        return false;
    int shared = 0;
    for (const auto hash : a.fingerprints)
        shared += b.fingerprints.contains(hash);
    return float(shared) / qMax(a.fingerprints.size(), b.fingerprints.size()) > 0.9f;
}
