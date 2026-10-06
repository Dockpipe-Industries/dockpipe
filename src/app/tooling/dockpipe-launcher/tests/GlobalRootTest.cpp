#include "LauncherEnvironment.h"
#include <QCoreApplication>

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    if (app.arguments().contains(QStringLiteral("--inherited"))) {
        qputenv("DOCKPIPE_GLOBAL_ROOT", "/explicit/root");
        applyGlobalRootDefault(QStringLiteral("/saved/root"));
        applyGlobalRootDefault(QString());
        return qEnvironmentVariable("DOCKPIPE_GLOBAL_ROOT") == QStringLiteral("/explicit/root") ? 0 : 1;
    }
    qunsetenv("DOCKPIPE_GLOBAL_ROOT");
    applyGlobalRootDefault(QStringLiteral("/saved/root"));
    if (qEnvironmentVariable("DOCKPIPE_GLOBAL_ROOT") != QStringLiteral("/saved/root"))
        return 1;
    applyGlobalRootDefault(QStringLiteral("/new/root"));
    if (qEnvironmentVariable("DOCKPIPE_GLOBAL_ROOT") != QStringLiteral("/new/root"))
        return 1;
    applyGlobalRootDefault(QString());
    return qEnvironmentVariableIsSet("DOCKPIPE_GLOBAL_ROOT") ? 1 : 0;
}
