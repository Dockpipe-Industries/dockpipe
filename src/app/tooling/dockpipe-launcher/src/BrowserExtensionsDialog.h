#pragma once

#include <QDialog>
#include <QPointer>
#include <QTemporaryDir>
#include <QUrl>
#include <memory>

class BrowserExtensionPackage;
class BrowserExtensionStore;
class QLabel;
class QListWidget;
class QPushButton;
class QWebEngineProfile;
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
class QWebEngineExtensionInfo;
#endif

class BrowserExtensionsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit BrowserExtensionsDialog(QWebEngineProfile *profile, QWidget *parent = nullptr);
    void addWebStoreExtension(const QString &urlOrId);
    void installPackage(const QString &file, const QString &sourceKey = QString());
    void reject() override;

signals:
    void openUrl(const QUrl &url);

private:
    struct PendingInstall {
        bool waiting = false;
        bool cancelled = false;
    };
    void refreshExtensions();
    void updateSelection();
    void loadUnpacked();
    void choosePackage();
    void chooseWebStoreExtension();
    void toggleSelected();
    void unloadSelected();
    void openSelected();
    void setBusy(bool busy);
#if QT_VERSION >= QT_VERSION_CHECK(6, 10, 0)
    void finishInstall(const QWebEngineExtensionInfo &extension);
#endif

    QPointer<QWebEngineProfile> m_profile;
    BrowserExtensionStore *m_store = nullptr;
    BrowserExtensionPackage *m_download = nullptr;
    QListWidget *m_extensions = nullptr;
    QLabel *m_details = nullptr;
    QLabel *m_status = nullptr;
    QPushButton *m_webStore = nullptr;
    QPushButton *m_package = nullptr;
    QPushButton *m_load = nullptr;
    QPushButton *m_toggle = nullptr;
    QPushButton *m_unload = nullptr;
    QPushButton *m_open = nullptr;
    QString m_pendingPath;
    QString m_pendingKey;
    QStringList m_existingIds;
    std::shared_ptr<QTemporaryDir> m_packageDirectory;
    std::shared_ptr<PendingInstall> m_pendingInstall;
    bool m_available = false;
    bool m_busy = false;
};
