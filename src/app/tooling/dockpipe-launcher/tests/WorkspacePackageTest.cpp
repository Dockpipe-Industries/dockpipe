#include "WorkspacePackageManager.h"
#include "SettingsDialog.h"
#include <QApplication>
#include <QCheckBox>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QPushButton>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>

#define CHECK(value) do { if (!(value)) { std::fprintf(stderr, "Package assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

static bool waitFor(const std::function<bool()> &predicate, int timeout = 5000)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (!predicate() && elapsed.elapsed() < timeout) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    return predicate();
}

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication app(argc, argv);
    const QString remote = qEnvironmentVariable("DOCKPIPE_PACKAGE_TEST_REMOTE");
    if (!remote.isEmpty()) {
        QTemporaryDir workdir;
        LauncherSettings settings;
        settings.projectFolder = workdir.path();
        settings.packageRemotes = {remote};
        const QString action = qEnvironmentVariable("DOCKPIPE_PACKAGE_TEST_ACTION");
        if (action == QStringLiteral("disable")) {
            CHECK(settings.load());
            CHECK(settings.cockpit.enabled);
            SettingsDialog dialog(settings);
            dialog.findChild<QCheckBox *>("settingsCockpitEnabled")->setChecked(false);
            const auto disabled = dialog.updatedSettings();
            CHECK(!disabled.cockpit.enabled);
            CHECK(disabled.save());
            LauncherSettings readback;
            CHECK(readback.load());
            CHECK(!readback.cockpit.enabled);
            std::puts("Marketplace Cockpit disabled; persisted settings read back.");
            return 0;
        }
        CHECK(action.isEmpty() || action == QStringLiteral("enable"));
        SettingsDialog dialog(settings);
        dialog.findChild<QCheckBox *>("settingsCockpitEnabled")->setChecked(true);
        CHECK(!dialog.updatedSettings().cockpit.enabled);
        CHECK(waitFor([]() { return !WorkspacePackageManager::instance().busy(); }, 120000));
        const QString status = dialog.findChild<QLabel *>("cockpitPackageStatus")->text();
        std::fprintf(stderr, "%s\n", qPrintable(status));
        CHECK(dialog.updatedSettings().cockpit.enabled);
        if (action == QStringLiteral("enable")) {
            CHECK(dialog.updatedSettings().save());
            LauncherSettings readback;
            CHECK(readback.load());
            CHECK(readback.cockpit.enabled);
        }
        auto *surface = WorkspacePackageManager::instance().createWorkspace(nullptr);
        CHECK(surface);
        surface->applySettings(CockpitSettings{true});
        surface->setWorkspace(workdir.path());
        delete surface;
        std::puts("Real HTTPS package install through Settings and native surface construction passed.");
        return 0;
    }
#ifdef Q_OS_WIN
    return 77; // The deterministic CLI fixture below is POSIX; native loader is cross-platform.
#endif
    const QByteArray assets = qgetenv("DOCKPIPE_WORKSPACE_PACKAGE");
    CHECK(!assets.isEmpty());
    qunsetenv("DOCKPIPE_WORKSPACE_PACKAGE");
    QTemporaryDir directory;
    CHECK(directory.isValid());
    QFile cli(directory.filePath("dockpipe"));
    CHECK(cli.open(QIODevice::WriteOnly));
    cli.write(R"SH(#!/bin/sh
printf '%s\n' "$*" >> "$FIXTURE_ROOT/commands"
if [ "$1" = scope ]; then
    if [ -f "$FIXTURE_ROOT/installed" ]; then
        printf '%s\n' "$FIXTURE_ASSETS"
        exit 0
    fi
    exit 1
fi
if [ "$FIXTURE_MODE" = slow ]; then
    exec sleep 20
fi
if [ "$2" = catalog ]; then
    checksum=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
    if [ "$FIXTURE_MODE" = checksum ]; then checksum=bad; fi
    printf '{"packages":[{"kind":"resolver","name":"cockpit","sha256":"%s"}]}' "$checksum"
    exit 0
fi
if [ "$2" = install ]; then
    if [ "$FIXTURE_MODE" = failure ]; then
        echo 'Fixture download failed' >&2
        exit 1
    fi
    touch "$FIXTURE_ROOT/installed"
    printf '{}'
    exit 0
fi
exit 2
)SH");
    cli.close();
    CHECK(cli.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner));
    qputenv("DOCKPIPE_BIN", cli.fileName().toUtf8());
    qputenv("FIXTURE_ROOT", directory.path().toUtf8());
    qputenv("FIXTURE_ASSETS", assets);
    LauncherSettings settings;
    settings.projectFolder = directory.path();
    settings.packageRemotes = {"https://packages.example.test"};
    WorkspacePackageManager manager;
    int failures = 0;
    int successes = 0;
    QString message;
    QObject::connect(&manager, &WorkspacePackageManager::failed, [&](const QString &error) { ++failures; message = error; });
    QObject::connect(&manager, &WorkspacePackageManager::prepared, [&]() { ++successes; });
    qputenv("FIXTURE_MODE", "checksum");
    manager.prepare(settings);
    CHECK(waitFor([&]() { return failures == 1; }));
    CHECK(!manager.ready() && message.contains("checksum"));
    qputenv("FIXTURE_MODE", "failure");
    manager.prepare(settings);
    CHECK(waitFor([&]() { return failures == 2; }));
    CHECK(!manager.ready() && message.contains("download failed"));
    qputenv("FIXTURE_MODE", "slow");
    manager.prepare(settings);
    CHECK(waitFor([&]() {
        QFile commands(directory.filePath("commands"));
        commands.open(QIODevice::ReadOnly);
        return commands.readAll().count("package catalog") == 3;
    }));
    manager.cancel();
    CHECK(waitFor([&]() { return !manager.busy(); }));
    CHECK(failures == 2 && successes == 0 && !manager.ready());
    qputenv("FIXTURE_MODE", "success");
    manager.prepare(settings);
    CHECK(waitFor([&]() { return successes == 1; }));
    CHECK(manager.ready());
    QFile commands(directory.filePath("commands"));
    CHECK(commands.open(QIODevice::ReadOnly));
    CHECK(commands.readAll().contains("package install --remote https://packages.example.test/packages/latest.json --kind resolver --name cockpit --sha256 aaaa"));

    // Settings cannot save an enabled preference while its download is pending.
    CHECK(QFile::remove(directory.filePath("installed")));
    qputenv("FIXTURE_MODE", "slow");
    SettingsDialog dialog(settings);
    auto *enabled = dialog.findChild<QCheckBox *>("settingsCockpitEnabled");
    enabled->setChecked(true);
    CHECK(!dialog.updatedSettings().cockpit.enabled);
    CHECK(!dialog.findChild<QWidget *>("cockpitSettingsOptions")->isEnabled());
    dialog.reject();
    CHECK(waitFor([&]() { return !WorkspacePackageManager::instance().busy(); }));
    qputenv("FIXTURE_MODE", "failure");
    SettingsDialog failedDialog(settings);
    failedDialog.findChild<QCheckBox *>("settingsCockpitEnabled")->setChecked(true);
    CHECK(waitFor([&]() { return !WorkspacePackageManager::instance().busy(); }));
    CHECK(!failedDialog.updatedSettings().cockpit.enabled);
    CHECK(failedDialog.findChild<QLabel *>("cockpitPackageStatus")->text().contains("download failed"));
    CHECK(!failedDialog.findChild<QPushButton *>("cockpitPackageRetry")->isHidden());
    std::puts("Package discovery, verified install delegation, cancellation, failure and settings gates passed.");
    return 0;
}
