#include "SpatialSiteExplorer.h"
#include <QCheckBox>
#include <QHBoxLayout>
#include <QVBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSpinBox>
#include <QTreeWidget>
#include <QHeaderView>
#include <QSignalBlocker>
#include <QBuffer>
#include <QImageReader>

SpatialSiteExplorer::SpatialSiteExplorer(QWidget *parent, SpatialSiteFetch *transport) : QWidget(parent)
{
    setObjectName("spatialSiteExplorer");
    m_crawler = new SpatialSiteCrawler(this, transport);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *toolbar = new QHBoxLayout;
    m_enabled = new QCheckBox(tr("Site-wide"));
    m_enabled->setObjectName("spatialSiteEnabled");
    m_enabled->setChecked(true);
    m_enabled->setToolTip(tr("Discover public pages anonymously. Single-page mode remains available."));
    toolbar->addWidget(m_enabled);
    auto add = [this](QHBoxLayout *row, const QString &text, auto action) {
        auto *button = new QPushButton(text);
        row->addWidget(button);
        connect(button, &QPushButton::clicked, this, action);
        return button;
    };
    add(toolbar, tr("World overview"), &SpatialSiteExplorer::overview)->setObjectName("spatialSiteOverview");
    auto *map = add(toolbar, tr("Map / discovery"), [this]() { m_details->setVisible(!m_details->isVisible()); });
    map->setObjectName("spatialSiteMap");
    add(toolbar, tr("← Area"), [this]() { enter(m_section, qMax(0, m_areaPage - 1)); });
    add(toolbar, tr("Area →"), [this]() { enter(m_section, qMin(m_crawler->world().areaPages(m_section) - 1, m_areaPage + 1)); });
    m_update = add(toolbar, tr("Update area"), [this]() { enter(m_section, m_areaPage); });
    m_breadcrumb = new QLabel(tr("Public site discovery · single-page fallback"));
    m_breadcrumb->setTextFormat(Qt::PlainText);
    toolbar->addWidget(m_breadcrumb, 1);
    layout->addLayout(toolbar);
    m_details = new QWidget;
    auto *details = new QVBoxLayout(m_details);
    details->setContentsMargins(0, 0, 0, 0);
    auto *controls = new QHBoxLayout;
    auto number = [controls](const QString &name, int value, int maximum) {
        controls->addWidget(new QLabel(name));
        auto *spin = new QSpinBox;
        spin->setRange(0, maximum);
        spin->setValue(value);
        controls->addWidget(spin);
        return spin;
    };
    m_pages = number(tr("Pages"), 32, 64);
    m_pages->setMinimum(1);
    m_depth = number(tr("Depth"), 3, 6);
    m_score = number(tr("Relevance"), 45, 100);
    add(controls, tr("Discover / refresh"), [this]() {
        if (!m_crawler->world().origin.isEmpty())
            m_crawler->start(m_crawler->world().origin, budget(), true);
    });
    add(controls, tr("Stop"), [this]() { pause(); });
    add(controls, tr("Clear world cache"), [this]() {
        pause();
        QString error;
        if (!m_crawler->world().clear(error))
            m_status->setText(error);
        else {
            m_images.clear();
            m_enteredContent = false;
            updateMap();
            overview();
        }
    });
    details->addLayout(controls);
    m_search = new QLineEdit;
    m_search->setPlaceholderText(tr("Find a section, topic, source URL or exclusion reason"));
    m_search->setMaxLength(180);
    details->addWidget(m_search);
    m_tree = new QTreeWidget;
    m_tree->setObjectName("spatialSiteTree");
    m_tree->setHeaderLabels({tr("Site hierarchy / discovered sources"), tr("State"), tr("Score / depth")});
    m_tree->header()->setSectionResizeMode(0, QHeaderView::Stretch);
    m_tree->setMaximumHeight(230);
    details->addWidget(m_tree);
    m_inspect = new QLabel;
    m_inspect->setTextFormat(Qt::PlainText);
    m_inspect->setWordWrap(true);
    m_inspect->setMaximumHeight(100);
    details->addWidget(m_inspect);
    auto *actions = new QHBoxLayout;
    add(actions, tr("Enter / read"), &SpatialSiteExplorer::showSelected);
    add(actions, tr("Original page"), [this]() {
        if (const auto *page = m_crawler->world().find(selectedId()))
            emit sourceRequested(page->node.url);
    });
    for (const auto &choice : QList<QPair<QString, SpatialSitePage::Override>>{{tr("Include"), SpatialSitePage::Override::Include},
        {tr("Exclude"), SpatialSitePage::Override::Exclude}, {tr("Automatic"), SpatialSitePage::Override::Automatic}}) {
        add(actions, choice.first, [this, choice]() {
            const QString id = selectedId();
            if (!id.startsWith("section:")) {
                m_crawler->setOverride(id, choice.second);
                return;
            }
            // Apply to this section's discovered sources without triggering a crawl per source.
            const auto pages = m_crawler->world().pages;
            for (const auto &page : pages) {
                if (page.group == id.mid(8))
                    m_crawler->world().setOverride(page.node.id, choice.second);
            }
            m_crawler->save();
            updateMap();
        });
    }
    add(actions, tr("Refresh source"), [this]() { m_crawler->request(selectedId(), true); });
    details->addLayout(actions);
    m_status = new QLabel;
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    details->addWidget(m_status);
    layout->addWidget(m_details);
    m_details->hide();
    connect(m_crawler, &SpatialSiteCrawler::imageReady, this, [this](const QUrl &url, const QByteArray &bytes) {
        QBuffer buffer;
        buffer.setData(bytes);
        buffer.open(QIODevice::ReadOnly);
        QImageReader reader(&buffer);
        const auto size = reader.size();
        if (!QList<QByteArray>{"png", "jpeg", "webp", "gif"}.contains(reader.format())
            || !size.isValid() || size.width() > 2048 || size.height() > 2048)
            return;
        const auto image = reader.read().scaled(512, 320, Qt::KeepAspectRatio, Qt::SmoothTransformation);
        if (image.isNull())
            return;
        m_images.insert(url.toString(), new QImage(image));
        for (const auto &item : m_published.items) {
            if (item.imageUrl == url)
                emit textureReady(item.id, image);
        }
    });
    connect(this, &SpatialSiteExplorer::areaReady, this, [this](const SpatialWebDocument &document) {
        const quint64 generation = ++m_areaGeneration;
        QTimer::singleShot(0, this, [this, document, generation]() {
            if (generation != m_areaGeneration || !isVisible() || !m_active)
                return;
            m_published = document;
            for (const auto &item : document.items) {
                if (item.imageUrl.isEmpty())
                    continue;
                if (const auto *image = m_images.object(item.imageUrl.toString()))
                    emit textureReady(item.id, *image);
                else
                    m_crawler->requestImage(item.imageUrl);
            }
        });
    });
    connect(m_crawler, &SpatialSiteCrawler::changed, this, &SpatialSiteExplorer::updateMap);
    connect(m_crawler, &SpatialSiteCrawler::stopped, this, [this]() {
        if (m_active && m_crawler->world().sections.isEmpty() && isVisible()) {
            m_active = false;
            emit singlePageRequested();
        }
    });
    connect(m_search, &QLineEdit::textChanged, this, &SpatialSiteExplorer::updateMap);
    connect(m_score, &QSpinBox::valueChanged, this, [this](int value) {
        m_crawler->world().threshold = value;
        m_crawler->world().classifyAll();
        m_crawler->save();
        updateMap();
    });
    connect(m_tree, &QTreeWidget::itemActivated, this, [this]() { showSelected(); });
    connect(m_tree, &QTreeWidget::itemSelectionChanged, this, [this]() {
        const auto &world = m_crawler->world();
        const QString id = selectedId();
        const auto *page = world.find(id);
        const QString section = page ? page->group : id.mid(8);
        const auto area = world.project(section);
        const auto features = world.areaFeatures.value(area.layoutIdentity);
        QString design = tr("Layout: deterministic construction; no cached neural suggestion.");
        if (features.size() == 8) {
            const QStringList labels{tr("woodland"), tr("stonework"), tr("orbital"), tr("library"),
                tr("retail"), tr("workshop"), tr("driving"), tr("carnival")};
            QStringList scores;
            for (int index = 0; index < features.size(); ++index)
                scores.append(labels[index] + " " + QString::number(features[index], 'f', 2));
            design = tr("Cached layout features: ") + scores.join(", ");
        }
        if (!page) {
            m_inspect->setText(id.startsWith("section:") ? area.title + "\n" + design : QString());
            return;
        }
        m_inspect->setText(page->node.url.toDisplayString() + "\n" + page->reason
            + tr(" · Rule-based classification: ") + page->kind
            + "\nCanonical: " + page->canonical.toDisplayString() + " · Fetched: " + page->fetched.toString(Qt::ISODate)
            + (page->duplicateOf.isEmpty() ? QString() : " · Duplicate group: " + page->duplicateOf)
            + "\n" + design);
    });
    connect(m_enabled, &QCheckBox::toggled, this, [this](bool enabled) {
        if (!enabled) {
            pause();
            m_active = false;
            emit singlePageRequested();
        } else if (!m_crawler->world().origin.isEmpty()) {
            begin(m_crawler->world().origin);
        }
    });
}
bool SpatialSiteExplorer::enabled() const { return m_enabled->isChecked(); }
SpatialSiteBudget SpatialSiteExplorer::budget() const
{
    SpatialSiteBudget result;
    result.pages = m_pages->value();
    result.depth = m_depth->value();
    return result;
}
bool SpatialSiteExplorer::begin(const QUrl &url)
{
    ++m_areaGeneration;
    m_crawler->cancel();
    if (!enabled() || !SpatialSitePolicy::restriction(url, url).isEmpty()) {
        m_active = false;
        return false;
    }
    m_active = false;
    m_section.clear();
    m_initialSource = url.path().isEmpty() || url.path() == "/" ? QString()
        : SpatialSitePolicy::identity(SpatialSitePolicy::normalize(url).toString());
    m_areaPage = 0;
    m_enteredContent = false;
    m_images.clear();
    m_crawler->start(url, budget());
    m_active = true;
    const QSignalBlocker scoreBlocker(m_score);
    m_score->setValue(m_crawler->world().threshold);
    if (!m_initialSource.isEmpty())
        updateMap();
    // A deep link must resolve to its requested source before any scene is
    // published. Otherwise a cached overview can open the corridor prematurely.
    if (m_active && m_initialSource.isEmpty())
        enter({}, 0);
    return true;
}
void SpatialSiteExplorer::pause()
{
    ++m_areaGeneration;
    m_crawler->cancel();
    m_crawler->save();
    m_status->setText(tr("Discovery paused; cached world retained."));
}
void SpatialSiteExplorer::resumeArea()
{
    m_active = true;
}
void SpatialSiteExplorer::leaveArea()
{
    ++m_areaGeneration;
    m_active = false;
}
void SpatialSiteExplorer::overview()
{
    m_initialSource.clear();
    if (!m_crawler->world().origin.isEmpty())
        enter({}, 0);
}
void SpatialSiteExplorer::enter(const QString &section, int page)
{
    auto document = m_crawler->world().project(section, page);
    if (document.url.isEmpty())
        return;
    m_active = true;
    m_section = section;
    m_areaPage = page;
    m_enteredContent = !document.items.isEmpty();
    if (document.items.isEmpty()) {
        SpatialWebItem item;
        item.id = "site:waiting";
        item.title = m_crawler->running() ? tr("Discovering this site") : tr("No eligible content yet");
        item.description = tr("Use Map / discovery to inspect sources, change limits or include content. Single-page mode is available.");
        item.kind = "notice";
        item.spatialSlot = 0;
        document.items.append(item);
    }
    decorate(document);
    m_breadcrumb->setText(document.title + tr(" · area %1/%2").arg(page + 1).arg(m_crawler->world().areaPages(section)));
    m_update->setText(tr("Update area"));
    emit areaReady(document);
    m_crawler->prefetch(section);
}
bool SpatialSiteExplorer::activate(const QString &id)
{
    if (id.startsWith("site:")) {
        if (id == "site:waiting")
            m_details->show();
        else
            enter(id.mid(5));
        return true;
    }
    if (id.startsWith("source:")) {
        const auto *page = m_crawler->world().find(id.mid(7));
        if (page) {
            if (page->status != SpatialSitePage::Status::Ready) {
                m_details->show();
                m_status->setText(page->reason);
                if (page->status == SpatialSitePage::Status::Discovered)
                    m_crawler->request(page->node.id);
                return true;
            }
            SpatialWebDocument document;
            document.url = page->node.url;
            document.title = page->node.title;
            document.venue = page->kind == "product" ? "store" : "library";
            document.evidence = page->section + " " + page->headings.join(' ');
            document.stableLayout = true;
            document.layoutIdentity = m_crawler->world().origin.toString() + ":source:" + page->node.id;
            for (int offset = 0; offset < page->summary.size() && document.items.size() < 4; offset += 300) {
                SpatialWebItem item;
                item.id = "read:" + page->node.id + ":" + QString::number(offset);
                item.title = tr("Source excerpt · %1").arg(document.items.size() + 1);
                item.description = page->summary.mid(offset, 300);
                item.detail = page->node.url.host();
                item.url = page->node.url;
                item.kind = "excerpt";
                item.category = page->section;
                item.spatialSlot = document.items.size();
                document.items.append(item);
            }
            for (const auto &link : page->links) {
                const auto *related = m_crawler->world().find(SpatialSitePolicy::identity(SpatialSitePolicy::normalize(link.url).toString()));
                if (!related || related->node.id == page->node.id || related->status != SpatialSitePage::Status::Ready || document.items.size() >= 12)
                    continue;
                SpatialWebItem item;
                item.id = "source:" + related->node.id;
                item.title = related->node.title;
                item.description = related->summary.left(200);
                item.url = related->node.url;
                item.kind = "related";
                item.category = related->section;
                item.spatialSlot = document.items.size();
                document.items.append(item);
            }
            if (!document.items.isEmpty()) {
                m_active = true;
                m_section = page->group;
                m_breadcrumb->setText(m_crawler->world().title + " / " + page->section + " / " + page->node.title);
                decorate(document);
                emit areaReady(document);
            }
            m_crawler->request(page->node.id);
        }
        return true;
    }
    if (id.startsWith("read:")) {
        const auto *page = m_crawler->world().find(id.mid(5).section(':', 0, 0));
        if (page)
            emit sourceRequested(page->node.url);
        return true;
    }
    return false;
}
QString SpatialSiteExplorer::selectedId() const
{
    return m_tree->currentItem() ? m_tree->currentItem()->data(0, Qt::UserRole).toString() : QString();
}
void SpatialSiteExplorer::showSelected()
{
    const QString id = selectedId();
    if (id.startsWith("section:"))
        enter(id.mid(8));
    else if (const auto *page = m_crawler->world().find(id)) {
        if (!page->group.isEmpty())
            enter(page->group, qMax(0, page->slot / 12));
        activate("source:" + id);
    }
}
void SpatialSiteExplorer::updateMap()
{
    const QString selected = selectedId();
    const QSignalBlocker blocker(m_tree);
    m_tree->clear();
    QMap<QString, QTreeWidgetItem *> groups;
    const auto &world = m_crawler->world();
    for (const auto &section : world.sections) {
        auto *item = new QTreeWidgetItem(m_tree, {section.title, tr("Section"), {}});
        item->setData(0, Qt::UserRole, "section:" + section.id);
        groups[section.id] = item;
    }
    auto *other = new QTreeWidgetItem(m_tree, {tr("Pending / excluded / unavailable"), {}, {}});
    int accepted = 0;
    const QString query = m_search->text();
    for (const auto &page : world.pages) {
        accepted += page.status == SpatialSitePage::Status::Ready;
        if (!query.isEmpty() && !(page.node.title + page.node.url.toString() + page.reason + page.section).contains(query, Qt::CaseInsensitive))
            continue;
        auto *parent = groups.value(page.group, other);
        const QString state = QStringList{tr("Pending"), tr("Accepted"), tr("Excluded"), tr("Unavailable")}[int(page.status)];
        auto *item = new QTreeWidgetItem(parent, {page.node.title.isEmpty() ? page.node.url.path() : page.node.title,
            state, QString("%1 / %2").arg(page.score).arg(page.depth)});
        item->setData(0, Qt::UserRole, page.node.id);
        item->setToolTip(0, page.node.url.toDisplayString() + "\n" + page.reason);
        if (page.node.id == selected)
            m_tree->setCurrentItem(item);
    }
    m_tree->expandToDepth(0);
    m_status->setText(QString("%1 · %2 discovered · %3 accepted · %4 requests · %5 KiB · cached slots retained")
        .arg(m_crawler->status()).arg(world.pages.size()).arg(accepted).arg(m_crawler->requests()).arg(m_crawler->bytes() / 1024));
    m_update->setText(tr("Update area · %1 sources").arg(accepted));
    if (m_active && !m_initialSource.isEmpty()) {
        const auto *requested = world.find(m_initialSource);
        if (requested && requested->status != SpatialSitePage::Status::Discovered) {
            const QUrl source = requested->node.url;
            m_initialSource.clear();
            m_enteredContent = true;
            pause();
            m_active = false;
            emit sourceRequested(source);
            return;
        }
    }
    if (m_active && m_initialSource.isEmpty() && !m_enteredContent && !world.sections.isEmpty())
        enter(m_section, m_areaPage);
}
void SpatialSiteExplorer::rememberDesign(const SpatialWebDocument &document)
{
    if (!document.stableLayout || document.worldFeatures.size() != 8)
        return;
    auto &features = m_crawler->world().areaFeatures;
    if (features.size() < 48 || features.contains(document.layoutIdentity))
        features[document.layoutIdentity] = document.worldFeatures;
    m_crawler->save();
}
void SpatialSiteExplorer::decorate(SpatialWebDocument &document) const
{
    document.worldFeatures = m_crawler->world().areaFeatures.value(document.layoutIdentity);
}
