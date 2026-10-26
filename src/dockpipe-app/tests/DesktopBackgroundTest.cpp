#include "DesktopBackground.h"

#include <QCoreApplication>
#include <QElapsedTimer>
#include <QFile>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <QUrl>
#include <cstdio>
#ifdef Q_OS_LINUX
#include <cerrno>
#include <signal.h>
#endif

class DesktopBackgroundTest final : public QObject {
    Q_OBJECT
private slots:
    void localOnlyAndStatic();
    void settingsParsing();
    void plasmaParsing();
    void commandFailureBounds();
};

namespace {
QString fixtureFile(const QString &directory, const QString &name)
{
    const QString path = directory + '/' + name;
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly))
        return {};
    file.write("Image bytes are loaded and validated by the canvas, not this resolver.");
    return path;
}

QByteArray settings(const QString &path)
{
    return "org.gnome.desktop.background picture-options 'zoom'\n"
        "org.gnome.desktop.background picture-uri '" + QUrl::fromLocalFile(path).toEncoded() + "'\n";
}
}

void DesktopBackgroundTest::localOnlyAndStatic()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = fixtureFile(directory.path(), "wall paper.png");
    QCOMPARE(DesktopBackground::localImage(path).path, path);
    QCOMPARE(DesktopBackground::localImage(QUrl::fromLocalFile(path).toString()).path, path);
    QCOMPARE(DesktopBackground::localImage(QUrl::fromLocalFile(path).toString(QUrl::FullyEncoded)).path, path);
    QVERIFY(!DesktopBackground::localImage("https://example.invalid/image.png").error.isEmpty());
    QVERIFY(!DesktopBackground::localImage("file://server/share/image.png").error.isEmpty());
    QVERIFY(!DesktopBackground::localImage("//server/share/image.png").error.isEmpty());
    QVERIFY(!DesktopBackground::localImage("").error.isEmpty());
    QVERIFY(DesktopBackground::localImage(directory.path()).error.contains("package"));
    QVERIFY(DesktopBackground::localImage(directory.path() + "/missing.png").error.contains("missing"));
    QVERIFY(DesktopBackground::localImage(fixtureFile(directory.path(), "timed.xml")).error.contains("slideshow"));
    QVERIFY(DesktopBackground::localImage(fixtureFile(directory.path(), "dynamic.heic")).error.contains("format"));
}

void DesktopBackgroundTest::settingsParsing()
{
    QTemporaryDir directory;
    const QString light = fixtureFile(directory.path(), "light.png");
    const QString dark = fixtureFile(directory.path(), "dark.png");
    QByteArray configuration = settings(light);
    configuration += "org.gnome.desktop.background picture-uri-dark '" + QUrl::fromLocalFile(dark).toEncoded() + "'\n";
    QCOMPARE(DesktopBackground::fromGSettings(configuration, false).path, light);
    QCOMPARE(DesktopBackground::fromGSettings(configuration, true).path, dark);
    QCOMPARE(DesktopBackground::fromGSettings(settings(light), true).path, light);
    QVERIFY(DesktopBackground::fromGSettings(configuration + "schema picture-options 'none'\n", false).error.contains("solid color"));
    QVERIFY(DesktopBackground::fromGSettings(configuration + "schema slideshow-enabled true\n", false).error.contains("slideshow"));
    QVERIFY(DesktopBackground::fromGSettings(configuration + "schema picture-uri-list [{'index': <0>}]\n", false).error.contains("Per-monitor"));
    QVERIFY(DesktopBackground::fromGSettings(QByteArray(16385, 'x'), false).error.contains("limit"));
    QVERIFY(!DesktopBackground::fromGSettings("schema picture-uri 'unterminated", false).error.isEmpty());
}

void DesktopBackgroundTest::plasmaParsing()
{
    QTemporaryDir directory;
    const QString image = fixtureFile(directory.path(), "plasma.jpg");
    QByteArray configuration = "[Containments][3]\nlastScreen=0\nwallpaperplugin=org.kde.image\n"
        "[Containments][3][Wallpaper][org.kde.image][General]\nImage=" + image.toUtf8() + "\n";
    QCOMPARE(DesktopBackground::fromPlasmaConfig(configuration).path, image);
    const QString configPath = directory.path() + "/plasma-config";
    QFile configFile(configPath);
    QVERIFY(configFile.open(QIODevice::WriteOnly));
    QCOMPARE(configFile.write(configuration), configuration.size());
    configFile.close();
    QCOMPARE(DesktopBackground::resolveLinux("KDE", {}, configPath).path, image);
    QByteArray slideshow = configuration;
    slideshow.replace("wallpaperplugin=org.kde.image", "wallpaperplugin=org.kde.slideshow");
    QVERIFY(DesktopBackground::fromPlasmaConfig(slideshow).error.contains("slideshow"));
    QVERIFY(DesktopBackground::fromPlasmaConfig(QByteArray(1048577, 'x')).error.contains("limit"));
    configuration += "[Containments][4]\nlastScreen=0\nwallpaperplugin=org.kde.image\n"
        "[Containments][4][Wallpaper][org.kde.image][General]\nImage=/another/activity.png\n";
    QVERIFY(DesktopBackground::fromPlasmaConfig(configuration).error.contains("activities"));
}

void DesktopBackgroundTest::commandFailureBounds()
{
#ifndef Q_OS_LINUX
    QSKIP("Mock gsettings command uses the Linux resolver.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString image = fixtureFile(directory.path(), "mock.png");
    const QString program = directory.path() + "/gsettings-fixture";
    QVERIFY(QFile::link(QCoreApplication::applicationFilePath(), program));
    qputenv("DOCKPIPE_WALLPAPER_FIXTURE", image.toUtf8());
    qputenv("DOCKPIPE_WALLPAPER_BEHAVIOR", "normal");
    QCOMPARE(DesktopBackground::resolveLinux("GNOME", program, {}).path, image);
    QVERIFY(!DesktopBackground::resolveLinux("unknown", program, {}).error.isEmpty());
    QVERIFY(!DesktopBackground::resolveLinux("GNOME", directory.path() + "/missing", {}).error.isEmpty());
    qputenv("DOCKPIPE_WALLPAPER_BEHAVIOR", "flood");
    QElapsedTimer timer;
    timer.start();
    QVERIFY(DesktopBackground::resolveLinux("GNOME", program, {}).error.contains("limit"));
    QVERIFY(timer.elapsed() < 2600);
    qputenv("DOCKPIPE_WALLPAPER_BEHAVIOR", "hang");
    const QString pidPath = directory.path() + "/fixture.pid";
    qputenv("DOCKPIPE_WALLPAPER_PID_FILE", pidPath.toUtf8());
    timer.restart();
    QVERIFY(DesktopBackground::resolveLinux("GNOME", program, {}).error.contains("limit"));
    QVERIFY(timer.elapsed() >= 1800);
    QVERIFY(timer.elapsed() < 2600);
    QFile pidFile(pidPath);
    QVERIFY(pidFile.open(QIODevice::ReadOnly));
    const qint64 pid = pidFile.read(32).trimmed().toLongLong();
    QVERIFY(pid > 1);
    errno = 0;
    QVERIFY(::kill(pid_t(pid), 0) == -1 && errno == ESRCH);
    qunsetenv("DOCKPIPE_WALLPAPER_FIXTURE");
    qunsetenv("DOCKPIPE_WALLPAPER_BEHAVIOR");
    qunsetenv("DOCKPIPE_WALLPAPER_PID_FILE");
#endif
}

int main(int argc, char **argv)
{
    QCoreApplication application(argc, argv);
    if (application.arguments().value(1) == "list-recursively" || application.arguments().value(1) == "get") {
        const QByteArray behavior = qgetenv("DOCKPIPE_WALLPAPER_BEHAVIOR");
        if (behavior == "hang") {
            QFile pidFile(qEnvironmentVariable("DOCKPIPE_WALLPAPER_PID_FILE"));
            if (pidFile.open(QIODevice::WriteOnly)) {
                pidFile.write(QByteArray::number(application.applicationPid()));
                pidFile.close();
            }
            QThread::sleep(10);
            return 0;
        }
        QByteArray output;
        if (behavior == "flood")
            output = QByteArray(20000, 'x');
        else if (application.arguments().value(1) == "get")
            output = "'default'\n";
        else
            output = settings(qEnvironmentVariable("DOCKPIPE_WALLPAPER_FIXTURE"));
        std::fwrite(output.constData(), 1, size_t(output.size()), stdout);
        std::fflush(stdout);
        return 0;
    }
    DesktopBackgroundTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "DesktopBackgroundTest.moc"
