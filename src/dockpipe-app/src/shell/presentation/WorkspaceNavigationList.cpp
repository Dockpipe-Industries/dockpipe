#include "WorkspaceNavigationList.h"

#include <QTimer>

WorkspaceNavigationList::WorkspaceNavigationList(QWidget *parent)
    : QListWidget(parent)
{
}

void WorkspaceNavigationList::ensureCurrentItemVisible()
{
    if (m_visibilityUpdateQueued)
        return;
    m_visibilityUpdateQueued = true;
    // Wait for the viewport and scroll range to reflect the completed layout.
    QTimer::singleShot(0, this, [this]() {
        m_visibilityUpdateQueued = false;
        if (currentItem() && !currentItem()->isHidden())
            scrollToItem(currentItem(), QAbstractItemView::EnsureVisible);
    });
}

void WorkspaceNavigationList::resizeEvent(QResizeEvent *event)
{
    QListWidget::resizeEvent(event);
    ensureCurrentItemVisible();
}

void WorkspaceNavigationList::showEvent(QShowEvent *event)
{
    QListWidget::showEvent(event);
    ensureCurrentItemVisible();
}
