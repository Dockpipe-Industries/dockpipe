#pragma once
#include <QWidget>
class TerminalHub;
class QComboBox;
class QPlainTextEdit;
class QLabel;
class QPushButton;

// A selected destination owns execution; the composer never broadcasts input.
class SendInputWidget final : public QWidget {
    Q_OBJECT
public:
    explicit SendInputWidget(TerminalHub &hub, QWidget *parent = nullptr);
private:
    void refreshTargets();
    void send();
    TerminalHub &m_hub;
    QComboBox *m_targets;
    QPlainTextEdit *m_input;
    QLabel *m_status;
    QPushButton *m_send;
};
