#include "ProjectFilesWidget.h"
#include "WorkspaceGraphWidget.h"
#include "WorkspaceGraphPage.h"
#include "WorkspaceGraphReader.h"
#include "WorkspaceGraphNode.h"
#include "WorkspacePackageManager.h"
#include "Theme.h"
#include <QAction>
#include <QApplication>
#include <QDir>
#include <QFile>
#include <QGraphicsItem>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLayout>
#include <QPointer>
#include <QScreen>
#include <QPushButton>
#include <QSignalSpy>
#include <QStyle>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeView>
#include <filesystem>

class ProjectFilesGraphTest final : public QObject {
    Q_OBJECT
private slots:
    void builtInGraphDoesNotLoadPackage();
    void readerSecurityAndCursor();
    void boundedPagesProjectionAndDrone();
    void cancelledReaderDiscardsStaleReply();
    void graphButtonFitsTextAndLargerFonts_data();
    void graphButtonFitsTextAndLargerFonts();
};

static bool writeFile(const QString &path)
{
    QFile file(path);
    return file.open(QIODevice::WriteOnly) && file.write("example\n") == 8;
}

void ProjectFilesGraphTest::builtInGraphDoesNotLoadPackage()
{
    QTemporaryDir root;
    QTemporaryDir other;
    QVERIFY(root.isValid() && other.isValid());
    ProjectFilesWidget files;
    files.resize(1200, 760);
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{false});
    files.show();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    auto *tree = files.findChild<QTreeView *>("fileTree");
    QVERIFY(button && tree && button->isEnabled());
    QVERIFY(!WorkspacePackageManager::instance().ready());
    QTest::mouseClick(button, Qt::LeftButton);
    QPointer<WorkspaceGraphWidget> graph = files.findChild<WorkspaceGraphWidget *>();
    QVERIFY(graph && graph->isVisible() && !tree->isVisible());
    QTRY_VERIFY(!graph->busy());
    QVERIFY(!WorkspacePackageManager::instance().ready());
    QVERIFY(!WorkspacePackageManager::instance().busy());
    files.applyCockpitSettings(CockpitSettings{false});
    QVERIFY(graph && graph->isVisible());
    files.closeGraph();
    QTRY_VERIFY(graph.isNull());
    QVERIFY(tree->isVisible() && button->isEnabled());
    files.openGraph();
    QVERIFY(files.setRoot(other.path()));
    QTRY_VERIFY(!files.findChild<WorkspaceGraphWidget *>());
    files.openGraph();
    files.hide();
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
    QTRY_VERIFY(!files.findChild<WorkspaceGraphWidget *>());
}

void ProjectFilesGraphTest::readerSecurityAndCursor()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    QDir directory(root.path());
    QVERIFY(directory.mkpath("nested/deep"));
    QVERIFY(directory.mkdir(".git"));
    QVERIFY(writeFile(directory.filePath("README.md")));
    QVERIFY(writeFile(directory.filePath(".settings")));
    QVERIFY(writeFile(directory.filePath("nested/deep/hidden.txt")));
    const auto read = [&](const QString &path, int limit, const QString &after = {}, const QString &stamp = {}) {
        return WorkspaceGraphPage::read({{"root", root.path()}, {"directory", path},
            {"limit", limit}, {"after", after}, {"stamp", stamp}, {"hidden", true}});
    };
    const auto first = read(root.path(), 2);
    QVERIFY(!first.contains("error"));
    QCOMPARE(first.value("entries").toArray().size(), 2);
    QVERIFY(first.value("more").toBool());
    QVERIFY(!QJsonDocument(first).toJson().contains("hidden.txt"));
    const auto second = read(root.path(), 2, first.value("after").toString(), first.value("stamp").toString());
    QVERIFY(!second.contains("error"));
    QCOMPARE(second.value("entries").toArray().size(), 1);
    QVERIFY(read(directory.filePath(".."), 2).contains("error"));
    QVERIFY(read(directory.filePath(".git"), 2).contains("error"));
    QVERIFY(read(root.path(), 151).contains("error"));
    QVERIFY(read(directory.filePath("missing"), 2).contains("error"));
    const auto nativeDirectory = std::filesystem::u8path(root.path().toUtf8().constData());
    const auto timestamp = std::filesystem::last_write_time(nativeDirectory);
    QVERIFY(QFile::rename(directory.filePath("README.md"), directory.filePath("RENAMED.md")));
    std::filesystem::last_write_time(nativeDirectory, timestamp);
    QVERIFY(read(root.path(), 2, first.value("after").toString(), first.value("stamp").toString()).contains("error"));
#ifdef Q_OS_UNIX
    QVERIFY(QFile::link(root.path(), directory.filePath("cycle")));
    QVERIFY(QFile::link(directory.filePath("missing-target"), directory.filePath("broken")));
    QVERIFY(read(directory.filePath("cycle"), 2).contains("error"));
    int links = 0;
    for (const auto &entry : read(root.path(), 150).value("entries").toArray()) {
        if (entry.toObject().value("kind") == "link")
            ++links;
    }
    QCOMPARE(links, 2);
#endif
    const auto preview = WorkspaceGraphPage::read({{"operation", "preview"},
        {"root", root.path()}, {"path", directory.filePath("../outside")}});
    QVERIFY(preview.contains("error"));
}

void ProjectFilesGraphTest::cancelledReaderDiscardsStaleReply()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    WorkspaceGraphReader reader;
    QSignalSpy replies(&reader, &WorkspaceGraphReader::completed);
    const QJsonObject request{{"root", root.path()}, {"directory", root.path()}, {"limit", 5}};
    reader.request(1, request);
    reader.cancel();
    QTRY_VERIFY(!reader.busy());
    QCOMPARE(replies.size(), 0);
    reader.request(2, request);
    reader.request(3, request);
    QTRY_VERIFY(!reader.busy());
    QCOMPARE(replies.size(), 1);
    QCOMPARE(replies.first().first().toULongLong(), quint64(3));
    QVERIFY(!replies.first().at(1).toJsonObject().contains("error"));
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
}

void ProjectFilesGraphTest::boundedPagesProjectionAndDrone()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    QDir directory(root.path());
    QVERIFY(directory.mkpath("branch/deep"));
    for (int index = 0; index < 25; ++index)
        QVERIFY(writeFile(directory.filePath(QString("file-%1.txt").arg(index, 2, 10, QChar('0')))));
    WorkspaceGraphWidget graph(root.path(), root.path(), nullptr, {5, 12});
    graph.resize(1200, 800);
    graph.show();
    QTRY_VERIFY(!graph.busy());
    QVERIFY(graph.nodeCount() <= 6);
    QVERIFY(graph.hasMore(root.path()));
    graph.expand(directory.filePath("branch"));
    QTRY_VERIFY(!graph.busy());
    QVERIFY(graph.hasPath(directory.filePath("branch/deep")));
    graph.collapse(directory.filePath("branch"));
    QVERIFY(!graph.hasPath(directory.filePath("branch/deep")));
    graph.loadMore(root.path());
    QTRY_VERIFY(!graph.busy());
    QVERIFY(graph.nodeCount() <= 12);
    graph.loadMore(root.path());
    QTRY_VERIFY(!graph.busy());
    QVERIFY(graph.nodeCount() <= 12);
    QVERIFY(!graph.hasPath(directory.filePath("branch/deep")));
    QGraphicsItem *rootItem = nullptr;
    for (auto *item : graph.canvas()->scene()->items()) {
        if (item->data(0).toString() == root.path())
            rootItem = item;
    }
    QVERIFY(rootItem);
    rootItem->setPos(QPointF(35, 57));
    const int loaded = graph.nodeCount();
    graph.setSpatial(true);
    QVERIFY(!graph.findChild<QAction *>("openGraphFlight"));
    QVERIFY(!graph.findChild<QAction *>("enableAppBillboards"));
    QCOMPARE(graph.nodeCount(), loaded);
    QVERIFY(!graph.busy() && !WorkspaceGraphReader::helperActive());
    QVERIFY(!(rootItem->flags() & QGraphicsItem::ItemIsMovable));
    const QPointF projected = rootItem->pos();
    graph.findChild<QPushButton *>("graphRotate")->click();
    QVERIFY(rootItem->pos() != projected);
    graph.selectPath(directory.filePath("branch"));
    QTest::qWait(400);
    QVERIFY(!graph.canvas()->viewport()->grab().isNull());
    const QString images = qEnvironmentVariable("DOCKPIPE_GRAPH_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(graph.screen()->grabWindow(graph.winId()).save(QDir(images).filePath("workspace-graph-3d-drone.png")));
    }
    qInfo() << "Graph renderer:" << static_cast<WorkspaceGraphView *>(graph.canvas())->rendererDescription();
    QVERIFY(!graph.busy());
    graph.setSpatial(false);
    QCOMPARE(rootItem->pos(), QPointF(35, 57));
    QVERIFY(rootItem->flags() & QGraphicsItem::ItemIsMovable);
    graph.navigate(directory.filePath("branch"));
    QTRY_VERIFY(!graph.busy());
    QVERIFY(graph.hasPath(directory.filePath("branch/deep")));
    graph.hide();
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
}

void ProjectFilesGraphTest::graphButtonFitsTextAndLargerFonts_data()
{
    QTest::addColumn<QString>("theme");
    QTest::newRow("dark") << QString("dark");
    QTest::newRow("light") << QString("light");
}

void ProjectFilesGraphTest::graphButtonFitsTextAndLargerFonts()
{
    QFETCH(QString, theme);
    QFile shared(":/theme/pipeon.qss");
    QFile companion(":/theme/pipeon-" + theme + ".qss");
    QVERIFY(shared.open(QIODevice::ReadOnly));
    QVERIFY(companion.open(QIODevice::ReadOnly));
    qApp->setStyleSheet(QString());
    qApp->setPalette(qApp->style()->standardPalette());
    qApp->setStyleSheet(QString::fromUtf8(shared.readAll() + companion.readAll()));
    QTemporaryDir root;
    ProjectFilesWidget files;
    files.resize(1500, 760);
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{true});
    files.show();
    QCoreApplication::processEvents();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    QVERIFY(button);
    QCOMPARE(button->text(), QString("Graph"));
    QVERIFY(button->width() >= button->sizeHint().width());
    const QSize normalSize = button->size();
    const QString images = qEnvironmentVariable("DOCKPIPE_GRAPH_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(files.grab().save(QDir(images).filePath("files-graph-" + theme + ".png")));
    }
    button->setStyleSheet("font-size: 26px;");
    QTRY_VERIFY(button->width() > normalSize.width());
    QCoreApplication::processEvents();
    const QWidget *controls = button->parentWidget();
    qInfo() << "Large-font Graph geometry:" << "button" << button->size()
        << "minimum" << button->minimumSize() << "maximum" << button->maximumSize()
        << "hint" << button->sizeHint() << "policy" << button->sizePolicy()
        << "controls" << controls->size() << "minimum" << controls->minimumSize()
        << "maximum" << controls->maximumSize() << "hint" << controls->sizeHint()
        << "layout constraint" << controls->layout()->sizeConstraint();
    const auto geometryDiagnostic = [&] {
        const auto *tree = files.findChild<QTreeView *>("fileTree");
        return QString("button=%1x%2 hint=%3x%4 minimumHint=%5x%6 textWidth=%7 iconWidth=%8 explorerWidth=%9")
            .arg(button->width()).arg(button->height())
            .arg(button->sizeHint().width()).arg(button->sizeHint().height())
            .arg(button->minimumSizeHint().width()).arg(button->minimumSizeHint().height())
            .arg(button->fontMetrics().horizontalAdvance(button->text())).arg(button->iconSize().width())
            .arg(tree ? tree->parentWidget()->width() : -1);
    };
    QTRY_VERIFY2(button->width() >= button->sizeHint().width(), qPrintable(geometryDiagnostic()));
    QTRY_VERIFY2(button->height() >= button->sizeHint().height(), qPrintable(geometryDiagnostic()));
    QTRY_VERIFY(button->width() >= button->fontMetrics().horizontalAdvance(button->text()) + button->iconSize().width());
    QTRY_VERIFY(button->parentWidget()->rect().contains(button->geometry()));
    QTRY_VERIFY(button->visibleRegion().boundingRect().contains(button->rect()));
    QTest::qWait(50);
    QVERIFY(button->visibleRegion().boundingRect().contains(button->rect()));
    if (!images.isEmpty())
        QVERIFY(files.grab().save(QDir(images).filePath("files-graph-" + theme + "-large-font.png")));
}

int main(int argc, char **argv)
{
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    QCoreApplication::setApplicationName("DockpipeGraphTest");
    applyDockpipeLauncherTheme(application);
    ProjectFilesGraphTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "ProjectFilesGraphTest.moc"
