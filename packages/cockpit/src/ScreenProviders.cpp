#include "ScreenProviders.h"
#include <QFile>
#include <QFileInfo>
#include <QImageReader>
#include <QPainter>
#include <QFontDatabase>
#include <QStringDecoder>

ScreenCapabilities ScreenProviders::capabilities(const ScreenBinding &binding, const QStringList &sceneSources)
{
    if (binding.provider == "text")
        return {"Text / notice", true, false, false, "Passive screen"};
    if (binding.provider == "file")
        return {"Local file / image", true, false, false, "Static preview · reapply input to refresh"};
    if (binding.provider == "application") {
        const bool available = !binding.application.windowClass.isEmpty() || !binding.application.desktopId.isEmpty();
        return {"Application", available, available, true,
            available ? "Select to connect · requires App control" : "Choose a running or installed application"};
    }
    if (binding.provider == "surface") {
        const bool available = sceneSources.contains(binding.target);
        return {"Scene content surface", available, available, true,
            available ? "Shares the source's pixels and interaction" : "Source unavailable in this environment"};
    }
    return {binding.provider, false, false, false, "No provider is connected for this input. The binding is saved for future support."};
}
QImage ScreenProviders::preview(const ScreenBinding &binding, const ScreenCapabilities &capabilities)
{
    QString text = capabilities.available ? binding.text : capabilities.notice;
    if (capabilities.available && binding.provider == "file") {
        QFile file(binding.target);
        if (!QFileInfo(binding.target).isFile() || !file.open(QIODevice::ReadOnly) || file.size() > 4 * 1024 * 1024) {
            text = "Cannot preview this file (readable files up to 4 MiB).";
        } else {
            QImageReader reader(&file);
            const auto size = reader.size();
            if (size.isValid()) {
                if (size.width() > 2048 || size.height() > 2048) {
                    text = "Image exceeds the 2048 × 2048 decoding budget.";
                } else {
                    const auto image = reader.read();
                    if (!image.isNull())
                        return image.scaled(512, 320, Qt::KeepAspectRatio, Qt::SmoothTransformation);
                    text = "Image could not be decoded.";
                }
            } else {
                file.seek(0);
                const auto bytes = file.read(65537);
                QStringDecoder decoder(QStringDecoder::Utf8);
                text = decoder(bytes.first(qMin(bytes.size(), qsizetype(65536))));
                if (decoder.hasError() || bytes.contains('\0'))
                    text = "Binary file · no text preview";
                else if (bytes.size() > 65536)
                    text += "\n[Preview limited to 64 KiB]";
            }
        }
    }
    QImage image(512, 320, QImage::Format_ARGB32_Premultiplied);
    image.fill(QColor("#0b1724"));
    QPainter painter(&image);
    painter.fillRect(0, 0, 512, 36, QColor("#254352"));
    painter.setPen(QColor("#b6e6db"));
    painter.setFont(QFontDatabase::systemFont(QFontDatabase::GeneralFont));
    painter.drawText(QRect(16, 0, 480, 36), Qt::AlignVCenter, capabilities.label);
    painter.setPen(QColor("#e0e9f0"));
    // Rasterized plain text only. Repository/file text never becomes HTML.
    painter.drawText(QRect(20, 50, 472, 230), Qt::TextWordWrap, text.left(4096));
    painter.setPen(QColor("#829eaf"));
    painter.drawText(QRect(16, 285, 480, 28), Qt::AlignVCenter, capabilities.notice.left(85));
    return image;
}

QImage ScreenProviders::marquee(const ScreenBinding &binding)
{
    if (binding.provider != "text")
        return preview(binding, capabilities(binding, {}));
    QImage image(512, 138, QImage::Format_ARGB32_Premultiplied);
    image.fill(QColor("#21172e"));
    QPainter painter(&image);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.setPen(QPen(QColor("#74e9d6"), 4));
    painter.drawRect(image.rect().adjusted(4, 4, -5, -5));
    painter.fillRect(20, 118, 472, 5, QColor("#f26fa8"));
    QFont font = QFontDatabase::systemFont(QFontDatabase::GeneralFont);
    font.setBold(true);
    font.setPixelSize(38);
    painter.setFont(font);
    painter.setPen(QColor("#fff0cb"));
    painter.drawText(image.rect().adjusted(20, 10, -20, -20), Qt::AlignCenter,
        painter.fontMetrics().elidedText(binding.text.left(4096).simplified(), Qt::ElideRight, 472));
    return image;
}
