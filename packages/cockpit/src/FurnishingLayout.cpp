#include "FurnishingLayout.h"
#include "SpatialFoliagePlacement.h"
#include "SpatialAssetLibrary.h"
#include "ContextStore.h"
#include <QCryptographicHash>
#include <QDir>
#include <QFile>
#include <QFileInfo>
#include <QJsonArray>
#include <QJsonDocument>
#include <QLockFile>
#include <QSaveFile>
#include <QSet>
#include <cmath>
#include <algorithm>
#include <limits>

namespace {
QByteArray digest(const QByteArray &bytes)
{
    return QCryptographicHash::hash(bytes, QCryptographicHash::Sha256);
}
bool read(const QString &path, QByteArray &bytes, QString &error)
{
    QFile file(path);
    if (!file.exists())
        return true;
    if (!QFileInfo(path).isFile()) {
        error = "Furnishing layout must be a regular file.";
        return false;
    }
    if (!file.open(QIODevice::ReadOnly)) {
        error = file.errorString();
        return false;
    }
    bytes = file.read(FurnishingLayout::maximumBytes + 1);
    if (bytes.isEmpty() || bytes.size() > FurnishingLayout::maximumBytes || file.error() != QFile::NoError) {
        error = "Furnishing layout is unreadable or exceeds 512 KiB. The file is preserved.";
        return false;
    }
    return true;
}
FurnishingHit boxHit(const QVector3D &origin, const QVector3D &direction, const SpatialSolid &box)
{
    const auto ray = FurnishingCatalog::rotate(direction, -box.yaw);
    const auto offset = FurnishingCatalog::rotate(origin - box.center, -box.yaw);
    float near = 0, far = 2500;
    QVector3D normal;
    for (int axis = 0; axis < 3; ++axis) {
        const float half = box.size[axis] / 2;
        if (std::abs(ray[axis]) < 0.00001f) {
            if (std::abs(offset[axis]) > half)
                return {};
            continue;
        }
        float a = (-half - offset[axis]) / ray[axis];
        float b = (half - offset[axis]) / ray[axis];
        const float sign = a < b ? -1 : 1;
        if (a > b)
            std::swap(a, b);
        if (a > near) {
            near = a;
            normal = {};
            normal[axis] = sign;
        }
        far = qMin(far, b);
        if (near > far)
            return {};
    }
    if (near <= 0 || near >= 2500)
        return {};
    return {true, origin + direction * near, FurnishingCatalog::rotate(normal, box.yaw), near};
}
bool overlap(const SpatialSolid &a, const SpatialSolid &b)
{
    if (std::abs(a.center.y() - b.center.y()) >= (a.size.y() + b.size.y()) / 2 - 1)
        return false;
    const auto ax = FurnishingCatalog::rotate({1, 0, 0}, a.yaw);
    const auto az = FurnishingCatalog::rotate({0, 0, 1}, a.yaw);
    const auto bx = FurnishingCatalog::rotate({1, 0, 0}, b.yaw);
    const auto bz = FurnishingCatalog::rotate({0, 0, 1}, b.yaw);
    for (const auto &axis : {ax, az, bx, bz}) {
        const auto project = [&](const SpatialSolid &solid, const QVector3D &x, const QVector3D &z) {
            return std::abs(QVector3D::dotProduct(axis, x)) * solid.size.x() / 2
                + std::abs(QVector3D::dotProduct(axis, z)) * solid.size.z() / 2;
        };
        if (std::abs(QVector3D::dotProduct(a.center - b.center, axis)) >= project(a, ax, az) + project(b, bx, bz) - 1)
            return false;
    }
    return true;
}
}

bool FurnishingLayout::validObject(const FurnishingObject &object)
{
    const auto dimensions = FurnishingCatalog::size(object);
    return FurnishingCatalog::find(object.type) && !object.id.isEmpty() && object.id.size() <= 160
        && object.baseDisplay.size() <= 160 && object.binding.provider.size() <= 40
        && object.binding.target.size() <= 4096 && object.binding.text.size() <= 4096
        && object.binding.application.desktopId.size() <= 512 && object.binding.application.windowClass.size() <= 512
        && object.binding.application.name.size() <= 512
        && (object.marqueeBinding.provider == "text" || object.marqueeBinding.provider == "file")
        && object.marqueeBinding.target.size() <= 4096 && object.marqueeBinding.text.size() <= 4096
        && std::isfinite(object.yaw) && std::abs(object.yaw) <= 360
        && std::isfinite(object.position.x()) && std::isfinite(object.position.y()) && std::isfinite(object.position.z())
        && object.position.length() < 1000000 && dimensions.x() > 0 && dimensions.x() <= 4000
        && dimensions.y() > 0 && dimensions.y() <= 4000 && dimensions.z() > 0;
}
bool FurnishingLayout::load(const QString &location, QString &error)
{
    m_loaded = false;
    objects.clear();
    environment = location;
    error.clear();
    if (location.isEmpty() || location.size() > 8192) {
        error = "This environment has no durable furnishing identity.";
        return false;
    }
    const QString directory = ContextStore::configDir() + "/cockpit-furnishings";
    if (!QDir().mkpath(directory)) {
        error = "Cannot create the furnishing layout directory.";
        return false;
    }
    m_path = directory + "/" + QString::fromLatin1(digest(location.toUtf8()).toHex()) + ".json";
    QByteArray bytes;
    if (!read(m_path, bytes, error))
        return false;
    if (!bytes.isEmpty()) {
        const auto document = QJsonDocument::fromJson(bytes);
        const auto root = document.object();
        const auto entries = root.value("objects").toArray();
        if (!document.isObject() || root.value("version").toInt() != 1 || root.value("environment").toString() != location
            || !root.value("objects").isArray() || entries.size() > maximumObjects) {
            error = "Invalid furnishing layout. The saved file has been preserved.";
            return false;
        }
        QSet<QString> ids;
        for (const auto &value : entries) {
            const auto item = value.toObject();
            const auto position = item.value("position").toArray();
            const auto size = item.value("screenSize").toArray();
            if (position.size() != 3 || size.size() != 2) {
                error = "Invalid furnishing transform.";
                objects.clear();
                return false;
            }
            const double invalid = std::numeric_limits<double>::quiet_NaN();
            FurnishingObject object;
            object.id = item.value("id").toString();
            object.type = item.value("type").toString();
            object.position = {float(position.at(0).toDouble(invalid)), float(position.at(1).toDouble(invalid)), float(position.at(2).toDouble(invalid))};
            object.yaw = item.value("yaw").toDouble(invalid);
            object.baseDisplay = item.value("baseDisplay").toString();
            object.screenSize = {size.at(0).toDouble(), size.at(1).toDouble()};
            object.removed = item.value("removed").toBool();
            const auto binding = item.value("binding").toObject();
            object.binding = {binding.value("provider").toString(), binding.value("target").toString(), binding.value("text").toString()};
            const auto application = binding.value("application").toObject();
            object.binding.application = {application.value("desktopId").toString(),
                application.value("windowClass").toString(), application.value("name").toString()};
            if (item.contains("marquee")) {
                const auto marquee = item.value("marquee").toObject();
                object.marqueeBinding = {marquee.value("provider").toString(), marquee.value("target").toString(), marquee.value("text").toString()};
            }
            if (!validObject(object) || ids.contains(object.id)) {
                error = "Invalid or duplicate furnishing object. The saved file has been preserved.";
                objects.clear();
                return false;
            }
            ids.insert(object.id);
            objects.append(object);
        }
    }
    m_revision = digest(bytes);
    m_loaded = true;
    return true;
}
bool FurnishingLayout::save(QString &error)
{
    error.clear();
    if (!m_loaded || objects.size() > maximumObjects) {
        error = "Load a valid layout first; at most 192 saved objects per environment.";
        return false;
    }
    QJsonArray entries;
    QSet<QString> ids;
    for (const auto &object : objects) {
        if (!validObject(object) || ids.contains(object.id)) {
            error = "Invalid or duplicate furnishing object.";
            return false;
        }
        ids.insert(object.id);
        entries.append(QJsonObject{{"id", object.id}, {"type", object.type}, {"yaw", object.yaw},
            {"position", QJsonArray{object.position.x(), object.position.y(), object.position.z()}},
            {"baseDisplay", object.baseDisplay}, {"screenSize", QJsonArray{object.screenSize.width(), object.screenSize.height()}},
            {"marquee", QJsonObject{{"provider", object.marqueeBinding.provider}, {"target", object.marqueeBinding.target}, {"text", object.marqueeBinding.text}}},
            {"removed", object.removed}, {"binding", QJsonObject{{"provider", object.binding.provider}, {"target", object.binding.target}, {"text", object.binding.text},
                {"application", QJsonObject{{"desktopId", object.binding.application.desktopId},
                    {"windowClass", object.binding.application.windowClass}, {"name", object.binding.application.name}}}}}});
    }
    const auto bytes = QJsonDocument(QJsonObject{{"version", 1}, {"environment", environment}, {"objects", entries}}).toJson();
    if (bytes.size() > maximumBytes) {
        error = "Furnishing layout exceeds 512 KiB.";
        return false;
    }
    QLockFile lock(m_path + ".lock");
    if (!lock.tryLock(0)) {
        error = "Another instance is saving this layout. Try again.";
        return false;
    }
    QByteArray existing;
    if (!read(m_path, existing, error))
        return false;
    if (digest(existing) != m_revision) {
        error = "Layout changed in another instance. Re-enter the environment before editing.";
        return false;
    }
    QSaveFile file(m_path);
    if (!file.open(QIODevice::WriteOnly) || file.write(bytes) != bytes.size() || !file.commit()) {
        error = file.errorString();
        return false;
    }
    m_revision = digest(bytes);
    return true;
}
FurnishingHit FurnishingLayout::target(const SpatialInterior &room, const QVector3D &origin, const QVector3D &direction)
{
    FurnishingHit best;
    auto consider = [&](const SpatialSolid &box) {
        const auto hit = boxHit(origin, direction, box);
        if (hit.hit && hit.distance < best.distance)
            best = hit;
    };
    const float w = room.size.width(), d = room.size.height(), h = room.height;
    consider({{0, -5, 0}, {w, 10, d}, {}});
    if (!room.outdoors) {
        consider({{0, h + 5, 0}, {w, 10, d}, {}});
        consider({{0, h / 2, d / 2 + 5}, {w, h, 10}, {}});
        consider({{0, h / 2, -d / 2 - 5}, {w, h, 10}, {}});
        consider({{w / 2 + 5, h / 2, 0}, {10, h, d}, {}});
        consider({{-w / 2 - 5, h / 2, 0}, {10, h, d}, {}});
    }
    for (const auto &solid : room.solids)
        consider(solid);
    return best;
}
float FurnishingLayout::rayObject(const FurnishingObject &object, const QVector3D &origin, const QVector3D &direction)
{
    const auto hit = boxHit(origin, direction, {object.position, FurnishingCatalog::size(object), {}, true, SpatialSolidShape::Box, {}, object.yaw});
    return hit.hit ? hit.distance : 2500;
}
QString FurnishingLayout::placementError(const FurnishingObject &object, const SpatialInterior &base,
    const QVector<FurnishingObject> &others, const FurnishingHit &hit)
{
    const auto *item = FurnishingCatalog::find(object.type);
    if (!item || !validObject(object) || !hit.hit)
        return "Aim at a placement surface";
    if (item->mount == FurnishingMount::Floor && (hit.normal.y() < 0.9f || hit.point.y() > 30))
        return "Place this item on the floor";
    if (item->mount == FurnishingMount::Surface && hit.normal.y() < 0.9f)
        return "Aim at a table top or horizontal surface";
    if (item->mount == FurnishingMount::Wall && std::abs(hit.normal.y()) > 0.1f)
        return "Aim at a wall or vertical surface";
    if (item->mount == FurnishingMount::Ceiling && hit.normal.y() > -0.9f)
        return "Aim at the ceiling or an overhead surface";
    const auto dimensions = FurnishingCatalog::size(object);
    const auto x = FurnishingCatalog::rotate({dimensions.x() / 2, 0, 0}, object.yaw);
    const auto z = FurnishingCatalog::rotate({0, 0, dimensions.z() / 2}, object.yaw);
    if (std::abs(object.position.x()) + std::abs(x.x()) + std::abs(z.x()) > base.size.width() / 2 + 1
        || std::abs(object.position.z()) + std::abs(x.z()) + std::abs(z.z()) > base.size.height() / 2 + 1
        || object.position.y() - dimensions.y() / 2 < -1 || object.position.y() + dimensions.y() / 2 > base.height + 1)
        return "Keep the whole object inside the environment";
    if ((QPointF(object.position.x(), object.position.z()) - base.exit).manhattanLength() < 130)
        return "Keep the return doorway clear";
    const auto pieces = FurnishingCatalog::parts(object);
    int parts = pieces.size();
    int faces = FurnishingCatalog::faceCount(object);
    int objects = 1;
    const auto replacesBase = [&](const FurnishingObject &item) {
        return !item.baseDisplay.isEmpty() && std::any_of(base.displays.cbegin(), base.displays.cend(),
            [&](const auto &display) { return display.id == item.baseDisplay; });
    };
    int screens = base.displays.size() + FurnishingCatalog::screenCount(object) - int(replacesBase(object));
    int actions = base.actions.size() + FurnishingCatalog::actions(object).size();
    for (const auto &other : others) {
        if (other.id == object.id)
            continue;
        screens -= int(replacesBase(other));
        if (other.removed)
            continue;
        ++objects;
        faces += FurnishingCatalog::faceCount(other);
        const auto otherParts = FurnishingCatalog::parts(other);
        if (other.baseDisplay.isEmpty()) {
            parts += otherParts.size();
        }
        screens += FurnishingCatalog::screenCount(other);
        actions += FurnishingCatalog::actions(other).size();
        for (const auto &a : pieces) {
            for (const auto &b : otherParts) {
                if (overlap(a, b))
                    return "Overlaps another furnishing";
            }
        }
    }
    if (objects > maximumObjects || parts > maximumParts || screens > SpatialWorkspace::maximumDisplays
        || faces > SpatialAssetLibrary::maximumSceneFaces)
        return "Scene budget reached (192 objects / 768 collision parts / 8192 mesh faces / 24 screens)";
    if (actions > SpatialInterior::maximumActions)
        return "Room interaction capacity reached (32 actions including seats)";
    for (const auto &a : pieces) {
        for (int index = 0; index < qMin(base.foliage.size(), qsizetype(SpatialInterior::maximumFoliage)); ++index) {
            if (overlap(a, SpatialFoliagePlacement::envelope(base.foliage[index])))
                return "Intersects tree canopy";
        }
        for (const auto &b : base.solids) {
            if (overlap(a, b))
                return "Intersects the environment";
        }
    }
    return {};
}
