#include "DirtBikeTrackArt.h"
#include "SpatialFoliagePlacement.h"
#include <cmath>
#include <algorithm>

namespace {
constexpr float pi = 3.14159265358979323846f;
}

void DirtBikeTrackArt::decorate(SpatialInterior &room)
{
    // Covers use the existing fabric material and collision blocks. Only the
    // track-facing side and top need extra geometry; half-centimetre offsets
    // prevent coplanar depth flicker. Placement stays outside the riding line.
    SpatialMesh covers;
    covers.color = QColor("#eee4cd");
    covers.material = SpatialSurfaceMaterial::Fabric;
    const auto coverFace = [&](const QVector<QVector3D> &points, const QColor &color) {
        const int first = covers.vertices.size();
        covers.vertices += points;
        covers.faces.append({first, first + 1, first + 2, first + 3});
        covers.faceColors.append(color);
    };
    for (float side : {-1.f, 1.f}) {
        for (int index = 0; index < 9; ++index) {
            const float x = side * 3350;
            const float z = -8000 + index * 2000;
            const float inside = x - side * 50.5f;
            const QColor color(index % 2 ? "#eee4cd" : "#ce5635");
            QVector<QVector3D> face{{inside,0,z-110}, {inside,90,z-110},
                {inside,90,z+110}, {inside,0,z+110}};
            if (side > 0)
                std::reverse(face.begin(), face.end());
            coverFace(face, color);
            coverFace({{x-50,90.5f,z-110}, {x-50,90.5f,z+110},
                {x+50,90.5f,z+110}, {x+50,90.5f,z-110}}, color);
        }
    }
    room.meshes.append(covers);

    // A hip-roof fabric canopy, shared with the same software/native mesh path.
    // The eave ceiling and posts retain their existing collision role; the
    // raised fabric is visual detail, not a new walkable roof surface.
    SpatialMesh canopy;
    canopy.material = SpatialSurfaceMaterial::Fabric;
    canopy.color = QColor("#ddd3bd");
    canopy.vertices = {{-6250,440,-8850}, {-4350,440,-8850},
        {-4350,440,-7550}, {-6250,440,-7550},
        {-5300,620,-8550}, {-5300,620,-7850}};
    canopy.faces = {{0,4,1}, {1,4,5,2}, {2,5,3}, {3,5,4,0}};
    canopy.faceColors = {QColor("#be5a37"), canopy.color, QColor("#be5a37"), canopy.color};
    room.meshes.append(canopy);

    SpatialMesh pitPad;
    pitPad.material = SpatialSurfaceMaterial::Rock;
    pitPad.color = QColor("#a5a39a");
    pitPad.vertices = {{-6250,2.5f,-8850}, {-6250,2.5f,-7550},
        {-4350,2.5f,-7550}, {-4350,2.5f,-8850}};
    pitPad.faces = {{0,1,2,3}};
    room.meshes.append(pitPad);

    QVector<QVector3D> path;
    for (int step = 0; step < 7; ++step)
        path.append({-2700, 2, -8600.f+step*(17200.f/7)});
    for (int step = 0; step < 10; ++step) {
        const float angle = pi-step*pi/10;
        path.append({2700*std::cos(angle), 2, 8600+2700*std::sin(angle)});
    }
    for (int step = 0; step < 7; ++step)
        path.append({2700, 2, 8600.f-step*(17200.f/7)});
    for (int step = 0; step < 10; ++step) {
        const float angle = -step*pi/10;
        path.append({2700*std::cos(angle), 2, -8600+2700*std::sin(angle)});
    }
    SpatialMesh course;
    course.color = QColor("#756c63");
    course.material = SpatialSurfaceMaterial::Dirt;
    for (int index = 0; index <= path.size(); ++index) {
        const int current = index % path.size();
        const auto tangent = (path[(current+1)%path.size()]-path[(current+path.size()-1)%path.size()]).normalized();
        const QVector3D side(tangent.z(), 0, -tangent.x());
        for (float offset : {-600.f, -500.f, 500.f, 600.f})
            course.vertices.append(path[current]+side*offset);
    }
    for (int segment = 0; segment < path.size(); ++segment) {
        for (int lane = 0; lane < 3; ++lane) {
            course.faces.append({segment*4+lane, segment*4+lane+1, (segment+1)*4+lane+1, (segment+1)*4+lane});
            course.faceColors.append(lane == 1 ? QColor("#756c63") : QColor("#887e6d"));
        }
    }
    room.meshes.append(course);
    // Rounded, layered escarpments replace the vertical block cliffs. Their
    // coarse silhouette stays inside the shared 256-face scene contract; the
    // repeating rock material carries the close detail without more geometry.
    for (int hill = 0; hill < 2; ++hill) {
        SpatialMesh cliff;
        cliff.color = QColor("#918b82");
        cliff.material = SpatialSurfaceMaterial::Rock;
        const QVector3D center(hill ? 6600 : -6600, 0, hill ? 5300 : 1200);
        for (int ring = 0; ring < 3; ++ring) {
            for (int corner = 0; corner < 8; ++corner) {
                const float angle = corner*pi/4;
                const float radius = (2700 - ring*750) * (1 + .12f*std::sin(corner*2.3f + hill));
                const float height = ring == 0 ? 0 : ring*650.f + 150*std::sin(corner*1.7f);
                cliff.vertices.append(center + QVector3D(radius*std::cos(angle), height, radius*2.4f*std::sin(angle)));
            }
        }
        cliff.vertices.append(center + QVector3D(200, 1750, -300));
        for (int ring = 0; ring < 2; ++ring) {
            for (int corner = 0; corner < 8; ++corner) {
                const int a = ring*8 + corner;
                const int b = ring*8 + (corner+1)%8;
                cliff.faces.append({a, b, b+8});
                cliff.faces.append({a, b+8, a+8});
            }
        }
        for (int corner = 0; corner < 8; ++corner)
            cliff.faces.append({16 + corner, 16 + (corner+1)%8, 24});
        // Area-weighted shared normals smooth the existing low-cost silhouette.
        // The native pass lights it once; software keeps its own face shading.
        // No additional vertices, material images or collision work are needed.
        cliff.normals.fill({}, cliff.vertices.size());
        for (auto &face : cliff.faces) {
            std::reverse(face.begin(), face.end());
            const auto normal = QVector3D::crossProduct(cliff.vertices[face[1]] - cliff.vertices[face[0]],
                cliff.vertices[face[2]] - cliff.vertices[face[0]]);
            for (int vertex : face)
                cliff.normals[vertex] += normal;
        }
        for (auto &normal : cliff.normals)
            normal.normalize();
        room.meshes.append(cliff);
        room.solids.append({center + QVector3D(0, 200, 0), {1400, 400, 4500}, QColor("#9b7958"), true});
    }
    for (int flag = 0; flag < 8; ++flag) {
        const float side = flag%2 ? 1 : -1;
        const float z = -7000+(flag/2)*4400;
        room.solids.append({{side*3450, 210, z}, {12, 420, 12}, QColor("#e4dcc0"), true});
        room.solids.append({{side*3450+80, 350, z}, {150, 120, 8}, QColor(flag%2 ? "#ed742f" : "#1c727c"), false, SpatialSolidShape::Box,"MOTO"});
    }
}

void DirtBikeTrackArt::plant(SpatialInterior &room)
{
    SpatialFoliagePlacement placement(room);
    room.foliage.clear();
    for (int tree = 0; tree < 72; ++tree) {
        const float side = tree % 2 ? 1 : -1;
        const float stagger = 260 * std::sin(tree * 2.7f);
        float height = 620 + (tree % 5) * 85;
        float width = height * .95f;
        QVector3D preferred(side * (4200 + (tree % 3) * 270), 0,
            -9800 + (tree / 2) * 1750.f + stagger);
        if (tree >= 24) {
            const float angle = (tree - 24) * 2 * pi / 48;
            height = 1800 + (tree % 6) * 100;
            width = height * .8f;
            preferred = {13000 * std::sin(angle) + stagger, 0, 16000 * std::cos(angle)};
        }
        bool placed = false;
        // Eight 400 cm search rings cover at most 32 metres; at most 65
        // candidates per tree, with no endless rejection loop.
        for (int ring = 0; ring <= 8 && !placed; ++ring) {
            const int samples = ring == 0 ? 1 : 8;
            for (int sample = 0; sample < samples; ++sample) {
                const float angle = sample * pi / 4;
                const QVector3D offset(std::cos(angle) * ring * 400, 0, std::sin(angle) * ring * 400);
                const SpatialFoliage candidate{preferred + offset, height, width};
                if (placement.admit(candidate)) {
                    room.foliage.append(candidate);
                    placed = true;
                    break;
                }
            }
        }
        // Dense regions may contain fewer trees. Never force an overlap just
        // to meet the decorative target count.
    }
}
