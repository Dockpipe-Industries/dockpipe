#pragma once
#include "SpatialWebGraph.h"
#include <QDateTime>
#include <QSet>

struct SpatialSiteLink {
    QUrl url;
    QString title;
    QString relation;
    bool prominent = false;
};
struct SpatialSitePage {
    enum class Status { Discovered, Ready, Excluded, Unavailable };
    enum class Override { Automatic, Include, Exclude };
    SpatialWebNode node; // Existing semantic identity and action model; no geometry.
    QUrl canonical;
    QString summary;
    QString section;
    QString kind;
    QStringList headings;
    QStringList breadcrumbs;
    QVector<SpatialSiteLink> links;
    QVector<QUrl> assets;
    QSet<quint32> fingerprints;
    QString contentHash;
    QString duplicateOf;
    QString reason;
    QString modified;
    QDateTime fetched;
    Status status = Status::Discovered;
    Override override = Override::Automatic;
    int depth = 0;
    int score = 0;
    int characters = 0;
    bool prominent = false;
    bool noFollow = false;
    bool restricted = false;
    int slot = -1;
    QString group;
};
namespace SpatialSiteContent {
SpatialSitePage parse(const QByteArray &html, const QUrl &url);
void classify(SpatialSitePage &page, int threshold, const QString &siteSubject);
bool duplicate(const SpatialSitePage &first, const SpatialSitePage &second);
QString plain(const QString &html, int limit);
}
