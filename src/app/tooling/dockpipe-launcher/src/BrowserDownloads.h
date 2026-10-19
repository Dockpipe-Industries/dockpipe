#pragma once

#include <QObject>
#include <QPointer>
#include <QString>
#include <map>
#include <memory>

class QDialog;
class QPushButton;
class QTreeWidget;
class QTreeWidgetItem;
class QTemporaryDir;
class QWebEngineDownloadRequest;
class QWebEngineProfile;
class QWidget;

// Owns the ordinary browser's downloads and bounded, session-only download list.
// Closing its panel does not cancel transfers. Destroying the manager does.
class BrowserDownloads final : public QObject
{
    Q_OBJECT

public:
    explicit BrowserDownloads(QWebEngineProfile *profile, QWidget *host);
    ~BrowserDownloads() override;

    void showPanel();

signals:
    void statusChanged(const QString &message);

private:
    struct Entry {
        QPointer<QWebEngineDownloadRequest> request;
        QString fileName;
        QString destination;
        QString status;
        qint64 receivedBytes = 0;
        qint64 totalBytes = -1;
        bool finished = false;
        bool completed = false;
        bool retainedForRecovery = false;
        QString automaticDirectory;
        std::unique_ptr<QTemporaryDir> stagingDirectory;
        QTreeWidgetItem *row = nullptr;
    };

    void requestDownload(QWebEngineDownloadRequest *request);
    void updateDownload(quint64 id);
    void updateRow(Entry &entry);
    void updateActions();
    bool finalizeAutomaticDownload(Entry &entry);
    Entry *selectedEntry();
    void clearFinished();
    void removeOldestFinished();

    QPointer<QWidget> m_host;
    QPointer<QDialog> m_panel;
    QTreeWidget *m_list = nullptr;
    QPushButton *m_pause = nullptr;
    QPushButton *m_cancel = nullptr;
    QPushButton *m_reveal = nullptr;
    QPushButton *m_clear = nullptr;
    std::map<quint64, Entry> m_entries;
    quint64 m_nextId = 0;
    bool m_promptOpen = false;
};
