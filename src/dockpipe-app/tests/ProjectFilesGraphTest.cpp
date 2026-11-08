#include "ProjectFilesWidget.h"
#include "WorkspaceFilesTestSupport.h"
#include "WorkspaceGraphWidget.h"
#include "WorkspaceGraphPage.h"
#include "WorkspaceGraphTimestamp.h"
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
#include <QStackedWidget>
#include <QStyle>
#include <QTemporaryDir>
#include <QTest>
#include <QTreeView>
#include <filesystem>

class ProjectFilesGraphTest final : public QObject {
    Q_OBJECT
private slots:
    void builtInGraphDoesNotLoadPackage();
    void embeddedGraphSurvivesRendererSwitches();
    void constellationGraph_data();
    void constellationGraph();
    void pendingLayoutDoesNotSurviveNavigationOrDestruction();
    void readerSecurityAndCursor();
    void timestampCountPreservesFullRange();
    void submillisecondTimestampChangeInvalidatesCursor();
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
    QVERIFY(FilesTest::setRoot(files, root.path()));
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
    QVERIFY(FilesTest::setRoot(files, other.path()));
    QTRY_VERIFY(!files.findChild<WorkspaceGraphWidget *>());
    files.openGraph();
    files.hide();
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
    QTRY_VERIFY(!files.findChild<WorkspaceGraphWidget *>());
}

void ProjectFilesGraphTest::embeddedGraphSurvivesRendererSwitches()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    QFile file(root.filePath("example.json"));
    QVERIFY(file.open(QIODevice::WriteOnly));
    file.write("{\"example\":true}\n");
    file.close();
    QStackedWidget host;
    ProjectFilesWidget files;
    host.addWidget(&files);
    host.addWidget(new QWidget);
    host.resize(1200, 800);
    QVERIFY(FilesTest::setRoot(files, root.path()));
    host.show();
    QVERIFY(QTest::qWaitForWindowExposed(&host));
    files.openGraph();
    QPointer<WorkspaceGraphWidget> graph = files.findChild<WorkspaceGraphWidget *>();
    QVERIFY(graph);
    QTRY_VERIFY(!graph->busy());
    graph->findChild<QPushButton *>("graph3D")->click();
    QTest::qWait(400);
    QVERIFY(graph);
    QVERIFY(graph->isVisible());
    QVERIFY(graph->spatial());
    auto *view = static_cast<WorkspaceGraphView *>(graph->canvas());
    view->setHardwareAcceleration(false);
    QTest::qWait(50);
    QVERIFY(graph && graph->spatial());
    QVERIFY(!view->hardwareAccelerated());
    view->setHardwareAcceleration(true);
    QTest::qWait(300);
    QVERIFY(graph && graph->isVisible());
    qInfo() << "Embedded graph renderer:" << view->rendererDescription();
    graph->findChild<QPushButton *>("graph2D")->click();
    QTest::qWait(100);
    QVERIFY(graph);
    QVERIFY(graph->isVisible());
    QVERIFY(!graph->spatial());
    for (int cycle = 0; cycle < 3; ++cycle) {
        graph->setSpatial(true);
        QTest::qWait(300);
        QVERIFY(graph && graph->isVisible());
        graph->setSpatial(false);
        QTest::qWait(50);
        QVERIFY(graph && graph->isVisible());
    }
    host.setCurrentIndex(1);
    QTRY_VERIFY(graph.isNull());
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
}

void ProjectFilesGraphTest::constellationGraph_data()
{
    QTest::addColumn<QString>("theme");
    QTest::newRow("dark") << QString("dark");
    QTest::newRow("light") << QString("light");
}

void ProjectFilesGraphTest::constellationGraph()
{
    QFETCH(QString, theme);
    previewDockpipeLauncherTheme(*qApp, theme == "light" ? LauncherThemeMode::Light : LauncherThemeMode::Dark);
    QTemporaryDir root;
    QVERIFY(root.isValid());
    QDir directory(root.path());
    const QStringList folders{"assets", "docs", "src", "tests"};
    const QStringList names{"overview.md", "settings.json", "index.ts", "app.cpp", "theme.css", "page.html", "build.yml", "layout.tsx"};
    for (const auto &folder : folders) {
        QVERIFY(directory.mkdir(folder));
        for (const auto &name : names)
            QVERIFY(writeFile(directory.filePath(folder + '/' + name)));
    }
    QVERIFY(writeFile(directory.filePath("README.md")));
    QVERIFY(writeFile(directory.filePath("package.json")));
    WorkspaceGraphWidget graph(root.path(), root.path());
    graph.resize(1180, 780);
    graph.show();
    QTRY_VERIFY(!graph.busy());
    for (const auto &folder : folders) {
        graph.expand(directory.filePath(folder));
        QTRY_VERIFY(!graph.busy());
    }
    auto *view = static_cast<WorkspaceGraphView *>(graph.canvas());
    QVERIFY(view->constellation());
    QVERIFY(!graph.spatial());
    QMap<QString, WorkspaceGraphNode *> nodes;
    bool hasDepth = false;
    for (auto *item : view->scene()->items()) {
        if (auto *node = dynamic_cast<WorkspaceGraphNode *>(item)) {
            nodes.insert(node->data(0).toString(), node);
            QVERIFY(node->property("workspaceGraphNode").toBool());
            hasDepth = hasDepth || std::abs(node->property("graphPosition").value<QVector3D>().y()) > 20;
        }
    }
    QCOMPARE(nodes.size(), 39);
    QVERIFY(hasDepth);
    auto *sample = nodes.value(directory.filePath("src/app.cpp"));
    QVERIFY(sample);
    const QVector3D world = sample->property("graphPosition").value<QVector3D>();
    const QPointF beforeOrbit = sample->pos();
    view->orbit(30, 10);
    QVERIFY(QLineF(beforeOrbit, sample->pos()).length() > 1);
    QCOMPARE(sample->property("graphPosition").value<QVector3D>(), world);
    view->scene()->clearSelection();
    view->fitInView(view->scene()->itemsBoundingRect().adjusted(-40, -40, 40, 40), Qt::KeepAspectRatio);
    const QString images = qEnvironmentVariable("DOCKPIPE_GRAPH_TEST_IMAGES");
    if (!images.isEmpty()) {
        QVERIFY(QDir().mkpath(images));
        QVERIFY(graph.grab().save(QDir(images).filePath("workspace-graph-constellation-" + theme + ".png")));
    }
    graph.selectPath(directory.filePath("src/app.cpp"));
    QVERIFY(sample->opacity() > nodes.value(directory.filePath("docs/overview.md"))->opacity());
    const QPointF arranged = sample->pos();
    graph.setSpatial(true);
    QVERIFY(!view->constellation());
    graph.setSpatial(false);
    QVERIFY(view->constellation());
    QVERIFY(QLineF(sample->pos(), arranged).length() < 0.01);
    graph.close();
    resetDockpipeLauncherThemePreview(*qApp);
}

void ProjectFilesGraphTest::pendingLayoutDoesNotSurviveNavigationOrDestruction()
{
    QTemporaryDir root;
    QTemporaryDir other;
    QVERIFY(root.isValid() && other.isValid());
    for (int index = 0; index < 100; ++index)
        QVERIFY(writeFile(QDir(root.path()).filePath(QString("file-%1.txt").arg(index))));
    QVERIFY(writeFile(other.filePath("replacement.txt")));
    QVERIFY(QDir(root.path()).mkdir("branch"));
    const QString branch = root.filePath("branch");
    QVERIFY(writeFile(QDir(branch).filePath("child.txt")));
    auto *graph = new WorkspaceGraphWidget(root.path(), root.path());
    graph->show();
    QTRY_VERIFY(!graph->busy());
    QAction *reset = nullptr;
    for (auto *action : graph->findChildren<QAction *>()) {
        if (action->text() == "Reset layout")
            reset = action;
    }
    QVERIFY(reset);
    // No event processing between requests: one active solve and the latest
    // replacement must remain pending even if the worker finishes immediately.
    reset->trigger();
    QVERIFY(graph->busy());
    reset->trigger();
    reset->trigger();
    graph->navigate(branch);
    QTRY_VERIFY(!graph->busy());
    QCOMPARE(graph->nodeCount(), 2);
    QVERIFY(graph->hasPath(QDir(branch).filePath("child.txt")));
    for (auto *item : graph->canvas()->scene()->items()) {
        if (auto *node = dynamic_cast<WorkspaceGraphNode *>(item))
            QVERIFY(node->property("graphPosition").isValid());
    }
    reset->trigger();
    QVERIFY(graph->busy());
    graph->setSpatial(true);
    graph->collapse(branch);
    QTRY_VERIFY(!graph->busy());
    QCOMPARE(graph->nodeCount(), 1);
    graph->setSpatial(false);
    reset->trigger();
    QVERIFY(graph->busy());
    // Destruction must disconnect completion without waiting for the solver.
    delete graph;
    QTRY_VERIFY(!WorkspaceGraphReader::helperActive());
    WorkspaceGraphWidget replacement(other.path(), other.path());
    replacement.show();
    QTRY_VERIFY(!replacement.busy());
    QVERIFY(replacement.hasPath(other.filePath("replacement.txt")));
    QCOMPARE(replacement.nodeCount(), 2);
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

void ProjectFilesGraphTest::timestampCountPreservesFullRange()
{
    using WorkspaceGraphTimestamp::decimalCount;
    QCOMPARE(decimalCount(qint64(0)), QString("0"));
    QCOMPARE(decimalCount(qint64(-1)), QString("-1"));
    QCOMPARE(decimalCount(std::numeric_limits<qint64>::max()), QString("9223372036854775807"));
    QCOMPARE(decimalCount(std::numeric_limits<qint64>::min()), QString("-9223372036854775808"));
#if defined(__SIZEOF_INT128__)
    using WideCount = __int128;
    const WideCount beyond64 = static_cast<WideCount>(std::numeric_limits<quint64>::max()) + 1;
    QCOMPARE(decimalCount(beyond64), QString("18446744073709551616"));
    QCOMPARE(decimalCount(-beyond64), QString("-18446744073709551616"));
    QCOMPARE(decimalCount(std::numeric_limits<WideCount>::max()),
        QString("170141183460469231731687303715884105727"));
    QCOMPARE(decimalCount(std::numeric_limits<WideCount>::min()),
        QString("-170141183460469231731687303715884105728"));
#endif
}

void ProjectFilesGraphTest::submillisecondTimestampChangeInvalidatesCursor()
{
    QTemporaryDir root;
    QVERIFY(root.isValid());
    QVERIFY(writeFile(QDir(root.path()).filePath("first.txt")));
    QVERIFY(writeFile(QDir(root.path()).filePath("second.txt")));
    const auto directory = std::filesystem::u8path(root.path().toUtf8().constData());
    using Clock = std::filesystem::file_time_type;
    using Duration = Clock::duration;
    const auto original = std::filesystem::last_write_time(directory);
    const auto milliseconds = std::chrono::duration_cast<std::chrono::milliseconds>(original.time_since_epoch());
    const auto aligned = Clock(std::chrono::duration_cast<Duration>(milliseconds))
        + std::chrono::duration_cast<Duration>(std::chrono::microseconds(100));
    std::filesystem::last_write_time(directory, aligned);
    const auto before = std::filesystem::last_write_time(directory);
    const QJsonObject request{{"root", root.path()}, {"directory", root.path()}, {"limit", 1}};
    const auto first = WorkspaceGraphPage::read(request);
    QVERIFY(!first.contains("error"));
    QVERIFY(first.value("more").toBool());
    QJsonObject next = request;
    next.insert("after", first.value("after"));
    next.insert("stamp", first.value("stamp"));
    QVERIFY(!WorkspaceGraphPage::read(next).contains("error"));
    std::filesystem::last_write_time(directory,
        before + std::chrono::duration_cast<Duration>(std::chrono::microseconds(1)));
    const auto after = std::filesystem::last_write_time(directory);
    QVERIFY2(after != before, "The temporary filesystem must retain a one-microsecond timestamp change.");
    QCOMPARE(std::chrono::duration_cast<std::chrono::milliseconds>(before.time_since_epoch()).count(),
        std::chrono::duration_cast<std::chrono::milliseconds>(after.time_since_epoch()).count());
    QVERIFY(WorkspaceGraphPage::read(next).contains("error"));
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
    QVERIFY(QLineF(rootItem->pos(), QPointF(35, 57)).length() < 0.01);
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
    QVERIFY(FilesTest::setRoot(files, root.path()));
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
