#pragma once

#include <QDialog>
#include <QProcess>
#include <QTimer>
#include <QTemporaryDir>

class QLabel;
class QPlainTextEdit;
class QPushButton;

// Thin client of the CLI change/recovery contract. The CLI owns all validation,
// approval binding and state; this dialog never mutates an installation.
class DependencyChangeDialog : public QDialog {
    Q_OBJECT
public:
    explicit DependencyChangeDialog(const QString &workdir, QWidget *parent = nullptr);
    void loadRequest(const QString &path);

protected:
    void reject() override;

private:
    void chooseRequest();
    void execute(const QString &operation, bool dryRun);
    void finish(bool success);
    void readOutput();
    void setBusy(bool busy);

    QString m_workdir;
    QString m_request;
    QByteArray m_requestPreview;
    QTemporaryDir m_requestDirectory;
    QString m_applyApproval;
    QString m_recoveryApproval;
    QString m_activateApproval;
    QProcess m_process;
    QTimer m_timeout;
    QByteArray m_stdout;
    QByteArray m_stderr;
    bool m_reviewing = false;
    QString m_operation;
    bool m_failed = false;
    QLabel *m_requestLabel = nullptr;
    QPlainTextEdit *m_output = nullptr;
    QPushButton *m_choose = nullptr;
    QPushButton *m_reviewUpdate = nullptr;
    QPushButton *m_reviewRecovery = nullptr;
    QPushButton *m_apply = nullptr;
    QPushButton *m_recover = nullptr;
    QPushButton *m_reviewActivation = nullptr;
    QPushButton *m_activate = nullptr;
    QPushButton *m_health = nullptr;
};
