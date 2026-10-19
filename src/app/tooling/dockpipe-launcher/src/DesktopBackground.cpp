#include "DesktopBackground.h"

#include <QCoreApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QFileInfo>
#include <QMap>
#include <QProcess>
#include <QRegularExpression>
#include <QStandardPaths>
#include <QThread>
#include <QUrl>

#ifdef Q_OS_WIN
#include <windows.h>
#endif
#ifdef Q_OS_MACOS
DesktopBackgroundResult resolveMacDesktopBackground();
#endif

namespace {
constexpr qsizetype commandOutputLimit = 16 * 1024;
constexpr qsizetype plasmaConfigLimit = 1024 * 1024;
constexpr int commandBudgetMs = 2000;

DesktopBackgroundResult failure(const char *message)
{
    return {{}, QCoreApplication::translate("DesktopBackground", message)};
}

QString unquote(QString text, bool *valid)
{
    text = text.trimmed();
    *valid = text.size() >= 2 && (text.front() == '\'' || text.front() == '"') && text.back() == text.front();
    if (!*valid)
        return {};
    QString value;
    for (int index = 1; index < text.size() - 1; ++index) {
        QChar current = text[index];
        if (current == '\\') {
            if (++index >= text.size() - 1) {
                *valid = false;
                return {};
            }
            current = text[index];
            if (current == 'n')
                current = '\n';
            else if (current == 'r')
                current = '\r';
            else if (current == 't')
                current = '\t';
            else if (current != '\\' && current != '\'' && current != '"') {
                *valid = false;
                return {};
            }
        }
        value += current;
    }
    return value;
}

struct CommandResult {
    QByteArray output;
    bool success = false;
    bool exhausted = false;
};

void disposeProcess(QProcess *process)
{
    if (process->state() == QProcess::NotRunning) {
        delete process;
        return;
    }
    process->closeReadChannel(QProcess::StandardOutput);
    process->closeReadChannel(QProcess::StandardError);
    process->kill();
    if (process->waitForFinished(100)) {
        delete process;
        return;
    }
    // Do not let QProcess's blocking destructor extend this worker's deadline.
    // The application reaps an unusually slow killed child asynchronously.
    process->moveToThread(QCoreApplication::instance()->thread());
    QMetaObject::invokeMethod(process, [process] {
        process->setParent(QCoreApplication::instance());
        if (process->state() == QProcess::NotRunning)
            process->deleteLater();
        else
            QObject::connect(process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished),
                process, &QObject::deleteLater);
    }, Qt::QueuedConnection);
}

CommandResult readSetting(const QString &program, const QStringList &arguments,
    QElapsedTimer &deadline, qsizetype &remainingOutput)
{
    CommandResult result;
    if (deadline.elapsed() >= commandBudgetMs || remainingOutput <= 0) {
        result.exhausted = true;
        return result;
    }
    auto *process = new QProcess;
    process->setProcessChannelMode(QProcess::MergedChannels);
    process->start(program, arguments, QIODevice::ReadOnly);
    const auto collect = [&] {
        if (!process->isOpen())
            return;
        const QByteArray bytes = process->read(remainingOutput + 1);
        if (bytes.size() > remainingOutput) {
            result.exhausted = true;
            return;
        }
        remainingOutput -= bytes.size();
        result.output += bytes;
    };
    while (process->state() != QProcess::NotRunning) {
        if (deadline.elapsed() >= commandBudgetMs) {
            result.exhausted = true;
            break;
        }
        process->waitForReadyRead(qMin(50, commandBudgetMs - int(deadline.elapsed())));
        collect();
        if (result.exhausted)
            break;
    }
    if (!result.exhausted)
        collect();
    result.success = !result.exhausted && process->state() == QProcess::NotRunning
        && process->exitStatus() == QProcess::NormalExit && process->exitCode() == 0
        && process->error() != QProcess::FailedToStart;
    disposeProcess(process);
    return result;
}
}

DesktopBackgroundResult DesktopBackground::localImage(const QString &pathOrUrl)
{
    if (pathOrUrl.isEmpty())
        return failure("The desktop has no configured wallpaper image; it may use a solid color.");
    if (pathOrUrl.size() > 4096 || pathOrUrl.contains(QChar::Null))
        return failure("The desktop wallpaper path is invalid or too long.");
    QString path = pathOrUrl;
    if (!QDir::isAbsolutePath(path)) {
        // Desktop settings may contain display-form file URLs with literal spaces.
        const QUrl url(path, QUrl::TolerantMode);
        if (!url.isValid() || !url.isLocalFile() || (!url.host().isEmpty() && url.host() != "localhost")
            || url.hasQuery() || url.hasFragment())
            return failure("Only local static wallpaper images can be inherited; network or dynamic wallpaper URLs are unsupported.");
        path = url.toLocalFile();
    }
    if (path.startsWith("//") || path.startsWith("\\\\"))
        return failure("Network wallpaper paths are unsupported.");
    const QFileInfo image(path);
    if (!image.exists() || !image.isReadable())
        return failure("The configured desktop wallpaper image is missing or unreadable.");
    if (!image.isFile())
        return failure("The desktop uses a wallpaper package or folder. Choose a static image instead.");
    const QString suffix = image.suffix().toLower();
    if (suffix == "xml")
        return failure("The desktop uses a timed wallpaper slideshow. Choose a static image instead.");
    if (!QStringList{"png", "jpg", "jpeg", "webp", "bmp"}.contains(suffix))
        return failure("This wallpaper format is unsupported. Choose a PNG, JPEG, WebP or BMP image.");
    if (image.size() > 20 * 1024 * 1024)
        return failure("The desktop wallpaper exceeds the 20 MiB image limit.");
    return {image.absoluteFilePath(), {}};
}

DesktopBackgroundResult DesktopBackground::fromGSettings(const QByteArray &configuration, bool preferDark)
{
    if (configuration.size() > commandOutputLimit)
        return failure("Desktop wallpaper settings exceed the 16 KiB read limit.");
    QMap<QString, QString> values;
    for (const QByteArray &raw : configuration.split('\n')) {
        const QString line = QString::fromUtf8(raw).trimmed();
        const int first = line.indexOf(' ');
        const int second = line.indexOf(' ', first + 1);
        if (first <= 0 || second <= first)
            continue;
        values.insert(line.mid(first + 1, second - first - 1), line.mid(second + 1).trimmed());
    }
    if (values.value("slideshow-enabled") == "true")
        return failure("The desktop uses a wallpaper slideshow. Choose a static image instead.");
    const QString monitors = values.value("picture-uri-list");
    if (!monitors.isEmpty() && monitors != "[]" && monitors != "@aa{sv} []")
        return failure("Per-monitor Cinnamon wallpaper settings are not supported. Choose a static image instead.");
    bool valid = false;
    if (values.contains("picture-options") && unquote(values.value("picture-options"), &valid) == "none")
        return failure("The desktop background is a solid color, not a wallpaper image.");
    QString selected = values.value(preferDark ? "picture-uri-dark" : "picture-uri");
    if (selected.isEmpty())
        selected = values.value("picture-uri");
    const QString path = unquote(selected, &valid);
    if (!valid)
        return failure("The desktop did not provide a supported wallpaper image setting.");
    return localImage(path);
}

DesktopBackgroundResult DesktopBackground::fromPlasmaConfig(const QByteArray &configuration)
{
    if (configuration.size() > plasmaConfigLimit)
        return failure("Plasma wallpaper configuration exceeds the 1 MiB read limit.");
    const QRegularExpression header("^\\[Containments\\]\\[([0-9]+)\\](.*)$");
    QMap<QString, QMap<QString, QString>> containments;
    QString id;
    QString section;
    for (const QByteArray &raw : configuration.split('\n')) {
        const QString line = QString::fromUtf8(raw).trimmed();
        if (line.startsWith('[')) {
            const auto match = header.match(line);
            id = match.hasMatch() ? match.captured(1) : QString();
            section = match.captured(2);
            continue;
        }
        const int equals = line.indexOf('=');
        if (id.isEmpty() || equals < 0)
            continue;
        const QString key = line.left(equals);
        if (section.isEmpty() && (key == "wallpaperplugin" || key == "lastScreen"))
            containments[id].insert(key, line.mid(equals + 1));
        else if (section == "[Wallpaper][org.kde.image][General]" && key == "Image")
            containments[id].insert(key, line.mid(equals + 1));
    }
    QString image;
    bool active = false;
    for (const auto &entry : containments) {
        if (entry.value("lastScreen") != "0")
            continue;
        active = true;
        if (entry.value("wallpaperplugin") != "org.kde.image")
            return failure("The primary Plasma desktop uses a slideshow, color or wallpaper plugin. Choose a static image instead.");
        const QString candidate = entry.value("Image");
        if (candidate.contains('\\'))
            return failure("This Plasma wallpaper path uses unsupported configuration escaping. Choose a static image instead.");
        if (!image.isEmpty() && image != candidate)
            return failure("Plasma has different wallpapers across activities. Choose the image for this dashboard explicitly.");
        image = candidate;
    }
    if (!active)
        return failure("No primary-screen static wallpaper was found in Plasma's desktop configuration.");
    return localImage(image);
}

DesktopBackgroundResult DesktopBackground::resolveLinux(const QString &desktop,
    const QString &settingsProgram, const QString &plasmaConfigFile)
{
    const QString name = desktop.toLower();
    if (name.contains("kde") || name.contains("plasma")) {
        QFile configuration(plasmaConfigFile);
        if (!configuration.open(QIODevice::ReadOnly))
            return failure("Plasma's desktop configuration is missing or unreadable.");
        return fromPlasmaConfig(configuration.read(plasmaConfigLimit + 1));
    }
    const bool cinnamon = name.contains("cinnamon");
    if (!cinnamon && !name.contains("gnome") && !name.contains("pop") && !name.contains("unity"))
        return failure("Desktop wallpaper inheritance is unsupported on this desktop. Choose an image instead.");
    if (!QCoreApplication::instance())
        return failure("Desktop wallpaper resolution requires an application runtime.");
    QElapsedTimer deadline;
    deadline.start();
    qsizetype remaining = commandOutputLimit;
    const QString schema = cinnamon ? "org.cinnamon.desktop.background" : "org.gnome.desktop.background";
    const auto configuration = readSetting(settingsProgram, {"list-recursively", schema}, deadline, remaining);
    if (!configuration.success)
        return failure("Could not read desktop wallpaper settings within the 2 second / 16 KiB limit. The gsettings utility and desktop schema are required.");
    bool preferDark = false;
    if (!cinnamon) {
        const auto style = readSetting(settingsProgram, {"get", "org.gnome.desktop.interface", "color-scheme"}, deadline, remaining);
        if (style.exhausted)
            return failure("Reading desktop wallpaper settings exceeded the 2 second / 16 KiB limit.");
        bool valid = false;
        preferDark = style.success && unquote(QString::fromUtf8(style.output), &valid) == "prefer-dark" && valid;
    }
    QByteArray values = configuration.output;
    if (cinnamon) {
        const auto slideshow = readSetting(settingsProgram,
            {"get", "org.cinnamon.desktop.background.slideshow", "slideshow-enabled"}, deadline, remaining);
        if (slideshow.exhausted)
            return failure("Reading desktop wallpaper settings exceeded the 2 second / 16 KiB limit.");
        if (slideshow.success && slideshow.output.trimmed() == "true")
            values += "\norg.cinnamon.desktop.background.slideshow slideshow-enabled true\n";
    }
    return fromGSettings(values, preferDark);
}

DesktopBackgroundResult DesktopBackground::resolve()
{
#ifdef Q_OS_LINUX
    return resolveLinux(qEnvironmentVariable("XDG_CURRENT_DESKTOP"), "gsettings",
        QStandardPaths::writableLocation(QStandardPaths::GenericConfigLocation)
            + "/plasma-org.kde.plasma.desktop-appletsrc");
#elif defined(Q_OS_WIN)
    WCHAR path[MAX_PATH]{};
    if (!SystemParametersInfoW(SPI_GETDESKWALLPAPER, MAX_PATH, path, 0))
        return failure("Windows could not provide its current desktop wallpaper path.");
    return localImage(QString::fromWCharArray(path));
#elif defined(Q_OS_MACOS)
    return resolveMacDesktopBackground();
#else
    return failure("Desktop wallpaper inheritance is unsupported on this platform.");
#endif
}
