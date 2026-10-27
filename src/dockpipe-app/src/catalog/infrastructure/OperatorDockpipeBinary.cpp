#include "DockpipeChoices.h"

#include <QCoreApplication>
#include <QDir>
#include <QFileInfo>
#include <QProcessEnvironment>

namespace {
QString absoluteExecutable(const QString &path)
{
    const QFileInfo candidate(path);
    if (!candidate.isAbsolute() || !candidate.isFile() || !candidate.isExecutable())
        return {};
    return candidate.canonicalFilePath();
}
}

QString DockpipeChoices::operatorDockpipeBinary(QString *error)
{
    if (error)
        error->clear();
    const auto environment = QProcessEnvironment::systemEnvironment();
    const QString configured = environment.value("DOCKPIPE_BIN").trimmed();
    if (!configured.isEmpty()) {
        const QString executable = absoluteExecutable(configured);
        if (executable.isEmpty() && error)
            *error = QCoreApplication::translate("DockpipeChoices",
                "DOCKPIPE_BIN must name an absolute, existing Dockpipe executable for security-key operations.");
        return executable;
    }
#ifdef Q_OS_WIN
    const QString installedName = QStringLiteral("dockpipe.exe");
    const QString pathName = QStringLiteral("dockpipe.exe");
#elif defined(Q_OS_MACOS)
    const QString installedName = QStringLiteral("dockpipe-cli");
    const QString pathName = QStringLiteral("dockpipe");
#else
    const QString installedName = QStringLiteral("dockpipe");
    const QString pathName = QStringLiteral("dockpipe");
#endif
    const QString installed = absoluteExecutable(
        QDir(QCoreApplication::applicationDirPath()).filePath(installedName));
    if (!installed.isEmpty())
        return installed;

    const QString currentDirectory = QFileInfo(QDir::currentPath()).canonicalFilePath();
    const auto entries = environment.value("PATH").split(QDir::listSeparator(), Qt::KeepEmptyParts);
    for (const QString &entry : entries) {
        const QFileInfo directory(entry);
        if (entry.isEmpty() || !directory.isAbsolute() || !directory.isDir()
            || directory.canonicalFilePath() == currentDirectory)
            continue;
        const QString executable = absoluteExecutable(QDir(entry).filePath(pathName));
        if (!executable.isEmpty())
            return executable;
    }
    if (error)
        *error = QCoreApplication::translate("DockpipeChoices",
            "Install the Dockpipe CLI beside this application or in an absolute PATH directory, "
            "or set DOCKPIPE_BIN to an absolute trusted executable before launching Dockpipe. "
            "Project binaries cannot handle security-key operations automatically.");
    return {};
}
