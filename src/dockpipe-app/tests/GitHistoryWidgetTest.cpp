#include "GitHistoryWidget.h"
#include "GitCommitQueueModel.h"
#include "SourceEditor.h"
#include "GitDiffWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QAction>
#include <QClipboard>
#include <QDialog>
#include <QMenu>
#include <QComboBox>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QListView>
#include <QListWidget>
#include <QMessageBox>
#include <QPushButton>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QThread>
#include <QTimer>
#include <QtTest>
#include <cstdio>

namespace {
QString commitId(int value)
{
    return QString::number(value, 16).rightJustified(40, '0');
}

QJsonObject snapshot(const QString &token = "reviewed-snapshot", const QString &branch = "feature")
{
    return {{"snapshot", token}, {"branch", branch},
        {"catalog", QJsonObject{{"branches", QJsonArray{QJsonObject{{"name", "feature"}}, QJsonObject{{"name", "origin/main"}}}}}}};
}

QList<QJsonObject> requests(const QString &path)
{
    QFile file(path);
    QList<QJsonObject> result;
    if (!file.open(QIODevice::ReadOnly))
        return result;
    for (const auto &line : file.readAll().split('\n')) {
        if (!line.isEmpty())
            result.append(QJsonDocument::fromJson(line).object());
    }
    return result;
}

int fakeCli(const QString &requestPath)
{
    QFile input(requestPath);
    if (!input.open(QIODevice::ReadOnly))
        return 2;
    const auto request = QJsonDocument::fromJson(input.readAll()).object();
    QFile log(qEnvironmentVariable("DOCKPIPE_HISTORY_FIXTURE_REQUESTS"));
    if (!log.open(QIODevice::WriteOnly | QIODevice::Append))
        return 3;
    log.write(QJsonDocument(request).toJson(QJsonDocument::Compact) + '\n');
    log.close();
    const QString query = request.value("query").toString();
    if (query == "fail") {
        std::fputs("Fixture read failed\n", stderr);
        return 4;
    }
    if (query == "slow")
        QThread::msleep(1500);
    QJsonObject reply;
    if (request.value("action").toString() == "history") {
        const int offset = request.value("offset").toInt();
        QJsonArray commits;
        for (int row = 1; row <= 3; ++row)
            commits.append(QJsonObject{{"id", commitId(offset + row)}, {"subject", QString("Commit %1 <literal>").arg(offset + row)},
                {"author", "Fixture <author>"}, {"parents", QJsonArray{}}});
        reply["history"] = QJsonObject{{"commits", commits}, {"offset", offset}, {"limit", 50}, {"has_more", offset == 0}};
    } else if (request.value("action").toString() == "commit-review") {
        reply["commit_review"] = QJsonObject{{"commit", QJsonObject{{"id", request.value("commit")}}},
            {"body", "Fixture commit message <literal>"}, {"patch", "diff --git a/a.txt b/a.txt\n--- /dev/null\n+++ b/a.txt\n@@ -0,0 +1 @@\n+fixture content\n"}};
        if (request.value("commit").toString() == commitId(2)) {
            auto review = reply.value("commit_review").toObject();
            review["patch"] = review.value("patch").toString()
                + "diff --git a/src/second.py b/src/second.py\n--- /dev/null\n+++ b/src/second.py\n@@ -0,0 +1 @@\n+second_file = 2\n";
            reply["commit_review"] = review;
        }
        if (request.value("commit").toString() == commitId(3))
            reply["commit_review"] = QJsonObject{{"body", "Large commit"}, {"limited", true}, {"notice", "Fixture patch exceeds the 2 MiB preview limit."}};
    } else {
        return 5; // The child view must never execute a mutation.
    }
    const QByteArray output = QJsonDocument(reply).toJson(QJsonDocument::Compact);
    return std::fwrite(output.constData(), 1, output.size(), stdout) == size_t(output.size()) ? 0 : 6;
}

bool chooseCommitAction(GitHistoryWidget &widget, int row, const QString &name,
    const std::function<void()> &beforeAction = {})
{
    auto *commits = widget.findChild<QListWidget *>("gitHistoryCommits");
    commits->scrollToItem(commits->item(row));
    const QPoint position = commits->visualItemRect(commits->item(row)).center();
    bool found = false;
    QTimer watchdog;
    watchdog.setSingleShot(true);
    QObject::connect(&watchdog, &QTimer::timeout, &widget, [] {
        for (auto *surface : QApplication::topLevelWidgets()) {
            if (auto *menu = qobject_cast<QMenu *>(surface))
                menu->close();
            if (auto *dialog = qobject_cast<QDialog *>(surface))
                dialog->reject();
        }
    });
    watchdog.start(3000);
    QObject timerScope;
    QTimer::singleShot(0, &timerScope, [&] {
        auto *menu = qobject_cast<QMenu *>(QApplication::activePopupWidget());
        if (!menu)
            return;
        auto *action = menu->findChild<QAction *>(name);
        if (!action || !action->isEnabled()) {
            menu->close();
            return;
        }
        found = true;
        if (beforeAction)
            beforeAction();
        QTest::mouseClick(menu, Qt::LeftButton, Qt::NoModifier, menu->actionGeometry(action).center());
    });
    QMetaObject::invokeMethod(commits, "customContextMenuRequested", Qt::DirectConnection, Q_ARG(QPoint, position));
    watchdog.stop();
    return found;
}

void answerConfirmation(const std::function<void()> &beforeAnswer = {})
{
    QTimer::singleShot(0, qApp, [beforeAnswer] {
        for (QWidget *widget : QApplication::topLevelWidgets()) {
            auto *box = qobject_cast<QMessageBox *>(widget);
            if (!box || !box->isVisible())
                continue;
            if (beforeAnswer)
                beforeAnswer();
            box->button(QMessageBox::Yes)->click();
            return;
        }
    });
}
}

class GitHistoryWidgetTest final : public QObject {
    Q_OBJECT
private slots:
    void init();
    void cleanup();
    void searchModesPagingAndDetail();
    void queueOrderingAndRefresh();
    void commitContextActionsTargetClickedRow();
    void contextPreviewShowsAllFiles();
    void changedSnapshotInvalidatesContextAction();
    void confirmedMutationUsesExactSnapshot();
    void changedSnapshotInvalidatesConfirmation();
    void failedReadPreservesQueue();
    void cancellationAndProjectChangeIgnoreLateReply();

private:
    QTemporaryDir m_root;
    QString m_log;
    QByteArray m_previousBinary;
    QByteArray m_previousLog;
};

void GitHistoryWidgetTest::init()
{
    QVERIFY(m_root.isValid());
    m_previousBinary = qgetenv("DOCKPIPE_BIN");
    m_previousLog = qgetenv("DOCKPIPE_HISTORY_FIXTURE_REQUESTS");
    m_log = m_root.filePath(QString(QTest::currentTestFunction()) + ".jsonl");
    QFile::remove(m_log);
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    qputenv("DOCKPIPE_HISTORY_FIXTURE_REQUESTS", m_log.toUtf8());
}

void GitHistoryWidgetTest::cleanup()
{
    if (m_previousBinary.isNull())
        qunsetenv("DOCKPIPE_BIN");
    else
        qputenv("DOCKPIPE_BIN", m_previousBinary);
    if (m_previousLog.isNull())
        qunsetenv("DOCKPIPE_HISTORY_FIXTURE_REQUESTS");
    else
        qputenv("DOCKPIPE_HISTORY_FIXTURE_REQUESTS", m_previousLog);
}

void GitHistoryWidgetTest::searchModesPagingAndDetail()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    auto *query = widget.findChild<QLineEdit *>("gitHistoryQuery");
    auto *mode = widget.findChild<QComboBox *>("gitHistoryMatch");
    auto *ref = widget.findChild<QComboBox *>("gitHistoryRef");
    auto *commits = widget.findChild<QListWidget *>("gitHistoryCommits");
    auto *next = widget.findChild<QPushButton *>("gitHistoryNext");
    auto *previous = widget.findChild<QPushButton *>("gitHistoryPrevious");
    query->setText("Commit");
    mode->setCurrentIndex(mode->findData("prefix"));
    ref->setCurrentIndex(ref->findData("origin/main"));
    search->click();
    QTRY_VERIFY(search->isEnabled());
    QCOMPARE(commits->count(), 3);
    auto sent = requests(m_log);
    QCOMPARE(sent.last().value("query").toString(), "Commit");
    QCOMPARE(sent.last().value("match").toString(), "prefix");
    QCOMPARE(sent.last().value("ref").toString(), "origin/main");
    QCOMPARE(sent.last().value("limit").toInt(), 50);
    QVERIFY(next->isEnabled());
    QVERIFY(!previous->isEnabled());
    next->click();
    QTRY_VERIFY(search->isEnabled());
    QCOMPARE(requests(m_log).last().value("offset").toInt(), 50);
    QVERIFY(previous->isEnabled());
    QVERIFY(!next->isEnabled());
    previous->click();
    QTRY_VERIFY(search->isEnabled());
    commits->setCurrentRow(1);
    QTRY_VERIFY(search->isEnabled());
    const auto *diff = widget.findChild<GitDiffWidget *>("gitHistoryDetail");
    const auto *detail = diff->findChild<QPlainTextEdit *>("gitDiffAfterEditor");
    QVERIFY(diff->findChild<QPlainTextEdit *>("gitDiffCommitMessage")->toPlainText().contains("Fixture commit message <literal>"));
    QVERIFY(detail->toPlainText().contains("fixture content"));
    QVERIFY(!detail->toPlainText().contains("+fixture content"));
    QCOMPARE(requests(m_log).last().value("commit").toString(), commitId(2));
    commits->setCurrentRow(2);
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(diff->findChild<QLabel *>("gitDiffNotice")->text().contains("exceeds the 2 MiB preview limit"));
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().contains("exceeds the 2 MiB preview limit"));
    query->setText("new query");
    QVERIFY(!next->isEnabled());
    mode->setCurrentIndex(mode->findData("keyword"));
    ref->setCurrentIndex(ref->findData(QString()));
    QMetaObject::invokeMethod(query, "returnPressed");
    QTRY_VERIFY(search->isEnabled());
    QCOMPARE(requests(m_log).last().value("match").toString(), "keyword");
    QCOMPARE(requests(m_log).last().value("ref").toString(), QString());
}

void GitHistoryWidgetTest::commitContextActionsTargetClickedRow()
{
    GitHistoryWidget widget;
    widget.resize(1180, 760);
    widget.show();
    widget.setSnapshot(m_root.path(), snapshot());
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(widget.queueModel()->addCommit(commitId(1), "Existing queue entry"));
    QSignalSpy actions(&widget, &GitHistoryWidget::actionRequested);
    QVERIFY(chooseCommitAction(widget, 1, "gitCommitCopyAction"));
    QCOMPARE(QApplication::clipboard()->text(), commitId(2));
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(chooseCommitAction(widget, 1, "gitCommitCherryPickAction", [] { answerConfirmation(); }));
    QCOMPARE(actions.count(), 1);
    const auto pick = actions.first().first().toJsonObject();
    QCOMPARE(pick.value("commits").toArray(), QJsonArray({commitId(2)}));
    QCOMPARE(pick.value("snapshot").toString(), "reviewed-snapshot");
    QVERIFY(!widget.property("gitPickingQueuedCommits").toBool());
    QCOMPARE(widget.queueModel()->commitIds(), QStringList({commitId(1)}));
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(chooseCommitAction(widget, 1, "gitCommitQueueAction"));
    QCOMPARE(widget.queueModel()->commitIds(), QStringList({commitId(1), commitId(2)}));
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(chooseCommitAction(widget, 2, "gitCommitRevertAction", [] { answerConfirmation(); }));
    QCOMPARE(actions.count(), 2);
    QCOMPARE(actions.last().first().toJsonObject().value("commit").toString(), commitId(3));
}

void GitHistoryWidgetTest::contextPreviewShowsAllFiles()
{
    GitHistoryWidget widget;
    widget.resize(1180, 760);
    widget.show();
    widget.setSnapshot(m_root.path(), snapshot());
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    QSignalSpy actions(&widget, &GitHistoryWidget::actionRequested);
    bool selectedSecondFile = false;
    QTimer driver;
    driver.setInterval(10);
    connect(&driver, &QTimer::timeout, &widget, [&] {
        auto *dialog = widget.findChild<QDialog *>("gitCommitPreview");
        if (!dialog || !dialog->isVisible())
            return;
        auto *files = dialog->findChild<QListWidget *>("gitCommitPreviewFiles");
        if (files->count() != 2)
            return;
        driver.stop();
        files->setCurrentRow(1);
        auto *after = dialog->findChild<QPlainTextEdit *>("gitDiffAfterEditor");
        selectedSecondFile = after->toPlainText().contains("second_file = 2")
            && files->item(1)->data(Qt::UserRole).toString() == "src/second.py";
        const QString screenshot = qEnvironmentVariable("DOCKPIPE_COMMIT_PREVIEW_SCREENSHOT");
        if (!screenshot.isEmpty()) {
            dialog->resize(1180, 760);
            QApplication::processEvents();
            dialog->grab().save(screenshot);
        }
        dialog->accept();
    });
    driver.start();
    QVERIFY(chooseCommitAction(widget, 1, "gitCommitPreviewAction"));
    driver.stop();
    QVERIFY(selectedSecondFile);
    QCOMPARE(actions.count(), 0);
    QTRY_VERIFY(search->isEnabled());
}

void GitHistoryWidgetTest::changedSnapshotInvalidatesContextAction()
{
    GitHistoryWidget widget;
    widget.resize(1180, 760);
    widget.show();
    widget.setSnapshot(m_root.path(), snapshot());
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    QSignalSpy actions(&widget, &GitHistoryWidget::actionRequested);
    QVERIFY(chooseCommitAction(widget, 0, "gitCommitCherryPickAction", [&] {
        widget.setSnapshot(m_root.path(), snapshot("new-snapshot"));
    }));
    QCOMPARE(actions.count(), 0);
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().contains("repository changed"));
}

void GitHistoryWidgetTest::queueOrderingAndRefresh()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    auto *commits = widget.findChild<QListWidget *>("gitHistoryCommits");
    for (int row = 0; row < commits->count(); ++row)
        commits->item(row)->setSelected(true);
    auto *add = widget.findChild<QPushButton *>("gitHistoryAdd");
    add->click();
    QCOMPARE(widget.queueModel()->commitIds(), QStringList({commitId(1), commitId(2), commitId(3)}));
    add->click();
    QCOMPARE(widget.queueModel()->rowCount(), 3);
    auto *queue = widget.findChild<QListView *>("gitCherryPickQueue");
    queue->setCurrentIndex(widget.queueModel()->index(1));
    widget.findChild<QPushButton *>("gitQueueUp")->click();
    QCOMPARE(widget.queueModel()->commitIds().first(), commitId(2));
    widget.findChild<QPushButton *>("gitQueueDown")->click();
    widget.findChild<QPushButton *>("gitQueueReverse")->click();
    QCOMPARE(widget.queueModel()->commitIds(), QStringList({commitId(3), commitId(2), commitId(1)}));
    widget.setSnapshot(m_root.path(), snapshot("new-snapshot"));
    QCOMPARE(widget.queueModel()->rowCount(), 3);
    widget.setBusy(true);
    QVERIFY(!widget.findChild<QPushButton *>("gitQueueStart")->isEnabled());
    QVERIFY(!widget.findChild<QPushButton *>("gitQueueClear")->isEnabled());
    widget.setBusy(false);
    queue->setCurrentIndex(widget.queueModel()->index(0));
    widget.findChild<QPushButton *>("gitQueueRemove")->click();
    QCOMPARE(widget.queueModel()->rowCount(), 2);
    widget.clearQueue();
    QCOMPARE(widget.queueModel()->rowCount(), 0);
}

void GitHistoryWidgetTest::confirmedMutationUsesExactSnapshot()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    QVERIFY(widget.queueModel()->addCommit(commitId(3), "Third"));
    QVERIFY(widget.queueModel()->addCommit(commitId(1), "First"));
    QSignalSpy actions(&widget, &GitHistoryWidget::actionRequested);
    answerConfirmation();
    widget.findChild<QPushButton *>("gitQueueStart")->click();
    QCOMPARE(actions.count(), 1);
    const auto request = actions.first().first().toJsonObject();
    QCOMPARE(request.value("action").toString(), "cherry-pick");
    QCOMPARE(request.value("snapshot").toString(), "reviewed-snapshot");
    QCOMPARE(request.value("commits").toArray(), QJsonArray({commitId(3), commitId(1)}));
    QVERIFY(request.value("confirm").toBool());
    QCOMPARE(widget.queueModel()->rowCount(), 2); // Parent clears only after runtime acceptance.
    QVERIFY(requests(m_log).isEmpty()); // No mutation is executed by the child.

    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    auto *commits = widget.findChild<QListWidget *>("gitHistoryCommits");
    commits->item(0)->setSelected(true);
    answerConfirmation();
    widget.findChild<QPushButton *>("gitHistoryRevert")->click();
    QCOMPARE(actions.count(), 2);
    const auto revert = actions.last().first().toJsonObject();
    QCOMPARE(revert.value("action").toString(), "revert");
    QCOMPARE(revert.value("commit").toString(), commitId(1));
    QCOMPARE(revert.value("snapshot").toString(), "reviewed-snapshot");
}

void GitHistoryWidgetTest::changedSnapshotInvalidatesConfirmation()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    QVERIFY(widget.queueModel()->addCommit(commitId(1), "First"));
    QSignalSpy actions(&widget, &GitHistoryWidget::actionRequested);
    answerConfirmation([this, &widget] { widget.setSnapshot(m_root.path(), snapshot("changed", "main")); });
    widget.findChild<QPushButton *>("gitQueueStart")->click();
    QCOMPARE(actions.count(), 0);
    QCOMPARE(widget.queueModel()->rowCount(), 1);
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().contains("changed during confirmation"));
}

void GitHistoryWidgetTest::failedReadPreservesQueue()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    QVERIFY(widget.queueModel()->addCommit(commitId(1), "First"));
    widget.findChild<QLineEdit *>("gitHistoryQuery")->setText("fail");
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    search->click();
    QTRY_VERIFY(search->isEnabled());
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().contains("Fixture read failed"));
    QCOMPARE(widget.queueModel()->rowCount(), 1);
    QVERIFY(widget.findChild<QPushButton *>("gitQueueStart")->isEnabled());
}

void GitHistoryWidgetTest::cancellationAndProjectChangeIgnoreLateReply()
{
    GitHistoryWidget widget;
    widget.setSnapshot(m_root.path(), snapshot());
    QVERIFY(widget.queueModel()->addCommit(commitId(1), "First"));
    auto *query = widget.findChild<QLineEdit *>("gitHistoryQuery");
    auto *search = widget.findChild<QPushButton *>("gitHistorySearch");
    auto *cancel = widget.findChild<QPushButton *>("gitHistoryCancel");
    query->setText("slow");
    search->click();
    QTRY_VERIFY(!requests(m_log).isEmpty());
    QVERIFY(cancel->isEnabled());
    cancel->click();
    QTRY_VERIFY(search->isEnabled());
    QCOMPARE(widget.queueModel()->rowCount(), 1);
    QCOMPARE(widget.findChild<QListWidget *>("gitHistoryCommits")->count(), 0);
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().contains("cancelled"));

    search->click();
    QTRY_VERIFY(requests(m_log).size() == 2);
    widget.setSnapshot("another-project", snapshot("other-token", "other"));
    QTRY_VERIFY(search->isEnabled());
    QCOMPARE(widget.queueModel()->rowCount(), 0);
    QCOMPARE(widget.findChild<QListWidget *>("gitHistoryCommits")->count(), 0);
    QVERIFY(widget.findChild<QLabel *>("gitHistoryStatus")->text().isEmpty());
    QVERIFY(widget.findChild<QLabel *>("gitQueueTarget")->text().contains("other"));
}

int main(int argc, char **argv)
{
    if (argc == 4 && QByteArray(argv[1]) == "git" && QByteArray(argv[2]) == "--request") {
        QCoreApplication app(argc, argv);
        return fakeCli(QString::fromLocal8Bit(argv[3]));
    }
    QTemporaryDir configuration;
    qputenv("XDG_CONFIG_HOME", configuration.path().toUtf8());
    QApplication app(argc, argv);
    app.setOrganizationName("DockpipeGitHistoryTests");
    app.setApplicationName("GitHistoryWidgetTest");
    applyDockpipeLauncherTheme(app);
    previewDockpipeLauncherTheme(app, qEnvironmentVariable("DOCKPIPE_TEST_THEME") == "light"
        ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    GitHistoryWidgetTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "GitHistoryWidgetTest.moc"
