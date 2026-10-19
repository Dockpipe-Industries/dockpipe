#pragma once

#include "LauncherUiExport.h"

#include <QObject>
#include <QUrl>
#include <functional>

// The desktop's single owner of QDesktopServices HTTP(S) handlers. Explicit
// browser navigation and file/folder URLs do not pass through this router.
class DOCKPIPE_LAUNCHER_UI_EXPORT BrowserLinkRouter final : public QObject {
    Q_OBJECT
public:
    using ExternalOpener = std::function<bool(const QUrl &)>;

    explicit BrowserLinkRouter(QObject *parent = nullptr, ExternalOpener externalOpener = {});
    ~BrowserLinkRouter() override;

    static bool openInSystemBrowser(const QUrl &url);

signals:
    void openInDockPipe(const QUrl &url);
    void failed(const QString &message);

private slots:
    void openUrl(const QUrl &url);

private:
    bool openExternal(const QUrl &url);
    ExternalOpener m_externalOpener;
};
