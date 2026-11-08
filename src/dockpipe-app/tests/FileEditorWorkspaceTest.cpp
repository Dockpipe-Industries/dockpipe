#include "FileEditorWorkspace.h"
#include "SourceEditor.h"
#include <QApplication>
#include <QPlainTextDocumentLayout>
#include <QStackedWidget>
#include <QTabBar>
#include <QTextDocument>
#include <QTextCursor>
#include <cstdio>

namespace {
struct WorkspaceFixture {
    QStackedWidget *stack = new QStackedWidget;
    SourceEditor *normal = new SourceEditor;
    SourceEditor *workflow = new SourceEditor;
    QWidget *empty = new QWidget;
    FileEditorWorkspace workspace;

    WorkspaceFixture() : workspace(prepareStack(), normal, workflow, empty) {}

    QStackedWidget *prepareStack()
    {
        stack->addWidget(normal);
        stack->addWidget(workflow);
        stack->addWidget(empty);
        return stack;
    }

    bool add(const QString &path, const QString &text = "initial\nsecond line")
    {
        return workspace.addDocument(path, text.toUtf8(), text, false, false);
    }
};

bool check(bool condition, const char *description)
{
    if (!condition)
        std::fprintf(stderr, "%s\n", description);
    return condition;
}
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    {
        WorkspaceFixture fixture;
        int confirmations = 0;
        fixture.workspace.confirmClose = [&confirmations](const QString &) {
            ++confirmations;
            return false;
        };
        if (!fixture.add("first.txt"))
            return 1;
        auto *first = fixture.workspace.document("first.txt")->document.data();
        fixture.normal->moveCursor(QTextCursor::End);
        fixture.normal->insertPlainText(" unsaved");
        const QString draft = first->toPlainText();
        if (!fixture.add("second.txt") || !fixture.workspace.activate("first.txt")
            || !check(first == fixture.normal->document() && first->toPlainText() == draft,
                "Switching tabs lost the retained draft")
            || !check(confirmations == 0, "Switching tabs prompted for a draft"))
            return 2;
        fixture.normal->undo();
        if (!check(first->toPlainText() == "initial\nsecond line", "Switching tabs lost undo history"))
            return 3;
        fixture.normal->insertPlainText("modified");
        if (!check(!fixture.workspace.closeActiveTab() && confirmations == 1
                && fixture.workspace.contains("first.txt"), "Cancelled close discarded the draft"))
            return 4;
        fixture.workspace.confirmClose = [](const QString &) { return true; };
        QPointer<QTextDocument> removed = first;
        if (!fixture.workspace.closeActiveTab() || !removed.isNull())
            return 5;
    }
    {
        WorkspaceFixture fixture;
        fixture.add("shared.txt", "abcdef");
        QTextCursor primaryCursor = fixture.normal->textCursor();
        primaryCursor.setPosition(2);
        fixture.normal->setTextCursor(primaryCursor);
        fixture.workspace.openInOtherPane("shared.txt");
        auto *secondary = fixture.workspace.activeNormalEditor();
        auto *shared = fixture.workspace.document("shared.txt")->document.data();
        if (!check(secondary != fixture.normal && secondary->document() == shared
                && fixture.normal->document() == shared, "Split views do not share one document"))
            return 6;
        QTextCursor secondaryCursor = secondary->textCursor();
        secondaryCursor.setPosition(5);
        secondary->setTextCursor(secondaryCursor);
        fixture.add("other.txt");
        fixture.workspace.activate("shared.txt");
        if (!check(secondary->textCursor().position() == 5
                && fixture.normal->textCursor().position() == 2,
                "Pane cursor states were not retained independently"))
            return 7;
        secondary->insertPlainText("!");
        if (!check(fixture.normal->toPlainText() == "abcde!f", "Split edits did not propagate"))
            return 8;
        const QTextCursor mergeCursor = secondary->textCursor();
        fixture.workspace.singlePane();
        if (!check(fixture.workspace.activeEditor()->textCursor().position() == mergeCursor.position()
                && fixture.workspace.activeEditor()->textCursor().anchor() == mergeCursor.anchor(),
                "Closing split lost the active secondary editing location"))
            return 26;
        if (!check(fixture.workspace.paths().size() == 2 && shared->isModified()
                && shared->toPlainText() == "abcde!f", "Closing split lost a document or draft"))
            return 9;
        auto *tabs = fixture.workspace.findChild<QTabBar *>("fileTabsPrimary");
        if (!check(tabs && tabs->count() == 2, "Closing split did not merge unique tabs"))
            return 10;
    }
    {
        WorkspaceFixture fixture;
        for (int index = 0; index < 32; ++index) {
            if (!fixture.add(QString("file-%1.txt").arg(index)))
                return 11;
        }
        fixture.normal->insertPlainText("retained");
        if (!check(!fixture.add("overflow.txt") && fixture.workspace.paths().size() == 32
                && fixture.workspace.error().contains("Close unused tabs")
                && fixture.workspace.hasUnsavedChanges(), "Document capacity silently evicted a draft"))
            return 12;
        if (!check(fixture.workspace.openInOtherPane("file-0.txt"),
                "Duplicating an existing view consumed document capacity"))
            return 13;
    }
    {
        QTextDocument external;
        external.setDocumentLayout(new QPlainTextDocumentLayout(&external));
        external.setPlainText("workflow: shared");
        external.setModified(false);
        QPointer<QTextDocument> externalGuard = &external;
        {
            WorkspaceFixture fixture;
            fixture.workspace.setWorkflowDocument("workflow.yml", &external);
            fixture.workspace.openWorkflow("workflow.yml");
            fixture.workspace.activeWorkflowEditor()->insertPlainText("# draft\n");
            int confirmations = 0;
            fixture.workspace.confirmClose = [&confirmations](const QString &) {
                ++confirmations;
                return false;
            };
            if (!check(fixture.workspace.closeActiveTab() && confirmations == 0
                    && external.isModified() && externalGuard,
                    "Closing workflow view changed external ownership or prompted"))
                return 14;
            fixture.workspace.clearDocuments();
            if (!check(fixture.workspace.openWorkflow("workflow.yml")
                    && fixture.workspace.activeWorkflowEditor()->document() == &external,
                    "Clearing Files discarded the external workflow binding"))
                return 15;
        }
        if (!check(externalGuard && external.isModified(), "Workspace destroyed an external workflow document"))
            return 16;
    }
    {
        WorkspaceFixture fixture;
        auto *external = new QTextDocument;
        external->setDocumentLayout(new QPlainTextDocumentLayout(external));
        fixture.workspace.setWorkflowDocument("external.yml", external);
        fixture.workspace.openWorkflow("external.yml");
        delete external;
        if (!check(!fixture.workspace.contains("external.yml")
                && fixture.workspace.activePath().isEmpty(), "Destroyed external document left stale tabs"))
            return 17;
        fixture.add("draft.yml");
        fixture.normal->insertPlainText("draft");
        QTextDocument replacement;
        if (!check(!fixture.workspace.setWorkflowDocument("draft.yml", &replacement)
                && fixture.workspace.hasUnsavedChanges(), "Workflow conversion replaced an unsaved regular draft"))
            return 18;
    }
    {
        WorkspaceFixture fixture;
        fixture.add("move.txt", "move draft");
        fixture.normal->moveCursor(QTextCursor::End);
        fixture.normal->insertPlainText(" retained");
        const QString draft = fixture.normal->toPlainText();
        int confirmations = 0;
        fixture.workspace.confirmClose = [&confirmations](const QString &) {
            ++confirmations;
            return false;
        };
        if (!check(fixture.workspace.moveToOtherPane("move.txt")
                && fixture.workspace.activeNormalEditor() != fixture.normal
                && fixture.workspace.activeEditor()->toPlainText() == draft
                && confirmations == 0, "Moving a draft prompted or changed its contents"))
            return 19;
        fixture.workspace.singlePane();
        if (!check(fixture.workspace.activeNormalEditor() == fixture.normal
                && fixture.workspace.activeEditor()->toPlainText() == draft
                && fixture.workspace.hasUnsavedChanges(), "Merging a moved draft lost its state"))
            return 20;
    }
    {
        WorkspaceFixture fixture;
        fixture.add("before/name.txt", "abcdef");
        fixture.normal->moveCursor(QTextCursor::End);
        fixture.normal->insertPlainText("draft");
        QTextDocument *shared = fixture.normal->document();
        fixture.workspace.openInOtherPane("before/name.txt");
        auto *secondary = fixture.workspace.activeNormalEditor();
        QTextCursor primaryCursor(shared);
        primaryCursor.setPosition(2);
        fixture.normal->setTextCursor(primaryCursor);
        QTextCursor secondaryCursor(shared);
        secondaryCursor.setPosition(4);
        secondary->setTextCursor(secondaryCursor);
        if (!check(fixture.workspace.renameDocument("before/name.txt", "after/renamed.txt")
                && !fixture.workspace.contains("before/name.txt")
                && fixture.workspace.document("after/renamed.txt")->document == shared
                && fixture.workspace.activePath() == "after/renamed.txt"
                && fixture.normal->textCursor().position() == 2
                && secondary->textCursor().position() == 4,
                "Renaming a split draft replaced its document or cursor state"))
            return 21;
        for (const QString &name : {QString("fileTabsPrimary"), QString("fileTabsSecondary")}) {
            auto *tabs = fixture.workspace.findChild<QTabBar *>(name);
            if (!check(tabs->tabData(0).toString() == "after/renamed.txt"
                    && tabs->tabToolTip(0) == "after/renamed.txt"
                    && tabs->tabText(0).contains("renamed.txt"),
                    "Renaming did not update both pane tab identities"))
                return 22;
        }
        secondary->undo();
        if (!check(shared->toPlainText() == "abcdef", "Renaming lost shared undo history"))
            return 23;
        fixture.add("other/renamed.txt");
        auto *tabs = fixture.workspace.findChild<QTabBar *>("fileTabsPrimary");
        if (!check(tabs->tabText(0).contains("after")
                && !fixture.workspace.renameDocument("after/renamed.txt", "other/renamed.txt")
                && !fixture.workspace.renameDocument("missing.txt", "new.txt"),
                "Basename labels or rename collision checks failed"))
            return 24;
        fixture.workspace.activate("after/renamed.txt");
        secondary->insertPlainText("new draft");
        if (!check(tabs->tabText(0).contains(QString::fromUtf8("•")),
                "Renamed document lost its modification notification binding"))
            return 25;
    }
    std::puts("File tabs retain drafts and undo, share split documents, preserve independent cursors, honor cancellation and capacity, and respect external workflow ownership.");
    return 0;
}
