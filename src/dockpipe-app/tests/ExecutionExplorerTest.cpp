#include "ExecutionExplorer.h"
#include "ExecutionMetricChart.h"
#include "SystemProcessInspector.h"
#include "ResourceCanvas.h"
#include "Theme.h"
#include <QApplication>
#include <QComboBox>
#include <QCheckBox>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QHeaderView>
#include <QMouseEvent>
#include <QStyle>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>

#define SNAPSHOT_CHECK(value) do { if (!(value)) return 90; } while (false)

static int remoteFixture(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QString root = qEnvironmentVariable("DOCKPIPE_TEST_ROOT");
    if (root.isEmpty())
        return 30;

    const QString projectBinaryDirectory = QDir(root).filePath("project/src/bin");
    if (QDir::cleanPath(QCoreApplication::applicationDirPath()) == QDir::cleanPath(projectBinaryDirectory)) {
        QFile marker(QDir(root).filePath("project-cli-invoked"));
        return marker.open(QIODevice::WriteOnly) ? 31 : 32;
    }

    QFile arguments(QDir(root).filePath("operator-args"));
    const QByteArray encoded = (application.arguments().mid(1).join('\n') + '\n').toUtf8();
    if (!arguments.open(QIODevice::WriteOnly) || arguments.write(encoded) != encoded.size())
        return 33;
    arguments.close();

    QFile output;
    const QByteArray snapshot = "{\"schema\":\"dockpipe.machine_snapshot.v1\",\"node\":\"fixture-node\",\"state\":\"live\",\"captured_at\":\"2026-01-01T12:00:00Z\",\"execution\":{}}\n";
    if (!output.open(stdout, QIODevice::WriteOnly) || output.write(snapshot) != snapshot.size() || !output.flush())
        return 34;
    return 0;
}

int main(int argc, char **argv)
{
    if (argc > 1 && QString::fromLocal8Bit(argv[1]) == "remote")
        return remoteFixture(argc, argv);

    QTemporaryDir temporary;
    qputenv("XDG_CONFIG_HOME", temporary.path().toUtf8());
    QApplication app(argc, argv);
    applyDockpipeLauncherTheme(app);
    if (qEnvironmentVariableIsSet("DOCKPIPE_EXPLORER_LIGHT")) {
        QFile base(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!base.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 11;
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(base.readAll() + light.readAll()));
    }
    ResourceCanvas exported("operationsFixture", "Fixture", "Fixture", new QLabel("Details"));
    exported.setGraph({{"network", "network", "Network"}, {"container", "container", "Container", "", "Running", 1, ResourceStatus::Running}},
        {{"network", "container", "attachment"}, {"missing", "container", "invalid"}});
    SNAPSHOT_CHECK(exported.snapshot().value("nodes").toArray().size() == 2);
    SNAPSHOT_CHECK(exported.snapshot().value("edges").toArray().size() == 1);
    SNAPSHOT_CHECK(exported.snapshot().value("nodes").toArray()[1].toObject().value("active").toBool());

    ExecutionExplorer explorer;
    explorer.resize(1600, 1000);
    explorer.setListView(false);
    explorer.show();
    QJsonObject process{{"pid", 42}, {"start_token", "boot:100"}, {"parent_pid", 7}, {"name", "fixture"},
        {"memory_bytes", 10485760}, {"cpu_seconds", 1.0}, {"state", "S"}};
    QJsonObject execution{{"id", "abc12345"}, {"workflow", "Build and verify"}, {"workflow_run", "run-1"},
        {"task", "compile"}, {"runtime", "host"}, {"machine", "local"}, {"state", "running"},
        {"process", process}, {"metrics", process}, {"started_at", "2026-01-01T12:00:00.000Z"},
        {"operation_events", QJsonArray{QJsonObject{{"ts", "2026-01-01T12:00:01Z"}, {"unit", "step.run"}, {"status", "start"}}}}};
    QJsonObject snapshot{{"schema", "dockpipe.execution_snapshot.v1"}, {"machine", "local"},
        {"captured_at", "2026-01-01T12:00:04.000Z"}, {"executions", QJsonArray{execution}}, {"processes", QJsonArray{process}}};
    if (!explorer.applySnapshot(snapshot))
        return 1;
    auto *canvas = explorer.findChild<ResourceCanvas *>();
    if (!canvas || !canvas->focusResource("abc12345"))
        return 2;
    auto *details = explorer.findChild<QLabel *>("executionDetails");
    auto *events = explorer.findChild<QPlainTextEdit *>("executionEvents");
    auto *runCpu = explorer.findChild<QLabel *>("workflowRunCpu");
    if (!details || !events || !runCpu || runCpu->text() != "Unavailable" || !events->toPlainText().contains("Process registered") || !events->toPlainText().contains("step.run"))
        return 3;
    process.insert("cpu_seconds", 3.0);
    execution.insert("metrics", process);
    snapshot.insert("executions", QJsonArray{execution});
    snapshot.insert("captured_at", "2026-01-01T12:00:08.000Z");
    if (!explorer.applySnapshot(snapshot) || runCpu->text() != "50 %")
        return 4;
    process.insert("start_token", "boot:200");
    execution.insert("metrics", process);
    snapshot.insert("executions", QJsonArray{execution});
    snapshot.insert("captured_at", "2026-01-01T12:00:12.000Z");
    if (!explorer.applySnapshot(snapshot) || runCpu->text() != "Unavailable")
        return 5;
    ExecutionMetricChart bounded;
    for (int index = 0; index < 100; ++index)
        bounded.append(double(index));
    if (bounded.sampleCount() != 60)
        return 6;
    app.processEvents();
    if (qEnvironmentVariableIsSet("DOCKPIPE_EXPLORER_CAPTURE"))
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_EXPLORER_CAPTURE"));
    auto completed = execution;
    completed.insert("id", "finished-run");
    completed.insert("state", "failed");
    completed.insert("finished_at", "2026-01-01T12:00:10.000Z");
    completed.insert("duration_ms", 2000);
    snapshot.insert("executions", QJsonArray{execution, completed});
    explorer.applySnapshot(snapshot);
    explorer.setListView(true);
    auto *runs = explorer.findChild<QTableWidget *>("workflowRunsTable");
    if (!runs || runs->rowCount() != 1 || runs->item(0, 0)->data(Qt::UserRole).toString() != "abc12345")
        return 40;
    if (explorer.findChild<QLabel *>("workflowSummaryValue0")->text() != "1")
        return 54;
    if (qEnvironmentVariableIsSet("DOCKPIPE_WORKFLOW_RUNS_CAPTURE")) {
        runs->selectRow(0);
        app.processEvents();
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_WORKFLOW_RUNS_CAPTURE") + ".running.png");
    }
    explorer.setListView(false);
    explorer.setHistory(true);
    if (runs->rowCount() != 1 || runs->item(0, 0)->data(Qt::UserRole).toString() != "finished-run")
        return 41;
    runs->selectRow(0);
    if (explorer.findChild<QLabel *>("workflowRunState")->text() != "Failed")
        return 42;
    if (explorer.findChild<QLabel *>("workflowSummaryValue2")->text() != "1")
        return 55;
    auto *outcome = explorer.findChild<QComboBox *>("workflowRunStateFilter");
    outcome->setCurrentIndex(outcome->findData("succeeded"));
    if (runs->rowCount() != 0 || explorer.findChild<QLabel *>("executionTitle")->text() != "Select a workflow task")
        return 56;
    outcome->setCurrentIndex(0);
    auto faster = completed;
    faster.insert("id", "fast-run");
    faster.insert("duration_ms", 1000);
    snapshot.insert("executions", QJsonArray{execution, completed, faster});
    explorer.applySnapshot(snapshot);
    runs->horizontalHeader()->setSortIndicator(5, Qt::AscendingOrder);
    if (runs->rowCount() != 2 || runs->item(0, 0)->data(Qt::UserRole).toString() != "fast-run")
        return 57;
    runs->horizontalHeader()->setSortIndicator(5, Qt::DescendingOrder);
    if (runs->item(0, 0)->data(Qt::UserRole).toString() != "finished-run")
        return 58;
    QJsonArray retained;
    for (int index = 0; index < 80; ++index) {
        auto record = completed;
        record.insert("id", QString("retained-%1").arg(index));
        retained.append(record);
    }
    snapshot.insert("executions", retained);
    explorer.applySnapshot(snapshot);
    auto *more = explorer.findChild<QPushButton *>("executionMore");
    more->click();
    more->click();
    if (runs->rowCount() != 80 || !more->isHidden())
        return 46;
    snapshot.insert("executions", QJsonArray{execution, completed});
    explorer.applySnapshot(snapshot);
    runs->selectRow(0);
    app.processEvents();
    if (qEnvironmentVariableIsSet("DOCKPIPE_WORKFLOW_RUNS_CAPTURE")) {
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_WORKFLOW_RUNS_CAPTURE"));
        explorer.resize(1000, 800);
        app.processEvents();
        explorer.grab().save(qEnvironmentVariable("DOCKPIPE_WORKFLOW_RUNS_CAPTURE") + ".narrow.png");
        explorer.resize(1600, 1000);
    }
    {
        ExecutionExplorer restored;
        if (!restored.findChild<QPushButton *>("executionCanvasView")->isChecked())
            return 43;
    }
    explorer.setListView(false);
    if (!runs->isVisible() || canvas->isVisible()
        || explorer.findChild<QPushButton *>("executionCanvasView")->isVisible())
        return 44;
    explorer.setHistory(false);
    if (runs->isVisible() || !canvas->isVisible()
        || !explorer.findChild<QPushButton *>("executionCanvasView")->isVisible())
        return 59;
    {
        ExecutionExplorer system(nullptr, ExecutionExplorer::Purpose::System);
        system.show();
        system.applySnapshot(snapshot);
        if (system.findChild<QComboBox *>("executionMode")->currentIndex() != 1
            || !system.findChild<QComboBox *>("executionMode")->isHidden()
            || !system.findChild<QCheckBox *>("executionStandalone")->isHidden()
            || system.findChild<QPushButton *>("executionListView")->isVisible()
            || system.findChild<QTableWidget *>("systemProcesses")->rowCount() != 1)
            return 45;
        auto *processes = system.findChild<QTableWidget *>("systemProcesses");
        auto *processTitle = system.findChild<QLabel *>("systemProcessTitle");
        auto *processSearch = system.findChild<QLineEdit *>("executionSearch");
        processes->selectRow(0);
        if (processTitle->text() != "fixture"
            || system.findChild<QLabel *>("systemProcessState")->text() != "Sleeping")
            return 47;
        processSearch->setText("42");
        if (processes->rowCount() != 1)
            return 48;
        auto *filter = system.findChild<QComboBox *>("systemProcessFilter");
        filter->setCurrentIndex(filter->findData("R"));
        if (processes->rowCount() != 0 || processTitle->text() != "Select a process")
            return 49;
        filter->setCurrentIndex(0);
        processSearch->clear();
        processes->selectRow(0);
        auto replacement = snapshot.value("processes").toArray().first().toObject();
        replacement.insert("start_token", "boot:replacement");
        auto replacedSnapshot = snapshot;
        replacedSnapshot.insert("processes", QJsonArray{replacement});
        system.applySnapshot(replacedSnapshot);
        if (processTitle->text() != "Select a process")
            return 50; // A reused PID must not inherit the old process's selection or chart.
        processes->selectRow(0);
        if (system.findChild<QLabel *>("systemProcessIdentity")->text() != "boot:replacement")
            return 51;
        replacement.remove("memory_bytes");
        replacement.remove("cpu_seconds");
        replacedSnapshot.insert("processes", QJsonArray{replacement});
        system.applySnapshot(replacedSnapshot);
        if (system.findChild<QLabel *>("systemProcessMemory")->text() != "Unavailable"
            || system.findChild<QLabel *>("systemProcessCpu")->text() != "Unavailable")
            return 52;
    }
    if (SystemProcessInspector::stateLabel("Z") != "Zombie" || SystemProcessInspector::stateLabel("?") != "?"
        || SystemProcessInspector::stateCode("running") != "R")
        return 53;
    auto *mode = explorer.findChild<QComboBox *>("executionMode");
    mode->setCurrentIndex(1);
    QJsonArray processes;
    for (int index = 0; index < 300; ++index) {
        process.insert("pid", index + 1);
        process.insert("name", QString("fixture-%1").arg(index));
        processes.append(process);
    }
    snapshot.insert("processes", processes);
    explorer.applySnapshot(snapshot);
    auto *table = explorer.findChild<QTableWidget *>("systemProcesses");
    if (!table || table->rowCount() != 250)
        return 7;
    explorer.findChild<QLineEdit *>("executionSearch")->setText("fixture-299");
    if (table->rowCount() != 1)
        return 8;
    table->selectRow(0);
    explorer.applySnapshot(snapshot);
    if (table->currentRow() != 0)
        return 12;
    explorer.findChild<QLineEdit *>("executionSearch")->clear();
    auto *header = table->horizontalHeader();
    header->setSortIndicator(1, Qt::AscendingOrder);
    if (table->item(0, 1)->text() != "1" || table->item(1, 1)->text() != "2")
        return 13;
    const QPoint clickPoint(header->sectionViewportPosition(1) + header->sectionSize(1) / 2, header->height() / 2);
    QMouseEvent press(QEvent::MouseButtonPress, clickPoint, header->viewport()->mapToGlobal(clickPoint),
        Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
    QMouseEvent release(QEvent::MouseButtonRelease, clickPoint, header->viewport()->mapToGlobal(clickPoint),
        Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(header->viewport(), &press);
    QApplication::sendEvent(header->viewport(), &release);
    if (header->sortIndicatorOrder() != Qt::DescendingOrder || table->item(0, 1)->text() != "300")
        return 14;
    table->selectRow(0);
    explorer.applySnapshot(snapshot);
    if (table->item(table->currentRow(), 1)->text() != "300" || header->sortIndicatorOrder() != Qt::DescendingOrder)
        return 15;

    QJsonArray sortable;
    const QStringList names{"Zulu", "alpha", "Middle", "unknown"};
    const QStringList states{"Z", "S", "R", "?"};
    const int pids[]{100, 2, 10, 5};
    const int parents[]{20, 3, 100, 5};
    const double memory[]{2147483648.0, 104857600.0, 10485760.0};
    for (int index = 0; index < 4; ++index) {
        QJsonObject row{{"pid", pids[index]}, {"parent_pid", parents[index]}, {"name", names[index]},
            {"state", states[index]}, {"start_token", "sort-fixture"}};
        if (index < 3) {
            row.insert("memory_bytes", memory[index]);
            row.insert("cpu_seconds", 1.0);
        }
        sortable.append(row);
    }
    snapshot.insert("processes", sortable);
    snapshot.insert("captured_at", "2026-01-01T12:00:16.000Z");
    explorer.applySnapshot(snapshot);
    const double counters[]{1.2, 4.0, 2.0};
    for (int index = 0; index < 3; ++index) {
        auto row = sortable[index].toObject();
        row.insert("cpu_seconds", counters[index]);
        sortable[index] = row;
    }
    snapshot.insert("processes", sortable);
    snapshot.insert("captured_at", "2026-01-01T12:00:20.000Z");
    explorer.applySnapshot(snapshot);
    const QString ascendingFirst[]{"2", "2", "2", "5", "10", "100"};
    const QString descendingFirst[]{"100", "100", "10", "100", "100", "2"};
    for (int column = 0; column < 6; ++column) {
        header->setSortIndicator(column, Qt::AscendingOrder);
        if (table->item(0, 1)->text() != ascendingFirst[column])
            return 16;
        header->setSortIndicator(column, Qt::DescendingOrder);
        if (table->item(0, 1)->text() != descendingFirst[column])
            return 17;
        if (column >= 4 && table->item(3, 1)->text() != "5")
            return 18;
    }
    snapshot.insert("schema", "unknown");
    if (explorer.applySnapshot(snapshot))
        return 9;
    explorer.findChild<QPushButton *>("executionPause")->setChecked(true);
    if (!explorer.findChild<QLabel *>("executionStatus")->text().contains("paused"))
        return 10;

    const QString testRoot = temporary.path();
    const QString projectRoot = testRoot + "/project";
    const QString projectBinaryDirectory = projectRoot + "/src/bin";
    const QString trustedBinaryDirectory = testRoot + "/trusted-bin";
    if (!QDir().mkpath(projectBinaryDirectory) || !QDir().mkpath(trustedBinaryDirectory))
        return 19;
#ifdef Q_OS_WIN
    const QString cliName = "dockpipe.exe";
#else
    const QString cliName = "dockpipe";
#endif
    const QString trustedCli = QDir(trustedBinaryDirectory).filePath(cliName);
    const QString projectCli = QDir(projectBinaryDirectory).filePath(cliName);
    if (!QFile::copy(QCoreApplication::applicationFilePath(), trustedCli))
        return 20;
    if (!QFile::setPermissions(trustedCli, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner))
        return 21;
    if (!QFile::copy(QCoreApplication::applicationFilePath(), projectCli))
        return 22;
    if (!QFile::setPermissions(projectCli, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner))
        return 26;

    qputenv("DOCKPIPE_TEST_ROOT", testRoot.toUtf8());
    const QByteArray previousPath = qgetenv("PATH");
    qputenv("PATH", trustedBinaryDirectory.toUtf8() + QDir::listSeparator().toLatin1() + previousPath);
    qunsetenv("DOCKPIPE_BIN");
    explorer.setWorkdir(projectRoot);
    explorer.setNodes({"fixture-node"});
    auto *machine = explorer.findChild<QComboBox *>("monitoringMachine");
    if (!machine || machine->count() < 2)
        return 23;
    machine->setCurrentIndex(1);
    explorer.findChild<QPushButton *>("executionPause")->setChecked(false);
    explorer.setActive(true);
    QElapsedTimer remoteTimer;
    remoteTimer.start();
    const QString operatorArgsPath = testRoot + "/operator-args";
    while (!QFile::exists(operatorArgsPath) && remoteTimer.elapsed() < 3000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (!QFile::exists(operatorArgsPath) || QFile::exists(testRoot + "/project-cli-invoked")) {
        std::fprintf(stderr, "Trusted remote observation fixture did not run: %s\n",
            qPrintable(explorer.findChild<QLabel *>("executionStatus")->text()));
        return 24;
    }
    QFile operatorArgs(operatorArgsPath);
    if (!operatorArgs.open(QIODevice::ReadOnly)
        || operatorArgs.readAll() != "remote\nobserve\n--node\nfixture-node\n--auth\nnever\n")
        return 25;
    explorer.setActive(false);
    return 0;
}
