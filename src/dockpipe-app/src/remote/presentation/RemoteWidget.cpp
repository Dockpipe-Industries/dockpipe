#include "RemoteWidget.h"
#include "HardwareKeyDialog.h"
#include "ResourceCanvas.h"
#include "DockpipeChoices.h"
#include "SetupTerminal.h"

#include <QCheckBox>
#include <QComboBox>
#include <QSignalBlocker>
#include <QDateTime>
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
    auto *root = new QVBoxLayout(this);
    root->setContentsMargins(28, 24, 28, 24);
    root->setSpacing(14);
    root->addWidget(textLabel(tr("Machines"), "appTitle"));
    root->addWidget(textLabel(tr("Choose which computers can run your workflows.")));

    m_pages = new QStackedWidget;
    m_pages->setObjectName(QStringLiteral("machinePages"));
    // Each screen scrolls independently so hidden setup forms cannot enlarge the overview.
    const auto addPage = [this](QWidget *page) {
        auto *scroll = new QScrollArea;
        scroll->setObjectName(QStringLiteral("machineScroll"));
        scroll->setFrameShape(QFrame::NoFrame);
        scroll->setWidgetResizable(true);
        auto *content = new QWidget;
        content->setObjectName(QStringLiteral("machineContent"));
        content->setAttribute(Qt::WA_StyledBackground);
        auto *layout = pageLayout(content);
        page->setMaximumWidth(1120);
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
    details->setCheckable(true);
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
    auto *detailsPanel = new QWidget;
    detailsPanel->setLayout(root);
    m_canvas = new ResourceCanvas("machines", tr("Machines"),
        tr("Pairing and access relationships · Connection health is not checked here"), detailsPanel, this);
    auto *canvasLayout = new QVBoxLayout(this);
    canvasLayout->setContentsMargins(0, 0, 0, 0);
    canvasLayout->addWidget(m_canvas);
    connect(m_canvas, &ResourceCanvas::resourceSelected, this, [this](const QString &id) {
        showPage(0);
        for (int row = 0; row < m_nodes->rowCount(); ++row) {
            if (m_nodes->item(row, 0)->text() == id)
                m_nodes->selectRow(row);
        }
    });
    updateMachineGraph();

}

QWidget *RemoteWidget::buildOverview()
{
    auto *page = new QWidget;
    auto *layout = pageLayout(page);
    layout->addWidget(textLabel(tr("Remote connection"), "sectionTitle"));
    m_connectionSummary = textLabel(tr("Loading connection details…"));
    m_connectionSummary->setObjectName(QStringLiteral("remoteConnectionSummary"));
    m_connectionSummary->setTextInteractionFlags(Qt::TextSelectableByMouse);
    layout->addWidget(m_connectionSummary);
    m_brokerServiceHint = textLabel(QString(), "hintText");
    m_workerServiceHint = textLabel(QString(), "hintText");
    m_updateBrokerService = new QPushButton(tr("Update broker service…"));
    m_updateWorkerService = new QPushButton(tr("Update worker service…"));
    m_brokerServiceHint->setObjectName(QStringLiteral("brokerServiceHint"));
    m_workerServiceHint->setObjectName(QStringLiteral("workerServiceHint"));
    for (auto *hint : {m_brokerServiceHint, m_workerServiceHint}) {
        layout->addWidget(hint);
        hint->hide();
    }
    for (auto *button : {m_updateBrokerService, m_updateWorkerService}) {
        button->setObjectName(QStringLiteral("secondaryButton"));
        layout->addWidget(button, 0, Qt::AlignLeft);
        m_actions.append(button);
        button->hide();
    }
    connect(m_updateBrokerService, &QPushButton::clicked, this, [this]() { replaceService("broker"); });
    connect(m_updateWorkerService, &QPushButton::clicked, this, [this]() { replaceService("worker"); });
    auto *actions = new QHBoxLayout;
    auto *add = actionButton(tr("Add another machine"), actions, "primaryButton");
    auto *join = actionButton(tr("Connect this computer"), actions);
    actions->addStretch();
    auto *refreshButton = actionButton(tr("Refresh"), actions, "quietButton");
    m_actions.append(refreshButton);
    connect(add, &QPushButton::clicked, this, [this]() { showPage(Pairing); });
    connect(join, &QPushButton::clicked, this, [this]() { showPage(Connection); });
    connect(refreshButton, &QPushButton::clicked, this, [this]() { refresh(true); });
    layout->addLayout(actions);
    m_securityStatus = textLabel(tr("Checking remote access lock status…"), "hintText");
    m_securityStatus->setObjectName(QStringLiteral("remoteLockStatus"));
    layout->addWidget(m_securityStatus);
    layout->addWidget(textLabel(tr("Machines managed from this computer"), "sectionTitle"));
    m_overviewHint = textLabel(tr("Loading machines…"));
    layout->addWidget(m_overviewHint);
    m_review = new QPushButton;
    m_review->setObjectName(QStringLiteral("secondaryButton"));
    m_review->hide();
    connect(m_review, &QPushButton::clicked, this, [this]() { showPage(Pairing); });
    layout->addWidget(m_review, 0, Qt::AlignLeft);
    m_nodes = table({tr("Machine"), tr("Access"), tr("What this means")});
    m_nodes->setObjectName(QStringLiteral("managedMachines"));
    layout->addWidget(m_nodes);
    auto *management = new QHBoxLayout;
    m_revoke = actionButton(tr("Remove access…"), management, "quietButton");
    m_actions.append(m_revoke);
    management->addStretch();
    connect(m_nodes, &QTableWidget::itemSelectionChanged, this, &RemoteWidget::updateActions);
    connect(m_revoke, &QPushButton::clicked, this, [this]() {
        const int row = m_nodes->currentRow();
        if (row < 0)
            return;
        const QString node = m_nodes->item(row, 0)->text();
        if (QMessageBox::question(this, tr("Remove machine access"),
                tr("Remove access for %1? Its credential will stop working. This does not undo work already executed.").arg(node),
                QMessageBox::Yes | QMessageBox::No, QMessageBox::No) != QMessageBox::Yes)
            return;
        run({"revoke", "--id", node}, [this](bool ok, const QByteArray &) {
            if (ok)
                refresh();
        });
    });
    layout->addLayout(management);
    layout->addWidget(textLabel(tr("Paired machines appear under Run on in Workflows. Their connection status is not checked here."), "hintText"));
    auto *setupActions = new QHBoxLayout;
    auto *setup = actionButton(tr("Set up remote access…"), setupActions, "quietButton");
    setup->setObjectName(QStringLiteral("remoteSetupNavigation"));
    setupActions->addStretch();
    connect(setup, &QPushButton::clicked, this, [this]() { showPage(Setup); });
    layout->addLayout(setupActions);
    return page;
}

QWidget *RemoteWidget::buildPairing()
{
    auto *page = new QWidget;
    auto *layout = pageLayout(page);
    auto *back = new QPushButton(tr("← Machines"));
    back->setObjectName(QStringLiteral("quietButton"));
    connect(back, &QPushButton::clicked, this, [this]() { showPage(Overview); });
    layout->addWidget(back, 0, Qt::AlignLeft);
    layout->addWidget(textLabel(tr("Add another machine"), "sectionTitle"));
    layout->addWidget(textLabel(tr("1. Allow a new connection here"), "sectionTitle"));
    m_windowHint = textLabel(tr("Open a 15-minute pairing window. This does not grant access until you approve a matching code."));
    layout->addWidget(m_windowHint);
    auto *actions = new QHBoxLayout;
    auto *open = actionButton(tr("Allow pairing for 15 minutes"), actions, "primaryButton");
    auto *close = actionButton(tr("Stop accepting requests"), actions, "quietButton");
    actions->addStretch();
    m_actions.append(open);
    m_actions.append(close);
    connect(open, &QPushButton::clicked, this, [this]() {
        run({"pairing-open"}, [this](bool ok, const QByteArray &) {
            if (!ok)
                return;
            m_requestOutcome.clear();
            m_windowHint->setText(tr("Pairing opened. Continue on the other computer, then approve its code below. The window expires after 15 minutes."));
            m_brokerAvailable = true;
            m_pairingTimer->start();
            refreshPairings();
        });
    });
    connect(close, &QPushButton::clicked, this, [this]() {
        run({"pairing-close"}, [this](bool ok, const QByteArray &) {
            if (ok) {
                m_windowHint->setText(tr("Pairing is closed. Open a new window when you want to add a machine."));
                refreshPairings();
            }
        });
    });
    layout->addLayout(actions);
    layout->addWidget(textLabel(tr("2. On the computer you want to add"), "sectionTitle"));
    layout->addWidget(textLabel(tr("Open Dockpipe → Machines → Connect this computer. Enter the remote address you set up for this computer, then request pairing.")));
    m_pairingAddress = textLabel(tr("Refresh Machines to see your configured remote address."));
    m_pairingAddress->setTextInteractionFlags(Qt::TextSelectableByMouse);
    layout->addWidget(m_pairingAddress);
    auto *reviewSetup = new QPushButton(tr("Review remote setup…"));
    reviewSetup->setObjectName(QStringLiteral("reviewRemoteSetup"));
    connect(reviewSetup, &QPushButton::clicked, this, [this]() { showPage(Setup); });
    layout->addWidget(reviewSetup, 0, Qt::AlignLeft);
    layout->addWidget(textLabel(tr("3. Compare and approve its code here"), "sectionTitle"));
    m_pairingHint = textLabel(tr("No requests yet. They appear here automatically while this page is open."));
    layout->addWidget(m_pairingHint);
    m_pairings = table({tr("Machine"), tr("Verification code"), tr("Expires")});
    m_pairings->setFixedHeight(150);
    m_pairings->setObjectName(QStringLiteral("pairingRequests"));
    layout->addWidget(m_pairings);
    auto *decisions = new QHBoxLayout;
    m_approve = actionButton(tr("Verify key and approve"), decisions, "primaryButton");
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
    layout->addLayout(decisions);
    return page;
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
    layout->addWidget(textLabel(tr("Use this page on the computer that will run the workflows. On your managing computer, choose Add another machine first.")));
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
    layout->addWidget(textLabel(tr("Only connect to a computer you trust. Supplied workflows can access this user's files and tools; pairing does not sandbox them."), "hintText"));
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
    layout->addWidget(textLabel(tr("After pairing"), "sectionTitle"));
    layout->addWidget(textLabel(tr("The worker starts automatically after pairing. Already paired, or setup interrupted? Start it here without pairing again. Keep this computer awake while work runs.")));
    auto *serviceActions = new QHBoxLayout;
    auto *start = actionButton(tr("Start worker service"), serviceActions);
    m_actions.append(start);
    serviceActions->addStretch();
    connect(start, &QPushButton::clicked, this, &RemoteWidget::startWorkerService);
    layout->addLayout(serviceActions);
    return page;
}

void RemoteWidget::updateServiceNotices(const QJsonObject &info)
{
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
    const int row = m_nodes->currentRow();
    const auto *selected = row < 0 ? nullptr : m_nodes->item(row, 0);
    m_revoke->setEnabled(idle && selected && selected->data(Qt::UserRole).toString() != "revoked");
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
        m_nodes->setRowCount(0);
        if (!ok) {
            m_overviewHint->setText(tr("Couldn't load managed machines. Retry, or use Connect this computer to join a machine you've already set up."));
            m_review->hide();
            updateMachineGraph();
        emit nodesChanged({});
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
        }
        m_nodes->setVisible(!nodes.isEmpty());
        m_nodes->setFixedHeight(qBound(90, 42 + nodes.size() * 48, 260));
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
        m_pairings->setVisible(count > 0);
        m_pairings->setFixedHeight(qBound(90, 42 + count * 48, 190));
        m_pairingHint->setText(count == 0 ? tr("No requests yet. Continue on the other computer; requests appear here automatically.")
                                          : tr("Select a request. Approve only when the other computer shows the same code."));
        if (count == 0 && !m_requestOutcome.isEmpty())
            m_pairingHint->setText(m_requestOutcome);
        m_review->setText(tr("Review connection requests (%1)").arg(count));
        m_review->setVisible(count > 0);
        updateActions();
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
            m_pairingAddress->setText(tr("Connection details unavailable. Use the address from your remote setup."));
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
            m_pairingAddress->setText(tr("Remote address: %1\nProvider: %2").arg(address, provider));
            refreshNodes(allowAuthentication);
            return;
        }
        m_graphRemoteEndpoint = worker.value("endpoint").toString();
        m_connectionSummary->setText(worker.isEmpty()
            ? tr("No remote connection configured. Set up remote access, or connect this computer to an existing broker.")
            : tr("This computer is paired as %1\nRemote address: %2\nThe provider is configured on the managing computer.")
                  .arg(worker.value("node").toString(), worker.value("endpoint").toString()));
        m_pairingAddress->setText(tr("Set up remote access before adding another machine."));
        m_brokerAvailable = false;
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
