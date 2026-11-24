#pragma once

#include "LauncherUiExport.h"
#include <QByteArray>
#include <QPointer>
#include <QStringList>
#include <QWidget>
#include <functional>
#include <map>
#include <memory>

class QStackedWidget;
class QSplitter;
class QTextDocument;
class SourceEditor;

struct FileEditorDocument {
    QString path;
    QByteArray original;
    bool crlf = false;
    bool bom = false;
    bool workflow = false;
    bool preview = false;
    QPointer<QTextDocument> document;
};

// Private Files-surface controller. Documents outlive their individual pane views.
class DOCKPIPE_LAUNCHER_UI_EXPORT FileEditorWorkspace : public QWidget {
    Q_OBJECT
public:
    FileEditorWorkspace(QStackedWidget *primaryStack, SourceEditor *primaryNormal,
        SourceEditor *primaryWorkflow, QWidget *primaryEmpty, QWidget *parent = nullptr);
    ~FileEditorWorkspace() override;

    FileEditorDocument *document(const QString &path);
    const FileEditorDocument *document(const QString &path) const;
    FileEditorDocument *activeDocument();
    QStringList paths() const;
    bool contains(const QString &path) const;
    bool activate(const QString &path);
    bool addDocument(const QString &path, const QByteArray &originalBytes,
        const QString &text, bool crlf, bool bom);
    void setReadOnly(bool readOnly);
    bool setWorkflowDocument(const QString &path, QTextDocument *document);
    bool openWorkflow(const QString &path);
    bool renameDocument(const QString &oldPath, const QString &newPath);
    void removeDocument(const QString &path);
    void clearDocuments();
    void split(Qt::Orientation orientation);
    void singlePane();
    bool openInOtherPane(const QString &path);
    bool moveToOtherPane(const QString &path);
    bool closeActiveTab();
    void refreshTabs();
    QStackedWidget *activeStack() const;
    SourceEditor *activeNormalEditor() const;
    SourceEditor *activeWorkflowEditor() const;
    SourceEditor *activeEditor() const;
    QString activePath() const;
    bool hasUnsavedChanges() const;
    QString error() const;

    std::function<bool(const QString &)> confirmClose;

signals:
    void currentChanged();
    void documentsChanged();

protected:
    bool eventFilter(QObject *object, QEvent *event) override;

private:
    struct Pane;
    Pane *createPane(QStackedWidget *stack, SourceEditor *normal,
        SourceEditor *workflow, QWidget *empty, const QString &tabName);
    void saveView(Pane *pane);
    void detachDocument(Pane *pane, QTextDocument *document);
    void showCurrent(Pane *pane);
    void select(Pane *pane, const QString &path);
    void addTab(Pane *pane, const QString &path);
    bool closeTab(Pane *pane, int index);
    void removeTab(Pane *pane, int index);
    void watchDocument(const QString &path, QTextDocument *document);
    int viewCount(const QString &path) const;

    QSplitter *m_splitter;
    std::unique_ptr<Pane> m_primary;
    std::unique_ptr<Pane> m_secondary;
    Pane *m_active = nullptr;
    std::map<QString, FileEditorDocument> m_documents;
    QString m_workflowPath;
    QPointer<QTextDocument> m_workflowDocument;
    QString m_error;
    bool m_readOnly = false;
};
