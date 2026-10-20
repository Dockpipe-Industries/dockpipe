#include "BrowserTabBar.h"

#include <QApplication>
#include <QHideEvent>
#include <QKeyEvent>
#include <QMouseEvent>

BrowserTabBar::BrowserTabBar(QWidget *parent) : QTabBar(parent)
{
}

void BrowserTabBar::setTearOutEnabled(bool enabled)
{
    m_tearOutEnabled = enabled;
    m_dragCandidate = false;
}

void BrowserTabBar::mousePressEvent(QMouseEvent *event)
{
    m_pressPosition = event->position().toPoint();
    m_dragCandidate = m_tearOutEnabled && event->button() == Qt::LeftButton
        && tabAt(m_pressPosition) >= 0;
    QTabBar::mousePressEvent(event);
}

void BrowserTabBar::mouseReleaseEvent(QMouseEvent *event)
{
    const QPoint position = event->position().toPoint();
    const bool tearOut = m_dragCandidate && event->button() == Qt::LeftButton
        && !rect().adjusted(-16, -16, 16, 16).contains(position)
        && (position - m_pressPosition).manhattanLength() >= QApplication::startDragDistance();
    m_dragCandidate = false;
    const int index = currentIndex();
    QTabBar::mouseReleaseEvent(event);
    if (tearOut && index >= 0)
        emit tearOutRequested(index, event->globalPosition().toPoint());
}

void BrowserTabBar::keyPressEvent(QKeyEvent *event)
{
    if (event->key() == Qt::Key_Escape)
        m_dragCandidate = false;
    QTabBar::keyPressEvent(event);
}

void BrowserTabBar::hideEvent(QHideEvent *event)
{
    m_dragCandidate = false;
    QTabBar::hideEvent(event);
}
