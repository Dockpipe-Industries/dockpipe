#pragma once

#include <QGraphicsView>
#include <QGraphicsScene>
#include <QPainter>
#include <QWheelEvent>
#include <cmath>

// Shared native pan/zoom/grid surface. No timers or simulation.
class CanvasView : public QGraphicsView {
public:
    explicit CanvasView(QWidget *parent = nullptr) : QGraphicsView(parent)
    {
        setScene(new QGraphicsScene(this));
        setRenderHints(QPainter::Antialiasing | QPainter::TextAntialiasing);
        setFrameShape(QFrame::NoFrame);
        setDragMode(ScrollHandDrag);
        setTransformationAnchor(AnchorUnderMouse);
        setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
        setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    }
protected:
    void drawBackground(QPainter *painter, const QRectF &rect) override
    {
        const bool dark = palette().color(QPalette::Window).lightness() < 128;
        painter->fillRect(rect, QColor(dark ? "#10151c" : "#f6f8fc"));
        const int spacing = transform().m11() < 0.4 ? 128 : 32;
        painter->setPen(QPen(QColor(dark ? "#242d39" : "#dfe6f0"), 0));
        for (qreal x = std::floor(rect.left() / spacing) * spacing; x < rect.right(); x += spacing)
            painter->drawLine(QPointF(x, rect.top()), QPointF(x, rect.bottom()));
        for (qreal y = std::floor(rect.top() / spacing) * spacing; y < rect.bottom(); y += spacing)
            painter->drawLine(QPointF(rect.left(), y), QPointF(rect.right(), y));
    }
    void wheelEvent(QWheelEvent *event) override
    {
        const qreal next = qBound(0.15, transform().m11() * std::pow(1.0015, event->angleDelta().y()), 2.5);
        scale(next / transform().m11(), next / transform().m11());
        event->accept();
    }
};

