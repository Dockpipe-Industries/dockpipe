#include "SourceEditor.h"
#include <QTextDocument>
#include <QTextOption>
#include "SourceHighlighter.h"
#include <definition.h>
#include <KSyntaxHighlighting/Theme>
#include <QFontDatabase>
#include <QPainter>
#include <QPaintEvent>
#include <QTextBlock>
#include <QKeyEvent>
#include <algorithm>
#include <QVariant>

namespace {
bool usesDarkEditorTheme(const QPalette &palette)
{
    // Text contrast remains meaningful when a stylesheet leaves a transparent
    // editor's Base palette role pointing at its previous opaque background.
    return palette.color(QPalette::Text).lightness() > 128;
}
}

SourceEditor::SourceEditor(QWidget *parent) : QPlainTextEdit(parent), m_gutter(new QWidget(this))
{
    setLayoutDirection(Qt::LeftToRight);
    auto textOptions = document()->defaultTextOption();
    textOptions.setTextDirection(Qt::LeftToRight);
    document()->setDefaultTextOption(textOptions);
    auto font = QFontDatabase::systemFont(QFontDatabase::FixedFont);
    font.setPointSize(11);
    setFont(font);
    document()->setDocumentMargin(14);
    setLineWrapMode(QPlainTextEdit::NoWrap);
    setTabStopDistance(fontMetrics().horizontalAdvance(' ') * 4);
    new SourceHighlighter(document());
    m_gutter->installEventFilter(this);
    connect(this, &QPlainTextEdit::blockCountChanged, this, [this]() { updateGutter(); });
    connect(this, &QPlainTextEdit::updateRequest, this, [this](const QRect &, int) { updateGutter(); });
    connect(this, &QPlainTextEdit::cursorPositionChanged, this, &SourceEditor::highlightCurrentLine);
    highlightCurrentLine();
    updateGutter();
    document()->setModified(false);
}

void SourceEditor::setLineMapping(const QVector<int> &lines)
{
    // Keep optional mapping on the existing gutter object so the shared native
    // editor class retains its ABI for already installed workspace packages.
    m_gutter->setProperty("sourceLineMapping", QVariant::fromValue(lines));
    m_gutter->setProperty("sourceLineMaximum", lines.isEmpty() ? 0 : *std::max_element(lines.cbegin(), lines.cend()));
    updateGutter();
}

void SourceEditor::setPlainText(const QString &text)
{
    auto *highlighter = document()->findChild<SourceHighlighter *>();
    const auto definition = highlighter ? highlighter->definition() : KSyntaxHighlighting::Definition{};
    const auto theme = highlighter ? highlighter->theme() : KSyntaxHighlighting::Theme{};

    // The syntax engine queues rehighlightBlock with QTextBlock handles. A full
    // replacement invalidates those handles. Destroy their receiver first so Qt
    // discards pending callbacks before the old document storage is replaced.
    delete highlighter;
    QPlainTextEdit::setPlainText(text);

    highlighter = new SourceHighlighter(document());
    if (theme.isValid())
        highlighter->setTheme(theme);
    highlighter->setDefinition(definition);
    highlighter->rehighlight();
}

void SourceEditor::loadFileText(const QString &text, const QString &path)
{
    // Retire queued QTextBlock callbacks before replacing their document storage.
    // Configure the next grammar while detached: opening a file must not first
    // highlight the previous file using the new file's language.
    delete document()->findChild<SourceHighlighter *>();
    QPlainTextEdit::setPlainText(text);
    auto *highlighter = new SourceHighlighter(nullptr);
    ensurePolished();
    highlighter->updateTheme(usesDarkEditorTheme(palette()));
    highlighter->setFileName(path);
    // Qt schedules the initial highlighting after attachment. The file's text
    // is available immediately, without several synchronous grammar passes.
    highlighter->setParent(document());
    highlighter->setDocument(document());
}

void SourceEditor::setFileDocument(QTextDocument *fileDocument, const QString &path)
{
    if (!fileDocument)
        return;
    QPlainTextEdit::setDocument(fileDocument);
    ensurePolished();
    if (fileDocument->defaultFont() != font())
        fileDocument->setDefaultFont(font());
    auto *highlighter = fileDocument->findChild<SourceHighlighter *>();
    if (!highlighter) {
        // Configure before attaching so the first syntax pass remains queued.
        highlighter = new SourceHighlighter(nullptr);
        highlighter->updateTheme(usesDarkEditorTheme(palette()));
        highlighter->setFileName(path);
        highlighter->setParent(fileDocument);
        highlighter->setDocument(fileDocument);
    } else {
        highlighter->updateTheme(usesDarkEditorTheme(palette()));
        highlighter->setFileName(path);
    }
    updateGutter();
    highlightCurrentLine();
}

int SourceEditor::gutterWidth() const
{
    return 30 + fontMetrics().horizontalAdvance('9') * QString::number(qMax(qMax(1, blockCount()), m_gutter->property("sourceLineMaximum").toInt())).size();
}

void SourceEditor::updateGutter()
{
    setViewportMargins(gutterWidth(), 0, 0, 0);
    const QRect area = contentsRect();
    m_gutter->setGeometry(area.left(), area.top(), gutterWidth(), area.height());
    m_gutter->update();
}

void SourceEditor::resizeEvent(QResizeEvent *event)
{
    QPlainTextEdit::resizeEvent(event);
    updateGutter();
}

bool SourceEditor::eventFilter(QObject *object, QEvent *event)
{
    if (object != m_gutter || event->type() != QEvent::Paint)
        return QPlainTextEdit::eventFilter(object, event);
    QPainter painter(m_gutter);
    painter.fillRect(m_gutter->rect(), palette().base());
    painter.setPen(palette().color(QPalette::PlaceholderText));
    const auto mapping = m_gutter->property("sourceLineMapping").value<QVector<int>>();
    auto block = firstVisibleBlock();
    int top = qRound(blockBoundingGeometry(block).translated(contentOffset()).top());
    const int bottom = static_cast<QPaintEvent *>(event)->rect().bottom();
    while (block.isValid() && top <= bottom) {
        const int height = qRound(blockBoundingRect(block).height());
        const int line = mapping.isEmpty() ? block.blockNumber() + 1 : mapping.value(block.blockNumber());
        if (block.isVisible() && line > 0)
            painter.drawText(0, top, m_gutter->width() - 8, fontMetrics().height(), Qt::AlignRight, QString::number(line));
        top += height;
        block = block.next();
    }
    return true;
}

void SourceEditor::setFileName(const QString &path)
{
    auto *highlighter = document()->findChild<SourceHighlighter *>();
    if (!highlighter)
        highlighter = new SourceHighlighter(document());
    ensurePolished();
    highlighter->updateTheme(usesDarkEditorTheme(palette()));
    highlighter->setFileName(path);
}

QString SourceEditor::languageName() const
{
    const auto *highlighter = document()->findChild<SourceHighlighter *>();
    return highlighter ? highlighter->languageName() : tr("Plain text");
}

void SourceEditor::highlightCurrentLine()
{
    QTextEdit::ExtraSelection line;
    QColor color = palette().color(QPalette::Highlight);
    color.setAlpha(24);
    line.format.setBackground(color);
    line.format.setProperty(QTextFormat::FullWidthSelection, true);
    line.cursor = textCursor();
    line.cursor.clearSelection();
    QList<QTextEdit::ExtraSelection> selections{line};
    for (const auto &value : document()->property("pipeLangDiagnostics").toList()) {
        const auto diagnostic = value.toMap();
        const int start = diagnostic.value("start").toInt();
        const int end = diagnostic.value("end").toInt();
        if (start < 0 || end < start || end >= document()->characterCount())
            continue;

        QTextEdit::ExtraSelection selection;
        selection.cursor = QTextCursor(document());
        selection.cursor.setPosition(start);
        selection.cursor.setPosition(end, QTextCursor::KeepAnchor);
        if (start == end && end < document()->characterCount() - 1)
            selection.cursor.movePosition(QTextCursor::NextCharacter, QTextCursor::KeepAnchor);
        selection.format.setUnderlineStyle(QTextCharFormat::WaveUnderline);
        selection.format.setUnderlineColor(diagnostic.value("severity").toInt() == 1
            ? QColor(Qt::red) : QColor(180, 120, 0));
        selection.format.setToolTip(diagnostic.value("message").toString());
        selections.append(selection);
    }
    setExtraSelections(selections);
}

void SourceEditor::changeEvent(QEvent *event)
{
    QPlainTextEdit::changeEvent(event);
    if (event->type() == QEvent::PaletteChange) {
        if (auto *highlighter = document()->findChild<SourceHighlighter *>())
            highlighter->updateTheme(usesDarkEditorTheme(palette()));
        highlightCurrentLine();
        m_gutter->update();
    }
    if (event->type() == QEvent::FontChange) {
        setTabStopDistance(fontMetrics().horizontalAdvance(' ') * 4);
        updateGutter();
    }
}

void SourceEditor::keyPressEvent(QKeyEvent *event)
{
    if (event->modifiers() == Qt::ControlModifier && (event->key() == Qt::Key_Plus
        || event->key() == Qt::Key_Equal || event->key() == Qt::Key_Minus || event->key() == Qt::Key_0)) {
        auto nextFont = font();
        const int delta = event->key() == Qt::Key_Minus ? -1 : 1;
        nextFont.setPointSize(event->key() == Qt::Key_0 ? 11 : qBound(8, font().pointSize() + delta, 24));
        setFont(nextFont);
        return;
    }
    if (!isReadOnly() && (event->key() == Qt::Key_Backtab
        || (event->key() == Qt::Key_Tab && textCursor().hasSelection()))) {
        const bool outdent = event->key() == Qt::Key_Backtab;
        auto selection = textCursor();
        const int start = selection.selectionStart();
        int end = selection.selectionEnd();
        if (end > start && document()->findBlock(end).position() == end)
            --end;
        auto first = document()->findBlock(start);
        const int lastBlock = document()->findBlock(end).blockNumber();
        auto edit = QTextCursor(document());
        edit.beginEditBlock();
        for (auto block = first; block.isValid() && block.blockNumber() <= lastBlock; block = block.next()) {
            edit.setPosition(block.position());
            if (outdent) {
                const QString text = block.text();
                int remove = 0;
                if (text.startsWith('\t')) {
                    remove = 1;
                } else {
                    while (remove < qMin(4, text.size()) && text[remove] == ' ')
                        ++remove;
                }
                edit.setPosition(block.position() + remove, QTextCursor::KeepAnchor);
                edit.removeSelectedText();
            } else {
                edit.insertText("    ");
            }
        }
        edit.endEditBlock();
        setTextCursor(selection);
        return;
    }
    if (!isReadOnly() && event->key() == Qt::Key_Tab && event->modifiers() == Qt::NoModifier && !textCursor().hasSelection()) {
        // Preserve tab-indented files; otherwise use spaces at four-column stops.
        auto cursor = textCursor();
        const QString line = cursor.block().text();
        cursor.insertText(line.startsWith('\t') ? QString("\t") : QString(4 - cursor.positionInBlock() % 4, ' '));
        return;
    }
    if (!isReadOnly() && (event->key() == Qt::Key_Return || event->key() == Qt::Key_Enter)
        && event->modifiers() == Qt::NoModifier && !textCursor().hasSelection()) {
        auto cursor = textCursor();
        const QString before = cursor.block().text().left(cursor.positionInBlock());
        int spaces = 0;
        while (spaces < before.size() && (before[spaces] == ' ' || before[spaces] == '\t'))
            ++spaces;
        cursor.beginEditBlock();
        cursor.insertText("\n" + before.left(spaces));
        cursor.endEditBlock();
        setTextCursor(cursor);
        return;
    }
    QPlainTextEdit::keyPressEvent(event);
}

void SourceEditor::refreshLanguageDiagnostics()
{
    highlightCurrentLine();
}
