#include "CockpitInputMenu.h"
#include "FurnishingPreview.h"
#include <QHBoxLayout>
#include <QKeyEvent>
#include <QLabel>
#include <QListWidget>
#include <QPushButton>
#include <QScrollBar>
#include <QVBoxLayout>
#include <cmath>

CockpitInputMenu::CockpitInputMenu(QWidget *parent) : QFrame(parent)
{
    setObjectName("cockpitInputMenu");
    setProperty("flightOverlay", true);
    setStyleSheet("QFrame#cockpitInputMenu { background: #102330; border: 1px solid #69bfb9; border-radius: 12px; }"
        "QLabel { color: #d7eeef; } QListWidget { background: #0b1924; color: #d7eeef; border: 0; }"
        "QListWidget::item { padding: 8px; border: 1px solid #294354; border-radius: 6px; }"
        "QListWidget::item:selected { background: #28596a; color: white; border: 2px solid #70e9cb; }"
        "QPushButton { background: #244553; color: #d7eeef; padding: 8px 16px; border-radius: 6px; }");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(18, 18, 18, 18);
    m_title = new QLabel;
    m_categoryBar = new QWidget;
    auto *categories = new QHBoxLayout(m_categoryBar);
    categories->setContentsMargins(0, 0, 0, 0);
    auto *previous = new QPushButton(tr("‹  LB / Q"));
    auto *next = new QPushButton(tr("RB / E  ›"));
    previous->setFocusPolicy(Qt::NoFocus);
    next->setFocusPolicy(Qt::NoFocus);
    m_categoryLabel = new QLabel;
    m_categoryLabel->setObjectName("cockpitModelCategory");
    m_categoryLabel->setAlignment(Qt::AlignCenter);
    m_categoryLabel->setTextFormat(Qt::PlainText);
    categories->addWidget(previous);
    categories->addWidget(m_categoryLabel, 1);
    categories->addWidget(next);
    connect(previous, &QPushButton::clicked, this, [this]() { changeCategory(-1); });
    connect(next, &QPushButton::clicked, this, [this]() { changeCategory(1); });
    m_entries = new QListWidget;
    m_entries->setObjectName("cockpitMenuEntries");
    m_entries->setMovement(QListView::Static);
    m_entries->setResizeMode(QListView::Adjust);
    m_entries->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOn);
    m_entries->setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_description = new QLabel;
    m_description->setObjectName("cockpitMenuDescription");
    m_description->setWordWrap(true);
    m_description->setMinimumHeight(48);
    m_hints = new QLabel;
    m_hints->setObjectName("cockpitMenuHints");
    m_hints->setWordWrap(true);
    for (auto *label : {m_title, m_description, m_hints})
        label->setTextFormat(Qt::PlainText);
    layout->addWidget(m_title);
    layout->addWidget(m_categoryBar);
    layout->addWidget(m_entries, 1);
    layout->addWidget(m_description);
    layout->addWidget(m_hints);
    connect(m_entries, &QListWidget::currentItemChanged, this, [this](QListWidgetItem *item) {
        m_description->setText(item ? item->data(Qt::UserRole + 1).toString() : QString());
        if (m_catalogue && item)
            m_lastModel = item->data(Qt::UserRole).toString();
    });
    connect(m_entries, &QListWidget::itemClicked, this, [this]() { activate(); });
    m_previewTimer.setSingleShot(true);
    connect(&m_previewTimer, &QTimer::timeout, this, &CockpitInputMenu::refreshPreviews);
    connect(m_entries->verticalScrollBar(), &QScrollBar::valueChanged, this, [this]() {
        if (m_catalogue && isVisible())
            m_previewTimer.start(0);
    });
    hide();
}

void CockpitInputMenu::present(QWidget *surface, const QString &title, const QVector<CockpitMenuEntry> &entries, bool controller)
{
    m_previewTimer.stop();
    if (parentWidget() != surface)
        setParent(surface);
    m_title->setText(title);
    m_sourceEntries = entries;
    m_catalogue = !entries.isEmpty() && !entries.first().previewType.isEmpty();
    m_categories = {QString()}; // Empty key is the complete, package-owned catalogue.
    if (m_catalogue) {
        for (const auto &entry : entries) {
            if (!m_categories.contains(entry.category))
                m_categories.append(entry.category);
        }
        if (!m_categories.contains(m_category))
            m_category.clear();
    }
    m_categoryBar->setVisible(m_catalogue);
    m_entries->setViewMode(m_catalogue ? QListView::IconMode : QListView::ListMode);
    m_entries->setFlow(m_catalogue ? QListView::LeftToRight : QListView::TopToBottom);
    m_entries->setWrapping(m_catalogue);
    m_entries->setWordWrap(m_catalogue);
    m_entries->setUniformItemSizes(m_catalogue);
    m_entries->setSpacing(0);
    m_entries->setIconSize(m_catalogue ? QSize(144, 112) : QSize());
    m_entries->setGridSize(m_catalogue ? QSize(176, 174) : QSize());
    populate();
    m_stickDirection = 0;
    setControllerHints(controller);
    reposition();
    show();
    raise();
    m_entries->setFocus();
    if (m_catalogue)
        m_previewTimer.start(0);
}

void CockpitInputMenu::populate()
{
    const QString restore = m_lastModel;
    m_entries->clear();
    int selected = 0;
    for (const auto &entry : m_sourceEntries) {
        if (m_catalogue && !m_category.isEmpty() && entry.category != m_category)
            continue;
        auto *item = new QListWidgetItem(entry.label, m_entries);
        item->setData(Qt::UserRole, entry.id);
        item->setData(Qt::UserRole + 1, entry.description);
        item->setData(Qt::UserRole + 2, entry.enabled);
        item->setData(Qt::UserRole + 3, entry.previewType);
        item->setToolTip(entry.description);
        if (m_catalogue) {
            item->setSizeHint(QSize(168, 166));
            item->setTextAlignment(Qt::AlignHCenter | Qt::AlignBottom);
        }
        if (!entry.enabled)
            item->setForeground(QColor("#829198"));
        if (entry.id == restore)
            selected = m_entries->count() - 1;
    }
    m_entries->setCurrentRow(selected);
    m_categoryLabel->setText(tr("%1 · %2 models").arg(m_category.isEmpty() ? tr("All models") : m_category).arg(m_entries->count()));
    m_entries->scrollToItem(m_entries->currentItem());
}

void CockpitInputMenu::changeCategory(int delta)
{
    if (!m_catalogue || m_categories.isEmpty())
        return;
    const int index = m_categories.indexOf(m_category);
    m_category = m_categories[(index + delta + m_categories.size()) % m_categories.size()];
    populate();
    m_stickDirection = 0;
    m_previewTimer.start(0);
}

void CockpitInputMenu::refreshPreviews()
{
    if (!isVisible() || !m_catalogue)
        return;
    // Share the furnishing browser's cache. Generate only visible tiles, at
    // most six per event turn, so imported models cannot stall menu opening.
    int generated = 0;
    for (int index = 0; index < m_entries->count(); ++index) {
        auto *item = m_entries->item(index);
        if (item->data(Qt::UserRole + 4).toBool() || !m_entries->visualItemRect(item).intersects(m_entries->viewport()->rect()))
            continue;
        item->setData(Qt::UserRole + 4, true);
        item->setIcon(QIcon(FurnishingPreview::thumbnail(item->data(Qt::UserRole + 3).toString())));
        if (++generated == 6) {
            m_previewTimer.start(0);
            break;
        }
    }
}

void CockpitInputMenu::reposition()
{
    const auto area = parentWidget()->rect().adjusted(16, 16, -16, -16);
    const QSize size(qMax(1, qMin(m_catalogue ? 1120 : 540, area.width())),
        qMax(1, qMin(m_catalogue ? 780 : 580, area.height())));
    setGeometry(QRect(area.center() - QPoint(size.width() / 2, size.height() / 2), size));
    if (m_catalogue && isVisible())
        m_previewTimer.start(0);
}

void CockpitInputMenu::setControllerHints(bool controller)
{
    if (m_catalogue) {
        m_hints->setText(controller
            ? tr("D-pad / left stick: browse · LB / RB: category · A: spawn · B: back · Start: close")
            : tr("Arrow keys: browse · Q / E: category · Enter: spawn · Esc: back · F1: close"));
    } else {
        m_hints->setText(controller ? tr("D-pad / left stick: choose · A: select · B: back · Start: close")
            : tr("↑ / ↓: choose · Enter: select · Esc: back · F1: close"));
    }
}

int CockpitInputMenu::columns() const
{
    return m_catalogue ? qMax(1, m_entries->viewport()->width() / m_entries->gridSize().width()) : 1;
}

void CockpitInputMenu::move(int delta)
{
    const int count = m_entries->count();
    if (!count)
        return;
    const int next = m_entries->currentRow() + delta;
    m_entries->setCurrentRow(m_catalogue ? qBound(0, next, count - 1) : (next + count) % count);
    m_entries->scrollToItem(m_entries->currentItem());
}

void CockpitInputMenu::activate()
{
    const auto *item = m_entries->currentItem();
    if (item && item->data(Qt::UserRole + 2).toBool())
        emit triggered(item->data(Qt::UserRole).toString());
}

bool CockpitInputMenu::key(QKeyEvent *event)
{
    switch (event->key()) {
    case Qt::Key_Up: move(-columns()); break;
    case Qt::Key_Down: move(columns()); break;
    case Qt::Key_Right:
        if (m_catalogue)
            move(1);
        else if (!event->isAutoRepeat())
            activate();
        break;
    case Qt::Key_Left:
        if (m_catalogue)
            move(-1);
        else if (!event->isAutoRepeat())
            emit backRequested();
        break;
    case Qt::Key_Q: changeCategory(-1); break;
    case Qt::Key_E: changeCategory(1); break;
    case Qt::Key_Return: case Qt::Key_Enter:
        if (!event->isAutoRepeat())
            activate();
        break;
    case Qt::Key_Escape: case Qt::Key_Backspace:
        if (!event->isAutoRepeat())
            emit backRequested();
        break;
    default: return false;
    }
    return true;
}

void CockpitInputMenu::button(int button)
{
    switch (button) {
    case 0: activate(); break;
    case 1: emit backRequested(); break;
    case 11: move(-columns()); break;
    case 12: move(columns()); break;
    case 13:
        if (m_catalogue)
            move(-1);
        else
            emit backRequested();
        break;
    case 14:
        if (m_catalogue)
            move(1);
        else
            activate();
        break;
    case 9: changeCategory(-1); break;
    case 10: changeCategory(1); break;
    default: break;
    }
}

void CockpitInputMenu::stick(qreal vertical, qreal horizontal)
{
    int direction = 0;
    if (m_catalogue && std::abs(horizontal) > std::abs(vertical) && std::abs(horizontal) > .5)
        direction = horizontal > 0 ? 1 : -1;
    else if (vertical > .5)
        direction = -columns();
    else if (vertical < -.5)
        direction = columns();
    if (!direction) {
        m_stickDirection = 0;
        return;
    }
    if (direction != m_stickDirection || !m_repeat.isValid() || m_repeat.elapsed() >= 250) {
        move(direction);
        m_stickDirection = direction;
        m_repeat.start();
    }
}
