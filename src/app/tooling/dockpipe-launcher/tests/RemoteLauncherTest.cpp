#include "MainWindow.h"
#include "BasicModeWidget.h"
#include "AppCardWidget.h"
#include "DockerObservabilityWidget.h"
#include "RemoteWidget.h"
#include "RemoteRunDialog.h"
#include "ActivityWidget.h"
#include "PackageManagerDialog.h"
#include "LauncherSettings.h"
#include "Theme.h"
#include "ResourceLinkItem.h"
#include <QPainter>

#include <QApplication>
#include <QGraphicsView>
#include <QGraphicsObject>
#include <QPointer>
#include <QGraphicsScene>
#include <QGraphicsPathItem>
#include <QCheckBox>
#include <QComboBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QMenu>
#include <QMenuBar>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QStyle>
#include <QTableWidget>
#include <QTabWidget>
#include <QStackedWidget>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <QToolButton>
#include <functional>
#include <cstdio>

int verifySystemMonitoring(const QString &directory);

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

// Render fixed phases without sleeping or depending on the event-loop timer.
bool connectionFramesChange()
{
    QGraphicsScene scene;
    auto *link = new ResourceLinkItem;
    QPainterPath path(QPointF(10, 20));
    path.cubicTo(QPointF(100, 20), QPointF(180, 80), QPointF(290, 80));
    link->setPath(path);
    link->setFlowPath(path);
    link->setPen(QPen(QColor("#798ba8"), 1.6, Qt::DashLine));
    link->setFlowColor(QColor("#a4c7f2"));
    scene.addItem(link);
    const auto render = [&](qreal zoom = 1.0) {
        QImage image(320, 110, QImage::Format_ARGB32_Premultiplied);
        image.fill(QColor("#10151c"));
        QPainter painter(&image);
        painter.scale(zoom, zoom);
        scene.render(&painter, QRectF(0, 0, 320, 110), QRectF(0, 0, 320, 110));
        return image;
    };
    link->setMotionEnabled(true);
    link->setPhase(0);
    const QImage first = render();
    link->setPhase(8);
    if (first == render())
        return false;
    const QImage overview = render(0.25);
    link->setPhase(0);
    if (overview != render(0.25))
        return false;
    link->setMotionEnabled(false);
    const QImage paused = render();
    link->setPhase(8);
    return paused == render();
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
remote:info)
  if [ -f "$DOCKPIPE_TEST_ROOT/stale-service" ]; then
    echo '{"broker":{"mode":"local","endpoint":"https://remote.example.com","service":{"state":"update-required"}}}'
  elif [ -f "$DOCKPIPE_TEST_ROOT/worker-connection" ]; then
    echo '{"worker":{"endpoint":"https://remote.example.com","node":"mac-mini"}}'
  elif [ -f "$DOCKPIPE_TEST_ROOT/legacy-connection" ]; then
    echo '{"broker":{"mode":"local","endpoint":"https://remote.example.com"}}'
  else
    echo '{"broker":{"mode":"local","endpoint":"https://remote.example.com","resolver":{"name":"example.tunnel","title":"Example Tunnel","version":"1.2.0"}}}'
  fi ;;
remote:jobs)
  if [ -f "$DOCKPIPE_TEST_ROOT/jobs-fail" ]; then
    echo 'Could not reach fixture broker' >&2
    exit 1
  fi
  echo called >> "$DOCKPIPE_TEST_ROOT/jobs-calls"
  echo '[{"id":"build-1","node":"mac-mini","status":"completed"}]' ;;
remote:cancel)
  printf '%s\n' "$@" > "$DOCKPIPE_TEST_ROOT/cancel-job-args"
  echo '{}' ;;
remote:nodes) echo '[{"node":"mac-mini","status":"paired"},{"node":"studio-invite","status":"invited"}]' ;;
remote:observe)
  if [ -f "$DOCKPIPE_TEST_ROOT/monitor-delay" ]; then sleep 0.2; fi
  cat "$DOCKPIPE_TEST_ROOT/monitor.json" ;;
remote:pairings) echo '[{"node":"studio","code":"ABCD-1234-5678","status":"pending","expires_at":"2026-10-09T07:00:00Z"}]' ;;
remote:pair)
  echo called >> "$DOCKPIPE_TEST_ROOT/pair-calls"
  if [ -f "$DOCKPIPE_TEST_ROOT/pair-approved" ]; then
    exit 0
  fi
  echo '{"node":"studio","code":"ABCD-1234-5678","status":"pending"}'
  exec sleep 60 ;;
remote:service)
  printf '%s\n' "$@" >> "$DOCKPIPE_TEST_ROOT/service-calls"
  if [ -f "$DOCKPIPE_TEST_ROOT/service-fails" ]; then
    echo 'Service fixture failed' >&2
    exit 1
  fi
  if [ "$5" = "--replace" ]; then
    rm -f "$DOCKPIPE_TEST_ROOT/stale-service"
  fi ;;
remote:submit)
  printf '%s\n' "$@" > "$DOCKPIPE_TEST_ROOT/submit-args"
  echo '{"submission":{"bundle_hash":"abc123"},"files":["work/config.yml"],"bytes":100}' ;;
package:list) echo '{"packages":[],"warnings":[],"install_root":"/example/packages"}' ;;
package:catalog) echo '{"manifest":"https://example.com/pinned/store.json","platform":"test-platform","packages":[{"name":"example.hosted","kind":"resolver","capability":"remote.broker","version":"2.0.0","sha256":"provider-digest","size_bytes":1572864}]}' ;;
package:install)
  printf '%s\n' "$@" > "$DOCKPIPE_TEST_ROOT/provider-install-args"
  echo '{"path":"/example/packages/provider.tar.gz"}' ;;
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
container:ls)
    echo '{"ID":"fixture","Names":"pipeon","State":"running","Status":"Up 2 hours","Image":"example/pipeon:test","Ports":"127.0.0.1:8080->8080/tcp"}'
    echo '{"ID":"database","Names":"database","State":"running","Status":"Up 2 hours (unhealthy)","Image":"postgres:test"}'
    echo '{"ID":"stopped","Names":"old-worker","State":"exited","Status":"Exited (0) 2 hours ago","Image":"example/worker:test"}' ;;
container:inspect)
    echo '{"Id":"fixture","Name":"/pipeon","Labels":{"com.docker.compose.project":"demo","com.docker.compose.service":"web","com.docker.compose.depends_on":"db:service_healthy:false"},"Networks":{"workspace":{}},"Mounts":[{"Type":"volume","Name":"data","Destination":"/data"}]}'
    echo '{"Id":"database","Name":"/database","Labels":{"com.docker.compose.project":"demo","com.docker.compose.service":"db"},"Networks":{"workspace":{}},"Mounts":[]}'
    echo '{"Id":"stopped","Name":"/old-worker","Labels":{},"Networks":{"workspace":{}},"Mounts":[]}' ;;
inspect:*) echo '[{"Id":"fixture","Name":"/pipeon"}]' ;;
start:*) echo 'Fixture start rejected' >&2; exit 1 ;;
logs:*) echo 'Fixture logs ready'; echo 'Fixture error stream' >&2 ;;
network:inspect) echo '[{"Name":"workspace","Driver":"bridge"}]' ;;
network:ls) echo '{"ID":"network","Name":"workspace","Driver":"bridge","Scope":"local"}' ;;
volume:ls) echo '{"Name":"data","Driver":"local"}' ;;
volume:inspect) echo '[{"Name":"data","Mountpoint":"/fixture/data"}]' ;;
*) echo 'Unexpected Docker fixture command' >&2; exit 1 ;;
esac
)SH");
    docker.close();
    docker.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    QFile terminal(directory.filePath("xdg-terminal-exec"));
    if (!terminal.open(QIODevice::WriteOnly))
        return 58;
    terminal.write("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$DOCKPIPE_TEST_ROOT/exec-terminal-args\"\n");
    terminal.close();
    terminal.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    // Exercise Flatpak's host-terminal routing without contacting the real host
    // service. The forwarded command still runs the isolated terminal fixture.
    QFile terminalBridge(directory.filePath("flatpak-spawn"));
    if (!terminalBridge.open(QIODevice::WriteOnly))
        return 70;
    terminalBridge.write(R"SH(#!/bin/sh
if [ "$1" != "--host" ]; then exit 1; fi
printf '%s\n' "$@" > "$DOCKPIPE_TEST_ROOT/terminal-bridge-args"
shift
exec "$@"
)SH");
    terminalBridge.close();
    terminalBridge.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    qputenv("PATH", directory.path().toUtf8() + ":" + qgetenv("PATH"));
    QApplication app(argc, argv);
    if (!connectionFramesChange())
        return 55;
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
        || navigation->count() != 5 || navigation->item(3)->text() != "Containers"
        || navigation->item(4)->text() != "Executions")
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
    navigation->setCurrentRow(3);
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
    auto *graph = dockerPage->findChild<QGraphicsView *>("dockerGraph");
    int dependencies = 0;
    int attachments = 0;
    QGraphicsItem *containerCard = nullptr;
    for (auto *item : graph->scene()->items()) {
        if (item->toolTip().startsWith("Depends on db"))
            ++dependencies;
        if (item->toolTip().startsWith("Network attachment") || item->toolTip().startsWith("Mounted at"))
            ++attachments;
        if (item->data(0).toString() == "container:fixture")
            containerCard = item;
    }
    if (dependencies != 1 || attachments != 4 || !containerCard)
        return 40;
    containerCard->setSelected(true);
    if (!dockerPage->findChild<QWidget *>("dockerDetailsPanel")->isVisible())
        return 41;
    auto *logs = dockerPage->findChild<QPlainTextEdit *>("dockerSelectedLogs");
    if (!waitUntil([&]() { return logs->toPlainText().contains("Fixture logs ready")
        && logs->toPlainText().contains("Fixture error stream"); }))
        return 59;
    for (auto *table : dockerPage->findChildren<QTableWidget *>()) {
        if (table->isVisible())
            return 60;
    }
    capture("-docker-details");
    auto *related = dockerPage->findChild<QLabel *>("dockerRelatedResources");
    if (!related->text().contains("network%3Anetwork") || !related->text().contains("volume%3Adata"))
        return 61;
    QMetaObject::invokeMethod(related, "linkActivated", Q_ARG(QString, "network%3Anetwork"));
    auto *resourceTitle = dockerPage->findChild<QLabel *>("dockerResourceTitle");
    if (resourceTitle->text() != "workspace")
        return 62;
    capture("-docker-network-inspector");
    QMetaObject::invokeMethod(related, "linkActivated", Q_ARG(QString, "container%3Afixture"));
    if (resourceTitle->text() != "pipeon")
        return 63;
    auto *exec = dockerPage->findChild<QPushButton *>("dockerExec");
    if (!exec->isEnabled())
        return 64;
    exec->menu()->actions().first()->trigger();
    const QString terminalArgs = directory.filePath("exec-terminal-args");
    if (!waitUntil([&]() { return QFile::exists(terminalArgs); })) {
        std::fprintf(stderr, "Container shell did not reach the terminal fixture\n");
        return 65;
    }
    QFile arguments(terminalArgs);
    if (!arguments.open(QIODevice::ReadOnly)
        || !arguments.readAll().endsWith("docker\nexec\n-it\nfixture\n/bin/sh\n"))
        return 66;
    arguments.close();
    if (QFile::exists("/.flatpak-info")) {
        QFile bridgeArguments(directory.filePath("terminal-bridge-args"));
        if (!bridgeArguments.open(QIODevice::ReadOnly)
            || !bridgeArguments.readAll().startsWith("--host\nsh\n-c\n"))
            return 71;
    }
    auto *networksToggle = dockerPage->findChild<QPushButton *>("dockerFilter_network");
    auto *volumesToggle = dockerPage->findChild<QPushButton *>("dockerFilter_volume");
    auto *stoppedToggle = dockerPage->findChild<QPushButton *>("dockerFilter_inactive");
    auto *dependenciesToggle = dockerPage->findChild<QPushButton *>("dockerFilter_dependencies");
    const auto resource = [&](const QString &id) -> QGraphicsItem * {
        for (auto *item : graph->scene()->items()) {
            if (item->data(0).toString() == id)
                return item;
        }
        return nullptr;
    };
    auto *stoppedCard = resource("container:stopped");
    if (!stoppedCard || !stoppedCard->data(4).toBool()
        || !(stoppedCard->flags() & QGraphicsItem::ItemIsSelectable)
        || !stoppedCard->toolTip().contains("Stopped")
        || !resource("container:fixture")->toolTip().contains("Running")
        || !resource("container:database")->toolTip().contains("Unhealthy"))
        return 43;
    QMetaObject::invokeMethod(related, "linkActivated", Q_ARG(QString, "container%3Astopped"));
    if (exec->isEnabled() || resourceTitle->text() != "old-worker")
        return 67;
    auto *start = dockerPage->findChild<QPushButton *>("dockerStart");
    start->click();
    if (start->isEnabled())
        return 68;
    if (!waitUntil([&]() { return dockerPage->findChild<QLabel *>("dockerInspectorStatus")->text()
        .contains("Fixture start rejected") && start->isEnabled(); }))
        return 69;
    QMetaObject::invokeMethod(related, "linkActivated", Q_ARG(QString, "container%3Afixture"));
    auto *motion = dockerPage->findChild<QPushButton *>("dockerFilter_motion");
    auto *animation = dockerPage->findChild<QTimer *>("dependencyAnimationTimer");
    if (!motion || !animation || !animation->isActive())
        return 48;
    motion->setChecked(false);
    if (animation->isActive())
        return 49;
    motion->setChecked(true);
    if (!animation->isActive())
        return 50;
    int stationaryLinks = 0;
    int movingLinks = 0;
    for (auto *item : graph->scene()->items()) {
        if (auto *link = dynamic_cast<ResourceLinkItem *>(item)) {
            if (link->motionEnabled())
                ++movingLinks;
            else
                ++stationaryLinks;
        }
    }
    if (stationaryLinks != 1 || movingLinks != 4)
        return 57;
    dependenciesToggle->setChecked(false);
    if (!animation->isActive())
        return 56;
    dependenciesToggle->setChecked(true);
    const QPointF networkPosition = resource("network:network")->pos();
    networksToggle->setChecked(false);
    volumesToggle->setChecked(false);
    stoppedToggle->setChecked(false);
    dependenciesToggle->setChecked(false);
    if (animation->isActive())
        return 51;
    for (auto *item : graph->scene()->items()) {
        if (dynamic_cast<QGraphicsPathItem *>(item) && item->isVisible())
            return 44;
    }
    if (stoppedCard->isVisible() || resource("network:network")->isVisible()
        || resource("volume:data")->isVisible() || !containerCard->isVisible())
        return 45;
    dockerPage->findChild<QWidget *>("dockerDetailsPanel")->hide();
    capture("-docker-filtered");
    const QPointF movedPosition = containerCard->pos() + QPointF(25, 35);
    containerCard->setPos(movedPosition);
    const QTransform graphTransform = graph->transform();
    QPointer<QGraphicsObject> previousCard = dynamic_cast<QGraphicsObject *>(containerCard);
    dockerPage->refresh();
    if (!waitUntil([&]() {
        if (previousCard)
            return false;
        for (auto *item : graph->scene()->items()) {
            if (item->data(0).toString() == "container:fixture")
                return item->pos() == movedPosition && item->isSelected()
                    && graph->transform() == graphTransform;
        }
        return false;
    }))
        return 42;
    if (resource("container:stopped")->isVisible() || resource("network:network")->isVisible())
        return 46;
    networksToggle->setChecked(true);
    volumesToggle->setChecked(true);
    stoppedToggle->setChecked(true);
    dependenciesToggle->setChecked(true);
    if (!resource("container:stopped")->isVisible()
        || resource("network:network")->pos() != networkPosition)
        return 47;
    if (!animation->isActive())
        return 52;
    navigation->setCurrentRow(2);
    if (animation->isActive())
        return 53;
    navigation->setCurrentRow(3);
    if (!animation->isActive())
        return 54;
    navigation->setCurrentRow(2);
    if (!waitUntil([&]() { return target->findData("mac-mini") >= 0 && button(remote, "Refresh")->isEnabled(); })) {
        std::fprintf(stderr, "Machine discovery failed\n");
        return 3;
    }
    // All screenshots use isolated fixture data, never a live account or broker.
    if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
        window.grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-machines.png");
    remote->findChild<QPushButton *>("machinesDetailsButton")->click();
    auto *machinePages = remote->findChild<QStackedWidget *>("machinePages");
    auto *machineDetails = remote->findChild<QPlainTextEdit *>("machineDetails");
    auto *machines = remote->findChild<QTableWidget *>("managedMachines");
    if (!machinePages || machinePages->currentIndex() != 0 || machineDetails->isVisible()
        || machines->item(1, 1)->text() != "Invitation created"
        || !machines->item(1, 2)->text().startsWith("Not connected yet"))
        return 22;
    window.resize(1800, 1800);
    capture("-machines-tall");
    if (!remote->findChild<QWidget *>("machinesDetailsPanel")->isVisible())
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
    if (hostname->text() != QStringLiteral("remote.example.com"))
        return 37;
    QFile legacy(directory.filePath(QStringLiteral("legacy-connection")));
    if (!legacy.open(QIODevice::WriteOnly))
        return 38;
    legacy.close();
    hostname->setText(QStringLiteral("edited.example.com"));
    remote->refresh();
    if (!waitUntil([&]() { return summary->text().contains("Choose your provider"); })
        || summary->text().contains("older setup") || hostname->text() != "edited.example.com")
        return 39;
    remote->setResolvers({});
    auto *installProvider = remote->findChild<QPushButton *>("installRemoteProvider");
    auto *setupAction = machinePages->currentWidget()->findChild<QPushButton *>("primaryButton");
    if (!installProvider || !setupAction || !waitUntil([&]() { return installProvider->isEnabled(); })
        || installProvider->text() != "Install provider…" || setupAction->isEnabled())
        return 40;
    capture("-setup-missing-provider");
    bool providerInstalled = false;
    QTimer::singleShot(0, &window, [&]() {
        auto *dialog = window.findChild<PackageManagerDialog *>();
        if (!dialog)
            return;
        auto *install = dialog->findChild<QPushButton *>("installPackage");
        if (!install || !waitUntil([&]() { return install->isEnabled(); })) {
            dialog->reject();
            return;
        }
        QObject::connect(dialog, &PackageManagerDialog::packageInstalled, &window, [&](const QString &name) {
            providerInstalled = name == QStringLiteral("example.hosted");
        });
        if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
            dialog->grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-provider-marketplace.png");
        install->click();
    });
    installProvider->click();
    if (!providerInstalled || !waitUntil([&]() { return resolver->currentData().toString() == "example.hosted"; })
        || hostname->isVisible() || machinePages->currentIndex() != 3)
        return 41;
    resolver->setCurrentIndex(0);
    legacy.remove();
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
    if (QFile::exists(directory.filePath("service-calls")))
        return 49;
    const auto hasRemoteStatus = [&](const QString &text) {
        for (auto *label : remote->findChildren<QLabel *>()) {
            if (label->text().startsWith(text))
                return true;
        }
        return false;
    };
    QFile approved(directory.filePath("pair-approved"));
    if (!approved.open(QIODevice::WriteOnly))
        return 50;
    approved.close();
    pair->click();
    if (!waitUntil([&]() { return hasRemoteStatus("Worker service started."); }))
        return 51;
    QFile serviceCalls(directory.filePath("service-calls"));
    if (!serviceCalls.open(QIODevice::ReadOnly) || serviceCalls.readAll() != "remote\nservice\n--role\nworker\n")
        return 52;
    serviceCalls.close();
    QFile serviceFails(directory.filePath("service-fails"));
    if (!serviceFails.open(QIODevice::WriteOnly))
        return 53;
    serviceFails.close();
    pair->click();
    if (!waitUntil([&]() { return hasRemoteStatus("Could not start the worker service. Your pairing is preserved."); }))
        return 54;
    QFile pairCalls(directory.filePath("pair-calls"));
    if (!pairCalls.open(QIODevice::ReadOnly))
        return 55;
    const QByteArray completedPairCalls = pairCalls.readAll();
    pairCalls.close();
    serviceFails.remove();
    button(remote, "Start worker service")->click();
    if (!waitUntil([&]() { return hasRemoteStatus("Worker service started."); })
        || !pairCalls.open(QIODevice::ReadOnly) || pairCalls.readAll() != completedPairCalls)
        return 56;
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
    navigation->setCurrentRow(4);
    auto *executionTabs = window.findChild<QTabWidget *>("executionTabs");
    if (!executionTabs || !executionTabs->isVisible() || executionTabs->count() != 2)
        return 42;
    auto *activity = executionTabs->findChild<ActivityWidget *>();
    if (!activity || executionTabs->widget(1) != activity)
        return 127;
    executionTabs->setCurrentIndex(1);
    if (!activity->isVisible() || !button(activity, "Open logs")->isVisible()
        || !button(activity, "Stop selected run")->isVisible())
        return 128;
    auto *emptyRuns = activity->findChild<QWidget *>("localRunsEmpty");
    auto *localRuns = activity->findChild<QTableWidget *>("localActivityRuns");
    if (!emptyRuns || !emptyRuns->isVisible() || localRuns->isVisible()
        || button(activity, "Open logs")->isEnabled() || button(activity, "Stop selected run")->isEnabled())
        return 134;
    capture("-execution-local-runs");
    const QSize normalSize = window.size();
    window.resize(1910, 1040);
    capture("-execution-local-wide");
    window.resize(normalSize);
    button(activity, "Browse workflows")->click();
    if (navigation->currentRow() != 1)
        return 135;
    navigation->setCurrentRow(4);
    activity->showRemoteJobs();
    auto *download = button(activity, "Download results");
    auto *jobTable = activity->findChild<QTableWidget *>("remoteJobs");
    if (!waitUntil([&]() { return jobTable->rowCount() == 1 && button(activity, "Refresh jobs")->isEnabled(); }))
        return 43;
    if (download->isEnabled() || activity->findChild<QPlainTextEdit *>("detailConsole")->isVisible())
        return 136;
    if (!download->isVisible())
        return 129;
    if (!jobTable || jobTable->rowCount() != 1)
        return 130;
    jobTable->selectRow(0);
    if (!download->isEnabled())
        return 137;
    button(activity, "Cancel selected job")->click();
    if (!waitUntil([&]() { return download->isEnabled(); }))
        return 131;
    QFile cancelJobArgs(directory.filePath("cancel-job-args"));
    if (!cancelJobArgs.open(QIODevice::ReadOnly)
        || cancelJobArgs.readAll() != "remote\ncancel\n--id\nbuild-1\n")
        return 132;
    capture("-execution-remote-jobs");
    QFile jobsFailure(directory.filePath("jobs-fail"));
    if (!jobsFailure.open(QIODevice::WriteOnly))
        return 139;
    jobsFailure.close();
    activity->refreshRemote();
    auto *diagnostics = activity->findChild<QPlainTextEdit *>("detailConsole");
    if (!waitUntil([&]() { return diagnostics->isVisible()
        && diagnostics->toPlainText().contains("Could not reach fixture broker"); })
        || download->isEnabled() || button(activity, "Cancel selected job")->isEnabled())
        return 140;
    jobsFailure.remove();
    activity->refreshRemote();
    if (!waitUntil([&]() { return download->isEnabled(); }) || diagnostics->isVisible())
        return 141;
    executionTabs->setCurrentIndex(0);
    if (activity->isVisible() || !executionTabs->widget(0)->isVisible())
        return 133;
    QFile jobsCalls(directory.filePath("jobs-calls"));
    if (!jobsCalls.open(QIODevice::ReadOnly))
        return 44;
    const QByteArray operatorCalls = jobsCalls.readAll();
    jobsCalls.close();
    QFile workerConnection(directory.filePath("worker-connection"));
    if (!workerConnection.open(QIODevice::WriteOnly))
        return 45;
    workerConnection.close();
    activity->refreshRemote();
    if (!waitUntil([&]() {
        for (auto *label : activity->findChildren<QLabel *>()) {
            if (label->text().contains("paired to receive workflows"))
                return true;
        }
        return false;
    }))
        return 46;
    if (download->isEnabled() || button(activity, "Cancel selected job")->isEnabled()
        || !jobsCalls.open(QIODevice::ReadOnly) || jobsCalls.readAll() != operatorCalls)
        return 47;
    for (auto *table : activity->findChildren<QTableWidget *>()) {
        if (table->rowCount() != 0)
            return 48;
    }
    QFile staleService(directory.filePath("stale-service"));
    if (!staleService.open(QIODevice::WriteOnly))
        return 57;
    staleService.close();
    navigation->setCurrentRow(2);
    button(remote, "← Machines")->click();
    remote->refresh();
    auto *repair = button(remote, "Update broker service…");
    auto *serviceHint = remote->findChild<QLabel *>("brokerServiceHint");
    if (!waitUntil([&]() { return repair->isVisible() && repair->isEnabled() && serviceHint->isVisible(); }))
        return 58;
    capture("-service-update");
    if (!serviceCalls.open(QIODevice::ReadOnly))
        return 59;
    const QByteArray beforeRepair = serviceCalls.readAll();
    serviceCalls.close();
    const auto answerConfirmation = [](QMessageBox::StandardButton answer) {
        QTimer::singleShot(0, [answer]() {
            for (auto *widget : QApplication::topLevelWidgets()) {
                if (auto *dialog = qobject_cast<QMessageBox *>(widget))
                    dialog->button(answer)->click();
            }
        });
    };
    answerConfirmation(QMessageBox::No);
    repair->click();
    if (!serviceCalls.open(QIODevice::ReadOnly) || serviceCalls.readAll() != beforeRepair)
        return 60;
    serviceCalls.close();
    if (!serviceFails.open(QIODevice::WriteOnly))
        return 61;
    serviceFails.close();
    answerConfirmation(QMessageBox::Yes);
    repair->click();
    if (!waitUntil([&]() { return hasRemoteStatus("The service update did not finish."); }) || !repair->isVisible())
        return 62;
    serviceFails.remove();
    answerConfirmation(QMessageBox::Yes);
    repair->click();
    if (!waitUntil([&]() { return !repair->isVisible(); }))
        return 63;
    if (!serviceCalls.open(QIODevice::ReadOnly)
        || serviceCalls.readAll().mid(beforeRepair.size()) != "remote\nservice\n--role\nbroker\n--replace\nremote\nservice\n--role\nbroker\n--replace\n")
        return 64;
    serviceCalls.close();

    auto *viewMenu = window.findChild<QMenu *>("workspaceViewMenu");
    auto *sidebarToggle = window.findChild<QToolButton *>("toggleSidebarButton");
    auto *sidebarAction = window.findChild<QAction *>("toggleSidebarAction");
    auto *rail = window.findChild<QFrame *>("navigationRail");
    auto *pages = window.findChild<QStackedWidget *>("workspacePages");
    if (!viewMenu || !sidebarToggle || !sidebarAction || !rail || !pages)
        return 120;
    for (const QSize size : {QSize(1180, 780), QSize(1910, 1040)}) {
        window.resize(size);
        QApplication::processEvents();
        const QRect toggleRect(sidebarToggle->mapTo(window.menuBar(), QPoint()), sidebarToggle->size());
        const QRect fileRect = window.menuBar()->actionGeometry(window.menuBar()->actions().first());
        if (toggleRect.left() < 10 || fileRect.left() - toggleRect.right() < 12
            || toggleRect.intersects(fileRect))
            return 138;
    }
    window.resize(1180, 780);
    for (int index = 0; index < navigation->count(); ++index) {
        auto *action = viewMenu->actions()[index];
        if (action->text() != navigation->item(index)->text())
            return 121;
        action->trigger();
        if (navigation->currentRow() != index || !action->isChecked())
            return 122;
        navigation->setCurrentRow((index + 1) % navigation->count());
        if (action->isChecked())
            return 123;
    }
    navigation->setCurrentRow(1);
    QApplication::processEvents();
    const int expandedPageWidth = pages->width();
    sidebarToggle->click();
    QApplication::processEvents();
    if (!rail->isHidden() || sidebarAction->isChecked() || !sidebarToggle->isVisible()
        || pages->width() <= expandedPageWidth || LauncherSettings::current().sidebarVisible)
        return 124;
    capture("-sidebar-hidden");
    viewMenu->actions()[4]->trigger();
    if (navigation->currentRow() != 4 || !rail->isHidden())
        return 125;
    sidebarAction->trigger();
    if (rail->isHidden() || !LauncherSettings::current().sidebarVisible)
        return 126;
    navigation->setCurrentRow(1);
    viewMenu->popup(window.menuBar()->mapToGlobal(QPoint(40, window.menuBar()->height())));
    QApplication::processEvents();
    if (!qEnvironmentVariableIsEmpty("DOCKPIPE_UI_CAPTURE"))
        viewMenu->grab().save(qEnvironmentVariable("DOCKPIPE_UI_CAPTURE") + "-view-menu.png");
    viewMenu->hide();

    // The offscreen platform has no tray. Closing must really close the window,
    // rather than pretending an unavailable tray can reopen it.
    if (QSystemTrayIcon::isSystemTrayAvailable() || window.findChild<QSystemTrayIcon *>())
        return 31;
    window.close();
    if (window.isVisible())
        return 32;
    if (const int monitoringResult = verifySystemMonitoring(directory.path()))
        return monitoringResult;
    return 0;
}
