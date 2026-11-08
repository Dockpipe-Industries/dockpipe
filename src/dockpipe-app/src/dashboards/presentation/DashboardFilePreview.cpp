#include "DashboardFilePreview.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceFilesController.h"
#include "RemoteWorkspaceFilesDispatch.h"

#include <QDir>
#include <QVBoxLayout>

DashboardFilePreview::DashboardFilePreview(const QString &root, const QString &path, QWidget *parent)
    : DashboardFilePreview(root, path, {}, {}, parent)
{
}

DashboardFilePreview::DashboardFilePreview(const QString &root, const QString &path, const QJsonObject &catalog,
                                         const QString &rootId, QWidget *parent)
    : QWidget(parent), m_files(new ProjectFilesWidget(this))
{
    setObjectName("dashboardFilePreview");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->addWidget(m_files);
    m_files->setFocusedPreview(true);
    connect(m_files, &ProjectFilesWidget::workflowOpenRequested, this, &DashboardFilePreview::workflowOpenRequested);
    connect(m_files, &ProjectFilesWidget::fileSaved, this, &DashboardFilePreview::fileSaved);
    connect(m_files, &ProjectFilesWidget::dirtyChanged, this, &DashboardFilePreview::dirtyChanged);
    // The existing Files owner enforces containment, UTF-8/size bounds, atomic
    // saves and concurrent-edit checks for the same editor used by full Files.
    QJsonObject machine{{"mode", catalog.isEmpty() ? "local" : "remote"}};
    if (!catalog.isEmpty()) {
        auto target = catalog.value("target").toObject();
        if (target.isEmpty())
            target.insert("node", "unavailable");
        machine.insert("target", target);
        machine.insert("rootId", rootId);
        WorkspaceFilesClient::Context context;
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.nodeId = target.value("node").toString();
        context.fingerprint = target.value("fingerprint").toString();
        context.workerGeneration = target.value("generation").toString();
        context.rootId = rootId;
        context.generation = 1;
        if (WorkspaceFilesController::configure(m_files, context, RemoteWorkspaceFilesDispatch::create(m_files)))
            m_files->openFileAsync(QDir(QStringLiteral("/")).absoluteFilePath(path));
    } else {
        m_files->setRootAsync(root, [this, path](bool accepted) {
            if (accepted)
                m_files->openFileAsync(QDir(m_files->rootPath()).absoluteFilePath(path));
        });
    }
    setProperty("dashboardMachineContext", machine);
}

QString DashboardFilePreview::rootPath() const
{
    return m_files->rootPath();
}

QString DashboardFilePreview::currentPath() const
{
    return m_files->currentPath();
}

bool DashboardFilePreview::hasUnsavedChanges() const
{
    return m_files->hasUnsavedChanges();
}

bool DashboardFilePreview::confirmLeave()
{
    return m_files->confirmLeave();
}

void DashboardFilePreview::confirmLeaveAsync(std::function<void(bool)> completion)
{
    m_files->confirmLeaveAsync(std::move(completion));
}
