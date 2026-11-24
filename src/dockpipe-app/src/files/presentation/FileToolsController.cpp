#include "FileToolsController.h"
#include "CanvasIcons.h"
#include "DockpipeChoices.h"
#include "FileToolDialog.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceFiles.h"
#include "WorkspaceFilesController.h"

#include <QAction>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QMenu>
#include <QMessageBox>
#include <QTextDocument>
#include <QToolButton>

void FileToolsController::attach(ProjectFilesWidget *files, const QVector<ResolverMeta> &providers)
{
    auto *controller = files->findChild<FileToolsController *>();
    if (!controller)
        controller = new FileToolsController(files);
    controller->m_providers.clear();
    for (const auto &provider : providers) {
        for (const auto &action : provider.surface.actions) {
            if (action.scope == "file") {
                controller->m_providers.append(provider);
                break;
            }
        }
    }
    controller->m_button->setVisible(!controller->m_providers.isEmpty());
}

FileToolsController::FileToolsController(ProjectFilesWidget *files) : QObject(files), m_files(files)
{
    auto *actions = files->findChild<QWidget *>("fileDocumentActions");
    m_button = new QToolButton(actions);
    m_button->setObjectName("filePackageTools");
    m_button->setText(tr("Tools"));
    m_button->setAccessibleName(tr("Package file tools"));
    m_button->setToolTip(tr("Tools provided by installed packages"));
    m_button->setToolButtonStyle(Qt::ToolButtonTextBesideIcon);
    m_button->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Code));
    m_menu = new QMenu(m_button);
    m_button->setMenu(m_menu);
    m_button->setPopupMode(QToolButton::InstantPopup);
    static_cast<QHBoxLayout *>(actions->layout())->insertWidget(0, m_button);
    connect(m_menu, &QMenu::aboutToShow, this, &FileToolsController::rebuildMenu);
}

void FileToolsController::rebuildMenu()
{
    m_menu->clear();
    const QString extension = QFileInfo(m_files->currentPath()).suffix().toLower();
    bool available = false;
    for (const auto &provider : m_providers) {
        bool addedHeading = false;
        for (const auto &action : provider.surface.actions) {
            if (action.scope != "file" || !action.fileExtensions.contains(extension))
                continue;
            if (!addedHeading) {
                m_menu->addSection(provider.surface.title);
                addedHeading = true;
            }
            auto *item = m_menu->addAction(action.title);
            item->setData(provider.surface.id + ":" + action.id);
            connect(item, &QAction::triggered, this, [this, provider, action]() { run(provider, action); });
            available = true;
        }
    }
    if (!available)
        m_menu->addAction(tr("No package tools for this file type"))->setEnabled(false);
}

void FileToolsController::run(const ResolverMeta &provider, const PackageSurfaceActionMeta &action)
{
    const auto *files = WorkspaceFilesController::get(m_files);
    if (files && files->reader->context().kind == WorkspaceFilesClient::Context::Kind::Remote)
        return;
    if (m_dialog) {
        m_dialog->raise();
        m_dialog->activateWindow();
        return;
    }
    const QString path = m_files->currentPath();
    if (!WorkspaceFiles::contains(m_files->rootPath(), path) || !QFileInfo(path).isFile()
        || !action.fileExtensions.contains(QFileInfo(path).suffix().toLower()))
        return;
    if (m_files->currentDocument() && m_files->currentDocument()->isModified()) {
        QMessageBox::information(m_files, tr("Save this file first"),
            tr("Package file tools read the saved file. Save your changes, then run the tool again."));
        return;
    }
    if (action.workflow.isEmpty() || !WorkspaceFiles::contains(provider.surface.packageRoot, action.workflow)
        || !QFileInfo(action.workflow).isFile()) {
        QMessageBox::information(m_files, tr("File tool unavailable"),
            tr("The tool's workflow is no longer available inside its installed package."));
        return;
    }
    QString error;
    const QString binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty()) {
        QMessageBox::information(m_files, tr("File tool unavailable"), error);
        return;
    }
    Context context = Context::createNew();
    context.label = action.title;
    context.workdir = QFileInfo(m_files->rootPath()).canonicalFilePath();
    context.workflowFile = action.workflow;
    context.dockpipeBinary = binary;
    context.maximumOutputLineBytes = 1024 * 1024;
    context.extraDockpipeEnv = {"DOCKPIPE_FILE_PATH=" + QFileInfo(path).canonicalFilePath(),
        "DOCKPIPE_FILE_ROOT=" + context.workdir, "DOCKPIPE_FILE_ACTION_ID=" + action.id,
        "DOCKPIPE_FILE_SURFACE_ID=" + provider.surface.id};
    m_dialog = new FileToolDialog(context, action.title, provider.surface.title, path, m_files);
    m_dialog->show();
}
