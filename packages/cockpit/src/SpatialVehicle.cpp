#include "SpatialVehicle.h"
#include "DirtBikeModel.h"
#include "SpatialWorkspace.h"
#include "SpatialFoliagePlacement.h"
#include <QStringList>
#include <cmath>

namespace {
constexpr float pi = 3.14159265358979323846f;
// Separating axes for two oriented footprints, including narrow obstructions
// between the wheels. Vehicle collision never uses only the driver's point.
bool overlaps(QVector3D first, QVector3D firstSize, float firstHeading,
    QVector3D second, QVector3D secondSize, float secondHeading)
{
    const auto firstRight = SpatialVehicleModel::rotate({1, 0, 0}, firstHeading);
    const auto firstForward = SpatialVehicleModel::rotate({0, 0, 1}, firstHeading);
    const auto secondRight = SpatialVehicleModel::rotate({1, 0, 0}, secondHeading);
    const auto secondForward = SpatialVehicleModel::rotate({0, 0, 1}, secondHeading);
    for (const auto &axis : {firstRight, firstForward, secondRight, secondForward}) {
        const float firstRadius = (std::abs(QVector3D::dotProduct(axis, firstRight)) * firstSize.x()
            + std::abs(QVector3D::dotProduct(axis, firstForward)) * firstSize.z()) / 2;
        const float secondRadius = (std::abs(QVector3D::dotProduct(axis, secondRight)) * secondSize.x()
            + std::abs(QVector3D::dotProduct(axis, secondForward)) * secondSize.z()) / 2;
        if (std::abs(QVector3D::dotProduct(second - first, axis)) >= firstRadius + secondRadius)
            return false;
    }
    return true;
}
}

bool SpatialVehicleModel::valid(const SpatialVehicle &vehicle)
{
    return !vehicle.id.isEmpty() && vehicle.id.size() <= 64 && !vehicle.name.isEmpty() && vehicle.name.size() <= 140
        && QStringList{"sedan", "coupe", "suv", "pickup", "van", "dirt-bike"}.contains(vehicle.body)
        && (vehicle.year == 0 || (vehicle.year >= 1886 && vehicle.year <= 2100)) && vehicle.paint.isValid()
        && std::isfinite(vehicle.heading) && std::abs(vehicle.heading) <= 2 * pi
        && std::isfinite(vehicle.position.x()) && std::isfinite(vehicle.position.z()) && vehicle.position.y() == 0;
}

QVector3D SpatialVehicleModel::size(const SpatialVehicle &vehicle)
{
    if (vehicle.body == "dirt-bike")
        return {85, 135, 220};
    if (vehicle.body == "pickup")
        return {210, 178, 510};
    if (vehicle.body == "van")
        return {210, 215, 510};
    if (vehicle.body == "suv")
        return {205, 180, 455};
    if (vehicle.body == "coupe")
        return {195, 135, 430};
    return {195, 150, 455};
}

QVector3D SpatialVehicleModel::rotate(const QVector3D &point, float heading)
{
    return {point.x() * std::cos(heading) + point.z() * std::sin(heading), point.y(),
        -point.x() * std::sin(heading) + point.z() * std::cos(heading)};
}

QVector<SpatialVehicleFace> SpatialVehicleModel::build(const SpatialVehicle &vehicle)
{
    QVector<SpatialVehicleFace> faces;
    if (!valid(vehicle))
        return faces;
    const auto dimensions = size(vehicle);
    const float width = dimensions.x() / 2 - 10;
    const float length = dimensions.z() / 2;
    const float roof = dimensions.y();
    const QColor glass("#18364b");
    const QColor rubber("#151b23");
    const QColor metal("#9cabb8");
    const auto box = [&](QVector3D center, QVector3D extent, QColor color) {
        const auto low = center - extent / 2;
        const auto high = center + extent / 2;
        const float a = low.x(), b = high.x(), c = low.y(), d = high.y(), e = low.z(), f = high.z();
        faces.append({{{a,c,e},{b,c,e},{b,d,e},{a,d,e}}, color.darker(115)});
        faces.append({{{a,c,f},{b,c,f},{b,d,f},{a,d,f}}, color});
        faces.append({{{a,c,e},{a,c,f},{a,d,f},{a,d,e}}, color.darker(130)});
        faces.append({{{b,c,e},{b,c,f},{b,d,f},{b,d,e}}, color.darker(110)});
        faces.append({{{a,d,e},{b,d,e},{b,d,f},{a,d,f}}, color.lighter(120)});
        faces.append({{{a,c,e},{b,c,e},{b,c,f},{a,c,f}}, color.darker(150)});
    };
    if (vehicle.body == "dirt-bike")
        return DirtBikeModel::build(vehicle.paint, false);
    box({0, 54, 0}, {width * 2, 50, length * 2}, vehicle.paint);
    box({0, 88, length - 65}, {width * 2 - 6, 28, 125}, vehicle.paint);
    const bool pickup = vehicle.body == "pickup";
    const bool longCab = vehicle.body == "suv" || vehicle.body == "van";
    const float rear = pickup ? -35 : (longCab ? -length + 25 : -105);
    const float front = length - 130;
    const float roofWidth = width - 16;
    // Sloped windscreen, rear glass and side windows form a closed cabin.
    faces.append({{{-width,100,front},{width,100,front},{roofWidth,roof-8,front-40},{-roofWidth,roof-8,front-40}}, glass});
    faces.append({{{-width,100,rear},{width,100,rear},{roofWidth,roof-8,rear+20},{-roofWidth,roof-8,rear+20}}, glass.darker(115)});
    for (float side : {-1.0f, 1.0f}) {
        faces.append({{{side*width,100,rear},{side*width,100,front},{side*roofWidth,roof-8,front-40},
            {side*roofWidth,roof-8,rear+20}}, glass.lighter(side < 0 ? 105 : 135)});
        box({side * (width - 5), 91, (rear + front) / 2}, {10, 24, front - rear}, vehicle.paint);
        box({side * (roofWidth + 3), (roof + 100) / 2, (rear + front) / 2}, {7, roof - 100, 9}, vehicle.paint);
    }
    box({0, roof - 4, (rear + front - 20) / 2}, {roofWidth * 2, 8, front - rear - 60}, vehicle.paint);
    if (pickup) {
        box({0, 79, (-length + rear) / 2}, {width * 2 - 12, 5, length + rear}, rubber);
        for (float side : {-1.0f, 1.0f})
            box({side * (width - 6), 99, (-length + rear) / 2}, {12, 42, length + rear}, vehicle.paint);
        box({0, 99, -length + 6}, {width * 2, 42, 12}, vehicle.paint);
    } else {
        box({0, 86, -length + 45}, {width * 2, 20, 90}, vehicle.paint);
    }
    box({0, 57, length + 1}, {width * 1.15f, 27, 4}, rubber);
    for (float side : {-1.0f, 1.0f}) {
        box({side * width * 0.8f, 83, length + 2}, {29, 16, 4}, QColor("#fff1bd"));
        box({side * width * 0.8f, 83, -length - 2}, {28, 18, 4}, QColor("#c83749"));
        // Eight-sided tyres with inset metal hubs; no cylinders/tessellation per frame.
        for (float axle : {-1.0f, 1.0f}) {
            const float z = axle * (length - 73);
            const float inside = side * (width - 10);
            const float outside = side * (width + 10);
            QVector<QVector3D> hub;
            for (int edge = 0; edge < 8; ++edge) {
                const float a = edge * pi / 4;
                const float b = (edge + 1) * pi / 4;
                const QVector3D first(outside, 35 + 35 * std::sin(a), z + 35 * std::cos(a));
                const QVector3D second(outside, 35 + 35 * std::sin(b), z + 35 * std::cos(b));
                faces.append({{first, second, {inside, second.y(), second.z()}, {inside, first.y(), first.z()}}, rubber});
                faces.append({{{outside + side, 35, z}, first, second}, rubber.lighter(140)});
                hub.append({outside + side * 2, 35 + 19 * std::sin(a), z + 19 * std::cos(a)});
            }
            for (int edge = 0; edge < 8; edge += 2)
                faces.append({{{outside + side * 2, 35, z}, hub[edge], hub[edge + 1], hub[(edge + 2) % 8]}, metal});
        }
    }
    return faces;
}

bool SpatialVehicleModel::contains(const SpatialVehicle &vehicle, const QVector3D &point, float padding)
{
    const auto local = rotate(point - vehicle.position, -vehicle.heading);
    const auto half = size(vehicle) / 2;
    return std::abs(local.x()) < half.x() + padding && std::abs(local.z()) < half.z() + padding
        && local.y() < half.y() * 2 + 25 && local.y() > -25;
}

bool SpatialVehicleModel::blocked(const SpatialInterior &room, const SpatialVehicle &vehicle)
{
    const auto dimensions = size(vehicle);
    for (int index = 0; index < qMin(room.foliage.size(), qsizetype(SpatialInterior::maximumFoliage)); ++index) {
        const auto trunk = SpatialFoliagePlacement::trunk(room.foliage[index]);
        if (trunk.center.y() + trunk.size.y() / 2 > vehicle.position.y() + 25
            && trunk.center.y() - trunk.size.y() / 2 < vehicle.position.y() + dimensions.y()
            && overlaps(vehicle.position, dimensions, vehicle.heading, trunk.center, trunk.size, 0))
            return true;
    }
    for (float x : {-1.0f, 1.0f}) {
        for (float z : {-1.0f, 1.0f}) {
            const auto corner = vehicle.position + rotate({x * dimensions.x() / 2, 0, z * dimensions.z() / 2}, vehicle.heading);
            if (std::abs(corner.x()) > room.size.width() / 2 - 10 || std::abs(corner.z()) > room.size.height() / 2 - 10)
                return true;
        }
    }
    for (const auto &solid : room.solids) {
        if (solid.collision && solid.center.y() + solid.size.y() / 2 > vehicle.position.y() + 25
            && solid.center.y() - solid.size.y() / 2 < vehicle.position.y() + dimensions.y()
            && overlaps(vehicle.position, dimensions, vehicle.heading, solid.center, solid.size, -solid.yaw * pi / 180))
            return true;
    }
    for (const auto &other : room.vehicles) {
        if (other.id != vehicle.id && overlaps(vehicle.position, dimensions, vehicle.heading, other.position, size(other), other.heading))
            return true;
    }
    return false;
}

float SpatialVehicleModel::rayDistance(const SpatialVehicle &vehicle, const QVector3D &origin, const QVector3D &direction)
{
    const auto local = rotate(origin - vehicle.position, -vehicle.heading);
    const auto ray = rotate(direction, -vehicle.heading);
    const auto dimensions = size(vehicle);
    const QVector3D low(-dimensions.x() / 2, 0, -dimensions.z() / 2);
    const QVector3D high(dimensions.x() / 2, dimensions.y(), dimensions.z() / 2);
    float near = 0, far = 10000;
    for (int axis = 0; axis < 3; ++axis) {
        if (std::abs(ray[axis]) < 0.0001f) {
            if (local[axis] < low[axis] || local[axis] > high[axis])
                return -1;
        } else {
            float entry = (low[axis] - local[axis]) / ray[axis];
            float exit = (high[axis] - local[axis]) / ray[axis];
            if (entry > exit)
                std::swap(entry, exit);
            near = qMax(near, entry);
            far = qMin(far, exit);
            if (near > far)
                return -1;
        }
    }
    return near;
}
