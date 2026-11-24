#include "MainWindow.h"
#include "WorkspaceNavigationList.h"
#include "DockpipeChoices.h"
#include <QFileInfo>
#include "LauncherStatusBar.h"
#include "DashboardPage.h"
#include "DashboardWidgetFactory.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceMachineMenu.h"

#include <QAction>
#include <QActionGroup>
#include <QEvent>
#include <QFrame>
#include <QComboBox>
#include <QInputDialog>
#include <QListWidget>
#include <QMenu>
#include <QMenuBar>
#include <QMessageBox>
#include <QPushButton>
#include <QSignalBlocker>
#include <QStackedWidget>
#include <QStatusBar>
#include <QTabWidget>
#include <QTabBar>
#include <QTimer>

namespace {
constexpr int cockpitPageIndex = 8;
}

int MainWindow::selectedPage() const
{
    return m_navigation->currentItem() ? m_navigation->currentItem()->data(Qt::UserRole).toInt() : -1;
}

void MainWindow::selectPage(int page)
{
    for (int row = 0; row < m_navigation->count(); ++row) {
        if (m_navigation->item(row)->data(Qt::UserRole).toInt() == page) {
            m_navigation->setCurrentRow(row);
            m_navigation->ensureCurrentItemVisible();
            return;
        }
    }
}

void MainWindow::setupDashboards()
{
    m_dashboardFactory = new DashboardWidgetFactory(m_sessions, m_store, this);
    m_dashboardFactory->setWorkdir(m_settings.projectFolder);
    m_dashboardFactory->setRemoteDesktopAccessMonitor(m_remoteAccess);
    m_dashboardFactory->setBrowserAccessMonitor(m_remoteAccess);
    connect(m_dashboardFactory, &DashboardWidgetFactory::remoteDesktopInstallProviderRequested,
        this, &MainWindow::onManagePackages);
    connect(m_dashboardFactory, &DashboardWidgetFactory::remoteDesktopProviderSetupRequested, this,
        [this](const QString &workflow, const QString &surface, const QString &action) {
        launchSurfaceWorkflow(workflow, surface, action, {});
    });
    if (m_workspaceMachineMenu) {
        m_dashboardFactory->setMachineCatalog(m_workspaceMachineMenu->catalog());
        m_dashboardFactory->setSharedMachineContext(currentSharedMachineCatalog(), m_workspaceMachineMenu->currentRoot());
    }
    connect(m_dashboardFactory, &DashboardWidgetFactory::machineCatalogRefreshRequested,
            this, &MainWindow::refreshWorkspaceMachines);
    m_dashboardFactory->setWorkflows(m_workspaceWorkflows);
    connect(m_dashboardFactory, &DashboardWidgetFactory::cloneWorkflow, this, &MainWindow::cloneWorkflow);
    connect(m_dashboardFactory, &DashboardWidgetFactory::launchWorkflowOnMachine, this,
        [this](const QString &workflowId, const QJsonObject &target, const QString &root) {
        for (const auto &context : m_advancedSourceContexts) {
            if (context.workflow == workflowId) {
                const Context selected = context;
                launchWorkflow(selected, target, root);
                return;
            }
        }
        statusBar()->showMessage(tr("This workflow is no longer in the project catalog. Refresh Workflows and try again."), 10000);
    });
    connect(m_dashboardFactory, &DashboardWidgetFactory::openFile, this, [this](const QString &path) {
        m_projectFiles->openFileAsync(path, [this](bool opened) {
            if (opened)
                selectPage(5);
        });
    });
    connect(m_dashboardFactory, &DashboardWidgetFactory::openFullView, this, [this](const QString &key) {
        const QMap<QString, int> pages{{"workflows", 1}, {"machines", 2}, {"containers", 3},
            {"system", 4}, {"executions", 1}, {"logs", 1}, {"files", 5}, {"git", 6}, {"browser", 7}, {"terminal", 9}, {"remote-desktop", 10}};
        if (!pages.contains(key))
            return;
        selectPage(pages.value(key));
        if (key == "executions")
            m_workflowSections->setCurrentIndex(2);
        else if (key == "logs")
            openWorkflowRunLogs();
    });
    QString error;
    m_dashboardStoreReady = m_dashboardStore.load(m_dashboards, error);
    m_addDashboard->setEnabled(m_dashboardStoreReady);
    m_addDashboardAction->setEnabled(m_dashboardStoreReady);
    if (!m_dashboardStoreReady) {
        m_addDashboard->setToolTip(error);
        statusBar()->showMessage(error);
        return;
    }
    m_dashboardDirty = DashboardStore::ensureDefaultDashboard(m_dashboards);
    for (const auto &definition : m_dashboards)
        addDashboardNavigation(definition);
    applyDefaultDashboardVisibility();
    if (m_dashboardDirty)
        persistDashboards();
}

void MainWindow::addDashboardNavigation(const DashboardDefinition &definition)
{
    const QString displayName = definition.id == DashboardStore::defaultDashboardId() && definition.name == "Dashboard"
        ? tr("Dashboard") : definition.name;
    auto *placeholder = new QWidget;
    placeholder->setProperty("dashboardId", definition.id);
    const int page = m_stack->addWidget(placeholder);
    auto *item = new QListWidgetItem(displayName);
    item->setData(Qt::UserRole, page);
    item->setData(Qt::UserRole + 1, definition.id);
    item->setSizeHint(QSize(140, 42));
    item->setToolTip(displayName);
    // Keep Cockpit last without changing the built-in page identities.
    int row = m_navigation->count();
    for (int index = 0; index < m_navigation->count(); ++index) {
        if (m_navigation->item(index)->data(Qt::UserRole).toInt() == cockpitPageIndex) {
            row = index;
            break;
        }
    }
    if (definition.id == DashboardStore::defaultDashboardId())
        row = 0;
    const QSignalBlocker blockNavigation(m_navigation);
    m_navigation->insertItem(row, item);
    auto *action = new QAction(displayName, this);
    action->setCheckable(true);
    action->setData(definition.id);
    m_pageGroup->addAction(action);
    m_pageActions.append(action);
    auto *view = findChild<QMenu *>("workspaceViewMenu");
    view->insertAction(m_pageActions[definition.id == DashboardStore::defaultDashboardId() ? 0 : cockpitPageIndex], action);
    connect(action, &QAction::triggered, this, [this, id = definition.id]() {
        for (int index = 0; index < m_stack->count(); ++index) {
            if (m_stack->widget(index)->property("dashboardId").toString() == id) {
                selectPage(index);
                return;
            }
        }
    });
}

void MainWindow::applyDefaultDashboardVisibility()
{
    for (int row = 0; row < m_navigation->count(); ++row) {
        auto *item = m_navigation->item(row);
        if (item->data(Qt::UserRole + 1).toString() != DashboardStore::defaultDashboardId())
            continue;
        const int page = item->data(Qt::UserRole).toInt();
        if (!m_settings.defaultDashboardVisible && selectedPage() == page)
            selectPage(0);
        item->setHidden(!m_settings.defaultDashboardVisible);
        m_pageActions[page]->setVisible(m_settings.defaultDashboardVisible);
        m_pageActions[page]->setEnabled(m_settings.defaultDashboardVisible);
        return;
    }
}

void MainWindow::ensureDashboard(int page)
{
    if (!m_dashboardFactory || page < 0 || page >= m_stack->count())
        return;
    const QString id = m_stack->widget(page)->property("dashboardId").toString();
    if (id.isEmpty() || m_dashboardPages.value(id))
        return;
    for (const auto &definition : m_dashboards) {
        if (definition.id != id)
            continue;
        auto *dashboard = new DashboardPage(definition, m_dashboardFactory);
        dashboard->setCanAddPanel([this]() {
            int panels = 0;
            for (const auto &current : m_dashboards) {
                const auto *loaded = m_dashboardPages.value(current.id).data();
                panels += loaded ? loaded->panelCount() : current.panels.size();
            }
            return panels < DashboardStore::maximumTotalPanels;
        });
        connect(dashboard, &DashboardPage::definitionChanged, this, [this]() {
            m_dashboardDirty = true;
            persistDashboards();
        });
        connect(dashboard, &DashboardPage::renameRequested, this, [this, id]() { renameDashboard(id); });
        connect(dashboard, &DashboardPage::removeRequested, this, [this, id]() { removeDashboard(id); });
        connect(dashboard, &DashboardPage::fullscreenRequested, this, &MainWindow::toggleDashboardFullscreen);
        connect(dashboard, &DashboardPage::projectChangeRequested, this, &MainWindow::onFileOpenProject);
        connect(dashboard, &DashboardPage::sidebarRequested, this, [this, dashboard]() {
            auto *menu = new QMenu(dashboard);
            menu->setObjectName("dashboardNavigationMenu");
            menu->setAttribute(Qt::WA_DeleteOnClose);
            for (int row = 0; row < m_navigation->count(); ++row) {
                const auto *item = m_navigation->item(row);
                const int destination = item->data(Qt::UserRole).toInt();
                auto *action = menu->addAction(item->text());
                action->setVisible(!item->isHidden());
                action->setEnabled(!item->isHidden());
                action->setCheckable(true);
                action->setChecked(destination == selectedPage());
                connect(action, &QAction::triggered, this, [this, destination]() { selectPage(destination); });
            }
            auto *anchor = dashboard->findChild<QPushButton *>("dashboardFullscreenSidebar");
            menu->popup(anchor->mapToGlobal(QPoint(0, anchor->height())));
        });
        connect(dashboard, &DashboardPage::workflowRunOnMachineRequested, this,
            [this](const QString &repository, const QString &path, const QJsonObject &target, const QString &root) {
                Context context = Context::createNew();
                context.workdir = repository;
                context.workflowFile = path;
                context.label = QFileInfo(path).completeBaseName();
                context.dockpipeBinary = DockpipeChoices::preferredDockpipeBinary(repository);
                launchWorkflow(context, target, root);
            });
        connect(dashboard, &DashboardPage::cloneWorkflowRequested, this, &MainWindow::cloneWorkflow);
        connect(dashboard, &DashboardPage::appWorkflowRequested, this, [this, dashboard](const QString &workflowId) {
            for (const auto &context : m_advancedSourceContexts) {
                if (context.workflow != workflowId)
                    continue;
                const Context selected = context;
                const bool local = m_workspaceMachineMenu->currentTarget().isEmpty();
                if (launchWorkflow(selected) && local)
                    dashboard->addWindowForApplication(selected.label);
                return;
            }
            statusBar()->showMessage(tr("This app is no longer in the project catalog. Refresh Apps and try again."), 10000);
        });
        m_dashboardPages.insert(id, dashboard);
        updateWorkspaceMachineAccess();
        const QSignalBlocker blockStack(m_stack);
        auto *placeholder = m_stack->widget(page);
        m_stack->removeWidget(placeholder);
        m_stack->insertWidget(page, dashboard);
        delete placeholder;
        return;
    }
}

void MainWindow::toggleDashboardFullscreen()
{
    if (isFullScreen()) {
        if (m_fullscreenWasMaximized)
            showMaximized();
        else
            showNormal();
    } else {
        m_fullscreenWasMaximized = isMaximized();
        showFullScreen();
    }
    updateDashboardFullscreen();
}

void MainWindow::changeEvent(QEvent *event)
{
    QMainWindow::changeEvent(event);
    if (event->type() == QEvent::WindowStateChange)
        updateDashboardFullscreen();
}

void MainWindow::updateStatusBarVisibility()
{
    auto *bar = static_cast<LauncherStatusBar *>(statusBar());
    bar->setNoticeVisible(m_disclaimerContainer != nullptr);
}

void MainWindow::updateDashboardFullscreen()
{
    if (!m_stack || !m_navigationRail)
        return;
    const bool dashboardFullscreen = isFullScreen() && qobject_cast<DashboardPage *>(m_stack->currentWidget());
    updateStatusBarVisibility();
    menuBar()->setVisible(!dashboardFullscreen);
    m_navigationRail->setVisible(!dashboardFullscreen && m_settings.sidebarVisible);
    for (const auto &dashboard : m_dashboardPages) {
        if (dashboard)
            dashboard->setFullscreen(isFullScreen());
    }
    if (m_fullscreenAction) {
        const QSignalBlocker blocker(m_fullscreenAction);
        m_fullscreenAction->setChecked(isFullScreen());
    }
}

QVector<DashboardDefinition> MainWindow::dashboardSnapshots() const
{
    auto definitions = m_dashboards;
    for (auto &definition : definitions) {
        if (const auto *page = m_dashboardPages.value(definition.id).data())
            definition = page->snapshot();
    }
    return definitions;
}

bool MainWindow::persistDashboards()
{
    updateWorkspaceMachineAccess();
    if (!m_dashboardStoreReady)
        return true;
    bool pending = m_dashboardDirty;
    for (const auto &page : m_dashboardPages)
        pending = pending || (page && page->hasPendingChanges());
    if (!pending)
        return true;
    const auto definitions = dashboardSnapshots();
    QString error;
    const bool saved = m_dashboardStore.save(definitions, error);
    for (const auto &page : m_dashboardPages) {
        if (page)
            page->showSaveError(error);
    }
    if (!saved) {
        m_dashboardDirty = true;
        statusBar()->showMessage(error);
        return false;
    }
    m_dashboards = definitions;
    m_dashboardDirty = false;
    return true;
}

namespace {
void confirmDashboardPages(QPointer<MainWindow> owner, QList<QPointer<DashboardPage>> pages,
                           std::function<void(bool)> completion)
{
    if (!owner)
        return;
    while (!pages.isEmpty() && !pages.first())
        pages.removeFirst();
    if (pages.isEmpty()) {
        completion(true);
        return;
    }
    const QPointer<DashboardPage> page = pages.takeFirst();
    page->confirmLeaveAsync([owner, pages = std::move(pages), completion = std::move(completion)](bool accepted) mutable {
        if (!owner)
            return;
        if (!accepted) {
            completion(false);
            return;
        }
        confirmDashboardPages(owner, std::move(pages), std::move(completion));
    });
}
}

void MainWindow::confirmDashboardCloseAsync(std::function<void(bool)> completion)
{
    QList<QPointer<DashboardPage>> pages;
    for (const auto &page : m_dashboardPages) {
        if (page)
            pages.append(page);
    }
    const QPointer<MainWindow> guard(this);
    confirmDashboardPages(this, std::move(pages), [guard, completion = std::move(completion)](bool accepted) {
        if (!guard)
            return;
        if (!accepted || guard->persistDashboards()) {
            completion(accepted);
            return;
        }
        QMessageBox dialog(QMessageBox::Warning, tr("Unsaved dashboard changes"),
            tr("Dashboard changes could not be saved. Close without saving these changes? The saved dashboard file will be kept."),
            QMessageBox::Discard | QMessageBox::Cancel, guard);
        dialog.setObjectName("dashboardUnsavedClose");
        dialog.setDefaultButton(QMessageBox::Cancel);
        dialog.button(QMessageBox::Discard)->setText(tr("Close without saving"));
        completion(dialog.exec() == QMessageBox::Discard);
    });
}

void MainWindow::applyTilingAvailability()
{
    for (const auto &page : m_dashboardPages) {
        if (page)
            page->setTilingEnabled(m_settings.tilingEnabled);
    }
}

void MainWindow::releaseDashboardWindows()
{
    for (const auto &page : m_dashboardPages) {
        if (page)
            page->releaseDesktopWindows();
    }
}

void MainWindow::createDashboard()
{
    if (!m_dashboardStoreReady)
        return;
    if (m_dashboards.size() >= DashboardStore::maximumDashboards) {
        statusBar()->showMessage(tr("Up to 12 custom dashboards are supported. Remove one before creating another."), 10000);
        return;
    }
    auto *dialog = new QInputDialog(this);
    dialog->setObjectName("dashboardNameDialog");
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setWindowTitle(tr("New dashboard"));
    dialog->setLabelText(tr("Dashboard name"));
    dialog->setTextValue(tr("My dashboard"));
    connect(dialog, &QInputDialog::textValueSelected, this, [this](const QString &text) {
        const QString name = text.trimmed();
        if (name.isEmpty() || name.size() > 80) {
            statusBar()->showMessage(tr("Use a dashboard name between 1 and 80 characters."), 10000);
            return;
        }
        auto definitions = dashboardSnapshots();
        DashboardDefinition created{DashboardStore::newId(), name, {}, {}};
        definitions.append(created);
        QString error;
        if (!m_dashboardStore.save(definitions, error)) {
            statusBar()->showMessage(error);
            return;
        }
        m_dashboards = definitions;
        addDashboardNavigation(created);
        selectPage(m_stack->count() - 1);
        if (auto *page = m_dashboardPages.value(created.id).data())
            QTimer::singleShot(0, page, [page]() { page->showTour(true); });
    });
    dialog->open();
}

void MainWindow::renameDashboard(const QString &id)
{
    if (id == DashboardStore::defaultDashboardId())
        return;
    auto *dialog = new QInputDialog(this);
    dialog->setObjectName("dashboardRenameDialog");
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setWindowTitle(tr("Rename dashboard"));
    dialog->setLabelText(tr("Dashboard name"));
    for (const auto &definition : m_dashboards) {
        if (definition.id == id)
            dialog->setTextValue(definition.name);
    }
    connect(dialog, &QInputDialog::textValueSelected, this, [this, id](const QString &text) {
        auto definitions = dashboardSnapshots();
        for (auto &definition : definitions) {
            if (definition.id == id)
                definition.name = text.trimmed();
        }
        QString error;
        if (!m_dashboardStore.save(definitions, error)) {
            statusBar()->showMessage(error);
            return;
        }
        m_dashboards = definitions;
        if (auto *page = m_dashboardPages.value(id).data())
            page->setName(text.trimmed());
        for (int row = 0; row < m_navigation->count(); ++row) {
            auto *item = m_navigation->item(row);
            if (item->data(Qt::UserRole + 1).toString() == id) {
                item->setText(text.trimmed());
                item->setToolTip(text.trimmed());
                m_pageActions[item->data(Qt::UserRole).toInt()]->setText(text.trimmed());
            }
        }
    });
    dialog->open();
}

void MainWindow::removeDashboard(const QString &id)
{
    if (id == DashboardStore::defaultDashboardId())
        return;
    auto *confirmation = new QMessageBox(QMessageBox::Question, tr("Remove dashboard"),
        tr("Remove this dashboard and its layout? Running workflows and saved files are kept. Browser and capture widgets will close."),
        QMessageBox::Yes | QMessageBox::Cancel, this);
    confirmation->setObjectName("dashboardRemoveConfirmation");
    confirmation->setTextFormat(Qt::PlainText);
    confirmation->setAttribute(Qt::WA_DeleteOnClose);
    confirmation->setDefaultButton(QMessageBox::Cancel);
    connect(confirmation, &QMessageBox::finished, this, [this, id](int result) {
        if (result != QMessageBox::Yes)
            return;
        auto remove = [this, id](bool accepted) {
            if (!accepted)
                return;
            auto definitions = dashboardSnapshots();
            for (int index = 0; index < definitions.size(); ++index) {
                if (definitions[index].id == id) {
                    definitions.removeAt(index);
                    break;
                }
            }
            QString error;
            if (!m_dashboardStore.save(definitions, error)) {
                statusBar()->showMessage(error);
                return;
            }
            m_dashboards = definitions;
            selectPage(0);
            const QSignalBlocker blockNavigation(m_navigation);
            const QSignalBlocker blockStack(m_stack);
            for (int row = 0; row < m_navigation->count(); ++row) {
                auto *item = m_navigation->item(row);
                if (item->data(Qt::UserRole + 1).toString() != id)
                    continue;
                const int page = item->data(Qt::UserRole).toInt();
                auto *widget = m_stack->widget(page);
                m_dashboardPages.remove(id);
                m_stack->removeWidget(widget);
                widget->deleteLater();
                delete m_navigation->takeItem(row);
                delete m_pageActions.takeAt(page);
                for (int remaining = 0; remaining < m_navigation->count(); ++remaining) {
                    auto *entry = m_navigation->item(remaining);
                    const int previous = entry->data(Qt::UserRole).toInt();
                    if (previous > page)
                        entry->setData(Qt::UserRole, previous - 1);
                }
                break;
            }
        };
        if (auto *page = m_dashboardPages.value(id).data())
            page->confirmLeaveAsync(std::move(remove));
        else
            remove(true);
    });
    confirmation->open();
}
