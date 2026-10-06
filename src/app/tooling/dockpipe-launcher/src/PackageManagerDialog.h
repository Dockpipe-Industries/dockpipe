#pragma once

#include <QDialog>
#include <QJsonArray>

class QComboBox;
class QLabel;
class QLineEdit;
class QPushButton;
class QTabWidget;
class QTableWidget;
class QTextBrowser;
class PackageCommand;

class PackageManagerDialog : public QDialog {
    Q_OBJECT
public:
    explicit PackageManagerDialog(const QString &hintWorkdir, QWidget *parent = nullptr);
    ~PackageManagerDialog() override;

private:
    void loadInstalled();
    void loadRemote();
    void installSelected();
    void applyFilter();
    void refreshDetails();
    void updateButtons();
    QJsonObject selection() const;
    void run(PackageCommand *command, const QStringList &arguments);

    QString m_hintWorkdir;
    QString m_manifest;
    QString m_installRoot;
    bool m_installing = false;
    QJsonArray m_installed;
    QJsonArray m_available;
    PackageCommand *m_localCommand = nullptr;
    PackageCommand *m_remoteCommand = nullptr;
    QComboBox *m_remote = nullptr;
    QLineEdit *m_search = nullptr;
    QLabel *m_status = nullptr;
    QLabel *m_localStatus = nullptr;
    QPushButton *m_refresh = nullptr;
    QPushButton *m_install = nullptr;
    QPushButton *m_cancel = nullptr;
    QTabWidget *m_tabs = nullptr;
    QTableWidget *m_installedTable = nullptr;
    QTableWidget *m_marketplaceTable = nullptr;
    QTextBrowser *m_details = nullptr;
};
