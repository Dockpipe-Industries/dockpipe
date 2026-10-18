#pragma once

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
class QSplitter;
class QTabWidget;
class WindowControlButton;

// Native widgets are reparented between layouts, never rendered through a proxy.
class DashboardWorkspace final : public QWidget {
    Q_OBJECT
public:
    explicit DashboardWorkspace(const QString &dashboardId, QWidget *parent = nullptr);
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
    bool minimizePanel(const QString &id);
    bool activatePanel(const QString &id);
    bool togglePanelExpanded(const QString &id);
    QString expandedPanel() const;

signals:
    void layoutChanged();
    void panelCloseRequested(const QString &id);

protected:
    bool eventFilter(QObject *object, QEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;
    void showEvent(QShowEvent *event) override;
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
    };
    QWidget *buildTree(const QJsonObject &node, QWidget *parent);
    QJsonObject captureTree(QWidget *widget) const;
    void rebuild();
    void applyPanelChrome(const QString &id, bool tiled);
    void applyTilingChrome();
    void applyMinimizedPanels();
    QString panelId(QWidget *frame) const;
    void rememberGeometry();
    void showPanelMenu(const QString &id, const QPoint &position);
    QSize minimumPanelSize(const QString &id) const;
    QRect defaultGeometry(const QString &id, const QMap<QString, QRect> &occupied) const;
    QRect constrainedGeometry(const QString &id, const QRect &geometry) const;
    void placePendingPanels();
    QString panelAt(const QPoint &position) const;
    QString dropPosition(const QString &targetId, const QPoint &position) const;
    void showDropPreview(const QString &targetId, const QString &position);
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
    QWidget *m_parking = nullptr;
    QLabel *m_empty = nullptr;
    QWidget *m_dropPreview = nullptr;
    QPoint m_dragStart;
    QString m_dragPanel;
    bool m_rebuilding = false;
    bool m_restoredGeometryPending = false;
    bool m_compactTiling = true;
    bool m_showPanelHeaders = true;
    QString m_backgroundKind = QStringLiteral("grid");
    QString m_backgroundPath;
    QPixmap m_backgroundImage;
};
