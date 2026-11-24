#include "WorkspaceFilesController.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceFileModel.h"
#include "FileEditorWorkspace.h"
#include "FileExplorerView.h"
#include "FileExplorerIcons.h"

#include <QDir>
#include <QLabel>
#include <QPushButton>
#include <QSortFilterProxyModel>

namespace {
class FilesFilter final : public QSortFilterProxyModel {
public:
    using QSortFilterProxyModel::QSortFilterProxyModel;
    QVariant data(const QModelIndex &index, int role) const override
    {
        if (role == Qt::DecorationRole)
            return FileExplorerIcons::instance().iconForEntry(index.data(Qt::DisplayRole).toString(),
                index.data(WorkspaceFileModel::DirectoryRole).toBool());
        return QSortFilterProxyModel::data(index, role);
    }
protected:
    bool filterAcceptsRow(int row, const QModelIndex &parent) const override
    {
        const auto index = sourceModel()->index(row, 0, parent);
        return index.data(WorkspaceFileModel::DirectoryRole).toBool()
            || QSortFilterProxyModel::filterAcceptsRow(row, parent);
    }
};
}

WorkspaceFilesController::WorkspaceFilesController(ProjectFilesWidget *widget)
    : QObject(widget), reader(new WorkspaceFilesClient(this)), writer(new WorkspaceFilesClient(this)),
      filter(new FilesFilter(this))
{
    setObjectName(QStringLiteral("workspaceFilesController"));
    filter->setFilterCaseSensitivity(Qt::CaseInsensitive);
    // Keep directories visible so a filename filter does not remove navigation.
}

WorkspaceFilesController *WorkspaceFilesController::get(const ProjectFilesWidget *widget)
{
    return widget->findChild<WorkspaceFilesController *>(QStringLiteral("workspaceFilesController"));
}

void WorkspaceFilesController::setContext(const WorkspaceFilesClient::Context &context)
{
    ++openGeneration;
    revisions.clear();
    reader->setContext(context);
    writer->setContext(context);

    auto *widget = qobject_cast<ProjectFilesWidget *>(parent());
    widget->setWriteAvailable(context.canWrite());
}

void WorkspaceFilesController::beginWrite()
{
    ++pendingWrites;
    auto *widget = qobject_cast<ProjectFilesWidget *>(parent());
    widget->editorWorkspace()->setProperty("fileWritePending", true);
}

void WorkspaceFilesController::endWrite()
{
    --pendingWrites;
    auto *widget = qobject_cast<ProjectFilesWidget *>(parent());
    widget->editorWorkspace()->setProperty("fileWritePending", pendingWrites > 0 || savingAll);
}

void WorkspaceFilesController::setSavingAll(bool saving)
{
    savingAll = saving;
    auto *widget = qobject_cast<ProjectFilesWidget *>(parent());
    widget->editorWorkspace()->setProperty("fileWritePending", pendingWrites > 0 || savingAll);
}

bool WorkspaceFilesController::configure(ProjectFilesWidget *widget,
    const WorkspaceFilesClient::Context &context, WorkspaceFilesClient::RemoteDispatch dispatch)
{
    auto *controller = get(widget);
    if (!widget->confirmLeave())
        return false;
    if (context.kind == WorkspaceFilesClient::Context::Kind::Local && context.localRoot.isEmpty())
        return widget->setRoot({});
    widget->closeGraph();
    widget->editorWorkspace()->clearDocuments();
    widget->editorWorkspace()->setWorkflowDocument({}, nullptr);
    widget->m_workflowPath.clear();
    controller->reader->setRemoteDispatch(dispatch);
    controller->writer->setRemoteDispatch(dispatch);
    controller->setContext(context);
    // Remote editor keys are virtual absolute paths for existing tab/view APIs.
    widget->m_root = context.kind == WorkspaceFilesClient::Context::Kind::Remote
        ? QStringLiteral("/") : QDir::cleanPath(context.localRoot);
    widget->m_model->setRemoteDispatch(std::move(dispatch));
    widget->m_model->setContext(context);
    widget->m_controls->setEnabled(true);
    widget->m_explorer->setAvailable(true);
    widget->m_graphButton->setEnabled(context.kind == WorkspaceFilesClient::Context::Kind::Local);
    widget->m_explorerDirectory = widget->m_root;
    widget->navigateExplorer(widget->m_root);
    widget->findChild<QLabel *>(QStringLiteral("filesPageSubtitle"))->setText(
        context.kind == WorkspaceFilesClient::Context::Kind::Remote ? context.rootId : context.localRoot);
    widget->syncActiveDocument();
    return true;
}
