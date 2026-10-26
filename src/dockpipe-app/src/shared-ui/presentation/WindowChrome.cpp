#include "WindowChrome.h"

#include <QApplication>
#include <QDialog>
#include <QFileDialog>
#include <QHBoxLayout>
#include <QLabel>
#include <QLayout>
#include <QMainWindow>
#include <QMenuBar>
#include <QMessageBox>
#include <QMouseEvent>
#include <QPainter>
#include <QPainterPath>
#include <QToolButton>
#include <QWindow>

namespace {
class WindowButton final : public QToolButton {
public:
    enum Action { Minimize, Maximize, Close };

    WindowButton(Action action, QWidget *window, QWidget *parent)
        : QToolButton(parent), m_action(action), m_window(window)
    {
        setObjectName(action == Close ? "windowClose" : "windowControl");
        setFixedSize(36, 32);
        setFocusPolicy(Qt::StrongFocus);
        setCursor(Qt::ArrowCursor);
        QString label = tr("Maximize or restore");
        if (action == Close)
            label = tr("Close");
        else if (action == Minimize)
            label = tr("Minimize");
        setToolTip(label);
        setAccessibleName(label);
        connect(this, &QToolButton::clicked, this, [this]() {
            if (m_action == Close)
                m_window->close();
            else if (m_action == Minimize)
                m_window->showMinimized();
            else if (m_window->isMaximized())
                m_window->showNormal();
            else
                m_window->showMaximized();
        });
    }

protected:
    void paintEvent(QPaintEvent *event) override
    {
        QToolButton::paintEvent(event);
        QPainter painter(this);
        painter.setRenderHint(QPainter::Antialiasing);
        QColor color = palette().color(QPalette::WindowText);
        if (m_action == Close && underMouse())
            color = Qt::white;
        painter.setPen(QPen(color, 1.3, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
        painter.translate(width() / 2.0, height() / 2.0);
        if (m_action == Close) {
            painter.drawLine(QPointF(-4, -4), QPointF(4, 4));
            painter.drawLine(QPointF(4, -4), QPointF(-4, 4));
        } else if (m_action == Minimize) {
            painter.drawLine(QPointF(-5, 3), QPointF(5, 3));
        } else if (m_window->isMaximized()) {
            painter.drawRect(QRectF(-5, -2, 7, 7));
            painter.drawLine(QPointF(-2, -5), QPointF(5, -5));
            painter.drawLine(QPointF(5, -5), QPointF(5, 2));
        } else {
            painter.drawRoundedRect(QRectF(-4.5, -4.5, 9, 9), 1, 1);
        }
    }

private:
    Action m_action;
    QWidget *m_window;
};

class WindowChrome final : public QObject {
public:
    explicit WindowChrome(QWidget *window) : QObject(window), m_window(window)
    {
        window->setProperty("launcherChrome", true);
        window->setWindowFlag(Qt::FramelessWindowHint);
        window->setMouseTracking(true);
        qApp->installEventFilter(this);
    }

protected:
    bool eventFilter(QObject *object, QEvent *event) override
    {
        if (object == m_window) {
            if (event->type() == QEvent::Show)
                prepare();
            if (event->type() == QEvent::Resize || event->type() == QEvent::Show)
                arrange();
            if (event->type() == QEvent::WindowTitleChange && m_title)
                m_title->setText(m_window->windowTitle());
            if (event->type() == QEvent::Hide || event->type() == QEvent::WindowDeactivate)
                finishDrag();
        }
        auto *target = qobject_cast<QWidget *>(object);
        if (!target || !m_window->isVisible())
            return false;
        const bool inside = target == m_window || m_window->isAncestorOf(target);
        if (!inside || target->window() != m_window)
            return false;
        const bool header = target == m_header || target == m_title;
        if (event->type() == QEvent::MouseButtonDblClick && header && m_mainWindow) {
            auto *mouse = static_cast<QMouseEvent *>(event);
            if (mouse->button() != Qt::LeftButton || menuActionAt(mouse->globalPosition().toPoint()))
                return false;
            if (m_window->isMaximized())
                m_window->showNormal();
            else
                m_window->showMaximized();
            return true;
        }
        if (event->type() == QEvent::MouseButtonPress) {
            auto *mouse = static_cast<QMouseEvent *>(event);
            if (mouse->button() != Qt::LeftButton)
                return false;
            const QPoint global = mouse->globalPosition().toPoint();
            const QPoint point = m_window->mapFromGlobal(global);
            const Qt::Edges edges = resizeEdges(point);
            if (edges && m_window->windowHandle()
                && m_window->windowHandle()->startSystemResize(edges))
                return true;
            if (!header || menuActionAt(global))
                return false;
            if (m_window->windowHandle() && m_window->windowHandle()->startSystemMove())
                return true;
            // Offscreen tests and platforms without native interactive moving.
            if (!m_window->isMaximized()) {
                m_dragOffset = global - m_window->pos();
                m_dragging = true;
                m_window->grabMouse();
            }
            return true;
        }
        if (event->type() == QEvent::MouseMove && m_dragging) {
            const auto *mouse = static_cast<QMouseEvent *>(event);
            m_window->move(mouse->globalPosition().toPoint() - m_dragOffset);
            return true;
        }
        if (event->type() == QEvent::MouseButtonRelease && m_dragging) {
            finishDrag();
            return true;
        }
        return false;
    }

private:
    void prepare()
    {
        if (m_header)
            return;
        m_mainWindow = qobject_cast<QMainWindow *>(m_window);
        if (m_mainWindow) {
            auto *menu = m_mainWindow->menuBar();
            menu->setNativeMenuBar(false);
            m_header = menu;
            auto *controls = new QWidget(menu);
            auto *row = new QHBoxLayout(controls);
            row->setContentsMargins(8, 2, 6, 2);
            row->setSpacing(2);
            row->addWidget(new WindowButton(WindowButton::Minimize, m_window, controls));
            row->addWidget(new WindowButton(WindowButton::Maximize, m_window, controls));
            row->addWidget(new WindowButton(WindowButton::Close, m_window, controls));
            menu->setCornerWidget(controls, Qt::TopRightCorner);
            controls->show();
        } else {
            if (auto *message = qobject_cast<QMessageBox *>(m_window))
                refineMessageIcon(message);
            m_header = new QWidget(m_window);
            m_header->setObjectName("dialogHeader");
            auto *row = new QHBoxLayout(m_header);
            row->setContentsMargins(0, 0, 0, 0);
            m_title = new QLabel(m_window->windowTitle(), m_header);
            m_title->setObjectName("dialogHeading");
            m_title->setTextFormat(Qt::PlainText);
            m_title->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
            row->addWidget(m_title, 1);
            row->addWidget(new WindowButton(WindowButton::Close, m_window, m_header));
            if (auto *layout = m_window->layout()) {
                layout->setContentsMargins(24, 72, 24, 24);
                layout->setSpacing(14);
                layout->activate();
            }
            m_header->show();
        }
        m_header->setCursor(Qt::ArrowCursor);
    }

    void arrange()
    {
        if (!m_header || m_mainWindow)
            return;
        m_header->setGeometry(24, 16, m_window->width() - 48, 36);
        m_header->raise();
        QPainterPath outline;
        outline.addRoundedRect(QRectF(m_window->rect()), 14, 14);
        m_window->setMask(QRegion(outline.toFillPolygon().toPolygon()));
    }

    void refineMessageIcon(QMessageBox *message)
    {
        const auto icon = message->icon();
        if (icon == QMessageBox::NoIcon)
            return;
        const qreal scale = message->devicePixelRatioF();
        QPixmap pixels(QSize(32, 32) * scale);
        pixels.setDevicePixelRatio(scale);
        pixels.fill(Qt::transparent);
        QPainter painter(&pixels);
        painter.setRenderHint(QPainter::Antialiasing);
        QColor accent("#79aee8");
        if (icon == QMessageBox::Warning)
            accent = QColor("#d29c43");
        else if (icon == QMessageBox::Critical)
            accent = QColor("#de6b78");
        painter.setPen(QPen(accent, 1.7, Qt::SolidLine, Qt::RoundCap, Qt::RoundJoin));
        QColor fill = accent;
        fill.setAlpha(22);
        painter.setBrush(fill);
        painter.drawEllipse(QRectF(3, 3, 26, 26));
        if (icon == QMessageBox::Critical) {
            painter.drawLine(QPointF(12, 12), QPointF(20, 20));
            painter.drawLine(QPointF(20, 12), QPointF(12, 20));
        } else if (icon == QMessageBox::Question) {
            QPainterPath question;
            question.moveTo(12, 12);
            question.cubicTo(12, 7, 23, 8, 20, 14);
            question.cubicTo(19, 16, 16, 16, 16, 19);
            painter.setBrush(Qt::NoBrush);
            painter.drawPath(question);
            painter.drawPoint(QPointF(16, 23));
        } else if (icon == QMessageBox::Warning) {
            painter.drawLine(QPointF(16, 10), QPointF(16, 18));
            painter.drawPoint(QPointF(16, 23));
        } else {
            painter.drawPoint(QPointF(16, 10));
            painter.drawLine(QPointF(16, 15), QPointF(16, 23));
        }
        painter.end();
        message->setIconPixmap(pixels);
    }

    bool menuActionAt(const QPoint &global) const
    {
        auto *menu = qobject_cast<QMenuBar *>(m_header);
        return menu && menu->actionAt(menu->mapFromGlobal(global));
    }

    Qt::Edges resizeEdges(const QPoint &point) const
    {
        if (m_window->isMaximized() || m_window->minimumSize() == m_window->maximumSize())
            return {};
        Qt::Edges edges;
        if (point.x() < 5)
            edges |= Qt::LeftEdge;
        if (point.x() >= m_window->width() - 5)
            edges |= Qt::RightEdge;
        if (point.y() < 5)
            edges |= Qt::TopEdge;
        if (point.y() >= m_window->height() - 5)
            edges |= Qt::BottomEdge;
        return edges;
    }

    void finishDrag()
    {
        if (!m_dragging)
            return;
        m_dragging = false;
        m_window->releaseMouse();
    }

    QWidget *m_window;
    QMainWindow *m_mainWindow = nullptr;
    QWidget *m_header = nullptr;
    QLabel *m_title = nullptr;
    QPoint m_dragOffset;
    bool m_dragging = false;
};

class ChromeInstaller final : public QObject {
public:
    using QObject::QObject;

protected:
    bool eventFilter(QObject *object, QEvent *event) override
    {
        if (event->type() != QEvent::Polish)
            return false;
        auto *window = qobject_cast<QWidget *>(object);
        if (!window || !window->isWindow() || window->property("launcherChrome").toBool()
            || qobject_cast<QFileDialog *>(window))
            return false;
        if (!qobject_cast<QDialog *>(window) && !qobject_cast<QMainWindow *>(window))
            return false;
#ifdef Q_OS_MACOS
        // Keep native traffic lights and the global menu on macOS.
        if (qobject_cast<QMainWindow *>(window))
            return false;
#endif
        new WindowChrome(window);
        return false;
    }
};
}

void installLauncherWindowChrome(QApplication &application)
{
    if (application.property("launcherChromeInstalled").toBool())
        return;
    application.setProperty("launcherChromeInstalled", true);
    application.installEventFilter(new ChromeInstaller(&application));
}
