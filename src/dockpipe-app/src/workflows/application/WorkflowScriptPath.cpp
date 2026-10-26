#include "WorkflowScriptPath.h"

#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QObject>
#include <QRegularExpression>

#ifdef Q_OS_LINUX
#include <limits.h>
#include <sys/xattr.h>
#endif

namespace {
QString portalHostPath(const QString &path)
{
#ifdef Q_OS_LINUX
    // The document portal exposes the original path without requiring host
    // execution or extra filesystem permissions. Linux paths are PATH_MAX bound.
    char bytes[PATH_MAX + 1];
    const ssize_t size = getxattr(QFile::encodeName(path).constData(),
        "user.document-portal.host-path", bytes, sizeof(bytes));
    if (size > 0 && size <= PATH_MAX) {
        QByteArray value(bytes, size);
        if (value.endsWith('\0'))
            value.chop(1);
        if (!value.contains('\0')) {
            const QString hostPath = QFile::decodeName(value);
            if (QDir::isAbsolutePath(hostPath))
                return QDir::cleanPath(hostPath);
        }
    }
#else
    Q_UNUSED(path);
#endif
    return {};
}

bool isPortalPath(const QString &path)
{
    static const QRegularExpression portal(QStringLiteral("^/run/(user/[0-9]+/doc|flatpak/doc)(/|$)"));
    return portal.match(QDir::cleanPath(path)).hasMatch();
}

bool isWithinWorkspace(const QString &relative)
{
    return !QDir::isAbsolutePath(relative) && relative != ".." && !relative.startsWith("../");
}
}

WorkflowScriptPath WorkflowScriptPath::resolve(const QString &workspace, const QString &script)
{
    const QString relative = QDir(workspace).relativeFilePath(script);
    // A picker browsing the workspace itself already gives a portable path,
    // even when the workspace is a directory granted through the portal.
    if (isWithinWorkspace(relative))
        return {relative, {}};

    const QString workspaceHost = portalHostPath(workspace);
    const QString scriptHost = portalHostPath(script);
    if (!workspaceHost.isEmpty() || !scriptHost.isEmpty()) {
        const QString hostRelative = QDir(workspaceHost.isEmpty() ? workspace : workspaceHost)
            .relativeFilePath(scriptHost.isEmpty() ? script : scriptHost);
        const QFileInfo accessible(QDir(workspace).filePath(hostRelative));
        if (isWithinWorkspace(hostRelative) && accessible.isFile() && accessible.isReadable())
            return {hostRelative, {}};
    }

    if (isPortalPath(workspace) || isPortalPath(script) || !workspaceHost.isEmpty() || !scriptHost.isEmpty()) {
        return {{}, QObject::tr("Select a script inside the workspace folder. This file was shared separately by "
                               "Flatpak, so its temporary document path cannot be saved in a portable workflow.")};
    }
    return {relative, {}};
}
