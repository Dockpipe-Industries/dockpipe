#pragma once
#include <QObject>
#include <QTimer>
#include <memory>

// Deterministic, original procedural sound. No samples, network or file I/O.
class DirtBikeSynth {
public:
    struct Mix {
        float throttle = 0;
        float speed = 0; // cm/s
        float skid = 0;
        float impact = 0; // one-shot normalized landing impulse
        bool airborne = false;
        bool engineRunning = true;
        float engineRpm = 0; // Positive values come from the bike powertrain.
    };
    void render(float *samples, int count, const Mix &mix);
private:
    float m_phase = 0;
    float m_rpm = 1600;
    float m_noise = 0;
    float m_impact = 0;
    unsigned m_random = 0x1256a3;
};

// Optional SDL2 playback, using the controller runtime already supported by
// Cockpit. One mono 48 kHz stream, at most 64 ms queued; no microphone access.
class DirtBikeSound final : public QObject {
public:
    DirtBikeSound();
    ~DirtBikeSound() override;
    bool start();
    void stop();
    void update(const DirtBikeSynth::Mix &mix);
    bool playing() const { return m_timer.isActive(); }
private:
    struct Native;
    std::unique_ptr<Native> m_native;
    DirtBikeSynth m_synth;
    DirtBikeSynth::Mix m_mix;
    QTimer m_timer;
    void fill();
};
