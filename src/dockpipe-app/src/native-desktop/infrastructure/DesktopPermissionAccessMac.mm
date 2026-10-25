#include "DesktopPermissionAccess.h"
#include <QDesktopServices>
#include <QUrl>

#import <ApplicationServices/ApplicationServices.h>
#import <CoreGraphics/CoreGraphics.h>

namespace {
class MacDesktopPermissionAccess final : public DesktopPermissionAccess {
public:
    DesktopPermissionState state(DesktopPermission permission) const override
    {
        if (permission == DesktopPermission::Accessibility)
            return AXIsProcessTrusted() ? DesktopPermissionState::Allowed : DesktopPermissionState::NotAllowed;
        if (@available(macOS 10.15, *))
            return CGPreflightScreenCaptureAccess() ? DesktopPermissionState::Allowed : DesktopPermissionState::NotAllowed;
        return DesktopPermissionState::Unavailable;
    }

    void request(DesktopPermission permission) override
    {
        if (state(permission) != DesktopPermissionState::NotAllowed)
            return;
        if (permission == DesktopPermission::Accessibility) {
            const void *keys[] = {kAXTrustedCheckOptionPrompt};
            const void *values[] = {kCFBooleanTrue};
            auto options = CFDictionaryCreate(kCFAllocatorDefault, keys, values, 1,
                &kCFTypeDictionaryKeyCallBacks, &kCFTypeDictionaryValueCallBacks);
            if (options) {
                AXIsProcessTrustedWithOptions(options);
                CFRelease(options);
            }
        } else {
            if (@available(macOS 10.15, *))
                CGRequestScreenCaptureAccess();
        }
    }

    bool openSettings(DesktopPermission permission) override
    {
        const auto pane = permission == DesktopPermission::Accessibility
            ? QStringLiteral("Privacy_Accessibility") : QStringLiteral("Privacy_ScreenCapture");
        return QDesktopServices::openUrl(QUrl(
            QStringLiteral("x-apple.systempreferences:com.apple.preference.security?") + pane));
    }
};
}

std::unique_ptr<DesktopPermissionAccess> createDesktopPermissionAccess()
{
    return std::make_unique<MacDesktopPermissionAccess>();
}
