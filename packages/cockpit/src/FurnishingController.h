#pragma once
#include "FurnishingLayout.h"
#include <QObject>
#include <QTimer>
#include <QSet>
#include <QMap>
#include <optional>

class WorkspaceFlightView;
class QFrame;
class QLabel;
class QComboBox;
class QLineEdit;
class QPlainTextEdit;
class QPushButton;
class QKeyEvent;
class QListWidget;
class QSpinBox;

class FurnishingController final : public QObject {
    Q_OBJECT
public:
    explicit FurnishingController(WorkspaceFlightView *view);
    void attach(const SpatialInterior &base);
    void detach();
    void toggle();
    bool active() const { return m_active; }
    bool available() const { return m_ready; }
    bool configuring() const { return m_active && m_configuring; }
    bool key(QKeyEvent *event);
    bool button(int button, bool pressed);
    void point(const QPointF &position);
    void updatePreview();
    void selectType(const QString &type);
    void selectObject(const QString &id);
    void selectAt(const QPointF &position);
    void confirm();
    void cancel();
    void rotate(float degrees);
    void moveSelected();
    void removeSelected();
    void duplicateSelected();
    bool bindSelected(const ScreenBinding &binding);
    bool activateScreen(const QString &id);
    void configure();
    void resizeHud();
    void refreshInputHints();
    QString sourceId(const QString &display) const;
    bool suppress(const QString &display) const { return m_suppressed.contains(display); }
    bool baseSource(const QString &id) const;
    bool applicationSource(const QString &id) const;
    const SpatialDisplay *baseDisplay(const QString &id) const;
    const QVector<FurnishingObject> &objects() const { return m_objects; }
    const std::optional<FurnishingObject> &ghost() const { return m_ghost; }
    QString validity() const { return m_validity; }
    QString selectedId() const { return m_selected; }
private:
    ScreenBinding configuredBinding() const;
    void refreshApplications();
    void loadSurfaceBinding();
    void rebuild(bool refreshSources = false);
    void updateHud();
    void browse(int categoryDelta, int itemDelta);
    void cycleBuildHeight();
    bool publish(const FurnishingObject &object);
    FurnishingObject *selected();
    WorkspaceFlightView *m_view;
    SpatialInterior m_base;
    FurnishingLayout m_layout;
    QVector<FurnishingObject> m_objects;
    std::optional<FurnishingObject> m_ghost;
    FurnishingHit m_hit;
    QSet<QString> m_suppressed;
    QMap<QString, QString> m_sources;
    QString m_selected;
    QString m_validity;
    QString m_error;
    bool m_active = false;
    bool m_ready = false;
    bool m_configuring = false;
    float m_rotation = 0;
    QPointF m_pointer;
    bool m_mouseTarget = false;
    QTimer m_tick;
    QFrame *m_hud;
    QLabel *m_info;
    QLabel *m_hints;
    QComboBox *m_categories;
    QComboBox *m_items;
    QListWidget *m_grid;
    QLineEdit *m_search;
    QSpinBox *m_buildHeight;
    QWidget *m_configuration;
    QComboBox *m_provider;
    QComboBox *m_surface;
    QComboBox *m_source;
    QComboBox *m_application;
    QVector<SpatialApplicationReference> m_applications;
    QPushButton *m_browseFile;
    QLineEdit *m_target;
    QPlainTextEdit *m_text;
    QPushButton *m_toggle;
};
