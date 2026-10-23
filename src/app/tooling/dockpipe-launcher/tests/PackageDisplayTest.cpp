#include "PackageDisplayWidget.h"
#include "LauncherDisplayPackage.h"
#include "SurfaceLaunchEnvironment.h"
#include "Context.h"
#include "SessionManager.h"
#include <QApplication>
#include <QFile>
#include <QFileInfo>
#include <QScopeGuard>
#include <QJsonDocument>
#include <QDir>
#include <QJsonArray>
#include <QPluginLoader>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

class PackageDisplayTest final : public QObject {
    Q_OBJECT
private slots:
    void optionsBounds();
    void transientLaunchEnvironmentPrivacy();
    void installedConsumerLeaseAndCompatibility();
};

void PackageDisplayTest::transientLaunchEnvironmentPrivacy()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
#if defined(Q_OS_WIN)
    const QString asset = root.filePath("fixture-tool.exe");
#else
    const QString asset = root.filePath("fixture-tool");
#endif
    QVERIFY(QFile::copy(QCoreApplication::applicationFilePath(), asset));
    QFile tool(asset);
    QVERIFY(tool.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner));
    const QByteArray sentinel("inherit-private-sentinel-43");
    const QByteArray previous = qgetenv("PACKAGE_DISPLAY_EXECUTABLE");
    const bool wasPresent = qEnvironmentVariableIsSet("PACKAGE_DISPLAY_EXECUTABLE");
    qputenv("PACKAGE_DISPLAY_EXECUTABLE", sentinel);
    const auto restoreEnvironment = qScopeGuard([&]() {
        if (wasPresent)
            qputenv("PACKAGE_DISPLAY_EXECUTABLE", previous);
        else
            qunsetenv("PACKAGE_DISPLAY_EXECUTABLE");
    });
    Context original = Context::createNew();
    original.workflow = "env-child";
    original.workdir = root.path();
    original.dockpipeBinary = QCoreApplication::applicationFilePath();
    original.transientLaunchEnv.insert("PACKAGE_DISPLAY_PROOF", root.filePath("proof.json"));
    Context invocation = original;
    PackageSurfaceActionMeta action;
    action.scope = "surface";
    action.workflowId = "env-child";
    action.launchEnvironment[PackageDisplayWidget::platformName()] = {
        {"PACKAGE_DISPLAY_REAL", {{}, "PACKAGE_DISPLAY_EXECUTABLE"}},
        {"PACKAGE_DISPLAY_EXECUTABLE", {asset, {}}}
    };
    QString error;
    QVERIFY2(SurfaceLaunchEnvironment::apply(invocation, action, root.path(), error), qPrintable(error));
    QCOMPARE(invocation.transientLaunchEnv.value("PACKAGE_DISPLAY_REAL").toUtf8(), sentinel);
    QCOMPARE(invocation.transientLaunchEnv.value("PACKAGE_DISPLAY_EXECUTABLE"), asset);
    QVERIFY(!QJsonDocument(invocation.toJson()).toJson().contains(sentinel));
    QVERIFY(!QJsonDocument(original.toJson()).toJson().contains(sentinel));
    SessionManager sessions;
    QVERIFY(sessions.launch(invocation, root.path()));
    QTRY_VERIFY_WITH_TIMEOUT(!sessions.isRunning(invocation.id), 5000);
    for (const QString &argument : sessions.info(invocation.id).arguments)
        QVERIFY(!argument.contains(QString::fromUtf8(sentinel)));
    QFile proof(root.filePath("proof.json"));
    QVERIFY(proof.open(QIODevice::ReadOnly));
    const auto verified = QJsonDocument::fromJson(proof.readAll()).object();
    QVERIFY(verified.value("private_inherited").toBool());
    QVERIFY(verified.value("asset_override").toBool());
    QVERIFY(verified.value("argv_private_free").toBool());
    QFile log(sessions.info(invocation.id).logPath);
    QVERIFY(log.open(QIODevice::ReadOnly));
    QVERIFY(!log.readAll().contains(sentinel));
    Context explicitValue = original;
    explicitValue.extraDockpipeEnv << "PACKAGE_DISPLAY_EXECUTABLE=";
    QVERIFY(SurfaceLaunchEnvironment::apply(explicitValue, action, root.path(), error));
    QVERIFY(explicitValue.transientLaunchEnv.contains("PACKAGE_DISPLAY_REAL"));
    QVERIFY(explicitValue.transientLaunchEnv.value("PACKAGE_DISPLAY_REAL").isEmpty());
    explicitValue = original;
    explicitValue.extraDockpipeEnv << "PACKAGE_DISPLAY_EXECUTABLE=operator-original";
    QVERIFY(SurfaceLaunchEnvironment::apply(explicitValue, action, root.path(), error));
    QCOMPARE(explicitValue.transientLaunchEnv.value("PACKAGE_DISPLAY_REAL"), QString("operator-original"));
    QVERIFY(explicitValue.extraDockpipeEnv.isEmpty());
    QVERIFY(!QJsonDocument(explicitValue.toJson()).toJson().contains("operator-original"));
    Context missingSource = original;
    missingSource.transientLaunchEnv.insert("PACKAGE_DISPLAY_REAL", "existing-reference");
    auto missingAction = action;
    missingAction.launchEnvironment[PackageDisplayWidget::platformName()]["PACKAGE_DISPLAY_REAL"].inherit = "PACKAGE_DISPLAY_SOURCE_NOT_PRESENT";
    QVERIFY(SurfaceLaunchEnvironment::apply(missingSource, missingAction, root.path(), error));
    QCOMPARE(missingSource.transientLaunchEnv.value("PACKAGE_DISPLAY_REAL"), QString("existing-reference"));
    Context otherPlatform = original;
    auto platformAction = action;
    platformAction.launchEnvironment.clear();
    QString otherPlatformName = "linux";
    if (PackageDisplayWidget::platformName() == "linux")
        otherPlatformName = "windows";
    platformAction.launchEnvironment[otherPlatformName] = action.launchEnvironment.value(PackageDisplayWidget::platformName());
    QVERIFY(SurfaceLaunchEnvironment::apply(otherPlatform, platformAction, root.path(), error));
    QCOMPARE(otherPlatform.transientLaunchEnv, original.transientLaunchEnv);
    action.launchEnvironment[PackageDisplayWidget::platformName()]["PACKAGE_DISPLAY_EXECUTABLE"].asset = QCoreApplication::applicationFilePath();
    QVERIFY(!SurfaceLaunchEnvironment::apply(explicitValue, action, root.path(), error));
}

void PackageDisplayTest::optionsBounds()
{
    QString error;
    QVERIFY(PackageDisplayWidget::validateOptions({{"resource_id", "public-reference"}}, error));
    QVERIFY(!PackageDisplayWidget::validateOptions({{"value", QString(4097, 'x')}}, error));
    QVERIFY(!PackageDisplayWidget::validateOptions({{QString(65, 'k'), true}}, error));
    QJsonObject deep{{"value", true}};
    for (int depth = 0; depth < 9; ++depth)
        deep = {{"child", deep}};
    QVERIFY(!PackageDisplayWidget::validateOptions(deep, error));
    QJsonArray many;
    for (int index = 0; index < 128; ++index)
        many.append(true);
    QVERIFY(!PackageDisplayWidget::validateOptions({{"values", many}}, error));
    QJsonObject large;
    for (int index = 0; index < 5; ++index)
        large.insert(QString::number(index), QString(4000, 'x'));
    QVERIFY(!PackageDisplayWidget::validateOptions(large, error));
}

void PackageDisplayTest::installedConsumerLeaseAndCompatibility()
{
    const QString root = qEnvironmentVariable("DOCKPIPE_DISPLAY_FIXTURE_ROOT");
    if (root.isEmpty())
        QSKIP("Run the explicit installed SDK consumer fixture for native plugin proof.");
    const QString library = QDir(root).filePath("assets/libdisplay-good.so");
    QPluginLoader observer(library);
    observer.setLoadHints(QLibrary::PreventUnloadHint);
    auto *plugin = observer.instance();
    QVERIFY2(plugin, qPrintable(observer.errorString()));
    PackageDisplayMeta display;
    display.id = "resource";
    display.leaseId = "resource";
    display.nativePackage = {1, root, {{"linux", library}}};
    display.options = {{"resource_id", "resource"}};
    PackageDisplayWidget page;
    PackageDisplayWidget panel;
    page.resize(400, 300);
    panel.resize(400, 300);
    page.setDisplay(display);
    page.show();
    QTRY_VERIFY(page.hasPackageSurface());
    QCOMPARE(plugin->property("liveViews").toInt(), 1);
    auto *surface = page.findChild<QWidget *>("installedConsumerDisplay");
    QVERIFY(surface);
    QCOMPARE(surface->property("resourceId").toString(), QString("resource"));
    QVERIFY(surface->property("active").toBool());
    panel.setDisplay(display);
    panel.show();
    QTRY_VERIFY(panel.hasPackageSurface());
    QVERIFY(!page.hasPackageSurface());
    QCOMPARE(plugin->property("liveViews").toInt(), 1);
    // Background snapshots must not steal ownership from the explicit new view.
    page.setDisplay(display);
    QVERIFY(!page.hasPackageSurface());
    auto refreshed = display;
    refreshed.options.insert("state_ref", "refreshed-public-reference");
    page.setDisplay(refreshed);
    QVERIFY(!page.hasPackageSurface());
    QVERIFY(panel.hasPackageSurface());
    panel.hide();
    QTRY_VERIFY(!panel.hasPackageSurface());
    QCOMPARE(plugin->property("liveViews").toInt(), 0);
    page.hide();
    page.show();
    QTRY_VERIFY(page.hasPackageSurface());
    page.clearDisplay();
    QCOMPARE(plugin->property("liveViews").toInt(), 0);
    QVERIFY(plugin->property("closedViews").toInt() >= 3);
    for (const QString &variant : {QString("wrong-qt"), QString("wrong-compiler"), QString("wrong-abi"), QString("wrong-iid")}) {
        display.nativePackage.libraries["linux"] = QDir(root).filePath("assets/libdisplay-" + variant + ".so");
        QSignalSpy failures(&page, &PackageDisplayWidget::displayFailed);
        page.setDisplay(display);
        QVERIFY(!page.hasPackageSurface());
        QCOMPARE(failures.size(), 1);
    }
    QTemporaryDir outside;
    QVERIFY(outside.isValid());
    display.nativePackage.packageRoot = outside.path();
    QSignalSpy escaped(&page, &PackageDisplayWidget::displayFailed);
    page.setDisplay(display);
    QCOMPARE(escaped.size(), 1);
    QVERIFY(!page.hasPackageSurface());
}

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    if (application.arguments().contains("--workflow")) {
        const QString sentinel = "inherit-private-sentinel-43";
        QJsonObject proof{
            {"private_inherited", qEnvironmentVariable("PACKAGE_DISPLAY_REAL") == sentinel},
            {"asset_override", QFileInfo(qEnvironmentVariable("PACKAGE_DISPLAY_EXECUTABLE")).fileName().startsWith("fixture-tool")},
            {"argv_private_free", !application.arguments().join(' ').contains(sentinel)}
        };
        QFile output(qEnvironmentVariable("PACKAGE_DISPLAY_PROOF"));
        if (!output.open(QIODevice::WriteOnly))
            return 2;
        output.write(QJsonDocument(proof).toJson());
        return 0;
    }
    PackageDisplayTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "PackageDisplayTest.moc"
