#pragma once

#include <QJsonArray>
#include <QJsonObject>
#include <QWidget>

class BrowserWidget;
class RemoteCommand;
class QWebEngineView;
class WorkspaceMachineMenu;
class QStackedWidget;
class BrowserNetworkContext;

class RemoteBrowserWidget final : public QWidget {
    Q_OBJECT
public:
    explicit RemoteBrowserWidget(const QString &contextId, QWidget *parent = nullptr);
    ~RemoteBrowserWidget() override;
    void setMachineCatalog(const QJsonArray &catalog, bool unlocked);
    void setMachineContext(const QJsonObject &target, const QString &root = {});
    void navigate(const QUrl &url);
    void setChromeVisible(bool visible);
    QString contextId() const { return m_contextId; }
    QJsonObject machineContext() const;
    void setCanvasPresentationEnabled(bool enabled);
    void setCanvasViewportMode(bool enabled);
    void setBackgroundSuspended(bool suspended);
    BrowserWidget *localBrowser() const;
    QWebEngineView *activeView() const;
    QJsonObject machineTarget() const { return m_target; }
    QString machineRoot() const { return m_root; }

signals:
    void machineContextChanged(const QJsonObject &target, const QString &root);
    void refreshRequested();
    void unlockRequested();
    void tabTearOutRequested(QWebEngineView *view, const QPoint &globalPosition);
    void activeViewChanged(QWebEngineView *view);

private:
    QString contextKey(const QJsonObject &target) const;
    void activateContext();
    void refreshNetworkCatalog();
    void updateMachineMenu();
    QString m_contextId;
    QJsonObject m_target;
    QString m_root;
    QJsonArray m_catalog;
    QJsonArray m_networkCatalog;
    QJsonArray m_pairedCatalog;
    RemoteCommand *m_catalogCommand;
    bool m_unlocked = false;
    bool m_chromeVisible = true;
    bool m_canvasPresentation = false;
    bool m_canvasViewport = true;
    bool m_backgroundSuspended = false;
    WorkspaceMachineMenu *m_machine;
    QStackedWidget *m_stack;
    BrowserWidget *m_local = nullptr;
    QHash<QString, BrowserNetworkContext *> m_contexts;
};
