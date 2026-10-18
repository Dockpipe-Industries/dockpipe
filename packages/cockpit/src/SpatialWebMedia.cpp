#include "SpatialWebMedia.h"
#include "SpatialWebDocument.h"
#include <QFile>
#include <QApplication>
#include <QMouseEvent>
#ifdef DOCKPIPE_SPATIAL_WEB
#include <QWebEnginePage>
#include <QWebEngineScript>
#include <QWebEngineSettings>
#include <QWebEngineView>
#endif

SpatialWebMedia::SpatialWebMedia(QWebEngineView *browser, QObject *parent)
    : QObject(parent), m_browser(browser)
{
    m_tick.setInterval(83); // One 512x288 stream, at most twelve captures/second.
    connect(&m_tick, &QTimer::timeout, this, &SpatialWebMedia::probe);
    m_deadline.setSingleShot(true);
    m_deadline.setInterval(15000);
    connect(&m_deadline, &QTimer::timeout, this, [this]() {
        freeze();
        emit statusChanged(tr("The site has not provided a playable video. Its sign-in, consent or playback requirements may need Live page."));
    });
    QFile script(":/web/media.js");
    if (script.open(QIODevice::ReadOnly))
        m_probe = QString::fromUtf8(script.readAll());
#ifdef DOCKPIPE_SPATIAL_WEB
    connect(browser, &QWebEngineView::loadFinished, this, [this](bool success) {
        if (!m_active)
            return;
        if (success) {
            begin();
        } else {
            freeze();
            emit statusChanged(tr("The video page could not load. The theatre is preserved."));
        }
    });
#endif
}

void SpatialWebMedia::play(const QUrl &url)
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!SpatialWebDocument::allowedUrl(url) || m_probe.isEmpty())
        return;
    stop();
    m_active = true;
    m_frames = 0;
    m_time = -1;
    m_paused = false;
    m_requestPlay = true;
    m_gestureAttempted = false;
    // Only explicit selection of a film permits autoplay. Ordinary background
    // page loads retain the browser's user-gesture requirement and stay muted.
    m_browser->settings()->setAttribute(QWebEngineSettings::PlaybackRequiresUserGesture, false);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->show();
    m_browser->lower();
    m_browser->page()->setAudioMuted(false);
    m_deadline.start();
    emit statusChanged(tr("Loading the selected film on the centre screen…"));
    if (m_browser->url() == url)
        begin();
    else
        m_browser->load(url);
#endif
}

void SpatialWebMedia::releaseToPage()
{
    // Hand ownership to the website without pausing, seeking or navigating.
    ++m_generation;
    m_tick.stop();
    m_deadline.stop();
    m_busy = false;
    m_requestPlay = false;
    m_active = false;
#ifdef DOCKPIPE_SPATIAL_WEB
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->settings()->setAttribute(QWebEngineSettings::PlaybackRequiresUserGesture, true);
    m_browser->page()->runJavaScript(
        "(() => { const v = document.querySelector('video'); if (v && v.__dockpipeStyle !== undefined) { v.style.cssText = v.__dockpipeStyle; delete v.__dockpipeStyle; } window.__dockpipeMediaError = ''; })()",
        QWebEngineScript::ApplicationWorld);
#endif
}

void SpatialWebMedia::resumePage()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_probe.isEmpty() || !SpatialWebDocument::allowedUrl(m_browser->url()))
        return;
    ++m_generation;
    m_busy = false;
    m_active = true;
    m_requestPlay = false; // Observe the website's current playing/paused state.
    m_gestureAttempted = true; // Returning to a room is not an autoplay gesture.
    m_frames = 0;
    m_time = -1;
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->show();
    m_browser->lower();
    m_browser->page()->setAudioMuted(false);
    m_deadline.start();
    begin();
#endif
}

void SpatialWebMedia::begin()
{
    if (!m_active)
        return;
    m_tick.start();
    probe();
}

void SpatialWebMedia::probe()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_active || m_busy)
        return;
    m_busy = true;
    const auto generation = m_generation;
    const QPointer<SpatialWebMedia> guard(this);
    m_browser->page()->runJavaScript(m_probe, QWebEngineScript::ApplicationWorld, [guard, generation](const QVariant &value) {
        if (!guard || guard->m_generation != generation || !guard->m_active)
            return;
        guard->m_busy = false;
        const auto data = value.toMap();
        if (!data.value("found").toBool())
            return;
        if (guard->m_requestPlay) {
            guard->m_requestPlay = false;
            guard->m_browser->page()->runJavaScript(
                "(() => { const v = document.querySelector('video'); if (v) { v.play().catch(e => { window.__dockpipeMediaError = e.name; }); } })()",
                QWebEngineScript::ApplicationWorld);
            return;
        }
        const QString error = data.value("error").toString();
        if (error == "NotAllowedError" && !guard->m_gestureAttempted) {
            guard->m_gestureAttempted = true;
            guard->requestPlaybackGesture();
            return;
        }
        if (!error.isEmpty()) {
            guard->freeze();
            emit guard->statusChanged(tr("The site could not play this video (%1). The theatre remains open; Live page provides the site's controls.").arg(error.left(80)));
            return;
        }
        if (data.value("ready").toInt() < 2)
            return;
        const QRectF bounds(data.value("x").toDouble(), data.value("y").toDouble(),
            data.value("width").toDouble(), data.value("height").toDouble());
        const QRect crop = bounds.intersected(QRectF(guard->m_browser->rect())).toAlignedRect();
        if (crop.isEmpty())
            return;
        const QImage frame = guard->m_browser->grab(crop).toImage().scaled(512, 288, Qt::KeepAspectRatio, Qt::SmoothTransformation);
        if (frame.isNull())
            return;
        guard->m_deadline.stop();
        const bool wasPaused = guard->m_paused;
        guard->m_time = data.value("time").toDouble();
        guard->m_paused = data.value("paused").toBool();
        ++guard->m_frames;
        emit guard->frameReady(frame);
        if (!guard->m_paused && (guard->m_frames == 1 || wasPaused))
            emit guard->statusChanged(tr("Playing on the centre screen · select another wall poster to change films."));
        if (guard->m_paused || data.value("ended").toBool()) {
            guard->freeze();
            emit guard->statusChanged(tr("Playback paused or finished · use Play / pause to resume."));
        }
    });
#endif
}

void SpatialWebMedia::requestPlaybackGesture()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    // A native poster click is outside Chromium's user-activation scope. Forward
    // that explicit play intent once, only to a visible video or play button.
    // Consent/sign-in overlays are never clicked by this fallback.
    m_busy = true;
    const auto generation = m_generation;
    const QPointer<SpatialWebMedia> guard(this);
    m_browser->page()->runJavaScript(R"JS((() => {
        const video = document.querySelector('video');
        if (!video) return {};
        const player = video.closest('#movie_player') || video.parentElement;
        const button = player && player.querySelector('button.ytp-large-play-button,button[aria-label="Play"]');
        const target = button || video;
        const rect = target.getBoundingClientRect();
        const x = rect.x + rect.width / 2;
        const y = rect.y + rect.height / 2;
        const hit = document.elementFromPoint(x, y);
        if (rect.width < 1 || rect.height < 1 || !hit || (hit !== target && !target.contains(hit))) return {};
        window.__dockpipeMediaError = '';
        return {x, y, allowed: true};
    })())JS", QWebEngineScript::ApplicationWorld, [guard, generation](const QVariant &value) {
        if (!guard || guard->m_generation != generation || !guard->m_active)
            return;
        guard->m_busy = false;
        const auto point = value.toMap();
        auto *receiver = guard->m_browser->focusProxy();
        if (!point.value("allowed").toBool() || !receiver) {
            guard->freeze();
            emit guard->statusChanged(tr("The website requires its own playback controls. The theatre is preserved; open Live page to continue."));
            return;
        }
        const QPoint browserPoint(qRound(point.value("x").toDouble()), qRound(point.value("y").toDouble()));
        const QPoint position = receiver->mapFrom(guard->m_browser, browserPoint);
        QMouseEvent press(QEvent::MouseButtonPress, position, receiver->mapToGlobal(position),
            Qt::LeftButton, Qt::LeftButton, Qt::NoModifier);
        QMouseEvent release(QEvent::MouseButtonRelease, position, receiver->mapToGlobal(position),
            Qt::LeftButton, Qt::NoButton, Qt::NoModifier);
        const QPointer<QWidget> previousFocus = QApplication::focusWidget();
        QApplication::sendEvent(receiver, &press);
        QApplication::sendEvent(receiver, &release);
        if (previousFocus)
            previousFocus->setFocus(Qt::OtherFocusReason);
    });
#endif
}

void SpatialWebMedia::freeze()
{
    ++m_generation;
    m_busy = false;
    m_tick.stop();
    m_deadline.stop();
#ifdef DOCKPIPE_SPATIAL_WEB
    m_browser->hide();
    m_browser->page()->setVisible(false);
    m_browser->page()->setAudioMuted(true);
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
#endif
}

void SpatialWebMedia::togglePause()
{
#ifdef DOCKPIPE_SPATIAL_WEB
    if (!m_active)
        return;
    m_browser->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    m_browser->show();
    m_browser->lower();
    m_browser->page()->setAudioMuted(false);
    m_deadline.start();
    m_browser->page()->runJavaScript(
        "(() => { const v = document.querySelector('video'); if (v) { if (v.paused) v.play().catch(e => { window.__dockpipeMediaError = e.name; }); else v.pause(); } })()",
        QWebEngineScript::ApplicationWorld);
    m_tick.start();
#endif
}

void SpatialWebMedia::stop()
{
    ++m_generation;
    m_tick.stop();
    m_deadline.stop();
    m_busy = false;
    m_requestPlay = false;
#ifdef DOCKPIPE_SPATIAL_WEB
    if (m_active) {
        m_browser->page()->runJavaScript(
            "(() => { const v = document.querySelector('video'); if (v) { v.pause(); if (v.__dockpipeStyle !== undefined) v.style.cssText = v.__dockpipeStyle; } window.__dockpipeMediaError = ''; })()",
            QWebEngineScript::ApplicationWorld);
        m_browser->page()->setAudioMuted(true);
        m_browser->settings()->setAttribute(QWebEngineSettings::PlaybackRequiresUserGesture, true);
    }
#endif
    m_active = false;
}
