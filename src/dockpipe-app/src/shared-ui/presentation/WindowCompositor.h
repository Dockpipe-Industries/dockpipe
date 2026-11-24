#pragma once

class QWidget;

// Call before showing a window that will later host accelerated widget content.
void prepareLauncherWindowCompositor(QWidget &window);
