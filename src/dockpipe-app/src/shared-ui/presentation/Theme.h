#pragma once

class QApplication;

enum class LauncherThemeMode { System, Light, Dark };

LauncherThemeMode dockpipeLauncherThemeMode();
bool saveDockpipeLauncherThemeMode(LauncherThemeMode mode);
void previewDockpipeLauncherTheme(QApplication &app, LauncherThemeMode mode);
void resetDockpipeLauncherThemePreview(QApplication &app);

/// Applies the saved appearance preference, or the active Settings preview.
void applyDockpipeLauncherTheme(QApplication &app);

/// Re-applies when the platform color scheme changes (Qt 6.5+).
void connectDockpipeLauncherThemeUpdates(QApplication &app);
