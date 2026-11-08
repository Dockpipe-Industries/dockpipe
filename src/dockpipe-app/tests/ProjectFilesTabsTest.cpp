#include "FileEditorWorkspace.h"
#include "ProjectFilesWidget.h"
#include "WorkspaceFilesTestSupport.h"
#include "WorkspaceFilesController.h"
#include "SourceEditor.h"
#include "Theme.h"

#include <QApplication>
#include <QAbstractButton>
#include <QDir>
#include <QCryptographicHash>
#include <QJsonArray>
#include <QFile>
#include <QFileInfo>
#include <QMessageBox>
#include <QPixmap>
#include <QPushButton>
#include <QSignalSpy>
#include <QTabBar>
#include <QTemporaryDir>
#include <QTest>
#include <QTextCursor>
#include <QTextDocument>
#include <QTimer>
#include <QToolButton>

namespace {
bool writeBytes(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly | QIODevice::Truncate)
        && file.write(bytes) == bytes.size();
}

QByteArray readBytes(const QString &path)
{
    QFile file(path);
    return file.open(QIODevice::ReadOnly) ? file.readAll() : QByteArray();
}

class ModalAnswer {
public:
    explicit ModalAnswer(QMessageBox::StandardButton answer = QMessageBox::Cancel)
    {
        QObject::connect(&m_timer, &QTimer::timeout, [this, answer] {
            for (QWidget *widget : QApplication::topLevelWidgets()) {
                auto *dialog = qobject_cast<QMessageBox *>(widget);
                if (dialog && dialog->isVisible()) {
                    if (QAbstractButton *button = dialog->button(answer)) {
                        ++count;
                        button->click();
                    }
                }
            }
        });
        m_timer.start(10);
    }

    int count = 0;

private:
    QTimer m_timer;
};

struct FilesFixture {
    QTemporaryDir root;
    ProjectFilesWidget files;

    FileEditorWorkspace *workspace() const
    {
        return files.findChild<FileEditorWorkspace *>("fileEditorWorkspace");
    }

    QString path(const QString &name) const
    {
        return QDir(QFileInfo(root.path()).canonicalFilePath()).filePath(name);
    }

    bool prepare(const QByteArray &first = "first\n", const QByteArray &second = "second\n")
    {
        return root.isValid() && writeBytes(path("first.txt"), first)
            && writeBytes(path("second.txt"), second) && FilesTest::setRoot(files, root.path());
    }

    void replaceText(const QString &text)
    {
        SourceEditor *editor = workspace()->activeEditor();
        editor->selectAll();
        editor->insertPlainText(text);
    }
};
}

class ProjectFilesTabsTest final : public QObject {
    Q_OBJECT
private slots:
    void openingRetainsDraftAndUndo();
    void rootChangeCancelPreservesEveryDraft();
    void saveAllPreservesEncodingAndSelection();
    void paneFocusSelectsSaveTarget();
    void renamePreservesSharedDraftAndEncoding();
    void failedReloadAndConflictPreserveDraft();
    void closingDuplicateAndCancellingLastView();
    void singlePaneMergesDirtyTabs();
    void asynchronousSaveKeepsNewerEditsAndCapturedPath();
    void rapidOpenDiscardsStaleResponse();
    void closingSaveContinuesAfterCompletion();
    void remoteContextDoesNotReuseLocalWorkflow();
    void captureLayouts();
};

void ProjectFilesTabsTest::openingRetainsDraftAndUndo()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    QTextDocument *first = fixture.files.currentDocument();
    fixture.workspace()->activeEditor()->moveCursor(QTextCursor::End);
    fixture.workspace()->activeEditor()->insertPlainText("draft\n");
    const QString draft = first->toPlainText();
    ModalAnswer cancelUnexpectedDialog;
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    QCOMPARE(cancelUnexpectedDialog.count, 0);
    QCOMPARE(fixture.files.currentDocument(), first);
    QCOMPARE(first->toPlainText(), draft);
    QVERIFY(first->isModified());
    fixture.workspace()->activeEditor()->undo();
    QCOMPARE(first->toPlainText(), QString("first\n"));
    QCOMPARE(fixture.workspace()->paths().size(), 2);
}

void ProjectFilesTabsTest::rootChangeCancelPreservesEveryDraft()
{
    FilesFixture fixture;
    QTemporaryDir otherRoot;
    QVERIFY(fixture.prepare());
    QVERIFY(otherRoot.isValid());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("first unsaved\n");
    QTextDocument *first = fixture.files.currentDocument();
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    fixture.replaceText("second unsaved\n");
    QTextDocument *second = fixture.files.currentDocument();
    ModalAnswer cancel;
    QVERIFY(!FilesTest::setRoot(fixture.files, otherRoot.path()));
    QCOMPARE(cancel.count, 1);
    QCOMPARE(fixture.files.rootPath(), QFileInfo(fixture.root.path()).canonicalFilePath());
    QCOMPARE(fixture.workspace()->paths().size(), 2);
    QCOMPARE(first->toPlainText(), QString("first unsaved\n"));
    QCOMPARE(second->toPlainText(), QString("second unsaved\n"));
    QVERIFY(first->isModified());
    QVERIFY(second->isModified());
    QCOMPARE(fixture.files.currentDocument(), second);
}

void ProjectFilesTabsTest::saveAllPreservesEncodingAndSelection()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare(QByteArray("\xEF\xBB\xBF") + "first\r\n", "second\n"));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("first saved\nnext\n");
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    fixture.replaceText("second saved\n");
    QTextDocument *second = fixture.files.currentDocument();
    QTextCursor selection(second);
    selection.setPosition(1);
    selection.setPosition(6, QTextCursor::KeepAnchor);
    fixture.workspace()->activeEditor()->setTextCursor(selection);
    QSignalSpy saved(&fixture.files, &ProjectFilesWidget::fileSaved);
    QVERIFY(FilesTest::saveAll(fixture.files));
    QCOMPARE(saved.count(), 2);
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("\xEF\xBB\xBF") + "first saved\r\nnext\r\n");
    QCOMPARE(readBytes(fixture.path("second.txt")), QByteArray("second saved\n"));
    QVERIFY(!fixture.files.hasUnsavedChanges());
    QCOMPARE(fixture.files.currentPath(), fixture.path("second.txt"));
    QCOMPARE(fixture.files.currentDocument(), second);
    QCOMPARE(fixture.workspace()->activeEditor()->textCursor().anchor(), 1);
    QCOMPARE(fixture.workspace()->activeEditor()->textCursor().position(), 6);
}

void ProjectFilesTabsTest::paneFocusSelectsSaveTarget()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    fixture.files.resize(1280, 800);
    fixture.files.show();
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    auto *primary = fixture.workspace()->activeNormalEditor();
    QTextDocument *first = fixture.files.currentDocument();
    QVERIFY(fixture.workspace()->openInOtherPane(fixture.path("first.txt")));
    auto *secondary = fixture.workspace()->activeNormalEditor();
    QCOMPARE(secondary->document(), first);
    secondary->moveCursor(QTextCursor::End);
    secondary->insertPlainText("shared edit\n");
    QCOMPARE(primary->toPlainText(), QString("first\nshared edit\n"));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    fixture.replaceText("second draft\n");
    QTextDocument *second = fixture.files.currentDocument();
    QTest::mouseClick(primary->viewport(), Qt::LeftButton, Qt::NoModifier, QPoint(35, 25));
    QCOMPARE(fixture.files.currentPath(), fixture.path("first.txt"));
    QCOMPARE(fixture.files.currentDocument(), first);
    QVERIFY(FilesTest::save(fixture.files));
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("first\nshared edit\n"));
    QCOMPARE(readBytes(fixture.path("second.txt")), QByteArray("second\n"));
    QVERIFY(second->isModified());
    QTest::mouseClick(secondary->viewport(), Qt::LeftButton, Qt::NoModifier, QPoint(35, 25));
    QCOMPARE(fixture.files.currentPath(), fixture.path("second.txt"));
    QCOMPARE(fixture.files.currentDocument(), second);
    QVERIFY(FilesTest::save(fixture.files));
    QCOMPARE(readBytes(fixture.path("second.txt")), QByteArray("second draft\n"));
}

void ProjectFilesTabsTest::renamePreservesSharedDraftAndEncoding()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare(QByteArray("\xEF\xBB\xBF") + "first\r\n"));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.workspace()->activeEditor()->moveCursor(QTextCursor::End);
    fixture.workspace()->activeEditor()->insertPlainText("retained\n");
    QTextDocument *document = fixture.files.currentDocument();
    QVERIFY(fixture.workspace()->openInOtherPane(fixture.path("first.txt")));
    const QString renamed = fixture.path("renamed.txt");
    ModalAnswer cancelUnexpectedDialog;
    QVERIFY(FilesTest::rename(fixture.files, renamed));
    QCOMPARE(cancelUnexpectedDialog.count, 0);
    QVERIFY(!QFile::exists(fixture.path("first.txt")));
    QVERIFY(QFile::exists(renamed));
    QCOMPARE(fixture.files.currentDocument(), document);
    QVERIFY(document->isModified());
    QCOMPARE(fixture.files.currentPath(), renamed);
    for (const QString &name : {QString("fileTabsPrimary"), QString("fileTabsSecondary")}) {
        auto *tabs = fixture.workspace()->findChild<QTabBar *>(name);
        QVERIFY(tabs);
        QCOMPARE(tabs->tabData(0).toString(), renamed);
        QCOMPARE(tabs->tabToolTip(0), renamed);
        QVERIFY(tabs->tabText(0).contains("renamed.txt"));
    }
    fixture.workspace()->activeEditor()->undo();
    QCOMPARE(document->toPlainText(), QString("first\n"));
    fixture.workspace()->activeEditor()->redo();
    QVERIFY(document->isModified());
    QVERIFY(FilesTest::save(fixture.files));
    QCOMPARE(readBytes(renamed), QByteArray("\xEF\xBB\xBF") + "first\r\nretained\r\n");
    QVERIFY(!document->isModified());
}

void ProjectFilesTabsTest::failedReloadAndConflictPreserveDraft()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("local draft\n");
    QTextDocument *document = fixture.files.currentDocument();
    QVERIFY(writeBytes(fixture.path("first.txt"), "external edit\n"));
    QVERIFY(!FilesTest::save(fixture.files));
    QCOMPARE(document->toPlainText(), QString("local draft\n"));
    QVERIFY(document->isModified());
    QVERIFY(writeBytes(fixture.path("first.txt"), QByteArray("invalid\0binary", 14)));
    ModalAnswer discard(QMessageBox::Discard);
    QVERIFY(!FilesTest::reload(fixture.files));
    QCOMPARE(discard.count, 1);
    QCOMPARE(fixture.files.currentDocument(), document);
    QCOMPARE(document->toPlainText(), QString("local draft\n"));
    QVERIFY(document->isModified());
}

void ProjectFilesTabsTest::closingDuplicateAndCancellingLastView()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("retained draft\n");
    QTextDocument *document = fixture.files.currentDocument();
    QVERIFY(fixture.workspace()->openInOtherPane(fixture.path("first.txt")));
    ModalAnswer cancel;
    auto *tabs = fixture.files.findChild<QTabBar *>("fileTabsSecondary");
    QVERIFY(tabs);
    auto *closeSlot = tabs->tabButton(tabs->currentIndex(), QTabBar::RightSide);
    QVERIFY(closeSlot);
    auto *close = closeSlot->findChild<QToolButton *>("fileTabClose");
    QVERIFY(close);
    close->click();
    QCOMPARE(tabs->count(), 0);
    QCOMPARE(cancel.count, 0);
    QVERIFY(document->isModified());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    QVERIFY(!fixture.workspace()->closeActiveTab());
    QCOMPARE(cancel.count, 1);
    QCOMPARE(fixture.files.currentDocument(), document);
    QCOMPARE(document->toPlainText(), QString("retained draft\n"));
    QVERIFY(fixture.files.hasUnsavedChanges());
}

void ProjectFilesTabsTest::singlePaneMergesDirtyTabs()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("first draft\n");
    QTextDocument *first = fixture.files.currentDocument();
    QVERIFY(fixture.workspace()->openInOtherPane(fixture.path("first.txt")));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    fixture.replaceText("second draft\n");
    QTextDocument *second = fixture.files.currentDocument();
    ModalAnswer cancelUnexpectedDialog;
    fixture.workspace()->singlePane();
    QCOMPARE(cancelUnexpectedDialog.count, 0);
    QCOMPARE(fixture.workspace()->paths().size(), 2);
    QCOMPARE(fixture.workspace()->findChild<QTabBar *>("fileTabsPrimary")->count(), 2);
    QCOMPARE(fixture.workspace()->findChild<QTabBar *>("fileTabsSecondary")->count(), 0);
    QCOMPARE(first->toPlainText(), QString("first draft\n"));
    QCOMPARE(second->toPlainText(), QString("second draft\n"));
    QVERIFY(first->isModified());
    QVERIFY(second->isModified());
    QVERIFY(FilesTest::saveAll(fixture.files));
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("first draft\n"));
    QCOMPARE(readBytes(fixture.path("second.txt")), QByteArray("second draft\n"));
}

void ProjectFilesTabsTest::asynchronousSaveKeepsNewerEditsAndCapturedPath()
{
    FilesFixture fixture;
    QTemporaryDir otherRoot;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("saved snapshot\n");
    QTextDocument *first = fixture.files.currentDocument();
    bool completed = false;
    bool saved = false;
    fixture.files.saveFileAsync([&](bool success) { saved = success; completed = true; });
    QVERIFY(!completed);
    QVERIFY(fixture.files.filesOperationPending());
    QVERIFY(!fixture.workspace()->closeActiveTab());
    QVERIFY(!fixture.files.setRoot(otherRoot.path()));
    fixture.replaceText("newer draft\n");
    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    QTRY_VERIFY(completed);
    QVERIFY(saved);
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("saved snapshot\n"));
    QCOMPARE(first->toPlainText(), QString("newer draft\n"));
    QVERIFY(first->isModified());
    QCOMPARE(fixture.workspace()->document(fixture.path("first.txt"))->original, QByteArray("saved snapshot\n"));
    QCOMPARE(fixture.files.currentPath(), fixture.path("second.txt"));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    QVERIFY(FilesTest::save(fixture.files));
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("newer draft\n"));
    QVERIFY(!first->isModified());
}

void ProjectFilesTabsTest::rapidOpenDiscardsStaleResponse()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    bool firstCompleted = false;
    bool firstOpened = true;
    bool secondOpened = false;
    fixture.files.openFileAsync(fixture.path("first.txt"), [&](bool success) {
        firstOpened = success;
        firstCompleted = true;
    });
    fixture.files.openFileAsync(fixture.path("second.txt"), [&](bool success) { secondOpened = success; });
    QTRY_VERIFY(secondOpened && firstCompleted);
    QVERIFY(!firstOpened);
    QCOMPARE(fixture.files.currentPath(), fixture.path("second.txt"));
    QCOMPARE(fixture.files.currentDocument()->toPlainText(), QString("second\n"));
    QVERIFY(!fixture.workspace()->contains(fixture.path("first.txt")));
}

void ProjectFilesTabsTest::closingSaveContinuesAfterCompletion()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("save on close\n");
    ModalAnswer save(QMessageBox::Save);
    QVERIFY(!fixture.workspace()->closeActiveTab());
    QCOMPARE(save.count, 1);
    QTRY_VERIFY(!fixture.workspace()->contains(fixture.path("first.txt")));
    QCOMPARE(readBytes(fixture.path("first.txt")), QByteArray("save on close\n"));

    QVERIFY(FilesTest::open(fixture.files, fixture.path("second.txt")));
    fixture.replaceText("snapshot on close\n");
    QVERIFY(!fixture.workspace()->closeActiveTab());
    fixture.replaceText("typed during save\n");
    QTRY_VERIFY(!fixture.files.filesOperationPending());
    QVERIFY(fixture.workspace()->contains(fixture.path("second.txt")));
    QCOMPARE(fixture.files.currentDocument()->toPlainText(), QString("typed during save\n"));
    QVERIFY(fixture.files.currentDocument()->isModified());
    QCOMPARE(readBytes(fixture.path("second.txt")), QByteArray("snapshot on close\n"));
}

void ProjectFilesTabsTest::remoteContextDoesNotReuseLocalWorkflow()
{
    FilesFixture fixture;
    QVERIFY(fixture.prepare());
    const QString path = fixture.path("workflow.yml");
    QVERIFY(writeBytes(path, "local disk\n"));
    SourceEditor localWorkflow;
    localWorkflow.setPlainText("local shared document\n");
    fixture.files.setWorkflowDocument(path, localWorkflow.document());
    QVERIFY(FilesTest::open(fixture.files, path));
    QCOMPARE(fixture.files.currentDocument(), localWorkflow.document());

    WorkspaceFilesClient::Context context;
    context.kind = WorkspaceFilesClient::Context::Kind::Remote;
    context.nodeId = "worker";
    context.fingerprint = QString(64, 'a');
    context.workerGeneration = "generation";
    context.rootId = "opaque-root";
    context.generation = 44;
    QByteArray remoteBytes("remote document\n");
    int remoteReads = 0;
    auto dispatch = [&](const auto &captured, const auto &request, auto completion) {
        QJsonObject value{{"schema", "dockpipe.files/v1"}, {"path", request.value("path")}};
        if (captured.rootId != context.rootId)
            return WorkspaceFilesClient::Cancel();
        const QString action = request.value("action").toString();
        if (action == "read") {
            ++remoteReads;
            value.insert("content", QString::fromLatin1(remoteBytes.toBase64()));
        } else if (action == "write") {
            remoteBytes = QByteArray::fromBase64(request.value("content").toString().toLatin1());
        } else {
            value.insert("entries", QJsonArray());
        }
        value.insert("revision", QString::fromLatin1(QCryptographicHash::hash(remoteBytes, QCryptographicHash::Sha256).toHex()));
        completion({WorkspaceFilesClient::Outcome::Success, value, {}, captured});
        return WorkspaceFilesClient::Cancel();
    };
    QVERIFY(FilesTest::open(fixture.files, fixture.path("first.txt")));
    fixture.replaceText("local unsaved draft\n");
    const QPointer<QTextDocument> localDraft = fixture.files.currentDocument();
    QVERIFY(!WorkspaceFilesController::configure(&fixture.files, context, dispatch));
    QVERIFY(localDraft && localDraft->isModified());
    ModalAnswer discard(QMessageBox::Discard);
    bool configured = false;
    fixture.files.confirmLeaveAsync([&](bool allowed) {
        if (allowed)
            configured = WorkspaceFilesController::configure(&fixture.files, context, dispatch);
    });
    QVERIFY(configured);
    QCOMPARE(discard.count, 1);
    QVERIFY(localDraft.isNull());
    QVERIFY(FilesTest::open(fixture.files, path));
    QCOMPARE(remoteReads, 1);
    QVERIFY(fixture.files.currentDocument() != localWorkflow.document());
    QCOMPARE(fixture.files.currentDocument()->toPlainText(), QString("remote document\n"));
    QVERIFY(fixture.files.findChild<QPushButton *>("fileOpenWorkflow")->isHidden());
    QVERIFY(!fixture.files.findChild<QPushButton *>("openWorkspaceGraph")->isEnabled());
    fixture.replaceText("remote saved\n");
    QVERIFY(FilesTest::save(fixture.files));
    QCOMPARE(remoteBytes, QByteArray("remote saved\n"));
    QCOMPARE(readBytes(path), QByteArray("local disk\n"));
    QCOMPARE(localWorkflow.toPlainText(), QString("local shared document\n"));
}

void ProjectFilesTabsTest::captureLayouts()
{
    const QString destination = qEnvironmentVariable("DOCKPIPE_FILES_TABS_SCREENSHOT");
    if (destination.isEmpty())
        QSKIP("Set DOCKPIPE_FILES_TABS_SCREENSHOT to capture Files layouts.");
    FilesFixture fixture;
    QVERIFY(fixture.root.isValid());
    QVERIFY(writeBytes(fixture.path("README.md"),
        "# Signal Garden\n\nA small workspace for clear ideas.\n\n"
        "## Getting started\n\nRun `python app.py` to start the preview.\n\n"
        "- Keep configuration in settings.yml\n- Review changes before publishing\n"));
    QVERIFY(writeBytes(fixture.path("app.py"),
        "from pathlib import Path\n\n\n"
        "def greet(name: str) -> str:\n"
        "    return f\"Welcome to Signal Garden, {name}!\"\n\n\n"
        "def main() -> None:\n"
        "    configuration = Path(\"settings.yml\")\n"
        "    print(greet(\"reader\"))\n"
        "    print(f\"Configuration: {configuration}\")\n\n\n"
        "if __name__ == \"__main__\":\n"
        "    main()\n"));
    QVERIFY(writeBytes(fixture.path("settings.yml"),
        "name: signal-garden\nversion: 1.0\n\n"
        "preview:\n  host: localhost\n  port: 8080\n\n"
        "appearance:\n  theme: system\n  show_line_numbers: true\n"));
    QVERIFY(FilesTest::setRoot(fixture.files, fixture.root.path()));
    fixture.files.resize(1280, 800);
    fixture.files.show();
    QVERIFY(FilesTest::open(fixture.files, fixture.path("README.md")));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("app.py")));
    fixture.workspace()->activeEditor()->moveCursor(QTextCursor::End);
    fixture.workspace()->activeEditor()->insertPlainText("\n# Ready for a fresh idea.\n");
    QVERIFY(fixture.workspace()->openInOtherPane(fixture.path("app.py")));
    QVERIFY(FilesTest::open(fixture.files, fixture.path("settings.yml")));
    fixture.workspace()->activeEditor()->moveCursor(QTextCursor::End);
    fixture.workspace()->activeEditor()->insertPlainText("  accent: mint\n");
    QDir().mkpath(QFileInfo(destination).absolutePath());
    QTest::qWait(300);
    QVERIFY(fixture.files.grab().save(destination));
    const QFileInfo info(destination);
    const QString prefix = info.absolutePath() + '/' + info.completeBaseName();
    fixture.workspace()->split(Qt::Vertical);
    QTest::qWait(100);
    QVERIFY(fixture.files.grab().save(prefix + "-stacked.png"));
    fixture.workspace()->split(Qt::Horizontal);
    fixture.files.resize(1000, 700);
    QTest::qWait(100);
    QVERIFY(fixture.files.grab().save(prefix + "-compact.png"));
}

int main(int argc, char **argv)
{
    QTemporaryDir configuration;
    if (!configuration.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", configuration.path().toUtf8());
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    applyDockpipeLauncherTheme(application);
    previewDockpipeLauncherTheme(application,
        qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
            ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    ProjectFilesTabsTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "ProjectFilesTabsTest.moc"
