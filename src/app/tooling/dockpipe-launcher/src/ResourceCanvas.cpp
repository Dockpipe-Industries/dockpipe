#include "ResourceCanvas.h"
#include "CanvasView.h"
#include "ResourceLinkItem.h"
#include "CanvasPanel.h"
#include "CanvasIcons.h"

#include <QApplication>
#include <QFrame>
#include <QGridLayout>
#include <QSettings>
#include <QTimer>
#include <QSignalBlocker>
#include <QGraphicsObject>
#include <QGraphicsPathItem>
#include <QGraphicsScene>
#include <QGraphicsSceneHoverEvent>
#include <QHBoxLayout>
#include <QLabel>
#include <QLineEdit>
#include <QPainter>
#include <QPushButton>
#include <QScrollArea>
#include <QVBoxLayout>
#include <QWheelEvent>
#include <cmath>
#include <functional>

namespace {
class ResourceCard final : public QGraphicsObject {
public:
    explicit ResourceCard(const ResourceNode &node) : m_node(node)
    {
        setFlags(ItemIsMovable | ItemIsSelectable | ItemSendsGeometryChanges);
        setCursor(Qt::OpenHandCursor);
        setData(0, node.id);
        setData(3, node.kind);
        setData(4, node.inactive);
        setData(5, static_cast<int>(node.status));
        setData(6, node.motionEligible);
        setData(1, node.title + " " + node.detail + " " + node.state);
        setToolTip(node.title + "\n" + node.detail + "\n" + node.state);
    }
    QRectF boundingRect() const override { return QRectF(-3, -3, 266, 132); }
    std::function<void()> moved;
protected:
    QVariant itemChange(GraphicsItemChange change, const QVariant &value) override
    {
        if (change == ItemPositionHasChanged && moved)
            moved();
        return QGraphicsObject::itemChange(change, value);
    }
    void paint(QPainter *painter, const QStyleOptionGraphicsItem *, QWidget *) override
    {
        const bool dark = scene()->views().first()->palette().color(QPalette::Base).lightness() < 128;
        QColor accent(dark ? "#89b7ed" : "#346da9");
        if (m_node.kind == "network")
            accent = QColor("#a58bda");
        else if (m_node.kind == "volume" || m_node.kind == "mount")
            accent = QColor("#b7935e");
        const QString state = m_node.state.toLower();
        if (state.contains("failed") || state.contains("unhealthy") || state.contains("revoked"))
            accent = QColor("#df7685");
        switch (m_node.status) {
        case ResourceStatus::Running:
            accent = QColor(dark ? "#70c8a6" : "#217757");
            break;
        case ResourceStatus::Inactive:
            accent = QColor(dark ? "#8b97a8" : "#647184");
            break;
        case ResourceStatus::Warning:
            accent = QColor(dark ? "#e3b66d" : "#946017");
            break;
        case ResourceStatus::Error:
            accent = QColor(dark ? "#ee8290" : "#b33148");
            break;
        case ResourceStatus::Neutral:
            break;
        }
        painter->setRenderHint(QPainter::Antialiasing);
        painter->setPen(QPen(isSelected() ? accent : QColor(dark ? "#364252" : "#cfd9e7"), isSelected() ? 2 : 1));
        QColor surface(dark ? "#1c2430" : "#ffffff");
        if (m_node.inactive)
            surface = QColor(dark ? "#171d26" : "#edf0f5");
        painter->setBrush(surface);
        painter->drawRoundedRect(QRectF(0, 0, 260, 126), 12, 12);
        painter->setPen(accent);
        QFont small = painter->font();
        small.setPointSizeF(8.5);
        painter->setFont(small);
        painter->drawText(QRectF(18, 12, 224, 18), m_node.kind.toUpper());
        QFont title = small;
        title.setPointSizeF(11);
        title.setWeight(QFont::DemiBold);
        painter->setFont(title);
        QColor titleColor(dark ? "#ebeff6" : "#25374d");
        if (m_node.inactive)
            titleColor = QColor(dark ? "#99a5b5" : "#667487");
        painter->setPen(titleColor);
        painter->drawText(QRectF(18, 35, 224, 24), painter->fontMetrics().elidedText(m_node.title, Qt::ElideRight, 224));
        painter->setFont(small);
        painter->setPen(QColor(dark ? "#a8b6c9" : "#677b92"));
        painter->drawText(QRectF(18, 67, 224, 20), painter->fontMetrics().elidedText(m_node.detail.simplified(), Qt::ElideRight, 224));
        painter->setPen(accent);
        painter->setBrush(accent);
        painter->setPen(Qt::NoPen);
        if (m_node.inactive)
            painter->drawRoundedRect(QRectF(18, 103, 7, 7), 1, 1);
        else
            painter->drawEllipse(QRectF(18, 103, 7, 7));
        painter->setPen(accent);
        painter->drawText(QRectF(33, 96, 209, 20), painter->fontMetrics().elidedText(m_node.state, Qt::ElideRight, 209));
    }
private:
    ResourceNode m_node;
};
}

ResourceCanvas::ResourceCanvas(const QString &identity, const QString &title, const QString &description,
                               QWidget *details, QWidget *parent, bool scrollDetails) : QWidget(parent)
{
    m_identity = identity;
    m_animationTimer = new QTimer(this);
    m_animationTimer->setObjectName("dependencyAnimationTimer");
    m_animationTimer->setInterval(50);
    m_animationClock.start();
    connect(m_animationTimer, &QTimer::timeout, this, &ResourceCanvas::advanceAnimation);
    m_filters = new QFrame(this);
    m_filters->setProperty("canvasOverlay", true);
    m_filterLayout = new QGridLayout(m_filters);
    m_filterLayout->setContentsMargins(6, 6, 6, 6);
    m_filterLayout->setSpacing(4);
    m_filters->hide();
    m_view = new CanvasView(this);
    m_view->lower();
    m_view->setObjectName(identity + "Graph");
    m_view->viewport()->installEventFilter(this);
    m_heading = new QFrame(this);
    m_heading->setProperty("canvasOverlay", true);
    auto *headingLayout = new QVBoxLayout(m_heading);
    headingLayout->setContentsMargins(16, 12, 16, 12);
    auto *heading = new QLabel(title);
    heading->setObjectName("sectionTitle");
    headingLayout->addWidget(heading);
    auto *subtitle = new QLabel(description);
    subtitle->setWordWrap(true);
    subtitle->setMaximumWidth(440);
    headingLayout->addWidget(subtitle);
    m_controls = new QFrame(this);
    m_controls->setProperty("canvasOverlay", true);
    auto *controls = new QHBoxLayout(m_controls);
    controls->setContentsMargins(6, 6, 6, 6);
    auto *search = new QLineEdit;
    search->setPlaceholderText(tr("Find on canvas…"));
    search->setMaximumWidth(180);
    controls->addWidget(search);
    connect(search, &QLineEdit::textChanged, this, &ResourceCanvas::setFilter);
    auto *fit = new QPushButton;
    fit->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Fit));
    fit->setToolTip(tr("Fit graph"));
    fit->setAccessibleName(tr("Fit graph"));
    controls->addWidget(fit);
    connect(fit, &QPushButton::clicked, this, &ResourceCanvas::fitGraph);
    auto *inspect = new QPushButton(tr("Details & actions"));
    m_detailsButton = inspect;
    inspect->setObjectName(identity + "DetailsButton");
    controls->addWidget(inspect);
    connect(inspect, &QPushButton::clicked, this, &ResourceCanvas::showDetails);
    auto *reset = new QPushButton(tr("Reset layout"));
    controls->addWidget(reset);
    connect(reset, &QPushButton::clicked, this, [this]() {
        QMap<int, int> rows;
        for (auto *card : m_nodes) {
            const int column = card->data(2).toInt();
            card->setPos(column * 380, rows[column]++ * 190);
        }
        m_details->resetPlacement();
        arrange();
        fitGraph();
    });
    m_notice = new QLabel(this);
    m_notice->setProperty("canvasOverlay", true);
    m_notice->setWordWrap(true);
    m_notice->setMargin(12);
    m_details = new CanvasPanel(identity + "Details", this);
    m_details->setObjectName(identity + "DetailsPanel");
    auto *panelLayout = new QVBoxLayout(m_details);
    panelLayout->setContentsMargins(12, 10, 12, 12);
    auto *header = new QWidget;
    auto *headerLayout = new QHBoxLayout(header);
    headerLayout->setContentsMargins(0, 0, 0, 0);
    m_detailsTitle = new QLabel(title + tr(" · Details & actions"));
    headerLayout->addWidget(m_detailsTitle, 1);
    auto *close = new QPushButton;
    close->setIcon(CanvasIcons::icon(CanvasIcons::Symbol::Close));
    close->setAccessibleName(tr("Close details"));
    headerLayout->addWidget(close);
    connect(close, &QPushButton::clicked, m_details, &QWidget::hide);
    m_details->setDragHandle(header);
    panelLayout->addWidget(header);
    details->setObjectName("resourceDetailContent");
    if (scrollDetails) {
        auto *scroll = new QScrollArea;
        scroll->setWidgetResizable(true);
        scroll->setFrameShape(QFrame::NoFrame);
        scroll->setWidget(details);
        panelLayout->addWidget(scroll, 1);
    } else {
        panelLayout->addWidget(details, 1);
    }
    m_details->hide();
    connect(m_view->scene(), &QGraphicsScene::selectionChanged, this, [this]() {
        const auto selected = m_view->scene()->selectedItems();
        if (selected.isEmpty())
            return;
        showDetails();
        keepSelectionVisible(selected.first());
        emit resourceSelected(selected.first()->data(0).toString());
    });
}

void ResourceCanvas::setGraph(const QVector<ResourceNode> &nodes, const QVector<ResourceEdge> &edges, const QString &notice)
{
    QMap<QString, QPointF> positions;
    QString selected;
    for (auto it = m_nodes.begin(); it != m_nodes.end(); ++it) {
        positions.insert(it.key(), it.value()->pos());
        if (it.value()->isSelected())
            selected = it.key();
    }
    bool topologyChanged = positions.size() != nodes.size();
    for (const auto &node : nodes)
        topologyChanged = topologyChanged || !positions.contains(node.id);
    const QSignalBlocker blocker(m_view->scene());
    m_view->scene()->clear();
    m_nodes.clear();
    m_paths.clear();
    m_edges.clear();
    QMap<int, int> rows;
    // Rendering is bounded; original inventory remains in Details & actions.
    constexpr int maximumNodes = 1024;
    constexpr int maximumEdges = 4096;
    for (const auto &node : nodes) {
        if (m_nodes.size() >= maximumNodes)
            break;
        if (m_nodes.contains(node.id))
            continue;
        auto *card = new ResourceCard(node);
        card->setData(2, node.column);
        m_view->scene()->addItem(card);
        card->setPos(positions.value(node.id, QPointF(node.column * 380, rows[node.column]++ * 190)));
        card->setSelected(node.id == selected);
        card->moved = [this]() { updateEdges(); };
        m_nodes.insert(node.id, card);
    }
    for (const auto &edge : edges) {
        if (m_edges.size() >= maximumEdges)
            break;
        if (!m_nodes.contains(edge.from) || !m_nodes.contains(edge.to))
            continue;
        m_edges.append(edge);
        auto *path = new ResourceLinkItem;
        path->setFlowColor(QColor(edge.dependency ? "#a0e4ce" : "#a4c7f2"));
        m_view->scene()->addItem(path);
        path->setZValue(-1);
        path->setToolTip(edge.label);
        m_paths.append(path);
    }
    QString message = notice;
    if (nodes.size() > maximumNodes || edges.size() > maximumEdges)
        message += tr(" Graph limited to 1,024 objects and 4,096 links; the full inventory remains in Details & actions.");
    if (nodes.isEmpty() && message.isEmpty())
        message = tr("No resources to display. Open Details & actions to refresh or get started.");
    m_notice->setText(message);
    m_notice->setVisible(!message.isEmpty());
    updateEdges();
    updateVisibility();
    if ((!m_fitted || (topologyChanged && !m_userAdjusted))
        && !nodes.isEmpty() && isVisible() && width() > 300) {
        fitGraph();
        m_fitted = true;
    }
    arrange();
}

void ResourceCanvas::updateEdges()
{
    for (int i = 0; i < m_edges.size(); ++i) {
        const auto &edge = m_edges[i];
        const QPointF from = m_nodes[edge.from]->pos() + QPointF(260, 63);
        const QPointF to = m_nodes[edge.to]->pos() + QPointF(0, 63);
        QPainterPath path(from);
        const qreal bend = qMax(60.0, std::abs(to.x() - from.x()) / 2);
        path.cubicTo(from + QPointF(bend, 0), to - QPointF(bend, 0), to);
        static_cast<ResourceLinkItem *>(m_paths[i])->setFlowPath(path);
        if (edge.dependency) {
            path.moveTo(to + QPointF(-8, -5));
            path.lineTo(to);
            path.lineTo(to + QPointF(-8, 5));
        }
        const QColor color(edge.dependency ? "#75bda9" : "#798ba8");
        m_paths[i]->setPen(QPen(color, 1.6, edge.dependency ? Qt::SolidLine : Qt::DashLine));
        m_paths[i]->setPath(path);
    }
    m_view->scene()->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-180, -160, 180, 160));
}

QPushButton *ResourceCanvas::addFilterButton(const QString &key, const QString &label)
{
    auto *button = new QPushButton(label, m_filters);
    button->setObjectName(m_identity + "Filter_" + key);
    button->setCheckable(true);
    button->setProperty("canvasLayerFilter", true);
    const QString setting = "resourceCanvas/" + m_identity + "/show/" + key;
    button->setChecked(QSettings().value(setting, true).toBool());
    button->setToolTip(tr("Show or hide %1 on the canvas. Inventory and controls remain available.").arg(label.toLower()));
    m_filterLayout->addWidget(button, m_filterCount / 3, m_filterCount % 3);
    ++m_filterCount;
    connect(button, &QPushButton::toggled, this, [this, setting](bool checked) {
        QSettings().setValue(setting, checked);
        updateVisibility();
    });
    m_filters->show();
    arrange();
    return button;
}

void ResourceCanvas::addKindFilter(const QString &kind, const QString &label)
{
    m_kindFilters.insert(kind, addFilterButton(kind, label));
    updateVisibility();
}

void ResourceCanvas::addInactiveFilter(const QString &label)
{
    m_inactiveFilter = addFilterButton("inactive", label.isEmpty() ? tr("Stopped") : label);
    updateVisibility();
}

void ResourceCanvas::addDependencyFilter()
{
    m_dependencyFilter = addFilterButton("dependencies", tr("Dependencies"));
    m_motionFilter = addFilterButton("motion", tr("Motion"));
    m_motionFilter->setToolTip(tr("Animate graph connections. Decorative only; this does not indicate traffic or execution."));
    updateVisibility();
}

void ResourceCanvas::setFilter(const QString &filter)
{
    m_filter = filter;
    updateVisibility();
}

void ResourceCanvas::updateVisibility()
{
    const QSignalBlocker blocker(m_view->scene());
    for (auto *node : m_nodes) {
        const auto *kindFilter = m_kindFilters.value(node->data(3).toString(), nullptr);
        const bool kindVisible = !kindFilter || kindFilter->isChecked();
        const bool inactiveVisible = !m_inactiveFilter || m_inactiveFilter->isChecked() || !node->data(4).toBool();
        node->setVisible(kindVisible && inactiveVisible);
        const bool matches = m_filter.isEmpty() || node->data(1).toString().contains(m_filter, Qt::CaseInsensitive);
        node->setOpacity(matches ? 1 : 0.18);
    }
    for (int i = 0; i < m_edges.size(); ++i) {
        const auto &edge = m_edges[i];
        const auto *from = m_nodes.value(edge.from);
        const auto *to = m_nodes.value(edge.to);
        const bool dependencyVisible = !edge.dependency || !m_dependencyFilter || m_dependencyFilter->isChecked();
        m_paths[i]->setVisible(from->isVisible() && to->isVisible() && dependencyVisible);
        m_paths[i]->setOpacity(qMin(from->opacity(), to->opacity()));
    }
    updateAnimationState();
}

void ResourceCanvas::updateAnimationState()
{
    const bool enabled = m_motionFilter && m_motionFilter->isChecked();
    bool hasVisibleLink = false;
    for (int i = 0; i < m_edges.size(); ++i) {
        auto *path = static_cast<ResourceLinkItem *>(m_paths[i]);
        const auto &edge = m_edges[i];
        const bool endpointsActive = m_nodes.value(edge.from)->data(6).toBool()
            && m_nodes.value(edge.to)->data(6).toBool();
        path->setMotionEnabled(enabled && endpointsActive);
        hasVisibleLink = hasVisibleLink || (path->motionEnabled() && path->isVisible());
    }
    if (enabled && hasVisibleLink && isVisible() && !window()->isMinimized())
        m_animationTimer->start();
    else
        m_animationTimer->stop();
}

void ResourceCanvas::advanceAnimation()
{
    // The painter fades motion completely at overview scale; skip repaints there.
    if (m_view->transform().m11() <= 0.35)
        return;
    const qreal phase = std::fmod(m_animationClock.elapsed() / 90.0, 18.0);
    const QRectF visible = m_view->mapToScene(m_view->viewport()->rect()).boundingRect();
    for (int i = 0; i < m_edges.size(); ++i) {
        auto *path = static_cast<ResourceLinkItem *>(m_paths[i]);
        if (path->motionEnabled() && path->isVisible()
            && path->sceneBoundingRect().intersects(visible))
            path->setPhase(phase);
    }
}

void ResourceCanvas::setDetailsLabel(const QString &label)
{
    m_detailsTitle->setText(label);
    m_detailsButton->setText(label);
}

bool ResourceCanvas::focusResource(const QString &id)
{
    auto *node = m_nodes.value(id, nullptr);
    if (!node)
        return false;
    if (auto *filter = m_kindFilters.value(node->data(3).toString(), nullptr))
        filter->setChecked(true);
    if (node->data(4).toBool() && m_inactiveFilter)
        m_inactiveFilter->setChecked(true);
    m_view->scene()->clearSelection();
    node->setSelected(true);
    keepSelectionVisible(node);
    m_userAdjusted = true;
    return true;
}

void ResourceCanvas::keepSelectionVisible(QGraphicsItem *node)
{
    QRect available = m_view->viewport()->rect().adjusted(24, 24, -24, -24);
    if (m_details->isVisible()) {
        const QRect panel(m_view->viewport()->mapFrom(this, m_details->pos()), m_details->size());
        if (panel.left() > width() - panel.right())
            available.setRight(panel.left() - 24);
        else
            available.setLeft(panel.right() + 24);
    }
    // A deliberately enlarged inspector can cover the whole canvas. Do not
    // fight that placement; the panel remains movable or dismissible.
    if (available.width() < 300)
        return;
    const QRect selected = m_view->mapFromScene(node->sceneBoundingRect()).boundingRect();
    if (available.contains(selected))
        return;
    const QPointF viewportCenter = m_view->mapToScene(m_view->viewport()->rect().center());
    const QPointF target = node->sceneBoundingRect().center()
        + viewportCenter - m_view->mapToScene(available.center());
    const QRectF visibleScene = m_view->mapToScene(m_view->viewport()->rect()).boundingRect();
    const QRectF required(target.x() - visibleScene.width() / 2, target.y() - visibleScene.height() / 2,
        visibleScene.width(), visibleScene.height());
    m_view->setSceneRect(m_view->scene()->sceneRect().united(required));
    m_view->centerOn(target);
}

void ResourceCanvas::showDetails()
{
    m_details->show();
    arrange();
    m_details->raise();
}

void ResourceCanvas::fitGraph()
{
    if (m_nodes.isEmpty())
        return;
    m_view->setSceneRect(m_view->scene()->sceneRect());
    QRectF bounds;
    for (auto *node : m_nodes) {
        if (node->isVisible())
            bounds = bounds.united(node->sceneBoundingRect());
    }
    if (bounds.isEmpty())
        return;
    m_view->fitInView(bounds.adjusted(-90, -130, 90, 90), Qt::KeepAspectRatio);
    if (m_view->transform().m11() > 1)
        m_view->resetTransform();
    m_view->centerOn(bounds.center());
}

bool ResourceCanvas::eventFilter(QObject *watched, QEvent *event)
{
    if (watched == m_view->viewport()
        && (event->type() == QEvent::MouseButtonPress || event->type() == QEvent::Wheel))
        m_userAdjusted = true;
    if (watched == window() && event->type() == QEvent::WindowStateChange)
        updateAnimationState();
    return QWidget::eventFilter(watched, event);
}

void ResourceCanvas::resizeEvent(QResizeEvent *event)
{
    QWidget::resizeEvent(event);
    arrange();
}

void ResourceCanvas::showEvent(QShowEvent *event)
{
    QWidget::showEvent(event);
    window()->installEventFilter(this);
    updateAnimationState();
    arrange();
    if (!m_fitted && !m_nodes.isEmpty()) {
        fitGraph();
        m_fitted = true;
    }
}

void ResourceCanvas::hideEvent(QHideEvent *event)
{
    m_animationTimer->stop();
    QWidget::hideEvent(event);
}

void ResourceCanvas::arrange()
{
    m_view->setGeometry(rect());
    m_heading->adjustSize();
    m_heading->move(16, 16);
    m_filters->adjustSize();
    m_filters->move(qMax(16, width() - m_filters->width() - 16), 16);
    m_controls->adjustSize();
    m_controls->move(qMax(16, width() - m_controls->width() - 16), height() - m_controls->height() - 16);
    m_notice->setGeometry(16, height() - 150, qMax(200, qMin(700, width() - 32)), 80);
    m_details->placeDefault(QRect(qMax(16, width() - 760), 100, 730, qMax(300, height() - 180)));
}
