#include "RemoteSecurityWidget.h"

#include <QApplication>
#include <QCheckBox>
#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPushButton>
#include <QSpinBox>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <cstring>
#include <functional>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Remote security assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 5000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

static QByteArray readFile(const QString &path)
{
    QFile file(path);
    if (!file.open(QIODevice::ReadOnly))
        return {};
    return file.readAll();
}

static bool writeFile(const QString &path, const QByteArray &contents = {})
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(contents) == contents.size();
}

static int runSecurityFixture()
{
    const QStringList arguments = QCoreApplication::arguments().mid(1);
    const QDir directory(qEnvironmentVariable("DOCKPIPE_TEST_ROOT"));
    const QString command = arguments.value(1);
    QString captureName;
    if (command == QStringLiteral("security-status"))
        captureName = QStringLiteral("security-status-args");
    else if (command == QStringLiteral("unlock") || command == QStringLiteral("lock"))
        captureName = QStringLiteral("security-action-args");
    else if (command == QStringLiteral("security-policy"))
        captureName = QStringLiteral("security-policy-args");
    else
        return 2;

    if (!writeFile(directory.filePath(captureName), arguments.join('\n').toUtf8() + '\n'))
        return 10;

    QFile output;
    if (!output.open(stdout, QIODevice::WriteOnly))
        return 10;

    if (command == QStringLiteral("security-status")) {
        QJsonObject capabilities{{"idle", true}, {"screen_lock", false}, {"sleep", true}};
        QJsonObject status{
            {"supported", true}, {"authority_available", true}, {"locked", true},
            {"idle_seconds", 300}, {"lock_on_screen_lock", false}, {"lock_on_sleep", true},
            {"absolute_seconds", 3600}, {"expires_at", ""},
            {"detail", "Fixture authorization is locked."}
        };
        if (directory.exists(QStringLiteral("security-unavailable"))) {
            status["supported"] = false;
            status["authority_available"] = false;
            status["detail"] = "Fixture host has no security authority.";
            capabilities = {{"idle", false}, {"screen_lock", false}, {"sleep", false}};
        } else if (directory.exists(QStringLiteral("security-capability-limited"))) {
            status["supported"] = false;
            status["lock_on_screen_lock"] = true;
            status["detail"] = "Native idle, screen lock and sleep notifications are unavailable; the remote authority remains reachable.";
            capabilities = {{"idle", false}, {"screen_lock", false}, {"sleep", false}};
        } else if (directory.exists(QStringLiteral("security-idle-unavailable"))) {
            status["detail"] = "Native idle, screen lock and sleep notifications are unavailable.";
            capabilities = {{"idle", false}, {"screen_lock", false}, {"sleep", false}};
        } else if (directory.exists(QStringLiteral("security-unlocked"))) {
            status["locked"] = false;
            status["expires_at"] = "2026-10-27T14:00:00Z";
            status["detail"] = "Fixture authorization active.";
        }
        status["capabilities"] = capabilities;
        const QByteArray json = QJsonDocument(status).toJson(QJsonDocument::Compact) + '\n';
        return output.write(json) == json.size() && output.flush() ? 0 : 10;
    }

    if (command == QStringLiteral("unlock")) {
        if (directory.exists(QStringLiteral("security-unlock-fails"))) {
            std::fputs("Fixture native authentication was unavailable.\n", stderr);
            return 1;
        }
        return writeFile(directory.filePath(QStringLiteral("security-unlocked"))) ? 0 : 10;
    }
    if (command == QStringLiteral("lock")) {
        const QString marker = directory.filePath(QStringLiteral("security-unlocked"));
        return !QFileInfo::exists(marker) || QFile::remove(marker) ? 0 : 10;
    }
    return output.write("{}\n") == 3 && output.flush() ? 0 : 10;
}

int main(int argc, char **argv)
{
    if (argc > 1 && std::strcmp(argv[1], "remote") == 0) {
        QCoreApplication app(argc, argv);
        return runSecurityFixture();
    }

    QApplication app(argc, argv);
    QTemporaryDir directory;
    CHECK(directory.isValid());
    qputenv("DOCKPIPE_TEST_ROOT", directory.path().toUtf8());

#ifdef Q_OS_WIN
    const QString cliName = QStringLiteral("dockpipe.exe");
#else
    const QString cliName = QStringLiteral("dockpipe");
#endif
    const QString cliPath = directory.filePath(cliName);
    CHECK(QFile::copy(QCoreApplication::applicationFilePath(), cliPath));
    CHECK(QFile::setPermissions(cliPath, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    qputenv("DOCKPIPE_BIN", cliPath.toUtf8());

    RemoteSecurityWidget widget;
    widget.resize(680, 560);
    widget.show();
    widget.refresh();

    auto *status = widget.findChild<QLabel *>(QStringLiteral("remoteSecurityStatus"));
    auto *capabilities = widget.findChild<QLabel *>(QStringLiteral("remoteSecurityCapabilities"));
    auto *idle = widget.findChild<QSpinBox *>(QStringLiteral("remoteSecurityIdleSeconds"));
    auto *screenLock = widget.findChild<QCheckBox *>(QStringLiteral("remoteSecurityLockOnScreenLock"));
    auto *sleepLock = widget.findChild<QCheckBox *>(QStringLiteral("remoteSecurityLockOnSleep"));
    auto *unlock = widget.findChild<QPushButton *>(QStringLiteral("remoteSecurityUnlock"));
    auto *lock = widget.findChild<QPushButton *>(QStringLiteral("remoteSecurityLock"));
    auto *savePolicy = widget.findChild<QPushButton *>(QStringLiteral("remoteSecuritySavePolicy"));
    CHECK(status && capabilities && idle && screenLock && sleepLock && unlock && lock && savePolicy);
    CHECK(waitUntil([&]() { return status->text().contains(QStringLiteral("locked")); }));
    CHECK(idle->value() == 300 && idle->isEnabled());
    CHECK(!screenLock->isChecked() && screenLock->isEnabled());
    CHECK(sleepLock->isChecked() && sleepLock->isEnabled());
    CHECK(capabilities->text().contains(QStringLiteral("Idle lock: after 300 seconds")));
    CHECK(capabilities->text().contains(QStringLiteral("Screen lock: unavailable")));
    CHECK(unlock->isEnabled() && lock->isEnabled());

    unlock->click();
    CHECK(waitUntil([&]() { return status->text().contains(QStringLiteral("unlocked")); }));
    CHECK(status->text().contains(QStringLiteral("expires at")));
    CHECK(readFile(directory.filePath(QStringLiteral("security-action-args"))) == "remote\nunlock\n--auth\nauto\n");
    CHECK(readFile(directory.filePath(QStringLiteral("security-status-args"))) == "remote\nsecurity-status\n--auth\nnever\n");

    idle->setValue(420);
    sleepLock->setChecked(false);
    CHECK(QMetaObject::invokeMethod(&widget, "savePolicy", Qt::DirectConnection));
    const QString policyPath = directory.filePath(QStringLiteral("security-policy-args"));
    CHECK(waitUntil([&]() { return QFileInfo::exists(policyPath); }));
    CHECK(readFile(policyPath) == "remote\nsecurity-policy\n--idle-seconds\n420\n"
                                  "--lock-on-screen-lock=false\n--lock-on-sleep=false\n--auth\nauto\n");
    CHECK(waitUntil([&]() { return lock->isEnabled(); }));

    lock->click();
    CHECK(waitUntil([&]() { return status->text().contains(QStringLiteral("is locked.")); }));
    CHECK(readFile(directory.filePath(QStringLiteral("security-action-args"))) == "remote\nlock\n--auth\nauto\n");

    QFile idleUnavailable(directory.filePath(QStringLiteral("security-idle-unavailable")));
    CHECK(idleUnavailable.open(QIODevice::WriteOnly));
    idleUnavailable.close();
    widget.refresh();
    CHECK(waitUntil([&]() { return capabilities->text().contains(QStringLiteral("conservative duration cap applies")); }));
    CHECK(idle->isEnabled() && sleepLock->isEnabled() && screenLock->isEnabled());
    CHECK(unlock->isEnabled() && lock->isEnabled());

    QFile capabilityLimited(directory.filePath(QStringLiteral("security-capability-limited")));
    CHECK(capabilityLimited.open(QIODevice::WriteOnly));
    capabilityLimited.close();
    widget.refresh();
    CHECK(waitUntil([&]() { return status->text().contains(QStringLiteral("Required native lock capabilities are unavailable")); }));
    CHECK(idle->isEnabled() && screenLock->isEnabled() && sleepLock->isEnabled());
    CHECK(savePolicy->isEnabled() && unlock->isEnabled() && lock->isEnabled());
    idle->setValue(480);
    screenLock->setChecked(false);
    CHECK(QMetaObject::invokeMethod(&widget, "savePolicy", Qt::DirectConnection));
    CHECK(waitUntil([&]() { return readFile(directory.filePath(QStringLiteral("security-policy-args"))).contains("480"); }));
    CHECK(readFile(directory.filePath(QStringLiteral("security-policy-args"))).endsWith("--auth\nauto\n"));

    QFile::remove(directory.filePath(QStringLiteral("security-idle-unavailable")));
    QFile::remove(directory.filePath(QStringLiteral("security-capability-limited")));
    QFile unlockFailure(directory.filePath(QStringLiteral("security-unlock-fails")));
    CHECK(unlockFailure.open(QIODevice::WriteOnly));
    unlockFailure.close();
    auto *refreshButton = widget.findChild<QPushButton *>(QStringLiteral("remoteSecurityRefresh"));
    CHECK(refreshButton && waitUntil([&]() { return refreshButton->isEnabled(); }));
    unlock->click();
    CHECK(waitUntil([&]() { return widget.findChild<QLabel *>(QStringLiteral("remoteSecurityDetail"))
        ->text().contains(QStringLiteral("Fixture native authentication was unavailable.")); }));

    QFile unsupported(directory.filePath(QStringLiteral("security-unavailable")));
    CHECK(unsupported.open(QIODevice::WriteOnly));
    unsupported.close();
    CHECK(waitUntil([&]() { return refreshButton->isEnabled(); }));
    widget.refresh();
    CHECK(waitUntil([&]() { return status->text().contains(QStringLiteral("security controls are unavailable on this host")); }));
    CHECK(!savePolicy->isEnabled() && unlock->isEnabled() && lock->isEnabled());

    std::puts("Remote security status, CLI policy updates, lock/unlock and unavailable capabilities passed.");
    return 0;
}
