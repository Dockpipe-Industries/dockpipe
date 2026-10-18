#include "SpatialSiteCrawler.h"
#include <QRegularExpression>

SpatialSiteCrawler::SpatialSiteCrawler(QObject *parent, SpatialSiteFetch *transport)
    : QObject(parent), m_fetch(transport ? transport : new SpatialSiteFetch(this))
{
    m_interpreter.setMaxThreadCount(1);
    m_interpreter.setExpiryTimeout(1000);
    m_timer.setSingleShot(true);
    m_deadline.setSingleShot(true);
    connect(&m_timer, &QTimer::timeout, this, &SpatialSiteCrawler::pump);
    connect(&m_deadline, &QTimer::timeout, this, [this]() { complete("Discovery time budget reached; explore or resume from the map"); });
    connect(m_fetch, &SpatialSiteFetch::completed, this, &SpatialSiteCrawler::receive);
}
SpatialSiteCrawler::~SpatialSiteCrawler()
{
    cancel();
    // The worker owns only bounded source data. Keep this QObject alive until it
    // has posted its result; QObject destruction then removes queued callbacks.
    m_interpreter.waitForDone();
}
void SpatialSiteCrawler::cancel()
{
    ++m_generation;
    m_interpreting = false;
    m_interpreter.clear();
    m_timer.stop();
    m_deadline.stop();
    m_fetch->cancel();
    m_jobs.clear();
    m_busy = false;
    m_running = false;
}
void SpatialSiteCrawler::complete(const QString &reason)
{
    cancel();
    m_status = reason;
    save();
    emit changed();
    emit stopped();
}
void SpatialSiteCrawler::save()
{
    if (m_world.origin.isEmpty())
        return;
    QString error;
    if (!m_world.save(error))
        m_status += " · " + error;
}
void SpatialSiteCrawler::start(const QUrl &url, SpatialSiteBudget budget, bool refresh)
{
    cancel();
    m_seen.clear();
    m_policyReady = false;
    m_robots = {};
    m_requests = 0;
    m_pageRequests = 0;
    m_maps = 0;
    m_bytes = 0;
    m_budget = budget;
    m_budget.pages = qBound(1, budget.pages, 64);
    m_budget.depth = qBound(0, budget.depth, 6);
    m_budget.bytes = qBound(512 * 1024, budget.bytes, 16 * 1024 * 1024);
    m_budget.milliseconds = qBound(1000, budget.milliseconds, 120000);
    const QString restriction = SpatialSitePolicy::restriction(url, url);
    if (!restriction.isEmpty()) {
        m_world = {};
        m_status = restriction;
        emit changed();
        emit stopped();
        return;
    }
    auto origin = SpatialSitePolicy::normalize(url);
    origin.setPath("/");
    origin.setQuery(QString());
    QString error;
    m_world.load(origin, error);
    if (!error.isEmpty())
        m_status = error;
    m_world.discover({SpatialSitePolicy::normalize(url), url.host(), {}, true}, 0);
    m_world.discover({origin, url.host(), {}, true}, 0);
    if (!SpatialSitePolicy::restriction(origin, origin).isEmpty()) {
        m_status = SpatialSitePolicy::restriction(origin, origin);
        emit changed();
        emit stopped();
        return;
    }
    m_running = true;
    m_clock.start();
    m_deadline.start(m_budget.milliseconds);
    enqueue({origin.resolved(QUrl("/robots.txt")), Kind::Robots});
    // A requested deep link must precede older cached discoveries. Crawl rules
    // are still resolved first, and pump applies them to every page request.
    enqueue({SpatialSitePolicy::normalize(url), Kind::Page, 0});
    for (auto &page : m_world.pages) {
        if (refresh)
            page.fetched = {};
    }
    m_status = "Checking site crawl rules; cached spaces are available immediately";
    emit changed();
    pump();
}
void SpatialSiteCrawler::enqueue(const Job &job, bool priority)
{
    const QUrl scope = job.kind == Kind::Image ? job.url : m_world.origin;
    if (job.url.isEmpty() || !SpatialSitePolicy::restriction(job.url, scope).isEmpty() || job.depth > m_budget.depth
        || m_jobs.size() >= SpatialSiteWorld::maximumPages || m_seen.contains(job.url.toString()))
        return;
    m_seen.insert(job.url.toString());
    if (priority)
        m_jobs.prepend(job);
    else
        m_jobs.enqueue(job);
}
void SpatialSiteCrawler::pump()
{
    if (!m_running || m_busy)
        return;
    if (m_bytes >= m_budget.bytes || m_requests >= m_budget.pages + 29) {
        complete("Discovery budget reached; cached world retained");
        return;
    }
    while (!m_jobs.isEmpty()) {
        m_current = m_jobs.dequeue();
        const bool siteResource = SpatialSitePolicy::sameOrigin(m_current.url, m_world.origin);
        if (m_current.kind != Kind::Robots && (!m_policyReady || (siteResource && !m_robots.allows(m_current.url)))) {
            if (auto *page = m_world.find(SpatialSitePolicy::identity(m_current.url.toString()))) {
                page->restricted = true;
                SpatialSiteContent::classify(*page, m_world.threshold, m_world.title);
                page->reason = "Disallowed by robots.txt";
            }
            continue;
        }
        if (m_current.kind == Kind::Page) {
            if (m_pageRequests >= m_budget.pages)
                continue;
            auto *page = m_world.find(SpatialSitePolicy::identity(m_current.url.toString()));
            if (page && (page->override == SpatialSitePage::Override::Exclude
                || (page->status == SpatialSitePage::Status::Excluded && page->override != SpatialSitePage::Override::Include)
                || (page->fetched.isValid() && page->fetched.secsTo(QDateTime::currentDateTimeUtc()) < 3600)))
                continue;
            ++m_pageRequests;
        }
        if (m_current.kind == Kind::Sitemap) {
            if (m_maps >= 16)
                continue;
            ++m_maps;
        }
        m_busy = true;
        ++m_requests;
        m_status = QString("Discovering · %1/%2 pages · %3 KiB · depth %4/%5")
            .arg(m_pageRequests).arg(m_budget.pages).arg(m_bytes / 1024).arg(m_current.depth).arg(m_budget.depth);
        emit changed();
        // Only explicitly referenced image jobs may use a public CDN. The
        // transport still pins DNS, rejects private addresses and sends no cookies.
        const QUrl scope = m_current.kind == Kind::Image ? m_current.url : m_world.origin;
        m_fetch->get(m_current.url, scope, m_budget.bytes - m_bytes);
        return;
    }
    complete("Discovery complete within this pass's limits; deeper pages load on demand");
}
void SpatialSiteCrawler::receive(const SpatialSiteResponse &response)
{
    if (!m_running || !m_busy || response.url != m_current.url)
        return;
    m_busy = false;
    m_bytes += response.body.size();
    if (m_bytes > m_budget.bytes) {
        complete("Site byte budget reached");
        return;
    }
    if (response.status == 429 || response.status == 503) {
        complete("Site requested a pause; discovery stopped without retrying");
        return;
    }
    if (!response.redirect.isEmpty()) {
        const bool image = m_current.kind == Kind::Image;
        const auto target = image ? SpatialSitePolicy::assetUrl(response.redirect) : SpatialSitePolicy::normalize(response.redirect);
        const QString restriction = SpatialSitePolicy::restriction(target, image ? target : m_world.origin);
        const auto *destination = m_world.find(SpatialSitePolicy::identity(target.toString()));
        if (m_current.kind == Kind::Page && restriction.isEmpty() && destination
            && destination->status == SpatialSitePage::Status::Ready && target != m_current.url) {
            if (auto *page = m_world.find(SpatialSitePolicy::identity(m_current.url.toString()))) {
                page->canonical = target;
                page->duplicateOf = destination->node.id;
                page->status = SpatialSitePage::Status::Excluded;
                page->reason = "Redirect to an already represented canonical destination";
            }
        } else if (m_current.redirects < 5 && restriction.isEmpty() && !m_seen.contains(target.toString())) {
            if (m_current.kind == Kind::Page) {
                m_world.discover({target, {}, {}, false}, m_current.depth);
                if (auto *page = m_world.find(SpatialSitePolicy::identity(m_current.url.toString()))) {
                    page->canonical = target;
                    page->duplicateOf = SpatialSitePolicy::identity(target.toString());
                    page->status = SpatialSitePage::Status::Excluded;
                    page->reason = "Redirect to canonical destination: " + target.toDisplayString();
                }
            }
            enqueue({target, m_current.kind, m_current.depth, m_current.redirects + 1, m_current.imageSource}, true);
        } else if (m_current.kind == Kind::Robots) {
            complete("Cannot safely resolve robots.txt; using single-page browsing");
            return;
        } else if (auto *page = m_world.find(SpatialSitePolicy::identity(m_current.url.toString()))) {
            page->status = SpatialSitePage::Status::Unavailable;
            page->reason = restriction.isEmpty() ? "Redirect loop or hop budget reached" : restriction;
        }
        m_timer.start(m_robots.delayMs);
        return;
    }
    if (m_current.kind == Kind::Robots) {
        if (!response.error.isEmpty() || (response.status != 200 && response.status != 404 && response.status != 410)) {
            complete("Crawl rules unavailable or access restricted; single-page browsing remains available");
            return;
        }
        if (response.status == 200 && response.body.trimmed().startsWith('<')) {
            complete("Site returned HTML instead of crawl rules; using single-page browsing");
            return;
        }
        if (response.status == 200)
            m_robots = SpatialRobotsPolicy::parse(response.body, m_world.origin);
        m_policyReady = true;
        // Fetch the entrance before sitemap expansion, so entry does not wait for a full map.
        for (auto &page : m_world.pages) {
            if (page.reason == "Disallowed by robots.txt" && m_robots.allows(page.node.url)) {
                page.restricted = false;
                page.status = SpatialSitePage::Status::Discovered;
                page.fetched = {};
                page.reason = "Crawl rules now permit discovery";
            }
            enqueue({page.node.url, Kind::Page, page.depth});
        }
        for (const auto &map : m_robots.sitemaps)
            enqueue({map, Kind::Sitemap, 0});
        enqueue({m_world.origin.resolved(QUrl("/sitemap.xml")), Kind::Sitemap, 0});
    } else if (m_current.kind == Kind::Sitemap) {
        if (response.status == 200 && response.error.isEmpty()) {
            QString error;
            const auto entries = SpatialSitePolicy::sitemap(response.body, m_world.origin, error);
            for (const auto &entry : entries) {
                if (entry.index && m_maps < 16)
                    enqueue({entry.url, Kind::Sitemap, m_current.depth + 1});
                else if (!entry.index) {
                    m_world.discover({entry.url, {}, {}, false}, 1);
                    if (auto *page = m_world.find(SpatialSitePolicy::identity(entry.url.toString())))
                        page->modified = entry.modified;
                    enqueue({entry.url, Kind::Page, 1});
                }
            }
            if (!error.isEmpty())
                m_status = error + " · using HTML navigation";
        }
    } else if (m_current.kind == Kind::Image) {
        if (response.status == 200 && response.error.isEmpty() && QList<QByteArray>{"image/png", "image/jpeg", "image/webp", "image/gif"}.contains(response.contentType.split(';').first()))
            emit imageReady(m_current.imageSource, response.body);
    } else {
        const QString id = SpatialSitePolicy::identity(m_current.url.toString());
        auto *known = m_world.find(id);
        if (!known) {
            m_world.discover({m_current.url, {}, {}, false}, m_current.depth);
            known = m_world.find(id);
        }
        if (response.status == 200 && response.error.isEmpty() && response.contentType.contains("html")) {
            m_busy = true;
            m_interpreting = true;
            m_status = "Interpreting public content in the background";
            const quint64 generation = m_generation;
            // A single worker and the serial fetch pipeline bound both queued
            // bodies and CPU work. No mutable world or QWidget is used here.
            m_interpreter.start([this, response, generation]() {
                const auto page = SpatialSiteContent::parse(response.body, response.url);
                QMetaObject::invokeMethod(this, [this, page, response, generation]() {
                    if (generation != m_generation || !m_running)
                        return;
                    m_busy = false;
                    m_interpreting = false;
                    acceptPage(page, response);
                    save();
                    emit changed();
                    m_timer.start(m_robots.delayMs);
                }, Qt::QueuedConnection);
            });
            emit changed();
            return;
        } else if (known) {
            known->status = SpatialSitePage::Status::Unavailable;
            known->reason = response.error.isEmpty() ? QString("HTTP %1 or unsupported content type").arg(response.status) : response.error;
            known->restricted = response.status == 401 || response.status == 403;
            if (known->restricted)
                SpatialSiteContent::classify(*known, m_world.threshold, m_world.title);
            known->fetched = QDateTime::currentDateTimeUtc();
        }
    }
    save();
    emit changed();
    m_timer.start(m_robots.delayMs);
}
void SpatialSiteCrawler::acceptPage(SpatialSitePage page, const SpatialSiteResponse &response)
{
    if (!response.modified.isEmpty())
        page.modified = response.modified;
    page.restricted = page.restricted || response.robots.contains("noindex") || response.robots.contains("noarchive") || response.robots.contains("none");
    page.noFollow = page.noFollow || response.robots.contains("nofollow");
    if (!SpatialSitePolicy::sameOrigin(page.canonical, m_world.origin)) {
        page.restricted = true;
        page.reason = "Canonical source is outside this site";
    }
    if (m_world.title.isEmpty() && m_current.depth == 0)
        m_world.title = page.node.title;
    if (!page.noFollow && !page.restricted && m_current.depth < m_budget.depth) {
        QVector<Job> navigation;
        for (const auto &link : page.links) {
            if (!m_world.discover(link, m_current.depth + 1))
                continue;
            const Job job{SpatialSitePolicy::normalize(link.url), Kind::Page, m_current.depth + 1};
            if (link.prominent)
                navigation.append(job);
            else
                enqueue(job);
        }
        // Prepending each link independently reverses the site's navigation and
        // spends small crawl budgets on footer/version controls first.
        for (auto link = navigation.crbegin(); link != navigation.crend(); ++link)
            enqueue(*link, true);
    }
    m_world.update(page);
}
void SpatialSiteCrawler::request(const QString &id, bool refresh)
{
    auto *page = m_world.find(id);
    if (!page || page->restricted || page->override == SpatialSitePage::Override::Exclude)
        return;
    if (!refresh && page->fetched.isValid() && page->fetched.secsTo(QDateTime::currentDateTimeUtc()) < 3600)
        return;
    const QUrl url = page->node.url;
    if (refresh)
        page->fetched = {};
    if (!m_running) {
        save();
        start(url, m_budget, false);
    }
    m_seen.remove(url.toString());
    enqueue({url, Kind::Page, 0}, true);
    if (!m_busy && !m_timer.isActive())
        m_timer.start(m_robots.delayMs);
}
void SpatialSiteCrawler::prefetch(const QString &section)
{
    // At most two related undiscovered destinations; normal pass budgets still apply.
    QStringList targets;
    for (const auto &page : m_world.pages) {
        if (page.group != section || page.status != SpatialSitePage::Status::Ready)
            continue;
        for (const auto &link : page.links) {
            const auto url = SpatialSitePolicy::normalize(link.url);
            const auto *target = m_world.find(SpatialSitePolicy::identity(url.toString()));
            if (target && target->status == SpatialSitePage::Status::Discovered && targets.size() < 2
                && !targets.contains(target->node.id))
                targets.append(target->node.id);
        }
    }
    for (const auto &id : targets)
        request(id);
}
void SpatialSiteCrawler::setOverride(const QString &id, SpatialSitePage::Override value)
{
    if (!m_world.setOverride(id, value))
        return;
    save();
    emit changed();
    if (value == SpatialSitePage::Override::Include)
        request(id);
}

void SpatialSiteCrawler::requestImage(const QUrl &url)
{
    const bool referenced = std::any_of(m_world.pages.cbegin(), m_world.pages.cend(), [&](const auto &page) {
        return page.status == SpatialSitePage::Status::Ready && !page.restricted && page.assets.contains(url);
    });
    if (!referenced || (!m_policyReady && !m_running)
        || (SpatialSitePolicy::sameOrigin(url, m_world.origin) && m_policyReady && !m_robots.allows(url))
        || !SpatialSitePolicy::restriction(url, url).isEmpty())
        return;
    if (!m_running && m_requests < m_budget.pages + 29 && m_bytes < m_budget.bytes && m_clock.elapsed() < m_budget.milliseconds) {
        m_running = true;
        m_deadline.start(qMax(1, m_budget.milliseconds - int(m_clock.elapsed())));
    }
    if (!m_running)
        return;
    enqueue({url, Kind::Image, 0, 0, url}, m_policyReady);
    if (!m_busy && !m_timer.isActive())
        m_timer.start(m_robots.delayMs);
}
