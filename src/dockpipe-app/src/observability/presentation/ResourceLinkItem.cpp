#include "ResourceLinkItem.h"

#include <QPainter>
#include <QStyleOptionGraphicsItem>

QRectF ResourceLinkItem::boundingRect() const
{
    // Include the constant-pixel highlight at the lowest animated zoom level.
    return QGraphicsPathItem::boundingRect().adjusted(-3, -3, 3, 3);
}

void ResourceLinkItem::setFlowPath(const QPainterPath &path)
{
    m_flowPath = path;
    update();
}

void ResourceLinkItem::setFlowColor(const QColor &color)
{
    m_flowColor = color;
    update();
}

void ResourceLinkItem::setPhase(qreal phase)
{
    m_phase = phase;
    update();
}

void ResourceLinkItem::setMotionEnabled(bool enabled)
{
    m_motionEnabled = enabled;
    update();
}

void ResourceLinkItem::paint(QPainter *painter, const QStyleOptionGraphicsItem *option, QWidget *widget)
{
    QGraphicsPathItem::paint(painter, option, widget);
    if (!m_motionEnabled)
        return;
    const qreal zoom = QStyleOptionGraphicsItem::levelOfDetailFromTransform(painter->worldTransform());
    // Fade between overview and editing scales to avoid subpixel shimmer.
    const qreal strength = qBound(0.0, (zoom - 0.35) / 0.5, 1.0);
    if (strength <= 0)
        return;
    painter->save();
    painter->setOpacity(painter->opacity() * strength);
    QPen flow(m_flowColor, pen().widthF(), Qt::CustomDashLine, Qt::RoundCap);
    flow.setCosmetic(true);
    flow.setDashPattern({3.0, 15.0});
    flow.setDashOffset(-m_phase);
    painter->setPen(flow);
    painter->setBrush(Qt::NoBrush);
    painter->drawPath(m_flowPath);
    painter->restore();
}
