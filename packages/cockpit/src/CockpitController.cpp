#include "CockpitController.h"
#include <QLibrary>
#include <array>
#include <cmath>

// SDL2's stable public controller ABI. Optional runtime loading keeps the base
// Qt launcher independent of controller packages and uses SDL's device mappings.
struct CockpitController::Native {
#if defined(Q_OS_LINUX)
    QLibrary library{"SDL2-2.0", 0};
#else
    QLibrary library{"SDL2"};
#endif
    int (*init)(quint32) = nullptr;
    void (*quit)(quint32) = nullptr;
    int (*count)() = nullptr;
    int (*isController)(int) = nullptr;
    void *(*open)(int) = nullptr;
    void (*close)(void *) = nullptr;
    int (*attached)(void *) = nullptr;
    void (*update)() = nullptr;
    qint16 (*axis)(void *, int) = nullptr;
    quint8 (*button)(void *, int) = nullptr;
    const char *(*name)(void *) = nullptr;
    void *device = nullptr;
    std::array<bool, 15> previous{};
    int scanTicks = 0;
    bool initialized = false;

    template<typename Function>
    bool resolve(Function &function, const char *symbol)
    {
        function = reinterpret_cast<Function>(library.resolve(symbol));
        return function != nullptr;
    }
};

CockpitController::CockpitController(QObject *parent) : QObject(parent)
{
    m_timer.setInterval(33);
    connect(&m_timer, &QTimer::timeout, this, &CockpitController::poll);
}
CockpitController::~CockpitController()
{
    // Sibling widgets may already be gone during QObject child destruction.
    // Destruction releases the device without sending a final pilot event.
    blockSignals(true);
    disable();
}

qreal CockpitController::normalizedAxis(int value)
{
    const qreal normalized = qBound(-1.0, value / 32767.0, 1.0);
    constexpr qreal deadZone = 0.18;
    if (std::abs(normalized) <= deadZone)
        return 0;
    return std::copysign((std::abs(normalized) - deadZone) / (1 - deadZone), normalized);
}

float CockpitController::normalizedTrigger(int value)
{
    return qBound(0.f, (value / 32767.f - .04f) / .96f, 1.f);
}

bool CockpitController::enable()
{
    disable();
    m_native = std::make_unique<Native>();
    auto &api = *m_native;
    if (!api.library.load()
        || !api.resolve(api.init, "SDL_InitSubSystem")
        || !api.resolve(api.quit, "SDL_QuitSubSystem")
        || !api.resolve(api.count, "SDL_NumJoysticks")
        || !api.resolve(api.isController, "SDL_IsGameController")
        || !api.resolve(api.open, "SDL_GameControllerOpen")
        || !api.resolve(api.close, "SDL_GameControllerClose")
        || !api.resolve(api.attached, "SDL_GameControllerGetAttached")
        || !api.resolve(api.update, "SDL_GameControllerUpdate")
        || !api.resolve(api.axis, "SDL_GameControllerGetAxis")
        || !api.resolve(api.button, "SDL_GameControllerGetButton")
        || !api.resolve(api.name, "SDL_GameControllerName")
        || api.init(0x2000) < 0) {
        m_native.reset();
        emit statusChanged(tr("Controller unavailable. Install the optional SDL2 runtime."));
        return false;
    }
    api.initialized = true;
    emit statusChanged(tr("Looking for an SDL-compatible controller…"));
    m_timer.start();
    poll();
    return true;
}

void CockpitController::disable()
{
    m_timer.stop();
    emit pilotInput({}, {}, false);
    emit inputSuspended();
    if (m_native) {
        if (m_native->device)
            m_native->close(m_native->device);
        if (m_native->initialized)
            m_native->quit(0x2000);
        m_native.reset();
    }
}

void CockpitController::setInputActive(bool active)
{
    if (m_inputActive && !active)
        emit inputSuspended();
    m_inputActive = active;
    if (!active)
        emit pilotInput({}, {}, false);
}

void CockpitController::poll()
{
    if (!m_native)
        return;
    auto &api = *m_native;
    api.update();
    if (api.device && !api.attached(api.device)) {
        api.close(api.device);
        api.device = nullptr;
        for (int button = 0; button < int(api.previous.size()); ++button) {
            if (api.previous[button])
                emit buttonChanged(button, false);
        }
        api.previous.fill(false);
        emit pilotInput({}, {}, false);
        emit inputSuspended();
        emit statusChanged(tr("Controller disconnected. Flight stopped."));
    }
    if (!api.device) {
        if (api.scanTicks++ % 60 != 0)
            return;
        const int count = qMin(32, api.count());
        for (int index = 0; index < count; ++index) {
            if (api.isController(index)) {
                api.device = api.open(index);
                if (api.device)
                    break;
            }
        }
        if (api.device)
            emit statusChanged(tr("Controller: %1").arg(QString::fromUtf8(api.name(api.device)).left(120)));
        return;
    }
    for (int button = 0; button < int(api.previous.size()); ++button) {
        const bool pressed = api.button(api.device, button);
        if (pressed != api.previous[button] && m_inputActive)
            emit buttonChanged(button, pressed);
        api.previous[button] = pressed;
    }
    if (!m_inputActive)
        return;
    emit vehicleTriggers(normalizedTrigger(api.axis(api.device, 5)), normalizedTrigger(api.axis(api.device, 4)));
    const QVector3D movement(normalizedAxis(api.axis(api.device, 0)),
        normalizedAxis(api.axis(api.device, 5)) - normalizedAxis(api.axis(api.device, 4)),
        -normalizedAxis(api.axis(api.device, 1)));
    const QPointF look(normalizedAxis(api.axis(api.device, 2)), -normalizedAxis(api.axis(api.device, 3)));
    const bool pilot = pilotFlightActive && pilotFlightActive();
    const int rollLeft = pilot ? 9 : 13; // Pilot: shoulders. Explorer: D-pad.
    const int rollRight = pilot ? 10 : 14;
    const int boost = pilot ? 7 : 9; // Left-stick click frees both shoulders for roll.
    emit flightControls(int(api.button(api.device, rollRight)) - int(api.button(api.device, rollLeft)),
        api.button(api.device, 2));
    emit pilotInput(movement, look, api.button(api.device, boost));
}
