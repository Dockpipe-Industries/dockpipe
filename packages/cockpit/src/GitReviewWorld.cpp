#include "GitReviewWorld.h"
#include "SourceHighlighter.h"
#include <QFileInfo>
#include <QFontDatabase>
#include <QPainter>
#include <QTextDocument>
#include <QAbstractTextDocumentLayout>
#include <QTextCursor>
#include <QTextBlock>
#include <QtMath>
#include <algorithm>

GitReviewWorld GitReviewWorld::build(const GitReviewModel &model, const GitReviewProgress &progress, int firstFile)
{
    GitReviewWorld result;
    auto &plan = result.plan;
    const int count = qMin(filesPerSection, int(model.files.size()) - firstFile);
    if (count <= 0) {
        result.world.diagnostic = "No changes in this review section.";
        return result;
    }
    plan.options.topology = SpatialTopology::Spine;
    plan.options.style.ground = QColor("#101c2b");
    plan.options.style.sky = QColor("#101629");
    plan.options.style.structure = QColor("#38556d");
    plan.options.style.accent = QColor("#65b8d6");
    plan.options.locomotion = "walk";
    plan.size = {4000, count * 1100.0 + 1800};
    plan.height = 600;
    const float firstZ = -(count - 1) * 550.0f;
    plan.spawn = {0, 160, firstZ - 650};
    plan.transforms.append(SpatialWorldTransform{});
    plan.graph.nodes.append({SpatialWebNode::Role::Hub, SpatialWebNode::Action::None, "review", "Working Changes", {}});
    plan.anchors.append({0, plan.spawn});
    int itemIndex = 0;
    auto surface = [&](int collection, const QString &id, const QString &title, const QVector3D &center,
                       const QVector3D &anchor, float yaw, bool portal) {
        const int node = plan.graph.nodes.size();
        plan.graph.nodes.append({SpatialWebNode::Role::Destination, SpatialWebNode::Action::None,
            id, title.left(180), {}, collection, itemIndex++});
        plan.anchors.append({node, anchor});
        plan.connections.append({collection, node});
        SpatialDisplay display;
        display.id = id;
        display.freestanding = true;
        display.center = center;
        display.size = {620, 330};
        display.yaw = yaw;
        display.portalCenter = anchor;
        display.portalApproachDistance = 110;
        // Route gates use ordinary display interaction plus a position trigger
        // owned by navigation, not web URLs or provider logic in the renderer.
        display.portal = false;
        display.prompt = portal ? "Continue review" : "Inspect aligned code · E / A";
        plan.displays.append(display);
    };
    int previous = 0;
    QString previousGroup;
    float wing = 0;
    for (int slot = 0; slot < count; ++slot) {
        const int index = firstFile + slot;
        const auto &file = model.files[index];
        const QString group = QFileInfo(file.path).path();
        if (group != previousGroup) {
            wing = slot % 2 == 0 ? -180 : 180;
            previousGroup = group;
        }
        const QVector3D anchor(wing, 160, firstZ + slot * 1100);
        result.filePositions.append(anchor);
        const int collection = plan.graph.nodes.size();
        plan.graph.nodes.append({SpatialWebNode::Role::Collection, SpatialWebNode::Action::None,
            "file-" + QString::number(index), file.path.left(180), {}, 0});
        plan.anchors.append({collection, anchor});
        plan.connections.append({previous, collection});
        previous = collection;
        QColor completion("#48667b");
        if (file.conflict())
            completion = QColor("#c95a68");
        else if (progress.stale(file))
            completion = QColor("#bd8eeb");
        else if (progress.reviewed(file))
            completion = QColor("#54c6a0");
        const bool infrastructure = file.path.contains("terraform") || file.path.endsWith(".tf") || file.path.startsWith(".github/");
        const QColor material = infrastructure ? QColor("#5a5264") : QColor("#263d56");
        for (int side : {-1, 1}) {
            const float x = wing + side * 365;
            plan.parts.append({0, {x, 220, anchor.z()}, {40, 440, 760}, material});
            plan.parts.append({0, {x, 445, anchor.z()}, {48, 10, 770}, completion, SpatialSolidShape::Box, false});
            plan.parts.append({0, {wing + side * 390, 250, anchor.z() - 430}, {65, 500, 65}, material});
            plan.parts.append({0, {wing + side * 390, 250, anchor.z() + 430}, {65, 500, 65}, material});
        }
        // Open chamber roof beams, restrained completion strips, and a clear
        // central route; dimensions and wings follow actual file grouping.
        plan.parts.append({0, {wing, 510, anchor.z() - 430}, {840, 30, 65}, completion, SpatialSolidShape::Box, false,
            QString::number(index + 1) + " · " + file.path.left(140)});
        surface(collection, "before-" + QString::number(index), file.path,
            {wing - 338, 220, anchor.z()}, anchor, 90, false);
        surface(collection, "after-" + QString::number(index), file.path,
            {wing + 338, 220, anchor.z()}, anchor, -90, false);
    }
    const int route = plan.graph.nodes.size();
    plan.graph.nodes.append({SpatialWebNode::Role::Collection, SpatialWebNode::Action::None, "route", "Review navigation", {}, 0});
    const QVector3D end(result.filePositions.last().x(), 160, result.filePositions.last().z() + 650);
    plan.anchors.append({route, end});
    plan.connections.append({previous, route});
    surface(route, "next", "Next section / decision chamber", end + QVector3D(0, 50, 210), end, 0, true);
    result.world = SpatialWebWorld::compile(plan);
    return result;
}

QImage GitReviewWorld::codeSurface(const GitReviewFile &file, const GitReviewDiff *diff, int hunkIndex, bool before)
{
    return codePreview(file, diff, hunkIndex, before).imageAt(0);
}

ScrollingPreview GitReviewWorld::codePreview(const GitReviewFile &file, const GitReviewDiff *diff, int hunkIndex, bool before)
{
    QImage image(512, 320, QImage::Format_ARGB32_Premultiplied);
    image.fill(QColor("#0b1320"));
    QPainter painter(&image);
    painter.setPen(QColor("#c3d6e5"));
    QFont font = QFontDatabase::systemFont(QFontDatabase::FixedFont);
    font.setPixelSize(12);
    painter.setFont(font);
    painter.fillRect(0, 0, 512, 36, before ? QColor("#362129") : QColor("#16372f"));
    const QString label = diff ? (before ? diff->beforeLabel : diff->afterLabel) : (before ? "ORIGINAL" : "MODIFIED");
    painter.drawText(QRect(12, 3, 488, 30), Qt::AlignVCenter,
        painter.fontMetrics().elidedText(label + " · " + file.path, Qt::ElideMiddle, 488));
    if (!diff || diff->hunks.isEmpty()) {
        const QString notice = !diff ? "Approach this chamber to load its code.\nSelect a wall to inspect and copy." :
            diff->notice.isEmpty() ? "No textual hunks in this comparison.\nEmpty file, rename or metadata-only change." : diff->notice;
        painter.drawText(QRect(20, 60, 472, 230), Qt::TextWordWrap, notice);
        return ScrollingPreview(image);
    }
    const auto &hunk = diff->hunks[qBound(0, hunkIndex, int(diff->hunks.size()) - 1)];
    QStringList lines;
    // Keep preview preparation bounded even for a 20,000-row comparison.
    const int count = qMin(128, int(hunk.rows.size()));
    bool excerpt = count < hunk.rows.size();
    for (int index = 0; index < count; ++index) {
        const auto &row = hunk.rows[index];
        const QString text = before ? row.before : row.after;
        excerpt = excerpt || text.size() > 240;
        lines.append(text.size() > 240 ? text.left(239) + QChar(0x2026) : text);
    }
    QTextDocument document;
    font.setPixelSize(12);
    document.setDefaultFont(font);
    document.setDefaultStyleSheet("body { color: #d5e1ee; }");
    document.setDocumentMargin(0);
    document.setPlainText(lines.join('\n'));
    QTextOption option;
    option.setWrapMode(QTextOption::WrapAtWordBoundaryOrAnywhere);
    document.setDefaultTextOption(option);
    document.setTextWidth(455);
    SourceHighlighter syntax(&document);
    syntax.updateTheme(true);
    syntax.setFileName(file.path);
    syntax.rehighlight();
    const int documentHeight = qCeil(document.size().height());
    excerpt = excerpt || documentHeight > ScrollingPreview::maximumHeight;
    QImage content(512, qBound(242, documentHeight, ScrollingPreview::maximumHeight), QImage::Format_ARGB32_Premultiplied);
    content.fill(QColor("#0b1320"));
    QPainter textPainter(&content);
    textPainter.setFont(font);
    for (int index = 0; index < count; ++index) {
        const auto &row = hunk.rows[index];
        const auto bounds = document.documentLayout()->blockBoundingRect(document.findBlockByNumber(index));
        if (before ? row.removed : row.added)
            textPainter.fillRect(QRectF(0, bounds.top(), 512, bounds.height()), before ? QColor("#472630") : QColor("#163d32"));
        const int line = before ? row.beforeLine : row.afterLine;
        textPainter.setPen(QColor("#8ea9ba"));
        if (line > 0)
            textPainter.drawText(QRectF(4, bounds.top(), 40, 15), Qt::AlignRight | Qt::AlignVCenter, QString::number(line));
    }
    textPainter.setClipRect(48, 0, 455, content.height());
    textPainter.translate(48, 0);
    QAbstractTextDocumentLayout::PaintContext context;
    context.palette.setColor(QPalette::Text, QColor("#d5e1ee"));
    context.clip = QRectF(0, 0, 455, content.height());
    document.documentLayout()->draw(&textPainter, context);
    textPainter.end();
    painter.setPen(QColor("#a2bed0"));
    painter.drawText(QRect(12, 292, 488, 22), Qt::AlignVCenter,
        excerpt ? QString("Preview excerpt · Select to inspect the full hunk")
            : QString("Hunk %1/%2 · +%3 −%4 · Select to focus").arg(hunkIndex + 1).arg(diff->hunks.size()).arg(hunk.additions).arg(hunk.deletions));
    painter.end();
    return ScrollingPreview(image, content, QRect(0, 44, 512, 242));
}
