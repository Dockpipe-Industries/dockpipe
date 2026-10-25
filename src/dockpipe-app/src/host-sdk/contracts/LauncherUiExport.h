#pragma once

#include <QtCore/qglobal.h>

// MSVC consumers need imported QObject metadata as well as exported methods.
#if defined(Q_OS_WIN)
#  if defined(DOCKPIPE_LAUNCHER_UI_BUILD)
#    define DOCKPIPE_LAUNCHER_UI_EXPORT Q_DECL_EXPORT
#  elif defined(DOCKPIPE_LAUNCHER_UI_USE)
#    define DOCKPIPE_LAUNCHER_UI_EXPORT Q_DECL_IMPORT
#  else
#    define DOCKPIPE_LAUNCHER_UI_EXPORT
#  endif
#else
#  define DOCKPIPE_LAUNCHER_UI_EXPORT
#endif
