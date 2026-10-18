#include "DashboardDock.h"
#include "CanvasIcons.h"
#include "DashboardStore.h"

#include <QEvent>
#include <QApplication>
#include <QCursor>
#include <QFocusEvent>
#include <QPropertyAnimation>
#include <QTimer>
#include <QFrame>
#include <QHBoxLayout>
#include <QPainter>
#include <QPainterPath>
#include <QRegion>
#include <QScrollArea>
#include <QScrollBar>
#include <QSet>
#include <QStyle>
#include <QToolButton>
#include <QVBoxLayout>

namespace {
const QStringList pinnedKinds = {"browser", "terminal", "files", "git", "workflows", "machines", "desktop-window"};

CanvasIcons::Symbol iconForKind(const QString &kind)
{
    if (kind == "browser")
        return CanvasIcons::Symbol::Globe;
    if (kind == "files")
        return CanvasIcons::Symbol::Folder;
    if (kind == "workflows")
        return CanvasIcons::Symbol::Workflow;
    if (kind == "send-input")
        return CanvasIcons::Symbol::Keyboard;
    if (kind == "logs" || kind == "terminal")
        return CanvasIcons::Symbol::Terminal;
    if (kind == "git")
        return CanvasIcons::Symbol::Branch;
    if (kind == "executions")
        return CanvasIcons::Symbol::Play;
    if (kind == "containers")
        return CanvasIcons::Symbol::Extensions;
    return CanvasIcons::Symbol::Monitor;
}

class DockButton final : public QToolButton {
public:
    explicit DockButton(QWidget *parent) : QToolButton(parent)
    {
        setFixedSize(48, 48);
        setIconSize(QSize(26, 26));
        setFocusPolicy(Qt::StrongFocus);
        setCursor(Qt::PointingHandCursor);
        setToolButtonStyle(Qt::ToolButtonIconOnly);
    }

protected:
    void paintEvent(QPaintEvent *event) override
    {
        QToolButton::paintEvent(event);
        if (!property("dashboardRunning").toBool())
            return;
        QPainter painter(this);
        painter.setRenderHint(QPainter::Antialiasing);
        painter.setPen(Qt::NoPen);
        painter.setBrush(isChecked() ? palette().highlight() : palette().text());
        painter.drawEllipse(QPointF(width() / 2.0, height() - 5.0), 2.0, 2.0);
    }
};

class DockScrollArea final : public QScrollArea {
public:
    QSize sizeHint() const override
    {
        // Let the surrounding layouts account for their own frame and margins.
        // QScrollArea's default preferred width caps a larger row of shortcuts.
        const QSize content = widget() ? widget()->sizeHint() : QSize();
        return content + QSize(2 * frameWidth(), 2 * frameWidth());
    }
};

void describeButton(QToolButton *button, const QString &title)
{
    button->setAccessibleName(title);
    button->setToolTip(QStringLiteral("<qt>%1</qt>").arg(title.toHtmlEscaped()));
}
}

DashboardDock::DashboardDock(QWidget *parent) : QWidget(parent)
{
    setObjectName("dashboardDock");
    setSizePolicy(QSizePolicy::Preferred, QSizePolicy::Fixed);
    setFixedHeight(68);
    auto *outer = new QHBoxLayout(this);
    outer->setContentsMargins(0, 0, 0, 0);
    auto *surface = new QFrame;
    surface->setObjectName("dashboardDockSurface");
    surface->setStyleSheet(R"CSS(
        QFrame#dashboardDockSurface { background: palette(alternate-base);
            border: 1px solid palette(mid); border-radius: 18px; }
        QScrollArea#dashboardDockScroll, QWidget#dashboardDockContent { background: transparent; border: none; }
        QToolButton { background: transparent; border: 2px solid transparent;
            border-radius: 12px; padding: 2px 2px 7px 2px; }
        QToolButton:hover { background: palette(base); }
        QToolButton:checked { background: palette(base); border-color: palette(mid); }
        QToolButton:focus { border-color: palette(highlight); }
        QToolButton:pressed { background: palette(mid); }
    )CSS");
    outer->addWidget(surface);
    auto *surfaceLayout = new QHBoxLayout(surface);
    surfaceLayout->setContentsMargins(6, 6, 6, 6);
    surfaceLayout->setSpacing(4);
    m_scroll = new DockScrollArea;
    m_scroll->setObjectName("dashboardDockScroll");
    m_scroll->setFrameShape(QFrame::NoFrame);
    m_scroll->setWidgetResizable(true);
    m_scroll->setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    m_scroll->setHorizontalScrollBarPolicy(Qt::ScrollBarAsNeeded);
    surfaceLayout->addWidget(m_scroll);
    m_content = new QWidget;
    m_content->setObjectName("dashboardDockContent");
    m_buttons = new QHBoxLayout(m_content);
    m_buttons->setContentsMargins(4, 4, 4, 4);
    m_buttons->setSpacing(4);
    m_buttons->setSizeConstraint(QLayout::SetMinimumSize);
    m_scroll->setWidget(m_content);
    auto *appSlot = new QWidget(surface);
    auto *appLayout = new QVBoxLayout(appSlot);
    appLayout->setContentsMargins(0, 4, 0, 4);
    m_apps = new DockButton(appSlot);
    m_apps->setObjectName("dashboardDockApps");
    m_apps->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Apps));
    describeButton(m_apps, tr("Apps and workflows"));
    m_apps->installEventFilter(this);
    appLayout->addWidget(m_apps);
    surfaceLayout->insertWidget(0, appSlot, 0, Qt::AlignTop);
    connect(m_apps, &QToolButton::clicked, this, &DashboardDock::appsRequested);
    const QStringList pinnedTitles = {tr("Browser"), tr("Terminal"), tr("Files"), tr("Git"), tr("Workflows"), tr("Machines"), tr("Desktop window")};
    for (int index = 0; index < pinnedKinds.size(); ++index) {
        const QString kind = pinnedKinds[index];
        auto *button = new DockButton(m_content);
        button->setObjectName("dashboardDockPinned_" + kind);
        button->setIcon(CanvasIcons::icon(iconForKind(kind)));
        button->setCheckable(true);
        describeButton(button, pinnedTitles[index]);
        button->installEventFilter(this);
        m_pinned.insert(kind, button);
        m_buttons->addWidget(button);
        connect(button, &QToolButton::clicked, this, [this, kind]() { activateKind(kind); });
    }
    m_separator = new QFrame;
    m_separator->setObjectName("dashboardDockSeparator");
    m_separator->setFrameShape(QFrame::VLine);
    m_separator->setFixedSize(1, 32);
    m_buttons->addWidget(m_separator);
    m_separator->hide();
    m_buttons->addStretch();
    connect(m_scroll->horizontalScrollBar(), &QScrollBar::rangeChanged, this, [this](int, int maximum) {
        const int scrollbarHeight = maximum > 0 ? style()->pixelMetric(QStyle::PM_ScrollBarExtent) : 0;
        setFixedHeight(68 + scrollbarHeight);
        updateGeometry();
    });
    updateIndicators();
    updateFocusOrder();
}

void DashboardDock::setOverlayArea(QWidget *area)
{
    if (m_overlayArea == area)
        return;
    if (m_overlayArea)
        m_overlayArea->removeEventFilter(this);
    if (m_hideTimer)
        m_hideTimer->stop();
    if (m_slide)
        m_slide->stop();
    delete m_revealEdge;
    m_overlayArea = area;
    m_revealed = false;
    m_keyboardFocus = false;
    if (!area) {
        clearMask();
        return;
    }
    if (!m_hideTimer) {
        m_hideTimer = new QTimer(this);
        m_hideTimer->setObjectName("dashboardDockHideDelay");
        m_hideTimer->setSingleShot(true);
        m_hideTimer->setInterval(550);
        connect(m_hideTimer, &QTimer::timeout, this, &DashboardDock::hideIfIdle);
        m_slide = new QPropertyAnimation(this, "pos", this);
        m_slide->setDuration(140);
        m_slide->setEasingCurve(QEasingCurve::OutCubic);
        connect(m_slide, &QPropertyAnimation::finished, this, [this] {
            if (!m_revealed && m_overlayArea)
                hide();
            else
                positionOverlay();
        });
        connect(qApp, &QApplication::focusChanged, this, [this](QWidget *, QWidget *focused) {
            if (!m_overlayArea || !m_revealed)
                return;
            if (!focused || !isAncestorOf(focused))
                m_keyboardFocus = false;
            if (!m_keyboardFocus)
                m_hideTimer->start();
        });
        installEventFilter(this);
    }
    // Native siblings must stack above embedded application and browser surfaces.
    setAttribute(Qt::WA_NativeWindow);
    m_revealEdge = new QWidget(area);
    m_revealEdge->setObjectName("dashboardDockRevealEdge");
    m_revealEdge->setAttribute(Qt::WA_NativeWindow);
    m_revealEdge->setAccessibleName(tr("Reveal dashboard dock"));
    m_revealEdge->installEventFilter(this);
    area->installEventFilter(this);
    hide();
    positionOverlay();
}

void DashboardDock::positionOverlay()
{
    if (!m_overlayArea)
        return;
    const int dockWidth = qMin(sizeHint().width(), qMax(120, m_overlayArea->width() - 16));
    resize(dockWidth, height());
    QPainterPath outline;
    outline.addRoundedRect(QRectF(rect()), 18, 18);
    setMask(QRegion(outline.toFillPolygon().toPolygon()));
    const int left = (m_overlayArea->width() - width()) / 2;
    const int top = m_overlayArea->height() - height() - 8;
    if (m_slide->state() != QAbstractAnimation::Running)
        move(left, m_revealed ? top : m_overlayArea->height());
    m_revealEdge->setGeometry(0, qMax(0, m_overlayArea->height() - 4), m_overlayArea->width(), 4);
    m_revealEdge->show();
    m_revealEdge->raise();
    if (m_revealed)
        raise();
}

void DashboardDock::reveal(bool keyboardFocus)
{
    if (!m_overlayArea) {
        if (keyboardFocus)
            m_apps->setFocus(Qt::ShortcutFocusReason);
        return;
    }
    m_hideTimer->stop();
    m_keyboardFocus |= keyboardFocus;
    if (!m_revealed || m_slide->state() == QAbstractAnimation::Running) {
        m_slide->stop();
        positionOverlay();
        m_revealed = true;
        show();
        raise();
        m_slide->setStartValue(pos());
        m_slide->setEndValue(QPoint((m_overlayArea->width() - width()) / 2,
            m_overlayArea->height() - height() - 8));
        m_slide->start();
    }
    if (keyboardFocus)
        m_apps->setFocus(Qt::ShortcutFocusReason);
    else
        m_hideTimer->start();
}

void DashboardDock::hideIfIdle()
{
    if (!m_overlayArea || !m_revealed || m_keyboardFocus || QApplication::activePopupWidget()
        || QApplication::activeModalWidget())
        return;
    const QPoint cursor = QCursor::pos();
    if (rect().contains(mapFromGlobal(cursor)) || m_revealEdge->rect().contains(m_revealEdge->mapFromGlobal(cursor)))
        return;
    m_revealed = false;
    m_slide->stop();
    m_slide->setStartValue(pos());
    m_slide->setEndValue(QPoint(x(), m_overlayArea->height()));
    m_slide->start();
}

void DashboardDock::setPanels(const QVector<DashboardDockItem> &panels)
{
    QVector<DashboardDockItem> admitted;
    QSet<QString> ids;
    // Match the workspace/store's existing 16-panel contract. The dock never
    // allocates controls for an unbounded external descriptor collection.
    const int count = qMin(panels.size(), qsizetype(DashboardStore::maximumPanelsPerDashboard));
    for (int index = 0; index < count; ++index) {
        DashboardDockItem item = panels[index];
        if (item.id.isEmpty() || ids.contains(item.id))
            continue;
        item.title = item.title.simplified().left(200);
        if (item.title.isEmpty())
            item.title = tr("Untitled panel");
        ids.insert(item.id);
        admitted.append(item);
    }
    setAccessibleDescription(panels.size() > count
        ? tr("Showing the first %1 panels, the dashboard panel limit.").arg(count) : QString());
    if (m_panels == admitted)
        return;
    for (const auto &id : m_running.keys()) {
        if (!ids.contains(id))
            delete m_running.take(id);
    }
    for (int index = 0; index < admitted.size(); ++index) {
        const auto &item = admitted[index];
        auto *button = m_running.value(item.id);
        if (!button) {
            button = new DockButton(m_content);
            button->setObjectName("dashboardDockPanel_" + item.id);
            button->setCheckable(true);
            button->setProperty("panelId", item.id);
            button->setProperty("dashboardRunning", true);
            button->installEventFilter(this);
            m_running.insert(item.id, button);
            connect(button, &QToolButton::clicked, this, [this, id = item.id]() {
                emit activatePanelRequested(id);
                updateIndicators();
            });
        }
        if (button->property("panelKind").toString() != item.kind
            || button->property("panelIconKey").toLongLong() != item.icon.cacheKey()) {
            button->setProperty("panelKind", item.kind);
            button->setProperty("panelIconKey", item.icon.cacheKey());
            button->setIcon(item.icon.isNull() ? CanvasIcons::icon(iconForKind(item.kind)) : item.icon);
        }
        if (button->accessibleName() != item.title)
            describeButton(button, item.title);
        m_buttons->insertWidget(pinnedKinds.size() + 1 + index, button);
        button->show();
    }
    m_panels = admitted;
    if (!ids.contains(m_activePanel))
        m_activePanel.clear();
    m_separator->setVisible(!m_panels.isEmpty());
    m_buttons->activate();
    updateIndicators();
    updateFocusOrder();
    m_scroll->updateGeometry();
    updateGeometry();
    positionOverlay();
}

void DashboardDock::setActivePanel(const QString &id)
{
    if (m_activePanel == id)
        return;
    m_activePanel = id;
    updateIndicators();
}

void DashboardDock::activateKind(const QString &kind)
{
    if (kind == "desktop-window" && !m_desktopAppsEnabled)
        return;
    QString target;
    for (const auto &item : m_panels) {
        if (item.kind == kind && (target.isEmpty() || item.id == m_activePanel))
            target = item.id;
    }
    if (target.isEmpty())
        emit addWidgetRequested(kind);
    else
        emit activatePanelRequested(target);
    updateIndicators();
}

void DashboardDock::setDesktopAppsEnabled(bool enabled)
{
    if (m_desktopAppsEnabled == enabled)
        return;
    m_desktopAppsEnabled = enabled;
    m_pinned.value("desktop-window")->setVisible(enabled);
    updateFocusOrder();
    m_buttons->activate();
    m_scroll->updateGeometry();
    updateGeometry();
    positionOverlay();
}

void DashboardDock::updateIndicators()
{
    for (const auto &kind : pinnedKinds) {
        bool running = false;
        bool active = false;
        for (const auto &item : m_panels) {
            if (item.kind == kind) {
                running = true;
                active = active || item.id == m_activePanel;
            }
        }
        auto *button = m_pinned.value(kind);
        if (button->property("dashboardRunning").toBool() != running) {
            button->setProperty("dashboardRunning", running);
            button->update();
        }
        button->setChecked(active);
        button->setAccessibleDescription(running ? tr("Running on this dashboard") : tr("Add to this dashboard"));
    }
    for (auto button = m_running.cbegin(); button != m_running.cend(); ++button) {
        button.value()->setChecked(button.key() == m_activePanel);
        button.value()->setAccessibleDescription(button.key() == m_activePanel ? tr("Active panel") : tr("Running panel"));
    }
}

void DashboardDock::updateFocusOrder()
{
    QWidget *previous = m_apps;
    for (const auto &kind : pinnedKinds) {
        if (kind == "desktop-window" && !m_desktopAppsEnabled)
            continue;
        QWidget *button = m_pinned.value(kind);
        QWidget::setTabOrder(previous, button);
        previous = button;
    }
    for (const auto &item : m_panels) {
        QWidget *button = m_running.value(item.id);
        QWidget::setTabOrder(previous, button);
        previous = button;
    }
}

bool DashboardDock::eventFilter(QObject *watched, QEvent *event)
{
    if (m_overlayArea) {
        if (watched == m_overlayArea && (event->type() == QEvent::Resize
            || event->type() == QEvent::Show || event->type() == QEvent::LayoutRequest))
            positionOverlay();
        if (watched == m_overlayArea && event->type() == QEvent::Hide) {
            m_hideTimer->stop();
            m_slide->stop();
            m_revealed = false;
            m_keyboardFocus = false;
            hide();
        }
        if ((watched == this || watched == m_revealEdge) && event->type() == QEvent::Enter)
            reveal();
        if ((watched == this || watched == m_revealEdge) && event->type() == QEvent::Leave)
            m_hideTimer->start();
        if (event->type() == QEvent::MouseButtonPress)
            m_keyboardFocus = false;
    }
    if (event->type() == QEvent::FocusIn) {
        const auto reason = static_cast<QFocusEvent *>(event)->reason();
        if (m_overlayArea && (reason == Qt::TabFocusReason || reason == Qt::BacktabFocusReason
            || reason == Qt::ShortcutFocusReason)) {
            m_keyboardFocus = true;
            reveal();
        }
        if (auto *button = qobject_cast<QToolButton *>(watched)) {
            if (m_content->isAncestorOf(button))
                m_scroll->ensureWidgetVisible(button, 8, 0);
        }
    }
    return QWidget::eventFilter(watched, event);
}

QSize DashboardDock::sizeHint() const
{
    return QSize(layout()->totalSizeHint().width(), height());
}

QSize DashboardDock::minimumSizeHint() const
{
    return QSize(120, height());
}
