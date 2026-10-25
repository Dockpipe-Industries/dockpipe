#pragma once

#include <QByteArray>
#include <QImage>
#include <QString>

// Bounded transport for the local capture helper's concatenated JPEG frames.
// Retains only the latest decoded frame and one incomplete frame.
class DesktopCaptureFrameBuffer final {
public:
    static constexpr qsizetype MaximumFrameBytes = 4 * 1024 * 1024;
    bool append(const QByteArray &bytes, QImage &latest, QString &error);
    void clear();
private:
    QByteArray m_pending;
};
