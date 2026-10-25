#include "DesktopPermissionsWidget.h"
#include <QLabel>
#include <QPushButton>
#include <QTimer>
#include <QtTest>

namespace {
class FakePermissionAccess final : public DesktopPermissionAccess {
public:
    DesktopPermissionState screen = DesktopPermissionState::NotAllowed;
    DesktopPermissionState accessibility = DesktopPermissionState::NotAllowed;
    int requests = 0;
    int settingsOpens = 0;
    bool settingsAvailable = true;

    DesktopPermissionState state(DesktopPermission permission) const override
    {
        return permission == DesktopPermission::ScreenCapture ? screen : accessibility;
    }

    void request(DesktopPermission) override
    {
        ++requests;
    }

    bool openSettings(DesktopPermission) override
    {
        ++settingsOpens;
        return settingsAvailable;
    }
};
}

class DesktopPermissionsWidgetTest : public QObject {
    Q_OBJECT
private slots:
    void checkingStatusNeverPromptsOrClaimsPermission()
    {
        auto access = std::make_unique<FakePermissionAccess>();
        auto *observed = access.get();
        DesktopPermissionsWidget widget(nullptr, std::move(access));
        widget.show();
        QCOMPARE(observed->requests, 0);
        auto *button = widget.findChild<QPushButton *>("accessibilityRequest");
        QVERIFY(button);
        button->click();
        QCOMPARE(observed->requests, 1);
        QCOMPARE(widget.findChild<QLabel *>("accessibilityStatus")->text(), QStringLiteral("Not allowed"));
        QCOMPARE(observed->settingsOpens, 0);
    }

    void externalGrantRefreshesAndHiddenWidgetStopsPolling()
    {
        auto access = std::make_unique<FakePermissionAccess>();
        auto *observed = access.get();
        DesktopPermissionsWidget widget(nullptr, std::move(access));
        widget.show();
        observed->accessibility = DesktopPermissionState::Allowed;
        QTRY_COMPARE_WITH_TIMEOUT(widget.findChild<QLabel *>("accessibilityStatus")->text(),
            QStringLiteral("Allowed"), 2000);
        QVERIFY(!widget.findChild<QPushButton *>("accessibilityRequest")->isEnabled());
        QCOMPARE(observed->requests, 0);
        widget.hide();
        QVERIFY(!widget.findChild<QTimer *>()->isActive());
    }

    void unavailableAccessAndSettingsFailureRemainVisible()
    {
        auto access = std::make_unique<FakePermissionAccess>();
        auto *observed = access.get();
        access->screen = DesktopPermissionState::Unavailable;
        access->settingsAvailable = false;
        DesktopPermissionsWidget widget(nullptr, std::move(access));
        QVERIFY(!widget.findChild<QPushButton *>("screenCaptureRequest")->isEnabled());
        QVERIFY(!widget.findChild<QPushButton *>("screenCaptureSettings")->isEnabled());
        QCOMPARE(widget.findChild<QLabel *>("screenCaptureStatus")->text(), QStringLiteral("Unavailable"));
        widget.findChild<QPushButton *>("accessibilitySettings")->click();
        QCOMPARE(observed->settingsOpens, 1);
        bool failureVisible = false;
        for (auto *label : widget.findChildren<QLabel *>())
            failureVisible |= label->text().contains(QStringLiteral("Could not open System Settings"));
        QVERIFY(failureVisible);
    }
};

QTEST_MAIN(DesktopPermissionsWidgetTest)
#include "DesktopPermissionsWidgetTest.moc"
