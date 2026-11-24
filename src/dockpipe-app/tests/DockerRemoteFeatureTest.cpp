#include "DockerObservabilityWidget.h"
#include "MonitoringMachineSelector.h"

#include <QApplication>
#include <QFile>
#include <QJsonDocument>
#include <QLabel>
#include <QPlainTextEdit>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QSettings>
#include <QTest>
#include <QThread>
#include <cstdio>

namespace {
QJsonObject target(const char letter = 'a')
{
    return {{"node", QStringLiteral("fixture-worker")}, {"fingerprint", QString(64, letter)},
            {"generation", QString(64, 'b')}};
}

int fixture(bool local, bool prepare = false)
{
    QFile input;
    if (!input.open(stdin, QIODevice::ReadOnly))
        return 2;
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    if (prepare) {
        QJsonObject reference{{"schema", request.value("schema")}, {"id", QString(64, 'e')},
            {"target", request.value("target")}, {"action", request.value("action")}, {"deadline", request.value("deadline")}};
        if (request.contains("root"))
            reference["root"] = request.value("root");
        const auto output = QJsonDocument(reference).toJson(QJsonDocument::Compact);
        std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
        return 0;
    }
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "pause")
        QThread::msleep(350);
    else if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-target")
        QThread::msleep(100);
    request["id"] = QString(64, 'e');
    QJsonObject payload;
    const QString action = request.value("action").toString();
    const auto payloadInput = local ? request : request.value("payload").toObject();
    if (action == "containers.snapshot" || action == "snapshot") {
        payload = {{"available", true}, {"containers", QJsonArray{QJsonObject{
            {"ID", QString(64, 'c')}, {"Names", "remote-only"}, {"State", "exited"}}}},
            {"networks", QJsonArray{}}, {"volumes", QJsonArray{}}, {"topology", QJsonArray{}}};
    } else if (action == "containers.inspect" || action == "containers.logs" || action == "inspect" || action == "logs") {
        const auto operation = local ? action : action.mid(11);
        payload = {{"schema", "dockpipe.containers/v1"}, {"action", operation},
            {"kind", payloadInput.value("kind")}, {"object", payloadInput.value("object")}};
        if (operation == "inspect")
            payload["inspect"] = QJsonArray{QJsonObject{{"Id", payloadInput.value("object")}, {"Name", "fixture-detail"}}};
        else
            payload["logs"] = "fixture logs";
    } else {
        payload = {{"schema", "dockpipe.containers/v1"}, {"action", action.mid(11)},
                   {"container", request.value("payload").toObject().value("container")}};
    }
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-target")
        request["target"] = target('f');
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-receipt") {
        payload["container"] = QString(64, 'f');
        payload["object"] = QString(64, 'f');
    }
    const auto envelope = local ? payload : QJsonObject{{"request", request}, {"status", "success"},
        {"result", QJsonObject{{"status", "success"}, {"payload", payload}}}};
    const auto output = QJsonDocument(envelope).toJson(QJsonDocument::Compact);
    std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
    return 0;
}
}

class DockerRemoteFeatureTest final : public QObject {
    Q_OBJECT
private slots:
    void localInventoryAndDetailsUseTypedCoreTransport()
    {
        const auto binary = QCoreApplication::applicationFilePath();
        const auto snapshot = DockerObservabilityWidget::collectDockerSnapshot(binary, {});
        QVERIFY(snapshot.containersOk);
        QCOMPARE(snapshot.containers.size(), 1);
        const auto id = snapshot.containers.first().id;
        const auto result = DockerObservabilityWidget::runContainerRequest(binary,
            {{"action", "inspect"}, {"kind", "container"}, {"object", id}});
        QVERIFY(result.ok);
        QVERIFY(result.stdoutText.contains("fixture-detail"));
        qputenv("DOCKPIPE_DOCKER_FIXTURE", "wrong-receipt");
        const auto wrong = DockerObservabilityWidget::runContainerRequest(binary,
            {{"action", "inspect"}, {"kind", "container"}, {"object", id}});
        QVERIFY(!wrong.ok);
        auto cancelled = std::make_shared<std::atomic_bool>(true);
        const auto stopped = DockerObservabilityWidget::runContainerRequest(binary, {{"action", "snapshot"}}, cancelled);
        QVERIFY(!stopped.ok);
    }

    void init()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_DOCKER_FIXTURE");
    }

    void exactTargetAndReadOnlyInventory()
    {
        DockerObservabilityWidget widget;
        QString error;
        QVERIFY(!widget.setFeatureTarget({{"node", "unverified"}}, true, &error));
        QVERIFY(!error.isEmpty());
        QVERIFY(widget.setFeatureTarget(target(), false));
        QVERIFY(widget.m_machine->isHidden());
        QVERIFY(widget.isRemoteTarget());
        QVERIFY(!widget.canControlTarget());
        widget.setActive(true);
        QTRY_COMPARE(widget.m_containers->rowCount(), 1);
        QCOMPARE(widget.m_snapshot.containers.first().name, QStringLiteral("remote-only"));
        widget.m_containers->selectRow(0);
        widget.m_selectedResource = "container:" + QString(64, 'c');
        widget.runContainerAction("start");
        QVERIFY(!widget.m_containerActionBusy);
        widget.refreshContainerDetailsAsync(QString(64, 'c'));
        QVERIFY(!widget.m_containerDetailWatcher->isRunning());
        QTRY_VERIFY(widget.m_containerLogs->toPlainText().contains("fixture logs"));
        QVERIFY(widget.m_containerDetails->toPlainText().contains("fixture-detail"));
        widget.setActive(false);
    }

    void monitoringRemainsReadOnlyWithLocalWorkspace()
    {
        DockerObservabilityWidget widget;
        QVERIFY(widget.setFeatureCatalog({}));
        widget.setNodes({"monitor-only"});
        QVERIFY(!widget.m_machine->isHidden());
        widget.m_machine->setCurrentIndex(widget.m_machine->findData("monitor-only"));
        QCOMPARE(widget.targetNode(), QStringLiteral("monitor-only"));
        QVERIFY(!widget.canControlTarget());

        QVERIFY(widget.setFeatureTarget(target(), true));
        QVERIFY(widget.m_machine->isHidden());
        QCOMPARE(widget.targetNode(), target().value("node").toString());
        QVERIFY(widget.canControlTarget());

        QVERIFY(widget.setFeatureCatalog({}));
        QVERIFY(!widget.m_machine->isHidden());
        QCOMPARE(widget.targetNode(), QStringLiteral("monitor-only"));
        QVERIFY(!widget.canControlTarget());
    }

    void writeKeepsOriginalTargetUntilReceipt()
    {
        DockerObservabilityWidget widget;
        QVERIFY(widget.setFeatureTarget(target(), true));
        widget.setActive(true);
        QTRY_COMPARE(widget.m_containers->rowCount(), 1);
        widget.m_containers->selectRow(0);
        widget.m_selectedResource = "container:" + QString(64, 'c');
        qputenv("DOCKPIPE_DOCKER_FIXTURE", "pause");
        widget.runContainerAction("start");
        QVERIFY(widget.m_containerActionBusy);
        QString error;
        QVERIFY(!widget.setFeatureTarget({}, true, &error));
        QVERIFY(!error.isEmpty());
        QCOMPARE(widget.m_featureTarget, target());
        QTRY_VERIFY(!widget.m_containerActionBusy);
        QVERIFY(widget.setFeatureTarget(target('d'), false));
        QCOMPARE(widget.m_featureTarget, target('d'));
        widget.setActive(false);
    }

    void mismatchedReceiptDoesNotRefreshOrReportSuccess()
    {
        DockerObservabilityWidget widget;
        QVERIFY(widget.setFeatureTarget(target(), true));
        widget.setActive(true);
        QTRY_COMPARE(widget.m_containers->rowCount(), 1);
        widget.m_containers->selectRow(0);
        widget.m_selectedResource = "container:" + QString(64, 'c');
        qputenv("DOCKPIPE_DOCKER_FIXTURE", "wrong-receipt");
        widget.runContainerAction("start");
        QVERIFY(widget.m_containerActionBusy);
        QTRY_VERIFY(!widget.m_containerActionBusy);
        QVERIFY(!widget.m_featureReadBusy);
        bool failureShown = false;
        for (auto *label : widget.findChildren<QLabel *>()) {
            if (label->text().contains("Invalid container control receipt"))
                failureShown = true;
        }
        if (!failureShown) {
            for (auto *label : widget.findChildren<QLabel *>())
                qWarning().noquote() << label->text();
        }
        QVERIFY(failureShown);
        const auto retained = RemoteFeatureClient::pendingRecoveries();
        QCOMPARE(retained.size(), 1);
        QCOMPARE(retained.first().toObject().value("action").toString(), QString("containers.start"));
        widget.setActive(false);
    }

    void switchAndHideInvalidatePendingRead()
    {
        qputenv("DOCKPIPE_DOCKER_FIXTURE", "pause");
        DockerObservabilityWidget widget;
        QVERIFY(widget.setFeatureTarget(target(), true));
        widget.setActive(true);
        QTRY_VERIFY(widget.m_featureReadBusy);
        QVERIFY(widget.setFeatureTarget(target('d'), false));
        widget.setActive(false);
        QTest::qWait(450);
        QCOMPARE(widget.m_containers->rowCount(), 0);
        QCOMPARE(widget.m_featureTarget, target('d'));
        QVERIFY(widget.isRemoteTarget());
        QVERIFY(!widget.m_featureReadBusy);
    }

    void unavailableRemoteNeverStartsLocalInventory()
    {
        qputenv("DOCKPIPE_DOCKER_FIXTURE", "wrong-target");
        DockerObservabilityWidget widget;
        QVERIFY(widget.setFeatureTarget(target(), true));
        widget.setActive(true);
        QTRY_VERIFY(widget.m_featureReadBusy);
        QTRY_VERIFY(!widget.m_featureReadBusy);
        QCOMPARE(widget.m_featureTarget, target());
        QCOMPARE(widget.m_containers->rowCount(), 0);
        QVERIFY(widget.isRemoteTarget());
        QVERIFY(!widget.m_refreshWatcher->isRunning());
        widget.setActive(false);
    }
};

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    if (application.arguments().value(1) == "remote")
        return fixture(false, application.arguments().value(2) == "feature-prepare");
    if (application.arguments().value(1) == "containers")
        return fixture(true);
    QCoreApplication::setOrganizationName("DockpipeContainerFixture");
    QCoreApplication::setApplicationName("DockerRemoteFeatureTest");
    QTemporaryDir settings;
    if (!settings.isValid())
        return 8;
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    DockerRemoteFeatureTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DockerRemoteFeatureTest.moc"
