#include "GitCommitQueueModel.h"

#include <QRegularExpression>

GitCommitQueueModel::GitCommitQueueModel(QObject *parent) : QAbstractListModel(parent)
{
}

int GitCommitQueueModel::rowCount(const QModelIndex &parent) const
{
    return parent.isValid() ? 0 : m_commits.size();
}

QVariant GitCommitQueueModel::data(const QModelIndex &index, int role) const
{
    if (!index.isValid() || index.model() != this || index.column() != 0
        || index.row() < 0 || index.row() >= m_commits.size())
        return {};

    const Commit &commit = m_commits[index.row()];
    switch (role) {
    case Qt::DisplayRole:
        return commit.id.left(12) + QStringLiteral("  ") + commit.subject;
    case Qt::ToolTipRole:
        return commit.id + QLatin1Char('\n') + commit.subject;
    case IdRole:
        return commit.id;
    case SubjectRole:
        return commit.subject;
    default:
        return {};
    }
}

QHash<int, QByteArray> GitCommitQueueModel::roleNames() const
{
    auto names = QAbstractListModel::roleNames();
    names.insert(IdRole, "commitId");
    names.insert(SubjectRole, "subject");
    return names;
}

bool GitCommitQueueModel::addCommit(const QString &id, const QString &subject)
{
    static const QRegularExpression fullId(QStringLiteral("\\A(?:[0-9a-fA-F]{40}|[0-9a-fA-F]{64})\\z"));
    if (m_commits.size() >= maximumCommits || !fullId.match(id).hasMatch()
        || subject.size() > maximumSubjectLength || subject.contains(QLatin1Char('\n'))
        || subject.contains(QLatin1Char('\r')) || subject.contains(QChar::Null))
        return false;

    const QString normalizedId = id.toLower();
    for (const Commit &commit : m_commits) {
        if (commit.id == normalizedId)
            return false;
    }

    const int row = m_commits.size();
    beginInsertRows(QModelIndex(), row, row);
    m_commits.append({normalizedId, subject});
    endInsertRows();
    return true;
}

QStringList GitCommitQueueModel::commitIds() const
{
    QStringList ids;
    ids.reserve(m_commits.size());
    for (const Commit &commit : m_commits)
        ids.append(commit.id);
    return ids;
}

bool GitCommitQueueModel::moveCommit(int from, int to)
{
    if (from < 0 || to < 0 || from >= m_commits.size() || to >= m_commits.size())
        return false;
    if (from == to)
        return true;

    // Qt's destination is the insertion position before removing the source.
    const int destination = to > from ? to + 1 : to;
    if (!beginMoveRows(QModelIndex(), from, from, QModelIndex(), destination))
        return false;
    m_commits.move(from, to);
    endMoveRows();
    return true;
}

bool GitCommitQueueModel::removeCommit(int row)
{
    if (row < 0 || row >= m_commits.size())
        return false;

    beginRemoveRows(QModelIndex(), row, row);
    m_commits.removeAt(row);
    endRemoveRows();
    return true;
}

void GitCommitQueueModel::clear()
{
    if (m_commits.isEmpty())
        return;

    beginResetModel();
    m_commits.clear();
    endResetModel();
}
