#include "RemoteRunDialog.h"
#include "DockpipeChoices.h"

#include <QFormLayout>
#include <QJsonDocument>
#include <QJsonObject>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QVBoxLayout>
#include <QUuid>

RemoteRunDialog::RemoteRunDialog(const Context &context, const QString &node, QWidget *parent)
    : QDialog(parent), m_context(context), m_node(node), m_command(this)
{
    setWindowTitle(tr("Run workflow on %1").arg(node));
    resize(720, 650);
    m_job = QStringLiteral("launcher-") + QUuid::createUuid().toString(QUuid::WithoutBraces);
    auto *root = new QVBoxLayout(this);
    auto *heading = new QLabel(tr("%1 → %2").arg(context.label, node));
    heading->setObjectName(QStringLiteral("appTitle"));
    root->addWidget(heading);
    auto *intro = new QLabel(tr("Review the source snapshot before sending. The worker uses the workflow's YAML settings. "
                               "Include required source files and unpacked package dependencies explicitly. Review the contents and never include credentials."));
    intro->setWordWrap(true);
    root->addWidget(intro);
    auto *form = new QFormLayout;
    auto editor = [this, form](const QString &label, const QString &placeholder) {
        auto *field = new QPlainTextEdit;
        field->setMaximumHeight(65);
        field->setPlaceholderText(placeholder);
        form->addRow(label, field);
        connect(field, &QPlainTextEdit::textChanged, this, &RemoteRunDialog::invalidatePreview);
        return field;
    };
    m_includes = editor(tr("Additional source paths"), tr("One path inside the project per line"));
    m_dependencies = editor(tr("Package dependencies"), tr("One unpacked package directory per line"));
    m_artifacts = editor(tr("Files to collect"), tr("One result path relative to the delivered project per line"));
    root->addLayout(form);
    m_status = new QLabel(tr("Preview the snapshot to enable Run."));
    m_status->setWordWrap(true);
    root->addWidget(m_status);
    m_preview = new QPlainTextEdit;
    m_preview->setReadOnly(true);
    m_preview->setMaximumBlockCount(5000);
    root->addWidget(m_preview, 1);
    auto *buttons = new QHBoxLayout;
    auto *preview = new QPushButton(tr("Preview files"));
    m_send = new QPushButton(tr("Run on %1").arg(node));
    m_send->setEnabled(false);
    auto *close = new QPushButton(tr("Close"));
    buttons->addWidget(preview);
    buttons->addStretch();
    buttons->addWidget(close);
    buttons->addWidget(m_send);
    root->addLayout(buttons);
    connect(close, &QPushButton::clicked, this, &QDialog::reject);
    connect(&m_command, &RemoteCommand::output, this, [this](const QString &text) {
        m_preview->appendPlainText(text.trimmed());
    });
    connect(&m_command, &RemoteCommand::busyChanged, this, [this, preview](bool busy) {
        preview->setEnabled(!busy);
        m_includes->setEnabled(!busy);
        m_dependencies->setEnabled(!busy);
        m_artifacts->setEnabled(!busy);
        m_send->setEnabled(!busy && !m_digest.isEmpty());
    });
    connect(preview, &QPushButton::clicked, this, [this]() {
        invalidatePreview();
        m_preview->clear();
        auto args = arguments();
        args.append(QStringLiteral("--dry-run"));
        m_command.run(DockpipeChoices::preferredDockpipeBinary(m_context.workdir), args,
                      [this](bool ok, const QByteArray &data) {
            if (!ok) {
                m_status->setText(tr("Preview failed. Nothing was submitted."));
                return;
            }
            const auto doc = QJsonDocument::fromJson(data);
            m_digest = doc.object().value("submission").toObject().value("bundle_hash").toString();
            m_preview->setPlainText(QString::fromUtf8(doc.toJson(QJsonDocument::Indented)));
            m_send->setEnabled(!m_digest.isEmpty());
            m_status->setText(tr("Review these files. Run will reject changes made after this preview."));
        }, 120000);
    });
    connect(m_send, &QPushButton::clicked, this, [this]() {
        auto args = arguments();
        args << "--expected-digest" << m_digest;
        m_command.run(DockpipeChoices::preferredDockpipeBinary(m_context.workdir), args,
                      [this](bool ok, const QByteArray &) {
            if (ok) {
                accept();
            } else {
                m_status->setText(tr("Submission was not confirmed. Retry uses the same job ID; inspect Activity if the response was lost."));
            }
        }, 120000);
    });
}

void RemoteRunDialog::invalidatePreview()
{
    m_digest.clear();
    if (m_send)
        m_send->setEnabled(false);
}

QStringList RemoteRunDialog::arguments() const
{
    QStringList args{"remote", "submit", "--node", m_node, "--id", m_job,
                     "--workdir", m_context.workdir, "--workflow-file", m_context.workflowFile};
    auto append = [&args](const QString &flag, QPlainTextEdit *field) {
        for (const QString &line : field->toPlainText().split('\n')) {
            if (!line.trimmed().isEmpty())
                args << flag << line.trimmed();
        }
    };
    append("--include", m_includes);
    append("--dependency", m_dependencies);
    append("--artifact", m_artifacts);
    return args;
}
