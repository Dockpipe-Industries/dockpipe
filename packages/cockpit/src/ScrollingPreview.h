#pragma once
#include <QImage>
#include <QRect>

// A bounded text raster, prepared once. Animation only copies its visible crop.
class ScrollingPreview {
public:
    static constexpr int maximumHeight = 4096;
    static constexpr int pauseMilliseconds = 3000;
    static constexpr int pixelsPerSecond = 10;
    ScrollingPreview() = default;
    explicit ScrollingPreview(const QImage &frame);
    ScrollingPreview(const QImage &frame, const QImage &content, const QRect &window);
    bool scrolling() const;
    int offsetAt(qint64 elapsedMilliseconds) const;
    QImage imageAt(qint64 elapsedMilliseconds) const;
private:
    QImage m_frame;
    QImage m_content;
    QRect m_window;
};
