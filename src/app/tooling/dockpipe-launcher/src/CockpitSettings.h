#pragma once

// Optional launcher feature preferences. MainWindow owns the availability gate;
// the cockpit owns the lifetimes of enabled input, capture and rendering services.
struct CockpitSettings {
    bool enabled = false;
    bool appControl = false;
    bool orbitalMotion = true;
    bool controller = false;
    bool showShip = true;
    bool voiceCommands = false;
    bool hardwareAcceleration = true;
};
