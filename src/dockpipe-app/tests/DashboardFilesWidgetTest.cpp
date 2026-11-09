#include "DashboardFilesWidget.h"
#include "DashboardFilesController.h"
#include "WorkspaceFileModel.h"
#include "WorkspaceFilesTestSupport.h"
#include "FileEditorWorkspace.h"
#include "ProjectFilesWidget.h"
#include "FileExplorerView.h"
#include "FileExplorerIcons.h"
#include "CanvasIcons.h"
#include "Theme.h"

#include <QDir>
#include <QFile>
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
#include <QTextDocument>
#include <QToolButton>
#include <QStyle>
#include <QMessageBox>
#include <QTimer>
#include <QStandardItemModel>
#include <QJsonArray>
#include <QPointer>
#include <memory>
#include <vector>

class DashboardFilesWidgetTest final : public QObject {
    Q_OBJECT
private slots:
    void projectNavigationAndClearing();
    void independentSystemNavigation();
    void linksAndFileOpening();
    void fileIconPixelDensity_data();
    void fileIconPixelDensity();
    void folderViewUsesSharedIcons_data();
    void folderViewUsesSharedIcons();
    void filesPageUsesSharedExplorer();
    void filesPageRefreshesOnlyWhileVisible();
    void symbolicParentUsesCanonicalNavigation();
    void boundedPagesAndMachineContexts();
    void enterActivatesBothViews_data();
    void enterActivatesBothViews();
};

void DashboardFilesWidgetTest::filesPageRefreshesOnlyWhileVisible()
{
    QTemporaryDir directory;
    ProjectFilesWidget files;
    QVERIFY(FilesTest::setRoot(files, directory.path()));
    auto *model = files.findChild<WorkspaceFileModel *>();
    files.show();
    QFile first(directory.filePath("first.txt"));
    QVERIFY(first.open(QIODevice::WriteOnly));
    first.close();
    QTRY_VERIFY_WITH_TIMEOUT(model->indexForPath("first.txt").isValid(), 7000);
    files.hide();
    // Finish the existing read, then verify hidden pages do not poll.
    QTest::qWait(100);
    QSignalSpy loaded(model, &WorkspaceFileModel::directoryLoaded);
    QFile second(directory.filePath("second.txt"));
    QVERIFY(second.open(QIODevice::WriteOnly));
    second.close();
    QTest::qWait(5200);
    QCOMPARE(loaded.count(), 0);
    QVERIFY(!model->indexForPath("second.txt").isValid());
    files.show();
    QTRY_VERIFY_WITH_TIMEOUT(model->indexForPath("second.txt").isValid(), 2000);
}

void DashboardFilesWidgetTest::projectNavigationAndClearing()
{
    QTemporaryDir directory;
    const QString root = directory.path() + "/project";
    QVERIFY(QDir().mkpath(root + "/nested/deeper"));
    DashboardFilesWidget files;
    auto *tree = files.findChild<QTreeView *>("dashboardFiles");
    auto *model = files.findChild<WorkspaceFileModel *>();
    auto *up = files.findChild<QPushButton *>("dashboardFilesUp");
    auto *home = files.findChild<QPushButton *>("dashboardFilesHome");
    auto *location = files.findChild<QLineEdit *>("dashboardFilesLocation");
    QVERIFY(tree->isHidden());
    QVERIFY(!home->isEnabled());
    files.setContext("project", root);
    QTRY_COMPARE(files.currentDirectory(), root);
    QVERIFY(!tree->isHidden());
    QCOMPARE(model->relativePath({}), QString("."));
    QVERIFY(!up->isEnabled());
    QVERIFY(!files.navigateTo(".."));
    QVERIFY(!files.navigateTo(directory.path()));
    QVERIFY(files.navigateTo("nested/deeper"));
    QTRY_COMPARE(files.currentDirectory(), root + "/nested/deeper");
    files.setContext("project", root);
    QCOMPARE(files.currentDirectory(), root + "/nested/deeper");
    up->click();
    QTRY_COMPARE(files.currentDirectory(), root + "/nested");
    location->setText(root);
    QMetaObject::invokeMethod(location, "returnPressed");
    QTRY_COMPARE(files.currentDirectory(), root);
    QVERIFY(!files.navigateTo(QString("bad") + QChar::Null + "path"));
    QVERIFY(!files.navigateTo(QString(4097, 'x')));
    QVERIFY(!files.navigateTo("https://example.invalid"));
    QSignalSpy failed(model, &WorkspaceFileModel::listingFailed);
    files.setContext("project", directory.path() + "/missing");
    QTRY_VERIFY(!failed.isEmpty());
    QVERIFY(files.currentDirectory().isEmpty());
    QVERIFY(tree->isHidden());
    files.setContext("project", {});
    QVERIFY(!home->isEnabled() && !location->isEnabled());
    QVERIFY(location->text().isEmpty());
}

void DashboardFilesWidgetTest::independentSystemNavigation()
{
    QTemporaryDir directory;
    QVERIFY(QDir().mkpath(directory.path() + "/a"));
    QVERIFY(QDir().mkpath(directory.path() + "/b"));
    DashboardFilesWidget files;
    files.setContext("system", directory.path() + "/a");
    QTRY_COMPARE(files.currentDirectory(), QDir::fromNativeSeparators(QDir::homePath()));
    QVERIFY(files.navigateTo(directory.path() + "/b"));
    QTRY_COMPARE(files.currentDirectory(), directory.path() + "/b");
    files.setContext("system", directory.path() + "/missing");
    QCOMPARE(files.currentDirectory(), directory.path() + "/b");
    files.setContext("invalid", {});
    QCOMPARE(files.scope(), QString("system"));
    QCOMPARE(files.currentDirectory(), directory.path() + "/b");
    files.findChild<QPushButton *>("dashboardFilesFilesystem")->click();
#ifdef Q_OS_WIN
    QTRY_VERIFY(files.currentDirectory().isEmpty());
#else
    QTRY_COMPARE(files.currentDirectory(), QDir::rootPath());
#endif
    files.findChild<QPushButton *>("dashboardFilesHome")->click();
    QTRY_COMPARE(files.currentDirectory(), QDir::fromNativeSeparators(QDir::homePath()));
    files.setContext("project", directory.path() + "/a");
    QTRY_COMPARE(files.currentDirectory(), directory.path() + "/a");
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
#ifndef Q_OS_WIN
    QVERIFY(QFile::link(directory.path() + "/outside", root + "/outside-link"));
    QVERIFY(QFile::link(root + "/inside", root + "/inside-link"));
#endif
    DashboardFilesWidget files;
    files.setContext("project", root);
    auto *tree = files.findChild<QTreeView *>("dashboardFiles");
    auto *filter = qobject_cast<QSortFilterProxyModel *>(tree->model());
    auto *model = files.findChild<WorkspaceFileModel *>();
    QVERIFY(filter && model && tree->uniformRowHeights());
    QTRY_COMPARE(files.currentDirectory(), root);
    QSignalSpy opened(&files, &DashboardFilesWidget::openFileRequested);
    const auto fileIndex = filter->mapFromSource(model->indexForPath("document.txt"));
    QVERIFY(fileIndex.isValid());
    QCOMPARE(fileIndex.data(WorkspaceFileModel::SizeBytesRole).toString(), QString("7"));
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, fileIndex));
    QCOMPARE(opened.size(), 1);
    QCOMPARE(opened.first().first().toString(), document.fileName());
#ifndef Q_OS_WIN
    const auto linkIndex = filter->mapFromSource(model->indexForPath("outside-link"));
    QVERIFY(linkIndex.isValid());
    QVERIFY(!linkIndex.flags().testFlag(Qt::ItemIsEnabled));
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, linkIndex));
    QCOMPARE(files.currentDirectory(), root);
    QCOMPARE(opened.size(), 1);
    QSignalSpy failures(model, &WorkspaceFileModel::listingFailed);
    QVERIFY(files.navigateTo(root + "/inside-link"));
    QTRY_COMPARE(failures.count(), 1);
    QVERIFY(files.currentDirectory().isEmpty());
    QVERIFY(files.navigateTo(root));
    QTRY_COMPARE(files.currentDirectory(), root);
#endif
    const auto directoryIndex = filter->mapFromSource(model->indexForPath("inside"));
    QVERIFY(directoryIndex.isValid());
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, directoryIndex));
    QTRY_COMPARE(files.currentDirectory(), root + "/inside");
}

void DashboardFilesWidgetTest::fileIconPixelDensity_data()
{
    QTest::addColumn<QString>("filename");
    QTest::addColumn<qreal>("scale");
    QTest::addColumn<int>("extent");
    const QStringList filenames{
        "README.md", "main.cpp", "config.yml", "data.csv", "photo.png", "release.zip", "notes.txt"};
    for (const QString &filename : filenames) {
        for (qreal scale : {qreal(1), qreal(1.25), qreal(1.5), qreal(2), qreal(3)}) {
            for (int extent : {18, 44}) {
                const QByteArray row = QString("%1-%2x-%3px").arg(filename).arg(scale).arg(extent).toUtf8();
                QTest::newRow(row.constData()) << filename << scale << extent;
            }
        }
    }
}

void DashboardFilesWidgetTest::fileIconPixelDensity()
{
    QFETCH(QString, filename);
    QFETCH(qreal, scale);
    QFETCH(int, extent);
    const QIcon icon = FileExplorerIcons::instance().icon(QFileInfo(filename));
    const QPixmap pixmap = icon.pixmap(QSize(extent, extent), scale);
    QCOMPARE(pixmap.devicePixelRatio(), scale);
    QCOMPARE(pixmap.size(), QSize(extent, extent) * scale);
    QVERIFY(!pixmap.toImage().isNull());
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
    auto *model = files.findChild<WorkspaceFileModel *>();
    auto *filter = qobject_cast<QSortFilterProxyModel *>(grid->model());
    QVERIFY(tree->header()->isHidden());
    QVERIFY(tree->property("dockpipeFileView").toBool());
    QTRY_COMPARE(filter->rowCount(grid->rootIndex()), 8);
    QCOMPARE(FileExplorerIcons::instance().iconForEntry("Documents", true).pixmap(32, 32).toImage(),
        CanvasIcons::icon(CanvasIcons::Symbol::Folder).pixmap(32, 32).toImage());
    const auto highDpiIcon = FileExplorerIcons::instance().iconForEntry("README.md", false)
        .pixmap(QSize(18, 18), qreal(2));
    QCOMPARE(highDpiIcon.devicePixelRatio(), qreal(2));
    QCOMPARE(highDpiIcon.size(), QSize(36, 36));
    QVERIFY(FileExplorerIcons::instance().iconForEntry("README.md", false).pixmap(32, 32).toImage()
        != FileExplorerIcons::instance().iconForEntry("main.cpp", false).pixmap(32, 32).toImage());
    const QString images = qEnvironmentVariable("DOCKPIPE_EXPLORER_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(files.grab().save(QDir(images).filePath("dashboard-tree-" + theme + ".png")));
    }
    const auto fileIndex = filter->mapFromSource(model->indexForPath("README.md"));
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
    const auto folderIndex = filter->mapFromSource(model->indexForPath("Documents"));
    grid->setCurrentIndex(folderIndex);
    QTest::keyClick(grid, Qt::Key_Return);
    QTRY_COMPARE(files.currentDirectory(), root.filePath("Documents"));
    files.findChild<QPushButton *>("dashboardFilesUp")->click();
    QTRY_COMPARE(files.currentDirectory(), root.path());
    QCOMPARE(explorer->mode(), FileExplorerView::Mode::Folders);
    files.hide();
    files.show();
    QCOMPARE(explorer->mode(), FileExplorerView::Mode::Folders);
    QSignalSpy failures(model, &WorkspaceFileModel::listingFailed);
    files.setContext("project", root.filePath("missing"));
    QTRY_VERIFY(!failures.isEmpty());
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
    QVERIFY(FilesTest::setRoot(files, root.path()));
    QVERIFY(FilesTest::open(files, root.filePath("README.md")));
    auto *explorer = files.findChild<FileExplorerView *>();
    auto *grid = explorer->iconView();
    auto *model = files.findChild<WorkspaceFileModel *>();
    auto *proxy = qobject_cast<QSortFilterProxyModel *>(grid->model());
    auto *editor = files.findChild<QPlainTextEdit *>("fileTextEditor");
    editor->appendPlainText("Draft");
    const QPointer<QTextDocument> draft = files.currentDocument();
    const QString text = editor->toPlainText();
    explorer->setMode(FileExplorerView::Mode::Folders);
    QTRY_VERIFY(model->indexForPath("nested").isValid());
    grid->setCurrentIndex(proxy->mapFromSource(model->indexForPath("nested")));
    QTest::keyClick(grid, Qt::Key_Return);
    QTRY_COMPARE(model->relativePath({}), QString("nested"));
    QTRY_VERIFY(model->indexForPath("nested/other.txt").isValid());
    grid->setCurrentIndex(proxy->mapFromSource(model->indexForPath("nested/other.txt")));
    QTest::keyClick(grid, Qt::Key_Return);
    QTRY_COMPARE(files.currentPath(), root.filePath("nested/other.txt"));
    QVERIFY(draft && draft->isModified());
    QCOMPARE(draft->toPlainText(), text);
    files.findChild<QToolButton *>("fileExplorerRoot")->click();
    QTRY_COMPARE(model->relativePath({}), QString("."));
    QVERIFY(files.hasUnsavedChanges());
}

void DashboardFilesWidgetTest::symbolicParentUsesCanonicalNavigation()
{
#ifdef Q_OS_WIN
    QSKIP("Directory symlinks require host-specific Windows privileges.");
#else
    QTemporaryDir directory;
    QTemporaryDir aliases;
    const QString root = directory.filePath("project");
    const QString alias = aliases.filePath("parent");
    QVERIFY(QDir().mkpath(root + "/nested"));
    QVERIFY(QFile::link(directory.path(), alias));
    DashboardFilesWidget dashboard;
    dashboard.setContext("project", alias + "/project");
    // Root selection is trusted composition; core pins it. The view preserves
    // the selected spelling and performs no canonicalization of host metadata.
    QTRY_COMPARE(dashboard.currentDirectory(), alias + "/project");
    QVERIFY(!dashboard.navigateTo(".."));
    QVERIFY(dashboard.navigateTo("nested"));
    QTRY_COMPARE(dashboard.currentDirectory(), alias + "/project/nested");
    dashboard.findChild<QPushButton *>("dashboardFilesUp")->click();
    QTRY_COMPARE(dashboard.currentDirectory(), alias + "/project");
#endif
}

void DashboardFilesWidgetTest::boundedPagesAndMachineContexts()
{
    struct Call {
        WorkspaceFilesClient::Context context;
        QJsonObject request;
        WorkspaceFilesClient::Completion completion;
        bool cancelled = false;
    };
    std::vector<std::shared_ptr<Call>> calls;
    const auto dispatch = [&calls](const auto &context, const auto &request, auto completion) {
        auto call = std::make_shared<Call>();
        call->context = context;
        call->request = request;
        call->completion = std::move(completion);
        calls.push_back(call);
        return WorkspaceFilesClient::Cancel([call]() { call->cancelled = true; });
    };
    const auto respond = [](const std::shared_ptr<Call> &call, const QString &name) {
        const QJsonArray entries{QJsonObject{{"name", name}, {"kind", "file"}, {"size_bytes", "9007199254740993"}}};
        const QJsonObject value{{"schema", "dockpipe.files/v1"}, {"path", call->request.value("path")},
            {"revision", QString(64, 'c')}, {"entries", entries}};
        call->completion({WorkspaceFilesClient::Outcome::Success, value, {}, call->context});
    };
    QTemporaryDir root;
    for (int index = 0; index < 151; ++index) {
        QFile file(root.filePath(QString("local-%1.txt").arg(index, 3, 10, QChar('0'))));
        QVERIFY(file.open(QIODevice::WriteOnly));
        QCOMPARE(file.write("local"), qint64(5));
    }
    DashboardFilesWidget files;
    files.setContext("project", root.path());
    auto *model = files.findChild<WorkspaceFileModel *>();
    auto *controller = DashboardFilesController::get(&files);
    auto *tree = files.findChild<QTreeView *>("dashboardFiles");
    auto *proxy = qobject_cast<QSortFilterProxyModel *>(tree->model());
    QTRY_COMPARE(files.currentDirectory(), root.path());
    QTRY_VERIFY(model->rowCount() >= 150);
    if (model->canFetchMore({}))
        model->fetchMore({});
    QTRY_COMPARE(model->rowCount(), 151);
    controller->setRemoteDispatch(dispatch);
    const QJsonObject invalid{{"node", "worker"}};
    QString error;
    QVERIFY(!files.setMachineContext(invalid, "root-a", root.path(), &error));
    QVERIFY(!error.isEmpty());
    QCOMPARE(files.machineTarget(), invalid);
    QVERIFY(files.currentDirectory().isEmpty());
    QVERIFY(tree->isHidden());
    QVERIFY(!files.navigateTo(root.path()));

    const QJsonObject first{{"node", "worker-a"}, {"fingerprint", QString(64, 'a')}, {"generation", QString(64, 'b')}};
    const QJsonObject second{{"node", "worker-b"}, {"fingerprint", QString(64, 'd')}, {"generation", QString(64, 'e')}};
    QVERIFY(files.setMachineContext(first, "root-a", root.path(), &error));
    QTRY_VERIFY(!calls.empty());
    const auto oldCall = calls.back();
    QCOMPARE(oldCall->context.nodeId, QString("worker-a"));
    QCOMPARE(oldCall->context.rootId, QString("root-a"));
    QVERIFY(oldCall->context.localRoot.isEmpty());
    QVERIFY(files.setMachineContext(second, "root-b", root.path(), &error));
    QTRY_VERIFY(calls.back() != oldCall);
    const auto currentCall = calls.back();
    QVERIFY(oldCall->cancelled);
    respond(oldCall, "stale-local-looking.txt");
    respond(currentCall, "hardware-key-remote.txt");
    QTRY_COMPARE(files.currentDirectory(), QString("/"));
    QCOMPARE(files.machineTarget(), second);
    QCOMPARE(files.remoteRootId(), QString("root-b"));
    QCOMPARE(model->rowCount(), 1);
    QCOMPARE(model->index(0, 0).data().toString(), QString("hardware-key-remote.txt"));
    QCOMPARE(model->index(0, 0).data(WorkspaceFileModel::SizeBytesRole).toString(), QString("9007199254740993"));
    QVERIFY(files.findChild<QPushButton *>("dashboardFilesFilesystem")->isHidden());
    QSignalSpy opened(&files, &DashboardFilesWidget::openFileRequested);
    const auto remoteIndex = proxy->mapFromSource(model->index(0, 0));
    QMetaObject::invokeMethod(tree, "doubleClicked", Q_ARG(QModelIndex, remoteIndex));
    QCOMPARE(opened.count(), 1);
    QCOMPARE(opened.first().first().toString(), QString("/hardware-key-remote.txt"));
    QVERIFY(files.navigateTo("/folder"));
    QTRY_VERIFY(calls.back() != currentCall);
    const auto directoryCall = calls.back();
    QCOMPARE(directoryCall->request.value("path").toString(), QString("folder"));
    QCOMPARE(directoryCall->context.workerGeneration, QString(64, 'e'));
    respond(directoryCall, "nested.txt");
    QTRY_COMPARE(files.currentDirectory(), QString("/folder"));
    QVERIFY(files.setMachineContext({}, {}, root.path(), &error));
    QTRY_COMPARE(files.currentDirectory(), root.path());
    QVERIFY(files.machineTarget().isEmpty());
    QVERIFY(files.remoteRootId().isEmpty());

    auto pending = std::make_unique<DashboardFilesWidget>();
    DashboardFilesController::get(pending.get())->setRemoteDispatch(dispatch);
    QVERIFY(pending->setMachineContext(first, "root-a", {}, &error));
    const auto ownedCall = calls.back();
    pending.reset();
    QVERIFY(ownedCall->cancelled);
    respond(ownedCall, "ignored-after-destruction.txt");
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
