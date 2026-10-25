#include "WorkflowCanvas.h"
#include "WorkflowEditorWidget.h"
#include "ProjectFilesWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QAction>
#include <QElapsedTimer>
#include <QFile>
#include <QFileDialog>
#include <QGraphicsItem>
#include <QGraphicsScene>
#include <QLabel>
#include <QLineEdit>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QTabWidget>
#include <QStyle>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <cstdio>
#include <functional>

namespace {
bool waitUntil(const std::function<bool()> &predicate)
{
    QElapsedTimer timer;
    timer.start();
    while (timer.elapsed() < 10000) {
        QApplication::processEvents();
        if (predicate())
            return true;
        QThread::msleep(10);
    }
    return false;
}

bool containsLabel(QWidget *widget, const QString &text)
{
    for (auto *label : widget->findChildren<QLabel *>()) {
        if (label->text().contains(text))
            return true;
    }
    return false;
}

QPushButton *button(QWidget *widget, const QString &text)
{
    for (auto *candidate : widget->findChildren<QPushButton *>()) {
        if (candidate->text() == text)
            return candidate;
    }
    return nullptr;
}

bool selectCard(WorkflowCanvas *canvas, const QJsonArray &path)
{
    canvas->scene()->clearSelection();
    for (auto *item : canvas->scene()->items()) {
        if ((item->flags() & QGraphicsItem::ItemIsSelectable) && item->data(0).toJsonArray() == path) {
            item->setSelected(true);
            return true;
        }
    }
    return false;
}
}

int main(int argc, char **argv)
{
    QTemporaryDir temporary;
    qputenv("XDG_CONFIG_HOME", temporary.path().toUtf8());
    QApplication app(argc, argv);
    app.setOrganizationName("DockpipeStudioTest");
    app.setApplicationName("WorkflowStudioTest");
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
    WorkflowCanvas canvas;
    const QJsonObject command{{"id", "prepare"}, {"kind", "host"}, {"cmd", "echo ready"}};
    const QJsonObject parallel{{"group", QJsonObject{{"mode", "async"}, {"tasks", QJsonArray{command, command}}}}};
    canvas.setDocument({{"steps", QJsonArray{command, parallel, command}}});
    const QJsonObject before = canvas.positions();
    canvas.restorePositions({{"0", QJsonArray{120, 80}}});
    if (canvas.positions() == before || canvas.positions().value("0").toArray()[0] != 120)
        return 1;
    int selected = 0;
    QObject::connect(&canvas, &WorkflowCanvas::nodeSelected, [&selected](const QJsonArray &path) {
        if (path == QJsonArray{"steps", "1", "group", "tasks", "0"}) ++selected;
    });
    if (!selectCard(&canvas, {"steps", "1", "group", "tasks", "0"}) || selected != 1)
        return 2;

    // Failure must preserve the draft and return control to the editor.
    WorkflowEditorWidget unavailable(temporary.path(), temporary.filePath("missing-cli"));
    unavailable.show();
    if (!waitUntil([&]() { return containsLabel(&unavailable, "Cannot start Dockpipe"); }))
        return 3;
    if (!unavailable.findChild<QPlainTextEdit *>("workflowYaml")->toPlainText().contains("my-workflow"))
        return 4;
    unavailable.hide();

#ifdef Q_OS_UNIX
    QFile stalled(temporary.filePath("stalled-cli"));
    if (!stalled.open(QIODevice::WriteOnly))
        return 20;
    stalled.write("#!/bin/sh\nexec sleep 30\n");
    stalled.close();
    stalled.setPermissions(QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    WorkflowEditorWidget cancellable(temporary.path(), stalled.fileName());
    cancellable.show();
    if (!waitUntil([&]() { return button(&cancellable, "Cancel operation")->isVisible(); }))
        return 21;
    button(&cancellable, "Cancel operation")->click();
    if (!waitUntil([&]() { return containsLabel(&cancellable, "Operation cancelled"); }))
        return 22;
    cancellable.hide();
#endif

    const QString cli = qEnvironmentVariable("DOCKPIPE_STUDIO_TEST_CLI");
    if (cli.isEmpty()) {
        std::puts("Canvas and CLI failure checks passed; set DOCKPIPE_STUDIO_TEST_CLI for real CLI round-trip/save checks.");
        return 0;
    }
    const QString path = temporary.filePath("config.yml");
    QFile file(path);
    if (!file.open(QIODevice::WriteOnly))
        return 5;
    file.write(R"YAML(# Keep this workflow note.
name: development
description: Build, verify, and launch your project.
docker_preflight: false
steps:
  - id: Prepare workspace
    kind: host
    cmd: echo Preparing workspace
    cwd: repo
  - group:
      mode: async
      tasks:
        - id: Unit tests
          kind: host
          cmd: echo Run unit tests here
        - id: Check formatting
          kind: host
          cmd: echo Check formatting here
  - id: Start application
    kind: host
    cmd: echo Start your application here
    vars:
      APP_ENV: development
)YAML");
    file.close();
    WorkflowEditorWidget editor(temporary.path(), cli);
    editor.resize(1320, 840);
    editor.openWorkflow(path);
    editor.show();
    auto *yaml = editor.findChild<QPlainTextEdit *>("workflowYaml");
    auto *view = editor.findChild<WorkflowCanvas *>("workflowCanvas");
    if (!waitUntil([&]() { return containsLabel(&editor, "3 steps"); }))
        return 6;
    view->fitDocument();
    if (!selectCard(view, {"steps", "0"}))
        return 7;
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_STUDIO_SCREENSHOT");
    if (!screenshot.isEmpty())
        editor.grab().save(screenshot);
    auto *workflowFiles = editor.findChild<ProjectFilesWidget *>();
    auto *filesToggle = editor.findChild<QPushButton *>("workflowFilesToggle");
    filesToggle->click();
    if (!workflowFiles->isVisible() || !workflowFiles->openFile(path))
        return 36;
    const QString filesScreenshot = qEnvironmentVariable("DOCKPIPE_WORKFLOW_FILES_SCREENSHOT");
    if (!filesScreenshot.isEmpty()) {
        QApplication::processEvents();
        editor.grab().save(filesScreenshot);
    }
    int fileRouteRuns = 0;
    QObject::connect(&editor, &WorkflowEditorWidget::runRequested, &editor,
        [&](const QString &) { ++fileRouteRuns; });
    auto *openWorkflow = workflowFiles->findChild<QPushButton *>("fileOpenWorkflow");
    if (!openWorkflow || !openWorkflow->isVisible())
        return 43;
    openWorkflow->click();
    if (fileRouteRuns != 0 || !view->isVisible() || editor.workflowPath() != path)
        return 44;
    if (!waitUntil([&]() { return button(&editor, "Save")->isEnabled(); }) || !selectCard(view, {"steps", "0"}))
        return 37;
    editor.resize(1000, 700);
    QApplication::processEvents();
    if (editor.width() > 1020 || !editor.findChild<QPushButton *>("applyStep")->isVisible())
        return 29;
    editor.resize(1320, 840);
    if (!selectCard(view, {"steps", "1"}))
        return 23;
    editor.findChild<QAction *>("addCommand")->trigger();
    if (!waitUntil([&]() { return yaml->toPlainText().contains("echo ready") && button(&editor, "Save")->isEnabled(); }))
        return 24;
    if (!selectCard(view, {"steps", "1", "group", "tasks", "2"}))
        return 25;
    button(&editor, "Remove step")->click();
    if (!waitUntil([&]() { return !yaml->toPlainText().contains("echo ready") && button(&editor, "Save")->isEnabled(); }))
        return 26;
    if (!selectCard(view, {"steps", "0"}))
        return 27;
    auto *commandEdit = editor.findChild<QPlainTextEdit *>("stepCommand");
    commandEdit->setPlainText("echo changed");
    button(&editor, "Save")->click();
    if (!containsLabel(&editor, "Apply your inspector changes"))
        return 8;
    editor.findChild<QPushButton *>("applyStep")->click();
    if (!waitUntil([&]() { return yaml->toPlainText().contains("echo changed") && button(&editor, "Save")->isEnabled(); }))
        return 9;
    if (!yaml->toPlainText().contains("# Keep this workflow note."))
        return 10;
    int saves = 0;
    int runs = 0;
    QObject::connect(&editor, &WorkflowEditorWidget::workflowSaved, [&saves](const QString &) { ++saves; });
    QObject::connect(&editor, &WorkflowEditorWidget::runRequested, [&runs](const QString &) { ++runs; });
    button(&editor, "Save")->click();
    if (!waitUntil([&]() { return saves == 1; }))
        return 11;
    if (!file.open(QIODevice::ReadOnly))
        return 12;
    const QByteArray saved = file.readAll();
    file.close();
    if (!saved.contains("echo changed") || runs != 0)
        return 13;
    button(&editor, "Run")->click();
    if (!waitUntil([&]() { return runs == 1; }))
        return 14;

    // Invalid YAML never replaces the existing file or emits a run request.
    editor.findChild<QTabWidget *>()->setCurrentIndex(1);
    yaml->setPlainText("name: invalid\nsteps: [{kind: impossible}]\n");
    button(&editor, "Run")->click();
    if (!waitUntil([&]() { return button(&editor, "Save")->isEnabled(); }))
        return 15;
    if (!file.open(QIODevice::ReadOnly) || file.readAll() != saved || runs != 1)
        return 16;
    file.close();
    yaml->setPlainText(QString::fromUtf8(saved));
    // External changes must not be overwritten.
    if (!file.open(QIODevice::WriteOnly | QIODevice::Truncate))
        return 17;
    file.write("name: external\nsteps: []\n");
    file.close();
    button(&editor, "Save")->click();
    if (!containsLabel(&editor, "changed on disk") || saves != 2)
        return 18;

    // Flatpak authoring browses the workspace instead of creating a separate
    // document grant for the script. Exercise the actual picker and YAML edit.
    QFile script(temporary.filePath("convert obs.sh"));
    if (!script.open(QIODevice::WriteOnly))
        return 30;
    script.write("echo ready\n");
    script.close();
    WorkflowEditorWidget scriptEditor(temporary.path(), cli);
    scriptEditor.show();
    if (!waitUntil([&]() { return button(&scriptEditor, "Save")->isEnabled(); }))
        return 31;
    const QByteArray previousFlatpak = qgetenv("FLATPAK_ID");
    qputenv("FLATPAK_ID", "com.dockpipe.PathTest");
    bool workspacePicker = false;
    QTimer::singleShot(0, &scriptEditor, [&]() {
        auto *dialog = scriptEditor.findChild<QFileDialog *>();
        if (!dialog)
            return;
        workspacePicker = dialog->testOption(QFileDialog::DontUseNativeDialog);
        dialog->selectFile(script.fileName());
        QMetaObject::invokeMethod(dialog, "accept", Qt::DirectConnection);
    });
    QTimer::singleShot(2000, &scriptEditor, [&]() {
        if (auto *dialog = scriptEditor.findChild<QFileDialog *>())
            dialog->reject();
    });
    scriptEditor.findChild<QAction *>("addScript")->trigger();
    if (previousFlatpak.isNull())
        qunsetenv("FLATPAK_ID");
    else
        qputenv("FLATPAK_ID", previousFlatpak);
    auto *scriptYaml = scriptEditor.findChild<QPlainTextEdit *>("workflowYaml");
    if (!workspacePicker || !waitUntil([&]() { return scriptYaml->toPlainText().contains("convert obs.sh"); }))
        return 32;
    if (scriptYaml->toPlainText().contains(temporary.path()))
        return 33;
    std::puts("Workflow studio real-CLI editing, comment preservation, save, validation, conflict and run routing passed.");
    return 0;
}
