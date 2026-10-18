#include "LauncherWorkspacePackage.h"
#include "CockpitWidget.h"
#include "WorkspaceGraphWidget.h"
#include <QObject>
#include <QDir>
#include <QFileInfo>

class CockpitPackage final : public QObject, public LauncherWorkspacePackage {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID LauncherWorkspacePackage_iid)
    Q_INTERFACES(LauncherWorkspacePackage)
public:
    void initialize(const QString &assetsDirectory) override
    {
        const QDir assets(assetsDirectory);
#ifdef Q_OS_WIN
        const QString helper = assets.filePath("ui/cockpit-reader.exe");
#else
        const QString helper = assets.filePath("ui/cockpit-reader");
#endif
        qputenv("DOCKPIPE_COCKPIT_READER", helper.toUtf8());
        const QString renderer = assets.filePath("ui/QtWebEngineProcess");
        if (QFileInfo::exists(renderer)) {
            qputenv("QTWEBENGINEPROCESS_PATH", renderer.toUtf8());
            qputenv("QTWEBENGINE_RESOURCES_PATH", assets.filePath("resources").toUtf8());
            qputenv("QTWEBENGINE_LOCALES_PATH", assets.filePath("locales").toUtf8());
        }
    }
    LauncherWorkspaceSurface *createWorkspace(QWidget *parent) override
    {
        return new CockpitWidget(parent);
    }
    LauncherFileSurface *createFiles(const QString &root, const QString &directory, QWidget *parent) override
    {
        return new WorkspaceGraphWidget(root, directory, parent);
    }
};

#include "CockpitPackage.moc"
