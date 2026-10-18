#pragma once
#include "SpatialSitePage.h"

struct SpatialSiteSection {
    QString id;
    QString title;
    int slot = 0;
};
class SpatialSiteWorld {
public:
    static constexpr int maximumPages = 256;
    static constexpr int maximumSections = 24;
    QUrl origin;
    QString title;
    QVector<SpatialSitePage> pages;
    QVector<SpatialSiteSection> sections;
    int threshold = 45;
    QMap<QString, QVector<float>> areaFeatures;
    SpatialSitePage *find(const QString &id);
    const SpatialSitePage *find(const QString &id) const;
    bool discover(const SpatialSiteLink &link, int depth);
    void update(SpatialSitePage page);
    void classifyAll();
    bool setOverride(const QString &id, SpatialSitePage::Override value);
    SpatialWebDocument project(const QString &section = {}, int page = 0) const;
    int areaPages(const QString &section) const;
    bool load(const QUrl &site, QString &error);
    bool save(QString &error) const;
    bool clear(QString &error);
    static QString cachePath(const QUrl &origin);
private:
    void organize(SpatialSitePage &page);
    mutable QByteArray m_revision;
    bool m_cacheWritable = true;
};
