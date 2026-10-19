#pragma once

#include <QFrame>

class CanvasPanel : public QFrame {
    Q_OBJECT
public:
    explicit CanvasPanel(const QString &identity, QWidget *parent = nullptr, bool persistent = true);
    void setDragHandle(QWidget *handle);
    void placeDefault(const QRect &geometry);
    void resetPlacement();
    void setInteractionEnabled(bool enabled);

signals:
    void placementChanged();
    void activated();

protected:
    bool eventFilter(QObject *object, QEvent *event) override;
    void paintEvent(QPaintEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    QSize minimumPanelSize(const QSize &available) const;
    QRect constrained(const QRect &geometry) const;
    void finishInteraction();
    bool m_persistent = true;
    bool m_interactionEnabled = true;
    QString m_settingsKey;
    QWidget *m_handle = nullptr;
    QRect m_startGeometry;
    QRect m_preferredGeometry;
    QPoint m_startPointer;
    bool m_customPlacement = false;
    bool m_dragging = false;
    Qt::Edges m_resizeEdges;
};
