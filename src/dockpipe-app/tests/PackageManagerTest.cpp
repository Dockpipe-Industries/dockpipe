#include "PackageManagerDialog.h"
#include "LauncherSettings.h"

#include <QApplication>
#include <QDir>
#include <QComboBox>
#include <QLineEdit>
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
        echo '{"packages":[{"name":"installed-tool","kind":"workflow","version":"1.0.0","source":"System","removable":false},{"name":"remote-tool","title":"Example desktop workspace","catalog_type":"app","categories":["development"],"kind":"workflow","version":"1.2.3","size_bytes":2048,"source":"User","path":"/user/packages/workflows/dockpipe-workflow-remote-tool-1.2.3.tar.gz","removable":true}],"install_root":"/user/packages","warnings":[]}'
    else
        echo '{"packages":[{"name":"installed-tool","kind":"workflow","version":"1.0.0","source":"System","removable":false}],"install_root":"/user/packages","warnings":[]}'
    fi
    ;;
catalog)
    if [ -f slow-catalog ]; then
        exec sleep 10
    fi
    if [ -f provider-catalog ]; then
        echo '{"manifest":"https://example.com/pinned/store.json","platform":"test-platform","packages":[{"name":"example.tunnel","kind":"resolver","capability":"remote.edge","version":"1.0.0","sha256":"provider-digest","size_bytes":1073741824},{"name":"example.hosted","kind":"resolver","capability":"remote.broker","version":"1.0.0","sha256":"hosted-digest","size_bytes":-1},{"name":"unrelated","kind":"resolver","capability":"editor"},{"name":"misleading-workflow","kind":"workflow","capability":"remote.edge"}]}'
        exit 0
    fi
    echo '{"manifest":"https://example.com/pinned/store.json","platform":"test-platform","packages":[{"name":"remote-tool","title":"Example desktop workspace","catalog_type":"app","categories":["development"],"kind":"workflow","version":"1.2.3","sha256":"selection-digest","size_bytes":1572864}]}'
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
    auto *typeFilter = dialog.findChild<QComboBox *>(QStringLiteral("packageTypeFilter"));
    if (!typeFilter || marketplace->item(0, 0)->text() != QStringLiteral("Example desktop workspace")
        || marketplace->item(0, 2)->text() != QStringLiteral("app"))
        return 1;
    typeFilter->setCurrentIndex(typeFilter->findData(QStringLiteral("app")));
    if (installed->rowCount() != 0 || marketplace->rowCount() != 1)
        return 1;
    typeFilter->setCurrentIndex(typeFilter->findData(QStringLiteral("integration")));
    if (marketplace->rowCount() != 0)
        return 1;
    typeFilter->setCurrentIndex(0);
    if (install->isVisible() || !uninstall->isVisible() || uninstall->isEnabled() || cancel->isVisible()) {
        std::fprintf(stderr, "Idle installed actions are misleading\n");
        return 1;
    }
    tabs->setCurrentIndex(1);
    if (marketplace->item(0, 4)->text() != QStringLiteral("1.5 MiB")
        || installed->item(0, 4)->text() != QStringLiteral("Not available")) {
        std::fprintf(stderr, "Package download size or missing-size fallback is incorrect\n");
        return 1;
    }
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
    if (installed->item(1, 4)->text() != QStringLiteral("2.0 KiB"))
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
    QFile providerCatalog(directory.filePath(QStringLiteral("provider-catalog")));
    if (!providerCatalog.open(QIODevice::WriteOnly))
        return 1;
    providerCatalog.close();
    dialog.showRemoteProviders();
    refresh->click();
    if (!waitUntil([&]() { return marketplace->rowCount() == 2 && install->isEnabled(); })) {
        std::fprintf(stderr, "Provider catalog must include only remote resolver capabilities\n");
        return 1;
    }
    if (tabs->currentIndex() != 1)
        return 1;
    if (marketplace->item(0, 4)->text() != QStringLiteral("1.0 GiB")
        || marketplace->item(1, 4)->text() != QStringLiteral("Not available"))
        return 1;
    QString installedProvider;
    QObject::connect(&dialog, &PackageManagerDialog::packageInstalled, &dialog, [&](const QString &name) {
        installedProvider = name;
    });
    if (!screenshot.isEmpty())
        dialog.grab().save(screenshot + QStringLiteral("-providers.png"));
    install->click();
    if (!waitUntil([&]() { return installedProvider == QStringLiteral("example.tunnel"); }))
        return 1;
    arguments.close();
    if (!arguments.open(QIODevice::ReadOnly)
        || arguments.readAll() != "package\ninstall\n--remote\nhttps://example.com/pinned/store.json\n--kind\nresolver\n--name\nexample.tunnel\n--sha256\nprovider-digest\n") {
        std::fprintf(stderr, "Provider installation did not use the pinned package selection\n");
        return 1;
    }
    settings.packageRemotes.clear();
    if (!settings.save() || !LauncherSettings::current().packageRemotes.isEmpty())
        return 1;
    return 0;
}
