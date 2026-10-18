#include "WorkspaceFlightView.h"
#include "SpatialReturnPortal.h"
#include "FurnishingController.h"
#include "SpatialWebPassage.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialFoliagePlacement.h"
#include "FlightAppBillboards.h"
#include "FlightProjection.h"
#include <QPainter>
#include <QPainterPath>
#include <QPaintEngine>
#include <algorithm>
#include <cmath>

void WorkspaceFlightView::setInterior(const SpatialInterior &room, const QString &name, bool preserveLook, bool preservePosition, const QString &environmentKey)
{
    const auto previousPosition = m_camera;
    const auto previousYaw = m_yaw;
    const auto previousPitch = m_pitch;
    m_changingInterior = true;
    setCockpitWorld({}, false);
    m_changingInterior = false;
    m_interior = room;
    if (room.terrain)
        m_interior->terrain = std::make_shared<SpatialTerrain>(*room.terrain);
    m_interior->foliage.clear();
    SpatialFoliagePlacement foliagePlacement(room);
    const int foliageCount = qMin(int(room.foliage.size()), SpatialInterior::maximumFoliage);
    for (int index = 0; index < foliageCount; ++index) {
        auto plant = room.foliage[index];
        if (m_interior->terrain) {
            if (!std::isfinite(plant.base.y()))
                continue;
            plant.base.setY(0);
        }
        if (!foliagePlacement.admit(plant))
            continue;
        if (m_interior->terrain)
            plant.base.setY(m_interior->terrain->sample(plant.base).height);
        m_interior->foliage.append(plant);
    }
    m_interior->actions.resize(qMin(room.actions.size(), qsizetype(SpatialInterior::maximumActions)));
    m_interior->walkSurfaces.resize(qMin(room.walkSurfaces.size(), qsizetype(16)));
    m_interior->rideSurfaces.clear();
    for (const auto &surface : room.rideSurfaces) {
        if (m_interior->rideSurfaces.size() == SpatialRideSurface::maximumSurfaces)
            break;
        if (surface.valid())
            m_interior->rideSurfaces.append(surface);
    }
    if (!environmentKey.isEmpty())
        m_interior->environmentKey = environmentKey;
    m_interior->connections.resize(qMin(room.connections.size(), qsizetype(SpatialInterior::maximumConnections)));
    m_interiorName = name;
    m_interior->vehicles.clear();
    for (const auto &source : room.vehicles) {
        if (m_interior->vehicles.size() >= SpatialVehicleModel::maximumVehicles)
            break;
        auto vehicle = source;
        // Authored vehicle models retain their local ground-plane contract.
        // A terrain scene projects the admitted runtime pose onto its surface.
        if (m_interior->terrain) {
            if (!std::isfinite(vehicle.position.y()))
                continue;
            vehicle.position.setY(0);
        }
        if (!SpatialVehicleModel::valid(vehicle))
            continue;
        const auto model = SpatialVehicleModel::build(vehicle);
        if (model.size() > SpatialVehicleModel::maximumFaces)
            continue;
        if (m_interior->terrain)
            vehicle.position.setY(m_interior->terrain->sample(vehicle.position).height);
        if (SpatialVehicleModel::blocked(*m_interior, vehicle))
            continue;
        m_interior->vehicles.append(vehicle);
        m_vehicleModels.append(model);
        m_vehicleGeometry.append(QVector<MeshFace>{});
        updateVehicleGeometry(m_interior->vehicles.size() - 1);
    }
    // Cache geometry and its spatial ordering; frames only traverse, cull and project.
    m_interior->solids.resize(qMin(room.solids.size(), qsizetype(room.streaming ? SpatialWebPassage::solidBudget : SpatialInterior::maximumSolids)));
    m_interior->displays.resize(qMin(room.displays.size(), qsizetype(SpatialWorkspace::maximumDisplays)));
    const auto appendFace = [this](const QVector<QVector3D> &vertices, const QColor &color, const QString &displayId = {}) {
        MeshFace mesh;
        mesh.vertices = vertices;
        for (const auto &vertex : vertices)
            mesh.center += vertex;
        mesh.center /= vertices.size();
        mesh.color = color;
        mesh.displayId = displayId;
        m_interiorGeometry.append(mesh);
    };
    for (const auto &solid : m_interior->solids) {
        const qsizetype firstFace = m_interiorGeometry.size();
        const auto low = solid.center - solid.size / 2;
        const auto high = solid.center + solid.size / 2;
        const float a = low.x(), b = high.x();
        const float c = low.y(), d = high.y();
        const float e = low.z(), f = high.z();
        if (solid.shape == SpatialSolidShape::Box) {
            appendFace({{a,c,e}, {b,c,e}, {b,d,e}, {a,d,e}}, solid.color);
            m_interiorGeometry.last().label = solid.label;
            appendFace({{a,c,f}, {b,c,f}, {b,d,f}, {a,d,f}}, solid.color.darker(125));
            appendFace({{a,c,e}, {a,c,f}, {a,d,f}, {a,d,e}}, solid.color.darker(140));
            appendFace({{b,c,e}, {b,c,f}, {b,d,f}, {b,d,e}}, solid.color.darker(115));
            appendFace({{a,d,e}, {b,d,e}, {b,d,f}, {a,d,f}}, solid.color.lighter(135));
            appendFace({{a,c,e}, {b,c,e}, {b,c,f}, {a,c,f}}, solid.color.darker(160));
        } else {
            const float belt = solid.shape == SpatialSolidShape::Pyramid ? c : solid.center.y();
            const QVector<QVector3D> ring{{a,belt,e}, {b,belt,e}, {b,belt,f}, {a,belt,f}};
            const QVector3D top(solid.center.x(), d, solid.center.z());
            for (int edge = 0; edge < 4; ++edge) {
                appendFace({ring[edge], ring[(edge + 1) % 4], top}, solid.color.lighter(100 + edge * 10));
                if (solid.shape == SpatialSolidShape::Octahedron)
                    appendFace({ring[edge], ring[(edge + 1) % 4], {solid.center.x(), c, solid.center.z()}}, solid.color.darker(120));
            }
            if (solid.shape == SpatialSolidShape::Pyramid)
                appendFace(ring, solid.color.darker(140));
        }
        for (qsizetype index = firstFace; index < m_interiorGeometry.size(); ++index) {
            auto &mesh = m_interiorGeometry[index];
            const float angle = solid.yaw * 3.14159265358979323846f / 180;
            const auto rotate = [&](const QVector3D &world) {
                const auto local = world - solid.center;
                return solid.center + QVector3D(local.x() * std::cos(angle) - local.z() * std::sin(angle),
                    local.y(), local.x() * std::sin(angle) + local.z() * std::cos(angle));
            };
            mesh.center = rotate(mesh.center);
            for (auto &vertex : mesh.vertices)
                vertex = rotate(vertex);
            mesh.normal = QVector3D::crossProduct(mesh.vertices[1] - mesh.vertices[0], mesh.vertices[2] - mesh.vertices[0]).normalized();
            if (QVector3D::dotProduct(mesh.normal, mesh.center - solid.center) < 0)
                mesh.normal = -mesh.normal;
            const QVector3D sunlight = QVector3D(-0.4f, 0.85f, -0.3f).normalized();
            const float illumination = 0.62f + 0.38f * qMax(0.0f, QVector3D::dotProduct(mesh.normal, sunlight));
            mesh.color = QColor::fromRgbF(solid.color.redF() * illumination,
                solid.color.greenF() * illumination, solid.color.blueF() * illumination, solid.color.alphaF());
            mesh.albedo = solid.color;
        }
    }
    // One small ground contact shadow per grounded solid. Cached geometry only;
    // no shadow maps, ray queries, blur pass or inference during a flight frame.
    for (const auto &solid : m_interior->solids) {
        if (std::abs(solid.center.y() - solid.size.y() / 2) > 2)
            continue;
        const float halfWidth = solid.size.x() * 0.6f;
        const float halfDepth = solid.size.z() * 0.6f;
        const float angle = solid.yaw * 3.14159265358979323846f / 180;
        QVector<QVector3D> corners;
        for (const auto &corner : QVector<QVector3D>{{-halfWidth, 0, -halfDepth}, {halfWidth, 0, -halfDepth},
            {halfWidth, 0, halfDepth}, {-halfWidth, 0, halfDepth}}) {
            corners.append({solid.center.x() + corner.x() * std::cos(angle) - corner.z() * std::sin(angle),
                2, solid.center.z() + corner.x() * std::sin(angle) + corner.z() * std::cos(angle)});
        }
        appendFace(corners, QColor(0, 0, 0, 38));
    }
    for (const auto &surface : m_interior->rideSurfaces) {
        const auto mesh = surface.mesh();
        for (int index = 0; index < mesh.faces.size(); ++index) {
            QVector<QVector3D> polygon;
            for (int vertex : mesh.faces[index])
                polygon.append(mesh.vertices[vertex]);
            appendFace(polygon, mesh.faceColors.value(index, surface.color));
            m_interiorGeometry.last().material = mesh.material;
            m_interiorGeometry.last().albedo = surface.color;
            m_interiorGeometry.last().edge = surface.color;
        }
    }
    int vertices = 0;
    int faces = 0;
    for (int index = 0; index < qMin(room.meshes.size(), qsizetype(SpatialInterior::maximumMeshes)); ++index) {
        const auto &mesh = room.meshes[index];
        if (vertices + mesh.vertices.size() > SpatialInterior::maximumMeshVertices
            || faces + mesh.faces.size() > SpatialInterior::maximumMeshFaces)
            break;
        vertices += mesh.vertices.size();
        faces += mesh.faces.size();
        const bool smooth = mesh.hasSmoothNormals();
        for (int faceIndex = 0; faceIndex < mesh.faces.size(); ++faceIndex) {
            const auto &indices = mesh.faces[faceIndex];
            if (indices.size() < 3 || indices.size() > 4)
                continue;
            QVector<QVector3D> polygon;
            for (int vertex : indices) {
                if (vertex < 0 || vertex >= mesh.vertices.size())
                    break;
                const auto point = mesh.vertices[vertex];
                if (!std::isfinite(point.x()) || !std::isfinite(point.y()) || !std::isfinite(point.z()))
                    break;
                polygon.append(point);
            }
            if (polygon.size() == indices.size()) {
                appendFace(polygon, mesh.faceColors.value(faceIndex, mesh.color));
                auto &face = m_interiorGeometry.last();
                face.material = mesh.material;
                if (smooth) {
                    for (int vertex : indices)
                        face.normals.append(mesh.normals[vertex].normalized());
                    face.normal = QVector3D::crossProduct(polygon[1] - polygon[0], polygon[2] - polygon[0]).normalized();
                    face.albedo = face.color;
                    const auto sunlight = QVector3D(-.4f, .85f, -.3f).normalized();
                    const float illumination = .62f + .38f * qMax(0.f, QVector3D::dotProduct(face.normal, sunlight));
                    face.color = QColor::fromRgbF(face.albedo.redF() * illumination,
                        face.albedo.greenF() * illumination, face.albedo.blueF() * illumination, face.albedo.alphaF());
                }
                if (mesh.material != SpatialSurfaceMaterial::Plain)
                    face.edge = face.color;
            }
        }
    }
    if (!m_interior->foliage.isEmpty()) {
        static const QImage foliage(":/forgepipe/materials/juniper.png");
        static const QImage shadow = [&]() {
            QImage result = foliage.convertToFormat(QImage::Format_ARGB32_Premultiplied);
            QPainter tint(&result);
            tint.setCompositionMode(QPainter::CompositionMode_SourceIn);
            tint.fillRect(result.rect(), QColor(18, 23, 26, 90));
            tint.end();
            return result;
        }();
        m_interiorImages.insert("scenery:juniper", foliage);
        m_interiorImages.insert("scenery:shadow", shadow);
        const QVector3D light(-.4f, .85f, -.3f);
        const QVector3D side(-light.z(), 0, light.x());
        for (const auto &plant : m_interior->foliage) {
            const auto bottom = plant.base + QVector3D(0, 3, 0);
            const auto half = side.normalized() * plant.width * .5f;
            const auto offset = QVector3D(-light.x(), 0, -light.z()) * (plant.height / light.y());
            appendFace({bottom - half + offset, bottom + half + offset, bottom + half, bottom - half},
                Qt::transparent, "scenery:shadow");
        }
    }
    for (const auto &display : m_interior->displays) {
        const QColor backing = display.sprite ? QColor(Qt::transparent) : QColor("#0b131d");
        appendFace(SpatialInteriorGeometry::surface(*m_interior, display), backing, display.id);
    }
    if (!m_furnishing)
        m_furnishing = new FurnishingController(this);
    if (room.streaming)
        m_furnishing->detach();
    else
        m_furnishing->attach(*m_interior);
    m_camera = preservePosition ? previousPosition : QVector3D(room.spawn.x(), room.spawnHeight, room.spawn.y());
    if (preservePosition && blocked(m_camera))
        m_camera = QVector3D(room.spawn.x(), room.spawnHeight, room.spawn.y());
    m_pitch = preserveLook ? previousPitch : 0;
    m_yaw = preserveLook ? previousYaw : room.spawnYaw;
    SpatialReturnPortal::place(*m_interior, m_camera, m_yaw);
    updateTerrainGeometry();
    updateOrbitAnimation();
    updateFlightModeButton();
    viewport()->update();
}

void WorkspaceFlightView::streamInterior(const SpatialInterior &room, const QString &name,
    const QVector3D &position, float yawDelta)
{
    const auto keys = m_keys;
    const auto movement = m_pilotMovement;
    const auto look = m_pilotLook;
    const bool boost = m_pilotBoost;
    setInterior(room, name, true, true, room.environmentKey);
    m_camera = position;
    m_yaw += yawDelta;
    SpatialReturnPortal::place(*m_interior, m_camera, m_yaw);
    m_keys = keys;
    setPilotInput(movement, look, boost);
    if (!keys.isEmpty()) {
        m_elapsed.start();
        m_tick.start();
    }
    viewport()->update();
}

bool WorkspaceFlightView::setInteriorImage(const QString &id, const QImage &image)
{
    if (!m_interior || image.isNull() || image.width() > 512 || image.height() > 320)
        return false;
    bool found = false;
    for (const auto &display : m_interior->displays) {
        if (display.id == id)
            found = true;
    }
    // Up to 24 scene sources plus 24 furnishing previews: at most 30 MiB of RGBA pixels.
    found = found || (m_furnishing && m_furnishing->baseSource(id));
    if (!found || (!m_interiorImages.contains(id) && m_interiorImages.size() >= 48))
        return false;
    m_interiorImages.insert(id, image.convertToFormat(QImage::Format_ARGB32));
    viewport()->update();
    return true;
}

void WorkspaceFlightView::setInteriorPhase(qreal amount)
{
    const qreal bounded = qBound(0.0, amount, 1.0);
    if (qFuzzyCompare(m_interiorPhase, bounded))
        return;
    m_interiorPhase = bounded;
    viewport()->update();
}

void WorkspaceFlightView::setInteriorLocomotion(const QString &mode)
{
    if (!m_interior || m_interior->streaming || !QStringList{"walk", "fly", "drive"}.contains(mode))
        return;
    if (m_seatReturn || m_droneReturn) {
        leaveInteriorAction();
        if (m_seatReturn || m_droneReturn)
            return;
    }
    if (mode == "drive" && !m_interior->vehicles.isEmpty()) {
        if (m_activeVehicle < 0) {
            for (const auto &vehicle : m_interior->vehicles) {
                if (enterVehicle(vehicle.id))
                    return;
            }
            setFlightStatus(tr("Walk up to a vehicle, aim at it and press E to enter."));
        }
        if (locomotionChanged)
            locomotionChanged();
        return;
    }
    if (m_activeVehicle >= 0 && !exitVehicle())
        return;
    stop();
    m_interior->locomotion = mode;
    m_attitudeActive = false;
    updateFlightModeButton();
    if (mode != "fly")
        m_camera.setY(mode == "drive" ? 110 : SpatialInteriorGeometry::walkingHeight(*m_interior, m_camera));
    m_shipVisible = mode == "fly";
    if (locomotionChanged)
        locomotionChanged();
    viewport()->update();
}

void WorkspaceFlightView::setTerrainSharp(bool enabled)
{
    if (m_terrainSharp == enabled)
        return;
    m_terrainSharp = enabled;
    m_terrainFrame = {};
    viewport()->update();
}

void WorkspaceFlightView::paintInterior(QPainter *painter)
{
    if (m_terrainSharp || m_interior->groundMaterial == SpatialSurfaceMaterial::Plain
        || painter->paintEngine()->type() != QPaintEngine::OpenGL2) {
        paintInteriorScene(painter);
        return;
    }
    // Qt's projective textured brushes issue many small GL operations for BSP
    // fragments. Profiling this scene found raster composition substantially
    // faster. Keep the existing viewport/context and upload one bounded layer;
    // ordinary rooms, browser streams and orbit retain their existing path.
    // Explicit reduced-resolution mode only. Native rendering above uses the
    // viewport's actual framebuffer, without an intermediate image or size cap.
    QSize pixels = viewport()->size() * viewport()->devicePixelRatioF();
    const QSize limit(1280, 800);
    if (pixels.width() > limit.width() || pixels.height() > limit.height())
        pixels.scale(limit, Qt::KeepAspectRatio);
    if (pixels.isEmpty())
        return;
    if (m_terrainFrame.size() != pixels)
        m_terrainFrame = QImage(pixels, QImage::Format_RGB32);
    if (m_terrainFrame.isNull()) {
        paintInteriorScene(painter);
        return;
    }
    QPainter raster(&m_terrainFrame);
    raster.setRenderHints(painter->renderHints());
    raster.scale(qreal(pixels.width()) / viewport()->width(), qreal(pixels.height()) / viewport()->height());
    paintInteriorScene(&raster);
    raster.end();
    painter->setRenderHint(QPainter::SmoothPixmapTransform, true);
    painter->drawImage(viewport()->rect(), m_terrainFrame);
}

void WorkspaceFlightView::paintInteriorScene(QPainter *painter)
{
    const auto &room = *m_interior;
    const auto clipPolygon = [painter](const QPolygonF &polygon) {
        // Preserve vector coverage instead of converting each projected
        // polygon into a scanline region on every camera turn.
        QPainterPath path;
        path.addPolygon(polygon);
        path.closeSubpath();
        painter->setClipPath(path, Qt::IntersectClip);
    };
    const auto drawSurface = [painter](const QPolygonF &polygon) {
        // Qt's general polygon path handles arbitrary winding through stencil
        // passes. The common triangle/convex-quad case can draw directly.
        // Keep the general path for concave, clipped or imported polygons.
        bool convex = polygon.size() == 3;
        if (polygon.size() == 4) {
            double orientation = 0;
            convex = true;
            for (int index = 0; index < 4; ++index) {
                const QPointF first = polygon[(index + 1) % 4] - polygon[index];
                const QPointF second = polygon[(index + 2) % 4] - polygon[(index + 1) % 4];
                const double cross = first.x() * second.y() - first.y() * second.x();
                if (cross * orientation < 0) {
                    convex = false;
                    break;
                }
                if (cross != 0)
                    orientation = cross;
            }
        }
        if (convex)
            painter->drawConvexPolygon(polygon);
        else
            painter->drawPolygon(polygon);
    };
    // Individual terrain triangles share edges. Smoothing every split edge
    // adds seams/overdraw; texture filtering still smooths the surface itself.
    if (room.groundMaterial != SpatialSurfaceMaterial::Plain)
        painter->setRenderHint(QPainter::Antialiasing, false);
    const float x = room.size.width() / 2;
    const float z = room.size.height() / 2;
    const float ceiling = room.height;
    QLinearGradient sky(0, 0, 0, viewport()->height());
    sky.setColorAt(0, room.skyColor.darker(room.outdoors ? 130 : 100));
    sky.setColorAt(0.65, room.skyColor.lighter(room.outdoors ? 115 : 100));
    sky.setColorAt(1, room.skyColor);
    painter->fillRect(viewport()->rect(), sky);
    if (room.outdoors && room.groundMaterial != SpatialSurfaceMaterial::Plain) {
        const auto sun = cameraPoint(m_camera + QVector3D(-.4f, .85f, -.3f).normalized() * 10000);
        if (sun.z() > 8) {
            const auto center = screenPoint(sun);
            QRadialGradient glow(center, 100);
            glow.setColorAt(0, QColor(255, 248, 218, 235));
            glow.setColorAt(.12, QColor(255, 246, 216, 220));
            glow.setColorAt(.25, QColor(255, 245, 218, 45));
            glow.setColorAt(1, QColor(255, 245, 218, 0));
            painter->setPen(Qt::NoPen);
            painter->setBrush(glow);
            painter->drawEllipse(center, 100, 100);
        }
    }
    if (paintNativeInterior(painter)) {
        paintInteriorOverlay(painter);
        return;
    }
    // The clipped polygon controls coverage; the original plane controls UVs.
    // BSP cuts and near-plane clipping therefore never stretch or restart a tile.
    QHash<const void *, QBrush> textureBrushes;
    painter->setRenderHint(QPainter::SmoothPixmapTransform, true);
    const auto texture = [&](const QVector<QVector3D> &vertices, const QPolygonF &polygon,
                             SpatialSurfaceMaterial material, const QColor &tint, float depth, const void *cacheKey = nullptr) {
        if (material == SpatialSurfaceMaterial::Plain || vertices.size() < 3 || polygon.isEmpty())
            return false;
        if (cacheKey && textureBrushes.contains(cacheKey)) {
            const auto brush = textureBrushes.value(cacheKey);
            if (brush.style() == Qt::NoBrush)
                return false;
            painter->setPen(Qt::NoPen);
            painter->setBrush(brush);
            drawSurface(polygon);
            return true;
        }
        // LOD belongs to the original surface, not each BSP fragment. Switching
        // tiny fragments to flat colour produces stripes across otherwise
        // continuously textured jumps and cliffs.
        const auto footprint = cacheKey ? projectPolygon(vertices) : polygon;
        double twiceArea = 0;
        for (int index = 0; index < footprint.size(); ++index) {
            const auto &a = footprint[index];
            const auto &b = footprint[(index + 1) % footprint.size()];
            twiceArea += a.x()*b.y() - b.x()*a.y();
        }
        if (std::abs(twiceArea) < 256) {
            if (cacheKey)
                textureBrushes.insert(cacheKey, QBrush());
            return false;
        }
        const auto normal = QVector3D::crossProduct(vertices[1] - vertices[0], vertices[2] - vertices[0]).normalized();
        if (normal.isNull())
            return false;
        auto origin = vertices[0];
        auto u = (vertices[1] - vertices[0]).normalized();
        auto v = QVector3D::crossProduct(normal, u).normalized();
        if (material == SpatialSurfaceMaterial::Dirt || material == SpatialSurfaceMaterial::Grass
            || material == SpatialSurfaceMaterial::Rock) {
            // World-aligned terrain UVs stay continuous across triangles, ramp
            // shoulders and scene chunks. No per-face rotation or tile restart.
            const float plane = QVector3D::dotProduct(normal, origin);
            if (std::abs(normal.y()) >= std::abs(normal.x()) && std::abs(normal.y()) >= std::abs(normal.z())) {
                origin = {0, plane / normal.y(), 0};
                u = {1, -normal.x() / normal.y(), 0};
                v = {0, -normal.z() / normal.y(), 1};
            } else if (std::abs(normal.x()) > std::abs(normal.z())) {
                origin = {plane / normal.x(), 0, 0};
                u = {-normal.z() / normal.x(), 0, 1};
                v = {-normal.y() / normal.x(), 1, 0};
            } else {
                origin = {0, 0, plane / normal.z()};
                u = {1, 0, -normal.x() / normal.z()};
                v = {0, 1, -normal.y() / normal.z()};
            }
        }
        const float scale = SpatialSurfaceTextures::scale(material);
        const float tilePixels = scale * viewFocalLength() / qMax(80.f, depth);
        const int mip = qBound(0, int(std::log2(qMax(1.f, SpatialSurfaceTextures::tileSize / qMax(16.f, tilePixels)))), 4);
        const auto image = SpatialSurfaceTextures::tile(material, tint, mip);
        if (image.isNull())
            return false;
        // Direct plane-to-screen homography also works when the plane's
        // chosen UV origin lies on/behind the camera's near plane.
        const auto originCamera = cameraPoint(origin);
        const auto uCamera = (cameraPoint(origin + u * scale) - originCamera) / image.width();
        const auto vCamera = (cameraPoint(origin + v * scale) - originCamera) / image.width();
        const qreal focal = viewFocalLength();
        const qreal cx = viewport()->width() * .5;
        const qreal cy = viewport()->height() * .5;
        const QTransform transform(cx*uCamera.z() + focal*uCamera.x(), cy*uCamera.z() - focal*uCamera.y(), uCamera.z(),
            cx*vCamera.z() + focal*vCamera.x(), cy*vCamera.z() - focal*vCamera.y(), vCamera.z(),
            cx*originCamera.z() + focal*originCamera.x(), cy*originCamera.z() - focal*originCamera.y(), originCamera.z());
        if (!transform.isInvertible())
            return false;
        QBrush brush(image);
        brush.setTransform(transform);
        if (cacheKey)
            textureBrushes.insert(cacheKey, brush);
        painter->setPen(Qt::NoPen);
        painter->setBrush(brush);
        drawSurface(polygon);
        return true;
    };
    const auto face = [&](const QVector<QVector3D> &vertices, const QColor &color) {
        const auto polygon = projectPolygon(vertices);
        if (polygon.isEmpty())
            return;
        painter->setPen(QPen(QColor("#416274"), 1));
        painter->setBrush(color);
        drawSurface(polygon);
    };
    if (!room.terrain && (room.locomotion != "fly" || room.naturalGround)) {
        const float groundX = room.outdoors && room.naturalGround ? qMax(60000.f, x) : x;
        const float groundZ = room.outdoors && room.naturalGround ? qMax(60000.f, z) : z;
        const QVector<QVector3D> ground{{-groundX, 0, -groundZ}, {groundX, 0, -groundZ},
            {groundX, 0, groundZ}, {-groundX, 0, groundZ}};
        face(ground, room.floorColor);
        if (room.groundMaterial != SpatialSurfaceMaterial::Plain) {
            // Sixty-four fixed ground patches permit distance mip selection;
            // a single giant near-resolution tile shimmers at the horizon.
            for (int row = 0; row < 8; ++row) {
                for (int column = 0; column < 8; ++column) {
                    const float left = -x + column*x/4;
                    const float back = -z + row*z/4;
                    const QVector<QVector3D> patch{{left, 0, back}, {left+x/4, 0, back},
                        {left+x/4, 0, back+z/4}, {left, 0, back+z/4}};
                    const auto polygon = projectPolygon(patch);
                    if (polygon.isEmpty())
                        continue;
                    const float depth = cameraPoint({left+x/8, 0, back+z/8}).z();
                    texture(patch, polygon, room.groundMaterial, room.floorColor, depth);
                }
            }
            const auto forward = viewOrientation().rotatedVector({0, 0, 1});
            const QVector3D horizontal(forward.x(), 0, forward.z());
            const auto horizon = cameraPoint(m_camera + horizontal.normalized() * 100000);
            if (horizon.z() > 8) {
                const qreal top = screenPoint(horizon).y();
                QLinearGradient haze(0, top, 0, top + viewport()->height() * .12);
                QColor distant = room.skyColor;
                distant.setAlpha(175);
                haze.setColorAt(0, distant);
                distant.setAlpha(0);
                haze.setColorAt(1, distant);
                painter->save();
                clipPolygon(projectPolygon(ground));
                painter->fillRect(viewport()->rect(), haze);
                painter->restore();
            }
        }
    }
    if (!room.outdoors) {
    face({{-x, ceiling, -z}, {x, ceiling, -z}, {x, ceiling, z}, {-x, ceiling, z}}, QColor("#111d27"));
    face({{-x, 0, z}, {x, 0, z}, {x, ceiling, z}, {-x, ceiling, z}}, room.wallColor);
    face({{-x, 0, -z}, {x, 0, -z}, {x, ceiling, -z}, {-x, ceiling, -z}}, QColor("#1a303e"));
    face({{-x, 0, -z}, {-x, 0, z}, {-x, ceiling, z}, {-x, ceiling, -z}}, QColor("#203747"));
    face({{x, 0, -z}, {x, 0, z}, {x, ceiling, z}, {x, ceiling, -z}}, QColor("#294456"));
    }
    painter->setPen(QPen(QColor("#305363"), 1));
    const int grid = room.outdoors ? 400 : 200;
    for (int line = -int(x); !room.naturalGround && room.locomotion != "fly" && line <= x; line += grid)
        drawWorldLine(painter, {float(line), 1, -z}, {float(line), 1, z});
    for (int line = -int(z); !room.naturalGround && room.locomotion != "fly" && line <= z; line += grid)
        drawWorldLine(painter, {-x, 1, float(line)}, {x, 1, float(line)});
    if (room.locomotion == "fly" && !room.naturalGround) {
        for (int star = 0; star < 180; ++star) {
            const QVector3D direction(std::sin(star * 2.4), std::cos(star * 0.7), std::cos(star * 2.4));
            const auto projected = cameraPoint(m_camera + direction.normalized() * 10000);
            if (projected.z() > 8) {
                painter->setPen(QColor("#c0d5e8"));
                painter->drawPoint(screenPoint(projected));
            }
        }
    }
    painter->setPen(QPen(QColor("#8ddbd5"), 3));
    if (!room.outdoors) {
        drawWorldLine(painter, {-x, ceiling - 30, z - 1}, {x, ceiling - 30, z - 1});
        drawWorldLine(painter, {-x + 1, ceiling - 30, -z}, {-x + 1, ceiling - 30, z});
        drawWorldLine(painter, {x - 1, ceiling - 30, -z}, {x - 1, ceiling - 30, z});
    }
    // Fixed bounded cables, with state pulses. Geometry and discovery never run here.
    for (int index = 0; index < room.connections.size(); ++index) {
        const auto &connection = room.connections[index];
        const QVector3D start = connection.from + QVector3D(0, 150, 0);
        const QVector3D end = connection.to + QVector3D(0, 150, 0);
        painter->setPen(QPen(connection.color.darker(180), 2));
        drawWorldLine(painter, connection.from, start);
        drawWorldLine(painter, start, end);
        drawWorldLine(painter, end, connection.to);
        if (!connection.active)
            continue;
        const float phase = std::fmod(m_orbitSeconds * 0.32 + index * 0.17, 1.0);
        const QVector3D pulse = start * (1 - phase) + end * phase;
        painter->setPen(QPen(connection.color, 5));
        drawWorldLine(painter, pulse, pulse + (end - start) * 0.035f);
    }
    struct VisibleFace {
        const MeshFace *mesh;
        qreal depth;
        QPolygonF polygon;
    };
    QVector<VisibleFace> visible;
    visible.reserve(m_interiorGeometry.size() + m_furnishingGeometry.size());
    QVector<const MeshFace *> sources;
    sources.reserve(m_interiorGeometry.size() + m_furnishingGeometry.size() + 256);
    for (const auto &mesh : m_interiorGeometry)
        sources.append(&mesh);
    for (const auto &mesh : m_furnishingGeometry)
        sources.append(&mesh);
    const int staticFaces = m_interiorGeometry.size() + m_furnishingStaticFaces;
    if (m_interiorPaintDirty) {
        QVector<SpatialPaintOrder::Polygon> polygons;
        for (int index = 0; index < staticFaces; ++index)
            polygons.append({sources[index]->vertices, index});
        m_interiorPaintOrder.rebuild(std::move(polygons));
        m_interiorPaintDirty = false;
    }
    QVector<SpatialPaintOrder::Polygon> moving;
    const auto outsideView = [&](const QVector<QVector3D> &vertices) {
        unsigned planes = 31;
        for (const auto &vertex : vertices) {
            planes &= FlightProjection::outsidePlanes(cameraPoint(vertex), viewport()->size(), viewFocalLength());
            if (!planes)
                return false;
        }
        return true;
    };
    // One upright camera-facing card per tree. The existing BSP order clips
    // it against buildings/terrain; shared alpha artwork supplies fine detail.
    m_foliageGeometry.resize(room.foliage.size());
    for (int index = 0; index < room.foliage.size(); ++index) {
        const auto &plant = room.foliage[index];
        auto &mesh = m_foliageGeometry[index];
        auto direction = m_camera - plant.base;
        direction.setY(0);
        if (direction.lengthSquared() < 1)
            direction = {0, 0, -1};
        const QVector3D side(-direction.z(), 0, direction.x());
        const auto half = side.normalized() * plant.width * .5f;
        const auto up = QVector3D(0, plant.height, 0);
        mesh.vertices = {plant.base - half + up, plant.base + half + up,
            plant.base + half, plant.base - half};
        mesh.center = plant.base + up * .5f;
        mesh.color = Qt::transparent;
        mesh.edge = Qt::transparent;
        mesh.displayId = "scenery:juniper";
    }
    for (int index = staticFaces; index < sources.size(); ++index) {
        if (!outsideView(sources[index]->vertices))
            moving.append({sources[index]->vertices, index});
    }
    for (const auto &mesh : m_foliageGeometry) {
        if (outsideView(mesh.vertices))
            continue;
        moving.append({mesh.vertices, int(sources.size())});
        sources.append(&mesh);
    }
    for (const auto &mesh : m_terrainGeometry) {
        if (outsideView(mesh.vertices))
            continue;
        moving.append({mesh.vertices, int(sources.size())});
        sources.append(&mesh);
    }
    for (int index = 0; index < m_vehicleGeometry.size(); ++index) {
        for (int face = 0; face < m_vehicleGeometry[index].size(); ++face) {
            if (!visibleVehicleFace(index, face))
                continue;
            const auto &mesh = m_vehicleGeometry[index][face];
            if (!mesh.normal.isNull() && QVector3D::dotProduct(mesh.normal, m_camera - mesh.center) <= 0)
                continue;
            if (outsideView(mesh.vertices))
                continue;
            moving.append({mesh.vertices, int(sources.size())});
            sources.append(&mesh);
        }
    }
    m_interiorPaintOrder.visit(m_camera, viewOrientation().rotatedVector({0, 0, 1}), moving,
        [&](const SpatialPaintOrder::Polygon &fragment) {
            const auto &mesh = *sources[fragment.source];
            if ((!mesh.displayId.isEmpty() && m_furnishing && m_furnishing->suppress(mesh.displayId))
                || (!mesh.normal.isNull() && QVector3D::dotProduct(mesh.normal, m_camera - mesh.center) <= 0))
                return;
            if (outsideView(fragment.vertices))
                return;
            const auto polygon = projectPolygon(fragment.vertices);
            if (mesh.material != SpatialSurfaceMaterial::Plain) {
                double twiceArea = 0;
                for (int index = 0; index < polygon.size(); ++index) {
                    const auto &a = polygon[index];
                    const auto &b = polygon[(index + 1) % polygon.size()];
                    twiceArea += a.x()*b.y() - b.x()*a.y();
                }
                if (std::abs(twiceArea) < 4)
                    return;
            }
            if (!polygon.isEmpty() && polygon.boundingRect().intersects(viewport()->rect()))
                visible.append({&mesh, cameraPoint(mesh.center).z(), polygon});
        });
    QSet<int> visibleStreams;
    for (const auto &visibleFace : visible) {
        const auto &mesh = *visibleFace.mesh;
        const auto &polygon = visibleFace.polygon;
        QColor surface = mesh.color;
        if (room.outdoors && mesh.displayId.isEmpty() && surface.alpha() == 255) {
            const float fog = qBound(0.0, (visibleFace.depth - 1000) / (std::isfinite(room.fogDistance) ? qBound(3000.f,room.fogDistance,30000.f) : 6500.f), 0.55);
            surface = QColor::fromRgbF(surface.redF() * (1 - fog) + room.skyColor.redF() * fog,
                surface.greenF() * (1 - fog) + room.skyColor.greenF() * fog,
                surface.blueF() * (1 - fog) + room.skyColor.blueF() * fog);
        }
        painter->setBrush(surface);
        const bool selectedScreen = m_furnishing && m_furnishing->active()
            && !mesh.displayId.isEmpty()
            && (m_furnishing->selectedId() == "base:" + mesh.displayId
                || mesh.displayId == "furnishing:" + m_furnishing->selectedId());
        // Filled model faces need no same-colour outline when edge smoothing
        // is off. Avoid drawing every triangle twice; keep selection/legacy
        // accents and the existing seam treatment in antialiased rooms.
        const QColor edge = mesh.edge.isValid() ? surface : surface.lighter(110);
        if (!selectedScreen && (mesh.material != SpatialSurfaceMaterial::Plain
            || (mesh.edge.isValid() && !painter->testRenderHint(QPainter::Antialiasing))))
            painter->setPen(Qt::NoPen);
        else
            painter->setPen(QPen(selectedScreen ? QColor("#a4ffdf") : edge, selectedScreen ? 4 : 1));
        const bool textured = texture(mesh.vertices, polygon, mesh.material, mesh.color, visibleFace.depth, &mesh);
        if (!textured)
            drawSurface(polygon);
        // Base-colour fallbacks already contain fog. Applying it twice made
        // distant small shoulder triangles visibly brighter than adjacent dirt.
        if (textured && room.outdoors) {
            const float fog = qBound(0.0, (visibleFace.depth - 1000) / (std::isfinite(room.fogDistance) ? qBound(3000.f, room.fogDistance, 30000.f) : 6500.f), .55);
            QColor haze = room.skyColor;
            haze.setAlphaF(fog);
            if (fog > .01f) {
                painter->setPen(Qt::NoPen);
                painter->setBrush(haze);
                drawSurface(polygon);
            }
        }
        if (!mesh.label.isEmpty()) {
            painter->save();
            painter->setPen(QColor("#fff0ce"));
            clipPolygon(polygon);
            const auto bounds = projectPolygon(mesh.vertices).boundingRect().adjusted(4, 4, -4, -4);
            QFont signage = painter->font();
            signage.setBold(true);
            signage.setPixelSize(qBound(11, int(qMin(bounds.height() * 0.5,
                bounds.width() / qMax(1.0, mesh.label.size() * 0.65))), 44));
            painter->setFont(signage);
            painter->drawText(bounds, Qt::AlignCenter,
                painter->fontMetrics().elidedText(mesh.label, Qt::ElideRight, int(bounds.width())));
            painter->restore();
        }
        if (mesh.displayId.isEmpty())
            continue;
        const QString source = m_furnishing ? m_furnishing->sourceId(mesh.displayId) : mesh.displayId;
        const int stream = m_interiorBindings.value(source, -1);
        const FlightAppFrame *frame = nullptr;
        if (m_billboards && stream >= 0 && stream < m_billboards->frames().size()) {
            frame = &m_billboards->frames()[stream];
            visibleStreams.insert(stream);
        }
        painter->save();
        clipPolygon(polygon);
        const QImage displayImage = frame ? frame->image : m_interiorImages.value(source);
        if (!displayImage.isNull()) {
            QPolygonF projected;
            bool inFront = true;
            for (const auto &corner : mesh.vertices) {
                const auto point = cameraPoint(corner);
                if (point.z() < 8) {
                    inFront = false;
                    break;
                }
                projected.append(screenPoint(point));
            }
            // Textured surfaces remain readable from either side. Their front
            // normal follows the display's horizontal direction and down axis.
            const auto normal = QVector3D::crossProduct(mesh.vertices[1] - mesh.vertices[0], mesh.vertices[3] - mesh.vertices[0]);
            if (projected.size() == 4 && QVector3D::dotProduct(normal, m_camera - mesh.center) < 0)
                projected = {projected[1], projected[0], projected[3], projected[2]};
            QTransform transform;
            const qreal width = (mesh.vertices[1] - mesh.vertices[0]).length();
            const qreal height = (mesh.vertices[3] - mesh.vertices[0]).length();
            const QPolygonF source{{0, 0}, {width, 0}, {width, height}, {0, height}};
            if (inFront && QTransform::quadToQuad(source, projected, transform)) {
                const QSizeF fitted = mesh.displayId.startsWith("scenery:") ? QSizeF(width, height)
                    : QSizeF(displayImage.size()).scaled(QSizeF(width, height), Qt::KeepAspectRatio);
                const QRectF imageRect(QPointF((width - fitted.width()) / 2, (height - fitted.height()) / 2), fitted);
                painter->save();
                painter->setTransform(transform);
                painter->drawImage(imageRect, displayImage);
                painter->restore();
            }
        } else {
            for (const auto &display : room.displays) {
                if (display.id != mesh.displayId)
                    continue;
                painter->setPen(QColor("#d5e9f0"));
                painter->drawText(projectPolygon(mesh.vertices).boundingRect().adjusted(8, 8, -8, -8), Qt::AlignCenter | Qt::TextWordWrap,
                    display.application.name + "\n" + (frame ? frame->status
                        : (display.prompt.isEmpty() ? tr("Select or press E to connect") : display.prompt)));
                break;
            }
        }
        painter->restore();
    }
    if (m_billboards)
        m_billboards->setVisibleStreams(visibleStreams);
    paintInteriorOverlay(painter);
}

void WorkspaceFlightView::paintInteriorOverlay(QPainter *painter)
{
    const auto &room = *m_interior;
    if (room.locomotion == "fly" && m_shipVisible)
        paintCockpit(painter);
    if (m_activeVehicle >= 0)
        paintVehicleCockpit(painter);
    paintTerrainWeather(painter);
    paintReturnPortal(painter);
    const QPoint center = viewport()->rect().center();
    painter->setPen(QPen(QColor("#bef7eb"), 1));
    painter->drawLine(center + QPoint(-6, 0), center + QPoint(6, 0));
    painter->drawLine(center + QPoint(0, -6), center + QPoint(0, 6));
    const auto target = room.actions.isEmpty() || m_seatReturn || m_droneReturn ? QString() : SpatialInteriorGeometry::target(room, m_camera,
        selectionRay(viewport()->rect().center()));
    for (const auto &action : room.actions) {
        if (target != "action:" + action.id)
            continue;
        painter->setPen(QColor("#ffe3ab"));
        painter->drawText(QRect(20, viewport()->height() / 2 + 35, viewport()->width() - 40, 30),
            Qt::AlignCenter, action.title);
        break;
    }
    painter->setPen(Qt::white);
    painter->drawText(QRect(20, 65, viewport()->width() - 40, 32), Qt::AlignCenter, m_interiorName);
    painter->drawText(QRect(20, viewport()->height() - 64, viewport()->width() - 40, 50), Qt::AlignCenter | Qt::TextWordWrap,
        inputHints() + "\n" + m_worldStatus);
    if (m_interiorPhase > 0) {
        QColor phase("#abd8cf");
        phase.setAlphaF(m_interiorPhase);
        painter->fillRect(viewport()->rect(), phase);
    }
}

bool WorkspaceFlightView::connectInteriorDisplay(const QString &id, const DesktopAppChoice &choice, bool control)
{
    if (!m_interior || !choice.id || choice.applicationClass.isEmpty())
        return false;
    const SpatialDisplay *selected = m_furnishing ? m_furnishing->baseDisplay(id) : nullptr;
    for (const auto &display : m_interior->displays) {
        if (display.id == id)
            selected = &display;
    }
    if (!selected || selected->application.windowClass.compare(choice.applicationClass, Qt::CaseInsensitive) != 0)
        return false;
    int stream = -1;
    if (m_billboards) {
        for (int index = 0; index < m_interiorStreams.size() && index < m_billboards->frames().size(); ++index)
            m_interiorStreams[index] = m_billboards->choice(index);
    }
    for (int index = 0; index < m_interiorStreams.size(); ++index) {
        if (m_interiorStreams[index].id == choice.id)
            stream = index;
    }
    if (stream < 0) {
        const int previous = m_interiorBindings.value(id, -1);
        if (m_billboards && previous >= 0 && previous < m_billboards->frames().size()
            && !m_billboards->windowAlive(previous))
            stream = previous;
    }
    if (stream < 0) {
        // A rebound or removed furnishing must not permanently occupy a live slot.
        for (int index = 0; index < m_interiorStreams.size(); ++index) {
            if (!m_interiorBindings.values().contains(index)) {
                stream = index;
                break;
            }
        }
    }
    if (stream < 0) {
        if (m_interiorStreams.size() >= 3) {
            setFlightStatus(tr("Three live apps are connected. Disable App control to disconnect them before choosing others."));
            return false;
        }
        stream = m_interiorStreams.size();
        m_interiorStreams.append(choice);
        setAppBillboards(m_interiorStreams);
        m_billboards->setVisibleStreams({});
    } else if (m_interiorStreams[stream].id != choice.id) {
        m_interiorStreams[stream] = choice;
        setAppBillboards(m_interiorStreams);
        m_billboards->setVisibleStreams({});
    }
    m_interiorBindings.insert(id, stream);
    // References to the same catalogue identity share this explicitly selected stream.
    for (const auto &display : m_interior->displays) {
        if (m_furnishing && (m_furnishing->applicationSource(id) || m_furnishing->applicationSource(display.id)))
            continue;
        if (display.application.desktopId == selected->application.desktopId
            && display.application.windowClass == selected->application.windowClass)
            m_interiorBindings.insert(display.id, stream);
    }
    viewport()->update();
    if (control) {
        if (!beginCabinetControl(id))
            return false;
        m_billboards->control(stream);
        if (!appControlling()) {
            endCabinetControl();
            return false;
        }
    }
    return true;
}

bool WorkspaceFlightView::beginCabinetControl(const QString &id)
{
    if (!m_furnishing || !m_interior)
        return true;
    for (const auto &object : m_furnishing->objects()) {
        if (object.removed || object.type != "arcade-cabinet" || FurnishingCatalog::display(object).id != id)
            continue;
        if (appControlling() || m_cabinetReturn || m_seatReturn)
            return false;
        const auto display = FurnishingCatalog::display(object);
        const auto position = display.center + FurnishingCatalog::rotate({0, 22, -220}, object.yaw);
        if ((m_camera - display.center).length() > 500 || SpatialInteriorGeometry::blocked(*m_interior, position)) {
            setFlightStatus(tr("Move closer and leave space in front of the arcade cabinet to play."));
            return false;
        }
        stop();
        m_cabinetReturn = m_camera;
        m_cabinetLook = {m_yaw, m_pitch};
        m_cabinetDisplay = id;
        m_camera = position;
        m_yaw = -object.yaw * 3.14159265358979323846 / 180;
        m_pitch = 0;
        m_attitudeActive = false;
        viewport()->update();
        break;
    }
    return true;
}

QRect WorkspaceFlightView::cabinetControlRect() const
{
    if (!m_interior || m_cabinetDisplay.isEmpty())
        return {};
    for (const auto &display : m_interior->displays) {
        if (display.id == m_cabinetDisplay)
            return projectPolygon(SpatialInteriorGeometry::surface(*m_interior, display)).boundingRect().toAlignedRect();
    }
    return {};
}

void WorkspaceFlightView::endCabinetControl()
{
    if (!m_cabinetReturn)
        return;
    m_camera = *m_cabinetReturn;
    m_yaw = m_cabinetLook.x();
    m_pitch = m_cabinetLook.y();
    m_cabinetReturn.reset();
    m_cabinetDisplay.clear();
    viewport()->update();
}

quint64 WorkspaceFlightView::captureCount() const
{
    return m_billboards ? m_billboards->captureCount() : 0;
}

bool WorkspaceFlightView::hasInteriorPreview(const QString &id) const
{
    const int stream = m_interiorBindings.value(id, -1);
    return m_billboards && stream >= 0 && stream < m_billboards->frames().size()
        && !m_billboards->frames()[stream].image.isNull();
}

qint64 WorkspaceFlightView::lastCaptureNanoseconds() const
{
    return m_billboards ? m_billboards->lastCaptureNanoseconds() : 0;
}
