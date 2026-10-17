#pragma once

#include "CanvasView.h"
#include <QMouseEvent>
#include <QVector3D>
#include <functional>

// Orthographic 3D camera over the existing native scene. Only input or data
// changes repaint it; no rendering loop, physics or extra model.
class WorkspaceGraphView : public CanvasView {
public:
    explicit WorkspaceGraphView(QWidget *parent = nullptr);
    void setHardwareAcceleration(bool enabled, bool deferUntilShown = false);
    bool hardwareAccelerationRequested() const { return m_hardwareRequested; }
    bool hardwareAccelerated() const { return m_hardwareActive; }
    QString rendererDescription() const { return m_rendererDescription; }

    bool spatial() const { return m_spatial; }
    void setSpatial(bool enabled)
    {
        m_spatial = enabled;
        updateRenderer();
        m_orbitButton = Qt::NoButton;
        viewport()->unsetCursor();
        viewport()->update();
    }
    void orbit(qreal yawDelta, qreal pitchDelta)
    {
        m_yaw = std::remainder(m_yaw + yawDelta, 360.0);
        m_pitch = qBound(15.0, m_pitch + pitchDelta, 75.0);
        updateCameraBasis();
        if (cameraChanged)
            cameraChanged();
        viewport()->update();
    }
    void resetCamera()
    {
        m_yaw = -20;
        m_pitch = 50;
        orbit(0, 0);
    }
    QPointF project(const QVector3D &point) const
    {
        const qreal horizontal = point.x() * m_cosYaw - point.z() * m_sinYaw;
        const qreal forward = point.x() * m_sinYaw + point.z() * m_cosYaw;
        return {horizontal, forward * m_sinPitch - point.y() * m_cosPitch};
    }
    qreal depth(const QVector3D &point) const
    {
        return (point.x() * m_sinYaw + point.z() * m_cosYaw) * m_cosPitch + point.y() * m_sinPitch;
    }
    void setGridExtent(qreal extent) { m_gridExtent = qBound(900.0, extent, 24000.0); }
    std::function<void()> cameraChanged;
    std::function<void()> rendererChanged;
    std::function<void(QPainter *)> foregroundPainter;

protected:
    void showEvent(QShowEvent *event) override;
    void drawForeground(QPainter *painter, const QRectF &) override
    {
        if (foregroundPainter)
            foregroundPainter(painter);
    }
    void drawBackground(QPainter *painter, const QRectF &rect) override
    {
        if (!m_spatial) {
            CanvasView::drawBackground(painter, rect);
            return;
        }
        const bool dark = palette().color(QPalette::Window).lightness() < 128;
        painter->fillRect(rect, QColor(dark ? "#10151c" : "#f6f8fc"));
        // Fixed 41 lines per axis, independent of viewport zoom or node count.
        const qreal spacing = m_gridExtent / 20;
        for (int line = -20; line <= 20; ++line) {
            const qreal coordinate = line * spacing;
            painter->setPen(QPen(QColor(dark ? "#293846" : "#d5e0ea"), 0));
            painter->drawLine(project(QVector3D(coordinate, -100, -m_gridExtent)),
                              project(QVector3D(coordinate, -100, m_gridExtent)));
            painter->drawLine(project(QVector3D(-m_gridExtent, -100, coordinate)),
                              project(QVector3D(m_gridExtent, -100, coordinate)));
        }
        painter->setPen(QPen(QColor(dark ? "#548786" : "#6c9b99"), 0));
        painter->drawLine(project(QVector3D(-m_gridExtent, -100, 0)),
                          project(QVector3D(m_gridExtent, -100, 0)));
        painter->drawLine(project(QVector3D(0, -100, -m_gridExtent)),
                          project(QVector3D(0, -100, m_gridExtent)));
    }
    void mousePressEvent(QMouseEvent *event) override
    {
        const bool middleDrag = event->button() == Qt::MiddleButton;
        const bool shiftDrag = event->button() == Qt::LeftButton && (event->modifiers() & Qt::ShiftModifier);
        if (m_spatial && (middleDrag || shiftDrag)) {
            m_orbitButton = event->button();
            m_lastMouse = event->pos();
            viewport()->setCursor(Qt::ClosedHandCursor);
            event->accept();
            return;
        }
        CanvasView::mousePressEvent(event);
    }
    void mouseMoveEvent(QMouseEvent *event) override
    {
        if (m_orbitButton != Qt::NoButton && !(event->buttons() & m_orbitButton)) {
            m_orbitButton = Qt::NoButton;
            viewport()->unsetCursor();
        }
        if (m_orbitButton != Qt::NoButton) {
            const QPoint delta = event->pos() - m_lastMouse;
            m_lastMouse = event->pos();
            orbit(delta.x() * 0.35, delta.y() * 0.25);
            event->accept();
            return;
        }
        CanvasView::mouseMoveEvent(event);
    }
    void mouseReleaseEvent(QMouseEvent *event) override
    {
        if (m_orbitButton != Qt::NoButton && event->button() == m_orbitButton) {
            m_orbitButton = Qt::NoButton;
            viewport()->unsetCursor();
            event->accept();
            return;
        }
        CanvasView::mouseReleaseEvent(event);
    }

private:
    void updateRenderer();
    void useSoftware(const QString &reason);
    void updateCameraBasis()
    {
        // One trigonometric evaluation per camera change, not per chip vertex.
        m_cosYaw = std::cos(m_yaw * radians);
        m_sinYaw = std::sin(m_yaw * radians);
        m_cosPitch = std::cos(m_pitch * radians);
        m_sinPitch = std::sin(m_pitch * radians);
    }
    static constexpr qreal radians = 3.14159265358979323846 / 180;
    bool m_spatial = false;
    bool m_hardwareRequested = true;
    bool m_hardwareActive = false;
    bool m_hardwareFailed = false;
    QString m_rendererDescription;
    Qt::MouseButton m_orbitButton = Qt::NoButton;
    QPoint m_lastMouse;
    qreal m_yaw = -20;
    qreal m_pitch = 50;
    qreal m_cosYaw = 1;
    qreal m_sinYaw = 0;
    qreal m_cosPitch = 1;
    qreal m_sinPitch = 0;
    qreal m_gridExtent = 1200;
};
