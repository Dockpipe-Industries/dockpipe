#include "DockerObservabilityWidget.h"
#include "MonitoringMachineSelector.h"

#include <QApplication>
#include <QFile>
#include <QJsonDocument>
#include <QLabel>
#include <QPlainTextEdit>
#include <QTableWidget>
#include <QTest>
#include <QThread>
#include <cstdio>

namespace {
QJsonObject target(const char letter = 'a')
{
    return {{"node", QStringLiteral("fixture-worker")}, {"fingerprint", QString(64, letter)},
            {"generation", QString(64, 'b')}};
}

int fixture()
{
    QFile input;
    if (!input.open(stdin, QIODevice::ReadOnly))
        return 2;
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "pause")
        QThread::msleep(350);
    else if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-target")
        QThread::msleep(100);
    request["id"] = QString(64, 'e');
    QJsonObject payload;
    const QString action = request.value("action").toString();
    if (action == "containers.snapshot") {
        payload = {{"available", true}, {"containers", QJsonArray{QJsonObject{
            {"ID", QString(64, 'c')}, {"Names", "remote-only"}, {"State", "exited"}}}},
            {"networks", QJsonArray{}}, {"volumes", QJsonArray{}}, {"topology", QJsonArray{}}};
    } else {
        payload = {{"schema", "dockpipe.containers/v1"}, {"action", action.mid(11)},
                   {"container", request.value("payload").toObject().value("container")}};
    }
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-target")
        request["target"] = target('f');
    if (qgetenv("DOCKPIPE_DOCKER_FIXTURE") == "wrong-receipt")
        payload["container"] = QString(64, 'f');
    const auto output = QJsonDocument(QJsonObject{{"request", request}, {"status", "success"},
        {"result", QJsonObject{{"status", "success"}, {"payload", payload}}}}).toJson(QJsonDocument::Compact);
    std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
    return 0;
}
}

class DockerRemoteFeatureTest final : public QObject {
    Q_OBJECT
private slots:
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
        QVERIFY(widget.m_containerLogs->toPlainText().contains("unavailable", Qt::CaseInsensitive));
        widget.setActive(false);
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
        QVERIFY(failureShown);
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
        return fixture();
    DockerRemoteFeatureTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DockerRemoteFeatureTest.moc"
