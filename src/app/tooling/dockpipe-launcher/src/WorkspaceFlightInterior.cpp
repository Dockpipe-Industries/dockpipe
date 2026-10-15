#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include "FlightAppBillboards.h"
#include <QPainter>

void WorkspaceFlightView::setInterior(const SpatialInterior &room, const QString &name)
{
    setCockpitWorld({}, false);
    m_interior = room;
    m_interiorName = name;
    m_camera = QVector3D(room.spawn.x(), 160, room.spawn.y());
    m_pitch = 0;
    m_yaw = 0;
    viewport()->update();
}

void WorkspaceFlightView::paintInterior(QPainter *painter)
{
    const auto &room = *m_interior;
    const float x = room.size.width() / 2;
    const float z = room.size.height() / 2;
    const float ceiling = room.height;
    painter->fillRect(viewport()->rect(), QColor("#101820"));
    const auto face = [&](const QVector<QVector3D> &vertices, const QColor &color) {
        const auto polygon = projectPolygon(vertices);
        if (polygon.isEmpty())
            return;
        painter->setPen(QPen(QColor("#416274"), 1));
        painter->setBrush(color);
        painter->drawPolygon(polygon);
    };
    face({{-x, 0, -z}, {x, 0, -z}, {x, 0, z}, {-x, 0, z}}, QColor("#172a34"));
    face({{-x, ceiling, -z}, {x, ceiling, -z}, {x, ceiling, z}, {-x, ceiling, z}}, QColor("#111d27"));
    face({{-x, 0, z}, {x, 0, z}, {x, ceiling, z}, {-x, ceiling, z}}, QColor("#233b4b"));
    face({{-x, 0, -z}, {x, 0, -z}, {x, ceiling, -z}, {-x, ceiling, -z}}, QColor("#1a303e"));
    face({{-x, 0, -z}, {-x, 0, z}, {-x, ceiling, z}, {-x, ceiling, -z}}, QColor("#203747"));
    face({{x, 0, -z}, {x, 0, z}, {x, ceiling, z}, {x, ceiling, -z}}, QColor("#294456"));
    painter->setPen(QPen(QColor("#305363"), 1));
    for (int line = -int(x); line <= x; line += 100)
        drawWorldLine(painter, {float(line), 1, -z}, {float(line), 1, z});
    for (int line = -int(z); line <= z; line += 100)
        drawWorldLine(painter, {-x, 1, float(line)}, {x, 1, float(line)});
    painter->setPen(QPen(QColor("#8ddbd5"), 3));
    drawWorldLine(painter, {-x, ceiling - 30, z - 1}, {x, ceiling - 30, z - 1});
    drawWorldLine(painter, {-x + 1, ceiling - 30, -z}, {-x + 1, ceiling - 30, z});
    drawWorldLine(painter, {x - 1, ceiling - 30, -z}, {x - 1, ceiling - 30, z});
    QSet<int> visibleStreams;
    for (const auto &display : room.displays) {
        const auto polygon = projectPolygon(SpatialInteriorGeometry::surface(room, display));
        if (polygon.isEmpty())
            continue;
        const int stream = m_interiorBindings.value(display.id, -1);
        const FlightAppFrame *frame = nullptr;
        if (m_billboards && stream >= 0 && stream < m_billboards->frames().size()) {
            frame = &m_billboards->frames()[stream];
            visibleStreams.insert(stream);
        }
        painter->setBrush(QColor("#0b131d"));
        painter->setPen(QPen(QColor("#70c6d3"), 2));
        painter->drawPolygon(polygon);
        painter->save();
        painter->setClipRegion(QRegion(polygon.toPolygon()), Qt::IntersectClip);
        if (frame && !frame->image.isNull()) {
            QPolygonF projected;
            bool inFront = true;
            for (const auto &corner : SpatialInteriorGeometry::surface(room, display)) {
                const auto point = cameraPoint(corner);
                if (point.z() < 8) {
                    inFront = false;
                    break;
                }
                projected.append(screenPoint(point));
            }
            QTransform transform;
            const QPolygonF source{{0, 0}, {qreal(frame->image.width()), 0},
                {qreal(frame->image.width()), qreal(frame->image.height())}, {0, qreal(frame->image.height())}};
            if (inFront && QTransform::quadToQuad(source, projected, transform)) {
                painter->save();
                painter->setTransform(transform);
                painter->drawImage(QPointF(0, 0), frame->image);
                painter->restore();
            }
        }
        painter->setPen(QColor("#d5e9f0"));
        if (!frame || frame->image.isNull())
            painter->drawText(polygon.boundingRect().adjusted(8, 8, -8, -8), Qt::AlignCenter | Qt::TextWordWrap,
                display.application.name + "\n" + (frame ? frame->status : tr("Select or press E to connect")));
        painter->restore();
    }
    if (m_billboards)
        m_billboards->setVisibleStreams(visibleStreams);
    paintReturnPortal(painter);
    const QPoint center = viewport()->rect().center();
    painter->setPen(QPen(QColor("#bef7eb"), 1));
    painter->drawLine(center + QPoint(-6, 0), center + QPoint(6, 0));
    painter->drawLine(center + QPoint(0, -6), center + QPoint(0, 6));
    painter->setPen(Qt::white);
    painter->drawText(QRect(20, 65, viewport()->width() - 40, 32), Qt::AlignCenter, m_interiorName);
    painter->drawText(QRect(20, viewport()->height() - 64, viewport()->width() - 40, 50), Qt::AlignCenter | Qt::TextWordWrap,
        tr("WALK · WASD / left stick · Right-drag / right stick look · Shift boost · E / A interact · Esc / B exit\n%1").arg(m_worldStatus));
}

bool WorkspaceFlightView::connectInteriorDisplay(const QString &id, const DesktopAppChoice &choice, bool control)
{
    if (!m_interior || !choice.id || choice.applicationClass.isEmpty())
        return false;
    const SpatialDisplay *selected = nullptr;
    for (const auto &display : m_interior->displays) {
        if (display.id == id)
            selected = &display;
    }
    if (!selected || selected->application.windowClass.compare(choice.applicationClass, Qt::CaseInsensitive) != 0)
        return false;
    int stream = -1;
    for (int index = 0; index < m_interiorStreams.size(); ++index) {
        if (m_interiorStreams[index].id == choice.id)
            stream = index;
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
    }
    // References to the same catalogue identity share this explicitly selected stream.
    for (const auto &display : m_interior->displays) {
        if (display.application.desktopId == selected->application.desktopId
            && display.application.windowClass == selected->application.windowClass)
            m_interiorBindings.insert(display.id, stream);
    }
    viewport()->update();
    if (control)
        m_billboards->control(stream);
    return true;
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
