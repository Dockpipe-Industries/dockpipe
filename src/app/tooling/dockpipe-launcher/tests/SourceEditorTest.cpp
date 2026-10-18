#include "SourceEditor.h"
#include "SourceHighlighter.h"
#include "Theme.h"
#include <QApplication>
#include <QKeyEvent>
#include <QTextBlock>
#include <QTextLayout>
#include <cstdio>

namespace {
class HighlightCallObserver : public QObject {
public:
    bool delivered = false;
protected:
    bool eventFilter(QObject *, QEvent *event) override
    {
        if (event->type() == QEvent::MetaCall)
            delivered = true;
        return false;
    }
};
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    applyDockpipeLauncherTheme(app);
    SourceEditor editor;
    if (editor.document()->isModified())
        return 8; // Styling a new editor must never create an unsaved document.
    const QList<QPair<QString, QString>> samples{
        {"sample.c", "int main() { return 0; }"},
        {"sample.cpp", "class Sample { public: int value; };"},
        {"sample.cs", "public class Sample { string value = \"hello\"; }"},
        {"Sample.java", "public class Sample { int value = 1; }"},
        {"sample.go", "package main\nfunc main() { println(\"hello\") }"},
        {"sample.rs", "fn main() { let value = 1; }"},
        {"sample.py", "def sample():\n    return \"hello\""},
        {"sample.js", "const value = true; // comment"},
        {"sample.rb", "def sample\n  puts 'hello'\nend"},
        {"sample.pipe", "public Class Sample { string value = \"hello\"; }"},
        {"sample.R", "value <- function(x) { return(TRUE) }"},
        {"sample.sh", "#!/bin/sh\necho \"hello\""},
        {"sample.kt", "fun main() { val value = 1 }"},
        {"sample.php", "<?php echo \"hello\"; ?>"},
        {"config.yml", "name: example\nsteps:\n  - run: echo hello"},
        {"sample.ts", "const value: string = \"hello\";"},
        {"review.diff", "@@ -1 +1 @@\n-old\n+new"},
    };
    for (const auto &sample : samples) {
        editor.setFileName(sample.first);
        editor.setPlainText(sample.second);
        auto *highlighter = editor.document()->findChild<SourceHighlighter *>();
        highlighter->rehighlight();
        int ranges = 0;
        for (auto block = editor.document()->begin(); block.isValid(); block = block.next())
            ranges += block.layout()->formats().size();
        if (editor.languageName() == "Plain text" || ranges == 0) {
            std::fprintf(stderr, "No language highlighting for %s\n", qPrintable(sample.first));
            return 1;
        }
    }
    editor.setFileName("sample.cpp");
    editor.setPlainText("/* begin\ninside comment\n*/ int value = 1;");
    auto *highlighter = editor.document()->findChild<SourceHighlighter *>();
    highlighter->rehighlight();
    const auto comment = editor.document()->findBlockByNumber(1).layout()->formats();
    if (comment.isEmpty())
        return 2;
    const auto darkColor = comment.first().format.foreground().color();
    highlighter->updateTheme(false);
    const auto lightColor = editor.document()->findBlockByNumber(1).layout()->formats().first().format.foreground().color();
    if (darkColor == lightColor)
        return 3;
    editor.setStyleSheet("QPlainTextEdit { background: rgba(246, 251, 253, 0); color: #173e49; }");
    editor.setFileName("sample.cpp");
    if (editor.document()->findBlockByNumber(1).layout()->formats().first().format.foreground().color() != lightColor)
        return 9;
    editor.setStyleSheet("QPlainTextEdit { background: rgba(14, 27, 38, 0); color: #e1f8f4; }");
    editor.setFileName("sample.cpp");
    if (editor.document()->findBlockByNumber(1).layout()->formats().first().format.foreground().color() != darkColor)
        return 10;
    editor.setStyleSheet(QString());
    editor.setPlainText(QString(20000, 'x') + "\nint after = 1;");
    highlighter = editor.document()->findChild<SourceHighlighter *>();
    highlighter->rehighlight();
    if (!editor.document()->firstBlock().layout()->formats().isEmpty()
        || editor.document()->lastBlock().layout()->formats().isEmpty())
        return 4;
    editor.setFileName("unknown.no-such-language");
    if (editor.languageName() != "Plain text")
        return 5;
    editor.setPlainText("    return value;");
    editor.moveCursor(QTextCursor::End);
    QKeyEvent enter(QEvent::KeyPress, Qt::Key_Return, Qt::NoModifier);
    QApplication::sendEvent(&editor, &enter);
    if (editor.toPlainText() != "    return value;\n    ")
        return 6;
    editor.undo();
    if (editor.toPlainText() != "    return value;")
        return 7;
    // KSyntaxHighlighting queues propagation with QTextBlock values. A review
    // pane can replace the document before those queued calls are delivered.
    editor.setFileName("sample.cpp");
    editor.setPlainText(QString("/* comment */\nint value = 1;\n").repeated(300));
    highlighter = editor.document()->findChild<SourceHighlighter *>();
    HighlightCallObserver obsoleteCalls;
    highlighter->installEventFilter(&obsoleteCalls);
    const auto obsoleteBlock = editor.document()->findBlockByNumber(500);
    if (!QMetaObject::invokeMethod(highlighter, "rehighlightBlock", Qt::QueuedConnection,
            Q_ARG(QTextBlock, obsoleteBlock)))
        return 11;
    editor.setPlainText("int replacement = 2;");
    QApplication::processEvents();
    if (obsoleteCalls.delivered || editor.toPlainText() != "int replacement = 2;"
        || editor.languageName() != "C++" || editor.document()->firstBlock().layout()->formats().isEmpty())
        return 12;
    std::puts("Native highlighting covers all 14 SDK languages, YAML, TypeScript and diffs; multiline, themes, long lines, editing and queued replacement verified.");
    return 0;
}
