#include "BuildVersion.h"
#include "DockpipeChoices.h"
#include "FlatpakTerminal.h"
#include "LauncherEnvironment.h"
#include "LauncherDiagnostics.h"
#include "LauncherTranslations.h"
#include "MainWindow.h"
#include "SingleInstanceGuard.h"
#include "Theme.h"
#include "WebMermaidRenderer.h"
#include "TerminalPreferences.h"
#ifdef DOCKPIPE_ENABLE_QUALIFICATION
#include "PackageDisplayQualification.h"
#endif

#include <QApplication>
#include <QDir>
#include <QIcon>
#include <QMessageBox>
#include <QProcess>
#include <QProcessEnvironment>
#include <QSize>
#include <QStatusBar>
#include <QFutureWatcher>
#include <QtConcurrent/QtConcurrentRun>
#include <QStyleFactory>
#include <QTextStream>
#include <cstring>

static bool allowSecondInstance(int argc, char *argv[])
{
    for (int i = 1; i < argc; ++i) {
        if (std::strcmp(argv[i], "--allow-second-instance") == 0)
            return true;
    }
    return false;
}

static QIcon dockpipeLauncherIcon()
{
    QIcon icon;
    const struct {
        const char *path;
        int size;
    } sizes[] = {
        {":/icons/hicolor/16x16/apps/dockpipe-launcher.png", 16},
        {":/icons/hicolor/24x24/apps/dockpipe-launcher.png", 24},
        {":/icons/hicolor/32x32/apps/dockpipe-launcher.png", 32},
        {":/icons/hicolor/48x48/apps/dockpipe-launcher.png", 48},
        {":/icons/hicolor/64x64/apps/dockpipe-launcher.png", 64},
        {":/icons/hicolor/96x96/apps/dockpipe-launcher.png", 96},
        {":/icons/hicolor/128x128/apps/dockpipe-launcher.png", 128},
        {":/icons/hicolor/256x256/apps/dockpipe-launcher.png", 256},
        {":/icons/hicolor/512x512/apps/dockpipe-launcher.png", 512},
    };
    for (const auto &entry : sizes)
        icon.addFile(QString::fromUtf8(entry.path), QSize(entry.size, entry.size));
    if (icon.isNull())
        icon = QIcon(QStringLiteral(":/icon.png"));
    return icon;
}

static int runDesktop(QApplication &app, bool startHome, bool allowSecond, bool &restart)
{
    SingleInstanceGuard singleGuard;
    if (!allowSecond && !singleGuard.tryRunPrimaryInstance(startHome))
        return 0;

    LauncherTranslations::initialize(app);
    MainWindow window;
    if (!allowSecond)
        singleGuard.setActivationTarget(&window);
    QObject::connect(&window, &MainWindow::restartRequested, &app, [&app, &restart]() {
        restart = true;
        // The normal unsaved-work checks have already succeeded. quit() would
        // send closeEvent again, allowing the tray to hide instead of restarting.
        app.exit(0);
    });
    window.show();
    if (!qEnvironmentVariableIsEmpty("FLATPAK_ID")) {
        auto *terminalSetup = new QFutureWatcher<FlatpakTerminal::Result>(&window);
        QObject::connect(terminalSetup, &QFutureWatcher<FlatpakTerminal::Result>::finished,
            &window, [&window, terminalSetup]() {
                const auto result = terminalSetup->result();
                if (!result.success) {
                    window.statusBar()->showMessage(result.message);
                    QTextStream(stderr) << result.message << '\n';
                }
                terminalSetup->deleteLater();
            });
        terminalSetup->setFuture(QtConcurrent::run(&FlatpakTerminal::setupForCurrentUser));
    }
    return app.exec();
}

int main(int argc, char *argv[])
{
    for (int i = 1; i < argc; ++i) {
        if (std::strcmp(argv[i], "--version") == 0) {
            QTextStream(stdout) << DOCKPIPE_RELEASE_VERSION << '\n';
            return 0;
        }
    }
#if defined(Q_OS_MACOS)
    extendMacOSExecutablePath();
#endif
    const QProcessEnvironment startupEnv = QProcessEnvironment::systemEnvironment();
#if defined(Q_OS_LINUX)
    if (startupEnv.value(QStringLiteral("XDG_SESSION_TYPE")).compare(QStringLiteral("x11"), Qt::CaseInsensitive) == 0) {
        qunsetenv("DESKTOP_STARTUP_ID");
        qunsetenv("XDG_ACTIVATION_TOKEN");
    }
#endif

    installLauncherDiagnostics();
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication app(argc, argv);
    // Keep the persisted config/cache identity compatible with existing installations.
    QApplication::setApplicationName(QStringLiteral("dockpipe-launcher"));
    QApplication::setApplicationVersion(QStringLiteral(DOCKPIPE_RELEASE_VERSION));
    QApplication::setApplicationDisplayName(QStringLiteral("Dockpipe"));
    QApplication::setOrganizationName(QStringLiteral("dockpipe"));
#ifdef DOCKPIPE_ENABLE_QUALIFICATION
    int qualificationExit = -1;
    const bool qualification = preparePackageDisplayQualification(app, qualificationExit);
    if (qualification && qualificationExit >= 0)
        return qualificationExit;
#else
    if (app.arguments().contains("--qualify-package-display")
        || app.arguments().contains("--qualification-profile-path")) {
        QTextStream(stderr) << "This build does not enable desktop qualification.\n";
        return 2;
    }
#endif
    // Transparent WebEngine surfaces otherwise bypass normal widget stacking.
    // Select the compatible compositor before constructing any browser views.
    if (TerminalPreferences::instance()->settings().opacity < 100
        && qEnvironmentVariableIsEmpty("QT_QUICK_BACKEND"))
        qputenv("QT_QUICK_BACKEND", "software");
    app.setDesktopFileName(qEnvironmentVariableIsSet("FLATPAK_ID")
        ? qEnvironmentVariable("FLATPAK_ID") : QStringLiteral("dockpipe-launcher"));
    app.setWindowIcon(dockpipeLauncherIcon());

    if (app.arguments().contains(QStringLiteral("--check-installation"))) {
        // Exercise the same CLI selection as a desktop launch, including bundled
        // wrappers, without opening a window or modifying contexts.
        const QString binary = DockpipeChoices::preferredDockpipeBinary(QString());
        QProcess cli;
        cli.start(binary, {QStringLiteral("--version")});
        if (!cli.waitForStarted() || !cli.waitForFinished(30000)) {
            cli.kill();
            cli.waitForFinished();
            QTextStream(stderr) << "Cannot run Dockpipe CLI: " << binary << '\n';
            return 1;
        }
        QTextStream(stdout) << "CLI: " << binary << '\n' << cli.readAllStandardOutput();
        QTextStream(stderr) << cli.readAllStandardError();
        return cli.exitStatus() == QProcess::NormalExit ? cli.exitCode() : 1;
    }

    app.setStyle(QStyleFactory::create(QStringLiteral("Fusion")));

    applyDockpipeLauncherTheme(app);
    connectDockpipeLauncherThemeUpdates(app);
    installWebMermaidRenderer();

    app.setQuitOnLastWindowClosed(true);

#ifdef DOCKPIPE_ENABLE_QUALIFICATION
    if (qualification)
        return runPackageDisplayQualification(app);
#endif
    const bool startHome = app.arguments().contains(QStringLiteral("--start-home"));
    const bool allowSecond = allowSecondInstance(argc, argv);
    bool restart = false;
    const int exitCode = runDesktop(app, startHome, allowSecond, restart);
    if (!restart)
        return exitCode;

    // The main window, child sessions and single-instance guard are gone before
    // the replacement starts, so it cannot activate the instance being shut down.
    QProcess replacement;
    replacement.setProgram(app.applicationFilePath());
    replacement.setArguments(app.arguments().mid(1));
    replacement.setWorkingDirectory(QDir::currentPath());
    replacement.setProcessEnvironment(launcherRestartEnvironment(startupEnv));
    if (replacement.startDetached())
        return 0;
    QMessageBox::critical(nullptr,
        QCoreApplication::translate("LauncherRestart", "Could not restart Dockpipe"),
        QCoreApplication::translate("LauncherRestart", "Your language is saved. Reopen Dockpipe manually to apply it."));
    return 1;
}
