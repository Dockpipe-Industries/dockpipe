#include "MainWindow.h"
#include "RemoteBrowserWidget.h"

#include "DashboardPage.h"
#include "DashboardWidgetFactory.h"
#include "DockerObservabilityWidget.h"
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
#include <QStatusBar>
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
        updateWorkspaceMachineAccess();
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
        menu->setAccessUnlocked(unlocked);
        if (!menu->property("remoteAccessConnected").toBool()) {
            connect(menu, &WorkspaceMachineMenu::unlockRequested, m_remoteAccess, &RemoteAccessMonitor::unlock);
            menu->setProperty("remoteAccessConnected", true);
        }
    }
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
        if (m_browserWidget)
            m_browserWidget->setMachineCatalog(catalog, m_remoteAccess->state() == RemoteAccessMonitor::State::Unlocked);
        const auto selectedCatalog = currentSharedMachineCatalog();
        m_dockerWidget->setFeatureCatalog(selectedCatalog);
        m_dockerWidget->setEnabled(m_workspaceMachineMenu->currentTarget().isEmpty()
            || selectedCatalog.value("docker_read").toBool());
        if (m_terminalWidget)
            m_terminalWidget->updateMachineCatalog(currentSharedMachineCatalog());
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
    m_runTarget->addItem(m_workspaceMachineMenu->text());
    m_pickerTarget->setCurrentIndex(0);
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
    }
    const bool filesAccepted = WorkspaceFilesController::configure(m_projectFiles, files,
        target.isEmpty() ? WorkspaceFilesClient::RemoteDispatch{} : RemoteWorkspaceFilesDispatch::create(m_projectFiles));
    const bool gitAccepted = target.isEmpty()
        ? RemoteGitContext::configureLocal(m_projectGit, m_settings.projectFolder, &error)
        : RemoteGitContext::configure(m_projectGit, target, root, &error);
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
        m_terminalWidget->setMachineContext(catalog, root);
    }
    m_workspaceMachineMenu->setCurrent(target, root);
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
