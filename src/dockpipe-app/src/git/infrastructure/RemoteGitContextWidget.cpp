#include "RemoteGitContext.h"

#include "ProjectFilesWidget.h"
#include "ProjectGitClient.h"
#include "ProjectGitWidget.h"
#include "RemoteWorkspaceFilesDispatch.h"
#include "WorkspaceFilesController.h"

#include <QDir>
#include <QLabel>

namespace {
bool canChange(ProjectGitWidget *widget, QString *error)
{
    if (error)
        error->clear();
    if (!widget)
        return false;
    for (const auto *client : widget->findChildren<ProjectGitClient *>()) {
        if (client->busy()) {
            if (error)
                *error = QObject::tr("Wait for all Git operations before changing machines or roots.");
            return false;
        }
    }
    for (auto *editor : widget->findChildren<ProjectFilesWidget *>()) {
        if (!editor->confirmLeave()) {
            if (error)
                *error = QObject::tr("Save or discard the conflict draft before changing machines or roots.");
            return false;
        }
    }
    return true;
}

void bindClients(ProjectGitWidget *widget, bool remote, const QJsonObject &target, const QString &root,
                 const QJsonObject &grant = {})
{
    RemoteGitContext::bind(widget, remote, target, root, grant);
    for (auto *client : widget->findChildren<ProjectGitClient *>())
        RemoteGitContext::bind(client, remote, target, root, grant);
}
}

bool RemoteGitContext::configure(ProjectGitWidget *widget, const QJsonObject &target,
                                 const QString &root, const QJsonObject &grant, QString *error)
{
    if (!canChange(widget, error))
        return false;
    bindClients(widget, true, target, root, grant);
    for (auto *editor : widget->findChildren<ProjectFilesWidget *>()) {
        WorkspaceFilesClient::Context context;
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.nodeId = target.value("node").toString();
        context.fingerprint = target.value("fingerprint").toString();
        context.workerGeneration = target.value("generation").toString();
        context.rootId = root;
        context.generation = WorkspaceFilesController::get(editor)->reader->context().generation + 1;
        context.writeGranted = grant.value("files_write").toBool();
        if (!WorkspaceFilesController::configure(editor, context, RemoteWorkspaceFilesDispatch::create(editor)))
            return false;
    }
    // Both remote targets use virtual '/'. Clear prior review tokens and queues
    // even when the virtual path is unchanged across an explicit selection.
    widget->setWorkdir(QString());
    widget->setWorkdir(QStringLiteral("/"));
    if (auto *heading = widget->findChild<QLabel *>(QStringLiteral("gitBranch")))
        heading->setText(root);
    return widget->workdir() == QStringLiteral("/");
}

bool RemoteGitContext::configureLocal(ProjectGitWidget *widget, const QString &workdir, QString *error)
{
    if (!canChange(widget, error))
        return false;
    if (!workdir.isEmpty() && !QDir::isAbsolutePath(workdir)) {
        if (error)
            *error = QObject::tr("Select an absolute local project folder.");
        return false;
    }
    bindClients(widget, false, {}, {});
    for (auto *editor : widget->findChildren<ProjectFilesWidget *>()) {
        WorkspaceFilesClient::Context context;
        context.localRoot = workdir;
        context.generation = WorkspaceFilesController::get(editor)->reader->context().generation + 1;
        if (!WorkspaceFilesController::configure(editor, context))
            return false;
    }
    widget->setWorkdir(QString());
    widget->setWorkdir(workdir);
    return widget->workdir() == workdir;
}
