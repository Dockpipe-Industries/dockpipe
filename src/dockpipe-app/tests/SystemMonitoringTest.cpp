#include "DockerObservabilityWidget.h"
#include "ExecutionExplorer.h"
#include "SystemResourceOverview.h"
#include <QApplication>
#include <QComboBox>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QTableWidget>
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
    explorer.resize(1400, 900);
    explorer.setNodes({"monitor-host", "other-host"});
    auto *machine = explorer.findChild<QComboBox *>("monitoringMachine");
    machine->setCurrentIndex(1);
    explorer.show();
    explorer.setActive(true);
    auto *processes = explorer.findChild<QTableWidget *>("systemProcesses");
    if (!waitFor([&]() { return processes->rowCount() == 1; }) || processes->item(0, 0)->text() != "remote-worker")
        return 105;
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
    if (qEnvironmentVariableIsSet("DOCKPIPE_SYSTEM_CAPTURE"))
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_SYSTEM_CAPTURE"));

    DockerObservabilityWidget dockerView;
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
    for (auto *button : dockerView.findChildren<QPushButton *>()) {
        if ((button->text() == "Start" || button->text() == "Stop" || button->text().contains("/bin/")) && button->isEnabled())
            return 107;
    }

    // A wrong-node response must never be rendered, even with valid sample data.
    machine->setCurrentIndex(2);
    if (processes->rowCount() != 0 || !waitFor([&]() {
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
    return 0;
}
