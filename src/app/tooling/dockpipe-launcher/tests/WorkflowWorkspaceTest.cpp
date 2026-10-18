#include "MainWindow.h"
#include "ActivityWidget.h"
#include <QTableWidget>
#include "LauncherSettings.h"
#include "WorkflowCanvas.h"
#include "CanvasPanel.h"
#include <QMouseEvent>
#include "WorkflowEditorWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QDialog>
#include <QDir>
#include <QElapsedTimer>
#include <QFile>
#include <QGraphicsScene>
#include <QKeyEvent>
#include <QLabel>
#include <QListWidget>
#include <QMessageBox>
#include <QMenuBar>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QProxyStyle>
#include <QStyle>
#include <QTabWidget>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <QToolButton>
#include <cstdio>
#include <functional>

namespace {
// Exercise desktops that request stock icons on standard dialog buttons.
class StockDialogIconStyle final : public QProxyStyle {
public:
    StockDialogIconStyle() : QProxyStyle("Fusion") {}

    int styleHint(StyleHint hint, const QStyleOption *option = nullptr,
                  const QWidget *widget = nullptr, QStyleHintReturn *data = nullptr) const override
    {
        if (hint == SH_DialogButtonBox_ButtonsHaveIcons)
            return 1;
        return QProxyStyle::styleHint(hint, option, widget, data);
    }
};

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

QPushButton *button(QWidget *widget, const QString &text)
{
    for (auto *candidate : widget->findChildren<QPushButton *>()) {
        if (candidate->text() == text)
            return candidate;
    }
    return nullptr;
}

bool containsLabel(QWidget *widget, const QString &text)
{
    for (auto *label : widget->findChildren<QLabel *>()) {
        if (label->text().contains(text))
            return true;
    }
    return false;
}

bool writeFile(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(bytes) == bytes.size();
}
}

int main(int argc, char **argv)
{
    QTemporaryDir temporary;
    qputenv("XDG_CONFIG_HOME", temporary.filePath("settings").toUtf8());
    qputenv("DOCKPIPE_GLOBAL_ROOT", temporary.filePath("global").toUtf8());
    qputenv("DOCKPIPE_TEST_ROOT", temporary.path().toUtf8());
    const QString cli = temporary.filePath("dockpipe");
    if (!writeFile(cli, R"SH(#!/bin/sh
case "$1:$2" in
catalog:list)
    printf '{"workflows":[{"workflow_id":"demo","display_name":"Development","config_path":"%s/config.yml"}]}' "$DOCKPIPE_TEST_ROOT" ;;
workflow:document|workflow:validate)
    if [ -n "$DOCKPIPE_STUDIO_TEST_CLI" ]; then
        exec "$DOCKPIPE_STUDIO_TEST_CLI" "$@"
    fi
    echo '{"source":"name: my-workflow\ndescription: \"\"\ndocker_preflight: false\nsteps: []\n","document":{"name":"my-workflow","steps":[]}}' ;;
--workflow-file:*|--workflow:*)
    printf 'Preparing workspace\nRunning checks\nApplication ready\n' ;;
*) echo '{}' ;;
esac
)SH"))
        return 1;
    QFile::setPermissions(cli, QFile::ReadOwner | QFile::WriteOwner | QFile::ExeOwner);
    qputenv("DOCKPIPE_BIN", cli.toUtf8());
    qputenv("PATH", temporary.path().toUtf8() + ":" + qgetenv("PATH"));
    QApplication app(argc, argv);
    app.setStyle(new StockDialogIconStyle);
    app.setOrganizationName("DockpipeWorkspaceTest");
    app.setApplicationName("CanvasWorkspaceTest");
    applyDockpipeLauncherTheme(app);
    if (qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light") {
        QFile shared(":/theme/pipeon.qss");
        QFile light(":/theme/pipeon-light.qss");
        if (!shared.open(QIODevice::ReadOnly) || !light.open(QIODevice::ReadOnly))
            return 2;
        app.setStyleSheet(QString());
        app.setPalette(app.style()->standardPalette());
        app.setStyleSheet(QString::fromUtf8(shared.readAll() + light.readAll()));
    }
    LauncherSettings settings;
    settings.projectFolder = temporary.path();
    settings.uiMode = "advanced";
    settings.thirdPartyDisclaimerDismissed = true;
    settings.save();
    const QString path = temporary.filePath("config.yml");
    if (!writeFile(path, R"YAML(name: development
description: Prepare, verify, and start your application.
docker_preflight: false
steps:
  - id: Prepare workspace
    kind: host
    cmd: echo preparing
  - id: Run checks
    kind: host
    cmd: echo checking
  - id: Start application
    kind: host
    cmd: echo ready
)YAML"))
        return 3;

    MainWindow window;
    window.resize(1380, 900);
    window.show();
#ifndef Q_OS_MACOS
    if (!(window.windowFlags() & Qt::FramelessWindowHint)
        || !window.findChild<QToolButton *>("windowClose")
        || !window.findChild<QToolButton *>("windowClose")->isVisible())
        return 28;
    QToolButton *maximize = nullptr;
    for (auto *control : window.findChildren<QToolButton *>("windowControl")) {
        if (control->accessibleName() == "Maximize or restore")
            maximize = control;
    }
    if (!maximize)
        return 30;
    maximize->click();
    if (!window.isMaximized())
        return 31;
    maximize->click();
    if (window.isMaximized())
        return 32;
    QApplication::processEvents();
    window.activateWindow();
    QApplication::processEvents();
    auto *menu = window.menuBar();
    const QPoint start = window.pos();
    const QPoint local(500, 20);
    const QPoint global = menu->mapToGlobal(local);
    QMouseEvent press(QEvent::MouseButtonPress, local, global,
        Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(menu, &press);
    QMouseEvent move(QEvent::MouseMove, local + QPoint(20, 20), global + QPoint(20, 20),
        Qt::NoButton, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(menu, &move);
    QMouseEvent release(QEvent::MouseButtonRelease, local + QPoint(20, 20), global + QPoint(20, 20),
        Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(menu, &release);
    if (window.pos() != start + QPoint(20, 20)) {
        std::fprintf(stderr, "Header drag: (%d,%d) -> (%d,%d)\n",
            start.x(), start.y(), window.x(), window.y());
        return 33;
    }
#endif
    auto *editor = window.findChild<WorkflowEditorWidget *>();
    auto *navigation = window.findChild<QListWidget *>("workspaceNavigation");
    auto *output = window.findChild<QPlainTextEdit *>("inlineConsole");
    auto *toggle = window.findChild<QPushButton *>("workflowConsoleToggle");
    auto *properties = window.findChild<QPushButton *>("workflowPropertiesButton");
    if (!editor || editor->isWindow() || !output || !toggle || !properties)
        return 4;
    if (!waitUntil([&]() { return containsLabel(editor, "0 steps") && navigation->currentRow() == 2; }))
        return 5;
    if (output->isVisible() || properties->isChecked() || !editor->workflowPath().isEmpty())
        return 6;
    const auto capture = [&](const QString &suffix) {
        QApplication::processEvents();
        const QString prefix = qEnvironmentVariable("DOCKPIPE_UI_CAPTURE");
        if (!prefix.isEmpty())
            window.grab().save(prefix + suffix + ".png");
    };
    auto *canvas = editor->findChild<WorkflowCanvas *>();
    if (canvas->width() < editor->width() - 4 || canvas->height() < editor->height() - 4)
        return 17;
    const QSize canvasSize = canvas->size();
    capture("-blank");
    toggle->click();
    if (canvas->size() != canvasSize)
        return 18;
    if (!output->isVisible())
        return 7;
    button(window.findChild<QWidget *>("workflowTerminalPanel"), "Hide")->click();
    if (output->isVisible() || toggle->isChecked())
        return 8;
    window.activateWindow();
    editor->findChild<WorkflowCanvas *>()->setFocus();
    QApplication::processEvents();
    QKeyEvent shortcut(QEvent::KeyPress, Qt::Key_QuoteLeft, Qt::ControlModifier);
    QApplication::sendEvent(editor->findChild<WorkflowCanvas *>(), &shortcut);
    if (!waitUntil([&]() { return output->isVisible(); }))
        return 9;
    toggle->click();

    if (qEnvironmentVariableIsEmpty("DOCKPIPE_STUDIO_TEST_CLI")) {
        std::puts("Embedded blank canvas, hidden panels and output shortcut passed; set DOCKPIPE_STUDIO_TEST_CLI for picker/run integration.");
        return 0;
    }
    bool picked = false;
    QTimer::singleShot(0, &window, [&]() {
        auto *picker = window.findChild<QDialog *>("workflowPicker");
        auto *list = window.findChild<QListWidget *>("workflowPickerList");
        if (picker && waitUntil([&]() { return list && list->count() == 1; })) {
            list->setCurrentRow(0);
            capture("-picker");
            const QString prefix = qEnvironmentVariable("DOCKPIPE_UI_CAPTURE");
            if (!prefix.isEmpty())
                picker->grab().save(prefix + "-picker-dialog.png");
            window.findChild<QPushButton *>("openWorkflowOnCanvas")->click();
            picked = editor->workflowPath() == path;
        }
        if (picker && picker->isVisible())
            picker->reject();
    });
    window.findChild<QPushButton *>("workflowPickerButton")->click();
    if (!picked || !waitUntil([&]() { return containsLabel(editor, "3 steps"); }))
        return 10;
    bool safeClose = false;
    QTimer::singleShot(0, &window, [&]() {
        auto *message = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        if (!message)
            return;
        auto *close = message->findChild<QToolButton *>("windowClose");
        auto *heading = message->findChild<QLabel *>("dialogHeading");
        const QString prefix = qEnvironmentVariable("DOCKPIPE_UI_CAPTURE");
        if (!prefix.isEmpty())
            message->grab().save(prefix + "-confirmation.png");
        safeClose = close && heading && heading->text() == "Unsaved workflow";
        if (close)
            close->click();
        else
            message->reject();
    });
    const auto answer = QMessageBox::question(&window, "Unsaved workflow",
        "Discard the unsaved changes to this workflow?",
        QMessageBox::Discard | QMessageBox::Cancel, QMessageBox::Cancel);
    if (!safeClose || answer != QMessageBox::Cancel)
        return 29;
    bool aboutButtonIsClean = false;
    QTimer::singleShot(0, &window, [&]() {
        auto *about = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        if (!about)
            return;
        auto *ok = about->button(QMessageBox::Ok);
        aboutButtonIsClean = ok && ok->icon().isNull();
        const QString prefix = qEnvironmentVariable("DOCKPIPE_UI_CAPTURE");
        if (!prefix.isEmpty())
            about->grab().save(prefix + "-about.png");
        if (ok)
            ok->click();
        else
            about->reject();
    });
    if (!QMetaObject::invokeMethod(&window, "onAbout", Qt::DirectConnection)
        || !aboutButtonIsClean)
        return 34;
    editor->findChild<WorkflowCanvas *>()->fitDocument();
    capture("-loaded");
    button(editor, "Run")->click();
    if (!waitUntil([&]() { return output->toPlainText().contains("Application ready"); }))
        return 11;
    if (navigation->currentRow() != 2 || !output->isVisible() || editor->workflowPath() != path)
        return 12;
    capture("-output");
    window.resize(1180, 780);
    QApplication::processEvents();
    if (window.width() > 1180 || editor->findChild<WorkflowCanvas *>()->height() < 180)
        return 13;
    capture("-compact");
    properties->click();
    QApplication::processEvents();
    auto *inspector = editor->findChild<QWidget *>("studioSidebar");
    auto *terminal = editor->findChild<QWidget *>("workflowTerminalPanel");
    if (!inspector->isVisible() || !editor->rect().contains(inspector->geometry())
        || terminal->geometry().intersects(inspector->geometry()))
        return 19;
    capture("-properties");
    auto *panel = static_cast<CanvasPanel *>(inspector);
    auto *handle = panel->findChild<QWidget *>("canvasPropertiesTitleBar");
    const auto drag = [](QWidget *target, const QPoint &local, const QPoint &delta) {
        const QPoint global = target->mapToGlobal(local);
        QMouseEvent press(QEvent::MouseButtonPress, local, global, Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(target, &press);
        QMouseEvent move(QEvent::MouseMove, local + delta, global + delta, Qt::NoButton, Qt::LeftButton, Qt::NoModifier);
        QApplication::sendEvent(target, &move);
        QMouseEvent release(QEvent::MouseButtonRelease, local + delta, global + delta, Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
        QApplication::sendEvent(target, &release);
    };
    const QRect original = panel->geometry();
    drag(handle, QPoint(15, 15), QPoint(-140, -20));
    if (panel->pos() == original.topLeft() || !editor->rect().contains(panel->geometry()))
        return 20;
    const QRect moved = panel->geometry();
    properties->click();
    properties->click();
    if (panel->geometry() != moved)
        return 21;
    drag(panel, QPoint(panel->width() - 3, panel->height() - 3), QPoint(100, -40));
    if (panel->size() == moved.size() || !editor->rect().contains(panel->geometry()))
        return 22;
    capture("-moved");
    const QRect resized = panel->geometry();
    {
        CanvasPanel restored("properties", editor);
        restored.placeDefault(QRect(16, 16, 300, 300));
        if (restored.geometry() != resized)
            return 24;
    }
    drag(panel, QPoint(2, 2), QPoint(2000, 2000));
    if (panel->geometry().bottomRight() != resized.bottomRight()
        || panel->width() < 280 || panel->height() < 180)
        return 25;
    auto *outputPanel = static_cast<CanvasPanel *>(terminal);
    const QRect outputOriginal = outputPanel->geometry();
    drag(outputPanel->findChild<QWidget *>("canvasOutputTitleBar"), QPoint(15, 15), QPoint(70, -90));
    if (outputPanel->pos() == outputOriginal.topLeft())
        return 26;
    const QRect outputMoved = outputPanel->geometry();
    toggle->click();
    toggle->click();
    if (outputPanel->geometry() != outputMoved)
        return 27;
    window.resize(980, 700);
    QApplication::processEvents();
    if (!editor->rect().contains(panel->geometry()))
        return 23;
    panel->resetPlacement();
    outputPanel->resetPlacement();
    properties->click();
    navigation->setCurrentRow(1);
    navigation->setCurrentRow(2);
    if (editor->workflowPath() != path)
        return 14;

    // Cancelled switches must preserve both the file and unapplied YAML draft.
    editor->findChild<QTabWidget *>()->setCurrentIndex(1);
    auto *yaml = editor->findChild<QPlainTextEdit *>("workflowYaml");
    const QString draft = yaml->toPlainText() + "# draft retained\n";
    yaml->setPlainText(draft);
    bool quickRan = false;
    QTimer::singleShot(0, &window, [&]() {
        auto *picker = window.findChild<QDialog *>("workflowPicker");
        auto *list = window.findChild<QListWidget *>("workflowPickerList");
        auto *run = window.findChild<QPushButton *>("runPickedWorkflow");
        if (picker && waitUntil([&]() { return list && list->count() == 1 && run->isEnabled(); })) {
            list->setCurrentRow(0);
            quickRan = true;
            run->click();
        }
        if (picker && picker->isVisible())
            picker->reject();
    });
    window.findChild<QPushButton *>("workflowPickerButton")->click();
    if (!quickRan || yaml->toPlainText() != draft || editor->workflowPath() != path
        || !waitUntil([&]() { return output->toPlainText().contains("Application ready"); }))
        return 35;
    window.findChild<QPushButton *>("workflowActivityButton")->click();
    auto *activityPanel = editor->findChild<CanvasPanel *>("workflowActivityPanel");
    if (!activityPanel || !activityPanel->isVisible()
        || !editor->rect().contains(activityPanel->geometry()))
        return 36;
    auto *activity = activityPanel->findChild<ActivityWidget *>();
    auto *localRuns = activity->findChild<QTableWidget *>("localActivityRuns");
    if (!localRuns || localRuns->rowCount() < 1)
        return 37;
    activity->setWorkdir(temporary.filePath("other"));
    activity->refreshLocal();
    if (localRuns->rowCount() != 0)
        return 38;
    activity->setWorkdir(temporary.path());
    activity->refreshLocal();
    capture("-activity");
    activityPanel->hide();
    const auto cancelDiscard = [&]() {
        QTimer::singleShot(0, &window, [&]() {
            if (auto *message = qobject_cast<QMessageBox *>(QApplication::activeModalWidget()))
                message->done(QMessageBox::Cancel);
        });
    };
    cancelDiscard();
    if (editor->openWorkflow(path) || yaml->toPlainText() != draft)
        return 15;
    cancelDiscard();
    if (editor->setWorkspace(temporary.filePath("other"), cli) || editor->workdir() != temporary.path())
        return 16;
    std::puts("Workspace picker, canvas run/output, responsive layout and cancelled draft switches passed.");
    return 0;
}
