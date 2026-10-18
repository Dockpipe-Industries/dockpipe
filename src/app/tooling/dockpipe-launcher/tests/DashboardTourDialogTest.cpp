#include "DashboardTourDialog.h"
#include "DesktopWindowWidget.h"
#include "Theme.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDir>
#include <QPushButton>
#include <QRadioButton>
#include <QSignalSpy>
#include <QStackedWidget>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>

namespace {
template<typename T>
T *control(DashboardTourDialog &dialog, const char *name)
{
    return dialog.findChild<T *>(QString::fromLatin1(name));
}

bool captureIfRequested(DashboardTourDialog &dialog, const QString &name)
{
    const QString directory = qEnvironmentVariable("DASHBOARD_TOUR_SCREENSHOT_DIR");
    if (directory.isEmpty())
        return true;
    if (!QDir().mkpath(directory))
        return false;
    QCoreApplication::processEvents();
    return dialog.grab().save(QDir(directory).filePath(name + ".png"));
}
}

class DashboardTourDialogTest final : public QObject {
    Q_OBJECT
private slots:
    void defaultsAndNavigation();
    void backgroundChoices_data();
    void backgroundChoices();
    void unavailableAcquisitionPreservesSavedChoice();
    void rejection_data();
    void rejection();
};

void DashboardTourDialogTest::defaultsAndNavigation()
{
    const DashboardOptions original;
    DashboardTourDialog dialog(original);
    QSignalSpy accepted(&dialog, &QDialog::accepted);
    QSignalSpy rejected(&dialog, &QDialog::rejected);
    auto *pages = control<QStackedWidget>(dialog, "dashboardTourPages");
    auto *back = control<QPushButton>(dialog, "dashboardTourBack");
    auto *next = control<QPushButton>(dialog, "dashboardTourNext");
    auto *finish = control<QPushButton>(dialog, "dashboardTourFinish");
    auto *tiling = control<QRadioButton>(dialog, "dashboardTourTiling");
    auto *compact = control<QCheckBox>(dialog, "dashboardTourCompact");
    auto *background = control<QComboBox>(dialog, "dashboardTourBackground");
    auto *acquire = control<QCheckBox>(dialog, "dashboardTourAutoAcquire");
    auto *doNotShow = control<QCheckBox>(dialog, "dashboardTourDoNotShowAgain");
    QVERIFY(pages && back && next && finish && tiling && compact && background && acquire && doNotShow);
    dialog.show();
    QCoreApplication::processEvents();

    QCOMPARE(pages->count(), 3);
    QCOMPARE(pages->currentIndex(), 0);
    QCOMPARE(dialog.initialEnvironment(), QString("canvas"));
    QVERIFY(!back->isEnabled());
    QVERIFY(next->isVisible());
    QVERIFY(!finish->isVisible());
    QVERIFY(compact->isChecked());
    QVERIFY(!acquire->isChecked());
    QVERIFY(!dialog.doNotShowAgain());
    QCOMPARE(background->currentIndex(), 0);
    QVERIFY(dialog.findChildren<QTimer *>().isEmpty());
    QVERIFY(captureIfRequested(dialog, "page-1"));

    tiling->click();
    // Enter advances the current page; it must not accept the tour early.
    QTest::keyClick(next, Qt::Key_Return);
    QCOMPARE(pages->currentIndex(), 1);
    QCOMPARE(accepted.count(), 0);
    QVERIFY(back->isEnabled());
    QVERIFY(captureIfRequested(dialog, "page-2"));
    compact->setChecked(false);
    background->setCurrentIndex(background->findData("ocean"));
    back->click();
    QCOMPARE(pages->currentIndex(), 0);
    QCOMPARE(dialog.initialEnvironment(), QString("tiling"));
    next->click();
    QVERIFY(!compact->isChecked());
    QCOMPARE(background->currentData().toString(), QString("ocean"));
    next->click();
    QCOMPARE(pages->currentIndex(), 2);
    QVERIFY(!next->isVisible());
    QVERIFY(finish->isVisible());
    QCOMPARE(accepted.count(), 0);
    QVERIFY(captureIfRequested(dialog, "page-3"));
    if (acquire->isEnabled())
        acquire->click();
    const bool chosenAcquisition = acquire->isChecked();
    doNotShow->click();
    back->click();
    next->click();
    QCOMPARE(acquire->isChecked(), chosenAcquisition);
    QVERIFY(dialog.doNotShowAgain());
    finish->click();
    QCOMPARE(dialog.result(), int(QDialog::Accepted));
    QCOMPARE(accepted.count(), 1);
    QCOMPARE(rejected.count(), 0);
    const DashboardOptions selected = dialog.selectedOptions();
    QVERIFY(!selected.compactTiling);
    QCOMPARE(selected.autoAcquireApps, chosenAcquisition);
    QCOMPARE(selected.canvasBackground, QString("ocean"));
    QCOMPARE(selected.showPanelHeaders, original.showPanelHeaders);
    QVERIFY(original.compactTiling);
    QVERIFY(!original.autoAcquireApps);
    QCOMPARE(original.canvasBackground, QString("grid"));
}

void DashboardTourDialogTest::backgroundChoices_data()
{
    QTest::addColumn<QString>("choice");
    QTest::addColumn<QString>("expected");
    QTest::newRow("keep-current") << QString() << QString("image");
    QTest::newRow("grid") << QString("grid") << QString("grid");
    QTest::newRow("desktop") << QString("desktop") << QString("desktop");
    QTest::newRow("dusk") << QString("dusk") << QString("dusk");
    QTest::newRow("ocean") << QString("ocean") << QString("ocean");
}

void DashboardTourDialogTest::backgroundChoices()
{
    QFETCH(QString, choice);
    QFETCH(QString, expected);
    DashboardOptions original;
    original.canvasBackground = "image";
    original.canvasImagePath = "/fixture/current-background.png";
    original.showPanelHeaders = false;
    DashboardTourDialog dialog(original);
    auto *background = control<QComboBox>(dialog, "dashboardTourBackground");
    QVERIFY(background);
    const int index = background->findData(choice);
    QVERIFY(index >= 0);
    background->setCurrentIndex(index);
    const DashboardOptions selected = dialog.selectedOptions();
    QCOMPARE(selected.canvasBackground, expected);
    QCOMPARE(selected.canvasImagePath, choice.isEmpty() ? original.canvasImagePath : QString());
    QVERIFY(!selected.showPanelHeaders);
    QCOMPARE(original.canvasBackground, QString("image"));
    QCOMPARE(original.canvasImagePath, QString("/fixture/current-background.png"));
}

void DashboardTourDialogTest::unavailableAcquisitionPreservesSavedChoice()
{
    DashboardOptions original;
    original.autoAcquireApps = true;
    original.compactTiling = false;
    DashboardTourDialog dialog(original);
    auto *acquire = control<QCheckBox>(dialog, "dashboardTourAutoAcquire");
    QVERIFY(acquire);
    QCOMPARE(acquire->isEnabled(), DesktopWindowWidget::automaticAcquisitionUnavailableReason().isEmpty());
    QVERIFY(acquire->isChecked());
    QVERIFY(dialog.selectedOptions().autoAcquireApps);
    QVERIFY(!dialog.selectedOptions().compactTiling);
    // Inspecting the tour never creates a native acquisition widget.
    QVERIFY(dialog.findChildren<DesktopWindowWidget *>().isEmpty());
}

void DashboardTourDialogTest::rejection_data()
{
    QTest::addColumn<QString>("action");
    QTest::newRow("dismiss") << QString("dismiss");
    QTest::newRow("escape") << QString("escape");
    QTest::newRow("window-close") << QString("close");
}

void DashboardTourDialogTest::rejection()
{
    QFETCH(QString, action);
    const DashboardOptions original;
    DashboardTourDialog dialog(original);
    QSignalSpy accepted(&dialog, &QDialog::accepted);
    QSignalSpy rejected(&dialog, &QDialog::rejected);
    auto *next = control<QPushButton>(dialog, "dashboardTourNext");
    auto *compact = control<QCheckBox>(dialog, "dashboardTourCompact");
    auto *doNotShow = control<QCheckBox>(dialog, "dashboardTourDoNotShowAgain");
    auto *dismiss = control<QPushButton>(dialog, "dashboardTourDismiss");
    QVERIFY(next && compact && doNotShow && dismiss);
    dialog.show();
    next->click();
    compact->setChecked(false);
    doNotShow->setChecked(true);
    if (action == "dismiss")
        dismiss->click();
    else if (action == "escape")
        QTest::keyClick(&dialog, Qt::Key_Escape);
    else
        dialog.close();
    QCOMPARE(dialog.result(), int(QDialog::Rejected));
    QCOMPARE(accepted.count(), 0);
    QCOMPARE(rejected.count(), 1);
    QVERIFY(dialog.doNotShowAgain());
    QVERIFY(original.compactTiling);
    QVERIFY(!original.autoAcquireApps);
}

int main(int argc, char **argv)
{
    QTemporaryDir config;
    if (!config.isValid())
        return 1;
    qputenv("XDG_CONFIG_HOME", config.path().toUtf8());
    qputenv("APPDATA", config.path().toUtf8());
    QApplication application(argc, argv);
    application.setOrganizationName("DockpipeDashboardTourTest");
    application.setApplicationName("DashboardTourFixture");
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    if (application.styleSheet().isEmpty())
        return 1;
    DashboardTourDialogTest test;
    return QTest::qExec(&test, argc, argv);
}

#include "DashboardTourDialogTest.moc"
