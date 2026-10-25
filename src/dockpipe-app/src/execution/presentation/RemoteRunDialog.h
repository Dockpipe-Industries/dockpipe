#pragma once

#include "Context.h"
#include "RemoteCommand.h"
#include <QDialog>

class QLabel;
class QPlainTextEdit;
class QPushButton;

class RemoteRunDialog : public QDialog {
    Q_OBJECT
public:
    RemoteRunDialog(const Context &context, const QString &node, QWidget *parent = nullptr);
private:
    QStringList arguments() const;
    void invalidatePreview();
    Context m_context;
    QString m_node;
    QString m_job;
    QString m_digest;
    RemoteCommand m_command;
    QPlainTextEdit *m_includes;
    QPlainTextEdit *m_dependencies;
    QPlainTextEdit *m_artifacts;
    QPlainTextEdit *m_preview;
    QLabel *m_status;
    QPushButton *m_send = nullptr;
};
