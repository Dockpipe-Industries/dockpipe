#include "DesktopCaptureFrameBuffer.h"
#include <QBuffer>
#include <QTest>

class DesktopCaptureFrameBufferTest final : public QObject {
    Q_OBJECT
private slots:
    void acceptsSplitFramesAndRetainsLatest();
    void rejectsMalformedOversizedAndOverBudgetFrames();
};

namespace {
QByteArray jpeg(const QSize &size, const QColor &color)
{
    QImage image(size, QImage::Format_RGB32);
    image.fill(color);
    QByteArray bytes;
    QBuffer output(&bytes);
    output.open(QIODevice::WriteOnly);
    image.save(&output, "JPEG");
    return bytes;
}
}

void DesktopCaptureFrameBufferTest::acceptsSplitFramesAndRetainsLatest()
{
    DesktopCaptureFrameBuffer frames;
    QImage latest;
    QString error;
    const auto first = jpeg(QSize(1280, 720), Qt::red);
    const auto second = jpeg(QSize(320, 240), Qt::blue);
    QVERIFY(frames.append(first.left(7), latest, error));
    QVERIFY(latest.isNull());
    QVERIFY(frames.append(first.mid(7) + second, latest, error));
    QCOMPARE(latest.size(), QSize(320, 240));
    QVERIFY(latest.pixelColor(5, 5).blue() > 200);
    frames.clear();
    QVERIFY(frames.append(first, latest, error));
    QCOMPARE(latest.size(), QSize(1280, 720));
}

void DesktopCaptureFrameBufferTest::rejectsMalformedOversizedAndOverBudgetFrames()
{
    DesktopCaptureFrameBuffer frames;
    QImage latest;
    QString error;
    QVERIFY(!frames.append("not a JPEG", latest, error));
    QVERIFY(!error.isEmpty());
    frames.clear();
    QVERIFY(!frames.append(jpeg(QSize(1281, 720), Qt::red), latest, error));
    QVERIFY(error.contains("1280"));
    frames.clear();
    QVERIFY(!frames.append(QByteArray::fromHex("ffd800112233ffd9"), latest, error));
    frames.clear();
    QByteArray incomplete(DesktopCaptureFrameBuffer::MaximumFrameBytes, '\0');
    incomplete[0] = char(0xff);
    incomplete[1] = char(0xd8);
    QVERIFY(frames.append(incomplete, latest, error));
    QVERIFY(!frames.append("x", latest, error));
    QVERIFY(error.contains("4 MiB"));
}

QTEST_GUILESS_MAIN(DesktopCaptureFrameBufferTest)
#include "DesktopCaptureFrameBufferTest.moc"
