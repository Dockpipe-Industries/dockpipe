#pragma once

#include <QHash>
#include <QPointer>
#include <QIcon>
#include <QString>
#include <QVector>
#include <QWidget>

class QFrame;
class QTimer;
class QPropertyAnimation;
class QHBoxLayout;
class QScrollArea;
class QToolButton;

struct DashboardDockItem {
    QString id;
    QString title;
    QString kind;
    QIcon icon;

    bool operator==(const DashboardDockItem &other) const
    {
        return id == other.id && title == other.title && kind == other.kind
            && icon.cacheKey() == other.icon.cacheKey();
    }
};

// Dashboard controls only: panel creation, application launch and activation stay
// with DashboardPage and its existing workspace/services in either layout mode.
class DashboardDock final : public QWidget {
    Q_OBJECT
public:
    explicit DashboardDock(QWidget *parent = nullptr);
    void setOverlayArea(QWidget *area);
    void reveal(bool keyboardFocus = false);
    void positionOverlay();
    void setPanels(const QVector<DashboardDockItem> &panels);
    void setActivePanel(const QString &id);
    void setDesktopAppsEnabled(bool enabled);
    QSize sizeHint() const override;
    QSize minimumSizeHint() const override;

signals:
    void addWidgetRequested(const QString &kind);
    void appsRequested();
    void activatePanelRequested(const QString &id);

protected:
    bool eventFilter(QObject *watched, QEvent *event) override;

private:
    void hideIfIdle();
    void activateKind(const QString &kind);
    void updateIndicators();
    void updateFocusOrder();
    QVector<DashboardDockItem> m_panels;
    QString m_activePanel;
    QScrollArea *m_scroll;
    QWidget *m_content;
    QHBoxLayout *m_buttons;
    QToolButton *m_apps;
    QFrame *m_separator;
    QHash<QString, QToolButton *> m_pinned;
    QHash<QString, QToolButton *> m_running;
    bool m_desktopAppsEnabled = true;
    QPointer<QWidget> m_overlayArea;
    QPointer<QWidget> m_revealEdge;
    QTimer *m_hideTimer = nullptr;
    QPropertyAnimation *m_slide = nullptr;
    bool m_revealed = false;
    bool m_keyboardFocus = false;
};
