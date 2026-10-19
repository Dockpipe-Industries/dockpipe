#pragma once

#include "LauncherUiExport.h"

#include <QAbstractListModel>
#include <QStringList>
#include <QVector>

// An explicitly ordered selection of exact commits. Execution belongs to the
// project Git runtime; editing this model never reads or changes a repository.
class DOCKPIPE_LAUNCHER_UI_EXPORT GitCommitQueueModel final : public QAbstractListModel {
    Q_OBJECT
public:
    enum Role { IdRole = Qt::UserRole + 1, SubjectRole };
    Q_ENUM(Role)

    // Match the runtime's bounded batch and message contracts.
    static constexpr int maximumCommits = 128;
    static constexpr int maximumSubjectLength = 4096;

    explicit GitCommitQueueModel(QObject *parent = nullptr);
    int rowCount(const QModelIndex &parent = QModelIndex()) const override;
    QVariant data(const QModelIndex &index, int role = Qt::DisplayRole) const override;
    QHash<int, QByteArray> roleNames() const override;

    bool addCommit(const QString &id, const QString &subject);
    QStringList commitIds() const;
    bool moveCommit(int from, int to);
    bool removeCommit(int row);
    void clear();

private:
    struct Commit {
        QString id;
        QString subject;
    };
    QVector<Commit> m_commits;
};
