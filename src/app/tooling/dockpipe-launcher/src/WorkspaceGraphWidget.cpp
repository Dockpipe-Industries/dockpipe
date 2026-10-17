#include "WorkspaceGraphWidget.h"
#include "WorkspaceGraphNode.h"
#include "WorkspaceGraphPage.h"
#include "WorkspaceFlightView.h"
#include "FlightAppBillboards.h"
#include "CanvasView.h"
#include "CanvasIcons.h"
#include <QDialog>
#include <QHideEvent>
#include <QButtonGroup>
#include <QDialogButtonBox>
#include <QDir>
#include <QElapsedTimer>
#include <QFormLayout>
#include <QFileInfo>
#include <QGraphicsPathItem>
#include <QHBoxLayout>
#include <QJsonArray>
#include <QLabel>
#include <QMenu>
#include <QPushButton>
#include <QSignalBlocker>
#include <QSet>
#include <QSpinBox>
#include <QTimer>
#include <QVBoxLayout>

namespace {
constexpr qint64 metadataBudget = 4 * 1024 * 1024; // UTF-16 path storage; node count also bounds scene objects.
}

WorkspaceGraphWidget::WorkspaceGraphWidget(const QString &workspaceRoot, const QString &directory,
    QWidget *parent, WorkspaceGraphLimits limits)
    : QWidget(parent), m_workspaceRoot(workspaceRoot), m_limits(limits), m_reader(this)
{
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
    for (const QString &label : {QStringLiteral("2D"), QStringLiteral("3D")}) {
        auto *mode = new QPushButton(label);
        mode->setObjectName(label == "3D" ? "graph3D" : "graph2D");
        mode->setCheckable(true);
        mode->setProperty("canvasLayerFilter", true);
        mode->setChecked(label == "2D");
        mode->setAccessibleName(tr("%1 graph view").arg(label));
        modes->addButton(mode);
        header->addWidget(mode);
        connect(mode, &QPushButton::clicked, this, [this, label]() { setSpatial(label == "3D"); });
    }
    auto *parentFolder = new QPushButton(tr("Parent"));
    parentFolder->setObjectName("graphParent");
    header->addWidget(parentFolder);
    connect(parentFolder, &QPushButton::clicked, this, [this]() {
        if (m_flight)
            m_flight->goUp();
        else if (m_directory != m_workspaceRoot)
            navigate(QDir::cleanPath(QDir(m_directory).filePath("..")));
    });
    auto *root = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Folder), tr("Root"));
    header->addWidget(root);
    connect(root, &QPushButton::clicked, this, [this]() { navigate(m_workspaceRoot); });
    auto *options = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::More), tr("Options"));
    auto *menu = new QMenu(options);
    m_flightAction = menu->addAction(tr("Drone flight · Cockpit mode"), this, &WorkspaceGraphWidget::openFlight);
    m_flightAction->setObjectName("openGraphFlight");
    m_flightAction->setVisible(false);
    auto *billboards = menu->addAction(tr("App billboards · Experimental (next flight)"));
    m_billboardsAction = billboards;
    billboards->setVisible(false);
    billboards->setObjectName("enableAppBillboards");
    billboards->setCheckable(true);
    connect(billboards, &QAction::toggled, this, [this](bool enabled) {
        m_appBillboards = enabled;
        if (!enabled && m_flight)
            m_flight->stopAppBillboards();
    });
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
    auto *accelerated = menu->addAction(tr("Hardware acceleration (3D)"));
    accelerated->setCheckable(true);
    accelerated->setChecked(!qEnvironmentVariableIsSet("DOCKPIPE_GRAPH_SOFTWARE"));
    connect(accelerated, &QAction::toggled, this, [this](bool enabled) {
        if (m_flight) {
            m_flightHardware = enabled;
            m_flight->setFlightHardwareAcceleration(enabled);
        } else {
            m_view->setHardwareAcceleration(enabled);
        }
    });
    auto *hidden = menu->addAction(tr("Show dotfiles"));
    hidden->setCheckable(true);
    hidden->setChecked(m_showHidden);
    connect(hidden, &QAction::toggled, this, [this](bool show) { m_showHidden = show; navigate(m_directory); });
    menu->addAction(tr("Node limits…"), this, &WorkspaceGraphWidget::configureLimits);
    menu->addAction(CanvasIcons::icon(CanvasIcons::Symbol::Fit), tr("Reset layout"), this, [this]() {
        for (auto &node : m_nodes) {
            node.flatPosition = QPointF(node.depth * 290, node.row * 76);
            if (!spatial())
                node.item->setPos(node.flatPosition);
        }
        m_view->resetCamera();
        m_view->resetTransform();
        m_view->centerOn(m_nodes.value(m_directory).item);
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
    auto *hint = new QLabel(tr("Chip height = file size (log scale) · Middle/Shift-drag to orbit · Drag to pan · Scroll to zoom"));
    hint->setObjectName("editorMuted");
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
    auto *rotate = new QPushButton(tr("Rotate"));
    rotate->setObjectName("graphRotate");
    connect(rotate, &QPushButton::clicked, this, [this]() { m_view->orbit(30, 0); });
    camera->addWidget(rotate);
    auto *reset = new QPushButton(tr("Reset camera"));
    connect(reset, &QPushButton::clicked, this, [this]() { m_view->resetCamera(); });
    camera->addWidget(reset);
    layout->addWidget(m_cameraControls);
    m_cameraControls->hide();
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
    m_flightControls = new QWidget;
    auto *flightActions = new QHBoxLayout(m_flightControls);
    flightActions->setContentsMargins(0, 0, 0, 0);
    flightActions->addWidget(new QLabel(tr("Directory worlds · fly through teal hangars")), 1);
    auto *parentWorld = new QPushButton(tr("Parent · Backspace"));
    connect(parentWorld, &QPushButton::clicked, this, [this]() {
        if (m_flight)
            m_flight->goUp();
    });
    flightActions->addWidget(parentWorld);
    auto *nextWorld = new QPushButton(tr("Next page · N"));
    connect(nextWorld, &QPushButton::clicked, this, [this]() {
        if (m_flight)
            m_flight->nextPage();
    });
    flightActions->addWidget(nextWorld);
    auto *exitFlight = new QPushButton(tr("Exit flight · Esc"));
    connect(exitFlight, &QPushButton::clicked, this, &WorkspaceGraphWidget::closeFlight);
    flightActions->addWidget(exitFlight);
    layout->addWidget(m_flightControls);
    m_flightControls->hide();
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
    emit flightClosing();
    m_transition.stop();
    m_droneMotion.stop();
    if (m_flight) {
        m_flight->exitRequested = {};
        m_flight->fileTargeted = {};
        m_flight->rendererChanged = {};
    }
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
    closeFlight();
    if (spatial() == enabled)
        return;
    if (enabled) {
        for (auto &node : m_nodes)
            node.flatPosition = node.item->pos();
    }
    m_view->setSpatial(enabled);
    m_cameraControls->setVisible(enabled);
    findChild<QPushButton *>(enabled ? "graph3D" : "graph2D")->setChecked(true);
    projectNodes();
    if (m_nodes.size() <= 30) {
        m_view->fitInView(m_view->scene()->itemsBoundingRect().adjusted(-60, -60, 60, 60), Qt::KeepAspectRatio);
        const qreal zoom = m_view->transform().m11();
        const qreal bounded = qBound(0.65, zoom, 1.0);
        m_view->scale(bounded / zoom, bounded / zoom);
    } else if (m_nodes.contains(selection())) {
        m_view->centerOn(m_nodes.value(selection()).item);
    }
    for (auto it = m_nodes.cbegin(); it != m_nodes.cend(); ++it)
        updateSummary(it.key());
    animateSelection();
}

void WorkspaceGraphWidget::applyCockpitSettings(const CockpitSettings &settings)
{
    m_cockpitSettings = settings;
    m_flightAction->setVisible(settings.enabled);
    m_billboardsAction->setVisible(settings.enabled && settings.appControl);
    if (!settings.enabled) {
        closeFlight();
        return;
    }
    if (!m_flight)
        return;
    if (!settings.appControl)
        m_flight->stopAppBillboards();
    m_flight->setShipVisible(settings.showShip);
    m_flight->setFlightHardwareAcceleration(settings.hardwareAcceleration);
}

void WorkspaceGraphWidget::openFlight()
{
    if (!m_cockpitSettings.enabled || m_flight)
        return;
    QVector<DesktopAppChoice> apps;
    if (m_cockpitSettings.appControl && m_appBillboards) {
        apps = FlightAppBillboards::chooseWindows(this);
        if (apps.isEmpty())
            return;
    }
    setSpatial(true);
    m_droneMotion.stop();
    m_reader.cancel();
    ++m_token;
    m_loadingPath.clear();
    m_flightHardware = m_view->hardwareAccelerationRequested();
    m_view->hide();
    m_view->setHardwareAcceleration(false);
    m_flight = new WorkspaceFlightView({}, m_cockpitSettings.hardwareAcceleration, this);
    m_flight->setShipVisible(m_cockpitSettings.showShip);
    if (!apps.isEmpty())
        m_flight->setAppBillboards(apps);
    m_flight->exitRequested = [this]() { closeFlight(); };
    m_flight->fileTargeted = [this](const QString &path) { emit flightFileRequested(path); };
    m_flight->rendererChanged = [this]() { emit flightRendererChanged(); };
    auto *stack = qobject_cast<QVBoxLayout *>(layout());
    stack->insertWidget(stack->indexOf(m_view), m_flight, 1);
    m_cameraControls->hide();
    m_graphActions->hide();
    m_flightControls->show();
    m_status->setText(tr("Teal hangars open as you approach. Fly through to enter. Backspace returns to the parent directory; Escape returns to your graph."));
    m_flight->show();
    m_flight->setFocus();
    m_flight->setWorkspace(m_workspaceRoot, m_directory, m_showHidden);
}

void WorkspaceGraphWidget::closeFlight()
{
    if (!m_flight)
        return;
    emit flightClosing();
    m_flight->setFlightFullscreen(false);
    m_flight->hide();
    layout()->removeWidget(m_flight);
    m_flight->deleteLater();
    m_flight = nullptr;
    m_flightControls->hide();
    m_graphActions->show();
    m_view->show();
    m_view->setHardwareAcceleration(m_flightHardware);
    m_cameraControls->setVisible(spatial());
    m_status->setText(tr("Returned to the workspace graph. Your loaded branches and layout are preserved."));
    m_view->setFocus();
    updateActions();
}

void WorkspaceGraphWidget::projectNodes()
{
    // Project the already-visible set only. Camera changes never touch disk.
    m_projecting = true;
    qreal extent = 900;
    for (auto &node : m_nodes) {
        node.item->setFlag(QGraphicsItem::ItemIsMovable, !spatial());
        node.item->setCursor(spatial() ? Qt::PointingHandCursor : Qt::OpenHandCursor);
        node.item->setCityView(spatial() ? m_view : nullptr);
        if (spatial()) {
            // Five columns per level, stable slots as pages arrive. Deeper
            // folders rise above the floor; labels always face the camera.
            const qreal x = node.parent.isEmpty() ? 0 : ((node.row % 5) - 2) * 350 + node.depth * 80;
            const qreal z = node.parent.isEmpty() ? -800 : (node.row / 5) * 320;
            const QVector3D position(x, node.depth * 260, z);
            node.cityPosition = position;
            node.item->setPos(m_view->project(position) - QPointF(110, 29));
            node.item->setZValue(1 + m_view->depth(position));
            extent = qMax(extent, z + 300);
        } else {
            node.item->setPos(node.flatPosition);
            node.item->setZValue(0);
        }
    }
    m_projecting = false;
    for (auto it = m_nodes.cbegin(); it != m_nodes.cend(); ++it)
        updateLinks(it.key());
    m_view->setGridExtent(extent);
    m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
    m_view->viewport()->update();
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
    closeFlight();
    m_reader.cancel();
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
    if (spatial())
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
    node.item->moved = [this, path]() { updateLinks(path); };
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
    if (spatial())
        projectNodes();
    m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
    if (firstRootPage && m_nodes.size() <= 30) {
        m_view->fitInView(m_view->scene()->itemsBoundingRect().adjusted(-40, -40, 40, 40), Qt::KeepAspectRatio);
        const qreal zoom = m_view->transform().m11();
        const qreal bounded = qBound(0.5, zoom, 1.0);
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
    if (spatial())
        projectNodes();
    m_view->setSceneRect(m_view->scene()->itemsBoundingRect().adjusted(-150, -150, 300, 200));
    m_status->setText(tr("Branch collapsed. Its nodes and metadata have been released."));
    updateActions();
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
        const QPointF start = m_nodes[node.parent].item->pos() + QPointF(spatial() ? 110 : 220, 29);
        const QPointF end = node.item->pos() + QPointF(spatial() ? 110 : 0, 29);
        QPainterPath curve(start);
        if (spatial()) {
            const auto origin = m_nodes[node.parent].cityPosition;
            const auto destination = node.cityPosition;
            curve.lineTo(m_view->project(QVector3D(destination.x(), origin.y(), origin.z())));
            curve.lineTo(m_view->project(QVector3D(destination.x(), origin.y(), destination.z())));
            curve.lineTo(end);
        } else {
            curve.cubicTo(start + QPointF(40, 0), end - QPointF(40, 0), end);
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
