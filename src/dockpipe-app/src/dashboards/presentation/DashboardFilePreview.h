#pragma once

#include "LauncherUiExport.h"
#include <QWidget>
#include <QJsonObject>
#include <functional>

class ProjectFilesWidget;

// A canvas editor scoped to the originating Files panel's root.
class DOCKPIPE_LAUNCHER_UI_EXPORT DashboardFilePreview final : public QWidget {
    Q_OBJECT
public:
    DashboardFilePreview(const QString &root, const QString &path, QWidget *parent = nullptr);
    DashboardFilePreview(const QString &root, const QString &path, const QJsonObject &catalog,
                         const QString &rootId, QWidget *parent = nullptr);
    QString rootPath() const;
    QString currentPath() const;
    bool hasUnsavedChanges() const;
    bool confirmLeave();
    void confirmLeaveAsync(std::function<void(bool)> completion);

signals:
    void workflowOpenRequested(const QString &path);
    void fileSaved(const QString &path);
    void dirtyChanged(bool dirty);

private:
    ProjectFilesWidget *m_files;
};
