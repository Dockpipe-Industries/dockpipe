#pragma once

#include <QFileIconProvider>

// QFileSystemModel borrows its provider. Keep one alive for every explorer model.
class FileExplorerIcons final : public QFileIconProvider {
public:
    static FileExplorerIcons &instance();
    // Core metadata already establishes kind; this never inspects a local path.
    QIcon iconForEntry(const QString &name, bool directory) const;

    QIcon icon(const QFileInfo &file) const override;
    QIcon icon(IconType type) const override;
    QString type(const QFileInfo &file) const override;
};
