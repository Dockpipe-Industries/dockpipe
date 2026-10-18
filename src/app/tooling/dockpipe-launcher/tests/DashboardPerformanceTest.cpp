#include "DashboardWorkspace.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPointer>
#include <QProgressBar>
#include <QTableWidget>
#include <QTest>
#include <QTimer>
#include <QTreeWidget>
#include <QVBoxLayout>
#include <algorithm>

namespace {
constexpr int WarmupPairs = 8;
constexpr int MeasuredPairs = 40;
constexpr int RowsPerTable = 40;
constexpr int LogLines = 250;

QWidget *representativeWidget(int index)
{
    auto *content = new QWidget;
    content->setObjectName(QStringLiteral("retainedContent_%1").arg(index));
    auto *layout = new QVBoxLayout(content);
    layout->setContentsMargins(5, 5, 5, 5);
    layout->addWidget(new QLabel(QStringLiteral("Live widget %1").arg(index)));
    switch (index % 4) {
    case 0: {
        auto *table = new QTableWidget(RowsPerTable, 4, content);
        table->setHorizontalHeaderLabels({"Name", "Status", "CPU", "Memory"});
        for (int row = 0; row < RowsPerTable; ++row) {
            table->setItem(row, 0, new QTableWidgetItem(QStringLiteral("container-%1").arg(row)));
            table->setItem(row, 1, new QTableWidgetItem("Running"));
            table->setItem(row, 2, new QTableWidgetItem("2.4%"));
            table->setItem(row, 3, new QTableWidgetItem("128 MiB"));
        }
        layout->addWidget(table);
        break;
    }
    case 1: {
        auto *logs = new QPlainTextEdit(content);
        logs->setReadOnly(true);
        logs->setMaximumBlockCount(LogLines);
        QStringList lines;
        lines.reserve(LogLines);
        for (int line = 0; line < LogLines; ++line)
            lines.append(QStringLiteral("12:34:%1  Workflow step completed successfully").arg(line % 60, 2, 10, QChar('0')));
        logs->setPlainText(lines.join('\n'));
        layout->addWidget(logs);
        break;
    }
    case 2: {
        auto *tree = new QTreeWidget(content);
        tree->setHeaderLabels({"Workflow", "Status"});
        for (int row = 0; row < RowsPerTable; ++row) {
            auto *item = new QTreeWidgetItem(tree, {QStringLiteral("Workflow %1").arg(row), "Ready"});
            new QTreeWidgetItem(item, {"Build", "Completed"});
            new QTreeWidgetItem(item, {"Verify", "Completed"});
        }
        layout->addWidget(tree);
        break;
    }
    default:
        for (int metric = 0; metric < 4; ++metric) {
            auto *progress = new QProgressBar(content);
            progress->setRange(0, 100);
            progress->setValue(20 + metric * 17);
            progress->setFormat(QStringLiteral("Metric %1: %p%").arg(metric));
            layout->addWidget(progress);
        }
        layout->addStretch();
        break;
    }
    return content;
}

void drainEvents()
{
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    QCoreApplication::processEvents(QEventLoop::AllEvents, 50);
}

qint64 percentile(QList<qint64> values, int percentileValue)
{
    std::sort(values.begin(), values.end());
    const int count = static_cast<int>(values.size());
    const int index = qBound(0, (count * percentileValue + 99) / 100 - 1, count - 1);
    return values.at(index);
}

QString milliseconds(qint64 nanoseconds)
{
    return QString::number(double(nanoseconds) / 1000000.0, 'f', 3);
}
}

class DashboardPerformanceTest : public QObject {
    Q_OBJECT
private slots:
    void modeSwitchLatencyAndRetention_data();
    void modeSwitchLatencyAndRetention();
};

void DashboardPerformanceTest::modeSwitchLatencyAndRetention_data()
{
    QTest::addColumn<int>("panelCount");
    QTest::newRow("one-widget") << 1;
    QTest::newRow("eight-widgets") << 8;
    QTest::newRow("sixteen-widgets") << 16;
}

void DashboardPerformanceTest::modeSwitchLatencyAndRetention()
{
    QFETCH(int, panelCount);
    DashboardWorkspace workspace(QStringLiteral("performance-%1").arg(panelCount));
    workspace.resize(1440, 900);
    QList<QPointer<QWidget>> retained;
    QList<int> contentObjectCounts;
    for (int index = 0; index < panelCount; ++index) {
        QWidget *content = representativeWidget(index);
        retained.append(content);

        QVERIFY(workspace.addPanel(QString::number(index), QStringLiteral("Widget %1").arg(index), content));
    }
    workspace.show();
    drainEvents();

    for (int pair = 0; pair < WarmupPairs; ++pair) {
        workspace.setMode("tiling");
        drainEvents();
        workspace.setMode("canvas");
        drainEvents();
    }
    const int baselineCanvasObjects = workspace.findChildren<QObject *>().size();
    workspace.setMode("tiling");
    drainEvents();
    const int baselineTileObjects = workspace.findChildren<QObject *>().size();
    workspace.setMode("canvas");
    drainEvents();

    for (const auto &content : retained)
        contentObjectCounts.append(content->findChildren<QObject *>().size());

    QList<qint64> canvasToTiles;
    QList<qint64> tilesToCanvas;
    QList<qint64> eventLoopLatency;
    for (int pair = 0; pair < MeasuredPairs; ++pair) {
        for (const QString &mode : {QStringLiteral("tiling"), QStringLiteral("canvas")}) {
            QElapsedTimer elapsed;
            elapsed.start();
            qint64 responseTime = -1;
            QTimer::singleShot(0, &workspace, [&responseTime, &elapsed] {
                responseTime = elapsed.nsecsElapsed();
            });
            workspace.setMode(mode);
            const qint64 switchTime = elapsed.nsecsElapsed();
            drainEvents();
            QVERIFY2(responseTime >= 0, "The UI event-loop probe must run during every bounded switch sample.");
            eventLoopLatency.append(responseTime);
            if (mode == "tiling")
                canvasToTiles.append(switchTime);
            else
                tilesToCanvas.append(switchTime);
            const int expectedObjects = mode == "tiling" ? baselineTileObjects : baselineCanvasObjects;
            QCOMPARE(workspace.findChildren<QObject *>().size(), expectedObjects);
        }
    }

    for (int index = 0; index < panelCount; ++index) {
        QVERIFY(retained.at(index));
        QCOMPARE(workspace.findChild<QWidget *>(QStringLiteral("retainedContent_%1").arg(index)), retained.at(index).data());
        QCOMPARE(retained.at(index)->findChildren<QObject *>().size(), contentObjectCounts.at(index));
    }
    const auto report = [panelCount](const QString &operation, const QList<qint64> &samples) {
        qInfo().noquote() << QStringLiteral("dashboard-perf panels=%1 operation=%2 samples=%3 p50_ms=%4 p95_ms=%5 max_ms=%6")
            .arg(panelCount).arg(operation).arg(samples.size())
            .arg(milliseconds(percentile(samples, 50)))
            .arg(milliseconds(percentile(samples, 95)))
            .arg(milliseconds(*std::max_element(samples.cbegin(), samples.cend())));
    };
    report("canvas-to-tiling", canvasToTiles);
    report("tiling-to-canvas", tilesToCanvas);
    report("event-loop-response", eventLoopLatency);
    qInfo().noquote() << QStringLiteral("dashboard-perf panels=%1 canvas_objects=%2 tile_objects=%3 retained_content=%4 object_growth=0")
        .arg(panelCount).arg(baselineCanvasObjects).arg(baselineTileObjects).arg(retained.size());

    // A mode change should respond within the commonly perceptible 100 ms UI
    // boundary at p95. Max 500 ms catches stalls while allowing a scheduler hiccup.
    // These are desktop interaction gates, not claims about browser/GPU/capture
    // frame rates. Root integration measures those separate runtime workloads.
    constexpr qint64 P95BudgetNanoseconds = 100000000;
    constexpr qint64 MaximumBudgetNanoseconds = 500000000;
    QVERIFY2(percentile(eventLoopLatency, 95) < P95BudgetNanoseconds,
             "Dashboard switching exceeded the 100 ms p95 UI-response budget.");
    QVERIFY2(*std::max_element(eventLoopLatency.cbegin(), eventLoopLatency.cend()) < MaximumBudgetNanoseconds,
             "Dashboard switching stalled the UI for 500 ms or more.");
}

QTEST_MAIN(DashboardPerformanceTest)
#include "DashboardPerformanceTest.moc"
