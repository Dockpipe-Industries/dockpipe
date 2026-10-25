#include "WorkflowScriptPath.h"

#include <QDir>
#include <QFile>
#include <QTemporaryDir>
#include <cstdio>

#ifdef Q_OS_LINUX
#include <sys/xattr.h>
#endif

int main()
{
    QTemporaryDir temporary;
    if (!temporary.isValid())
        return 1;
    const QString workspace = temporary.filePath("workspace");
    if (!QDir().mkpath(workspace))
        return 2;
    const QString name = "user's script.sh";
    QFile script(QDir(workspace).filePath(name));
    if (!script.open(QIODevice::WriteOnly))
        return 3;
    script.close();

    auto result = WorkflowScriptPath::resolve(workspace, script.fileName());
    if (!result.error.isEmpty() || result.relativePath != name)
        return 4;
    result = WorkflowScriptPath::resolve(workspace, temporary.filePath("outside.sh"));
    if (!result.error.isEmpty() || result.relativePath != "../outside.sh")
        return 5;
    result = WorkflowScriptPath::resolve("/run/user/1000/doc/workspace-id/OBS",
        "/run/user/1000/doc/script-id/convert_obs.sh");
    if (result.error.isEmpty() || !result.relativePath.isEmpty())
        return 6;
    result = WorkflowScriptPath::resolve("/run/user/1000/doc/workspace-id/OBS",
        "/run/user/1000/doc/workspace-id/OBS/convert_obs.sh");
    if (!result.error.isEmpty() || result.relativePath != "convert_obs.sh")
        return 7;

#ifdef Q_OS_LINUX
    // Two independent document grants point to the same workspace and script.
    // Real xattrs exercise the portal metadata reader without a desktop session.
    const QString grant = temporary.filePath("separate-script-grant");
    QFile grantFile(grant);
    if (!grantFile.open(QIODevice::WriteOnly))
        return 8;
    grantFile.close();
    const QByteArray originalScript = QFile::encodeName("/home/developer/OBS/" + name);
    const QByteArray originalWorkspace = QFile::encodeName("/home/developer/OBS");
    if (setxattr(QFile::encodeName(grant).constData(), "user.document-portal.host-path",
            originalScript.constData(), originalScript.size() + 1, 0) != 0
        || setxattr(QFile::encodeName(workspace).constData(), "user.document-portal.host-path",
            originalWorkspace.constData(), originalWorkspace.size() + 1, 0) != 0) {
        std::perror("portal xattr fixture");
        return 9;
    }
    result = WorkflowScriptPath::resolve(workspace, grant);
    if (!result.error.isEmpty() || result.relativePath != name)
        return 10;
    if (!script.remove())
        return 11;
    result = WorkflowScriptPath::resolve(workspace, grant);
    if (result.error.isEmpty())
        return 12;
#endif
    std::puts("Workspace and document-portal script paths passed.");
    return 0;
}
