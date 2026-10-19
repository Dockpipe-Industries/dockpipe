#include "SendInputWidget.h"
#include "TerminalHub.h"
#include "CanvasIcons.h"
#include <QDynamicPropertyChangeEvent>
#include <QGridLayout>
#include <QTextDocument>
#include <QComboBox>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPainter>
#include <QPushButton>
#include <QShortcut>
#include <QSignalBlocker>
#include <QVBoxLayout>

SendInputWidget::SendInputWidget(TerminalHub &hub, QWidget *parent)
    : QWidget(parent), m_hub(hub)
{
    setObjectName("sendInputWidget");
    m_layout = new QGridLayout(this);
    m_layout->setRowStretch(1, 1);
    m_targets = new QComboBox;
    m_targets->setObjectName("sendInputTarget");
    m_targets->setAccessibleName(tr("Send input to"));
    m_layout->addWidget(m_targets, 0, 0);
    m_input = new QPlainTextEdit;
    m_input->setObjectName("sendInputText");
    m_documentMargin = m_input->document()->documentMargin();
    m_input->setPlaceholderText(tr("Input for the selected terminal…"));
    m_input->setMaximumHeight(160);
    m_layout->addWidget(m_input, 1, 0);
    m_send = new QPushButton(tr("Send + Enter"));
    m_send->setObjectName("sendInputButton");
    m_send->setToolTip(tr("Send to the selected terminal (Ctrl+Enter)"));
    m_layout->addWidget(m_send, 2, 0);
    m_status = new QLabel(tr("Choose a terminal. Input runs with that session's permissions."));
    m_status->setTextFormat(Qt::PlainText);
    m_status->setWordWrap(true);
    m_layout->addWidget(m_status, 3, 0);
    setToolTip(m_status->text());
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
            showStatus(tr("Input is limited to 64 KiB."));
        }
        updateComposerHeight();
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
        showStatus(error);
        return;
    }
    m_input->clear();
    showStatus(tr("Sent to %1").arg(m_hub.title(m_targets->currentData().toString())));
}


bool SendInputWidget::event(QEvent *event)
{
    if (event->type() == QEvent::DynamicPropertyChange) {
        const auto *change = static_cast<QDynamicPropertyChangeEvent *>(event);
        if (change->propertyName() == "dashboardCanvasPresentation")
            setCanvasPresentationEnabled(property("dashboardCanvasPresentation").toBool());
    }
    if (event->type() == QEvent::PaletteChange && m_canvasPresentation)
        updateSendIcon();
    return QWidget::event(event);
}

void SendInputWidget::setCanvasPresentationEnabled(bool enabled)
{
    if (m_canvasPresentation == enabled)
        return;
    m_canvasPresentation = enabled;
    for (QWidget *widget : QList<QWidget *>{m_targets, m_input, m_send, m_status})
        m_layout->removeWidget(widget);
    m_layout->setRowStretch(1, 0);
    m_layout->setColumnStretch(0, 0);
    m_layout->setColumnStretch(1, enabled ? 1 : 0);
    if (enabled) {
        m_layout->setContentsMargins(10, 8, 10, 8);
        m_layout->setHorizontalSpacing(8);
        m_layout->setVerticalSpacing(4);
        m_layout->addWidget(m_targets, 0, 0, Qt::AlignTop);
        m_layout->addWidget(m_input, 0, 1);
        m_layout->addWidget(m_send, 0, 2, Qt::AlignTop);
        m_layout->addWidget(m_status, 1, 0, 1, 3);
        m_targets->setMaximumWidth(190);
        m_targets->setMinimumHeight(36);
        m_send->setText({});
        updateSendIcon();
        m_send->setFixedSize(36, 36);
        m_send->setAccessibleName(tr("Send input and Enter"));
        m_input->document()->setDocumentMargin(0);
        m_input->setPlaceholderText(tr("Send input…"));
        m_status->hide();
        setStyleSheet(QStringLiteral(
            "QWidget#sendInputWidget { background: transparent; }"
            "QPlainTextEdit#sendInputText { border: none; border-radius: 16px; padding: 7px 10px; background: palette(base); }"
            "QPushButton#sendInputButton { border: none; border-radius: 18px; min-width: 36px; max-width: 36px; min-height: 36px; max-height: 36px; padding: 0; background: palette(highlight); color: palette(highlighted-text); }"
            "QPushButton#sendInputButton:disabled { background: palette(mid); color: palette(button-text); }"
            "QComboBox#sendInputTarget { border: none; border-radius: 14px; padding: 5px 8px; background: palette(alternate-base); }"
            "QLabel { color: palette(text); padding-left: 5px; }"));
    } else {
        m_layout->setContentsMargins(9, 9, 9, 9);
        m_layout->setSpacing(6);
        m_layout->addWidget(m_targets, 0, 0);
        m_layout->addWidget(m_input, 1, 0);
        m_layout->addWidget(m_send, 2, 0);
        m_layout->addWidget(m_status, 3, 0);
        m_layout->setRowStretch(1, 1);
        m_targets->setMaximumWidth(QWIDGETSIZE_MAX);
        m_targets->setMinimumHeight(0);
        m_send->setMinimumSize(0, 0);
        m_send->setMaximumSize(QWIDGETSIZE_MAX, QWIDGETSIZE_MAX);
        m_send->setIcon({});
        m_send->setText(tr("Send + Enter"));
        m_input->document()->setDocumentMargin(m_documentMargin);
        m_input->setPlaceholderText(tr("Input for the selected terminal…"));
        m_status->show();
        setStyleSheet({});
    }
    updateComposerHeight();
    updateGeometry();
}

void SendInputWidget::updateComposerHeight()
{
    if (m_canvasPresentation) {
        const int lines = qBound(1, m_input->document()->blockCount(), 5);
        const int height = qMax(36, lines * fontMetrics().lineSpacing() + 16);
        m_input->setFixedHeight(height);
    } else {
        m_input->setMinimumHeight(0);
        m_input->setMaximumHeight(160);
    }
}

void SendInputWidget::showStatus(const QString &message)
{
    m_status->setText(message);
    m_status->setVisible(true);
    updateGeometry();
}


void SendInputWidget::updateSendIcon()
{
    const qreal ratio = devicePixelRatioF();
    const QSize physicalSize(qRound(20 * ratio), qRound(20 * ratio));
    QIcon icon;
    for (const auto mode : {QIcon::Normal, QIcon::Disabled}) {
        QPixmap pixmap = CanvasIcons::icon(CanvasIcons::Symbol::Up).pixmap(physicalSize);
        QPainter painter(&pixmap);
        painter.setCompositionMode(QPainter::CompositionMode_SourceIn);
        const QColor color = mode == QIcon::Normal
            ? palette().color(QPalette::HighlightedText) : palette().color(QPalette::ButtonText);
        painter.fillRect(pixmap.rect(), color);
        painter.end();
        pixmap.setDevicePixelRatio(ratio);
        icon.addPixmap(pixmap, mode);
    }
    m_send->setIcon(icon);
    m_send->setIconSize(QSize(20, 20));
}
