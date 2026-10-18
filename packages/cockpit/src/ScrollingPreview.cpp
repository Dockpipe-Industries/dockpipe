#include "ScrollingPreview.h"
#include <QPainter>
#include <algorithm>

ScrollingPreview::ScrollingPreview(const QImage &frame) : m_frame(frame)
{
}

ScrollingPreview::ScrollingPreview(const QImage &frame, const QImage &content, const QRect &window)
    : m_frame(frame)
{
    if (!frame.rect().contains(window) || window.isEmpty() || content.isNull()
        || content.height() > maximumHeight || content.width() != window.width())
        return;
    m_content = content;
    m_window = window;
}

bool ScrollingPreview::scrolling() const
{
    return m_content.height() > m_window.height();
}

int ScrollingPreview::offsetAt(qint64 elapsedMilliseconds) const
{
    if (!scrolling())
        return 0;
    const int distance = m_content.height() - m_window.height();
    const qint64 travelMilliseconds = qint64(distance) * 1000 / pixelsPerSecond;
    const qint64 phase = std::max(qint64(0), elapsedMilliseconds)
        % (travelMilliseconds + 2 * pauseMilliseconds);
    if (phase <= pauseMilliseconds)
        return 0;
    return int(std::min(qint64(distance), (phase - pauseMilliseconds) * pixelsPerSecond / 1000));
}

QImage ScrollingPreview::imageAt(qint64 elapsedMilliseconds) const
{
    if (m_content.isNull())
        return m_frame;
    QImage image = m_frame;
    QPainter painter(&image);
    painter.setClipRect(m_window);
    painter.drawImage(m_window.topLeft(), m_content,
        QRect(0, offsetAt(elapsedMilliseconds), m_window.width(), m_window.height()));
    return image;
}
