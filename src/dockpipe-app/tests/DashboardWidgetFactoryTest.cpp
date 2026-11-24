#include "ContextStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardLogWidget.h"
#include "DashboardFilesWidget.h"
#include "SessionManager.h"
#include "TerminalHub.h"
#include "TerminalWidget.h"
#include "WorkspaceMachineMenu.h"
#include "WorkspaceFileModel.h"
#include "RemoteDesktopWidget.h"
#include "RemoteAccessMonitor.h"
#include "PackageDisplayWidget.h"

#include <QAction>
#include <QApplication>
#include <QCoreApplication>
#include <QComboBox>
#include <QCheckBox>
#include <QMenu>
#include <QDir>
#include <QPlainTextEdit>
#include <QTextCursor>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QMetaMethod>
#include <QTimer>
#include <QVBoxLayout>
#include <cstdio>
#ifdef Q_OS_UNIX
#include <cerrno>
#include <csignal>
#endif
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QProgressBar>
#include <QPushButton>
#include <QSignalSpy>
#include <QSet>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeView>
#include <memory>

class DashboardWidgetFactoryTest final : public QObject {
    Q_OBJECT
private slots:
    void catalogAndCompactWorkflowActions();
    void compactWorkflowLaunchRetainsIndependentMachine();
    void monitoringAndRunLinksHaveSeparateDestinations();
    void boundedSnapshotViewsAndFailure();
    void systemCountersWithoutUnixOnlyMetrics();
    void filesRequireProjectAndLogsAreScoped();
    void duplicateWidgetsShareObservationReaders_data();
    void duplicateWidgetsShareObservationReaders();
    void hiddenAndDestroyedWidgetsReleaseObservationReaders();
    void unchangedLogTailPreservesSelectionAndRunModel();
    void terminalHasContextActionsWithoutInnerChrome();
    void desktopTilesReuseLockedIndependentViewsAndUpdates();
};

void DashboardWidgetFactoryTest::monitoringAndRunLinksHaveSeparateDestinations()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    QSignalSpy opened(&factory, &DashboardWidgetFactory::openFullView);
    std::unique_ptr<QWidget> system(factory.create("system"));
    system->findChild<QPushButton *>("dashboardOpenFullView")->click();
    QCOMPARE(opened.takeFirst().first().toString(), QString("system"));
    std::unique_ptr<QWidget> runs(factory.create("executions"));
    runs->findChild<QPushButton *>("dashboardOpenFullView")->click();
    QCOMPARE(opened.takeFirst().first().toString(), QString("executions"));
}

void DashboardWidgetFactoryTest::systemCountersWithoutUnixOnlyMetrics()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> system(factory.create("system"));
    QJsonObject resources{{"boot_id", "windows-boot"}, {"logical_cpus", 8},
        {"cpu_total", 1000}, {"cpu_idle", 400},
        {"memory_total", 1000}, {"memory_available", 250},
        {"disk_total", 2000}, {"disk_available", 500},
        {"network_received", 1000}, {"network_sent", 2000}, {"uptime_seconds", 7200}};
    QJsonObject snapshot{{"machine", "local"}, {"captured_at", "2026-01-01T12:00:00.000Z"},
        {"executions", QJsonArray{}}, {"system", resources}};
    emit factory.snapshotChanged(snapshot, {});
    auto *cpu = system->findChild<QLabel *>("systemMetric0");
    auto *swap = system->findChild<QLabel *>("systemMetric2");
    auto *network = system->findChild<QLabel *>("systemMetric4");
    auto *uptime = system->findChild<QLabel *>("systemMetric5");
    QVERIFY(cpu->text().contains("Waiting"));
    QCOMPARE(swap->text(), QString("Unavailable"));
    QCOMPARE(uptime->text(), QString("2.0 h"));

    resources.insert("cpu_total", 1200);
    resources.insert("cpu_idle", 450);
    resources.insert("network_received", 5096);
    snapshot.insert("system", resources);
    snapshot.insert("captured_at", "2026-01-01T12:00:04.000Z");
    emit factory.snapshotChanged(snapshot, {});
    QVERIFY(cpu->text().contains("75.0%"));
    QVERIFY(network->text().contains("1.0 KiB/s"));
    QVERIFY(system->findChild<QLabel *>("systemMetric1")->text().contains("750"));
    QVERIFY(system->findChild<QLabel *>("systemMetric3")->text().contains("1.5 KiB"));

    resources.insert("boot_id", "windows-reboot");
    snapshot.insert("system", resources);
    emit factory.snapshotChanged(snapshot, {});
    QVERIFY(cpu->text().contains("Waiting"));
    QVERIFY(!network->text().contains("/s"));
}

void DashboardWidgetFactoryTest::terminalHasContextActionsWithoutInnerChrome()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> panel(factory.create("terminal"));
    auto *terminal = panel->findChild<TerminalWidget *>();
    auto *footer = panel->findChild<QWidget *>("dashboardWidgetFooter");
    QVERIFY(terminal && footer);
    QVERIFY(!footer->isHidden());
    QVERIFY(footer->findChild<WorkspaceMachineMenu *>("dashboardWidgetMachine"));
    QVERIFY(panel->findChild<QPushButton *>("dashboardOpenFullView")->isHidden());
    auto *scoped = panel->findChild<DashboardWidgetFactory *>();
    QVERIFY(scoped);
    QCOMPARE(&scoped->terminals(), &factory.terminals());
    QVERIFY(terminal->findChild<QWidget *>("terminalToolbar")->isHidden());
    QVERIFY(factory.terminals().sessionIds().isEmpty());
    auto *open = terminal->findChild<QAction *>("terminalOpenFullViewAction");
    QVERIFY(open);
    QSignalSpy requested(&factory, &DashboardWidgetFactory::openFullView);
    open->trigger();
    QCOMPARE(requested.size(), 1);
    QCOMPARE(requested.first().first().toString(), QString("terminal"));
    panel.reset();
    QVERIFY(factory.terminals().sessionIds().isEmpty());
}

void DashboardWidgetFactoryTest::desktopTilesReuseLockedIndependentViewsAndUpdates()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    const QJsonArray paired{QJsonObject{{"node", "first"}, {"status", "paired"}, {"fingerprint", QString(64, 'a')}}};
    factory.setRemoteDesktopMachines(paired);
    std::unique_ptr<QWidget> first(factory.create("remote-desktop"));
    std::unique_ptr<QWidget> second(factory.create("remote-desktop"));
    QVERIFY(first && second);
    auto *left = first->findChild<RemoteDesktopWidget *>();
    auto *right = second->findChild<RemoteDesktopWidget *>();
    QVERIFY(left && right && left != right);
    auto *leftDisplay = left->findChild<PackageDisplayWidget *>();
    auto *rightDisplay = right->findChild<PackageDisplayWidget *>();
    QVERIFY(leftDisplay && rightDisplay && leftDisplay != rightDisplay);
    QVERIFY(!leftDisplay->hasPackageSurface());
    QVERIFY(!rightDisplay->hasPackageSurface());
    QVERIFY(!left->findChild<QCheckBox *>("desktopKeepConnected")->isChecked());
    QVERIFY(!right->findChild<QCheckBox *>("desktopKeepConnected")->isChecked());
    QVERIFY(!left->findChild<QPushButton *>("desktopConnect")->isEnabled());
    QVERIFY(first->findChild<QWidget *>("dashboardWidgetFooter")->isHidden());
    QVERIFY(!first->findChild<WorkspaceMachineMenu *>());
    QVERIFY(factory.widgetMachineContext(first.get()).isEmpty());
    QCOMPARE(left->findChild<QComboBox *>("desktopMachine")->count(), 1);
    QCOMPARE(right->findChild<QComboBox *>("desktopMachine")->count(), 1);

    // Exercise the real monitor adapter without dispatching its deferred CLI.
    // Checking/locked admission still refuses Connect for every dashboard copy.
    auto access = std::make_unique<RemoteAccessMonitor>();
    factory.setRemoteDesktopAccessMonitor(access.get());
    QVERIFY(left->findChild<QPushButton *>("desktopUnlock")->isEnabled());
    QVERIFY(right->findChild<QPushButton *>("desktopUnlock")->isEnabled());
    QVERIFY(!left->findChild<QPushButton *>("desktopConnect")->isEnabled());
    factory.setRemoteDesktopAccessMonitor(nullptr);
    access.reset();
    QVERIFY(!left->findChild<QPushButton *>("desktopUnlock")->isEnabled());

    const QJsonArray changed{QJsonObject{{"node", "second"}, {"status", "paired"}, {"fingerprint", QString(64, 'b')}}};
    factory.setRemoteDesktopMachines(changed);
    QCOMPARE(left->findChild<QComboBox *>("desktopMachine")->itemText(0), QString("second"));
    QCOMPARE(right->findChild<QComboBox *>("desktopMachine")->itemText(0), QString("second"));
    const QJsonObject workspaceTarget{{"node", "workspace"}, {"fingerprint", QString(64, 'c')},
        {"generation", QString(64, 'd')}};
    left->setProperty("workspaceOperationPending", true);
    QVERIFY(factory.setSharedMachineContext({{"target", workspaceTarget}}, "source"));
    left->setProperty("workspaceOperationPending", false);
    QCOMPARE(left->findChild<QComboBox *>("desktopMachine")->itemText(0), QString("second"));
    QVERIFY(!first->property("dashboardMachineTarget").isValid());
    QVERIFY(factory.widgetMachineContext(first.get()).isEmpty());
    QSignalSpy setup(&factory, &DashboardWidgetFactory::remoteDesktopProviderSetupRequested);
    ResolverMeta resolver;
    resolver.name = "fixture.desktop";
    resolver.title = "Updated desktop provider";
    resolver.capability = "remote.desktop";
    resolver.surface.id = "desktop";
    PackageSurfaceActionMeta action;
    action.id = "setup";
    action.scope = "surface";
    action.workflowId = "setup.changed";
    resolver.surface.actions = {action};
    factory.setRemoteDesktopResolvers({resolver});
    bool selectedUpdatedProvider = false;
    QTimer::singleShot(0, left, [left, &selectedUpdatedProvider] {
        if (auto *menu = left->findChild<QMenu *>()) {
            for (auto *entry : menu->actions()) {
                if (entry->text() == "Updated desktop provider") {
                    selectedUpdatedProvider = true;
                    entry->trigger();
                    break;
                }
            }
            menu->close();
        }
    });
    QTimer::singleShot(250, left, [left] {
        if (auto *menu = left->findChild<QMenu *>())
            menu->close();
    });
    left->findChild<QPushButton *>("desktopSetup")->click();
    QVERIFY(selectedUpdatedProvider);
    QCOMPARE(setup.size(), 1);
    QCOMPARE(setup.first().at(0).toString(), QString("setup.changed"));
    QCOMPARE(setup.first().at(1).toString(), QString("desktop"));
    QCOMPARE(setup.first().at(2).toString(), QString("setup"));

    first->show();
    second->show();
    QSignalSpy unrelatedWorkflowRefresh(&factory, &DashboardWidgetFactory::workflowsChanged);
    factory.refreshRemoteDesktopMetadata();
    QCOMPARE(unrelatedWorkflowRefresh.size(), 0);
    factory.refresh();
    QCoreApplication::processEvents();
    QVERIFY(!leftDisplay->hasPackageSurface());
    QVERIFY(!rightDisplay->hasPackageSurface());
    QVERIFY(!left->findChild<QPushButton *>("desktopConnect")->isEnabled());
    first->hide();
    second->hide();
    first.reset();
    factory.setRemoteDesktopMachines({});
    QCOMPARE(right->findChild<QComboBox *>("desktopMachine")->count(), 0);
    QVERIFY(!rightDisplay->hasPackageSurface());
}

void DashboardWidgetFactoryTest::compactWorkflowLaunchRetainsIndependentMachine()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    WorkflowMeta workflow;
    workflow.workflowId = "build";
    workflow.displayName = "Build";
    factory.setWorkflows({workflow});
    const QJsonObject target{{"node", "worker"}, {"fingerprint", QString(64, 'a')},
        {"generation", QString(64, 'b')}};
    const QJsonObject catalog{{"target", target},
        {"roots", QJsonArray{QJsonObject{{"id", "project"}, {"files_read", true}}}}};
    factory.setMachineCatalog({catalog});
    QVERIFY(factory.setSharedMachineContext(catalog, "project"));
    std::unique_ptr<QWidget> local(factory.create("workflows", nullptr, "project", {{"mode", "local"}}));
    std::unique_ptr<QWidget> remote(factory.create("workflows", nullptr, "project",
        {{"mode", "remote"}, {"target", target}, {"rootId", "project"}}));
    QVERIFY(local && remote);
    auto *continuation = new QObject(remote.get());
    continuation->setProperty("workspaceOperationPending", true);
    QString switchError;
    QVERIFY(!factory.setWidgetMachineContext(remote.get(), {{"mode", "local"}}, &switchError));
    QVERIFY(!switchError.isEmpty());
    QCOMPARE(factory.widgetMachineContext(remote.get()).value("target").toObject(), target);
    continuation->setProperty("workspaceOperationPending", false);
    QVERIFY(local->findChild<WorkspaceMachineMenu *>("dashboardWidgetMachine"));
    QSignalSpy launched(&factory, &DashboardWidgetFactory::launchWorkflowOnMachine);
    local->findChild<QPushButton *>("dashboardLaunchWorkflow")->click();
    QCOMPARE(launched.size(), 1);
    QVERIFY(launched.takeFirst().at(1).toJsonObject().isEmpty());
    QVERIFY(factory.setSharedMachineContext({}, {}));
    factory.setMachineCatalog({});
    remote->findChild<QPushButton *>("dashboardLaunchWorkflow")->click();
    QCOMPARE(launched.size(), 1);
    const auto captured = launched.takeFirst();
    QCOMPARE(captured.at(0).toString(), QString("build"));
    QCOMPARE(captured.at(1).toJsonObject(), target);
    QCOMPARE(captured.at(2).toString(), QString("project"));
    // Losing discovery never turns the captured remote intent into local work.
    QVERIFY(factory.widgetMachineContext(remote.get()).value("target").toObject() == target);
}

void DashboardWidgetFactoryTest::catalogAndCompactWorkflowActions()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    QSet<QString> kinds;
    for (const auto &entry : factory.catalog()) {
        QVERIFY(!entry.id.isEmpty());
        QVERIFY(!entry.title.isEmpty());
        QVERIFY(!entry.description.isEmpty());
        QVERIFY(!kinds.contains(entry.id));
        kinds.insert(entry.id);
    }
    QCOMPARE(kinds.size(), 12);
    QVERIFY(kinds.contains("remote-desktop"));
    QVERIFY(!factory.create("unknown"));
    QVector<WorkflowMeta> workflows;
    for (int index = 0; index < 60; ++index) {
        WorkflowMeta workflow;
        workflow.workflowId = QString("build-%1").arg(index);
        workflow.displayName = QString("Build %1").arg(index);
        workflows.append(workflow);
    }
    factory.setWorkflows(workflows);
    std::unique_ptr<QWidget> widget(factory.create("workflows"));
    auto *list = widget->findChild<QListWidget *>("dashboardWorkflowList");
    auto *search = widget->findChild<QLineEdit *>("dashboardWorkflowSearch");
    auto *launch = widget->findChild<QPushButton *>("dashboardLaunchWorkflow");
    QCOMPARE(list->count(), 40);
    QSignalSpy launchSpy(&factory, &DashboardWidgetFactory::launchWorkflowOnMachine);
    search->setText("build-59");
    QCOMPARE(list->count(), 1);
    launch->click();
    QCOMPARE(launchSpy.size(), 1);
    QCOMPARE(launchSpy[0][0].toString(), QString("build-59"));
    search->setText("missing");
    QVERIFY(!launch->isEnabled());
    QSignalSpy openSpy(&factory, &DashboardWidgetFactory::openFullView);
    QSignalSpy canvasSpy(&factory, &DashboardWidgetFactory::canvasViewRequested);
    QVERIFY(widget->findChild<QPushButton *>("dashboardOpenFullView")->isHidden());
    widget->findChild<QPushButton *>("dashboardOpenCanvasView")->click();
    QCOMPARE(canvasSpy.size(), 1);
    QVERIFY(widget->isAncestorOf(qvariant_cast<QWidget *>(canvasSpy[0][0])));
    QCOMPARE(canvasSpy[0][1].toString(), QString("workflows-full"));
    QCOMPARE(canvasSpy[0][2].toString(), factory.workdir());
    QVERIFY(openSpy.isEmpty());
    search->clear();
    QCOMPARE(list->count(), 40);
    WorkflowMeta imported;
    imported.workflowId = "vscode-task:build";
    imported.displayName = "Build project";
    imported.source = "vscode";
    imported.sourceKind = "task";
    imported.sourcePath = ".vscode/tasks.json";
    imported.cloneable = true;
    imported.supported = false;
    imported.unsupportedReason = "Unsupported command variable";
    QSignalSpy cloneSpy(&factory, &DashboardWidgetFactory::cloneWorkflow);
    factory.setWorkflows({imported});
    QCOMPARE(list->count(), 1);
    QVERIFY(list->item(0)->text().contains("VS Code"));
    QVERIFY(list->item(0)->toolTip().contains("Unsupported command variable"));
    QVERIFY(!launch->isEnabled());
    auto *clone = widget->findChild<QPushButton *>("dashboardCloneWorkflow");
    QVERIFY(clone && !clone->isHidden());
    clone->click();
    QCOMPARE(cloneSpy.size(), 1);
    QCOMPARE(cloneSpy[0][0].toString(), imported.workflowId);
    QCOMPARE(launchSpy.size(), 1);
    imported.sourceKind = "launch";
    imported.sourcePath = ".vscode/launch.json";
    imported.supported = true;
    imported.warnings = {"Runs without a debugger"};
    factory.setWorkflows({imported});
    QVERIFY(launch->isEnabled());
    QVERIFY(list->item(0)->text().contains("launch.json"));
    QVERIFY(list->item(0)->toolTip().contains("Runs without a debugger"));
    factory.setWorkdir("/different-project");
    QCOMPARE(list->count(), 0);
    QVERIFY(!launch->isEnabled());
}

void DashboardWidgetFactoryTest::boundedSnapshotViewsAndFailure()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> system(factory.create("system"));
    std::unique_ptr<QWidget> executions(factory.create("executions"));
    std::unique_ptr<QWidget> containers(factory.create("containers"));
    std::unique_ptr<QWidget> git(factory.create("git"));
    std::unique_ptr<QWidget> machines(factory.create("machines"));
    QJsonArray records;
    for (int index = 0; index < 30; ++index) {
        records.append(QJsonObject{{"workflow", QString("Build %1").arg(index)}, {"state", "running"},
            {"started_at", QString("2026-01-01T12:00:%1Z").arg(index, 2, 10, QLatin1Char('0'))}});
    }
    const QJsonObject resources{{"memory_total", 1000}, {"memory_available", 250},
        {"swap_total", 0}, {"swap_free", 0}, {"disk_total", 1000}, {"disk_available", 900}};
    emit factory.snapshotChanged({{"executions", records}, {"system", resources}}, {});
    auto *table = executions->findChild<QTableWidget *>("dashboardExecutions");
    QCOMPARE(table->rowCount(), 12);
    QCOMPARE(table->item(0, 0)->text(), QString("Build 29"));
    auto *memory = system->findChild<QLabel *>("systemMetric1");
    QVERIFY(memory->text().contains("750"));
    emit factory.snapshotChanged({}, "Unavailable fixture");
    QCOMPARE(table->rowCount(), 0);
    QCOMPARE(memory->text(), QString("Unavailable"));
    QCOMPARE(executions->findChild<QLabel *>("dashboardWidgetStatus")->text(), QString("Unavailable fixture"));

    QJsonArray inventory;
    for (int index = 0; index < 25; ++index)
        inventory.append(QJsonObject{{"Names", QString("Container %1").arg(index)}, {"State", "running"}, {"Image", "fixture"}});
    emit factory.containersChanged(inventory, {});
    auto *containerTable = containers->findChild<QTableWidget *>("dashboardContainers");
    QCOMPARE(containerTable->rowCount(), 20);
    QVERIFY(containers->findChild<QLabel *>("dashboardWidgetStatus")->text().contains("20 of 25"));
    emit factory.containersChanged({}, "Docker unavailable");
    QCOMPARE(containerTable->rowCount(), 0);

    emit factory.gitChanged({{"branch", "feature"}, {"changes", QJsonArray{QJsonObject{{"status", "M"}, {"path", "sample.cpp"}}}}}, {});
    QCOMPARE(git->findChild<QLabel *>("dashboardGitBranch")->text(), QString("feature"));
    QCOMPARE(git->findChild<QListWidget *>("dashboardGitChanges")->count(), 1);
    QTemporaryDir repository;
    factory.setWorkdir(repository.path());
    QSignalSpy gitPreview(&factory, &DashboardWidgetFactory::gitPreviewRequested);
    QSignalSpy sidebar(&factory, &DashboardWidgetFactory::openFullView);
    QSignalSpy globalFile(&factory, &DashboardWidgetFactory::openFile);
    emit factory.gitChanged({{"branch", "feature"}, {"changes", QJsonArray{
        QJsonObject{{"status", "MM"}, {"path", "both.cpp"}},
        QJsonObject{{"status", "M "}, {"path", "staged.cpp"}},
        QJsonObject{{"status", "??"}, {"path", "new.cpp"}},
        QJsonObject{{"status", "UU"}, {"path", "conflict.cpp"}}}}}, {});
    auto *changes = git->findChild<QListWidget *>("dashboardGitChanges");
    QCOMPARE(changes->count(), 5);
    const QStringList expectedPaths{"both.cpp", "both.cpp", "staged.cpp", "new.cpp", "conflict.cpp"};
    const QList<bool> expectedStaged{false, true, true, false, false};
    for (int index = 0; index < changes->count(); ++index) {
        emit changes->itemActivated(changes->item(index));
        QCOMPARE(gitPreview.size(), index + 1);
        const auto request = gitPreview.at(index);
        QVERIFY(git->isAncestorOf(qvariant_cast<QWidget *>(request[0])));
        QCOMPARE(request[1].toString(), repository.path());
        QCOMPARE(request[2].toString(), expectedPaths[index]);
        QCOMPARE(request[3].toBool(), expectedStaged[index]);
    }
    QVERIFY(sidebar.isEmpty());
    QVERIFY(globalFile.isEmpty());
    emit factory.gitChanged({}, "Git unavailable");
    QCOMPARE(git->findChild<QListWidget *>("dashboardGitChanges")->count(), 0);
    QVERIFY(git->findChild<QLabel *>("dashboardGitBranch")->text().isEmpty());

    emit factory.machinesChanged({QJsonObject{{"node", "test-machine"}, {"status", "paired"}}}, {});
    QCOMPARE(machines->findChild<QTableWidget *>("dashboardMachines")->item(0, 1)->text(), QString("paired"));
    emit factory.machinesChanged({}, "Machine unavailable");
    QCOMPARE(machines->findChild<QTableWidget *>("dashboardMachines")->rowCount(), 0);
}

void DashboardWidgetFactoryTest::filesRequireProjectAndLogsAreScoped()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> files(factory.create("files"));
    auto *tree = files->findChild<QTreeView *>("dashboardFiles");
    QVERIFY(tree->isHidden());
    QTemporaryDir project;
    factory.setWorkdir(project.path());
    QTRY_VERIFY(!tree->isHidden());
    QCOMPARE(files->findChild<WorkspaceFileModel *>()->relativePath({}), QString("."));
    auto *browser = files->findChild<DashboardFilesWidget *>();
    QVERIFY(browser);
    QSignalSpy preview(&factory, &DashboardWidgetFactory::filePreviewRequested);
    QSignalSpy sidebar(&factory, &DashboardWidgetFactory::openFullView);
    QSignalSpy globalFile(&factory, &DashboardWidgetFactory::openFile);
    const QString path = project.filePath("notes.txt");
    emit browser->openFileRequested(path);
    QCOMPARE(preview.size(), 1);
    QCOMPARE(qvariant_cast<QWidget *>(preview[0][0]), browser);
    QCOMPARE(preview[0][1].toString(), project.path());
    QCOMPARE(preview[0][2].toString(), path);
    QSignalSpy canvas(&factory, &DashboardWidgetFactory::canvasViewRequested);
    files->findChild<QPushButton *>("dashboardOpenCanvasView")->click();
    QCOMPARE(canvas.size(), 1);
    QCOMPARE(qvariant_cast<QWidget *>(canvas[0][0]), browser);
    QCOMPARE(canvas[0][1].toString(), QString("files-full"));
    QCOMPARE(canvas[0][2].toString(), project.path());
    QVERIFY(sidebar.isEmpty());
    QVERIFY(globalFile.isEmpty());
    factory.setWorkdir({});
    QVERIFY(tree->isHidden());
    std::unique_ptr<QWidget> logs(factory.create("logs"));
    QVERIFY(logs->findChild<QLabel *>()->text().contains("No desktop session logs"));
    // The widgets were never shown: construction and project changes must not
    // launch observation commands or manufacture sessions.
    QCOMPARE(store.contexts.size(), 0);
}

namespace {
class ScopedEnvironment final {
public:
    ScopedEnvironment(const char *name, const QByteArray &value)
        : m_name(name), m_previous(qgetenv(name)), m_wasSet(qEnvironmentVariableIsSet(name))
    {
        qputenv(m_name.constData(), value);
    }
    ~ScopedEnvironment()
    {
        if (m_wasSet)
            qputenv(m_name.constData(), m_previous);
        else
            qunsetenv(m_name.constData());
    }
private:
    QByteArray m_name;
    QByteArray m_previous;
    bool m_wasSet;
};

QList<qint64> helperPids(const QString &directory)
{
    QFile file(directory + "/helpers");
    if (!file.open(QIODevice::ReadOnly))
        return {};
    QList<qint64> pids;
    for (const auto &line : file.readAll().split('\n')) {
        bool valid = false;
        const qint64 pid = line.toLongLong(&valid);
        if (valid && pid > 0)
            pids.append(pid);
    }
    return pids;
}

bool helperAlive(qint64 pid)
{
#ifdef Q_OS_UNIX
    return ::kill(pid_t(pid), 0) == 0 || errno == EPERM;
#else
    Q_UNUSED(pid);
    return false;
#endif
}

int runObservationHelper(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QString directory = qEnvironmentVariable("DOCKPIPE_DASHBOARD_READER_FIXTURE");
    QFile starts(directory + "/helpers");
    if (directory.isEmpty() || !starts.open(QIODevice::WriteOnly | QIODevice::Append))
        return 30;
    starts.write(QByteArray::number(QCoreApplication::applicationPid()) + '\n');
    starts.close();
    QFile mode(directory + "/mode");
    if (mode.open(QIODevice::ReadOnly) && mode.readAll() == "hold") {
        // A real owned process stays alive until the reader cancels it. Its
        // independent deadline prevents a failed test from leaking a helper.
        QTimer::singleShot(15000, &application, [&application]() { application.exit(31); });
        return application.exec();
    }
    const QString command = application.arguments().value(1);
    QTimer::singleShot(100, &application, [&application, command]() {
        QJsonDocument document;
        if (command == "remote") {
            document = QJsonDocument(QJsonArray{});
        } else if (command == "git") {
            document = QJsonDocument(QJsonObject{{"branch", "fixture"}, {"changes", QJsonArray{}}});
        } else if (command == "container") {
            document = QJsonDocument(QJsonObject{{"Names", "fixture"}, {"State", "running"}, {"Image", "fixture"}});
        } else {
            document = QJsonDocument(QJsonObject{{"schema", "dockpipe.execution_snapshot.v1"},
                {"executions", QJsonArray{}}, {"processes", QJsonArray{}}});
        }
        QByteArray bytes = document.toJson(QJsonDocument::Compact);
        if (command == "--workflow") {
            bytes.clear();
            for (int line = 0; line < 600; ++line)
                bytes += "fixture line " + QByteArray::number(line) + '\n';
        }
        std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
        std::fflush(stdout);
        application.exit(0);
    });
    return application.exec();
}
}

void DashboardWidgetFactoryTest::duplicateWidgetsShareObservationReaders_data()
{
    QTest::addColumn<QString>("kind");
    QTest::newRow("executions") << QString("executions");
    QTest::newRow("system") << QString("system");
    QTest::newRow("git") << QString("git");
    QTest::newRow("machines") << QString("machines");
#ifdef Q_OS_UNIX
    QTest::newRow("containers") << QString("containers");
#endif
}

void DashboardWidgetFactoryTest::duplicateWidgetsShareObservationReaders()
{
    QFETCH(QString, kind);
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    ScopedEnvironment path("PATH", fixture.path().toUtf8() + QDir::listSeparator().toLatin1() + qgetenv("PATH"));
    if (kind == "containers")
        QVERIFY(QFile::link(QCoreApplication::applicationFilePath(), fixture.path() + "/docker"));
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    factory.setWorkdir(fixture.path());
    QWidget dashboard;
    auto *layout = new QVBoxLayout(&dashboard);
    for (int index = 0; index < 8; ++index)
        layout->addWidget(factory.create(kind, &dashboard));
    QMetaMethod signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::snapshotChanged);
    if (kind == "git")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::gitChanged);
    else if (kind == "machines")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::machinesChanged);
    else if (kind == "containers")
        signal = QMetaMethod::fromSignal(&DashboardWidgetFactory::containersChanged);
    QSignalSpy observations(&factory, signal);
    dashboard.resize(420, 500);
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(observations.size(), 1, 3000);
    QCOMPARE(helperPids(fixture.path()).size(), 1);
    QVERIFY(observations[0][1].toString().isEmpty());
    const QString timerName = kind == "git" || kind == "machines" ? "dashboardInventoryPoll" : "dashboardSnapshotPoll";
    auto *poll = factory.findChild<QTimer *>(timerName);
    QVERIFY(poll);
    // Exercise actual timer delivery without waiting four seconds per tick.
    QVERIFY(QMetaObject::invokeMethod(poll, "timeout", Qt::DirectConnection));
    for (int index = 0; index < 8; ++index)
        factory.refresh();
    QTRY_COMPARE_WITH_TIMEOUT(observations.size(), 2, 3000);
    QCOMPARE(helperPids(fixture.path()).size(), 2);
    QVERIFY(observations[1][1].toString().isEmpty());
    for (auto *table : dashboard.findChildren<QTableWidget *>("dashboardExecutions"))
        QCOMPARE(table->rowCount(), 0);
}

void DashboardWidgetFactoryTest::hiddenAndDestroyedWidgetsReleaseObservationReaders()
{
#ifndef Q_OS_UNIX
    QSKIP("Process identity checks in this regression target Unix desktop hosts.");
#else
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    QFile mode(fixture.path() + "/mode");
    QVERIFY(mode.open(QIODevice::WriteOnly));
    QCOMPARE(mode.write("hold"), qint64(4));
    mode.close();
    ScopedEnvironment binary("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    SessionManager sessions;
    ContextStore store;
    auto factory = std::make_unique<DashboardWidgetFactory>(sessions, store);
    QWidget dashboard;
    auto *layout = new QVBoxLayout(&dashboard);
    layout->addWidget(factory->create("executions", &dashboard));
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(helperPids(fixture.path()).size(), 1, 3000);
    const qint64 first = helperPids(fixture.path()).first();
    QVERIFY(helperAlive(first));
    dashboard.hide();
    QTRY_VERIFY_WITH_TIMEOUT(!helperAlive(first), 3000);
    auto *poll = factory->findChild<QTimer *>("dashboardSnapshotPoll");
    QVERIFY(poll);
    for (int index = 0; index < 8; ++index) {
        QVERIFY(QMetaObject::invokeMethod(poll, "timeout", Qt::DirectConnection));
        factory->refresh();
    }
    QTest::qWait(150);
    QCOMPARE(helperPids(fixture.path()).size(), 1);
    dashboard.show();
    QTRY_COMPARE_WITH_TIMEOUT(helperPids(fixture.path()).size(), 2, 3000);
    const qint64 second = helperPids(fixture.path()).last();
    QVERIFY(helperAlive(second));
    QElapsedTimer shutdown;
    shutdown.start();
    factory.reset();
    QVERIFY2(shutdown.elapsed() < 100, "Reader destruction blocked the UI thread");
    QTRY_VERIFY_WITH_TIMEOUT(!helperAlive(second), 3000);
#endif
}

void DashboardWidgetFactoryTest::unchangedLogTailPreservesSelectionAndRunModel()
{
    QTemporaryDir fixture;
    QVERIFY(fixture.isValid());
    ScopedEnvironment state("DOCKPIPE_DASHBOARD_READER_FIXTURE", fixture.path().toUtf8());
    SessionManager sessions;
    ContextStore store;
    Context context = Context::createNew();
    context.label = "Fixture logs";
    context.workdir = fixture.path();
    context.workflow = "dashboard-log-fixture";
    context.dockpipeBinary = QCoreApplication::applicationFilePath();
    store.contexts.append(context);
    QVERIFY(sessions.launch(context, fixture.path()));
    QTRY_VERIFY_WITH_TIMEOUT(!sessions.isRunning(context.id), 3000);
    DashboardLogWidget logs(sessions, store);
    logs.setWorkdir(fixture.path());
    auto *editor = logs.findChild<QPlainTextEdit *>("dashboardLogTail");
    auto *runs = logs.findChild<QComboBox *>("dashboardLogRun");
    QVERIFY(editor->toPlainText().contains("fixture line 599"));
    QVERIFY(editor->document()->blockCount() <= 400);
    QTextCursor selection = editor->textCursor();
    selection.setPosition(0);
    selection.movePosition(QTextCursor::Right, QTextCursor::KeepAnchor, 7);
    editor->setTextCursor(selection);
    const QString selected = editor->textCursor().selectedText();
    QSignalSpy textChanged(editor, &QPlainTextEdit::textChanged);
    QSignalSpy modelReset(runs->model(), &QAbstractItemModel::modelReset);
    for (int refresh = 0; refresh < 5; ++refresh)
        logs.refresh();
    QCOMPARE(textChanged.size(), 0);
    QCOMPARE(modelReset.size(), 0);
    QCOMPARE(editor->textCursor().selectedText(), selected);
}

int main(int argc, char **argv)
{
    if (argc > 1 && qEnvironmentVariableIsSet("DOCKPIPE_DASHBOARD_READER_FIXTURE")) {
        const QByteArray command(argv[1]);
        if (command == "runs" || command == "remote" || command == "git" || command == "container" || command == "--workflow")
            return runObservationHelper(argc, argv);
    }
    QTemporaryDir config;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QApplication application(argc, argv);
    DashboardWidgetFactoryTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardWidgetFactoryTest.moc"
