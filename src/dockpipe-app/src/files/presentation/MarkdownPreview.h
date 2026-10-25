#pragma once

#include <QTextBrowser>
#include <QTextCursor>

class MermaidRenderer;
class QTimer;
class QImage;

// A local text renderer, independent of the browser and optional Cockpit package.
// Markdown cannot fetch resources or navigate away from the editor implicitly.
class MarkdownPreview final : public QTextBrowser {
public:
    explicit MarkdownPreview(QWidget *parent = nullptr);
    void render(const QString &markdown);

protected:
    void hideEvent(QHideEvent *event) override;
    void showEvent(QShowEvent *event) override;

private:
    struct Diagram {
        QString source;
        QString marker;
        QTextCursor placeholder;
    };
    void cancelDiagrams();
    void renderNextDiagram();
    void finishDiagram(const QImage &image, const QString &error);
    void showDiagramError(const Diagram &diagram, const QString &error);
    MermaidRenderer *m_renderer = nullptr;
    QTimer *m_deadline;
    QList<Diagram> m_diagrams;
    int m_nextDiagram = 0;
    qint64 m_imageBytes = 0;
    QString m_markdown;
    bool m_resumeDiagrams = false;
};
