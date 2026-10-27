#include "HardwareKeyDialog.h"

#include <QComboBox>
#include <QDialogButtonBox>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QPushButton>
#include <QPlainTextEdit>
#include <QVBoxLayout>

HardwareKeyDialog::HardwareKeyDialog(QWidget *parent)
    : QDialog(parent), m_details(new QPlainTextEdit(this)), m_status(new QLabel(this)),
      m_devices(new QComboBox(this)), m_pin(new QLineEdit(this)),
      m_confirm(new QPushButton(tr("Confirm and use security key"), this))
{
    setWindowModality(Qt::WindowModal);
    setMinimumWidth(480);
    auto *layout = new QVBoxLayout(this);
    m_details->setReadOnly(true);
    m_details->setMinimumHeight(120);
    m_details->setMaximumHeight(260);
    layout->addWidget(m_details);
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    m_devices->setObjectName("hardwareKeyDevice");
    layout->addWidget(m_devices);
    m_pin->setObjectName("hardwareKeyPin");
    m_pin->setEchoMode(QLineEdit::Password);
    m_pin->setMaxLength(256);
    m_pin->setPlaceholderText(tr("Existing FIDO2 security-key PIN"));
    m_pin->setToolTip(tr("Use the PIN already configured on this key. This does not create or change its PIN."));
    m_pin->setInputMethodHints(Qt::ImhSensitiveData | Qt::ImhNoPredictiveText);
    layout->addWidget(m_pin);
    connect(m_pin, &QLineEdit::textChanged, this, [this]() {
        m_confirm->setEnabled(!m_promptId.isEmpty() && (!m_pin->isEnabled() || !m_pin->text().isEmpty()));
    });
    m_confirm->setObjectName("hardwareKeyConfirm");
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Cancel, this);
    buttons->addButton(m_confirm, QDialogButtonBox::AcceptRole);
    connect(buttons, &QDialogButtonBox::rejected, this, &QDialog::reject);
    connect(m_confirm, &QPushButton::clicked, this, [this]() {
        if (m_promptId.isEmpty() || m_devices->currentIndex() < 0)
            return;
        const QString id = m_promptId;
        const QString device = m_devices->currentData().toString();
        QString pin = m_pin->text();
        showStatus(tr("Waiting for your physical security key…"));
        emit responseReady(id, device, pin);
        pin.fill(QChar(0));
        pin.clear();
    });
    connect(this, &QDialog::rejected, m_pin, &QLineEdit::clear);
    layout->addWidget(buttons);
}

void HardwareKeyDialog::showPrompt(const QJsonObject &prompt)
{
    m_pin->clear();
    m_promptId = prompt.value("id").toString();
    setWindowTitle(prompt.value("title").toString());
    m_details->setPlainText(prompt.value("details").toString());
    m_status->clear();
    m_devices->clear();
    for (const auto &value : prompt.value("devices").toArray()) {
        const auto device = value.toObject();
        m_devices->addItem(device.value("name").toString(), device.value("id").toString());
    }
    m_devices->setEnabled(true);
    const bool pinRequired = prompt.value("pin_required").toBool();
    m_pin->setVisible(pinRequired);
    m_pin->setEnabled(pinRequired);
    m_confirm->setEnabled(!pinRequired);
    if (pinRequired)
        m_pin->setFocus();
}

void HardwareKeyDialog::showStatus(const QString &message)
{
    m_promptId.clear();
    m_pin->clear();
    m_pin->setEnabled(false);
    m_pin->hide();
    m_devices->setEnabled(false);
    m_confirm->setEnabled(false);
    m_status->setText(message);
}
