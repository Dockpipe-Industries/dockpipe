#include "CockpitHudPanel.h"
#include "CanvasIcons.h"
#include <QLabel>
#include <QMouseEvent>
#include <QPushButton>
#include <QVBoxLayout>
#include <cmath>

CockpitHudPanel::CockpitHudPanel(const QString &title, QWidget *parent) : QFrame(parent)
{
    setObjectName("cockpitHudPanel");
    setProperty("cockpitPanel", true);
    setAttribute(Qt::WA_StyledBackground);
    setStyleSheet("QFrame[cockpitPanel=\"true\"] { background: rgba(9, 24, 36, 235); border: 1px solid #3d687c; border-radius: 10px; }"
        "QLabel { color: #d0e7ee; background: transparent; border: none; }");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(12, 8, 12, 10);
    layout->setSpacing(6);
    auto *header = new QHBoxLayout;
    m_title = new QLabel(title);
    m_title->setObjectName("cockpitDragHandle");
    m_title->setTextFormat(Qt::PlainText);
    m_title->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    m_title->setMinimumHeight(28);
    m_title->installEventFilter(this);
    header->addWidget(m_title, 1);
    m_fullscreen = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Fit), QString());
    m_fullscreen->setFixedSize(28, 28);
    m_fullscreen->setToolTip(tr("Toggle panel fullscreen"));
    connect(m_fullscreen, &QPushButton::clicked, this, &CockpitHudPanel::fullscreenRequested);
    header->addWidget(m_fullscreen);
    m_close = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Close), QString());
    m_close->setFixedSize(28, 28);
    m_close->setToolTip(tr("Return to flight"));
    connect(m_close, &QPushButton::clicked, this, &CockpitHudPanel::closeRequested);
    header->addWidget(m_close);
    layout->addLayout(header);
    m_body = new QVBoxLayout;
    m_body->setContentsMargins(0, 0, 0, 0);
    layout->addLayout(m_body, 1);
    auto *grip = new QLabel;
    grip->setPixmap(CanvasIcons::icon(CanvasIcons::Symbol::Fit).pixmap(16, 16));
    grip->setObjectName("cockpitResizeGrip");
    grip->setFixedSize(20, 20);
    grip->setCursor(Qt::SizeFDiagCursor);
    grip->setToolTip(tr("Resize this HUD panel"));
    m_grip = grip;
    layout->addWidget(m_grip, 0, Qt::AlignRight);
    m_grip->installEventFilter(this);
    m_grip->hide();
}

void CockpitHudPanel::setTitle(const QString &title) { m_title->setText(title); }
void CockpitHudPanel::setClosable(bool enabled)
{
    m_close->setVisible(enabled);
    m_fullscreen->setVisible(enabled);
}

void CockpitHudPanel::setArrangementEnabled(bool enabled)
{
    m_arrange = enabled;
    m_title->setCursor(enabled ? Qt::SizeAllCursor : Qt::ArrowCursor);
    m_title->setToolTip(enabled ? tr("Drag to arrange your windshield") : QString());
    m_grip->setVisible(enabled);
}

QRectF CockpitHudPanel::placement() const
{
    const QRect area = placementBounds();
    const QSize bounds = area.size().expandedTo(QSize(1, 1));
    return QRectF(qreal(x() - area.x()) / bounds.width(), qreal(y() - area.y()) / bounds.height(),
        qreal(width()) / bounds.width(), qreal(height()) / bounds.height());
}

QRect CockpitHudPanel::placementBounds() const
{
    if (m_placementBounds.isValid())
        return m_placementBounds.intersected(parentWidget()->rect());
    return parentWidget()->rect();
}

void CockpitHudPanel::place(const QRectF &normalized)
{
    if (!std::isfinite(normalized.x()) || !std::isfinite(normalized.y())
        || !std::isfinite(normalized.width()) || !std::isfinite(normalized.height()))
        return;
    const QRect area = placementBounds();
    const QSize bounds = area.size();
    setGeometry(area.x() + qBound(0.0, normalized.x(), 1.0) * bounds.width(),
        area.y() + qBound(0.0, normalized.y(), 1.0) * bounds.height(),
        qBound(0.1, normalized.width(), 1.0) * bounds.width(),
        qBound(0.1, normalized.height(), 1.0) * bounds.height());
    fitToViewport();
}

void CockpitHudPanel::fitToViewport()
{
    const QRect area = placementBounds();
    const QSize bounds = area.size();
    const QSize readable = size().expandedTo(minimumSizeHint());
    resize(qMin(readable.width(), bounds.width()), qMin(readable.height(), bounds.height()));
    move(qBound(area.x(), x(), area.x() + qMax(0, bounds.width() - width())),
        qBound(area.y(), y(), area.y() + qMax(0, bounds.height() - height())));
}

bool CockpitHudPanel::eventFilter(QObject *object, QEvent *event)
{
    if (object == m_title && m_arrange) {
        if (event->type() == QEvent::MouseButtonPress) {
            auto *mouse = static_cast<QMouseEvent *>(event);
            if (mouse->button() == Qt::LeftButton) {
                m_dragOffset = mouse->globalPosition().toPoint() - mapToGlobal(QPoint());
                m_dragging = true;
                raise();
                return true;
            }
        } else if (event->type() == QEvent::MouseMove && m_dragging) {
            auto *mouse = static_cast<QMouseEvent *>(event);
            move(parentWidget()->mapFromGlobal(mouse->globalPosition().toPoint() - m_dragOffset));
            fitToViewport();
            return true;
        } else if (event->type() == QEvent::MouseButtonRelease && m_dragging) {
            m_dragging = false;
            emit placementChanged();
            return true;
        }
    }
    if (object == m_grip && m_arrange) {
        if (event->type() == QEvent::MouseButtonPress) {
            auto *mouse = static_cast<QMouseEvent *>(event);
            if (mouse->button() == Qt::LeftButton) {
                m_resizeOrigin = mouse->globalPosition().toPoint();
                m_resizeStart = size();
                m_resizing = true;
                return true;
            }
        } else if (event->type() == QEvent::MouseMove && m_resizing) {
            const auto *mouse = static_cast<QMouseEvent *>(event);
            const QPoint delta = mouse->globalPosition().toPoint() - m_resizeOrigin;
            resize(m_resizeStart + QSize(delta.x(), delta.y()));
            fitToViewport();
            return true;
        } else if (event->type() == QEvent::MouseButtonRelease && m_resizing) {
            m_resizing = false;
            emit placementChanged();
            return true;
        }
    }
    return QFrame::eventFilter(object, event);
}
