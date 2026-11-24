#include "DashboardWorkflowView.h"
#include "WorkflowEditorWidget.h"

#include <QApplication>
#include <QDir>
#include <QMessageBox>
#include <QPlainTextEdit>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>

class DashboardWorkflowViewTest final : public QObject {
    Q_OBJECT
private slots:
    void independentRootsAndLaunchOrigin();
    void cancelledRootChangeKeepsDocument();
};

void DashboardWorkflowViewTest::independentRootsAndLaunchOrigin()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString firstRoot = directory.path() + "/first";
    const QString secondRoot = directory.path() + "/second";
    QVERIFY(QDir().mkpath(firstRoot));
    QVERIFY(QDir().mkpath(secondRoot));
    DashboardWorkflowView first;
    DashboardWorkflowView second;
    QVERIFY(first.setWorkdir(firstRoot));
    QVERIFY(second.setWorkdir(secondRoot));
    QVERIFY(!first.setWorkdir(directory.path() + "/missing"));
    QCOMPARE(first.workdir(), firstRoot);
    QCOMPARE(second.workdir(), secondRoot);

    auto *editor = first.findChild<WorkflowEditorWidget *>();
    QVERIFY(editor);
    QCOMPARE(editor->workdir(), firstRoot);
    QSignalSpy launched(&first, &DashboardWorkflowView::workflowRunRequested);
    const QString path = firstRoot + "/workflow.yml";
    editor->runRequested(path);
    QCOMPARE(launched.size(), 1);
    QCOMPARE(launched.first().at(0).toString(), firstRoot);
    QCOMPARE(launched.first().at(1).toString(), path);
    QCOMPARE(second.workdir(), secondRoot);
}

void DashboardWorkflowViewTest::cancelledRootChangeKeepsDocument()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString firstRoot = directory.path() + "/first";
    const QString secondRoot = directory.path() + "/second";
    QVERIFY(QDir().mkpath(firstRoot));
    QVERIFY(QDir().mkpath(secondRoot));
    DashboardWorkflowView view;
    QVERIFY(view.setWorkdir(firstRoot));
    auto *source = view.findChild<QPlainTextEdit *>("workflowYaml");
    QVERIFY(source);
    const QString changed = "name: keep-this-draft\nsteps: []\n";
    source->setPlainText(changed);
    QTimer::singleShot(0, []() {
        auto *confirmation = qobject_cast<QMessageBox *>(QApplication::activeModalWidget());
        QVERIFY(confirmation);
        confirmation->done(QMessageBox::Cancel);
    });
    QVERIFY(!view.setWorkdir(secondRoot));
    QCOMPARE(view.workdir(), firstRoot);
    QCOMPARE(source->toPlainText(), changed);
}

QTEST_MAIN(DashboardWorkflowViewTest)
#include "DashboardWorkflowViewTest.moc"
