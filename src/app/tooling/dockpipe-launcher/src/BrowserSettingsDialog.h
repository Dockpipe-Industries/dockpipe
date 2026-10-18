#pragma once

#include <QDialog>
#include <QMap>

#include <functional>

struct BrowserSettings;
class QCheckBox;
class QComboBox;
class QLabel;
class QPushButton;
class QTreeWidget;

class BrowserSettingsDialog final : public QDialog {
    Q_OBJECT
public:
    explicit BrowserSettingsDialog(QWidget *parent = nullptr);

signals:
    void historyRequested();
    void clearDataRequested();
    void extensionsRequested();
    void downloadsRequested();

private:
    void synchronize();
    void persist(const std::function<void(BrowserSettings &)> &change);
    void chooseDownloadDirectory();
    void editSiteRule(bool editSelected);
    void removeSiteRule();

    QComboBox *m_webLinks = nullptr;
    QComboBox *m_localLinks = nullptr;
    QCheckBox *m_fullUrl = nullptr;
    QCheckBox *m_askDownload = nullptr;
    QCheckBox *m_javascript = nullptr;
    QCheckBox *m_images = nullptr;
    QLabel *m_downloadDirectory = nullptr;
    QLabel *m_error = nullptr;
    QMap<QString, QComboBox *> m_permissionDefaults;
    QTreeWidget *m_siteRules = nullptr;
    QPushButton *m_editRule = nullptr;
    QPushButton *m_removeRule = nullptr;
    bool m_synchronizing = false;
};
