#include "LauncherWorkspacePackage.h"
#include <QObject>

// A small native package exercises the host lifecycle without a renderer/browser.
class WorkspacePackageFixture final : public QObject, public LauncherWorkspacePackage {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID LauncherWorkspacePackage_iid)
    Q_INTERFACES(LauncherWorkspacePackage)
public:
    void initialize(const QString &) override {}
    LauncherWorkspaceSurface *createWorkspace(QWidget *) override { return nullptr; }
    LauncherFileSurface *createFiles(const QString &, const QString &, QWidget *) override { return nullptr; }
};
#include "WorkspacePackageFixture.moc"
