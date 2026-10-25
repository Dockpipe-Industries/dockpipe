#pragma once

#include <QFileIconProvider>

// QFileSystemModel borrows its provider. Keep one alive for every explorer model.
class FileExplorerIcons final : public QFileIconProvider {
public:
    static FileExplorerIcons &instance();

    QIcon icon(const QFileInfo &file) const override;
    QIcon icon(IconType type) const override;
    QString type(const QFileInfo &file) const override;
};
