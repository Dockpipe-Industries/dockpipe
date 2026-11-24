#pragma once

#include <QCoreApplication>

#include <QJsonArray>
#include <QStyledItemDelegate>
#include <QVector>

class QListWidget;

// A segment uses row-relative y coordinates (0 = top, 1 = bottom).
struct GitCommitGraphSegment {
    int fromColumn = 0;
    int toColumn = 0;
    qreal fromY = 0;
    qreal toY = 1;
    int color = 0;
};

struct GitCommitGraphRow {
    int column = 0;
    int columns = 1;
    bool parentOutsidePage = false;
    QVector<GitCommitGraphSegment> segments;
};

class GitCommitGraphDelegate final : public QStyledItemDelegate {
    Q_DECLARE_TR_FUNCTIONS(GitCommitGraphDelegate)
public:
    explicit GitCommitGraphDelegate(QObject *parent = nullptr);

    // The History protocol supplies at most 50 commits in child-before-parent order.
    // Empty results also represent invalid/oversized input; prepare reports failure.
    static QVector<GitCommitGraphRow> layout(const QJsonArray &commits);
    static bool prepare(QListWidget *list, const QJsonArray &commits);

    void paint(QPainter *painter, const QStyleOptionViewItem &option,
        const QModelIndex &index) const override;
    QSize sizeHint(const QStyleOptionViewItem &option, const QModelIndex &index) const override;

private:
    enum Roles {
        GraphRole = Qt::UserRole + 2,
        AuthorRole,
        DateRole,
        RefsRole
    };
};
