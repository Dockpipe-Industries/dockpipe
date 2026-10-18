#pragma once
#include <QObject>
#include <QImage>
#include <QPointer>
#include <QTimer>
#include <QUrl>

class QWebEngineView;

// Streams one user-selected browser video into a scene texture. The browser
// remains the media player: no URL extraction, downloader or alternate decoder.
class SpatialWebMedia final : public QObject {
    Q_OBJECT
public:
    explicit SpatialWebMedia(QWebEngineView *browser, QObject *parent = nullptr);
    void play(const QUrl &url);
    void togglePause();
    void releaseToPage();
    void resumePage();
    void stop();
    bool active() const { return m_active; }
    quint64 frameCount() const { return m_frames; }
    double mediaTime() const { return m_time; }
    bool paused() const { return m_paused; }
signals:
    void frameReady(const QImage &image);
    void statusChanged(const QString &status);
private:
    void begin();
    void probe();
    void freeze();
    void requestPlaybackGesture();
    QWebEngineView *m_browser;
    QTimer m_tick;
    QTimer m_deadline;
    QString m_probe;
    quint64 m_generation = 0;
    quint64 m_frames = 0;
    double m_time = -1;
    bool m_active = false;
    bool m_busy = false;
    bool m_requestPlay = false;
    bool m_paused = false;
    bool m_gestureAttempted = false;
};
