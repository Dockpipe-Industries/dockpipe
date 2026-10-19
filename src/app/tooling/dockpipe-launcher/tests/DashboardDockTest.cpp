#include "DashboardDock.h"
#include "DashboardStore.h"

#include <QApplication>
#include <QCursor>
#include <QEnterEvent>
#include <QPointer>
#include <QScrollArea>
#include <QScrollBar>
#include <QSignalSpy>
#include <QTest>
#include <QTimer>
#include <QToolButton>
#include <QVBoxLayout>

class DashboardDockTest final : public QObject {
    Q_OBJECT
private slots:
    void pinnedShortcutsActivateExistingPanels();
    void overlayRevealsWithoutResizingAndStopsWhenHidden();
    void preservesButtonsAndUpdatesActiveIndicators();
    void preferredWidthFitsWithoutScrolling();
    void applicationIconsAndDesktopMode();
    void boundsRunningPanelsAndScrollsKeyboardFocus();
};

void DashboardDockTest::overlayRevealsWithoutResizingAndStopsWhenHidden()
{
    QWidget area;
    area.resize(900, 600);
    auto *dock = new DashboardDock(&area);
    dock->setOverlayArea(&area);
    area.show();
    auto *edge = area.findChild<QWidget *>("dashboardDockRevealEdge");
    QVERIFY(edge);
    QVERIFY(edge->isHidden());
    auto *hover = dock->findChild<QTimer *>("dashboardDockHoverMonitor");
    QVERIFY(hover && hover->isActive());
    QVERIFY(dock->isHidden());
    const QRect original = area.rect();
    QCursor::setPos(area.mapToGlobal(QPoint(20, 20)));
    QCursor::setPos(edge->mapToGlobal(edge->rect().center()));
    QTRY_VERIFY(dock->isVisible());
    QTRY_VERIFY(dock->geometry().bottom() < area.height());
    QCOMPARE(area.rect(), original);
    QVERIFY(edge->isHidden());
    QCursor::setPos(area.mapToGlobal(QPoint(20, 20)));
    dock->reveal(true);
    auto *apps = dock->findChild<QToolButton *>("dashboardDockApps");
    auto *browser = dock->findChild<QToolButton *>("dashboardDockPinned_browser");
    QTRY_VERIFY(apps->hasFocus());
    QTest::keyClick(apps, Qt::Key_Tab);
    QTRY_VERIFY(browser->hasFocus());
    QTest::qWait(750);
    QVERIFY(dock->isVisible());
    browser->clearFocus();
    QEvent leave(QEvent::Leave);
    QApplication::sendEvent(dock, &leave);
    QTRY_VERIFY_WITH_TIMEOUT(dock->isHidden(), 1500);
    QVERIFY(!dock->findChild<QTimer *>("dashboardDockHideDelay")->isActive());
    dock->reveal();
    area.resize(450, 400);
    QTRY_VERIFY(dock->width() <= area.width());
    area.hide();
    QVERIFY(!dock->findChild<QTimer *>("dashboardDockHideDelay")->isActive());
    QVERIFY(!hover->isActive());
    area.show();
    QVERIFY(dock->isHidden());
    dock->setOverlayArea(nullptr);
    QVERIFY(!hover->isActive());
    QVERIFY(!area.findChild<QWidget *>("dashboardDockRevealEdge"));
}

void DashboardDockTest::pinnedShortcutsActivateExistingPanels()
{
    DashboardDock dock;
    QSignalSpy add(&dock, &DashboardDock::addWidgetRequested);
    QSignalSpy activate(&dock, &DashboardDock::activatePanelRequested);
    QSignalSpy apps(&dock, &DashboardDock::appsRequested);
    auto *browser = dock.findChild<QToolButton *>("dashboardDockPinned_browser");
    auto *desktop = dock.findChild<QToolButton *>("dashboardDockPinned_desktop-window");
    auto *terminal = dock.findChild<QToolButton *>("dashboardDockPinned_terminal");
    QVERIFY(browser && desktop && terminal);
    browser->click();
    QCOMPARE(add.count(), 1);
    QCOMPARE(add.takeFirst().at(0).toString(), QString("browser"));
    desktop->click();
    QCOMPARE(add.takeFirst().at(0).toString(), QString("desktop-window"));
    terminal->click();
    QCOMPARE(add.takeFirst().at(0).toString(), QString("terminal"));
    dock.findChild<QToolButton *>("dashboardDockApps")->click();
    QCOMPARE(apps.count(), 1);
    dock.setPanels({{"first", "First browser", "browser"}, {"second", "Second browser", "browser"}});
    browser->click();
    QCOMPARE(activate.takeFirst().at(0).toString(), QString("first"));
    dock.setActivePanel("second");
    browser->click();
    QCOMPARE(activate.takeFirst().at(0).toString(), QString("second"));
    QVERIFY(add.isEmpty());
    QVERIFY(browser->property("dashboardRunning").toBool());
    QVERIFY(browser->isChecked());
}

void DashboardDockTest::preservesButtonsAndUpdatesActiveIndicators()
{
    DashboardDock dock;
    dock.resize(700, dock.sizeHint().height());
    dock.show();
    const QVector<DashboardDockItem> panels = {{"files", "Project files", "files"}, {"logs", "Run logs", "logs"}};
    dock.setPanels(panels);
    QPointer<QToolButton> files = dock.findChild<QToolButton *>("dashboardDockPanel_files");
    QPointer<QToolButton> logs = dock.findChild<QToolButton *>("dashboardDockPanel_logs");
    QVERIFY(files && logs);
    dock.setActivePanel("files");
    QVERIFY(files->isChecked());
    QVERIFY(dock.findChild<QToolButton *>("dashboardDockPinned_files")->isChecked());
    dock.setPanels(panels);
    QCOMPARE(dock.findChild<QToolButton *>("dashboardDockPanel_files"), files.data());
    QCOMPARE(dock.findChild<QToolButton *>("dashboardDockPanel_logs"), logs.data());
    dock.setPanels({{"logs", "Updated <logs>", "logs"}, {"files", "Project files", "files"}});
    QCOMPARE(dock.findChild<QToolButton *>("dashboardDockPanel_logs"), logs.data());
    QCOMPARE(logs->accessibleName(), QString("Updated <logs>"));
    QVERIFY(logs->toolTip().contains("&lt;logs&gt;"));
    QSignalSpy activated(&dock, &DashboardDock::activatePanelRequested);
    logs->click();
    QCOMPARE(activated.takeFirst().at(0).toString(), QString("logs"));
    dock.setActivePanel("logs");
    QVERIFY(logs->isChecked() && !files->isChecked());
    dock.setPanels({{"logs", "Updated <logs>", "logs"}});
    QVERIFY(!files && logs);
    QVERIFY(!dock.findChild<QToolButton *>("dashboardDockPinned_files")->property("dashboardRunning").toBool());
    QVERIFY(dock.findChildren<QTimer *>().isEmpty());
}

void DashboardDockTest::applicationIconsAndDesktopMode()
{
    DashboardDock dock;
    QPixmap pixels(24, 24);
    pixels.fill(Qt::red);
    const QIcon red(pixels);
    dock.setPanels({{"app", "An application", "desktop-window", red}});
    auto *button = dock.findChild<QToolButton *>("dashboardDockPanel_app");
    QVERIFY(button);
    QCOMPARE(button->icon().cacheKey(), red.cacheKey());
    pixels.fill(Qt::blue);
    const QIcon blue(pixels);
    dock.setPanels({{"app", "An application", "desktop-window", blue}});
    QCOMPARE(dock.findChild<QToolButton *>("dashboardDockPanel_app"), button);
    QCOMPARE(button->icon().cacheKey(), blue.cacheKey());
    dock.setPanels({{"app", "An application", "desktop-window"}});
    QVERIFY(!button->icon().isNull());
    QVERIFY(button->icon().cacheKey() != blue.cacheKey());

    QSignalSpy add(&dock, &DashboardDock::addWidgetRequested);
    auto *desktop = dock.findChild<QToolButton *>("dashboardDockPinned_desktop-window");
    dock.setPanels({});
    dock.setDesktopAppsEnabled(false);
    QVERIFY(desktop->isHidden());
    QVERIFY(!dock.findChild<QToolButton *>("dashboardDockApps")->isHidden());
    desktop->click();
    QVERIFY(add.isEmpty());
    dock.setDesktopAppsEnabled(true);
    QVERIFY(!desktop->isHidden());
    desktop->click();
    QCOMPARE(add.size(), 1);
}

void DashboardDockTest::preferredWidthFitsWithoutScrolling()
{
    QWidget dashboard;
    auto *layout = new QVBoxLayout(&dashboard);
    auto *dock = new DashboardDock;
    layout->addWidget(dock, 0, Qt::AlignHCenter);
    dashboard.resize(800, 160);
    dashboard.show();
    QTest::qWait(1);
    dock->setPanels({{"browser", "Browser", "browser"}, {"files", "Files", "files"},
                     {"logs", "Logs", "logs"}});
    dashboard.resize(800, 160);
    dashboard.show();
    auto *scroll = dock->findChild<QScrollArea *>("dashboardDockScroll");
    QVERIFY(scroll);
    QTRY_COMPARE(dock->width(), dock->sizeHint().width());
    QTRY_COMPARE(scroll->horizontalScrollBar()->maximum(), 0);
    QTRY_VERIFY(!scroll->horizontalScrollBar()->isVisible());

    dashboard.resize(220, 160);
    QTRY_VERIFY(scroll->horizontalScrollBar()->maximum() > 0);
    dashboard.resize(800, 160);
    QTRY_COMPARE(scroll->horizontalScrollBar()->maximum(), 0);
    QTRY_VERIFY(!scroll->horizontalScrollBar()->isVisible());
}

void DashboardDockTest::boundsRunningPanelsAndScrollsKeyboardFocus()
{
    DashboardDock dock;
    QVector<DashboardDockItem> panels;
    for (int index = 0; index < DashboardStore::maximumPanelsPerDashboard + 4; ++index)
        panels.append({QString::number(index), QString("Panel %1").arg(index), "logs"});
    dock.setPanels(panels);
    int running = 0;
    for (auto *button : dock.findChildren<QToolButton *>()) {
        if (button->property("panelId").isValid())
            ++running;
    }
    QCOMPARE(running, DashboardStore::maximumPanelsPerDashboard);
    QVERIFY(!dock.accessibleDescription().isEmpty());
    dock.resize(220, dock.sizeHint().height());
    dock.show();
    dock.activateWindow();
    auto *scroll = dock.findChild<QScrollArea *>("dashboardDockScroll");
    QVERIFY(scroll);
    QTRY_VERIFY(scroll->horizontalScrollBar()->maximum() > 0);
    QVERIFY(scroll->verticalScrollBarPolicy() == Qt::ScrollBarAlwaysOff);
    auto *last = dock.findChild<QToolButton *>("dashboardDockPanel_15");
    QVERIFY(last);
    last->setFocus();
    QTRY_VERIFY(scroll->horizontalScrollBar()->value() > 0);
    const QPoint position = last->mapTo(scroll->viewport(), QPoint());
    QVERIFY(position.x() >= 0);
    QVERIFY(position.x() + last->width() <= scroll->viewport()->width());
    auto *apps = dock.findChild<QToolButton *>("dashboardDockApps");
    QVERIFY(apps->isVisible());
    QVERIFY(dock.rect().contains(QRect(apps->mapTo(&dock, QPoint()), apps->size())));
    dock.setPanels({});
    QVERIFY(dock.findChild<QToolButton *>("dashboardDockPinned_browser"));
    QVERIFY(!dock.findChild<QToolButton *>("dashboardDockPanel_15"));
}

QTEST_MAIN(DashboardDockTest)
#include "DashboardDockTest.moc"
