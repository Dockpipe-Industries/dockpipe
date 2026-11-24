#include "RemoteBrowserWidget.h"
#include "BrowserNetworkContext.h"
#include "BrowserWidget.h"
#include "DockpipeChoices.h"
#include "RemoteCommand.h"
#include "WorkspaceMachineMenu.h"

#include <QCryptographicHash>
#include <QDir>
#include <QJsonDocument>
#include <QHBoxLayout>
#include <QLabel>
#include <QSettings>
#include <QSet>
#include <QStackedWidget>
#include <QStandardPaths>
#include <QVBoxLayout>

RemoteBrowserWidget::RemoteBrowserWidget(const QString &contextId, QWidget *parent)
    : QWidget(parent), m_contextId(contextId), m_machine(new WorkspaceMachineMenu(this)),
      m_stack(new QStackedWidget(this)), m_catalogCommand(new RemoteCommand(this))
{
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    m_machine->setNodeOnlyMode(true);
    auto *toolbar = new QHBoxLayout;
    toolbar->setObjectName(QStringLiteral("browserNetworkToolbar"));
    toolbar->addWidget(m_machine);
    toolbar->addStretch();
    layout->addLayout(toolbar);
    layout->addWidget(m_stack, 1);
    connect(m_machine, &WorkspaceMachineMenu::contextRequested, this, &RemoteBrowserWidget::setMachineContext);
    connect(m_machine, &WorkspaceMachineMenu::refreshRequested, this, [this] {
        refreshNetworkCatalog();
        emit refreshRequested();
    });
    connect(m_machine, &WorkspaceMachineMenu::unlockRequested, this, &RemoteBrowserWidget::unlockRequested);
    const auto saved = QJsonDocument::fromJson(QSettings().value(
        QStringLiteral("browserMachines/") + m_contextId).toByteArray()).object();
    m_target = saved.value("target").toObject();
    m_root = saved.value("root").toString();
    activateContext();
}

RemoteBrowserWidget::~RemoteBrowserWidget() = default;

QString RemoteBrowserWidget::contextKey(const QJsonObject &target) const
{
    const auto identity = m_contextId.toUtf8() + '\0' + target.value("node").toString().toUtf8()
        + '\0' + target.value("fingerprint").toString().toUtf8();
    return QString::fromLatin1(QCryptographicHash::hash(identity, QCryptographicHash::Sha256).toHex());
}

QJsonObject RemoteBrowserWidget::machineContext() const
{
    return {{"target", m_target}, {"root", m_root}};
}

void RemoteBrowserWidget::setMachineCatalog(const QJsonArray &catalog, bool unlocked)
{
    m_catalog = catalog;
    m_unlocked = unlocked;
    if (!unlocked) {
        m_catalogCommand->cancel();
        m_networkCatalog = {};
        for (auto *context : m_contexts)
            context->stop(tr("Remote access is locked."));
    }
    updateMachineMenu();
    activateContext();
    if (unlocked)
        refreshNetworkCatalog();
}

void RemoteBrowserWidget::refreshNetworkCatalog()
{
    if (!m_unlocked || m_catalogCommand->busy())
        return;
    QString error;
    const auto binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty())
        return;
    m_catalogCommand->run(binary, {"remote", "network-catalog", "--auth", "never"}, [this](bool success, const QByteArray &bytes) {
        m_networkCatalog = {};
        if (success && bytes.size() <= 256 * 1024) {
            const auto entries = QJsonDocument::fromJson(bytes).array();
            if (entries.size() <= 64)
                m_networkCatalog = entries;
        }
        updateMachineMenu();
        // Inactive contexts whose permission disappeared must also stop.
        for (auto it = m_contexts.begin(); it != m_contexts.end(); ++it) {
            bool available = false;
            for (const auto &entry : m_networkCatalog) {
                const auto catalog = entry.toObject();
                const auto target = catalog.value("target").toObject();
                if (catalog.value("enabled").toBool() && contextKey(target) == it.key()) {
                    available = true;
                    it.value()->setTarget(target);
                }
            }
            if (!available)
                it.value()->stop(tr("Remote browser access is unavailable."));
        }
        activateContext();
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty() || !m_unlocked)
            return;
        m_catalogCommand->run(binary, {"remote", "nodes", "--auth", "never"}, [this](bool ok, const QByteArray &data) {
            m_pairedCatalog = {};
            const auto nodes = QJsonDocument::fromJson(data).array();
            if (ok && data.size() <= 256 * 1024 && nodes.size() <= 64) {
                for (const auto &value : nodes) {
                    const auto node = value.toObject();
                    if (node.value("status") == "paired")
                        m_pairedCatalog.append(QJsonObject{{"target", QJsonObject{
                            {"node", node.value("node")}, {"fingerprint", node.value("fingerprint")}}}, {"enabled", false}});
                }
            }
            updateMachineMenu();
        });
    });
}

void RemoteBrowserWidget::updateMachineMenu()
{
    QJsonArray choices;
    QSet<QString> seen;
    bool networkSource = true;
    for (const auto &source : {m_networkCatalog, m_pairedCatalog, m_catalog}) {
        for (const auto &value : source) {
            auto entry = value.toObject();
            const auto target = entry.value("target").toObject();
            const auto key = contextKey(target);
            if (target.isEmpty() || seen.contains(key) || choices.size() >= 64)
                continue;
            seen.insert(key);
            if (!networkSource)
                entry["enabled"] = false;
            choices.append(entry);
        }
        networkSource = false;
    }
    QJsonObject displayTarget = m_target;
    for (const auto &value : choices) {
        const auto candidate = value.toObject().value("target").toObject();
        if (contextKey(candidate) == contextKey(m_target))
            displayTarget = candidate;
    }
    m_machine->setCatalog(choices);
    m_machine->setAccessUnlocked(m_unlocked);
    m_machine->setCurrent(displayTarget, {});
}

void RemoteBrowserWidget::setMachineContext(const QJsonObject &target, const QString &root)
{
    m_target = target;
    m_root = root;
    QSettings().setValue(QStringLiteral("browserMachines/") + m_contextId,
        QJsonDocument(machineContext()).toJson(QJsonDocument::Compact));
    updateMachineMenu();
    activateContext();
    emit machineContextChanged(target, root);
}

void RemoteBrowserWidget::activateContext()
{
    const auto selectedKey = m_target.isEmpty() ? QString() : contextKey(m_target);
    // One network browser process per panel. Inactive machines retain their
    // private persistent sessions and reopen them when selected again.
    for (auto it = m_contexts.begin(); it != m_contexts.end(); ++it) {
        if (it.key() != selectedKey)
            it.value()->stop(tr("Browser context saved. Select this machine to reconnect."));
    }
    if (m_target.isEmpty()) {
        if (!m_local) {
            m_local = new BrowserWidget(m_stack);
            m_stack->addWidget(m_local);
            m_local->setChromeVisible(m_chromeVisible);
            m_local->setCanvasPresentationEnabled(m_canvasPresentation);
            m_local->setCanvasViewportMode(m_canvasViewport);
            m_local->setBackgroundSuspended(m_backgroundSuspended);
            connect(m_local, &BrowserWidget::tabTearOutRequested, this, &RemoteBrowserWidget::tabTearOutRequested);
            connect(m_local, &BrowserWidget::activeViewChanged, this, &RemoteBrowserWidget::activeViewChanged);
        }
        m_stack->setCurrentWidget(m_local);
        emit activeViewChanged(m_local->activeView());
        return;
    }
    const auto key = contextKey(m_target);
    QJsonObject networkTarget;
    for (const auto &value : m_networkCatalog) {
        const auto entry = value.toObject();
        if (entry.value("enabled").toBool() && contextKey(entry.value("target").toObject()) == key)
            networkTarget = entry.value("target").toObject();
    }
    auto *context = m_contexts.value(key);
    if (!context) {
        // A panel retains at most the 64-machine catalog's browser contexts.
        if (m_contexts.size() >= 64) {
            auto *old = m_contexts.take(m_contexts.constBegin().key());
            delete old;
        }
        context = new BrowserNetworkContext(networkTarget.isEmpty() ? m_target : networkTarget,
            QStandardPaths::writableLocation(QStandardPaths::AppDataLocation)
                + QStringLiteral("/browser-contexts/") + key, m_stack);
        m_contexts.insert(key, context);
        m_stack->addWidget(context);
    }
    context->setPresentation(m_chromeVisible, m_canvasPresentation, m_canvasViewport, m_backgroundSuspended);
    m_stack->setCurrentWidget(context);
    if (!m_unlocked || networkTarget.isEmpty())
        context->stop(tr("Remote browser access is unavailable. Refresh machines or unlock remote access."));
    else {
        context->setTarget(networkTarget);
        context->start();
    }
    emit activeViewChanged(nullptr);
}

BrowserWidget *RemoteBrowserWidget::localBrowser() const
{
    return m_target.isEmpty() ? m_local : nullptr;
}

QWebEngineView *RemoteBrowserWidget::activeView() const
{
    return localBrowser() ? m_local->activeView() : nullptr;
}

void RemoteBrowserWidget::navigate(const QUrl &url)
{
    if (auto *browser = localBrowser()) {
        if (browser->activeView())
            browser->navigate(url);
    }
    else if (auto *context = m_contexts.value(contextKey(m_target)))
        context->navigate(url);
}

void RemoteBrowserWidget::setChromeVisible(bool visible)
{
    m_chromeVisible = visible;
    if (m_local)
        m_local->setChromeVisible(visible);
    if (auto *context = m_contexts.value(contextKey(m_target)))
        context->setPresentation(m_chromeVisible, m_canvasPresentation, m_canvasViewport, m_backgroundSuspended);
}

void RemoteBrowserWidget::setCanvasPresentationEnabled(bool enabled)
{
    m_canvasPresentation = enabled;
    if (m_local)
        m_local->setCanvasPresentationEnabled(enabled);
    if (auto *context = m_contexts.value(contextKey(m_target)))
        context->setPresentation(m_chromeVisible, m_canvasPresentation, m_canvasViewport, m_backgroundSuspended);
}

void RemoteBrowserWidget::setCanvasViewportMode(bool enabled)
{
    m_canvasViewport = enabled;
    if (m_local)
        m_local->setCanvasViewportMode(enabled);
    if (auto *context = m_contexts.value(contextKey(m_target)))
        context->setPresentation(m_chromeVisible, m_canvasPresentation, m_canvasViewport, m_backgroundSuspended);
}

void RemoteBrowserWidget::setBackgroundSuspended(bool suspended)
{
    m_backgroundSuspended = suspended;
    if (m_local)
        m_local->setBackgroundSuspended(suspended);
    if (auto *context = m_contexts.value(contextKey(m_target)))
        context->setPresentation(m_chromeVisible, m_canvasPresentation, m_canvasViewport, m_backgroundSuspended);
}
