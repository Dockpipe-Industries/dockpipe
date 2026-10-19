#include "FurnishingPreview.h"
#include "FurnishingCatalog.h"
#include <QPainter>
#include <QPixmapCache>
#include <algorithm>

QPixmap FurnishingPreview::thumbnail(const QString &type)
{
    QPixmap image;
    const QString key = "cockpit-furnishing:" + type;
    if (QPixmapCache::find(key, &image))
        return image;
    const auto *definition = FurnishingCatalog::find(type);
    if (!definition)
        return {};
    image = QPixmap(144, 112);
    image.fill(QColor("#142836"));
    FurnishingObject object;
    object.type = type;
    struct Face {
        QPolygonF points;
        QColor color;
        float depth;
    };
    QVector<Face> faces;
    QRectF bounds;
    const auto append = [&](const QVector<QVector3D> &vertices, QColor color) {
        Face face;
        face.depth = 0;
        for (const auto &vertex : vertices) {
            face.points.append({0.83 * vertex.x() - 0.55 * vertex.z(),
                -vertex.y() + 0.3 * vertex.x() + 0.45 * vertex.z()});
            face.depth += vertex.x() * 0.55 + vertex.z() * 0.83 + vertex.y() * 0.5;
        }
        face.depth /= vertices.size();
        const auto normal = QVector3D::crossProduct(vertices[1] - vertices[0], vertices[2] - vertices[0]).normalized();
        const float light = 0.68f + 0.32f * std::abs(QVector3D::dotProduct(normal, QVector3D(-0.3, 0.8, -0.5).normalized()));
        face.color = QColor::fromRgbF(color.redF() * light, color.greenF() * light, color.blueF() * light);
        bounds = bounds.united(face.points.boundingRect());
        faces.append(face);
    };
    const auto mesh = FurnishingCatalog::mesh(object);
    if (!mesh.faces.isEmpty()) {
        for (int index = 0; index < mesh.faces.size(); ++index) {
            QVector<QVector3D> vertices;
            for (int vertex : mesh.faces[index])
                vertices.append(mesh.vertices[vertex]);
            append(vertices, mesh.faceColors.value(index, mesh.color));
        }
    } else {
        for (const auto &part : FurnishingCatalog::parts(object)) {
            QVector<QVector3D> vertices;
            for (int y : {-1, 1}) {
                for (int z : {-1, 1}) {
                    for (int x : {-1, 1})
                        vertices.append(part.center + FurnishingCatalog::rotate(part.size * QVector3D(x, y, z) / 2, part.yaw));
                }
            }
            if (part.shape == SpatialSolidShape::Octahedron) {
                const QVector<QVector3D> ring{part.center + QVector3D(-part.size.x() / 2, 0, 0),
                    part.center + QVector3D(0, 0, part.size.z() / 2), part.center + QVector3D(part.size.x() / 2, 0, 0),
                    part.center - QVector3D(0, 0, part.size.z() / 2)};
                for (int side = 0; side < 4; ++side) {
                    append({ring[side], ring[(side + 1) % 4], part.center + QVector3D(0, part.size.y() / 2, 0)}, part.color);
                    append({ring[(side + 1) % 4], ring[side], part.center - QVector3D(0, part.size.y() / 2, 0)}, part.color);
                }
            } else {
                for (const auto &indices : QVector<QVector<int>>{{0, 1, 5, 4}, {3, 2, 6, 7}, {2, 0, 4, 6},
                    {1, 3, 7, 5}, {4, 5, 7, 6}, {2, 3, 1, 0}}) {
                    QVector<QVector3D> polygon;
                    for (int index : indices)
                        polygon.append(vertices[index]);
                    append(polygon, part.color);
                }
            }
        }
    }
    std::stable_sort(faces.begin(), faces.end(), [](const Face &a, const Face &b) { return a.depth < b.depth; });
    const qreal scale = qMin(122 / qMax(1.0, bounds.width()), 92 / qMax(1.0, bounds.height()));
    QPainter painter(&image);
    painter.setRenderHint(QPainter::Antialiasing);
    painter.translate(72, 56);
    painter.scale(scale, scale);
    painter.translate(-bounds.center());
    painter.setPen(Qt::NoPen);
    for (const auto &face : faces) {
        painter.setBrush(face.color);
        painter.drawPolygon(face.points);
    }
    painter.end();
    QPixmapCache::insert(key, image);
    return image;
}
