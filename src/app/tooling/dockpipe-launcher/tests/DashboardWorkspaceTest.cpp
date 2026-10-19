#include "DashboardWorkspace.h"
#include "CanvasPanel.h"

#include <QApplication>
#include <QAbstractButton>
#include <QFile>
#include <QElapsedTimer>
#include <QDragEnterEvent>
#include <QDropEvent>
#include <QMimeData>
#include <QImage>
#include <QMenu>
#include <QPushButton>
#include <QTabBar>
#include <QTemporaryDir>
#include <QVBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QPointer>
#include <QSignalSpy>
#include <QScrollArea>
#include <QShortcut>
#include <QSplitter>
#include <QTabWidget>
#include <QTest>
#include <QTimer>
#include <QToolButton>

namespace {
int panelTabIndex(QTabWidget *group, CanvasPanel *frame)
{
    const QString id = frame->objectName().mid(QString("dashboardPanel_").size());
    for (int index = 0; index < group->count(); ++index) {
        if (group->widget(index)->property("dashboardPanelId").toString() == id)
            return index;
    }
    return -1;
}

bool selectedFrameIsVisible(QTabWidget *group, CanvasPanel *frame)
{
    const int index = panelTabIndex(group, frame);
    if (index < 0 || group->currentIndex() != index)
        return false;
    QWidget *slot = group->currentWidget();
    return slot->isAncestorOf(frame) && frame->isVisibleTo(group)
        && slot->rect().contains(QRect(frame->mapTo(slot, QPoint()), frame->size()));
}
}

class DashboardWorkspaceTest : public QObject {
    Q_OBJECT
private slots:
    void keepsContentOwnedContainersUnchanged();
    void retainsTilingPresentationAcrossCanvasChanges();
    void deferredConstraintsFollowHiddenFrameChanges();
    void deferredTabsRetainMetadataAndAttachVisibleContent();
    void deferredSelectionSurvivesModeChangesAndRemoval();
    void preservesLiveWidgetsAndBothArrangements();
    void replacingTileViewportPreservesMinimizedAndFloatingPanels();
    void restoresCanvasAfterStartupSizing();
    void clickingContentSavesStackingOrder();
    void validatesBeforeChangingLayout();
    void onlySelectedTabIsVisibleAfterRebuild();
    void boundsPanelCountAndKeepsCloseExplicit();
    void placesNewPanelsInFreeSpaceAndHonorsContentSize();
    void tiledChromeUsesOneHeaderAndRetainsActions();
    void tightTilesFillPanesAndMinimizeToDock();
    void canvasBackgroundCachesDecodedImage();
    void droppingOnTabStripJoinsGroup();
    void expandsCanvasWithoutChangingSavedLayout();
    void expandsTilesWithoutReparentingLiveContent();
    void expansionRestoresBeforeRemovalAndLayoutChanges();
    void restoringExpansionAfterShrinkKeepsPanelsReachable();
    void externalTilesFillPaneAndRetainMenuActions();
    void externalTileActionsFollowVisibilityAndTabGrouping();
};

void DashboardWorkspaceTest::keepsContentOwnedContainersUnchanged()
{
    DashboardWorkspace workspace("container-ownership");
    workspace.resize(1300, 850);
    workspace.show();
    auto *content = new QWidget;
    content->setFixedSize(460, 260);
    auto *layout = new QVBoxLayout(content);
    auto *innerTabs = new QTabWidget(content);
    innerTabs->setObjectName("dashboardTabGroup");
    innerTabs->setMovable(true);
    auto *innerSplit = new QSplitter(Qt::Horizontal);
    innerSplit->setObjectName("dashboardSplit");
    innerSplit->setHandleWidth(13);
    innerSplit->addWidget(new QLabel("owned left"));
    innerSplit->addWidget(new QLabel("owned right"));
    innerTabs->addTab(innerSplit, "Content");
    innerTabs->addTab(new QLabel("intentionally unavailable"), "Hidden");
    innerTabs->setTabVisible(1, false);
    innerTabs->tabBar()->hide();
    layout->addWidget(innerTabs);
    auto *innerTimer = new QTimer(content);
    innerTimer->setObjectName("dashboardPanelAttachTimer");
    innerTimer->start(100000);
    QVERIFY(workspace.addPanel("one", "One", content));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("other panel")));
    QCoreApplication::processEvents();
    innerSplit->setSizes({120, 300});
    const QList<int> originalSizes = innerSplit->sizes();
    const auto verifyContent = [&]() {
        return innerTabs->tabBar()->isHidden() && !innerTabs->isTabVisible(1)
            && innerTabs->isMovable() && innerTabs->currentWidget() == innerSplit
            && innerSplit->handleWidth() == 13 && innerSplit->sizes() == originalSizes
            && !innerSplit->property("dashboardUnminimizedSizes").isValid()
            && innerTimer->isActive();
    };
    QVERIFY(verifyContent());
    for (int cycle = 0; cycle < 3; ++cycle) {
        workspace.setMode("tiling");
        QCoreApplication::processEvents();
        QVERIFY(verifyContent());
        QVERIFY(workspace.minimizePanel("two"));
        QVERIFY(verifyContent());
        QVERIFY(workspace.activatePanel("two"));
        QVERIFY(verifyContent());
        QVERIFY(workspace.togglePanelExpanded("one"));
        QVERIFY(verifyContent());
        QVERIFY(workspace.togglePanelExpanded("one"));
        QVERIFY(verifyContent());
        workspace.setMode("canvas");
        QCoreApplication::processEvents();
        QVERIFY(verifyContent());
    }
    const auto saved = workspace.saveLayout();
    QVERIFY(workspace.restoreLayout(saved));
    QVERIFY(verifyContent());
}

void DashboardWorkspaceTest::retainsTilingPresentationAcrossCanvasChanges()
{
    DashboardWorkspace workspace("retained-presentation");
    workspace.resize(1300, 800);
    workspace.show();
    QPointer<QLineEdit> first = new QLineEdit("first draft");
    QPointer<QLineEdit> second = new QLineEdit("second draft");
    QPointer<QLineEdit> third = new QLineEdit("third draft");
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", second));
    QVERIFY(workspace.addPanel("three", "Three", third));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    QCoreApplication::processEvents();
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    auto *firstFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    auto *secondFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_two");
    auto *thirdFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_three");
    QPointer<QScrollArea> viewport = workspace.findChild<QScrollArea *>("dashboardTileViewport");
    QPointer<QTabWidget> group;
    for (auto *candidate : workspace.findChildren<QTabWidget *>()) {
        if (panelTabIndex(candidate, firstFrame) >= 0)
            group = candidate;
    }
    QVERIFY(group && viewport);
    QPointer<QWidget> page = group->currentWidget();
    QPointer<QWidget> controls = group->tabBar()->tabButton(group->currentIndex(), QTabBar::LeftSide);
    const QJsonObject savedTree = workspace.saveLayout().value("tree").toObject();
    const int objectCount = workspace.findChildren<QObject *>().size();
    for (int cycle = 0; cycle < 5; ++cycle) {
        workspace.setMode("canvas");
        QVERIFY(viewport && group && page && controls);
        QVERIFY(!viewport->isVisible());
        QCOMPARE(firstFrame->parentWidget(), &workspace);
        QCOMPARE(secondFrame->parentWidget(), &workspace);
        QCOMPARE(workspace.saveLayout().value("tree").toObject(), savedTree);
        QCoreApplication::processEvents();
        QVERIFY(first->isVisible() && second->isVisible() && third->isVisible());
        workspace.setMode("tiling");
        QCOMPARE(workspace.findChild<QScrollArea *>("dashboardTileViewport"), viewport.data());
        QCOMPARE(group->currentWidget(), page.data());
        QCOMPARE(group->tabBar()->tabButton(group->currentIndex(), QTabBar::LeftSide), controls.data());
        QVERIFY(selectedFrameIsVisible(group, firstFrame));
        QVERIFY(!second->isVisible());
        QCOMPARE(workspace.findChildren<QObject *>().size(), objectCount);
    }
    workspace.setMode("canvas");
    workspace.setPanelTitle("two", "Changed while canvas");
    QPixmap pixels(12, 12);
    pixels.fill(Qt::cyan);
    QIcon icon(pixels);
    workspace.setPanelIcon("two", icon);
    const int secondIndex = panelTabIndex(group, secondFrame);
    QCOMPARE(group->tabText(secondIndex), QString("Changed while canvas"));
    QCOMPARE(group->tabIcon(secondIndex).cacheKey(), icon.cacheKey());
    workspace.setCompactTiling(false);
    second->setMinimumSize(400, 200);
    workspace.resize(1450, 900);
    workspace.setMode("tiling");
    QVERIFY(selectedFrameIsVisible(group, firstFrame));
    QVERIFY(workspace.activatePanel("two"));
    QVERIFY(selectedFrameIsVisible(group, secondFrame));
    QVERIFY(second->width() >= 400 && second->height() >= 200);
    for (auto *split : workspace.findChildren<QSplitter *>())
        QCOMPARE(split->handleWidth(), 7);
    const auto beforeMinimize = workspace.saveLayout().value("tree");
    QVERIFY(workspace.minimizePanel("three"));
    workspace.setMode("canvas");
    QVERIFY(workspace.activatePanel("three"));
    workspace.setMode("tiling");
    QCoreApplication::processEvents();
    QVERIFY(third->isVisible());
    QVERIFY(thirdFrame->width() >= thirdFrame->minimumWidth());
    QCOMPARE(workspace.saveLayout().value("tree"), beforeMinimize);
    const QJsonObject layout = workspace.saveLayout();
    workspace.setMode("canvas");
    QVERIFY(workspace.restoreLayout(layout));
    QVERIFY(!viewport && !group && !page && !controls);
    QVERIFY(first && second && third);
    QCOMPARE(first->text(), QString("first draft"));
    QCOMPARE(second->text(), QString("second draft"));
    QCOMPARE(third->text(), QString("third draft"));
}

void DashboardWorkspaceTest::deferredConstraintsFollowHiddenFrameChanges()
{
    DashboardWorkspace workspace("constraints");
    workspace.resize(1200, 800);
    workspace.show();
    auto *content = new QLabel("Wrapped content tests dynamic height for width.");
    QVERIFY(workspace.addPanel("one", "One", new QLabel("active")));
    QVERIFY(workspace.addPanel("two", "Two", content));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    workspace.setMode("canvas");
    workspace.setMode("tiling");
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_two");
    auto *group = workspace.findChild<QTabWidget *>();
    QVERIFY(frame);
    QVERIFY(group);
    auto *page = group->widget(group->tabText(0) == "Two" ? 0 : 1);
    QCoreApplication::processEvents();
    const QSize unchangedGeometry = frame->size();
    const QSize originalGroupMinimum = group->minimumSizeHint();
    frame->setMinimumSize(320, 160);
    QCoreApplication::processEvents();
    QCOMPARE(frame->size(), unchangedGeometry);
    QCOMPARE(group->minimumSizeHint().width(), originalGroupMinimum.width() + 40);
    QCOMPARE(page->minimumSize(), frame->minimumSize());
    frame->setMinimumSize(180, 80);
    QCoreApplication::processEvents();
    QCOMPARE(frame->size(), unchangedGeometry);
    QCOMPARE(group->minimumSizeHint(), originalGroupMinimum);
    QCOMPARE(page->minimumSize(), frame->minimumSize());

    frame->setMaximumSize(frame->width() + 100, frame->height() + 100);
    QCoreApplication::processEvents();
    group->sizeHint();
    QCOMPARE(frame->size(), unchangedGeometry);
    QCOMPARE(page->maximumSize(), frame->maximumSize());
    frame->setMaximumSize(frame->width() + 200, frame->height() + 200);
    QCoreApplication::processEvents();
    group->minimumSizeHint();
    QCOMPARE(frame->size(), unchangedGeometry);
    QCOMPARE(page->maximumSize(), frame->maximumSize());

    frame->setSizePolicy(QSizePolicy::Expanding, QSizePolicy::Preferred);
    group->sizeHint();
    QCOMPARE(page->sizePolicy(), frame->sizePolicy());
    QVERIFY(!frame->hasHeightForWidth());
    content->setWordWrap(true);
    QCoreApplication::processEvents();
    QVERIFY(frame->hasHeightForWidth());
    QVERIFY(page->hasHeightForWidth());
    QCOMPARE(page->heightForWidth(200), frame->heightForWidth(200));
    QVERIFY(workspace.activatePanel("two"));
    QVERIFY(content->isVisible());
    QVERIFY(selectedFrameIsVisible(group, frame));
    QVERIFY(frame->width() <= frame->maximumWidth());
    QVERIFY(frame->height() <= frame->maximumHeight());
    QVERIFY(frame->width() >= frame->minimumWidth());
    QVERIFY(frame->height() >= frame->minimumHeight());
}

void DashboardWorkspaceTest::deferredTabsRetainMetadataAndAttachVisibleContent()
{
    DashboardWorkspace workspace("deferred-metadata");
    workspace.resize(1000, 700);
    workspace.show();
    auto *first = new QLineEdit("first retained value");
    auto *second = new QLineEdit("second retained value");
    auto *third = new QLabel("Wrapped content retains its height-for-width layout contract.");
    third->setWordWrap(true);
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", second));
    QVERIFY(workspace.addPanel("three", "Three", third));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.dockPanel("three", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    workspace.setMode("canvas");
    workspace.setMode("tiling");
    auto *group = workspace.findChild<QTabWidget *>();
    QVERIFY(group);
    QCOMPARE(group->count(), 3);
    const auto tabIndex = [group](const QString &title) {
        for (int index = 0; index < group->count(); ++index) {
            if (group->tabText(index) == title)
                return index;
        }
        return -1;
    };
    auto *secondFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_two");
    auto *thirdFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_three");
    QVERIFY(secondFrame);
    QVERIFY(thirdFrame);
    QVERIFY(!group->widget(tabIndex("Two"))->isAncestorOf(second));
    QWidget *thirdPage = group->widget(tabIndex("Three"));
    QCOMPARE(thirdPage->hasHeightForWidth(), thirdFrame->hasHeightForWidth());
    QCOMPARE(thirdPage->heightForWidth(250), thirdFrame->heightForWidth(250));
    third->setMinimumSize(420, 140);
    QCoreApplication::processEvents();
    QVERIFY(thirdPage->minimumSizeHint().width() >= 420);
    QVERIFY(thirdPage->minimumSizeHint().height() >= 140);

    workspace.setPanelTitle("two", "Renamed two");
    QPixmap iconPixels(12, 12);
    iconPixels.fill(Qt::green);
    const QIcon icon(iconPixels);
    workspace.setPanelIcon("two", icon);
    int index = tabIndex("Renamed two");
    QVERIFY(index >= 0);
    QCOMPARE(group->tabIcon(index).cacheKey(), icon.cacheKey());
    group->tabBar()->setTabData(index, "retained metadata");
    group->setTabToolTip(index, "retained tooltip");
    group->tabBar()->moveTab(index, 1);
    index = tabIndex("Renamed two");
    QCOMPARE(index, 1);
    QElapsedTimer elapsed;
    elapsed.start();
    group->tabBar()->setCurrentIndex(index);
    QTRY_VERIFY_WITH_TIMEOUT(second->isVisible(), 100);
    QVERIFY2(elapsed.elapsed() < 100, "First deferred selection exceeded the response deadline");
    QVERIFY(selectedFrameIsVisible(group, secondFrame));
    QCOMPARE(group->tabBar()->tabData(index).toString(), QString("retained metadata"));
    QCOMPARE(group->tabToolTip(index), QString("retained tooltip"));
    QCOMPARE(second->text(), QString("second retained value"));
    QVERIFY(!first->isVisible());
    QVERIFY(!third->isVisible());

    QVERIFY(workspace.minimizePanel("one"));
    QVERIFY(workspace.minimizePanel("two"));
    QVERIFY(workspace.minimizePanel("three"));
    QVERIFY(workspace.activatePanel("three"));
    QVERIFY(third->isVisible());
    QVERIFY(selectedFrameIsVisible(group, thirdFrame));
    QVERIFY(workspace.togglePanelExpanded("three"));
    QCOMPARE(workspace.expandedPanel(), QString("three"));
    QVERIFY(third->isVisible());
    QVERIFY(workspace.togglePanelExpanded("three"));
}

void DashboardWorkspaceTest::deferredSelectionSurvivesModeChangesAndRemoval()
{
    DashboardWorkspace workspace("deferred-lifetime");
    workspace.resize(1000, 700);
    workspace.show();
    QPointer<QLineEdit> first = new QLineEdit("first retained value");
    QPointer<QLineEdit> second = new QLineEdit("second retained value");
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", second));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    workspace.setMode("canvas");
    workspace.setMode("tiling");
    QPointer<QTabWidget> oldGroup = workspace.findChild<QTabWidget *>();
    QVERIFY(oldGroup);
    const int secondIndex = oldGroup->tabText(0) == "Two" ? 0 : 1;
    oldGroup->tabBar()->setCurrentIndex(secondIndex);
    workspace.setMode("canvas");
    QVERIFY(oldGroup);
    QVERIFY(!oldGroup->isVisible());
    QCoreApplication::processEvents();
    QVERIFY(first);
    QVERIFY(second);
    QVERIFY(first->isVisible());
    QVERIFY(second->isVisible());
    QCOMPARE(second->text(), QString("second retained value"));

    workspace.setMode("tiling");
    QVERIFY(workspace.activatePanel("one"));
    workspace.setMode("canvas");
    workspace.setMode("tiling");
    auto *group = workspace.findChild<QTabWidget *>();
    QVERIFY(group);
    group->tabBar()->setCurrentIndex(group->tabText(0) == "Two" ? 0 : 1);
    QSignalSpy destroyed(second, &QObject::destroyed);
    workspace.removePanel("two");
    QCoreApplication::processEvents();
    QVERIFY(!oldGroup);
    QVERIFY(!second);
    QCOMPARE(destroyed.count(), 1);
    QVERIFY(first);
    QVERIFY(workspace.activatePanel("one"));
    QVERIFY(first->isVisible());
    QCOMPARE(first->text(), QString("first retained value"));
}

void DashboardWorkspaceTest::externalTilesFillPaneAndRetainMenuActions()
{
    DashboardWorkspace workspace("external-tiles");
    workspace.resize(1100, 700);
    auto *content = new QWidget;
    content->setProperty("dashboardExternalApp", true);
    QVERIFY(workspace.addPanel("app", "External app", content));
    QVERIFY(workspace.addPanel("other", "Other", new QLabel("Other content")));
    workspace.setMode("tiling");
    workspace.show();
    QCoreApplication::processEvents();
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_app");
    QTabWidget *group = nullptr;
    for (auto *candidate : workspace.findChildren<QTabWidget *>()) {
        if (panelTabIndex(candidate, frame) >= 0)
            group = candidate;
    }
    QVERIFY(group);
    QVERIFY(group->tabBar()->isHidden());
    QVERIFY(frame->findChild<QWidget *>("dashboardPanelHeader")->isHidden());
    QCOMPARE(content->size(), group->size());
    QCOMPARE(content->mapTo(group, QPoint()), QPoint());
    const QJsonObject saved = workspace.saveLayout();
    QWidget *originalParent = content->parentWidget();
    auto *actions = frame->findChild<QToolButton *>("dashboardTileActions");
    QVERIFY(actions);
    QVERIFY(actions->testAttribute(Qt::WA_NativeWindow));
    QSignalSpy released(&workspace, &DashboardWorkspace::panelReleaseRequested);
    QSignalSpy closed(&workspace, &DashboardWorkspace::panelCloseRequested);

    const auto openMenu = [&]() {
        QMetaObject::invokeMethod(actions, "customContextMenuRequested", Qt::DirectConnection,
                                  Q_ARG(QPoint, QPoint(5, 5)));
        return qobject_cast<QMenu *>(QApplication::activePopupWidget());
    };
    const auto findAction = [](QMenu *menu, const QString &text) -> QAction * {
        for (QAction *action : menu->actions()) {
            if (action->text() == text)
                return action;
        }
        return nullptr;
    };
    QMenu *menu = openMenu();
    QVERIFY(menu);
    QVERIFY(findAction(menu, "Move beside Other"));
    QVERIFY(!findAction(menu, "Remove widget"));
    auto *expand = findAction(menu, "Expand panel");
    QVERIFY(expand);
    menu->close();
    expand->trigger();
    QCoreApplication::processEvents();
    QCOMPARE(workspace.expandedPanel(), QString("app"));
    QCOMPARE(content->size(), workspace.size());
    QCOMPARE(content->parentWidget(), originalParent);
    QCOMPARE(workspace.saveLayout(), saved);

    menu = openMenu();
    QVERIFY(menu);
    auto *restore = findAction(menu, "Restore panel");
    QVERIFY(restore);
    menu->close();
    restore->trigger();
    QCOMPARE(workspace.expandedPanel(), QString());
    menu = openMenu();
    auto *release = findAction(menu, "Release to desktop");
    QVERIFY(release);
    menu->close();
    release->trigger();
    QCOMPARE(released.count(), 1);
    QCOMPARE(released.first().first().toString(), QString("app"));
    QCOMPARE(closed.count(), 0);
    menu = openMenu();
    auto *minimize = findAction(menu, "Minimize to dock");
    QVERIFY(minimize);
    menu->close();
    minimize->trigger();
    QVERIFY(!content->isVisible());
    QCOMPARE(content->parentWidget(), originalParent);
    QVERIFY(workspace.activatePanel("app"));
    QVERIFY(content->isVisible());
    workspace.setCompactTiling(false);
    QCOMPARE(frame->layout()->contentsMargins(), QMargins());
}

void DashboardWorkspaceTest::externalTileActionsFollowVisibilityAndTabGrouping()
{
    DashboardWorkspace workspace("external-actions");
    workspace.resize(1100, 700);
    auto *content = new QWidget;
    content->setProperty("dashboardExternalApp", true);
    QVERIFY(workspace.addPanel("app", "App", content));
    QVERIFY(workspace.addPanel("other", "Other", new QLabel("Other")));
    workspace.setMode("tiling");
    workspace.show();
    QCoreApplication::processEvents();
    auto *timer = workspace.findChild<QTimer *>("dashboardTileActionsTimer");
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_app");
    auto *actions = frame->findChild<QToolButton *>("dashboardTileActions");
    QVERIFY(timer->isActive());
    auto *shortcut = workspace.findChild<QShortcut *>("dashboardTileMenuShortcut");
    QVERIFY(shortcut);
    QVERIFY(QMetaObject::invokeMethod(shortcut, "activated", Qt::DirectConnection));
    auto *menu = qobject_cast<QMenu *>(QApplication::activePopupWidget());
    QVERIFY(menu);
    QCOMPARE(menu->property("dashboardPanelId").toString(), QString("app"));
    menu->close();
    QTest::mouseMove(content, QPoint(content->width() / 2, 4));
    QTRY_VERIFY(actions->isVisible());
    QTest::mouseMove(content, QPoint(20, content->height() / 2));
    QTRY_VERIFY(actions->isHidden());
    workspace.hide();
    QVERIFY(!timer->isActive());
    workspace.show();
    QVERIFY(timer->isActive());
    QVERIFY(workspace.dockPanel("other", "app", "tab"));
    auto *group = workspace.findChild<QTabWidget *>();
    QVERIFY(group->tabBar()->isVisible());
    QVERIFY(actions->isHidden());
    workspace.setMode("canvas");
    QVERIFY(!timer->isActive());
    QVERIFY(actions->isHidden());
    QVERIFY(frame->findChild<QWidget *>("dashboardPanelHeader")->isVisible());
}

void DashboardWorkspaceTest::restoresCanvasAfterStartupSizing()
{
    DashboardWorkspace original("saved");
    original.resize(1400, 850);
    original.show();
    QVERIFY(original.addPanel("one", "One", new QLabel("one")));
    QVERIFY(original.addPanel("two", "Two", new QLabel("two")));
    original.findChild<CanvasPanel *>("dashboardPanel_one")->setGeometry(40, 60, 420, 280);
    original.findChild<CanvasPanel *>("dashboardPanel_two")->setGeometry(780, 330, 500, 380);
    const QJsonObject saved = original.saveLayout();

    DashboardWorkspace reopened("saved");
    reopened.resize(320, 220);
    QVERIFY(reopened.addPanel("one", "One", new QLabel("one")));
    QVERIFY(reopened.addPanel("two", "Two", new QLabel("two")));
    QVERIFY(reopened.restoreLayout(saved));
    // Saving another dashboard before this one is shown must not overwrite it.
    QCOMPARE(reopened.saveLayout().value("canvas"), saved.value("canvas"));
    reopened.resize(1400, 850);
    reopened.show();
    QTest::qWait(1);
    QCOMPARE(reopened.findChild<CanvasPanel *>("dashboardPanel_one")->geometry(), QRect(40, 60, 420, 280));
    QCOMPARE(reopened.findChild<CanvasPanel *>("dashboardPanel_two")->geometry(), QRect(780, 330, 500, 380));
    QCOMPARE(reopened.saveLayout().value("canvasOrder"), saved.value("canvasOrder"));
}

void DashboardWorkspaceTest::clickingContentSavesStackingOrder()
{
    DashboardWorkspace workspace("stacking");
    workspace.resize(1100, 700);
    workspace.show();
    auto *first = new QPushButton("First panel content");
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    QSignalSpy changes(&workspace, &DashboardWorkspace::layoutChanged);
    QTest::mouseClick(first, Qt::LeftButton);
    QVERIFY(!changes.isEmpty());
    QCOMPARE(workspace.saveLayout().value("canvasOrder").toArray().last().toString(), QString("one"));
    changes.clear();
    QTest::mouseClick(first, Qt::LeftButton);
    QVERIFY(changes.isEmpty());
}

void DashboardWorkspaceTest::preservesLiveWidgetsAndBothArrangements()
{
    DashboardWorkspace workspace("test");
    workspace.resize(1100, 700);
    workspace.show();
    auto *input = new QLineEdit("ongoing session");
    QPointer<QLineEdit> retained(input);
    QVERIFY(workspace.addPanel("one", "One", input));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    QVERIFY(workspace.addPanel("three", "Three", new QLabel("third")));
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    frame->setGeometry(80, 90, 400, 260);
    const QJsonObject canvasLayout = workspace.saveLayout();

    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.floatPanel("three"));
    QVERIFY(!frame->isWindow());
    QCOMPARE(workspace.findChildren<QTabWidget *>().size(), 1);
    QCOMPARE(workspace.findChild<QTabWidget *>()->count(), 2);
    auto *tabs = workspace.findChild<QTabWidget *>();
    QVERIFY(workspace.activatePanel("one"));
    QVERIFY(selectedFrameIsVisible(tabs, frame));
    QVERIFY(workspace.activatePanel("two"));
    QVERIFY(selectedFrameIsVisible(tabs, workspace.findChild<CanvasPanel *>("dashboardPanel_two")));
    QVERIFY(!workspace.activatePanel("missing"));
    QCOMPARE(retained->text(), QString("ongoing session"));
    auto *floating = workspace.findChild<CanvasPanel *>("dashboardPanel_three");
    QCOMPARE(floating->parentWidget(), &workspace);
    QVERIFY(!floating->isWindow());
    const QJsonObject tileLayout = workspace.saveLayout();

    workspace.setMode("canvas");
    QCOMPARE(frame->geometry(), QRect(80, 90, 400, 260));
    QVERIFY(retained);
    QCOMPARE(retained->text(), QString("ongoing session"));
    workspace.setMode("tiling");
    QCOMPARE(workspace.findChild<QTabWidget *>()->count(), 2);
    QCOMPARE(workspace.saveLayout().value("floating"), tileLayout.value("floating"));
    QVERIFY(workspace.restoreLayout(canvasLayout));
    QCOMPARE(workspace.mode(), QString("canvas"));
    QVERIFY(workspace.restoreLayout(tileLayout));
    QCOMPARE(workspace.mode(), QString("tiling"));
    QVERIFY(retained);
    QVERIFY(workspace.dockPanel("three", "one", "bottom"));
    QCOMPARE(workspace.findChildren<QSplitter *>().size(), 1);
    QVERIFY(workspace.saveLayout().value("floating").toArray().isEmpty());
}

void DashboardWorkspaceTest::replacingTileViewportPreservesMinimizedAndFloatingPanels()
{
    DashboardWorkspace workspace("viewport-lifetime");
    workspace.resize(1100, 700);
    workspace.show();
    QPointer<QLineEdit> active = new QLineEdit("active draft");
    QPointer<QLineEdit> minimized = new QLineEdit("minimized draft");
    QPointer<QLineEdit> floating = new QLineEdit("floating draft");
    QVERIFY(workspace.addPanel("active", "Active", active));
    QVERIFY(workspace.addPanel("minimized", "Minimized", minimized));
    QVERIFY(workspace.addPanel("floating", "Floating", floating));
    workspace.setMode("tiling");
    QVERIFY(workspace.minimizePanel("minimized"));
    QPointer<CanvasPanel> minimizedFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_minimized");
    QPointer<CanvasPanel> floatingFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_floating");
    QPointer<QScrollArea> oldViewport = workspace.findChild<QScrollArea *>("dashboardTileViewport");
    QVERIFY(minimizedFrame && floatingFrame && oldViewport);

    QVERIFY(workspace.floatPanel("floating"));
    QVERIFY(oldViewport.isNull());
    QVERIFY(active && minimized && floating && minimizedFrame && floatingFrame);
    QVERIFY(!minimizedFrame->isVisible());
    QCOMPARE(floatingFrame->parentWidget(), &workspace);
    const QJsonObject tiledLayout = workspace.saveLayout();
    oldViewport = workspace.findChild<QScrollArea *>("dashboardTileViewport");
    QVERIFY(oldViewport);

    // Public layouts must account for every panel before replacing its owner.
    QJsonObject omittedPanel = tiledLayout;
    omittedPanel["tree"] = QJsonObject{{"type", "tabs"}, {"ids", QJsonArray{"active"}}, {"active", "active"}};
    QVERIFY(!workspace.restoreLayout(omittedPanel));
    QVERIFY(oldViewport && minimizedFrame);
    QCOMPARE(workspace.saveLayout(), tiledLayout);

    QJsonObject allFloating = tiledLayout;
    allFloating["tree"] = QJsonObject{};
    allFloating["floating"] = QJsonArray{"active", "minimized", "floating"};
    QVERIFY(workspace.restoreLayout(allFloating));
    QVERIFY(oldViewport.isNull());
    QVERIFY(workspace.findChildren<QScrollArea *>("dashboardTileViewport").isEmpty());
    QVERIFY(active && minimized && floating && minimizedFrame && floatingFrame);
    QCOMPARE(minimizedFrame->parentWidget(), &workspace);
    QCOMPARE(floatingFrame->parentWidget(), &workspace);
    QVERIFY(!minimizedFrame->isVisible());

    QVERIFY(workspace.restoreLayout(tiledLayout));
    QCOMPARE(workspace.findChildren<QScrollArea *>("dashboardTileViewport").size(), 1);
    QVERIFY(!minimizedFrame->isVisible());
    QVERIFY(workspace.activatePanel("minimized"));
    QVERIFY(minimizedFrame->isVisible());
    QCOMPARE(active->text(), QString("active draft"));
    QCOMPARE(minimized->text(), QString("minimized draft"));
    QCOMPARE(floating->text(), QString("floating draft"));
}

void DashboardWorkspaceTest::onlySelectedTabIsVisibleAfterRebuild()
{
    DashboardWorkspace workspace("tab-visibility");
    workspace.resize(1100, 700);
    workspace.show();
    const QStringList ids{"one", "two", "three"};
    for (const QString &id : ids)
        QVERIFY(workspace.addPanel(id, id, new QLineEdit(id)));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.dockPanel("three", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    const QJsonObject saved = workspace.saveLayout();

    // Rebuilding a group whose selected page comes before other pages used to
    // show every later page, including their native browser/terminal surfaces.
    for (int cycle = 0; cycle < 3; ++cycle) {
        workspace.setMode("canvas");
        QVERIFY(workspace.restoreLayout(saved));
        auto *group = workspace.findChild<QTabWidget *>();
        QVERIFY(group);
        QCOMPARE(group->count(), 3);
        for (const QString &active : ids) {
            QVERIFY(workspace.activatePanel(active));
            for (const QString &id : ids) {
                auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_" + id);
                QVERIFY(frame);
                QVERIFY(!frame->isWindow());
                QCOMPARE(frame->isVisible(), id == active);
                QCOMPARE(frame->findChild<QLineEdit *>()->isVisible(), id == active);
            }
        }
    }
}

void DashboardWorkspaceTest::validatesBeforeChangingLayout()
{
    DashboardWorkspace workspace("validation");
    workspace.resize(900, 600);
    QVERIFY(workspace.addPanel("one", "One", new QLabel("one")));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("two")));
    const QJsonObject original = workspace.saveLayout();
    QJsonObject malformed = original;
    malformed["tree"] = QJsonObject{{"type", "tabs"}, {"ids", QJsonArray{"one", "one"}}, {"active", "one"}};
    QVERIFY(!workspace.restoreLayout(malformed));
    QCOMPARE(workspace.saveLayout(), original);
    malformed = original;
    malformed["canvas"] = QJsonObject{{"one", QJsonArray{0, 0, 1000000000, 200}}};
    QVERIFY(!workspace.restoreLayout(malformed));
    malformed = original;
    malformed["floating"] = QJsonArray{"unknown"};
    QVERIFY(!workspace.restoreLayout(malformed));
    QJsonObject tree{{"type", "tabs"}, {"ids", QJsonArray{"one"}}, {"active", "one"}};
    for (int depth = 0; depth < 32; ++depth)
        tree = {{"type", "split"}, {"orientation", "horizontal"}, {"sizes", QJsonArray{1, 1}},
                {"children", QJsonArray{tree, QJsonObject{{"type", "tabs"}, {"ids", QJsonArray{"two"}}, {"active", "two"}}}}};
    malformed = original;
    malformed["tree"] = tree;
    QVERIFY(!workspace.restoreLayout(malformed));
    QCOMPARE(workspace.saveLayout(), original);
    QVERIFY(!workspace.dockPanel("one", "missing", "tab"));
    QCOMPARE(workspace.saveLayout(), original);
}

void DashboardWorkspaceTest::boundsPanelCountAndKeepsCloseExplicit()
{
    DashboardWorkspace workspace("limits");
    workspace.resize(1100, 700);
    for (int index = 0; index < 16; ++index)
        QVERIFY(workspace.addPanel(QString::number(index), QString::number(index), new QLabel("widget")));
    auto *extra = new QLabel;
    QVERIFY(!workspace.addPanel("extra", "Extra", extra));
    QVERIFY(!extra->parent());
    delete extra;
    QVERIFY(workspace.restoreLayout(workspace.saveLayout()));
    QPointer<CanvasPanel> frame = workspace.findChild<CanvasPanel *>("dashboardPanel_0");
    workspace.removePanel("0");
    QVERIFY(!frame);
    QCOMPARE(workspace.saveLayout().value("canvas").toObject().size(), 15);
}

void DashboardWorkspaceTest::placesNewPanelsInFreeSpaceAndHonorsContentSize()
{
    DashboardWorkspace workspace("placement");
    // Pages can receive widgets before the enclosing stacked layout assigns its
    // final size. First show must use that real space rather than a tiny default.
    auto *browser = new QLabel("browser stand-in");
    browser->setMinimumSize(600, 320);
    QVERIFY(workspace.addPanel("browser", "Browser", browser));
    QVERIFY(workspace.addPanel("logs", "Logs", new QLabel("logs")));
    workspace.resize(1200, 750);
    workspace.show();
    QTest::qWait(1);
    auto *browserFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_browser");
    auto *logsFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_logs");
    QVERIFY(browserFrame->width() >= browser->minimumWidth());
    QVERIFY(browserFrame->height() >= browser->minimumHeight());
    QVERIFY(!browserFrame->geometry().intersects(logsFrame->geometry()));
    QVERIFY(workspace.rect().contains(browserFrame->geometry()));
    QVERIFY(workspace.rect().contains(logsFrame->geometry()));

    // Arranging another widget does not reset a user's intentional overlap.
    logsFrame->setGeometry(browserFrame->geometry().translated(30, 30));
    const QRect manual = logsFrame->geometry();
    QVERIFY(workspace.addPanel("system", "System", new QLabel("system")));
    QCOMPARE(logsFrame->geometry(), manual);
    QVERIFY(browserFrame->geometry().intersects(logsFrame->geometry()));
    workspace.setMode("tiling");
    workspace.setMode("canvas");
    QCOMPARE(logsFrame->geometry(), manual);
}

void DashboardWorkspaceTest::tightTilesFillPanesAndMinimizeToDock()
{
    DashboardWorkspace workspace("tight-tiles");
    workspace.resize(1000, 700);
    workspace.show();
    auto *first = new QLineEdit("retained input");
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    workspace.setMode("tiling");
    QCoreApplication::processEvents();
    auto *split = workspace.findChild<QSplitter *>();
    QVERIFY(split);
    QCOMPARE(split->handleWidth(), 0);
    QCOMPARE(split->widget(1)->x(), split->widget(0)->width());
    for (auto *group : workspace.findChildren<QTabWidget *>()) {
        QCOMPARE(group->tabBar()->tabRect(0).width(), group->width());
        auto *frame = group->currentWidget()->findChild<CanvasPanel *>();
        QVERIFY(frame);
        QCOMPARE(frame->width(), group->width());
        QCOMPARE(frame->layout()->contentsMargins(), QMargins());
        auto *controls = group->tabBar()->tabButton(0, QTabBar::LeftSide);
        QCOMPARE(controls->findChildren<QAbstractButton *>().size(), 3);
    }
    const QJsonObject saved = workspace.saveLayout();
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    auto *originalParent = frame->parentWidget();
    QVERIFY(workspace.minimizePanel("one"));
    QVERIFY(!first->isVisible());
    QCoreApplication::processEvents();
    QVERIFY(split->widget(1)->width() >= workspace.width());
    QCOMPARE(frame->parentWidget(), originalParent);
    QCOMPARE(workspace.saveLayout(), saved);
    QVERIFY(workspace.activatePanel("one"));
    QCoreApplication::processEvents();
    QVERIFY(first->isVisible());
    QCOMPARE(first->text(), QString("retained input"));
    QCOMPARE(workspace.saveLayout().value("tree"), saved.value("tree"));
    QVERIFY(workspace.minimizePanel("one"));
    QVERIFY(workspace.minimizePanel("two"));
    QVERIFY(!split->isVisible());
    QVERIFY(workspace.activatePanel("one"));
    QVERIFY(first->isVisible());
    QVERIFY(workspace.activatePanel("two"));
    workspace.setCompactTiling(false);
    QCOMPARE(split->handleWidth(), 7);
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.minimizePanel("one"));
    QVERIFY(workspace.minimizePanel("two"));
    const QJsonObject minimizedLayout = workspace.saveLayout();
    QVERIFY(workspace.restoreLayout(minimizedLayout));
    QVERIFY(workspace.activatePanel("one"));
    QVERIFY(first->isVisible());
    QVERIFY(workspace.activatePanel("two"));
    workspace.setMode("canvas");
    const QRect originalGeometry = frame->geometry();
    auto *minimize = frame->findChild<QAbstractButton *>("dashboardPanelMinimize");
    QVERIFY(minimize);
    minimize->click();
    QVERIFY(!first->isVisible());
    QVERIFY(workspace.activatePanel("one"));
    QVERIFY(first->isVisible());
    QCOMPARE(frame->geometry(), originalGeometry);
}

void DashboardWorkspaceTest::tiledChromeUsesOneHeaderAndRetainsActions()
{
    DashboardWorkspace workspace("compact-chrome");
    workspace.resize(1000, 700);
    workspace.show();
    auto *content = new QWidget;
    auto *contentLayout = new QVBoxLayout(content);
    contentLayout->addWidget(new QLineEdit("retained state"));
    auto *footer = new QWidget(content);
    footer->setObjectName("dashboardWidgetFooter");
    auto *footerLayout = new QVBoxLayout(footer);
    auto *open = new QPushButton("Open full view", footer);
    open->setObjectName("dashboardOpenFullView");
    footerLayout->addWidget(open);
    contentLayout->addWidget(footer);
    content->setProperty("canvasPanelTransparent", true);
    QVERIFY(workspace.addPanel("one", "One", content));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    auto *header = frame->findChild<QWidget *>("dashboardPanelHeader");
    QVERIFY(header->isVisible());
    QVERIFY(footer->isVisible());
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    auto *group = workspace.findChild<QTabWidget *>();
    QVERIFY(group);
    QVERIFY(header->isHidden());
    QVERIFY(footer->isHidden());
    QVERIFY(group->tabsClosable());
    QCOMPARE(frame->layout()->contentsMargins(), QMargins(0, 0, 0, 0));
    QVERIFY(frame->property("canvasPanelTransparent").toBool());

    workspace.setPanelTitle("one", "Updated app title");
    QPixmap iconImage(16, 16);
    iconImage.fill(Qt::cyan);
    workspace.setPanelIcon("one", QIcon(iconImage));
    const int index = panelTabIndex(group, frame);
    QCOMPARE(group->tabText(index), QString("Updated app title"));
    QVERIFY(!group->tabIcon(index).isNull());
    QSignalSpy opened(open, &QPushButton::clicked);
    const QPoint tabPoint = group->tabBar()->tabRect(index).center();
    QVERIFY(QMetaObject::invokeMethod(group->tabBar(), "customContextMenuRequested", Qt::DirectConnection, Q_ARG(QPoint, tabPoint)));
    auto *menu = workspace.findChild<QMenu *>();
    QVERIFY(menu);
    QAction *openAction = nullptr;
    for (QAction *action : menu->actions()) {
        if (action->text() == "Open full view")
            openAction = action;
    }
    QVERIFY(openAction);
    openAction->trigger();
    QCOMPARE(opened.count(), 1);
    menu->close();

    workspace.setCompactTiling(false);
    QVERIFY(footer->isVisible());
    QVERIFY(header->isHidden());
    workspace.setShowPanelHeaders(false);
    workspace.setMode("canvas");
    QVERIFY(header->isHidden());
    workspace.setShowPanelHeaders(true);
    QVERIFY(header->isVisible());
    workspace.setCompactTiling(true);
    workspace.setMode("tiling");
    group = workspace.findChild<QTabWidget *>();
    const int closingIndex = panelTabIndex(group, frame);
    auto *close = group->tabBar()->tabButton(closingIndex, QTabBar::LeftSide)->findChild<QAbstractButton *>("dashboardTabClose");
    QVERIFY(close);
    QCOMPARE(close->objectName(), QString("dashboardTabClose"));
    auto *expand = group->tabBar()->tabButton(closingIndex, QTabBar::LeftSide)->findChild<QAbstractButton *>("dashboardTabExpand");
    QVERIFY(expand);
    QCOMPARE(expand->objectName(), QString("dashboardTabExpand"));
    QSignalSpy closed(&workspace, &DashboardWorkspace::panelCloseRequested);
    QPointer<QWidget> retained(content);
    close->click();
    QCOMPARE(closed.count(), 1);
    QCOMPARE(closed.first().first().toString(), QString("one"));
    QVERIFY(retained); // Owner decides removal; clicking does not cancel a job.
    workspace.removePanel("one");
    QVERIFY(!retained);
}

void DashboardWorkspaceTest::canvasBackgroundCachesDecodedImage()
{
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    const QString path = directory.filePath("wallpaper.png");
    QImage image(64, 64, QImage::Format_RGB32);
    image.fill(Qt::red);
    QVERIFY(image.save(path));
    DashboardWorkspace workspace("background");
    workspace.resize(640, 480);
    workspace.show();
    QVERIFY(workspace.setCanvasBackground("image", path));
    QCOMPARE(workspace.grab().toImage().pixelColor(5, 5), QColor(Qt::red));
    QVERIFY(QFile::remove(path));
    DashboardWorkspace tiling("shared-background");
    tiling.resize(640, 480);
    tiling.setMode("tiling");
    tiling.show();
    tiling.copyBackgroundFrom(workspace);
    QCOMPARE(tiling.grab().toImage().pixelColor(5, 5), QColor(Qt::red));
    const int objects = workspace.findChildren<QObject *>().size();
    // Repainting and resizes continue from the cache after the source vanishes.
    // This verifies the performance-critical no-decode/no-file-read paint path.
    for (int index = 0; index < 12; ++index) {
        workspace.resize(640 + index, 480 + index);
        QCOMPARE(workspace.grab().toImage().pixelColor(5, 5), QColor(Qt::red));
    }
    QCOMPARE(workspace.findChildren<QObject *>().size(), objects);
    QVERIFY(workspace.setCanvasBackground("image", path));
    QVERIFY(!workspace.setCanvasBackground("image", directory.filePath("missing.png")));
    QVERIFY(!workspace.setCanvasBackground("unsupported"));
    QFile oversized(directory.filePath("oversized.png"));
    QVERIFY(oversized.open(QIODevice::WriteOnly));
    QVERIFY(oversized.resize(20 * 1024 * 1024 + 1));
    oversized.close();
    QVERIFY(!workspace.setCanvasBackground("image", oversized.fileName()));
    QFile corrupt(directory.filePath("corrupt.png"));
    QVERIFY(corrupt.open(QIODevice::WriteOnly));
    QCOMPARE(corrupt.write("not an image"), qint64(12));
    corrupt.close();
    QVERIFY(!workspace.setCanvasBackground("image", corrupt.fileName()));
    QCOMPARE(workspace.grab().toImage().pixelColor(5, 5), QColor(Qt::red));
    QVERIFY(workspace.setCanvasBackground("plain"));
    QVERIFY(workspace.grab().toImage().pixelColor(5, 5) != QColor(Qt::red));
    QVERIFY(workspace.setCanvasBackground("dusk"));
    QVERIFY(workspace.setCanvasBackground("ocean"));
    QVERIFY(workspace.setCanvasBackground("grid"));
    workspace.setMode("tiling");
    const QImage tiled = workspace.grab().toImage();
    QVERIFY(workspace.setCanvasBackground("dusk"));
    QVERIFY(workspace.grab().toImage() != tiled);
}

void DashboardWorkspaceTest::droppingOnTabStripJoinsGroup()
{
    DashboardWorkspace workspace("tab-drop");
    workspace.resize(1000, 700);
    workspace.show();
    QVERIFY(workspace.addPanel("one", "One", new QLabel("first")));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    workspace.setMode("tiling");
    QCoreApplication::processEvents();
    auto *targetFrame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    QTabWidget *targetGroup = nullptr;
    for (auto *group : workspace.findChildren<QTabWidget *>()) {
        if (panelTabIndex(group, targetFrame) >= 0)
            targetGroup = group;
    }
    QVERIFY(targetGroup);
    const QPoint target = targetGroup->tabBar()->mapTo(&workspace, targetGroup->tabBar()->tabRect(0).center());
    QMimeData mime;
    mime.setData("application/x-dockpipe-dashboard-panel", "tab-drop\ntwo");
    QDragEnterEvent enter(target, Qt::MoveAction, &mime, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(&workspace, &enter);
    QVERIFY(enter.isAccepted());
    QDropEvent drop(QPointF(target), Qt::MoveAction, &mime, Qt::LeftButton, Qt::NoModifier);
    QApplication::sendEvent(&workspace, &drop);
    QVERIFY(drop.isAccepted());
    QCOMPARE(workspace.findChildren<QTabWidget *>().size(), 1);
    QCOMPARE(workspace.findChild<QTabWidget *>()->count(), 2);
    QCOMPARE(workspace.findChildren<QSplitter *>().size(), 0);
}

void DashboardWorkspaceTest::expandsCanvasWithoutChangingSavedLayout()
{
    DashboardWorkspace workspace("canvas-expansion");
    workspace.resize(1100, 700);
    workspace.show();
    auto *input = new QLineEdit("live state");
    QVERIFY(workspace.addPanel("one", "One", input));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    auto *other = workspace.findChild<CanvasPanel *>("dashboardPanel_two");
    frame->setGeometry(40, 50, 450, 280);
    const QRect original = frame->geometry();
    QWidget *originalParent = input->parentWidget();
    workspace.setShowPanelHeaders(false);
    const QJsonObject saved = workspace.saveLayout();
    QSignalSpy changed(&workspace, &DashboardWorkspace::layoutChanged);
    QVERIFY(!workspace.togglePanelExpanded("missing"));
    QVERIFY(workspace.togglePanelExpanded("one"));
    QCOMPARE(workspace.expandedPanel(), QString("one"));
    QCOMPARE(frame->geometry(), workspace.rect());
    QVERIFY(other->isHidden());
    auto *header = frame->findChild<QWidget *>("dashboardPanelHeader");
    QVERIFY(header->isVisible());
    auto *restore = header->findChild<QAbstractButton *>("dashboardPanelExpand");
    QVERIFY(restore);
    QCOMPARE(restore->toolTip(), QString("Restore panel"));
    QCOMPARE(workspace.saveLayout(), saved);
    QCOMPARE(input->parentWidget(), originalParent);
    QCOMPARE(input->text(), QString("live state"));
    workspace.resize(1250, 820);
    QCOMPARE(frame->geometry(), workspace.rect());
    QCOMPARE(workspace.saveLayout(), saved);
    restore->click();
    QVERIFY(workspace.expandedPanel().isEmpty());
    QCOMPARE(frame->geometry(), original);
    QVERIFY(other->isVisible());
    QVERIFY(header->isHidden());
    QCOMPARE(workspace.saveLayout(), saved);
    QCOMPARE(changed.count(), 0);
}

void DashboardWorkspaceTest::expandsTilesWithoutReparentingLiveContent()
{
    DashboardWorkspace workspace("tile-expansion");
    workspace.resize(1300, 800);
    workspace.show();
    auto *input = new QLineEdit("live terminal stand-in");
    QVERIFY(workspace.addPanel("one", "One", input));
    QVERIFY(workspace.addPanel("two", "Two", new QLabel("second")));
    QVERIFY(workspace.addPanel("three", "Three", new QLabel("third")));
    workspace.setMode("tiling");
    QVERIFY(workspace.dockPanel("two", "one", "tab"));
    QVERIFY(workspace.activatePanel("one"));
    QCoreApplication::processEvents();
    const QJsonObject saved = workspace.saveLayout();
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    QWidget *originalParent = frame->parentWidget();
    QPointer<QSplitter> retainedSplit = workspace.findChild<QSplitter *>();
    QPointer<QTabWidget> retainedGroup;
    for (auto *group : workspace.findChildren<QTabWidget *>()) {
        if (panelTabIndex(group, frame) >= 0)
            retainedGroup = group;
    }
    QVERIFY(retainedGroup);
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    const int objectCount = workspace.findChildren<QObject *>().size();
    const int index = panelTabIndex(retainedGroup, frame);
    auto *expand = retainedGroup->tabBar()->tabButton(index, QTabBar::LeftSide)->findChild<QAbstractButton *>("dashboardTabExpand");
    QVERIFY(expand);
    expand->click();
    QCOMPARE(workspace.expandedPanel(), QString("one"));
    QVERIFY(retainedSplit);
    QCOMPARE(frame->parentWidget(), originalParent);
    QVERIFY(frame->findChild<QWidget *>("dashboardPanelHeader")->isHidden());
    QVERIFY(retainedGroup->isVisible());
    QVERIFY(retainedGroup->width() >= workspace.width() - 24);
    QTRY_VERIFY(retainedGroup->height() >= workspace.height() - 24);
    QCOMPARE(workspace.saveLayout(), saved);
    QCOMPARE(workspace.findChildren<QObject *>().size(), objectCount);
    // Group tabs remain usable while expanded, including the restore control.
    const int secondIndex = panelTabIndex(retainedGroup, workspace.findChild<CanvasPanel *>("dashboardPanel_two"));
    retainedGroup->setCurrentIndex(secondIndex);
    QCOMPARE(workspace.expandedPanel(), QString("two"));
    QVERIFY(workspace.togglePanelExpanded("two"));
    QVERIFY(workspace.expandedPanel().isEmpty());
    QCOMPARE(workspace.saveLayout(), saved);
    QVERIFY(retainedSplit);
    QCOMPARE(frame->parentWidget(), originalParent);
    QCOMPARE(input->text(), QString("live terminal stand-in"));
}

void DashboardWorkspaceTest::expansionRestoresBeforeRemovalAndLayoutChanges()
{
    DashboardWorkspace workspace("expansion-lifecycle");
    workspace.resize(1000, 700);
    workspace.show();
    QPointer<QLineEdit> first = new QLineEdit("first");
    QPointer<QLineEdit> second = new QLineEdit("second");
    QVERIFY(workspace.addPanel("one", "One", first));
    QVERIFY(workspace.addPanel("two", "Two", second));
    const QJsonObject saved = workspace.saveLayout();
    QVERIFY(workspace.togglePanelExpanded("one"));
    QVERIFY(workspace.restoreLayout(saved));
    QVERIFY(workspace.expandedPanel().isEmpty());
    QVERIFY(first);
    QVERIFY(second);
    QVERIFY(workspace.togglePanelExpanded("one"));
    workspace.setMode("tiling");
    QVERIFY(workspace.expandedPanel().isEmpty());
    QVERIFY(workspace.togglePanelExpanded("one"));
    workspace.hide();
    QCOMPARE(workspace.expandedPanel(), QString("one"));
    workspace.show();
    QCOMPARE(workspace.expandedPanel(), QString("one"));
    connect(&workspace, &DashboardWorkspace::panelCloseRequested, &workspace, &DashboardWorkspace::removePanel);
    auto *frame = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    for (auto *group : workspace.findChildren<QTabWidget *>()) {
        const int index = panelTabIndex(group, frame);
        if (index < 0)
            continue;
        auto *close = group->tabBar()->tabButton(index, QTabBar::LeftSide)->findChild<QAbstractButton *>("dashboardTabClose");
        QVERIFY(close);
        close->click();
        break;
    }
    QVERIFY(workspace.expandedPanel().isEmpty());
    QVERIFY(!first);
    QVERIFY(second);
    QVERIFY(second->isVisible());
    QVERIFY(workspace.restoreLayout(workspace.saveLayout()));
}

void DashboardWorkspaceTest::restoringExpansionAfterShrinkKeepsPanelsReachable()
{
    DashboardWorkspace workspace("shrink-expanded");
    workspace.resize(1100, 750);
    workspace.show();
    QVERIFY(workspace.addPanel("one", "One", new QLineEdit("first")));
    QVERIFY(workspace.addPanel("two", "Two", new QLineEdit("second")));
    auto *first = workspace.findChild<CanvasPanel *>("dashboardPanel_one");
    auto *second = workspace.findChild<CanvasPanel *>("dashboardPanel_two");
    first->setGeometry(40, 50, 500, 340);
    second->setGeometry(560, 380, 500, 340);
    const QJsonObject original = workspace.saveLayout();
    QVERIFY(workspace.togglePanelExpanded("one"));
    workspace.resize(380, 280);
    QCOMPARE(first->geometry(), workspace.rect());
    QCOMPARE(workspace.saveLayout(), original);
    QVERIFY(workspace.togglePanelExpanded("one"));
    QVERIFY(workspace.rect().contains(first->geometry()));
    QVERIFY(workspace.rect().contains(second->geometry()));
    QVERIFY(first->isVisible());
    QVERIFY(second->isVisible());
    const QJsonObject restored = workspace.saveLayout();
    QCOMPARE(restored.value("canvasOrder"), original.value("canvasOrder"));
    QCOMPARE(restored.value("tree"), original.value("tree"));
    QVERIFY(!restored.contains("expandedPanel"));
    QVERIFY(workspace.restoreLayout(restored));
}

QTEST_MAIN(DashboardWorkspaceTest)
#include "DashboardWorkspaceTest.moc"
