#include "SpatialGpuRenderer.h"
#include "WorkspaceFlightView.h"
#include "SpatialReturnPortal.h"
#include "FurnishingController.h"
#include "FlightAppBillboards.h"
#include "SpatialInteriorGeometry.h"
#include "FlightSolarSystem.h"
#include "FlightPlanetArt.h"
#include "FlightProjection.h"
#include "CanvasIcons.h"
#include <QPushButton>
#include <QRadialGradient>
#include <QKeyEvent>
#include <QLinearGradient>
#include <algorithm>
#include <QDir>
#include <QFileInfo>
#include <QJsonArray>
#include "WorkspaceGraphNode.h"
#include "WorkspaceGraphPage.h"
#include "SourceHighlighter.h"
#include <QTextDocument>
#include <QAbstractTextDocumentLayout>
#include <QFontDatabase>
#include <QScopeGuard>
#include <QtMath>

namespace {
using FlightProjection::nearPlane;
constexpr qreal speed = 260;
constexpr float billboardZ = -180;
constexpr float billboardTop = 275;
constexpr float billboardBottom = 95;
constexpr float billboardHalfWidth = 160;
constexpr float billboardSpacing = 400;

QRectF portalLabelRect(const QPointF &center)
{
    return QRectF(center.x() - 145, center.y() - 26, 290, 52);
}
}

WorkspaceFlightView::~WorkspaceFlightView()
{
    delete m_gpuRenderer;
    delete m_billboards;
    setFlightFullscreen(false);
}

void WorkspaceFlightView::setFlightFullscreen(bool enabled)
{
    if (m_fullscreen == enabled)
        return;
    stop();
    m_fullscreen = enabled;
    if (enabled) {
        m_fullscreenWindow = window();
        m_previousWindowState = m_fullscreenWindow->windowState();
        // Expand through existing layouts, preserving native app parentage and
        // the editor HUD. Reparenting would interrupt the X11 app handoff.
        QWidget *branch = this;
        while (branch != m_fullscreenWindow && branch->parentWidget()) {
            QWidget *parent = branch->parentWidget();
            const auto siblings = parent->findChildren<QWidget *>(QString(), Qt::FindDirectChildrenOnly);
            for (QWidget *sibling : siblings) {
                if (sibling != branch && !sibling->isWindow() && !sibling->isHidden()
                    && !sibling->property("flightOverlay").toBool()
                    && !sibling->property("flightRenderSource").toBool()) {
                    m_hiddenChrome.append(sibling);
                    sibling->hide();
                }
            }
            branch = parent;
        }
        m_fullscreenWindow->setWindowState(m_previousWindowState | Qt::WindowFullScreen);
    } else {
        for (const auto &widget : m_hiddenChrome) {
            if (widget)
                widget->show();
        }
        m_hiddenChrome.clear();
        if (m_fullscreenWindow)
            m_fullscreenWindow->setWindowState(m_previousWindowState);
        m_fullscreenWindow.clear();
    }
    m_fullscreenButton->setText(enabled ? tr("Exit fullscreen · F11") : tr("Fullscreen · F11"));
    m_fullscreenButton->adjustSize();
    m_fullscreenButton->move(qMax(8, width() - m_fullscreenButton->width() - 16), 12);
    if (!m_editorActive)
        setFocus();
}

bool WorkspaceFlightView::furnishingActive() const
{
    return m_furnishing && m_furnishing->active();
}
bool WorkspaceFlightView::furnishingButton(int button, bool pressed)
{
    return m_interior && m_furnishing && m_furnishing->button(button, pressed);
}
void WorkspaceFlightView::setCockpitWorld(const QVector<WorkspaceFlightBuilding> &buildings, bool orbit)
{
    if (m_furnishing && !m_changingInterior)
        m_furnishing->detach();
    m_travel.stop();
    m_travelDirectory.clear();
    clearHologram();
    stop();
    stopAppBillboards();
    m_reader.cancel();
    ++m_generation;
    m_root.clear();
    m_directory.clear();
    m_activeVehicle = -1;
    m_bikeRace = {};
    m_bikeStart = {};
    m_bikeDust.clear();
    m_bikeCameraPending = false;
    m_bikeLook.reset();
    m_seatReturn.reset();
    m_droneReturn.reset();
    m_vehicleModels.clear();
    m_vehicleGeometry.clear();
    if (m_gpuRenderer)
        m_gpuRenderer->release();
    m_nativeSceneActive = false;
    m_interior.reset();
    m_interiorGeometry.clear();
    m_foliageGeometry.clear();
    m_terrainGeometry.clear();
    m_terrainGeometryAge = 1;
    m_weatherTime = 0;
    m_terrainFrame = {};
    m_interiorPaintOrder.rebuild({});
    m_interiorPaintDirty = true;
    m_interiorImages.clear();
    m_interiorName.clear();
    m_cockpit = true;
    m_orbit = orbit;
    m_worldStatus.clear();
    setBuildings(buildings);
    if (orbit) {
        const double outerRadius = FlightSolarSystem::radius(qMax(6, int(m_buildings.size()) - 1));
        m_extent = qMax(12000.0, outerRadius * 3);
        for (int index = 0; index < m_buildings.size(); ++index)
            m_buildings[index].position = FlightSolarSystem::position(index, m_orbitSeconds);
        m_camera = FlightSolarSystem::sun() + QVector3D(0, outerRadius * 1.2, -outerRadius * 1.8);
        m_pitch = -0.70;
        // Prepare finite procedural assets on world entry, never during a turn.
        FlightPlanetArt::surface(0);
        FlightPlanetArt::atmosphere(0);
        FlightPlanetArt::shadow(0);
    }
    updateOrbitAnimation();
    updateFlightModeButton();
    viewport()->update();
}

void WorkspaceFlightView::setPilotInput(const QVector3D &movement, const QPointF &look, bool boost)
{
    if (bikeCrashed()) {
        // Focus/menu suspension pauses the fall as well as riding. Resume only
        // its bounded animation when control returns; never apply held throttle.
        if ((m_bike.unsettled() || m_bikeRace.active()) && isVisible() && !m_editorActive && !m_tick.isActive()) {
            m_elapsed.start();
            m_tick.start();
        }
        return;
    }
    if (m_editorActive || !isVisible() || travelling() || (m_furnishing && m_furnishing->configuring()))
        return;
    m_pilotMovement = movement;
    m_pilotLook = look;
    m_pilotBoost = boost;
    if (!movement.isNull() || !look.isNull() || (ridingBike() && (m_bikeLook.pending() || m_bikeRace.active()))) {
        if (!m_tick.isActive()) {
            m_elapsed.start();
            m_tick.start();
        }
    } else if (m_keys.isEmpty() && std::abs(m_driveSpeed) < 0.1
        && !terrainWeatherActive() && !m_dynamics.unsettled() && !bikeUnsettled() && m_pilotRoll == 0 && !m_pilotBrake) {
        m_tick.stop();
    }
}

void WorkspaceFlightView::controlApp(const DesktopAppChoice &choice, bool fullscreen)
{
    if (fullscreen)
        setFlightFullscreen(true);
    const QString path = "app:" + QString::number(choice.id);
    if (!m_appSurfaceBindings.contains(path)) {
        const bool hasBuilding = std::any_of(m_buildings.cbegin(), m_buildings.cend(),
            [&](const Building &building) { return building.source.path == path; });
        if (hasBuilding)
            previewAppWindows({choice});
        else
            setAppBillboards({choice});
    }
    m_billboards->control(m_appSurfaceBindings.value(path, 0));
}

void WorkspaceFlightView::previewApp(const QString &path, const DesktopAppChoice &choice)
{
    clearHologram();
    for (const auto &building : m_buildings) {
        if (building.source.path != path)
            continue;
        m_hologramPath = path;
        m_hologramTitle = choice.title;
        m_hologramAnchor = building.position + QVector3D(0, building.source.height * 3 + 36, 0);
        if (!m_appSurfaceBindings.contains(path))
            previewAppWindows({choice});
        m_hologramApp = m_appSurfaceBindings.value(path, 0);
        viewport()->update();
        return;
    }
}

void WorkspaceFlightView::clearHologram()
{
    m_textPreviewTimer.stop();
    m_textPreview = {};
    m_previewReader.cancel();
    ++m_previewGeneration;
    m_hologramPath.clear();
    m_hologramTitle.clear();
    m_hologramImage = {};
    m_hologramScreenRect = {};
    m_hologramApp = -1;
    m_hologramNative = false;
}

void WorkspaceFlightView::previewSurface(const QString &path, const QString &title, const QImage &image)
{
    clearHologram();
    for (const auto &building : m_buildings) {
        if (building.source.path != path)
            continue;
        m_hologramPath = path;
        m_hologramTitle = title;
        m_hologramAnchor = building.position + QVector3D(0, building.source.height * 3 + 36, 0);
        m_hologramImage = image;
        m_hologramNative = true;
        viewport()->update();
        return;
    }
}

void WorkspaceFlightView::previewCard(const QString &path, const QString &title, const QString &text)
{
    previewSurface(path, title, {});
    if (!m_hologramPath.isEmpty())
        renderFilePreview(text);
}

void WorkspaceFlightView::previewDocument(const QString &path, const QString &file)
{
    previewCard(path, QFileInfo(file).fileName(), tr("Reading workflow preview…"));
    if (!m_hologramPath.isEmpty())
        m_previewReader.request(m_previewGeneration, {{"operation", "preview"},
            {"root", QFileInfo(file).absolutePath()}, {"path", file}});
}

void WorkspaceFlightView::selectFile(const Building &building)
{
    clearHologram();
    m_hologramPath = building.source.path;
    m_hologramTitle = building.source.name;
    m_hologramAnchor = building.position + QVector3D(0, building.source.height * 3 + 36, 0);
    renderFilePreview(tr("Reading preview…"));
    if (m_root.isEmpty()) {
        renderFilePreview(tr("Snapshot file · Select again to open in the editor."));
    } else {
        m_previewReader.request(m_previewGeneration, {{"operation", "preview"},
            {"root", m_root}, {"path", m_hologramPath}});
    }
}

void WorkspaceFlightView::renderFilePreview(const QString &text, bool truncated)
{
    m_textPreviewTimer.stop();
    m_hologramImage = QImage(768, 432, QImage::Format_ARGB32_Premultiplied);
    m_hologramImage.fill(QColor("#0b1a27"));
    QTextDocument document;
    QFont font = QFontDatabase::systemFont(QFontDatabase::FixedFont);
    font.setPixelSize(16);
    document.setDefaultFont(font);
    document.setPlainText(text.left(8192));
    document.setTextWidth(736);
    SourceHighlighter highlighter(&document);
    highlighter.setFileName(m_hologramPath);
    highlighter.updateTheme(true);
    highlighter.rehighlight();
    const int documentHeight = qCeil(document.size().height());
    truncated = truncated || text.size() > 8192 || documentHeight > ScrollingPreview::maximumHeight;
    QImage content(736, qBound(376, documentHeight, ScrollingPreview::maximumHeight), QImage::Format_ARGB32_Premultiplied);
    content.fill(QColor("#0b1a27"));
    QPainter textPainter(&content);
    QAbstractTextDocumentLayout::PaintContext context;
    context.palette.setColor(QPalette::Text, QColor("#d4edf4"));
    context.clip = QRectF(0, 0, 736, content.height());
    document.documentLayout()->draw(&textPainter, context);
    textPainter.end();
    QPainter painter(&m_hologramImage);
    painter.translate(16, 12);
    painter.setPen(QColor("#69d5cb"));
    QString caption = tr("READ ONLY PREVIEW · Select again to edit");
    if (m_hologramNative)
        caption = tr("PREVIEW · Select again to open in the HUD");
    else if (truncated)
        caption = tr("EXCERPT · Select again to open the full editor");
    painter.drawText(QRect(0, 382, 736, 26), caption);
    painter.end();
    m_textPreview = ScrollingPreview(m_hologramImage, content, QRect(16, 12, 736, 376));
    m_hologramImage = m_textPreview.imageAt(0);
    m_textPreviewOffset = 0;
    m_textPreviewClock.start();
    m_textPreviewMilliseconds = 0;
    if (m_textPreview.scrolling() && isVisible())
        m_textPreviewTimer.start();
    viewport()->update();
}

void WorkspaceFlightView::setAppBillboards(const QVector<DesktopAppChoice> &choices)
{
    delete m_billboards;
    m_appSurfaceBindings.clear();
    m_billboards = new FlightAppBillboards(this, choices);
}

void WorkspaceFlightView::previewAppWindows(const QVector<DesktopAppChoice> &choices)
{
    const auto bounded = choices.mid(0, 3);
    setAppBillboards(bounded);
    for (int index = 0; index < bounded.size(); ++index)
        m_appSurfaceBindings.insert("app:" + QString::number(bounded[index].id), index);
    viewport()->update();
}

bool WorkspaceFlightView::hasAppPreview(const QString &path) const
{
    const int index = m_appSurfaceBindings.value(path, -1);
    return m_billboards && index >= 0 && index < m_billboards->frames().size()
        && !m_billboards->frames()[index].image.isNull();
}

bool WorkspaceFlightView::appControlling() const
{
    return m_billboards && m_billboards->controlling();
}

void WorkspaceFlightView::setFlightStatus(const QString &status)
{
    m_worldStatus = status;
    viewport()->update();
}

void WorkspaceFlightView::stopAppBillboards()
{
    m_appSurfaceBindings.clear();
    m_interiorStreams.clear();
    m_interiorBindings.clear();
    if (m_hologramApp >= 0)
        clearHologram();
    if (m_billboards)
        m_billboards->stopSharing();
}

void WorkspaceFlightView::setFlightHardwareAcceleration(bool enabled)
{
    if (m_billboards && m_billboards->controlling())
        m_billboards->setResumeHardware(enabled);
    else
        setHardwareAcceleration(enabled);
}

WorkspaceFlightView::WorkspaceFlightView(const QVector<WorkspaceFlightBuilding> &buildings,
                                       bool hardware, QWidget *parent)
    : WorkspaceGraphView(parent)
{
    setObjectName("workspaceDroneFlight");
    setFocusPolicy(Qt::StrongFocus);
    setDragMode(NoDrag);
    setAlignment(Qt::AlignLeft | Qt::AlignTop);
    m_fullscreenButton = new QPushButton(CanvasIcons::icon(CanvasIcons::Symbol::Fit), tr("Fullscreen · F11"), this);
    m_fullscreenButton->setObjectName("flightFullscreen");
    m_fullscreenButton->setToolTip(tr("Fill the screen with flight; restore the workspace with F11 or Escape."));
    connect(m_fullscreenButton, &QPushButton::clicked, this, [this]() {
        setFlightFullscreen(!m_fullscreen);
    });
    m_flightModeButton = new QPushButton(this);
    m_flightModeButton->setObjectName("cockpitFlightMode");
    m_flightModeButton->setFocusPolicy(Qt::NoFocus);
    connect(m_flightModeButton, &QPushButton::clicked, this, [this]() {
        setFlightMode(flightMode() == ArcadeFlightDynamics::Mode::Explorer
            ? ArcadeFlightDynamics::Mode::Pilot : ArcadeFlightDynamics::Mode::Explorer);
        setFocus();
    });
    setBuildings(buildings);
    updateFlightModeButton();
    setHardwareAcceleration(hardware);
    setSpatial(true);
    m_textPreviewTimer.setInterval(100);
    connect(&m_textPreviewTimer, &QTimer::timeout, this, [this]() {
        if (!isVisible() || m_hologramPath.isEmpty()) {
            m_textPreviewTimer.stop();
            return;
        }
        const auto delta = m_textPreviewClock.restart();
        if (m_previewAutoScroll)
            m_textPreviewMilliseconds += delta;
        const auto elapsed = m_textPreviewMilliseconds;
        const int offset = m_textPreview.offsetAt(elapsed);
        if (offset == m_textPreviewOffset)
            return;
        m_textPreviewOffset = offset;
        m_hologramImage = m_textPreview.imageAt(elapsed);
        viewport()->update();
    });
    connect(&m_previewReader, &WorkspaceGraphReader::completed, this,
        [this](quint64 generation, const QJsonObject &reply) {
            if (generation != m_previewGeneration || m_hologramPath.isEmpty())
                return;
            renderFilePreview(reply.contains("error") ? reply.value("error").toString()
                : reply.value("text").toString(), reply.value("truncated").toBool());
        });
    m_tick.setTimerType(Qt::PreciseTimer);
    m_tick.setInterval(16);
    m_travel.setTimerType(Qt::PreciseTimer);
    m_travel.setInterval(16);
    m_orbitTick.setTimerType(Qt::PreciseTimer);
    m_orbitTick.setInterval(33);
    connect(&m_orbitTick, &QTimer::timeout, this, &WorkspaceFlightView::advanceOrbit);
    connect(&m_travel, &QTimer::timeout, this, &WorkspaceFlightView::advanceTravel);
    m_lookFrame.setSingleShot(true);
    m_lookFrame.setTimerType(Qt::PreciseTimer);
    m_lookFrame.setInterval(16);
    connect(&m_lookFrame, &QTimer::timeout, this, [this]() { viewport()->update(); });
    connect(&m_tick, &QTimer::timeout, this, [this]() { advance(); });
    m_shot.setSingleShot(true);
    m_shot.setInterval(140);
    connect(&m_shot, &QTimer::timeout, this, [this]() {
        const QString path = m_shotPath;
        m_shotPath.clear();
        viewport()->update();
        if (!path.isEmpty() && !m_editorActive && fileTargeted)
            fileTargeted(path);
    });
}

void WorkspaceFlightView::setBuildings(const QVector<WorkspaceFlightBuilding> &buildings)
{
    clearHologram();
    m_attitudeActive = false;
    m_dynamics.halt();
    m_buildings.clear();
    m_planetLabels.clear();
    // The caller supplies at most the graph's 500 visible nodes.
    const int count = qMin(500, int(buildings.size()));
    const int columns = qMax(1, int(std::ceil(std::sqrt(count))));
    for (int index = 0; index < count; ++index) {
        const QVector3D position((index % columns - (columns - 1) / 2.0) * 320, 0, (index / columns) * 320);
        const auto &source = buildings[index];
        const QVector3D placed(source.position.x(), 0, source.position.y());
        m_buildings.append({source, source.positioned ? placed : position});
    }
    if (!m_orbit) {
        for (auto &building : m_buildings) {
            building.detailedMesh = createBuildingMesh(building, true);
            building.distantMesh = createBuildingMesh(building, false);
        }
    }
    m_extent = qMax(1600.0, columns * 320.0 + 640);
    for (const auto &building : m_buildings)
        m_extent = qMax(m_extent, qMax(std::abs(qreal(building.position.x())), std::abs(qreal(building.position.z()))) + 800);
    m_camera = QVector3D(0, 180, -700);
    m_yaw = 0;
    m_pitch = -0.08;
}

void WorkspaceFlightView::setWorkspace(const QString &root, const QString &directory, bool hidden)
{
    m_orbit = false;
    updateOrbitAnimation();
    m_root = QDir::cleanPath(root);
    m_hidden = hidden;
    disconnect(&m_reader, &WorkspaceGraphReader::completed, this, nullptr);
    connect(&m_reader, &WorkspaceGraphReader::completed, this,
        [this](quint64 generation, const QJsonObject &page) {
            if (generation != m_generation)
                return;
            if (page.contains("error")) {
                m_worldStatus = tr("Cannot enter: %1 · Backspace for parent, R to retry").arg(page.value("error").toString());
                viewport()->update();
                return;
            }
            QVector<WorkspaceFlightBuilding> buildings;
            const auto entries = page.value("entries").toArray();
            if (entries.size() > WorkspaceGraphPage::maximumPageNodes)
                return;
            QString after;
            for (const auto &entry : entries) {
                const auto object = entry.toObject();
                const QString name = object.value("name").toString();
                if (name.isEmpty() || name == "." || name == ".." || name.contains('/') || name.contains('\\')) {
                    m_worldStatus = tr("Cannot enter: invalid directory reply");
                    viewport()->update();
                    return;
                }
                const QString kind = object.value("kind").toString();
                const QString path = QDir(m_pendingDirectory).filePath(name);
                bool valid = false;
                const qint64 bytes = object.value("size_bytes").toString().toLongLong(&valid);
                WorkspaceGraphNode node(path, kind, valid && bytes >= 0 ? bytes : -1);
                buildings.append({name, node.chipHeight(), kind == "directory",
                    kind == "link" || kind == "unavailable", path});
                after = name;
            }
            stop();
            setBuildings(buildings);
            m_directory = m_pendingDirectory;
            m_pendingDirectory.clear();
            m_after = after;
            m_stamp = page.value("stamp").toString();
            m_more = page.value("more").toBool();
            m_worldStatus = buildings.isEmpty() ? tr("Empty directory · Backspace to return")
                : tr("%1 structures · %2").arg(buildings.size()).arg(
                    m_directory == m_root ? tr("Workspace root") : QDir(m_root).relativeFilePath(m_directory));
            if (m_more)
                m_worldStatus += tr(" · N: next district page");
            viewport()->update();
        });
    enterDirectory(directory);
}

void WorkspaceFlightView::enterDirectory(const QString &directory, bool nextPage)
{
    if (m_root.isEmpty() || m_editorActive)
        return;
    m_travel.stop();
    m_travelDirectory.clear();
    stop();
    clearHologram();
    m_shot.stop();
    m_shotPath.clear();
    m_reader.cancel();
    m_pendingDirectory = QDir::cleanPath(directory);
    m_worldStatus = tr("Opening hangar… · Esc cancels and returns to graph");
    m_reader.request(++m_generation, {{"root", m_root}, {"directory", m_pendingDirectory},
        {"after", nextPage ? m_after : QString()}, {"stamp", nextPage ? m_stamp : QString()},
        {"limit", WorkspaceGraphPage::maximumPageNodes}, {"hidden", m_hidden}});
    viewport()->update();
}

void WorkspaceFlightView::goUp()
{
    if (!m_directory.isEmpty() && m_directory != m_root)
        enterDirectory(QDir::cleanPath(QDir(m_directory).filePath("..")));
}

void WorkspaceFlightView::goBack()
{
    if (appControlling()) {
        m_billboards->releaseControl();
        return;
    }
    if (leaveInteriorAction())
        return;
    if (m_activeVehicle >= 0) {
        exitVehicle();
        return;
    }
    m_travel.stop();
    stop();
    m_shot.stop();
    m_shotPath.clear();
    clearHologram();
    if (m_cockpit && orbitRequested) {
        orbitRequested();
    } else if (!m_directory.isEmpty() && m_directory != m_root) {
        goUp();
    } else if (exitRequested) {
        exitRequested();
    }
}

bool WorkspaceFlightView::hasReturnPortal() const
{
    return !m_orbit && (m_cockpit || !m_directory.isEmpty());
}

QVector3D WorkspaceFlightView::returnPortalPosition() const
{
    if (m_interior)
        return SpatialReturnPortal::center(*m_interior);
    // The planet exit is above the skyline. Nested worlds use an entrance
    // gate in the clear arrival apron, outside every building's collision box.
    if (m_directory.isEmpty() || m_directory == m_root)
        return QVector3D(-550, 600, 1400);
    return QVector3D(-180, 160, -100);
}

qreal WorkspaceFlightView::returnPortalRadius() const
{
    if (m_interior)
        return SpatialReturnPortal::interiorRadius;
    return m_directory.isEmpty() || m_directory == m_root ? 160 : 90;
}

bool WorkspaceFlightView::targetsReturnPortal(const QVector3D &ray) const
{
    if (!hasReturnPortal() || std::abs(ray.z()) < 0.00001)
        return false;
    const QVector3D portal = returnPortalPosition();
    const QVector3D projectedPortal = cameraPoint(portal);
    const QVector3D projectedRay = cameraPoint(m_camera + ray * 100);
    // The label is a navigation beacon painted above the city. Its complete
    // visible plate is clickable, even when the distant opening is smaller.
    if (projectedPortal.z() >= nearPlane && projectedRay.z() >= nearPlane
        && portalLabelRect(screenPoint(projectedPortal)).contains(screenPoint(projectedRay)))
        return true;
    const qreal distance = (portal.z() - m_camera.z()) / ray.z();
    qreal buildingDistance = 10000;
    aimedBuilding(ray, &buildingDistance);
    if (distance <= 0 || distance >= buildingDistance)
        return false;
    const QVector3D point = m_camera + ray * distance;
    const qreal radius = returnPortalRadius();
    return std::hypot(point.x() - portal.x(), point.y() - portal.y()) <= radius;
}

void WorkspaceFlightView::paintReturnPortal(QPainter *painter)
{
    if (!hasReturnPortal())
        return;
    const bool surface = m_directory.isEmpty() || m_directory == m_root;
    const qreal radius = returnPortalRadius();
    const QVector3D center = returnPortalPosition();
    if (!sphereVisible(cameraPoint(center), radius * 1.2))
        return;
    // A fixed 32-segment gate, no texture, particles or idle animation timer.
    painter->save();
    painter->setRenderHint(QPainter::Antialiasing);
    painter->setBrush(QColor(70, 130, 240, 35));
    painter->setPen(QPen(QColor("#b9a6ff"), 2));
    QVector<QVector3D> rim;
    rim.reserve(32);
    for (int index = 0; index < 32; ++index) {
        const qreal angle = index * 2 * 3.14159265358979323846 / 32;
        rim.append(center + QVector3D(std::cos(angle) * radius, std::sin(angle) * radius, 0));
    }
    painter->drawPolygon(projectPolygon(rim));
    painter->setBrush(QColor(110, 90, 210, 95));
    for (int index = 0; index < rim.size(); ++index) {
        const QVector3D a = rim[index];
        const QVector3D b = rim[(index + 1) % rim.size()];
        painter->drawPolygon(projectPolygon({a, b, center + (b - center) * 1.12,
            center + (a - center) * 1.12}));
    }
    const QVector3D point = cameraPoint(center);
    if (point.z() >= nearPlane) {
        const QPointF label = screenPoint(point);
        const QRectF plate = portalLabelRect(label);
        painter->setBrush(QColor(12, 16, 35, 225));
        painter->drawRoundedRect(plate, 6, 6);
        painter->setPen(QColor("#ede8ff"));
        QString destination = tr("RETURN TO PARENT");
        if (m_interior)
            destination = tr("RETURN OUTSIDE");
        else if (surface)
            destination = m_cockpit ? tr("RETURN TO ORBIT") : tr("RETURN TO GRAPH");
        painter->drawText(plate, Qt::AlignCenter, destination + (m_interior ? tr("\nWalk through · or click") : tr("\nFly through · or click")));
    }
    painter->restore();
}

void WorkspaceFlightView::nextPage()
{
    if (m_more && !loading())
        enterDirectory(m_directory, true);
}

qreal WorkspaceFlightView::doorOpening(const Building &building) const
{
    if (!building.source.directory || building.source.unavailable || building.source.path.isEmpty())
        return 0;
    const qreal distance = std::hypot(m_camera.x() - building.position.x(),
                                     m_camera.z() - (building.position.z() - 48));
    return qBound(0.0, (300 - distance) / 180, 1.0);
}

void WorkspaceFlightView::setEditorActive(bool active)
{
    m_travel.stop();
    m_travelDirectory.clear();
    stop();
    m_shot.stop();
    m_shotPath.clear();
    m_editorActive = active;
    updateOrbitAnimation();
    m_fullscreenButton->setVisible(!active);
    m_fullscreenButton->raise();
    updateFlightModeButton();
    viewport()->update();
}

QVector3D WorkspaceFlightView::selectionRay(const QPointF &position) const
{
    const qreal focal = viewFocalLength();
    const qreal x = (position.x() - viewport()->width() / 2.0) / focal;
    const qreal y = (viewport()->height() / 2.0 - position.y()) / focal;
    return viewOrientation().rotatedVector(QVector3D(x, y, 1)).normalized();
}

const WorkspaceFlightView::Building *WorkspaceFlightView::aimedBuilding(qreal *distance) const
{
    return aimedBuilding(selectionRay(QPointF(viewport()->width() / 2.0, viewport()->height() / 2.0)), distance);
}

const WorkspaceFlightView::Building *WorkspaceFlightView::aimedBuilding(const QVector3D &direction, qreal *distance) const
{
    // Cast the crosshair ray through the bounded world. Include folders/links
    // in occlusion, so a shot cannot open a file behind another building.
    const Building *target = nullptr;
    qreal nearest = 10000;
    for (const auto &building : m_buildings) {
        const QVector3D minimum = building.position + QVector3D(-72, 0, -48);
        const QVector3D maximum = building.position + QVector3D(72, building.source.height * 3 + 24, 48);
        qreal enter = 0;
        qreal leave = nearest;
        bool intersects = true;
        for (int axis = 0; axis < 3; ++axis) {
            if (std::abs(direction[axis]) < 0.00001) {
                if (m_camera[axis] < minimum[axis] || m_camera[axis] > maximum[axis]) {
                    intersects = false;
                    break;
                }
                continue;
            }
            qreal first = (minimum[axis] - m_camera[axis]) / direction[axis];
            qreal last = (maximum[axis] - m_camera[axis]) / direction[axis];
            if (first > last)
                std::swap(first, last);
            enter = qMax(enter, first);
            leave = qMin(leave, last);
            if (enter > leave) {
                intersects = false;
                break;
            }
        }
        if (intersects && enter < nearest) {
            nearest = enter;
            target = &building;
        }
    }
    if (distance)
        *distance = nearest;
    return target;
}

int WorkspaceFlightView::aimedBillboard() const
{
    return aimedBillboard(selectionRay(QPointF(viewport()->width() / 2.0, viewport()->height() / 2.0)));
}

int WorkspaceFlightView::aimedBillboard(const QVector3D &direction) const
{
    if (!m_billboards || m_hologramPath.startsWith("app:"))
        return -1;
    if (std::abs(direction.z()) < 0.00001)
        return -1;
    const qreal distance = (billboardZ - m_camera.z()) / direction.z();
    qreal buildingDistance = 10000;
    aimedBuilding(direction, &buildingDistance);
    if (distance <= 0 || distance >= buildingDistance)
        return -1;
    const QVector3D point = m_camera + direction * distance;
    const int count = m_billboards->frames().size();
    for (int index = 0; index < count; ++index) {
        const qreal x = (index - (count - 1) / 2.0) * billboardSpacing;
        if (point.x() >= x - billboardHalfWidth && point.x() <= x + billboardHalfWidth
            && point.y() >= billboardBottom && point.y() <= billboardTop)
            return index;
    }
    return -1;
}

void WorkspaceFlightView::fire()
{
    if (leaveInteriorAction())
        return;
    if (furnishingActive()) {
        m_furnishing->confirm();
        return;
    }
    if (m_activeVehicle >= 0) {
        exitVehicle();
        return;
    }
    selectAt(QPointF(viewport()->width() / 2.0, viewport()->height() / 2.0));
}

void WorkspaceFlightView::selectAt(const QPointF &position)
{
    if (m_editorActive || loading() || m_shot.isActive() || travelling())
        return;
    if (m_activeVehicle >= 0)
        return;
    stop();
    const QVector3D direction = selectionRay(position);
    if (!m_hologramPath.isEmpty() && m_hologramScreenRect.contains(position)) {
        if ((m_hologramNative || m_hologramPath.startsWith("app:")) && destinationActivated) {
            destinationActivated(m_hologramPath, false);
        } else if (m_hologramApp >= 0 && m_billboards) {
            m_billboards->control(m_hologramApp);
        } else {
            m_shotPath = m_hologramPath;
            m_shot.start();
        }
        return;
    }
    if (targetsReturnPortal(direction)) {
        goBack();
        return;
    }
    if (m_interior) {
        const QString target = SpatialInteriorGeometry::target(*m_interior, m_camera, direction);
        if (target.startsWith("action:")) {
            activateInteriorAction(target.mid(7));
            return;
        }
        if (target.startsWith("vehicle:")) {
            enterVehicle(target.mid(8));
            return;
        }
        if (m_furnishing && m_furnishing->activateScreen(target))
            return;
        if (!target.isEmpty() && destinationActivated)
            destinationActivated("display:" + target, false);
        return;
    }
    const int billboard = aimedBillboard(direction);
    if (billboard >= 0) {
        if (m_hologramApp != billboard) {
            clearHologram();
            m_hologramApp = billboard;
            m_hologramTitle = m_billboards->frames()[billboard].title;
            m_hologramPath = "billboard:" + QString::number(billboard);
            m_hologramAnchor = QVector3D((billboard - (m_billboards->frames().size() - 1) / 2.0)
                * billboardSpacing, billboardTop + 36, billboardZ);
            viewport()->update();
            return;
        }
        m_billboards->control(billboard);
        return;
    }
    const auto *target = aimedBuilding(direction);
    if (m_orbit) {
        target = nullptr;
        for (const auto &planet : m_buildings) {
            if (m_planetLabels.value(planet.source.path).contains(position)) {
                approachPlanet(planet);
                return;
            }
        }
        qreal nearest = 100000;
        for (const auto &planet : m_buildings) {
            const QVector3D offset = planet.position - m_camera;
            const qreal along = QVector3D::dotProduct(offset, direction);
            if (along > 0 && along < nearest && (offset - direction * along).length() < 190) {
                target = &planet;
                nearest = along;
            }
        }
        if (target)
            approachPlanet(*target);
        return;
    }
    if (m_cockpit && m_root.isEmpty() && target && destinationActivated) {
        const QString destination = target->source.path;
        destinationActivated(destination, false);
        return;
    }
    m_shotPath.clear();
    if (!target) {
        m_worldStatus = tr("No file under the crosshair. Aim at a building and fire.");
    } else if (target->source.directory) {
        m_worldStatus = tr("Folder hangar — double-click to fly inside, or fly through its door.");
    } else if (target->source.unavailable || target->source.path.isEmpty()) {
        m_worldStatus = tr("This structure cannot be edited. Linked files stay read-only.");
    } else {
        if (m_hologramPath != target->source.path) {
            selectFile(*target);
            m_worldStatus = tr("Hologram · %1 · Select again to edit").arg(target->source.name);
            return;
        }
        m_shotPath = target->source.path;
        m_worldStatus = tr("File acquired · %1").arg(target->source.name);
    }
    m_shot.start();
    viewport()->update();
}

void WorkspaceFlightView::stop()
{
    m_bikeSound.stop();
    m_bikeThrottle = 0;
    m_bikeBrake = 0;
    m_bikeDust.clear();
    m_driveSpeed = 0;
    m_dynamics.halt();
    m_pilotRoll = 0;
    m_pilotBrake = false;
    m_lookFrame.stop();
    m_freeLooking = false;
    m_tick.stop();
    m_keys.clear();
    m_pilotMovement = {};
    m_pilotLook = {};
    m_pilotBoost = false;
    m_looking = false;
    viewport()->unsetCursor();
}

void WorkspaceFlightView::keyPressEvent(QKeyEvent *event)
{
    if (m_interior && m_furnishing && !m_editorActive && m_furnishing->key(event)) {
        event->accept();
        return;
    }
    if (m_editorActive) {
        event->accept();
        return;
    }
    if (event->key() == Qt::Key_F11) {
        if (!event->isAutoRepeat())
            setFlightFullscreen(!m_fullscreen);
        event->accept();
        return;
    }
    if (travelling() && (event->key() == Qt::Key_Space || event->key() == Qt::Key_Escape)) {
        if (event->key() == Qt::Key_Space)
            skipTravel();
        else
            m_travel.stop();
        if (event->key() == Qt::Key_Escape) {
            m_travelDirectory.clear();
            m_worldStatus = tr("Travel cancelled");
            viewport()->update();
        }
        updateOrbitAnimation();
        event->accept();
        return;
    }
    if (event->key() == Qt::Key_F && craftFlying() && !travelling()) {
        if (!event->isAutoRepeat())
            m_flightModeButton->click();
        event->accept();
        return;
    }
    const bool driving = m_interior && m_interior->locomotion == "drive";
    const bool flying = m_interior && m_interior->locomotion == "fly";
    if ((!driving && event->key() == Qt::Key_Space) || (m_interior && !flying && event->key() == Qt::Key_E)) {
        if (!event->isAutoRepeat())
            fire();
        event->accept();
        return;
    }
    if ((m_shot.isActive() || travelling()) && event->key() != Qt::Key_Escape && event->key() != Qt::Key_Backspace) {
        event->accept();
        return;
    }
    if (event->key() == Qt::Key_Escape) {
        m_travel.stop();
        stop();
        if (m_interior)
            goBack();
        else if (m_fullscreen)
            setFlightFullscreen(false);
        else if (exitRequested)
            exitRequested();
        event->accept();
        return;
    }
    if (event->key() == Qt::Key_Backspace) {
        if (!event->isAutoRepeat())
            goBack();
        event->accept();
        return;
    }
    if (terrainWeatherAvailable() && event->key() == Qt::Key_T) {
        if (!event->isAutoRepeat())
            toggleTerrainRain();
        event->accept();
        return;
    }
    if (ridingBike() && (event->key() == Qt::Key_V || event->key() == Qt::Key_M)) {
        if (!event->isAutoRepeat()) {
            if (event->key() == Qt::Key_V)
                toggleBikeCamera();
            else
                toggleBikeSound();
        }
        event->accept();
        return;
    }
    if (bikeRaceAvailable() && event->key() == Qt::Key_G) {
        if (!event->isAutoRepeat())
            startBikeRace();
        event->accept();
        return;
    }
    if (ridingBike() && event->key() == Qt::Key_R) {
        if (!event->isAutoRepeat())
            resetBike();
        event->accept();
        return;
    }
    if (event->key() == Qt::Key_N || event->key() == Qt::Key_R) {
        if (!event->isAutoRepeat()) {
            if (event->key() == Qt::Key_N)
                nextPage();
            else
                enterDirectory(m_pendingDirectory.isEmpty() ? m_directory : m_pendingDirectory);
        }
        event->accept();
        return;
    }
    if (loading()) {
        event->accept();
        return;
    }
    const QSet<int> controls{Qt::Key_W, Qt::Key_A, Qt::Key_S, Qt::Key_D, Qt::Key_Q, Qt::Key_E,
        Qt::Key_Shift, Qt::Key_Space, Qt::Key_X, Qt::Key_Z, Qt::Key_C, Qt::Key_Left, Qt::Key_Right, Qt::Key_Up, Qt::Key_Down};
    if (!controls.contains(event->key())) {
        event->ignore();
        return;
    }
    if (bikeCrashed()) {
        setPilotInput({}, {}, false);
        event->accept();
        return;
    }
    m_keys.insert(event->key());
    if (!m_tick.isActive()) {
        m_elapsed.start();
        m_tick.start();
    }
    event->accept();
}

void WorkspaceFlightView::keyReleaseEvent(QKeyEvent *event)
{
    if (!event->isAutoRepeat())
        m_keys.remove(event->key());
    if (m_keys.isEmpty() && m_pilotMovement.isNull() && m_pilotLook.isNull()
        && std::abs(m_driveSpeed) < 0.1 && !terrainWeatherActive() && !m_dynamics.unsettled() && !bikeUnsettled() && m_pilotRoll == 0 && !m_pilotBrake)
        m_tick.stop();
    event->accept();
}

bool WorkspaceFlightView::blocked(const QVector3D &position) const
{
    if (m_interior)
        return SpatialInteriorGeometry::blocked(*m_interior, position);
    if (m_orbit)
        return (position - FlightSolarSystem::sun()).lengthSquared() < 400 * 400;
    for (const auto &building : m_buildings) {
        const bool doorway = doorOpening(building) > 0.85
            && std::abs(position.x() - building.position.x()) < 46
            && position.y() >= 28 && position.y() < 210
            && position.z() < building.position.z();
        if (doorway)
            continue;
        if (std::abs(position.x() - building.position.x()) < 90
            && std::abs(position.z() - building.position.z()) < 70
            && position.y() < building.source.height * 3 + 18)
            return true;
    }
    return false;
}

void WorkspaceFlightView::advance()
{
    // Retain sub-millisecond time: truncating every tick makes constant thrust
    // alternate between different travel distances even at a steady refresh rate.
    const qreal seconds = qMin(0.05, m_elapsed.nsecsElapsed() / 1e9);
    m_elapsed.start();
    if (terrainWeatherAvailable()) {
        QElapsedTimer terrainTime;
        terrainTime.start();
        m_interior->terrain->advance(seconds);
        m_weatherTime += seconds;
        m_terrainGeometryAge += seconds;
        updateTerrainGeometry();
        m_lastTerrainUpdateNanoseconds = terrainTime.nsecsElapsed();
    }
    const qreal forward = int(m_keys.contains(Qt::Key_W)) - int(m_keys.contains(Qt::Key_S)) + m_pilotMovement.z();
    const qreal sideways = int(m_keys.contains(Qt::Key_D)) - int(m_keys.contains(Qt::Key_A)) + m_pilotMovement.x();
    const bool flying = m_interior && m_interior->locomotion == "fly";
    const bool driving = m_interior && m_interior->locomotion == "drive";
    const qreal upward = m_interior && !flying ? 0 : int(m_keys.contains(Qt::Key_E)) - int(m_keys.contains(Qt::Key_Q)) + m_pilotMovement.y();
    const qreal yawInput = int(m_keys.contains(Qt::Key_Right)) - int(m_keys.contains(Qt::Key_Left)) + m_pilotLook.x();
    const qreal pitchInput = int(m_keys.contains(Qt::Key_Up)) - int(m_keys.contains(Qt::Key_Down)) + m_pilotLook.y();
    const bool boost = m_keys.contains(Qt::Key_Shift) || m_pilotBoost;
    QVector3D delta;
    if (craftFlying()) {
        prepareCraft();
        ArcadeFlightDynamics::Input input;
        input.thrust = QVector3D(sideways, upward, forward);
        input.turn = QVector3D(pitchInput, yawInput, int(m_keys.contains(Qt::Key_C)) - int(m_keys.contains(Qt::Key_Z)) + m_pilotRoll);
        input.boost = boost;
        input.brake = m_keys.contains(Qt::Key_X) || m_pilotBrake;
        delta = m_dynamics.advance(input, seconds, m_orbit);
        const auto heading = m_dynamics.orientation().rotatedVector({0, 0, 1});
        m_yaw = std::atan2(heading.x(), heading.z());
        m_pitch = std::asin(qBound(-1.0f, heading.y(), 1.0f));
    } else {
        if (!ridingBike())
            m_yaw += yawInput * seconds;
        if (driving && m_activeVehicle < 0 && std::abs(m_driveSpeed) > 1)
            m_yaw += sideways * seconds * qBound(-1.4, m_driveSpeed / 250, 1.4);
        if (!ridingBike())
            m_pitch = qBound(-1.1, m_pitch + pitchInput * seconds, 1.1);
        m_yaw = std::remainder(m_yaw, 2 * 3.14159265358979323846);
        const qreal strafe = driving ? 0 : sideways;
        const QVector3D direction(strafe * std::cos(m_yaw) + forward * std::sin(m_yaw), 0,
            forward * std::cos(m_yaw) - strafe * std::sin(m_yaw));
        delta = direction.normalized() * (speed * (boost ? 2.5 : 1) * seconds);
    }
    if (m_seatReturn)
        delta = {};
    if (driving && !ridingBike()) {
        if (std::abs(forward) > 0.01 && !m_keys.contains(Qt::Key_Space)) {
            m_driveSpeed = qBound(-180.0, m_driveSpeed + forward * 320 * seconds, 650.0);
        } else {
            const qreal drag = (m_keys.contains(Qt::Key_Space) ? 1000 : 200) * seconds;
            m_driveSpeed = std::copysign(qMax(0.0, std::abs(m_driveSpeed) - drag), m_driveSpeed);
        }
        const qreal heading = m_activeVehicle >= 0 ? m_interior->vehicles[m_activeVehicle].heading : m_yaw;
        delta = QVector3D(std::sin(heading), 0, std::cos(heading)) * m_driveSpeed * seconds;
    }
    const QVector3D previousPosition = m_camera;
    if (ridingBike())
        advanceBike(seconds);
    else if (m_activeVehicle >= 0)
        advanceVehicle(delta, sideways, seconds);
    // Maximum boosted speed is 4,050 units/s and dt <= 50 ms: at most 26
    // samples, 8 units apart, below the interior collider's 25-unit clearance margin.
    const int samples = qBound(1, int(std::ceil(delta.length() / 8)), 32);
    const QVector3D increment = delta / samples;
    for (int sample = 0; m_activeVehicle < 0 && sample < samples; ++sample) {
        for (int axis = 0; axis < 3; ++axis) {
            QVector3D next = m_camera;
            next[axis] += increment[axis];
            const QVector3D proposed = next;
            if (!m_interior) {
                next.setX(qBound(float(-m_extent), next.x(), float(m_extent)));
                next.setY(qBound(20.0f, next.y(), m_orbit ? float(m_extent) : 2200.0f));
                next.setZ(qBound(m_orbit ? float(-m_extent) : -1200.0f, next.z(), float(m_extent)));
            } else if (flying) {
                next.setY(qBound(60.0f, next.y(), float(m_interior->height - 40)));
            }
            if (m_interior && !flying && !driving && !m_seatReturn) {
                const float height = SpatialInteriorGeometry::walkingHeight(*m_interior, next);
                if (std::abs(height - m_camera.y()) > 45)
                    continue;
                next.setY(height);
            }
            if (blocked(next)) {
                m_dynamics.blockAxis(axis);
            } else {
                m_camera = next;
                if (next[axis] != proposed[axis])
                    m_dynamics.blockAxis(axis);
            }
        }
    }
    if (craftFlying() && !m_dynamics.unsettled() && m_keys.isEmpty()
        && m_pilotMovement.isNull() && m_pilotLook.isNull() && m_pilotRoll == 0 && !m_pilotBrake)
        m_tick.stop();
    if (m_interior) {
        if (flying)
            m_camera.setY(qBound(60.0f, m_camera.y(), float(m_interior->height - 40)));
        else if (m_activeVehicle < 0 && !m_seatReturn)
            m_camera.setY(driving ? 110 : SpatialInteriorGeometry::walkingHeight(*m_interior, m_camera));
        if (m_droneReturn && m_camera.y() >= m_interior->height - 60 && orbitRequested) {
            stop();
            orbitRequested();
            return;
        }
        if (driving && delta.lengthSquared() > 0.000001 && (m_camera - previousPosition).lengthSquared() < 0.00000001)
            m_driveSpeed = 0;
        if (driving && !bikeUnsettled() && std::abs(m_driveSpeed) < 0.1 && m_keys.isEmpty() && m_pilotMovement.isNull() && m_pilotLook.isNull())
            m_tick.stop();
        const QVector3D lookDirection(std::sin(m_yaw), 0, std::cos(m_yaw));
        const QString portal = m_activeVehicle < 0 && !furnishingActive() ? SpatialInteriorGeometry::approachedPortal(*m_interior, previousPosition, m_camera, lookDirection) : QString();
        if (!portal.isEmpty()) {
            if (entranceApproached && entranceApproached(portal))
                return;
            stop();
            if (destinationActivated)
                destinationActivated("display:" + portal, false);
            return;
        }
    }
    if (m_activeVehicle < 0 && !furnishingActive() && hasReturnPortal()
        && (!m_interior || m_interior->returnPortalWalkEntry)
        && SpatialReturnPortal::entered(previousPosition, m_camera, returnPortalPosition(), returnPortalRadius())) {
        goBack();
        return;
    }
    for (const auto &building : m_buildings) {
        if (m_orbit && (m_camera - building.position).length() < 280 && destinationActivated) {
            approachPlanet(building);
            return;
        }
        if (!m_orbit && doorOpening(building) > 0.85
            && std::abs(m_camera.x() - building.position.x()) < 46
            && m_camera.y() >= 28 && m_camera.y() < 210
            && m_camera.z() > building.position.z() - 48
            && m_camera.z() < building.position.z()) {
            // Stop outside the threshold if the read fails; never retry every frame.
            m_camera.setZ(building.position.z() - 120);
            if (m_cockpit && m_root.isEmpty() && destinationActivated) {
                const QString destination = building.source.path;
                stop();
                destinationActivated(destination, true);
            } else {
                enterDirectory(building.source.path);
            }
            return;
        }
    }
    viewport()->update();
}

void WorkspaceFlightView::mousePressEvent(QMouseEvent *event)
{
    if (m_editorActive || travelling()) {
        event->accept();
        return;
    }
    setFocus();
    if (event->button() == Qt::LeftButton) {
        if (furnishingActive())
            m_furnishing->selectAt(event->position());
        else
            selectAt(event->position());
        event->accept();
        return;
    }
    if (event->button() == Qt::MiddleButton && m_cockpit) {
        m_freeLooking = true;
        m_lastMouse = event->pos();
        viewport()->setCursor(Qt::ClosedHandCursor);
    } else if (event->button() == Qt::RightButton) {
        m_looking = true;
        m_lastMouse = event->pos();
        viewport()->setCursor(Qt::ClosedHandCursor);
    }
    event->accept();
}

void WorkspaceFlightView::mouseDoubleClickEvent(QMouseEvent *event)
{
    event->accept();
    if (event->button() != Qt::LeftButton || m_editorActive || loading() || travelling()
        || m_root.isEmpty() || m_interior || m_orbit)
        return;
    const QVector3D ray = selectionRay(event->position());
    if ((!m_hologramPath.isEmpty() && m_hologramScreenRect.contains(event->position()))
        || targetsReturnPortal(ray) || aimedBillboard(ray) >= 0)
        return;
    const Building *target = aimedBuilding(ray);
    if (target && target->source.directory && !target->source.unavailable
        && !target->source.path.isEmpty())
        approachDirectory(*target);
}

void WorkspaceFlightView::requestLookFrame()
{
    // High-rate mice can deliver hundreds of events per second. Keep the latest
    // orientation immediately, but request at most one extra paint per frame.
    if (!m_tick.isActive() && !m_lookFrame.isActive())
        m_lookFrame.start();
}

void WorkspaceFlightView::mouseMoveEvent(QMouseEvent *event)
{
    if (furnishingActive())
        m_furnishing->point(event->position());
    if (!m_editorActive && !travelling() && m_freeLooking && (event->buttons() & Qt::MiddleButton)) {
        const QPoint delta = event->pos() - m_lastMouse;
        m_lastMouse = event->pos();
        m_headYaw = qBound(-2.5, m_headYaw + delta.x() * 0.004, 2.5);
        m_headPitch = qBound(-0.8, m_headPitch - delta.y() * 0.004, 0.8);
        requestLookFrame();
    }
    if (!m_editorActive && !travelling() && m_looking && (event->buttons() & Qt::RightButton)) {
        const QPoint delta = event->pos() - m_lastMouse;
        m_lastMouse = event->pos();
        if (craftFlying()) {
            prepareCraft();
            m_dynamics.steer(-delta.y() * 0.004, delta.x() * 0.004);
            if (!m_tick.isActive()) {
                m_elapsed.start();
                m_tick.start();
            }
        } else if (ridingBike() && !bikeCrashed()) {
            m_bikeLook.nudge({delta.x() * .004, -delta.y() * .004});
            if (!m_tick.isActive()) {
                m_elapsed.start();
                m_tick.start();
            }
        } else {
            m_yaw = std::remainder(m_yaw + delta.x() * 0.004, 2 * 3.14159265358979323846);
            m_pitch = qBound(-1.1, m_pitch - delta.y() * 0.004, 1.1);
            requestLookFrame();
        }
    }
    event->accept();
}
void WorkspaceFlightView::mouseReleaseEvent(QMouseEvent *event)
{
    if (event->button() == Qt::MiddleButton) {
        m_freeLooking = false;
        m_headYaw = 0;
        m_headPitch = 0;
        viewport()->unsetCursor();
        viewport()->update();
    }
    if (event->button() == Qt::RightButton) {
        m_looking = false;
        viewport()->unsetCursor();
    }
    event->accept();
}
void WorkspaceFlightView::focusOutEvent(QFocusEvent *event)
{
    if (!m_travelDirectory.isEmpty()) {
        m_travel.stop();
        m_travelDirectory.clear();
    }
    m_headYaw = 0;
    m_headPitch = 0;
    stop();
    m_shot.stop();
    m_shotPath.clear();
    WorkspaceGraphView::focusOutEvent(event);
}
void WorkspaceFlightView::hideEvent(QHideEvent *event)
{
    if (furnishingActive())
        m_furnishing->toggle();
    m_orbitTick.stop();
    m_interiorStreams.clear();
    m_interiorBindings.clear();
    m_travel.stop();
    clearHologram();
    stop();
    m_shot.stop();
    m_shotPath.clear();
    m_reader.cancel();
    ++m_generation;
    WorkspaceGraphView::hideEvent(event);
}

void WorkspaceFlightView::showEvent(QShowEvent *event)
{
    WorkspaceGraphView::showEvent(event);
    updateOrbitAnimation();
}

void WorkspaceFlightView::setOrbitMotion(bool enabled)
{
    m_orbitMotion = enabled;
    updateOrbitAnimation();
    viewport()->update();
}

void WorkspaceFlightView::updateOrbitAnimation()
{
    const bool activeConnections = m_interior && std::any_of(m_interior->connections.begin(),
        m_interior->connections.end(), [](const auto &connection) { return connection.active; });
    const bool animate = ((m_orbit && m_orbitMotion) || activeConnections)
        && isVisible() && !m_editorActive && !travelling();
    if (animate && !m_orbitTick.isActive()) {
        m_orbitElapsed.start();
        m_orbitTick.start();
    } else if (!animate) {
        m_orbitTick.stop();
    }
}

void WorkspaceFlightView::advanceOrbit()
{
    // Pause with the view; a hidden/minimized scene never catches up with a jump.
    if (!isVisible() || window()->isMinimized()) {
        m_orbitElapsed.start();
        return;
    }
    m_orbitSeconds = std::fmod(m_orbitSeconds + qMin(0.1, m_orbitElapsed.nsecsElapsed() / 1e9), 1e9);
    m_orbitElapsed.start();
    if (m_orbit && m_orbitMotion) {
        for (int index = 0; index < m_buildings.size(); ++index)
            m_buildings[index].position = FlightSolarSystem::position(index, m_orbitSeconds);
    }
    if (!m_tick.isActive())
        viewport()->update();
}
void WorkspaceFlightView::resizeEvent(QResizeEvent *event)
{
    WorkspaceGraphView::resizeEvent(event);
    if (m_furnishing)
        m_furnishing->resizeHud();
    setSceneRect(0, 0, viewport()->width(), viewport()->height());
    resetTransform();
    m_fullscreenButton->adjustSize();
    m_fullscreenButton->move(qMax(8, width() - m_fullscreenButton->width() - 16), 12);
    m_fullscreenButton->raise();
    updateFlightModeButton();
}

qreal WorkspaceFlightView::viewFocalLength() const
{
    // About 84.5 degrees vertically in rider view, at native resolution. Use
    // the same lens for native rendering, painter projection and frustum tests.
    if (ridingBike() && !bikeChaseCamera())
        return viewport()->height() * .55;
    return FlightProjection::focalLength(viewport()->size());
}

QVector3D WorkspaceFlightView::cameraPoint(const QVector3D &world) const
{
    const QVector3D relative = world - m_camera;
    return QVector3D(QVector3D::dotProduct(relative, m_viewRight),
        QVector3D::dotProduct(relative, m_viewUp), QVector3D::dotProduct(relative, m_viewForward));
}
QPointF WorkspaceFlightView::screenPoint(const QVector3D &point) const
{
    const qreal focal = viewFocalLength();
    return QPointF(viewport()->width() / 2.0 + point.x() * focal / point.z(),
                   viewport()->height() / 2.0 - point.y() * focal / point.z());
}
QPolygonF WorkspaceFlightView::projectPolygon(const QVector<QVector3D> &world) const
{
    QVector<QVector3D> points;
    points.reserve(world.size());
    for (const auto &point : world)
        points.append(cameraPoint(point));
    QPolygonF projected;
    projected.reserve(world.size() + 1);
    for (int index = 0; index < points.size(); ++index) {
        const auto a = points[index];
        const auto b = points[(index + 1) % points.size()];
        if (a.z() >= nearPlane)
            projected.append(screenPoint(a));
        if ((a.z() >= nearPlane) != (b.z() >= nearPlane)) {
            const qreal portion = (nearPlane - a.z()) / (b.z() - a.z());
            projected.append(screenPoint(a + (b - a) * portion));
        }
    }
    if (projected.size() < 3)
        return {};
    const QRectF screenBounds(viewport()->rect());
    const QRectF polygonBounds = projected.boundingRect();
    if (!screenBounds.intersects(polygonBounds))
        return {};
    if (screenBounds.contains(polygonBounds))
        return projected;
    // Clip in screen space before passing paths to Qt's raster/OpenGL engines.
    // Near-plane clipping alone can still produce coordinates above +/-32767.
    const qreal bounds[] = {0, qreal(viewport()->width()), 0, qreal(viewport()->height())};
    for (int plane = 0; plane < 4 && !projected.isEmpty(); ++plane) {
        QPolygonF clipped;
        clipped.reserve(projected.size() + 1);
        const auto coordinate = [plane](QPointF point) { return plane < 2 ? point.x() : point.y(); };
        const auto inside = [&](QPointF point) {
            return plane % 2 == 0 ? coordinate(point) >= bounds[plane] : coordinate(point) <= bounds[plane];
        };
        for (int index = 0; index < projected.size(); ++index) {
            const QPointF a = projected[index];
            const QPointF b = projected[(index + 1) % projected.size()];
            if (inside(a))
                clipped.append(a);
            if (inside(a) != inside(b)) {
                const qreal fraction = (bounds[plane] - coordinate(a)) / (coordinate(b) - coordinate(a));
                clipped.append(a + (b - a) * fraction);
            }
        }
        projected = clipped;
    }
    return projected;
}
void WorkspaceFlightView::drawWorldLine(QPainter *painter, const QVector3D &a, const QVector3D &b) const
{
    auto first = cameraPoint(a);
    auto second = cameraPoint(b);
    if (first.z() < nearPlane && second.z() < nearPlane)
        return;
    if (first.z() < nearPlane)
        first += (second - first) * ((nearPlane - first.z()) / (second.z() - first.z()));
    if (second.z() < nearPlane)
        second += (first - second) * ((nearPlane - second.z()) / (first.z() - second.z()));
    QPointF start = screenPoint(first);
    const QPointF end = screenPoint(second);
    const QPointF delta = end - start;
    qreal enter = 0;
    qreal leave = 1;
    const qreal direction[] = {-delta.x(), delta.x(), -delta.y(), delta.y()};
    const qreal distance[] = {start.x(), viewport()->width() - start.x(), start.y(), viewport()->height() - start.y()};
    for (int edge = 0; edge < 4; ++edge) {
        if (qFuzzyIsNull(direction[edge])) {
            if (distance[edge] < 0)
                return;
            continue;
        }
        const qreal fraction = distance[edge] / direction[edge];
        if (direction[edge] < 0)
            enter = qMax(enter, fraction);
        else
            leave = qMin(leave, fraction);
        if (enter > leave)
            return;
    }
    painter->drawLine(start + delta * enter, start + delta * leave);
}

QVector<WorkspaceFlightView::MeshFace> WorkspaceFlightView::createBuildingMesh(const Building &building, bool detailed) const
{
    QVector<MeshFace> mesh;
    mesh.reserve(detailed ? 60 : 5);
    const auto addFace = [&](const QVector<QVector3D> &vertices, QColor color, QColor edge, const QString &label = QString()) {
        QVector3D center;
        for (const auto &vertex : vertices)
            center += vertex;
        mesh.append({vertices, center / vertices.size(), {}, color, edge, label});
    };
    const qreal tower = building.source.height * 3;
    const auto world = [&](qreal x, qreal y, qreal z) { return building.position + QVector3D(x, y, z); };
    QColor edge(building.source.directory ? "#4fdac5" : "#7baeff");
    if (building.source.theme == "desert")
        edge = QColor("#dfb173");
    else if (building.source.theme == "forest")
        edge = QColor("#65ba8a");
    else if (building.source.theme == "ocean")
        edge = QColor("#549fe3");
    if (building.source.unavailable)
        edge = QColor("#e3b675");
    const auto box = [&](qreal x, qreal y, qreal z, qreal halfWidth, qreal height, qreal halfDepth, QColor accent, bool bevel = false) {
        QVector<QVector3D> bottom{world(x - halfWidth, y, z - halfDepth), world(x + halfWidth, y, z - halfDepth),
            world(x + halfWidth, y, z + halfDepth), world(x - halfWidth, y, z + halfDepth)};
        if (bevel) {
            const qreal cut = qMin(10.0, halfWidth / 3);
            bottom = {world(x - halfWidth + cut, y, z - halfDepth), world(x + halfWidth - cut, y, z - halfDepth),
                world(x + halfWidth, y, z - halfDepth + cut), world(x + halfWidth, y, z + halfDepth - cut),
                world(x + halfWidth - cut, y, z + halfDepth), world(x - halfWidth + cut, y, z + halfDepth),
                world(x - halfWidth, y, z + halfDepth - cut), world(x - halfWidth, y, z - halfDepth + cut)};
        }
        for (int face = 0; face <= bottom.size(); ++face) {
            QVector<QVector3D> vertices;
            if (face == bottom.size()) {
                for (const auto &point : bottom)
                    vertices.append(point + QVector3D(0, height, 0));
            } else {
                const auto a = bottom[face];
                const auto b = bottom[(face + 1) % bottom.size()];
                vertices = {a, b, b + QVector3D(0, height, 0), a + QVector3D(0, height, 0)};
            }
            addFace(vertices, accent.darker(face == bottom.size() ? 260 : 550 + face * 25), accent);
            auto &surface = mesh.last();
            surface.normal = QVector3D::crossProduct(vertices[1] - vertices[0], vertices[2] - vertices[0]).normalized();
            if (QVector3D::dotProduct(surface.normal, surface.center - world(x, y + height / 2, z)) < 0)
                surface.normal = -surface.normal;
        }
    };
    if (!detailed) {
        box(0, 0, 0, 72, tower, 48, edge);
    } else if (building.source.directory) {
        // Two buttresses and a suspended command deck frame an actual opening.
        box(-61, 0, 0, 11, tower, 48, edge);
        box(61, 0, 0, 11, tower, 48, edge);
        box(0, 218, 0, 50, qMax(20.0, tower - 218), 48, edge);
        box(0, tower, 8, 52, 24, 30, QColor("#73b8cb"));
        addFace({world(-49, 220, -49), world(49, 220, -49), world(49, 250, -49), world(-49, 250, -49)},
                QColor("#13333d"), edge, building.source.name);
        // Runway and side pylons lead directly to the folder threshold.
        for (int light = 0; light < 4; ++light) {
            const qreal z = -75 - light * 35;
            addFace({world(-44, 1, z), world(44, 1, z), world(44, 1, z - 4), world(-44, 1, z - 4)}, edge, edge);
        }
    } else {
        // Stable filename-derived silhouettes: stepped towers with roof machinery.
        unsigned style = 0;
        for (const QChar character : building.source.name)
            style = style * 31 + character.unicode();
        const qreal setback = 14 + (style % 3) * 7;
        box(0, 0, 0, 72, tower * 0.58, 48, edge, true);
        box((style % 2 ? 1 : -1) * 10, tower * 0.58, 0, 72 - setback, tower * 0.32, 36, edge, true);
        box(0, tower * 0.9, 0, 23, tower * 0.1, 23, QColor("#ca85f5"), true);
        // Rooftop aerial and emissive facade strips, fixed count per structure.
        box(12, tower, 4, 2, 24, 2, edge);
        for (int floor = 1; floor <= 4; ++floor) {
            const qreal y = tower * 0.105 * floor;
            for (int side : {-1, 1}) {
                const qreal x = side * 44;
                addFace({world(x - 14, y, -49), world(x + 14, y, -49),
                         world(x + 14, y + 4, -49), world(x - 14, y + 4, -49)},
                        QColor("#6cc9ed"), QColor("#6cc9ed"));
            }
        }
        addFace({world(-73, 8, -35), world(-73, 8, 35), world(-73, tower * 0.5, 35), world(-73, tower * 0.5, 28)},
                QColor("#d269d5"), QColor("#d269d5"));
    }
    return mesh;
}

bool WorkspaceFlightView::sphereVisible(const QVector3D &center, qreal radius) const
{
    const qreal farPlane = m_orbit ? m_extent * 3 : 10000;
    if (center.z() + radius < nearPlane || center.z() - radius > farPlane)
        return false;
    const qreal focal = viewFocalLength();
    const qreal horizontal = viewport()->width() / (2 * focal);
    const qreal vertical = viewport()->height() / (2 * focal);
    return std::abs(center.x()) <= center.z() * horizontal + radius * std::sqrt(1 + horizontal * horizontal)
        && std::abs(center.y()) <= center.z() * vertical + radius * std::sqrt(1 + vertical * vertical);
}

void WorkspaceFlightView::drawBackground(QPainter *painter, const QRectF &)
{
    QElapsedTimer drawTimer;
    drawTimer.start();
    const auto recordFrame = qScopeGuard([this, &drawTimer]() {
        m_lastDrawNanoseconds = drawTimer.nsecsElapsed();
        ++m_renderedFrames;
    });
    painter->save();
    painter->resetTransform();
    const int width = viewport()->width();
    const int height = viewport()->height();
    const QQuaternion view = viewOrientation();
    m_viewRight = view.rotatedVector({1, 0, 0});
    m_viewUp = view.rotatedVector({0, 1, 0});
    m_viewForward = view.rotatedVector({0, 0, 1});
    if (m_interior) {
        paintInterior(painter);
        painter->restore();
        return;
    }
    QLinearGradient sky(0, 0, 0, height);
    sky.setColorAt(0, QColor("#080c22"));
    sky.setColorAt(1, QColor("#243c58"));
    if (m_orbit)
        painter->fillRect(QRect(0, 0, width, height), Qt::black);
    else
        painter->fillRect(QRect(0, 0, width, height), sky);
    painter->setPen(QColor("#7393bd"));
    for (int star = 0; star < 1000; ++star) {
        const qreal longitude = star * 2.399963;
        const qreal vertical = 1 - 2 * (star + 0.5) / 1000;
        const qreal radial = std::sqrt(1 - vertical * vertical);
        const QVector3D direction(radial * std::sin(longitude), vertical, radial * std::cos(longitude));
        const QVector3D point = cameraPoint(m_camera + direction * 10000);
        if (point.z() > nearPlane) {
            const QPointF screen = screenPoint(point);
            if (m_orbit || direction.y() > 0) {
                const int brightness = 80 + star * 37 % 130;
                painter->setPen(QPen(QColor(brightness, brightness, qMin(255, brightness + 25)), star % 19 == 0 ? 1.7 : 0.8));
                painter->drawPoint(screen);
            }
        }
    }
    if (m_orbit) {
        paintPlanets(painter);
        paintCockpit(painter);
        painter->restore();
        return;
    }
    const qreal focal = viewFocalLength();
    const auto altitude = [&](const QPointF &point) {
        return m_viewRight.y() * (point.x() - width / 2.0)
            + m_viewUp.y() * (height / 2.0 - point.y()) + m_viewForward.y() * focal;
    };
    const QPolygonF corners{QPointF(0, 0), QPointF(width, 0), QPointF(width, height), QPointF(0, height)};
    QPolygonF ground;
    for (int edge = 0; edge < corners.size(); ++edge) {
        const QPointF a = corners[edge];
        const QPointF b = corners[(edge + 1) % corners.size()];
        const qreal ay = altitude(a), by = altitude(b);
        if (ay <= 0)
            ground.append(a);
        if ((ay < 0) != (by < 0))
            ground.append(a + (b - a) * (ay / (ay - by)));
    }
    painter->setPen(Qt::NoPen);
    painter->setBrush(QColor("#091820"));
    painter->drawPolygon(ground);
    painter->setPen(QPen(QColor("#254b58"), 1));
    for (int line = -28; line <= 28; ++line) {
        const float coordinate = line * 320;
        drawWorldLine(painter, QVector3D(coordinate, 0, -m_extent), QVector3D(coordinate, 0, m_extent));
        drawWorldLine(painter, QVector3D(-m_extent, 0, coordinate), QVector3D(m_extent, 0, coordinate));
    }

    struct Face {
        QPolygonF polygon;
        QColor color;
        QColor edge;
        qreal depth;
        QString label;
        int billboard = -1;
        QPolygonF textureQuad;
    };
    QVector<Face> faces;
    // Detailed geometry is limited to nearby buildings; distant ones use 5 faces.
    faces.reserve(m_buildings.size() * 12);
    QString nearby;
    qreal nearest = 100000;
    const auto addFace = [&](const QVector<QVector3D> &vertices, QColor color, QColor edge, const QString &label = QString()) {
        const QPolygonF polygon = projectPolygon(vertices);
        if (polygon.size() < 3)
            return;
        QVector3D middle;
        for (const auto &vertex : vertices)
            middle += vertex;
        faces.append({polygon, color, edge, cameraPoint(middle / vertices.size()).z(), label});
    };
    for (const auto &building : m_buildings) {
        const QVector3D center = cameraPoint(building.position);
        if (center.z() < -140 || center.z() > 10000)
            continue;
        const qreal tower = building.source.height * 3;
        const qreal radius = std::sqrt(80 * 80 + 184 * 184 + std::pow(tower / 2 + 30, 2));
        if (!sphereVisible(cameraPoint(building.position + QVector3D(0, tower / 2, 0)), radius))
            continue;
        const bool detailed = center.lengthSquared() < 1800 * 1800;
        const auto &mesh = detailed ? building.detailedMesh : building.distantMesh;
        for (const auto &surface : mesh) {
            if (!surface.normal.isNull() && QVector3D::dotProduct(surface.normal, m_camera - surface.center) <= 0)
                continue;
            const QPolygonF polygon = projectPolygon(surface.vertices);
            if (polygon.size() >= 3)
                faces.append({polygon, surface.color, surface.edge, cameraPoint(surface.center).z(), surface.label});
        }
        // Only the moving hangar shutter changes geometry during flight.
        if (detailed && building.source.directory && m_camera.z() < building.position.z()) {
            const auto world = [&](qreal x, qreal y, qreal z) { return building.position + QVector3D(x, y, z); };
            const QColor edge(building.source.unavailable ? "#e3b675" : "#4fdac5");
            const qreal opening = doorOpening(building);
            addFace({world(-49, 0, -47), world(49, 0, -47), world(49, 218, -47), world(-49, 218, -47)},
                    QColor("#020914"), QColor("#64ffe1"));
            if (opening < 0.999) {
                const qreal bottom = opening * 218;
                addFace({world(-48, bottom, -49), world(48, bottom, -49), world(48, 218, -49), world(-48, 218, -49)},
                        QColor("#163e49"), edge);
                for (int rib = 1; rib < 5; ++rib) {
                    const qreal y = bottom + (218 - bottom) * rib / 5;
                    addFace({world(-48, y, -50), world(48, y, -50), world(48, y + 2, -50), world(-48, y + 2, -50)}, edge, edge);
                }
            }
        }
        if (center.z() > nearPlane && center.z() < nearest && std::abs(screenPoint(center).x() - width / 2.0) < 100) {
            nearest = center.z();
            nearby = building.source.name;
            if (building.source.directory && !building.source.path.isEmpty())
                nearby += tr(" / HANGAR · %1% open").arg(qRound(doorOpening(building) * 100));
        }
    }
    if (m_billboards && !m_hologramPath.startsWith("app:")) {
        QSet<int> visibleStreams;
        const auto &frames = m_billboards->frames();
        for (int index = 0; index < frames.size(); ++index) {
            const float x = (index - (frames.size() - 1) / 2.0) * billboardSpacing;
            QVector<QVector3D> corners{{x - billboardHalfWidth, billboardTop, billboardZ},
                {x + billboardHalfWidth, billboardTop, billboardZ},
                {x + billboardHalfWidth, billboardBottom, billboardZ},
                {x - billboardHalfWidth, billboardBottom, billboardZ}};
            if (!m_appSurfaceBindings.isEmpty()) {
                const QString path = m_appSurfaceBindings.key(index);
                const auto building = std::find_if(m_buildings.cbegin(), m_buildings.cend(),
                    [&](const Building &candidate) { return candidate.source.path == path; });
                if (building == m_buildings.cend() || m_camera.z() >= building->position.z() - 52)
                    continue;
                corners = {building->position + QVector3D(-70, 215, -52),
                    building->position + QVector3D(70, 215, -52),
                    building->position + QVector3D(70, 136.25, -52),
                    building->position + QVector3D(-70, 136.25, -52)};
            }
            QPolygonF textureQuad;
            bool inFront = true;
            for (const auto &corner : corners) {
                const auto point = cameraPoint(corner);
                if (point.z() < nearPlane) {
                    inFront = false;
                    break;
                }
                textureQuad.append(screenPoint(point));
            }
            if (!inFront)
                continue;
            const auto polygon = projectPolygon(corners);
            if (polygon.size() < 3)
                continue;
            visibleStreams.insert(index);
            const auto &frame = frames[index];
            QString label = tr("APP LINK · %1 · select to control").arg(frame.title);
            if (!frame.controlError.isEmpty())
                label = tr("%1 · %2").arg(frame.title, frame.controlError);
            else if (!frame.status.isEmpty())
                label = tr("%1 · %2").arg(frame.title, frame.status);
            faces.append({polygon, QColor("#132530"), QColor("#ecc878"),
                cameraPoint((corners[0] + corners[2]) / 2).z(), label, index, textureQuad});
            if (m_appSurfaceBindings.isEmpty()) {
                painter->setPen(QPen(QColor("#ecc878"), 2));
                drawWorldLine(painter, {x, 0, billboardZ}, {x, billboardBottom, billboardZ});
            }
        }
        m_billboards->setVisibleStreams(visibleStreams);
    } else if (m_billboards && m_hologramApp >= 0) {
        m_billboards->setVisibleStreams({m_hologramApp});
    }
    std::stable_sort(faces.begin(), faces.end(), [](const Face &a, const Face &b) { return a.depth > b.depth; });
    for (const auto &face : faces) {
        painter->setBrush(face.color);
        painter->setPen(QPen(face.edge, 1));
        // Mesh faces and their clipped projections are convex. Avoid Qt's
        // general polygon tessellation for every face on every flight frame.
        painter->drawConvexPolygon(face.polygon);
        if (face.billboard >= 0) {
            const auto &image = m_billboards->frames()[face.billboard].image;
            const QPolygonF source{QPointF(0, 0), QPointF(image.width(), 0),
                QPointF(image.width(), image.height()), QPointF(0, image.height())};
            QTransform mapping;
            if (!image.isNull() && QTransform::quadToQuad(source, face.textureQuad, mapping)) {
                painter->save();
                QPainterPath clip;
                clip.addPolygon(face.polygon);
                painter->setClipPath(clip, Qt::IntersectClip);
                painter->setRenderHint(QPainter::SmoothPixmapTransform);
                painter->setTransform(mapping);
                painter->drawImage(QPointF(0, 0), image);
                painter->restore();
            }
        }
        QRectF sign = face.polygon.boundingRect().adjusted(3, 1, -3, -1);
        if (face.billboard >= 0)
            sign = QRectF(sign.left(), sign.bottom() + 4, sign.width(), 28);
        if (!face.label.isEmpty() && sign.width() > 36 && sign.height() > 12) {
            if (face.billboard >= 0)
                painter->fillRect(sign, QColor(10, 23, 33, 240));
            painter->setPen(QColor("#c1fff0"));
            painter->drawText(sign, Qt::AlignCenter, painter->fontMetrics().elidedText(face.label, Qt::ElideMiddle, int(sign.width())));
        }
    }

    paintReturnPortal(painter);
    if (m_cockpit) {
        paintCockpit(painter);
        paintHologram(painter);
        painter->restore();
        return;
    }
    // Static cockpit overlay; telemetry is explicitly part of the simulation.
    painter->setPen(QPen(QColor("#6ce8dd"), 1.4));
    const QPointF reticle(width / 2.0, height / 2.0);
    if (m_shot.isActive()) {
        painter->setPen(QPen(QColor("#ddfff5"), 2));
        painter->drawLine(QPointF(width * 0.26, height - 30), reticle);
        painter->drawLine(QPointF(width * 0.74, height - 30), reticle);
        painter->drawEllipse(reticle, 19, 19);
    }
    const auto *aim = aimedBuilding();
    const bool editableTarget = aim && !aim->source.directory && !aim->source.unavailable && !aim->source.path.isEmpty();
    painter->setPen(QPen(QColor(editableTarget ? "#f1d77c" : "#6ce8dd"), 1.4));
    painter->drawLine(reticle - QPointF(14, 0), reticle - QPointF(4, 0));
    painter->drawLine(reticle + QPointF(4, 0), reticle + QPointF(14, 0));
    painter->drawLine(reticle - QPointF(0, 14), reticle - QPointF(0, 4));
    painter->drawLine(reticle + QPointF(0, 4), reticle + QPointF(0, 14));
    painter->setBrush(QColor("#11222e"));
    for (int side : {-1, 1}) {
        const qreal center = width / 2.0 + side * width * 0.34;
        const QPolygonF wing{QPointF(center - 95, height), QPointF(center - 70, height - 58),
            QPointF(center + 40, height - 90), QPointF(center + 95, height)};
        painter->drawPolygon(wing);
        painter->drawEllipse(QPointF(center, height - 32), 38, 10);
    }
    painter->setPen(QColor("#cbf7ef"));
    painter->drawText(QRect(24, 20, width - 48, 26), tr("DOCKPIPE / DRONE SIM     ·     %1 loaded structures").arg(m_buildings.size()));
    painter->drawText(QRect(24, 48, width - 48, 26), inputHints());
    const bool appTarget = aimedBillboard() >= 0;
    QString targetHint;
    if (m_controllerHints) {
        targetHint = appTarget ? tr("A: preview / control app · Start: command menu")
            : tr("A: preview / edit · View: pause/resume preview · Start: command menu");
    } else {
        targetHint = appTarget ? tr("Click: preview / control app · F1: command menu")
            : tr("Click / Space: preview / edit · P: pause/resume preview · F1: command menu");
    }
    painter->drawText(QRect(24, height - 67, width - 48, 24), targetHint);
    const QString location = m_root.isEmpty() ? tr("Snapshot flight") : QDir(m_root).relativeFilePath(m_directory);
    painter->drawText(QRect(24, 78, width - 48, 24), painter->fontMetrics().elidedText(location, Qt::ElideMiddle, width - 48));
    painter->drawText(QRect(24, 104, width - 48, 24), painter->fontMetrics().elidedText(m_worldStatus, Qt::ElideRight, width - 48));
    painter->drawText(QRect(24, height - 38, width - 48, 24), tr("ALT %1  ·  %2").arg(qRound(m_camera.y())).arg(nearby));
    paintHologram(painter);
    painter->restore();
}

void WorkspaceFlightView::approachPlanet(const Building &planet)
{
    stop();
    m_travelDirectory.clear();
    m_travelDestination = planet.source.path;
    m_travelStart = m_camera;
    m_travelStartYaw = m_yaw;
    m_travelStartPitch = m_pitch;
    const QVector3D direction = (planet.position - m_camera).normalized();
    m_travelEnd = planet.position - direction * 185;
    m_travelArc = {};
    const QVector3D route = m_travelEnd - m_travelStart;
    if (!route.isNull()) {
        const float along = qBound(0.0f, QVector3D::dotProduct(FlightSolarSystem::sun() - m_travelStart, route)
            / route.lengthSquared(), 1.0f);
        if ((m_travelStart + route * along - FlightSolarSystem::sun()).length() < 750)
            m_travelArc = QVector3D(0, 2400, 0);
    }
    m_travelYaw = std::atan2(direction.x(), direction.z());
    m_travelPitch = std::asin(direction.y());
    m_descending = false;
    m_travelElapsed.start();
    m_travel.start();
    updateOrbitAnimation();
}

void WorkspaceFlightView::advanceTravel()
{
    m_attitudeActive = false;
    m_dynamics.halt();
    if (!m_travelDirectory.isEmpty()) {
        const qreal progress = qMin(1.0, m_travelElapsed.elapsed() / 2400.0);
        // Rise above the bounded page, cross to the clear apron, descend, then
        // enter the doorway. Never fly a straight line through other buildings.
        const qreal phase = progress * (m_directoryRoute.size() - 1);
        const int segment = qMin(int(phase), int(m_directoryRoute.size()) - 2);
        const qreal portion = phase - segment;
        const qreal eased = portion * portion * (3 - 2 * portion);
        m_camera = m_directoryRoute[segment] * (1 - eased) + m_directoryRoute[segment + 1] * eased;
        m_yaw = m_travelYaw;
        m_pitch = 0;
        viewport()->update();
        if (progress >= 1)
            finishDirectoryTravel();
        return;
    }
    const qreal progress = qMin(1.0, m_travelElapsed.elapsed() / 1400.0);
    const qreal eased = progress * progress * (3 - 2 * progress);
    m_camera = m_travelStart * (1 - eased) + m_travelEnd * eased + m_travelArc * (4 * eased * (1 - eased));
    const qreal turn = qMin(1.0, progress * 2.5);
    const qreal easedTurn = turn * turn * (3 - 2 * turn);
    m_yaw = m_travelStartYaw + std::remainder(m_travelYaw - m_travelStartYaw, 6.283185307179586) * easedTurn;
    m_pitch = m_travelStartPitch + (m_travelPitch - m_travelStartPitch) * easedTurn;
    viewport()->update();
    if (progress < 1)
        return;
    m_travel.stop();
    if (m_descending || !destinationActivated)
        return;
    const QString destination = m_travelDestination;
    destinationActivated(destination, false);
    // Swap only the bounded destination world at the atmosphere boundary.
    // The new city grows into view during descent; no other worlds stay loaded.
    m_descending = true;
    m_travelEnd = m_camera;
    m_travelStart = m_camera + QVector3D(0, 770, -550);
    m_travelYaw = 0;
    m_travelStartYaw = 0;
    m_travelStartPitch = -0.35;
    m_travelPitch = -0.08;
    m_travelArc = {};
    m_camera = m_travelStart;
    m_travelElapsed.start();
    m_travel.start();
}

void WorkspaceFlightView::approachDirectory(const Building &building)
{
    stop();
    m_shot.stop();
    m_shotPath.clear();
    clearHologram();
    float cruiseHeight = m_camera.y();
    for (const auto &obstacle : m_buildings)
        cruiseHeight = qMax(cruiseHeight, float(obstacle.source.height * 3 + 120));
    const QVector3D apron = building.position + QVector3D(0, 180, -220);
    m_travelEnd = building.position + QVector3D(0, 180, -120);
    m_directoryRoute = {m_camera, {m_camera.x(), cruiseHeight, m_camera.z()},
        {apron.x(), cruiseHeight, apron.z()}, apron, m_travelEnd};
    m_travelDirectory = building.source.path;
    m_travelYaw = 0;
    m_worldStatus = tr("Flying into %1 · Space: arrive now · Escape: cancel").arg(building.source.name);
    m_travelElapsed.start();
    m_travel.start();
}

bool WorkspaceFlightView::finishDirectoryTravel()
{
    if (m_travelDirectory.isEmpty())
        return false;
    m_travel.stop();
    m_camera = m_travelEnd;
    const QString directory = m_travelDirectory;
    m_travelDirectory.clear();
    enterDirectory(directory);
    return true;
}

void WorkspaceFlightView::paintHologram(QPainter *painter)
{
    m_hologramScreenRect = {};
    if (m_hologramPath.isEmpty())
        return;
    const QVector3D point = cameraPoint(m_hologramAnchor);
    if (point.z() < nearPlane || point.z() > 6000)
        return;
    // A camera-facing sprite: the anchor lives on the roof in world space,
    // while its readable texture stays upright at every viewing angle.
    const QPointF base = screenPoint(point);
    const qreal projectedWidth = viewFocalLength() * 380 / point.z();
    const qreal viewportWidth = qMin(viewport()->width() * 0.8, viewport()->height() * 0.45 * 16 / 9);
    const qreal width = qMin(760.0, qMin(projectedWidth, viewportWidth));
    if (width < 36)
        return;
    const qreal height = width * 9 / 16;
    // Preserve the world anchor, but keep a selected projection readable when
    // its roof approaches the top edge of a short window.
    const qreal top = qBound(64.0, base.y() - height - 28, qMax(64.0, viewport()->height() - height - 80));
    const QRectF panel(base.x() - width / 2, top, width, height);
    if (!viewport()->rect().intersects(panel.toAlignedRect()))
        return;
    m_hologramScreenRect = panel;
    painter->save();
    painter->setRenderHint(QPainter::Antialiasing);
    const QColor cyan("#79f1df");
    painter->setPen(QPen(QColor(121, 241, 223, 75), 1));
    painter->setBrush(QColor(73, 215, 204, 18));
    painter->drawPolygon(QPolygonF{base, panel.bottomLeft(), panel.bottomRight()});
    painter->setBrush(QColor(6, 23, 35, 225));
    painter->setPen(QPen(cyan, 1.5));
    painter->drawRoundedRect(panel.adjusted(-4, -26, 4, 22), 5, 5);
    const QImage *image = &m_hologramImage;
    QString status;
    QString controlError;
    if (m_hologramApp >= 0 && m_billboards && m_hologramApp < m_billboards->frames().size()) {
        const auto &frame = m_billboards->frames()[m_hologramApp];
        image = &frame.image;
        status = frame.status;
        controlError = frame.controlError;
    }
    if (!image->isNull()) {
        painter->setRenderHint(QPainter::SmoothPixmapTransform);
        painter->setOpacity(0.91);
        const QSizeF fitted = QSizeF(image->size()).scaled(panel.size(), Qt::KeepAspectRatio);
        painter->drawImage(QRectF(panel.center() - QPointF(fitted.width() / 2, fitted.height() / 2), fitted), *image);
        painter->setOpacity(1);
    } else {
        painter->drawText(panel, Qt::AlignCenter, status.isEmpty() ? tr("Connecting preview…") : status);
    }
    // Subtle scan lines give the projection character without an idle timer.
    painter->setPen(QColor(105, 235, 224, 20));
    for (qreal y = panel.top(); y < panel.bottom(); y += 6)
        painter->drawLine(QPointF(panel.left(), y), QPointF(panel.right(), y));
    if (!controlError.isEmpty()) {
        const QRectF notice(panel.left(), panel.bottom() - qMin(96.0, height * 0.6), width, qMin(96.0, height * 0.6));
        painter->fillRect(notice, QColor(22, 20, 30, 245));
        painter->setPen(QColor("#f3c897"));
        painter->drawText(notice.adjusted(10, 6, -10, -6), Qt::AlignVCenter | Qt::TextWordWrap, controlError);
    }
    painter->setPen(cyan);
    painter->drawText(QRectF(panel.left() + 6, panel.top() - 24, width - 12, 22),
        Qt::AlignVCenter, painter->fontMetrics().elidedText(m_hologramTitle, Qt::ElideMiddle, int(width - 12)));
    QString action = tr("SELECT AGAIN TO EDIT · PREVIEW");
    if (m_hologramNative)
        action = tr("SELECT AGAIN TO OPEN · HUD PREVIEW");
    else if (m_hologramApp >= 0)
        action = tr("SELECT AGAIN TO CONTROL · LIVE");
    painter->drawText(QRectF(panel.left() + 6, panel.bottom(), width - 12, 22), Qt::AlignVCenter,
        painter->fontMetrics().elidedText(action, Qt::ElideRight, int(width - 12)));
    painter->restore();
}

void WorkspaceFlightView::setSurfaceSpawn(const QPointF &spawn)
{
    m_camera = QVector3D(spawn.x(), 180, spawn.y());
    m_extent = qMax(m_extent, qMax(std::abs(spawn.x()), std::abs(spawn.y())) + 800);
    viewport()->update();
}

void WorkspaceFlightView::approach(const QString &path)
{
    if (m_interior || m_editorActive)
        return;
    for (const auto &building : m_buildings) {
        if (building.source.path != path)
            continue;
        if (m_orbit) {
            approachPlanet(building);
        } else {
            stop();
            m_travelDirectory.clear();
            m_travelStart = m_camera;
            m_travelArc = {};
            m_travelStartYaw = m_yaw;
            m_travelStartPitch = m_pitch;
            m_travelEnd = building.position + QVector3D(0, 180, -220);
            m_travelYaw = 0;
            m_travelPitch = 0;
            m_descending = true;
            m_travelElapsed.start();
            m_travel.start();
        }
        return;
    }
}

void WorkspaceFlightView::skipTravel()
{
    if (!m_travel.isActive())
        return;
    if (finishDirectoryTravel())
        return;
    m_travel.stop();
    m_camera = m_travelEnd;
    if (!m_descending && destinationActivated) {
        const QString destination = m_travelDestination;
        destinationActivated(destination, false);
    }
    viewport()->update();
    updateOrbitAnimation();
}
