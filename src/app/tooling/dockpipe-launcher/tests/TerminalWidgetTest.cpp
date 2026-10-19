#include "TerminalHub.h"
#include "TerminalPreferences.h"
#include "TerminalWidget.h"
#include "SendInputWidget.h"
#include "DashboardWorkspace.h"
#include "Theme.h"

#include <QApplication>
#include <QAction>
#include <QComboBox>
#include <QElapsedTimer>
#include <QDialog>
#include <QEventLoop>
#include <QProcess>
#include <QFile>
#include <QLabel>
#include <QPlainTextEdit>
#include <QPointer>
#include <QPushButton>
#include <QSignalSpy>
#include <QScreen>
#include <QPainter>
#include <QVBoxLayout>
#include <QTemporaryDir>
#include <QTest>
#include <QTimer>
#include <QWebEnginePage>
#include <QWebEngineProfile>
#include <QWebEngineView>
#include <ctime>
#include <memory>

// A composited host with an opaque backdrop, like the launcher main window.
class TerminalTestHost final : public QWidget {
protected:
    void paintEvent(QPaintEvent *) override
    {
        QPainter painter(this);
        painter.fillRect(rect(), palette().color(QPalette::Window));
    }
};

class TerminalWidgetTest final : public QObject {
    Q_OBJECT
private slots:
    void terminalAndComposer();
    void compactPanelOwnsOnlyItsSession();
    void canvasTransparencyAndStacking();
};

void TerminalWidgetTest::canvasTransparencyAndStacking()
{
    if (QGuiApplication::platformName() != "xcb")
        QSKIP("Compositor pixel checks require the native X11 test session.");
    TerminalSettings settings;
    settings.theme = "amber";
    settings.opacity = 60;
    settings.backgroundTarget = "canvas";
    QVERIFY(TerminalPreferences::instance()->save(settings));
    QTemporaryDir directory;
    TerminalHub hub;
    DashboardWorkspace workspace("terminal-stacking-fixture");
    workspace.resize(900, 650);
    auto *underlay = new QWidget(&workspace);
    QPalette underlayPalette = underlay->palette();
    underlayPalette.setColor(QPalette::Window, QColor(200, 60, 120));
    underlay->setPalette(underlayPalette);
    underlay->setAutoFillBackground(true);
    underlay->setGeometry(workspace.rect());
    underlay->show();
    auto *terminal = new TerminalWidget(hub, directory.path(), nullptr, TerminalWidget::Mode::Dashboard);
    QVERIFY(workspace.addPanel("terminal", "Terminal", terminal));
    workspace.show();
    workspace.raise();
    workspace.activateWindow();
    QTRY_VERIFY_WITH_TIMEOUT(terminal->property("terminalRendererReady").toBool(), 10000);
    auto *view = terminal->findChild<QWebEngineView *>("terminalDisplay");
    QVERIFY(view);

    const QPoint sample(40, view->height() - 40);
    const auto screenPixel = [&]() {
        const QPoint global = view->mapToGlobal(sample);
        return workspace.screen()->grabWindow(0, global.x(), global.y(), 1, 1).toImage().pixelColor(0, 0);
    };
    const auto near = [](const QColor &actual, const QColor &expected) {
        return qAbs(actual.red() - expected.red()) <= 4
            && qAbs(actual.green() - expected.green()) <= 4
            && qAbs(actual.blue() - expected.blue()) <= 4;
    };
    view->setFocus();
    QTest::qWait(300);
    qInfo() << "Focused terminal pixel" << screenPixel();
    QVERIFY2(near(screenPixel(), QColor(99, 38, 55)), "Focused terminal must blend with the canvas underlay");
    auto *overlay = new QPushButton("Foreground panel", &workspace);
    overlay->setStyleSheet("background: rgb(40, 180, 220); border: none;");
    overlay->setGeometry(QRect(view->mapTo(&workspace, sample) - QPoint(20, 20), QSize(150, 100)));
    overlay->show();
    overlay->raise();
    overlay->setFocus();
    QTest::qWait(300);
    qInfo() << "Foreground panel pixel" << screenPixel();
    QVERIFY2(near(screenPixel(), QColor(40, 180, 220)), "Foreground widgets must cover the terminal renderer");
    overlay->hide();
    view->setFocus();
    QTest::qWait(300);
    QVERIFY2(near(screenPixel(), QColor(99, 38, 55)), "Returning focus must retain terminal opacity");
    workspace.removePanel("terminal");
    QTRY_VERIFY_WITH_TIMEOUT(QCoreApplication::instance()->findChildren<QProcess *>("terminalShell").isEmpty(), 2000);
    QVERIFY(TerminalPreferences::instance()->save(TerminalSettings()));
}

void TerminalWidgetTest::compactPanelOwnsOnlyItsSession()
{
#ifndef Q_OS_UNIX
    QSKIP("Native terminal fixture requires the Unix PTY backend.");
#else
    QVERIFY(TerminalPreferences::instance()->save(TerminalSettings()));
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalHub hub;
    const QString separate = hub.createSession(directory.path());
    QTRY_VERIFY(hub.running(separate));
    DashboardWorkspace workspace("compact-terminal-fixture");
    workspace.resize(800, 500);
    auto *terminal = new TerminalWidget(hub, directory.path(), nullptr, TerminalWidget::Mode::Dashboard);
    QVERIFY(workspace.addPanel("compact-terminal", "Terminal", terminal));
    QCOMPARE(hub.sessionIds(), QStringList{separate});
    QVERIFY(!terminal->findChild<QWebEngineView *>());
    workspace.show();
    QTRY_COMPARE(hub.sessionIds().size(), 2);
    const QString owned = terminal->sessionId();
    QVERIFY(!owned.isEmpty() && owned != separate);
    QTRY_VERIFY(hub.running(owned));
    QTRY_VERIFY_WITH_TIMEOUT(terminal->property("terminalRendererReady").toBool(), 10000);
    auto *view = terminal->findChild<QWebEngineView *>("terminalDisplay");
    QVERIFY(view);
    QPointer<QWebEnginePage> page = view->page();
    QPointer<QWebEngineProfile> profile = view->page()->profile();
    QVERIFY(!terminal->findChild<QWidget *>("terminalToolbar")->isVisible());
    QVERIFY(!terminal->findChild<QLabel *>("terminalStatus")->isVisible());
    QVERIFY(!terminal->findChild<QPushButton *>("terminalNew")->isVisible());
    QVERIFY(!terminal->findChild<QPushButton *>("terminalClose")->isVisible());
    QCOMPARE(view->contextMenuPolicy(), Qt::ActionsContextMenu);
    QVERIFY(terminal->findChild<QAction *>("terminalSettingsAction"));
    QSignalSpy open(terminal, &TerminalWidget::openFullViewRequested);
    terminal->findChild<QAction *>("terminalOpenFullViewAction")->trigger();
    QCOMPARE(open.size(), 1);
    for (int iteration = 0; iteration < 5; ++iteration) {
        workspace.hide();
        workspace.setMode("tiling");
        workspace.show();
        workspace.setMode("canvas");
        QApplication::processEvents();
    }
    QCOMPARE(terminal->sessionId(), owned);
    QCOMPARE(view->page(), page.data());
    QCOMPARE(hub.sessionIds().size(), 2);
    {
        TerminalWidget managed(hub, directory.path());
        auto *selector = managed.findChild<QComboBox *>("terminalSessions");
        selector->setCurrentIndex(selector->findData(owned));
        QCOMPARE(managed.sessionId(), owned);
    }
    QVERIFY(hub.running(owned));
    workspace.grab().save("/tmp/dockpipe-terminal-compact.png");
    workspace.removePanel("compact-terminal");
    QCOMPARE(hub.sessionIds(), QStringList{separate});
    QVERIFY(hub.running(separate));
    QTRY_VERIFY(page.isNull());
    QTRY_VERIFY(profile.isNull());

    workspace.hide();
    auto *hidden = new TerminalWidget(hub, directory.path(), nullptr, TerminalWidget::Mode::Dashboard);
    QVERIFY(workspace.addPanel("hidden-terminal", "Terminal", hidden));
    workspace.setMode("tiling");
    QCOMPARE(hub.sessionIds(), QStringList{separate});
    workspace.removePanel("hidden-terminal");
    QCOMPARE(hub.sessionIds(), QStringList{separate});

    auto *closed = new TerminalWidget(hub, directory.path(), nullptr, TerminalWidget::Mode::Dashboard);
    QVERIFY(workspace.addPanel("closed-terminal", "Terminal", closed));
    workspace.show();
    QTRY_COMPARE(hub.sessionIds().size(), 2);
    const QString closedId = closed->sessionId();
    hub.closeSession(closedId);
    workspace.hide();
    workspace.show();
    QCOMPARE(hub.sessionIds(), QStringList{separate});
    QVERIFY(closed->sessionId().isEmpty());
    QVERIFY(closed->findChild<QLabel *>("terminalStatus")->isVisible());
    workspace.removePanel("closed-terminal");
    QVERIFY(hub.running(separate));
    hub.closeSession(separate);
    QTRY_VERIFY_WITH_TIMEOUT(QCoreApplication::instance()->findChildren<QProcess *>("terminalShell").isEmpty(), 2000);
#endif
}

void TerminalWidgetTest::terminalAndComposer()
{
#ifndef Q_OS_UNIX
    QSKIP("Native terminal fixture requires the Unix PTY backend.");
#else
    QTemporaryDir directory;
    QVERIFY(directory.isValid());
    TerminalHub hub;
    QWidget desktopUnderlay;
    desktopUnderlay.setWindowFlags(Qt::Tool | Qt::FramelessWindowHint | Qt::BypassWindowManagerHint);
    desktopUnderlay.setAutoFillBackground(true);
    QPalette desktopPalette = desktopUnderlay.palette();
    desktopPalette.setColor(QPalette::Window, QColor(40, 180, 220));
    desktopUnderlay.setPalette(desktopPalette);
    desktopUnderlay.setGeometry(60, 60, 1000, 700);
    desktopUnderlay.show();
    TerminalTestHost host;
    host.setAttribute(Qt::WA_TranslucentBackground);
    host.setWindowFlags(Qt::FramelessWindowHint | Qt::BypassWindowManagerHint);
    host.setGeometry(60, 60, 1000, 700);
    DashboardWorkspace workspace("terminal-fixture", &host);
    auto *hostLayout = new QVBoxLayout(&host);
    hostLayout->setContentsMargins(0, 0, 0, 0);
    hostLayout->addWidget(&workspace);
    auto *terminal = new TerminalWidget(hub, directory.path());
    QVERIFY(workspace.addPanel("terminal-test", "Terminal", terminal));
    host.show();
    QCOMPARE(hub.sessionIds().size(), 0);
    QVERIFY(!terminal->findChild<QWebEngineView *>());
    terminal->findChild<QPushButton *>("terminalNew")->click();
    QCOMPARE(hub.sessionIds().size(), 1);
    QVERIFY(!terminal->isWindow());
    QCOMPARE(terminal->window(), &host);
    QVERIFY(!workspace.isFullScreen());
    const QString first = terminal->sessionId();
    QTRY_VERIFY_WITH_TIMEOUT(hub.running(first), 3000);
    QTRY_VERIFY_WITH_TIMEOUT(terminal->property("terminalRendererReady").toBool(), 10000);
    auto *view = terminal->findChild<QWebEngineView *>();
    QVERIFY(view);
    QPointer<QWebEnginePage> retainedPage = view->page();
    QPointer<QWebEngineProfile> retainedProfile = view->page()->profile();
    QVERIFY(retainedProfile->isOffTheRecord());
    QTest::qWait(150);
    // Disable echo before asserting output, so command text cannot satisfy the fixture.
    QVERIFY(hub.send(first, "stty -echo"));
    QVERIFY(hub.send(first, "printf '\\nRENDERED_%s\\n' terminal"));
    QString rendered;
    const auto readScreen = [&]() {
        auto resultState = std::make_shared<QPair<bool, QString>>();
        retainedPage->runJavaScript(
            "Array.from({length:dockpipeTerminal.buffer.active.length},(_,i)=>dockpipeTerminal.buffer.active.getLine(i).translateToString()).join('\\n')",
            [resultState](const QVariant &result) { resultState->second = result.toString(); resultState->first = true; });
        QElapsedTimer deadline;
        deadline.start();
        while (!resultState->first && deadline.elapsed() < 2000)
            QTest::qWait(10);
        rendered = resultState->second;
        return resultState->first;
    };
    QTRY_VERIFY_WITH_TIMEOUT(readScreen() && rendered.contains("RENDERED_terminal"), 5000);

    // Exercise actual key events through xterm and WebChannel, not just the hub.
    view->setFocus();
    QWidget *keyboardTarget = view->focusProxy() ? view->focusProxy() : view;
    QTest::mouseClick(keyboardTarget, Qt::LeftButton, {}, QPoint(12, 12));
    QTest::keyClicks(keyboardTarget, "printf bridge > bridge-input.txt");
    QTest::keyClick(keyboardTarget, Qt::Key_Return);
    QTRY_VERIFY_WITH_TIMEOUT(QFile::exists(directory.filePath("bridge-input.txt")), 3000);

    QCOMPARE(TerminalPreferences::instance()->settings().opacity, 100);
    // Software rendering must allocate the alpha surface before opacity changes.
    // The terminal DOM provides the opaque background at the default setting.
    QCOMPARE(view->page()->backgroundColor().alpha(), 0);
    const auto scriptValue = [&](const QString &script) {
        auto result = std::make_shared<QPair<bool, QVariant>>();
        retainedPage->runJavaScript(script, [result](const QVariant &value) {
            result->first = true;
            result->second = value;
        });
        QElapsedTimer deadline;
        deadline.start();
        while (!result->first && deadline.elapsed() < 2000)
            QTest::qWait(10);
        return result->second;
    };
    TerminalSettings appearance = TerminalPreferences::instance()->settings();
    appearance.theme = "amber";
    appearance.fontSize = 15;
    appearance.cursorStyle = "bar";
    appearance.scanlines = true;
    appearance.glow = true;
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QTRY_COMPARE_WITH_TIMEOUT(scriptValue("dockpipeTerminal.options.fontSize").toInt(), 15, 3000);
    QCOMPARE(scriptValue("dockpipeTerminal.options.theme.foreground").toString(), QString("#ffbf69"));
    QCOMPARE(scriptValue("dockpipeTerminal.options.cursorStyle").toString(), QString("bar"));
    QVERIFY(scriptValue("document.body.classList.contains('scanlines') && document.body.classList.contains('glow')").toBool());
    QCOMPARE(view->page(), retainedPage.data());
    QCOMPARE(terminal->sessionId(), first);
    QVERIFY(hub.running(first));
    QTRY_VERIFY(readScreen() && rendered.contains("RENDERED_terminal"));
    appearance.opacity = 60;
    appearance.scanlines = false;
    const QPalette originalPalette = terminal->palette();
    QPalette underlay = originalPalette;
    underlay.setColor(QPalette::Window, QColor(40, 180, 220));
    terminal->setPalette(underlay);
    terminal->setAutoFillBackground(true);
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QCOMPARE(view->page()->backgroundColor().alpha(), 0);
    QTRY_VERIFY(scriptValue("document.getElementById('terminal').style.backgroundColor").toString().contains("0.6"));
    QTest::qWait(200);
    const QImage opacityImage = workspace.grab().toImage();
    const QPoint samplePoint = view->mapTo(&workspace, QPoint(20, view->height() - 25));
    const QColor translucent = opacityImage.pixelColor(samplePoint * opacityImage.devicePixelRatio());
    qInfo() << "Terminal opacity pixel" << translucent;
    workspace.grab().save("/tmp/dockpipe-terminal-opacity.png");
    QVERIFY2(qAbs(translucent.red() - 35) <= 3 && qAbs(translucent.green() - 86) <= 3
        && qAbs(translucent.blue() - 95) <= 3,
        "60% Amber must blend with the contrasting native underlay, not an opaque browser canvas");
    appearance.backgroundTarget = "desktop";
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QTest::qWait(200);
    QImage desktopImage(host.size() * host.devicePixelRatioF(), QImage::Format_ARGB32_Premultiplied);
    desktopImage.setDevicePixelRatio(host.devicePixelRatioF());
    desktopImage.fill(Qt::transparent);
    host.render(&desktopImage);
    const QColor desktopPixel = desktopImage.pixelColor(samplePoint * desktopImage.devicePixelRatio());
    qInfo() << "Terminal desktop pixel" << desktopPixel;
    QVERIFY2(qAbs(desktopPixel.alpha() - 153) <= 3,
        "Desktop mode must preserve 60% alpha through the native window rather than blending into its panel");
    bool opaqueText = false;
    const QRect textArea(view->mapTo(&host, QPoint(8, 8)), QSize(view->width() - 24, 80));
    for (int y = textArea.top(); y < textArea.bottom() && !opaqueText; ++y) {
        for (int x = textArea.left(); x < textArea.right(); ++x) {
            const QColor pixel = desktopImage.pixelColor(QPoint(x, y) * desktopImage.devicePixelRatio());
            if (pixel.alpha() == 255 && qGray(pixel.rgb()) > 100) {
                opaqueText = true;
                break;
            }
        }
    }
    QVERIFY2(opaqueText, "Desktop transparency must not fade the terminal glyphs");
    QCOMPARE(desktopImage.pixelColor(QPoint(5, 5) * desktopImage.devicePixelRatio()).alpha(), 255);
    // X11 can read the compositor output directly. Other platforms need their
    // native capture/permission fixture; an ARGB backing image alone is not proof.
    if (QGuiApplication::platformName() == "xcb") {
        const QPoint globalSample = view->mapToGlobal(QPoint(20, view->height() - 25));
        const QImage composite = workspace.screen()->grabWindow(0, globalSample.x(), globalSample.y(), 1, 1).toImage();
        QVERIFY(!composite.isNull());
        const QColor compositePixel = composite.pixelColor(0, 0);
        qInfo() << "Terminal compositor pixel" << compositePixel;
        QVERIFY2(qAbs(compositePixel.red() - 35) <= 3 && qAbs(compositePixel.green() - 86) <= 3
            && qAbs(compositePixel.blue() - 95) <= 3,
            "Desktop target must visibly composite with the native window behind DockPipe");
    }
    workspace.grab().save("/tmp/dockpipe-terminal-desktop.png");
    terminal->setAutoFillBackground(false);
    terminal->setPalette(originalPalette);
    appearance.backgroundTarget = "canvas";
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    {
        QWidget canvasUnderlay(&workspace);
        QPalette canvasPalette = canvasUnderlay.palette();
        canvasPalette.setColor(QPalette::Window, QColor(200, 60, 120));
        canvasUnderlay.setPalette(canvasPalette);
        canvasUnderlay.setAutoFillBackground(true);
        canvasUnderlay.setGeometry(workspace.rect());
        canvasUnderlay.show();
        canvasUnderlay.lower();
        QTest::qWait(200);
        QVERIFY(terminal->parentWidget()->property("canvasPanelTransparent").toBool());
        const QImage canvasImage = workspace.grab().toImage();
        const QColor canvasPixel = canvasImage.pixelColor(samplePoint * canvasImage.devicePixelRatio());
        qInfo() << "Terminal canvas pixel" << canvasPixel;
        QVERIFY2(qAbs(canvasPixel.red() - 99) <= 3 && qAbs(canvasPixel.green() - 38) <= 3
            && qAbs(canvasPixel.blue() - 55) <= 3,
            "Canvas mode must show a sibling panel behind the terminal instead of its solid frame");
        canvasImage.save("/tmp/dockpipe-terminal-canvas.png");
    }
    appearance.backgroundTarget = "panel";
    appearance.opacity = 100;
    appearance.scanlines = true;
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QCOMPARE(view->page()->backgroundColor().alpha(), 0);
    QTest::qWait(150);
    QTRY_COMPARE(scriptValue("getComputedStyle(document.querySelector('.xterm-viewport')).backgroundColor").toString(),
        QString("rgba(0, 0, 0, 0)"));
    const QColor background = view->grab().toImage().pixelColor(20, view->height() - 25);
    QVERIFY2(background.red() >= 23 && background.red() <= 35, "Terminal viewport must show the Amber background, not a black fallback");
    workspace.grab().save("/tmp/dockpipe-terminal-amber.png");

    // The controls must alter the rendered halo, not just persist new numbers.
    appearance.glowStrength = 0;
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QTRY_COMPARE(scriptValue("getComputedStyle(document.querySelector('.xterm-screen')).filter").toString(), QString("none"));
    QTest::qWait(150);
    const QImage withoutGlow = view->grab().toImage();
    appearance.glowStrength = 100;
    appearance.glowSpread = 8;
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QTRY_VERIFY(scriptValue("getComputedStyle(document.querySelector('.xterm-screen')).filter").toString().contains("8px"));
    QTest::qWait(150);
    const QImage withGlow = view->grab().toImage();
    QCOMPARE(withGlow.size(), withoutGlow.size());
    int brighterPixels = 0;
    for (int y = 0; y < qMin(100, withGlow.height()); ++y) {
        for (int x = 0; x < withGlow.width(); ++x) {
            if (qGray(withGlow.pixel(x, y)) > qGray(withoutGlow.pixel(x, y)) + 5)
                ++brighterPixels;
        }
    }
    QVERIFY2(brighterPixels > 100, "Increasing glow strength and spread must visibly brighten the halo around terminal text");
    workspace.grab().save("/tmp/dockpipe-terminal-glow.png");

    bool settingsCaptured = false;
    QTimer::singleShot(200, &workspace, [&]() {
        auto *dialog = host.findChild<QDialog *>("terminalSettingsDialog");
        if (dialog) {
            settingsCaptured = dialog->grab().save("/tmp/dockpipe-terminal-settings.png");
            dialog->reject();
        }
    });
    terminal->findChild<QPushButton *>("terminalSettings")->click();
    QVERIFY(settingsCaptured);
    QCOMPARE(terminal->sessionId(), first);
    QCOMPARE(hub.sessionIds().size(), 1);

    SendInputWidget composer(hub);
    composer.show();
    auto *targets = composer.findChild<QComboBox *>("sendInputTarget");
    auto *input = composer.findChild<QPlainTextEdit *>("sendInputText");
    auto *send = composer.findChild<QPushButton *>("sendInputButton");
    QVERIFY(!send->isEnabled());
    targets->setCurrentIndex(targets->findData(first));
    QVERIFY(send->isEnabled());
    input->setPlainText("printf selected > selected-target.txt");
    send->click();
    QTRY_VERIFY_WITH_TIMEOUT(QFile::exists(directory.filePath("selected-target.txt")), 3000);
    QVERIFY(input->toPlainText().isEmpty());

    QSignalSpy outputFrames(terminal, &TerminalWidget::renderOutput);
    appearance.backgroundTarget = "desktop";
    appearance.opacity = 60;
    QVERIFY(TerminalPreferences::instance()->save(appearance));
    QElapsedTimer elapsed;
    elapsed.start();
    for (int i = 0; i < 20; ++i) {
        workspace.setMode("tiling");
        QApplication::processEvents();
        workspace.setMode("canvas");
        QApplication::processEvents();
    }
    const qint64 switchDuration = elapsed.elapsed();
    QCOMPARE(terminal->sessionId(), first);
    QCOMPARE(view->page(), retainedPage.data());
    QCOMPARE(hub.sessionIds().size(), 1);
    QVERIFY(switchDuration < 4000);
    QVERIFY(hub.send(first, "printf '\\nAFTER_%s\\n' layout"));
    QTRY_VERIFY_WITH_TIMEOUT(readScreen() && rendered.contains("AFTER_layout"), 5000);

    // WebChannel input queued against the old view generation must not reach a
    // newly selected shell, including terminal-generated query responses.
    QVERIFY(!outputFrames.isEmpty());
    const int oldGeneration = outputFrames.last().at(2).toInt();
    QVERIFY2(hub.running(first), qPrintable(hub.error(first)));
    QCOMPARE(targets->currentData().toString(), first);
    const QString second = hub.createSession(directory.path());
    QVERIFY2(hub.running(first), qPrintable(hub.error(first)));
    auto *sessions = terminal->findChild<QComboBox *>("terminalSessions");
    sessions->setCurrentIndex(sessions->findData(second));
    terminal->terminalInput("printf wrong > stale-input.txt\r", oldGeneration);
    QTest::qWait(150);
    QVERIFY(!QFile::exists(directory.filePath("stale-input.txt")));
    QCOMPARE(targets->currentData().toString(), first);
    hub.closeSession(first);
    QVERIFY(!send->isEnabled());
    input->setPlainText("preserve me");
    QMetaObject::invokeMethod(send, "clicked");
    QCOMPARE(input->toPlainText(), QString("preserve me"));
    QCOMPARE(targets->currentData().toString(), QString());

    // Hidden views keep a bounded native replay window and do not dispatch JS.
    workspace.hide();
    QTest::qWait(100);
    outputFrames.clear();
    QVERIFY(hub.send(second, "stty -echo; yes bounded-output"));
    int timerTicks = 0;
    QTimer heartbeat;
    heartbeat.setInterval(10);
    connect(&heartbeat, &QTimer::timeout, this, [&]() { ++timerTicks; });
    heartbeat.start();
    // CoreFoundation handles timers differently during manual processEvents
    // polling. Measure the application's real event loop with the same deadline.
    QEventLoop floodLoop;
    QTimer floodDeadline;
    floodDeadline.setSingleShot(true);
    floodDeadline.setTimerType(Qt::PreciseTimer);
    connect(&floodDeadline, &QTimer::timeout, &floodLoop, &QEventLoop::quit);
    QElapsedTimer floodElapsed;
    floodElapsed.start();
    floodDeadline.start(350);
    floodLoop.exec();
    qInfo() << "Hidden terminal flood elapsed ms" << floodElapsed.elapsed() << "heartbeat ticks" << timerTicks;
    QVERIFY(timerTicks >= 10);
    QCOMPARE(outputFrames.size(), 0);
    QVERIFY(hub.sendBytes(second, QByteArray(1, '\x03')));
    QTest::qWait(100);
    quint64 cursor = 0;
    bool truncated = false;
    const QByteArray chunk = hub.readOutput(second, cursor, truncated);
    QVERIFY(truncated);
    QVERIFY(chunk.size() <= 32 * 1024);
    workspace.show();
    QTRY_VERIFY_WITH_TIMEOUT(outputFrames.size() > 0, 3000);
    QTest::qWait(1000);
    heartbeat.stop();
    outputFrames.clear();
    const std::clock_t idleStart = std::clock();
    QEventLoop idle;
    QTimer::singleShot(700, &idle, &QEventLoop::quit);
    idle.exec();
    const double idleCpuPercent = 100.0 * double(std::clock() - idleStart) / CLOCKS_PER_SEC / 0.7;
    QCOMPARE(outputFrames.size(), 0);
    qInfo("Terminal: 40 layout switches %lld ms; hidden flood heartbeat %d; idle parent CPU %.2f%%",
        static_cast<long long>(switchDuration), timerTicks, idleCpuPercent);
    QVERIFY(idleCpuPercent < 10.0);
    workspace.grab().save("/tmp/dockpipe-terminal-widget.png");

    // Navigation cannot turn the privileged terminal page into a remote browser.
    view->load(QUrl("https://example.invalid/"));
    QTest::qWait(100);
    QCOMPARE(view->url(), QUrl("qrc:/terminal/index.html"));
    hub.closeSession(second);
    QVERIFY(hub.sessionIds().isEmpty());
    workspace.removePanel("terminal-test");
    QTest::qWait(100);
    QImage closedImage(host.size(), QImage::Format_ARGB32_Premultiplied);
    closedImage.fill(Qt::transparent);
    host.render(&closedImage);
    QCOMPARE(closedImage.pixelColor(samplePoint).alpha(), 255);
    QTRY_VERIFY(retainedPage.isNull());
    QTRY_VERIFY(retainedProfile.isNull());
    QTRY_VERIFY_WITH_TIMEOUT(QCoreApplication::instance()->findChildren<QProcess *>("terminalShell").isEmpty(), 2000);
    // Failed sessions count toward the same finite registry until closed.
    for (int index = 0; index < 8; ++index)
        QVERIFY(!hub.createSession(directory.filePath("missing-directory")).isEmpty());
    QVERIFY(hub.createSession(directory.path()).isEmpty());
    QCOMPARE(hub.sessionIds().size(), 8);
    for (const QString &id : hub.sessionIds())
        hub.closeSession(id);
    QVERIFY(hub.sessionIds().isEmpty());
#endif
}

int main(int argc, char **argv)
{
    if (qEnvironmentVariableIsEmpty("QT_QUICK_BACKEND"))
        qputenv("QT_QUICK_BACKEND", "software");
    QCoreApplication::setAttribute(Qt::AA_ShareOpenGLContexts);
    qputenv("SHELL", "/bin/sh");
    QTemporaryDir storage;
    qputenv("XDG_CONFIG_HOME", storage.path().toUtf8());
    qputenv("XDG_CACHE_HOME", storage.path().toUtf8());
    qputenv("XDG_DATA_HOME", storage.path().toUtf8());
    QApplication application(argc, argv);
    application.setStyle("Fusion");
    applyDockpipeLauncherTheme(application);
    TerminalWidgetTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "TerminalWidgetTest.moc"
