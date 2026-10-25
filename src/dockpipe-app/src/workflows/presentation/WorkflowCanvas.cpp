#include "WorkflowCanvas.h"
#include "CanvasIcons.h"
#include <QFontDatabase>
#include <QGraphicsSceneHoverEvent>

#include <QGraphicsObject>
#include <QGraphicsPathItem>
#include <QGraphicsScene>
#include <QPainter>
#include <QPainterPath>
#include <QWheelEvent>
#include <QSignalBlocker>
#include <cmath>
#include <functional>

namespace {
constexpr qreal cardWidth = 248;
constexpr qreal cardHeight = 126;

class StepCard final : public QGraphicsObject {
public:
    StepCard(const QJsonObject &step, const QJsonArray &path,
             qreal height, QGraphicsItem *parent = nullptr)
        : QGraphicsObject(parent), m_step(step), m_height(height)
    {
        setData(0, path);
        setAcceptHoverEvents(true);
        setFlag(ItemIsSelectable);
        setFlag(ItemIsMovable, parent == nullptr);
        setFlag(ItemSendsGeometryChanges);
        setCursor(parent ? Qt::PointingHandCursor : Qt::OpenHandCursor);
        setToolTip(step.value("cmd").toString(step.value("workflow").toString()));
        setZValue(1);
    }

    QRectF boundingRect() const override { return QRectF(-5, -5, cardWidth + 10, m_height + 12); }
    std::function<void()> moved;

protected:
    void hoverEnterEvent(QGraphicsSceneHoverEvent *event) override
    {
        m_hovered = true;
        update();
        QGraphicsObject::hoverEnterEvent(event);
    }

    void hoverLeaveEvent(QGraphicsSceneHoverEvent *event) override
    {
        m_hovered = false;
        update();
        QGraphicsObject::hoverLeaveEvent(event);
    }

    QVariant itemChange(GraphicsItemChange change, const QVariant &value) override
    {
        if (change == ItemPositionHasChanged && moved)
            moved();
        return QGraphicsObject::itemChange(change, value);
    }

    void paint(QPainter *painter, const QStyleOptionGraphicsItem *, QWidget *widget) override
    {
        const bool dark = !widget || widget->palette().color(QPalette::Window).lightness() < 128;
        const bool parallel = m_step.contains("group");
        const bool packaged = m_step.contains("workflow");
        QColor accent(dark ? "#80d9bd" : "#25785f");
        if (parallel)
            accent = QColor(dark ? "#b39aff" : "#7352af");
        else if (packaged)
            accent = QColor(dark ? "#f5c785" : "#956215");
        painter->setRenderHint(QPainter::Antialiasing);
        const QColor border(dark ? "#343e4c" : "#d0d8e3");
        const QColor surface(dark ? "#1b222c" : "#ffffff");
        painter->setPen(Qt::NoPen);
        painter->setBrush(QColor(0, 0, 0, dark ? 40 : 12));
        painter->drawRoundedRect(QRectF(0, 4, cardWidth, m_height), 10, 10);
        if (isSelected()) {
            QColor halo = accent;
            halo.setAlpha(30);
            painter->setPen(QPen(halo, 7));
            painter->setBrush(Qt::NoBrush);
            painter->drawRoundedRect(QRectF(0, 0, cardWidth, m_height), 10, 10);
        }
        QColor outline = border;
        if (isSelected())
            outline = accent;
        else if (m_hovered)
            outline = border.lighter(145);
        painter->setPen(QPen(outline, 1));
        painter->setBrush(surface);
        painter->drawRoundedRect(QRectF(0, 0, cardWidth, m_height), 10, 10);

        CanvasIcons::Symbol symbol = CanvasIcons::Symbol::Monitor;
        QString kind = tr("Container command");
        if (parallel) {
            symbol = CanvasIcons::Symbol::Parallel;
            kind = tr("Parallel group");
        } else if (packaged) {
            symbol = CanvasIcons::Symbol::Workflow;
            kind = tr("Package workflow");
        } else if (m_step.value("kind").toString() == "host") {
            symbol = CanvasIcons::Symbol::Terminal;
            kind = tr("Host command");
        }
        QColor badge = accent;
        badge.setAlpha(dark ? 18 : 12);
        painter->setPen(Qt::NoPen);
        painter->setBrush(badge);
        painter->drawRoundedRect(QRectF(16, 16, 30, 30), 7, 7);
        CanvasIcons::icon(symbol).paint(painter, QRect(22, 22, 18, 18));

        QFont title = painter->font();
        title.setPointSizeF(10.5);
        title.setWeight(QFont::DemiBold);
        painter->setFont(title);
        painter->setPen(QColor(dark ? "#e8edf4" : "#243449"));
        const QString name = parallel ? tr("Run together") : m_step.value("id").toString(tr("Untitled step"));
        painter->drawText(QRectF(57, 15, 166, 24), Qt::AlignVCenter,
            painter->fontMetrics().elidedText(name, Qt::ElideRight, 166));
        QFont small = title;
        small.setPointSize(9);
        small.setWeight(QFont::Normal);
        painter->setFont(small);
        painter->setPen(QColor(dark ? "#8f9daf" : "#6a778b"));
        painter->drawText(QRectF(57, 39, 172, 20), Qt::AlignVCenter, kind);
        painter->setPen(border);
        painter->drawLine(QPointF(16, 74), QPointF(cardWidth - 16, 74));
        QFont code = QFontDatabase::systemFont(QFontDatabase::FixedFont);
        code.setPointSizeF(8.5);
        painter->setFont(code);
        painter->setPen(QColor(dark ? "#a8b5c6" : "#586b82"));
        QString detail = m_step.value("cmd").toString(m_step.value("command").toString());
        if (packaged)
            detail = m_step.value("package").toString() + " / " + m_step.value("workflow").toString();
        if (parallel)
            detail = tr("Wait for all tasks");
        detail.replace('\n', ' ');
        painter->drawText(QRectF(16, 87, 214, 22), Qt::AlignVCenter,
            painter->fontMetrics().elidedText(detail, Qt::ElideRight, 214));
        painter->setBrush(surface);
        painter->setPen(QPen(accent, 1.5));
        painter->drawEllipse(QPointF(0, 60), 3.5, 3.5);
        painter->drawEllipse(QPointF(cardWidth, 60), 3.5, 3.5);
    }

private:
    bool m_hovered = false;
    QJsonObject m_step;
    qreal m_height;
};
}

WorkflowCanvas::WorkflowCanvas(QWidget *parent) : QGraphicsView(parent)
{
    setScene(new QGraphicsScene(this));
    setRenderHints(QPainter::Antialiasing | QPainter::TextAntialiasing);
    setFrameShape(QFrame::NoFrame);
    setDragMode(ScrollHandDrag);
    setHorizontalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    setVerticalScrollBarPolicy(Qt::ScrollBarAlwaysOff);
    setTransformationAnchor(AnchorUnderMouse);
    setResizeAnchor(AnchorViewCenter);
    setBackgroundBrush(QColor("#11151c"));
    connect(scene(), &QGraphicsScene::selectionChanged, this, [this]() {
        const auto selected = scene()->selectedItems();
        emit nodeSelected(selected.isEmpty() ? QJsonArray() : selected.first()->data(0).toJsonArray());
    });
}

void WorkflowCanvas::setDocument(const QJsonObject &document)
{
    const QJsonObject previous = positions();
    scene()->blockSignals(true);
    scene()->clear();
    m_roots.clear();
    m_connections.clear();
    const QJsonArray steps = document.value("steps").toArray();
    for (int i = 0; i < steps.size(); ++i) {
        const QJsonObject step = steps.at(i).toObject();
        const QJsonArray tasks = step.value("group").toObject().value("tasks").toArray();
        const qreal height = tasks.isEmpty() ? cardHeight : 118 + tasks.size() * (cardHeight + 14);
        const QJsonArray path{QStringLiteral("steps"), QString::number(i)};
        auto *card = new StepCard(step, path, height);
        scene()->addItem(card);
        card->setPos(40 + i * (cardWidth + 84), 90);
        m_roots.append(card);
        card->moved = [this]() { updateConnections(); };
        for (int j = 0; j < tasks.size(); ++j) {
            QJsonArray childPath = path;
            childPath.append("group");
            childPath.append("tasks");
            childPath.append(QString::number(j));
            auto *child = new StepCard(tasks.at(j).toObject(), childPath, cardHeight, card);
            child->setScale(0.90);
            child->setPos(12, 118 + j * (cardHeight + 14));
        }
        if (i > 0) {
            auto *edge = scene()->addPath(QPainterPath(), QPen(QColor("#526d80"), 2));
            edge->setZValue(-1);
            m_connections.append(edge);
        }
    }
    restorePositions(previous);
    updateConnections();
    scene()->blockSignals(false);
}

void WorkflowCanvas::updateConnections()
{
    for (int i = 0; i < m_connections.size(); ++i) {
        const QPointF start = m_roots[i]->pos() + QPointF(cardWidth, 60);
        const QPointF end = m_roots[i+1]->pos() + QPointF(0, 60);
        QPainterPath path(start);
        const qreal bend = qMax<qreal>(40, std::abs(end.x() - start.x()) / 2);
        path.cubicTo(start + QPointF(bend, 0), end - QPointF(bend, 0), end);
        path.moveTo(end + QPointF(-8, -5));
        path.lineTo(end);
        path.lineTo(end + QPointF(-8, 5));
        const bool dark = palette().color(QPalette::Window).lightness() < 128;
        m_connections[i]->setPen(QPen(QColor(dark ? "#526d80" : "#93a4b7"), 2));
        m_connections[i]->setPath(path);
    }
    if (m_roots.isEmpty())
        scene()->setSceneRect(-500, -350, 1000, 700);
    else
        scene()->setSceneRect(scene()->itemsBoundingRect().adjusted(-100, -100, 100, 100));
}

QJsonObject WorkflowCanvas::positions() const
{
    QJsonObject result;
    for (int i = 0; i < m_roots.size(); ++i)
        result.insert(QString::number(i), QJsonArray{m_roots[i]->pos().x(), m_roots[i]->pos().y()});
    return result;
}

void WorkflowCanvas::restorePositions(const QJsonObject &positions)
{
    for (int i = 0; i < m_roots.size(); ++i) {
        const QJsonArray position = positions.value(QString::number(i)).toArray();
        if (position.size() == 2 && std::isfinite(position[0].toDouble()) && std::isfinite(position[1].toDouble()))
            m_roots[i]->setPos(position[0].toDouble(), position[1].toDouble());
    }
}

void WorkflowCanvas::fitDocument()
{
    if (m_roots.isEmpty()) {
        resetTransform();
        emit zoomChanged(100);
        centerOn(0, 0);
        return;
    }
    fitInView(scene()->itemsBoundingRect().adjusted(-50, -50, 50, 50), Qt::KeepAspectRatio);
    if (transform().m11() > 1.0)
        resetTransform();
    emit zoomChanged(qRound(transform().m11() * 100));
}

void WorkflowCanvas::drawBackground(QPainter *painter, const QRectF &rect)
{
    const bool dark = palette().color(QPalette::Window).lightness() < 128;
    painter->fillRect(rect, QColor(dark ? "#11151c" : "#f7f9fc"));
    painter->setRenderHint(QPainter::Antialiasing, false);
    // Scale the grid to keep cells readable without drawing dense lines at low zoom.
    qreal spacing = 32;
    while (spacing * transform().m11() < 22)
        spacing *= 2;
    const qreal left = std::floor(rect.left() / spacing) * spacing;
    const qreal top = std::floor(rect.top() / spacing) * spacing;
    auto gridPen = [spacing, dark](qreal coordinate) {
        const bool major = std::fmod(std::round(coordinate / spacing), 4) == 0;
        QColor color(dark ? "#181f29" : "#e8edf3");
        if (major)
            color = QColor(dark ? "#222b36" : "#dce3ed");
        return QPen(color, 0);
    };
    for (qreal x = left; x < rect.right(); x += spacing) {
        painter->setPen(gridPen(x));
        painter->drawLine(QPointF(x, rect.top()), QPointF(x, rect.bottom()));
    }
    for (qreal y = top; y < rect.bottom(); y += spacing) {
        painter->setPen(gridPen(y));
        painter->drawLine(QPointF(rect.left(), y), QPointF(rect.right(), y));
    }
}

void WorkflowCanvas::drawForeground(QPainter *painter, const QRectF &)
{
    if (!m_roots.isEmpty())
        return;
    painter->save();
    painter->resetTransform();
    const QRect area = viewport()->rect();
    const int center = area.center().y();
    QFont title = font();
    title.setPointSize(16);
    title.setWeight(QFont::DemiBold);
    painter->setFont(title);
    painter->setPen(palette().color(QPalette::Text));
    painter->drawText(QRect(20, center - 36, area.width() - 40, 36), Qt::AlignCenter,
        tr("Your workflow starts here"));
    painter->setFont(font());
    painter->setPen(palette().color(QPalette::PlaceholderText));
    painter->drawText(QRect(20, center + 6, area.width() - 40, 50), Qt::AlignHCenter | Qt::TextWordWrap,
        tr("Choose + Add to begin, or open an existing workflow."));
    painter->restore();
}

void WorkflowCanvas::wheelEvent(QWheelEvent *event)
{
    const qreal factor = event->angleDelta().y() > 0 ? 1.12 : 1.0 / 1.12;
    zoomBy(factor);
    event->accept();
}

void WorkflowCanvas::selectPath(const QJsonArray &path)
{
    const QSignalBlocker blocker(scene());
    scene()->clearSelection();
    for (auto *item : scene()->items()) {
        if ((item->flags() & QGraphicsItem::ItemIsSelectable) && item->data(0).toJsonArray() == path) {
            item->setSelected(true);
            break;
        }
    }
}

void WorkflowCanvas::zoomBy(qreal factor)
{
    const qreal current = transform().m11();
    const qreal zoom = qBound<qreal>(0.1, current * factor, 2.0);
    scale(zoom / current, zoom / current);
    emit zoomChanged(qRound(zoom * 100));
}
