#include "RemoteWidget.h"
#include "DockpipeChoices.h"
#include "SetupTerminal.h"

#include <QCheckBox>
#include <QComboBox>
#include <QSignalBlocker>
#include <QDateTime>
#include <QFormLayout>
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

RemoteWidget::RemoteWidget(QWidget *parent) : QWidget(parent), m_command(this)
{
    auto *root = new QVBoxLayout(this);
    root->setContentsMargins(28, 24, 28, 24);
    root->setSpacing(14);
    root->addWidget(textLabel(tr("Machines"), "appTitle"));
    root->addWidget(textLabel(tr("Choose which computers can run your workflows.")));

    auto *scroll = new QScrollArea;
    scroll->setObjectName(QStringLiteral("machineScroll"));
    scroll->setFrameShape(QFrame::NoFrame);
    scroll->setWidgetResizable(true);
    auto *content = new QWidget;
    content->setObjectName(QStringLiteral("machineContent"));
    content->setAttribute(Qt::WA_StyledBackground);
    auto *contentLayout = pageLayout(content);
    m_pages = new QStackedWidget;
    m_pages->setObjectName(QStringLiteral("machinePages"));
    m_pages->setMaximumWidth(1120);
    m_pages->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Maximum);
    m_pages->addWidget(buildOverview());
    m_pages->addWidget(buildPairing());
    m_pages->addWidget(buildConnection());
    m_pages->addWidget(buildSetup());
    contentLayout->addWidget(m_pages);
    m_status = textLabel(QString(), "hintText");
    m_status->setObjectName(QStringLiteral("machineStatus"));
    m_status->hide();
    contentLayout->addWidget(m_status);
    contentLayout->addStretch();
    scroll->setWidget(content);
    root->addWidget(scroll, 1);

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
    connect(&m_command, &RemoteCommand::busyChanged, this, &RemoteWidget::updateActions);
    m_pairingTimer = new QTimer(this);
    m_pairingTimer->setInterval(5000);
    connect(m_pairingTimer, &QTimer::timeout, this, [this]() {
        if (isVisible() && m_pages->currentIndex() == Pairing && !m_command.busy())
            refreshPairings();
    });
    updateActions();
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
    auto *actions = new QHBoxLayout;
    auto *add = actionButton(tr("Add another machine"), actions, "primaryButton");
    auto *join = actionButton(tr("Connect this computer"), actions);
    actions->addStretch();
    auto *refreshButton = actionButton(tr("Refresh"), actions, "quietButton");
    m_actions.append(refreshButton);
    connect(add, &QPushButton::clicked, this, [this]() { showPage(Pairing); });
    connect(join, &QPushButton::clicked, this, [this]() { showPage(Connection); });
    connect(refreshButton, &QPushButton::clicked, this, &RemoteWidget::refresh);
    layout->addLayout(actions);
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
    layout->addWidget(textLabel(tr("3. Compare and approve its code here"), "sectionTitle"));
    m_pairingHint = textLabel(tr("No requests yet. They appear here automatically while this page is open."));
    layout->addWidget(m_pairingHint);
    m_pairings = table({tr("Machine"), tr("Verification code"), tr("Expires")});
    m_pairings->setFixedHeight(150);
    m_pairings->setObjectName(QStringLiteral("pairingRequests"));
    layout->addWidget(m_pairings);
    auto *decisions = new QHBoxLayout;
    m_approve = actionButton(tr("Approve matching code"), decisions, "primaryButton");
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
                    m_status->setText(tr("Paired successfully. Start the worker below to receive workflows."));
            }, 16 * 60 * 1000);
        m_status->setText(tr("Waiting for approval. Compare the code below on the managing computer."));
    });
    m_pairingCode = textLabel(QString(), "pairingCode");
    m_pairingCode->setTextInteractionFlags(Qt::TextSelectableByMouse);
    m_pairingCode->setStyleSheet(QStringLiteral("font-size: 26px; font-weight: 600; padding: 12px 0;"));
    m_pairingCode->hide();
    layout->addWidget(m_pairingCode);
    layout->addWidget(textLabel(tr("After pairing"), "sectionTitle"));
    layout->addWidget(textLabel(tr("Start the background worker to receive workflows. Already paired? You can start it directly. Keep this computer awake while work runs.")));
    auto *serviceActions = new QHBoxLayout;
    auto *start = actionButton(tr("Start worker service"), serviceActions);
    m_actions.append(start);
    serviceActions->addStretch();
    connect(start, &QPushButton::clicked, this, [this]() {
        run({"service", "--role", "worker"}, [this](bool ok, const QByteArray &) {
            if (ok)
                m_status->setText(tr("Worker service started. Choose this computer under Run on from your managing computer."));
        });
    });
    layout->addLayout(serviceActions);
    return page;
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
    layout->addWidget(textLabel(tr("Choose an installed provider. It handles authentication and connects your machines through a broker.")));
    auto *form = new QFormLayout;
    form->setSpacing(12);
    m_resolver = new QComboBox;
    m_resolver->setObjectName(QStringLiteral("remoteResolver"));
    m_hostname = new QLineEdit;
    m_hostname->setObjectName(QStringLiteral("remoteHostname"));
    m_hostname->setPlaceholderText(tr("remote.example.com"));
    m_hostnameLabel = textLabel(tr("Public hostname (if required)"));
    form->addRow(tr("Remote-access resolver"), m_resolver);
    form->addRow(m_hostnameLabel, m_hostname);
    layout->addLayout(form);
    m_setupHint = textLabel(QString());
    m_setupHint->setObjectName(QStringLiteral("remoteSetupHint"));
    layout->addWidget(m_setupHint);
    m_setup = new QPushButton(tr("Set up remote access…"));
    m_setup->setObjectName(QStringLiteral("primaryButton"));
    layout->addWidget(m_setup, 0, Qt::AlignLeft);
    connect(m_resolver, QOverload<int>::of(&QComboBox::currentIndexChanged), this, &RemoteWidget::updateSetup);
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

void RemoteWidget::run(const QStringList &arguments, RemoteCommand::Completion completion, int timeoutMs)
{
    if (m_command.busy())
        return;
    m_output->clear();
    m_pendingOutput.clear();
    const bool listing = arguments.first() == "nodes" || arguments.first() == "pairings" || arguments.first() == "info";
    if (!listing) {
        m_status->show();
        m_status->setText(tr("Working…"));
    }
    QStringList args{"remote"};
    args.append(arguments);
    m_command.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), args,
                  [this, completion](bool ok, const QByteArray &output) {
        m_status->setText(ok ? QString() : tr("Could not complete this step. See details for the error; saved setup is preserved."));
        m_status->setVisible(!ok);
        if (completion)
            completion(ok, output);
        if (!m_status->text().isEmpty())
            m_status->show();
    }, timeoutMs);
}

void RemoteWidget::refreshNodes()
{
    run({"nodes"}, [this](bool ok, const QByteArray &data) {
        m_brokerAvailable = ok;
        m_nodes->setRowCount(0);
        if (!ok) {
            m_overviewHint->setText(tr("Couldn't load managed machines. Retry, or use Connect this computer to join a machine you've already set up."));
            m_review->hide();
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
        emit nodesChanged(paired);
        updateActions();
        refreshPairings();
    });
}

void RemoteWidget::refreshPairings()
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
    });
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
    run({approve ? "approve" : "deny", "--code", code}, [this, approve, name](bool ok, const QByteArray &) {
        if (ok) {
            m_requestOutcome = approve ? tr("%1 is paired. On that computer, choose Start worker service to receive workflows.").arg(name)
                                       : tr("Request from %1 denied.").arg(name);
            refresh();
        }
    });
}

void RemoteWidget::setResolvers(const QVector<ResolverMeta> &resolvers)
{
    const QString selected = m_resolver->currentData().toString();
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

void RemoteWidget::updateSetup()
{
    const bool idle = !m_command.busy();
    const int index = m_resolver->currentIndex();
    const bool available = index >= 0 && index < m_resolvers.size();
    const bool hosted = available && m_resolvers[index].remoteSetup == "hosted";
    m_resolver->setEnabled(idle && available);
    m_hostname->setVisible(available && !hosted);
    m_hostnameLabel->setVisible(available && !hosted);
    m_hostname->setEnabled(idle);
    m_setup->setEnabled(idle && available);
    m_setup->setText(hosted ? tr("Sign in and connect…") : tr("Set up remote access…"));
    if (!available) {
        m_setupHint->setText(tr("No remote-access resolvers found. Install one from Packages, then refresh your workspace. If one is already installed, update the CLI to load provider details."));
        return;
    }
    const auto &resolver = m_resolvers[index];
    const QString mode = hosted ? tr("Hosted broker — the provider runs the service. No hostname or local broker setup is needed.")
                                : tr("Broker on this computer — the provider connects it to your public hostname. Keep this computer available for remote work.");
    QStringList details{resolver.name + "\n" + mode};
    if (!resolver.description.isEmpty())
        details.append(resolver.description);
    details.append(tr("Setup opens a terminal for installation prompts and browser sign-in. Selecting a provider does not change your current connection."));
    m_setupHint->setText(details.join("\n\n"));
}

void RemoteWidget::refresh()
{
    run({"info"}, [this](bool ok, const QByteArray &data) {
        const auto document = QJsonDocument::fromJson(data);
        if (!ok || !document.isObject()) {
            m_connectionSummary->setText(tr("Connection details unavailable. Update the CLI if it does not support remote info."));
            m_pairingAddress->setText(tr("Connection details unavailable. Use the address from your remote setup."));
            refreshNodes();
            return;
        }
        const auto info = document.object();
        const auto broker = info.value("broker").toObject();
        const auto worker = info.value("worker").toObject();
        if (!broker.isEmpty()) {
            const auto resolver = broker.value("resolver").toObject();
            QString provider = resolver.value("title").toString();
            const QString name = resolver.value("name").toString();
            if (provider.isEmpty())
                provider = name;
            if (!name.isEmpty() && provider != name)
                provider += " (" + name + ")";
            const QString version = resolver.value("version").toString();
            if (!version.isEmpty())
                provider += " · " + version;
            if (provider.isEmpty())
                provider = tr("Resolver not recorded by this older setup");
            const QString mode = broker.value("mode").toString() == "hosted" ? tr("Hosted broker") : tr("Broker on this computer");
            const QString address = broker.value("endpoint").toString();
            m_connectionSummary->setText(provider + "\n" + mode + " · " + address);
            m_pairingAddress->setText(tr("Remote address: %1\nProvider: %2").arg(address, provider));
            refreshNodes();
            return;
        }
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
        emit nodesChanged({});
        updateActions();
    });
}
