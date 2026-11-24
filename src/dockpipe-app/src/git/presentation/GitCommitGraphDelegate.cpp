#include "GitCommitGraphDelegate.h"

#include <QApplication>
#include <QHash>
#include <QJsonObject>
#include <QListWidget>
#include <QPainter>
#include <QPainterPath>
#include <QSet>
#include <QStyle>
#include <QTextDocument>

namespace {
constexpr int maximumCommits = 50;
constexpr int laneWidth = 16;
constexpr int graphMargin = 14;

QColor branchColor(int column, const QPalette &palette)
{
    const bool dark = palette.color(QPalette::Base).lightness() < 128;
    static const int hues[] = {211, 153, 32, 275, 345, 185, 65};
    return QColor::fromHsl(hues[column % 7], dark ? 165 : 180, dark ? 165 : 95);
}

QVariantMap encodeRow(const GitCommitGraphRow &row)
{
    QVariantList segments;
    for (const auto &segment : row.segments) {
        segments.append(QVariantMap{{"from", segment.fromColumn}, {"to", segment.toColumn},
            {"start", segment.fromY}, {"end", segment.toY}, {"color", segment.color}});
    }
    return {{"column", row.column}, {"columns", row.columns},
        {"outside", row.parentOutsidePage}, {"segments", segments}};
}
}

GitCommitGraphDelegate::GitCommitGraphDelegate(QObject *parent) : QStyledItemDelegate(parent)
{
}

QVector<GitCommitGraphRow> GitCommitGraphDelegate::layout(const QJsonArray &commits)
{
    if (commits.size() > maximumCommits)
        return {};

    QHash<QString, int> rowById;
    for (int row = 0; row < commits.size(); ++row) {
        const QString id = commits[row].toObject().value("id").toString();
        if (id.isEmpty() || rowById.contains(id))
            return {};
        rowById.insert(id, row);
    }

    QVector<GitCommitGraphRow> rows(commits.size());
    QStringList lanes;
    int maximumColumns = 1;
    auto reserveLane = [&lanes](const QString &id) {
        int column = lanes.indexOf(id);
        if (column >= 0)
            return column;
        column = lanes.indexOf(QString());
        if (column < 0) {
            column = lanes.size();
            lanes.append(id);
        } else {
            lanes[column] = id;
        }
        return column;
    };

    for (int row = 0; row < commits.size(); ++row) {
        const auto commit = commits[row].toObject();
        const int column = reserveLane(commit.value("id").toString());
        rows[row].column = column;
        lanes[column].clear();
        QSet<QString> seenParents;
        for (const auto &value : commit.value("parents").toArray()) {
            const QString parent = value.toString();
            if (parent.isEmpty() || seenParents.contains(parent))
                continue;
            seenParents.insert(parent);
            const int parentRow = rowById.value(parent, -1);
            if (parentRow <= row) {
                // Missing/filtered parents and out-of-order input never imply an edge
                // to an unrelated adjacent commit.
                rows[row].parentOutsidePage = true;
                continue;
            }
            reserveLane(parent);
        }
        maximumColumns = qMax(maximumColumns, int(lanes.size()));
    }

    // Each exact child/parent edge is sliced into row-local segments. The first
    // half-row fans out; intervening rows retain the parent's reserved lane.
    for (int row = 0; row < commits.size(); ++row) {
        rows[row].columns = maximumColumns;
        QSet<QString> seenParents;
        for (const auto &value : commits[row].toObject().value("parents").toArray()) {
            const QString parent = value.toString();
            const int parentRow = rowById.value(parent, -1);
            if (parentRow <= row || seenParents.contains(parent))
                continue;
            seenParents.insert(parent);
            const int parentColumn = rows[parentRow].column;
            rows[row].segments.append({rows[row].column, parentColumn, 0.5, 1, parentColumn});
            for (int middle = row + 1; middle < parentRow; ++middle)
                rows[middle].segments.append({parentColumn, parentColumn, 0, 1, parentColumn});
            rows[parentRow].segments.append({parentColumn, parentColumn, 0, 0.5, parentColumn});
        }
    }
    return rows;
}

bool GitCommitGraphDelegate::prepare(QListWidget *list, const QJsonArray &commits)
{
    if (!list || list->count() != commits.size() || commits.size() > maximumCommits)
        return false;
    const auto rows = layout(commits);
    if (rows.size() != commits.size())
        return false;
    for (int row = 0; row < rows.size(); ++row) {
        if (list->item(row)->data(Qt::UserRole).toString() != commits[row].toObject().value("id").toString())
            return false;
    }
    for (int row = 0; row < rows.size(); ++row) {
        auto *item = list->item(row);
        const auto commit = commits[row].toObject();
        item->setData(GraphRole, encodeRow(rows[row]));
        item->setData(AuthorRole, commit.value("author").toString());
        item->setData(DateRole, commit.value("date").toString());
        QStringList refs;
        const auto refValue = commit.value("refs");
        if (refValue.isArray()) {
            for (const auto &ref : refValue.toArray())
                refs.append(ref.toString());
        } else if (refValue.isString()) {
            refs.append(refValue.toString());
        }
        refs.removeAll(QString());
        item->setData(RefsRole, refs);
        QStringList tooltip{commit.value("id").toString(), commit.value("subject").toString(),
            commit.value("author").toString(), commit.value("date").toString()};
        tooltip.append(refs);
        if (rows[row].parentOutsidePage)
            tooltip.append(tr("Parent outside this history page or search results"));
        item->setToolTip(Qt::convertFromPlainText(tooltip.join('\n')));
    }
    list->doItemsLayout();
    list->viewport()->update();
    return true;
}

QSize GitCommitGraphDelegate::sizeHint(const QStyleOptionViewItem &option, const QModelIndex &index) const
{
    if (!index.data(GraphRole).isValid())
        return QStyledItemDelegate::sizeHint(option, index);
    const auto graph = index.data(GraphRole).toMap();
    const int graphWidth = graphMargin * 2 + graph.value("columns", 1).toInt() * laneWidth;
    return {graphWidth + 250, qMax(58, option.fontMetrics.height() * 2 + 20)};
}

void GitCommitGraphDelegate::paint(QPainter *painter, const QStyleOptionViewItem &option,
    const QModelIndex &index) const
{
    if (!index.data(GraphRole).isValid()) {
        QStyledItemDelegate::paint(painter, option, index);
        return;
    }
    QStyleOptionViewItem panel(option);
    initStyleOption(&panel, index);
    panel.text.clear();
    const QStyle *style = option.widget ? option.widget->style() : QApplication::style();
    style->drawControl(QStyle::CE_ItemViewItem, &panel, painter, option.widget);

    painter->save();
    painter->setClipRect(option.rect);
    painter->setRenderHint(QPainter::Antialiasing);
    const auto graph = index.data(GraphRole).toMap();
    auto point = [&option](int column, qreal y) {
        return QPointF(option.rect.left() + graphMargin + column * laneWidth,
            option.rect.top() + y * option.rect.height());
    };
    for (const auto &value : graph.value("segments").toList()) {
        const auto segment = value.toMap();
        const QPointF start = point(segment.value("from").toInt(), segment.value("start").toReal());
        const QPointF end = point(segment.value("to").toInt(), segment.value("end").toReal());
        painter->setPen(QPen(branchColor(segment.value("color").toInt(), option.palette), 2));
        QPainterPath path(start);
        path.cubicTo(QPointF(start.x(), (start.y() + end.y()) / 2),
            QPointF(end.x(), (start.y() + end.y()) / 2), end);
        painter->drawPath(path);
    }
    const int column = graph.value("column").toInt();
    const QColor color = branchColor(column, option.palette);
    const QPointF center = point(column, 0.5);
    if (graph.value("outside").toBool()) {
        painter->setPen(QPen(color, 2, Qt::DashLine));
        painter->drawLine(center, point(column, 0.86));
        painter->drawLine(point(column, 0.86) + QPointF(-3, -3), point(column, 0.86));
        painter->drawLine(point(column, 0.86) + QPointF(3, -3), point(column, 0.86));
    }
    painter->setPen(QPen(option.palette.color(QPalette::Base), 2));
    painter->setBrush(color);
    painter->drawEllipse(center, 4.5, 4.5);

    const bool selected = option.state.testFlag(QStyle::State_Selected);
    // The application styles selection backgrounds through QSS; palette Text
    // retains the appropriate contrast for both light and dark surfaces.
    const QColor text = option.palette.color(QPalette::Text);
    const int graphWidth = graphMargin * 2 + graph.value("columns", 1).toInt() * laneWidth;
    const QRect content = option.rect.adjusted(graphWidth, 7, -12, -7);
    painter->setPen(text);
    QFont subjectFont = option.font;
    subjectFont.setWeight(QFont::DemiBold);
    painter->setFont(subjectFont);
    const QFontMetrics subjectMetrics(subjectFont);
    const QRect subjectRect(content.left(), content.top(), content.width(), subjectMetrics.height());
    painter->drawText(subjectRect, Qt::AlignVCenter,
        subjectMetrics.elidedText(index.data(RefsRole).toStringList().isEmpty()
            ? index.data(Qt::UserRole + 1).toString()
            : "[" + index.data(RefsRole).toStringList().join(", ") + "]  " + index.data(Qt::UserRole + 1).toString(),
            Qt::ElideRight, qMax(0, content.width())));

    QFont metadataFont = option.font;
    painter->setFont(metadataFont);
    QColor muted = text;
    muted.setAlpha(selected ? 230 : 180);
    painter->setPen(muted);
    QStringList metadata{index.data(Qt::UserRole).toString().left(10)};
    const QString author = index.data(AuthorRole).toString();
    const QString date = index.data(DateRole).toString();
    if (!author.isEmpty())
        metadata.append(author);
    if (!date.isEmpty())
        metadata.append(date.left(10));
    const QRect metadataRect(content.left(), content.top() + subjectMetrics.height() + 4,
        content.width(), option.fontMetrics.height());
    painter->drawText(metadataRect, Qt::AlignVCenter,
        option.fontMetrics.elidedText(metadata.join(" · "), Qt::ElideRight, qMax(0, content.width())));
    painter->restore();
}
