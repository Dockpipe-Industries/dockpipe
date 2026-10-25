#pragma once
#include <QWidget>
class TerminalHub;
class QComboBox;
class QPlainTextEdit;
class QLabel;
class QPushButton;
class QGridLayout;

// A selected destination owns execution; the composer never broadcasts input.
class SendInputWidget final : public QWidget {
    Q_OBJECT
public:
    explicit SendInputWidget(TerminalHub &hub, QWidget *parent = nullptr);
    void setCanvasPresentationEnabled(bool enabled);
    bool canvasPresentationEnabled() const { return m_canvasPresentation; }

protected:
    bool event(QEvent *event) override;
private:
    void refreshTargets();
    void send();
    void updateComposerHeight();
    void updateSendIcon();
    void showStatus(const QString &message);
    bool m_canvasPresentation = false;
    qreal m_documentMargin = 0;
    QGridLayout *m_layout;
    TerminalHub &m_hub;
    QComboBox *m_targets;
    QPlainTextEdit *m_input;
    QLabel *m_status;
    QPushButton *m_send;
};
