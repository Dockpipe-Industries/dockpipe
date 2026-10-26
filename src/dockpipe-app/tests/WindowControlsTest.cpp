#include "Theme.h"

#include <QApplication>
#include <QDialog>
#include <QDir>
#include <QLabel>
#include <QMainWindow>
#include <QMenuBar>
#include <QTemporaryDir>
#include <QTest>
#include <QToolButton>
#include <QVBoxLayout>

class WindowControlsTest final : public QObject {
    Q_OBJECT
private slots:
    void mainWindowActions();
    void dialogCloseRejects();
};

void WindowControlsTest::mainWindowActions()
{
#ifdef Q_OS_MACOS
    QSKIP("macOS main windows retain their native system controls.");
#else
    QMainWindow window;
    window.setWindowTitle("DockPipe window controls");
    window.menuBar()->addMenu("Settings");
    window.menuBar()->addMenu("Packages");
    window.menuBar()->addMenu("Help");
    window.setCentralWidget(new QLabel("DockPipe · Standard window controls"));
    window.resize(720, 260);
    window.show();
    QCoreApplication::processEvents();
    auto *close = window.findChild<QToolButton *>("windowClose");
    QToolButton *minimize = nullptr;
    QToolButton *expand = nullptr;
    const auto windowControls = window.findChildren<QToolButton *>("windowControl");
    QCOMPARE(windowControls.size(), 2);
    for (auto *button : windowControls) {
        if (button->accessibleName() == "Minimize")
            minimize = button;
        else if (button->accessibleName() == "Maximize or restore")
            expand = button;
    }
    QVERIFY(close && minimize && expand);
    QVERIFY(!close->accessibleName().isEmpty());
    QVERIFY(!minimize->accessibleName().isEmpty());
    QVERIFY(!expand->accessibleName().isEmpty());
    for (auto *button : {close, minimize, expand}) {
        QCOMPARE(button->size(), QSize(36, 32));
        QVERIFY(!button->property("roundWindowControl").toBool());
    }
    QVERIFY(minimize->mapTo(&window, QPoint()).x() < expand->mapTo(&window, QPoint()).x());
    QVERIFY(expand->mapTo(&window, QPoint()).x() < close->mapTo(&window, QPoint()).x());
    QVERIFY(!window.menuBar()->cornerWidget(Qt::TopLeftCorner));
    auto *corner = window.menuBar()->cornerWidget(Qt::TopRightCorner);
    QVERIFY(corner);
    const QRect controls = corner->geometry();
    QVERIFY(controls.center().x() > window.menuBar()->width() / 2);
    qInfo() << "Control and menu bounds" << controls
            << window.menuBar()->actionGeometry(window.menuBar()->actions().first());
    const QString directory = qEnvironmentVariable("DOCKPIPE_CONTROLS_SCREENSHOT_DIR");
    if (!directory.isEmpty()) {
        QVERIFY(QDir().mkpath(directory));
        QVERIFY(window.grab().save(directory + "/window.png"));
        close->setFocus();
        QVERIFY(close->grab().save(directory + "/close-focused.png"));
    }
    QVERIFY(!controls.intersects(window.menuBar()->actionGeometry(window.menuBar()->actions().first())));
    QTest::mouseClick(expand, Qt::LeftButton);
    QTRY_VERIFY(window.isMaximized());
    QTest::mouseClick(expand, Qt::LeftButton);
    QTRY_VERIFY(!window.isMaximized());
    QTest::mouseClick(minimize, Qt::LeftButton);
    QTRY_VERIFY(window.isMinimized());
    window.showNormal();
    QTRY_VERIFY(!window.isMinimized());
    QTest::mouseClick(close, Qt::LeftButton);
    QTRY_VERIFY(!window.isVisible());
#endif
}

void WindowControlsTest::dialogCloseRejects()
{
    QDialog dialog;
    dialog.setWindowTitle("Dashboard settings");
    auto *layout = new QVBoxLayout(&dialog);
    layout->addWidget(new QLabel("The close control keeps the dialog's Cancel behavior."));
    dialog.resize(500, 210);
    dialog.show();
    QCoreApplication::processEvents();
    auto *close = dialog.findChild<QToolButton *>("windowClose");
    QVERIFY(close);
    QCOMPARE(close->size(), QSize(36, 32));
    QVERIFY(!close->property("roundWindowControl").toBool());
    auto *heading = dialog.findChild<QLabel *>("dialogHeading");
    QVERIFY(heading);
    const int closeLeft = close->mapTo(&dialog, QPoint()).x();
    const int headingRight = heading->mapTo(&dialog, QPoint(heading->width(), 0)).x();
    QVERIFY(closeLeft >= headingRight);
    const QString directory = qEnvironmentVariable("DOCKPIPE_CONTROLS_SCREENSHOT_DIR");
    if (!directory.isEmpty()) {
        QVERIFY(QDir().mkpath(directory));
        QVERIFY(dialog.grab().save(directory + "/dialog.png"));
    }
    close->setFocus();
    QTest::keyClick(close, Qt::Key_Space);
    QTRY_VERIFY(!dialog.isVisible());
    QCOMPARE(dialog.result(), int(QDialog::Rejected));
}

int main(int argc, char **argv)
{
    QTemporaryDir config;
    if (!config.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    QApplication application(argc, argv);
    application.setOrganizationName("DockpipeControlsTest");
    application.setApplicationName("ControlsFixture");
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    WindowControlsTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "WindowControlsTest.moc"
