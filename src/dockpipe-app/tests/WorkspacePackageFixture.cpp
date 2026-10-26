#include "LauncherWorkspacePackage.h"
#include <QLabel>
#include <QObject>
#include <QPushButton>
#include <QVariant>
#include <QVBoxLayout>

class FixtureFileSurface final : public LauncherFileSurface {
public:
    FixtureFileSurface(const QString &root, const QString &directory, QWidget *parent)
        : LauncherFileSurface(parent)
    {
        setObjectName("fixtureFileSurface");
        setProperty("fixtureRoot", root);
        setProperty("fixtureDirectory", directory);
        auto *contents = new QVBoxLayout(this);
        contents->addWidget(new QLabel("Workspace graph fixture", this));
        auto *close = new QPushButton("Back to files", this);
        close->setObjectName("fixtureCloseFiles");
        contents->addWidget(close);
        connect(close, &QPushButton::clicked, this, [this] {
            if (requestClose)
                requestClose();
        });
    }

    void applyCockpitSettings(const CockpitSettings &settings) override
    {
        setProperty("fixtureCockpitEnabled", settings.enabled);
    }

    QWidget *flightWidget() const override { return nullptr; }
    void pauseFlight(bool) override {}
};

// A small native package exercises the host lifecycle without a renderer/browser.
class WorkspacePackageFixture final : public QObject, public LauncherWorkspacePackage {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID LauncherWorkspacePackage_iid)
    Q_INTERFACES(LauncherWorkspacePackage)
public:
    void initialize(const QString &) override {}
    LauncherWorkspaceSurface *createWorkspace(QWidget *) override { return nullptr; }
    LauncherFileSurface *createFiles(const QString &root, const QString &directory, QWidget *parent) override
    {
        return new FixtureFileSurface(root, directory, parent);
    }
};
#include "WorkspacePackageFixture.moc"
