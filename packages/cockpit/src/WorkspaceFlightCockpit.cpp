#include "WorkspaceFlightView.h"
#include "FlightSolarSystem.h"
#include "FlightProjection.h"
#include <QFontDatabase>
#include <QLinearGradient>
#include <QPainter>
#include <QPainterPath>
#include <QRadialGradient>
#include <algorithm>

void WorkspaceFlightView::paintCockpit(QPainter *painter)
{
    const qreal width = viewport()->width();
    const qreal height = viewport()->height();
    const qreal focal = viewFocalLength();
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
        faces.reserve(128);
        const QQuaternion hull = craftOrientation();
        // Geometry follows the hull; the independent head-look still reveals
        // the consoles and structural canopy, rather than a screen-space border.
        const auto world = [&](const QVector3D &point) {
            return m_camera + hull.rotatedVector(point);
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
        const float deckY = -height * 0.22 * 175 / focal;
        const float canopyY = height * 0.44 * 175 / focal;
        face({{-125, deckY, 185}, {125, deckY, 185}, {160, deckY - 60, 65}, {-160, deckY - 60, 65}}, QColor("#111c25"));
        face({{-125, deckY, 185}, {125, deckY, 185}, {125, deckY - 6, 189}, {-125, deckY - 6, 189}}, QColor("#3a4b54"));
        face({{-115, deckY - 2, 179}, {115, deckY - 2, 179}, {140, deckY - 48, 85}, {-140, deckY - 48, 85}}, QColor("#090f17"));
        face({{-150, deckY - 70, -100}, {150, deckY - 70, -100}, {160, deckY - 60, 65}, {-160, deckY - 60, 65}}, QColor("#070c13"));
        for (int side : {-1, 1}) {
            const float sign = side;
            // Tapered canopy shoulders frame the view and lead into the console.
            beam({sign * 118, deckY - 20, 160}, {sign * 87, canopyY * 0.56f, 175}, 5, 7);
            beam({sign * 87, canopyY * 0.56f, 175}, {sign * 43, canopyY, 168}, 6, 7);
            beam({sign * 119, deckY - 18, 162}, {sign * 97, deckY + 10, 182}, 6, 6);
            face({{sign * 86, canopyY * 0.56f, 167}, {sign * 84, canopyY * 0.56f, 167},
                {sign * 40, canopyY - 1, 160}, {sign * 42, canopyY - 1, 160}}, QColor("#48b7dc"));
            face({{sign * 117, deckY - 20, 152}, {sign * 115, deckY - 20, 152},
                {sign * 84, canopyY * 0.56f, 167}, {sign * 86, canopyY * 0.56f, 167}}, QColor("#304b62"));
            // Recessed console ribs and physical switches catch the canopy light.
            for (int rib = 0; rib < 6; ++rib) {
                const float z = 140 - rib * 9;
                face({{sign * 108, deckY - 13, z}, {sign * 125, deckY - 18, z},
                    {sign * 125, deckY - 19, z - 3}, {sign * 108, deckY - 14, z - 3}},
                    rib == 1 ? QColor("#ebac61") : QColor("#3d5665"));
            }
            beam({sign * 130, deckY - 20, -70}, {sign * 110, 100, -70}, 6, 6);
            face({{sign * 115, deckY - 6, 165}, {sign * 145, deckY - 18, 95},
                {sign * 145, deckY - 65, -100}, {sign * 110, deckY - 65, -100}}, QColor("#1d2a33"));
            face({{sign * 115, deckY - 4, 163}, {sign * 118, deckY - 5, 163},
                {sign * 140, deckY - 20, 94}, {sign * 137, deckY - 19, 94}}, QColor("#689c9d"));
        }
        beam({-43, canopyY, 168}, {43, canopyY, 168}, 5, 7);
        face({{-38, canopyY - 4, 160}, {38, canopyY - 4, 160},
            {32, canopyY - 9, 156}, {-32, canopyY - 9, 156}}, QColor("#080e16"));
        face({{-20, canopyY - 5, 155}, {20, canopyY - 5, 155},
            {20, canopyY - 6, 155}, {-20, canopyY - 6, 155}}, QColor("#68b8cf"));
        // Raised central instrument hood, with beveled amber trim.
        face({{-39, deckY + 5, 177}, {39, deckY + 5, 177},
            {42, deckY - 2, 164}, {-42, deckY - 2, 164}}, QColor("#354651"));
        face({{-40, deckY - 2, 163}, {40, deckY - 2, 163},
            {40, deckY - 3, 163}, {-40, deckY - 3, 163}}, QColor("#c98b48"));
        std::sort(faces.begin(), faces.end(), [](const CabinFace &a, const CabinFace &b) { return a.depth > b.depth; });
        for (const auto &surface : faces) {
            painter->setBrush(surface.color);
            painter->setPen(QPen(surface.color.lighter(125), 0.8));
            painter->drawConvexPolygon(surface.polygon);
        }

        // Three physical display surfaces. Content is drawn directly through a
        // projective transform; no per-frame texture allocation or app capture.
        for (int instrument = 0; instrument < 3; ++instrument) {
            const float centerX = (instrument - 1) * 76;
            const float halfWidth = instrument == 1 ? 37 : 29;
            const float panelHeight = height * 0.19 * 174 / focal;
            const float leftDepth = instrument == 0 ? 190 : 174;
            const float rightDepth = instrument == 2 ? 190 : 174;
            const QVector<QVector3D> local{{centerX - halfWidth, deckY - 2, leftDepth}, {centerX + halfWidth, deckY - 2, rightDepth},
                {centerX + halfWidth, deckY - 2 - panelHeight, rightDepth}, {centerX - halfWidth, deckY - 2 - panelHeight, leftDepth}};
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
                instrument == 0 ? tr("PROPULSION") : instrument == 1 ? tr("LOCAL SPACE") : tr("FLIGHT ASSIST"));
            const bool explorer = flightMode() == ArcadeFlightDynamics::Mode::Explorer;
            const QColor amber("#f8b55e");
            if (instrument == 1) {
                const QPointF radar(160, 83);
                QRadialGradient radarGlow(radar, 75);
                radarGlow.setColorAt(0, QColor(221, 129, 42, 45));
                radarGlow.setColorAt(0.65, QColor(221, 129, 42, 18));
                radarGlow.setColorAt(1, QColor(221, 129, 42, 0));
                painter->setPen(Qt::NoPen);
                painter->setBrush(radarGlow);
                painter->drawEllipse(radar, 85, 60);
                painter->setBrush(QColor(74, 39, 14, 90));
                painter->setPen(QPen(QColor("#77552f"), 1));
                painter->drawEllipse(radar, 62, 47);
                painter->setBrush(Qt::NoBrush);
                painter->drawEllipse(radar, 40, 30);
                painter->drawEllipse(radar, 20, 15);
                painter->drawLine(QPointF(91, 83), QPointF(229, 83));
                painter->drawLine(QPointF(160, 32), QPointF(160, 132));
                const qreal range = m_orbit ? FlightSolarSystem::radius(qMax(6, int(m_buildings.size()) - 1)) * 3 : m_extent;
                const QQuaternion inverseHull = hull.conjugated();
                painter->setPen(QPen(amber, 1));
                painter->setBrush(amber);
                for (const auto &building : m_buildings) {
                    const QVector3D offset = inverseHull.rotatedVector(building.position - m_camera);
                    const QPointF point(offset.x() / range * 58, -offset.z() / range * 44);
                    if (std::hypot(point.x() / 62, point.y() / 47) <= 1) {
                        const QPointF marker = radar + point;
                        painter->drawLine(marker, marker - QPointF(0, qBound(-12.0f, offset.y() / float(range) * 35, 12.0f)));
                        painter->drawEllipse(marker, 2, 2);
                    }
                }
                painter->setPen(QPen(QColor("#ffe2b4"), 2));
                painter->drawPolyline(QPolygonF{radar + QPointF(-5, 5), radar + QPointF(0, -5), radar + QPointF(5, 5)});
                font.setPixelSize(12);
                painter->setFont(font);
                painter->setPen(amber);
                painter->drawText(QRectF(12, 127, 296, 18), Qt::AlignCenter,
                    travelling() ? tr("AUTOPILOT") : tr("%1 CONTACTS").arg(m_buildings.size()));
            } else if (instrument == 0) {
                painter->setPen(QColor("#e1ece9"));
                font.setPixelSize(34);
                painter->setFont(font);
                painter->drawText(QRectF(16, 38, 288, 44), QString::number(qRound(m_dynamics.velocity().length())));
                font.setPixelSize(12);
                painter->setFont(font);
                painter->setPen(light);
                painter->drawText(QRectF(16, 83, 288, 20), tr("SPEED / WORLD UNITS PER SECOND"));
                const qreal maximum = (explorer ? 260 : 540) * (m_orbit ? 3 : 1) * 2.5;
                const qreal throttle = qBound(0.0, m_dynamics.velocity().length() / maximum, 1.0);
                painter->fillRect(QRectF(16, 112, 288, 5), QColor("#253c49"));
                painter->fillRect(QRectF(16, 112, 288 * throttle, 5), m_pilotBoost || m_keys.contains(Qt::Key_Shift) ? amber : light);
                painter->drawText(QRectF(16, 125, 288, 18), m_keys.contains(Qt::Key_X) || m_pilotBrake ? tr("AIRBRAKE") : tr("SHIFT BOOST   /   X BRAKE"));
            } else {
                painter->setPen(explorer ? light : amber);
                font.setPixelSize(29);
                painter->setFont(font);
                painter->drawText(QRectF(16, 39, 288, 40), explorer ? tr("EXPLORER") : tr("PILOT"));
                font.setPixelSize(12);
                painter->setFont(font);
                painter->setPen(QColor("#a5b9c4"));
                QString assistance = tr("FREE ROTATION / MOMENTUM");
                if (explorer)
                    assistance = m_orbit ? tr("ATTITUDE HOLD / HOVER ASSIST") : tr("AUTO LEVEL / HOVER ASSIST");
                painter->drawText(QRectF(16, 83, 288, 20), assistance);
                painter->drawText(QRectF(16, 108, 288, 20), tr("HDG %1°    ALT %2")
                    .arg((qRound(m_yaw * 180 / 3.141592653589793) + 360) % 360).arg(qRound(m_camera.y())));
                painter->drawText(QRectF(16, 128, 288, 18), explorer && !m_orbit ? tr("F SWITCH MODE") : tr("Z / C ROLL   ·   F SWITCH MODE"));
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
        painter->drawText(QRectF(width * 0.32, height * 0.13, width * 0.36, 20), Qt::AlignCenter, tr("D O C K P I P E  /  FLIGHT DECK"));
    font.setLetterSpacing(QFont::AbsoluteSpacing, 0);
    font.setPixelSize(12);
    painter->setFont(font);
    painter->setPen(QColor("#97b3bc"));
    const QString hint = inputHints();
    painter->drawText(QRectF(24, height * 0.17, width - 48, 24), Qt::AlignCenter,
        painter->fontMetrics().elidedText(hint, Qt::ElideRight, int(width - 48)));
    if (!m_worldStatus.isEmpty()) {
        painter->setPen(light);
        painter->drawText(QRectF(24, height * 0.21, width - 48, 24), Qt::AlignCenter,
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
