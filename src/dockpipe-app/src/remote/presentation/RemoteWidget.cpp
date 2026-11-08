#include "RemoteWidget.h"
#include "DependencyChangeDialog.h"
#include "HardwareKeyDialog.h"
#include "ResourceCanvas.h"
#include "CanvasIcons.h"
#include "DockpipeChoices.h"
#include "SetupTerminal.h"

#include <QApplication>
#include <QClipboard>
#include <QButtonGroup>
#include <QCheckBox>
#include <QFrame>
#include <QSettings>
#include <QComboBox>
#include <QSignalBlocker>
#include <QDateTime>
#include <QDialog>
#include <QDialogButtonBox>
#include <QFormLayout>
#include <QFileDialog>
#include <QMenu>
#include <QHeaderView>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QLineEdit>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScrollArea>
#include <QStackedWidget>
#include <QSysInfo>
#include <QTableWidget>
#include <QTimer>
#include <QUrl>
#include <QVBoxLayout>
#include <QShowEvent>

namespace {
enum Page { Overview, Pairing, Connection, Setup };

QLabel *textLabel(const QString &text, const char *style = "appSubtitle")
{
    auto *label = new QLabel(text);
    label->setTextFormat(Qt::PlainText);
    label->setWordWrap(true);
    label->setObjectName(QString::fromUtf8(style));
    label->setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Maximum);
    return label;
}

QVBoxLayout *pageLayout(QWidget *page)
{
    auto *layout = new QVBoxLayout(page);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(12);
    layout->setAlignment(Qt::AlignTop);
    return layout;
}

QTableWidget *table(const QStringList &headers)
{
    auto *view = new QTableWidget(0, headers.size());
    view->setHorizontalHeaderLabels(headers);
    view->horizontalHeader()->setSectionResizeMode(QHeaderView::Stretch);
    view->setSelectionBehavior(QAbstractItemView::SelectRows);
    view->setSelectionMode(QAbstractItemView::SingleSelection);
    view->setEditTriggers(QAbstractItemView::NoEditTriggers);
    view->setShowGrid(false);
    view->verticalHeader()->hide();
    view->verticalHeader()->setDefaultSectionSize(48);
    view->setFixedHeight(220);
    return view;
}

QPushButton *actionButton(const QString &text, QHBoxLayout *row, const char *style = "secondaryButton")
{
    auto *button = new QPushButton(text);
    button->setObjectName(QString::fromUtf8(style));
    row->addWidget(button);
    return button;
}
}

RemoteWidget::RemoteWidget(QWidget *parent)
    : QWidget(parent), m_command(this), m_securityStatusCommand(this)
{
    setObjectName(QStringLiteral("machinesWorkspace"));
    setAttribute(Qt::WA_StyledBackground, true);
    m_management = new QWidget;
    m_management->setObjectName(QStringLiteral("machinesManagement"));
    auto *root = new QVBoxLayout(m_management);
    root->setContentsMargins(20, 16, 20, 16);
    root->setSpacing(14);

    m_pages = new QStackedWidget;
    m_pages->setObjectName(QStringLiteral("machinePages"));
    // Each screen scrolls independently so hidden setup forms cannot enlarge the overview.
    const auto addPage = [this](QWidget *page) {
        auto *scroll = new QScrollArea;
        scroll->setObjectName(QStringLiteral("machineScroll"));
        scroll->setFrameShape(QFrame::NoFrame);
        scroll->setWidgetResizable(true);
        scroll->viewport()->setObjectName(QStringLiteral("machineScrollViewport"));
        if (page->objectName().isEmpty())
            page->setObjectName(QStringLiteral("machineFormPage"));
        auto *content = new QWidget;
        content->setObjectName(QStringLiteral("machineContent"));
        content->setAttribute(Qt::WA_StyledBackground);
        auto *layout = pageLayout(content);
        page->setMinimumWidth(500);
        if (page->objectName() != "machinesOverview")
            page->setMaximumWidth(qMin(page->maximumWidth(), 1440));
        page->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Maximum);
        layout->addWidget(page);
        layout->addStretch();
        scroll->setWidget(content);
        m_pages->addWidget(scroll);
    };
    addPage(buildOverview());
    addPage(buildPairing());
    addPage(buildConnection());
    addPage(buildSetup());
    root->addWidget(m_pages, 1);
    m_status = textLabel(QString(), "hintText");
    m_status->setObjectName(QStringLiteral("machineStatus"));
    m_status->hide();
    root->addWidget(m_status);

    auto *footer = new QHBoxLayout;
    auto *details = actionButton(tr("Show details"), footer, "quietButton");
    m_detailsToggle = details;
    details->setCheckable(true);
    details->hide();
    connect(m_connectionSettings, &QPushButton::toggled, this, [details](bool expanded) {
        details->setVisible(expanded || details->isChecked());
    });
    footer->addStretch();
    m_cancel = actionButton(tr("Cancel current operation"), footer, "quietButton");
    m_cancel->hide();
    root->addLayout(footer);
    m_output = new QPlainTextEdit;
    m_output->setObjectName(QStringLiteral("machineDetails"));
    m_output->setReadOnly(true);
    m_output->setMaximumBlockCount(1000);
    m_output->setFixedHeight(140);
    m_output->hide();
    root->addWidget(m_output);
    connect(details, &QPushButton::toggled, this, [this, details](bool expanded) {
        details->setVisible(expanded || m_connectionSettings->isChecked());
        m_output->setVisible(expanded);
        details->setText(expanded ? tr("Hide details") : tr("Show details"));
    });
    connect(m_cancel, &QPushButton::clicked, &m_command, &RemoteCommand::cancel);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        m_output->appendPlainText(text.trimmed());
    });
    connect(&m_command, &RemoteCommand::standardOutput, this, [this](const QByteArray &data) {
        m_pendingOutput += QString::fromUtf8(data);
        while (m_pendingOutput.contains(QLatin1Char('\n'))) {
            const int end = m_pendingOutput.indexOf(QLatin1Char('\n'));
            const auto event = QJsonDocument::fromJson(m_pendingOutput.left(end).toUtf8()).object();
            m_pendingOutput.remove(0, end + 1);
            if (event.value("status").toString() == "pending" && !event.value("code").toString().isEmpty()) {
                m_pairingCode->setText(event.value("code").toString());
                m_pairingCode->show();
            }
        }
    });
    connect(&m_command, &RemoteCommand::hardwareKeyPrompt, this, [this](const QJsonObject &prompt) {
        if (!m_command.busy())
            return;
        if (!m_hardwareKeyDialog) {
            auto *dialog = new HardwareKeyDialog(this);
            m_hardwareKeyDialog = dialog;
            dialog->setAttribute(Qt::WA_DeleteOnClose);
            connect(dialog, &QDialog::rejected, &m_command, &RemoteCommand::cancel);
            connect(dialog, &HardwareKeyDialog::responseReady,
                    &m_command, &RemoteCommand::submitHardwareKeyResponse);
        }
        m_hardwareKeyDialog->showPrompt(prompt);
        m_hardwareKeyDialog->open();
    });
    connect(&m_command, &RemoteCommand::hardwareKeyStatus, this, [this](const QString &message) {
        if (m_hardwareKeyDialog)
            m_hardwareKeyDialog->showStatus(message);
    });
    connect(&m_command, &RemoteCommand::busyChanged, this, [this](bool busy) {
        if (!busy && m_hardwareKeyDialog) {
            m_hardwareKeyDialog->accept();
            m_hardwareKeyDialog.clear();
        }
    });
    connect(&m_command, &RemoteCommand::busyChanged, this, &RemoteWidget::updateActions);
    m_pairingTimer = new QTimer(this);
    m_pairingTimer->setInterval(5000);
    connect(m_pairingTimer, &QTimer::timeout, this, [this]() {
        if (isVisible() && m_pages->currentIndex() == Pairing && !m_command.busy())
            refreshPairings();
    });
    m_securityStatusTimer = new QTimer(this);
    m_securityStatusTimer->setInterval(10000);
    connect(m_securityStatusTimer, &QTimer::timeout, this, [this]() {
        if (isVisible() && m_pages->currentIndex() == Overview)
            refreshSecurityStatus();
    });
    updateActions();
    m_canvasManagementHost = new QWidget;
    auto *inspectorLayout = new QVBoxLayout(m_canvasManagementHost);
    inspectorLayout->setContentsMargins(0, 0, 0, 0);
    inspectorLayout->addWidget(m_machineSettingsPanel);
    m_canvas = new ResourceCanvas("machines", tr("Machines"),
        tr("Pairing and access relationships · Connection health is not checked here"),
        m_canvasManagementHost, this, false);
    m_canvas->setDetailsLabel(tr("Machine settings"));
    m_canvas->setCompactDetails(true);
    m_machineSettingsDialog = new QDialog(this);
    m_machineSettingsDialog->setObjectName("machineSettingsDialog");
    m_machineSettingsDialog->setWindowModality(Qt::WindowModal);
    m_machineSettingsDialog->resize(520, 300);
    auto *settingsLayout = new QVBoxLayout(m_machineSettingsDialog);
    auto *closeSettings = new QDialogButtonBox(QDialogButtonBox::Close);
    connect(closeSettings, &QDialogButtonBox::rejected, m_machineSettingsDialog, &QDialog::reject);
    settingsLayout->addWidget(closeSettings);
    connect(m_machineSettingsDialog, &QDialog::finished, this, [this]() {
        m_canvasManagementHost->layout()->addWidget(m_machineSettingsPanel);
        m_machineSettingsPanel->show();
    });
    m_managementDialog = new QDialog(this);
    m_managementDialog->setObjectName("machineSetupDialog");
    m_managementDialog->setWindowTitle(tr("Machine setup"));
    m_managementDialog->setWindowModality(Qt::WindowModal);
    m_managementDialog->resize(760, 680);
    auto *setupLayout = new QVBoxLayout(m_managementDialog);
    setupLayout->setContentsMargins(0, 0, 0, 0);

    auto *workspaceLayout = new QVBoxLayout(this);
    workspaceLayout->setContentsMargins(0, 0, 0, 0);
    workspaceLayout->setSpacing(0);
    auto *toolbar = new QHBoxLayout;
    toolbar->setContentsMargins(20, 20, 20, 8);
    auto *heading = new QVBoxLayout;
    heading->setSpacing(5);
    heading->addWidget(textLabel(tr("Machines"), "machinesTitle"));
    heading->addWidget(textLabel(tr("Manage trusted computers and workflow access."), "machinesSubtitle"));
    toolbar->addLayout(heading, 1);
    m_canvasAdd = new QPushButton(tr("Add machine"));
    m_canvasAdd->setObjectName("primaryButton");
    connect(m_canvasAdd, &QPushButton::clicked, this, [this]() { showPage(Pairing); });
    toolbar->addWidget(m_canvasAdd, 0, Qt::AlignVCenter);
    auto *switcher = new QFrame;
    switcher->setObjectName(QStringLiteral("machinesViewSwitcher"));
    switcher->setSizePolicy(QSizePolicy::Maximum, QSizePolicy::Fixed);
    auto *switcherLayout = new QHBoxLayout(switcher);
    switcherLayout->setContentsMargins(3, 3, 3, 3);
    switcherLayout->setSpacing(2);
    auto *viewButtons = new QButtonGroup(this);
    m_canvasView = new QPushButton(tr("Canvas"));
    m_canvasView->setObjectName(QStringLiteral("machinesCanvasView"));
    m_canvasView->setAccessibleName(tr("Machines canvas view"));
    m_canvasView->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Branch));
    m_listView = new QPushButton(tr("List"));
    m_listView->setObjectName(QStringLiteral("machinesListView"));
    m_listView->setAccessibleName(tr("Machines list view"));
    m_listView->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::List));
    for (auto *button : {m_canvasView, m_listView}) {
        button->setCheckable(true);
        viewButtons->addButton(button);
        switcherLayout->addWidget(button);
    }
    toolbar->addWidget(switcher, 0, Qt::AlignVCenter);
    workspaceLayout->addLayout(toolbar);
    m_views = new QStackedWidget;
    m_views->setObjectName(QStringLiteral("machinesViews"));
    m_views->addWidget(m_canvas);
    m_listManagementHost = new QWidget;
    auto *listLayout = new QVBoxLayout(m_listManagementHost);
    listLayout->setContentsMargins(0, 0, 0, 0);
    m_views->addWidget(m_listManagementHost);
    workspaceLayout->addWidget(m_views, 1);
    connect(m_canvasView, &QPushButton::clicked, this, [this]() { setListView(false); });
    connect(m_listView, &QPushButton::clicked, this, [this]() { setListView(true); });
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, [this](const QString &id) {
        m_nodes->clearSelection();
        for (int row = 0; row < m_nodes->rowCount(); ++row) {
            if (m_nodes->item(row, 0)->text() == id) {
                m_machineSearch->clear();
                m_accessFilter->setCurrentIndex(0);
                m_nodes->selectRow(row);
                break;
            }
        }
        m_settingsNode = id;
        m_machineSettingsError->hide();
        updateMachineSettings();
    });
    setListView(QSettings().value("machines/viewMode", "canvas").toString() == "list");
    updateMachineGraph();

}

QWidget *RemoteWidget::buildOverview()
{
    auto *page = new QWidget;
    auto *layout = pageLayout(page);
    page->setObjectName(QStringLiteral("machinesOverview"));
    auto *actions = new QHBoxLayout;
    auto *add = actionButton(tr("Add machine"), actions, "primaryButton");
    connect(add, &QPushButton::clicked, this, [this]() { showPage(Pairing); });
    actions->addStretch();
    auto *refreshButton = actionButton(tr("Refresh"), actions, "quietButton");
    m_actions.append(refreshButton);
    connect(refreshButton, &QPushButton::clicked, this, [this]() { refresh(true); });
    m_connectionSettings = actionButton(tr("Connection settings"), actions);
    m_connectionSettings->setObjectName("machinesConnectionSettings");
    m_connectionSettings->setCheckable(true);
    layout->addLayout(actions);

    auto *connectionCard = new QFrame;
    connectionCard->setObjectName(QStringLiteral("machinesConnectionCard"));
    auto *connectionLayout = new QVBoxLayout(connectionCard);
    connectionLayout->setContentsMargins(16, 14, 16, 14);
    connectionLayout->setSpacing(8);
    auto *connectionHeading = new QHBoxLayout;
    connectionHeading->addWidget(textLabel(tr("Remote connection"), "sectionTitle"), 1);
    connectionLayout->addLayout(connectionHeading);
    m_connectionSummary = textLabel(tr("Loading connection details…"));
    m_connectionSummary->setObjectName(QStringLiteral("remoteConnectionSummary"));
    m_connectionSummary->setTextInteractionFlags(Qt::TextSelectableByMouse);
    connectionLayout->addWidget(m_connectionSummary);
    m_brokerServiceHint = textLabel(QString(), "hintText");
    m_workerServiceHint = textLabel(QString(), "hintText");
    m_updateBrokerService = new QPushButton(tr("Update broker service…"));
    m_updateWorkerService = new QPushButton(tr("Update worker service…"));
    m_brokerServiceHint->setObjectName(QStringLiteral("brokerServiceHint"));
    m_workerServiceHint->setObjectName(QStringLiteral("workerServiceHint"));
    for (auto *hint : {m_brokerServiceHint, m_workerServiceHint}) {
        connectionLayout->addWidget(hint);
        hint->hide();
    }
    for (auto *button : {m_updateBrokerService, m_updateWorkerService}) {
        button->setObjectName(QStringLiteral("secondaryButton"));
        connectionLayout->addWidget(button, 0, Qt::AlignLeft);
        m_actions.append(button);
        button->hide();
    }
    connect(m_updateBrokerService, &QPushButton::clicked, this, [this]() { replaceService("broker"); });
    connect(m_updateWorkerService, &QPushButton::clicked, this, [this]() { replaceService("worker"); });
    auto *connectionActions = new QHBoxLayout;
    auto *join = actionButton(tr("Connect this computer"), connectionActions);
    connect(join, &QPushButton::clicked, this, [this]() { showPage(Connection); });
    connectionActions->addStretch();
    connectionLayout->addLayout(connectionActions);
    m_securityStatus = textLabel(tr("Checking remote access lock status…"), "hintText");
    m_securityStatus->setObjectName(QStringLiteral("remoteLockStatus"));
    m_securityStatus->setMaximumWidth(320);
    m_securityStatus->setSizePolicy(QSizePolicy::Maximum, QSizePolicy::Preferred);
    connectionHeading->addWidget(m_securityStatus);
    auto *inventoryCard = new QFrame;
    inventoryCard->setObjectName(QStringLiteral("machinesInventoryCard"));
    auto *inventoryLayout = new QVBoxLayout(inventoryCard);
    inventoryLayout->setContentsMargins(16, 16, 16, 16);
    inventoryLayout->setSpacing(10);
    auto *inventoryHeading = new QHBoxLayout;
    inventoryHeading->addWidget(textLabel(tr("Your machines"), "sectionTitle"));
    inventoryHeading->addStretch();
    inventoryLayout->addLayout(inventoryHeading);
    layout->addWidget(inventoryCard);
    m_overviewHint = textLabel(tr("Loading machines…"));

    m_review = new QPushButton;
    m_review->setObjectName(QStringLiteral("secondaryButton"));
    m_review->hide();
    connect(m_review, &QPushButton::clicked, this, [this]() {
        showPage(Pairing);
        setPairingStep(2);
    });
    inventoryHeading->addWidget(m_review);
    m_nodes = table({tr("Machine"), tr("Access"), tr("Details"), QString()});
    m_nodes->horizontalHeader()->setSectionResizeMode(3, QHeaderView::Fixed);
    m_nodes->setColumnWidth(3, 110);
    m_nodes->setToolTip(tr("Access shows pairing permission. Connection health is not checked here."));
    m_nodes->setObjectName(QStringLiteral("managedMachines"));
    m_nodes->setAlternatingRowColors(true);
    m_nodes->horizontalHeader()->setMinimumSectionSize(110);
    m_nodes->horizontalHeader()->setSectionResizeMode(0, QHeaderView::Interactive);
    m_nodes->horizontalHeader()->setSectionResizeMode(1, QHeaderView::Interactive);
    m_nodes->setColumnWidth(0, 190);
    m_nodes->setColumnWidth(1, 160);
    auto *filters = new QHBoxLayout;
    m_machineSearch = new QLineEdit;
    m_machineSearch->setObjectName(QStringLiteral("machinesSearch"));
    m_machineSearch->setPlaceholderText(tr("Search machines…"));
    m_machineSearch->setAccessibleName(tr("Search machines"));
    m_machineSearch->setClearButtonEnabled(true);
    m_machineSearch->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Search), QLineEdit::LeadingPosition);
    filters->addWidget(m_machineSearch, 1);
    m_accessFilter = new QComboBox;
    m_accessFilter->setObjectName(QStringLiteral("machinesAccessFilter"));
    m_accessFilter->setAccessibleName(tr("Filter machine access"));
    m_accessFilter->addItem(tr("All access states"), QString());
    m_accessFilter->addItem(tr("Paired"), QStringLiteral("paired"));
    m_accessFilter->addItem(tr("Invitations"), QStringLiteral("invited"));
    m_accessFilter->addItem(tr("Access removed"), QStringLiteral("revoked"));
    filters->addWidget(m_accessFilter);
    inventoryLayout->addLayout(filters);
    m_listResults = textLabel(tr("Refresh to load managed machines."), "hintText");
    m_listResults->setObjectName(QStringLiteral("machinesListResults"));
    inventoryLayout->addWidget(m_listResults);
    inventoryLayout->addWidget(m_nodes);
    m_machineSettingsPanel = new QWidget;
    m_machineSettingsPanel->setObjectName("machineSettingsPanel");
    auto *machineSettingsLayout = new QVBoxLayout(m_machineSettingsPanel);
    machineSettingsLayout->setContentsMargins(20, 20, 20, 20);
    machineSettingsLayout->setSpacing(16);
    m_selectedMachine = textLabel(tr("Select a machine to view its settings."), "machineSelectionCard");
    m_selectedMachine->setObjectName(QStringLiteral("machinesSelectedAccess"));
    m_selectedMachine->setTextInteractionFlags(Qt::TextSelectableByMouse);
    machineSettingsLayout->addWidget(m_selectedMachine);
    m_localSettings = new QPushButton(tr("Configure this computer…"));
    connect(m_localSettings, &QPushButton::clicked, this, [this]() { showPage(Setup); });
    machineSettingsLayout->addWidget(m_localSettings, 0, Qt::AlignLeft);
    m_localSettings->hide();
    m_machineSettingsError = textLabel(QString(), "hintText");
    m_machineSettingsError->setObjectName("machineSettingsError");
    machineSettingsLayout->addWidget(m_machineSettingsError);
    m_machineSettingsError->hide();
    connect(m_nodes, &QTableWidget::cellDoubleClicked, this, [this](int row, int) {
        if (m_nodes->item(row, 0))
            openMachineSettings(m_nodes->item(row, 0)->text());
    });
    connect(m_machineSearch, &QLineEdit::textChanged, this, &RemoteWidget::updateMachineList);
    connect(m_accessFilter, qOverload<int>(&QComboBox::currentIndexChanged), this, &RemoteWidget::updateMachineList);
    connect(m_nodes, &QTableWidget::itemSelectionChanged, this, &RemoteWidget::updateMachineList);
    auto *management = new QHBoxLayout;
    m_revoke = actionButton(tr("Remove access…"), management, "quietButton");
    m_actions.append(m_revoke);
    m_removeMachine = actionButton(tr("Remove machine…"), management, "quietButton");
    m_actions.append(m_removeMachine);
    connect(m_removeMachine, &QPushButton::clicked, this, [this]() {
        if (m_settingsNode.isEmpty() || m_settingsNode.startsWith('@'))
            return;
        const QString node = m_settingsNode;
        QString confirmation = tr("Remove %1 from your managed machines? Its saved enrollment and pairing records will be deleted and its credential will stop working. Job history is kept. To reconnect it later, pair it again.").arg(node);
        if (m_brokerServiceNeedsUpdate)
            confirmation += tr("\n\nThe broker service must be updated first. Continuing replaces its configuration with the current Dockpipe installation and restarts it. This replaces custom service settings and interrupts active remote work. Other machines' pairing and remote-access settings are preserved.");
        if (QMessageBox::question(this, tr("Remove machine"),
                confirmation,
                QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
            return;
        if (m_brokerServiceNeedsUpdate) {
            run({"service", "--role", "broker", "--replace"}, [this, node](bool ok, const QByteArray &) {
                if (ok) {
                    m_brokerServiceNeedsUpdate = false;
                    removeMachine(node);
                } else {
                    m_status->setText(tr("The broker service update failed. %1 has not been removed. See details below.").arg(node));
                    m_status->show();
                    m_detailsToggle->setChecked(true);
                    m_machineSettingsError->setText(m_status->text() + "\n" + m_output->toPlainText().right(2048));
                    m_machineSettingsError->show();
                }
            }, 35000);
        } else {
            removeMachine(node);
        }
    });
    management->addStretch();
    connect(m_nodes, &QTableWidget::itemSelectionChanged, this, &RemoteWidget::updateActions);
    connect(m_revoke, &QPushButton::clicked, this, [this]() {
        if (m_settingsNode.isEmpty() || m_settingsNode.startsWith('@'))
            return;
        const QString node = m_settingsNode;
        if (QMessageBox::question(this, tr("Remove machine access"),
                tr("Remove access for %1? Its credential will stop working. This does not undo work already executed.").arg(node),
                QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
            return;
        run({"revoke", "--id", node}, [this](bool ok, const QByteArray &) {
            if (ok) {
                refresh();
            } else {
                m_machineSettingsError->setText(tr("Could not remove access.\n%1").arg(m_output->toPlainText().right(2048)));
                m_machineSettingsError->show();
            }
        });
    });
    machineSettingsLayout->addLayout(management);
    machineSettingsLayout->addStretch();
    inventoryLayout->addWidget(m_overviewHint);
    auto *dependencies = new QPushButton(tr("Dependency update and recovery…"), this);
    dependencies->setObjectName(QStringLiteral("dependencyChangeNavigation"));
    connectionLayout->addWidget(dependencies, 0, Qt::AlignLeft);
    connect(dependencies, &QPushButton::clicked, this, [this]() {
        DependencyChangeDialog dialog(m_workdir, this);
        dialog.exec();
    });
    auto *setupActions = new QHBoxLayout;
    auto *setup = actionButton(tr("Set up remote access…"), setupActions, "quietButton");
    setup->setObjectName(QStringLiteral("remoteSetupNavigation"));
    setupActions->addStretch();
    connect(setup, &QPushButton::clicked, this, [this]() { showPage(Setup); });
    connectionLayout->addLayout(setupActions);
    layout->addWidget(connectionCard);
    connectionCard->hide();
    connect(m_connectionSettings, &QPushButton::toggled, connectionCard, &QWidget::setVisible);
    return page;
}

QWidget *RemoteWidget::buildPairing()
{
    auto *page = new QWidget;
    page->setObjectName("addMachinePage");
    page->setMaximumWidth(720);
    auto *layout = pageLayout(page);
    auto *back = new QPushButton(tr("← Machines"));
    m_pairingBack = back;
    back->setObjectName("quietButton");
    connect(back, &QPushButton::clicked, this, [this]() { showPage(Overview); });
    layout->addWidget(back, 0, Qt::AlignLeft);
    m_pairingTitle = textLabel(tr("Add machine"), "machinesTitle");
    layout->addWidget(m_pairingTitle);
    m_pairingProgress = textLabel(QString(), "machineMetricCaption");
    m_pairingProgress->setObjectName("machinePairingProgress");
    layout->addWidget(m_pairingProgress);
    auto *card = new QFrame;
    card->setObjectName("machinePairingBody");
    auto *cardLayout = new QVBoxLayout(card);
    cardLayout->setContentsMargins(0, 8, 0, 0);
    m_pairingSteps = new QStackedWidget;
    m_pairingSteps->setObjectName("machinePairingSteps");
    cardLayout->addWidget(m_pairingSteps);
    layout->addWidget(card);

    auto *prepare = new QWidget;
    prepare->setObjectName("machinePairingStep");
    auto *prepareLayout = pageLayout(prepare);
    prepareLayout->addWidget(textLabel(tr("Add another computer"), "sectionTitle"));
    prepareLayout->addWidget(textLabel(tr("Connect a computer so it can receive workflows from here. Have Dockpipe open on that computer before you begin.")));
    m_windowHint = textLabel(tr("Start pairing accepts requests for 15 minutes. You still approve each computer before it gets access."), "hintText");
    prepareLayout->addWidget(m_windowHint);
    auto *startActions = new QHBoxLayout;
    m_startPairing = actionButton(tr("Start pairing"), startActions, "primaryButton");
    m_startPairing->setObjectName("machinePairingStart");
    m_actions.append(m_startPairing);
    startActions->addStretch();
    prepareLayout->addLayout(startActions);
    auto *connectHere = new QPushButton(tr("Connect this computer instead"));
    connectHere->setObjectName("quietButton");
    connect(connectHere, &QPushButton::clicked, this, [this]() { showPage(Connection); });
    prepareLayout->addWidget(connectHere, 0, Qt::AlignLeft);
    prepareLayout->addStretch();
    m_pairingSteps->addWidget(prepare);
    connect(m_startPairing, &QPushButton::clicked, this, [this]() {
        if (m_pairingEndpoint.isEmpty()) {
            showPage(Setup);
            return;
        }
        run({"pairing-open"}, [this](bool ok, const QByteArray &) {
            if (!ok)
                return;
            m_requestOutcome.clear();
            m_windowHint->setText(tr("Requests are accepted for 15 minutes. Each computer still needs your approval."));
            m_brokerAvailable = true;
            setPairingStep(1);
            m_pairingTimer->start();
            refreshPairings();
        });
    });

    auto *instructions = new QWidget;
    instructions->setObjectName("machinePairingStep");
    auto *instructionsLayout = pageLayout(instructions);
    instructionsLayout->addWidget(textLabel(tr("On the other computer"), "sectionTitle"));
    instructionsLayout->addWidget(textLabel(tr("Open Dockpipe → Machines → Add machine → Connect this computer instead. Enter this address and request pairing.")));
    m_pairingAddress = textLabel(QString(), "machinesSelectedAccess");
    m_pairingAddress->setObjectName("machinePairingAddress");
    m_pairingAddress->setTextInteractionFlags(Qt::TextSelectableByMouse);
    instructionsLayout->addWidget(m_pairingAddress);
    auto *copy = new QPushButton(tr("Copy address"));
    copy->setObjectName("machinePairingCopyAddress");
    connect(copy, &QPushButton::clicked, this, [this]() {
        if (!m_pairingEndpoint.isEmpty())
            QApplication::clipboard()->setText(m_pairingEndpoint);
    });
    instructionsLayout->addWidget(copy, 0, Qt::AlignLeft);
    m_pairingWaiting = textLabel(tr("Leave this screen open. A request will appear when the other computer is ready."), "hintText");
    instructionsLayout->addWidget(m_pairingWaiting);
    auto *waitingActions = new QHBoxLayout;
    m_reviewPairing = actionButton(tr("Review request"), waitingActions, "primaryButton");
    m_reviewPairing->setObjectName("machinePairingReview");
    m_actions.append(m_reviewPairing);
    connect(m_reviewPairing, &QPushButton::clicked, this, [this]() { setPairingStep(2); });
    auto *checkAgain = actionButton(tr("Check again"), waitingActions, "quietButton");
    m_actions.append(checkAgain);
    connect(checkAgain, &QPushButton::clicked, this, &RemoteWidget::refreshPairings);
    auto *close = actionButton(tr("Stop pairing"), waitingActions, "quietButton");
    m_actions.append(close);
    waitingActions->addStretch();
    instructionsLayout->addLayout(waitingActions);
    connect(close, &QPushButton::clicked, this, [this]() {
        run({"pairing-close"}, [this](bool ok, const QByteArray &) {
            if (ok) {
                m_windowHint->setText(tr("Pairing stopped. Start again when you are ready to add a computer."));
                setPairingStep(0);
                refreshPairings();
            }
        });
    });
    instructionsLayout->addStretch();
    m_pairingSteps->addWidget(instructions);

    auto *review = new QWidget;
    review->setObjectName("machinePairingStep");
    auto *reviewLayout = pageLayout(review);
    reviewLayout->addWidget(textLabel(tr("Compare the verification code"), "sectionTitle"));
    m_pairingHint = textLabel(tr("Select the request and compare its code with the other computer. Approve only if they match."));
    reviewLayout->addWidget(m_pairingHint);
    m_pairings = table({tr("Machine"), tr("Verification code"), tr("Expires")});
    m_pairings->setFixedHeight(150);
    m_pairings->setObjectName("pairingRequests");
    reviewLayout->addWidget(m_pairings);
    auto *decisions = new QHBoxLayout;
    m_approve = actionButton(tr("Approve with security key"), decisions, "primaryButton");
    m_deny = actionButton(tr("Deny request"), decisions, "quietButton");
    decisions->addStretch();
    auto *refreshButton = actionButton(tr("Check for requests"), decisions, "quietButton");
    m_actions.append(m_approve);
    m_actions.append(m_deny);
    m_actions.append(refreshButton);
    connect(m_approve, &QPushButton::clicked, this, [this]() { decidePairing(true); });
    connect(m_deny, &QPushButton::clicked, this, [this]() { decidePairing(false); });
    connect(refreshButton, &QPushButton::clicked, this, &RemoteWidget::refreshPairings);
    connect(m_pairings, &QTableWidget::itemSelectionChanged, this, &RemoteWidget::updateActions);
    reviewLayout->addLayout(decisions);
    auto *instructionsButton = new QPushButton(tr("Back to connection instructions"));
    instructionsButton->setObjectName("quietButton");
    connect(instructionsButton, &QPushButton::clicked, this, [this]() { setPairingStep(1); });
    reviewLayout->addWidget(instructionsButton, 0, Qt::AlignLeft);
    reviewLayout->addStretch();
    m_pairingSteps->addWidget(review);

    auto *complete = new QWidget;
    complete->setObjectName("machinePairingStep");
    auto *completeLayout = pageLayout(complete);
    m_pairingSuccess = textLabel(QString(), "sectionTitle");
    completeLayout->addWidget(m_pairingSuccess);
    completeLayout->addWidget(textLabel(tr("Finish on the other computer. Dockpipe starts its worker after approval so it can receive workflows.")));
    auto *done = new QPushButton(tr("Done"));
    done->setObjectName("primaryButton");
    connect(done, &QPushButton::clicked, this, [this]() { showPage(Overview); });
    completeLayout->addWidget(done, 0, Qt::AlignLeft);
    completeLayout->addStretch();
    m_pairingSteps->addWidget(complete);
    setPairingStep(0);
    return page;
}

void RemoteWidget::setPairingStep(int step)
{
    m_pairingSteps->setCurrentIndex(step);
    const QStringList progress{tr("1 of 3 · Prepare"), tr("2 of 3 · Connect"), tr("3 of 3 · Approve"), tr("Pairing complete")};
    m_pairingProgress->setText(progress.value(step));
    QWidget *content = m_pairingSteps->currentWidget();
    content->ensurePolished();
    const int contentWidth = qMin(620, qMax(460, m_pairingSteps->width()));
    int contentHeight = content->layout()->totalHeightForWidth(contentWidth);
    if (contentHeight < 0)
        contentHeight = content->sizeHint().height();
    m_pairingSteps->setFixedHeight(qMax(160, contentHeight));
    if (m_managementDialog && m_canvasView && m_canvasView->isChecked()) {
        QWidget *page = m_pairingSteps->parentWidget()->parentWidget();
        m_managementDialog->resize(700, qBound(440, page->sizeHint().height() + 160, 740));
    }
}

QWidget *RemoteWidget::buildConnection()
{
    auto *page = new QWidget;
    auto *layout = pageLayout(page);
    auto *back = new QPushButton(tr("← Machines"));
    back->setObjectName(QStringLiteral("quietButton"));
    connect(back, &QPushButton::clicked, this, [this]() { showPage(Overview); });
    layout->addWidget(back, 0, Qt::AlignLeft);
    layout->addWidget(textLabel(tr("Connect this computer"), "sectionTitle"));
    layout->addWidget(textLabel(tr("Enter the address shown on the computer you are connecting to.")));
    auto *form = new QFormLayout;
    form->setSpacing(12);
    m_endpoint = new QLineEdit;
    m_endpoint->setPlaceholderText(tr("https://your-broker.example.com"));
    m_name = new QLineEdit(QSysInfo::machineHostName().section('.', 0, 0));
    form->addRow(tr("Remote address"), m_endpoint);
    form->addRow(tr("Name for this computer"), m_name);
    layout->addLayout(form);
    m_allow = new QCheckBox(tr("Allow workflows from the managing computer to run as my user"));
    layout->addWidget(m_allow);
    layout->addWidget(textLabel(tr("Connect only to a computer you trust. Its workflows run as your user and can access your files and tools."), "hintText"));
    layout->addWidget(textLabel(tr("After approval, Dockpipe starts a background worker service for your user so this computer can receive workflows."), "hintText"));
    auto *actions = new QHBoxLayout;
    m_request = actionButton(tr("Request pairing"), actions, "primaryButton");
    m_actions.append(m_request);
    actions->addStretch();
    layout->addLayout(actions);
    connect(m_endpoint, &QLineEdit::textChanged, this, &RemoteWidget::updateActions);
    connect(m_name, &QLineEdit::textChanged, this, &RemoteWidget::updateActions);
    connect(m_allow, &QCheckBox::toggled, this, &RemoteWidget::updateActions);
    connect(m_request, &QPushButton::clicked, this, [this]() {
        m_pairingCode->clear();
        m_pairingCode->hide();
        run({"pair", "--endpoint", m_endpoint->text().trimmed(), "--node", m_name->text().trimmed(), "--allow-delivery"},
            [this](bool ok, const QByteArray &) {
                m_pairingCode->hide();
                if (ok)
                    startWorkerService();
            }, 16 * 60 * 1000);
        m_status->setText(tr("Waiting for approval. Compare the code below on the managing computer."));
    });
    m_pairingCode = textLabel(QString(), "pairingCode");
    m_pairingCode->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_pairingCode->setStyleSheet(QStringLiteral("font-size: 26px; font-weight: 600; padding: 12px 0;"));
    m_pairingCode->hide();
    layout->addWidget(m_pairingCode);
    auto *recoveryToggle = new QPushButton(tr("Already paired? Recovery options"));
    recoveryToggle->setObjectName("machineConnectionRecovery");
    recoveryToggle->setCheckable(true);
    layout->addWidget(recoveryToggle, 0, Qt::AlignLeft);
    auto *recovery = new QWidget;
    auto *recoveryLayout = pageLayout(recovery);
    recoveryLayout->addWidget(textLabel(tr("Restart receiving workflows without pairing again. Keep this computer awake while work runs."), "hintText"));
    auto *serviceActions = new QHBoxLayout;
    auto *start = actionButton(tr("Start worker service"), serviceActions);
    m_actions.append(start);
    serviceActions->addStretch();
    connect(start, &QPushButton::clicked, this, &RemoteWidget::startWorkerService);
    recoveryLayout->addLayout(serviceActions);
    layout->addWidget(recovery);
    recovery->hide();
    connect(recoveryToggle, &QPushButton::toggled, recovery, &QWidget::setVisible);
    return page;
}

void RemoteWidget::updateServiceNotices(const QJsonObject &info)
{
    const QString brokerState = info.value("broker").toObject().value("service").toObject().value("state").toString();
    m_brokerServiceNeedsUpdate = brokerState == "update-required" || brokerState == "missing";
    m_connectionSettings->setText(m_brokerServiceNeedsUpdate
        ? tr("Connection settings · Update available") : tr("Connection settings"));
    const auto update = [](const QJsonObject &service, QLabel *hint, QPushButton *button) {
        const QString state = service.value("state").toString();
        const bool needsUpdate = state == "update-required";
        const bool missing = state == "missing";
        const bool unavailable = state == "unavailable";
        hint->setVisible(needsUpdate || missing || unavailable);
        button->setVisible(needsUpdate || missing);
        if (needsUpdate)
            hint->setText(tr("The background service needs an update. It may still use an older Dockpipe installation or an unfinished update. Update it before retrying remote work."));
        else if (missing)
            hint->setText(tr("The background service is not installed. Install it to receive remote work."));
        else if (unavailable)
            hint->setText(tr("The background service registration could not be inspected. Check its configuration before retrying remote work."));
    };
    update(info.value("broker").toObject().value("service").toObject(), m_brokerServiceHint, m_updateBrokerService);
    update(info.value("worker").toObject().value("service").toObject(), m_workerServiceHint, m_updateWorkerService);
}

void RemoteWidget::removeMachine(const QString &node)
{
    run({"remove", "--id", node}, [this, node](bool ok, const QByteArray &) {
        if (ok) {
            m_machineSettingsDialog->accept();
            m_settingsNode.clear();
            m_machineSettingsError->hide();
            refresh();
        } else {
            m_status->setText(tr("Could not remove %1. Its entry is unchanged. See details below.").arg(node));
            m_status->show();
            m_detailsToggle->setChecked(true);
            m_machineSettingsError->setText(m_status->text() + "\n" + m_output->toPlainText().right(2048));
            m_machineSettingsError->show();
        }
    });
}

void RemoteWidget::replaceService(const QString &role)
{
    const QString title = role == "broker" ? tr("Update broker service") : tr("Update worker service");
    if (QMessageBox::question(this, title,
            tr("Replace this background service's configuration with the current Dockpipe installation and restart it? "
               "This replaces custom service settings and interrupts active remote work. Pairing and remote-access settings are preserved."),
            QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
        return;
    run({"service", "--role", role, "--replace"}, [this](bool ok, const QByteArray &) {
        if (ok)
            refresh();
        else
            m_status->setText(tr("The service update did not finish. Pairing is preserved. See details, then retry Update service."));
    }, 35000);
}

void RemoteWidget::startWorkerService()
{
    run({"service", "--role", "worker"}, [this](bool ok, const QByteArray &) {
        m_status->setText(ok
            ? tr("Worker service started. Choose this computer under Run on from your managing computer.")
            : tr("Could not start the worker service. Your pairing is preserved. See details, then choose Start worker service to retry."));
    });
    m_status->setText(tr("Starting the background worker service…"));
}

QWidget *RemoteWidget::buildSetup()
{
    auto *page = new QWidget;
    auto *layout = pageLayout(page);
    auto *back = new QPushButton(tr("← Machines"));
    back->setObjectName(QStringLiteral("quietButton"));
    connect(back, &QPushButton::clicked, this, [this]() { showPage(Overview); });
    layout->addWidget(back, 0, Qt::AlignLeft);
    layout->addWidget(textLabel(tr("Set up remote access"), "sectionTitle"));
    layout->addWidget(textLabel(tr("Choose a provider to connect your machines. Install a provider package here if you don't have one yet.")));
    m_existingConnection = textLabel(QString());
    m_existingConnection->setObjectName(QStringLiteral("existingRemoteConnection"));
    m_existingConnection->hide();
    layout->addWidget(m_existingConnection);
    auto *form = new QFormLayout;
    form->setSpacing(12);
    m_resolver = new QComboBox;
    m_resolver->setObjectName(QStringLiteral("remoteResolver"));
    m_hostname = new QLineEdit;
    m_hostname->setObjectName(QStringLiteral("remoteHostname"));
    m_hostname->setPlaceholderText(tr("remote.example.com"));
    m_hostnameLabel = textLabel(tr("Public hostname (if required)"));
    form->addRow(tr("Provider"), m_resolver);
    form->addRow(m_hostnameLabel, m_hostname);
    layout->addLayout(form);
    m_installProvider = new QPushButton(tr("Install provider…"));
    m_installProvider->setObjectName(QStringLiteral("installRemoteProvider"));
    connect(m_installProvider, &QPushButton::clicked, this, &RemoteWidget::installProviderRequested);
    layout->addWidget(m_installProvider, 0, Qt::AlignLeft);
    m_setupHint = textLabel(QString());
    m_setupHint->setObjectName(QStringLiteral("remoteSetupHint"));
    layout->addWidget(m_setupHint);
    m_setup = new QPushButton(tr("Set up remote access…"));
    m_setup->setObjectName(QStringLiteral("primaryButton"));
    layout->addWidget(m_setup, 0, Qt::AlignLeft);
    auto *check = new QPushButton(tr("Check connection"));
    check->setObjectName(QStringLiteral("checkRemoteConnection"));
    m_actions.append(check);
    connect(check, &QPushButton::clicked, this, &RemoteWidget::refresh);
    layout->addWidget(check, 0, Qt::AlignLeft);
    connect(m_resolver, QOverload<int>::of(&QComboBox::currentIndexChanged), this, [this]() {
        m_preferredResolver = m_resolver->currentData().toString();
        updateSetup();
    });
    connect(m_hostname, &QLineEdit::textChanged, this, &RemoteWidget::updateSetup);
    connect(m_setup, &QPushButton::clicked, this, [this]() {
        const int index = m_resolver->currentIndex();
        if (index < 0 || index >= m_resolvers.size())
            return;
        const auto &resolver = m_resolvers[index];
        const bool hosted = resolver.remoteSetup == "hosted";
        const QString notice = hosted
            ? tr("Sign in using %1? Its package may install dependencies and open browser authentication. Your broker will be hosted by the provider.").arg(resolver.name)
            : tr("Set up %1 using %2? This may install dependencies, create a public tunnel and DNS record, and start a broker user service.")
                  .arg(m_hostname->text().trimmed(), resolver.name);
        if (QMessageBox::question(this, tr("Set up remote access"), notice,
                QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
            return;
        QStringList arguments{"remote", "setup", "--resolver", resolver.name};
        if (!hosted)
            arguments << "--hostname" << m_hostname->text().trimmed();
        if (!m_workdir.isEmpty())
            arguments << "--workdir" << m_workdir;
        QString error;
        m_status->show();
        if (openSetupTerminal(DockpipeChoices::preferredDockpipeBinary(m_workdir), arguments, &error))
            m_status->setText(tr("Continue setup in the terminal. When it finishes, return to Machines and refresh to check access."));
        else
            m_status->setText(error);
    });
    layout->addWidget(textLabel(tr("Pairing security keys"), "sectionTitle"));
    layout->addWidget(textLabel(tr("Register a physical security key before approving machines. Stop the broker before registering. For the first key, choose a trusted manufacturer attestation-root PEM file; Dockpipe does not download or select trust roots for you."), "hintText"));
    auto *registerKey = new QPushButton(tr("Register pairing key…"), page);
    registerKey->setObjectName(QStringLiteral("registerPairingKey"));
    auto *registrationChoices = new QMenu(registerKey);
    auto *firstKey = registrationChoices->addAction(tr("Register first pairing key…"));
    auto *spareKey = registrationChoices->addAction(tr("Add spare pairing key…"));
    registerKey->setMenu(registrationChoices);
    m_actions.append(registerKey);
    layout->addWidget(registerKey, 0, Qt::AlignLeft);
    const auto registerPairingKey = [this](bool first) {
        QStringList arguments{"security-key-register", "--interaction", "gui"};
        if (first) {
            const QString roots = QFileDialog::getOpenFileName(this,
                tr("Choose trusted manufacturer attestation roots"), QString(),
                tr("PEM certificates (*.pem);;All files (*)"));
            if (roots.isEmpty())
                return;
            arguments << "--attestation-roots" << roots;
        }
        run(arguments, [this](bool ok, const QByteArray &) {
            m_status->setText(ok ? tr("Pairing security key registered.")
                : tr("Security-key registration did not complete. Check details before retrying."));
            m_status->show();
        }, 240000);
    };
    connect(firstKey, &QAction::triggered, this, [registerPairingKey]() { registerPairingKey(true); });
    connect(spareKey, &QAction::triggered, this, [registerPairingKey]() { registerPairingKey(false); });
    updateSetup();
    return page;
}

void RemoteWidget::showPage(int index)
{
    m_pages->setCurrentIndex(index);
    updateManagementPresentation();
    if (index == Pairing) {
        setPairingStep(0);
    }
    if (m_canvasView && m_canvasView->isChecked()) {
        if (index == Overview) {
            m_managementDialog->reject();
        } else {
            m_managementDialog->layout()->addWidget(m_management);
            m_management->show();
            m_managementDialog->open();
        }
    }
    if (!m_command.busy()) {
        m_status->clear();
        m_status->hide();
    }
    if (index == Pairing && m_brokerAvailable) {
        m_pairingTimer->start();
        if (!m_command.busy())
            refreshPairings();
    } else {
        m_pairingTimer->stop();
    }
    if (index == Overview && isVisible())
        refreshSecurityStatus();
}

void RemoteWidget::updateManagementPresentation()
{
    const bool inDialog = m_canvasView && m_canvasView->isChecked();
    m_management->layout()->setContentsMargins(inDialog ? 0 : 20, inDialog ? 0 : 16,
                                               inDialog ? 0 : 20, inDialog ? 0 : 16);
    m_pairingBack->setVisible(!inDialog);
    m_pairingTitle->setVisible(!inDialog);
    if (m_managementDialog) {
        const QStringList titles{tr("Machines"), tr("Add machine"), tr("Connect this computer"), tr("Connection setup")};
        m_managementDialog->setWindowTitle(titles.value(m_pages->currentIndex(), tr("Machine setup")));
    }
}

void RemoteWidget::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_securityStatusTimer)
        m_securityStatusTimer->start();
    if (m_pages && m_pages->currentIndex() == Overview)
        refreshSecurityStatus();
}

void RemoteWidget::updateActions()
{
    const bool idle = !m_command.busy();
    for (auto *button : m_actions)
        button->setEnabled(idle);
    if (!m_request || !m_cancel)
        return;
    updateSetup();
    m_cancel->setVisible(!idle);
    m_cancel->setEnabled(!idle);
    m_endpoint->setEnabled(idle);
    m_name->setEnabled(idle);
    m_allow->setEnabled(idle);
    const QUrl endpoint(m_endpoint->text().trimmed());
    m_request->setEnabled(idle && endpoint.isValid() && endpoint.scheme() == "https"
                          && !endpoint.host().isEmpty() && !m_name->text().trimmed().isEmpty() && m_allow->isChecked());
    m_approve->setEnabled(idle && m_pairings->currentRow() >= 0);
    m_deny->setEnabled(idle && m_pairings->currentRow() >= 0);
    m_reviewPairing->setVisible(m_pairings->rowCount() > 0);
    m_reviewPairing->setEnabled(idle && m_pairings->rowCount() > 0);
    m_startPairing->setText(m_pairingEndpoint.isEmpty() ? tr("Set up connection") : tr("Start pairing"));
    updateMachineSettings();
}

void RemoteWidget::setWorkdir(const QString &workdir)
{
    m_workdir = workdir;
}

void RemoteWidget::run(const QStringList &arguments, RemoteCommand::Completion completion,
                       int timeoutMs, bool allowAuthentication)
{
    if (m_command.busy())
        return;
    if (allowAuthentication && timeoutMs < 150000)
        timeoutMs = 150000;
    m_output->clear();
    m_pendingOutput.clear();
    const bool listing = arguments.first() == "nodes" || arguments.first() == "pairings" || arguments.first() == "info";
    if (!listing) {
        m_status->show();
        m_status->setText(tr("Working…"));
    }
    QStringList args{"remote"};
    args.append(arguments);
    args << QStringLiteral("--auth")
         << (allowAuthentication ? QStringLiteral("auto") : QStringLiteral("never"));
    QString program;
    QString error;
    program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        m_status->setText(error);
        m_status->show();
        m_output->appendPlainText(error);
        return;
    }
    m_command.run(program, args,
                  [this, completion](bool ok, const QByteArray &output) {
        if (!ok)
            m_detailsToggle->show();
        m_status->setText(ok ? QString() : tr("Could not complete this step. See details for the error; saved setup is preserved."));
        m_status->setVisible(!ok);
        if (completion)
            completion(ok, output);
        if (!m_status->text().isEmpty())
            m_status->show();
    }, timeoutMs);
}

void RemoteWidget::refreshNodes(bool allowAuthentication)
{
    run({"nodes"}, [this, allowAuthentication](bool ok, const QByteArray &data) {
        m_brokerAvailable = ok;
        m_machineInventoryLoaded = ok;
        const int selectedRow = m_nodes->currentRow();
        const QString selectedName = selectedRow < 0 ? QString() : m_nodes->item(selectedRow, 0)->text();
        const QSignalBlocker selectionBlocker(m_nodes);
        m_nodes->setRowCount(0);
        if (!ok) {
            m_overviewHint->setText(tr("Couldn't load managed machines. Retry, or use Connect this computer to join a machine you've already set up."));
            m_review->hide();
            updateMachineGraph();
            emit nodesChanged({});
            updateActions();
            return;
        }
        const auto nodes = QJsonDocument::fromJson(data).array();
        QStringList paired;
        for (const auto &value : nodes) {
            const auto node = value.toObject();
            const QString name = node.value("node").toString();
            const QString status = node.value("status").toString();
            QString access = status;
            QString explanation;
            if (status == "paired") {
                access = tr("Paired");
                explanation = tr("Allowed to receive workflows");
                paired.append(name);
            } else if (status == "invited") {
                access = tr("Invitation created");
                explanation = tr("Not connected yet — use Add another machine");
            } else if (status == "revoked") {
                access = tr("Access removed");
                explanation = tr("Pair again to restore access");
            }
            const int row = m_nodes->rowCount();
            m_nodes->insertRow(row);
            auto *nameItem = new QTableWidgetItem(name);
            nameItem->setData(Qt::UserRole, status);
            m_nodes->setItem(row, 0, nameItem);
            m_nodes->setItem(row, 1, new QTableWidgetItem(access));
            auto *meaning = new QTableWidgetItem(explanation);
            meaning->setToolTip(explanation);
            m_nodes->setItem(row, 2, meaning);
            auto *settings = new QPushButton(tr("Settings"));
            settings->setObjectName("machineRowSettings");
            settings->setAccessibleName(tr("Settings for %1").arg(name));
            settings->setEnabled(!m_command.busy());
            connect(&m_command, &RemoteCommand::busyChanged, settings, [settings](bool busy) {
                settings->setEnabled(!busy);
            });
            connect(settings, &QPushButton::clicked, this, [this, name]() { openMachineSettings(name); });
            m_nodes->setCellWidget(row, 3, settings);
            if (name == selectedName)
                m_nodes->selectRow(row);
        }
        m_nodes->setVisible(!nodes.isEmpty());
        const int tableHeight = m_nodes->horizontalHeader()->sizeHint().height()
            + nodes.size() * m_nodes->verticalHeader()->defaultSectionSize() + 4;
        m_nodes->setFixedHeight(qBound(90, tableHeight, 260));
        m_overviewHint->setText(nodes.isEmpty() ? tr("No machines added yet. Choose Add another machine to get started.")
                                               : tr("Add a computer here, then choose it when launching a workflow."));
        updateMachineGraph();
        emit nodesChanged(paired);
        updateActions();
        refreshPairings(allowAuthentication);
    }, allowAuthentication ? 150000 : 30000, allowAuthentication);
}

void RemoteWidget::refreshSecurityStatus()
{
    if (!m_securityStatus || m_securityStatusCommand.busy())
        return;

    QString error;
    const QString program = DockpipeChoices::operatorDockpipeBinary(&error);
    if (program.isEmpty()) {
        m_securityStatus->setText(tr("Remote access lock status unavailable: %1").arg(error));
        return;
    }

    m_securityStatusCommand.run(program,
        {QStringLiteral("remote"), QStringLiteral("security-status"),
         QStringLiteral("--auth"), QStringLiteral("never")},
        [this](bool ok, const QByteArray &output) {
            if (!ok) {
                m_securityStatus->setText(tr("Remote access lock status unavailable. Open Settings > Security for details."));
                return;
            }

            const QJsonDocument document = QJsonDocument::fromJson(output);
            if (!document.isObject()) {
                m_securityStatus->setText(tr("Remote access lock status unavailable: invalid response."));
                return;
            }

            const QJsonObject status = document.object();
            if (!status.value(QStringLiteral("authority_available")).isBool()
                || !status.value(QStringLiteral("locked")).isBool()) {
                m_securityStatus->setText(tr("Remote access lock status unavailable: incomplete response."));
                return;
            }
            if (!status.value(QStringLiteral("authority_available")).toBool()) {
                m_securityStatus->setText(tr("Remote access lock status unavailable: no local security authority."));
                return;
            }

            const bool locked = status.value(QStringLiteral("locked")).toBool();
            QString text = locked ? tr("Remote access is locked.") : tr("Remote access is unlocked.");
            if (!status.value(QStringLiteral("supported")).toBool()) {
                const QJsonObject capabilities = status.value(QStringLiteral("capabilities")).toObject();
                QStringList unavailable;
                if (!capabilities.value(QStringLiteral("screen_lock")).toBool())
                    unavailable.append(tr("screen lock"));
                if (!capabilities.value(QStringLiteral("sleep")).toBool())
                    unavailable.append(tr("sleep"));
                if (!unavailable.isEmpty())
                    text += tr(" Native %1 detection unavailable; remote authority still enforces its lock policy.")
                        .arg(unavailable.join(tr(" and ")));
            }
            m_securityStatus->setText(text);
        }, 5000);
}

void RemoteWidget::refreshPairings(bool allowAuthentication)
{
    run({"pairings"}, [this](bool ok, const QByteArray &requests) {
        if (!ok) {
            m_pairingTimer->stop();
            m_pairings->setRowCount(0);
            m_review->hide();
            updateActions();
            m_pairingHint->setText(tr("Couldn't check requests. Use Check for requests to retry."));
            m_pairingWaiting->setText(tr("Couldn't check for requests. Choose Check again to retry."));
            setPairingStep(m_pairingSteps->currentIndex());
            return;
        }
        const int selectedRow = m_pairings->currentRow();
        const QString selectedCode = selectedRow < 0 ? QString() : m_pairings->item(selectedRow, 1)->text();
        m_pairings->setRowCount(0);
        for (const auto &value : QJsonDocument::fromJson(requests).array()) {
            const auto request = value.toObject();
            const int row = m_pairings->rowCount();
            m_pairings->insertRow(row);
            m_pairings->setItem(row, 0, new QTableWidgetItem(request.value("node").toString()));
            const QString code = request.value("code").toString();
            m_pairings->setItem(row, 1, new QTableWidgetItem(code));
            const auto expiry = QDateTime::fromString(request.value("expires_at").toString(), Qt::ISODate);
            m_pairings->setItem(row, 2, new QTableWidgetItem(expiry.toLocalTime().toString("HH:mm")));
            if (selectedCode == code)
                m_pairings->selectRow(row);
        }
        const int count = m_pairings->rowCount();
        m_pairingWaiting->setText(count == 0
            ? tr("Waiting for the other computer. Requests appear here automatically.")
            : tr("A connection request is ready. Review it to continue."));
        m_reviewPairing->setText(count == 1 ? tr("Review request") : tr("Review requests (%1)").arg(count));
        m_pairings->setVisible(count > 0);
        m_pairings->setFixedHeight(qBound(90, 42 + count * 48, 190));
        m_pairingHint->setText(count == 0 ? tr("No requests yet. Continue on the other computer; requests appear here automatically.")
                                          : tr("Select a request. Approve only when the other computer shows the same code."));
        if (count == 0 && !m_requestOutcome.isEmpty())
            m_pairingHint->setText(m_requestOutcome);
        m_review->setText(tr("Review connection requests (%1)").arg(count));
        m_review->setVisible(count > 0);
        updateActions();
        setPairingStep(m_pairingSteps->currentIndex());
    }, allowAuthentication ? 150000 : 30000, allowAuthentication);
}

void RemoteWidget::decidePairing(bool approve)
{
    const int row = m_pairings->currentRow();
    if (row < 0)
        return;
    const QString code = m_pairings->item(row, 1)->text();
    const QString name = m_pairings->item(row, 0)->text();
    if (approve && QMessageBox::question(this, tr("Verify this machine"),
            tr("Does %1 show exactly this code?\n\n%2\n\nApprove only if you can verify it on that machine.").arg(name, code),
            QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
        return;
    QStringList arguments{approve ? "approve" : "deny", "--code", code};
    if (approve)
        arguments << "--interaction" << "gui";
    run(arguments, [this, approve, name](bool ok, const QByteArray &) {
        if (ok) {
            m_requestOutcome = approve ? tr("%1 is paired. On that computer, choose Start worker service to receive workflows.").arg(name)
                                       : tr("Request from %1 denied.").arg(name);
            if (approve) {
                m_pairingSuccess->setText(tr("%1 is paired").arg(name));
                setPairingStep(3);
            }
            refresh();
        }
    }, approve ? 240000 : 30000);
}

void RemoteWidget::setResolvers(const QVector<ResolverMeta> &resolvers)
{
    const QString selected = m_preferredResolver.isEmpty() ? m_resolver->currentData().toString() : m_preferredResolver;
    const QSignalBlocker blocker(m_resolver);
    m_resolver->clear();
    m_resolvers.clear();
    for (const auto &resolver : resolvers) {
        if (resolver.remoteSetup != "local" && resolver.remoteSetup != "hosted")
            continue;
        m_resolvers.append(resolver);
        QString label = resolver.title.isEmpty() ? resolver.name : resolver.title;
        if (!resolver.version.isEmpty())
            label += " · " + resolver.version;
        m_resolver->addItem(label, resolver.name);
    }
    const int previous = m_resolver->findData(selected);
    if (previous >= 0)
        m_resolver->setCurrentIndex(previous);
    updateSetup();
}

void RemoteWidget::selectResolver(const QString &name)
{
    m_preferredResolver = name;
    const int index = m_resolver->findData(name);
    if (index >= 0)
        m_resolver->setCurrentIndex(index);
    updateSetup();
}

void RemoteWidget::updateSetup()
{
    const bool idle = !m_command.busy();
    const int index = m_resolver->currentIndex();
    const bool available = index >= 0 && index < m_resolvers.size();
    const bool hosted = available && m_resolvers[index].remoteSetup == "hosted";
    m_resolver->setEnabled(idle && available);
    m_installProvider->setEnabled(idle);
    m_installProvider->setText(available ? tr("Add provider…") : tr("Install provider…"));
    m_hostname->setVisible(available && !hosted);
    m_hostnameLabel->setVisible(available && !hosted);
    m_hostname->setEnabled(idle);
    m_setup->setEnabled(idle && available && (hosted || !m_hostname->text().trimmed().isEmpty()));
    m_setup->setText(hosted ? tr("Sign in and connect…") : tr("Set up remote access…"));
    if (!available) {
        m_setupHint->setText(tr("Install a provider to continue. You'll return here with it selected, ready to complete setup."));
        return;
    }
    const auto &resolver = m_resolvers[index];
    const QString mode = hosted ? tr("Hosted broker — the provider runs the service. No hostname or local broker setup is needed.")
                                : tr("Broker on this computer — the provider connects it to your public hostname. Keep this computer available for remote work.");
    QStringList details{mode};
    if (!resolver.description.isEmpty())
        details.append(resolver.description);
    details.append(hosted
        ? tr("Setup opens a terminal for browser sign-in and checks your connection. When it finishes, choose Check connection.")
        : tr("Setup opens a terminal for installation and browser sign-in, then configures your tunnel, DNS and broker service. When it finishes, choose Check connection."));
    m_setupHint->setText(details.join("\n\n"));
}

void RemoteWidget::refresh(bool allowAuthentication)
{
    run({"info"}, [this, allowAuthentication](bool ok, const QByteArray &data) {
        const auto document = QJsonDocument::fromJson(data);
        if (!ok || !document.isObject()) {
            m_connectionSummary->setText(tr("Connection details unavailable. Update the CLI if it does not support remote info."));
            m_pairingEndpoint.clear();
            m_pairingAddress->setText(tr("Connection address unavailable."));
            refreshNodes(allowAuthentication);
            return;
        }
        const auto info = document.object();
        updateServiceNotices(info);
        const auto broker = info.value("broker").toObject();
        const auto worker = info.value("worker").toObject();
        m_existingConnection->hide();
        if (!broker.isEmpty()) {
            m_graphRemoteEndpoint.clear();
            const auto resolver = broker.value("resolver").toObject();
            QString provider = resolver.value("title").toString();
            const QString name = resolver.value("name").toString();
            if (name != m_savedResolver) {
                m_savedResolver = name;
                selectResolver(name);
            }
            if (provider.isEmpty())
                provider = name;
            if (!name.isEmpty() && provider != name)
                provider += " (" + name + ")";
            const QString version = resolver.value("version").toString();
            if (!version.isEmpty())
                provider += " · " + version;
            if (provider.isEmpty())
                provider = tr("Choose your provider in Review remote setup.");
            const QString mode = broker.value("mode").toString() == "hosted" ? tr("Hosted broker") : tr("Broker on this computer");
            const QString address = broker.value("endpoint").toString();
            m_existingConnection->setText(tr("Current remote address: %1\nUse the provider that manages this address.").arg(address));
            m_existingConnection->show();
            if (m_hostname->text().isEmpty())
                m_hostname->setText(QUrl(address).host());
            m_connectionSummary->setText(provider + "\n" + mode + " · " + address);
            m_pairingEndpoint = address;
            m_pairingAddress->setText(address);
            refreshNodes(allowAuthentication);
            return;
        }
        m_graphRemoteEndpoint = worker.value("endpoint").toString();
        m_connectionSummary->setText(worker.isEmpty()
            ? tr("No remote connection configured. Set up remote access, or connect this computer to an existing broker.")
            : tr("This computer is paired as %1\nRemote address: %2\nThe provider is configured on the managing computer.")
                  .arg(worker.value("node").toString(), worker.value("endpoint").toString()));
        m_pairingEndpoint.clear();
        m_pairingAddress->setText(tr("Set up a connection before adding another computer."));
        m_brokerAvailable = false;
        m_machineInventoryLoaded = false;
        m_pairingTimer->stop();
        m_nodes->setRowCount(0);
        m_nodes->hide();
        m_review->hide();
        m_overviewHint->setText(tr("No broker is managed from this computer."));
        updateMachineGraph();
        emit nodesChanged({});
        updateActions();
    }, allowAuthentication ? 150000 : 30000, allowAuthentication);
}

void RemoteWidget::updateMachineGraph()
{
    updateMachineList();
    if (!m_canvas)
        return;
    QVector<ResourceNode> nodes;
    QVector<ResourceEdge> edges;
    nodes.append({"@local", "computer", tr("This computer"), m_connectionSummary->text(),
                  m_brokerAvailable ? tr("Manages pairing") : tr("Remote setup"), 0});
    if (!m_brokerAvailable && !m_graphRemoteEndpoint.isEmpty()) {
        nodes.append({"@broker", "broker", tr("Managing broker"), m_graphRemoteEndpoint,
                      tr("Configured address"), 1});
        edges.append({"@local", "@broker", tr("Worker configured for this broker; connectivity not verified"), false});
    }
    for (int row = 0; row < m_nodes->rowCount(); ++row) {
        const QString name = m_nodes->item(row, 0)->text();
        const QString access = m_nodes->item(row, 1)->text();
        nodes.append({name, "machine", name, m_nodes->item(row, 2)->text(), access, 1});
        edges.append({"@local", name, tr("Access relationship: %1").arg(access), false});
    }
    m_canvas->setGraph(nodes, edges);
}

void RemoteWidget::setListView(bool list)
{
    if (m_machineSettingsDialog->isVisible())
        m_machineSettingsDialog->reject();
    m_managementDialog->reject();
    QWidget *host = list ? m_listManagementHost : m_managementDialog;
    host->layout()->addWidget(m_management);
    m_management->show();
    m_views->setCurrentWidget(list ? m_listManagementHost : m_canvas);
    m_listView->setChecked(list);
    m_canvasView->setChecked(!list);
    updateManagementPresentation();
    if (m_pages->currentIndex() == Pairing)
        setPairingStep(m_pairingSteps->currentIndex());
    m_canvasAdd->setVisible(!list);
    if (!list && m_pages->currentIndex() != Overview)
        m_managementDialog->open();
    QSettings().setValue("machines/viewMode", list ? "list" : "canvas");
}

void RemoteWidget::updateMachineList()
{
    if (!m_machineSearch)
        return;

    const QString search = m_machineSearch->text().trimmed();
    const QString access = m_accessFilter->currentData().toString();
    int visible = 0;
    for (int row = 0; row < m_nodes->rowCount(); ++row) {
        const auto *name = m_nodes->item(row, 0);
        if (!name || !m_nodes->item(row, 1) || !m_nodes->item(row, 2))
            continue;
        const QString state = name->data(Qt::UserRole).toString();
        const QString searchable = name->text() + QLatin1Char(' ') + m_nodes->item(row, 1)->text()
            + QLatin1Char(' ') + m_nodes->item(row, 2)->text();
        const bool matches = searchable.contains(search, Qt::CaseInsensitive)
            && (access.isEmpty() || state == access);
        m_nodes->setRowHidden(row, !matches);
        if (matches)
            ++visible;
    }
    m_overviewHint->setVisible(!m_machineInventoryLoaded || m_nodes->rowCount() == 0);
    const bool hasVisibleMachines = m_machineInventoryLoaded && visible > 0;
    m_nodes->setVisible(hasVisibleMachines);

    if (!m_machineInventoryLoaded) {
        m_listResults->setText(tr("Managed machine inventory is unavailable. Refresh to check access."));
    } else if (m_nodes->rowCount() == 0) {
        m_listResults->setText(tr("No managed machines yet. Add another machine to grant access."));
    } else if (visible == 0) {
        m_listResults->setText(tr("No machines match your search and access filter."));
    } else if (visible == m_nodes->rowCount()) {
        m_listResults->setText(visible == 1 ? tr("1 machine") : tr("%1 machines").arg(visible));
    } else {
        m_listResults->setText(tr("%1 of %2 machines")
                               .arg(visible).arg(m_nodes->rowCount()));
    }
    const int row = m_nodes->currentRow();
    const bool selected = row >= 0 && m_nodes->item(row, 0) && m_nodes->item(row, 2)
        && !m_nodes->isRowHidden(row) && m_nodes->selectionModel()->isRowSelected(row, QModelIndex());
    if (m_listView && m_listView->isChecked() && !m_machineSettingsDialog->isVisible())
        m_settingsNode = selected ? m_nodes->item(row, 0)->text() : QString();
    updateMachineSettings();
}

void RemoteWidget::openMachineSettings(const QString &node)
{
    if (m_command.busy())
        return;
    m_settingsNode = node;
    m_machineSettingsError->hide();
    for (int row = 0; row < m_nodes->rowCount(); ++row) {
        if (m_nodes->item(row, 0)->text() == node) {
            m_nodes->selectRow(row);
            break;
        }
    }
    m_machineSettingsDialog->setWindowTitle(tr("%1 settings").arg(node));
    auto *layout = qobject_cast<QVBoxLayout *>(m_machineSettingsDialog->layout());
    layout->insertWidget(0, m_machineSettingsPanel, 1);
    m_machineSettingsPanel->show();
    updateMachineSettings();
    m_machineSettingsDialog->adjustSize();
    m_machineSettingsDialog->open();
}

void RemoteWidget::updateMachineSettings()
{
    if (!m_machineSettingsPanel)
        return;
    int row = -1;
    for (int index = 0; index < m_nodes->rowCount(); ++index) {
        if (m_nodes->item(index, 0)->text() == m_settingsNode) {
            row = index;
            break;
        }
    }
    const bool available = m_machineInventoryLoaded && row >= 0;
    const bool idle = !m_command.busy();
    m_revoke->setVisible(available);
    m_removeMachine->setVisible(available);
    m_revoke->setEnabled(available && idle && m_nodes->item(row, 0)->data(Qt::UserRole).toString() != "revoked");
    m_removeMachine->setEnabled(available && idle);
    m_selectedMachine->show();
    m_localSettings->setVisible(m_settingsNode == "@local");
    if (available) {
        m_selectedMachine->setText(tr("%1 · %2\n\n%3\n\nConnection health is not checked here.")
            .arg(m_settingsNode, m_nodes->item(row, 1)->text(), m_nodes->item(row, 2)->text()));
    } else if (m_settingsNode == "@local") {
        m_selectedMachine->setText(tr("This computer\n\n%1\n\n%2")
            .arg(m_connectionSummary->text(), m_securityStatus->text()));
    } else if (m_settingsNode == "@broker") {
        m_selectedMachine->setText(tr("Managing broker\n\n%1\n\nConnectivity is not checked here.").arg(m_graphRemoteEndpoint));
    } else {
        m_selectedMachine->setText(m_settingsNode.isEmpty()
            ? tr("Select a machine to view its settings.")
            : tr("%1\n\nThis machine is no longer in the available inventory.").arg(m_settingsNode));
    }
}
