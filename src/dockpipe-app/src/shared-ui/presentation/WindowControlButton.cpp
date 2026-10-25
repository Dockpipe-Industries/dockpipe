#include "WindowControlButton.h"

#include <QPainter>
#include <QVariant>

WindowControlButton::WindowControlButton(Action action, QWidget *parent)
    : QToolButton(parent), m_action(action)
{
    setProperty("roundWindowControl", true);
    setFixedSize(28, 28);
    setAutoRaise(true);
    setFocusPolicy(Qt::StrongFocus);
    setCursor(Qt::ArrowCursor);
    QString label;
    if (action == Close)
        label = tr("Close");
    else if (action == Minimize)
        label = tr("Minimize");
    else
        label = tr("Maximize or restore");
    setAccessibleName(label);
    setToolTip(label);
}

void WindowControlButton::setExpanded(bool expanded)
{
    if (m_expanded == expanded)
        return;
    m_expanded = expanded;
    update();
}

void WindowControlButton::paintEvent(QPaintEvent *)
{
    // Paint only on normal widget invalidation: no animations or idle timers.
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.translate(width() / 2.0, height() / 2.0);
    QColor fill;
    if (m_action == Close)
        fill = QColor("#e88791");
    else if (m_action == Minimize)
        fill = QColor("#e2bd70");
    else
        fill = QColor("#79cbb6");
    if (!isEnabled())
        fill = palette().color(QPalette::Mid);
    else if (isDown())
        fill = fill.darker(118);
    else if (underMouse())
        fill = fill.lighter(110);
    const qreal radius = qMin(8.0, (qMin(width(), height()) - 8.0) / 2.0);
    if (hasFocus()) {
        painter.setPen(QPen(palette().color(QPalette::Highlight), 1.5));
        painter.setBrush(Qt::NoBrush);
        painter.drawEllipse(QPointF(), radius + 3, radius + 3);
    }
    painter.setPen(QPen(fill.darker(125), 0.7));
    painter.setBrush(fill);
    painter.drawEllipse(QPointF(), radius, radius);
    const QColor symbol = isEnabled() ? QColor("#17252c") : palette().color(QPalette::WindowText);
    painter.setPen(QPen(symbol, 1.3, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
    painter.setBrush(Qt::NoBrush);
    if (m_action == Close) {
        painter.drawLine(QPointF(-2.6, -2.6), QPointF(2.6, 2.6));
        painter.drawLine(QPointF(2.6, -2.6), QPointF(-2.6, 2.6));
    } else if (m_action == Minimize) {
        painter.drawLine(QPointF(-3, 0), QPointF(3, 0));
    } else if (m_expanded) {
        painter.drawLine(QPointF(-4, -1), QPointF(-1, -1));
        painter.drawLine(QPointF(-1, -1), QPointF(-1, -4));
        painter.drawLine(QPointF(4, 1), QPointF(1, 1));
        painter.drawLine(QPointF(1, 1), QPointF(1, 4));
    } else {
        painter.drawLine(QPointF(-3, 0), QPointF(-3, 3));
        painter.drawLine(QPointF(-3, 3), QPointF(0, 3));
        painter.drawLine(QPointF(0, -3), QPointF(3, -3));
        painter.drawLine(QPointF(3, -3), QPointF(3, 0));
    }
}
