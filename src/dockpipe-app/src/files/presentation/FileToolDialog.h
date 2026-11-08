#pragma once

#include "Context.h"
#include "LauncherUiExport.h"
#include "SessionManager.h"
#include <QDialog>

class QLabel;
class MarkdownPreview;
class QPlainTextEdit;
class QTimer;

// Runs one explicitly selected package workflow. Results are rendered only after
// successful completion; closing the panel cancels its normal workflow session.
class DOCKPIPE_LAUNCHER_UI_EXPORT FileToolDialog final : public QDialog {
    Q_OBJECT
public:
    explicit FileToolDialog(const Context &context, const QString &title,
                            const QString &provider, const QString &file, QWidget *parent = nullptr);
    ~FileToolDialog() override;

protected:
    void done(int result) override;

private:
    void receive(const QString &text);
    void fail(const QString &message);
    void finish(int exitCode, QProcess::ExitStatus status);
    Context m_context;
    SessionManager m_sessions;
    QLabel *m_status;
    MarkdownPreview *m_preview;
    QPlainTextEdit *m_details;
    QTimer *m_deadline;
    QByteArray m_pending;
    QString m_result;
    int m_received = 0;
    bool m_hasResult = false;
    bool m_finished = false;
};
