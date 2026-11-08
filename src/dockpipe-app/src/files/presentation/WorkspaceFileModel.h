#pragma once

#include "WorkspaceFilesClient.h"
#include <QAbstractItemModel>
#include <memory>
#include <vector>

// Lazy metadata supplied exclusively by the shared core Files service.
class WorkspaceFileModel final : public QAbstractItemModel {
    Q_OBJECT
public:
    enum Role { RelativePathRole = Qt::UserRole + 1, DirectoryRole, SizeBytesRole };
    explicit WorkspaceFileModel(QObject *parent = nullptr);
    ~WorkspaceFileModel() override;
    void setContext(const WorkspaceFilesClient::Context &context);
    void setRemoteDispatch(WorkspaceFilesClient::RemoteDispatch dispatch);
    void setShowHidden(bool hidden);
    void setDirectory(const QString &relativePath);
    void refresh();
    QString relativePath(const QModelIndex &index) const;
    bool isDirectory(const QModelIndex &index) const;
    QModelIndex indexForPath(const QString &path) const;
    QModelIndex index(int row, int column, const QModelIndex &parent = {}) const override;
    QModelIndex parent(const QModelIndex &index) const override;
    int rowCount(const QModelIndex &parent = {}) const override;
    int columnCount(const QModelIndex &parent = {}) const override;
    QVariant data(const QModelIndex &index, int role = Qt::DisplayRole) const override;
    Qt::ItemFlags flags(const QModelIndex &index) const override;
    bool hasChildren(const QModelIndex &parent = {}) const override;
    bool canFetchMore(const QModelIndex &parent) const override;
    void fetchMore(const QModelIndex &parent) override;

signals:
    void listingFailed(const QString &path, const QString &error);
    void directoryLoaded(const QString &path);

private:
    struct Node;
    Node *node(const QModelIndex &index) const;
    QModelIndex nodeIndex(const Node *node) const;
    void fetch(Node *node);
    void fetchNext();
    WorkspaceFilesClient *m_client;
    std::unique_ptr<Node> m_root;
    quint64 m_generation = 0;
    bool m_hidden = false;
    std::vector<Node *> m_pending;
    QString m_directory = QStringLiteral(".");
};
