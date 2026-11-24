#include "DashboardWidgetFactory.h"
#include <QMetaMethod>

#include "DashboardLogWidget.h"
#include "DashboardFilesWidget.h"
#include <QDesktopServices>
#include <QMessageBox>
#include <QUrl>
#include "DockpipeChoices.h"
#include "TerminalHub.h"
#include "TerminalWidget.h"
#include "TerminalPreferences.h"
#include "SendInputWidget.h"
#include "WorkspaceMachineMenu.h"
#include "RemoteFeatureClient.h"
#include "RemoteGitContext.h"
#include "RemoteDesktopWidget.h"
#include "RemoteBrowserWidget.h"
#include "RemoteAccessMonitor.h"

#include <QEvent>
#include <QJsonDocument>
#include <QPushButton>
#include <QLabel>
#include <QStackedWidget>
#include <QTimer>
#include <QVBoxLayout>
#include <QWidget>

namespace {
class DashboardDesktopState final : public QObject {
public:
    explicit DashboardDesktopState(QObject *parent) : QObject(parent) {}
    QVector<ResolverMeta> resolvers;
    QPointer<RemoteAccessMonitor> access;
    QJsonArray machines;
    QVector<QPointer<RemoteDesktopWidget>> widgets;
};

DashboardDesktopState *desktopState(DashboardWidgetFactory *factory, bool create = true)
{
    for (auto *child : factory->children()) {
        if (auto *state = dynamic_cast<DashboardDesktopState *>(child))
            return state;
    }
    return create ? new DashboardDesktopState(factory) : nullptr;
}
}

DashboardWidgetFactory::DashboardWidgetFactory(SessionManager &sessions, ContextStore &store, QObject *parent)
    : DashboardWidgetFactory(sessions, store, nullptr, parent)
{
}

DashboardWidgetFactory::DashboardWidgetFactory(SessionManager &sessions, ContextStore &store, TerminalHub *terminals, QObject *parent)
    : QObject(parent), m_sessions(sessions), m_store(store), m_terminals(terminals ? terminals : new TerminalHub(this)), m_snapshotCommand(this),
      m_containerCommand(this), m_machineCommand(this), m_gitCommand(this), m_timer(new QTimer(this))
{
    // One reader per source serves all copies of that widget. Hidden dashboard
    // panels retain state but do not spawn background observation processes.
    m_timer->setObjectName("dashboardSnapshotPoll");
    m_timer->setInterval(4000);
    connect(m_timer, &QTimer::timeout, this, [this]() {
        refreshSnapshot();
        refreshContainers();
    });
    if (!terminals)
        m_timer->start();
    auto *slowTimer = new QTimer(this);
    slowTimer->setObjectName("dashboardInventoryPoll");
    slowTimer->setInterval(30000);
    connect(slowTimer, &QTimer::timeout, this, [this]() {
        refreshGit();
        refreshMachines();
    });
    if (!terminals)
        slowTimer->start();
    connect(this, &DashboardWidgetFactory::machinesChanged, this,
        [this](const QJsonArray &machines, const QString &error) {
        if (error.isEmpty())
            setRemoteDesktopMachines(machines);
    });
}

QVector<DashboardWidgetDescriptor> DashboardWidgetFactory::catalog()
{
    return {
        {"system", tr("System monitor"), tr("Live CPU, memory, disk and network measurements for this computer.")},
        {"workflows", tr("Workflow launcher"), tr("Find and launch workflows using the normal DockPipe configuration flow.")},
        {"executions", tr("Recent workflow runs"), tr("The latest 12 managed operations and their current state.")},
        {"logs", tr("Run logs"), tr("A live, bounded tail of workflows launched in this desktop session.")},
        {"containers", tr("Containers"), tr("Local Docker container names, states and images; management stays in Containers.")},
        {"files", tr("File manager"), tr("Browse project files in Canvas or the filesystem from Home in Tiling. Change either scope in dashboard settings.")},
        {"git", tr("Git changes"), tr("Current branch and a compact list of changes from the DockPipe Git service.")},
        {"machines", tr("Machines"), tr("Managed machine access states from the existing remote service.")},
        {"remote-desktop", tr("Remote Desktop"), tr("Connect to a paired computer using native Remote Desktop.")},
        {"browser", tr("Browser"), tr("Browse locally or through a paired machine with a separate browsing context.")},
        {"terminal", tr("Terminal"), tr("A shell that starts when its panel is first shown. Closing the panel stops its shell.")},
        {"send-input", tr("Send input"), tr("Send text to an explicitly selected terminal session.")}
    };
}

QWidget *DashboardWidgetFactory::wrap(const QString &kind, const QString &pageKey, QWidget *content, QWidget *parent)
{
    auto *widget = new QWidget(parent);
    widget->setObjectName("dashboardWidget_" + kind);
    widget->setProperty("dashboardKind", kind);
    if (kind == "terminal") {
        const auto updateBackground = [widget]() {
            const TerminalSettings settings = TerminalPreferences::instance()->settings();
            widget->setProperty("canvasPanelTransparent", settings.backgroundTarget == "canvas" && TerminalPreferences::renderingOpacity(settings) < 100);
        };
        connect(TerminalPreferences::instance(), &TerminalPreferences::changed, widget, updateBackground);
        updateBackground();
    }
    widget->setMinimumSize(260, 180);
    auto *layout = new QVBoxLayout(widget);
    layout->setContentsMargins(10, 10, 10, 8);
    layout->setSpacing(8);
    if (kind == "terminal") {
        layout->setContentsMargins(0, 0, 0, 0);
        layout->setSpacing(0);
    }
    layout->addWidget(content, 1);
    auto *footerWidget = new QWidget;
    footerWidget->setObjectName("dashboardWidgetFooter");
    auto *footer = new QHBoxLayout(footerWidget);
    footer->setContentsMargins(0, 0, 0, 0);
    if (m_upstream) {
        widget->setProperty("dashboardMachineFactory", QVariant::fromValue<QObject *>(this));
        auto *machine = new WorkspaceMachineMenu(footerWidget);
        machine->setObjectName(QStringLiteral("dashboardWidgetMachine"));
        machine->setCatalog(m_machineCatalog);
        machine->setAccessUnlocked(true);
        machine->setCurrent(machineTarget(), m_sharedRoot);
        machine->setWidgetMode(true, m_followMain);
        footer->addWidget(machine);
        connect(machine, &WorkspaceMachineMenu::contextRequested, widget,
            [this, widget](const QJsonObject &target, const QString &root) {
            QString error;
            const QJsonObject context{{"mode", target.isEmpty() ? "local" : "remote"},
                {"target", target}, {"rootId", root}};
            if (!setWidgetMachineContext(widget, context, &error))
                QMessageBox::information(widget, tr("Machine"), error);
        });
        connect(machine, &WorkspaceMachineMenu::followMainRequested, widget, [this, widget] {
            QString error;
            if (!setWidgetMachineContext(widget, {{"mode", "follow"}}, &error))
                QMessageBox::information(widget, tr("Machine"), error);
        });
        connect(machine, &WorkspaceMachineMenu::refreshRequested, this, &DashboardWidgetFactory::machineCatalogRefreshRequested);
    }
    if (kind != "browser" && kind != "files" && kind != "terminal" && kind != "send-input" && kind != "remote-desktop") {
        auto *refreshButton = new QPushButton(tr("Refresh"));
        refreshButton->setObjectName("dashboardWidgetRefresh");
        connect(refreshButton, &QPushButton::clicked, this, &DashboardWidgetFactory::refresh);
        footer->addWidget(refreshButton);
    }
    footer->addStretch();
    auto *open = new QPushButton(tr("Open full view"));
    open->setObjectName("dashboardOpenFullView");
    if (auto *files = qobject_cast<DashboardFilesWidget *>(content)) {
        open->setText(tr("Open folder"));
        open->setToolTip(tr("Open this folder in your system file manager"));
        connect(open, &QPushButton::clicked, files, [files]() {
            const QString directory = files->currentDirectory();
            if (files->machineTarget().isEmpty() && !directory.isEmpty() && !QDesktopServices::openUrl(QUrl::fromLocalFile(directory)))
                QMessageBox::warning(files, tr("Open folder"), tr("The system could not open this folder."));
        });
    } else {
        connect(open, &QPushButton::clicked, this, [this, pageKey]() { emit openFullView(pageKey); });
    }
    if (kind == "files" || kind == "git" || kind == "workflows") {
        auto *canvas = new QPushButton(tr("Full view on canvas"));
        canvas->setObjectName("dashboardOpenCanvasView");
        connect(canvas, &QPushButton::clicked, content, [this, content, kind]() {
            QString root = m_workdir;
            if (auto *files = qobject_cast<DashboardFilesWidget *>(content)) {
                if (files->scope() != "project")
                    root = files->currentDirectory();
            }
            emit canvasViewRequested(content, kind + "-full", root);
        });
        footer->addWidget(canvas);
        if (kind != "files")
            open->hide();
    }
    footer->addWidget(open);
    layout->addWidget(footerWidget);
    if (kind == "terminal" || kind == "remote-desktop")
        open->hide();
    if (kind == "remote-desktop" || kind == "browser")
        footerWidget->hide();
    widget->installEventFilter(this);
    m_widgets.append(widget);
    applyMachineContext(widget);
    connect(widget, &QObject::destroyed, this, [this]() {
        m_widgets.removeIf([](const QPointer<QWidget> &entry) { return entry.isNull(); });
    });
    return widget;
}

QWidget *DashboardWidgetFactory::create(const QString &kind, QWidget *parent, const QString &fileScope)
{
    return create(kind, parent, fileScope, {});
}

QWidget *DashboardWidgetFactory::create(const QString &kind, QWidget *parent, const QString &fileScope,
                                      const QJsonObject &machineContext, const QString &browserContextId)
{
    const QStringList scopedKinds{"files", "git", "containers", "terminal", "send-input", "workflows"};
    if (!m_upstream && scopedKinds.contains(kind)) {
        auto *factory = new DashboardWidgetFactory(m_sessions, m_store, m_terminals, this);
        factory->m_upstream = this;
        factory->m_workdir = m_workdir;
        factory->m_workflows = m_workflows;
        factory->m_machineCatalog = m_machineCatalog;
        const QString mode = machineContext.value("mode").toString("follow");
        factory->m_followMain = mode == "follow";
        if (mode == "remote" || (mode != "follow" && mode != "local")) {
            factory->m_sharedMachine = {{"target", machineContext.value("target").toObject()}};
            factory->m_sharedRoot = machineContext.value("rootId").toString();
        } else if (mode == "follow") {
            factory->m_sharedMachine = m_sharedMachine;
            factory->m_sharedRoot = m_sharedRoot;
        }
        RemoteGitContext::bind(&factory->m_gitCommand, !factory->m_sharedMachine.isEmpty(),
            factory->machineTarget(), factory->m_sharedRoot);
        auto *widget = factory->create(kind, parent, fileScope);
        if (!widget) {
            delete factory;
            return nullptr;
        }
        factory->setParent(widget);
        m_scopedFactories.append(factory);
        connect(factory, &QObject::destroyed, this, [this] {
            m_scopedFactories.removeIf([](const auto &entry) { return entry.isNull(); });
            if (!hasVisibleKind({"git"}))
                m_gitCommand.cancel();
            if (!hasVisibleKind({"containers"})) {
                m_containerCommand.cancel();
                if (auto *call = findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
                    delete call;
            }
        });
        connect(this, &DashboardWidgetFactory::gitChanged, factory,
            [this, factory](const QJsonObject &snapshot, const QString &error) {
            if (factory->m_followMain && factory->machineTarget() == machineTarget() && factory->m_sharedRoot == m_sharedRoot)
                emit factory->gitChanged(snapshot, error);
        });
        connect(this, &DashboardWidgetFactory::containersChanged, factory,
            [this, factory](const QJsonArray &snapshot, const QString &error) {
            if (factory->m_followMain && factory->machineTarget() == machineTarget() && factory->m_sharedRoot == m_sharedRoot)
                emit factory->containersChanged(snapshot, error);
        });
        connect(this, &DashboardWidgetFactory::workdirChanged, factory, &DashboardWidgetFactory::setWorkdir);
        connect(this, &DashboardWidgetFactory::workflowsChanged, factory, [this, factory] {
            factory->setWorkflows(m_workflows);
        });
        connect(factory, &DashboardWidgetFactory::launchWorkflowOnMachine, this,
            [this](const QString &workflow, const QJsonObject &target, const QString &root) {
            if (isSignalConnected(QMetaMethod::fromSignal(&DashboardWidgetFactory::launchWorkflowOnMachine)))
                emit launchWorkflowOnMachine(workflow, target, root);
            else if (target.isEmpty())
                emit launchWorkflow(workflow);
        });
        connect(factory, &DashboardWidgetFactory::cloneWorkflow, this, &DashboardWidgetFactory::cloneWorkflow);
        connect(factory, &DashboardWidgetFactory::filePreviewRequested, this, &DashboardWidgetFactory::filePreviewRequested);
        connect(factory, &DashboardWidgetFactory::gitPreviewRequested, this, &DashboardWidgetFactory::gitPreviewRequested);
        connect(factory, &DashboardWidgetFactory::canvasViewRequested, this, &DashboardWidgetFactory::canvasViewRequested);
        connect(factory, &DashboardWidgetFactory::openFullView, this, &DashboardWidgetFactory::openFullView);
        connect(factory, &DashboardWidgetFactory::widgetMachineContextChanged, this, &DashboardWidgetFactory::widgetMachineContextChanged);
        connect(factory, &DashboardWidgetFactory::machineCatalogRefreshRequested, this, &DashboardWidgetFactory::machineCatalogRefreshRequested);
        return widget;
    }
    QWidget *content = nullptr;
    QString page = kind;
    if (kind == "system") {
        content = createSystem(nullptr);
    } else if (kind == "executions") {
        content = createExecutions(nullptr);
    } else if (kind == "workflows") {
        content = createWorkflows(nullptr);
    } else if (kind == "containers") {
        content = createContainers(nullptr);
    } else if (kind == "logs") {
        auto *logs = new DashboardLogWidget(m_sessions, m_store);
        logs->setWorkdir(m_workdir);
        connect(this, &DashboardWidgetFactory::workdirChanged, logs, &DashboardLogWidget::setWorkdir);
        content = logs;
        page = "logs";
    } else if (kind == "files") {
        content = createFiles(nullptr, fileScope);
    } else if (kind == "git") {
        content = createGit(nullptr);
    } else if (kind == "machines") {
        content = createMachines(nullptr);
    } else if (kind == "browser") {
        content = createBrowser(nullptr, browserContextId);
        auto *browser = static_cast<RemoteBrowserWidget *>(content);
        if (machineContext.contains("mode"))
            browser->setMachineContext(machineContext.value("target").toObject());
        connect(browser, &RemoteBrowserWidget::machineContextChanged, this,
            [this, browser](const QJsonObject &, const QString &) {
                if (auto *widget = browser->parentWidget())
                    emit widgetMachineContextChanged(widget);
            });
    } else if (kind == "terminal") {
        auto *terminal = new TerminalWidget(*m_terminals, m_workdir, nullptr, TerminalWidget::Mode::Dashboard);
        terminal->setMachineContext(m_sharedMachine, m_sharedRoot);
        connect(this, &DashboardWidgetFactory::workdirChanged, terminal, &TerminalWidget::setWorkdir);
        connect(terminal, &TerminalWidget::openFullViewRequested, this, [this] { emit openFullView("terminal"); });
        content = terminal;
    } else if (kind == "remote-desktop") {
        auto *state = desktopState(this);
        auto *desktop = new RemoteDesktopWidget;
        desktop->setCompactPresentation(true);
        desktop->setResolvers(state->resolvers);
        desktop->setAccessMonitor(state->access);
        desktop->setPairedMachines(state->machines);
        state->widgets.append(desktop);
        connect(desktop, &QObject::destroyed, state, [state] {
            state->widgets.removeIf([](const auto &widget) { return widget.isNull(); });
        });
        connect(desktop, &RemoteDesktopWidget::installProviderRequested,
            this, &DashboardWidgetFactory::remoteDesktopInstallProviderRequested);
        connect(desktop, &RemoteDesktopWidget::providerSetupRequested,
            this, &DashboardWidgetFactory::remoteDesktopProviderSetupRequested);
        content = desktop;
    } else if (kind == "send-input") {
        auto *input = new SendInputWidget(*m_terminals);
        input->setMachineContext(m_sharedMachine, m_sharedRoot);
        content = input;
        page = "terminal";
    } else {
        return nullptr;
    }
    return wrap(kind, page, content, parent);
}

void DashboardWidgetFactory::setBrowserAccessMonitor(RemoteAccessMonitor *monitor)
{
    if (m_browserAccess == monitor)
        return;
    if (m_browserAccess)
        disconnect(m_browserAccess, nullptr, this, nullptr);
    m_browserAccess = monitor;
    if (monitor) {
        connect(monitor, &RemoteAccessMonitor::changed, this, &DashboardWidgetFactory::updateBrowserMachines);
        connect(monitor, &QObject::destroyed, this, &DashboardWidgetFactory::updateBrowserMachines);
    }
    updateBrowserMachines();
}

void DashboardWidgetFactory::updateBrowserMachines()
{
    const bool unlocked = m_browserAccess && m_browserAccess->state() == RemoteAccessMonitor::State::Unlocked;
    for (const auto &widget : m_widgets) {
        if (!widget)
            continue;
        if (auto *browser = widget->findChild<RemoteBrowserWidget *>())
            browser->setMachineCatalog(m_machineCatalog, unlocked);
    }
}

void DashboardWidgetFactory::setRemoteDesktopResolvers(const QVector<ResolverMeta> &resolvers)
{
    auto *state = desktopState(this);
    state->resolvers = resolvers;
    for (const auto &widget : state->widgets) {
        if (widget)
            widget->setResolvers(resolvers);
    }
}

void DashboardWidgetFactory::setRemoteDesktopAccessMonitor(RemoteAccessMonitor *monitor)
{
    auto *state = desktopState(this);
    state->access = monitor;
    for (const auto &widget : state->widgets) {
        if (widget)
            widget->setAccessMonitor(monitor);
    }
}

void DashboardWidgetFactory::setRemoteDesktopMachines(const QJsonArray &machines)
{
    auto *state = desktopState(this);
    state->machines = machines.size() <= 64 ? machines : QJsonArray{};
    for (const auto &widget : state->widgets) {
        if (widget)
            widget->setPairedMachines(state->machines);
    }
}

void DashboardWidgetFactory::refreshRemoteDesktopMetadata()
{
    // This focused path is also used for pairing changes. It must not refresh
    // unrelated Git/container/workflow panels or acquire a desktop surface.
    if (auto *state = desktopState(this, false)) {
        for (const auto &desktop : state->widgets) {
            if (desktop && desktop->isVisible())
                desktop->refresh();
        }
    }
}

void DashboardWidgetFactory::setWorkdir(const QString &workdir)
{
    if (m_workdir == workdir)
        return;
    m_workdir = workdir;
    m_packageDisplays.clear();
    emit packageDisplaysChanged();
    m_workflows.clear();
    emit workflowsChanged();
    ++m_generation;
    m_snapshotCommand.cancel();
    m_gitCommand.cancel();
    m_machineCommand.cancel();
    emit workdirChanged(workdir);
    emit snapshotChanged({}, tr("Waiting for this project's measurements…"));
    emit gitChanged({}, tr("Waiting for this project's Git status…"));
    QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
}

void DashboardWidgetFactory::setWorkflows(const QVector<WorkflowMeta> &workflows)
{
    m_workflows = workflows;
    emit workflowsChanged();
}

bool DashboardWidgetFactory::hasVisibleKind(const QStringList &kinds) const
{
    for (const auto &factory : m_scopedFactories) {
        if (factory && factory->m_followMain && factory->hasVisibleKind(kinds))
            return true;
    }
    for (const auto &widget : m_widgets) {
        if (widget && widget->isVisible() && kinds.contains(widget->property("dashboardKind").toString()))
            return true;
    }
    return false;
}

bool DashboardWidgetFactory::eventFilter(QObject *watched, QEvent *event)
{
    if (event->type() == QEvent::Show)
        QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
    if (event->type() == QEvent::Hide) {
        QTimer::singleShot(0, this, [this]() {
            if (!hasVisibleKind({"system", "executions"}))
                m_snapshotCommand.cancel();
            if (!hasVisibleKind({"containers"})) {
                m_containerCommand.cancel();
                if (auto *call = findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
                    delete call;
            }
            if (!hasVisibleKind({"git"}))
                m_gitCommand.cancel();
            if (!hasVisibleKind({"machines"}))
                m_machineCommand.cancel();
            if (m_upstream && m_followMain) {
                if (!m_upstream->hasVisibleKind({"git"}))
                    m_upstream->m_gitCommand.cancel();
                if (!m_upstream->hasVisibleKind({"containers"})) {
                    m_upstream->m_containerCommand.cancel();
                    if (auto *call = m_upstream->findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
                        delete call;
                }
            }
        });
    }
    return QObject::eventFilter(watched, event);
}

void DashboardWidgetFactory::refresh()
{
    refreshSnapshot();
    refreshContainers();
    refreshGit();
    refreshMachines();
    emit workflowsChanged();
    refreshRemoteDesktopMetadata();
    for (const auto &widget : m_widgets) {
        if (widget && widget->isVisible()) {
            if (auto *logs = widget->findChild<DashboardLogWidget *>())
                logs->refresh();
        }
    }
}

void DashboardWidgetFactory::refreshSnapshot()
{
    if (m_snapshotCommand.busy() || !hasVisibleKind({"system", "executions"}))
        return;
    QStringList args{"runs", "observe", "--json", "--resources"};
    if (m_workdir.isEmpty())
        args << "--global";
    else
        args << "--workdir" << m_workdir;
    const quint64 generation = m_generation;
    m_snapshotCommand.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), args,
        [this, generation](bool ok, const QByteArray &bytes) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            const auto document = QJsonDocument::fromJson(bytes);
            const auto snapshot = document.object();
            if (!ok || snapshot.value("schema").toString() != "dockpipe.execution_snapshot.v1"
                || !snapshot.value("executions").isArray() || snapshot.value("executions").toArray().size() > 256) {
                emit snapshotChanged({}, tr("Monitoring unavailable. Check the CLI and machine permissions."));
                return;
            }
            emit snapshotChanged(snapshot, {});
        }, 5000);
}

void DashboardWidgetFactory::refreshContainers()
{
    if (m_upstream && m_followMain) {
        m_upstream->refreshContainers();
        return;
    }
    for (const auto &factory : m_scopedFactories) {
        if (factory && !factory->m_followMain)
            factory->refreshContainers();
    }
    if (m_containerCommand.busy() || !hasVisibleKind({"containers"}))
        return;
    if (!m_sharedMachine.isEmpty()) {
        if (findChild<QObject *>(QStringLiteral("dashboardRemoteContainerCall")))
            return;
        auto *call = new QObject(this);
        call->setObjectName(QStringLiteral("dashboardRemoteContainerCall"));
        const quint64 generation = m_generation;
        const auto intent = RemoteFeatureClient::prepareIntent(machineTarget(), {}, "containers.snapshot", {});
        RemoteFeatureClient::invoke(call, intent, [this, call, generation](const auto &result) {
            call->deleteLater();
            if (generation != m_generation)
                return;
            const auto snapshot = result.value.toObject();
            const auto containers = snapshot.value("containers").toArray();
            if (result.outcome != RemoteFeatureClient::Outcome::Success || !snapshot.value("available").toBool()
                || !snapshot.value("containers").isArray() || containers.size() > 256) {
                emit containersChanged({}, result.error.isEmpty()
                    ? tr("Docker inventory unavailable. Check Docker access in Containers.") : result.error);
                return;
            }
            emit containersChanged(containers, {});
        });
        return;
    }
    // This is the same read-only Docker inventory contract as Containers.
    m_containerCommand.run("docker", {"container", "ls", "--all", "--format", "{{json .}}"},
        [this](bool ok, const QByteArray &bytes) {
            if (!ok) {
                emit containersChanged({}, tr("Docker inventory unavailable. Check Docker access in Containers."));
                return;
            }
            QJsonArray containers;
            // Display only a compact window into inventory, with an explicit
            // count. The command transport bounds the complete response bytes.
            for (const auto &line : bytes.split('\n')) {
                if (line.trimmed().isEmpty())
                    continue;
                const auto document = QJsonDocument::fromJson(line);
                if (!document.isObject()) {
                    emit containersChanged({}, tr("Docker returned an invalid inventory."));
                    return;
                }
                if (containers.size() >= 4096) {
                    emit containersChanged({}, tr("Docker inventory exceeds the dashboard limit of 4096 containers. Use Containers for the full inventory."));
                    return;
                }
                containers.append(document.object());
            }
            emit containersChanged(containers, {});
        }, 5000);
}

void DashboardWidgetFactory::refreshGit()
{
    if (m_upstream && m_followMain) {
        m_upstream->refreshGit();
        return;
    }
    for (const auto &factory : m_scopedFactories) {
        if (factory && !factory->m_followMain)
            factory->refreshGit();
    }
    if (m_gitCommand.busy() || !hasVisibleKind({"git"}))
        return;
    if (m_workdir.isEmpty() && m_sharedMachine.isEmpty()) {
        emit gitChanged({}, tr("Open a project to see Git changes."));
        return;
    }
    const quint64 generation = m_generation;
    m_gitCommand.run(m_sharedMachine.isEmpty() ? m_workdir : QStringLiteral("/"), {{"action", "status"}},
        [this, generation](bool ok, const QJsonObject &snapshot, const QString &) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            emit gitChanged(ok ? snapshot : QJsonObject{}, ok ? QString{} : tr("Git status unavailable. Open Git for details."));
        });
}

void DashboardWidgetFactory::refreshMachines()
{
    if (m_machineCommand.busy() || !hasVisibleKind({"machines"}))
        return;
    const quint64 generation = m_generation;
    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        emit machinesChanged({}, error);
        return;
    }
    m_machineCommand.run(program, {"remote", "nodes", "--auth", "never"},
        [this, generation](bool ok, const QByteArray &bytes) {
            if (generation != m_generation) {
                QTimer::singleShot(0, this, &DashboardWidgetFactory::refresh);
                return;
            }
            const auto document = QJsonDocument::fromJson(bytes);
            if (!ok || !document.isArray()) {
                emit machinesChanged({}, tr("Machine inventory unavailable. Configure remote access in Machines."));
                return;
            }
            if (document.array().size() > 4096) {
                emit machinesChanged({}, tr("Machine inventory exceeds the dashboard limit of 4096 entries. Use Machines for the full inventory."));
                return;
            }
            emit machinesChanged(document.array(), {});
        }, 5000);
}

void DashboardWidgetFactory::setPackageDisplays(const QString &source, const QVector<PackageDisplayMeta> &displays)
{
    if (displays.isEmpty())
        m_packageDisplays.remove(source);
    else
        m_packageDisplays[source] = displays;
    emit packageDisplaysChanged();
}

QVector<PackageDisplayMeta> DashboardWidgetFactory::packageDisplays() const
{
    QVector<PackageDisplayMeta> result;
    for (auto source = m_packageDisplays.cbegin(); source != m_packageDisplays.cend(); ++source) {
        for (auto display : source.value()) {
            display.id = source.key() + '/' + display.id;
            result.append(display);
        }
    }
    return result;
}
