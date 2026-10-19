#pragma once

#include <QTabBar>

// Reordering remains native QTabBar behavior. Releasing a dragged tab beyond
// the row offers the live tab to the owning dashboard instead of navigating it.
class BrowserTabBar final : public QTabBar
{
    Q_OBJECT
public:
    explicit BrowserTabBar(QWidget *parent = nullptr);
    void setTearOutEnabled(bool enabled);

signals:
    void tearOutRequested(int index, const QPoint &globalPosition);

protected:
    void mousePressEvent(QMouseEvent *event) override;
    void mouseReleaseEvent(QMouseEvent *event) override;
    void keyPressEvent(QKeyEvent *event) override;
    void hideEvent(QHideEvent *event) override;

private:
    QPoint m_pressPosition;
    bool m_dragCandidate = false;
    bool m_tearOutEnabled = false;
};
