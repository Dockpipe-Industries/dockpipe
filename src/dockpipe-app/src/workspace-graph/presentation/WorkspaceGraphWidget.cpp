#include "WorkspaceGraphWidget.h"
#include "WorkspaceGraphNode.h"
#include "WorkspaceGraphLayout.h"
#include "WorkspaceGraphPage.h"
#include "CanvasView.h"
#include "CanvasIcons.h"
#include <QDialog>
#include <QHideEvent>
#include <QButtonGroup>
#include <QDialogButtonBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFormLayout>
#include <QFutureWatcher>
#include <QFileInfo>
#include <QGraphicsPathItem>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QMenu>
#include <QPushButton>
#include <QPointer>
#include <QSignalBlocker>
#include <QSet>
#include <QSpinBox>
#include <QTimer>
#include <QVBoxLayout>
#include <QtConcurrent>

namespace {
constexpr qint64 metadataBudget = 4 * 1024 * 1024; // UTF-16 path storage; node count also bounds scene objects.

// The widget is part of the installed SDK. Keep its asynchronous implementation
// state in an owned child so separately compiled consumers retain its layout.
class GraphLayoutState final : public QObject {
public:
    explicit GraphLayoutState(QObject *parent) : QObject(parent)
    {
        setObjectName("workspaceGraphLayoutState");
    }
    QFutureWatcher<QVector<QVector3D>> watcher;
    QVector<WorkspaceGraphLayoutNode> pendingInputs;
    std::function<void()> completed;
    quint64 generation = 0;
    bool active = false;
    bool pending = false;
};

GraphLayoutState &graphLayoutState(WorkspaceGraphWidget *widget)
{
    return *static_cast<GraphLayoutState *>(widget->findChild<QObject *>(
        "workspaceGraphLayoutState", Qt::FindDirectChildrenOnly));
}
}

WorkspaceGraphWidget::WorkspaceGraphWidget(const QString &workspaceRoot, const QString &directory,
    QWidget *parent, WorkspaceGraphLimits limits)
    : QWidget(parent), m_workspaceRoot(workspaceRoot), m_limits(limits), m_reader(this)
{
    new GraphLayoutState(this);
    m_limits.pageNodes = qBound(1, m_limits.pageNodes, WorkspaceGraphPage::maximumPageNodes);
    m_limits.visibleNodes = qBound(2, m_limits.visibleNodes, WorkspaceGraphPage::maximumVisibleNodes);
    setObjectName("workspaceGraph");
    auto *layout = new QVBoxLayout(this);
    layout->setContentsMargins(0, 0, 0, 0);
    layout->setSpacing(10);
    auto *header = new QHBoxLayout;
    auto *back = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Code), tr("Editor"));
    back->setObjectName("closeWorkspaceGraph");
    header->addWidget(back);
    connect(back, &QPushButton::clicked, this, &WorkspaceGraphWidget::closeRequested);
    auto *title = new QLabel(tr("Workspace graph"));
    title->setObjectName("editorEmptyTitle");
    header->addWidget(title);
    header->addStretch();
    auto *modes = new QButtonGroup(this);
    for (const bool spatial : {false, true}) {
        const QString label = spatial ? tr("Explorer") : tr("Graph");
        auto *mode = new QPushButton(label);
        mode->setObjectName(spatial ? "graph3D" : "graph2D");
        mode->setCheckable(true);
        mode->setProperty("canvasLayerFilter", true);
        mode->setChecked(!spatial);
        mode->setAccessibleName(tr("%1 graph view").arg(label));
        mode->setToolTip(spatial ? tr("Explore folders and files in a spatial scene")
                                : tr("Explore a spatial graph of connected folders and files"));
        modes->addButton(mode);
        header->addWidget(mode);
        connect(mode, &QPushButton::clicked, this, [this, spatial]() { setSpatial(spatial); });
    }
    auto *parentFolder = new QPushButton(tr("Parent"));
    parentFolder->setObjectName("graphParent");
    header->addWidget(parentFolder);
    connect(parentFolder, &QPushButton::clicked, this, [this]() {
        if (m_directory != m_workspaceRoot)
            navigate(QDir::cleanPath(QDir(m_directory).filePath("..")));
    });
    auto *root = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Folder), tr("Root"));
    header->addWidget(root);
    connect(root, &QPushButton::clicked, this, [this]() { navigate(m_workspaceRoot); });
    auto *options = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::More), tr("Options"));
    auto *menu = new QMenu(options);
    auto *animations = menu->addAction(tr("Animate drone navigation"));
    animations->setCheckable(true);
    animations->setChecked(true);
    connect(animations, &QAction::toggled, this, [this](bool enabled) {
        m_animateNavigation = enabled;
        cancelTransition();
        m_droneMotion.stop();
        animateSelection();
    });
    menu->addSeparator();
    auto *accelerated = menu->addAction(tr("Hardware acceleration (Explorer)"));
    accelerated->setCheckable(true);
    accelerated->setChecked(!qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_SOFTWARE"));
    connect(accelerated, &QAction::toggled, this, [this](bool enabled) {
        m_view->setHardwareAcceleration(enabled);
    });
    auto *hidden = menu->addAction(tr("Show dotfiles"));
    hidden->setCheckable(true);
    hidden->setChecked(m_showHidden);
    connect(hidden, &QAction::toggled, this, [this](bool show) { m_showHidden = show; navigate(m_directory); });
    menu->addAction(tr("Node limits…"), this, &WorkspaceGraphWidget::configureLimits);
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Fit), tr("Reset layout"), this, [this]() {
        const auto resetCamera = [this] {
            m_view->resetCamera();
            m_view->resetTransform();
            m_view->centerOn(m_nodes.value(m_directory).item);
        };
        if (!spatial()) {
            for (auto &node : m_nodes)
                node.item->setProperty("graphPosition", QVariant());
            arrangeGraph(resetCamera);
        } else {
            resetCamera();
        }
    });
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Refresh), tr("Refresh view"), this, [this]() { navigate(m_directory); });
    options->setMenu(menu);
    header->addWidget(options);
    layout->addLayout(header);
    m_location = new QLabel;
    m_location->setObjectName("editorMuted");
    m_location->setTextFormat(Qt::PlainText);
    m_location->setSizePolicy(QSizePolicy::Ignored, QSizePolicy::Preferred);
    layout->addWidget(m_location);
    m_view = new WorkspaceGraphView;
    m_view->setProperty("workspaceGraphConstellation", true);
    m_view->orbit(0, -25);
    m_view->cameraChanged = [this]() {
        cancelTransition();
        projectNodes();
    };
    m_view->foregroundPainter = [this](QPainter *painter) {
        paintDrone(painter);
        paintSelectionLabel(painter);
    };
    m_view->setObjectName("workspaceGraphCanvas");
    m_view->scene()->setItemIndexMethod(QGraphicsScene::BspTreeIndex);
    layout->addWidget(m_view, 1);
    m_cameraControls = new QWidget;
    auto *camera = new QHBoxLayout(m_cameraControls);
    camera->setContentsMargins(0, 0, 0, 0);
    auto *hint = new QLabel(tr("Drag nodes to arrange · Middle/Shift-drag to orbit · Drag background to pan · Scroll to zoom"));
    hint->setObjectName("graphCameraHint");
    hint->setWordWrap(true);
    camera->addWidget(hint, 1);
    auto *renderer = new QLabel;
    renderer->setObjectName("graphRenderer");
    renderer->setTextFormat(Qt::PlainText);
    camera->addWidget(renderer);
    m_view->rendererChanged = [this, renderer]() {
        renderer->setText(m_view->hardwareAccelerated() ? tr("GPU") : tr("Software"));
        renderer->setToolTip(m_view->rendererDescription());
    };
    m_view->rendererChanged();
    renderer->hide();
    auto *rotate = new QPushButton(tr("Rotate"));
    rotate->setObjectName("graphRotate");
    connect(rotate, &QPushButton::clicked, this, [this]() { m_view->orbit(30, 0); });
    camera->addWidget(rotate);
    auto *reset = new QPushButton(tr("Reset camera"));
    connect(reset, &QPushButton::clicked, this, [this]() { m_view->resetCamera(); });
    camera->addWidget(reset);
    layout->addWidget(m_cameraControls);
    m_graphActions = new QWidget;
    auto *actions = new QHBoxLayout(m_graphActions);
    actions->setContentsMargins(0, 0, 0, 0);
    m_expand = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Add), tr("Expand"));
    m_expand->setObjectName("graphExpand");
    connect(m_expand, &QPushButton::clicked, this, [this]() {
        const QString path = selection();
        if (m_nodes.value(path).expanded)
            collapse(path);
        else
            expand(path);
    });
    actions->addWidget(m_expand);
    m_more = new QPushButton(tr("Load more"));
    m_more->setObjectName("graphLoadMore");
    connect(m_more, &QPushButton::clicked, this, [this]() { loadMore(selection()); });
    actions->addWidget(m_more);
    m_open = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Code), tr("Open file"));
    connect(m_open, &QPushButton::clicked, this, [this]() { activatePath(selection()); });
    actions->addWidget(m_open);
    auto *focus = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Search), tr("Focus"));
    connect(focus, &QPushButton::clicked, this, [this]() {
        if (m_nodes.contains(selection())) {
            m_view->resetTransform();
            m_view->centerOn(m_nodes.value(selection()).item);
        }
    });
    actions->addWidget(focus);
    auto *fit = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Fit), tr("Fit"));
    connect(fit, &QPushButton::clicked, this, [this]() {
        m_view->fitInView(m_view->scene()->itemsBoundingRect().adjusted(-40, -40, 40, 40), Qt::KeepAspectRatio);
    });
    actions->addWidget(fit);
    auto *cancel = new QPushButton(tr("Cancel read"));
    cancel->setObjectName("graphCancelRead");
    connect(cancel, &QPushButton::clicked, this, [this]() {
        m_reader.cancel();
        ++m_token;
        m_loadingPath.clear();
        m_status->setText(tr("Read cancelled. Expand the folder again to retry."));
        updateActions();
    });
    actions->addWidget(cancel);
    actions->addStretch();
    m_count = new QLabel;
    m_count->setObjectName("editorMuted");
    actions->addWidget(m_count);
    layout->addWidget(m_graphActions);
    m_status = new QLabel;
    m_status->setWordWrap(true);
    m_status->setTextFormat(Qt::PlainText);
    m_status->setObjectName("editorMuted");
    layout->addWidget(m_status);
    connect(m_view->scene(), &QGraphicsScene::selectionChanged, this, &WorkspaceGraphWidget::updateActions);
    connect(&m_reader, &WorkspaceGraphReader::completed, this, &WorkspaceGraphWidget::applyPage);
    connect(m_view->scene(), &QGraphicsScene::selectionChanged, this, &WorkspaceGraphWidget::animateSelection);
    m_droneMotion.setDuration(360);
    m_droneMotion.setEasingCurve(QEasingCurve::InOutCubic);
    connect(&m_droneMotion, &QVariantAnimation::valueChanged, this, [this](const QVariant &value) {
        m_dronePosition = value.toPointF();
        m_view->viewport()->update();
    });
    m_transition.setDuration(650);
    m_transition.setStartValue(0.0);
    m_transition.setEndValue(1.0);
    m_transition.setEasingCurve(QEasingCurve::InOutCubic);
    connect(&m_transition, &QVariantAnimation::valueChanged, this, [this](const QVariant &value) {
        m_transitionProgress = value.toReal();
        if (!m_nodes.contains(m_transitionPath))
            return;
        auto *item = m_nodes[m_transitionPath].item;
        const QPointF hover = item->pos() + item->roofCenter() + QPointF(0, -32);
        const qreal approach = qMin(1.0, m_transitionProgress / 0.6);
        const qreal arrival = qMax(0.0, (m_transitionProgress - 0.6) / 0.4);
        m_dronePosition = m_transitionStart + (hover - m_transitionStart) * approach;
        if (m_nodes[m_transitionPath].kind == "directory") {
            item->setDoorOpening(qMin(1.0, m_transitionProgress * 2));
            const QPointF target = item->pos() + item->hangarCenter();
            QTransform transform = m_beforeTransition;
            const qreal zoom = 1 + m_transitionProgress * 2;
            transform.scale(zoom, zoom);
            m_view->setTransform(transform);
            m_view->centerOn(m_beforeCenter + (target - m_beforeCenter) * m_transitionProgress);
            m_dronePosition += (target - hover) * arrival;
        } else {
            m_dronePosition += QPointF(0, 32 * arrival);
        }
        m_view->viewport()->update();
    });
    connect(&m_transition, &QVariantAnimation::finished, this, [this]() {
        const QString path = m_transitionPath;
        const bool directory = m_nodes.value(path).kind == "directory";
        cancelTransition();
        if (directory)
            navigate(path);
        else
            emit openFileRequested(path);
    });
    navigate(directory);
}

WorkspaceGraphWidget::~WorkspaceGraphWidget()
{
    m_transition.stop();
    m_droneMotion.stop();
    m_view->cameraChanged = {};
    m_view->rendererChanged = {};
    m_view->foregroundPainter = {};
    disconnect(m_view->scene(), nullptr, this, nullptr);
    m_reader.cancel();
    // Remove item callbacks before member destruction and scene teardown.
    for (auto &node : m_nodes) {
        node.item->moved = {};
        node.item->activated = {};
        node.item->menuRequested = {};
    }
}

void WorkspaceGraphWidget::cancelTransition()
{
    m_transition.stop();
    if (!m_transitionPath.isEmpty()) {
        if (m_nodes.contains(m_transitionPath))
            m_nodes[m_transitionPath].item->setDoorOpening(0);
        m_view->setTransform(m_beforeTransition);
        m_view->centerOn(m_beforeCenter);
    }
    m_transitionPath.clear();
    m_transitionProgress = 0;
}

void WorkspaceGraphWidget::hideEvent(QHideEvent *event)
{
    cancelTransition();
    m_droneMotion.stop();
    QWidget::hideEvent(event);
}

void WorkspaceGraphWidget::animateSelection()
{
    cancelTransition();
    if (!spatial() || !m_nodes.contains(selection())) {
        m_droneMotion.stop();
        return;
    }
    auto *item = m_nodes[selection()].item;
    const QPointF target = item->pos() + item->roofCenter() + QPointF(0, -32);
    m_droneMotion.stop();
    if (m_dronePath.isEmpty() || !m_animateNavigation)
        m_dronePosition = target;
    m_dronePath = selection();
    m_droneMotion.setStartValue(m_dronePosition);
    m_droneMotion.setEndValue(target);
    if (m_animateNavigation)
        m_droneMotion.start();
    m_view->viewport()->update();
}

void WorkspaceGraphWidget::activatePath(const QString &path)
{
    if (!m_nodes.contains(path))
        return;
    selectPath(path);
    const QString kind = m_nodes[path].kind;
    if (kind != "directory" && kind != "file")
        return;
    if (spatial() && m_animateNavigation) {
        cancelTransition();
        m_droneMotion.stop();
        m_transitionPath = path;
        m_transitionStart = m_dronePosition;
        m_beforeTransition = m_view->transform();
        m_beforeCenter = m_view->mapToScene(m_view->viewport()->rect().center());
        m_transition.start();
    } else if (kind == "file") {
        emit openFileRequested(path);
    } else if (spatial()) {
        navigate(path);
    } else if (m_nodes[path].expanded) {
        collapse(path);
    } else {
        expand(path);
    }
}

void WorkspaceGraphWidget::paintDrone(QPainter *painter)
{
    if (!spatial() || !m_nodes.contains(m_dronePath))
        return;
    if (m_transitionPath.isEmpty() && m_droneMotion.state() != QAbstractAnimation::Running)
        m_dronePosition = m_nodes[m_dronePath].item->pos() + m_nodes[m_dronePath].item->roofCenter() + QPointF(0, -32);
    painter->save();
    painter->resetTransform();
    const QPointF center = m_view->mapFromScene(m_dronePosition);
    painter->translate(center);
    painter->setPen(QPen(QColor("#7bf8e0"), 1.5));
    painter->setBrush(QColor("#102936"));
    // Four ducted fans and swept wings; no idle rotor repaint loop.
    painter->drawPolygon(QPolygonF{QPointF(-22, 4), QPointF(-9, -7), QPointF(0, -4),
        QPointF(9, -7), QPointF(22, 4), QPointF(0, 10)});
    for (int side : {-1, 1}) {
        painter->drawEllipse(QPointF(side * 18, -4), 8, 4);
        painter->drawEllipse(QPointF(side * 13, 9), 6, 3);
    }
    painter->setBrush(QColor("#d5fff6"));
    painter->drawEllipse(QPointF(0, 0), 4, 2);
    painter->restore();
}

void WorkspaceGraphWidget::paintSelectionLabel(QPainter *painter)
{
    if (!spatial())
        return;
    auto *item = m_nodes.value(selection()).item;
    if (!item)
        return;
    const QPoint anchor = m_view->mapFromScene(item->mapToScene(item->cityLabelRect().topLeft()));
    const qreal x = qBound(8.0, qreal(anchor.x()), qMax(8.0, m_view->viewport()->width() - 228.0));
    const qreal y = qBound(8.0, qreal(anchor.y()), qMax(8.0, m_view->viewport()->height() - 52.0));
    // Foreground painting keeps only the selected label above all geometry.
    // Screen-space size keeps it readable even at city-wide zoom.
    painter->save();
    painter->resetTransform();
    item->paintPriorityLabel(painter, QRectF(x, y, 220, 44));
    painter->restore();
}

void WorkspaceGraphWidget::setSpatial(bool enabled)
{
    cancelTransition();
    m_droneMotion.stop();
    if (spatial() == enabled)
        return;
    m_view->setProperty(spatial() ? "explorerCameraAngles" : "graphCameraAngles", m_view->cameraAngles());
    m_view->setProperty("workspaceGraphConstellation", !enabled);
    m_view->setSpatial(enabled);
    const QString cameraKey = enabled ? "explorerCameraAngles" : "graphCameraAngles";
    const QPointF angles = m_view->property(cameraKey.toUtf8().constData()).isValid()
        ? m_view->property(cameraKey.toUtf8().constData()).toPointF() : QPointF(-20, enabled ? 50 : 25);
    const QPointF current = m_view->cameraAngles();
    m_view->orbit(angles.x() - current.x(), angles.y() - current.y());
    findChild<QLabel *>("graphCameraHint")->setText(enabled
        ? tr("Chip height = file size · Middle/Shift-drag to orbit · Drag background to pan · Scroll to zoom")
        : tr("Drag nodes to arrange · Middle/Shift-drag to orbit · Drag background to pan · Scroll to zoom"));
    findChild<QLabel *>("graphRenderer")->setVisible(enabled);
    findChild<QPushButton *>(enabled ? "graph3D" : "graph2D")->setChecked(true);
    invalidateLayout();
    const auto finishProjection = [this, enabled] {
        if (!enabled || m_nodes.size() <= 30) {
            m_view->fitInView(m_view->scene()->itemsBoundingRect().adjusted(-60, -60, 60, 60), Qt::KeepAspectRatio);
            const qreal zoom = m_view->transform().m11();
            const qreal bounded = qBound(enabled ? 0.65 : 0.2, zoom, 1.0);
            m_view->scale(bounded / zoom, bounded / zoom);
        } else if (m_nodes.contains(selection())) {
            m_view->centerOn(m_nodes.value(selection()).item);
        }
        for (auto it = m_nodes.cbegin(); it != m_nodes.cend(); ++it)
            updateSummary(it.key());
        animateSelection();
    };
    if (!enabled) {
        bool needsLayout = false;
        for (const auto &node : m_nodes)
            needsLayout = needsLayout || !node.item->property("graphPosition").isValid();
        if (needsLayout) {
            arrangeGraph(finishProjection);
            return;
        }
        projectGraph();
    } else {
        projectNodes();
    }
    finishProjection();
}

void WorkspaceGraphWidget::projectNodes()
{
    if (!spatial()) {
        projectGraph();
        return;
    }
    // Project the already-visible set only. Camera changes never touch disk.
    m_projecting = true;
    qreal extent = 900;
    for (auto &node : m_nodes) {
        node.item->setGraphView(false);
        node.item->setScale(1);
        node.item->setOpacity(1);
        node.item->setFlag(QGraphicsItem::ItemIsMovable, false);
        node.item->setCursor(Qt::PointingHandCursor);
        node.item->setCityView(m_view);
        // Five columns per level, stable slots as pages arrive. Deeper
        // folders rise above the floor; labels always face the camera.
        const qreal x = node.parent.isEmpty() ? 0 : ((node.row % 5) - 2) * 350 + node.depth * 80;
        const qreal z = node.parent.isEmpty() ? -800 : (node.row / 5) * 320;
        const QVector3D position(x, node.depth * 260, z);
        node.cityPosition = position;
        node.item->setPos(m_view->project(position) - QPointF(110, 29));
        node.item->setZValue(1 + m_view->depth(position));
        extent = qMax(extent, z + 300);
    }
    m_projecting = false;
    for (auto it = m_nodes.cbegin(); it != m_nodes.cend(); ++it)
        updateLinks(it.key());
    m_view->setGridExtent(extent);
    m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
    m_view->viewport()->update();
}

void WorkspaceGraphWidget::invalidateLayout()
{
    auto &state = graphLayoutState(this);
    ++state.generation;
    state.pending = false;
    state.pendingInputs.clear();
    state.completed = {};
    setProperty("workspaceGraphLayoutBusy", state.active);
}

void WorkspaceGraphWidget::arrangeGraph(std::function<void()> completed)
{
    auto &state = graphLayoutState(this);
    invalidateLayout();
    QMap<QString, int> indexes;
    for (auto it = m_nodes.cbegin(); it != m_nodes.cend(); ++it) {
        const auto &node = it.value();
        const int parent = node.parent.isEmpty() ? -1 : indexes.value(node.parent, -1);
        indexes.insert(it.key(), state.pendingInputs.size());
        const QVariant position = node.item->property("graphPosition");
        state.pendingInputs.append({it.key(), parent, position.value<QVector3D>(), position.isValid()});
    }
    state.completed = std::move(completed);
    state.pending = true;
    setProperty("workspaceGraphLayoutBusy", true);
    startPendingLayout();
}

void WorkspaceGraphWidget::startPendingLayout()
{
    auto &state = graphLayoutState(this);
    if (state.active || !state.pending)
        return;
    const auto inputs = std::move(state.pendingInputs);
    const quint64 generation = state.generation;
    state.pending = false;
    state.active = true;
    disconnect(&state.watcher, nullptr, this, nullptr);
    connect(&state.watcher, &QFutureWatcher<QVector<QVector3D>>::finished, this,
        [this, inputs, generation] {
        auto &state = graphLayoutState(this);
        const auto positions = state.watcher.result();
        const QPointer<WorkspaceGraphWidget> guard(this);
        state.active = false;
        setProperty("workspaceGraphLayoutBusy", state.pending);
        if (generation == state.generation && !spatial()) {
            auto completed = std::move(state.completed);
            if (positions.size() != inputs.size()) {
                m_status->setText(tr("The graph could not be arranged. Refresh to retry."));
            } else {
                for (qsizetype index = 0; index < inputs.size(); ++index)
                    m_nodes[inputs.at(index).id].item->setProperty("graphPosition", QVariant::fromValue(positions.at(index)));
                projectGraph();
                if (completed)
                    completed();
            }
        }
        if (guard)
            startPendingLayout();
    });
    // The solver owns only bounded value data (500 nodes, 56 steps). Closing the
    // view drops its callback without waiting for or exposing UI to the worker.
    state.watcher.setFuture(QtConcurrent::run([inputs] {
        return WorkspaceGraphLayout::arrange(inputs);
    }));
}

void WorkspaceGraphWidget::projectGraph()
{
    m_projecting = true;
    for (auto &node : m_nodes) {
        node.item->setGraphView(true);
        node.item->setFlag(QGraphicsItem::ItemIsMovable, true);
        node.item->setCursor(Qt::OpenHandCursor);
        const QVector3D position = node.item->property("graphPosition").value<QVector3D>();
        const qreal depth = m_view->depth(position);
        const qreal scale = qBound(0.72, 1.0 + depth / 2200.0, 1.25);
        node.item->setScale(scale);
        node.item->setPos(m_view->project(position) - QPointF(110, 29) * scale);
        node.item->setZValue(depth);
        node.item->setProperty("graphDepthOpacity", qBound(0.65, 0.88 + depth / 3000.0, 1.0));
        node.item->setProperty("graphRadius", qBound(5.0, 5.0 + std::sqrt(node.children.size()) * 1.2, 16.0));
    }
    m_projecting = false;
    updateGraphEmphasis();
    m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 150, 150));
    m_view->viewport()->update();
}

void WorkspaceGraphWidget::moveGraphNode(const QString &path)
{
    if (m_projecting || !m_nodes.contains(path))
        return;
    auto &node = m_nodes[path];
    if (!spatial()) {
        QVector3D position = node.item->property("graphPosition").value<QVector3D>();
        const QPointF projected = m_view->project(position) - QPointF(110, 29) * node.item->scale();
        position += m_view->unprojectDelta(node.item->pos() - projected);
        node.item->setProperty("graphPosition", QVariant::fromValue(position));
        auto &layout = graphLayoutState(this);
        if (layout.active || layout.pending) {
            auto completed = std::move(layout.completed);
            arrangeGraph(std::move(completed));
        }
    }
    updateLinks(path);
}

void WorkspaceGraphWidget::updateGraphEmphasis()
{
    if (spatial())
        return;
    const QString selected = selection();
    QSet<QString> neighbors;
    if (m_nodes.contains(selected)) {
        neighbors.insert(selected);
        neighbors.insert(m_nodes[selected].parent);
        for (const auto &child : m_nodes[selected].children)
            neighbors.insert(child);
    }
    for (auto it = m_nodes.begin(); it != m_nodes.end(); ++it) {
        const bool related = selected.isEmpty() || neighbors.contains(it.key());
        const bool labelVisible = related && (m_nodes.size() <= 18 || it.key() == selected
            || it->kind == "directory" || (!selected.isEmpty() && neighbors.size() <= 12));
        it->item->setProperty("graphLabelVisible", labelVisible);
        it->item->setOpacity(it->item->property("graphDepthOpacity").toDouble() * (related ? 1.0 : 0.23));
        it->item->update();
        updateLinks(it.key());
    }
}

QString WorkspaceGraphWidget::selection() const
{
    const auto selected = m_view->scene()->selectedItems();
    return selected.isEmpty() ? QString() : selected.first()->data(0).toString();
}
QString WorkspaceGraphWidget::statusText() const { return m_status->text(); }
bool WorkspaceGraphWidget::hasMore(const QString &path) const { return m_nodes.value(path).more; }
void WorkspaceGraphWidget::selectPath(const QString &path)
{
    if (!m_nodes.contains(path))
        return;
    m_view->scene()->clearSelection();
    m_nodes[path].item->setSelected(true);
}

void WorkspaceGraphWidget::navigate(const QString &directory)
{
    cancelTransition();
    m_droneMotion.stop();
    m_dronePath.clear();
    m_reader.cancel();
    invalidateLayout();
    ++m_token;
    m_loadingPath.clear();
    const QSignalBlocker blocker(m_view->scene());
    m_view->scene()->clear();
    m_nodes.clear();
    m_metadataBytes = 0;
    m_directory = directory;
    const QString relative = QDir(m_workspaceRoot).relativeFilePath(directory);
    m_location->setText(relative == "." ? tr("Workspace · %1").arg(QFileInfo(m_workspaceRoot).fileName())
        : tr("Workspace / %1").arg(relative));
    m_location->setToolTip(directory);
    addNode(directory, "directory", {});
    projectNodes();
    selectPath(directory);
    m_view->resetTransform();
    m_view->setSceneRect(-100, -100, 900, 700);
    m_view->centerOn(280, 180);
    readPage(directory);
}

void WorkspaceGraphWidget::expand(const QString &path)
{
    if (!m_nodes.contains(path) || m_nodes[path].kind != "directory" || m_nodes[path].expanded)
        return;
    readPage(path);
}
void WorkspaceGraphWidget::loadMore(const QString &path)
{
    if (m_nodes.contains(path) && m_nodes[path].more)
        readPage(path);
}

void WorkspaceGraphWidget::readPage(const QString &path)
{
    if (!m_nodes.contains(path))
        return;
    const int available = m_limits.visibleNodes - m_nodes.size();
    if (available <= 0 || m_metadataBytes >= metadataBudget) {
        m_status->setText(tr("Graph limit reached. Collapse a branch or use ‘Explore this folder’ to narrow the view."));
        return;
    }
    m_loadingPath = path;
    m_status->setText(tr("Loading immediate children… You can cancel or keep navigating."));
    updateActions();
    const auto &node = m_nodes[path];
    m_reader.request(++m_token, {{"root", m_workspaceRoot}, {"directory", path}, {"after", node.after},
        {"stamp", node.stamp}, {"limit", qMin(available, m_limits.pageNodes)}, {"hidden", m_showHidden}});
}

void WorkspaceGraphWidget::addNode(const QString &path, const QString &kind, const QString &parent, qint64 fileBytes)
{
    Node node;
    node.parent = parent;
    node.kind = kind;
    node.depth = parent.isEmpty() ? 0 : m_nodes.value(parent).depth + 1;
    node.item = new WorkspaceGraphNode(path, kind, fileBytes);
    node.item->setGraphView(!spatial());
    m_view->scene()->addItem(node.item);
    QSet<int> usedRows;
    for (const auto &existing : m_nodes) {
        if (existing.depth == node.depth)
            usedRows.insert(existing.row);
    }
    node.row = parent.isEmpty() ? 0 : m_nodes.value(parent).row;
    while (usedRows.contains(node.row))
        ++node.row;
    node.item->setPos(node.depth * 290, node.row * 76);
    node.flatPosition = node.item->pos();
    if (!parent.isEmpty()) {
        node.edge = m_view->scene()->addPath(QPainterPath(), QPen(QColor("#607a96"), 1));
        node.edge->setZValue(-1000000);
        m_nodes[parent].children.append(path);
    }
    m_nodes.insert(path, node);
    m_metadataBytes += path.size() * sizeof(QChar);
    node.item->activated = [this, path]() { activatePath(path); };
    node.item->menuRequested = [this, path](const QPoint &position) { showMenu(path, position); };
    node.item->moved = [this, path]() { moveGraphNode(path); };
    updateLinks(path);
    updateSummary(path);
}

void WorkspaceGraphWidget::applyPage(quint64 token, const QJsonObject &page)
{
    if (token != m_token || !m_nodes.contains(m_loadingPath))
        return;
    const QString path = m_loadingPath;
    m_loadingPath.clear();
    if (page.contains("error")) {
        m_status->setText(page.value("error").toString());
        updateActions();
        return;
    }
    QElapsedTimer layout;
    layout.start();
    auto &node = m_nodes[path];
    const bool firstRootPage = path == m_directory && !node.expanded;
    bool limited = false;
    const auto entries = page.value("entries").toArray();
    for (const auto &entry : entries) {
        const auto object = entry.toObject();
        const QString name = object.value("name").toString();
        const QString child = QDir(path).filePath(name);
        // Recheck the helper protocol before using names as paths or scene keys.
        if (name.isEmpty() || name == "." || name == ".." || name.contains('/')) {
            m_status->setText(tr("Directory reader returned an invalid filename."));
            updateActions();
            return;
        }
        if (m_nodes.size() >= m_limits.visibleNodes || m_metadataBytes + child.size() * sizeof(QChar) > metadataBudget) {
            limited = true;
            break;
        }
        if (!m_nodes.contains(child)) {
            bool validSize = false;
            const qint64 fileBytes = object.value("size_bytes").toString().toLongLong(&validSize);
            addNode(child, object.value("kind").toString(), path, validSize && fileBytes >= 0 ? fileBytes : -1);
        }
        node.after = name;
    }
    node.stamp = page.value("stamp").toString();
    node.more = limited || page.value("more").toBool();
    node.expanded = true;
    updateSummary(path);
    const auto finishPage = [this, path, firstRootPage, limited, layout] {
        const auto &node = m_nodes[path];
        m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
        if (firstRootPage && (!spatial() || m_nodes.size() <= 30)) {
            m_view->fitInView(m_view->scene()->itemsBoundingRect().adjusted(-40, -40, 40, 40), Qt::KeepAspectRatio);
            const qreal zoom = m_view->transform().m11();
            const qreal bounded = qBound(spatial() ? 0.5 : 0.2, zoom, 1.0);
            m_view->scale(bounded / zoom, bounded / zoom);
        } else if (selection() == path && !node.children.isEmpty()) {
            const auto *child = m_nodes.value(node.children.first()).item;
            m_view->ensureVisible(node.item->sceneBoundingRect().united(child->sceneBoundingRect()), 24, 24);
        }
        m_layoutMicroseconds = layout.nsecsElapsed() / 1000;
        if (limited || (node.more && m_nodes.size() >= m_limits.visibleNodes)) {
            m_status->setText(tr("More entries exist. Graph limit reached; collapse a branch or explore this folder on its own."));
        } else if (node.more) {
            m_status->setText(tr("%1 children shown in this folder. More entries exist — select the folder and choose Load more.").arg(node.children.size()));
        } else {
            m_status->setText(node.children.isEmpty() ? tr("This folder is empty with the current filters.")
                : tr("%1 immediate children loaded. Double-click a folder to expand, or a file to edit.").arg(node.children.size()));
        }
        updateActions();
        emit pageApplied();
    };
    if (spatial()) {
        projectNodes();
        finishPage();
    } else {
        arrangeGraph(finishPage);
    }
}

void WorkspaceGraphWidget::collapse(const QString &path)
{
    cancelTransition();
    if (!m_nodes.contains(path))
        return;
    QStringList pending = m_nodes[path].children;
    const QSignalBlocker blocker(m_view->scene());
    while (!pending.isEmpty()) {
        const QString child = pending.takeLast();
        const Node node = m_nodes.take(child);
        pending.append(node.children);
        if (m_loadingPath == child || m_loadingPath == path) {
            m_reader.cancel();
            ++m_token;
            m_loadingPath.clear();
        }
        m_metadataBytes -= child.size() * sizeof(QChar);
        delete node.edge;
        delete node.item;
    }
    if (m_loadingPath == path) {
        m_reader.cancel();
        ++m_token;
        m_loadingPath.clear();
    }
    auto &node = m_nodes[path];
    node.children.clear();
    node.after.clear();
    node.stamp.clear();
    node.expanded = false;
    node.more = false;
    updateSummary(path);
    selectPath(path);
    const auto finishCollapse = [this] {
        m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
        m_status->setText(tr("Branch collapsed. Its nodes and metadata have been released."));
        updateActions();
    };
    if (spatial()) {
        projectNodes();
        finishCollapse();
    } else {
        arrangeGraph(finishCollapse);
    }
}

void WorkspaceGraphWidget::updateSummary(const QString &path)
{
    auto &node = m_nodes[path];
    QString text;
    if (node.kind == "directory") {
        text = node.expanded ? tr("%1 children%2").arg(node.children.size()).arg(node.more ? tr(" · more available") : QString()) : (spatial() ? tr("Hangar · double-click to enter") : tr("Folder · double-click to expand"));
    } else if (node.kind == "link") {
        text = tr("Symbolic link · not followed");
    } else if (node.kind == "unavailable") {
        text = tr("Unavailable or special file");
    } else {
        text = tr("File · double-click to edit");
    }
    node.item->setSummary(text);
}

void WorkspaceGraphWidget::updateLinks(const QString &path)
{
    if (m_projecting || !m_nodes.contains(path))
        return;
    QStringList links = m_nodes[path].children;
    links.append(path);
    for (const QString &child : links) {
        const auto &node = m_nodes[child];
        if (!node.edge || !m_nodes.contains(node.parent))
            continue;
        const QPointF start = m_nodes[node.parent].item->mapToScene(QPointF(110, 29));
        const QPointF end = node.item->mapToScene(QPointF(110, 29));
        QPainterPath curve(start);
        if (spatial()) {
            node.edge->setPen(QPen(QColor("#607a96"), 1));
            node.edge->setOpacity(1);
            const auto origin = m_nodes[node.parent].cityPosition;
            const auto destination = node.cityPosition;
            curve.lineTo(m_view->project(QVector3D(destination.x(), origin.y(), origin.z())));
            curve.lineTo(m_view->project(QVector3D(destination.x(), origin.y(), destination.z())));
            curve.lineTo(end);
        } else {
            curve.lineTo(end);
            const bool dark = m_view->palette().color(QPalette::Window).lightness() < 128;
            const QString selected = selection();
            const bool highlighted = selected == child || selected == node.parent;
            QColor color(dark ? "#40556e" : "#9bafc7");
            if (highlighted)
                color = QColor(dark ? "#80bcb6" : "#4f8e91");
            color.setAlpha(highlighted ? 185 : 95);
            QPen pen(color, highlighted ? 1.15 : 0.8);
            pen.setCosmetic(true);
            node.edge->setPen(pen);
            node.edge->setOpacity(selected.isEmpty() || highlighted ? 1.0 : 0.2);
        }
        node.edge->setPath(curve);
    }
}

void WorkspaceGraphWidget::updateActions()
{
    const auto node = m_nodes.value(selection());
    m_expand->setEnabled(node.kind == "directory");
    m_expand->setText(node.expanded ? tr("Collapse") : tr("Expand"));
    m_expand->setIcon(CanvasIcons::icon(node.expanded ? CanvasIcons::Symbol::Minus : CanvasIcons::Symbol::Add));
    m_more->setEnabled(node.more && m_loadingPath.isEmpty());
    m_more->setVisible(node.more);
    m_open->setEnabled(node.kind == "file");
    findChild<QPushButton *>("graphCancelRead")->setVisible(!m_loadingPath.isEmpty());
    m_count->setText(tr("%1 / %2 nodes").arg(m_nodes.size()).arg(m_limits.visibleNodes));
    updateGraphEmphasis();
    m_view->viewport()->update();
}

void WorkspaceGraphWidget::showMenu(const QString &path, const QPoint &position)
{
    selectPath(path);
    auto *menu = new QMenu(this);
    menu->setAttribute(Qt::WA_DeleteOnClose);
    const auto node = m_nodes.value(path);
    if (node.kind == "directory") {
        menu->addAction(node.expanded ? tr("Collapse") : tr("Expand"), this, [this, path, node]() {
            if (node.expanded)
                collapse(path);
            else
                expand(path);
        });
        if (node.more)
            menu->addAction(tr("Load more"), this, [this, path]() { loadMore(path); });
        menu->addAction(tr("Explore this folder"), this, [this, path]() {
            if (spatial())
                activatePath(path);
            else
                navigate(path);
        });
        menu->addAction(tr("Refresh branch"), this, [this, path]() { collapse(path); expand(path); });
    } else if (node.kind == "file") {
        menu->addAction(tr("Open in editor"), this, [this, path]() { activatePath(path); });
    }
    if (node.kind == "file" || node.kind == "directory")
        menu->addAction(tr("Show in explorer"), this, [this, path]() { emit revealRequested(path); });
    menu->popup(position);
}

void WorkspaceGraphWidget::configureLimits()
{
    QDialog dialog(this);
    dialog.setWindowTitle(tr("Graph limits"));
    auto *layout = new QFormLayout(&dialog);
    auto *page = new QSpinBox;
    page->setRange(1, WorkspaceGraphPage::maximumPageNodes);
    page->setValue(m_limits.pageNodes);
    auto *visible = new QSpinBox;
    visible->setRange(2, WorkspaceGraphPage::maximumVisibleNodes);
    visible->setValue(m_limits.visibleNodes);
    layout->addRow(tr("Nodes per expansion"), page);
    layout->addRow(tr("Visible nodes"), visible);
    auto *note = new QLabel(tr("Collapsed branches are released. Applying limits resets this view."));
    note->setWordWrap(true);
    layout->addRow(note);
    auto *buttons = new QDialogButtonBox(QDialogButtonBox::Apply | QDialogButtonBox::Cancel);
    connect(buttons->button(QDialogButtonBox::Apply), &QPushButton::clicked, &dialog, &QDialog::accept);
    connect(buttons, &QDialogButtonBox::rejected, &dialog, &QDialog::reject);
    layout->addRow(buttons);
    if (dialog.exec() != QDialog::Accepted)
        return;
    m_limits = {page->value(), visible->value()};
    navigate(m_directory);
}

