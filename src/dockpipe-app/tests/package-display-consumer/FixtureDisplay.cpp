#include <LauncherDisplayPackage.h>
#include <QLabel>
#include <QVBoxLayout>
#include <QTimer>

class FixtureSurface final : public LauncherDisplaySurface {
    Q_OBJECT
    Q_PROPERTY(int closeCompletionBudgetMs READ closeCompletionBudgetMs CONSTANT)
    Q_PROPERTY(QJsonObject closeReference READ closeReference)
    Q_PROPERTY(QJsonObject presentationOptions READ presentationOptions)
public:
    FixtureSurface(QObject &owner, const QJsonObject &options, QWidget *parent)
        : LauncherDisplaySurface(parent), m_owner(owner)
    {
        setObjectName("installedConsumerDisplay");
        setProperty("launchOptions", options);
        setProperty("captureOptions", options.value("capture").toObject());
        setProperty("decoderMode", options.value("decoder").toString());
        m_presentationOptions = options.value("display").toObject();
        const auto catalog = options.value("catalog").toObject();
        m_reference = {{"schema", "dockpipe.remote.desktop/v1"}, {"id", QString(64, '9')},
            {"target", catalog.value("target")}, {"epoch", catalog.value("epoch")}};
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
    int closeCompletionBudgetMs() const { return 150; }
    QJsonObject closeReference() const { return m_reference; }
    QJsonObject presentationOptions() const { return m_presentationOptions; }
    Q_INVOKABLE void setPresentationOptions(const QJsonObject &options)
    {
        if (options.size() != 2 || !options.value("hide_cursor").isBool()
            || (options.value("scale") != QJsonValue("fit") && options.value("scale") != QJsonValue("actual")))
            return;
        m_presentationOptions = options;
        setProperty("presentationUpdates", property("presentationUpdates").toInt() + 1);
    }
    Q_INVOKABLE void requestClose()
    {
        setProperty("closeRequested", true);
        const auto mode = qgetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
        if (mode == "timeout")
            return;
        QTimer::singleShot(70, this, [this, mode] {
            auto selected = m_reference;
            if (mode == "foreign")
                selected["id"] = QString(64, '8');
            emit closeCompleted({{"schema", "dockpipe.desktop.close/v1"}, {"reference", selected},
                {"outcome", mode == "unknown" ? "unknown" : "closed"}});
        });
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
signals:
    void closeCompleted(const QJsonObject &result);
private:
    QJsonObject m_reference;
    QJsonObject m_presentationOptions;
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
