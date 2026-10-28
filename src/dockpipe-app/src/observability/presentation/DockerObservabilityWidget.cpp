#include "DockerObservabilityWidget.h"
#include "MonitoringMachineSelector.h"
#include "DockpipeChoices.h"
#include "CanvasIcons.h"
#include "DockerResourceInspector.h"
#include "SetupTerminal.h"
#include <QDialog>
#include <QUrl>
#include <QMessageBox>

#include <QHeaderView>
#include <QElapsedTimer>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QMenu>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QScrollBar>
#include <QShowEvent>
#include <QSignalBlocker>
#include <QSplitter>
#include <QStyle>
#include <QTabWidget>
#include <QTableWidget>
#include <QTableWidgetItem>
#include <QTextCursor>
#include <QTimer>
#include <QSet>
#include <QVBoxLayout>
#include <QFrame>
#include <QtConcurrent>

namespace {

QTableWidgetItem *roItem(const QString &text, const QString &id = QString())
{
    auto *item = new QTableWidgetItem(text);
    item->setFlags(item->flags() & ~Qt::ItemIsEditable);
    if (!id.isEmpty())
        item->setData(Qt::UserRole, id);
    return item;
}

QString prettyJson(const QString &text)
{
    const QJsonDocument doc = QJsonDocument::fromJson(text.toUtf8());
    if (doc.isNull())
        return text;
    return QString::fromUtf8(doc.toJson(QJsonDocument::Indented));
}

QString firstIdFromInspect(const QString &inspectJson)
{
    const QJsonDocument doc = QJsonDocument::fromJson(inspectJson.toUtf8());
    if (!doc.isArray() || doc.array().isEmpty() || !doc.array().first().isObject())
        return {};
    return doc.array().first().toObject().value(QStringLiteral("Id")).toString();
}

QLabel *makeMetricPill(const QString &title)
{
    auto *label = new QLabel(title);
    label->setObjectName(QStringLiteral("dockerMetric"));
    label->setAlignment(Qt::AlignCenter);
    return label;
}

QTableWidgetItem *statusItem(const QString &text)
{
    auto *item = roItem(text);
    const QString lower = text.toLower();
    if (lower.contains(QStringLiteral("healthy")))
        item->setData(Qt::UserRole + 1, QStringLiteral("healthy"));
    else if (lower.contains(QStringLiteral("running")))
        item->setData(Qt::UserRole + 1, QStringLiteral("running"));
    else
        item->setData(Qt::UserRole + 1, QStringLiteral("other"));
    return item;
}

QString normalizeContainerState(QString state, const QString &statusText)
{
    state = state.trimmed().toLower();
    const QString lowerStatus = statusText.toLower();
    if (state == QStringLiteral("running")) {
        if (lowerStatus.contains(QStringLiteral("unhealthy")))
            return QStringLiteral("unhealthy");
        if (lowerStatus.contains(QStringLiteral("healthy")))
            return QStringLiteral("healthy");
        return QStringLiteral("running");
    }
    if (state == QStringLiteral("paused"))
        return QStringLiteral("paused");
    if (state == QStringLiteral("restarting"))
        return QStringLiteral("restarting");
    if (state == QStringLiteral("exited"))
        return QStringLiteral("exited");
    if (state == QStringLiteral("dead"))
        return QStringLiteral("dead");
    if (state == QStringLiteral("created"))
        return QStringLiteral("created");
    return QStringLiteral("other");
}

QLabel *makeStatusPill(const QString &text, const QString &state, QWidget *parent)
{
    auto *label = new QLabel(text, parent);
    label->setObjectName(QStringLiteral("dockerStatusPill"));
    label->setProperty("state", state);
    label->setAlignment(Qt::AlignCenter);
    return label;
}

QIcon statusIconForState(const QString &state, QWidget *widget)
{
    Q_UNUSED(widget);
    if (state == QStringLiteral("healthy") || state == QStringLiteral("running"))
        return CanvasIcons::icon(CanvasIcons::Symbol::Check);
    if (state == QStringLiteral("paused") || state == QStringLiteral("exited"))
        return CanvasIcons::icon(CanvasIcons::Symbol::Minus);
    return CanvasIcons::icon(CanvasIcons::Symbol::Monitor);
}

int rowForId(QTableWidget *table, const QString &id)
{
    if (!table || id.isEmpty())
        return -1;
    for (int row = 0; row < table->rowCount(); ++row) {
        auto *item = table->item(row, 0);
        if (item && item->data(Qt::UserRole).toString() == id)
            return row;
    }
    return -1;
}

void removeRowsNotIn(QTableWidget *table, const QSet<QString> &seenIds)
{
    if (!table)
        return;
    for (int row = table->rowCount() - 1; row >= 0; --row) {
        auto *item = table->item(row, 0);
        const QString id = item ? item->data(Qt::UserRole).toString() : QString();
        if (id.isEmpty() || !seenIds.contains(id))
            table->removeRow(row);
    }
}

void setItemText(QTableWidget *table, int row, int column, const QString &text, const QString &id = QString())
{
    auto *item = table->item(row, column);
    if (!item) {
        table->setItem(row, column, roItem(text, id));
        return;
    }
    item->setText(text);
    if (!id.isEmpty())
        item->setData(Qt::UserRole, id);
}

bool isScrolledToBottom(const QPlainTextEdit *edit)
{
    if (!edit || !edit->verticalScrollBar())
        return true;
    const QScrollBar *bar = edit->verticalScrollBar();
    return bar->value() >= bar->maximum() - 4;
}

void restoreScroll(QPlainTextEdit *edit, bool wasAtBottom, int previousValue)
{
    if (!edit || !edit->verticalScrollBar())
        return;
    QScrollBar *bar = edit->verticalScrollBar();
    if (wasAtBottom)
        bar->setValue(bar->maximum());
    else
        bar->setValue(qMin(previousValue, bar->maximum()));
}

void replacePlainTextWithoutFlash(QPlainTextEdit *edit, const QString &text)
{
    if (!edit || edit->toPlainText() == text)
        return;
    const bool wasAtBottom = isScrolledToBottom(edit);
    const int previousValue = edit->verticalScrollBar() ? edit->verticalScrollBar()->value() : 0;
    const QSignalBlocker blocker(edit);
    edit->setPlainText(text);
    restoreScroll(edit, wasAtBottom, previousValue);
}

void streamPlainTextWithoutFlash(QPlainTextEdit *edit, const QString &text)
{
    if (!edit)
        return;
    const QString current = edit->toPlainText();
    if (current == text)
        return;

    const bool wasAtBottom = isScrolledToBottom(edit);
    const int previousValue = edit->verticalScrollBar() ? edit->verticalScrollBar()->value() : 0;
    const QSignalBlocker blocker(edit);
    if (!current.isEmpty() && text.startsWith(current)) {
        edit->moveCursor(QTextCursor::End);
        edit->insertPlainText(text.mid(current.size()));
    } else {
        edit->setPlainText(text);
    }
    restoreScroll(edit, wasAtBottom, previousValue);
}

} // namespace

DockerObservabilityWidget::DockerObservabilityWidget(QWidget *parent) : QWidget(parent), m_remoteCommand(this)
{
    m_refreshWatcher = new QFutureWatcher<DockerSnapshot>(this);
    connect(m_refreshWatcher, &QFutureWatcher<DockerSnapshot>::finished, this, [this]() {
        const bool quiet = m_currentRefreshQuiet;
        m_currentRefreshQuiet = false;
        if (m_localRefreshGeneration == m_generation) {
            applySnapshot(m_refreshWatcher->result());
            m_hasLoadedOnce = true;
        }
        if (m_refreshPending) {
            const bool pendingQuiet = m_refreshPendingQuiet;
            m_refreshPending = false;
            m_refreshPendingQuiet = false;
            if (pendingQuiet)
                QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refreshQuietly, Qt::QueuedConnection);
            else
                QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        } else if (quiet) {
            setLoadingState(false);
        }
    });

    m_containerDetailWatcher = new QFutureWatcher<ContainerDetailSnapshot>(this);
    connect(m_containerDetailWatcher, &QFutureWatcher<ContainerDetailSnapshot>::finished, this, [this]() {
        if (m_containerDetailWatcher->property("generation").toULongLong() != m_generation)
            return;
        const ContainerDetailSnapshot snapshot = m_containerDetailWatcher->result();
        if (snapshot.containerId != m_pendingContainerDetailId) {
            refreshContainerDetailsAsync(m_pendingContainerDetailId);
            return;
        }
        m_displayedContainerDetailId = snapshot.containerId;
        if (m_containerDetails)
            replacePlainTextWithoutFlash(m_containerDetails, snapshot.inspectText);
        if (m_containerLogs)
            streamPlainTextWithoutFlash(m_containerLogs, snapshot.logsText);
    });

    m_networkDetailWatcher = new QFutureWatcher<ObjectDetailSnapshot>(this);
    connect(m_networkDetailWatcher, &QFutureWatcher<ObjectDetailSnapshot>::finished, this, [this]() {
        if (m_networkDetailWatcher->property("generation").toULongLong() != m_generation)
            return;
        const ObjectDetailSnapshot snapshot = m_networkDetailWatcher->result();
        if (snapshot.objectId != m_pendingNetworkDetailId) {
            refreshNetworkDetailsAsync(m_pendingNetworkDetailId);
            return;
        }
        if (m_networkDetails)
            m_networkDetails->setPlainText(snapshot.detailText);
    });

    m_volumeDetailWatcher = new QFutureWatcher<ObjectDetailSnapshot>(this);
    connect(m_volumeDetailWatcher, &QFutureWatcher<ObjectDetailSnapshot>::finished, this, [this]() {
        if (m_volumeDetailWatcher->property("generation").toULongLong() != m_generation)
            return;
        const ObjectDetailSnapshot snapshot = m_volumeDetailWatcher->result();
        if (snapshot.objectId != m_pendingVolumeDetailId) {
            refreshVolumeDetailsAsync(m_pendingVolumeDetailId);
            return;
        }
        if (m_volumeDetails)
            m_volumeDetails->setPlainText(snapshot.detailText);
    });

    m_autoRefreshTimer = new QTimer(this);
    m_autoRefreshTimer->setInterval(4000);
    connect(m_autoRefreshTimer, &QTimer::timeout, this, &DockerObservabilityWidget::refreshQuietly);

    buildUi();
    auto *refreshButton = m_inspector->findChild<QPushButton *>("dockerRefresh");
    if (refreshButton) {
        connect(&m_remoteCommand, &RemoteCommand::busyChanged, refreshButton,
                [refreshButton](bool busy) { refreshButton->setEnabled(!busy); });
    }
}

void DockerObservabilityWidget::buildUi()
{
    m_inventory = new QDialog(this);
    m_inventory->setWindowTitle(tr("Docker resources"));
    m_inventory->resize(900, 560);
    auto *inventoryLayout = new QVBoxLayout(m_inventory);
    m_tabs = new QTabWidget;
    m_tabs->addTab(buildContainersPage(), tr("Containers"));
    m_tabs->addTab(buildNetworksPage(), tr("Networks"));
    m_tabs->addTab(buildVolumesPage(), tr("Volumes"));
    inventoryLayout->addWidget(m_tabs);
    // The full inventory is available on demand, never duplicated in the inspector.
    m_containerDetails->parentWidget()->hide();
    m_inspector = new DockerResourceInspector(m_containerLogs,
        {m_containerDetails, m_networkDetails, m_volumeDetails});
    m_canvas = new ResourceCanvas("docker", tr("Containers"),
        tr("Solid arrows: declared dependencies · Dashed lines: network or storage attachments"), m_inspector, this, false);
    m_canvas->setDetailsLabel(tr("Inspector"));
    m_canvas->addKindFilter("container", tr("Containers"));
    m_canvas->addKindFilter("network", tr("Networks"));
    m_canvas->addKindFilter("volume", tr("Volumes"));
    m_canvas->addInactiveFilter();
    m_canvas->addDependencyFilter();
    auto *canvasLayout = new QVBoxLayout(this);
    canvasLayout->setContentsMargins(0, 0, 0, 0);
    auto *machineBar = new QHBoxLayout;
    machineBar->setContentsMargins(20, 12, 20, 4);
    m_machine = new MonitoringMachineSelector;
    machineBar->addWidget(new QLabel(tr("Machine")));
    machineBar->addWidget(m_machine);
    machineBar->addStretch();
    canvasLayout->addLayout(machineBar);
    connect(m_machine, qOverload<int>(&QComboBox::currentIndexChanged), this, &DockerObservabilityWidget::changeMachine);
    canvasLayout->addWidget(m_canvas);
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, &DockerObservabilityWidget::selectResource);
    for (auto *table : {m_containers, m_networks, m_volumes}) {
        connect(table, &QTableWidget::cellDoubleClicked, this, [this](int, int) {
            m_inventory->accept();
            if (!m_canvas->focusResource(m_selectedResource))
                m_canvas->showDetails();
        });
    }
    connect(m_inspector, &DockerResourceInspector::resourceRequested, this, [this](const QString &encoded) {
        const QString key = QUrl::fromPercentEncoding(encoded.toUtf8());
        if (!m_canvas->focusResource(key)) {
            selectResource(key);
            m_canvas->showDetails();
        }
    });
    connect(m_inspector, &DockerResourceInspector::refreshRequested, this, [this]() {
        if (!m_machine->currentData().toString().isEmpty())
            refreshRemote(true);
        else
            refresh();
    });
    connect(m_inspector, &DockerResourceInspector::pauseChanged, this, [this](bool paused) {
        m_monitorPaused = paused;
        if (paused)
            m_autoRefreshTimer->stop();
        else if (m_active) {
            m_autoRefreshTimer->start();
            refresh();
        }
    });
    connect(m_inspector, &DockerResourceInspector::inventoryRequested, m_inventory, &QDialog::show);
    connect(m_inspector, &DockerResourceInspector::startRequested, this, &DockerObservabilityWidget::onStartSelectedContainer);
    connect(m_inspector, &DockerResourceInspector::stopRequested, this, &DockerObservabilityWidget::onStopSelectedContainer);
    connect(m_inspector, &DockerResourceInspector::execRequested, this, &DockerObservabilityWidget::openExec);
}

void DockerObservabilityWidget::selectResource(const QString &key)
{
    const QString kind = key.section(':', 0, 0);
    const QString id = key.section(':', 1);
    QTableWidget *table = nullptr;
    if (kind == "container")
        table = m_containers;
    else if (kind == "network")
        table = m_networks;
    else if (kind == "volume")
        table = m_volumes;
    if (!table)
        return;
    const int row = rowForId(table, id);
    if (row < 0)
        return;
    m_selectedResource = key;
    table->selectRow(row);
    updateInspector();
    if (kind == "container")
        refreshContainerDetailsAsync(id);
    else if (kind == "network")
        refreshNetworkDetailsAsync(id);
    else
        refreshVolumeDetailsAsync(id);
}

void DockerObservabilityWidget::updateInspector()
{
    if (!m_inspector)
        return;
    DockerInspectorSelection selection;
    selection.key = m_selectedResource;
    const QString kind = m_selectedResource.section(':', 0, 0);
    const QString id = m_selectedResource.section(':', 1);
    QTableWidget *table = nullptr;
    if (kind == "container") {
        table = m_containers;
        selection.kind = 0;
    } else if (kind == "network") {
        table = m_networks;
        selection.kind = 1;
    } else if (kind == "volume") {
        table = m_volumes;
        selection.kind = 2;
    }
    const int row = table ? rowForId(table, id) : -1;
    if (row < 0) {
        selection.kind = -1;
        selection.status = m_selectedResource.isEmpty()
            ? tr("Choose a resource on the canvas.") : tr("This resource is no longer in the current inventory.");
        m_inspector->setSelection(selection);
        return;
    }
    selection.name = table->item(row, 0)->text();
    selection.status = kind;
    if (selection.kind == 0) {
        const QString state = table->item(row, 0)->data(Qt::UserRole + 1).toString();
        selection.canExec = state == "running" || state == "healthy" || state == "unhealthy";
        selection.canStart = state == "exited" || state == "created";
        selection.canStop = selection.canExec || state == "paused" || state == "restarting";
        selection.status = tr("Container · %1").arg(table->item(row, 1)->data(Qt::ToolTipRole).toString());
    }
    if (m_containerActionBusy || !m_machine->currentData().toString().isEmpty()) {
        selection.canExec = false;
        selection.canStart = false;
        selection.canStop = false;
    }
    selection.detailsHtml = "<p><b>ID</b><br>" + id.toHtmlEscaped() + "</p>";
    for (int column = 0; column < table->columnCount(); ++column) {
        QString value = table->item(row, column)->text();
        if (selection.kind == 0 && column == 1)
            value = table->item(row, column)->data(Qt::ToolTipRole).toString();
        selection.detailsHtml += "<p><b>" + table->horizontalHeaderItem(column)->text().toHtmlEscaped()
            + "</b><br>" + (value.isEmpty() ? tr("Not reported") : value.toHtmlEscaped()) + "</p>";
    }
    QStringList related;
    const auto append = [&](const QString &relatedKind, const QString &relatedId, const QString &name) {
        const QString key = relatedKind + ":" + relatedId;
        const QString link = QStringLiteral("<a href='%1'>%2</a>").arg(
            QString::fromLatin1(QUrl::toPercentEncoding(key)), (relatedKind + ": " + name).toHtmlEscaped());
        if (!related.contains(link))
            related.append(link);
    };
    for (const auto &value : m_snapshot.topology) {
        const auto object = value.toObject();
        QString name = object.value("Name").toString();
        if (name.startsWith('/'))
            name.remove(0, 1);
        QString containerId;
        for (const auto &container : m_snapshot.containers) {
            if (container.name == name) {
                containerId = container.id;
                break;
            }
        }
        if (containerId.isEmpty())
            continue;
        const bool selectedContainer = kind == "container" && id == containerId;
        const auto networks = object.value("Networks").toObject();
        for (auto network = networks.begin(); network != networks.end(); ++network) {
            for (const auto &inventory : m_snapshot.networks) {
                if (inventory.name != network.key())
                    continue;
                if (selectedContainer)
                    append("network", inventory.id, inventory.name);
                else if (kind == "network" && id == inventory.id)
                    append("container", containerId, name);
            }
        }
        for (const auto &mountValue : object.value("Mounts").toArray()) {
            const auto mount = mountValue.toObject();
            const QString volume = mount.value("Name").toString();
            if (mount.value("Type").toString() != "volume")
                continue;
            if (selectedContainer)
                append("volume", volume, volume);
            else if (kind == "volume" && id == volume)
                append("container", containerId, name);
        }
    }
    selection.relatedHtml = related.isEmpty() ? tr("No related resources in this snapshot.")
        : tr("Related: ") + related.join(" &nbsp; · &nbsp; ");
    m_inspector->setSelection(selection);
}

void DockerObservabilityWidget::openExec(const QString &shell)
{
    if (!m_machine->currentData().toString().isEmpty())
        return;
    if (!m_selectedResource.startsWith("container:") || (shell != "/bin/sh" && shell != "/bin/bash"))
        return;
    const QString state = selectedContainerState();
    if (state != "running" && state != "healthy" && state != "unhealthy")
        return;
    QString error;
    if (!openCommandTerminal("docker", {"exec", "-it", selectedContainerId(), shell}, &error))
        QMessageBox::warning(this, tr("Could not open container terminal"), error);
}

QWidget *DockerObservabilityWidget::buildContainersPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_containers = new QTableWidget(splitter);
    m_containers->setObjectName(QStringLiteral("dockerTable"));
    m_containers->setColumnCount(5);
    m_containers->setHorizontalHeaderLabels({tr("Name"), tr("State"), tr("Image"), tr("Ports"), tr("Created")});
    m_containers->horizontalHeader()->setStretchLastSection(false);
    m_containers->horizontalHeader()->setSectionResizeMode(0, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(1, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(2, QHeaderView::Stretch);
    m_containers->horizontalHeader()->setSectionResizeMode(3, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(4, QHeaderView::ResizeToContents);
    m_containers->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_containers->setSelectionMode(QAbstractItemView::SingleSelection);
    m_containers->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_containers->setAlternatingRowColors(true);
    m_containers->setShowGrid(false);
    m_containers->verticalHeader()->setVisible(false);
    m_containers->verticalHeader()->setDefaultSectionSize(36);
    m_containers->setContextMenuPolicy(Qt::CustomContextMenu);

    auto *detailSplitter = new QSplitter(Qt::Horizontal, splitter);
    detailSplitter->setChildrenCollapsible(false);
    m_containerDetails = new QPlainTextEdit(detailSplitter);
    m_containerDetails->setObjectName(QStringLiteral("detailConsole"));
    m_containerDetails->setReadOnly(true);
    m_containerDetails->setPlaceholderText(tr("Select a container to inspect bindings, mounts, networks, and config."));
    m_containerLogs = new QPlainTextEdit(detailSplitter);
    m_containerLogs->setObjectName(QStringLiteral("detailConsole"));
    m_containerLogs->setReadOnly(true);
    m_containerLogs->setPlaceholderText(tr("Recent docker logs appear here."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_containers, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onContainersSelectionChanged);
    connect(m_containers, &QWidget::customContextMenuRequested, this,
            &DockerObservabilityWidget::onContainersContextMenuRequested);
    return page;
}

QWidget *DockerObservabilityWidget::buildNetworksPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_networks = new QTableWidget(splitter);
    m_networks->setObjectName(QStringLiteral("dockerTable"));
    m_networks->setColumnCount(3);
    m_networks->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Scope")});
    m_networks->horizontalHeader()->setStretchLastSection(true);
    m_networks->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_networks->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_networks->setSelectionMode(QAbstractItemView::SingleSelection);
    m_networks->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_networks->setAlternatingRowColors(true);
    m_networks->setShowGrid(false);
    m_networks->verticalHeader()->setVisible(false);

    m_networkDetails = new QPlainTextEdit(splitter);
    m_networkDetails->setObjectName(QStringLiteral("detailConsole"));
    m_networkDetails->setReadOnly(true);
    m_networkDetails->setPlaceholderText(tr("Select a network to inspect attached containers and configuration."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_networks, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onNetworksSelectionChanged);
    return page;
}

QWidget *DockerObservabilityWidget::buildVolumesPage()
{
    auto *page = new QWidget(this);
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);

    auto *splitter = new QSplitter(Qt::Vertical, page);
    splitter->setChildrenCollapsible(false);

    m_volumes = new QTableWidget(splitter);
    m_volumes->setObjectName(QStringLiteral("dockerTable"));
    m_volumes->setColumnCount(3);
    m_volumes->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Mountpoint")});
    m_volumes->horizontalHeader()->setStretchLastSection(true);
    m_volumes->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_volumes->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_volumes->setSelectionMode(QAbstractItemView::SingleSelection);
    m_volumes->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_volumes->setAlternatingRowColors(true);
    m_volumes->setShowGrid(false);
    m_volumes->verticalHeader()->setVisible(false);

    m_volumeDetails = new QPlainTextEdit(splitter);
    m_volumeDetails->setObjectName(QStringLiteral("detailConsole"));
    m_volumeDetails->setReadOnly(true);
    m_volumeDetails->setPlaceholderText(tr("Select a volume to inspect usage details."));

    splitter->setStretchFactor(0, 2);
    splitter->setStretchFactor(1, 3);
    layout->addWidget(splitter, 1);

    connect(m_volumes, &QTableWidget::itemSelectionChanged, this, &DockerObservabilityWidget::onVolumesSelectionChanged);
    return page;
}

DockerObservabilityWidget::CommandResult DockerObservabilityWidget::runDocker(const QStringList &args, bool mergeOutput)
{
    QProcess proc;
    CommandResult result;
    if (mergeOutput)
        proc.setProcessChannelMode(QProcess::MergedChannels);
    proc.start(QStringLiteral("docker"), args);
    if (!proc.waitForStarted(3000)) {
        result.stderrText = proc.errorString();
        return result;
    }
    QElapsedTimer elapsed;
    elapsed.start();
    QByteArray output;
    QByteArray errors;
    constexpr int maximumBytes = 32 * 1024 * 1024;
    while (true) {
        proc.waitForFinished(50);
        output += proc.readAllStandardOutput();
        errors += proc.readAllStandardError();
        if (output.size() + errors.size() > maximumBytes || elapsed.elapsed() > 15000) {
            proc.kill();
            proc.waitForFinished(1000);
            result.stderrText = tr("Docker query exceeded its 15-second or 32-MiB response limit.");
            return result;
        }
        if (proc.state() == QProcess::NotRunning)
            break;
    }
    result.ok = proc.exitStatus() == QProcess::NormalExit && proc.exitCode() == 0;
    result.stdoutText = QString::fromLocal8Bit(output);
    result.stderrText = QString::fromLocal8Bit(errors);
    if (!result.ok && result.stderrText.trimmed().isEmpty())
        result.stderrText = mergeOutput ? result.stdoutText : proc.errorString();
    return result;
}

DockerObservabilityWidget::DockerSnapshot DockerObservabilityWidget::collectDockerSnapshot()
{
    DockerSnapshot snapshot;

    const auto containersResult = runDocker(
        {QStringLiteral("container"), QStringLiteral("ls"), QStringLiteral("--all"), QStringLiteral("--format"),
         QStringLiteral("{{json .}}")});
    snapshot.containersOk = containersResult.ok;
    if (!containersResult.ok) {
        snapshot.containersError = containersResult.stderrText.trimmed();
    } else {
        const QStringList lines = containersResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            ContainerRowData row;
            row.id = o.value(QStringLiteral("ID")).toString();
            row.name = o.value(QStringLiteral("Names")).toString();
            row.statusText = o.value(QStringLiteral("Status")).toString();
            row.state = normalizeContainerState(o.value(QStringLiteral("State")).toString(), row.statusText);
            row.image = o.value(QStringLiteral("Image")).toString();
            row.ports = o.value(QStringLiteral("Ports")).toString();
            row.created = o.value(QStringLiteral("RunningFor")).toString();
            snapshot.containers.append(row);
        }
    }

    const auto networksResult = runDocker(
        {QStringLiteral("network"), QStringLiteral("ls"), QStringLiteral("--format"), QStringLiteral("{{json .}}")});
    snapshot.networksOk = networksResult.ok;
    if (!networksResult.ok) {
        snapshot.networksError = networksResult.stderrText.trimmed();
    } else {
        const QStringList lines = networksResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            NetworkRowData row;
            row.id = o.value(QStringLiteral("ID")).toString();
            row.name = o.value(QStringLiteral("Name")).toString();
            row.driver = o.value(QStringLiteral("Driver")).toString();
            row.scope = o.value(QStringLiteral("Scope")).toString();
            snapshot.networks.append(row);
        }
    }

    const auto volumesResult = runDocker(
        {QStringLiteral("volume"), QStringLiteral("ls"), QStringLiteral("--format"), QStringLiteral("{{json .}}")});
    snapshot.volumesOk = volumesResult.ok;
    if (!volumesResult.ok) {
        snapshot.volumesError = volumesResult.stderrText.trimmed();
    } else {
        const QStringList lines = volumesResult.stdoutText.split(QLatin1Char('\n'), Qt::SkipEmptyParts);
        for (const QString &line : lines) {
            const QJsonDocument doc = QJsonDocument::fromJson(line.toUtf8());
            if (!doc.isObject())
                continue;
            const QJsonObject o = doc.object();
            VolumeRowData row;
            row.name = o.value(QStringLiteral("Name")).toString();
            row.driver = o.value(QStringLiteral("Driver")).toString();
            const auto inspect = runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), row.name});
            if (inspect.ok) {
                const QJsonDocument inspectDoc = QJsonDocument::fromJson(inspect.stdoutText.toUtf8());
                if (inspectDoc.isArray() && !inspectDoc.array().isEmpty() && inspectDoc.array().first().isObject())
                    row.mountpoint = inspectDoc.array().first().toObject().value(QStringLiteral("Mountpoint")).toString();
            }
            snapshot.volumes.append(row);
        }
    }

    // Keep discovery bounded and omit environment values from topology data.
    QStringList inspectArgs{"container", "inspect", "--format",
        R"({"Id":{{json .Id}},"Name":{{json .Name}},"Labels":{{json .Config.Labels}},"Networks":{{json .NetworkSettings.Networks}},"Mounts":{{json .Mounts}}})"};
    constexpr int maximumGraphContainers = 256;
    for (int i = 0; i < qMin(maximumGraphContainers, int(snapshot.containers.size())); ++i)
        inspectArgs.append(snapshot.containers[i].id);
    if (!snapshot.containers.isEmpty()) {
        const auto topology = runDocker(inspectArgs);
        if (topology.ok) {
            for (const auto &line : topology.stdoutText.split('\n', Qt::SkipEmptyParts)) {
                const auto document = QJsonDocument::fromJson(line.toUtf8());
                if (document.isObject())
                    snapshot.topology.append(document.object());
                else
                    snapshot.topologyNotice = tr("Some Docker relationships could not be read. Inspect the selected object for details.");
            }
        } else {
            snapshot.topologyNotice = tr("Relationships unavailable: %1").arg(topology.stderrText);
        }
        if (snapshot.containers.size() > maximumGraphContainers)
            snapshot.topologyNotice += tr(" Relationships shown for the first 256 containers. Full inventory is available in Details & actions.");
    }
    return snapshot;
}

void DockerObservabilityWidget::setStatus(const QString &text)
{
    if (m_inspector)
        m_inspector->setStatus(text);
}

void DockerObservabilityWidget::refresh()
{
    if (!m_machine->currentData().toString().isEmpty()) {
        refreshRemote();
        return;
    }
    m_currentRefreshQuiet = false;
    if (m_refreshWatcher && m_refreshWatcher->isRunning()) {
        m_refreshPending = true;
        m_refreshPendingQuiet = false;
        setLoadingState(true, tr("Loading Docker objects…"));
        return;
    }

    setLoadingState(true, tr("Loading Docker objects…"));
    m_localRefreshGeneration = m_generation;
    if (m_refreshWatcher)
        m_refreshWatcher->setFuture(QtConcurrent::run([]() { return DockerObservabilityWidget::collectDockerSnapshot(); }));
}

void DockerObservabilityWidget::refreshQuietly()
{
    if (!m_active || m_monitorPaused)
        return;
    if (!m_machine->currentData().toString().isEmpty()) {
        refreshRemote();
        return;
    }
    if (m_refreshWatcher && m_refreshWatcher->isRunning()) {
        if (!m_refreshPending) {
            m_refreshPending = true;
            m_refreshPendingQuiet = true;
        }
        return;
    }

    m_currentRefreshQuiet = true;
    if (!hasAnyObjects())
        setLoadingState(true, tr("Loading Docker objects…"));
    m_localRefreshGeneration = m_generation;
    if (m_refreshWatcher)
        m_refreshWatcher->setFuture(QtConcurrent::run([]() { return DockerObservabilityWidget::collectDockerSnapshot(); }));
}

void DockerObservabilityWidget::setActive(bool active)
{
    m_active = active;
    if (active) {
        if (!m_monitorPaused && m_autoRefreshTimer && !m_autoRefreshTimer->isActive())
            m_autoRefreshTimer->start();
        if (!m_hasLoadedOnce)
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
        else
            QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refreshQuietly, Qt::QueuedConnection);
        return;
    }

    if (m_autoRefreshTimer)
        m_autoRefreshTimer->stop();
    updateContainerActionState();
    m_pendingContainerDetailId.clear();
    m_pendingNetworkDetailId.clear();
    m_pendingVolumeDetailId.clear();
    m_refreshPending = false;
    m_refreshPendingQuiet = false;
    m_currentRefreshQuiet = false;
    setLoadingState(false);
    setStatus(tr("Docker observability opens cold and refreshes on demand."));
}

void DockerObservabilityWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (isVisible() && m_active && !m_monitorPaused && m_autoRefreshTimer && !m_autoRefreshTimer->isActive())
        m_autoRefreshTimer->start();
    if (!m_hasLoadedOnce && isVisible() && m_active)
        QMetaObject::invokeMethod(this, &DockerObservabilityWidget::refresh, Qt::QueuedConnection);
}

void DockerObservabilityWidget::updateSummary()
{
    if (m_containerCount)
        m_containerCount->setText(tr("Containers %1").arg(m_containers ? m_containers->rowCount() : 0));
    if (m_networkCount)
        m_networkCount->setText(tr("Networks %1").arg(m_networks ? m_networks->rowCount() : 0));
    if (m_volumeCount)
        m_volumeCount->setText(tr("Volumes %1").arg(m_volumes ? m_volumes->rowCount() : 0));
}

void DockerObservabilityWidget::applyContainerFilter()
{
    if (!m_containers)
        return;
    const QString needle = m_search ? m_search->text().trimmed().toCaseFolded() : QString();
    for (int row = 0; row < m_containers->rowCount(); ++row) {
        QStringList values;
        for (int col = 0; col < m_containers->columnCount(); ++col) {
            if (auto *item = m_containers->item(row, col))
                values << item->text();
        }
        const bool hidden = !needle.isEmpty() && !values.join(QLatin1Char(' ')).toCaseFolded().contains(needle);
        m_containers->setRowHidden(row, hidden);
    }
    updateContainerActionState();
}

void DockerObservabilityWidget::updateContainerActionState()
{
    const QString state = selectedContainerState();
    Q_UNUSED(state);
}

bool DockerObservabilityWidget::hasAnyObjects() const
{
    const int containerCount = m_containers ? m_containers->rowCount() : 0;
    const int networkCount = m_networks ? m_networks->rowCount() : 0;
    const int volumeCount = m_volumes ? m_volumes->rowCount() : 0;
    return (containerCount + networkCount + volumeCount) > 0;
}

void DockerObservabilityWidget::setLoadingState(bool loading, const QString &text)
{
    m_loading = loading;
    if (!loading)
        return;
    if (!text.isEmpty())
        setStatus(text);
    if (!hasAnyObjects()) {
        if (m_containerDetails)
            m_containerDetails->setPlainText(tr("Loading Docker containers…"));
        if (m_containerLogs)
            m_containerLogs->setPlainText(tr("Loading container logs…"));
        if (m_networkDetails)
            m_networkDetails->setPlainText(tr("Loading Docker networks…"));
        if (m_volumeDetails)
            m_volumeDetails->setPlainText(tr("Loading Docker volumes…"));
    }
}

QString DockerObservabilityWidget::selectedContainerId() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

QString DockerObservabilityWidget::selectedContainerName() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->text().trimmed();
}

QString DockerObservabilityWidget::selectedContainerState() const
{
    if (!m_containers)
        return {};
    const auto selected = m_containers->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole + 1).toString();
}

QString DockerObservabilityWidget::selectedNetworkId() const
{
    if (!m_networks)
        return {};
    const auto selected = m_networks->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

QString DockerObservabilityWidget::selectedVolumeName() const
{
    if (!m_volumes)
        return {};
    const auto selected = m_volumes->selectedItems();
    if (selected.isEmpty())
        return {};
    return selected.first()->data(Qt::UserRole).toString();
}

void DockerObservabilityWidget::applySnapshot(const DockerSnapshot &snapshot)
{
    m_snapshot = snapshot;
    const QString previousContainerId = selectedContainerId();
    const QString previousNetworkId = selectedNetworkId();
    const QString previousVolumeName = selectedVolumeName();
    int restoredContainerRow = -1;
    int restoredNetworkRow = -1;
    int restoredVolumeRow = -1;

    const QSignalBlocker blockContainers(m_containers);
    const QSignalBlocker blockNetworks(m_networks);
    const QSignalBlocker blockVolumes(m_volumes);

    if (!snapshot.containersOk) {
        replacePlainTextWithoutFlash(m_containerDetails, tr("Could not query docker containers.\n\n%1").arg(snapshot.containersError));
    } else {
        QSet<QString> seenIds;
        for (const ContainerRowData &rowData : snapshot.containers) {
            const QString state = rowData.state;
            seenIds.insert(rowData.id);
            int row = rowForId(m_containers, rowData.id);
            if (row < 0) {
                row = m_containers->rowCount();
                m_containers->insertRow(row);
            }
            setItemText(m_containers, row, 0, rowData.name, rowData.id);
            auto *nameItem = m_containers->item(row, 0);
            nameItem->setData(Qt::UserRole + 1, state);
            nameItem->setIcon(statusIconForState(state, m_containers));

            auto *stateItem = m_containers->item(row, 1);
            if (!stateItem) {
                stateItem = statusItem(QString());
                m_containers->setItem(row, 1, stateItem);
            }
            stateItem->setData(Qt::UserRole + 1, state);
            stateItem->setData(Qt::ToolTipRole, rowData.statusText);
            auto *statePill = makeStatusPill(rowData.statusText, state, m_containers);
            statePill->ensurePolished();
            stateItem->setSizeHint(statePill->sizeHint() + QSize(20, 0));
            m_containers->setCellWidget(row, 1, statePill);
            setItemText(m_containers, row, 2, rowData.image);
            setItemText(m_containers, row, 3, rowData.ports);
            setItemText(m_containers, row, 4, rowData.created);
            if (!previousContainerId.isEmpty() && rowData.id == previousContainerId)
                restoredContainerRow = row;
        }
        removeRowsNotIn(m_containers, seenIds);
        if (snapshot.containers.isEmpty())
            replacePlainTextWithoutFlash(m_containerDetails, tr("No Docker containers found. This view includes stopped containers too."));
    }

    if (!snapshot.networksOk) {
        replacePlainTextWithoutFlash(m_networkDetails, tr("Could not query docker networks.\n\n%1").arg(snapshot.networksError));
    } else {
        QSet<QString> seenIds;
        for (const NetworkRowData &rowData : snapshot.networks) {
            seenIds.insert(rowData.id);
            int row = rowForId(m_networks, rowData.id);
            if (row < 0) {
                row = m_networks->rowCount();
                m_networks->insertRow(row);
            }
            setItemText(m_networks, row, 0, rowData.name, rowData.id);
            setItemText(m_networks, row, 1, rowData.driver);
            setItemText(m_networks, row, 2, rowData.scope);
            if (!previousNetworkId.isEmpty() && rowData.id == previousNetworkId)
                restoredNetworkRow = row;
        }
        removeRowsNotIn(m_networks, seenIds);
    }

    if (!snapshot.volumesOk) {
        replacePlainTextWithoutFlash(m_volumeDetails, tr("Could not query docker volumes.\n\n%1").arg(snapshot.volumesError));
    } else {
        QSet<QString> seenIds;
        for (const VolumeRowData &rowData : snapshot.volumes) {
            seenIds.insert(rowData.name);
            int row = rowForId(m_volumes, rowData.name);
            if (row < 0) {
                row = m_volumes->rowCount();
                m_volumes->insertRow(row);
            }
            setItemText(m_volumes, row, 0, rowData.name, rowData.name);
            setItemText(m_volumes, row, 1, rowData.driver);
            setItemText(m_volumes, row, 2, rowData.mountpoint);
            if (!previousVolumeName.isEmpty() && rowData.name == previousVolumeName)
                restoredVolumeRow = row;
        }
        removeRowsNotIn(m_volumes, seenIds);
    }

    if (!previousContainerId.isEmpty())
        restoredContainerRow = rowForId(m_containers, previousContainerId);
    if (!previousNetworkId.isEmpty())
        restoredNetworkRow = rowForId(m_networks, previousNetworkId);
    if (!previousVolumeName.isEmpty())
        restoredVolumeRow = rowForId(m_volumes, previousVolumeName);

    if (restoredContainerRow >= 0)
        m_containers->selectRow(restoredContainerRow);
    if (restoredNetworkRow >= 0)
        m_networks->selectRow(restoredNetworkRow);
    if (restoredVolumeRow >= 0)
        m_volumes->selectRow(restoredVolumeRow);

    applyContainerFilter();
    updateSummary();
    updateContainerActionState();
    QStringList errors;
    if (!snapshot.containersOk)
        errors.append(tr("Containers: %1").arg(snapshot.containersError.left(600)));
    if (!snapshot.networksOk)
        errors.append(tr("Networks: %1").arg(snapshot.networksError.left(600)));
    if (!snapshot.volumesOk)
        errors.append(tr("Volumes: %1").arg(snapshot.volumesError.left(600)));
    const int totalObjects = snapshot.containers.size() + snapshot.networks.size() + snapshot.volumes.size();
    if (!errors.isEmpty()) {
        setStatus(errors.join("\n"));
    } else if (totalObjects > 0) {
        setStatus(tr("Docker observability refreshed."));
    } else if (snapshot.containersOk && snapshot.networksOk && snapshot.volumesOk) {
        setStatus(tr("No Docker objects found."));
    } else {
        setStatus(tr("Docker refresh completed with errors."));
    }
    m_loading = false;
    updateGraph(snapshot);
    updateInspector();

    if (restoredContainerRow >= 0 && m_selectedResource.startsWith("container:"))
        refreshContainerDetailsAsync(previousContainerId);
    if (restoredNetworkRow >= 0 && m_selectedResource.startsWith("network:"))
        refreshNetworkDetailsAsync(previousNetworkId);
    if (restoredVolumeRow >= 0 && m_selectedResource.startsWith("volume:"))
        refreshVolumeDetailsAsync(previousVolumeName);
}

void DockerObservabilityWidget::renderContainerDetails(const QString &containerId)
{
    refreshContainerDetailsAsync(containerId);
}

void DockerObservabilityWidget::renderNetworkDetails(const QString &networkId)
{
    refreshNetworkDetailsAsync(networkId);
}

void DockerObservabilityWidget::renderVolumeDetails(const QString &volumeName)
{
    refreshVolumeDetailsAsync(volumeName);
}

void DockerObservabilityWidget::refreshContainerDetailsAsync(const QString &containerId)
{
    if (!m_machine->currentData().toString().isEmpty()) {
        m_containerLogs->setPlainText(tr("Remote monitoring shares inventory and topology. Application logs and interactive controls remain local."));
        return;
    }

    if (containerId.isEmpty())
        return;
    const bool sameVisibleContainer = containerId == m_displayedContainerDetailId;
    m_pendingContainerDetailId = containerId;
    if (!sameVisibleContainer && m_containerDetails)
        m_containerDetails->setPlainText(tr("Loading container details…"));
    if (!sameVisibleContainer && m_containerLogs)
        m_containerLogs->setPlainText(tr("Loading container logs…"));
    if (m_containerDetailWatcher->isRunning())
        return;
    if (m_containerDetailWatcher) {
        m_containerDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_containerDetailWatcher->setFuture(QtConcurrent::run([containerId]() {
            ContainerDetailSnapshot snapshot;
            snapshot.containerId = containerId;
            const CommandResult inspect = DockerObservabilityWidget::runDocker({QStringLiteral("inspect"), containerId});
            if (!inspect.ok) {
                snapshot.inspectText = QObject::tr("Could not inspect container.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.inspectText = prettyJson(inspect.stdoutText);
            }
            const CommandResult logs =
                DockerObservabilityWidget::runDocker({QStringLiteral("logs"), QStringLiteral("--tail"), QStringLiteral("200"), containerId}, true);
            if (!logs.ok) {
                snapshot.logsText = QObject::tr("Could not load docker logs.\n\n%1").arg(logs.stderrText.trimmed());
            } else {
                snapshot.logsText = logs.stdoutText.trimmed();
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshNetworkDetailsAsync(const QString &networkId)
{
    if (!m_machine->currentData().toString().isEmpty()) {
        m_containerLogs->setPlainText(tr("Remote monitoring shares inventory and topology. Application logs and interactive controls remain local."));
        return;
    }

    if (networkId.isEmpty())
        return;
    m_pendingNetworkDetailId = networkId;
    if (m_networkDetails)
        m_networkDetails->setPlainText(tr("Loading network details…"));
    if (m_networkDetailWatcher->isRunning())
        return;
    if (m_networkDetailWatcher) {
        m_networkDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_networkDetailWatcher->setFuture(QtConcurrent::run([networkId]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = networkId;
            const CommandResult inspect =
                DockerObservabilityWidget::runDocker({QStringLiteral("network"), QStringLiteral("inspect"), networkId});
            if (!inspect.ok) {
                snapshot.detailText = QObject::tr("Could not inspect network.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = prettyJson(inspect.stdoutText);
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshVolumeDetailsAsync(const QString &volumeName)
{
    if (!m_machine->currentData().toString().isEmpty()) {
        m_containerLogs->setPlainText(tr("Remote monitoring shares inventory and topology. Application logs and interactive controls remain local."));
        return;
    }

    if (volumeName.isEmpty())
        return;
    m_pendingVolumeDetailId = volumeName;
    if (m_volumeDetails)
        m_volumeDetails->setPlainText(tr("Loading volume details…"));
    if (m_volumeDetailWatcher->isRunning())
        return;
    if (m_volumeDetailWatcher) {
        m_volumeDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_volumeDetailWatcher->setFuture(QtConcurrent::run([volumeName]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = volumeName;
            const CommandResult inspect =
                DockerObservabilityWidget::runDocker({QStringLiteral("volume"), QStringLiteral("inspect"), volumeName});
            if (!inspect.ok) {
                snapshot.detailText = QObject::tr("Could not inspect volume.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = prettyJson(inspect.stdoutText);
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::onContainersSelectionChanged()
{
    updateContainerActionState();
    const QString containerId = selectedContainerId();
    if (containerId.isEmpty())
        return;
    m_selectedResource = "container:" + containerId;
    updateInspector();
    renderContainerDetails(containerId);
}

void DockerObservabilityWidget::onContainersContextMenuRequested(const QPoint &pos)
{
    if (!m_containers)
        return;
    if (auto *item = m_containers->itemAt(pos))
        m_containers->selectRow(item->row());
    const QString containerId = selectedContainerId();
    if (containerId.isEmpty())
        return;

    updateContainerActionState();
    const QString state = selectedContainerState();
    const QString name = selectedContainerName();

    QMenu menu(this);
    menu.setObjectName(QStringLiteral("dockerContextMenu"));

    QAction *inspectAction = menu.addAction(style()->standardIcon(QStyle::SP_FileDialogDetailedView),
                                            tr("Inspect %1").arg(name));
    QAction *startAction = menu.addAction(style()->standardIcon(QStyle::SP_MediaPlay), tr("Start container"));
    QAction *stopAction = menu.addAction(style()->standardIcon(QStyle::SP_MediaStop), tr("Stop container"));
    menu.addSeparator();
    QAction *refreshAction = menu.addAction(style()->standardIcon(QStyle::SP_BrowserReload), tr("Refresh Docker view"));

    startAction->setEnabled(state == QStringLiteral("exited") || state == QStringLiteral("created"));
    stopAction->setEnabled(state == QStringLiteral("healthy") || state == QStringLiteral("unhealthy") || state == QStringLiteral("running")
                           || state == QStringLiteral("paused") || state == QStringLiteral("restarting"));
    if (!m_machine->currentData().toString().isEmpty()) {
        startAction->setEnabled(false);
        stopAction->setEnabled(false);
    }

    QAction *chosen = menu.exec(m_containers->viewport()->mapToGlobal(pos));
    if (chosen == inspectAction) {
        renderContainerDetails(containerId);
    } else if (chosen == startAction) {
        onStartSelectedContainer();
    } else if (chosen == stopAction) {
        onStopSelectedContainer();
    } else if (chosen == refreshAction) {
        if (!m_machine->currentData().toString().isEmpty())
            refreshRemote(true);
        else
            refresh();
    }
}

void DockerObservabilityWidget::onNetworksSelectionChanged()
{
    const auto selected = m_networks->selectedItems();
    if (selected.isEmpty())
        return;
    const QString networkId = selected.first()->data(Qt::UserRole).toString();
    if (!networkId.isEmpty()) {
        m_selectedResource = "network:" + networkId;
        updateInspector();
        renderNetworkDetails(networkId);
    }
}

void DockerObservabilityWidget::onVolumesSelectionChanged()
{
    const auto selected = m_volumes->selectedItems();
    if (selected.isEmpty())
        return;
    const QString volumeName = selected.first()->data(Qt::UserRole).toString();
    if (!volumeName.isEmpty()) {
        m_selectedResource = "volume:" + volumeName;
        updateInspector();
        renderVolumeDetails(volumeName);
    }
}

void DockerObservabilityWidget::onContainerSearchChanged(const QString &)
{
    applyContainerFilter();
}

void DockerObservabilityWidget::onStartSelectedContainer()
{
    runContainerAction("start");
}

void DockerObservabilityWidget::onStopSelectedContainer()
{
    runContainerAction("stop");
}

void DockerObservabilityWidget::runContainerAction(const QString &action)
{
    if (!m_machine->currentData().toString().isEmpty())
        return;
    const QString id = selectedContainerId();
    const QString name = selectedContainerName();
    if (id.isEmpty() || m_containerActionBusy || !m_selectedResource.startsWith("container:"))
        return;
    m_containerActionBusy = true;
    updateInspector();
    setStatus(tr("%1: %2…").arg(name, action));
    auto *watcher = new QFutureWatcher<CommandResult>(this);
    const quint64 generation = m_generation;
    connect(watcher, &QFutureWatcher<CommandResult>::finished, this, [this, watcher, name, action, generation]() {
        const auto result = watcher->result();
        watcher->deleteLater();
        m_containerActionBusy = false;
        updateInspector();
        if (generation != m_generation)
            return;
        if (!result.ok) {
            setStatus(tr("%1 failed for %2: %3").arg(action, name, result.stderrText.trimmed()));
            return;
        }
        refresh();
    });
    watcher->setFuture(QtConcurrent::run([action, id]() { return runDocker({action, id}); }));
}

void DockerObservabilityWidget::updateGraph(const DockerSnapshot &snapshot)
{
    QVector<ResourceNode> nodes;
    QVector<ResourceEdge> edges;
    QMap<QString, QString> containerNames;
    QMap<QString, QString> networks;
    QMap<QString, QStringList> services;
    for (const auto &container : snapshot.containers) {
        const QString key = "container:" + container.id;
        containerNames.insert(container.name, key);
        ResourceNode node{key, "container", container.name, container.image + " · " + container.ports,
                          container.statusText, 1};
        node.motionEligible = container.state == "running" || container.state == "healthy"
            || container.state == "unhealthy";
        QString label = tr("Unknown");
        if (container.state == "running" || container.state == "healthy") {
            label = container.state == "healthy" ? tr("Running · Healthy") : tr("Running");
            node.status = ResourceStatus::Running;
        } else if (container.state == "unhealthy") {
            label = tr("Running · Unhealthy");
            node.status = ResourceStatus::Error;
        } else if (container.state == "paused" || container.state == "restarting") {
            label = container.state == "paused" ? tr("Paused") : tr("Restarting");
            node.status = ResourceStatus::Warning;
        } else if (container.state == "exited" || container.state == "created" || container.state == "dead") {
            label = tr("Stopped");
            node.status = ResourceStatus::Inactive;
            node.inactive = true;
            if (container.state == "created")
                label = tr("Not started");
            else if (container.state == "dead") {
                label = tr("Stopped · Dead");
                node.status = ResourceStatus::Error;
            }
        }
        node.state = label + " · " + container.statusText;
        nodes.append(node);
    }
    for (const auto &network : snapshot.networks) {
        const QString key = "network:" + network.id;
        networks.insert(network.name, key);
        nodes.append({key, "network", network.name, network.driver, network.scope, 0});
    }
    for (const auto &volume : snapshot.volumes)
        nodes.append({"volume:" + volume.name, "volume", volume.name, volume.driver, volume.mountpoint, 2});
    for (const auto &value : snapshot.topology) {
        const auto object = value.toObject();
        const auto labels = object.value("Labels").toObject();
        const QString project = labels.value("com.docker.compose.project").toString();
        const QString service = labels.value("com.docker.compose.service").toString();
        QString name = object.value("Name").toString();
        if (name.startsWith('/'))
            name.remove(0, 1);
        if (!project.isEmpty() && !service.isEmpty() && containerNames.contains(name))
            services[project + "/" + service].append(containerNames[name]);
    }
    bool limited = false;
    int unresolved = 0;
    const auto addEdge = [&](const ResourceEdge &edge) {
        if (edges.size() < 4096)
            edges.append(edge);
        else
            limited = true;
    };
    for (const auto &value : snapshot.topology) {
        const auto object = value.toObject();
        QString name = object.value("Name").toString();
        if (name.startsWith('/'))
            name.remove(0, 1);
        const QString container = containerNames.value(name);
        if (container.isEmpty())
            continue;
        const auto labels = object.value("Labels").toObject();
        const QString project = labels.value("com.docker.compose.project").toString();
        const auto dependencies = labels.value("com.docker.compose.depends_on").toString().split(',', Qt::SkipEmptyParts);
        for (const auto &dependency : dependencies) {
            const auto targets = services.value(project + "/" + dependency.section(':', 0, 0));
            if (targets.isEmpty())
                ++unresolved;
            for (const auto &target : targets)
                addEdge({container, target, tr("Depends on %1 (%2)").arg(dependency.section(':', 0, 0), dependency.section(':', 1, 1)), true});
        }
        const auto attachedNetworks = object.value("Networks").toObject();
        for (auto it = attachedNetworks.begin(); it != attachedNetworks.end(); ++it) {
            if (networks.contains(it.key()))
                addEdge({networks[it.key()], container, tr("Network attachment: %1").arg(it.key()), false});
        }
        for (const auto &mountValue : object.value("Mounts").toArray()) {
            const auto mount = mountValue.toObject();
            if (mount.value("Type").toString() == "volume")
                addEdge({container, "volume:" + mount.value("Name").toString(),
                         tr("Mounted at %1").arg(mount.value("Destination").toString()), false});
        }
    }
    QString notice = snapshot.topologyNotice;
    if (!snapshot.containersOk || !snapshot.networksOk || !snapshot.volumesOk)
        notice += tr(" Docker inventory is incomplete. Inspector contains the query errors.");
    if (unresolved)
        notice += tr(" %1 declared dependencies are outside this snapshot.").arg(unresolved);
    if (limited)
        notice += tr(" Relationship display limited to 4,096 links; inspect data remains available.");
    m_canvas->setGraph(nodes, edges, notice);
}
