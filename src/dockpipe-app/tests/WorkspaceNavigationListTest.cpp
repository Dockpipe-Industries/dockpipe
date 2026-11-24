#include "WorkspaceNavigationList.h"

#include <QFile>
#include <QScrollBar>
#include <QSignalSpy>
#include <QStyleFactory>
#include <QtTest>

namespace {
QStringList titles()
{
    return {"Apps", "Workflows", "Machines", "Containers", "System", "Files", "Git", "Browser",
        "Terminal", "Remote Desktop", "Generic contribution"};
}

void populate(QListWidget &list, bool includeContribution = true)
{
    list.setObjectName("workspaceNavigation");
    list.setSpacing(6);
    list.setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    const auto names = titles();
    const int count = includeContribution ? names.size() : names.size() - 1;
    for (int index = 0; index < count; ++index) {
        auto *item = new QListWidgetItem(names[index], &list);
        item->setSizeHint(QSize(140, 42));
    }
}

bool currentFullyVisible(const QListWidget &list)
{
    return list.currentItem() && list.viewport()->rect().contains(list.visualItemRect(list.currentItem()));
}

QStringList observedTitles(const QListWidget &list)
{
    QStringList result;
    for (int index = 0; index < list.count(); ++index)
        result.append(list.item(index)->text());
    return result;
}
}

class WorkspaceNavigationListTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void selectedContributionSurvivesResize();
    void reselectingCurrentItemRestoresVisibility();
    void normalShowAndDynamicContribution();
};

void WorkspaceNavigationListTest::initTestCase()
{
    QApplication::setStyle(QStyleFactory::create(QStringLiteral("Fusion")));
    QString sheet;
    for (const QString &name : {"pipeon.qss", "pipeon-light.qss"}) {
        const QString path = QFINDTESTDATA("../resources/theme/" + name);
        QVERIFY2(!path.isEmpty(), qPrintable(name));
        QFile file(path);
        QVERIFY(file.open(QIODevice::ReadOnly));
        sheet += QString::fromUtf8(file.readAll());
    }
    qApp->setStyleSheet(sheet);
}

void WorkspaceNavigationListTest::selectedContributionSurvivesResize()
{
    WorkspaceNavigationList list;
    populate(list);
    list.resize(164, 800);
    list.show();
    list.setCurrentRow(10);
    QTRY_VERIFY(currentFullyVisible(list));
    QSignalSpy selection(&list, &QListWidget::currentRowChanged);
    QSignalSpy activation(&list, &QListWidget::itemActivated);
    list.resize(164, 584);
    QTRY_VERIFY(currentFullyVisible(list));
    QCOMPARE(list.currentRow(), 10);
    QCOMPARE(observedTitles(list), titles());
    QCOMPARE(selection.count(), 0);
    QCOMPARE(activation.count(), 0);
}

void WorkspaceNavigationListTest::reselectingCurrentItemRestoresVisibility()
{
    WorkspaceNavigationList list;
    populate(list);
    list.resize(164, 584);
    list.show();
    list.setCurrentRow(10);
    QTRY_VERIFY(currentFullyVisible(list));
    list.verticalScrollBar()->setValue(0);
    QVERIFY(!currentFullyVisible(list));
    QSignalSpy selection(&list, &QListWidget::currentRowChanged);
    QSignalSpy activation(&list, &QListWidget::itemActivated);
    // selectPage must also restore an already selected page without reactivation.
    list.setCurrentRow(10);
    list.ensureCurrentItemVisible();
    QTRY_VERIFY(currentFullyVisible(list));
    QCOMPARE(list.currentRow(), 10);
    QCOMPARE(selection.count(), 0);
    QCOMPARE(activation.count(), 0);
}

void WorkspaceNavigationListTest::normalShowAndDynamicContribution()
{
    WorkspaceNavigationList list;
    populate(list, false);
    list.resize(164, 584);
    list.setCurrentRow(2);
    list.show();
    QTRY_VERIFY(currentFullyVisible(list));
    QSignalSpy selection(&list, &QListWidget::currentRowChanged);
    QSignalSpy activation(&list, &QListWidget::itemActivated);
    auto *item = new QListWidgetItem(titles().last());
    item->setSizeHint(QSize(140, 42));
    list.addItem(item);
    QCoreApplication::processEvents();
    QCOMPARE(list.currentRow(), 2);
    QCOMPARE(selection.count(), 0);
    list.setCurrentRow(10);
    list.ensureCurrentItemVisible();
    QTRY_VERIFY(currentFullyVisible(list));
    QCOMPARE(observedTitles(list), titles());
    QCOMPARE(selection.count(), 1);
    QCOMPARE(activation.count(), 0);
}

QTEST_MAIN(WorkspaceNavigationListTest)
#include "WorkspaceNavigationListTest.moc"
