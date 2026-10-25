#include "DashboardWidgetFactory.h"
#include "ContextStore.h"
#include "SendInputWidget.h"
#include "SessionManager.h"
#include "TerminalHub.h"
#include "Theme.h"

#include <QApplication>
#include <QComboBox>
#include <QDir>
#include <QJsonArray>
#include <QFrame>
#include <QFile>
#include <QLayout>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPushButton>
#include <QProgressBar>
#include <QScopeGuard>
#include <QStyle>
#include <QScrollBar>
#include <QTextDocument>
#include <QTest>
#include <QTimer>

#include <memory>

class CanvasWidgetPresentationTest final : public QObject {
    Q_OBJECT
private slots:
    void initTestCase();
    void composerKeepsDraftAndExecutionBoundary();
    void resourcePresentationKeepsMissingMetricsHonest();
    void canvasRulesFollowPresentationAndTheme_data();
    void canvasRulesFollowPresentationAndTheme();
};

void CanvasWidgetPresentationTest::initTestCase()
{
    QVERIFY(QFile::exists(":/theme/pipeon.qss"));
    qApp->setStyle("Fusion");
    applyDockpipeLauncherTheme(*qApp);
}

void CanvasWidgetPresentationTest::composerKeepsDraftAndExecutionBoundary()
{
    TerminalHub hub;
    SendInputWidget composer(hub);
    auto *input = composer.findChild<QPlainTextEdit *>("sendInputText");
    auto *send = composer.findChild<QPushButton *>("sendInputButton");
    auto *targets = composer.findChild<QComboBox *>("sendInputTarget");
    QVERIFY(input);
    QVERIFY(send);
    QVERIFY(targets);
    QVERIFY(!send->isEnabled());
    QCOMPARE(targets->count(), 1);
    const qreal originalDocumentMargin = input->document()->documentMargin();
    input->setPlainText("echo retained draft");
    composer.setProperty("dashboardCanvasPresentation", true);
    QVERIFY(composer.canvasPresentationEnabled());
    QVERIFY(send->text().isEmpty());
    QVERIFY(!send->accessibleName().isEmpty());
    QCOMPARE(input->toPlainText(), QString("echo retained draft"));
    composer.resize(640, composer.sizeHint().height());
    composer.show();
    QTest::qWait(20);
    QVERIFY2(composer.minimumSizeHint().height() <= 110, qPrintable(QString::number(composer.minimumSizeHint().height())));
    QVERIFY(input->height() <= 50);
    QCOMPARE(input->document()->documentMargin(), 0.0);
    QVERIFY(!input->verticalScrollBar()->isVisible());
    QCOMPARE(send->size(), QSize(36, 36));
    input->setPlainText("first\nsecond\nthird");
    QVERIFY(input->height() > 50);
    const QString screenshotDirectory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    if (!screenshotDirectory.isEmpty()) {
        QDir().mkpath(screenshotDirectory);
        QVERIFY(composer.grab().save(screenshotDirectory + "/canvas-send-input.png"));
    }
    input->setPlainText(QString(70000, QChar('x')));
    QVERIFY(input->toPlainText().size() <= 65535);
    QVERIFY(!send->isEnabled());
    composer.setProperty("dashboardCanvasPresentation", false);
    QVERIFY(!composer.canvasPresentationEnabled());
    QCOMPARE(send->text(), QString("Send + Enter"));
    QCOMPARE(input->document()->documentMargin(), originalDocumentMargin);
    QVERIFY(input->toPlainText().size() <= 65535);
    // Changing presentation never creates a terminal or sends the retained text.
    QVERIFY(hub.sessionIds().isEmpty());
}

void CanvasWidgetPresentationTest::resourcePresentationKeepsMissingMetricsHonest()
{
    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> resource(factory.create("system"));
    // This presentation fixture owns the measurements. Prevent showing the
    // widget or a polling timer from starting a real collector that would race
    // the explicit sample (and depend on the host's installed CLI).
    resource->removeEventFilter(&factory);
    for (auto *timer : factory.findChildren<QTimer *>())
        timer->stop();
    auto *presentation = resource->findChild<QWidget *>("canvasDashboardView");
    QVERIFY(presentation);
    auto *metricCard = resource->findChild<QFrame *>("systemMetricCard");
    QVERIFY(metricCard);
    const QMargins originalCardMargins = metricCard->layout()->contentsMargins();
    const int originalCardSpacing = metricCard->layout()->spacing();
    const int originalFrameWidth = metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard);
    presentation->setProperty("dashboardCanvasPresentation", true);
    QCOMPARE(metricCard->layout()->contentsMargins(), QMargins(8, 6, 8, 6));
    QCOMPARE(metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard), 0);
    auto *memory = resource->findChild<QLabel *>("systemMetric1");
    auto *swap = resource->findChild<QLabel *>("systemMetric2");
    QVERIFY(memory);
    QVERIFY(swap);
    const QJsonObject system{{"boot_id", "canvas-fixture"}, {"memory_total", 1000},
        {"memory_available", 250}, {"disk_total", 2000}, {"disk_available", 1500}};
    emit factory.snapshotChanged(QJsonObject{{"machine", "local"}, {"system", system},
        {"executions", QJsonArray{}}, {"captured_at", "2026-10-19T12:00:00.000Z"}}, {});
    QVERIFY(memory->text().contains("750"));
    QCOMPARE(swap->text(), QString("Unavailable"));
    const QString original = memory->text();
    resource->resize(480, 380);
    resource->show();
    const QString screenshotDirectory = qEnvironmentVariable("DASHBOARD_PAGE_SCREENSHOT_DIR");
    const QPalette originalPalette = resource->palette();
    for (const bool dark : {true, false}) {
        QPalette palette = originalPalette;
        palette.setColor(QPalette::Window, dark ? QColor("#151f2b") : QColor("#edf2f7"));
        palette.setColor(QPalette::Base, dark ? QColor("#202e3e") : QColor("#ffffff"));
        palette.setColor(QPalette::AlternateBase, dark ? QColor("#293c4c") : QColor("#dce6ef"));
        palette.setColor(QPalette::WindowText, dark ? QColor("#e8eff7") : QColor("#243247"));
        palette.setColor(QPalette::Text, palette.color(QPalette::WindowText));
        palette.setColor(QPalette::Highlight, QColor("#3aa5b8"));
        resource->setPalette(palette);
        QTest::qWait(20);
        QCOMPARE(memory->text(), original);
        QCOMPARE(swap->text(), QString("Unavailable"));
        if (!screenshotDirectory.isEmpty()) {
            QDir().mkpath(screenshotDirectory);
            const QString suffix = dark ? "dark" : "light";
            QVERIFY(resource->grab().save(screenshotDirectory + "/canvas-resources-" + suffix + ".png"));
        }
    }
    presentation->setProperty("dashboardCanvasPresentation", false);
    QCOMPARE(memory->text(), original);
    QCOMPARE(metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard), originalFrameWidth);
    QCOMPARE(metricCard->layout()->contentsMargins(), originalCardMargins);
    QCOMPARE(metricCard->layout()->spacing(), originalCardSpacing);
    emit factory.snapshotChanged({}, "Collector unavailable");
    QCOMPARE(memory->text(), QString("Unavailable"));
}

void CanvasWidgetPresentationTest::canvasRulesFollowPresentationAndTheme_data()
{
    QTest::addColumn<bool>("light");
    QTest::newRow("dark-to-light") << false;
    QTest::newRow("light-to-dark") << true;
}

void CanvasWidgetPresentationTest::canvasRulesFollowPresentationAndTheme()
{
    QFETCH(bool, light);
    const QString originalStyleSheet = qApp->styleSheet();
    const QPalette originalPalette = qApp->palette();
    const auto restoreTheme = qScopeGuard([&]() {
        qApp->setPalette(originalPalette);
        qApp->setStyleSheet(originalStyleSheet);
    });
    auto applyTheme = [](bool useLight) {
        QFile common(":/theme/pipeon.qss");
        QFile companion(useLight ? ":/theme/pipeon-light.qss" : ":/theme/pipeon-dark.qss");
        if (!common.open(QIODevice::ReadOnly) || !companion.open(QIODevice::ReadOnly))
            return false;
        QPalette palette = qApp->palette();
        palette.setColor(QPalette::Window, QColor(useLight ? "#edf2f7" : "#151f2b"));
        palette.setColor(QPalette::Base, QColor(useLight ? "#ffffff" : "#202e3e"));
        palette.setColor(QPalette::AlternateBase, QColor(useLight ? "#dce6ef" : "#293c4c"));
        palette.setColor(QPalette::Highlight, QColor(useLight ? "#1a6172" : "#3aa5b8"));
        qApp->setPalette(palette);
        qApp->setStyleSheet(QString::fromUtf8(common.readAll() + companion.readAll()));
        return true;
    };
    QVERIFY(applyTheme(light));

    SessionManager sessions;
    ContextStore store;
    DashboardWidgetFactory factory(sessions, store);
    std::unique_ptr<QWidget> resource(factory.create("system"));
    resource->removeEventFilter(&factory);
    for (auto *timer : factory.findChildren<QTimer *>())
        timer->stop();
    auto *presentation = resource->findChild<QWidget *>("canvasDashboardView");
    auto *metricCard = resource->findChild<QFrame *>("systemMetricCard");
    auto *title = resource->findChild<QLabel *>("systemMetricTitle");
    auto *progress = resource->findChild<QProgressBar *>();
    QVERIFY(presentation);
    QVERIFY(metricCard);
    QVERIFY(title);
    QVERIFY(progress);
    resource->ensurePolished();
    metricCard->ensurePolished();
    progress->ensurePolished();
    progress->setMaximumHeight(13);
    const int originalProgressMaximum = progress->maximumHeight();
    const int originalFrameWidth = metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard);
    QFrame unrelated;
    unrelated.setObjectName("systemMetricCard");
    unrelated.ensurePolished();
    const int unrelatedFrameWidth = unrelated.style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, &unrelated);

    // Direct presentation changes must work without a workspace or reparent.
    presentation->setProperty("dashboardCanvasPresentation", true);
    QCOMPARE(metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard), 0);
    QCOMPARE(progress->maximumHeight(), 5);
    QCOMPARE(title->font().pixelSize(), 11);
    QCOMPARE(title->palette().color(QPalette::WindowText), resource->palette().color(QPalette::Highlight));
    QCOMPARE(unrelated.style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, &unrelated), unrelatedFrameWidth);

    // New descendants and a live theme reload inherit the same selector.
    auto *lateProgress = new QProgressBar(presentation);
    lateProgress->ensurePolished();
    QCOMPARE(lateProgress->maximumHeight(), 5);
    QVERIFY(applyTheme(!light));
    QCOMPARE(metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard), 0);
    QCOMPARE(progress->maximumHeight(), 5);
    QCOMPARE(title->font().pixelSize(), 11);
    QCOMPARE(title->palette().color(QPalette::WindowText), resource->palette().color(QPalette::Highlight));

    presentation->setProperty("dashboardCanvasPresentation", false);
    QCOMPARE(metricCard->style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, metricCard), originalFrameWidth);
    QCOMPARE(progress->maximumHeight(), originalProgressMaximum);
    QVERIFY(lateProgress->maximumHeight() > 5);

    progress->setMaximumHeight(17);
    presentation->setProperty("dashboardCanvasPresentation", true);
    QCOMPARE(progress->maximumHeight(), 5);
    presentation->setProperty("dashboardCanvasPresentation", false);
    QCOMPARE(progress->maximumHeight(), 17);
    QCOMPARE(unrelated.style()->pixelMetric(QStyle::PM_DefaultFrameWidth, nullptr, &unrelated), unrelatedFrameWidth);
}

QTEST_MAIN(CanvasWidgetPresentationTest)
#include "CanvasWidgetPresentationTest.moc"
