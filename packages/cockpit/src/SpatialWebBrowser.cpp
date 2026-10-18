#include <QCryptographicHash>
#include "SpatialWebBrowser.h"
#include "SpatialSiteExplorer.h"
#include "SpatialWebWorld.h"
#include "SpatialCivicWorld.h"
#include <QUrlQuery>
#include "WorkspaceFlightView.h"
#include "SpatialWebMedia.h"
#include "SpatialNeuralService.h"
#include <QToolButton>
#include <QComboBox>
#include <QSignalBlocker>
#include <QFile>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QPointer>
#include <QPushButton>
#include <QStackedWidget>
#include <QStackedLayout>
#include <QVBoxLayout>
#ifdef DOCKPIPE_SPATIAL_WEB
#include "BrowserRuntime.h"
#include "BrowserWidget.h"
#include <QWebEngineHistory>
#include <QWebEngineLoadingInfo>
#include <QWebEnginePage>
#include <QWebEngineScript>
#include <QWebEngineView>

#endif

SpatialWebBrowser::SpatialWebBrowser(bool hardware, QWidget *parent, SpatialSiteFetch *siteTransport) : QWidget(parent), m_hardware(hardware)
{
    setObjectName("spatialWebBrowser");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *toolbar = new QHBoxLayout;
    auto *cockpit = new QPushButton(tr("Back to cockpit"));
    cockpit->setObjectName("spatialWebExit");
    connect(cockpit, &QPushButton::clicked, this, &SpatialWebBrowser::exitRequested);
    toolbar->addWidget(cockpit);
    auto *home = new QPushButton(tr("HQ"));
    home->setObjectName("spatialWebHome");
    home->setToolTip(tr("Return to Dockpipe Industries headquarters"));
    connect(home, &QPushButton::clicked, this, &SpatialWebBrowser::showHeadquarters);
    toolbar->addWidget(home);
    auto *search = new QPushButton(tr("Search city"));
    search->setObjectName("spatialWebSearch");
    connect(search, &QPushButton::clicked, this, &SpatialWebBrowser::showSearch);
    toolbar->addWidget(search);
    m_back = new QPushButton(tr("←"));
    m_back->setObjectName("spatialWebBack");
    m_back->setAccessibleName(tr("Browser back"));
    m_forward = new QPushButton(tr("→"));
    m_forward->setObjectName("spatialWebForward");
    m_forward->setAccessibleName(tr("Browser forward"));
    toolbar->addWidget(m_back);
    toolbar->addWidget(m_forward);
    m_address = new QLineEdit;
    m_address->setObjectName("spatialWebAddress");
    m_address->setPlaceholderText(tr("Search the city or enter a website address"));

    m_address->setMaxLength(4096);
    toolbar->addWidget(m_address, 1);
    auto *go = new QPushButton(tr("Go"));
    m_go = go;
    go->setObjectName("spatialWebGo");
    toolbar->addWidget(go);
    auto *space = new QPushButton(tr("Build space"));
    m_buildSpace = space;
    space->setObjectName("spatialWebBuild");
    toolbar->addWidget(space);
    auto *page = new QPushButton(tr("Live page"));
    m_pageToggle = page;
    page->setObjectName("spatialWebPage");
    toolbar->addWidget(page);
    auto *worldTools = new QHBoxLayout;
    m_mobility = new QComboBox;
    m_mobility->setObjectName("spatialWebMobility");
    m_mobility->addItem(tr("Walk"), "walk");
    m_mobility->addItem(tr("Pilot ship"), "fly");
    m_mobility->addItem(tr("Drive"), "drive");
    m_mobility->setEnabled(false);
    worldTools->addWidget(m_mobility);
    m_layout = new QComboBox;
    m_layout->setObjectName("spatialWebLayout");
    m_layout->setToolTip(tr("Rebuild this content with a different arrangement"));
    m_layout->addItems({tr("Site layout"), tr("Branching paths"), tr("Loop"), tr("Promenade"), tr("Flight network")});
    m_layout->setEnabled(false);
    worldTools->addWidget(m_layout);
    connect(m_layout, &QComboBox::currentIndexChanged, this, [this](int index) {
        m_autoDesign = false;
        if (m_passage || m_building || m_document.items.isEmpty() || m_document.venue == "cinema" || m_document.venue == "city" || m_headquarters)
            return;
        auto options = SpatialWebWorld::optionsFor(m_document);
        if (index > 0)
            options.topology = static_cast<SpatialTopology>(index - 1);
        if (index == 4)
            options.style.outdoors = true;
        const auto result = SpatialWebWorld::construct(m_document, options);
        if (!result.success) {
            setStatus(result.diagnostic);
            return;
        }
        m_currentInterior = result.interior;
        m_currentInterior.environmentKey = m_document.environmentKey();
        m_scene->setInterior(result.interior, m_document.title, false, false, m_document.environmentKey());
        m_scene->setShipVisible(result.interior.locomotion == "fly");
        const QSignalBlocker blocker(m_mobility);
        m_mobility->setCurrentIndex(m_mobility->findData(result.interior.locomotion));
        for (const auto &item : m_document.items)
            m_scene->setInteriorImage(item.id, item.poster);
        setStatus(result.fallback ? result.diagnostic : tr("World rearranged. Your destinations and links are preserved."));
        m_scene->setFocus();
    });
    m_pause = new QPushButton(tr("Play / pause"));
    m_pause->setObjectName("spatialWebPause");
    m_pause->hide();
    worldTools->addWidget(m_pause);
    m_stopMedia = new QPushButton(tr("Stop film"));
    m_stopMedia->setObjectName("spatialWebStopFilm");
    m_stopMedia->hide();
    worldTools->addWidget(m_stopMedia);
    configureIntelligence(worldTools);
    worldTools->addStretch();
    layout->addLayout(toolbar);
    layout->addLayout(worldTools);
    m_site = new SpatialSiteExplorer(this, siteTransport);
    layout->addWidget(m_site);
    m_pages = new QStackedWidget;
    // Chromium must render to supply image pixels, but it stays behind the
    // opaque welcome/world surface. Only an explicit Live page action raises it.
    qobject_cast<QStackedLayout *>(m_pages->layout())->setStackingMode(QStackedLayout::StackAll);
    layout->addWidget(m_pages, 1);
    // The scene is hidden while Chromium loads. Defer its OpenGL viewport until
    // it can actually initialize; a hidden startup probe otherwise fails early.
    m_scene = new WorkspaceFlightView({}, false, this);
    m_scene->setShipVisible(false);
    m_scene->destinationActivated = [this](const QString &path, bool) {
        if (path == "civic:hq") {
            showHeadquarters();

        } else if (path == "civic:search") {
            showSearch();
        } else if (path.startsWith("civic:")) {
            navigate(QUrl(path.mid(6)), false);
        } else if (path.startsWith("display:")) {
            activateItem(path.mid(8));
        }
    };
    m_scene->entranceApproached = [this](const QString &id) {
        activateItem(id);
        return true;
    };
    m_scene->orbitRequested = [this]() {
        if (m_passage) {
            cancelPassage();
            return;
        }
        if (m_headquarters || m_scene->interiorLocomotion() == "fly") {
            m_scene->leaveInteriorAction();
            emit exitRequested();
        } else if (m_site->active()) {
            m_site->overview();
        } else {
            showHeadquarters();
        }
    };
    m_scene->exitRequested = m_scene->orbitRequested;
    m_scene->locomotionChanged = [this]() {
        const QSignalBlocker blocker(m_mobility);
        m_mobility->setCurrentIndex(m_mobility->findData(m_scene->interiorLocomotion()));
    };
    connect(m_mobility, &QComboBox::currentIndexChanged, this, [this]() {
        m_autoDesign = false;
        m_scene->setInteriorLocomotion(m_mobility->currentData().toString());
        m_scene->setFocus();
    });
    m_passageTick.setInterval(80);
    connect(&m_passageTick, &QTimer::timeout, this, &SpatialWebBrowser::updatePassage);
    m_passageDeadline.setParent(this);
    m_passageDeadline.setObjectName("spatialPassageDeadline");
    m_passageDeadline.setSingleShot(true);
    m_passageDeadline.setInterval(30000);
    connect(&m_passageDeadline, &QTimer::timeout, this, &SpatialWebBrowser::failPassage);
    // One short repaint-only transition; no second renderer, screen capture or
    // continuously running effect. The old world stays intact during loading.
    m_phase.setStartValue(0.0);
    m_phase.setEndValue(1.0);
    m_phase.setDuration(420);
    connect(&m_phase, &QVariantAnimation::valueChanged, this, [this](const QVariant &value) {
        const qreal progress = value.toReal();
        if (progress >= 0.4 && m_pendingDocument) {
            const SpatialWebDocument next = *m_pendingDocument;
            m_pendingDocument.reset();
            publishDocument(next, m_generation);
        }
        const qreal opacity = progress < 0.4 ? progress / 0.4 : (1.0 - progress) / 0.6;
        m_scene->setInteriorPhase(opacity);
    });
    connect(&m_phase, &QVariantAnimation::finished, this, [this]() {
        m_scene->setInteriorPhase(0);
        m_scene->setEditorActive(false);
        if (isVisible() && spatial())
            m_scene->setFocus();
    });
    m_pages->addWidget(m_scene);
    m_start = new QWidget;
    m_start->setObjectName("spatialWebStart");
    m_start->setStyleSheet("#spatialWebStart { background: #101e2c; color: #e1f3f0; border: 1px solid #285667; }");
    auto *welcome = new QVBoxLayout(m_start);
    welcome->setContentsMargins(40, 40, 40, 40);
    welcome->setSpacing(24);
    welcome->addStretch();
    auto *heading = new QLabel(tr("Step into the web"));
    heading->setAlignment(Qt::AlignCenter);
    heading->setStyleSheet("font-size: 28px; font-weight: 600; color: #72d9cb;");
    welcome->addWidget(heading);
    auto *introduction = new QLabel(tr("Open a website and explore its content as a 3D space."));
    introduction->setAlignment(Qt::AlignCenter);
    introduction->setWordWrap(true);
    welcome->addWidget(introduction);
    auto *explore = new QPushButton(tr("Explore GitHub repositories"));
    explore->setObjectName("spatialWebExplore");
    explore->setMinimumHeight(48);
    explore->setEnabled(false);
    welcome->addWidget(explore, 0, Qt::AlignHCenter);
    auto *instructions = new QLabel(tr("Or enter a website above and press Go.\nWalk with WASD and look with right-drag. Live page opens the website's original controls."));
    instructions->setAlignment(Qt::AlignCenter);
    instructions->setWordWrap(true);
    welcome->addWidget(instructions);
    welcome->addStretch();
    m_pages->addWidget(m_start);
    m_pages->setCurrentWidget(m_start);
    m_scene->hide();
    space->setEnabled(false);
    page->setEnabled(false);
    connect(explore, &QPushButton::clicked, this, [this]() {
        navigate(QUrl("https://github.com/topics/game-engine"));
    });
    m_status = new QLabel;
    m_status->setObjectName("spatialWebStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    configureSearch();
    const auto loadAddress = [this]() {
        const auto text = m_address->text().trimmed();
        if (text.isEmpty())
            return;
        if (!text.contains("://") && (text.contains(' ') || !text.contains('.'))) {
            QUrl search("https://www.google.com/search");
            QUrlQuery query;
            query.addQueryItem("q", text);
            search.setQuery(query);
            navigate(search, false);
        } else {
            navigate(QUrl::fromUserInput(text));
        }
    };
    connect(go, &QPushButton::clicked, this, loadAddress);
    connect(m_address, &QLineEdit::returnPressed, this, loadAddress);
    connect(space, &QPushButton::clicked, this, [this]() {
        cancelPassage();
        buildSpace();
    });
    connect(page, &QPushButton::clicked, this, [this]() {
        if (m_liveMode && m_scene->walking())
            showSpace();
        else
            showPage();
    });
    m_refresh.setSingleShot(true);
    m_refresh.setInterval(650);
    connect(&m_refresh, &QTimer::timeout, this, &SpatialWebBrowser::buildSpace);
    m_loadProbe.setSingleShot(true);
    m_loadProbe.setInterval(1000);
    connect(&m_loadProbe, &QTimer::timeout, this, &SpatialWebBrowser::probeLoadedContent);
    m_deadline.setSingleShot(true);
    m_deadline.setInterval(15000);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        cancelProjection();
        freezeBackgroundBrowser();
        failPassage();
        setStatus(tr("World preparation timed out. The current view is preserved; try Build space again or open Live page."));
    });
#ifdef DOCKPIPE_SPATIAL_WEB
    const QString rendererError = BrowserRuntime::rendererError();
    if (!rendererError.isEmpty()) {
        m_site->hide();
        go->setEnabled(false);
        space->setEnabled(false);
        page->setEnabled(false);
        m_back->setEnabled(false);
        m_forward->setEnabled(false);
        setStatus(rendererError);
        return;
    }
    m_browserUi = new BrowserWidget;
    m_browserUi->setObjectName("spatialWebBrowserTools");
    // The whole widget must remain visible behind the world for Chromium to
    // render. Fullscreen must not treat this capture source as ordinary chrome.
    m_browserUi->setProperty("flightRenderSource", true);
    m_pages->addWidget(m_browserUi);
    connect(m_browserUi, &BrowserWidget::activeViewAboutToChange,
        this, &SpatialWebBrowser::detachBrowserView);
    connect(m_browserUi, &BrowserWidget::activeViewChanged,
        this, &SpatialWebBrowser::bindBrowserView);
    bindBrowserView(m_browserUi->activeView());
    m_browserUi->hide();
    m_start->raise();
    connect(m_stopMedia, &QPushButton::clicked, this, [this]() {
        if (m_media)
            m_media->stop();
        freezeBackgroundBrowser();
        setStatus(tr("Film stopped. Choose another poster to play on the centre screen."));
    });
    explore->setEnabled(true);
    connect(m_back, &QPushButton::clicked, this, [this]() {
        if (m_passage) {
            cancelPassage();
            return;
        }
        if (m_site->active()) {
            m_site->overview();
            return;
        }
        if (!m_browser)
            return;
        m_media->stop();
        cancelProjection();
        if (!m_liveMode) {
            m_preserveWorld = false;
            prepareBackgroundBrowser();
        }
        m_browser->back();
    });
    connect(m_forward, &QPushButton::clicked, this, [this]() {
        cancelPassage();
        if (!m_browser)
            return;
        m_media->stop();
        cancelProjection();
        if (!m_liveMode) {
            m_preserveWorld = false;
            prepareBackgroundBrowser();
        }
        m_browser->forward();
    });
    QTimer::singleShot(0, this, [this]() {
        if (!m_loading && !m_headquarters && m_document.url.isEmpty() && !m_site->active())
            showHeadquarters();
    });
    setStatus(tr("Load a website to enter its space. Links become exhibits using the page's own rendered visuals. This is a private browser session."));
#else
    m_site->hide();
    go->setEnabled(false);
    space->setEnabled(false);
    page->setEnabled(false);
    setStatus(tr("Spatial browsing requires a build with Qt WebEngine Widgets. The other cockpit destinations remain available."));
#endif
    connect(m_site, &SpatialSiteExplorer::areaReady, this, [this](const SpatialWebDocument &document) {
        if (!isVisible())
            return;
        if (m_passage && !m_followingEntrance && m_arrivalDocument
            && document.layoutIdentity != m_arrivalDocument->layoutIdentity)
            cancelPassage();
        if (m_media)
            m_media->stop();
        cancelProjection();
        m_liveMode = false;
        m_preserveWorld = true;
        m_building = true;
        publishDocument(document, m_generation);
    });
    connect(m_site, &SpatialSiteExplorer::textureReady, this, [this](const QString &id, const QImage &image) {
        if (m_passage && m_arrivalDocument) {
            for (int index = 0; index < m_arrivalDocument->items.size(); ++index) {
                auto &item = m_arrivalDocument->items[index];
                if (item.id == id) {
                    item.visual = image;
                    item.poster = SpatialWebDocument::poster(item, image, index);
                    m_scene->setInteriorImage("arrival:" + id, item.poster);
                }
            }
            return;
        }
        if (!m_document.stableLayout)
            return;
        for (int index = 0; index < m_document.items.size(); ++index) {
            auto &item = m_document.items[index];
            if (item.id != id)
                continue;
            item.visual = image;
            item.poster = SpatialWebDocument::poster(item, image, index);
            m_scene->setInteriorImage(id, item.poster);
            if (m_enhance->isChecked() && item.kind == "product")
                m_neural->cutout(item.id, item.visual);
        }
    });
    connect(m_site, &SpatialSiteExplorer::sourceRequested, this, [this](const QUrl &url) {
#ifdef DOCKPIPE_SPATIAL_WEB
        if (!m_browser)
            return;
        m_site->leaveArea();
        cancelProjection();
        m_preserveWorld = false;
        prepareBackgroundBrowser();
        m_browser->load(url);
#endif
    });
    connect(m_site, &SpatialSiteExplorer::singlePageRequested, this, [this]() {
#ifdef DOCKPIPE_SPATIAL_WEB
        if (!m_browser)
            return;
        const QUrl source = m_document.url.isEmpty() || m_document.stableLayout
            ? QUrl::fromUserInput(m_address->text()) : m_document.url;
        m_site->leaveArea();
        if (SpatialWebDocument::allowedUrl(source)) {
            cancelProjection();
            m_preserveWorld = false;
            prepareBackgroundBrowser();
            m_browser->load(source);
        }
#endif
    });
    m_back->setEnabled(false);
    m_forward->setEnabled(false);
}

SpatialWebBrowser::~SpatialWebBrowser()
{
    m_passageTick.stop();
    m_passageDeadline.stop();
    m_site->pause();
    cancelProjection();
    if (m_media) {
        m_media->stop();
        delete m_media;
        m_media = nullptr;
    }
#ifdef DOCKPIPE_SPATIAL_WEB
    // BrowserWidget owns all tabs, their shared profile and session cleanup.
    if (m_browserUi)
        disconnect(m_browserUi, nullptr, this, nullptr);
    delete m_browserUi;
#endif
}

void SpatialWebBrowser::detachBrowserView(QWebEngineView *view)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser || view != m_browser)
        return;
    cancelPassage();
    cancelProjection();
    m_site->pause();
    m_loading = false;
    m_preserveWorld = true;
    m_browserTabChanged = true;
    m_resumeTheatre = false;
    if (m_media) {
        m_media->stop();
        delete m_media;
        m_media = nullptr;
    }
    disconnect(m_browser, nullptr, this, nullptr);
    disconnect(m_browser->page(), nullptr, this, nullptr);
    m_browser->setObjectName("browserWebView");
    m_browser->page()->setAudioMuted(true);
    m_browserUi->hide();
    m_browser->hide();
    m_browser->page()->setVisible(false);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
    m_browser = nullptr;
    m_back->setEnabled(false);
    m_forward->setEnabled(false);
    m_buildSpace->setEnabled(false);
#else
    Q_UNUSED(view);
#endif
}

void SpatialWebBrowser::bindBrowserView(QWebEngineView *view)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser == view)
        return;
    if (m_browser)
        detachBrowserView(m_browser);
    m_browser = view;
    if (!m_browser) {
        updatePageToggle();
        return;
    }
    m_browser->setObjectName("spatialWebLivePage");
    m_browser->setProperty("flightRenderSource", true);
    m_media = new SpatialWebMedia(m_browser, this);
    connect(m_media, &SpatialWebMedia::frameReady, this, [this](const QImage &frame) {
        m_scene->setInteriorImage("cinema-screen", frame);
    });
    connect(m_media, &SpatialWebMedia::statusChanged, this, [this](const QString &status) {
        if (m_media->active() && !m_liveMode) {
            m_browserUi->show();
            m_browserUi->lower();
            m_scene->raise();
        }
        setStatus(status);
        m_scene->setFlightStatus(status);
    });
    connect(m_pause, &QPushButton::clicked, m_media, [this]() {
        if (!m_media->active())
            return;
        m_browserUi->setBackgroundSuspended(false);
        m_browserUi->show();
        m_browserUi->lower();
        m_media->togglePause();
    });
    connect(m_browser->page(), &QWebEnginePage::renderProcessTerminated, this,
        [this](QWebEnginePage::RenderProcessTerminationStatus, int) {
            cancelProjection();
            m_loading = false;
            failPassage();
            setStatus(tr("The website renderer stopped. Reload it in Live page."));
        });
    connect(m_browser, &QWebEngineView::loadStarted, this, [this]() {
        if (m_site->active() && !m_liveMode) {
            m_loading = true;
            return;
        }
        cancelProjection();
        m_loading = true;
        m_earlyProjection = false;
        m_loadAttempts = 0;
        m_loadProbe.start();
        m_buildSpace->setEnabled(false);
        updatePageToggle();
        if (!m_liveMode && !m_media->active())
            prepareBackgroundBrowser();
        setStatus(tr("Loading website in the background…"));
    });
    connect(m_browser, &QWebEngineView::urlChanged, this, [this](const QUrl &url) {
        if (m_site->active() && !m_liveMode)
            return;
        cancelProjection();
        m_address->setText(url.toDisplayString());
        m_back->setEnabled(m_browser->history()->canGoBack());
        m_forward->setEnabled(m_browser->history()->canGoForward());
        if (!m_loading)
            scheduleProjection();
        else
            m_loadProbe.start();
    });
    connect(m_browser->page(), &QWebEnginePage::loadingChanged, this, [this](const QWebEngineLoadingInfo &info) {
        if (info.status() == QWebEngineLoadingInfo::LoadStartedStatus || m_browser->page()->isLoading())
            return;
        // Leaving a usable article can abort its remaining scripts. That old
        // load is cancelled, not a failure of the next corridor destination.
        if (info.status() == QWebEngineLoadingInfo::LoadStoppedStatus) {
            m_loading = false;
            m_loadProbe.stop();
            updatePageToggle();
            return;
        }
        if (info.url() != m_browser->url() && info.url() != m_browser->page()->requestedUrl())
            return;
        const bool success = info.status() == QWebEngineLoadingInfo::LoadSucceededStatus;
        m_loading = false;
        m_loadProbe.stop();
        if (m_site->active() && !m_liveMode) {
            freezeBackgroundBrowser();
            return;
        }
        m_buildSpace->setEnabled(success && SpatialWebDocument::allowedUrl(m_browser->url()));
        updatePageToggle();
        if (m_media->active())
            return;
        if (m_earlyProjection) {
            m_earlyProjection = false;
            return;
        }
        if (success && SpatialWebDocument::allowedUrl(m_browser->url())) {
            if (!m_liveMode && !m_preserveWorld) {
                scheduleProjection();
            } else if (m_liveMode) {
                setStatus(m_scene->walking()
                    ? tr("Live page · Return to space resumes your existing world. Build space updates it from this page.")
                    : tr("Live page · Build space creates a spatial world from this page."));
            } else {
                freezeBackgroundBrowser();
            }
        } else {
            freezeBackgroundBrowser();
            failPassage();
            setStatus(tr("The website could not be loaded. Check the address or use the live page."));
        }
    });
    m_loading = m_browser->page()->isLoading();
    m_back->setEnabled(m_browser->history()->canGoBack());
    m_forward->setEnabled(m_browser->history()->canGoForward());
    m_address->setText(m_browser->url().toDisplayString());
    m_buildSpace->setEnabled(!m_loading && SpatialWebDocument::allowedUrl(m_browser->url()));
    if (m_liveMode) {
        m_browserUi->setBackgroundSuspended(false);
        m_browserUi->show();
        m_browser->setVisible(!m_browser->url().isEmpty());
        m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
        m_browser->page()->setAudioMuted(false);
    } else {
        freezeBackgroundBrowser();
    }
    updatePageToggle();
#else
    Q_UNUSED(view);
#endif
}

void SpatialWebBrowser::setStatus(const QString &text)
{
    m_status->setText(text);
    if (m_searchOpen && m_searchStatus)
        m_searchStatus->setText(text);
    // Keep loading and failure feedback visible while the world is fullscreen.
    if (m_scene && spatial())
        m_scene->setFlightStatus(text);
}
QString SpatialWebBrowser::status() const { return m_status->text(); }
bool SpatialWebBrowser::spatial() const { return m_pages->currentWidget() == m_scene; }
void SpatialWebBrowser::setHardwareAcceleration(bool enabled)
{
    m_hardware = enabled;
    if (m_scene->isVisible() || !enabled)
        m_scene->setFlightHardwareAcceleration(enabled);
}

void SpatialWebBrowser::cancelProjection()
{
    ++m_generation;
    m_passageCapturing = false;
    m_pendingCaptures.clear();
    m_captureClock.invalidate();
    m_autoDesign = false;
    if (m_neural && !m_neural->installing())
        m_neural->cancel();
    m_proposal = {};
    if (m_applyDesign)
        m_applyDesign->hide();
    const bool wasTransitioning = m_phase.state() == QAbstractAnimation::Running;
    m_phase.stop();
    m_pendingDocument.reset();
    m_scene->setInteriorPhase(0);
    if (wasTransitioning)
        m_scene->setEditorActive(false);
    m_building = false;
    m_refresh.stop();
    m_loadProbe.stop();
    m_deadline.stop();
}

void SpatialWebBrowser::prepareBackgroundBrowser()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser)
        return;
    m_liveMode = false;
    updatePageToggle();
    m_browserUi->setBackgroundSuspended(false);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->page()->setAudioMuted(true);
    m_browserUi->show();
    m_browser->show();
    QWidget *foreground = m_scene->walking() ? m_scene : m_start;
    if (m_searchOpen)
        foreground = m_search;
    m_pages->setCurrentWidget(foreground);
    if (foreground == m_start)
        m_scene->hide();
    else
        m_start->hide();
    m_browserUi->lower();
    foreground->raise();
#endif
}

void SpatialWebBrowser::freezeBackgroundBrowser()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser || m_liveMode)
        return;
    m_browserUi->setBackgroundSuspended(true);
    m_browserUi->hide();
    m_browser->hide();
    m_browser->page()->setVisible(false);
    m_browser->page()->setAudioMuted(true);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
#endif
}

void SpatialWebBrowser::navigate(const QUrl &url, bool discoverSite)
{
    if (!m_followingEntrance)
        cancelPassage();
    if (!SpatialWebDocument::allowedUrl(url)) {
        setStatus(tr("Enter an HTTP or HTTPS website address without embedded credentials."));
        return;
    }
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser)
        return;
    m_headquarters = false;
    m_browserTabChanged = false;
    if (!SpatialWebDocument::searchResultsUrl(url)) {
        m_searchOpen = false;
        m_search->hide();
    }
    m_media->stop();
    m_resumeTheatre = false;
    m_preserveWorld = false;
    cancelProjection();
    m_address->setText(url.toDisplayString());
    if (discoverSite && !SpatialWebDocument::searchResultsUrl(url) && m_site->begin(url))
        return;
    m_site->leaveArea();
    prepareBackgroundBrowser();
    // Chromium may reuse an unfinished same-URL navigation without emitting a
    // new loadStarted signal. A deliberate retry must not inherit its old
    // "already projected" completion state.
    m_earlyProjection = false;
    m_loading = true;
    m_loadAttempts = 0;
    m_loadProbe.start();
    m_browserUi->navigate(url);
#endif
}

void SpatialWebBrowser::showPage()
{
    if (m_headquarters)
        return;
    m_searchOpen = false;
    m_search->hide();
    cancelPassage();
    m_site->leaveArea();
    if (!m_liveMode) {
        m_resumeTheatre = !m_browserTabChanged && m_scene->walking() && m_document.venue == "cinema";
        if (m_media && m_media->active())
            m_media->releaseToPage();
    }
    m_preserveWorld = true;
    cancelProjection();
    m_scene->setFlightFullscreen(false);
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser)
        return;
    if (m_document.stableLayout && !m_browserTabChanged && m_browser->url() != m_document.url) {
        m_liveMode = true;
        m_browser->load(m_document.url);
    }
    if (!SpatialWebDocument::allowedUrl(m_browser->url()) && !m_document.stableLayout && !m_browserTabChanged) {
        m_liveMode = false;
        m_pages->setCurrentWidget(m_start);
        m_browserUi->hide();
        m_browser->hide();
        m_scene->hide();
        m_address->setFocus();
        return;
    }
    m_liveMode = true;
    updatePageToggle();
    setStatus(m_scene->walking()
        ? tr("Live page · Return to space resumes your existing world. Build space updates it from this page.")
        : tr("Live page · Build space creates a spatial world from this page."));
    m_browserUi->setBackgroundSuspended(false);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->page()->setAudioMuted(false);
    m_pages->setCurrentWidget(m_browserUi);
    m_browserUi->show();
    m_browser->setVisible(!m_browser->url().isEmpty());
    m_start->hide();
    m_scene->hide();
    m_browser->setFocus();
#endif
}

void SpatialWebBrowser::updatePageToggle()
{
    m_address->setVisible(!m_liveMode);
    m_go->setVisible(!m_liveMode);
    m_back->setVisible(!m_liveMode);
    m_forward->setVisible(!m_liveMode);
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browserUi)
        m_browserUi->setChromeVisible(m_liveMode);
    if (m_passage) {
        m_mobility->setEnabled(false);
        m_layout->setEnabled(false);
        m_back->setEnabled(true);
    }
    const bool canReturn = m_liveMode && m_scene->walking();
    if (m_liveMode) {
        m_mobility->setEnabled(false);
        m_layout->setEnabled(false);
        m_pause->setEnabled(false);
        m_stopMedia->setEnabled(false);
    } else if (m_scene->walking() && !m_building && !m_passage) {
        m_mobility->setEnabled(true);
        m_layout->setEnabled(m_document.venue != "cinema" && m_document.venue != "city" && !m_headquarters);
        m_pause->setEnabled(true);
        m_stopMedia->setEnabled(true);
    }
    m_pageToggle->setText(canReturn ? tr("Return to space") : tr("Live page"));
    m_pageToggle->setToolTip(canReturn
        ? tr("Resume the existing world without rebuilding or moving your camera")
        : tr("Use the website's original controls"));
    const bool hasBrowserPage = m_browser && (m_browserTabChanged
        || (!m_loading && SpatialWebDocument::allowedUrl(m_browser->url())));
    m_pageToggle->setEnabled(!m_headquarters && (m_document.stableLayout || canReturn || hasBrowserPage));
#endif
}

void SpatialWebBrowser::showSpace()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser || !m_scene->walking())
        return;
    cancelProjection();
    m_liveMode = false;
    m_preserveWorld = true;
    if (m_document.stableLayout && !m_browserTabChanged)
        m_site->resumeArea();
    m_pages->setCurrentWidget(m_scene);
    m_start->hide();
    m_scene->show();
    m_scene->raise();
    updatePageToggle();
    setStatus(tr("Resumed %1 · Your position and world are preserved. Build space updates the world from the current page.").arg(m_document.title));
    if (m_resumeTheatre && m_document.venue == "cinema") {
        m_browserUi->setBackgroundSuspended(false);
        m_browserUi->show();
        m_browserUi->lower();
        m_media->resumePage();
        m_scene->raise();
    } else {
        freezeBackgroundBrowser();
    }
    m_resumeTheatre = false;
    m_scene->setFocus();
#endif
}

void SpatialWebBrowser::scheduleProjection()
{
    if (isVisible() && !m_site->active() && !m_liveMode && !m_preserveWorld && (!m_media || !m_media->active())) {
        setStatus(tr("Website loaded · preparing its space…"));
        m_refresh.start();
    }
}

void SpatialWebBrowser::probeLoadedContent()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser || !m_loading || m_liveMode || m_preserveWorld || m_site->active() || m_media->active() || !isVisible())
        return;
    if (++m_loadAttempts > 20) {
        m_loading = false;
        m_browser->stop();
        freezeBackgroundBrowser();
        failPassage();
        setStatus(tr("The website has not provided readable content. Use Live page for its consent or access requirements, then Build space."));
        return;
    }
    const auto generation = m_generation;
    const QPointer<SpatialWebBrowser> guard(this);
    // Ads, analytics and slow images may keep loadFinished pending after the
    // actual article is ready. Only accept a parsed DOM with meaningful content.
    m_browser->page()->runJavaScript(R"JS((() => {
        const root = document.querySelector('main,[role="main"],article,#search,#links');
        const search = /(^|\.)(google\.(?:[a-z]{2,3}|(?:co|com)\.[a-z]{2})|bing\.com|duckduckgo\.com|search\.yahoo\.com)$/.test(location.hostname)
            && (new URLSearchParams(location.search).has('q') || new URLSearchParams(location.search).has('p'));
        const heading = document.querySelector(search ? 'h2 a[href],h3,a[data-testid="result-title-a"]' : 'h1');
        return {url: location.href, ready: !!heading
            && !!root && root.textContent.slice(0, 8192).trim().length >= 120};
    })())JS", QWebEngineScript::ApplicationWorld, [guard, generation](const QVariant &ready) {
        if (!guard || guard->m_generation != generation || !guard->m_loading)
            return;
        const auto content = ready.toMap();
        // Chromium keeps the old DOM alive until the new response commits.
        // Never mistake that old article for the destination being requested.
        const QUrl source(content.value("url").toString());
        const QUrl requested = guard->m_browser->page()->requestedUrl();
        const QUrlQuery sourceQuery(source);
        const QUrlQuery requestedQuery(requested);
        const bool sameSearch = SpatialWebDocument::searchResultsUrl(source)
            && SpatialWebDocument::searchResultsUrl(requested) && source.host() == requested.host()
            && sourceQuery.queryItemValue("q") == requestedQuery.queryItemValue("q")
            && sourceQuery.queryItemValue("p") == requestedQuery.queryItemValue("p");
        // Search providers may append session parameters before ads finish.
        // Accept that committed result document, but never a different query.
        if (content.value("ready").toBool() && (source == requested || sameSearch)
            && source == guard->m_browser->url()) {
            guard->m_loading = false;
            guard->m_earlyProjection = true;
            guard->buildSpace();
        } else {
            guard->m_loadProbe.start();
        }
    });
#endif
}

void SpatialWebBrowser::buildSpace()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_browser)
        return;
    if (m_browserTabChanged)
        m_site->leaveArea();
    if (m_site->active()) {
        m_site->leaveArea();
        m_preserveWorld = false;
        prepareBackgroundBrowser();
        m_browser->load(m_document.url);
        return;
    }
    if (!isVisible() || m_loading || !SpatialWebDocument::allowedUrl(m_browser->url()))
        return;
    m_media->stop();
    cancelProjection();
    m_preserveWorld = false;
    m_browserTabChanged = false;
    m_resumeTheatre = false;
    m_building = true;
    m_deadline.start();
    const quint64 generation = m_generation;
    prepareBackgroundBrowser();
    QFile script(":/web/extract.js");
    if (!script.open(QIODevice::ReadOnly)) {
        cancelProjection();
        freezeBackgroundBrowser();
        setStatus(tr("The spatial parser is unavailable in this build."));
        return;
    }
    const QPointer<SpatialWebBrowser> guard(this);
    m_browser->page()->runJavaScript(QString::fromUtf8(script.readAll()), QWebEngineScript::ApplicationWorld,
        [guard, generation](const QVariant &result) {
            if (!guard || guard->m_generation != generation)
                return;
            const auto state = QJsonDocument::fromJson(result.toString().toUtf8()).object();
            if (state.value("pending").toBool()) {
                guard->cancelProjection();
                guard->m_loading = true;
                guard->m_loadProbe.start();
                guard->setStatus(tr("Waiting for the destination's content…"));
                return;
            }
            QString error;
            SpatialWebDocument document;
            if (!SpatialWebDocument::parse(result.toString().toUtf8(), document, error)) {
                guard->cancelProjection();
                guard->freezeBackgroundBrowser();
                guard->failPassage();
                guard->setStatus(error);
                return;
            }
            if (document.url != guard->m_browser->url()) {
                guard->cancelProjection();
                guard->freezeBackgroundBrowser();
                return;
            }
            if (document.items.isEmpty()) {
                guard->cancelProjection();
                guard->freezeBackgroundBrowser();
                guard->failPassage();
                guard->updatePageToggle();
                guard->setStatus(SpatialWebDocument::searchResultsUrl(document.url)
                    ? tr("No search results are available yet. Open Live page to check consent or access requirements, then Build space. Your current world is preserved.")
                    : tr("No spatial destinations were found. The current view is preserved; open Live page to use this website."));
                emit guard->spaceReady();
            } else if (guard->m_passage) {
                guard->publishDocument(document, generation);
            } else if (guard->spatial() && !guard->m_document.items.isEmpty()) {
                guard->m_pendingDocument = document;
                guard->m_scene->setEditorActive(true);
                guard->m_phase.start();
            } else {
                guard->publishDocument(document, generation);
            }
        });
#endif
}

void SpatialWebBrowser::publishDocument(const SpatialWebDocument &document, quint64 generation)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (generation != m_generation)
        return;
    m_pendingCaptures.clear();
    m_captureClock.start();
    for (int index = 0; index < document.items.size(); ++index) {
        if (!document.stableLayout && document.items[index].hasImage)
            m_pendingCaptures.insert(index);
    }
    // Geometry and signs precede image sprites. Publishing is the only point
    // that changes the room or camera; later texture arrivals leave both alone.
    SpatialInterior interior;
    m_worldNotice.clear();
    m_headquarters = false;
    if (document.venue == "cinema" || document.venue == "city") {
        interior = document.room();
    } else {
        const auto result = SpatialWebWorld::construct(document, SpatialWebWorld::optionsFor(document));
        if (!result.success) {
            cancelProjection();
            freezeBackgroundBrowser();
            setStatus(result.diagnostic);
            return;
        }
        interior = result.interior;
        m_worldNotice = result.diagnostic;
        if (result.fallback)
            setStatus(result.diagnostic);
    }
    if (m_passage) {
        if (m_passage->failed())
            return;
        if (document.items.isEmpty() || document.items.first().id == "site:waiting")
            return;
        if (!m_passage->stage(interior)) {
            failPassage();
            return;
        }
        m_arrivalInterior = interior;
        m_arrivalDocument = document;
        for (int index = 0; index < m_arrivalDocument->items.size(); ++index) {
            auto &item = m_arrivalDocument->items[index];
            item.poster = SpatialWebDocument::poster(item, item.visual, index);
        }
        m_deadline.stop();
        m_passageDeadline.stop();
        m_building = false;
        paintPassage();
        setStatus(tr("The next area is ready. Walk through the open door."));
        if (!document.stableLayout) {
            m_passageCapturing = true;
            captureNext(generation, 0);
        }
        return;
    }
    m_searchOpen = false;
    m_search->hide();
    m_currentInterior = interior;
    m_currentInterior.environmentKey = document.environmentKey();
    const bool retainPosition = document.stableLayout && m_document.layoutIdentity == document.layoutIdentity;
    m_document = document;
    {
        const QSignalBlocker blocker(m_layout);
        m_layout->setCurrentIndex(0);
    }
    m_layout->setEnabled(false);
    m_scene->setInterior(interior, document.title, retainPosition, retainPosition, document.environmentKey());
    updatePageToggle();
    m_scene->setShipVisible(m_scene->interiorLocomotion() == "fly");
    {
        const QSignalBlocker blocker(m_mobility);
        m_mobility->setCurrentIndex(m_mobility->findData(m_scene->interiorLocomotion()));
    }
    m_mobility->setEnabled(true);
    m_pause->setVisible(document.venue == "cinema");
    m_stopMedia->setVisible(document.venue == "cinema");
    for (int index = 0; index < m_document.items.size(); ++index) {
        auto &item = m_document.items[index];
        item.poster = SpatialWebDocument::poster(item, {}, index);
        m_scene->setInteriorImage(item.id, item.poster);
    }
    if (document.venue == "cinema" && !m_document.items.isEmpty())
        m_scene->setInteriorImage("cinema-screen", m_document.items.first().poster);
    m_pages->setCurrentWidget(m_scene);
    m_start->hide();
    m_browserUi->lower();
    m_scene->setFlightHardwareAcceleration(m_hardware);
    m_scene->setFlightStatus(tr("Images are arriving. Approach a destination while facing it to enter, or select its display."));
    m_scene->setFocus();
    requestEnhancement();
    if (document.stableLayout)
        finishProjection(generation);
    else
        captureNext(generation, 0);
#endif
}

void SpatialWebBrowser::captureNext(quint64 generation, int index)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (generation != m_generation || !isVisible() || !m_browser)
        return;
    if (m_captureClock.elapsed() >= 10000) {
        finishProjection(generation);
        return;
    }
    const auto &captureDocument = m_arrivalDocument ? *m_arrivalDocument : m_document;
    if (index >= captureDocument.items.size()) {
        // Rotate through pending images so one slow CDN cannot starve every
        // other exhibit. One ten-second budget covers all passes and images.
        if (!m_pendingCaptures.isEmpty() && m_captureClock.elapsed() < 10000) {
            QTimer::singleShot(160, this, [this, generation]() { captureNext(generation, 0); });
            return;
        }
        finishProjection(generation);
        return;
    }
    setStatus(tr("Building %1 · exhibit %2 of %3…").arg(captureDocument.venue).arg(index + 1).arg(captureDocument.items.size()));
    if (!m_pendingCaptures.contains(index)) {
        QTimer::singleShot(0, this, [this, generation, index]() { captureNext(generation, index + 1); });
        return;
    }
    const int sourceIndex = captureDocument.items[index].id.toInt();
    const QString scroll = QStringLiteral(R"JS((() => {
        const state = window.__dockpipeSpatial;
        const node = state && state.nodes[%1];
        if (!node || !node.capture)
            return;
        if (!node.capture.isConnected && node.capture.tagName === 'IMG' && node.capture.alt) {
            let replacement = null;
            let matches = 0;
            for (let index = 0; index < Math.min(document.images.length, 256); index++) {
                if (document.images[index].alt === node.capture.alt) {
                    replacement = document.images[index];
                    matches++;
                }
            }
            if (matches === 1)
                node.capture = replacement;
        }
        const image = node.capture;
        if (image.tagName === 'IMG') {
            image.loading = 'eager';
            const placeholder = !image.currentSrc || (image.complete && image.naturalWidth <= 1);
            if (placeholder) {
                const source = image.getAttribute('data-src') || image.getAttribute('data-lazy-src')
                    || image.getAttribute('data-original');
                if (source) {
                    try {
                        const url = new URL(source, document.baseURI);
                        if (/^https?:$/.test(url.protocol) && !url.username && !url.password && url.href.length <= 4096)
                            image.src = url.href;
                    } catch (_) {}
                }
                const responsive = target => {
                    const sourceSet = target.getAttribute('data-srcset');
                    if (!sourceSet || sourceSet.length > 4096)
                        return;
                    const candidates = sourceSet.split(',').slice(0, 16);
                    const safe = candidates.every(candidate => {
                        try {
                            const url = new URL(candidate.trim().split(/\s+/)[0], document.baseURI);
                            return /^https?:$/.test(url.protocol) && !url.username && !url.password;
                        } catch (_) { return false; }
                    });
                    if (safe)
                        target.srcset = sourceSet;
                };
                responsive(image);
                const picture = image.closest('picture');
                if (picture) {
                    const sources = picture.querySelectorAll('source[data-srcset]');
                    for (let index = 0; index < Math.min(sources.length, 8); index++)
                        responsive(sources[index]);
                }
            }
        }
        if (image.isConnected)
            image.scrollIntoView({block:'center',behavior:'instant'});
    })())JS").arg(sourceIndex);
    m_browser->page()->runJavaScript(scroll, QWebEngineScript::ApplicationWorld);
    QTimer::singleShot(160, this, [this, generation, index, sourceIndex]() {
        if (generation != m_generation)
            return;
        if (m_captureClock.elapsed() >= 10000) {
            finishProjection(generation);
            return;
        }
        const QString rectangle = QStringLiteral(R"JS((() => {
            const state = window.__dockpipeSpatial;
            const node = state && state.nodes[%1];
            if (!node || !node.capture || !node.capture.isConnected)
                return null;
            const bounds = node.capture.getBoundingClientRect();
            const ready = node.capture.tagName !== 'IMG'
                || (node.capture.complete && node.capture.naturalWidth > 1 && node.capture.naturalHeight > 1);
            return {x: bounds.x, y: bounds.y, width: bounds.width, height: bounds.height, ready};
        })())JS").arg(sourceIndex);
        const QPointer<SpatialWebBrowser> guard(this);
        m_browser->page()->runJavaScript(rectangle, QWebEngineScript::ApplicationWorld,
            [guard, generation, index](const QVariant &result) {
                if (!guard || generation != guard->m_generation)
                    return;
                const auto map = result.toMap();
                if (map.isEmpty() || !map.value("ready").toBool()) {
                    guard->captureNext(generation, index + 1);
                    return;
                }
                const QRectF bounds(map.value("x").toDouble(), map.value("y").toDouble(),
                    map.value("width").toDouble(), map.value("height").toDouble());
                const QRect crop = bounds.intersected(QRectF(guard->m_browser->rect())).toAlignedRect();
                QImage visual;
                if (!crop.isEmpty() && map.value("ready").toBool())
                    visual = guard->m_browser->grab(crop).toImage();
                if (visual.isNull()) {
                    guard->captureNext(generation, index + 1);
                    return;
                }
                guard->m_pendingCaptures.remove(index);
                auto &activeDocument = guard->m_arrivalDocument ? *guard->m_arrivalDocument : guard->m_document;
                if (index >= activeDocument.items.size())
                    return;
                auto &item = activeDocument.items[index];
                item.visual = visual.scaled(512, 320, Qt::KeepAspectRatio, Qt::SmoothTransformation);
                item.poster = SpatialWebDocument::poster(item, item.visual, index);
                if (guard->m_enhance->isChecked() && item.kind == "product")
                    guard->m_neural->cutout(item.id, item.visual);
                const QString textureId = guard->m_arrivalDocument ? "arrival:" + item.id : item.id;
                guard->m_scene->setInteriorImage(textureId, item.poster);
                if (guard->m_document.venue == "cinema" && index == 0)
                    guard->m_scene->setInteriorImage("cinema-screen", item.poster);
                guard->captureNext(generation, index + 1);
            });
    });
#endif
}

void SpatialWebBrowser::finishProjection(quint64 generation)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (generation != m_generation)
        return;
    m_deadline.stop();
    m_building = false;
    m_passageCapturing = false;
    if (m_passage) {
        freezeBackgroundBrowser();
        setStatus(tr("The next area is ready. Walk through the open door."));
        return;
    }
    updatePageToggle();
    m_layout->setEnabled(m_document.venue != "cinema" && m_document.venue != "city" && !m_headquarters);
    if (m_browser && !m_document.stableLayout)
        m_browser->page()->runJavaScript("(() => { const s = window.__dockpipeSpatial; if (s) scrollTo(s.scrollX, s.scrollY); })()", QWebEngineScript::ApplicationWorld);
    if (m_document.items.isEmpty()) {
        freezeBackgroundBrowser();
        setStatus(tr("No spatial exhibits were found on this page. Use the live page, then try Build space after its content loads."));
        emit spaceReady();
        return;
    }
    if (m_scene->activeVehicleId().isEmpty()) {
        if (!m_scene->vehicles().isEmpty())
            m_scene->setFlightStatus(tr("Walk up to a car, aim and press E / A to drive · Models are approximate · Select a sign to visit its listing"));
        else
            m_scene->setFlightStatus(tr("Approach a destination while facing it to enter its world. Click / E also opens a display. Live page handles website controls."));
    }
    freezeBackgroundBrowser();
    if (!m_pendingCaptures.isEmpty())
        m_worldNotice += tr(" %1 images did not become available within the loading budget; Build space retries them.").arg(m_pendingCaptures.size());
    setStatus(tr("%1 · %2 exhibits · WASD moves, right-drag looks, click / E opens. %3")
        .arg(m_document.venue, QString::number(m_document.items.size()), m_document.limited
            ? tr("Showing a bounded selection; the full website remains available in Live page.") : QString()) + " " + m_worldNotice);
    emit spaceReady();
#endif
}

void SpatialWebBrowser::activateItem(const QString &id)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_passage)
        return;
    const bool passage = beginPassage(id);
    m_followingEntrance = passage;
    const bool siteDestination = m_site->activate(id);
    m_followingEntrance = false;
    if (siteDestination)
        return;
    if (!m_browser || m_loading)
        return;
    if (id == "cinema-screen" && m_media->active()) {
        m_pause->click();
        return;
    }
    for (const auto &item : m_document.items) {
        if (item.id != id && !(id == "cinema-screen" && item.id == m_document.items.first().id))
            continue;
        if (item.kind == "excerpt" || item.kind == "image"
            || (item.kind != "video" && SpatialWebDocument::samePage(item.url, m_document.url))) {
            showPage();
            return;
        }
        if (m_document.venue == "cinema" && item.kind == "video") {
            cancelProjection();
            m_resumeTheatre = false;
            m_browserTabChanged = false;
            prepareBackgroundBrowser();
            m_media->play(item.url.isEmpty() ? m_document.url : item.url);
            return;
        }
        if (!item.url.isEmpty()) {
            // Held movement can reach the same threshold again while the next
            // page is being interpreted. Keep that in-flight navigation intact.
            if (!passage && item.url == m_browser->url() && (m_building || m_refresh.isActive() || transitioning()))
                return;
            m_scene->setFlightStatus(tr("Opening %1 · %2…").arg(item.title, item.url.host()));
            m_followingEntrance = passage;
            // A destination is an exact page, not a request to re-enter the
            // cached site overview. Discovery remains available in the map.
            navigate(item.url, false);
            m_followingEntrance = false;
        } else {
            showPage();
            if (m_document.url == m_browser->url())
                m_browser->page()->runJavaScript(QStringLiteral("(() => { const s = window.__dockpipeSpatial; const n = s && s.nodes[%1]; if (n && n.element.isConnected) n.element.scrollIntoView({block:'center'}); })()").arg(id.toInt()), QWebEngineScript::ApplicationWorld);
        }
        return;
    }
#endif
}

void SpatialWebBrowser::hideEvent(QHideEvent *event)
{
    cancelPassage();
    if (m_media)
        m_media->stop();
    cancelProjection();
    m_site->pause();
    m_scene->setFlightFullscreen(false);
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browserUi) {
        m_browserUi->setBackgroundSuspended(true);
        m_browserUi->hide();
    }
    if (m_browser) {
        // Mark the child explicitly hidden. Hiding only its parent lets Qt show
        // it again on theatre reentry, which wakes the supposedly frozen page.
        m_browser->hide();
        m_browser->stop();
        m_browser->page()->setVisible(false);
        m_browser->page()->setAudioMuted(true);
        m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
    }
#endif
    QWidget::hideEvent(event);
}

void SpatialWebBrowser::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser && m_liveMode) {
        m_browserUi->setBackgroundSuspended(false);
        m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
        m_browserUi->show();
        m_browser->setVisible(!m_browser->url().isEmpty());
        m_browser->page()->setAudioMuted(false);
    }
#endif
}

bool SpatialWebBrowser::beginPassage(const QString &id)
{
    if (!spatial() || m_liveMode || m_loading || m_document.venue == "cinema"
        || m_scene->interiorLocomotion() != "walk" || m_currentInterior.displays.size() > 12)
        return false;
    for (const auto &display : m_currentInterior.displays) {
        if (display.id != id || !display.portal)
            continue;
        SpatialWebPassage passage;
        if (!passage.begin(m_currentInterior, display))
            return false;
        m_passage = passage;
        m_passageStart = m_scene->cameraPosition();
        m_arrivalDocument.reset();
        m_neural->cancel();
        m_autoDesign = false;
        m_applyDesign->hide();
        paintPassage();
        m_passageTick.start();
        m_passageDeadline.start();
        m_back->setEnabled(true);
        m_mobility->setEnabled(false);
        m_layout->setEnabled(false);
        return true;
    }
    return false;
}

void SpatialWebBrowser::paintPassage()
{
    if (!m_passage)
        return;
    const auto position = m_scene->cameraPosition();
    m_scene->streamInterior(m_passage->room(), m_document.title, position);
    for (const auto &item : m_document.items)
        m_scene->setInteriorImage(item.id, item.poster);
    if (m_arrivalDocument) {
        for (const auto &item : m_arrivalDocument->items)
            m_scene->setInteriorImage("arrival:" + item.id, item.poster);
    }
    m_scene->setFlightStatus(m_passage->failed() ? tr("Loading failed · Back / Esc returns to the previous area")
        : tr("Walk along the passage · The far door opens when the next area is ready · Back / Esc cancels"));
}

void SpatialWebBrowser::updatePassage()
{
    if (!m_passage || !isVisible() || !spatial())
        return;
    const auto position = m_scene->cameraPosition();
    if (m_passage->arrived(position) && m_arrivalDocument) {
        const auto destinationPosition = m_passage->destinationPosition(position);
        // A coordinate rebase preserves the view direction and walking input;
        // no fade, camera reset or accumulating world coordinates.
        const float yawDelta = m_passage->yaw() * 3.14159265358979323846f / 180;
        m_document = *m_arrivalDocument;
        m_currentInterior = m_arrivalInterior;
        m_currentInterior.environmentKey = m_document.environmentKey();
        m_arrivalDocument.reset();
        m_passage.reset();
        m_passageTick.stop();
        m_passageDeadline.stop();
        m_scene->streamInterior(m_currentInterior, m_document.title, destinationPosition, yawDelta);
        for (const auto &item : m_document.items)
            m_scene->setInteriorImage(item.id, item.poster);
        m_scene->setShipVisible(m_currentInterior.locomotion == "fly");
        m_mobility->setCurrentIndex(m_mobility->findData(m_currentInterior.locomotion));
        m_pause->setVisible(m_document.venue == "cinema");
        m_stopMedia->setVisible(m_document.venue == "cinema");
        if (m_document.venue == "cinema" && !m_document.items.isEmpty())
            m_scene->setInteriorImage("cinema-screen", m_document.items.first().poster);
        updatePageToggle();
        if (!m_passageCapturing)
            finishProjection(m_generation);
        return;
    }
    if (m_passage->advance(position))
        paintPassage();
}

void SpatialWebBrowser::cancelPassage()
{
    if (!m_passage)
        return;
    m_passage.reset();
    m_arrivalDocument.reset();
    m_passageCapturing = false;
    m_passageTick.stop();
    m_passageDeadline.stop();
    cancelProjection();
    m_site->leaveArea();
    m_site->pause();
    m_preserveWorld = true;
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser)
        m_browser->stop();
#endif
    m_scene->streamInterior(m_currentInterior, m_document.title, m_passageStart);
    m_scene->setEditorActive(false);
    for (const auto &item : m_document.items)
        m_scene->setInteriorImage(item.id, item.poster);
    m_scene->setFlightStatus(tr("Returned to the entrance · Select a destination to try again"));
    setStatus(tr("Returned to the entrance · Select a destination to try again"));
    updatePageToggle();
}

void SpatialWebBrowser::failPassage()
{
    if (!m_passage || m_passage->failed())
        return;
    cancelProjection();
    m_passage->fail();
    m_passageTick.stop();
    m_passageDeadline.stop();
    m_site->leaveArea();
    m_site->pause();
    m_preserveWorld = true;
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser)
        m_browser->stop();
#endif
    freezeBackgroundBrowser();
    paintPassage();
    setStatus(tr("Unable to prepare this area. Back / Esc returns to the entrance; select the destination to retry."));
}

void SpatialWebBrowser::showHeadquarters()
{
    m_searchOpen = false;
    if (m_search)
        m_search->hide();
    cancelPassage();
    cancelProjection();
    m_site->leaveArea();
    m_site->pause();
    if (m_media)
        m_media->stop();
    m_preserveWorld = true;
    m_headquarters = true;
    m_liveMode = false;
    m_resumeTheatre = false;
    m_loading = false;
    m_loadProbe.stop();
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_browser)
        m_browser->stop();
#endif
    m_scene->setEditorActive(false);
    m_document = {};
    m_document.title = tr("Dockpipe Industries Inc. · Headquarters");
    m_currentInterior = SpatialCivicWorld::headquarters();
    m_scene->setInterior(m_currentInterior, m_document.title);
    m_scene->setInteriorImage("hq-brand", SpatialCivicWorld::headquartersSign());
    m_scene->setShipVisible(false);
    m_pages->setCurrentWidget(m_scene);
    m_start->hide();
    m_scene->show();
    m_scene->setFlightHardwareAcceleration(m_hardware);
    m_pause->hide();
    m_stopMedia->hide();
    m_address->clear();
    m_mobility->setCurrentIndex(0);
    m_layout->setEnabled(false);
    updatePageToggle();
    freezeBackgroundBrowser();
    m_scene->setFlightStatus(tr("Welcome to HQ · Reception: search · Outside: cinema, city directory and drone pad · E / A interacts"));
    setStatus(tr("Dockpipe Industries Inc. · Search or enter a website above. Your destinations open into connected spatial worlds."));
    m_scene->setFocus();
}

