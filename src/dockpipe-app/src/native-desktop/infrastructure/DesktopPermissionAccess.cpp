#include "DesktopPermissionAccess.h"
#include <QtGlobal>

#ifndef Q_OS_MAC
namespace {
class UnavailableDesktopPermissionAccess final : public DesktopPermissionAccess {
public:
    DesktopPermissionState state(DesktopPermission) const override
    {
        return DesktopPermissionState::Unavailable;
    }

    void request(DesktopPermission) override
    {
    }

    bool openSettings(DesktopPermission) override
    {
        return false;
    }
};
}

std::unique_ptr<DesktopPermissionAccess> createDesktopPermissionAccess()
{
    return std::make_unique<UnavailableDesktopPermissionAccess>();
}
#endif
