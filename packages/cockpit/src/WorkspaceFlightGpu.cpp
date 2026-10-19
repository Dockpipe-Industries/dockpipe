#include "WorkspaceFlightView.h"
#include "SpatialGpuRenderer.h"
#include "SpatialGpuFoliage.h"
#include "FlightProjection.h"
#include "FlightAppBillboards.h"
#include <QCache>
#include <QPainter>
#include <QPaintEngine>
#include <algorithm>

QString WorkspaceFlightView::nativeSceneError() const
{
    return m_gpuRenderer ? m_gpuRenderer->error() : QStringLiteral("Scene is not eligible for native geometry");
}

bool WorkspaceFlightView::paintNativeInterior(QPainter *painter)
{
    m_nativeSceneActive = false;
    // Reject the software painter before packing geometry or decoding native
    // scenery/material resources. The renderer's late guard is still required
    // for direct callers, but fallback frames should not pay both paths' cost.
    if (painter->paintEngine()->type() != QPaintEngine::OpenGL2)
        return false;
    const auto &room = *m_interior;
    if (!m_terrainSharp || !room.outdoors || room.groundMaterial == SpatialSurfaceMaterial::Plain
        || !room.displays.isEmpty() || !room.connections.isEmpty() || !m_interiorStreams.isEmpty())
        return false;
    if (!m_gpuRenderer)
        m_gpuRenderer = new SpatialGpuRenderer(this);
    QVector<SpatialGpuRenderer::Face> opaque;
    QVector<SpatialGpuRenderer::Face> transparent;
    // Natural terrain continues into the distance beyond the bounded playable
    // plot. A single shared ground plane avoids a visible floating-map edge.
    const float halfWidth = room.naturalGround ? qMax(60000.f, float(room.size.width() / 2)) : room.size.width() / 2;
    const float halfDepth = room.naturalGround ? qMax(60000.f, float(room.size.height() / 2)) : room.size.height() / 2;
    // Flat ride lanes touch y=0. Keep the underlying ground half a centimetre
    // below them so depth precision cannot alternate grass/dirt/shadow pixels.
    if (!room.terrain)
        opaque.append({{{-halfWidth, -.5f, -halfDepth}, {halfWidth, -.5f, -halfDepth},
        {halfWidth, -.5f, halfDepth}, {-halfWidth, -.5f, halfDepth}}, {0, 1, 0}, room.floorColor, room.groundMaterial});
    const auto append = [&](const MeshFace &mesh) {
        if (mesh.displayId == "scenery:shadow")
            return; // The native pass casts foliage shadows onto actual geometry.
        SpatialGpuRenderer::Face face{mesh.vertices, mesh.normal, mesh.color, mesh.material};
        if (mesh.albedo.isValid())
            face.color = mesh.albedo;
        face.normals = mesh.normals;
        face.finish = mesh.finish;
        face.uvs = mesh.uvs;
        if (!mesh.displayId.isEmpty()) {
            face.image = m_interiorImages.value(mesh.displayId);
            if (face.image.isNull())
                return;
            face.cutout = mesh.displayId == "scenery:juniper";
            face.unlit = true;
        }
        if (!face.cutout && (face.color.alpha() < 255 || face.image.hasAlphaChannel()))
            transparent.append(face);
        else
            opaque.append(face);
        if (mesh.label.isEmpty())
            return;
        // Bounded shared signage textures remain in world space and obey the
        // same depth test as buildings; no text drawing over distant geometry.
        static QCache<QString, QImage> labels(4 * 1024 * 1024);
        QImage label;
        if (const auto *cached = labels.object(mesh.label)) {
            label = *cached;
        } else {
            label = QImage(512, 128, QImage::Format_RGBA8888);
            label.fill(Qt::transparent);
            QPainter text(&label);
            text.setRenderHint(QPainter::TextAntialiasing);
            QFont font = painter->font();
            font.setBold(true);
            font.setPixelSize(qBound(16, 850 / qMax(1, mesh.label.size()), 44));
            text.setFont(font);
            text.setPen(QColor("#fff1d4"));
            text.drawText(label.rect().adjusted(12, 6, -12, -6), Qt::AlignCenter | Qt::TextWordWrap, mesh.label);
            text.end();
            labels.insert(mesh.label, new QImage(label), int(label.sizeInBytes()));
        }
        face.image = label;
        // Box labels use bottom-left, bottom-right, top-right, top-left
        // vertices. QImage row zero is the top; billboard UVs invert these.
        face.uvs = {{0,1}, {1,1}, {1,0}, {0,0}};
        // Fit the 4:1 text image inside the sign instead of stretching letters
        // to the sign's proportions. Keep the underlying solid face intact.
        const auto horizontal = face.vertices[1] - face.vertices[0];
        const auto vertical = face.vertices[3] - face.vertices[0];
        const float width = horizontal.length();
        const float height = vertical.length();
        if (width > .001f && height > .001f) {
            const auto center = (face.vertices[0] + face.vertices[2]) * .5f;
            const auto halfWidth = horizontal * (.5f * qMin(1.f, height * 4 / width));
            const auto halfHeight = vertical * (.5f * qMin(1.f, width / (height * 4)));
            face.vertices = {center - halfWidth - halfHeight, center + halfWidth - halfHeight,
                center + halfWidth + halfHeight, center - halfWidth + halfHeight};
        }
        // Keep the label just above its backing. Coplanar textured signage
        // otherwise alternates with the backing at oblique viewing angles.
        for (auto &vertex : face.vertices)
            vertex += face.normal * .3f;
        face.material = SpatialSurfaceMaterial::Plain;
        face.unlit = true;
        face.cutout = true;
        opaque.append(face);
    };
    for (const auto &mesh : m_terrainGeometry)
        append(mesh);
    for (const auto &mesh : m_interiorGeometry)
        append(mesh);
    for (const auto &mesh : m_furnishingGeometry)
        append(mesh);
    for (int index = 0; index < m_vehicleGeometry.size(); ++index) {
        for (int face = 0; face < m_vehicleGeometry[index].size(); ++face) {
            if (visibleVehicleFace(index, face))
                append(m_vehicleGeometry[index][face]);
        }
    }
    QVector<SpatialGpuRenderer::InstanceGroup> instances;
    const auto pine = (room.environmentKey == "dockpipe:mesa-moto:v1" || room.terrain) ? SpatialGpuFoliage::pine() : nullptr;
    if (pine && !pine->isEmpty())
        instances.append({pine, {}});
    int plantIndex = 0;
    for (const auto &plant : room.foliage) {
        if (!instances.isEmpty()) {
            // Placement and collision admission already reserve the complete
            // rotating canopy envelope. The shared mesh stays inside it.
            instances[0].instances.append({plant.base, {plant.width, plant.height, plant.width},
                plantIndex * 2.39996f, .88f + .03f * (plantIndex % 5)});
            ++plantIndex;
            continue;
        }
        auto direction = m_camera - plant.base;
        direction.setY(0);
        if (direction.lengthSquared() < 1)
            direction = {0, 0, -1};
        const QVector3D side(-direction.z(), 0, direction.x());
        const auto half = side.normalized() * plant.width * .5f;
        const QVector3D up(0, plant.height, 0);
        SpatialGpuRenderer::Face tree;
        tree.vertices = {plant.base - half + up, plant.base + half + up, plant.base + half, plant.base - half};
        tree.color = Qt::white;
        tree.normal = direction.normalized();
        tree.image = m_interiorImages.value("scenery:juniper");
        tree.cutout = true;
        tree.unlit = true;
        opaque.append(tree);
    }
    if (pine && !pine->isEmpty()) {
        SpatialGpuRenderer::InstanceGroup groundCover{SpatialGpuFoliage::grassPatch(), {}};
        // Use spare slots in the existing 96-instance contract. Tree admission
        // already checks this whole footprint against ramps, routes and terrain.
        // Grass is decorative and cannot introduce hidden riding obstacles.
        const int spare = qMax(0, SpatialGpuRenderer::maximumInstances - int(room.foliage.size()));
        const int patches = qMin(24, qMin(spare, int(room.foliage.size())));
        for (int index = 0; index < patches; ++index) {
            const auto &plant = room.foliage[index];
            const float distance = (plant.base - m_camera).length();
            const float growth = qBound(0.f, (8000 - distance) / 2500, 1.f);
            if (growth <= .001f)
                continue;
            const float height = (35 + (index % 4) * 7) * growth;
            groundCover.instances.append({plant.base, {plant.width, height, plant.width},
                index * 2.39996f, .9f + .04f * (index % 4)});
        }
        if (!groundCover.instances.isEmpty())
            instances.append(std::move(groundCover));
    }
    const auto depth = [this](const SpatialGpuRenderer::Face &face) {
        QVector3D center;
        for (const auto &point : face.vertices)
            center += point;
        return QVector3D::dotProduct(center / face.vertices.size() - m_camera, m_viewForward);
    };
    std::stable_sort(transparent.begin(), transparent.end(), [&](const auto &first, const auto &second) {
        return depth(first) > depth(second);
    });
    opaque += transparent;
    SpatialGpuRenderer::Camera camera;
    camera.position = m_camera;
    camera.right = m_viewRight;
    camera.up = m_viewUp;
    camera.forward = m_viewForward;
    camera.viewport = viewport()->size();
    camera.focalLength = viewFocalLength();
    camera.sky = room.skyColor;
    camera.fogDistance = room.fogDistance;
    camera.samples = viewport()->width() * qint64(viewport()->height()) <= 2560LL * 1440 ? 4 : 2;
    if (room.environmentKey == "dockpipe:mesa-moto:v1" || (room.terrain && !room.terrain->raining()))
        camera.environment = SpatialSurfaceTextures::daySky();
    m_nativeSceneActive = m_gpuRenderer->render(painter, camera, opaque, instances);
    if (m_nativeSceneActive && m_billboards)
        m_billboards->setVisibleStreams({});
    return m_nativeSceneActive;
}
