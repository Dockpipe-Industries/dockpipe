#include "BasicModeWidget.h"
#include "AppCardWidget.h"

#include <QAbstractItemView>
#include <QFrame>
#include <QWidget>
#include <QDir>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QLabel>
#include <QListWidget>
#include <QListWidgetItem>
#include <QMenu>
#include <QPushButton>
#include <QResizeEvent>
#include <QSizePolicy>
#include <QStackedWidget>
#include <QStyle>
#include <QTimer>
#include <QVBoxLayout>

namespace {

QString inputDisplayName(const WorkflowInputMeta &input)
{
    const QString display = input.attributes.value(QStringLiteral("displayname")).trimmed();
    if (!display.isEmpty())
        return display;
    if (!input.fieldName.trimmed().isEmpty())
        return input.fieldName.trimmed();
    return input.envName.trimmed();
}

void appendInputTooltipLines(QStringList &lines, const WorkflowInputMeta &input, int depth, int &count, int maxCount)
{
    if (count >= maxCount)
        return;
    const QString indent(depth * 2, QLatin1Char(' '));
    QString line = indent + QStringLiteral("• ") + inputDisplayName(input);
    if (!input.type.trimmed().isEmpty())
        line += QStringLiteral(" (") + input.type.trimmed() + QStringLiteral(")");
    if (!input.envName.trimmed().isEmpty())
        line += QStringLiteral(" → ") + input.envName.trimmed();
    if (!input.defaultValue.trimmed().isEmpty())
        line += QStringLiteral(" = ") + input.defaultValue.trimmed();
    lines << line;
    ++count;
    if (!input.description.trimmed().isEmpty() && count < maxCount) {
        lines << indent + QStringLiteral("  ") + input.description.trimmed();
        ++count;
    }
    for (const WorkflowInputMeta &child : input.children) {
        if (count >= maxCount)
            break;
        appendInputTooltipLines(lines, child, depth + 1, count, maxCount);
    }
}

QIcon defaultWorkflowIcon()
{
    QIcon icon = QIcon::fromTheme(QStringLiteral("applications-development"));
    if (icon.isNull())
        icon = QIcon::fromTheme(QStringLiteral("application-x-executable"));
    if (icon.isNull())
        icon = QIcon::fromTheme(QStringLiteral("applications-system"));
    if (icon.isNull())
        icon = QIcon(QStringLiteral(":/icon.png"));
    return icon;
}

QIcon appIconForWorkflow(const WorkflowMeta &workflow)
{
    if (!workflow.iconPath.isEmpty() && QFileInfo::exists(workflow.iconPath))
        return QIcon(workflow.iconPath);
    return defaultWorkflowIcon();
}

QString workflowTooltip(const WorkflowMeta &workflow)
{
    QStringList lines;
    if (!workflow.description.trimmed().isEmpty())
        lines << workflow.description.trimmed();
    if (!workflow.inputs.isEmpty()) {
        lines << QString() << QObject::tr("Inputs:");
        int count = 0;
        const int maxInputs = 8;
        for (const WorkflowInputMeta &input : workflow.inputs) {
            if (count >= maxInputs)
                break;
            appendInputTooltipLines(lines, input, 0, count, maxInputs);
        }
        if (count >= maxInputs)
            lines << QObject::tr("…and more");
    }
    if (lines.isEmpty())
        return workflow.displayName;
    return lines.join(QLatin1Char('\n'));
}

} // namespace

BasicModeWidget::BasicModeWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName(QStringLiteral("basicMode"));

    m_stack = new QStackedWidget(this);
    auto *outer = new QVBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    outer->addWidget(m_stack);

    // --- Home ---
    m_homePage = new QWidget;
    m_homePage->setObjectName(QStringLiteral("basicHomePage"));
    auto *homeLay = new QVBoxLayout(m_homePage);
    homeLay->setSpacing(12);
    homeLay->setContentsMargins(28, 24, 28, 24);

    auto *homeHero = new QFrame(m_homePage);
    homeHero->setObjectName(QStringLiteral("basicHero"));
    auto *homeHeroLay = new QVBoxLayout(homeHero);
    homeHeroLay->setSpacing(10);
    homeHeroLay->setContentsMargins(18, 18, 18, 18);

    auto *homeTitle = new QLabel(tr("Choose your workspace"));
    homeTitle->setObjectName(QStringLiteral("appTitle"));
    auto *homeSub = new QLabel(
        tr("Open a project to bring its apps and workflows together."));
    homeSub->setObjectName(QStringLiteral("appSubtitle"));
    homeSub->setWordWrap(true);
    homeHeroLay->addWidget(homeTitle);
    homeHeroLay->addWidget(homeSub);

    m_recentList = new QListWidget;
    m_recentList->setObjectName(QStringLiteral("basicRecentList"));
    m_recentList->setSpacing(2);
    m_recentList->setUniformItemSizes(true);
    m_recentList->setVerticalScrollMode(QAbstractItemView::ScrollPerPixel);
    m_recentList->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_recentList->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Minimum);
    auto emitRecent = [this](QListWidgetItem *it) {
        if (!it)
            return;
        const QString p = it->data(Qt::UserRole).toString();
        if (!p.isEmpty())
            emit recentProjectSelected(p);
    };
    connect(m_recentList, &QListWidget::itemClicked, this, emitRecent);
    connect(m_recentList, &QListWidget::itemActivated, this, emitRecent);

    m_homeEmptyHint = new QLabel(tr("No recent projects yet. Use Open project… below."));
    m_homeEmptyHint->setObjectName(QStringLiteral("hintText"));
    m_homeEmptyHint->setWordWrap(true);

    auto *recentPanel = new QFrame(m_homePage);
    recentPanel->setObjectName(QStringLiteral("surfacePanel"));
    auto *recentLay = new QVBoxLayout(recentPanel);
    recentLay->setContentsMargins(16, 16, 16, 16);
    recentLay->setSpacing(10);
    auto *recentTitle = new QLabel(tr("Recent projects"));
    recentTitle->setObjectName(QStringLiteral("sectionTitle"));
    recentLay->addWidget(recentTitle);
    recentLay->addWidget(m_homeEmptyHint);
    recentLay->addWidget(m_recentList, 0);

    auto *homeBtns = new QHBoxLayout;
    homeBtns->setSpacing(8);
    m_openProjectHome = new QPushButton(tr("Open project…"));
    m_openProjectHome->setObjectName(QStringLiteral("primaryButton"));
    m_continueLast = new QPushButton(tr("Continue last project"));
    m_continueLast->setObjectName(QStringLiteral("secondaryButton"));
    m_continueLast->setVisible(false);
    connect(m_openProjectHome, &QPushButton::clicked, this, &BasicModeWidget::openProjectRequested);
    connect(m_continueLast, &QPushButton::clicked, this, &BasicModeWidget::continueLastRequested);
    homeBtns->addWidget(m_openProjectHome);
    homeBtns->addWidget(m_continueLast);
    homeBtns->addStretch(1);

    homeLay->addWidget(homeHero);
    homeLay->addWidget(recentPanel, 0);
    homeLay->addStretch(1);
    homeLay->addLayout(homeBtns);

    // --- Workspace ---
    m_workspacePage = new QWidget;
    m_workspacePage->setObjectName(QStringLiteral("basicWorkspacePage"));
    auto *root = new QVBoxLayout(m_workspacePage);
    root->setSpacing(24);
    root->setContentsMargins(28, 24, 28, 24);

    auto *workspaceHero = new QWidget(m_workspacePage);
    auto *workspaceHeroLay = new QHBoxLayout(workspaceHero);
    workspaceHeroLay->setContentsMargins(0, 0, 0, 0);
    workspaceHeroLay->setSpacing(16);
    auto *heading = new QVBoxLayout;
    heading->setSpacing(6);
    auto *title = new QLabel(tr("Apps"));
    title->setObjectName(QStringLiteral("appTitle"));
    auto *sub = new QLabel(tr("Your installed apps, ready for this workspace."));
    sub->setObjectName(QStringLiteral("appSubtitle"));
    sub->setWordWrap(true);
    heading->addWidget(title);
    heading->addWidget(sub);
    workspaceHeroLay->addLayout(heading, 1);
    m_refresh = new QPushButton(tr("Refresh"));
    m_refresh->setObjectName(QStringLiteral("quietButton"));
    connect(m_refresh, &QPushButton::clicked, this, &BasicModeWidget::onRefresh);
    workspaceHeroLay->addWidget(m_refresh);
    auto *getApps = new QPushButton(tr("Browse packages"));
    getApps->setObjectName(QStringLiteral("secondaryButton"));
    connect(getApps, &QPushButton::clicked, this, &BasicModeWidget::packagesRequested);
    workspaceHeroLay->addWidget(getApps);

    m_loadingBanner = new QLabel;
    m_loadingBanner->setObjectName(QStringLiteral("hintText"));
    m_loadingBanner->setVisible(false);
    m_loadingBanner->setWordWrap(true);

    m_loadingTimer = new QTimer(this);
    m_loadingTimer->setInterval(170);
    connect(m_loadingTimer, &QTimer::timeout, this, &BasicModeWidget::updateLoadingBanner);

    m_appsPage = new QWidget(m_workspacePage);
    m_appsPage->setObjectName(QStringLiteral("basicAppsPage"));
    auto *appsLay = new QVBoxLayout(m_appsPage);
    appsLay->setContentsMargins(0, 0, 0, 0);

    m_list = new QListWidget(m_appsPage);
    m_list->setObjectName(QStringLiteral("basicAppList"));
    m_list->setMovement(QListWidget::Static);
    m_list->setResizeMode(QListWidget::Adjust);
    m_list->setSpacing(12);
    m_list->setWordWrap(true);
    m_list->setContextMenuPolicy(Qt::CustomContextMenu);
    m_list->setMouseTracking(true);
    m_list->setVerticalScrollMode(QAbstractItemView::ScrollPerPixel);
    auto launchItem = [this](QListWidgetItem *it) {
        if (!it)
            return;
        const QString id = it->data(Qt::UserRole).toString();
        if (!id.isEmpty())
            emit launchRequested(id);
    };
    connect(m_list, &QListWidget::itemDoubleClicked, this, launchItem);
    connect(m_list, &QListWidget::itemActivated, this, launchItem);
    connect(m_list, &QListWidget::customContextMenuRequested, this, [this](const QPoint &pos) {
        QListWidgetItem *it = m_list->itemAt(pos);
        if (!it)
            return;
        const QString id = it->data(Qt::UserRole).toString();
        if (id.isEmpty())
            return;
        QMenu menu(this);
        menu.addAction(tr("Launch"), this, [this, id]() { emit launchRequested(id); });
        menu.addAction(tr("Workflow settings…"), this, [this, id]() { emit configureRequested(id); });
        menu.exec(m_list->mapToGlobal(pos));
    });

    appsLay->addWidget(m_list, 1);
    m_emptyApps = new QWidget;
    m_emptyApps->setObjectName(QStringLiteral("appsEmptyState"));
    auto *emptyLayout = new QVBoxLayout(m_emptyApps);
    emptyLayout->setSpacing(12);
    emptyLayout->addStretch();
    auto *emptyGlyph = new QLabel;
    emptyGlyph->setPixmap(QIcon(QStringLiteral(":/icon.png")).pixmap(56, 56));
    emptyLayout->addWidget(emptyGlyph, 0, Qt::AlignHCenter);
    m_emptyAppsTitle = new QLabel;
    m_emptyAppsTitle->setObjectName(QStringLiteral("emptyTitle"));
    m_emptyAppsTitle->setAlignment(Qt::AlignCenter);
    m_emptyAppsBody = new QLabel;
    m_emptyAppsBody->setObjectName(QStringLiteral("appSubtitle"));
    m_emptyAppsBody->setAlignment(Qt::AlignCenter);
    m_emptyAppsBody->setWordWrap(true);
    emptyLayout->addWidget(m_emptyAppsTitle);
    emptyLayout->addWidget(m_emptyAppsBody);
    m_emptyPackages = new QPushButton(tr("Find an app"));
    m_emptyPackages->setObjectName(QStringLiteral("primaryButton"));
    connect(m_emptyPackages, &QPushButton::clicked, this, &BasicModeWidget::packagesRequested);
    emptyLayout->addWidget(m_emptyPackages, 0, Qt::AlignHCenter);
    emptyLayout->addStretch(2);
    appsLay->addWidget(m_emptyApps, 1);
    updateEmptyState();

    m_launchOverlay = new QWidget(m_appsPage);
    m_launchOverlay->setObjectName(QStringLiteral("launchOverlay"));
    m_launchOverlay->setVisible(false);

    auto *overlayLay = new QVBoxLayout(m_launchOverlay);
    overlayLay->setContentsMargins(24, 24, 24, 24);
    overlayLay->addStretch(1);

    m_launchOverlayCard = new QFrame(m_launchOverlay);
    m_launchOverlayCard->setObjectName(QStringLiteral("launchOverlayCard"));
    m_launchOverlayCard->setMaximumWidth(520);
    auto *cardLay = new QVBoxLayout(m_launchOverlayCard);
    cardLay->setContentsMargins(28, 28, 28, 28);
    cardLay->setSpacing(10);

    m_launchOverlayGlyph = new QLabel(m_launchOverlayCard);
    m_launchOverlayGlyph->setObjectName(QStringLiteral("launchOverlayGlyph"));
    m_launchOverlayGlyph->setAlignment(Qt::AlignCenter);

    m_launchOverlayTitle = new QLabel(tr("Launching"));
    m_launchOverlayTitle->setObjectName(QStringLiteral("launchOverlayTitle"));
    m_launchOverlayTitle->setAlignment(Qt::AlignCenter);

    m_launchOverlayBody = new QLabel;
    m_launchOverlayBody->setObjectName(QStringLiteral("launchOverlayBody"));
    m_launchOverlayBody->setWordWrap(true);
    m_launchOverlayBody->setAlignment(Qt::AlignCenter);
    m_launchOverlayBody->setMinimumWidth(320);
    m_launchOverlayBody->setMaximumWidth(420);
    m_launchOverlayBody->setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Minimum);

    cardLay->addWidget(m_launchOverlayGlyph);
    cardLay->addWidget(m_launchOverlayTitle);
    cardLay->addWidget(m_launchOverlayBody);

    overlayLay->addWidget(m_launchOverlayCard, 0, Qt::AlignHCenter);
    overlayLay->addStretch(1);

    root->addWidget(workspaceHero);
    root->addWidget(m_loadingBanner);
    root->addWidget(m_appsPage, 1);

    m_stack->addWidget(m_homePage);
    m_stack->addWidget(m_workspacePage);

    applyViewMode();
}

void BasicModeWidget::showHomePage()
{
    m_stack->setCurrentWidget(m_homePage);
}

void BasicModeWidget::showWorkspacePage()
{
    m_stack->setCurrentWidget(m_workspacePage);
    updateResponsiveMetrics();
    updateLaunchOverlayGeometry();
}

void BasicModeWidget::setRecentProjects(const QStringList &paths)
{
    m_recentPaths = paths;
    rebuildRecentList();
}

void BasicModeWidget::setContinueLastVisible(bool visible)
{
    m_continueLast->setVisible(visible);
}

void BasicModeWidget::rebuildRecentList()
{
    m_recentList->clear();
    for (const QString &p : m_recentPaths) {
        if (p.isEmpty())
            continue;
        const QFileInfo fi(p);
        auto *it = new QListWidgetItem;
        it->setText(fi.isDir() ? fi.fileName() : fi.filePath());
        it->setData(Qt::UserRole, QDir::cleanPath(p));
        it->setToolTip(QDir::toNativeSeparators(QDir::cleanPath(p)));
        m_recentList->addItem(it);
    }
    const bool empty = m_recentList->count() == 0;
    m_homeEmptyHint->setVisible(empty);
    m_recentList->setVisible(!empty);
    if (!empty) {
        const int n = m_recentList->count();
        const int rowH = 36;
        const int chrome = 8;
        const int maxVisible = 6;
        const int h = qMin(n, maxVisible) * rowH + chrome;
        m_recentList->setMinimumHeight(h);
        m_recentList->setMaximumHeight(h);
    } else {
        m_recentList->setMinimumHeight(0);
        m_recentList->setMaximumHeight(QWIDGETSIZE_MAX);
    }
}

void BasicModeWidget::applyViewMode()
{
    if (m_iconMode) {
        m_list->setViewMode(QListWidget::IconMode);
        m_list->setFlow(QListView::LeftToRight);
        m_list->setWrapping(true);
        m_list->setUniformItemSizes(false);
    } else {
        m_list->setViewMode(QListWidget::ListMode);
        m_list->setFlow(QListView::TopToBottom);
        m_list->setWrapping(false);
        m_list->setUniformItemSizes(true);
    }
    updateResponsiveMetrics();
}

void BasicModeWidget::setViewIconMode(bool icons)
{
    if (m_iconMode == icons)
        return;
    m_iconMode = icons;
    applyViewMode();
    rebuildItemTexts();
}

void BasicModeWidget::setApps(const QVector<WorkflowMeta> &apps)
{
    m_apps = apps;
    // Qt defers item-widget deletion; hide outgoing cards before rebuilding.
    for (int i = 0; i < m_list->count(); ++i) {
        if (auto *card = m_list->itemWidget(m_list->item(i)))
            card->hide();
    }
    m_list->clear();
    for (const WorkflowMeta &m : apps) {
        auto *it = new QListWidgetItem;
        it->setIcon(appIconForWorkflow(m));
        it->setData(Qt::UserRole, m.workflowId);
        it->setToolTip(workflowTooltip(m));
        m_list->addItem(it);
    }
    updateResponsiveMetrics();
    rebuildItemTexts();
    updateEmptyState();
}

void BasicModeWidget::setAppDiscoveryLoading(bool loading)
{
    m_appDiscoveryLoading = loading;
    updateEmptyState();
    if (m_launchingWorkflowId.isEmpty() && m_loadingBanner) {
        if (loading) {
            m_loadingBanner->setText(tr("Loading apps from Dockpipe..."));
            m_loadingBanner->setVisible(true);
        } else {
            m_loadingBanner->clear();
            m_loadingBanner->setVisible(false);
        }
    }
}

void BasicModeWidget::rebuildItemTexts()
{
    for (int i = 0; i < m_list->count() && i < m_apps.size(); ++i) {
        const WorkflowMeta &m = m_apps[i];
        QListWidgetItem *it = m_list->item(i);
        const bool run = m_running.value(m.workflowId, false);
        const bool launching = (m.workflowId == m_launchingWorkflowId);
        it->setText(QString());
        it->setIcon(QIcon());
        auto *card = new AppCardWidget(m, appIconForWorkflow(m), !m_iconMode, run, launching, m_list);
        connect(card, &AppCardWidget::launchRequested, this, [this, id = m.workflowId]() { emit launchRequested(id); });
        connect(card, &AppCardWidget::configureRequested, this, [this, id = m.workflowId]() { emit configureRequested(id); });
        if (auto *previous = m_list->itemWidget(it))
            previous->hide();
        m_list->setItemWidget(it, card);
    }
}

void BasicModeWidget::updateResponsiveMetrics()
{
    if (!m_list)
        return;
    const int availableWidth = qMax(220, m_list->viewport()->width() - 24);
    const int columns = qMax(1, availableWidth / 270);
    const int cellWidth = qMin(340, (availableWidth - (columns - 1) * 12) / columns);
    m_list->setGridSize(m_iconMode ? QSize(cellWidth, 206) : QSize());
    for (int i = 0; i < m_list->count(); ++i)
        m_list->item(i)->setSizeHint(m_iconMode ? QSize(cellWidth - 12, 194) : QSize(0, 86));
}

void BasicModeWidget::setRunningByWorkflow(const QHash<QString, bool> &running)
{
    if (m_running == running)
        return;
    m_running = running;
    rebuildItemTexts();
}

void BasicModeWidget::onRefresh()
{
    emit refreshAppsRequested();
}

void BasicModeWidget::updateEmptyState()
{
    if (!m_emptyApps)
        return;
    const bool empty = m_apps.isEmpty();
    m_emptyApps->setVisible(empty);
    m_list->setVisible(!empty);
    m_emptyAppsTitle->setText(m_appDiscoveryLoading ? tr("Finding your apps…") : tr("Make this workspace yours"));
    m_emptyAppsBody->setText(m_appDiscoveryLoading ? tr("Checking the installed app catalog.")
        : tr("No apps are installed for this workspace yet.\nBrowse packages to add the tools you use."));
    m_emptyPackages->setVisible(!m_appDiscoveryLoading);
}

void BasicModeWidget::setLaunchingWorkflow(const QString &workflowId, const QString &displayName)
{
    m_launchingWorkflowId = workflowId;
    m_launchingWorkflowName = displayName;
    m_loadingFrame = 0;
    updateLoadingBanner();
    m_loadingBanner->setVisible(false);
    if (m_launchOverlay) {
        m_launchOverlay->setVisible(true);
        m_launchOverlay->raise();
        updateLaunchOverlayGeometry();
    }
    if (m_loadingTimer)
        m_loadingTimer->start();
    rebuildItemTexts();
}

void BasicModeWidget::clearLaunchingWorkflow()
{
    if (m_launchingWorkflowId.isEmpty())
        return;
    m_launchingWorkflowId.clear();
    m_launchingWorkflowName.clear();
    if (m_loadingTimer)
        m_loadingTimer->stop();
    if (m_loadingBanner) {
        m_loadingBanner->clear();
        m_loadingBanner->setVisible(false);
    }
    if (m_launchOverlay)
        m_launchOverlay->setVisible(false);
    rebuildItemTexts();
}

void BasicModeWidget::updateLoadingBanner()
{
    if (m_launchingWorkflowId.isEmpty()) {
        return;
    }
    static const QStringList frames = {
        QStringLiteral("▖▘▝▗"),
        QStringLiteral("▘▝▗▖"),
        QStringLiteral("▝▗▖▘"),
        QStringLiteral("▗▖▘▝"),
    };
    const QString name = m_launchingWorkflowName.isEmpty() ? tr("workflow") : m_launchingWorkflowName;
    const QString frame = frames[m_loadingFrame % frames.size()];
    if (m_loadingBanner)
        m_loadingBanner->setText(tr("Launching %1  %2").arg(name, frame));
    if (m_launchOverlayGlyph)
        m_launchOverlayGlyph->setText(frame);
    if (m_launchOverlayTitle)
        m_launchOverlayTitle->setText(tr("Launching %1").arg(name));
    if (m_launchOverlayBody) {
        m_launchOverlayBody->setText(
            tr("Dockpipe is preparing the workflow, warming the session, and opening the app shell."));
    }
    m_loadingFrame += 1;
}

void BasicModeWidget::updateLaunchOverlayGeometry()
{
    if (!m_launchOverlay || !m_appsPage)
        return;
    const QRect r = m_appsPage->rect();
    m_launchOverlay->setGeometry(r.adjusted(12, 12, -12, -12));
}

void BasicModeWidget::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    updateResponsiveMetrics();
    updateLaunchOverlayGeometry();
}
