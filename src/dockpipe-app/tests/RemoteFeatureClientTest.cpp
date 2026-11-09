#include "RemoteFeatureClient.h"

#include <QCoreApplication>
#include <QDateTime>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QTest>
#include <QThread>
#include <QSettings>
#include <QTemporaryDir>
#include <cstdio>

namespace {
QJsonObject metadata(const QJsonObject &request)
{
    QJsonObject reference;
    for (const auto &key : {"schema", "id", "target", "root", "action", "deadline"}) {
        if (request.contains(key))
            reference.insert(key, request.value(key));
    }
    return reference;
}

int fixture(const QStringList &arguments)
{
    const QString command = arguments.value(2);
    if (command != "feature-prepare"
        && (!arguments.contains(QStringLiteral("--auth")) || arguments.last() == QStringLiteral("auto")))
        return 2;
    const auto mode = qgetenv("DOCKPIPE_FEATURE_FIXTURE");
    if ((mode == "pause" && command == "feature-call") || mode == "prepare-pause") {
        QThread::sleep(5);
        return 3;
    }
    if (mode == "garbage" && command == "feature-call") {
        std::fputs("invalid protocol", stdout);
        return 0;
    }
    QJsonDocument response;
    if (command == QStringLiteral("features")) {
        response = QJsonDocument(QJsonArray{});
    } else {
        QFile input;
        if (!input.open(stdin, QIODevice::ReadOnly))
            return 4;
        auto request = QJsonDocument::fromJson(input.readAll()).object();
        if (command == "feature-prepare") {
            request["id"] = QString(64, QLatin1Char('a'));
            response = QJsonDocument(metadata(request));
        } else {
            if (request.value("id").toString() != QString(64, QLatin1Char('a')))
                return 5;
            const bool recovery = command != "feature-call";
            if (recovery && request != metadata(request))
                return 6;
            if (mode == "wrong-target")
                request["target"] = QJsonObject{{"node", "other-machine"}};
            if (mode == "wrong-id")
                request["id"] = QString(64, QLatin1Char('b'));
            if (mode == "wrong-intent")
                request["nonce"] = QString(64, QLatin1Char('b'));
            const QString status = mode == "unknown" || (recovery && mode != "completed") ? QStringLiteral("unknown") : QStringLiteral("success");
            QJsonObject operation{{"request", request}, {"status", status},
                {"result", QJsonObject{{"status", status}, {"payload", request.value("payload")}}}};
            if (recovery) {
                operation.insert("receipt_only", true);
                operation.insert("metadata_only", true);
                if (command == "feature-reconcile") {
                    operation.insert("cancel_requested", true);
                    operation.insert("recovery_requested", true);
                    operation.insert("reconciled", true);
                }
            }
            response = QJsonDocument(operation);
        }
    }
    const auto output = response.toJson(QJsonDocument::Compact);
    std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
    return mode == "unknown" && command == "feature-call" ? 1 : 0;
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
        QSettings().clear();
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
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
    }
    void rejectsWrongIdentity_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("machine") << QByteArray("wrong-target");
        QTest::newRow("id") << QByteArray("wrong-id");
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
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
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
        RemoteFeatureClient::invoke(&owner, intent(), [&](const auto &value) { result = value; ++calls; }, 150);
        QTRY_COMPARE(calls, 2);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
    }
    void retainedMutationMetadataAndRecovery()
    {
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "unknown");
        auto prepared = intent();
        prepared.insert("action", "files.write");
        prepared.insert("payload", QJsonObject{{"content", "sensitive-file-marker"}});
        QObject owner;
        int calls = 0;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, prepared, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 1);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
        QCOMPARE(result.reference.value("id").toString(), QString(64, 'a'));
        QVERIFY(!result.reference.contains("nonce"));
        QVERIFY(!result.reference.contains("payload"));
        QCOMPARE(RemoteFeatureClient::pendingRecoveries().size(), 1);
        const auto stored = QSettings().value("remote/uncertainFeatureOperations").toByteArray();
        QVERIFY(!stored.contains("sensitive-file-marker"));
        QVERIFY(!stored.contains("nonce"));
        QVERIFY(!stored.contains("payload"));
        const auto reference = result.reference;
        qunsetenv("DOCKPIPE_FEATURE_FIXTURE");
        RemoteFeatureClient::inspect(&owner, reference, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 2);
        QCOMPARE(result.reference, reference);
        QVERIFY(result.value.toObject().value("metadata_only").toBool());
        RemoteFeatureClient::cancelOperation(&owner, reference, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 3);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
        QCOMPARE(RemoteFeatureClient::pendingRecoveries().size(), 1);
        RemoteFeatureClient::reconcile(&owner, reference, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 4);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
        QVERIFY(RemoteFeatureClient::pendingRecoveries().first().toObject().value("reconciled").toBool());
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "wrong-id");
        RemoteFeatureClient::inspect(&owner, reference, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 5);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Unknown);
        QVERIFY(result.value.isNull() || result.value.isUndefined());
        auto unsupported = reference;
        unsupported.insert("action", "unsupported.mutation");
        RemoteFeatureClient::reconcile(&owner, unsupported, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 6);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Failure);
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "completed");
        RemoteFeatureClient::inspect(&owner, reference, [&](const auto &value) { result = value; ++calls; });
        QTRY_COMPARE(calls, 7);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Success);
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
    }
    void recoveryCapacityRejectsAnotherMutation()
    {
        const auto prepared = intent();
        auto reference = metadata(prepared);
        reference.insert("action", "files.write");
        QJsonArray retained;
        for (int index = 0; index < 128; ++index) {
            reference.insert("id", QString::number(index, 16).rightJustified(64, '0'));
            retained.append(reference);
        }
        QSettings().setValue("remote/uncertainFeatureOperations", QJsonDocument(retained).toJson(QJsonDocument::Compact));
        auto write = prepared;
        write.insert("action", "files.write");
        QObject owner;
        bool done = false;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, write, [&](const auto &value) { result = value; done = true; });
        QTRY_VERIFY(done);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Failure);
        QCOMPARE(RemoteFeatureClient::pendingRecoveries().size(), 128);
    }
    void damagedRecoveryStorageRefusesAnotherMutation()
    {
        const QByteArray damaged("not a recovery journal");
        QSettings().setValue("remote/uncertainFeatureOperations", damaged);
        auto write = intent();
        write.insert("action", "files.write");
        QObject owner;
        bool done = false;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, write, [&](const auto &value) { result = value; done = true; });
        QTRY_VERIFY(done);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Failure);
        QCOMPARE(QSettings().value("remote/uncertainFeatureOperations").toByteArray(), damaged);
    }
    void preparationTimeoutNeverDispatches()
    {
        qputenv("DOCKPIPE_FEATURE_FIXTURE", "prepare-pause");
        QObject owner;
        bool done = false;
        RemoteFeatureClient::Result result;
        RemoteFeatureClient::invoke(&owner, intent(), [&](const auto &value) { result = value; done = true; }, 50);
        QTRY_VERIFY(done);
        QCOMPARE(result.outcome, RemoteFeatureClient::Outcome::Failure);
        QVERIFY(result.reference.isEmpty());
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
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
    QCoreApplication::setOrganizationName("DockpipeRemoteFeatureFixture");
    QCoreApplication::setApplicationName("RemoteFeatureClientTest");
    if (application.arguments().value(1) == QStringLiteral("remote"))
        return fixture(application.arguments());
    QTemporaryDir settings;
    if (!settings.isValid())
        return 8;
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    RemoteFeatureClientTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteFeatureClientTest.moc"
