#include "CanvasIcons.h"
#include "LauncherSettings.h"

#include <SpatialWeb/Browser/BrowserHostServices.h>
#include <QCoreApplication>

namespace {
QIcon browserIcon(BrowserHostServices::Icon icon)
{
    using Icon = BrowserHostServices::Icon;
    using Symbol = CanvasIcons::Symbol;
    switch (icon) {
    case Icon::Add:
        return CanvasIcons::icon(Symbol::Add);
    case Icon::Minus:
        return CanvasIcons::icon(Symbol::Minus);
    case Icon::Preview:
        return CanvasIcons::icon(Symbol::Preview);
    case Icon::Back:
        return CanvasIcons::icon(Symbol::Back);
    case Icon::Forward:
        return CanvasIcons::icon(Symbol::Forward);
    case Icon::Refresh:
        return CanvasIcons::icon(Symbol::Refresh);
    case Icon::Close:
        return CanvasIcons::icon(Symbol::Close);
    case Icon::Rename:
        return CanvasIcons::icon(Symbol::Rename);
    case Icon::Properties:
        return CanvasIcons::icon(Symbol::Properties);
    case Icon::Extensions:
        return CanvasIcons::icon(Symbol::Extensions);
    case Icon::Download:
        return CanvasIcons::icon(Symbol::Download);
    case Icon::More:
        return CanvasIcons::icon(Symbol::More);
    case Icon::Globe:
        return CanvasIcons::icon(Symbol::Globe);
    case Icon::ChevronDown:
        return CanvasIcons::icon(Symbol::ChevronDown);
    case Icon::Check:
        return CanvasIcons::icon(Symbol::Check);
    }
    return {};
}

void configureBrowserHostServices()
{
    BrowserHostServices::Adapter adapter;
    adapter.icon = browserIcon;
    adapter.loadPreferences = LauncherSettings::loadBrowserSettings;
    adapter.savePreferences = LauncherSettings::saveBrowserSettings;
    QString error;
    if (!BrowserHostServices::configure(std::move(adapter), &error))
        qFatal("Cannot configure the native browser integration: %s", qPrintable(error));
}
}

// Register the host integration before any lazy browser widget or preference
// reader is created. Existing settings retain their original storage owner.
Q_COREAPP_STARTUP_FUNCTION(configureBrowserHostServices)
