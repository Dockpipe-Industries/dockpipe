#include "ProjectFilesWidget.h"
#include "WorkspaceFilesTestSupport.h"
#include "Theme.h"
#include "SourceEditor.h"
#include "FileExplorerView.h"
#include <QApplication>
#include <QPushButton>
#include <QStyle>
#include <QDir>
#include <QFile>
#include <QElapsedTimer>
#include "WorkspaceFileModel.h"
#include <QSortFilterProxyModel>
#include <QPlainTextEdit>
#include <QMessageBox>
#include <QTemporaryDir>
#include <QTextDocument>
#include <QThread>
#include <QTreeView>
#include <QTimer>
#include <QTest>
#include <QListView>
#include <QLineEdit>
#include <cstdio>
#include <QDebug>

namespace {
int failedCheck(int code, int line)
{
    qCritical("ProjectFilesTest check %d failed at source line %d", code, line);
    return code;
}
}

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    applyDockpipeLauncherTheme(app);
    previewDockpipeLauncherTheme(app, qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
        ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    QTemporaryDir root;
    QTemporaryDir outside;
    SourceEditor workflowEditor;
    ProjectFilesWidget files;
    files.resize(1180, 760);
    files.show();
    if (!FilesTest::setRoot(files, root.path()) || !FilesTest::create(files, "script.py"))
        return failedCheck(1, __LINE__);
    auto *workflowAction = files.findChild<QPushButton *>("fileOpenWorkflow");
    if (!workflowAction || !workflowAction->isHidden())
        return failedCheck(40, __LINE__);
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    editor->insertPlainText("print('hello')\n");
    if (!files.hasUnsavedChanges() || !FilesTest::save(files) || files.hasUnsavedChanges())
        return failedCheck(2, __LINE__);
    QFile script(root.filePath("script.py"));
    if (!script.open(QIODevice::ReadOnly) || script.readAll() != "print('hello')\n")
        return failedCheck(3, __LINE__);
    script.close();
    if (!FilesTest::rename(files, root.filePath("renamed.py")) || QFile::exists(script.fileName()))
        return failedCheck(4, __LINE__);
    if (!FilesTest::remove(files) || QFile::exists(root.filePath("renamed.py")))
        return failedCheck(5, __LINE__);
    if (FilesTest::create(files, "../outside.txt") || FilesTest::create(files, outside.filePath("escape.txt")))
        return failedCheck(6, __LINE__);

    QFile text(root.filePath("settings.yml"));
    if (!text.open(QIODevice::WriteOnly))
        return failedCheck(7, __LINE__);
    text.write("\xEF\xBB\xBFname: hello\r\n");
    text.close();
    if (!FilesTest::open(files, text.fileName()))
        return failedCheck(8, __LINE__);
    if (workflowAction->isHidden())
        return failedCheck(41, __LINE__);
    QString requestedWorkflow;
    QObject::connect(&files, &ProjectFilesWidget::workflowOpenRequested, &files,
        [&](const QString &path) { requestedWorkflow = path; });
    workflowAction->click();
    if (requestedWorkflow != text.fileName())
        return failedCheck(42, __LINE__);
    editor->selectAll();
    editor->insertPlainText("name: updated\n");
    if (!FilesTest::save(files) || !text.open(QIODevice::ReadOnly) || text.readAll() != "\xEF\xBB\xBFname: updated\r\n")
        return failedCheck(9, __LINE__);
    text.close();
    editor->selectAll();
    editor->insertPlainText("name: unsaved\n");
    if (!text.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return failedCheck(10, __LINE__);
    text.write("external change\n");
    text.close();
    if (FilesTest::save(files) || !files.hasUnsavedChanges())
        return failedCheck(11, __LINE__);
    editor->document()->setModified(false);
    if (!FilesTest::reload(files) || editor->toPlainText() != "external change\n")
        return failedCheck(18, __LINE__);
    editor->insertPlainText("Keep this draft.\n");
    QTimer::singleShot(0, []() {
        for (auto *widget : QApplication::topLevelWidgets()) {
            if (auto *dialog = qobject_cast<QMessageBox *>(widget))
                dialog->done(QMessageBox::Cancel);
        }
    });
    const bool switchedRoot = FilesTest::setRoot(files, outside.path());
    if (switchedRoot || !files.hasUnsavedChanges() || files.currentPath() != text.fileName()) {
        qWarning() << "Cancelled root change:" << switchedRoot << files.hasUnsavedChanges()
                   << files.currentPath() << text.fileName() << editor->document()->isModified();
        return failedCheck(20, __LINE__);
    }
    editor->document()->setModified(false);
    QDir().mkdir(root.filePath(".git"));
    if (FilesTest::create(files, ".git/config"))
        return failedCheck(21, __LINE__);
#ifdef Q_OS_UNIX
    if (!QFile::link(text.fileName(), root.filePath("linked.yml")) || FilesTest::open(files, root.filePath("linked.yml")))
        return failedCheck(22, __LINE__);
#endif
    QFile oversized(root.filePath("large.txt"));
    if (!oversized.open(QIODevice::WriteOnly) || !oversized.resize(1024 * 1024 + 1))
        return failedCheck(23, __LINE__);
    oversized.close();
    if (FilesTest::open(files, oversized.fileName()))
        return failedCheck(24, __LINE__);

    QFile binary(root.filePath("image.bin"));
    if (!binary.open(QIODevice::WriteOnly))
        return failedCheck(12, __LINE__);
    binary.write(QByteArray("binary\0bytes", 12));
    binary.close();
    if (FilesTest::open(files, binary.fileName()))
        return failedCheck(13, __LINE__);
    QDir().mkdir(root.filePath("workflow"));
    if (!FilesTest::setRoot(files, root.filePath("workflow")) || FilesTest::open(files, text.fileName()))
        return failedCheck(14, __LINE__);
    QFile workflow(root.filePath("workflow/config.yml"));
    if (!workflow.open(QIODevice::WriteOnly))
        return failedCheck(15, __LINE__);
    workflow.write("name: hello\n");
    workflow.close();
    workflowEditor.setPlainText("name: shared draft\n");
    files.setWorkflowDocument(workflow.fileName(), workflowEditor.document());
    if (!FilesTest::open(files, workflow.fileName()))
        return failedCheck(16, __LINE__);
    auto *shared = files.findChild<QPlainTextEdit *>("workflowFileTextEditor");
    shared->setPlainText("name: updated shared draft\n");
    if (workflowEditor.toPlainText() != shared->toPlainText())
        return failedCheck(17, __LINE__);
    auto *tree = files.findChild<QTreeView *>("fileTree");
    auto *model = qobject_cast<QSortFilterProxyModel *>(tree->model());
    auto *coreModel = qobject_cast<WorkspaceFileModel *>(model->sourceModel());
    coreModel->refresh();
    QElapsedTimer timer;
    timer.start();
    while (model->rowCount(tree->rootIndex()) == 0 && timer.elapsed() < 3000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (files.rootPath() != QFileInfo(root.filePath("workflow")).canonicalFilePath()
        || model->rowCount(tree->rootIndex()) != 1) {
        std::fprintf(stderr, "Explorer root mismatch: actual=%s expected=%s rows=%d\n",
            files.rootPath().toUtf8().constData(),
            QFileInfo(root.filePath("workflow")).canonicalFilePath().toUtf8().constData(), model->rowCount(tree->rootIndex()));
        return failedCheck(19, __LINE__);
    }
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_FILES_SCREENSHOT");
    QFile quick(root.filePath("workflow/quick.py"));
    if (!quick.open(QIODevice::WriteOnly))
        return failedCheck(43, __LINE__);
    quick.write("def ready():\n    return True\n");
    quick.close();
    auto *explorer = files.findChild<FileExplorerView *>();
    explorer->setMode(FileExplorerView::Mode::Folders);
    auto *icons = explorer->iconView();
    coreModel->refresh();
    timer.restart();
    while (model->rowCount(tree->rootIndex()) < 2
        && timer.elapsed() < 3000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    const auto quickIndex = model->mapFromSource(coreModel->indexForPath("quick.py"));
    QApplication::processEvents();
    QTest::mouseClick(icons->viewport(), Qt::LeftButton, Qt::NoModifier, icons->visualRect(quickIndex).center());
    timer.restart();
    while (files.currentPath() != quick.fileName() && timer.elapsed() < 3000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (files.currentPath() != quick.fileName() || editor->toPlainText() != "def ready():\n    return True\n")
        return failedCheck(44, __LINE__);
    auto *filter = files.findChild<QLineEdit *>("filesFilter");
    filter->setText("config");
    QApplication::processEvents();
    if (model->rowCount(tree->rootIndex()) != 1)
        return failedCheck(45, __LINE__);
    filter->clear();
    explorer->setMode(FileExplorerView::Mode::Tree);
    FilesTest::open(files, workflow.fileName());
    if (!screenshot.isEmpty()) {
        QDir().mkdir(root.filePath("workflow/scripts"));
        QDir().mkdir(root.filePath("workflow/assets"));
        shared->setPlainText("# Build and verify this workspace\nname: build-project\ndescription: Build, test and package the application\n\nsteps:\n  - id: build\n    run: go build ./...\n\n  - id: test\n    run: go test ./...\n");
        QApplication::processEvents();
        files.grab().save(screenshot);
        files.resize(820, 620);
        QApplication::processEvents();
        files.grab().save(screenshot + ".compact.png");
    }
    std::puts("Built-in file CRUD, scope containment, encoding, conflict preservation and shared YAML passed.");
    return 0;
}
