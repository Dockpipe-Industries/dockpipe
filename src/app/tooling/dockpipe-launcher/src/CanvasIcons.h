#pragma once

#include <QIcon>

namespace CanvasIcons {
enum class Symbol {
    Add, Minus, Fit, Undo, Code, Properties, More, Folder, Workflow,
    Play, Save, Terminal, Keyboard, Monitor, Script, Parallel, Close, Check, Sidebar
};

// Stroke icons share a 24-unit grid and render at the requested device resolution.
QIcon icon(Symbol symbol);
}
