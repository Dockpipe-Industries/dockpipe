#include "DirtBikeModel.h"
#include "DirtBikeRiderPose.h"
#include "DirtBikeSteeringRig.h"
#include "SpatialAssetLibrary.h"
#include "SpatialMeshLoft.h"
#include <QDebug>
#include <cmath>
#include <optional>
#include <algorithm>
#include <QQuaternion>
#include <array>
#include <QVector4D>

namespace {
constexpr float pi = 3.14159265358979323846f;
class Builder {
public:
    QVector<SpatialVehicleFace> faces;
    QColor bodyPaint;
    QString error;
    bool asset(const QString &id, const QColor &paint, int part = DirtBikeModel::Frame)
    {
        const auto mesh = SpatialAssetLibrary::model("moto-" + id);
        if (mesh.faces.isEmpty() || faces.size() + mesh.faces.size() > DirtBikeModel::maximumFaces)
            return false;
        for (int index = 0; index < mesh.faces.size(); ++index) {
            QVector<QVector3D> points;
            QVector<QVector3D> normals;
            for (int vertex : mesh.faces[index]) {
                points.append(mesh.vertices[vertex]);
                if (!mesh.normals.isEmpty())
                    normals.append(mesh.normals[vertex]);
            }
            auto color = mesh.faceColors[index];
            if (color == QColor("#ed6837"))
                color = paint;
            face(points, color, part, {}, normals);
            if (id == "engine" || id == "cylinder" || id == "exhaust")
                faces.last().finish = SpatialSurfaceFinish::Metal;
            else if (id == "seat") {
                faces.last().material = SpatialSurfaceMaterial::Rubber;
                faces.last().finish = SpatialSurfaceFinish::Standard;
            } else {
                faces.last().finish = SpatialSurfaceFinish::Painted;
            }
        }
        return true;
    }
    void face(QVector<QVector3D> points, QColor color, int part = DirtBikeModel::Frame,
        std::optional<QVector3D> interior = {}, QVector<QVector3D> normals = {})
    {
        if (!interior && (part == DirtBikeModel::FrontWheel || part == DirtBikeModel::RearWheel))
            interior = QVector3D(0, 35, part == DirtBikeModel::FrontWheel ? 74 : -74);
        auto normal = QVector3D::crossProduct(points[1] - points[0], points[2] - points[0]).normalized();
        if (interior && QVector3D::dotProduct(normal, points[0] - *interior) < 0) {
            std::reverse(points.begin(), points.end());
            std::reverse(normals.begin(), normals.end());
            normal = -normal;
        }
        const QColor albedo = color;
        const float light = .48f + .52f * qMax(0.f, QVector3D::dotProduct(normal, QVector3D(-.4f, .85f, -.3f).normalized()));
        color = QColor::fromRgbF(color.redF()*light, color.greenF()*light, color.blueF()*light);
        SpatialSurfaceMaterial material = SpatialSurfaceMaterial::Plain;
        SpatialSurfaceFinish finish = SpatialSurfaceFinish::Standard;
        // Use authored albedo, never baked face lighting, to classify materials.
        // Otherwise one metal rim becomes rubber on its shadow-facing facets.
        if (part == DirtBikeModel::FrontWheel || part == DirtBikeModel::RearWheel) {
            if (albedo.lightnessF() < .22f)
                material = SpatialSurfaceMaterial::Rubber;
            else
                finish = SpatialSurfaceFinish::Metal;
        } else if (DirtBikeModel::riderPart(part)) {
            if (part == DirtBikeModel::Helmet)
                finish = SpatialSurfaceFinish::Painted;
            else if (part == DirtBikeModel::LeftBoot || part == DirtBikeModel::RightBoot) {
                if (albedo.lightnessF() < .3f)
                    material = SpatialSurfaceMaterial::Rubber;
                else
                    finish = SpatialSurfaceFinish::Painted;
            } else {
                material = SpatialSurfaceMaterial::Fabric;
            }
        } else if (albedo == QColor("#b3bdc3") || albedo == QColor("#d7b568")
            || albedo == QColor("#485257")) {
            finish = SpatialSurfaceFinish::Metal;
        } else if (albedo == bodyPaint || albedo == bodyPaint.lighter(115)
            || albedo == QColor("#efe3bf") || albedo == QColor("#edc23b")) {
            finish = SpatialSurfaceFinish::Painted;
        } else if (albedo == QColor("#17242b")) {
            material = SpatialSurfaceMaterial::Rubber;
        }
        faces.append({points, color, part, material, interior.has_value(), albedo, normals, finish});
    }
    void hull(QVector3D low, QVector3D high, float taper, QColor color, int part = DirtBikeModel::Frame)
    {
        const float left = low.x();
        const float right = high.x();
        const float bottom = low.y();
        const float top = high.y();
        const float back = low.z();
        const float front = high.z();
        const float inset = (right - left) * taper;
        const float endInset = qMin(4.f, (front - back) * .2f);
        const QVector<QVector3D> points{
            {left, bottom, back}, {right, bottom, back},
            {right, bottom, front}, {left, bottom, front},
            {left + inset, top, back + endInset}, {right - inset, top, back + endInset},
            {right - inset, top, front - endInset}, {left + inset, top, front - endInset}};
        for (const auto &indices : QVector<QVector<int>>{{0, 1, 5, 4}, {3, 7, 6, 2}, {0, 4, 7, 3}, {1, 2, 6, 5}, {4, 5, 6, 7}, {0, 3, 2, 1}}) {
            QVector<QVector3D> polygon;
            for (int index : indices)
                polygon.append(points[index]);
            face(polygon, color, part, (low + high) / 2);
        }
    }
    void tube(QVector3D from, QVector3D to, float radius, QColor color,
        int part = DirtBikeModel::Frame, int sides = 6, float endRadius = -1)
    {
        const auto direction = (to-from).normalized();
        if (endRadius < 0)
            endRadius = radius;
        const float slope = (radius - endRadius) / (to - from).length();
        auto right = QVector3D::crossProduct(direction, {0, 1, 0});
        if (right.lengthSquared() < .01f)
            right = QVector3D::crossProduct(direction, {1, 0, 0});
        right.normalize();
        const auto up = QVector3D::crossProduct(direction, right).normalized();
        for (int side = 0; side < sides; ++side) {
            const float a = side * 2*pi/sides, b = (side+1)*2*pi/sides;
            const auto first = right*std::cos(a)+up*std::sin(a);
            const auto second = right*std::cos(b)+up*std::sin(b);
            const auto firstNormal = (first + direction * slope).normalized();
            const auto secondNormal = (second + direction * slope).normalized();
            face({from+first*radius, to+first*endRadius, to+second*endRadius, from+second*radius},
                color, part, (from + to) / 2, {firstNormal, firstNormal, secondNormal, secondNormal});
        }
    }
    void loft(const QVector<SpatialLoftSection> &sections, const QVector3D &across,
        const QVector3D &depth, int sides, QColor color, int part,
        SpatialSurfaceFinish finish = SpatialSurfaceFinish::Standard)
    {
        if (!error.isEmpty())
            return;
        SpatialVehicleFace style;
        style.part = part;
        const auto mesh = SpatialMeshLoft::build(sections, across, depth, sides,
            DirtBikeModel::maximumFaces - faces.size(), style);
        if (!mesh.succeeded()) {
            error = mesh.error;
            return;
        }
        for (const auto &surface : mesh.faces) {
            face(surface.vertices, color, part, {}, surface.normals);
            if (finish != SpatialSurfaceFinish::Standard)
                faces.last().finish = finish;
        }
    }
    void shell(const QVector<QVector4D> &sections, QColor color, int part = DirtBikeModel::Frame)
    {
        const int first = faces.size();
        QVector<SpatialLoftSection> rings;
        for (const auto &section : sections)
            rings.append({{0, section.y() - section.w() * .5f, section.x()}, section.z(), section.w() * .5f});
        loft(rings, {1, 0, 0}, {0, 1, 0}, 12, color, part, SpatialSurfaceFinish::Painted);
        if (color == QColor("#17242b")) {
            for (int index = first; index < faces.size(); ++index) {
                faces[index].material = SpatialSurfaceMaterial::Fabric;
                faces[index].finish = SpatialSurfaceFinish::Standard;
            }
        }
    }
    void garment(const QVector3D &from, const QVector3D &to, float width,
        float endWidth, QColor color, int part)
    {
        const auto direction = (to - from).normalized();
        auto across = QVector3D::crossProduct(direction, {0, 1, 0});
        if (across.lengthSquared() < .01f)
            across = QVector3D::crossProduct(direction, {1, 0, 0});
        across.normalize();
        const auto depth = QVector3D::crossProduct(direction, across).normalized();
        // A tailored silhouette: rounded sleeve head, muscle/calf volume and
        // a narrow cuff. Sections stay in the bone's existing rest frame.
        const std::array<float, 6> offsets{0, .12f, .35f, .64f, .88f, 1};
        const std::array<float, 6> fullness{.82f, 1, 1.04f, .97f, .88f, .8f};
        QVector<SpatialLoftSection> rings;
        for (int index = 0; index < int(offsets.size()); ++index) {
            const float t = offsets[index];
            const float radius = (width * (1 - t) + endWidth * t) * fullness[index];
            rings.append({from + (to - from) * t, radius, radius * .85f});
        }
        loft(rings, across, depth, 10, color, part);
    }
    void panel(const QVector<QVector3D> &outline, QColor color, int part = DirtBikeModel::Frame)
    {
        // Convex panel outlines, fan-triangulated so no concave quad is handed
        // to either renderer. A small rolled edge gives each plastic thickness.
        QVector3D center;
        for (const auto &point : outline)
            center += point;
        center /= outline.size();
        const auto normal = QVector3D::crossProduct(outline[1] - outline[0], outline[2] - outline[0]).normalized();
        const auto crown = center + normal * .8f;
        QVector<QVector3D> normals;
        for (int index = 0; index < outline.size(); ++index)
            normals.append(QVector3D::crossProduct(outline[index] - crown,
                outline[(index + 1) % outline.size()] - crown).normalized());
        for (int index = 0; index < outline.size(); ++index) {
            const auto &a = outline[index];
            const auto &b = outline[(index + 1) % outline.size()];
            const auto firstNormal = (normals[(index + outline.size() - 1) % outline.size()] + normals[index]).normalized();
            const auto secondNormal = (normals[index] + normals[(index + 1) % outline.size()]).normalized();
            face({crown, a, b}, color, part, {}, {normal, firstNormal, secondNormal});
            faces.last().finish = SpatialSurfaceFinish::Painted;
            face({a, a - normal * 1.2f, b - normal * 1.2f, b}, color.darker(120), part);
        }
    }
    void raceBody(QColor paint)
    {
        const QColor white("#f0f3f4"), dark("#17242b"), metal("#b3bdc3");
        // The fuel cell stays underneath the shrouds and long gripper seat.
        hull({-13, 82, -14}, {13, 104, 32}, .12f, QColor("#242c32"));
        shell({{-90, 105, 7, 2}, {-72, 109, 12, 3}, {-45, 111, 13, 4},
            {-15, 111, 12, 4}, {16, 108, 9, 4}, {29, 107, 6, 3}}, dark);
        shell({{-116, 108, 6, 1.6f}, {-100, 109, 11, 2}, {-77, 108, 14, 3},
            {-52, 104, 14, 4}}, paint);
        shell({{42, 93, 11, 3}, {62, 94, 13, 3}, {87, 94, 12, 3},
            {111, 90, 10, 2}, {125, 86, 6, 1.5f}}, paint, DirtBikeModel::Fork);
        // Flat race plate replaces the old round trail-bike headlamp.
        panel({{-12, 98, 63}, {12, 98, 63}, {16, 119, 54},
            {12, 125, 52}, {-12, 125, 52}, {-16, 119, 54}}, white, DirtBikeModel::Fork);
        panel({{-10, 121, 53.8f}, {10, 121, 53.8f}, {10, 124, 52.6f}, {-10, 124, 52.6f}}, paint, DirtBikeModel::Fork);
        // Race number 11: painted, offset from the sloping plate to avoid z fighting.
        for (float x : {-5.f, 4.f})
            panel({{x, 102, 62.1f}, {x + 3, 102, 62.1f},
                {x + 3, 117, 55.7f}, {x, 117, 55.7f}}, dark, DirtBikeModel::Fork);
        for (float side : {-1.f, 1.f}) {
            // Compact radiators with visible slats below angular tank shrouds.
            hull({side * 17 - 4, 74, 20}, {side * 17 + 4, 96, 34}, .05f, dark);
            for (int fin = 0; fin < 6; ++fin)
                tube({side * 21, 76.f + fin * 3, 21}, {side * 21, 76.f + fin * 3, 32},
                    .6f, metal, DirtBikeModel::Frame, 4);
            QVector<QVector3D> shroud{{side * 14, 105, -9}, {side * 20, 106, 29},
                {side * 23, 98, 43}, {side * 22, 78, 33}, {side * 18, 83, 8}};
            QVector<QVector3D> sidePlate{{side * 15, 101, -81}, {side * 17, 102, -27},
                {side * 19, 88, -13}, {side * 18, 78, -35}, {side * 15, 86, -72}};
            // These authored outlines face outward on the right. Mirror winding.
            if (side < 0) {
                std::reverse(shroud.begin(), shroud.end());
                std::reverse(sidePlate.begin(), sidePlate.end());
            }
            panel(shroud, paint);
            panel(sidePlate, white);
            QVector<QVector3D> graphic{{side * 20.4f, 99, 13}, {side * 22.8f, 99, 36},
                {side * 22.2f, 95, 38}, {side * 20.5f, 90, 14}};
            if (side < 0)
                std::reverse(graphic.begin(), graphic.end());
            panel(graphic, QColor("#242c32"));
            for (float z : {-53.f, -43.f}) {
                QVector<QVector3D> numeral{{side * 18.9f, 85, z}, {side * 18.9f, 85, z + 3},
                    {side * 18.2f, 97, z + 3}, {side * 18.2f, 97, z}};
                if (side > 0)
                    std::reverse(numeral.begin(), numeral.end());
                panel(numeral, QColor("#242c32"));
            }
            // Fork guards move with the lower leg, separate from the shrouds.
            const auto guardBottom = DirtBikeSteeringRig::forkPoint(side > 0, 40);
            const auto guardTop = DirtBikeSteeringRig::forkPoint(side > 0, 69);
            loft({{guardBottom, 4, 4}, {guardTop, 4, 5}}, {1, 0, 0}, {0, 0, -1},
                6, white, DirtBikeModel::ForkLower, SpatialSurfaceFinish::Painted);
            // Wide alloy spars and a boxed swingarm, rather than a tube triangle.
            tube({side * 11, 98, 34}, {side * 16, 64, -20}, 5, metal, DirtBikeModel::Frame, 4, 6);
            tube({side * 16, 64, -20}, {side * 11, 43, -10}, 4, metal, DirtBikeModel::Frame, 4);
            tube({side * 12, 47, -12}, {side * 10, 35, -74}, 4, metal, DirtBikeModel::Swingarm, 4, 3);
            tube({side * 12, 84, -25}, {side * 12, 100, -75}, 2, metal);
        }
        // Short high-mounted silencer and swept header, tucked under the tail.
        tube({17, 65, 17}, {23, 62, 34}, 3, metal, DirtBikeModel::Frame, 12);
        tube({23, 62, 34}, {23, 79, -28}, 3, metal, DirtBikeModel::Frame, 12);
        tube({23, 79, -28}, {20, 91, -50}, 3, metal, DirtBikeModel::Frame, 12);
        loft({{{20, 93, -88}, 4, 5}, {{20, 93, -84}, 5, 7},
            {{20, 92, -55}, 5, 7}, {{20, 91, -50}, 3, 4}},
            {1, 0, 0}, {0, 1, 0}, 12, metal, DirtBikeModel::Frame, SpatialSurfaceFinish::Metal);
        tube({20, 93, -88}, {20, 93, -90}, 3, dark, DirtBikeModel::Frame, 12);
        // Fuel cap and handlebar pad remain separate physical parts.
        tube({0, 106, 25}, {0, 109, 25}, 3.5f, dark, DirtBikeModel::Frame, 12);
        hull({-12, 124, 35}, {12, 130, 42}, .1f, dark, DirtBikeModel::Handlebars);
    }
    void frontAssembly()
    {
        const QColor alloy("#b3bdc3"), rubber("#17242b"), gold("#d7b568");
        // Generate a mechanically related assembly from the rig's anchors.
        // Upper and lower tubes share a centreline; both triple clamps span it.
        for (bool right : {false, true}) {
            tube(DirtBikeSteeringRig::forkPoint(right, 116), DirtBikeSteeringRig::forkPoint(right, 60),
                3.6f, gold, DirtBikeModel::Fork, 12);
            tube(DirtBikeSteeringRig::forkPoint(right, 96), DirtBikeSteeringRig::forkPoint(right, 35),
                2.5f, alloy, DirtBikeModel::ForkLower, 12);
        }
        for (float height : {96.f, 113.f})
            tube(DirtBikeSteeringRig::forkPoint(false, height), DirtBikeSteeringRig::forkPoint(true, height),
                3, alloy, DirtBikeModel::Fork, 6);
        const auto stem = DirtBikeSteeringRig::stem();
        const auto axis = DirtBikeSteeringRig::axis();
        tube(stem - axis * 8, stem + axis * 10, 4.5f, alloy, DirtBikeModel::Frame, 8);
        tube(stem - axis * 9, stem + axis * 24, 2.5f, alloy, DirtBikeModel::Fork, 8);
        tube({-40, 127, 40}, {-14, 122, 37}, 1.8f, alloy, DirtBikeModel::Handlebars, 10);
        tube({-14, 122, 37}, {14, 122, 37}, 1.8f, alloy, DirtBikeModel::Handlebars, 10);
        tube({14, 122, 37}, {40, 127, 40}, 1.8f, alloy, DirtBikeModel::Handlebars, 10);
        for (bool right : {false, true}) {
            const auto grip = DirtBikeSteeringRig::grip(right);
            tube(grip - QVector3D(7.5f, 0, 0), grip + QVector3D(7.5f, 0, 0),
                2.7f, rubber, DirtBikeModel::Handlebars, 8);
            const float side = right ? 1.f : -1.f;
            tube({side * 27, 125, 43}, {side * 40, 125, 46}, 1,
                alloy, DirtBikeModel::Handlebars, 4);
        }
    }
    void drivetrain()
    {
        const QColor alloy("#b3bdc3"), dark("#485257"), rubber("#17242b");
        // Round clutch/ignition covers, raised lips and bolt heads. Their
        // cross-section advances along X rather than using a painted box.
        for (float side : {-1.f, 1.f}) {
            QVector<SpatialLoftSection> rings;
            for (float x : {16.f, 18.f, 19.f})
                rings.append({{side * x, 57, -1}, x == 18 ? 12.f : 11.f, x == 18 ? 12.f : 11.f});
            if (side < 0)
                std::reverse(rings.begin(), rings.end());
            loft(rings, {0, 1, 0}, {0, 0, 1}, 16, dark, DirtBikeModel::Frame, SpatialSurfaceFinish::Metal);
            for (int bolt = 0; bolt < 6; ++bolt) {
                const float angle = bolt * pi / 3;
                const auto center = QVector3D(side * 19.2f, 57 + 9 * std::sin(angle), -1 + 9 * std::cos(angle));
                tube(center, center + QVector3D(side, 0, 0), .9f, alloy, DirtBikeModel::Frame, 6);
            }
        }
        // Fixed caliper on the lower fork, rotating disc remains on the wheel.
        hull({8, 37, 60}, {14, 48, 68}, .08f, dark, DirtBikeModel::ForkLower);
        // Chain follows the swingarm group; two runs and compact sprocket teeth.
        for (float height : {29.f, 41.f})
            tube({-10, height, -74}, {-13, 46 + (height - 35) * .45f, -14}, .85f,
                dark, DirtBikeModel::Swingarm, 4);
        for (int tooth = 0; tooth < 16; ++tooth) {
            const float angle = tooth * pi / 8;
            const float next = angle + .2f;
            const auto point = [&](float radius, float theta) {
                return QVector3D(-8, 35 + radius * std::sin(theta), -74 + radius * std::cos(theta));
            };
            face({point(7, angle), point(12, angle), point(12, next), point(7, next)},
                alloy, DirtBikeModel::RearWheel);
        }
        // A coil spring and damper sit below the seat, independently of tyre detail.
        const QVector3D from(0, 79, -31), to(0, 48, -48);
        const auto axis = (to - from).normalized();
        const QVector3D across(1, 0, 0);
        const auto depth = QVector3D::crossProduct(axis, across).normalized();
        QVector3D previous;
        constexpr int steps = 24;
        for (int step = 0; step <= steps; ++step) {
            const float t = float(step) / steps;
            const float angle = t * 2 * pi * 6;
            const auto point = from + (to - from) * t
                + (across * std::cos(angle) + depth * std::sin(angle)) * 5;
            if (step > 0)
                tube(previous, point, .8f, QColor("#edc23b"), DirtBikeModel::Frame, 4);
            previous = point;
        }
        tube(from, to, 2, alloy, DirtBikeModel::Frame, 8);
        // Ribbed gripper seat surface is authored as fabric; retain a clean silhouette.
        for (int rib = 0; rib < 7; ++rib) {
            const float z = -65 + rib * 9;
            tube({-10, 111.2f, z}, {10, 111.2f, z}, .45f, rubber, DirtBikeModel::Frame, 4);
        }
    }
    void jersey(QColor suit, QColor trim)
    {
        const int first = faces.size();
        loft({{{0, 108, -25}, 18, 13}, {{0, 115, -24}, 18, 12},
            {{0, 126, -18}, 16, 11}, {{0, 138, -10}, 20, 12},
            {{0, 146, -3}, 23, 12}, {{0, 151, 0}, 20, 10},
            {{0, 155, 2}, 10, 8}}, {1, 0, 0}, {0, 0, -1}, 20, suit, DirtBikeModel::Rider);
        for (int index = first; index < faces.size(); ++index) {
            auto &surface = faces[index];
            QVector3D center;
            for (const auto &point : surface.vertices)
                center += point;
            center /= surface.vertices.size();
            // Sewn side panels and shoulder yoke, coloured in the same mesh.
            if ((std::abs(center.x()) > 13 && center.y() > 116 && center.y() < 141)
                || center.y() > 147) {
                surface.albedo = trim;
                surface.color = trim;
            }
        }
        // Collar is deliberately small; the helmet no longer floats above a ball.
        loft({{{0, 153, 2}, 8, 7}, {{0, 159, 3}, 7, 7}},
            {1, 0, 0}, {0, 0, -1}, 12, QColor("#17242b"), DirtBikeModel::Rider);
    }
    void boot(const QVector3D &ankle, float side, int part)
    {
        const QColor shell("#e5e9e7"), rubber("#17242b"), buckle("#88969c");
        const float x = ankle.x();
        // Flat sole, forward toe box, narrower ankle and flared shin guard.
        loft({{{x, 49, -10}, 7, 17}, {{x, 53, -10}, 7.5f, 17},
            {{x, 57, -15}, 6.5f, 10}, {{x, 64, -18}, 6, 7},
            {{x, 74, -15}, 7, 7}, {{x, 80, -12}, 7.5f, 6}},
            {1, 0, 0}, {0, 0, -1}, 12, shell, part, SpatialSurfaceFinish::Painted);
        hull({x - 7.5f, 47, -27}, {x + 7.5f, 50, 7}, .04f, rubber, part);
        for (int strap = 0; strap < 4; ++strap) {
            const float height = 56 + strap * 6;
            hull({x + side * 7 - 1, height, -22}, {x + side * 7 + 1, height + 2, -9},
                .02f, rubber, part);
            hull({x + side * 8 - 1, height, -13}, {x + side * 8 + 1, height + 2, -8},
                .02f, buckle, part);
        }
    }
    void ellipsoid(QVector3D center, QVector3D radii, QColor color, int part, int rings = 6, int sides = 12)
    {
        const auto point = [&](int ring, int side) {
            const float latitude = -pi/2 + ring*pi/rings;
            const float longitude = side*2*pi/sides;
            return center + QVector3D(radii.x()*std::cos(latitude)*std::cos(longitude),
                radii.y()*std::sin(latitude), radii.z()*std::cos(latitude)*std::sin(longitude));
        };
        const auto surface = [&](QVector<QVector3D> points) {
            QVector<QVector3D> normals;
            for (const auto &position : points) {
                const auto offset = position - center;
                normals.append(QVector3D(offset.x() / (radii.x() * radii.x()),
                    offset.y() / (radii.y() * radii.y()), offset.z() / (radii.z() * radii.z())).normalized());
            }
            face(points, color, part, center, normals);
        };
        for (int ring = 0; ring < rings; ++ring) {
            for (int side = 0; side < sides; ++side) {
                if (ring == 0) {
                    surface({point(0, 0), point(1, side), point(1, side+1)});
                } else if (ring + 1 == rings) {
                    surface({point(ring, side), point(rings, 0), point(ring, side+1)});
                } else {
                    surface({point(ring, side), point(ring+1, side), point(ring+1, side+1), point(ring, side+1)});
                }
            }
        }
    }
    void helmet(QVector3D center)
    {
        const QColor shell("#f0f3f4"), trim("#ed6837"), dark("#17242b");
        // The lower shell projects into a chin guard while the crown sweeps
        // back. Leave a real opening for goggles instead of hiding a lens
        // inside a complete ellipsoid.
        QVector<SpatialLoftSection> rings;
        for (const auto &section : QVector<QVector4D>{{-16, 4, 8, 12}, {-12, 5, 12, 18},
            {-5, 1, 14, 21}, {7, -3, 15, 18}, {14, -6, 12, 14},
            {18, -6, 7, 8}, {20, -6, 1, 2}})
            rings.append({center + QVector3D(0, section.x(), section.y()), section.z(), section.w()});
        SpatialVehicleFace style;
        style.part = DirtBikeModel::Helmet;
        const auto mesh = SpatialMeshLoft::build(rings, {1, 0, 0}, {0, 0, -1}, 20,
            DirtBikeModel::maximumFaces - faces.size(), style);
        if (!mesh.succeeded()) {
            error = mesh.error;
            return;
        }
        for (int index = 0; index < mesh.faces.size(); ++index) {
            const int ring = index / 20;
            const int side = index % 20;
            if (ring == 2 && side >= 11 && side <= 18)
                continue;
            const auto &surface = mesh.faces[index];
            QColor color = shell;
            if (ring == 0 || (ring == 3 && (side <= 2 || side >= 18 || (side >= 8 && side <= 11))))
                color = trim;
            face(surface.vertices, color, DirtBikeModel::Helmet, {}, surface.normals);
        }
        // Wrap the recessed lens and its rubber seal around the face opening.
        // Rails sit above/below it, with a lower projecting vented chin bar.
        const QVector<QPointF> lens{{-13, 8}, {-8, 16}, {0, 19}, {8, 16}, {13, 8}};
        for (int segment = 0; segment + 1 < lens.size(); ++segment) {
            const auto a = lens[segment];
            const auto b = lens[segment + 1];
            const auto point = [&](QPointF profile, float height, float offset = 0) {
                return center + QVector3D(profile.x(), height, profile.y() + offset);
            };
            face({point(a, -3), point(b, -3), point(b, 6), point(a, 6)},
                QColor("#17313e"), DirtBikeModel::Helmet);
            for (float height : {-5.f, 6.f})
                face({point(a, height, .5f), point(b, height, .5f),
                    point(b, height + 2, .5f), point(a, height + 2, .5f)}, dark, DirtBikeModel::Helmet);
        }
        panel({center + QVector3D(-7, -12, 24), center + QVector3D(7, -12, 24),
            center + QVector3D(7, -7, 23), center + QVector3D(-7, -7, 23)}, dark, DirtBikeModel::Helmet);
        for (float x : {-4.f, 0.f, 4.f})
            tube(center + QVector3D(x, -12, 24.8f), center + QVector3D(x, -7, 24),
                .5f, shell, DirtBikeModel::Helmet, 4);
        for (int strip = 0; strip < 4; ++strip) {
            const float left = -16 + strip * 8;
            const float right = left + 8;
            const auto peak = [&](float x, float z) {
                return center + QVector3D(x, 12 - std::abs(x) * .14f, z - std::abs(x) * .35f);
            };
            panel({peak(left, 6), peak(right, 6), peak(right, 31), peak(left, 31)},
                strip == 0 || strip == 3 ? trim : shell, DirtBikeModel::Helmet);
        }
    }
    void wheel(float z, int part)
    {
        const QVector<QPointF> profile{{-8, 24}, {-10, 29}, {-8, 33}, {-4, 35},
            {4, 35}, {8, 33}, {10, 29}, {8, 24}};
        constexpr int segments = 32;
        for (int segment = 0; segment < segments; ++segment) {
            const float a = segment * 2 * pi / segments;
            const float b = (segment + 1) * 2 * pi / segments;
            const auto point = [&](float x, float radius, float angle) {
                return QVector3D(x * (part == DirtBikeModel::FrontWheel ? .48f : .65f), 35+radius*std::sin(angle), z+radius*std::cos(angle));
            };
            for (int band = 0; band + 1 < profile.size(); ++band) {
                const auto first = profile[band];
                const auto second = profile[band+1];
                const auto normal = [&](int index, float angle) {
                    const auto tangent = profile[qMin(index + 1, int(profile.size()) - 1)]
                        - profile[qMax(index - 1, 0)];
                    const float width = part == DirtBikeModel::FrontWheel ? .48f : .65f;
                    return QVector3D(-tangent.y(), tangent.x() * width * std::sin(angle),
                        tangent.x() * width * std::cos(angle)).normalized();
                };
                face({point(first.x(), first.y(), a), point(second.x(), second.y(), a),
                    point(second.x(), second.y(), b), point(first.x(), first.y(), b)}, QColor("#252a2b"), part,
                    {}, {normal(band, a), normal(band + 1, a), normal(band + 1, b), normal(band, b)});
            }
            for (float side : {-1.f, 1.f})
                face({point(side*8, 26, a), point(side*8, 23, a), point(side*8, 23, b), point(side*8, 26, b)}, QColor("#b5c1c4"), part);
            // Staggered centre/shoulder knobs have actual side walls, so
            // light and silhouette describe tread even without a texture.
            const QVector<float> rows = segment % 4 == 0 ? QVector<float>{-6, 3} : QVector<float>{-2};
            for (float left : rows) {
                const float right = left + 4;
                const float start = a + .025f;
                const float end = b - .025f;
                const QVector<QVector3D> bottom{point(left, 34.5f, start), point(right, 34.5f, start),
                    point(right, 34.5f, end), point(left, 34.5f, end)};
                const QVector<QVector3D> top{point(left, 36, start), point(right, 36, start),
                    point(right, 36, end), point(left, 36, end)};
                face(top, QColor("#343839"), part);
                for (int edge = 0; edge < 4; ++edge) {
                    const int next = (edge + 1) % 4;
                    face({bottom[edge], bottom[next], top[next], top[edge]}, QColor("#343839"), part);
                }
            }
        }
        tube({-10, 35, z}, {10, 35, z}, 7, QColor("#747f85"), part);
        for (int spoke = 0; spoke < 24; ++spoke) {
            const float angle = spoke * 2 * pi / 24;
            const float hubAngle = angle + (spoke % 2 ? .48f : -.48f);
            const float side = spoke % 2 ? -1.f : 1.f;
            tube({side * 4, 35 + 5 * std::sin(hubAngle), z + 5 * std::cos(hubAngle)},
                {side * 2, 35 + 24 * std::sin(angle), z + 24 * std::cos(angle)},
                .4f, QColor("#bac5c6"), part, 4);
        }
        // Brake rotor ring, open in the centre; it rotates with its wheel.
        for (int segment = 0; segment < 16; ++segment) {
            const float a = segment * pi / 8;
            const float b = (segment + 1) * pi / 8;
            const auto point = [&](float radius, float angle) {
                return QVector3D(7, 35 + radius * std::sin(angle), z + radius * std::cos(angle));
            };
            face({point(9, a), point(14, a), point(14, b), point(9, b)}, QColor("#969fa5"), part);
        }
    }
};
}

QVector<SpatialVehicleFace> DirtBikeModel::build(const QColor &paint, bool rider)
{
    Builder builder;
    builder.bodyPaint = paint;
    builder.wheel(74, FrontWheel);
    builder.wheel(-74, RearWheel);
    const QColor frame("#b3bdc3"), rubber("#17242b"), suit("#303943"), cream("#efe3bf");
    builder.frontAssembly();
    if (!builder.asset("engine", paint))
        builder.hull({-17, 43, -18}, {17, 75, 24}, .15f, QColor("#485257"));
    builder.asset("cylinder", paint);
    for (int fin = 0; fin < 3; ++fin)
        builder.hull({-9, 61.f+fin*5, -3}, {9, 63.f+fin*5, 12}, .05f, frame);
    builder.raceBody(paint);
    builder.drivetrain();
    for (float side : {-1.f, 1.f}) {
        builder.tube({side*12, 47, -15}, {side*32, 47, -15}, 3, rubber);
    }
    if (rider) {
        builder.jersey(suit, cream);
        builder.ellipsoid({0, 113, -24}, {19, 13, 16}, rubber, Rider, 4, 10);
        builder.helmet({0, 176, 7});
        for (float side : {-1.f, 1.f}) {
            const bool right = side > 0;
            const auto shoulder = DirtBikeRiderPose::shoulder(right);
            const auto elbow = DirtBikeRiderPose::elbow(right);
            const auto hand = DirtBikeRiderPose::grip(right);
            const int upperArm = right ? RightUpperArm : LeftUpperArm;
            const int forearm = right ? RightForearm : LeftForearm;
            const int glove = right ? RightHand : LeftHand;
            builder.garment(shoulder, elbow, 8, 6.5f, suit, upperArm);
            builder.ellipsoid(elbow, {6.5f, 6.5f, 6.5f}, suit, upperArm, 4, 8);
            builder.garment(elbow, hand, 6.5f, 4.5f, cream, forearm);
            builder.hull(hand + QVector3D(-6, -5, -6), hand + QVector3D(6, 7, 7), .1f, rubber, glove);
            const auto hip = DirtBikeRiderPose::hip(right);
            const auto knee = DirtBikeRiderPose::knee(right);
            const auto ankle = DirtBikeRiderPose::ankle(right);
            const int thigh = right ? RightThigh : LeftThigh;
            const int shin = right ? RightShin : LeftShin;
            const int boot = right ? RightBoot : LeftBoot;
            builder.garment(hip, knee, 11, 8, suit, thigh);
            builder.ellipsoid(knee, {8, 8, 8}, rubber, thigh, 6, 12);
            builder.garment(knee, ankle, 8, 6, suit, shin);
            builder.boot(ankle, side, boot);
        }
    }
    if (!builder.error.isEmpty() || builder.faces.size() > maximumFaces) {
        qWarning() << "Dirt-bike mesh admission failed:" << builder.error << builder.faces.size();
        return {};
    }
    // Author texture coordinates once in the rest model. Wheel spin, rider
    // articulation and world movement transform geometry without moving its
    // weave/tread texture through the surface. Terrain remains world-aligned.
    for (auto &face : builder.faces) {
        if (face.material == SpatialSurfaceMaterial::Plain)
            continue;
        const auto normal = QVector3D::crossProduct(face.vertices[1] - face.vertices[0],
            face.vertices[2] - face.vertices[0]).normalized();
        const float scale = SpatialSurfaceTextures::scale(face.material);
        face.uvs.reserve(face.vertices.size());
        for (const auto &point : face.vertices) {
            if (std::abs(normal.y()) >= std::abs(normal.x()) && std::abs(normal.y()) >= std::abs(normal.z()))
                face.uvs.append({point.x() / scale, point.z() / scale});
            else if (std::abs(normal.x()) > std::abs(normal.z()))
                face.uvs.append({point.z() / scale, point.y() / scale});
            else
                face.uvs.append({point.x() / scale, point.y() / scale});
        }
    }
    return builder.faces;
}

QVector3D DirtBikeModel::steerPoint(const QVector3D &point, float angle)
{
    return DirtBikeSteeringRig::steerPoint(point, angle);
}
