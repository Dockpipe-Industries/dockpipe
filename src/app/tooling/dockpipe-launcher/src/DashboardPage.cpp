#include "DashboardPage.h"
#include <QShortcut>
#include "DashboardSettingsDialog.h"
#include "DashboardTourDialog.h"
#include "DesktopBackground.h"
#include "LauncherSettings.h"
#include <QFutureWatcher>
#include <QFileInfo>
#include <QScopedValueRollback>
#include <QtConcurrent/QtConcurrentRun>
#include "DashboardWidgetFactory.h"
#include "DashboardFilesWidget.h"
#include <QDir>
#include "DashboardWorkspace.h"
#include "DesktopWindowWidget.h"
#include "PackageDisplayWidget.h"
#include "DashboardDock.h"
#include "DashboardAppsDialog.h"
#include "CanvasIcons.h"
#include "BrowserWidget.h"
#include <QWebEngineView>

#include <QApplication>
#include <QButtonGroup>
#include <QDateTime>
#include <QDialog>
#include <QDialogButtonBox>
#include <QHBoxLayout>
#include <QFrame>
#include <QLabel>
#include <QListWidget>
#include <QMenu>
#include <QPainter>
#include <QPushButton>
#include <QTimer>
#include <QStyle>
#include <QStyleOption>
#include <QStackedWidget>
#include <QVBoxLayout>

DashboardPage::DashboardPage(const DashboardDefinition &definition, DashboardWidgetFactory *factory, QWidget *parent)
    : QWidget(parent), m_definition(definition), m_factory(factory)
{
    setObjectName("dashboardPage");
    setProperty("dashboardId", definition.id);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(16, 12, 16, 12);
    layout->setSpacing(10);
    m_toolbar = new QFrame;
    m_toolbar->setObjectName("dashboardTopBar");
    auto *toolbar = new QHBoxLayout(m_toolbar);
    toolbar->setContentsMargins(12, 5, 12, 5);
    toolbar->setSpacing(6);
    m_sidebarButton = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Sidebar), tr("DockPipe"));
    m_sidebarButton->setObjectName("dashboardFullscreenSidebar");
    m_sidebarButton->setToolTip(tr("Open navigation"));
    m_sidebarButton->hide();
    connect(m_sidebarButton, &QPushButton::clicked, this, &DashboardPage::sidebarRequested);
    toolbar->addWidget(m_sidebarButton);
    m_title = new QLabel(definition.name);
    m_title->setTextFormat(Qt::PlainText);
    m_title->setObjectName("dashboardTitle");
    m_title->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    m_title->setToolTip(definition.name);
    auto font = m_title->font();
    font.setPointSize(font.pointSize() + 4);
    font.setBold(true);
    m_title->setFont(font);
    toolbar->addWidget(m_title, 1);
    toolbar->addStretch(1);
    m_canvas = new QPushButton(tr("Canvas"));
    m_canvas->setObjectName("dashboardCanvasMode");
    m_tiling = new QPushButton(tr("Tiling"));
    m_tiling->setObjectName("dashboardTilingMode");
    auto *modeSwitch = new QFrame;
    modeSwitch->setObjectName("dashboardModeSwitch");
    auto *modeLayout = new QHBoxLayout(modeSwitch);
    modeLayout->setContentsMargins(2, 2, 2, 2);
    modeLayout->setSpacing(2);
    toolbar->addWidget(modeSwitch);
    auto *modeGroup = new QButtonGroup(this);
    for (auto *button : {m_canvas, m_tiling}) {
        button->setCheckable(true);
        modeGroup->addButton(button);
        modeLayout->addWidget(button);
    }
    auto *add = new QPushButton(tr("+ Add widget"));
    add->setObjectName("dashboardAddWidget");
    add->setToolTip(tr("Add dashboard widget"));
    toolbar->addWidget(add);
    auto *settings = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Settings), QString());
    settings->setObjectName("dashboardSettingsButton");
    settings->setAccessibleName(tr("Dashboard settings"));
    settings->setToolTip(tr("Dashboard settings"));
    settings->setFixedWidth(42);
    connect(settings, &QPushButton::clicked, this, &DashboardPage::showSettings);
    toolbar->addWidget(settings);
    m_clock = new QLabel;
    m_clock->setObjectName("dashboardClock");
    m_clock->hide();
    toolbar->addWidget(m_clock);
    m_fullscreenButton = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Fit), tr("Full screen"));
    m_fullscreenButton->setObjectName("dashboardFullscreenButton");
    m_fullscreenButton->setToolTip(tr("Toggle fullscreen · F11"));
    connect(m_fullscreenButton, &QPushButton::clicked, this, &DashboardPage::fullscreenRequested);
    toolbar->insertWidget(toolbar->indexOf(m_clock), m_fullscreenButton);
    layout->addWidget(m_toolbar);
    m_projectButton = new QPushButton;
    m_projectButton->setObjectName("dashboardCanvasProject");
    m_projectButton->setAccessibleName(tr("Change selected project"));
    const auto updateProject = [this](const QString &path) {
        const QString name = path.isEmpty() ? tr("Choose project…") : tr("Project: %1 · Change…").arg(QFileInfo(path).fileName());
        m_projectButton->setProperty("projectCaption", name);
        m_projectButton->setProperty("projectShortCaption", path.isEmpty() ? tr("Choose project") : QFileInfo(path).fileName());
        const QString caption = m_fullscreen ? m_projectButton->property("projectShortCaption").toString() : name;
        m_projectButton->setText(m_projectButton->fontMetrics().elidedText(caption, Qt::ElideMiddle, m_fullscreen ? 170 : 420));
        m_projectButton->setToolTip(path.isEmpty() ? tr("Choose the project used by DockPipe tools") : QDir::toNativeSeparators(path));
    };
    updateProject(m_factory->workdir());
    connect(m_factory, &DashboardWidgetFactory::workdirChanged, m_projectButton, updateProject);
    connect(m_projectButton, &QPushButton::clicked, this, &DashboardPage::projectChangeRequested);
    layout->addWidget(m_projectButton, 0, Qt::AlignLeft);
    m_clockTimer = new QTimer(this);
    m_clockTimer->setInterval(60000);
    m_clockTimer->setTimerType(Qt::VeryCoarseTimer);
    connect(m_clockTimer, &QTimer::timeout, this, &DashboardPage::updateClock);
    m_hint = new QLabel(tr("Your independent canvas. Drag panels to arrange them."));
    m_hint->setWordWrap(true);
    layout->addWidget(m_hint);
    m_errorRow = new QWidget;
    m_errorRow->hide();
    auto *errors = new QHBoxLayout(m_errorRow);
    errors->setContentsMargins(0, 0, 0, 0);
    m_error = new QLabel;
    m_error->setObjectName("dashboardSaveError");
    m_error->setTextFormat(Qt::PlainText);
    m_error->setWordWrap(true);
    m_error->hide();
    m_retry = new QPushButton(tr("Save again"));
    m_retry->hide();
    errors->addWidget(m_error, 1);
    errors->addWidget(m_retry);
    layout->addWidget(m_errorRow);
    m_backgroundStatus = new QLabel;
    m_backgroundStatus->setObjectName("dashboardBackgroundStatus");
    m_backgroundStatus->setTextFormat(Qt::PlainText);
    m_backgroundStatus->setWordWrap(true);
    m_backgroundStatus->hide();
    layout->addWidget(m_backgroundStatus);
    m_environments = new QStackedWidget(this);
    m_environments->setObjectName("dashboardEnvironments");
    m_canvasWorkspace = new DashboardWorkspace(definition.id + "/canvas", m_environments);
    m_tilingWorkspace = new DashboardWorkspace(definition.id + "/tiling", m_environments);
    m_canvasWorkspace->setObjectName("dashboardCanvasWorkspace");
    m_tilingWorkspace->setObjectName("dashboardTilingWorkspace");
    m_tilingWorkspace->setMode("tiling");
    m_environments->addWidget(m_canvasWorkspace);
    m_environments->addWidget(m_tilingWorkspace);
    m_workspace = m_canvasWorkspace;
    layout->addWidget(m_environments, 1);
    m_dock = new DashboardDock;
    layout->addWidget(m_dock, 0, Qt::AlignHCenter);
    auto *dockShortcut = new QShortcut(QKeySequence("Ctrl+Shift+D"), this);
    dockShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(dockShortcut, &QShortcut::activated, this, [this] { m_dock->reveal(true); });
    connect(m_dock, &DashboardDock::addWidgetRequested, this, &DashboardPage::addWidgetKind);
    connect(m_dock, &DashboardDock::appsRequested, this, &DashboardPage::chooseApp);
    connect(m_dock, &DashboardDock::activatePanelRequested, this, [this](const QString &id) {
        if (m_workspace->activatePanel(id))
            m_dock->setActivePanel(id);
    });
    connect(qApp, &QApplication::focusChanged, this, [this](QWidget *, QWidget *focused) {
        if (!isVisible())
            return;
        for (auto *widget = focused; widget && widget != this; widget = widget->parentWidget()) {
            if (widget->objectName().startsWith("dashboardPanel_")) {
                m_dock->setActivePanel(widget->objectName().mid(15));
                return;
            }
        }
    });
    m_saveTimer = new QTimer(this);
    m_saveTimer->setSingleShot(true);
    m_saveTimer->setInterval(350);
    connect(m_saveTimer, &QTimer::timeout, this, &DashboardPage::definitionChanged);
    connect(m_retry, &QPushButton::clicked, this, &DashboardPage::definitionChanged);
    connect(add, &QPushButton::clicked, this, &DashboardPage::chooseWidget);
    connect(m_canvas, &QPushButton::clicked, this, [this]() { setEnvironment("canvas"); });
    connect(m_tiling, &QPushButton::clicked, this, [this]() { setEnvironment("tiling"); });
    for (auto *workspace : {m_canvasWorkspace, m_tilingWorkspace}) {
        connect(workspace, &DashboardWorkspace::layoutChanged, this, &DashboardPage::scheduleSave);
        connect(workspace, &DashboardWorkspace::panelCloseRequested, this, [this, workspace](const QString &id) {
            closePanel(id, workspace);
        });
        connect(workspace, &DashboardWorkspace::panelReleaseRequested, this, [this, workspace](const QString &id) {
            removePanel(id, workspace);
        });
    }
    // Native windows and sharing grants only belong to the current visit.
    const bool removedDesktopPanels = m_definition.panels.removeIf([](const DashboardPanelDefinition &panel) {
        return panel.kind == "desktop-window" || panel.kind == "package-display";
    }) > 0;
    for (const auto &panel : m_definition.panels) {
        auto *content = createContent(panel.kind, panel.environment);
        auto *workspace = workspaceFor(panel.environment);
        if (!workspace->addPanel(panel.id, panelTitle(panel.kind), content)) {
            delete content;
            continue;
        }
        bindWindow(panel.id, content);
    }
    for (const QString &mode : {QString("canvas"), QString("tiling")}) {
        auto *workspace = workspaceFor(mode);
        const auto arrangement = DashboardStore::layoutForEnvironment(m_definition, mode);
        if (!arrangement.isEmpty() && !workspace->restoreLayout(arrangement))
            showSaveError(tr("A saved arrangement is invalid. Its widgets are retained in the default layout."));
        workspace->setMode(mode);
    }
    applyOptions(definition.options);
    setEnvironment(definition.layout.value("mode").toString("canvas"));
    m_loading = false;
    refreshDock();
    if (removedDesktopPanels)
        scheduleSave();
}

void DashboardPage::paintEvent(QPaintEvent *)
{
    // Own the surround between the workspace and page edges. Relying on an
    // ancestor's backing-store fill can expose the desktop during native
    // maximize/resize. Terminal children still clear only their own viewport.
    QPainter painter(this);
    QColor background = palette().color(QPalette::Window);
    background.setAlpha(255);
    painter.fillRect(rect(), background);
    QStyleOption option;
    option.initFrom(this);
    style()->drawPrimitive(QStyle::PE_Widget, &option, &painter, this);
}

DashboardPage::~DashboardPage()
{
    // QWidget destroys children after our value members. Window release signals
    // must not revisit dashboard metadata during that destruction.
    for (auto *window : findChildren<DesktopWindowWidget *>()) {
        disconnect(window, nullptr, this, nullptr);
        window->releaseWindow();
    }
    disconnect(qApp, nullptr, this, nullptr);
}

void DashboardPage::releaseDesktopWindows()
{
    // Release before removing native ancestors. Qt can destroy their platform
    // windows before the child widget destructors get a chance to detach apps.
    for (auto *window : findChildren<DesktopWindowWidget *>())
        window->releaseWindow();

    const auto panels = m_definition.panels;
    for (const auto &panel : panels) {
        if (panel.kind == "desktop-window" || panel.kind == "package-display")
            removePanel(panel.id, workspaceFor(panel.environment));
    }
}

DashboardDefinition DashboardPage::snapshot() const
{
    auto definition = m_definition;
    definition.panels.removeIf([](const DashboardPanelDefinition &panel) {
        return panel.kind == "desktop-window" || panel.kind == "package-display";
    });
    // Desktop panels disappear with their session instead of becoming empty tiles.
    DashboardDefinition canvas = definition;
    canvas.layout = m_canvasWorkspace->saveLayout();
    DashboardDefinition tiling = definition;
    tiling.layout = m_tilingWorkspace->saveLayout();
    definition.layout = {{"version", 2}, {"mode", environment()},
                         {"canvas", DashboardStore::layoutForEnvironment(canvas, "canvas")},
                         {"tiling", DashboardStore::layoutForEnvironment(tiling, "tiling")}};
    return definition;
}

bool DashboardPage::hasPendingChanges() const
{
    return m_saveTimer->isActive();
}

void DashboardPage::setName(const QString &name)
{
    m_definition.name = name;
    m_title->setText(name);
    m_title->setToolTip(name);
}

void DashboardPage::showSaveError(const QString &error)
{
    m_errorRow->setVisible(!error.isEmpty());
    m_error->setText(error);
    m_error->setVisible(!error.isEmpty());
    m_retry->setVisible(!error.isEmpty());
}

void DashboardPage::setCanAddPanel(std::function<bool()> check)
{
    m_canAddPanel = std::move(check);
}

QString DashboardPage::panelTitle(const QString &kind) const
{
    if (kind == "desktop-window" || kind == "package-display")
        return tr("Desktop window");
    for (const auto &descriptor : DashboardWidgetFactory::catalog()) {
        if (descriptor.id == kind)
            return descriptor.title;
    }
    return kind;
}

QWidget *DashboardPage::createContent(const QString &kind, const QString &mode)
{
    if (kind == "package-display")
        return new PackageDisplayWidget;
    if (kind == "desktop-window")
        return new DesktopWindowWidget;
    const QString scope = mode == "tiling" ? m_definition.options.tilingFileScope : m_definition.options.canvasFileScope;
    if (auto *widget = m_factory->create(kind, nullptr, scope)) {
        const bool canvas = mode == "canvas";
        widget->setProperty("dashboardCanvasPresentation", canvas);
        for (auto *child : widget->findChildren<QWidget *>())
            child->setProperty("dashboardCanvasPresentation", canvas);
        if (canvas && kind == "browser") {
            if (auto *browser = widget->findChild<BrowserWidget *>()) {
                connect(browser, &BrowserWidget::tabTearOutRequested, this,
                    [this, browser](QWebEngineView *view, const QPoint &position) {
                        tearOutBrowserTab(browser, view, position);
                    });
            }
        }
        return widget;
    }
    auto *missing = new QLabel(tr("This widget is unavailable: %1. Its saved identity is preserved.").arg(kind));
    missing->setTextFormat(Qt::PlainText);
    missing->setWordWrap(true);
    missing->setAlignment(Qt::AlignCenter);
    return missing;
}

void DashboardPage::tearOutBrowserTab(BrowserWidget *source, QWebEngineView *view, const QPoint &globalPosition)
{
    if (!source || !view || environment() != "canvas" || !addPanel("browser", false))
        return;
    const QString id = m_definition.panels.last().id;
    auto *frame = m_canvasWorkspace->findChild<QWidget *>("dashboardPanel_" + id);
    auto *destination = frame ? frame->findChild<BrowserWidget *>() : nullptr;
    if (!destination || !source->moveTabTo(view, destination)) {
        removePanel(id, m_canvasWorkspace);
        showSaveError(tr("The browser tab could not be moved. Its original tab was kept."));
        return;
    }
    m_canvasWorkspace->placePanelAt(id, m_canvasWorkspace->mapFromGlobal(globalPosition) - QPoint(60, 16));
    m_canvasWorkspace->activatePanel(id);
    scheduleSave();
}

bool DashboardPage::addWidgetKind(const QString &kind)
{
    if (kind.startsWith("package-display:")) {
        const QString identity = kind.mid(QString("package-display:").size());
        for (const auto &display : m_factory->packageDisplays()) {
            if (display.id != identity)
                continue;
            QString error;
            const bool attached = attachPackageDisplay(display, error);
            if (!attached)
                showSaveError(error);
            return attached;
        }
        showSaveError(tr("This package display is no longer available. Refresh its status."));
        return false;
    }
    return addPanel(kind, true);
}

bool DashboardPage::addPanel(const QString &kind, bool chooseWindow)
{
    if (kind == "desktop-window" && environment() != "tiling")
        return false;
    bool known = kind == "desktop-window" || kind == "package-display";
    for (const auto &descriptor : DashboardWidgetFactory::catalog())
        known = known || descriptor.id == kind;
    if (!known)
        return false;
    if (m_definition.panels.size() >= DashboardStore::maximumPanelsPerDashboard || (m_canAddPanel && !m_canAddPanel())) {
        showSaveError(tr("This workspace supports 16 widgets per dashboard and 64 in total. Remove a widget before adding another."));
        return false;
    }
    const QString id = DashboardStore::newId();
    auto *content = createContent(kind, environment());
    if (!m_workspace->addPanel(id, panelTitle(kind), content)) {
        delete content;
        return false;
    }
    m_definition.panels.append({id, kind, environment()});
    bindWindow(id, content);
    refreshDock();
    m_workspace->activatePanel(id);
    m_dock->setActivePanel(id);
    scheduleSave();
    if (chooseWindow && kind == "desktop-window") {
        auto *window = qobject_cast<DesktopWindowWidget *>(content);
        QTimer::singleShot(0, window, [this, window]() {
            if (isVisible() && environment() == "tiling")
                window->chooseWindow();
        });
    }
    return true;
}

void DashboardPage::scheduleSave()
{
    if (!m_loading)
        m_saveTimer->start();
}

void DashboardPage::refreshDock()
{
    QVector<DashboardDockItem> panels;
    for (const auto &panel : m_definition.panels) {
        if (panel.environment != environment())
            continue;
        QString title = panelTitle(panel.kind);
        QIcon icon;
        if (auto *frame = m_workspace->findChild<QWidget *>("dashboardPanel_" + panel.id)) {
            if (auto *window = frame->findChild<DesktopWindowWidget *>()) {
                if (!window->acquiredWindowTitle().isEmpty())
                    title = window->acquiredWindowTitle();
                icon = window->applicationIcon();
            }
        }
        panels.append({panel.id, title, panel.kind, icon});
    }
    m_dock->setDesktopAppsEnabled(environment() == "tiling");
    m_dock->setPanels(panels);
    m_dock->positionOverlay();
}

void DashboardPage::chooseApp()
{
    if (auto *existing = findChild<DashboardAppsDialog *>()) {
        existing->raise();
        existing->activateWindow();
        return;
    }
    m_factory->requestPackageDisplayInventory();
    auto *dialog = new DashboardAppsDialog(m_factory->workflows(), this, environment() == "tiling", m_factory->packageDisplays());
    connect(dialog, &DashboardAppsDialog::packageDisplayRequested, this, [this](const QString &id) {
        addWidgetKind("package-display:" + id);
    });
    connect(m_factory, &DashboardWidgetFactory::packageDisplaysChanged, dialog, [this, dialog]() {
        dialog->setPackageDisplays(m_factory->packageDisplays());
    });
    connect(dialog, &DashboardAppsDialog::cloneRequested, this, &DashboardPage::cloneWorkflowRequested);
    connect(dialog, &DashboardAppsDialog::workflowRequested, this, &DashboardPage::appWorkflowRequested);
    connect(dialog, &DashboardAppsDialog::desktopApplicationRequested,
        this, &DashboardPage::launchDesktopApplication);
    dialog->open();
}

bool DashboardPage::launchDesktopApplication(const DesktopApplicationEntry &application)
{
    if (environment() != "tiling" || !isVisible())
        return false;
    QStringList existingPanels;
    if (application.hasDeclaredWindowClass) {
        for (const auto &panel : m_definition.panels) {
            if (panel.environment != "tiling" || panel.kind != "desktop-window")
                continue;
            auto *frame = m_tilingWorkspace->findChild<QWidget *>("dashboardPanel_" + panel.id);
            auto *desktop = frame ? frame->findChild<DesktopWindowWidget *>() : nullptr;
            if (desktop && desktop->hasInteractiveWindow()
                && desktop->acquiredApplicationClass().compare(application.windowClass, Qt::CaseInsensitive) == 0)
                existingPanels.append(panel.id);
        }
    }
    if (existingPanels.size() == 1) {
        m_tilingWorkspace->activatePanel(existingPanels.first());
        m_dock->setActivePanel(existingPanels.first());
        return true;
    }
    if (existingPanels.size() > 1) {
        showSaveError(tr("This app already has several panels. Select the window you want from the dock."));
        return false;
    }
    // Reserve the destination before dispatching an external application. A full
    // dashboard or a mode switch must not launch an app with nowhere to put it.
    if (!addPanel("desktop-window", false))
        return false;
    const QString id = m_definition.panels.last().id;
    auto *frame = m_tilingWorkspace->findChild<QWidget *>("dashboardPanel_" + id);
    auto *desktop = frame ? frame->findChild<DesktopWindowWidget *>() : nullptr;
    if (!desktop)
        return false;
    m_tilingWorkspace->setPanelTitle(id, application.name);
    desktop->setApplicationIcon(DesktopApplicationCatalog::applicationIcon(application));
    desktop->launchApplication(application);
    refreshDock();
    return true;
}

bool DashboardPage::attachPackageDisplay(const QString &title, qint64 processId, const QString &startToken, QString &error)
{
    return attachPackageDisplay(PackageDisplayMeta{{}, title, processId, startToken}, error);
}

bool DashboardPage::attachPackageDisplay(const PackageDisplayMeta &display, QString &error)
{
    if (display.processId > 0) {
        const auto choices = DesktopWindowWidget::availableWindows(error);
        if (!error.isEmpty() || !DesktopWindowWidget::processWindowId(choices, display.processId, display.processStartToken, error))
            return false;
    } else if (display.nativePackage.interfaceVersion != 1 || !PackageDisplayWidget::validateOptions(display.options, error)) {
        if (error.isEmpty())
            error = tr("Invalid installed package display.");
        return false;
    }
    if (!addPanel("package-display", false)) {
        error = tr("The dashboard cannot accept another display panel.");
        return false;
    }
    const QString id = m_definition.panels.last().id;
    auto *frame = m_workspace->findChild<QWidget *>("dashboardPanel_" + id);
    auto *window = frame ? frame->findChild<PackageDisplayWidget *>() : nullptr;
    if (!window) {
        removePanel(id, m_workspace);
        return false;
    }
    window->setDisplay(display);
    m_workspace->setPanelTitle(id, display.title);
    connect(window, &PackageDisplayWidget::displayFailed, this, &DashboardPage::showSaveError);
    return true;
}

void DashboardPage::addWindowForApplication(const QString &name)
{
    if (environment() != "tiling" || !addWidgetKind("desktop-window"))
        return;
    const QString id = m_definition.panels.last().id;
    if (auto *frame = m_workspace->findChild<QWidget *>("dashboardPanel_" + id)) {
        if (auto *window = frame->findChild<DesktopWindowWidget *>())
            window->setApplicationHint(name);
    }
}

void DashboardPage::setFullscreen(bool fullscreen)
{
    if (m_fullscreen == fullscreen)
        return;
    m_fullscreen = fullscreen;
    m_toolbar->setProperty("dashboardFullscreen", fullscreen);
    auto *toolbarLayout = qobject_cast<QHBoxLayout *>(m_toolbar->layout());
    auto *pageLayout = qobject_cast<QVBoxLayout *>(layout());
    if (fullscreen) {
        pageLayout->removeWidget(m_projectButton);
        toolbarLayout->insertWidget(2, m_projectButton);
    } else {
        toolbarLayout->removeWidget(m_projectButton);
        pageLayout->insertWidget(1, m_projectButton, 0, Qt::AlignLeft);
    }
    m_projectButton->setIcon(fullscreen ? CanvasIcons::icon(CanvasIcons::Symbol::Folder) : QIcon());
    const QString caption = m_projectButton->property(fullscreen ? "projectShortCaption" : "projectCaption").toString();
    m_projectButton->setText(m_projectButton->fontMetrics().elidedText(caption, Qt::ElideMiddle, fullscreen ? 170 : 420));
    toolbarLayout->setStretchFactor(m_title, fullscreen ? 0 : 1);
    m_title->setMaximumWidth(fullscreen ? 220 : QWIDGETSIZE_MAX);
    m_title->setSizePolicy(fullscreen ? QSizePolicy::Preferred : QSizePolicy::Ignored, QSizePolicy::Preferred);
    findChild<QPushButton *>("dashboardSettingsButton")->setFixedWidth(fullscreen ? 32 : 42);
    const bool compact = environment() == "tiling" && m_definition.options.compactTiling;
    QMargins normalMargins = compact ? QMargins(0, 0, 0, 0) : QMargins(16, 12, 16, 12);
    if (m_definition.options.autoHideDock)
        normalMargins.setBottom(0);
    pageLayout->setContentsMargins(fullscreen ? QMargins() : normalMargins);
    m_toolbar->style()->unpolish(m_toolbar);
    m_toolbar->style()->polish(m_toolbar);
    for (auto *child : m_toolbar->findChildren<QWidget *>()) {
        child->style()->unpolish(child);
        child->style()->polish(child);
        child->updateGeometry();
    }
    m_sidebarButton->setVisible(fullscreen);
    m_clock->setVisible(fullscreen);
    m_hint->setVisible(!fullscreen && environment() == "canvas");
    m_fullscreenButton->setText(fullscreen ? QString() : tr("Full screen"));
    m_fullscreenButton->setAccessibleName(fullscreen ? tr("Exit fullscreen") : tr("Full screen"));
    auto *add = findChild<QPushButton *>("dashboardAddWidget");
    add->setText(fullscreen ? QString() : tr("+ Add widget"));
    add->setIcon(fullscreen ? CanvasIcons::icon(CanvasIcons::Symbol::Add) : QIcon());
    add->setAccessibleName(tr("Add dashboard widget"));
    updateClock();
}

void DashboardPage::updateClock()
{
    if (!m_fullscreen || !isVisible()) {
        m_clockTimer->stop();
        return;
    }
    m_clock->setText(QDateTime::currentDateTime().toString("ddd h:mm AP"));
    if (!m_clockTimer->isActive())
        m_clockTimer->start();
}

void DashboardPage::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    updateClock();
    if (environment() == "tiling")
        enterTiling();
    refreshDesktopBackground();
}

void DashboardPage::hideEvent(QHideEvent *event)
{
    QWidget::hideEvent(event);
    m_clockTimer->stop();
}

void DashboardPage::chooseWidget()
{
    auto *dialog = new QDialog(this);
    dialog->setWindowTitle(tr("Add dashboard widget"));
    dialog->setObjectName("dashboardWidgetPicker");
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->resize(510, 520);
    auto *layout = new QVBoxLayout(dialog);
    auto *list = new QListWidget;
    list->setObjectName("dashboardWidgetChoices");
    list->setWordWrap(true);
    for (const auto &descriptor : DashboardWidgetFactory::catalog()) {
        auto *item = new QListWidgetItem(descriptor.title + "\n" + descriptor.description, list);
        item->setData(Qt::UserRole, descriptor.id);
        item->setSizeHint(QSize(420, 64));
    }
    const auto updateDisplays = [this, list]() {
        const QString selected = list->currentItem()
            ? list->currentItem()->data(Qt::UserRole).toString() : QString();
        for (int row = list->count() - 1; row >= 0; --row) {
            if (list->item(row)->data(Qt::UserRole).toString().startsWith("package-display:"))
                delete list->takeItem(row);
        }
        for (const auto &display : m_factory->packageDisplays()) {
            auto *item = new QListWidgetItem(display.title + tr("\nLive package display"), list);
            item->setData(Qt::UserRole, "package-display:" + display.id);
            if (item->data(Qt::UserRole).toString() == selected)
                list->setCurrentItem(item);
        }
    };
    updateDisplays();
    connect(m_factory, &DashboardWidgetFactory::packageDisplaysChanged, dialog, updateDisplays);
    m_factory->requestPackageDisplayInventory();
    if (environment() == "tiling") {
        auto *desktop = new QListWidgetItem(tr("Desktop window\nChoose an app window. Available capture and control depend on your desktop."), list);
        desktop->setData(Qt::UserRole, "desktop-window");
        desktop->setSizeHint(QSize(420, 64));
    }
    layout->addWidget(list);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Ok | QDialogButtonBox::Cancel);
    buttons->button(QDialogButtonBox::Ok)->setText(tr("Add widget"));
    layout->addWidget(buttons);
    connect(buttons, &QDialogButtonBox::rejected, dialog, &QDialog::reject);
    const auto accept = [this, dialog, list]() {
        if (list->currentItem() && addWidgetKind(list->currentItem()->data(Qt::UserRole).toString()))
            dialog->accept();
    };
    connect(buttons, &QDialogButtonBox::accepted, dialog, accept);
    connect(list, &QListWidget::itemDoubleClicked, dialog, accept);
    list->setCurrentRow(0);
    dialog->open();
}

QString DashboardPage::environment() const
{
    return m_workspace == m_tilingWorkspace ? QStringLiteral("tiling") : QStringLiteral("canvas");
}

DashboardWorkspace *DashboardPage::workspaceFor(const QString &environment) const
{
    return environment == "tiling" ? m_tilingWorkspace : m_canvasWorkspace;
}

void DashboardPage::setEnvironment(const QString &mode)
{
    if (mode != "canvas" && mode != "tiling")
        return;
    const bool changed = environment() != mode;
    m_workspace = workspaceFor(mode);
    m_environments->setCurrentWidget(m_workspace);
    m_canvas->setChecked(mode == "canvas");
    m_tiling->setChecked(mode == "tiling");
    m_hint->setVisible(mode == "canvas" && !m_fullscreen);
    m_projectButton->setVisible(mode == "canvas");
    findChild<QPushButton *>("dashboardAddWidget")->setVisible(mode == "canvas");
    const bool compact = mode == "tiling" && m_definition.options.compactTiling;
    if (m_fullscreen) {
        layout()->setContentsMargins(0, 0, 0, 0);
    } else {
        int bottomMargin = compact ? 0 : 12;
        if (m_definition.options.autoHideDock)
            bottomMargin = 0;
        layout()->setContentsMargins(compact ? 0 : 16, compact ? 0 : 12, compact ? 0 : 16, bottomMargin);
    }
    layout()->setSpacing(compact ? 0 : 10);
    refreshDock();
    if (changed && !m_loading) {
        scheduleSave();
        if (isVisible() && mode == "tiling")
            enterTiling();
        if (isVisible())
            refreshDesktopBackground();
    }
}

bool DashboardPage::applyOptions(const DashboardOptions &options)
{
    DashboardDefinition candidate = m_definition;
    candidate.options = options;
    QString error;
    if (!DashboardStore::validate({candidate}, error)) {
        showSaveError(error);
        return false;
    }
    if (options.canvasBackground != "desktop"
        && !m_canvasWorkspace->setCanvasBackground(options.canvasBackground, options.canvasImagePath)) {
        showSaveError(tr("Could not load this background. Choose a PNG, JPEG, WebP or BMP under 20 MiB and 16 megapixels. Your previous background was kept."));
        return false;
    }
    const bool startAcquiring = !m_definition.options.autoAcquireApps && options.autoAcquireApps;
    m_tilingWorkspace->copyBackgroundFrom(*m_canvasWorkspace);
    const bool stopAcquiring = m_definition.options.autoAcquireApps && !options.autoAcquireApps;
    ++m_backgroundGeneration;
    m_backgroundStatus->clear();
    m_backgroundStatus->hide();
    m_definition.options = options;
    updateFileContexts();
    auto *pageLayout = qobject_cast<QVBoxLayout *>(layout());
    if (options.autoHideDock && m_dock->parentWidget() != m_environments) {
        pageLayout->removeWidget(m_dock);
        m_dock->setParent(m_environments);
        m_dock->setOverlayArea(m_environments);
    } else if (!options.autoHideDock && m_dock->parentWidget() == m_environments) {
        m_dock->setOverlayArea(nullptr);
        m_dock->setParent(this);
        pageLayout->addWidget(m_dock, 0, Qt::AlignHCenter);
        m_dock->show();
    }
    for (auto *workspace : {m_canvasWorkspace, m_tilingWorkspace}) {
        workspace->setCompactTiling(options.compactTiling);
        workspace->setShowPanelHeaders(options.showPanelHeaders);
    }
    if (stopAcquiring) {
        const auto acquired = m_automaticPanels.values();
        for (const QString &id : acquired)
            removePanel(id, m_tilingWorkspace);
    }
    setEnvironment(environment());
    scheduleSave();
    if (startAcquiring && !m_loading && isVisible() && environment() == "tiling")
        enterTiling();
    if (!m_loading)
        refreshDesktopBackground();
    return true;
}

void DashboardPage::showSettings()
{
    DashboardSettingsDialog dialog(m_definition.options, window());
    if (m_definition.id == DashboardStore::defaultDashboardId()) {
        dialog.findChild<QPushButton *>("dashboardRenameButton")->hide();
        dialog.findChild<QPushButton *>("dashboardRemoveButton")->hide();
    }
    connect(&dialog, &DashboardSettingsDialog::renameRequested, this, &DashboardPage::renameRequested);
    connect(&dialog, &DashboardSettingsDialog::removeRequested, this, &DashboardPage::removeRequested);
    connect(&dialog, &DashboardSettingsDialog::tourRequested, this, [this]() {
        QTimer::singleShot(0, this, [this]() { showTour(); });
    });
    if (dialog.exec() == QDialog::Accepted)
        applyOptions(dialog.selectedOptions());
}

void DashboardPage::closePanel(const QString &id, DashboardWorkspace *workspace)
{
    if (workspace == m_tilingWorkspace && m_definition.options.autoAcquireApps) {
        auto *frame = workspace->findChild<QWidget *>("dashboardPanel_" + id);
        auto *window = frame ? frame->findChild<DesktopWindowWidget *>() : nullptr;
        if (window && window->acquiredWindowId() && !window->property("packageDisplay").toBool()) {
            QString error;
            if (!window->requestClose(error)) {
                showSaveError(error);
                m_retry->hide();
            }
            // Keep the live panel for save prompts or a cancelled close. The
            // existing native monitor removes it once the OS window is gone.
            return;
        }
    }
    removePanel(id, workspace);
}

void DashboardPage::removePanel(const QString &id, DashboardWorkspace *workspace)
{
    m_automaticPanels.remove(id);
    m_definition.panels.removeIf([&id](const DashboardPanelDefinition &panel) { return panel.id == id; });
    workspace->removePanel(id);
    refreshDock();
    scheduleSave();
}

void DashboardPage::bindWindow(const QString &id, QWidget *content)
{
    auto *window = qobject_cast<DesktopWindowWidget *>(content);
    if (!window)
        window = content->findChild<DesktopWindowWidget *>();
    if (!window)
        return;
    auto *workspace = m_workspace;
    for (const auto &panel : m_definition.panels) {
        if (panel.id == id)
            workspace = workspaceFor(panel.environment);
    }
    const auto update = [this, id, window, workspace]() {
        const QString title = window->acquiredWindowTitle();
        workspace->setPanelTitle(id, title.isEmpty() ? tr("Desktop window") : title);
        workspace->setPanelIcon(id, window->applicationIcon());
        refreshDock();
    };
    connect(window, &DesktopWindowWidget::windowChanged, this, update);
    connect(window, &DesktopWindowWidget::applicationIconChanged, this, update);
    // Remove the panel after the sender has finished releasing native resources.
    connect(window, &DesktopWindowWidget::windowUnavailable, this, [this, id, workspace](const QString &reason) {
        if (!workspace->findChild<QWidget *>("dashboardPanel_" + id))
            return;
        removePanel(id, workspace);
        if (!reason.isEmpty()) {
            showSaveError(reason);
            m_retry->hide();
        }
    }, Qt::QueuedConnection);
}

void DashboardPage::enterTiling()
{
    if (m_loading || !isVisible() || environment() != "tiling")
        return;
    if (!m_definition.options.autoAcquireApps)
        return;
    const QString unavailable = DesktopWindowWidget::automaticAcquisitionUnavailableReason();
    if (!unavailable.isEmpty()) {
        showSaveError(unavailable);
        return;
    }
    QString error;
    const auto available = DesktopWindowWidget::availableWindows(error);
    if (!error.isEmpty()) {
        showSaveError(error);
        return;
    }
    QSet<quint64> acquired;
    for (auto *window : m_tilingWorkspace->findChildren<DesktopWindowWidget *>()) {
        if (window->acquiredWindowId())
            acquired.insert(window->acquiredWindowId());
    }
    for (const auto &choice : available) {
        if (acquired.contains(choice.id))
            continue;
        if (!addPanel("desktop-window", false))
            break;
        const QString id = m_definition.panels.last().id;
        m_automaticPanels.insert(id);
        auto *frame = m_tilingWorkspace->findChild<QWidget *>("dashboardPanel_" + id);
        auto *window = frame->findChild<DesktopWindowWidget *>();
        if (!window->acquireWindow(choice.id, error)) {
            removePanel(id, m_tilingWorkspace);
            showSaveError(error);
        } else {
            acquired.insert(choice.id);
        }
    }
}

void DashboardPage::showTour(bool automatic)
{
    if (m_tourOpen)
        return;
    bool completed = false;
    bool suppressed = false;
    if (!LauncherSettings::loadDashboardTourState(completed, suppressed)) {
        showSaveError(tr("Could not read dashboard tour preferences. Existing settings were kept."));
        return;
    }
    if (automatic && (completed || suppressed))
        return;
    QScopedValueRollback<bool> guard(m_tourOpen, true);
    DashboardTourDialog dialog(m_definition.options, window());
    dialog.setDoNotShowAgain(suppressed);
    const bool finished = dialog.exec() == QDialog::Accepted;
    if (finished) {
        // Apply the accepted mode and options together. Acquisition must only
        // run for the final environment, never an intermediate Tiling state.
        {
            QScopedValueRollback<bool> loading(m_loading, true);
            if (!applyOptions(dialog.selectedOptions()))
                return;
            setEnvironment(dialog.initialEnvironment());
        }
        scheduleSave();
        if (isVisible() && environment() == "tiling")
            enterTiling();
        refreshDesktopBackground();
    }
    // Dismissing never applies draft choices. Only explicit suppression is saved.
    if ((finished || dialog.doNotShowAgain())
        && !LauncherSettings::saveDashboardTourState(completed || finished, dialog.doNotShowAgain()))
        showSaveError(tr("Could not save dashboard tour preferences. You can replay the tour from Dashboard settings."));
}

void DashboardPage::refreshDesktopBackground()
{
    if (m_definition.options.canvasBackground != "desktop" || !isVisible() || m_backgroundLookupPending)
        return;
    m_backgroundLookupPending = true;
    const quint64 generation = m_backgroundGeneration;
    auto *watcher = new QFutureWatcher<DesktopBackgroundResult>(this);
    connect(watcher, &QFutureWatcher<DesktopBackgroundResult>::finished, this, [this, watcher, generation]() {
        const auto result = watcher->result();
        watcher->deleteLater();
        m_backgroundLookupPending = false;
        if (generation != m_backgroundGeneration) {
            refreshDesktopBackground();
            return;
        }
        if (m_definition.options.canvasBackground != "desktop")
            return;
        QString error = result.error;
        if (error.isEmpty()) {
            const QFileInfo image(result.path);
            const qint64 modified = image.lastModified().toMSecsSinceEpoch();
            const bool changed = result.path != m_desktopBackgroundPath
                || modified != m_desktopBackgroundModified || image.size() != m_desktopBackgroundSize;
            if (!m_canvasWorkspace->setCanvasBackground("image", result.path, changed)) {
                error = tr("The wallpaper is not a supported image under 20 MiB and 16 megapixels.");
            } else {
                m_tilingWorkspace->copyBackgroundFrom(*m_canvasWorkspace);
                m_desktopBackgroundPath = result.path;
                m_desktopBackgroundModified = modified;
                m_desktopBackgroundSize = image.size();
            }
        }
        m_backgroundStatus->setText(error.isEmpty() ? QString()
            : tr("Desktop background unavailable: %1 Your previous background was kept.").arg(error));
        m_backgroundStatus->setVisible(!error.isEmpty());
    });
    watcher->setFuture(QtConcurrent::run([]() { return DesktopBackground::resolve(); }));
}

void DashboardPage::updateFileContexts()
{
    for (const auto &panel : m_definition.panels) {
        if (panel.kind != "files")
            continue;
        auto *workspace = workspaceFor(panel.environment);
        auto *frame = workspace->findChild<QWidget *>("dashboardPanel_" + panel.id);
        auto *files = frame ? frame->findChild<DashboardFilesWidget *>() : nullptr;
        if (files) {
            const QString scope = panel.environment == "tiling"
                ? m_definition.options.tilingFileScope : m_definition.options.canvasFileScope;
            files->setContext(scope, m_factory->workdir());
        }
    }
}
