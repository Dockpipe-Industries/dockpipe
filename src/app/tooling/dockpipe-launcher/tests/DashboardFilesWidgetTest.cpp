#include "DashboardFilesWidget.h"
#include "ProjectFilesWidget.h"
#include "FileExplorerView.h"
#include "CanvasIcons.h"
#include "Theme.h"

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
#include <QHeaderView>
#include <QFileIconProvider>
#include <QListView>
#include <QPlainTextEdit>
#include <QToolButton>
#include <QStyle>
#include <QMessageBox>
#include <QTimer>
#include <QStandardItemModel>

class DashboardFilesWidgetTest final : public QObject {
    Q_OBJECT
private slots:
    void projectNavigationAndClearing();
    void independentSystemNavigation();
    void linksAndFileOpening();
    void folderViewUsesSharedIcons_data();
    void folderViewUsesSharedIcons();
    void filesPageUsesSharedExplorer();
    void symbolicParentUsesCanonicalNavigation();
    void enterActivatesBothViews_data();
    void enterActivatesBothViews();
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
    QTRY_VERIFY((fileIndex = filter->mapFromSource(model->index(QFileInfo(document.fileName()).canonicalFilePath()))).isValid());
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, fileIndex));
    QCOMPARE(opened.size(), 1);
    QCOMPARE(opened.first().first().toString(), QFileInfo(document).canonicalFilePath());
    QModelIndex directoryIndex;
    QTRY_VERIFY((directoryIndex = filter->mapFromSource(model->index(QFileInfo(root + "/inside").canonicalFilePath()))).isValid());
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

void DashboardFilesWidgetTest::folderViewUsesSharedIcons_data()
{
    QTest::addColumn<QString>("theme");
    QTest::newRow("dark") << QString("dark");
    QTest::newRow("light") << QString("light");
}

void DashboardFilesWidgetTest::folderViewUsesSharedIcons()
{
    QFETCH(QString, theme);
    applyDockpipeLauncherTheme(*qApp);
    QFile shared(":/theme/pipeon.qss");
    QFile companion(":/theme/pipeon-" + theme + ".qss");
    QVERIFY(shared.open(QIODevice::ReadOnly));
    QVERIFY(companion.open(QIODevice::ReadOnly));
    qApp->setStyleSheet({});
    QPalette palette = qApp->style()->standardPalette();
    if (theme == "dark") {
        palette.setColor(QPalette::Window, QColor("#191d24"));
        palette.setColor(QPalette::Base, QColor("#141920"));
        palette.setColor(QPalette::Text, QColor("#dce5f0"));
        palette.setColor(QPalette::WindowText, QColor("#dce5f0"));
    }
    qApp->setPalette(palette);
    qApp->setStyleSheet(QString::fromUtf8(shared.readAll() + companion.readAll()));
    QTemporaryDir root;
    QVERIFY(QDir().mkpath(root.filePath("Documents")));
    for (const auto &name : {"README.md", "workflow.yaml", "main.cpp", "data.csv", "photo.png", "release.zip", "notes.txt"}) {
        QFile file(root.filePath(name));
        QVERIFY(file.open(QIODevice::WriteOnly));
        file.write("fixture");
    }
    DashboardFilesWidget files;
    files.resize(640, 520);
    files.setContext("project", root.path());
    files.show();
    auto *explorer = files.findChild<FileExplorerView *>();
    auto *tree = explorer->treeView();
    auto *grid = explorer->iconView();
    auto *model = files.findChild<QFileSystemModel *>();
    auto *filter = qobject_cast<QSortFilterProxyModel *>(grid->model());
    QVERIFY(tree->header()->isHidden());
    QVERIFY(tree->property("dockpipeFileView").toBool());
    QTRY_COMPARE(filter->rowCount(grid->rootIndex()), 8);
    QCOMPARE(model->iconProvider()->icon(QFileInfo(root.filePath("Documents"))).pixmap(32, 32).toImage(),
        CanvasIcons::icon(CanvasIcons::Symbol::Folder).pixmap(32, 32).toImage());
    const auto highDpiIcon = model->iconProvider()->icon(QFileInfo(root.filePath("README.md")))
        .pixmap(QSize(18, 18), qreal(2));
    QCOMPARE(highDpiIcon.devicePixelRatio(), qreal(2));
    QCOMPARE(highDpiIcon.size(), QSize(36, 36));
    QVERIFY(model->iconProvider()->icon(QFileInfo(root.filePath("README.md"))).pixmap(32, 32).toImage()
        != model->iconProvider()->icon(QFileInfo(root.filePath("main.cpp"))).pixmap(32, 32).toImage());
    const QString images = qEnvironmentVariable("DOCKPIPE_EXPLORER_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(files.grab().save(QDir(images).filePath("dashboard-tree-" + theme + ".png")));
    }
    const auto fileIndex = filter->mapFromSource(model->index(QFileInfo(root.filePath("README.md")).canonicalFilePath()));
    tree->setCurrentIndex(fileIndex);
    QTest::mouseClick(files.findChild<QToolButton *>("explorerFoldersMode"), Qt::LeftButton);
    QVERIFY(tree->isHidden());
    QVERIFY(grid->isVisible());
    QCOMPARE(grid->currentIndex(), fileIndex);
    QSignalSpy opened(&files, &DashboardFilesWidget::openFileRequested);
    grid->setFocus();
    QTest::keyClick(grid, Qt::Key_Return);
    QCOMPARE(opened.size(), 1);
    QCOMPARE(opened.first().first().toString(), QFileInfo(root.filePath("README.md")).canonicalFilePath());
    if (!images.isEmpty()) {
        QVERIFY(files.grab().save(QDir(images).filePath("dashboard-folders-" + theme + ".png")));
    }
    const auto folderIndex = filter->mapFromSource(model->index(QFileInfo(root.filePath("Documents")).canonicalFilePath()));
    grid->setCurrentIndex(folderIndex);
    QTest::keyClick(grid, Qt::Key_Return);
    QCOMPARE(files.currentDirectory(), QFileInfo(root.filePath("Documents")).canonicalFilePath());
    files.findChild<QPushButton *>("dashboardFilesUp")->click();
    QCOMPARE(files.currentDirectory(), QFileInfo(root.path()).canonicalFilePath());
    QCOMPARE(explorer->mode(), FileExplorerView::Mode::Folders);
    files.hide();
    files.show();
    QCOMPARE(explorer->mode(), FileExplorerView::Mode::Folders);
    files.setContext("project", root.filePath("missing"));
    QVERIFY(grid->isHidden());
    QVERIFY(tree->isHidden());
}

void DashboardFilesWidgetTest::filesPageUsesSharedExplorer()
{
    QTemporaryDir root;
    QVERIFY(QDir().mkpath(root.filePath("nested")));
    for (const auto &name : {"README.md", "nested/other.txt"}) {
        QFile file(root.filePath(name));
        QVERIFY(file.open(QIODevice::WriteOnly));
        file.write("fixture");
    }
    ProjectFilesWidget files;
    files.resize(1180, 760);
    files.show();
    QVERIFY(files.setRoot(root.path()));
    QVERIFY(files.openFile(root.filePath("README.md")));
    auto *explorer = files.findChild<FileExplorerView *>();
    auto *grid = explorer->iconView();
    auto *model = files.findChild<QFileSystemModel *>();
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    editor->appendPlainText("Draft");
    const QString draft = editor->toPlainText();
    QTest::mouseClick(files.findChild<QToolButton *>("explorerFoldersMode"), Qt::LeftButton);
    QTRY_VERIFY(grid->isVisible());
    QModelIndex directory;
    QTRY_VERIFY((directory = model->index(QFileInfo(root.filePath("nested")).canonicalFilePath())).isValid());
    grid->setCurrentIndex(directory);
    grid->setFocus();
    QTest::keyClick(grid, Qt::Key_Return);
    QCOMPARE(grid->rootIndex(), directory);
    QCOMPARE(editor->toPlainText(), draft);
    QTRY_VERIFY(model->index(QFileInfo(root.filePath("nested/other.txt")).canonicalFilePath()).isValid());
    grid->setCurrentIndex(model->index(QFileInfo(root.filePath("nested/other.txt")).canonicalFilePath()));
    QTimer::singleShot(0, []() {
        for (auto *widget : QApplication::topLevelWidgets()) {
            if (auto *dialog = qobject_cast<QMessageBox *>(widget))
                dialog->done(QMessageBox::Cancel);
        }
    });
    QTest::keyClick(grid, Qt::Key_Return);
    QCOMPARE(files.currentPath(), root.filePath("README.md"));
    QCOMPARE(editor->toPlainText(), draft);
    files.findChild<QToolButton *>("fileExplorerRoot")->click();
    QCOMPARE(grid->rootIndex(), model->index(QFileInfo(root.path()).canonicalFilePath()));
    QVERIFY(files.hasUnsavedChanges());
}

void DashboardFilesWidgetTest::symbolicParentUsesCanonicalNavigation()
{
#ifdef Q_OS_WIN
    QSKIP("Directory symlinks require host-specific Windows privileges.");
#else
    QTemporaryDir directory;
    const QString realRoot = directory.filePath("project");
    QTemporaryDir aliases;
    const QString parentAlias = aliases.filePath("parent");
    const QString aliasRoot = parentAlias + "/project";
    QVERIFY(QDir().mkpath(realRoot + "/nested"));
    QFile document(realRoot + "/README.md");
    QVERIFY(document.open(QIODevice::WriteOnly));
    document.write("fixture");
    document.close();
    // macOS /var is an alias of /private/var: reproduce a linked ancestor,
    // while retaining the editor policy against directly linked root entries.
    QVERIFY(QFile::link(directory.path(), parentAlias));
    const QString canonicalRoot = QFileInfo(realRoot).canonicalFilePath();

    DashboardFilesWidget dashboard;
    dashboard.setContext("project", aliasRoot);
    QCOMPARE(dashboard.currentDirectory(), canonicalRoot);
    auto *dashboardTree = dashboard.findChild<QTreeView *>("dashboardFiles");
    auto *filter = qobject_cast<QSortFilterProxyModel *>(dashboardTree->model());
    auto *dashboardModel = dashboard.findChild<QFileSystemModel *>();
    QModelIndex documentIndex;
    QTRY_VERIFY((documentIndex = filter->mapFromSource(dashboardModel->index(
        canonicalRoot + "/README.md"))).isValid());
    QSignalSpy opened(&dashboard, &DashboardFilesWidget::openFileRequested);
    QMetaObject::invokeMethod(dashboardTree, "doubleClicked", Q_ARG(QModelIndex, documentIndex));
    QCOMPARE(opened.size(), 1);
    QCOMPARE(opened.first().first().toString(), canonicalRoot + "/README.md");
    QVERIFY(!dashboard.navigateTo(".."));

    ProjectFilesWidget files;
    QVERIFY(files.setRoot(aliasRoot));
    auto *explorer = files.findChild<FileExplorerView *>();
    auto *model = files.findChild<QFileSystemModel *>();
    auto *up = files.findChild<QToolButton *>("fileExplorerUp");
    QVERIFY(up);
    QVERIFY(!up->isEnabled());
    QCOMPARE(model->filePath(explorer->treeView()->rootIndex()), canonicalRoot);
    explorer->setMode(FileExplorerView::Mode::Folders);
    QModelIndex nested;
    QTRY_VERIFY((nested = model->index(canonicalRoot + "/nested")).isValid());
    explorer->iconView()->setCurrentIndex(nested);
    QTest::keyClick(explorer->iconView(), Qt::Key_Return);
    QCOMPARE(model->filePath(explorer->iconView()->rootIndex()), canonicalRoot + "/nested");
    QVERIFY(up->isEnabled());
    up->click();
    QCOMPARE(model->filePath(explorer->iconView()->rootIndex()), canonicalRoot);
    QVERIFY(!up->isEnabled());
#endif
}

void DashboardFilesWidgetTest::enterActivatesBothViews_data()
{
    QTest::addColumn<bool>("folders");
    QTest::addColumn<int>("key");
    QTest::newRow("tree-return") << false << int(Qt::Key_Return);
    QTest::newRow("tree-enter") << false << int(Qt::Key_Enter);
    QTest::newRow("folders-return") << true << int(Qt::Key_Return);
    QTest::newRow("folders-enter") << true << int(Qt::Key_Enter);
}

void DashboardFilesWidgetTest::enterActivatesBothViews()
{
    QFETCH(bool, folders);
    QFETCH(int, key);
    QStandardItemModel model;
    model.appendRow(new QStandardItem("document.md"));
    FileExplorerView explorer;
    explorer.setModel(&model);
    explorer.setMode(folders ? FileExplorerView::Mode::Folders : FileExplorerView::Mode::Tree);
    QAbstractItemView *view = folders ? static_cast<QAbstractItemView *>(explorer.iconView())
                                     : static_cast<QAbstractItemView *>(explorer.treeView());
    view->setCurrentIndex(model.index(0, 0));
    QSignalSpy activated(&explorer, &FileExplorerView::activated);
    QTest::keyClick(view, Qt::Key(key));
    QCOMPARE(activated.size(), 1);
    QCOMPARE(qvariant_cast<QModelIndex>(activated.first().first()), model.index(0, 0));
}

QTEST_MAIN(DashboardFilesWidgetTest)
#include "DashboardFilesWidgetTest.moc"
