#include "LauncherDiagnostics.h"

#include <QByteArray>
#include <QLoggingCategory>
#include <QRegularExpression>
#include <QString>
#include <cstdio>
#include <cstring>

namespace {

QtMessageHandler previousHandler = nullptr;

void launcherMessageHandler(QtMsgType type, const QMessageLogContext &context, const QString &message)
{
    // Qt's newer Wayland code recognizes a null wl_surface as normal teardown.
    // Match only the older diagnostic for that case, not other focus/IME errors.
    static const QRegularExpression destroyedSurface(
        QStringLiteral("^(?:virtual )?void QtWaylandClient::QWaylandTextInputv3::"
                       "zwp_text_input_v3_leave\\(wl_surface\\*\\) Got leave event for surface "
                       "0x0 with focusing surface 0x[0-9a-fA-F]+$"));
    if (type == QtWarningMsg && context.category
        && std::strcmp(context.category, "qt.qpa.wayland.textinput") == 0
        && destroyedSurface.match(message).hasMatch())
        return;

    if (previousHandler) {
        previousHandler(type, context, message);
    } else {
        const QByteArray formatted = qFormatLogMessage(type, context, message).toLocal8Bit();
        std::fprintf(stderr, "%s\n", formatted.constData());
        std::fflush(stderr);
    }
}

} // namespace

void installLauncherDiagnostics()
{
#if defined(Q_OS_LINUX)
    static bool installed = false;
    if (!installed && !qEnvironmentVariableIsEmpty("FLATPAK_ID")
        && qEnvironmentVariable("DOCKPIPE_QT_DIAGNOSTICS") != QStringLiteral("1")) {
        previousHandler = qInstallMessageHandler(launcherMessageHandler);
        installed = true;
    }
#endif
}
