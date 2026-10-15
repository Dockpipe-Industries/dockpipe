#pragma once

#include <QWidget>
#include <QJsonObject>
#include <QJsonArray>
#include <functional>

class WorkflowCanvas;
class ProjectFilesWidget;
class QLineEdit;
class QFormLayout;
class QPlainTextEdit;
class QComboBox;
class QLabel;
class QTabWidget;
class QProcess;
class QTimer;
class QPushButton;
class QTemporaryFile;
class QMenu;
class QResizeEvent;

class WorkflowEditorWidget : public QWidget {
    Q_OBJECT
public:
    explicit WorkflowEditorWidget(const QString &workdir, const QString &binary, QWidget *parent = nullptr);
    ~WorkflowEditorWidget() override;
    bool openWorkflow(const QString &path);
    bool setWorkspace(const QString &workdir, const QString &binary);
    bool confirmDiscardChanges();
    void setWorkspaceMenu(const QString &name, QMenu *menu);
    void setConsoleWidgets(QWidget *panel, QWidget *controls);
    void setConsoleVisible(bool visible);
    void setActivityPanel(QWidget *panel);
    QString workflowPath() const { return m_path; }
    QString workflowName() const { return m_document.value("name").toString(); }
    QString workdir() const { return m_workdir; }

signals:
    void browseRequested();
    void documentChanged(const QString &path);
    void runRequested(const QString &path);
    void workflowSaved(const QString &path);

protected:
    void resizeEvent(QResizeEvent *event) override;

private:
    QWidget *m_activityPanel = nullptr;
    void arrangeOverlays();
    void newWorkflow();
    bool discardChanges();
    void refreshCanvas(const QJsonArray &edits = {});
    void selectNode(const QJsonArray &path);
    void addStep(const QString &type);
    void applyInspector();
    void moveStep(int direction);
    void saveWorkflow(bool run);
    void validateSource(const QString &directory, const std::function<void()> &success);
    void startCli(const QStringList &arguments, const QByteArray &input,
                  const std::function<void(const QByteArray &)> &success);
    void finishCli();
    void setBusy(bool busy);
    void updateDocumentTitle();
    void showStatus(const QString &message, bool failure = false);
    QJsonValue selectedValue() const;
    QString layoutKey() const;
    void rememberSource(const QString &source);
    void updateStepVisibility();
    QString inspectorState() const;
    bool requireAppliedInspector();
    QString m_inspectorBaseline;

    QString m_workdir;
    QString m_binary;
    QString m_path;
    QByteArray m_diskBytes;
    QString m_savedSource;
    QJsonObject m_document;
    QJsonArray m_selection;
    QStringList m_history;
    WorkflowCanvas *m_canvas = nullptr;
    QTabWidget *m_tabs = nullptr;
    QPlainTextEdit *m_yaml = nullptr;
    ProjectFilesWidget *m_files = nullptr;
    QPushButton *m_filesToggle = nullptr;
    QWidget *m_actions = nullptr;
    QWidget *m_palette = nullptr;
    QWidget *m_inspector = nullptr;
    QWidget *m_viewControls = nullptr;
    QWidget *m_notice = nullptr;
    QWidget *m_consolePanel = nullptr;
    QWidget *m_consoleControls = nullptr;
    QPushButton *m_workspacePicker = nullptr;
    QPushButton *m_yamlToggle = nullptr;
    QTimer *m_noticeTimer = nullptr;
    QLabel *m_status = nullptr;
    QLabel *m_filename = nullptr;
    QLabel *m_selectionTitle = nullptr;
    QLineEdit *m_name = nullptr;
    QLineEdit *m_description = nullptr;
    QFormLayout *m_stepForm = nullptr;
    QLineEdit *m_id = nullptr;
    QComboBox *m_kind = nullptr;
    QLineEdit *m_cwd = nullptr;
    QLineEdit *m_runtime = nullptr;
    QLineEdit *m_resolver = nullptr;
    QLineEdit *m_package = nullptr;
    QLineEdit *m_workflow = nullptr;
    QLineEdit *m_outputs = nullptr;
    QPlainTextEdit *m_command = nullptr;
    QPlainTextEdit *m_vars = nullptr;
    QPushButton *m_properties = nullptr;
    QPushButton *m_cancel = nullptr;
    QProcess *m_process = nullptr;
    QTimer *m_deadline = nullptr;
    QTemporaryFile *m_validationFile = nullptr;
    QByteArray m_output;
    QByteArray m_errors;
    QString m_processFailure;
    std::function<void(const QByteArray &)> m_success;
    bool m_editPending = false;
    bool m_canvasReady = false;
    bool m_restoreLayout = false;
    bool m_busy = false;
    bool m_refreshing = false;
};
