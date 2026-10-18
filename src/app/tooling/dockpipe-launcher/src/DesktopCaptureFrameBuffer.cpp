#include "DesktopCaptureFrameBuffer.h"

#include <QBuffer>
#include <QImageReader>
#include <QObject>

bool DesktopCaptureFrameBuffer::append(const QByteArray &bytes, QImage &latest, QString &error)
{
    error.clear();
    if (bytes.size() > MaximumFrameBytes - m_pending.size()) {
        error = QObject::tr("Capture frame exceeded the 4 MiB transport limit.");
        return false;
    }
    m_pending.append(bytes);
    while (m_pending.size() >= 2) {
        if (!m_pending.startsWith(QByteArray::fromHex("ffd8"))) {
            error = QObject::tr("The capture helper returned an invalid JPEG stream.");
            return false;
        }
        const qsizetype end = m_pending.indexOf(QByteArray::fromHex("ffd9"), 2);
        if (end < 0)
            break;
        QByteArray encoded = m_pending.left(end + 2);
        m_pending.remove(0, end + 2);
        QBuffer input(&encoded);
        input.open(QIODevice::ReadOnly);
        QImageReader reader(&input, "JPEG");
        const QSize size = reader.size();
        if (size.isEmpty() || size.width() > 1280 || size.height() > 720) {
            error = QObject::tr("Capture frame exceeds the 1280 × 720 preview limit.");
            return false;
        }
        QImage frame = reader.read();
        if (frame.isNull()) {
            error = QObject::tr("The capture helper returned a damaged JPEG frame.");
            return false;
        }
        latest = std::move(frame);
    }
    return true;
}

void DesktopCaptureFrameBuffer::clear()
{
    m_pending.clear();
}
