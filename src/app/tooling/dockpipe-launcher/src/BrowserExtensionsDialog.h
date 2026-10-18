#pragma once

#include <QDialog>
#include <QPointer>
#include <QUrl>

class QLabel;
class QListWidget;
class QPushButton;
class QWebEngineProfile;

class BrowserExtensionsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit BrowserExtensionsDialog(QWebEngineProfile *profile, QWidget *parent = nullptr);

signals:
    void openUrl(const QUrl &url);

private:
    void refreshExtensions();
    void updateSelection();
    void loadUnpacked();
    void toggleSelected();
    void unloadSelected();
    void openSelected();
    void setBusy(bool busy);

    QPointer<QWebEngineProfile> m_profile;
    QListWidget *m_extensions = nullptr;
    QLabel *m_details = nullptr;
    QLabel *m_status = nullptr;
    QPushButton *m_load = nullptr;
    QPushButton *m_toggle = nullptr;
    QPushButton *m_unload = nullptr;
    QPushButton *m_open = nullptr;
    QString m_pendingPath;
    bool m_available = false;
    bool m_busy = false;
};
