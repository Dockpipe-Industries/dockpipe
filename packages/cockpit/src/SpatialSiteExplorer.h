#pragma once
#include "SpatialSiteCrawler.h"
#include <QWidget>
#include <QCache>

class QCheckBox;
class QLabel;
class QTreeWidget;
class QSpinBox;
class QLineEdit;
class QPushButton;
class SpatialSiteExplorer final : public QWidget {
    Q_OBJECT
public:
    explicit SpatialSiteExplorer(QWidget *parent = nullptr, SpatialSiteFetch *transport = nullptr);
    bool begin(const QUrl &url);
    bool active() const { return m_active; }
    bool enabled() const;
    void pause();
    void leaveArea();
    void resumeArea();
    void overview();
    bool activate(const QString &id);
    SpatialSiteCrawler *crawler() const { return m_crawler; }
    QString location() const { return m_section; }
    void rememberDesign(const SpatialWebDocument &document);
    void decorate(SpatialWebDocument &document) const;
signals:
    void areaReady(const SpatialWebDocument &document);
    void textureReady(const QString &id, const QImage &image);
    void sourceRequested(const QUrl &url);
    void singlePageRequested();
private:
    void updateMap();
    void enter(const QString &section, int page = 0);
    void showSelected();
    QString selectedId() const;
    SpatialSiteBudget budget() const;
    SpatialSiteCrawler *m_crawler;
    QCheckBox *m_enabled;
    QWidget *m_details;
    QTreeWidget *m_tree;
    QLineEdit *m_search;
    QLabel *m_status;
    QLabel *m_breadcrumb;
    QLabel *m_inspect;
    QSpinBox *m_pages;
    QSpinBox *m_depth;
    QSpinBox *m_score;
    QPushButton *m_update;
    QString m_section;
    QString m_initialSource;
    int m_areaPage = 0;
    bool m_active = false;
    bool m_enteredContent = false;
    // Existing local neural features only, bounded by the same area count.
    QCache<QString, QImage> m_images{12};
    SpatialWebDocument m_published;
    quint64 m_areaGeneration = 0;
};
