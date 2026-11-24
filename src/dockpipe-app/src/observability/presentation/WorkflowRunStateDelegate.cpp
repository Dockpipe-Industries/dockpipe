#include "WorkflowRunStateDelegate.h"

#include <QApplication>
#include <QPainter>
#include <QStyle>

WorkflowRunStateDelegate::WorkflowRunStateDelegate(QObject *parent) : QStyledItemDelegate(parent) {}

QString WorkflowRunStateDelegate::label(const QString &state)
{
    if (state == "running")
        return tr("Running");
    if (state == "succeeded")
        return tr("Succeeded");
    if (state == "failed")
        return tr("Failed");
    if (state == "cancelled")
        return tr("Cancelled");
    if (state == "terminated")
        return tr("Terminated");
    if (state == "unverified")
        return tr("Unverified");
    return state.isEmpty() ? tr("Unavailable") : state;
}

QString WorkflowRunStateDelegate::tone(const QString &state)
{
    if (state == "running" || state == "succeeded")
        return "positive";
    if (state == "failed")
        return "error";
    if (state == "unverified")
        return "warning";
    return "neutral";
}

QSize WorkflowRunStateDelegate::sizeHint(const QStyleOptionViewItem &option, const QModelIndex &index) const
{
    QFont font = option.font;
    font.setPixelSize(11);
    font.setWeight(QFont::DemiBold);
    return {QFontMetrics(font).horizontalAdvance(label(index.data(Qt::UserRole + 1).toString())) + 36, 48};
}

void WorkflowRunStateDelegate::paint(QPainter *painter, const QStyleOptionViewItem &option, const QModelIndex &index) const
{
    QStyleOptionViewItem background = option;
    initStyleOption(&background, index);
    background.text.clear();
    QStyle *style = option.widget ? option.widget->style() : QApplication::style();
    style->drawControl(QStyle::CE_ItemViewItem, &background, painter, option.widget);

    const QString state = index.data(Qt::UserRole + 1).toString();
    const QString kind = tone(state);
    const bool dark = option.palette.color(QPalette::Base).lightness() < 128;
    QColor color(dark ? "#a8b6c8" : "#596b82");
    if (kind == "positive")
        color = QColor(dark ? "#83d4b0" : "#226e4c");
    else if (kind == "error")
        color = QColor(dark ? "#f19ba4" : "#ab3544");
    else if (kind == "warning")
        color = QColor(dark ? "#e9c078" : "#896115");
    painter->save();
    painter->setRenderHint(QPainter::Antialiasing);
    QFont font = option.font;
    font.setPixelSize(11);
    font.setWeight(QFont::DemiBold);
    painter->setFont(font);
    const QString text = label(state);
    const int width = qMin(QFontMetrics(font).horizontalAdvance(text) + 20, option.rect.width() - 16);
    const QRect badge(option.rect.left() + 8, option.rect.center().y() - 12, width, 24);
    QColor fill = color;
    fill.setAlpha(dark ? 32 : 22);
    painter->setPen(Qt::NoPen);
    painter->setBrush(fill);
    painter->drawRoundedRect(badge, 5, 5);
    painter->setPen(color);
    painter->drawText(badge, Qt::AlignCenter, text);
    painter->restore();
}
