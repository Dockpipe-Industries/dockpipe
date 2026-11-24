#pragma once

#include <QString>
#include <QProcessEnvironment>

// Finder does not inherit shell startup files. Keep explicit PATH entries first.
void extendMacOSExecutablePath();

// Keep an inherited global root authoritative while allowing saved defaults to change.
void applyGlobalRootDefault(const QString &savedRoot);

// Preserve operator overrides, without promoting app-injected defaults to overrides.
QProcessEnvironment launcherRestartEnvironment(const QProcessEnvironment &startupEnvironment);
