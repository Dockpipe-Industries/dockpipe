#pragma once
#include "CockpitSettings.h"
#include "WorkspaceGraphReader.h"
#include "WorkspaceGraphView.h"
#include <QMap>
#include <QWidget>
#include <QVariantAnimation>

class QAction;
class CanvasView;
class WorkspaceGraphNode;
class QGraphicsPathItem;
class QLabel;
class QPushButton;
class WorkspaceFlightView;

struct WorkspaceGraphLimits {
    int pageNodes = 150;
    int visibleNodes = 500;
};

// Ephemeral filesystem hierarchy. Only visible nodes are retained; collapsed
// descendants are deleted. Other relationship providers are intentionally absent.
class WorkspaceGraphWidget final : public QWidget {
    Q_OBJECT
public:
    WorkspaceGraphWidget(const QString &workspaceRoot, const QString &directory,
                         QWidget *parent = nullptr, WorkspaceGraphLimits limits = {});
    ~WorkspaceGraphWidget() override;
    void navigate(const QString &directory);
    void expand(const QString &path);
    void collapse(const QString &path);
    void loadMore(const QString &path);
    void selectPath(const QString &path);
    int nodeCount() const { return m_nodes.size(); }
    bool hasPath(const QString &path) const { return m_nodes.contains(path); }
    bool hasMore(const QString &path) const;
    bool busy() const { return m_reader.busy(); }
    QString statusText() const;
    qint64 lastLayoutMicroseconds() const { return m_layoutMicroseconds; }
    CanvasView *canvas() const { return m_view; }
    bool spatial() const { return m_view->spatial(); }
    void setSpatial(bool enabled);
    void applyCockpitSettings(const CockpitSettings &settings);
    void openFlight();
    void closeFlight();
    void activatePath(const QString &path);
    WorkspaceFlightView *flightView() const { return m_flight; }
signals:
    void closeRequested();
    void openFileRequested(const QString &path);
    void flightFileRequested(const QString &path);
    void flightClosing();
    void flightRendererChanged();
    void revealRequested(const QString &path);
    void pageApplied();
protected:
    void hideEvent(QHideEvent *event) override;
private:
    struct Node {
        QString parent;
        QString kind;
        QStringList children;
        QString after;
        QString stamp;
        WorkspaceGraphNode *item = nullptr;
        QGraphicsPathItem *edge = nullptr;
        int depth = 0;
        int row = 0;
        QPointF flatPosition;
        QVector3D cityPosition;
        bool expanded = false;
        bool more = false;
    };
    void readPage(const QString &path);
    void applyPage(quint64 token, const QJsonObject &page);
    void updateActions();
    void updateSummary(const QString &path);
    void updateLinks(const QString &path);
    void addNode(const QString &path, const QString &kind, const QString &parent, qint64 fileBytes = -1);
    void showMenu(const QString &path, const QPoint &position);
    void configureLimits();
    void projectNodes();
    void paintSelectionLabel(QPainter *painter);
    QString selection() const;
    void animateSelection();
    void cancelTransition();
    void paintDrone(QPainter *painter);
    QVariantAnimation m_transition;
    QVariantAnimation m_droneMotion;
    QString m_transitionPath;
    QString m_dronePath;
    QPointF m_dronePosition;
    QPointF m_transitionStart;
    QTransform m_beforeTransition;
    QPointF m_beforeCenter;
    qreal m_transitionProgress = 0;
    bool m_animateNavigation = true;
    QString m_workspaceRoot;
    QString m_directory;
    QString m_loadingPath;
    WorkspaceGraphLimits m_limits;
    WorkspaceGraphReader m_reader;
    QMap<QString, Node> m_nodes;
    WorkspaceGraphView *m_view;
    QWidget *m_cameraControls;
    QWidget *m_graphActions;
    QWidget *m_flightControls;
    WorkspaceFlightView *m_flight = nullptr;
    bool m_flightHardware = false;
    bool m_projecting = false;
    QLabel *m_location;
    QLabel *m_status;
    QLabel *m_count;
    QPushButton *m_expand;
    QPushButton *m_more;
    QPushButton *m_open;
    quint64 m_token = 0;
    qint64 m_metadataBytes = 0;
    qint64 m_layoutMicroseconds = 0;
    bool m_showHidden = true;
    bool m_appBillboards = false;
    CockpitSettings m_cockpitSettings;
    QAction *m_flightAction;
    QAction *m_billboardsAction;
};
