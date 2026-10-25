#include "ActivityWidget.h"
#include "DockpipeChoices.h"
#include "LogViewerDialog.h"
#include "CanvasIcons.h"

#include <QDir>
#include <QFileDialog>
#include <QHeaderView>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QComboBox>
#include <QStackedWidget>
#include <QTimer>
#include <QTableWidget>
#include <QVBoxLayout>
#include <QUuid>

namespace {
QTableWidget *makeTable(const QStringList &headers)
{
    auto *view = new QTableWidget(0, headers.size());
    view->setHorizontalHeaderLabels(headers);
    view->horizontalHeader()->setSectionResizeMode(QHeaderView::Stretch);
    view->horizontalHeader()->setDefaultAlignment(Qt::AlignLeft | Qt::AlignVCenter);
    view->verticalHeader()->hide();
    view->verticalHeader()->setDefaultSectionSize(46);
    view->setShowGrid(false);
    view->setAlternatingRowColors(true);
    view->setEditTriggers(QAbstractItemView::NoEditTriggers);
    view->setSelectionBehavior(QAbstractItemView::SelectRows);
    view->setSelectionMode(QAbstractItemView::SingleSelection);
    return view;
}
}

ActivityWidget::ActivityWidget(SessionManager &sessions, ContextStore &store, QWidget *parent)
    : QWidget(parent), m_sessions(sessions), m_store(store), m_command(this)
{
    setObjectName("runHistoryPage");
    setAttribute(Qt::WA_StyledBackground, true);
    auto *root = new QVBoxLayout(this);
    root->setContentsMargins(28, 24, 28, 24);
    root->setSpacing(20);
    auto *title = new QLabel(tr("Runs & logs"));
    title->setObjectName(QStringLiteral("appTitle"));
    root->addWidget(title);
    auto *subtitle = new QLabel(tr("Run history, output and results — across your machines."));
    subtitle->setObjectName("appSubtitle");
    subtitle->setWordWrap(true);
    root->addWidget(subtitle);
    m_status = new QLabel;
    m_status->setObjectName("runStatus");
    m_status->setWordWrap(true);
    auto *toolbar = new QHBoxLayout;
    m_location = new QComboBox;
    m_location->setObjectName("runLocation");
    m_location->addItems({tr("This computer"), tr("Remote jobs")});
    m_location->setMinimumWidth(180);
    m_location->setAccessibleName(tr("Run location"));
    toolbar->addWidget(m_location);
    toolbar->addStretch();
    m_count = new QLabel;
    m_count->setObjectName("runCount");
    toolbar->addWidget(m_count);
    root->addLayout(toolbar);
    const auto emptyState = [this](bool local) {
        auto *page = new QWidget;
        page->setObjectName(local ? "localRunsEmpty" : "remoteJobsEmpty");
        auto *layout = new QVBoxLayout(page);
        layout->setContentsMargins(24, 32, 24, 32);
        layout->setSpacing(14);
        layout->addStretch();
        auto *icon = new QLabel;
        icon->setPixmap(CanvasIcons::icon(local ? CanvasIcons::Symbol::Terminal : CanvasIcons::Symbol::Monitor).pixmap(48, 48));
        layout->addWidget(icon, 0, Qt::AlignHCenter);
        auto *heading = new QLabel(local ? tr("Your runs will appear here") : tr("No remote jobs to show"));
        heading->setObjectName("runEmptyTitle");
        heading->setAlignment(Qt::AlignCenter);
        layout->addWidget(heading);
        auto *description = new QLabel(local
            ? tr("Launch an app or workflow from this window to follow its progress,\nopen its logs and manage the run.")
            : tr("Jobs sent to your paired machines appear here.\nRefresh to check for updates, or manage connections in Machines."));
        description->setObjectName("appSubtitle");
        description->setAlignment(Qt::AlignCenter);
        description->setWordWrap(true);
        layout->addWidget(description);
        if (local) {
            auto *browse = new QPushButton(tr("Browse workflows"));
            browse->setObjectName("browseRunWorkflows");
            browse->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Workflow));
            connect(browse, &QPushButton::clicked, this, &ActivityWidget::browseWorkflowsRequested);
            layout->addWidget(browse, 0, Qt::AlignHCenter);
        }
        layout->addStretch();
        return page;
    };
    auto *tabs = new QStackedWidget;
    tabs->setObjectName("runLocationTabs");
    auto *local = new QWidget;
    auto *localLayout = new QVBoxLayout(local);
    m_local = makeTable({tr("Workflow / app"), tr("Run on"), tr("Status")});
    m_local->setObjectName("localActivityRuns");
    localLayout->setContentsMargins(0, 0, 0, 0);
    m_localPages = new QStackedWidget;
    m_localPages->addWidget(emptyState(true));
    m_localPages->addWidget(m_local);
    localLayout->addWidget(m_localPages, 1);
    auto *localButtons = new QHBoxLayout;
    m_openLogs = new QPushButton(tr("Open logs"));
    m_stopRun = new QPushButton(tr("Stop selected run"));
    auto *logs = m_openLogs;
    auto *stop = m_stopRun;
    logs->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Terminal));
    localButtons->addWidget(logs);
    localButtons->addWidget(stop);
    localButtons->addStretch();
    localLayout->insertLayout(0, localButtons);
    connect(stop, &QPushButton::clicked, this, [this]() {
        if (m_local->currentRow() >= 0)
            m_sessions.stop(m_local->item(m_local->currentRow(), 0)->data(Qt::UserRole).toString());
    });
    connect(logs, &QPushButton::clicked, this, [this]() {
        if (m_local->currentRow() < 0)
            return;
        const QString id = m_local->item(m_local->currentRow(), 0)->data(Qt::UserRole).toString();
        auto *dialog = new LogViewerDialog(tr("Run logs"), m_sessions.info(id).logPath, QString(), m_sessions.isRunning(id), this);
        dialog->setAttribute(Qt::WA_DeleteOnClose);
        dialog->show();
    });
    tabs->addWidget(local);
    auto *remote = new QWidget;
    auto *remoteLayout = new QVBoxLayout(remote);
    m_jobs = makeTable({tr("Job"), tr("Machine"), tr("State")});
    remoteLayout->setContentsMargins(0, 0, 0, 0);
    m_jobs->setObjectName("remoteJobs");
    m_remotePages = new QStackedWidget;
    m_remotePages->addWidget(emptyState(false));
    m_remotePages->addWidget(m_jobs);
    remoteLayout->addWidget(m_remotePages, 1);
    auto *remoteButtons = new QHBoxLayout;
    auto *refresh = new QPushButton(tr("Refresh jobs"));
    m_cancelJob = new QPushButton(tr("Cancel selected job"));
    m_downloadResult = new QPushButton(tr("Download results"));
    m_downloadResult->setToolTip(tr("Save result artifacts and workflow logs."));
    auto *cancel = m_cancelJob;
    auto *download = m_downloadResult;
    cancel->setEnabled(false);
    download->setEnabled(false);
    remoteButtons->addWidget(refresh);
    remoteButtons->addStretch();
    remoteButtons->addWidget(cancel);
    remoteButtons->addWidget(download);
    remoteLayout->insertLayout(0, remoteButtons);
    tabs->addWidget(remote);
    connect(m_location, qOverload<int>(&QComboBox::currentIndexChanged), tabs, &QStackedWidget::setCurrentIndex);
    connect(m_location, qOverload<int>(&QComboBox::currentIndexChanged), this, [this](int index) {
        m_output->hide();
        m_status->clear();
        updateRunControls();
        if (index == 1)
            refreshRemote();
        else
            refreshLocal();
    });
    root->addWidget(tabs, 1);
    auto *refreshTimer = new QTimer(this);
    refreshTimer->setInterval(5000);
    connect(refreshTimer, &QTimer::timeout, this, [this, tabs]() {
        if (isVisible() && tabs->currentIndex() == 1 && m_remoteLoaded && !m_command.busy())
            refreshRemote();
    });
    refreshTimer->start();
    m_output = new QPlainTextEdit;
    m_output->setObjectName("detailConsole");
    m_output->setReadOnly(true);
    m_output->setMaximumBlockCount(1000);
    m_output->setMaximumHeight(160);
    m_output->hide();
    root->addWidget(m_output);
    root->addWidget(m_status);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        m_output->appendPlainText(text.trimmed());
    });
    connect(&m_command, &RemoteCommand::busyChanged, this, [this, refresh](bool busy) {
        refresh->setEnabled(!busy);
        updateRunControls();
    });
    connect(refresh, &QPushButton::clicked, this, &ActivityWidget::refreshRemote);
    connect(cancel, &QPushButton::clicked, this, [this]() {
        if (!selectedJob().isEmpty()) {
            run({"cancel", "--id", selectedJob()}, [this](bool ok, const QByteArray &) {
                if (ok)
                    refreshRemote();
            });
        }
    });
    connect(download, &QPushButton::clicked, this, [this]() {
        const QString job = selectedJob();
        if (job.isEmpty())
            return;
        const QString parent = QFileDialog::getExistingDirectory(this, tr("Save result inside folder"));
        if (parent.isEmpty())
            return;
        const QString destination = QDir(parent).filePath(QStringLiteral("dockpipe-result-") + QUuid::createUuid().toString(QUuid::WithoutBraces));
        run({"result", "--id", job, "--out", destination}, [this, destination](bool ok, const QByteArray &) {
            if (ok)
                m_status->setText(tr("Result and workflow.log saved to %1").arg(destination));
        });
    });
    connect(m_local, &QTableWidget::itemSelectionChanged, this, &ActivityWidget::updateRunControls);
    connect(m_jobs, &QTableWidget::itemSelectionChanged, this, &ActivityWidget::updateRunControls);
    updateRunControls();
    connect(&sessions, &SessionManager::sessionStarted, this, &ActivityWidget::refreshLocal);
    connect(&sessions, &SessionManager::sessionStopped, this, &ActivityWidget::refreshLocal);
    connect(&sessions, &SessionManager::sessionFailed, this, &ActivityWidget::refreshLocal);
}

void ActivityWidget::setWorkspaceFilter(bool enabled)
{
    m_workspaceOnly = enabled;
    findChild<QLabel *>("appTitle")->setVisible(!enabled);
    findChild<QLabel *>("appSubtitle")->setVisible(!enabled);
    findChild<QPushButton *>("browseRunWorkflows")->setVisible(!enabled);
    if (enabled)
        layout()->setContentsMargins(8, 8, 8, 8);
    else
        layout()->setContentsMargins(28, 24, 28, 24);
    m_output->hide();
}

void ActivityWidget::setWorkdir(const QString &workdir)
{
    m_workdir = workdir;
}

void ActivityWidget::showRemoteJobs()
{
    if (m_location->currentIndex() == 1)
        refreshRemote();
    else
        m_location->setCurrentIndex(1);
}

void ActivityWidget::refreshLocal()
{
    QString selected;
    if (m_local->currentRow() >= 0)
        selected = m_local->item(m_local->currentRow(), 0)->data(Qt::UserRole).toString();
    m_local->blockSignals(true);
    m_local->setRowCount(0);
    for (const auto &context : m_store.contexts) {
        if (m_workspaceOnly && QDir::cleanPath(context.workdir) != QDir::cleanPath(m_workdir))
            continue;
        const auto info = m_sessions.info(context.id);
        if (info.logPath.isEmpty())
            continue;
        const int row = m_local->rowCount();
        m_local->insertRow(row);
        auto *name = new QTableWidgetItem(context.label);
        name->setData(Qt::UserRole, context.id);
        m_local->setItem(row, 0, name);
        m_local->setItem(row, 1, new QTableWidgetItem(tr("This computer")));
        QString status = tr("Finished / stopped");
        if (m_sessions.isRunning(context.id))
            status = tr("Running");
        else if (info.status == SessionStatus::Failed)
            status = tr("Failed");
        m_local->setItem(row, 2, new QTableWidgetItem(status));
        if (context.id == selected)
            m_local->selectRow(row);
    }
    m_local->blockSignals(false);
    updateRunControls();
}

void ActivityWidget::updateRunControls()
{
    const int row = m_local->currentRow();
    auto *item = row >= 0 ? m_local->item(row, 0) : nullptr;
    m_openLogs->setEnabled(item != nullptr);
    m_stopRun->setEnabled(item && m_sessions.isRunning(item->data(Qt::UserRole).toString()));
    const bool jobSelected = m_remoteLoaded && !m_command.busy() && !selectedJob().isEmpty();
    m_cancelJob->setEnabled(jobSelected);
    m_downloadResult->setEnabled(jobSelected);
    m_localPages->setCurrentIndex(m_local->rowCount() == 0 ? 0 : 1);
    m_remotePages->setCurrentIndex(m_jobs->rowCount() == 0 ? 0 : 1);
    if (m_location->currentIndex() == 0) {
        const int count = m_local->rowCount();
        m_count->setText(count == 1 ? tr("1 local run") : tr("%1 local runs").arg(count));
    } else {
        const int count = m_jobs->rowCount();
        m_count->setText(count == 1 ? tr("1 remote job") : tr("%1 remote jobs").arg(count));
    }
}

QString ActivityWidget::selectedJob() const
{
    const int row = m_jobs->currentRow();
    auto *item = row >= 0 ? m_jobs->item(row, 0) : nullptr;
    return item ? item->text() : QString();
}

void ActivityWidget::run(const QStringList &args, RemoteCommand::Completion completion)
{
    if (m_command.busy())
        return;
    m_output->clear();
    m_output->hide();
    QStringList command{"remote"};
    command.append(args);
    m_command.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), command,
                  [this, completion](bool ok, const QByteArray &data) {
        m_status->setText(ok ? tr("Up to date with the broker.") : tr("Could not complete the operation. See details below."));
        m_output->setVisible(!ok && !m_output->toPlainText().isEmpty());
        completion(ok, data);
    });
}

void ActivityWidget::refreshRemote()
{
    if (m_command.busy())
        return;
    m_remoteLoaded = false;
    run({"info"}, [this](bool ok, const QByteArray &data) {
        if (!ok)
            return;
        QJsonParseError error;
        const auto document = QJsonDocument::fromJson(data, &error);
        if (error.error != QJsonParseError::NoError || !document.isObject()) {
            m_status->setText(tr("Could not read remote connection information. Refresh to try again."));
            return;
        }
        const auto info = document.object();
        if (info.value("broker").toObject().isEmpty()) {
            m_jobs->setRowCount(0);
            updateRunControls();
            m_status->setText(info.value("worker").toObject().isEmpty()
                ? tr("No broker is managed from this computer. Set up remote access in Machines to manage jobs.")
                : tr("This computer is paired to receive workflows. Manage remote jobs from the computer that approved pairing. If worker setup was interrupted, use Start worker service in Machines to finish it."));
            return;
        }
        loadRemoteJobs();
    });
}

void ActivityWidget::loadRemoteJobs()
{
    run({"jobs"}, [this](bool ok, const QByteArray &data) {
        m_remoteLoaded = ok;
        updateRunControls();
        if (!ok)
            return;
        const QString selected = selectedJob();
        const auto jobs = QJsonDocument::fromJson(data).array();
        m_jobs->blockSignals(true);
        m_jobs->setRowCount(0);
        for (const auto &value : jobs) {
            const auto job = value.toObject();
            const int row = m_jobs->rowCount();
            m_jobs->insertRow(row);
            m_jobs->setItem(row, 0, new QTableWidgetItem(job.value("id").toString()));
            m_jobs->setItem(row, 1, new QTableWidgetItem(job.value("node").toString()));
            m_jobs->setItem(row, 2, new QTableWidgetItem(job.value("status").toString()));
            if (job.value("id").toString() == selected)
                m_jobs->selectRow(row);
        }
        m_jobs->blockSignals(false);
        updateRunControls();
    });
}
