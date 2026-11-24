#include "GitDiffWidget.h"
#include "SourceEditor.h"

#include <QApplication>
#include <QComboBox>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QScrollBar>
#include <QTextBlock>
#include <QTest>

class GitDiffWidgetTest : public QObject {
    Q_OBJECT
private slots:
    void alignsRunsAndPreservesSource();
    void separatesHunksAndKeepsSourceNumbers();
    void showsBinaryConflictAndLimitedStates();
    void selectsCommitFilesAndSeparatesMessage();
    void boundsLayoutAndPatchInput();
    void restoresHighlightsAfterCursorAndThemeChanges();
    void synchronizesScrolling();
    void handlesQuotedPathsAndMetadata();
    void boundsFileCountAndRejectsInvalidRanges();
    void emphasizesEditedSpans();
};

void GitDiffWidgetTest::alignsRunsAndPreservesSource()
{
    GitDiffWidget widget;
    widget.setReview({{"path", "sample.cpp"}, {"before_label", "HEAD"}, {"after_label", "Working folder"},
        {"patch", "diff --git a/sample.cpp b/sample.cpp\r\n--- a/sample.cpp\r\n+++ b/sample.cpp\r\n"
            "@@ -10,4 +10,5 @@\r\n context\r\n---literal\r\n-old\r\n+++literal\r\n+new\r\n+extra\r\n tail\r\n\\ No newline at end of file\r\n"}});
    auto *before = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QVERIFY(before && after);
    QCOMPARE(before->document()->findBlockByNumber(2).text(), QString("--literal"));
    QCOMPARE(after->document()->findBlockByNumber(2).text(), QString("++literal"));
    QCOMPARE(before->document()->findBlockByNumber(4).text(), QString());
    QCOMPARE(after->document()->findBlockByNumber(4).text(), QString("extra"));
    QCOMPARE(before->blockCount(), after->blockCount());
    QCOMPARE(after->languageName(), QString("C++"));
    QCOMPARE(widget.findChild<QLabel *>("gitDiffCounts")->text(), QString("3 added  ·  2 removed"));
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("No newline"));
    QVERIFY(before->isReadOnly() && after->isReadOnly());
    QVERIFY(!before->toPlainText().contains("diff --git"));
    QCOMPARE(before->extraSelections().size(), 5);
    QCOMPARE(after->extraSelections().size(), 6);
}

void GitDiffWidgetTest::separatesHunksAndKeepsSourceNumbers()
{
    GitDiffWidget widget;
    widget.setReview({{"path", "a.py"}, {"patch", "@@ -3 +3 @@\n-first\n+second\n@@ -90,0 +91,2 @@\n+one\n+two\n"}});
    auto *before = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QVERIFY(before->document()->findBlockByNumber(2).text().isEmpty());
    QCOMPARE(after->document()->findBlockByNumber(3).text(), QString("one"));
    QVector<int> beforeMapping;
    QVector<int> afterMapping;
    for (auto *child : before->findChildren<QWidget *>()) {
        if (child->property("sourceLineMapping").isValid())
            beforeMapping = child->property("sourceLineMapping").value<QVector<int>>();
    }
    for (auto *child : after->findChildren<QWidget *>()) {
        if (child->property("sourceLineMapping").isValid())
            afterMapping = child->property("sourceLineMapping").value<QVector<int>>();
    }
    QCOMPARE(beforeMapping, QVector<int>({0, 3, 0, 0, 0}));
    QCOMPARE(afterMapping, QVector<int>({0, 3, 0, 91, 92}));
    auto *next = widget.findChild<QPushButton *>("gitDiffNextChange");
    next->click();
    QCOMPARE(after->textCursor().blockNumber(), 1);
    next->click();
    QCOMPARE(after->textCursor().blockNumber(), 3);
    QCOMPARE(widget.findChild<QLabel *>("gitDiffPosition")->text(), QString("2 / 2"));
    widget.findChild<QPushButton *>("gitDiffPreviousChange")->click();
    QCOMPARE(after->textCursor().blockNumber(), 1);
}

void GitDiffWidgetTest::showsBinaryConflictAndLimitedStates()
{
    GitDiffWidget widget;
    widget.setReview({{"path", "image.png"}, {"binary", true}, {"conflict", true}, {"limited", true},
        {"notice", "Fixture notice"}, {"patch", "Binary files a/image.png and b/image.png differ\n"}});
    QVERIFY(widget.findChild<QLabel *>("gitDiffEmpty")->text().contains("Binary file"));
    auto *notice = widget.findChild<QLabel *>("gitDiffNotice");
    QVERIFY(notice->text().contains("Fixture notice"));
    QVERIFY(notice->text().contains("limited"));
    QVERIFY(notice->text().contains("unresolved conflicts"));
    QVERIFY(!widget.findChild<QPushButton *>("gitDiffNextChange")->isEnabled());
    widget.setReview({{"path", "renamed.cpp"}, {"patch", "diff --git a/old.cpp b/renamed.cpp\n"
        "similarity index 100%\nrename from old.cpp\nrename to renamed.cpp\n"}});
    QVERIFY(notice->text().contains("Renamed from old.cpp"));
    QVERIFY(widget.findChild<QLabel *>("gitDiffEmpty")->text().contains("No textual changes"));
    widget.clear();
    QVERIFY(notice->text().isEmpty());
    QVERIFY(static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"))->toPlainText().isEmpty());
}

void GitDiffWidgetTest::selectsCommitFilesAndSeparatesMessage()
{
    GitDiffWidget widget;
    widget.setCommitReview({{"body", "A commit title\n\nDetailed explanation."}, {"patch",
        " a.cpp | 2 +-\n b.py | 2 +-\n 2 files changed\n\n"
        "diff --git a/a.cpp b/a.cpp\n--- a/a.cpp\n+++ b/a.cpp\n@@ -1 +1 @@\n-int a;\n+int b;\n"
        "diff --git a/b.py b/b.py\n--- a/b.py\n+++ b/b.py\n@@ -8 +8 @@\n-old = 1\n+new = 2\n"}});
    auto *files = widget.findChild<QComboBox *>("gitDiffFilePicker");
    QCOMPARE(files->count(), 2);
    QCOMPARE(files->itemText(0), QString("a.cpp"));
    QCOMPARE(files->itemText(1), QString("b.py"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QVERIFY(after->toPlainText().contains("int b;"));
    QVERIFY(!after->toPlainText().contains("commit title"));
    files->setCurrentIndex(1);
    QVERIFY(after->toPlainText().contains("new = 2"));
    QCOMPARE(after->languageName(), QString("Python"));
    QCOMPARE(widget.findChild<QPlainTextEdit *>("gitDiffCommitMessage")->toPlainText(),
        QString("A commit title\n\nDetailed explanation."));
    widget.setCommitReview({{"body", "Merge explanation"}, {"patch", ""}});
    QVERIFY(widget.findChild<QLabel *>("gitDiffEmpty")->text().contains("No textual changes"));
    QCOMPARE(widget.findChild<QPlainTextEdit *>("gitDiffCommitMessage")->toPlainText(), QString("Merge explanation"));
}

void GitDiffWidgetTest::boundsLayoutAndPatchInput()
{
    GitDiffWidget widget;
    widget.setReview({{"path", "large.txt"}, {"patch", "@@ -0,0 +1,31000 @@\n" + QString("+x\n").repeated(31000)}});
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QVERIFY(after->blockCount() <= 30000);
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("30,000"));
    widget.setReview({{"path", "huge.txt"}, {"patch", QString(2 * 1024 * 1024 + 1, 'x')}});
    QVERIFY(after->toPlainText().isEmpty());
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("2 MiB"));
    widget.setReview({{"path", "partial.txt"}, {"patch", "@@ -1,2 +1,2 @@\n-old\n+new\n"}});
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("incomplete"));
}

void GitDiffWidgetTest::restoresHighlightsAfterCursorAndThemeChanges()
{
    GitDiffWidget widget;
    widget.resize(1100, 600);
    widget.setReview({{"path", "sample.cpp"}, {"patch", "@@ -1 +1 @@\n-int old = 1;\n+int next = 2;\n"}});
    widget.show();
    QApplication::processEvents();
    auto *before = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QPalette light = after->palette();
    light.setColor(QPalette::Base, QColor("#ffffff"));
    light.setColor(QPalette::Text, QColor("#202020"));
    after->setPalette(light);
    before->setPalette(light);
    after->moveCursor(QTextCursor::End);
    QCOMPARE(after->extraSelections().size(), 2);
    const auto lightAdded = after->extraSelections().last().format.background().color();
    const auto lightRemoved = before->extraSelections().last().format.background().color();
    QVERIFY(lightAdded.green() > lightAdded.red());
    QVERIFY(lightRemoved.red() > lightRemoved.green());
    QPalette dark = light;
    dark.setColor(QPalette::Base, QColor("#182127"));
    dark.setColor(QPalette::Text, QColor("#eeeeee"));
    after->setPalette(dark);
    before->setPalette(dark);
    QApplication::processEvents();
    QCOMPARE(after->extraSelections().size(), 2);
    QVERIFY(after->extraSelections().last().format.background().color() != lightAdded);
    QVERIFY(before->extraSelections().last().format.background().color() != lightRemoved);
    QVERIFY(!widget.grab().isNull());
}

void GitDiffWidgetTest::synchronizesScrolling()
{
    GitDiffWidget widget;
    widget.resize(900, 400);
    const QString longLine(300, 'x');
    widget.setReview({{"path", "sample.txt"}, {"patch", "@@ -1,100 +1,100 @@\n"
        + QString(" %1\n").arg(longLine).repeated(100)}});
    widget.show();
    QApplication::processEvents();
    auto *before = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    before->verticalScrollBar()->setValue(40);
    QCOMPARE(after->verticalScrollBar()->value(), 40);
    after->horizontalScrollBar()->setValue(100);
    QCOMPARE(before->horizontalScrollBar()->value(), 100);
}

void GitDiffWidgetTest::handlesQuotedPathsAndMetadata()
{
    GitDiffWidget widget;
    widget.setCommitReview({{"body", "Binary update"}, {"patch",
        "diff --git \"a/file\\tname.png\" \"b/file\\tname.png\"\n"
        "Binary files differ\n"}});
    auto *files = widget.findChild<QComboBox *>("gitDiffFilePicker");
    QCOMPARE(files->count(), 1);
    QCOMPARE(files->itemText(0), QString("file\tname.png"));
    widget.setReview({{"path", "a/renamed.cpp"}, {"patch",
        "diff --git a/a/original.cpp b/a/renamed.cpp\n"
        "rename from a/original.cpp\nrename to a/renamed.cpp\nold mode 100644\nnew mode 100755\n"}});
    QCOMPARE(files->itemText(0), QString("a/renamed.cpp"));
    const auto notice = widget.findChild<QLabel *>("gitDiffNotice")->text();
    QVERIFY(notice.contains("Renamed from a/original.cpp"));
    QVERIFY(notice.contains("100755"));
}

void GitDiffWidgetTest::boundsFileCountAndRejectsInvalidRanges()
{
    GitDiffWidget widget;
    QString patch;
    for (int index = 0; index < 513; ++index)
        patch += QString("diff --git a/file%1.txt b/file%1.txt\nold mode 100644\nnew mode 100755\n").arg(index);
    widget.setCommitReview({{"body", "Many files"}, {"patch", patch}});
    QCOMPARE(widget.findChild<QComboBox *>("gitDiffFilePicker")->count(), 512);
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("512 files"));
    widget.setReview({{"path", "invalid.txt"}, {"patch", "@@ -999999999999999999999999 +1 @@\n-old\n+new\n"}});
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("incomplete"));
    QVERIFY(static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"))->toPlainText().isEmpty());
    widget.setReview({{"path", "invalid.txt"}, {"patch", "@@ -1,999999999999999999999999 +1 @@\n-old\n+new\n"}});
    QVERIFY(widget.findChild<QLabel *>("gitDiffNotice")->text().contains("incomplete"));
}

void GitDiffWidgetTest::emphasizesEditedSpans()
{
    GitDiffWidget widget;
    widget.setReview({{"path", "sample.cpp"}, {"patch", "@@ -1 +1 @@\n-const int value = 1;\n+const int value = 2;\n"}});
    auto *before = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffBeforeEditor"));
    auto *after = static_cast<SourceEditor *>(widget.findChild<QPlainTextEdit *>("gitDiffAfterEditor"));
    QCOMPARE(before->extraSelections().last().cursor.selectedText(), QString("1"));
    QCOMPARE(after->extraSelections().last().cursor.selectedText(), QString("2"));
    QVERIFY(!after->extraSelections().last().format.property(QTextFormat::FullWidthSelection).toBool());
    QVERIFY(after->extraSelections()[0].format.property(QTextFormat::FullWidthSelection).toBool());
}

QTEST_MAIN(GitDiffWidgetTest)
#include "GitDiffWidgetTest.moc"
