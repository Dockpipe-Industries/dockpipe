#include "DashboardFilesWidget.h"

#include <QDir>
#include <QFileInfo>
#include <QFileSystemModel>
#include <QHBoxLayout>
#include <QHeaderView>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QSortFilterProxyModel>
#include <QTreeView>
#include <QVBoxLayout>

namespace {
bool within(const QString &root, const QString &path)
{
    if (root.isEmpty() || path.isEmpty())
        return false;
#ifdef Q_OS_WIN
    constexpr auto sensitivity = Qt::CaseInsensitive;
#else
    constexpr auto sensitivity = Qt::CaseSensitive;
#endif
    const QString prefix = root.endsWith('/') ? root : root + '/';
    return path.compare(root, sensitivity) == 0 || path.startsWith(prefix, sensitivity);
}

bool validPath(const QString &path)
{
    return !path.isEmpty() && path.size() <= 4096 && !path.contains(QChar::Null);
}
}

// Filter at the model boundary as well as navigation: expanding an outside
// symlink must not expose its children inside a project-scoped tree.
class DashboardFilesFilter final : public QSortFilterProxyModel {
public:
    using QSortFilterProxyModel::QSortFilterProxyModel;
    void setContext(bool system, const QString &root)
    {
        m_system = system;
        m_root = root;
        invalidateFilter();
    }

protected:
    bool filterAcceptsRow(int row, const QModelIndex &parent) const override
    {
        if (m_system)
            return true;
        if (m_root.isEmpty())
            return false;
        const auto *files = qobject_cast<QFileSystemModel *>(sourceModel());
        const QString path = files->filePath(files->index(row, 0, parent));
        const QFileInfo entry(path);
        const QString canonical = entry.canonicalFilePath();
        if (within(m_root, canonical))
            return true;
        // Source-model ancestors are needed to map the selected root index.
        // A symlink inside the project pointing to an ancestor is not admitted.
        return QDir::cleanPath(path) == canonical && within(canonical, m_root);
    }

private:
    bool m_system = false;
    QString m_root;
};

DashboardFilesWidget::DashboardFilesWidget(QWidget *parent) : QWidget(parent)
{
    setObjectName("dashboardFilesWidget");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(4, 4, 4, 4);
    layout->setSpacing(4);
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
    navigation->addStretch();
    layout->addLayout(navigation);
    m_location = new QLineEdit;
    m_location->setObjectName("dashboardFilesLocation");
    m_location->setAccessibleName(tr("Directory"));
    m_location->setMaxLength(4096);
    layout->addWidget(m_location);
    m_model = new QFileSystemModel(this);
    m_model->setReadOnly(true);
    m_model->setOption(QFileSystemModel::DontUseCustomDirectoryIcons, true);
    m_model->setFilter(QDir::AllDirs | QDir::Files | QDir::NoDotAndDotDot);
    m_filter = new DashboardFilesFilter(this);
    m_filter->setSourceModel(m_model);
    m_tree = new QTreeView;
    m_tree->setObjectName("dashboardFiles");
    m_tree->setModel(m_filter);
    m_tree->setUniformRowHeights(true);
    m_tree->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_tree->setSelectionMode(QAbstractItemView::SingleSelection);
    m_tree->setSortingEnabled(true);
    m_tree->sortByColumn(0, Qt::AscendingOrder);
    m_tree->header()->setStretchLastSection(false);
    m_tree->header()->setSectionResizeMode(0, QHeaderView::Stretch);
    for (int column = 1; column < m_model->columnCount(); ++column)
        m_tree->hideColumn(column);
    layout->addWidget(m_tree, 1);
    m_status = new QLabel;
    m_status->setObjectName("dashboardFilesStatus");
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    connect(m_up, &QPushButton::clicked, this, [this] { navigateTo(".."); });
    connect(m_home, &QPushButton::clicked, this, [this] {
        navigateTo(m_scope == "system" ? QDir::homePath() : m_projectRoot);
    });
    connect(m_filesystem, &QPushButton::clicked, this, &DashboardFilesWidget::showFilesystem);
    connect(m_location, &QLineEdit::returnPressed, this, [this] { navigateTo(m_location->text()); });
    connect(m_tree, &QTreeView::doubleClicked, this, &DashboardFilesWidget::activate);
    clearProject();
}

void DashboardFilesWidget::setContext(const QString &scope, const QString &projectPath)
{
    if (scope != "project" && scope != "system") {
        m_status->setText(tr("Choose Project or System file scope."));
        return;
    }
    if (scope == "system" && m_scope == "system")
        return;
    const QString root = scope == "project" && validPath(projectPath) && QDir::isAbsolutePath(projectPath)
        ? QFileInfo(projectPath).canonicalFilePath() : QString();
    if (scope == "project" && (root.isEmpty() || !QFileInfo(root).isDir())) {
        m_scope = scope;
        clearProject();
        return;
    }
    if (scope == "project" && m_scope == scope && m_projectRoot == root && !m_directory.isEmpty()) {
        if (!QFileInfo(m_directory).isDir())
            navigateTo(root);
        return;
    }
    m_scope = scope;
    m_projectRoot = root;
    m_directory.clear();
    m_filter->setContext(scope == "system", root);
    m_tree->setRootIndex({});
    m_tree->hide();
    updateControls();
    navigateTo(scope == "system" ? QDir::homePath() : root);
}

bool DashboardFilesWidget::navigateTo(const QString &path)
{
    if (!validPath(path) || (m_scope == "project" && m_projectRoot.isEmpty())
        || (m_directory.isEmpty() && !QDir::isAbsolutePath(path))) {
        m_status->setText(tr("Choose an existing directory."));
        return false;
    }
    const QString candidate = QDir::isAbsolutePath(path) ? path : QDir(m_directory).absoluteFilePath(path);
    const QFileInfo entry(candidate);
    const QString canonical = entry.canonicalFilePath();
    if (canonical.isEmpty() || !entry.isDir() || !entry.isReadable()) {
        m_status->setText(tr("This directory is missing or cannot be read."));
        return false;
    }
    if (m_scope == "project" && !within(m_projectRoot, canonical)) {
        m_status->setText(tr("Project scope stays inside the selected project, including symbolic links."));
        return false;
    }
    const auto rootIndex = m_model->setRootPath(canonical);
    m_tree->setRootIndex(m_filter->mapFromSource(rootIndex));
    m_tree->collapseAll();
    m_directory = canonical;
    m_tree->show();
    m_status->clear();
    updateControls();
    return true;
}

void DashboardFilesWidget::activate(const QModelIndex &index)
{
    if (!index.isValid())
        return;
    const QFileInfo entry(m_model->filePath(m_filter->mapToSource(index)));
    const QString canonical = entry.canonicalFilePath();
    if (canonical.isEmpty() || (m_scope == "project" && !within(m_projectRoot, canonical))) {
        m_status->setText(tr("This item is unavailable in the current file scope."));
        return;
    }
    if (entry.isDir())
        navigateTo(canonical);
    else if (entry.isFile() && entry.isReadable())
        emit openFileRequested(canonical);
}

void DashboardFilesWidget::clearProject()
{
    m_projectRoot.clear();
    m_directory.clear();
    m_filter->setContext(false, {});
    m_tree->setRootIndex({});
    m_tree->hide();
    m_model->setRootPath({});
    m_status->setText(tr("Open a project to browse its files."));
    updateControls();
}

void DashboardFilesWidget::updateControls()
{
    const bool available = m_scope == "system" || !m_projectRoot.isEmpty();
    m_location->setText(QDir::toNativeSeparators(m_directory));
    m_location->setEnabled(available);
    m_home->setEnabled(available);
    m_home->setToolTip(m_scope == "system" ? tr("Your home directory") : tr("Project root"));
    m_filesystem->setVisible(m_scope == "system");
    m_filesystem->setEnabled(m_scope == "system");
    m_up->setEnabled(available && !m_directory.isEmpty()
        && (m_scope == "project" ? m_directory != m_projectRoot : !QDir(m_directory).isRoot()));
}

void DashboardFilesWidget::showFilesystem()
{
    if (m_scope != "system")
        return;
#ifdef Q_OS_WIN
    m_model->setRootPath({});
    m_tree->setRootIndex({});
    m_directory.clear();
    m_tree->show();
    m_status->setText(tr("Choose a drive."));
    updateControls();
#else
    navigateTo(QDir::rootPath());
#endif
}
