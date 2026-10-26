#include "DashboardStore.h"

#include <QCoreApplication>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLockFile>
#include <QStandardPaths>
#include <QTemporaryDir>
#include <QtTest>

class DashboardStoreTest final : public QObject {
    Q_OBJECT
private slots:
    void init();
    void roundTripAndConflicts();
    void defaultDashboardPreservesCustomDashboards();
    void corruptionAndBounds();
    void legacyMigration_data();
    void legacyMigration();
    void prunesLegacyLayoutsForIndependentEnvironments();
    void optionsRoundTripAndSafeDefaults();
    void invalidV2Fields_data();
    void invalidV2Fields();
};

namespace {
bool writeStore(const QJsonObject &object)
{
    QFile file(DashboardStore::filePath());
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return false;
    const QByteArray bytes = QJsonDocument(object).toJson();
    return file.write(bytes) == bytes.size();
}

QByteArray storeBytes()
{
    QFile file(DashboardStore::filePath());
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.readAll();
}

QJsonObject tabNode(const QJsonArray &ids, const QString &active)
{
    return {{"type", "tabs"}, {"ids", ids}, {"active", active}};
}
}

void DashboardStoreTest::init()
{
    const QString path = DashboardStore::filePath();
    if (QFile::exists(path))
        QVERIFY(QFile::remove(path));
}

void DashboardStoreTest::defaultDashboardPreservesCustomDashboards()
{
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY(store.load(definitions, error));
    for (int index = 0; index < 12; ++index)
        definitions.append({DashboardStore::newId(), QString("Custom %1").arg(index), {}, {}});
    const auto customId = definitions.first().id;
    QVERIFY(DashboardStore::ensureDefaultDashboard(definitions));
    QCOMPARE(definitions.size(), 13);
    QCOMPARE(definitions.first().id, DashboardStore::defaultDashboardId());
    QCOMPARE(definitions.first().name, QString("Dashboard"));
    QCOMPARE(definitions[1].id, customId);
    QVERIFY(!DashboardStore::ensureDefaultDashboard(definitions));
    QVERIFY(store.save(definitions, error));
    QVERIFY(store.load(definitions, error));
    QVERIFY(!DashboardStore::ensureDefaultDashboard(definitions));
    definitions.first().panels.append({DashboardStore::newId(), "files"});
    QVERIFY(store.save(definitions, error));
    QVERIFY(store.load(definitions, error));
    QCOMPARE(definitions.first().panels.size(), 1);
    definitions.append({DashboardStore::newId(), "Too many", {}, {}});
    QVERIFY(!store.save(definitions, error));
}

void DashboardStoreTest::roundTripAndConflicts()
{
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY(store.load(definitions, error));
    QVERIFY(definitions.isEmpty());
    DashboardDefinition definition{DashboardStore::newId(), "Operations", {{DashboardStore::newId(), "browser"}},
        QJsonObject{{"version", 1}, {"mode", "tiling"}}};
    definitions.append(definition);
    QVERIFY(store.save(definitions, error));
    DashboardStore restored;
    QVector<DashboardDefinition> readback;
    QVERIFY(restored.load(readback, error));
    QCOMPARE(readback.size(), 1);
    QCOMPARE(readback.first().id, definition.id);
    QCOMPARE(readback.first().panels.first().id, definition.panels.first().id);
    QCOMPARE(readback.first().layout, definition.layout);
    readback.first().name = "Renamed";
    QVERIFY(restored.save(readback, error));
    QVERIFY(!store.save(definitions, error));
    QVERIFY(error.contains("changed outside"));
    QVERIFY(store.load(definitions, error));
    QCOMPARE(definitions.first().name, QString("Renamed"));
    {
        QLockFile lock(DashboardStore::filePath() + ".lock");
        QVERIFY(lock.tryLock());
        QVERIFY(!store.save(definitions, error));
    }
    QVERIFY(store.save(definitions, error));
}

void DashboardStoreTest::corruptionAndBounds()
{
    QVector<DashboardDefinition> definitions;
    QString error;
    DashboardStore store;
    QVERIFY(store.load(definitions, error));
    definitions.append({DashboardStore::newId(), "Bounds", {{DashboardStore::newId(), "browser"}}, {}});
    QVERIFY(store.save(definitions, error));
    auto invalid = definitions;
    invalid.append(invalid.first());
    QVERIFY(!store.save(invalid, error));
    invalid = definitions;
    invalid.first().panels.append(invalid.first().panels.first());
    QVERIFY(!store.save(invalid, error));
    invalid = definitions;
    invalid.first().name = QString(81, 'x');
    QVERIFY(!store.save(invalid, error));
    invalid = definitions;
    invalid.first().layout = QJsonObject{{"oversized", QString(1024 * 1024, 'x')}};
    QVERIFY(!store.save(invalid, error));
    QFile file(DashboardStore::filePath());
    QVERIFY(file.open(QIODevice::WriteOnly | QIODevice::Truncate));
    const QByteArray malformed("{preserve malformed user data");
    QCOMPARE(file.write(malformed), malformed.size());
    file.close();
    QVERIFY(!store.load(definitions, error));
    QVERIFY(!store.save({}, error));
    QVERIFY(file.open(QIODevice::ReadOnly));
    QCOMPARE(file.readAll(), malformed);
}

void DashboardStoreTest::legacyMigration_data()
{
    QTest::addColumn<QString>("oldMode");
    QTest::addColumn<QString>("ordinaryEnvironment");
    QTest::newRow("canvas-with-native-app") << QString("canvas") << QString("canvas");
    QTest::newRow("tiling") << QString("tiling") << QString("tiling");
    QTest::newRow("missing-mode-defaults-canvas") << QString() << QString("canvas");
}

void DashboardStoreTest::legacyMigration()
{
    QFETCH(QString, oldMode);
    QFETCH(QString, ordinaryEnvironment);
    const QString dashboardId = DashboardStore::newId();
    const QString browserId = DashboardStore::newId();
    const QString nativeId = DashboardStore::newId();
    QJsonObject layout{{"version", 1}};
    if (!oldMode.isEmpty())
        layout["mode"] = oldMode;
    const QJsonObject dashboard{
        {"id", dashboardId}, {"name", "Existing dashboard"}, {"layout", layout},
        {"panels", QJsonArray{
            QJsonObject{{"id", browserId}, {"kind", "browser"}},
            QJsonObject{{"id", nativeId}, {"kind", "desktop-window"}}
        }}
    };
    QVERIFY(writeStore({{"version", 1}, {"dashboards", QJsonArray{dashboard}}}));
    const QByteArray original = storeBytes();
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY2(store.load(definitions, error), qPrintable(error));
    QCOMPARE(definitions.size(), 1);
    const auto &loaded = definitions.first();
    QCOMPARE(loaded.id, dashboardId);
    QCOMPARE(loaded.layout, layout);
    QCOMPARE(loaded.panels.size(), 2);
    QCOMPARE(loaded.panels.at(0).id, browserId);
    QCOMPARE(loaded.panels.at(0).environment, ordinaryEnvironment);
    QCOMPARE(loaded.panels.at(1).id, nativeId);
    QCOMPARE(loaded.panels.at(1).environment, QString("tiling"));
    QVERIFY(!loaded.options.autoAcquireApps);
    QCOMPARE(storeBytes(), original); // Reading a legacy store never rewrites it.
    QVERIFY2(store.save(definitions, error), qPrintable(error));
    const auto persisted = QJsonDocument::fromJson(storeBytes()).object();
    QCOMPARE(persisted.value("version").toInt(), 2);
    const auto savedPanels = persisted.value("dashboards").toArray().first().toObject().value("panels").toArray();
    QCOMPARE(savedPanels.at(0).toObject().value("id").toString(), browserId);
    QCOMPARE(savedPanels.at(0).toObject().value("environment").toString(), ordinaryEnvironment);
    QCOMPARE(savedPanels.at(1).toObject().value("id").toString(), nativeId);
    QCOMPARE(savedPanels.at(1).toObject().value("environment").toString(), QString("tiling"));
    DashboardStore reloaded;
    QVERIFY(reloaded.load(definitions, error));
    QCOMPARE(definitions.first().panels.at(0).environment, ordinaryEnvironment);
    QCOMPARE(definitions.first().panels.at(1).environment, QString("tiling"));
}

void DashboardStoreTest::prunesLegacyLayoutsForIndependentEnvironments()
{
    const QString first = DashboardStore::newId();
    const QString second = DashboardStore::newId();
    const QString tiled = DashboardStore::newId();
    const QString native = DashboardStore::newId();
    const QJsonObject geometry{
        {first, QJsonArray{8, 8, 440, 300}}, {second, QJsonArray{464, 8, 440, 300}},
        {tiled, QJsonArray{8, 324, 440, 300}}, {native, QJsonArray{464, 324, 440, 300}}
    };
    const QJsonObject tree{
        {"type", "split"}, {"orientation", "horizontal"}, {"sizes", QJsonArray{300, 500}},
        {"children", QJsonArray{tabNode(QJsonArray{first, tiled}, tiled), tabNode(QJsonArray{second}, second)}}
    };
    DashboardDefinition definition{
        DashboardStore::newId(), "Mixed legacy layout",
        {{first, "browser", "canvas"}, {second, "metrics", "canvas"},
         {tiled, "terminal", "tiling"}, {native, "desktop-window", "tiling"}},
        {{"version", 1}, {"mode", "canvas"}, {"canvas", geometry}, {"floatingGeometry", geometry},
         {"canvasOrder", QJsonArray{first, tiled, second, native}}, {"floating", QJsonArray{native}}, {"tree", tree}}
    };
    const QJsonObject original = definition.layout;
    const QJsonObject canvas = DashboardStore::layoutForEnvironment(definition, "canvas");
    const QJsonObject tiles = DashboardStore::layoutForEnvironment(definition, "tiling");
    QCOMPARE(definition.layout, original);
    QCOMPARE(canvas.value("mode").toString(), QString("canvas"));
    QCOMPARE(tiles.value("mode").toString(), QString("tiling"));
    QCOMPARE(canvas.value("canvasOrder").toArray(), QJsonArray({first, second}));
    QCOMPARE(tiles.value("canvasOrder").toArray(), QJsonArray({tiled, native}));
    QVERIFY(canvas.value("floating").toArray().isEmpty());
    QCOMPARE(tiles.value("floating").toArray(), QJsonArray({native}));
    for (const QString &key : {QString("canvas"), QString("floatingGeometry")}) {
        QCOMPARE(canvas.value(key).toObject().size(), 2);
        QVERIFY(canvas.value(key).toObject().contains(first));
        QVERIFY(canvas.value(key).toObject().contains(second));
        QCOMPARE(tiles.value(key).toObject().size(), 2);
        QVERIFY(tiles.value(key).toObject().contains(tiled));
        QVERIFY(tiles.value(key).toObject().contains(native));
    }
    const auto canvasTree = canvas.value("tree").toObject();
    QCOMPARE(canvasTree.value("type").toString(), QString("split"));
    QCOMPARE(canvasTree.value("sizes").toArray(), QJsonArray({300, 500}));
    const auto children = canvasTree.value("children").toArray();
    QCOMPARE(children.at(0).toObject(), tabNode(QJsonArray{first}, first));
    QCOMPARE(children.at(1).toObject(), tabNode(QJsonArray{second}, second));
    QCOMPARE(tiles.value("tree").toObject(), tabNode(QJsonArray{tiled}, tiled));

    definition.layout = {{"version", 2}, {"canvas", canvas}, {"tiling", tiles}};
    QCOMPARE(DashboardStore::layoutForEnvironment(definition, "canvas"), canvas);
    QCOMPARE(DashboardStore::layoutForEnvironment(definition, "tiling"), tiles);
    definition.layout = {};
    QVERIFY(DashboardStore::layoutForEnvironment(definition, "canvas").isEmpty());
}

void DashboardStoreTest::optionsRoundTripAndSafeDefaults()
{
    const QJsonObject dashboard{
        {"id", DashboardStore::newId()}, {"name", "Preferences"}, {"layout", QJsonObject{}},
        {"panels", QJsonArray{QJsonObject{{"id", DashboardStore::newId()}, {"kind", "browser"}, {"environment", "canvas"}}}}
    };
    // Version 2 files may omit options; absence must never opt into app capture.
    QVERIFY(writeStore({{"version", 2}, {"dashboards", QJsonArray{dashboard}}}));
    DashboardStore store;
    QVector<DashboardDefinition> definitions;
    QString error;
    QVERIFY(store.load(definitions, error));
    QCOMPARE(definitions.size(), 1);
    QVERIFY(definitions.first().options.compactTiling);
    QVERIFY(definitions.first().options.autoHideDock);
    QVERIFY(definitions.first().options.showPanelHeaders);
    QVERIFY(!definitions.first().options.autoAcquireApps);
    QCOMPARE(definitions.first().options.canvasBackground, QString("grid"));
    QCOMPARE(definitions.first().options.canvasFileScope, QString("project"));
    QCOMPARE(definitions.first().options.tilingFileScope, QString("system"));
    QVERIFY(store.save(definitions, error));
    DashboardStore restored;
    QVERIFY(restored.load(definitions, error));
    QVERIFY(!definitions.first().options.autoAcquireApps);
    auto &options = definitions.first().options;
    options.compactTiling = false;
    options.autoHideDock = false;
    options.showPanelHeaders = false;
    options.autoAcquireApps = true;
    options.canvasBackground = "image";
    options.canvasImagePath = "/tmp/user-background.png";
    options.canvasFileScope = "system";
    options.tilingFileScope = "project";
    QVERIFY(restored.save(definitions, error));
    DashboardStore readback;
    QVERIFY(readback.load(definitions, error));
    const auto &saved = definitions.first().options;
    QVERIFY(!saved.compactTiling);
    QVERIFY(!saved.autoHideDock);
    QVERIFY(!saved.showPanelHeaders);
    QVERIFY(saved.autoAcquireApps);
    QCOMPARE(saved.canvasBackground, QString("image"));
    QCOMPARE(saved.canvasImagePath, QString("/tmp/user-background.png"));
    QCOMPARE(saved.canvasFileScope, QString("system"));
    QCOMPARE(saved.tilingFileScope, QString("project"));
    auto invalidScope = definitions;
    invalidScope.first().options.canvasFileScope = "unknown";
    QVERIFY(!readback.save(invalidScope, error));
    invalidScope = definitions;
    invalidScope.first().options.tilingFileScope = "unknown";
    QVERIFY(!readback.save(invalidScope, error));
    auto invalid = definitions;
    invalid.first().options.canvasBackground = "unsupported";
    QVERIFY(!readback.save(invalid, error));
    invalid = definitions;
    invalid.first().options.canvasImagePath.clear();
    QVERIFY(!readback.save(invalid, error));
    invalid = definitions;
    invalid.first().options.canvasImagePath = QString(4097, 'x');
    QVERIFY(!readback.save(invalid, error));
    invalid = definitions;
    invalid.first().panels.first().environment = "both";
    QVERIFY(!readback.save(invalid, error));
    invalid = definitions;
    invalid.first().panels.first().kind = "desktop-window";
    QVERIFY(!readback.save(invalid, error));
    definitions.first().options.canvasBackground = "desktop";
    definitions.first().options.canvasImagePath.clear();
    QVERIFY(readback.save(definitions, error));
    DashboardStore desktopReadback;
    QVERIFY(desktopReadback.load(definitions, error));
    QCOMPARE(definitions.first().options.canvasBackground, QString("desktop"));
    QVERIFY(definitions.first().options.canvasImagePath.isEmpty());
}

void DashboardStoreTest::invalidV2Fields_data()
{
    QTest::addColumn<QString>("field");
    QTest::addColumn<QJsonValue>("value");
    QTest::newRow("auto-acquire-is-boolean") << QString("autoAcquireApps") << QJsonValue("true");
    QTest::newRow("dock-autohide-is-boolean") << QString("autoHideDock") << QJsonValue(1);
    QTest::newRow("compact-is-boolean") << QString("compactTiling") << QJsonValue(1);
    QTest::newRow("headers-is-boolean") << QString("showPanelHeaders") << QJsonValue(QJsonValue::Null);
    QTest::newRow("canvas-scope-is-string") << QString("canvasFileScope") << QJsonValue(false);
    QTest::newRow("tiling-scope-is-string") << QString("tilingFileScope") << QJsonValue(5);
    QTest::newRow("background-is-string") << QString("canvasBackground") << QJsonValue(false);
    QTest::newRow("path-is-string") << QString("canvasImagePath") << QJsonValue(1);
    QTest::newRow("options-is-object") << QString("options") << QJsonValue(QJsonArray{});
    QTest::newRow("environment-required") << QString("environment") << QJsonValue(QJsonValue::Undefined);
    QTest::newRow("environment-is-string") << QString("environment") << QJsonValue(true);
    QTest::newRow("environment-is-known") << QString("environment") << QJsonValue("external");
}

void DashboardStoreTest::invalidV2Fields()
{
    QFETCH(QString, field);
    QFETCH(QJsonValue, value);
    QJsonObject panel{{"id", DashboardStore::newId()}, {"kind", "browser"}, {"environment", "canvas"}};
    QJsonObject options;
    if (field == "environment")
        panel["environment"] = value;
    else if (field != "options")
        options[field] = value;
    QJsonObject dashboard{
        {"id", DashboardStore::newId()}, {"name", "Invalid settings"},
        {"panels", QJsonArray{panel}}, {"layout", QJsonObject{}},
        {"options", field == "options" ? value : QJsonValue(options)}
    };
    QVERIFY(writeStore({{"version", 2}, {"dashboards", QJsonArray{dashboard}}}));
    const QByteArray original = storeBytes();
    DashboardStore store;
    QVector<DashboardDefinition> definitions{{DashboardStore::newId(), "Retained caller state", {}, {}}};
    const QString retainedId = definitions.first().id;
    QString error;
    QVERIFY(!store.load(definitions, error));
    QVERIFY(!error.isEmpty());
    QCOMPARE(definitions.size(), 1);
    QCOMPARE(definitions.first().id, retainedId);
    QVERIFY(!store.save({}, error));
    QCOMPARE(storeBytes(), original);
}

int main(int argc, char **argv)
{
    QTemporaryDir config;
    if (!config.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("APPDATA", config.path().toUtf8());
    QCoreApplication app(argc, argv);
    QCoreApplication::setApplicationName("DashboardStoreTest");
    if (!QStandardPaths::writableLocation(QStandardPaths::AppConfigLocation).startsWith(config.path() + '/'))
        return 77;
    DashboardStoreTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardStoreTest.moc"
