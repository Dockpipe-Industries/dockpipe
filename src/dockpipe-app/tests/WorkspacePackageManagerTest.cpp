#include "WorkspacePackageManager.h"

#include <QApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>

#ifdef Q_OS_WIN
#ifndef NOMINMAX
#define NOMINMAX
#endif
#include <windows.h>
#endif

namespace {
QString libraryName()
{
#ifdef Q_OS_WIN
    return QStringLiteral("dockpipe-cockpit.dll");
#elif defined(Q_OS_MACOS)
    return QStringLiteral("libdockpipe-cockpit.dylib");
#else
    return QStringLiteral("libdockpipe-cockpit.so");
#endif
}

bool createSymbolicLink(const QString &target, const QString &link, bool directory)
{
#ifdef Q_OS_WIN
    const QString nativeTarget = QDir::toNativeSeparators(target);
    const QString nativeLink = QDir::toNativeSeparators(link);
    const DWORD flags = directory ? SYMBOLIC_LINK_FLAG_DIRECTORY : 0;
    // Developer mode permits unprivileged symlinks on current Windows. Older
    // policy may instead grant the runner SeCreateSymbolicLinkPrivilege.
    if (CreateSymbolicLinkW(reinterpret_cast<LPCWSTR>(nativeLink.utf16()),
            reinterpret_cast<LPCWSTR>(nativeTarget.utf16()), flags | 0x2))
        return true;
    return CreateSymbolicLinkW(reinterpret_cast<LPCWSTR>(nativeLink.utf16()),
        reinterpret_cast<LPCWSTR>(nativeTarget.utf16()), flags);
#else
    Q_UNUSED(directory);
    return QFile::link(target, link);
#endif
}
}

class WorkspacePackageManagerTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void canonicalNativeLibraryLoads_data();
    void canonicalNativeLibraryLoads();
    void rejectsOutsideSiblingSymlinks_data();
    void rejectsOutsideSiblingSymlinks();
    void retainsErrorAndClearsItAfterSuccessfulRetry();
    void cleanupTestCase();
private:
    QByteArray m_originalAssets;
    QString m_assets;
};

void WorkspacePackageManagerTest::initTestCase()
{
    m_originalAssets = qgetenv("DOCKPIPE_WORKSPACE_PACKAGE");
    QVERIFY2(!m_originalAssets.isEmpty(), "Set DOCKPIPE_WORKSPACE_PACKAGE to the built native fixture package.");
    m_assets = QFileInfo(QString::fromUtf8(m_originalAssets)).canonicalFilePath();
    QVERIFY(!m_assets.isEmpty());
    QVERIFY(QFileInfo(QDir(m_assets).filePath(QStringLiteral("ui/") + libraryName())).isFile());
}

void WorkspacePackageManagerTest::canonicalNativeLibraryLoads_data()
{
    QTest::addColumn<QString>("assets");
    QTest::newRow("canonical-forward-slashes") << QDir::fromNativeSeparators(m_assets);
#ifdef Q_OS_WIN
    QTest::newRow("native-backslashes") << QDir::toNativeSeparators(m_assets);
#endif
}

void WorkspacePackageManagerTest::canonicalNativeLibraryLoads()
{
    QFETCH(QString, assets);
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", assets.toUtf8());
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    LauncherSettings settings;
    settings.projectFolder = directory.path();
    WorkspacePackageManager manager;
    QSignalSpy prepared(&manager, &WorkspacePackageManager::prepared);
    QSignalSpy failed(&manager, &WorkspacePackageManager::failed);
    manager.prepare(settings, false);
    QVERIFY2(manager.ready(), qPrintable(manager.error()));
    QVERIFY(!manager.busy());
    QCOMPARE(prepared.size(), 1);
    QCOMPARE(failed.size(), 0);
    QVERIFY(manager.error().isEmpty());
#ifdef Q_OS_WIN
    // This is the original regression: native separator differs from Qt's
    // canonical representation, while the DLL is still a legitimate child.
    QVERIFY(m_assets.contains(QLatin1Char('/')));
    QCOMPARE(QDir::separator(), QChar('\\'));
#endif
}

void WorkspacePackageManagerTest::rejectsOutsideSiblingSymlinks_data()
{
    QTest::addColumn<bool>("directoryLink");
    QTest::newRow("library-symlink") << false;
    QTest::newRow("ui-directory-symlink") << true;
}

void WorkspacePackageManagerTest::rejectsOutsideSiblingSymlinks()
{
    QFETCH(bool, directoryLink);
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString assets = directory.filePath(QStringLiteral("assets"));
    const QString outside = assets + QStringLiteral("-other");
    const QString outsideUi = QDir(outside).filePath(QStringLiteral("ui"));
    const QString outsideLibrary = QDir(outsideUi).filePath(libraryName());
    const QString insideUi = QDir(assets).filePath(QStringLiteral("ui"));
    QVERIFY(QDir().mkpath(assets));
    QVERIFY(QDir().mkpath(outsideUi));
    QVERIFY(QFile::copy(QDir(m_assets).filePath(QStringLiteral("ui/") + libraryName()), outsideLibrary));
    if (!directoryLink)
        QVERIFY(QDir().mkpath(insideUi));
    const QString target = directoryLink ? outsideUi : outsideLibrary;
    const QString link = directoryLink ? insideUi : QDir(insideUi).filePath(libraryName());
    const bool linked = createSymbolicLink(target, link, directoryLink);
#ifdef Q_OS_WIN
    if (!linked)
        QSKIP("The native Windows runner cannot create symbolic links under its current filesystem policy.");
#else
    QVERIFY(linked);
#endif
    // A naive root string prefix accepts "assets-other". The actual library is
    // otherwise a valid fixture, so only package containment must reject it.
    QVERIFY(QFileInfo(QDir(insideUi).filePath(libraryName())).isFile());
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", assets.toUtf8());
    LauncherSettings settings;
    settings.projectFolder = directory.path();
    WorkspacePackageManager manager;
    QSignalSpy prepared(&manager, &WorkspacePackageManager::prepared);
    QSignalSpy failed(&manager, &WorkspacePackageManager::failed);
    manager.prepare(settings, false);
    QVERIFY(!manager.ready());
    QCOMPARE(prepared.size(), 0);
    QCOMPARE(failed.size(), 1);
    QVERIFY(manager.error().contains(QStringLiteral("native UI component")));
    QCOMPARE(failed.front().front().toString(), manager.error());
}

void WorkspacePackageManagerTest::retainsErrorAndClearsItAfterSuccessfulRetry()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", directory.path().toUtf8());
    LauncherSettings settings;
    settings.projectFolder = directory.path();
    WorkspacePackageManager manager;
    QSignalSpy failed(&manager, &WorkspacePackageManager::failed);
    manager.prepare(settings, false);
    QVERIFY(!manager.ready());
    QCOMPARE(failed.size(), 1);
    QCOMPARE(manager.error(), failed.front().front().toString());
    QVERIFY(!manager.error().isEmpty());
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", m_assets.toUtf8());
    manager.prepare(settings, false);
    QVERIFY2(manager.ready(), qPrintable(manager.error()));
    QVERIFY(manager.error().isEmpty());
}

void WorkspacePackageManagerTest::cleanupTestCase()
{
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", m_originalAssets);
}

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    WorkspacePackageManagerTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "WorkspacePackageManagerTest.moc"
