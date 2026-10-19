#include "FileExplorerIcons.h"

#include "CanvasIcons.h"

#include <QApplication>
#include <QCoreApplication>
#include <QFileInfo>
#include <QIconEngine>
#include <QPainter>
#include <QPainterPath>
#include <QPalette>

namespace {
enum class FileFamily {
    Document, Markdown, Source, Configuration, Data, Image, Archive, Computer, Drive, Network
};

bool matches(const QString &value, std::initializer_list<const char *> names)
{
    for (const char *name : names) {
        if (value == QLatin1String(name))
            return true;
    }
    return false;
}

FileFamily familyForFile(const QFileInfo &file)
{
    const QString suffix = file.suffix().toLower();
    const QString name = file.fileName().toLower();
    if (matches(suffix, {"md", "markdown", "mdown", "mkd", "mdx"}))
        return FileFamily::Markdown;
    if (matches(suffix, {"json", "jsonc", "yaml", "yml", "toml", "ini", "cfg", "conf", "env"})
        || matches(name, {".gitignore", ".gitattributes", ".editorconfig", ".env", "dockerfile", "makefile", "cmakelists.txt"}))
        return FileFamily::Configuration;
    if (matches(suffix, {"csv", "tsv", "sql", "db", "sqlite", "sqlite3", "xlsx", "ods", "parquet"}))
        return FileFamily::Data;
    if (matches(suffix, {"png", "jpg", "jpeg", "gif", "webp", "svg", "bmp", "ico", "avif", "tiff"}))
        return FileFamily::Image;
    if (matches(suffix, {"zip", "tar", "gz", "bz2", "xz", "zst", "7z", "rar", "tgz"}))
        return FileFamily::Archive;
    if (matches(suffix, {"c", "h", "cc", "cpp", "cxx", "hpp", "cs", "go", "rs", "py", "js", "jsx",
                         "ts", "tsx", "java", "kt", "swift", "rb", "php", "sh", "bash", "zsh", "ps1",
                         "bat", "cmd", "html", "css", "scss", "qml", "xml", "vue", "svelte"}))
        return FileFamily::Source;
    return FileFamily::Document;
}

QColor familyAccent(FileFamily family, bool dark)
{
    switch (family) {
    case FileFamily::Markdown:
        return QColor(dark ? "#8bb9df" : "#3c719b");
    case FileFamily::Source:
        return QColor(dark ? "#a9a0dc" : "#7563a6");
    case FileFamily::Configuration:
        return QColor(dark ? "#c2b18b" : "#8b713c");
    case FileFamily::Data:
        return QColor(dark ? "#8ebfaf" : "#3c806a");
    case FileFamily::Image:
        return QColor(dark ? "#c69eb5" : "#9a5d80");
    case FileFamily::Archive:
        return QColor(dark ? "#c3a385" : "#93683e");
    default:
        return QColor(dark ? "#c2cbd7" : "#526078");
    }
}

class FileIconEngine final : public QIconEngine {
public:
    explicit FileIconEngine(FileFamily family) : m_family(family) {}

    QIconEngine *clone() const override
    {
        return new FileIconEngine(m_family);
    }

    QPixmap pixmap(const QSize &size, QIcon::Mode mode, QIcon::State state) override
    {
        QPixmap result(size);
        result.fill(Qt::transparent);
        QPainter painter(&result);
        paint(&painter, QRect(QPoint(), size), mode, state);
        return result;
    }

    QPixmap scaledPixmap(const QSize &size, QIcon::Mode mode, QIcon::State state, qreal scale) override
    {
        QPixmap result = pixmap(size * scale, mode, state);
        result.setDevicePixelRatio(scale);
        return result;
    }

    void paint(QPainter *painter, const QRect &rect, QIcon::Mode mode, QIcon::State) override
    {
        painter->save();
        const qreal extent = qMin(rect.width(), rect.height());
        painter->translate(rect.x() + (rect.width() - extent) / 2,
                           rect.y() + (rect.height() - extent) / 2);
        painter->scale(extent / 24, extent / 24);
        painter->setRenderHint(QPainter::Antialiasing);
        const QPalette palette = QApplication::palette();
        const bool dark = palette.color(QPalette::Window).lightness() < 128;
        QColor foreground(dark ? "#c2cbd7" : "#526078");
        QColor accent = familyAccent(m_family, dark);
        if (mode == QIcon::Disabled) {
            foreground = palette.color(QPalette::Disabled, QPalette::Text);
            accent = foreground;
        }
        painter->setPen(QPen(foreground, 1.4, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
        painter->setBrush(Qt::NoBrush);
        auto line = [painter](qreal x1, qreal y1, qreal x2, qreal y2) {
            painter->drawLine(QPointF(x1, y1), QPointF(x2, y2));
        };
        auto path = [painter](std::initializer_list<QPointF> points) {
            QPainterPath shape;
            bool first = true;
            for (const QPointF &point : points) {
                if (first)
                    shape.moveTo(point);
                else
                    shape.lineTo(point);
                first = false;
            }
            painter->drawPath(shape);
        };

        if (m_family == FileFamily::Computer) {
            painter->drawRoundedRect(QRectF(3, 4, 18, 13), 1.5, 1.5);
            line(12, 17, 12, 21);
            line(8, 21, 16, 21);
        } else if (m_family == FileFamily::Drive) {
            painter->drawRoundedRect(QRectF(3, 6, 18, 13), 2, 2);
            line(3, 14, 21, 14);
            line(17, 16.5, 18, 16.5);
        } else if (m_family == FileFamily::Network) {
            painter->drawRoundedRect(QRectF(8, 3, 8, 6), 1, 1);
            line(12, 9, 12, 14);
            line(5, 14, 19, 14);
            line(5, 14, 5, 17);
            line(19, 14, 19, 17);
            painter->drawRoundedRect(QRectF(2, 17, 6, 4), 1, 1);
            painter->drawRoundedRect(QRectF(16, 17, 6, 4), 1, 1);
        } else {
            QColor fill = accent;
            fill.setAlpha(mode == QIcon::Disabled ? 8 : 18);
            painter->setBrush(fill);
            path({{5, 2.5}, {14, 2.5}, {19, 7.5}, {19, 21.5}, {5, 21.5}, {5, 2.5}});
            painter->setBrush(Qt::NoBrush);
            path({{14, 2.5}, {14, 7.5}, {19, 7.5}});
            painter->setPen(QPen(accent, 1.5, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
            switch (m_family) {
            case FileFamily::Markdown:
                path({{8, 17.5}, {8, 11.5}, {11, 15}, {14, 11.5}, {14, 17.5}});
                line(16.5, 12.5, 16.5, 17);
                path({{15.5, 16}, {16.5, 17.5}, {17.5, 16}});
                break;
            case FileFamily::Source:
                path({{10, 11.5}, {7.5, 14.5}, {10, 17.5}});
                path({{14, 11.5}, {16.5, 14.5}, {14, 17.5}});
                break;
            case FileFamily::Configuration:
                line(8, 12, 16, 12);
                line(8, 17, 16, 17);
                painter->setBrush(palette.color(QPalette::Base));
                painter->drawEllipse(QPointF(10, 12), 1.5, 1.5);
                painter->drawEllipse(QPointF(14, 17), 1.5, 1.5);
                break;
            case FileFamily::Data:
                painter->drawRoundedRect(QRectF(8, 11, 8, 7), 0.5, 0.5);
                line(8, 14, 16, 14);
                line(11, 11, 11, 18);
                break;
            case FileFamily::Image:
                painter->drawEllipse(QPointF(9, 11.5), 1, 1);
                path({{7.5, 18}, {10.5, 14}, {12.5, 16}, {15, 12.5}, {17, 18}, {7.5, 18}});
                break;
            case FileFamily::Archive:
                for (int row = 10; row <= 16; row += 2)
                    line(11, row, 13, row);
                painter->drawRoundedRect(QRectF(10.5, 17, 3, 2.5), 0.5, 0.5);
                break;
            default:
                line(8, 12, 15, 12);
                line(8, 15, 15, 15);
                line(8, 18, 12, 18);
                break;
            }
        }
        painter->restore();
    }

private:
    FileFamily m_family;
};
}

FileExplorerIcons &FileExplorerIcons::instance()
{
    static FileExplorerIcons provider;
    return provider;
}

QIcon FileExplorerIcons::icon(const QFileInfo &file) const
{
    if (file.isRoot())
        return icon(Drive);
    if (file.isDir())
        return CanvasIcons::icon(CanvasIcons::Symbol::Folder);
    return QIcon(new FileIconEngine(familyForFile(file)));
}

QIcon FileExplorerIcons::icon(IconType type) const
{
    switch (type) {
    case Folder:
        return CanvasIcons::icon(CanvasIcons::Symbol::Folder);
    case Computer:
    case Desktop:
        return QIcon(new FileIconEngine(FileFamily::Computer));
    case Drive:
        return QIcon(new FileIconEngine(FileFamily::Drive));
    case Network:
        return QIcon(new FileIconEngine(FileFamily::Network));
    default:
        return QIcon(new FileIconEngine(FileFamily::Document));
    }
}

QString FileExplorerIcons::type(const QFileInfo &file) const
{
    // Qt 6.2 needs a nonempty type before it inserts newly discovered model rows.
    if (file.isDir())
        return QCoreApplication::translate("FileExplorerIcons", "Folder");
    return QCoreApplication::translate("FileExplorerIcons", "File");
}
