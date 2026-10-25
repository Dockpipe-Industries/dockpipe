#pragma once

// Automatic uses the host bridge inside Flatpak and native PTYs elsewhere.
// Local explicitly selects this process's filesystem and process namespace.
// Callers that test or manage sandbox-local processes must select it deliberately.
enum class TerminalLaunchContext {
    Automatic,
    Local,
};
