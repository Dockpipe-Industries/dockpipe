#include "TerminalShellCatalog.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QSignalSpy>
#include <QPointer>
#include <QProcess>
#include <QSet>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <cstdio>
#ifdef Q_OS_WIN
#include <fcntl.h>
#include <io.h>
#endif

namespace {
bool writeExecutable(const QString &path)
{
    if (!QDir().mkpath(QFileInfo(path).absolutePath()))
        return false;
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write("fixture; never executed\n") > 0
        && file.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
}

QByteArray utf16(const QString &text)
{
    QByteArray bytes("\xff\xfe", 2);
    for (QChar character : text) {
        bytes.append(static_cast<char>(character.unicode() & 0xff));
        bytes.append(static_cast<char>(character.unicode() >> 8));
    }
    return bytes;
}

TerminalShellCatalog::Discovery probeDiscovery(bool flatpak)
{
    TerminalShellCatalog::Discovery discovery;
    discovery.windows = !flatpak;
    discovery.flatpak = flatpak;
    discovery.wslProgram = QCoreApplication::applicationFilePath();
    discovery.flatpakSpawnProgram = QCoreApplication::applicationFilePath();
    return discovery;
}
}

class TerminalShellCatalogTest final : public QObject {
    Q_OBJECT
private slots:
    void cleanup();
    void discoversInstalledWindowsVersionsWithoutLaunchingThem();
    void discoversUnixShellsAndDeduplicatesPaths();
    void parsesWslNamesAsStructuredArguments();
    void flatpakUsesOnlyHostProfiles();
    void rejectsIncompleteAndOversizedProbeOutput_data();
    void rejectsIncompleteAndOversizedProbeOutput();
    void cancelsAndTimesOutWithoutBlocking();
    void boundsLocalDiscovery();
    void destructionReapsListingProcess();
    void hostScriptOnlyEnumeratesExecutables();
};

void TerminalShellCatalogTest::cleanup()
{
    qunsetenv("DOCKPIPE_SHELL_TEST_MODE");
    qunsetenv("DOCKPIPE_SHELL_TEST_OUTPUT");
}

void TerminalShellCatalogTest::discoversInstalledWindowsVersionsWithoutLaunchingThem()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalShellCatalog::Discovery discovery;
    discovery.windows = true;
    discovery.windowsDirectory = directory.filePath("Windows");
    discovery.powerShellRoots = {directory.filePath("PowerShell")};
    discovery.gitRoots = {directory.filePath("Git")};
    discovery.searchDirectories = {directory.filePath("PowerShell/7")};
    for (const QString &relative : {QString("Windows/System32/WindowsPowerShell/v1.0/powershell.exe"),
             QString("PowerShell/7/pwsh.exe"), QString("PowerShell/7-preview/pwsh.exe"),
             QString("PowerShell/7.4.2/pwsh.exe"), QString("Git/bin/bash.exe")})
        QVERIFY(writeExecutable(directory.filePath(relative)));
    QVERIFY(QDir().mkpath(directory.filePath("Git/mingw64")));
    TerminalShellCatalog catalog(discovery);
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    catalog.refresh();
    QCOMPARE(finished.size(), 1);
    QVERIFY(catalog.error().isEmpty());
    QCOMPARE(catalog.profiles().size(), 5);
    QCOMPARE(catalog.profiles().first().label, QString("Windows PowerShell"));
    QSet<QString> labels;
    for (const auto &profile : catalog.profiles()) {
        labels.insert(profile.label);
        QVERIFY(QFileInfo(profile.program).isFile());
        QVERIFY(!profile.id.isEmpty());
        if (profile.label == "Git Bash") {
            QCOMPARE(profile.arguments, QStringList({"--login", "-i"}));
            QCOMPARE(profile.environment.value("CHERE_INVOKING"), QString("1"));
            QCOMPARE(profile.environment.value("MSYSTEM"), QString("MINGW64"));
        }
    }
    QVERIFY(labels.contains("PowerShell 7-preview"));
    QVERIFY(labels.contains("PowerShell 7.4.2"));
    QVERIFY(labels.contains("Git Bash"));
}

void TerminalShellCatalogTest::discoversUnixShellsAndDeduplicatesPaths()
{
#ifdef Q_OS_WIN
    QSKIP("Unix executable permission and shell-list fixture.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString shell = directory.filePath("bin/zsh");
    QVERIFY(writeExecutable(shell));
    QVERIFY(writeExecutable(directory.filePath("bin/pwsh")));
    TerminalShellCatalog::Discovery discovery;
    discovery.loginShell = shell;
    discovery.searchDirectories = {directory.filePath("bin")};
    discovery.shellsFile = directory.filePath("shells");
    QFile list(discovery.shellsFile);
    QVERIFY(list.open(QIODevice::WriteOnly));
    list.write(("# installed shells\n" + shell + "\n/nonexistent/shell\n" + shell + "\n").toUtf8());
    list.close();
    TerminalShellCatalog catalog(discovery);
    catalog.refresh();
    QVERIFY(catalog.error().isEmpty());
    QCOMPARE(catalog.profiles().size(), 2);
    QCOMPARE(catalog.profiles().first().program, QFileInfo(shell).canonicalFilePath());
    QCOMPARE(catalog.profiles().last().label, QString("PowerShell"));
#endif
}

void TerminalShellCatalogTest::parsesWslNamesAsStructuredArguments()
{
    qputenv("DOCKPIPE_SHELL_TEST_MODE", "output");
    qputenv("DOCKPIPE_SHELL_TEST_OUTPUT", utf16("Example Linux\r\nOther;literal\r\nExample Linux\r\n").toHex());
    TerminalShellCatalog catalog(probeDiscovery(false));
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    catalog.refresh();
    QVERIFY(catalog.discovering());
    QTRY_COMPARE(finished.size(), 1);
    QVERIFY(catalog.error().isEmpty());
    QCOMPARE(catalog.profiles().size(), 2);
    QCOMPARE(catalog.profiles()[0].arguments, QStringList({"--distribution", "Example Linux", "--exec", "bash", "-li"}));
    QCOMPARE(catalog.profiles()[1].arguments, QStringList({"--distribution", "Other;literal", "--exec", "bash", "-li"}));
    QCOMPARE(catalog.profiles()[0].label, QString("WSL Bash · Example Linux"));
    QCOMPARE(catalog.profiles()[0].workingDirectoryArgument, QString("--cd"));
}

void TerminalShellCatalogTest::flatpakUsesOnlyHostProfiles()
{
    qputenv("DOCKPIPE_SHELL_TEST_MODE", "output");
    // These paths deliberately do not exist in the sandbox's filesystem.
    QByteArray output;
    const QStringList fields{"/host/bin/zsh", "/host/bin/zsh", "/host/bin/pwsh", "/host/bin/pwsh",
        "/host/alias/zsh", "/host/bin/zsh", "/host/bin/sh", "/host/bin/zsh"};
    for (const QString &field : fields)
        output += field.toUtf8() + '\0';
    qputenv("DOCKPIPE_SHELL_TEST_OUTPUT", output.toHex());
    auto discovery = probeDiscovery(true);
    discovery.loginShell = "/sandbox/bin/sh";
    TerminalShellCatalog catalog(discovery);
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    catalog.refresh();
    QVERIFY(catalog.profiles().isEmpty());
    QTRY_COMPARE(finished.size(), 1);
    QVERIFY(catalog.error().isEmpty());
    QCOMPARE(catalog.profiles().size(), 3);
    QCOMPARE(catalog.profiles().first().program, QString("/host/bin/zsh"));
    QCOMPARE(catalog.profiles().last().program, QString("/host/bin/sh"));
    QVERIFY(catalog.profiles().first().arguments.isEmpty());
}

void TerminalShellCatalogTest::rejectsIncompleteAndOversizedProbeOutput_data()
{
    QTest::addColumn<QByteArray>("output");
    QTest::newRow("missing-nul") << QByteArray("/host/bin/bash");
    QTest::newRow("relative-path") << (QByteArray("relative/bash") + '\0');
    QTest::newRow("output-limit") << QByteArray(65537, 'x');
    QByteArray many;
    for (int index = 0; index < 129; ++index)
        many += (QByteArray("/host/shell") + QByteArray::number(index) + '\0').repeated(2);
    QTest::newRow("profile-limit") << many;
}

void TerminalShellCatalogTest::rejectsIncompleteAndOversizedProbeOutput()
{
    QFETCH(QByteArray, output);
    qputenv("DOCKPIPE_SHELL_TEST_MODE", output.size() > 65536 ? "flood" : "output");
    qputenv("DOCKPIPE_SHELL_TEST_OUTPUT", output.size() > 65536 ? QByteArray() : output.toHex());
    TerminalShellCatalog catalog(probeDiscovery(true));
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    catalog.refresh();
    QTRY_COMPARE(finished.size(), 1);
    QVERIFY(!catalog.error().isEmpty());
    QVERIFY(catalog.profiles().isEmpty());
    QVERIFY(!catalog.discovering());
}

void TerminalShellCatalogTest::cancelsAndTimesOutWithoutBlocking()
{
    qputenv("DOCKPIPE_SHELL_TEST_MODE", "wait");
    TerminalShellCatalog catalog(probeDiscovery(true));
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    QElapsedTimer elapsed;
    elapsed.start();
    catalog.refresh();
    catalog.cancel();
    QVERIFY(elapsed.elapsed() < 100);
    QTRY_COMPARE(finished.size(), 1);
    QVERIFY(catalog.error().contains("cancelled"));
    catalog.refresh();
    QTRY_COMPARE_WITH_TIMEOUT(finished.size(), 2, 5000);
    QVERIFY(catalog.error().contains("timed out"));
    QVERIFY(catalog.profiles().isEmpty());
}

void TerminalShellCatalogTest::boundsLocalDiscovery()
{
    TerminalShellCatalog::Discovery discovery;
    for (int index = 0; index < 257; ++index)
        discovery.searchDirectories.append(QString("/shells/%1").arg(index));
    TerminalShellCatalog catalog(discovery);
    catalog.refresh();
    QVERIFY(!catalog.error().isEmpty());
    QVERIFY(catalog.profiles().isEmpty());
}

void TerminalShellCatalogTest::destructionReapsListingProcess()
{
    qputenv("DOCKPIPE_SHELL_TEST_MODE", "wait");
    auto *catalog = new TerminalShellCatalog(probeDiscovery(true));
    catalog->refresh();
    QPointer<QProcess> process = catalog->findChild<QProcess *>();
    QVERIFY(process);
    QTRY_COMPARE(process->state(), QProcess::Running);
    QElapsedTimer elapsed;
    elapsed.start();
    delete catalog;
    QVERIFY(elapsed.elapsed() < 100);
    QTRY_VERIFY(process.isNull());
}

void TerminalShellCatalogTest::hostScriptOnlyEnumeratesExecutables()
{
#ifdef Q_OS_LINUX
    // Execute the exact fixed discovery script through a fixture bridge. This
    // proves its protocol and quoting, not Flatpak portal permission on a host.
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString loginShell = directory.filePath("login shell;literal");
    QVERIFY(writeExecutable(loginShell));
    qputenv("DOCKPIPE_SHELL_TEST_MODE", "host-script");
    qputenv("DOCKPIPE_SHELL_TEST_OUTPUT", loginShell.toUtf8());
    TerminalShellCatalog catalog(probeDiscovery(true));
    QSignalSpy finished(&catalog, &TerminalShellCatalog::discoveryFinished);
    catalog.refresh();
    QTRY_COMPARE(finished.size(), 1);
    QVERIFY2(catalog.error().isEmpty(), qPrintable(catalog.error()));
    QVERIFY(!catalog.profiles().isEmpty());
    QCOMPARE(catalog.profiles().first().program, loginShell);
    for (const auto &profile : catalog.profiles())
        QVERIFY(QFileInfo(profile.program).isExecutable());
#else
    QSKIP("The Flatpak host script targets Linux host utilities.");
#endif
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    const QStringList arguments = application.arguments().mid(1);
    if (arguments == QStringList({"--list", "--quiet"})
        || (arguments.size() == 5 && arguments[0] == "--watch-bus" && arguments[1] == "--host"
            && arguments[2] == "/bin/sh" && arguments[3] == "-c")) {
        if (qgetenv("DOCKPIPE_SHELL_TEST_MODE") == "host-script") {
            QProcess script;
            auto environment = QProcessEnvironment::systemEnvironment();
            environment.insert("SHELL", QString::fromUtf8(qgetenv("DOCKPIPE_SHELL_TEST_OUTPUT")));
            script.setProcessEnvironment(environment);
            script.setProcessChannelMode(QProcess::ForwardedChannels);
            script.start("/bin/sh", {"-c", arguments.last()});
            if (!script.waitForFinished(2000)) {
                script.kill();
                return 2;
            }
            return script.exitCode();
        }
        if (qgetenv("DOCKPIPE_SHELL_TEST_MODE") == "wait") {
            QThread::sleep(30);
            return 1;
        }
#ifdef Q_OS_WIN
        _setmode(_fileno(stdout), _O_BINARY);
#endif
        const QByteArray bytes = qgetenv("DOCKPIPE_SHELL_TEST_MODE") == "flood"
            ? QByteArray(65537, 'x') : QByteArray::fromHex(qgetenv("DOCKPIPE_SHELL_TEST_OUTPUT"));
        std::fwrite(bytes.constData(), 1, static_cast<size_t>(bytes.size()), stdout);
        return 0;
    }
    TerminalShellCatalogTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "TerminalShellCatalogTest.moc"
