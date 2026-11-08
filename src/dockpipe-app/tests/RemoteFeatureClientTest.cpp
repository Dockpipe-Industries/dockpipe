#include "RemoteFeatureClient.h"

#include <QCoreApplication>
#include <QDateTime>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QTest>
#include <QThread>
#include <cstdio>

namespace {
int fixture(const QStringList &arguments)
{
    if (!arguments.contains(QStringLiteral("--auth")) || arguments.last() == QStringLiteral("auto"))
        return 2;
    const auto mode = qgetenv("DOCKPIPE_FEATURE_FIXTURE");
    if (mode == "pause") {
        QThread::sleep(5);
        return 3;
    }
    if (mode == "garbage") {
        std::fputs("invalid protocol", stdout);
        return 0;
    }
    QJsonDocument response;
    if (arguments.value(2) == QStringLiteral("features")) {
        response = QJsonDocument(QJsonArray{});
    } else {
        QFile input;
        if (!input.open(stdin, QIODevice::ReadOnly))
            return 4;
        auto request = QJsonDocument::fromJson(input.readAll()).object();
        request["id"] = QString(64, QLatin1Char('a'));
        if (mode == "wrong-target")
            request["target"] = QJsonObject{{"node", "other-machine"}};
        if (mode == "wrong-intent")
            request["nonce"] = QString(64, QLatin1Char('b'));
        const QString status = mode == "unknown" ? QStringLiteral("unknown") : QStringLiteral("success");
        response = QJsonDocument(QJsonObject{{"request", request}, {"status", status},
            {"result", QJsonObject{{"status", status}, {"payload", request.value("payload")}}}});
    }
    const auto output = response.toJson(QJsonDocument::Compact);
    std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
    return mode == "unknown" ? 1 : 0;
}

QJsonObject intent()
{
    return RemoteFeatureClient::prepareIntent({{"node", "fixture"}, {"fingerprint", QString(64, 'a')},
        {"generation", QString(64, 'b')}}, QStringLiteral("project"), QStringLiteral("files.read"),
        {{"schema", "dockpipe.files/v1"}, {"action", "read"}, {"path", "hardware-key-file.txt"}});
}
}

class RemoteFeatureClientTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_FEATURE_FIXTURE");
    }
    void fixedIntentAndCatalog()
    {
        QObject owner;
        const auto prepared = intent();
        QVERIFY(prepared.value("nonce") != intent().value("nonce"));
        QCOMPARE(prepared.value("nonce").toString().size(), 64);
        int calls = 0;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, prepared, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 1);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Success);
        QCOMPARE(result.value.toObject(), prepared.value("payload").toObject());
        RemoteFeatureClient::catalog(&owner, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 2);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Success);
        QVERIFY(result.value.isArray());
    }
    void rejectsWrongIdentity_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("machine") << QByteArray("wrong-target");
        QTest::newRow("intent") << QByteArray("wrong-intent");
        QTest::newRow("invalid-json") << QByteArray("garbage");
        QTest::newRow("uncertain") << QByteArray("unknown");
    }
    void rejectsWrongIdentity()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_FEATURE_FIXTURE", mode);
        QObject owner;
        bool completed = false;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, intent(), [&](const auto &value) { result = value; completed = true; });
        QTRY_VERIFY(completed);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
    }
    void cancellationAndTimeout()
    {
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "pause");
        QObject owner;
        int calls = 0;
        RemoteFeatureClient::Result result;
        auto cancel = RemoteFeatureClient::invoke(&owner, intent(), [&](const auto &value) { result = value; ++calls; });
        cancel();
        QTRY_COMPARE(calls, 1);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Cancelled);
        cancel();
        QCOMPARE(calls, 1);
        RemoteFeatureClient::invoke(&owner, intent(), [&](const auto &value) { result = value; ++calls; }, 30);
        QTRY_COMPARE(calls, 2);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
    }
    void ownerDestructionDoesNotWaitOrCallBack()
    {
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "pause");
        auto *owner = new QObject;
        bool called = false;
        auto cancel = RemoteFeatureClient::invoke(owner, intent(), [&](const auto &) { called = true; });
        QTest::qWait(50);
        QElapsedTimer elapsed;
        elapsed.start();
        delete owner;
        QVERIFY(elapsed.elapsed() < 100);
        cancel();
        QTest::qWait(50);
        QVERIFY(!called);
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == QStringLiteral("remote"))
        return fixture(application.arguments());
    RemoteFeatureClientTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteFeatureClientTest.moc"
