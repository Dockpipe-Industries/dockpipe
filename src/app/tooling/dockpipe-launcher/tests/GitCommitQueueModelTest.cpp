#include "GitCommitQueueModel.h"

#include <QAbstractItemModelTester>
#include <QPersistentModelIndex>
#include <QSignalSpy>
#include <QtTest>

class GitCommitQueueModelTest final : public QObject {
    Q_OBJECT
private slots:
    void exactIdsAndSubjects();
    void rejectInvalidAndDuplicateEntries();
    void preserveExplicitOrderAndSelection();
    void enforceCapacityAndRecover();
};

void GitCommitQueueModelTest::exactIdsAndSubjects()
{
    GitCommitQueueModel model;
    QAbstractItemModelTester tester(&model, QAbstractItemModelTester::FailureReportingMode::QtTest);
    const QString sha1(40, 'a');
    const QString sha256(64, 'b');
    QVERIFY(model.addCommit(sha1.toUpper(), "Fix initial load"));
    QVERIFY(model.addCommit(sha256, "<b>Literal repository text</b>"));
    QCOMPARE(model.commitIds(), QStringList({sha1, sha256}));
    QCOMPARE(model.data(model.index(0), GitCommitQueueModel::IdRole).toString(), sha1);
    QCOMPARE(model.data(model.index(1), GitCommitQueueModel::SubjectRole).toString(), "<b>Literal repository text</b>");
    QVERIFY(model.data(model.index(0)).toString().contains("Fix initial load"));
    QCOMPARE(model.rowCount(model.index(0)), 0);
    QVERIFY(!model.data(QModelIndex()).isValid());
}

void GitCommitQueueModelTest::rejectInvalidAndDuplicateEntries()
{
    GitCommitQueueModel model;
    const QString id(40, 'c');
    QVERIFY(model.addCommit(id, "Initial subject"));
    QSignalSpy inserted(&model, &QAbstractItemModel::rowsInserted);
    for (const QString &invalid : QStringList{"", "abc1234", QString(39, 'a'), QString(41, 'a'), QString(40, 'z'), " " + id, id + "\n"})
        QVERIFY(!model.addCommit(invalid, "Rejected"));
    QVERIFY(!model.addCommit(id.toUpper(), "Duplicate"));
    QVERIFY(!model.addCommit(QString(40, 'd'), QString(GitCommitQueueModel::maximumSubjectLength + 1, 'x')));
    QVERIFY(!model.addCommit(QString(40, 'd'), "Multiple\nlines"));
    QVERIFY(!model.addCommit(QString(40, 'd'), QString("Null") + QChar::Null));
    QCOMPARE(inserted.count(), 0);
    QCOMPARE(model.rowCount(), 1);
    QCOMPARE(model.data(model.index(0), GitCommitQueueModel::SubjectRole).toString(), "Initial subject");
}

void GitCommitQueueModelTest::preserveExplicitOrderAndSelection()
{
    GitCommitQueueModel model;
    QAbstractItemModelTester tester(&model, QAbstractItemModelTester::FailureReportingMode::QtTest);
    const QString first(40, '1');
    const QString second(40, '2');
    const QString third(40, '3');
    QVERIFY(model.addCommit(first, "First"));
    QVERIFY(model.addCommit(second, "Second"));
    QVERIFY(model.addCommit(third, "Third"));
    QPersistentModelIndex selected = model.index(0);
    QSignalSpy moved(&model, &QAbstractItemModel::rowsMoved);
    QVERIFY(model.moveCommit(0, 2));
    QCOMPARE(model.commitIds(), QStringList({second, third, first}));
    QCOMPARE(selected.row(), 2);
    QCOMPARE(selected.data(GitCommitQueueModel::IdRole).toString(), first);
    QVERIFY(model.moveCommit(2, 0));
    QCOMPARE(model.commitIds(), QStringList({first, second, third}));
    QCOMPARE(selected.row(), 0);
    QVERIFY(model.moveCommit(0, 0));
    QCOMPARE(moved.count(), 2);
    QVERIFY(!model.moveCommit(-1, 0));
    QVERIFY(!model.moveCommit(0, 3));
    QVERIFY(!model.removeCommit(-1));
    QVERIFY(!model.removeCommit(3));
    QVERIFY(model.removeCommit(1));
    QCOMPARE(model.commitIds(), QStringList({first, third}));
    model.clear();
    QVERIFY(!selected.isValid());
    QCOMPARE(model.rowCount(), 0);
    model.clear();
}

void GitCommitQueueModelTest::enforceCapacityAndRecover()
{
    GitCommitQueueModel model;
    QAbstractItemModelTester tester(&model, QAbstractItemModelTester::FailureReportingMode::QtTest);
    for (int value = 0; value < GitCommitQueueModel::maximumCommits; ++value)
        QVERIFY(model.addCommit(QString::number(value, 16).rightJustified(40, '0'), "Commit"));
    const QString extra(64, 'f');
    QVERIFY(!model.addCommit(extra, "Over capacity"));
    QCOMPARE(model.rowCount(), GitCommitQueueModel::maximumCommits);
    QVERIFY(model.removeCommit(0));
    QVERIFY(model.addCommit(extra, QString(GitCommitQueueModel::maximumSubjectLength, 'x')));
    model.clear();
    QVERIFY(model.addCommit(extra, ""));
    QCOMPARE(model.rowCount(), 1);
}

QTEST_GUILESS_MAIN(GitCommitQueueModelTest)
#include "GitCommitQueueModelTest.moc"
