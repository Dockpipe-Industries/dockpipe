#pragma once

class QApplication;

// Installs shared window controls for launcher windows and application dialogs.
// Native file pickers retain the platform's own window and portal behavior.
void installLauncherWindowChrome(QApplication &application);
