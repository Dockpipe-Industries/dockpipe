#include "WorkspaceGraphNode.h"
#include "WorkspaceGraphView.h"
#include "CanvasIcons.h"
#include <QFileInfo>
#include <QLocale>
#include <QGraphicsScene>
#include <QGraphicsSceneMouseEvent>
#include <QGraphicsSceneContextMenuEvent>
#include <QGraphicsView>
#include <QPainter>
#include <QStyleOptionGraphicsItem>
#include <cmath>

WorkspaceGraphNode::WorkspaceGraphNode(const QString &path, const QString &kind, qint64 fileBytes)
    : m_name(QFileInfo(path).fileName()), m_kind(kind), m_fileBytes(fileBytes)
{
    if (m_name.isEmpty())
        m_name = path;
    setData(0, path);
    setFlags(ItemIsSelectable | ItemIsMovable | ItemSendsGeometryChanges);
    setCursor(Qt::OpenHandCursor);
    QString tooltip = path;
    if (m_kind == "file" && m_fileBytes >= 0) {
        tooltip += tr("\n%1 (%2 bytes)\nChip height uses a logarithmic size scale, capped for readability.")
            .arg(QLocale().formattedDataSize(m_fileBytes)).arg(QLocale().toString(m_fileBytes));
    }
    setToolTip(tooltip);
    setCacheMode(DeviceCoordinateCache);
}
QRectF WorkspaceGraphNode::boundingRect() const { return m_bounds; }
void WorkspaceGraphNode::setSummary(const QString &summary)
{
    m_summary = summary;
    if (m_kind == "file" && m_fileBytes >= 0)
        m_summary = QLocale().formattedDataSize(m_fileBytes) + QStringLiteral(" · ") + summary;
    update();
}

qreal WorkspaceGraphNode::chipHeight() const
{
    if (m_kind == "directory")
        return 85;
    if (m_kind != "file" || m_fileBytes < 0)
        return 24;
    // Logarithmic height keeps byte-sized files visible and multi-gigabyte
    // files within the same navigable city. These are visual units, not bytes.
    return qMin(180.0, 16 + 12 * std::log2(1 + static_cast<double>(m_fileBytes) / 1024));
}

void WorkspaceGraphNode::setCityView(const WorkspaceGraphView *view)
{
    prepareGeometryChange();
    m_city = view != nullptr;
    m_faces.clear();
    m_traces.clear();
    m_pins.clear();
    m_board.clear();
    m_door.clear();
    m_bounds = QRectF(-2, -2, 224, 62);
    if (!view) {
        update();
        return;
    }

    const auto project = [view](qreal x, qreal y, qreal z) {
        return view->project(QVector3D(x, y, z)) + QPointF(110, 29);
    };
    const qreal height = chipHeight();
    const QVector<QVector3D> corners{{-72, 0, -48}, {72, 0, -48}, {72, 0, 48}, {-72, 0, 48}};
    const QVector<QVector3D> normals{{0, 0, -1}, {1, 0, 0}, {0, 0, 1}, {-1, 0, 0}};
    m_board << project(-102, 0, -78) << project(102, 0, -78)
            << project(102, 0, 78) << project(-102, 0, 78);
    for (int side = 0; side < 4; ++side) {
        const auto a = corners[side];
        const auto b = corners[(side + 1) % 4];
        if (view->depth(normals[side]) <= 0)
            continue;
        QPolygonF face;
        face << project(a.x(), 0, a.z()) << project(b.x(), 0, b.z())
             << project(b.x(), height, b.z()) << project(a.x(), height, a.z());
        m_faces.append(face);
        if (m_kind == "directory" && m_door.isEmpty()) {
            const QVector3D left = a * 0.78 + b * 0.22;
            const QVector3D right = a * 0.22 + b * 0.78;
            m_door << project(left.x(), 1, left.z()) << project(right.x(), 1, right.z())
                   << project(right.x(), height * 0.78, right.z()) << project(left.x(), height * 0.78, left.z());
        }
        // Folder towers have illuminated storeys, without animated lights.
        if (m_kind == "directory") {
            for (int floor = 1; floor <= 3; ++floor)
                m_traces.append(QLineF(project(a.x(), floor * 20, a.z()), project(b.x(), floor * 20, b.z())));
        }
    }
    QPolygonF roof;
    for (const auto &corner : corners)
        roof << project(corner.x(), height, corner.z());
    m_faces.append(roof);

    // Small fixed pin count per chip keeps geometry independent of file size.
    for (int pin = -2; pin <= 2; ++pin) {
        for (int side : {-1, 1}) {
            m_pins.append(QLineF(project(pin * 24, 1, side * 50), project(pin * 24, 1, side * 72)));
            m_pins.append(QLineF(project(side * 74, 1, pin * 16), project(side * 96, 1, pin * 16)));
        }
    }
    const QPolygonF inset{project(-48, height + 1, -26), project(48, height + 1, -26),
                          project(48, height + 1, 26), project(-48, height + 1, 26)};
    for (int edge = 0; edge < 4; ++edge)
        m_traces.append(QLineF(inset[edge], inset[(edge + 1) % 4]));

    m_bounds = m_board.boundingRect();
    for (const auto &face : m_faces)
        m_bounds = m_bounds.united(face.boundingRect());
    m_labelRect = QRectF(0, m_bounds.bottom() + 8, 220, 44);
    m_bounds = m_bounds.united(m_labelRect).adjusted(-4, -4, 4, 4);
    update();
}

void WorkspaceGraphNode::paintCity(QPainter *painter, bool dark)
{
    const bool directory = m_kind == "directory";
    QColor accent(directory ? (dark ? "#52e4ce" : "#167b72") : (dark ? "#6faeff" : "#386cb3"));
    if (m_kind == "link" || m_kind == "unavailable")
        accent = QColor(dark ? "#e8b66d" : "#916019");
    painter->setPen(QPen(accent, isSelected() ? 2.4 : 1.1));
    painter->setBrush(QColor(dark ? "#11252e" : "#e4eff4"));
    painter->drawPolygon(m_board);
    const bool detailed = QStyleOptionGraphicsItem::levelOfDetailFromTransform(painter->worldTransform()) >= 0.45;
    // Pins sit on the board behind the body, including tall size-scaled chips.
    if (detailed)
        painter->drawLines(m_pins);
    for (int face = 0; face < m_faces.size(); ++face) {
        QColor fill = accent;
        if (dark)
            fill = accent.darker(face == m_faces.size() - 1 ? 260 : 430);
        else
            fill = accent.lighter(face == m_faces.size() - 1 ? 175 : 140);
        painter->setBrush(fill);
        painter->drawPolygon(m_faces[face]);
    }
    // At city-wide zoom, tiny pins and text are illegible. Keep silhouettes
    // and selection, and restore detail as the user zooms into a district.
    if (!detailed)
        return;
    painter->setPen(QPen(accent, 1.2));
    painter->drawLines(m_traces);
    if (!m_door.isEmpty()) {
        painter->setBrush(QColor(dark ? "#050e18" : "#294d5a"));
        painter->drawPolygon(m_door);
        QPolygonF shutter = m_door;
        shutter[0] += (shutter[3] - shutter[0]) * m_doorOpening;
        shutter[1] += (shutter[2] - shutter[1]) * m_doorOpening;
        painter->setBrush(accent.darker(dark ? 320 : 150));
        painter->drawPolygon(shutter);
    }
    const QPointF roofCenter = m_faces.last().boundingRect().center();
    CanvasIcons::icon(directory ? CanvasIcons::Symbol::Folder : CanvasIcons::Symbol::Code)
        .paint(painter, QRect(qRound(roofCenter.x() - 8), qRound(roofCenter.y() - 8), 16, 16));

    if (!isSelected())
        paintCityLabel(painter, m_labelRect, dark);
}

void WorkspaceGraphNode::paintPriorityLabel(QPainter *painter, const QRectF &rect)
{
    const bool dark = scene()->views().first()->palette().color(QPalette::Base).lightness() < 128;
    paintCityLabel(painter, rect, dark);
}

void WorkspaceGraphNode::paintCityLabel(QPainter *painter, const QRectF &rect, bool dark)
{
    const bool directory = m_kind == "directory";
    QColor accent(directory ? (dark ? "#52e4ce" : "#167b72") : (dark ? "#6faeff" : "#386cb3"));
    if (m_kind == "link" || m_kind == "unavailable")
        accent = QColor(dark ? "#e8b66d" : "#916019");

    painter->setPen(QPen(isSelected() ? accent : QColor(dark ? "#314755" : "#b9cddd"), 1));
    painter->setBrush(QColor(dark ? "#14202b" : "#ffffff"));
    painter->drawRoundedRect(rect, 5, 5);
    auto font = painter->font();
    font.setPointSizeF(10);
    font.setWeight(directory ? QFont::DemiBold : QFont::Normal);
    painter->setFont(font);
    painter->setPen(QColor(dark ? "#e4f3ff" : "#273b52"));
    painter->drawText(rect.adjusted(9, 3, -9, -20), Qt::AlignVCenter,
        painter->fontMetrics().elidedText(m_name, Qt::ElideMiddle, 202));
    font.setPointSizeF(8);
    font.setWeight(QFont::Normal);
    painter->setFont(font);
    painter->setPen(accent);
    painter->drawText(rect.adjusted(9, 24, -9, -3), Qt::AlignVCenter,
        painter->fontMetrics().elidedText(m_summary, Qt::ElideRight, 202));
}

void WorkspaceGraphNode::paint(QPainter *painter, const QStyleOptionGraphicsItem *, QWidget *)
{
    const bool dark = scene()->views().first()->palette().color(QPalette::Base).lightness() < 128;
    if (m_city) {
        paintCity(painter, dark);
        return;
    }
    const bool directory = m_kind == "directory";
    QColor accent(directory ? (dark ? "#79c7c5" : "#287977") : (dark ? "#9cb8e1" : "#496b9e"));
    if (m_kind == "link" || m_kind == "unavailable")
        accent = QColor(dark ? "#d8b477" : "#8b652b");
    painter->setRenderHint(QPainter::Antialiasing);
    painter->setPen(QPen(isSelected() ? accent : QColor(dark ? "#354253" : "#cfdbe9"), isSelected() ? 2 : 1));
    painter->setBrush(QColor(dark ? "#1b2531" : "#ffffff"));
    painter->drawRoundedRect(QRectF(0, 0, 220, 58), directory ? 12 : 6, directory ? 12 : 6);
    painter->setPen(Qt::NoPen);
    QColor tint = accent;
    tint.setAlpha(28);
    painter->setBrush(tint);
    painter->drawEllipse(QRectF(12, 15, 28, 28));
    CanvasIcons::icon(directory ? CanvasIcons::Symbol::Folder : CanvasIcons::Symbol::Code).paint(painter, QRect(18, 21, 16, 16));
    auto font = painter->font();
    font.setPointSizeF(10);
    font.setWeight(directory ? QFont::DemiBold : QFont::Normal);
    painter->setFont(font);
    painter->setPen(QColor(dark ? "#e3ebf5" : "#273b52"));
    painter->drawText(QRectF(50, 9, 158, 22), Qt::AlignVCenter,
        painter->fontMetrics().elidedText(m_name, Qt::ElideMiddle, 158));
    font.setPointSizeF(8);
    font.setWeight(QFont::Normal);
    painter->setFont(font);
    painter->setPen(accent);
    painter->drawText(QRectF(50, 32, 158, 18), Qt::AlignVCenter,
        painter->fontMetrics().elidedText(m_summary, Qt::ElideRight, 158));
}
void WorkspaceGraphNode::mouseDoubleClickEvent(QGraphicsSceneMouseEvent *event)
{
    if (event->button() == Qt::LeftButton && activated)
        activated();
    event->accept();
}
void WorkspaceGraphNode::contextMenuEvent(QGraphicsSceneContextMenuEvent *event)
{
    if (menuRequested)
        menuRequested(event->screenPos());
    event->accept();
}
QVariant WorkspaceGraphNode::itemChange(GraphicsItemChange change, const QVariant &value)
{
    if (change == ItemPositionHasChanged && moved)
        moved();
    return QGraphicsObject::itemChange(change, value);
}
