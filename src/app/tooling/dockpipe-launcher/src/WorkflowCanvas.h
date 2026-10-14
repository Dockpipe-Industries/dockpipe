#pragma once

#include <QGraphicsView>
#include <QJsonArray>
#include <QJsonObject>

class WorkflowCanvas : public QGraphicsView {
    Q_OBJECT
public:
    explicit WorkflowCanvas(QWidget *parent = nullptr);
    void setDocument(const QJsonObject &document);
    QJsonObject positions() const;
    void restorePositions(const QJsonObject &positions);
    void fitDocument();
    void zoomBy(qreal factor);
    void selectPath(const QJsonArray &path);

signals:
    void zoomChanged(int percent);
    void nodeSelected(const QJsonArray &path);

protected:
    void drawBackground(QPainter *painter, const QRectF &rect) override;
    void drawForeground(QPainter *painter, const QRectF &rect) override;
    void wheelEvent(QWheelEvent *event) override;

private:
    void updateConnections();
    QVector<QGraphicsItem *> m_roots;
    QVector<QGraphicsPathItem *> m_connections;
};
