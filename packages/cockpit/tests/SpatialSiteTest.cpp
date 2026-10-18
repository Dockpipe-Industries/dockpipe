#include "SpatialSitePolicy.h"
#include "SpatialSitePage.h"
#include "SpatialSiteCrawler.h"
#include "SpatialSiteExplorer.h"
#include "SpatialWebWorld.h"
#include "WorkspaceFlightView.h"
#include "SpatialWebBrowser.h"
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QApplication>
#include <QElapsedTimer>
#include <QTemporaryDir>
#include <QThread>
#include <QFile>
#include <QScreen>
#include <QVBoxLayout>
#include <cstdio>
#include <cmath>
#ifdef DOCKPIPE_SPATIAL_WEB
#include <QWebEngineView>
#include <QWebEngineProfile>
#include <QWebEnginePage>
#include <QWebEngineUrlRequestInterceptor>
#include <QWebEngineUrlRequestInfo>
#include <QTcpServer>
#include <QTcpSocket>
#endif

#define SITE_CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Site assertion %d: %s\n", __LINE__, #value); return 1; } } while (false)
namespace {
bool waitUntil(const std::function<bool()> &condition, int milliseconds = 15000)
{
    QElapsedTimer clock;
    clock.start();
    while (!condition() && clock.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(3);
    }
    return condition();
}
QByteArray article(const QString &title, const QString &section, const QString &extra = {})
{
    const QString text = title + " explains the principles, evidence and practical context of this subject. "
        + (title + " provides detailed " + title + " information and " + title + " examples for readers. ").repeated(12);
    return ("<html><head><title>" + title + "</title><meta property='article:section' content='" + section
        + "'></head><body><nav><a href='/science' >Science</a><a href='/history'>History</a><a href='/login'>Login</a></nav>"
        + "<main><h1>" + title + "</h1><p>" + text + "</p>" + extra + "</main></body></html>").toUtf8();
}
#ifdef DOCKPIPE_SPATIAL_WEB
class FixturePageRouter final : public QWebEngineUrlRequestInterceptor {
public:
    QUrl target;
    void interceptRequest(QWebEngineUrlRequestInfo &info) override
    {
        if (info.requestUrl().host() == "fixture.example.org")
            info.redirect(target);
        else if (info.requestUrl().host() != "127.0.0.1")
            info.block(true);
    }
};
#endif
class FixtureFetch final : public SpatialSiteFetch {
public:
    QMap<QString, SpatialSiteResponse> responses;
    QStringList requested;
    QList<QUrl> requestedUrls;
    quint64 generation = 0;
    void cancel() override { ++generation; }
    void get(const QUrl &url, const QUrl &, int = 512 * 1024) override
    {
        requested.append(url.path());
        requestedUrls.append(url);
        auto response = responses.value(url.path());
        response.url = url;
        if (!response.status)
            response.status = 404;
        const auto current = generation;
        QTimer::singleShot(0, this, [this, current, response]() {
            if (current == generation)
                emit completed(response);
        });
    }
    void add(const QString &path, const QByteArray &body, const QByteArray &type = "text/html", int status = 200)
    {
        SpatialSiteResponse response;
        response.body = body;
        response.contentType = type;
        response.status = status;
        responses.insert(path, response);
    }
};
}
int spatialSiteTest()
{
    QTemporaryDir cache;
    qputenv("XDG_CACHE_HOME", cache.path().toUtf8());
    const QUrl origin("https://example.org/");
    const QByteArray prose = "Brown bears explore woodland habitats and forage for seasonal food. This fixture describes their habitat and behaviour.";
    const QByteArray gallery = "<div aria-roledescription='carousel'><div><div class='Image__Copyright'>"
        "Image rights notice.</div></div><p>Repeated gallery caption that should never become the first source excerpt.</p>"
        "<span>1 / 3</span></div><p class='Caption__Title'>A long gallery caption with a credit belongs with its photograph.</p>";
    const auto cleanArticle = SpatialSiteContent::parse("<title>Bear habitat</title><meta name='description' content=''>"
        "<article>" + gallery + "<h1>Bear habitat</h1><p>" + prose + "</p><p>" + prose + "</p>"
        "<p aria-hidden='true'>Hidden duplicated article content must not appear in source excerpts.</p>"
        "<p><span class='Image__Copyright'><span>Nested image rights notice.</span></span>"
        "Copyright law is a legitimate educational subject and must remain readable in ordinary article prose.</p></article>", origin);
    SITE_CHECK(cleanArticle.summary.startsWith(prose));
    SITE_CHECK(cleanArticle.summary.count(QString::fromUtf8(prose)) == 1);
    SITE_CHECK(!cleanArticle.summary.contains("notice") && !cleanArticle.summary.contains("caption"));
    SITE_CHECK(!cleanArticle.summary.contains("1 / 3") && !cleanArticle.summary.contains("Hidden"));
    SITE_CHECK(cleanArticle.summary.contains("Copyright law"));
    const auto noParagraphs = SpatialSiteContent::parse("<article><div class='Image__Copyright'>Image rights notice.</div>"
        "<div>Useful source text without paragraph markup remains available.</div></article>", origin);
    SITE_CHECK(noParagraphs.summary == "Useful source text without paragraph markup remains available.");
    const auto malformedGallery = SpatialSiteContent::parse("<article><div class='Image__Copyright'><div>Unclosed gallery", origin);
    SITE_CHECK(malformedGallery.summary.isEmpty());
    const QString sourceProbe = qEnvironmentVariable("DOCKPIPE_SITE_ARTICLE_SOURCE");
    if (!sourceProbe.isEmpty()) {
        QFile source(sourceProbe);
        SITE_CHECK(source.open(QIODevice::ReadOnly));
        const auto parsed = SpatialSiteContent::parse(source.read(512 * 1024), origin);
        SITE_CHECK(!parsed.summary.isEmpty());
        std::fprintf(stderr, "Article source probe: %s\n", qPrintable(parsed.summary));
    }
    SITE_CHECK(SpatialSitePolicy::normalize(QUrl("https://EXAMPLE.org:443/a/../science?utm_source=x&lang=en#part")) == QUrl("https://example.org/science?lang=en"));
    SITE_CHECK(SpatialSitePolicy::normalize(QUrl("https://user:secret@example.org/")).isEmpty());
    const QUrl imageUrl("https://cdn.example.org/photo?width=640&format=webp&signature=abc");
    SITE_CHECK(SpatialSitePolicy::assetUrl(imageUrl) == imageUrl);
    SITE_CHECK(SpatialSitePolicy::assetUrl(QUrl("data:image/png;base64,AA==")).isEmpty());
    const auto images = SpatialSiteContent::parse(article("Images", "Media", R"(
        <meta property="og:image" content="https://cdn.example.org/photo?width=640&amp;format=webp&amp;signature=abc">
        <img src="data:image/gif;base64,AA==" data-src="/lazy.png?w=600">
        <img srcset="/responsive.png?w=480 480w, /responsive.png?w=960 960w">
        <script type="application/ld+json">{"@type":"Article","image":{"@type":"ImageObject","url":"https://cdn.example.org/hero.png?w=900"}}</script>
    )"), origin);
    SITE_CHECK(images.assets.size() == 4 && images.assets.contains(imageUrl));
    SITE_CHECK(images.assets.contains(origin.resolved(QUrl("/lazy.png?w=600"))));
    SITE_CHECK(images.assets.contains(origin.resolved(QUrl("/responsive.png?w=480"))));
    for (const auto &address : QStringList{"127.0.0.1", "10.0.0.1", "169.254.169.254", "100.64.0.1", "::1", "::ffff:192.168.1.2", "fe80::1", "2001:db8::1"})
        SITE_CHECK(!SpatialSitePolicy::publicAddress(QHostAddress(address)));
    SITE_CHECK(SpatialSitePolicy::publicAddress(QHostAddress("8.8.8.8")));
    SITE_CHECK(!SpatialSitePolicy::restriction(QUrl("http://127.0.0.1/"), QUrl("http://127.0.0.1/")).isEmpty());
    SITE_CHECK(!SpatialSitePolicy::restriction(QUrl("https://example.org:8443/"), origin).isEmpty());
    SITE_CHECK(!SpatialSitePolicy::restriction(QUrl("https://other.example.org/"), origin).isEmpty());
    for (const auto &path : QStringList{"/login", "/cart", "/api/data", "/search?q=hello", "/page/2", "/guide.css", "/download.html"})
        SITE_CHECK(!SpatialSitePolicy::utilityReason(origin.resolved(QUrl(path))).isEmpty());
    auto robots = SpatialRobotsPolicy::parse("User-agent: *\nDisallow: /blocked\nAllow: /blocked/open$\nCrawl-delay: 2\nSitemap: https://example.org/maps.xml\n", origin);
    SITE_CHECK(!robots.allows(origin.resolved(QUrl("/blocked/private"))));
    SITE_CHECK(robots.allows(origin.resolved(QUrl("/blocked/open"))));
    SITE_CHECK(!robots.allows(origin.resolved(QUrl("/blocked/open/more"))));
    const auto unicodeRobots = SpatialRobotsPolicy::parse(QString("User-agent: *\nDisallow: /café\nDisallow: /%62locked\n").toUtf8(), origin);
    SITE_CHECK(!unicodeRobots.allows(origin.resolved(QUrl("/caf%C3%A9"))));
    SITE_CHECK(!unicodeRobots.allows(origin.resolved(QUrl("/blocked"))));
    SITE_CHECK(robots.delayMs == 2000 && robots.sitemaps.size() == 1);
    robots = SpatialRobotsPolicy::parse("User-agent: *\nDisallow: /\nUser-agent: DockpipeSpatial\nAllow: /\n", origin);
    SITE_CHECK(robots.allows(origin));
    QString error;
    auto maps = SpatialSitePolicy::sitemap("<sitemapindex><sitemap><loc>https://example.org/child.xml</loc></sitemap></sitemapindex>", origin, error);
    SITE_CHECK(error.isEmpty() && maps.size() == 1 && maps[0].index);
    maps = SpatialSitePolicy::sitemap("<urlset><url><loc>https://example.org/science</loc><lastmod>2026-10-01</lastmod></url></urlset>", origin, error);
    SITE_CHECK(error.isEmpty() && maps.size() == 1 && maps[0].modified == "2026-10-01");
    SITE_CHECK(SpatialSitePolicy::sitemap("<urlset><bad", origin, error).isEmpty() && !error.isEmpty());
    SITE_CHECK(SpatialSitePolicy::sitemap("<!DOCTYPE x [<!ENTITY x SYSTEM 'file:///etc/passwd'>]><urlset/>", origin, error).isEmpty() && !error.isEmpty());

    SpatialSiteWorld world;
    world.origin = origin;
    world.title = "Knowledge and collections";
    const QList<QPair<QString, QString>> subjects{{"Physics courses", "Science"}, {"Ancient artifacts", "History"},
        {"Software API reference", "Engineering"}, {"Garden products and prices", "Garden"}, {"Encyclopedia of birds", "Nature"}};
    int ordinal = 0;
    for (const auto &subject : subjects) {
        const QUrl url = origin.resolved(QUrl("/unrelated-url-" + QString::number(ordinal++)));
        auto page = SpatialSiteContent::parse(article(subject.first, subject.second), url);
        SITE_CHECK(page.node.title == subject.first && page.section == subject.second && page.characters > 500);
        world.update(page);
        SITE_CHECK(world.find(page.node.id)->status == SpatialSitePage::Status::Ready);
    }
    SITE_CHECK(world.sections.size() == 5);
    auto duplicate = SpatialSiteContent::parse(article(subjects[0].first, subjects[0].second), origin.resolved(QUrl("/duplicate")));
    world.update(duplicate);
    SITE_CHECK(world.find(duplicate.node.id)->status == SpatialSitePage::Status::Excluded);
    SITE_CHECK(!world.find(duplicate.node.id)->duplicateOf.isEmpty());
    SITE_CHECK(world.setOverride(duplicate.node.id, SpatialSitePage::Override::Include));
    SITE_CHECK(world.find(duplicate.node.id)->status == SpatialSitePage::Status::Ready);
    auto canonical = SpatialSiteContent::parse(article("Canonical source", "Science", "<link rel='canonical' href='/unrelated-url-0?utm_campaign=test'>"), origin.resolved(QUrl("/alias")));
    SITE_CHECK(canonical.canonical == origin.resolved(QUrl("/unrelated-url-0")));
    auto privatePage = SpatialSiteContent::parse(article("Private lesson", "Science", "<meta name='robots' content='noindex,nofollow'>"), origin.resolved(QUrl("/private")));
    world.update(privatePage);
    SITE_CHECK(!world.setOverride(privatePage.node.id, SpatialSitePage::Override::Include));
    SITE_CHECK(world.find(privatePage.node.id)->summary.isEmpty());
    SITE_CHECK(world.find(privatePage.node.id)->assets.isEmpty());
    const auto first = world.pages.first();
    const auto projection = world.project(first.group);
    auto result = SpatialWebWorld::construct(projection, SpatialWebWorld::optionsFor(projection));
    SITE_CHECK(result.success && !result.interior.displays.isEmpty());
    const auto center = result.interior.displays.first().center;
    auto added = SpatialSiteContent::parse(article("New discoveries about light", "Science"), origin.resolved(QUrl("/new")));
    added.override = SpatialSitePage::Override::Include;
    world.update(added);
    const auto updated = world.project(first.group);
    auto next = SpatialWebWorld::construct(updated, SpatialWebWorld::optionsFor(updated));
    SITE_CHECK(next.success && next.interior.displays.first().center == center);
    SITE_CHECK(next.interior.spawn == result.interior.spawn);
    SITE_CHECK(world.find(first.node.id)->slot == first.slot && world.find(first.node.id)->group == first.group);
    world.find(first.node.id)->assets = {imageUrl};
    SITE_CHECK(world.save(error));
    SpatialSiteWorld loaded;
    SITE_CHECK(loaded.load(origin, error));
    SITE_CHECK(loaded.pages.size() == world.pages.size() && loaded.find(first.node.id)->slot == first.slot);
    SITE_CHECK(loaded.find(first.node.id)->assets == QVector<QUrl>{imageUrl});
    QFile legacy(SpatialSiteWorld::cachePath(origin));
    SITE_CHECK(legacy.open(QIODevice::ReadOnly));
    auto oldCache = QJsonDocument::fromJson(legacy.readAll()).object();
    legacy.close();
    oldCache.remove("contentVersion");
    SITE_CHECK(legacy.open(QIODevice::WriteOnly));
    legacy.write(QJsonDocument(oldCache).toJson());
    legacy.close();
    SITE_CHECK(loaded.load(origin, error));
    const auto *refreshing = loaded.find(first.node.id);
    SITE_CHECK(refreshing && refreshing->slot == first.slot && refreshing->group == first.group);
    SITE_CHECK(refreshing->summary.isEmpty() && !refreshing->fetched.isValid());
    SITE_CHECK(refreshing->status == SpatialSitePage::Status::Discovered);
    SITE_CHECK(refreshing->override == first.override);
    QFile corrupt(SpatialSiteWorld::cachePath(origin));
    SITE_CHECK(corrupt.open(QIODevice::WriteOnly));
    corrupt.write("not a site graph");
    corrupt.close();
    SITE_CHECK(!loaded.load(origin, error));
    SITE_CHECK(!loaded.save(error));
    SITE_CHECK(!world.save(error));

    // Cache conflicts do not overwrite another window's graph or invalid content.
    SITE_CHECK(loaded.clear(error));
    SITE_CHECK(loaded.load(origin, error));
    loaded.update(first);
    SITE_CHECK(loaded.save(error));
    SpatialSiteWorld competing;
    SITE_CHECK(competing.load(origin, error));
    loaded.title = "Updated in the first window";
    SITE_CHECK(loaded.save(error));
    competing.title = "Stale competing write";
    SITE_CHECK(!competing.save(error));

    SpatialSiteWorld versions;
    versions.origin = origin;
    versions.discover({origin.resolved(QUrl("/3.12/")), "Python 3.12 (security fixes)", {}, true}, 1);
    SITE_CHECK(versions.pages.first().status == SpatialSitePage::Status::Excluded);
    SITE_CHECK(versions.setOverride(versions.pages.first().node.id, SpatialSitePage::Override::Include));
    SITE_CHECK(versions.pages.first().status == SpatialSitePage::Status::Discovered);
    versions.discover({origin.resolved(QUrl("/history/1912/")), "1912", {}, true}, 1);
    SITE_CHECK(versions.pages.last().status == SpatialSitePage::Status::Discovered);
    auto documentation = SpatialSiteContent::parse(article("3.15 Documentation", ""), origin);
    SpatialSiteContent::classify(documentation, 45, "Documentation");
    SITE_CHECK(documentation.section == "Software & systems");
    auto fallback = SpatialSiteContent::parse(article("Physics research laboratory", ""), origin.resolved(QUrl("/opaque/42")));
    SpatialSiteContent::classify(fallback, 45, "University science");
    SITE_CHECK(fallback.section == "Science & discovery" && fallback.status == SpatialSitePage::Status::Ready);
    auto legal = SpatialSiteContent::parse(article("Privacy policy", "Legal"), origin.resolved(QUrl("/privacy")));
    SpatialSiteContent::classify(legal, 45, "Garden shop");
    const auto ordinaryScore = legal.score;
    SpatialSiteContent::classify(legal, 45, "Legal policy research");
    SITE_CHECK(legal.score > ordinaryScore && legal.status == SpatialSitePage::Status::Ready);
    SpatialSiteContent::classify(legal, 100, "Legal policy research");
    SITE_CHECK(legal.status == SpatialSitePage::Status::Excluded);
    const QByteArray metadata = R"(<script type="application/ld+json">{
        "@graph":[{"@type":"Product","description":"Source product description"},
        {"@type":"BreadcrumbList","itemListElement":[{"name":"Home"},{"name":"Garden"},{"name":"Tools"}]}]}
        </script><script>var ignored = '<a href="/danger">Private</a>';</script>
        <base href="/catalog/"><a href="tool">Garden tool</a>)";
    auto structured = SpatialSiteContent::parse(article("Garden tool", "", QString::fromUtf8(metadata)), origin);
    SITE_CHECK(structured.kind == "product" && structured.section == "Garden");
    SITE_CHECK(structured.summary == "Source product description");
    SITE_CHECK(std::any_of(structured.links.begin(), structured.links.end(), [](const auto &link) { return link.url.path() == "/catalog/tool"; }));
    SITE_CHECK(std::none_of(structured.links.begin(), structured.links.end(), [](const auto &link) { return link.url.path() == "/danger"; }));
    auto crumbs = SpatialSiteContent::parse(article("Pottery", "", "<nav aria-label='Breadcrumb'><a>Home</a><a>Collections</a><span>Pottery</span></nav>"), origin);
    SITE_CHECK(crumbs.section == "Collections");
    SpatialSiteWorld large;
    large.origin = QUrl("https://large.example.org/");
    for (int index = 0; index < 400; ++index)
        large.discover({large.origin.resolved(QUrl("/article/" + QString::number(index))), "Article", {}, false}, 1);
    SITE_CHECK(large.pages.size() == SpatialSiteWorld::maximumPages);
    for (int index = 0; index < 32; ++index) {
        auto page = SpatialSiteContent::parse(article("Subject " + QString::number(index), "Topic " + QString::number(index)), large.pages[index].node.url);
        page.override = SpatialSitePage::Override::Include;
        large.update(page);
    }
    SITE_CHECK(large.sections.size() <= SpatialSiteWorld::maximumSections && large.project().items.size() <= 12);
    SITE_CHECK(large.sections.last().title == "More topics");
    SITE_CHECK(large.project({}, 1).items.size() <= 12);
    auto unavailable = large.pages.first();
    const auto originalSlot = unavailable.slot;
    auto *deleted = large.find(unavailable.node.id);
    deleted->status = SpatialSitePage::Status::Unavailable;
    SITE_CHECK(large.project(deleted->group).items.isEmpty());
    large.update(unavailable);
    SITE_CHECK(large.find(unavailable.node.id)->slot == originalSlot);

    FixtureFetch fixture;
    fixture.add("/robots.txt", "User-agent: *\nDisallow: /blocked\nSitemap: https://fixture.example.org/map-index.xml\n", "text/plain");
    fixture.add("/", article("Learning and history", "Welcome", "<a href='/blocked'>Restricted</a><a href='/alias'>Science alias</a>"));
    fixture.add("/science", article("Physics laboratory", "Science"));
    fixture.add("/alias", {}, "text/html", 302);
    fixture.responses["/alias"].redirect = QUrl("https://fixture.example.org/science");
    fixture.add("/history", article("Museum collection", "History"));
    fixture.add("/map-index.xml", "<sitemapindex><sitemap><loc>https://fixture.example.org/map-pages.xml</loc></sitemap></sitemapindex>", "application/xml");
    fixture.add("/map-pages.xml", "<urlset><url><loc>https://fixture.example.org/software</loc></url><url><loc>https://fixture.example.org/empty</loc></url></urlset>", "application/xml");
    fixture.add("/software", article("Programming API tutorial", "Engineering"));
    fixture.add("/empty", "<html><title>Empty</title><main>Coming soon</main></html>");
    SpatialSiteCrawler crawler(nullptr, &fixture);
    int changes = 0;
    QObject::connect(&crawler, &SpatialSiteCrawler::changed, [&]() { ++changes; });
    crawler.start(QUrl("https://fixture.example.org/"));
    SITE_CHECK(waitUntil([&]() { return !crawler.world().sections.isEmpty(); }, 4000));
    SITE_CHECK(crawler.running()); // Entrance is usable before discovery completes.
    SITE_CHECK(waitUntil([&]() { return !crawler.running(); }));
    SITE_CHECK(changes > 3 && crawler.requests() <= 49);
    SITE_CHECK(!fixture.requested.contains("/blocked") && !fixture.requested.contains("/login"));
    SITE_CHECK(fixture.requested.contains("/software"));
    SITE_CHECK(fixture.requested.indexOf("/science") < fixture.requested.indexOf("/history"));
    SITE_CHECK(crawler.world().sections.size() >= 3);
    const auto *alias = crawler.world().find(SpatialSitePolicy::identity("https://fixture.example.org/alias"));
    SITE_CHECK(alias && alias->status == SpatialSitePage::Status::Excluded);
    SITE_CHECK(alias->duplicateOf == SpatialSitePolicy::identity("https://fixture.example.org/science"));
    auto *empty = crawler.world().find(SpatialSitePolicy::identity("https://fixture.example.org/empty"));
    SITE_CHECK(empty && empty->status == SpatialSitePage::Status::Excluded && !empty->reason.isEmpty());
    crawler.start(QUrl("https://fixture.example.org/"), {}, true);
    crawler.cancel();
    const auto count = fixture.requested.size();
    waitUntil([]() { return false; }, 100);
    SITE_CHECK(!crawler.running() && fixture.requested.size() == count);
    FixtureFetch priorityFixture;
    priorityFixture.add("/robots.txt", {}, "text/plain", 404);
    priorityFixture.add("/requested", article("Animal facts", "Animals"));
    SpatialSiteCrawler priority(nullptr, &priorityFixture);
    priority.world().origin = QUrl("https://priority.example.org/");
    for (int index = 0; index < 40; ++index)
        priority.world().discover({QUrl("https://priority.example.org/older-" + QString::number(index)), "Older discovery", {}, false}, 1);
    priority.save();
    SpatialSiteBudget priorityBudget;
    priorityBudget.pages = 1;
    priority.start(QUrl("https://priority.example.org/requested"), priorityBudget);
    SITE_CHECK(waitUntil([&]() { return !priority.running(); }, 5000));
    SITE_CHECK(priorityFixture.requested.indexOf("/requested") == 1);
    SITE_CHECK(!priorityFixture.requested.contains("/older-0"));
    SpatialSiteFetch transport;
    bool denied = false;
    QObject::connect(&transport, &SpatialSiteFetch::completed, [&](const SpatialSiteResponse &response) { denied = !response.error.isEmpty(); });
    transport.get(QUrl("http://127.0.0.1/"), QUrl("http://127.0.0.1/"));
    SITE_CHECK(waitUntil([&]() { return denied; }, 1000));

    FixtureFetch limitedFixture;
    limitedFixture.add("/robots.txt", {}, "text/plain", 404);
    limitedFixture.add("/", article("Software learning", "Software"));
    SpatialSiteCrawler limited(nullptr, &limitedFixture);
    SpatialSiteBudget small;
    small.pages = 1;
    small.depth = 0;
    limited.start(QUrl("https://limited.example.org/"), small);
    SITE_CHECK(waitUntil([&]() { return !limited.running(); }, 5000));
    SITE_CHECK(!limitedFixture.requested.contains("/science") && limited.world().pages.size() == 1);
    SITE_CHECK(limited.world().sections.size() == 1); // Missing sitemap still has useful content.
    const auto readyId = limited.world().pages.first().node.id;
    const auto previousRequests = limitedFixture.requested.size();
    limited.request(readyId);
    SITE_CHECK(limitedFixture.requested.size() == previousRequests && !limited.running());
    limitedFixture.add("/", {}, "text/html", 404);
    limited.request(readyId, true);
    SITE_CHECK(waitUntil([&]() { return !limited.running(); }, 5000));
    SITE_CHECK(limited.world().find(readyId)->status == SpatialSitePage::Status::Unavailable);

    FixtureFetch redirectFixture;
    redirectFixture.add("/robots.txt", {}, "text/plain", 404);
    redirectFixture.add("/", {}, "text/html", 302);
    redirectFixture.responses["/"].redirect = QUrl("http://169.254.169.254/latest/meta-data/");
    SpatialSiteCrawler redirects(nullptr, &redirectFixture);
    redirects.start(QUrl("https://redirect.example.org/"));
    SITE_CHECK(waitUntil([&]() { return !redirects.running(); }, 5000));
    SITE_CHECK(!redirectFixture.requested.contains("/latest/meta-data/"));
    SITE_CHECK(redirects.world().pages.first().status == SpatialSitePage::Status::Unavailable);
    FixtureFetch imageFixture;
    imageFixture.add("/robots.txt", {}, "text/plain", 404);
    imageFixture.add("/", article("Photographic collection", "Media",
        "<meta property='og:image' content='" + imageUrl.toString() + "'>"
        "<img src='https://cdn.example.org/unsafe.png'><img src='https://cdn.example.org/cancel.png'>"));
    imageFixture.add("/photo", {}, "image/png", 302);
    imageFixture.responses["/photo"].redirect = QUrl("https://images.example.org/resolved.png?width=640");
    imageFixture.add("/resolved.png", "fixture pixels", "image/png");
    imageFixture.add("/unsafe.png", {}, "image/png", 302);
    imageFixture.responses["/unsafe.png"].redirect = QUrl("http://127.0.0.1/private.png");
    SpatialSiteCrawler imageCrawler(nullptr, &imageFixture);
    imageCrawler.start(QUrl("https://photos.example.org/"), small);
    SITE_CHECK(waitUntil([&]() { return !imageCrawler.running(); }, 5000));
    QUrl receivedImage;
    QObject::connect(&imageCrawler, &SpatialSiteCrawler::imageReady, [&](const QUrl &url, const QByteArray &) { receivedImage = url; });
    imageCrawler.requestImage(imageUrl);
    SITE_CHECK(waitUntil([&]() { return !receivedImage.isEmpty(); }, 5000));
    SITE_CHECK(receivedImage == imageUrl && imageFixture.requestedUrls.contains(imageUrl));
    SITE_CHECK(imageFixture.requestedUrls.contains(QUrl("https://images.example.org/resolved.png?width=640")));
    imageCrawler.requestImage(QUrl("https://cdn.example.org/unsafe.png"));
    SITE_CHECK(waitUntil([&]() { return !imageCrawler.running(); }, 5000));
    SITE_CHECK(!imageFixture.requested.contains("/private.png"));
    const int imageRequests = imageFixture.requested.size();
    imageCrawler.requestImage(QUrl("https://unreferenced.example.org/unrequested.png"));
    SITE_CHECK(!imageCrawler.running() && imageFixture.requested.size() == imageRequests);
    imageCrawler.requestImage(QUrl("https://cdn.example.org/cancel.png"));
    imageCrawler.cancel();
    waitUntil([]() { return false; }, 800);
    SITE_CHECK(imageFixture.requested.size() == imageRequests && !imageCrawler.running());
    FixtureFetch rejectedFixture;
    rejectedFixture.add("/robots.txt", {}, "text/plain", 403);
    SpatialSiteCrawler rejected(nullptr, &rejectedFixture);
    rejected.start(QUrl("https://restricted.example.org/"));
    SITE_CHECK(waitUntil([&]() { return !rejected.running(); }, 1000));
    SITE_CHECK(rejectedFixture.requested == QStringList{"/robots.txt"});
    FixtureFetch oversizedFixture;
    oversizedFixture.add("/robots.txt", {}, "text/plain", 404);
    oversizedFixture.add("/", QByteArray(512 * 1024 + 1, 'x'));
    SpatialSiteCrawler oversized(nullptr, &oversizedFixture);
    small.bytes = 512 * 1024;
    oversized.start(QUrl("https://oversized.example.org/"), small);
    SITE_CHECK(waitUntil([&]() { return !oversized.running(); }, 3000));
    SITE_CHECK(oversized.world().sections.isEmpty() && oversized.status().contains("byte budget"));
    FixtureFetch pausedFixture;
    pausedFixture.add("/robots.txt", {}, "text/plain", 429);
    SpatialSiteCrawler paused(nullptr, &pausedFixture);
    paused.start(QUrl("https://paused.example.org/"));
    SITE_CHECK(waitUntil([&]() { return !paused.running(); }, 1000));
    SITE_CHECK(pausedFixture.requested.size() == 1);
    SITE_CHECK(robots.allows(origin));
    const auto slow = SpatialRobotsPolicy::parse("User-agent: *\nCrawl-delay: 60\n", origin);
    SITE_CHECK(slow.delayMs == 60000);
    FixtureFetch timedFixture;
    timedFixture.add("/robots.txt", "User-agent: *\nCrawl-delay: 60\n", "text/plain");
    SpatialSiteCrawler timed(nullptr, &timedFixture);
    small.milliseconds = 1000;
    timed.start(QUrl("https://timed.example.org/"), small);
    SITE_CHECK(waitUntil([&]() { return !timed.running(); }, 2000));
    SITE_CHECK(timedFixture.requested.size() == 1 && timed.status().contains("time budget"));

    QByteArray dense("<title>Engineering reference</title><main><h1>Software concepts</h1>");
    for (int index = 0; index < 2048; ++index)
        dense += "<a href='/topic/" + QByteArray::number(index) + "'>Reference topic</a>";
    dense += QByteArray(380 * 1024, 'a') + "</main>";
    FixtureFetch backgroundFixture;
    backgroundFixture.add("/robots.txt", {}, "text/plain", 404);
    backgroundFixture.add("/", dense);
    SpatialSiteCrawler background(nullptr, &backgroundFixture);
    SpatialSiteBudget backgroundBudget;
    backgroundBudget.depth = 0;
    backgroundBudget.pages = 1;
    background.start(QUrl("https://background.example.org/"), backgroundBudget);
    SITE_CHECK(waitUntil([&]() { return background.interpreting(); }, 3000));
    int heartbeats = 0;
    QTimer heartbeat;
    heartbeat.setInterval(1);
    QObject::connect(&heartbeat, &QTimer::timeout, [&]() {
        if (background.interpreting())
            ++heartbeats;
    });
    heartbeat.start();
    SITE_CHECK(waitUntil([&]() { return !background.running(); }, 5000));
    SITE_CHECK(heartbeats > 0 && background.world().sections.size() == 1);
    heartbeat.stop();
    background.start(QUrl("https://cancel-parse.example.org/"), backgroundBudget);
    SITE_CHECK(waitUntil([&]() { return background.interpreting(); }, 3000));
    background.cancel();
    waitUntil([]() { return false; }, 300);
    SITE_CHECK(!background.running() && background.world().sections.isEmpty());
    std::printf("Background interpretation: %d UI heartbeats during a %lld-byte page; cancelled results discarded.\n",
        heartbeats, static_cast<long long>(dense.size()));

    // Render the real explorer controls and existing scene renderer using the same fixture graph.
    QWidget shell;
    auto *layout = new QVBoxLayout(&shell);
    auto *explorer = new SpatialSiteExplorer(&shell, &fixture);
    auto *scene = new WorkspaceFlightView({}, QGuiApplication::platformName() != "offscreen", &shell);
    layout->addWidget(explorer);
    layout->addWidget(scene, 1);
    QObject::connect(explorer, &SpatialSiteExplorer::areaReady, [&](const SpatialWebDocument &document) {
        const auto result = SpatialWebWorld::construct(document, SpatialWebWorld::optionsFor(document));
        if (!result.success)
            std::fprintf(stderr, "Site scene: %s\n", qPrintable(result.diagnostic));
        if (result.success) {
            scene->setInterior(result.interior, document.title, false, false, "site:" + document.layoutIdentity);
            for (int index = 0; index < document.items.size(); ++index)
                scene->setInteriorImage(document.items[index].id, SpatialWebDocument::poster(document.items[index], {}, index));
        }
    });
    shell.resize(1400, 950);
    shell.show();
    SITE_CHECK(explorer->begin(QUrl("https://fixture.example.org/")));
    SITE_CHECK(waitUntil([&]() { return scene->walking(); }, 1000));
    explorer->pause();
    int excludedPublications = 0;
    const auto excludedConnection = QObject::connect(explorer, &SpatialSiteExplorer::areaReady,
        [&]() { ++excludedPublications; });
    SITE_CHECK(explorer->activate("source:" + SpatialSitePolicy::identity("https://fixture.example.org/empty")));
    SITE_CHECK(excludedPublications == 0);
    QObject::disconnect(excludedConnection);
    // Deep links must not publish a cached site overview into a loading corridor.
    int deepPublications = 0;
    QUrl resolvedSource;
    const auto deepConnection = QObject::connect(explorer, &SpatialSiteExplorer::areaReady,
        [&]() { ++deepPublications; });
    const auto sourceConnection = QObject::connect(explorer, &SpatialSiteExplorer::sourceRequested,
        [&](const QUrl &url) { resolvedSource = url; });
    SITE_CHECK(explorer->begin(QUrl("https://fixture.example.org/science")));
    SITE_CHECK(deepPublications == 0 && resolvedSource.path() == "/science");
    resolvedSource = QUrl();
    fixture.add("/animals/invertebrates", article("Invertebrate animals", "Animals"));
    SITE_CHECK(explorer->begin(QUrl("https://fixture.example.org/animals/invertebrates")));
    SITE_CHECK(deepPublications == 0);
    SITE_CHECK(waitUntil([&]() { return !resolvedSource.isEmpty(); }, 8000));
    SITE_CHECK(deepPublications == 0 && resolvedSource.path() == "/animals/invertebrates");
    QObject::disconnect(deepConnection);
    QObject::disconnect(sourceConnection);
    SITE_CHECK(explorer->begin(QUrl("https://fixture.example.org/")));
    explorer->pause();
    const auto group = explorer->crawler()->world().sections.first().id;
    SITE_CHECK(explorer->activate("site:" + group));
    waitUntil([]() { return false; }, 150);
    explorer->overview();
    waitUntil([]() { return false; }, 150);
    const QString capture = qEnvironmentVariable("DOCKPIPE_SITE_CAPTURE");
    if (!capture.isEmpty()) {
        const auto picture = QGuiApplication::platformName() == "offscreen" ? shell.grab() : shell.screen()->grabWindow(shell.winId());
        SITE_CHECK(picture.save(capture + ".png"));
    }
#ifdef DOCKPIPE_SPATIAL_WEB
    if (QGuiApplication::platformName() != "offscreen") {
        shell.hide();
        SpatialWebBrowser browser(true, nullptr, &fixture);
        browser.resize(1400, 950);
        browser.show();
        int publications = 0;
        QObject::connect(&browser, &SpatialWebBrowser::spaceReady, [&]() { ++publications; });
        browser.navigate(QUrl("https://fixture.example.org/"));
        SITE_CHECK(waitUntil([&]() { return browser.spatial() && !browser.building(); }, 10000));
        SITE_CHECK(browser.document().stableLayout && browser.document().siteOverview);
        const auto &siteWorld = browser.siteExplorer()->crawler()->world();
        const QString section = siteWorld.sections.first().id;
        const QString source = siteWorld.pages.first().node.id;
        const auto walkThrough = [&](const QString &id) {
            const auto room = browser.document().room();
            const auto found = std::find_if(room.displays.cbegin(), room.displays.cend(), [&](const auto &display) {
                return display.id == id;
            });
            if (found == room.displays.cend())
                return false;
            const float angle = found->yaw * 3.14159265358979323846f / 180;
            const QVector3D forward(-std::sin(angle), 0, std::cos(angle));
            browser.activateItem(id);
            if (!browser.transitioning())
                return false;
            // Site fixtures are cached; the first two sections are enough.
            const auto arrival = found->portalCenter + forward * 960;
            browser.scene()->setSurfaceSpawn({arrival.x(), arrival.z()});
            return waitUntil([&]() { return !browser.transitioning(); }, 3000);
        };
        SITE_CHECK(walkThrough("site:" + section));
        SITE_CHECK(!browser.document().siteOverview && browser.document().stableLayout);
        SITE_CHECK(walkThrough("source:" + source));
        SITE_CHECK(browser.document().items.first().kind == "excerpt");
        browser.siteExplorer()->overview();
        SITE_CHECK(waitUntil([&]() { return !browser.building(); }, 3000));
        SITE_CHECK(browser.document().siteOverview && publications >= 4);
        // Serve the source locally; the fixture router never contacts the public origin.
        QTcpServer liveServer;
        SITE_CHECK(liveServer.listen(QHostAddress::LocalHost, 0));
        QObject::connect(&liveServer, &QTcpServer::newConnection, [&]() {
            auto *socket = liveServer.nextPendingConnection();
            QObject::connect(socket, &QTcpSocket::readyRead, socket, [socket]() {
                socket->readAll();
                const auto html = article("Fixture live page", "Welcome");
                socket->write("HTTP/1.1 200 OK\r\nContent-Type: text/html\r\nContent-Length: "
                    + QByteArray::number(html.size()) + "\r\nConnection: close\r\n\r\n" + html);
                socket->disconnectFromHost();
            });
            QObject::connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
        });
        FixturePageRouter router;
        router.target = QUrl(QString("http://127.0.0.1:%1/").arg(liveServer.serverPort()));
        auto *live = browser.findChild<QWebEngineView *>("spatialWebLivePage");
        SITE_CHECK(live);
        live->page()->profile()->setUrlRequestInterceptor(&router);
        bool liveReady = false;
        QObject::connect(live, &QWebEngineView::loadFinished, [&](bool success) { liveReady = success; });
        const auto savedPosition = browser.scene()->cameraPosition();
        const auto savedIdentity = browser.document().environmentKey();
        browser.showPage();
        SITE_CHECK(waitUntil([&]() { return liveReady; }, 5000));
        SITE_CHECK(!browser.spatial() && live->url().host() == "127.0.0.1");
        browser.showSpace();
        SITE_CHECK(browser.spatial() && browser.document().environmentKey() == savedIdentity);
        SITE_CHECK(browser.scene()->cameraPosition() == savedPosition && browser.siteExplorer()->active());
        waitUntil([]() { return false; }, 150);
        if (!capture.isEmpty())
            SITE_CHECK(browser.screen()->grabWindow(browser.winId()).save(capture + "-browser.png"));
        browser.hide();
        live->page()->profile()->setUrlRequestInterceptor(nullptr);
        SITE_CHECK(!browser.siteExplorer()->crawler()->running());
        std::puts("Native site browser: immediate cached entrance, sections, source excerpts, overview and exit cancellation passed.");
    }
#endif
    std::puts("Site worlds: five semantic content types, robots, nested/malformed sitemaps, filtering, overrides, duplicates, stable layout/cache, progressive crawl, cancellation, SSRF policy and explorer rendering passed.");
    return 0;
}
