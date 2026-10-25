#pragma once

#include <QGraphicsView>
#include <QElapsedTimer>
#include <QMap>
#include <QJsonObject>
#include <QVector>
#include <QWidget>

class CanvasPanel;
class QLabel;
class QFrame;
class QPushButton;
class QGridLayout;
class QTimer;

enum class ResourceStatus { Neutral, Running, Inactive, Warning, Error };

struct ResourceNode {
    QString id;
    QString kind;
    QString title;
    QString detail;
    QString state;
    int column = 0;
    ResourceStatus status = ResourceStatus::Neutral;
    bool inactive = false;
    bool motionEligible = true;
};

struct ResourceEdge {
    QString from;
    QString to;
    QString label;
    bool dependency = false;
};

class ResourceCanvas : public QWidget {
    Q_OBJECT
public:
    ResourceCanvas(const QString &identity, const QString &title, const QString &description,
                   QWidget *details, QWidget *parent = nullptr, bool scrollDetails = true);
    void setGraph(const QVector<ResourceNode> &nodes, const QVector<ResourceEdge> &edges,
                  const QString &notice = QString());
    void showDetails();
    void setDetailsLabel(const QString &label);
    bool focusResource(const QString &id);
    void setFilter(const QString &filter);
    void addKindFilter(const QString &kind, const QString &label);
    void addInactiveFilter(const QString &label = QString());
    void addDependencyFilter();
    QJsonObject snapshot() const { return m_snapshot; }

signals:
    void resourceSelected(const QString &id);

protected:
    bool eventFilter(QObject *watched, QEvent *event) override;
    void showEvent(QShowEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;

private:
    QPushButton *addFilterButton(const QString &key, const QString &label);
    void updateVisibility();
    void updateAnimationState();
    void advanceAnimation();
    void updateEdges();
    void fitGraph();
    void keepSelectionVisible(QGraphicsItem *node);
    void arrange();
    QGraphicsView *m_view;
    QFrame *m_heading;
    QFrame *m_controls;
    QLabel *m_notice;
    QLabel *m_detailsTitle;
    QPushButton *m_detailsButton;
    CanvasPanel *m_details;
    QMap<QString, QGraphicsItem *> m_nodes;
    QVector<ResourceEdge> m_edges;
    QVector<QGraphicsPathItem *> m_paths;
    QString m_filter;
    QString m_identity;
    QFrame *m_filters;
    QGridLayout *m_filterLayout;
    QMap<QString, QPushButton *> m_kindFilters;
    QPushButton *m_inactiveFilter = nullptr;
    QPushButton *m_dependencyFilter = nullptr;
    QPushButton *m_motionFilter = nullptr;
    QTimer *m_animationTimer;
    QElapsedTimer m_animationClock;
    int m_filterCount = 0;
    bool m_fitted = false;
    bool m_userAdjusted = false;
    QJsonObject m_snapshot;
};
