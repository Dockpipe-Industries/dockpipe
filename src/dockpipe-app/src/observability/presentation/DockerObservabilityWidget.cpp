#include "DockerObservabilityWidget.h"
#include <QCoreApplication>
#include "MonitoringMachineSelector.h"
#include "DockpipeChoices.h"
#include "CanvasIcons.h"
#include "DockerResourceInspector.h"
#include "SetupTerminal.h"
#include <QDialog>
#include <QButtonGroup>
#include <QSettings>
#include <QComboBox>
#include <QResizeEvent>
#include <QScrollArea>
#include <QStyledItemDelegate>
#include <QPainter>
#include <QStackedWidget>
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
#include <QRegularExpression>
#include <QPushButton>
#include <QScrollBar>
#include <QShowEvent>
#include <QHideEvent>
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

QString inspectedObjectText(const QString &response)
{
    const auto receipt = QJsonDocument::fromJson(response.toUtf8()).object();
    return QString::fromUtf8(QJsonDocument(receipt.value("inspect").toArray()).toJson(QJsonDocument::Indented));
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

class ContainerStateDelegate final : public QStyledItemDelegate {
public:
    explicit ContainerStateDelegate(QObject *parent) : QStyledItemDelegate(parent) {}

    void paint(QPainter *painter, const QStyleOptionViewItem &option, const QModelIndex &index) const override
    {
        QStyledItemDelegate::paint(painter, option, index);
        const QString state = index.data(Qt::UserRole + 1).toString();
        const bool dark = option.palette.color(QPalette::Base).lightness() < 128;
        QColor color(dark ? "#a8b6c8" : "#596b82");
        if (state == "running" || state == "healthy")
            color = QColor(dark ? "#83d4b0" : "#226e4c");
        else if (state == "unhealthy" || state == "dead")
            color = QColor(dark ? "#f19ba4" : "#ab3544");
        else if (state == "paused" || state == "restarting")
            color = QColor(dark ? "#e9c078" : "#896115");

        painter->save();
        painter->setRenderHint(QPainter::Antialiasing);
        QFont font = option.font;
        font.setPixelSize(11);
        font.setWeight(QFont::DemiBold);
        painter->setFont(font);
        const QString text = index.data(Qt::AccessibleTextRole).toString();
        const int width = qMin(QFontMetrics(font).horizontalAdvance(text) + 20, option.rect.width() - 12);
        const QRect badge(option.rect.left() + 8, option.rect.center().y() - 12, width, 24);
        QColor background = color;
        background.setAlpha(dark ? 32 : 22);
        painter->setBrush(background);
        painter->setPen(Qt::NoPen);
        painter->drawRoundedRect(badge, 5, 5);
        painter->setPen(color);
        painter->drawText(badge, Qt::AlignCenter, text);
        painter->restore();
    }
};

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
        if (m_localRefreshGeneration == m_generation && m_active && !m_monitorPaused) {
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
        if (m_containerDetailWatcher->property("generation").toULongLong() != m_generation || !m_active || m_monitorPaused)
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
        if (m_networkDetailWatcher->property("generation").toULongLong() != m_generation || !m_active || m_monitorPaused)
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
        if (m_volumeDetailWatcher->property("generation").toULongLong() != m_generation || !m_active || m_monitorPaused)
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
    auto *refreshButton = findChild<QPushButton *>("dockerRefresh");
    if (refreshButton) {
        connect(&m_remoteCommand, &RemoteCommand::busyChanged, refreshButton,
                [refreshButton](bool busy) { refreshButton->setEnabled(!busy); });
    }
}

void DockerObservabilityWidget::buildUi()
{
    setObjectName("dockerObservabilityPage");
    setAttribute(Qt::WA_StyledBackground, true);
    m_inventory = new QDialog(this);
    m_inventory->setWindowTitle(tr("Docker resources"));
    m_inventory->resize(900, 560);
    auto *inventoryLayout = new QVBoxLayout(m_inventory);
    m_tabs = new QTabWidget;
    m_tabs->setObjectName("dockerInventoryTabs");
    m_tabs->addTab(buildContainersPage(), tr("Containers"));
    m_tabs->addTab(buildNetworksPage(), tr("Networks"));
    m_tabs->addTab(buildVolumesPage(), tr("Volumes"));
    inventoryLayout->addWidget(m_tabs);
    // The full inventory is available on demand, never duplicated in the inspector.
    m_containerDetails->parentWidget()->hide();
    m_inspector = new DockerResourceInspector(m_containerLogs,
        {m_containerDetails, m_networkDetails, m_volumeDetails});
    m_canvasInspectorHost = new QWidget;
    auto *inspectorLayout = new QVBoxLayout(m_canvasInspectorHost);
    inspectorLayout->setContentsMargins(0, 0, 0, 0);
    inspectorLayout->addWidget(m_inspector);
    m_canvas = new ResourceCanvas("docker", tr("Containers"),
        tr("Solid arrows: declared dependencies · Dashed lines: network or storage attachments"), m_canvasInspectorHost, this, false);
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
    m_machineLabel = new QLabel(tr("Machine"));
    machineBar->addWidget(m_machineLabel);
    machineBar->addWidget(m_machine);
    machineBar->addStretch();
    auto *viewButtons = new QButtonGroup(this);
    auto *dashboardButton = new QPushButton(tr("List"));
    dashboardButton->setObjectName("dockerDashboardView");
    dashboardButton->setAccessibleName(tr("Containers list view"));
    dashboardButton->setCheckable(true);
    auto *canvasButton = new QPushButton(tr("Canvas"));
    canvasButton->setObjectName("dockerCanvasView");
    canvasButton->setAccessibleName(tr("Containers canvas view"));
    canvasButton->setCheckable(true);
    viewButtons->addButton(dashboardButton);
    viewButtons->addButton(canvasButton);
    auto *viewSwitcher = new QFrame;
    viewSwitcher->setObjectName("dockerViewSwitcher");
    auto *viewLayout = new QHBoxLayout(viewSwitcher);
    viewLayout->setContentsMargins(3, 3, 3, 3);
    viewLayout->setSpacing(2);
    canvasButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Branch));
    dashboardButton->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::List));
    viewLayout->addWidget(canvasButton);
    viewLayout->addWidget(dashboardButton);
    machineBar->addWidget(viewSwitcher);
    canvasLayout->addLayout(machineBar);
    connect(m_machine, qOverload<int>(&QComboBox::currentIndexChanged), this, [this]() {
        if (m_featureTarget.isEmpty())
            changeMachine();
    });
    m_views = new QStackedWidget;
    m_views->addWidget(m_canvas);
    m_dashboard = buildListDashboard();
    auto *dashboardScroll = new QScrollArea;
    dashboardScroll->setFrameShape(QFrame::NoFrame);
    dashboardScroll->setWidgetResizable(true);
    dashboardScroll->setWidget(m_dashboard);
    m_dashboardScroll = dashboardScroll;
    m_views->addWidget(dashboardScroll);
    canvasLayout->addWidget(m_views);
    updateSummary();
    connect(m_tabs, &QTabWidget::currentChanged, this, [this](int index) {
        m_search->setVisible(index == 0);
        m_stateFilter->setVisible(index == 0);
        const QStringList ids{selectedContainerId(), selectedNetworkId(), selectedVolumeName()};
        const QStringList kinds{"container", "network", "volume"};
        if (index < 0 || index >= ids.size())
            return;
        m_selectedResource.clear();
        if (!ids[index].isEmpty())
            selectResource(kinds[index] + ":" + ids[index]);
        else
            updateInspector();
        updateListResults();
    });
    connect(dashboardButton, &QPushButton::clicked, this, [this]() { setDashboardView(true); });
    connect(canvasButton, &QPushButton::clicked, this, [this]() { setDashboardView(false); });
    const bool dashboard = QSettings().value("docker/viewMode", "canvas").toString() == "dashboard";
    dashboardButton->setChecked(dashboard);
    canvasButton->setChecked(!dashboard);
    setDashboardView(dashboard);
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, &DockerObservabilityWidget::selectResource);
    for (auto *table : {m_containers, m_networks, m_volumes}) {
        connect(table, &QTableWidget::cellDoubleClicked, this, [this](int, int) {
            if (m_dashboardView)
                return;
            m_inventory->accept();
            if (!m_canvas->focusResource(m_selectedResource))
                m_canvas->showDetails();
        });
    }
    connect(m_inspector, &DockerResourceInspector::resourceRequested, this, [this](const QString &encoded) {
        const QString key = QUrl::fromPercentEncoding(encoded.toUtf8());
        if (m_dashboardView) {
            selectResource(key);
        } else if (!m_canvas->focusResource(key)) {
            selectResource(key);
            m_canvas->showDetails();
        }
    });
    connect(m_inspector, &DockerResourceInspector::refreshRequested, this, [this]() {
        if (isRemoteTarget())
            refreshRemote(true);
        else
            refresh();
    });
    connect(m_inspector, &DockerResourceInspector::pauseChanged, this, [this](bool paused) {
        m_monitorPaused = paused;
        if (paused) {
            cancelFeatureRead();
            m_remoteCommand.cancel();
            m_autoRefreshTimer->stop();
        } else if (m_active) {
            m_autoRefreshTimer->start();
            refresh();
        }
    });
    connect(m_inspector, &DockerResourceInspector::inventoryRequested, this, [this]() {
        if (m_dashboardView)
            m_tabs->setFocus();
        else
            m_inventory->show();
    });
    connect(m_inspector, &DockerResourceInspector::startRequested, this, &DockerObservabilityWidget::onStartSelectedContainer);
    connect(m_inspector, &DockerResourceInspector::stopRequested, this, &DockerObservabilityWidget::onStopSelectedContainer);
    connect(m_inspector, &DockerResourceInspector::execRequested, this, &DockerObservabilityWidget::openExec);
}

QWidget *DockerObservabilityWidget::buildListDashboard()
{
    auto *page = new QWidget;
    page->setObjectName("dockerDashboard");
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(24, 16, 24, 24);
    layout->setSpacing(20);

    auto *header = new QHBoxLayout;
    auto *titles = new QVBoxLayout;
    titles->setSpacing(4);
    auto *heading = new QLabel(tr("Containers"));
    heading->setObjectName("dockerPageTitle");
    titles->addWidget(heading);
    auto *description = new QLabel(tr("Manage containers, networks, and persistent storage."));
    description->setObjectName("dockerPageDescription");
    titles->addWidget(description);
    header->addLayout(titles, 1);
    m_listMonitoringHost = new QWidget;
    auto *monitoringLayout = new QHBoxLayout(m_listMonitoringHost);
    monitoringLayout->setContentsMargins(0, 0, 0, 0);
    header->addWidget(m_listMonitoringHost);
    layout->addLayout(header);

    auto *summary = new QHBoxLayout;
    summary->setSpacing(12);
    const auto addMetric = [summary](const QString &title, const QString &caption, QLabel *&value) {
        auto *card = new QFrame;
        card->setObjectName("dockerSummaryCard");
        auto *cardLayout = new QVBoxLayout(card);
        cardLayout->setContentsMargins(18, 14, 18, 14);
        cardLayout->setSpacing(4);
        auto *label = new QLabel(title);
        label->setObjectName("dockerMetricTitle");
        cardLayout->addWidget(label);
        value = new QLabel(QStringLiteral("—"));
        value->setObjectName("dockerMetricValue");
        cardLayout->addWidget(value);
        auto *hint = new QLabel(caption);
        hint->setObjectName("dockerMetricCaption");
        cardLayout->addWidget(hint);
        summary->addWidget(card, 1);
    };
    addMetric(tr("Containers"), tr("Across all states"), m_containerCount);
    addMetric(tr("Running"), tr("Currently active"), m_runningCount);
    addMetric(tr("Networks"), tr("Available connections"), m_networkCount);
    addMetric(tr("Volumes"), tr("Persistent storage"), m_volumeCount);
    layout->addLayout(summary);

    m_dashboardSplitter = new QSplitter(Qt::Horizontal);
    m_dashboardSplitter->setObjectName("dockerListSplitter");
    m_dashboardSplitter->setChildrenCollapsible(false);
    m_dashboardSplitter->setHandleWidth(12);
    auto *inventory = new QFrame;
    inventory->setObjectName("dockerInventoryCard");
    inventory->setMinimumHeight(400);
    auto *inventoryLayout = new QVBoxLayout(inventory);
    inventoryLayout->setContentsMargins(16, 16, 16, 12);
    inventoryLayout->setSpacing(16);
    auto *toolbar = new QHBoxLayout;
    toolbar->setSpacing(12);
    m_search = new QLineEdit;
    m_search->setObjectName("dockerContainerSearch");
    m_search->setPlaceholderText(tr("Search by name, image, or port…"));
    m_search->setAccessibleName(tr("Search containers"));
    m_search->setClearButtonEnabled(true);
    m_search->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Search), QLineEdit::LeadingPosition);
    toolbar->addWidget(m_search, 1);
    m_stateFilter = new QComboBox;
    m_stateFilter->setObjectName("dockerStateFilter");
    m_stateFilter->setAccessibleName(tr("Filter containers by state"));
    m_stateFilter->addItem(tr("All states"), "all");
    m_stateFilter->addItem(tr("Running"), "running");
    m_stateFilter->addItem(tr("Stopped"), "stopped");
    m_stateFilter->addItem(tr("Needs attention"), "attention");
    toolbar->addWidget(m_stateFilter);
    inventoryLayout->addLayout(toolbar);
    m_listInventoryHost = new QWidget;
    auto *inventoryHostLayout = new QVBoxLayout(m_listInventoryHost);
    inventoryHostLayout->setContentsMargins(0, 0, 0, 0);
    inventoryLayout->addWidget(m_listInventoryHost, 1);
    m_listEmptyState = new QLabel(page);
    m_listEmptyState->setObjectName("dockerListEmptyState");
    m_listEmptyState->setAlignment(Qt::AlignCenter);
    m_listEmptyState->setWordWrap(true);
    m_listEmptyState->setMinimumHeight(140);
    m_listEmptyState->hide();
    m_listResults = new QLabel;
    m_listResults->setObjectName("dockerListResults");
    inventoryLayout->addWidget(m_listResults);
    m_dashboardSplitter->addWidget(inventory);
    m_dashboardSplitter->setStretchFactor(0, 3);
    layout->addWidget(m_dashboardSplitter, 1);
    connect(m_search, &QLineEdit::textChanged, this, &DockerObservabilityWidget::onContainerSearchChanged);
    connect(m_stateFilter, qOverload<int>(&QComboBox::currentIndexChanged), this, [this]() {
        applyContainerFilter();
    });
    return page;
}

void DockerObservabilityWidget::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    if (m_dashboardView)
        updateDashboardLayout();
}

void DockerObservabilityWidget::updateDashboardLayout()
{
    // Below this width the table and a readable 360 px inspector no longer fit side by side.
    const Qt::Orientation orientation = width() < 1180 ? Qt::Vertical : Qt::Horizontal;
    if (m_dashboardSplitter->orientation() != orientation || m_dashboardSplitter->sizes().value(1) == 0) {
        m_dashboardSplitter->setOrientation(orientation);
        m_dashboardSplitter->setSizes(orientation == Qt::Horizontal ? QList<int>{800, 420} : QList<int>{300, 400});
    }
}

void DockerObservabilityWidget::updateListResults()
{
    for (auto *table : {m_containers, m_networks, m_volumes})
        table->parentWidget()->show();
    m_listEmptyState->hide();
    if (!m_dashboardView)
        return;
    const QList<QTableWidget *> tables{m_containers, m_networks, m_volumes};
    const int index = m_tabs->currentIndex();
    if (index < 0 || index >= tables.size())
        return;
    QTableWidget *table = tables[index];
    int visible = 0;
    for (int row = 0; row < table->rowCount(); ++row) {
        if (!table->isRowHidden(row))
            ++visible;
    }
    const QStringList names{tr("containers"), tr("networks"), tr("volumes")};
    m_listResults->setText(tr("%1 of %2 %3").arg(visible).arg(table->rowCount()).arg(names[index]));
    if (visible == 0) {
        QWidget *page = m_tabs->currentWidget();
        if (m_listEmptyState->parentWidget() != page)
            page->layout()->addWidget(m_listEmptyState);
        m_listEmptyState->setText(table->rowCount() > 0
            ? tr("No matching containers\nTry a different search or state filter.")
            : tr("No resources to display\nRefresh to load inventory from the selected machine."));
        table->parentWidget()->hide();
        m_listEmptyState->show();
    }
}

void DockerObservabilityWidget::setDashboardView(bool dashboard)
{
    m_dashboardView = dashboard;
    m_inventory->hide();
    if (dashboard) {
        m_listInventoryHost->layout()->addWidget(m_tabs);
        m_dashboardSplitter->addWidget(m_inspector);
        m_views->setCurrentWidget(m_dashboardScroll);
        updateDashboardLayout();
    } else {
        m_inventory->layout()->addWidget(m_tabs);
        m_canvasInspectorHost->layout()->addWidget(m_inspector);
        m_views->setCurrentWidget(m_canvas);
    }
    m_inspector->setListPresentation(dashboard, m_listMonitoringHost);
    m_tabs->show();
    m_inspector->show();
    applyContainerFilter();
    updateInspector();
    QSettings().setValue("docker/viewMode", dashboard ? "dashboard" : "canvas");
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
    const QStringList kinds{"container", "network", "volume"};
    {
        const QSignalBlocker blocker(m_tabs);
        m_tabs->setCurrentIndex(kinds.indexOf(kind));
    }
    m_search->setVisible(kind == "container");
    m_stateFilter->setVisible(kind == "container");
    updateListResults();
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
        cancelFeatureDetails();
        selection.kind = -1;
        selection.status = m_selectedResource.isEmpty()
            ? tr("Choose a resource in the table or on the canvas.") : tr("This resource is no longer in the current inventory.");
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
    if (isRemoteTarget())
        selection.canExec = selection.canExec && m_featureCatalog.value("docker_exec").toBool()
            && !m_featureCatalog.value("terminal_epoch").toString().isEmpty();
    if (!canControlTarget()) {
        selection.canStart = false;
        selection.canStop = false;
    }
    if (m_containerActionBusy) {
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
    selection.relatedHtml = related.isEmpty() ? tr("No linked resources.")
        : tr("Related: ") + related.join(" &nbsp; · &nbsp; ");
    m_inspector->setSelection(selection);
}

void DockerObservabilityWidget::openExec(const QString &shell)
{
    if (!m_selectedResource.startsWith("container:") || (shell != "/bin/sh" && shell != "/bin/bash"))
        return;
    const QString state = selectedContainerState();
    if (state != "running" && state != "healthy" && state != "unhealthy")
        return;
    if (isRemoteTarget()) {
        if (m_featureCatalog.value("docker_exec").toBool()
            && m_featureCatalog.value("target").toObject() == m_featureTarget
            && !m_featureCatalog.value("terminal_epoch").toString().isEmpty())
            emit remoteExecRequested(m_featureCatalog, selectedContainerId(), shell);
        return;
    }
    QString error;
    const QString binary = DockpipeChoices::operatorDockpipeBinary(&error);
    if (binary.isEmpty() || !openDockpipeTerminal(binary,
            {"containers", "exec", "--container", selectedContainerId(), "--shell", shell}, &error))
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
    // Keep the header and at least three 48 px rows readable in a stacked layout.
    m_containers->setMinimumHeight(200);
    m_containers->setColumnCount(5);
    m_containers->setItemDelegateForColumn(1, new ContainerStateDelegate(m_containers));
    m_containers->setHorizontalHeaderLabels({tr("Name"), tr("State"), tr("Image"), tr("Ports"), tr("Created")});
    m_containers->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_containers->horizontalHeader()->setStretchLastSection(false);
    m_containers->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Interactive);
    m_containers->setColumnWidth(0, 220);
    m_containers->horizontalHeader()->setSectionResizeMode(1, QHeaderView::ResizeToContents);
    m_containers->horizontalHeader()->setSectionResizeMode(2, QHeaderView::Stretch);
    m_containers->horizontalHeader()->setSectionResizeMode(3, QHeaderView::Interactive);
    m_containers->setColumnWidth(3, 150);
    m_containers->horizontalHeader()->setSectionResizeMode(4, QHeaderView::ResizeToContents);
    m_containers->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_containers->setSelectionMode(QAbstractItemView::SingleSelection);
    m_containers->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_containers->setAlternatingRowColors(true);
    m_containers->setShowGrid(false);
    m_containers->verticalHeader()->setVisible(false);
    m_containers->verticalHeader()->setDefaultSectionSize(48);
    m_containers->setWordWrap(false);
    m_containers->setTextElideMode(Qt::ElideRight);
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
    // Keep the header and at least three 48 px rows readable in a stacked layout.
    m_networks->setMinimumHeight(200);
    m_networks->setColumnCount(3);
    m_networks->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Scope")});
    m_networks->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_networks->horizontalHeader()->setStretchLastSection(true);
    m_networks->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_networks->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_networks->setSelectionMode(QAbstractItemView::SingleSelection);
    m_networks->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_networks->setAlternatingRowColors(true);
    m_networks->setShowGrid(false);
    m_networks->verticalHeader()->setVisible(false);
    m_networks->verticalHeader()->setDefaultSectionSize(48);

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
    // Keep the header and at least three 48 px rows readable in a stacked layout.
    m_volumes->setMinimumHeight(200);
    m_volumes->setColumnCount(3);
    m_volumes->setHorizontalHeaderLabels({tr("Name"), tr("Driver"), tr("Mountpoint")});
    m_volumes->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    m_volumes->horizontalHeader()->setStretchLastSection(true);
    m_volumes->horizontalHeader()->setSectionResizeMode(QHeaderView::ResizeToContents);
    m_volumes->setSelectionBehavior(QAbstractItemView::SelectRows);
    m_volumes->setSelectionMode(QAbstractItemView::SingleSelection);
    m_volumes->setEditTriggers(QAbstractItemView::NoEditTriggers);
    m_volumes->setAlternatingRowColors(true);
    m_volumes->setShowGrid(false);
    m_volumes->verticalHeader()->setVisible(false);
    m_volumes->verticalHeader()->setDefaultSectionSize(48);

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

DockerObservabilityWidget::CommandResult DockerObservabilityWidget::runContainerRequest(
    const QString &binary, const QJsonObject &request, const std::shared_ptr<std::atomic_bool> &cancelled)
{
    CommandResult result;
    if (binary.isEmpty() || (cancelled && cancelled->load())) {
        result.stderrText = tr("Container query cancelled or the trusted Dockpipe CLI is unavailable.");
        return result;
    }
    QProcess process;
    process.start(binary, {"containers", "--request", "-"});
    if (!process.waitForStarted(3000)) {
        result.stderrText = process.errorString();
        return result;
    }
    const auto stop = [&process]() {
        process.terminate();
        if (!process.waitForFinished(1500)) {
            process.kill();
            process.waitForFinished(1000);
        }
    };
    const auto input = QJsonDocument(request).toJson(QJsonDocument::Compact);
    if (process.write(input) != input.size()) {
        stop();
        result.stderrText = tr("Could not send the container request.");
        return result;
    }
    process.closeWriteChannel();
    QElapsedTimer elapsed;
    elapsed.start();
    QByteArray output;
    QByteArray errors;
    // Escaped 512-KiB diagnostics and the existing inventory fit this wire budget.
    constexpr int maximumBytes = 4 * 1024 * 1024;
    while (true) {
        process.waitForFinished(50);
        bool oversized = false;
        for (const auto channel : {QProcess::StandardOutput, QProcess::StandardError}) {
            process.setReadChannel(channel);
            QByteArray &retained = channel == QProcess::StandardOutput ? output : errors;
            while (process.bytesAvailable() > 0) {
                const QByteArray chunk = process.read(8192);
                if (chunk.size() > maximumBytes - output.size() - errors.size()) {
                    oversized = true;
                    break;
                }
                retained += chunk;
            }
            if (oversized)
                break;
        }
        if (oversized || elapsed.elapsed() > 21000
            || (cancelled && cancelled->load())) {
            stop();
            result.stderrText = tr("Container query cancelled or exceeded its deadline or response limit.");
            return result;
        }
        if (process.state() == QProcess::NotRunning)
            break;
    }
    result.ok = process.exitStatus() == QProcess::NormalExit && process.exitCode() == 0;
    result.stdoutText = QString::fromUtf8(output);
    result.stderrText = QString::fromUtf8(errors);
    if (!result.ok && result.stderrText.trimmed().isEmpty())
        result.stderrText = process.errorString();
    if (result.ok && request.value("action").toString() != "snapshot") {
        const auto receipt = QJsonDocument::fromJson(output).object();
        result.ok = receipt.value("schema").toString() == "dockpipe.containers/v1"
            && receipt.value("action") == request.value("action")
            && receipt.value("kind") == request.value("kind")
            && receipt.value("object") == request.value("object");
        if (result.ok && request.value("action").toString() == "inspect") {
            const auto rows = receipt.value("inspect").toArray();
            result.ok = receipt.value("inspect").isArray() && rows.size() == 1 && rows.first().isObject();
        }
        if (result.ok && request.value("action").toString() == "logs")
            result.ok = !receipt.contains("logs") || receipt.value("logs").isString();
        if (!result.ok)
            result.stderrText = tr("Invalid container detail response.");
    }
    return result;
}

DockerObservabilityWidget::DockerSnapshot DockerObservabilityWidget::collectDockerSnapshot(
    const QString &binary, const std::shared_ptr<std::atomic_bool> &cancelled)
{
    DockerSnapshot snapshot;
    const auto result = runContainerRequest(binary, {{"action", "snapshot"}}, cancelled);
    const auto document = QJsonDocument::fromJson(result.stdoutText.toUtf8());
    const auto inventory = document.object();
    snapshot.containersOk = result.ok && document.isObject() && inventory.value("available").toBool();
    snapshot.networksOk = snapshot.containersOk;
    snapshot.volumesOk = snapshot.containersOk;
    QStringList notices;
    for (const auto &notice : inventory.value("notices").toArray())
        notices.append(notice.toString());
    snapshot.topologyNotice = notices.join(" · ");
    for (const auto &kind : {"containers", "networks", "volumes", "topology"}) {
        if (inventory.value(kind).toArray().size() > 256) {
            snapshot.containersOk = snapshot.networksOk = snapshot.volumesOk = false;
            snapshot.topologyNotice = tr("Container inventory exceeds its object limit.");
        }
    }
    if (!snapshot.containersOk) {
        snapshot.containersError = result.stderrText.isEmpty() ? snapshot.topologyNotice : result.stderrText;
        snapshot.networksError = snapshot.volumesError = snapshot.containersError;
        return snapshot;
    }
    for (const auto &value : inventory.value("containers").toArray()) {
        const auto row = value.toObject();
        const auto status = row.value("Status").toString();
        snapshot.containers.append({row.value("ID").toString(), row.value("Names").toString(),
            normalizeContainerState(row.value("State").toString(), status), status,
            row.value("Image").toString(), row.value("Ports").toString(), row.value("RunningFor").toString()});
    }
    for (const auto &value : inventory.value("networks").toArray()) {
        const auto row = value.toObject();
        snapshot.networks.append({row.value("ID").toString(), row.value("Name").toString(),
            row.value("Driver").toString(), row.value("Scope").toString()});
    }
    for (const auto &value : inventory.value("volumes").toArray()) {
        const auto row = value.toObject();
        snapshot.volumes.append({row.value("Name").toString(), row.value("Driver").toString(), tr("Select for details")});
    }
    snapshot.topology = inventory.value("topology").toArray();
    if (inventory.value("truncated").toBool())
        snapshot.topologyNotice += tr(" Inventory limited to 256 objects of each kind.");
    return snapshot;
}

void DockerObservabilityWidget::setStatus(const QString &text)
{
    if (m_inspector)
        m_inspector->setStatus(text);
}

void DockerObservabilityWidget::refresh()
{
    if (isRemoteTarget()) {
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
    if (m_refreshWatcher) {
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty()) {
            setLoadingState(false);
            setStatus(binaryError);
            return;
        }
        const auto cancelled = m_localReadCancelled;
        m_refreshWatcher->setFuture(QtConcurrent::run([binary, cancelled]() {
            return DockerObservabilityWidget::collectDockerSnapshot(binary, cancelled);
        }));
    }
}

void DockerObservabilityWidget::refreshQuietly()
{
    if (!m_active || m_monitorPaused)
        return;
    if (isRemoteTarget()) {
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
    if (m_refreshWatcher) {
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty()) {
            setLoadingState(false);
            setStatus(binaryError);
            return;
        }
        const auto cancelled = m_localReadCancelled;
        m_refreshWatcher->setFuture(QtConcurrent::run([binary, cancelled]() {
            return DockerObservabilityWidget::collectDockerSnapshot(binary, cancelled);
        }));
    }
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

    ++m_generation;
    cancelFeatureRead();
    m_remoteCommand.cancel();
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
    setStatus(tr("Refresh to load container inventory."));
}

void DockerObservabilityWidget::hideEvent(QHideEvent *event)
{
    setActive(false);
    QWidget::hideEvent(event);
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
    const auto count = [](bool available, int value) {
        return available ? QString::number(value) : QStringLiteral("—");
    };
    int running = 0;
    for (const auto &container : m_snapshot.containers) {
        if (container.state == "running" || container.state == "healthy" || container.state == "unhealthy")
            ++running;
    }
    m_containerCount->setText(count(m_snapshot.containersOk, m_containers->rowCount()));
    m_runningCount->setText(count(m_snapshot.containersOk, running));
    m_networkCount->setText(count(m_snapshot.networksOk, m_networks->rowCount()));
    m_volumeCount->setText(count(m_snapshot.volumesOk, m_volumes->rowCount()));
    m_tabs->setTabText(0, tr("Containers · %1").arg(m_containers->rowCount()));
    m_tabs->setTabText(1, tr("Networks · %1").arg(m_networks->rowCount()));
    m_tabs->setTabText(2, tr("Volumes · %1").arg(m_volumes->rowCount()));
    updateListResults();
}

void DockerObservabilityWidget::applyContainerFilter()
{
    if (!m_containers)
        return;
    const QString needle = m_search ? m_search->text().trimmed().toCaseFolded() : QString();
    const QString filter = m_stateFilter->currentData().toString();
    for (int row = 0; row < m_containers->rowCount(); ++row) {
        QStringList values;
        for (int col = 0; col < m_containers->columnCount(); ++col) {
            if (auto *item = m_containers->item(row, col)) {
                values << item->text();
                if (col == 1)
                    values << item->data(Qt::ToolTipRole).toString();
            }
        }
        const QString state = m_containers->item(row, 0)->data(Qt::UserRole + 1).toString();
        const bool running = state == "running" || state == "healthy" || state == "unhealthy";
        const bool stopped = state == "exited" || state == "created" || state == "dead";
        const bool attention = state == "unhealthy" || state == "dead" || state == "restarting";
        const bool matchesState = filter == "all" || (filter == "running" && running)
            || (filter == "stopped" && stopped) || (filter == "attention" && attention);
        const bool matchesSearch = needle.isEmpty() || values.join(QLatin1Char(' ')).toCaseFolded().contains(needle);
        m_containers->setRowHidden(row, m_dashboardView && (!matchesSearch || !matchesState));
    }
    if (m_dashboardView && m_tabs->currentIndex() == 0) {
        const int row = m_containers->currentRow();
        m_selectedResource.clear();
        if (row >= 0 && !m_containers->isRowHidden(row))
            m_selectedResource = "container:" + selectedContainerId();
        updateInspector();
    }
    updateListResults();
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
            QString stateLabel = state;
            if (state == "exited")
                stateLabel = tr("Stopped");
            else if (!stateLabel.isEmpty())
                stateLabel[0] = stateLabel[0].toUpper();
            stateItem->setData(Qt::AccessibleTextRole, stateLabel);
            stateItem->setSizeHint(QSize(120, 48));
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
        setStatus(tr("Inventory updated."));
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
    if (isRemoteTarget()) {
        if (!m_featureTarget.isEmpty())
            refreshFeatureDetails("container", containerId);
        else
            m_containerLogs->setPlainText(tr("Detailed inspection requires the paired worker's Docker read feature."));
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
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty()) {
            setStatus(binaryError);
            return;
        }
        const auto cancelled = m_localReadCancelled;
        m_containerDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_containerDetailWatcher->setFuture(QtConcurrent::run([containerId, binary, cancelled]() {
            ContainerDetailSnapshot snapshot;
            snapshot.containerId = containerId;
            const CommandResult inspect = DockerObservabilityWidget::runContainerRequest(binary, {{"action", "inspect"}, {"kind", "container"}, {"object", containerId}}, cancelled);
            if (!inspect.ok) {
                snapshot.inspectText = QCoreApplication::translate("DockerObservabilityWidget", "Could not inspect container.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.inspectText = inspectedObjectText(inspect.stdoutText);
            }
            const CommandResult logs =
                DockerObservabilityWidget::runContainerRequest(binary, {{"action", "logs"}, {"kind", "container"}, {"object", containerId}}, cancelled);
            if (!logs.ok) {
                snapshot.logsText = QCoreApplication::translate("DockerObservabilityWidget", "Could not load docker logs.\n\n%1").arg(logs.stderrText.trimmed());
            } else {
                snapshot.logsText = QJsonDocument::fromJson(logs.stdoutText.toUtf8()).object().value("logs").toString().trimmed();
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshNetworkDetailsAsync(const QString &networkId)
{
    if (isRemoteTarget()) {
        if (!m_featureTarget.isEmpty())
            refreshFeatureDetails("network", networkId);
        else
            m_containerLogs->setPlainText(tr("Detailed inspection requires the paired worker's Docker read feature."));
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
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty()) {
            setStatus(binaryError);
            return;
        }
        const auto cancelled = m_localReadCancelled;
        m_networkDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_networkDetailWatcher->setFuture(QtConcurrent::run([networkId, binary, cancelled]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = networkId;
            const CommandResult inspect =
                DockerObservabilityWidget::runContainerRequest(binary, {{"action", "inspect"}, {"kind", "network"}, {"object", networkId}}, cancelled);
            if (!inspect.ok) {
                snapshot.detailText = QCoreApplication::translate("DockerObservabilityWidget", "Could not inspect network.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = inspectedObjectText(inspect.stdoutText);
            }
            return snapshot;
        }));
    }
}

void DockerObservabilityWidget::refreshVolumeDetailsAsync(const QString &volumeName)
{
    if (isRemoteTarget()) {
        if (!m_featureTarget.isEmpty())
            refreshFeatureDetails("volume", volumeName);
        else
            m_containerLogs->setPlainText(tr("Detailed inspection requires the paired worker's Docker read feature."));
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
        QString binaryError;
        const auto binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
        if (binary.isEmpty()) {
            setStatus(binaryError);
            return;
        }
        const auto cancelled = m_localReadCancelled;
        m_volumeDetailWatcher->setProperty("generation", QVariant::fromValue(m_generation));
        m_volumeDetailWatcher->setFuture(QtConcurrent::run([volumeName, binary, cancelled]() {
            ObjectDetailSnapshot snapshot;
            snapshot.objectId = volumeName;
            const CommandResult inspect =
                DockerObservabilityWidget::runContainerRequest(binary, {{"action", "inspect"}, {"kind", "volume"}, {"object", volumeName}}, cancelled);
            if (!inspect.ok) {
                snapshot.detailText = QCoreApplication::translate("DockerObservabilityWidget", "Could not inspect volume.\n\n%1").arg(inspect.stderrText.trimmed());
            } else {
                snapshot.detailText = inspectedObjectText(inspect.stdoutText);
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
    if (m_containerActionBusy || !canControlTarget()) {
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
        if (isRemoteTarget())
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
    if (!canControlTarget())
        return;
    const QString id = selectedContainerId();
    const QString name = selectedContainerName();
    if (id.isEmpty() || m_containerActionBusy || !m_selectedResource.startsWith("container:"))
        return;
    static const QRegularExpression fullContainerId(QStringLiteral("^[0-9a-f]{64}$"));
    if (!fullContainerId.match(id).hasMatch()
        || (action != QStringLiteral("start") && action != QStringLiteral("stop") && action != QStringLiteral("restart"))) {
        setStatus(tr("Container control requires a supported action and an exact full container ID."));
        return;
    }
    if (isRemoteTarget()) {
        runRemoteContainerAction(action, id, name);
        return;
    }
    QString binaryError;
    const QString binary = DockpipeChoices::operatorDockpipeBinary(&binaryError);
    if (binary.isEmpty()) {
        setStatus(binaryError);
        return;
    }
    m_containerActionBusy = true;
    m_machine->setEnabled(false);
    updateInspector();
    setStatus(tr("%1: %2…").arg(name, action));
    auto *watcher = new QFutureWatcher<CommandResult>(this);
    const quint64 generation = m_generation;
    connect(watcher, &QFutureWatcher<CommandResult>::finished, this, [this, watcher, name, action, generation]() {
        const auto result = watcher->result();
        watcher->deleteLater();
        m_containerActionBusy = false;
        m_machine->setEnabled(true);
        updateInspector();
        if (generation != m_generation)
            return;
        if (!result.ok) {
            setStatus(tr("%1 failed for %2: %3").arg(action, name, result.stderrText.trimmed()));
            return;
        }
        refresh();
    });
    watcher->setFuture(QtConcurrent::run([binary, action, id]() {
        CommandResult result;
        QProcess process;
        process.setProcessChannelMode(QProcess::MergedChannels);
        process.start(binary, {QStringLiteral("containers"), QStringLiteral("--request"), QStringLiteral("-")});
        if (!process.waitForStarted(3000)) {
            result.stderrText = process.errorString();
            return result;
        }
        const QByteArray request = QJsonDocument(QJsonObject{{"action", action}, {"container", id}}).toJson(QJsonDocument::Compact);
        if (process.write(request) != request.size()) {
            process.kill();
            process.waitForFinished(1000);
            result.stderrText = QCoreApplication::translate("DockerObservabilityWidget", "Could not send the container control request.");
            return result;
        }
        process.closeWriteChannel();
        QElapsedTimer elapsed;
        elapsed.start();
        QByteArray output;
        constexpr int maximumResponseBytes = 64 * 1024;
        while (true) {
            process.waitForFinished(50);
            output += process.read(maximumResponseBytes + 1 - output.size());
            if (output.size() > maximumResponseBytes || elapsed.elapsed() > 21000) {
                process.kill();
                process.waitForFinished(1000);
                result.stderrText = QCoreApplication::translate("DockerObservabilityWidget", "Container control exceeded its deadline or response limit.");
                return result;
            }
            if (process.state() == QProcess::NotRunning)
                break;
        }
        if (process.exitStatus() != QProcess::NormalExit || process.exitCode() != 0) {
            result.stderrText = QString::fromUtf8(output).trimmed();
            if (result.stderrText.isEmpty())
                result.stderrText = process.errorString();
            return result;
        }
        const auto response = QJsonDocument::fromJson(output);
        const auto object = response.object();
        result.ok = response.isObject() && object.value("schema").toString() == QStringLiteral("dockpipe.containers/v1")
            && object.value("action").toString() == action && object.value("container").toString() == id;
        if (!result.ok)
            result.stderrText = QCoreApplication::translate("DockerObservabilityWidget", "Invalid container control response.");
        return result;
    }));
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
