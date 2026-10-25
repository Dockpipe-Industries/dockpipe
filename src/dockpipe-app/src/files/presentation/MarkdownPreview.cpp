#include "MarkdownPreview.h"
#include "MermaidRenderer.h"

#include <QHideEvent>
#include <QShowEvent>
#include <QRegularExpression>
#include <QScrollBar>
#include <QTextDocument>
#include <QTextImageFormat>
#include <QTimer>
#include <QUuid>

namespace {
// Live preview budgets: at most 16 diagrams, 32 MiB of decoded images and
// 20 seconds for a document. Additional diagrams remain readable as source.
constexpr int maxDiagrams = 16;
constexpr qint64 maxImageBytes = 32 * 1024 * 1024;
constexpr int documentDeadlineMs = 20000;

class MarkdownDocument final : public QTextDocument {
public:
    explicit MarkdownDocument(QObject *parent) : QTextDocument(parent) {}

protected:
    QVariant loadResource(int, const QUrl &) override
    {
        // Override at the document boundary: an empty browser result alone lets
        // QTextDocument fall back to reading local files.
        return {};
    }
};
}

MarkdownPreview::MarkdownPreview(QWidget *parent) : QTextBrowser(parent)
{
    setDocument(new MarkdownDocument(this));
    setObjectName("fileMarkdownPreview");
    setAccessibleName(tr("Markdown preview"));
    setOpenLinks(false);
    setOpenExternalLinks(false);
    setFrameShape(QFrame::NoFrame);
    document()->setDocumentMargin(24);
    m_deadline = new QTimer(this);
    m_deadline->setSingleShot(true);
    connect(m_deadline, &QTimer::timeout, this, [this]() {
        if (m_renderer)
            m_renderer->cancel();
        while (m_nextDiagram < m_diagrams.size())
            showDiagramError(m_diagrams.at(m_nextDiagram++), tr("Document rendering exceeded 20 seconds."));
    });
}

void MarkdownPreview::render(const QString &markdown)
{
    cancelDiagrams();
    m_markdown = markdown;
    m_resumeDiagrams = false;
    const int scroll = verticalScrollBar()->value();
    QString prepared;
    const auto lines = markdown.split('\n');
    const QRegularExpression fencePattern(QStringLiteral("^ {0,3}(`{3,}|~{3,})(.*)$"));
    QString openingFence;
    QString language;
    QStringList fencedLines;
    bool additionalDiagrams = false;
    for (const auto &line : lines) {
        const auto fence = fencePattern.match(line);
        if (openingFence.isEmpty()) {
            if (fence.hasMatch()) {
                openingFence = fence.captured(1);
                language = fence.captured(2).trimmed().section(QRegularExpression("\\s+"), 0, 0);
                // CommonMark forbids backticks in a backtick fence's info string.
                if (openingFence.startsWith('`') && fence.captured(2).contains('`'))
                    openingFence.clear();
                else
                    fencedLines.append(line);
            }
            if (openingFence.isEmpty())
                prepared += line + '\n';
            continue;
        }
        fencedLines.append(line);
        const bool closes = fence.hasMatch() && fence.captured(1).front() == openingFence.front()
            && fence.captured(1).size() >= openingFence.size() && fence.captured(2).trimmed().isEmpty();
        if (!closes)
            continue;
        if (language.compare("mermaid", Qt::CaseInsensitive) == 0 && m_diagrams.size() < maxDiagrams) {
            Diagram diagram;
            diagram.marker = "dockpipe-mermaid-" + QUuid::createUuid().toString(QUuid::WithoutBraces);
            diagram.source = fencedLines.mid(1, fencedLines.size() - 2).join('\n');
            m_diagrams.append(diagram);
            prepared += '\n' + diagram.marker + "\n\n";
        } else {
            if (language.compare("mermaid", Qt::CaseInsensitive) == 0)
                additionalDiagrams = true;
            prepared += fencedLines.join('\n') + '\n';
        }
        openingFence.clear();
        fencedLines.clear();
    }
    // An unfinished fence stays source while the user types its closing marker.
    prepared += fencedLines.join('\n');
    if (additionalDiagrams)
        prepared += tr("\n\nMermaid preview limit: diagrams after the first 16 are shown as source.\n");
    const auto features = QTextDocument::MarkdownFeatures(QTextDocument::MarkdownDialectGitHub)
        | QTextDocument::MarkdownNoHTML;
    document()->setMarkdown(prepared, features);
    for (int index = 0; index < m_diagrams.size(); ++index) {
        auto &diagram = m_diagrams[index];
        auto cursor = document()->find(diagram.marker);
        if (cursor.isNull())
            continue;
        const int position = cursor.selectionStart();
        const QString label = tr("Rendering Mermaid diagram %1…").arg(index + 1);
        cursor.insertText(label);
        cursor.setPosition(position);
        cursor.setPosition(position + label.size(), QTextCursor::KeepAnchor);
        diagram.placeholder = cursor;
    }
    verticalScrollBar()->setValue(scroll);
    if (m_diagrams.isEmpty())
        return;
    if (!m_renderer) {
        m_renderer = MermaidRenderer::create(this);
        if (m_renderer)
            connect(m_renderer, &MermaidRenderer::finished, this, &MarkdownPreview::finishDiagram);
    }
    if (!m_renderer) {
        while (m_nextDiagram < m_diagrams.size())
            showDiagramError(m_diagrams.at(m_nextDiagram++), tr("Mermaid rendering is unavailable in this host."));
        return;
    }
    m_deadline->start(documentDeadlineMs);
    renderNextDiagram();
}

void MarkdownPreview::cancelDiagrams()
{
    if (m_renderer)
        m_renderer->cancel();
    m_deadline->stop();
    m_diagrams.clear();
    m_nextDiagram = 0;
    m_imageBytes = 0;
}

void MarkdownPreview::renderNextDiagram()
{
    if (m_nextDiagram >= m_diagrams.size()) {
        m_deadline->stop();
        return;
    }
    m_renderer->render(m_diagrams.at(m_nextDiagram).source, palette().color(QPalette::Base).lightness() < 128);
}

void MarkdownPreview::showDiagramError(const Diagram &diagram, const QString &error)
{
    auto cursor = diagram.placeholder;
    if (!cursor.isNull())
        cursor.insertText(tr("Mermaid diagram: %1\n%2").arg(error.left(512), diagram.source));
}

void MarkdownPreview::finishDiagram(const QImage &image, const QString &error)
{
    if (m_nextDiagram >= m_diagrams.size())
        return;
    const auto diagram = m_diagrams.at(m_nextDiagram++);
    if (!error.isEmpty() || image.isNull()) {
        showDiagramError(diagram, error.isEmpty() ? tr("The renderer returned no diagram.") : error);
    } else if (image.sizeInBytes() > maxImageBytes - m_imageBytes) {
        showDiagramError(diagram, tr("Document diagrams exceed the 32 MiB image budget."));
    } else {
        m_imageBytes += image.sizeInBytes();
        const QUrl resource("dockpipe-mermaid:/" + diagram.marker);
        document()->addResource(QTextDocument::ImageResource, resource, image);
        auto cursor = diagram.placeholder;
        if (!cursor.isNull()) {
            QTextImageFormat format;
            format.setName(resource.toString());
            const int width = qMax(1, qMin(image.width(), viewport()->width() - 48));
            format.setWidth(width);
            format.setHeight(qreal(image.height()) * width / image.width());
            format.setProperty(QTextFormat::ImageAltText, tr("Mermaid diagram"));
            cursor.insertImage(format);
        }
    }
    renderNextDiagram();
}

void MarkdownPreview::hideEvent(QHideEvent *event)
{
    m_resumeDiagrams = m_nextDiagram < m_diagrams.size();
    cancelDiagrams();
    QTextBrowser::hideEvent(event);
}

void MarkdownPreview::showEvent(QShowEvent *event)
{
    QTextBrowser::showEvent(event);
    if (m_resumeDiagrams) {
        m_resumeDiagrams = false;
        QTimer::singleShot(0, this, [this]() {
            if (isVisible())
                render(m_markdown);
        });
    }
}

