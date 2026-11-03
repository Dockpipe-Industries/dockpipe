#include "MainWindow.h"

#include "DashboardWidgetFactory.h"
#include "DockerObservabilityWidget.h"
#include "ExecutionExplorer.h"
#include "SshMachineConnectionDialog.h"
#include "SshMachineConnectionStore.h"
#include "TerminalHub.h"
#include "TerminalWidget.h"

#include <QDir>
#include <QFileInfo>
#include <QMessageBox>
#include <QStackedWidget>
#include <QTimer>

void MainWindow::loadHostConnection()
{
    m_hostConnectionState = qEnvironmentVariable("DOCKPIPE_MACHINE_CONNECTION_STATE");
    if (m_hostConnectionState.isEmpty())
        m_hostConnectionState = QDir::homePath() + QStringLiteral("/.dockpipe-ssh");

    const QString storePath = m_hostConnectionState + QStringLiteral("/connection.json");
    const bool required = qEnvironmentVariableIntValue("DOCKPIPE_MACHINE_CONNECTION_REQUIRED") == 1;
    const bool stored = QFileInfo::exists(storePath) || QFileInfo(storePath).isSymLink();
    if (!required && !stored)
        return;

    QString error;
    if (!stored || !SshMachineConnectionStore(storePath).load(&m_hostConnection, &error)) {
        // An unavailable required host stays selected. Container data is only an
        // explicit local choice, never a substitute for an unsuccessful SSH host.
        m_hostConnection.id = QStringLiteral("host");
        m_hostConnection.label = qEnvironmentVariable("DOCKPIPE_DEFAULT_MACHINE_LABEL", "Host");
        m_hostConnection.host = qEnvironmentVariable("DOCKPIPE_DEFAULT_MACHINE_HOST");
        m_hostConnection.nativeCliPath = qEnvironmentVariable("DOCKPIPE_NATIVE_CLI_PATH");
        applyHostConnection();
        QTimer::singleShot(0, this, [this, error]() {
            if (!error.isEmpty())
                QMessageBox::warning(this, tr("Host connection unavailable"), error);
            onHostConnection();
        });
        return;
    }
    applyHostConnection();
}

void MainWindow::applyHostConnection()
{
    if (m_executionExplorer)
        m_executionExplorer->setSshMachineConnection(m_hostConnection, m_hostConnectionState);
    if (m_dockerWidget)
        m_dockerWidget->setSshMachineConnection(m_hostConnection, m_hostConnectionState);
    if (m_dashboardFactory)
        m_dashboardFactory->terminals().setSshMachineConnection(m_hostConnection, m_hostConnectionState);
}

void MainWindow::onHostConnection()
{
    if (m_hostConnectionState.isEmpty()) {
        m_hostConnectionState = qEnvironmentVariable("DOCKPIPE_MACHINE_CONNECTION_STATE");
        if (m_hostConnectionState.isEmpty())
            m_hostConnectionState = QDir::homePath() + QStringLiteral("/.dockpipe-ssh");
    }
    SshMachineConnection initial = m_hostConnection;
    if (initial.nativeCliPath.isEmpty())
        initial.nativeCliPath = qEnvironmentVariable("DOCKPIPE_NATIVE_CLI_PATH");
    SshMachineConnectionDialog dialog(initial, this);
    if (dialog.exec() != QDialog::Accepted)
        return;

    const SshMachineConnection connection = dialog.connection();
    QString error;
    if (!SshMachineConnectionStore(m_hostConnectionState + QStringLiteral("/connection.json"))
             .save(connection, &error)) {
        QMessageBox::warning(this, tr("Host connection not saved"), error);
        return;
    }
    m_hostConnection = connection;
    applyHostConnection();
    onOpenHostTerminal();
}

void MainWindow::onOpenHostTerminal()
{
    if (!m_hostConnection.validationError().isEmpty()) {
        onHostConnection();
        return;
    }
    ensureTerminal();
    if (!m_terminalWidget)
        return;
    selectPage(m_stack->indexOf(m_terminalWidget));
    m_terminalWidget->openShell(QStringLiteral("ssh-machine:") + m_hostConnection.id);
}
