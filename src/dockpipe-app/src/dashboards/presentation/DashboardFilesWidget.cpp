#include "DashboardFilesWidget.h"
#include "FileExplorerView.h"
#include "FileExplorerIcons.h"
#include "CanvasIcons.h"
#include "WorkspaceFileModel.h"
#include "DashboardFilesController.h"

#include <QDir>
#include <QHBoxLayout>
#include <QHeaderView>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSortFilterProxyModel>
#include <QTreeView>
#include <QVBoxLayout>

// The core has already established entry kinds and scope. Presentation only
// sorts directory rows and renders icons from names, without inspecting paths.
class DashboardFilesFilter final : public QSortFilterProxyModel {
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
    bool lessThan(const QModelIndex &left, const QModelIndex &right) const override
    {
        const bool leftDirectory = left.data(WorkspaceFileModel::DirectoryRole).toBool();
        const bool rightDirectory = right.data(WorkspaceFileModel::DirectoryRole).toBool();
        if (leftDirectory != rightDirectory)
            return leftDirectory;
        return QSortFilterProxyModel::lessThan(left, right);
    }
};

DashboardFilesWidget::DashboardFilesWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("dashboardFilesWidget");
    setAttribute(Qt::WA_StyledBackground);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(8, 8, 8, 8);
    layout->setSpacing(8);
    auto *navigation = new QHBoxLayout;
    m_up = new QPushButton(tr("Up"));
    m_up->setObjectName("dashboardFilesUp");
    m_home = new QPushButton(tr("Home"));
    m_home->setObjectName("dashboardFilesHome");
    m_filesystem = new QPushButton(tr("Filesystem"));
    m_filesystem->setObjectName("dashboardFilesFilesystem");
    navigation->addWidget(m_up);
    navigation->addWidget(m_home);
    navigation->addWidget(m_filesystem);

    m_location = new QLineEdit;
    m_location->setObjectName("dashboardFilesLocation");
    m_location->setAccessibleName(tr("Directory"));
    m_location->setMaxLength(4096);
    navigation->addWidget(m_location, 1);
    layout->addLayout(navigation);
    const QList<QPushButton *> navigationButtons{m_up, m_home, m_filesystem};
    const QList<CanvasIcons::Symbol> navigationIcons{
        CanvasIcons::Symbol::Up, CanvasIcons::Symbol::Folder, CanvasIcons::Symbol::Monitor};
    for (int index = 0; index < navigationButtons.size(); ++index) {
        auto *button = navigationButtons.at(index);
        button->setAccessibleName(button->text());
        button->setToolTip(button->text());
        button->setText({});
        button->setIcon(CanvasIcons::icon(navigationIcons.at(index)));
        button->setProperty("fileNavigation", true);
        button->setFixedSize(32, 32);
    }
    m_up->setToolTip(tr("Up one folder"));
    m_filesystem->setToolTip(tr("Browse the filesystem and drives"));
    m_model = new WorkspaceFileModel(this);
    m_filter = new DashboardFilesFilter(this);
    m_filter->setSourceModel(m_model);
    m_explorer = new FileExplorerView;
    m_explorer->setModel(m_filter);
    m_tree = m_explorer->treeView();
    m_tree->setObjectName("dashboardFiles");
    m_tree->setSortingEnabled(true);
    m_tree->sortByColumn(0, Qt::AscendingOrder);
    layout->addWidget(m_explorer, 1);
    m_status = new QLabel;
    m_status->setObjectName("dashboardFilesStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    new DashboardFilesController(this);
    connect(m_up, &QPushButton::clicked, this, [this] { navigateTo(".."); });
    connect(m_home, &QPushButton::clicked, this, [this] {
        auto *controller = DashboardFilesController::get(this);
        navigateTo(controller->remote() ? QStringLiteral("/")
            : (m_scope == "system" ? QDir::homePath() : m_projectRoot));
    });
    connect(m_filesystem, &QPushButton::clicked, this, &DashboardFilesWidget::showFilesystem);
    connect(m_location, &QLineEdit::returnPressed, this, [this] { navigateTo(m_location->text()); });
    connect(m_explorer, &FileExplorerView::activated, this, &DashboardFilesWidget::activate);
    clearProject();
}

void DashboardFilesWidget::setContext(const QString &scope, const QString &projectPath)
{
    DashboardFilesController::get(this)->setScope(scope, projectPath);
}

bool DashboardFilesWidget::setMachineContext(const QJsonObject &target, const QString &rootId,
                                            const QString &localRoot, QString *error)
{
    return DashboardFilesController::get(this)->setMachineContext(target, rootId, localRoot, error);
}

QJsonObject DashboardFilesWidget::machineTarget() const
{
    return DashboardFilesController::get(this)->machineTarget();
}

QString DashboardFilesWidget::remoteRootId() const
{
    return DashboardFilesController::get(this)->remoteRootId();
}

bool DashboardFilesWidget::navigateTo(const QString &path)
{
    return DashboardFilesController::get(this)->navigate(path);
}

void DashboardFilesWidget::activate(const QModelIndex &index)
{
    if (!index.isValid() || !index.flags().testFlag(Qt::ItemIsEnabled))
        return;
    const auto *controller = DashboardFilesController::get(this);
    if (!controller->available())
        return;
    const QString relative = index.data(WorkspaceFileModel::RelativePathRole).toString();
    const QString path = controller->drives() ? relative : QDir(controller->root()).filePath(relative);
    if (index.data(WorkspaceFileModel::DirectoryRole).toBool())
        navigateTo(path);
    else
        emit openFileRequested(path);
}

void DashboardFilesWidget::clearProject()
{
    DashboardFilesController::get(this)->clearView();
    showStatus(tr("Open a project to browse its files."));
}

void DashboardFilesWidget::updateControls()
{
    const auto *controller = DashboardFilesController::get(this);
    const bool available = controller->available();
    const bool remote = controller->remote();
    m_location->setText(QDir::toNativeSeparators(m_directory));
    m_location->setEnabled(available);
    m_home->setEnabled(available);
    m_home->setToolTip(m_scope == "system" && !remote ? tr("Your home directory") : tr("Project root"));
    m_filesystem->setVisible(m_scope == "system" && !remote);
    m_filesystem->setEnabled(m_scope == "system" && !remote && available);
    m_up->setEnabled(available && !m_directory.isEmpty() && m_directory != controller->root());
}

void DashboardFilesWidget::showFilesystem()
{
    DashboardFilesController::get(this)->showFilesystem();
}

void DashboardFilesWidget::showStatus(const QString &message)
{
    m_status->setText(message);
    m_status->setVisible(!message.isEmpty());
}
