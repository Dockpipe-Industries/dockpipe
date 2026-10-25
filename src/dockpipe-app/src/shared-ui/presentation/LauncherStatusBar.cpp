#include "LauncherStatusBar.h"

LauncherStatusBar::LauncherStatusBar(QWidget *parent) : QStatusBar(parent)
{
    setSizeGripEnabled(false);
    connect(this, &QStatusBar::messageChanged, this, [this]() { updateVisibility(); });
    updateVisibility();
}

void LauncherStatusBar::setNoticeVisible(bool visible)
{
    m_noticeVisible = visible;
    updateVisibility();
}

void LauncherStatusBar::updateVisibility()
{
    setVisible(m_noticeVisible || !currentMessage().isEmpty());
}
