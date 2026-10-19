#include "ProjectFilesWidget.h"
#include "Theme.h"
#include "SourceEditor.h"
#include <QApplication>
#include <QPushButton>
#include <QStyle>
#include <QDir>
#include <QFile>
#include <QElapsedTimer>
#include <QFileSystemModel>
#include <QPlainTextEdit>
#include <QMessageBox>
#include <QTemporaryDir>
#include <QTextDocument>
#include <QThread>
#include <QTreeView>
#include <QTimer>
#include <cstdio>

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    applyDockpipeLauncherTheme(app);
    if (qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light") {
        QFile shared(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!shared.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 28;
        app.setStyleSheet(QString());
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
    }
    QTemporaryDir root;
    QTemporaryDir outside;
    SourceEditor workflowEditor;
    ProjectFilesWidget files;
    files.resize(1180, 760);
    files.show();
    if (!files.setRoot(root.path()) || !files.createFile("script.py"))
        return 1;
    auto *workflowAction = files.findChild<QPushButton *>("fileOpenWorkflow");
    if (!workflowAction || !workflowAction->isHidden())
        return 40;
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    editor->insertPlainText("print('hello')\n");
    if (!files.hasUnsavedChanges() || !files.saveFile() || files.hasUnsavedChanges())
        return 2;
    QFile script(root.filePath("script.py"));
    if (!script.open(QIODevice::ReadOnly) || script.readAll() != "print('hello')\n")
        return 3;
    script.close();
    if (!files.renameFile(root.filePath("renamed.py")) || QFile::exists(script.fileName()))
        return 4;
    if (!files.deleteFile() || QFile::exists(root.filePath("renamed.py")))
        return 5;
    if (files.createFile("../outside.txt") || files.createFile(outside.filePath("escape.txt")))
        return 6;

    QFile text(root.filePath("settings.yml"));
    if (!text.open(QIODevice::WriteOnly))
        return 7;
    text.write("\xEF\xBB\xBFname: hello\r\n");
    text.close();
    if (!files.openFile(text.fileName()))
        return 8;
    if (workflowAction->isHidden())
        return 41;
    QString requestedWorkflow;
    QObject::connect(&files, &ProjectFilesWidget::workflowOpenRequested, &files,
        [&](const QString &path) { requestedWorkflow = path; });
    workflowAction->click();
    if (requestedWorkflow != text.fileName())
        return 42;
    editor->selectAll();
    editor->insertPlainText("name: updated\n");
    if (!files.saveFile() || !text.open(QIODevice::ReadOnly) || text.readAll() != "\xEF\xBB\xBFname: updated\r\n")
        return 9;
    text.close();
    editor->selectAll();
    editor->insertPlainText("name: unsaved\n");
    if (!text.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return 10;
    text.write("external change\n");
    text.close();
    if (files.saveFile() || !files.hasUnsavedChanges())
        return 11;
    editor->document()->setModified(false);
    if (!files.reloadFile() || editor->toPlainText() != "external change\n")
        return 18;
    editor->insertPlainText("Keep this draft.\n");
    QTimer::singleShot(0, []() {
        for (auto *widget : QApplication::topLevelWidgets()) {
            if (auto *dialog = qobject_cast<QMessageBox *>(widget))
                dialog->done(QMessageBox::Cancel);
        }
    });
    if (files.setRoot(outside.path()) || !files.hasUnsavedChanges() || files.currentPath() != text.fileName())
        return 20;
    editor->document()->setModified(false);
    QDir().mkdir(root.filePath(".git"));
    if (files.createFile(".git/config"))
        return 21;
#ifdef Q_OS_UNIX
    if (!QFile::link(text.fileName(), root.filePath("linked.yml")) || files.openFile(root.filePath("linked.yml")))
        return 22;
#endif
    QFile oversized(root.filePath("large.txt"));
    if (!oversized.open(QIODevice::WriteOnly) || !oversized.resize(1024 * 1024 + 1))
        return 23;
    oversized.close();
    if (files.openFile(oversized.fileName()))
        return 24;

    QFile binary(root.filePath("image.bin"));
    if (!binary.open(QIODevice::WriteOnly))
        return 12;
    binary.write(QByteArray("binary\0bytes", 12));
    binary.close();
    if (files.openFile(binary.fileName()))
        return 13;
    QDir().mkdir(root.filePath("workflow"));
    if (!files.setRoot(root.filePath("workflow")) || files.openFile(text.fileName()))
        return 14;
    QFile workflow(root.filePath("workflow/config.yml"));
    if (!workflow.open(QIODevice::WriteOnly))
        return 15;
    workflow.write("name: hello\n");
    workflow.close();
    workflowEditor.setPlainText("name: shared draft\n");
    files.setWorkflowDocument(workflow.fileName(), workflowEditor.document());
    if (!files.openFile(workflow.fileName()))
        return 16;
    auto *shared = files.findChild<QPlainTextEdit *>("workflowFileTextEditor");
    shared->setPlainText("name: updated shared draft\n");
    if (workflowEditor.toPlainText() != shared->toPlainText())
        return 17;
    auto *tree = files.findChild<QTreeView *>("fileTree");
    auto *model = qobject_cast<QFileSystemModel *>(tree->model());
    QElapsedTimer timer;
    timer.start();
    while ((!tree->rootIndex().isValid() || model->rowCount(tree->rootIndex()) == 0) && timer.elapsed() < 3000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (!tree->rootIndex().isValid() || model->filePath(tree->rootIndex()) != root.filePath("workflow")
        || model->rowCount(tree->rootIndex()) != 1) {
        std::fprintf(stderr, "Explorer root mismatch: actual=%s expected=%s rows=%d\n",
            model->filePath(tree->rootIndex()).toUtf8().constData(),
            root.filePath("workflow").toUtf8().constData(), model->rowCount(tree->rootIndex()));
        return 19;
    }
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_FILES_SCREENSHOT");
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
