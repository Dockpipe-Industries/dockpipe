#include "WorkspaceFileModel.h"

#include <QJsonArray>
#include <QSet>
#include <QTimer>

struct WorkspaceFileModel::Node {
    QString name;
    QString path;
    QString sizeBytes;
    QString revision;
    QString after;
    Node *parent = nullptr;
    std::vector<std::unique_ptr<Node>> children;
    bool directory = true;
    bool loaded = false;
    bool more = true;
    bool loading = false;
    bool failed = false;
    bool available = true;
};

WorkspaceFileModel::WorkspaceFileModel(QObject *parent)
    : QAbstractItemModel(parent), m_client(new WorkspaceFilesClient(this)), m_root(std::make_unique<Node>())
{
    m_root->path = QStringLiteral(".");
    connect(m_client, &WorkspaceFilesClient::busyChanged, this, [this](bool busy) {
        if (!busy)
            QTimer::singleShot(0, this, &WorkspaceFileModel::fetchNext);
    });
}

WorkspaceFileModel::~WorkspaceFileModel() = default;

void WorkspaceFileModel::setContext(const WorkspaceFilesClient::Context &context)
{
    m_client->setContext(context);
    m_directory = QStringLiteral(".");
    refresh();
}

void WorkspaceFileModel::setDirectory(const QString &relativePath)
{
    if (m_directory == relativePath)
        return;
    m_directory = relativePath;
    refresh();
}

void WorkspaceFileModel::setRemoteDispatch(WorkspaceFilesClient::RemoteDispatch dispatch)
{
    m_client->setRemoteDispatch(std::move(dispatch));
    refresh();
}

void WorkspaceFileModel::setShowHidden(bool hidden)
{
    if (m_hidden == hidden)
        return;
    m_hidden = hidden;
    refresh();
}

void WorkspaceFileModel::refresh()
{
    ++m_generation;
    m_pending.clear();
    m_client->cancel();
    beginResetModel();
    m_root = std::make_unique<Node>();
    m_root->path = m_directory;
    endResetModel();
    if (m_client->context().kind == WorkspaceFilesClient::Context::Kind::Local
        && m_client->context().localRoot.isEmpty())
        return;
    // A cancelled helper may still be reaping. Fetch only after its slot clears.
    if (m_client->busy()) {
        const quint64 generation = m_generation;
        auto connection = std::make_shared<QMetaObject::Connection>();
        *connection = connect(m_client, &WorkspaceFilesClient::busyChanged, this,
            [this, generation, connection](bool busy) {
            if (busy)
                return;
            disconnect(*connection);
            if (generation == m_generation)
                fetch(m_root.get());
        });
    } else {
        fetch(m_root.get());
    }
}

WorkspaceFileModel::Node *WorkspaceFileModel::node(const QModelIndex &index) const
{
    return index.isValid() ? static_cast<Node *>(index.internalPointer()) : m_root.get();
}

QModelIndex WorkspaceFileModel::nodeIndex(const Node *item) const
{
    if (item == m_root.get() || !item->parent)
        return {};
    const auto &siblings = item->parent->children;
    for (size_t row = 0; row < siblings.size(); ++row) {
        if (siblings[row].get() == item)
            return createIndex(static_cast<int>(row), 0, item);
    }
    return {};
}

QModelIndex WorkspaceFileModel::index(int row, int column, const QModelIndex &parent) const
{
    if (row < 0 || column != 0 || (parent.isValid() && parent.column() != 0))
        return {};
    Node *directory = node(parent);
    if (static_cast<size_t>(row) >= directory->children.size())
        return {};
    return createIndex(row, column, directory->children[row].get());
}

QModelIndex WorkspaceFileModel::parent(const QModelIndex &index) const
{
    return index.isValid() ? nodeIndex(node(index)->parent) : QModelIndex();
}

int WorkspaceFileModel::rowCount(const QModelIndex &parent) const
{
    if (parent.isValid() && parent.column() != 0)
        return 0;
    return static_cast<int>(node(parent)->children.size());
}

int WorkspaceFileModel::columnCount(const QModelIndex &) const
{
    return 1;
}

QString WorkspaceFileModel::relativePath(const QModelIndex &index) const
{
    return node(index)->path;
}

bool WorkspaceFileModel::isDirectory(const QModelIndex &index) const
{
    return node(index)->directory;
}

QVariant WorkspaceFileModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid())
        return {};
    const Node *item = node(index);
    switch (role) {
    case Qt::DisplayRole:
    case Qt::ToolTipRole:
        return item->name;
    case RelativePathRole:
        return item->path;
    case DirectoryRole:
        return item->directory;
    case SizeBytesRole:
        return item->sizeBytes;
    default:
        return {};
    }
}

Qt::ItemFlags WorkspaceFileModel::flags(const QModelIndex &index) const
{
    return index.isValid() && node(index)->available
        ? Qt::ItemIsEnabled | Qt::ItemIsSelectable : Qt::NoItemFlags;
}

bool WorkspaceFileModel::hasChildren(const QModelIndex &parent) const
{
    const Node *item = node(parent);
    return item->directory && (!item->loaded || !item->children.empty());
}

bool WorkspaceFileModel::canFetchMore(const QModelIndex &parent) const
{
    const Node *item = node(parent);
    const auto context = m_client->context();
    const bool selected = context.kind == WorkspaceFilesClient::Context::Kind::Remote || !context.localRoot.isEmpty();
    return selected && item->directory && item->more && !item->loading && !item->failed;
}

void WorkspaceFileModel::fetchMore(const QModelIndex &parent)
{
    if (canFetchMore(parent))
        fetch(node(parent));
}

QModelIndex WorkspaceFileModel::indexForPath(const QString &path) const
{
    if (path.isEmpty() || path == m_directory)
        return {};
    QString descendant = path;
    if (m_directory != QStringLiteral(".")) {
        if (!descendant.startsWith(m_directory + '/'))
            return {};
        descendant.remove(0, m_directory.size() + 1);
    }
    Node *current = m_root.get();
    for (const QString &part : descendant.split('/')) {
        Node *found = nullptr;
        for (const auto &child : current->children) {
            if (child->name == part) {
                found = child.get();
                break;
            }
        }
        if (!found)
            return {};
        current = found;
    }
    return nodeIndex(current);
}

void WorkspaceFileModel::fetch(Node *directory)
{
    if (directory->loading || !directory->more)
        return;
    if (m_client->busy()) {
        // At most one directory page of outstanding user expansions is retained.
        if (m_pending.size() >= 150) {
            directory->failed = true;
            emit listingFailed(directory->path, tr("Too many pending workspace directory requests; refresh to continue."));
            return;
        }
        directory->loading = true;
        m_pending.push_back(directory);
        return;
    }
    directory->loading = true;
    QJsonObject request{{"schema", "dockpipe.files/v1"}, {"action", "list"},
        {"path", directory->path}, {"limit", 150}, {"hidden", m_hidden}};
    if (!directory->after.isEmpty()) {
        request.insert("after", directory->after);
        request.insert("revision", directory->revision);
    }
    const quint64 generation = m_generation;
    m_client->run(request, this, [this, directory, generation](const WorkspaceFilesClient::Result &result) {
        if (generation != m_generation)
            return;
        if (!result.success()) {
            directory->loading = false;
            directory->failed = true;
            emit listingFailed(directory->path, result.error);
            return;
        }
        const auto entries = result.value.value("entries").toArray();
        const bool more = result.value.value("more").toBool();
        const QString after = result.value.value("after").toString();
        const QString revision = result.value.value("revision").toString();
        bool valid = entries.size() <= 150 && result.value.value("path").toString() == directory->path
            && revision.size() == 64 && (directory->revision.isEmpty() || revision == directory->revision)
            && (!more || (!after.isEmpty() && after != directory->after));
        QSet<QString> names;
        for (const auto &child : directory->children)
            names.insert(child->name);
        for (const auto &entry : entries) {
            const auto value = entry.toObject();
            const QString name = value.value("name").toString();
            const QString kind = value.value("kind").toString();
            valid = valid && !name.isEmpty() && name != "." && name != ".." && !name.contains('/')
                && !name.contains('\\') && !name.contains(QChar(0)) && name.compare(".git", Qt::CaseInsensitive) != 0
                && !names.contains(name) && (kind == "file" || kind == "directory" || kind == "link" || kind == "unavailable");
            names.insert(name);
        }
        if (!valid) {
            directory->loading = false;
            directory->failed = true;
            emit listingFailed(directory->path, tr("Invalid workspace directory page."));
            return;
        }
        if (!entries.isEmpty()) {
            const int first = static_cast<int>(directory->children.size());
            beginInsertRows(nodeIndex(directory), first, first + entries.size() - 1);
            for (const auto &entry : entries) {
                const auto value = entry.toObject();
                auto child = std::make_unique<Node>();
                child->name = value.value("name").toString();
                child->directory = value.value("kind") == QStringLiteral("directory");
                child->available = child->directory || value.value("kind") == QStringLiteral("file");
                child->parent = directory;
                child->path = directory->path == "." ? child->name : directory->path + '/' + child->name;
                const auto size = value.value("size_bytes");
                child->sizeBytes = size.isString() ? size.toString() : QString::number(size.toInteger());
                directory->children.push_back(std::move(child));
            }
            endInsertRows();
        }
        directory->loaded = true;
        directory->more = more;
        directory->after = after;
        directory->revision = revision;
        directory->loading = false;
        emit directoryLoaded(directory->path);
    });
}

void WorkspaceFileModel::fetchNext()
{
    if (m_client->busy() || m_pending.empty())
        return;
    Node *directory = m_pending.front();
    m_pending.erase(m_pending.begin());
    directory->loading = false;
    fetch(directory);
}
