#pragma once
#include "SpatialSiteWorld.h"
#include "SpatialSitePolicy.h"
#include "SpatialSiteFetch.h"
#include <QElapsedTimer>
#include <QQueue>
#include <QSet>
#include <QThreadPool>

struct SpatialSiteBudget {
    int pages = 32;
    int depth = 3;
    int milliseconds = 90000;
    int bytes = 8 * 1024 * 1024;
};
class SpatialSiteCrawler final : public QObject {
    Q_OBJECT
public:
    explicit SpatialSiteCrawler(QObject *parent = nullptr, SpatialSiteFetch *transport = nullptr);
    ~SpatialSiteCrawler() override;
    void start(const QUrl &url, SpatialSiteBudget budget = {}, bool refresh = false);
    void cancel();
    void request(const QString &id, bool refresh = false);
    void prefetch(const QString &section);
    void requestImage(const QUrl &url);
    void setOverride(const QString &id, SpatialSitePage::Override value);
    void save();
    SpatialSiteWorld &world() { return m_world; }
    const SpatialSiteWorld &world() const { return m_world; }
    bool running() const { return m_running; }
    bool interpreting() const { return m_interpreting; }
    int requests() const { return m_requests; }
    int bytes() const { return m_bytes; }
    QString status() const { return m_status; }
signals:
    void changed();
    void imageReady(const QUrl &url, const QByteArray &bytes);
    void stopped();
private:
    enum class Kind { Robots, Sitemap, Page, Image };
    struct Job {
        QUrl url;
        Kind kind = Kind::Page;
        int depth = 0;
        int redirects = 0;
        QUrl imageSource;
    };
    void pump();
    void receive(const SpatialSiteResponse &response);
    void acceptPage(SpatialSitePage page, const SpatialSiteResponse &response);
    void enqueue(const Job &job, bool priority = false);
    void complete(const QString &reason);
    SpatialSiteWorld m_world;
    SpatialSiteFetch *m_fetch;
    SpatialSiteBudget m_budget;
    SpatialRobotsPolicy m_robots;
    QQueue<Job> m_jobs;
    QSet<QString> m_seen;
    QTimer m_timer;
    QTimer m_deadline;
    QElapsedTimer m_clock;
    Job m_current;
    QString m_status;
    int m_requests = 0;
    int m_pageRequests = 0;
    int m_maps = 0;
    int m_bytes = 0;
    bool m_running = false;
    bool m_busy = false;
    bool m_policyReady = false;
    bool m_interpreting = false;
    quint64 m_generation = 0;
    QThreadPool m_interpreter;
};
