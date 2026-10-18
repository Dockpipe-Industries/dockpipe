#include "SpatialAssetLibrary.h"
#include <QCache>
#include <QFile>
#include <QJsonArray>
#include <QJsonDocument>
#include <QJsonObject>
#include <QRegularExpression>
#include <cmath>

SpatialMesh SpatialAssetLibrary::model(const QString &id)
{
    // GUI-thread asset cache: bounded independently of installed pack size.
    static QCache<QString, SpatialMesh> cache(8 * 1024 * 1024);
    if (const auto *cached = cache.object(id))
        return *cached;
    static const QRegularExpression identifier("^(kenney|moto)-[A-Za-z0-9]+$");
    if (!identifier.match(id).hasMatch())
        return {};
    QFile file(":/cockpit/models/" + id + ".json");
    if (!file.open(QIODevice::ReadOnly) || file.size() > 512 * 1024)
        return {};
    const auto root = QJsonDocument::fromJson(file.readAll()).object();
    const auto vertices = root.value("vertices").toArray();
    const auto faces = root.value("faces").toArray();
    const auto colors = root.value("colors").toArray();
    const auto normals = root.value("normals").toArray();
    if (vertices.isEmpty() || vertices.size() > 4096 || faces.isEmpty()
        || faces.size() > maximumModelFaces || colors.size() != faces.size()
        || (!normals.isEmpty() && normals.size() != vertices.size()))
        return {};
    SpatialMesh mesh;
    mesh.color = QColor("#b9c4cf");
    for (const auto &value : vertices) {
        const auto coordinates = value.toArray();
        if (coordinates.size() != 3)
            return {};
        QVector3D vertex;
        for (int axis = 0; axis < 3; ++axis) {
            if (!coordinates[axis].isDouble() || !std::isfinite(coordinates[axis].toDouble())
                || std::abs(coordinates[axis].toDouble()) > 4000)
                return {};
            vertex[axis] = coordinates[axis].toDouble();
        }
        mesh.vertices.append(vertex);
    }
    for (const auto &value : normals) {
        const auto coordinates = value.toArray();
        if (coordinates.size() != 3)
            return {};
        QVector3D normal;
        for (int axis = 0; axis < 3; ++axis) {
            const double component = coordinates[axis].toDouble(2);
            if (!std::isfinite(component) || std::abs(component) > 1.001)
                return {};
            normal[axis] = component;
        }
        if (normal.lengthSquared() < .99f || normal.lengthSquared() > 1.01f)
            return {};
        mesh.normals.append(normal.normalized());
    }
    for (int index = 0; index < faces.size(); ++index) {
        const auto indices = faces[index].toArray();
        const QColor color(colors[index].toString());
        if (indices.size() < 3 || indices.size() > 4 || !color.isValid())
            return {};
        QVector<int> face;
        for (const auto &value : indices) {
            const int vertex = value.toInt(-1);
            if (!value.isDouble() || value.toDouble() != vertex || vertex < 0 || vertex >= vertices.size())
                return {};
            face.append(vertex);
        }
        mesh.faces.append(face);
        mesh.faceColors.append(color);
    }
    const int cost = (mesh.vertices.size() + mesh.normals.size()) * sizeof(QVector3D)
        + mesh.faces.size() * (sizeof(QVector<int>) + 4 * sizeof(int) + sizeof(QColor));
    cache.insert(id, new SpatialMesh(mesh), cost);
    return mesh;
}
