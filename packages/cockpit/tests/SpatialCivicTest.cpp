#include "SpatialCivicWorld.h"
#include "SpatialInteriorGeometry.h"
#include "SpatialWebPassage.h"
#include "WorkspaceFlightView.h"
#include "FlightProjection.h"
#include <QApplication>
#include <QDir>
#include <QElapsedTimer>
#include <QThread>
#include <QScreen>
#include "DesktopAppWindow.h"
#include <cstdio>
#include <cmath>

#define VERIFY_CIVIC(value) do { if (!(value)) { std::fprintf(stderr, "Civic assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void settle(int milliseconds)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}
}

int spatialCivicTest()
{
    SpatialWebDocument document;
    document.venue = "cinema";
    for (int index = 0; index < 12; ++index) {
        SpatialWebItem item;
        item.id = QString::number(index);
        item.title = QString("Destination %1").arg(index + 1);
        item.url = QUrl(QString("https://example.org/article/%1").arg(index));
        document.items.append(item);
    }
    const auto cinema = document.room();
    document.venue = "city";
    const auto city = document.room();
    const auto hub = SpatialCivicWorld::headquarters();
    VERIFY_CIVIC(!SpatialCivicWorld::headquartersSign().isNull());
    for (const auto &room : {cinema, city, hub}) {
        VERIFY_CIVIC(room.solids.size() <= SpatialInterior::maximumSolids);
        VERIFY_CIVIC(room.actions.size() <= SpatialInterior::maximumActions);
        VERIFY_CIVIC(room.displays.size() <= 12);
        int vertices = 0;
        int faces = 0;
        for (const auto &mesh : room.meshes) {
            vertices += mesh.vertices.size();
            faces += mesh.faces.size();
        }
        std::printf("Civic budget: %lld solids, %d vertices, %d faces, %lld materials\n", static_cast<long long>(room.solids.size()), vertices, faces, static_cast<long long>(room.meshes.size()));
        VERIFY_CIVIC(room.meshes.size() <= SpatialInterior::maximumMeshes);
        VERIFY_CIVIC(vertices <= SpatialInterior::maximumMeshVertices);
        VERIFY_CIVIC(faces <= SpatialInterior::maximumMeshFaces);
        VERIFY_CIVIC(!SpatialInteriorGeometry::blocked(room, {float(room.spawn.x()), 160, float(room.spawn.y())}));
    }
    // Both lobby doors are actually traversable; no geometry change or load is needed.
    for (int z = -2750; z < 2400; z += 20)
        VERIFY_CIVIC(!SpatialInteriorGeometry::blocked(cinema, {0, 160, float(z)}));
    VERIFY_CIVIC(SpatialInteriorGeometry::walkingHeight(cinema, {-1100, 600, 300}) == 600);
    VERIFY_CIVIC(SpatialInteriorGeometry::walkingHeight(cinema, {-1100, 160, 300}) == 160);
    VERIFY_CIVIC(!SpatialInteriorGeometry::blocked(cinema, {-1100, 600, 300}));
    VERIFY_CIVIC(cinema.displays.first().size.width() >= 2000);
    for (const auto &display : city.displays) {
        const auto approach = display.portalCenter + QVector3D(0, 0, -100);
        VERIFY_CIVIC(!SpatialInteriorGeometry::blocked(city, approach));
        VERIFY_CIVIC(SpatialInteriorGeometry::approachedPortal(city, approach + QVector3D(0, 0, -20), approach, {0, 0, 1}) == display.id);
        SpatialWebPassage passage;
        VERIFY_CIVIC(passage.begin(city, display));
        VERIFY_CIVIC(passage.stage(cinema));
        VERIFY_CIVIC(passage.room().actions.isEmpty());
    }
    SpatialInterior occluded;
    occluded.actions.append({"hidden", "Hidden seat", {0, 160, 100}, {120, 150, 120}, SpatialActionKind::Seat});
    occluded.solids.append({{0, 160, 70}, {300, 320, 10}, QColor("#333333")});
    VERIFY_CIVIC(SpatialInteriorGeometry::target(occluded, {0, 160, -100}, {0, 0, 1}).isEmpty());
    WorkspaceFlightView view({}, false);
    view.resize(1400, 900);
    view.setInterior(cinema, "Grand Cinema");
    view.show();
    settle(60);
    view.setSurfaceSpawn({-690, 1310});
    VERIFY_CIVIC(view.activateInteriorAction("stalls-0-0"));
    VERIFY_CIVIC(view.seated());
    const auto cameraInverse = view.craftOrientation().conjugated();
    for (const auto &corner : SpatialInteriorGeometry::surface(cinema, cinema.displays.first())) {
        const auto projected = cameraInverse.rotatedVector(corner - view.cameraPosition());
        VERIFY_CIVIC(projected.z() > 0);
        const qreal focal = FlightProjection::focalLength(view.viewport()->size());
        VERIFY_CIVIC(std::abs(projected.x() / projected.z()) * focal < view.viewport()->width() / 2);
        VERIFY_CIVIC(std::abs(projected.y() / projected.z()) * focal < view.viewport()->height() / 2);
    }
    const QVector3D screenBottom(0, 136, 4000);
    VERIFY_CIVIC(SpatialInteriorGeometry::target(cinema, view.cameraPosition(),
        (screenBottom - view.cameraPosition()).normalized()) == "cinema-screen");
    const auto seatPosition = view.cameraPosition();
    view.setPilotInput({0, 0, 1}, {0.2, 0}, false);
    settle(100);
    VERIFY_CIVIC(view.cameraPosition() == seatPosition);
    view.fire();
    VERIFY_CIVIC(!view.seated());
    VERIFY_CIVIC(view.cameraPosition().z() == 1310);
    view.setSurfaceSpawn({-1260, -450});
    VERIFY_CIVIC(view.activateInteriorAction("balcony-up"));
    VERIFY_CIVIC(view.cameraPosition().y() == 600);
    view.setPilotInput({0, 0, 1}, {}, false);
    settle(120);
    view.setPilotInput({}, {}, false);
    VERIFY_CIVIC(view.cameraPosition().y() == 600);
    VERIFY_CIVIC(view.activateInteriorAction("balcony-down"));
    VERIFY_CIVIC(view.cameraPosition().y() == 160);
    view.setSurfaceSpawn({1800, -2300});
    VERIFY_CIVIC(view.activateInteriorAction("drone"));
    VERIFY_CIVIC(view.interiorLocomotion() == "fly");
    view.setPilotInput({0, 1, 0}, {}, false);
    settle(120);
    VERIFY_CIVIC(view.cameraPosition().y() > 320);
    VERIFY_CIVIC(view.leaveInteriorAction());
    VERIFY_CIVIC(view.interiorLocomotion() == "walk");
    VERIFY_CIVIC(view.cameraPosition().z() == -2300);
    VERIFY_CIVIC(!view.activateInteriorAction("stalls-0-0"));

    auto lowSky = cinema;
    lowSky.height = 380;
    view.setInterior(lowSky, "Orbit departure fixture");
    view.setSurfaceSpawn({1800, -2300});
    bool orbitRequested = false;
    view.orbitRequested = [&]() { orbitRequested = true; };
    VERIFY_CIVIC(view.activateInteriorAction("drone"));
    view.setPilotInput({0, 1, 0}, {}, false);
    settle(100);
    VERIFY_CIVIC(orbitRequested);
    view.orbitRequested = {};
    const QString directory = qEnvironmentVariable("DOCKPIPE_CIVIC_CAPTURE");
    if (!directory.isEmpty()) {
        QDir().mkpath(directory);
        const auto capture = [&](const SpatialInterior &room, const QString &name, QPointF position) {
            view.setInterior(room, name);
            view.setSurfaceSpawn(position);
            view.setInteriorImage("hq-brand", SpatialCivicWorld::headquartersSign());
            for (const auto &item : document.items)
                view.setInteriorImage(item.id, SpatialWebDocument::poster(item, {}, item.id.toInt()));
            view.setInteriorImage("cinema-screen", SpatialWebDocument::poster(document.items.first(), {}, 0));
            settle(60);
            return view.grab().save(directory + "/" + name + ".png");
        };
        VERIFY_CIVIC(capture(cinema, "cinema", {0, 950}));
        for (float yaw : {-0.5f, 0.5f}) {
            view.setInterior(cinema, "Cinema angle regression");
            view.streamInterior(cinema, "Cinema angle regression", {0, 160, 950}, yaw);
            settle(60);
            VERIFY_CIVIC(view.grab().save(directory + QString("/cinema-angle-%1.png").arg(yaw)));
        }
        VERIFY_CIVIC(capture(cinema, "lobby", {0, -1000}));
        VERIFY_CIVIC(capture(hub, "headquarters", {0, 0}));
        VERIFY_CIVIC(capture(hub, "street", {-2800, -2600}));
        VERIFY_CIVIC(capture(city, "city", city.spawn));
        view.setInterior(cinema, "Balcony");
        view.setSurfaceSpawn({-1260, -450});
        VERIFY_CIVIC(view.activateInteriorAction("balcony-up"));
        settle(60);
        VERIFY_CIVIC(view.grab().save(directory + "/balcony.png"));
        view.setInterior(cinema, "Seated in the grand cinema");
        view.setInteriorImage("cinema-screen", SpatialWebDocument::poster(document.items.first(), {}, 0));
        view.setSurfaceSpawn({-690, 1310});
        VERIFY_CIVIC(view.activateInteriorAction("stalls-0-0"));
        settle(60);
        VERIFY_CIVIC(view.grab().save(directory + "/seated.png"));
    }
    // Crossing panels need splitting, and moving surfaces must participate in
    // the same ordering without rebuilding the resident tree.
    SpatialPaintOrder order;
    order.rebuild({{{{-200, 0, 500}, {200, 0, 500}, {200, 200, 500}, {-200, 200, 500}}, 0},
        {{{0, 0, 200}, {0, 0, 800}, {0, 200, 800}, {0, 200, 200}}, 1}});
    VERIFY_CIVIC(order.fragments() > 2 && !order.boundedFallback());
    int visits = 0;
    QSet<int> retained;
    order.visit({50, 100, 0}, {0, 0, 1}, {{{{-10, 50, 300}, {10, 50, 300}, {10, 100, 300}, {-10, 100, 300}}, 2}},
        [&](const auto &polygon) { ++visits; retained.insert(polygon.source); });
    VERIFY_CIVIC(visits >= 4 && retained.size() == 3);
    order.rebuild({});
    visits = 0;
    order.visit({}, {0, 0, 1}, {}, [&](const auto &) { ++visits; });
    VERIFY_CIVIC(visits == 0 && order.fragments() == 0);
    QVector<SpatialPaintOrder::Polygon> dense;
    for (int index = 0; index < SpatialPaintOrder::maximumFragments; ++index) {
        const float z = index;
        dense.append({{{0, 0, z}, {10, 0, z}, {10, 10, z}, {0, 10, z}}, index});
    }
    order.rebuild(dense);
    VERIFY_CIVIC(order.boundedFallback());
    int previous = SpatialPaintOrder::maximumFragments;
    bool completeOrder = true;
    order.visit({}, {0, 0, 1}, {}, [&](const auto &polygon) {
        completeOrder = completeOrder && polygon.source == --previous;
    });
    VERIFY_CIVIC(completeOrder && previous == 0);

    // A long wall's centroid is closer than the red exhibit, but the wall
    // intersects the viewing ray BEHIND it. Distance sorting used to erase it.
    SpatialInterior occlusionRoom;
    occlusionRoom.size = {5000, 5000};
    occlusionRoom.height = 800;
    occlusionRoom.solids = {
        {{400, 200, 0}, {20, 400, 4000}, QColor("#2050c0")},
        {{250, 90, 1000}, {110, 180, 110}, QColor("#ed3020")},
        {{0, -20, 800}, {2500, 40, 4500}, QColor("#305050")}
    };
    view.hide();
    WorkspaceFlightView occlusionView({}, QGuiApplication::platformName() != "offscreen");
    const auto available = occlusionView.screen()->availableGeometry();
    occlusionView.resize(qMin(1200, available.width() - 40), qMin(800, available.height() - 80));
    occlusionView.move(available.topLeft() + QPoint(20, 30));
    occlusionView.setWindowFlag(Qt::WindowStaysOnTopHint);
    occlusionView.show();
    occlusionView.raise();
    occlusionView.activateWindow();
    settle(200);
    for (float yaw : {-0.25f, 0.0f, 0.5f, 0.75f}) {
        occlusionView.setInterior(occlusionRoom, "Wall and seat occlusion regression");
        occlusionView.streamInterior(occlusionRoom, "Wall and seat occlusion regression", {0, 160, 0}, yaw);
        settle(80);
        const auto local = occlusionView.craftOrientation().conjugated().rotatedVector(
            QVector3D(250, 110, 944) - occlusionView.cameraPosition());
        const qreal focal = FlightProjection::focalLength(occlusionView.viewport()->size());
        const QPoint sample = occlusionView.viewport()->rect().center()
            + QPoint(qRound(local.x() / local.z() * focal), qRound(-local.y() / local.z() * focal));
        QImage image;
        QPoint inView = occlusionView.viewport()->mapTo(&occlusionView, sample);
        if (QGuiApplication::platformName() == "offscreen") {
            image = occlusionView.grab().toImage();
        } else {
            DesktopAppWindow capture({quint64(occlusionView.winId()), "Occlusion fixture", "DockpipeCockpitTest"});
            QString captureError;
            image = capture.capture(captureError);
            VERIFY_CIVIC(captureError.isEmpty());
            const auto fitted = QSizeF(occlusionView.size()).scaled(1280, 720, Qt::KeepAspectRatio);
            const qreal scale = fitted.width() / occlusionView.width();
            inView = (QPointF((1280 - fitted.width()) / 2, (720 - fitted.height()) / 2) + QPointF(inView) * scale).toPoint();
        }
        VERIFY_CIVIC(image.rect().contains(inView));
        const auto pixel = image.pixelColor(inView);
        std::printf("Occlusion yaw %.2f: sample %d,%d = %s\n", yaw, inView.x(), inView.y(), qPrintable(pixel.name()));
        if (!directory.isEmpty())
            VERIFY_CIVIC(image.save(directory + QString("/occlusion-%1.png").arg(yaw)));
        VERIFY_CIVIC(pixel.red() > pixel.blue() * 2 && pixel.red() > pixel.green() * 2);
    }
    std::printf("Civic worlds: geometry bounds, doorways, balcony, seats, drone and city passages passed.\n");
    return 0;
}
