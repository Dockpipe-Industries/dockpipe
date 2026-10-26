#include "GitConflictWidget.h"
#include "GitRepositoryWidget.h"
#include "ProjectFilesWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QCheckBox>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QLineEdit>
#include <QListWidget>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <cstdio>

namespace {
const QString stashId(40, 'a');

QJsonObject catalogReply()
{
    return QJsonObject{{"catalog", QJsonObject{
        {"branches", QJsonArray{
            QJsonObject{{"name", "main"}, {"id", QString(40, 'b')}},
            QJsonObject{{"name", "topic/<b>plain</b>"}, {"id", QString(40, 'c')}}}},
        {"remotes", QJsonArray{"origin"}}, {"notice", "Fixture catalog limit notice"}}}};
}

QJsonObject conflictSnapshot()
{
    return QJsonObject{{"snapshot", "conflict-reviewed-token"}, {"operation", QJsonObject{
        {"kind", "cherry-pick"}, {"current", QString(40, 'd')},
        {"remaining", QJsonArray{QString(40, 'e')}}, {"conflicts", QJsonArray{"conflict.txt"}},
        {"can_continue", false}, {"can_skip", true}, {"can_abort", true}}}};
}

void answerNextDialog(QMessageBox::StandardButton answer)
{
    QTimer::singleShot(0, qApp, [answer] {
        auto *dialog = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        if (dialog)
            dialog->done(answer);
    });
}

bool writeFile(const QString &path, const QByteArray &bytes)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write(bytes) == bytes.size();
}
}

class GitRepositoryConflictTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void repositoryReadsCatalogAndEmitsExactSelections();
    void repositoryBusyAndOperationGuard();
    void conflictPreservesDraftAndGuardsProjectSwitch();
    void conflictSaveRefreshAndConfirmedResolution();
    void conflictControlsFollowOperationState();
    void compactConflictEditorRetainsControls();
};

void GitRepositoryConflictTest::initTestCase()
{
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
}

void GitRepositoryConflictTest::repositoryReadsCatalogAndEmitsExactSelections()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    GitRepositoryWidget widget;
    QSignalSpy actions(&widget, &GitRepositoryWidget::actionRequested);
    widget.setSnapshot(root.path(), QJsonObject{{"branch", "main"}, {"snapshot", "repository-reviewed-token"}});
    auto *branches = widget.findChild<QListWidget *>("gitRepositoryBranches");
    auto *stashes = widget.findChild<QListWidget *>("gitRepositoryStashes");
    QVERIFY(branches && stashes);
    QTRY_COMPARE(branches->count(), 2);
    QTRY_COMPARE(stashes->count(), 2);
    auto *switchButton = widget.findChild<QPushButton *>("gitRepository-branch-switch");
    QTRY_VERIFY(switchButton->isEnabled());
    QCOMPARE(branches->item(0)->text(), "main (current)");
    branches->setCurrentRow(1);
    switchButton->click();
    QCOMPARE(actions.count(), 1);
    QCOMPARE(actions.takeFirst().at(0).toJsonObject(), QJsonObject({
        {"action", "branch-switch"}, {"name", "topic/<b>plain</b>"}, {"snapshot", "repository-reviewed-token"}}));
    answerNextDialog(QMessageBox::Yes);
    widget.findChild<QPushButton *>("gitRepository-merge")->click();
    const auto merge = actions.takeFirst().at(0).toJsonObject();
    QCOMPARE(merge.value("action").toString(), "merge");
    QCOMPARE(merge.value("commit").toString(), QString(40, 'c'));
    QCOMPARE(merge.value("ref").toString(), "topic/<b>plain</b>");
    QCOMPARE(merge.value("snapshot").toString(), "repository-reviewed-token");
    QVERIFY(merge.value("confirm").toBool());
    stashes->setCurrentRow(1);
    widget.findChild<QPushButton *>("gitRepository-stash-apply")->click();
    const auto apply = actions.takeFirst().at(0).toJsonObject();
    QCOMPARE(apply.value("stash").toString(), stashId);
    QCOMPARE(apply.value("stash_selector").toString(), "stash@{1}");
    answerNextDialog(QMessageBox::Yes);
    widget.findChild<QPushButton *>("gitRepository-stash-drop")->click();
    const auto drop = actions.takeFirst().at(0).toJsonObject();
    QCOMPARE(drop.value("action").toString(), "stash-drop");
    QVERIFY(drop.value("confirm").toBool());
    QCOMPARE(drop.value("stash_selector").toString(), "stash@{1}");
    widget.findChild<QLineEdit *>("gitRepositoryBranchName")->setText("new/topic");
    widget.findChild<QPushButton *>("gitRepositoryBranchCreate")->click();
    QCOMPARE(actions.takeFirst().at(0).toJsonObject().value("name").toString(), "new/topic");
    const auto *status = widget.findChild<QLabel *>("gitRepositoryStatus");
    QCOMPARE(status->text(), "Fixture catalog limit notice");
}

void GitRepositoryConflictTest::repositoryBusyAndOperationGuard()
{
    QTemporaryDir root;
    GitRepositoryWidget widget;
    widget.setSnapshot(root.path(), {});
    auto *fetch = widget.findChild<QPushButton *>("gitRepositoryFetch");
    QTRY_VERIFY(fetch->isEnabled());
    widget.setBusy(true);
    QVERIFY(!fetch->isEnabled());
    widget.setBusy(false);
    QVERIFY(fetch->isEnabled());
    widget.setSnapshot(root.path(), conflictSnapshot());
    QTRY_VERIFY(!widget.findChild<QListWidget *>("gitRepositoryStashes")->isEnabled());
    QVERIFY(!fetch->isEnabled());
}

void GitRepositoryConflictTest::conflictPreservesDraftAndGuardsProjectSwitch()
{
    QTemporaryDir root;
    QTemporaryDir other;
    QVERIFY(writeFile(root.filePath("conflict.txt"), "original\n"));
    GitConflictWidget widget;
    widget.setSnapshot(root.path(), conflictSnapshot());
    widget.findChild<QPushButton *>("gitConflictOpen")->click();
    auto *files = widget.findChild<ProjectFilesWidget *>("gitConflictEditor");
    auto *editor = files->findChild<QPlainTextEdit *>("fileTextEditor");
    QVERIFY(editor);
    editor->insertPlainText("unsaved draft\n");
    const auto draft = editor->toPlainText();
    QVERIFY(widget.hasUnsavedChanges());
    widget.setSnapshot(root.path(), conflictSnapshot());
    QCOMPARE(editor->toPlainText(), draft);
    QVERIFY(widget.hasUnsavedChanges());
    answerNextDialog(QMessageBox::Cancel);
    widget.setSnapshot(other.path(), {});
    QCOMPARE(files->currentPath(), root.filePath("conflict.txt"));
    QCOMPARE(editor->toPlainText(), draft);
    answerNextDialog(QMessageBox::Cancel);
    QVERIFY(!widget.confirmLeave());
    QVERIFY(widget.hasUnsavedChanges());
}

void GitRepositoryConflictTest::conflictSaveRefreshAndConfirmedResolution()
{
    QTemporaryDir root;
    QVERIFY(writeFile(root.filePath("conflict.txt"), "original\n"));
    GitConflictWidget widget;
    QSignalSpy actions(&widget, &GitConflictWidget::actionRequested);
    QSignalSpy refresh(&widget, &GitConflictWidget::refreshRequested);
    widget.setSnapshot(root.path(), conflictSnapshot());
    widget.findChild<QPushButton *>("gitConflictOpen")->click();
    auto *files = widget.findChild<ProjectFilesWidget *>("gitConflictEditor");
    auto *editor = files->findChild<QPlainTextEdit *>("fileTextEditor");
    editor->setPlainText("resolved\n");
    QVERIFY(files->saveFile());
    QCOMPARE(refresh.count(), 1);
    answerNextDialog(QMessageBox::Cancel);
    widget.findChild<QPushButton *>("gitConflict-resolve-stage")->click();
    QCOMPARE(actions.count(), 0);
    QTimer::singleShot(0, &widget, [&widget] {
        auto newer = conflictSnapshot();
        newer.insert("snapshot", "newer-unreviewed-token");
        widget.setSnapshot(QFileInfo(widget.findChild<ProjectFilesWidget *>("gitConflictEditor")->currentPath()).absolutePath(), newer);
        if (auto *dialog = qobject_cast<QMessageBox *>(QApplication::activeModalWidget()))
            dialog->done(QMessageBox::Yes);
    });
    widget.findChild<QPushButton *>("gitConflict-resolve-stage")->click();
    QCOMPARE(actions.count(), 1);
    const auto request = actions.takeFirst().at(0).toJsonObject();
    QCOMPARE(request.value("action").toString(), "resolve-stage");
    QCOMPARE(request.value("paths").toArray(), QJsonArray{"conflict.txt"});
    QVERIFY(request.value("confirm").toBool());
    QCOMPARE(request.value("snapshot").toString(), "conflict-reviewed-token");
    answerNextDialog(QMessageBox::Yes);
    widget.findChild<QPushButton *>("gitConflict-resolve-ours")->click();
    QVERIFY(writeFile(root.filePath("conflict.txt"), "chosen ours\n"));
    widget.setSnapshot(root.path(), conflictSnapshot());
    QCOMPARE(editor->toPlainText(), "chosen ours\n");
}

void GitRepositoryConflictTest::conflictControlsFollowOperationState()
{
    QTemporaryDir root;
    GitConflictWidget widget;
    widget.setSnapshot(root.path(), conflictSnapshot());
    auto *continueButton = widget.findChild<QPushButton *>("gitConflictContinue");
    auto *skip = widget.findChild<QPushButton *>("gitConflictSkip");
    auto *abort = widget.findChild<QPushButton *>("gitConflictAbort");
    QVERIFY(!continueButton->isEnabled());
    QVERIFY(skip->isEnabled());
    QVERIFY(abort->isEnabled());
    widget.setBusy(true);
    QVERIFY(!skip->isEnabled());
    QVERIFY(!abort->isEnabled());
    widget.setBusy(false);
    auto snapshot = conflictSnapshot();
    auto operation = snapshot.value("operation").toObject();
    operation.insert("can_continue", true);
    operation.insert("conflicts", QJsonArray{});
    snapshot.insert("operation", operation);
    widget.setSnapshot(root.path(), snapshot);
    QVERIFY(continueButton->isEnabled());
    widget.setSnapshot(root.path(), {});
    QVERIFY(!continueButton->isEnabled());
    QVERIFY(!skip->isEnabled());
    QVERIFY(!abort->isEnabled());
}

void GitRepositoryConflictTest::compactConflictEditorRetainsControls()
{
    applyDockpipeLauncherTheme(*qobject_cast<QApplication *>(qApp));
    QTemporaryDir root;
    QVERIFY(writeFile(root.filePath("conflict.txt"), "<<<<<<< ours\nours\n=======\ntheirs\n>>>>>>> theirs\n"));
    GitConflictWidget widget;
    widget.setSnapshot(root.path(), conflictSnapshot());
    widget.resize(928, 600);
    widget.show();
    widget.findChild<QPushButton *>("gitConflictOpen")->click();
    QCoreApplication::processEvents();
    QVERIFY(widget.width() <= 928);
    for (const auto &name : {"gitConflictOpen", "gitConflict-resolve-stage", "gitConflict-resolve-ours",
             "gitConflict-resolve-theirs", "gitConflict-resolve-delete", "gitConflictContinue",
             "gitConflictSkip", "gitConflictAbort", "editorSave"}) {
        const auto *button = widget.findChild<QPushButton *>(name);
        QVERIFY2(button && button->isVisible(), name);
        QVERIFY2(widget.rect().contains(QRect(button->mapTo(&widget, QPoint()), button->size())), name);
    }
    auto *editor = widget.findChild<QPlainTextEdit *>("fileTextEditor");
    QVERIFY(editor && editor->isVisible() && editor->width() >= 180);
    const QString image = qEnvironmentVariable("DOCKPIPE_CONFLICT_COMPACT_SCREENSHOT");
    if (!image.isEmpty())
        QVERIFY(widget.grab().save(image));
}

int main(int argc, char **argv)
{
    if (argc == 4 && QByteArray(argv[1]) == "git" && QByteArray(argv[2]) == "--request") {
        QFile file(QString::fromLocal8Bit(argv[3]));
        if (!file.open(QIODevice::ReadOnly))
            return 2;
        const auto request = QJsonDocument::fromJson(file.readAll()).object();
        QJsonObject reply;
        if (request.value("action") == "catalog") {
            reply = catalogReply();
        } else if (request.value("action") == "stash-list") {
            reply.insert("stashes", QJsonArray{
                QJsonObject{{"id", stashId}, {"selector", "stash@{0}"}, {"subject", "first"}},
                QJsonObject{{"id", stashId}, {"selector", "stash@{1}"}, {"subject", "same OID"}}});
        } else {
            return 3;
        }
        const auto bytes = QJsonDocument(reply).toJson(QJsonDocument::Compact);
        return std::fwrite(bytes.constData(), 1, size_t(bytes.size()), stdout) == size_t(bytes.size()) ? 0 : 4;
    }
    QApplication application(argc, argv);
    GitRepositoryConflictTest tests;
    return QTest::qExec(&tests, argc, argv);
}

#include "GitRepositoryConflictTest.moc"
