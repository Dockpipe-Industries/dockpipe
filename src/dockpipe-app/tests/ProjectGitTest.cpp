#include "ProjectGitWidget.h"
#include "GitDiffWidget.h"
#include "Theme.h"
#include "GitCommitQueueModel.h"
#include "GitHistoryWidget.h"
#include <QTabWidget>
#include <QToolButton>
#include <QTreeWidget>
#include <QTreeWidgetItemIterator>
#include <QDir>
#include <QMessageBox>
#include <QTimer>
#include <QApplication>
#include <QStyle>
#include <QElapsedTimer>
#include <QFile>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QPlainTextEdit>
#include <QProcess>
#include <QPushButton>
#include <QTemporaryDir>
#include <QThread>
#include <cstdio>

namespace {
bool waitForGit(ProjectGitWidget &widget)
{
    QElapsedTimer timer;
    timer.start();
    while (widget.busy() && timer.elapsed() < 10000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    return !widget.busy();
}

bool git(const QString &root, const QStringList &arguments, QByteArray *output = nullptr)
{
    QProcess process;
    process.start("git", QStringList{"-C", root} + arguments);
    if (!process.waitForFinished(5000) || process.exitCode() != 0)
        return false;
    if (output)
        *output = process.readAllStandardOutput();
    return true;
}

bool writeFile(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly | QIODevice::Truncate) && file.write(bytes) == bytes.size();
}
}

int main(int argc, char **argv)
{
    QTemporaryDir configuration;
    qputenv("XDG_CONFIG_HOME", configuration.path().toUtf8());
    QApplication app(argc, argv);
    app.setOrganizationName("DockpipeGitTests");
    app.setApplicationName("ProjectGitTest");
    applyDockpipeLauncherTheme(app);
    previewDockpipeLauncherTheme(app, qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
        ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    const QString cli = qEnvironmentVariable("DOCKPIPE_GIT_TEST_CLI");
    if (cli.isEmpty()) {
        std::puts("Set DOCKPIPE_GIT_TEST_CLI to run real project Git UI actions in an isolated repository.");
        return 77;
    }
    qputenv("DOCKPIPE_BIN", cli.toUtf8());
    QTemporaryDir root;
    if (!git(root.path(), {"init"}) || !git(root.path(), {"config", "user.name", "Fixture"})
        || !git(root.path(), {"config", "user.email", "fixture@example.invalid"})
        || !git(root.path(), {"config", "commit.gpgsign", "false"})
        || !writeFile(root.filePath("script.py"), "print('hello')\n")
        || !writeFile(root.filePath("notes.txt"), "Keep this file uncommitted.\n"))
        return 1;
    ProjectGitWidget widget;
    widget.resize(1180, 760);
    widget.show();
    widget.setWorkdir(root.path());
    widget.refresh();
    auto *files = widget.findChild<QListWidget *>("gitChangedFiles");
    auto *staged = widget.findChild<QListWidget *>("gitStagedFiles");
    auto *diff = widget.findChild<GitDiffWidget *>("projectGitDiff")->findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    auto *commit = widget.findChild<QPushButton *>("gitCommitStaged");
    auto *message = widget.findChild<QLineEdit *>("gitCommitMessage");
    if (!waitForGit(widget) || files->count() != 2 || !diff->toPlainText().contains("Keep this file"))
        return 2;
    commit->click();
    if (widget.busy())
        return 3; // Incomplete selections never start a write.
    for (int row = 0; row < files->count(); ++row) {
        auto *item = files->item(row);
        if (item->data(Qt::UserRole).toString() == "script.py")
        {
            files->setCurrentItem(item);
            if (!waitForGit(widget) || !diff->toPlainText().contains("print('hello')")
                || diff->toPlainText().contains("Keep this file"))
                return 9;
            auto *filter = widget.findChild<QLineEdit *>("gitFileFilter");
            filter->setText("script");
            auto *stageShown = widget.findChild<QPushButton *>("gitStageAll");
            if (stageShown->text() != "Stage shown")
                return 52;
            stageShown->click();
            if (!waitForGit(widget) || staged->count() != 1 || files->count() != 1
                || files->item(0)->data(Qt::UserRole).toString() != "notes.txt")
                return 41;
            filter->clear();
            break;
        }
    }
    if (!writeFile(root.filePath("script.py"), "print('newer unstaged edit')\n"))
        return 42;
    widget.refresh();
    if (!waitForGit(widget) || files->count() != 2 || staged->count() != 1)
        return 43;
    for (int row = 0; row < files->count(); ++row) {
        if (files->item(row)->data(Qt::UserRole).toString() == "script.py")
            files->setCurrentRow(row);
    }
    if (!waitForGit(widget) || !diff->toPlainText().contains("newer unstaged edit"))
        return 44;
    const QString diffScreenshot = qEnvironmentVariable("DOCKPIPE_GIT_SCREENSHOT");
    if (!diffScreenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(diffScreenshot + ".modified.png");
    }
    staged->setCurrentRow(0);
    if (!waitForGit(widget) || !diff->toPlainText().contains("print('hello')")
        || diff->toPlainText().contains("newer unstaged edit"))
        return 45;
    message->setText("Add script");
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_GIT_SCREENSHOT");
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(screenshot);
        widget.resize(1000, 760);
        QApplication::processEvents();
        if (widget.width() > 1000)
            return 39;
        widget.grab().save(screenshot + ".compact.png");
        widget.resize(1180, 760);
    }
    commit->click();
    if (!waitForGit(widget) || files->count() != 2 || staged->count() != 0 || !message->text().isEmpty())
        return 4;
    QByteArray committed;
    if (!git(root.path(), {"ls-tree", "--name-only", "HEAD"}, &committed) || committed.trimmed() != "script.py")
        return 5;
    if (!git(root.path(), {"show", "HEAD:script.py"}, &committed) || committed != "print('hello')\n")
        return 46;
    if (!writeFile(root.filePath("script.py"), "print('hello')\n"))
        return 47;
    widget.refresh();
    if (!waitForGit(widget) || files->count() != 1)
        return 48;
    files->setCurrentRow(0);
    if (!waitForGit(widget))
        return 49;
    widget.findChild<QPushButton *>("gitFile-stage")->click();
    if (!waitForGit(widget) || staged->count() != 1)
        return 50;
    message->setText("Stale draft");
    if (!writeFile(root.filePath("notes.txt"), "Changed after review.\n"))
        return 6;
    commit->click();
    if (!waitForGit(widget) || message->text() != "Stale draft")
        return 7;
    bool rejected = false;
    for (auto *label : widget.findChildren<QLabel *>())
        rejected |= label->text().contains("changed since review");
    if (!rejected || !git(root.path(), {"ls-tree", "--name-only", "HEAD"}, &committed)
        || committed.trimmed() != "script.py")
        return 8;
    widget.refresh();
    if (!waitForGit(widget))
        return 30;
    widget.findChild<QPushButton *>("gitUnstageAll")->click();
    if (!waitForGit(widget) || staged->count() != 0 || files->count() != 1)
        return 31;
    widget.findChild<QPushButton *>("gitStageAll")->click();
    if (!waitForGit(widget) || staged->count() != 1 || files->count() != 0)
        return 51;
    message->setText("Commit staged notes");
    widget.findChild<QPushButton *>("gitCommitStaged")->click();
    if (!waitForGit(widget) || files->count() != 0)
        return 32;
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(screenshot + ".clean.png");
        auto *tabs = widget.findChild<QTabWidget *>("gitTabs");
        tabs->setCurrentWidget(widget.findChild<QWidget *>("gitRepository"));
        QApplication::processEvents();
        widget.grab().save(screenshot + ".repository.png");
        tabs->setCurrentIndex(0);
    }
    QByteArray targetBranch;
    if (!git(root.path(), {"branch", "--show-current"}, &targetBranch)
        || !git(root.path(), {"switch", "-c", "incoming"})
        || !writeFile(root.filePath("incoming.txt"), "Cherry-picked through the UI.\n")
        || !git(root.path(), {"add", "incoming.txt"})
        || !git(root.path(), {"commit", "-m", "Incoming searchable feature"})
        || !git(root.path(), {"switch", QString::fromUtf8(targetBranch).trimmed()}))
        return 33;
    widget.refresh();
    if (!waitForGit(widget))
        return 34;
    auto *history = widget.findChild<GitHistoryWidget *>();
    widget.findChild<QTabWidget *>("gitTabs")->setCurrentWidget(history);
    history->findChild<QLineEdit *>("gitHistoryQuery")->setText("Incoming searchable");
    history->findChild<QPushButton *>("gitHistorySearch")->click();
    auto *commits = history->findChild<QListWidget *>("gitHistoryCommits");
    QElapsedTimer historyTimer;
    historyTimer.start();
    while ((commits->count() == 0 || !history->findChild<QPushButton *>("gitHistorySearch")->isEnabled())
           && historyTimer.elapsed() < 10000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (commits->count() != 1)
        return 35;
    commits->setCurrentRow(0);
    historyTimer.restart();
    auto *detail = history->findChild<GitDiffWidget *>("gitHistoryDetail")->findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    while ((!detail->toPlainText().contains("Cherry-picked through the UI")
            || !history->findChild<QPushButton *>("gitHistorySearch")->isEnabled()) && historyTimer.elapsed() < 10000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (!detail->toPlainText().contains("Cherry-picked through the UI"))
        return 38;
    commits->item(0)->setSelected(true);
    history->findChild<QPushButton *>("gitHistoryAdd")->click();
    if (history->queueModel()->rowCount() != 1)
        return 36;
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        if (!history->findChild<QWidget *>("gitCherryPickPanel")->isVisible())
            return 40;
        widget.grab().save(screenshot + ".history.png");
    }
    QTimer::singleShot(0, [] {
        if (auto *dialog = qobject_cast<QMessageBox *>(QApplication::activeModalWidget()))
            dialog->done(QMessageBox::Yes);
    });
    history->findChild<QPushButton *>("gitQueueStart")->click();
    if (!waitForGit(widget) || history->queueModel()->rowCount() != 0
        || !git(root.path(), {"ls-tree", "--name-only", "HEAD"}, &committed)
        || !committed.contains("incoming.txt"))
        return 37;
    if (!git(root.path(), {"switch", "-c", "graph-feature"})
        || !writeFile(root.filePath("feature.txt"), "Feature branch work.\n")
        || !git(root.path(), {"add", "feature.txt"})
        || !git(root.path(), {"commit", "-m", "Add feature branch work"})
        || !git(root.path(), {"switch", QString::fromUtf8(targetBranch).trimmed()})
        || !writeFile(root.filePath("main.txt"), "Parallel main branch work.\n")
        || !git(root.path(), {"add", "main.txt"})
        || !git(root.path(), {"commit", "-m", "Improve main branch"})
        || !git(root.path(), {"merge", "--no-ff", "graph-feature", "-m", "Merge feature branch"}))
        return 53;
    widget.refresh();
    if (!waitForGit(widget))
        return 54;
    history->findChild<QPushButton *>("gitQueueToggle")->setChecked(false);
    history->findChild<QLineEdit *>("gitHistoryQuery")->clear();
    history->findChild<QPushButton *>("gitHistorySearch")->click();
    historyTimer.restart();
    while (!history->findChild<QPushButton *>("gitHistorySearch")->isEnabled()
           && historyTimer.elapsed() < 10000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (commits->count() < 5 || commits->item(0)->data(Qt::UserRole + 2).toMap().isEmpty()
        || !commits->item(0)->data(Qt::UserRole + 5).toStringList().contains("HEAD")
        || commits->item(0)->data(Qt::UserRole + 4).toString().isEmpty())
        return 55;
    commits->setCurrentRow(0);
    historyTimer.restart();
    while (!history->findChild<QPushButton *>("gitHistorySearch")->isEnabled()
           && historyTimer.elapsed() < 10000) {
        QApplication::processEvents();
        QThread::msleep(10);
    }
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(screenshot + ".graph.png");
    }
    if (!QDir(root.path()).mkpath("src/core") || !QDir(root.path()).mkpath("src/ui")
        || !writeFile(root.filePath("src/core/shared.cpp"), "int core_value = 1;\n")
        || !writeFile(root.filePath("src/ui/shared.cpp"), "int ui_value = 2;\n"))
        return 56;
    widget.refresh();
    if (!waitForGit(widget) || files->count() != 2)
        return 57;
    widget.findChild<QTabWidget *>("gitTabs")->setCurrentIndex(0);
    widget.findChild<QToolButton *>("gitFoldersView")->click();
    auto *tree = widget.findChild<QTreeWidget *>("gitUnstagedTree");
    QTreeWidgetItem *uiFile = nullptr;
    for (QTreeWidgetItemIterator iterator(tree); *iterator; ++iterator) {
        if ((*iterator)->data(0, Qt::UserRole).toString() == "src/ui/shared.cpp")
            uiFile = *iterator;
    }
    if (!uiFile || !uiFile->parent() || uiFile->parent()->text(0) != "ui")
        return 58;
    tree->setCurrentItem(uiFile);
    if (!waitForGit(widget) || !diff->toPlainText().contains("ui_value"))
        return 59;
    tree->setCurrentItem(uiFile->parent());
    if (widget.busy() || !diff->toPlainText().contains("ui_value"))
        return 60;
    widget.findChild<QToolButton *>("gitFlatView")->click();
    if (widget.busy() || files->currentItem()->data(Qt::UserRole).toString() != "src/ui/shared.cpp")
        return 61;
    widget.findChild<QToolButton *>("gitFoldersView")->click();
    auto *filter = widget.findChild<QLineEdit *>("gitFileFilter");
    filter->setText("src/core");
    if (!uiFile->isHidden() || !uiFile->parent()->isHidden())
        return 62;
    widget.findChild<QPushButton *>("gitStageAll")->click();
    if (!waitForGit(widget) || staged->count() != 1
        || staged->item(0)->data(Qt::UserRole).toString() != "src/core/shared.cpp"
        || files->count() != 1 || files->item(0)->data(Qt::UserRole).toString() != "src/ui/shared.cpp")
        return 63;
    filter->clear();
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(screenshot + ".folders.png");
    }
    std::puts("Project Git UI passed split staging, index-only commits with newer edits, stage/unstage all, stale review rejection, history search and queued cherry-pick.");
    return 0;
}
