#include "DirtBikeSound.h"
#include <QLibrary>
#include <QtEndian>
#include <array>
#include <cmath>

namespace {
float unit(float value)
{
    return std::isfinite(value) ? qBound(0.f, value, 1.f) : 0;
}
// SDL2 public audio ABI (SDL_audio.h); same optional dynamic loading approach
// as CockpitController. Fixed float format lets SDL own device conversion.
struct AudioSpec {
    int frequency = 48000;
    quint16 format = Q_BYTE_ORDER == Q_LITTLE_ENDIAN ? 0x8120 : 0x9120;
    quint8 channels = 1;
    quint8 silence = 0;
    quint16 samples = 512;
    quint16 padding = 0;
    quint32 size = 0;
    void (*callback)(void *, quint8 *, int) = nullptr;
    void *user = nullptr;
};
}

void DirtBikeSynth::render(float *samples, int count, const Mix &mix)
{
    const float throttle = unit(mix.throttle);
    const float speed = std::isfinite(mix.speed) ? qBound(0.f, std::abs(mix.speed) / 2300, 1.f) : 0;
    const float skid = mix.airborne ? 0 : unit(mix.skid);
    // Simulated gearing gives the exhaust a rise/drop through the speed range.
    const float gearPhase = std::fmod(speed * 3.7f, 1.f);
    float targetRpm = mix.airborne ? 2000 + throttle * 7400 : 1600 + gearPhase * 4200 + throttle * 2500;
    if (std::isfinite(mix.engineRpm) && mix.engineRpm > 0)
        targetRpm = qBound(1600.f, mix.engineRpm, 10500.f);
    m_impact = qMax(m_impact, unit(mix.impact));
    for (int index = 0; index < count; ++index) {
        m_rpm += (targetRpm - m_rpm) * .0004f;
        m_phase += m_rpm / (120 * 48000);
        m_phase -= std::floor(m_phase);
        m_random ^= m_random << 13;
        m_random ^= m_random >> 17;
        m_random ^= m_random << 5;
        const float white = float(m_random & 65535) / 32768 - 1;
        m_noise += (white - m_noise) * .075f;
        const float angle = m_phase * 6.283185307f;
        const float exhaust = std::sin(angle) + .45f * std::sin(angle * 2 + .4f)
            + .24f * std::sin(angle * 4) + .12f * white;
        const float engine = std::tanh(exhaust * (1.1f + throttle)) * (.065f + .09f * throttle);
        const float gravel = (white - m_noise) * skid * .10f;
        const float wind = m_noise * speed * .15f;
        const float landing = m_noise * m_impact * .65f;
        samples[index] = qBound(-.7f, (mix.engineRunning ? engine : 0) + gravel + wind + landing, .7f);
        m_impact *= .99945f;
    }
}

struct DirtBikeSound::Native {
#if defined(Q_OS_LINUX)
    QLibrary library{"SDL2-2.0", 0};
#else
    QLibrary library{"SDL2"};
#endif
    int (*init)(quint32) = nullptr;
    void (*quit)(quint32) = nullptr;
    quint32 (*open)(const char *, int, const AudioSpec *, AudioSpec *, int) = nullptr;
    void (*close)(quint32) = nullptr;
    void (*pause)(quint32, int) = nullptr;
    int (*queue)(quint32, const void *, quint32) = nullptr;
    quint32 (*queued)(quint32) = nullptr;
    void (*clear)(quint32) = nullptr;
    quint32 device = 0;
    bool initialized = false;
    bool attempted = false;
    template<typename Function>
    bool resolve(Function &function, const char *symbol)
    {
        function = reinterpret_cast<Function>(library.resolve(symbol));
        return function != nullptr;
    }
};

DirtBikeSound::DirtBikeSound() : m_native(std::make_unique<Native>())
{
    m_timer.setInterval(20);
    connect(&m_timer, &QTimer::timeout, this, [this]() { fill(); });
}
DirtBikeSound::~DirtBikeSound()
{
    stop();
    if (m_native->device)
        m_native->close(m_native->device);
    if (m_native->initialized)
        m_native->quit(0x10);
}

bool DirtBikeSound::start()
{
    auto &api = *m_native;
    if (!api.attempted) {
        api.attempted = true;
        if (!api.library.load() || !api.resolve(api.init, "SDL_InitSubSystem")
            || !api.resolve(api.quit, "SDL_QuitSubSystem") || !api.resolve(api.open, "SDL_OpenAudioDevice")
            || !api.resolve(api.close, "SDL_CloseAudioDevice") || !api.resolve(api.pause, "SDL_PauseAudioDevice")
            || !api.resolve(api.queue, "SDL_QueueAudio") || !api.resolve(api.queued, "SDL_GetQueuedAudioSize")
            || !api.resolve(api.clear, "SDL_ClearQueuedAudio") || api.init(0x10) < 0)
            return false;
        api.initialized = true;
        const AudioSpec desired;
        AudioSpec obtained;
        api.device = api.open(nullptr, 0, &desired, &obtained, 0);
        if (api.device && (obtained.frequency != desired.frequency || obtained.format != desired.format || obtained.channels != 1)) {
            api.close(api.device);
            api.device = 0;
        }
    }
    if (!api.device)
        return false;
    if (!m_timer.isActive()) {
        fill();
        if (!api.device)
            return false;
        api.pause(api.device, 0);
        m_timer.start();
    }
    return true;
}

void DirtBikeSound::stop()
{
    m_timer.stop();
    if (m_native->device) {
        m_native->pause(m_native->device, 1);
        m_native->clear(m_native->device);
    }
    m_mix = {};
    m_synth = {};
}

void DirtBikeSound::update(const DirtBikeSynth::Mix &mix)
{
    const float pendingImpact = m_mix.impact;
    m_mix = mix;
    m_mix.impact = qMax(pendingImpact, mix.impact);
}

void DirtBikeSound::fill()
{
    auto &api = *m_native;
    std::array<float, 512> samples;
    for (int block = 0; block < 6 && api.queued(api.device) < 3072 * sizeof(float); ++block) {
        m_synth.render(samples.data(), int(samples.size()), m_mix);
        m_mix.impact = 0;
        if (api.queue(api.device, samples.data(), sizeof(samples)) < 0) {
            stop();
            api.close(api.device);
            api.device = 0;
            return;
        }
    }
}
