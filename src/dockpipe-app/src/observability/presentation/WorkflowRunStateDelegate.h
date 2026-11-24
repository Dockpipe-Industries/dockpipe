#pragma once

#include <QStyledItemDelegate>

class WorkflowRunStateDelegate : public QStyledItemDelegate {
    Q_OBJECT
public:
    explicit WorkflowRunStateDelegate(QObject *parent = nullptr);
    static QString label(const QString &state);
    static QString tone(const QString &state);
    void paint(QPainter *painter, const QStyleOptionViewItem &option, const QModelIndex &index) const override;
    QSize sizeHint(const QStyleOptionViewItem &option, const QModelIndex &index) const override;
};
