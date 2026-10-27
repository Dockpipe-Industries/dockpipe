#include "DockpipeChoices.h"

#include <QCoreApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QTemporaryDir>

namespace {
bool executable(const QString &path)
{
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly) || file.write("test fixture\n") < 0)
        return false;
    return file.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner | QFileDevice::ExeOwner);
}
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    QTemporaryDir temporary;
    if (!temporary.isValid())
        return 1;
    const QString project = temporary.path() + "/project";
    const QString trusted = temporary.path() + "/operator-bin";
    if (!QDir().mkpath(project + "/src/bin") || !QDir().mkpath(project + "/workflows")
        || !QDir().mkpath(trusted))
        return 2;
#ifdef Q_OS_WIN
    const QString name = "dockpipe.exe";
#else
    const QString name = "dockpipe";
#endif
    const QString repositoryBinary = project + "/src/bin/" + name;
    const QString currentDirectoryBinary = project + '/' + name;
    const QString operatorBinary = trusted + '/' + name;
    if (!executable(repositoryBinary) || !executable(currentDirectoryBinary)
        || !executable(operatorBinary) || !QDir::setCurrent(project))
        return 3;
    qunsetenv("DOCKPIPE_BIN");
    const QString unsafePath = QStringList{"", ".", "src/bin", project}.join(QDir::listSeparator());
    qputenv("PATH", unsafePath.toUtf8());
    QString error;
    const QString automatic = DockpipeChoices::operatorDockpipeBinary(&error);
    if (automatic == repositoryBinary || automatic == currentDirectoryBinary)
        return 4;
    // An installed CLI beside the test process may legitimately win over PATH.
    if (automatic.isEmpty() && error.isEmpty())
        return 5;
    qputenv("PATH", (unsafePath + QDir::listSeparator() + trusted).toUtf8());
    const QString selected = DockpipeChoices::operatorDockpipeBinary(&error);
    if (!error.isEmpty() || selected.isEmpty() || !QFileInfo(selected).isAbsolute()
        || selected == repositoryBinary || selected == currentDirectoryBinary)
        return 6;
    if (automatic.isEmpty() && selected != QFileInfo(operatorBinary).canonicalFilePath())
        return 7;
    // A host's explicit absolute override is the only way to opt into this project binary.
    qputenv("DOCKPIPE_BIN", repositoryBinary.toUtf8());
    if (DockpipeChoices::operatorDockpipeBinary(&error) != QFileInfo(repositoryBinary).canonicalFilePath()
        || !error.isEmpty())
        return 8;
    qputenv("DOCKPIPE_BIN", "src/bin/dockpipe");
    if (!DockpipeChoices::operatorDockpipeBinary(&error).isEmpty() || error.isEmpty())
        return 9;
    qputenv("DOCKPIPE_BIN", (trusted + "/missing").toUtf8());
    if (!DockpipeChoices::operatorDockpipeBinary(&error).isEmpty() || error.isEmpty())
        return 10;
    return 0;
}
