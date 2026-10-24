#include <LauncherDisplayPackage.h>
#include <QLabel>
#include <QVBoxLayout>

class FixtureSurface final : public LauncherDisplaySurface {
public:
    FixtureSurface(QObject &owner, const QJsonObject &options, QWidget *parent)
        : LauncherDisplaySurface(parent), m_owner(owner)
    {
        setObjectName("installedConsumerDisplay");
        setProperty("resourceId", options.value("resource_id").toString());
        m_owner.setProperty("liveViews", m_owner.property("liveViews").toInt() + 1);
        auto *layout = new QVBoxLayout(this);
        layout->addWidget(new QLabel("Installed package display fixture"));
    }
    ~FixtureSurface() override
    {
        closeDisplay();
        m_owner.setProperty("liveViews", m_owner.property("liveViews").toInt() - 1);
    }
    void setActive(bool active) override
    {
        setProperty("active", active);
    }
    void closeDisplay() override
    {
        if (m_closed)
            return;
        m_closed = true;
        setActive(false);
        m_owner.setProperty("closedViews", m_owner.property("closedViews").toInt() + 1);
    }
private:
    QObject &m_owner;
    bool m_closed = false;
};

class FixturePackage final : public QObject, public LauncherDisplayPackage {
    Q_OBJECT
    Q_PLUGIN_METADATA(IID FIXTURE_IID FILE FIXTURE_METADATA)
    Q_INTERFACES(LauncherDisplayPackage)
public:
    void initialize(const QString &assets) override
    {
        setProperty("packageRoot", assets);
    }
    LauncherDisplaySurface *createDisplay(const QJsonObject &options, QWidget *parent) override
    {
        return new FixtureSurface(*this, options, parent);
    }
};
#include "FixtureDisplay.moc"
