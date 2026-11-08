#include "GitDiffWidget.h"
#include "SourceEditor.h"

#include <QComboBox>
#include <QEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QRegularExpression>
#include <QScrollBar>
#include <QSignalBlocker>
#include <QShortcut>
#include <QSplitter>
#include <QTextBlock>
#include <QVBoxLayout>
#include <functional>
#include <limits>

namespace {
// Match the CLI preview's 2 MiB contract, with an additional native layout
// budget. A small-line patch must not allocate hundreds of thousands of blocks.
constexpr int maximumPatchBytes = 2 * 1024 * 1024;
constexpr int maximumDisplayRows = 30000;
constexpr int maximumFiles = 512;
constexpr int maximumMessageCharacters = 16000;

// SourceEditor owns current-line highlighting. Restore diff selections after
// its palette handler and cursor signal, without changing its published ABI.
class DiffSourceEditor final : public SourceEditor {
public:
    explicit DiffSourceEditor(QWidget *parent) : SourceEditor(parent) {}
    std::function<void()> refresh;
    std::function<void()> fontChanged;
protected:
    void changeEvent(QEvent *event) override
    {
        SourceEditor::changeEvent(event);
        if (fontChanged && event->type() == QEvent::FontChange)
            fontChanged();
        if (refresh && (event->type() == QEvent::PaletteChange || event->type() == QEvent::StyleChange))
            refresh();
    }
};

QString patchPath(QString path, bool stripPrefix = true)
{
    // Git quotes paths containing tabs, quotes or non-ASCII bytes with C escapes.
    if (path.startsWith('"') && path.endsWith('"')) {
        const auto encoded = path.mid(1, path.size() - 2).toUtf8();
        QByteArray decoded;
        for (int index = 0; index < encoded.size(); ++index) {
            char character = encoded[index];
            if (character == '\\' && index + 1 < encoded.size()) {
                character = encoded[++index];
                if (character >= '0' && character <= '7') {
                    int value = character - '0';
                    int digits = 1;
                    while (digits < 3 && index + 1 < encoded.size()
                        && encoded[index + 1] >= '0' && encoded[index + 1] <= '7') {
                        value = value * 8 + encoded[++index] - '0';
                        ++digits;
                    }
                    character = static_cast<char>(value);
                } else if (character == 't') {
                    character = '\t';
                } else if (character == 'n') {
                    character = '\n';
                } else if (character == 'r') {
                    character = '\r';
                }
            }
            decoded.append(character);
        }
        path = QString::fromUtf8(decoded);
    }
    if (stripPrefix && (path.startsWith("a/") || path.startsWith("b/")))
        path.remove(0, 2);
    return path;
}

void appendNotice(QString &text, const QString &notice)
{
    if (notice.isEmpty())
        return;
    if (!text.isEmpty())
        text += '\n';
    text += notice.left(4096);
}
}

GitDiffWidget::GitDiffWidget(QWidget *parent) : QWidget(parent)
{
    setLayoutDirection(Qt::LeftToRight);
    setObjectName("gitDiffWidget");
    setAttribute(Qt::WA_StyledBackground, true);
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(8);

    auto *messageHeading = new QLabel(tr("Commit message"), this);
    messageHeading->setObjectName("gitDiffCommitMessageHeading");
    layout->addWidget(messageHeading);
    m_commitMessage = new QPlainTextEdit(this);
    m_commitMessage->setObjectName("gitDiffCommitMessage");
    m_commitMessage->setReadOnly(true);
    m_commitMessage->setMaximumHeight(120);
    m_commitMessage->setMinimumHeight(64);
    m_commitMessage->setAccessibleName(tr("Commit message"));
    messageHeading->setBuddy(m_commitMessage);
    layout->addWidget(m_commitMessage);

    auto *fileSelector = new QWidget(this);
    fileSelector->setObjectName("gitDiffFileSelector");
    auto *fileLayout = new QVBoxLayout(fileSelector);
    fileLayout->setContentsMargins(10, 8, 10, 8);
    fileLayout->setSpacing(4);
    auto *fileHeading = new QLabel(fileSelector);
    fileHeading->setObjectName("gitDiffFileHeading");
    fileLayout->addWidget(fileHeading);
    m_filePicker = new QComboBox(fileSelector);
    m_filePicker->setObjectName("gitDiffFilePicker");
    m_filePicker->setSizeAdjustPolicy(QComboBox::AdjustToMinimumContentsLengthWithIcon);
    m_filePicker->setMinimumContentsLength(8);
    m_filePicker->setAccessibleName(tr("Changed files"));
    fileHeading->setBuddy(m_filePicker);
    fileLayout->addWidget(m_filePicker);
    layout->addWidget(fileSelector);
    m_counts = new QLabel(this);
    m_counts->setObjectName("gitDiffCounts");
    auto *navigation = new QHBoxLayout;
    navigation->addWidget(m_counts);
    navigation->addStretch();
    m_previous = new QPushButton(tr("Previous"), this);
    m_previous->setObjectName("gitDiffPreviousChange");
    m_previous->setAutoDefault(false);
    m_previous->setToolTip(tr("Previous change (Alt+Up)"));
    m_previous->setAccessibleName(tr("Previous change"));
    auto *previousShortcut = new QShortcut(QKeySequence(Qt::ALT | Qt::Key_Up), this);
    previousShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(previousShortcut, &QShortcut::activated, m_previous, &QPushButton::click);
    navigation->addWidget(m_previous);
    m_position = new QLabel(this);
    m_position->setObjectName("gitDiffPosition");
    navigation->addWidget(m_position);
    m_next = new QPushButton(tr("Next"), this);
    m_next->setObjectName("gitDiffNextChange");
    m_next->setAutoDefault(false);
    m_next->setToolTip(tr("Next change (Alt+Down)"));
    m_next->setAccessibleName(tr("Next change"));
    auto *nextShortcut = new QShortcut(QKeySequence(Qt::ALT | Qt::Key_Down), this);
    nextShortcut->setContext(Qt::WidgetWithChildrenShortcut);
    connect(nextShortcut, &QShortcut::activated, m_next, &QPushButton::click);
    navigation->addWidget(m_next);
    layout->addLayout(navigation);

    m_notice = new QLabel(this);
    m_notice->setObjectName("gitDiffNotice");
    m_notice->setTextFormat(Qt::PlainText);
    m_notice->setWordWrap(true);
    m_notice->setTextInteractionFlags(Qt::TextSelectableByMouse);
    layout->addWidget(m_notice);
    m_empty = new QLabel(this);
    m_empty->setObjectName("gitDiffEmpty");
    m_empty->setTextFormat(Qt::PlainText);
    m_empty->setWordWrap(true);
    m_empty->setAlignment(Qt::AlignCenter);
    layout->addWidget(m_empty, 1);

    m_splitter = new QSplitter(Qt::Horizontal, this);
    m_splitter->setObjectName("gitDiffSplitter");
    m_splitter->setChildrenCollapsible(false);
    const auto createPane = [this](const QString &name, QLabel **label, SourceEditor **editor) {
        auto *pane = new QWidget(m_splitter);
        pane->setObjectName("gitDiffPane");
        auto *paneLayout = new QVBoxLayout(pane);
        paneLayout->setContentsMargins(0, 0, 0, 0);
        paneLayout->setSpacing(0);
        *label = new QLabel(pane);
        (*label)->setObjectName("gitDiffPaneLabel");
        (*label)->setTextFormat(Qt::PlainText);
        (*label)->setContentsMargins(12, 8, 12, 8);
        (*label)->setTextInteractionFlags(Qt::TextSelectableByMouse);
        paneLayout->addWidget(*label);
        auto *source = new DiffSourceEditor(pane);
        source->setObjectName(name);
        source->setReadOnly(true);
        source->setUndoRedoEnabled(false);
        source->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOn);
        source->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOn);
        source->refresh = [this]() { refreshHighlights(); };
        source->fontChanged = [this, source]() {
            if (!m_before || !m_after)
                return;
            auto *other = source == m_before ? m_after : m_before;
            if (other->font() != source->font())
                other->setFont(source->font());
        };
        connect(source, &QPlainTextEdit::cursorPositionChanged, this, &GitDiffWidget::refreshHighlights);
        *editor = source;
        paneLayout->addWidget(source, 1);
        return pane;
    };
    m_splitter->addWidget(createPane("gitDiffBeforeEditor", &m_beforeLabel, &m_before));
    m_splitter->addWidget(createPane("gitDiffAfterEditor", &m_afterLabel, &m_after));
    m_splitter->setStretchFactor(0, 1);
    m_splitter->setStretchFactor(1, 1);
    layout->addWidget(m_splitter, 1);

    for (auto *source : {m_before, m_after}) {
        auto *other = source == m_before ? m_after : m_before;
        connect(source->verticalScrollBar(), &QScrollBar::valueChanged, this, [other](int value) {
            const QSignalBlocker blocker(other->verticalScrollBar());
            other->verticalScrollBar()->setValue(value);
        });
        connect(source->horizontalScrollBar(), &QScrollBar::valueChanged, this, [other](int value) {
            const QSignalBlocker blocker(other->horizontalScrollBar());
            other->horizontalScrollBar()->setValue(value);
        });
    }
    connect(m_filePicker, qOverload<int>(&QComboBox::currentIndexChanged), this, &GitDiffWidget::displayFile);
    connect(m_previous, &QPushButton::clicked, this, [this]() { navigateChange(-1); });
    connect(m_next, &QPushButton::clicked, this, [this]() { navigateChange(1); });
    clear();
}

void GitDiffWidget::clear()
{
    m_commitReview = false;
    m_files.clear();
    m_rows.clear();
    m_changes.clear();
    m_noticeText.clear();
    m_beforeText = tr("Before");
    m_afterText = tr("After");
    m_commitMessage->clear();
    m_commitMessage->hide();
    findChild<QLabel *>("gitDiffCommitMessageHeading")->hide();
    const QSignalBlocker blocker(m_filePicker);
    m_filePicker->clear();
    displayFile(-1);
}

void GitDiffWidget::setReview(const QJsonObject &review)
{
    loadPatch(review, false);
}

void GitDiffWidget::setCommitReview(const QJsonObject &review)
{
    loadPatch(review, true);
}

void GitDiffWidget::loadPatch(const QJsonObject &review, bool commit)
{
    clear();
    m_commitReview = commit;
    m_noticeText = review.value("notice").toString().left(4096);
    m_beforeText = review.value("before_label").toString(tr("Before"));
    m_afterText = review.value("after_label").toString(tr("After"));
    if (commit) {
        QString message = review.value("body").toString();
        const auto metadata = review.value("commit").toObject();
        QString title = metadata.value("subject").toString();
        if (title.isEmpty())
            title = metadata.value("summary").toString();
        if (message.isEmpty())
            message = title;
        if (message.size() > maximumMessageCharacters)
            message = message.left(maximumMessageCharacters) + tr("\n… Commit message preview limited.");
        m_commitMessage->setPlainText(message);
        m_commitMessage->setVisible(!message.isEmpty());
        findChild<QLabel *>("gitDiffCommitMessageHeading")->setVisible(!message.isEmpty());
    }
    if (review.value("limited").toBool())
        appendNotice(m_noticeText, tr("The CLI limited this preview. Additional changes may be omitted."));
    if (review.value("conflict").toBool())
        appendNotice(m_noticeText, tr("This file has unresolved conflicts. This preview does not resolve them."));

    const QString patch = review.value("patch").toString();
    if (patch.size() > maximumPatchBytes || patch.toUtf8().size() > maximumPatchBytes) {
        appendNotice(m_noticeText, tr("Diff preview exceeds the 2 MiB display limit. Select a smaller change to review."));
        displayFile(-1);
        return;
    }
    File file;
    file.path = review.value("path").toString();
    file.beforePath = file.path;
    file.binary = review.value("binary").toBool();
    bool hasFile = !commit;
    int totalRows = 0;
    bool exhausted = false;
    bool malformed = false;
    int beforeLine = 0;
    int afterLine = 0;
    int beforeRemaining = 0;
    int afterRemaining = 0;
    bool inHunk = false;
    QVector<Row> removed;
    QVector<Row> added;
    const auto appendRow = [&](const Row &row) {
        if (totalRows >= maximumDisplayRows) {
            exhausted = true;
            return;
        }
        file.rows.append(row);
        ++totalRows;
    };
    const auto flushChanges = [&]() {
        const int length = qMax(removed.size(), added.size());
        for (int index = 0; index < length && !exhausted; ++index) {
            Row row;
            if (index < removed.size()) {
                row.before = removed[index].before;
                row.beforeLine = removed[index].beforeLine;
                row.removed = true;
            }
            if (index < added.size()) {
                row.after = added[index].after;
                row.afterLine = added[index].afterLine;
                row.added = true;
            }
            appendRow(row);
        }
        removed.clear();
        added.clear();
    };
    const auto finishFile = [&]() {
        flushChanges();
        if (hasFile && m_files.size() < maximumFiles)
            m_files.append(file);
        else if (hasFile)
            exhausted = true;
    };
    const QRegularExpression hunkExpression("^@@ -([0-9]+)(?:,([0-9]+))? \\+([0-9]+)(?:,([0-9]+))? @@");
    const auto lines = QStringView(patch).split('\n');
    for (auto lineView : lines) {
        if (exhausted)
            break;
        if (lineView.endsWith('\r'))
            lineView.chop(1);
        const QString line = lineView.toString();
        // Consume hunk content before interpreting file headers: an actual source
        // line may start with +++ or ---. Counts distinguish those from headers.
        if (inHunk && (beforeRemaining > 0 || afterRemaining > 0)) {
            const QChar prefix = line.isEmpty() ? QChar() : line[0];
            if (prefix == '-' && beforeRemaining > 0) {
                removed.append({line.mid(1), {}, beforeLine++, 0, true, false, false});
                --beforeRemaining;
                if (removed.size() > maximumDisplayRows - totalRows) {
                    flushChanges();
                    exhausted = true;
                }
                continue;
            }
            if (prefix == '+' && afterRemaining > 0) {
                added.append({{}, line.mid(1), 0, afterLine++, false, true, false});
                --afterRemaining;
                if (added.size() > maximumDisplayRows - totalRows) {
                    flushChanges();
                    exhausted = true;
                }
                continue;
            }
            if (prefix == ' ' && beforeRemaining > 0 && afterRemaining > 0) {
                flushChanges();
                appendRow({line.mid(1), line.mid(1), beforeLine++, afterLine++, false, false, false});
                --beforeRemaining;
                --afterRemaining;
                continue;
            }
            if (!line.startsWith("\\ No newline at end of file")) {
                malformed = true;
                inHunk = false;
                flushChanges();
            }
        }
        if (line.startsWith("\\ No newline at end of file")) {
            // Keep this fact visible outside source text and line numbering.
            appendNotice(file.notice, tr("No newline at end of file in one or both versions."));
            continue;
        }
        if (line.startsWith("diff --git ")) {
            if (hasFile && (!file.rows.isEmpty() || commit))
                finishFile();
            if (exhausted)
                break;
            if (commit || !file.rows.isEmpty())
                file = File{};
            hasFile = true;
            inHunk = false;
            // The ---/+++ headers below are authoritative, including quoted paths.
            // Rename-only/binary patches may have no such headers.
            int separator = line.indexOf(" b/", 11);
            if (line.mid(11).startsWith('"')) {
                bool escaped = false;
                for (int index = 12; index < line.size(); ++index) {
                    if (line[index] == '"' && !escaped) {
                        separator = index + 1;
                        break;
                    }
                    if (line[index] == '\\' && !escaped)
                        escaped = true;
                    else
                        escaped = false;
                }
            }
            if (separator >= 0) {
                file.beforePath = patchPath(line.mid(11, separator - 11));
                file.path = patchPath(line.mid(separator + 1));
            }
            continue;
        }
        if (line.startsWith("--- ")) {
            file.beforePath = patchPath(line.mid(4));
            hasFile = true;
            continue;
        }
        if (line.startsWith("+++ ")) {
            const auto path = patchPath(line.mid(4));
            file.path = path == "/dev/null" ? file.beforePath : path;
            hasFile = true;
            continue;
        }
        if (line.startsWith("rename from ")) {
            file.beforePath = patchPath(line.mid(12), false);
            appendNotice(file.notice, tr("Renamed from %1").arg(file.beforePath));
            continue;
        }
        if (line.startsWith("rename to ")) {
            file.path = patchPath(line.mid(10), false);
            continue;
        }
        if (line.startsWith("old mode ")) {
            appendNotice(file.notice, tr("Previous file mode: %1").arg(line.mid(9)));
            continue;
        }
        if (line.startsWith("new mode ")) {
            appendNotice(file.notice, tr("New file mode: %1").arg(line.mid(9)));
            continue;
        }
        if (line.startsWith("Binary files ") || line == "GIT binary patch") {
            file.binary = true;
            continue;
        }
        const auto match = hunkExpression.match(line);
        if (!match.hasMatch())
            continue;
        if (inHunk && (beforeRemaining != 0 || afterRemaining != 0))
            malformed = true;
        flushChanges();
        bool beforeValid = false;
        bool afterValid = false;
        const qlonglong beforeStart = match.captured(1).toLongLong(&beforeValid);
        const qlonglong afterStart = match.captured(3).toLongLong(&afterValid);
        bool beforeCountValid = true;
        bool afterCountValid = true;
        const qlonglong beforeCount = match.captured(2).isEmpty() ? 1 : match.captured(2).toLongLong(&beforeCountValid);
        const qlonglong afterCount = match.captured(4).isEmpty() ? 1 : match.captured(4).toLongLong(&afterCountValid);
        if (!beforeValid || !afterValid || !beforeCountValid || !afterCountValid
            || beforeCount < 0 || afterCount < 0
            || beforeStart > std::numeric_limits<int>::max() || afterStart > std::numeric_limits<int>::max()
            || beforeCount > std::numeric_limits<int>::max() || afterCount > std::numeric_limits<int>::max()
            || beforeStart + beforeCount > std::numeric_limits<int>::max()
            || afterStart + afterCount > std::numeric_limits<int>::max()) {
            malformed = true;
            break;
        }
        beforeLine = static_cast<int>(beforeStart);
        afterLine = static_cast<int>(afterStart);
        beforeRemaining = static_cast<int>(beforeCount);
        afterRemaining = static_cast<int>(afterCount);
        if (!file.rows.isEmpty() || beforeLine > 1 || afterLine > 1)
            appendRow({QString(), QString(), 0, 0, false, false, true});
        hasFile = true;
        inHunk = true;
    }
    if (inHunk && (beforeRemaining != 0 || afterRemaining != 0))
        malformed = true;
    finishFile();
    if (exhausted)
        appendNotice(m_noticeText, tr("Diff display limited to 30,000 aligned rows and 512 files. Additional changes are omitted."));
    if (malformed)
        appendNotice(m_noticeText, tr("The patch contains incomplete or unsupported hunks. Only readable excerpts are shown."));
    const QSignalBlocker blocker(m_filePicker);
    for (const auto &item : m_files)
        m_filePicker->addItem(item.path.isEmpty() ? tr("Changed file") : item.path);
    displayFile(m_files.isEmpty() ? -1 : 0);
    if (commit && m_files.isEmpty())
        m_empty->setText(tr("No textual changes to compare\nThis commit may contain only merge information or file metadata."));
}

void GitDiffWidget::displayFile(int index)
{
    m_loading = true;
    m_rows.clear();
    m_changes.clear();
    m_changeIndex = -1;
    QString beforePath;
    QString afterPath;
    QString notice = m_noticeText;
    bool binary = false;
    if (index >= 0 && index < m_files.size()) {
        const auto &file = m_files[index];
        m_rows = file.rows;
        beforePath = file.beforePath;
        afterPath = file.path;
        binary = file.binary;
        appendNotice(notice, file.notice);
    }
    m_notice->setText(notice);
    m_notice->setVisible(!notice.isEmpty());
    m_filePicker->parentWidget()->setVisible(!property("gitHideFileSelector").toBool()
        && !m_files.isEmpty() && (m_commitReview || m_files.size() > 1));
    const QString fileHeading = tr("Changed files (%1)").arg(m_files.size());
    findChild<QLabel *>("gitDiffFileHeading")->setText(fileHeading);
    m_filePicker->setAccessibleName(fileHeading);
    m_filePicker->setAccessibleDescription(tr("Select a file from this commit to review its changes."));
    m_filePicker->setToolTip(tr("Select a changed file to review\n%1").arg(afterPath));
    m_beforeLabel->setText(m_beforeText);
    m_afterLabel->setText(m_afterText);
    m_beforeLabel->setToolTip(beforePath);
    m_afterLabel->setToolTip(afterPath);
    m_before->setAccessibleName(tr("Before version"));
    m_before->setToolTip(tr("Changed regions with surrounding context. Gaps in line numbers indicate omitted source."));
    m_after->setToolTip(m_before->toolTip());
    m_after->setAccessibleName(tr("After version"));
    QStringList before;
    QStringList after;
    QVector<int> beforeMapping;
    QVector<int> afterMapping;
    int removed = 0;
    int added = 0;
    bool previousChanged = false;
    for (int rowIndex = 0; rowIndex < m_rows.size(); ++rowIndex) {
        const auto &row = m_rows[rowIndex];
        before.append(row.before);
        after.append(row.after);
        beforeMapping.append(row.beforeLine);
        afterMapping.append(row.afterLine);
        removed += row.removed;
        added += row.added;
        const bool changed = row.removed || row.added;
        if (changed && !previousChanged)
            m_changes.append(rowIndex);
        previousChanged = changed;
    }
    m_before->loadFileText(before.join('\n'), beforePath);
    m_after->loadFileText(after.join('\n'), afterPath);
    m_before->setLineMapping(beforeMapping);
    m_after->setLineMapping(afterMapping);
    m_counts->setText(tr("%1 added  ·  %2 removed").arg(added).arg(removed));
    m_counts->setAccessibleName(tr("%1 added lines, %2 removed lines").arg(added).arg(removed));
    const bool hasRows = !m_rows.isEmpty();
    m_splitter->setVisible(hasRows);
    m_empty->setVisible(!hasRows);
    if (binary) {
        m_empty->setText(tr("Binary file changed\nA source comparison is unavailable for binary content."));
    } else if (index >= 0) {
        m_empty->setText(tr("No textual changes to compare\nThis change may contain only a rename, file metadata, or merge information."));
    } else {
        m_empty->setText(tr("Select a change to compare its before and after versions."));
    }
    m_loading = false;
    refreshHighlights();
    updateNavigation();
}

void GitDiffWidget::refreshHighlights()
{
    if (m_loading || !m_before || !m_after)
        return;
    for (auto *editor : {m_before, m_after}) {
        const bool before = editor == m_before;
        const bool dark = editor->palette().color(QPalette::Text).lightness() > 128;
        const QColor removed = dark ? QColor(107, 41, 52) : QColor(255, 220, 224);
        const QColor added = dark ? QColor(27, 79, 61) : QColor(213, 245, 223);
        QColor separator = editor->palette().color(QPalette::AlternateBase);
        QList<QTextEdit::ExtraSelection> selections;
        for (int index = 0; index < m_rows.size(); ++index) {
            const auto &row = m_rows[index];
            const bool changed = before ? row.removed : row.added;
            if (!changed && !row.separator)
                continue;
            QTextEdit::ExtraSelection selection;
            selection.cursor = QTextCursor(editor->document()->findBlockByNumber(index));
            selection.format.setProperty(QTextFormat::FullWidthSelection, true);
            selection.format.setBackground(row.separator ? separator : (before ? removed : added));
            if (row.separator)
                selection.format.setForeground(editor->palette().color(QPalette::PlaceholderText));
            selections.append(selection);
            if (!row.removed || !row.added)
                continue;
            // A linear common-prefix/suffix comparison emphasizes the edited
            // span without an expensive character-level sequence alignment.
            int prefix = 0;
            const int commonLength = qMin(row.before.size(), row.after.size());
            while (prefix < commonLength && row.before[prefix] == row.after[prefix])
                ++prefix;
            if (prefix > 0 && prefix < commonLength && row.before[prefix].isLowSurrogate())
                --prefix;
            int suffix = 0;
            while (suffix < commonLength - prefix
                && row.before[row.before.size() - suffix - 1] == row.after[row.after.size() - suffix - 1])
                ++suffix;
            if (suffix > 0 && row.before[row.before.size() - suffix].isLowSurrogate())
                --suffix;
            const QString &text = before ? row.before : row.after;
            const int length = text.size() - prefix - suffix;
            if (length <= 0)
                continue;
            QTextEdit::ExtraSelection span;
            span.cursor = QTextCursor(editor->document()->findBlockByNumber(index));
            span.cursor.setPosition(span.cursor.position() + prefix);
            span.cursor.setPosition(span.cursor.position() + length, QTextCursor::KeepAnchor);
            QColor emphasis;
            if (before)
                emphasis = dark ? QColor(146, 56, 71) : QColor(255, 177, 187);
            else
                emphasis = dark ? QColor(35, 111, 78) : QColor(158, 226, 179);
            span.format.setBackground(emphasis);
            selections.append(span);
        }
        editor->setExtraSelections(selections);
    }
}

void GitDiffWidget::navigateChange(int direction)
{
    if (m_changes.isEmpty())
        return;
    if (m_changeIndex < 0)
        m_changeIndex = direction > 0 ? 0 : m_changes.size() - 1;
    else
        m_changeIndex = (m_changeIndex + direction + m_changes.size()) % m_changes.size();
    const int row = m_changes[m_changeIndex];
    for (auto *editor : {m_before, m_after}) {
        editor->setTextCursor(QTextCursor(editor->document()->findBlockByNumber(row)));
        editor->centerCursor();
    }
    updateNavigation();
}

void GitDiffWidget::updateNavigation()
{
    m_previous->setEnabled(!m_changes.isEmpty());
    m_next->setEnabled(!m_changes.isEmpty());
    if (m_changes.isEmpty())
        m_position->setText(tr("No changes"));
    else if (m_changeIndex < 0)
        m_position->setText(m_changes.size() == 1 ? tr("1 change") : tr("%1 changes").arg(m_changes.size()));
    else
        m_position->setText(tr("%1 / %2").arg(m_changeIndex + 1).arg(m_changes.size()));
}

QStringList GitDiffWidget::changedFiles() const
{
    QStringList paths;
    for (const auto &file : m_files)
        paths.append(file.path);
    return paths;
}

void GitDiffWidget::selectFile(int index)
{
    if (index >= 0 && index < m_filePicker->count())
        m_filePicker->setCurrentIndex(index);
}

void GitDiffWidget::setFileSelectorVisible(bool visible)
{
    setProperty("gitHideFileSelector", !visible);
    m_filePicker->parentWidget()->setVisible(visible && !m_files.isEmpty()
        && (m_commitReview || m_files.size() > 1));
}
