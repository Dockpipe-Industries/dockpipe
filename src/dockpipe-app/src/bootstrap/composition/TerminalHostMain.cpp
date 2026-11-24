#include "NativeTerminalProvider.h"

#include <QCoreApplication>

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    // This is deliberately independent of the GUI startup, settings, browser,
    // plugin loading and single-instance process. The private protocol has one
    // supported entrypoint and never accepts a caller-selected provider path.
    qInstallMessageHandler([](QtMsgType, const QMessageLogContext &, const QString &) {});
    if (application.arguments().size() != 2 || application.arguments().at(1) != QStringLiteral("--stdio"))
        return 64;
    NativeTerminalProvider provider;
    provider.start();
    return application.exec();
}
