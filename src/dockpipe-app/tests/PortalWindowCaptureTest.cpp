#include "PortalWindowCapture.h"

#include <QDBusConnection>
#include <QDBusArgument>
#include <QDBusContext>
#include <QDBusMetaType>
#include <QDBusMessage>
#include <QDBusObjectPath>
#include <QDBusUnixFileDescriptor>
#include <QElapsedTimer>
#include <QFile>
#include <QPointer>
#include <QProcess>
#include <QSet>
#include <QSignalSpy>
#include <QStandardPaths>
#include <QTest>
#include <QTimer>
#include <QTemporaryDir>
#include <fcntl.h>
#include <unistd.h>

struct FixtureStream {
    uint node;
    QVariantMap properties;
};
Q_DECLARE_METATYPE(FixtureStream)
Q_DECLARE_METATYPE(QList<FixtureStream>)

QDBusArgument &operator<<(QDBusArgument &argument, const FixtureStream &stream)
{
    argument.beginStructure();
    argument << stream.node << stream.properties;
    argument.endStructure();
    return argument;
}

const QDBusArgument &operator>>(const QDBusArgument &argument, FixtureStream &stream)
{
    argument.beginStructure();
    argument >> stream.node >> stream.properties;
    argument.endStructure();
    return argument;
}

namespace {
const QString Service = "org.freedesktop.portal.Desktop";
const QString DesktopPath = "/org/freedesktop/portal/desktop";
}

// Behaves like a portal up to consent. It never shares a real desktop surface.
class MockPortal final : public QObject, protected QDBusContext {
    Q_OBJECT
    Q_CLASSINFO("D-Bus Interface", "org.freedesktop.portal.ScreenCast")
public:
    enum Decision { Pending, Cancel, Deny, CloseSession, ApproveFixture };
    explicit MockPortal(const QDBusConnection &bus) : m_bus(bus)
    {
        m_bus.connect("org.freedesktop.DBus", "/org/freedesktop/DBus", "org.freedesktop.DBus",
            "NameOwnerChanged", this, SLOT(ownerChanged(QString,QString,QString)));
    }
    Decision decision = Pending;
    int starts = 0;
    int selections = 0;
    bool correctSelection = false;
    QSet<QString> sessionOwners;
    QString lastSession;
public slots:
    QDBusObjectPath CreateSession(const QVariantMap &options)
    {
        const QString owner = message().service();
        sessionOwners.insert(owner);
        QString sender = owner.mid(1);
        sender.replace('.', '_');
        lastSession = "/org/freedesktop/portal/desktop/session/" + sender + '/' + options.value("session_handle_token").toString();
        return respond(options, 0, {{"session_handle", lastSession}});
    }
    QDBusObjectPath SelectSources(const QDBusObjectPath &, const QVariantMap &options)
    {
        ++selections;
        correctSelection = options.value("types").toUInt() == 2
            && !options.value("multiple").toBool() && options.value("persist_mode").toUInt() == 0;
        return respond(options, 0, {});
    }
    QDBusObjectPath Start(const QDBusObjectPath &, const QString &, const QVariantMap &options)
    {
        ++starts;
        if (decision == Cancel)
            return respond(options, 1, {});
        if (decision == Deny)
            return respond(options, 2, {});
        if (decision == CloseSession) {
            QTimer::singleShot(20, this, [this] {
                auto closed = QDBusMessage::createSignal(lastSession, "org.freedesktop.portal.Session", "Closed");
                closed.setArguments({QVariantMap()});
                m_bus.send(closed);
            });
        }
        if (decision == ApproveFixture) {
            const QList<FixtureStream> streams{{42, {{"source_type", uint(2)}}}};
            return respond(options, 0, {{"streams", QVariant::fromValue(streams)}});
        }
        return requestPath(options);
    }
    QDBusUnixFileDescriptor OpenPipeWireRemote(const QDBusObjectPath &, const QVariantMap &)
    {
        // The lifecycle fixture never reads this descriptor or a real window.
        const int descriptor = open("/dev/null", O_RDONLY | O_CLOEXEC);
        QDBusUnixFileDescriptor remote(descriptor);
        close(descriptor);
        return remote;
    }
    void ownerChanged(const QString &name, const QString &, const QString &newOwner)
    {
        if (newOwner.isEmpty())
            sessionOwners.remove(name);
    }
private:
    QDBusObjectPath requestPath(const QVariantMap &options) const
    {
        QString sender = message().service().mid(1);
        sender.replace('.', '_');
        return QDBusObjectPath("/org/freedesktop/portal/desktop/request/" + sender + '/' + options.value("handle_token").toString());
    }
    QDBusObjectPath respond(const QVariantMap &options, uint response, const QVariantMap &values)
    {
        const auto path = requestPath(options);
        QTimer::singleShot(5, this, [this, path, response, values] {
            QDBusMessage signal = QDBusMessage::createSignal(path.path(), "org.freedesktop.portal.Request", "Response");
            signal.setArguments({response, values});
            m_bus.send(signal);
        });
        return path;
    }
    QDBusConnection m_bus;
};

class PortalWindowCaptureTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void cancelledPendingConsentClosesItsSession();
    void portalCancellationAndFailureCloseSessions();
    void destructionWhileCreatingClosesLateSession();
    void idleAllocatesNoHelperAndStoppedChildDrainsWithoutBlocking();
    void cleanupTestCase();
private:
    QProcess m_daemon;
    std::unique_ptr<QDBusConnection> m_bus;
    std::unique_ptr<MockPortal> m_portal;
};

void PortalWindowCaptureTest::initTestCase()
{
    qDBusRegisterMetaType<FixtureStream>();
    qDBusRegisterMetaType<QList<FixtureStream>>();
    if (QStandardPaths::findExecutable("dbus-daemon").isEmpty()
        || QStandardPaths::findExecutable("gst-launch-1.0").isEmpty())
        QSKIP("Portal fixture needs dbus-daemon and gst-launch-1.0; no desktop capture is started.");
    m_daemon.start("dbus-daemon", {"--session", "--nofork", "--print-address=1"});
    QVERIFY(m_daemon.waitForStarted(3000));
    QVERIFY(m_daemon.waitForReadyRead(3000));
    const QByteArray address = m_daemon.readLine().trimmed();
    QVERIFY(!address.isEmpty());
    qputenv("DBUS_SESSION_BUS_ADDRESS", address);
    m_bus = std::make_unique<QDBusConnection>(QDBusConnection::connectToBus(QString::fromUtf8(address), "portal-fixture"));
    QVERIFY(m_bus->isConnected());
    QVERIFY(m_bus->registerService(Service));
    m_portal = std::make_unique<MockPortal>(*m_bus);
    QVERIFY(m_bus->registerObject(DesktopPath, m_portal.get(), QDBusConnection::ExportAllSlots));
}

void PortalWindowCaptureTest::cancelledPendingConsentClosesItsSession()
{
    m_portal->decision = MockPortal::Pending;
    const int starts = m_portal->starts;
    PortalWindowCapture capture;
    QSignalSpy failure(&capture, &PortalWindowCapture::failed);
    capture.start();
    QTRY_COMPARE_WITH_TIMEOUT(m_portal->starts, starts + 1, 3000);
    QVERIFY(m_portal->correctSelection);
    QCOMPARE(m_portal->sessionOwners.size(), 1);
    capture.stop();
    QTRY_VERIFY_WITH_TIMEOUT(m_portal->sessionOwners.isEmpty(), 3000);
    QCOMPARE(failure.size(), 0);
    capture.stop();
}

void PortalWindowCaptureTest::portalCancellationAndFailureCloseSessions()
{
    for (const auto decision : {MockPortal::Cancel, MockPortal::Deny, MockPortal::CloseSession}) {
        m_portal->decision = decision;
        PortalWindowCapture capture;
        QSignalSpy failure(&capture, &PortalWindowCapture::failed);
        QSignalSpy cancellation(&capture, &PortalWindowCapture::cancelled);
        capture.start();
        if (decision == MockPortal::Cancel)
            QTRY_COMPARE_WITH_TIMEOUT(cancellation.size(), 1, 3000);
        else
            QTRY_COMPARE_WITH_TIMEOUT(failure.size(), 1, 3000);
        QTRY_VERIFY_WITH_TIMEOUT(m_portal->sessionOwners.isEmpty(), 3000);
    }
}

void PortalWindowCaptureTest::destructionWhileCreatingClosesLateSession()
{
    m_portal->decision = MockPortal::Pending;
    {
        PortalWindowCapture capture;
        capture.start();
    }
    QTest::qWait(100);
    QTRY_VERIFY_WITH_TIMEOUT(m_portal->sessionOwners.isEmpty(), 3000);
}

void PortalWindowCaptureTest::idleAllocatesNoHelperAndStoppedChildDrainsWithoutBlocking()
{
    if (!QFile::exists("/usr/bin/python3"))
        QSKIP("The unresponsive-child fixture requires Python 3.");
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    QFile executable(directory.filePath("gst-launch-1.0"));
    QVERIFY(executable.open(QIODevice::WriteOnly));
    const QByteArray script = "#!/usr/bin/python3\nimport signal, time\nsignal.signal(signal.SIGTERM, signal.SIG_IGN)\nprint('fixture ready', flush=True, file=__import__('sys').stderr)\ntime.sleep(30)\n";
    QCOMPARE(executable.write(script), script.size());
    executable.close();
    QVERIFY(executable.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    const QByteArray previousPath = qgetenv("PATH");
    struct RestorePath {
        QByteArray value;
        ~RestorePath() { qputenv("PATH", value); }
    } restore{previousPath};
    qputenv("PATH", directory.path().toUtf8() + ':' + previousPath);
    m_portal->decision = MockPortal::ApproveFixture;

    for (int cycle = 0; cycle < 2; ++cycle) {
        auto capture = std::make_unique<PortalWindowCapture>();
        QCOMPARE(capture->findChildren<QProcess *>().size(), 0);
        QSignalSpy failure(capture.get(), &PortalWindowCapture::failed);
        capture->start();
        QPointer<QProcess> helper;
        QTRY_VERIFY_WITH_TIMEOUT((helper = capture->findChild<QProcess *>("portalWindowCaptureHelper")), 3000);
        QTRY_COMPARE_WITH_TIMEOUT(helper->state(), QProcess::Running, 3000);
        // Give the fixture time to install its SIGTERM handler; no frames are
        // produced. This tests the bounded kill path, not successful sharing.
        QTest::qWait(100);
        auto *deadline = capture->findChild<QTimer *>("portalCaptureDeadline");
        QVERIFY(deadline && deadline->isActive());
        const auto stopped = [&helper] {
            if (!helper)
                return false;
            QFile status(QString("/proc/%1/status").arg(helper->processId()));
            if (!status.open(QIODevice::ReadOnly))
                return false;
            const auto bytes = status.readAll();
            return bytes.contains("State:\tT");
        };
        const int remaining = deadline->remainingTime();
        capture->setPaused(true);
        QTRY_VERIFY_WITH_TIMEOUT(stopped(), 1000);
        QVERIFY(!deadline->isActive());
        QTest::qWait(100);
        capture->setPaused(false);
        QTRY_VERIFY_WITH_TIMEOUT(!stopped(), 1000);
        QVERIFY(deadline->isActive());
        QVERIFY(deadline->remainingTime() <= remaining);
        QVERIFY(deadline->remainingTime() >= remaining - 1000);
        // A suspended helper must be resumed for termination and still reach
        // the bounded kill path when it ignores SIGTERM.
        capture->setPaused(true);
        QTRY_VERIFY_WITH_TIMEOUT(stopped(), 1000);
        QElapsedTimer elapsed;
        elapsed.start();
        if (cycle == 0)
            capture->stop();
        else
            capture.reset();
        QVERIFY2(elapsed.elapsed() < 200, "Stopping a share blocked the GUI thread");
        QTRY_VERIFY_WITH_TIMEOUT(!helper, 2000);
        QTRY_VERIFY_WITH_TIMEOUT(m_portal->sessionOwners.isEmpty(), 3000);
        QCOMPARE(failure.size(), 0);
    }
}

void PortalWindowCaptureTest::cleanupTestCase()
{
    m_portal.reset();
    QDBusConnection::disconnectFromBus("portal-fixture");
    m_bus.reset();
    if (m_daemon.state() != QProcess::NotRunning) {
        m_daemon.terminate();
        if (!m_daemon.waitForFinished(1000)) {
            m_daemon.kill();
            m_daemon.waitForFinished(1000);
        }
    }
}

QTEST_GUILESS_MAIN(PortalWindowCaptureTest)
#include "PortalWindowCaptureTest.moc"
