#include "MainWindow.h"
#include "RemoteBrowserWidget.h"

#include "DashboardPage.h"
#include "DashboardWidgetFactory.h"
#include "DockerObservabilityWidget.h"
#include "EnvironmentsPage.h"
#include "ProjectFilesWidget.h"
#include "ProjectGitClient.h"
#include "ProjectGitWidget.h"
#include "RemoteAccessMonitor.h"
#include "RemoteFeatureClient.h"
#include "RemoteFeatureRecoveryDialog.h"
#include "RemoteGitContext.h"
#include "RemoteWorkspaceFilesDispatch.h"
#include "TerminalWidget.h"
#include "TerminalHub.h"
#include "WorkspaceFilesController.h"
#include "WorkspaceMachineMenu.h"
#include "WorkflowEditorWidget.h"

#include <QHBoxLayout>
#include <QDialog>
#include <QVBoxLayout>
#include <QComboBox>
#include <QSignalBlocker>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QRegularExpression>
#include <QSet>
#include <QStatusBar>
#include <QStackedWidget>
#include <QStandardItemModel>
#include <QTimer>

void MainWindow::setupWorkspaceMachineMenu(QHBoxLayout *layout)
{
    m_workspaceMachineMenu = new WorkspaceMachineMenu(this);
    layout->addWidget(m_workspaceMachineMenu);
    layout->addWidget(RemoteFeatureRecoveryDialog::createButton(this,
        [this](const QJsonObject &reference, const QJsonObject &) {
            if (m_dashboardFactory)
                m_dashboardFactory->terminals().releaseRecoveredSession(reference);
        }));
    connect(m_workspaceMachineMenu, &WorkspaceMachineMenu::contextRequested,
            this, &MainWindow::chooseWorkspaceMachine);
    connect(m_workspaceMachineMenu, &WorkspaceMachineMenu::refreshRequested,
            this, &MainWindow::refreshWorkspaceMachines);
    connect(m_remoteAccess, &RemoteAccessMonitor::unlockFinished, this, [this](bool succeeded) {
        if (succeeded)
            refreshWorkspaceMachines();
    });
    connect(m_remoteAccess, &RemoteAccessMonitor::changed, this, [this] {
        const bool unlocked = m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked;
        if (!unlocked)
            m_environmentsPage->setMachineCapabilities({});
        if (!unlocked && m_terminalWidget)
            m_terminalWidget->setWorkspaceTarget(
                m_workspaceMachineMenu->currentTarget().isEmpty() ? QJsonObject{}
                    : QJsonObject{{"target", m_workspaceMachineMenu->currentTarget()}},
                m_workspaceMachineMenu->currentRoot());
        updateWorkspaceMachineAccess();
        updateWorkspaceMachineLabels();
        if (unlocked && m_workspaceMachineMenu->catalog().isEmpty())
            refreshWorkspaceMachines();
    });
    connect(m_dockerWidget, &DockerObservabilityWidget::remoteExecRequested,
            this, &MainWindow::openRemoteContainerExec);
    m_dockerWidget->setFeatureCatalog({});
    updateWorkspaceMachineAccess();
    if (m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked)
        QTimer::singleShot(0, this, &MainWindow::refreshWorkspaceMachines);
}

void MainWindow::updateWorkspaceMachineAccess()
{
    const bool unlocked = m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked;
    for (auto *menu : findChildren<WorkspaceMachineMenu *>()) {
        menu->setPairedNodes(m_pairedMachines);
        menu->setAccessUnlocked(unlocked);
        if (!menu->property("remoteAccessConnected").toBool()) {
            connect(menu, &WorkspaceMachineMenu::unlockRequested, m_remoteAccess, &RemoteAccessMonitor::unlock);
            menu->setProperty("remoteAccessConnected", true);
        }
    }
    if (m_terminalWidget && m_workspaceMachineMenu)
        m_terminalWidget->setMachineTargets(m_pairedMachines, m_workspaceMachineMenu->catalog(), unlocked);
}

QJsonObject MainWindow::currentSharedMachineCatalog() const
{
    if (!m_workspaceMachineMenu || m_workspaceMachineMenu->currentTarget().isEmpty())
        return {};
    for (const auto &value : m_workspaceMachineMenu->catalog()) {
        const auto catalog = value.toObject();
        if (catalog.value("target").toObject() == m_workspaceMachineMenu->currentTarget())
            return catalog;
    }
    return {{"target", m_workspaceMachineMenu->currentTarget()}};
}

void MainWindow::refreshWorkspaceMachines()
{
    if (m_remoteAccess->state() != RemoteAccessMonitor::State::Unlocked) {
        statusBar()->showMessage(m_remoteAccess->detail(), 8000);
        return;
    }
    if (property("workspaceMachineCatalogPending").toBool())
        return;
    setProperty("workspaceMachineCatalogPending", true);
    RemoteFeatureClient::catalog(this, [this](const RemoteFeatureClient::Result &result) {
        setProperty("workspaceMachineCatalogPending", false);
        if (result.outcome != RemoteFeatureClient::Outcome::Success) {
            statusBar()->showMessage(result.error, 8000);
            return;
        }
        const auto catalog = result.value.toArray();
        m_workspaceMachineMenu->setCatalog(catalog);
        if (m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked)
            m_environmentsPage->setMachineCapabilities(catalog);
        if (m_browserWidget)
            m_browserWidget->setMachineCatalog(catalog, m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked);
        const auto selectedCatalog = currentSharedMachineCatalog();
        m_dockerWidget->setFeatureCatalog(selectedCatalog);
        m_dockerWidget->setEnabled(m_workspaceMachineMenu->currentTarget().isEmpty()
            || selectedCatalog.value("docker_read").toBool());
        if (m_terminalWidget && m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked) {
            m_terminalWidget->setWorkspaceTarget(currentSharedMachineCatalog(), m_workspaceMachineMenu->currentRoot());
            refreshTerminalRemoteContainers();
        }
        if (m_dashboardFactory)
            m_dashboardFactory->setMachineCatalog(catalog);
        for (const auto &page : m_dashboardPages) {
            if (page)
                page->setMachineCatalog(catalog);
        }
        updateWorkspaceMachineAccess();
        updateWorkspaceMachineLabels();
    });
}

void MainWindow::updateWorkspaceMachineLabels()
{
    if (!m_workspaceMachineMenu)
        return;
    const bool local = m_workspaceMachineMenu->currentTarget().isEmpty();
    const QSignalBlocker runBlocker(m_runTarget);
    const QSignalBlocker pickerBlocker(m_pickerTarget);
    m_runTarget->clear();
    m_runTarget->addItem(tr("This computer"), QString());
    for (const auto &node : m_pairedMachines)
        m_runTarget->addItem(node, node);
    const auto selectedNode = m_workspaceMachineMenu->currentTarget().value("node").toString();
    if (m_runTarget->findData(selectedNode) < 0)
        m_runTarget->addItem(tr("%1 · Unavailable").arg(selectedNode), selectedNode);
    m_runTarget->setCurrentIndex(m_runTarget->findData(selectedNode));

    const auto selectedDelivery = m_pickerTarget->currentData().toJsonObject();
    m_pickerTarget->clear();
    m_pickerTarget->addItem(m_workspaceMachineMenu->text());
    QSet<QString> deliveryNodes;
    for (const auto &entry : m_workspaceMachineMenu->catalog()) {
        const auto catalog = entry.toObject();
        const auto target = catalog.value("target").toObject();
        if (!catalog.value("workflow_only").toBool() || target.value("node").toString().isEmpty()
            || m_remoteAccess->state() != RemoteAccessMonitor::State::Unlocked)
            continue;
        deliveryNodes.insert(target.value("node").toString());
        m_pickerTarget->addItem(target.value("node").toString(), QVariant::fromValue(target));
    }
    auto *deliveryModel = qobject_cast<QStandardItemModel *>(m_pickerTarget->model());
    for (const auto &node : m_pairedMachines) {
        if (deliveryNodes.contains(node))
            continue;
        m_pickerTarget->addItem(tr("%1 · Workflow access unavailable").arg(node));
        if (deliveryModel)
            deliveryModel->item(m_pickerTarget->count() - 1)->setEnabled(false);
    }
    if (!selectedDelivery.isEmpty()) {
        const auto data = QVariant::fromValue(selectedDelivery);
        int index = m_pickerTarget->findData(data);
        if (index < 0) {
            m_pickerTarget->addItem(tr("%1 · Unavailable").arg(selectedDelivery.value("node").toString()), data);
            index = m_pickerTarget->count() - 1;
            if (deliveryModel)
                deliveryModel->item(index)->setEnabled(false);
        }
        m_pickerTarget->setCurrentIndex(index);
    }
    m_pickerTarget->setEnabled(m_pickerTarget->count() > 1);
    updateWorkflowPreview();
    m_workspaceButton->setEnabled(local);
    if (local)
        return;
    m_workspaceButton->setText(m_workspaceMachineMenu->currentRoot());
    m_workspacePath->setText(m_workspaceMachineMenu->currentTarget().value("node").toString());
    m_workspacePath->setToolTip(m_workspaceMachineMenu->text());
}

void MainWindow::chooseWorkspaceMachine(const QJsonObject &target, const QString &root)
{
    if (property("workspaceMachineSwitching").toBool()
        || property("workspaceChangePending").toBool() || property("leaveConfirmationPending").toBool())
        return;
    if (!property("workspaceMachineCommitFailed").toBool()
        && target == m_workspaceMachineMenu->currentTarget() && root == m_workspaceMachineMenu->currentRoot())
        return;

    QJsonObject catalog;
    QJsonObject rootGrant;
    if (!target.isEmpty()) {
        if (m_remoteAccess->state() != RemoteAccessMonitor::State::Unlocked) {
            m_remoteAccess->unlock();
            return;
        }
        for (const auto &value : m_workspaceMachineMenu->catalog()) {
            const auto candidate = value.toObject();
            if (candidate.value("target").toObject() == target) {
                catalog = candidate;
                break;
            }
        }
        for (const auto &value : catalog.value("roots").toArray()) {
            if (value.toObject().value("id").toString() == root)
                rootGrant = value.toObject();
        }
        if (catalog.isEmpty() || (!root.isEmpty() && rootGrant.isEmpty())) {
            statusBar()->showMessage(tr("This machine or root changed. Refresh machines and select it again."), 8000);
            return;
        }
    }

    bool protectedWork = (m_workflowEditor && m_workflowEditor->operationPending())
        || m_projectFiles->hasUnsavedChanges() || m_projectFiles->filesOperationPending()
        || !m_projectGit->confirmLeave() || m_dockerWidget->controlPending();
    for (auto *client : m_projectGit->findChildren<ProjectGitClient *>())
        protectedWork = protectedWork || client->busy();
    if (auto *message = m_projectGit->findChild<QLineEdit *>("gitCommitMessage"))
        protectedWork = protectedWork || !message->text().trimmed().isEmpty();
    if (protectedWork) {
        statusBar()->showMessage(tr("Save or discard open drafts and finish active operations before changing machines."), 10000);
        return;
    }
    QString error;
    for (const auto &page : m_dashboardPages) {
        if (page && !page->canChangeSharedMachineContext(&error)) {
            statusBar()->showMessage(error, 10000);
            return;
        }
    }

    // All checks above are side-effect free. Commit without yielding to the event
    // loop so another view cannot start a write between context checks and binding.
    setProperty("workspaceMachineSwitching", true);
    const auto denyCommit = [this, target, root](const QString &detail) {
    // No request is redirected to localhost. Preserve the exact attempted
    // selection and deny interaction if an unexpected commit precondition fails.
    m_projectFiles->setEnabled(false);
    m_projectGit->setEnabled(false);
    m_dockerWidget->setEnabled(false);
    for (const auto &page : m_dashboardPages) {
        if (page)
            page->setEnabled(false);
    }
    if (m_terminalWidget)
        m_terminalWidget->setEnabled(false);
    m_workspaceMachineMenu->setCurrent(target, root);
    setProperty("workspaceMachineCommitFailed", true);
    setProperty("workspaceMachineSwitching", false);
    statusBar()->showMessage(detail.isEmpty()
        ? tr("Choose another computer or reconnect this machine before running.") : detail, 10000);
    };
    WorkspaceFilesClient::Context files;
    files.generation = WorkspaceFilesController::get(m_projectFiles)->reader->context().generation + 1;
    if (target.isEmpty()) {
        files.localRoot = m_settings.projectFolder;
    } else {
        files.kind = WorkspaceFilesClient::Context::Kind::Remote;
        files.nodeId = target.value("node").toString();
        files.fingerprint = target.value("fingerprint").toString();
        files.workerGeneration = target.value("generation").toString();
        files.rootId = root;
        files.writeGranted = rootGrant.value("files_write").toBool();
    }
    const bool filesAccepted = WorkspaceFilesController::configure(m_projectFiles, files,
        target.isEmpty() ? WorkspaceFilesClient::RemoteDispatch{} : RemoteWorkspaceFilesDispatch::create(m_projectFiles));
    const bool gitAccepted = target.isEmpty()
        ? RemoteGitContext::configureLocal(m_projectGit, m_settings.projectFolder, &error)
        : RemoteGitContext::configure(m_projectGit, target, root, rootGrant, &error);
    const bool containersAccepted = m_dockerWidget->setFeatureCatalog(catalog, &error);
    if (!filesAccepted || !gitAccepted || !containersAccepted) {
        denyCommit(error);
        return;
    }
    m_projectFiles->setEnabled(target.isEmpty() || rootGrant.value("files_read").toBool());
    m_projectGit->setEnabled(target.isEmpty() || rootGrant.value("git_read").toBool());
    m_dockerWidget->setEnabled(target.isEmpty() || catalog.value("docker_read").toBool());
    bool dashboardsAccepted = true;
    if (m_dashboardFactory)
        dashboardsAccepted = m_dashboardFactory->setSharedMachineContext(catalog, root, &error);
    for (const auto &page : m_dashboardPages) {
        if (page) {
            const bool accepted = page->setSharedMachineContext(catalog, root, &error);
            dashboardsAccepted = accepted && dashboardsAccepted;
            page->setEnabled(accepted);
        }
    }
    if (!dashboardsAccepted) {
        denyCommit(error);
        return;
    }
    if (m_terminalWidget) {
        m_terminalWidget->setEnabled(true);
        m_terminalWidget->setWorkspaceTarget(catalog, root);
    }
    m_workspaceMachineMenu->setCurrent(target, root);
    refreshTerminalRemoteContainers();
    m_workspaceButton->setEnabled(target.isEmpty());
    if (target.isEmpty())
        updateBasicPage();
    else
        updateWorkspaceMachineLabels();
    setProperty("workspaceMachineCommitFailed", false);
    setProperty("workspaceMachineSwitching", false);
    if (m_projectGit->isEnabled())
        m_projectGit->refresh();
}

void MainWindow::refreshTerminalRemoteContainers()
{
    if (!m_terminalWidget || m_stack->currentWidget() != m_terminalWidget
        || m_remoteAccess->state() != RemoteAccessMonitor::State::Unlocked)
        return;

    const auto catalog = currentSharedMachineCatalog();
    const auto target = catalog.value("target").toObject();
    if (target.isEmpty() || !catalog.value("docker_read").toBool() || !catalog.value("docker_exec").toBool()
        || catalog.value("terminal_epoch").toString().isEmpty())
        return;

    const auto request = RemoteFeatureClient::prepareIntent(target, {}, "containers.snapshot", {});
    RemoteFeatureClient::invoke(m_terminalWidget, request, [this, catalog, target](const auto &result) {
        if (m_remoteAccess->state() != RemoteAccessMonitor::State::Unlocked || currentSharedMachineCatalog() != catalog)
            return;

        QMap<QString, QString> containers;
        const auto snapshot = result.value.toObject();
        const auto rows = snapshot.value("containers").toArray();
        if (result.outcome == RemoteFeatureClient::Outcome::Success && snapshot.value("available").toBool()
            && rows.size() <= 256) {
            static const QRegularExpression fullId(QStringLiteral("\\A[0-9a-f]{64}\\z"));
            for (const auto &entry : rows) {
                const auto row = entry.toObject();
                const auto id = row.value("ID").toString();
                const auto name = row.value("Names").toString();
                if (row.value("State") == "running" && fullId.match(id).hasMatch() && !name.isEmpty())
                    containers.insert(id, name);
            }
        }
        m_terminalWidget->setRemoteContainers(catalog, containers);
    });
}

void MainWindow::openRemoteContainerExec(const QJsonObject &catalog, const QString &container, const QString &shell)
{
    if (!m_dashboardFactory)
        return;
    auto *dialog = new QDialog(this);
    dialog->setObjectName("remoteContainerTerminal");
    dialog->setAttribute(Qt::WA_DeleteOnClose);
    dialog->setWindowTitle(tr("Container terminal · %1 · %2")
        .arg(catalog.value("target").toObject().value("node").toString(), container.left(12)));
    dialog->resize(1000, 640);
    auto *layout = new QVBoxLayout(dialog);
    auto *terminal = new TerminalWidget(m_dashboardFactory->terminals(), {}, dialog, TerminalWidget::Mode::Dashboard);
    QString error;
    if (!terminal->setContainerContext(catalog, container, shell, &error)) {
        dialog->deleteLater();
        statusBar()->showMessage(error, 10000);
        return;
    }
    layout->addWidget(terminal);
    dialog->show();
}
