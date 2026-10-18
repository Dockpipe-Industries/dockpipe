#pragma once
#include "SpatialPaintOrder.h"
#include "WorkspaceGraphView.h"
#include "WorkspaceGraphReader.h"
#include <QElapsedTimer>
#include <QSet>
#include <QPointer>
#include <QTimer>
#include "DesktopAppWindow.h"
#include "SpatialWorkspace.h"
#include "ArcadeFlightDynamics.h"
#include "DirtBikeDynamics.h"
#include "SpatialTerrain.h"
#include "DirtBikeRace.h"
#include "DirtBikeSound.h"
#include "DirtBikeDust.h"
#include "DirtBikeCameraLook.h"
#include "ScrollingPreview.h"
#include <optional>

class FlightAppBillboards;
class FurnishingController;
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
// movement and proximity doors repaint while controls or craft inertia are active.
class SpatialGpuRenderer;

class WorkspaceFlightView final : public WorkspaceGraphView {
public:
    explicit WorkspaceFlightView(const QVector<WorkspaceFlightBuilding> &buildings,
                                 bool hardware, QWidget *parent = nullptr);
    ~WorkspaceFlightView() override;
    void setAppBillboards(const QVector<DesktopAppChoice> &choices);
    void previewAppWindows(const QVector<DesktopAppChoice> &choices);
    void stopAppBillboards();
    bool appControlling() const;
    bool beginCabinetControl(const QString &id);
    void endCabinetControl();
    QRect cabinetControlRect() const;
    bool hasAppPreview(const QString &path) const;
    void setFlightFullscreen(bool enabled);
    bool flightFullscreen() const { return m_fullscreen; }
    void setFlightHardwareAcceleration(bool enabled);
    std::function<void()> exitRequested;
    std::function<void(const QString &)> fileTargeted;
    std::function<void(const QString &, bool)> applicationDisplayRequested;
    QVector<SpatialDisplay> interiorDisplays() const { return m_interior ? m_interior->displays : QVector<SpatialDisplay>{}; }
    void fire();
    bool activateInteriorAction(const QString &id);
    bool leaveInteriorAction();
    bool seated() const { return m_seatReturn.has_value(); }
    FurnishingController *furnishing() const { return m_furnishing; }
    bool furnishingActive() const;
    bool furnishingButton(int button, bool pressed);
    void setCockpitWorld(const QVector<WorkspaceFlightBuilding> &buildings, bool orbit);
    void approach(const QString &path);
    void skipTravel();
    void setShipVisible(bool visible) { m_shipVisible = visible; viewport()->update(); }
    void setInterior(const SpatialInterior &room, const QString &name, bool preserveLook = false, bool preservePosition = false, const QString &environmentKey = {});
    std::function<bool(const QString &)> entranceApproached;
    void streamInterior(const SpatialInterior &room, const QString &name, const QVector3D &position, float yawDelta = 0);
    bool setInteriorImage(const QString &id, const QImage &image);
    void setInteriorPhase(qreal amount);
    bool enterVehicle(const QString &id);
    bool exitVehicle();
    QString activeVehicleId() const;
    bool ridingBike() const;
    void resetBike();
    bool bikeRaceAvailable() const;
    bool startBikeRace();
    const DirtBikeRace &bikeRace() const { return m_bikeRace; }
    const DirtBikeDynamics::State &bikeState() const { return m_bike.state(); }
    void setPilotVehicleControls(float throttle, float brake);
    void toggleBikeCamera();
    void toggleBikeSound();
    bool terrainWeatherAvailable() const { return m_interior && bool(m_interior->terrain); }
    bool terrainRaining() const { return terrainWeatherAvailable() && m_interior->terrain->raining(); }
    bool terrainWeatherActive() const { return terrainWeatherAvailable() && m_interior->terrain->weatherActive(); }
    void toggleTerrainRain();
    const SpatialTerrain *terrain() const { return terrainWeatherAvailable() ? m_interior->terrain.get() : nullptr; }
    qint64 lastTerrainUpdateNanoseconds() const { return m_lastTerrainUpdateNanoseconds; }
    bool bikeCrashed() const { return ridingBike() && m_bike.state().crashed; }
    bool bikeChaseCamera() const { return m_bikeChase || bikeCrashed(); }
    bool bikeSoundEnabled() const { return !m_bikeMuted; }
    qint64 lastBikeUpdateNanoseconds() const { return m_lastBikeUpdateNanoseconds; }
    bool nativeSceneActive() const { return m_nativeSceneActive; }
    QString nativeSceneError() const;
    bool terrainSharp() const { return m_terrainSharp; }
    bool hasTexturedTerrain() const { return m_interior && m_interior->groundMaterial != SpatialSurfaceMaterial::Plain; }
    void setTerrainSharp(bool enabled);
    QVector<SpatialVehicle> vehicles() const { return m_interior ? m_interior->vehicles : QVector<SpatialVehicle>{}; }
    std::function<void()> locomotionChanged;
    void setInteriorLocomotion(const QString &mode);
    QString interiorLocomotion() const { return m_interior ? m_interior->locomotion : QString(); }
    bool connectInteriorDisplay(const QString &id, const DesktopAppChoice &choice, bool control);
    bool hasInteriorPreview(const QString &id) const;
    int interiorStreamCount() const { return m_interiorStreams.size(); }
    quint64 captureCount() const;
    qint64 lastCaptureNanoseconds() const;
    bool walking() const { return m_interior.has_value(); }
    void setSurfaceSpawn(const QPointF &spawn);
    void setFlightMode(ArcadeFlightDynamics::Mode mode);
    ArcadeFlightDynamics::Mode flightMode() const { return m_dynamics.mode(); }
    QVector3D flightVelocity() const { return m_dynamics.velocity(); }
    QQuaternion craftOrientation() const;
    void setPilotFlightControls(float roll, bool brake);
    void suspendPilot();
    void setPilotInput(const QVector3D &movement, const QPointF &look, bool boost);
    void controlApp(const DesktopAppChoice &choice, bool fullscreen);
    void previewApp(const QString &path, const DesktopAppChoice &choice);
    void previewSurface(const QString &path, const QString &title, const QImage &image);
    void previewDocument(const QString &path, const QString &file);
    void previewCard(const QString &path, const QString &title, const QString &text);
    QString previewPath() const { return m_hologramPath; }
    bool previewLoading() const { return m_previewReader.busy(); }
    QImage previewImage() const { return m_hologramImage; }
    void setControllerHints(bool controller);
    bool controllerHints() const { return m_controllerHints; }
    QString inputHints() const;
    bool previewAutoScroll() const { return m_previewAutoScroll; }
    void setPreviewAutoScroll(bool enabled) { m_previewAutoScroll = enabled; }
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
    friend class FurnishingController;
    QString m_cabinetDisplay;
    std::optional<QVector3D> m_cabinetReturn;
    QPointF m_cabinetLook;
    FurnishingController *m_furnishing = nullptr;
    bool m_changingInterior = false;
    struct MeshFace {
        QVector<QVector3D> vertices;
        QVector3D center;
        QVector3D normal;
        QColor color;
        QColor edge;
        QString label;
        QString displayId;
        SpatialSurfaceMaterial material = SpatialSurfaceMaterial::Plain;
        QColor albedo;
        QVector<QVector3D> normals;
        SpatialSurfaceFinish finish = SpatialSurfaceFinish::Standard;
        QVector<QVector2D> uvs;
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
    bool craftFlying() const;
    void prepareCraft();
    void updateFlightModeButton();
    QQuaternion viewOrientation() const;
    ArcadeFlightDynamics m_dynamics;
    bool m_attitudeActive = false;
    float m_pilotRoll = 0;
    bool m_pilotBrake = false;
    QVector3D m_viewRight{1, 0, 0};
    QVector3D m_viewUp{0, 1, 0};
    QVector3D m_viewForward{0, 0, 1};
    QPushButton *m_flightModeButton = nullptr;
    void updateOrbitAnimation();
    void advanceOrbit();
    std::optional<SpatialInterior> m_interior;
    QString m_interiorName;
    qreal m_interiorPhase = 0;
    qreal m_driveSpeed = 0;
    DirtBikeDynamics m_bike;
    DirtBikeRace m_bikeRace;
    DirtBikeSound m_bikeSound;
    DirtBikeCameraLook m_bikeLook;
    float m_bikeThrottle = 0;
    float m_bikeBrake = 0;
    bool m_bikeChase = true;
    bool m_bikeMuted = false;
    bool m_bikeCameraPending = false;
    DirtBikeDust m_bikeDust;
    void updateBikeCamera(float seconds, bool snap = false);
    bool visibleVehicleFace(int vehicle, int face) const;
    SpatialVehicle m_bikeStart;
    void advanceBike(qreal seconds);
    bool bikeUnsettled() const { return ridingBike() && (m_bike.unsettled() || m_bikeRace.active() || terrainWeatherActive() || !m_bikeDust.isEmpty() || m_bikeCameraPending || m_bikeLook.pending() || m_bikeThrottle > 0 || m_bikeBrake > 0); }
    SpatialPaintOrder m_interiorPaintOrder;
    bool m_interiorPaintDirty = true;
    QVector<MeshFace> m_interiorGeometry;
    QVector<MeshFace> m_foliageGeometry;
    QVector<MeshFace> m_terrainGeometry;
    QVector3D m_terrainGeometryCamera;
    float m_terrainGeometryAge = 1;
    float m_weatherTime = 0;
    qint64 m_lastTerrainUpdateNanoseconds = 0;
    void updateTerrainGeometry();
    void paintTerrainWeather(QPainter *painter);
    qint64 m_lastBikeUpdateNanoseconds = 0;
    QVector<MeshFace> m_furnishingGeometry;
    int m_furnishingStaticFaces = 0;
    QVector<QVector<SpatialVehicleFace>> m_vehicleModels;
    QVector<QVector<MeshFace>> m_vehicleGeometry;
    int m_activeVehicle = -1;
    std::optional<QVector3D> m_seatReturn;
    std::optional<QVector3D> m_droneReturn;
    void updateVehicleGeometry(int index);
    void advanceVehicle(const QVector3D &delta, qreal steering, qreal seconds);
    void paintVehicleCockpit(QPainter *painter);
    QMap<QString, QImage> m_interiorImages;
    QVector<DesktopAppChoice> m_interiorStreams;
    QMap<QString, int> m_interiorBindings;
    void paintInterior(QPainter *painter);
    void paintInteriorScene(QPainter *painter);
    void paintInteriorOverlay(QPainter *painter);
    bool paintNativeInterior(QPainter *painter);
    SpatialGpuRenderer *m_gpuRenderer = nullptr;
    bool m_nativeSceneActive = false;
    QImage m_terrainFrame; // One bounded raster layer for textured OpenGL scenes.
    bool m_terrainSharp = true;
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
    qreal viewFocalLength() const;
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
    ScrollingPreview m_textPreview;
    QTimer m_textPreviewTimer;
    QElapsedTimer m_textPreviewClock;
    qint64 m_textPreviewMilliseconds = 0;
    bool m_previewAutoScroll = true;
    bool m_controllerHints = false;
    int m_textPreviewOffset = 0;
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
