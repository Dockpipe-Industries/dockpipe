#pragma once

// Keep genuine Qt diagnostics while hiding one known, benign Flatpak/Wayland
// surface teardown warning. DOCKPIPE_QT_DIAGNOSTICS=1 restores that warning.
void installLauncherDiagnostics();
