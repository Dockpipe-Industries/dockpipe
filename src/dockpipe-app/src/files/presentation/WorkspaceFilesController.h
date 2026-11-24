#pragma once

#include "WorkspaceFilesClient.h"
#include <QHash>
#include <QObject>

class ProjectFilesWidget;
class QAction;
class QLabel;
class QPushButton;
class QSortFilterProxyModel;
class QToolButton;

// Private child state keeps the published ProjectFilesWidget object layout stable.
class WorkspaceFilesController final : public QObject {
    Q_OBJECT
public:
    explicit WorkspaceFilesController(ProjectFilesWidget *widget);
    static WorkspaceFilesController *get(const ProjectFilesWidget *widget);
    // A remote context is explicit and never interpreted as a local root path.
    static bool configure(ProjectFilesWidget *widget, const WorkspaceFilesClient::Context &context,
                          WorkspaceFilesClient::RemoteDispatch dispatch = {});
    void setContext(const WorkspaceFilesClient::Context &context);
    void beginWrite();
    void endWrite();
    void setSavingAll(bool saving);

    WorkspaceFilesClient *reader;
    WorkspaceFilesClient *writer;
    QSortFilterProxyModel *filter;
    QToolButton *newFileButton = nullptr;
    QToolButton *newFolderButton = nullptr;
    QPushButton *createFileButton = nullptr;
    QLabel *emptyHint = nullptr;
    QAction *renameFileAction = nullptr;
    QAction *saveAllFilesAction = nullptr;
    QAction *deleteFileAction = nullptr;

    QHash<QString, QString> revisions;
    quint64 openGeneration = 0;
    int pendingWrites = 0;
    bool savingAll = false;
};
