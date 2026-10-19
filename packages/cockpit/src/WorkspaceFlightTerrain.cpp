#include "WorkspaceFlightView.h"
#include <QPainter>
#include <cmath>

void WorkspaceFlightView::toggleTerrainRain()
{
    if (!terrainWeatherAvailable())
        return;
    auto &terrain = *m_interior->terrain;
    terrain.setRaining(!terrain.raining());
    m_interior->skyColor = terrain.raining() ? QColor("#687a83") : QColor("#a2b4be");
    m_terrainGeometryAge = 1;
    setFlightStatus(terrain.raining() ? tr("Rain on · Soil softens and low spots collect runoff")
        : tr("Rain off · Wet ground and puddles drain gradually"));
    m_elapsed.start();
    m_tick.start();
    viewport()->update();
}

void WorkspaceFlightView::updateTerrainGeometry()
{
    if (!terrainWeatherAvailable())
        return;
    if (!m_terrainGeometry.isEmpty() && m_terrainGeometryAge < .2f
        && (m_camera - m_terrainGeometryCamera).lengthSquared() < 40000)
        return;
    int destination = 0;
    const auto meshes = m_interior->terrain->meshes(m_camera);
    for (int meshIndex = 0; meshIndex < meshes.size(); ++meshIndex) {
        const auto &mesh = meshes[meshIndex];
        for (int index = 0; index < mesh.faces.size(); ++index) {
            if (destination == m_terrainGeometry.size())
                m_terrainGeometry.append(MeshFace{});
            auto &face = m_terrainGeometry[destination++];
            face.vertices.clear();
            face.center = {};
            face.finish = SpatialSurfaceFinish::Standard;
            for (int vertex : mesh.faces[index]) {
                face.vertices.append(mesh.vertices[vertex]);
                face.center += mesh.vertices[vertex];
            }
            face.center /= face.vertices.size();
            face.normal = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
                face.vertices[2] - face.vertices[0]).normalized();
            face.color = mesh.faceColors[index];
            face.albedo = face.color;
            face.edge = face.color;
            face.material = mesh.material;

        }
    }
    m_terrainGeometry.resize(destination);
    m_terrainGeometryCamera = m_camera;
    m_terrainGeometryAge = 0;
}

void WorkspaceFlightView::paintTerrainWeather(QPainter *painter)
{
    if (!terrainWeatherAvailable())
        return;
    const auto &terrain = *m_interior->terrain;
    if (terrain.raining()) {
        const auto rect = viewport()->rect();
        painter->setPen(QPen(QColor(195, 215, 225, 95), 1));
        // Fixed deterministic screen-space streaks; water simulation is in
        // world space and does not depend on these presentation particles.
        for (int index = 0; index < 180; ++index) {
            const float x = std::fmod(index * 137.2f + m_weatherTime * 110, float(rect.width()));
            const float y = std::fmod(index * 91.7f + m_weatherTime * (650 + index % 5 * 70), float(rect.height()));
            painter->drawLine(QPointF(x, y), QPointF(x - 7, y + 22));
        }
    }
    QString weather = tr("DRY");
    if (terrain.raining())
        weather = tr("RAIN");
    else if (terrain.wetness() > .01f)
        weather = tr("DRYING");
    painter->setPen(QColor("#f4eee1"));
    painter->drawText(QRect(18, 62, 500, 26), Qt::AlignLeft,
        tr("%1 · Soil %2% wet · T rain · Ruts persist this session")
            .arg(weather)
            .arg(qRound(terrain.wetness() * 100)));
}
