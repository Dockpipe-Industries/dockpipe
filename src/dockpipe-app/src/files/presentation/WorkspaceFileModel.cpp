#include "WorkspaceFileModel.h"

#include <QJsonArray>
#include <QJsonDocument>
#include <QSet>
#include <QTimer>

struct WorkspaceFileModel::Node {
    QString name;
    QString path;
    QString sizeBytes;
    QString revision;
    QString after;
    QJsonArray firstPage;
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
    m_backgroundClient = new WorkspaceFilesClient(this);
    m_root->path = QStringLiteral(".");
    connect(m_backgroundClient, &WorkspaceFilesClient::busyChanged, this, [this](bool busy) {
        if (!busy) {
            QTimer::singleShot(0, this, [this]() {
                if (m_refreshPending)
                    revalidate();
                else
                    prefetchNext();
            });
        }
    });
    connect(m_client, &WorkspaceFilesClient::busyChanged, this, [this](bool busy) {
        if (!busy)
            QTimer::singleShot(0, this, &WorkspaceFileModel::fetchNext);
    });
}

WorkspaceFileModel::~WorkspaceFileModel() = default;

void WorkspaceFileModel::setContext(const WorkspaceFilesClient::Context &context)
{
    m_cache.clear();
    m_cacheOrder.clear();
    m_cacheBytes = 0;
    m_backgroundClient->setContext(context);
    m_client->setContext(context);
    m_directory = QStringLiteral(".");
    resetDirectory(false);
}

void WorkspaceFileModel::setDirectory(const QString &relativePath)
{
    if (m_directory == relativePath)
        return;
    m_directory = relativePath;
    resetDirectory(true);
}

void WorkspaceFileModel::setRemoteDispatch(WorkspaceFilesClient::RemoteDispatch dispatch)
{
    m_backgroundClient->setRemoteDispatch(dispatch);
    m_client->setRemoteDispatch(std::move(dispatch));
    m_cache.clear();
    m_cacheOrder.clear();
    m_cacheBytes = 0;
    resetDirectory(false);
}

void WorkspaceFileModel::setShowHidden(bool hidden)
{
    if (m_hidden == hidden)
        return;
    m_hidden = hidden;
    m_cache.clear();
    m_cacheOrder.clear();
    m_cacheBytes = 0;
    resetDirectory(false);
}

void WorkspaceFileModel::refresh()
{
    m_cache.clear();
    m_cacheOrder.clear();
    m_cacheBytes = 0;
    m_backgroundClient->cancel();
    revalidate();
}

void WorkspaceFileModel::resetDirectory(bool useCache)
{
    ++m_generation;
    m_refreshPending = false;
    m_prefetch.clear();
    m_backgroundClient->cancel();
    m_pending.clear();
    m_client->cancel();
    m_changingModel = true;
    beginResetModel();
    m_root = std::make_unique<Node>();
    m_root->path = m_directory;
    endResetModel();
    m_changingModel = false;
    if (useCache && m_cache.contains(m_directory)) {
        appendPage(m_root.get(), m_cache.value(m_directory));
        revalidate();
        return;
    }
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
    return !m_changingModel && selected && item->directory && item->more && !item->loading && !item->failed;
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
    if (!directory->loaded && m_cache.contains(directory->path)) {
        directory->loading = true;
        appendPage(directory, m_cache.value(directory->path));
        if (m_active) {
            m_prefetch.removeAll(directory->path);
            m_prefetch.prepend(directory->path);
            while (m_prefetch.size() > 4)
                m_prefetch.removeLast();
            QTimer::singleShot(0, this, &WorkspaceFileModel::prefetchNext);
        }
        return;
    }
    // User-requested reads take priority over speculative directory work.
    if (m_backgroundClient->busy())
        m_backgroundClient->cancel();
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
        if (!validPage(directory, result.value)) {
            directory->loading = false;
            directory->failed = true;
            emit listingFailed(directory->path, tr("Invalid workspace directory page."));
            return;
        }
        if (directory->revision.isEmpty())
            cachePage(directory->path, result.value);
        appendPage(directory, result.value);
        if (m_active && directory == m_root.get()) {
            m_prefetch.clear();
            for (const auto &child : directory->children) {
                if (child->directory && (!m_cache.contains(child->path) || child->loaded || child->failed)
                    && m_prefetch.size() < 4)
                    m_prefetch.append(child->path);
            }
            QTimer::singleShot(0, this, &WorkspaceFileModel::prefetchNext);
        }
    });
}

void WorkspaceFileModel::fetchNext()
{
    if (m_client->busy())
        return;
    if (m_pending.empty()) {
        if (m_refreshPending)
            revalidate();
        else
            prefetchNext();
        return;
    }
    Node *directory = m_pending.front();
    m_pending.erase(m_pending.begin());
    directory->loading = false;
    fetch(directory);
}

bool WorkspaceFileModel::validPage(const Node *directory, const QJsonObject &page) const
{
    const auto entries = page.value("entries").toArray();
    const bool more = page.value("more").toBool();
    const QString after = page.value("after").toString();
    const QString revision = page.value("revision").toString();
    bool valid = entries.size() <= 150 && page.value("path").toString() == directory->path
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
    return valid;
}

void WorkspaceFileModel::appendPage(Node *directory, const QJsonObject &page)
{
    const auto entries = page.value("entries").toArray();
    const bool more = page.value("more").toBool();
    const QString after = page.value("after").toString();
    const QString revision = page.value("revision").toString();
    if (directory->revision.isEmpty())
        directory->firstPage = entries;
    if (!entries.isEmpty()) {
        const int first = static_cast<int>(directory->children.size());
        m_changingModel = true;
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
        m_changingModel = false;
    }
    directory->loaded = true;
    directory->more = more;
    directory->after = after;
    directory->revision = revision;
    directory->loading = false;
    directory->failed = false;
    emit directoryLoaded(directory->path);
}

void WorkspaceFileModel::cachePage(const QString &path, const QJsonObject &page)
{
    constexpr qsizetype maximumBytes = 2 << 20;
    constexpr int maximumDirectories = 32;
    const auto size = QJsonDocument(page).toJson(QJsonDocument::Compact).size();
    if (size > maximumBytes)
        return;
    if (m_cache.contains(path)) {
        m_cacheBytes -= QJsonDocument(m_cache.take(path)).toJson(QJsonDocument::Compact).size();
        m_cacheOrder.removeAll(path);
    }
    while (!m_cacheOrder.isEmpty()
        && (m_cacheOrder.size() >= maximumDirectories || m_cacheBytes + size > maximumBytes)) {
        m_cacheBytes -= QJsonDocument(m_cache.take(m_cacheOrder.takeFirst())).toJson(QJsonDocument::Compact).size();
    }
    m_cache.insert(path, page);
    m_cacheOrder.append(path);
    m_cacheBytes += size;
}

void WorkspaceFileModel::setActive(bool active)
{
    m_active = active;
    if (active) {
        revalidate();
    } else {
        m_prefetch.clear();
        m_refreshPending = false;
        m_backgroundClient->cancel();
    }
}

void WorkspaceFileModel::revalidate()
{
    const auto context = m_client->context();
    if (context.kind == WorkspaceFilesClient::Context::Kind::Local && context.localRoot.isEmpty())
        return;
    if (m_client->busy() || m_backgroundClient->busy()) {
        m_refreshPending = true;
        return;
    }
    m_refreshPending = false;
    const quint64 generation = m_generation;
    const QString path = m_directory;
    const QJsonObject request{{"schema", "dockpipe.files/v1"}, {"action", "list"},
        {"path", path}, {"limit", 150}, {"hidden", m_hidden}};
    m_backgroundClient->run(request, this, [this, generation, path](const auto &result) {
        if (generation != m_generation || result.outcome == WorkspaceFilesClient::Outcome::Cancelled)
            return;
        Node fresh;
        fresh.path = path;
        if (!result.success() || !validPage(&fresh, result.value)) {
            emit listingFailed(path, result.success() ? tr("Invalid workspace directory page.") : result.error);
            return;
        }
        const bool metadataChanged = m_root->firstPage != result.value.value("entries").toArray();
        cachePage(path, result.value);
        // Do not invalidate node pointers owned by an in-flight foreground expansion.
        if (m_client->busy() || !m_pending.empty())
            return;
        if (!m_root->loaded || metadataChanged
            || m_root->revision != result.value.value("revision").toString()) {
            m_changingModel = true;
            beginResetModel();
            m_root = std::make_unique<Node>();
            m_root->path = path;
            endResetModel();
            m_changingModel = false;
            appendPage(m_root.get(), result.value);
        } else {
            emit directoryLoaded(path);
        }
        if (m_active) {
            m_prefetch.clear();
            for (const auto &child : m_root->children) {
                if (child->directory && (!m_cache.contains(child->path) || child->loaded || child->failed)
                    && m_prefetch.size() < 4)
                    m_prefetch.append(child->path);
            }
        }
    });
}

void WorkspaceFileModel::prefetchNext()
{
    if (!m_active || m_client->busy() || m_backgroundClient->busy() || m_prefetch.isEmpty())
        return;
    const QString path = m_prefetch.takeFirst();
    const quint64 generation = m_generation;
    const QJsonObject request{{"schema", "dockpipe.files/v1"}, {"action", "list"},
        {"path", path}, {"limit", 150}, {"hidden", m_hidden}};
    m_backgroundClient->run(request, this, [this, generation, path](const auto &result) {
        if (generation != m_generation || !result.success())
            return;
        Node fresh;
        fresh.path = path;
        if (!validPage(&fresh, result.value))
            return;
        cachePage(path, result.value);
        const auto index = indexForPath(path);
        if (!index.isValid() || m_client->busy() || !m_pending.empty())
            return;
        Node *directory = node(index);
        // Prefetch alone does not expand the tree. Previously opened folders
        // can update in place without resetting their parent or editor state.
        if (!directory->loaded && !directory->failed)
            return;
        if (directory->revision != result.value.value("revision").toString()
            || directory->firstPage != result.value.value("entries").toArray()) {
            if (!directory->children.empty()) {
                m_changingModel = true;
                beginRemoveRows(index, 0, static_cast<int>(directory->children.size()) - 1);
                directory->children.clear();
                endRemoveRows();
                m_changingModel = false;
            }
            directory->revision.clear();
            directory->after.clear();
            directory->failed = false;
            appendPage(directory, result.value);
        } else if (directory->failed) {
            directory->failed = false;
            if (directory->more)
                fetch(directory);
        }
    });
}
