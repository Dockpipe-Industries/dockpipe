#pragma once
#include <QFrame>
#include <QRectF>

class QLabel;
class QPushButton;
class QVBoxLayout;

// A bounded, movable windshield panel. Positions are normalized to its viewport.
class CockpitHudPanel final : public QFrame {
    Q_OBJECT
public:
    CockpitHudPanel(const QString &title, QWidget *parent);
    QVBoxLayout *body() const { return m_body; }
    void place(const QRectF &normalized);
    QRectF placement() const;
    void fitToViewport();
    void setPlacementBounds(const QRect &bounds) { m_placementBounds = bounds; }
    void setArrangementEnabled(bool enabled);
    void setClosable(bool enabled);
    void setTitle(const QString &title);
    QPushButton *closeButton() const { return m_close; }
    QPushButton *fullscreenButton() const { return m_fullscreen; }
signals:
    void closeRequested();
    void fullscreenRequested();
    void placementChanged();
protected:
    bool eventFilter(QObject *object, QEvent *event) override;
private:
    QRect placementBounds() const;
    QRect m_placementBounds;
    QLabel *m_title;
    QWidget *m_grip;
    QPushButton *m_close;
    QPushButton *m_fullscreen;
    QVBoxLayout *m_body;
    QPoint m_dragOffset;
    bool m_arrange = false;
    bool m_dragging = false;
    bool m_resizing = false;
    QPoint m_resizeOrigin;
    QSize m_resizeStart;
};
