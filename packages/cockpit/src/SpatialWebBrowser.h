#pragma once

#include "SpatialWebDocument.h"
#include "SpatialWebPassage.h"
#include <QWidget>
#include <QTimer>
#include <QElapsedTimer>
#include <QSet>
#include <QVariantAnimation>
#include <optional>
#include <QJsonObject>

class QLineEdit;
class QHBoxLayout;
class QLabel;
class QPushButton;
class QStackedWidget;
class QWebEngineView;
class BrowserWidget;
class WorkspaceFlightView;
class SpatialWebMedia;
class QComboBox;
class QToolButton;
class SpatialNeuralService;
class SpatialSiteExplorer;
class SpatialSiteFetch;

// Shares the standard browser UI and projects only its selected page into one world.
class SpatialWebBrowser final : public QWidget {
    Q_OBJECT
public:
    explicit SpatialWebBrowser(bool hardware, QWidget *parent = nullptr, SpatialSiteFetch *siteTransport = nullptr);
    ~SpatialWebBrowser() override;
    void navigate(const QUrl &url, bool discoverSite = true);
    void buildSpace();
    void showPage();
    void showSpace();
    void showHeadquarters();
    void showSearch();
    void activateItem(const QString &id);
    void setHardwareAcceleration(bool enabled);
    const SpatialWebDocument &document() const { return m_document; }
    bool spatial() const;
    bool building() const { return m_building; }
    bool transitioning() const { return m_passage.has_value() || m_phase.state() == QAbstractAnimation::Running; }
    WorkspaceFlightView *scene() const { return m_scene; }
    QString status() const;
    SpatialWebMedia *media() const { return m_media; }
    SpatialSiteExplorer *siteExplorer() const { return m_site; }
signals:
    void spaceReady();
    void exitRequested();
protected:
    void hideEvent(QHideEvent *event) override;
    void showEvent(QShowEvent *event) override;
private:
    void updatePageToggle();
    void detachBrowserView(QWebEngineView *view);
    void bindBrowserView(QWebEngineView *view);
    bool beginPassage(const QString &id);
    void updatePassage();
    void cancelPassage();
    void failPassage();
    void paintPassage();
    std::optional<SpatialWebPassage> m_passage;
    SpatialInterior m_currentInterior;
    SpatialInterior m_arrivalInterior;
    std::optional<SpatialWebDocument> m_arrivalDocument;
    QVector3D m_passageStart;
    QTimer m_passageTick;
    QTimer m_passageDeadline;
    bool m_followingEntrance = false;
    bool m_passageCapturing = false;
    void configureSearch();
    void closeSearch();
    QWidget *m_search = nullptr;
    QLineEdit *m_searchQuery = nullptr;
    QLabel *m_searchStatus = nullptr;
    bool m_searchOpen = false;
    void cancelProjection();
    void configureIntelligence(QHBoxLayout *toolbar);
    void requestEnhancement();
    void applyNeuralDesign(bool preserveLook);
    void prepareBackgroundBrowser();
    void freezeBackgroundBrowser();
    void captureNext(quint64 generation, int index);
    void finishProjection(quint64 generation);
    void scheduleProjection();
    void probeLoadedContent();
    void setStatus(const QString &text);
    void publishDocument(const SpatialWebDocument &document, quint64 generation);
    SpatialSiteExplorer *m_site = nullptr;
    SpatialNeuralService *m_neural = nullptr;
    QToolButton *m_enhance = nullptr;
    QPushButton *m_applyDesign = nullptr;
    QJsonObject m_proposal;
    QVector3D m_planOrigin;
    bool m_autoDesign = false;
    QLineEdit *m_address = nullptr;
    SpatialWebMedia *m_media = nullptr;
    QComboBox *m_mobility = nullptr;
    QComboBox *m_layout = nullptr;
    QPushButton *m_pause = nullptr;
    QPushButton *m_stopMedia = nullptr;
    QLabel *m_status = nullptr;
    QPushButton *m_pageToggle = nullptr;
    QPushButton *m_go = nullptr;
    QPushButton *m_buildSpace = nullptr;
    QPushButton *m_back = nullptr;
    QPushButton *m_forward = nullptr;
    QStackedWidget *m_pages = nullptr;
    QWidget *m_start = nullptr;
    BrowserWidget *m_browserUi = nullptr;
    QWebEngineView *m_browser = nullptr; // Non-owning selected view; detached before deletion.
    WorkspaceFlightView *m_scene = nullptr;
    QTimer m_refresh;
    QTimer m_loadProbe;
    int m_loadAttempts = 0;
    bool m_earlyProjection = false;
    QTimer m_deadline;
    QElapsedTimer m_captureClock;
    QSet<int> m_pendingCaptures;
    QVariantAnimation m_phase;
    std::optional<SpatialWebDocument> m_pendingDocument;
    SpatialWebDocument m_document;
    QString m_worldNotice;
    quint64 m_generation = 0;
    bool m_liveMode = false;
    bool m_headquarters = false;
    bool m_preserveWorld = false;
    bool m_resumeTheatre = false;
    bool m_browserTabChanged = false;
    bool m_hardware = false;
    bool m_loading = false;
    bool m_building = false;
};
