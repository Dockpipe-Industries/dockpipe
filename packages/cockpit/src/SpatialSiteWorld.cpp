#include "SpatialSiteWorld.h"
#include "SpatialSitePolicy.h"
#include <QDir>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLockFile>
#include <QSaveFile>
#include <QStandardPaths>
#include <algorithm>
#include <QCryptographicHash>
#include <cmath>
#include <QRegularExpression>

SpatialSitePage *SpatialSiteWorld::find(const QString &id)
{
    for (auto &page : pages) {
        if (page.node.id == id)
            return &page;
    }
    return nullptr;
}
const SpatialSitePage *SpatialSiteWorld::find(const QString &id) const
{
    for (const auto &page : pages) {
        if (page.node.id == id)
            return &page;
    }
    return nullptr;
}
bool SpatialSiteWorld::discover(const SpatialSiteLink &link, int depth)
{
    const auto url = SpatialSitePolicy::normalize(link.url);
    if (url.isEmpty() || !SpatialSitePolicy::sameOrigin(url, origin) || pages.size() >= maximumPages)
        return false;
    const auto id = SpatialSitePolicy::identity(url.toString());
    if (auto *existing = find(id)) {
        existing->prominent = existing->prominent || link.prominent;
        existing->depth = qMin(existing->depth, depth);
        return false;
    }
    SpatialSitePage page;
    page.node.id = id;
    page.node.role = SpatialWebNode::Role::Destination;
    page.node.action = SpatialWebNode::Action::Navigate;
    page.node.url = url;
    page.node.title = link.title.left(180);
    page.canonical = url;
    page.depth = depth;
    page.prominent = link.prominent;
    page.reason = SpatialSitePolicy::utilityReason(link.url);
    // Version switchers are repeated navigation, not primary subject sections.
    // Require both a version label and a version-only path to preserve dated archives.
    if (page.reason.isEmpty()
        && link.title.trimmed().contains(QRegularExpression("\\bv?[0-9]+\\.[0-9]+(?:\\.[0-9]+)?\\b", QRegularExpression::CaseInsensitiveOption))
        && url.path().contains(QRegularExpression("^/v?[0-9]+\\.[0-9]+(?:\\.[0-9]+)?/?$", QRegularExpression::CaseInsensitiveOption)))
        page.reason = "Alternate version navigation; include explicitly to explore this version";
    if (!page.reason.isEmpty()) {
        page.status = SpatialSitePage::Status::Excluded;
        page.restricted = page.reason.startsWith("Authentication");
    } else {
        page.reason = link.prominent ? "Discovered in primary navigation; awaiting public content" : "Discovered link; awaiting public content";
    }
    pages.append(page);
    return true;
}
void SpatialSiteWorld::organize(SpatialSitePage &page)
{
    if (page.status != SpatialSitePage::Status::Ready || !page.group.isEmpty())
        return;
    QString title = page.section.simplified().left(80);
    if (title.isEmpty())
        title = "Explore";
    QString group = SpatialSitePolicy::identity("section:" + title.toCaseFolded());
    auto found = std::find_if(sections.begin(), sections.end(), [&](const auto &section) { return section.id == group; });
    if (found == sections.end()) {
        if (sections.size() >= maximumSections - 1) {
            title = "More topics";
            group = SpatialSitePolicy::identity("section:" + title.toCaseFolded());
        }
        const bool exists = std::any_of(sections.begin(), sections.end(), [&](const auto &section) { return section.id == group; });
        if (!exists)
            sections.append({group, title, int(sections.size())});
    }
    page.group = group;
    page.slot = 0;
    for (const auto &other : pages) {
        if (other.node.id != page.node.id && other.group == group)
            page.slot = qMax(page.slot, other.slot + 1);
    }
}
void SpatialSiteWorld::update(SpatialSitePage page)
{
    auto *previous = find(page.node.id);
    if (!previous && pages.size() >= maximumPages)
        return;
    if (previous) {
        if (page.modified.isEmpty())
            page.modified = previous->modified;
        page.override = previous->override;
        page.prominent = previous->prominent;
        page.depth = previous->depth;
        page.group = previous->group;
        page.slot = previous->slot;
    }
    if (!previous) {
        page.group.clear();
        page.slot = -1;
    }
    SpatialSiteContent::classify(page, threshold, title);
    if (page.status == SpatialSitePage::Status::Ready && page.override == SpatialSitePage::Override::Automatic) {
        for (const auto &other : pages) {
            if (other.node.id != page.node.id && other.status == SpatialSitePage::Status::Ready
                && SpatialSiteContent::duplicate(page, other)) {
                page.status = SpatialSitePage::Status::Excluded;
                page.duplicateOf = other.node.id;
                page.reason = "Duplicate content/canonical of " + other.node.title;
                break;
            }
        }
    }
    organize(page);
    if (previous)
        *previous = page;
    else
        pages.append(page);
}
void SpatialSiteWorld::classifyAll()
{
    const auto snapshot = pages;
    for (auto page : snapshot) {
        if (!page.fetched.isValid() || page.status == SpatialSitePage::Status::Unavailable)
            continue;
        update(page);
    }
}
bool SpatialSiteWorld::setOverride(const QString &id, SpatialSitePage::Override value)
{
    auto *page = find(id);
    if (!page || page->restricted)
        return false;
    page->override = value;
    if (page->fetched.isValid()) {
        update(*page);
    } else {
        page->status = value == SpatialSitePage::Override::Exclude ? SpatialSitePage::Status::Excluded : SpatialSitePage::Status::Discovered;
        page->reason = value == SpatialSitePage::Override::Exclude ? "Excluded by user" : "Requested by user; awaiting public content";
    }
    return true;
}
int SpatialSiteWorld::areaPages(const QString &section) const
{
    int largest = 0;
    if (section.isEmpty())
        return qMax(1, (int(sections.size()) + 11) / 12);
    for (const auto &entry : pages) {
        if (entry.group == section)
            largest = qMax(largest, entry.slot);
    }
    return largest / 12 + 1;
}
SpatialWebDocument SpatialSiteWorld::project(const QString &section, int areaPage) const
{
    SpatialWebDocument document;
    document.url = origin;
    document.title = title.isEmpty() ? origin.host() : title;
    document.venue = "library";
    document.evidence = document.title;
    document.stableLayout = true;
    document.siteOverview = section.isEmpty();
    document.layoutIdentity = origin.toString() + ":" + section + ":" + QString::number(areaPage);
    if (section.isEmpty()) {
        for (const auto &group : sections) {
            if (group.slot / 12 != areaPage)
                continue;
            SpatialWebItem item;
            item.id = "site:" + group.id;
            item.title = group.title;
            item.description = "Enter this section of " + origin.host();
            item.category = group.title;
            item.url = origin;
            item.kind = "section";
            item.spatialSlot = group.slot % 12;
            for (const auto &page : pages) {
                if (page.group == group.id && page.status == SpatialSitePage::Status::Ready && !page.assets.isEmpty()) {
                    item.imageUrl = page.assets.first();
                    item.hasImage = true;
                    break;
                }
            }
            document.items.append(item);
            document.evidence += " " + group.title;
        }
    } else {
        for (const auto &group : sections) {
            if (group.id == section) {
                document.title += " / " + group.title;
                document.evidence += " " + group.title;
            }
        }
        for (const auto &page : pages) {
            if (page.group != section || page.slot / 12 != areaPage || page.status != SpatialSitePage::Status::Ready)
                continue;
            SpatialWebItem item;
            item.id = "source:" + page.node.id;
            item.title = page.node.title;
            item.url = page.node.url;
            item.description = page.summary.left(200);
            item.category = page.section;
            item.kind = page.kind == "product" ? "product" : "article";
            item.spatialSlot = page.slot % 12;
            item.imageUrl = page.assets.value(0);
            item.hasImage = !item.imageUrl.isEmpty();
            document.items.append(item);
            document.evidence += " " + page.node.title + " " + page.summary.left(160);

        }
    }
    document.evidence = document.evidence.left(2200);
    return document;
}
QString SpatialSiteWorld::cachePath(const QUrl &origin)
{
    return QStandardPaths::writableLocation(QStandardPaths::CacheLocation) + "/cockpit-sites/"
        + SpatialSitePolicy::identity(origin.toString()) + ".json";
}
bool SpatialSiteWorld::save(QString &error) const
{
    error.clear();
    if (!m_cacheWritable) {
        error = "Invalid cache preserved; clear the world cache before saving";
        return false;
    }
    if (origin.isEmpty() || pages.size() > maximumPages || sections.size() > maximumSections) {
        error = "Invalid site graph size";
        return false;
    }
    QJsonArray entries, groups;
    for (const auto &section : sections)
        groups.append(QJsonObject{{"id", section.id}, {"title", section.title}, {"slot", section.slot}});
    for (const auto &page : pages) {
        QJsonArray links, assets, hashes;
        for (const auto &link : page.links)
            links.append(QJsonObject{{"url", link.url.toString()}, {"title", link.title}, {"relation", link.relation}, {"prominent", link.prominent}});
        for (const auto &asset : page.assets)
            assets.append(asset.toString());
        for (auto hash : page.fingerprints)
            hashes.append(double(hash));
        entries.append(QJsonObject{{"url", page.node.url.toString()}, {"title", page.node.title}, {"canonical", page.canonical.toString()},
            {"summary", page.summary}, {"section", page.section}, {"kind", page.kind}, {"headings", QJsonArray::fromStringList(page.headings)},
            {"breadcrumbs", QJsonArray::fromStringList(page.breadcrumbs)}, {"links", links}, {"assets", assets}, {"fingerprints", hashes},
            {"hash", page.contentHash}, {"duplicate", page.duplicateOf}, {"reason", page.reason}, {"modified", page.modified},
            {"fetched", page.fetched.toString(Qt::ISODate)}, {"status", int(page.status)}, {"override", int(page.override)},
            {"depth", page.depth}, {"score", page.score}, {"characters", page.characters}, {"prominent", page.prominent},
            {"nofollow", page.noFollow}, {"restricted", page.restricted}, {"slot", page.slot}, {"group", page.group}});
    }
    QJsonObject features;
    for (auto it = areaFeatures.begin(); it != areaFeatures.end(); ++it) {
        if (features.size() >= 48 || it.value().size() != 8)
            break;
        QJsonArray scores;
        for (float score : it.value())
            scores.append(qBound(0.0f, score, 1.0f));
        features.insert(it.key(), scores);
    }
    const auto bytes = QJsonDocument(QJsonObject{{"features", features}, {"version", 1}, {"contentVersion", 2}, {"origin", origin.toString()}, {"title", title},
        {"threshold", threshold}, {"sections", groups}, {"pages", entries}}).toJson(QJsonDocument::Compact);
    if (bytes.size() > 2 * 1024 * 1024) {
        error = "Site cache exceeds 2 MiB";
        return false;
    }
    const QString path = cachePath(origin);
    QDir directory(QFileInfo(path).absolutePath());
    if (!directory.mkpath(".")) {
        error = "Cannot create site cache directory";
        return false;
    }
    QLockFile lock(path + ".lock");
    if (!lock.tryLock(0)) {
        error = "Site cache is being updated by another window";
        return false;
    }
    QFile previous(path);
    QByteArray revision;
    if (previous.exists()) {
        if (!previous.open(QIODevice::ReadOnly) || previous.size() > 2 * 1024 * 1024) {
            error = "Cannot validate existing site cache";
            return false;
        }
        revision = QCryptographicHash::hash(previous.read(2 * 1024 * 1024 + 1), QCryptographicHash::Sha256);
    }
    if (revision != m_revision) {
        error = "Site cache changed in another window; reload before saving";
        return false;
    }
    QSaveFile file(path);
    if (!file.open(QIODevice::WriteOnly) || file.write(bytes) != bytes.size() || !file.commit()) {
        error = file.errorString();
        return false;
    }
    m_revision = QCryptographicHash::hash(bytes, QCryptographicHash::Sha256);
    const auto files = directory.entryInfoList({"*.json"}, QDir::Files | QDir::NoSymLinks, QDir::Time);
    for (int index = 16; index < files.size(); ++index) {
        if (files[index].absoluteFilePath() != path)
            QFile::remove(files[index].absoluteFilePath());
    }
    return true;
}
bool SpatialSiteWorld::load(const QUrl &site, QString &error)
{
    error.clear();
    *this = {};
    origin = site;
    QFile file(cachePath(site));
    if (!file.exists())
        return true;
    m_cacheWritable = false;
    if (!QFileInfo(file).isFile() || !file.open(QIODevice::ReadOnly) || file.size() > 2 * 1024 * 1024) {
        error = "Unreadable or oversized site cache";
        return false;
    }
    const auto bytes = file.read(2 * 1024 * 1024 + 1);
    const auto document = QJsonDocument::fromJson(bytes);
    const auto root = document.object();
    const auto entries = root.value("pages").toArray();
    const auto groups = root.value("sections").toArray();
    if (!document.isObject() || !root.value("pages").isArray() || !root.value("sections").isArray() || root.value("version").toInt() != 1 || root.value("origin").toString() != site.toString()
        || entries.size() > maximumPages || groups.size() > maximumSections) {
        error = "Invalid site cache";
        return false;
    }
    SpatialSiteWorld candidate;
    candidate.origin = site;
    candidate.m_revision = QCryptographicHash::hash(bytes, QCryptographicHash::Sha256);
    candidate.title = root.value("title").toString().left(180);
    candidate.threshold = qBound(0, root.value("threshold").toInt(45), 100);
    QSet<QString> identities;
    QSet<int> sectionSlots;
    QSet<QString> pageSlots;
    for (const auto &value : groups) {
        const auto group = value.toObject();
        const QString id = group.value("id").toString();
        const int slot = group.value("slot").toInt(-1);
        if (id.size() != 24 || identities.contains(id) || slot < 0 || slot >= maximumSections || sectionSlots.contains(slot)) {
            error = "Invalid cached section";
            return false;
        }
        identities.insert(id);
        sectionSlots.insert(slot);
        candidate.sections.append({id, group.value("title").toString().left(80), slot});
    }
    for (const auto &value : entries) {
        const auto entry = value.toObject();
        SpatialSitePage page;
        page.node.url = SpatialSitePolicy::normalize(QUrl(entry.value("url").toString()));
        if (page.node.url.isEmpty() || !SpatialSitePolicy::sameOrigin(site, page.node.url)) {
            error = "Invalid cached source URL";
            return false;
        }
        page.node.id = SpatialSitePolicy::identity(page.node.url.toString());
        page.node.role = SpatialWebNode::Role::Destination;
        page.node.action = SpatialWebNode::Action::Navigate;
        if (candidate.find(page.node.id)) {
            error = "Duplicate cached source";
            return false;
        }
        page.node.title = entry.value("title").toString().left(180);
        page.canonical = SpatialSitePolicy::normalize(QUrl(entry.value("canonical").toString()));
        page.summary = entry.value("summary").toString().left(1200);
        page.section = entry.value("section").toString().left(80);
        page.kind = entry.value("kind").toString().left(40);
        for (const auto &heading : entry.value("headings").toArray()) {
            if (page.headings.size() < 8)
                page.headings.append(heading.toString().left(120));
        }
        for (const auto &crumb : entry.value("breadcrumbs").toArray()) {
            if (page.breadcrumbs.size() < 8)
                page.breadcrumbs.append(crumb.toString().left(80));
        }
        for (const auto &value : entry.value("links").toArray()) {
            const auto link = value.toObject();
            const auto url = SpatialSitePolicy::normalize(QUrl(link.value("url").toString()));
            if (!url.isEmpty() && page.links.size() < 64)
                page.links.append({url, link.value("title").toString().left(180), link.value("relation").toString().left(80), link.value("prominent").toBool()});
        }
        for (const auto &asset : entry.value("assets").toArray()) {
            const auto url = SpatialSitePolicy::assetUrl(QUrl(asset.toString()));
            if (!url.isEmpty() && page.assets.size() < 4)
                page.assets.append(url);
        }
        for (const auto &hash : entry.value("fingerprints").toArray()) {
            if (page.fingerprints.size() < 256 && hash.toDouble() >= 0 && hash.toDouble() <= 4294967295.0)
                page.fingerprints.insert(quint32(hash.toDouble()));
        }
        page.contentHash = entry.value("hash").toString().left(24);
        page.duplicateOf = entry.value("duplicate").toString().left(24);
        page.reason = entry.value("reason").toString().left(400);
        page.modified = entry.value("modified").toString().left(64);
        page.fetched = QDateTime::fromString(entry.value("fetched").toString(), Qt::ISODate);
        page.status = static_cast<SpatialSitePage::Status>(qBound(0, entry.value("status").toInt(), 3));
        page.override = static_cast<SpatialSitePage::Override>(qBound(0, entry.value("override").toInt(), 2));
        page.depth = qBound(0, entry.value("depth").toInt(), 6);
        page.score = qBound(0, entry.value("score").toInt(), 100);
        page.characters = qBound(0, entry.value("characters").toInt(), 24000);
        page.prominent = entry.value("prominent").toBool();
        page.noFollow = entry.value("nofollow").toBool();
        page.restricted = entry.value("restricted").toBool();
        page.group = entry.value("group").toString();
        page.slot = qBound(-1, entry.value("slot").toInt(-1), maximumPages - 1);
        if (!page.group.isEmpty() && !identities.contains(page.group)) {
            error = "Cached parent section is missing";
            return false;
        }
        const QString placement = page.group + ":" + QString::number(page.slot);
        if (!page.group.isEmpty()) {
            if (page.slot < 0 || pageSlots.contains(placement)) {
                error = "Duplicate or invalid cached placement";
                return false;
            }
            pageSlots.insert(placement);
        }
        if (root.value("contentVersion").toInt() < 2 && !page.restricted) {
            // Refresh old flattened excerpts without discarding placements or
            // user inclusion/exclusion choices. Fetches still obey crawl policy.
            page.summary.clear();
            page.fingerprints.clear();
            page.contentHash.clear();
            page.fetched = {};
            if (page.status == SpatialSitePage::Status::Ready)
                page.status = SpatialSitePage::Status::Discovered;
        }
        candidate.pages.append(page);
    }
    const auto features = root.value("features").toObject();
    for (auto it = features.begin(); it != features.end() && candidate.areaFeatures.size() < 48; ++it) {
        const auto values = it.value().toArray();
        QVector<float> scores;
        if (it.key().size() > 8192 || values.size() != 8)
            continue;
        for (const auto &entry : values) {
            const double value = entry.toDouble(-1);
            if (std::isfinite(value) && value >= 0 && value <= 1)
                scores.append(float(value));
        }
        if (scores.size() == 8)
            candidate.areaFeatures.insert(it.key(), scores);
    }
    *this = candidate;
    return true;
}
bool SpatialSiteWorld::clear(QString &error)
{
    error.clear();
    const QString path = cachePath(origin);
    QLockFile lock(path + ".lock");
    if (QFile::exists(path) && !lock.tryLock(0)) {
        error = "Site cache is being updated by another window";
        return false;
    }
    QFile file(path);
    if (file.exists() && !file.remove()) {
        error = file.errorString();
        return false;
    }
    pages.clear();
    sections.clear();
    areaFeatures.clear();
    m_revision.clear();
    m_cacheWritable = true;
    return true;
}
