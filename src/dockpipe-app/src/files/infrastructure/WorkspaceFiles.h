#pragma once
#include <QDir>
#include <QFileInfo>

namespace WorkspaceFiles {
// The editor and its graph share one root/metadata/link containment policy.
inline bool contains(const QString &root, const QString &path)
{
    if (root.isEmpty())
        return false;
    const QString canonicalRoot = QFileInfo(root).canonicalFilePath();
    const QFileInfo info(path);
    if (canonicalRoot.isEmpty() || info.isSymLink())
        return false;
    QString canonical = info.canonicalFilePath();
    if (!info.exists()) {
        const QString parent = QFileInfo(info.absolutePath()).canonicalFilePath();
        if (parent.isEmpty())
            return false;
        canonical = QDir(parent).filePath(info.fileName());
    }
    const QString relative = QDir(canonicalRoot).relativeFilePath(canonical);
    return relative != ".." && !relative.startsWith("../") && !QDir::isAbsolutePath(relative)
        && relative != ".git" && !relative.startsWith(".git/");
}
}
