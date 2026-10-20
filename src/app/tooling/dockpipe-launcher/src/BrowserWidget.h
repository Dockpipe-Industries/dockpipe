#pragma once

#include <QDateTime>
#include <QHash>
#include <QPointer>
#include <QSet>
#include <QUrl>
#include <QWidget>
#include <QWebEnginePage>
#include <memory>

class BrowserDownloads;
class BrowserTabBar;
struct BrowserSession;
class QLabel;
class QLineEdit;
class QMenu;
class QProgressBar;
class QPrinter;
class QPushButton;
class QStackedWidget;
class QTabBar;
class QTemporaryDir;
class QWebEngineProfile;
class QWebEngineView;

class BrowserWidget final : public QWidget {
    Q_OBJECT
public:
    explicit BrowserWidget(QWidget *parent = nullptr);
    ~BrowserWidget() override;
    void navigate(const QUrl &url);
    QWebEngineView *activeView() const { return m_view; }
    void setChromeVisible(bool visible);
    void setCanvasPresentationEnabled(bool enabled);
    void setCanvasViewportMode(bool enabled);
    bool canvasViewportMode() const;
    bool moveTabTo(QWebEngineView *view, BrowserWidget *destination);
    void setBackgroundSuspended(bool suspended);
    static QUrl addressUrl(const QString &text);

signals:
    void activeViewAboutToChange(QWebEngineView *previous);
    void activeViewChanged(QWebEngineView *current);
    void tabTearOutRequested(QWebEngineView *view, const QPoint &globalPosition);

protected:
    bool eventFilter(QObject *object, QEvent *event) override;
    void hideEvent(QHideEvent *event) override;
    void resizeEvent(QResizeEvent *event) override;

private:
    struct Visit {
        QString title;
        QUrl url;
        QDateTime time;
    };
    void buildChrome();
    void createSession();
    void destroySession();
    bool hasSharedSession() const;
    QWebEngineView *addTab(const QUrl &url = {});
    void attachTab(QWidget *container, QWebEngineView *view);
    void updatePresentation();
    void closeTab(int index);
    void selectTab(int index);
    void updateNavigation();
    void updateToolbarLayout();
    void submitAddress();
    void showStatus(const QString &text);
    void showFind();
    void findText(bool backwards = false);
    void showHistory();
    void showExtensions();
    void showSettings();
    void applyPreferences();
    void cancelPermissionPrompts();
    void requestSitePermission(QWebEngineView *view, const QUrl &origin, QWebEnginePage::Feature feature);
    void showSiteInfo();
    void clearBrowsingData();
    void capturePage(bool annotate);
    void printPage();
    void setZoom(qreal zoom);
    void setViewportWidth(int width);
    QMenu *createMenu();

    std::shared_ptr<BrowserSession> m_session;
    QHash<QWebEngineView *, std::shared_ptr<BrowserSession>> m_tabSessions;
    QWebEngineProfile *m_profile = nullptr;
    BrowserDownloads *m_downloads = nullptr;
    QWebEngineView *m_view = nullptr;
    BrowserTabBar *m_tabs = nullptr;
    QWidget *m_toolbar = nullptr;
    QPushButton *m_viewportButton = nullptr;
    QPushButton *m_newTabButton = nullptr;
    QStackedWidget *m_pages = nullptr;
    QLineEdit *m_address = nullptr;
    QPushButton *m_back = nullptr;
    QPushButton *m_forward = nullptr;
    QPushButton *m_reload = nullptr;
    QPushButton *m_stop = nullptr;
    QPushButton *m_annotate = nullptr;
    QPushButton *m_extensionsButton = nullptr;
    QPushButton *m_installExtension = nullptr;
    QPushButton *m_downloadsButton = nullptr;
    QPushButton *m_goButton = nullptr;
    QLabel *m_status = nullptr;
    QProgressBar *m_progress = nullptr;
    QWidget *m_findBar = nullptr;
    QLineEdit *m_findText = nullptr;
    QLabel *m_findResult = nullptr;
    QList<Visit> m_history;
    QHash<QWebEngineView *, QSet<QString>> m_permissionOrigins;
    std::unique_ptr<QPrinter> m_printer;
    bool m_printing = false;
    bool m_javascript = true;
    bool m_images = true;
    bool m_showFullUrl = false;
    bool m_destroyingSession = false;
    bool m_chromeVisible = true;
    bool m_canvasPresentationEnabled = false;
    bool m_canvasViewportRequested = true;
    bool m_backgroundSuspended = false;
    bool m_suspendInactivePages = false;
};
