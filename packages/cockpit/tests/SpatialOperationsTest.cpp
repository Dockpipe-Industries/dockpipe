#include "CockpitWidget.h"
#include "SpatialOperationsWorld.h"
#include <QApplication>
#include <QComboBox>
#include <QElapsedTimer>
#include <QJsonArray>
#include <QLabel>
#include <QLineEdit>
#include <QKeyEvent>
#include <QPushButton>
#include <QScreen>
#include <QThread>
#include <cstdio>

#define VERIFY(value) do { if (!(value)) { std::fprintf(stderr, "Operations assertion failed at %d: %s\n", __LINE__, #value); return 1; } } while (false)

namespace {
void idle(int milliseconds)
{
    QElapsedTimer clock;
    clock.start();
    while (clock.elapsed() < milliseconds) {
        QApplication::processEvents();
        QThread::msleep(2);
    }
}
QJsonObject node(const QString &id, const QString &kind, bool active = false)
{
    return {{"id", id}, {"kind", kind}, {"title", id}, {"state", active ? "Running" : "Stopped"},
        {"active", active}, {"status", active ? 1 : 2}};
}
QJsonObject edge(const QString &from, const QString &to, bool dependency = false)
{
    return {{"from", from}, {"to", to}, {"dependency", dependency}, {"label", from + " → " + to}};
}
}

int spatialOperationsTest()
{
    QJsonObject snapshot{{"nodes", QJsonArray{node("network:public", "network"), node("network:private", "network"),
        node("container:web", "container", true), node("container:db", "container"), node("volume:data", "volume")}},
        {"edges", QJsonArray{edge("network:public", "container:web"), edge("network:private", "container:web"),
            edge("network:private", "container:db"), edge("container:db", "volume:data"),
            edge("container:web", "container:db", true), edge("missing", "container:web")}}};
    auto scene = SpatialOperationsWorld::campus("containers", snapshot, 0);
    VERIFY(scene.interior.connections.size() == 4);
    int active = 0;
    for (const auto &connection : scene.interior.connections) {
        active += connection.active ? 1 : 0;
        VERIFY(connection.sourceId != "missing");
        VERIFY(!(connection.sourceId == "container:web" && connection.targetId == "container:db"));
        if (connection.targetId == "volume:data")
            VERIFY(connection.color == QColor("#e7b46f") && !connection.active);
    }
    VERIFY(active == 2);
    VERIFY(scene.notice.contains("not measured traffic"));
    VERIFY(scene.interior.displays.size() == 6);
    VERIFY(SpatialOperationsWorld::campus("containers", {}, 900).notice.contains("No resources"));

    QJsonArray manyNodes;
    QJsonArray manyEdges;
    for (int index = 0; index < 1200; ++index) {
        manyNodes.append(node(QString("container:%1").arg(index, 4, 10, QChar('0')), "container", true));
        manyNodes.append(node(QString("network:%1").arg(index, 4, 10, QChar('0')), "network"));
        manyEdges.append(edge(QString("network:%1").arg(index, 4, 10, QChar('0')), QString("container:%1").arg(index, 4, 10, QChar('0'))));
    }
    const QJsonObject large{{"nodes", manyNodes}, {"edges", manyEdges}};
    const auto last = SpatialOperationsWorld::campus("containers", large, 9999);
    VERIFY(last.page == last.pages - 1);
    for (int page = 0; page < last.pages; ++page) {
        const auto sector = SpatialOperationsWorld::campus("containers", large, page);
        VERIFY(sector.interior.solids.size() <= SpatialInterior::maximumSolids);
        VERIFY(sector.interior.displays.size() <= SpatialWorkspace::maximumDisplays);
        VERIFY(sector.interior.connections.size() <= SpatialInterior::maximumConnections);
        VERIFY(sector.signs.size() <= 12);
        QSet<QString> visible;
        for (const auto &display : sector.interior.displays) {
            if (display.id.startsWith("resource:"))
                visible.insert(display.id.mid(9));
        }
        for (const auto &connection : sector.interior.connections)
            VERIFY(visible.contains(connection.sourceId) && visible.contains(connection.targetId));
    }
    const QJsonObject execution{{"nodes", QJsonArray{node("run:1", "workflow", true), node("task:1", "task", true), node("pid:1", "process", true)}},
        {"edges", QJsonArray{edge("run:1", "task:1"), edge("task:1", "pid:1")}}};
    VERIFY(SpatialOperationsWorld::campus("executions", execution, 0).interior.connections.size() == 2);

    CockpitWidget cockpit;
    QWidget controls;
    cockpit.resize(1200, 800);
    if (QGuiApplication::platformName() != "offscreen")
        cockpit.setWindowFlag(Qt::WindowStaysOnTopHint);
    auto preferences = cockpit.settings();
    preferences.enabled = true;
    preferences.appControl = false;
    preferences.controller = false;
    cockpit.applySettings(preferences);
    int reads = 0;
    QString selected;
    const QJsonObject machine{{"id", "machine:test"}, {"kind", "machine"}, {"title", "Test machine"}, {"state", "Paired"}};
    cockpit.resourceSnapshot = [&](const QString &section) {
        ++reads;
        if (section == "machines")
            return QJsonObject{{"nodes", QJsonArray{machine, node("broker", "broker")}}};
        if (section == "executions")
            return execution;
        return snapshot;
    };
    cockpit.pageProvider = [&](const QString &) { return &controls; };
    cockpit.resourceSelected = [&](const QString &, const QString &id) { selected = id; };
    cockpit.show();
    cockpit.visit("containers");
    idle(150);
    VERIFY(cockpit.resourceSection() == "containers" && cockpit.flight()->walking());
    VERIFY(cockpit.flight()->orbitAnimating());
    const auto position = cockpit.flight()->cameraPosition();
    cockpit.flight()->destinationActivated("display:resource:container:web", false);
    VERIFY(selected == "container:web" && cockpit.flight()->editorActive());
    VERIFY(!cockpit.flight()->orbitAnimating());
    cockpit.closePanel();
    VERIFY(cockpit.flight()->cameraPosition() == position && !cockpit.flight()->editorActive());
    VERIFY(cockpit.flight()->orbitAnimating());
    const QString capture = qEnvironmentVariable("DOCKPIPE_OPERATIONS_CAPTURE");
    const auto captureScene = [&](const QString &suffix) {
        idle(150);
        // QGraphicsView paints outside QOpenGLWidget::paintGL; framebuffer grab
        // repaints an empty paintGL. Capture the isolated, topmost fixture window.
        const QImage image = (QGuiApplication::platformName() == "offscreen"
            ? cockpit.grab() : cockpit.screen()->grabWindow(cockpit.winId())).toImage();
        QSet<QRgb> colors;
        for (int y = 100; y < image.height() - 200; y += 15) {
            for (int x = 100; x < image.width() - 100; x += 15)
                colors.insert(image.pixel(x, y));
        }
        const bool saved = capture.isEmpty() || image.save(capture + suffix);
        std::fprintf(stdout, "%s: %lld sampled colors, last frame %.2f ms\n", qPrintable(suffix),
            static_cast<long long>(colors.size()), cockpit.flight()->lastDrawNanoseconds() / 1e6);
        // Indoor flat-shaded rooms intentionally have a small palette.
        return colors.size() > 8 && saved;
    };
    VERIFY(captureScene("-factories.png"));
    snapshot.insert("notice", "Updated snapshot fixture");
    cockpit.flight()->setPilotInput({0, 0, 1}, {}, false);
    idle(1100);
    VERIFY(cockpit.flight()->moving() && cockpit.flight()->cameraPosition() != position);
    cockpit.flight()->setPilotInput({}, {}, false);
    const auto stoppedPosition = cockpit.flight()->cameraPosition();
    idle(1100);
    VERIFY(cockpit.flight()->cameraPosition() == stoppedPosition);
    VERIFY(cockpit.flight()->worldStatus().contains("Updated snapshot fixture"));
    cockpit.hide();
    const int hiddenReads = reads;
    idle(1100);
    VERIFY(reads == hiddenReads && !cockpit.flight()->orbitAnimating());
    cockpit.show();
    cockpit.visit("executions");
    VERIFY(cockpit.resourceSection() == "executions" && cockpit.flight()->walking());
    cockpit.visit("machines");
    VERIFY(cockpit.flight()->orbit() && cockpit.flight()->buildingCount() == 2);
    cockpit.flight()->destinationActivated("machine-planet:machine:test", false);
    VERIFY(cockpit.flight()->walking());
    VERIFY(captureScene("-lab.png"));
    cockpit.flight()->setSurfaceSpawn({0, -390});
    cockpit.activateWindow();
    cockpit.flight()->setFocus();
    idle(30);
    QKeyEvent walk(QEvent::KeyPress, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &walk);
    for (int attempt = 0; attempt < 80 && !cockpit.flight()->worldStatus().contains("Pairing/access status"); ++attempt)
        idle(25);
    QKeyEvent releaseWalk(QEvent::KeyRelease, Qt::Key_W, Qt::NoModifier);
    QApplication::sendEvent(cockpit.flight(), &releaseWalk);
    VERIFY(cockpit.flight()->worldStatus().contains("Pairing/access status"));
    cockpit.flight()->destinationActivated("display:machine:customize", false);
    VERIFY(cockpit.flight()->editorActive());
    cockpit.findChild<QLineEdit *>("androidName")->setText("Atlas");
    cockpit.findChild<QComboBox *>("androidColor")->setCurrentIndex(1);
    cockpit.findChild<QComboBox *>("androidBody")->setCurrentIndex(1);
    cockpit.findChild<QPushButton *>("saveAndroid")->click();
    VERIFY(!cockpit.flight()->editorActive());
    idle(50);
    VERIFY(captureScene("-android.png"));
    cockpit.flight()->destinationActivated("display:machine:customize", false);
    VERIFY(cockpit.findChild<QLineEdit *>("androidName")->text() == "Atlas");
    cockpit.closePanel();
    cockpit.goBack();
    VERIFY(cockpit.flight()->walking());
    cockpit.goBack();
    VERIFY(cockpit.flight()->orbit() && cockpit.resourceSection() == "machines");
    cockpit.goBack();
    VERIFY(cockpit.flight()->orbit() && cockpit.resourceSection().isEmpty());
    const int stoppedReads = reads;
    idle(1100);
    VERIFY(stoppedReads == reads);
    cockpit.visit("settings");
    VERIFY(cockpit.destination() == "orbit");
    return 0;
}
