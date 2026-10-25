#pragma once

#include <QGraphicsPathItem>

// A decorative direction cue over a readable relationship line, not traffic telemetry.
class ResourceLinkItem final : public QGraphicsPathItem {
public:
    QRectF boundingRect() const override;
    void setFlowPath(const QPainterPath &path);
    void setFlowColor(const QColor &color);
    void setPhase(qreal phase);
    void setMotionEnabled(bool enabled);
    bool motionEnabled() const { return m_motionEnabled; }
    void paint(QPainter *painter, const QStyleOptionGraphicsItem *option, QWidget *widget) override;

private:
    QPainterPath m_flowPath;
    QColor m_flowColor;
    qreal m_phase = 0;
    bool m_motionEnabled = false;
};
