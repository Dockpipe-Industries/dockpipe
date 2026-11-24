#pragma once

#include "PipeLangLanguageClient.h"
#include <QPointer>
#include <QHash>
#include <QPoint>

class FileEditorWorkspace;
class QLabel;
class SourceEditor;
class QTextDocument;
class QToolButton;
class QWidget;

// Owns one shared language session for unique Files documents, independent of
// tab visibility and split views. Configuration is explicit and session-local.
class PipeLangEditorSession : public QObject {
    Q_OBJECT
public:
    explicit PipeLangEditorSession(FileEditorWorkspace *workspace);
    ~PipeLangEditorSession() override;
    void attachEditor(SourceEditor *editor);
    void synchronize();
    QWidget *createControls(QWidget *parent);

protected:
    bool eventFilter(QObject *object, QEvent *event) override;

private:
    struct Binding {
        QPointer<QTextDocument> document;
        int revision = -1;
    };
    void configure();
    void refreshEditors();
    void applyDiagnostics(const QString &path, int version, const QJsonArray &items);
    void showHover(const QString &path, int version, const QJsonValue &contents);
    FileEditorWorkspace *m_workspace;
    PipeLangLanguageClient m_client;
    QHash<QString, Binding> m_bindings;
    QList<QPointer<SourceEditor>> m_editors;
    QPointer<QLabel> m_status;
    QPointer<SourceEditor> m_hoverEditor;
    QPoint m_hoverPosition;
    int m_hoverOffset = -1;
};
