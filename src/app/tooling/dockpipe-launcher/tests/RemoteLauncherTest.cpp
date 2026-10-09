#include "MainWindow.h"
#include "BasicModeWidget.h"
#include "AppCardWidget.h"
#include "DockerObservabilityWidget.h"
#include "RemoteWidget.h"
#include "RemoteRunDialog.h"
#include "LauncherSettings.h"
#include "Theme.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QStyle>
#include <QTableWidget>
#include <QTabWidget>
#include <QStackedWidget>
#include <QTemporaryDir>
#include <QThread>
#include <functional>
#include <cstdio>

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 5000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

static QPushButton *button(QWidget *parent, const QString &text)
{
    for (auto *candidate : parent->findChildren<QPushButton *>()) {
        if (candidate->text() == text)
            return candidate;
    }
    return nullptr;
}

int main(int argc, char **argv)
{
    QTemporaryDir directory;
    qputenv("XDG_CONFIG_HOME", directory.path().toUtf8());
    qputenv("DOCKPIPE_GLOBAL_ROOT", directory.filePath("global").toUtf8());
    qputenv("DOCKPIPE_TEST_ROOT", directory.path().toUtf8());
    const QString cli = directory.filePath("dockpipe");
    QFile script(cli);
    if (!script.open(QIODevice::WriteOnly))
        return 1;
    script.write(R"SH(#!/bin/sh
case "$1:$2" in
catalog:list)
  printf '{"workflows":[{"workflow_id":"build","display_name":"Build project","description":"Compile and verify the project","config_path":"%s/work/config.yml"},{"workflow_id":"editor","display_name":"Project editor","category":"app"}],"runtimes":["host"],"resolver_details":[{"name":"example.tunnel","title":"Example Tunnel","version":"1.2.0","capability":"remote.edge","remote_setup":"local"},{"name":"example.hosted","title":"Example Hosted","version":"2.0.0","capability":"remote.broker","remote_setup":"hosted"},{"name":"unrelated.tool","capability":"editor"}]}\n' "$DOCKPIPE_TEST_ROOT" ;;
remote:info) echo '{"broker":{"mode":"local","endpoint":"https://remote.example.com","resolver":{"name":"example.tunnel","title":"Example Tunnel","version":"1.2.0"}}}' ;;
remote:nodes) echo '[{"node":"mac-mini","status":"paired"},{"node":"studio-invite","status":"invited"}]' ;;
remote:pairings) echo '[{"node":"studio","code":"ABCD-1234-5678","status":"pending","expires_at":"2026-10-09T07:00:00Z"}]' ;;
remote:pair)
  echo '{"node":"studio","code":"ABCD-1234-5678","status":"pending"}'
  exec sleep 60 ;;
remote:submit)
  printf '%s\n' "$@" > "$DOCKPIPE_TEST_ROOT/submit-args"
  echo '{"submission":{"bundle_hash":"abc123"},"files":["work/config.yml"],"bytes":100}' ;;
*) echo '{}' ;;
esac
)SH");
    script.close();
    script.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    qputenv("DOCKPIPE_BIN", cli.toUtf8());
    QFile docker(directory.filePath("docker"));
    if (!docker.open(QIODevice::WriteOnly))
        return 18;
    docker.write(R"SH(#!/bin/sh
case "$1:$2" in
container:ls) echo '{"ID":"fixture","Names":"pipeon","State":"running","Status":"Up 2 hours","Image":"example/pipeon:test","Ports":"127.0.0.1:8080->8080/tcp"}' ;;
network:ls) echo '{"ID":"network","Name":"workspace","Driver":"bridge","Scope":"local"}' ;;
volume:ls) ;;
*) echo 'Unexpected Docker fixture command' >&2; exit 1 ;;
esac
)SH");
    docker.close();
    docker.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    qputenv("PATH", directory.path().toUtf8() + ":" + qgetenv("PATH"));
    QApplication app(argc, argv);
    QApplication::setApplicationName("dockpipe-launcher-test");
    applyDockpipeLauncherTheme(app);
    if (qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light") {
        QFile shared(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!shared.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 28;
        app.setStyleSheet(QString());
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
    }
    LauncherSettings settings;
    settings.load();
    settings.projectFolder = directory.path();
    settings.uiMode = "advanced";
    settings.thirdPartyDisclaimerDismissed = true;
    settings.save();
    MainWindow window;
    window.show();
    auto *navigation = window.findChild<QListWidget *>("workspaceNavigation");
    auto *target = window.findChild<QComboBox *>("runTarget");
    auto *remote = window.findChild<RemoteWidget *>();
    if (!navigation || !target || !remote || !waitUntil([&]() { return navigation->currentRow() == 1; }))
        return 2;
    auto *apps = window.findChild<BasicModeWidget *>();
    if (!apps || !apps->findChildren<QTabWidget *>().isEmpty()
        || window.findChildren<DockerObservabilityWidget *>().size() != 1
        || navigation->count() != 5 || navigation->item(4)->text() != "Docker")
        return 13;
    navigation->setCurrentRow(0);
    if (!waitUntil([&]() { return button(apps, "Open app") != nullptr; }))
        return 14;
    const auto capture = [&](const QString &suffix) {
        QApplication::processEvents();
        if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
            window.grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + suffix + ".png");
    };
    // Exercise the library with a representative fixture, without launching programs.
    QVector<WorkflowMeta> library;
    const QStringList names = {"Pipeon", "Project editor", "Linux workspace", "Developer console"};
    for (const QString &name : names) {
        WorkflowMeta meta;
        meta.workflowId = name.toLower().replace(' ', '-');
        meta.displayName = name;
        meta.description = "Your tools and project, together in an isolated workspace.";
        library.append(meta);
    }
    apps->setApps(library);
    apps->setRunningByWorkflow({{"pipeon", true}});
    QString launched;
    QString configured;
    QObject::connect(apps, &BasicModeWidget::launchRequested, [&](const QString &id) { launched = id; });
    QObject::connect(apps, &BasicModeWidget::configureRequested, [&](const QString &id) { configured = id; });
    // Disconnect the shell's execution handlers; these assertions must never start real work.
    QObject::disconnect(apps, &BasicModeWidget::launchRequested, &window, nullptr);
    QObject::disconnect(apps, &BasicModeWidget::configureRequested, &window, nullptr);
    auto *appList = apps->findChild<QListWidget *>("basicAppList");
    auto *firstCard = qobject_cast<AppCardWidget *>(appList->itemWidget(appList->item(0)));
    button(firstCard, "Open app")->click();
    button(firstCard, "Configure")->click();
    if (launched != "pipeon" || configured != "pipeon")
        return 15;
    capture("-apps");
    {
        const QColor headingColor = window.findChild<QLabel *>("appTitle")->palette().color(QPalette::WindowText);
        const auto palette = app.palette();
        const QString stylesheet = app.styleSheet();
        QFile shared(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!shared.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 19;
        app.setStyleSheet(QString());
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
        capture("-apps-light");
        app.setStyleSheet(QString());
        app.setPalette(palette);
        app.setStyleSheet(stylesheet);
        app.setPalette(palette);
        QApplication::processEvents();
        if (window.findChild<QLabel *>("appTitle")->palette().color(QPalette::WindowText) != headingColor)
            return 21;
    }
    window.resize(1920, 1080);
    capture("-apps-wide");
    window.resize(1180, 780);
    apps->setViewIconMode(false);
    capture("-apps-compact");
    apps->setViewIconMode(true);
    apps->setApps({});
    apps->setAppDiscoveryLoading(false);
    int packageRequests = 0;
    QObject::disconnect(apps, &BasicModeWidget::packagesRequested, &window, nullptr);
    QObject::connect(apps, &BasicModeWidget::packagesRequested, [&]() { ++packageRequests; });
    auto *findApp = button(apps, "Find an app");
    if (!findApp || !findApp->isVisible() || appList->isVisible())
        return 16;
    findApp->click();
    if (packageRequests != 1)
        return 17;
    capture("-apps-empty");
    navigation->setCurrentRow(4);
    auto *dockerPage = window.findChild<DockerObservabilityWidget *>();
    if (!waitUntil([&]() {
        for (auto *table : dockerPage->findChildren<QTableWidget *>()) {
            if (table->rowCount() > 0)
                return true;
        }
        return false;
    }))
        return 20;
    capture("-docker");
    navigation->setCurrentRow(2);
    if (!waitUntil([&]() { return target->findData("mac-mini") >= 0 && button(remote, "Refresh")->isEnabled(); })) {
        std::fprintf(stderr, "Machine discovery failed\n");
        return 3;
    }
    // All screenshots use isolated fixture data, never a live account or broker.
    if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
        window.grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-machines.png");
    auto *machinePages = remote->findChild<QStackedWidget *>("machinePages");
    auto *machineDetails = remote->findChild<QPlainTextEdit *>("machineDetails");
    auto *machines = remote->findChild<QTableWidget *>("managedMachines");
    if (!machinePages || machinePages->currentIndex() != 0 || machineDetails->isVisible()
        || machines->item(1, 1)->text() != "Invitation created"
        || !machines->item(1, 2)->text().startsWith("Not connected yet"))
        return 22;
    window.resize(1800, 1800);
    capture("-machines-tall");
    if (machines->mapTo(&window, QPoint()).y() > 500)
        return 23;
    window.resize(1180, 780);
    auto *summary = remote->findChild<QLabel *>("remoteConnectionSummary");
    if (!summary || !summary->text().contains("example.tunnel")
        || !summary->text().contains("1.2.0") || !summary->text().contains("https://remote.example.com"))
        return 33;
    remote->findChild<QPushButton *>("remoteSetupNavigation")->click();
    auto *resolver = remote->findChild<QComboBox *>("remoteResolver");
    auto *hostname = remote->findChild<QLineEdit *>("remoteHostname");
    auto *setupHint = remote->findChild<QLabel *>("remoteSetupHint");
    if (!resolver || resolver->count() != 2 || !hostname->isVisible()) {
        std::fprintf(stderr, "Provider setup: count=%d, hostnameVisible=%d, page=%d\n",
                     resolver ? resolver->count() : -1, hostname->isVisible(), machinePages->currentIndex());
        return 34;
    }
    capture("-setup-local");
    resolver->setCurrentIndex(1);
    if (hostname->isVisible() || !button(remote, "Sign in and connect…")->isEnabled()
        || !setupHint->text().contains("Hosted broker") || !summary->text().contains("example.tunnel"))
        return 35;
    capture("-setup-hosted");
    resolver->setCurrentIndex(0);
    if (!hostname->isVisible() || !setupHint->text().contains("Broker on this computer"))
        return 36;
    for (auto *back : remote->findChildren<QPushButton *>()) {
        if (back->text() == "← Machines" && back->isVisible()) {
            back->click();
            break;
        }
    }
    button(remote, "Add another machine")->click();
    auto *requests = remote->findChild<QTableWidget *>("pairingRequests");
    if (!waitUntil([&]() { return requests->rowCount() == 1 && button(remote, "Check for requests")->isEnabled(); }))
        return 24;
    if (button(remote, "Approve matching code")->isEnabled())
        return 25;
    requests->selectRow(0);
    if (!button(remote, "Approve matching code")->isEnabled())
        return 26;
    capture("-add-machine");
    // Return through the visible navigation, then exercise the worker-side path.
    for (auto *back : remote->findChildren<QPushButton *>()) {
        if (back->text() == "← Machines" && back->isVisible()) {
            back->click();
            break;
        }
    }
    button(remote, "Connect this computer")->click();
    if (button(remote, "Request pairing")->isEnabled())
        return 27;
    for (auto *edit : remote->findChildren<QLineEdit *>()) {
        if (edit->placeholderText().startsWith("https://"))
            edit->setText("https://broker.example.com");
    }
    remote->findChild<QCheckBox *>()->setChecked(true);
    auto *pair = button(remote, "Request pairing");
    auto *cancel = button(remote, "Cancel current operation");
    if (!waitUntil([&]() { return pair->isEnabled(); }))
        return 12;
    pair->click();
    auto *code = remote->findChild<QLabel *>("pairingCode");
    if (!waitUntil([&]() { return code->text() == "ABCD-1234-5678" && cancel->isEnabled(); }))
        return 4;
    if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
        window.grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-pairing.png");
    cancel->click();
    if (!waitUntil([&]() { return pair->isEnabled() && !cancel->isEnabled(); })) {
        std::fprintf(stderr, "Cancellation left pairing controls busy\n");
        return 5;
    }
    navigation->setCurrentRow(1);
    target->setCurrentIndex(target->findData("mac-mini"));
    QApplication::processEvents();
    if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
        window.grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-workflows.png");
    Context context = Context::createNew();
    context.label = "Build project";
    context.workdir = directory.path();
    context.workflowFile = directory.filePath("work/config.yml");
    RemoteRunDialog dialog(context, "mac-mini");
    dialog.show();
    auto *send = button(&dialog, "Run on mac-mini");
    if (!send || send->isEnabled())
        return 6;
    button(&dialog, "Preview files")->click();
    if (!waitUntil([&]() { return send->isEnabled(); }))
        return 7;
    auto editors = dialog.findChildren<QPlainTextEdit *>();
    for (auto *editor : editors) {
        if (!editor->isReadOnly()) {
            editor->setPlainText("extra-source");
            break;
        }
    }
    if (send->isEnabled()) {
        std::fprintf(stderr, "Editing source selection did not invalidate preview\n");
        return 8;
    }
    button(&dialog, "Preview files")->click();
    if (!waitUntil([&]() { return send->isEnabled(); }))
        return 9;
    send->click();
    if (!waitUntil([&]() { return dialog.result() == QDialog::Accepted; }))
        return 10;
    QFile submitted(directory.filePath("submit-args"));
    if (!submitted.open(QIODevice::ReadOnly) || !submitted.readAll().contains("--expected-digest\nabc123\n"))
        return 11;
    return 0;
}
