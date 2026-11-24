#include "MainWindow.h"
#include "WorkspaceNavigationList.h"
#include "PackageSurfacePage.h"
#include "DockpipeChoices.h"
#include "DashboardPage.h"
#include "DashboardWidgetFactory.h"
#include "EnvironmentsPage.h"

#include <QAction>
#include <QActionGroup>
#include <QDir>
#include <QListWidget>
#include <QUuid>
#include <memory>
#include <QMenu>
#include <QMessageBox>
#include <QSignalBlocker>
#include <QStackedWidget>

void MainWindow::syncPackageSurfaces(const QVector<ResolverMeta> &providers)
{
    QMap<QString, QVector<ResolverMeta>> groups;
    for (const auto &provider : providers) {
        if (provider.capability == "remote.desktop" || provider.surface.id.isEmpty() || provider.surface.snapshot.isEmpty())
            continue;
        ResolverMeta contribution = provider;
        for (int index = contribution.surface.actions.size() - 1; index >= 0; --index) {
            const auto &action = contribution.surface.actions[index];
            if (action.scope == "file") {
                contribution.surface.actions.removeAt(index);
            } else if (!action.workflowId.isEmpty()) {
                const auto workflow = findWorkflowMeta(m_settings.projectFolder, action.workflowId, QString());
                if (workflow.workflowId.isEmpty() || !workflow.supported)
                    contribution.surface.actions.removeAt(index);
            }
        }
        groups[provider.surface.id].append(contribution);
    }
    for (const auto &id : m_surfacePages.keys()) {
        if (!groups.contains(id))
            removePackageSurface(id);
    }
    for (auto group = groups.cbegin(); group != groups.cend(); ++group) {
        auto *page = m_surfacePages.value(group.key()).data();
        if (!page) {
            page = new PackageSurfacePage(m_sessions);
            page->setProperty("surfaceId", group.key());
            m_surfacePages.insert(group.key(), page);
            if (group.key() == QStringLiteral("dockpipe.vm")) {
                m_environmentsPage->setVirtualMachineControls(page);
                connect(m_environmentsPage, &EnvironmentsPage::virtualMachineControlsActivityChanged,
                        page, [this, page](bool active) {
                            page->setActive(active);
                            if (active || page->property("environmentConsoleWorkdir").toString().isEmpty())
                                return;
                            page->setProperty("environmentConsoleWorkdir", QString());
                            const QString workdir = m_settings.projectFolder.isEmpty()
                                ? QDir::homePath() : m_settings.projectFolder;
                            page->setWorkspace(workdir, DockpipeChoices::preferredDockpipeBinary(workdir));
                        });
                connect(page, &PackageSurfacePage::resourceInventoryChanged,
                        m_environmentsPage, &EnvironmentsPage::setVirtualMachineInventory);
                connect(m_environmentsPage, &EnvironmentsPage::runtimeInventoryRequested,
                        page, &PackageSurfacePage::refreshProcessInventory);
            } else {
                const int index = m_stack->addWidget(page);
                auto *item = new QListWidgetItem(group.value().first().surface.title);
                item->setData(Qt::UserRole, index);
                item->setData(Qt::UserRole + 2, group.key());
                item->setSizeHint(QSize(140, 42));
                const QSignalBlocker block(m_navigation);
                m_navigation->addItem(item);
                auto *action = new QAction(item->text(), this);
                action->setCheckable(true);
                m_pageGroup->addAction(action);
                m_pageActions.append(action);
                if (auto *view = findChild<QMenu *>("workspaceViewMenu"))
                    view->addAction(action);
                const QPointer<PackageSurfacePage> guarded(page);
                connect(action, &QAction::triggered, this, [this, guarded]() {
                    if (guarded)
                        selectPage(m_stack->indexOf(guarded));
                });
            }
            connect(page, &PackageSurfacePage::displaysChanged, this,
                [this, id = group.key()](const QVector<PackageDisplayMeta> &displays) {
                    if (m_dashboardFactory)
                        m_dashboardFactory->setPackageDisplays(id, displays);
                });
            connect(m_dashboardFactory, &DashboardWidgetFactory::packageDisplayInventoryRequested,
                page, &PackageSurfacePage::refreshInventory);
            connect(page, &PackageSurfacePage::publicWorkflowRequested, this, &MainWindow::launchSurfaceWorkflow);
            if (m_environmentsPage && group.key() == QStringLiteral("dockpipe.vm"))
                connect(page, &PackageSurfacePage::instanceStatesObserved,
                        m_environmentsPage, &EnvironmentsPage::setVmObservations);
        }
        QString workdir = m_settings.projectFolder;
        if (group.key() == QStringLiteral("dockpipe.vm")) {
            const auto consoleWorkdir = page->property("environmentConsoleWorkdir").toString();
            if (!consoleWorkdir.isEmpty())
                workdir = consoleWorkdir;
            else if (workdir.isEmpty())
                workdir = QDir::homePath();
        }
        page->setWorkspace(workdir, DockpipeChoices::preferredDockpipeBinary(workdir));
        page->setProviders(group.value());
        page->setActive(group.key() == QStringLiteral("dockpipe.vm")
            ? m_environmentsPage->virtualMachineControlsActive() : m_stack->currentWidget() == page);
        if (group.key() == QStringLiteral("dockpipe.vm") && m_stack->currentWidget() == m_environmentsPage)
            page->refreshProcessInventory();
    }
}

void MainWindow::removePackageSurface(const QString &id)
{
    if (m_dashboardFactory)
        m_dashboardFactory->setPackageDisplays(id, {});
    auto *page = m_surfacePages.take(id).data();
    if (!page)
        return;
    if (id == QStringLiteral("dockpipe.vm")) {
        m_environmentsPage->setVmObservations({}, 0);
        m_environmentsPage->setVirtualMachineInventory({});
        m_environmentsPage->setVirtualMachineControls(nullptr);
        delete page;
        return;
    }
    if (m_stack->currentWidget() == page)
        selectPage(0);
    const int index = m_stack->indexOf(page);
    const QSignalBlocker blockNavigation(m_navigation);
    const QSignalBlocker blockStack(m_stack);
    for (int row = m_navigation->count() - 1; row >= 0; --row) {
        auto *item = m_navigation->item(row);
        const int previous = item->data(Qt::UserRole).toInt();
        if (previous == index)
            delete m_navigation->takeItem(row);
        else if (previous > index)
            item->setData(Qt::UserRole, previous - 1);
    }
    m_stack->removeWidget(page);
    delete m_pageActions.takeAt(index);
    delete page;
}

void MainWindow::launchSurfaceWorkflow(const QString &workflowId, const QString &surfaceId,
                                     const QString &actionId, const QString &resourceId)
{
    const auto meta = findWorkflowMeta(m_settings.projectFolder, workflowId, QString());
    if (meta.workflowId.isEmpty() || !meta.supported) {
        QMessageBox::information(this, tr("Workflow unavailable"), tr("The package action's public workflow is not installed or supported."));
        return;
    }
    Context display = Context::createNew();
    display.workdir = m_settings.projectFolder;
    display.workflow = workflowId;
    display.workflowFile = meta.configPath;
    display.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(display.workdir);
    display.label = meta.displayName;
    auto *context = ensureStoredContextForDisplay(display);
    if (!meta.inputs.isEmpty() && !configureContextForWorkflow(*context, meta, true))
        return;
    // Per-invocation resource binding is never persisted as a workflow default.
    Context invocation = *context;
    invocation.extraDockpipeEnv << "DOCKPIPE_SURFACE_ID=" + surfaceId
                               << "DOCKPIPE_SURFACE_ACTION_ID=" + actionId
                               << "DOCKPIPE_SURFACE_RESOURCE_ID=" + resourceId;
    launchPackageAwareContext(invocation, workflowLaunchOptions(meta));
}

bool MainWindow::launchPackageAwareContext(const Context &context, const SessionLaunchOptions &options)
{
    QPointer<PackageSurfacePage> destination;
    for (const auto &page : m_surfacePages) {
        if (!page || !page->supportsWorkflow(context.workflow))
            continue;
        // An ambiguous contribution cannot silently choose an unrelated provider.
        if (destination)
            return m_sessions.launch(context, ContextStore::logsDir(), options);
        destination = page;
    }
    Context invocation = context;
    const QString launchId = destination ? QUuid::createUuid().toString(QUuid::WithoutBraces) : QString();
    if (destination) {
        QString error;
        if (!destination->prepareLaunchContext(invocation, error)) {
            QMessageBox::information(this, tr("Package launch unavailable"), error);
            return false;
        }
        invocation.extraDockpipeEnv.removeIf([](const QString &entry) {
            return entry.startsWith("DOCKPIPE_SURFACE_LAUNCH_ID=");
        });
        invocation.extraDockpipeEnv.append("DOCKPIPE_SURFACE_LAUNCH_ID=" + launchId);
    }
    if (!m_sessions.launch(invocation, ContextStore::logsDir(), options))
        return false;
    if (!destination)
        return true;
    destination->expectLaunch(context.workflow, launchId);
    if (destination->property("surfaceId").toString() == "dockpipe.vm") {
        selectPage(m_stack->indexOf(m_environmentsPage));
        m_environmentsPage->showVirtualMachineControls();
    } else {
        selectPage(m_stack->indexOf(destination));
    }
    const auto failed = std::make_shared<QMetaObject::Connection>();
    const auto stopped = std::make_shared<QMetaObject::Connection>();
    const auto resolved = std::make_shared<QMetaObject::Connection>();
    const auto cleanup = [failed, stopped, resolved]() {
        QObject::disconnect(*failed);
        QObject::disconnect(*stopped);
        QObject::disconnect(*resolved);
    };
    *failed = connect(&m_sessions, &SessionManager::sessionFailed, destination,
        [destination, contextId = context.id, launchId, cleanup](const QString &id, const QString &) {
            if (id == contextId) {
                destination->cancelLaunch(launchId);
                cleanup();
            }
        });
    *stopped = connect(&m_sessions, &SessionManager::sessionStopped, destination,
        [this, destination, contextId = context.id, launchId, cleanup](const QString &id, int exitCode, QProcess::ExitStatus status) {
            if (id == contextId && (exitCode != 0 || status != QProcess::NormalExit || m_sessions.info(id).stopRequested)) {
                destination->cancelLaunch(launchId);
                cleanup();
            }
        });
    *resolved = connect(destination, &PackageSurfacePage::launchResolved, destination,
        [launchId, cleanup](const QString &id) {
            if (id == launchId)
                cleanup();
        });
    return true;
}
