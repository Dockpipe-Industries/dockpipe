#include "ManagedTerminalSession.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QPointer>
#include <QSignalSpy>
#include <QSocketNotifier>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <cstdio>

namespace {
constexpr auto Schema = "dockpipe.local-terminal/v1";

NativeTerminalLaunch::Snapshot profile()
{
    NativeTerminalLaunch::Snapshot result;
    result.id = QStringLiteral("frozen-fixture");
    result.executable = QCoreApplication::applicationFilePath();
    result.arguments = {QStringLiteral("literal argument"), QStringLiteral("$DO_NOT_EXPAND")};
    result.environment = {QStringLiteral("TERM=xterm-256color"), QStringLiteral("COLORTERM=truecolor"),
        QStringLiteral("FROZEN_VALUE=before")};
    result.directory = QDir::tempPath();
    return result;
}

void print(QJsonObject frame)
{
    frame.insert(QStringLiteral("schema"), Schema);
    const auto line = QJsonDocument(frame).toJson(QJsonDocument::Compact) + '\n';
    std::fwrite(line.constData(), 1, size_t(line.size()), stdout);
    std::fflush(stdout);
}

int fixture(QCoreApplication &application)
{
#ifdef Q_OS_WIN
    Q_UNUSED(application);
    return 77;
#else
    if (application.arguments() != QStringList{application.applicationFilePath(), "terminal", "--stdio"})
        return 2;
    const auto mode = qgetenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE");
    QFile input;
    if (!input.open(0, QIODevice::ReadOnly | QIODevice::Unbuffered, QFileDevice::DontCloseHandle))
        return 3;
    quint64 cursor = 0;
    QJsonValue pendingInputId;
    QByteArray pendingInput;
    auto output = [&](const QByteArray &bytes) {
        cursor += quint64(bytes.size());
        print({{"type", "output"}, {"id", 0}, {"data", QString::fromLatin1(bytes.toBase64())},
            {"cursor", double(cursor)}}); // Match the core's omitted zero/default fields.
    };
    auto close = [&] {
        print({{"type", "closed"}, {"id", 0}, {"exit_code", mode == "tail" ? 37 : 0},
            {"cleanup", mode == "unknown-cleanup" ? "unknown" : "complete"}});
        application.quit();
    };
    QSocketNotifier notifier(0, QSocketNotifier::Read);
    QObject::connect(&notifier, &QSocketNotifier::activated, &application, [&] {
        const auto line = input.readLine(2 * 1024 * 1024 + 1);
        if (line.isEmpty()) {
            application.quit();
            return;
        }
        const auto command = QJsonDocument::fromJson(line).object();
        if (command.value("schema") != Schema) {
            application.exit(4);
            return;
        }
        const QString type = command.value("type").toString();
        const auto id = command.value("id");
        if (type == "start") {
            if (id.toInt() != 1 || !command.value("profile").isObject()) {
                application.exit(5);
                return;
            }
            if (mode == "profile") {
                QFile receipt(qEnvironmentVariable("DOCKPIPE_MANAGED_TERMINAL_RECEIPT"));
                if (!receipt.open(QIODevice::WriteOnly) || receipt.write(line) != line.size()) {
                    application.exit(6);
                    return;
                }
            }
            QTimer::singleShot(50, &application, [&, command] {
                print({{"type", "ready"}, {"id", 1}, {"session_id", "fixture-session"}});
                if (mode == "profile")
                    output(QJsonDocument(command).toJson(QJsonDocument::Compact));
                if (mode == "tail") {
                    for (int index = 0; index < 3; ++index)
                        output(QByteArray(32 * 1024, char('a' + index)));
                    close();
                }
            });
        } else if (type == "resize") {
            print({{"type", "ack"}, {"id", id}});
            if (mode == "interleave") {
                output(QByteArray::number(command.value("columns").toInt()) + 'x'
                    + QByteArray::number(command.value("rows").toInt()));
                if (!pendingInput.isEmpty()) {
                    print({{"type", "ack"}, {"id", pendingInputId}, {"bytes", pendingInput.size()}});
                    output(pendingInput);
                    pendingInput.clear();
                }
            }
        } else if (type == "input") {
            const auto bytes = QByteArray::fromBase64(command.value("data").toString().toLatin1());
            if (mode == "lost-core") {
                application.quit();
                return;
            }
            if (mode == "interleave") {
                pendingInputId = id;
                pendingInput = bytes;
                return;
            }
            QTimer::singleShot(150, &application, [&, id, bytes] {
                if (mode == "invalid-oldest") {
                    print({{"type", "output"}, {"id", 0}, {"data", "YQ=="},
                        {"cursor", 1}, {"oldest", QJsonValue::Null}});
                    return;
                }
                if (mode == "bad-output") {
                    print({{"type", "output"}, {"id", 0}, {"data", "!!!"},
                        {"cursor", 3}, {"oldest", 0}, {"truncated", false}});
                    return;
                }
                if (mode == "large-output") {
                    output(QByteArray(32 * 1024 + 1, 'x'));
                    return;
                }
                if (mode == "cursor-gap") {
                    ++cursor;
                    output(bytes);
                    return;
                }
                const int written = mode == "short-ack" ? bytes.size() - 1 : bytes.size();
                const double ackId = mode == "wrong-ack" ? id.toDouble() + 1 : id.toDouble();
                print({{"type", "ack"}, {"id", ackId}, {"bytes", written}});
                output(bytes);
            });
        } else if (type == "close") {
            close();
        } else {
            application.exit(7);
        }
    });
    QTimer::singleShot(5000, &application, &QCoreApplication::quit);
    return application.exec();
#endif
}
}

class ManagedTerminalSessionTest final : public QObject {
    Q_OBJECT
private slots:
    void init()
    {
#ifdef Q_OS_WIN
        QSKIP("The private stdin fixture uses a POSIX notifier.");
#endif
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE");
        qunsetenv("DOCKPIPE_MANAGED_TERMINAL_RECEIPT");
    }

    void sendsFrozenProfileOnlyThroughPrivateStdin()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        const QString receiptPath = directory.filePath(QStringLiteral("start.json"));
        qputenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE", "profile");
        qputenv("DOCKPIPE_MANAGED_TERMINAL_RECEIPT", receiptPath.toUtf8());
        auto snapshot = profile();
        const auto expectedProfile = snapshot.toJson();
        ManagedTerminalSession session;
        QSignalSpy output(&session, &ManagedTerminalSession::output);
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        session.resize(132, 43);
        QVERIFY(session.start(snapshot));
        snapshot.environment = {QStringLiteral("FROZEN_VALUE=after")};
        snapshot.arguments.clear();
        QTRY_COMPARE(output.size(), 1);
        QFile receipt(receiptPath);
        QVERIFY(receipt.open(QIODevice::ReadOnly));
        const auto start = QJsonDocument::fromJson(receipt.readAll()).object();
        QCOMPARE(start.value("profile").toObject(), expectedProfile);
        QCOMPARE(start.value("columns").toInt(), 132);
        QCOMPARE(start.value("rows").toInt(), 43);
        QCOMPARE(start.value("type").toString(), QStringLiteral("start"));
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(session.cleanupVerified());
    }

    void acknowledgesOnlyFullInputAndCoalescesPendingBytes()
    {
        ManagedTerminalSession session;
        QSignalSpy output(&session, &ManagedTerminalSession::output);
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QTRY_VERIFY(session.ready());
        QList<bool> completions;
        QVERIFY(session.sendAcknowledged("first", [&](bool accepted) { completions.append(accepted); }));
        QVERIFY(session.sendAcknowledged("second", [&](bool accepted) { completions.append(accepted); }));
        QVERIFY(session.send("third"));
        QVERIFY(completions.isEmpty());
        QTRY_COMPARE(output.size(), 2);
        QCOMPARE(completions, QList<bool>({true, true}));
        QCOMPARE(output.at(0).first().toByteArray(), QByteArray("first"));
        QCOMPARE(output.at(1).first().toByteArray(), QByteArray("secondthird"));
        QCOMPARE(output.at(1).at(1).toULongLong(), quint64(16));
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(session.error().isEmpty());
    }

    void resizeAcknowledgementDoesNotCompleteInput()
    {
        qputenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE", "interleave");
        ManagedTerminalSession session;
        QSignalSpy output(&session, &ManagedTerminalSession::output);
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        session.resize(110, 35);
        QTRY_COMPARE(output.size(), 1);
        QCOMPARE(output.first().first().toByteArray(), QByteArray("110x35"));
        int completion = -1;
        bool resizeObservedBeforeInputAck = false;
        connect(&session, &ManagedTerminalSession::output, &session, [&](const QByteArray &bytes) {
            if (bytes == "140x52")
                resizeObservedBeforeInputAck = completion == -1;
        });
        QVERIFY(session.sendAcknowledged("payload", [&](bool accepted) { completion = accepted ? 1 : 0; }));
        session.resize(140, 52);
        QTRY_COMPARE(output.size(), 3);
        QCOMPARE(output.at(1).first().toByteArray(), QByteArray("140x52"));
        QVERIFY(resizeObservedBeforeInputAck);
        QCOMPARE(completion, 1);
        QCOMPARE(output.last().first().toByteArray(), QByteArray("payload"));
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
    }

    void rejectsUncertainInputResponses_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("partial-byte-count") << QByteArray("short-ack");
        QTest::newRow("wrong-request-id") << QByteArray("wrong-ack");
        QTest::newRow("malformed-output") << QByteArray("bad-output");
        QTest::newRow("null-oldest-cursor") << QByteArray("invalid-oldest");
        QTest::newRow("oversize-output") << QByteArray("large-output");
        QTest::newRow("unreported-output-gap") << QByteArray("cursor-gap");
        QTest::newRow("lost-core") << QByteArray("lost-core");
    }

    void rejectsUncertainInputResponses()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE", mode);
        ManagedTerminalSession session;
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QTRY_VERIFY(session.ready());
        QList<bool> completions;
        QVERIFY(session.sendAcknowledged("never-replay", [&](bool accepted) { completions.append(accepted); }));
        QTRY_COMPARE(finished.size(), 1);
        QCOMPARE(completions, QList<bool>({false}));
        QVERIFY(!session.running());
        QVERIFY(!session.error().isEmpty());
        QCOMPARE(session.cleanupVerified(), mode != "lost-core");
    }

    void drainsFinalOutputAndPreservesTerminalExitCode()
    {
        qputenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE", "tail");
        ManagedTerminalSession session;
        QSignalSpy output(&session, &ManagedTerminalSession::output);
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QTRY_COMPARE(finished.size(), 1);
        QCOMPARE(finished.first().first().toInt(), 37);
        QCOMPARE(output.size(), 3);
        for (int index = 0; index < 3; ++index) {
            QCOMPARE(output.at(index).first().toByteArray(), QByteArray(32 * 1024, char('a' + index)));
            QCOMPARE(output.at(index).at(1).toULongLong(), quint64((index + 1) * 32 * 1024));
        }
        QVERIFY(session.cleanupVerified());
        QVERIFY(session.error().isEmpty());
    }

    void reportsUnknownCleanup()
    {
        qputenv("DOCKPIPE_MANAGED_TERMINAL_FIXTURE", "unknown-cleanup");
        ManagedTerminalSession session;
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QTRY_VERIFY(session.ready());
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.cleanupVerified());
        QVERIFY(!session.error().isEmpty());
    }

    void failedProcessStartHasNoProviderToClean()
    {
        QTemporaryDir directory;
        QVERIFY(directory.isValid());
        QFile invalidExecutable(directory.filePath(QStringLiteral("invalid-cli")));
        QVERIFY(invalidExecutable.open(QIODevice::WriteOnly));
        invalidExecutable.write("#!/does/not/exist/dockpipe-test-interpreter\n");
        invalidExecutable.close();
        QVERIFY(invalidExecutable.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
        qputenv("DOCKPIPE_BIN", invalidExecutable.fileName().toUtf8());
        ManagedTerminalSession session;
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QTRY_COMPARE(finished.size(), 1);
        QVERIFY(!session.active());
        QVERIFY(session.cleanupVerified());
        QVERIFY(!session.error().isEmpty());
    }

    void boundsPendingInputAndCancelsBeforeReady()
    {
        ManagedTerminalSession session;
        QSignalSpy finished(&session, &ManagedTerminalSession::finished);
        QVERIFY(session.start(profile()));
        QList<bool> completions;
        QVERIFY(session.sendAcknowledged(QByteArray(64 * 1024, 'x'),
            [&](bool accepted) { completions.append(accepted); }));
        QVERIFY(!session.send("overflow"));
        QVERIFY(!session.send({}));
        session.stop();
        QTRY_COMPARE(finished.size(), 1);
        QCOMPARE(completions, QList<bool>({false}));
        QVERIFY(!session.send("after-close"));
    }

    void completionMayDeleteOwner()
    {
        auto *owner = new QObject;
        QPointer<ManagedTerminalSession> session = new ManagedTerminalSession(owner);
        QVERIFY(session->start(profile()));
        QTRY_VERIFY(session->ready());
        bool acknowledged = false;
        QVERIFY(session->sendAcknowledged("delete-owner", [&](bool accepted) {
            acknowledged = accepted;
            delete owner;
        }));
        QTRY_VERIFY(session.isNull());
        QVERIFY(acknowledged);
    }

    void ownerDeletionDoesNotWaitForChild()
    {
        auto *owner = new QObject;
        auto *session = new ManagedTerminalSession(owner);
        QVERIFY(session->start(profile()));
        QTRY_VERIFY(session->ready());
        QElapsedTimer elapsed;
        elapsed.start();
        delete owner;
        QVERIFY(elapsed.elapsed() < 100);
    }
};

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == "terminal")
        return fixture(application);
    ManagedTerminalSessionTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "ManagedTerminalSessionTest.moc"
