#pragma once

#include "LauncherUiExport.h"
#include "WorkflowCatalogData.h"
struct Context;

class DOCKPIPE_LAUNCHER_UI_EXPORT SurfaceLaunchEnvironment {
public:
    static bool apply(Context &invocation, const PackageSurfaceActionMeta &action,
                      const QString &packageRoot, QString &error);
};
