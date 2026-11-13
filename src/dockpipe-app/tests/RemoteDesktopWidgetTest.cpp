#include "RemoteDesktopWidget.h"
#include "RemoteAccessMonitor.h"
#include "PackageDisplayWidget.h"
#include "RemoteDesktopSettingsDialog.h"

#include <QApplication>
#include <QCheckBox>
#include <QComboBox>
#include <QDialog>
#include <QDialogButtonBox>
#include <QSettings>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLabel>
#include <QPushButton>
#include <QStandardItemModel>
#include <QSignalSpy>
#include <QTemporaryDir>
#include <QTest>
#include <QThread>
#include <QTimer>
#include <cstdio>
#include <memory>

namespace {
QJsonObject catalog()
{
    return {{"schema", "dockpipe.remote.desktop/v1"}, {"target", QJsonObject{{"node", "fixture"}, {"fingerprint", QString(64, 'a')}, {"generation", QString(64, 'b')}}},
        {"provider", QJsonObject{{"id", qEnvironmentVariable("DOCKPIPE_DESKTOP_WIDGET_PROVIDER", "fixture.native")}, {"version", "1"}, {"artifact", QString(64, 'c')}}}, {"epoch", QString(64, 'd')}, {"view", true}, {"input", true}};
}

int fixture(const QStringList &arguments)
{
    if (!arguments.contains("--auth") || arguments.last() != "never")
        return 2;
    const QString command = arguments.value(2);
    QFile commands(qEnvironmentVariable("DOCKPIPE_DESKTOP_WIDGET_ROOT") + "/commands");
    if (!commands.open(QIODevice::WriteOnly | QIODevice::Append))
        return 3;
    commands.write(command.toUtf8() + '\n');
    commands.close();
    const auto mode = qgetenv("DOCKPIPE_DESKTOP_WIDGET_MODE");
    QJsonDocument response;
    if (command == "security-status") {
        response = QJsonDocument(QJsonObject{{"supported", true}, {"authority_available", true}, {"locked", mode == "locked"},
            {"idle_seconds", 300}, {"absolute_seconds", 3600}, {"lock_on_screen_lock", true}, {"lock_on_sleep", true},
            {"capabilities", QJsonObject{{"idle", true}, {"screen_lock", true}, {"sleep", true}}}});
    } else if (command == "info") {
        response = QJsonDocument(QJsonObject{{"state_directory", qEnvironmentVariable("DOCKPIPE_DESKTOP_WIDGET_ROOT")}});
    } else if (command == "nodes") {
        response = QJsonDocument(QJsonArray{QJsonObject{{"node", "fixture"}, {"status", "paired"}, {"fingerprint", QString(64, 'a')}},
            QJsonObject{{"node", "removed"}, {"status", "revoked"}, {"fingerprint", QString(64, 'e')}}});
    } else if (command == "desktop-status" || command == "desktop-close") {
        QFile requestFile(arguments.value(arguments.indexOf("--request") + 1));
        if (!requestFile.open(QIODevice::ReadOnly))
            return 6;
        const auto request = QJsonDocument::fromJson(requestFile.readAll()).object();
        if (command == "desktop-status" && !request.value("metadata_only").toBool())
            return 7;
        auto reference = command == "desktop-close" ? request : request.value("reference").toObject();
        if (reference.size() != 4)
            return 8;
        if (mode == "recover-wrong")
            reference["id"] = QString(64, 'f');
        QJsonObject view{{"reference", reference}, {"state", mode == "recover-closed" || mode == "recover-malformed" ? "closed" : "unknown"}, {"lease_millis", 0}};
        if (mode == "recover-malformed")
            view.remove("lease_millis");
        response = QJsonDocument(view);
    } else if (command == "desktop-catalog") {
        if (mode == "pause")
            QThread::msleep(300);
        auto value = catalog();
        if (mode == "view-only")
            value["input"] = false;
        if (mode == "foreign") {
            auto target = value.value("target").toObject();
            target["fingerprint"] = QString(64, 'f');
            value["target"] = target;
        }
        response = QJsonDocument(mode == "offline" ? QJsonArray{} : QJsonArray{value});
    } else {
        return 4; // The application page must never implement desktop transport.
    }
    const auto data = response.toJson(QJsonDocument::Compact);
    return std::fwrite(data.constData(), 1, size_t(data.size()), stdout) == size_t(data.size()) ? 0 : 5;
}

ResolverMeta provider()
{
    ResolverMeta result;
    result.name = qEnvironmentVariable("DOCKPIPE_DESKTOP_WIDGET_PROVIDER", "fixture.native");
    result.version = "1";
    result.capability = "remote.desktop";
    // PackageDisplayWidget owns actual contained-library/SDK qualification.
    const QString root = qEnvironmentVariable("DOCKPIPE_DISPLAY_FIXTURE_ROOT");
    result.surface.nativeDisplay = {1, "/missing/fixture", {{PackageDisplayWidget::platformName(), "/missing/fixture/display.so"}}};
    if (!root.isEmpty())
        result.surface.nativeDisplay = {1, root, {{PackageDisplayWidget::platformName(), root + "/assets/libdisplay-good.so"}}};
    return result;
}
}

class RemoteDesktopWidgetTest final : public QObject {
    Q_OBJECT
    QTemporaryDir m_settingsRoot;
private slots:
    void init()
    {
        QSettings::setDefaultFormat(QSettings::IniFormat);
        QSettings::setPath(QSettings::IniFormat, QSettings::UserScope, m_settingsRoot.path());
        QSettings().clear();
        qputenv("DOCKPIPE_BIN", QCoreApplication::applicationFilePath().toUtf8());
        qunsetenv("DOCKPIPE_DESKTOP_WIDGET_MODE");
        qunsetenv("DOCKPIPE_DESKTOP_WIDGET_PROVIDER");
    }
    void controlsFitNarrowWorkspaceAcrossPresentationChanges()
    {
        RemoteDesktopWidget page;
        // A 1180-pixel workspace leaves roughly 960 pixels beside navigation.
        page.resize(960, 600);
        page.show();
        for (const bool compact : {false, true, false}) {
            page.setCompactPresentation(compact);
            QApplication::processEvents();
            QVERIFY(page.minimumSizeHint().width() <= 960);
            QCOMPARE(page.width(), 960);
            for (const auto *name : {"desktopMachine", "desktopMode", "desktopConnect", "desktopDisconnect",
                     "desktopRefresh", "desktopUnlock", "desktopKeepConnected", "desktopSettings", "desktopFullscreen"}) {
                auto *control = page.findChild<QWidget *>(name);
                QVERIFY(control);
                if (control->isVisibleTo(&page))
                    QVERIFY2(page.rect().contains(QRect(control->mapTo(&page, QPoint()), control->size())), name);
            }
            for (const auto *name : {"desktopKeepConnected", "desktopSettings"}) {
                auto *control = page.findChild<QWidget *>(name);
                QVERIFY2(control && control->isVisibleTo(&page), name);
            }
        }
    }
    void pairedOfflineAndFingerprintMismatch_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("offline") << QByteArray("offline");
        QTest::newRow("different-machine-key") << QByteArray("foreign");
    }
    void pairedOfflineAndFingerprintMismatch()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", mode);
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        auto *refresh = page.findChild<QPushButton *>("desktopRefresh");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(refresh->isEnabled());
        machine->setCurrentIndex(0);
        QVERIFY(!page.findChild<QPushButton *>("desktopConnect")->isEnabled());
        QVERIFY(page.findChild<QLabel *>("desktopStatus")->text().contains("offline"));
    }
    void modePermissionsAndNoAutomaticConnection()
    {
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", "view-only");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        QCOMPARE(machine->currentIndex(), -1);
        machine->setCurrentIndex(0);
        auto *mode = page.findChild<QComboBox *>("desktopMode");
        QCOMPARE(mode->currentData().toString(), QStringLiteral("view"));
        QVERIFY(!qobject_cast<QStandardItemModel *>(mode->model())->item(1)->isEnabled());
        QVERIFY(page.findChild<QPushButton *>("desktopConnect")->isEnabled());
        QVERIFY(!page.findChild<PackageDisplayWidget *>()->hasPackageSurface());
        page.hide();
        page.show();
        QVERIFY(!page.findChild<PackageDisplayWidget *>()->hasPackageSurface());
        QVERIFY(!page.findChild<QPushButton *>("desktopDisconnect")->isEnabled());
    }
    void discoveryIsBoundedAndDoesNotBlockWidgets()
    {
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", "pause");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        int ticks = 0;
        QTimer timer;
        timer.setInterval(10);
        connect(&timer, &QTimer::timeout, this, [&] { ++ticks; });
        timer.start();
        page.show();
        for (int index = 0; index < 20; ++index)
            page.refresh();
        QTRY_COMPARE(page.findChild<QComboBox *>("desktopMachine")->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        QVERIFY(ticks > 10);
        QFile commands(root.filePath("commands"));
        QVERIFY(commands.open(QIODevice::ReadOnly));
        const auto contents = commands.readAll();
        QCOMPARE(contents.count("desktop-catalog\n"), 1);
        QVERIFY(!contents.contains("feature"));
        QVERIFY(!contents.contains("desktop-open"));
    }
    void explicitConnectRechecksCurrentDesktopReadiness()
    {
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        QCOMPARE(page.findChild<QComboBox *>("desktopMode")->currentData().toString(), QStringLiteral("control"));
        auto *button = page.findChild<QPushButton *>("desktopConnect");
        QVERIFY(button->isEnabled());
        QSignalSpy displays(page.findChild<PackageDisplayWidget *>(), &PackageDisplayWidget::displayFailed);
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", "offline");
        QTest::mouseClick(button, Qt::LeftButton);
        QTRY_VERIFY(page.findChild<QLabel *>("desktopStatus")->text().contains("changed"));
        QVERIFY(!button->isEnabled());
        QCOMPARE(displays.size(), 0); // No package opening against stale readiness.
        QVERIFY(!page.findChild<QPushButton *>("desktopDisconnect")->isEnabled());
        QFile commands(root.filePath("commands"));
        QVERIFY(commands.open(QIODevice::ReadOnly));
        const auto contents = commands.readAll();
        QCOMPARE(contents.count("desktop-catalog\n"), 2);
        QVERIFY(!contents.contains("desktop-open"));
    }
    void explicitViewSurvivesReadinessRefresh()
    {
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        auto *mode = page.findChild<QComboBox *>("desktopMode");
        QCOMPARE(mode->currentData().toString(), QStringLiteral("control"));
        mode->setCurrentIndex(0);
        QVERIFY(QMetaObject::invokeMethod(mode, "activated", Q_ARG(int, 0)));
        page.refresh();
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        QCOMPARE(mode->currentData().toString(), QStringLiteral("view"));
    }
    void retainedRecoveryRequiresExactClosedProof_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("closed") << QByteArray("recover-closed");
        QTest::newRow("unknown") << QByteArray("recover-unknown");
        QTest::newRow("foreign-result") << QByteArray("recover-wrong");
        QTest::newRow("missing-lease") << QByteArray("recover-malformed");
    }
    void retainedRecoveryRequiresExactClosedProof()
    {
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", mode);
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        const QJsonObject reference{{"schema", "dockpipe.remote.desktop/v1"}, {"id", QString(64, 'a')},
            {"target", catalog().value("target")}, {"epoch", QString(64, 'd')}};
        auto other = reference;
        other["id"] = QString(64, 'b');
        QSettings().setValue("remote/uncertainDesktopSessions", QJsonDocument(QJsonArray{reference, other}).toJson(QJsonDocument::Compact));
        RemoteDesktopWidget page;
        page.show();
        QTest::mouseClick(page.findChild<QPushButton *>("desktopRecoveryButton"), Qt::LeftButton);
        auto *dialog = page.findChild<QDialog *>("desktopRecoveryDialog");
        QVERIFY(dialog);
        QCOMPARE(dialog->findChild<QComboBox *>("desktopRecoverySessions")->count(), 2);
        QFile commands(root.filePath("commands"));
        QVERIFY(!commands.exists()); // Opening a recovery list never contacts or reopens a peer.
        QTest::mouseClick(dialog->findChild<QPushButton *>(mode == "recover-unknown" ? "desktopRecoveryRequestClose" : "desktopRecoveryInspect"), Qt::LeftButton);
        QTRY_VERIFY(!dialog->findChild<QLabel *>("desktopRecoveryStatus")->text().isEmpty());
        const auto retained = QJsonDocument::fromJson(QSettings().value("remote/uncertainDesktopSessions").toByteArray()).array();
        QCOMPARE(retained.size(), mode == "recover-closed" ? 1 : 2);
        QVERIFY(retained.contains(other));
        if (mode == "recover-closed")
            QVERIFY(!retained.contains(reference));
        else
            QVERIFY(retained.contains(reference));
        QVERIFY(commands.open(QIODevice::ReadOnly));
        const auto calls = commands.readAll();
        QVERIFY(calls.contains(mode == "recover-unknown" ? "desktop-close" : "desktop-status"));
        QVERIFY(!calls.contains("desktop-open"));
        dialog->close();
    }
    void disconnectWaitsForExactPackageClose_data()
    {
        QTest::addColumn<QByteArray>("mode");
        QTest::newRow("closed") << QByteArray("closed");
        QTest::newRow("unknown") << QByteArray("unknown");
        QTest::newRow("timeout") << QByteArray("timeout");
        QTest::newRow("foreign-completion") << QByteArray("foreign");
        QTest::newRow("page-hidden") << QByteArray("hide");
        QTest::newRow("destroyed-during-close") << QByteArray("destroy");
    }
    void disconnectWaitsForExactPackageClose()
    {
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        QFETCH(QByteArray, mode);
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", mode);
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        auto page = std::make_unique<RemoteDesktopWidget>();
        QVERIFY(!page->findChild<QCheckBox *>("desktopKeepConnected")->isChecked());
        page->resize(1600, 1000);
        page->setAccessMonitor(&access);
        page->setResolvers({provider()});
        page->show();
        auto *machine = page->findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page->findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        QTest::mouseClick(page->findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page->findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        QPointer<QWidget> surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        auto *caption = display->findChild<QLabel *>(QString{}, Qt::FindDirectChildrenOnly);
        QVERIFY(caption);
        caption->setText("Connected · encrypted relay · H.264 · view only");
        caption->show();
        QTRY_VERIFY(surface->height() > page->height() * 0.7);
        QVERIFY(caption->height() <= caption->sizeHint().height() + 2);
        QVERIFY(surface->mapTo(page.get(), QPoint()).y() < 220);
        if (mode == "hide")
            page->hide();
        else
            page->disconnectDesktop();
        QVERIFY(display->isClosing());
        QVERIFY(surface);
        QVERIFY(!surface->isEnabled());
        QVERIFY(surface->property("closeRequested").toBool());
        QVERIFY(!page->findChild<QPushButton *>("desktopConnect")->isEnabled());
        QVERIFY(!machine->isEnabled());
        QVERIFY(page->findChild<QLabel *>("desktopStatus")->text().contains("Closing"));
        if (mode == "destroy") {
            page.reset();
            QVERIFY(!surface);
            QTest::qWait(180); // The old package timer cannot reach a deleted consumer.
            return;
        }
        QTRY_VERIFY(!display->isClosing());
        QVERIFY(!surface);
        const auto message = page->findChild<QLabel *>("desktopStatus")->text();
        if (mode == "closed" || mode == "hide")
            QVERIFY2(message.contains("cleanup was verified"), qPrintable(message));
        else
            QVERIFY2(message.contains("unconfirmed remote cleanup"), qPrintable(message));
        qunsetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
    }
    void keepConnectedAcrossTabs_data()
    {
        QTest::addColumn<QByteArray>("finish");
        QTest::newRow("explicit-disconnect") << QByteArray("disconnect");
        QTest::newRow("destroy-hidden-page") << QByteArray("destroy");
        QTest::newRow("disable-while-hidden") << QByteArray("disable");
    }
    void settingsSaveAndCancel_data()
    {
        QTest::addColumn<bool>("save");
        QTest::newRow("save") << true;
        QTest::newRow("cancel") << false;
    }
    void settingsSaveAndCancel()
    {
        QFETCH(bool, save);
        RemoteDesktopWidget page;
        page.show();
        bool visited = false;
        QTimer::singleShot(0, &page, [&] {
            auto *dialog = page.findChild<RemoteDesktopSettingsDialog *>();
            if (!dialog)
                return;
            visited = true;
            auto *quality = dialog->findChild<QComboBox *>("desktopQuality");
            auto *frameRate = dialog->findChild<QComboBox *>("desktopFrameRate");
            auto *scale = dialog->findChild<QComboBox *>("desktopScale");
            auto *cursor = dialog->findChild<QCheckBox *>("desktopHideCursor");
            auto *swapControlCommand = dialog->findChild<QCheckBox *>("desktopSwapControlCommand");
            auto *clipboard = dialog->findChild<QCheckBox *>("desktopShareClipboard");
            auto *images = dialog->findChild<QCheckBox *>("desktopShareImages");
            auto *files = dialog->findChild<QCheckBox *>("desktopShareFiles");
            auto *decoder = dialog->findChild<QComboBox *>("desktopDecoder");
            auto *displaySource = dialog->findChild<QComboBox *>("desktopDisplaySource");
            QCOMPARE(quality->currentData().toString(), QStringLiteral("high"));
            QCOMPARE(frameRate->currentData().toInt(), 30);
            QCOMPARE(scale->currentData().toString(), QStringLiteral("fit"));
            QVERIFY(cursor->isChecked());
            QVERIFY(!swapControlCommand->isChecked());
            QVERIFY(clipboard->isChecked());
            QVERIFY(!images->isChecked());
            QVERIFY(!files->isChecked());
            QCOMPARE(decoder->currentData().toString(), QStringLiteral("software"));
            QCOMPARE(displaySource->currentData().toString(), QStringLiteral("existing"));
            displaySource->setCurrentIndex(displaySource->findData("virtual"));
            quality->setCurrentIndex(0);
            frameRate->setCurrentIndex(1);
            scale->setCurrentIndex(1);
            cursor->setChecked(false);
            swapControlCommand->setChecked(true);
            clipboard->setChecked(false);
            images->setChecked(true);
            files->setChecked(true);
            auto *buttons = dialog->findChild<QDialogButtonBox *>();
            QTest::mouseClick(buttons->button(save ? QDialogButtonBox::Save : QDialogButtonBox::Cancel), Qt::LeftButton);
        });
        QTest::mouseClick(page.findChild<QPushButton *>("desktopSettings"), Qt::LeftButton);
        QVERIFY(visited);
        const QSettings preferences;
        QCOMPARE(preferences.value("remote/desktopQuality", "high").toString(), save ? "low" : "high");
        QCOMPARE(preferences.value("remote/desktopFrameRate", 30).toInt(), save ? 15 : 30);
        QCOMPARE(preferences.value("remote/desktopShareClipboard", true).toBool(), !save);
        QCOMPARE(preferences.value("remote/desktopShareImages", false).toBool(), save);
        QCOMPARE(preferences.value("remote/desktopShareFiles", false).toBool(), save);
        QCOMPARE(preferences.value("remote/desktopScale", "fit").toString(), save ? "actual" : "fit");
        QCOMPARE(preferences.value("remote/desktopHideCursor", true).toBool(), !save);
        QCOMPARE(preferences.value("remote/desktopSwapControlCommand", false).toBool(), save);
        QCOMPARE(preferences.value("remote/desktopDisplaySource", "existing").toString(), save ? "virtual" : "existing");
    }
    void displaySourcePreferences_data()
    {
        QTest::addColumn<QString>("source");
        QTest::newRow("current-desktop") << QString("existing");
        QTest::newRow("virtual-desktop") << QString("virtual");
        QTest::newRow("invalid-source") << QString("invalid");
    }
    void shortcutAndRichClipboardPreferencesSurviveReopen()
    {
        QSettings preferences;
        preferences.setValue("remote/desktopSwapControlCommand", true);
        preferences.setValue("remote/desktopShareClipboard", false);
        preferences.setValue("remote/desktopShareImages", true);
        preferences.setValue("remote/desktopShareFiles", true);
        RemoteDesktopWidget page;
        page.show();
        bool visited = false;
        QTimer::singleShot(0, &page, [&] {
            auto *dialog = page.findChild<RemoteDesktopSettingsDialog *>();
            QVERIFY(dialog);
            visited = true;
            QCOMPARE(dialog->presentationOptions().value("swap_control_command"), QJsonValue(true));
            const auto capture = dialog->captureOptions();
            QVERIFY(!capture.contains("clipboard_text"));
            QCOMPARE(capture.value("clipboard_images"), QJsonValue(true));
            QCOMPARE(capture.value("clipboard_files"), QJsonValue(true));
            dialog->reject();
        });
        QTest::mouseClick(page.findChild<QPushButton *>("desktopSettings"), Qt::LeftButton);
        QVERIFY(visited);
    }
    void displaySourcePreferences()
    {
        QFETCH(QString, source);
        QSettings preferences;
        preferences.setValue("remote/desktopDisplaySource", source);
        RemoteDesktopWidget page;
        page.show();
        bool visited = false;
        QTimer::singleShot(0, &page, [&] {
            auto *dialog = page.findChild<RemoteDesktopSettingsDialog *>();
            QVERIFY(dialog);
            visited = true;
            const auto options = dialog->captureOptions();
            if (source == "virtual")
                QCOMPARE(options.value("display_source").toString(), QStringLiteral("virtual"));
            else
                QVERIFY(!options.contains("display_source"));
            dialog->accept();
        });
        QTest::mouseClick(page.findChild<QPushButton *>("desktopSettings"), Qt::LeftButton);
        QVERIFY(visited);
        QCOMPARE(preferences.value("remote/desktopDisplaySource").toString(), source == "virtual" ? "virtual" : "existing");
    }
    void virtualDesktopUsesExistingResolutionPresets()
    {
        for (const auto &quality : {"low", "balanced", "high", "ultra"}) {
            RemoteDesktopSettingsDialog dialog({{"quality", quality}, {"frame_rate", 30}, {"display_source", "virtual"}},
                {{"scale", "fit"}, {"hide_cursor", true}}, "software");
            QCOMPARE(dialog.captureOptions(), (QJsonObject{{"quality", quality}, {"frame_rate", 30}, {"display_source", "virtual"}}));
            auto *displaySource = dialog.findChild<QComboBox *>("desktopDisplaySource");
            displaySource->setCurrentIndex(displaySource->findData("existing"));
            QVERIFY(!dialog.captureOptions().contains("display_source"));
        }
    }
    void extendedCaptureAndDecoderPreferences_data()
    {
        QTest::addColumn<QString>("quality");
        QTest::addColumn<int>("frameRate");
        QTest::addColumn<QString>("decoder");
        QTest::newRow("4k-60-software") << QString("ultra") << 60 << QString("software");
        QTest::newRow("4k-60-hardware") << QString("ultra") << 60 << QString("hardware");
        QTest::newRow("1080p-60-auto") << QString("high") << 60 << QString("auto");
        QTest::newRow("invalid-preferences") << QString("invalid") << 120 << QString("invalid");
    }
    void extendedCaptureAndDecoderPreferences()
    {
        QFETCH(QString, quality);
        QFETCH(int, frameRate);
        QFETCH(QString, decoder);
        QSettings preferences;
        preferences.setValue("remote/desktopQuality", quality);
        preferences.setValue("remote/desktopFrameRate", frameRate);
        preferences.setValue("remote/desktopDecoder", decoder);
        if (quality == "invalid") {
            quality = QStringLiteral("high");
            frameRate = 30;
            decoder = QStringLiteral("software");
        }
        RemoteDesktopWidget page;
        page.show();
        bool visited = false;
        QTimer::singleShot(0, &page, [&] {
            auto *dialog = page.findChild<RemoteDesktopSettingsDialog *>();
            if (!dialog)
                return;
            visited = true;
            QCOMPARE(dialog->captureOptions(), (QJsonObject{{"quality", quality}, {"frame_rate", frameRate}, {"clipboard_text", true}}));
            QCOMPARE(dialog->decoderMode(), decoder);
            auto *buttons = dialog->findChild<QDialogButtonBox *>();
            QTest::mouseClick(buttons->button(QDialogButtonBox::Save), Qt::LeftButton);
        });
        QTest::mouseClick(page.findChild<QPushButton *>("desktopSettings"), Qt::LeftButton);
        QVERIFY(visited);
        QCOMPARE(preferences.value("remote/desktopQuality").toString(), quality);
        QCOMPARE(preferences.value("remote/desktopFrameRate").toInt(), frameRate);
        QCOMPARE(preferences.value("remote/desktopDecoder").toString(), decoder);
    }
    void fullscreenKeepsSessionUntilExplicitDisconnect_data()
    {
        QTest::addColumn<bool>("keepConnected");
        QTest::addColumn<bool>("revoke");
        QTest::newRow("disconnect-on-tab-switch") << false << false;
        QTest::newRow("keep-across-tabs") << true << false;
        QTest::newRow("revoke-default") << false << true;
        QTest::newRow("revoke-kept-session") << true << true;
    }
    void arbitraryResolverReceivesGenericLaunchEnvelope_data()
    {
        QTest::addColumn<QString>("mode");
        QTest::newRow("view") << QString("view");
        QTest::newRow("control") << QString("control");
    }
    void arbitraryResolverReceivesGenericLaunchEnvelope()
    {
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        QFETCH(QString, mode);
        qputenv("DOCKPIPE_DESKTOP_WIDGET_PROVIDER", "example.rdp.viewer");
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", "closed");
        QSettings preferences;
        preferences.setValue("remote/desktopQuality", "balanced");
        preferences.setValue("remote/desktopFrameRate", 60);
        preferences.setValue("remote/desktopDecoder", "auto");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        auto *accessMode = page.findChild<QComboBox *>("desktopMode");
        accessMode->setCurrentIndex(accessMode->findData(mode));
        QTest::mouseClick(page.findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page.findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        auto *surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        const auto options = surface->property("launchOptions").toJsonObject();
        QCOMPARE(options.value("schema").toString(), QStringLiteral("dockpipe.remote.desktop-display/v1"));
        QCOMPARE(options.value("catalog").toObject(), catalog());
        QCOMPARE(options.value("mode").toString(), mode);
        QCOMPARE(options.value("cli").toString(), QCoreApplication::applicationFilePath());
        QCOMPARE(options.value("state_directory").toString(), root.path());
        QJsonObject expectedCapture{{"quality", "balanced"}, {"frame_rate", 60}};
        if (mode == "control")
            expectedCapture.insert("clipboard_text", true);
        QCOMPARE(options.value("capture").toObject(), expectedCapture);
        QCOMPARE(options.value("display").toObject(), (QJsonObject{{"scale", "fit"}, {"hide_cursor", true}}));
        QCOMPARE(options.value("decoder").toString(), QStringLiteral("auto"));
        page.disconnectDesktop();
        QTRY_VERIFY(!display->hasPackageSurface());
        qunsetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
    }
    void connectionReceivesExtendedPreferences_data()
    {
        QTest::addColumn<bool>("virtualDesktop");
        QTest::newRow("current-desktop") << false;
        QTest::newRow("virtual-desktop") << true;
    }
    void clipboardIsControlOnly_data()
    {
        QTest::addColumn<QString>("mode");
        QTest::addColumn<bool>("enabled");
        QTest::newRow("control-enabled") << QString("control") << true;
        QTest::newRow("control-disabled") << QString("control") << false;
        QTest::newRow("view-never-shares") << QString("view") << true;
    }
    void clipboardIsControlOnly()
    {
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        QFETCH(QString, mode);
        QFETCH(bool, enabled);
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", "closed");
        QSettings().setValue("remote/desktopShareClipboard", enabled);
        QSettings().setValue("remote/desktopShareImages", enabled);
        QSettings().setValue("remote/desktopShareFiles", enabled);
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        auto *accessMode = page.findChild<QComboBox *>("desktopMode");
        accessMode->setCurrentIndex(accessMode->findData(mode));
        QTest::mouseClick(page.findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page.findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        auto *surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        const auto capture = surface->property("captureOptions").toJsonObject();
        QCOMPARE(capture.value("clipboard_text").toBool(), enabled && mode == "control");
        QCOMPARE(capture.contains("clipboard_text"), enabled && mode == "control");
        QCOMPARE(capture.contains("clipboard_images"), enabled && mode == "control");
        QCOMPARE(capture.contains("clipboard_files"), enabled && mode == "control");
        page.disconnectDesktop();
        QTRY_VERIFY(!display->hasPackageSurface());
    }
    void connectionReceivesExtendedPreferences()
    {
        QFETCH(bool, virtualDesktop);
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", "closed");
        QSettings preferences;
        preferences.setValue("remote/desktopQuality", "ultra");
        preferences.setValue("remote/desktopFrameRate", 60);
        preferences.setValue("remote/desktopDecoder", "hardware");
        preferences.setValue("remote/desktopSwapControlCommand", true);
        preferences.setValue("remote/desktopDisplaySource", virtualDesktop ? "virtual" : "existing");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        QTest::mouseClick(page.findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page.findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        auto *surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        QJsonObject expectedCapture{{"quality", "ultra"}, {"frame_rate", 60}, {"clipboard_text", true}};
        if (virtualDesktop)
            expectedCapture.insert("display_source", "virtual");
        QCOMPARE(surface->property("captureOptions").toJsonObject(), expectedCapture);
        QCOMPARE(surface->property("presentationOptions").toJsonObject().value("swap_control_command"), QJsonValue(true));
        QCOMPARE(surface->property("decoderMode").toString(), QStringLiteral("hardware"));
        surface->setProperty("decoderFailure", QJsonObject{{"code", "hardware-unavailable"}});
        display->displayFailed(QStringLiteral("Opaque fixed package failure"));
        QVERIFY(page.findChild<QLabel *>("desktopStatus")->text().contains("Choose Automatic or Software"));
        QTRY_VERIFY(!display->hasPackageSurface());
        qunsetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
    }
    void fullscreenKeepsSessionUntilExplicitDisconnect()
    {
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        QFETCH(bool, keepConnected);
        QFETCH(bool, revoke);
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", "closed");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        RemoteDesktopWidget page;
        page.setCompactPresentation(true);
        page.findChild<QCheckBox *>("desktopKeepConnected")->setChecked(keepConnected);
        page.setAccessMonitor(&access);
        page.setResolvers({provider()});
        page.show();
        auto *machine = page.findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page.findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        QTest::mouseClick(page.findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page.findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        QPointer<QWidget> surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        QCOMPARE(surface->property("captureOptions").toJsonObject(), (QJsonObject{{"quality", "high"}, {"frame_rate", 30}, {"clipboard_text", true}}));
        if (!keepConnected && !revoke) {
            auto *status = page.findChild<QLabel *>("desktopDecoderStatus");
            QVERIFY(status->isHidden());
            surface->setProperty("decoderStatus", QJsonObject{{"requested", "hardware"}, {"backend", "cuda"}, {"fallback_reason", ""}});
            QTRY_COMPARE(status->text(), QStringLiteral("Decoder: Hardware (CUDA/NVDEC)"));
            QVERIFY(!status->isHidden());
            surface->setProperty("decoderStatus", QJsonObject{{"requested", "auto"}, {"backend", "software"}, {"fallback_reason", "hardware-unavailable"}});
            QTRY_COMPARE(status->text(), QStringLiteral("Decoder: Software (automatic fallback)"));
            surface->setProperty("decoderStatus", QJsonObject{{"backend", "cuda"}, {"oversized", QString(20000, 'x')}});
            QTRY_VERIFY(status->isHidden());
            QVERIFY(display->surfaceMetadata(QByteArray(65, 'x')).isEmpty());
        }
        const QJsonObject presentation{{"scale", "actual"}, {"hide_cursor", false}};
        QVERIFY(display->updatePresentationOptions(presentation));
        QCOMPARE(surface->property("presentationOptions").toJsonObject(), presentation);
        QVERIFY(!display->updatePresentationOptions({{"oversized", QString(20000, 'x')}}));
        QVERIFY(!display->updatePresentationOptions({{"scale", "invalid"}, {"hide_cursor", false}}));
        QCOMPARE(surface->property("presentationOptions").toJsonObject(), presentation);
        QCOMPARE(surface->property("presentationUpdates").toInt(), 1);
        for (int cycle = 0; cycle < 2; ++cycle) {
            QTest::mouseClick(page.findChild<QPushButton *>("desktopFullscreen"), Qt::LeftButton);
            QPointer<QDialog> fullscreen = page.findChild<QDialog *>("desktopFullscreenWindow");
            QVERIFY(fullscreen);
            QTRY_VERIFY(fullscreen->isFullScreen());
            QTest::qWait(180);
            QVERIFY(surface);
            QVERIFY(!surface->property("closeRequested").toBool());
            QCOMPARE(display->parentWidget(), fullscreen);
            if (cycle == 0)
                QTest::mouseClick(fullscreen->findChild<QPushButton *>("desktopExitFullscreen"), Qt::LeftButton);
            else
                fullscreen->close();
            QTRY_COMPARE(display->parentWidget(), &page);
            QTRY_VERIFY(!fullscreen);
            QVERIFY(surface);
            QVERIFY(!surface->property("closeRequested").toBool());
        }
        QTest::mouseClick(page.findChild<QPushButton *>("desktopFullscreen"), Qt::LeftButton);
        QPointer<QDialog> fullscreen = page.findChild<QDialog *>("desktopFullscreenWindow");
        QVERIFY(fullscreen);
        if (revoke)
            page.setPairedMachines({});
        else
            page.hide();
        QTRY_VERIFY(!fullscreen);
        if (keepConnected && !revoke) {
            QTest::qWait(180);
            QVERIFY(surface);
            page.disconnectDesktop();
        }
        QTRY_VERIFY(!surface);
        qunsetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
    }
    void keepConnectedAcrossTabs()
    {
        if (qEnvironmentVariableIsEmpty("DOCKPIPE_DISPLAY_FIXTURE_ROOT"))
            QSKIP("Requires the explicit contained display fixture.");
        QFETCH(QByteArray, finish);
        qputenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE", "closed");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Unlocked);
        auto page = std::make_unique<RemoteDesktopWidget>();
        auto *keepConnected = page->findChild<QCheckBox *>("desktopKeepConnected");
        QVERIFY(!keepConnected->isChecked());
        keepConnected->setChecked(true);
        {
            RemoteDesktopWidget otherPage;
            QVERIFY(otherPage.findChild<QCheckBox *>("desktopKeepConnected")->isChecked());
        }
        page->setAccessMonitor(&access);
        page->setResolvers({provider()});
        page->show();
        auto *machine = page->findChild<QComboBox *>("desktopMachine");
        QTRY_COMPARE(machine->count(), 1);
        QTRY_VERIFY(page->findChild<QPushButton *>("desktopRefresh")->isEnabled());
        machine->setCurrentIndex(0);
        QTest::mouseClick(page->findChild<QPushButton *>("desktopConnect"), Qt::LeftButton);
        auto *display = page->findChild<PackageDisplayWidget *>();
        QTRY_VERIFY(display->hasPackageSurface());
        QPointer<QWidget> surface = display->findChild<QWidget *>("installedConsumerDisplay");
        QVERIFY(surface);
        page->hide();
        QTest::qWait(180);
        QVERIFY(surface);
        QVERIFY(!surface->property("closeRequested").toBool());
        QVERIFY(!display->isClosing());
        page->show();
        QTest::qWait(180);
        QCOMPARE(display->findChild<QWidget *>("installedConsumerDisplay"), surface.data());
        QVERIFY(!surface->property("closeRequested").toBool());
        if (finish == "destroy") {
            page->hide();
            page.reset();
            QVERIFY(!surface);
        } else {
            if (finish == "disable") {
                page->hide();
                keepConnected->setChecked(false);
                QVERIFY(!QSettings().value("remote/keepDesktopConnectedAcrossTabs").toBool());
            } else {
                page->disconnectDesktop();
            }
            QVERIFY(display->isClosing());
            QTRY_VERIFY(!surface);
            QVERIFY(!display->isClosing());
        }
        qunsetenv("DOCKPIPE_DISPLAY_CLOSE_FIXTURE");
    }
    void lockedAccessDoesNotDiscoverOrOpen()
    {
        qputenv("DOCKPIPE_DESKTOP_WIDGET_MODE", "locked");
        QTemporaryDir root;
        qputenv("DOCKPIPE_DESKTOP_WIDGET_ROOT", root.path().toUtf8());
        RemoteAccessMonitor access;
        QTRY_COMPARE(access.state(), RemoteAccessMonitor::State::Locked);
        RemoteDesktopWidget page;
        page.setAccessMonitor(&access);
        page.show();
        QVERIFY(page.findChild<QPushButton *>("desktopUnlock")->isVisible());
        QVERIFY(!page.findChild<QPushButton *>("desktopConnect")->isEnabled());
        QFile commands(root.filePath("commands"));
        QVERIFY(commands.open(QIODevice::ReadOnly));
        QVERIFY(!commands.readAll().contains("desktop-catalog"));
    }
};

int main(int argc, char **argv)
{
    QApplication app(argc, argv);
    QCoreApplication::setOrganizationName("DockpipeDesktopWidgetFixture");
    QCoreApplication::setApplicationName("remote-desktop-widget-test");
    if (app.arguments().value(1) == "remote")
        return fixture(app.arguments());
    RemoteDesktopWidgetTest test;
    return QTest::qExec(&test, argc, argv);
}
#include "RemoteDesktopWidgetTest.moc"
