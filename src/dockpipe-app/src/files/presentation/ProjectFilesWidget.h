#pragma once

#include "LauncherUiExport.h"

#include "CockpitSettings.h"
#include <QWidget>

class QFileSystemModel;
class QTreeView;
class QPlainTextEdit;
class QLabel;
class QTextDocument;
class QStackedWidget;
class WorkspaceGraphWidget;
class QSplitter;
class QPushButton;
class QToolButton;
class QTimer;
class MarkdownPreview;
class FileExplorerView;

// Reused with a project root or a single workflow directory. Never opens an
// external editor, and never traverses a link outside the selected root.
class DOCKPIPE_LAUNCHER_UI_EXPORT ProjectFilesWidget : public QWidget {
    Q_OBJECT
public:
    explicit ProjectFilesWidget(QWidget *parent = nullptr);
    ~ProjectFilesWidget() override;
    bool setRoot(const QString &root);
    bool openFile(const QString &path);
    bool reloadFile();
    bool saveFile();
    bool confirmLeave();
    bool hasUnsavedChanges() const;
    void setWorkflowDocument(const QString &path, QTextDocument *document);
    QString currentPath() const { return m_path; }
    bool createFile(const QString &relativePath);
    bool renameFile(const QString &destination);
    bool deleteFile();
    void applyCockpitSettings(const CockpitSettings &settings);
    void openGraph();
    void closeGraph();

signals:
    void workflowOpenRequested(const QString &path);
    void workflowSaveRequested();
    void fileSaved(const QString &path);
    void dirtyChanged(bool dirty);

protected:
    void hideEvent(QHideEvent *event) override;
    bool eventFilter(QObject *object, QEvent *event) override;

private:
    bool contained(const QString &path) const;
    void showMessage(const QString &message);
    void updateTitle();
    void clearFile();
    void updateEditorStatus();
    void updateMarkdownPreview();
    void renderMarkdownPreview();
    bool navigateExplorer(const QString &directory);
    QString m_root;
    QString m_path;
    QString m_workflowPath;
    QByteArray m_original;
    bool m_crlf = false;
    bool m_bom = false;
    QFileSystemModel *m_model;
    QTreeView *m_tree;
    FileExplorerView *m_explorer;
    QToolButton *m_explorerUp;
    QLabel *m_explorerLocation;
    QString m_explorerDirectory;
    QPlainTextEdit *m_editor;
    QPlainTextEdit *m_workflowEditor;
    QStackedWidget *m_documents;
    QLabel *m_title;
    QLabel *m_status;
    QWidget *m_controls;
    QWidget *m_fileActions;
    QPushButton *m_openWorkflow;
    QPushButton *m_graphButton;
    QToolButton *m_previewButton;
    MarkdownPreview *m_markdownPreview;
    QTimer *m_previewTimer;
    QWidget *m_findBar;
    QWidget *m_empty = nullptr;
    QLabel *m_editorStatus = nullptr;
    QSplitter *m_splitter;
    QWidget *m_workspace;
    WorkspaceGraphWidget *m_graph = nullptr;
};
