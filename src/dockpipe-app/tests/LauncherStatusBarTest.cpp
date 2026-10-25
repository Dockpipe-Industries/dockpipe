#include "LauncherStatusBar.h"

#include <QLabel>
#include <QMainWindow>
#include <QStackedWidget>
#include <QTest>

class LauncherStatusBarTest final : public QObject {
    Q_OBJECT
private slots:
    void idlePagesReachWindowBottom();
    void messagesAppearAndExpiredMessagesReleaseSpace();
    void noticeRemainsUntilDismissed();
};

void LauncherStatusBarTest::idlePagesReachWindowBottom()
{
    QMainWindow window;
    auto *pages = new QStackedWidget;
    pages->addWidget(new QLabel("Dashboard"));
    pages->addWidget(new QLabel("Files"));
    pages->addWidget(new QLabel("Settings"));
    window.setCentralWidget(pages);
    auto *bar = new LauncherStatusBar;
    window.setStatusBar(bar);
    window.resize(800, 600);
    window.show();
    QVERIFY(!bar->isSizeGripEnabled());
    for (int page = 0; page < pages->count(); ++page) {
        pages->setCurrentIndex(page);
        QTRY_VERIFY(bar->isHidden());
        QTRY_COMPARE(pages->mapTo(&window, QPoint(0, pages->height())).y(), window.height());
    }
}

void LauncherStatusBarTest::messagesAppearAndExpiredMessagesReleaseSpace()
{
    QMainWindow window;
    auto *content = new QLabel("Files");
    window.setCentralWidget(content);
    auto *bar = new LauncherStatusBar;
    window.setStatusBar(bar);
    window.resize(800, 600);
    window.show();
    bar->showMessage("A real operation failed", 500);
    QTRY_VERIFY(bar->isVisible());
    QTRY_VERIFY(content->height() < window.height());
    QTRY_VERIFY(bar->isHidden());
    QTRY_COMPARE(content->height(), window.height());
    bar->showMessage("Another message");
    QVERIFY(bar->isVisible());
    bar->clearMessage();
    QTRY_VERIFY(bar->isHidden());
}

void LauncherStatusBarTest::noticeRemainsUntilDismissed()
{
    QMainWindow window;
    window.setCentralWidget(new QLabel("Files"));
    auto *bar = new LauncherStatusBar;
    window.setStatusBar(bar);
    auto *notice = new QLabel("An undismissed notice");
    bar->addWidget(notice);
    bar->setNoticeVisible(true);
    window.show();
    QVERIFY(bar->isVisible());
    bar->showMessage("Temporary operation message");
    bar->clearMessage();
    QVERIFY(bar->isVisible());
    bar->removeWidget(notice);
    notice->deleteLater();
    bar->setNoticeVisible(false);
    QTRY_VERIFY(bar->isHidden());
    bar->showMessage("Still useful after notice dismissal");
    QVERIFY(bar->isVisible());
    bar->clearMessage();
    QTRY_VERIFY(bar->isHidden());
}

QTEST_MAIN(LauncherStatusBarTest)
#include "LauncherStatusBarTest.moc"
