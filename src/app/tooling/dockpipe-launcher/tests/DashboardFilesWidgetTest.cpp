#include "DashboardFilesWidget.h"

#include <QDir>
#include <QFile>
#include <QFileSystemModel>
#include <QLineEdit>
#include <QPushButton>
#include <QSignalSpy>
#include <QSortFilterProxyModel>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeView>

class DashboardFilesWidgetTest final : public QObject {
    Q_OBJECT
private slots:
    void projectNavigationAndClearing();
    void independentSystemNavigation();
    void linksAndFileOpening();
};

void DashboardFilesWidgetTest::projectNavigationAndClearing()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString root = directory.path() + "/project";
    QVERIFY(QDir().mkpath(root + "/nested/deeper"));
    DashboardFilesWidget files;
    auto *tree = files.findChild<QTreeView *>("dashboardFiles");
    auto *up = files.findChild<QPushButton *>("dashboardFilesUp");
    auto *home = files.findChild<QPushButton *>("dashboardFilesHome");
    auto *location = files.findChild<QLineEdit *>("dashboardFilesLocation");
    QCOMPARE(files.scope(), QString("project"));
    QVERIFY(tree->isHidden());
    QVERIFY(!home->isEnabled() && !location->isEnabled());
    files.setContext("project", root);
    QCOMPARE(files.currentDirectory(), QFileInfo(root).canonicalFilePath());
    QVERIFY(tree->rootIndex().isValid());
    QVERIFY(!tree->isHidden());
    QVERIFY(!up->isEnabled());
    QVERIFY(!files.navigateTo(".."));
    QVERIFY(!files.navigateTo(directory.path()));
    QVERIFY(files.navigateTo("nested/deeper"));
    files.setContext("project", root);
    QCOMPARE(files.currentDirectory(), QFileInfo(root + "/nested/deeper").canonicalFilePath());
    up->click();
    QCOMPARE(files.currentDirectory(), QFileInfo(root + "/nested").canonicalFilePath());
    location->setText(root);
    QMetaObject::invokeMethod(location, "returnPressed");
    QCOMPARE(files.currentDirectory(), QFileInfo(root).canonicalFilePath());
    QVERIFY(!files.navigateTo(QString("bad") + QChar::Null + "path"));
    QVERIFY(!files.navigateTo(QString(4097, 'x')));
    QVERIFY(!files.navigateTo("https://example.invalid"));
    files.setContext("project", directory.path() + "/missing");
    QVERIFY(files.currentDirectory().isEmpty());
    QVERIFY(tree->isHidden() && !tree->rootIndex().isValid());
    QVERIFY(!home->isEnabled() && !location->isEnabled());
    QVERIFY(location->text().isEmpty());
    files.setContext("project", {});
    QVERIFY(tree->isHidden());
}

void DashboardFilesWidgetTest::independentSystemNavigation()
{
    QTemporaryDir directory;
    QVERIFY(QDir().mkpath(directory.path() + "/a"));
    QVERIFY(QDir().mkpath(directory.path() + "/b"));
    DashboardFilesWidget files;
    files.setContext("system", directory.path() + "/a");
    QCOMPARE(files.currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    QVERIFY(files.navigateTo(directory.path() + "/b"));
    const QString retained = files.currentDirectory();
    files.setContext("system", directory.path() + "/missing");
    QCOMPARE(files.currentDirectory(), retained);
    files.setContext("invalid", {});
    QCOMPARE(files.scope(), QString("system"));
    QCOMPARE(files.currentDirectory(), retained);
    files.findChild<QPushButton *>("dashboardFilesFilesystem")->click();
#ifdef Q_OS_WIN
    QVERIFY(files.currentDirectory().isEmpty());
    QVERIFY(!files.findChild<QTreeView *>("dashboardFiles")->rootIndex().isValid());
#else
    QCOMPARE(files.currentDirectory(), QDir::rootPath());
#endif
    files.findChild<QPushButton *>("dashboardFilesHome")->click();
    QCOMPARE(files.currentDirectory(), QFileInfo(QDir::homePath()).canonicalFilePath());
    files.setContext("project", directory.path() + "/a");
    QVERIFY(!files.navigateTo(directory.path() + "/b"));
    QVERIFY(files.findChild<QPushButton *>("dashboardFilesFilesystem")->isHidden());
}

void DashboardFilesWidgetTest::linksAndFileOpening()
{
    QTemporaryDir directory;
    const QString root = directory.path() + "/project";
    QVERIFY(QDir().mkpath(root + "/inside"));
    QVERIFY(QDir().mkpath(directory.path() + "/outside"));
    QFile document(root + "/document.txt");
    QVERIFY(document.open(QIODevice::WriteOnly));
    document.write("fixture");
    document.close();
    DashboardFilesWidget files;
    files.setContext("project", root);
    auto *tree = files.findChild<QTreeView *>("dashboardFiles");
    auto *filter = qobject_cast<QSortFilterProxyModel *>(tree->model());
    auto *model = files.findChild<QFileSystemModel *>();
    QVERIFY(filter && model && model->isReadOnly());
    QVERIFY(tree->uniformRowHeights());
    QSignalSpy opened(&files, &DashboardFilesWidget::openFileRequested);
    QModelIndex fileIndex;
    QTRY_VERIFY((fileIndex = filter->mapFromSource(model->index(document.fileName()))).isValid());
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, fileIndex));
    QCOMPARE(opened.size(), 1);
    QCOMPARE(opened.first().first().toString(), QFileInfo(document).canonicalFilePath());
    QModelIndex directoryIndex;
    QTRY_VERIFY((directoryIndex = filter->mapFromSource(model->index(root + "/inside"))).isValid());
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, directoryIndex));
    QCOMPARE(files.currentDirectory(), QFileInfo(root + "/inside").canonicalFilePath());
#ifndef Q_OS_WIN
    QVERIFY(QFile::link(directory.path() + "/outside", root + "/outside-link"));
    QVERIFY(QFile::link(root + "/inside", root + "/inside-link"));
    QVERIFY(QFile::link(directory.path(), root + "/parent-link"));
    QFile outsideDocument(directory.path() + "/outside/private.txt");
    QVERIFY(outsideDocument.open(QIODevice::WriteOnly));
    outsideDocument.write("outside fixture");
    outsideDocument.close();
    QVERIFY(QFile::link(outsideDocument.fileName(), root + "/outside-file"));
    QVERIFY(files.navigateTo(root));
    QVERIFY(!files.navigateTo("outside-link"));
    QVERIFY(!files.navigateTo("parent-link"));
    QVERIFY(!filter->mapFromSource(model->index(root + "/outside-link")).isValid());
    QVERIFY(!filter->mapFromSource(model->index(root + "/parent-link")).isValid());
    const QModelIndex outsideIndex = filter->mapFromSource(model->index(root + "/outside-file"));
    QVERIFY(!outsideIndex.isValid());
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, outsideIndex));
    QCOMPARE(opened.size(), 1);
    QVERIFY(files.navigateTo("inside-link"));
    QCOMPARE(files.currentDirectory(), QFileInfo(root + "/inside").canonicalFilePath());
    files.setContext("system", root);
    QVERIFY(files.navigateTo(root + "/outside-link"));
    QCOMPARE(files.currentDirectory(), QFileInfo(directory.path() + "/outside").canonicalFilePath());
#endif
}

QTEST_MAIN(DashboardFilesWidgetTest)
#include "DashboardFilesWidgetTest.moc"
