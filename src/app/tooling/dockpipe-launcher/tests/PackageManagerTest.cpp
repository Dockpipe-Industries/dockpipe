#include "PackageManagerDialog.h"
#include "LauncherSettings.h"

#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QPushButton>
#include <QTabWidget>
#include <QTableWidget>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>
#include <functional>

static bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (!predicate() && elapsed.elapsed() < 5000) {
        QCoreApplication::processEvents();
        QThread::msleep(5);
    }
    return predicate();
}

int main(int argc, char **argv)
{
    QTemporaryDir directory;
    qputenv("HOME", directory.path().toUtf8());
    qputenv("XDG_CONFIG_HOME", directory.path().toUtf8());
    QApplication app(argc, argv);
    QCoreApplication::setApplicationName(QStringLiteral("package-manager-test"));
    const QString script = directory.filePath(QStringLiteral("dockpipe"));
    QFile file(script);
    if (!file.open(QIODevice::WriteOnly))
        return 1;
    file.write("#!/bin/sh\ncase \"$2\" in\n"
               "list) echo '{\"packages\":[{\"name\":\"installed-tool\",\"kind\":\"workflow\",\"version\":\"1.0.0\",\"source\":\"System\"}],\"install_root\":\"/user/packages\",\"warnings\":[]}' ;;\n"
               "catalog) echo '{\"manifest\":\"https://example.com/pinned/store.json\",\"platform\":\"test-platform\",\"packages\":[{\"name\":\"remote-tool\",\"kind\":\"workflow\",\"version\":\"1.2.3\",\"sha256\":\"selection-digest\"}]}' ;;\n"
               "install) printf '%s\\n' \"$@\" > install-args; echo '{\"path\":\"/user/packages/remote-tool.tar.gz\"}' ;;\nesac\n");
    file.close();
    file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner);
    qputenv("DOCKPIPE_BIN", script.toUtf8());
    LauncherSettings settings;
    settings.load();
    if (settings.packageRemotes != QStringList{QStringLiteral("https://packages.dockpipe.com")})
        return 1;
    settings.packageRemotes = {QStringLiteral("https://example.com")};
    if (!settings.save())
        return 1;
    PackageManagerDialog dialog(directory.path());
    dialog.show();
    auto *installed = dialog.findChild<QTableWidget *>(QStringLiteral("installedPackages"));
    auto *marketplace = dialog.findChild<QTableWidget *>(QStringLiteral("marketplacePackages"));
    auto *tabs = dialog.findChild<QTabWidget *>();
    auto *install = dialog.findChild<QPushButton *>(QStringLiteral("installPackage"));
    auto *status = dialog.findChild<QLabel *>(QStringLiteral("packageStatus"));
    if (!waitUntil([&]() { return installed->rowCount() == 1 && marketplace->rowCount() == 1; })) {
        std::fprintf(stderr, "Package tables did not load\n");
        return 1;
    }
    tabs->setCurrentIndex(1);
    if (!install->isEnabled())
        return 1;
    install->click();
    if (!waitUntil([&]() { return status->text().startsWith(QStringLiteral("Installed:")); }))
        return 1;
    QFile arguments(directory.filePath(QStringLiteral("install-args")));
    if (!arguments.open(QIODevice::ReadOnly))
        return 1;
    const QByteArray expected = "package\ninstall\n--remote\nhttps://example.com/pinned/store.json\n--kind\nworkflow\n--name\nremote-tool\n--sha256\nselection-digest\n";
    if (arguments.readAll() != expected) {
        std::fprintf(stderr, "Install did not pin the displayed selection\n");
        return 1;
    }
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_PACKAGE_TEST_SCREENSHOT");
    if (!screenshot.isEmpty())
        dialog.grab().save(screenshot);
    settings.packageRemotes.clear();
    if (!settings.save() || !LauncherSettings::current().packageRemotes.isEmpty())
        return 1;
    return 0;
}
