#include "DependencyChangeDialog.h"
#include "DockpipeChoices.h"

#include <QFile>
#include <QFileDialog>
#include <QFileInfo>
#include <QGridLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QRegularExpression>
#include <QSaveFile>
#include <QVBoxLayout>

DependencyChangeDialog::DependencyChangeDialog(const QString &workdir, QWidget *parent)
    : QDialog(parent), m_workdir(workdir)
{
    setWindowTitle(tr("Dependency update and recovery"));
    resize(760, 520);
    auto *layout = new QVBoxLayout(this);
    auto *hint = new QLabel(tr("Choose a local update request prepared for your installed resolver. Review it before applying or recovering. Use only trusted requests and resolvers. Install/recover leave processes running. Activation uses the reviewed maintenance policy to replace the connector; the broker owns health checks and bounded recovery even if this dialog closes."), this);
    hint->setWordWrap(true);
    layout->addWidget(hint);
    m_requestLabel = new QLabel(tr("No request selected"), this);
    m_requestLabel->setTextFormat(Qt::PlainText);
    m_requestLabel->setWordWrap(true);
    layout->addWidget(m_requestLabel);
    m_choose = new QPushButton(tr("Choose request…"), this);
    layout->addWidget(m_choose);
    m_output = new QPlainTextEdit(this);
    m_output->setReadOnly(true);
    layout->addWidget(m_output);
    auto *actions = new QGridLayout;
    m_reviewUpdate = new QPushButton(tr("Review update"), this);
    m_reviewRecovery = new QPushButton(tr("Review recovery"), this);
    m_apply = new QPushButton(tr("Apply reviewed update"), this);
    m_recover = new QPushButton(tr("Recover reviewed update"), this);
    m_reviewActivation = new QPushButton(tr("Review activation"), this);
    m_activate = new QPushButton(tr("Activate reviewed connector"), this);
    m_health = new QPushButton(tr("Refresh activation health"), this);
    actions->addWidget(m_reviewUpdate, 0, 0);
    actions->addWidget(m_apply, 0, 1);
    actions->addWidget(m_reviewRecovery, 1, 0);
    actions->addWidget(m_recover, 1, 1);
    actions->addWidget(m_reviewActivation, 2, 0);
    actions->addWidget(m_activate, 2, 1);
    actions->addWidget(m_health, 3, 0, 1, 2);
    m_reviewActivation->setObjectName(QStringLiteral("dependencyReviewActivation"));
    m_activate->setObjectName(QStringLiteral("dependencyActivate"));
    m_health->setObjectName(QStringLiteral("dependencyActivationHealth"));
    m_apply->setObjectName(QStringLiteral("dependencyApply"));
    m_recover->setObjectName(QStringLiteral("dependencyRecover"));
    m_reviewUpdate->setObjectName(QStringLiteral("dependencyReviewUpdate"));
    m_reviewRecovery->setObjectName(QStringLiteral("dependencyReviewRecovery"));
    layout->addLayout(actions);
    auto *close = new QPushButton(tr("Close"), this);
    layout->addWidget(close);
    connect(close, &QPushButton::clicked, this, &QDialog::reject);
    connect(m_choose, &QPushButton::clicked, this, &DependencyChangeDialog::chooseRequest);
    connect(m_reviewUpdate, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("update"), true); });
    connect(m_reviewRecovery, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("recover"), true); });
    connect(m_apply, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("update"), false); });
    connect(m_recover, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("recover"), false); });
    connect(m_reviewActivation, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("activate"), true); });
    connect(m_activate, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("activate"), false); });
    connect(m_health, &QPushButton::clicked, this, [this]() { execute(QStringLiteral("activation-status"), false); });
    connect(&m_process, &QProcess::readyReadStandardOutput, this, &DependencyChangeDialog::readOutput);
    connect(&m_process, &QProcess::readyReadStandardError, this, &DependencyChangeDialog::readOutput);
    connect(&m_process, qOverload<int, QProcess::ExitStatus>(&QProcess::finished), this,
        [this](int code, QProcess::ExitStatus status) { finish(code == 0 && status == QProcess::NormalExit); });
    connect(&m_process, &QProcess::errorOccurred, this, [this](QProcess::ProcessError error) {
        if (error == QProcess::FailedToStart)
            finish(false);
    });
    m_timeout.setSingleShot(true);
    connect(&m_timeout, &QTimer::timeout, this, [this]() {
        m_failed = true;
        m_process.kill();
    });
    setBusy(false);
}

void DependencyChangeDialog::chooseRequest()
{
    const QString path = QFileDialog::getOpenFileName(this, tr("Choose dependency change request"), QString(), tr("JSON files (*.json)"));
    if (path.isEmpty())
        return;
    loadRequest(path);
}

void DependencyChangeDialog::loadRequest(const QString &path)
{
    if (m_process.state() != QProcess::NotRunning)
        return;
    m_applyApproval.clear();
    m_recoveryApproval.clear();
    m_activateApproval.clear();
    m_request.clear();
    QFile file(path);
    const QFileInfo info(path);
    if (info.isSymLink() || !info.isFile() || info.size() > 65536 || !file.open(QIODevice::ReadOnly)) {
        m_output->setPlainText(tr("Choose a readable regular request file no larger than 64 KiB."));
        setBusy(false);
        return;
    }
    const QByteArray data = file.read(65537);
    if (data.size() > 65536) {
        m_output->setPlainText(tr("Request exceeds 64 KiB."));
        setBusy(false);
        return;
    }
    if (!m_requestDirectory.isValid()) {
        m_output->setPlainText(tr("A private request snapshot could not be created."));
        setBusy(false);
        return;
    }
    const QString snapshotPath = m_requestDirectory.filePath(QStringLiteral("request.json"));
    QSaveFile snapshot(snapshotPath);
    if (!snapshot.open(QIODevice::WriteOnly)
        || !snapshot.setPermissions(QFileDevice::ReadOwner | QFileDevice::WriteOwner)
        || snapshot.write(data) != data.size() || !snapshot.commit()) {
        m_output->setPlainText(tr("The private request snapshot could not be saved."));
        setBusy(false);
        return;
    }
    m_request = snapshotPath;
    m_requestPreview = data;
    m_requestLabel->setText(info.absoluteFilePath());
    m_output->setPlainText(QString::fromUtf8(data));
    setBusy(false);
}

void DependencyChangeDialog::execute(const QString &operation, bool dryRun)
{
    if (m_process.state() != QProcess::NotRunning || m_request.isEmpty())
        return;
    QString approval;
    if (operation == QStringLiteral("recover"))
        approval = m_recoveryApproval;
    else if (operation == QStringLiteral("activate"))
        approval = m_activateApproval;
    else
        approval = m_applyApproval;
    if (!dryRun && operation != QStringLiteral("activation-status") && approval.isEmpty())
        return;
    m_applyApproval.clear();
    m_recoveryApproval.clear();
    m_activateApproval.clear();
    m_stdout.clear();
    m_stderr.clear();
    m_failed = false;
    m_reviewing = dryRun;
    m_operation = operation;
    QStringList arguments{QStringLiteral("dependency"), operation,
        QStringLiteral("--request"), m_request, QStringLiteral("--workdir"), m_workdir};
    if (dryRun)
        arguments << QStringLiteral("--dry-run");
    else if (operation != QStringLiteral("activation-status"))
        arguments << QStringLiteral("--approve") << approval;
    setBusy(true);
    m_output->setPlainText(tr("Running local Dockpipe validation…"));
    m_process.setWorkingDirectory(m_workdir);
    m_process.setProgram(DockpipeChoices::operatorDockpipeBinary());
    m_process.setArguments(arguments);
    m_timeout.start(135000);
    m_process.start();
}

void DependencyChangeDialog::readOutput()
{
    m_stdout += m_process.readAllStandardOutput();
    m_stderr += m_process.readAllStandardError();
    if (m_stdout.size() + m_stderr.size() > 131072) {
        m_failed = true;
        m_stdout = m_stdout.left(65536);
        m_stderr = m_stderr.left(65536);
        m_process.kill();
    }
}

void DependencyChangeDialog::finish(bool success)
{
    m_timeout.stop();
    readOutput();
    const auto document = QJsonDocument::fromJson(m_stdout);
    const auto envelope = document.object();
    const auto result = envelope.value(QStringLiteral("result")).toObject();
    const QString status = result.value(QStringLiteral("status")).toString();
    const QString message = result.value(QStringLiteral("message")).toString();
    if (success && !m_failed && result.value(QStringLiteral("schema")).toString() == QStringLiteral("dockpipe.dependency.change/v1") && m_reviewing && status == QStringLiteral("ready")) {
        QString approvalField = QStringLiteral("apply_approval");
        if (m_operation == QStringLiteral("recover"))
            approvalField = QStringLiteral("recovery_approval");
        else if (m_operation == QStringLiteral("activate"))
            approvalField = QStringLiteral("activate_approval");
        const QString approval = envelope.value(approvalField).toString();
        static const QRegularExpression digest(QStringLiteral("^[0-9a-f]{64}$"));
        if (digest.match(approval).hasMatch()) {
            if (m_operation == QStringLiteral("recover"))
                m_recoveryApproval = approval;
            else if (m_operation == QStringLiteral("activate"))
                m_activateApproval = approval;
            else
                m_applyApproval = approval;
        }
    }
    if (!message.isEmpty()) {
        m_output->setPlainText(status + QStringLiteral("\n\n") + message
            + (m_reviewing ? tr("\n\nExact request reviewed:\n") + QString::fromUtf8(m_requestPreview) : QString()));
    } else {
        m_output->setPlainText(tr("The operation did not return a validated outcome. Inspect the provider journal before recovery; do not retry automatically.\n\n") + QString::fromUtf8(m_stderr));
    }
    if (!success || m_failed)
        m_output->appendPlainText(tr("\nThe operation did not complete successfully. Preserve and inspect the provider journal before another change."));
    setBusy(false);
}

void DependencyChangeDialog::setBusy(bool busy)
{
    m_choose->setEnabled(!busy);
    m_reviewUpdate->setEnabled(!busy && !m_request.isEmpty());
    m_reviewRecovery->setEnabled(!busy && !m_request.isEmpty());
    m_apply->setEnabled(!busy && !m_applyApproval.isEmpty());
    m_recover->setEnabled(!busy && !m_recoveryApproval.isEmpty());
    m_reviewActivation->setEnabled(!busy && !m_request.isEmpty());
    m_activate->setEnabled(!busy && !m_activateApproval.isEmpty());
    m_health->setEnabled(!busy && !m_request.isEmpty());
}

void DependencyChangeDialog::reject()
{
    if (m_process.state() != QProcess::NotRunning) {
        m_output->appendPlainText(tr("Wait for the bounded operation to finish; its recovery journal must be preserved."));
        return;
    }
    QDialog::reject();
}
