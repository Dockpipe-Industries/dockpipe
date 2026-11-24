#include "WindowCompositor.h"

#include <QQuickWidget>
#include <QWidget>

void prepareLauncherWindowCompositor(QWidget &window)
{
    // Qt chooses the native surface from the child hierarchy on first show.
    // An empty, hidden Quick widget declares the same compositor as WebEngine,
    // without loading a web page or starting a renderer or terminal process.
    // Keep it alive: removing the final accelerated child can change the surface.
    auto *compositor = new QQuickWidget(&window);
    compositor->setObjectName("windowCompositor");
    compositor->setFixedSize(0, 0);
    compositor->hide();
}
