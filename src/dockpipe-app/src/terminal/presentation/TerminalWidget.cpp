#include "TerminalWidget.h"
#include "TerminalHub.h"
#include "TerminalShellCatalog.h"
#include "NativeHostConnection.h"
#include "TerminalBridge.h"
#include "TerminalPreferences.h"
#include "TerminalSettingsDialog.h"
#include "CanvasIcons.h"

#include <QComboBox>
#include <QTabBar>
#include <QToolButton>
#include <QAction>
#include <QApplication>
#include <QClipboard>
#include <QJsonArray>
#include <QJsonDocument>
#include <QFileInfo>
#include <QPointer>
#include <QPainter>
#include <QShortcut>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QMenu>
#include <QPushButton>
#include <QShowEvent>
#include <QSignalBlocker>
#include <QStandardItemModel>
#include <QTimer>
#include <QVBoxLayout>
#include <QWebChannel>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineSettings>
#include <QWebEngineUrlRequestInterceptor>
#include <QWebEngineUrlRequestInfo>
#include <QWebEngineView>

static void initializeTerminalResources()
{
    Q_INIT_RESOURCE(terminal);
}

namespace {
QString remoteContainerTarget(const QJsonObject &catalog, const QString &container)
{
    const auto target = catalog.value("target").toObject();
    return QStringLiteral("remote:%1:%2:%3")
        .arg(target.value("fingerprint").toString(), target.value("generation").toString(), container);
}

// The privileged channel is never exposed to arbitrary web content.
class TerminalRequests final : public QWebEngineUrlRequestInterceptor {
public:
    using QWebEngineUrlRequestInterceptor::QWebEngineUrlRequestInterceptor;
    void interceptRequest(QWebEngineUrlRequestInfo &info) override
    {
        const QUrl url = info.requestUrl();
        const bool allowed = url.scheme() == "qrc"
            && (url.path().startsWith("/terminal/") || url.path() == "/qtwebchannel/qwebchannel.js");
        info.block(!allowed);
    }
};

class TerminalPage final : public QWebEnginePage {
public:
    TerminalPage(QWebEngineProfile *profile, QObject *parent) : QWebEnginePage(profile, parent) {}
protected:
    bool acceptNavigationRequest(const QUrl &url, NavigationType, bool isMainFrame) override
    {
        return isMainFrame && url == QUrl("qrc:/terminal/index.html");
    }
    QWebEnginePage *createWindow(WebWindowType) override { return nullptr; }
};
}

TerminalWidget::TerminalWidget(TerminalHub &hub, const QString &workdir, QWidget *parent, Mode mode)
    : QWidget(parent), m_hub(hub), m_hubGuard(&hub), m_mode(mode), m_workdir(workdir)
{
    setObjectName("terminalWidget");
    setMinimumSize(260, 180);
    auto *layout = new QVBoxLayout(this);
    const bool dashboard = mode == Mode::Dashboard;
    const int margin = dashboard ? 0 : 16;
    layout->setContentsMargins(margin, margin, margin, margin);
    layout->setSpacing(dashboard ? 0 : 8);
    auto *toolbarWidget = new QWidget;
    toolbarWidget->setObjectName("terminalToolbar");
    auto *toolbar = new QHBoxLayout(toolbarWidget);
    toolbar->setContentsMargins(0, 0, 0, 0);
    toolbar->setSpacing(8);
    auto *title = new QLabel(tr("Terminal"));
    title->setObjectName("terminalHeading");
    toolbar->addWidget(title);
    if (!dashboard) {
        m_targets = new QComboBox;
        m_targets->setObjectName("terminalTargets");
        m_targets->setAccessibleName(tr("Terminal target"));
        m_targets->setMinimumContentsLength(16);
        m_targets->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
        toolbar->addWidget(m_targets);
        refreshTargets();
        connect(m_targets, &QComboBox::activated, this, [this](int index) {
            selectTarget(m_targets->itemData(index).toString());
        });
    }
    m_tabs = new QTabBar;
    m_tabs->setObjectName("terminalTabs");
    m_tabs->setExpanding(false);
    m_tabs->setUsesScrollButtons(true);
    m_tabs->setElideMode(Qt::ElideMiddle);
    m_tabs->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    toolbar->addWidget(m_tabs, 1);
    // Keep the shared session-selection route for existing embedded consumers.
    m_sessions = new QComboBox(this);
    m_sessions->setObjectName("terminalSessions");
    m_sessions->hide();
    m_shells = new QComboBox(this);
    m_shells->setObjectName("terminalShells");
    m_shells->hide();
    m_shellMenu = new QMenu(tr("Shells and sessions"), this);
    m_shellMenu->setObjectName("terminalShellMenu");
    auto *create = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Add), tr("New terminal"));
    create->setObjectName("terminalNew");
    create->setToolTip(tr("Open a new terminal (Ctrl+Shift+T)"));
    toolbar->addWidget(create);
    auto *shellChoices = new QToolButton;
    shellChoices->setObjectName("terminalShellChoices");
    shellChoices->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::ChevronDown));
    shellChoices->setToolTip(tr("Choose a shell for a new terminal"));
    shellChoices->setAccessibleName(tr("Choose terminal shell"));
    shellChoices->setPopupMode(QToolButton::InstantPopup);
    shellChoices->setMenu(m_shellMenu);
    toolbar->addWidget(shellChoices);
    auto *refreshShellsButton = new QPushButton(this);
    refreshShellsButton->setObjectName("terminalRefreshShells");
    refreshShellsButton->hide();
    connect(refreshShellsButton, &QPushButton::clicked, &m_hub.shellCatalog(), &TerminalShellCatalog::refresh);
    m_close = new QPushButton(this);
    m_close->setObjectName("terminalClose");
    m_close->hide();
    auto *settingsButton = new QPushButton;
    settingsButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Settings));
    settingsButton->setObjectName("terminalSettings");
    settingsButton->setAccessibleName(tr("Terminal settings"));
    settingsButton->setToolTip(tr("Terminal settings"));
    settingsButton->setFixedSize(32, 32);
    toolbar->addWidget(settingsButton);
    connect(settingsButton, &QPushButton::clicked, this, &TerminalWidget::showSettings);
    connect(TerminalPreferences::instance(), &TerminalPreferences::changed, this, &TerminalWidget::applyAppearance);
    layout->addWidget(toolbarWidget);
    toolbarWidget->setVisible(!dashboard);
    m_status = new QLabel(tr("Open a terminal or select an existing session."));
    m_status->setObjectName("terminalStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    m_status->hide();
    m_transparencyHint = new QLabel(tr("Restart DockPipe to enable desktop transparency. Your desktop must support compositing."));
    m_transparencyHint->setObjectName("terminalTransparencyHint");
    m_transparencyHint->setWordWrap(true);
    m_transparencyHint->hide();
    layout->addWidget(m_transparencyHint);
    m_empty = new QWidget(this);
    m_empty->setObjectName("terminalEmpty");
    auto *emptyLayout = new QVBoxLayout(m_empty);
    emptyLayout->addStretch();
    auto *emptyTitle = new QLabel(tr("No terminals open"));
    emptyTitle->setObjectName("terminalEmptyTitle");
    emptyTitle->setAlignment(Qt::AlignCenter);
    emptyLayout->addWidget(emptyTitle);
    auto *emptyHint = new QLabel(tr("Open a terminal to work in this workspace."));
    emptyHint->setObjectName("terminalEmptyHint");
    emptyHint->setAlignment(Qt::AlignCenter);
    emptyHint->setWordWrap(true);
    emptyLayout->addWidget(emptyHint);
    emptyLayout->addStretch();
    layout->addWidget(m_empty, 1);
    m_flushTimer = new QTimer(this);
    m_flushTimer->setSingleShot(true);
    m_flushTimer->setInterval(16);
    connect(m_flushTimer, &QTimer::timeout, this, &TerminalWidget::flushOutput);
    connect(create, &QPushButton::clicked, this, [this]() {
        createTerminal(m_shells->currentData().toString());
    });
    connect(m_close, &QPushButton::clicked, this, [this]() { m_hub.closeSession(m_sessionId); });
    connect(m_tabs, &QTabBar::currentChanged, this, [this](int index) {
        if (m_mode == Mode::Managed && index >= 0)
            selectSession(m_tabs->tabData(index).toString());
    });
    connect(m_tabs, &QTabBar::tabBarClicked, this, [this](int index) {
        if (m_mode == Mode::Managed && index >= 0)
            selectSession(m_tabs->tabData(index).toString());
    });
    auto *newShortcut = new QShortcut(QKeySequence(Qt::CTRL | Qt::SHIFT | Qt::Key_T), this);
    newShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(newShortcut, &QShortcut::activated, create, &QPushButton::click);
    connect(m_sessions, &QComboBox::currentIndexChanged, this, [this]() {
        if (m_mode == Mode::Managed)
            selectSession(m_sessions->currentData().toString());
    });
    connect(&m_hub, &TerminalHub::sessionsChanged, this, &TerminalWidget::refreshSessions);
    connect(&m_hub, &TerminalHub::outputAvailable, this, [this](const QString &id) {
        if (id == m_sessionId)
            scheduleOutput();
    });
    if (dashboard) {
        setContextMenuPolicy(Qt::ActionsContextMenu);
        auto *settingsAction = new QAction(tr("Terminal settings…"), this);
        settingsAction->setObjectName("terminalSettingsAction");
        addAction(settingsAction);
        connect(settingsAction, &QAction::triggered, this, &TerminalWidget::showSettings);
        auto *open = new QAction(tr("Open full view"), this);
        open->setObjectName("terminalOpenFullViewAction");
        addAction(open);
        connect(open, &QAction::triggered, this, &TerminalWidget::openFullViewRequested);
        m_restart = new QAction(tr("Start terminal"), this);
        m_restart->setObjectName("terminalStartAction");
        addAction(m_restart);
        connect(m_restart, &QAction::triggered, this, &TerminalWidget::startDashboardSession);
        addAction(m_shellMenu->menuAction());
    }
    connect(&m_hub, &TerminalHub::shellProfilesChanged, this, [this] {
        refreshShells();
        if (m_mode == Mode::Dashboard && isVisible() && !m_dashboardStarted)
            startDashboardSession();
    });
    connect(&m_hub.shellCatalog(), &TerminalShellCatalog::discoveryFinished, this, [this] {
        refreshShells();
        if (!isVisible())
            return;
        if (m_mode == Mode::Dashboard && !m_dashboardStarted)
            startDashboardSession();
        else if (m_mode == Mode::Managed)
            startManagedSession();
    });
    refreshShells();
    refreshSessions();
    applyAppearance();
}

TerminalWidget::~TerminalWidget()
{
    m_flushTimer->stop();
    if (m_hubGuard) {
        disconnect(m_hubGuard, nullptr, this, nullptr);
        for (const auto &id : m_ownedSessionIds)
            m_hubGuard->closeSession(id);
    }
}

void TerminalWidget::startDashboardSession()
{
    if (m_mode != Mode::Dashboard || !m_hubGuard || m_hub.running(m_ownedSessionId))
        return;
    for (const auto &id : m_hub.sessionIds(m_machineCatalog, m_machineRoot)) {
        if (m_ownedSessionIds.contains(id) && m_hub.running(id)) {
            m_ownedSessionId = id;
            m_dashboardStarted = true;
            selectSession(id);
            return;
        }
    }
    const bool flatpak = !qEnvironmentVariable("FLATPAK_ID").isEmpty() || QFileInfo::exists("/.flatpak-info");
    if (m_machineCatalog.isEmpty() && m_containerId.isEmpty() && !m_hub.nativeHostConfigured()
        && m_hub.shellCatalog().discovering() && flatpak) {
        showStatus(tr("Discovering installed host shells…"));
        return;
    }
    if (!m_ownedSessionId.isEmpty())
        m_hub.closeSession(m_ownedSessionId);
    createTerminal(m_shells->currentData().toString());
}

void TerminalWidget::startManagedSession()
{
    if (m_mode != Mode::Managed || m_managedStarted || !m_hubGuard)
        return;
    if (!m_sessionId.isEmpty()) {
        m_managedStarted = true;
        return;
    }
    for (const auto &id : m_hub.sessionIds(m_machineCatalog, m_machineRoot)) {
        if (m_hub.running(id)) {
            m_managedStarted = true;
            selectSession(id);
            return;
        }
    }
    const bool flatpak = !qEnvironmentVariable("FLATPAK_ID").isEmpty() || QFileInfo::exists("/.flatpak-info");
    if (m_containerId.isEmpty() && m_machineCatalog.isEmpty() && !m_hub.nativeHostConfigured()
        && flatpak && m_hub.shellCatalog().discovering()) {
        showStatus(tr("Discovering installed host shells…"));
        return;
    }
    m_managedStarted = true;
    createTerminal();
}

void TerminalWidget::createTerminal(const QString &shellProfileId)
{
    if (!explicitTargetAvailable()) {
        showStatus(tr("The selected terminal target is unavailable."));
        return;
    }
    QString id;
    if (m_containerId.isEmpty())
        id = m_hub.createSession(m_workdir, shellProfileId, m_machineCatalog, m_machineRoot);
    else if (m_machineCatalog.isEmpty())
        id = m_hub.createLocalContainerSession(m_containerId, m_containerName, m_containerShell);
    else
        id = m_hub.createContainerSession(m_machineCatalog, m_containerId, m_containerShell);
    if (id.isEmpty()) {
        showStatus(m_hub.creationError());
        return;
    }
    if (m_mode == Mode::Dashboard) {
        m_dashboardStarted = true;
        m_ownedSessionId = id;
        m_ownedSessionIds.append(id);
    }
    selectSession(id);
}

void TerminalWidget::refreshShells()
{
    const QString selected = m_shells->currentData().toString();
    QSignalBlocker blocker(m_shells);
    m_shells->clear();
    const bool usesShellProfiles = m_machineCatalog.isEmpty() && m_containerId.isEmpty();
    const bool nativeDefault = usesShellProfiles && m_hub.nativeHostConfigured();
    m_shells->addItem(nativeDefault ? tr("Default · native host") : tr("Default shell"), QString());
    const auto profiles = usesShellProfiles ? m_hub.shellProfiles() : QVector<TerminalShellProfile>{};
    for (const auto &profile : profiles)
        m_shells->addItem(profile.label, profile.id);
    m_shells->setCurrentIndex(qMax(0, m_shells->findData(selected)));
    if (!m_shellMenu)
        return;
    m_shellMenu->clear();
    auto *defaultShell = m_shellMenu->addAction(tr("New default shell"));
    connect(defaultShell, &QAction::triggered, this, [this] {
        m_shells->setCurrentIndex(0);
        createTerminal();
    });
    m_shellMenu->addSeparator();
    auto *refresh = m_shellMenu->addAction(tr("Refresh installed shells"));
    connect(refresh, &QAction::triggered, &m_hub.shellCatalog(), &TerminalShellCatalog::refresh);
    m_shellMenu->addSeparator();
    for (const auto &profile : profiles) {
        auto *action = m_shellMenu->addAction(tr("New %1").arg(profile.label));
        action->setData(profile.id);
        connect(action, &QAction::triggered, this, [this, profile] {
            m_shells->setCurrentIndex(qMax(0, m_shells->findData(profile.id)));
            createTerminal(profile.id);
        });
    }
    if (!m_ownedSessionIds.isEmpty())
        m_shellMenu->addSeparator();
    for (const auto &id : m_ownedSessionIds) {
        if (!m_hub.sessionIds(m_machineCatalog, m_machineRoot).contains(id))
            continue;
        auto *action = m_shellMenu->addAction(m_hub.title(id));
        action->setCheckable(true);
        action->setChecked(id == m_sessionId);
        connect(action, &QAction::triggered, this, [this, id] {
            m_ownedSessionId = id;
            selectSession(id);
        });
    }
}

void TerminalWidget::showSettings()
{
    TerminalSettingsDialog dialog(window());
    dialog.exec();
}

void TerminalWidget::showStatus(const QString &message, bool issue)
{
    m_status->setText(message);
    m_status->setVisible(issue && !message.isEmpty());
}

void TerminalWidget::setWorkdir(const QString &workdir) { m_workdir = workdir; }

bool TerminalWidget::setMachineContext(const QJsonObject &catalog, const QString &root, QString *error)
{
    return setTerminalContext(catalog, root, {}, {}, error);
}

bool TerminalWidget::setContainerContext(const QJsonObject &catalog, const QString &container, const QString &shell, QString *error)
{
    return setTerminalContext(catalog, "container:" + container + ":" + shell, container, shell, error);
}

void TerminalWidget::setWorkspaceTarget(const QJsonObject &catalog, const QString &root)
{
    QString terminalRoot = root;
    if (terminalRoot.isEmpty()) {
        for (const auto &entry : catalog.value("roots").toArray()) {
            const auto permission = entry.toObject();
            if (permission.value("id").toString() == "home" && permission.value("terminal").toBool()) {
                terminalRoot = QStringLiteral("home");
                break;
            }
        }
    }

    const bool sameWorkspace = m_workspaceCatalog.value("target") == catalog.value("target") && m_workspaceRoot == terminalRoot;
    m_workspaceCatalog = catalog;
    m_workspaceRoot = terminalRoot;
    if (m_remoteContainerCatalog.value("target") != catalog.value("target")
        || !catalog.value("docker_exec").toBool() || !catalog.value("docker_read").toBool()) {
        m_remoteContainerCatalog = {};
        m_remoteContainers.clear();
    }
    refreshTargets();
    if (m_target.isEmpty() && sameWorkspace)
        updateMachineCatalog(catalog);
    else if (m_target.isEmpty())
        setMachineContext(catalog, terminalRoot);
    else
        updateNewTargetTerminal();
}

void TerminalWidget::setMachineTargets(const QStringList &paired, const QJsonArray &catalog, bool unlocked)
{
    m_pairedMachines = paired;
    m_machineTargetsCatalog = catalog;
    m_machineTargetsUnlocked = unlocked;
    refreshTargets();
    if (m_hostTargets.contains(m_target))
        updateMachineCatalog(m_hostTargets.value(m_target).first);
    updateNewTargetTerminal();
}

void TerminalWidget::setEnvironmentContainers(const QMap<QString, QString> &containers)
{
    if (m_environmentContainers == containers)
        return;

    m_environmentContainers = containers;
    refreshTargets();
    updateNewTargetTerminal();
}

void TerminalWidget::setRemoteContainers(const QJsonObject &catalog, const QMap<QString, QString> &containers)
{
    if (catalog.value("target") != m_workspaceCatalog.value("target"))
        return;

    m_remoteContainerCatalog = catalog;
    m_remoteContainers = containers;
    if (m_target.startsWith("remote:"))
        updateMachineCatalog(catalog);
    refreshTargets();
    updateNewTargetTerminal();
}

bool TerminalWidget::openEnvironmentContainer(const QString &container)
{
    if (!m_environmentContainers.contains(container))
        return false;

    selectTarget("local:" + container);
    if (!m_hub.running(m_sessionId)) {
        m_managedStarted = false;
        selectSession({});
        if (isVisible())
            startManagedSession();
    }
    return true;
}

void TerminalWidget::refreshTargets()
{
    if (!m_targets)
        return;

    const QSignalBlocker blocker(m_targets);
    m_targets->clear();
    m_hostTargets.clear();
    const auto workspace = m_workspaceCatalog.value("target").toObject();
    const auto node = workspace.value("node").toString();
    m_targets->addItem(node.isEmpty() ? tr("This computer") : node, QString());
    if (!node.isEmpty() || m_target == "host:local") {
        const auto key = QStringLiteral("host:local");
        m_targets->addItem(tr("This computer"), key);
        m_hostTargets.insert(key, {});
    }
    QMap<QString, QJsonObject> machines;
    for (const auto &name : m_pairedMachines) {
        if (!name.isEmpty())
            machines.insert(name, {});
    }
    for (const auto &entry : m_machineTargetsCatalog) {
        const auto catalog = entry.toObject();
        const auto name = catalog.value("target").toObject().value("node").toString();
        if (!name.isEmpty() && (!machines.contains(name) || !catalog.value("workflow_only").toBool()))
            machines.insert(name, catalog);
    }
    for (auto machine = machines.cbegin(); machine != machines.cend(); ++machine) {
        const auto target = machine.value().value("target").toObject();
        QVector<QJsonObject> roots;
        if (m_machineTargetsUnlocked && !machine.value().value("terminal_epoch").toString().isEmpty()) {
            for (const auto &entry : machine.value().value("roots").toArray()) {
                const auto root = entry.toObject();
                const auto id = root.value("id").toString();
                if (!root.value("terminal").toBool() || id.isEmpty())
                    continue;
                if (id == "home")
                    roots.prepend(root);
                else
                    roots.append(root);
            }
        }
        if (roots.isEmpty() && machine.key() != node) {
            const auto suffix = m_machineTargetsUnlocked ? tr("Terminal unavailable") : tr("Remote access locked");
            m_targets->addItem(tr("%1 · %2").arg(machine.key(), suffix), "unavailable:" + machine.key());
            if (auto *model = qobject_cast<QStandardItemModel *>(m_targets->model()))
                model->item(m_targets->count() - 1)->setEnabled(false);
        }
        for (const auto &root : roots) {
            const auto id = root.value("id").toString();
            const auto key = QStringLiteral("host:%1:%2:%3:%4").arg(machine.key(),
                target.value("fingerprint").toString(), target.value("generation").toString(), id);
            if (target == workspace && id == m_workspaceRoot && key != m_target)
                continue;
            const auto label = roots.size() == 1 || id == "home" ? machine.key()
                : tr("%1 · %2").arg(machine.key(), root.value("name").toString());
            m_hostTargets.insert(key, {machine.value(), id});
            m_targets->addItem(label, key);
        }
    }
    for (auto it = m_environmentContainers.cbegin(); it != m_environmentContainers.cend(); ++it)
        m_targets->addItem(tr("%1 · Container").arg(it.value()), "local:" + it.key());
    for (auto it = m_remoteContainers.cbegin(); it != m_remoteContainers.cend(); ++it)
        m_targets->addItem(tr("%1 · %2").arg(it.value(), node), remoteContainerTarget(m_remoteContainerCatalog, it.key()));
    if (m_targets->findData(m_target) < 0)
        m_targets->addItem(tr("%1 · Unavailable").arg(m_targetLabel), m_target);
    m_targets->setCurrentIndex(m_targets->findData(m_target));
}

void TerminalWidget::selectTarget(const QString &target)
{
    if (target == m_target || target.startsWith("unavailable:"))
        return;

    m_targetLabel = m_targets->itemText(m_targets->findData(target));
    m_target = target;
    if (target.isEmpty()) {
        m_containerName.clear();
        setMachineContext(m_workspaceCatalog, m_workspaceRoot);
    } else if (m_hostTargets.contains(target)) {
        const auto host = m_hostTargets.value(target);
        m_containerName.clear();
        setMachineContext(host.first, host.second);
    } else if (target.startsWith("local:")) {
        const auto container = target.mid(6);
        m_containerName = m_environmentContainers.value(container);
        setContainerContext({}, container, QStringLiteral("/bin/sh"));
    } else {
        const auto container = target.section(QLatin1Char(':'), -1);
        m_containerName = m_remoteContainers.value(container);
        setContainerContext(m_remoteContainerCatalog, container, QStringLiteral("/bin/sh"));
    }
    refreshTargets();
}

bool TerminalWidget::explicitTargetAvailable() const
{
    if (m_target.isEmpty())
        return true;
    if (m_target.startsWith("host:"))
        return m_hostTargets.contains(m_target);
    if (m_target.startsWith("local:"))
        return m_environmentContainers.contains(m_containerId);
    return m_target == remoteContainerTarget(m_remoteContainerCatalog, m_containerId)
        && m_remoteContainers.contains(m_containerId) && m_remoteContainerCatalog.value("docker_exec").toBool()
        && !m_remoteContainerCatalog.value("terminal_epoch").toString().isEmpty();
}

void TerminalWidget::updateNewTargetTerminal()
{
    if (!m_target.isEmpty())
        findChild<QPushButton *>("terminalNew")->setEnabled(explicitTargetAvailable());
}

bool TerminalWidget::setTerminalContext(const QJsonObject &catalog, const QString &root,
                                       const QString &container, const QString &shell, QString *error)
{
    if (error)
        error->clear();
    if (m_machineCatalog == catalog && m_machineRoot == root)
        return true;
    m_machineCatalog = catalog;
    m_machineRoot = root;
    m_containerId = container;
    m_containerShell = shell;
    m_managedStarted = false;
    m_dashboardStarted = false;
    m_ownedSessionId.clear();
    selectSession({});
    bool allowed = catalog.isEmpty();
    if (!catalog.isEmpty() && !container.isEmpty())
        allowed = catalog.value("docker_exec").toBool() && catalog.value("docker_read").toBool();
    for (const auto &value : catalog.value("roots").toArray()) {
        const auto permission = value.toObject();
        if (container.isEmpty() && permission.value("id").toString() == root && permission.value("terminal").toBool())
            allowed = true;
    }
    auto *newTerminal = findChild<QPushButton *>("terminalNew");
    if (newTerminal)
        newTerminal->setEnabled(allowed);
    if (auto *shellChoices = findChild<QToolButton *>("terminalShellChoices"))
        shellChoices->setVisible(catalog.isEmpty() && container.isEmpty());
    refreshShells();
    refreshSessions();
    if (!allowed) {
        const QString message = tr("Remote terminal access is unavailable for this machine or root.");
        if (error)
            *error = message;
        showStatus(message);
        return false;
    }
    if (isVisible()) {
        if (m_mode == Mode::Dashboard)
            startDashboardSession();
        else
            startManagedSession();
    }
    return true;
}

void TerminalWidget::updateMachineCatalog(const QJsonObject &catalog)
{
    if (m_machineCatalog.isEmpty() || catalog.value("target") != m_machineCatalog.value("target"))
        return;
    // Only new explicit launches use a refreshed broker epoch. Existing session
    // IDs keep their original backend/reference and are never resumed or replayed.
    m_machineCatalog = catalog;
    bool allowed = !m_containerId.isEmpty() && catalog.value("docker_exec").toBool() && catalog.value("docker_read").toBool();
    for (const auto &value : catalog.value("roots").toArray()) {
        const auto root = value.toObject();
        if (m_containerId.isEmpty() && root.value("id").toString() == m_machineRoot && root.value("terminal").toBool())
            allowed = true;
    }
    if (auto *create = findChild<QPushButton *>("terminalNew"))
        create->setEnabled(allowed);
    refreshSessions();
}

void TerminalWidget::refreshSessions()
{
    const auto availableSessions = m_hub.sessionIds(m_machineCatalog, m_machineRoot);
    const auto allSessions = m_hub.sessionIds();
    m_ownedSessionIds.removeIf([&allSessions](const QString &id) { return !allSessions.contains(id); });
    QSignalBlocker blocker(m_sessions);
    m_sessions->clear();
    m_sessions->addItem(tr("Choose a terminal…"), QString());
    for (const QString &id : m_hub.sessionIds(m_machineCatalog, m_machineRoot)) {
        QString label = m_hub.title(id);
        if (!m_hub.running(id))
            label += tr(" (stopped)");
        m_sessions->addItem(label, id);
    }
    if (!m_sessionId.isEmpty() && !availableSessions.contains(m_sessionId)) {
        QString next;
        if (m_mode == Mode::Managed && !availableSessions.isEmpty())
            next = availableSessions.at(qBound(0, m_tabs->currentIndex(), int(availableSessions.size()) - 1));
        selectSession(next);
        return;
    }
    m_sessions->setCurrentIndex(qMax(0, m_sessions->findData(m_sessionId)));
    const QSignalBlocker tabsBlocker(m_tabs);
    while (m_tabs->count() > 0) {
        auto *closeSlot = m_tabs->tabButton(0, QTabBar::RightSide);
        m_tabs->removeTab(0);
        if (closeSlot)
            closeSlot->deleteLater();
    }
    for (const auto &id : availableSessions) {
        const QString title = m_hub.title(id);
        const int directorySeparator = title.lastIndexOf(QStringLiteral(" · "));
        QString label = directorySeparator < 0 ? title : title.left(directorySeparator);
        if (!m_hub.running(id))
            label += tr(" · Exited");
        const int index = m_tabs->addTab(label);
        m_tabs->setTabData(index, id);
        m_tabs->setTabToolTip(index, title);
        auto *closeSlot = new QWidget(m_tabs);
        auto *closeLayout = new QHBoxLayout(closeSlot);
        closeLayout->setContentsMargins(0, 0, 8, 0);
        closeLayout->setSpacing(0);
        auto *close = new QToolButton(closeSlot);
        close->setObjectName("terminalTabClose");
        close->setFixedSize(20, 20);
        close->setToolTip(tr("Stop and close %1 for all views").arg(label));
        close->setAccessibleName(close->toolTip());
        closeLayout->addWidget(close);
        closeSlot->setFixedSize(28, 24);
        m_tabs->setTabButton(index, QTabBar::RightSide, closeSlot);
        connect(close, &QToolButton::clicked, this, [this, id] {
            if (m_hubGuard)
                m_hub.closeSession(id);
        });
        if (id == m_sessionId)
            m_tabs->setCurrentIndex(index);
    }
    m_close->setEnabled(!m_sessionId.isEmpty());
    if (m_restart)
        m_restart->setEnabled(!m_hub.running(m_ownedSessionId));
    if (m_mode == Mode::Dashboard)
        refreshShells();
    if (!m_sessionId.isEmpty()) {
        const QString error = m_hub.error(m_sessionId);
        QString message = error;
        if (message.isEmpty()) {
            if (m_hub.connecting(m_sessionId)) {
                message = tr("Connecting to remote terminal…");
            } else if (m_hub.running(m_sessionId)) {
                message = tr("Interactive shell · Ctrl+Shift+C/V to copy/paste");
            } else if (m_mode == Mode::Dashboard) {
                message = tr("Shell exited. Choose Start terminal from the context menu to open another shell.");
            } else {
                message = tr("Shell exited. Open a new terminal to continue.");
            }
        }
        showStatus(message, !error.isEmpty() || !m_hub.running(m_sessionId));
    }
}

void TerminalWidget::selectSession(const QString &id)
{
    if (m_sessionId == id) {
        if (m_mode == Mode::Managed && m_view && isVisible())
            m_view->setFocus(Qt::OtherFocusReason);
        return;
    }
    m_sessionId = id;
    m_cursor = 0;
    ++m_generation;
    m_inFlight = false;
    m_reset = true;
    if (!id.isEmpty())
        ensureRenderer();
    m_empty->setVisible(id.isEmpty());
    if (m_view)
        m_view->setVisible(!id.isEmpty());
    if (m_view && !id.isEmpty() && isVisible() && m_mode == Mode::Managed)
        m_view->setFocus(Qt::OtherFocusReason);
    if (id.isEmpty()) {
        if (m_mode == Mode::Dashboard)
            showStatus(tr("Terminal closed. Choose Start terminal from the context menu to open another shell."));
        else
            m_status->hide();
    }
    refreshSessions();
    scheduleOutput();
}

void TerminalWidget::ensureRenderer()
{
    if (m_view)
        return;
    initializeTerminalResources();
    m_view = new QWebEngineView(this);
    m_view->setObjectName("terminalDisplay");
    m_view->setContextMenuPolicy(Qt::NoContextMenu);
    if (m_mode == Mode::Dashboard) {
        m_view->setContextMenuPolicy(Qt::ActionsContextMenu);
        m_view->addActions(actions());
    }
    auto *profile = new QWebEngineProfile(this);
    auto *requests = new TerminalRequests(profile);
    profile->setUrlRequestInterceptor(requests);
    auto *page = new TerminalPage(profile, m_view);
    m_view->setPage(page);
    applyAppearance();
    auto *settings = page->settings();
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessFileUrls, false);
    settings->setAttribute(QWebEngineSettings::LocalContentCanAccessRemoteUrls, false);
    settings->setAttribute(QWebEngineSettings::JavascriptCanAccessClipboard, false);
    settings->setAttribute(QWebEngineSettings::JavascriptCanOpenWindows, false);
    auto *channel = new QWebChannel(page);
    auto *bridge = new TerminalBridge(channel);
    connect(bridge, &TerminalBridge::ready, this, &TerminalWidget::rendererReady);
    connect(bridge, &TerminalBridge::input, this, &TerminalWidget::terminalInput);
    connect(bridge, &TerminalBridge::sizeChanged, this, &TerminalWidget::terminalSize);
    connect(bridge, &TerminalBridge::acknowledged, this, &TerminalWidget::outputRendered);
    connect(this, &TerminalWidget::renderOutput, bridge, &TerminalBridge::renderOutput);
    connect(this, &TerminalWidget::appearanceChanged, bridge, &TerminalBridge::appearanceChanged);
    channel->registerObject("terminalBridge", bridge);
    page->setWebChannel(channel);
    auto *column = qobject_cast<QVBoxLayout *>(layout());
    m_empty->hide();
    column->addWidget(m_view, 1);
    connect(m_view, &QWebEngineView::renderProcessTerminated, this, [this]() {
        m_ready = false;
        m_inFlight = false;
        showStatus(tr("Terminal display stopped. The shell is still available from another view."));
    });
    auto *copy = new QShortcut(QKeySequence(Qt::CTRL | Qt::SHIFT | Qt::Key_C), m_view);
    copy->setContext(Qt::WidgetWithChildrenShortcut);
    connect(copy, &QShortcut::activated, this, [this]() {
        QPointer<TerminalWidget> guard(this);
        m_view->page()->runJavaScript("window.dockpipeTerminal.getSelection()", [guard](const QVariant &text) {
            if (guard && !text.toString().isEmpty())
                QApplication::clipboard()->setText(text.toString());
        });
    });
    auto *paste = new QShortcut(QKeySequence(Qt::CTRL | Qt::SHIFT | Qt::Key_V), m_view);
    paste->setContext(Qt::WidgetWithChildrenShortcut);
    connect(paste, &QShortcut::activated, this, [this]() {
        const QString text = QApplication::clipboard()->text();
        if (text.toUtf8().size() > 64 * 1024) {
            showStatus(tr("Paste is limited to 64 KiB."));
            return;
        }
        const QByteArray argument = QJsonDocument(QJsonArray{text}).toJson(QJsonDocument::Compact);
        m_view->page()->runJavaScript("window.dockpipeTerminal.paste(" + QString::fromUtf8(argument) + "[0])");
    });
    m_view->load(QUrl("qrc:/terminal/index.html"));
}

void TerminalWidget::rendererReady()
{
    m_ready = true;
    setProperty("terminalRendererReady", true);
    m_inFlight = false;
    m_reset = true;
    applyAppearance();
    scheduleOutput();
}

void TerminalWidget::terminalInput(const QString &text, int generation)
{
    if (generation != m_generation || m_sessionId.isEmpty() || !explicitTargetAvailable())
        return;
    if (text.toUtf8().size() > 64 * 1024 || !m_hub.sendBytes(m_sessionId, text.toUtf8()))
        showStatus(tr("Input was not accepted. Check that the selected shell is running."));
}

void TerminalWidget::terminalSize(int columns, int rows, int generation)
{
    if (isVisible() && generation == m_generation)
        m_hub.resize(m_sessionId, qBound(2, columns, 500), qBound(2, rows, 200));
}

void TerminalWidget::outputRendered(int generation)
{
    if (generation != m_generation)
        return;
    m_inFlight = false;
    scheduleOutput();
}

void TerminalWidget::scheduleOutput()
{
    if (isVisible() && m_ready && !m_inFlight && !m_sessionId.isEmpty() && !m_flushTimer->isActive())
        m_flushTimer->start();
}

void TerminalWidget::flushOutput()
{
    if (!isVisible() || !m_ready || m_inFlight || m_sessionId.isEmpty())
        return;
    bool truncated = false;
    const QByteArray bytes = m_hub.readOutput(m_sessionId, m_cursor, truncated);
    if (bytes.isEmpty() && !m_reset)
        return;
    if (truncated)
        showStatus(tr("Output exceeded the 256 KiB replay window; older output was omitted."));
    m_inFlight = true;
    emit renderOutput(QString::fromLatin1(bytes.toBase64()), m_reset || truncated, m_generation);
    m_reset = false;
}

void TerminalWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_mode == Mode::Dashboard && !m_dashboardStarted)
        startDashboardSession();
    else if (m_mode == Mode::Managed)
        startManagedSession();
    if (m_mode == Mode::Managed && m_view && !m_sessionId.isEmpty())
        m_view->setFocus(Qt::OtherFocusReason);
    applyAppearance();
    scheduleOutput();
}

void TerminalWidget::hideEvent(QHideEvent *event)
{
    m_flushTimer->stop();
    QWidget::hideEvent(event);
}

void TerminalWidget::applyAppearance()
{
    const TerminalSettings settings = TerminalPreferences::instance()->settings();
    const int opacity = TerminalPreferences::renderingOpacity(settings);
    m_desktopBackground = settings.backgroundTarget == "desktop" && opacity < 100;
    setProperty("canvasPanelTransparent", settings.backgroundTarget == "canvas" && opacity < 100);
    const bool needsCompositorRestart = opacity != settings.opacity;
    m_transparencyHint->setText(needsCompositorRestart
        ? tr("Restart DockPipe to enable terminal transparency with compatible browser rendering.")
        : tr("Restart DockPipe to enable desktop transparency. Your desktop must support compositing."));
    m_transparencyHint->setVisible(needsCompositorRestart
        || (m_desktopBackground && !window()->testAttribute(Qt::WA_TranslucentBackground)));
    update();
    const QJsonObject theme = TerminalPreferences::theme(settings.theme);
    if (m_view) {
        // Qt 6.2 chooses the WebEngine alpha surface when the renderer starts.
        // Keep it available throughout a software-rendered session; the terminal
        // DOM still paints an opaque background when the requested opacity is 100%.
        const bool softwareRendering = qEnvironmentVariable("QT_QUICK_BACKEND") == "software";
        m_view->page()->setBackgroundColor(softwareRendering
            ? QColor(Qt::transparent) : QColor(theme.value("background").toString()));
    }
    if (!m_ready)
        return;
    QJsonObject appearance;
    appearance.insert("theme", theme);
    appearance.insert("fontFamily", settings.fontFamily);
    appearance.insert("fontSize", settings.fontSize);
    appearance.insert("cursorStyle", settings.cursorStyle);
    appearance.insert("cursorBlink", settings.cursorBlink);
    appearance.insert("scrollback", settings.scrollback);
    appearance.insert("opacity", opacity);
    appearance.insert("scanlines", settings.scanlines);
    appearance.insert("glow", settings.glow);
    appearance.insert("glowStrength", settings.glowStrength);
    appearance.insert("glowSpread", settings.glowSpread);
    emit appearanceChanged(appearance);
}

void TerminalWidget::paintEvent(QPaintEvent *event)
{
    QWidget::paintEvent(event);
    if (!m_desktopBackground || !m_view || !m_view->isVisible()
        || !window()->testAttribute(Qt::WA_TranslucentBackground))
        return;
    // Clear only the terminal viewport in the shared backing store. The browser
    // paints its translucent background and fully opaque text over this region.
    // Source composition removes ancestor panel fills without fading controls.
    QPainter painter(this);
    painter.setCompositionMode(QPainter::CompositionMode_Source);
    painter.fillRect(m_view->geometry(), Qt::transparent);
}
