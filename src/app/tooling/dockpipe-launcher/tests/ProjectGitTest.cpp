#include "ProjectGitWidget.h"
#include "Theme.h"
#include "GitCommitQueueModel.h"
#include "GitHistoryWidget.h"
#include <QTabWidget>
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
    auto *diff = widget.findChild<QPlainTextEdit *>("projectGitDiff");
    auto *commit = widget.findChild<QPushButton *>("gitCommitSelected");
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
            item->setCheckState(Qt::Checked);
        }
    }
    message->setText("Add script");
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_GIT_SCREENSHOT");
    if (!screenshot.isEmpty()) {
        QApplication::processEvents();
        widget.grab().save(screenshot);
    }
    commit->click();
    if (!waitForGit(widget) || files->count() != 1 || !message->text().isEmpty())
        return 4;
    QByteArray committed;
    if (!git(root.path(), {"ls-tree", "--name-only", "HEAD"}, &committed) || committed.trimmed() != "script.py")
        return 5;
    files->item(0)->setCheckState(Qt::Checked);
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
    widget.findChild<QPushButton *>("gitFile-stage")->click();
    if (!waitForGit(widget))
        return 31;
    message->setText("Commit staged notes");
    widget.findChild<QPushButton *>("gitCommitStaged")->click();
    if (!waitForGit(widget) || files->count() != 0)
        return 32;
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
    auto *detail = history->findChild<QPlainTextEdit *>("gitHistoryDetail");
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
    if (!screenshot.isEmpty())
        widget.grab().save(screenshot + ".history.png");
    QTimer::singleShot(0, [] {
        if (auto *dialog = qobject_cast<QMessageBox *>(QApplication::activeModalWidget()))
            dialog->done(QMessageBox::Yes);
    });
    history->findChild<QPushButton *>("gitQueueStart")->click();
    if (!waitForGit(widget) || history->queueModel()->rowCount() != 0
        || !git(root.path(), {"ls-tree", "--name-only", "HEAD"}, &committed)
        || !committed.contains("incoming.txt"))
        return 37;
    std::puts("Project Git UI passed selected/staged commits, stale review rejection, history search and queued cherry-pick.");
    return 0;
}
