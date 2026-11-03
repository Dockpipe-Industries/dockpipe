#include "TerminalHub.h"
#include "SshMachineConnectionDialog.h"

#include <QDialogButtonBox>
#include <QDir>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QScopeGuard>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

namespace {
SshMachineConnection exampleConnection()
{
    SshMachineConnection connection;
    connection.id = "host";
    connection.label = "Office host";
    connection.host = "192.0.2.7";
    connection.user = "operator";
    connection.nativeCliPath = "/opt/dockpipe/bin/dockpipe";
    return connection;
}
}

class TerminalHubSshTest : public QObject {
    Q_OBJECT

private slots:
    void sshLaunchUsesHostIdentityAndLocalHome();
    void explicitLocalProfileRemainsLocal();
    void invalidConnectionDoesNotCreateLocalSession();
    void connectionDialogValidatesMetadataOnly();
};

void TerminalHubSshTest::sshLaunchUsesHostIdentityAndLocalHome()
{
#if defined(Q_OS_LINUX) || defined(Q_OS_MACOS)
    QTemporaryDir state("/tmp/dp-hub-XXXXXX");
    TerminalHub hub(TerminalLaunchContext::Local);
    QSignalSpy changes(&hub, &TerminalHub::shellProfilesChanged);
    hub.setSshMachineConnection(exampleConnection(), state.path());
    QCOMPARE(changes.count(), 1);
    QVERIFY(hub.hasSshMachineConnection());
    QCOMPARE(hub.shellProfiles().first().id, QString("ssh-machine:host"));
    const auto launch = hub.prepareSessionLaunch("/nonexistent/container/workdir");
    QVERIFY2(launch.valid(), qPrintable(launch.error));
    QVERIFY(launch.remote);
    QVERIFY(launch.program.endsWith("/ssh"));
    QCOMPARE(launch.workingDirectory, QDir::homePath());
    QCOMPARE(launch.context, TerminalLaunchContext::Local);
    QCOMPARE(launch.arguments.last(), QString("192.0.2.7"));
    QVERIFY(launch.arguments.contains("-tt"));
    QVERIFY(launch.arguments.contains("StrictHostKeyChecking=ask"));
    QVERIFY(!launch.arguments.contains("/nonexistent/container/workdir"));
    QVERIFY(launch.label.contains("operator@192.0.2.7"));
    const auto explicitRemote = hub.prepareSessionLaunch("/another/container/path", "ssh-machine:host");
    QCOMPARE(explicitRemote.program, launch.program);
    QCOMPARE(explicitRemote.arguments, launch.arguments);
    hub.clearSshMachineConnection();
    QVERIFY(!hub.hasSshMachineConnection());
    QCOMPARE(changes.count(), 2);
    QVERIFY(!hub.prepareSessionLaunch({}, "ssh-machine:host").valid());
#else
    QSKIP("Shared SSH sessions require Linux or macOS");
#endif
}

void TerminalHubSshTest::explicitLocalProfileRemainsLocal()
{
    const bool hadLabel = qEnvironmentVariableIsSet("DOCKPIPE_LOCAL_MACHINE_LABEL");
    const QByteArray previousLabel = qgetenv("DOCKPIPE_LOCAL_MACHINE_LABEL");
    const auto restoreLabel = qScopeGuard([hadLabel, previousLabel] {
        if (hadLabel)
            qputenv("DOCKPIPE_LOCAL_MACHINE_LABEL", previousLabel);
        else
            qunsetenv("DOCKPIPE_LOCAL_MACHINE_LABEL");
    });
    qputenv("DOCKPIPE_LOCAL_MACHINE_LABEL", "GUI container");
    TerminalHub hub(TerminalLaunchContext::Local);
    const auto local = hub.shellCatalog().profiles();
    QVERIFY(!local.isEmpty());
    QTemporaryDir state;
    hub.setSshMachineConnection(exampleConnection(), state.path());
    const auto launch = hub.prepareSessionLaunch("/explicit/local/workdir", local.first().id);
    QVERIFY(launch.valid());
    QVERIFY(!launch.remote);
    QCOMPARE(launch.program, local.first().program);
    QCOMPARE(launch.workingDirectory, QString("/explicit/local/workdir"));
    auto expectedArguments = local.first().arguments;
    if (!local.first().workingDirectoryArgument.isEmpty()) {
        expectedArguments.prepend(QDir::toNativeSeparators("/explicit/local/workdir"));
        expectedArguments.prepend(local.first().workingDirectoryArgument);
    }
    QCOMPARE(launch.arguments, expectedArguments);
    QVERIFY(launch.label.contains(local.first().label));
    QVERIFY(launch.label.startsWith("GUI container · "));
    hub.clearSshMachineConnection();
    const auto ordinary = hub.prepareSessionLaunch("/ordinary/local/workdir");
    QVERIFY(ordinary.valid());
    QVERIFY(!ordinary.remote);
    QCOMPARE(ordinary.workingDirectory, QString("/ordinary/local/workdir"));
}

void TerminalHubSshTest::invalidConnectionDoesNotCreateLocalSession()
{
    TerminalHub hub(TerminalLaunchContext::Local);
    QTemporaryDir state;
    auto invalid = exampleConnection();
    invalid.host = "-invalid";
    hub.setSshMachineConnection(invalid, state.path());
    QVERIFY(hub.hasSshMachineConnection());
    const auto launch = hub.prepareSessionLaunch(QDir::homePath());
    QVERIFY(!launch.valid());
    QVERIFY(launch.program.isEmpty());
    QVERIFY(hub.createSession(QDir::homePath()).isEmpty());
    QVERIFY(!hub.creationError().isEmpty());
    QVERIFY(hub.sessionIds().isEmpty());
    QVERIFY(!hub.prepareSessionLaunch({}, "ssh-machine:another-host").valid());
}

void TerminalHubSshTest::connectionDialogValidatesMetadataOnly()
{
    SshMachineConnectionDialog dialog(exampleConnection());
    QCOMPARE(dialog.connection().host, QString("192.0.2.7"));
    const auto fields = dialog.findChildren<QLineEdit *>();
    QVERIFY(!fields.isEmpty());
    for (const auto *field : fields)
        QCOMPARE(field->echoMode(), QLineEdit::Normal);
    auto *host = dialog.findChild<QLineEdit *>("sshMachineHost");
    auto *buttons = dialog.findChild<QDialogButtonBox *>();
    QVERIFY(host);
    QVERIFY(buttons);
    host->setText("-invalid");
    buttons->button(QDialogButtonBox::Save)->click();
    QCOMPARE(dialog.result(), int(QDialog::Rejected));
    QVERIFY(!dialog.findChild<QLabel *>("sshMachineError")->text().isEmpty());
    host->setText("host.example");
    buttons->button(QDialogButtonBox::Save)->click();
    QCOMPARE(dialog.result(), int(QDialog::Accepted));
    QCOMPARE(dialog.connection().host, QString("host.example"));
}

QTEST_MAIN(TerminalHubSshTest)
#include "TerminalHubSshTest.moc"
