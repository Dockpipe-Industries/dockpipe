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
    file.write(R"SCRIPT(#!/bin/sh
case "$2" in
list)
    if [ -f installed-marker ]; then
        echo '{"packages":[{"name":"installed-tool","kind":"workflow","version":"1.0.0","source":"System","removable":false},{"name":"remote-tool","kind":"workflow","version":"1.2.3","source":"User","path":"/user/packages/workflows/dockpipe-workflow-remote-tool-1.2.3.tar.gz","removable":true}],"install_root":"/user/packages","warnings":[]}'
    else
        echo '{"packages":[{"name":"installed-tool","kind":"workflow","version":"1.0.0","source":"System","removable":false}],"install_root":"/user/packages","warnings":[]}'
    fi
    ;;
catalog)
    if [ -f slow-catalog ]; then
        exec sleep 10
    fi
    echo '{"manifest":"https://example.com/pinned/store.json","platform":"test-platform","packages":[{"name":"remote-tool","kind":"workflow","version":"1.2.3","sha256":"selection-digest"}]}'
    ;;
install)
    printf '%s\n' "$@" > install-args
    touch installed-marker
    echo '{"path":"/user/packages/workflows/dockpipe-workflow-remote-tool-1.2.3.tar.gz"}'
    ;;
uninstall)
    printf '%s\n' "$@" > uninstall-args
    rm installed-marker
    echo '{"path":"/user/packages/workflows/dockpipe-workflow-remote-tool-1.2.3.tar.gz"}'
    ;;
esac
)SCRIPT");
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
    auto *uninstall = dialog.findChild<QPushButton *>(QStringLiteral("uninstallPackage"));
    auto *cancel = dialog.findChild<QPushButton *>(QStringLiteral("cancelPackageOperation"));
    auto *refresh = dialog.findChild<QPushButton *>(QStringLiteral("refreshPackages"));
    if (!waitUntil([&]() { return installed->rowCount() == 1 && marketplace->rowCount() == 1; })) {
        std::fprintf(stderr, "Package tables did not load\n");
        return 1;
    }
    if (install->isVisible() || !uninstall->isVisible() || uninstall->isEnabled() || cancel->isVisible()) {
        std::fprintf(stderr, "Idle installed actions are misleading\n");
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
    if (!waitUntil([&]() { return installed->rowCount() == 2 && !cancel->isVisible(); }))
        return 1;
    if (install->isEnabled() || install->text() != QStringLiteral("Installed")) {
        std::fprintf(stderr, "Marketplace still offers duplicate installation\n");
        return 1;
    }
    tabs->setCurrentIndex(0);
    installed->selectRow(1);
    if (!uninstall->isEnabled() || install->isVisible())
        return 1;
    uninstall->click();
    if (!waitUntil([&]() { return installed->rowCount() == 1 && !cancel->isVisible(); }))
        return 1;
    QFile removedArguments(directory.filePath(QStringLiteral("uninstall-args")));
    if (!removedArguments.open(QIODevice::ReadOnly)
        || removedArguments.readAll() != "package\nuninstall\n--path\n/user/packages/workflows/dockpipe-workflow-remote-tool-1.2.3.tar.gz\n")
        return 1;
    tabs->setCurrentIndex(1);
    if (!install->isEnabled())
        return 1;
    QFile slowCatalog(directory.filePath(QStringLiteral("slow-catalog")));
    if (!slowCatalog.open(QIODevice::WriteOnly))
        return 1;
    slowCatalog.close();
    refresh->click();
    if (!cancel->isVisible() || !cancel->isEnabled())
        return 1;
    cancel->click();
    if (!waitUntil([&]() { return !cancel->isVisible() && status->text().contains(QStringLiteral("cancelled")); })) {
        std::fprintf(stderr, "Cancel failed to stop the operation and restore idle controls\n");
        return 1;
    }
    slowCatalog.remove();
    refresh->click();
    if (!waitUntil([&]() { return !cancel->isVisible() && marketplace->rowCount() == 1; }))
        return 1;
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_PACKAGE_TEST_SCREENSHOT");
    if (!screenshot.isEmpty())
        dialog.grab().save(screenshot);
    settings.packageRemotes.clear();
    if (!settings.save() || !LauncherSettings::current().packageRemotes.isEmpty())
        return 1;
    return 0;
}
