#include "LauncherEnvironment.h"

#include <QStringList>

void extendMacOSExecutablePath()
{
    const QString originalPath = qEnvironmentVariable("PATH");
    QStringList paths;
    if (!originalPath.isEmpty())
        paths = originalPath.split(QLatin1Char(':'));
    const QStringList fallbacks = {
        QStringLiteral("/opt/homebrew/bin"),
        QStringLiteral("/usr/local/bin"),
        QStringLiteral("/usr/bin"),
        QStringLiteral("/bin"),
        QStringLiteral("/usr/sbin"),
        QStringLiteral("/sbin"),
    };
    for (const QString &path : fallbacks) {
        if (!paths.contains(path))
            paths.append(path);
    }
    qputenv("PATH", paths.join(QLatin1Char(':')).toLocal8Bit());
}
