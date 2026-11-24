#include "RemoteFeatureClient.h"
#include "RemoteFeatureRecoveryDialog.h"

#include <QApplication>
#include <QComboBox>
#include <QDateTime>
#include <QDialog>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QSettings>
#include <QTemporaryDir>
#include <QTest>
#include <cstdio>

namespace {
QJsonObject target(const char generation)
{
    return {{"node", "fixture-worker"}, {"fingerprint", QString(64, 'a')},
            {"generation", QString(64, generation)}};
}

QJsonObject terminalReference()
{
    return {{"schema", "dockpipe.remote.terminal/v1"}, {"target", target('c')},
            {"id", QString(64, 'f')}, {"epoch", QString(64, 'd')}};
}

void print(const QJsonDocument &document)
{
    const auto data = document.toJson(QJsonDocument::Compact);
    std::fwrite(data.constData(), 1, size_t(data.size()), stdout);
}

int fixture(const QStringList &arguments)
{
    const auto mode = qgetenv("DOCKPIPE_HOST_RECOVERY_FIXTURE");
    const auto command = arguments.value(2);
    if (command == "features") {
        if (mode.startsWith("receipt-only")) {
            print(QJsonDocument(QJsonArray{}));
            return 0;
        }
        const QJsonObject catalog{{"target", target('b')},
            {"roots", QJsonArray{QJsonObject{{"id", "project"}, {"terminal", mode != "denied"}}}},
            {"host_terminals", QJsonArray{QJsonObject{{"reference", terminalReference()},
                {"root", "project"}, {"phase", "running"}}}}};
        print(QJsonDocument(QJsonArray{catalog}));
        return 0;
    }
    QFile input;
    if (!input.open(stdin, QIODevice::ReadOnly))
        return 2;
    auto request = QJsonDocument::fromJson(input.readAll()).object();
    if (command == "feature-status") {
        if (request.value("target").toObject() != target('b')
            || request.value("action") != QStringLiteral("terminal.host-recover")
            || request.value("root") != QStringLiteral("project")
            || request.contains("payload") || request.contains("nonce"))
            return 4;
        const QJsonObject proof{{"schema", "dockpipe.host-terminal-receipt/v1"},
            {"reference", terminalReference()}, {"reconciled", true},
            {"root", mode == "receipt-only-wrong-root" ? "other" : "project"}};
        print(QJsonDocument(QJsonObject{{"request", request}, {"status", "success"},
            {"receipt_only", true}, {"host_recovery", proof},
            {"result", QJsonObject{{"status", "success"}}}}));
        return 0;
    }
    if (request.value("target").toObject() != target('b')
        || request.value("root") != QStringLiteral("project")
        || request.value("action") != QStringLiteral("terminal.host-recover")
        || request.value("payload").toObject() != QJsonObject{{"reference", terminalReference()}})
        return 3;
    request["id"] = QString(64, 'e');
    if (command == "feature-prepare") {
        QJsonObject reference;
        for (const auto *key : {"schema", "id", "target", "root", "action", "deadline"})
            reference.insert(QLatin1String(key), request.value(QLatin1String(key)));
        print(QJsonDocument(reference));
        return 0;
    }
    const QJsonObject receipt{{"schema", "dockpipe.host-terminal-receipt/v1"},
        {"reference", terminalReference()}, {"root", mode == "wrong-root" ? "other" : "project"},
        {"reconciled", true}};
    print(QJsonDocument(QJsonObject{{"request", request}, {"status", "success"},
        {"result", QJsonObject{{"status", "success"}, {"payload", receipt}}}}));
    return 0;
}
}

class RemoteTerminalRecoveryDialogTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_HOST_RECOVERY_FIXTURE");
        QSettings().clear();
    }

    void hostRecoveryUsesItsRootGrantAndOriginalReference()
    {
        QWidget owner;
        int verified = 0;
        auto *button = RemoteFeatureRecoveryDialog::createButton(&owner,
            [&](const QJsonObject &reference, const QJsonObject &receipt) {
                QCOMPARE(reference, terminalReference());
                QCOMPARE(receipt.value("root").toString(), QStringLiteral("project"));
                ++verified;
            });
        button->click();
        auto *dialog = button->findChild<QDialog *>(QStringLiteral("remoteFeatureRecoveryDialog"));
        QVERIFY(dialog);
        auto *operations = dialog->findChild<QComboBox *>(QStringLiteral("remoteRecoveryOperations"));
        auto *verify = dialog->findChild<QPushButton *>(QStringLiteral("remoteRecoveryVerifyCleanup"));
        auto *retire = dialog->findChild<QPushButton *>(QStringLiteral("remoteRecoveryRetireContainer"));
        auto *status = dialog->findChild<QLabel *>(QStringLiteral("remoteRecoveryStatus"));
        QVERIFY(operations && verify && retire && status);
        QTRY_COMPARE(operations->count(), 1);
        QVERIFY(verify->isEnabled());
        QVERIFY(!retire->isEnabled());
        verify->click();
        QTRY_COMPARE(verified, 1);
        QTRY_VERIFY(verify->isEnabled());
        QVERIFY(status->text().startsWith(QStringLiteral("Cleanup verified")));
        button->click();
        QTest::qWait(250);
        QVERIFY(status->text().startsWith(QStringLiteral("Cleanup verified")));
        QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
    }

    void changedRootReceiptDoesNotReleaseSession()
    {
        qputenv("DOCKPIPE_HOST_RECOVERY_FIXTURE", "wrong-root");
        QWidget owner;
        int verified = 0;
        auto *button = RemoteFeatureRecoveryDialog::createButton(&owner,
            [&](const QJsonObject &, const QJsonObject &) { ++verified; });
        button->click();
        auto *dialog = button->findChild<QDialog *>(QStringLiteral("remoteFeatureRecoveryDialog"));
        QVERIFY(dialog);
        auto *operations = dialog->findChild<QComboBox *>(QStringLiteral("remoteRecoveryOperations"));
        auto *verify = dialog->findChild<QPushButton *>(QStringLiteral("remoteRecoveryVerifyCleanup"));
        QVERIFY(operations && verify);
        QTRY_COMPARE(operations->count(), 1);
        verify->click();
        QTRY_COMPARE(RemoteFeatureClient::pendingRecoveries().size(), 1);
        QCOMPARE(verified, 0);
    }

    void persistedProofRecoversLostCompletion_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::addColumn<int>("expectedVerified");
        QTest::newRow("valid") << QByteArray("receipt-only") << 1;
        QTest::newRow("wrong-root") << QByteArray("receipt-only-wrong-root") << 0;
    }

    void persistedProofRecoversLostCompletion()
    {
        QFETCH(QByteArray, mode);
        QFETCH(int, expectedVerified);
        qputenv("DOCKPIPE_HOST_RECOVERY_FIXTURE", mode);
        const QJsonObject metadata{{"schema", "dockpipe.remote.features/v1"},
            {"id", QString(64, 'e')}, {"target", target('b')}, {"root", "project"},
            {"action", "terminal.host-recover"},
            {"deadline", QDateTime::currentDateTimeUtc().addSecs(25).toString(Qt::ISODateWithMs)}};
        RemoteFeatureClient::retainUncertain(metadata);
        QWidget owner;
        int verified = 0;
        auto *button = RemoteFeatureRecoveryDialog::createButton(&owner,
            [&](const QJsonObject &reference, const QJsonObject &) {
                QCOMPARE(reference, terminalReference());
                ++verified;
            });
        button->click();
        auto *dialog = button->findChild<QDialog *>(QStringLiteral("remoteFeatureRecoveryDialog"));
        QVERIFY(dialog);
        auto *refresh = dialog->findChild<QPushButton *>(QStringLiteral("remoteRecoveryRefreshStatus"));
        QVERIFY(refresh && refresh->isEnabled());
        refresh->click();
        if (expectedVerified) {
            QTRY_COMPARE(verified, expectedVerified);
            QVERIFY(RemoteFeatureClient::pendingRecoveries().isEmpty());
        } else {
            auto *operations = dialog->findChild<QComboBox *>(QStringLiteral("remoteRecoveryOperations"));
            QTRY_VERIFY(operations->isEnabled());
            QCOMPARE(verified, 0);
            QCOMPARE(RemoteFeatureClient::pendingRecoveries().size(), 1);
        }
    }

    void missingTerminalGrantCannotRecoverHost()
    {
        qputenv("DOCKPIPE_HOST_RECOVERY_FIXTURE", "denied");
        QWidget owner;
        auto *button = RemoteFeatureRecoveryDialog::createButton(&owner);
        button->click();
        auto *dialog = button->findChild<QDialog *>(QStringLiteral("remoteFeatureRecoveryDialog"));
        QVERIFY(dialog);
        auto *operations = dialog->findChild<QComboBox *>(QStringLiteral("remoteRecoveryOperations"));
        auto *verify = dialog->findChild<QPushButton *>(QStringLiteral("remoteRecoveryVerifyCleanup"));
        QVERIFY(operations && verify);
        QTRY_COMPARE(operations->count(), 1);
        QVERIFY(!verify->isEnabled());
    }
};

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    if (application.arguments().value(1) == "remote")
        return fixture(application.arguments());
    QCoreApplication::setOrganizationName("DockpipeHostRecoveryFixture");
    QCoreApplication::setApplicationName("RemoteTerminalRecoveryDialogTest");
    QTemporaryDir settings;
    if (!settings.isValid())
        return 8;
    QSettings::setDefaultFormat(QSettings::IniFormat);
    QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, settings.path());
    RemoteTerminalRecoveryDialogTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteTerminalRecoveryDialogTest.moc"
