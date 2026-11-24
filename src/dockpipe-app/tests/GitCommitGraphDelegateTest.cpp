#include "GitCommitGraphDelegate.h"

#include <QJsonObject>
#include <QListWidget>
#include <QImage>
#include <QPainter>
#include <QtTest>

namespace {
QJsonObject commit(const QString &id, const QStringList &parents = {})
{
    return {{"id", id}, {"parents", QJsonArray::fromStringList(parents)},
        {"subject", "Subject " + id}, {"author", "Test author"}, {"date", "2026-11-08"}};
}

int segmentCount(const QVector<GitCommitGraphRow> &rows)
{
    int count = 0;
    for (const auto &row : rows)
        count += row.segments.size();
    return count;
}
}

class GitCommitGraphDelegateTest final : public QObject {
    Q_OBJECT
private slots:
    void linearHistory();
    void mergeAndFork();
    void octopusMerge();
    void interleavedBranchesKeepReservedParentLanes();
    void missingParentsNeverJoinAdjacentRows();
    void rejectsInvalidInput();
    void preservesSelectionRoles();
    void paintsPalette_data();
    void paintsPalette();
};

void GitCommitGraphDelegateTest::linearHistory()
{
    const auto rows = GitCommitGraphDelegate::layout({commit("a", {"b"}), commit("b", {"c"}), commit("c")});
    QCOMPARE(rows.size(), 3);
    QCOMPARE(segmentCount(rows), 4);
    for (const auto &row : rows) {
        QCOMPARE(row.column, 0);
        QCOMPARE(row.columns, 1);
        QVERIFY(!row.parentOutsidePage);
    }
}

void GitCommitGraphDelegateTest::mergeAndFork()
{
    const QJsonArray commits{commit("merge", {"left", "right"}), commit("left", {"base"}),
        commit("right", {"base"}), commit("base")};
    const auto rows = GitCommitGraphDelegate::layout(commits);
    QCOMPARE(rows.size(), 4);
    QCOMPARE(rows[0].column, 0);
    QCOMPARE(rows[1].column, 0);
    QCOMPARE(rows[2].column, 1);
    QCOMPARE(rows[3].column, 0);
    QCOMPARE(segmentCount(rows), 10);
    QCOMPARE(rows[0].segments.size(), 2);
    QCOMPARE(rows[0].segments[0].toColumn, rows[1].column);
    QCOMPARE(rows[0].segments[1].toColumn, rows[2].column);
    QCOMPARE(rows[2].segments.last().toColumn, rows[3].column);
    const auto repeated = GitCommitGraphDelegate::layout(commits);
    for (int row = 0; row < rows.size(); ++row)
        QCOMPARE(rows[row].column, repeated[row].column);
}

void GitCommitGraphDelegateTest::interleavedBranchesKeepReservedParentLanes()
{
    // The second parent appears first. Processing it must retain the reserved
    // first-parent lane, while its own ancestry continues through later rows.
    const auto rows = GitCommitGraphDelegate::layout({commit("merge", {"left", "right"}),
        commit("right", {"base"}), commit("left", {"base"}), commit("base")});
    QCOMPARE(rows[0].column, 0);
    QCOMPARE(rows[1].column, 1);
    QCOMPARE(rows[2].column, 0);
    QCOMPARE(rows[3].column, 1);
    QCOMPARE(rows[0].segments[0].toColumn, rows[2].column);
    QCOMPARE(rows[0].segments[1].toColumn, rows[1].column);
    QCOMPARE(rows[2].segments.last().toColumn, rows[3].column);
    bool foundContinuingRightBranch = false;
    for (const auto &segment : rows[2].segments) {
        if (segment.fromColumn == 1 && segment.toColumn == 1
            && segment.fromY == 0 && segment.toY == 1)
            foundContinuingRightBranch = true;
    }
    QVERIFY(foundContinuingRightBranch);
}

void GitCommitGraphDelegateTest::octopusMerge()
{
    const auto rows = GitCommitGraphDelegate::layout({commit("merge", {"a", "b", "c"}),
        commit("a"), commit("b"), commit("c")});
    QCOMPARE(rows.size(), 4);
    QCOMPARE(rows[0].segments.size(), 3);
    QCOMPARE(rows[1].column, 0);
    QCOMPARE(rows[2].column, 1);
    QCOMPARE(rows[3].column, 2);
    QCOMPARE(rows[0].columns, 3);
    QCOMPARE(segmentCount(rows), 9);
}

void GitCommitGraphDelegateTest::missingParentsNeverJoinAdjacentRows()
{
    const auto rows = GitCommitGraphDelegate::layout({commit("a", {"hidden"}), commit("unrelated"),
        commit("b", {"older"})});
    QCOMPARE(rows.size(), 3);
    QVERIFY(rows[0].parentOutsidePage);
    QVERIFY(!rows[1].parentOutsidePage);
    QVERIFY(rows[2].parentOutsidePage);
    QCOMPARE(segmentCount(rows), 0);
    const auto unordered = GitCommitGraphDelegate::layout({commit("parent"), commit("child", {"parent"})});
    QVERIFY(unordered[1].parentOutsidePage);
    QCOMPARE(segmentCount(unordered), 0);
}

void GitCommitGraphDelegateTest::rejectsInvalidInput()
{
    QVERIFY(GitCommitGraphDelegate::layout({commit("same"), commit("same")}).isEmpty());
    QVERIFY(GitCommitGraphDelegate::layout({commit("")}).isEmpty());
    QJsonArray oversized;
    for (int row = 0; row < 51; ++row)
        oversized.append(commit(QString::number(row)));
    QVERIFY(GitCommitGraphDelegate::layout(oversized).isEmpty());
    oversized.removeLast();
    QCOMPARE(GitCommitGraphDelegate::layout(oversized).size(), 50);
    QListWidget list;
    QVERIFY(!GitCommitGraphDelegate::prepare(&list, {commit("a")}));
    QVERIFY(GitCommitGraphDelegate::prepare(&list, {}));
    auto *wrong = new QListWidgetItem("Wrong commit", &list);
    wrong->setData(Qt::UserRole, "wrong");
    QVERIFY(!GitCommitGraphDelegate::prepare(&list, {commit("a")}));
    QVERIFY(!wrong->data(Qt::UserRole + 2).isValid());
}

void GitCommitGraphDelegateTest::preservesSelectionRoles()
{
    QListWidget list;
    auto *item = new QListWidgetItem("Original accessible text", &list);
    item->setData(Qt::UserRole, "a");
    item->setData(Qt::UserRole + 1, "Original subject");
    item->setSelected(true);
    auto object = commit("a", {"missing"});
    object.insert("refs", QJsonArray{"HEAD → main", "tag: release"});
    QVERIFY(GitCommitGraphDelegate::prepare(&list, {object}));
    QCOMPARE(item->data(Qt::UserRole).toString(), "a");
    QCOMPARE(item->data(Qt::UserRole + 1).toString(), "Original subject");
    QVERIFY(item->isSelected());
    QVERIFY(item->toolTip().contains("HEAD"));
    QVERIFY(item->toolTip().contains("outside"));
    QVERIFY(item->data(Qt::UserRole + 2).isValid());
}

void GitCommitGraphDelegateTest::paintsPalette_data()
{
    QTest::addColumn<bool>("dark");
    QTest::addColumn<bool>("selected");
    QTest::newRow("light") << false << false;
    QTest::newRow("light-selected") << false << true;
    QTest::newRow("dark") << true << false;
    QTest::newRow("dark-selected") << true << true;
}

void GitCommitGraphDelegateTest::paintsPalette()
{
    QFETCH(bool, dark);
    QFETCH(bool, selected);
    QListWidget list;
    auto *item = new QListWidgetItem("Commit", &list);
    item->setData(Qt::UserRole, "a");
    item->setData(Qt::UserRole + 1, "Readable commit subject");
    auto object = commit("a", {"outside"});
    object.insert("refs", QJsonArray{"HEAD → main"});
    QVERIFY(GitCommitGraphDelegate::prepare(&list, {object}));
    GitCommitGraphDelegate delegate;
    QStyleOptionViewItem option;
    option.initFrom(&list);
    option.rect = QRect(0, 0, 550, 60);
    option.font = list.font();
    option.fontMetrics = QFontMetrics(option.font);
    option.palette.setColor(QPalette::Base, dark ? QColor("#20252d") : QColor("#ffffff"));
    option.palette.setColor(QPalette::Text, dark ? QColor("#eff3fa") : QColor("#182132"));
    option.palette.setColor(QPalette::Highlight, dark ? QColor("#324762") : QColor("#dceaff"));
    // Deliberately unsuitable for the pale light-mode QSS selection: the
    // delegate must use Text instead of the global HighlightedText value.
    option.palette.setColor(QPalette::HighlightedText, QColor("#ffffff"));
    if (selected)
        option.state |= QStyle::State_Selected;
    QImage image(550, 60, QImage::Format_ARGB32_Premultiplied);
    image.fill(option.palette.color(QPalette::Base));
    QPainter painter(&image);
    delegate.paint(&painter, option, list.model()->index(0, 0));
    painter.end();
    const QColor node = image.pixelColor(14, 30);
    const QColor background = option.palette.color(QPalette::Base);
    QVERIFY(qAbs(node.lightness() - background.lightness()) > 40);
    const QColor expectedText = option.palette.color(QPalette::Text);
    bool foundSubjectText = false;
    for (int y = 7; y < 27 && !foundSubjectText; ++y) {
        for (int x = 44; x < image.width(); ++x) {
            const QColor pixel = image.pixelColor(x, y);
            const int distance = qAbs(pixel.red() - expectedText.red())
                + qAbs(pixel.green() - expectedText.green())
                + qAbs(pixel.blue() - expectedText.blue());
            if (distance < 15) {
                foundSubjectText = true;
                break;
            }
        }
    }
    QVERIFY(foundSubjectText);
    QVERIFY(delegate.sizeHint(option, list.model()->index(0, 0)).height() >= 58);
}

QTEST_MAIN(GitCommitGraphDelegateTest)
#include "GitCommitGraphDelegateTest.moc"
