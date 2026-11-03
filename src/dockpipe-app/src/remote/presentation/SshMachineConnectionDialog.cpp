#include "SshMachineConnectionDialog.h"

#include <QDialogButtonBox>
#include <QFormLayout>
#include <QLabel>
#include <QLineEdit>
#include <QSpinBox>
#include <QVBoxLayout>

namespace {
QLineEdit *textField(const QString &name, const QString &value, int maximum, QWidget *parent)
{
    auto *field = new QLineEdit(value, parent);
    field->setObjectName(name);
    field->setMaxLength(maximum);
    return field;
}
}

SshMachineConnectionDialog::SshMachineConnectionDialog(const SshMachineConnection &initial, QWidget *parent)
    : QDialog(parent), m_id(initial.id.isEmpty() ? QStringLiteral("host") : initial.id),
      m_remoteDirectory(initial.remoteDirectory)
{
    setObjectName(QStringLiteral("sshMachineConnectionDialog"));
    setWindowTitle(tr("Host connection"));
    setMinimumWidth(480);
    auto *layout = new QVBoxLayout(this);
    auto *explanation = new QLabel(tr("Connect to a host using your SSH account. Sign in and verify its host key in the terminal. "
        "Passwords are entered directly into SSH and are never saved in this connection."), this);
    explanation->setWordWrap(true);
    explanation->setTextFormat(Qt::PlainText);
    layout->addWidget(explanation);
    auto *form = new QFormLayout;
    m_label = textField("sshMachineLabel", initial.label, 160, this);
    m_host = textField("sshMachineHost", initial.host, 253, this);
    m_user = textField("sshMachineUser", initial.user, 64, this);
    m_port = new QSpinBox(this);
    m_port->setObjectName(QStringLiteral("sshMachinePort"));
    m_port->setRange(1, 65535);
    m_port->setValue(initial.port);
    m_nativeCliPath = textField("sshMachineCli", initial.nativeCliPath, 4096, this);
    m_nativeCliPath->setPlaceholderText(tr("Absolute path to the host's Dockpipe CLI"));
    m_identityFile = textField("sshMachineIdentity", initial.identityFile, 4096, this);
    m_identityFile->setPlaceholderText(tr("Optional absolute path to an SSH identity file"));
    form->addRow(tr("Label"), m_label);
    form->addRow(tr("Host"), m_host);
    form->addRow(tr("SSH account"), m_user);
    form->addRow(tr("SSH port"), m_port);
    form->addRow(tr("Native CLI path"), m_nativeCliPath);
    form->addRow(tr("Identity reference"), m_identityFile);
    layout->addLayout(form);
    m_error = new QLabel(this);
    m_error->setObjectName(QStringLiteral("sshMachineError"));
    m_error->setTextFormat(Qt::PlainText);
    m_error->setWordWrap(true);
    layout->addWidget(m_error);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Save | QDialogButtonBox::Cancel, this);
    connect(buttons, &QDialogButtonBox::accepted, this, &SshMachineConnectionDialog::acceptConnection);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    layout->addWidget(buttons);
}

SshMachineConnection SshMachineConnectionDialog::connection() const
{
    SshMachineConnection value;
    value.id = m_id;
    value.label = m_label->text().trimmed();
    value.host = m_host->text().trimmed();
    value.user = m_user->text().trimmed();
    value.port = m_port->value();
    value.nativeCliPath = m_nativeCliPath->text();
    value.identityFile = m_identityFile->text();
    value.remoteDirectory = m_remoteDirectory;
    return value;
}

void SshMachineConnectionDialog::acceptConnection()
{
    const QString error = connection().validationError();
    m_error->setText(error);
    if (error.isEmpty())
        accept();
}
