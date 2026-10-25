#include "DesktopApplicationCatalog.h"

#include <QDirIterator>
#include <QElapsedTimer>
#include <QFileInfo>
#include <QFileIconProvider>
#include <QProcess>
#include <QSet>
#include <QSettings>
#include <QStandardPaths>
#include <algorithm>

#ifdef Q_OS_WIN
#include <windows.h>
#include <shellapi.h>
#include <shlobj.h>
#endif

QVector<DesktopApplicationEntry> DesktopApplicationCatalog::enumerate(QString *error)
{
    if (error)
        error->clear();
    QVector<DesktopApplicationEntry> result;
#if defined(Q_OS_LINUX) || defined(Q_OS_WIN) || defined(Q_OS_MACOS)
    // One interactive picker catalog: at most 500 apps and 8 MiB of desktop
    // metadata. Bound visited directories too, including ones with no apps.
    constexpr int maximumEntries = 4000;
    constexpr int maximumApplications = 500;
    constexpr qint64 maximumMetadata = 8 * 1024 * 1024;
    QElapsedTimer elapsed;
    elapsed.start();
    int examined = 0;
    qint64 metadataBytes = 0;
    QSet<QString> seen;
    auto roots = QStandardPaths::standardLocations(QStandardPaths::ApplicationsLocation);
#ifdef Q_OS_WIN
    PWSTR commonPrograms = nullptr;
    if (SUCCEEDED(SHGetKnownFolderPath(FOLDERID_CommonPrograms, KF_FLAG_DONT_VERIFY, nullptr, &commonPrograms))) {
        roots.append(QDir::fromNativeSeparators(QString::fromWCharArray(commonPrograms)));
        CoTaskMemFree(commonPrograms);
    }
#elif defined(Q_OS_MACOS)
    // Qt includes user and local app domains, but not Apple's system app domain.
    roots.append(QStringLiteral("/System/Applications"));
#endif
    roots.removeDuplicates();
    for (const QString &root : roots) {
        QStringList pending{root};
        while (!pending.isEmpty()) {
            const QString directory = pending.takeLast();
            QDirIterator entries(directory, QDir::AllEntries | QDir::NoDotAndDotDot);
            while (entries.hasNext()) {
                if (++examined > maximumEntries || elapsed.elapsed() > 3000) {
                    if (error)
                        *error = QStringLiteral("Installed application catalog exceeds its directory or time limit.");
                    return {};
                }
                const QString path = entries.next();
                const QFileInfo info = entries.fileInfo();
                bool applicationBundle = false;
#ifdef Q_OS_MACOS
                applicationBundle = info.isDir() && info.suffix() == "app";
#endif
                if (info.isDir() && !applicationBundle) {
                    if (!info.isSymLink())
                        pending.append(path);
                    continue;
                }
                DesktopApplicationEntry app;
                app.launchTarget = info.absoluteFilePath();
                app.id = QDir(root).relativeFilePath(path).replace('/', '-');
#ifdef Q_OS_LINUX
                if (!info.isFile() || info.suffix() != "desktop")
                    continue;
                // The first location wins, including a hidden user override.
                if (seen.contains(app.id))
                    continue;
                seen.insert(app.id);
                if (info.size() < 0 || info.size() > 128 * 1024)
                    continue;
                metadataBytes += info.size();
                if (metadataBytes > maximumMetadata) {
                    if (error)
                        *error = QStringLiteral("Installed application metadata exceeds the 8 MiB catalog limit.");
                    return {};
                }
                QSettings desktop(path, QSettings::IniFormat);
                desktop.beginGroup("Desktop Entry");
                if (desktop.value("Type").toString() != "Application"
                    || desktop.value("Hidden", false).toBool()
                    || desktop.value("NoDisplay", false).toBool())
                    continue;
                app.name = desktop.value("Name").toString().left(180);
                app.windowClass = desktop.value("StartupWMClass", info.completeBaseName()).toString().left(180);
                app.hasDeclaredWindowClass = desktop.contains("StartupWMClass") && !app.windowClass.trimmed().isEmpty();
                app.iconName = desktop.value("Icon").toString().left(1024);
#elif defined(Q_OS_WIN)
                if (!info.isFile() || info.suffix().compare("lnk", Qt::CaseInsensitive) != 0)
                    continue;
                app.name = info.completeBaseName();
#else
                if (!applicationBundle)
                    continue;
                app.name = info.completeBaseName();
#endif
                if (app.name.isEmpty())
                    continue;
#ifndef Q_OS_LINUX
                if (seen.contains(app.id))
                    continue;
                seen.insert(app.id);
#endif
                if (result.size() == maximumApplications) {
                    if (error)
                        *error = QStringLiteral("Installed application catalog exceeds the 500 application limit.");
                    return {};
                }
                result.append(app);
            }
        }
    }
    std::sort(result.begin(), result.end(), [](const auto &left, const auto &right) {
        return QString::localeAwareCompare(left.name, right.name) < 0;
    });
#else
    if (error)
        *error = QStringLiteral("Installed application discovery is unavailable on this platform.");
#endif
    return result;
}

bool DesktopApplicationCatalog::launch(const DesktopApplicationEntry &entry, QString &error)
{
    error.clear();
    const auto current = enumerate(&error);
    if (!error.isEmpty())
        return false;
    const auto known = std::find_if(current.cbegin(), current.cend(), [&entry](const auto &app) {
        return app.id == entry.id && app.launchTarget == entry.launchTarget;
    });
    if (known == current.cend()) {
        error = QStringLiteral("This application is no longer in the installed catalog. Refresh the app list.");
        return false;
    }
#ifdef Q_OS_WIN
    const HRESULT initialized = CoInitializeEx(nullptr, COINIT_APARTMENTTHREADED | COINIT_DISABLE_OLE1DDE);
    if (FAILED(initialized)) {
        error = QStringLiteral("Windows could not initialize the application launch service (error %1).").arg(quint32(initialized));
        return false;
    }
    struct ComScope {
        ~ComScope()
        {
            CoUninitialize();
        }
    } comScope;
    const auto target = known->launchTarget.toStdWString();
    SHELLEXECUTEINFOW launchInfo{};
    launchInfo.cbSize = sizeof(launchInfo);
    launchInfo.fMask = SEE_MASK_FLAG_NO_UI;
    launchInfo.lpVerb = L"open";
    launchInfo.lpFile = target.c_str();
    launchInfo.nShow = SW_SHOWNORMAL;
    if (!ShellExecuteExW(&launchInfo)) {
        error = QStringLiteral("Windows could not launch this application (error %1).").arg(GetLastError());
        return false;
    }
    return true;
#elif defined(Q_OS_LINUX) || defined(Q_OS_MACOS)
#ifdef Q_OS_LINUX
    const QString program = QStandardPaths::findExecutable("gio");
    const QStringList arguments{"launch", known->launchTarget};
#else
    const QString program = QStringLiteral("/usr/bin/open");
    const QStringList arguments{known->launchTarget};
#endif
    if (program.isEmpty()) {
        error = QStringLiteral("Application launch requires the desktop GIO launcher (gio).");
        return false;
    }
    QProcess launcher;
    launcher.setStandardOutputFile(QProcess::nullDevice());
    launcher.setStandardErrorFile(QProcess::nullDevice());
    launcher.start(program, arguments);
    if (!launcher.waitForStarted(1000)) {
        error = QStringLiteral("Could not start the desktop application launcher: %1").arg(launcher.errorString());
        return false;
    }
    if (!launcher.waitForFinished(2000)) {
        launcher.kill();
        launcher.waitForFinished(500);
        error = QStringLiteral("The desktop launcher timed out. The application may have opened; check before retrying.");
        return false;
    }
    if (launcher.exitStatus() != QProcess::NormalExit || launcher.exitCode() != 0) {
        error = QStringLiteral("The desktop rejected this application launch (exit %1).").arg(launcher.exitCode());
        return false;
    }
    return true;
#else
    error = QStringLiteral("Application launch is unavailable on this platform.");
    return false;
#endif
}

QIcon DesktopApplicationCatalog::applicationIcon(const DesktopApplicationEntry &entry)
{
#ifdef Q_OS_LINUX
    if (QFileInfo(entry.iconName).isAbsolute())
        return QIcon(entry.iconName);
    return QIcon::fromTheme(entry.iconName);
#else
    QFileIconProvider provider;
    provider.setOptions(QFileIconProvider::DontUseCustomDirectoryIcons);
    return provider.icon(QFileInfo(entry.launchTarget));
#endif
}
