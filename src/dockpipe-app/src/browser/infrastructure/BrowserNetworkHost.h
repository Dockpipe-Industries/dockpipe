#pragma once
#include <optional>

// Dispatch before QApplication: Chromium and Qt's proxy are process scoped.
std::optional<int> dispatchBrowserNetworkHost(int argc, char **argv);
