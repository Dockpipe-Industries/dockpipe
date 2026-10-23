#include "PackageSurfacePage.h"
#include "SessionManager.h"
#include <QApplication>
#include <QAction>
#include <QComboBox>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSignalSpy>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <cstdio>

namespace {
QJsonObject snapshot()
{
    return {{"schema", 1}, {"kind", "snapshot"}, {"resources", QJsonArray{QJsonObject{
        {"id", "resource-one"}, {"title", "Example resource"}, {"state", "ready"},
        {"details", QJsonObject{{"Health", "Available"}}}, {"actions", QJsonArray{"inspect"}}}}}};
}
ResolverMeta provider(const QString &name, const QString &workflow = "snapshot")
{
    ResolverMeta result;
    result.name = name;
    result.title = name;
    result.surface.id = "example.resources";
    result.surface.title = "Example Resources";
    result.surface.snapshot = workflow;
    result.surface.actions = {{"inspect", "Inspect", "resource", "inspect", {}},
        {"create", "Create", "surface", {}, "example-create"}};
    return result;
}
void emitEvent(const QJsonObject &event)
{
    const QByteArray line = "::dockpipe-surface::" + QJsonDocument(event).toJson(QJsonDocument::Compact) + '\n';
    std::fwrite(line.constData(), 1, size_t(line.size()), stdout);
    std::fflush(stdout);
}
}

class PackageSurfaceTest final : public QObject {
    Q_OBJECT
private slots:
    void retainsSnapshotOnMalformedOrExhaustedInput();
    void validatesNativeDisplayIdentityAndStaleAttachment();
    void validatesInstalledWidgetProtocol();
    void initialSelectionPrefersDisplayAndPreservesUserChoice();
    void launchRoutingRequiresExactOriginAndNeverSelectsBackgroundResources();
    void refreshesOnlyWhenActiveAndBindsResourceActions();
    void cancelsOneSnapshotAndRejectsOversizedLines();
    void coalescesInventoryRequestsUntilSnapshotFinishes();
};

void PackageSurfaceTest::initialSelectionPrefersDisplayAndPreservesUserChoice()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    auto installed = provider("widget");
    installed.surface.nativeDisplay = {1, "/installed/package", {{"linux", "/installed/package/display.so"}}};
    page.setProviders({installed});
    page.setActive(true);

    auto event = snapshot();
    const auto historical = event.value("resources").toArray().first().toObject();
    auto live = historical;
    live["id"] = "live-display";
    live["display"] = QJsonObject{{"kind", "package-native"}, {"options", QJsonObject{}}};
    event["resources"] = QJsonArray{historical, live};
    QString error;
    QVERIFY(page.applyEvent(event, error));
    auto *table = page.findChild<QTableWidget *>("surfaceResources");
    QCOMPARE(table->currentRow(), 1);

    table->selectRow(0);
    QVERIFY(page.applyEvent(event, error));
    QCOMPARE(table->currentRow(), 0);
}

void PackageSurfaceTest::validatesInstalledWidgetProtocol()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    auto installed = provider("widget");
    auto event = snapshot();
    auto row = event.value("resources").toArray().first().toObject();
    row["display"] = QJsonObject{{"kind", "package-native"}, {"options", QJsonObject{{"resource_id", "public-ref"}}}};
    event["resources"] = QJsonArray{row};
    page.setProviders({installed});
    QString error;
    QVERIFY(!page.applyEvent(event, error));
    installed.surface.nativeDisplay = {1, "/installed/package", {{"linux", "/installed/package/display.so"}}};
    page.setProviders({installed});
    QVector<PackageDisplayMeta> offered;
    connect(&page, &PackageSurfacePage::displaysChanged, &page, [&](const QVector<PackageDisplayMeta> &values) { offered = values; });
    QVERIFY(page.applyEvent(event, error));
    QCOMPARE(offered.size(), 1);
    QCOMPARE(offered.first().nativePackage.packageRoot, QString("/installed/package"));
    QCOMPARE(offered.first().leaseId, row.value("id").toString());
    QCOMPARE(offered.first().options.value("resource_id").toString(), QString("public-ref"));
    row["display"] = QJsonObject{{"kind", "package-native"}, {"options", QJsonObject{{"value", QString(4097, 'x')}}}};
    event["resources"] = QJsonArray{row};
    QVERIFY(!page.applyEvent(event, error));
    row["display"] = QJsonObject{{"kind", "package-native"}, {"options", QJsonObject{}}, {"library", "/override.so"}};
    event["resources"] = QJsonArray{row};
    QVERIFY(!page.applyEvent(event, error));
}

void PackageSurfaceTest::retainsSnapshotOnMalformedOrExhaustedInput()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    page.setProviders({provider("first"), provider("second")});
    QCOMPARE(page.findChild<QComboBox *>("surfaceProvider")->count(), 2);
    QString error;
    QVERIFY(page.applyEvent(snapshot(), error));
    auto *table = page.findChild<QTableWidget *>("surfaceResources");
    QCOMPARE(table->rowCount(), 1);
    auto invalid = snapshot();
    auto records = invalid["resources"].toArray();
    records.append(records.first());
    invalid["resources"] = records;
    QVERIFY(!page.applyEvent(invalid, error));
    QCOMPARE(table->rowCount(), 1);
    records = {};
    for (int index = 0; index <= PackageSurfacePage::maximumResources; ++index)
        records.append(QJsonObject{});
    invalid["resources"] = records;
    QVERIFY(!page.applyEvent(invalid, error));
    QCOMPARE(table->rowCount(), 1);
    QVERIFY(!page.applyEvent({{"schema", 1}, {"kind", "progress"}, {"phase", "work"}, {"message", "progress"}, {"percent", 101}}, error));
    QVERIFY(page.applyEvent({{"schema", 1}, {"kind", "progress"}, {"phase", "work"}, {"message", "progress"}, {"percent", 50}}, error));
}

void PackageSurfaceTest::validatesNativeDisplayIdentityAndStaleAttachment()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    page.setProviders({provider("first")});
    auto event = snapshot();
    auto records = event["resources"].toArray();
    auto resource = records.first().toObject();
    const QJsonObject display{{"kind", "native-window"}, {"process_id", 42}, {"process_start_token", "boot:100"}};
    const auto withDisplay = [&](const QJsonValue &value) {
        auto changed = event;
        auto row = resource;
        row["display"] = value;
        changed["resources"] = QJsonArray{row};
        return changed;
    };
    QString error;
    for (const QJsonValue &invalid : {QJsonValue(), QJsonValue("window"), QJsonValue(QJsonObject{})})
        QVERIFY(!page.applyEvent(withDisplay(invalid), error));
    for (const QJsonValue &pid : {QJsonValue(0), QJsonValue(-1), QJsonValue(1.5), QJsonValue(2147483648.0), QJsonValue("42")}) {
        auto invalid = display;
        invalid["process_id"] = pid;
        QVERIFY(!page.applyEvent(withDisplay(invalid), error));
    }
    for (const QString &token : {QString(), QString(257, 'x'), QString("boot:100\n")}) {
        auto invalid = display;
        invalid["process_start_token"] = token;
        QVERIFY(!page.applyEvent(withDisplay(invalid), error));
    }
    QVERIFY(page.applyEvent(withDisplay(display), error));
    auto *table = page.findChild<QTableWidget *>("surfaceResources");
    table->selectRow(0);
    QVERIFY(!page.findChild<QPushButton *>("surfaceAttachDisplay"));
    QVERIFY(page.findChild<QWidget *>("surfaceNativeDisplay"));
    int changes = 0;
    QVector<PackageDisplayMeta> offered;
    connect(&page, &PackageSurfacePage::displaysChanged, &page, [&](const QVector<PackageDisplayMeta> &displays) {
        ++changes;
        offered = displays;
    });
    QVERIFY(page.applyEvent(withDisplay(display), error));
    QCOMPARE(offered.size(), 1);
    QCOMPARE(offered.first().processId, 42);
    QCOMPARE(offered.first().processStartToken, QString("boot:100"));
    page.show();
    page.setActive(true);
    QVERIFY(page.findChild<QWidget *>("surfaceNativeDisplay")->isVisible());
    page.setWorkspace("/changed-workspace", "unused");
    QVERIFY(offered.isEmpty());
    QVERIFY(changes >= 2);
}

void PackageSurfaceTest::launchRoutingRequiresExactOriginAndNeverSelectsBackgroundResources()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    page.setProviders({provider("first")});
    page.show();
    page.setActive(true);
    QString error;
    auto empty = snapshot();
    empty["resources"] = QJsonArray{};
    QVERIFY(page.applyEvent(empty, error));
    auto event = snapshot();
    auto row = event["resources"].toArray().first().toObject();
    row["display"] = QJsonObject{{"kind", "native-window"}, {"process_id", 42}, {"process_start_token", "boot:100"}};
    row["launch_id"] = "terminal-origin";
    event["resources"] = QJsonArray{row};
    QVERIFY(page.applyEvent(event, error));
    auto *table = page.findChild<QTableWidget *>("surfaceResources");
    QCOMPARE(table->currentRow(), -1);
    QVERIFY(!page.findChild<QWidget *>("surfaceNativeDisplay")->isVisible());
    QVERIFY(page.supportsWorkflow("example-create"));
    QVERIFY(!page.supportsWorkflow("unrelated-create"));
    page.expectLaunch("example-create", "launcher-origin");
    QVERIFY(page.applyEvent(event, error));
    QCOMPARE(table->currentRow(), -1);
    QSignalSpy resolved(&page, &PackageSurfacePage::launchResolved);
    row["launch_id"] = "launcher-origin";
    event["resources"] = QJsonArray{row};
    QVERIFY(page.applyEvent(event, error));
    QCOMPARE(table->currentRow(), 0);
    QCOMPARE(resolved.count(), 1);
    page.expectLaunch("example-create", "expired-launch");
    auto *deadline = page.findChild<QTimer *>("surfaceLaunchDeadline");
    QVERIFY(deadline->isActive());
    QVERIFY(QMetaObject::invokeMethod(deadline, "timeout"));
    QVERIFY(!deadline->isActive());
    row["launch_id"] = "expired-launch";
    event["resources"] = QJsonArray{row};
    QVERIFY(page.applyEvent(event, error));
    QCOMPARE(table->currentRow(), -1);
    row["launch_id"] = "unsafe launch id";
    event["resources"] = QJsonArray{row};
    QVERIFY(!page.applyEvent(event, error));
}

void PackageSurfaceTest::refreshesOnlyWhenActiveAndBindsResourceActions()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    QTemporaryDir temporary;
    page.setWorkspace(temporary.path(), QCoreApplication::applicationFilePath());
    page.setProviders({provider("first")});
    page.show();
    QSignalSpy starts(&sessions, &SessionManager::sessionStarted);
    QSignalSpy output(&sessions, &SessionManager::sessionOutput);
    QSignalSpy publicWorkflow(&page, &PackageSurfacePage::publicWorkflowRequested);
    page.refresh();
    QCOMPARE(starts.count(), 0);
    page.setActive(true);
    auto *table = page.findChild<QTableWidget *>("surfaceResources");
    QTRY_COMPARE_WITH_TIMEOUT(table->rowCount(), 1, 10000);
    auto *cancel = page.findChild<QPushButton *>("surfaceCancel");
    QTRY_VERIFY_WITH_TIMEOUT(!cancel->isEnabled(), 10000);
    table->selectRow(0);
    page.findChild<QAction *>("surfaceAction-create")->trigger();
    QCOMPARE(publicWorkflow.count(), 1);
    QCOMPARE(publicWorkflow.first()[0].toString(), QString("example-create"));
    page.findChild<QAction *>("surfaceAction-inspect")->trigger();
    QTRY_VERIFY_WITH_TIMEOUT(output.count() >= 2, 10000);
    QTRY_VERIFY_WITH_TIMEOUT(!cancel->isEnabled(), 10000);
    bool bound = false;
    for (const auto &record : output)
        bound = bound || record[1].toString().contains("bound resource-one");
    QVERIFY(bound);
    page.setActive(false);
    QVERIFY(!page.findChild<QTimer *>("surfacePoll")->isActive());
}

void PackageSurfaceTest::coalescesInventoryRequestsUntilSnapshotFinishes()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    QTemporaryDir temporary;
    page.setWorkspace(temporary.path(), QCoreApplication::applicationFilePath());
    page.setProviders({provider("first", "delayed")});
    QSignalSpy starts(&sessions, &SessionManager::sessionStarted);
    QSignalSpy displays(&page, &PackageSurfacePage::displaysChanged);
    page.refreshInventory();
    QCOMPARE(starts.count(), 1);
    page.refreshInventory();
    page.refreshInventory();
    QVERIFY(page.inventoryDiagnostic().value("pending_request").toBool());
    QTRY_COMPARE_WITH_TIMEOUT(starts.count(), 2, 10000);
    QTRY_VERIFY_WITH_TIMEOUT(!page.inventoryDiagnostic().value("snapshot_busy").toBool(), 10000);
    QCOMPARE(displays.count(), 2);
    QVERIFY(!page.inventoryDiagnostic().value("pending_request").toBool());
    QVERIFY(!page.findChild<QTimer *>("surfacePoll")->isActive());
    QTest::qWait(300);
    QCOMPARE(starts.count(), 2);

    page.refreshInventory();
    page.refreshInventory();
    QVERIFY(page.inventoryDiagnostic().value("pending_request").toBool());
    page.setWorkspace(QString(), QString());
    QTest::qWait(300);
    QCOMPARE(starts.count(), 3);
    QVERIFY(!page.inventoryDiagnostic().value("pending_request").toBool());
}

void PackageSurfaceTest::cancelsOneSnapshotAndRejectsOversizedLines()
{
    SessionManager sessions;
    PackageSurfacePage page(sessions);
    QTemporaryDir temporary;
    page.setWorkspace(temporary.path(), QCoreApplication::applicationFilePath());
    page.setProviders({provider("first", "wait")});
    page.show();
    QSignalSpy starts(&sessions, &SessionManager::sessionStarted);
    page.setActive(true);
    page.refresh();
    page.refresh();
    QCOMPARE(starts.count(), 1);
    page.findChild<QPushButton *>("surfaceCancel")->click();
    QTRY_VERIFY_WITH_TIMEOUT(!page.findChild<QPushButton *>("surfaceCancel")->isEnabled(), 10000);
    QVERIFY(page.findChild<QLabel *>("surfaceStatus")->text().contains("cancelled"));
    page.setProviders({provider("first", "oversized")});
    QTRY_VERIFY_WITH_TIMEOUT(page.findChild<QLabel *>("surfaceStatus")->text().contains("capacity"), 10000);
    QTRY_VERIFY_WITH_TIMEOUT(!page.findChild<QPushButton *>("surfaceCancel")->isEnabled(), 10000);
}

int main(int argc, char **argv)
{
    if (argc > 2 && QByteArray(argv[1]) == "--workflow-file") {
        const QByteArray workflow(argv[2]);
        if (workflow == "delayed")
            QThread::msleep(200);
        if (workflow == "wait") {
            QThread::sleep(30);
            return 0;
        }
        if (workflow == "oversized") {
            const QByteArray bytes(2 * PackageSurfacePage::maximumEventBytes, 'x');
            std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout);
            std::fflush(stdout);
            return 0;
        }
        if (workflow == "inspect") {
            QString resource;
            for (int index = 1; index < argc; ++index) {
                const QString value = QString::fromLocal8Bit(argv[index]);
                if (value.startsWith("DOCKPIPE_SURFACE_RESOURCE_ID="))
                    resource = value.mid(QString("DOCKPIPE_SURFACE_RESOURCE_ID=").size());
            }
            emitEvent({{"schema", 1}, {"kind", "progress"}, {"phase", "inspect"}, {"message", "bound " + resource}});
        } else {
            emitEvent(snapshot());
        }
        return 0;
    }
    QTemporaryDir temporary;
    qputenv("XDG_CONFIG_HOME", temporary.path().toUtf8());
    QApplication app(argc, argv);
    PackageSurfaceTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "PackageSurfaceTest.moc"
