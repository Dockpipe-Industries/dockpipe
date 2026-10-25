#pragma once

#include "LauncherUiExport.h"
#include <QStatusBar>

// Reserve a bottom row only while there is something for the user to read.
class DOCKPIPE_LAUNCHER_UI_EXPORT LauncherStatusBar final : public QStatusBar {
public:
    explicit LauncherStatusBar(QWidget *parent = nullptr);
    void setNoticeVisible(bool visible);

private:
    void updateVisibility();
    bool m_noticeVisible = false;
};
