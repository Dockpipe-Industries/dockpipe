#include "PackageSurfacePage.h"
#include "PackageDisplayWidget.h"
#include "SurfaceLaunchEnvironment.h"
#include "Context.h"
#include "ContextStore.h"
#include "SessionManager.h"

#include <QComboBox>
#include <QHBoxLayout>
#include <QHeaderView>
#include <QShowEvent>
#include <QHideEvent>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QMenu>
#include <QAction>
#include <QMenuBar>
#include <QSignalBlocker>
#include <QSet>
#include <QTableWidget>
#include <QTimer>
#include <QVBoxLayout>

namespace {
const QByteArray eventPrefix("::dockpipe-surface::");
constexpr int maximumOperationBytes = 2 * PackageSurfacePage::maximumEventBytes;
bool boundedText(const QJsonValue &value, int maximum = 4096)
{
    return value.isString() && value.toString().size() <= maximum;
}
}

PackageSurfacePage::PackageSurfacePage(SessionManager &sessions, QWidget *parent)
    : QWidget(parent), m_sessions(sessions), m_snapshotId(Context::createNew().id), m_actionId(Context::createNew().id)
{
    setObjectName("packageSurfacePage");
    auto *layout = new QVBoxLayout(this);
    auto *toolbar = new QHBoxLayout;
    m_providerSelector = new QComboBox;
    m_providerSelector->setObjectName("surfaceProvider");
    toolbar->addWidget(m_providerSelector, 1);
    m_resourceSelector = new QComboBox;
    m_resourceSelector->setObjectName("surfaceResourceSelector");
    toolbar->addWidget(m_resourceSelector, 2);
    auto *menuBar = new QMenuBar;
    menuBar->setObjectName("surfaceMenuBar");
    auto *view = menuBar->addMenu(tr("View"));
    auto *details = view->addAction(tr("Details and operation output"));
    details->setCheckable(true);
    m_actionsMenu = menuBar->addMenu(tr("Actions"));
    layout->addWidget(menuBar);
    auto *refresh = new QPushButton(tr("Refresh"));
    refresh->setObjectName("surfaceRefresh");
    toolbar->addWidget(refresh);
    m_cancel = new QPushButton(tr("Cancel operation"));
    m_cancel->setObjectName("surfaceCancel");
    toolbar->addWidget(m_cancel);
    layout->addLayout(toolbar);
    m_status = new QLabel(tr("Choose a provider to inspect its resources."));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setObjectName("surfaceStatus");
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    m_table = new QTableWidget(0, 2);
    m_table->setObjectName("surfaceResources");
    m_table->setHorizontalHeaderLabels({tr("Resource"), tr("State")});
    m_table->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Stretch);
    m_table->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_table->setSelectionMode(QAbstractItemView::SingleSelection);
    m_table->setEditTriggers(QAbstractItemView::NoEditTriggers);
    layout->addWidget(m_table, 2);
    m_details = new QPlainTextEdit;
    m_details->setReadOnly(true);
    m_details->setObjectName("surfaceDetails");
    layout->addWidget(m_details, 1);
    m_details->hide();
    m_display = new PackageDisplayWidget;
    m_display->setObjectName("surfaceNativeDisplay");
    m_display->setMinimumHeight(200);
    m_display->hide();
    layout->insertWidget(3, m_display, 4);

    m_output = new QPlainTextEdit;
    m_output->setReadOnly(true);
    m_output->setMaximumBlockCount(200);
    m_output->setObjectName("surfaceOutput");
    layout->addWidget(m_output, 1);
    m_output->hide();
    connect(details, &QAction::toggled, this, [this](bool shown) {
        m_details->setVisible(shown);
        m_output->setVisible(shown);
    });
    connect(m_resourceSelector, &QComboBox::currentIndexChanged, this, [this](int index) {
        if (index >= 0 && index < m_resources.size())
            m_table->selectRow(index);
    });
    m_poll = new QTimer(this);
    m_poll->setInterval(5000);
    m_poll->setObjectName("surfacePoll");
    m_snapshotDeadline = new QTimer(this);
    m_snapshotDeadline->setSingleShot(true);
    m_launchDeadline = new QTimer(this);
    m_launchDeadline->setObjectName("surfaceLaunchDeadline");
    m_launchDeadline->setSingleShot(true);
    connect(m_launchDeadline, &QTimer::timeout, this, [this]() {
        cancelLaunch(m_expectedLaunchId);
        m_status->setText(tr("The launched resource did not report a matching display within 120 seconds."));
    });
    m_actionDeadline = new QTimer(this);
    m_actionDeadline->setSingleShot(true);
    connect(m_poll, &QTimer::timeout, this, &PackageSurfacePage::refresh);
    connect(refresh, &QPushButton::clicked, this, &PackageSurfacePage::refresh);
    connect(m_providerSelector, &QComboBox::currentIndexChanged, this, &PackageSurfacePage::selectProvider);
    connect(m_table, &QTableWidget::itemSelectionChanged, this, [this]() {
        if (m_table->currentRow() >= 0)
            cancelLaunch(m_expectedLaunchId);
        showResource();
    });
    connect(m_cancel, &QPushButton::clicked, this, [this]() {
        if (m_actionBusy)
            failOperation(m_actionId, tr("Operation cancelled. Refresh to inspect the current state."));
        else if (m_snapshotBusy)
            failOperation(m_snapshotId, tr("Refresh cancelled. Displayed state is stale."));
    });
    connect(m_snapshotDeadline, &QTimer::timeout, this, [this]() {
        failOperation(m_snapshotId, tr("Refresh timed out. Displayed state is stale."));
    });
    connect(m_actionDeadline, &QTimer::timeout, this, [this]() {
        failOperation(m_actionId, tr("Operation timed out. Refresh to inspect the current state."));
    });
    connect(&m_sessions, &SessionManager::sessionOutput, this, &PackageSurfacePage::receiveOutput);
    connect(&m_sessions, &SessionManager::sessionFailed, this, &PackageSurfacePage::failOperation);
    connect(&m_sessions, &SessionManager::sessionStopped, this, [this](const QString &id, int code, QProcess::ExitStatus status) {
        finishOperation(id, code == 0 && status == QProcess::NormalExit);
        if (id == m_snapshotId)
            drainInventoryRequest();
    });
    updateActions();
}

PackageSurfacePage::~PackageSurfacePage()
{
    disconnect(&m_sessions, nullptr, this, nullptr);
    cancelOperations();
}

void PackageSurfacePage::setWorkspace(const QString &workdir, const QString &binary)
{
    if (m_workdir == workdir && m_binary == binary)
        return;
    recordInventoryTransition("workspace-reset");
    cancelOperations();
    m_workdir = workdir;
    m_binary = binary;
    cancelLaunch(m_expectedLaunchId);
    m_resources.clear();
    m_display->clearDisplay();
    m_display->hide();
    emit displaysChanged({});
    m_table->setRowCount(0);
    m_details->clear();
}

void PackageSurfacePage::setProviders(const QVector<ResolverMeta> &providers)
{
    bool unchanged = providers.size() == m_providers.size();
    for (int index = 0; unchanged && index < providers.size(); ++index) {
        const auto &before = m_providers[index];
        const auto &after = providers[index];
        unchanged = before.name == after.name && before.title == after.title && before.version == after.version
            && before.surface.id == after.surface.id && before.surface.title == after.surface.title
            && before.surface.packageRoot == after.surface.packageRoot
            && before.surface.snapshot == after.surface.snapshot && before.surface.actions.size() == after.surface.actions.size()
            && before.surface.nativeDisplay.interfaceVersion == after.surface.nativeDisplay.interfaceVersion
            && before.surface.nativeDisplay.packageRoot == after.surface.nativeDisplay.packageRoot
            && before.surface.nativeDisplay.libraries == after.surface.nativeDisplay.libraries;
        for (int action = 0; unchanged && action < before.surface.actions.size(); ++action) {
            const auto &left = before.surface.actions[action];
            const auto &right = after.surface.actions[action];
            unchanged = left.id == right.id && left.title == right.title && left.scope == right.scope
                && left.workflow == right.workflow && left.workflowId == right.workflowId
                && left.launchEnvironment == right.launchEnvironment;
        }
    }
    if (unchanged)
        return;
    recordInventoryTransition("providers-reset");
    const QString selected = m_providerSelector->currentData().toString();
    cancelOperations();
    m_providers = providers;
    m_providerSelector->blockSignals(true);
    m_providerSelector->clear();
    for (const auto &entry : providers)
        m_providerSelector->addItem(entry.title.isEmpty() ? entry.name : entry.title, entry.name);
    const int previous = m_providerSelector->findData(selected);
    if (previous >= 0)
        m_providerSelector->setCurrentIndex(previous);
    m_providerSelector->blockSignals(false);
    selectProvider();
}

const ResolverMeta *PackageSurfacePage::provider() const
{
    const int index = m_providerSelector->currentIndex();
    return index >= 0 && index < m_providers.size() ? &m_providers[index] : nullptr;
}

void PackageSurfacePage::setActive(bool active)
{
    if (active && !m_active)
        m_selectInitialResource = true;
    m_active = active;
    updateActions();
    if (active && isVisible()) {
        m_poll->start();
        refresh();
    } else {
        m_poll->stop();
        if (m_snapshotBusy)
            failOperation(m_snapshotId, tr("Refresh paused. Displayed state is stale."));
    }
}

void PackageSurfacePage::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_active) {
        m_poll->start();
        QTimer::singleShot(0, this, &PackageSurfacePage::refresh);
    }
}

void PackageSurfacePage::hideEvent(QHideEvent *event)
{
    QWidget::hideEvent(event);
    m_poll->stop();
    if (m_snapshotBusy)
        failOperation(m_snapshotId, tr("Refresh paused. Displayed state is stale."));
}

void PackageSurfacePage::selectProvider()
{
    recordInventoryTransition("provider-selection-reset");
    cancelOperations();
    cancelLaunch(m_expectedLaunchId);
    m_resources.clear();
    m_display->clearDisplay();
    m_display->hide();
    emit displaysChanged({});
    m_table->setRowCount(0);
    m_details->clear();
    m_output->clear();
    for (auto *button : m_actions.keys())
        delete button;
    m_actions.clear();
    if (const auto *selected = provider()) {
        for (const auto &action : selected->surface.actions) {
            auto *button = new QAction(action.title, this);
            button->setObjectName("surfaceAction-" + action.id);
            m_actions.insert(button, action);
            m_actionsMenu->addAction(button);
            connect(button, &QAction::triggered, this, [this, action]() { runAction(action); });
        }
    }
    m_status->setText(tr("State has not been refreshed."));
    updateActions();
    refresh();
}

QString PackageSurfacePage::selectedResource() const
{
    const int row = m_table->currentRow();
    return row >= 0 && row < m_resources.size() ? m_resources[row].id : QString();
}

void PackageSurfacePage::updateActions()
{
    const int row = m_table->currentRow();
    for (auto entry = m_actions.cbegin(); entry != m_actions.cend(); ++entry) {
        const bool available = entry.value().scope == "surface"
            || (row >= 0 && row < m_resources.size() && m_stateFresh && m_resources[row].actions.contains(entry.value().id));
        entry.key()->setEnabled(available && !m_actionBusy);
    }
    const bool hasDisplay = row >= 0 && row < m_resources.size() && (m_resources[row].displayProcessId > 0 || m_resources[row].packageDisplay);
    const bool displayVisible = hasDisplay && m_stateFresh && m_active && isVisible() && m_expectedLaunchId.isEmpty();
    m_display->setVisible(displayVisible);
    m_table->setVisible(!hasDisplay);
    if (displayVisible) {
        const auto &resource = m_resources[row];
        PackageDisplayMeta display{resource.id, resource.title, resource.displayProcessId, resource.displayStartToken};
        if (resource.packageDisplay) {
            display.nativePackage = provider()->surface.nativeDisplay;
            display.options = resource.displayOptions;
            display.leaseId = resource.id;
        }
        m_display->setDisplay(display);
    }
    else
        m_display->clearDisplay();
    m_cancel->setEnabled(m_actionBusy || m_snapshotBusy);
    m_providerSelector->setEnabled(!m_actionBusy);
}

void PackageSurfacePage::showResource()
{
    m_details->clear();
    const int row = m_table->currentRow();
    const QSignalBlocker resourceBlock(m_resourceSelector);
    m_resourceSelector->setCurrentIndex(row);
    if (row >= 0 && row < m_resources.size()) {
        QStringList lines;
        for (auto detail = m_resources[row].details.cbegin(); detail != m_resources[row].details.cend(); ++detail)
            lines.append(detail.key() + ": " + detail.value());
        m_details->setPlainText(lines.join('\n'));
    }
    updateActions();
}

void PackageSurfacePage::refresh()
{
    if (!m_active || !isVisible() || !provider() || m_workdir.isEmpty() || m_snapshotBusy || m_sessions.isRunning(m_snapshotId))
        return;
    runWorkflow(provider()->surface.snapshot, QString(), QString(), true);
}

bool PackageSurfacePage::supportsWorkflow(const QString &workflowId) const
{
    int matchingProviders = 0;
    for (const auto &contribution : m_providers) {
        for (const auto &action : contribution.surface.actions) {
            if (action.scope != "surface" || action.workflowId != workflowId || workflowId.isEmpty())
                continue;
            ++matchingProviders;
        }
    }
    return matchingProviders == 1;
}

bool PackageSurfacePage::prepareLaunchContext(Context &invocation, QString &error) const
{
    if (!supportsWorkflow(invocation.workflow)) {
        error = tr("The installed package launch action is ambiguous or unavailable.");
        return false;
    }
    for (const auto &contribution : m_providers) {
        for (const auto &action : contribution.surface.actions) {
            if (action.scope == "surface" && action.workflowId == invocation.workflow)
                return SurfaceLaunchEnvironment::apply(invocation, action, contribution.surface.packageRoot, error);
        }
    }
    return false;
}

void PackageSurfacePage::expectLaunch(const QString &workflowId, const QString &launchId)
{
    for (int index = 0; index < m_providers.size(); ++index) {
        for (const auto &action : m_providers[index].surface.actions) {
            if (action.scope == "surface" && action.workflowId == workflowId)
                m_providerSelector->setCurrentIndex(index);
        }
    }
    m_expectedLaunchId = launchId;
    m_selectInitialResource = false;
    m_launchDeadline->start(120000);
    const QSignalBlocker blockSelection(m_table);
    m_table->clearSelection();
    m_table->setCurrentCell(-1, -1);
    updateActions();
}

void PackageSurfacePage::cancelLaunch(const QString &launchId)
{
    if (launchId.isEmpty() || m_expectedLaunchId != launchId)
        return;
    m_expectedLaunchId.clear();
    m_launchDeadline->stop();
    m_selectInitialResource = false;
    emit launchResolved(launchId);
    updateActions();
}

void PackageSurfacePage::refreshInventory()
{
    recordInventoryTransition("inventory-request");
    if (!provider() || m_workdir.isEmpty())
        return;
    m_inventoryRefreshPending = true;
    drainInventoryRequest();
}

void PackageSurfacePage::drainInventoryRequest()
{
    if (!m_inventoryRefreshPending || m_snapshotBusy || m_sessions.isRunning(m_snapshotId))
        return;
    m_inventoryRefreshPending = false;
    if (provider() && !m_workdir.isEmpty())
        runWorkflow(provider()->surface.snapshot, QString(), QString(), true);
}

void PackageSurfacePage::recordInventoryTransition(const QString &transition)
{
    m_inventoryTransitions.append(transition);
    if (m_inventoryTransitions.size() > 32)
        m_inventoryTransitions.removeFirst();
}

QJsonObject PackageSurfacePage::inventoryDiagnostic() const
{
    return {{"transitions", QJsonArray::fromStringList(m_inventoryTransitions)},
        {"active", m_active}, {"visible", isVisible()}, {"snapshot_busy", m_snapshotBusy},
        {"snapshot_running", m_sessions.isRunning(m_snapshotId)},
        {"pending_request", m_inventoryRefreshPending}, {"provider_count", m_providers.size()},
        {"resource_count", m_resources.size()}, {"state_fresh", m_stateFresh},
        {"status", m_status->text().left(1024)}};
}

void PackageSurfacePage::runAction(const PackageSurfaceActionMeta &action)
{
    if (!provider() || m_actionBusy)
        return;
    const QString resource = action.scope == "resource" ? selectedResource() : QString();
    if (action.scope == "resource" && resource.isEmpty())
        return;
    if (!action.workflowId.isEmpty()) {
        emit publicWorkflowRequested(action.workflowId, provider()->surface.id, action.id, resource);
        return;
    }
    runWorkflow(action.workflow, action.id, resource, false);
}

void PackageSurfacePage::runWorkflow(const QString &file, const QString &action, const QString &resource, bool snapshot)
{
    if (!provider() || file.isEmpty() || m_sessions.isRunning(snapshot ? m_snapshotId : m_actionId))
        return;
    Context context = Context::createNew();
    context.id = snapshot ? m_snapshotId : m_actionId;
    context.workdir = m_workdir;
    context.workflowFile = file;
    context.dockpipeBinary = m_binary;
    context.maximumOutputLineBytes = maximumEventBytes;
    context.extraDockpipeEnv = {"DOCKPIPE_SURFACE_ID=" + provider()->surface.id,
        "DOCKPIPE_SURFACE_ACTION_ID=" + action, "DOCKPIPE_SURFACE_RESOURCE_ID=" + resource};
    m_pending[context.id].clear();
    m_received[context.id] = 0;
    m_failures.remove(context.id);
    if (snapshot) {
        recordInventoryTransition("snapshot-start");
        m_snapshotBusy = true;
        m_snapshotReceived = false;
        m_snapshotDeadline->start(30000);
    } else {
        m_actionBusy = true;
        m_output->clear();
        m_actionDeadline->start(120000);
    }
    m_status->setText(snapshot ? tr("Refreshing… Previous state is retained.") : tr("Operation running…"));
    updateActions();
    if (!m_sessions.launch(context, ContextStore::logsDir()))
        finishOperation(context.id, false);
}

void PackageSurfacePage::receiveOutput(const QString &id, const QString &text)
{
    if ((id != m_snapshotId || !m_snapshotBusy) && (id != m_actionId || !m_actionBusy))
        return;
    if (m_failures.contains(id))
        return;
    const QByteArray data = text.toUtf8();
    m_received[id] += data.size();
    if (m_received[id] > maximumOperationBytes) {
        failOperation(id, tr("Package output exceeded the 2 MiB operation capacity."));
        return;
    }
    auto &pending = m_pending[id];
    pending += data;
    while (true) {
        const int newline = pending.indexOf('\n');
        if (newline < 0)
            break;
        const auto line = pending.left(newline);
        pending.remove(0, newline + 1);
        if (line.startsWith(eventPrefix)) {
            if (line.size() > maximumEventBytes) {
                failOperation(id, tr("Package event exceeded the 1 MiB capacity."));
                return;
            }
            QJsonParseError parse;
            const auto document = QJsonDocument::fromJson(line.mid(eventPrefix.size()), &parse);
            QString error;
            if (parse.error != QJsonParseError::NoError || !document.isObject() || !applyEvent(document.object(), error)) {
                failOperation(id, error.isEmpty() ? tr("Package returned an invalid surface event.") : error);
                return;
            }
            if (id == m_snapshotId && document.object().value("kind").toString() == "snapshot")
                m_snapshotReceived = true;
        } else if (id == m_actionId) {
            m_output->appendPlainText(QString::fromUtf8(line).left(4096));
        }
    }
}

void PackageSurfacePage::failOperation(const QString &id, const QString &message)
{
    if ((id != m_snapshotId || !m_snapshotBusy) && (id != m_actionId || !m_actionBusy))
        return;
    if (m_failures.contains(id))
        return;
    m_failures[id] = message;
    if (id == m_snapshotId) {
        recordInventoryTransition("snapshot-failed");
        m_stateFresh = false;
    }
    m_status->setText(message);
    updateActions();
    if (id == m_actionId)
        m_output->appendPlainText(message);
    m_sessions.stop(id);
}

void PackageSurfacePage::finishOperation(const QString &id, bool success)
{
    const bool snapshot = id == m_snapshotId && m_snapshotBusy;
    const bool action = id == m_actionId && m_actionBusy;
    if (!snapshot && !action)
        return;
    if (!m_pending[id].isEmpty())
        receiveOutput(id, "\n");
    if (snapshot) {
        recordInventoryTransition(success ? "snapshot-finished" : "snapshot-finished-error");
        m_snapshotBusy = false;
        m_stateFresh = success && m_snapshotReceived && !m_failures.contains(id);
        m_snapshotDeadline->stop();
    } else {
        m_actionBusy = false;
        m_actionDeadline->stop();
    }
    if (m_failures.contains(id))
        m_status->setText(m_failures.value(id));
    else if (!success || (snapshot && !m_snapshotReceived))
        m_status->setText(tr("Package operation failed or returned no snapshot. Displayed state is stale."));
    else
        m_status->setText(snapshot ? tr("State refreshed.") : tr("Operation completed. Refreshing state…"));
    m_pending.remove(id);
    updateActions();
    if (action)
        refresh();
}

void PackageSurfacePage::cancelOperations()
{
    m_inventoryRefreshPending = false;
    m_snapshotDeadline->stop();
    m_actionDeadline->stop();
    if (m_snapshotBusy)
        m_sessions.stop(m_snapshotId);
    if (m_actionBusy)
        m_sessions.stop(m_actionId);
    m_snapshotBusy = false;
    m_actionBusy = false;
    m_stateFresh = false;
    m_pending.clear();
}

bool PackageSurfacePage::applyEvent(const QJsonObject &event, QString &error)
{
    if (event.value("schema") != QJsonValue(1)) {
        error = tr("Unsupported package surface event schema.");
        return false;
    }
    if (event.value("kind").toString() == "progress") {
        if (!boundedText(event.value("message")) || !boundedText(event.value("phase"), 160)
            || (event.contains("resource_id") && !boundedText(event.value("resource_id"), 256))
            || (event.contains("percent") && (!event.value("percent").isDouble()
                || event.value("percent").toDouble() < 0 || event.value("percent").toDouble() > 100))) {
            error = tr("Invalid package progress event.");
            return false;
        }
        QString progress = event.value("phase").toString() + ": " + event.value("message").toString();
        if (event.contains("percent"))
            progress += QString(" (%1%)").arg(event.value("percent").toDouble());
        m_status->setText(progress);
        return true;
    }
    const auto values = event.value("resources");
    if (event.value("kind").toString() != "snapshot" || !values.isArray() || values.toArray().size() > maximumResources) {
        error = tr("Package snapshot is invalid or exceeds 256 resources.");
        return false;
    }
    QVector<Resource> resources;
    QSet<QString> seen;
    for (const auto &value : values.toArray()) {
        if (!value.isObject()) {
            error = tr("Invalid package resource.");
            return false;
        }
        const auto object = value.toObject();
        const QString id = object.value("id").toString();
        if (!boundedText(object.value("id"), 256) || id.isEmpty() || id.contains('\n') || id.contains('\r') || id.contains(QChar(0))
            || seen.contains(id) || !boundedText(object.value("title"), 160) || !boundedText(object.value("state"), 160)
            || !object.value("details").isObject() || object.value("details").toObject().size() > 32
            || !object.value("actions").isArray() || object.value("actions").toArray().size() > 32) {
            error = tr("Package resource fields exceed the surface contract.");
            return false;
        }
        seen.insert(id);
        Resource resource{id, object.value("title").toString(), object.value("state").toString(), {}, {}};
        if (object.contains("display")) {
            const auto value = object.value("display");
            const auto display = value.toObject();
            if (display.value("kind") == QJsonValue("package-native")) {
                if (!value.isObject() || display.size() != 2 || !display.value("options").isObject()
                    || !provider() || provider()->surface.nativeDisplay.interfaceVersion != 1
                    || !PackageDisplayWidget::validateOptions(display.value("options").toObject(), error)) {
                    if (error.isEmpty())
                        error = tr("Invalid package widget display options or missing installed factory.");
                    return false;
                }
                resource.packageDisplay = true;
                resource.displayOptions = display.value("options").toObject();
            } else {
                const auto pid = display.value("process_id");
                const auto token = display.value("process_start_token");
                const double number = pid.toDouble();
                const QString tokenText = token.toString();
                bool control = false;
                for (const auto character : tokenText)
                    control = control || character.unicode() < 32 || character.unicode() == 127;
                if (!value.isObject() || display.size() != 3 || display.value("kind") != QJsonValue("native-window")
                    || !pid.isDouble() || number < 1 || number > 2147483647 || number != qint64(number)
                    || !boundedText(token, 256) || tokenText.isEmpty() || control) {
                    error = tr("Invalid package native display identity.");
                    return false;
                }
                resource.displayProcessId = qint64(number);
                resource.displayStartToken = tokenText;
            }
        }
        if (object.contains("launch_id")) {
            const auto launch = object.value("launch_id");
            const QString value = launch.toString();
            bool safe = !value.isEmpty() && value.size() <= 128;
            for (const auto character : value) {
                const bool asciiLetterOrDigit = character.unicode() < 128 && character.isLetterOrNumber();
                const bool punctuation = character == '-' || character == '_' || character == '.';
                safe = safe && (asciiLetterOrDigit || punctuation);
            }
            if (!launch.isString() || !safe) {
                error = tr("Invalid package resource launch identity.");
                return false;
            }
            resource.launchId = value;
        }
        const auto details = object.value("details").toObject();
        for (auto detail = details.begin(); detail != details.end(); ++detail) {
            if (detail.key().size() > 160 || !boundedText(detail.value())) {
                error = tr("Invalid package resource details.");
                return false;
            }
            resource.details.insert(detail.key(), detail.value().toString());
        }
        for (const auto &available : object.value("actions").toArray()) {
            if (!boundedText(available, 64)) {
                error = tr("Invalid package resource action.");
                return false;
            }
            resource.actions.append(available.toString());
        }
        resources.append(resource);
    }
    const QString selected = selectedResource();
    const QSignalBlocker resourceBlock(m_resourceSelector);
    m_resourceSelector->clear();
    for (const auto &resource : resources)
        m_resourceSelector->addItem(resource.title, resource.id);
    m_table->blockSignals(true);
    m_table->clearSelection();
    m_table->setCurrentCell(-1, -1);
    m_resources = resources;
    m_stateFresh = true;
    m_table->setRowCount(resources.size());
    for (int row = 0; row < resources.size(); ++row) {
        m_table->setItem(row, 0, new QTableWidgetItem(resources[row].title));
        m_table->setItem(row, 1, new QTableWidgetItem(resources[row].state));
        if (m_expectedLaunchId.isEmpty() && resources[row].id == selected)
            m_table->selectRow(row);
        if (!m_expectedLaunchId.isEmpty() && (resources[row].displayProcessId > 0 || resources[row].packageDisplay)
            && resources[row].launchId == m_expectedLaunchId) {
            const QString resolved = m_expectedLaunchId;
            m_expectedLaunchId.clear();
            m_launchDeadline->stop();
            m_table->selectRow(row);
            emit launchResolved(resolved);
        }
    }
    if (m_selectInitialResource && m_expectedLaunchId.isEmpty() && m_table->currentRow() < 0 && !resources.isEmpty()) {
        int initialRow = 0;
        for (int row = 0; row < resources.size(); ++row) {
            if (resources[row].displayProcessId > 0 || resources[row].packageDisplay) {
                initialRow = row;
                break;
            }
        }
        m_table->selectRow(initialRow);
    }
    m_selectInitialResource = false;
    m_table->blockSignals(false);
    QVector<PackageDisplayMeta> displays;
    for (const auto &resource : resources) {
        if (resource.displayProcessId > 0 || resource.packageDisplay) {
            PackageDisplayMeta display{resource.id, resource.title, resource.displayProcessId, resource.displayStartToken};
            if (resource.packageDisplay) {
                display.nativePackage = provider()->surface.nativeDisplay;
                display.options = resource.displayOptions;
                display.leaseId = resource.id;
            }
            displays.append(display);
        }
    }
    recordInventoryTransition(QString("snapshot-published-%1").arg(displays.size()));
    emit displaysChanged(displays);
    showResource();
    return true;
}
