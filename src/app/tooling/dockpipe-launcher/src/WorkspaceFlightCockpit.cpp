#include "WorkspaceFlightView.h"
#include "FlightSolarSystem.h"
#include "FlightProjection.h"
#include <QFontDatabase>
#include <QLinearGradient>
#include <QPainter>
#include <QPainterPath>
#include <algorithm>

void WorkspaceFlightView::paintCockpit(QPainter *painter)
{
    const qreal width = viewport()->width();
    const qreal height = viewport()->height();
    const qreal focal = FlightProjection::focalLength(viewport()->size());
    painter->save();
    painter->setRenderHint(QPainter::Antialiasing);
    const QColor light("#a5d5ce");
    if (m_shipVisible) {
        struct CabinFace {
            QPolygonF polygon;
            QColor color;
            qreal depth;
        };
        QVector<CabinFace> faces;
        faces.reserve(96);
        // Geometry follows the hull; the independent head-look still reveals
        // the consoles and structural canopy, rather than a screen-space border.
        const auto world = [&](const QVector3D &point) {
            const qreal vertical = point.y() * std::cos(m_pitch) + point.z() * std::sin(m_pitch);
            const qreal forward = point.z() * std::cos(m_pitch) - point.y() * std::sin(m_pitch);
            return m_camera + QVector3D(point.x() * std::cos(m_yaw) + forward * std::sin(m_yaw),
                vertical, forward * std::cos(m_yaw) - point.x() * std::sin(m_yaw));
        };
        const auto face = [&](const QVector<QVector3D> &local, const QColor &color) {
            QVector<QVector3D> vertices;
            QVector3D center;
            for (const auto &point : local) {
                vertices.append(world(point));
                center += vertices.last();
            }
            const auto polygon = projectPolygon(vertices);
            if (polygon.size() >= 3)
                faces.append({polygon, color, cameraPoint(center / local.size()).z()});
        };
        const auto beam = [&](QVector3D start, QVector3D end, float halfWidth, float halfDepth) {
            const QVector3D direction = (end - start).normalized();
            const QVector3D right = QVector3D::crossProduct(direction, {0, 0, 1}).normalized() * halfWidth;
            const QVector3D depth(0, 0, halfDepth);
            face({start - right - depth, start + right - depth, end + right - depth, end - right - depth}, QColor("#182631"));
            face({start - right + depth, start + right + depth, end + right + depth, end - right + depth}, QColor("#293944"));
            face({start - right - depth, start - right + depth, end - right + depth, end - right - depth}, QColor("#0a111a"));
            face({start + right - depth, start + right + depth, end + right + depth, end + right - depth}, QColor("#34434e"));
        };
        const float deckY = -height * 0.25 * 175 / focal;
        face({{-125, deckY, 185}, {125, deckY, 185}, {160, deckY - 60, 65}, {-160, deckY - 60, 65}}, QColor("#111c25"));
        face({{-125, deckY, 185}, {125, deckY, 185}, {125, deckY - 6, 189}, {-125, deckY - 6, 189}}, QColor("#3a4b54"));
        face({{-115, deckY - 2, 179}, {115, deckY - 2, 179}, {140, deckY - 48, 85}, {-140, deckY - 48, 85}}, QColor("#090f17"));
        face({{-150, deckY - 70, -100}, {150, deckY - 70, -100}, {160, deckY - 60, 65}, {-160, deckY - 60, 65}}, QColor("#070c13"));
        for (int side : {-1, 1}) {
            const float sign = side;
            beam({sign * 123, deckY - 20, 160}, {sign * 101, 105, 175}, 4, 5);
            beam({sign * 101, 105, 175}, {sign * 45, 129, 168}, 4, 5);
            beam({sign * 130, deckY - 20, -70}, {sign * 110, 100, -70}, 6, 6);
            face({{sign * 115, deckY - 6, 165}, {sign * 145, deckY - 18, 95},
                {sign * 145, deckY - 65, -100}, {sign * 110, deckY - 65, -100}}, QColor("#1d2a33"));
            face({{sign * 115, deckY - 4, 163}, {sign * 118, deckY - 5, 163},
                {sign * 140, deckY - 20, 94}, {sign * 137, deckY - 19, 94}}, QColor("#689c9d"));
        }
        beam({-45, 129, 168}, {45, 129, 168}, 4, 5);
        std::sort(faces.begin(), faces.end(), [](const CabinFace &a, const CabinFace &b) { return a.depth > b.depth; });
        for (const auto &surface : faces) {
            painter->setBrush(surface.color);
            painter->setPen(QPen(surface.color.lighter(125), 0.8));
            painter->drawConvexPolygon(surface.polygon);
        }

        // Three physical display surfaces. Content is drawn directly through a
        // projective transform; no per-frame texture allocation or app capture.
        for (int instrument = 0; instrument < 3; ++instrument) {
            const float centerX = (instrument - 1) * 64;
            const float panelHeight = height * 0.12 * 174 / focal;
            const float leftDepth = instrument == 0 ? 190 : 174;
            const float rightDepth = instrument == 2 ? 190 : 174;
            const QVector<QVector3D> local{{centerX - 29, deckY - 2, leftDepth}, {centerX + 29, deckY - 2, rightDepth},
                {centerX + 29, deckY - 2 - panelHeight, rightDepth}, {centerX - 29, deckY - 2 - panelHeight, leftDepth}};
            QPolygonF projection;
            bool visible = true;
            for (const auto &point : local) {
                const QVector3D camera = cameraPoint(world(point));
                if (camera.z() < FlightProjection::nearPlane) {
                    visible = false;
                    break;
                }
                projection.append(screenPoint(camera));
            }
            if (!visible || !viewport()->rect().intersects(projection.boundingRect().toAlignedRect()))
                continue;
            QTransform mapping;
            const QPolygonF source{QPointF(0, 0), QPointF(320, 0), QPointF(320, 150), QPointF(0, 150)};
            if (!QTransform::quadToQuad(source, projection, mapping))
                continue;
            painter->save();
            QPainterPath clip;
            clip.addPolygon(projection);
            painter->setClipPath(clip, Qt::IntersectClip);
            painter->setTransform(mapping);
            QLinearGradient glass(0, 0, 0, 150);
            glass.setColorAt(0, QColor("#101f2b"));
            glass.setColorAt(1, QColor("#050b12"));
            painter->setBrush(glass);
            painter->setPen(QPen(QColor("#44535c"), 5));
            painter->drawRoundedRect(QRectF(0, 0, 320, 150), 7, 7);
            painter->setPen(QPen(light, 2));
            painter->drawLine(14, 1, 62, 1);
            QFont font = QFontDatabase::systemFont(QFontDatabase::FixedFont);
            font.setPixelSize(15);
            painter->setFont(font);
            painter->setPen(QColor("#72929e"));
            painter->drawText(QRectF(16, 10, 288, 22), Qt::AlignLeft,
                instrument == 0 ? tr("LOCAL RADAR") : instrument == 1 ? tr("NAVIGATION") : tr("FLIGHT VECTOR"));
            if (instrument == 0) {
                const QPointF radar(86, 88);
                painter->setBrush(Qt::NoBrush);
                painter->setPen(QPen(QColor("#29444d"), 1));
                painter->drawEllipse(radar, 42, 42);
                painter->drawEllipse(radar, 24, 24);
                painter->drawLine(QPointF(40, 88), QPointF(132, 88));
                painter->drawLine(QPointF(86, 42), QPointF(86, 134));
                const QVector3D origin = m_orbit ? FlightSolarSystem::sun() : m_camera;
                const qreal range = m_orbit ? FlightSolarSystem::radius(qMax(6, int(m_buildings.size()) - 1)) : m_extent;
                painter->setPen(Qt::NoPen);
                painter->setBrush(light);
                for (const auto &building : m_buildings) {
                    const QVector3D offset = building.position - origin;
                    const QPointF point(offset.x() / range * 40, offset.z() / range * 40);
                    if (std::hypot(point.x(), point.y()) <= 42)
                        painter->drawEllipse(radar + point, 2, 2);
                }
                painter->setBrush(QColor("#e5b67c"));
                painter->drawEllipse(radar, 3, 3);
                painter->setPen(light);
                font.setPixelSize(28);
                painter->setFont(font);
                painter->drawText(QRectF(155, 51, 150, 38), QString::number(m_buildings.size()));
                font.setPixelSize(13);
                painter->setFont(font);
                painter->setPen(QColor("#8aa2aa"));
                painter->drawText(QRectF(155, 93, 150, 30), m_orbit ? tr("DESTINATIONS") : tr("STRUCTURES"));
            } else if (instrument == 1) {
                painter->setPen(QColor("#e1ece9"));
                font.setPixelSize(29);
                painter->setFont(font);
                painter->drawText(QRectF(16, 45, 288, 43), m_orbit ? tr("SOL SYSTEM") : tr("SURFACE"));
                font.setPixelSize(14);
                painter->setFont(font);
                painter->setPen(light);
                QString state = tr("HOLDING POSITION");
                if (travelling())
                    state = tr("AUTOPILOT ENGAGED");
                else if (m_tick.isActive())
                    state = tr("MANUAL FLIGHT");
                else if (m_orbit && m_orbitMotion)
                    state = tr("ORBITAL MOTION ACTIVE");
                painter->drawText(QRectF(16, 105, 288, 22), state);
            } else {
                painter->setPen(QColor("#e1ece9"));
                font.setPixelSize(28);
                painter->setFont(font);
                painter->drawText(QRectF(16, 45, 288, 38), tr("%1°  /  %2°")
                    .arg((qRound(m_yaw * 180 / 3.141592653589793) + 360) % 360)
                    .arg(qRound(m_pitch * 180 / 3.141592653589793)));
                font.setPixelSize(14);
                painter->setFont(font);
                painter->setPen(QColor("#8aa2aa"));
                painter->drawText(QRectF(16, 103, 288, 22), tr("HEADING / PITCH    ALT %1").arg(qRound(m_camera.y())));
            }
            painter->restore();
        }
    }

    // Sparse flight symbology preserves the central field of view.
    const QPointF center(width / 2, height / 2);
    painter->setPen(QPen(QColor(167, 220, 211, 180), 1));
    painter->setBrush(Qt::NoBrush);
    painter->drawEllipse(center, 3, 3);
    painter->drawLine(center + QPointF(-18, 0), center + QPointF(-8, 0));
    painter->drawLine(center + QPointF(8, 0), center + QPointF(18, 0));
    QFont font = painter->font();
    font.setPixelSize(11);
    font.setLetterSpacing(QFont::AbsoluteSpacing, 2.4);
    painter->setFont(font);
    painter->setPen(QColor("#a9bdbf"));
    if (width >= 1150)
        painter->drawText(QRectF(width * 0.32, 24, width * 0.36, 20), Qt::AlignCenter, tr("D O C K P I P E  /  FLIGHT DECK"));
    font.setLetterSpacing(QFont::AbsoluteSpacing, 0);
    font.setPixelSize(12);
    painter->setFont(font);
    painter->setPen(QColor("#97b3bc"));
    const QString hint = m_orbit ? tr("Select a destination to engage autopilot · WASD to fly · Right-drag to steer")
        : tr("Double-click a folder to fly inside · Select files to preview · Backspace to return");
    painter->drawText(QRectF(24, 65, width - 48, 24), Qt::AlignCenter,
        painter->fontMetrics().elidedText(hint, Qt::ElideRight, int(width - 48)));
    if (!m_worldStatus.isEmpty()) {
        painter->setPen(light);
        painter->drawText(QRectF(24, 93, width - 48, 24), Qt::AlignCenter,
            painter->fontMetrics().elidedText(m_worldStatus, Qt::ElideRight, int(width - 48)));
    }
    if (travelling()) {
        const qreal duration = m_travelDirectory.isEmpty() ? 1400.0 : 2400.0;
        const qreal progress = qBound(0.0, m_travelElapsed.elapsed() / duration, 1.0);
        const QRectF status(width / 2 - 150, height * 0.19, 300, 46);
        painter->setBrush(QColor(5, 14, 22, 230));
        painter->setPen(QPen(QColor("#608d91"), 1));
        painter->drawRoundedRect(status, 6, 6);
        painter->setPen(light);
        painter->drawText(status.adjusted(12, 3, -12, -8), Qt::AlignCenter, tr("AUTOPILOT · SPACE TO ARRIVE"));
        painter->fillRect(QRectF(status.left() + 10, status.bottom() - 7, (status.width() - 20) * progress, 2), light);
    }
    painter->restore();
}
