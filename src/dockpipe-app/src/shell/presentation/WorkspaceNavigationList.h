#pragma once

#include <QListWidget>

class WorkspaceNavigationList final : public QListWidget {
public:
    explicit WorkspaceNavigationList(QWidget *parent = nullptr);

    void ensureCurrentItemVisible();

protected:
    void resizeEvent(QResizeEvent *event) override;
    void showEvent(QShowEvent *event) override;

private:
    bool m_visibilityUpdateQueued = false;
};
