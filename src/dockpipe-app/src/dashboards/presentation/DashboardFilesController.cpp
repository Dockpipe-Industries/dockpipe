#include "DashboardFilesController.h"
#include "DashboardFilesWidget.h"
#include "WorkspaceFileModel.h"
#include "RemoteWorkspaceFilesDispatch.h"
#include "FileExplorerView.h"

#include <QCoreApplication>
#include <QDir>
#include <QLabel>
#include <QLineEdit>
#include <QRegularExpression>
#include <QSortFilterProxyModel>
#include <QStandardItemModel>
#include <QTreeView>

namespace {
bool validPath(const QString &path)
{
    return !path.isEmpty() && path.size() <= 4096 && !path.contains(QChar::Null) && !path.contains("://");
}

bool within(const QString &root, const QString &path)
{
#ifdef Q_OS_WIN
    constexpr auto sensitivity = Qt::CaseInsensitive;
#else
    constexpr auto sensitivity = Qt::CaseSensitive;
#endif
    const QString prefix = root.endsWith('/') ? root : root + '/';
    return !root.isEmpty() && (path.compare(root, sensitivity) == 0 || path.startsWith(prefix, sensitivity));
}

bool validTarget(const QJsonObject &target, const QString &rootId)
{
    static const QRegularExpression identifier(QStringLiteral("\\A[A-Za-z0-9][A-Za-z0-9_-]{0,63}\\z"));
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    return target.size() == 3 && target.value("node").isString() && target.value("fingerprint").isString()
        && target.value("generation").isString() && identifier.match(target.value("node").toString()).hasMatch()
        && digest.match(target.value("fingerprint").toString()).hasMatch()
        && digest.match(target.value("generation").toString()).hasMatch() && identifier.match(rootId).hasMatch();
}

QString filesystemRoot(const QString &path)
{
#ifdef Q_OS_WIN
    const QString normalized = QDir::fromNativeSeparators(path);
    static const QRegularExpression drive(QStringLiteral("\\A[A-Za-z]:/"));
    if (drive.match(normalized).hasMatch())
        return normalized.left(3);
    if (normalized.startsWith("//")) {
        const QStringList parts = normalized.mid(2).split('/', Qt::SkipEmptyParts);
        if (parts.size() >= 2)
            return "//" + parts[0] + '/' + parts[1] + '/';
    }
    return {};
#else
    Q_UNUSED(path);
    return QStringLiteral("/");
#endif
}
}

DashboardFilesController::DashboardFilesController(DashboardFilesWidget *widget)
    : QObject(widget), m_widget(widget), m_driveModel(new QStandardItemModel(this))
{
    setObjectName(QStringLiteral("dashboardFilesController"));
    widget->m_model->setRemoteDispatch(RemoteWorkspaceFilesDispatch::create(widget));
    connect(widget->m_model, &WorkspaceFileModel::directoryLoaded, this, &DashboardFilesController::listingLoaded);
    connect(widget->m_model, &WorkspaceFileModel::listingFailed, this, &DashboardFilesController::listingFailed);
}

DashboardFilesController *DashboardFilesController::get(const DashboardFilesWidget *widget)
{
    return widget->findChild<DashboardFilesController *>(QStringLiteral("dashboardFilesController"));
}

void DashboardFilesController::setRemoteDispatch(WorkspaceFilesClient::RemoteDispatch dispatch)
{
    m_widget->m_model->setRemoteDispatch(std::move(dispatch));
}

bool DashboardFilesController::setMachineContext(const QJsonObject &target, const QString &rootId,
                                                const QString &localRoot, QString *error)
{
    if (error)
        error->clear();
    const QString normalizedRoot = QDir::fromNativeSeparators(localRoot);
    const bool sameLocalRoot = m_widget->m_scope == "system" || normalizedRoot == m_localRoot;
    if (m_valid && target == m_target && rootId == m_rootId && sameLocalRoot
        && m_appliedScope == m_widget->m_scope && (!m_widget->m_directory.isEmpty() || m_drives)) {
        m_localRoot = normalizedRoot;
        return true;
    }
    // Record explicit remote intent before validating, so failure cannot expose
    // the previous local model or treat an invalid remote selection as local.
    m_remote = !target.isEmpty();
    m_target = target;
    m_rootId = rootId;
    m_localRoot = normalizedRoot;
    m_valid = m_remote ? validTarget(target, rootId)
        : (m_widget->m_scope == "system" || (validPath(m_localRoot) && QDir::isAbsolutePath(m_localRoot)));
    if (!m_remote && m_widget->m_scope == "project" && m_localRoot.isEmpty()) {
        clearView();
        m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Open a project to browse its files."));
        return true;
    }
    if (!m_valid) {
        clearView();
        const QString message = QCoreApplication::translate("DashboardFilesWidget", "This directory is missing or cannot be read.");
        m_widget->showStatus(message);
        if (error)
            *error = message;
        return false;
    }
    if (m_remote) {
        selectRoot(QStringLiteral("/"), QStringLiteral("/"));
    } else if (m_widget->m_scope == "system") {
        const QString home = QDir::fromNativeSeparators(QDir::homePath());
        selectRoot(filesystemRoot(home), home);
    } else {
        const QString root = QDir::cleanPath(m_localRoot);
        selectRoot(root, root);
    }
    return true;
}

void DashboardFilesController::setScope(const QString &scope, const QString &projectPath)
{
    if (scope != "project" && scope != "system") {
        m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Choose Project or System file scope."));
        return;
    }
    const bool sameScope = m_widget->m_scope == scope;
    m_widget->m_scope = scope;
    if (m_remote) {
        if (!m_valid) {
            clearView();
        } else if (!sameScope || m_widget->m_directory.isEmpty()) {
            selectRoot(QStringLiteral("/"), QStringLiteral("/"));
        }
        m_widget->updateControls();
        return;
    }
    const QString normalized = QDir::fromNativeSeparators(projectPath);
    if (sameScope && m_valid && !m_widget->m_directory.isEmpty()
        && (scope == "system" || QDir::cleanPath(normalized) == m_root))
        return;
    setMachineContext({}, {}, normalized, nullptr);
    if (!m_valid && scope == "project")
        m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Open a project to browse its files."));
}

void DashboardFilesController::selectRoot(const QString &root, const QString &directory)
{
    m_root = root;
    m_appliedScope = m_widget->m_scope;
    m_drives = false;
    m_widget->m_projectRoot = m_remote || m_widget->m_scope == "project" ? root : QString();
    m_widget->m_directory.clear();
    auto *proxy = static_cast<QSortFilterProxyModel *>(m_widget->m_tree->model());
    proxy->setSourceModel(m_widget->m_model);
    WorkspaceFilesClient::Context context;
    context.generation = ++m_epoch;
    if (m_remote) {
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
        context.nodeId = m_target.value("node").toString();
        context.fingerprint = m_target.value("fingerprint").toString();
        context.workerGeneration = m_target.value("generation").toString();
        context.rootId = m_rootId;
    } else {
        context.localRoot = root;
    }
    m_pendingDirectory = QDir::cleanPath(directory);
    m_pendingRelative = QDir(root).relativeFilePath(m_pendingDirectory);
    m_widget->m_explorer->setRootIndex({});
    m_widget->m_explorer->setAvailable(false);
    m_widget->m_model->setContext(context);
    if (m_pendingRelative != ".")
        m_widget->m_model->setDirectory(m_pendingRelative);
    m_widget->showStatus({});
    m_widget->updateControls();
    m_widget->m_location->setText(QDir::toNativeSeparators(m_pendingDirectory));
}

bool DashboardFilesController::navigate(const QString &path)
{
    if (!m_valid || !validPath(path) || (m_widget->m_directory.isEmpty() && !QDir::isAbsolutePath(path))) {
        m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Choose an existing directory."));
        return false;
    }
    const QString candidate = QDir::cleanPath(QDir::fromNativeSeparators(QDir::isAbsolutePath(path)
        ? path : QDir(m_widget->m_directory).absoluteFilePath(path)));
    if (m_remote || m_widget->m_scope == "project") {
        if (!within(m_root, candidate)) {
            m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Project scope stays inside the selected project, including symbolic links."));
            return false;
        }
    } else {
        const QString selected = filesystemRoot(candidate);
        if (selected.isEmpty())
            return false;
        if (m_drives || selected != m_root) {
            selectRoot(selected, candidate);
            return true;
        }
    }
    requestDirectory(candidate);
    return true;
}

void DashboardFilesController::requestDirectory(const QString &directory)
{
    m_pendingDirectory = directory;
    m_pendingRelative = QDir(m_root).relativeFilePath(directory);
    m_widget->m_explorer->setAvailable(false);
    m_widget->m_explorer->setRootIndex({});
    m_widget->m_tree->collapseAll();
    const bool unchanged = m_widget->m_model->relativePath({}) == m_pendingRelative;
    m_widget->m_model->setDirectory(m_pendingRelative);
    if (unchanged)
        m_widget->m_model->refresh();
    m_widget->showStatus({});
    m_widget->m_location->setText(QDir::toNativeSeparators(directory));
}

void DashboardFilesController::listingLoaded(const QString &relativePath)
{
    if (!m_valid || m_drives || m_pendingDirectory.isEmpty() || relativePath != m_pendingRelative)
        return;
    m_widget->m_directory = m_pendingDirectory;
    m_pendingDirectory.clear();
    m_widget->m_explorer->setAvailable(true);
    m_widget->showStatus({});
    m_widget->updateControls();
}

void DashboardFilesController::listingFailed(const QString &relativePath, const QString &error)
{
    if (!m_pendingDirectory.isEmpty() && relativePath == m_pendingRelative) {
        m_pendingDirectory.clear();
        m_widget->m_directory.clear();
        m_widget->m_explorer->setAvailable(false);
        m_widget->updateControls();
    }
    m_widget->showStatus(error);
}

void DashboardFilesController::clearView()
{
    m_root.clear();
    m_drives = false;
    m_pendingDirectory.clear();
    m_pendingRelative.clear();
    m_widget->m_projectRoot.clear();
    m_widget->m_directory.clear();
    WorkspaceFilesClient::Context context;
    context.generation = ++m_epoch;
    if (m_remote)
        context.kind = WorkspaceFilesClient::Context::Kind::Remote;
    m_widget->m_model->setContext(context);
    m_widget->m_explorer->setRootIndex({});
    m_widget->m_explorer->setAvailable(false);
    m_widget->updateControls();
}

void DashboardFilesController::showFilesystem()
{
    if (m_remote || m_widget->m_scope != "system" || !m_valid)
        return;
#ifdef Q_OS_WIN
    clearView();
    m_valid = true;
    m_drives = true;
    m_driveModel->clear();
    // Drive inventory selects local root grants; entry metadata and traversal
    // still come exclusively from the core Files service after selection.
    for (const auto &drive : QDir::drives()) {
        const QString path = QDir::fromNativeSeparators(drive.absoluteFilePath());
        auto *item = new QStandardItem(QDir::toNativeSeparators(path));
        item->setData(path, WorkspaceFileModel::RelativePathRole);
        item->setData(true, WorkspaceFileModel::DirectoryRole);
        m_driveModel->appendRow(item);
    }
    static_cast<QSortFilterProxyModel *>(m_widget->m_tree->model())->setSourceModel(m_driveModel);
    m_widget->m_explorer->setAvailable(true);
    m_widget->showStatus(QCoreApplication::translate("DashboardFilesWidget", "Choose a drive."));
    m_widget->updateControls();
#else
    navigate(QStringLiteral("/"));
#endif
}
