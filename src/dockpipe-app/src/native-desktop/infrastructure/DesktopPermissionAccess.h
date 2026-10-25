#pragma once

#include <memory>

enum class DesktopPermission { ScreenCapture, Accessibility };
enum class DesktopPermissionState { Allowed, NotAllowed, Unavailable };

class DesktopPermissionAccess {
public:
    virtual ~DesktopPermissionAccess() = default;
    virtual DesktopPermissionState state(DesktopPermission permission) const = 0;
    virtual void request(DesktopPermission permission) = 0;
    virtual bool openSettings(DesktopPermission permission) = 0;
};

std::unique_ptr<DesktopPermissionAccess> createDesktopPermissionAccess();
