#pragma once
#include <optional>

bool browserNetworkHostSupportsIsolatedSessions();

// Dispatch before QApplication: Chromium and Qt's proxy are process scoped.
std::optional<int> dispatchBrowserNetworkHost(int argc, char **argv);
