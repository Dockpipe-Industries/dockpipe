#include "DockerObservabilityWidget.h"
#include "DockerResourceInspector.h"
#include "ExecutionExplorer.h"
#include "SystemResourceOverview.h"
#include "NativeHostConnection.h"
#include <QApplication>
#include <QComboBox>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QSettings>
#include <QLineEdit>
#include <QTabWidget>
#include <QTableWidget>
#include <QSplitter>
#include <QScrollArea>
#include <QScrollBar>
#include <QPlainTextEdit>
#include <QThread>
#include <functional>

namespace {
bool waitFor(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 3000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

bool saveSnapshot(const QString &path, const QJsonObject &snapshot)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly | QIODevice::Truncate)
        && file.write(QJsonDocument(snapshot).toJson()) > 0;
}
}

// Called by RemoteLauncherTest using its isolated CLI and Docker fixtures.
int verifySystemMonitoring(const QString &directory)
{
    SystemResourceOverview overview;
    QJsonObject resources{{"boot_id", "boot-one"}, {"logical_cpus", 8}, {"cpu_total", 1000}, {"cpu_idle", 500},
        {"memory_total", 8589934592.0}, {"memory_available", 2147483648.0}, {"swap_total", 0}, {"swap_free", 0},
        {"disk_total", 1099511627776.0}, {"disk_available", 549755813888.0},
        {"network_received", 1000}, {"network_sent", 1000}, {"uptime_seconds", 7200}, {"load_1", 1.25}};
    QJsonObject process{{"pid", 42}, {"start_token", "boot:100"}, {"name", "remote-worker"}, {"state", "S"},
        {"parent_pid", 1}, {"memory_bytes", 10485760}, {"cpu_seconds", 1.0}};
    QJsonObject execution{{"schema", "dockpipe.execution_snapshot.v1"}, {"machine", "monitor-host"},
        {"captured_at", "2026-01-01T12:00:00.000Z"}, {"executions", QJsonArray{}},
        {"processes", QJsonArray{process}}, {"system", resources}};
    overview.applySnapshot(execution);
    resources.insert("cpu_total", 1100);
    resources.insert("cpu_idle", 550);
    resources.insert("network_received", 5096);
    execution.insert("captured_at", "2026-01-01T12:00:04.000Z");
    execution.insert("system", resources);
    overview.applySnapshot(execution);
    auto *cpu = overview.findChild<QLabel *>("systemMetric0");
    auto *memory = overview.findChild<QLabel *>("systemMetric1");
    auto *network = overview.findChild<QLabel *>("systemMetric4");
    if (!cpu->text().contains("50.0%") || !memory->text().contains("6.0 GiB") || !network->text().contains("1.0 KiB/s"))
        return 101;
    auto *capacity = overview.findChild<QLabel *>("systemMetricCaption1");
    if (!capacity->text().contains("8.0 GiB") || !capacity->text().contains("75.0%")
        || overview.findChild<QLabel *>("systemMetric2")->text() != "No swap")
        return 136;
    overview.applySnapshot(execution);
    if (!cpu->text().contains("50.0%"))
        return 102;
    resources.insert("boot_id", "boot-two");
    execution.insert("system", resources);
    overview.applySnapshot(execution);
    if (cpu->text().contains("50.0%"))
        return 103;
    QJsonObject docker{{"available", true}, {"containers", QJsonArray{QJsonObject{
        {"ID", "remote-container"}, {"Names", "remote-web"}, {"State", "running"}, {"Status", "Up"}, {"Image", "example/web"}}}},
        {"networks", QJsonArray{}}, {"volumes", QJsonArray{}}, {"topology", QJsonArray{}}};
    QJsonObject envelope{{"schema", "dockpipe.machine_snapshot.v1"}, {"node", "monitor-host"}, {"state", "live"},
        {"execution", execution}, {"docker", docker}};
    const QString path = directory + "/monitor.json";
    if (!saveSnapshot(path, envelope))
        return 104;

    ExecutionExplorer explorer;
    explorer.resize(1600, 1050);
    explorer.setNodes({"monitor-host", "other-host"});
    auto *machine = explorer.findChild<QComboBox *>("monitoringMachine");
    machine->setCurrentIndex(1);
    explorer.show();
    explorer.setActive(true);
    auto *processes = explorer.findChild<QTableWidget *>("systemProcesses");
    if (!waitFor([&]() { return processes->rowCount() == 1; }) || processes->item(0, 0)->text() != "remote-worker")
        return 105;
    QFile passiveObserveArgs(directory + "/observe-args");
    if (!passiveObserveArgs.open(QIODevice::ReadOnly)
        || passiveObserveArgs.readAll() != "remote\nobserve\n--node\nmonitor-host\n--auth\nnever\n")
        return 115;
    auto *executionRefresh = explorer.findChild<QPushButton *>("executionRefresh");
    if (!executionRefresh)
        return 116;
    executionRefresh->click();
    if (!waitFor([&]() {
            QFile automaticObserveArgs(directory + "/observe-auto-args");
            return automaticObserveArgs.open(QIODevice::ReadOnly)
                && automaticObserveArgs.readAll().contains("--auth\nauto\n");
        }))
        return 117;
    if (!waitFor([&]() { return executionRefresh->isEnabled(); }))
        return 121;
    resources.insert("cpu_total", 1200);
    resources.insert("cpu_idle", 600);
    resources.insert("network_received", 9192);
    execution.insert("system", resources);
    execution.insert("captured_at", "2026-01-01T12:00:08.000Z");
    envelope.insert("execution", execution);
    if (!saveSnapshot(path, envelope))
        return 113;
    explorer.refresh();
    if (!waitFor([&]() { return explorer.findChild<QLabel *>("systemMetric0")->text().contains("50.0%"); }))
        return 114;
    processes->selectRow(0);
    auto *processTitle = explorer.findChild<QLabel *>("systemProcessTitle");
    if (processTitle->text() != "remote-worker"
        || explorer.findChild<QLabel *>("systemProcessMemory")->text() != "10.0 MiB")
        return 137;
    if (qEnvironmentVariableIsSet("DOCKPIPE_SYSTEM_CAPTURE"))
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_SYSTEM_CAPTURE"));
    explorer.resize(1000, 850);
    QApplication::processEvents();
    auto *processSplitter = explorer.findChild<QSplitter *>("systemProcessSplitter");
    if (processSplitter->orientation() != Qt::Vertical)
        return 138;
    const int requiredHeight = processSplitter->widget(0)->minimumHeight()
        + processSplitter->widget(1)->minimumHeight() + processSplitter->handleWidth();
    if (processSplitter->minimumHeight() < requiredHeight)
        return 139;
    if (qEnvironmentVariableIsSet("DOCKPIPE_SYSTEM_NARROW_CAPTURE")) {
        const QString path = qEnvironmentVariable("DOCKPIPE_SYSTEM_NARROW_CAPTURE");
        explorer.grab().save(path);
        auto *scroll = explorer.findChild<QScrollArea *>("systemObservabilityScroll");
        if (!waitFor([&]() { return scroll->verticalScrollBar()->maximum() > 0; }))
            return 140;
        scroll->verticalScrollBar()->setValue(scroll->verticalScrollBar()->maximum());
        QApplication::processEvents();
        explorer.grab().save(path + ".details.png");
        scroll->verticalScrollBar()->setValue(0);
    }
    explorer.resize(1400, 900);

    QSettings().setValue("docker/viewMode", "invalid");
    DockerObservabilityWidget dockerView;
    auto *dashboardButton = dockerView.findChild<QPushButton *>("dockerDashboardView");
    auto *canvasButton = dockerView.findChild<QPushButton *>("dockerCanvasView");
    auto *dashboard = dockerView.findChild<QWidget *>("dockerDashboard");
    auto *inspector = dockerView.findChild<DockerResourceInspector *>();
    auto *canvas = dockerView.findChild<ResourceCanvas *>();
    if (!dashboardButton || !canvasButton || !dashboard || !inspector || !canvas
        || !canvasButton->isChecked() || !canvas->isAncestorOf(inspector))
        return 123;
    dockerView.setNodes({"monitor-host"});
    auto *dockerMachine = dockerView.findChild<QComboBox *>("monitoringMachine");
    dockerMachine->setCurrentIndex(1);
    dockerView.setActive(true);
    QTableWidget *containers = nullptr;
    for (auto *table : dockerView.findChildren<QTableWidget *>("dockerTable")) {
        if (table->columnCount() == 5)
            containers = table;
    }
    if (!containers || !waitFor([&]() { return containers->rowCount() == 1; }) || containers->item(0, 0)->text() != "remote-web")
        return 106;
    containers->selectRow(0);
    // Switching reuses inventory and inspector, preserving machine and selection.
    for (int iteration = 0; iteration < 3; ++iteration) {
        dashboardButton->click();
        if (!dashboard->isAncestorOf(containers) || !dashboard->isAncestorOf(inspector)
            || containers->currentRow() != 0 || dockerMachine->currentData().toString() != "monitor-host"
            || !dashboardButton->isChecked() || canvasButton->isChecked())
            return 124;
        auto *search = dockerView.findChild<QLineEdit *>("dockerContainerSearch");
        search->setText("missing-container");
        if (!containers->isRowHidden(0))
            return 125;
        search->clear();
        if (containers->isRowHidden(0))
            return 126;
        auto *stateFilter = dockerView.findChild<QComboBox *>("dockerStateFilter");
        stateFilter->setCurrentIndex(stateFilter->findData("stopped"));
        if (!containers->isRowHidden(0)
            || dockerView.findChild<QLabel *>("dockerResourceTitle")->text() != "Select a resource")
            return 132;
        stateFilter->setCurrentIndex(stateFilter->findData("running"));
        if (containers->isRowHidden(0))
            return 133;
        stateFilter->setCurrentIndex(0);
        canvasButton->click();
        if (!canvas->isAncestorOf(inspector) || dashboard->isAncestorOf(containers)
            || containers->currentRow() != 0 || !canvasButton->isChecked())
            return 127;
    }
    dashboardButton->click();
    {
        DockerObservabilityWidget restored;
        auto *restoredDashboard = restored.findChild<QWidget *>("dockerDashboard");
        auto *restoredInspector = restored.findChild<DockerResourceInspector *>();
        if (!restored.findChild<QPushButton *>("dockerDashboardView")->isChecked()
            || !restoredDashboard->isAncestorOf(restoredInspector))
            return 128;
    }
    auto *title = dockerView.findChild<QLabel *>("dockerResourceTitle");
    if (!title || title->text() != "remote-web")
        return 129;
    auto *inventoryTabs = dockerView.findChild<QTabWidget *>("dockerInventoryTabs");
    inventoryTabs->setCurrentIndex(1);
    if (title->text() != "Select a resource")
        return 130;
    inventoryTabs->setCurrentIndex(0);
    if (title->text() != "remote-web")
        return 131;
    dockerView.resize(1600, 960);
    dockerView.show();
    QApplication::processEvents();
    auto *listSplitter = dockerView.findChild<QSplitter *>("dockerListSplitter");
    if (listSplitter->orientation() != Qt::Horizontal || !inspector->isVisible())
        return 134;
    if (qEnvironmentVariableIsSet("DOCKPIPE_DOCKER_CAPTURE"))
        dockerView.grab().save(qEnvironmentVariable("DOCKPIPE_DOCKER_CAPTURE"));
    dockerView.resize(1000, 850);
    QApplication::processEvents();
    if (listSplitter->orientation() != Qt::Vertical || !inspector->isVisible())
        return 135;
    if (qEnvironmentVariableIsSet("DOCKPIPE_DOCKER_NARROW_CAPTURE"))
        dockerView.grab().save(qEnvironmentVariable("DOCKPIPE_DOCKER_NARROW_CAPTURE"));
    dockerView.resize(1600, 960);
    auto *dockerRefresh = dockerView.findChild<QPushButton *>("dockerRefresh");
    if (!dockerRefresh)
        return 119;
    if (!waitFor([&]() { return dockerRefresh->isEnabled(); }))
        return 122;
    QFile priorAutomaticArgs(directory + "/observe-auto-args");
    if (!priorAutomaticArgs.open(QIODevice::ReadOnly))
        return 118;
    const qsizetype previousAutomaticSize = priorAutomaticArgs.size();
    dockerRefresh->click();
    if (!waitFor([&]() {
            QFile updatedArgs(directory + "/observe-auto-args");
            return updatedArgs.open(QIODevice::ReadOnly) && updatedArgs.size() > previousAutomaticSize;
        }))
        return 120;
    if (!waitFor([&]() { return dockerRefresh->isEnabled(); }))
        return 122;
    for (auto *button : dockerView.findChildren<QPushButton *>()) {
        if ((button->text() == "Start" || button->text() == "Stop" || button->text().contains("/bin/")) && button->isEnabled())
            return 107;
    }

    // A wrong-node response must never be rendered, even with valid sample data.
    machine->setCurrentIndex(2);
    if (processes->rowCount() != 0 || processTitle->text() != "Select a process" || !waitFor([&]() {
        return explorer.findChild<QLabel *>("executionStatus")->text().contains("unavailable");
    }))
        return 108;
    machine->setCurrentIndex(1);
    if (!waitFor([&]() { return processes->rowCount() == 1; }))
        return 109;
    QFile delay(directory + "/monitor-delay");
    if (!delay.open(QIODevice::WriteOnly))
        return 115;
    delay.close();
    explorer.refresh();
    machine->setCurrentIndex(2);
    auto *command = explorer.findChild<RemoteCommand *>();
    if (!waitFor([&]() { return !command->busy(); }) || processes->rowCount() != 0)
        return 116;
    delay.remove();
    machine->setCurrentIndex(1);
    if (!waitFor([&]() { return processes->rowCount() == 1; }))
        return 117;
    envelope.insert("state", "stale");
    if (!saveSnapshot(path, envelope))
        return 110;
    explorer.refresh();
    dockerView.refresh();
    if (!waitFor([&]() { return processes->rowCount() == 0 && containers->rowCount() == 0; }))
        return 111;
    explorer.setActive(false);
    dockerView.setActive(false);
    machine->setCurrentIndex(0);
    if (!explorer.findChild<QComboBox *>("executionMode")->isEnabled()
        || explorer.findChild<QLabel *>("systemMetric1")->text() != "Unavailable")
        return 112;

    // Native monitoring uses the authenticated connection status and the actual
    // host envelope; paired-node refresh must preserve the native selection.
    const QString reference = directory + "/native-reference.json";
    const QString nativeStatusPath = directory + "/native-status.json";
    QJsonObject nativeStatus{{"schema", "dockpipe.native_host_status.v1"}, {"available", true},
        {"connected", true}, {"node", "monitor-host"}, {"username", "operator"}, {"uid", 1026},
        {"gid", 100}, {"groups", QJsonArray{100, 101}}, {"generation", "fixture-one"}};
    envelope.insert("state", "live");
    auto nativeDocker = docker;
    auto nativeContainers = nativeDocker.value("containers").toArray();
    auto nativeContainer = nativeContainers.first().toObject();
    const QString nativeContainerID(64, 'a');
    nativeContainer.insert("ID", nativeContainerID);
    nativeContainers[0] = nativeContainer;
    nativeDocker.insert("containers", nativeContainers);
    envelope.insert("docker", nativeDocker);
    if (!saveSnapshot(reference, QJsonObject{{"fixture", true}})
        || !saveSnapshot(nativeStatusPath, nativeStatus) || !saveSnapshot(path, envelope))
        return 123;
    NativeHostConnection native(reference);
    explorer.setNativeHostConnection(&native);
    dockerView.setNativeHostConnection(&native);
    explorer.setNodes({"monitor-host"});
    dockerView.setNodes({"monitor-host"});
    if (machine->currentData().toString() != NativeHostConnection::selectionKey()
        || dockerMachine->currentData().toString() != NativeHostConnection::selectionKey()
        || machine->itemText(0) != "Application container")
        return 124;
    native.refresh();
    if (!waitFor([&]() { return native.available(); }))
        return 125;
    explorer.setActive(true);
    dockerView.setActive(true);
    if (!waitFor([&]() { return processes->rowCount() == 1 && containers->rowCount() == 1; }))
        return 126;
    QJsonObject nativeQuery{{"schema", "dockpipe.native_host_docker_query.v1"}, {"outcome", "Success"},
        {"kind", "container"}, {"id", nativeContainerID},
        {"inspect", QJsonObject{{"Id", nativeContainerID},
            {"Config", QJsonObject{{"Image", "native-image"}, {"Hostname", "<untrusted-host>"}}}}},
        {"logs", "native recent log"}, {"logs_truncated", true}};
    const QString queryPath = directory + "/native-query.json";
    if (!saveSnapshot(queryPath, nativeQuery))
        return 134;
    containers->selectRow(0);
    auto *nativeLogs = dockerView.findChild<QPlainTextEdit *>("dockerSelectedLogs");
    auto *nativeDetails = dockerView.findChild<QLabel *>("dockerReadableDetails");
    if (!nativeLogs || !nativeDetails || !waitFor([&]() {
            return nativeLogs->toPlainText().contains("native recent log");
        }))
        return 135;
    if (!nativeLogs->toPlainText().contains("64 KiB")
        || !nativeDetails->text().contains("native-image")
        || !nativeDetails->text().contains("&lt;untrusted-host&gt;"))
        return 136;
    bool foundRawInspect = false;
    for (auto *view : dockerView.findChildren<QPlainTextEdit *>()) {
        if (view != nativeLogs && view->toPlainText().contains("native-image"))
            foundRawInspect = true;
    }
    if (!foundRawInspect)
        return 137;
    QFile queryArguments(directory + "/native-query-args");
    const QString expectedQueryArguments = "host\ndocker-query\n--connection\n" + reference
        + "\n--kind\ncontainer\n--id\n" + nativeContainerID + "\n--json\n";
    if (!queryArguments.open(QIODevice::ReadOnly)
        || queryArguments.readAll() != expectedQueryArguments.toUtf8())
        return 138;
    nativeQuery.insert("logs_error", "docker_logs_unavailable");
    nativeQuery.insert("logs", "");
    if (!saveSnapshot(queryPath, nativeQuery))
        return 139;
    dockerView.refresh();
    if (!waitFor([&]() { return nativeLogs->toPlainText().contains("logging driver"); })
        || !nativeDetails->text().contains("native-image"))
        return 140;
    // Pause cancels an in-flight query and rejects its delayed response. It
    // also blocks explicit refresh until the operator resumes updates.
    queryArguments.close();
    if (!queryArguments.remove())
        return 141;
    QFile queryDelay(directory + "/native-query-delay");
    if (!queryDelay.open(QIODevice::WriteOnly))
        return 146;
    queryDelay.close();
    nativeQuery.remove("logs_error");
    nativeQuery.insert("logs", "late query must not render");
    if (!saveSnapshot(queryPath, nativeQuery))
        return 142;
    dockerView.refresh();
    if (!waitFor([&]() { return QFile::exists(queryArguments.fileName()); }))
        return 147;
    auto *pause = dockerView.findChild<QPushButton *>("dockerPauseUpdates");
    pause->click();
    QElapsedTimer cancelledQuery;
    cancelledQuery.start();
    if (!waitFor([&]() { return cancelledQuery.elapsed() > 1200; })
        || !nativeLogs->toPlainText().contains("logging driver"))
        return 148;
    if (!queryDelay.remove() || !queryArguments.remove())
        return 149;
    nativeQuery.insert("logs", "after resume");
    if (!saveSnapshot(queryPath, nativeQuery))
        return 150;
    dockerView.refresh();
    QApplication::processEvents();
    if (QFile::exists(queryArguments.fileName()) || !nativeLogs->toPlainText().contains("logging driver"))
        return 143;
    pause->click();
    if (!waitFor([&]() { return nativeLogs->toPlainText().contains("after resume"); }))
        return 144;
    QPushButton *nativeStop = nullptr;
    for (auto *button : dockerView.findChildren<QPushButton *>()) {
        if (button->text() == "Stop")
            nativeStop = button;
        if (button->text().contains("/bin/") && button->isEnabled())
            return 130;
    }
    if (!nativeStop || !nativeStop->isEnabled())
        return 131;
    nativeStop->click();
    const QString nativeArgumentsPath = directory + "/native-docker-args";
    if (!waitFor([&]() { return QFile::exists(nativeArgumentsPath) && nativeStop->isEnabled(); }))
        return 132;
    QFile nativeArguments(nativeArgumentsPath);
    const QString expectedNativeArguments = "host\ndocker\nstop\n--connection\n" + reference
        + "\n--id\n" + nativeContainerID + "\n--json\n";
    if (!nativeArguments.open(QIODevice::ReadOnly)
        || nativeArguments.readAll() != expectedNativeArguments.toUtf8())
        return 133;
    nativeStatus.insert("available", false);
    nativeStatus.insert("connected", false);
    if (!saveSnapshot(nativeStatusPath, nativeStatus))
        return 127;
    native.refresh();
    if (!waitFor([&]() {
            return !native.available() && processes->rowCount() == 0 && containers->rowCount() == 0;
        }))
        return 128;
    if (machine->currentData().toString() != NativeHostConnection::selectionKey()
        || dockerMachine->currentData().toString() != NativeHostConnection::selectionKey())
        return 129;
    if (!nativeLogs->toPlainText().isEmpty() || nativeDetails->text().contains("native-image"))
        return 145;
    explorer.setActive(false);
    dockerView.setActive(false);
    return 0;
}
