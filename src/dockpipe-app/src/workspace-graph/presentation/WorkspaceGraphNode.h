#pragma once
#include "LauncherUiExport.h"
#include <QGraphicsObject>
#include <QPolygonF>
#include <functional>

class WorkspaceGraphView;

class DOCKPIPE_LAUNCHER_UI_EXPORT WorkspaceGraphNode final : public QGraphicsObject {
public:
    WorkspaceGraphNode(const QString &path, const QString &kind, qint64 fileBytes = -1);
    QRectF boundingRect() const override;
    void setSummary(const QString &summary);
    void setCityView(const WorkspaceGraphView *view);
    qreal chipHeight() const;
    QRectF cityLabelRect() const { return m_labelRect; }
    void paintPriorityLabel(QPainter *painter, const QRectF &rect);
    QPointF roofCenter() const { return m_faces.isEmpty() ? QPointF(110, 29) : m_faces.last().boundingRect().center(); }
    QPointF hangarCenter() const { return m_door.boundingRect().center(); }
    void setDoorOpening(qreal opening) { m_doorOpening = opening; update(); }
    std::function<void()> activated;
    std::function<void(const QPoint &)> menuRequested;
    std::function<void()> moved;
protected:
    void paint(QPainter *painter, const QStyleOptionGraphicsItem *option, QWidget *widget) override;
    void mouseDoubleClickEvent(QGraphicsSceneMouseEvent *event) override;
    void contextMenuEvent(QGraphicsSceneContextMenuEvent *event) override;
    QVariant itemChange(GraphicsItemChange change, const QVariant &value) override;
private:
    void paintCity(QPainter *painter, bool dark);
    void paintCityLabel(QPainter *painter, const QRectF &rect, bool dark);
    QString m_name;
    QString m_kind;
    QString m_summary;
    qint64 m_fileBytes = -1;
    bool m_city = false;
    QPolygonF m_board;
    QPolygonF m_door;
    qreal m_doorOpening = 0;
    QVector<QPolygonF> m_faces;
    QVector<QLineF> m_traces;
    QVector<QLineF> m_pins;
    QRectF m_labelRect;
    QRectF m_bounds = QRectF(-2, -2, 224, 62);
};
