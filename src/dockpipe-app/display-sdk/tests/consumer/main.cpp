#include <LauncherDisplayPackage.h>
#include <LauncherNativeLibraryScope.h>
#include <QApplication>
#include <QJsonObject>

class ConsumerSurface final : public LauncherDisplaySurface {
public:
    using LauncherDisplaySurface::LauncherDisplaySurface;
    void setActive(bool active) override
    {
        setProperty("active", active);
    }
    void closeDisplay() override
    {
        setActive(false);
    }
};

class ConsumerPackage final : public LauncherDisplayPackage {
public:
    void initialize(const QString &directory) override
    {
        m_directory = directory;
    }
    LauncherDisplaySurface *createDisplay(const QJsonObject &options, QWidget *parent) override
    {
        auto *surface = new ConsumerSurface(parent);
        surface->setProperty("resource", options.value("resource_id").toString());
        surface->setProperty("directory", m_directory);
        return surface;
    }
private:
    QString m_directory;
};

int main(int argc, char **argv)
{
    QApplication application(argc, argv);
    LauncherNativeLibraryScope nativeLibrary;
    QString nativeError;
    const bool loaded = nativeLibrary.load(application.applicationDirPath() + "/missing-display-library.dll", nativeError);
#if defined(Q_OS_WIN)
    if (loaded || !nativeError.contains("Native module load failed"))
        return 1;
#else
    if (!loaded || !nativeError.isEmpty())
        return 1;
#endif
    ConsumerPackage package;
    package.initialize("installed-resource-root");
    auto *surface = package.createDisplay({{"resource_id", "fixture"}}, nullptr);
    surface->setActive(true);
    const bool valid = surface->property("active").toBool()
        && surface->property("resource").toString() == "fixture"
        && surface->property("directory").toString() == "installed-resource-root";
    surface->closeDisplay();
    surface->closeDisplay();
    const bool closed = !surface->property("active").toBool();
    delete surface;
    return valid && closed ? 0 : 1;
}
