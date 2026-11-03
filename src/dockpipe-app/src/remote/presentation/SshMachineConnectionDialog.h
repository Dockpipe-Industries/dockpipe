#pragma once

#include "SshMachineConnection.h"

#include <QDialog>

class QLabel;
class QLineEdit;
class QSpinBox;

class SshMachineConnectionDialog final : public QDialog {
public:
    explicit SshMachineConnectionDialog(const SshMachineConnection &initial, QWidget *parent = nullptr);
    SshMachineConnection connection() const;

private:
    void acceptConnection();

    QString m_id;
    QLineEdit *m_label;
    QLineEdit *m_host;
    QLineEdit *m_user;
    QSpinBox *m_port;
    QLineEdit *m_nativeCliPath;
    QLineEdit *m_identityFile;
    QString m_remoteDirectory;
    QLabel *m_error;
};
