#include "LauncherDiagnostics.h"

#include <QCoreApplication>
#include <QLoggingCategory>
#include <QStringList>
#include <cstdio>

static QStringList received;

static void recordMessage(QtMsgType, const QMessageLogContext &, const QString &message)
{
    received.append(message);
}

int main(int argc, char **argv)
{
    QCoreApplication app(argc, argv);
    const bool native = app.arguments().contains(QStringLiteral("--native"));
    const bool diagnostics = app.arguments().contains(QStringLiteral("--diagnostics"));
    if (native)
        qunsetenv("FLATPAK_ID");
    else
        qputenv("FLATPAK_ID", "com.dockpipe.Dockpipe");
    qputenv("DOCKPIPE_QT_DIAGNOSTICS", diagnostics ? "1" : "0");
    qInstallMessageHandler(recordMessage);
    installLauncherDiagnostics();
    installLauncherDiagnostics();

    const char *category = "qt.qpa.wayland.textinput";
    const QString benign = QStringLiteral(
        "virtual void QtWaylandClient::QWaylandTextInputv3::zwp_text_input_v3_leave(wl_surface*) "
        "Got leave event for surface 0x0 with focusing surface 0x556e893ffe00");
    QMessageLogger(nullptr, 0, nullptr, category).warning("%s", qPrintable(benign));
    bool filtered = false;
#if defined(Q_OS_LINUX)
    filtered = !native && !diagnostics;
#endif
    const QStringList expected = filtered ? QStringList{} : QStringList{benign};
    if (received != expected)
        return 1;
    received.clear();

    const QString genuineMismatch = QString(benign).replace(QStringLiteral("surface 0x0 with"),
                                                            QStringLiteral("surface 0x123 with"));
    QMessageLogger(nullptr, 0, nullptr, category).warning("%s", qPrintable(genuineMismatch));
    QMessageLogger(nullptr, 0, nullptr, category).critical("%s", qPrintable(benign));
    QMessageLogger(nullptr, 0, nullptr, "other.category").warning("%s", qPrintable(benign));
    QMessageLogger(nullptr, 0, nullptr, category).warning("Input method failed");
    if (received != QStringList{genuineMismatch, benign, benign, QStringLiteral("Input method failed")})
        return 2;
    std::puts("PASS: exact benign warning handled; genuine warnings, critical messages and diagnostics preserved");
    return 0;
}
