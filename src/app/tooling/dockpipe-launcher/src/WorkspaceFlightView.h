#pragma once
#include "WorkspaceGraphView.h"
#include "WorkspaceGraphReader.h"
#include <QElapsedTimer>
#include <QSet>
#include <QPointer>
#include <QTimer>
#include "DesktopAppWindow.h"
#include "SpatialWorkspace.h"
#include <optional>

class FlightAppBillboards;
class QPushButton;

struct WorkspaceFlightBuilding {
    QString name;
    qreal height = 80;
    bool directory = false;
    bool unavailable = false;
    QString path;
    QString theme;
    QPointF position;
    bool positioned = false;
};

// One read-only, paged directory world. The helper owns filesystem reads;
// movement and proximity doors repaint only while controls are held.
class WorkspaceFlightView final : public WorkspaceGraphView {
public:
    explicit WorkspaceFlightView(const QVector<WorkspaceFlightBuilding> &buildings,
                                 bool hardware, QWidget *parent = nullptr);
    ~WorkspaceFlightView() override;
    void setAppBillboards(const QVector<DesktopAppChoice> &choices);
    void previewAppWindows(const QVector<DesktopAppChoice> &choices);
    void stopAppBillboards();
    bool appControlling() const;
    bool hasAppPreview(const QString &path) const;
    void setFlightFullscreen(bool enabled);
    bool flightFullscreen() const { return m_fullscreen; }
    void setFlightHardwareAcceleration(bool enabled);
    std::function<void()> exitRequested;
    std::function<void(const QString &)> fileTargeted;
    void fire();
    void setCockpitWorld(const QVector<WorkspaceFlightBuilding> &buildings, bool orbit);
    void approach(const QString &path);
    void skipTravel();
    void setShipVisible(bool visible) { m_shipVisible = visible; viewport()->update(); }
    void setInterior(const SpatialInterior &room, const QString &name);
    bool connectInteriorDisplay(const QString &id, const DesktopAppChoice &choice, bool control);
    bool hasInteriorPreview(const QString &id) const;
    int interiorStreamCount() const { return m_interiorStreams.size(); }
    quint64 captureCount() const;
    qint64 lastCaptureNanoseconds() const;
    bool walking() const { return m_interior.has_value(); }
    void setSurfaceSpawn(const QPointF &spawn);
    void setPilotInput(const QVector3D &movement, const QPointF &look, bool boost);
    void controlApp(const DesktopAppChoice &choice, bool fullscreen);
    void previewApp(const QString &path, const DesktopAppChoice &choice);
    void previewSurface(const QString &path, const QString &title, const QImage &image);
    void previewDocument(const QString &path, const QString &file);
    void previewCard(const QString &path, const QString &title, const QString &text);
    QString previewPath() const { return m_hologramPath; }
    bool previewLoading() const { return m_previewReader.busy(); }
    QImage previewImage() const { return m_hologramImage; }
    std::function<void(const QString &, bool)> destinationActivated;
    std::function<void()> orbitRequested;
    bool orbit() const { return m_orbit; }
    void setOrbitMotion(bool enabled);
    bool orbitMotion() const { return m_orbitMotion; }
    bool orbitAnimating() const { return m_orbitTick.isActive(); }
    bool travelling() const { return m_travel.isActive(); }
    qint64 lastDrawNanoseconds() const { return m_lastDrawNanoseconds; }
    quint64 renderedFrames() const { return m_renderedFrames; }
    void setEditorActive(bool active);
    bool editorActive() const { return m_editorActive; }
    QVector3D cameraPosition() const { return m_camera; }
    bool moving() const { return m_tick.isActive(); }
    void setWorkspace(const QString &root, const QString &directory, bool hidden);
    void enterDirectory(const QString &directory, bool nextPage = false);
    void goUp();
    void goBack();
    bool hasReturnPortal() const;
    QVector3D returnPortalPosition() const;
    void nextPage();
    QString directory() const { return m_directory; }
    QString worldStatus() const { return m_worldStatus; }
    void setFlightStatus(const QString &status);
    bool loading() const { return m_reader.busy(); }
    bool hasNextPage() const { return m_more; }
    int buildingCount() const { return m_buildings.size(); }
protected:
    void drawBackground(QPainter *painter, const QRectF &) override;
    void keyPressEvent(QKeyEvent *event) override;
    void keyReleaseEvent(QKeyEvent *event) override;
    void mousePressEvent(QMouseEvent *event) override;
    void mouseDoubleClickEvent(QMouseEvent *event) override;
    void mouseMoveEvent(QMouseEvent *event) override;
    void mouseReleaseEvent(QMouseEvent *event) override;
    void wheelEvent(QWheelEvent *event) override { event->accept(); }
    void focusOutEvent(QFocusEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void showEvent(QShowEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;
private:
    struct MeshFace {
        QVector<QVector3D> vertices;
        QVector3D center;
        QVector3D normal;
        QColor color;
        QColor edge;
        QString label;
    };
    struct Building {
        WorkspaceFlightBuilding source;
        QVector3D position;
        QVector<MeshFace> detailedMesh;
        QVector<MeshFace> distantMesh;
    };
    QVector<MeshFace> createBuildingMesh(const Building &building, bool detailed) const;
    bool sphereVisible(const QVector3D &center, qreal radius) const;
    void advance();
    void updateOrbitAnimation();
    void advanceOrbit();
    std::optional<SpatialInterior> m_interior;
    QString m_interiorName;
    QVector<DesktopAppChoice> m_interiorStreams;
    QMap<QString, int> m_interiorBindings;
    void paintInterior(QPainter *painter);
    void paintCockpit(QPainter *painter);
    void paintPlanets(QPainter *painter);
    void paintHologram(QPainter *painter);
    void paintReturnPortal(QPainter *painter);
    void selectAt(const QPointF &position);
    QVector3D selectionRay(const QPointF &position) const;
    bool targetsReturnPortal(const QVector3D &ray) const;
    qreal returnPortalRadius() const;
    void clearHologram();
    void selectFile(const Building &building);
    void renderFilePreview(const QString &text, bool truncated = false);
    void approachPlanet(const Building &planet);
    void approachDirectory(const Building &building);
    bool finishDirectoryTravel();
    void requestLookFrame();
    void advanceTravel();
    void stop();
    void setBuildings(const QVector<WorkspaceFlightBuilding> &buildings);
    qreal doorOpening(const Building &building) const;
    const Building *aimedBuilding(qreal *distance = nullptr) const;
    const Building *aimedBuilding(const QVector3D &ray, qreal *distance = nullptr) const;
    int aimedBillboard() const;
    int aimedBillboard(const QVector3D &ray) const;
    bool blocked(const QVector3D &position) const;
    QVector3D cameraPoint(const QVector3D &world) const;
    QPointF screenPoint(const QVector3D &point) const;
    QPolygonF projectPolygon(const QVector<QVector3D> &world) const;
    void drawWorldLine(QPainter *painter, const QVector3D &a, const QVector3D &b) const;
    QVector<Building> m_buildings;
    QMap<QString, QRectF> m_planetLabels;
    QVector3D m_camera;
    QSet<int> m_keys;
    QTimer m_tick;
    QTimer m_shot;
    QTimer m_travel;
    QTimer m_lookFrame;
    QTimer m_orbitTick;
    QElapsedTimer m_orbitElapsed;
    double m_orbitSeconds = 0;
    bool m_orbitMotion = true;
    QString m_travelDirectory;
    QVector<QVector3D> m_directoryRoute;
    QElapsedTimer m_travelElapsed;
    QString m_travelDestination;
    QVector3D m_travelStart;
    QVector3D m_travelEnd;
    QVector3D m_travelArc;
    qreal m_travelYaw = 0;
    qreal m_travelPitch = 0;
    qreal m_travelStartYaw = 0;
    qreal m_travelStartPitch = 0;
    bool m_descending = false;
    QString m_shotPath;
    bool m_editorActive = false;
    QElapsedTimer m_elapsed;
    qreal m_yaw = 0;
    qreal m_pitch = -0.08;
    qreal m_headYaw = 0;
    qreal m_headPitch = 0;
    bool m_freeLooking = false;
    qreal m_sinYaw = 0;
    qreal m_cosYaw = 1;
    qreal m_sinPitch = 0;
    qreal m_cosPitch = 1;
    qreal m_extent = 1600;
    bool m_looking = false;
    QPoint m_lastMouse;
    WorkspaceGraphReader m_reader;
    WorkspaceGraphReader m_previewReader{nullptr, 2500};
    quint64 m_previewGeneration = 0;
    QString m_hologramPath;
    QString m_hologramTitle;
    QVector3D m_hologramAnchor;
    QImage m_hologramImage;
    QRectF m_hologramScreenRect;
    int m_hologramApp = -1;
    bool m_hologramNative = false;
    QString m_root;
    QString m_directory;
    QString m_pendingDirectory;
    QString m_after;
    QString m_stamp;
    QString m_worldStatus;
    quint64 m_generation = 0;
    bool m_more = false;
    bool m_hidden = true;
    FlightAppBillboards *m_billboards = nullptr;
    QHash<QString, int> m_appSurfaceBindings;
    QPushButton *m_fullscreenButton = nullptr;
    QVector<QPointer<QWidget>> m_hiddenChrome;
    QPointer<QWidget> m_fullscreenWindow;
    Qt::WindowStates m_previousWindowState;
    bool m_fullscreen = false;
    bool m_shipVisible = true;
    bool m_cockpit = false;
    bool m_orbit = false;
    QVector3D m_pilotMovement;
    QPointF m_pilotLook;
    bool m_pilotBoost = false;
    qint64 m_lastDrawNanoseconds = 0;
    quint64 m_renderedFrames = 0;
};
