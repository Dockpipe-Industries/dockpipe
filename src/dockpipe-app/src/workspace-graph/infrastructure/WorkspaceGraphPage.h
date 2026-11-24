#pragma once
#include "LauncherUiExport.h"
#include <QJsonObject>

namespace WorkspaceGraphPage {
// Legacy exported helper retained for binary compatibility. The graph UI uses
// WorkspaceGraphReader and the admitted core Files CLI instead of this entrypoint.
// A transient read-only helper uses streaming directory iteration, not a tree index.
// Request: root, directory, after (exclusive name), limit (1..150), hidden.
DOCKPIPE_LAUNCHER_UI_EXPORT QJsonObject read(const QJsonObject &request);
DOCKPIPE_LAUNCHER_UI_EXPORT int run(int argc, char **argv);
constexpr int maximumPageNodes = 150;
constexpr int maximumVisibleNodes = 500;
constexpr int maximumReplyBytes = 2 * 1024 * 1024;
}
