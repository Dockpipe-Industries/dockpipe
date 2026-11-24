#include "RemoteFeatureRecoveryDialog.h"
#include "RemoteFeatureClient.h"

#include <QComboBox>
#include <QDialog>
#include <QHBoxLayout>
#include <QLabel>
#include <QMessageBox>
#include <QPushButton>
#include <QRegularExpression>
#include <QSignalBlocker>
#include <QVBoxLayout>

namespace {
QString text(const char *source)
{
    return QCoreApplication::translate("RemoteFeatureRecoveryDialog", source);
}

QJsonObject verifiedHostRecovery(const QJsonObject &operation)
{
    const auto request = operation.value("request").toObject();
    const auto receipt = operation.value("host_recovery").toObject();
    const auto original = receipt.value("reference").toObject();
    const auto originalTarget = original.value("target").toObject();
    const auto executionTarget = request.value("target").toObject();
    static const QRegularExpression digest(QStringLiteral("\\A[0-9a-f]{64}\\z"));
    if (operation.value("status") != QStringLiteral("success")
        || request.value("action") != QStringLiteral("terminal.host-recover")
        || receipt.value("schema") != QStringLiteral("dockpipe.host-terminal-receipt/v1")
        || !receipt.value("reconciled").toBool()
        || receipt.value("root").toString().isEmpty() || receipt.value("root") != request.value("root")
        || original.value("schema") != QStringLiteral("dockpipe.remote.terminal/v1")
        || !digest.match(original.value("id").toString()).hasMatch()
        || !digest.match(original.value("epoch").toString()).hasMatch()
        || !digest.match(originalTarget.value("generation").toString()).hasMatch()
        || originalTarget.value("node") != executionTarget.value("node")
        || originalTarget.value("fingerprint") != executionTarget.value("fingerprint"))
        return {};
    return receipt;
}

class RecoveryDialog final : public QDialog {
public:
    explicit RecoveryDialog(QWidget *parent, RemoteFeatureRecoveryDialog::VerifiedRecovery verifiedRecovery)
        : QDialog(parent), m_verifiedRecovery(std::move(verifiedRecovery))
    {
        setAttribute(Qt::WA_DeleteOnClose);
        setWindowTitle(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Uncertain remote operations")));
        resize(680, 360);
        auto *layout = new QVBoxLayout(this);
        auto *introduction = new QLabel(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Inspect the original operation before another write. Cancellation does not prove that a mutation had no effect.")), this);
        introduction->setWordWrap(true);
        layout->addWidget(introduction);
        m_operations = new QComboBox(this);
        m_operations->setObjectName(QStringLiteral("remoteRecoveryOperations"));
        layout->addWidget(m_operations);
        m_identity = new QLabel(this);
        m_identity->setTextFormat(Qt::PlainText);
        m_identity->setTextInteractionFlags(Qt::TextSelectableByMouse);
        m_identity->setWordWrap(true);
        layout->addWidget(m_identity);
        m_status = new QLabel(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Choose a retained operation.")), this);
        m_status->setObjectName(QStringLiteral("remoteRecoveryStatus"));
        m_status->setTextFormat(Qt::PlainText);
        m_status->setWordWrap(true);
        layout->addWidget(m_status);
        layout->addStretch();
        auto *actions = new QHBoxLayout;
        m_inspect = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Refresh status")), this);
        m_inspect->setObjectName(QStringLiteral("remoteRecoveryRefreshStatus"));
        m_cancel = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Request cancellation")), this);
        m_reconcile = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Verify cleanup")), this);
        m_reconcile->setObjectName(QStringLiteral("remoteRecoveryVerifyCleanup"));
        m_reconcile->setToolTip(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Revoke the original invocation and verify its native cleanup evidence. Boot-fenced workers require a host reboot for uncertain process-backed operations. The mutation outcome remains unknown.")));
        m_retire = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Retire container…")), this);
        m_retire->setObjectName(QStringLiteral("remoteRecoveryRetireContainer"));
        m_retire->setToolTip(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Explicitly remove the original exact container after native client cleanup. Requires the worker's independent Docker recovery grant. Volumes and images are retained.")));
        actions->addWidget(m_inspect);
        actions->addWidget(m_cancel);
        actions->addWidget(m_reconcile);
        actions->addWidget(m_retire);
        layout->addLayout(actions);
        auto *close = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Close")), this);
        layout->addWidget(close, 0, Qt::AlignRight);
        connect(close, &QPushButton::clicked, this, &QDialog::close);
        connect(m_operations, &QComboBox::currentIndexChanged, this, [this] {
            m_actionFeedback.clear();
            updateSelection();
        });
        connect(m_inspect, &QPushButton::clicked, this, [this] { run(QStringLiteral("status")); });
        connect(m_cancel, &QPushButton::clicked, this, [this] { run(QStringLiteral("cancel")); });
        connect(m_reconcile, &QPushButton::clicked, this, [this] { run(QStringLiteral("reconcile")); });
        connect(m_retire, &QPushButton::clicked, this, [this] {
            const auto reference = m_operations->currentData().toJsonObject();
            const auto resource = reference.contains("terminal_reference")
                ? reference.value("container").toString() : reference.value("recovery_resource").toString();
            const auto node = reference.value("target").toObject().value("node").toString();
            const auto answer = QMessageBox::question(this,
                text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Retire the captured container?")),
                text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Remove container %1 on %2 to fence delayed daemon work for operation %3? This removes the container and its writable layer. Volumes and images are retained. The original mutation outcome remains unknown."))
                    .arg(resource, node, reference.value("id").toString()),
                QMessageBox::Yes | QMessageBox::No, QMessageBox::No);
            if (answer == QMessageBox::Yes && m_operations->currentData().toJsonObject() == reference)
                run(QStringLiteral("retire"));
        });
        reload();
        refreshCatalogs();
    }

    void refresh()
    {
        if (!m_busy) {
            reload();
            refreshCatalogs();
        }
    }

private:
    void reload()
    {
        const auto original = m_operations->currentData().toJsonObject().value("id").toString();
        const QSignalBlocker blockSelection(m_operations);
        m_operations->clear();
        int retainedIndex = -1;
        for (const auto &entry : RemoteFeatureClient::pendingRecoveries()) {
            const auto reference = entry.toObject();
            const auto target = reference.value("target").toObject();
            const QString label = target.value("node").toString() + QStringLiteral(" · ")
                + reference.value("action").toString() + QStringLiteral(" · ")
                + reference.value("id").toString().left(12);
            m_operations->addItem(label, reference);
            if (reference.value("id").toString() == original)
                retainedIndex = m_operations->count() - 1;
        }
        for (const auto &entry : m_terminalRecoveries) {
            const auto reference = entry.toObject();
            const QString kind = reference.value("action").toString() == QStringLiteral("terminal.host-recover")
                ? QStringLiteral("terminal.host") : QStringLiteral("containers.exec");
            const QString label = reference.value("target").toObject().value("node").toString()
                + QStringLiteral(" · ") + kind + QStringLiteral(" · ")
                + reference.value("id").toString().left(12);
            m_operations->addItem(label, reference);
            if (reference.value("id").toString() == original)
                retainedIndex = m_operations->count() - 1;
        }
        // A completed operation can disappear while another one remains.
        // Leave the new operation unselected until the operator chooses it.
        if (!original.isEmpty() || !m_actionFeedback.isEmpty())
            m_operations->setCurrentIndex(retainedIndex);
        updateSelection();
    }

    void refreshCatalogs()
    {
        if (m_catalogBusy || m_busy)
            return;
        m_catalogBusy = true;
        RemoteFeatureClient::catalog(this, [this](const RemoteFeatureClient::Result &result) {
            m_catalogBusy = false;
            if (result.outcome != RemoteFeatureClient::Outcome::Success || !result.value.isArray())
                return;
            QJsonArray terminals;
            for (const auto &catalogValue : result.value.toArray()) {
                const auto catalog = catalogValue.toObject();
                for (const auto &entry : catalog.value("host_terminals").toArray()) {
                    const auto terminal = entry.toObject();
                    const auto original = terminal.value("reference").toObject();
                    const auto root = terminal.value("root").toString();
                    if (original.value("id").toString().size() != 64 || root.isEmpty())
                        continue;
                    bool allowed = false;
                    for (const auto &grantValue : catalog.value("roots").toArray()) {
                        const auto grant = grantValue.toObject();
                        if (grant.value("id").toString() == root)
                            allowed = grant.value("terminal").toBool();
                    }
                    terminals.append(QJsonObject{{"id", original.value("id")}, {"target", original.value("target")},
                        {"terminal_reference", original}, {"execution_target", catalog.value("target")},
                        {"root", root}, {"action", "terminal.host-recover"},
                        {"verify_allowed", allowed}, {"retire_allowed", false}});
                }
                for (const auto &entry : catalog.value("container_terminals").toArray()) {
                    const auto terminal = entry.toObject();
                    const auto original = terminal.value("reference").toObject();
                    const auto container = terminal.value("container").toString();
                    if (original.value("id").toString().size() != 64 || container.size() != 64)
                        continue;
                    terminals.append(QJsonObject{{"id", original.value("id")}, {"target", original.value("target")},
                        {"terminal_reference", original}, {"execution_target", catalog.value("target")},
                        {"container", container}, {"action", "containers.terminal-recover"},
                        {"verify_allowed", catalog.value("docker_exec").toBool() && catalog.value("docker_read").toBool()},
                        {"retire_allowed", catalog.value("docker_recovery").toBool() && catalog.value("docker_read").toBool()}});
                }
            }
            m_terminalRecoveries = terminals;
            if (!m_busy)
                reload();
        });
    }

    void updateSelection()
    {
        const auto reference = m_operations->currentData().toJsonObject();
        const auto target = reference.value("target").toObject();
        m_identity->setText(reference.isEmpty() ? text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "No uncertain mutations are retained."))
            : text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Machine: %1\nFingerprint: %2\nGeneration: %3\nRoot: %4\nAction: %5\nOperation: %6"))
                .arg(target.value("node").toString(), target.value("fingerprint").toString(),
                    target.value("generation").toString(), reference.value("root").toString(),
                    reference.value("action").toString(), reference.value("id").toString()));
        const bool available = !reference.isEmpty() && !m_busy;
        const bool terminal = reference.contains("terminal_reference");
        m_operations->setEnabled(!m_busy);
        m_inspect->setEnabled(available);
        m_cancel->setEnabled(available && !terminal && !reference.value("reconciled").toBool());
        m_reconcile->setEnabled(available && (!terminal || reference.value("verify_allowed").toBool()) && !reference.value("reconciled").toBool());
        m_retire->setEnabled(available && reference.value("action").toString().startsWith("containers.")
            && (terminal ? reference.value("retire_allowed").toBool() && reference.value("container").toString().size() == 64
                         : reference.value("recovery_resource").toString().size() == 64)
            && !reference.value("reconciled").toBool());
        if (!m_actionFeedback.isEmpty()) {
            m_status->setText(m_actionFeedback);
        } else if (reference.value("reconciled").toBool()) {
            m_status->setText(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Cleanup verified; the mutation outcome remains unknown. Refresh Files, Git or the container provider and review the affected state before another write.")));
        } else if (!m_busy) {
            m_status->setText(reference.isEmpty() ? text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "No uncertain mutations are retained."))
                : text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "The mutation outcome remains uncertain. Refresh its original status before another write.")));
        }
    }

    void showActionFeedback(const QString &feedback)
    {
        m_actionFeedback = feedback;
        m_status->setText(feedback);
    }

    void run(const QString &action)
    {
        const auto reference = m_operations->currentData().toJsonObject();
        if (reference.isEmpty() || m_busy)
            return;
        if (reference.contains("terminal_reference") && action == "status") {
            refreshCatalogs();
            showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Refreshing the worker's retained terminal metadata. No cleanup was requested.")));
            return;
        }
        m_busy = true;
        updateSelection();
        showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Contacting the original captured machine…")));
        if (reference.contains("terminal_reference")) {
            const auto original = reference.value("terminal_reference").toObject();
            const bool host = reference.value("action").toString() == QStringLiteral("terminal.host-recover");
            QJsonObject payload{{"reference", original}};
            if (!host) {
                payload.insert("expected_container", reference.value("container"));
                payload.insert("retire", action == "retire");
            }
            const auto root = host ? reference.value("root").toString() : QString();
            const auto intent = RemoteFeatureClient::prepareIntent(reference.value("execution_target").toObject(), root,
                reference.value("action").toString(), payload);
            RemoteFeatureClient::invoke(this, intent, [this, original, root, host](const RemoteFeatureClient::Result &result) {
                m_busy = false;
                const auto receipt = result.value.toObject();
                const bool verified = result.outcome == RemoteFeatureClient::Outcome::Success
                    && receipt.value("schema") == (host ? QStringLiteral("dockpipe.host-terminal-receipt/v1")
                                                        : QStringLiteral("dockpipe.container-terminal-receipt/v1"))
                    && receipt.value("reference").toObject() == original && receipt.value("reconciled").toBool()
                    && (!host || receipt.value("root").toString() == root);
                if (verified) {
                    if (m_verifiedRecovery)
                        m_verifiedRecovery(original, receipt);
                    showActionFeedback(host
                        ? text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Cleanup verified; the mutation outcome remains unknown. Refresh Files, Git or the container provider and review the affected state before another write."))
                        : text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Container terminal cleanup was verified for the original captured session.")));
                } else {
                    if (result.outcome == RemoteFeatureClient::Outcome::Success)
                        RemoteFeatureClient::retainUncertain(result.reference);
                    showActionFeedback(result.error.isEmpty()
                        ? (host
                            ? text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "The mutation outcome remains uncertain. Refresh its original status before another write."))
                            : text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Container terminal cleanup remains uncertain. The original session reference is retained.")))
                        : result.error);
                }
                updateSelection();
                refreshCatalogs();
            });
            return;
        }
        auto completion = [this](const RemoteFeatureClient::Result &result) {
            m_busy = false;
            const auto operation = result.value.toObject();
            reload();
            const auto hostRecovery = verifiedHostRecovery(operation);
            if (result.outcome == RemoteFeatureClient::Outcome::Success && !hostRecovery.isEmpty()) {
                if (m_verifiedRecovery)
                    m_verifiedRecovery(hostRecovery.value("reference").toObject(), hostRecovery);
                showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Cleanup verified; the mutation outcome remains unknown. Refresh Files, Git or the container provider and review the affected state before another write.")));
            } else if (operation.value("request").toObject().value("action") == QStringLiteral("terminal.host-recover")
                       && operation.value("status") == QStringLiteral("success")) {
                RemoteFeatureClient::retainUncertain(result.reference);
                reload();
                showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "The mutation outcome remains uncertain. Refresh its original status before another write.")));
            } else if (operation.value("reconciled").toBool()) {
                showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Cleanup verified; the mutation outcome remains unknown. Refresh Files, Git or the container provider and review the affected state before another write.")));
            } else if (result.outcome == RemoteFeatureClient::Outcome::Cancelled) {
                showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "The original operation was cancelled. Refresh the affected state before another write.")));
            } else if (!result.error.isEmpty()) {
                showActionFeedback(result.error);
            } else {
                const QString status = operation.value("status").toString();
                showActionFeedback(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Original operation status: %1. No operation was replayed.")).arg(status.isEmpty() ? text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "unknown")) : status));
            }
        };
        if (action == "cancel")
            RemoteFeatureClient::cancelOperation(this, reference, std::move(completion));
        else if (action == "reconcile")
            RemoteFeatureClient::reconcile(this, reference, std::move(completion));
        else if (action == "retire")
            RemoteFeatureClient::retireContainer(this, reference, std::move(completion));
        else
            RemoteFeatureClient::inspect(this, reference, std::move(completion));
    }

    QComboBox *m_operations;
    QLabel *m_identity;
    QLabel *m_status;
    QPushButton *m_inspect;
    QPushButton *m_cancel;
    QPushButton *m_reconcile;
    QPushButton *m_retire;
    QJsonArray m_terminalRecoveries;
    QString m_actionFeedback;
    RemoteFeatureRecoveryDialog::VerifiedRecovery m_verifiedRecovery;
    bool m_catalogBusy = false;
    bool m_busy = false;
};
}

QPushButton *RemoteFeatureRecoveryDialog::createButton(QWidget *parent, VerifiedRecovery verifiedRecovery)
{
    auto *button = new QPushButton(text(QT_TRANSLATE_NOOP("RemoteFeatureRecoveryDialog", "Uncertain operations…")), parent);
    button->setObjectName(QStringLiteral("remoteFeatureRecovery"));
    QObject::connect(button, &QPushButton::clicked, button, [button, verifiedRecovery = std::move(verifiedRecovery)] {
        auto *dialog = static_cast<RecoveryDialog *>(button->findChild<QDialog *>(QStringLiteral("remoteFeatureRecoveryDialog")));
        if (!dialog) {
            dialog = new RecoveryDialog(button, verifiedRecovery);
            dialog->setObjectName(QStringLiteral("remoteFeatureRecoveryDialog"));
        }
        dialog->refresh();
        dialog->show();
        dialog->raise();
        dialog->activateWindow();
    });
    return button;
}
