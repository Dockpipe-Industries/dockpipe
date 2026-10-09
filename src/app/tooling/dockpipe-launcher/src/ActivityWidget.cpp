#include "ActivityWidget.h"
#include "DockpipeChoices.h"
#include "LogViewerDialog.h"

#include <QDir>
#include <QFileDialog>
#include <QHeaderView>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QTabWidget>
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
    view->verticalHeader()->hide();
    view->setEditTriggers(QAbstractItemView::NoEditTriggers);
    view->setSelectionBehavior(QAbstractItemView::SelectRows);
    view->setSelectionMode(QAbstractItemView::SingleSelection);
    return view;
}
}

ActivityWidget::ActivityWidget(SessionManager &sessions, ContextStore &store, QWidget *parent)
    : QWidget(parent), m_sessions(sessions), m_store(store), m_command(this)
{
    auto *root = new QVBoxLayout(this);
    root->setContentsMargins(28, 24, 28, 24);
    root->setSpacing(14);
    auto *title = new QLabel(tr("Activity"));
    title->setObjectName(QStringLiteral("appTitle"));
    root->addWidget(title);
    m_status = new QLabel(tr("Local sessions and remote jobs. Refresh to retrieve the broker's latest state."));
    m_status->setWordWrap(true);
    root->addWidget(m_status);
    auto *tabs = new QTabWidget;
    auto *local = new QWidget;
    auto *localLayout = new QVBoxLayout(local);
    m_local = makeTable({tr("Workflow / app"), tr("Run on"), tr("Status")});
    localLayout->addWidget(m_local);
    auto *localButtons = new QHBoxLayout;
    auto *logs = new QPushButton(tr("Open logs"));
    auto *stop = new QPushButton(tr("Stop selected run"));
    localButtons->addWidget(logs);
    localButtons->addWidget(stop);
    localButtons->addStretch();
    localLayout->addLayout(localButtons);
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
    tabs->addTab(local, tr("This computer"));
    auto *remote = new QWidget;
    auto *remoteLayout = new QVBoxLayout(remote);
    m_jobs = makeTable({tr("Job"), tr("Machine"), tr("State")});
    remoteLayout->addWidget(m_jobs);
    auto *remoteButtons = new QHBoxLayout;
    auto *refresh = new QPushButton(tr("Refresh jobs"));
    auto *cancel = new QPushButton(tr("Cancel selected job"));
    auto *download = new QPushButton(tr("Download result & logs"));
    remoteButtons->addWidget(refresh);
    remoteButtons->addWidget(cancel);
    remoteButtons->addWidget(download);
    remoteLayout->addLayout(remoteButtons);
    tabs->addTab(remote, tr("Remote"));
    root->addWidget(tabs, 1);
    auto *refreshTimer = new QTimer(this);
    refreshTimer->setInterval(5000);
    connect(refreshTimer, &QTimer::timeout, this, [this, tabs]() {
        if (isVisible() && tabs->currentIndex() == 1 && m_remoteLoaded && !m_command.busy())
            refreshRemote();
    });
    refreshTimer->start();
    m_output = new QPlainTextEdit;
    m_output->setReadOnly(true);
    m_output->setMaximumBlockCount(1000);
    m_output->setMaximumHeight(160);
    root->addWidget(m_output);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        m_output->appendPlainText(text.trimmed());
    });
    connect(&m_command, &RemoteCommand::busyChanged, this, [refresh, cancel, download](bool busy) {
        refresh->setEnabled(!busy);
        cancel->setEnabled(!busy);
        download->setEnabled(!busy);
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
    connect(&sessions, &SessionManager::sessionStarted, this, &ActivityWidget::refreshLocal);
    connect(&sessions, &SessionManager::sessionStopped, this, &ActivityWidget::refreshLocal);
    connect(&sessions, &SessionManager::sessionFailed, this, &ActivityWidget::refreshLocal);
}

void ActivityWidget::setWorkdir(const QString &workdir)
{
    m_workdir = workdir;
}

void ActivityWidget::refreshLocal()
{
    m_local->setRowCount(0);
    for (const auto &context : m_store.contexts) {
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
    }
}

QString ActivityWidget::selectedJob() const
{
    return m_jobs->currentRow() < 0 ? QString() : m_jobs->item(m_jobs->currentRow(), 0)->text();
}

void ActivityWidget::run(const QStringList &args, RemoteCommand::Completion completion)
{
    if (m_command.busy())
        return;
    m_output->clear();
    QStringList command{"remote"};
    command.append(args);
    m_command.run(DockpipeChoices::preferredDockpipeBinary(m_workdir), command,
                  [this, completion](bool ok, const QByteArray &data) {
        m_status->setText(ok ? tr("Updated from broker.") : tr("Could not complete the operation. See details below."));
        completion(ok, data);
    });
}

void ActivityWidget::refreshRemote()
{
    run({"jobs"}, [this](bool ok, const QByteArray &data) {
        m_remoteLoaded = ok;
        if (!ok)
            return;
        const QString selected = selectedJob();
        const auto jobs = QJsonDocument::fromJson(data).array();
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
    });
}
