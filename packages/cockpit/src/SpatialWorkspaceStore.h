#pragma once
#include "SpatialWorkspace.h"
#include <QByteArray>

class SpatialWorkspaceStore final {
public:
    explicit SpatialWorkspaceStore(const QString &path = {});
    bool load(SpatialWorkspace &workspace, QString &error);
    bool save(const SpatialWorkspace &workspace, QString &error);
    QString path() const { return m_path; }
    static constexpr qint64 maximumBytes = 2 * 1024 * 1024;
private:
    QString m_path;
    QByteArray m_revision;
    bool m_loaded = false;
};
