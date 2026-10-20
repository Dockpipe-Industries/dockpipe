#include "ProjectFilesWidget.h"
#include "Theme.h"
#include "WorkspacePackageManager.h"

#include <QApplication>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QLabel>
#include <QLayout>
#include <QPointer>
#include <QProcess>
#include <QPushButton>
#include <QSignalSpy>
#include <QStyle>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <QTreeView>
#include <cstdio>

class ProjectFilesGraphTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void firstClickLoadsInstalledPackageAndCloseRestoresTree();
    void pendingOpenIsCancelled_data();
    void pendingOpenIsCancelled();
    void missingPackageShowsAccurateError();
    void disabledGraphExplainsSettingsRequirement();
    void graphButtonFitsTextAndLargerFonts_data();
    void graphButtonFitsTextAndLargerFonts();

private:
    bool runIsolated(const QString &selector);
    QString m_childOutput;
};

void ProjectFilesGraphTest::initTestCase()
{
    QVERIFY2(QFileInfo(qEnvironmentVariable("DOCKPIPE_WORKSPACE_PACKAGE")).isDir(),
        "The test requires the workspace-package-fixture assets directory.");
}

bool ProjectFilesGraphTest::runIsolated(const QString &selector)
{
    QTemporaryDir home;
    if (!home.isValid())
        return false;
    QProcess child;
    auto environment = QProcessEnvironment::systemEnvironment();
    environment.insert("DOCKPIPE_GRAPH_TEST_CHILD", "1");
    environment.insert("DOCKPIPE_GRAPH_TEST_HOME", home.path());
    child.setProcessEnvironment(environment);
    child.setProcessChannelMode(QProcess::MergedChannels);
    child.start(QCoreApplication::applicationFilePath(), {selector});
    if (!child.waitForStarted(3000) || !child.waitForFinished(15000)) {
        child.kill();
        child.waitForFinished(1000);
        m_childOutput = QString::fromUtf8(child.readAll());
        return false;
    }
    m_childOutput = QString::fromUtf8(child.readAll());
    return child.exitStatus() == QProcess::NormalExit && child.exitCode() == 0;
}

void ProjectFilesGraphTest::firstClickLoadsInstalledPackageAndCloseRestoresTree()
{
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_CHILD")) {
        QVERIFY2(runIsolated(QTest::currentTestFunction()), qPrintable(m_childOutput));
        return;
    }
    QTemporaryDir root;
    QVERIFY(root.isValid());
    ProjectFilesWidget files;
    files.resize(1200, 760);
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{true});
    files.show();
    QCoreApplication::processEvents();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    auto *tree = files.findChild<QTreeView *>("fileTree");
    QVERIFY(button && tree);
    QVERIFY(button->isVisible() && button->isEnabled());
    QVERIFY(tree->isVisible());
    QVERIFY(!WorkspacePackageManager::instance().ready());
    QTest::mouseClick(button, Qt::LeftButton);
    QTRY_VERIFY(WorkspacePackageManager::instance().ready());
    QPointer<QWidget> surface = files.findChild<QWidget *>("fixtureFileSurface");
    QVERIFY(surface);
    QVERIFY(surface->isVisible());
    QCOMPARE(surface->property("fixtureRoot").toString(), root.path());
    QCOMPARE(surface->property("fixtureDirectory").toString(), QFileInfo(root.path()).canonicalFilePath());
    QVERIFY(surface->property("fixtureCockpitEnabled").toBool());
    QVERIFY(!tree->isVisible());
    auto *close = surface->findChild<QPushButton *>("fixtureCloseFiles");
    QVERIFY(close);
    QTest::mouseClick(close, Qt::LeftButton);
    QTRY_VERIFY(surface.isNull());
    QVERIFY(tree->isVisible());
    QVERIFY(button->isEnabled());
}

void ProjectFilesGraphTest::pendingOpenIsCancelled_data()
{
    QTest::addColumn<QString>("change");
    QTest::newRow("hide") << QString("hide");
    QTest::newRow("root") << QString("root");
    QTest::newRow("disable") << QString("disable");
}

void ProjectFilesGraphTest::pendingOpenIsCancelled()
{
    QFETCH(QString, change);
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_CHILD")) {
        const QString selector = QString(QTest::currentTestFunction()) + ':' + QTest::currentDataTag();
        QVERIFY2(runIsolated(selector), qPrintable(m_childOutput));
        return;
    }
    QTemporaryDir root;
    QTemporaryDir otherRoot;
    QVERIFY(root.isValid() && otherRoot.isValid());
    qputenv("DOCKPIPE_GRAPH_TEST_ASSETS", qgetenv("DOCKPIPE_WORKSPACE_PACKAGE"));
    qunsetenv("DOCKPIPE_WORKSPACE_PACKAGE");
    qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
    ProjectFilesWidget files;
    files.resize(1200, 760);
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{true});
    files.show();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    QVERIFY(button);
    QSignalSpy prepared(&WorkspacePackageManager::instance(), &WorkspacePackageManager::prepared);
    QTest::mouseClick(button, Qt::LeftButton);
    QVERIFY(WorkspacePackageManager::instance().busy());
    if (change == "hide") {
        files.hide();
    } else if (change == "root") {
        QVERIFY(files.setRoot(otherRoot.path()));
    } else {
        files.applyCockpitSettings(CockpitSettings{false});
    }
    QTRY_COMPARE(prepared.size(), 1);
    QVERIFY(!files.findChild<QWidget *>("fixtureFileSurface"));
    files.show();
    QCoreApplication::processEvents();
    QVERIFY(!files.findChild<QWidget *>("fixtureFileSurface"));
    QVERIFY(files.findChild<QTreeView *>("fileTree")->isVisible());
}

void ProjectFilesGraphTest::missingPackageShowsAccurateError()
{
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_CHILD")) {
        QVERIFY2(runIsolated(QTest::currentTestFunction()), qPrintable(m_childOutput));
        return;
    }
    QTemporaryDir root;
    QVERIFY(root.isValid());
    qputenv("DOCKPIPE_WORKSPACE_PACKAGE", root.path().toUtf8());
    ProjectFilesWidget files;
    files.resize(1200, 760);
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{true});
    files.show();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    QVERIFY(button);
    QTest::mouseClick(button, Qt::LeftButton);
    QTRY_VERIFY(!WorkspacePackageManager::instance().busy());
    QVERIFY(!WorkspacePackageManager::instance().ready());
    QVERIFY(!files.findChild<QWidget *>("fixtureFileSurface"));
    bool accurateVisibleError = false;
    for (QLabel *label : files.findChildren<QLabel *>()) {
        accurateVisibleError |= label->isVisible()
            && label->text().contains("missing", Qt::CaseInsensitive)
            && label->text().contains("native", Qt::CaseInsensitive);
    }
    QVERIFY(accurateVisibleError);
    QVERIFY(button->isEnabled());
}

void ProjectFilesGraphTest::disabledGraphExplainsSettingsRequirement()
{
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_CHILD")) {
        QVERIFY2(runIsolated(QTest::currentTestFunction()), qPrintable(m_childOutput));
        return;
    }
    QTemporaryDir root;
    ProjectFilesWidget files;
    QVERIFY(files.setRoot(root.path()));
    files.applyCockpitSettings(CockpitSettings{false});
    files.show();
    auto *button = files.findChild<QPushButton *>("openWorkspaceGraph");
    QVERIFY(button);
    QVERIFY(button->isVisible());
    QVERIFY(!button->isEnabled());
    QVERIFY(button->toolTip().contains("Settings", Qt::CaseInsensitive));
    QVERIFY(button->toolTip().contains("Cockpit", Qt::CaseInsensitive));
    QVERIFY(!WorkspacePackageManager::instance().busy());
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
    if (qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_CHILD")) {
        const QString selector = QString(QTest::currentTestFunction()) + ':' + QTest::currentDataTag();
        QVERIFY2(runIsolated(selector), qPrintable(m_childOutput));
        return;
    }
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
    // The async package resolver fixture never initializes widgets or launches
    // a real CLI. A delayed response makes cancellation deterministic.
    if (argc > 1 && QByteArray(argv[1]) == "scope" && !qEnvironmentVariableIsEmpty("DOCKPIPE_GRAPH_TEST_ASSETS")) {
        QThread::msleep(250);
        const QByteArray assets = qgetenv("DOCKPIPE_GRAPH_TEST_ASSETS") + '\n';
        std::fwrite(assets.constData(), 1, static_cast<size_t>(assets.size()), stdout);
        return 0;
    }
    const QByteArray home = qgetenv("DOCKPIPE_GRAPH_TEST_HOME");
    if (!home.isEmpty()) {
        qputenv("HOME", home);
        qputenv("USERPROFILE", home);
        qputenv("APPDATA", home);
        qputenv("LOCALAPPDATA", home);
        qputenv("XDG_CONFIG_HOME", home);
    }
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    QApplication application(argc, argv);
    QCoreApplication::setApplicationName("DockpipeGraphTest");
    applyDockpipeLauncherTheme(application);
    ProjectFilesGraphTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "ProjectFilesGraphTest.moc"
