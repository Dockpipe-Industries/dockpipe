#include "DashboardWorkspace.h"
#include "WindowControlButton.h"
#include "CanvasPanel.h"
#include "CanvasIcons.h"

#include <QApplication>
#include <QBuffer>
#include <QFile>
#include <QFileInfo>
#include <QImageReader>
#include <QImage>
#include <QLinearGradient>
#include <QPaintEvent>
#include <QPushButton>
#include <QDrag>
#include <QDragEnterEvent>
#include <QDragLeaveEvent>
#include <QDropEvent>
#include <QDynamicPropertyChangeEvent>
#include <QCursor>
#include <QHideEvent>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QMenu>
#include <QMimeData>
#include <QMouseEvent>
#include <QPainter>
#include <QShowEvent>
#include <QShortcut>
#include <QScopedValueRollback>
#include <algorithm>
#include <QSplitter>
#include <QStyle>
#include <QScrollArea>
#include <QScrollBar>
#include <QTabBar>
#include <QTabWidget>
#include <QToolButton>
#include <QTimer>
#include <QVBoxLayout>

namespace {
constexpr int MaximumPanels = 16;
constexpr int MaximumCoordinate = 32768;
const QString PanelMime = QStringLiteral("application/x-dockpipe-dashboard-panel");

QJsonObject tabs(const QString &id)
{
    return {{"type", "tabs"}, {"ids", QJsonArray{id}}, {"active", id}};
}

bool containsPanel(const QJsonObject &node, const QString &id)
{
    if (node.value("type") == "tabs")
        return node.value("ids").toArray().contains(id);
    for (const QJsonValue &child : node.value("children").toArray()) {
        if (containsPanel(child.toObject(), id))
            return true;
    }
    return false;
}

// Trees originate from validated input or bounded panel operations (16 leaves).
QJsonObject withoutPanel(QJsonObject node, const QString &id)
{
    if (node.value("type") == "tabs") {
        QJsonArray ids = node.value("ids").toArray();
        for (int index = ids.size() - 1; index >= 0; --index) {
            if (ids.at(index).toString() == id)
                ids.removeAt(index);
        }
        if (ids.isEmpty())
            return {};
        node["ids"] = ids;
        if (node.value("active").toString() == id)
            node["active"] = ids.first();
        return node;
    }
    QJsonArray children;
    for (const QJsonValue &child : node.value("children").toArray()) {
        const QJsonObject retained = withoutPanel(child.toObject(), id);
        if (!retained.isEmpty())
            children.append(retained);
    }
    if (children.size() == 1)
        return children.first().toObject();
    if (children.isEmpty())
        return {};
    node["children"] = children;
    return node;
}

QJsonObject insertPanel(QJsonObject node, const QString &id, const QString &target, const QString &position)
{
    if (node.value("type") == "tabs" && containsPanel(node, target)) {
        if (position == "tab") {
            QJsonArray ids = node.value("ids").toArray();
            ids.append(id);
            node["ids"] = ids;
            node["active"] = id;
            return node;
        }
        const bool before = position == "left" || position == "top";
        const bool horizontal = position == "left" || position == "right";
        QJsonArray children;
        children.append(before ? tabs(id) : node);
        children.append(before ? node : tabs(id));
        return {{"type", "split"}, {"orientation", horizontal ? "horizontal" : "vertical"},
                {"children", children}, {"sizes", QJsonArray{1, 1}}};
    }
    QJsonArray children;
    for (const QJsonValue &child : node.value("children").toArray())
        children.append(insertPanel(child.toObject(), id, target, position));
    node["children"] = children;
    return node;
}

bool validateTree(const QJsonObject &node, int depth, int &nodes, QSet<QString> &seen,
                  const QSet<QString> &known)
{
    // A binary split tree for 16 panel groups has at most 31 nodes and depth 16.
    if (depth > MaximumPanels || ++nodes > MaximumPanels * 2 - 1)
        return false;
    if (node.value("type") == "tabs") {
        const QJsonArray ids = node.value("ids").toArray();
        if (ids.isEmpty() || ids.size() > MaximumPanels)
            return false;
        for (const QJsonValue &value : ids) {
            const QString id = value.toString();
            if (!known.contains(id) || seen.contains(id))
                return false;
            seen.insert(id);
        }
        return ids.contains(node.value("active"));
    }
    if (node.value("type") != "split")
        return false;
    const QString orientation = node.value("orientation").toString();
    const QJsonArray children = node.value("children").toArray();
    const QJsonArray sizes = node.value("sizes").toArray();
    if ((orientation != "horizontal" && orientation != "vertical") || children.size() != 2 || sizes.size() != 2)
        return false;
    for (int index = 0; index < 2; ++index) {
        const double size = sizes.at(index).toDouble(-1);
        if (size < 1 || size > MaximumCoordinate || size != sizes.at(index).toInt(-1))
            return false;
        if (!validateTree(children.at(index).toObject(), depth + 1, nodes, seen, known))
            return false;
    }
    return true;
}

QJsonObject geometryJson(const QMap<QString, QRect> &geometries)
{
    QJsonObject result;
    for (auto iterator = geometries.cbegin(); iterator != geometries.cend(); ++iterator) {
        const QRect rect = iterator.value();
        result[iterator.key()] = QJsonArray{rect.x(), rect.y(), rect.width(), rect.height()};
    }
    return result;
}

bool readGeometry(const QJsonObject &object, const QSet<QString> &known, QMap<QString, QRect> &result)
{
    if (object.size() > MaximumPanels)
        return false;
    for (auto iterator = object.begin(); iterator != object.end(); ++iterator) {
        if (!known.contains(iterator.key()) || !iterator.value().isArray())
            return false;
        const QJsonArray values = iterator.value().toArray();
        if (values.size() != 4)
            return false;
        for (const QJsonValue &value : values) {
            const int number = value.toInt(-1);
            if (number < 0 || number > MaximumCoordinate || number != value.toDouble(-1))
                return false;
        }
        if (values.at(2).toInt() < 1 || values.at(3).toInt() < 1)
            return false;
        result[iterator.key()] = QRect(values.at(0).toInt(), values.at(1).toInt(),
                                      values.at(2).toInt(), values.at(3).toInt());
    }
    return true;
}
}

DashboardWorkspace::DashboardWorkspace(const QString &dashboardId, QWidget *parent)
    : QWidget(parent), m_dashboardId(dashboardId)
{
    setObjectName("dashboardWorkspace");
    setAcceptDrops(true);
    setMinimumSize(320, 220);
    m_parking = new QWidget(this);
    m_parking->hide();
    m_empty = new QLabel(tr("Your dashboard, your way\nAdd a widget to get started."), this);
    m_empty->setAlignment(Qt::AlignCenter);
    m_empty->setObjectName("dashboardEmptyState");
    m_dropPreview = new QWidget(this);
    m_dropPreview->setAttribute(Qt::WA_TransparentForMouseEvents);
    m_dropPreview->setStyleSheet("background: rgba(85, 185, 165, 70); border: 2px solid #79cbb6; border-radius: 8px;");
    m_dropPreview->hide();
    m_tileActionsTimer = new QTimer(this);
    m_tileActionsTimer->setObjectName("dashboardTileActionsTimer");
    // Foreign windows do not deliver QWidget hover events. Poll only while
    // this workspace is visible, inspecting at most the 16 admitted panels.
    m_tileActionsTimer->setInterval(100);
    connect(m_tileActionsTimer, &QTimer::timeout, this, &DashboardWorkspace::updateTileActions);
    auto *tileMenuShortcut = new QShortcut(QKeySequence(Qt::CTRL | Qt::ALT | Qt::Key_M), this);
    tileMenuShortcut->setObjectName("dashboardTileMenuShortcut");
    connect(tileMenuShortcut, &QShortcut::activated, this, &DashboardWorkspace::showFocusedTileMenu);
}

bool DashboardWorkspace::addPanel(const QString &id, const QString &title, QWidget *content)
{
    if (!content || id.isEmpty() || id.size() > 128 || m_panels.contains(id) || m_panels.size() >= MaximumPanels)
        return false;
    restoreExpandedPanel();
    rememberGeometry();
    if (m_tiles)
        m_tree = captureTree(m_tiles);
    auto *frame = new CanvasPanel(m_dashboardId + "/" + id, m_parking, false);
    // Native browser and captured-window surfaces cannot use QWidget effects.
    frame->setGraphicsEffect(nullptr);
    frame->setObjectName("dashboardPanel_" + id);
    frame->setMinimumSize(280, 180);
    frame->setProperty("canvasPanelTransparent", content->property("canvasPanelTransparent").toBool());
    const bool externalApp = content->property("dashboardExternalApp").toBool();
    frame->setProperty("dashboardExternalApp", externalApp);
    content->installEventFilter(this);
    auto *layout = new QVBoxLayout(frame);
    layout->setContentsMargins(7, 5, 7, 7);
    layout->setSpacing(4);
    auto *header = new QWidget(frame);
    header->setObjectName("dashboardPanelHeader");
    header->setProperty("dashboardPanelId", id);
    header->installEventFilter(this);
    auto *headerLayout = new QHBoxLayout(header);
    headerLayout->setContentsMargins(5, 0, 0, 0);
    auto *label = new QLabel(title, header);
    label->setObjectName("dashboardPanelTitle");
    label->setTextFormat(Qt::PlainText);
    label->setAttribute(Qt::WA_TransparentForMouseEvents);
    headerLayout->addWidget(label, 1);
    auto *menu = new QToolButton(header);
    menu->setText(QString::fromUtf8("⋯"));
    menu->setToolTip(tr("Arrange widget"));
    menu->setAccessibleName(tr("Arrange %1").arg(title));
    headerLayout->addWidget(menu);
    auto *expand = new WindowControlButton(WindowControlButton::Maximize, header);
    expand->setObjectName("dashboardPanelExpand");
    expand->setFixedSize(22, 22);
    expand->setAccessibleName(tr("Expand panel"));
    expand->setToolTip(tr("Expand panel"));
    headerLayout->insertWidget(0, expand);
    connect(expand, &QToolButton::clicked, this, [this, id] { togglePanelExpanded(id); });
    auto *close = new WindowControlButton(WindowControlButton::Close, header);
    close->setObjectName("dashboardPanelClose");
    close->setFixedSize(24, 24);
    close->setToolTip(tr("Remove widget"));
    close->setAccessibleName(tr("Remove %1").arg(title));
    headerLayout->insertWidget(0, close);
    auto *minimize = new WindowControlButton(WindowControlButton::Minimize, header);
    minimize->setObjectName("dashboardPanelMinimize");
    minimize->setFixedSize(22, 22);
    minimize->setToolTip(tr("Minimize to dock"));
    minimize->setAccessibleName(tr("Minimize %1 to dock").arg(title));
    headerLayout->insertWidget(1, minimize);
    connect(minimize, &QToolButton::clicked, this, [this, id] { minimizePanel(id); });
    layout->addWidget(header);
    layout->addWidget(content, 1);
    frame->setDragHandle(header);
    connect(menu, &QToolButton::clicked, this, [this, id, menu] {
        showPanelMenu(id, menu->mapToGlobal(QPoint(0, menu->height())));
    });
    connect(close, &QToolButton::clicked, this, [this, id] { emit panelCloseRequested(id); });
    connect(frame, &CanvasPanel::activated, this, [this, id] {
        if (m_rebuilding || !m_expandedPanel.isEmpty() || m_mode != "canvas"
            || m_canvasOrder.last() == id)
            return;
        m_canvasOrder.removeAll(id);
        m_canvasOrder.append(id);
        emit layoutChanged();
    });
    connect(frame, &CanvasPanel::placementChanged, this, [this] {
        if (!m_rebuilding) {
            restoreExpandedPanel();
            rememberGeometry();
            emit layoutChanged();
        }
    });
    auto *footer = content->findChild<QWidget *>("dashboardWidgetFooter");
    QToolButton *tileActions = nullptr;
    if (externalApp) {
        tileActions = new QToolButton(frame);
        tileActions->setObjectName("dashboardTileActions");
        // A native sibling can appear above an embedded application's native
        // surface. A painted QWidget overlay alone is occluded by that surface.
        tileActions->setAttribute(Qt::WA_NativeWindow);
        tileActions->setText(QString::fromUtf8("⋯"));
        tileActions->setFixedSize(30, 24);
        tileActions->setFocusPolicy(Qt::StrongFocus);
        tileActions->setAccessibleName(tr("Tile actions for %1").arg(title));
        tileActions->setToolTip(tr("Tile actions · Ctrl+Alt+M when dashboard has focus"));
        tileActions->setContextMenuPolicy(Qt::CustomContextMenu);
        tileActions->hide();
        connect(tileActions, &QToolButton::clicked, this, [this, id, tileActions] {
            showPanelMenu(id, tileActions->mapToGlobal(QPoint(0, tileActions->height())));
        });
        connect(tileActions, &QWidget::customContextMenuRequested, this,
                [this, id, tileActions](const QPoint &position) {
                    showPanelMenu(id, tileActions->mapToGlobal(position));
                });
    }
    m_panels.insert(id, {frame, header, title, {}, footer, footer && footer->isHidden(), expand, tileActions});
    frame->setContextMenuPolicy(Qt::CustomContextMenu);
    connect(frame, &QWidget::customContextMenuRequested, this, [this, id, frame](const QPoint &position) {
        showPanelMenu(id, frame->mapToGlobal(position));
    });
    m_canvasOrder.append(id);
    m_canvasGeometry[id] = defaultGeometry(id, m_canvasGeometry);
    if (!isVisible())
        m_pendingPlacement.insert(id);
    if (m_tree.isEmpty()) {
        m_tree = tabs(id);
    } else if (m_panels.size() <= 4) {
        const QString orientation = m_panels.size() % 2 == 0 ? "horizontal" : "vertical";
        m_tree = {{"type", "split"}, {"orientation", orientation},
                  {"children", QJsonArray{m_tree, tabs(id)}}, {"sizes", QJsonArray{1, 1}}};
    } else {
        QJsonObject first = m_tree;
        while (first.value("type") == "split")
            first = first.value("children").toArray().first().toObject();
        m_tree = insertPanel(m_tree, id, first.value("ids").toArray().first().toString(), "tab");
    }
    rebuild();
    emit layoutChanged();
    return true;
}

void DashboardWorkspace::removePanel(const QString &id)
{
    if (!m_panels.contains(id))
        return;
    restoreExpandedPanel();
    rememberGeometry();
    if (m_tiles)
        m_tree = captureTree(m_tiles);
    delete m_panels.take(id).frame;
    m_canvasGeometry.remove(id);
    m_canvasOrder.removeAll(id);
    m_pendingPlacement.remove(id);
    m_minimized.remove(id);
    m_floatingGeometry.remove(id);
    m_floating.remove(id);
    m_tree = withoutPanel(m_tree, id);
    rebuild();
    emit layoutChanged();
}

QString DashboardWorkspace::mode() const { return m_mode; }

void DashboardWorkspace::setMode(const QString &mode)
{
    if ((mode != "canvas" && mode != "tiling") || mode == m_mode)
        return;
    restoreExpandedPanel();
    rememberGeometry();
    if (m_tiles)
        m_tree = captureTree(m_tiles);
    m_mode = mode;
    rebuild();
    update();
    emit layoutChanged();
}

void DashboardWorkspace::rememberGeometry()
{
    if (!m_expandedPanel.isEmpty() || m_restoredGeometryPending)
        return;
    if (m_mode == "canvas") {
        QStringList order;
        for (QObject *child : children()) {
            for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
                if (iterator.value().frame == child)
                    order.append(iterator.key());
            }
        }
        if (order.size() == m_panels.size())
            m_canvasOrder = order;
    }
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_mode == "canvas")
            m_canvasGeometry[iterator.key()] = iterator.value().frame->geometry();
        else if (m_floating.contains(iterator.key()))
            m_floatingGeometry[iterator.key()] = iterator.value().frame->geometry();
    }
}

QWidget *DashboardWorkspace::buildTree(const QJsonObject &node, QWidget *parent)
{
    if (node.isEmpty())
        return nullptr;
    if (node.value("type") == "tabs") {
        auto *group = new QTabWidget(parent);
        group->setObjectName("dashboardTabGroup");
        group->setDocumentMode(true);
        group->setMovable(true);
        group->setTabsClosable(true);
        group->tabBar()->setObjectName("dashboardTabBar");
        group->tabBar()->setElideMode(Qt::ElideRight);
        group->tabBar()->setExpanding(true);
        group->tabBar()->setContextMenuPolicy(Qt::CustomContextMenu);
        group->tabBar()->setToolTip(tr("Drag to arrange · Right-click for widget actions"));
        connect(group, &QTabWidget::tabCloseRequested, this, [this, group](int index) {
            const QString id = panelId(group->widget(index));
            if (!id.isEmpty())
                emit panelCloseRequested(id);
        });
        connect(group->tabBar(), &QWidget::customContextMenuRequested, this, [this, group](const QPoint &position) {
            const int index = group->tabBar()->tabAt(position);
            const QString id = panelId(group->widget(index));
            if (!id.isEmpty())
                showPanelMenu(id, group->tabBar()->mapToGlobal(position));
        });
        group->tabBar()->installEventFilter(this);
        for (const QJsonValue &value : node.value("ids").toArray()) {
            const QString id = value.toString();
            CanvasPanel *frame = m_panels.value(id).frame;
            frame->setInteractionEnabled(false);
            applyPanelChrome(id, true);
            m_panels.value(id).header->setCursor(Qt::OpenHandCursor);
            m_panels.value(id).header->setToolTip(tr("Drag onto another widget: center for a tab, edge for a split"));
            // QTabWidget owns page visibility. Showing an inactive page here
            // also maps native browser/terminal children over the selected tab.
            const int tabIndex = group->addTab(frame, m_panels.value(id).icon, m_panels.value(id).title);
            // Reuse the same accessible control as the app and canvas panels.
            for (const auto side : {QTabBar::LeftSide, QTabBar::RightSide}) {
                if (auto *previous = group->tabBar()->tabButton(tabIndex, side)) {
                    group->tabBar()->setTabButton(tabIndex, side, nullptr);
                    previous->deleteLater();
                }
            }
            auto *controls = new QWidget(group->tabBar());
            auto *controlLayout = new QHBoxLayout(controls);
            controlLayout->setContentsMargins(0, 0, 0, 0);
            controlLayout->setSpacing(2);
            auto *close = new WindowControlButton(WindowControlButton::Close, controls);
            close->setObjectName("dashboardTabClose");
            close->setFixedSize(22, 22);
            close->setAccessibleName(tr("Close panel"));
            close->setToolTip(tr("Close panel"));
            controlLayout->addWidget(close);
            auto *minimize = new WindowControlButton(WindowControlButton::Minimize, controls);
            minimize->setObjectName("dashboardTabMinimize");
            minimize->setFixedSize(22, 22);
            minimize->setAccessibleName(tr("Minimize panel to dock"));
            minimize->setToolTip(tr("Minimize to dock"));
            controlLayout->addWidget(minimize);
            connect(minimize, &QToolButton::clicked, this, [this, id] { minimizePanel(id); });
            connect(close, &QToolButton::clicked, group, [group, frame]() {
                emit group->tabCloseRequested(group->indexOf(frame));
            });
            auto *expand = new WindowControlButton(WindowControlButton::Maximize, controls);
            expand->setObjectName("dashboardTabExpand");
            expand->setProperty("dashboardPanelId", id);
            expand->setFixedSize(22, 22);
            expand->setAccessibleName(tr("Expand panel"));
            expand->setToolTip(tr("Expand panel"));
            controlLayout->addWidget(expand);
            group->tabBar()->setTabButton(tabIndex, QTabBar::LeftSide, controls);
            connect(expand, &QToolButton::clicked, this, [this, id] { togglePanelExpanded(id); });
            if (id == node.value("active").toString())
                group->setCurrentWidget(frame);
        }
        connect(group, &QTabWidget::currentChanged, this, [this, group] {
            if (m_rebuilding)
                return;
            if (!m_expandedPanel.isEmpty() && m_expandedGroup == group) {
                m_expandedPanel = panelId(group->currentWidget());
                updateExpansionControls();
            } else {
                emit layoutChanged();
            }
        });
        connect(group->tabBar(), &QTabBar::tabMoved, this, [this] { emit layoutChanged(); });
        return group;
    }
    const Qt::Orientation orientation = node.value("orientation") == "horizontal" ? Qt::Horizontal : Qt::Vertical;
    auto *split = new QSplitter(orientation, parent);
    split->setObjectName("dashboardSplit");
    split->setProperty("dashboardCompact", m_compactTiling);
    split->setChildrenCollapsible(false);
    split->setHandleWidth(m_compactTiling ? 0 : 7);
    for (const QJsonValue &child : node.value("children").toArray())
        split->addWidget(buildTree(child.toObject(), split));
    QList<int> sizes;
    for (const QJsonValue &size : node.value("sizes").toArray())
        sizes.append(size.toInt(1));
    split->setSizes(sizes);
    connect(split, &QSplitter::splitterMoved, this, [this] { emit layoutChanged(); });
    return split;
}

QJsonObject DashboardWorkspace::captureTree(QWidget *widget) const
{
    if (auto *split = qobject_cast<QSplitter *>(widget)) {
        QJsonArray children;
        QJsonArray sizes;
        for (int index = 0; index < split->count(); ++index) {
            children.append(captureTree(split->widget(index)));
            const QList<int> savedSizes = split->property("dashboardUnminimizedSizes").value<QList<int>>();
            const QList<int> currentSizes = savedSizes.isEmpty() ? split->sizes() : savedSizes;
            sizes.append(qMax(1, currentSizes.at(index)));
        }
        return {{"type", "split"}, {"orientation", split->orientation() == Qt::Horizontal ? "horizontal" : "vertical"},
                {"children", children}, {"sizes", sizes}};
    }
    auto *group = qobject_cast<QTabWidget *>(widget);
    if (!group)
        return {};
    QJsonArray ids;
    QString active;
    for (int index = 0; index < group->count(); ++index) {
        for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
            if (iterator.value().frame == group->widget(index)) {
                ids.append(iterator.key());
                if (index == group->currentIndex())
                    active = iterator.key();
            }
        }
    }
    if (active.isEmpty() && !ids.isEmpty())
        active = ids.first().toString();
    return {{"type", "tabs"}, {"ids", ids}, {"active", active}};
}

QSize DashboardWorkspace::minimumPanelSize(const QString &id) const
{
    const CanvasPanel *frame = m_panels.value(id).frame;
    if (!frame)
        return QSize(280, 180);
    return frame->minimumSize().expandedTo(frame->minimumSizeHint()).expandedTo(QSize(280, 180));
}

QRect DashboardWorkspace::defaultGeometry(const QString &id, const QMap<QString, QRect> &occupied) const
{
    constexpr int gap = 16;
    const QRect bounds = rect().adjusted(8, 8, -8, -8);
    const QSize minimum = minimumPanelSize(id);
    const QSize preferred = QSize(440, 300).expandedTo(minimum);
    QList<int> columns{bounds.left()};
    QList<int> rows{bounds.top()};
    for (const QRect &area : occupied) {
        columns.append(area.right() + gap + 1);
        rows.append(area.bottom() + gap + 1);
    }
    std::sort(columns.begin(), columns.end());
    std::sort(rows.begin(), rows.end());
    // At most 17 x 17 candidates for the 16-widget product limit. Prefer full
    // card size; use a smaller free rectangle only when its content still fits.
    for (bool allowCompact : {false, true}) {
        for (int top : rows) {
            for (int left : columns) {
                const QSize remaining(bounds.right() - left + 1, bounds.bottom() - top + 1);
                const QSize size = allowCompact ? preferred.boundedTo(remaining) : preferred;
                if (size.width() < minimum.width() || size.height() < minimum.height())
                    continue;
                const QRect candidate(QPoint(left, top), size);
                if (!bounds.contains(candidate))
                    continue;
                bool overlaps = false;
                for (const QRect &area : occupied) {
                    if (candidate.intersects(area.adjusted(-gap / 2, -gap / 2, gap / 2, gap / 2))) {
                        overlaps = true;
                        break;
                    }
                }
                if (!overlaps)
                    return candidate;
            }
        }
    }
    const int offset = occupied.size() * 28;
    return constrainedGeometry(id, QRect(QPoint(8 + offset, 8 + offset), preferred));
}

QRect DashboardWorkspace::constrainedGeometry(const QString &id, const QRect &geometry) const
{
    const QSize available(qMax(1, width() - 16), qMax(1, height() - 16));
    const QSize size = geometry.size().expandedTo(minimumPanelSize(id)).boundedTo(available);
    return QRect(QPoint(qBound(8, geometry.x(), qMax(8, width() - size.width() - 8)),
                        qBound(8, geometry.y(), qMax(8, height() - size.height() - 8))), size);
}

void DashboardWorkspace::placePendingPanels()
{
    if (m_pendingPlacement.isEmpty())
        return;
    QMap<QString, QRect> occupied = m_canvasGeometry;
    for (const QString &id : m_pendingPlacement)
        occupied.remove(id);
    for (const QString &id : m_canvasOrder) {
        if (!m_pendingPlacement.contains(id))
            continue;
        const QRect geometry = defaultGeometry(id, occupied);
        m_canvasGeometry[id] = geometry;
        occupied[id] = geometry;
        if (m_mode == "canvas")
            m_panels.value(id).frame->setGeometry(geometry);
    }
    m_pendingPlacement.clear();
    emit layoutChanged();
}

void DashboardWorkspace::rebuild()
{
    m_rebuilding = true;
    for (const Panel &panel : m_panels) {
        panel.frame->hide();
        // Parking protects pages owned by the viewport we are about to delete.
        // Canvas pages can move straight to their new tabs without a second
        // parent change and stylesheet/layout pass over every child widget.
        if (m_tileViewport)
            panel.frame->setParent(m_parking);
    }
    delete m_tileViewport;
    m_tileViewport = nullptr;
    m_tiles = nullptr;
    if (m_mode == "tiling" && !m_tree.isEmpty()) {
        m_tileViewport = new QScrollArea(this);
        m_tileViewport->setObjectName("dashboardTileViewport");
        m_tileViewport->setFrameShape(QFrame::NoFrame);
        m_tileViewport->setWidgetResizable(true);
        m_tiles = buildTree(m_tree, m_tileViewport);
        m_tileViewport->setWidget(m_tiles);
        m_tileViewport->setAutoFillBackground(false);
        m_tileViewport->viewport()->setAutoFillBackground(false);
        m_tiles->setAutoFillBackground(false);
        m_tileViewport->setGeometry(rect().adjusted(8, 8, -8, -8));
        m_tileViewport->show();
    }
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_mode != "canvas" && !m_floating.contains(iterator.key()))
            continue;
        CanvasPanel *frame = iterator.value().frame;
        frame->setParent(this);
        frame->setInteractionEnabled(true);
        applyPanelChrome(iterator.key(), false);
        iterator.value().header->setToolTip(tr("Drag to move · Resize from the edges or lower-right corner"));
        const auto &geometries = m_mode == "canvas" ? m_canvasGeometry : m_floatingGeometry;
        frame->setGeometry(constrainedGeometry(iterator.key(), geometries.value(iterator.key(), QRect(24, 24, 440, 300))));
        frame->show();
        frame->raise();
    }
    if (m_mode == "canvas") {
        for (const QString &id : m_canvasOrder) {
            if (m_panels.contains(id))
                m_panels.value(id).frame->raise();
        }
    }
    applyTilingChrome();
    applyMinimizedPanels();
    m_empty->setGeometry(rect());
    m_empty->setVisible(m_panels.isEmpty());
    m_dropPreview->hide();
    m_rebuilding = false;
    updateTileActionsPolling();
}

bool DashboardWorkspace::minimizePanel(const QString &id)
{
    if (!m_panels.contains(id) || m_minimized.contains(id))
        return false;
    restoreExpandedPanel();
    rememberGeometry();
    m_minimized.insert(id);
    applyMinimizedPanels();
    return true;
}

void DashboardWorkspace::applyMinimizedPanels()
{
    QScopedValueRollback<bool> rebuilding(m_rebuilding, true);
    // Keep live widgets and native attachments in place. The dock restores them
    // through activatePanel; minimization is session state, not a saved layout.
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_mode == "canvas" || m_floating.contains(iterator.key()))
            iterator.value().frame->setVisible(!m_minimized.contains(iterator.key()));
    }
    if (!m_tiles)
        return;
    QList<QSplitter *> splits = m_tiles->findChildren<QSplitter *>();
    if (auto *split = qobject_cast<QSplitter *>(m_tiles))
        splits.prepend(split);
    for (auto *split : splits) {
        if (!split->property("dashboardUnminimizedSizes").isValid())
            split->setProperty("dashboardUnminimizedSizes", QVariant::fromValue(split->sizes()));
    }
    QList<QTabWidget *> groups = m_tiles->findChildren<QTabWidget *>();
    if (auto *group = qobject_cast<QTabWidget *>(m_tiles))
        groups.append(group);
    for (auto *group : groups) {
        bool visible = false;
        for (int index = 0; index < group->count(); ++index) {
            const bool tabVisible = !m_minimized.contains(panelId(group->widget(index)));
            group->setTabVisible(index, tabVisible);
            visible |= tabVisible;
        }
        group->setVisible(visible);
    }
    // Children are visited before parents so empty split branches also collapse.
    for (auto iterator = splits.crbegin(); iterator != splits.crend(); ++iterator) {
        QSplitter *split = *iterator;
        bool visible = false;
        for (int index = 0; index < split->count(); ++index)
            visible |= !split->widget(index)->isHidden();
        split->setVisible(visible);
    }
    if (m_minimized.isEmpty()) {
        for (auto *split : splits) {
            split->refresh();
            split->setSizes(split->property("dashboardUnminimizedSizes").value<QList<int>>());
            split->setProperty("dashboardUnminimizedSizes", QVariant());
        }
    }
}

bool DashboardWorkspace::activatePanel(const QString &id)
{
    if (!m_panels.contains(id))
        return false;
    if (!m_expandedPanel.isEmpty() && id != m_expandedPanel)
        restoreExpandedPanel();
    if (m_minimized.remove(id))
        applyMinimizedPanels();
    auto *frame = m_panels.value(id).frame;
    for (auto *parent = frame->parentWidget(); parent && parent != this; parent = parent->parentWidget()) {
        if (auto *tabs = qobject_cast<QTabWidget *>(parent))
            tabs->setCurrentWidget(frame);
    }
    if (m_mode == "canvas" || m_floating.contains(id)) {
        frame->raise();
        if (m_mode == "canvas") {
            m_canvasOrder.removeAll(id);
            m_canvasOrder.append(id);
        }
    } else if (m_tileViewport) {
        m_tileViewport->ensureWidgetVisible(frame);
    }
    frame->setFocusPolicy(Qt::StrongFocus);
    frame->setFocus(Qt::OtherFocusReason);
    emit layoutChanged();
    return true;
}

QJsonObject DashboardWorkspace::saveLayout() const
{
    if (!m_expandedPanel.isEmpty())
        return m_expansionLayout;
    auto canvas = m_canvasGeometry;
    auto floating = m_floatingGeometry;
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_restoredGeometryPending)
            continue;
        if (m_mode == "canvas")
            canvas[iterator.key()] = iterator.value().frame->geometry();
        else if (m_floating.contains(iterator.key()))
            floating[iterator.key()] = iterator.value().frame->geometry();
    }
    QStringList order = m_canvasOrder;
    if (m_mode == "canvas") {
        order.clear();
        for (QObject *child : children()) {
            for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
                if (iterator.value().frame == child)
                    order.append(iterator.key());
            }
        }
    }
    QJsonArray canvasOrder;
    for (const QString &id : order)
        canvasOrder.append(id);
    QJsonArray floatingIds;
    for (const QString &id : m_floating)
        floatingIds.append(id);
    return {{"version", 1}, {"mode", m_mode}, {"canvas", geometryJson(canvas)}, {"canvasOrder", canvasOrder},
            {"floatingGeometry", geometryJson(floating)}, {"floating", floatingIds},
            {"tree", m_tiles ? captureTree(m_tiles) : m_tree}};
}

bool DashboardWorkspace::restoreLayout(const QJsonObject &layout)
{
    const QString mode = layout.value("mode").toString();
    if (layout.value("version").toInt() != 1 || (mode != "canvas" && mode != "tiling") ||
        !layout.value("canvas").isObject() || !layout.value("floatingGeometry").isObject() ||
        !layout.value("tree").isObject() || !layout.value("floating").isArray())
        return false;
    const QList<QString> keys = m_panels.keys();
    const QSet<QString> known(keys.cbegin(), keys.cend());
    const QJsonArray canvasOrder = layout.value("canvasOrder").toArray();
    QSet<QString> ordered;
    QStringList order;
    if (canvasOrder.size() != known.size())
        return false;
    for (const QJsonValue &value : canvasOrder) {
        const QString id = value.toString();
        if (!known.contains(id) || ordered.contains(id))
            return false;
        ordered.insert(id);
        order.append(id);
    }
    QSet<QString> seen;
    QSet<QString> floating;
    const QJsonArray floatingIds = layout.value("floating").toArray();
    if (floatingIds.size() > MaximumPanels)
        return false;
    for (const QJsonValue &value : floatingIds) {
        const QString id = value.toString();
        if (!known.contains(id) || seen.contains(id))
            return false;
        seen.insert(id);
        floating.insert(id);
    }
    const QJsonObject tree = layout.value("tree").toObject();
    int nodes = 0;
    if (!tree.isEmpty() && !validateTree(tree, 1, nodes, seen, known))
        return false;
    if (seen != known)
        return false;
    QMap<QString, QRect> canvas;
    QMap<QString, QRect> floatingGeometry;
    if (!readGeometry(layout.value("canvas").toObject(), known, canvas) ||
        !readGeometry(layout.value("floatingGeometry").toObject(), known, floatingGeometry))
        return false;
    restoreExpandedPanel();
    m_mode = mode;
    m_canvasOrder = order;
    m_pendingPlacement.clear();
    m_tree = tree;
    m_floating = floating;
    m_canvasGeometry = canvas;
    m_floatingGeometry = floatingGeometry;
    // A page is restored before its parent assigns the visible workspace size.
    // Keep the saved rectangles authoritative until that first real layout.
    m_restoredGeometryPending = !isVisible();
    rebuild();
    return true;
}

bool DashboardWorkspace::dockPanel(const QString &id, const QString &targetId, const QString &position)
{
    const QSet<QString> positions{"left", "right", "top", "bottom", "tab"};
    if (!m_panels.contains(id) || id == targetId || !positions.contains(position))
        return false;
    restoreExpandedPanel();
    rememberGeometry();
    QJsonObject tree = m_tiles ? captureTree(m_tiles) : m_tree;
    tree = withoutPanel(tree, id);
    if (tree.isEmpty()) {
        if (!targetId.isEmpty())
            return false;
        tree = tabs(id);
    } else {
        if (!containsPanel(tree, targetId))
            return false;
        tree = insertPanel(tree, id, targetId, position);
    }
    m_tree = tree;
    m_floating.remove(id);
    rebuild();
    emit layoutChanged();
    return true;
}

bool DashboardWorkspace::floatPanel(const QString &id)
{
    if (!m_panels.contains(id) || m_floating.contains(id))
        return false;
    restoreExpandedPanel();
    rememberGeometry();
    m_tree = withoutPanel(m_tiles ? captureTree(m_tiles) : m_tree, id);
    m_floating.insert(id);
    m_floatingGeometry[id] = defaultGeometry(id, m_floatingGeometry);
    rebuild();
    emit layoutChanged();
    return true;
}

void DashboardWorkspace::showPanelMenu(const QString &id, const QPoint &position)
{
    if (!m_panels.contains(id))
        return;
    auto *menu = new QMenu(this);
    menu->setObjectName("dashboardPanelMenu");
    menu->setProperty("dashboardPanelId", id);
    menu->setAttribute(Qt::WA_DeleteOnClose);
    const QString expansionLabel = id == m_expandedPanel ? tr("Restore panel") : tr("Expand panel");
    connect(menu->addAction(expansionLabel), &QAction::triggered, this, [this, id] { togglePanelExpanded(id); });
    connect(menu->addAction(tr("Minimize to dock")), &QAction::triggered, this, [this, id] { minimizePanel(id); });
    menu->addSeparator();
    auto *openFullView = m_panels.value(id).frame->findChild<QPushButton *>("dashboardOpenFullView");
    if (openFullView) {
        auto *openAction = menu->addAction(openFullView->text());
        openAction->setEnabled(openFullView->isEnabled());
        connect(openAction, &QAction::triggered, openFullView, &QPushButton::click);
        menu->addSeparator();
    }
    if (m_mode == "tiling") {
        if (!m_floating.contains(id))
            connect(menu->addAction(tr("Float inside dashboard")), &QAction::triggered, this, [this, id] { floatPanel(id); });
        for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
            if (iterator.key() == id || m_floating.contains(iterator.key()))
                continue;
            QMenu *target = menu->addMenu(tr("Move beside %1").arg(iterator.value().title));
            const QString targetId = iterator.key();
            const QList<QPair<QString, QString>> placements{{tr("As tab"), "tab"}, {tr("Left"), "left"},
                {tr("Right"), "right"}, {tr("Above"), "top"}, {tr("Below"), "bottom"}};
            for (const auto &placement : placements) {
                connect(target->addAction(placement.first), &QAction::triggered, this,
                        [this, id, targetId, placement] { dockPanel(id, targetId, placement.second); });
            }
        }
        if (m_tree.isEmpty())
            connect(menu->addAction(tr("Return to tiles")), &QAction::triggered, this, [this, id] { dockPanel(id, {}, "tab"); });
        menu->addSeparator();
    }
    if (m_panels.value(id).frame->property("dashboardExternalApp").toBool()) {
        connect(menu->addAction(tr("Release to desktop")), &QAction::triggered, this,
                [this, id] { emit panelReleaseRequested(id); });
    } else {
        connect(menu->addAction(tr("Remove widget")), &QAction::triggered, this,
                [this, id] { emit panelCloseRequested(id); });
    }
    menu->popup(position);
}

bool DashboardWorkspace::eventFilter(QObject *object, QEvent *event)
{
    auto *header = qobject_cast<QWidget *>(object);
    if (header && event->type() == QEvent::DynamicPropertyChange) {
        auto *change = static_cast<QDynamicPropertyChangeEvent *>(event);
        auto *frame = qobject_cast<CanvasPanel *>(header->parentWidget());
        if (frame && change->propertyName() == "canvasPanelTransparent") {
            frame->setProperty("canvasPanelTransparent", header->property("canvasPanelTransparent").toBool());
            frame->style()->unpolish(frame);
            frame->style()->polish(frame);
            frame->update();
        }
    }
    if (!header || m_mode != "tiling" || !m_expandedPanel.isEmpty())
        return QWidget::eventFilter(object, event);
    QString id = header->property("dashboardPanelId").toString();
    // Dragging a tab out of its strip also permits docking it in another group.
    auto *tabBar = qobject_cast<QTabBar *>(header);
    if (event->type() == QEvent::MouseButtonPress) {
        auto *mouse = static_cast<QMouseEvent *>(event);
        if (tabBar) {
            auto *group = qobject_cast<QTabWidget *>(tabBar->parentWidget());
            const int index = tabBar->tabAt(mouse->position().toPoint());
            if (group && index >= 0) {
                for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
                    if (iterator.value().frame == group->widget(index))
                        id = iterator.key();
                }
            }
        }
        if (mouse->button() == Qt::LeftButton && !id.isEmpty() && !m_floating.contains(id)) {
            m_dragStart = mouse->position().toPoint();
            m_dragPanel = id;
        }
    } else if (event->type() == QEvent::MouseMove && !m_dragPanel.isEmpty()) {
        auto *mouse = static_cast<QMouseEvent *>(event);
        const bool outsideTabStrip = !tabBar || mouse->position().y() < -8 || mouse->position().y() > tabBar->height() + 8;
        if ((mouse->buttons() & Qt::LeftButton) && outsideTabStrip &&
            (mouse->position().toPoint() - m_dragStart).manhattanLength() >= QApplication::startDragDistance()) {
            auto *drag = new QDrag(this);
            auto *mime = new QMimeData;
            mime->setData(PanelMime, m_dashboardId.toUtf8() + '\n' + m_dragPanel.toUtf8());
            drag->setMimeData(mime);
            drag->exec(Qt::MoveAction);
            drag->deleteLater();
            m_dragPanel.clear();
            m_dropPreview->hide();
            return true;
        }
    } else if (event->type() == QEvent::MouseButtonRelease) {
        m_dragPanel.clear();
    }
    return QWidget::eventFilter(object, event);
}

bool DashboardWorkspace::acceptsDrag(const QMimeData *mime) const
{
    if (m_mode != "tiling" || !m_expandedPanel.isEmpty() || !mime->hasFormat(PanelMime))
        return false;
    const QByteArray bytes = mime->data(PanelMime);
    if (bytes.size() > 1024)
        return false;
    const QList<QByteArray> parts = bytes.split('\n');
    return parts.size() == 2 && parts.first() == m_dashboardId.toUtf8() && m_panels.contains(QString::fromUtf8(parts.last()));
}

QString DashboardWorkspace::panelAt(const QPoint &position) const
{
    QWidget *child = childAt(position);
    for (QWidget *ancestor = child; ancestor && ancestor != this; ancestor = ancestor->parentWidget()) {
        if (auto *bar = qobject_cast<QTabBar *>(ancestor)) {
            auto *group = qobject_cast<QTabWidget *>(bar->parentWidget());
            if (!group)
                continue;
            const int index = bar->tabAt(bar->mapFrom(this, position));
            return panelId(index >= 0 ? group->widget(index) : group->currentWidget());
        }
    }
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        CanvasPanel *frame = iterator.value().frame;
        if (!m_floating.contains(iterator.key()) && child && (child == frame || frame->isAncestorOf(child)))
            return iterator.key();
    }
    return {};
}

QString DashboardWorkspace::dropPosition(const QString &targetId, const QPoint &position) const
{
    if (!m_panels.contains(targetId))
        return "tab";
    const QWidget *frame = m_panels.value(targetId).frame;
    for (QWidget *ancestor = frame->parentWidget(); ancestor && ancestor != this; ancestor = ancestor->parentWidget()) {
        if (auto *group = qobject_cast<QTabWidget *>(ancestor)) {
            QTabBar *bar = group->tabBar();
            if (bar->rect().contains(bar->mapFrom(this, position)))
                return "tab";
        }
    }
    const QPoint point = frame->mapFrom(this, position);
    const double x = double(point.x()) / qMax(1, frame->width());
    const double y = double(point.y()) / qMax(1, frame->height());
    if (x < 0.24)
        return "left";
    if (x > 0.76)
        return "right";
    if (y < 0.24)
        return "top";
    if (y > 0.76)
        return "bottom";
    return "tab";
}

void DashboardWorkspace::showDropPreview(const QString &targetId, const QString &position)
{
    if (!m_panels.contains(targetId)) {
        m_dropPreview->hide();
        return;
    }
    QWidget *frame = m_panels.value(targetId).frame;
    QRect area(frame->mapTo(this, QPoint()), frame->size());
    if (position == "left")
        area.setWidth(area.width() / 2);
    else if (position == "right")
        area.setLeft(area.center().x());
    else if (position == "top")
        area.setHeight(area.height() / 2);
    else if (position == "bottom")
        area.setTop(area.center().y());
    m_dropPreview->setGeometry(area.adjusted(3, 3, -3, -3));
    m_dropPreview->show();
    m_dropPreview->raise();
}

void DashboardWorkspace::dragEnterEvent(QDragEnterEvent *event)
{
    if (acceptsDrag(event->mimeData()))
        event->acceptProposedAction();
}

void DashboardWorkspace::dragMoveEvent(QDragMoveEvent *event)
{
    if (!acceptsDrag(event->mimeData()))
        return;
    const QString target = panelAt(event->position().toPoint());
    showDropPreview(target, dropPosition(target, event->position().toPoint()));
    event->acceptProposedAction();
}

void DashboardWorkspace::dragLeaveEvent(QDragLeaveEvent *event)
{
    m_dropPreview->hide();
    event->accept();
}

void DashboardWorkspace::dropEvent(QDropEvent *event)
{
    m_dropPreview->hide();
    if (!acceptsDrag(event->mimeData()))
        return;
    const QString id = QString::fromUtf8(event->mimeData()->data(PanelMime).split('\n').last());
    const QString target = panelAt(event->position().toPoint());
    if (!target.isEmpty() && dockPanel(id, target, dropPosition(target, event->position().toPoint())))
        event->acceptProposedAction();
}

void DashboardWorkspace::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    m_empty->setGeometry(rect());
    if (m_tileViewport) {
        const int inset = m_compactTiling ? 0 : 8;
        m_tileViewport->setGeometry(rect().adjusted(inset, inset, -inset, -inset));
    }
    if (!m_expandedPanel.isEmpty()) {
        resizeExpandedPanel();
        return;
    }
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_mode != "canvas" && !m_floating.contains(iterator.key()))
            continue;
        auto *frame = iterator.value().frame;
        const auto &saved = m_mode == "canvas" ? m_canvasGeometry : m_floatingGeometry;
        const QRect geometry = m_restoredGeometryPending ? saved.value(iterator.key(), frame->geometry()) : frame->geometry();
        frame->setGeometry(constrainedGeometry(iterator.key(), geometry));
    }
}

void DashboardWorkspace::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    if (m_restoredGeometryPending) {
        for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
            if (m_mode != "canvas" && !m_floating.contains(iterator.key()))
                continue;
            auto *frame = iterator.value().frame;
            const auto &saved = m_mode == "canvas" ? m_canvasGeometry : m_floatingGeometry;
            frame->setGeometry(constrainedGeometry(iterator.key(), saved.value(iterator.key(), frame->geometry())));
        }
        m_restoredGeometryPending = false;
    }
    placePendingPanels();
    updateTileActionsPolling();
}

void DashboardWorkspace::hideEvent(QHideEvent *event)
{
    m_tileActionsTimer->stop();
    QWidget::hideEvent(event);
}

void DashboardWorkspace::paintEvent(QPaintEvent *event)
{
    QWidget::paintEvent(event);
    QPainter painter(this);
    const bool dark = palette().color(QPalette::Window).lightness() < 128;
    painter.fillRect(event->rect(), QColor(dark ? "#11151c" : "#f7f9fc"));
    if (m_backgroundKind == "image" && !m_backgroundImage.isNull()) {
        const QSizeF imageSize = m_backgroundImage.size();
        const qreal scale = qMax(width() / imageSize.width(), height() / imageSize.height());
        const QSizeF drawnSize = imageSize * scale;
        const QRectF destination(QPointF((width() - drawnSize.width()) / 2, (height() - drawnSize.height()) / 2), drawnSize);
        painter.setRenderHint(QPainter::SmoothPixmapTransform);
        painter.drawPixmap(destination, m_backgroundImage, QRectF(QPointF(), imageSize));
        if (m_panels.isEmpty()) {
            const QSize messageSize = m_empty->sizeHint() + QSize(48, 32);
            const QRect message(QPoint((width() - messageSize.width()) / 2,
                (height() - messageSize.height()) / 2), messageSize);
            painter.setPen(Qt::NoPen);
            painter.setBrush(dark ? QColor(18, 24, 32, 220) : QColor(250, 250, 250, 230));
            painter.drawRoundedRect(message, 10, 10);
        }
        return;
    }
    if (m_backgroundKind == "dusk" || m_backgroundKind == "ocean") {
        QLinearGradient gradient(0, 0, width(), height());
        if (m_backgroundKind == "dusk") {
            gradient.setColorAt(0, QColor(dark ? "#282238" : "#eee7f3"));
            gradient.setColorAt(1, QColor(dark ? "#131b2a" : "#dfe8f5"));
        } else {
            gradient.setColorAt(0, QColor(dark ? "#102c35" : "#e2f2ef"));
            gradient.setColorAt(1, QColor(dark ? "#142031" : "#e0eafa"));
        }
        painter.fillRect(event->rect(), gradient);
        return;
    }
    if (m_backgroundKind != "grid")
        return;
    // Match WorkflowCanvas's 32px grid and every-fourth-line emphasis.
    constexpr int spacing = 32;
    const QRect dirty = event->rect();
    const auto gridColor = [dark](int coordinate) {
        const bool major = (coordinate / spacing) % 4 == 0;
        if (major)
            return QColor(dark ? "#222b36" : "#dce3ed");
        return QColor(dark ? "#181f29" : "#e8edf3");
    };
    for (int x = dirty.left() / spacing * spacing; x <= dirty.right(); x += spacing) {
        painter.setPen(gridColor(x));
        painter.drawLine(x, dirty.top(), x, dirty.bottom());
    }
    for (int y = dirty.top() / spacing * spacing; y <= dirty.bottom(); y += spacing) {
        painter.setPen(gridColor(y));
        painter.drawLine(dirty.left(), y, dirty.right(), y);
    }
}

QString DashboardWorkspace::panelId(QWidget *frame) const
{
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (iterator.value().frame == frame)
            return iterator.key();
    }
    return {};
}

void DashboardWorkspace::applyPanelChrome(const QString &id, bool tiled)
{
    const Panel &panel = m_panels[id];
    panel.header->setVisible(!tiled && (m_showPanelHeaders || m_expandedPanel == id));
    const bool compact = tiled && (m_compactTiling || panel.frame->property("dashboardExternalApp").toBool());
    if (panel.footer)
        panel.footer->setVisible(!compact && !panel.footerInitiallyHidden);
    panel.frame->layout()->setContentsMargins(compact ? 0 : 7, compact ? 0 : 5, compact ? 0 : 7, compact ? 0 : 7);
    panel.frame->layout()->setSpacing(compact ? 0 : 4);
    if (panel.frame->property("dashboardTile").toBool() != tiled) {
        panel.frame->setProperty("dashboardTile", tiled);
        panel.frame->style()->unpolish(panel.frame);
        panel.frame->style()->polish(panel.frame);
    }
}

void DashboardWorkspace::applyTilingChrome()
{
    if (!m_tileViewport)
        return;
    const int inset = m_compactTiling ? 0 : 8;
    m_tileViewport->setGeometry(rect().adjusted(inset, inset, -inset, -inset));
    auto splits = m_tiles->findChildren<QSplitter *>();
    if (auto *split = qobject_cast<QSplitter *>(m_tiles))
        splits.append(split);
    for (auto *split : splits) {
        split->setHandleWidth(m_compactTiling ? 0 : 7);
        if (split->property("dashboardCompact").toBool() != m_compactTiling) {
            split->setProperty("dashboardCompact", m_compactTiling);
            split->style()->unpolish(split);
            split->style()->polish(split);
        }
    }
    auto groups = m_tiles->findChildren<QTabWidget *>();
    if (auto *group = qobject_cast<QTabWidget *>(m_tiles))
        groups.append(group);
    for (auto *group : groups) {
        const bool singleExternalApp = group->count() == 1
            && group->widget(0)->property("dashboardExternalApp").toBool();
        group->tabBar()->setVisible(!singleExternalApp);
        group->setProperty("dashboardSingleExternalApp", singleExternalApp);
    }
    updateTileActionsPolling();
}

void DashboardWorkspace::updateTileActionsPolling()
{
    bool hasExternalTile = false;
    for (const Panel &panel : m_panels)
        hasExternalTile |= panel.tileActions != nullptr;
    if (isVisible() && m_mode == "tiling" && hasExternalTile)
        m_tileActionsTimer->start();
    else
        m_tileActionsTimer->stop();
    updateTileActions();
}

void DashboardWorkspace::updateTileActions()
{
    const QPoint cursor = QCursor::pos();
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        const Panel &panel = iterator.value();
        if (!panel.tileActions)
            continue;
        QTabWidget *group = nullptr;
        for (QWidget *parent = panel.frame->parentWidget(); parent && parent != this; parent = parent->parentWidget()) {
            group = qobject_cast<QTabWidget *>(parent);
            if (group)
                break;
        }
        const bool eligible = isVisible() && m_mode == "tiling" && panel.frame->isVisible()
            && group && group->property("dashboardSingleExternalApp").toBool();
        const QRect revealArea(panel.frame->width() / 2 - 40, 0, 80, 32);
        QWidget *popup = QApplication::activePopupWidget();
        const bool menuOpen = popup && popup->property("dashboardPanelId").toString() == iterator.key();
        const bool reveal = eligible && (revealArea.contains(panel.frame->mapFromGlobal(cursor))
            || panel.tileActions->hasFocus() || menuOpen);
        panel.tileActions->move((panel.frame->width() - panel.tileActions->width()) / 2, 0);
        panel.tileActions->setVisible(reveal);
        if (reveal)
            panel.tileActions->raise();
    }
}

void DashboardWorkspace::showFocusedTileMenu()
{
    if (m_mode != "tiling")
        return;
    QWidget *focus = QApplication::focusWidget();
    QString target;
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        const Panel &panel = iterator.value();
        if (!panel.tileActions || !panel.frame->isVisible() || m_floating.contains(iterator.key()))
            continue;
        if (focus == panel.frame || (focus && panel.frame->isAncestorOf(focus))) {
            target = iterator.key();
            break;
        }
        if (target.isEmpty() || panel.frame->rect().contains(panel.frame->mapFromGlobal(QCursor::pos())))
            target = iterator.key();
    }
    if (target.isEmpty())
        return;
    CanvasPanel *frame = m_panels.value(target).frame;
    showPanelMenu(target, frame->mapToGlobal(QPoint(frame->width() / 2, 24)));
}

void DashboardWorkspace::setCompactTiling(bool compact)
{
    if (m_compactTiling == compact)
        return;
    restoreExpandedPanel();
    m_compactTiling = compact;
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator)
        applyPanelChrome(iterator.key(), m_mode == "tiling" && !m_floating.contains(iterator.key()));
    applyTilingChrome();
}

void DashboardWorkspace::setShowPanelHeaders(bool show)
{
    if (m_showPanelHeaders == show)
        return;
    m_showPanelHeaders = show;
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator)
        applyPanelChrome(iterator.key(), m_mode == "tiling" && !m_floating.contains(iterator.key()));
}

void DashboardWorkspace::setPanelTitle(const QString &id, const QString &title)
{
    if (!m_panels.contains(id) || title.size() > 512)
        return;
    Panel &panel = m_panels[id];
    panel.title = title;
    if (panel.tileActions)
        panel.tileActions->setAccessibleName(tr("Tile actions for %1").arg(title));
    panel.header->findChild<QLabel *>("dashboardPanelTitle")->setText(title);
    if (m_tiles) {
        auto groups = m_tiles->findChildren<QTabWidget *>();
        if (auto *group = qobject_cast<QTabWidget *>(m_tiles))
            groups.append(group);
        for (auto *group : groups) {
            const int index = group->indexOf(panel.frame);
            if (index >= 0)
                group->setTabText(index, title);
        }
    }
}

void DashboardWorkspace::setPanelIcon(const QString &id, const QIcon &icon)
{
    if (!m_panels.contains(id))
        return;
    Panel &panel = m_panels[id];
    panel.icon = icon;
    if (m_tiles) {
        auto groups = m_tiles->findChildren<QTabWidget *>();
        if (auto *group = qobject_cast<QTabWidget *>(m_tiles))
            groups.append(group);
        for (auto *group : groups) {
            const int index = group->indexOf(panel.frame);
            if (index >= 0)
                group->setTabIcon(index, icon);
        }
    }
}

bool DashboardWorkspace::setCanvasBackground(const QString &kind, const QString &path, bool refreshImage)
{
    const QSet<QString> kinds{"grid", "plain", "dusk", "ocean", "image"};
    if (!kinds.contains(kind))
        return false;
    if (kind == m_backgroundKind && (kind != "image" || (path == m_backgroundPath && !refreshImage)))
        return true;
    QPixmap image;
    if (kind == "image") {
        // Bound both compressed input and decoded pixels. Cache a single 4K
        // image; paints never inspect the filesystem or invoke an image decoder.
        constexpr qint64 maximumBytes = 20 * 1024 * 1024;
        constexpr qint64 maximumPixels = 16 * 1024 * 1024;
        if (path.isEmpty() || path.size() > 4096 || !QFileInfo(path).isFile())
            return false;
        QFile file(path);
        if (!file.open(QIODevice::ReadOnly) || file.size() > maximumBytes)
            return false;
        QByteArray bytes = file.read(maximumBytes + 1);
        if (file.error() != QFile::NoError || bytes.size() > maximumBytes)
            return false;
        QBuffer buffer(&bytes);
        buffer.open(QIODevice::ReadOnly);
        QImageReader reader(&buffer);
        const QSet<QByteArray> formats{"png", "jpeg", "webp", "bmp"};
        if (!formats.contains(reader.format()))
            return false;
        const QSize sourceSize = reader.size();
        if (!sourceSize.isValid() || sourceSize.width() > 16384 || sourceSize.height() > 16384 ||
            qint64(sourceSize.width()) * sourceSize.height() > maximumPixels)
            return false;
        reader.setAutoTransform(true);
        reader.setScaledSize(sourceSize.scaled(QSize(3840, 2160), Qt::KeepAspectRatio).boundedTo(sourceSize));
        const QImage decoded = reader.read();
        if (decoded.isNull())
            return false;
        image = QPixmap::fromImage(decoded);
        if (image.isNull())
            return false;
    }
    m_backgroundKind = kind;
    m_backgroundPath = kind == "image" ? path : QString();
    m_backgroundImage = image;
    update();
    return true;
}

void DashboardWorkspace::copyBackgroundFrom(const DashboardWorkspace &workspace)
{
    if (m_backgroundKind == workspace.m_backgroundKind && m_backgroundPath == workspace.m_backgroundPath
        && m_backgroundImage.cacheKey() == workspace.m_backgroundImage.cacheKey())
        return;
    m_backgroundKind = workspace.m_backgroundKind;
    m_backgroundPath = workspace.m_backgroundPath;
    // QPixmap is implicitly shared: both environments use one decoded image.
    m_backgroundImage = workspace.m_backgroundImage;
    update();
}

QString DashboardWorkspace::expandedPanel() const
{
    return m_expandedPanel;
}

bool DashboardWorkspace::togglePanelExpanded(const QString &id)
{
    if (!m_panels.contains(id))
        return false;
    if (m_expandedPanel == id) {
        restoreExpandedPanel();
        return true;
    }
    restoreExpandedPanel();
    m_expansionLayout = saveLayout();
    m_expandedPanel = id;
    CanvasPanel *frame = m_panels.value(id).frame;
    m_expansionGeometry = frame->geometry();
    m_rebuilding = true;
    if (m_tiles) {
        auto splitters = m_tiles->findChildren<QSplitter *>();
        if (auto *splitter = qobject_cast<QSplitter *>(m_tiles))
            splitters.prepend(splitter);
        for (auto *splitter : splitters)
            m_expansionSplitters.append({splitter, splitter->sizes()});
        auto groups = m_tiles->findChildren<QTabWidget *>();
        if (auto *group = qobject_cast<QTabWidget *>(m_tiles))
            groups.prepend(group);
        for (auto *group : groups) {
            m_expansionTabs.append({group, group->currentIndex()});
            group->setMovable(false);
            if (group->indexOf(frame) >= 0)
                m_expandedGroup = group;
        }
    }
    if (m_tileViewport) {
        m_expansionScroll = QPoint(m_tileViewport->horizontalScrollBar()->value(), m_tileViewport->verticalScrollBar()->value());
        if (!m_expandedGroup) {
            m_expansionVisibility.append({m_tileViewport, m_tileViewport->isHidden()});
            m_tileViewport->hide();
        }
    }
    // Direct canvas/floating siblings can be hidden without changing ownership.
    for (const Panel &panel : m_panels) {
        if (panel.frame->parentWidget() == this && panel.frame != frame) {
            m_expansionVisibility.append({panel.frame, panel.frame->isHidden()});
            panel.frame->hide();
        }
    }
    if (m_expandedGroup) {
        m_expandedGroup->setCurrentWidget(frame);
        QWidget *branch = m_expandedGroup;
        for (QWidget *parent = branch->parentWidget(); parent && parent != m_tileViewport;
             branch = parent, parent = parent->parentWidget()) {
            if (auto *splitter = qobject_cast<QSplitter *>(parent)) {
                for (int index = 0; index < splitter->count(); ++index) {
                    QWidget *sibling = splitter->widget(index);
                    if (sibling == branch)
                        continue;
                    m_expansionVisibility.append({sibling, sibling->isHidden()});
                    sibling->hide();
                }
            }
        }
    } else {
        frame->setInteractionEnabled(false);
        applyPanelChrome(id, false);
        resizeExpandedPanel();
        frame->raise();
    }
    m_rebuilding = false;
    updateExpansionControls();
    return true;
}

void DashboardWorkspace::resizeExpandedPanel()
{
    if (m_expandedPanel.isEmpty() || m_expandedGroup || !m_panels.contains(m_expandedPanel))
        return;
    m_panels.value(m_expandedPanel).frame->setGeometry(rect());
}

void DashboardWorkspace::updateExpansionControls()
{
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        WindowControlButton *button = iterator.value().expand;
        const bool expanded = iterator.key() == m_expandedPanel;
        button->setExpanded(expanded);
        button->setAccessibleName(expanded ? tr("Restore panel") : tr("Expand panel"));
        button->setToolTip(expanded ? tr("Restore panel") : tr("Expand panel"));
    }
    if (!m_tiles)
        return;
    for (auto *button : m_tiles->findChildren<QToolButton *>("dashboardTabExpand")) {
        const bool expanded = button->property("dashboardPanelId").toString() == m_expandedPanel;
        static_cast<WindowControlButton *>(button)->setExpanded(expanded);
        button->setAccessibleName(expanded ? tr("Restore panel") : tr("Expand panel"));
        button->setToolTip(expanded ? tr("Restore panel") : tr("Expand panel"));
    }
}

void DashboardWorkspace::restoreExpandedPanel()
{
    if (m_expandedPanel.isEmpty())
        return;
    const QString id = m_expandedPanel;
    const bool wasTiled = !m_expandedGroup.isNull();
    m_rebuilding = true;
    m_expandedPanel.clear();
    for (const auto &entry : m_expansionVisibility) {
        if (entry.widget)
            entry.widget->setHidden(entry.hidden);
    }
    for (const auto &entry : m_expansionTabs) {
        if (entry.group) {
            entry.group->setCurrentIndex(entry.currentIndex);
            entry.group->setMovable(true);
        }
    }
    for (const auto &entry : m_expansionSplitters) {
        if (entry.splitter) {
            // Showing a sibling schedules handle-layout work. Refresh it before
            // restoring sizes, otherwise the missing handle gap rescales them.
            entry.splitter->refresh();
            entry.splitter->setSizes(entry.sizes);
        }
    }
    if (!wasTiled && m_panels.contains(id)) {
        CanvasPanel *frame = m_panels.value(id).frame;
        frame->setGeometry(constrainedGeometry(id, m_expansionGeometry));
        frame->setInteractionEnabled(true);
        applyPanelChrome(id, false);
    }
    // A smaller workspace can arrive while siblings are hidden. Restore the
    // arrangement, then apply the same visible bounds as an ordinary resize.
    for (auto iterator = m_panels.cbegin(); iterator != m_panels.cend(); ++iterator) {
        if (m_mode == "canvas" || m_floating.contains(iterator.key())) {
            CanvasPanel *frame = iterator.value().frame;
            frame->setGeometry(constrainedGeometry(iterator.key(), frame->geometry()));
        }
    }
    // Raising the expanded canvas must not silently change saved overlap order.
    if (m_mode == "canvas") {
        for (const QJsonValue &value : m_expansionLayout.value("canvasOrder").toArray()) {
            const QString panel = value.toString();
            if (m_panels.contains(panel))
                m_panels.value(panel).frame->raise();
        }
    }
    if (m_tileViewport) {
        m_tileViewport->horizontalScrollBar()->setValue(m_expansionScroll.x());
        m_tileViewport->verticalScrollBar()->setValue(m_expansionScroll.y());
    }
    m_expandedGroup.clear();
    m_expansionLayout = {};
    m_expansionVisibility.clear();
    m_expansionSplitters.clear();
    m_expansionTabs.clear();
    m_rebuilding = false;
    updateExpansionControls();
}

