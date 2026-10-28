#pragma once

#include <QDialog>
#include <QJsonObject>

class QLabel;
class QPlainTextEdit;
class QComboBox;
class QLineEdit;
class QPushButton;

// Collects one CLI-owned consent response without persisting credentials.
class HardwareKeyDialog : public QDialog {
    Q_OBJECT
public:
    explicit HardwareKeyDialog(QWidget *parent = nullptr);
    void showPrompt(const QJsonObject &prompt);
    void showStatus(const QString &message);

signals:
    void responseReady(const QString &id, const QString &device, const QString &pin);

private:
    QString m_promptId;
    QPlainTextEdit *m_details;
    QLabel *m_status;
    QComboBox *m_devices;
    QLineEdit *m_pin;
    QPushButton *m_confirm;
};
