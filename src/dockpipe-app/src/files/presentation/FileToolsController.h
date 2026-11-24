#pragma once

#include "WorkflowCatalogData.h"
#include "LauncherUiExport.h"
#include <QObject>
#include <QPointer>

class ProjectFilesWidget;
class QDialog;
class QMenu;
class QToolButton;

// Bridges data-only catalog contributions to a file's explicit Tools menu.
// Package code is never loaded or executed while discovering contributions.
class DOCKPIPE_LAUNCHER_UI_EXPORT FileToolsController final : public QObject {
    Q_OBJECT
public:
    static void attach(ProjectFilesWidget *files, const QVector<ResolverMeta> &providers);

private:
    explicit FileToolsController(ProjectFilesWidget *files);
    void rebuildMenu();
    void run(const ResolverMeta &provider, const PackageSurfaceActionMeta &action);
    ProjectFilesWidget *m_files;
    QVector<ResolverMeta> m_providers;
    QToolButton *m_button;
    QMenu *m_menu;
    QPointer<QDialog> m_dialog;
};
