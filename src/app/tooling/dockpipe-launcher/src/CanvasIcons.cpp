#include "CanvasIcons.h"

#include <QApplication>
#include <QIconEngine>
#include <QPainter>
#include <QPainterPath>
#include <QPalette>

namespace {
class StrokeIconEngine final : public QIconEngine {
public:
    explicit StrokeIconEngine(CanvasIcons::Symbol symbol) : m_symbol(symbol) {}
    QIconEngine *clone() const override { return new StrokeIconEngine(m_symbol); }

    QPixmap pixmap(const QSize &size, QIcon::Mode mode, QIcon::State state) override
    {
        QPixmap result(size);
        result.fill(Qt::transparent);
        QPainter painter(&result);
        paint(&painter, QRect(QPoint(), size), mode, state);
        return result;
    }

    void paint(QPainter *painter, const QRect &rect, QIcon::Mode mode, QIcon::State state) override
    {
        painter->save();
        const qreal extent = qMin(rect.width(), rect.height());
        painter->translate(rect.center().x() - extent / 2, rect.center().y() - extent / 2);
        painter->scale(extent / 24, extent / 24);
        painter->setRenderHint(QPainter::Antialiasing);
        const bool dark = QApplication::palette().color(QPalette::Window).lightness() < 128;
        QColor color(dark ? "#c2cbd7" : "#526078");
        if (m_symbol == CanvasIcons::Symbol::Play)
            color = QColor("#ffffff");
        if (mode == QIcon::Disabled)
            color = QColor(dark ? "#566070" : "#a7b0bf");
        else if (state == QIcon::On)
            color = QColor(dark ? "#91dbc6" : "#16745c");
        painter->setPen(QPen(color, 1.65, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
        painter->setBrush(Qt::NoBrush);
        auto line = [painter](qreal x1, qreal y1, qreal x2, qreal y2) {
            painter->drawLine(QPointF(x1, y1), QPointF(x2, y2));
        };
        auto path = [painter](std::initializer_list<QPointF> points) {
            QPainterPath shape;
            bool first = true;
            for (const auto &point : points) {
                if (first)
                    shape.moveTo(point);
                else
                    shape.lineTo(point);
                first = false;
            }
            painter->drawPath(shape);
        };
        using S = CanvasIcons::Symbol;
        switch (m_symbol) {
        case S::Add:
            line(12, 5, 12, 19);
            line(5, 12, 19, 12);
            break;
        case S::Minus:
            line(5, 12, 19, 12);
            break;
        case S::Fit:
            path({{9, 4}, {4, 4}, {4, 9}});
            path({{15, 4}, {20, 4}, {20, 9}});
            path({{4, 15}, {4, 20}, {9, 20}});
            path({{20, 15}, {20, 20}, {15, 20}});
            break;
        case S::Undo:
            path({{8, 4}, {3, 9}, {8, 14}});
            painter->drawArc(QRectF(3, 9, 17, 11), 0, 180 * 16);
            line(20, 14.5, 20, 18);
            break;
        case S::Code:
            path({{7, 6}, {2, 12}, {7, 18}});
            path({{17, 6}, {22, 12}, {17, 18}});
            line(14, 4, 10, 20);
            break;
        case S::Properties:
            line(6, 3, 6, 8);
            line(6, 12, 6, 21);
            line(18, 3, 18, 14);
            line(18, 18, 18, 21);
            painter->drawRoundedRect(QRectF(3, 8, 6, 4), 1, 1);
            painter->drawRoundedRect(QRectF(15, 14, 6, 4), 1, 1);
            break;
        case S::Sidebar:
            painter->drawRoundedRect(QRectF(3, 4, 18, 16), 2, 2);
            line(9, 4, 9, 20);
            break;
        case S::More:
            painter->setBrush(color);
            for (int x : {5, 12, 19})
                painter->drawEllipse(QPointF(x, 12), 0.9, 0.9);
            break;
        case S::Folder:
            path({{3, 7}, {3, 5}, {9, 5}, {12, 8}, {21, 8}, {21, 19}, {3, 19}, {3, 7}});
            break;
        case S::Workflow:
        case S::Parallel:
            painter->drawRoundedRect(QRectF(2, 9, 6, 6), 1.5, 1.5);
            painter->drawRoundedRect(QRectF(16, 3, 6, 6), 1.5, 1.5);
            painter->drawRoundedRect(QRectF(16, 15, 6, 6), 1.5, 1.5);
            path({{8, 12}, {12, 12}, {12, 6}, {16, 6}});
            path({{12, 12}, {12, 18}, {16, 18}});
            break;
        case S::Play:
            path({{7, 4}, {20, 12}, {7, 20}, {7, 4}});
            break;
        case S::Save:
            path({{4, 3}, {17, 3}, {21, 7}, {21, 21}, {3, 21}, {3, 3}, {4, 3}});
            painter->drawRect(QRectF(7, 3, 9, 6));
            painter->drawRect(QRectF(7, 14, 10, 7));
            break;
        case S::Terminal:
            path({{4, 5}, {10, 11}, {4, 17}});
            line(12, 18, 20, 18);
            break;
        case S::Keyboard:
            painter->drawRoundedRect(QRectF(2, 5, 20, 14), 2, 2);
            for (int y : {9, 12}) {
                for (int x : {6, 10, 14, 18})
                    line(x, y, x + 0.1, y);
            }
            line(7, 16, 17, 16);
            break;
        case S::Monitor:
            painter->drawRoundedRect(QRectF(3, 3, 18, 13), 2, 2);
            line(12, 16, 12, 21);
            line(7, 21, 17, 21);
            break;
        case S::Script:
            path({{5, 3}, {14, 3}, {20, 9}, {20, 21}, {5, 21}, {5, 3}});
            path({{14, 3}, {14, 9}, {20, 9}});
            line(9, 13, 16, 13);
            line(9, 17, 14, 17);
            break;
        case S::Close:
            line(6, 6, 18, 18);
            line(18, 6, 6, 18);
            break;
        case S::Check:
            path({{4, 12}, {9, 17}, {20, 6}});
            break;
        }
        painter->restore();
    }

private:
    CanvasIcons::Symbol m_symbol;
};
}

QIcon CanvasIcons::icon(Symbol symbol)
{
    return QIcon(new StrokeIconEngine(symbol));
}
