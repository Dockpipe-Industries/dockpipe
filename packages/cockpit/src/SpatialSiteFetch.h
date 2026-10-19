#pragma once
#include <QObject>
#include <QUrl>
#include <QByteArray>
#include <QTimer>
#include <QPointer>
#include <QNetworkAccessManager>

struct SpatialSiteResponse {
    QUrl url;
    int status = 0;
    QByteArray body;
    QByteArray contentType;
    QByteArray robots;
    QString modified;
    QUrl redirect;
    QString error;
};
// Anonymous, pinned public-IP HTTP transport. Test fixtures inject a transport,
// never a production flag that bypasses the public-network boundary.
class SpatialSiteFetch : public QObject {
    Q_OBJECT
public:
    explicit SpatialSiteFetch(QObject *parent = nullptr);
    ~SpatialSiteFetch() override;
    virtual void get(const QUrl &url, const QUrl &origin, int maximumBytes = 512 * 1024);
    virtual void cancel();
    bool busy() const { return m_busy; }
signals:
    void completed(const SpatialSiteResponse &response);
private:
    void finish(const SpatialSiteResponse &response);
    QNetworkAccessManager m_network;
    QPointer<QNetworkReply> m_reply;
    QTimer m_deadline;
    int m_lookup = -1;
    quint64 m_generation = 0;
    bool m_busy = false;
    int m_maximumBytes = 512 * 1024;
    SpatialSiteResponse m_response;
};
