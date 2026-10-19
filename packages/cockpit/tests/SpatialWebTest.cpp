#include "SpatialWebBrowser.h"
#include "SpatialWebMedia.h"
#include "SpatialWebWorld.h"
#include "SpatialWebPassage.h"
#include "SpatialSiteExplorer.h"
#include <QComboBox>
#include <QFile>
#include <QBuffer>
#include <QPainter>
#include <QToolButton>
#include <QMenu>
#include "SpatialNeuralService.h"
#include "SpatialAnimalModel.h"
#include <limits>
#include "CockpitWidget.h"
#include "SpatialInteriorGeometry.h"
#ifdef DOCKPIPE_SPATIAL_WEB
#include "BrowserWidget.h"
#include <QTabBar>
#include <QWebEngineView>
#include <QWebEnginePage>
#include <QWebEngineScript>
#include <QWebEngineScriptCollection>
#endif
#include <QApplication>
#include <QElapsedTimer>
#include <QJsonArray>
#include <QUrlQuery>
#include <QJsonDocument>
#include <QJsonObject>
#include <QPointer>
#include <QLineEdit>
#include <QMouseEvent>
#include <QKeyEvent>
#include <QScreen>
#include <QStackedWidget>
#include <QOpenGLWidget>
#include <QPushButton>
#include <QTcpServer>
#include <QTcpSocket>
#include <QThread>
#include <QTimer>
#include <QSharedPointer>
#include <cstdio>
#include <algorithm>
#include <cmath>

#define VERIFY_WEB(value) do { if (!(value)) { std::fprintf(stderr, "Spatial web assertion at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
bool waitForWeb(const std::function<bool()> &ready, int milliseconds = 15000)
{
    QElapsedTimer elapsed;
    elapsed.start();
    while (!ready() && elapsed.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    return ready();
}
}

int spatialWebModelTest()
{
    VERIFY_WEB(SpatialWebDocument::allowedUrl(QUrl("https://example.org/gallery")));
    for (const QString &url : {"file:///etc/passwd", "javascript:alert(1)", "https://user:pass@example.org", "data:text/html,test"})
        VERIFY_WEB(!SpatialWebDocument::allowedUrl(QUrl(url)));
    SpatialWebDocument document;
    QString error;
    VERIFY_WEB(!SpatialWebDocument::parse("not json", document, error));
    QJsonObject value{{"url", "https://example.org"}, {"title", "Gallery"}, {"venue", "gallery"},
        {"items", QJsonArray{QJsonObject{{"id", "0"}, {"title", "First exhibit"}, {"href", "https://example.org/first"}}}}};
    VERIFY_WEB(SpatialWebDocument::parse(QJsonDocument(value).toJson(), document, error));
    VERIFY_WEB(document.items.size() == 1 && document.room().displays.size() == 1);
    VERIFY_WEB(SpatialWebDocument::poster(document.items[0], {}, 0).size() == QSize(512, 320));
    auto items = value.value("items").toArray();
    items.append(items[0]);
    value["items"] = items;
    VERIFY_WEB(!SpatialWebDocument::parse(QJsonDocument(value).toJson(), document, error));
    VERIFY_WEB(!SpatialWebDocument::parse(QByteArray(SpatialWebDocument::maximumBytes + 1, 'x'), document, error));
    QJsonArray tooMany;
    for (int index = 0; index <= SpatialWebDocument::maximumItems; ++index)
        tooMany.append(QJsonObject{{"id", QString::number(index)}, {"title", "Exhibit"}});
    value["items"] = tooMany;
    VERIFY_WEB(!SpatialWebDocument::parse(QJsonDocument(value).toJson(), document, error));
    VERIFY_WEB(SpatialWebDocument::searchResultsUrl(QUrl("https://www.google.com/search?q=rainforest")));
    VERIFY_WEB(SpatialWebDocument::searchResultsUrl(QUrl("https://www.bing.com/search?q=rainforest")));
    VERIFY_WEB(SpatialWebDocument::searchResultsUrl(QUrl("https://duckduckgo.com/?q=rainforest")));
    VERIFY_WEB(!SpatialWebDocument::searchResultsUrl(QUrl("https://example.org/search?q=rainforest")));
    VERIFY_WEB(!SpatialWebDocument::searchResultsUrl(QUrl("https://google.com.evil.test/search?q=rainforest")));
    VERIFY_WEB(!SpatialWebDocument::searchResultsUrl(QUrl("https://google.com/search?q=")));
    SpatialWebDocument capacity;
    for (int index = 0; index < SpatialWebDocument::maximumItems; ++index) {
        SpatialWebItem item;
        item.id = QString::number(index);
        item.title = "Repository " + item.id;
        item.url = QUrl("https://example.org/" + item.id);
        capacity.items.append(item);
    }
    for (const QString &venue : {"repositories", "gallery", "store", "cinema"}) {
        capacity.venue = venue;
        const auto room = capacity.room();
        VERIFY_WEB(room.displays.size() == SpatialWebDocument::maximumItems);
        VERIFY_WEB(!room.solids.isEmpty() && room.solids.size() <= SpatialInterior::maximumSolids);
        VERIFY_WEB(!SpatialInteriorGeometry::blocked(room, {float(room.spawn.x()), 160, float(room.spawn.y())}));
        VERIFY_WEB(!SpatialInteriorGeometry::blocked(room, {0, 160, 0}));
        for (const auto &display : room.displays)
            VERIFY_WEB(display.freestanding);
        if (venue == "repositories" || venue == "gallery") {
            for (const auto &surface : room.displays) {
                const float radians = surface.yaw * 3.14159265f / 180;
                const QVector3D outward(std::sin(radians), 0, -std::cos(radians));
                const QVector3D approach = surface.portalCenter + outward * 100;
                const QVector3D previous = surface.portalCenter + outward * 120;
                VERIFY_WEB(!SpatialInteriorGeometry::blocked(room, approach));
                VERIFY_WEB(SpatialInteriorGeometry::approachedPortal(room, previous, approach, -outward) == surface.id);
                VERIFY_WEB(SpatialInteriorGeometry::approachedPortal(room, approach, previous, -outward).isEmpty());
                VERIFY_WEB(SpatialInteriorGeometry::approachedPortal(room, previous, approach, outward).isEmpty());
                VERIFY_WEB(SpatialInteriorGeometry::approachedPortal(room, approach, approach, -outward).isEmpty());
            }
        }
        if (venue == "cinema") {
            VERIFY_WEB(room.displays[0].id == "cinema-screen");
            for (int index = 1; index < room.displays.size(); ++index) {
                const auto &surface = room.displays[index];
                const auto corners = SpatialInteriorGeometry::surface(room, surface);
                const auto normal = QVector3D::crossProduct(corners[1] - corners[0], corners[3] - corners[0]);
                VERIFY_WEB(QVector3D::dotProduct(normal, QVector3D(0, 260, surface.center.z()) - surface.center) > 0);
            }
        }
    }
    capacity.venue = "mall";
    capacity.url = QUrl("https://example.org");
    capacity.title = "Shared content";
    auto animals = capacity;
    animals.venue = "park";
    animals.title = "Invertebrate animals";
    animals.evidence = "Animal habitats and wildlife";
    for (const QString &form : {"spider", "crab", "insect", "worm", "butterfly", "fish", "bird", "quadruped", "cephalopod"}) {
        for (auto &item : animals.items) {
            item.kind = "animal";
            item.animalForm = form;
        }
        const auto plan = SpatialWebWorld::plan(animals, SpatialWebGraph::fromDocument(animals), SpatialWebWorld::optionsFor(animals));
        const auto result = SpatialWebWorld::compile(plan);
        if (!result.success)
            std::fprintf(stderr, "Animal %s construction: %s\n", qPrintable(form), qPrintable(result.diagnostic));
        VERIFY_WEB(result.success);
        VERIFY_WEB(std::none_of(result.interior.displays.cbegin(), result.interior.displays.cend(), [](const auto &display) { return display.portal; }));
        VERIFY_WEB(std::count_if(result.interior.solids.cbegin(), result.interior.solids.cend(), [](const auto &solid) {
            return solid.label.startsWith("Illustrative model");
        }) == 4);
        const auto model = SpatialAnimalModel::assemble(form);
        VERIFY_WEB(model.vertices == SpatialAnimalModel::assemble(form).vertices);
        VERIFY_WEB(model.vertices.size() <= 64 && model.faces.size() <= 40);
    }
    VERIFY_WEB(SpatialAnimalModel::assemble("unknown").vertices.isEmpty());
    SpatialWebItem spider;
    spider.kind = "link";
    spider.title = "Black Widow";
    SpatialAnimalModel::interpret(spider, "Movies and cinema");
    VERIFY_WEB(spider.animalForm.isEmpty());
    SpatialAnimalModel::interpret(spider, "Invertebrates and animals");
    VERIFY_WEB(spider.kind == "animal" && spider.animalForm == "spider");
    auto selfLink = capacity;
    selfLink.items.first().url = QUrl("https://example.org/#section");
    VERIFY_WEB(SpatialWebGraph::fromDocument(selfLink).nodes[2].action == SpatialWebNode::Action::None);
    for (int index = 0; index < capacity.items.size(); ++index)
        capacity.items[index].category = "Group " + QString::number(index / 3);
    auto departmental = capacity;
    const auto departmentOptions = SpatialWebWorld::optionsFor(departmental);
    VERIFY_WEB(departmentOptions.topology == SpatialTopology::Branches);
    auto catalogue = departmental;
    catalogue.accent = QColor("#bd4f89");
    for (auto &item : catalogue.items) {
        item.category.clear();
        item.hasImage = false;
    }
    const auto catalogueOptions = SpatialWebWorld::optionsFor(catalogue);
    VERIFY_WEB(catalogueOptions.topology == SpatialTopology::Spine);
    VERIFY_WEB(catalogueOptions.style.accent == catalogue.accent);
    auto naturalArticle = catalogue;
    naturalArticle.venue = "park";
    naturalArticle.evidence = "Wildlife habitat facts";
    const auto naturalOptions = SpatialWebWorld::optionsFor(naturalArticle);
    VERIFY_WEB(naturalOptions.topology == SpatialTopology::Spine);
    const auto naturalPlan = SpatialWebWorld::plan(naturalArticle, SpatialWebGraph::fromDocument(naturalArticle), naturalOptions);
    VERIFY_WEB(SpatialWebWorld::compile(naturalPlan).success);
    VERIFY_WEB(!naturalPlan.meshes.isEmpty());
    for (const auto &hill : naturalPlan.meshes) {
        // Scenery here consists of five-vertex hills; paths are added by compile.
        VERIFY_WEB(hill.vertices.size() == 5);
        const auto peak = hill.vertices.last();
        VERIFY_WEB(std::abs(peak.x() - naturalPlan.spawn.x()) >= 480
            || std::abs(peak.z() - naturalPlan.spawn.z()) >= 480);
    }
    auto illustrated = catalogue;
    for (auto &item : illustrated.items) {
        item.hasImage = true;
        item.visualAspect = 2.3f;
    }
    const auto illustratedOptions = SpatialWebWorld::optionsFor(illustrated);
    VERIFY_WEB(illustratedOptions.topology == SpatialTopology::Loop);
    const auto illustratedGraph = SpatialWebGraph::fromDocument(illustrated);
    const auto widePlan = SpatialWebWorld::plan(illustrated, illustratedGraph, illustratedOptions);
    if (!SpatialWebWorld::compile(widePlan).success)
        std::fprintf(stderr, "Illustrated construction: %s\n", qPrintable(SpatialWebWorld::compile(widePlan).diagnostic));
    VERIFY_WEB(SpatialWebWorld::compile(widePlan).success);
    const auto wideRoom = SpatialWebWorld::compile(widePlan).interior;
    for (const auto &entrance : wideRoom.displays) {
        SpatialWebPassage passage;
        VERIFY_WEB(passage.begin(wideRoom, entrance));
        VERIFY_WEB(passage.stage(wideRoom));
        const float radians = entrance.yaw * 3.14159265f / 180;
        const QVector3D inward(-std::sin(radians), 0, std::cos(radians));
        const auto joined = passage.room();
        for (int distance = 0; distance <= 750; distance += 25)
            VERIFY_WEB(!SpatialInteriorGeometry::blocked(joined, entrance.portalCenter + inward * distance));
    }
    for (auto &item : illustrated.items)
        item.visualAspect = 0.6f;
    const auto narrowPlan = SpatialWebWorld::plan(illustrated, illustratedGraph, illustratedOptions);
    VERIFY_WEB(SpatialWebWorld::compile(narrowPlan).success);
    VERIFY_WEB(widePlan.displays.first().size.width() > narrowPlan.displays.first().size.width());
    VERIFY_WEB(qAbs(widePlan.displays.first().size.width() / widePlan.displays.first().size.height() - 1.6) < 0.001);
    // Artwork adapts to source aspect; the walk-through doorway keeps its clearance.
    const auto graph = SpatialWebGraph::fromDocument(capacity);
    VERIFY_WEB(graph.validate(error));
    auto rainforest = capacity;
    rainforest.venue = "park";
    rainforest.evidence = "Rainforest habitats and wildlife";
    for (int index = 0; index < 8; ++index)
        rainforest.items[index].kind = "excerpt";
    const auto habitat = SpatialWebWorld::construct(rainforest, SpatialWebWorld::optionsFor(rainforest));
    VERIFY_WEB(habitat.success && !habitat.fallback && habitat.interior.naturalGround);
    VERIFY_WEB(habitat.interior.solids.size() <= SpatialInterior::maximumSolids);
    VERIFY_WEB(!habitat.interior.displays.first().portal);
    VERIFY_WEB(habitat.interior.displays.last().portalApproachDistance == 240);
    QSet<QString> layouts;
    for (auto topology : {SpatialTopology::Branches, SpatialTopology::Loop, SpatialTopology::Spine, SpatialTopology::Layers}) {
        SpatialWorldOptions options;
        options.topology = topology;
        const auto plan = SpatialWebWorld::plan(capacity, graph, options);
        const auto world = SpatialWebWorld::compile(plan);
        if (!world.success)
            std::fprintf(stderr, "Composition rejected: %s\n", qPrintable(world.diagnostic));
        VERIFY_WEB(world.success && !world.fallback);
        VERIFY_WEB(world.interior.displays.size() == 12);
        VERIFY_WEB(world.interior.solids.size() <= SpatialInterior::maximumSolids);
        VERIFY_WEB(world.interior.meshes.first().vertices.size() <= SpatialInterior::maximumMeshVertices);
        QString geometry;
        for (const auto &anchor : plan.anchors)
            geometry += QString("%1,%2,%3;").arg(anchor.position.x()).arg(anchor.position.y()).arg(anchor.position.z());
        for (const auto &edge : plan.connections)
            geometry += QString("%1>%2;").arg(edge.from).arg(edge.to);
        layouts.insert(geometry);
        const auto repeat = SpatialWebWorld::plan(capacity, graph, options);
        VERIFY_WEB(repeat.anchors.size() == plan.anchors.size());
        for (int index = 0; index < plan.anchors.size(); ++index)
            VERIFY_WEB(repeat.anchors[index].position == plan.anchors[index].position);
        for (const auto &surface : world.interior.displays) {
            const auto node = std::find_if(graph.nodes.cbegin(), graph.nodes.cend(), [&](const SpatialWebNode &candidate) {
                return candidate.id == surface.id;
            });
            VERIFY_WEB(node != graph.nodes.cend());
            VERIFY_WEB(node->url == capacity.items[node->itemIndex].url);
            if (world.interior.locomotion == "walk") {
                SpatialWebPassage passage;
                VERIFY_WEB(passage.begin(world.interior, surface));
                VERIFY_WEB(passage.stage(world.interior));
                const float angle = surface.yaw * 3.14159265358979323846f / 180;
                const QVector3D direction(-std::sin(angle), 0, std::cos(angle));
                const auto connected = passage.room();
                VERIFY_WEB(connected.solids.size() <= SpatialWebPassage::solidBudget);
                VERIFY_WEB(connected.displays.size() <= SpatialWorkspace::maximumDisplays);
                for (int distance = 0; distance <= 950; distance += 25) {
                    const auto position = surface.portalCenter + direction * distance;
                    if (SpatialInteriorGeometry::blocked(connected, position))
                        std::fprintf(stderr, "Blocked passage: topology=%d item=%s distance=%d\n", int(topology), qPrintable(surface.id), distance);
                    VERIFY_WEB(!SpatialInteriorGeometry::blocked(connected, position));
                }
            }
        }
        auto extended = plan;
        SpatialMesh landmark;
        landmark.color = Qt::yellow;
        landmark.vertices = {{-100, 800, 0}, {100, 800, 0}, {0, 880, 0}};
        landmark.faces = {{0, 1, 2}};
        extended.meshes.append(landmark);
        const int nested = extended.transforms.size();
        extended.transforms.append({0, {0, 850, 0}, 35, 0.5f});
        extended.parts.append({nested, {}, {200, 100, 200}, Qt::cyan, SpatialSolidShape::Octahedron, false});
        VERIFY_WEB(SpatialWebWorld::compile(extended).success);
        extended.meshes[0].faces[0][0] = 20;
        VERIFY_WEB(!SpatialWebWorld::compile(extended).success);
        auto hidden = plan;
        const auto surface = hidden.displays.first();
        const auto corners = SpatialInteriorGeometry::surface(world.interior, surface);
        SpatialMesh curtain;
        curtain.color = Qt::black;
        const auto towardsAnchor = (surface.portalCenter - surface.center).normalized() * 20;
        for (const auto &corner : corners)
            curtain.vertices.append(corner + towardsAnchor);
        curtain.faces = {{0, 1, 2, 3}};
        hidden.meshes.append(curtain);
        VERIFY_WEB(!SpatialWebWorld::compile(hidden).success);
        auto obstructed = plan;
        obstructed.parts.append({0, plan.spawn, {200, 360, 200}, Qt::red});
        VERIFY_WEB(!SpatialWebWorld::compile(obstructed).success);
        auto disconnected = plan;
        disconnected.connections.clear();
        VERIFY_WEB(!SpatialWebWorld::compile(disconnected).success);
        auto cyclic = plan;
        cyclic.transforms[0].parent = 0;
        VERIFY_WEB(!SpatialWebWorld::compile(cyclic).success);
        auto invalid = plan;
        invalid.displays[0].center.setX(std::numeric_limits<float>::quiet_NaN());
        VERIFY_WEB(!SpatialWebWorld::compile(invalid).success);
        auto lost = plan;
        lost.displays.removeLast();
        VERIFY_WEB(!SpatialWebWorld::compile(lost).success);
        auto excessive = plan;
        excessive.parts.resize(SpatialInterior::maximumSolids + 1);
        VERIFY_WEB(!SpatialWebWorld::compile(excessive).success);
    }
    VERIFY_WEB(layouts.size() == 4);
    SpatialWorldOptions invalidOptions;
    invalidOptions.spacing = 100;
    const auto fallback = SpatialWebWorld::construct(capacity, invalidOptions);
    VERIFY_WEB(fallback.success && fallback.fallback && !fallback.diagnostic.isEmpty());
    VERIFY_WEB(fallback.interior.displays.size() == capacity.items.size());
    auto unsafe = capacity;
    unsafe.items[0].url = QUrl("javascript:alert(1)");
    VERIFY_WEB(!SpatialWebWorld::construct(unsafe, {}).success);
    for (int count : {0, 1}) {
        auto sparse = capacity;
        sparse.items.resize(count);
        VERIFY_WEB(SpatialWebWorld::construct(sparse, {}).success);
    }
    // Appearance evidence cannot rewrite semantic containment or URLs.
    auto themed = capacity;
    themed.evidence = "interstellar forest castle";
    const auto themedGraph = SpatialWebGraph::fromDocument(themed);
    VERIFY_WEB(themedGraph.nodes.size() == graph.nodes.size());
    for (int index = 0; index < graph.nodes.size(); ++index) {
        VERIFY_WEB(themedGraph.nodes[index].parent == graph.nodes[index].parent);
        VERIFY_WEB(themedGraph.nodes[index].url == graph.nodes[index].url);
    }

    // Slow pages can extend only four sections. Every orientation has a real
    // collision door, a clear walking lane, and a reversible coordinate rebase.
    for (float yaw : {0.0f, 90.0f, 180.0f, -90.0f}) {
        SpatialInterior source;
        source.size = {6000, 6000};
        SpatialDisplay entrance;
        entrance.id = "entrance";
        entrance.freestanding = true;
        entrance.portal = true;
        entrance.yaw = yaw;
        source.displays.append(entrance);
        SpatialWebPassage passage;
        VERIFY_WEB(passage.begin(source, entrance));
        const float angle = yaw * 3.14159265358979323846f / 180;
        const auto point = [angle](float distance) {
            return QVector3D(-std::sin(angle) * distance, 160, std::cos(angle) * distance);
        };
        VERIFY_WEB(passage.segments() == 2);
        VERIFY_WEB(SpatialInteriorGeometry::blocked(passage.room(), point(800)));
        VERIFY_WEB(passage.advance(point(450)) && passage.segments() == 3);
        VERIFY_WEB(passage.advance(point(850)) && passage.segments() == 4);
        VERIFY_WEB(!passage.advance(point(100000)) && passage.segments() == 4);
        VERIFY_WEB(SpatialInteriorGeometry::blocked(passage.room(), point(1600)));
        VERIFY_WEB(SpatialInteriorGeometry::blocked(passage.room(), point(1100)));
        for (int distance = 0; distance < 1030; distance += 20)
            VERIFY_WEB(!SpatialInteriorGeometry::blocked(passage.room(), point(distance)));
        SpatialInterior destination;
        destination.spawn = {0, -350};
        destination.displays.append(entrance);
        VERIFY_WEB(passage.stage(destination));
        VERIFY_WEB(!SpatialInteriorGeometry::blocked(passage.room(), point(1600)));
        VERIFY_WEB(!SpatialInteriorGeometry::blocked(passage.room(), point(1100)));
        VERIFY_WEB(passage.room().displays.last().id == "arrival:entrance");
        VERIFY_WEB(passage.arrived(point(1760)));
        VERIFY_WEB((passage.destinationPosition(point(1760)) - QVector3D(0, 160, -350)).length() < 0.01);
        VERIFY_WEB(passage.room().solids.size() <= SpatialWebPassage::solidBudget);
        VERIFY_WEB(passage.begin(source, entrance));
        passage.fail();
        VERIFY_WEB(!passage.stage(destination));
        VERIFY_WEB(!passage.advance(point(450)));
        VERIFY_WEB(SpatialInteriorGeometry::blocked(passage.room(), point(800)));
    }

    WorkspaceFlightView scene({}, false);
    scene.show();
    SpatialInterior controls;
    controls.size = {6000, 6000};
    controls.height = 2000;
    controls.locomotion = "drive";
    controls.spawnHeight = 110;
    scene.setInterior(controls, "Controls fixture");
    const auto driveStart = scene.cameraPosition();
    scene.setPilotInput({0, 0, 1}, {}, false);
    VERIFY_WEB(waitForWeb([&]() { return scene.cameraPosition().z() > driveStart.z() + 40; }, 1500));
    VERIFY_WEB(scene.cameraPosition().y() == 110);
    const auto steerStart = scene.cameraPosition();
    scene.setPilotInput({1, 0, 1}, {}, false);
    VERIFY_WEB(waitForWeb([&]() { return scene.cameraPosition().x() > steerStart.x() + 5; }, 1000));
    scene.setPilotInput({}, {}, false);
    VERIFY_WEB(waitForWeb([&]() { return !scene.moving(); }, 3000));
    scene.setInteriorLocomotion("fly");
    const auto flightStart = scene.cameraPosition();
    scene.setPilotInput({0, 1, 0}, {}, false);
    VERIFY_WEB(waitForWeb([&]() { return scene.cameraPosition().y() > flightStart.y() + 50; }, 1000));
    scene.setPilotInput({}, {}, false);
    VERIFY_WEB(waitForWeb([&]() { return !scene.moving(); }, 3000));
    scene.setInteriorLocomotion("walk");
    VERIFY_WEB(scene.cameraPosition().y() == 160);
    scene.hide();
    scene.setInterior(document.room(), "Fixture");
    VERIFY_WEB(!scene.setInteriorImage("missing", QImage(16, 16, QImage::Format_RGB32)));
    VERIFY_WEB(!scene.setInteriorImage("0", QImage(513, 320, QImage::Format_RGB32)));
#if defined(DOCKPIPE_SPATIAL_WEB) && defined(Q_OS_LINUX)
    const QByteArray renderer = qgetenv("QTWEBENGINEPROCESS_PATH");
    const bool hadRenderer = qEnvironmentVariableIsSet("QTWEBENGINEPROCESS_PATH");
    qputenv("QTWEBENGINEPROCESS_PATH", "/nonexistent/dockpipe-web-test/QtWebEngineProcess");
    {
        SpatialWebBrowser missing(false);
        missing.show();
        QApplication::processEvents();
        VERIFY_WEB(missing.status().contains("renderer is missing"));
        missing.navigate(QUrl("https://example.org"));
        missing.buildSpace();
        missing.showPage();
        missing.showSpace();
        missing.activateItem("0");
        missing.hide();
        VERIFY_WEB(!missing.building());
    }
    if (hadRenderer)
        qputenv("QTWEBENGINEPROCESS_PATH", renderer);
    else
        qunsetenv("QTWEBENGINEPROCESS_PATH");
#endif
    std::puts("Spatial document URL, bounds, duplicate identity, room, image and missing-runtime contracts passed.");
    return 0;
}

int spatialWebTest()
{
#ifndef DOCKPIPE_SPATIAL_WEB
    return 77;
#else
    if (QGuiApplication::platformName() == "offscreen")
        return 77;
    int delayedResponses = 1;
    int responseDelay = 5500;
    QTcpServer server;
    VERIFY_WEB(server.listen(QHostAddress::LocalHost));
    const QUrl base(QString("http://127.0.0.1:%1").arg(server.serverPort()));
    QObject::connect(&server, &QTcpServer::newConnection, &server, [&]() {
        while (auto *socket = server.nextPendingConnection()) {
            QObject::connect(socket, &QTcpSocket::disconnected, socket, &QObject::deleteLater);
            QObject::connect(socket, &QTcpSocket::readyRead, socket, [socket, &delayedResponses, &responseDelay]() {
                auto request = socket->property("request").toByteArray() + socket->readAll();
                if (!request.contains("\r\n\r\n")) {
                    socket->setProperty("request", request);
                    return;
                }
                const auto path = request.split(' ').value(1);
                QByteArray body;
                QByteArray type = "text/html; charset=utf-8";
                if (path == "/art.svg") {
                    type = "image/svg+xml";
                    body = "<svg xmlns='http://www.w3.org/2000/svg' width='480' height='240'><rect width='480' height='240' fill='#187f91'/><circle cx='240' cy='120' r='90' fill='#e8bc5d'/><text x='130' y='210' font-size='44' fill='#10232b'>LEFT →</text></svg>";
                } else if (path.startsWith("/product.png")) {
                    type = "image/png";
                    QImage product(320, 240, QImage::Format_ARGB32);
                    product.fill(Qt::white);
                    QPainter painter(&product);
                    painter.setPen(Qt::NoPen);
                    painter.setBrush(QColor::fromHsv((path.mid(path.indexOf('=') + 1).toInt() * 23 + 8) % 360, 180, 175));
                    painter.drawRoundedRect(QRect(100, 45, 120, 160), 15, 15);
                    painter.setBrush(QColor("#292f36"));
                    painter.drawRoundedRect(QRect(125, 25, 70, 30), 8, 8);
                    painter.end();
                    QBuffer buffer(&body);
                    buffer.open(QIODevice::WriteOnly);
                    product.save(&buffer, "PNG");
                } else if (path == "/vehicles") {
                    body = "<title>Vehicle inventory</title><main><h1>Cars and trucks</h1>";
                    for (const auto &model : {"2026 Pickup", "2025 SUV", "2024 Coupe", "2023 Van"})
                        body += "<article itemtype='https://schema.org/Vehicle'><a href='/vehicles/" + QByteArray(model).replace(' ', '-')
                            + "'><h2>" + model + "</h2><img src='/art.svg'></a></article>";
                    body += "<a href='/vehicles/2027-pickup-news'>Receive up-to-date information on the 2027 pickup</a></main>";
                } else if (path == "/mixed") {
                    body = "<title>Woodland outfitters and fantasy books</title><style>body{background:#ece9df;font:20px sans-serif}main{display:grid;grid-template-columns:repeat(3,1fr)}img{width:240px}</style><main>";
                    for (int index = 0; index < 12; ++index) {
                        const QByteArray title = index % 2 ? "Dragon novels about knights and castles" : "Tent for camping in the woods";
                        body += "<article itemtype='https://schema.org/Product'><a href='/second?item=" + QByteArray::number(index)
                            + "'><h2>" + title + "</h2><img src='/product.png?id=" + QByteArray::number(index) + "'></a></article>";
                    }
                    body += "</main>";
                } else if (path == "/second") {
                    body = "<title>Second destination</title><main><h1>Repository interior</h1><a href='/'>Back to gallery</a></main>";
                } else if (path.startsWith("/watch?")) {
                    body = "<title>Video fixture</title><main><video controls width='640' height='360'></video></main><script>const c=document.createElement('canvas');c.width=640;c.height=360;const g=c.getContext('2d');let n=0;setInterval(()=>{g.fillStyle='hsl('+(n++*17%360)+',90%,50%)';g.fillRect(0,0,640,360);},83);document.querySelector('video').srcObject=c.captureStream(12);</script>";
                } else if (path == "/empty") {
                    body = "<title>Empty destination</title><main>No navigable content</main>";
                } else if (path == "/invertebrates") {
                    body = "<title>Invertebrate animals</title><style>article{display:inline-block;width:290px}img{width:280px;height:160px}a{display:block;font:22px sans-serif}</style>"
                        "<div class='BottomNavBar'><a href='/'>Animals home</a><a href='/games'>Games</a></div>"
                        "<h1>Invertebrates</h1><main><a href='/invertebrates'>Current category</a>";
                    for (const auto &name : {"Black Widow", "Christmas Island Red Crab", "Cicada", "Dung Beetle", "Earthworm"})
                        body += "<article><img src='/art.svg' alt='" + QByteArray(name) + "'><h2><a href='/animal-facts?name="
                            + QUrl::toPercentEncoding(name) + "'>" + name + "</a></h2></article>";
                    body += "<a href='/invertebrates?page=2'><img src='/art.svg'>load more</a></main>";
                } else if (path.startsWith("/animal-facts?")) {
                    body = "<script src='/parser-script'></script><title>Black Widow animal facts</title><meta property='og:type' content='article'>"
                        "<main><article><h1>Black Widow</h1><p>This controlled fixture describes the animal in source text. "
                        "The factual content belongs on an exhibit inside the room instead of opening the category again.</p>"
                        "<img src='/art.svg' width='480' height='240' alt='Black Widow source illustration'>"
                        "<a href='/invertebrates'>Invertebrate collection</a></article></main>";
                } else if (path == "/parser-script") {
                    type = "application/javascript";
                    body = "/* Delayed parser-blocking resource. */";
                } else if (path == "/habitats") {
                    body = "<title>Habitat collection</title><main><h1>Natural habitats</h1>"
                        "<a href='/rainforest'><h2>Rainforest Habitat</h2></a></main>";
                } else if (path == "/image-fixture") {
                    body = R"HTML(<title>Modern image fixture</title><meta property="og:type" content="article">
                        <style>img{width:360px;height:180px;display:block}</style><main><article><h1>Image collection</h1>
                        <p>This fixture contains source illustrations arriving through hydration, lazy attributes and a delayed image response. Images must fill existing exhibits without moving the visitor.</p>
                        <div hidden><img alt="Hidden photo" data-src="/art.svg"></div>
                        <img id="hydrate" alt="Hydrated photo" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7">
                        <img alt="Lazy photo" data-src="/art.svg" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7">
                        <picture><source data-srcset="/art.svg 480w"><img alt="Responsive photo" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"></picture>
                        <img alt="Unresolved placeholder" src="data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7">
                        </article></main><script>class Hydrator { static { setTimeout(() => {
                            const replacement = document.getElementById('hydrate').cloneNode();
                            replacement.src = ['/slow-photo.svg'].at(-1);
                            document.getElementById('hydrate').replaceWith(replacement);
                        }, 1800); } }</script>)HTML";
                } else if (path == "/slow-photo.svg") {
                    type = "image/svg+xml";
                    body = "<svg xmlns='http://www.w3.org/2000/svg' width='480' height='240'><rect width='480' height='240' fill='#187f91'/><circle cx='240' cy='120' r='90' fill='#e8bc5d'/></svg>";
                } else if (path == "/rainforest") {
                    body = "<title>Rainforest habitat</title><meta property='og:type' content='article'>"
                        "<main><article><h1>Rainforest habitat</h1>"
                        "<div aria-roledescription='carousel'><p>Gallery caption and repeated image rights notice belong beside the photograph, not in an article excerpt.</p><span>1 / 3</span></div>"
                        "<p class='Image__Copyright'>Image rights notice repeated by the gallery must not become an educational exhibit in this fixture.</p>"
                        "<p>Fixture article: tall trees form a canopy above the forest floor. This educational page presents habitat information, photographs and related destinations.</p>"
                        "<p>Copyright law is also a valid article topic. <span class='Image__Copyright'>Nested image rights notice.</span>This text should remain readable while the nested gallery label is excluded.</p>"
                        "<img src='/art.svg' width='480' height='240' alt='Forest habitat illustration'>"
                        "<video width='480' height='240' poster='/art.svg'></video>"
                        "<a href='/second'><h2>Explore another habitat</h2></a>"
                        "</article></main><aside><a href='/unrelated'><h2>Unrelated state navigation</h2></a></aside>"
                        "<script src='/slow-resource'></script>";
                } else if (path == "/slow-resource") {
                    type = "image/png";
                } else {
                    body = "<title>Spatial browser fixture</title><style>body{background:#10232b;color:white;font:20px sans-serif;margin:32px}main{display:grid;grid-template-columns:repeat(3,1fr);gap:20px}a{color:white;padding:12px;background:#234451}img{width:100%}nav{display:none}</style><h1>Live website gallery</h1><main>";
                    for (int index = 0; index < 16; ++index) {
                        if (path == "/store")
                            body += "<article itemtype='https://schema.org/Product'>";
                        body += "<a href='" + QByteArray(path == "/cinema" ? "/watch?film=" : "/second?item=") + QByteArray::number(index) + "'><img src='/art.svg' alt='Real page image'><h2>Exhibit " + QByteArray::number(index + 1) + "</h2></a>";
                        if (path == "/store")
                            body += "<span itemprop='price'>$25.00</span></article>";
                    }
                    if (path == "/cinema")
                        body += "<video width='480' height='240' poster='/art.svg'></video>";
                    body += "</main><a href='javascript:alert(1)'>Unsafe target</a><script>setTimeout(()=>{document.title='JavaScript rendered gallery';if(!document.querySelector('video'))document.querySelector('a').href='/second'},80)</script>";
                }
                const QByteArray response = "HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Type: " + type
                    + "\r\nContent-Length: " + QByteArray::number(body.size()) + "\r\n\r\n" + body;
                int delay = 0;
                if (path == "/parser-script")
                    delay = 1200;
                else if (path == "/slow-resource")
                    delay = 6000;
                else if (path == "/slow-photo.svg")
                    delay = 1600;
                else if (path == "/second" && delayedResponses > 0)
                    delay = responseDelay;
                if (delay && path == "/second")
                    --delayedResponses;
                QTimer::singleShot(delay, socket, [socket, response]() {
                    socket->write(response);
                    socket->disconnectFromHost();
                });
            });
        }
    });
    CockpitWidget cockpit;
    cockpit.resize(1480, 960);
    cockpit.show();
    VERIFY_WEB(!cockpit.findChild<SpatialWebBrowser *>());
    cockpit.visit("web");
    QPointer<SpatialWebBrowser> browser = cockpit.findChild<SpatialWebBrowser *>();
    VERIFY_WEB(browser && browser->isVisible());
    auto *mainViews = cockpit.findChild<QStackedWidget *>("cockpitMainViews");
    VERIFY_WEB(mainViews && mainViews->currentWidget() == browser);
    VERIFY_WEB(browser->parentWidget() == mainViews && cockpit.activePanel().isEmpty());
    VERIFY_WEB(!cockpit.flight()->isVisible());
    auto *start = browser->findChild<QWidget *>("spatialWebStart");
    auto *address = browser->findChild<QLineEdit *>("spatialWebAddress");
    auto *go = browser->findChild<QPushButton *>("spatialWebGo");
    auto *explore = browser->findChild<QPushButton *>("spatialWebExplore");
    VERIFY_WEB(start && address && go && explore);
    VERIFY_WEB(waitForWeb([&]() { return browser->spatial() && browser->document().title.contains("Headquarters"); }));
    VERIFY_WEB(!start->isVisible() && address->text().isEmpty());
    VERIFY_WEB(explore->isEnabled());
    VERIFY_WEB(!browser->findChild<QPushButton *>("spatialWebBuild")->isEnabled());
    browser->showPage();
    VERIFY_WEB(browser->spatial() && !start->isVisible());
    browser->scene()->setFlightFullscreen(true);
    browser->showSearch();
    VERIFY_WEB(cockpit.isFullScreen());
    auto *terminal = browser->findChild<QWidget *>("spatialCityTerminal");
    auto *query = browser->findChild<QLineEdit *>("spatialCityQuery");
    auto *submitSearch = browser->findChild<QPushButton *>("spatialCitySubmit");
    auto *cancelSearch = browser->findChild<QPushButton *>("spatialCityCancel");
    VERIFY_WEB(terminal && query && submitSearch && cancelSearch && terminal->isVisible());
    submitSearch->click();
    VERIFY_WEB(!browser->building() && browser->document().title.contains("Headquarters"));
    const auto terminalCapture = qEnvironmentVariable("DOCKPIPE_SEARCH_TERMINAL_CAPTURE");
    if (!terminalCapture.isEmpty()) {
        waitForWeb([]() { return false; }, 300);
        VERIFY_WEB(cockpit.screen()->grabWindow(cockpit.winId()).save(terminalCapture));
    }
    cancelSearch->click();
    VERIFY_WEB(browser->spatial() && !terminal->isVisible() && cockpit.isFullScreen());
    browser->scene()->setFlightFullscreen(false);
    const QString startScreenshot = qEnvironmentVariable("DOCKPIPE_WEB_START_SCREENSHOT");
    if (!startScreenshot.isEmpty()) {
        cockpit.raise();
        cockpit.activateWindow();
        QElapsedTimer settle;
        settle.start();
        while (settle.elapsed() < 350) {
            QApplication::processEvents();
            QThread::msleep(5);
        }
        VERIFY_WEB(cockpit.screen()->grabWindow(cockpit.winId()).save(startScreenshot));
    }
    browser->scene()->setSurfaceSpawn({1800, -2300});
    VERIFY_WEB(browser->scene()->activateInteriorAction("drone"));
    VERIFY_WEB(browser->scene()->interiorLocomotion() == "fly");
    browser->scene()->orbitRequested();
    VERIFY_WEB(mainViews->currentWidget() == cockpit.flight());
    cockpit.visit("web");
    VERIFY_WEB(browser->spatial() && browser->scene()->interiorLocomotion() == "walk");
    VERIFY_WEB(browser->scene()->cameraPosition().z() == -2300);
    if (qEnvironmentVariableIsSet("DOCKPIPE_CIVIC_STARTUP_ONLY")) {
        std::puts("HQ startup, drone departure, orbit and safe return to street passed.");
        return 0;
    }
    int ready = 0;
    QObject::connect(browser, &SpatialWebBrowser::spaceReady, browser, [&]() { ++ready; });
    const QString liveUrl = qEnvironmentVariable("DOCKPIPE_WEB_LIVE_URL");
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_SEARCH_PROBE")) {
        VERIFY_WEB(!liveUrl.isEmpty());
        // Deliberately use the default site-wide setting: search URLs must bypass
        // crawler filtering and reach the browser's result-to-city interpretation.
        if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_NATIVE_SEARCH")) {
            browser->showSearch();
            query->setText(QUrlQuery(QUrl(liveUrl)).queryItemValue("q"));
            submitSearch->click();
            VERIFY_WEB(terminal->isVisible());
        } else {
            browser->navigate(QUrl(liveUrl));
        }
        const bool completed = waitForWeb([&]() { return ready > 0; }, 45000);
        std::fprintf(stderr, "Search completed=%d venue=%s count=%lld status=%s\n", completed,
            qPrintable(browser->document().venue), static_cast<long long>(browser->document().items.size()),
            qPrintable(browser->status()));
        auto *live = browser->findChild<QWebEngineView *>("spatialWebLivePage");
        bool inspected = false;
        live->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
        live->page()->runJavaScript(R"JS(JSON.stringify({url:location.href,title:document.title,
            state:document.readyState,
            headings:Array.from(document.querySelectorAll('h3')).slice(0,12).map(n=>n.textContent.slice(0,140))}))JS",
            [&](const QVariant &value) {
                std::fprintf(stderr, "Search page: %s\n", qPrintable(value.toString()));
                inspected = true;
            });
        VERIFY_WEB(waitForWeb([&]() { return inspected; }));
        const auto capture = qEnvironmentVariable("DOCKPIPE_WEB_SCREENSHOT");
        if (!capture.isEmpty()) {
            waitForWeb([]() { return false; }, 300);
            VERIFY_WEB(cockpit.screen()->grabWindow(cockpit.winId()).save(capture));
        }
        VERIFY_WEB(completed && browser->document().venue == "city" && !browser->document().items.isEmpty());
        if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_SEARCH_FOLLOW")) {
            const auto destination = browser->document().items.first();
            const int previousReady = ready;
            browser->activateItem(destination.id);
            const bool followed = waitForWeb([&]() {
                if (browser->transitioning())
                    browser->scene()->setPilotInput({0, 0, 1}, {}, true);
                return ready > previousReady && !browser->transitioning()
                    && browser->document().venue != "city" && !browser->document().items.isEmpty();
            }, 45000);
            browser->scene()->setPilotInput({}, {}, false);
            std::fprintf(stderr, "Search entrance followed=%d source=%s status=%s\n", followed,
                qPrintable(browser->document().url.toString()), qPrintable(browser->status()));
            VERIFY_WEB(followed);
        }
        return 0;
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_IMAGE_FIXTURE")) {
        browser->navigate(base.resolved(QUrl("/image-fixture")), false);
        VERIFY_WEB(waitForWeb([&]() { return browser->building() && browser->document().title == "Modern image fixture"; }));
        browser->scene()->setSurfaceSpawn(browser->document().room().spawn + QPointF(0, 100));
        const auto camera = browser->scene()->cameraPosition();
        VERIFY_WEB(waitForWeb([&]() { return ready > 0; }));
        VERIFY_WEB(browser->scene()->cameraPosition() == camera);
        int photographs = 0;
        int placeholders = 0;
        for (const auto &item : browser->document().items) {
            if (item.kind != "image")
                continue;
            if (item.title == "Unresolved placeholder") {
                VERIFY_WEB(item.visual.isNull());
                ++placeholders;
            } else {
                VERIFY_WEB(!item.visual.isNull());
                ++photographs;
            }
        }
        VERIFY_WEB(photographs == 3 && placeholders == 1);
        auto *live = browser->findChild<QWebEngineView *>("spatialWebLivePage");
        VERIFY_WEB(live->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
        browser->navigate(base.resolved(QUrl("/image-fixture?retry")), false);
        browser->showHeadquarters();
        waitForWeb([]() { return false; }, 1000);
        VERIFY_WEB(!browser->building() && browser->document().title.contains("Dockpipe"));
        std::puts("Modern hydration, lazy/responsive media, placeholder rejection, bounded capture and cancellation passed.");
        return 0;
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_IMAGE_PROBE")) {
        VERIFY_WEB(!liveUrl.isEmpty());
        auto *live = browser->findChild<QWebEngineView *>("spatialWebLivePage");
        QWebEngineScript errors;
        errors.setName("fixture-image-errors");
        errors.setInjectionPoint(QWebEngineScript::DocumentCreation);
        errors.setWorldId(QWebEngineScript::MainWorld);
        errors.setSourceCode("window.addEventListener('error', e => console.error('IMAGE_PROBE ' + e.message + ' ' + e.filename + ':' + e.lineno + ':' + e.colno));");
        live->page()->scripts().insert(errors);
        browser->navigate(QUrl(liveUrl), false);
        VERIFY_WEB(waitForWeb([&]() { return live->url() == QUrl(liveUrl); }, 30000));
        VERIFY_WEB(waitForWeb([&]() { return ready > 0; }, 30000));
        int coldVisuals = 0;
        for (const auto &item : browser->document().items) {
            if (!item.visual.isNull())
                ++coldVisuals;
        }
        std::fprintf(stderr, "Cold image world: %d captured photographs; %s\n", coldVisuals, qPrintable(browser->status()));
        VERIFY_WEB(coldVisuals > 0);
        browser->showPage();
        waitForWeb([]() { return false; }, 25000);
        bool inspected = false;
        live->page()->runJavaScript(R"JS(JSON.stringify({agent:navigator.userAgent,
            state:document.readyState, title:document.title,
            images:Array.from(document.images).slice(0,40).map(n=>({
                html:n.outerHTML.slice(0,2200), source:n.currentSrc, loaded:n.complete,
                pixels:[n.naturalWidth,n.naturalHeight], size:[n.width,n.height]})),
            resources:performance.getEntriesByType('resource').filter(n=>n.initiatorType==='img').slice(0,40)
                .map(n=>({url:n.name,duration:n.duration,bytes:n.transferSize}))}))JS",
            [&](const QVariant &value) {
                std::fprintf(stderr, "Image probe: %s\n", qPrintable(value.toString()));
                inspected = true;
            });
        VERIFY_WEB(waitForWeb([&]() { return inspected; }));
        const QString capture = qEnvironmentVariable("DOCKPIPE_WEB_SCREENSHOT");
        if (!capture.isEmpty())
            VERIFY_WEB(live->grab().save(capture));
        const int previousReady = ready;
        browser->buildSpace();
        VERIFY_WEB(waitForWeb([&]() { return ready > previousReady; }, 20000));
        int visuals = 0;
        for (const auto &item : browser->document().items) {
            if (!item.visual.isNull()) {
                if (visuals == 0 && !capture.isEmpty())
                    VERIFY_WEB(item.visual.save(capture + ".exhibit.png"));
                ++visuals;
            }
        }
        std::fprintf(stderr, "Image world: %d captured photographs; %s\n", visuals, qPrintable(browser->status()));
        VERIFY_WEB(visuals > 0);
        browser->hide();
        return 0;
    }
    if (!liveUrl.isEmpty() && liveUrl == address->text()) {
        explore->click();
    } else {
        address->setText(liveUrl.isEmpty() ? base.resolved(QUrl(qEnvironmentVariableIsSet("DOCKPIPE_WEB_TEST_VEHICLES") ? "/vehicles" : "/")).toString() : liveUrl);
        go->click();
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_SITE_PROBE")) {
        auto *crawler = browser->siteExplorer()->crawler();
        const bool loaded = waitForWeb([&]() {
            return ready > 0 && !browser->building() && !browser->document().items.isEmpty()
                && browser->document().items.first().id != "site:waiting";
        }, 45000);
        std::fprintf(stderr, "Live site loaded=%d status=%s discovery=%s title=%s venue=%s\n", loaded,
            qPrintable(browser->status()), qPrintable(crawler->status()), qPrintable(browser->document().title),
            qPrintable(browser->document().venue));
        for (const auto &page : crawler->world().pages)
            std::fprintf(stderr, "Source %s: %s\n", qPrintable(page.node.url.toString()), qPrintable(page.reason));
        for (const auto &item : browser->document().items)
            std::fprintf(stderr, "Item %s %s: %s\n", qPrintable(item.kind), qPrintable(item.id), qPrintable(item.title));
        const auto path = qEnvironmentVariable("DOCKPIPE_WEB_SCREENSHOT");
        const auto captureProbe = [&](const QString &file) {
            browser->scene()->viewport()->update();
            waitForWeb([]() { return false; }, 350);
            // Read only this fixture's pixels even when another app is in front.
            if (auto *viewport = qobject_cast<QOpenGLWidget *>(browser->scene()->viewport()))
                return viewport->grabFramebuffer().save(file);
            return browser->scene()->viewport()->grab().save(file);
        };
        if (!path.isEmpty())
            VERIFY_WEB(captureProbe(path));
        const QString follow = qEnvironmentVariable("DOCKPIPE_WEB_FOLLOW_TITLE");
        if (loaded && !follow.isEmpty()) {
            const auto items = browser->document().items;
            const auto target = std::find_if(items.cbegin(), items.cend(), [&](const auto &item) { return item.title == follow; });
            VERIFY_WEB(target != items.cend());
            const int previousReady = ready;
            const auto displays = browser->scene()->interiorDisplays();
            const auto entrance = std::find_if(displays.cbegin(), displays.cend(),
                [&](const auto &display) { return display.id == target->id && display.portal; });
            if (entrance != displays.cend()) {
                const float angle = entrance->yaw * 3.14159265358979323846f / 180;
                const QVector3D outward(std::sin(angle), 0, -std::cos(angle));
                const auto approach = entrance->portalCenter + outward * 140;
                browser->scene()->setSurfaceSpawn({approach.x(), approach.z()});
                const int turnPixels = qRound(std::atan2(-outward.x(), -outward.z()) / 0.004);
                QMouseEvent press(QEvent::MouseButtonPress, QPointF(600, 400), QPointF(600, 400),
                    Qt::RightButton, Qt::RightButton, Qt::NoModifier);
                QApplication::sendEvent(browser->scene()->viewport(), &press);
                QMouseEvent turn(QEvent::MouseMove, QPointF(600 + turnPixels, 400), QPointF(600 + turnPixels, 400),
                    Qt::NoButton, Qt::RightButton, Qt::NoModifier);
                QApplication::sendEvent(browser->scene()->viewport(), &turn);
                QMouseEvent release(QEvent::MouseButtonRelease, QPointF(600 + turnPixels, 400), QPointF(600 + turnPixels, 400),
                    Qt::RightButton, Qt::NoButton, Qt::NoModifier);
                QApplication::sendEvent(browser->scene()->viewport(), &release);
            }
            browser->activateItem(target->id);
            if (entrance != displays.cend())
                browser->scene()->setPilotInput({0, 0, 1}, {}, true);
            QString previousStatus;
            int statusUpdates = 0;
            const bool followed = waitForWeb([&]() {
                if (SpatialWebDocument::samePage(browser->document().url, target->url) && !browser->transitioning()) {
                    browser->scene()->setPilotInput({}, {}, false);
                } else if (entrance != displays.cend() && browser->transitioning()) {
                    // Keep this disposable probe moving if desktop focus changes.
                    browser->scene()->setPilotInput({0, 0, 1}, {}, true);
                }
                if (browser->status() != previousStatus && statusUpdates < 32) {
                    previousStatus = browser->status();
                    ++statusUpdates;
                    std::fprintf(stderr, "Following: %s\n", qPrintable(previousStatus.left(500)));
                }
                return ready > previousReady && !browser->transitioning();
            }, 45000);
            browser->scene()->setPilotInput({}, {}, false);
            std::fprintf(stderr, "Follow target=%s published=%s status=%s\n", qPrintable(target->url.toString()),
                qPrintable(browser->document().url.toString()), qPrintable(browser->status()));
            if (!followed && entrance != displays.cend()) {
                const auto position = browser->scene()->cameraPosition();
                const float angle = entrance->yaw * 3.14159265358979323846f / 180;
                const QVector3D forward(-std::sin(angle), 0, std::cos(angle));
                std::fprintf(stderr, "Passage distance=%.1f camera=%.1f,%.1f,%.1f\n",
                    QVector3D::dotProduct(position - entrance->portalCenter, forward), position.x(), position.y(), position.z());
            }
            if (!followed && !path.isEmpty())
                VERIFY_WEB(captureProbe(path + ".failed.png"));
            if (!SpatialWebDocument::samePage(browser->document().url, target->url)) {
                auto *live = browser->findChild<QWebEngineView *>("spatialWebLivePage");
                bool inspected = false;
                live->page()->runJavaScript(R"JS(JSON.stringify({url:location.href,title:document.title,
                    state:document.readyState,html:document.documentElement.outerHTML.slice(0,3000),
                    h1:Array.from(document.querySelectorAll('h1')).slice(0,3).map(n=>n.textContent),
                    paragraphs:Array.from(document.querySelectorAll('article p')).slice(0,6).map(n=>({
                        size:[n.getBoundingClientRect().width,n.getBoundingClientRect().height],
                        hidden:!!n.closest('[hidden],[aria-hidden="true"]'), display:getComputedStyle(n).display,
                        length:n.textContent.length})), article:document.querySelector('meta[property="og:type"]')?.content}))JS",
                    [&](const QVariant &value) { std::fprintf(stderr, "Article DOM: %s\n", qPrintable(value.toString())); inspected = true; });
                VERIFY_WEB(waitForWeb([&]() { return inspected; }));
            }
            VERIFY_WEB(followed);
            VERIFY_WEB(SpatialWebDocument::samePage(browser->document().url, target->url));
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && !item.description.isEmpty(); }));
            std::fprintf(stderr, "Live source article reached: %s · %lld exhibits\n", qPrintable(browser->document().url.toString()),
                static_cast<long long>(browser->document().items.size()));
            for (const auto &item : browser->document().items)
                std::fprintf(stderr, "Destination %s: %s\n", qPrintable(item.kind), qPrintable(item.title));
            if (!path.isEmpty())
                VERIFY_WEB(captureProbe(path + ".article.png"));
        }
        browser->hide();
        return loaded ? 0 : 1;
    }
    VERIFY_WEB(waitForWeb([&]() { return browser->building() && browser->spatial() && !browser->document().items.isEmpty(); }, 30000));
    VERIFY_WEB(ready == 0 && !browser->document().items.isEmpty());
    browser->scene()->setSurfaceSpawn(browser->document().room().spawn + QPointF(0, 100));
    const auto streamingCamera = browser->scene()->cameraPosition();
    VERIFY_WEB(waitForWeb([&]() { return ready > 0; }, 30000));
    VERIFY_WEB(browser->scene()->cameraPosition() == streamingCamera);
    std::printf("Web title: %s; exhibits: %lld; spatial: %d\n", qPrintable(browser->document().title), static_cast<long long>(browser->document().items.size()), browser->spatial());
    VERIFY_WEB(browser->spatial());
    const QString screenshot = qEnvironmentVariable("DOCKPIPE_WEB_SCREENSHOT");
    const auto captureWorld = [&](const QString &path) {
        // QWidget::grab does not reliably composite nested OpenGL viewports.
        // Inspect the fixture window's actual displayed pixels, as the native
        // app-handoff regression does, so a blank GPU scene cannot pass.
        cockpit.raise();
        cockpit.activateWindow();
        browser->scene()->viewport()->update();
        QElapsedTimer settle;
        settle.start();
        while (settle.elapsed() < 350) {
            QApplication::processEvents();
            QThread::msleep(5);
        }
        const auto *viewport = browser->scene()->viewport();
        const QPoint origin = viewport->mapTo(&cockpit, QPoint());
        const QImage pixels = cockpit.screen()->grabWindow(cockpit.winId(), origin.x(), origin.y(),
            viewport->width(), viewport->height()).toImage();
        int colored = 0;
        int samples = 0;
        QSet<QRgb> colors;
        for (int y = pixels.height() / 5; y < pixels.height() * 4 / 5; y += 8) {
            for (int x = 20; x < pixels.width() - 20; x += 8) {
                const QColor pixel = pixels.pixelColor(x, y);
                colors.insert(pixel.rgb());
                ++samples;
                if (std::abs(pixel.blue() - pixel.red()) > 12 || std::abs(pixel.green() - pixel.red()) > 12)
                    ++colored;
            }
        }
        std::printf("visible_world_colored=%d/%d distinct_colors=%lld\n", colored, samples, static_cast<long long>(colors.size()));
        if (!path.isEmpty() && !cockpit.screen()->grabWindow(cockpit.winId()).save(path))
            return false;
        if (!browser->scene()->activeVehicleId().isEmpty()) {
            // An empty road naturally has few colours. Verify the actual cabin
            // surfaces rather than demanding gallery detail in the windscreen.
            const QColor pillar = pixels.pixelColor(5, pixels.height() / 2);
            const QColor dashboard = pixels.pixelColor(pixels.width() / 2, pixels.height() * 83 / 100);
            const QColor bonnet = pixels.pixelColor(pixels.width() / 2, pixels.height() * 75 / 100);
            return pillar == QColor("#15212c") && dashboard == pillar && bonnet != pillar
                && colors.size() >= 6 && colored > samples / 3;
        }
        return samples > 0 && colored > samples / 3 && colors.size() > 16;
    };
    VERIFY_WEB(captureWorld(screenshot));
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_TEST_INTELLIGENCE")) {
        VERIFY_WEB(!qEnvironmentVariableIsEmpty("DOCKPIPE_COCKPIT_NEURAL_ROOT"));
        auto *enhance = browser->findChild<QToolButton *>("spatialWebEnhance");
        auto *apply = browser->findChild<QPushButton *>("spatialWebApplyDesign");
        VERIFY_WEB(enhance && apply);
        if (enhance->isChecked())
            enhance->click();
        if (qEnvironmentVariableIsSet("DOCKPIPE_TEST_NEURAL_GPU")) {
            for (auto *action : enhance->menu()->actions()) {
                if (action->text().startsWith("Prefer GPU"))
                    action->trigger();
            }
        }
        int cutouts = 0;
        auto *neural = browser->findChild<SpatialNeuralService *>();
        VERIFY_WEB(neural);
        QObject::connect(neural, &SpatialNeuralService::imageReady, browser, [&](const QString &, const QImage &) { ++cutouts; });
        browser->navigate(base.resolved(QUrl("/mixed")));
        VERIFY_WEB(waitForWeb([&]() { return browser->document().url.path() == "/mixed" && !browser->building(); }, 30000));
        enhance->click();
        const auto origin = browser->scene()->cameraPosition();
        browser->scene()->setSurfaceSpawn({origin.x(), origin.z() - 120});
        const auto exploring = browser->scene()->cameraPosition();
        VERIFY_WEB(waitForWeb([&]() { return apply->isVisible(); }, 15000));
        VERIFY_WEB(browser->scene()->cameraPosition() == exploring);
        apply->click();
        VERIFY_WEB(browser->document().worldFeatures.size() == 8);
        VERIFY_WEB(browser->document().items.first().worldFeatures != browser->document().items.last().worldFeatures);
        const int cutoutsBeforeMovement = cutouts;
        auto *scene = browser->scene();
        scene->setPilotInput({0, 0, 1}, {}, false);
        QElapsedTimer movement;
        movement.start();
        QVector<double> times;
        auto previousFrame = scene->renderedFrames();
        while (movement.elapsed() < 1200) {
            QApplication::processEvents();
            if (scene->renderedFrames() != previousFrame) {
                times.append(scene->lastDrawNanoseconds() / 1e6);
                previousFrame = scene->renderedFrames();
            }
            QThread::msleep(2);
        }
        scene->setPilotInput({}, {}, false);
        VERIFY_WEB(!times.isEmpty());
        std::sort(times.begin(), times.end());
        std::printf("neural_world_frames=%lld interval_ms=%lld draw_p95_ms=%.3f\n",
            static_cast<long long>(times.size()), static_cast<long long>(movement.elapsed()),
            times[qMin(times.size() - 1, times.size() * 95 / 100)]);
        std::printf("cutouts_during_movement=%d backend=%s\n", cutouts - cutoutsBeforeMovement, qPrintable(browser->status()));
        VERIFY_WEB(cutouts > cutoutsBeforeMovement);
        VERIFY_WEB(waitForWeb([&]() {
            return browser->document().items.first().poster.pixelColor(120, 40).alpha() < 50;
        }, 15000));
        VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".enhanced.png"));
        const auto current = scene->cameraPosition();
        enhance->click();
        VERIFY_WEB(!enhance->isChecked() && scene->cameraPosition() == current);
        cockpit.visit("files");
        std::puts("Native neural world, deferred design, movement and cancellation passed.");
        return 0;
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_TEST_VEHICLES")) {
        auto *scene = browser->scene();
        const auto cars = scene->vehicles();
        std::printf("vehicle_count=%lld venue=%s\n", static_cast<long long>(cars.size()), qPrintable(browser->document().venue));
        for (const auto &car : cars)
            std::printf("vehicle=%s body=%s year=%d\n", qPrintable(car.name), qPrintable(car.body), car.year);
        VERIFY_WEB(!cars.isEmpty());
        if (liveUrl.isEmpty()) {
            VERIFY_WEB(cars.size() == 4);
            VERIFY_WEB(cars[0].year == 2026 && cars[0].body == "pickup");
        }
        const auto car = cars.first();
        const auto inspect = car.position + QVector3D(0, 0, -650);
        scene->setSurfaceSpawn({inspect.x(), inspect.z()});
        VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".car.png"));
        const auto door = car.position + SpatialVehicleModel::rotate({-190, 0, 0}, car.heading);
        scene->setSurfaceSpawn({door.x(), door.z()});
        const auto direction = car.position + QVector3D(0, SpatialVehicleModel::size(car).y() / 2, 0) - scene->cameraPosition();
        const int horizontal = qRound(std::atan2(direction.x(), direction.z()) / .004);
        const int vertical = qRound(-std::atan2(direction.y(), std::hypot(direction.x(), direction.z())) / .004);
        QMouseEvent press(QEvent::MouseButtonPress, QPointF(400, 300), QPointF(400, 300), Qt::RightButton, Qt::RightButton, Qt::NoModifier);
        QMouseEvent look(QEvent::MouseMove, QPointF(400 + horizontal, 300 + vertical), QPointF(400 + horizontal, 300 + vertical), Qt::NoButton, Qt::RightButton, Qt::NoModifier);
        QMouseEvent release(QEvent::MouseButtonRelease, look.position(), look.position(), Qt::RightButton, Qt::NoButton, Qt::NoModifier);
        QApplication::sendEvent(scene->viewport(), &press);
        QApplication::sendEvent(scene->viewport(), &look);
        QApplication::sendEvent(scene->viewport(), &release);
        QKeyEvent enter(QEvent::KeyPress, Qt::Key_E, Qt::NoModifier);
        QApplication::sendEvent(scene, &enter);
        VERIFY_WEB(scene->activeVehicleId() == car.id);
        const auto beforeDrive = scene->vehicles().first().position;
        scene->setPilotInput({0, 0, 1}, {}, false);
        QElapsedTimer drive;
        drive.start();
        QVector<qreal> driveTimes;
        auto previousFrame = scene->renderedFrames();
        while (drive.elapsed() < 1100) {
            QApplication::processEvents();
            if (scene->renderedFrames() != previousFrame) {
                driveTimes.append(scene->lastDrawNanoseconds() / 1e6);
                previousFrame = scene->renderedFrames();
            }
            QThread::msleep(2);
        }
        VERIFY_WEB((scene->vehicles().first().position - beforeDrive).length() > 100);
        VERIFY_WEB(!driveTimes.isEmpty());
        std::sort(driveTimes.begin(), driveTimes.end());
        std::printf("vehicle_moving_frames=%lld interval_ms=%lld draw_p95_ms=%.3f\n",
            static_cast<long long>(driveTimes.size()), static_cast<long long>(drive.elapsed()),
            driveTimes[qMin(driveTimes.size() - 1, driveTimes.size() * 95 / 100)]);
        scene->setPilotInput({}, {}, false);
        VERIFY_WEB(waitForWeb([&]() { return !scene->moving(); }, 4000));
        VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".driving.png"));
        const auto parked = scene->vehicles().first().position;
        VERIFY_WEB(scene->exitVehicle());
        VERIFY_WEB(scene->vehicles().first().position == parked && scene->interiorLocomotion() == "walk");
        VERIFY_WEB(scene->enterVehicle(car.id));
        cockpit.visit("files");
        VERIFY_WEB(!scene->moving());
        cockpit.visit("web");
        VERIFY_WEB(scene->activeVehicleId() == car.id && scene->vehicles().first().position == parked);
        VERIFY_WEB(scene->exitVehicle());
        std::puts("Native website vehicle extraction, visible model, entry, driving, parking and hide/resume passed.");
        return 0;
    }
    if (liveUrl.isEmpty()) {
        auto *layout = browser->findChild<QComboBox *>("spatialWebLayout");
        VERIFY_WEB(layout && layout->isEnabled());
        const auto contentUrl = browser->document().url;
        for (int index = 1; index <= 4; ++index) {
            layout->setCurrentIndex(index);
            VERIFY_WEB(browser->document().url == contentUrl && !browser->building());
            VERIFY_WEB(browser->scene()->interiorLocomotion() == (index == 4 ? "fly" : "walk"));
            VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".layout-" + QString::number(index) + ".png"));
        }
        layout->setCurrentIndex(0);
    }
    if (qEnvironmentVariableIsSet("DOCKPIPE_WEB_TEST_MEDIA")) {
        VERIFY_WEB(browser->document().venue == "cinema");
        const auto camera = browser->scene()->cameraPosition();
        browser->activateItem(browser->document().items.first().id);
        const bool played = waitForWeb([&]() { return browser->media()->frameCount() >= 5 && browser->media()->mediaTime() > 0.25; }, 25000);
        std::printf("Live media frames=%llu time=%.3f status=%s\n", static_cast<unsigned long long>(browser->media()->frameCount()),
            browser->media()->mediaTime(), qPrintable(browser->status()));
        VERIFY_WEB(browser->spatial() && browser->scene()->cameraPosition() == camera);
        captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".media.png");
        VERIFY_WEB(played);
        return 0;
    }
    const auto benchmark = [&]() {
        QVector<double> drawTimes;
        for (int frame = 0; frame < 50; ++frame) {
            browser->scene()->viewport()->repaint();
            QApplication::processEvents();
            if (frame >= 10)
                drawTimes.append(browser->scene()->lastDrawNanoseconds() / 1e6);
        }
        std::sort(drawTimes.begin(), drawTimes.end());
        std::printf("venue=%s solids=%lld draw_median_ms=%.3f draw_p95_ms=%.3f renderer=%s\n",
            qPrintable(browser->document().venue), static_cast<long long>(browser->document().room().solids.size()),
            drawTimes[20], drawTimes[38], qPrintable(browser->scene()->rendererDescription()));
        // Exercise the actual motion timer down the central aisle, with the page
        // frozen. Report draw cost separately from host scheduling/refresh rate.
        drawTimes.clear();
        auto previousFrame = browser->scene()->renderedFrames();
        browser->scene()->setPilotInput({0, 0, 1}, {}, false);
        QElapsedTimer motion;
        motion.start();
        while (motion.elapsed() < 1000) {
            QApplication::processEvents();
            const auto currentFrame = browser->scene()->renderedFrames();
            if (currentFrame != previousFrame) {
                drawTimes.append(browser->scene()->lastDrawNanoseconds() / 1e6);
                previousFrame = currentFrame;
            }
            QThread::msleep(2);
        }
        browser->scene()->setPilotInput({}, {}, false);
        std::sort(drawTimes.begin(), drawTimes.end());
        if (!drawTimes.isEmpty())
            std::printf("moving_frames=%lld moving_draw_p95_ms=%.3f\n",
                static_cast<long long>(drawTimes.size()), drawTimes[qMin(drawTimes.size() - 1, drawTimes.size() * 95 / 100)]);
    };
    benchmark();
    auto *livePage = browser->findChild<QWebEngineView *>("spatialWebLivePage");
    VERIFY_WEB(livePage && livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    VERIFY_WEB(!browser->scene()->moving());
    auto *browserTools = browser->findChild<BrowserWidget *>("spatialWebBrowserTools");
    VERIFY_WEB(browserTools && browserTools->activeView() == livePage);
    VERIFY_WEB(browserTools->findChild<QPushButton *>("browserDownloads"));
    VERIFY_WEB(browserTools->findChild<QPushButton *>("browserMore"));
    auto *tabs = browserTools->findChild<QTabBar *>("browserTabs");
    auto *newTab = browserTools->findChild<QPushButton *>("browserNewTab");
    VERIFY_WEB(tabs && newTab);
    const QString worldBeforeTabs = browser->document().title;
    const auto cameraBeforeTabs = browser->scene()->cameraPosition();
    const int readyBeforeTabs = ready;
    browser->showPage();
    VERIFY_WEB(browserTools->isVisible() && !address->isVisible());
    QPointer<SpatialWebMedia> previousMedia = browser->media();
    newTab->click();
    VERIFY_WEB(!previousMedia && browser->media());
    QPointer<QWebEngineView> secondTab = browserTools->activeView();
    VERIFY_WEB(secondTab && secondTab != livePage && tabs->count() == 2);
    VERIFY_WEB(livePage->page()->isAudioMuted());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    // Keep the delayed /second response reserved for the corridor test below.
    browserTools->navigate(base.resolved(QUrl("/browser-tab-fixture")));
    VERIFY_WEB(waitForWeb([&]() {
        return secondTab->url().path() == "/browser-tab-fixture" && !secondTab->page()->isLoading();
    }));
    browser->showSpace();
    VERIFY_WEB(browser->spatial() && browser->document().title == worldBeforeTabs);
    VERIFY_WEB(browser->scene()->cameraPosition() == cameraBeforeTabs && ready == readyBeforeTabs);
    VERIFY_WEB(secondTab->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    browser->showPage();
    VERIFY_WEB(browserTools->activeView() == secondTab && secondTab->url().path() == "/browser-tab-fixture");
    tabs->tabCloseRequested(1);
    VERIFY_WEB(!secondTab && tabs->count() == 1 && browserTools->activeView() == livePage);
    VERIFY_WEB(browser->media() && ready == readyBeforeTabs);
    browser->showSpace();
    VERIFY_WEB(address->isVisible() && browser->document().title == worldBeforeTabs);
    std::puts("Shared browser tools, tab disposal and one spatial media controller preserve the existing world.");
    auto *exit = browser->findChild<QPushButton *>("spatialWebExit");
    VERIFY_WEB(exit);
    exit->click();
    VERIFY_WEB(mainViews->currentWidget() == cockpit.flight() && !browser->isVisible());
    VERIFY_WEB(!cockpit.flight()->editorActive());
    const auto orbitCamera = cockpit.flight()->cameraPosition();
    cockpit.flight()->setPilotInput({0, 0, 1}, {}, false);
    VERIFY_WEB(waitForWeb([&]() {
        // Viewport creation can briefly transfer desktop focus. A real
        // controller supplies continuous input; don't model it as one event.
        cockpit.flight()->setPilotInput({0, 0, 1}, {}, false);
        return cockpit.flight()->cameraPosition() != orbitCamera;
    }));
    cockpit.flight()->setPilotInput({}, {}, false);
    cockpit.visit("web");
    VERIFY_WEB(mainViews->currentWidget() == browser && browser->spatial());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    const auto framesAtIdle = browser->scene()->renderedFrames();
    QElapsedTimer idle;
    idle.start();
    while (idle.elapsed() < 250) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_WEB(browser->scene()->renderedFrames() <= framesAtIdle + 1);
    if (liveUrl.isEmpty()) {
        VERIFY_WEB(browser->document().title == "JavaScript rendered gallery");
        VERIFY_WEB(browser->document().items.size() == 12 && browser->document().limited);
        VERIFY_WEB(!browser->document().items[0].poster.isNull());
        const auto poster = browser->document().items[0].poster;
        VERIFY_WEB(poster.pixelColor(250, 110).red() > 150);
    }
    // Exercise search interpretation against the controlled DOM. Shadow only
    // location, so no request is made to Google or another external service.
    QFile extraction(":/web/extract.js");
    VERIFY_WEB(extraction.open(QIODevice::ReadOnly));
    const QString extractionScript = QString::fromUtf8(extraction.readAll());
    bool searchInterpreted = false;
    SpatialWebDocument searchDocument;
    QString searchError;
    livePage->page()->setLifecycleState(QWebEnginePage::LifecycleState::Active);
    livePage->page()->runJavaScript("(() => { const location = new URL('https://www.google.com/search?q=fixtures'); return "
        + extractionScript + "; })()", [&](const QVariant &result) {
            SpatialWebDocument::parse(result.toString().toUtf8(), searchDocument, searchError);
            searchInterpreted = true;
        });
    VERIFY_WEB(waitForWeb([&]() { return searchInterpreted; }));
    VERIFY_WEB(searchError.isEmpty() && searchDocument.venue == "city" && !searchDocument.items.isEmpty());
    VERIFY_WEB(searchDocument.room().displays.first().portal);
    bool opaqueInterpreted = false;
    SpatialWebDocument opaqueDocument;
    QString opaqueError;
    const QString opaqueFixture = R"JS((() => {
        const original = window.document.body.innerHTML;
        window.document.body.innerHTML = `<main id="search"><div id="rso">
            <a href="https://www.google.com/goto?url=CAESopaque_fixture"><h3>Rainforest habitats</h3>
                <cite>https://kids.nationalgeographic.com<span> › nature › habitats</span></cite></a>
            <p>A source destination with meaningful rainforest information and enough visible content to test a search while unrelated resources are still loading.</p>
            <a href="https://www.google.com/search?q=next"><h3>Next page</h3></a>
            <a href="javascript:alert(1)"><h3>Unsafe result</h3></a>
            </div></main>`;
        const location = new URL('https://www.google.com/search?q=rainforest&sei=session');
        const document = new Proxy(window.document, {get(target, key) {
            if (key === 'readyState') return 'loading';
            const value = Reflect.get(target, key, target);
            return typeof value === 'function' ? value.bind(target) : value;
        }});
        const result = EXTRACT;
        window.document.body.innerHTML = original;
        return result;
    })())JS";
    livePage->page()->runJavaScript(QString(opaqueFixture).replace("EXTRACT", extractionScript),
        [&](const QVariant &result) {
            SpatialWebDocument::parse(result.toString().toUtf8(), opaqueDocument, opaqueError);
            opaqueInterpreted = true;
        });
    VERIFY_WEB(waitForWeb([&]() { return opaqueInterpreted; }));
    VERIFY_WEB(opaqueError.isEmpty() && opaqueDocument.venue == "city" && opaqueDocument.items.size() == 1);
    VERIFY_WEB(opaqueDocument.items.first().url.path() == "/goto");
    VERIFY_WEB(opaqueDocument.items.first().detail == "kids.nationalgeographic.com");
    livePage->page()->setLifecycleState(QWebEnginePage::LifecycleState::Frozen);
    const QString sourceTitle = browser->document().title;
    const QUrl sourceUrl = browser->document().url;
    // Turn toward a stall and approach it using normal controls. Its threshold
    // is in front of the counter, separate from the image on the back wall.
    const auto portal = browser->document().room().displays[0];
    const float radians = portal.yaw * 3.14159265f / 180;
    const QVector3D outward(std::sin(radians), 0, -std::cos(radians));
    const auto approach = portal.portalCenter + outward * 140;
    browser->scene()->setSurfaceSpawn({approach.x(), approach.z()});
    const int turnPixels = qRound(std::atan2(-outward.x(), -outward.z()) / 0.004);
    QMouseEvent press(QEvent::MouseButtonPress, QPointF(600, 400), QPointF(600, 400),
        Qt::RightButton, Qt::RightButton, Qt::NoModifier);
    QApplication::sendEvent(browser->scene()->viewport(), &press);
    QMouseEvent turn(QEvent::MouseMove, QPointF(600 + turnPixels, 400), QPointF(600 + turnPixels, 400),
        Qt::NoButton, Qt::RightButton, Qt::NoModifier);
    QApplication::sendEvent(browser->scene()->viewport(), &turn);
    QMouseEvent release(QEvent::MouseButtonRelease, QPointF(600 + turnPixels, 400), QPointF(600 + turnPixels, 400),
        Qt::RightButton, Qt::NoButton, Qt::NoModifier);
    QApplication::sendEvent(browser->scene()->viewport(), &release);
    VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".stall.png"));
    browser->scene()->setPilotInput({0, 0, 1}, {}, true);
    VERIFY_WEB(waitForWeb([&]() {
        browser->scene()->setPilotInput({0, 0, 1}, {}, true);
        return browser->transitioning();
    }, 30000));
    VERIFY_WEB(browser->document().title == sourceTitle && browser->spatial());
    browser->activateItem(portal.id);
    VERIFY_WEB(browser->transitioning());
    if (liveUrl.isEmpty()) {
        VERIFY_WEB(waitForWeb([&]() {
            browser->scene()->setPilotInput({0, 0, 1}, {}, true);
            return QVector3D::dotProduct(browser->scene()->cameraPosition() - portal.portalCenter, -outward) > 1050;
        }, 4500));
        const auto waitingPosition = browser->scene()->cameraPosition();
        VERIFY_WEB(browser->document().title == sourceTitle);
        QElapsedTimer waitingAtDoor;
        waitingAtDoor.start();
        while (waitingAtDoor.elapsed() < 200) {
            QApplication::processEvents();
            QThread::msleep(5);
        }
        VERIFY_WEB((browser->scene()->cameraPosition() - waitingPosition).length() < 25);
        if (!screenshot.isEmpty()) {
            auto *screen = browser->scene()->screen();
            VERIFY_WEB(screen->grabWindow(browser->scene()->winId()).save(screenshot + ".passage.png"));
        }
    }
    VERIFY_WEB(waitForWeb([&]() {
        if (browser->transitioning())
            browser->scene()->setPilotInput({0, 0, 1}, {}, true);
        return ready > 1 && !browser->transitioning();
    }));
    VERIFY_WEB(!browser->scene()->editorActive());
    if (!liveUrl.isEmpty()) {
        VERIFY_WEB(browser->document().url != sourceUrl);
        VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".destination.png"));
        std::printf("Live approach transition passed: %s -> %s\n", qPrintable(sourceUrl.toString()),
            qPrintable(browser->document().url.toString()));
        return 0;
    }
    VERIFY_WEB(browser->document().title == "Second destination");
    browser->scene()->setPilotInput({}, {}, false);
    auto *back = browser->findChild<QPushButton *>("spatialWebBack");
    VERIFY_WEB(back && back->isEnabled());
    back->click();
    VERIFY_WEB(waitForWeb([&]() { return ready > 2; }));
    VERIFY_WEB(browser->document().title == "JavaScript rendered gallery");
    auto *forward = browser->findChild<QPushButton *>("spatialWebForward");
    VERIFY_WEB(forward && forward->isEnabled());
    forward->click();
    VERIFY_WEB(waitForWeb([&]() { return ready > 3; }));
    VERIFY_WEB(browser->document().title == "Second destination");
    for (const QString &venue : {"invertebrates", "habitats", "rainforest", "store", "cinema"}) {
        const int before = ready;
        QElapsedTimer loaded;
        loaded.start();
        browser->navigate(base.resolved(QUrl("/" + venue)));
        const bool venueReady = waitForWeb([&]() { return ready > before && !browser->transitioning(); });
        if (!venueReady)
            std::fprintf(stderr, "Venue %s failed: %s\n", qPrintable(venue), qPrintable(browser->status()));
        VERIFY_WEB(venueReady);
        if (venue == "invertebrates") {
            VERIFY_WEB(browser->document().items.size() == 5);
            VERIFY_WEB(std::all_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "animal" && item.url.path() == "/animal-facts" && !item.visual.isNull(); }));
            VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".animals.png"));
            const int collectionReady = ready;
            const QUrl animalUrl = browser->document().items.first().url;
            browser->activateItem(browser->document().items.first().id);
            auto *articlePage = browser->findChild<QWebEngineView *>("spatialWebLivePage");
            VERIFY_WEB(waitForWeb([&]() { return articlePage->url() == animalUrl; }));
            // Legacy completion signals cannot cancel or publish the new page;
            // its destination still has a parser-blocking script in flight.
            VERIFY_WEB(QMetaObject::invokeMethod(articlePage, "loadFinished", Q_ARG(bool, true)));
            VERIFY_WEB(QMetaObject::invokeMethod(articlePage, "loadFinished", Q_ARG(bool, false)));
            waitForWeb([]() { return false; }, 600);
            VERIFY_WEB(ready == collectionReady);
            VERIFY_WEB(waitForWeb([&]() { return ready > collectionReady && !browser->transitioning(); }));
            VERIFY_WEB(browser->document().url == animalUrl);
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && item.description.contains("controlled fixture"); }));
            const int articleReady = ready;
            waitForWeb([]() { return false; }, 300);
            VERIFY_WEB(ready == articleReady && browser->document().url == animalUrl);
        } else if (venue == "habitats") {
            const auto displays = browser->scene()->interiorDisplays();
            VERIFY_WEB(displays.size() == 1 && displays.first().portal);
            const auto entrance = displays.first();
            const int habitatReady = ready;
            loaded.restart();
            browser->activateItem(entrance.id);
            VERIFY_WEB(browser->transitioning());
            VERIFY_WEB(waitForWeb([&]() {
                const auto staged = browser->scene()->interiorDisplays();
                return std::any_of(staged.cbegin(), staged.cend(), [](const auto &display) {
                    return display.id.startsWith("arrival:");
                });
            }, 5500));
            // Cross the already-open two-segment passage while a script after
            // the article still blocks HTML parsing for six seconds.
            const float angle = entrance.yaw * 3.14159265358979323846f / 180;
            const QVector3D outward(std::sin(angle), 0, -std::cos(angle));
            const auto arrival = entrance.portalCenter - outward * (2 * SpatialWebPassage::segmentLength + 170);
            browser->scene()->setSurfaceSpawn({arrival.x(), arrival.z()});
            VERIFY_WEB(waitForWeb([&]() { return ready > habitatReady && !browser->transitioning(); }));
            VERIFY_WEB(loaded.elapsed() < 5500);
            VERIFY_WEB(browser->document().url.path() == "/rainforest");
            VERIFY_WEB(std::none_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.url.path() == "/unrelated"; }));
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && item.description.contains("canopy"); }));
            std::puts("Habitat corridor opens from readable article content before a stalled script completes.");
        } else if (venue == "rainforest") {
            VERIFY_WEB(loaded.elapsed() < 5500);
            VERIFY_WEB(browser->document().venue == "park");
            VERIFY_WEB(browser->document().room().naturalGround);
            VERIFY_WEB(std::none_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && item.description.contains("rights notice"); }));
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && item.description.contains("Copyright law"); }));
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "excerpt" && item.description.contains("canopy"); }));
            VERIFY_WEB(std::any_of(browser->document().items.cbegin(), browser->document().items.cend(),
                [](const auto &item) { return item.kind == "image" && !item.visual.isNull(); }));
        } else {
            VERIFY_WEB(browser->document().venue == venue);
            VERIFY_WEB(browser->document().room().solids.size() > 20);
        }
        benchmark();
        VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + "." + venue + ".png"));
        if (venue == "store")
            VERIFY_WEB(browser->document().items[0].detail == "$25.00");
    }
    const auto theatreCamera = browser->scene()->cameraPosition();
    QSet<QRgb> videoColors;
    QObject::connect(browser->media(), &SpatialWebMedia::frameReady, browser, [&](const QImage &frame) {
        videoColors.insert(frame.pixel(frame.width() / 2, frame.height() / 2));
    });
    browser->activateItem(browser->document().items.first().id);
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->frameCount() >= 4 && browser->media()->mediaTime() > 0.15; }));
    if (!browser->spatial() || browser->scene()->cameraPosition() != theatreCamera) {
        const auto actual = browser->scene()->cameraPosition();
        std::fprintf(stderr, "Theatre pose: spatial=%d moving=%d phase=%d expected=(%.3f,%.3f,%.3f) actual=(%.3f,%.3f,%.3f)\n",
            browser->spatial(), browser->scene()->moving(), browser->transitioning(),
            theatreCamera.x(), theatreCamera.y(), theatreCamera.z(), actual.x(), actual.y(), actual.z());
    }
    VERIFY_WEB(browser->spatial() && browser->scene()->cameraPosition() == theatreCamera);
    VERIFY_WEB(videoColors.size() > 1);
    // Fullscreen must retain the hidden-behind-the-scene Chromium compositor:
    // advancing media time alone does not prove that video pixels are updating.
    browser->scene()->setFlightFullscreen(true);
    VERIFY_WEB(livePage->isVisible());
    videoColors.clear();
    const double fullscreenStartTime = browser->media()->mediaTime();
    VERIFY_WEB(waitForWeb([&]() {
        return videoColors.size() >= 3 && browser->media()->mediaTime() > fullscreenStartTime + 0.3;
    }));
    VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".fullscreen-playing.png"));
    auto *pause = browser->findChild<QPushButton *>("spatialWebPause");
    VERIFY_WEB(pause);
    pause->click();
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->paused(); }));
    const auto pausedFrames = browser->media()->frameCount();
    QElapsedTimer pauseTime;
    pauseTime.start();
    while (pauseTime.elapsed() < 250) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_WEB(browser->media()->frameCount() == pausedFrames);
    VERIFY_WEB(!livePage->isVisible());
    browser->scene()->setFlightFullscreen(false);
    VERIFY_WEB(!livePage->isVisible());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    browser->scene()->setFlightFullscreen(true);
    pause->click();
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->frameCount() > pausedFrames + 2; }));
    VERIFY_WEB(livePage->isVisible());
    videoColors.clear();
    VERIFY_WEB(waitForWeb([&]() { return videoColors.size() >= 3; }));
    browser->scene()->setFlightFullscreen(false);
    videoColors.clear();
    VERIFY_WEB(waitForWeb([&]() { return videoColors.size() >= 3; }));
    VERIFY_WEB(browser->scene()->cameraPosition() == theatreCamera);
    std::puts("Fullscreen theatre retains changing video pixels; pause, resume and window restoration preserve media lifecycle.");
    VERIFY_WEB(captureWorld(screenshot.isEmpty() ? QString() : screenshot + ".playing.png"));

    // Website controls and the existing theatre share one page, without a new
    // extraction, navigation or world publication on either toggle.
    auto *pageToggle = browser->findChild<QPushButton *>("spatialWebPage");
    VERIFY_WEB(pageToggle);
    const int readyBeforeToggle = ready;
    const QUrl mediaUrl = livePage->url();
    const double timeBeforeToggle = browser->media()->mediaTime();
    pageToggle->click();
    VERIFY_WEB(!browser->spatial() && pageToggle->text() == "Return to space");
    VERIFY_WEB(!browser->media()->active() && !livePage->page()->isAudioMuted());
    auto pageState = QSharedPointer<QVariant>::create();
    livePage->page()->runJavaScript("window.spatialToggleMarker = 42; ({paused: document.querySelector('video').paused})",
        [pageState](const QVariant &value) { *pageState = value; });
    VERIFY_WEB(waitForWeb([&]() { return pageState->isValid(); }));
    VERIFY_WEB(!pageState->toMap().value("paused").toBool());
    pageToggle->click();
    VERIFY_WEB(browser->spatial() && browser->scene()->cameraPosition() == theatreCamera);
    VERIFY_WEB(pageToggle->text() == "Live page" && !browser->building());
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->frameCount() >= 3; }));
    VERIFY_WEB(browser->media()->mediaTime() >= timeBeforeToggle && livePage->url() == mediaUrl);
    VERIFY_WEB(ready == readyBeforeToggle);
    pageToggle->click();
    auto pausedByWebsite = QSharedPointer<QVariant>::create();
    livePage->page()->runJavaScript("document.querySelector('video').pause(); window.spatialToggleMarker",
        [pausedByWebsite](const QVariant &value) { *pausedByWebsite = value; });
    VERIFY_WEB(waitForWeb([&]() { return pausedByWebsite->isValid(); }));
    VERIFY_WEB(pausedByWebsite->toInt() == 42);
    pageToggle->click();
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->paused() && browser->media()->frameCount() > 0; }));
    VERIFY_WEB(ready == readyBeforeToggle && browser->scene()->cameraPosition() == theatreCamera);
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    pause->click();
    VERIFY_WEB(waitForWeb([&]() { return !browser->media()->paused(); }));
    std::puts("Live page / existing theatre toggle preserves DOM, camera, playback time and website pause state without rebuilding.");

    // Leaving a playing theatre must return control, not just swap visible pages.
    // Repeat after reentry to catch stale media callbacks and suspended input.
    for (int transition = 0; transition < 2; ++transition) {
        if (transition == 1)
            browser->scene()->setFlightFullscreen(true);
        const auto framesBeforeExit = browser->media()->frameCount();
        exit->click();
        VERIFY_WEB(mainViews->currentWidget() == cockpit.flight() && !browser->isVisible());
        VERIFY_WEB(!browser->scene()->flightFullscreen());
        VERIFY_WEB(!cockpit.flight()->editorActive() && cockpit.flight()->orbit());
        VERIFY_WEB(!browser->media()->active());
        VERIFY_WEB(livePage->page()->isAudioMuted());
        VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
        const auto position = cockpit.flight()->cameraPosition();
        QKeyEvent forwardKey(QEvent::KeyPress, Qt::Key_W, Qt::NoModifier);
        QApplication::sendEvent(cockpit.flight(), &forwardKey);
        VERIFY_WEB(waitForWeb([&]() { return cockpit.flight()->cameraPosition() != position; }));
        QKeyEvent releaseKey(QEvent::KeyRelease, Qt::Key_W, Qt::NoModifier);
        QApplication::sendEvent(cockpit.flight(), &releaseKey);
        VERIFY_WEB(browser->media()->frameCount() == framesBeforeExit);
        const auto orbitFrames = cockpit.flight()->renderedFrames();
        VERIFY_WEB(waitForWeb([&]() { return cockpit.flight()->renderedFrames() >= orbitFrames + 3; }));
        VERIFY_WEB(browser->media()->frameCount() == framesBeforeExit);
        cockpit.visit("web");
        VERIFY_WEB(browser->spatial() && browser->scene()->cameraPosition() == theatreCamera);
        VERIFY_WEB(!cockpit.flight()->moving());
        VERIFY_WEB(!browser->media()->active());
        VERIFY_WEB(!livePage->isVisible() && livePage->page()->isAudioMuted());
        VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
        browser->activateItem(browser->document().items.first().id);
        VERIFY_WEB(waitForWeb([&]() { return browser->media()->frameCount() >= 4; }));
    }
    std::puts("Playing theatre -> orbit -> flight input -> theatre transitions passed twice.");
    browser->findChild<QPushButton *>("spatialWebStopFilm")->click();
    VERIFY_WEB(!browser->media()->active());
    const QString preservedTitle = browser->document().title;
    const int beforeEmpty = ready;
    browser->navigate(base.resolved(QUrl("/empty")));
    VERIFY_WEB(waitForWeb([&]() { return ready > beforeEmpty; }));
    VERIFY_WEB(browser->spatial() && browser->document().title == preservedTitle);
    VERIFY_WEB(browser->status().contains("No spatial destinations"));
    VERIFY_WEB(!browser->transitioning() && !browser->scene()->editorActive());
    const auto retainedLink = std::find_if(browser->document().items.cbegin(), browser->document().items.cend(),
        [](const SpatialWebItem &item) { return !item.url.isEmpty(); });
    VERIFY_WEB(retainedLink != browser->document().items.cend());
    browser->activateItem(retainedLink->id);
    VERIFY_WEB(waitForWeb([&]() { return browser->media()->frameCount() > 2; }));
    VERIFY_WEB(browser->spatial() && browser->document().title == preservedTitle);
    browser->findChild<QPushButton *>("spatialWebStopFilm")->click();
    browser->navigate(base.resolved(QUrl("/cinema")));
    VERIFY_WEB(waitForWeb([&]() { return ready > beforeEmpty + 1 && !browser->transitioning(); }));
    browser->scene()->setFlightFullscreen(true);
    browser->showPage();
    VERIFY_WEB(!browser->scene()->flightFullscreen() && !browser->spatial());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Active);
    exit->click();
    VERIFY_WEB(!cockpit.flight()->editorActive());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    cockpit.visit("web");
    VERIFY_WEB(!browser->spatial() && livePage->isVisible());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Active);
    const auto retainedCamera = browser->scene()->cameraPosition();
    const QString retainedTitle = browser->document().title;
    const int beforeLiveNavigation = ready;
    livePage->load(base.resolved(QUrl("/second")));
    VERIFY_WEB(waitForWeb([&]() { return livePage->url().path() == "/second" && browser->findChild<QPushButton *>("spatialWebBuild")->isEnabled(); }));
    pageToggle->click();
    VERIFY_WEB(browser->spatial() && !browser->building());
    VERIFY_WEB(browser->scene()->cameraPosition() == retainedCamera && browser->document().title == retainedTitle);
    QElapsedTimer unchangedWorld;
    unchangedWorld.start();
    while (unchangedWorld.elapsed() < 700) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_WEB(ready == beforeLiveNavigation && !browser->building());
    const int beforeRebuild = ready;
    browser->buildSpace();
    VERIFY_WEB(waitForWeb([&]() { return ready > beforeRebuild; }));
    VERIFY_WEB(browser->spatial());
    const int beforePassageFailure = ready;
    browser->navigate(base);
    VERIFY_WEB(waitForWeb([&]() { return ready > beforePassageFailure; }));
    delayedResponses = 1;
    responseDelay = 600;
    auto *passageDeadline = browser->findChild<QTimer *>("spatialPassageDeadline");
    VERIFY_WEB(passageDeadline);
    passageDeadline->setInterval(100);
    const auto failureEntrance = browser->scene()->cameraPosition();
    const QString failureSource = browser->document().title;
    browser->activateItem(browser->document().items.first().id);
    VERIFY_WEB(waitForWeb([&]() { return browser->status().contains("Unable to prepare"); }, 2000));
    QElapsedTimer lateResponse;
    lateResponse.start();
    while (lateResponse.elapsed() < 1300) {
        QApplication::processEvents();
        QThread::msleep(5);
    }
    VERIFY_WEB(browser->transitioning() && browser->document().title == failureSource);
    back->click();
    VERIFY_WEB(!browser->transitioning() && browser->scene()->cameraPosition() == failureEntrance);
    passageDeadline->setInterval(30000);
    std::puts("Bounded slow-load corridor, solid waiting door, timeout, late response and return-to-entrance passed.");
    browser->scene()->setFlightFullscreen(true);
    browser->navigate(QUrl("file:///etc/passwd"));
    VERIFY_WEB(browser->status().contains("HTTP"));
    browser->navigate(base);
    VERIFY_WEB(waitForWeb([&]() { return browser->transitioning(); }));
    cockpit.hide();
    VERIFY_WEB(!browser->building() && !browser->scene()->moving());
    VERIFY_WEB(!browser->transitioning() && !browser->scene()->editorActive());
    VERIFY_WEB(!browser->scene()->flightFullscreen());
    VERIFY_WEB(livePage->page()->lifecycleState() == QWebEnginePage::LifecycleState::Frozen);
    cockpit.show();
    auto preferences = cockpit.settings();
    preferences.enabled = false;
    cockpit.applySettings(preferences);
    QCoreApplication::sendPostedEvents(nullptr, QEvent::DeferredDelete);
    VERIFY_WEB(!browser);
    std::puts("Real browser JS, semantic worlds, walk-through navigation, history, idle freeze, bounds, cancellation and teardown passed.");
    return 0;
#endif
}
