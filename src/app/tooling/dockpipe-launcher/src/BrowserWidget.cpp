#include "BrowserWidget.h"
#include "BrowserTabBar.h"
#include "BrowserDownloads.h"
#include "BrowserRuntime.h"
#include "BrowserPreferences.h"
#include "BrowserExtensionStore.h"
#include "BrowserExtensionPackage.h"
#include "CanvasIcons.h"

#include <QAction>
#include <QCoreApplication>
#include <QDir>
#include <QEvent>
#include <QDynamicPropertyChangeEvent>
#include <QFrame>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QMenu>
#include <QMessageBox>
#include <QProgressBar>
#include <QResizeEvent>
#include <QPrinter>
#include <functional>
#include <QPushButton>
#include <QShortcut>
#include <QSignalBlocker>
#include <QStackedWidget>
#include <QTabBar>
#include <QTemporaryDir>
#include <QTimer>
#include <QUrlQuery>
#include <QVBoxLayout>
#include <QWebEngineNewWindowRequest>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
#include <QWebEngineProfileBuilder>
#endif
#include <QWebEngineSettings>
#include <QWebEngineView>

namespace {
// Chromium can finish preference writes on its I/O thread after the profile
// destructor returns. Keep ownership through a bounded settling period instead
// of racing those writes with a single temporary-directory removal.
class SessionCleanup final : public QObject
{
public:
    explicit SessionCleanup(std::unique_ptr<QTemporaryDir> directory)
        : QObject(QCoreApplication::instance()), m_directory(std::move(directory))
    {
        auto *timer = new QTimer(this);
        timer->setInterval(250);
        connect(timer, &QTimer::timeout, this, [this]() {
            QDir(m_directory->path()).removeRecursively();
            if (++m_attempts == 8)
                deleteLater();
        });
        timer->start();
    }

private:
    std::unique_ptr<QTemporaryDir> m_directory;
    int m_attempts = 0;
};

QPushButton *iconButton(CanvasIcons::Symbol icon, const QString &label, const QString &name)
{
    auto *button = new QPushButton;
    button->setObjectName(name);
    button->setProperty("browserIcon", true);
    button->setIcon(CanvasIcons::icon(icon));
    button->setIconSize(QSize(19, 19));
    button->setToolTip(label);
    button->setAccessibleName(label);
    button->setFixedSize(36, 36);
    return button;
}
}

// A detached live tab retains the exact Chromium profile and download manager.
// The last browser/tab lease releases pages before releasing their profile.
struct BrowserSession
{
    std::unique_ptr<QTemporaryDir> directory;
    QWebEngineProfile *profile = nullptr;
    BrowserDownloads *downloads = nullptr;
    QList<QPointer<BrowserWidget>> browsers;

    ~BrowserSession()
    {
        delete downloads;
        delete profile;
        if (directory && directory->isValid())
            new SessionCleanup(std::move(directory));
    }
};

BrowserWidget::BrowserWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("browserWidget");
    buildChrome();
    installEventFilter(this);
    applyPreferences();
    connect(BrowserPreferences::instance(), &BrowserPreferences::changed, this, &BrowserWidget::applyPreferences);
    createSession();
}

BrowserWidget::~BrowserWidget()
{
    destroySession();
}

void BrowserWidget::buildChrome()
{
    setStyleSheet(R"CSS(
        QWidget#browserWidget { background: palette(window); }
        QFrame#browserChrome { border: none; border-bottom: 1px solid palette(mid); }
        QPushButton[browserIcon="true"] { border: none; border-radius: 18px; padding: 0; }
        QPushButton[browserIcon="true"]:hover { background: palette(alternate-base); }
        QWidget#browserTabCloseContainer { background: transparent; border: none; }
        QPushButton#browserCloseTab { background: transparent; border: none; border-radius: 12px;
            min-width: 0; min-height: 0; padding: 0; margin: 0; }
        QPushButton#browserCloseTab:hover { background: palette(alternate-base); }
        QPushButton#browserMore::menu-indicator { width: 0; }
        QPushButton#browserAnnotate { background: palette(alternate-base); border: none;
            border-radius: 18px; padding: 7px 14px; }
        QFrame#browserAddressPill { background: palette(base); border: 1px solid palette(mid); border-radius: 19px; }
        QLineEdit#browserAddress { background: transparent; border: none; padding: 6px 4px; }
        QTabBar#browserTabs::tab { padding: 8px 14px; min-width: 120px; max-width: 240px;
            border: 1px solid transparent; border-radius: 10px; margin: 3px 2px; }
        QTabBar#browserTabs::tab:selected { background: palette(base); border-color: palette(mid); }
        QTabBar#browserTabs::tab:hover { background: palette(alternate-base); }
        QLabel#browserStatus { padding: 3px 10px; font-size: 11px; }
    )CSS");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(0);
    auto *chrome = new QFrame;
    chrome->setObjectName("browserChrome");
    // The toolbar adapts after a resize. Its expanded row must not impose a
    // minimum width that prevents that resize from reaching compact mode.
    chrome->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    auto *chromeLayout = new QVBoxLayout(chrome);
    chromeLayout->setContentsMargins(10, 4, 10, 8);
    chromeLayout->setSpacing(4);
    auto *tabRow = new QHBoxLayout;
    tabRow->setObjectName("browserTabRow");
    tabRow->setSpacing(4);
    m_tabs = new BrowserTabBar;
    m_tabs->setObjectName("browserTabs");
    m_tabs->setMovable(true);
    m_tabs->setDrawBase(false);
    m_tabs->setExpanding(false);
    m_tabs->setElideMode(Qt::ElideRight);
    auto *newTab = iconButton(CanvasIcons::Symbol::Add, tr("New tab (Ctrl+T)"), "browserNewTab");
    tabRow->addWidget(m_tabs);
    m_newTabButton = newTab;
    tabRow->addWidget(newTab);
    tabRow->addStretch();
    m_viewportButton = iconButton(CanvasIcons::Symbol::Preview,
        tr("Show browser controls"), "browserCanvasViewport");
    m_viewportButton->setCheckable(true);
    m_viewportButton->hide();
    tabRow->addWidget(m_viewportButton);
    connect(m_viewportButton, &QPushButton::clicked, this, [this]() {
        setCanvasViewportMode(!m_canvasViewportRequested);
    });
    connect(m_tabs, &BrowserTabBar::tearOutRequested, this, [this](int index, const QPoint &position) {
        if (!m_canvasPresentationEnabled || m_printing || index < 0 || index >= m_pages->count())
            return;
        auto *view = m_pages->widget(index)->findChild<QWebEngineView *>();
        if (view)
            emit tabTearOutRequested(view, position);
    });
    chromeLayout->addLayout(tabRow);
    m_toolbar = new QWidget;
    auto *toolbar = new QHBoxLayout(m_toolbar);
    toolbar->setContentsMargins(0, 0, 0, 0);
    toolbar->setSpacing(6);
    m_back = iconButton(CanvasIcons::Symbol::Back, tr("Back (Alt+Left)"), "browserBack");
    m_forward = iconButton(CanvasIcons::Symbol::Forward, tr("Forward (Alt+Right)"), "browserForward");
    m_reload = iconButton(CanvasIcons::Symbol::Refresh, tr("Reload (Ctrl+R)"), "browserReload");
    m_stop = iconButton(CanvasIcons::Symbol::Close, tr("Stop loading"), "browserStop");
    auto *annotate = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Rename), tr("Annotate"));
    m_annotate = annotate;
    annotate->setObjectName("browserAnnotate");
    annotate->setFixedHeight(36);
    annotate->setToolTip(tr("Draw and add notes on a capture of this page"));
    toolbar->addWidget(m_back);
    toolbar->addWidget(m_forward);
    toolbar->addWidget(m_reload);
    toolbar->addWidget(m_stop);
    toolbar->addWidget(annotate);
    auto *addressPill = new QFrame;
    addressPill->setObjectName("browserAddressPill");
    auto *addressLayout = new QHBoxLayout(addressPill);
    addressLayout->setContentsMargins(2, 0, 2, 0);
    addressLayout->setSpacing(0);
    auto *site = iconButton(CanvasIcons::Symbol::Properties, tr("Site information"), "browserSiteInfo");
    m_address = new QLineEdit;
    m_address->setObjectName("browserAddress");
    m_address->setAccessibleName(tr("Website address or search"));
    m_address->setPlaceholderText(tr("Search or enter an address"));
    m_address->setMaxLength(4096);
    m_address->setAlignment(Qt::AlignCenter);
    m_address->installEventFilter(this);
    auto *go = iconButton(CanvasIcons::Symbol::Forward, tr("Go"), "browserGo");
    m_goButton = go;
    addressLayout->addWidget(site);
    addressLayout->addWidget(m_address, 1);
    addressLayout->addWidget(go);
    toolbar->addWidget(addressPill, 1);
    m_installExtension = new QPushButton(tr("Add extension…"));
    m_installExtension->setObjectName("browserInstallExtension");
    m_installExtension->setToolTip(tr("Install this Chrome Web Store extension in Dockpipe"));
    m_installExtension->hide();
    toolbar->addWidget(m_installExtension);
    connect(m_installExtension, &QPushButton::clicked, this, &BrowserWidget::showExtensions);
    auto *extensions = iconButton(CanvasIcons::Symbol::Extensions, tr("Extensions"), "browserExtensions");
    auto *downloads = iconButton(CanvasIcons::Symbol::Download, tr("Downloads"), "browserDownloads");
    m_extensionsButton = extensions;
    m_downloadsButton = downloads;
    auto *more = iconButton(CanvasIcons::Symbol::More, tr("More browser tools"), "browserMore");
    more->setMenu(createMenu());
    toolbar->addWidget(extensions);
    toolbar->addWidget(downloads);
    toolbar->addWidget(more);
    chromeLayout->addWidget(m_toolbar);
    layout->addWidget(chrome);
    m_progress = new QProgressBar;
    m_progress->setTextVisible(false);
    m_progress->setFixedHeight(2);
    m_progress->hide();
    layout->addWidget(m_progress);
    m_findBar = new QWidget;
    m_findBar->setObjectName("browserFindBar");
    auto *findLayout = new QHBoxLayout(m_findBar);
    findLayout->addStretch();
    m_findText = new QLineEdit;
    m_findText->setObjectName("browserFindText");
    m_findText->setPlaceholderText(tr("Find in page"));
    m_findText->setMaxLength(500);
    m_findResult = new QLabel;
    auto *previous = iconButton(CanvasIcons::Symbol::Back, tr("Previous match"), "browserFindPrevious");
    auto *next = iconButton(CanvasIcons::Symbol::Forward, tr("Next match"), "browserFindNext");
    auto *closeFind = iconButton(CanvasIcons::Symbol::Close, tr("Close find"), "browserFindClose");
    findLayout->addWidget(m_findText);
    findLayout->addWidget(m_findResult);
    findLayout->addWidget(previous);
    findLayout->addWidget(next);
    findLayout->addWidget(closeFind);
    layout->addWidget(m_findBar);
    m_findBar->hide();
    m_pages = new QStackedWidget;
    layout->addWidget(m_pages, 1);
    m_status = new QLabel;
    m_status->setObjectName("browserStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_status->hide();
    layout->addWidget(m_status);
    connect(m_tabs, &QTabBar::currentChanged, this, &BrowserWidget::selectTab);
    connect(m_tabs, &QTabBar::tabCloseRequested, this, &BrowserWidget::closeTab);
    connect(m_tabs, &QTabBar::tabMoved, this, [this](int from, int to) {
        auto *page = m_pages->widget(from);
        m_pages->removeWidget(page);
        m_pages->insertWidget(to, page);
        selectTab(m_tabs->currentIndex());
    });
    connect(newTab, &QPushButton::clicked, this, [this]() { addTab(); });
    connect(m_address, &QLineEdit::returnPressed, this, &BrowserWidget::submitAddress);
    connect(go, &QPushButton::clicked, this, &BrowserWidget::submitAddress);
    connect(m_back, &QPushButton::clicked, this, [this]() {
        if (m_view)
            m_view->back();
    });
    connect(m_forward, &QPushButton::clicked, this, [this]() {
        if (m_view)
            m_view->forward();
    });
    connect(m_reload, &QPushButton::clicked, this, [this]() {
        if (m_view)
            m_view->reload();
    });
    connect(m_stop, &QPushButton::clicked, this, [this]() {
        if (m_view)
            m_view->stop();
    });
    connect(annotate, &QPushButton::clicked, this, [this]() { capturePage(true); });
    connect(site, &QPushButton::clicked, this, &BrowserWidget::showSiteInfo);
    connect(extensions, &QPushButton::clicked, this, &BrowserWidget::showExtensions);
    connect(downloads, &QPushButton::clicked, this, [this]() {
        if (m_downloads) {
            m_downloads->setHost(this);
            m_downloads->showPanel();
        }
    });
    connect(m_findText, &QLineEdit::textChanged, this, [this]() { findText(); });
    connect(m_findText, &QLineEdit::returnPressed, this, [this]() { findText(); });
    connect(next, &QPushButton::clicked, this, [this]() { findText(); });
    connect(previous, &QPushButton::clicked, this, [this]() { findText(true); });
    connect(closeFind, &QPushButton::clicked, this, [this]() {
        m_findBar->hide();
        if (m_view)
            m_view->findText({});
    });
    const auto shortcut = [this](const QKeySequence &key, const std::function<void()> &action) {
        auto *binding = new QShortcut(key, this);
        binding->setContext(Qt::WidgetWithChildrenShortcut);
        connect(binding, &QShortcut::activated, this, action);
    };
    shortcut(QKeySequence::AddTab, [this]() { addTab(); });
    shortcut(QKeySequence::Close, [this]() { closeTab(m_tabs->currentIndex()); });
    shortcut(QKeySequence(Qt::CTRL | Qt::Key_L), [this]() {
        if (m_canvasPresentationEnabled)
            setCanvasViewportMode(false);
        m_address->setFocus();
        m_address->selectAll();
    });
    shortcut(QKeySequence::Find, [this]() { showFind(); });
    shortcut(QKeySequence::Refresh, [this]() {
        if (m_view)
            m_view->reload();
    });
    shortcut(QKeySequence(Qt::CTRL | Qt::Key_R), [this]() {
        if (m_view)
            m_view->reload();
    });
    shortcut(QKeySequence(Qt::ALT | Qt::Key_Left), [this]() {
        if (m_view)
            m_view->back();
    });
    shortcut(QKeySequence(Qt::ALT | Qt::Key_Right), [this]() {
        if (m_view)
            m_view->forward();
    });
    shortcut(QKeySequence::Print, [this]() { printPage(); });
    updateToolbarLayout();
}

void BrowserWidget::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    updateToolbarLayout();
}

void BrowserWidget::updateToolbarLayout()
{
    if (!m_annotate || !m_goButton || !m_extensionsButton || !m_downloadsButton)
        return;
    // Reserve a readable address before exposing secondary shortcuts. The full
    // row has eight icon buttons (including site info/Go), one text action,
    // seven gaps, outer margins and the address pill's inner margins.
    const int addressWidth = m_address->fontMetrics().horizontalAdvance(QStringLiteral("https://example.com/path"));
    const int fullToolbarWidth = 8 * m_back->width() + m_annotate->sizeHint().width()
        + addressWidth + 7 * 6 + 20 + 4;
    const bool compact = width() < fullToolbarWidth;
    m_annotate->setVisible(!compact);
    m_extensionsButton->setVisible(!compact);
    m_downloadsButton->setVisible(!compact);
    m_goButton->setVisible(!compact);
}

void BrowserWidget::createSession()
{
    const QString error = BrowserRuntime::rendererError();
    if (!error.isEmpty()) {
        showStatus(error);
        m_address->setEnabled(false);
        updateNavigation();
        return;
    }
    m_session = std::make_shared<BrowserSession>();
    m_session->directory = std::make_unique<QTemporaryDir>();
    if (!m_session->directory->isValid()) {
        showStatus(tr("Could not create browser session storage."));
        return;
    }
    // Extensions need a named profile. Its storage is still temporary, preserving
    // the existing session-only contract instead of silently retaining site data.
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
    QWebEngineProfileBuilder builder;
    builder.setPersistentStoragePath(m_session->directory->path());
    builder.setCachePath(m_session->directory->filePath("cache"));
    builder.setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
    m_profile = builder.createProfile("DockPipeBrowser", nullptr);
#else
    m_profile = new QWebEngineProfile("DockPipeBrowser");
    m_profile->setPersistentStoragePath(m_session->directory->path());
    m_profile->setCachePath(m_session->directory->filePath("cache"));
    m_profile->setPersistentCookiesPolicy(QWebEngineProfile::NoPersistentCookies);
#endif
    if (!m_profile) {
        showStatus(tr("Could not create the browser profile."));
        return;
    }
    new BrowserExtensionStore(m_profile);
    m_session->profile = m_profile;
    m_downloads = new BrowserDownloads(m_profile, this);
    m_downloads->setParent(nullptr);
    m_session->downloads = m_downloads;
    m_session->browsers.append(this);
    const std::weak_ptr<BrowserSession> weakSession = m_session;
    connect(m_downloads, &BrowserDownloads::hostRequested, m_downloads, [weakSession]() {
        if (const auto session = weakSession.lock()) {
            for (const auto &browser : session->browsers) {
                if (browser) {
                    session->downloads->setHost(browser);
                    break;
                }
            }
        }
    });
    connect(m_downloads, &BrowserDownloads::statusChanged, this, &BrowserWidget::showStatus);
    addTab();
}

void BrowserWidget::destroySession()
{
    m_destroyingSession = true;
    if (m_view)
        emit activeViewAboutToChange(m_view);
    m_view = nullptr;
    emit activeViewChanged(nullptr);
    if (m_downloads)
        disconnect(m_downloads, nullptr, this, nullptr);
    m_downloads = nullptr;
    const QSignalBlocker blockTabs(m_tabs);
    while (m_pages->count()) {
        auto *page = m_pages->widget(0);
        m_pages->removeWidget(page);
        delete page;
        m_tabs->removeTab(0);
    }
    m_tabSessions.clear();
    m_session.reset();
    m_profile = nullptr;
    m_destroyingSession = false;
}

bool BrowserWidget::hasSharedSession() const
{
    QHash<const BrowserSession *, int> localLeases;
    for (const auto &session : m_tabSessions)
        ++localLeases[session.get()];
    if (m_session)
        ++localLeases[m_session.get()];
    for (const auto &session : m_tabSessions) {
        if (session.use_count() > localLeases.value(session.get()))
            return true;
    }
    return m_session && m_session.use_count() > localLeases.value(m_session.get());
}

QWebEngineView *BrowserWidget::addTab(const QUrl &url)
{
    if (!m_profile)
        return nullptr;
    // Each tab owns a renderer-backed page. Limit this desktop session to 32.
    if (m_tabs->count() >= 32) {
        showStatus(tr("32 tabs are open. Close a tab before opening another."));
        return nullptr;
    }
    auto *container = new QWidget;
    auto *layout = new QVBoxLayout(container);
    layout->setContentsMargins(0, 0, 0, 0);
    auto *welcome = new QLabel(canvasViewportMode() ? tr("Open a site\n\nPress Ctrl+L or show browser controls.")
        : tr("Browse the web\n\nSearch or enter an address above."));
    welcome->setObjectName("browserWelcome");
    welcome->setAlignment(Qt::AlignCenter);
    layout->addWidget(welcome, 1);
    auto *view = BrowserRuntime::createView(m_profile, container);
    view->setObjectName("browserWebView");
    view->settings()->setAttribute(QWebEngineSettings::JavascriptEnabled, m_javascript);
    view->settings()->setAttribute(QWebEngineSettings::AutoLoadImages, m_images);
    layout->addWidget(view, 1);
    view->hide();
    m_tabSessions.insert(view, m_session);
    attachTab(container, view);
    if (url.isValid())
        navigate(url);
    else if (!canvasViewportMode())
        m_address->setFocus();
    return view;
}

void BrowserWidget::attachTab(QWidget *container, QWebEngineView *view)
{
    auto *welcome = container->findChild<QLabel *>("browserWelcome");
    connect(view, &QObject::destroyed, this, [this, view]() { m_permissionOrigins.remove(view); });
    const int index = m_pages->addWidget(container);
    m_tabs->addTab(view->icon().isNull() ? CanvasIcons::icon(CanvasIcons::Symbol::Globe) : view->icon(),
        view->title().isEmpty() ? tr("New tab") : view->title().left(100));
    m_tabs->setTabToolTip(index, view->title().left(200));
    auto *close = iconButton(CanvasIcons::Symbol::Close, tr("Close tab"), "browserCloseTab");
    close->setFixedSize(24, 24);
    auto *closeContainer = new QWidget;
    closeContainer->setObjectName("browserTabCloseContainer");
    closeContainer->setFixedSize(32, 24);
    auto *closeLayout = new QHBoxLayout(closeContainer);
    closeLayout->setContentsMargins(0, 0, 8, 0);
    closeLayout->setSpacing(0);
    closeLayout->addWidget(close);
    connect(close, &QPushButton::clicked, this, [this, container]() {
        closeTab(m_pages->indexOf(container));
    });
    m_tabs->setTabButton(index, QTabBar::RightSide, closeContainer);
    connect(view, &QWebEngineView::loadStarted, this, [this, view, welcome]() {
        welcome->hide();
        view->show();
        view->setProperty("loading", true);
        view->setProperty("progress", 0);
        if (m_view == view)
            updateNavigation();
    });
    connect(view, &QWebEngineView::loadProgress, this, [this, view](int value) {
        view->setProperty("progress", value);
        if (m_view == view)
            m_progress->setValue(value);
    });
    connect(view, &QWebEngineView::loadFinished, this, [this, view](bool success) {
        view->setProperty("loading", false);
        if (m_backgroundSuspended || (m_suspendInactivePages && view != m_view)) {
            view->page()->setAudioMuted(true);
            view->page()->setVisible(false);
            view->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
        }
        if (success && BrowserRuntime::allowsUrl(view->url())) {
            if (m_history.isEmpty() || m_history.last().url != view->url())
                m_history.append({view->title().left(200), view->url(), QDateTime::currentDateTime()});
            if (m_history.size() > 500)
                m_history.removeFirst();
        }
        if (m_view == view) {
            updateNavigation();
            showStatus(success ? QString() : tr("The page did not finish loading. Reload or try another address."));
        }
    });
    connect(view, &QWebEngineView::urlChanged, this, [this, view](const QUrl &) {
        if (m_view == view)
            updateNavigation();
    });
    connect(view, &QWebEngineView::titleChanged, this, [this, container](const QString &title) {
        const int tab = m_pages->indexOf(container);
        m_tabs->setTabText(tab, title.isEmpty() ? tr("New tab") : title.left(100));
        m_tabs->setTabToolTip(tab, title.left(200));
    });
    connect(view, &QWebEngineView::iconChanged, this, [this, container](const QIcon &icon) {
        m_tabs->setTabIcon(m_pages->indexOf(container), icon.isNull() ? CanvasIcons::icon(CanvasIcons::Symbol::Globe) : icon);
    });
    for (const auto action : {QWebEnginePage::Back, QWebEnginePage::Forward, QWebEnginePage::Reload})
        connect(view->page()->action(action), &QAction::changed, this, [this, view]() {
            if (m_view == view)
                updateNavigation();
        });
    connect(view->page(), &QWebEnginePage::renderProcessTerminated, this, [this, view]() {
        view->setProperty("loading", false);
        if (m_view == view) {
            updateNavigation();
            showStatus(tr("The website renderer stopped. Reload to try again."));
        }
    });
    connect(view->page(), &QWebEnginePage::featurePermissionRequested, this,
        [this, view](const QUrl &origin, QWebEnginePage::Feature feature) {
            requestSitePermission(view, origin, feature);
        });
    connect(view->page(), &QWebEnginePage::newWindowRequested, this, [this, view](QWebEngineNewWindowRequest &request) {
        if (!request.isUserInitiated() || !BrowserRuntime::allowsNavigation(request.requestedUrl(), view->page()->profile()))
            return;
        if (auto *destination = addTab())
            request.openIn(destination->page());
    });
    m_tabs->setCurrentIndex(index);
    selectTab(index);
}

void BrowserWidget::closeTab(int index)
{
    if (index < 0 || index >= m_pages->count())
        return;
    if (m_printing) {
        showStatus(tr("Wait for printing to finish before closing a tab."));
        return;
    }
    auto *page = m_pages->widget(index);
    if (page->findChild<QWebEngineView *>() == m_view) {
        emit activeViewAboutToChange(m_view);
        m_view = nullptr;
    }
    {
        const QSignalBlocker blocker(m_tabs);
        m_pages->removeWidget(page);
        m_tabs->removeTab(index);
        auto *view = page->findChild<QWebEngineView *>();
        delete page;
        m_tabSessions.remove(view);
    }
    if (m_tabs->count())
        selectTab(m_tabs->currentIndex());
    else
        addTab();
}

void BrowserWidget::selectTab(int index)
{
    if (index < 0 || index >= m_pages->count() || m_destroyingSession)
        return;
    auto *next = m_pages->widget(index)->findChild<QWebEngineView *>();
    const bool changed = m_view != next;
    if (changed && m_view)
        emit activeViewAboutToChange(m_view);
    m_pages->setCurrentIndex(index);
    m_view = next;
    if (const auto session = m_tabSessions.value(next)) {
        m_session = session;
        m_profile = session->profile;
        m_downloads = session->downloads;
        m_downloads->setHost(this);
    }
    m_address->setModified(false);
    updateNavigation();
    if (m_findBar->isVisible())
        findText();
    if (changed)
        emit activeViewChanged(m_view);
}

void BrowserWidget::updateNavigation()
{
    if (m_installExtension)
        m_installExtension->setVisible(m_view && !BrowserExtensionPackage::extensionId(m_view->url().toString()).isEmpty());
    if (m_destroyingSession)
        return;
    const bool loading = m_view && m_view->property("loading").toBool();
    m_back->setEnabled(m_view && m_view->page()->action(QWebEnginePage::Back)->isEnabled());
    m_forward->setEnabled(m_view && m_view->page()->action(QWebEnginePage::Forward)->isEnabled());
    m_reload->setEnabled(m_view && m_view->page()->action(QWebEnginePage::Reload)->isEnabled());
    m_reload->setVisible(!loading);
    m_stop->setVisible(loading);
    m_stop->setEnabled(loading);
    m_progress->setVisible(m_chromeVisible && !canvasViewportMode() && loading);
    m_progress->setValue(m_view ? m_view->property("progress").toInt() : 0);
    if (!m_address->hasFocus() || !m_address->isModified()) {
        const QUrl url = m_view ? m_view->url() : QUrl();
        QString display = url.toDisplayString(QUrl::RemoveUserInfo);
        if (!m_address->hasFocus() && !m_showFullUrl && BrowserRuntime::allowsUrl(url)) {
            display = url.adjusted(QUrl::RemoveUserInfo).authority();
        }
        if (m_address->text() != display)
            m_address->setText(display);
    }
}

bool BrowserWidget::eventFilter(QObject *object, QEvent *event)
{
    if (object == this && event->type() == QEvent::DynamicPropertyChange) {
        auto *change = static_cast<QDynamicPropertyChangeEvent *>(event);
        if (change->propertyName() == "dashboardCanvasPresentation")
            setCanvasPresentationEnabled(property("dashboardCanvasPresentation").toBool());
    }
    if (object == m_address && (event->type() == QEvent::FocusIn || event->type() == QEvent::FocusOut)) {
        m_address->setAlignment(event->type() == QEvent::FocusIn ? Qt::AlignLeft : Qt::AlignCenter);
        if (!m_address->isModified())
            updateNavigation();
    }
    return QWidget::eventFilter(object, event);
}

void BrowserWidget::showStatus(const QString &text)
{
    m_status->setText(text);
    m_status->setVisible(m_chromeVisible && !text.isEmpty());
}

void BrowserWidget::setChromeVisible(bool visible)
{
    m_chromeVisible = visible;
    if (!visible)
        cancelPermissionPrompts();
    updatePresentation();
    if (!visible)
        m_findBar->hide();
    m_status->setVisible(visible && !m_status->text().isEmpty());
    updateNavigation();
}

bool BrowserWidget::canvasViewportMode() const
{
    return m_canvasPresentationEnabled && m_canvasViewportRequested;
}

void BrowserWidget::setCanvasPresentationEnabled(bool enabled)
{
    m_canvasPresentationEnabled = enabled;
    updatePresentation();
}

void BrowserWidget::setCanvasViewportMode(bool enabled)
{
    m_canvasViewportRequested = enabled;
    updatePresentation();
}

void BrowserWidget::updatePresentation()
{
    const bool viewport = canvasViewportMode();
    auto *chrome = findChild<QFrame *>("browserChrome");
    chrome->setVisible(m_chromeVisible);
    auto *chromeLayout = qobject_cast<QVBoxLayout *>(chrome->layout());
    chromeLayout->setContentsMargins(viewport ? QMargins(6, 1, 6, 1) : QMargins(10, 4, 10, 8));
    chromeLayout->setSpacing(viewport ? 0 : 4);
    m_toolbar->setVisible(!viewport);
    m_viewportButton->setVisible(m_canvasPresentationEnabled);
    m_viewportButton->setChecked(viewport);
    m_viewportButton->setToolTip(viewport ? tr("Show browser controls (Ctrl+L)") : tr("Focus on the page"));
    m_viewportButton->setAccessibleName(m_viewportButton->toolTip());
    m_viewportButton->setFixedSize(viewport ? QSize(28, 28) : QSize(36, 36));
    m_newTabButton->setFixedSize(viewport ? QSize(28, 28) : QSize(36, 36));
    for (auto *welcome : m_pages->findChildren<QLabel *>("browserWelcome"))
        welcome->setText(viewport ? tr("Open a site\n\nPress Ctrl+L or show browser controls.")
            : tr("Browse the web\n\nSearch or enter an address above."));
    m_tabs->setTearOutEnabled(m_canvasPresentationEnabled);
    m_tabs->setStyleSheet(viewport
        ? QStringLiteral("QTabBar::tab { padding: 4px 10px; min-width: 72px; max-width: 180px; margin: 1px 2px; border-radius: 8px; }")
        : QString());
    updateNavigation();
}

bool BrowserWidget::moveTabTo(QWebEngineView *view, BrowserWidget *destination)
{
    if (!view || !destination || destination == this || m_printing || destination->m_printing
        || !m_canvasPresentationEnabled || !destination->m_canvasPresentationEnabled
        || destination->m_tabs->count() >= 32)
        return false;
    auto *container = view->parentWidget();
    const int index = m_pages->indexOf(container);
    const auto session = m_tabSessions.value(view);
    if (index < 0 || !session)
        return false;

    cancelPermissionPrompts();
    if (view == m_view) {
        emit activeViewAboutToChange(view);
        m_view = nullptr;
    }
    disconnect(view, nullptr, this, nullptr);
    disconnect(view->page(), nullptr, this, nullptr);
    for (const auto action : {QWebEnginePage::Back, QWebEnginePage::Forward, QWebEnginePage::Reload})
        disconnect(view->page()->action(action), nullptr, this, nullptr);
    const auto permissionOrigins = m_permissionOrigins.take(view);
    {
        const QSignalBlocker blocker(m_tabs);
        auto *closeButton = m_tabs->tabButton(index, QTabBar::RightSide);
        m_tabs->setTabButton(index, QTabBar::RightSide, nullptr);
        delete closeButton;
        m_pages->removeWidget(container);
        m_tabs->removeTab(index);
    }
    m_tabSessions.remove(view);
    container->setParent(nullptr);

    // A newly created dashboard browser has one unused welcome tab. Replace it
    // only after retaining the incoming live tab's session.
    QWidget *blank = nullptr;
    if (destination->m_pages->count() == 1) {
        auto *candidate = destination->m_pages->widget(0);
        auto *candidateView = candidate->findChild<QWebEngineView *>();
        if (candidateView && candidateView->url().isEmpty())
            blank = candidate;
    }
    destination->m_tabSessions.insert(view, session);
    if (!permissionOrigins.isEmpty())
        destination->m_permissionOrigins.insert(view, permissionOrigins);
    session->browsers.removeIf([](const QPointer<BrowserWidget> &browser) { return browser.isNull(); });
    if (!session->browsers.contains(destination))
        session->browsers.append(destination);
    connect(session->downloads, &BrowserDownloads::statusChanged,
        destination, &BrowserWidget::showStatus, Qt::UniqueConnection);
    destination->attachTab(container, view);
    destination->setBackgroundSuspended(destination->m_backgroundSuspended);
    if (blank)
        destination->closeTab(destination->m_pages->indexOf(blank));
    if (m_tabs->count())
        selectTab(m_tabs->currentIndex());
    else
        addTab();
    return true;
}

void BrowserWidget::setBackgroundSuspended(bool suspended)
{
    m_suspendInactivePages = true;
    m_backgroundSuspended = suspended;
    if (suspended)
        cancelPermissionPrompts();
    for (auto *view : m_pages->findChildren<QWebEngineView *>()) {
        auto *page = view->page();
        const bool activate = !suspended && view == m_view;
        page->setAudioMuted(!activate);
        if (activate) {
            page->setLifecycleState(QWebEnginePage::LifecycleState::Active);
        } else {
            page->setVisible(false);
            if (!page->isLoading())
                page->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
        }
    }
}

QUrl BrowserWidget::addressUrl(const QString &text)
{
    const QString input = text.trimmed();
    if (input.isEmpty() || input.size() > 4096)
        return {};
    if (input.contains("://") || input.startsWith("about:") || input.startsWith("file:")
        || input.startsWith("javascript:") || input.startsWith("data:"))
        return QUrl(input);
    if (!input.contains(' ') && (input.contains('.') || input.startsWith("localhost")))
        return QUrl::fromUserInput(input);
    QUrl search("https://duckduckgo.com/");
    QUrlQuery query;
    query.addQueryItem("q", input);
    search.setQuery(query);
    return search;
}

void BrowserWidget::submitAddress()
{
    navigate(addressUrl(m_address->text()));
}

void BrowserWidget::navigate(const QUrl &url)
{
    if (!m_view)
        return;
    if (!BrowserRuntime::allowsNavigation(url, m_profile)) {
        showStatus(tr("Enter an HTTP or HTTPS website address without embedded credentials."));
        return;
    }
    m_address->setModified(false);
    m_view->load(url);
}
