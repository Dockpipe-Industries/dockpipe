#include "SendInputWidget.h"
#include "TerminalHub.h"
#include <QComboBox>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QShortcut>
#include <QSignalBlocker>
#include <QVBoxLayout>

SendInputWidget::SendInputWidget(TerminalHub &hub, QWidget *parent)
    : QWidget(parent), m_hub(hub)
{
    setObjectName("sendInputWidget");
    auto *layout = new QVBoxLayout(this);
    m_targets = new QComboBox;
    m_targets->setObjectName("sendInputTarget");
    m_targets->setAccessibleName(tr("Send input to"));
    layout->addWidget(m_targets);
    m_input = new QPlainTextEdit;
    m_input->setObjectName("sendInputText");
    m_input->setPlaceholderText(tr("Input for the selected terminal…"));
    m_input->setMaximumHeight(160);
    layout->addWidget(m_input, 1);
    m_send = new QPushButton(tr("Send + Enter"));
    m_send->setObjectName("sendInputButton");
    m_send->setToolTip(tr("Send to the selected terminal (Ctrl+Enter)"));
    layout->addWidget(m_send);
    m_status = new QLabel(tr("Choose a terminal. Input runs with that session's permissions."));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    layout->addWidget(m_status);
    connect(m_send, &QPushButton::clicked, this, &SendInputWidget::send);
    auto *shortcut = new QShortcut(QKeySequence(Qt::CTRL | Qt::Key_Return), this);
    shortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(shortcut, &QShortcut::activated, this, &SendInputWidget::send);
    connect(&m_hub, &TerminalHub::sessionsChanged, this, &SendInputWidget::refreshTargets);
    connect(m_targets, &QComboBox::currentIndexChanged, this, [this]() {
        m_send->setEnabled(m_hub.running(m_targets->currentData().toString()));
    });
    connect(m_input, &QPlainTextEdit::textChanged, this, [this]() {
        // Bound retained composer state too, not only the transport payload.
        if (m_input->document()->characterCount() > 65536) {
            QSignalBlocker blocker(m_input);
            auto cursor = m_input->textCursor();
            cursor.setPosition(65535);
            cursor.movePosition(QTextCursor::End, QTextCursor::KeepAnchor);
            cursor.removeSelectedText();
            m_status->setText(tr("Input is limited to 64 KiB."));
        }
    });
    refreshTargets();
}

void SendInputWidget::refreshTargets()
{
    const QString selected = m_targets->currentData().toString();
    QSignalBlocker blocker(m_targets);
    m_targets->clear();
    m_targets->addItem(tr("Choose a terminal…"), QString());
    for (const QString &id : m_hub.sessionIds()) {
        if (m_hub.running(id))
            m_targets->addItem(m_hub.title(id), id);
    }
    m_targets->setCurrentIndex(qMax(0, m_targets->findData(selected)));
    m_send->setEnabled(m_hub.running(m_targets->currentData().toString()));
}

void SendInputWidget::send()
{
    if (m_input->toPlainText().isEmpty())
        return;
    QString error;
    if (!m_hub.send(m_targets->currentData().toString(), m_input->toPlainText(), &error)) {
        m_status->setText(error);
        return;
    }
    m_input->clear();
    m_status->setText(tr("Sent to %1").arg(m_hub.title(m_targets->currentData().toString())));
}
