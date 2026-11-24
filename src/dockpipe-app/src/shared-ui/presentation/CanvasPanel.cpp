#include "CanvasPanel.h"

#include <QAbstractButton>
#include <QAbstractItemView>
#include <QAbstractSlider>
#include <QAbstractSpinBox>
#include <QComboBox>
#include <QLineEdit>
#include <QKeyEvent>
#include <QTabBar>
#include <QApplication>
#include <QGraphicsDropShadowEffect>
#include <QMouseEvent>
#include <QPainter>
#include <QSettings>

CanvasPanel::CanvasPanel(const QString &identity, QWidget *parent, bool persistent)
    : QFrame(parent), m_persistent(persistent), m_settingsKey("workflowStudio/panels/" + identity)
{
    setProperty("canvasOverlay", true);
    setProperty("canvasWindow", true);
    setMouseTracking(true);
    auto *shadow = new QGraphicsDropShadowEffect(this);
    shadow->setBlurRadius(24);
    shadow->setOffset(0, 6);
    shadow->setColor(QColor(0, 0, 0, 65));
    setGraphicsEffect(shadow);
    const QRect stored = m_persistent ? QSettings().value(m_settingsKey).toRect() : QRect();
    if (stored.isValid()) {
        m_preferredGeometry = stored;
        m_customPlacement = true;
    }
    qApp->installEventFilter(this);
}

void CanvasPanel::setDragHandle(QWidget *handle)
{
    setDragHandle(handle, false);
}

void CanvasPanel::setDragHandle(QWidget *handle, bool fullWidth)
{
    m_handle = handle;
    setProperty("canvasFullWidthDrag", fullWidth);
    handle->setCursor(Qt::OpenHandCursor);
    handle->setToolTip(tr("Drag to move · Resize from the edges or lower-right corner"));
}

QSize CanvasPanel::minimumPanelSize(const QSize &available) const
{
    const QSize baseline = property("dashboardSpatial").toBool() ? QSize(280, 72) : QSize(280, 180);
    return minimumSizeHint().expandedTo(minimumSize()).expandedTo(baseline).boundedTo(available);
}

QRect CanvasPanel::constrained(const QRect &geometry) const
{
    if (!parentWidget())
        return geometry;
    const QRect bounds = parentWidget()->rect().adjusted(8, 8, -8, -8);
    const QSize minimum = minimumPanelSize(bounds.size());
    const QSize size = geometry.size().expandedTo(minimum).boundedTo(bounds.size()).boundedTo(maximumSize());
    const QPoint position(qBound(bounds.left(), geometry.x(), bounds.right() - size.width() + 1),
                          qBound(bounds.top(), geometry.y(), bounds.bottom() - size.height() + 1));
    return QRect(position, size);
}

void CanvasPanel::placeDefault(const QRect &geometry)
{
    if (m_dragging || m_resizeEdges)
        return;
    setGeometry(constrained(m_customPlacement ? m_preferredGeometry : geometry));
}

void CanvasPanel::resetPlacement()
{
    finishInteraction();
    m_customPlacement = false;
    if (m_persistent)
        QSettings().remove(m_settingsKey);
}

void CanvasPanel::finishInteraction()
{
    if (!m_dragging && !m_resizeEdges)
        return;
    m_dragging = false;
    m_resizeEdges = {};
    if (QWidget::mouseGrabber() == this)
        releaseMouse();
    unsetCursor();
    if (m_handle)
        m_handle->setCursor(Qt::OpenHandCursor);
    m_preferredGeometry = geometry();
    if (m_persistent)
        QSettings().setValue(m_settingsKey, m_preferredGeometry);
    emit placementChanged();
}

bool CanvasPanel::eventFilter(QObject *object, QEvent *event)
{
    if (m_dragging || m_resizeEdges) {
        const bool interrupted = (object == this && event->type() == QEvent::UngrabMouse)
            || (object == window() && (event->type() == QEvent::WindowDeactivate
                || event->type() == QEvent::WindowBlocked))
            || event->type() == QEvent::ApplicationDeactivate;
        if (interrupted)
            finishInteraction();
        if (event->type() == QEvent::KeyPress
            && static_cast<QKeyEvent *>(event)->key() == Qt::Key_Escape) {
            finishInteraction();
            return true;
        }
        // A native surface or system gesture can take capture without sending
        // the release to this panel. Never carry that gesture into the next click.
        if (event->type() == QEvent::MouseButtonPress
            || (event->type() == QEvent::MouseMove
                && !(static_cast<QMouseEvent *>(event)->buttons() & Qt::LeftButton)))
            finishInteraction();
    }
    auto *target = qobject_cast<QWidget *>(object);
    if (!target || !isVisible() || !m_interactionEnabled)
        return QFrame::eventFilter(object, event);
    const bool inside = target == this || isAncestorOf(target);
    if (event->type() == QEvent::MouseButtonPress && inside) {
        raise();
        emit activated();
        auto *mouse = static_cast<QMouseEvent *>(event);
        if (mouse->button() != Qt::LeftButton)
            return false;
        const QPoint point = mapFromGlobal(mouse->globalPosition().toPoint());
        // Header layouts may contain tabs and controls. Only unused space is a
        // drag surface; walk ancestors because buttons can own labels/icons.
        for (QWidget *control = target; control && control != this; control = control->parentWidget()) {
            if (qobject_cast<QAbstractButton *>(control) || qobject_cast<QLineEdit *>(control)
                || qobject_cast<QComboBox *>(control) || qobject_cast<QAbstractSpinBox *>(control)
                || qobject_cast<QAbstractSlider *>(control) || qobject_cast<QAbstractItemView *>(control))
                return false;
            if (auto *tabs = qobject_cast<QTabBar *>(control)) {
                if (tabs->tabAt(tabs->mapFromGlobal(mouse->globalPosition().toPoint())) >= 0)
                    return false;
            }
        }

        Qt::Edges edges;
        if (point.x() < 6)
            edges |= Qt::LeftEdge;
        if (point.x() >= width() - 6)
            edges |= Qt::RightEdge;
        if (point.y() < 6)
            edges |= Qt::TopEdge;
        if (point.y() >= height() - 6)
            edges |= Qt::BottomEdge;
        if (point.x() >= width() - 20 && point.y() >= height() - 20)
            edges = Qt::RightEdge | Qt::BottomEdge;
        bool header = m_handle && (target == m_handle || m_handle->isAncestorOf(target));
        if (m_handle && property("canvasFullWidthDrag").toBool()) {
            const int top = m_handle->mapTo(this, QPoint()).y();
            header = point.y() >= top && point.y() < top + m_handle->height();
        }
        if (!edges && !header)
            return false;
        m_resizeEdges = edges;
        m_dragging = !edges;
        m_startGeometry = geometry();
        m_startPointer = mouse->globalPosition().toPoint();
        m_customPlacement = true;
        if (m_handle && m_dragging)
            m_handle->setCursor(Qt::ClosedHandCursor);
        grabMouse();
        return true;
    }
    if (event->type() == QEvent::MouseMove && (m_dragging || m_resizeEdges)) {
        auto *mouse = static_cast<QMouseEvent *>(event);
        const QPoint delta = mouse->globalPosition().toPoint() - m_startPointer;
        QRect next = m_startGeometry;
        if (m_dragging) {
            next.translate(delta);
        } else {
            const QRect bounds = parentWidget()->rect().adjusted(8, 8, -8, -8);
            const QSize minimum = minimumPanelSize(bounds.size());
            if (m_resizeEdges & Qt::LeftEdge)
                next.setLeft(qBound(bounds.left(), next.left() + delta.x(), next.right() - minimum.width() + 1));
            if (m_resizeEdges & Qt::RightEdge)
                next.setRight(qBound(next.left() + minimum.width() - 1, next.right() + delta.x(), bounds.right()));
            if (m_resizeEdges & Qt::TopEdge)
                next.setTop(qBound(bounds.top(), next.top() + delta.y(), next.bottom() - minimum.height() + 1));
            if (m_resizeEdges & Qt::BottomEdge)
                next.setBottom(qBound(next.top() + minimum.height() - 1, next.bottom() + delta.y(), bounds.bottom()));
        }
        setGeometry(constrained(next));
        return true;
    }
    if (event->type() == QEvent::MouseButtonRelease && (m_dragging || m_resizeEdges)) {
        if (static_cast<QMouseEvent *>(event)->button() != Qt::LeftButton)
            return false;
        finishInteraction();
        return inside;
    }
    if (event->type() == QEvent::MouseMove && target == this) {
        auto *mouse = static_cast<QMouseEvent *>(event);
        const QPoint point = mouse->position().toPoint();
        if (point.x() > width() - 20 && point.y() > height() - 20)
            setCursor(Qt::SizeFDiagCursor);
        else if (point.x() < 6 || point.x() > width() - 6)
            setCursor(Qt::SizeHorCursor);
        else if (point.y() < 6 || point.y() > height() - 6)
            setCursor(Qt::SizeVerCursor);
        else
            unsetCursor();
    }
    return QFrame::eventFilter(object, event);
}

void CanvasPanel::paintEvent(QPaintEvent *event)
{
    QFrame::paintEvent(event);
    if (!m_interactionEnabled)
        return;
    QPainter painter(this);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.setPen(QPen(palette().color(QPalette::PlaceholderText), 1));
    painter.drawLine(width() - 14, height() - 7, width() - 7, height() - 14);
    painter.drawLine(width() - 10, height() - 7, width() - 7, height() - 10);
}

void CanvasPanel::hideEvent(QHideEvent *event)
{
    finishInteraction();
    QFrame::hideEvent(event);
}

void CanvasPanel::setInteractionEnabled(bool enabled)
{
    finishInteraction();
    m_interactionEnabled = enabled;
    if (m_handle)
        m_handle->setCursor(enabled ? Qt::OpenHandCursor : Qt::ArrowCursor);
    update();
}
