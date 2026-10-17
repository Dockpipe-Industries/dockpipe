#include "WorkspaceFlightView.h"
#include "FlightPlanetArt.h"
#include "FlightSolarSystem.h"
#include "FlightProjection.h"
#include <QLinearGradient>
#include <QPainter>
#include <QPainterPath>
#include <QRadialGradient>
#include <algorithm>

void WorkspaceFlightView::paintPlanets(QPainter *painter)
{
    const qreal focal = FlightProjection::focalLength(viewport()->size());
    const QVector3D sun = FlightSolarSystem::sun();
    m_planetLabels.clear();
    painter->save();
    painter->setRenderHint(QPainter::Antialiasing);
    painter->setRenderHint(QPainter::SmoothPixmapTransform);
    painter->setBrush(Qt::NoBrush);
    // A finite 64-segment orbit per destination; no trails or retained frames.
    QPainterPath orbitalPaths;
    for (int index = 0; index < m_buildings.size(); ++index) {
        const qreal radius = FlightSolarSystem::radius(index);
        QVector<QVector3D> vertices;
        vertices.reserve(64);
        for (int segment = 0; segment < 64; ++segment) {
            const qreal angle = segment * 6.283185307179586 / 64;
            vertices.append(sun + QVector3D(std::cos(angle) * radius, 0, std::sin(angle) * radius));
        }
        const QPolygonF orbit = projectPolygon(vertices);
        if (!orbit.isEmpty()) {
            orbitalPaths.addPolygon(orbit);
            orbitalPaths.closeSubpath();
        }
    }
    painter->save();
    // Polygon clipping can add edges exactly on the viewport boundary. Exclude
    // that one-pixel border so only real orbital arcs remain visible.
    painter->setClipRect(viewport()->rect().adjusted(1, 1, -1, -1), Qt::IntersectClip);
    painter->setPen(QPen(QColor(124, 161, 174, 44), 1));
    painter->drawPath(orbitalPaths);
    painter->restore();

    struct Body {
        int index;
        QVector3D point;
        QPointF center;
        qreal radius;
    };
    QVector<Body> bodies;
    for (int index = -1; index < m_buildings.size(); ++index) {
        const QVector3D world = index < 0 ? sun : m_buildings[index].position;
        const QVector3D point = cameraPoint(world);
        const qreal radius = index < 0 ? 360 : 180;
        if (point.z() <= FlightProjection::nearPlane || !sphereVisible(point, radius * 1.3))
            continue;
        bodies.append({index, point, screenPoint(point),
            qMin(qreal(qMax(viewport()->width(), viewport()->height())) * 2, focal * radius / point.z())});
    }
    std::sort(bodies.begin(), bodies.end(), [](const Body &a, const Body &b) { return a.point.z() > b.point.z(); });
    for (const auto &body : bodies) {
        const qreal radius = body.radius;
        const QPointF center = body.center;
        if (body.index < 0) {
            const qreal halo = radius / 0.28;
            painter->drawImage(QRectF(center.x() - halo, center.y() - halo, halo * 2, halo * 2), FlightPlanetArt::sun());
            continue;
        }
        const auto &planet = m_buildings[body.index];
        const int material = FlightPlanetArt::material(body.index, planet.source.theme);
        const QColor accent = FlightPlanetArt::color(material);
        painter->setPen(Qt::NoPen);
        const qreal haloRadius = radius * 1.22;
        painter->drawImage(QRectF(center.x() - haloRadius, center.y() - haloRadius, haloRadius * 2, haloRadius * 2),
            FlightPlanetArt::atmosphere(material));
        const QRectF disk(center.x() - radius, center.y() - radius, radius * 2, radius * 2);
        painter->drawImage(disk, FlightPlanetArt::surface(material));

        // The sun determines the illuminated side and phase in camera space.
        const QVector3D towardSun = (sun - planet.position).normalized();
        const QVector3D towardCamera = (m_camera - planet.position).normalized();
        const qreal phase = QVector3D::dotProduct(towardSun, towardCamera);
        const QVector3D light = cameraPoint(planet.position + towardSun * 100) - body.point;
        QPointF lightDirection(light.x(), -light.y());
        const qreal length = std::hypot(lightDirection.x(), lightDirection.y());
        if (length > 0.001)
            lightDirection /= length;
        else
            lightDirection = QPointF(-1, -0.5);
        painter->save();
        painter->translate(center);
        painter->rotate(std::atan2(lightDirection.y(), lightDirection.x()) * 180 / 3.141592653589793);
        painter->drawImage(QRectF(-radius, -radius, radius * 2, radius * 2), FlightPlanetArt::shadow(phase));
        painter->restore();
        painter->setBrush(Qt::NoBrush);
        painter->setPen(QPen(QColor(accent.red(), accent.green(), accent.blue(), 110), 0.8));
        painter->drawEllipse(disk);
    }

    // Labels remain readable at system scale and keep their anchors in the world.
    QFont labelFont = painter->font();
    const bool compact = viewport()->width() < 1000;
    labelFont.setPixelSize(compact ? 11 : 12);
    painter->setFont(labelFont);
    QVector<QRectF> labels;
    for (auto iterator = bodies.crbegin(); iterator != bodies.crend(); ++iterator) {
        const auto &body = *iterator;
        if (body.index < 0)
            continue;
        const auto &planet = m_buildings[body.index];
        const qreal labelY = body.center.y() + body.radius + 15;
        const qreal labelWidth = compact ? 108 : 150;
        const QRectF label(body.center.x() - labelWidth / 2, labelY, labelWidth, compact ? 24 : 42);
        if (label.bottom() > viewport()->height() * 0.72 || !viewport()->rect().contains(label.toAlignedRect()))
            continue;
        const bool overlaps = std::any_of(labels.cbegin(), labels.cend(), [&](const QRectF &other) {
            return other.adjusted(-6, -6, 6, 6).intersects(label);
        });
        if (overlaps)
            continue;
        labels.append(label);
        m_planetLabels.insert(planet.source.path, label);
        painter->setPen(QPen(QColor(160, 187, 196, 110), 1));
        painter->drawLine(body.center + QPointF(0, body.radius + 3), QPointF(body.center.x(), labelY - 3));
        painter->setBrush(QColor(5, 12, 20, 120));
        painter->setPen(Qt::NoPen);
        painter->drawRoundedRect(label, 5, 5);
        painter->setPen(QColor("#e0ebe9"));
        painter->drawText(QRectF(label.left() + 6, label.top() + 2, label.width() - 12, 20), Qt::AlignCenter,
            painter->fontMetrics().elidedText(planet.source.name, Qt::ElideRight, int(labelWidth - 12)));
        if (!compact) {
            painter->setPen(QColor("#87a4ad"));
            painter->drawText(label.adjusted(6, 21, -6, -2), Qt::AlignCenter,
                tr("%1 km · select to travel").arg((planet.position - m_camera).length() / 1000.0, 0, 'f', 1));
        }
    }
    painter->restore();
}
