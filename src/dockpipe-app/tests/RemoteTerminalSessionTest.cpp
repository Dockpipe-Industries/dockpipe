#include "RemoteTerminalSession.h"
#include "TerminalHub.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QSignalSpy>
#include <QSocketNotifier>
#include <QTest>
#include <QTimer>
#include <cstdio>
#ifdef Q_OS_UNIX
#include <unistd.h>
#endif

namespace {
QJsonObject catalog()
{
    return {{"target", QJsonObject{{"node", "fixture"}, {"fingerprint", QString(64, 'a')},
        {"generation", QString(64, 'b')}}}, {"terminal_epoch", QString(64, 'c')},
        {"roots", QJsonArray{QJsonObject{{"id", "project"}, {"terminal", true}}}}};
}

void print(const QJsonObject &event)
{
    const auto line = QJsonDocument(event).toJson(QJsonDocument::Compact) + '\n';
    std::fwrite(line.constData(), 1, size_t(line.size()), stdout);
    std::fflush(stdout);
}

int fixture(QCoreApplication &application)
{
#ifdef Q_OS_WIN
    Q_UNUSED(application);
    return 77;
#else
    const auto arguments = application.arguments();
    if (arguments.value(arguments.indexOf("--auth") + 1) != "never")
        return 2;
    QFile requestFile(arguments.value(arguments.indexOf("--request") + 1));
    if (!requestFile.open(QIODevice::ReadOnly))
        return 3;
    const auto intent = QJsonDocument::fromJson(requestFile.readAll()).object();
    const auto mode = qgetenv("DOCKPIPE_TERMINAL_FIXTURE");
    const bool prepare = arguments.contains("--prepare") || !intent.value("container").toString().isEmpty();
    const auto identifier = prepare ? intent.value("nonce") : QJsonValue(QString(64, 'd'));
    QJsonObject reference{{"schema", intent.value("schema")}, {"target", intent.value("target")},
        {"epoch", intent.value("epoch")}, {"id", identifier}};
    if (mode == "wrong-target")
        reference["target"] = QJsonObject{{"node", "wrong"}};
    QFile input;
    // Keep unread frames on the descriptor so the notifier sees each one.
    if (!input.open(STDIN_FILENO, QIODevice::ReadOnly | QIODevice::Unbuffered))
        return 4;
    if (prepare) {
        if (!intent.value("container").toString().isEmpty()
            && (intent.value("root").toString() != QString()
                || (intent.value("shell").toString() != "/bin/sh" && intent.value("shell").toString() != "/bin/bash")))
            return 5;
        print({{"type", "prepared"}, {"reference", reference}});
        const auto acknowledgment = QJsonDocument::fromJson(input.readLine(96 * 1024)).object();
        if (acknowledgment.value("type").toString() != "prepared-ack"
            || acknowledgment.value("reference").toObject() != reference)
            return 6;
        if (mode == "lost-open")
            return 7;
    }
    print({{"type", "session"}, {"reference", reference}});
    if (mode == "lost-host-open")
        return 7;
    quint64 cursor = 0;
    quint64 acknowledged = 0;
    auto view = [&](const QString &state, const QByteArray &data = {}) {
        cursor += quint64(data.size());
        return QJsonObject{{"reference", reference}, {"state", state}, {"cursor", double(cursor)},
            {"end", double(cursor)}, {"oldest", 0}, {"ack_sequence", double(acknowledged)},
            {"data", QString::fromLatin1(data.toBase64())}, {"truncated", false}};
    };
    print({{"type", "output"}, {"view", view("running")}});
    if (mode == "malformed-close" || mode == "malformed-close-ack") {
        auto final = view("closed");
        if (mode == "malformed-close")
            final["end"] = 1;
        else
            final["ack_sequence"] = 1;
        print({{"type", "closed"}, {"view", final}});
        return 0;
    }
    if (mode == "tail") {
        QTimer::singleShot(100, &application, [&] {
            for (int index = 0; index < 3; ++index) {
                auto chunk = view("closed", QByteArray(32 * 1024, 'x'));
                chunk["end"] = 3 * 32 * 1024;
                print({{"type", "output"}, {"view", chunk}});
            }
            print({{"type", "closed"}, {"view", view("closed")}});
            application.quit();
        });
    }
    QSocketNotifier notifier(STDIN_FILENO, QSocketNotifier::Read);
    QObject::connect(&notifier, &QSocketNotifier::activated, &application, [&] {
        const auto line = input.readLine(96 * 1024);
        if (line.isEmpty()) {
            application.quit();
            return;
        }
        const auto command = QJsonDocument::fromJson(line).object();
        const QString type = command.value("type").toString();
        if (type == "close") {
            print({{"type", "closed"}, {"view", view("closed")}});
            application.quit();
        } else if (type == "resize" && mode == "size") {
            const auto size = QByteArray::number(command.value("columns").toInt()) + 'x'
                + QByteArray::number(command.value("rows").toInt());
            print({{"type", "output"}, {"view", view("running", size)}});
        } else if (type == "input") {
            const auto bytes = QByteArray::fromBase64(command.value("data").toString().toLatin1());
            const auto sequence = quint64(command.value("sequence").toDouble());
            QTimer::singleShot(100, &application, [&, bytes, sequence] {
                acknowledged = sequence;
                print({{"type", "ack"}, {"sequence", mode == "wrong-ack" ? double(sequence + 1) : double(sequence)}});
                print({{"type", "output"}, {"view", view("running", bytes)}});
            });
        }
    });
    QTimer::singleShot(5000, &application, &QCoreApplication::quit);
    return application.exec();
#endif
}
}

class RemoteTerminalSessionTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
#ifdef Q_OS_WIN
        QSKIP("The private stdin fixture uses a POSIX notifier.");
#endif
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_TERMINAL_FIXTURE");
    }
    void containerGrantIsIndependentAndPreparedBeforeAmbiguousOpen()
    {
        auto context = catalog();
        context["roots"] = QJsonArray{};
        context["docker_read"] = true;
        context["docker_control"] = false;
        RemoteTerminalSession denied;
        QVERIFY(!denied.startContainer(context, QString(64, 'e'), "/bin/sh"));
        context["docker_exec"] = true;
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", "lost-open");
        RemoteTerminalSession session;
        QSignalSpy finished(&session, &RemoteTerminalSession::finished);
        QVERIFY(session.startContainer(context, QString(64, 'e'), "/bin/sh"));
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.reference().isEmpty());
        QCOMPARE(session.reference().value("target"), context.value("target"));
        QVERIFY(!session.cleanupVerified());
        auto changed = session.reference();
        changed["id"] = QString(64, 'f');
        QVERIFY(!session.confirmContainerRecovery(changed));
        QVERIFY(session.confirmContainerRecovery(session.reference()));
        QVERIFY(session.cleanupVerified());
    }

    void containerSessionsFreezeTargetAndShareQuota()
    {
        TerminalHub hub;
        auto first = catalog();
        first["docker_read"] = true;
        first["docker_exec"] = true;
        first["docker_control"] = false;
        auto second = first;
        auto target = second.value("target").toObject();
        target["generation"] = QString(64, 'e');
        second["target"] = target;
        const QString container(64, 'f');
        const auto firstId = hub.createContainerSession(first, container, "/bin/sh");
        const auto secondId = hub.createContainerSession(second, container, "/bin/bash");
        QVERIFY(!firstId.isEmpty() && !secondId.isEmpty());
        QCOMPARE(hub.sessionIds(first, "container:" + container + ":/bin/sh"), QStringList{firstId});
        QCOMPARE(hub.sessionIds(second, "container:" + container + ":/bin/bash"), QStringList{secondId});
        QStringList hostSessions;
        for (int index = 0; index < 6; ++index) {
            const auto id = hub.createSession({}, {}, first, "project");
            QVERIFY(!id.isEmpty());
            hostSessions.append(id);
        }
        QVERIFY(hub.createContainerSession(first, container, "/bin/sh").isEmpty());
        QTRY_VERIFY(!hub.connecting(firstId) && !hub.connecting(secondId));
        bool acknowledged = false;
        hub.sendAsync(firstId, "selected-container", [&](bool accepted, const QString &) { acknowledged = accepted; });
        QTRY_VERIFY(acknowledged);
        quint64 cursor = 0;
        bool truncated = false;
        QVERIFY(hub.readOutput(secondId, cursor, truncated).isEmpty());
        for (const auto &id : hub.sessionIds())
            hub.closeSession(id);
    }

    void unknownContainerQuotaRequiresExactVerifiedRecovery()
    {
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", "lost-open");
        TerminalHub hub;
        auto context = catalog();
        context["docker_read"] = true;
        context["docker_exec"] = true;
        QStringList sessions;
        for (int index = 0; index < 8; ++index) {
            const auto id = hub.createContainerSession(context, QString(64, 'e'), "/bin/sh");
            QVERIFY(!id.isEmpty());
            sessions.append(id);
        }
        QTRY_VERIFY(([&] {
            for (auto *session : hub.findChildren<RemoteTerminalSession *>()) {
                if (session->active())
                    return false;
            }
            return true;
        })());
        const auto reference = hub.findChildren<RemoteTerminalSession *>().first()->reference();
        QVERIFY(!reference.isEmpty());
        for (const auto &id : sessions)
            hub.closeSession(id);
        QVERIFY(hub.createContainerSession(context, QString(64, 'e'), "/bin/sh").isEmpty());
        auto changed = reference;
        changed["epoch"] = QString(64, 'f');
        hub.releaseRecoveredContainerSession(changed);
        QVERIFY(hub.createContainerSession(context, QString(64, 'e'), "/bin/sh").isEmpty());
        hub.releaseRecoveredContainerSession(reference);
        qunsetenv("DOCKPIPE_TERMINAL_FIXTURE");
        const auto replacement = hub.createContainerSession(context, QString(64, 'e'), "/bin/sh");
        QVERIFY(!replacement.isEmpty());
        QVERIFY(hub.createContainerSession(context, QString(64, 'e'), "/bin/sh").isEmpty());
        hub.closeSession(replacement);
    }

    void unknownHostQuotaRequiresExactVerifiedRecovery()
    {
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", "lost-host-open");
        TerminalHub hub;
        const auto context = catalog();
        QStringList sessions;
        for (int index = 0; index < 8; ++index) {
            const auto id = hub.createSession({}, {}, context, "project");
            QVERIFY(!id.isEmpty());
            sessions.append(id);
        }
        QTRY_VERIFY(([&] {
            for (auto *session : hub.findChildren<RemoteTerminalSession *>()) {
                if (session->active())
                    return false;
            }
            return true;
        })());
        auto *retained = hub.findChildren<RemoteTerminalSession *>().first();
        const auto reference = retained->reference();
        QVERIFY(!reference.isEmpty());
        QVERIFY(!retained->confirmContainerRecovery(reference));
        QVERIFY(!retained->cleanupVerified());
        for (const auto &id : sessions)
            hub.closeSession(id);
        QVERIFY(hub.createSession({}, {}, context, "project").isEmpty());
        auto changed = reference;
        changed["epoch"] = QString(64, 'f');
        hub.releaseRecoveredSession(changed);
        QVERIFY(hub.createSession({}, {}, context, "project").isEmpty());
        hub.releaseRecoveredSession(reference);
        qunsetenv("DOCKPIPE_TERMINAL_FIXTURE");
        const auto replacement = hub.createSession({}, {}, context, "project");
        QVERIFY(!replacement.isEmpty());
        QVERIFY(hub.createSession({}, {}, context, "project").isEmpty());
        hub.closeSession(replacement);
    }

    void malformedFinalFrameCannotVerifyCleanup_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("cursor") << QByteArray("malformed-close");
        QTest::newRow("acknowledgment") << QByteArray("malformed-close-ack");
    }

    void malformedFinalFrameCannotVerifyCleanup()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", mode);
        RemoteTerminalSession session;
        QSignalSpy finished(&session, &RemoteTerminalSession::finished);
        QVERIFY(session.start(catalog(), "project"));
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.reference().isEmpty());
        QVERIFY(!session.cleanupVerified());
    }

    void coalescesInputUntilAcknowledged()
    {
        RemoteTerminalSession session;
        QSignalSpy output(&session, &RemoteTerminalSession::output);
        QSignalSpy finished(&session, &RemoteTerminalSession::finished);
        QVERIFY(session.start(catalog(), "project"));
        QTRY_VERIFY(session.ready());
        int acknowledged = -1;
        QVERIFY(session.sendAcknowledged("first", [&acknowledged](bool accepted) { acknowledged = accepted ? 1 : 0; }));
        QCOMPARE(acknowledged, -1);
        QVERIFY(session.send("second"));
        QTRY_COMPARE(output.size(), 2);
        QCOMPARE(acknowledged, 1);
        QCOMPARE(output.at(0).at(0).toByteArray(), QByteArray("first"));
        QCOMPARE(output.at(1).at(0).toByteArray(), QByteArray("second"));
        QCOMPARE(output.at(1).at(1).toULongLong(), quint64(11));
        QVERIFY(!session.send(QByteArray(64 * 1024 + 1, 'x')));
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.active());
    }
    void wrongTargetAndAcknowledgementFailClosed_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("target") << QByteArray("wrong-target");
        QTest::newRow("ack") << QByteArray("wrong-ack");
    }
    void wrongTargetAndAcknowledgementFailClosed()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", mode);
        RemoteTerminalSession session;
        QSignalSpy finished(&session, &RemoteTerminalSession::finished);
        QVERIFY(session.start(catalog(), "project"));
        if (mode == "wrong-ack") {
            QTRY_VERIFY(session.ready());
            QVERIFY(session.send("never-replay"));
        }
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.error().isEmpty());
        QVERIFY(!session.running());
    }
    void appliesSizeReportedBeforeReady()
    {
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", "size");
        RemoteTerminalSession session;
        QSignalSpy output(&session, &RemoteTerminalSession::output);
        QVERIFY(session.start(catalog(), "project"));
        session.resize(132, 43);
        QTRY_COMPARE(output.size(), 1);
        QCOMPARE(output.first().first().toByteArray(), QByteArray("132x43"));
        session.stop();
    }
    void drainsFinalOutputBeforeClosing()
    {
        qputenv("DOCKPIPE_TERMINAL_FIXTURE", "tail");
        RemoteTerminalSession session;
        QSignalSpy output(&session, &RemoteTerminalSession::output);
        QSignalSpy finished(&session, &RemoteTerminalSession::finished);
        QVERIFY(session.start(catalog(), "project"));
        QTRY_COMPARE(finished.size(), 1);
        QCOMPARE(output.size(), 3);
        QByteArray tail;
        for (const auto &frame : output)
            tail.append(frame.first().toByteArray());
        QCOMPARE(tail, QByteArray(3 * 32 * 1024, 'x'));
        QVERIFY(session.error().isEmpty());
    }
    void missingGrantNeverStartsLocal()
    {
        RemoteTerminalSession session;
        auto denied = catalog();
        denied["roots"] = QJsonArray{};
        QVERIFY(!session.start(denied, "project"));
        QVERIFY(!session.active());
        QVERIFY(!session.error().isEmpty());
        QVERIFY(!session.start({}, "project"));
    }
    void hubKeepsSessionsBoundAcrossContextChanges()
    {
        TerminalHub hub;
        auto first = catalog();
        auto second = catalog();
        auto otherTarget = second.value("target").toObject();
        otherTarget["node"] = "other-worker";
        second["target"] = otherTarget;
        const QString firstId = hub.createSession({}, {}, first, "project");
        const QString secondId = hub.createSession({}, {}, second, "project");
        QVERIFY(!firstId.isEmpty() && !secondId.isEmpty());
        QCOMPARE(hub.sessionIds({}, {}), QStringList{});
        QCOMPARE(hub.sessionIds(first, "project"), QStringList{firstId});
        QCOMPARE(hub.sessionIds(second, "project"), QStringList{secondId});
        QTRY_VERIFY(!hub.connecting(firstId) && !hub.connecting(secondId));
        bool acknowledged = false;
        hub.sendAsync(firstId, "first-worker-only", [&](bool accepted, const QString &) { acknowledged = accepted; });
        QTRY_VERIFY(acknowledged);
        quint64 cursor = 0;
        bool truncated = false;
        QByteArray bytes;
        QTRY_VERIFY_WITH_TIMEOUT(([&] {
            if (bytes.isEmpty())
                bytes = hub.readOutput(firstId, cursor, truncated);
            return !bytes.isEmpty();
        })(), 5000);
        QCOMPARE(bytes, QByteArray("first-worker-only\r"));
        quint64 secondCursor = 0;
        QVERIFY(hub.readOutput(secondId, secondCursor, truncated).isEmpty());
        hub.closeSession(firstId);
        hub.closeSession(secondId);
    }
    void closingSessionRetainsItsQuotaUntilReaped()
    {
        TerminalHub hub;
        QStringList sessions;
        const auto context = catalog();
        for (int index = 0; index < 8; ++index) {
            const auto id = hub.createSession({}, {}, context, "project");
            QVERIFY(!id.isEmpty());
            sessions.append(id);
        }
        QVERIFY(hub.createSession({}, {}, context, "project").isEmpty());
        QTRY_VERIFY(([&] {
            for (const auto &id : sessions) {
                if (hub.connecting(id))
                    return false;
            }
            return true;
        })());
        hub.closeSession(sessions.takeFirst());
        QVERIFY(hub.createSession({}, {}, context, "project").isEmpty());
        QString replacementId;
        QTRY_VERIFY(([&] {
            if (replacementId.isEmpty())
                replacementId = hub.createSession({}, {}, context, "project");
            return !replacementId.isEmpty();
        })());
        sessions.append(replacementId);
        for (const auto &id : sessions)
            hub.closeSession(id);
    }
    void destructionDoesNotWait()
    {
        auto *session = new RemoteTerminalSession;
        QVERIFY(session->start(catalog(), "project"));
        QTRY_VERIFY(session->ready());
        QElapsedTimer elapsed;
        elapsed.start();
        delete session;
        QVERIFY(elapsed.elapsed() < 100);
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == "remote")
        return fixture(application);
    RemoteTerminalSessionTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "RemoteTerminalSessionTest.moc"
