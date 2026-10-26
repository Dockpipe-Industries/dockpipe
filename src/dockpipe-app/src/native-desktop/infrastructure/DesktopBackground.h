#pragma once

#include <QByteArray>
#include <QString>

struct DesktopBackgroundResult {
    QString path;
    QString error;
};

// Read-only, one-shot resolution. Run on a worker; the caller owns image loading
// and refreshing. No screenshots, network fetches or desktop settings changes.
class DesktopBackground final {
public:
    static DesktopBackgroundResult resolve();
    // Explicit sources keep platform parsing and command failure tests isolated
    // from the user's real desktop. Production callers use resolve().
    static DesktopBackgroundResult resolveLinux(const QString &desktop,
        const QString &settingsProgram, const QString &plasmaConfigFile);
    static DesktopBackgroundResult fromGSettings(const QByteArray &configuration, bool preferDark);
    static DesktopBackgroundResult fromPlasmaConfig(const QByteArray &configuration);
    static DesktopBackgroundResult localImage(const QString &pathOrUrl);
};
