#pragma once

#include "LauncherUiExport.h"

#include <QJsonObject>
#include <QIcon>
#include <QPixmap>
#include <QPointer>
#include <QMap>
#include <QSet>
#include <QStringList>
#include <QWidget>

class CanvasPanel;
class QLabel;
class QMimeData;
class QScrollArea;
class QShortcut;
class QSplitter;
class QTabWidget;
class QTimer;
class QToolButton;
class WindowControlButton;

// Native widgets are reparented between layouts, never rendered through a proxy.
class DOCKPIPE_LAUNCHER_UI_EXPORT DashboardWorkspace final : public QWidget {
    Q_OBJECT
public:
    explicit DashboardWorkspace(const QString &dashboardId, QWidget *parent = nullptr);
    ~DashboardWorkspace() override;
    bool addPanel(const QString &id, const QString &title, QWidget *content);
    void removePanel(const QString &id);
    void setMode(const QString &mode);
    QString mode() const;
    void setCompactTiling(bool compact);
    void setShowPanelHeaders(bool show);
    bool setCanvasBackground(const QString &kind, const QString &path = {}, bool refreshImage = false);
    void copyBackgroundFrom(const DashboardWorkspace &workspace);
    void setPanelTitle(const QString &id, const QString &title);
    void setPanelIcon(const QString &id, const QIcon &icon);
    QJsonObject saveLayout() const;
    bool restoreLayout(const QJsonObject &layout);

    // These also back the panel menu and drag/drop operations.
    bool dockPanel(const QString &id, const QString &targetId, const QString &position);
    bool floatPanel(const QString &id);
    bool placePanelAt(const QString &id, const QPoint &position);
    bool minimizePanel(const QString &id);
    bool activatePanel(const QString &id);
    bool togglePanelExpanded(const QString &id);
    QString expandedPanel() const;

signals:
    void layoutChanged();
    void panelCloseRequested(const QString &id);
    void panelReleaseRequested(const QString &id);

protected:
    bool eventFilter(QObject *object, QEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void paintEvent(QPaintEvent *event) override;
    void dragEnterEvent(QDragEnterEvent *event) override;
    void dragMoveEvent(QDragMoveEvent *event) override;
    void dragLeaveEvent(QDragLeaveEvent *event) override;
    void dropEvent(QDropEvent *event) override;

private:
    struct Panel {
        CanvasPanel *frame = nullptr;
        QWidget *header = nullptr;
        QString title;
        QIcon icon;
        QWidget *footer = nullptr;
        bool footerInitiallyHidden = false;
        WindowControlButton *expand = nullptr;
        QToolButton *tileActions = nullptr;
        QPointer<QWidget> tabPage;
    };
    QWidget *buildTree(const QJsonObject &node, QWidget *parent);
    QJsonObject captureTree(QWidget *widget) const;
    void rebuild();
    void invalidateTilingPresentation();
    void applyPanelChrome(const QString &id, bool tiled);
    void applyTilingChrome();
    void updateTileActions();
    void updateTileActionsPolling();
    void showFocusedTileMenu();
    void applyMinimizedPanels();
    QString panelId(QWidget *frame) const;
    QWidget *panelTabPage(const QString &id) const;
    void materializePanel(const QString &id);
    void rememberGeometry();
    void showPanelMenu(const QString &id, const QPoint &position);
    QSize minimumPanelSize(const QString &id) const;
    QRect defaultGeometry(const QString &id, const QMap<QString, QRect> &occupied) const;
    QRect constrainedGeometry(const QString &id, const QRect &geometry) const;
    void placePendingPanels();
    QString panelAt(const QPoint &position) const;
    QString dropPosition(const QString &targetId, const QPoint &position) const;
    QString panelDropTarget(const QString &source, const QString &target, const QString &position) const;
    void showDropPreview(const QString &targetId, const QString &position);
    void hideDropPreview();
    void finishPanelDrag(bool cancelClick = true);
    bool acceptsDrag(const QMimeData *mime) const;
    void restoreExpandedPanel();
    void updateExpansionControls();
    void resizeExpandedPanel();

    struct ExpandedVisibility {
        QPointer<QWidget> widget;
        bool hidden = false;
    };
    struct ExpandedSplitter {
        QPointer<QSplitter> splitter;
        QList<int> sizes;
    };
    struct ExpandedTabs {
        QPointer<QTabWidget> group;
        int currentIndex = 0;
    };
    QString m_expandedPanel;
    QJsonObject m_expansionLayout;
    QRect m_expansionGeometry;
    QPointer<QTabWidget> m_expandedGroup;
    QList<ExpandedVisibility> m_expansionVisibility;
    QList<ExpandedSplitter> m_expansionSplitters;
    QList<ExpandedTabs> m_expansionTabs;
    QPoint m_expansionScroll;
    QString m_dashboardId;
    QString m_mode = QStringLiteral("canvas");
    QMap<QString, Panel> m_panels;
    QStringList m_canvasOrder;
    QSet<QString> m_pendingPlacement;
    QSet<QString> m_minimized;
    QMap<QString, QRect> m_canvasGeometry;
    QMap<QString, QRect> m_floatingGeometry;
    QSet<QString> m_floating;
    QJsonObject m_tree;
    QWidget *m_tiles = nullptr;
    QScrollArea *m_tileViewport = nullptr;
    QScrollArea *m_retainedViewport = nullptr;
    QWidget *m_retainedTiles = nullptr;
    QList<QTabWidget *> m_retainedGroups;
    QList<QSplitter *> m_retainedSplits;
    QList<QTimer *> m_retainedAttachTimers;
    QMap<QSplitter *, QList<int>> m_retainedSplitterSizes;
    QPoint m_retainedScroll;
    QSet<QString> m_retainedMinimized;
    quint64 m_presentationRevision = 0;
    QWidget *m_parking = nullptr;
    QLabel *m_empty = nullptr;
    QList<QWidget *> m_dropPreviewEdges;
    QPoint m_dragStart;
    QString m_dragPanel;
    QPointer<QWidget> m_dragHandle;
    QShortcut *m_cancelDrag = nullptr;
    bool m_dragging = false;
    bool m_rebuilding = false;
    bool m_restoredGeometryPending = false;
    bool m_compactTiling = true;
    bool m_showPanelHeaders = true;
    QTimer *m_tileActionsTimer = nullptr;
    QString m_backgroundKind = QStringLiteral("grid");
    QString m_backgroundPath;
    QPixmap m_backgroundImage;
};
