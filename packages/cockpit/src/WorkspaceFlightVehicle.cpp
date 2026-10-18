#include "WorkspaceFlightView.h"
#include "SpatialInteriorGeometry.h"
#include "DirtBikeModel.h"
#include "DirtBikeRiderPose.h"
#include "DirtBikeSteeringRig.h"
#include "DirtBikeCrashPose.h"
#include "DirtBikeCameraBoom.h"
#include <QPainter>
#include <QScopeGuard>
#include <QRandomGenerator>
#include <cmath>
#include <algorithm>
#include <optional>

QString WorkspaceFlightView::activeVehicleId() const
{
    return m_interior && m_activeVehicle >= 0 ? m_interior->vehicles[m_activeVehicle].id : QString();
}

bool WorkspaceFlightView::enterVehicle(const QString &id)
{
    if (m_bikeRace.rival(id))
        return false;
    if (!m_interior || m_activeVehicle >= 0 || m_interior->locomotion != "walk")
        return false;
    for (int index = 0; index < m_interior->vehicles.size(); ++index) {
        const auto &vehicle = m_interior->vehicles[index];
        if (vehicle.id != id)
            continue;
        const auto center = vehicle.position + QVector3D(0, SpatialVehicleModel::size(vehicle).y() / 2, 0);
        if ((m_camera - center).length() > 360)
            return false;
        // A nearby vehicle cannot be entered through a wall or another car.
        SpatialInterior probe = *m_interior;
        probe.displays.clear();
        if (SpatialInteriorGeometry::target(probe, m_camera, (center - m_camera).normalized()) != "vehicle:" + id)
            return false;
        stop();
        m_activeVehicle = index;
        if (vehicle.body == "dirt-bike") {
            if (m_bikeStart.id != vehicle.id)
                m_bikeStart = vehicle;
            m_bike.reset(vehicle);
            m_bikeLook.reset();
            m_vehicleModels[index] = DirtBikeModel::build(vehicle.paint, true);
        }
        m_interior->locomotion = "drive";
        m_attitudeActive = false;
        updateFlightModeButton();
        m_yaw = vehicle.heading;
        m_pitch = 0;
        m_camera = vehicle.position + SpatialVehicleModel::rotate({-35, SpatialVehicleModel::size(vehicle).y() - 25, 25}, vehicle.heading);
        setFlightStatus(tr("%1 · Approximate %2 model · W/S throttle · A/D steer · Space brake · E / B exit")
            .arg(vehicle.name, vehicle.body));
        if (ridingBike()) {
            updateBikeCamera(0, true);
            updateVehicleGeometry(index);
            if (!m_bikeMuted && !m_bikeSound.start())
                setFlightStatus(tr("Bike audio unavailable: optional SDL2 playback device could not open. Riding remains available."));
            else
                setFlightStatus(tr("MOTO · W / RT throttle · Down / left stick back: wheelie · Space / X: lower front · G / menu: race · V / Y camera · R recover"));
        }
        if (locomotionChanged)
            locomotionChanged();
        viewport()->update();
        return true;
    }
    return false;
}

bool WorkspaceFlightView::exitVehicle()
{
    if (!m_interior || m_activeVehicle < 0)
        return false;
    if (ridingBike() && (!m_bike.state().grounded || std::abs(m_bike.state().speed) > 100)) {
        setFlightStatus(tr("Stop on the ground before dismounting · R / menu: Recover bike"));
        return false;
    }
    stop();
    const auto &vehicle = m_interior->vehicles[m_activeVehicle];
    const auto dimensions = SpatialVehicleModel::size(vehicle);
    const QVector<QVector3D> doors{{-dimensions.x() / 2 - 65, 160, 0}, {dimensions.x() / 2 + 65, 160, 0},
        {-dimensions.x() / 2 - 65, 160, -100}, {dimensions.x() / 2 + 65, 160, -100},
        {0, 160, -dimensions.z() / 2 - 65}, {0, 160, dimensions.z() / 2 + 65}};
    SpatialInterior walkingRoom = *m_interior;
    walkingRoom.locomotion = "walk";
    for (const auto &door : doors) {
        auto position = vehicle.position + SpatialVehicleModel::rotate(door, vehicle.heading);
        position.setY(SpatialInteriorGeometry::walkingHeight(walkingRoom, position));
        if (SpatialInteriorGeometry::blocked(walkingRoom, position))
            continue;
        // Reject an exit across a thin wall even if its endpoint is clear.
        auto passage = walkingRoom;
        passage.vehicles.removeAt(m_activeVehicle);
        const QVector3D start(vehicle.position.x(), vehicle.position.y() + 160, vehicle.position.z());
        const int steps = qMax(1, int(std::ceil((position - start).length() / 20)));
        bool clear = true;
        for (int step = 0; step <= steps; ++step) {
            if (SpatialInteriorGeometry::blocked(passage, start + (position - start) * (float(step) / steps))) {
                clear = false;
                break;
            }
        }
        if (!clear)
            continue;
        m_camera = position;
        auto restModel = m_bikeStart.id == vehicle.id ? m_bikeStart : vehicle;
        restModel.position.setY(0);
        m_vehicleModels[m_activeVehicle] = SpatialVehicleModel::build(restModel);
        updateVehicleGeometry(m_activeVehicle);
        m_activeVehicle = -1;
        m_bikeRace.cancel();
        m_interior->locomotion = "walk";
        m_pitch = 0;
        setFlightStatus(tr("Parked %1 · Aim at the vehicle and press E / A to enter").arg(vehicle.name));
        if (locomotionChanged)
            locomotionChanged();
        viewport()->update();
        return true;
    }
    setFlightStatus(tr("No clear place to exit. Move the vehicle away from the obstruction."));
    return false;
}

void WorkspaceFlightView::updateVehicleGeometry(int index)
{
    const auto &vehicle = m_interior->vehicles[index];
    auto &geometry = m_vehicleGeometry[index];
    const auto *rival = m_bikeRace.rival(vehicle.id);
    const bool animateBike = (ridingBike() && index == m_activeVehicle) || rival;
    const auto &bike = rival ? rival->bike.state() : m_bike.state();
    const auto attitude = ArcadeFlightDynamics::attitude(0, bike.pitch, bike.lean);
    const DirtBikeSteeringRig frontAssembly(bike.steeringAngle, bike.frontCompression, bike.frontWheelAngle);
    const auto rearSpin = QQuaternion::fromAxisAndAngle(1, 0, 0, bike.rearWheelAngle * 57.29578f);
    const QVector3D swingPivot(0, 47, -12);
    const QVector3D swingRest(0, -12, -62);
    const auto swingDirection = swingRest + QVector3D(0, qBound(-24.f, bike.rearCompression, 24.f), 0);
    const auto swingRotation = QQuaternion::rotationTo(swingRest, swingDirection);
    const float swingScale = swingDirection.length() / swingRest.length();
    const auto swingAxis = swingRest.normalized();
    const auto headingRotation = QQuaternion::fromAxisAndAngle(0, 1, 0, vehicle.heading * 57.29578f);
    const float fall = bike.crashed ? qMin(1.f, bike.crashTime / .9f) : 0;
    std::optional<DirtBikeCrashPose> crashPose;
    if (animateBike && bike.crashed)
        crashPose.emplace(bike, m_vehicleModels[index], *m_interior);
    std::optional<DirtBikeRiderPose> riderPose;
    if (animateBike && !bike.crashed)
        riderPose.emplace(bike.steeringAngle, bike.lean, bike.riderWeight,
            (bike.frontCompression + bike.rearCompression) * .2f);
    geometry.resize(m_vehicleModels[index].size());
    for (int faceIndex = 0; faceIndex < m_vehicleModels[index].size(); ++faceIndex) {
        const auto &face = m_vehicleModels[index][faceIndex];
        auto &mesh = geometry[faceIndex];
        mesh.vertices.clear();
        mesh.center = {};
        mesh.color = face.color;
        mesh.albedo = face.albedo;
        mesh.normals.clear();
        mesh.material = face.material;
        mesh.finish = face.finish;
        mesh.uvs = face.uvs;
        mesh.edge = face.color;
        for (const auto &point : face.vertices) {
            auto local = point;
            if (animateBike) {
                if (DirtBikeSteeringRig::steeredPart(face.part))
                    local = frontAssembly.point(face.part, local);
                if (face.part == DirtBikeModel::RearWheel) {
                    const QVector3D axle(0, 35, -74);
                    local = axle + rearSpin.rotatedVector(local - axle);
                    local.setY(local.y() + qBound(-24.f, bike.rearCompression, 24.f));
                }
                if (face.part == DirtBikeModel::Swingarm) {
                    const auto offset = local - swingPivot;
                    // Preserve the authored tube width while its two endpoints
                    // follow the fixed pivot and the solver's rear contact.
                    local = swingPivot + swingRotation.rotatedVector(offset
                        + swingAxis * QVector3D::dotProduct(offset, swingAxis) * (swingScale - 1));
                }
                const bool riderPart = DirtBikeModel::riderPart(face.part);
                if (riderPart && !bike.crashed)
                    local = riderPose->point(face.part, local);
                if (bike.crashed && riderPart) {
                    local = crashPose->point(local);
                } else {
                    local = DirtBikeModel::chassisPoint(local, attitude);
                    local.setY(local.y() - 22 * fall);
                }
            }
            const auto world = vehicle.position + SpatialVehicleModel::rotate(local, vehicle.heading);
            mesh.vertices.append(world);
            mesh.center += world;
        }
        mesh.center /= mesh.vertices.size();
        if (face.normals.size() == face.vertices.size()) {
            QQuaternion rotation;
            if (animateBike) {
                if (DirtBikeSteeringRig::steeredPart(face.part))
                    rotation = frontAssembly.rotation(face.part);
                else if (face.part == DirtBikeModel::RearWheel)
                    rotation = rearSpin;
                else if (face.part == DirtBikeModel::Swingarm)
                    rotation = swingRotation;
                const bool rider = DirtBikeModel::riderPart(face.part);
                if (rider && !bike.crashed)
                    rotation = riderPose->rotation(face.part);
                rotation = (bike.crashed && rider ? crashPose->rotation() : attitude) * rotation;
            }
            rotation = headingRotation * rotation;
            const bool deformedTorso = animateBike && !bike.crashed
                && (face.part == DirtBikeModel::Rider || face.part == DirtBikeModel::RiderArm
                    || face.part == DirtBikeModel::Helmet);
            for (int vertex = 0; vertex < face.normals.size(); ++vertex) {
                auto normal = face.normals[vertex];
                if (deformedTorso)
                    normal = riderPose->bodyNormal(face.vertices[vertex], normal);
                mesh.normals.append(rotation.rotatedVector(normal));
            }
        }
        mesh.normal = {};
        if (face.singleSided)
            mesh.normal = QVector3D::crossProduct(mesh.vertices[1] - mesh.vertices[0], mesh.vertices[2] - mesh.vertices[0]).normalized();
        // Low-poly windows and wheels are deliberately two-sided. The common
        // world depth sort occludes them against the rest of the scene.
    }
    if (ridingBike() && index == m_activeVehicle) {
        MeshFace shadow;
        shadow.color = QColor(24, 18, 13, 65);
        const float floor = DirtBikeDynamics::groundHeight(*m_interior, vehicle.position);
        shadow.center = {vehicle.position.x(), floor+2, vehicle.position.z()};
        for (int corner = 0; corner < 8; ++corner) {
            const float angle = corner * .785398163f;
            shadow.vertices.append(shadow.center + SpatialVehicleModel::rotate({55*std::cos(angle), 0, 125*std::sin(angle)}, vehicle.heading));
        }
        geometry.append(shadow);
        const auto right = SpatialVehicleModel::rotate({1, 0, 0}, vehicle.heading);
        for (const auto &dust : m_bikeDust.particles()) {
            const float age = DirtBikeDust::lifetime-dust.life;
            const float radius = 15+age*55;
            MeshFace cloud;
            cloud.color = QColor(180, 132, 81, int(dust.life*55));
            cloud.center = dust.position + QVector3D(0, age*55, 0);
            cloud.vertices = {cloud.center-right*radius, cloud.center+QVector3D(0, radius, 0),
                cloud.center+right*radius, cloud.center-QVector3D(0, radius*.5f, 0)};
            geometry.append(cloud);
        }
    }
}

bool WorkspaceFlightView::visibleVehicleFace(int vehicle, int face) const
{
    if (vehicle != m_activeVehicle)
        return true;
    if (!ridingBike())
        return false;
    if (bikeChaseCamera() || face >= m_vehicleModels[vehicle].size())
        return true;
    return DirtBikeSteeringRig::visibleFromRider(m_vehicleModels[vehicle][face].part);
}

void WorkspaceFlightView::advanceVehicle(const QVector3D &delta, qreal steering, qreal seconds)
{
    auto &vehicle = m_interior->vehicles[m_activeVehicle];
    const qreal turn = steering * seconds * qBound(-1.4, m_driveSpeed / 250, 1.4);
    const int steps = qMax(1, int(std::ceil(qMax(delta.length() / 15.0, std::abs(turn) / 0.03))));
    const auto oldPosition = vehicle.position;
    const float oldHeading = vehicle.heading;
    // Sweep in short translation/rotation increments; rejected movement stops
    // the car rather than sliding its wheels sideways along a wall.
    for (int step = 0; step < steps; ++step) {
        auto next = vehicle;
        next.heading = std::remainder(next.heading + turn / steps, 2 * 3.14159265358979323846);
        next.position += SpatialVehicleModel::rotate({0, 0, float(m_driveSpeed * seconds / steps)}, next.heading);
        if (SpatialVehicleModel::blocked(*m_interior, next)) {
            m_driveSpeed = 0;
            break;
        }
        vehicle = next;
    }
    m_yaw += std::remainder(vehicle.heading - oldHeading, 2 * 3.14159265358979323846);
    m_camera = vehicle.position + SpatialVehicleModel::rotate({-35, SpatialVehicleModel::size(vehicle).y() - 25, 25}, vehicle.heading);
    if (vehicle.position != oldPosition || vehicle.heading != oldHeading)
        updateVehicleGeometry(m_activeVehicle);
}

void WorkspaceFlightView::paintVehicleCockpit(QPainter *painter)
{
    const auto &vehicle = m_interior->vehicles[m_activeVehicle];
    const qreal width = viewport()->width();
    const qreal height = viewport()->height();
    QString surface = tr("AIR");
    if (ridingBike() && m_bike.state().grounded) {
        const auto material = DirtBikeDynamics::groundMaterial(*m_interior, vehicle.position);
        surface = material == SpatialSurfaceMaterial::Grass ? tr("GRASS") : tr("DIRT");
        if (material == SpatialSurfaceMaterial::Rock)
            surface = tr("ROCK");
    }
    painter->save();
    if (ridingBike()) {
        painter->setPen(Qt::NoPen);
        painter->setBrush(QColor(12, 24, 29, 210));
        const QRectF panel(width-260, height-170, 230, 105);
        painter->drawRoundedRect(panel, 12, 12);
        painter->setPen(QColor("#fff1d7"));
        painter->setFont(QFont("Sans", 30, QFont::Bold));
        painter->drawText(panel.adjusted(14, 5, -14, -42), Qt::AlignLeft, QString::number(qRound(std::abs(m_driveSpeed)*.036)));
        painter->setFont(QFont("Sans", 10, QFont::Bold));
        painter->drawText(panel.adjusted(90, 17, -12, -40), Qt::AlignLeft,
            tr("KM/H · %1\nGEAR %2").arg(surface).arg(m_bike.state().reversing ? tr("R · PUSH") : QString::number(m_bike.state().gear)));
        painter->drawText(panel.adjusted(14, 67, -10, 0), Qt::AlignLeft, m_bikeMuted ? tr("SOUND OFF · M") : tr("250 · V / Y CAMERA"));
        painter->setBrush(QColor("#eb6c31"));
        painter->drawRoundedRect(QRectF(panel.x()+14, panel.y()+58, 200*qBound(0.f, (m_bike.state().engineRpm-1600)/8900, 1.f), 4), 2, 2);
        if (m_bikeRace.phase() != DirtBikeRace::Phase::Idle) {
            const QRectF racePanel(width - 330, height - 275, 300, 90);
            painter->setBrush(QColor(12, 24, 29, 220));
            painter->drawRoundedRect(racePanel, 10, 10);
            painter->setFont(QFont("Sans", 13, QFont::Bold));
            QString title;
            if (m_bikeRace.phase() == DirtBikeRace::Phase::Countdown) {
                title = tr("GET READY · %1").arg(int(std::ceil(m_bikeRace.countdown())));
            } else if (m_bikeRace.phase() == DirtBikeRace::Phase::Finished) {
                title = tr("FINISHED · %1 / 3").arg(m_bikeRace.place(vehicle.position));
            } else {
                title = tr("LAP %1 / 3 · PLACE %2 / 3").arg(m_bikeRace.lap())
                    .arg(m_bikeRace.place(vehicle.position));
            }
            painter->drawText(racePanel.adjusted(12, 8, -10, -45), Qt::AlignLeft, title);
            painter->setFont(QFont("Sans", 10));
            painter->drawText(racePanel.adjusted(12, 38, -10, -8), Qt::AlignLeft,
                tr("%1 s · Blue: Riley · Green: Alex\nG restart · R checkpoint recovery")
                    .arg(m_bikeRace.elapsed(), 0, 'f', 1));
        }
        if (m_bike.state().frontSupport < .05f && m_bike.state().rearSupport > .2f
            && m_bike.state().pitch > .2f && !bikeCrashed()) {
            painter->setFont(QFont("Sans", 13, QFont::Bold));
            painter->drawText(QRectF(width / 2 - 200, height * .68, 400, 40), Qt::AlignCenter,
                tr("WHEELIE · rear brake to lower"));
        }
        if (bikeCrashed()) {
            const QRectF banner(width / 2 - 250, 110, 500, 86);
            painter->setBrush(QColor(70, 24, 21, 230));
            painter->drawRoundedRect(banner, 10, 10);
            painter->setFont(QFont("Sans", 15, QFont::Bold));
            painter->drawText(banner.adjusted(12, 8, -12, -45), Qt::AlignCenter, tr("BIKE DOWN"));
            painter->setFont(QFont("Sans", 10));
            painter->drawText(banner.adjusted(12, 40, -12, -6), Qt::AlignCenter,
                DirtBikeDynamics::crashDescription(m_bike.state().crashReason) + tr(" · R / Start → Recover bike"));
        }
        painter->restore();
        return;
    }
    // Bonnet, windscreen pillars and dashboard retain a readable road view.
    painter->setPen(QPen(vehicle.paint.lighter(140), 2));
    painter->setBrush(vehicle.paint);
    painter->drawPolygon(QPolygonF{{width * .15, height * .82}, {width * .28, height * .70},
        {width * .72, height * .70}, {width * .85, height * .82}});
    painter->setPen(Qt::NoPen);
    painter->setBrush(QColor("#15212c"));
    painter->drawPolygon(QPolygonF{{0, 0}, {width*.025, 0}, {width*.13, height*.81}, {0, height}});
    painter->drawPolygon(QPolygonF{{width, 0}, {width*.975, 0}, {width*.87, height*.81}, {width, height}});
    painter->drawRoundedRect(QRectF(-10, height * .81, width + 20, height * .22), 35, 35);
    painter->setPen(QPen(QColor("#435668"), 5));
    painter->drawLine(QPointF(width * .13, height * .82), QPointF(width * .87, height * .82));
    const QPointF wheel(width * .31, height * .91);
    const qreal radius = qMin(width * .075, height * .13);
    painter->setBrush(Qt::NoBrush);
    painter->setPen(QPen(QColor("#a6b5bf"), 9));
    painter->drawEllipse(wheel, radius, radius);
    painter->drawLine(wheel + QPointF(-radius, 0), wheel + QPointF(radius, 0));
    painter->drawLine(wheel, wheel + QPointF(0, radius));
    painter->setPen(QColor("#9ee6d4"));
    painter->setFont(QFont("Sans", 16, QFont::DemiBold));
    painter->drawText(QRectF(width * .45, height * .84, width * .35, 36), Qt::AlignCenter,
        tr("%1 km/h  ·  %2").arg(qRound(std::abs(m_driveSpeed) * .036)).arg(m_driveSpeed < 0 ? "R" : "D"));
    painter->restore();
}

bool WorkspaceFlightView::ridingBike() const
{
    return m_interior && m_activeVehicle >= 0
        && m_interior->vehicles[m_activeVehicle].body == "dirt-bike";
}

bool WorkspaceFlightView::bikeRaceAvailable() const
{
    return ridingBike() && (m_interior->environmentKey == "dockpipe:mesa-moto:v1" || m_interior->environmentKey == "dockpipe:redbud-moto:v1")
        && activeVehicleId() == "mesa-bike";
}

bool WorkspaceFlightView::startBikeRace()
{
    if (!bikeRaceAvailable())
        return false;
    const auto layout = m_interior->terrain ? DirtBikeRaceCourse::Layout::RedBud : DirtBikeRaceCourse::Layout::Mesa;
    DirtBikeRace race;
    race.start(QRandomGenerator::global()->generate(), layout);
    std::array<int, DirtBikeRace::rivalCount> indices;
    std::array<QVector<SpatialVehicleFace>, DirtBikeRace::rivalCount> models;
    int missing = 0;
    for (int rival = 0; rival < DirtBikeRace::rivalCount; ++rival) {
        const auto &vehicle = race.rivals()[rival].vehicle;
        indices[rival] = -1;
        for (int index = 0; index < m_interior->vehicles.size(); ++index) {
            if (m_interior->vehicles[index].id == vehicle.id) {
                if (!m_bikeRace.rival(vehicle.id)) {
                    setFlightStatus(tr("Race rival identifiers are already in use."));
                    return false;
                }
                indices[rival] = index;
            }
        }
        if (indices[rival] < 0)
            ++missing;
        models[rival] = DirtBikeModel::build(vehicle.paint, true);
        if (models[rival].isEmpty() || models[rival].size() > DirtBikeModel::maximumFaces)
            return false;
    }
    if (m_interior->vehicles.size() + missing > SpatialVehicleModel::maximumVehicles) {
        setFlightStatus(tr("Race needs room for two rival bikes."));
        return false;
    }
    auto gridRoom = *m_interior;
    for (int index = gridRoom.vehicles.size() - 1; index >= 0; --index) {
        const auto &id = gridRoom.vehicles[index].id;
        if (id == activeVehicleId() || m_bikeRace.rival(id))
            gridRoom.vehicles.removeAt(index);
    }
    auto playerGrid = DirtBikeRace::playerGrid(layout);
    if (SpatialVehicleModel::blocked(gridRoom, playerGrid)) {
        setFlightStatus(tr("The race grid is obstructed. Clear the start area first."));
        return false;
    }
    gridRoom.vehicles.append(playerGrid);
    for (const auto &rival : race.rivals()) {
        if (SpatialVehicleModel::blocked(gridRoom, rival.vehicle)) {
            setFlightStatus(tr("The race grid is obstructed. Clear the start area first."));
            return false;
        }
        gridRoom.vehicles.append(rival.vehicle);
    }
    stop();
    m_bikeRace = std::move(race);
    auto grid = DirtBikeRace::playerGrid(layout);
    grid.paint = m_interior->vehicles[m_activeVehicle].paint;
    m_interior->vehicles[m_activeVehicle] = grid;
    m_bike.reset(grid);
    m_bikeLook.reset();
    for (int rival = 0; rival < DirtBikeRace::rivalCount; ++rival) {
        const auto &vehicle = m_bikeRace.rivals()[rival].vehicle;
        int index = indices[rival];
        if (index < 0) {
            index = m_interior->vehicles.size();
            m_interior->vehicles.append(vehicle);
            m_vehicleModels.append(std::move(models[rival]));
            m_vehicleGeometry.append(QVector<MeshFace>{});
        } else {
            m_interior->vehicles[index] = vehicle;
            m_vehicleModels[index] = std::move(models[rival]);
        }
        updateVehicleGeometry(index);
    }
    updateBikeCamera(0, true);
    updateVehicleGeometry(m_activeVehicle);
    setFlightStatus(tr("MOTO RACE · 3 laps · Riley / Blue · Alex / Green · G restart · R recover"));
    m_elapsed.start();
    m_tick.start();
    viewport()->update();
    return true;
}

void WorkspaceFlightView::resetBike()
{
    if (!ridingBike())
        return;
    stop();
    const auto recovery = m_bikeRace.active() ? m_bikeRace.recoverPlayer(*m_interior) : m_bikeStart;
    m_interior->vehicles[m_activeVehicle] = recovery;
    m_bike.reset(recovery);
    m_bikeLook.reset();
    m_yaw = m_bikeStart.heading;
    m_pitch = 0;
    updateBikeCamera(0, true);
    updateVehicleGeometry(m_activeVehicle);
    setFlightStatus(m_bikeRace.active() ? tr("Rejoined at your last checkpoint · Race continues")
        : tr("Bike recovered at the start · Down / left stick back + throttle: wheelie"));
    if (m_bikeRace.active()) {
        m_elapsed.start();
        m_tick.start();
    }
    viewport()->update();
}

void WorkspaceFlightView::advanceBike(qreal seconds)
{
    QElapsedTimer updateTime;
    updateTime.start();
    const auto recordUpdate = qScopeGuard([&]() { m_lastBikeUpdateNanoseconds = updateTime.nsecsElapsed(); });
    m_bikeRace.advance(seconds, m_bike.state(), *m_interior);
    for (const auto &rival : m_bikeRace.rivals()) {
        if (rival.vehicle.id.isEmpty())
            continue;
        for (int index = 0; index < m_interior->vehicles.size(); ++index) {
            if (m_interior->vehicles[index].id != rival.vehicle.id)
                continue;
            m_interior->vehicles[index] = rival.vehicle;
            updateVehicleGeometry(index);
        }
    }
    DirtBikeDynamics::Input input;
    input.throttle = int(m_keys.contains(Qt::Key_W)) - int(m_keys.contains(Qt::Key_S)) + m_bikeThrottle;
    input.steering = int(m_keys.contains(Qt::Key_D)) - int(m_keys.contains(Qt::Key_A)) + m_pilotMovement.x();
    input.pitch = int(m_keys.contains(Qt::Key_Down)) - int(m_keys.contains(Qt::Key_Up)) - m_pilotMovement.z();
    input.serviceBrake = m_bikeBrake;
    input.rearBrake = m_keys.contains(Qt::Key_Space) || m_keys.contains(Qt::Key_X) || m_pilotBrake;
    if (m_bikeRace.phase() == DirtBikeRace::Phase::Countdown) {
        input = {};
        input.rearBrake = true;
    }
    auto &vehicle = m_interior->vehicles[m_activeVehicle];
    const float heading = vehicle.heading;
    const bool wasCrashed = m_bike.state().crashed;
    m_bike.advance(input, seconds, *m_interior);
    const auto &bike = m_bike.state();
    if (bike.crashed)
        m_bikeLook.reset();
    else
        m_bikeLook.advance(m_pilotLook, seconds);
    vehicle.position = bike.position;
    vehicle.heading = bike.heading;
    m_driveSpeed = bike.speed;
    m_yaw += std::remainder(bike.heading - heading, 6.283185307f);
    updateBikeCamera(seconds, bike.crashed && !wasCrashed);
    m_bikeDust.advance(bike, seconds, *m_interior);
    updateVehicleGeometry(m_activeVehicle);
    DirtBikeSynth::Mix sound;
    sound.throttle = qMax(0.f, bike.throttle);
    sound.engineRpm = bike.engineRpm;
    sound.speed = bike.speed;
    sound.skid = bike.skid;
    sound.airborne = !bike.grounded;
    sound.engineRunning = !bike.crashed;
    sound.impact = bike.landingImpact;
    m_bikeSound.update(sound);
    if (!m_bikeMuted && (!bike.crashed || m_bike.unsettled()))
        m_bikeSound.start();
    if (bike.crashed) {
        if (!wasCrashed) {
            m_keys.clear();
            m_bikeThrottle = 0;
            m_bikeBrake = 0;
            m_pilotMovement = {};
            m_pilotLook = {};
            m_pilotBrake = false;
            setFlightStatus(DirtBikeDynamics::crashDescription(bike.crashReason)
                + tr(" · R or Start → Recover bike to return to the pit"));
        }
        if (!m_bike.unsettled()) {
            m_bikeSound.stop();
            if (!m_bikeRace.active())
                m_tick.stop();
        }
    }
}

void WorkspaceFlightView::setPilotVehicleControls(float throttle, float brake)
{
    if (!ridingBike() || bikeCrashed() || !isVisible() || m_editorActive)
        return;
    m_bikeThrottle = std::isfinite(throttle) ? qBound(0.f, throttle, 1.f) : 0;
    m_bikeBrake = std::isfinite(brake) ? qBound(0.f, brake, 1.f) : 0;
    if ((m_bikeThrottle > 0 || m_bikeBrake > 0) && !m_tick.isActive()) {
        m_elapsed.start();
        m_tick.start();
    }
}

void WorkspaceFlightView::updateBikeCamera(float seconds, bool snap)
{
    const auto &bike = m_bike.state();
    const auto lookOffset = m_bikeLook.offset();
    if (!bikeChaseCamera()) {
        // Rider view uses a wider vertical lens so the actual controls fit
        // below the road. The eye anchor follows the chassis, not the bars.
        const auto attitude = ArcadeFlightDynamics::attitude(0, bike.pitch, bike.lean);
        const auto anchor = DirtBikeModel::chassisPoint({0, 165, 0}, attitude);
        m_camera = bike.position + SpatialVehicleModel::rotate(anchor, bike.heading);
        m_yaw = bike.heading + lookOffset.x();
        m_pitch = bike.pitch - .16f + lookOffset.y();
        m_bikeCameraPending = false;
        return;
    }
    const auto forward = SpatialVehicleModel::rotate({0, 0, 1}, bike.heading);
    auto focus = bike.position + QVector3D(0, 130, 0) + forward * 120;
    const auto near = bike.position + QVector3D(0, 190, 0);
    QVector3D desired = bike.position - forward * 470 + QVector3D(0, 280, 0);
    if (!lookOffset.isNull()) {
        const float distance = std::hypot(590.f, 150.f);
        const float elevation = std::atan2(150.f, 590.f) - lookOffset.y();
        const float heading = bike.heading + lookOffset.x();
        const float horizontal = distance * std::cos(elevation);
        desired = focus + QVector3D(-std::sin(heading) * horizontal,
            distance * std::sin(elevation), -std::cos(heading) * horizontal);
    }
    if (bike.crashed) {
        // Frame both the rider and the bike, clear of the lower-right HUD.
        focus = bike.position + SpatialVehicleModel::rotate({70, 110, 0}, bike.heading);
        desired = bike.position - forward * 650 + QVector3D(0, 320, 0);
    }
    const auto camera = DirtBikeCameraBoom::resolve(near, desired, m_camera, seconds, snap,
        [&](const QVector3D &point) {
            return SpatialInteriorGeometry::blocked(*m_interior, point)
                || point.y() < DirtBikeDynamics::groundHeight(*m_interior, point) + 55;
        });
    m_camera = camera.position;
    const auto look = focus-m_camera;
    m_yaw = std::atan2(look.x(), look.z());
    m_pitch = std::atan2(look.y(), std::hypot(look.x(), look.z()));
    m_bikeCameraPending = camera.pending;
}

void WorkspaceFlightView::toggleBikeCamera()
{
    if (!ridingBike())
        return;
    m_bikeChase = !m_bikeChase;
    m_bikeLook.reset();
    m_yaw = m_bike.state().heading;
    m_pitch = 0;
    updateBikeCamera(0, true);
    viewport()->update();
}

void WorkspaceFlightView::toggleBikeSound()
{
    m_bikeMuted = !m_bikeMuted;
    if (m_bikeMuted)
        m_bikeSound.stop();
    else if (ridingBike() && !bikeCrashed() && isVisible())
        m_bikeSound.start();
    viewport()->update();
}
