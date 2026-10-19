#pragma once

#include <QToolButton>

// Shared, original DockPipe artwork for window and embedded-panel controls.
// The caller owns the action; this widget only renders its accessible control.
class WindowControlButton : public QToolButton {
public:
    enum Action { Minimize, Maximize, Close };
    explicit WindowControlButton(Action action, QWidget *parent = nullptr);
    void setExpanded(bool expanded);

protected:
    void paintEvent(QPaintEvent *event) override;

private:
    Action m_action;
    bool m_expanded = false;
};
