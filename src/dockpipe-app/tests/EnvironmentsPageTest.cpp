#include "EnvironmentsPage.h"
#include "ResourceCanvas.h"

#include <QApplication>
#include <QDateTime>
#include <QDesktopServices>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QGraphicsItem>
#include <QGraphicsScene>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QTabBar>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeWidget>
#include <QUrl>
#include <cstdio>

namespace {

const QString demoInstanceId(32, QLatin1Char('a'));
const QString demoContainerId(64, QLatin1Char('b'));

int definitionsFixture(const QStringList &arguments)
{
    if (arguments.size() < 4 || arguments.at(1) != "environment" || arguments.at(2) != "definitions"
        || arguments.at(3) != "--observe")
        return 2;

    const auto directory = QDir::current();
    QJsonArray entries{QJsonObject{
        {"scope", "personal"},
        {"path", directory.absoluteFilePath("demo-vm.yaml")},
        {"instance_id", demoInstanceId},
        {"definition", QJsonObject{{"schema", "dockpipe.environment/v1"}, {"name", "demo-vm"}, {"kind", "vm"},
            {"provision", QJsonObject{{"workflow", "linux-vm"}, {"package", "qemu"}}},
            {"storage", QJsonObject{{"profile", "private-cache"}}}}},
    }, QJsonObject{
        {"scope", "personal"},
        {"path", directory.absoluteFilePath("bad.yaml")},
        {"error", "invalid"},
    }, QJsonObject{
        {"scope", "personal"},
        {"path", directory.absoluteFilePath("dev-pod.yaml")},
        {"error", "invalid"},
        {"reason", "unsupported-kind"},
    }};
    if (directory.dirName() == "duplicate") {
        auto duplicate = entries.first().toObject();
        duplicate["scope"] = "explicit";
        duplicate["path"] = directory.absoluteFilePath("other-vm.yaml");
        entries.append(duplicate);
    }
    if (directory.dirName() == "vm-observed" || directory.dirName() == "container-observed"
        || directory.dirName() == "container-running") {
        auto vm = entries.first().toObject();
        vm["runtime"] = QJsonObject{{"schema", "dockpipe.environment.runtime/v1"},
            {"instance_id", demoInstanceId}, {"state", "stopped"}};
        if (directory.dirName() != "vm-observed") {
            auto definition = vm.value("definition").toObject();
            definition["kind"] = QStringLiteral("container");
            definition.remove("storage");
            definition.remove("provision");
            definition["container"] = QJsonObject{{"image", "ubuntu:24.04"}, {"network", "none"}, {"security", "restricted"}};
            vm["definition"] = definition;
            if (directory.dirName() == "container-running") {
                vm["runtime"] = QJsonObject{{"schema", "dockpipe.environment.runtime/v1"},
                    {"instance_id", demoInstanceId}, {"state", "running"}, {"container_id", demoContainerId}};
            }
        } else {
            auto definition = vm.value("definition").toObject();
            definition.remove("storage");
            definition.remove("provision");
            definition["vm"] = QJsonObject{{"os", "ubuntu"}, {"cpus", 3}, {"memory_gib", 2}, {"disk_gib", 9}};
            vm["definition"] = definition;
        }
        entries[0] = vm;
    }
    for (qsizetype option = 4; option < arguments.size(); option += 2) {
        if (arguments.at(option) != "--file" || option + 1 >= arguments.size())
            return 2;

        entries.append(QJsonObject{{"scope", "explicit"}, {"path", arguments.at(option + 1)}, {"error", "missing"}});
    }
    const QJsonDocument result(QJsonObject{
        {"schema", "dockpipe.environment.definitions/v1"},
        {"definitions", entries},
    });
    const auto payload = result.toJson(QJsonDocument::Compact);
    std::fwrite(payload.constData(), 1, size_t(payload.size()), stdout);
    return 0;
}

}

class EnvironmentsPageTest final : public QObject {
    Q_OBJECT

private slots:
    void hostsAndExistingVmsShareTheInventory()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        EnvironmentsPage page;
        page.setWorkspace(workspace.path());
        page.setActive(true);
        page.setVirtualMachineControls(new QWidget);
        page.setMachineInventory({
            {"@local", "computer", "This computer", {}, {}, 0},
            {"remote-host", "machine", "Remote host", {}, "Paired", 1},
        }, {{"@local", "remote-host", "Paired"}});
        page.setVirtualMachineInventory({
            QJsonObject{{"id", "1234abcd"}, {"title", "Existing VM"}, {"state", "running"}},
            QJsonObject{{"id", "2345abcd"}, {"title", "Saved VM duplicate"},
                {"state", "running"}, {"instance_id", demoInstanceId}},
        });

        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        auto *canvas = page.findChild<ResourceCanvas *>();
        QVERIFY(list && canvas);
        QTRY_COMPARE(list->topLevelItemCount(), 5);
        QCOMPARE(canvas->snapshot().value("nodes").toArray().size(), 5);
        QCOMPARE(canvas->snapshot().value("edges").toArray().size(), 2);
        QVERIFY(canvas->focusResource("vm:1234abcd"));
        QCOMPARE(list->currentItem()->text(0), QString("Existing VM"));
        auto *controls = page.findChild<QPushButton *>("environmentResourceControls");
        QVERIFY(controls);
        QCOMPARE(controls->text(), QString("Open VM console"));
        QString requested;
        connect(&page, &EnvironmentsPage::resourceControlsRequested, this,
            [&](const QString &kind, const QString &id, const QString &) { requested = kind + ":" + id; });
        controls->click();
        QCOMPARE(requested, QString("vm:1234abcd"));
    }

    void exactOwnerObservationIsShared_data()
    {
        QTest::addColumn<QString>("fixture");
        QTest::newRow("vm") << QString("vm-observed");
        QTest::newRow("container") << QString("container-observed");
    }

    void exactOwnerObservationIsShared()
    {
        QFETCH(QString, fixture);
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        QDir directory(workspace.path());
        QVERIFY(directory.mkdir(fixture));
        EnvironmentsPage page;
        page.setWorkspace(directory.absoluteFilePath(fixture));
        page.setActive(true);

        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        auto *canvas = page.findChild<ResourceCanvas *>();
        QVERIFY(list && canvas);
        QTRY_COMPARE(list->topLevelItemCount(), 3);
        QCOMPARE(list->topLevelItem(0)->text(3), QString("Observed stopped"));
        const auto graph = canvas->snapshot();
        QCOMPARE(graph.value("nodes").toArray().first().toObject().value("state").toString(),
            QString("Observed stopped"));
        QCOMPARE(graph.value("edges").toArray().size(), 1);

        page.setVmObservations({{demoInstanceId, "running"}}, QDateTime::currentMSecsSinceEpoch());
        QCOMPARE(list->topLevelItem(0)->text(3), QString("Observed stopped"));
    }

    void canvasAndListShareSelection()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        EnvironmentsPage page;
        page.setWorkspace(workspace.path());
        page.setActive(true);

        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        auto *canvas = page.findChild<ResourceCanvas *>();
        auto *tabs = page.findChild<QTabBar *>("environmentViewTabs");
        auto *details = page.findChild<QLabel *>("environmentListDefinition");
        auto *apply = page.findChild<QPushButton *>("applyEnvironmentDefinition");
        QVERIFY(list && canvas && tabs && details && apply);

        QTRY_COMPARE(list->topLevelItemCount(), 2);
        QCOMPARE(canvas->snapshot().value("nodes").toArray().size(), 2);
        QCOMPARE(list->topLevelItem(0)->text(3), QStringLiteral("Runtime unavailable"));
        QCOMPARE(list->topLevelItem(1)->text(3), QStringLiteral("Invalid definition"));
        QVERIFY(canvas->focusResource(QDir(workspace.path()).absoluteFilePath("demo-vm.yaml")));
        QCOMPARE(list->currentItem(), list->topLevelItem(0));
        QVERIFY(details->text().contains("linux-vm"));
        QVERIFY(details->text().contains("private-cache"));
        const auto path = QDir(workspace.path()).absoluteFilePath("demo-vm.yaml");
        QString requested;
        connect(&page, &EnvironmentsPage::applyRequested, this, [&](const EnvironmentDefinition &definition) {
            requested = definition.path;
        });
        QVERIFY(apply->isEnabled());
        apply->click();
        QCOMPARE(requested, path);
        page.setApplying(path, true);
        QVERIFY(!apply->isEnabled());
        page.setApplying(path, false);
        QVERIFY(apply->isEnabled());

        tabs->setCurrentIndex(1);
        list->setCurrentItem(list->topLevelItem(1));
        QVERIFY(!apply->isEnabled());
        QVERIFY(details->text().contains("Invalid definition"));
        QVERIFY(!details->text().contains("private-cache"));
        tabs->setCurrentIndex(0);
        auto *graph = canvas->findChild<QGraphicsView *>("environmentsGraph");
        QVERIFY(graph);
        QCOMPARE(graph->scene()->selectedItems().size(), 1);
        QCOMPARE(graph->scene()->selectedItems().first()->data(0).toString(),
            QDir(workspace.path()).absoluteFilePath("bad.yaml"));
    }

    void runningContainerTerminalUsesExactRuntimeIdentity()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        QDir directory(workspace.path());
        QVERIFY(directory.mkdir("container-running"));
        EnvironmentsPage page;
        page.setWorkspace(directory.absoluteFilePath("container-running"));
        page.setActive(true);

        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        QVERIFY(list);
        QTRY_COMPARE(list->topLevelItem(0) ? list->topLevelItem(0)->text(3) : QString(), QString("Observed running"));
        list->setCurrentItem(list->topLevelItem(0));
        auto *terminal = page.findChild<QPushButton *>("environmentResourceControls");
        QVERIFY(terminal && terminal->isEnabled());
        QCOMPARE(terminal->text(), QString("Open terminal"));
        QString requested;
        connect(&page, &EnvironmentsPage::resourceControlsRequested, this,
            [&](const QString &kind, const QString &id, const QString &) { requested = kind + ":" + id; });
        terminal->click();
        QCOMPARE(requested, QString("container:") + demoContainerId);
    }

    void explicitPathsArePassedAsOneArgument()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        const auto definitionPath = QDir(workspace.path()).absoluteFilePath("definition with spaces.yml");
        EnvironmentDefinitionsClient client;
        QVector<EnvironmentDefinition> definitions;
        connect(&client, &EnvironmentDefinitionsClient::loaded, this, [&](const auto &loaded) {
            definitions = loaded;
        });
        client.load(workspace.path(), {definitionPath});
        QTRY_COMPARE(definitions.size(), 3);
        QCOMPARE(definitions.last().path, definitionPath);
        QCOMPARE(definitions.last().scope, QStringLiteral("explicit"));
        QCOMPARE(definitions.last().error, QStringLiteral("missing"));
    }

    void editorOpensOnlyTheSelectedYaml()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        QFile source(QDir(workspace.path()).absoluteFilePath("bad.yaml"));
        QVERIFY(source.open(QIODevice::WriteOnly));
        QVERIFY(source.write("invalid saved definition\n") > 0);
        source.close();

        EnvironmentsPage page;
        page.setWorkspace(workspace.path());
        page.setActive(true);
        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        auto *edit = page.findChild<QPushButton *>("editEnvironmentDefinition");
        QVERIFY(list && edit);
        QTRY_COMPARE(list->topLevelItemCount(), 2);
        QVERIFY(!edit->isEnabled());
        list->setCurrentItem(list->topLevelItem(1));
        QVERIFY(edit->isEnabled());

        QDesktopServices::setUrlHandler("file", this, "captureOpen");
        edit->click();
        QDesktopServices::unsetUrlHandler("file");
        QCOMPARE(m_opened, QUrl::fromLocalFile(QFileInfo(source).canonicalFilePath()));
    }

    void savedIdentityRequiresFreshUnambiguousProviderObservation()
    {
        QTemporaryDir workspace;
        QVERIFY(workspace.isValid());
        EnvironmentsPage page;
        page.setWorkspace(workspace.path());
        page.setActive(true);
        auto *list = page.findChild<QTreeWidget *>("environmentDefinitionsList");
        auto *canvas = page.findChild<ResourceCanvas *>();
        QVERIFY(list && canvas);
        QTRY_COMPARE(list->topLevelItemCount(), 2);

        const auto verifyVmState = [&](const QString &expected) {
            QCOMPARE(list->topLevelItem(0)->text(3), expected);
            QCOMPARE(canvas->snapshot().value("nodes").toArray().first().toObject().value("state").toString(), expected);
        };
        verifyVmState("Runtime unavailable");
        page.setVmObservations({{QString(32, 'b'), "running"}}, QDateTime::currentMSecsSinceEpoch());
        verifyVmState("Runtime unavailable");
        page.setVmObservations({{demoInstanceId, "running"}}, QDateTime::currentMSecsSinceEpoch());
        verifyVmState("Observed running");
        page.setVmObservations({{demoInstanceId, "starting"}}, QDateTime::currentMSecsSinceEpoch());
        verifyVmState("Runtime unavailable");
        page.setVmObservations({{demoInstanceId, "stopped"}}, QDateTime::currentMSecsSinceEpoch() - 14000);
        verifyVmState("Observed stopped");
        QTRY_COMPARE_WITH_TIMEOUT(list->topLevelItem(0)->text(3), QStringLiteral("Runtime unavailable"), 2500);
        verifyVmState("Runtime unavailable");

        const auto duplicate = QDir(workspace.path()).absoluteFilePath("duplicate");
        QVERIFY(QDir().mkpath(duplicate));
        page.setWorkspace(duplicate);
        page.setVmObservations({{demoInstanceId, "running"}}, QDateTime::currentMSecsSinceEpoch());
        QTRY_COMPARE(list->topLevelItemCount(), 3);
        verifyVmState("Runtime unavailable");
        QCOMPARE(list->topLevelItem(2)->text(3), QStringLiteral("Runtime unavailable"));
    }

    void captureOpen(const QUrl &url)
    {
        m_opened = url;
    }

private:
    QUrl m_opened;
};

int main(int argc, char **argv)
{
    if (argc > 1 && QByteArray(argv[1]) == "environment") {
        QCoreApplication application(argc, argv);
        return definitionsFixture(application.arguments());
    }

    QApplication application(argc, argv);
    qputenv("DOCKPIPE_BIN", QFile::encodeName(QCoreApplication::applicationFilePath()));
    EnvironmentsPageTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "EnvironmentsPageTest.moc"
