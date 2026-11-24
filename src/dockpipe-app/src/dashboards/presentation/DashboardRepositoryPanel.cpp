#include "DashboardRepositoryPanel.h"

#include "DashboardWorkflowView.h"
#include "ProjectFilesWidget.h"
#include "FileEditorWorkspace.h"
#include "ProjectGitWidget.h"
#include "WorkflowEditorWidget.h"
#include "DashboardWidgetFactory.h"
#include "WorkspaceFilesController.h"
#include "RemoteWorkspaceFilesDispatch.h"
#include "RemoteGitContext.h"
#include <QDir>
#include <QFileDialog>
#include <QFileInfo>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QMessageBox>
#include <QLineEdit>
#include <QPushButton>
#include <QVBoxLayout>

DashboardRepositoryPanel::DashboardRepositoryPanel(const QString &kind, const QString &repository, QWidget *parent)
    : DashboardRepositoryPanel(kind, repository, {}, {}, parent)
{
}

DashboardRepositoryPanel::DashboardRepositoryPanel(const QString &kind, const QString &repository,
    const QJsonObject &catalog, const QString &rootId, QWidget *parent)
    : QWidget(parent), m_kind(kind), m_location(new QLabel(this)), m_error(new QLabel(this))
{
    setObjectName("dashboardRepositoryPanel");
    setProperty("dashboardFullKind", kind);
    setMinimumSize(640, 520);
    auto *column = new QVBoxLayout(this);
    column->setContentsMargins(0, 0, 0, 0);
    column->setSpacing(8);
    auto *toolbar = new QHBoxLayout;
    auto *choose = new QPushButton(tr("Repository…"), this);
    choose->setObjectName("dashboardRepositoryChoose");
    toolbar->addWidget(choose);
    m_location->setTextFormat(Qt::PlainText);
    m_location->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    m_location->setObjectName("dashboardRepositoryLocation");
    toolbar->addWidget(m_location, 1);
    auto *duplicate = new QPushButton(tr("Duplicate view"), this);
    duplicate->setObjectName("dashboardDuplicateView");
    duplicate->setToolTip(tr("Open another independent view, then choose its repository."));
    toolbar->addWidget(duplicate);
    column->addLayout(toolbar);
    m_error->setWordWrap(true);
    m_error->setTextFormat(Qt::PlainText);
    m_error->setObjectName("dashboardRepositoryError");
    m_error->hide();
    column->addWidget(m_error);
    if (kind == "files-full") {
        m_files = new ProjectFilesWidget(this);
        column->addWidget(m_files, 1);
        if (auto *editors = m_files->findChild<FileEditorWorkspace *>())
            connect(editors, &FileEditorWorkspace::currentChanged, this, &DashboardRepositoryPanel::contextChanged);
        connect(m_files, &ProjectFilesWidget::fileSaved, this, &DashboardRepositoryPanel::contextChanged);
        connect(m_files, &ProjectFilesWidget::workflowOpenRequested, this, [this](const QString &path) {
            m_files->confirmLeaveAsync([this, path](bool accepted) {
                if (accepted)
                    emit workflowOpenRequested(m_repository, path);
            });
        });
    } else if (kind == "git-full") {
        m_git = new ProjectGitWidget(this);
        column->addWidget(m_git, 1);
    } else {
        m_workflows = new DashboardWorkflowView(this);
        column->addWidget(m_workflows, 1);
        if (auto *editor = m_workflows->findChild<WorkflowEditorWidget *>()) {
            connect(editor, &WorkflowEditorWidget::documentChanged, this, &DashboardRepositoryPanel::contextChanged);
            connect(editor, &WorkflowEditorWidget::workflowSaved, this, &DashboardRepositoryPanel::contextChanged);
        }
        connect(m_workflows, &DashboardWorkflowView::workflowRunRequested, this, &DashboardRepositoryPanel::workflowRunRequested);
    }
    connect(choose, &QPushButton::clicked, this, [this]() {
        if (!property("dashboardMachineCatalog").toJsonObject().isEmpty() && m_kind != "workflows-full")
            return;
        const QString directory = QFileDialog::getExistingDirectory(this, tr("Choose this view’s repository"), m_repository);
        if (!directory.isEmpty())
            setRepositoryAsync(directory);
    });
    connect(duplicate, &QPushButton::clicked, this, &DashboardRepositoryPanel::duplicateRequested);
    m_location->setText(tr("Choose a repository for this view"));
    setProperty("dashboardLocalRepository", repository);
    if (!catalog.isEmpty() && m_kind != "workflows-full") {
        setMachineContext(catalog, rootId);
    } else if (!repository.isEmpty() && m_files) {
        setRepositoryAsync(repository);
    } else if (!repository.isEmpty() && !setRepository(repository)) {
        m_repository = QDir::cleanPath(QFileInfo(repository).absoluteFilePath());
        m_location->setText(QDir::toNativeSeparators(m_repository));
        m_location->setToolTip(m_repository);
    }
    if (!catalog.isEmpty() && m_kind == "workflows-full")
        setMachineContext(catalog, rootId);
}

bool DashboardRepositoryPanel::canChangeMachineContext(QString *error) const
{
    return DashboardWidgetFactory::canChangeWidgetMachineContext(const_cast<DashboardRepositoryPanel *>(this), error);
}

bool DashboardRepositoryPanel::setMachineContext(const QJsonObject &catalog, const QString &rootId, QString *error)
{
    if (property("dashboardMachineConfigured").toBool() && property("dashboardMachineCatalog").toJsonObject() == catalog
        && property("dashboardMachineRootId").toString() == rootId)
        return true;
    if (!canChangeMachineContext(error))
        return false;
    auto target = catalog.value("target").toObject();
    if (!catalog.isEmpty() && target.isEmpty())
        target.insert("node", "unavailable");
    const QString localRoot = property("dashboardLocalRepository").toString();
    QJsonObject grant;
    for (const auto &value : catalog.value("roots").toArray()) {
        const auto candidate = value.toObject();
        if (candidate.value("id").toString() == rootId) {
            grant = candidate;
            break;
        }
    }
    bool accepted = true;
    if (m_files) {
        WorkspaceFilesClient::Context context;
        context.generation = WorkspaceFilesController::get(m_files)->reader->context().generation + 1;
        if (catalog.isEmpty()) {
            context.localRoot = localRoot;
        } else {
            context.kind = WorkspaceFilesClient::Context::Kind::Remote;
            context.nodeId = target.value("node").toString();
            context.fingerprint = target.value("fingerprint").toString();
            context.workerGeneration = target.value("generation").toString();
            context.rootId = rootId;
            context.writeGranted = grant.value("files_write").toBool();
        }
        accepted = WorkspaceFilesController::configure(m_files, context, RemoteWorkspaceFilesDispatch::create(m_files));
    } else if (m_git) {
        accepted = catalog.isEmpty() ? RemoteGitContext::configureLocal(m_git, localRoot, error)
            : RemoteGitContext::configure(m_git, target, rootId, grant, error);
    }
    if (!accepted)
        return false;
    setProperty("dashboardMachineConfigured", true);
    setProperty("dashboardMachineCatalog", catalog);
    setProperty("dashboardMachineTarget", target);
    setProperty("dashboardMachineRootId", rootId);
    if (!property("dashboardMachineContext").isValid()) {
        QJsonObject machine{{"mode", catalog.isEmpty() ? "local" : "remote"}};
        if (!catalog.isEmpty()) {
            machine.insert("target", target);
            machine.insert("rootId", rootId);
        }
        setProperty("dashboardMachineContext", machine);
    }
    if (m_kind != "workflows-full") {
        m_repository = catalog.isEmpty() ? localRoot : QStringLiteral("/");
        m_location->setText(catalog.isEmpty() ? QDir::toNativeSeparators(localRoot) : rootId);
        m_location->setToolTip(m_location->text());
        findChild<QPushButton *>("dashboardRepositoryChoose")->setEnabled(catalog.isEmpty());
    }
    m_error->hide();
    emit contextChanged();
    return true;
}

bool DashboardRepositoryPanel::confirmCommitDraft()
{
    if (!m_git)
        return true;
    auto *message = m_git->findChild<QLineEdit *>("gitCommitMessage");
    if (!message || message->text().trimmed().isEmpty())
        return true;
    QMessageBox question(QMessageBox::Question, tr("Unsaved commit message"),
        tr("Discard this view’s commit message? Repository files will be kept."),
        QMessageBox::Discard | QMessageBox::Cancel, this);
    question.setDefaultButton(QMessageBox::Cancel);
    if (question.exec() != QMessageBox::Discard)
        return false;
    return true;
}

bool DashboardRepositoryPanel::setRepository(const QString &repository)
{
    if (!property("dashboardMachineCatalog").toJsonObject().isEmpty() && m_kind != "workflows-full")
        return false;
    const QFileInfo directory(repository);
    const QString root = directory.canonicalFilePath();
    if (root.isEmpty() || !directory.isDir()) {
        m_error->setText(tr("This repository folder is unavailable. Choose another folder."));
        m_error->show();
        return false;
    }
    if (root == m_repository)
        return true;
    bool accepted = false;
    if (m_files) {
        accepted = m_files->setRoot(root);
    } else if (m_workflows) {
        accepted = m_workflows->setWorkdir(root);
    } else if (!m_git->busy() && confirmCommitDraft()) {
        m_git->setWorkdir(root);
        accepted = m_git->workdir() == root;
        if (accepted)
            m_git->refresh();
    }
    if (!accepted)
        return false;
    m_repository = root;
    setProperty("dashboardLocalRepository", root);
    m_location->setText(QDir::toNativeSeparators(root));
    m_location->setToolTip(root);
    m_error->hide();
    emit contextChanged();
    return true;
}

QJsonObject DashboardRepositoryPanel::context() const
{
    QJsonObject result{{"root", m_repository}};
    if (property("dashboardMachineContext").isValid())
        result.insert("machine", property("dashboardMachineContext").toJsonObject());
    if (!property("dashboardMachineCatalog").toJsonObject().isEmpty())
        result.insert("localRoot", property("dashboardLocalRepository").toString());
    if (m_files && !m_files->currentPath().isEmpty())
        result["path"] = m_files->currentPath();
    if (m_workflows) {
        if (auto *editor = m_workflows->findChild<WorkflowEditorWidget *>())
            result["path"] = editor->workflowPath();
    }
    return result;
}

bool DashboardRepositoryPanel::restoreContext(const QJsonObject &context)
{
    const QString path = context.value("path").toString();
    if (m_files) {
        if (!property("dashboardMachineCatalog").toJsonObject().isEmpty()) {
            if (!path.isEmpty())
                m_files->openFileAsync(path);
            return path.isEmpty();
        }
        setRepositoryAsync(context.value("root").toString(), [this, path](bool accepted) {
            if (accepted && !path.isEmpty())
                m_files->openFileAsync(path);
        });
        return false; // A pending open is not a completed restore.
    }
    if (m_git && !property("dashboardMachineCatalog").toJsonObject().isEmpty())
        return true;
    if (m_workflows) {
        setRepositoryAsync(context.value("root").toString(), [this, path](bool accepted) {
            if (accepted && !path.isEmpty())
                m_workflows->openWorkflowAsync(path);
        });
        return false;
    }
    if (!setRepository(context.value("root").toString()))
        return false;
    return path.isEmpty() || !m_workflows || m_workflows->openWorkflow(path);
}

void DashboardRepositoryPanel::setRepositoryAsync(const QString &repository, std::function<void(bool)> completion)
{
    if (!property("dashboardMachineCatalog").toJsonObject().isEmpty() && m_kind != "workflows-full") {
        if (completion)
            completion(false);
        return;
    }
    const QFileInfo directory(repository);
    const QString root = directory.canonicalFilePath();
    if (root.isEmpty() || !directory.isDir()) {
        m_error->setText(tr("This repository folder is unavailable. Choose another folder."));
        m_error->show();
        if (completion)
            completion(false);
        return;
    }
    auto applied = [this, root, completion = std::move(completion)](bool accepted) {
        if (accepted) {
            m_repository = root;
            setProperty("dashboardLocalRepository", root);
            m_location->setText(QDir::toNativeSeparators(root));
            m_location->setToolTip(root);
            m_error->hide();
            emit contextChanged();
        }
        if (completion)
            completion(accepted);
    };
    if (m_files) {
        m_files->setRootAsync(root, std::move(applied));
    } else if (m_workflows) {
        m_workflows->setWorkdirAsync(root, std::move(applied));
    } else if (!m_git->busy() && confirmCommitDraft()) {
        m_git->setWorkdirAsync(root, [this, applied = std::move(applied)](bool accepted) {
            if (accepted)
                m_git->refresh();
            applied(accepted);
        });
    } else {
        applied(false);
    }
}

void DashboardRepositoryPanel::confirmLeaveAsync(std::function<void(bool)> completion)
{
    if (m_files)
        m_files->confirmLeaveAsync(std::move(completion));
    else if (m_workflows)
        m_workflows->confirmLeaveAsync(std::move(completion));
    else if (!m_git->busy() && confirmCommitDraft())
        m_git->confirmLeaveAsync(std::move(completion));
    else
        completion(false);
}

bool DashboardRepositoryPanel::confirmLeave()
{
    if (m_files)
        return m_files->confirmLeave();
    if (m_workflows)
        return m_workflows->confirmLeave();
    if (m_git->busy()) {
        m_error->setText(tr("Wait for this view’s Git action to finish before closing it."));
        m_error->show();
        return false;
    }
    return m_git->confirmLeave() && confirmCommitDraft();
}

bool DashboardRepositoryPanel::openWorkflow(const QString &path)
{
    return m_workflows && m_workflows->openWorkflow(path);
}
