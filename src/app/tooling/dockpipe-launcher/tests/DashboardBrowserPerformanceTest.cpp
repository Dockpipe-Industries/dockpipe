#include "BrowserWidget.h"
#include "CanvasPanel.h"
#include "ContextStore.h"
#include "DashboardWidgetFactory.h"
#include "DashboardWorkspace.h"
#include "SessionManager.h"
#include "Theme.h"

#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QEventLoop>
#include <QFile>
#include <QJsonDocument>
#include <QPointer>
#include <QSet>
#include <QTabWidget>
#include <QTcpServer>
#include <QTcpSocket>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <QUuid>
#include <QWebEnginePage>
#include <QWebEngineView>
#include <algorithm>
#include <cstdio>
#include <functional>
#include <memory>
#ifdef Q_OS_LINUX
#include <unistd.h>
#endif

namespace {
constexpr int IdleSampleMilliseconds = 2000;
constexpr int WarmupCycles = 8;
constexpr int MeasuredCycles = 50;

struct ProcessSample {
    qint64 cpuTicks = -1;
    qint64 residentBytes = -1;
};

ProcessSample sampleParent()
{
    ProcessSample sample;
#ifdef Q_OS_LINUX
    QFile statistics("/proc/self/stat");
    if (statistics.open(QIODevice::ReadOnly)) {
        const auto bytes = statistics.readAll();
        const auto fields = bytes.mid(bytes.lastIndexOf(')') + 2).split(' ');
        if (fields.size() > 12)
            sample.cpuTicks = fields[11].toLongLong() + fields[12].toLongLong();
    }
    QFile memory("/proc/self/statm");
    if (memory.open(QIODevice::ReadOnly)) {
        const auto fields = memory.readAll().simplified().split(' ');
        if (fields.size() > 1)
            sample.residentBytes = fields[1].toLongLong() * sysconf(_SC_PAGESIZE);
    }
#endif
    return sample;
}

int descendantCount()
{
#ifdef Q_OS_LINUX
    // Only this fixture's Chromium/helpers, never unrelated host processes.
    QList<qint64> pending{QCoreApplication::applicationPid()};
    QSet<qint64> seen;
    for (int index = 0; index < pending.size(); ++index) {
        if (pending.size() > 128)
            return 129;
        const qint64 pid = pending[index];
        const QDir tasks(QString("/proc/%1/task").arg(pid));
        const auto threads = tasks.entryList(QDir::Dirs | QDir::NoDotAndDotDot);
        if (threads.size() > 1024)
            return 129;
        // Chromium also spawns from non-main threads; inspect every owned
        // thread's children rather than silently missing those processes.
        for (const QString &thread : threads) {
            QFile children(tasks.filePath(thread + "/children"));
            if (!children.open(QIODevice::ReadOnly))
                continue;
            for (const auto &value : children.read(8192).simplified().split(' ')) {
                bool valid = false;
                const qint64 child = value.toLongLong(&valid);
                if (valid && child > 0 && !seen.contains(child)) {
                    seen.insert(child);
                    pending.append(child);
                }
            }
        }
    }
    return seen.size();
#else
    return -1;
#endif
}

void idleFor(int milliseconds)
{
    QEventLoop loop;
    QTimer::singleShot(milliseconds, &loop, &QEventLoop::quit);
    loop.exec();
}

void drainEvents()
{
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    QCoreApplication::processEvents(QEventLoop::AllEvents, 30);
}

bool waitUntil(const std::function<bool()> &condition, int timeoutMilliseconds = 5000)
{
    QElapsedTimer timeout;
    timeout.start();
    while (!condition() && timeout.elapsed() < timeoutMilliseconds) {
        drainEvents();
        QTest::qWait(10);
    }
    return condition();
}

bool evaluate(QWebEnginePage *page, const QString &script, QVariant &result)
{
    // Late Chromium replies cannot access a departed stack frame after timeout.
    struct Completion {
        bool finished = false;
        QVariant result;
    };
    auto completion = std::make_shared<Completion>();
    page->runJavaScript(script, [completion](const QVariant &value) {
        completion->result = value;
        completion->finished = true;
    });
    if (!waitUntil([completion]() { return completion->finished; }))
        return false;
    result = completion->result;
    return true;
}

qint64 percentile(QList<qint64> values, int requested)
{
    std::sort(values.begin(), values.end());
    return values[qBound(0, (int(values.size()) * requested + 99) / 100 - 1, int(values.size()) - 1)];
}

double milliseconds(qint64 nanoseconds)
{
    return double(nanoseconds) / 1000000.0;
}

void reportLatency(const QString &direction, const QList<qint64> &samples)
{
    qInfo().noquote() << QString("dashboard-browser-perf direction=%1 samples=%2 ui_response_p50_ms=%3 ui_response_p95_ms=%4 ui_response_max_ms=%5")
        .arg(direction).arg(samples.size())
        .arg(milliseconds(percentile(samples, 50)), 0, 'f', 3)
        .arg(milliseconds(percentile(samples, 95)), 0, 'f', 3)
        .arg(milliseconds(*std::max_element(samples.begin(), samples.end())), 0, 'f', 3);
}

struct IdleSample {
    double parentCpuPercent = -1;
    qint64 parentResidentBytes = -1;
};

IdleSample measureIdle(const QString &phase)
{
    const auto before = sampleParent();
    QElapsedTimer clock;
    clock.start();
    idleFor(IdleSampleMilliseconds);
    const qint64 elapsed = clock.elapsed();
    const auto after = sampleParent();
    IdleSample sample;
    sample.parentResidentBytes = after.residentBytes;
#ifdef Q_OS_LINUX
    const long ticksPerSecond = sysconf(_SC_CLK_TCK);
    if (before.cpuTicks >= 0 && after.cpuTicks >= before.cpuTicks && ticksPerSecond > 0)
        sample.parentCpuPercent = 100000.0 * double(after.cpuTicks - before.cpuTicks) / (ticksPerSecond * elapsed);
#endif
    qInfo().noquote() << QString("dashboard-browser-perf phase=%1 idle_wall_ms=%2 parent_only_cpu_percent_one_core=%3 parent_only_rss_mib=%4 descendants=%5")
        .arg(phase).arg(elapsed).arg(sample.parentCpuPercent, 0, 'f', 3)
        .arg(double(sample.parentResidentBytes) / (1024 * 1024), 0, 'f', 3).arg(descendantCount());
    return sample;
}

int runObservationHelper(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QJsonObject resources{{"boot_id", "fixture-boot"}, {"logical_cpus", 8},
        {"cpu_total", 5000}, {"cpu_idle", 4000}, {"memory_total", 8589934592.0},
        {"memory_available", 4294967296.0}, {"disk_total", 100000000000.0},
        {"disk_available", 50000000000.0}};
    const QJsonObject execution{{"id", "fixture"}, {"workflow", "Fixture build"},
        {"task", "compile"}, {"state", "running"}, {"started_at", "2026-01-01T00:00:00Z"}};
    const QJsonObject snapshot{{"schema", "dockpipe.execution_snapshot.v1"}, {"machine", "fixture"},
        {"captured_at", "2026-01-01T00:00:04Z"}, {"system", resources},
        {"executions", QJsonArray{execution}}, {"processes", QJsonArray{}}};
    const auto bytes = QJsonDocument(snapshot).toJson(QJsonDocument::Compact);
    std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
    std::fflush(stdout);
    return 0;
}
}

#define CHECK_BROWSER_PERF(condition) \
    do { \
        if (!(condition)) { \
            std::fprintf(stderr, "Browser performance check failed at line %d: %s\n", __LINE__, #condition); \
            return 1; \
        } \
    } while (false)

int main(int argc, char **argv)
{
    if (argc > 1 && QByteArray(argv[1]) == "runs"
        && qEnvironmentVariableIsSet("DOCKPIPE_DASHBOARD_BROWSER_PERF_HELPER"))
        return runObservationHelper(argc, argv);

    if (qEnvironmentVariable("QT_QPA_PLATFORM").section(':', 0, 0) == "offscreen") {
        // Hosted offscreen jobs may lack a usable graphics context. Measure UI
        // response deterministically instead of exercising their virtual GPU.
        // Explicit renderer choices remain available for diagnostic runs.
        if (qEnvironmentVariableIsEmpty("QT_QUICK_BACKEND"))
            qputenv("QT_QUICK_BACKEND", "software");
        QByteArray chromiumFlags = qgetenv("QTWEBENGINE_CHROMIUM_FLAGS");
        const auto options = chromiumFlags.split(' ');
        const bool graphicsSpecified = std::any_of(options.cbegin(), options.cend(), [](const QByteArray &option) {
            return option == "--disable-gpu" || option.startsWith("--enable-gpu")
                || option.startsWith("--use-gl") || option.startsWith("--use-angle")
                || option.startsWith("--use-vulkan");
        });
        if (!graphicsSpecified) {
            if (!chromiumFlags.isEmpty())
                chromiumFlags.append(' ');
            chromiumFlags.append("--disable-gpu");
            qputenv("QTWEBENGINE_CHROMIUM_FLAGS", chromiumFlags);
        }
    }

    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QCoreApplication::setAttribute(Qt::AA_DontUseNativeDialogs);
    QTemporaryDir storage;
    CHECK_BROWSER_PERF(storage.isValid());
    qputenv("XDG_CONFIG_HOME", storage.path().toUtf8());
    qputenv("XDG_DATA_HOME", storage.path().toUtf8());
    qputenv("XDG_CACHE_HOME", storage.path().toUtf8());
    QApplication application(argc, argv);
    application.setQuitOnLastWindowClosed(false);
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    qputenv("DOCKPIPE_DASHBOARD_BROWSER_PERF_HELPER", "1");
    qInfo().noquote() << QString("dashboard-browser-perf platform=%1 rendering=real-webengine qt_quick_backend=%2 chromium_gpu_disabled=%3 cpu_memory_scope=parent-only descendants_scope=process-count")
        .arg(QApplication::platformName(), qEnvironmentVariable("QT_QUICK_BACKEND", "default"))
        .arg(qgetenv("QTWEBENGINE_CHROMIUM_FLAGS").split(' ').contains("--disable-gpu"));

    QTcpServer server;
    CHECK_BROWSER_PERF(server.listen(QHostAddress::LocalHost));
    QObject::connect(&server, &QTcpServer::newConnection, &server, [&server]() {
        while (server.hasPendingConnections()) {
            auto *socket = server.nextPendingConnection();
            QObject::connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            QTimer::singleShot(3000, socket, &QTcpSocket::abort);
            QObject::connect(socket, &QTcpSocket::readyRead, socket, [socket]() {
                const auto request = socket->property("request").toByteArray() + socket->readAll();
                if (request.size() > 8192) {
                    socket->abort();
                    return;
                }
                if (!request.contains("\r\n\r\n")) {
                    socket->setProperty("request", request);
                    return;
                }
                const QByteArray documentIdentity = QUuid::createUuid().toByteArray(QUuid::WithoutBraces);
                const QByteArray body = "<!doctype html><html><head><title>Dashboard browser fixture</title>"
                    "<style>body{background:#1b2836;color:#e9f0f8;font:18px sans-serif;padding:36px}"
                    "input{padding:12px;font:18px sans-serif;width:70%}section{margin-top:24px;padding:20px;"
                    "background:#263b4d;border-radius:12px}</style></head><body><h1>Live browser workspace</h1>"
                    "<input id='draft' aria-label='Retained browser draft' value='Initial draft'>"
                    "<section>One browser page remains alive through canvas and tiling.</section>"
                    "<script>window.fixtureIdentity='" + documentIdentity
                    + "';window.fixtureReady=true;</script></body></html>";
                const QByteArray response = "HTTP/1.1 200 OK\r\nContent-Type: text/html; charset=utf-8\r\nConnection: close\r\nContent-Length: "
                    + QByteArray::number(body.size()) + "\r\n\r\n" + body;
                socket->write(response);
                socket->disconnectFromHost();
            });
        }
    });

    auto *browser = new BrowserWidget;
    std::unique_ptr<BrowserWidget> standaloneOwner(browser);
    browser->resize(1440, 900);
    browser->show();
    auto loaded = std::make_shared<bool>(false);
    QObject::connect(browser->activeView(), &QWebEngineView::loadFinished, browser, [loaded](bool ok) { *loaded = ok; });
    browser->navigate(QUrl(QString("http://127.0.0.1:%1/fixture").arg(server.serverPort())));
    CHECK_BROWSER_PERF(waitUntil([loaded]() { return *loaded; }, 8000));
    QPointer<QWebEngineView> originalView = browser->activeView();
    QPointer<QWebEnginePage> originalPage = originalView->page();
    QVariant result;
    CHECK_BROWSER_PERF(evaluate(originalPage, "document.getElementById('draft').value='A draft retained through 50 switches'; window.fixtureIdentity", result));
    const QString identity = result.toString();
    CHECK_BROWSER_PERF(!identity.isEmpty());
    const qint64 rendererPid = originalPage->renderProcessPid();
    CHECK_BROWSER_PERF(rendererPid > 0);
    idleFor(750);
    CHECK_BROWSER_PERF(!originalView->grab().isNull());
    const IdleSample standaloneIdle = measureIdle("standalone-browser");

    SessionManager sessions;
    ContextStore contexts;
    DashboardWidgetFactory factory(sessions, contexts);
    factory.setWorkdir(storage.path());
    QVector<WorkflowMeta> workflows;
    for (int index = 0; index < 30; ++index) {
        WorkflowMeta workflow;
        workflow.workflowId = QString("fixture-build-%1").arg(index);
        workflow.displayName = QString("Build fixture %1").arg(index);
        workflows.append(workflow);
    }
    factory.setWorkflows(workflows);
    DashboardWorkspace workspace("browser-performance");
    workspace.resize(1440, 900);
    CHECK_BROWSER_PERF(workspace.addPanel("browser", "Browser", browser));
    standaloneOwner.release();
    const QStringList kinds{"system", "executions", "workflows", "logs", "files", "workflows", "logs"};
    for (int index = 0; index < kinds.size(); ++index)
        CHECK_BROWSER_PERF(workspace.addPanel(QString::number(index), kinds[index], factory.create(kinds[index])));
    workspace.show();
    workspace.setMode("tiling");
    CHECK_BROWSER_PERF(workspace.activatePanel("browser"));
    CHECK_BROWSER_PERF(originalView->isVisible());
    workspace.setMode("canvas");
    auto *browserPanel = workspace.findChild<CanvasPanel *>("dashboardPanel_browser");
    CHECK_BROWSER_PERF(browserPanel);
    browserPanel->setGeometry(12, 12, 860, 520);
    browserPanel->raise();
    for (const QString &mode : {QStringLiteral("canvas"), QStringLiteral("tiling")}) {
        workspace.setMode(mode);
        const auto layoutBeforeExpand = workspace.saveLayout();
        for (int cycle = 0; cycle < 5; ++cycle) {
            CHECK_BROWSER_PERF(workspace.togglePanelExpanded("browser"));
            drainEvents();
            CHECK_BROWSER_PERF(workspace.expandedPanel() == "browser");
            CHECK_BROWSER_PERF(originalView && originalPage && browser->activeView() == originalView.data());
            CHECK_BROWSER_PERF(originalView->isVisible());
            CHECK_BROWSER_PERF(originalPage->renderProcessPid() == rendererPid);
            CHECK_BROWSER_PERF(workspace.togglePanelExpanded("browser"));
            drainEvents();
            CHECK_BROWSER_PERF(workspace.expandedPanel().isEmpty());
            CHECK_BROWSER_PERF(workspace.saveLayout() == layoutBeforeExpand);
        }
    }
    workspace.setMode("canvas");
    for (int cycle = 0; cycle < WarmupCycles; ++cycle) {
        workspace.setMode("tiling");
        drainEvents();
        idleFor(20);
        workspace.setMode("canvas");
        drainEvents();
        idleFor(20);
    }
    idleFor(750);
    const IdleSample canvasIdle = measureIdle("dashboard-canvas-eight-widgets");
    workspace.setMode("tiling");
    drainEvents();
    idleFor(250);
    const IdleSample tilingIdle = measureIdle("dashboard-tiling-eight-widgets");
    workspace.setMode("canvas");
    drainEvents();
    idleFor(250);
    const auto warmMemory = sampleParent();
    const int warmCanvasObjects = workspace.findChildren<QObject *>().size();
    const int warmDescendants = descendantCount();
    int maximumDescendants = warmDescendants;
    QList<qint64> toTiling;
    QList<qint64> toCanvas;
    QList<qint64> synchronousTiling;
    QList<qint64> synchronousCanvas;
    QElapsedTimer measuredDuration;
    measuredDuration.start();
    for (int cycle = 0; cycle < MeasuredCycles; ++cycle) {
        CHECK_BROWSER_PERF(measuredDuration.elapsed() < 20000);
        for (const QString &mode : {QStringLiteral("tiling"), QStringLiteral("canvas")}) {
            QElapsedTimer elapsed;
            elapsed.start();
            auto response = std::make_shared<qint64>(-1);
            QTimer::singleShot(0, &workspace, [response, elapsed]() { *response = elapsed.nsecsElapsed(); });
            workspace.setMode(mode);
            const qint64 synchronous = elapsed.nsecsElapsed();
            if (mode == "tiling")
                synchronousTiling.append(synchronous);
            else
                synchronousCanvas.append(synchronous);
            drainEvents();
            CHECK_BROWSER_PERF(*response >= 0);
            if (mode == "tiling")
                toTiling.append(*response);
            else
                toCanvas.append(*response);
            CHECK_BROWSER_PERF(originalView && originalPage && browser->activeView() == originalView.data());
            CHECK_BROWSER_PERF(originalView->isVisible());
            CHECK_BROWSER_PERF(originalPage->renderProcessPid() == rendererPid);
            // Native rendering gets a frame opportunity between parent changes.
            // The measured response is not a GPU frame-rate measurement.
            idleFor(16);
            maximumDescendants = qMax(maximumDescendants, descendantCount());
        }
    }
    drainEvents();
    idleFor(250);
    CHECK_BROWSER_PERF(evaluate(originalPage,
        "JSON.stringify({identity:window.fixtureIdentity,draft:document.getElementById('draft').value})", result));
    const auto finalDocument = QJsonDocument::fromJson(result.toString().toUtf8()).object();
    CHECK_BROWSER_PERF(finalDocument.value("identity").toString() == identity);
    CHECK_BROWSER_PERF(finalDocument.value("draft").toString() == "A draft retained through 50 switches");
    CHECK_BROWSER_PERF(!originalView->grab().isNull());
    const auto finalMemory = sampleParent();
    const int finalObjects = workspace.findChildren<QObject *>().size();
    const int finalDescendants = descendantCount();
    // Keep the response deadline on the zero-timer sample, including queued
    // events. Synchronous samples distinguish rebuild cost from scheduling.
    reportLatency("synchronous-canvas-to-tiling", synchronousTiling);
    reportLatency("synchronous-tiling-to-canvas", synchronousCanvas);
    reportLatency("canvas-to-tiling", toTiling);
    reportLatency("tiling-to-canvas", toCanvas);
    qInfo().noquote() << QString("dashboard-browser-perf cycles=%1 elapsed_ms=%2 parent_only_warm_rss_mib=%3 parent_only_final_rss_mib=%4 parent_only_growth_mib=%5 canvas_objects_before=%6 canvas_objects_after=%7 descendants_before=%8 descendants_after=%9 descendants_peak=%10 renderer_pid_retained=true dom_retained=true input_retained=true")
        .arg(MeasuredCycles).arg(measuredDuration.elapsed())
        .arg(double(warmMemory.residentBytes) / (1024 * 1024), 0, 'f', 3)
        .arg(double(finalMemory.residentBytes) / (1024 * 1024), 0, 'f', 3)
        .arg(double(finalMemory.residentBytes - warmMemory.residentBytes) / (1024 * 1024), 0, 'f', 3)
        .arg(warmCanvasObjects).arg(finalObjects).arg(warmDescendants).arg(finalDescendants).arg(maximumDescendants);
    qInfo().noquote() << QString("dashboard-browser-perf idle_parent_cpu_delta_canvas_pp=%1 idle_parent_cpu_delta_tiling_pp=%2")
        .arg(canvasIdle.parentCpuPercent - standaloneIdle.parentCpuPercent, 0, 'f', 3)
        .arg(tilingIdle.parentCpuPercent - standaloneIdle.parentCpuPercent, 0, 'f', 3);

    for (const auto &samples : {toTiling, toCanvas}) {
        CHECK_BROWSER_PERF(percentile(samples, 95) < 100000000);
        CHECK_BROWSER_PERF(*std::max_element(samples.begin(), samples.end()) < 500000000);
    }
    CHECK_BROWSER_PERF(finalObjects <= warmCanvasObjects + 16);
#ifdef Q_OS_LINUX
    CHECK_BROWSER_PERF(warmMemory.residentBytes > 0 && finalMemory.residentBytes > 0);
    CHECK_BROWSER_PERF(warmDescendants >= 0 && maximumDescendants < 129);
    CHECK_BROWSER_PERF(finalMemory.residentBytes - warmMemory.residentBytes < 32 * 1024 * 1024);
    // One observer and one deferred Chromium helper may appear after warmup.
    CHECK_BROWSER_PERF(finalDescendants <= warmDescendants + 2);
    CHECK_BROWSER_PERF(maximumDescendants <= warmDescendants + 2);
#endif
    const QString capture = qEnvironmentVariable("DOCKPIPE_DASHBOARD_BROWSER_PERF_CAPTURE");
    if (!capture.isEmpty())
        CHECK_BROWSER_PERF(workspace.grab().save(capture));
    return 0;
}
