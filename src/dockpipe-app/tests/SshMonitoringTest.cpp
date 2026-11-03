#include "MonitoringMachineSelector.h"
#include "SshMachineSnapshot.h"

#include <QScopeGuard>
#include <QSignalSpy>
#include <QTest>

namespace {
QJsonObject executionSnapshot()
{
    return {{"schema", "dockpipe.execution_snapshot.v1"}, {"executions", QJsonArray{}},
        {"processes", QJsonArray{QJsonObject{{"pid", 42}, {"name", "host-process"}}}}};
}

QJsonObject envelope(const QJsonObject &execution = executionSnapshot())
{
    return {{"schema", "dockpipe.machine_snapshot.v1"}, {"node", "actual-hostname"},
        {"state", "live"}, {"execution", execution},
        {"docker", QJsonObject{{"available", true}, {"containers", QJsonArray{}},
             {"networks", QJsonArray{}}, {"volumes", QJsonArray{}}, {"topology", QJsonArray{}}}}};
}

QByteArray encode(const QJsonObject &object)
{
    return QJsonDocument(object).toJson(QJsonDocument::Compact);
}
}

class SshMonitoringTest : public QObject {
    Q_OBJECT

private slots:
    void selectorPreservesSshDuringNodeRefresh();
    void removedSshSelectionStaysUnavailable();
    void localEntryUsesDeploymentLabel();
    void snapshotAcceptsReportedHostnameAndRejectsInvalidEnvelope();
    void snapshotEnforcesComponentBounds();
};

void SshMonitoringTest::selectorPreservesSshDuringNodeRefresh()
{
    MonitoringMachineSelector selector;
    QSignalSpy changes(&selector, &QComboBox::currentIndexChanged);
    selector.setNodes({"paired-one"});
    selector.setSshConnection("host", "Office host");
    QCOMPARE(selector.currentData().toString(), QString("ssh-machine:host"));
    selector.setNodes({"paired-two", "ssh-machine:injected", "paired-two", ""});
    QCOMPARE(selector.currentData().toString(), QString("ssh-machine:host"));
    QVERIFY(selector.findData("paired-two") >= 0);
    QCOMPARE(selector.findData("ssh-machine:injected"), -1);
    QCOMPARE(selector.count(), 3);
    QCOMPARE(changes.count(), 0);
    selector.setCurrentIndex(selector.findData("paired-two"));
    selector.setNodes({"paired-two", "paired-three"});
    QCOMPARE(selector.currentData().toString(), QString("paired-two"));
}

void SshMonitoringTest::removedSshSelectionStaysUnavailable()
{
    MonitoringMachineSelector selector;
    selector.setSshConnection("host", "Office host");
    selector.clearSshConnection();
    QCOMPARE(selector.currentData().toString(), QString("ssh-machine:host"));
    QVERIFY(selector.currentText().contains("unavailable"));
    selector.setNodes({"paired-one"});
    QCOMPARE(selector.currentData().toString(), QString("ssh-machine:host"));
    QVERIFY(selector.currentIndex() != 0);
}

void SshMonitoringTest::localEntryUsesDeploymentLabel()
{
    const bool present = qEnvironmentVariableIsSet("DOCKPIPE_LOCAL_MACHINE_LABEL");
    const QByteArray previous = qgetenv("DOCKPIPE_LOCAL_MACHINE_LABEL");
    const auto restore = qScopeGuard([present, previous] {
        if (present)
            qputenv("DOCKPIPE_LOCAL_MACHINE_LABEL", previous);
        else
            qunsetenv("DOCKPIPE_LOCAL_MACHINE_LABEL");
    });
    qputenv("DOCKPIPE_LOCAL_MACHINE_LABEL", "GUI container");
    MonitoringMachineSelector selector;
    QCOMPARE(selector.itemText(0), QString("GUI container"));
    selector.setNodes({"paired-one"});
    QCOMPARE(selector.itemText(0), QString("GUI container"));
    QVERIFY(selector.itemData(0).toString().isEmpty());
}

void SshMonitoringTest::snapshotAcceptsReportedHostnameAndRejectsInvalidEnvelope()
{
    QJsonObject snapshot;
    QString node;
    QVERIFY(SshMachineSnapshot::parse(encode(envelope()), "execution", &snapshot, &node));
    QCOMPARE(node, QString("actual-hostname"));
    QCOMPARE(snapshot.value("schema").toString(), QString("dockpipe.execution_snapshot.v1"));
    const QJsonObject previous = snapshot;
    auto invalid = envelope();
    invalid.insert("node", "");
    QVERIFY(!SshMachineSnapshot::parse(encode(invalid), "execution", &snapshot));
    QCOMPARE(snapshot, previous);
    invalid = envelope();
    invalid.insert("state", "stale");
    QVERIFY(!SshMachineSnapshot::parse(encode(invalid), "execution", &snapshot));
    invalid = envelope();
    invalid.insert("node", "host\nother");
    QVERIFY(!SshMachineSnapshot::parse(encode(invalid), "execution", &snapshot));
    QVERIFY(!SshMachineSnapshot::parse(QByteArray(1024 * 1024 + 1, ' '), "execution", &snapshot));
    QVERIFY(!SshMachineSnapshot::parse("not-json", "execution", &snapshot));
    QVERIFY(!SshMachineSnapshot::parse(encode(envelope()), "unknown", &snapshot));
}

void SshMonitoringTest::snapshotEnforcesComponentBounds()
{
    QJsonObject snapshot;
    auto execution = executionSnapshot();
    QJsonArray processes;
    for (int index = 0; index < 4097; ++index)
        processes.append(QJsonObject{{"pid", index + 1}});
    execution.insert("processes", processes);
    QVERIFY(!SshMachineSnapshot::parse(encode(envelope(execution)), "execution", &snapshot));
    processes.removeLast();
    execution.insert("processes", processes);
    QVERIFY(SshMachineSnapshot::parse(encode(envelope(execution)), "execution", &snapshot));
    execution.insert("processes", QJsonArray{"wrong-row-type"});
    QVERIFY(!SshMachineSnapshot::parse(encode(envelope(execution)), "execution", &snapshot));
    auto object = envelope();
    auto docker = object.value("docker").toObject();
    QJsonArray containers;
    for (int index = 0; index < 257; ++index)
        containers.append(QJsonObject{{"ID", QString::number(index)}});
    docker.insert("containers", containers);
    object.insert("docker", docker);
    QVERIFY(!SshMachineSnapshot::parse(encode(object), "docker", &snapshot));
    containers.removeLast();
    docker.insert("containers", containers);
    object.insert("docker", docker);
    QVERIFY(SshMachineSnapshot::parse(encode(object), "docker", &snapshot));
    docker.insert("containers", "wrong-collection-type");
    object.insert("docker", docker);
    QVERIFY(!SshMachineSnapshot::parse(encode(object), "docker", &snapshot));
    object.insert("docker", QJsonObject{{"available", false}});
    QVERIFY(SshMachineSnapshot::parse(encode(object), "docker", &snapshot));
    QVERIFY(!snapshot.value("available").toBool());
}

QTEST_MAIN(SshMonitoringTest)
#include "SshMonitoringTest.moc"
